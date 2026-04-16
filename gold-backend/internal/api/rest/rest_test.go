package rest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/api/rest"
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

// ---------------------------------------------------------------------------
// Fake repository implementations
// ---------------------------------------------------------------------------

// fakeCandleRepository satisfies postgres.CandleRepository.
type fakeCandleRepository struct {
	candles []domain.Candle
	err     error
}

func (f *fakeCandleRepository) InsertCandle(_ context.Context, _ domain.Candle) (int64, error) {
	return 0, nil
}
func (f *fakeCandleRepository) InsertCandlesBatch(_ context.Context, _ []domain.Candle) error {
	return nil
}
func (f *fakeCandleRepository) UpsertCandle(_ context.Context, _ domain.Candle) (int64, error) {
	return 0, nil
}
func (f *fakeCandleRepository) FindCandlesByRange(_ context.Context, _, _ string, _, _ time.Time, _ int) ([]domain.Candle, error) {
	return f.candles, f.err
}
func (f *fakeCandleRepository) FindCandlesByRangePaginated(_ context.Context, _, _ string, _, _ time.Time, _, _ int) ([]domain.Candle, error) {
	return f.candles, f.err
}
func (f *fakeCandleRepository) FindLatestCandles(_ context.Context, _, _ string, _ int) ([]domain.Candle, error) {
	return f.candles, f.err
}

// fakeIndicatorRepository satisfies postgres.IndicatorRepository.
type fakeIndicatorRepository struct {
	indicators []domain.Indicator
	err        error
}

func (f *fakeIndicatorRepository) InsertIndicator(_ context.Context, _ domain.Indicator) (int64, error) {
	return 0, nil
}
func (f *fakeIndicatorRepository) FindLatestIndicator(_ context.Context, _, _ string) (*domain.Indicator, error) {
	return nil, nil
}
func (f *fakeIndicatorRepository) FindIndicatorsByRange(_ context.Context, _, _ string, _, _ time.Time) ([]domain.Indicator, error) {
	return f.indicators, f.err
}

// fakePositionRepository satisfies postgres.PositionRepository.
type fakePositionRepository struct {
	openPositions   []domain.Position
	closedPositions []domain.Position
	err             error
}

func (f *fakePositionRepository) InsertPosition(_ context.Context, _ domain.Position) (int64, error) {
	return 0, nil
}
func (f *fakePositionRepository) UpdatePositionTrailingStop(_ context.Context, _ int64, _ decimal.Decimal) error {
	return nil
}
func (f *fakePositionRepository) ClosePosition(_ context.Context, _ int64, _ int64, _, _ decimal.Decimal, _ string) error {
	return nil
}
func (f *fakePositionRepository) FindOpenPositions(_ context.Context) ([]domain.Position, error) {
	return f.openPositions, f.err
}
func (f *fakePositionRepository) CountOpenPositions(_ context.Context) (int, error) {
	return len(f.openPositions), nil
}
func (f *fakePositionRepository) FindClosedPositions(_ context.Context, limit, offset int) ([]domain.Position, error) {
	if f.err != nil {
		return nil, f.err
	}
	end := offset + limit
	if offset >= len(f.closedPositions) {
		return []domain.Position{}, nil
	}
	if end > len(f.closedPositions) {
		end = len(f.closedPositions)
	}
	return f.closedPositions[offset:end], nil
}
func (f *fakePositionRepository) FindPositionByID(_ context.Context, _ int64) (*domain.Position, error) {
	return nil, nil
}

// fakeOrderRepository satisfies postgres.OrderRepository.
type fakeOrderRepository struct {
	orders []domain.Order
	err    error
}

func (f *fakeOrderRepository) InsertOrder(_ context.Context, _ domain.Order) (int64, error) {
	return 0, nil
}
func (f *fakeOrderRepository) UpdateOrderStatus(_ context.Context, _ int64, _ domain.OrderStatus, _, _, _ decimal.Decimal, _ string, _ []byte) error {
	return nil
}
func (f *fakeOrderRepository) FindOrderByID(_ context.Context, _ int64) (*domain.Order, error) {
	return nil, nil
}
func (f *fakeOrderRepository) FindOrdersBySymbol(_ context.Context, _ string, _, _ int) ([]domain.Order, error) {
	return f.orders, f.err
}
func (f *fakeOrderRepository) FindRecentOrders(_ context.Context, _, _ int) ([]domain.Order, error) {
	return f.orders, f.err
}

// fakeDecisionRepository satisfies postgres.DecisionRepository.
type fakeDecisionRepository struct {
	decisions []domain.Decision
	err       error
}

func (f *fakeDecisionRepository) InsertDecision(_ context.Context, _ domain.Decision) (int64, error) {
	return 0, nil
}
func (f *fakeDecisionRepository) UpdateDecisionExecutionStatus(_ context.Context, _ int64, _ domain.DecisionExecutionStatus, _ string) error {
	return nil
}
func (f *fakeDecisionRepository) FindDecisionsBySymbol(_ context.Context, _ string, _, _ int) ([]domain.Decision, error) {
	return f.decisions, f.err
}
func (f *fakeDecisionRepository) FindRecentDecisions(_ context.Context, _, _ int) ([]domain.Decision, error) {
	return f.decisions, f.err
}

// fakePriceCache satisfies rest.PriceCache.
type fakePriceCache struct {
	prices map[string]decimal.Decimal
	err    error
}

func (f *fakePriceCache) GetTickerPrice(_ context.Context, symbol string) (*domain.TickerPrice, error) {
	if f.err != nil {
		return nil, f.err
	}
	price, ok := f.prices[symbol]
	if !ok {
		return nil, nil
	}
	return &domain.TickerPrice{Symbol: symbol, Price: price, Timestamp: time.Now()}, nil
}

// fakePortfolioManager satisfies rest.PortfolioMetricsProvider.
type fakePortfolioManager struct {
	metrics domain.PortfolioMetrics
}

func (f *fakePortfolioManager) CurrentMetrics() domain.PortfolioMetrics {
	return f.metrics
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func buildDeps(opts ...func(*rest.HandlerDependencies)) rest.HandlerDependencies {
	deps := rest.HandlerDependencies{
		CandleRepository:    &fakeCandleRepository{},
		IndicatorRepository: &fakeIndicatorRepository{},
		PositionRepository:  &fakePositionRepository{},
		OrderRepository:     &fakeOrderRepository{},
		DecisionRepository:  &fakeDecisionRepository{},
		Cache:               &fakePriceCache{prices: map[string]decimal.Decimal{}},
		PortfolioManager:    &fakePortfolioManager{},
	}
	for _, opt := range opts {
		opt(&deps)
	}
	return deps
}

func newTestRouter(deps rest.HandlerDependencies) http.Handler {
	router := chi.NewRouter()
	rest.RegisterRoutes(router, deps, "http://localhost:4000")
	return router
}

// ---------------------------------------------------------------------------
// Pagination tests
// ---------------------------------------------------------------------------

func TestParsePagination_Defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	p := rest.ParsePagination(r)

	if p.Limit != 100 {
		t.Errorf("expected default limit 100, got %d", p.Limit)
	}
	if p.Offset != 0 {
		t.Errorf("expected default offset 0, got %d", p.Offset)
	}
}

func TestParsePagination_CapsLimit(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?limit=99999", nil)
	p := rest.ParsePagination(r)

	if p.Limit != 1000 {
		t.Errorf("expected capped limit 1000, got %d", p.Limit)
	}
}

func TestParsePagination_NegativeOffset(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?offset=-5", nil)
	p := rest.ParsePagination(r)

	if p.Offset != 0 {
		t.Errorf("expected offset clamped to 0, got %d", p.Offset)
	}
}

// ---------------------------------------------------------------------------
// Candle handler tests
// ---------------------------------------------------------------------------

func TestCandleHandler_RequiresSymbolAndInterval(t *testing.T) {
	router := newTestRouter(buildDeps())

	cases := []struct {
		name string
		url  string
	}{
		{"missing both", "/api/v1/candles"},
		{"missing interval", "/api/v1/candles?symbol=BTCUSDT"},
		{"missing symbol", "/api/v1/candles?interval=1h"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, tc.url, nil)
			router.ServeHTTP(w, r)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestCandleHandler_ReturnsCandlesWithIndicators(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	candle := domain.Candle{
		ID:         1,
		Symbol:     "BTCUSDT",
		Interval:   "1h",
		OpenTime:   now,
		ClosePrice: decimal.NewFromFloat(65000.50),
	}
	indicator := domain.Indicator{
		ID:        10,
		Symbol:    "BTCUSDT",
		Interval:  "1h",
		Timestamp: now, // matches candle.OpenTime
		Rsi:       decimal.NewFromFloat(55.5),
	}

	deps := buildDeps(func(d *rest.HandlerDependencies) {
		d.CandleRepository = &fakeCandleRepository{candles: []domain.Candle{candle}}
		d.IndicatorRepository = &fakeIndicatorRepository{indicators: []domain.Indicator{indicator}}
	})
	router := newTestRouter(deps)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/candles?symbol=BTCUSDT&interval=1h", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Items []struct {
			ID        int64            `json:"id"`
			Indicator *domain.Indicator `json:"indicator"`
		} `json:"items"`
		Count int `json:"count"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Count != 1 {
		t.Errorf("expected count 1, got %d", resp.Count)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].Indicator == nil {
		t.Error("expected indicator to be attached to candle, got nil")
	}
}

// ---------------------------------------------------------------------------
// Position handler tests
// ---------------------------------------------------------------------------

func TestPositionHandler_OpenPositions_ComputesUnrealizedPnl(t *testing.T) {
	entryPrice := decimal.NewFromFloat(60000)
	quantity := decimal.NewFromFloat(0.5)

	positions := []domain.Position{
		{
			ID:         1,
			Symbol:     "BTCUSDT",
			Side:       "LONG",
			EntryPrice: entryPrice,
			Quantity:   quantity,
			Status:     "open",
		},
		{
			ID:         2,
			Symbol:     "ETHUSDT",
			Side:       "SHORT",
			EntryPrice: decimal.NewFromFloat(3000),
			Quantity:   decimal.NewFromFloat(2),
			Status:     "open",
		},
	}

	deps := buildDeps(func(d *rest.HandlerDependencies) {
		d.PositionRepository = &fakePositionRepository{openPositions: positions}
		d.Cache = &fakePriceCache{prices: map[string]decimal.Decimal{
			"BTCUSDT": decimal.NewFromFloat(65000), // LONG profit: +5000 * 0.5 = +2500
			"ETHUSDT": decimal.NewFromFloat(2800),  // SHORT profit: (3000-2800) * 2 = +400
		}}
	})
	router := newTestRouter(deps)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/positions", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var items []struct {
		ID            int64  `json:"id"`
		Symbol        string `json:"symbol"`
		CurrentPrice  string `json:"currentPrice"`
		UnrealizedPnl string `json:"unrealizedPnl"`
	}
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Check LONG P&L
	btc := items[0]
	pnl, _ := decimal.NewFromString(btc.UnrealizedPnl)
	expected := decimal.NewFromFloat(2500)
	if !pnl.Equal(expected) {
		t.Errorf("LONG unrealizedPnl: expected %s, got %s", expected, pnl)
	}

	// Check SHORT P&L
	eth := items[1]
	ethPnl, _ := decimal.NewFromString(eth.UnrealizedPnl)
	ethExpected := decimal.NewFromFloat(400)
	if !ethPnl.Equal(ethExpected) {
		t.Errorf("SHORT unrealizedPnl: expected %s, got %s", ethExpected, ethPnl)
	}
}

func TestPositionHandler_OpenPositions_ZeroPnlWhenCacheMiss(t *testing.T) {
	positions := []domain.Position{
		{
			ID:         1,
			Symbol:     "BTCUSDT",
			Side:       "LONG",
			EntryPrice: decimal.NewFromFloat(60000),
			Quantity:   decimal.NewFromFloat(1),
			Status:     "open",
		},
	}

	deps := buildDeps(func(d *rest.HandlerDependencies) {
		d.PositionRepository = &fakePositionRepository{openPositions: positions}
		// Cache returns no price for symbol (cache miss)
		d.Cache = &fakePriceCache{prices: map[string]decimal.Decimal{}}
	})
	router := newTestRouter(deps)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/positions", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var items []struct {
		UnrealizedPnl string `json:"unrealizedPnl"`
	}
	if err := json.NewDecoder(w.Body).Decode(&items); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	pnl, _ := decimal.NewFromString(items[0].UnrealizedPnl)
	if !pnl.IsZero() {
		t.Errorf("expected zero P&L on cache miss, got %s", pnl)
	}
}

func TestPositionHandler_ClosedPositions_Paginated(t *testing.T) {
	closedPositions := make([]domain.Position, 5)
	for i := range closedPositions {
		closedPositions[i] = domain.Position{
			ID:     int64(i + 1),
			Symbol: "BTCUSDT",
			Status: "closed",
		}
	}

	deps := buildDeps(func(d *rest.HandlerDependencies) {
		d.PositionRepository = &fakePositionRepository{closedPositions: closedPositions}
	})
	router := newTestRouter(deps)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/positions/history?limit=2&offset=0", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Count   int  `json:"count"`
		Limit   int  `json:"limit"`
		Offset  int  `json:"offset"`
		HasMore bool `json:"hasMore"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Count != 2 {
		t.Errorf("expected count 2, got %d", resp.Count)
	}
	if !resp.HasMore {
		t.Error("expected hasMore=true when full page returned")
	}
}

// ---------------------------------------------------------------------------
// Trade handler tests
// ---------------------------------------------------------------------------

func TestTradeHandler_ListTrades(t *testing.T) {
	orders := []domain.Order{
		{ID: 1, Symbol: "BTCUSDT", Side: domain.OrderSideBuy, Status: domain.OrderStatusFilled},
		{ID: 2, Symbol: "ETHUSDT", Side: domain.OrderSideSell, Status: domain.OrderStatusFilled},
	}

	deps := buildDeps(func(d *rest.HandlerDependencies) {
		d.OrderRepository = &fakeOrderRepository{orders: orders}
	})
	router := newTestRouter(deps)

	t.Run("without symbol filter", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/trades", nil)
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp struct {
			Count int `json:"count"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Count != 2 {
			t.Errorf("expected count 2, got %d", resp.Count)
		}
	})

	t.Run("with symbol filter", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/trades?symbol=BTCUSDT", nil)
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// Metrics handler tests
// ---------------------------------------------------------------------------

func TestMetricsHandler_ReturnsCurrentMetrics(t *testing.T) {
	expectedMetrics := domain.PortfolioMetrics{
		Balance:     decimal.NewFromFloat(10500.00),
		TotalPnl:    decimal.NewFromFloat(500.00),
		WinCount:    7,
		LossCount:   3,
		TotalTrades: 10,
		WinRate:     decimal.NewFromFloat(0.7),
	}

	deps := buildDeps(func(d *rest.HandlerDependencies) {
		d.PortfolioManager = &fakePortfolioManager{metrics: expectedMetrics}
	})
	router := newTestRouter(deps)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	router.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var result struct {
		Balance     string `json:"balance"`
		TotalTrades int    `json:"totalTrades"`
		WinCount    int    `json:"winCount"`
	}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.TotalTrades != 10 {
		t.Errorf("expected totalTrades 10, got %d", result.TotalTrades)
	}
	if result.WinCount != 7 {
		t.Errorf("expected winCount 7, got %d", result.WinCount)
	}
}

// ---------------------------------------------------------------------------
// Decision handler tests
// ---------------------------------------------------------------------------

func TestDecisionHandler_ListDecisions(t *testing.T) {
	decisions := []domain.Decision{
		{ID: 1, Symbol: "BTCUSDT", Action: domain.DecisionActionBuy, Confidence: 80},
		{ID: 2, Symbol: "ETHUSDT", Action: domain.DecisionActionHold, Confidence: 45},
	}

	deps := buildDeps(func(d *rest.HandlerDependencies) {
		d.DecisionRepository = &fakeDecisionRepository{decisions: decisions}
	})
	router := newTestRouter(deps)

	t.Run("all decisions", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/decisions", nil)
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Count int `json:"count"`
		}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Count != 2 {
			t.Errorf("expected count 2, got %d", resp.Count)
		}
	})

	t.Run("filtered by symbol", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/decisions?symbol=BTCUSDT", nil)
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// Decimal serialization test
// ---------------------------------------------------------------------------

// TestDecimalSerialization_QuotedString verifies that shopspring/decimal values
// serialize as JSON strings (e.g., "65000.5"), not bare numbers. This is the
// required behavior for monetary values per FR-8.
func TestDecimalSerialization_QuotedString(t *testing.T) {
	d := decimal.NewFromFloat(65000.50)
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("failed to marshal decimal: %v", err)
	}

	got := string(data)
	// shopspring/decimal marshals as a quoted string
	if len(got) < 2 || got[0] != '"' || got[len(got)-1] != '"' {
		t.Errorf("expected decimal to serialize as quoted string, got: %s", got)
	}

	// Verify the numeric value round-trips correctly.
	var back decimal.Decimal
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("failed to unmarshal decimal: %v", err)
	}
	if !back.Equal(d) {
		t.Errorf("decimal round-trip failed: expected %s, got %s", d, back)
	}
}
