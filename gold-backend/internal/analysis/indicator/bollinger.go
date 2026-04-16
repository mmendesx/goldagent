package indicator

import (
	"math"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// BollingerBandsResult holds the three Bollinger Band levels.
type BollingerBandsResult struct {
	Upper  decimal.Decimal
	Middle decimal.Decimal // SMA over period
	Lower  decimal.Decimal
}

// CalculateBollingerBands computes Bollinger Bands for the most recent `period` candles.
// Middle = SMA(close, period).
// Upper = Middle + stddev * standardDeviationMultiplier.
// Lower = Middle - stddev * standardDeviationMultiplier.
// Standard parameters: period=20, standardDeviationMultiplier=2.
// Returns zero-valued result if there aren't enough candles (need at least `period`).
//
// Note: square root is computed via float64 conversion. This is the one place
// where floating-point arithmetic is acceptable — Bollinger Bands are a
// volatility indicator, not a financial settlement value.
func CalculateBollingerBands(candles []domain.Candle, period int, standardDeviationMultiplier float64) BollingerBandsResult {
	if period <= 0 || len(candles) < period {
		return BollingerBandsResult{}
	}

	// Use the most recent `period` candles.
	window := candles[len(candles)-period:]
	periodDecimal := decimal.NewFromInt(int64(period))

	// Compute SMA (middle band).
	sum := decimal.Zero
	for _, c := range window {
		sum = sum.Add(c.ClosePrice)
	}
	middle := sum.Div(periodDecimal)

	// Compute population standard deviation.
	sumSquaredDiffs := decimal.Zero
	for _, c := range window {
		diff := c.ClosePrice.Sub(middle)
		sumSquaredDiffs = sumSquaredDiffs.Add(diff.Mul(diff))
	}
	variance := sumSquaredDiffs.Div(periodDecimal)

	// Convert to float64 for sqrt, then back to decimal.
	stdDev := decimal.NewFromFloat(math.Sqrt(variance.InexactFloat64()))

	multiplier := decimal.NewFromFloat(standardDeviationMultiplier)
	band := stdDev.Mul(multiplier)

	return BollingerBandsResult{
		Upper:  middle.Add(band),
		Middle: middle,
		Lower:  middle.Sub(band),
	}
}
