package engine

import (
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// CompositeWeights configures how individual signals combine.
// All weights should sum to 1.0.
type CompositeWeights struct {
	Rsi                float64
	Macd               float64
	Bollinger          float64
	EmaAlignment       float64
	CandlestickPattern float64
	SupportResistance  float64
	Sentiment          float64
}

// DefaultCompositeWeights returns sensible defaults where all weights sum to 1.0.
// The sentiment weight comes from config; the remaining share is split evenly
// across the six technical signals.
func DefaultCompositeWeights(sentimentWeight float64) CompositeWeights {
	technicalShare := 1.0 - sentimentWeight
	// 6 technical signals share equally
	perSignal := technicalShare / 6.0
	return CompositeWeights{
		Rsi:                perSignal,
		Macd:               perSignal,
		Bollinger:          perSignal,
		EmaAlignment:       perSignal,
		CandlestickPattern: perSignal,
		SupportResistance:  perSignal,
		Sentiment:          sentimentWeight,
	}
}

// SignalScores groups all individual scores for a single decision evaluation.
// All values are in the range [-100, 100].
type SignalScores struct {
	Rsi                decimal.Decimal
	Macd               decimal.Decimal
	Bollinger          decimal.Decimal
	EmaAlignment       decimal.Decimal
	CandlestickPattern decimal.Decimal
	SupportResistance  decimal.Decimal
	Sentiment          decimal.Decimal
}

// ComputeCompositeScore returns the weighted-average score in [-100, 100].
// Each signal score is multiplied by its weight, then summed.
func ComputeCompositeScore(scores SignalScores, weights CompositeWeights) decimal.Decimal {
	wRsi := decimal.NewFromFloat(weights.Rsi)
	wMacd := decimal.NewFromFloat(weights.Macd)
	wBollinger := decimal.NewFromFloat(weights.Bollinger)
	wEma := decimal.NewFromFloat(weights.EmaAlignment)
	wPattern := decimal.NewFromFloat(weights.CandlestickPattern)
	wSR := decimal.NewFromFloat(weights.SupportResistance)
	wSentiment := decimal.NewFromFloat(weights.Sentiment)

	composite := scores.Rsi.Mul(wRsi).
		Add(scores.Macd.Mul(wMacd)).
		Add(scores.Bollinger.Mul(wBollinger)).
		Add(scores.EmaAlignment.Mul(wEma)).
		Add(scores.CandlestickPattern.Mul(wPattern)).
		Add(scores.SupportResistance.Mul(wSR)).
		Add(scores.Sentiment.Mul(wSentiment))

	return clampScore(composite)
}

// DeriveActionAndConfidence converts a composite score into an Action and confidence (0-100).
//
// |compositeScore| < 20  → HOLD,  confidence = 100 - int(|score|)*5
// compositeScore >= 20   → BUY,   confidence = int(|score|), capped at 100
// compositeScore <= -20  → SELL,  confidence = int(|score|), capped at 100
func DeriveActionAndConfidence(compositeScore decimal.Decimal) (domain.DecisionAction, int) {
	absScore := compositeScore.Abs()
	threshold := twenty

	if absScore.LessThan(threshold) {
		// HOLD: confidence decreases the closer we get to the ±20 boundary.
		// At |score|=0, confidence=100; at |score|=19, confidence=5.
		absInt := int(absScore.IntPart())
		confidence := 100 - absInt*5
		if confidence < 0 {
			confidence = 0
		}
		return domain.DecisionActionHold, confidence
	}

	confidence := int(absScore.IntPart())
	if confidence > 100 {
		confidence = 100
	}

	if compositeScore.IsPositive() {
		return domain.DecisionActionBuy, confidence
	}
	return domain.DecisionActionSell, confidence
}
