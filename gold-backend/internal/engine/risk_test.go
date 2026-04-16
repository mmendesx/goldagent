package engine

import (
	"testing"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

func defaultRiskConfig() RiskConfig {
	return RiskConfig{
		ConfidenceThreshold: 70,
		MaxOpenPositions:    3,
		MaxDrawdownPercent:  decimal.NewFromFloat(15.0),
	}
}

func TestApplyRiskGates_HoldIsNeverAllowed(t *testing.T) {
	result := ApplyRiskGates(domain.DecisionActionHold, 90, 0, decimal.Zero, defaultRiskConfig())
	if result.IsAllowed {
		t.Error("HOLD action should never be allowed as a trade")
	}
	if result.RejectionReason != "" {
		t.Errorf("HOLD should have no rejection reason, got %q", result.RejectionReason)
	}
}

func TestApplyRiskGates_BelowConfidenceThreshold(t *testing.T) {
	// confidence 65 < threshold 70
	result := ApplyRiskGates(domain.DecisionActionBuy, 65, 0, decimal.Zero, defaultRiskConfig())
	if result.IsAllowed {
		t.Error("below confidence threshold should not be allowed")
	}
	if result.RejectionReason != string(domain.DecisionExecutionStatusBelowConfidenceThreshold) {
		t.Errorf("expected rejection reason %q, got %q",
			domain.DecisionExecutionStatusBelowConfidenceThreshold, result.RejectionReason)
	}
}

func TestApplyRiskGates_AtConfidenceThreshold_Passes(t *testing.T) {
	// confidence exactly at threshold (70) should pass
	result := ApplyRiskGates(domain.DecisionActionBuy, 70, 0, decimal.Zero, defaultRiskConfig())
	if !result.IsAllowed {
		t.Errorf("confidence at threshold should pass, rejection reason: %q", result.RejectionReason)
	}
}

func TestApplyRiskGates_MaxPositionsReached(t *testing.T) {
	// 3 open positions with max 3 → blocked
	result := ApplyRiskGates(domain.DecisionActionBuy, 80, 3, decimal.Zero, defaultRiskConfig())
	if result.IsAllowed {
		t.Error("max positions reached should not be allowed")
	}
	if result.RejectionReason != string(domain.DecisionExecutionStatusMaxPositionsReached) {
		t.Errorf("expected rejection reason %q, got %q",
			domain.DecisionExecutionStatusMaxPositionsReached, result.RejectionReason)
	}
}

func TestApplyRiskGates_BelowMaxPositions_Passes(t *testing.T) {
	// 2 open positions with max 3 → allowed (if other checks pass)
	result := ApplyRiskGates(domain.DecisionActionBuy, 80, 2, decimal.Zero, defaultRiskConfig())
	if !result.IsAllowed {
		t.Errorf("below max positions should pass, rejection reason: %q", result.RejectionReason)
	}
}

func TestApplyRiskGates_CircuitBreakerActive(t *testing.T) {
	// drawdown at max (15%) → blocked
	result := ApplyRiskGates(domain.DecisionActionBuy, 80, 0, decimal.NewFromFloat(15.0), defaultRiskConfig())
	if result.IsAllowed {
		t.Error("drawdown at max should not be allowed")
	}
	if result.RejectionReason != string(domain.DecisionExecutionStatusCircuitBreakerActive) {
		t.Errorf("expected rejection reason %q, got %q",
			domain.DecisionExecutionStatusCircuitBreakerActive, result.RejectionReason)
	}
}

func TestApplyRiskGates_CircuitBreakerExceeded(t *testing.T) {
	// drawdown beyond max → blocked
	result := ApplyRiskGates(domain.DecisionActionBuy, 80, 0, decimal.NewFromFloat(20.0), defaultRiskConfig())
	if result.IsAllowed {
		t.Error("drawdown beyond max should not be allowed")
	}
	if result.RejectionReason != string(domain.DecisionExecutionStatusCircuitBreakerActive) {
		t.Errorf("expected rejection reason %q, got %q",
			domain.DecisionExecutionStatusCircuitBreakerActive, result.RejectionReason)
	}
}

func TestApplyRiskGates_BelowDrawdownMax_Passes(t *testing.T) {
	result := ApplyRiskGates(domain.DecisionActionBuy, 80, 0, decimal.NewFromFloat(14.9), defaultRiskConfig())
	if !result.IsAllowed {
		t.Errorf("drawdown below max should pass, rejection reason: %q", result.RejectionReason)
	}
}

func TestApplyRiskGates_AllConditionsMet_IsAllowed(t *testing.T) {
	result := ApplyRiskGates(domain.DecisionActionBuy, 80, 1, decimal.NewFromFloat(5.0), defaultRiskConfig())
	if !result.IsAllowed {
		t.Errorf("all conditions met should allow trade, rejection reason: %q", result.RejectionReason)
	}
	if result.RejectionReason != "" {
		t.Errorf("allowed trade should have no rejection reason, got %q", result.RejectionReason)
	}
}

func TestApplyRiskGates_ConfidenceCheckedBeforePositions(t *testing.T) {
	// Both confidence low AND max positions exceeded — should return confidence reason
	result := ApplyRiskGates(domain.DecisionActionBuy, 50, 5, decimal.Zero, defaultRiskConfig())
	if result.IsAllowed {
		t.Error("should not be allowed")
	}
	if result.RejectionReason != string(domain.DecisionExecutionStatusBelowConfidenceThreshold) {
		t.Errorf("confidence check should precede positions check; got reason %q", result.RejectionReason)
	}
}

func TestApplyRiskGates_SellAction_AllConditionsMet(t *testing.T) {
	result := ApplyRiskGates(domain.DecisionActionSell, 75, 0, decimal.Zero, defaultRiskConfig())
	if !result.IsAllowed {
		t.Errorf("SELL with all conditions met should be allowed, rejection reason: %q", result.RejectionReason)
	}
}
