package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
)

// openTestPool connects to the database identified by GOLD_DATABASE_URL.
// The test is skipped when the variable is absent or the connection fails.
func openTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("GOLD_DATABASE_URL")
	if dsn == "" {
		t.Skip("GOLD_DATABASE_URL not set — skipping integration test")
	}
	pool, err := postgres.NewConnectionPool(context.Background(), dsn)
	if err != nil {
		t.Skipf("cannot connect to test database: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

// mustDecimal panics during tests if s is not a valid decimal string.
func mustDecimal(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("mustDecimal: " + err.Error())
	}
	return d
}

// --- Candle repository ---

func TestCandleRepository_InsertAndFind(t *testing.T) {
	p := openTestPool(t)
	repo := postgres.NewCandleRepository(p)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	candle := domain.Candle{
		Symbol:      "BTCUSDT",
		Interval:    "1m",
		OpenTime:    now,
		CloseTime:   now.Add(time.Minute),
		OpenPrice:   mustDecimal("50000.12345678"),
		HighPrice:   mustDecimal("50100.00000000"),
		LowPrice:    mustDecimal("49900.00000000"),
		ClosePrice:  mustDecimal("50050.12345678"),
		Volume:      mustDecimal("12.34567890"),
		QuoteVolume: mustDecimal("617500.00000000"),
		TradeCount:  42,
		IsClosed:    true,
	}

	id, err := repo.InsertCandle(ctx, candle)
	if err != nil {
		t.Fatalf("InsertCandle: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive ID, got %d", id)
	}
	t.Cleanup(func() {
		p.Exec(context.Background(), "DELETE FROM candles WHERE id = $1 AND symbol = $2", id, candle.Symbol)
	})

	found, err := repo.FindLatestCandles(ctx, "BTCUSDT", "1m", 1)
	if err != nil {
		t.Fatalf("FindLatestCandles: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("expected at least one candle, got none")
	}

	got := found[0]
	if got.Symbol != candle.Symbol {
		t.Errorf("symbol: got %q want %q", got.Symbol, candle.Symbol)
	}
	if !got.OpenPrice.Equal(candle.OpenPrice) {
		t.Errorf("open_price: got %s want %s", got.OpenPrice, candle.OpenPrice)
	}
	if got.TradeCount != candle.TradeCount {
		t.Errorf("trade_count: got %d want %d", got.TradeCount, candle.TradeCount)
	}
}

func TestCandleRepository_InsertCandlesBatch(t *testing.T) {
	p := openTestPool(t)
	repo := postgres.NewCandleRepository(p)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second).Add(-10 * time.Minute)
	candles := make([]domain.Candle, 3)
	for i := range candles {
		candles[i] = domain.Candle{
			Symbol:      "BTCUSDT",
			Interval:    "1m",
			OpenTime:    base.Add(time.Duration(i+1) * time.Minute),
			CloseTime:   base.Add(time.Duration(i+2) * time.Minute),
			OpenPrice:   mustDecimal("60000"),
			HighPrice:   mustDecimal("60100"),
			LowPrice:    mustDecimal("59900"),
			ClosePrice:  mustDecimal("60050"),
			Volume:      mustDecimal("5"),
			QuoteVolume: mustDecimal("300000"),
			TradeCount:  10,
			IsClosed:    true,
		}
	}

	if err := repo.InsertCandlesBatch(ctx, candles); err != nil {
		t.Fatalf("InsertCandlesBatch: %v", err)
	}
	t.Cleanup(func() {
		for _, c := range candles {
			p.Exec(context.Background(),
				"DELETE FROM candles WHERE symbol = $1 AND interval = $2 AND open_time = $3",
				c.Symbol, c.Interval, c.OpenTime)
		}
	})
}

func TestCandleRepository_UpsertCandle(t *testing.T) {
	p := openTestPool(t)
	repo := postgres.NewCandleRepository(p)
	ctx := context.Background()

	openTime := time.Now().UTC().Truncate(time.Second).Add(-30 * time.Minute)
	candle := domain.Candle{
		Symbol:      "BTCUSDT",
		Interval:    "1m",
		OpenTime:    openTime,
		CloseTime:   openTime.Add(time.Minute),
		OpenPrice:   mustDecimal("40000"),
		HighPrice:   mustDecimal("40100"),
		LowPrice:    mustDecimal("39900"),
		ClosePrice:  mustDecimal("40050"),
		Volume:      mustDecimal("1"),
		QuoteVolume: mustDecimal("40000"),
		TradeCount:  5,
		IsClosed:    false,
	}

	id1, err := repo.UpsertCandle(ctx, candle)
	if err != nil {
		t.Fatalf("first UpsertCandle: %v", err)
	}
	t.Cleanup(func() {
		p.Exec(context.Background(), "DELETE FROM candles WHERE id = $1 AND symbol = $2", id1, candle.Symbol)
	})

	// Update — same open_time, new close_price and is_closed.
	candle.ClosePrice = mustDecimal("40099")
	candle.IsClosed = true
	id2, err := repo.UpsertCandle(ctx, candle)
	if err != nil {
		t.Fatalf("second UpsertCandle: %v", err)
	}
	if id1 != id2 {
		t.Errorf("upsert should return same id: first=%d second=%d", id1, id2)
	}
}

// --- Indicator repository ---

func TestIndicatorRepository_InsertAndFind(t *testing.T) {
	p := openTestPool(t)
	candleRepo := postgres.NewCandleRepository(p)
	indRepo := postgres.NewIndicatorRepository(p)
	ctx := context.Background()

	openTime := time.Now().UTC().Truncate(time.Second).Add(-5 * time.Minute)
	candle := domain.Candle{
		Symbol: "BTCUSDT", Interval: "1m",
		OpenTime: openTime, CloseTime: openTime.Add(time.Minute),
		OpenPrice: mustDecimal("50000"), HighPrice: mustDecimal("50100"),
		LowPrice: mustDecimal("49900"), ClosePrice: mustDecimal("50050"),
		Volume: mustDecimal("10"), QuoteVolume: mustDecimal("500000"),
		TradeCount: 20, IsClosed: true,
	}
	candleID, err := candleRepo.InsertCandle(ctx, candle)
	if err != nil {
		t.Fatalf("insert candle for indicator test: %v", err)
	}
	t.Cleanup(func() {
		p.Exec(context.Background(), "DELETE FROM candles WHERE id = $1 AND symbol = $2", candleID, candle.Symbol)
	})

	ind := domain.Indicator{
		CandleID: candleID, Symbol: "BTCUSDT", Interval: "1m",
		Timestamp:       openTime,
		Rsi:             mustDecimal("55.1234"),
		MacdLine:        mustDecimal("100.5"),
		MacdSignal:      mustDecimal("98.3"),
		MacdHistogram:   mustDecimal("2.2"),
		BollingerUpper:  mustDecimal("50500"),
		BollingerMiddle: mustDecimal("50000"),
		BollingerLower:  mustDecimal("49500"),
		Ema9:            mustDecimal("49950"),
		Ema21:           mustDecimal("49800"),
		Ema50:           mustDecimal("49600"),
		Ema200:          mustDecimal("48000"),
		Vwap:            mustDecimal("50025"),
		Atr:             mustDecimal("300"),
	}

	indID, err := indRepo.InsertIndicator(ctx, ind)
	if err != nil {
		t.Fatalf("InsertIndicator: %v", err)
	}
	t.Cleanup(func() {
		p.Exec(context.Background(), "DELETE FROM indicators WHERE id = $1", indID)
	})

	latest, err := indRepo.FindLatestIndicator(ctx, "BTCUSDT", "1m")
	if err != nil {
		t.Fatalf("FindLatestIndicator: %v", err)
	}
	if latest == nil {
		t.Fatal("expected indicator, got nil")
	}
	if !latest.Rsi.Equal(ind.Rsi) {
		t.Errorf("RSI: got %s want %s", latest.Rsi, ind.Rsi)
	}
}

// --- Decision repository ---

func TestDecisionRepository_InsertAndFind(t *testing.T) {
	p := openTestPool(t)
	repo := postgres.NewDecisionRepository(p)
	ctx := context.Background()

	d := domain.Decision{
		Symbol:                  "BTCUSDT",
		Action:                  domain.DecisionActionBuy,
		Confidence:              75,
		ExecutionStatus:         domain.DecisionExecutionStatusPending,
		RsiSignal:               mustDecimal("0.6"),
		MacdSignal:              mustDecimal("0.4"),
		BollingerSignal:         mustDecimal("0.5"),
		EmaSignal:               mustDecimal("0.7"),
		PatternSignal:           mustDecimal("0.3"),
		SentimentSignal:         mustDecimal("0.8"),
		SupportResistanceSignal: mustDecimal("0.5"),
		CompositeScore:          mustDecimal("0.65"),
		IsDryRun:                true,
	}

	id, err := repo.InsertDecision(ctx, d)
	if err != nil {
		t.Fatalf("InsertDecision: %v", err)
	}
	t.Cleanup(func() {
		p.Exec(context.Background(), "DELETE FROM decisions WHERE id = $1", id)
	})

	if err := repo.UpdateDecisionExecutionStatus(ctx, id, domain.DecisionExecutionStatusDryRun, ""); err != nil {
		t.Fatalf("UpdateDecisionExecutionStatus: %v", err)
	}

	decisions, err := repo.FindDecisionsBySymbol(ctx, "BTCUSDT", 10, 0)
	if err != nil {
		t.Fatalf("FindDecisionsBySymbol: %v", err)
	}

	var found *domain.Decision
	for i := range decisions {
		if decisions[i].ID == id {
			found = &decisions[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("inserted decision id=%d not found in FindDecisionsBySymbol results", id)
	}
	if found.ExecutionStatus != domain.DecisionExecutionStatusDryRun {
		t.Errorf("execution_status: got %q want %q", found.ExecutionStatus, domain.DecisionExecutionStatusDryRun)
	}
	if found.Confidence != d.Confidence {
		t.Errorf("confidence: got %d want %d", found.Confidence, d.Confidence)
	}
}

// --- Order repository ---

func TestOrderRepository_InsertAndFind(t *testing.T) {
	p := openTestPool(t)
	repo := postgres.NewOrderRepository(p)
	ctx := context.Background()

	o := domain.Order{
		Exchange:       domain.OrderExchangeBinance,
		Symbol:         "BTCUSDT",
		Side:           domain.OrderSideBuy,
		Quantity:       mustDecimal("0.001"),
		Price:          mustDecimal("50000"),
		FilledQuantity: decimal.Zero,
		Fee:            decimal.Zero,
		Status:         domain.OrderStatusPending,
	}

	id, err := repo.InsertOrder(ctx, o)
	if err != nil {
		t.Fatalf("InsertOrder: %v", err)
	}
	t.Cleanup(func() {
		p.Exec(context.Background(), "DELETE FROM orders WHERE id = $1", id)
	})

	got, err := repo.FindOrderByID(ctx, id)
	if err != nil {
		t.Fatalf("FindOrderByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected order, got nil")
	}
	if got.Symbol != o.Symbol {
		t.Errorf("symbol: got %q want %q", got.Symbol, o.Symbol)
	}
	if !got.Quantity.Equal(o.Quantity) {
		t.Errorf("quantity: got %s want %s", got.Quantity, o.Quantity)
	}

	if err := repo.UpdateOrderStatus(
		ctx, id,
		domain.OrderStatusFilled,
		mustDecimal("0.001"), mustDecimal("50010"),
		mustDecimal("0.05"), "BNB", nil,
	); err != nil {
		t.Fatalf("UpdateOrderStatus: %v", err)
	}

	updated, err := repo.FindOrderByID(ctx, id)
	if err != nil {
		t.Fatalf("FindOrderByID after update: %v", err)
	}
	if updated.Status != domain.OrderStatusFilled {
		t.Errorf("status after update: got %q want %q", updated.Status, domain.OrderStatusFilled)
	}
}

// --- Position repository ---

func TestPositionRepository_InsertAndFind(t *testing.T) {
	p := openTestPool(t)
	posRepo := postgres.NewPositionRepository(p)
	ctx := context.Background()

	pos := domain.Position{
		Symbol:               "BTCUSDT",
		Side:                 "LONG",
		EntryPrice:           mustDecimal("50000"),
		Quantity:             mustDecimal("0.001"),
		TakeProfitPrice:      mustDecimal("55000"),
		StopLossPrice:        mustDecimal("48000"),
		TrailingStopDistance: mustDecimal("500"),
		FeeTotal:             mustDecimal("0.5"),
		Status:               "open",
	}

	id, err := posRepo.InsertPosition(ctx, pos)
	if err != nil {
		t.Fatalf("InsertPosition: %v", err)
	}
	t.Cleanup(func() {
		p.Exec(context.Background(), "DELETE FROM positions WHERE id = $1", id)
	})

	count, err := posRepo.CountOpenPositions(ctx)
	if err != nil {
		t.Fatalf("CountOpenPositions: %v", err)
	}
	if count < 1 {
		t.Error("expected at least 1 open position")
	}

	got, err := posRepo.FindPositionByID(ctx, id)
	if err != nil {
		t.Fatalf("FindPositionByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected position, got nil")
	}
	if !got.EntryPrice.Equal(pos.EntryPrice) {
		t.Errorf("entry_price: got %s want %s", got.EntryPrice, pos.EntryPrice)
	}

	if err := posRepo.UpdatePositionTrailingStop(ctx, id, mustDecimal("49500")); err != nil {
		t.Fatalf("UpdatePositionTrailingStop: %v", err)
	}

	// We need an exit order to satisfy the FK on exit_order_id.
	orderRepo := postgres.NewOrderRepository(p)
	exitOrder := domain.Order{
		Exchange: domain.OrderExchangeBinance, Symbol: "BTCUSDT",
		Side: domain.OrderSideSell, Quantity: mustDecimal("0.001"),
		FilledQuantity: decimal.Zero, Fee: decimal.Zero,
		Status: domain.OrderStatusFilled,
	}
	exitOrderID, err := orderRepo.InsertOrder(ctx, exitOrder)
	if err != nil {
		t.Fatalf("insert exit order: %v", err)
	}
	t.Cleanup(func() {
		p.Exec(context.Background(), "DELETE FROM orders WHERE id = $1", exitOrderID)
	})

	if err := posRepo.ClosePosition(ctx, id, exitOrderID, mustDecimal("51000"), mustDecimal("1"), "TAKE_PROFIT"); err != nil {
		t.Fatalf("ClosePosition: %v", err)
	}

	closed, err := posRepo.FindClosedPositions(ctx, 10, 0)
	if err != nil {
		t.Fatalf("FindClosedPositions: %v", err)
	}
	var foundClosed bool
	for _, cp := range closed {
		if cp.ID == id {
			foundClosed = true
			break
		}
	}
	if !foundClosed {
		t.Errorf("closed position id=%d not found in FindClosedPositions", id)
	}
}

// --- Portfolio repository ---

func TestPortfolioRepository_InsertAndFind(t *testing.T) {
	p := openTestPool(t)
	repo := postgres.NewPortfolioRepository(p)
	ctx := context.Background()

	m := domain.PortfolioMetrics{
		Balance:                mustDecimal("10000"),
		PeakBalance:            mustDecimal("10500"),
		DrawdownPercent:        mustDecimal("4.76"),
		TotalPnl:               mustDecimal("500"),
		WinCount:               7,
		LossCount:              3,
		TotalTrades:            10,
		WinRate:                mustDecimal("0.7"),
		ProfitFactor:           mustDecimal("2.3"),
		AverageWin:             mustDecimal("100"),
		AverageLoss:            mustDecimal("43.5"),
		SharpeRatio:            mustDecimal("1.8"),
		MaxDrawdownPercent:     mustDecimal("10"),
		IsCircuitBreakerActive: false,
	}

	id, err := repo.InsertSnapshot(ctx, m)
	if err != nil {
		t.Fatalf("InsertSnapshot: %v", err)
	}
	t.Cleanup(func() {
		p.Exec(context.Background(), "DELETE FROM portfolio_snapshots WHERE id = $1", id)
	})

	latest, err := repo.FindLatestSnapshot(ctx)
	if err != nil {
		t.Fatalf("FindLatestSnapshot: %v", err)
	}
	if latest == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if !latest.Balance.Equal(m.Balance) {
		t.Errorf("balance: got %s want %s", latest.Balance, m.Balance)
	}
	if latest.WinCount != m.WinCount {
		t.Errorf("win_count: got %d want %d", latest.WinCount, m.WinCount)
	}
}

// --- News repository ---

func TestNewsRepository_InsertAndFind(t *testing.T) {
	p := openTestPool(t)
	repo := postgres.NewNewsRepository(p)
	ctx := context.Background()

	article := domain.NewsArticle{
		ExternalID:  "test-ext-id-001",
		Source:      "coindesk",
		Title:       "Bitcoin hits new ATH",
		URL:         "https://coindesk.com/btc-ath",
		PublishedAt: time.Now().UTC().Add(-1 * time.Hour),
		RawContent:  "Bitcoin has surpassed...",
	}

	id, err := repo.InsertArticle(ctx, article)
	if err != nil {
		t.Fatalf("InsertArticle: %v", err)
	}
	t.Cleanup(func() {
		p.Exec(context.Background(), "DELETE FROM news_articles WHERE id = $1", id)
	})

	found, err := repo.FindArticleByExternalID(ctx, article.ExternalID)
	if err != nil {
		t.Fatalf("FindArticleByExternalID: %v", err)
	}
	if found == nil {
		t.Fatal("expected article, got nil")
	}
	if found.Title != article.Title {
		t.Errorf("title: got %q want %q", found.Title, article.Title)
	}

	recent, err := repo.FindRecentArticles(ctx, 5)
	if err != nil {
		t.Fatalf("FindRecentArticles: %v", err)
	}
	if len(recent) == 0 {
		t.Error("expected at least one recent article")
	}
}

// --- Sentiment repository ---

func TestSentimentRepository_InsertAndAggregate(t *testing.T) {
	p := openTestPool(t)
	newsRepo := postgres.NewNewsRepository(p)
	sentRepo := postgres.NewSentimentRepository(p)
	ctx := context.Background()

	article := domain.NewsArticle{
		ExternalID:  "test-sent-001",
		Source:      "reuters",
		Title:       "Gold rallies on inflation fears",
		PublishedAt: time.Now().UTC().Add(-30 * time.Minute),
	}
	articleID, err := newsRepo.InsertArticle(ctx, article)
	if err != nil {
		t.Fatalf("insert parent article: %v", err)
	}
	t.Cleanup(func() {
		p.Exec(context.Background(), "DELETE FROM news_articles WHERE id = $1", articleID)
	})

	score := domain.SentimentScore{
		ArticleID:  articleID,
		Symbol:     "BTCUSDT",
		Direction:  domain.SentimentDirectionPositive,
		Confidence: mustDecimal("0.85"),
		RawScore:   mustDecimal("0.72"),
		ModelUsed:  "gpt-4",
	}

	scoreID, err := sentRepo.InsertScore(ctx, score)
	if err != nil {
		t.Fatalf("InsertScore: %v", err)
	}
	t.Cleanup(func() {
		p.Exec(context.Background(), "DELETE FROM sentiment_scores WHERE id = $1", scoreID)
	})

	scores, err := sentRepo.FindLatestScoresBySymbol(ctx, "BTCUSDT", 5)
	if err != nil {
		t.Fatalf("FindLatestScoresBySymbol: %v", err)
	}
	if len(scores) == 0 {
		t.Error("expected at least one sentiment score")
	}

	agg, err := sentRepo.AggregateSentimentForSymbol(ctx, "BTCUSDT", time.Now().UTC().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("AggregateSentimentForSymbol: %v", err)
	}
	if agg.LessThanOrEqual(decimal.Zero) {
		t.Errorf("expected positive aggregate sentiment, got %s", agg)
	}
}
