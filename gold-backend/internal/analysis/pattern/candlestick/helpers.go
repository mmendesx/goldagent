package candlestick

import (
	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

// getBodySize returns the absolute difference between open and close prices.
func getBodySize(c domain.Candle) decimal.Decimal {
	return c.ClosePrice.Sub(c.OpenPrice).Abs()
}

// getUpperShadow returns the distance from the top of the body to the high.
func getUpperShadow(c domain.Candle) decimal.Decimal {
	bodyTop := c.OpenPrice
	if c.ClosePrice.GreaterThan(c.OpenPrice) {
		bodyTop = c.ClosePrice
	}
	return c.HighPrice.Sub(bodyTop)
}

// getLowerShadow returns the distance from the bottom of the body to the low.
func getLowerShadow(c domain.Candle) decimal.Decimal {
	bodyBottom := c.OpenPrice
	if c.ClosePrice.LessThan(c.OpenPrice) {
		bodyBottom = c.ClosePrice
	}
	return bodyBottom.Sub(c.LowPrice)
}

// getTotalRange returns the distance from low to high.
func getTotalRange(c domain.Candle) decimal.Decimal {
	return c.HighPrice.Sub(c.LowPrice)
}

// isBullish returns true when the close is strictly above the open.
func isBullish(c domain.Candle) bool {
	return c.ClosePrice.GreaterThan(c.OpenPrice)
}

// isBearish returns true when the close is strictly below the open.
func isBearish(c domain.Candle) bool {
	return c.ClosePrice.LessThan(c.OpenPrice)
}
