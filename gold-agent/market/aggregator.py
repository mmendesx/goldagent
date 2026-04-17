"""
Candle aggregator for the gold-agent trading pipeline.

Consumes raw candle updates from the Binance stream queue and routes them:
  - All candles: update the Redis ticker price and cache the latest candle.
  - Closed candles only: upsert to Postgres, fan out to closed_candle_queue,
    and broadcast to the WebSocket hub.
  - Open (in-progress) candles: Redis update and broadcast only. No DB write.
"""

from __future__ import annotations

import asyncio
import logging
from typing import Any, Callable, Coroutine

from domain.types import Candle
from storage import postgres, redis_client

logger = logging.getLogger(__name__)


class CandleAggregator:
    """
    Processes candle updates from a stream queue and routes them downstream.

    Args:
        candle_queue: Input queue; receives every candle update (open + closed).
        closed_candle_queue: Output queue; receives only fully closed candles.
            Used by the indicator calculation and LLM decision pipeline.
        on_broadcast: Optional async callback for WebSocket fan-out.
            Called with a typed message dict for both open and closed candles.
    """

    def __init__(
        self,
        candle_queue: asyncio.Queue,
        closed_candle_queue: asyncio.Queue,
        on_broadcast: Callable[[dict], Coroutine[Any, Any, None]] | None = None,
    ) -> None:
        self._candle_queue = candle_queue
        self._closed_candle_queue = closed_candle_queue
        self._on_broadcast = on_broadcast
        self._running = False

    async def run(self) -> None:
        """Consume candles from the input queue indefinitely until stop() is called.

        Uses a 1-second drain timeout so the loop can observe the _running flag
        without blocking shutdown for the duration of an idle queue wait.
        """
        self._running = True
        logger.info("CandleAggregator started")

        while self._running:
            try:
                candle: Candle = await asyncio.wait_for(
                    self._candle_queue.get(), timeout=1.0
                )
            except asyncio.TimeoutError:
                # No candle arrived within the drain window — check _running and loop.
                continue
            except Exception as e:
                logger.error(
                    "CandleAggregator: unexpected error reading from queue",
                    exc_info=True,
                    extra={"error": str(e)},
                )
                continue

            try:
                await self._process(candle)
            except Exception as e:
                logger.error(
                    "CandleAggregator: error processing candle",
                    exc_info=True,
                    extra={
                        "symbol": candle.symbol,
                        "interval": candle.interval,
                        "is_closed": candle.is_closed,
                        "error": str(e),
                    },
                )
            finally:
                self._candle_queue.task_done()

        logger.info("CandleAggregator stopped")

    async def stop(self) -> None:
        """Signal the run loop to exit after the current drain timeout."""
        self._running = False

    async def _process(self, candle: Candle) -> None:
        """Route a single candle through the appropriate persistence and fan-out steps.

        Always:
          1. Cache ticker price in Redis (used by position monitor for P&L checks).
          2. Cache the latest candle snapshot in Redis.
          3. Broadcast to WebSocket hub.

        Additionally, if the candle is closed:
          4. Upsert to Postgres for durable storage.
          5. Place on closed_candle_queue for indicator + LLM pipeline.
        """
        # Step 1: Keep the hot ticker price fresh for the position monitor.
        # Cache failure is best-effort — log and continue.
        try:
            await redis_client.cache_ticker_price(candle.symbol, float(candle.close_price))
        except Exception as e:
            logger.warning(
                "CandleAggregator: failed to cache ticker price",
                extra={"symbol": candle.symbol, "error": str(e)},
            )

        # Step 2: Cache the latest candle snapshot (open or closed).
        try:
            await redis_client.cache_candle(
                candle.symbol,
                candle.interval,
                candle.model_dump(by_alias=True, mode="json"),
            )
        except Exception as e:
            logger.warning(
                "CandleAggregator: failed to cache candle",
                extra={"symbol": candle.symbol, "interval": candle.interval, "error": str(e)},
            )

        if candle.is_closed:
            await self._handle_closed_candle(candle)
        else:
            logger.debug(
                "CandleAggregator: open candle — Redis updated, skipping DB write",
                extra={"symbol": candle.symbol, "interval": candle.interval},
            )

        # Step 3: Broadcast to dashboard (both open and closed candles).
        await self._broadcast_candle(candle)

    async def _handle_closed_candle(self, candle: Candle) -> None:
        """Persist a closed candle and fan it out to downstream consumers."""
        # Step 4: Upsert to Postgres. Failure is logged but does not drop the fan-out;
        # a missing DB record is recoverable, but a stalled decision pipeline is not.
        try:
            await postgres.save_candle(candle)
            logger.info(
                "CandleAggregator: closed candle persisted",
                extra={
                    "symbol": candle.symbol,
                    "interval": candle.interval,
                    "open_time": str(candle.open_time),
                },
            )
        except Exception as e:
            logger.error(
                "CandleAggregator: failed to persist closed candle",
                exc_info=True,
                extra={
                    "symbol": candle.symbol,
                    "interval": candle.interval,
                    "open_time": str(candle.open_time),
                    "error": str(e),
                },
            )

        # Step 5: Fan out to the indicator + LLM pipeline queue.
        # Non-blocking put — a full queue drops the candle rather than stalling
        # the aggregator. The warning gives ops visibility into backpressure.
        try:
            self._closed_candle_queue.put_nowait(candle)
            logger.debug(
                "CandleAggregator: closed candle queued for analysis",
                extra={"symbol": candle.symbol, "interval": candle.interval},
            )
        except asyncio.QueueFull:
            logger.warning(
                "CandleAggregator: closed candle queue full, dropping candle",
                extra={"symbol": candle.symbol, "interval": candle.interval},
            )

    async def _broadcast_candle(self, candle: Candle) -> None:
        """Send a candle update to the WebSocket hub if a broadcast callback is registered."""
        if self._on_broadcast is None:
            return

        message = {
            "type": "candle_update",
            "payload": candle.model_dump(by_alias=True, mode="json"),
        }

        try:
            await self._on_broadcast(message)
        except Exception as e:
            logger.error(
                "CandleAggregator: broadcast callback raised an error",
                exc_info=True,
                extra={
                    "symbol": candle.symbol,
                    "interval": candle.interval,
                    "error": str(e),
                },
            )
