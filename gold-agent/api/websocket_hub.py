import asyncio
import json
import logging
from typing import Any

from fastapi import WebSocket

logger = logging.getLogger(__name__)


class WebSocketHub:
    """
    Manages all active WebSocket connections and broadcasts messages.
    Thread-safe via asyncio.Lock.
    """

    def __init__(self):
        self._connections: set[WebSocket] = set()
        self._lock = asyncio.Lock()

    async def connect(self, websocket: WebSocket) -> None:
        """Accept and register a new WebSocket connection."""
        await websocket.accept()
        async with self._lock:
            self._connections.add(websocket)
        logger.info("WebSocket client connected. Total: %d", len(self._connections))

    async def disconnect(self, websocket: WebSocket) -> None:
        """Remove a WebSocket connection from the active set."""
        async with self._lock:
            self._connections.discard(websocket)
        logger.info("WebSocket client disconnected. Total: %d", len(self._connections))

    async def broadcast(self, message: dict[str, Any]) -> None:
        """
        Send a JSON message to all connected clients.
        Silently removes disconnected clients on send failure.
        """
        payload = json.dumps(message, default=str)

        async with self._lock:
            # Snapshot to avoid mutation during iteration
            connections = set(self._connections)

        dead: set[WebSocket] = set()
        for ws in connections:
            try:
                await ws.send_text(payload)
            except Exception:
                dead.add(ws)

        if dead:
            async with self._lock:
                self._connections -= dead
            logger.info("Removed %d dead WebSocket connection(s)", len(dead))

    # ------------------------------------------------------------------
    # Typed publish helpers — match the dashboard WebSocket message contract.
    # Callers must pass dicts already serialized with model_dump(by_alias=True)
    # so that all keys are camelCase.
    # ------------------------------------------------------------------

    async def publish_candle(self, candle_dict: dict) -> None:
        """Broadcast a candle_update event."""
        await self.broadcast({"type": "candle_update", "payload": candle_dict})

    async def publish_decision(self, decision_dict: dict) -> None:
        """Broadcast a decision_made event."""
        await self.broadcast({"type": "decision_made", "payload": decision_dict})

    async def publish_position(self, position_dict: dict, closed: bool = False) -> None:
        """Broadcast a position_update or position_closed event."""
        event_type = "position_closed" if closed else "position_update"
        await self.broadcast({"type": event_type, "payload": position_dict})

    async def publish_trade(self, position_dict: dict) -> None:
        """Broadcast a trade_executed event."""
        await self.broadcast({"type": "trade_executed", "payload": position_dict})

    async def publish_metrics(self, metrics_dict: dict) -> None:
        """Broadcast a metric_update event."""
        await self.broadcast({"type": "metric_update", "payload": metrics_dict})
