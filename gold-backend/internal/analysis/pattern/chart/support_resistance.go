package chart

import (
	"sort"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// IdentifySupportLevels groups nearby swing lows into support levels.
// Two swing lows are considered the same level if they are within
// tolerancePercent of each other (e.g., 0.5%).
// Returns levels sorted by descending strength (most touches first).
// Only levels with strength >= 2 are returned.
func IdentifySupportLevels(candles []domain.Candle, pivotWindow int, tolerancePercent float64) []PriceLevel {
	swings := FindSwingPoints(candles, pivotWindow)

	lows := make([]SwingPoint, 0)
	for _, sp := range swings {
		if !sp.IsHigh {
			lows = append(lows, sp)
		}
	}

	return clusterIntoLevels(lows, tolerancePercent)
}

// IdentifyResistanceLevels groups nearby swing highs into resistance levels.
// Two swing highs are considered the same level if they are within
// tolerancePercent of each other (e.g., 0.5%).
// Returns levels sorted by descending strength (most touches first).
// Only levels with strength >= 2 are returned.
func IdentifyResistanceLevels(candles []domain.Candle, pivotWindow int, tolerancePercent float64) []PriceLevel {
	swings := FindSwingPoints(candles, pivotWindow)

	highs := make([]SwingPoint, 0)
	for _, sp := range swings {
		if sp.IsHigh {
			highs = append(highs, sp)
		}
	}

	return clusterIntoLevels(highs, tolerancePercent)
}

// clusterIntoLevels groups swing points that are within tolerancePercent of
// each other into a single PriceLevel. Uses greedy clustering: each ungrouped
// point seeds a new cluster and pulls in all other ungrouped points within
// tolerance. The level price is the average of all grouped prices; strength is
// the count. Only levels with strength >= 2 are returned, sorted by descending
// strength.
func clusterIntoLevels(points []SwingPoint, tolerancePercent float64) []PriceLevel {
	if len(points) == 0 {
		return nil
	}

	used := make([]bool, len(points))
	levels := make([]PriceLevel, 0)

	toleranceFactor := decimal.NewFromFloat(tolerancePercent / 100.0)

	for i := 0; i < len(points); i++ {
		if used[i] {
			continue
		}

		seed := points[i]
		group := []SwingPoint{seed}
		used[i] = true

		tolerance := seed.Price.Mul(toleranceFactor).Abs()

		for j := i + 1; j < len(points); j++ {
			if used[j] {
				continue
			}
			diff := points[j].Price.Sub(seed.Price).Abs()
			if diff.LessThanOrEqual(tolerance) {
				group = append(group, points[j])
				used[j] = true
			}
		}

		if len(group) < 2 {
			continue
		}

		sum := decimal.Zero
		lastTouch := group[0].Timestamp
		for _, sp := range group {
			sum = sum.Add(sp.Price)
			if sp.Timestamp.After(lastTouch) {
				lastTouch = sp.Timestamp
			}
		}
		avgPrice := sum.Div(decimal.NewFromInt(int64(len(group))))

		levels = append(levels, PriceLevel{
			Price:       avgPrice,
			Strength:    len(group),
			LastTouchAt: lastTouch,
		})
	}

	sort.Slice(levels, func(i, j int) bool {
		return levels[i].Strength > levels[j].Strength
	})

	return levels
}
