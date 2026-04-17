"""
LLM context builder for the gold-agent decision engine.

Assembles a JSON-serializable payload from recent candles, computed indicators,
open positions, portfolio metrics, recent decisions, and an optional Polymarket
snapshot. This dict is the user message content sent to Claude.
"""

import asyncio
import logging
from datetime import datetime

from config import settings
from domain.types import Candle
from storage import postgres

logger = logging.getLogger(__name__)


async def build_context(
    symbol: str,
    interval: str,
    candle: Candle,
    polymarket_cache: dict[str, dict] | None = None,
    # ^ in-memory dict: symbol -> {"value": float, "timestamp": str}
) -> dict:
    """
    Assemble the LLM context payload for a closed candle.

    All five DB fetches run concurrently via asyncio.gather.

    Returns a JSON-serializable dict (no datetime objects, no Pydantic models —
    only Python primitives). Shape:

        {
            "symbol": str,
            "interval": str,
            "timestamp": str,          # ISO-8601 close_time of the triggering candle
            "current_price": str,
            "candles": [...],          # last N candles as compact OHLCV dicts
            "indicators": {...} | null,
            "open_positions": [...],
            "portfolio": {
                "balance": str,
                "drawdown_percent": str,
                "open_position_count": int,
                "is_circuit_breaker_active": bool,
            },
            "recent_decisions": [...],
            "polymarket_snapshot": {...} | null,
        }
    """
    limit = settings.gold_llm_context_candles

    logger.debug(
        "building llm context",
        extra={
            "symbol": symbol,
            "interval": interval,
            "candle_open_time": str(candle.open_time),
            "candle_limit": limit,
        },
    )

    candles, indicator, open_positions, portfolio, recent_decisions = await asyncio.gather(
        postgres.fetch_candles(symbol, interval, limit=limit),
        postgres.fetch_indicator(symbol, interval, candle.open_time),
        postgres.fetch_open_positions(symbol=symbol),
        postgres.fetch_latest_portfolio(),
        postgres.fetch_recent_decisions(symbol=symbol, limit=5),
    )

    # Compact OHLCV list — short keys to stay within LLM token budget
    candles_data = [
        {
            "t": c.open_time.isoformat(),
            "o": c.open_price,
            "h": c.high_price,
            "l": c.low_price,
            "c": c.close_price,
            "v": c.volume,
        }
        for c in candles
    ]

    # Indicators — None when not yet computed for this candle
    indicators_data = None
    if indicator is not None:
        indicators_data = {
            "rsi": indicator.rsi,
            "macd_line": indicator.macd_line,
            "macd_signal": indicator.macd_signal,
            "macd_histogram": indicator.macd_histogram,
            "bb_upper": indicator.bb_upper,
            "bb_middle": indicator.bb_middle,
            "bb_lower": indicator.bb_lower,
            "ema_9": indicator.ema_9,
            "ema_21": indicator.ema_21,
            "ema_50": indicator.ema_50,
            "ema_200": indicator.ema_200,
            "vwap": indicator.vwap,
            "atr": indicator.atr,
        }

    # Open positions with estimated unrealized P&L at the current close price
    positions_data = [
        {
            "side": p.side,
            "entry_price": p.entry_price,
            "quantity": p.quantity,
            "take_profit_price": p.take_profit_price,
            "stop_loss_price": p.stop_loss_price,
            "unrealized_pnl": _estimate_unrealized_pnl(p, candle.close_price),
        }
        for p in open_positions
    ]

    # Portfolio summary — fall back to safe defaults when no snapshot exists yet
    if portfolio is not None:
        portfolio_data = {
            "balance": portfolio.balance,
            "drawdown_percent": portfolio.drawdown_percent,
            "open_position_count": len(open_positions),
            "is_circuit_breaker_active": portfolio.is_circuit_breaker_active,
        }
    else:
        logger.warning(
            "no portfolio snapshot found; using fallback defaults",
            extra={"symbol": symbol},
        )
        portfolio_data = {
            "balance": "10000",
            "drawdown_percent": "0",
            "open_position_count": len(open_positions),
            "is_circuit_breaker_active": False,
        }

    # Recent decisions — DecisionAction is a str enum, .value is always a plain str
    decisions_data = [
        {
            "action": d.action.value if hasattr(d.action, "value") else d.action,
            "confidence": d.confidence,
            "reasoning": d.reasoning,
            "created_at": d.created_at.isoformat() if d.created_at is not None else None,
        }
        for d in recent_decisions
    ]

    # Polymarket snapshot from the in-memory cache (already primitive types)
    polymarket_data = None
    if polymarket_cache is not None and symbol in polymarket_cache:
        polymarket_data = polymarket_cache[symbol]

    logger.debug(
        "llm context assembled",
        extra={
            "symbol": symbol,
            "candle_count": len(candles_data),
            "has_indicators": indicators_data is not None,
            "open_position_count": len(positions_data),
            "has_polymarket": polymarket_data is not None,
        },
    )

    return {
        "symbol": symbol,
        "interval": interval,
        "timestamp": candle.close_time.isoformat(),
        "current_price": candle.close_price,
        "candles": candles_data,
        "indicators": indicators_data,
        "open_positions": positions_data,
        "portfolio": portfolio_data,
        "recent_decisions": decisions_data,
        "polymarket_snapshot": polymarket_data,
    }


def _estimate_unrealized_pnl(position, current_price_str: str) -> str:
    """
    Rough unrealized P&L estimate for LLM context only.

    Not used for any trading decision or accounting. Computed as:
        LONG:  (current - entry) * qty
        SHORT: (entry - current) * qty
    """
    try:
        entry = float(position.entry_price)
        current = float(current_price_str)
        qty = float(position.quantity)
        if position.side == "LONG":
            pnl = (current - entry) * qty
        else:
            pnl = (entry - current) * qty
        return f"{pnl:.2f}"
    except Exception:
        # Non-fatal: malformed price strings should not prevent context assembly
        logger.warning(
            "could not estimate unrealized pnl",
            extra={
                "entry_price": getattr(position, "entry_price", None),
                "current_price": current_price_str,
                "side": getattr(position, "side", None),
            },
        )
        return "0"
