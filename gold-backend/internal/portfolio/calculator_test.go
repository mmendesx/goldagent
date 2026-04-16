package portfolio

import (
	"math"
	"testing"
	"time"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// mustDecimal parses a decimal string and panics on failure — test helper only.
func mustDecimal(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("mustDecimal: " + err.Error())
	}
	return d
}

func makeClosedPosition(pnl string) domain.Position {
	return domain.Position{
		Status:      "closed",
		RealizedPnl: mustDecimal(pnl),
		ClosedAt:    ptrTime(time.Now()),
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

// --- AggregateClosedPositions ---

func TestAggregateClosedPositions_MixedWinsAndLosses(t *testing.T) {
	positions := []domain.Position{
		makeClosedPosition("100"),
		makeClosedPosition("200"),
		makeClosedPosition("-50"),
		makeClosedPosition("-75"),
		makeClosedPosition("0"),
		// open position should be ignored
		{Status: "open", RealizedPnl: mustDecimal("999")},
	}

	stats := AggregateClosedPositions(positions)

	if stats.TotalTrades != 5 {
		t.Errorf("TotalTrades: got %d, want 5", stats.TotalTrades)
	}
	if stats.WinCount != 2 {
		t.Errorf("WinCount: got %d, want 2", stats.WinCount)
	}
	if stats.LossCount != 2 {
		t.Errorf("LossCount: got %d, want 2", stats.LossCount)
	}
	if !stats.SumOfWins.Equal(mustDecimal("300")) {
		t.Errorf("SumOfWins: got %s, want 300", stats.SumOfWins)
	}
	if !stats.SumOfLosses.Equal(mustDecimal("125")) {
		t.Errorf("SumOfLosses: got %s, want 125", stats.SumOfLosses)
	}
	if !stats.TotalPnl.Equal(mustDecimal("175")) {
		t.Errorf("TotalPnl: got %s, want 175", stats.TotalPnl)
	}
}

func TestAggregateClosedPositions_IgnoresOpenPositions(t *testing.T) {
	positions := []domain.Position{
		{Status: "open", RealizedPnl: mustDecimal("500")},
	}

	stats := AggregateClosedPositions(positions)

	if stats.TotalTrades != 0 {
		t.Errorf("TotalTrades: got %d, want 0", stats.TotalTrades)
	}
}

func TestAggregateClosedPositions_EmptySlice(t *testing.T) {
	stats := AggregateClosedPositions(nil)

	if stats.TotalTrades != 0 {
		t.Errorf("TotalTrades: got %d, want 0", stats.TotalTrades)
	}
	if !stats.TotalPnl.IsZero() {
		t.Errorf("TotalPnl: got %s, want 0", stats.TotalPnl)
	}
}

// --- CalculateWinRate ---

func TestCalculateWinRate_SevenWinsThreeLosses(t *testing.T) {
	stats := AggregateStats{WinCount: 7, TotalTrades: 10}
	rate := CalculateWinRate(stats)

	if !rate.Equal(mustDecimal("70")) {
		t.Errorf("WinRate: got %s, want 70", rate)
	}
}

func TestCalculateWinRate_NoTrades(t *testing.T) {
	stats := AggregateStats{}
	rate := CalculateWinRate(stats)

	if !rate.IsZero() {
		t.Errorf("WinRate: got %s, want 0", rate)
	}
}

// --- CalculateProfitFactor ---

func TestCalculateProfitFactor_WinsAndLosses(t *testing.T) {
	stats := AggregateStats{
		SumOfWins:   mustDecimal("300"),
		SumOfLosses: mustDecimal("100"),
	}
	pf := CalculateProfitFactor(stats)

	if !pf.Equal(mustDecimal("3")) {
		t.Errorf("ProfitFactor: got %s, want 3", pf)
	}
}

func TestCalculateProfitFactor_NoLosses(t *testing.T) {
	stats := AggregateStats{SumOfWins: mustDecimal("500")}
	pf := CalculateProfitFactor(stats)

	if !pf.IsZero() {
		t.Errorf("ProfitFactor: got %s, want 0", pf)
	}
}

// --- CalculateAverageWin ---

func TestCalculateAverageWin_KnownTotals(t *testing.T) {
	stats := AggregateStats{
		SumOfWins: mustDecimal("300"),
		WinCount:  3,
	}
	avg := CalculateAverageWin(stats)

	if !avg.Equal(mustDecimal("100")) {
		t.Errorf("AverageWin: got %s, want 100", avg)
	}
}

func TestCalculateAverageWin_NoWins(t *testing.T) {
	stats := AggregateStats{}
	avg := CalculateAverageWin(stats)

	if !avg.IsZero() {
		t.Errorf("AverageWin: got %s, want 0", avg)
	}
}

// --- CalculateAverageLoss ---

func TestCalculateAverageLoss_KnownTotals(t *testing.T) {
	stats := AggregateStats{
		SumOfLosses: mustDecimal("150"),
		LossCount:   3,
	}
	avg := CalculateAverageLoss(stats)

	if !avg.Equal(mustDecimal("50")) {
		t.Errorf("AverageLoss: got %s, want 50", avg)
	}
}

func TestCalculateAverageLoss_NoLosses(t *testing.T) {
	stats := AggregateStats{}
	avg := CalculateAverageLoss(stats)

	if !avg.IsZero() {
		t.Errorf("AverageLoss: got %s, want 0", avg)
	}
}

// --- CalculateSharpeRatio ---

func TestCalculateSharpeRatio_KnownSeries(t *testing.T) {
	// Series: [10, 20, 30, 40, 50]
	// mean = 30
	// population variance = ((10-30)^2 + (20-30)^2 + (30-30)^2 + (40-30)^2 + (50-30)^2) / 5
	//                     = (400 + 100 + 0 + 100 + 400) / 5 = 200
	// stddev = sqrt(200) ≈ 14.1421356
	// sharpe (factor=252) = (30 / 14.1421356) * sqrt(252) ≈ 2.1213 * 15.8745 ≈ 33.674
	pnlSeries := []decimal.Decimal{
		mustDecimal("10"), mustDecimal("20"), mustDecimal("30"),
		mustDecimal("40"), mustDecimal("50"),
	}

	sharpe := CalculateSharpeRatio(pnlSeries, 252)
	expected := (30.0 / math.Sqrt(200.0)) * math.Sqrt(252.0)

	got, _ := sharpe.Float64()
	tolerance := 0.001
	if math.Abs(got-expected) > tolerance {
		t.Errorf("SharpeRatio: got %.6f, want %.6f (tolerance %.3f)", got, expected, tolerance)
	}
}

func TestCalculateSharpeRatio_SingleTrade(t *testing.T) {
	pnlSeries := []decimal.Decimal{mustDecimal("100")}
	sharpe := CalculateSharpeRatio(pnlSeries, 252)

	if !sharpe.IsZero() {
		t.Errorf("SharpeRatio with 1 trade: got %s, want 0", sharpe)
	}
}

func TestCalculateSharpeRatio_EmptySeries(t *testing.T) {
	sharpe := CalculateSharpeRatio(nil, 252)

	if !sharpe.IsZero() {
		t.Errorf("SharpeRatio with empty series: got %s, want 0", sharpe)
	}
}

func TestCalculateSharpeRatio_IdenticalValues(t *testing.T) {
	// All values the same → stddev = 0 → should return 0.
	pnlSeries := []decimal.Decimal{
		mustDecimal("50"), mustDecimal("50"), mustDecimal("50"),
	}
	sharpe := CalculateSharpeRatio(pnlSeries, 252)

	if !sharpe.IsZero() {
		t.Errorf("SharpeRatio with zero stddev: got %s, want 0", sharpe)
	}
}

// --- CalculateDrawdownPercent ---

func TestCalculateDrawdownPercent_TypicalDrawdown(t *testing.T) {
	// peak=1000, current=850 → drawdown = (1000-850)/1000 * 100 = 15
	dd := CalculateDrawdownPercent(mustDecimal("850"), mustDecimal("1000"))

	if !dd.Equal(mustDecimal("15")) {
		t.Errorf("DrawdownPercent: got %s, want 15", dd)
	}
}

func TestCalculateDrawdownPercent_NoPeakEstablished(t *testing.T) {
	dd := CalculateDrawdownPercent(mustDecimal("500"), decimal.Zero)

	if !dd.IsZero() {
		t.Errorf("DrawdownPercent with zero peak: got %s, want 0", dd)
	}
}

func TestCalculateDrawdownPercent_BalanceAtPeak(t *testing.T) {
	dd := CalculateDrawdownPercent(mustDecimal("1000"), mustDecimal("1000"))

	if !dd.IsZero() {
		t.Errorf("DrawdownPercent at peak: got %s, want 0", dd)
	}
}

func TestCalculateDrawdownPercent_NegativePeak(t *testing.T) {
	// Negative peak makes no financial sense — should return 0 rather than panic.
	dd := CalculateDrawdownPercent(mustDecimal("-100"), mustDecimal("-50"))

	if !dd.IsZero() {
		t.Errorf("DrawdownPercent with negative peak: got %s, want 0", dd)
	}
}
