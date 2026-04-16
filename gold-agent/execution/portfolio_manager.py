"""
Portfolio manager: periodically snapshots PnL metrics from all closed positions
and persists them to the database. Trips the circuit breaker when drawdown
exceeds the configured threshold.
"""

from __future__ import annotations

import asyncio
import logging

from domain.types import PortfolioMetrics
from storage import postgres
from config import settings

logger = logging.getLogger(__name__)

SNAPSHOT_INTERVAL_SECONDS = 30


class PortfolioManager:
    """Computes and persists portfolio metrics on a fixed interval.

    Args:
        initial_balance: Starting account balance used when no prior snapshot
                         exists. Defaults to 10,000 USDT.
    """

    def __init__(self, initial_balance: float = 10_000.0) -> None:
        self._initial_balance = initial_balance
        self._running = False

    async def run(self) -> None:
        """Periodic snapshot loop — takes a snapshot every 30 seconds."""
        self._running = True
        logger.info(
            "PortfolioManager started, interval=%ds", SNAPSHOT_INTERVAL_SECONDS
        )

        while self._running:
            try:
                await self.snapshot()
            except Exception as exc:
                logger.error("Portfolio snapshot error: %s", exc, exc_info=True)
            await asyncio.sleep(SNAPSHOT_INTERVAL_SECONDS)

    async def stop(self) -> None:
        """Signal the snapshot loop to stop after its current iteration."""
        self._running = False
        logger.info("PortfolioManager stop requested")

    async def snapshot(self) -> PortfolioMetrics:
        """Compute and persist current portfolio metrics.

        Calculation steps:
        1. Fetch all closed positions.
        2. Aggregate: total_pnl, win_count, loss_count, win_rate, profit_factor,
           avg_win, avg_loss.
        3. Compute current balance = initial_balance + total_pnl.
        4. Load peak_balance from the last snapshot (or use initial_balance if
           none exists), then update it if current balance is a new high.
        5. Compute drawdown = (peak - current) / peak * 100.
        6. Trip circuit breaker when drawdown >= GOLD_MAX_DRAWDOWN_PERCENT.
        7. Persist snapshot and return the metrics.

        Returns:
            The freshly-computed and persisted :class:`PortfolioMetrics`.
        """
        closed_positions = await postgres.fetch_closed_positions()

        # --- Aggregate PnL stats ---
        wins: list[float] = []
        losses: list[float] = []
        total_pnl = 0.0

        for pos in closed_positions:
            try:
                pnl = float(pos.realized_pnl or "0")
                total_pnl += pnl
                if pnl > 0:
                    wins.append(pnl)
                elif pnl < 0:
                    losses.append(abs(pnl))
            except (ValueError, TypeError):
                logger.warning(
                    "unparseable realized_pnl on position id=%s, skipping",
                    pos.id,
                )
                continue

        win_count = len(wins)
        loss_count = len(losses)
        total_trades = win_count + loss_count

        win_rate = (win_count / total_trades) if total_trades > 0 else 0.0
        avg_win = sum(wins) / len(wins) if wins else 0.0
        avg_loss = sum(losses) / len(losses) if losses else 0.0
        # profit_factor is infinite when there are wins and no losses; use 1.0
        # as a sentinel that fits the string-serialized contract.
        profit_factor = (
            sum(wins) / sum(losses)
            if losses
            else (1.0 if wins else 0.0)
        )

        # --- Balance and drawdown ---
        current_balance = self._initial_balance + total_pnl

        prev_snapshot = await postgres.fetch_latest_portfolio()
        if prev_snapshot is not None:
            peak_balance = max(float(prev_snapshot.peak_balance), current_balance)
        else:
            peak_balance = max(self._initial_balance, current_balance)

        drawdown_pct = (
            (peak_balance - current_balance) / peak_balance * 100
            if peak_balance > 0
            else 0.0
        )
        max_drawdown = max(
            float(prev_snapshot.max_drawdown_percent) if prev_snapshot else 0.0,
            drawdown_pct,
        )

        # --- Circuit breaker ---
        cb_active = drawdown_pct >= settings.gold_max_drawdown_percent

        # --- Simple Sharpe approximation (population std, not rolling) ---
        sharpe = 0.0
        if total_trades >= 5:
            all_pnls = [
                float(p.realized_pnl)
                for p in closed_positions
                if p.realized_pnl is not None
            ]
            if all_pnls:
                mean_pnl = sum(all_pnls) / len(all_pnls)
                variance = sum((x - mean_pnl) ** 2 for x in all_pnls) / len(all_pnls)
                std_pnl = variance ** 0.5
                sharpe = (mean_pnl / std_pnl) if std_pnl > 0 else 0.0

        metrics = PortfolioMetrics(
            balance=f"{current_balance:.2f}",
            peak_balance=f"{peak_balance:.2f}",
            drawdown_percent=f"{drawdown_pct:.2f}",
            total_pnl=f"{total_pnl:.2f}",
            win_count=win_count,
            loss_count=loss_count,
            total_trades=total_trades,
            win_rate=f"{win_rate:.4f}",
            profit_factor=f"{profit_factor:.4f}",
            average_win=f"{avg_win:.2f}",
            average_loss=f"{avg_loss:.2f}",
            sharpe_ratio=f"{sharpe:.4f}",
            max_drawdown_percent=f"{max_drawdown:.2f}",
            is_circuit_breaker_active=cb_active,
        )

        await postgres.save_portfolio_snapshot(metrics)

        if cb_active:
            logger.warning(
                "CIRCUIT BREAKER ACTIVE: drawdown=%.1f%% (limit=%.1f%%)",
                drawdown_pct,
                settings.gold_max_drawdown_percent,
            )

        logger.info(
            "portfolio snapshot persisted: balance=%s drawdown=%.2f%% "
            "trades=%d circuit_breaker=%s",
            metrics.balance,
            drawdown_pct,
            total_trades,
            cb_active,
        )

        return metrics
