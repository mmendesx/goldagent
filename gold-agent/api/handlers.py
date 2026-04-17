"""
REST and WebSocket handlers for the Gold Trading Agent API.

All endpoints maintain wire-format compatibility with the Go backend so the
existing React dashboard works without changes. Responses are serialized with
model_dump(by_alias=True) to produce camelCase keys.
"""

import asyncio
import logging
from typing import Optional

from fastapi import APIRouter, Query, WebSocket, WebSocketDisconnect

from domain.types import ExchangeBalance, ExchangeBalanceStatus, ExchangeBalances, Paginated, PortfolioMetrics
from storage import postgres
from exchange.binance_rest import BinanceRestClient
from exchange.polymarket_rest import PolymarketRestClient
from .websocket_hub import WebSocketHub

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Module-level state — set once at startup via init_handlers()
# ---------------------------------------------------------------------------

_hub: Optional[WebSocketHub] = None
_binance_client: Optional[BinanceRestClient] = None
_polymarket_client: Optional[PolymarketRestClient] = None


def init_handlers(
    hub: WebSocketHub,
    binance: BinanceRestClient,
    polymarket: PolymarketRestClient,
) -> None:
    """Inject runtime dependencies into handler module state.

    Called once during application startup before any requests are served.
    """
    global _hub, _binance_client, _polymarket_client
    _hub = hub
    _binance_client = binance
    _polymarket_client = polymarket
    logger.info("handlers initialised with hub=%r binance=%r polymarket=%r", hub, binance, polymarket)


# ---------------------------------------------------------------------------
# Router
# ---------------------------------------------------------------------------

router = APIRouter()


# ---------------------------------------------------------------------------
# REST endpoints
# ---------------------------------------------------------------------------

@router.get("/api/v1/candles")
async def get_candles(
    symbol: str = Query("BTCUSDT"),
    interval: str = Query("5m"),
    limit: int = Query(50, ge=1, le=500),
    offset: int = Query(0, ge=0),
):
    """Return a paginated window of candles for a symbol+interval, ordered ASC by openTime."""
    candles = await postgres.fetch_candles(symbol, interval, limit=limit, offset=offset)
    page: Paginated = Paginated(
        items=candles,
        limit=limit,
        offset=offset,
        count=offset + len(candles),
        has_more=len(candles) == limit,
    )
    return page.model_dump(by_alias=True)


@router.get("/api/v1/positions")
async def get_positions(
    symbol: Optional[str] = Query(None),
    status: Optional[str] = Query(None),
):
    """Return positions filtered by optional symbol and status (open/closed)."""
    positions = await postgres.fetch_positions(symbol=symbol, status=status)
    return [p.model_dump(by_alias=True) for p in positions]


@router.get("/api/v1/trades")
async def get_trades(
    symbol: Optional[str] = Query(None),
    limit: int = Query(50, ge=1, le=500),
    offset: int = Query(0, ge=0),
):
    """Return a paginated window of closed trade history. Closed positions serve as trade records."""
    positions = await postgres.fetch_positions(symbol=symbol, status="closed", limit=limit, offset=offset)
    page: Paginated = Paginated(
        items=positions,
        limit=limit,
        offset=offset,
        count=offset + len(positions),
        has_more=len(positions) == limit,
    )
    return page.model_dump(by_alias=True)


@router.get("/api/v1/decisions")
async def get_decisions(
    symbol: Optional[str] = Query(None),
    limit: int = Query(50, ge=1, le=500),
    offset: int = Query(0, ge=0),
):
    """Return a paginated window of the decision log ordered by createdAt DESC."""
    decisions = await postgres.fetch_decisions(symbol=symbol, limit=limit, offset=offset)
    page: Paginated = Paginated(
        items=decisions,
        limit=limit,
        offset=offset,
        count=offset + len(decisions),
        has_more=len(decisions) == limit,
    )
    return page.model_dump(by_alias=True)


@router.get("/api/v1/metrics")
async def get_metrics():
    """Return the latest portfolio metrics snapshot.

    Returns a zero-value metrics object when no snapshot exists yet so the
    dashboard never receives a null response.
    """
    metrics = await postgres.fetch_latest_portfolio()
    if metrics is None:
        logger.info("get_metrics: no portfolio snapshot found, returning zero defaults")
        metrics = PortfolioMetrics(
            balance="0",
            peak_balance="0",
            drawdown_percent="0",
            total_pnl="0",
            win_count=0,
            loss_count=0,
            total_trades=0,
            win_rate="0",
            profit_factor="0",
            average_win="0",
            average_loss="0",
            sharpe_ratio="0",
            max_drawdown_percent="0",
            is_circuit_breaker_active=False,
        )
    return metrics.model_dump(by_alias=True)


@router.get("/api/v1/exchange/balances")
async def get_exchange_balances():
    """Return live USDT (Binance) and USDC (Polymarket) balances.

    Returns status="not_configured" for any exchange whose client was not
    initialised (missing API credentials).
    """
    not_configured = ExchangeBalance(
        balance="0",
        status=ExchangeBalanceStatus.NOT_CONFIGURED,
    )

    if _binance_client is None or _polymarket_client is None:
        logger.warning(
            "get_exchange_balances called before init_handlers; "
            "binance_client=%r polymarket_client=%r",
            _binance_client,
            _polymarket_client,
        )
        balances = ExchangeBalances(
            binance=not_configured,
            polymarket=not_configured,
        )
        return balances.model_dump(by_alias=True)

    binance_balance, polymarket_balance = await asyncio.gather(
        _binance_client.fetch_usdt_balance(),
        _polymarket_client.fetch_usdc_balance(),
    )

    balances = ExchangeBalances(
        binance=binance_balance,
        polymarket=polymarket_balance,
    )
    return balances.model_dump(by_alias=True)


# ---------------------------------------------------------------------------
# WebSocket endpoint
# ---------------------------------------------------------------------------

@router.websocket("/ws/v1/stream")
async def websocket_stream(websocket: WebSocket):
    """Real-time push stream. Read-only: messages from the client are discarded.

    The hub broadcasts candle, position, decision, and metric events to all
    connected clients. The connection is kept alive until the client disconnects
    or an unrecoverable error occurs.
    """
    if _hub is None:
        # Hub not ready; close immediately rather than leaving a dangling connection.
        logger.warning("websocket_stream: hub not initialised, rejecting connection")
        await websocket.close(code=1011, reason="server not ready")
        return

    await _hub.connect(websocket)
    try:
        while True:
            # Read and discard incoming messages. Serves two purposes:
            # 1. Detects disconnects via WebSocketDisconnect.
            # 2. Prevents the read buffer from filling if the client sends pings.
            await websocket.receive_text()
    except WebSocketDisconnect:
        logger.debug("websocket_stream: client disconnected normally")
    except Exception as exc:
        # Covers RuntimeError from broken frames and any unexpected transport error.
        logger.warning("websocket_stream: connection closed with error: %s", exc)
    finally:
        await _hub.disconnect(websocket)
