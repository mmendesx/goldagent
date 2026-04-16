package chart

import (
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// DetectBreakouts checks if the most recent candle has broken above any
// resistance level or below any support level, with volume confirmation.
// volumeAverageWindow is the number of preceding candles used to compute
// average volume (the breakout candle itself is excluded from the average).
// volumeRatioThreshold is the minimum ratio of current volume to average volume
// required for confirmation (e.g., 1.5).
// Returns all confirmed breakout events; may return more than one if multiple
// levels are broken simultaneously.
func DetectBreakouts(
	candles []domain.Candle,
	supportLevels []PriceLevel,
	resistanceLevels []PriceLevel,
	volumeAverageWindow int,
	volumeRatioThreshold float64,
) []BreakoutEvent {
	if len(candles) < 2 {
		return nil
	}

	lastIndex := len(candles) - 1
	lastCandle := candles[lastIndex]

	avgVolume := computeAverageVolume(candles, lastIndex, volumeAverageWindow)
	if avgVolume.IsZero() {
		return nil
	}

	volumeRatio := lastCandle.Volume.Div(avgVolume)
	threshold := decimal.NewFromFloat(volumeRatioThreshold)

	if volumeRatio.LessThan(threshold) {
		return nil
	}

	events := make([]BreakoutEvent, 0)

	for _, level := range resistanceLevels {
		if lastCandle.ClosePrice.GreaterThan(level.Price) {
			events = append(events, BreakoutEvent{
				Direction:     BreakoutDirectionUp,
				BrokenLevel:   level.Price,
				BreakoutPrice: lastCandle.ClosePrice,
				VolumeRatio:   volumeRatio,
				AtCandle:      lastIndex,
			})
		}
	}

	for _, level := range supportLevels {
		if lastCandle.ClosePrice.LessThan(level.Price) {
			events = append(events, BreakoutEvent{
				Direction:     BreakoutDirectionDown,
				BrokenLevel:   level.Price,
				BreakoutPrice: lastCandle.ClosePrice,
				VolumeRatio:   volumeRatio,
				AtCandle:      lastIndex,
			})
		}
	}

	return events
}

// computeAverageVolume returns the mean volume of the candles preceding the
// candle at breakoutIndex. Uses up to volumeAverageWindow candles. The candle
// at breakoutIndex itself is excluded.
func computeAverageVolume(candles []domain.Candle, breakoutIndex int, volumeAverageWindow int) decimal.Decimal {
	start := breakoutIndex - volumeAverageWindow
	if start < 0 {
		start = 0
	}

	window := candles[start:breakoutIndex]
	if len(window) == 0 {
		return decimal.Zero
	}

	sum := decimal.Zero
	for _, c := range window {
		sum = sum.Add(c.Volume)
	}

	return sum.Div(decimal.NewFromInt(int64(len(window))))
}
