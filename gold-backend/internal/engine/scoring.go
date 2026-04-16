package engine

import (
	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/pattern/candlestick"
	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/pattern/chart"
	"github.com/shopspring/decimal"
)

var (
	hundred    = decimal.NewFromInt(100)
	fifty      = decimal.NewFromInt(50)
	seventy    = decimal.NewFromInt(70)
	thirty     = decimal.NewFromInt(30)
	twenty     = decimal.NewFromInt(20)
	negHundred = decimal.NewFromInt(-100)
)

// ScoreFromRsi converts RSI value to a signed signal in [-100, 100].
// RSI < 30 = oversold (bullish, +score), RSI > 70 = overbought (bearish, -score),
// RSI 30-70 = mild signal scaled linearly from 0 at RSI 50.
// Examples:
//
//	RSI 20 → +80 (oversold, strong buy signal)
//	RSI 50 → 0 (neutral)
//	RSI 80 → -80 (overbought, strong sell signal)
func ScoreFromRsi(rsi decimal.Decimal) decimal.Decimal {
	// Scale: each unit away from 50 contributes 2 points (range 0-100 RSI → -100 to +100 score)
	// Score = (50 - RSI) * 2, clamped to [-100, 100]
	score := fifty.Sub(rsi).Mul(decimal.NewFromInt(2))
	return clampScore(score)
}

// ScoreFromMacdHistogram converts the MACD histogram to a signal in [-100, 100].
// The histogram is normalized by current price to account for different asset scales.
// A positive histogram is bullish; negative is bearish.
func ScoreFromMacdHistogram(histogram decimal.Decimal, currentPrice decimal.Decimal) decimal.Decimal {
	if currentPrice.IsZero() {
		return decimal.Zero
	}

	// Normalize histogram as percentage of price, then scale by 5000 to map small
	// fractional values into a meaningful [-100, 100] range.
	normalized := histogram.Div(currentPrice).Mul(decimal.NewFromInt(5000))
	return clampScore(normalized)
}

// ScoreFromBollingerPosition returns where the current price sits relative to Bollinger Bands.
// Below lower band = strongly oversold (+100), above upper band = strongly overbought (-100).
// Within the bands, score is scaled linearly based on position relative to middle.
func ScoreFromBollingerPosition(currentPrice, upper, middle, lower decimal.Decimal) decimal.Decimal {
	bandWidth := upper.Sub(lower)
	if bandWidth.IsZero() {
		return decimal.Zero
	}

	// Price above upper band
	if currentPrice.GreaterThan(upper) {
		excess := currentPrice.Sub(upper).Div(bandWidth).Mul(hundred)
		return clampScore(excess.Neg())
	}

	// Price below lower band
	if currentPrice.LessThan(lower) {
		deficit := lower.Sub(currentPrice).Div(bandWidth).Mul(hundred)
		return clampScore(deficit)
	}

	// Price within bands: score based on distance from middle, scaled to half-band width
	halfBand := bandWidth.Div(decimal.NewFromInt(2))
	distFromMiddle := middle.Sub(currentPrice) // positive when price below middle (bullish)
	score := distFromMiddle.Div(halfBand).Mul(hundred)
	return clampScore(score)
}

// ScoreFromEmaAlignment returns a trend signal based on EMA alignment.
// Fully bullish when EMA9 > EMA21 > EMA50 > EMA200 (+100).
// Fully bearish when EMA9 < EMA21 < EMA50 < EMA200 (-100).
// Partial alignments score proportionally (3 relationships checked, each worth ~33 points).
func ScoreFromEmaAlignment(ema9, ema21, ema50, ema200 decimal.Decimal) decimal.Decimal {
	const relationshipScore = 33

	score := decimal.Zero

	// Bullish: shorter > longer (+), Bearish: shorter < longer (-)
	score = score.Add(compareEmas(ema9, ema21, relationshipScore))
	score = score.Add(compareEmas(ema21, ema50, relationshipScore))
	score = score.Add(compareEmas(ema50, ema200, relationshipScore))

	return clampScore(score)
}

// compareEmas returns +points if shorter > longer (bullish), -points if shorter < longer (bearish), 0 if equal.
func compareEmas(shorter, longer decimal.Decimal, points int) decimal.Decimal {
	if shorter.GreaterThan(longer) {
		return decimal.NewFromInt(int64(points))
	}
	if shorter.LessThan(longer) {
		return decimal.NewFromInt(int64(-points))
	}
	return decimal.Zero
}

// ScoreFromCandlestickPatterns aggregates detected patterns into a single score in [-100, 100].
// Bullish patterns add positive contributions, bearish subtract, weighted by confidence.
// Neutral patterns contribute zero.
func ScoreFromCandlestickPatterns(patterns []candlestick.DetectedPattern) decimal.Decimal {
	if len(patterns) == 0 {
		return decimal.Zero
	}

	total := decimal.Zero
	for _, p := range patterns {
		contribution := decimal.NewFromInt(int64(p.Confidence)) // 0-100
		switch p.Direction {
		case candlestick.PatternDirectionBullish:
			total = total.Add(contribution)
		case candlestick.PatternDirectionBearish:
			total = total.Sub(contribution)
		// Neutral contributes zero
		}
	}

	// Normalize by pattern count so many weak patterns don't dominate.
	score := total.Div(decimal.NewFromInt(int64(len(patterns))))
	return clampScore(score)
}

// ScoreFromSupportResistance returns a breakout-weighted score in [-100, 100].
// Upward breakouts are bullish (+), downward breakouts are bearish (-).
// Proximity to support (without breakout) adds mild positive bias.
// Proximity to resistance (without breakout) adds mild negative bias.
func ScoreFromSupportResistance(currentPrice decimal.Decimal, analysis chart.AnalysisResult) decimal.Decimal {
	score := decimal.Zero

	// Weight breakouts most heavily: each confirmed breakout = ±60 points (normalized by count)
	breakoutScore := decimal.Zero
	for _, b := range analysis.Breakouts {
		switch b.Direction {
		case chart.BreakoutDirectionUp:
			breakoutScore = breakoutScore.Add(decimal.NewFromInt(60))
		case chart.BreakoutDirectionDown:
			breakoutScore = breakoutScore.Sub(decimal.NewFromInt(60))
		}
	}
	if len(analysis.Breakouts) > 0 {
		breakoutScore = breakoutScore.Div(decimal.NewFromInt(int64(len(analysis.Breakouts))))
	}
	score = score.Add(breakoutScore)

	// Proximity to nearest support: +10 if within 1% of nearest support
	if len(analysis.SupportLevels) > 0 && !currentPrice.IsZero() {
		nearest := nearestLevel(currentPrice, analysis.SupportLevels)
		proximity := nearest.Sub(currentPrice).Abs().Div(currentPrice)
		if proximity.LessThan(decimal.NewFromFloat(0.01)) {
			score = score.Add(decimal.NewFromInt(10))
		}
	}

	// Proximity to nearest resistance: -10 if within 1% of nearest resistance
	if len(analysis.ResistanceLevels) > 0 && !currentPrice.IsZero() {
		nearest := nearestLevel(currentPrice, analysis.ResistanceLevels)
		proximity := nearest.Sub(currentPrice).Abs().Div(currentPrice)
		if proximity.LessThan(decimal.NewFromFloat(0.01)) {
			score = score.Sub(decimal.NewFromInt(10))
		}
	}

	// Chart patterns (double top/bottom) adjust score
	for _, p := range analysis.Patterns {
		contribution := decimal.NewFromInt(int64(p.Confidence) / 3) // mild impact
		switch p.Direction {
		case "bullish":
			score = score.Add(contribution)
		case "bearish":
			score = score.Sub(contribution)
		}
	}

	return clampScore(score)
}

// nearestLevel finds the price level closest to currentPrice from a set of levels.
func nearestLevel(currentPrice decimal.Decimal, levels []chart.PriceLevel) decimal.Decimal {
	nearest := levels[0].Price
	minDist := currentPrice.Sub(nearest).Abs()

	for _, level := range levels[1:] {
		dist := currentPrice.Sub(level.Price).Abs()
		if dist.LessThan(minDist) {
			minDist = dist
			nearest = level.Price
		}
	}
	return nearest
}

// ScoreFromSentiment converts raw sentiment in [-1, 1] to a score in [-100, 100].
func ScoreFromSentiment(rawSentiment decimal.Decimal) decimal.Decimal {
	return clampScore(rawSentiment.Mul(hundred))
}

// clampScore ensures a score stays within [-100, 100].
func clampScore(score decimal.Decimal) decimal.Decimal {
	if score.GreaterThan(hundred) {
		return hundred
	}
	if score.LessThan(negHundred) {
		return negHundred
	}
	return score
}
