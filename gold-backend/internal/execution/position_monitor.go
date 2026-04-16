package execution

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
)

const defaultEvaluationInterval = time.Second

// PriceReader abstracts the price source. *redisstore.CacheClient implements it.
type PriceReader interface {
	GetTickerPrice(ctx context.Context, symbol string) (*domain.TickerPrice, error)
}

// PortfolioRecorder is an optional sink notified when a position closes.
// The portfolio manager (ICT-16) implements this. Can be nil.
type PortfolioRecorder interface {
	RecordPositionClose(ctx context.Context, position domain.Position) error
}

// PositionActionType identifies the action EvaluatePosition decided on.
type PositionActionType string

const (
	PositionActionNone               PositionActionType = "none"
	PositionActionCloseTakeProfit    PositionActionType = "close_take_profit"
	PositionActionCloseStopLoss      PositionActionType = "close_stop_loss"
	PositionActionCloseTrailingStop  PositionActionType = "close_trailing_stop"
	PositionActionAdjustTrailingStop PositionActionType = "adjust_trailing_stop"
)

// PositionAction is the outcome of EvaluatePosition.
type PositionAction struct {
	ActionType           PositionActionType
	NewTrailingStopPrice decimal.Decimal // set for AdjustTrailingStop
}

// PositionMonitorConfig configures the monitor.
type PositionMonitorConfig struct {
	PositionOpenedChannel <-chan domain.Position
	PriceReader           PriceReader
	OrderClient           OrderClient
	OrderRepository       postgres.OrderRepository
	PositionRepository    postgres.PositionRepository
	PortfolioManager      PortfolioRecorder  // optional
	EvaluationInterval    time.Duration      // how often to poll prices; defaults to 1s
	Logger                *slog.Logger
}

// PositionMonitor evaluates open positions for TP/SL/trailing-stop hits.
type PositionMonitor struct {
	config          PositionMonitorConfig
	activePositions map[int64]*domain.Position
	mu              sync.RWMutex
}

// NewPositionMonitor constructs a PositionMonitor. If EvaluationInterval is zero, defaults to 1s.
func NewPositionMonitor(config PositionMonitorConfig) *PositionMonitor {
	if config.EvaluationInterval <= 0 {
		config.EvaluationInterval = defaultEvaluationInterval
	}
	return &PositionMonitor{
		config:          config,
		activePositions: make(map[int64]*domain.Position),
	}
}

// LoadOpenPositionsFromDatabase populates the in-memory active set from the database.
// Call this once before Run to avoid losing track of positions on restart.
func (monitor *PositionMonitor) LoadOpenPositionsFromDatabase(ctx context.Context) error {
	positions, err := monitor.config.PositionRepository.FindOpenPositions(ctx)
	if err != nil {
		return fmt.Errorf("load open positions from database: %w", err)
	}

	monitor.mu.Lock()
	defer monitor.mu.Unlock()

	for i := range positions {
		p := positions[i]
		monitor.activePositions[p.ID] = &p
	}

	monitor.config.Logger.Info("position monitor: loaded open positions from database",
		"count", len(positions),
	)
	return nil
}

// Run starts the monitor. It listens on the PositionOpenedChannel for new positions and
// evaluates all active positions on each EvaluationInterval tick.
// Blocks until ctx is cancelled.
func (monitor *PositionMonitor) Run(ctx context.Context) error {
	ticker := time.NewTicker(monitor.config.EvaluationInterval)
	defer ticker.Stop()

	positionChan := monitor.config.PositionOpenedChannel

	monitor.config.Logger.Info("position monitor: started",
		"evaluationInterval", monitor.config.EvaluationInterval.String(),
	)

	for {
		select {
		case <-ctx.Done():
			monitor.config.Logger.Info("position monitor: context cancelled, stopping",
				"reason", ctx.Err().Error(),
			)
			return ctx.Err()

		case pos, ok := <-positionChan:
			if !ok {
				// Executor shut down and closed the channel; set to nil so this arm becomes inert.
				monitor.config.Logger.Info("position monitor: position opened channel closed")
				positionChan = nil
				continue
			}
			monitor.addPosition(pos)

		case <-ticker.C:
			monitor.evaluateAllPositions(ctx)
		}
	}
}

// EvaluatePosition is the pure decision core: given a position and current price,
// returns the action to take. Priority: TP > SL > Trailing.
//
// For LONG positions:
//   - currentPrice >= TakeProfitPrice → CloseTakeProfit
//   - currentPrice <= StopLossPrice   → CloseStopLoss
//   - TrailingStopDistance > 0:
//     - currentPrice <= TrailingStopPrice          → CloseTrailingStop
//     - (currentPrice - TrailingStopDistance) > TrailingStopPrice → AdjustTrailingStop
//
// For SHORT positions:
//   - currentPrice <= TakeProfitPrice → CloseTakeProfit
//   - currentPrice >= StopLossPrice   → CloseStopLoss
//
// Returns ActionNone if no trigger.
func EvaluatePosition(position domain.Position, currentPrice decimal.Decimal) PositionAction {
	none := PositionAction{ActionType: PositionActionNone}

	if position.Side == "LONG" {
		// Take-profit: highest priority
		if currentPrice.GreaterThanOrEqual(position.TakeProfitPrice) {
			return PositionAction{ActionType: PositionActionCloseTakeProfit}
		}
		// Stop-loss
		if currentPrice.LessThanOrEqual(position.StopLossPrice) {
			return PositionAction{ActionType: PositionActionCloseStopLoss}
		}
		// Trailing stop
		if position.TrailingStopDistance.IsPositive() {
			if currentPrice.LessThanOrEqual(position.TrailingStopPrice) {
				return PositionAction{ActionType: PositionActionCloseTrailingStop}
			}
			candidateTrailing := currentPrice.Sub(position.TrailingStopDistance)
			if candidateTrailing.GreaterThan(position.TrailingStopPrice) {
				return PositionAction{
					ActionType:           PositionActionAdjustTrailingStop,
					NewTrailingStopPrice: candidateTrailing,
				}
			}
		}
		return none
	}

	if position.Side == "SHORT" {
		// Take-profit: price has fallen to or below TP
		if currentPrice.LessThanOrEqual(position.TakeProfitPrice) {
			return PositionAction{ActionType: PositionActionCloseTakeProfit}
		}
		// Stop-loss: price has risen to or above SL
		if currentPrice.GreaterThanOrEqual(position.StopLossPrice) {
			return PositionAction{ActionType: PositionActionCloseStopLoss}
		}
		return none
	}

	return none
}

// addPosition registers a new position in the active set.
func (monitor *PositionMonitor) addPosition(pos domain.Position) {
	monitor.mu.Lock()
	defer monitor.mu.Unlock()

	monitor.activePositions[pos.ID] = &pos
	monitor.config.Logger.Info("position monitor: tracking new position",
		"positionId", pos.ID,
		"symbol", pos.Symbol,
		"side", pos.Side,
		"entryPrice", pos.EntryPrice.String(),
		"takeProfit", pos.TakeProfitPrice.String(),
		"stopLoss", pos.StopLossPrice.String(),
	)
}

// evaluateAllPositions iterates all active positions and acts on any trigger.
// It copies the active set under the read lock before processing to avoid holding
// the lock across slow I/O operations.
func (monitor *PositionMonitor) evaluateAllPositions(ctx context.Context) {
	monitor.mu.RLock()
	snapshot := make([]*domain.Position, 0, len(monitor.activePositions))
	for _, p := range monitor.activePositions {
		snapshot = append(snapshot, p)
	}
	monitor.mu.RUnlock()

	for _, pos := range snapshot {
		monitor.evaluatePosition(ctx, pos)
	}
}

// evaluatePosition fetches the current price for a position and acts on any trigger.
func (monitor *PositionMonitor) evaluatePosition(ctx context.Context, pos *domain.Position) {
	ticker, err := monitor.config.PriceReader.GetTickerPrice(ctx, pos.Symbol)
	if err != nil {
		monitor.config.Logger.Error("position monitor: failed to read ticker price",
			"positionId", pos.ID,
			"symbol", pos.Symbol,
			"error", err,
		)
		return
	}
	if ticker == nil {
		monitor.config.Logger.Debug("position monitor: ticker price unavailable, skipping",
			"positionId", pos.ID,
			"symbol", pos.Symbol,
		)
		return
	}

	action := EvaluatePosition(*pos, ticker.Price)

	logger := monitor.config.Logger.With(
		"positionId", pos.ID,
		"symbol", pos.Symbol,
		"currentPrice", ticker.Price.String(),
		"triggerType", string(action.ActionType),
	)

	switch action.ActionType {
	case PositionActionNone:
		// No trigger — nothing to do.

	case PositionActionAdjustTrailingStop:
		monitor.adjustTrailingStop(ctx, pos, action.NewTrailingStopPrice, logger)

	case PositionActionCloseTakeProfit:
		monitor.closePosition(ctx, pos, "TAKE_PROFIT", ticker.Price, logger)

	case PositionActionCloseStopLoss:
		monitor.closePosition(ctx, pos, "STOP_LOSS", ticker.Price, logger)

	case PositionActionCloseTrailingStop:
		monitor.closePosition(ctx, pos, "TRAILING_STOP", ticker.Price, logger)
	}
}

// adjustTrailingStop updates the trailing stop price in Postgres and in the active set.
func (monitor *PositionMonitor) adjustTrailingStop(
	ctx context.Context,
	pos *domain.Position,
	newTrailingStopPrice decimal.Decimal,
	logger *slog.Logger,
) {
	oldTrailingStop := pos.TrailingStopPrice

	if err := monitor.config.PositionRepository.UpdatePositionTrailingStop(ctx, pos.ID, newTrailingStopPrice); err != nil {
		logger.Error("position monitor: failed to update trailing stop in database",
			"positionId", pos.ID,
			"oldTrailingStop", oldTrailingStop.String(),
			"newTrailingStop", newTrailingStopPrice.String(),
			"error", err,
		)
		return
	}

	monitor.mu.Lock()
	if active, ok := monitor.activePositions[pos.ID]; ok {
		active.TrailingStopPrice = newTrailingStopPrice
	}
	monitor.mu.Unlock()

	logger.Info("position monitor: trailing stop adjusted",
		"positionId", pos.ID,
		"oldTrailingStop", oldTrailingStop.String(),
		"newTrailingStop", newTrailingStopPrice.String(),
	)
}

// closePosition places an exit order and, on success, marks the position closed.
// On order failure, the position remains active so the next tick can retry.
func (monitor *PositionMonitor) closePosition(
	ctx context.Context,
	pos *domain.Position,
	closeReason string,
	currentPrice decimal.Decimal,
	logger *slog.Logger,
) {
	exitSide := exitOrderSide(pos.Side)

	// Insert pending exit order.
	pendingOrder := domain.Order{
		Exchange: domain.OrderExchangeBinance,
		Symbol:   pos.Symbol,
		Side:     exitSide,
		Quantity: pos.Quantity,
		Status:   domain.OrderStatusPending,
	}

	exitOrderID, err := monitor.config.OrderRepository.InsertOrder(ctx, pendingOrder)
	if err != nil {
		logger.Error("position monitor: failed to insert pending exit order",
			"closeReason", closeReason,
			"error", err,
		)
		return
	}

	// Place the market exit order.
	result, err := monitor.config.OrderClient.PlaceMarketOrder(ctx, pos.Symbol, exitSide, pos.Quantity)
	if err != nil {
		logger.Error("position monitor: exit order placement failed, position remains active",
			"exitOrderId", exitOrderID,
			"closeReason", closeReason,
			"error", err,
		)
		updateErr := monitor.config.OrderRepository.UpdateOrderStatus(
			ctx, exitOrderID,
			domain.OrderStatusRejected,
			decimal.Zero, decimal.Zero, decimal.Zero, "",
			result.RawResponse,
		)
		if updateErr != nil {
			logger.Error("position monitor: failed to update rejected exit order status",
				"exitOrderId", exitOrderID,
				"error", updateErr,
			)
		}
		// Leave the position active — next tick will retry.
		return
	}

	// Update exit order to filled.
	updateErr := monitor.config.OrderRepository.UpdateOrderStatus(
		ctx, exitOrderID,
		domain.OrderStatusFilled,
		result.ExecutedQuantity,
		result.AverageFillPrice,
		result.TotalCommission,
		result.CommissionAsset,
		result.RawResponse,
	)
	if updateErr != nil {
		logger.Error("position monitor: failed to update exit order to filled",
			"exitOrderId", exitOrderID,
			"error", updateErr,
		)
		// Continue — the order is filled on the exchange even if our DB update failed.
	}

	// Compute realized P&L: entry and exit fees combined.
	exitPrice := result.AverageFillPrice
	realizedPnl := computeRealizedPnl(pos.Side, pos.EntryPrice, exitPrice, pos.Quantity, pos.FeeTotal, result.TotalCommission)

	// Close the position in the database.
	if err := monitor.config.PositionRepository.ClosePosition(ctx, pos.ID, exitOrderID, exitPrice, realizedPnl, closeReason); err != nil {
		logger.Error("position monitor: failed to close position in database",
			"exitOrderId", exitOrderID,
			"exitPrice", exitPrice.String(),
			"realizedPnl", realizedPnl.String(),
			"closeReason", closeReason,
			"error", err,
		)
		// Even on DB failure, remove from active set — the exchange fill happened.
		// Operator must reconcile. Leaving in the active set risks a double-close attempt.
	}

	logger.Info("position monitor: position closed",
		"exitOrderId", exitOrderID,
		"exitPrice", exitPrice.String(),
		"realizedPnl", realizedPnl.String(),
		"closeReason", closeReason,
	)

	// Remove from active set.
	monitor.mu.Lock()
	closedPos := *pos
	closedPos.ExitOrderID = &exitOrderID
	closedPos.ExitPrice = exitPrice
	closedPos.RealizedPnl = realizedPnl
	closedPos.Status = "closed"
	closedPos.CloseReason = closeReason
	delete(monitor.activePositions, pos.ID)
	monitor.mu.Unlock()

	// Notify portfolio manager if configured.
	if monitor.config.PortfolioManager != nil {
		if err := monitor.config.PortfolioManager.RecordPositionClose(ctx, closedPos); err != nil {
			logger.Error("position monitor: portfolio manager failed to record position close",
				"error", err,
			)
		}
	}
}

// exitOrderSide returns the exit order side for a given position side.
// LONG positions exit with SELL; SHORT positions exit with BUY.
func exitOrderSide(positionSide string) domain.OrderSide {
	if positionSide == "LONG" {
		return domain.OrderSideSell
	}
	return domain.OrderSideBuy
}

// computeRealizedPnl calculates realized P&L net of entry and exit fees.
// LONG: (exitPrice - entryPrice) * quantity - entryFee - exitFee
// SHORT: (entryPrice - exitPrice) * quantity - entryFee - exitFee
func computeRealizedPnl(
	side string,
	entryPrice, exitPrice, quantity decimal.Decimal,
	entryFee, exitFee decimal.Decimal,
) decimal.Decimal {
	var rawPnl decimal.Decimal
	if side == "LONG" {
		rawPnl = exitPrice.Sub(entryPrice).Mul(quantity)
	} else {
		rawPnl = entryPrice.Sub(exitPrice).Mul(quantity)
	}
	return rawPnl.Sub(entryFee).Sub(exitFee)
}
