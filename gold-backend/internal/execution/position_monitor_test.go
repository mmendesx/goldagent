package execution

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

// --- Pure-function tests for EvaluatePosition ---

func TestEvaluatePosition_LONG_PriceAboveTP_CloseTakeProfit(t *testing.T) {
	pos := makeLongPosition(
		decimal.NewFromFloat(65000), // entry
		decimal.NewFromFloat(67000), // TP
		decimal.NewFromFloat(63000), // SL
		decimal.Zero,                // no trailing
		decimal.Zero,
	)
	action := EvaluatePosition(pos, decimal.NewFromFloat(67500))
	if action.ActionType != PositionActionCloseTakeProfit {
		t.Errorf("expected CloseTakeProfit, got %q", action.ActionType)
	}
}

func TestEvaluatePosition_LONG_PriceAtTP_CloseTakeProfit(t *testing.T) {
	pos := makeLongPosition(
		decimal.NewFromFloat(65000),
		decimal.NewFromFloat(67000),
		decimal.NewFromFloat(63000),
		decimal.Zero,
		decimal.Zero,
	)
	action := EvaluatePosition(pos, decimal.NewFromFloat(67000))
	if action.ActionType != PositionActionCloseTakeProfit {
		t.Errorf("expected CloseTakeProfit at exact TP, got %q", action.ActionType)
	}
}

func TestEvaluatePosition_LONG_PriceBelowSL_CloseStopLoss(t *testing.T) {
	pos := makeLongPosition(
		decimal.NewFromFloat(65000),
		decimal.NewFromFloat(67000),
		decimal.NewFromFloat(63000),
		decimal.Zero,
		decimal.Zero,
	)
	action := EvaluatePosition(pos, decimal.NewFromFloat(62500))
	if action.ActionType != PositionActionCloseStopLoss {
		t.Errorf("expected CloseStopLoss, got %q", action.ActionType)
	}
}

func TestEvaluatePosition_LONG_PriceAtSL_CloseStopLoss(t *testing.T) {
	pos := makeLongPosition(
		decimal.NewFromFloat(65000),
		decimal.NewFromFloat(67000),
		decimal.NewFromFloat(63000),
		decimal.Zero,
		decimal.Zero,
	)
	action := EvaluatePosition(pos, decimal.NewFromFloat(63000))
	if action.ActionType != PositionActionCloseStopLoss {
		t.Errorf("expected CloseStopLoss at exact SL, got %q", action.ActionType)
	}
}

func TestEvaluatePosition_LONG_TrailingEnabled_PriceBelowTrailing_CloseTrailingStop(t *testing.T) {
	pos := makeLongPosition(
		decimal.NewFromFloat(65000),
		decimal.NewFromFloat(67000),
		decimal.NewFromFloat(63000),
		decimal.NewFromFloat(500),   // trailing distance
		decimal.NewFromFloat(64000), // trailing stop price
	)
	// Price falls below trailing stop
	action := EvaluatePosition(pos, decimal.NewFromFloat(63900))
	if action.ActionType != PositionActionCloseTrailingStop {
		t.Errorf("expected CloseTrailingStop, got %q", action.ActionType)
	}
}

func TestEvaluatePosition_LONG_TrailingEnabled_PriceRises_AdjustTrailingStop(t *testing.T) {
	pos := makeLongPosition(
		decimal.NewFromFloat(65000),
		decimal.NewFromFloat(70000), // TP far away
		decimal.NewFromFloat(63000),
		decimal.NewFromFloat(500),   // trailing distance
		decimal.NewFromFloat(64000), // trailing stop price (current)
	)
	// Price rises to 65500; candidate trailing = 65500 - 500 = 65000 > 64000
	action := EvaluatePosition(pos, decimal.NewFromFloat(65500))
	if action.ActionType != PositionActionAdjustTrailingStop {
		t.Errorf("expected AdjustTrailingStop, got %q", action.ActionType)
	}
	expected := decimal.NewFromFloat(65000)
	if !action.NewTrailingStopPrice.Equal(expected) {
		t.Errorf("new trailing stop: got %s, want %s", action.NewTrailingStopPrice, expected)
	}
}

func TestEvaluatePosition_LONG_PriceBetweenSLAndTP_NoTrailingTrigger_None(t *testing.T) {
	pos := makeLongPosition(
		decimal.NewFromFloat(65000),
		decimal.NewFromFloat(67000),
		decimal.NewFromFloat(63000),
		decimal.NewFromFloat(500),
		decimal.NewFromFloat(64000), // trailing stop
	)
	// Price at 65100: not ≥TP, not ≤SL, not ≤trailing(64000), candidate=64600 not > 64000 by much
	// candidate = 65100 - 500 = 64600 > 64000 → actually triggers AdjustTrailingStop
	// Use a price that does NOT advance the trailing: candidate=64400 ≤ 64000? No.
	// Use trailing stop at 64500, price=64900: candidate=64400 < 64500 → none
	pos.TrailingStopPrice = decimal.NewFromFloat(64500)
	action := EvaluatePosition(pos, decimal.NewFromFloat(64900))
	if action.ActionType != PositionActionNone {
		t.Errorf("expected None, got %q", action.ActionType)
	}
}

func TestEvaluatePosition_SHORT_PriceBelowTP_CloseTakeProfit(t *testing.T) {
	pos := makeShortPosition(
		decimal.NewFromFloat(65000), // entry
		decimal.NewFromFloat(62000), // TP (below entry for short)
		decimal.NewFromFloat(68000), // SL (above entry for short)
	)
	action := EvaluatePosition(pos, decimal.NewFromFloat(61500))
	if action.ActionType != PositionActionCloseTakeProfit {
		t.Errorf("expected CloseTakeProfit for SHORT, got %q", action.ActionType)
	}
}

func TestEvaluatePosition_SHORT_PriceAboveSL_CloseStopLoss(t *testing.T) {
	pos := makeShortPosition(
		decimal.NewFromFloat(65000),
		decimal.NewFromFloat(62000),
		decimal.NewFromFloat(68000),
	)
	action := EvaluatePosition(pos, decimal.NewFromFloat(68500))
	if action.ActionType != PositionActionCloseStopLoss {
		t.Errorf("expected CloseStopLoss for SHORT, got %q", action.ActionType)
	}
}

// TestEvaluatePosition_TPBeforeSL verifies TP takes priority when both could trigger.
func TestEvaluatePosition_TPBeforeSL(t *testing.T) {
	// Pathological: SL above TP (misconfiguration). TP should still win.
	pos := domain.Position{
		Side:                 "LONG",
		EntryPrice:           decimal.NewFromFloat(65000),
		TakeProfitPrice:      decimal.NewFromFloat(64000), // below entry — weird but tests priority
		StopLossPrice:        decimal.NewFromFloat(66000), // above entry — weird
		TrailingStopDistance: decimal.Zero,
		TrailingStopPrice:    decimal.Zero,
	}
	// Price at 64000: satisfies both SL (66000) and TP (64000). TP must win.
	action := EvaluatePosition(pos, decimal.NewFromFloat(64000))
	if action.ActionType != PositionActionCloseTakeProfit {
		t.Errorf("expected CloseTakeProfit to take priority over StopLoss, got %q", action.ActionType)
	}
}

// --- Integration tests with fakes ---

// fakeMonitorPositionRepository is a full in-memory PositionRepository for monitor tests.
type fakeMonitorPositionRepository struct {
	mu sync.Mutex

	openPositions []domain.Position

	trailingStopUpdates []trailingStopUpdate
	closedPositions     []closedPositionRecord

	insertErr           error
	findOpenErr         error
	updateTrailingErr   error
	closeErr            error
}

type trailingStopUpdate struct {
	id    int64
	price decimal.Decimal
}

type closedPositionRecord struct {
	id          int64
	exitOrderID int64
	exitPrice   decimal.Decimal
	realizedPnl decimal.Decimal
	closeReason string
}

func (r *fakeMonitorPositionRepository) InsertPosition(_ context.Context, p domain.Position) (int64, error) {
	return 0, r.insertErr
}

func (r *fakeMonitorPositionRepository) UpdatePositionTrailingStop(_ context.Context, id int64, price decimal.Decimal) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateTrailingErr != nil {
		return r.updateTrailingErr
	}
	r.trailingStopUpdates = append(r.trailingStopUpdates, trailingStopUpdate{id: id, price: price})
	// Update the in-memory open positions list too.
	for i := range r.openPositions {
		if r.openPositions[i].ID == id {
			r.openPositions[i].TrailingStopPrice = price
		}
	}
	return nil
}

func (r *fakeMonitorPositionRepository) ClosePosition(_ context.Context, id int64, exitOrderID int64, exitPrice, realizedPnl decimal.Decimal, closeReason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closeErr != nil {
		return r.closeErr
	}
	r.closedPositions = append(r.closedPositions, closedPositionRecord{
		id:          id,
		exitOrderID: exitOrderID,
		exitPrice:   exitPrice,
		realizedPnl: realizedPnl,
		closeReason: closeReason,
	})
	return nil
}

func (r *fakeMonitorPositionRepository) FindOpenPositions(_ context.Context) ([]domain.Position, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.findOpenErr != nil {
		return nil, r.findOpenErr
	}
	result := make([]domain.Position, len(r.openPositions))
	copy(result, r.openPositions)
	return result, nil
}

func (r *fakeMonitorPositionRepository) CountOpenPositions(_ context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.openPositions), nil
}

func (r *fakeMonitorPositionRepository) FindClosedPositions(_ context.Context, _, _ int) ([]domain.Position, error) {
	return nil, nil
}

func (r *fakeMonitorPositionRepository) FindPositionByID(_ context.Context, id int64) (*domain.Position, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.openPositions {
		if r.openPositions[i].ID == id {
			p := r.openPositions[i]
			return &p, nil
		}
	}
	return nil, nil
}

// fakePriceReader returns a fixed price per symbol.
type fakePriceReader struct {
	mu     sync.Mutex
	prices map[string]decimal.Decimal
	err    error
}

func newFakePriceReader() *fakePriceReader {
	return &fakePriceReader{prices: make(map[string]decimal.Decimal)}
}

func (f *fakePriceReader) setPrice(symbol string, price decimal.Decimal) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prices[symbol] = price
}

func (f *fakePriceReader) GetTickerPrice(_ context.Context, symbol string) (*domain.TickerPrice, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	price, ok := f.prices[symbol]
	if !ok {
		return nil, nil
	}
	return &domain.TickerPrice{Symbol: symbol, Price: price}, nil
}

// fakeMonitorOrderRepository is a full in-memory OrderRepository for monitor tests.
type fakeMonitorOrderRepository struct {
	mu sync.Mutex

	nextID        int64
	insertedOrders []domain.Order
	insertErr     error

	updatedOrders []updatedOrderRecord
	updateErr     error
}

type updatedOrderRecord struct {
	id          int64
	status      domain.OrderStatus
	filledQty   decimal.Decimal
	filledPrice decimal.Decimal
}

func (r *fakeMonitorOrderRepository) InsertOrder(_ context.Context, order domain.Order) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.insertErr != nil {
		return 0, r.insertErr
	}
	r.nextID++
	order.ID = r.nextID
	r.insertedOrders = append(r.insertedOrders, order)
	return r.nextID, nil
}

func (r *fakeMonitorOrderRepository) UpdateOrderStatus(_ context.Context, id int64, status domain.OrderStatus, filledQty, filledPrice, fee decimal.Decimal, _ string, _ []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateErr != nil {
		return r.updateErr
	}
	r.updatedOrders = append(r.updatedOrders, updatedOrderRecord{
		id:          id,
		status:      status,
		filledQty:   filledQty,
		filledPrice: filledPrice,
	})
	return nil
}

func (r *fakeMonitorOrderRepository) FindOrderByID(_ context.Context, _ int64) (*domain.Order, error) {
	return nil, nil
}

func (r *fakeMonitorOrderRepository) FindOrdersBySymbol(_ context.Context, _ string, _, _ int) ([]domain.Order, error) {
	return nil, nil
}

func (r *fakeMonitorOrderRepository) FindRecentOrders(_ context.Context, _, _ int) ([]domain.Order, error) {
	return nil, nil
}

// fakePortfolioRecorder records calls to RecordPositionClose.
type fakePortfolioRecorder struct {
	mu              sync.Mutex
	recordedClosures []domain.Position
	recordErr       error
}

func (f *fakePortfolioRecorder) RecordPositionClose(_ context.Context, pos domain.Position) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recordErr != nil {
		return f.recordErr
	}
	f.recordedClosures = append(f.recordedClosures, pos)
	return nil
}

// --- Test helpers ---

func makeLongPosition(entry, tp, sl, trailingDistance, trailingStopPrice decimal.Decimal) domain.Position {
	return domain.Position{
		ID:                   1,
		Symbol:               "BTCUSDT",
		Side:                 "LONG",
		EntryPrice:           entry,
		Quantity:             decimal.NewFromFloat(0.001),
		TakeProfitPrice:      tp,
		StopLossPrice:        sl,
		TrailingStopDistance: trailingDistance,
		TrailingStopPrice:    trailingStopPrice,
		FeeTotal:             decimal.NewFromFloat(0.0001),
		Status:               "open",
	}
}

func makeShortPosition(entry, tp, sl decimal.Decimal) domain.Position {
	return domain.Position{
		ID:              2,
		Symbol:          "BTCUSDT",
		Side:            "SHORT",
		EntryPrice:      entry,
		Quantity:        decimal.NewFromFloat(0.001),
		TakeProfitPrice: tp,
		StopLossPrice:   sl,
		FeeTotal:        decimal.NewFromFloat(0.0001),
		Status:          "open",
	}
}

func newTestMonitor(
	positionChan <-chan domain.Position,
	priceReader PriceReader,
	orderClient OrderClient,
	orderRepo *fakeMonitorOrderRepository,
	posRepo *fakeMonitorPositionRepository,
	portfolio PortfolioRecorder,
) *PositionMonitor {
	return NewPositionMonitor(PositionMonitorConfig{
		PositionOpenedChannel: positionChan,
		PriceReader:           priceReader,
		OrderClient:           orderClient,
		OrderRepository:       orderRepo,
		PositionRepository:    posRepo,
		PortfolioManager:      portfolio,
		EvaluationInterval:    10 * time.Millisecond, // fast for tests
		Logger:                testLogger(),
	})
}

// --- Integration tests ---

// TestPositionMonitor_TPExitFlow verifies that when a position is added and the price exceeds TP,
// an exit order is placed, the position is closed with reason TAKE_PROFIT, and the portfolio
// manager is notified.
func TestPositionMonitor_TPExitFlow(t *testing.T) {
	positionChan := make(chan domain.Position, 1)
	priceReader := newFakePriceReader()
	orderClient := &fakeOrderClient{
		result: BinanceOrderResult{
			OrderID:          5001,
			Symbol:           "BTCUSDT",
			Status:           "FILLED",
			ExecutedQuantity: decimal.NewFromFloat(0.001),
			AverageFillPrice: decimal.NewFromFloat(67100),
			TotalCommission:  decimal.NewFromFloat(0.0001),
			CommissionAsset:  "BTC",
			RawResponse:      []byte(`{}`),
		},
	}
	orderRepo := &fakeMonitorOrderRepository{}
	posRepo := &fakeMonitorPositionRepository{}
	portfolio := &fakePortfolioRecorder{}

	monitor := newTestMonitor(positionChan, priceReader, orderClient, orderRepo, posRepo, portfolio)

	// Add position via channel
	pos := makeLongPosition(
		decimal.NewFromFloat(65000),
		decimal.NewFromFloat(67000), // TP
		decimal.NewFromFloat(63000),
		decimal.Zero,
		decimal.Zero,
	)
	positionChan <- pos

	// Set price above TP
	priceReader.setPrice("BTCUSDT", decimal.NewFromFloat(67100))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- monitor.Run(ctx)
	}()

	// Wait for position to be closed
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		posRepo.mu.Lock()
		closed := len(posRepo.closedPositions) > 0
		posRepo.mu.Unlock()
		if closed {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	<-done

	posRepo.mu.Lock()
	defer posRepo.mu.Unlock()

	if len(posRepo.closedPositions) == 0 {
		t.Fatal("expected position to be closed, got none")
	}
	closed := posRepo.closedPositions[0]
	if closed.closeReason != "TAKE_PROFIT" {
		t.Errorf("close reason: got %q, want TAKE_PROFIT", closed.closeReason)
	}
	if !closed.exitPrice.Equal(decimal.NewFromFloat(67100)) {
		t.Errorf("exit price: got %s, want 67100", closed.exitPrice)
	}

	orderRepo.mu.Lock()
	defer orderRepo.mu.Unlock()
	if len(orderRepo.insertedOrders) == 0 {
		t.Error("expected exit order to be inserted")
	}
	if orderRepo.insertedOrders[0].Side != domain.OrderSideSell {
		t.Errorf("exit order side: got %q, want SELL", orderRepo.insertedOrders[0].Side)
	}

	portfolio.mu.Lock()
	defer portfolio.mu.Unlock()
	if len(portfolio.recordedClosures) == 0 {
		t.Error("expected portfolio manager to be notified of position close")
	}
	if portfolio.recordedClosures[0].CloseReason != "TAKE_PROFIT" {
		t.Errorf("portfolio close reason: got %q, want TAKE_PROFIT", portfolio.recordedClosures[0].CloseReason)
	}
}

// TestPositionMonitor_SLExitFlow verifies stop-loss triggers an exit with STOP_LOSS reason.
func TestPositionMonitor_SLExitFlow(t *testing.T) {
	positionChan := make(chan domain.Position, 1)
	priceReader := newFakePriceReader()
	orderClient := &fakeOrderClient{
		result: BinanceOrderResult{
			OrderID:          5002,
			Symbol:           "BTCUSDT",
			Status:           "FILLED",
			ExecutedQuantity: decimal.NewFromFloat(0.001),
			AverageFillPrice: decimal.NewFromFloat(62800),
			TotalCommission:  decimal.NewFromFloat(0.0001),
			CommissionAsset:  "BTC",
			RawResponse:      []byte(`{}`),
		},
	}
	orderRepo := &fakeMonitorOrderRepository{}
	posRepo := &fakeMonitorPositionRepository{}

	monitor := newTestMonitor(positionChan, priceReader, orderClient, orderRepo, posRepo, nil)

	pos := makeLongPosition(
		decimal.NewFromFloat(65000),
		decimal.NewFromFloat(67000),
		decimal.NewFromFloat(63000), // SL
		decimal.Zero,
		decimal.Zero,
	)
	positionChan <- pos

	// Price below SL
	priceReader.setPrice("BTCUSDT", decimal.NewFromFloat(62800))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- monitor.Run(ctx)
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		posRepo.mu.Lock()
		closed := len(posRepo.closedPositions) > 0
		posRepo.mu.Unlock()
		if closed {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	<-done

	posRepo.mu.Lock()
	defer posRepo.mu.Unlock()

	if len(posRepo.closedPositions) == 0 {
		t.Fatal("expected position to be closed via stop-loss, got none")
	}
	if posRepo.closedPositions[0].closeReason != "STOP_LOSS" {
		t.Errorf("close reason: got %q, want STOP_LOSS", posRepo.closedPositions[0].closeReason)
	}
}

// TestPositionMonitor_TrailingStopAdjustmentAndTrigger verifies:
//  1. Rising price advances the trailing stop.
//  2. Price reversal below the new trailing stop triggers a TRAILING_STOP exit.
func TestPositionMonitor_TrailingStopAdjustmentAndTrigger(t *testing.T) {
	positionChan := make(chan domain.Position, 1)
	priceReader := newFakePriceReader()
	orderClient := &fakeOrderClient{
		result: BinanceOrderResult{
			OrderID:          5003,
			Symbol:           "BTCUSDT",
			Status:           "FILLED",
			ExecutedQuantity: decimal.NewFromFloat(0.001),
			AverageFillPrice: decimal.NewFromFloat(65400),
			TotalCommission:  decimal.NewFromFloat(0.0001),
			CommissionAsset:  "BTC",
			RawResponse:      []byte(`{}`),
		},
	}
	orderRepo := &fakeMonitorOrderRepository{}
	posRepo := &fakeMonitorPositionRepository{}

	monitor := newTestMonitor(positionChan, priceReader, orderClient, orderRepo, posRepo, nil)

	pos := makeLongPosition(
		decimal.NewFromFloat(65000),
		decimal.NewFromFloat(70000), // TP far away
		decimal.NewFromFloat(63000),
		decimal.NewFromFloat(500),   // trailing distance
		decimal.NewFromFloat(64000), // initial trailing stop
	)
	positionChan <- pos

	// Phase 1: price rises to 65500 → candidate trailing = 65000 > 64000 → adjust
	priceReader.setPrice("BTCUSDT", decimal.NewFromFloat(65500))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- monitor.Run(ctx)
	}()

	// Wait for trailing stop to be updated
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		posRepo.mu.Lock()
		updated := len(posRepo.trailingStopUpdates) > 0
		posRepo.mu.Unlock()
		if updated {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	posRepo.mu.Lock()
	if len(posRepo.trailingStopUpdates) == 0 {
		posRepo.mu.Unlock()
		t.Fatal("expected trailing stop to be updated after price rise, got none")
	}
	expectedNewTrailing := decimal.NewFromFloat(65000)
	if !posRepo.trailingStopUpdates[0].price.Equal(expectedNewTrailing) {
		t.Errorf("new trailing stop: got %s, want %s", posRepo.trailingStopUpdates[0].price, expectedNewTrailing)
	}
	posRepo.mu.Unlock()

	// Phase 2: price falls to 64900 — below new trailing stop of 65000 → exit
	priceReader.setPrice("BTCUSDT", decimal.NewFromFloat(64900))

	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		posRepo.mu.Lock()
		closed := len(posRepo.closedPositions) > 0
		posRepo.mu.Unlock()
		if closed {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	<-done

	posRepo.mu.Lock()
	defer posRepo.mu.Unlock()

	if len(posRepo.closedPositions) == 0 {
		t.Fatal("expected position to be closed via trailing stop, got none")
	}
	if posRepo.closedPositions[0].closeReason != "TRAILING_STOP" {
		t.Errorf("close reason: got %q, want TRAILING_STOP", posRepo.closedPositions[0].closeReason)
	}
}

// TestPositionMonitor_FailedExitOrderLeavesPositionActive verifies that when the exit order
// placement fails, the position remains in the active set for retry on the next tick.
func TestPositionMonitor_FailedExitOrderLeavesPositionActive(t *testing.T) {
	positionChan := make(chan domain.Position, 1)
	priceReader := newFakePriceReader()
	orderClient := &fakeOrderClient{
		err: errors.New("binance: connection timeout"),
	}
	orderRepo := &fakeMonitorOrderRepository{}
	posRepo := &fakeMonitorPositionRepository{}

	monitor := newTestMonitor(positionChan, priceReader, orderClient, orderRepo, posRepo, nil)

	pos := makeLongPosition(
		decimal.NewFromFloat(65000),
		decimal.NewFromFloat(67000), // TP
		decimal.NewFromFloat(63000),
		decimal.Zero,
		decimal.Zero,
	)
	positionChan <- pos

	// Set price above TP to trigger exit
	priceReader.setPrice("BTCUSDT", decimal.NewFromFloat(67500))

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- monitor.Run(ctx)
	}()

	<-done

	// Position must NOT be in closedPositions
	posRepo.mu.Lock()
	defer posRepo.mu.Unlock()
	if len(posRepo.closedPositions) > 0 {
		t.Error("expected position to remain active after failed exit order, but it was closed")
	}

	// Position must remain in active set
	monitor.mu.RLock()
	_, stillActive := monitor.activePositions[pos.ID]
	monitor.mu.RUnlock()
	if !stillActive {
		t.Error("expected position to remain in active set after failed exit order, but it was removed")
	}
}

// TestPositionMonitor_LoadOpenPositionsFromDatabase verifies that existing open positions
// are loaded from the database on startup and are tracked for evaluation.
func TestPositionMonitor_LoadOpenPositionsFromDatabase(t *testing.T) {
	priceReader := newFakePriceReader()
	orderRepo := &fakeMonitorOrderRepository{}
	posRepo := &fakeMonitorPositionRepository{
		openPositions: []domain.Position{
			makeLongPosition(
				decimal.NewFromFloat(65000),
				decimal.NewFromFloat(67000),
				decimal.NewFromFloat(63000),
				decimal.Zero,
				decimal.Zero,
			),
		},
	}

	monitor := newTestMonitor(make(chan domain.Position), priceReader, &fakeOrderClient{}, orderRepo, posRepo, nil)

	ctx := context.Background()
	if err := monitor.LoadOpenPositionsFromDatabase(ctx); err != nil {
		t.Fatalf("LoadOpenPositionsFromDatabase returned unexpected error: %v", err)
	}

	monitor.mu.RLock()
	count := len(monitor.activePositions)
	monitor.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 active position after load, got %d", count)
	}
}

// TestPositionMonitor_RealizedPnL verifies correct P&L calculation net of fees.
func TestPositionMonitor_RealizedPnL(t *testing.T) {
	// LONG: entry=65000, exit=67000, qty=0.001
	// raw PnL = (67000-65000)*0.001 = 2
	// entryFee = 0.5, exitFee = 0.3 → net PnL = 2 - 0.5 - 0.3 = 1.2
	entry := decimal.NewFromFloat(65000)
	exit := decimal.NewFromFloat(67000)
	qty := decimal.NewFromFloat(0.001)
	entryFee := decimal.NewFromFloat(0.5)
	exitFee := decimal.NewFromFloat(0.3)

	pnl := computeRealizedPnl("LONG", entry, exit, qty, entryFee, exitFee)
	expected := decimal.NewFromFloat(1.2)
	if !pnl.Equal(expected) {
		t.Errorf("LONG realized PnL: got %s, want %s", pnl, expected)
	}

	// SHORT: entry=65000, exit=63000, qty=0.001
	// raw PnL = (65000-63000)*0.001 = 2
	// net PnL = 2 - 0.5 - 0.3 = 1.2
	pnlShort := computeRealizedPnl("SHORT", entry, decimal.NewFromFloat(63000), qty, entryFee, exitFee)
	if !pnlShort.Equal(expected) {
		t.Errorf("SHORT realized PnL: got %s, want %s", pnlShort, expected)
	}
}
