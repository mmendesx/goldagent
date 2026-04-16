package indicator

import (
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// CalculateVwap computes the Volume-Weighted Average Price over all provided candles.
// VWAP = sum(typicalPrice * volume) / sum(volume)
// typicalPrice = (high + low + close) / 3
// Returns decimal.Zero if candles is empty or total volume is zero.
func CalculateVwap(candles []domain.Candle) decimal.Decimal {
	if len(candles) == 0 {
		return decimal.Zero
	}

	three := decimal.NewFromInt(3)
	totalTypicalVolume := decimal.Zero
	totalVolume := decimal.Zero

	for _, c := range candles {
		typicalPrice := c.HighPrice.Add(c.LowPrice).Add(c.ClosePrice).Div(three)
		totalTypicalVolume = totalTypicalVolume.Add(typicalPrice.Mul(c.Volume))
		totalVolume = totalVolume.Add(c.Volume)
	}

	if totalVolume.IsZero() {
		return decimal.Zero
	}

	return totalTypicalVolume.Div(totalVolume)
}
