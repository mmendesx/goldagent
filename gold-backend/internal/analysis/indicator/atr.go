package indicator

import (
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// CalculateAtr computes the Average True Range over the given period using Wilder's smoothing.
// True Range = max(high - low, abs(high - prev_close), abs(low - prev_close))
// Initial ATR: simple average of true range over first `period` values.
// Subsequent: Wilder's smoothing: ATR = (prev_ATR * (period - 1) + current_TR) / period
// Returns decimal.Zero if there aren't enough candles (need at least period+1).
func CalculateAtr(candles []domain.Candle, period int) decimal.Decimal {
	if period <= 0 || len(candles) < period+1 {
		return decimal.Zero
	}

	// Compute true range for each candle after the first.
	trValues := make([]decimal.Decimal, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		current := candles[i]
		prevClose := candles[i-1].ClosePrice

		highMinusLow := current.HighPrice.Sub(current.LowPrice)
		highMinusPrevClose := current.HighPrice.Sub(prevClose).Abs()
		lowMinusPrevClose := current.LowPrice.Sub(prevClose).Abs()

		tr := maxDecimal(highMinusLow, maxDecimal(highMinusPrevClose, lowMinusPrevClose))
		trValues[i-1] = tr
	}

	if len(trValues) < period {
		return decimal.Zero
	}

	periodDecimal := decimal.NewFromInt(int64(period))
	periodMinusOne := decimal.NewFromInt(int64(period - 1))

	// Seed: simple average of first `period` true range values.
	sum := decimal.Zero
	for i := 0; i < period; i++ {
		sum = sum.Add(trValues[i])
	}
	atr := sum.Div(periodDecimal)

	// Wilder's smoothing for subsequent values.
	for i := period; i < len(trValues); i++ {
		atr = atr.Mul(periodMinusOne).Add(trValues[i]).Div(periodDecimal)
	}

	return atr
}

// maxDecimal returns the larger of two decimal values.
func maxDecimal(a, b decimal.Decimal) decimal.Decimal {
	if a.GreaterThan(b) {
		return a
	}
	return b
}
