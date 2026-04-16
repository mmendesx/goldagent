package chart

import (
	"time"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// SwingPoint represents a local high or low (pivot) in the price series.
type SwingPoint struct {
	CandleIndex int
	Price       decimal.Decimal
	IsHigh      bool // true = swing high, false = swing low
	Timestamp   time.Time
}

// FindSwingPoints identifies local highs and lows using a simple pivot algorithm.
// A candle is a swing high if its high is strictly greater than the highs of N
// candles before AND after. A swing low is the inverse using lows.
// pivotWindow is N — typically 2 or 3.
// Candles within pivotWindow of either end of the slice are skipped.
func FindSwingPoints(candles []domain.Candle, pivotWindow int) []SwingPoint {
	if pivotWindow < 1 {
		pivotWindow = 1
	}

	points := make([]SwingPoint, 0)
	last := len(candles) - 1

	for i := pivotWindow; i <= last-pivotWindow; i++ {
		if isSwingHigh(candles, i, pivotWindow) {
			points = append(points, SwingPoint{
				CandleIndex: i,
				Price:       candles[i].HighPrice,
				IsHigh:      true,
				Timestamp:   candles[i].CloseTime,
			})
		} else if isSwingLow(candles, i, pivotWindow) {
			points = append(points, SwingPoint{
				CandleIndex: i,
				Price:       candles[i].LowPrice,
				IsHigh:      false,
				Timestamp:   candles[i].CloseTime,
			})
		}
	}

	return points
}

// isSwingHigh returns true when candle[i].HighPrice is strictly greater than all
// surrounding candles within the pivot window.
func isSwingHigh(candles []domain.Candle, i, pivotWindow int) bool {
	pivot := candles[i].HighPrice
	for offset := 1; offset <= pivotWindow; offset++ {
		if !pivot.GreaterThan(candles[i-offset].HighPrice) {
			return false
		}
		if !pivot.GreaterThan(candles[i+offset].HighPrice) {
			return false
		}
	}
	return true
}

// isSwingLow returns true when candle[i].LowPrice is strictly less than all
// surrounding candles within the pivot window.
func isSwingLow(candles []domain.Candle, i, pivotWindow int) bool {
	pivot := candles[i].LowPrice
	for offset := 1; offset <= pivotWindow; offset++ {
		if !pivot.LessThan(candles[i-offset].LowPrice) {
			return false
		}
		if !pivot.LessThan(candles[i+offset].LowPrice) {
			return false
		}
	}
	return true
}
