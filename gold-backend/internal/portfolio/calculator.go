package portfolio

import (
	"math"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// AggregateStats holds raw aggregates computed from a set of closed positions.
type AggregateStats struct {
	TotalPnl    decimal.Decimal
	WinCount    int
	LossCount   int
	TotalTrades int
	SumOfWins   decimal.Decimal
	SumOfLosses decimal.Decimal // stored as positive (absolute value)
}

// AggregateClosedPositions computes raw stats from a slice of positions.
// Only positions with Status == "closed" are included; zero-value RealizedPnl
// positions are counted in TotalTrades but do not affect wins/losses (consistent
// with the storage layer's COALESCE-to-zero behaviour for NULL realized_pnl).
func AggregateClosedPositions(positions []domain.Position) AggregateStats {
	var stats AggregateStats
	for _, p := range positions {
		if p.Status != "closed" {
			continue
		}

		stats.TotalTrades++
		stats.TotalPnl = stats.TotalPnl.Add(p.RealizedPnl)

		switch {
		case p.RealizedPnl.IsPositive():
			stats.WinCount++
			stats.SumOfWins = stats.SumOfWins.Add(p.RealizedPnl)
		case p.RealizedPnl.IsNegative():
			stats.LossCount++
			stats.SumOfLosses = stats.SumOfLosses.Add(p.RealizedPnl.Neg())
		}
	}
	return stats
}

// CalculateWinRate returns winCount / totalTrades * 100.
// Returns 0 if no trades have been recorded.
func CalculateWinRate(stats AggregateStats) decimal.Decimal {
	if stats.TotalTrades == 0 {
		return decimal.Zero
	}
	return decimal.NewFromInt(int64(stats.WinCount)).
		Div(decimal.NewFromInt(int64(stats.TotalTrades))).
		Mul(decimal.NewFromInt(100))
}

// CalculateProfitFactor returns sumOfWins / sumOfLosses.
// Returns 0 if no losing trades have been recorded.
func CalculateProfitFactor(stats AggregateStats) decimal.Decimal {
	if stats.SumOfLosses.IsZero() {
		return decimal.Zero
	}
	return stats.SumOfWins.Div(stats.SumOfLosses)
}

// CalculateAverageWin returns sumOfWins / winCount.
// Returns 0 if no winning trades have been recorded.
func CalculateAverageWin(stats AggregateStats) decimal.Decimal {
	if stats.WinCount == 0 {
		return decimal.Zero
	}
	return stats.SumOfWins.Div(decimal.NewFromInt(int64(stats.WinCount)))
}

// CalculateAverageLoss returns sumOfLosses / lossCount (result is positive).
// Returns 0 if no losing trades have been recorded.
func CalculateAverageLoss(stats AggregateStats) decimal.Decimal {
	if stats.LossCount == 0 {
		return decimal.Zero
	}
	return stats.SumOfLosses.Div(decimal.NewFromInt(int64(stats.LossCount)))
}

// CalculateSharpeRatio computes the annualized Sharpe Ratio from a series of per-trade P&L values.
//
// Formula: mean(pnl) / stddev(pnl) * sqrt(annualizationFactor)
//
// The standard deviation uses the population formula (divide by N), consistent with the
// Bollinger Bands implementation in the indicator package. pnlSeries values are absolute
// P&L amounts (not percentage returns) — the ratio is therefore unit-consistent across
// the series but not directly comparable to traditional Sharpe Ratios computed from returns.
//
// The square root and division inside stddev computation use float64 (via InexactFloat64)
// because this is an analytics metric, not a financial settlement value. Precision loss
// at this step is acceptable and mirrors the pattern used in Bollinger Band calculations.
//
// Returns 0 if fewer than 2 trades are in the series or if the population standard
// deviation is zero (all P&L values are identical).
func CalculateSharpeRatio(pnlSeries []decimal.Decimal, annualizationFactor int) decimal.Decimal {
	n := len(pnlSeries)
	if n < 2 {
		return decimal.Zero
	}

	count := decimal.NewFromInt(int64(n))

	// Compute mean.
	sum := decimal.Zero
	for _, v := range pnlSeries {
		sum = sum.Add(v)
	}
	mean := sum.Div(count)

	// Compute population standard deviation.
	sumSquaredDiffs := decimal.Zero
	for _, v := range pnlSeries {
		diff := v.Sub(mean)
		sumSquaredDiffs = sumSquaredDiffs.Add(diff.Mul(diff))
	}
	variance := sumSquaredDiffs.Div(count)

	// Convert to float64 for sqrt — analytics metric, precision tradeoff is acceptable.
	stdDev := math.Sqrt(variance.InexactFloat64())
	if stdDev == 0 {
		return decimal.Zero
	}

	annualization := math.Sqrt(float64(annualizationFactor))

	sharpe := (mean.InexactFloat64() / stdDev) * annualization
	return decimal.NewFromFloat(sharpe)
}

// CalculateDrawdownPercent returns ((peakBalance - currentBalance) / peakBalance) * 100.
// Returns 0 if peakBalance is zero or negative (no reference point established yet).
func CalculateDrawdownPercent(currentBalance, peakBalance decimal.Decimal) decimal.Decimal {
	if !peakBalance.IsPositive() {
		return decimal.Zero
	}
	return peakBalance.Sub(currentBalance).
		Div(peakBalance).
		Mul(decimal.NewFromInt(100))
}
