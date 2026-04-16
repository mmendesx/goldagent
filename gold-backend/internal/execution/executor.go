package execution

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
)

// positionOpenedChannelBuffer is the number of positions that can be buffered
// on the positionOpenedChannel before the send blocks. Sized to absorb bursts
// from rapid intent processing without blocking the executor.
const positionOpenedChannelBuffer = 16

// OrderClient is the interface the executor uses to place orders.
// In production, *BinanceOrderClient satisfies this interface.
// In tests, a fake can be substituted.
type OrderClient interface {
	PlaceMarketOrder(ctx context.Context, symbol string, side domain.OrderSide, quantity decimal.Decimal) (BinanceOrderResult, error)
}

// ExecutorConfig wires the executor to upstream intents and downstream repositories.
type ExecutorConfig struct {
	TradeIntentChannel <-chan domain.TradeIntent
	OrderClient        OrderClient
	OrderRepository    postgres.OrderRepository
	PositionRepository postgres.PositionRepository
	DecisionRepository postgres.DecisionRepository
	Logger             *slog.Logger
}

// Executor consumes TradeIntents and places real orders on Binance.
// It never checks a dry-run flag — gating is the decision engine's responsibility.
type Executor struct {
	config           ExecutorConfig
	positionOpened   chan domain.Position
}

// NewExecutor constructs an Executor with a buffered position-opened channel.
func NewExecutor(config ExecutorConfig) *Executor {
	return &Executor{
		config:         config,
		positionOpened: make(chan domain.Position, positionOpenedChannelBuffer),
	}
}

// PositionOpenedChannel returns a read-only channel that emits every newly-opened Position.
// The position monitor (ICT-15) should consume from this channel.
func (executor *Executor) PositionOpenedChannel() <-chan domain.Position {
	return executor.positionOpened
}

// Run drains the TradeIntent channel until it closes or ctx is cancelled.
//
// For each intent:
//  1. Insert a pending Order into the database.
//  2. Call OrderClient.PlaceMarketOrder.
//  3. On success: update the order to "filled", create a Position, emit on positionOpenedChannel,
//     and update the decision execution_status to "executed".
//  4. On failure: update the order to "rejected", update the decision to "rejected".
//     No position is created.
//
// Processing errors per intent are logged and do not crash the loop.
// Returns ctx.Err() on context cancellation, nil when the input channel closes.
func (executor *Executor) Run(ctx context.Context) error {
	defer close(executor.positionOpened)

	for {
		select {
		case <-ctx.Done():
			executor.config.Logger.Info("executor: context cancelled, stopping",
				"reason", ctx.Err().Error(),
			)
			return ctx.Err()

		case intent, open := <-executor.config.TradeIntentChannel:
			if !open {
				executor.config.Logger.Info("executor: trade intent channel closed, stopping")
				return nil
			}
			executor.processTradeIntent(ctx, intent)
		}
	}
}

// processTradeIntent handles a single TradeIntent end-to-end.
// All errors are logged; the loop continues regardless.
func (executor *Executor) processTradeIntent(ctx context.Context, intent domain.TradeIntent) {
	logger := executor.config.Logger.With(
		"decisionId", intent.DecisionID,
		"symbol", intent.Symbol,
		"side", intent.Side,
		"quantity", intent.PositionSizeQuantity.String(),
	)

	logger.Info("executor: processing trade intent")

	// Step 1: Insert a pending order record.
	decisionID := intent.DecisionID
	pendingOrder := domain.Order{
		Exchange:   domain.OrderExchangeBinance,
		DecisionID: &decisionID,
		Symbol:     intent.Symbol,
		Side:       intent.Side,
		Quantity:   intent.PositionSizeQuantity,
		Status:     domain.OrderStatusPending,
	}

	orderID, err := executor.config.OrderRepository.InsertOrder(ctx, pendingOrder)
	if err != nil {
		logger.Error("executor: failed to insert pending order", "error", err)
		// Cannot update order status since we have no ID; update decision as rejected.
		executor.rejectDecision(ctx, intent.DecisionID, fmt.Sprintf("failed to insert pending order: %s", err.Error()), logger)
		return
	}

	logger.Info("executor: pending order inserted", "orderId", orderID)

	// Step 2: Place the market order on Binance.
	result, err := executor.config.OrderClient.PlaceMarketOrder(ctx, intent.Symbol, intent.Side, intent.PositionSizeQuantity)
	if err != nil {
		rejectionReason := err.Error()
		logger.Error("executor: order placement failed",
			"orderId", orderID,
			"error", rejectionReason,
		)

		updateErr := executor.config.OrderRepository.UpdateOrderStatus(
			ctx,
			orderID,
			domain.OrderStatusRejected,
			decimal.Zero,
			decimal.Zero,
			decimal.Zero,
			"",
			result.RawResponse,
		)
		if updateErr != nil {
			logger.Error("executor: failed to update order to rejected",
				"orderId", orderID,
				"error", updateErr,
			)
		}

		executor.rejectDecision(ctx, intent.DecisionID, rejectionReason, logger)
		return
	}

	logger.Info("executor: order filled",
		"orderId", orderID,
		"binanceOrderId", result.OrderID,
		"executedQty", result.ExecutedQuantity.String(),
		"avgFillPrice", result.AverageFillPrice.String(),
		"commission", result.TotalCommission.String(),
		"commissionAsset", result.CommissionAsset,
	)

	// Step 3a: Update the order record with fill details.
	updateErr := executor.config.OrderRepository.UpdateOrderStatus(
		ctx,
		orderID,
		domain.OrderStatusFilled,
		result.ExecutedQuantity,
		result.AverageFillPrice,
		result.TotalCommission,
		result.CommissionAsset,
		result.RawResponse,
	)
	if updateErr != nil {
		logger.Error("executor: failed to update order to filled",
			"orderId", orderID,
			"error", updateErr,
		)
		// Order is actually filled on Binance but we failed to persist the status.
		// Log the discrepancy; do not abandon position creation.
	}

	// Step 3b: Create a position from the fill details.
	positionSide := "LONG" // v1 only supports BUY/LONG
	if intent.Side == domain.OrderSideSell {
		positionSide = "SHORT"
	}

	now := time.Now()
	position := domain.Position{
		Symbol:               intent.Symbol,
		Side:                 positionSide,
		EntryOrderID:         &orderID,
		EntryPrice:           result.AverageFillPrice,
		Quantity:             result.ExecutedQuantity,
		TakeProfitPrice:      intent.SuggestedTakeProfit,
		StopLossPrice:        intent.SuggestedStopLoss,
		TrailingStopDistance: intent.SuggestedTrailingStopDistance,
		TrailingStopPrice:    intent.SuggestedStopLoss, // initial trailing stop equals SL
		FeeTotal:             result.TotalCommission,
		Status:               "open",
		OpenedAt:             now,
	}

	positionID, insertErr := executor.config.PositionRepository.InsertPosition(ctx, position)
	if insertErr != nil {
		logger.Error("executor: failed to insert position",
			"orderId", orderID,
			"error", insertErr,
		)
		// The order is filled on Binance. Record the decision as executed even though
		// position creation failed — the operator must reconcile manually.
		executor.markDecisionExecuted(ctx, intent.DecisionID, logger)
		return
	}

	position.ID = positionID
	logger.Info("executor: position created",
		"positionId", positionID,
		"entryPrice", position.EntryPrice.String(),
		"takeProfit", position.TakeProfitPrice.String(),
		"stopLoss", position.StopLossPrice.String(),
	)

	// Step 3c: Emit the new position for the position monitor.
	select {
	case executor.positionOpened <- position:
	default:
		logger.Warn("executor: position opened channel full, dropping position notification",
			"positionId", positionID,
		)
	}

	// Step 3d: Update the decision execution status to "executed".
	executor.markDecisionExecuted(ctx, intent.DecisionID, logger)
}

// rejectDecision updates the decision execution status to "rejected" with the given reason.
func (executor *Executor) rejectDecision(ctx context.Context, decisionID int64, reason string, logger *slog.Logger) {
	if err := executor.config.DecisionRepository.UpdateDecisionExecutionStatus(
		ctx,
		decisionID,
		domain.DecisionExecutionStatusRejected,
		reason,
	); err != nil {
		logger.Error("executor: failed to update decision to rejected",
			"decisionId", decisionID,
			"error", err,
		)
	}
}

// markDecisionExecuted updates the decision execution status to "executed".
func (executor *Executor) markDecisionExecuted(ctx context.Context, decisionID int64, logger *slog.Logger) {
	if err := executor.config.DecisionRepository.UpdateDecisionExecutionStatus(
		ctx,
		decisionID,
		domain.DecisionExecutionStatusExecuted,
		"",
	); err != nil {
		logger.Error("executor: failed to update decision to executed",
			"decisionId", decisionID,
			"error", err,
		)
	}
}
