package engine

import (
	"testing"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// --- ComputeCompositeScore ---

func TestComputeCompositeScore_AllZero(t *testing.T) {
	weights := DefaultCompositeWeights(0.3)
	scores := SignalScores{} // all zero
	result := ComputeCompositeScore(scores, weights)
	if !result.IsZero() {
		t.Errorf("all zero scores should produce zero composite, got %s", result.String())
	}
}

func TestComputeCompositeScore_AllMaxBullish(t *testing.T) {
	weights := DefaultCompositeWeights(0.3)
	scores := SignalScores{
		Rsi:                d(100),
		Macd:               d(100),
		Bollinger:          d(100),
		EmaAlignment:       d(100),
		CandlestickPattern: d(100),
		SupportResistance:  d(100),
		Sentiment:          d(100),
	}
	result := ComputeCompositeScore(scores, weights)
	// Allow minor floating-point drift from weight arithmetic
	diff := result.Sub(d(100)).Abs()
	if diff.GreaterThan(d(0.01)) {
		t.Errorf("all max bullish: expected ~100, got %s", result.String())
	}
}

func TestComputeCompositeScore_AllMaxBearish(t *testing.T) {
	weights := DefaultCompositeWeights(0.3)
	scores := SignalScores{
		Rsi:                d(-100),
		Macd:               d(-100),
		Bollinger:          d(-100),
		EmaAlignment:       d(-100),
		CandlestickPattern: d(-100),
		SupportResistance:  d(-100),
		Sentiment:          d(-100),
	}
	result := ComputeCompositeScore(scores, weights)
	// Allow minor floating-point drift from weight arithmetic
	diff := result.Sub(d(-100)).Abs()
	if diff.GreaterThan(d(0.01)) {
		t.Errorf("all max bearish: expected ~-100, got %s", result.String())
	}
}

func TestComputeCompositeScore_WeightedByKnownValues(t *testing.T) {
	// Use equal weights (1/7 each) for predictability
	equalWeight := 1.0 / 7.0
	weights := CompositeWeights{
		Rsi:                equalWeight,
		Macd:               equalWeight,
		Bollinger:          equalWeight,
		EmaAlignment:       equalWeight,
		CandlestickPattern: equalWeight,
		SupportResistance:  equalWeight,
		Sentiment:          equalWeight,
	}
	scores := SignalScores{
		Rsi:                d(70),
		Macd:               d(70),
		Bollinger:          d(70),
		EmaAlignment:       d(70),
		CandlestickPattern: d(70),
		SupportResistance:  d(70),
		Sentiment:          d(70),
	}
	// All 70 × equal weights → composite = 70
	result := ComputeCompositeScore(scores, weights)
	// Allow minor floating-point tolerance
	expected := d(70)
	diff := result.Sub(expected).Abs()
	if diff.GreaterThan(d(0.01)) {
		t.Errorf("all scores 70 with equal weights should produce ~70, got %s", result.String())
	}
}

func TestComputeCompositeScore_SentimentDominates(t *testing.T) {
	// Sentiment weight = 0.9, technical signals nearly zeroed out
	weights := CompositeWeights{
		Rsi:                0.1 / 6.0,
		Macd:               0.1 / 6.0,
		Bollinger:          0.1 / 6.0,
		EmaAlignment:       0.1 / 6.0,
		CandlestickPattern: 0.1 / 6.0,
		SupportResistance:  0.1 / 6.0,
		Sentiment:          0.9,
	}
	scores := SignalScores{
		Sentiment: d(80),
		// All technical neutral
	}
	result := ComputeCompositeScore(scores, weights)
	// Should be close to 80 * 0.9 = 72
	expected := d(72)
	diff := result.Sub(expected).Abs()
	if diff.GreaterThan(d(1)) {
		t.Errorf("sentiment-dominated score should be ~72, got %s", result.String())
	}
}

// --- DeriveActionAndConfidence ---

func TestDeriveActionAndConfidence_Hold_Zero(t *testing.T) {
	action, confidence := DeriveActionAndConfidence(decimal.Zero)
	if action != domain.DecisionActionHold {
		t.Errorf("score 0 should be HOLD, got %s", action)
	}
	if confidence != 100 {
		t.Errorf("score 0 HOLD should have confidence 100, got %d", confidence)
	}
}

func TestDeriveActionAndConfidence_Hold_BelowThreshold_Positive(t *testing.T) {
	// Score +15 → HOLD, confidence = 100 - 15*5 = 25
	action, confidence := DeriveActionAndConfidence(d(15))
	if action != domain.DecisionActionHold {
		t.Errorf("score 15 should be HOLD, got %s", action)
	}
	if confidence != 25 {
		t.Errorf("score 15 HOLD should have confidence 25, got %d", confidence)
	}
}

func TestDeriveActionAndConfidence_Hold_BelowThreshold_Negative(t *testing.T) {
	// Score -10 → HOLD, confidence = 100 - 10*5 = 50
	action, confidence := DeriveActionAndConfidence(d(-10))
	if action != domain.DecisionActionHold {
		t.Errorf("score -10 should be HOLD, got %s", action)
	}
	if confidence != 50 {
		t.Errorf("score -10 HOLD should have confidence 50, got %d", confidence)
	}
}

func TestDeriveActionAndConfidence_Buy_AtThreshold(t *testing.T) {
	// Score exactly 20 → BUY, confidence = 20
	action, confidence := DeriveActionAndConfidence(d(20))
	if action != domain.DecisionActionBuy {
		t.Errorf("score 20 should be BUY, got %s", action)
	}
	if confidence != 20 {
		t.Errorf("score 20 BUY should have confidence 20, got %d", confidence)
	}
}

func TestDeriveActionAndConfidence_Buy_StrongSignal(t *testing.T) {
	action, confidence := DeriveActionAndConfidence(d(85))
	if action != domain.DecisionActionBuy {
		t.Errorf("score 85 should be BUY, got %s", action)
	}
	if confidence != 85 {
		t.Errorf("score 85 BUY should have confidence 85, got %d", confidence)
	}
}

func TestDeriveActionAndConfidence_Buy_MaxCapped(t *testing.T) {
	// Score 100 → BUY, confidence capped at 100
	action, confidence := DeriveActionAndConfidence(d(100))
	if action != domain.DecisionActionBuy {
		t.Errorf("score 100 should be BUY, got %s", action)
	}
	if confidence != 100 {
		t.Errorf("score 100 BUY should have confidence 100, got %d", confidence)
	}
}

func TestDeriveActionAndConfidence_Sell_AtThreshold(t *testing.T) {
	// Score -20 → SELL, confidence = 20
	action, confidence := DeriveActionAndConfidence(d(-20))
	if action != domain.DecisionActionSell {
		t.Errorf("score -20 should be SELL, got %s", action)
	}
	if confidence != 20 {
		t.Errorf("score -20 SELL should have confidence 20, got %d", confidence)
	}
}

func TestDeriveActionAndConfidence_Sell_StrongSignal(t *testing.T) {
	action, confidence := DeriveActionAndConfidence(d(-75))
	if action != domain.DecisionActionSell {
		t.Errorf("score -75 should be SELL, got %s", action)
	}
	if confidence != 75 {
		t.Errorf("score -75 SELL should have confidence 75, got %d", confidence)
	}
}

// --- DefaultCompositeWeights ---

func TestDefaultCompositeWeights_SumToOne(t *testing.T) {
	weights := DefaultCompositeWeights(0.3)
	sum := weights.Rsi + weights.Macd + weights.Bollinger +
		weights.EmaAlignment + weights.CandlestickPattern +
		weights.SupportResistance + weights.Sentiment

	// Allow tiny floating-point drift
	if sum < 0.9999 || sum > 1.0001 {
		t.Errorf("composite weights should sum to 1.0, got %f", sum)
	}
}

func TestDefaultCompositeWeights_SentimentWeightHonored(t *testing.T) {
	weights := DefaultCompositeWeights(0.5)
	if weights.Sentiment != 0.5 {
		t.Errorf("sentiment weight should be 0.5, got %f", weights.Sentiment)
	}
}
