package indicator

import (
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// MacdResult holds the three MACD component values.
type MacdResult struct {
	Line      decimal.Decimal // EMA(fast) - EMA(slow)
	Signal    decimal.Decimal // EMA of Line over signalPeriod
	Histogram decimal.Decimal // Line - Signal
}

// CalculateMacd computes the MACD using close prices.
// Standard parameters: fast=12, slow=26, signal=9.
// Returns zero-valued MacdResult if there aren't enough candles.
// Minimum required: slowPeriod + signalPeriod - 1 candles.
func CalculateMacd(candles []domain.Candle, fastPeriod, slowPeriod, signalPeriod int) MacdResult {
	minRequired := slowPeriod + signalPeriod - 1
	if fastPeriod <= 0 || slowPeriod <= 0 || signalPeriod <= 0 || len(candles) < minRequired {
		return MacdResult{}
	}

	closes := extractClosePrices(candles)

	// Compute full EMA series for both fast and slow periods.
	fastSeries := calculateEmaSeries(closes, fastPeriod)
	slowSeries := calculateEmaSeries(closes, slowPeriod)

	if len(fastSeries) == 0 || len(slowSeries) == 0 {
		return MacdResult{}
	}

	// The slow series is shorter (starts later). Align: the slow series starts at
	// index (slowPeriod-1) of closes; the fast series starts at index (fastPeriod-1).
	// MACD line series is computed where both are available, i.e., starting from
	// index (slowPeriod-1) of closes.
	//
	// fastSeries[i] corresponds to closes index (fastPeriod-1+i)
	// slowSeries[i] corresponds to closes index (slowPeriod-1+i)
	//
	// Offset from fastSeries start to slowSeries start:
	fastOffset := slowPeriod - fastPeriod // fastSeries index where slowSeries begins

	if fastOffset < 0 || fastOffset >= len(fastSeries) {
		return MacdResult{}
	}

	// Build MACD line series (aligned with slow series).
	macdLineValues := make([]decimal.Decimal, len(slowSeries))
	for i := range slowSeries {
		macdLineValues[i] = fastSeries[fastOffset+i].Sub(slowSeries[i])
	}

	if len(macdLineValues) < signalPeriod {
		return MacdResult{}
	}

	// Signal line: EMA of the MACD line series over signalPeriod.
	signalSeries := calculateEmaSeries(macdLineValues, signalPeriod)
	if len(signalSeries) == 0 {
		return MacdResult{}
	}

	line := macdLineValues[len(macdLineValues)-1]
	signal := signalSeries[len(signalSeries)-1]
	histogram := line.Sub(signal)

	return MacdResult{
		Line:      line,
		Signal:    signal,
		Histogram: histogram,
	}
}
