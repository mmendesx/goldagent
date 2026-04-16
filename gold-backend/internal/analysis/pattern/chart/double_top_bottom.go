package chart

import (
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// DetectDoubleTopBottom looks for double-top and double-bottom formations in
// the swing-point series.
//
// A double top forms when:
//   - Two consecutive swing highs (H1, H2) appear at similar prices (within tolerancePercent)
//   - A swing low (L) sits between them
//   - Some candle after H2 closes below L (confirmation)
//
// Confidence scales from 50 to 100 based on how close H1 and H2 are in price
// (closer = higher confidence).
//
// A double bottom is the mirror: two lows at similar prices with a high between,
// confirmed by a close above the intervening high.
func DetectDoubleTopBottom(candles []domain.Candle, pivotWindow int, tolerancePercent float64) []DetectedChartPattern {
	swings := FindSwingPoints(candles, pivotWindow)
	if len(swings) < 3 {
		return nil
	}

	toleranceFactor := decimal.NewFromFloat(tolerancePercent / 100.0)
	patterns := make([]DetectedChartPattern, 0)

	for i := 0; i < len(swings)-2; i++ {
		first := swings[i]
		middle := swings[i+1]
		second := swings[i+2]

		// Double top: high - low - high
		if first.IsHigh && !middle.IsHigh && second.IsHigh {
			tolerance := first.Price.Mul(toleranceFactor).Abs()
			priceDiff := first.Price.Sub(second.Price).Abs()

			if priceDiff.LessThanOrEqual(tolerance) {
				if confirmed, atCandle := isDoubleTopConfirmed(candles, second.CandleIndex, middle.Price); confirmed {
					confidence := computeDoubleTopConfidence(first.Price, second.Price, tolerance)
					avgPeak := first.Price.Add(second.Price).Div(decimal.NewFromInt(2))
					patterns = append(patterns, DetectedChartPattern{
						Name:       ChartPatternNameDoubleTop,
						Direction:  "bearish",
						Confidence: confidence,
						KeyPrice:   avgPeak,
						AtCandle:   atCandle,
					})
				}
			}
		}

		// Double bottom: low - high - low
		if !first.IsHigh && middle.IsHigh && !second.IsHigh {
			tolerance := first.Price.Mul(toleranceFactor).Abs()
			priceDiff := first.Price.Sub(second.Price).Abs()

			if priceDiff.LessThanOrEqual(tolerance) {
				if confirmed, atCandle := isDoubleBottomConfirmed(candles, second.CandleIndex, middle.Price); confirmed {
					confidence := computeDoubleBottomConfidence(first.Price, second.Price, tolerance)
					avgTrough := first.Price.Add(second.Price).Div(decimal.NewFromInt(2))
					patterns = append(patterns, DetectedChartPattern{
						Name:       ChartPatternNameDoubleBottom,
						Direction:  "bullish",
						Confidence: confidence,
						KeyPrice:   avgTrough,
						AtCandle:   atCandle,
					})
				}
			}
		}
	}

	return patterns
}

// isDoubleTopConfirmed checks whether any candle after secondPeakIndex closes
// below the neckline (the intervening swing low). Returns true and the index of
// the confirming candle.
func isDoubleTopConfirmed(candles []domain.Candle, secondPeakIndex int, neckline decimal.Decimal) (bool, int) {
	for i := secondPeakIndex + 1; i < len(candles); i++ {
		if candles[i].ClosePrice.LessThan(neckline) {
			return true, i
		}
	}
	return false, 0
}

// isDoubleBottomConfirmed checks whether any candle after secondTroughIndex
// closes above the neckline (the intervening swing high). Returns true and the
// index of the confirming candle.
func isDoubleBottomConfirmed(candles []domain.Candle, secondTroughIndex int, neckline decimal.Decimal) (bool, int) {
	for i := secondTroughIndex + 1; i < len(candles); i++ {
		if candles[i].ClosePrice.GreaterThan(neckline) {
			return true, i
		}
	}
	return false, 0
}

// computeDoubleTopConfidence returns a confidence score from 50 to 100.
// Peaks that are exactly equal score 100; peaks at the tolerance boundary
// score 50. Linear interpolation between.
func computeDoubleTopConfidence(price1, price2, tolerance decimal.Decimal) int {
	if tolerance.IsZero() {
		return 100
	}
	diff := price1.Sub(price2).Abs()
	ratio := diff.Div(tolerance) // 0 = perfect match, 1 = at tolerance boundary
	// confidence = 100 - ratio * 50
	penalty := ratio.Mul(decimal.NewFromInt(50))
	confidence := decimal.NewFromInt(100).Sub(penalty)
	result := confidence.IntPart()
	if result < 50 {
		return 50
	}
	if result > 100 {
		return 100
	}
	return int(result)
}

// computeDoubleBottomConfidence mirrors computeDoubleTopConfidence for troughs.
func computeDoubleBottomConfidence(price1, price2, tolerance decimal.Decimal) int {
	return computeDoubleTopConfidence(price1, price2, tolerance)
}
