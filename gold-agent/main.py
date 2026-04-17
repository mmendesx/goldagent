"""
Gold Agent entry point.

Wires all components and starts the concurrent pipeline using asyncio.TaskGroup
(Python 3.12). Handles SIGINT/SIGTERM for graceful shutdown.
"""

from __future__ import annotations

import asyncio
import logging
import os
import signal
import sys

_PKG_DIR = os.path.dirname(os.path.abspath(__file__))
if _PKG_DIR not in sys.path:
    sys.path.insert(0, _PKG_DIR)

from datetime import datetime, timezone  # noqa: E402

import uvicorn  # noqa: E402

from config import settings  # noqa: E402
from storage import postgres, redis_client  # noqa: E402
from exchange.binance_stream import BinanceStreamClient  # noqa: E402
from exchange.binance_rest import BinanceRestClient  # noqa: E402
from exchange.polymarket_stream import PolymarketStreamClient  # noqa: E402
from exchange.polymarket_rest import PolymarketRestClient  # noqa: E402
from market.aggregator import CandleAggregator  # noqa: E402
from analysis.indicators import compute_indicators  # noqa: E402
from engine.context_builder import build_context  # noqa: E402
from engine.llm_engine import LLMDecisionEngine  # noqa: E402
from engine.risk import RiskGate  # noqa: E402
from execution.executor import Executor  # noqa: E402
from execution.position_monitor import PositionMonitor  # noqa: E402
from execution.portfolio_manager import PortfolioManager  # noqa: E402
from api.websocket_hub import WebSocketHub  # noqa: E402
from api.router import create_app  # noqa: E402
from domain.types import DecisionAction, OrderSide, TradeIntent  # noqa: E402

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s %(name)s %(levelname)s %(message)s",
)
logger = logging.getLogger(__name__)

# Queue capacity constants — backpressure limits, not ring buffers.
_CANDLE_STREAM_BUFFER = 512
_CLOSED_CANDLE_BUFFER = 256
_POLYMARKET_BUFFER = 64


async def main() -> None:
    logger.info("Gold Agent starting up...")

    # --- Storage initialisation ---
    try:
        await postgres.create_pool()
        logger.info("PostgreSQL pool connected")
    except Exception as exc:
        logger.critical("Failed to connect to PostgreSQL: %s", exc)
        sys.exit(1)

    await redis_client.create_client(settings.gold_redis_url)
    logger.info("Redis client connected")

    # --- Queues ---
    candle_queue: asyncio.Queue = asyncio.Queue(maxsize=_CANDLE_STREAM_BUFFER)
    closed_candle_queue: asyncio.Queue = asyncio.Queue(maxsize=_CLOSED_CANDLE_BUFFER)
    polymarket_queue: asyncio.Queue = asyncio.Queue(maxsize=_POLYMARKET_BUFFER)

    # --- Exchange clients ---
    binance_rest = BinanceRestClient(
        api_key=settings.binance_api_key,
        api_secret=settings.binance_api_secret,
        base_url=settings.binance_rest_api_url,
    )
    polymarket_rest = PolymarketRestClient(
        api_key=settings.polymarket_api_key,
        api_secret=settings.polymarket_api_secret,
        passphrase=settings.polymarket_api_passphrase,
        private_key=settings.polymarket_private_key,
        wallet_address=settings.polymarket_wallet_address,
    )

    # --- WebSocket hub ---
    hub = WebSocketHub()

    # --- Core components ---
    aggregator = CandleAggregator(
        candle_queue=candle_queue,
        closed_candle_queue=closed_candle_queue,
        on_broadcast=hub.broadcast,
    )

    llm_engine = LLMDecisionEngine()
    risk_gate = RiskGate()
    executor = Executor(binance_client=binance_rest, polymarket_client=polymarket_rest)
    portfolio_manager = PortfolioManager(initial_balance=10_000.0)
    position_monitor = PositionMonitor(executor=executor)

    # In-memory Polymarket price cache: {symbol: {"value": float, "timestamp": str}}
    polymarket_cache: dict[str, dict] = {}

    # --- Binance stream client ---
    async def on_ticker(sym: str, price: float) -> None:
        await redis_client.cache_ticker_price(sym, price)
        await hub.publish_ticker(sym, str(price), datetime.now(timezone.utc).isoformat())

    loop = asyncio.get_running_loop()
    binance_stream = BinanceStreamClient(
        stream_url=settings.binance_websocket_stream_url,
        symbols=settings.gold_symbols,
        interval=settings.gold_default_interval,
        candle_queue=candle_queue,
        loop=loop,
        on_ticker=on_ticker,
    )

    # --- Polymarket stream client ---
    polymarket_stream = PolymarketStreamClient(
        api_key=settings.polymarket_api_key,
        api_secret=settings.polymarket_api_secret,
        api_passphrase=settings.polymarket_api_passphrase,
        price_queue=polymarket_queue,
        symbols=settings.gold_symbols,
    )

    # --- FastAPI application ---
    app = create_app(
        hub=hub,
        binance_client=binance_rest,
        polymarket_client=polymarket_rest,
    )

    # --- Uvicorn server ---
    # loop="none" reuses the existing asyncio event loop rather than creating its own.
    uv_config = uvicorn.Config(
        app=app,
        host="0.0.0.0",
        port=settings.gold_http_port,
        log_level="info",
        loop="none",
    )
    uv_server = uvicorn.Server(uv_config)

    # --- Graceful shutdown ---
    shutdown_event = asyncio.Event()

    def _handle_signal(sig: signal.Signals) -> None:
        logger.info("Received signal %s — initiating shutdown...", sig.name)
        shutdown_event.set()

    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, _handle_signal, sig)

    # --- Task implementations ---

    async def indicator_and_decision_loop() -> None:
        """Consume closed candles, compute indicators, run LLM, execute if warranted."""
        logger.info("Decision loop started")
        while True:
            try:
                candle = await asyncio.wait_for(
                    closed_candle_queue.get(), timeout=1.0
                )
            except asyncio.TimeoutError:
                # Check whether shutdown was requested during the idle wait.
                if shutdown_event.is_set():
                    logger.info("Decision loop stopping")
                    return
                continue
            except Exception as exc:
                logger.error("Decision loop: queue read error: %s", exc, exc_info=True)
                continue

            try:
                # Indicators — may return None if insufficient candle history.
                await compute_indicators(
                    symbol=candle.symbol,
                    interval=candle.interval,
                    candle=candle,
                )

                # Build LLM context from DB + polymarket cache.
                context = await build_context(
                    symbol=candle.symbol,
                    interval=candle.interval,
                    candle=candle,
                    polymarket_cache=polymarket_cache,
                )

                # Call LLM and persist the decision.
                decision = await llm_engine.evaluate(
                    context=context,
                    symbol=candle.symbol,
                    is_dry_run=settings.gold_dry_run,
                )

                # Broadcast decision to dashboard.
                await hub.publish_decision(decision.model_dump(by_alias=True))

                # Apply risk gates before execution.
                portfolio = await postgres.fetch_latest_portfolio()
                open_positions = await postgres.fetch_open_positions(symbol=candle.symbol)
                risk_result = risk_gate.check(decision, portfolio, len(open_positions))

                if risk_result.is_allowed and not settings.gold_dry_run:
                    balance = float(portfolio.balance) if portfolio else 10_000.0
                    entry_price = float(candle.close_price)
                    pct = settings.gold_max_position_size_percent / 100.0
                    qty = (balance * pct) / entry_price if entry_price > 0 else 0.0

                    side = (
                        OrderSide.BUY
                        if decision.action == DecisionAction.BUY
                        else OrderSide.SELL
                    )

                    intent = TradeIntent(
                        decision_id=decision.id,
                        symbol=candle.symbol,
                        side=side,
                        estimated_entry_price=entry_price,
                        position_size_qty=qty,
                        created_at=datetime.now(timezone.utc),
                    )
                    await executor.execute(intent)

                elif risk_result.rejection_reason:
                    logger.info(
                        "Decision rejected by risk gate: symbol=%s action=%s reason=%s",
                        candle.symbol,
                        decision.action.value,
                        risk_result.rejection_reason,
                    )

            except Exception as exc:
                logger.error(
                    "Decision loop: unhandled error for symbol=%s: %s",
                    candle.symbol,
                    exc,
                    exc_info=True,
                )
            finally:
                closed_candle_queue.task_done()

    async def polymarket_price_collector() -> None:
        """Drain the Polymarket price queue into the in-memory cache."""
        while True:
            try:
                price_update = await asyncio.wait_for(
                    polymarket_queue.get(), timeout=1.0
                )
                polymarket_cache[price_update.symbol] = {
                    "value": price_update.value,
                    "timestamp": price_update.timestamp.isoformat(),
                }
                polymarket_queue.task_done()
            except asyncio.TimeoutError:
                if shutdown_event.is_set():
                    logger.info("Polymarket price collector stopping")
                    return
                continue
            except Exception as exc:
                logger.error("Polymarket collector: error: %s", exc, exc_info=True)

    async def shutdown_watcher() -> None:
        """Wait for shutdown signal, then stop all components and the HTTP server.

        Does NOT close Postgres or Redis here — those must outlive all other tasks
        so that in-flight DB writes during the drain period complete cleanly.
        Postgres/Redis are closed after the TaskGroup exits.
        """
        await shutdown_event.wait()
        logger.info("Shutting down components...")

        # Signal all looping components to stop.
        await aggregator.stop()
        await binance_stream.stop()
        await polymarket_stream.stop()
        await portfolio_manager.stop()
        await position_monitor.stop()

        # Tell uvicorn to finish serving in-flight requests and exit.
        uv_server.should_exit = True

        logger.info("Shutdown signal propagated to all components")

    # --- Startup log ---
    logger.info("Starting HTTP server on port %d", settings.gold_http_port)
    logger.info("Dry-run mode: %s", settings.gold_dry_run)
    logger.info("Symbols: %s", settings.gold_symbols)
    logger.info("LLM model: %s", settings.gold_llm_model)

    # --- Launch all tasks concurrently ---
    # TaskGroup cancels all sibling tasks if any task raises an unhandled exception.
    # Every long-running task catches its own exceptions internally, so the group
    # only exits via the shutdown path.
    try:
        async with asyncio.TaskGroup() as tg:
            tg.create_task(aggregator.run(), name="candle-aggregator")
            tg.create_task(binance_stream.start(), name="binance-stream")
            tg.create_task(polymarket_stream.start(), name="polymarket-stream")
            tg.create_task(indicator_and_decision_loop(), name="decision-loop")
            tg.create_task(polymarket_price_collector(), name="polymarket-collector")
            tg.create_task(portfolio_manager.run(), name="portfolio-manager")
            tg.create_task(uv_server.serve(), name="http-server")
            tg.create_task(shutdown_watcher(), name="shutdown-watcher")

            if not settings.gold_dry_run:
                # PositionMonitor.run() also guards internally, but the explicit
                # guard here matches the spec and avoids an unnecessary running task.
                tg.create_task(position_monitor.run(), name="position-monitor")
    except* Exception as eg:
        for exc in eg.exceptions:
            logger.error("Task group exited with error: %s", exc, exc_info=exc)

    # --- Teardown storage after all tasks have finished ---
    # Postgres and Redis are closed here, not inside shutdown_watcher, so that
    # any in-flight DB writes during the drain window complete before the pool
    # is released.
    await postgres.close_pool()
    await redis_client.close_client()
    logger.info("Shutdown complete")


if __name__ == "__main__":
    asyncio.run(main())
