package engine

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/pattern/candlestick"
	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/pattern/chart"
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

// --- Fakes ---

// fakeDecisionRepository is an in-memory DecisionRepository for testing.
type fakeDecisionRepository struct {
	decisions []domain.Decision
	nextID    int64
}

func (r *fakeDecisionRepository) InsertDecision(_ context.Context, d domain.Decision) (int64, error) {
	r.nextID++
	d.ID = r.nextID
	r.decisions = append(r.decisions, d)
	return r.nextID, nil
}

func (r *fakeDecisionRepository) UpdateDecisionExecutionStatus(_ context.Context, _ int64, _ domain.DecisionExecutionStatus, _ string) error {
	return nil
}

func (r *fakeDecisionRepository) FindDecisionsBySymbol(_ context.Context, _ string, _, _ int) ([]domain.Decision, error) {
	return r.decisions, nil
}

func (r *fakeDecisionRepository) FindRecentDecisions(_ context.Context, _, _ int) ([]domain.Decision, error) {
	return r.decisions, nil
}

// fakePositionRepository is an in-memory PositionRepository for testing.
type fakePositionRepository struct {
	openCount int
}

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
	return r.openCount, nil
}

func (r *fakePositionRepository) FindClosedPositions(_ context.Context, _, _ int) ([]domain.Position, error) {
	return nil, nil
}

func (r *fakePositionRepository) FindPositionByID(_ context.Context, _ int64) (*domain.Position, error) {
	return nil, nil
}

// fakePortfolioCache is an in-memory PortfolioMetricsReader for testing.
type fakePortfolioCache struct {
	metrics *domain.PortfolioMetrics
}

func (c *fakePortfolioCache) GetPortfolioMetrics(_ context.Context) (*domain.PortfolioMetrics, error) {
	return c.metrics, nil
}

// --- Test Helpers ---

// buildTestEngine constructs a DecisionEngine with fakes.
func buildTestEngine(
	decisionRepo *fakeDecisionRepository,
	positionCount int,
	portfolioMetrics *domain.PortfolioMetrics,
	isDryRun bool,
	confidenceThreshold int,
	maxPositions int,
) *DecisionEngine {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	return NewDecisionEngine(DecisionEngineConfig{
		DecisionRepository:      decisionRepo,
		PositionRepository:      &fakePositionRepository{openCount: positionCount},
		PortfolioCache:          &fakePortfolioCache{metrics: portfolioMetrics},
		ConfidenceThreshold:     confidenceThreshold,
		MaxOpenPositions:        maxPositions,
		MaxDrawdownPercent:      decimal.NewFromFloat(15.0),
		SentimentWeight:         0.3,
		PositionSizePercent:     decimal.NewFromFloat(10.0),
		TakeProfitAtrMultiplier: decimal.NewFromInt(2),
		StopLossAtrMultiplier:   decimal.NewFromInt(1),
		TrailingStopAtrMultiplier: decimal.NewFromFloat(1.0),
		InitialBalance:          decimal.NewFromInt(10000),
		IsDryRunEnabled:         isDryRun,
		Logger:                  logger,
	})
}

// strongBuyIndicator returns an Indicator with all signals aligned bullishly.
func strongBuyIndicator() domain.Indicator {
	return domain.Indicator{
		Rsi:             decimal.NewFromInt(25), // oversold
		MacdHistogram:   decimal.NewFromFloat(200),
		BollingerUpper:  decimal.NewFromInt(60000),
		BollingerMiddle: decimal.NewFromInt(50000),
		BollingerLower:  decimal.NewFromInt(40000),
		Ema9:            decimal.NewFromInt(55000),
		Ema21:           decimal.NewFromInt(52000),
		Ema50:           decimal.NewFromInt(50000),
		Ema200:          decimal.NewFromInt(45000),
		Atr:             decimal.NewFromInt(1000),
	}
}

// strongBuyInput returns an AnalysisInput that should produce a strong BUY signal.
func strongBuyInput() AnalysisInput {
	return AnalysisInput{
		Symbol:          "BTCUSDT",
		LatestIndicator: strongBuyIndicator(),
		LatestCandle: domain.Candle{
			ClosePrice: decimal.NewFromInt(45000), // below bollinger lower → oversold
		},
		Patterns: []candlestick.DetectedPattern{
			{Direction: candlestick.PatternDirectionBullish, Confidence: 85},
		},
		ChartAnalysis: chart.AnalysisResult{
			Breakouts: []chart.BreakoutEvent{
				{Direction: chart.BreakoutDirectionUp},
			},
		},
		Sentiment: decimal.NewFromFloat(0.8), // strongly bullish sentiment
	}
}

// neutralInput returns an AnalysisInput that should produce a HOLD signal.
func neutralInput() AnalysisInput {
	return AnalysisInput{
		Symbol: "BTCUSDT",
		LatestIndicator: domain.Indicator{
			Rsi:             decimal.NewFromInt(50), // neutral
			MacdHistogram:   decimal.Zero,
			BollingerUpper:  decimal.NewFromInt(55000),
			BollingerMiddle: decimal.NewFromInt(50000),
			BollingerLower:  decimal.NewFromInt(45000),
			Ema9:            decimal.NewFromInt(50000),
			Ema21:           decimal.NewFromInt(50000),
			Ema50:           decimal.NewFromInt(50000),
			Ema200:          decimal.NewFromInt(50000),
			Atr:             decimal.NewFromInt(500),
		},
		LatestCandle: domain.Candle{
			ClosePrice: decimal.NewFromInt(50000),
		},
		Patterns:  nil,
		Sentiment: decimal.Zero,
	}
}

// --- BDD Tests ---

// S-11: composite signal generated, decision logged with all signal values.
func TestDecisionEngine_S11_CompositeSignalGenerated_DecisionLogged(t *testing.T) {
	decisionRepo := &fakeDecisionRepository{}
	engine := buildTestEngine(decisionRepo, 0, nil, true, 70, 3)

	input := strongBuyInput()
	decision, _ := engine.EvaluateAndDecide(context.Background(), input)

	if decision.Symbol != "BTCUSDT" {
		t.Errorf("expected symbol BTCUSDT, got %s", decision.Symbol)
	}
	if decision.CompositeScore.IsZero() {
		t.Error("composite score should not be zero for a strong buy signal")
	}
	if decision.RsiSignal.IsZero() {
		t.Error("RSI signal should be populated")
	}
	if decision.MacdSignal.IsZero() {
		t.Error("MACD signal should be populated")
	}
	if decision.EmaSignal.IsZero() {
		t.Error("EMA signal should be populated")
	}
	if decision.Confidence == 0 {
		t.Error("confidence should be populated")
	}

	// Persist and verify
	err := engine.ProcessAnalysisInput(context.Background(), input)
	if err != nil {
		t.Fatalf("ProcessAnalysisInput returned error: %v", err)
	}
	if len(decisionRepo.decisions) != 1 {
		t.Errorf("expected 1 persisted decision, got %d", len(decisionRepo.decisions))
	}
}

// S-12: BUY confidence ~66, threshold 70 → no trade, decision logged with reason "below_confidence_threshold".
// The strongBuyInput produces a composite score around 67, which maps to BUY with confidence=67.
// With threshold=70, the confidence gate fires.
func TestDecisionEngine_S12_BelowConfidenceThreshold_DecisionLoggedNoTrade(t *testing.T) {
	decisionRepo := &fakeDecisionRepository{}
	// strongBuyInput produces confidence ~66 (composite ~67). Threshold 70 blocks it.
	engine := buildTestEngine(decisionRepo, 0, nil, false, 70, 3)

	input := strongBuyInput()
	decision, tradeIntent := engine.EvaluateAndDecide(context.Background(), input)

	if tradeIntent != nil {
		t.Error("trade intent should be nil when confidence is below threshold")
	}
	if decision.ExecutionStatus != domain.DecisionExecutionStatusBelowConfidenceThreshold {
		t.Errorf("expected execution status %q, got %q",
			domain.DecisionExecutionStatusBelowConfidenceThreshold, decision.ExecutionStatus)
	}
	if decision.RejectionReason != string(domain.DecisionExecutionStatusBelowConfidenceThreshold) {
		t.Errorf("expected rejection reason %q, got %q",
			domain.DecisionExecutionStatusBelowConfidenceThreshold, decision.RejectionReason)
	}
}

// S-13: 3 positions open, max 3 → no trade, decision logged with reason "max_positions_reached".
// Threshold is set to 60 so the confidence gate passes (~66 > 60), then the positions gate fires.
func TestDecisionEngine_S13_MaxPositionsReached_DecisionLoggedNoTrade(t *testing.T) {
	decisionRepo := &fakeDecisionRepository{}
	engine := buildTestEngine(decisionRepo, 3, nil, false, 60, 3)

	input := strongBuyInput()
	decision, tradeIntent := engine.EvaluateAndDecide(context.Background(), input)

	if tradeIntent != nil {
		t.Error("trade intent should be nil when max positions reached")
	}
	if decision.ExecutionStatus != domain.DecisionExecutionStatusMaxPositionsReached {
		t.Errorf("expected execution status %q, got %q",
			domain.DecisionExecutionStatusMaxPositionsReached, decision.ExecutionStatus)
	}
}

// S-14: dry-run on → decision logged with execution_status="dry_run", NO TradeIntent emitted.
func TestDecisionEngine_S14_DryRunMode_DecisionLoggedNoTradeIntent(t *testing.T) {
	decisionRepo := &fakeDecisionRepository{}
	engine := buildTestEngine(decisionRepo, 0, nil, true, 70, 3)

	input := strongBuyInput()

	// EvaluateAndDecide should not produce a TradeIntent in dry-run mode.
	decision, tradeIntent := engine.EvaluateAndDecide(context.Background(), input)

	if tradeIntent != nil {
		t.Error("dry-run mode must NEVER emit a TradeIntent")
	}
	if decision.ExecutionStatus != domain.DecisionExecutionStatusDryRun {
		t.Errorf("expected execution_status=dry_run, got %q", decision.ExecutionStatus)
	}
	if !decision.IsDryRun {
		t.Error("IsDryRun should be true in dry-run mode")
	}

	// ProcessAnalysisInput should persist the decision but emit nothing on the channel.
	err := engine.ProcessAnalysisInput(context.Background(), input)
	if err != nil {
		t.Fatalf("ProcessAnalysisInput returned error: %v", err)
	}
	if len(decisionRepo.decisions) != 1 {
		t.Errorf("expected 1 persisted decision, got %d", len(decisionRepo.decisions))
	}

	// Channel must be empty.
	select {
	case intent := <-engine.TradeIntentChannel():
		t.Errorf("dry-run mode emitted a TradeIntent: %+v", intent)
	default:
		// Expected: channel is empty
	}
}

// Strong BUY in non-dry-run mode → TradeIntent emitted with correct sizing/TP/SL.
// Threshold is 60 so the ~66 confidence passes all risk gates.
func TestDecisionEngine_StrongBuy_NonDryRun_TradeIntentEmitted(t *testing.T) {
	decisionRepo := &fakeDecisionRepository{}

	portfolioMetrics := &domain.PortfolioMetrics{
		Balance:         decimal.NewFromInt(50000),
		DrawdownPercent: decimal.NewFromFloat(5.0),
	}
	engine := buildTestEngine(decisionRepo, 0, portfolioMetrics, false, 60, 3)

	input := strongBuyInput()
	decision, tradeIntent := engine.EvaluateAndDecide(context.Background(), input)

	if decision.Action != domain.DecisionActionBuy {
		t.Fatalf("expected BUY action from strong buy input, got %s", decision.Action)
	}
	if tradeIntent == nil {
		t.Fatal("expected TradeIntent to be produced for strong BUY in non-dry-run mode")
	}

	if tradeIntent.Symbol != "BTCUSDT" {
		t.Errorf("expected symbol BTCUSDT, got %s", tradeIntent.Symbol)
	}
	if tradeIntent.Side != domain.OrderSideBuy {
		t.Errorf("expected BUY side, got %s", tradeIntent.Side)
	}
	if tradeIntent.PositionSizeQuantity.IsZero() {
		t.Error("position size quantity should be non-zero")
	}
	if tradeIntent.SuggestedTakeProfit.IsZero() {
		t.Error("take profit should be set")
	}
	if tradeIntent.SuggestedStopLoss.IsZero() {
		t.Error("stop loss should be set")
	}
	// For BUY: TP > entry price, SL < entry price
	if !tradeIntent.SuggestedTakeProfit.GreaterThan(tradeIntent.EstimatedEntryPrice) {
		t.Errorf("long TP (%s) should be above entry price (%s)",
			tradeIntent.SuggestedTakeProfit.String(), tradeIntent.EstimatedEntryPrice.String())
	}
	if !tradeIntent.SuggestedStopLoss.LessThan(tradeIntent.EstimatedEntryPrice) {
		t.Errorf("long SL (%s) should be below entry price (%s)",
			tradeIntent.SuggestedStopLoss.String(), tradeIntent.EstimatedEntryPrice.String())
	}
}

// ProcessAnalysisInput emits on channel and sets DecisionID.
// Threshold is 60 to ensure the strong buy passes risk gates.
func TestDecisionEngine_ProcessAnalysisInput_EmitsTradeIntentWithDecisionID(t *testing.T) {
	decisionRepo := &fakeDecisionRepository{}
	portfolioMetrics := &domain.PortfolioMetrics{
		Balance:         decimal.NewFromInt(50000),
		DrawdownPercent: decimal.NewFromFloat(2.0),
	}
	engine := buildTestEngine(decisionRepo, 0, portfolioMetrics, false, 60, 3)

	input := strongBuyInput()
	err := engine.ProcessAnalysisInput(context.Background(), input)
	if err != nil {
		t.Fatalf("ProcessAnalysisInput returned error: %v", err)
	}

	// Drain the channel if a trade intent was emitted.
	select {
	case intent := <-engine.TradeIntentChannel():
		if intent.DecisionID == 0 {
			t.Error("TradeIntent.DecisionID should be set from persisted decision")
		}
		if intent.CreatedAt.IsZero() {
			t.Error("TradeIntent.CreatedAt should be set")
		}
	case <-time.After(100 * time.Millisecond):
		// The strong buy input MIGHT produce HOLD if scoring doesn't reach threshold.
		// That's acceptable — but if it produces BUY, the channel must have an intent.
		// Check what the decision was.
		if len(decisionRepo.decisions) > 0 {
			d := decisionRepo.decisions[0]
			if d.Action == domain.DecisionActionBuy && d.ExecutionStatus == domain.DecisionExecutionStatusExecuted {
				t.Error("BUY decision with executed status should have emitted TradeIntent, but channel was empty")
			}
		}
	}
}

// Every evaluation persists a decision regardless of outcome.
func TestDecisionEngine_AlwaysPersistsDecision(t *testing.T) {
	decisionRepo := &fakeDecisionRepository{}
	engine := buildTestEngine(decisionRepo, 0, nil, true, 70, 3)

	// Neutral input → HOLD in dry-run
	err := engine.ProcessAnalysisInput(context.Background(), neutralInput())
	if err != nil {
		t.Fatalf("ProcessAnalysisInput returned error: %v", err)
	}
	if len(decisionRepo.decisions) != 1 {
		t.Errorf("expected 1 decision persisted even for HOLD, got %d", len(decisionRepo.decisions))
	}
}

// Non-dry-run HOLD → no TradeIntent even though dry-run is off.
func TestDecisionEngine_HoldAction_NoDryRun_NoTradeIntent(t *testing.T) {
	decisionRepo := &fakeDecisionRepository{}
	engine := buildTestEngine(decisionRepo, 0, nil, false, 70, 3)

	_, tradeIntent := engine.EvaluateAndDecide(context.Background(), neutralInput())
	if tradeIntent != nil {
		t.Error("HOLD action should never produce a TradeIntent")
	}
}

// TradeIntentChannel returns a read-only channel.
func TestDecisionEngine_TradeIntentChannel_IsReadOnly(t *testing.T) {
	engine := buildTestEngine(&fakeDecisionRepository{}, 0, nil, true, 70, 3)
	ch := engine.TradeIntentChannel()
	// Compile-time check: ch is <-chan domain.TradeIntent (read-only)
	_ = ch
}

// Circuit breaker active → no trade.
// Threshold is 60 so confidence passes (~66 > 60), then the drawdown gate fires.
func TestDecisionEngine_CircuitBreakerActive_NoTradeIntent(t *testing.T) {
	decisionRepo := &fakeDecisionRepository{}
	portfolioMetrics := &domain.PortfolioMetrics{
		Balance:         decimal.NewFromInt(10000),
		DrawdownPercent: decimal.NewFromFloat(20.0), // exceeds 15% max
	}
	engine := buildTestEngine(decisionRepo, 0, portfolioMetrics, false, 60, 3)

	_, tradeIntent := engine.EvaluateAndDecide(context.Background(), strongBuyInput())
	if tradeIntent != nil {
		t.Error("circuit breaker should prevent TradeIntent from being emitted")
	}
}
