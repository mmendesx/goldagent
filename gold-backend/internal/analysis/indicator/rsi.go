package indicator

import (
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// CalculateRsi computes the Relative Strength Index over the given period
// using Wilder's smoothing method.
// Returns the RSI value for the most recent candle (range 0-100).
// Returns decimal.Zero if there aren't enough candles (need at least period+1).
func CalculateRsi(candles []domain.Candle, period int) decimal.Decimal {
	if period <= 0 || len(candles) < period+1 {
		return decimal.Zero
	}

	// Compute price changes between consecutive closes.
	changes := make([]decimal.Decimal, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		changes[i-1] = candles[i].ClosePrice.Sub(candles[i-1].ClosePrice)
	}

	if len(changes) < period {
		return decimal.Zero
	}

	// Seed: simple average of first `period` gains and losses.
	var sumGain, sumLoss decimal.Decimal
	for i := 0; i < period; i++ {
		if changes[i].IsPositive() {
			sumGain = sumGain.Add(changes[i])
		} else {
			sumLoss = sumLoss.Add(changes[i].Abs())
		}
	}

	periodDecimal := decimal.NewFromInt(int64(period))
	avgGain := sumGain.Div(periodDecimal)
	avgLoss := sumLoss.Div(periodDecimal)

	// Apply Wilder's smoothing for the remaining changes.
	// avg = (prev_avg * (period - 1) + current) / period
	periodMinusOne := decimal.NewFromInt(int64(period - 1))
	for i := period; i < len(changes); i++ {
		change := changes[i]
		var gain, loss decimal.Decimal
		if change.IsPositive() {
			gain = change
		} else {
			loss = change.Abs()
		}
		avgGain = avgGain.Mul(periodMinusOne).Add(gain).Div(periodDecimal)
		avgLoss = avgLoss.Mul(periodMinusOne).Add(loss).Div(periodDecimal)
	}

	// Edge cases: no losses means overbought (100); no gains means oversold (0).
	if avgLoss.IsZero() {
		if avgGain.IsZero() {
			return decimal.Zero
		}
		return decimal.NewFromInt(100)
	}
	if avgGain.IsZero() {
		return decimal.Zero
	}

	// RSI = 100 - (100 / (1 + RS)), RS = avgGain / avgLoss
	hundred := decimal.NewFromInt(100)
	rs := avgGain.Div(avgLoss)
	rsi := hundred.Sub(hundred.Div(decimal.NewFromInt(1).Add(rs)))
	return rsi
}
