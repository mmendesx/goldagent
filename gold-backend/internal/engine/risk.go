package engine

import (
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// RiskConfig holds the thresholds used by ApplyRiskGates.
type RiskConfig struct {
	ConfidenceThreshold int
	MaxOpenPositions    int
	MaxDrawdownPercent  decimal.Decimal
}

// RiskCheckResult is the outcome of pre-trade risk checks.
type RiskCheckResult struct {
	IsAllowed       bool
	RejectionReason string // empty if allowed; one of the DecisionExecutionStatus string values
}

// ApplyRiskGates checks whether a proposed trade should be allowed.
//
// Rejection order:
//  1. HOLD action — not an error, just no trade (no rejection reason stored)
//  2. Confidence below threshold → "below_confidence_threshold"
//  3. Open positions at or beyond maximum → "max_positions_reached"
//  4. Current drawdown at or beyond maximum → "circuit_breaker_active"
//
// Only one rejection reason is recorded — the first check that fails.
func ApplyRiskGates(
	action domain.DecisionAction,
	confidence int,
	openPositionCount int,
	currentDrawdownPercent decimal.Decimal,
	config RiskConfig,
) RiskCheckResult {
	// HOLD is never a trade — no rejection reason, just not actionable.
	if action == domain.DecisionActionHold {
		return RiskCheckResult{IsAllowed: false, RejectionReason: ""}
	}

	if confidence < config.ConfidenceThreshold {
		return RiskCheckResult{
			IsAllowed:       false,
			RejectionReason: string(domain.DecisionExecutionStatusBelowConfidenceThreshold),
		}
	}

	if openPositionCount >= config.MaxOpenPositions {
		return RiskCheckResult{
			IsAllowed:       false,
			RejectionReason: string(domain.DecisionExecutionStatusMaxPositionsReached),
		}
	}

	if currentDrawdownPercent.GreaterThanOrEqual(config.MaxDrawdownPercent) {
		return RiskCheckResult{
			IsAllowed:       false,
			RejectionReason: string(domain.DecisionExecutionStatusCircuitBreakerActive),
		}
	}

	return RiskCheckResult{IsAllowed: true}
}
