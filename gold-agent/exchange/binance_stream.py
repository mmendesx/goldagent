"""
Binance WebSocket stream client for kline and trade data.

Subscribes to ``{symbol}@kline_{interval}`` and ``{symbol}@miniTicker`` streams
for all configured symbols, parses incoming messages into domain objects, and
delivers them to an ``asyncio.Queue`` (candles) or an optional async callback
(ticker prices).

Async model
-----------
``binance-sdk-spot`` uses an aiohttp-based async WebSocket client.  Callbacks
registered via ``handle.on("message", cb)`` are called synchronously from
within the SDK's async receive loop on the running event loop.  No thread
bridging is required:

- ``queue.put_nowait(candle)``             for candle delivery (sync, non-blocking).
- ``asyncio.ensure_future(coro)``          for the optional async on_ticker callback.
"""

from __future__ import annotations

import asyncio
import logging
from collections.abc import Coroutine
from datetime import datetime, timezone
from typing import Callable

from binance_sdk_spot.websocket_streams import SpotWebSocketStreams
from binance_common.configuration import ConfigurationWebSocketStreams
from binance_sdk_spot.websocket_streams.models import (
    KlineResponse,
    MiniTickerResponse,
    KlineIntervalEnum,
)

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

        # asyncio.Event is safe here — everything runs on the single event loop.
        self._stopped = asyncio.Event()

        self._ws_client: SpotWebSocketStreams | None = None

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
                await self._ws_client.close_connection(close_session=True)
            except Exception as exc:  # noqa: BLE001
                logger.warning("Error while closing WebSocket connection: %s", exc)
            self._ws_client = None

    # ------------------------------------------------------------------
    # Reconnect loop
    # ------------------------------------------------------------------

    async def _run_with_reconnect(self) -> None:
        delay = _BASE_RECONNECT_DELAY
        while not self._stopped.is_set():
            try:
                await self._connect()
                # Successful connection: reset backoff only after clean return.
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
        configuration = ConfigurationWebSocketStreams(stream_url=self._stream_url)
        self._ws_client = SpotWebSocketStreams(configuration)

        # Establish the underlying aiohttp WebSocket connection before subscribing.
        await self._ws_client.connect(self._stream_url, configuration)

        interval_enum = KlineIntervalEnum(self._interval)

        for symbol in self._symbols:
            lower = symbol.lower()

            kline_handle = await self._ws_client.kline(lower, interval_enum)
            kline_handle.on("message", self._make_kline_callback(symbol))

            ticker_handle = await self._ws_client.mini_ticker(lower)
            ticker_handle.on("message", self._make_ticker_callback())

            logger.info(
                "Subscribed to %s@kline_%s and %s@miniTicker",
                lower,
                self._interval,
                lower,
            )

        # Block until stopped; the SDK's receive loop runs as a background task.
        await self._stopped.wait()

        # Tear down the connection now that stop() has been called.
        if self._ws_client is not None:
            try:
                await self._ws_client.close_connection(close_session=True)
            except Exception as exc:  # noqa: BLE001
                logger.warning("Error closing connection after stop signal: %s", exc)
            self._ws_client = None

    # ------------------------------------------------------------------
    # Callback factories
    # ------------------------------------------------------------------

    def _make_kline_callback(self, symbol: str) -> Callable:
        """Return a sync callback that handles KlineResponse messages for *symbol*."""

        def _on_kline_message(msg: KlineResponse) -> None:
            try:
                candle = _parse_kline_response(msg)
            except Exception as exc:  # noqa: BLE001
                logger.error(
                    "Failed to parse kline message: %s | symbol=%s", exc, symbol
                )
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

        return _on_kline_message

    def _make_ticker_callback(self) -> Callable:
        """Return a sync callback that handles MiniTickerResponse messages."""

        def _on_ticker_message(msg: MiniTickerResponse) -> None:
            if self._on_ticker is None:
                return

            try:
                symbol: str = msg.s
                price: float = float(msg.c)
            except (AttributeError, ValueError) as exc:
                logger.error("Failed to parse miniTicker message: %s", exc)
                return

            # on_ticker is an async coroutine — schedule it on the running loop.
            asyncio.ensure_future(self._on_ticker(symbol, price))

        return _on_ticker_message


# ---------------------------------------------------------------------------
# Pure parsing functions — no side effects, easy to unit-test
# ---------------------------------------------------------------------------

def _parse_kline_response(msg: KlineResponse) -> Candle:
    """Parse a :class:`KlineResponse` SDK model into a :class:`~domain.types.Candle`.

    Prices are kept as strings exactly as Binance sends them to avoid
    floating-point rounding artefacts and to match the ``Candle`` model's
    ``str`` price fields.

    Args:
        msg: A ``KlineResponse`` object delivered by the SDK.

    Returns:
        A :class:`~domain.types.Candle` with ``is_closed`` reflecting the
        ``x`` flag from the kline payload.

    Raises:
        AttributeError: If a required attribute is missing from the model.
        TypeError:      If a field has an unexpected type.
    """
    k = msg.k

    open_time = datetime.fromtimestamp(k.t / 1000.0, tz=timezone.utc)
    close_time = datetime.fromtimestamp(k.T / 1000.0, tz=timezone.utc)

    return Candle(
        symbol=k.s,
        interval=k.i,
        open_time=open_time,
        close_time=close_time,
        open_price=k.o,
        high_price=k.h,
        low_price=k.l,
        close_price=k.c,
        volume=k.v,
        quote_volume=k.q,
        trade_count=int(k.n),
        is_closed=bool(k.x),
    )


# ---------------------------------------------------------------------------
# Backward-compatible dict-based parser (kept for unit tests that inject raw dicts)
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
