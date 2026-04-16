package engine

import (
	"context"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/pattern/candlestick"
	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/pattern/chart"
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
)

const tradeIntentChannelBuffer = 64

// PortfolioMetricsReader is a minimal interface for reading the current portfolio metrics.
// In production, *redis.CacheClient satisfies this via GetPortfolioMetrics.
type PortfolioMetricsReader interface {
	GetPortfolioMetrics(ctx context.Context) (*domain.PortfolioMetrics, error)
}

// AnalysisInput bundles all upstream analysis results for a single decision evaluation.
// The engine consumes these — it does not compute them.
type AnalysisInput struct {
	Symbol          string
	LatestIndicator domain.Indicator
	LatestCandle    domain.Candle
	Patterns        []candlestick.DetectedPattern
	ChartAnalysis   chart.AnalysisResult
	Sentiment       decimal.Decimal // [-1, 1]
}

// DecisionEngineConfig wires the engine with its dependencies and tuning parameters.
type DecisionEngineConfig struct {
	DecisionRepository postgres.DecisionRepository
	PositionRepository postgres.PositionRepository
	PortfolioCache     PortfolioMetricsReader

	ConfidenceThreshold       int
	MaxOpenPositions          int
	MaxDrawdownPercent        decimal.Decimal
	SentimentWeight           float64
	PositionSizePercent       decimal.Decimal
	TakeProfitAtrMultiplier   decimal.Decimal // default 2.0
	StopLossAtrMultiplier     decimal.Decimal // default 1.0
	TrailingStopAtrMultiplier decimal.Decimal
	InitialBalance            decimal.Decimal // fallback when no portfolio metrics available
	IsDryRunEnabled           bool
	Logger                    *slog.Logger
}

// DecisionEngine evaluates analysis inputs and produces decisions.
// It emits TradeIntent values on a buffered channel ONLY when a real trade should
// be placed (i.e. dry-run is disabled AND risk gates pass).
type DecisionEngine struct {
	config             DecisionEngineConfig
	tradeIntentChannel chan domain.TradeIntent
}

// NewDecisionEngine constructs a DecisionEngine with a buffered trade intent channel.
func NewDecisionEngine(config DecisionEngineConfig) *DecisionEngine {
	return &DecisionEngine{
		config:             config,
		tradeIntentChannel: make(chan domain.TradeIntent, tradeIntentChannelBuffer),
	}
}

// TradeIntentChannel returns a read-only channel of trade intents.
// The executor consumes from this channel.
// IMPORTANT: In dry-run mode, NOTHING is ever emitted on this channel.
func (engine *DecisionEngine) TradeIntentChannel() <-chan domain.TradeIntent {
	return engine.tradeIntentChannel
}

// EvaluateAndDecide is the pure-functional core:
// computes all signal scores, derives action+confidence, applies risk gates,
// and returns the Decision (not yet persisted) and an optional TradeIntent.
//
// This is intentionally split from ProcessAnalysisInput so it can be unit-tested
// without a database.
//
// The one I/O concession: CountOpenPositions and GetPortfolioMetrics are called here
// because they represent live system state required for risk gating. On failure,
// the safe default is used (max positions assumed, drawdown zero).
func (engine *DecisionEngine) EvaluateAndDecide(
	ctx context.Context,
	input AnalysisInput,
) (domain.Decision, *domain.TradeIntent) {
	cfg := engine.config
	log := cfg.Logger
	ind := input.LatestIndicator
	currentPrice := input.LatestCandle.ClosePrice

	// Step 1: Compute all individual signal scores.
	weights := DefaultCompositeWeights(cfg.SentimentWeight)
	scores := SignalScores{
		Rsi:                ScoreFromRsi(ind.Rsi),
		Macd:               ScoreFromMacdHistogram(ind.MacdHistogram, currentPrice),
		Bollinger:          ScoreFromBollingerPosition(currentPrice, ind.BollingerUpper, ind.BollingerMiddle, ind.BollingerLower),
		EmaAlignment:       ScoreFromEmaAlignment(ind.Ema9, ind.Ema21, ind.Ema50, ind.Ema200),
		CandlestickPattern: ScoreFromCandlestickPatterns(input.Patterns),
		SupportResistance:  ScoreFromSupportResistance(currentPrice, input.ChartAnalysis),
		Sentiment:          ScoreFromSentiment(input.Sentiment),
	}

	// Step 2: Compute composite score.
	compositeScore := ComputeCompositeScore(scores, weights)

	// Step 3: Derive action and confidence.
	action, confidence := DeriveActionAndConfidence(compositeScore)

	// Step 4: Read open position count (fail safe: assume max if DB unreachable).
	openPositionCount, err := cfg.PositionRepository.CountOpenPositions(ctx)
	if err != nil {
		log.WarnContext(ctx, "failed to count open positions; assuming max for safety",
			"symbol", input.Symbol,
			"error", err,
		)
		openPositionCount = cfg.MaxOpenPositions
	}

	// Step 5: Read current drawdown (fail safe: use zero if cache miss or error).
	currentDrawdown := decimal.Zero
	metrics, err := cfg.PortfolioCache.GetPortfolioMetrics(ctx)
	if err != nil {
		log.WarnContext(ctx, "failed to read portfolio metrics; using zero drawdown",
			"symbol", input.Symbol,
			"error", err,
		)
	} else if metrics != nil {
		currentDrawdown = metrics.DrawdownPercent
	}

	// Step 6: Apply risk gates.
	riskResult := ApplyRiskGates(action, confidence, openPositionCount, currentDrawdown, RiskConfig{
		ConfidenceThreshold: cfg.ConfidenceThreshold,
		MaxOpenPositions:    cfg.MaxOpenPositions,
		MaxDrawdownPercent:  cfg.MaxDrawdownPercent,
	})

	// Step 7: Determine execution status.
	var executionStatus domain.DecisionExecutionStatus
	var rejectionReason string

	switch {
	case cfg.IsDryRunEnabled:
		executionStatus = domain.DecisionExecutionStatusDryRun
	case !riskResult.IsAllowed:
		if riskResult.RejectionReason == "" {
			// HOLD: no rejection to record
			executionStatus = domain.DecisionExecutionStatusRejected
		} else {
			executionStatus = domain.DecisionExecutionStatus(riskResult.RejectionReason)
			rejectionReason = riskResult.RejectionReason
		}
	default:
		executionStatus = domain.DecisionExecutionStatusExecuted
	}

	decision := domain.Decision{
		Symbol:                  input.Symbol,
		Action:                  action,
		Confidence:              confidence,
		ExecutionStatus:         executionStatus,
		RejectionReason:         rejectionReason,
		RsiSignal:               scores.Rsi,
		MacdSignal:              scores.Macd,
		BollingerSignal:         scores.Bollinger,
		EmaSignal:               scores.EmaAlignment,
		PatternSignal:           scores.CandlestickPattern,
		SentimentSignal:         scores.Sentiment,
		SupportResistanceSignal: scores.SupportResistance,
		CompositeScore:          compositeScore,
		IsDryRun:                cfg.IsDryRunEnabled,
		CreatedAt:               time.Now(),
	}

	// Step 8: Build TradeIntent only when conditions are fully met.
	// Dry-run enforcement: TradeIntent is NEVER emitted in dry-run mode.
	if cfg.IsDryRunEnabled || !riskResult.IsAllowed {
		return decision, nil
	}

	// Determine balance for sizing.
	balance := cfg.InitialBalance
	if metrics != nil && metrics.Balance.IsPositive() {
		balance = metrics.Balance
	}

	// Compute position sizing.
	sizingInputs := SizingInputs{
		AccountBalance:            balance,
		PositionSizePercent:       cfg.PositionSizePercent,
		CurrentPrice:              currentPrice,
		AtrValue:                  ind.Atr,
		TakeProfitAtrMultiplier:   cfg.TakeProfitAtrMultiplier,
		StopLossAtrMultiplier:     cfg.StopLossAtrMultiplier,
		TrailingStopAtrMultiplier: cfg.TrailingStopAtrMultiplier,
	}

	var sizing SizingResult
	var orderSide domain.OrderSide
	if action == domain.DecisionActionBuy {
		sizing = ComputeSizingForLong(sizingInputs)
		orderSide = domain.OrderSideBuy
	} else {
		sizing = ComputeSizingForShort(sizingInputs)
		orderSide = domain.OrderSideSell
	}

	tradeIntent := &domain.TradeIntent{
		// DecisionID is set after persisting; populated in ProcessAnalysisInput.
		Symbol:                        input.Symbol,
		Side:                          orderSide,
		EstimatedEntryPrice:           currentPrice,
		PositionSizeQuantity:          sizing.Quantity,
		SuggestedTakeProfit:           sizing.TakeProfitPrice,
		SuggestedStopLoss:             sizing.StopLossPrice,
		SuggestedTrailingStopDistance: sizing.TrailingStopDistance,
		AtrValue:                      ind.Atr,
		CreatedAt:                     time.Now(),
	}

	return decision, tradeIntent
}

// ProcessAnalysisInput evaluates analysis, persists the decision, and emits the
// trade intent on the channel when applicable.
// This is the method called by the orchestrator on each candle close.
func (engine *DecisionEngine) ProcessAnalysisInput(ctx context.Context, input AnalysisInput) error {
	log := engine.config.Logger

	decision, tradeIntent := engine.EvaluateAndDecide(ctx, input)

	// Persist every decision regardless of outcome.
	id, err := engine.config.DecisionRepository.InsertDecision(ctx, decision)
	if err != nil {
		return err
	}
	decision.ID = id

	log.InfoContext(ctx, "decision evaluated",
		"decision_id", id,
		"symbol", decision.Symbol,
		"action", string(decision.Action),
		"confidence", decision.Confidence,
		"composite_score", decision.CompositeScore.String(),
		"execution_status", string(decision.ExecutionStatus),
		"is_dry_run", decision.IsDryRun,
		"rejection_reason", decision.RejectionReason,
	)

	if tradeIntent == nil {
		return nil
	}

	// Stamp the persisted decision ID onto the intent before emitting.
	tradeIntent.DecisionID = id

	// Non-blocking send: a full channel means the executor is lagging.
	// Drop and warn rather than block the evaluation loop.
	select {
	case engine.tradeIntentChannel <- *tradeIntent:
		log.InfoContext(ctx, "trade intent emitted",
			"decision_id", id,
			"symbol", tradeIntent.Symbol,
			"side", string(tradeIntent.Side),
			"estimated_entry_price", tradeIntent.EstimatedEntryPrice.String(),
			"quantity", tradeIntent.PositionSizeQuantity.String(),
			"take_profit", tradeIntent.SuggestedTakeProfit.String(),
			"stop_loss", tradeIntent.SuggestedStopLoss.String(),
		)
	default:
		log.WarnContext(ctx, "trade intent channel full; intent dropped — executor may be lagging",
			"decision_id", id,
			"symbol", tradeIntent.Symbol,
			"side", string(tradeIntent.Side),
		)
	}

	return nil
}
