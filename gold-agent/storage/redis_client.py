"""
Async Redis client for the gold-agent trading pipeline.

Provides a module-level singleton client with JSON serialization helpers and
convenience accessors for the two cache namespaces used by the pipeline:
  - candle:{symbol}:{interval}:latest  (TTL 3600 s)
  - ticker:{symbol}:price              (TTL 60 s)
"""

from __future__ import annotations

import json
import logging
from typing import Any

import redis.asyncio as redis

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Module-level singleton
# ---------------------------------------------------------------------------

_client: redis.Redis | None = None


async def create_client(url: str) -> redis.Redis:
    """Create and store the global Redis client.

    Args:
        url: Redis connection URL, e.g. ``redis://localhost:6379/0``.

    Returns:
        The newly created :class:`redis.asyncio.Redis` instance.
    """
    global _client
    _client = redis.from_url(url, decode_responses=True)
    logger.info("Redis client created for %s", url)
    return _client


async def close_client() -> None:
    """Close the Redis connection and release the global reference."""
    global _client
    if _client is not None:
        await _client.aclose()
        _client = None
        logger.info("Redis client closed")


def get_client() -> redis.Redis:
    """Return the global Redis client.

    Raises:
        RuntimeError: If :func:`create_client` has not been called yet.
    """
    if _client is None:
        raise RuntimeError(
            "Redis client is not initialized. Call create_client() before use."
        )
    return _client


# ---------------------------------------------------------------------------
# Generic key/value helpers
# ---------------------------------------------------------------------------

async def set_value(key: str, value: Any, ttl: int | None = None) -> None:
    """Serialize *value* to JSON and store it under *key*.

    Args:
        key: Redis key.
        value: Any JSON-serializable Python object.
        ttl: Optional expiry in seconds. Omit to store without expiry.
    """
    client = get_client()
    serialized = json.dumps(value)
    await client.set(key, serialized, ex=ttl)
    logger.debug("set key=%s ttl=%s", key, ttl)


async def get_value(key: str) -> Any | None:
    """Retrieve and JSON-deserialize a value.

    Returns:
        The deserialized value, or ``None`` on a cache miss.
    """
    client = get_client()
    raw = await client.get(key)
    if raw is None:
        logger.debug("cache miss key=%s", key)
        return None
    logger.debug("cache hit key=%s", key)
    return json.loads(raw)


async def delete_value(key: str) -> None:
    """Delete *key* from Redis. No-op if the key does not exist."""
    client = get_client()
    await client.delete(key)
    logger.debug("deleted key=%s", key)


# ---------------------------------------------------------------------------
# Trading pipeline convenience helpers
# ---------------------------------------------------------------------------

_CANDLE_TTL = 3600  # seconds
_TICKER_TTL = 60    # seconds


def _candle_key(symbol: str, interval: str) -> str:
    return f"candle:{symbol}:{interval}:latest"


def _ticker_key(symbol: str) -> str:
    return f"ticker:{symbol}:price"


async def cache_candle(symbol: str, interval: str, candle_data: dict) -> None:
    """Cache the latest candle for *symbol* / *interval*.

    Stored under ``candle:{symbol}:{interval}:latest`` with a 3600 s TTL.

    Args:
        symbol: Trading symbol, e.g. ``"XAUUSDT"``.
        interval: Candle interval, e.g. ``"5m"``.
        candle_data: Plain dict representation of a candle.
    """
    key = _candle_key(symbol, interval)
    await set_value(key, candle_data, ttl=_CANDLE_TTL)
    logger.debug("cached candle symbol=%s interval=%s", symbol, interval)


async def get_cached_candle(symbol: str, interval: str) -> dict | None:
    """Retrieve the latest cached candle for *symbol* / *interval*.

    Returns:
        The candle dict, or ``None`` on a cache miss.
    """
    key = _candle_key(symbol, interval)
    return await get_value(key)


async def cache_ticker_price(symbol: str, price: float) -> None:
    """Cache the current ticker price for *symbol*.

    Stored under ``ticker:{symbol}:price`` with a 60 s TTL.

    Args:
        symbol: Trading symbol, e.g. ``"XAUUSDT"``.
        price: Current mid/last price as a float.
    """
    key = _ticker_key(symbol)
    await set_value(key, price, ttl=_TICKER_TTL)
    logger.debug("cached ticker price symbol=%s price=%s", symbol, price)


async def get_ticker_price(symbol: str) -> float | None:
    """Get the cached ticker price for *symbol*.

    Returns:
        The price as a float, or ``None`` on a cache miss.
    """
    key = _ticker_key(symbol)
    value = await get_value(key)
    if value is None:
        return None
    return float(value)
