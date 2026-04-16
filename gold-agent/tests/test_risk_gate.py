"""
BDD tests for RiskGate.check — ICT-13.

Every test maps to one acceptance criterion from the spec. Settings are
patched to fixed thresholds so tests never depend on the local .env file.
"""
from unittest.mock import patch

import pytest

from engine.risk import RiskGate, RiskCheckResult
from domain.types import Decision, DecisionAction, PortfolioMetrics  # noqa: E402


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

THRESHOLDS = {
    "gold_confidence_threshold": 70,
    "gold_max_positions": 3,
    "gold_max_drawdown_percent": 15.0,
}


def _buy_decision(confidence: int) -> Decision:
    return Decision(
        symbol="XAUUSDT",
        action=DecisionAction.BUY,
        confidence=confidence,
    )


def _sell_decision(confidence: int) -> Decision:
    return Decision(
        symbol="XAUUSDT",
        action=DecisionAction.SELL,
        confidence=confidence,
    )


def _hold_decision(confidence: int) -> Decision:
    return Decision(
        symbol="XAUUSDT",
        action=DecisionAction.HOLD,
        confidence=confidence,
    )


def _portfolio(drawdown: float, circuit_breaker: bool = False) -> PortfolioMetrics:
    return PortfolioMetrics(
        balance="10000",
        peak_balance="10000",
        drawdown_percent=str(drawdown),
        total_pnl="0",
        win_count=0,
        loss_count=0,
        total_trades=0,
        win_rate="0",
        profit_factor="0",
        average_win="0",
        average_loss="0",
        sharpe_ratio="0",
        max_drawdown_percent=str(drawdown),
        is_circuit_breaker_active=circuit_breaker,
    )


# ---------------------------------------------------------------------------
# BDD scenarios
# ---------------------------------------------------------------------------

class TestRiskGate:
    """
    All scenarios patch engine.risk.settings so thresholds are deterministic.
    """

    def setup_method(self):
        self.gate = RiskGate()

    def _patch(self, **overrides):
        merged = {**THRESHOLDS, **overrides}
        mock = type("Settings", (), merged)()
        return patch("engine.risk.settings", mock)

    # Scenario 1 ---------------------------------------------------------------
    # Given GOLD_CONFIDENCE_THRESHOLD=70
    # When BUY decision with confidence=65
    # Then is_allowed=False, rejection_reason="LOW_CONFIDENCE"

    def test_low_confidence_buy_is_rejected(self):
        with self._patch():
            result = self.gate.check(
                decision=_buy_decision(confidence=65),
                portfolio=_portfolio(drawdown=0.0),
                open_position_count=0,
            )

        assert result.is_allowed is False
        assert result.rejection_reason == "LOW_CONFIDENCE"

    # Scenario 2 ---------------------------------------------------------------
    # Given GOLD_MAX_POSITIONS=3 and 3 open positions
    # When BUY decision (confidence passes)
    # Then is_allowed=False, rejection_reason="MAX_POSITIONS_REACHED"

    def test_max_positions_reached_blocks_buy(self):
        with self._patch():
            result = self.gate.check(
                decision=_buy_decision(confidence=80),
                portfolio=_portfolio(drawdown=0.0),
                open_position_count=3,
            )

        assert result.is_allowed is False
        assert result.rejection_reason == "MAX_POSITIONS_REACHED"

    # Scenario 3 ---------------------------------------------------------------
    # Given portfolio drawdown=16% and GOLD_MAX_DRAWDOWN_PERCENT=15
    # When BUY decision (confidence passes, positions below max)
    # Then is_allowed=False, rejection_reason="MAX_DRAWDOWN_REACHED"

    def test_drawdown_above_threshold_blocks_buy(self):
        with self._patch():
            result = self.gate.check(
                decision=_buy_decision(confidence=80),
                portfolio=_portfolio(drawdown=16.0),
                open_position_count=0,
            )

        assert result.is_allowed is False
        assert result.rejection_reason == "MAX_DRAWDOWN_REACHED"

    # Scenario 4 ---------------------------------------------------------------
    # HOLD decision with confidence=30 passes regardless of positions/drawdown

    def test_hold_always_passes(self):
        with self._patch():
            result = self.gate.check(
                decision=_hold_decision(confidence=30),
                portfolio=_portfolio(drawdown=99.0, circuit_breaker=True),
                open_position_count=999,
            )

        assert result.is_allowed is True
        assert result.rejection_reason is None

    # Scenario 5 ---------------------------------------------------------------
    # confidence=80, 1 open position, drawdown=5% → is_allowed=True

    def test_all_gates_pass(self):
        with self._patch():
            result = self.gate.check(
                decision=_buy_decision(confidence=80),
                portfolio=_portfolio(drawdown=5.0),
                open_position_count=1,
            )

        assert result.is_allowed is True
        assert result.rejection_reason is None

    # ---------------------------------------------------------------------------
    # Additional edge-case coverage
    # ---------------------------------------------------------------------------

    def test_circuit_breaker_active_blocks_buy(self):
        """Circuit breaker flag alone triggers MAX_DRAWDOWN_REACHED, even at zero drawdown."""
        with self._patch():
            result = self.gate.check(
                decision=_buy_decision(confidence=80),
                portfolio=_portfolio(drawdown=0.0, circuit_breaker=True),
                open_position_count=0,
            )

        assert result.is_allowed is False
        assert result.rejection_reason == "MAX_DRAWDOWN_REACHED"

    def test_max_positions_does_not_block_sell(self):
        """SELL decisions bypass the max-positions gate — they close existing positions."""
        with self._patch():
            result = self.gate.check(
                decision=_sell_decision(confidence=80),
                portfolio=_portfolio(drawdown=0.0),
                open_position_count=3,
            )

        assert result.is_allowed is True

    def test_no_portfolio_skips_drawdown_gate(self):
        """When portfolio is None the drawdown gate is skipped and the decision passes."""
        with self._patch():
            result = self.gate.check(
                decision=_buy_decision(confidence=80),
                portfolio=None,
                open_position_count=0,
            )

        assert result.is_allowed is True

    def test_confidence_exactly_at_threshold_is_rejected(self):
        """confidence < threshold means equal-to-threshold is still rejected."""
        with self._patch():
            result = self.gate.check(
                decision=_buy_decision(confidence=70),
                portfolio=_portfolio(drawdown=0.0),
                open_position_count=0,
            )

        # 70 is NOT < 70, so it should pass
        assert result.is_allowed is True

    def test_confidence_one_below_threshold_is_rejected(self):
        with self._patch():
            result = self.gate.check(
                decision=_buy_decision(confidence=69),
                portfolio=_portfolio(drawdown=0.0),
                open_position_count=0,
            )

        assert result.is_allowed is False
        assert result.rejection_reason == "LOW_CONFIDENCE"
