package candlestick

import (
	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

var (
	decimalTwo        = decimal.NewFromInt(2)
	decimalThree      = decimal.NewFromInt(3)
	decimalHalf       = decimal.NewFromFloat(0.5)
	dojiBodyRatio     = decimal.NewFromFloat(0.1)
	shadowCapRatio    = decimal.NewFromFloat(0.3)
	longBodyRatio     = decimal.NewFromFloat(0.6)
	smallBodyRatio    = decimal.NewFromFloat(0.3)
)

// Detector runs all pattern checks against a candle sequence.
type Detector struct{}

// NewDetector constructs a Detector.
func NewDetector() *Detector { return &Detector{} }

// DetectPatterns scans the most recent candles for matches.
// Returns all patterns that complete on the last candle (or nearby).
// Requires at least 5 candles for full coverage; with fewer, only some
// pattern types are detectable.
func (detector *Detector) DetectPatterns(candles []domain.Candle) []DetectedPattern {
	if len(candles) == 0 {
		return nil
	}

	results := make([]DetectedPattern, 0)
	last := len(candles) - 1

	// Single-candle patterns (always attempted when at least 1 candle present).
	if p := detectDoji(candles, last); p != nil {
		results = append(results, *p)
	}
	if p := detectHammer(candles, last); p != nil {
		results = append(results, *p)
	}
	if p := detectInvertedHammer(candles, last); p != nil {
		results = append(results, *p)
	}
	if p := detectShootingStar(candles, last); p != nil {
		results = append(results, *p)
	}

	// Two-candle patterns.
	if last >= 1 {
		if p := detectBullishEngulfing(candles, last); p != nil {
			results = append(results, *p)
		}
		if p := detectBearishEngulfing(candles, last); p != nil {
			results = append(results, *p)
		}
	}

	// Three-candle patterns.
	if last >= 2 {
		if p := detectMorningStar(candles, last); p != nil {
			results = append(results, *p)
		}
		if p := detectEveningStar(candles, last); p != nil {
			results = append(results, *p)
		}
	}

	return results
}

// detectDoji checks whether the last candle is a doji:
// body is very small relative to the total range (< 10%).
// Neutral signal; confidence scales inversely with body/range ratio.
func detectDoji(candles []domain.Candle, idx int) *DetectedPattern {
	c := candles[idx]
	bodySize := getBodySize(c)
	totalRange := getTotalRange(c)

	if totalRange.IsZero() {
		return nil
	}

	ratio := bodySize.Div(totalRange)
	if ratio.GreaterThanOrEqual(dojiBodyRatio) {
		return nil
	}

	// Confidence: tighter body → higher confidence (scales linearly 70-100).
	// ratio=0 → 100, ratio≈0.1 → 70.
	tightness := decimal.NewFromInt(1).Sub(ratio.Div(dojiBodyRatio))
	confidence := decimal.NewFromInt(70).Add(decimal.NewFromInt(30).Mul(tightness)).IntPart()

	return &DetectedPattern{
		Name:       PatternNameDoji,
		Direction:  PatternDirectionNeutral,
		Confidence: int(confidence),
		AtCandle:   idx,
	}
}

// detectHammer checks whether the last candle forms a hammer:
// small body, long lower shadow (≥ 2× body), little upper shadow (≤ 0.3× body).
// Bullish reversal signal.
func detectHammer(candles []domain.Candle, idx int) *DetectedPattern {
	c := candles[idx]
	bodySize := getBodySize(c)
	lowerShadow := getLowerShadow(c)
	upperShadow := getUpperShadow(c)

	if bodySize.IsZero() {
		return nil
	}

	minLower := bodySize.Mul(decimalTwo)
	maxUpper := bodySize.Mul(shadowCapRatio)

	if lowerShadow.LessThan(minLower) {
		return nil
	}
	if upperShadow.GreaterThan(maxUpper) {
		return nil
	}

	// Confidence: how much longer the lower shadow is relative to body.
	ratio := lowerShadow.Div(bodySize)
	confidence := 70
	if ratio.GreaterThanOrEqual(decimal.NewFromInt(3)) {
		confidence = 85
	}

	return &DetectedPattern{
		Name:       PatternNameHammer,
		Direction:  PatternDirectionBullish,
		Confidence: confidence,
		AtCandle:   idx,
	}
}

// detectInvertedHammer checks whether the last candle forms an inverted hammer:
// small body at bottom, long upper shadow (≥ 2× body), little lower shadow (≤ 0.3× body).
// Bullish reversal signal.
func detectInvertedHammer(candles []domain.Candle, idx int) *DetectedPattern {
	c := candles[idx]
	bodySize := getBodySize(c)
	upperShadow := getUpperShadow(c)
	lowerShadow := getLowerShadow(c)

	if bodySize.IsZero() {
		return nil
	}

	minUpper := bodySize.Mul(decimalTwo)
	maxLower := bodySize.Mul(shadowCapRatio)

	if upperShadow.LessThan(minUpper) {
		return nil
	}
	if lowerShadow.GreaterThan(maxLower) {
		return nil
	}

	ratio := upperShadow.Div(bodySize)
	confidence := 70
	if ratio.GreaterThanOrEqual(decimal.NewFromInt(3)) {
		confidence = 80
	}

	return &DetectedPattern{
		Name:       PatternNameInvertedHammer,
		Direction:  PatternDirectionBullish,
		Confidence: confidence,
		AtCandle:   idx,
	}
}

// detectShootingStar checks whether the last candle forms a shooting star:
// same shape as inverted hammer (long upper shadow, small body, minimal lower shadow)
// but is a bearish reversal signal. Shape detection only — trend context is left to
// the decision engine.
func detectShootingStar(candles []domain.Candle, idx int) *DetectedPattern {
	c := candles[idx]
	bodySize := getBodySize(c)
	upperShadow := getUpperShadow(c)
	lowerShadow := getLowerShadow(c)

	if bodySize.IsZero() {
		return nil
	}

	minUpper := bodySize.Mul(decimalTwo)
	maxLower := bodySize.Mul(shadowCapRatio)

	if upperShadow.LessThan(minUpper) {
		return nil
	}
	if lowerShadow.GreaterThan(maxLower) {
		return nil
	}

	ratio := upperShadow.Div(bodySize)
	confidence := 70
	if ratio.GreaterThanOrEqual(decimal.NewFromInt(3)) {
		confidence = 80
	}

	return &DetectedPattern{
		Name:       PatternNameShootingStar,
		Direction:  PatternDirectionBearish,
		Confidence: confidence,
		AtCandle:   idx,
	}
}

// detectBullishEngulfing checks the two most recent candles for a bullish engulfing pattern:
// previous candle is bearish; current candle is bullish and its body fully engulfs the previous body.
func detectBullishEngulfing(candles []domain.Candle, idx int) *DetectedPattern {
	prev := candles[idx-1]
	curr := candles[idx]

	if !isBearish(prev) {
		return nil
	}
	if !isBullish(curr) {
		return nil
	}

	// Current body must engulf previous body:
	// current_open < prev_close AND current_close > prev_open
	if !curr.OpenPrice.LessThan(prev.ClosePrice) {
		return nil
	}
	if !curr.ClosePrice.GreaterThan(prev.OpenPrice) {
		return nil
	}

	prevBody := getBodySize(prev)
	currBody := getBodySize(curr)

	confidence := 75
	if !prevBody.IsZero() {
		ratio := currBody.Div(prevBody)
		if ratio.GreaterThanOrEqual(decimalTwo) {
			confidence = 90
		} else if ratio.GreaterThanOrEqual(decimal.NewFromFloat(1.5)) {
			confidence = 82
		}
	}

	return &DetectedPattern{
		Name:       PatternNameBullishEngulfing,
		Direction:  PatternDirectionBullish,
		Confidence: confidence,
		AtCandle:   idx,
	}
}

// detectBearishEngulfing checks the two most recent candles for a bearish engulfing pattern:
// previous candle is bullish; current candle is bearish and its body fully engulfs the previous body.
func detectBearishEngulfing(candles []domain.Candle, idx int) *DetectedPattern {
	prev := candles[idx-1]
	curr := candles[idx]

	if !isBullish(prev) {
		return nil
	}
	if !isBearish(curr) {
		return nil
	}

	// current_open > prev_close AND current_close < prev_open
	if !curr.OpenPrice.GreaterThan(prev.ClosePrice) {
		return nil
	}
	if !curr.ClosePrice.LessThan(prev.OpenPrice) {
		return nil
	}

	prevBody := getBodySize(prev)
	currBody := getBodySize(curr)

	confidence := 75
	if !prevBody.IsZero() {
		ratio := currBody.Div(prevBody)
		if ratio.GreaterThanOrEqual(decimalTwo) {
			confidence = 90
		} else if ratio.GreaterThanOrEqual(decimal.NewFromFloat(1.5)) {
			confidence = 82
		}
	}

	return &DetectedPattern{
		Name:       PatternNameBearishEngulfing,
		Direction:  PatternDirectionBearish,
		Confidence: confidence,
		AtCandle:   idx,
	}
}

// detectMorningStar checks three consecutive candles ending at idx for a morning star:
//   - Candle 1: long bearish body
//   - Candle 2: small body (any color), opens below candle 1's close
//   - Candle 3: long bullish body, closes above midpoint of candle 1
//
// Bullish reversal.
func detectMorningStar(candles []domain.Candle, idx int) *DetectedPattern {
	c1 := candles[idx-2]
	c2 := candles[idx-1]
	c3 := candles[idx]

	// Candle 1: long bearish
	if !isBearish(c1) {
		return nil
	}
	c1Body := getBodySize(c1)
	c1Range := getTotalRange(c1)
	if c1Range.IsZero() {
		return nil
	}
	if c1Body.Div(c1Range).LessThan(longBodyRatio) {
		return nil
	}

	// Candle 2: small body (≤ 30% of its total range), gaps down from candle 1's close
	c2Body := getBodySize(c2)
	c2Range := getTotalRange(c2)
	if !c2Range.IsZero() {
		if c2Body.Div(c2Range).GreaterThan(smallBodyRatio) {
			return nil
		}
	}
	// Gap down: candle 2 opens below candle 1's close
	if !c2.OpenPrice.LessThan(c1.ClosePrice) {
		return nil
	}

	// Candle 3: long bullish
	if !isBullish(c3) {
		return nil
	}
	c3Body := getBodySize(c3)
	c3Range := getTotalRange(c3)
	if c3Range.IsZero() {
		return nil
	}
	if c3Body.Div(c3Range).LessThan(longBodyRatio) {
		return nil
	}

	// Candle 3 closes above midpoint of candle 1
	c1Midpoint := c1.OpenPrice.Add(c1.ClosePrice).Div(decimalTwo)
	if !c3.ClosePrice.GreaterThan(c1Midpoint) {
		return nil
	}

	return &DetectedPattern{
		Name:       PatternNameMorningStar,
		Direction:  PatternDirectionBullish,
		Confidence: 80,
		AtCandle:   idx,
	}
}

// detectEveningStar checks three consecutive candles ending at idx for an evening star:
//   - Candle 1: long bullish body
//   - Candle 2: small body (any color), gaps up from candle 1's close
//   - Candle 3: long bearish body, closes below midpoint of candle 1
//
// Bearish reversal.
func detectEveningStar(candles []domain.Candle, idx int) *DetectedPattern {
	c1 := candles[idx-2]
	c2 := candles[idx-1]
	c3 := candles[idx]

	// Candle 1: long bullish
	if !isBullish(c1) {
		return nil
	}
	c1Body := getBodySize(c1)
	c1Range := getTotalRange(c1)
	if c1Range.IsZero() {
		return nil
	}
	if c1Body.Div(c1Range).LessThan(longBodyRatio) {
		return nil
	}

	// Candle 2: small body, gaps up from candle 1's close
	c2Body := getBodySize(c2)
	c2Range := getTotalRange(c2)
	if !c2Range.IsZero() {
		if c2Body.Div(c2Range).GreaterThan(smallBodyRatio) {
			return nil
		}
	}
	// Gap up: candle 2 opens above candle 1's close
	if !c2.OpenPrice.GreaterThan(c1.ClosePrice) {
		return nil
	}

	// Candle 3: long bearish
	if !isBearish(c3) {
		return nil
	}
	c3Body := getBodySize(c3)
	c3Range := getTotalRange(c3)
	if c3Range.IsZero() {
		return nil
	}
	if c3Body.Div(c3Range).LessThan(longBodyRatio) {
		return nil
	}

	// Candle 3 closes below midpoint of candle 1
	c1Midpoint := c1.OpenPrice.Add(c1.ClosePrice).Div(decimalTwo)
	if !c3.ClosePrice.LessThan(c1Midpoint) {
		return nil
	}

	_ = decimalThree // used for future extension; suppress unused warning

	return &DetectedPattern{
		Name:       PatternNameEveningStar,
		Direction:  PatternDirectionBearish,
		Confidence: 80,
		AtCandle:   idx,
	}
}
