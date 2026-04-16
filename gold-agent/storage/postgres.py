"""
Asyncpg-backed repository layer for the gold-agent trading pipeline.

Schema notes (source of truth: gold-backend/migrations/):
- indicators: DB columns are `timestamp` and `bollinger_*`; mapped to Pydantic
  fields `candle_open_time` and `bb_*` in the record-to-model helpers below.
- indicators.candle_id is NOT NULL; save_indicator resolves it via subquery on
  (symbol, interval, open_time).
- orders: DB has `decision_id`, not `position_id`. Order.position_id is ignored
  on insert; treated as if it maps to decision_id when the field is set.
  TODO: align Order model to match DB once caller convention is settled.
- positions: DB column is `fee_total`; mapped to Position.fees.
- decisions: DB has no `reasoning` column (it stores signal floats instead).
  Decision.reasoning is not persisted.
  TODO: add `reasoning TEXT` column to decisions via migration if needed.
- portfolio_snapshots: DB uses `snapshot_at`; PortfolioMetrics has no timestamp
  field, so it is written as NOW() and not round-tripped on fetch.
"""

from __future__ import annotations

import logging
from datetime import datetime
from typing import Optional

import asyncpg

from gold_agent.config import settings
from gold_agent.domain.types import (
    Candle,
    Decision,
    Indicator,
    Order,
    Position,
    PortfolioMetrics,
)

logger = logging.getLogger(__name__)

# ---------------------------------------------------------------------------
# Connection pool
# ---------------------------------------------------------------------------

_pool: asyncpg.Pool | None = None


async def create_pool() -> asyncpg.Pool:
    """Create and store the global asyncpg connection pool."""
    global _pool
    logger.info(
        "creating asyncpg pool",
        extra={"dsn": settings.gold_database_url.split("@")[-1]},  # omit credentials
    )
    _pool = await asyncpg.create_pool(
        settings.gold_database_url,
        min_size=2,
        max_size=10,
    )
    logger.info("asyncpg pool ready")
    return _pool


async def close_pool() -> None:
    """Close the global connection pool gracefully."""
    global _pool
    if _pool:
        await _pool.close()
        logger.info("asyncpg pool closed")
        _pool = None


def get_pool() -> asyncpg.Pool:
    """Return the active pool or raise if not yet initialised."""
    if _pool is None:
        raise RuntimeError(
            "Database pool not initialized. Call create_pool() before using repositories."
        )
    return _pool


# ---------------------------------------------------------------------------
# Record-to-model helpers
# ---------------------------------------------------------------------------

def _record_to_candle(r: asyncpg.Record) -> Candle:
    return Candle(
        id=r["id"],
        symbol=r["symbol"],
        interval=r["interval"],
        open_time=r["open_time"],
        close_time=r["close_time"],
        open_price=str(r["open_price"]),
        high_price=str(r["high_price"]),
        low_price=str(r["low_price"]),
        close_price=str(r["close_price"]),
        volume=str(r["volume"]),
        quote_volume=str(r["quote_volume"]),
        trade_count=r["trade_count"],
        is_closed=r["is_closed"],
    )


def _record_to_indicator(r: asyncpg.Record) -> Indicator:
    def _f(val: object) -> Optional[float]:
        return float(val) if val is not None else None

    return Indicator(
        id=r["id"],
        symbol=r["symbol"],
        interval=r["interval"],
        candle_open_time=r["timestamp"],      # DB: timestamp → model: candle_open_time
        rsi=_f(r["rsi"]),
        macd_line=_f(r["macd_line"]),
        macd_signal=_f(r["macd_signal"]),
        macd_histogram=_f(r["macd_histogram"]),
        bb_upper=_f(r["bollinger_upper"]),    # DB: bollinger_upper → model: bb_upper
        bb_middle=_f(r["bollinger_middle"]),
        bb_lower=_f(r["bollinger_lower"]),
        ema_9=_f(r["ema_9"]),
        ema_21=_f(r["ema_21"]),
        ema_50=_f(r["ema_50"]),
        ema_200=_f(r["ema_200"]),
        vwap=_f(r["vwap"]),
        atr=_f(r["atr"]),
    )


def _record_to_decision(r: asyncpg.Record) -> Decision:
    return Decision(
        id=r["id"],
        symbol=r["symbol"],
        action=r["action"],
        confidence=r["confidence"],
        reasoning=None,            # not stored in DB — see module docstring
        execution_status=r["execution_status"],
        rejection_reason=r["rejection_reason"],
        composite_score=str(r["composite_score"]) if r["composite_score"] is not None else None,
        is_dry_run=r["is_dry_run"],
        created_at=r["created_at"],
    )


def _record_to_position(r: asyncpg.Record) -> Position:
    def _s(val: object) -> Optional[str]:
        return str(val) if val is not None else None

    return Position(
        id=r["id"],
        symbol=r["symbol"],
        side=r["side"],
        entry_price=str(r["entry_price"]),
        exit_price=_s(r["exit_price"]),
        quantity=str(r["quantity"]),
        take_profit_price=_s(r["take_profit_price"]),
        stop_loss_price=_s(r["stop_loss_price"]),
        trailing_stop_distance=_s(r["trailing_stop_distance"]),
        trailing_stop_price=_s(r["trailing_stop_price"]),
        realized_pnl=_s(r["realized_pnl"]),
        fees=_s(r["fee_total"]),               # DB: fee_total → model: fees
        status=r["status"],
        close_reason=r["close_reason"] if r["close_reason"] else None,
        opened_at=r["opened_at"],
        closed_at=r["closed_at"],
    )


def _record_to_order(r: asyncpg.Record) -> Order:
    def _s(val: object) -> Optional[str]:
        return str(val) if val is not None else None

    return Order(
        id=r["id"],
        position_id=None,                      # not in schema; see module docstring
        exchange=r["exchange"],
        external_order_id=r["external_order_id"],
        side=r["side"],
        symbol=r["symbol"],
        quantity=str(r["quantity"]),
        price=_s(r["price"]),
        filled_quantity=str(r["filled_quantity"]),
        filled_price=_s(r["filled_price"]),
        fee=str(r["fee"]),
        fee_asset=r["fee_asset"] or "",
        status=r["status"],
        created_at=r["created_at"],
    )


def _record_to_portfolio(r: asyncpg.Record) -> PortfolioMetrics:
    def _s(val: object) -> str:
        return str(val) if val is not None else "0"

    return PortfolioMetrics(
        balance=_s(r["balance"]),
        peak_balance=_s(r["peak_balance"]),
        drawdown_percent=_s(r["drawdown_percent"]),
        total_pnl=_s(r["total_pnl"]),
        win_count=r["win_count"],
        loss_count=r["loss_count"],
        total_trades=r["total_trades"],
        win_rate=_s(r["win_rate"]),
        profit_factor=_s(r["profit_factor"]),
        average_win=_s(r["average_win"]),
        average_loss=_s(r["average_loss"]),
        sharpe_ratio=_s(r["sharpe_ratio"]),
        max_drawdown_percent=_s(r["max_drawdown_percent"]),
        is_circuit_breaker_active=r["is_circuit_breaker_active"],
    )


# ---------------------------------------------------------------------------
# Candle repository
# ---------------------------------------------------------------------------

async def save_candle(candle: Candle) -> None:
    """Upsert a candle by the unique key (symbol, interval, open_time)."""
    query = """
        INSERT INTO candles (
            symbol, interval, open_time, close_time,
            open_price, high_price, low_price, close_price,
            volume, quote_volume, trade_count, is_closed
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
        ON CONFLICT (symbol, interval, open_time) DO UPDATE SET
            close_time   = EXCLUDED.close_time,
            open_price   = EXCLUDED.open_price,
            high_price   = EXCLUDED.high_price,
            low_price    = EXCLUDED.low_price,
            close_price  = EXCLUDED.close_price,
            volume       = EXCLUDED.volume,
            quote_volume = EXCLUDED.quote_volume,
            trade_count  = EXCLUDED.trade_count,
            is_closed    = EXCLUDED.is_closed
    """
    pool = get_pool()
    await pool.execute(
        query,
        candle.symbol,
        candle.interval,
        candle.open_time,
        candle.close_time,
        candle.open_price,
        candle.high_price,
        candle.low_price,
        candle.close_price,
        candle.volume,
        candle.quote_volume,
        candle.trade_count,
        candle.is_closed,
    )
    logger.debug(
        "candle upserted",
        extra={"symbol": candle.symbol, "interval": candle.interval, "open_time": str(candle.open_time)},
    )


async def fetch_candles(symbol: str, interval: str, limit: int = 200) -> list[Candle]:
    """Return the last `limit` candles for a symbol+interval, ordered ASC by open_time."""
    query = """
        SELECT id, symbol, interval, open_time, close_time,
               open_price, high_price, low_price, close_price,
               volume, quote_volume, trade_count, is_closed
        FROM (
            SELECT id, symbol, interval, open_time, close_time,
                   open_price, high_price, low_price, close_price,
                   volume, quote_volume, trade_count, is_closed
            FROM candles
            WHERE symbol = $1 AND interval = $2
            ORDER BY open_time DESC
            LIMIT $3
        ) sub
        ORDER BY open_time ASC
    """
    pool = get_pool()
    rows = await pool.fetch(query, symbol, interval, limit)
    return [_record_to_candle(r) for r in rows]


# ---------------------------------------------------------------------------
# Indicator repository
# ---------------------------------------------------------------------------

async def save_indicator(indicator: Indicator) -> None:
    """
    Upsert an indicator keyed on (symbol, interval, timestamp).

    The indicators table requires a candle_id FK. This is resolved at insert
    time via a correlated subquery against the candles table using
    (symbol, interval, open_time). If no matching candle exists the insert
    will fail with a foreign-key or null violation — callers must persist the
    candle first.

    There is no UNIQUE constraint on (symbol, interval, timestamp) in the
    current schema, so this performs a DELETE + INSERT within a transaction to
    achieve idempotent behaviour for the same candle_open_time.
    """
    query_delete = """
        DELETE FROM indicators
        WHERE symbol = $1 AND interval = $2 AND timestamp = $3
    """
    query_insert = """
        INSERT INTO indicators (
            candle_id, symbol, interval, timestamp,
            rsi, macd_line, macd_signal, macd_histogram,
            bollinger_upper, bollinger_middle, bollinger_lower,
            ema_9, ema_21, ema_50, ema_200, vwap, atr
        )
        VALUES (
            (SELECT id FROM candles WHERE symbol = $1 AND interval = $2 AND open_time = $3),
            $1, $2, $3,
            $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
        )
    """
    pool = get_pool()
    async with pool.acquire() as conn:
        async with conn.transaction():
            await conn.execute(
                query_delete,
                indicator.symbol,
                indicator.interval,
                indicator.candle_open_time,
            )
            await conn.execute(
                query_insert,
                indicator.symbol,
                indicator.interval,
                indicator.candle_open_time,
                indicator.rsi,
                indicator.macd_line,
                indicator.macd_signal,
                indicator.macd_histogram,
                indicator.bb_upper,
                indicator.bb_middle,
                indicator.bb_lower,
                indicator.ema_9,
                indicator.ema_21,
                indicator.ema_50,
                indicator.ema_200,
                indicator.vwap,
                indicator.atr,
            )
    logger.debug(
        "indicator saved",
        extra={"symbol": indicator.symbol, "interval": indicator.interval},
    )


async def fetch_indicator(symbol: str, interval: str, candle_open_time: datetime) -> Optional[Indicator]:
    """Fetch the indicator for a specific candle identified by its open_time."""
    query = """
        SELECT id, symbol, interval, timestamp,
               rsi, macd_line, macd_signal, macd_histogram,
               bollinger_upper, bollinger_middle, bollinger_lower,
               ema_9, ema_21, ema_50, ema_200, vwap, atr
        FROM indicators
        WHERE symbol = $1 AND interval = $2 AND timestamp = $3
        LIMIT 1
    """
    pool = get_pool()
    row = await pool.fetchrow(query, symbol, interval, candle_open_time)
    return _record_to_indicator(row) if row is not None else None


# ---------------------------------------------------------------------------
# Decision repository
# ---------------------------------------------------------------------------

async def save_decision(decision: Decision) -> int:
    """
    Persist a decision and return its generated ID.

    Note: Decision.reasoning is not persisted — the decisions table has no
    reasoning column. A migration is required to store free-text reasoning.
    """
    query = """
        INSERT INTO decisions (
            symbol, action, confidence, execution_status,
            rejection_reason, composite_score, is_dry_run
        ) VALUES ($1, $2::decision_action, $3, $4::decision_execution_status, $5, $6, $7)
        RETURNING id
    """
    pool = get_pool()
    row = await pool.fetchrow(
        query,
        decision.symbol,
        decision.action.value,
        decision.confidence,
        decision.execution_status,
        decision.rejection_reason,
        decision.composite_score,
        decision.is_dry_run,
    )
    decision_id: int = row["id"]
    logger.info(
        "decision saved",
        extra={
            "decision_id": decision_id,
            "symbol": decision.symbol,
            "action": decision.action.value,
            "confidence": decision.confidence,
        },
    )
    return decision_id


async def fetch_decisions(symbol: Optional[str] = None, limit: int = 50) -> list[Decision]:
    """Fetch recent decisions ordered by created_at DESC, optionally filtered by symbol."""
    if symbol is not None:
        query = """
            SELECT id, symbol, action, confidence, execution_status,
                   rejection_reason, composite_score, is_dry_run, created_at
            FROM decisions
            WHERE symbol = $1
            ORDER BY created_at DESC
            LIMIT $2
        """
        pool = get_pool()
        rows = await pool.fetch(query, symbol, limit)
    else:
        query = """
            SELECT id, symbol, action, confidence, execution_status,
                   rejection_reason, composite_score, is_dry_run, created_at
            FROM decisions
            ORDER BY created_at DESC
            LIMIT $1
        """
        pool = get_pool()
        rows = await pool.fetch(query, limit)
    return [_record_to_decision(r) for r in rows]


async def fetch_recent_decisions(symbol: str, limit: int = 5) -> list[Decision]:
    """Fetch the last `limit` decisions for a symbol (used to provide LLM context)."""
    query = """
        SELECT id, symbol, action, confidence, execution_status,
               rejection_reason, composite_score, is_dry_run, created_at
        FROM decisions
        WHERE symbol = $1
        ORDER BY created_at DESC
        LIMIT $2
    """
    pool = get_pool()
    rows = await pool.fetch(query, symbol, limit)
    return [_record_to_decision(r) for r in rows]


# ---------------------------------------------------------------------------
# Position repository
# ---------------------------------------------------------------------------

async def save_position(position: Position) -> int:
    """Insert a new position and return its generated ID."""
    query = """
        INSERT INTO positions (
            symbol, side, entry_price, quantity,
            take_profit_price, stop_loss_price,
            trailing_stop_distance, trailing_stop_price,
            fee_total, status
        ) VALUES (
            $1, $2::position_side, $3::numeric, $4::numeric,
            $5::numeric, $6::numeric,
            $7::numeric, $8::numeric,
            $9::numeric, $10::position_status
        )
        RETURNING id
    """
    pool = get_pool()
    row = await pool.fetchrow(
        query,
        position.symbol,
        position.side,
        position.entry_price,
        position.quantity,
        position.take_profit_price,
        position.stop_loss_price,
        position.trailing_stop_distance,
        position.trailing_stop_price,
        position.fees or "0",
        position.status.value,
    )
    position_id: int = row["id"]
    logger.info(
        "position opened",
        extra={
            "position_id": position_id,
            "symbol": position.symbol,
            "side": position.side,
            "entry_price": position.entry_price,
        },
    )
    return position_id


async def fetch_open_positions(symbol: Optional[str] = None) -> list[Position]:
    """Fetch all open positions, optionally filtered by symbol."""
    if symbol is not None:
        query = """
            SELECT id, symbol, side, entry_price, exit_price, quantity,
                   take_profit_price, stop_loss_price,
                   trailing_stop_distance, trailing_stop_price,
                   realized_pnl, fee_total,
                   status, close_reason, opened_at, closed_at
            FROM positions
            WHERE status = 'open' AND symbol = $1
            ORDER BY opened_at ASC
        """
        pool = get_pool()
        rows = await pool.fetch(query, symbol)
    else:
        query = """
            SELECT id, symbol, side, entry_price, exit_price, quantity,
                   take_profit_price, stop_loss_price,
                   trailing_stop_distance, trailing_stop_price,
                   realized_pnl, fee_total,
                   status, close_reason, opened_at, closed_at
            FROM positions
            WHERE status = 'open'
            ORDER BY opened_at ASC
        """
        pool = get_pool()
        rows = await pool.fetch(query)
    return [_record_to_position(r) for r in rows]


async def fetch_positions(
    symbol: Optional[str] = None,
    status: Optional[str] = None,
    limit: int = 50,
) -> list[Position]:
    """Fetch positions with optional symbol and status filters."""
    pool = get_pool()
    conditions: list[str] = []
    args: list[object] = []

    if symbol is not None:
        args.append(symbol)
        conditions.append(f"symbol = ${len(args)}")
    if status is not None:
        args.append(status)
        conditions.append(f"status = ${len(args)}::position_status")

    where_clause = ("WHERE " + " AND ".join(conditions)) if conditions else ""
    args.append(limit)

    query = f"""
        SELECT id, symbol, side, entry_price, exit_price, quantity,
               take_profit_price, stop_loss_price,
               trailing_stop_distance, trailing_stop_price,
               realized_pnl, fee_total,
               status, close_reason, opened_at, closed_at
        FROM positions
        {where_clause}
        ORDER BY opened_at DESC
        LIMIT ${len(args)}
    """
    rows = await pool.fetch(query, *args)
    return [_record_to_position(r) for r in rows]


async def update_position(position: Position) -> None:
    """Update mutable fields of a position by ID."""
    if position.id is None:
        raise ValueError("update_position requires a position with a non-null id")
    query = """
        UPDATE positions SET
            exit_price            = $2::numeric,
            take_profit_price     = $3::numeric,
            stop_loss_price       = $4::numeric,
            trailing_stop_distance = $5::numeric,
            trailing_stop_price   = $6::numeric,
            realized_pnl          = $7::numeric,
            fee_total             = $8::numeric,
            status                = $9::position_status,
            close_reason          = $10::position_close_reason,
            closed_at             = $11,
            updated_at            = NOW()
        WHERE id = $1
    """
    pool = get_pool()
    tag = await pool.execute(
        query,
        position.id,
        position.exit_price,
        position.take_profit_price,
        position.stop_loss_price,
        position.trailing_stop_distance,
        position.trailing_stop_price,
        position.realized_pnl,
        position.fees,
        position.status.value,
        position.close_reason.value if position.close_reason else None,
        position.closed_at,
    )
    if tag == "UPDATE 0":
        logger.warning(
            "update_position matched no rows",
            extra={"position_id": position.id},
        )


async def close_position(
    position_id: int,
    exit_price: str,
    realized_pnl: str,
    close_reason: str,
    closed_at: datetime,
) -> None:
    """Mark a position as closed, recording exit price, P&L, and reason."""
    query = """
        UPDATE positions SET
            status       = 'closed'::position_status,
            exit_price   = $2::numeric,
            realized_pnl = $3::numeric,
            close_reason = $4::position_close_reason,
            closed_at    = $5,
            updated_at   = NOW()
        WHERE id = $1
    """
    pool = get_pool()
    tag = await pool.execute(query, position_id, exit_price, realized_pnl, close_reason, closed_at)
    if tag == "UPDATE 0":
        raise ValueError(f"close_position: position with id {position_id!r} not found")
    logger.info(
        "position closed",
        extra={
            "position_id": position_id,
            "exit_price": exit_price,
            "realized_pnl": realized_pnl,
            "close_reason": close_reason,
        },
    )


async def fetch_closed_positions() -> list[Position]:
    """Fetch all closed positions for portfolio P&L calculation."""
    query = """
        SELECT id, symbol, side, entry_price, exit_price, quantity,
               take_profit_price, stop_loss_price,
               trailing_stop_distance, trailing_stop_price,
               realized_pnl, fee_total,
               status, close_reason, opened_at, closed_at
        FROM positions
        WHERE status = 'closed'
        ORDER BY closed_at DESC
    """
    pool = get_pool()
    rows = await pool.fetch(query)
    return [_record_to_position(r) for r in rows]


# ---------------------------------------------------------------------------
# Order repository
# ---------------------------------------------------------------------------

async def save_order(order: Order) -> int:
    """
    Insert an order and return its generated ID.

    The orders table has a `decision_id` column, not `position_id`. Order.position_id
    is not written to the DB.
    TODO: reconcile Order.position_id vs orders.decision_id when caller convention is settled.
    """
    query = """
        INSERT INTO orders (
            exchange, external_order_id, symbol, side,
            quantity, price, filled_quantity, filled_price,
            fee, fee_asset, status
        ) VALUES (
            $1::order_exchange, $2, $3, $4::order_side,
            $5::numeric, $6::numeric, $7::numeric, $8::numeric,
            $9::numeric, $10, $11::order_status
        )
        RETURNING id
    """
    pool = get_pool()
    row = await pool.fetchrow(
        query,
        order.exchange,
        order.external_order_id,
        order.symbol,
        order.side.value,
        order.quantity,
        order.price,
        order.filled_quantity,
        order.filled_price,
        order.fee,
        order.fee_asset or None,
        order.status.value,
    )
    order_id: int = row["id"]
    logger.info(
        "order saved",
        extra={
            "order_id": order_id,
            "symbol": order.symbol,
            "side": order.side.value,
            "exchange": order.exchange,
        },
    )
    return order_id


async def update_order(order: Order) -> None:
    """Update an order's status and fill information by ID."""
    if order.id is None:
        raise ValueError("update_order requires an order with a non-null id")
    query = """
        UPDATE orders SET
            status          = $2::order_status,
            filled_quantity = $3::numeric,
            filled_price    = $4::numeric,
            fee             = $5::numeric,
            fee_asset       = $6,
            external_order_id = $7,
            updated_at      = NOW()
        WHERE id = $1
    """
    pool = get_pool()
    tag = await pool.execute(
        query,
        order.id,
        order.status.value,
        order.filled_quantity,
        order.filled_price,
        order.fee,
        order.fee_asset or None,
        order.external_order_id,
    )
    if tag == "UPDATE 0":
        logger.warning(
            "update_order matched no rows",
            extra={"order_id": order.id},
        )


# ---------------------------------------------------------------------------
# Portfolio repository
# ---------------------------------------------------------------------------

async def save_portfolio_snapshot(metrics: PortfolioMetrics) -> None:
    """Insert a portfolio snapshot. `snapshot_at` is set to NOW() by the DB default."""
    query = """
        INSERT INTO portfolio_snapshots (
            balance, peak_balance, drawdown_percent, total_pnl,
            win_count, loss_count, total_trades, win_rate,
            profit_factor, average_win, average_loss,
            sharpe_ratio, max_drawdown_percent, is_circuit_breaker_active
        ) VALUES (
            $1::numeric, $2::numeric, $3::numeric, $4::numeric,
            $5, $6, $7, $8::numeric,
            $9::numeric, $10::numeric, $11::numeric,
            $12::numeric, $13::numeric, $14
        )
    """
    pool = get_pool()
    await pool.execute(
        query,
        metrics.balance,
        metrics.peak_balance,
        metrics.drawdown_percent,
        metrics.total_pnl,
        metrics.win_count,
        metrics.loss_count,
        metrics.total_trades,
        metrics.win_rate,
        metrics.profit_factor,
        metrics.average_win,
        metrics.average_loss,
        metrics.sharpe_ratio,
        metrics.max_drawdown_percent,
        metrics.is_circuit_breaker_active,
    )
    logger.debug(
        "portfolio snapshot saved",
        extra={"balance": metrics.balance, "circuit_breaker": metrics.is_circuit_breaker_active},
    )


async def fetch_latest_portfolio() -> Optional[PortfolioMetrics]:
    """Fetch the most recent portfolio snapshot, or None if none exists."""
    query = """
        SELECT balance, peak_balance, drawdown_percent, total_pnl,
               win_count, loss_count, total_trades, win_rate,
               profit_factor, average_win, average_loss,
               sharpe_ratio, max_drawdown_percent, is_circuit_breaker_active
        FROM portfolio_snapshots
        ORDER BY snapshot_at DESC
        LIMIT 1
    """
    pool = get_pool()
    row = await pool.fetchrow(query)
    return _record_to_portfolio(row) if row is not None else None
