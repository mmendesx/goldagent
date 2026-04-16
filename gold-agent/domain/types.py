from datetime import datetime
from enum import Enum
from typing import Optional

from pydantic import BaseModel, ConfigDict


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def _to_camel(s: str) -> str:
    """Convert snake_case field name to camelCase for JSON serialization."""
    parts = s.split("_")
    return parts[0] + "".join(w.capitalize() for w in parts[1:])


_CAMEL_CONFIG = ConfigDict(
    populate_by_name=True,
    alias_generator=_to_camel,
)


# ---------------------------------------------------------------------------
# Enums
# ---------------------------------------------------------------------------

class OrderSide(str, Enum):
    BUY = "BUY"
    SELL = "SELL"


class PositionStatus(str, Enum):
    OPEN = "open"
    CLOSED = "closed"


class CloseReason(str, Enum):
    TAKE_PROFIT = "TAKE_PROFIT"
    STOP_LOSS = "STOP_LOSS"
    TRAILING_STOP = "TRAILING_STOP"
    MANUAL = "MANUAL"
    CIRCUIT_BREAKER = "CIRCUIT_BREAKER"


class DecisionAction(str, Enum):
    BUY = "BUY"
    SELL = "SELL"
    HOLD = "HOLD"


class OrderStatus(str, Enum):
    PENDING = "pending"
    FILLED = "filled"
    PARTIALLY_FILLED = "partially_filled"
    CANCELLED = "cancelled"
    REJECTED = "rejected"
    EXPIRED = "expired"


class ExchangeBalanceStatus(str, Enum):
    OK = "ok"
    NOT_CONFIGURED = "not_configured"
    ERROR = "error"


# ---------------------------------------------------------------------------
# API-facing models (camelCase serialization)
# ---------------------------------------------------------------------------

class Candle(BaseModel):
    model_config = _CAMEL_CONFIG

    id: Optional[int] = None
    symbol: str
    interval: str
    open_time: datetime
    close_time: datetime
    open_price: str
    high_price: str
    low_price: str
    close_price: str
    volume: str
    quote_volume: str
    trade_count: int
    is_closed: bool


class Position(BaseModel):
    model_config = _CAMEL_CONFIG

    id: Optional[int] = None
    symbol: str
    # "LONG" or "SHORT" — not the same as OrderSide, intentionally a plain str
    side: str
    entry_price: str
    exit_price: Optional[str] = None
    quantity: str
    take_profit_price: Optional[str] = None
    stop_loss_price: Optional[str] = None
    trailing_stop_distance: Optional[str] = None
    trailing_stop_price: Optional[str] = None
    realized_pnl: Optional[str] = None
    fees: Optional[str] = None
    status: PositionStatus = PositionStatus.OPEN
    close_reason: Optional[CloseReason] = None
    opened_at: Optional[datetime] = None
    closed_at: Optional[datetime] = None


class PortfolioMetrics(BaseModel):
    model_config = _CAMEL_CONFIG

    balance: str
    peak_balance: str
    drawdown_percent: str
    total_pnl: str
    win_count: int
    loss_count: int
    total_trades: int
    win_rate: str
    profit_factor: str
    average_win: str
    average_loss: str
    sharpe_ratio: str
    max_drawdown_percent: str
    is_circuit_breaker_active: bool


class Decision(BaseModel):
    model_config = _CAMEL_CONFIG

    id: Optional[int] = None
    symbol: str
    action: DecisionAction
    confidence: int  # 0-100
    reasoning: Optional[str] = None
    execution_status: str = "pending"
    rejection_reason: Optional[str] = None
    # TS declares compositeScore as string; stored as Optional[str] to match contract
    composite_score: Optional[str] = None
    is_dry_run: bool = False
    created_at: Optional[datetime] = None


class TradeRecord(BaseModel):
    model_config = _CAMEL_CONFIG

    id: Optional[int] = None
    symbol: str
    side: str
    entry_price: str
    exit_price: str
    quantity: str
    realized_pnl: str
    close_reason: Optional[CloseReason] = None
    opened_at: Optional[datetime] = None
    closed_at: Optional[datetime] = None


class ExchangeBalance(BaseModel):
    model_config = _CAMEL_CONFIG

    balance: str
    status: ExchangeBalanceStatus


class ExchangeBalances(BaseModel):
    model_config = _CAMEL_CONFIG

    binance: ExchangeBalance
    polymarket: ExchangeBalance


# ---------------------------------------------------------------------------
# Internal-only models (no camelCase aliases)
# ---------------------------------------------------------------------------

class LLMDecisionResponse(BaseModel):
    """Parses raw LLM JSON output. Not persisted directly."""

    action: DecisionAction  # BUY, SELL, or HOLD — anything else raises ValidationError
    confidence: int
    reasoning: str = ""
    suggested_entry: Optional[float] = None
    suggested_take_profit: Optional[float] = None
    suggested_stop_loss: Optional[float] = None


class TradeIntent(BaseModel):
    """Carries the intent to open a position from the decision engine to the executor."""

    decision_id: Optional[int] = None
    symbol: str
    side: OrderSide
    estimated_entry_price: float
    position_size_qty: float
    suggested_take_profit: Optional[float] = None
    suggested_stop_loss: Optional[float] = None
    trailing_stop_distance: Optional[float] = None
    atr_value: Optional[float] = None
    created_at: Optional[datetime] = None


class Order(BaseModel):
    """Represents a single exchange order, either open or settled."""

    id: Optional[int] = None
    position_id: Optional[int] = None
    exchange: str  # "binance" or "polymarket"
    external_order_id: Optional[str] = None
    side: OrderSide
    symbol: str
    quantity: str
    price: Optional[str] = None
    filled_quantity: str = "0"
    filled_price: Optional[str] = None
    fee: str = "0"
    fee_asset: str = ""
    status: OrderStatus = OrderStatus.PENDING
    created_at: Optional[datetime] = None


class Indicator(BaseModel):
    """Technical indicator snapshot aligned to a candle."""

    id: Optional[int] = None
    symbol: str
    interval: str
    candle_open_time: datetime
    rsi: Optional[float] = None
    macd_line: Optional[float] = None
    macd_signal: Optional[float] = None
    macd_histogram: Optional[float] = None
    bb_upper: Optional[float] = None
    bb_middle: Optional[float] = None
    bb_lower: Optional[float] = None
    ema_9: Optional[float] = None
    ema_21: Optional[float] = None
    ema_50: Optional[float] = None
    ema_200: Optional[float] = None
    vwap: Optional[float] = None
    atr: Optional[float] = None


class PolymarketCryptoPrice(BaseModel):
    """Price snapshot from Polymarket oracle feed."""

    symbol: str
    value: float
    timestamp: datetime
