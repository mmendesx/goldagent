"""
BDD tests for PositionMonitor — ICT-15.

Every test maps to one acceptance criterion from the spec.
External dependencies (postgres, redis_client, executor) are fully mocked so
tests run without a running database, Redis, or exchange SDK packages.

Import isolation strategy
--------------------------
``execution.position_monitor`` transitively pulls in ``storage.postgres`` and
``storage.redis_client``, which require ``asyncpg`` and ``redis`` — packages
not installed in the test environment.  We inject MagicMock stubs into
``sys.modules`` *before* importing the module under test so that the real
packages are never loaded.  The stubs are replaced by AsyncMock instances in
each test via ``patch`` calls.

Async execution
---------------
``pytest-asyncio`` is not installed in the test environment.  Async coroutines
are run via ``asyncio.run()`` inside each test method.
"""
from __future__ import annotations

import asyncio
import sys
import types
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

# ---------------------------------------------------------------------------
# Stub out storage modules before any module under test is imported.
# ---------------------------------------------------------------------------

_storage_pkg = types.ModuleType("storage")
_postgres_stub = MagicMock()
_redis_stub = MagicMock()

sys.modules.setdefault("storage", _storage_pkg)
sys.modules.setdefault("storage.postgres", _postgres_stub)
sys.modules.setdefault("storage.redis_client", _redis_stub)
sys.modules.setdefault("asyncpg", MagicMock())
sys.modules.setdefault("redis", MagicMock())
sys.modules.setdefault("redis.asyncio", MagicMock())

# Stub exchange packages that executor.py would otherwise require.
for _mod in (
    "binance",
    "binance.spot",
    "exchange",
    "exchange.binance_rest",
    "exchange.polymarket_rest",
    "py_clob_client",
):
    sys.modules.setdefault(_mod, MagicMock())

# Now safe to import the module under test and the domain types.
from domain.types import CloseReason, Order, OrderSide, OrderStatus, Position, PositionStatus  # noqa: E402
from execution.position_monitor import PositionMonitor  # noqa: E402


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------


def _run(coro):
    """Run a coroutine synchronously — replacement for pytest-asyncio."""
    return asyncio.run(coro)


def _long_position(
    *,
    id: int = 1,
    symbol: str = "BTCUSDT",
    entry_price: str = "48000",
    quantity: str = "0.1",
    take_profit_price: str | None = None,
    stop_loss_price: str | None = None,
    trailing_stop_distance: str | None = None,
    trailing_stop_price: str | None = None,
) -> Position:
    return Position(
        id=id,
        symbol=symbol,
        side="LONG",
        entry_price=entry_price,
        quantity=quantity,
        take_profit_price=take_profit_price,
        stop_loss_price=stop_loss_price,
        trailing_stop_distance=trailing_stop_distance,
        trailing_stop_price=trailing_stop_price,
        status=PositionStatus.OPEN,
    )


def _short_position(
    *,
    id: int = 2,
    symbol: str = "BTCUSDT",
    entry_price: str = "52000",
    quantity: str = "0.1",
    take_profit_price: str | None = None,
    stop_loss_price: str | None = None,
    trailing_stop_distance: str | None = None,
    trailing_stop_price: str | None = None,
) -> Position:
    return Position(
        id=id,
        symbol=symbol,
        side="SHORT",
        entry_price=entry_price,
        quantity=quantity,
        take_profit_price=take_profit_price,
        stop_loss_price=stop_loss_price,
        trailing_stop_distance=trailing_stop_distance,
        trailing_stop_price=trailing_stop_price,
        status=PositionStatus.OPEN,
    )


def _make_order() -> Order:
    return Order(
        id=99,
        exchange="binance",
        side=OrderSide.SELL,
        symbol="BTCUSDT",
        quantity="0.1",
        status=OrderStatus.FILLED,
    )


def _mock_executor(order: Order | None = None) -> MagicMock:
    """Return a mock that satisfies PositionMonitor's executor.execute() contract."""
    executor = MagicMock()
    executor.execute = AsyncMock(return_value=order)
    return executor


# ---------------------------------------------------------------------------
# Scenario 1: LONG TP hit — closing order placed, reason=TAKE_PROFIT
# ---------------------------------------------------------------------------

class TestTakeProfitLong:
    """
    Given a LONG position with take_profit_price=50000
    When current price is 50001
    Then a closing SELL order is placed and close_reason="TAKE_PROFIT"
    """

    def test_long_tp_triggers_close(self):
        position = _long_position(take_profit_price="50000")
        executor = _mock_executor(order=_make_order())
        monitor = PositionMonitor(executor)

        async def _run_check():
            with (
                patch("execution.position_monitor.postgres") as mock_pg,
                patch("execution.position_monitor.redis_client") as mock_redis,
                patch("execution.position_monitor.settings") as mock_settings,
            ):
                mock_settings.gold_dry_run = False
                mock_redis.get_ticker_price = AsyncMock(return_value=50001.0)
                mock_pg.fetch_open_positions = AsyncMock(return_value=[position])
                mock_pg.update_position = AsyncMock()
                mock_pg.close_position = AsyncMock()

                await monitor._check_all_positions()

                executor.execute.assert_awaited_once()
                intent_used = executor.execute.call_args.args[0]
                assert intent_used.side == OrderSide.SELL

                mock_pg.close_position.assert_awaited_once()
                call_kwargs = mock_pg.close_position.call_args.kwargs
                assert call_kwargs["close_reason"] == CloseReason.TAKE_PROFIT.value
                assert call_kwargs["position_id"] == position.id

        _run(_run_check())


# ---------------------------------------------------------------------------
# Scenario 2: LONG SL hit — closing order placed, reason=STOP_LOSS
# ---------------------------------------------------------------------------

class TestStopLossLong:
    """
    Given a LONG position with stop_loss_price=45000
    When current price is 44999
    Then a closing SELL order is placed and close_reason="STOP_LOSS"
    """

    def test_long_sl_triggers_close(self):
        position = _long_position(stop_loss_price="45000")
        executor = _mock_executor(order=_make_order())
        monitor = PositionMonitor(executor)

        async def _run_check():
            with (
                patch("execution.position_monitor.postgres") as mock_pg,
                patch("execution.position_monitor.redis_client") as mock_redis,
                patch("execution.position_monitor.settings") as mock_settings,
            ):
                mock_settings.gold_dry_run = False
                mock_redis.get_ticker_price = AsyncMock(return_value=44999.0)
                mock_pg.fetch_open_positions = AsyncMock(return_value=[position])
                mock_pg.update_position = AsyncMock()
                mock_pg.close_position = AsyncMock()

                await monitor._check_all_positions()

                executor.execute.assert_awaited_once()
                mock_pg.close_position.assert_awaited_once()
                call_kwargs = mock_pg.close_position.call_args.kwargs
                assert call_kwargs["close_reason"] == CloseReason.STOP_LOSS.value

        _run(_run_check())


# ---------------------------------------------------------------------------
# Scenario 3: Trailing stop moves up when LONG price rises
# ---------------------------------------------------------------------------

class TestTrailingStopUpdate:
    """
    Given a LONG position with trailing_stop_distance=1000 and
          trailing_stop_price=49000
    When current price rises to 51000 (new candidate = 51000-1000 = 50000 > 49000)
    Then trailing_stop_price is updated to 50000 in the DB
    And no closing order is placed (price > new stop)
    """

    def test_trailing_stop_ratchets_upward(self):
        position = _long_position(
            trailing_stop_distance="1000",
            trailing_stop_price="49000",
        )
        executor = _mock_executor()
        monitor = PositionMonitor(executor)

        async def _run_check():
            with (
                patch("execution.position_monitor.postgres") as mock_pg,
                patch("execution.position_monitor.redis_client") as mock_redis,
                patch("execution.position_monitor.settings") as mock_settings,
            ):
                mock_settings.gold_dry_run = False
                mock_redis.get_ticker_price = AsyncMock(return_value=51000.0)
                mock_pg.fetch_open_positions = AsyncMock(return_value=[position])
                mock_pg.update_position = AsyncMock()
                mock_pg.close_position = AsyncMock()

                await monitor._check_all_positions()

                mock_pg.update_position.assert_awaited_once()
                saved_position = mock_pg.update_position.call_args.args[0]
                assert float(saved_position.trailing_stop_price) == pytest.approx(50000.0)

                # Price (51000) is above new stop (50000), so no closing order
                executor.execute.assert_not_awaited()
                mock_pg.close_position.assert_not_awaited()

        _run(_run_check())

    def test_trailing_stop_not_moved_when_price_falls(self):
        """
        Trailing stop must never move against the position direction.
        When price falls below the current stop reference, the stop stays
        in place (and no exit fires because price is above the stop).
        """
        position = _long_position(
            entry_price="52000",
            trailing_stop_distance="1000",
            trailing_stop_price="49000",
        )
        executor = _mock_executor(order=_make_order())
        monitor = PositionMonitor(executor)

        async def _run_check():
            with (
                patch("execution.position_monitor.postgres") as mock_pg,
                patch("execution.position_monitor.redis_client") as mock_redis,
                patch("execution.position_monitor.settings") as mock_settings,
            ):
                mock_settings.gold_dry_run = False
                # price=49500 → candidate=49500-1000=48500 < 49000 → no stop update
                # price=49500 > current stop=49000 → no exit yet
                mock_redis.get_ticker_price = AsyncMock(return_value=49500.0)
                mock_pg.fetch_open_positions = AsyncMock(return_value=[position])
                mock_pg.update_position = AsyncMock()
                mock_pg.close_position = AsyncMock()

                await monitor._check_all_positions()

                mock_pg.update_position.assert_not_awaited()
                executor.execute.assert_not_awaited()
                mock_pg.close_position.assert_not_awaited()

        _run(_run_check())


# ---------------------------------------------------------------------------
# Scenario 4: Dry-run — run() returns immediately
# ---------------------------------------------------------------------------

class TestDryRunGuard:
    """
    Given GOLD_DRY_RUN=true
    When PositionMonitor.run() is called
    Then the method returns immediately without starting the loop
    """

    def test_dry_run_exits_immediately(self):
        executor = _mock_executor()
        monitor = PositionMonitor(executor)

        async def _run_check():
            with patch("execution.position_monitor.settings") as mock_settings:
                mock_settings.gold_dry_run = True
                await monitor.run()

            assert monitor._running is False
            executor.execute.assert_not_awaited()

        _run(_run_check())


# ---------------------------------------------------------------------------
# Additional edge-case coverage
# ---------------------------------------------------------------------------

class TestEdgeCases:
    """Coverage for boundary conditions and error paths."""

    def test_no_price_in_cache_skips_position(self):
        """A cache miss should not trigger any action on the position."""
        position = _long_position(take_profit_price="50000")
        executor = _mock_executor()
        monitor = PositionMonitor(executor)

        async def _run_check():
            with (
                patch("execution.position_monitor.postgres") as mock_pg,
                patch("execution.position_monitor.redis_client") as mock_redis,
                patch("execution.position_monitor.settings") as mock_settings,
            ):
                mock_settings.gold_dry_run = False
                mock_redis.get_ticker_price = AsyncMock(return_value=None)
                mock_pg.fetch_open_positions = AsyncMock(return_value=[position])
                mock_pg.update_position = AsyncMock()
                mock_pg.close_position = AsyncMock()

                await monitor._check_all_positions()

                executor.execute.assert_not_awaited()
                mock_pg.close_position.assert_not_awaited()

        _run(_run_check())

    def test_short_tp_triggers_buy_close(self):
        """SHORT position TP fires a BUY closing order."""
        position = _short_position(take_profit_price="48000")
        executor = _mock_executor(order=_make_order())
        monitor = PositionMonitor(executor)

        async def _run_check():
            with (
                patch("execution.position_monitor.postgres") as mock_pg,
                patch("execution.position_monitor.redis_client") as mock_redis,
                patch("execution.position_monitor.settings") as mock_settings,
            ):
                mock_settings.gold_dry_run = False
                mock_redis.get_ticker_price = AsyncMock(return_value=47999.0)
                mock_pg.fetch_open_positions = AsyncMock(return_value=[position])
                mock_pg.update_position = AsyncMock()
                mock_pg.close_position = AsyncMock()

                await monitor._check_all_positions()

                executor.execute.assert_awaited_once()
                intent_used = executor.execute.call_args.args[0]
                assert intent_used.side == OrderSide.BUY

                call_kwargs = mock_pg.close_position.call_args.kwargs
                assert call_kwargs["close_reason"] == CloseReason.TAKE_PROFIT.value

        _run(_run_check())

    def test_pnl_computed_correctly_for_long(self):
        """Realized PnL = (exit - entry) * qty for LONG positions."""
        position = _long_position(
            entry_price="48000",
            quantity="1.0",
            stop_loss_price="44000",
        )
        executor = _mock_executor(order=_make_order())
        monitor = PositionMonitor(executor)

        async def _run_check():
            with (
                patch("execution.position_monitor.postgres") as mock_pg,
                patch("execution.position_monitor.redis_client") as mock_redis,
                patch("execution.position_monitor.settings") as mock_settings,
            ):
                mock_settings.gold_dry_run = False
                mock_redis.get_ticker_price = AsyncMock(return_value=43999.0)
                mock_pg.fetch_open_positions = AsyncMock(return_value=[position])
                mock_pg.update_position = AsyncMock()
                mock_pg.close_position = AsyncMock()

                await monitor._check_all_positions()

                call_kwargs = mock_pg.close_position.call_args.kwargs
                # (43999 - 48000) * 1.0 = -4001
                assert float(call_kwargs["realized_pnl"]) == pytest.approx(-4001.0)

        _run(_run_check())

    def test_take_profit_priority_over_stop_loss(self):
        """TP is evaluated before SL — TP wins when both could fire."""
        # price at 50001 is above TP=50000 AND above a stale SL value
        position = _long_position(
            take_profit_price="50000",
            stop_loss_price="50500",  # misconfigured, but TP fires first
        )
        executor = _mock_executor(order=_make_order())
        monitor = PositionMonitor(executor)

        async def _run_check():
            with (
                patch("execution.position_monitor.postgres") as mock_pg,
                patch("execution.position_monitor.redis_client") as mock_redis,
                patch("execution.position_monitor.settings") as mock_settings,
            ):
                mock_settings.gold_dry_run = False
                mock_redis.get_ticker_price = AsyncMock(return_value=50001.0)
                mock_pg.fetch_open_positions = AsyncMock(return_value=[position])
                mock_pg.update_position = AsyncMock()
                mock_pg.close_position = AsyncMock()

                await monitor._check_all_positions()

                call_kwargs = mock_pg.close_position.call_args.kwargs
                assert call_kwargs["close_reason"] == CloseReason.TAKE_PROFIT.value

        _run(_run_check())

    def test_failed_closing_order_still_marks_position_closed(self):
        """If executor returns None, the DB record is still updated to prevent re-triggering."""
        position = _long_position(stop_loss_price="45000")
        executor = _mock_executor(order=None)  # simulates exchange failure
        monitor = PositionMonitor(executor)

        async def _run_check():
            with (
                patch("execution.position_monitor.postgres") as mock_pg,
                patch("execution.position_monitor.redis_client") as mock_redis,
                patch("execution.position_monitor.settings") as mock_settings,
            ):
                mock_settings.gold_dry_run = False
                mock_redis.get_ticker_price = AsyncMock(return_value=44999.0)
                mock_pg.fetch_open_positions = AsyncMock(return_value=[position])
                mock_pg.update_position = AsyncMock()
                mock_pg.close_position = AsyncMock()

                await monitor._check_all_positions()

                # The position must be marked closed regardless of order success
                mock_pg.close_position.assert_awaited_once()

        _run(_run_check())
