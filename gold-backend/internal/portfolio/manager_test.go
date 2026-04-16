package portfolio

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// --- Fakes ---

// fakePortfolioRepository is an in-memory PortfolioRepository for tests.
type fakePortfolioRepository struct {
	mu        sync.Mutex
	snapshots []domain.PortfolioMetrics
}

func (r *fakePortfolioRepository) InsertSnapshot(_ context.Context, m domain.PortfolioMetrics) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snapshots = append(r.snapshots, m)
	return int64(len(r.snapshots)), nil
}

func (r *fakePortfolioRepository) FindLatestSnapshot(_ context.Context) (*domain.PortfolioMetrics, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.snapshots) == 0 {
		return nil, nil
	}
	last := r.snapshots[len(r.snapshots)-1]
	return &last, nil
}

func (r *fakePortfolioRepository) FindSnapshotsByRange(_ context.Context, _, _ time.Time) ([]domain.PortfolioMetrics, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.PortfolioMetrics, len(r.snapshots))
	copy(out, r.snapshots)
	return out, nil
}

// fakePositionRepository returns a fixed slice of closed positions.
type fakePositionRepository struct {
	closed []domain.Position
}

func (r *fakePositionRepository) FindClosedPositions(_ context.Context, limit, offset int) ([]domain.Position, error) {
	end := offset + limit
	if end > len(r.closed) {
		end = len(r.closed)
	}
	if offset >= len(r.closed) {
		return nil, nil
	}
	return r.closed[offset:end], nil
}

// Unused interface methods — needed to satisfy postgres.PositionRepository.
func (r *fakePositionRepository) InsertPosition(_ context.Context, _ domain.Position) (int64, error) {
	return 0, nil
}
func (r *fakePositionRepository) UpdatePositionTrailingStop(_ context.Context, _ int64, _ decimal.Decimal) error {
	return nil
}
func (r *fakePositionRepository) ClosePosition(_ context.Context, _ int64, _ int64, _, _ decimal.Decimal, _ string) error {
	return nil
}
func (r *fakePositionRepository) FindOpenPositions(_ context.Context) ([]domain.Position, error) {
	return nil, nil
}
func (r *fakePositionRepository) CountOpenPositions(_ context.Context) (int, error) {
	return 0, nil
}
func (r *fakePositionRepository) FindPositionByID(_ context.Context, _ int64) (*domain.Position, error) {
	return nil, nil
}

// fakeCache is an in-memory MetricsCache for tests.
type fakeCache struct {
	mu      sync.Mutex
	metrics *domain.PortfolioMetrics
}

func (c *fakeCache) SetPortfolioMetrics(_ context.Context, m domain.PortfolioMetrics) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics = &m
	return nil
}

func (c *fakeCache) GetPortfolioMetrics(_ context.Context) (*domain.PortfolioMetrics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.metrics, nil
}

// --- Helpers ---

func buildManager(portfolioRepo *fakePortfolioRepository, positionRepo *fakePositionRepository, cache *fakeCache, initialBalance string, maxDrawdown string) *Manager {
	return NewManager(ManagerConfig{
		PortfolioRepository: portfolioRepo,
		PositionRepository:  positionRepo,
		Cache:               cache,
		InitialBalance:      mustDecimal(initialBalance),
		MaxDrawdownPercent:  mustDecimal(maxDrawdown),
		SnapshotInterval:    time.Hour, // long interval — tests drive snapshots explicitly
	})
}

func closedPosition(id int64, pnl string) domain.Position {
	t := time.Now()
	return domain.Position{
		ID:          id,
		Symbol:      "BTCUSDT",
		Status:      "closed",
		RealizedPnl: mustDecimal(pnl),
		ClosedAt:    &t,
	}
}

// --- Tests ---

// S-21: Bootstrap with no snapshot uses InitialBalance.
func TestBootstrap_NoSnapshot_UsesInitialBalance(t *testing.T) {
	ctx := context.Background()
	portfolioRepo := &fakePortfolioRepository{}
	positionRepo := &fakePositionRepository{}
	cache := &fakeCache{}

	manager := buildManager(portfolioRepo, positionRepo, cache, "10000", "15")
	if err := manager.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	metrics := manager.CurrentMetrics()
	if !metrics.Balance.Equal(mustDecimal("10000")) {
		t.Errorf("Balance: got %s, want 10000", metrics.Balance)
	}
	if !metrics.PeakBalance.Equal(mustDecimal("10000")) {
		t.Errorf("PeakBalance: got %s, want 10000", metrics.PeakBalance)
	}
	if metrics.TotalTrades != 0 {
		t.Errorf("TotalTrades: got %d, want 0", metrics.TotalTrades)
	}
}

// S-21: Bootstrap with an existing snapshot restores state.
func TestBootstrap_WithSnapshot_RestoresState(t *testing.T) {
	ctx := context.Background()

	existing := domain.PortfolioMetrics{
		ID:                 1,
		Balance:            mustDecimal("9500"),
		PeakBalance:        mustDecimal("10000"),
		MaxDrawdownPercent: mustDecimal("5"),
		TotalPnl:           mustDecimal("-500"),
		WinCount:           3,
		LossCount:          2,
		TotalTrades:        5,
		AverageWin:         mustDecimal("200"),
		AverageLoss:        mustDecimal("400"),
		SnapshotAt:         time.Now(),
	}
	portfolioRepo := &fakePortfolioRepository{snapshots: []domain.PortfolioMetrics{existing}}
	positionRepo := &fakePositionRepository{}
	cache := &fakeCache{}

	manager := buildManager(portfolioRepo, positionRepo, cache, "10000", "15")
	if err := manager.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	metrics := manager.CurrentMetrics()
	if !metrics.Balance.Equal(mustDecimal("9500")) {
		t.Errorf("Balance: got %s, want 9500", metrics.Balance)
	}
	if !metrics.PeakBalance.Equal(mustDecimal("10000")) {
		t.Errorf("PeakBalance: got %s, want 10000", metrics.PeakBalance)
	}
	if metrics.TotalTrades != 5 {
		t.Errorf("TotalTrades: got %d, want 5", metrics.TotalTrades)
	}
}

// S-21: RecordPositionClose with a profit updates balance and peak.
func TestRecordPositionClose_Profit_UpdatesBalanceAndPeak(t *testing.T) {
	ctx := context.Background()
	portfolioRepo := &fakePortfolioRepository{}
	positionRepo := &fakePositionRepository{}
	cache := &fakeCache{}

	manager := buildManager(portfolioRepo, positionRepo, cache, "10000", "15")
	if err := manager.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	pos := closedPosition(1, "500")
	if err := manager.RecordPositionClose(ctx, pos); err != nil {
		t.Fatalf("RecordPositionClose: %v", err)
	}

	metrics := manager.CurrentMetrics()
	if !metrics.Balance.Equal(mustDecimal("10500")) {
		t.Errorf("Balance: got %s, want 10500", metrics.Balance)
	}
	if !metrics.PeakBalance.Equal(mustDecimal("10500")) {
		t.Errorf("PeakBalance: got %s, want 10500 (peak should update with balance)", metrics.PeakBalance)
	}
	if !metrics.DrawdownPercent.IsZero() {
		t.Errorf("DrawdownPercent: got %s, want 0 (balance equals peak)", metrics.DrawdownPercent)
	}
	if metrics.TotalTrades != 1 {
		t.Errorf("TotalTrades: got %d, want 1", metrics.TotalTrades)
	}
	if metrics.WinCount != 1 {
		t.Errorf("WinCount: got %d, want 1", metrics.WinCount)
	}
}

// S-22: RecordPositionClose with a loss after prior wins computes drawdown correctly.
func TestRecordPositionClose_LossAfterWins_ComputesDrawdown(t *testing.T) {
	ctx := context.Background()
	portfolioRepo := &fakePortfolioRepository{}
	positionRepo := &fakePositionRepository{}
	cache := &fakeCache{}

	manager := buildManager(portfolioRepo, positionRepo, cache, "10000", "15")
	if err := manager.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	// Win: balance goes to 11000, peak = 11000.
	if err := manager.RecordPositionClose(ctx, closedPosition(1, "1000")); err != nil {
		t.Fatalf("RecordPositionClose win: %v", err)
	}

	// Loss: balance drops to 10000, peak still 11000.
	// Drawdown = (11000 - 10000) / 11000 * 100 ≈ 9.0909...
	if err := manager.RecordPositionClose(ctx, closedPosition(2, "-1000")); err != nil {
		t.Fatalf("RecordPositionClose loss: %v", err)
	}

	metrics := manager.CurrentMetrics()
	if !metrics.Balance.Equal(mustDecimal("10000")) {
		t.Errorf("Balance: got %s, want 10000", metrics.Balance)
	}
	if !metrics.PeakBalance.Equal(mustDecimal("11000")) {
		t.Errorf("PeakBalance: got %s, want 11000", metrics.PeakBalance)
	}

	expectedDrawdown := CalculateDrawdownPercent(mustDecimal("10000"), mustDecimal("11000"))
	if !metrics.DrawdownPercent.Equal(expectedDrawdown) {
		t.Errorf("DrawdownPercent: got %s, want %s", metrics.DrawdownPercent, expectedDrawdown)
	}
	if metrics.LossCount != 1 {
		t.Errorf("LossCount: got %d, want 1", metrics.LossCount)
	}
}

// S-22: IsCircuitBreakerActive is false below threshold and true at threshold.
func TestIsCircuitBreakerActive_BelowAndAtThreshold(t *testing.T) {
	ctx := context.Background()
	portfolioRepo := &fakePortfolioRepository{}
	positionRepo := &fakePositionRepository{}
	cache := &fakeCache{}

	// threshold = 10%
	manager := buildManager(portfolioRepo, positionRepo, cache, "1000", "10")
	if err := manager.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	// Win: peak becomes 1100.
	if err := manager.RecordPositionClose(ctx, closedPosition(1, "100")); err != nil {
		t.Fatalf("RecordPositionClose: %v", err)
	}

	// Small loss — drawdown 5%: balance=1045, peak=1100.
	if err := manager.RecordPositionClose(ctx, closedPosition(2, "-55")); err != nil {
		t.Fatalf("RecordPositionClose: %v", err)
	}

	if manager.IsCircuitBreakerActive() {
		t.Error("IsCircuitBreakerActive: got true, want false at ~5% drawdown")
	}

	// Large loss — push drawdown to exactly 10%: need balance = 990, which means
	// additional loss = 1045 - 990 = 55. Drawdown = (1100-990)/1100 = 10%.
	if err := manager.RecordPositionClose(ctx, closedPosition(3, "-55")); err != nil {
		t.Fatalf("RecordPositionClose: %v", err)
	}

	if !manager.IsCircuitBreakerActive() {
		metrics := manager.CurrentMetrics()
		t.Errorf("IsCircuitBreakerActive: got false, want true at 10%% drawdown (drawdown=%s)", metrics.DrawdownPercent)
	}
}

// S-23: CurrentMetrics returns a consistent snapshot of all derived metrics.
func TestCurrentMetrics_ConsistentSnapshot(t *testing.T) {
	ctx := context.Background()
	portfolioRepo := &fakePortfolioRepository{}
	positionRepo := &fakePositionRepository{}
	cache := &fakeCache{}

	manager := buildManager(portfolioRepo, positionRepo, cache, "1000", "20")
	if err := manager.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	positions := []domain.Position{
		closedPosition(1, "100"),
		closedPosition(2, "200"),
		closedPosition(3, "-50"),
	}
	for _, p := range positions {
		if err := manager.RecordPositionClose(ctx, p); err != nil {
			t.Fatalf("RecordPositionClose: %v", err)
		}
	}

	metrics := manager.CurrentMetrics()

	if metrics.TotalTrades != 3 {
		t.Errorf("TotalTrades: got %d, want 3", metrics.TotalTrades)
	}
	if metrics.WinCount != 2 {
		t.Errorf("WinCount: got %d, want 2", metrics.WinCount)
	}
	if metrics.LossCount != 1 {
		t.Errorf("LossCount: got %d, want 1", metrics.LossCount)
	}

	expectedBalance := mustDecimal("1250") // 1000 + 100 + 200 - 50
	if !metrics.Balance.Equal(expectedBalance) {
		t.Errorf("Balance: got %s, want 1250", metrics.Balance)
	}

	// Win rate: 2/3 * 100 ≈ 66.666...
	expectedWinRate := decimal.NewFromInt(2).Div(decimal.NewFromInt(3)).Mul(decimal.NewFromInt(100))
	if !metrics.WinRate.Equal(expectedWinRate) {
		t.Errorf("WinRate: got %s, want %s", metrics.WinRate, expectedWinRate)
	}

	// Profit factor: 300 / 50 = 6
	if !metrics.ProfitFactor.Equal(mustDecimal("6")) {
		t.Errorf("ProfitFactor: got %s, want 6", metrics.ProfitFactor)
	}

	// Average win: 300 / 2 = 150
	if !metrics.AverageWin.Equal(mustDecimal("150")) {
		t.Errorf("AverageWin: got %s, want 150", metrics.AverageWin)
	}

	// Average loss: 50 / 1 = 50
	if !metrics.AverageLoss.Equal(mustDecimal("50")) {
		t.Errorf("AverageLoss: got %s, want 50", metrics.AverageLoss)
	}

	// Cache should have been updated.
	cached, err := cache.GetPortfolioMetrics(ctx)
	if err != nil {
		t.Fatalf("GetPortfolioMetrics: %v", err)
	}
	if cached == nil {
		t.Fatal("expected cached metrics, got nil")
	}
	if !cached.Balance.Equal(expectedBalance) {
		t.Errorf("cached Balance: got %s, want 1250", cached.Balance)
	}
}

// S-22: Concurrent RecordPositionClose calls do not corrupt state.
// Run with -race to detect data races.
func TestRecordPositionClose_Concurrent_NoRaceCondition(t *testing.T) {
	ctx := context.Background()
	portfolioRepo := &fakePortfolioRepository{}
	positionRepo := &fakePositionRepository{}
	cache := &fakeCache{}

	manager := buildManager(portfolioRepo, positionRepo, cache, "10000", "50")
	if err := manager.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int64) {
			defer wg.Done()
			pos := closedPosition(id, "10")
			if err := manager.RecordPositionClose(ctx, pos); err != nil {
				t.Errorf("RecordPositionClose goroutine %d: %v", id, err)
			}
		}(int64(i + 1))
	}

	wg.Wait()

	metrics := manager.CurrentMetrics()
	if metrics.TotalTrades != goroutines {
		t.Errorf("TotalTrades: got %d, want %d after concurrent updates", metrics.TotalTrades, goroutines)
	}

	expectedBalance := mustDecimal("10000").Add(mustDecimal("10").Mul(decimal.NewFromInt(goroutines)))
	if !metrics.Balance.Equal(expectedBalance) {
		t.Errorf("Balance: got %s, want %s after concurrent updates", metrics.Balance, expectedBalance)
	}
}

// S-21: Bootstrap seeds pnlSeries from existing closed positions.
func TestBootstrap_SeedsPnlSeriesFromClosedPositions(t *testing.T) {
	ctx := context.Background()

	// Two recent closed positions stored in repo, returned DESC by closed_at.
	closed := []domain.Position{
		closedPosition(2, "200"),
		closedPosition(1, "100"),
	}
	portfolioRepo := &fakePortfolioRepository{}
	positionRepo := &fakePositionRepository{closed: closed}
	cache := &fakeCache{}

	manager := buildManager(portfolioRepo, positionRepo, cache, "10000", "15")
	if err := manager.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	// After bootstrap, SharpeRatio should be non-zero (series has ≥2 entries).
	metrics := manager.CurrentMetrics()
	if metrics.SharpeRatio.IsZero() {
		t.Error("SharpeRatio: got 0, want non-zero after seeding 2 entries")
	}
}

// RecordPositionClose with zero P&L is skipped without error.
func TestRecordPositionClose_ZeroPnl_IsSkipped(t *testing.T) {
	ctx := context.Background()
	portfolioRepo := &fakePortfolioRepository{}
	positionRepo := &fakePositionRepository{}
	cache := &fakeCache{}

	manager := buildManager(portfolioRepo, positionRepo, cache, "10000", "15")
	if err := manager.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	pos := closedPosition(1, "0")
	if err := manager.RecordPositionClose(ctx, pos); err != nil {
		t.Fatalf("RecordPositionClose: %v", err)
	}

	metrics := manager.CurrentMetrics()
	if metrics.TotalTrades != 0 {
		t.Errorf("TotalTrades: got %d, want 0 for zero-P&L position", metrics.TotalTrades)
	}
	if !metrics.Balance.Equal(mustDecimal("10000")) {
		t.Errorf("Balance: got %s, want 10000 (unchanged)", metrics.Balance)
	}
}
