package execution

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
)

// --- Fakes ---

// fakeOrderClient is a test double for OrderClient that returns a pre-configured result.
type fakeOrderClient struct {
	result BinanceOrderResult
	err    error
}

func (f *fakeOrderClient) PlaceMarketOrder(
	_ context.Context,
	_ string,
	_ domain.OrderSide,
	_ decimal.Decimal,
) (BinanceOrderResult, error) {
	return f.result, f.err
}

// fakeOrderRepository records calls and returns pre-configured responses.
type fakeOrderRepository struct {
	postgres.OrderRepository // embed to satisfy unused methods

	insertedOrder   domain.Order
	insertedOrderID int64
	insertErr       error

	updatedOrderID     int64
	updatedStatus      domain.OrderStatus
	updatedFilledQty   decimal.Decimal
	updatedFilledPrice decimal.Decimal
	updatedFee         decimal.Decimal
	updatedFeeAsset    string
	updatedRawResponse []byte
	updateErr          error

	mu sync.Mutex
}

func (f *fakeOrderRepository) InsertOrder(_ context.Context, order domain.Order) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.insertedOrder = order
	return f.insertedOrderID, f.insertErr
}

func (f *fakeOrderRepository) UpdateOrderStatus(
	_ context.Context,
	id int64,
	status domain.OrderStatus,
	filledQty, filledPrice, fee decimal.Decimal,
	feeAsset string,
	rawResponse []byte,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updatedOrderID = id
	f.updatedStatus = status
	f.updatedFilledQty = filledQty
	f.updatedFilledPrice = filledPrice
	f.updatedFee = fee
	f.updatedFeeAsset = feeAsset
	f.updatedRawResponse = rawResponse
	return f.updateErr
}

// fakePositionRepository records the inserted position.
type fakePositionRepository struct {
	postgres.PositionRepository // embed to satisfy unused methods

	insertedPosition domain.Position
	insertedID       int64
	insertErr        error

	mu sync.Mutex
}

func (f *fakePositionRepository) InsertPosition(_ context.Context, position domain.Position) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.insertedPosition = position
	return f.insertedID, f.insertErr
}

// fakeDecisionRepository records calls to UpdateDecisionExecutionStatus.
type fakeDecisionRepository struct {
	postgres.DecisionRepository // embed to satisfy unused methods

	updatedID     int64
	updatedStatus domain.DecisionExecutionStatus
	updatedReason string
	updateErr     error

	mu sync.Mutex
}

func (f *fakeDecisionRepository) UpdateDecisionExecutionStatus(
	_ context.Context,
	id int64,
	status domain.DecisionExecutionStatus,
	rejectionReason string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updatedID = id
	f.updatedStatus = status
	f.updatedReason = rejectionReason
	return f.updateErr
}

// --- Helpers ---

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func newTestIntent() domain.TradeIntent {
	return domain.TradeIntent{
		DecisionID:                    42,
		Symbol:                        "BTCUSDT",
		Side:                          domain.OrderSideBuy,
		EstimatedEntryPrice:           decimal.NewFromFloat(65000.0),
		PositionSizeQuantity:          decimal.NewFromFloat(0.001),
		SuggestedTakeProfit:           decimal.NewFromFloat(67000.0),
		SuggestedStopLoss:             decimal.NewFromFloat(63000.0),
		SuggestedTrailingStopDistance: decimal.NewFromFloat(500.0),
		AtrValue:                      decimal.NewFromFloat(800.0),
		CreatedAt:                     time.Now(),
	}
}

func newFilledOrderResult() BinanceOrderResult {
	return BinanceOrderResult{
		OrderID:          99991,
		Symbol:           "BTCUSDT",
		Status:           "FILLED",
		OriginalQuantity: decimal.NewFromFloat(0.001),
		ExecutedQuantity: decimal.NewFromFloat(0.001),
		AverageFillPrice: decimal.NewFromFloat(65100.0),
		TotalCommission:  decimal.NewFromFloat(0.000001),
		CommissionAsset:  "BTC",
		RawResponse:      []byte(`{"id":"abc","status":200,"result":{"orderId":99991}}`),
	}
}

// --- Tests ---

// TestExecutor_SuccessfulExecution verifies the full happy path:
// intent received → pending order inserted → order filled → position created →
// position emitted on channel → decision updated to "executed".
func TestExecutor_SuccessfulExecution(t *testing.T) {
	intentChan := make(chan domain.TradeIntent, 1)
	orderRepo := &fakeOrderRepository{insertedOrderID: 1001}
	positionRepo := &fakePositionRepository{insertedID: 2001}
	decisionRepo := &fakeDecisionRepository{}
	orderClient := &fakeOrderClient{result: newFilledOrderResult()}

	executor := NewExecutor(ExecutorConfig{
		TradeIntentChannel: intentChan,
		OrderClient:        orderClient,
		OrderRepository:    orderRepo,
		PositionRepository: positionRepo,
		DecisionRepository: decisionRepo,
		Logger:             testLogger(),
	})

	intent := newTestIntent()
	intentChan <- intent
	close(intentChan)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := executor.Run(ctx)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// Order was inserted as pending.
	orderRepo.mu.Lock()
	if orderRepo.insertedOrder.Status != domain.OrderStatusPending {
		t.Errorf("inserted order status: got %q, want %q", orderRepo.insertedOrder.Status, domain.OrderStatusPending)
	}
	if orderRepo.insertedOrder.Symbol != intent.Symbol {
		t.Errorf("inserted order symbol: got %q, want %q", orderRepo.insertedOrder.Symbol, intent.Symbol)
	}
	if orderRepo.insertedOrder.Exchange != domain.OrderExchangeBinance {
		t.Errorf("inserted order exchange: got %q, want %q", orderRepo.insertedOrder.Exchange, domain.OrderExchangeBinance)
	}
	orderRepo.mu.Unlock()

	// Order was updated to filled.
	orderRepo.mu.Lock()
	if orderRepo.updatedStatus != domain.OrderStatusFilled {
		t.Errorf("updated order status: got %q, want %q", orderRepo.updatedStatus, domain.OrderStatusFilled)
	}
	if orderRepo.updatedOrderID != 1001 {
		t.Errorf("updated order ID: got %d, want 1001", orderRepo.updatedOrderID)
	}
	orderRepo.mu.Unlock()

	// Position was created.
	positionRepo.mu.Lock()
	pos := positionRepo.insertedPosition
	if pos.Symbol != intent.Symbol {
		t.Errorf("position symbol: got %q, want %q", pos.Symbol, intent.Symbol)
	}
	if pos.Side != "LONG" {
		t.Errorf("position side: got %q, want LONG", pos.Side)
	}
	if pos.Status != "open" {
		t.Errorf("position status: got %q, want open", pos.Status)
	}
	result := newFilledOrderResult()
	if !pos.EntryPrice.Equal(result.AverageFillPrice) {
		t.Errorf("position entry price: got %s, want %s", pos.EntryPrice, result.AverageFillPrice)
	}
	if !pos.TakeProfitPrice.Equal(intent.SuggestedTakeProfit) {
		t.Errorf("position take profit: got %s, want %s", pos.TakeProfitPrice, intent.SuggestedTakeProfit)
	}
	if !pos.StopLossPrice.Equal(intent.SuggestedStopLoss) {
		t.Errorf("position stop loss: got %s, want %s", pos.StopLossPrice, intent.SuggestedStopLoss)
	}
	if !pos.TrailingStopPrice.Equal(intent.SuggestedStopLoss) {
		t.Errorf("position trailing stop price: got %s, want %s", pos.TrailingStopPrice, intent.SuggestedStopLoss)
	}
	positionRepo.mu.Unlock()

	// Position was emitted on the channel.
	positionsChan := executor.PositionOpenedChannel()
	select {
	case emitted, ok := <-positionsChan:
		if !ok {
			t.Error("positionOpenedChannel was closed before emitting position")
			return
		}
		if emitted.Symbol != intent.Symbol {
			t.Errorf("emitted position symbol: got %q, want %q", emitted.Symbol, intent.Symbol)
		}
	case <-time.After(time.Second):
		t.Error("timed out waiting for position on positionOpenedChannel")
	}

	// Decision was updated to "executed".
	decisionRepo.mu.Lock()
	if decisionRepo.updatedStatus != domain.DecisionExecutionStatusExecuted {
		t.Errorf("decision status: got %q, want %q", decisionRepo.updatedStatus, domain.DecisionExecutionStatusExecuted)
	}
	if decisionRepo.updatedID != intent.DecisionID {
		t.Errorf("decision ID: got %d, want %d", decisionRepo.updatedID, intent.DecisionID)
	}
	decisionRepo.mu.Unlock()
}

// TestExecutor_RejectedOrder verifies that when Binance rejects an order:
// - Order is updated to "rejected".
// - No position is created.
// - Decision is updated to "rejected" with the error message.
func TestExecutor_RejectedOrder(t *testing.T) {
	intentChan := make(chan domain.TradeIntent, 1)
	orderRepo := &fakeOrderRepository{insertedOrderID: 1002}
	positionRepo := &fakePositionRepository{}
	decisionRepo := &fakeDecisionRepository{}
	orderClient := &fakeOrderClient{
		err: errors.New("binance order rejected: code -2010 — Account has insufficient balance"),
	}

	executor := NewExecutor(ExecutorConfig{
		TradeIntentChannel: intentChan,
		OrderClient:        orderClient,
		OrderRepository:    orderRepo,
		PositionRepository: positionRepo,
		DecisionRepository: decisionRepo,
		Logger:             testLogger(),
	})

	intent := newTestIntent()
	intent.DecisionID = 77
	intentChan <- intent
	close(intentChan)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := executor.Run(ctx); err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}

	// Order must be updated to rejected.
	orderRepo.mu.Lock()
	if orderRepo.updatedStatus != domain.OrderStatusRejected {
		t.Errorf("order status after rejection: got %q, want %q", orderRepo.updatedStatus, domain.OrderStatusRejected)
	}
	orderRepo.mu.Unlock()

	// No position must be inserted.
	positionRepo.mu.Lock()
	if positionRepo.insertedPosition.Symbol != "" {
		t.Errorf("expected no position to be created, but got position with symbol %q", positionRepo.insertedPosition.Symbol)
	}
	positionRepo.mu.Unlock()

	// Decision must be updated to rejected.
	decisionRepo.mu.Lock()
	if decisionRepo.updatedStatus != domain.DecisionExecutionStatusRejected {
		t.Errorf("decision status: got %q, want %q", decisionRepo.updatedStatus, domain.DecisionExecutionStatusRejected)
	}
	if decisionRepo.updatedReason == "" {
		t.Error("expected non-empty rejection reason on decision, got empty string")
	}
	if decisionRepo.updatedID != intent.DecisionID {
		t.Errorf("decision ID: got %d, want %d", decisionRepo.updatedID, intent.DecisionID)
	}
	decisionRepo.mu.Unlock()
}

// TestExecutor_ChannelClose verifies that closing the trade intent channel causes Run to return nil.
func TestExecutor_ChannelClose(t *testing.T) {
	intentChan := make(chan domain.TradeIntent)

	executor := NewExecutor(ExecutorConfig{
		TradeIntentChannel: intentChan,
		OrderClient:        &fakeOrderClient{},
		OrderRepository:    &fakeOrderRepository{},
		PositionRepository: &fakePositionRepository{},
		DecisionRepository: &fakeDecisionRepository{},
		Logger:             testLogger(),
	})

	done := make(chan error, 1)
	go func() {
		done <- executor.Run(context.Background())
	}()

	close(intentChan)

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned unexpected error after channel close: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Run did not return after trade intent channel was closed")
	}
}

// TestExecutor_ContextCancellation verifies that cancelling the context causes Run to return ctx.Err().
func TestExecutor_ContextCancellation(t *testing.T) {
	intentChan := make(chan domain.TradeIntent) // never sends

	executor := NewExecutor(ExecutorConfig{
		TradeIntentChannel: intentChan,
		OrderClient:        &fakeOrderClient{},
		OrderRepository:    &fakeOrderRepository{},
		PositionRepository: &fakePositionRepository{},
		DecisionRepository: &fakeDecisionRepository{},
		Logger:             testLogger(),
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- executor.Run(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Run returned %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Run did not return after context cancellation")
	}
}

// TestExecutor_PositionSideIsLongForBuy verifies that BUY side produces a LONG position.
func TestExecutor_PositionSideIsLongForBuy(t *testing.T) {
	intentChan := make(chan domain.TradeIntent, 1)
	orderRepo := &fakeOrderRepository{insertedOrderID: 1003}
	positionRepo := &fakePositionRepository{insertedID: 2003}
	decisionRepo := &fakeDecisionRepository{}
	orderClient := &fakeOrderClient{result: newFilledOrderResult()}

	executor := NewExecutor(ExecutorConfig{
		TradeIntentChannel: intentChan,
		OrderClient:        orderClient,
		OrderRepository:    orderRepo,
		PositionRepository: positionRepo,
		DecisionRepository: decisionRepo,
		Logger:             testLogger(),
	})

	intent := newTestIntent()
	intent.Side = domain.OrderSideBuy
	intentChan <- intent
	close(intentChan)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	executor.Run(ctx)

	positionRepo.mu.Lock()
	defer positionRepo.mu.Unlock()
	if positionRepo.insertedPosition.Side != "LONG" {
		t.Errorf("expected position side LONG for BUY order, got %q", positionRepo.insertedPosition.Side)
	}
}
