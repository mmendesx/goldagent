package indicator

import (
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// CalculateExponentialMovingAverage computes EMA over close prices for the given period.
// Multiplier: 2 / (period + 1). Initial value: SMA over first `period` candles.
// Returns the EMA for the most recent candle.
// Returns decimal.Zero if there aren't enough candles (need at least `period` candles).
func CalculateExponentialMovingAverage(candles []domain.Candle, period int) decimal.Decimal {
	if period <= 0 || len(candles) < period {
		return decimal.Zero
	}

	closes := extractClosePrices(candles)
	series := calculateEmaSeries(closes, period)
	if len(series) == 0 {
		return decimal.Zero
	}
	return series[len(series)-1]
}

// calculateEmaSeries computes the full EMA series over a slice of decimal values.
// The output slice is aligned with the input: index i corresponds to the EMA at position i,
// starting from index (period-1). Values before that position are not included.
// Returns nil if there aren't enough values.
func calculateEmaSeries(values []decimal.Decimal, period int) []decimal.Decimal {
	if period <= 0 || len(values) < period {
		return nil
	}

	periodDecimal := decimal.NewFromInt(int64(period))
	// multiplier = 2 / (period + 1)
	multiplier := decimal.NewFromInt(2).Div(periodDecimal.Add(decimal.NewFromInt(1)))

	// Seed with SMA of first `period` values.
	sum := decimal.Zero
	for i := 0; i < period; i++ {
		sum = sum.Add(values[i])
	}
	initialSma := sum.Div(periodDecimal)

	result := make([]decimal.Decimal, len(values)-period+1)
	result[0] = initialSma

	// Apply standard EMA smoothing for subsequent values.
	// EMA = current * multiplier + prev_ema * (1 - multiplier)
	// Note: this uses the standard EMA multiplier 2/(period+1).
	// Wilder's smoothing uses 1/period and is applied separately in rsi.go and atr.go.
	for i := period; i < len(values); i++ {
		prev := result[i-period]
		current := values[i]
		ema := current.Mul(multiplier).Add(prev.Mul(decimal.NewFromInt(1).Sub(multiplier)))
		result[i-period+1] = ema
	}

	return result
}

// extractClosePrices extracts close prices from candles in order.
func extractClosePrices(candles []domain.Candle) []decimal.Decimal {
	prices := make([]decimal.Decimal, len(candles))
	for i, c := range candles {
		prices[i] = c.ClosePrice
	}
	return prices
}
