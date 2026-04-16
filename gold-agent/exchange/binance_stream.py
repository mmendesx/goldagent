"""
Binance WebSocket stream client for kline and trade data.

Subscribes to ``{symbol}@kline_{interval}`` and ``{symbol}@miniTicker`` streams
for all configured symbols, parses incoming messages into domain objects, and
delivers them to an ``asyncio.Queue`` (candles) or an optional async callback
(ticker prices).

Thread model
------------
``binance-connector-python`` runs its WebSocket reader on a background daemon
thread.  All ``on_*`` callbacks are therefore invoked on that thread, not on
the event loop.  This module bridges the sync callbacks back to the async world
using:

- ``asyncio.run_coroutine_threadsafe`` for queue puts (coroutines).
- ``loop.call_soon_threadsafe``          for event signals (sync callables).
"""

from __future__ import annotations

import asyncio
import json
import logging
import threading
from collections.abc import Coroutine
from datetime import datetime, timezone
from typing import Callable

from binance.websocket.spot.websocket_client import SpotWebsocketClient

from domain.types import Candle

logger = logging.getLogger(__name__)

# Backoff constants — mirror the Go stream client.
_BASE_RECONNECT_DELAY = 1.0   # seconds
_MAX_RECONNECT_DELAY = 60.0   # seconds


class BinanceStreamClient:
    """Maintains a persistent Binance WebSocket connection for kline + ticker data.

    Args:
        stream_url:   Binance combined-stream base URL,
                      e.g. ``"wss://stream.binance.com:9443"``.
        symbols:      Symbols to subscribe to, e.g. ``["BTCUSDT", "ETHUSDT"]``.
        interval:     Kline interval, e.g. ``"5m"``.
        candle_queue: Async queue that receives :class:`~domain.types.Candle`
                      objects.  Both open (``is_closed=False``) and closed
                      (``is_closed=True``) candles are published.
        loop:         The running event loop.  Pass
                      ``asyncio.get_event_loop()`` from the main coroutine.
        on_ticker:    Optional async callback invoked with ``(symbol, price)``
                      on every miniTicker update.  Must be a coroutine function.
    """

    def __init__(
        self,
        stream_url: str,
        symbols: list[str],
        interval: str,
        candle_queue: asyncio.Queue,
        loop: asyncio.AbstractEventLoop,
        on_ticker: Callable[[str, float], Coroutine] | None = None,
    ) -> None:
        self._stream_url = stream_url
        # Normalise to uppercase for consistent logging; subscriptions use lowercase.
        self._symbols = [s.upper() for s in symbols]
        self._interval = interval
        self._candle_queue = candle_queue
        self._loop = loop
        self._on_ticker = on_ticker

        # _stopped is checked from both the async side (start/stop) and the sync
        # callbacks running on the connector thread.  threading.Event is
        # thread-safe; asyncio.Event is not.
        self._stopped = threading.Event()

        # Set by on_close / on_error to unblock _connect().
        self._disconnected: asyncio.Event | None = None

        self._ws_client: SpotWebsocketClient | None = None

    # ------------------------------------------------------------------
    # Public interface
    # ------------------------------------------------------------------

    async def start(self) -> None:
        """Start streaming with automatic reconnect on disconnect.

        Runs until :meth:`stop` is called.  Blocks the calling coroutine.
        """
        logger.info(
            "BinanceStreamClient starting: symbols=%s interval=%s",
            self._symbols,
            self._interval,
        )
        await self._run_with_reconnect()

    async def stop(self) -> None:
        """Gracefully stop the stream client."""
        logger.info("BinanceStreamClient stopping")
        self._stopped.set()
        if self._ws_client is not None:
            try:
                self._ws_client.stop()
            except Exception as exc:  # noqa: BLE001
                logger.warning("Error while stopping WebSocket client: %s", exc)
            self._ws_client = None
        # Unblock any pending _connect() wait.
        if self._disconnected is not None:
            self._loop.call_soon_threadsafe(self._disconnected.set)

    # ------------------------------------------------------------------
    # Reconnect loop
    # ------------------------------------------------------------------

    async def _run_with_reconnect(self) -> None:
        delay = _BASE_RECONNECT_DELAY
        while not self._stopped.is_set():
            try:
                await self._connect()
                # Successful connection reset the backoff only after _connect()
                # returns normally (i.e. the stream closed without an error).
                delay = _BASE_RECONNECT_DELAY
            except Exception as exc:  # noqa: BLE001
                logger.warning(
                    "Stream error: %s — reconnecting in %.1fs", exc, delay
                )
            else:
                if not self._stopped.is_set():
                    logger.info(
                        "Stream closed cleanly — reconnecting in %.1fs", delay
                    )

            if not self._stopped.is_set():
                await asyncio.sleep(delay)
                delay = min(delay * 2, _MAX_RECONNECT_DELAY)

    # ------------------------------------------------------------------
    # Connection lifecycle
    # ------------------------------------------------------------------

    async def _connect(self) -> None:
        """Open a single WebSocket connection, subscribe, and block until it drops."""
        # Fresh event for each connection attempt.
        self._disconnected = asyncio.Event()

        self._ws_client = SpotWebsocketClient(
            stream_url=self._stream_url,
            on_message=self._on_raw_message,
            on_open=self._on_open,
            on_error=self._on_error,
            on_close=self._on_close,
        )
        self._ws_client.start()

        # Subscribe to kline and miniTicker for each symbol.
        for i, symbol in enumerate(self._symbols):
            lower = symbol.lower()
            self._ws_client.kline(
                symbol=lower,
                interval=self._interval,
                id=i * 2 + 1,
            )
            self._ws_client.mini_ticker(
                symbol=lower,
                id=i * 2 + 2,
            )
            logger.info(
                "Subscribed to %s@kline_%s and %s@miniTicker",
                lower,
                self._interval,
                lower,
            )

        # Block until on_close or on_error signals the event.
        await self._disconnected.wait()

        # If stopped was set during the wait, return normally so the reconnect
        # loop exits cleanly on the next iteration check.

    # ------------------------------------------------------------------
    # Sync callbacks (called from connector background thread)
    # ------------------------------------------------------------------

    def _on_open(self, ws) -> None:  # noqa: ANN001
        logger.info("Binance WebSocket connection opened")

    def _on_close(self, ws) -> None:  # noqa: ANN001
        logger.info("Binance WebSocket connection closed")
        if self._disconnected is not None:
            self._loop.call_soon_threadsafe(self._disconnected.set)

    def _on_error(self, ws, error) -> None:  # noqa: ANN001
        logger.error("Binance WebSocket error: %s", error)
        if self._disconnected is not None:
            self._loop.call_soon_threadsafe(self._disconnected.set)

    def _on_raw_message(self, ws, raw_message: str) -> None:  # noqa: ANN001
        """Dispatch raw text messages to the appropriate parser.

        Called on the connector's background thread.  Must not block.
        """
        # The combined-stream endpoint wraps each message in:
        # {"stream": "btcusdt@kline_5m", "data": {...}}
        # Individual-stream subscriptions (SpotWebsocketClient.kline / .mini_ticker)
        # deliver the inner payload directly when connected to the combined-stream
        # endpoint, BUT when the client is using individual stream subscriptions it
        # delivers the raw inner event.  Handle both shapes.
        try:
            data = json.loads(raw_message)

            # Strip combined-stream envelope if present.
            if "stream" in data and "data" in data:
                data = data["data"]

            event_type = data.get("e")

            if event_type == "kline":
                self._handle_kline(data)
            elif event_type in ("24hrMiniTicker", "miniTicker"):
                self._handle_mini_ticker(data)
            # Ignore subscription confirmation messages and unknown types silently.
        except Exception as exc:  # noqa: BLE001
            logger.error("Error processing raw message: %s | raw=%s", exc, raw_message)

    # ------------------------------------------------------------------
    # Message handlers
    # ------------------------------------------------------------------

    def _handle_kline(self, data: dict) -> None:
        """Parse a kline event and put the resulting Candle onto the queue."""
        try:
            candle = _parse_kline(data)
        except Exception as exc:  # noqa: BLE001
            logger.error("Failed to parse kline message: %s | data=%s", exc, data)
            return

        try:
            self._candle_queue.put_nowait(candle)
        except asyncio.QueueFull:
            logger.warning(
                "Candle queue full — dropping candle: symbol=%s interval=%s open_time=%s",
                candle.symbol,
                candle.interval,
                candle.open_time,
            )

    def _handle_mini_ticker(self, data: dict) -> None:
        """Parse a miniTicker event and invoke the optional on_ticker callback."""
        if self._on_ticker is None:
            return

        try:
            symbol: str = data["s"]
            price: float = float(data["c"])  # "c" = last close price in miniTicker
        except (KeyError, ValueError) as exc:
            logger.error(
                "Failed to parse miniTicker message: %s | data=%s", exc, data
            )
            return

        asyncio.run_coroutine_threadsafe(
            self._on_ticker(symbol, price),
            self._loop,
        )


# ---------------------------------------------------------------------------
# Pure parsing functions — no side effects, easy to unit-test
# ---------------------------------------------------------------------------

def _parse_kline(data: dict) -> Candle:
    """Parse a raw Binance kline event dict into a :class:`~domain.types.Candle`.

    Prices are kept as strings exactly as Binance sends them to avoid
    floating-point rounding artefacts and to match the ``Candle`` model's
    ``str`` price fields.

    Args:
        data: The decoded JSON object for a ``kline`` event, e.g.::

                {
                    "e": "kline",
                    "E": 1638747660000,
                    "s": "BTCUSDT",
                    "k": {
                        "t": 1638747600000,
                        "T": 1638747659999,
                        "s": "BTCUSDT",
                        "i": "1m",
                        "o": "49000.00",
                        "h": "49100.00",
                        "l": "48900.00",
                        "c": "49050.00",
                        "v": "100.5",
                        "q": "4925025.0",
                        "n": 1234,
                        "x": true
                    }
                }

    Returns:
        A :class:`~domain.types.Candle` with ``is_closed`` reflecting the
        ``"x"`` flag from the message.

    Raises:
        KeyError:  If a required field is missing from the payload.
        TypeError: If a field has an unexpected type.
    """
    k = data["k"]

    open_time = datetime.fromtimestamp(k["t"] / 1000.0, tz=timezone.utc)
    close_time = datetime.fromtimestamp(k["T"] / 1000.0, tz=timezone.utc)

    return Candle(
        symbol=k["s"],
        interval=k["i"],
        open_time=open_time,
        close_time=close_time,
        open_price=k["o"],
        high_price=k["h"],
        low_price=k["l"],
        close_price=k["c"],
        volume=k["v"],
        quote_volume=k["q"],
        trade_count=int(k["n"]),
        is_closed=bool(k["x"]),
    )
