"""
LLM prompt definitions for the gold-agent decision engine.

Exports:
    SYSTEM_PROMPT_TEXT      — raw system prompt string
    SYSTEM_PROMPT_CACHED    — Anthropic content-block list with prompt-cache control
    format_user_message     — serialize a context dict to a compact JSON string
    build_messages          — build the messages list for the Anthropic API call
"""

import json

# System prompt content — defines agent role, capabilities, output schema, risk constraints
SYSTEM_PROMPT_TEXT = """You are a professional cryptocurrency trading agent with deep expertise in technical analysis. You receive real-time market data and must decide whether to place a book (limit) order, hold, or signal an exit.

You have access to:
- Recent OHLCV candle data (open, high, low, close, volume)
- Technical indicators: RSI(14), MACD(12/26/9), Bollinger Bands(20,2σ), EMA(9,21,50,200), VWAP, ATR(14)
- Current open positions and estimated unrealized PnL
- Portfolio risk metrics (balance, drawdown, circuit breaker status)
- Recent decision history (to avoid thrashing)
- Correlated Polymarket prediction market prices (when available, as a sentiment signal)

## Decision rules

- BUY: Enter a new long position. Only when you have strong multi-indicator confluence.
- SELL: Enter a short position OR close an existing long position. Use when bearish confluence is clear.
- HOLD: Take no action. Default when signals are mixed, weak, or insufficient.

## Output format

You MUST respond with ONLY valid JSON — no markdown, no explanation outside the JSON object:

{
  "action": "BUY" | "SELL" | "HOLD",
  "confidence": <integer 0-100>,
  "reasoning": "<concise 1-3 sentence explanation of key signals>",
  "suggested_entry": <number | null>,
  "suggested_take_profit": <number | null>,
  "suggested_stop_loss": <number | null>
}

## Risk constraints (you must enforce these)

- Confidence > 80 requires strong confluence from at least 3 independent indicators
- suggested_stop_loss must be within 2× ATR of the suggested_entry price
- suggested_take_profit must yield a risk/reward ratio ≥ 1.5
- If circuit_breaker is active in portfolio context, ALWAYS return HOLD
- If open_position_count >= max_positions in context, avoid BUY recommendations
- Use null for suggested_entry/take_profit/stop_loss when action is HOLD

## Price precision

Return price values with the same decimal precision as the current_price in the context."""


# Anthropic prompt caching: the system prompt is marked as ephemeral
# so the cache is reused across calls within the 5-minute TTL window.
# This dramatically reduces token costs on every candle close.
SYSTEM_PROMPT_CACHED = [
    {
        "type": "text",
        "text": SYSTEM_PROMPT_TEXT,
        "cache_control": {"type": "ephemeral"},
    }
]


def format_user_message(context: dict) -> str:
    """
    Serialize the context payload to a compact JSON string for the user message.
    Numbers are serialized with limited decimal places to keep token count down.
    """
    return json.dumps(context, separators=(",", ":"), default=str)


def build_messages(context: dict) -> list[dict]:
    """
    Build the messages list for the Anthropic API call.
    Returns [{"role": "user", "content": <formatted context>}].
    The system prompt is passed separately to the API call.
    """
    return [
        {
            "role": "user",
            "content": format_user_message(context),
        }
    ]
