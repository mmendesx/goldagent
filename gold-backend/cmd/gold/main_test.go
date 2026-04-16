package main

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/indicator"
	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/pattern/candlestick"
	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/pattern/chart"
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/engine"
	"github.com/mmendesx/goldagent/gold-backend/internal/market/candle"
)

// ---- In-memory fakes --------------------------------------------------------

// fakeCandleRepo satisfies postgres.CandleRepository without a database.
type fakeCandleRepo struct {
	mu      sync.Mutex
	candles []domain.Candle
	nextID  int64
}

func (r *fakeCandleRepo) InsertCandle(_ context.Context, c domain.Candle) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	c.ID = r.nextID
	r.candles = append(r.candles, c)
	return r.nextID, nil
}

func (r *fakeCandleRepo) InsertCandlesBatch(_ context.Context, candles []domain.Candle) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range candles {
		r.nextID++
		c.ID = r.nextID
		r.candles = append(r.candles, c)
	}
	return nil
}

func (r *fakeCandleRepo) UpsertCandle(_ context.Context, c domain.Candle) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, existing := range r.candles {
		if existing.Symbol == c.Symbol &&
			existing.Interval == c.Interval &&
			existing.OpenTime.Equal(c.OpenTime) {
			r.candles[i] = c
			return existing.ID, nil
		}
	}
	r.nextID++
	c.ID = r.nextID
	r.candles = append(r.candles, c)
	return r.nextID, nil
}

func (r *fakeCandleRepo) FindCandlesByRange(
	_ context.Context,
	symbol, interval string,
	from, to time.Time,
	limit int,
) ([]domain.Candle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var results []domain.Candle
	for _, c := range r.candles {
		if c.Symbol == symbol && c.Interval == interval &&
			!c.OpenTime.Before(from) && !c.OpenTime.After(to) {
			results = append(results, c)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

func (r *fakeCandleRepo) FindCandlesByRangePaginated(
	_ context.Context,
	symbol, interval string,
	from, to time.Time,
	limit, offset int,
) ([]domain.Candle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var results []domain.Candle
	skipped := 0
	for _, c := range r.candles {
		if c.Symbol == symbol && c.Interval == interval &&
			!c.OpenTime.Before(from) && !c.OpenTime.After(to) {
			if skipped < offset {
				skipped++
				continue
			}
			results = append(results, c)
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

func (r *fakeCandleRepo) FindLatestCandles(
	_ context.Context,
	symbol, interval string,
	limit int,
) ([]domain.Candle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Collect matching candles.
	var matching []domain.Candle
	for _, c := range r.candles {
		if c.Symbol == symbol && c.Interval == interval {
			matching = append(matching, c)
		}
	}
	// Return up to limit from the end (newest-first to match real behaviour).
	start := 0
	if len(matching) > limit {
		start = len(matching) - limit
	}
	slice := matching[start:]
	result := make([]domain.Candle, len(slice))
	for i, c := range slice {
		result[len(slice)-1-i] = c
	}
	return result, nil
}

// fakeIndicatorRepo satisfies postgres.IndicatorRepository.
type fakeIndicatorRepo struct {
	mu         sync.Mutex
	indicators []domain.Indicator
	nextID     int64
}

func (r *fakeIndicatorRepo) InsertIndicator(_ context.Context, ind domain.Indicator) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	ind.ID = r.nextID
	r.indicators = append(r.indicators, ind)
	return r.nextID, nil
}

func (r *fakeIndicatorRepo) FindLatestIndicator(
	_ context.Context,
	symbol, interval string,
) (*domain.Indicator, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var latest *domain.Indicator
	for i := range r.indicators {
		ind := &r.indicators[i]
		if ind.Symbol == symbol && ind.Interval == interval {
			if latest == nil || ind.Timestamp.After(latest.Timestamp) {
				latest = ind
			}
		}
	}
	if latest == nil {
		return nil, nil
	}
	copy := *latest
	return &copy, nil
}

func (r *fakeIndicatorRepo) FindIndicatorsByRange(
	_ context.Context,
	symbol, interval string,
	from, to time.Time,
) ([]domain.Indicator, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var results []domain.Indicator
	for _, ind := range r.indicators {
		if ind.Symbol == symbol && ind.Interval == interval &&
			!ind.Timestamp.Before(from) && !ind.Timestamp.After(to) {
			results = append(results, ind)
		}
	}
	return results, nil
}

// fakeDecisionRepo satisfies postgres.DecisionRepository and records inserted decisions.
type fakeDecisionRepo struct {
	mu        sync.Mutex
	decisions []domain.Decision
	nextID    int64
}

func (r *fakeDecisionRepo) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.decisions)
}

func (r *fakeDecisionRepo) InsertDecision(_ context.Context, d domain.Decision) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	d.ID = r.nextID
	r.decisions = append(r.decisions, d)
	return r.nextID, nil
}

func (r *fakeDecisionRepo) UpdateDecisionExecutionStatus(
	_ context.Context,
	id int64,
	status domain.DecisionExecutionStatus,
	rejectionReason string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.decisions {
		if r.decisions[i].ID == id {
			r.decisions[i].ExecutionStatus = status
			r.decisions[i].RejectionReason = rejectionReason
			return nil
		}
	}
	return nil
}

func (r *fakeDecisionRepo) FindDecisionsBySymbol(
	_ context.Context,
	symbol string,
	limit, offset int,
) ([]domain.Decision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var results []domain.Decision
	for _, d := range r.decisions {
		if d.Symbol == symbol {
			results = append(results, d)
		}
	}
	return results, nil
}

func (r *fakeDecisionRepo) FindRecentDecisions(
	_ context.Context,
	limit, offset int,
) ([]domain.Decision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]domain.Decision{}, r.decisions...), nil
}

// fakePositionRepo satisfies postgres.PositionRepository.
type fakePositionRepo struct {
	mu        sync.Mutex
	positions []domain.Position
	nextID    int64
}

func (r *fakePositionRepo) InsertPosition(_ context.Context, p domain.Position) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	p.ID = r.nextID
	r.positions = append(r.positions, p)
	return r.nextID, nil
}

func (r *fakePositionRepo) UpdatePositionTrailingStop(
	_ context.Context,
	id int64,
	price decimal.Decimal,
) error {
	return nil
}

func (r *fakePositionRepo) ClosePosition(
	_ context.Context,
	id int64,
	exitOrderID int64,
	exitPrice, realizedPnl decimal.Decimal,
	closeReason string,
) error {
	return nil
}

func (r *fakePositionRepo) FindOpenPositions(_ context.Context) ([]domain.Position, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var open []domain.Position
	for _, p := range r.positions {
		if p.Status == "open" {
			open = append(open, p)
		}
	}
	return open, nil
}

func (r *fakePositionRepo) CountOpenPositions(_ context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	for _, p := range r.positions {
		if p.Status == "open" {
			count++
		}
	}
	return count, nil
}

func (r *fakePositionRepo) FindClosedPositions(
	_ context.Context,
	limit, offset int,
) ([]domain.Position, error) {
	return nil, nil
}

func (r *fakePositionRepo) FindPositionByID(
	_ context.Context,
	id int64,
) (*domain.Position, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.positions {
		if r.positions[i].ID == id {
			copy := r.positions[i]
			return &copy, nil
		}
	}
	return nil, nil
}

// fakePortfolioRepo satisfies postgres.PortfolioRepository.
type fakePortfolioRepo struct{}

func (r *fakePortfolioRepo) InsertSnapshot(
	_ context.Context,
	metrics domain.PortfolioMetrics,
) (int64, error) {
	return 1, nil
}

func (r *fakePortfolioRepo) FindLatestSnapshot(_ context.Context) (*domain.PortfolioMetrics, error) {
	return nil, nil
}

func (r *fakePortfolioRepo) FindSnapshotsByRange(
	_ context.Context,
	from, to time.Time,
) ([]domain.PortfolioMetrics, error) {
	return nil, nil
}

// fakeMetricsCache satisfies portfolio.MetricsCache and engine.PortfolioMetricsReader.
type fakeMetricsCache struct {
	mu      sync.Mutex
	metrics *domain.PortfolioMetrics
}

func (c *fakeMetricsCache) SetPortfolioMetrics(_ context.Context, m domain.PortfolioMetrics) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	copy := m
	c.metrics = &copy
	return nil
}

func (c *fakeMetricsCache) GetPortfolioMetrics(_ context.Context) (*domain.PortfolioMetrics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.metrics == nil {
		return nil, nil
	}
	copy := *c.metrics
	return &copy, nil
}

// fakeCandleCache satisfies candle.CandleCache.
type fakeCandleCache struct {
	mu      sync.Mutex
	candles map[string]domain.Candle
}

func newFakeCandleCache() *fakeCandleCache {
	return &fakeCandleCache{candles: make(map[string]domain.Candle)}
}

func (c *fakeCandleCache) SetLatestCandle(_ context.Context, candle domain.Candle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := candle.Symbol + ":" + candle.Interval
	c.candles[key] = candle
	return nil
}

// ---- Helpers ----------------------------------------------------------------

// buildCannedCandles returns a sequence of closed candles for pipeline testing.
// Generates count candles with small price variation to allow indicator computation.
func buildCannedCandles(symbol, interval string, count int) []domain.Candle {
	candles := make([]domain.Candle, count)
	base := decimal.NewFromFloat(50000.0)
	now := time.Now().UTC().Truncate(5 * time.Minute)

	for i := 0; i < count; i++ {
		offset := decimal.NewFromInt(int64(i * 10))
		open := base.Add(offset)
		close := open.Add(decimal.NewFromFloat(5.0))
		high := close.Add(decimal.NewFromFloat(10.0))
		low := open.Sub(decimal.NewFromFloat(5.0))

		candles[i] = domain.Candle{
			Symbol:      symbol,
			Interval:    interval,
			OpenTime:    now.Add(time.Duration(i) * 5 * time.Minute),
			CloseTime:   now.Add(time.Duration(i+1) * 5 * time.Minute),
			OpenPrice:   open,
			ClosePrice:  close,
			HighPrice:   high,
			LowPrice:    low,
			Volume:      decimal.NewFromFloat(100.0),
			QuoteVolume: decimal.NewFromFloat(5000000.0),
			TradeCount:  1000,
			IsClosed:    true,
		}
	}
	return candles
}

// ---- Test -------------------------------------------------------------------

// TestPipelineWiring verifies that when closed candles flow through the aggregator,
// they are persisted and the decision engine processes at least one decision.
// Uses in-memory fakes for all external dependencies — no Postgres or Redis required.
//
// Scenario:
//
//	Given: an aggregator, indicator computer, and decision engine wired together
//	When:  a sequence of closed candles is pushed through the pipeline
//	Then:  candles are persisted, indicators are computed, and at least one decision is logged
//	And:   in dry-run mode, zero positions are opened
func TestPipelineWiring(t *testing.T) {
	t.Parallel()

	const (
		symbol   = "BTCUSDT"
		interval = "5m"
		// Feed enough candles to warm up the indicator computer (200 EMA needs 200 candles).
		candleCount = 210
	)

	// --- Fakes ---
	candleRepo := &fakeCandleRepo{}
	indicatorRepo := &fakeIndicatorRepo{}
	decisionRepo := &fakeDecisionRepo{}
	positionRepo := &fakePositionRepo{}
	_ = &fakePortfolioRepo{} // satisfies postgres.PortfolioRepository; not used directly here
	metricsCache := &fakeMetricsCache{}
	candleCache := newFakeCandleCache()

	// Set initial metrics so the engine has a balance to work with.
	_ = metricsCache.SetPortfolioMetrics(context.Background(), domain.PortfolioMetrics{
		Balance:     decimal.NewFromFloat(10000.0),
		PeakBalance: decimal.NewFromFloat(10000.0),
	})

	// --- Input channel: test-controlled ---
	inputCh := make(chan domain.Candle, 256)

	// --- Aggregator ---
	aggregator := candle.NewAggregator(candle.AggregatorConfig{
		InputChannel:       inputCh,
		PostgresRepository: candleRepo,
		RedisCache:         candleCache,
		Logger:             newTestLogger(),
	})

	// Fan-out: indicator computer + orchestrator each need their own channel.
	indicatorCh := make(chan domain.Candle, 256)
	orchestratorCh := make(chan domain.Candle, 256)

	// --- Indicator computer ---
	computer := indicator.NewComputer(indicator.ComputerConfig{
		InputChannel:        indicatorCh,
		CandleRepository:    candleRepo,
		IndicatorRepository: indicatorRepo,
		RsiPeriod:           14,
		MacdFast:            12,
		MacdSlow:            26,
		MacdSignalPeriod:    9,
		BollingerPeriod:     20,
		BollingerStdDev:     2.0,
		EmaPeriods:          []int{9, 21, 50, 200},
		AtrPeriod:           14,
		HistoryLimit:        300,
		Logger:              newTestLogger(),
	})

	// --- Pattern detectors ---
	detector := candlestick.NewDetector()
	analyzer := chart.NewAnalyzer(chart.AnalyzerConfig{})

	// --- Decision engine (dry-run = true → no positions opened) ---
	eng := engine.NewDecisionEngine(engine.DecisionEngineConfig{
		DecisionRepository:        decisionRepo,
		PositionRepository:        positionRepo,
		PortfolioCache:            metricsCache,
		ConfidenceThreshold:       70,
		MaxOpenPositions:          3,
		MaxDrawdownPercent:        decimal.NewFromFloat(15.0),
		SentimentWeight:           0.3,
		PositionSizePercent:       decimal.NewFromFloat(10.0),
		TakeProfitAtrMultiplier:   decimal.NewFromFloat(2.0),
		StopLossAtrMultiplier:     decimal.NewFromFloat(1.0),
		TrailingStopAtrMultiplier: decimal.NewFromFloat(1.0),
		InitialBalance:            decimal.NewFromFloat(10000.0),
		IsDryRunEnabled:           true,
		Logger:                    newTestLogger(),
	})

	// --- Run pipeline components in goroutines ---
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	// Fan-out goroutine: distributes aggregator's closed-candle channel to both consumers.
	wg.Add(1)
	go func() {
		defer wg.Done()
		fanOutCandles(ctx, aggregator.ClosedCandleChannel(), []chan<- domain.Candle{
			indicatorCh,
			orchestratorCh,
		})
	}()

	// Aggregator
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = aggregator.Run(ctx)
	}()

	// Indicator computer
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = computer.Run(ctx)
	}()

	// Decision orchestrator — track whether it has produced at least one decision.
	var decisionsProcessed atomic.Int32

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case c, ok := <-orchestratorCh:
				if !ok {
					return
				}
				history, _ := candleRepo.FindLatestCandles(ctx, c.Symbol, c.Interval, 300)
				reverseCandles(history)

				// Wait briefly for the indicator computer to persist the indicator.
				var ind *domain.Indicator
				for attempt := 0; attempt < 5; attempt++ {
					ind, _ = indicatorRepo.FindLatestIndicator(ctx, c.Symbol, c.Interval)
					if ind != nil {
						break
					}
					time.Sleep(20 * time.Millisecond)
				}
				if ind == nil {
					continue
				}

				patterns := detector.DetectPatterns(history)
				chartResult := analyzer.Analyze(history)

				input := engine.AnalysisInput{
					Symbol:          c.Symbol,
					LatestIndicator: *ind,
					LatestCandle:    c,
					Patterns:        patterns,
					ChartAnalysis:   chartResult,
					Sentiment:       decimal.Zero,
				}

				if err := eng.ProcessAnalysisInput(ctx, input); err == nil {
					decisionsProcessed.Add(1)
				}
			}
		}
	}()

	// --- Feed canned candles into the pipeline ---
	candles := buildCannedCandles(symbol, interval, candleCount)
	for _, c := range candles {
		select {
		case inputCh <- c:
		case <-ctx.Done():
			t.Fatal("context expired before all candles were fed")
		}
	}
	close(inputCh)

	// Wait for pipeline to drain (up to 15 seconds).
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer waitCancel()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-waitCtx.Done():
		cancel() // unblock remaining goroutines
		t.Fatal("pipeline did not drain within 15 seconds")
	}

	// --- Assertions ---

	// Candles persisted to the fake store.
	if candleRepo.nextID == 0 {
		t.Error("expected at least one candle to be persisted, got 0")
	}
	t.Logf("candles persisted: %d", candleRepo.nextID)

	// Indicators computed and stored.
	if indicatorRepo.nextID == 0 {
		t.Error("expected at least one indicator to be computed and stored, got 0")
	}
	t.Logf("indicators computed: %d", indicatorRepo.nextID)

	// At least one decision logged.
	decisionsLogged := decisionRepo.count()
	if decisionsLogged == 0 {
		t.Error("expected at least one decision to be logged, got 0")
	}
	t.Logf("decisions logged: %d", decisionsLogged)

	// In dry-run mode: zero positions opened.
	openPositions, _ := positionRepo.FindOpenPositions(context.Background())
	if len(openPositions) != 0 {
		t.Errorf("expected 0 open positions in dry-run mode, got %d", len(openPositions))
	}
}

// newTestLogger returns a discard logger to keep test output clean.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}
