from dataclasses import dataclass

from domain.types import Decision, DecisionAction, PortfolioMetrics
from config import settings


@dataclass
class RiskCheckResult:
    is_allowed: bool
    rejection_reason: str | None = None


class RiskGate:
    """Rule-based risk gating applied after LLM decision."""

    def check(
        self,
        decision: Decision,
        portfolio: PortfolioMetrics | None,
        open_position_count: int,
    ) -> RiskCheckResult:
        """
        Apply risk gates to a decision.

        Gates (in order):
        1. HOLD always passes — no further checks
        2. Confidence must exceed GOLD_CONFIDENCE_THRESHOLD
        3. Open positions must be below GOLD_MAX_POSITIONS (BUY only — SELL can close)
        4. Drawdown must be below GOLD_MAX_DRAWDOWN_PERCENT, or circuit breaker active

        Returns RiskCheckResult(is_allowed=True) if all gates pass,
        or RiskCheckResult(is_allowed=False, rejection_reason="...") on first failure.
        """
        # Normalise action to its string value for comparisons
        action = decision.action
        if isinstance(action, DecisionAction):
            action = action.value

        # Gate 0: HOLD always passes
        if action == "HOLD":
            return RiskCheckResult(is_allowed=True)

        # Gate 1: Confidence threshold
        if decision.confidence < settings.gold_confidence_threshold:
            return RiskCheckResult(
                is_allowed=False,
                rejection_reason="LOW_CONFIDENCE",
            )

        # Gate 2: Max open positions (BUY only — SELL closes an existing position)
        if action == "BUY" and open_position_count >= settings.gold_max_positions:
            return RiskCheckResult(
                is_allowed=False,
                rejection_reason="MAX_POSITIONS_REACHED",
            )

        # Gate 3: Drawdown / circuit breaker
        if portfolio is not None:
            if portfolio.is_circuit_breaker_active:
                return RiskCheckResult(
                    is_allowed=False,
                    rejection_reason="MAX_DRAWDOWN_REACHED",
                )

            try:
                drawdown = float(portfolio.drawdown_percent)
            except (ValueError, TypeError):
                drawdown = 0.0

            if drawdown >= settings.gold_max_drawdown_percent:
                return RiskCheckResult(
                    is_allowed=False,
                    rejection_reason="MAX_DRAWDOWN_REACHED",
                )

        return RiskCheckResult(is_allowed=True)
