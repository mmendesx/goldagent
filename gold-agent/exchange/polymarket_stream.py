"""
Polymarket WebSocket stream client for crypto price feeds.

Connects to the Polymarket live-data WebSocket endpoint, subscribes to the
``crypto_prices`` topic, and delivers parsed :class:`~domain.types.PolymarketCryptoPrice`
objects to an ``asyncio.Queue``.

Credentials are required to start the stream (per spec).  When any credential
is absent the client logs a warning and returns immediately without raising.

Reconnect behaviour
-------------------
Disconnections are handled with exponential backoff starting at 1 s and capped
at 60 s.  The backoff resets to 1 s whenever a connection succeeds and delivers
at least one message before dropping — matching the strategy in the Go reference
client.
"""

from __future__ import annotations

import asyncio
import json
import logging
from datetime import datetime, timezone
from typing import TYPE_CHECKING

import websockets
from websockets.exceptions import WebSocketException

from domain.types import PolymarketCryptoPrice

if TYPE_CHECKING:
    pass

logger = logging.getLogger(__name__)

# Polymarket live-data WebSocket endpoint (source: Go reference client).
_WS_URL = "wss://ws-live-data.polymarket.com"

# Backoff constants — mirror the Go stream client.
_BASE_RECONNECT_DELAY = 1.0   # seconds
_MAX_RECONNECT_DELAY = 60.0   # seconds


class PolymarketStreamClient:
    """Maintains a persistent WebSocket connection to Polymarket for crypto price updates.

    Args:
        api_key:         Polymarket L2 API key.
        api_secret:      Polymarket L2 API secret.
        api_passphrase:  Polymarket L2 API passphrase.
        price_queue:     Async queue that receives :class:`~domain.types.PolymarketCryptoPrice`
                         objects.
        symbols:         Optional allowlist of symbols to forward, e.g.
                         ``["BTCUSDT", "ETHUSDT"]``.  When ``None`` or empty, all
                         symbols reported by the feed are forwarded.
    """

    def __init__(
        self,
        api_key: str,
        api_secret: str,
        api_passphrase: str,
        price_queue: asyncio.Queue,
        symbols: list[str] | None = None,
    ) -> None:
        self._configured = bool(api_key and api_secret and api_passphrase)
        self._price_queue = price_queue
        # Normalise to uppercase for consistent comparison; the feed sends e.g. "BTCUSDT".
        self._symbols: set[str] | None = (
            {s.upper() for s in symbols} if symbols else None
        )
        self._stopped = False

    # ------------------------------------------------------------------
    # Public interface
    # ------------------------------------------------------------------

    async def start(self) -> None:
        """Start streaming.  Skips gracefully if credentials are absent.

        Blocks the calling coroutine until :meth:`stop` is called.
        """
        if not self._configured:
            logger.warning(
                "Polymarket credentials not set — stream client disabled"
            )
            return

        logger.info(
            "PolymarketStreamClient starting: symbols_filter=%s",
            sorted(self._symbols) if self._symbols else "all",
        )
        await self._run_with_reconnect()

    async def stop(self) -> None:
        """Signal the client to stop after the current reconnect iteration."""
        logger.info("PolymarketStreamClient stopping")
        self._stopped = True

    # ------------------------------------------------------------------
    # Reconnect loop
    # ------------------------------------------------------------------

    async def _run_with_reconnect(self) -> None:
        delay = _BASE_RECONNECT_DELAY
        while not self._stopped:
            received_any = False
            try:
                received_any = await self._connect()
                # Healthy connection that delivered at least one message resets backoff.
                if received_any:
                    delay = _BASE_RECONNECT_DELAY
            except Exception as exc:  # noqa: BLE001
                logger.warning(
                    "Polymarket stream error: %s, reconnecting in %.1fs", exc, delay
                )
            else:
                if not self._stopped:
                    logger.info(
                        "Polymarket stream closed cleanly — reconnecting in %.1fs",
                        delay,
                    )

            if not self._stopped:
                await asyncio.sleep(delay)
                delay = min(delay * 2, _MAX_RECONNECT_DELAY)

    # ------------------------------------------------------------------
    # Connection lifecycle
    # ------------------------------------------------------------------

    async def _connect(self) -> bool:
        """Open one WebSocket connection, subscribe, and read until it closes.

        Returns:
            ``True`` if at least one message was received before the connection
            closed; ``False`` otherwise.  The caller uses this to decide whether
            to reset the reconnect backoff.

        Raises:
            WebSocketException: On connection or protocol errors.
            OSError:            On network-layer failures.
        """
        received_any = False

        async with websockets.connect(_WS_URL) as ws:
            logger.info("Polymarket WebSocket connected: url=%s", _WS_URL)

            await self._send_subscription(ws)

            async for raw in ws:
                if self._stopped:
                    break
                received_any = True
                self._handle_raw_message(raw)

        return received_any

    # ------------------------------------------------------------------
    # Subscription
    # ------------------------------------------------------------------

    async def _send_subscription(self, ws) -> None:  # noqa: ANN001
        """Send the crypto_prices subscription request."""
        request = {
            "action": "subscribe",
            "subscriptions": [
                {
                    "topic": "crypto_prices",
                    "type": "*",
                    "filters": "{}",
                }
            ],
        }
        payload = json.dumps(request)
        await ws.send(payload)
        logger.info("Polymarket subscription sent: topic=crypto_prices")

    # ------------------------------------------------------------------
    # Message handling
    # ------------------------------------------------------------------

    def _handle_raw_message(self, raw: str | bytes) -> None:
        """Parse an incoming WebSocket message and route it.

        Unrecognised topics are silently dropped.  Parse errors are logged
        at WARNING level and do not propagate — a single bad message must
        never interrupt the stream.
        """
        try:
            envelope = json.loads(raw)
        except json.JSONDecodeError as exc:
            logger.warning(
                "Polymarket failed to parse message envelope: %s | raw=%.200s",
                exc,
                raw,
            )
            return

        topic = envelope.get("topic")
        if topic == "crypto_prices":
            self._handle_crypto_price_message(envelope)
        else:
            logger.debug(
                "Polymarket received unhandled topic: topic=%s type=%s",
                topic,
                envelope.get("type"),
            )

    def _handle_crypto_price_message(self, envelope: dict) -> None:
        """Parse a crypto_prices payload and push it onto the price queue.

        The inner payload shape (from Go types.go)::

            {
                "symbol":    "BTCUSDT",
                "value":     "65432.10",
                "timestamp": 1713456789012
            }
        """
        payload = envelope.get("payload")
        if not isinstance(payload, dict):
            logger.warning(
                "Polymarket crypto_prices message missing or malformed payload: %s",
                envelope,
            )
            return

        symbol = payload.get("symbol", "")
        raw_value = payload.get("value")
        raw_timestamp = payload.get("timestamp")

        # Apply symbol filter when configured.
        if self._symbols and symbol.upper() not in self._symbols:
            logger.debug(
                "Polymarket dropping price for untracked symbol: symbol=%s", symbol
            )
            return

        try:
            value = float(raw_value)
        except (TypeError, ValueError) as exc:
            logger.warning(
                "Polymarket crypto_prices has unparseable value: symbol=%s value=%s error=%s",
                symbol,
                raw_value,
                exc,
            )
            return

        try:
            timestamp = datetime.fromtimestamp(
                int(raw_timestamp) / 1000.0, tz=timezone.utc
            )
        except (TypeError, ValueError, OSError) as exc:
            logger.warning(
                "Polymarket crypto_prices has unparseable timestamp: symbol=%s timestamp=%s error=%s",
                symbol,
                raw_timestamp,
                exc,
            )
            return

        price = PolymarketCryptoPrice(
            symbol=symbol,
            value=value,
            timestamp=timestamp,
        )

        try:
            self._price_queue.put_nowait(price)
        except asyncio.QueueFull:
            logger.warning(
                "Polymarket price queue full — dropping price update: symbol=%s value=%s",
                symbol,
                value,
            )
