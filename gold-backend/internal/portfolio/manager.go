package portfolio

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
	"github.com/shopspring/decimal"
)

// pnlSeriesCap is the maximum number of per-trade P&L entries retained for Sharpe Ratio
// computation. 252 corresponds to one trading year of daily data.
const pnlSeriesCap = 252

// MetricsCache abstracts cache writes and reads for portfolio metrics.
// *redis.CacheClient satisfies this interface.
type MetricsCache interface {
	SetPortfolioMetrics(ctx context.Context, metrics domain.PortfolioMetrics) error
	GetPortfolioMetrics(ctx context.Context) (*domain.PortfolioMetrics, error)
}

// ManagerConfig holds all dependencies and tuning parameters for the Manager.
type ManagerConfig struct {
	PortfolioRepository postgres.PortfolioRepository
	PositionRepository  postgres.PositionRepository
	Cache               MetricsCache
	// InitialBalance is the starting balance used when no prior snapshot exists.
	InitialBalance decimal.Decimal
	// MaxDrawdownPercent is the circuit breaker threshold. When current drawdown
	// reaches or exceeds this value, IsCircuitBreakerActive returns true.
	MaxDrawdownPercent decimal.Decimal
	// SnapshotInterval controls how often the manager persists a snapshot to Postgres.
	// Defaults to 5 minutes if zero.
	SnapshotInterval time.Duration
	Logger           *slog.Logger
}

// managerState is the in-memory rolling portfolio state protected by Manager.mu.
type managerState struct {
	balance            decimal.Decimal
	peakBalance        decimal.Decimal
	maxDrawdownPercent decimal.Decimal // historical high-water mark for drawdown
	pnlSeries          []decimal.Decimal
	aggregateStats     AggregateStats
}

// Manager maintains real-time portfolio state, enforces the drawdown circuit breaker,
// and periodically persists snapshots to Postgres.
type Manager struct {
	config ManagerConfig
	state  *managerState
	mu     sync.RWMutex
}

// NewManager constructs a Manager. Call Bootstrap before Run.
func NewManager(config ManagerConfig) *Manager {
	if config.SnapshotInterval == 0 {
		config.SnapshotInterval = 5 * time.Minute
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	return &Manager{
		config: config,
		state:  &managerState{},
	}
}

// Bootstrap loads the latest portfolio snapshot from Postgres (or starts fresh from
// InitialBalance), then seeds pnlSeries with the most recent closed positions.
// Call once on startup before Run.
func (manager *Manager) Bootstrap(ctx context.Context) error {
	snapshot, err := manager.config.PortfolioRepository.FindLatestSnapshot(ctx)
	if err != nil {
		return err
	}

	state := &managerState{}

	if snapshot == nil {
		manager.config.Logger.InfoContext(ctx, "no portfolio snapshot found, starting fresh",
			"initial_balance", manager.config.InitialBalance)
		state.balance = manager.config.InitialBalance
		state.peakBalance = manager.config.InitialBalance
	} else {
		manager.config.Logger.InfoContext(ctx, "loaded portfolio snapshot",
			"snapshot_id", snapshot.ID,
			"balance", snapshot.Balance,
			"peak_balance", snapshot.PeakBalance,
			"total_trades", snapshot.TotalTrades,
		)
		state.balance = snapshot.Balance
		state.peakBalance = snapshot.PeakBalance
		state.maxDrawdownPercent = snapshot.MaxDrawdownPercent
		state.aggregateStats = AggregateStats{
			TotalPnl:    snapshot.TotalPnl,
			WinCount:    snapshot.WinCount,
			LossCount:   snapshot.LossCount,
			TotalTrades: snapshot.TotalTrades,
		}
		// SumOfWins and SumOfLosses are not stored in the snapshot directly.
		// We reconstruct them from AverageWin/AverageLoss * counts.
		if snapshot.WinCount > 0 {
			state.aggregateStats.SumOfWins = snapshot.AverageWin.
				Mul(decimal.NewFromInt(int64(snapshot.WinCount)))
		}
		if snapshot.LossCount > 0 {
			state.aggregateStats.SumOfLosses = snapshot.AverageLoss.
				Mul(decimal.NewFromInt(int64(snapshot.LossCount)))
		}
	}

	// Seed pnlSeries from recent closed positions (up to pnlSeriesCap).
	closedPositions, err := manager.config.PositionRepository.FindClosedPositions(ctx, pnlSeriesCap, 0)
	if err != nil {
		return err
	}

	// FindClosedPositions returns DESC by closed_at; reverse so pnlSeries is chronological.
	pnlSeries := make([]decimal.Decimal, 0, len(closedPositions))
	for i := len(closedPositions) - 1; i >= 0; i-- {
		p := closedPositions[i]
		if p.Status == "closed" && !p.RealizedPnl.IsZero() {
			pnlSeries = append(pnlSeries, p.RealizedPnl)
		}
	}
	state.pnlSeries = pnlSeries

	manager.mu.Lock()
	manager.state = state
	manager.mu.Unlock()

	manager.config.Logger.InfoContext(ctx, "portfolio manager bootstrapped",
		"balance", state.balance,
		"peak_balance", state.peakBalance,
		"pnl_series_len", len(state.pnlSeries),
	)
	return nil
}

// RecordPositionClose updates the portfolio state with the P&L from a newly-closed position.
// This is the primary mutation path called by the position monitor on every close event.
// Cache writes are best-effort: failures are logged but not propagated.
func (manager *Manager) RecordPositionClose(ctx context.Context, position domain.Position) error {
	if position.RealizedPnl.IsZero() {
		manager.config.Logger.InfoContext(ctx, "skipping position close with zero realized P&L",
			"position_id", position.ID,
			"symbol", position.Symbol,
		)
		return nil
	}

	manager.mu.Lock()

	pnl := position.RealizedPnl

	// Update aggregate stats.
	manager.state.aggregateStats.TotalTrades++
	manager.state.aggregateStats.TotalPnl = manager.state.aggregateStats.TotalPnl.Add(pnl)
	if pnl.IsPositive() {
		manager.state.aggregateStats.WinCount++
		manager.state.aggregateStats.SumOfWins = manager.state.aggregateStats.SumOfWins.Add(pnl)
	} else if pnl.IsNegative() {
		manager.state.aggregateStats.LossCount++
		manager.state.aggregateStats.SumOfLosses = manager.state.aggregateStats.SumOfLosses.Add(pnl.Neg())
	}

	// Append to pnlSeries and cap at pnlSeriesCap (FIFO).
	manager.state.pnlSeries = append(manager.state.pnlSeries, pnl)
	if len(manager.state.pnlSeries) > pnlSeriesCap {
		manager.state.pnlSeries = manager.state.pnlSeries[len(manager.state.pnlSeries)-pnlSeriesCap:]
	}

	// Update balance and peak.
	manager.state.balance = manager.state.balance.Add(pnl)
	if manager.state.balance.GreaterThan(manager.state.peakBalance) {
		manager.state.peakBalance = manager.state.balance
	}

	// Compute and track maximum drawdown.
	currentDrawdown := CalculateDrawdownPercent(manager.state.balance, manager.state.peakBalance)
	if currentDrawdown.GreaterThan(manager.state.maxDrawdownPercent) {
		manager.state.maxDrawdownPercent = currentDrawdown
	}

	// Determine circuit breaker state.
	circuitBreakerActive := currentDrawdown.GreaterThanOrEqual(manager.config.MaxDrawdownPercent)

	if circuitBreakerActive {
		manager.config.Logger.LogAttrs(ctx, slog.LevelError, "circuit breaker activated",
			slog.String("current_drawdown_percent", currentDrawdown.String()),
			slog.String("max_drawdown_threshold_percent", manager.config.MaxDrawdownPercent.String()),
			slog.String("balance", manager.state.balance.String()),
			slog.String("peak_balance", manager.state.peakBalance.String()),
			slog.Int64("position_id", position.ID),
			slog.String("symbol", position.Symbol),
		)
	}

	// Build metrics snapshot for caching — computed while holding the lock.
	metrics := manager.buildMetricsLocked(currentDrawdown, circuitBreakerActive)

	manager.mu.Unlock()

	// Cache write is best-effort: release the lock first so slow Redis I/O does not
	// block concurrent reads via CurrentMetrics.
	if err := manager.config.Cache.SetPortfolioMetrics(ctx, metrics); err != nil {
		manager.config.Logger.WarnContext(ctx, "failed to cache portfolio metrics after position close",
			"position_id", position.ID,
			"error", err,
		)
	}

	manager.config.Logger.InfoContext(ctx, "portfolio updated",
		"position_id", position.ID,
		"symbol", position.Symbol,
		"realized_pnl", pnl.String(),
		"new_balance", metrics.Balance.String(),
		"drawdown_percent", currentDrawdown.String(),
		"circuit_breaker_active", circuitBreakerActive,
	)

	return nil
}

// CurrentMetrics returns a consistent snapshot of the current portfolio metrics.
func (manager *Manager) CurrentMetrics() domain.PortfolioMetrics {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	currentDrawdown := CalculateDrawdownPercent(manager.state.balance, manager.state.peakBalance)
	circuitBreakerActive := currentDrawdown.GreaterThanOrEqual(manager.config.MaxDrawdownPercent)

	return manager.buildMetricsLocked(currentDrawdown, circuitBreakerActive)
}

// IsCircuitBreakerActive returns true when the current drawdown has reached or
// exceeded the configured MaxDrawdownPercent threshold. The decision engine
// reads this before allowing new trades.
func (manager *Manager) IsCircuitBreakerActive() bool {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	currentDrawdown := CalculateDrawdownPercent(manager.state.balance, manager.state.peakBalance)
	return currentDrawdown.GreaterThanOrEqual(manager.config.MaxDrawdownPercent)
}

// Run periodically persists portfolio snapshots to Postgres on the configured interval.
// Blocks until ctx is cancelled; always returns ctx.Err() on exit.
func (manager *Manager) Run(ctx context.Context) error {
	ticker := time.NewTicker(manager.config.SnapshotInterval)
	defer ticker.Stop()

	manager.config.Logger.InfoContext(ctx, "portfolio snapshot loop started",
		"interval", manager.config.SnapshotInterval,
	)

	for {
		select {
		case <-ctx.Done():
			manager.config.Logger.InfoContext(ctx, "portfolio snapshot loop stopped")
			return ctx.Err()

		case <-ticker.C:
			metrics := manager.CurrentMetrics()
			if _, err := manager.config.PortfolioRepository.InsertSnapshot(ctx, metrics); err != nil {
				manager.config.Logger.ErrorContext(ctx, "failed to persist portfolio snapshot",
					"error", err,
					"balance", metrics.Balance.String(),
				)
				continue
			}
			manager.config.Logger.InfoContext(ctx, "portfolio snapshot persisted",
				"balance", metrics.Balance.String(),
				"drawdown_percent", metrics.DrawdownPercent.String(),
			)
		}
	}
}

// buildMetricsLocked constructs a PortfolioMetrics value from the current state.
// Caller must hold at least a read lock.
func (manager *Manager) buildMetricsLocked(currentDrawdown decimal.Decimal, circuitBreakerActive bool) domain.PortfolioMetrics {
	stats := manager.state.aggregateStats
	return domain.PortfolioMetrics{
		Balance:                manager.state.balance,
		PeakBalance:            manager.state.peakBalance,
		DrawdownPercent:        currentDrawdown,
		MaxDrawdownPercent:     manager.state.maxDrawdownPercent,
		TotalPnl:               stats.TotalPnl,
		WinCount:               stats.WinCount,
		LossCount:              stats.LossCount,
		TotalTrades:            stats.TotalTrades,
		WinRate:                CalculateWinRate(stats),
		ProfitFactor:           CalculateProfitFactor(stats),
		AverageWin:             CalculateAverageWin(stats),
		AverageLoss:            CalculateAverageLoss(stats),
		SharpeRatio:            CalculateSharpeRatio(manager.state.pnlSeries, 252),
		IsCircuitBreakerActive: circuitBreakerActive,
		SnapshotAt:             time.Now().UTC(),
	}
}
