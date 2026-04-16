"""
Async position monitor for the gold-agent trading pipeline.

Runs every CHECK_INTERVAL_SECONDS and evaluates every open position against
the current market price for take-profit, stop-loss, and trailing-stop
conditions. When a condition fires, a closing order is placed via the Executor
and the position record is updated to CLOSED.

Does NOT start in dry-run mode (GOLD_DRY_RUN=true).
"""

from __future__ import annotations

import asyncio
import logging
from datetime import datetime, timezone
from typing import TYPE_CHECKING

from domain.types import CloseReason, OrderSide, Position, TradeIntent
from storage import postgres, redis_client
from config import settings

if TYPE_CHECKING:
    from execution.executor import Executor

logger = logging.getLogger(__name__)

CHECK_INTERVAL_SECONDS = 5


class PositionMonitor:
    """Monitors open positions and triggers automatic exit orders.

    Args:
        executor: Configured :class:`Executor` instance used to place
                  closing orders.
    """

    def __init__(self, executor: Executor) -> None:
        self._executor = executor
        self._running = False

    async def run(self) -> None:
        """Start the monitoring loop.

        Returns immediately without starting the loop when ``GOLD_DRY_RUN``
        is ``true``. Otherwise, runs until :meth:`stop` is called.
        """
        if settings.gold_dry_run:
            logger.info("PositionMonitor disabled in dry-run mode")
            return

        self._running = True
        logger.info("PositionMonitor started, interval=%ds", CHECK_INTERVAL_SECONDS)

        while self._running:
            try:
                await self._check_all_positions()
            except Exception as exc:
                logger.error("PositionMonitor cycle failed: %s", exc, exc_info=True)
            await asyncio.sleep(CHECK_INTERVAL_SECONDS)

    async def stop(self) -> None:
        """Signal the monitoring loop to stop after its current iteration."""
        self._running = False
        logger.info("PositionMonitor stop requested")

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    async def _check_all_positions(self) -> None:
        """Fetch all open positions and evaluate each against its current price."""
        positions = await postgres.fetch_open_positions()

        for position in positions:
            try:
                current_price = await redis_client.get_ticker_price(position.symbol)
                if current_price is None:
                    logger.debug(
                        "no cached price for symbol=%s, skipping position id=%s",
                        position.symbol,
                        position.id,
                    )
                    continue
                await self._check_position(position, current_price)
            except Exception as exc:
                logger.error(
                    "error checking position id=%s symbol=%s: %s",
                    position.id,
                    position.symbol,
                    exc,
                    exc_info=True,
                )

    async def _check_position(self, position: Position, current_price: float) -> None:
        """Evaluate a single position for trailing-stop update and exit conditions."""
        is_long = position.side == "LONG"

        if position.trailing_stop_distance:
            await self._update_trailing_stop(position, current_price, is_long)

        close_reason = self._evaluate_exit(position, current_price, is_long)
        if close_reason:
            await self._close_position(position, current_price, close_reason)

    async def _update_trailing_stop(
        self,
        position: Position,
        current_price: float,
        is_long: bool,
    ) -> None:
        """Ratchet the trailing stop in the direction of a favourable price move.

        For LONG positions the stop moves up; for SHORT it moves down. The stop
        is never moved against the position.
        """
        try:
            distance = float(position.trailing_stop_distance)  # type: ignore[arg-type]
            existing = (
                float(position.trailing_stop_price)
                if position.trailing_stop_price is not None
                else None
            )

            if is_long:
                candidate = current_price - distance
                should_update = existing is None or candidate > existing
            else:
                candidate = current_price + distance
                should_update = existing is None or candidate < existing

            if should_update:
                position.trailing_stop_price = str(candidate)
                await postgres.update_position(position)
                logger.debug(
                    "trailing stop ratcheted: position_id=%s symbol=%s new_stop=%.4f",
                    position.id,
                    position.symbol,
                    candidate,
                )
        except (ValueError, TypeError) as exc:
            logger.warning(
                "could not update trailing stop for position id=%s: %s",
                position.id,
                exc,
            )

    def _evaluate_exit(
        self,
        position: Position,
        current_price: float,
        is_long: bool,
    ) -> CloseReason | None:
        """Return the exit reason if a condition is met, or ``None``.

        Priority: take_profit > stop_loss > trailing_stop.
        """
        try:
            if position.take_profit_price:
                tp = float(position.take_profit_price)
                if is_long and current_price >= tp:
                    return CloseReason.TAKE_PROFIT
                if not is_long and current_price <= tp:
                    return CloseReason.TAKE_PROFIT

            if position.stop_loss_price:
                sl = float(position.stop_loss_price)
                if is_long and current_price <= sl:
                    return CloseReason.STOP_LOSS
                if not is_long and current_price >= sl:
                    return CloseReason.STOP_LOSS

            if position.trailing_stop_price:
                ts = float(position.trailing_stop_price)
                if is_long and current_price <= ts:
                    return CloseReason.TRAILING_STOP
                if not is_long and current_price >= ts:
                    return CloseReason.TRAILING_STOP

        except (ValueError, TypeError) as exc:
            logger.warning(
                "corrupt price field on position id=%s, cannot evaluate exit: %s",
                position.id,
                exc,
            )

        return None

    async def _close_position(
        self,
        position: Position,
        current_price: float,
        close_reason: CloseReason,
    ) -> None:
        """Place a closing order and mark the position as closed in the database."""
        logger.info(
            "closing position: id=%s symbol=%s side=%s reason=%s price=%.4f",
            position.id,
            position.symbol,
            position.side,
            close_reason.value,
            current_price,
        )

        closing_side = OrderSide.SELL if position.side == "LONG" else OrderSide.BUY

        intent = TradeIntent(
            decision_id=None,
            symbol=position.symbol,
            side=closing_side,
            estimated_entry_price=current_price,
            position_size_qty=float(position.quantity),
            created_at=datetime.now(timezone.utc),
        )

        order = await self._executor.execute(intent)
        if order is None:
            logger.critical(
                "closing order returned None for position id=%s symbol=%s — "
                "the position may remain open on the exchange",
                position.id,
                position.symbol,
            )
            # Still update DB to reflect the attempted close so the next cycle
            # does not re-trigger the same exit condition.

        pnl = self._compute_pnl(position, current_price)

        await postgres.close_position(
            position_id=position.id,  # type: ignore[arg-type]
            exit_price=str(current_price),
            realized_pnl=f"{pnl:.8f}",
            close_reason=close_reason.value,
            closed_at=datetime.now(timezone.utc),
        )

        logger.info(
            "position closed in DB: id=%s realized_pnl=%.8f",
            position.id,
            pnl,
        )

    def _compute_pnl(self, position: Position, exit_price: float) -> float:
        """Compute realized P&L for a position closed at *exit_price*.

        Returns 0.0 if the position's entry_price or quantity fields are
        unparseable, logging a warning rather than raising.
        """
        try:
            entry = float(position.entry_price)
            qty = float(position.quantity)
            if position.side == "LONG":
                return (exit_price - entry) * qty
            return (entry - exit_price) * qty
        except (ValueError, TypeError) as exc:
            logger.warning(
                "could not compute PnL for position id=%s: %s",
                position.id,
                exc,
            )
            return 0.0
