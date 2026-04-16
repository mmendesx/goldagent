package sentiment

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
)

// SymbolToCurrencyMap maps trading symbols (BTCUSDT) to currency codes (BTC).
// Used to filter CryptoPanic articles relevant to a given trading symbol.
var SymbolToCurrencyMap = map[string]string{
	"BTCUSDT": "BTC",
	"ETHUSDT": "ETH",
	"SOLUSDT": "SOL",
	"BNBUSDT": "BNB",
}

// CoordinatorConfig wires the news fetcher and scorer with the database.
type CoordinatorConfig struct {
	Fetcher             *NewsFetcher
	Scorer              *Scorer
	NewsRepository      postgres.NewsRepository
	SentimentRepository postgres.SentimentRepository
	Symbols             []string      // e.g., ["BTCUSDT", "ETHUSDT"]
	PollInterval        time.Duration // how often to fetch+score
	Logger              *slog.Logger
}

// Coordinator orchestrates the news -> score -> store pipeline.
type Coordinator struct {
	fetcher             *NewsFetcher
	scorer              *Scorer
	newsRepo            postgres.NewsRepository
	sentimentRepo       postgres.SentimentRepository
	symbols             []string
	pollInterval        time.Duration
	logger              *slog.Logger
	// symbolByCurrencyCode is the reverse lookup: "BTC" -> "BTCUSDT"
	symbolByCurrencyCode map[string]string
}

// NewCoordinator constructs a Coordinator.
func NewCoordinator(config CoordinatorConfig) *Coordinator {
	// Build reverse map: currency code -> trading symbol, filtered to configured symbols.
	symbolByCurrencyCode := make(map[string]string, len(config.Symbols))
	for _, tradingSymbol := range config.Symbols {
		if currencyCode, ok := SymbolToCurrencyMap[tradingSymbol]; ok {
			symbolByCurrencyCode[currencyCode] = tradingSymbol
		}
	}

	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Coordinator{
		fetcher:              config.Fetcher,
		scorer:               config.Scorer,
		newsRepo:             config.NewsRepository,
		sentimentRepo:        config.SentimentRepository,
		symbols:              config.Symbols,
		pollInterval:         config.PollInterval,
		logger:               logger,
		symbolByCurrencyCode: symbolByCurrencyCode,
	}
}

// Run blocks: every PollInterval, fetches new articles, scores each
// against each configured symbol, and persists results. Skips articles
// already stored (deduplicated by ExternalID). Returns when ctx is done.
func (coordinator *Coordinator) Run(ctx context.Context) error {
	// Run one cycle immediately before entering the ticker loop.
	coordinator.runCycle(ctx)

	ticker := time.NewTicker(coordinator.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			coordinator.logger.Info("sentiment coordinator stopping", "reason", ctx.Err())
			return ctx.Err()
		case <-ticker.C:
			coordinator.runCycle(ctx)
		}
	}
}

// runCycle executes one fetch-score-persist cycle.
func (coordinator *Coordinator) runCycle(ctx context.Context) {
	articles, err := coordinator.fetcher.FetchOnce(ctx)
	if err != nil {
		coordinator.logger.Error("failed to fetch news articles", "error", err)
		return
	}

	coordinator.logger.Info("fetched news articles", "count", len(articles))

	for _, article := range articles {
		coordinator.processArticle(ctx, article)
	}
}

// processArticle handles deduplication, persistence, and scoring for a single fetched article.
func (coordinator *Coordinator) processArticle(ctx context.Context, fetched FetchedArticle) {
	// Deduplicate by external ID.
	existing, err := coordinator.newsRepo.FindArticleByExternalID(ctx, fetched.ExternalID)
	if err != nil {
		coordinator.logger.Error("failed to check for existing article",
			"external_id", fetched.ExternalID,
			"error", err,
		)
		return
	}
	if existing != nil {
		coordinator.logger.Debug("skipping duplicate article", "external_id", fetched.ExternalID)
		return
	}

	// Persist the article.
	articleID, err := coordinator.newsRepo.InsertArticle(ctx, domain.NewsArticle{
		ExternalID:  fetched.ExternalID,
		Source:      fetched.Source,
		Title:       fetched.Title,
		URL:         fetched.URL,
		PublishedAt: fetched.PublishedAt,
	})
	if err != nil {
		coordinator.logger.Error("failed to insert news article",
			"external_id", fetched.ExternalID,
			"title", fetched.Title,
			"error", err,
		)
		return
	}

	coordinator.logger.Info("inserted news article",
		"article_id", articleID,
		"external_id", fetched.ExternalID,
		"title", fetched.Title,
	)

	// Score for each currency code in the article that maps to a configured trading symbol.
	for _, currencyCode := range fetched.CurrencyCodes {
		tradingSymbol, ok := coordinator.symbolByCurrencyCode[currencyCode]
		if !ok {
			continue
		}

		coordinator.scoreAndPersist(ctx, fetched.Title, currencyCode, tradingSymbol, articleID)
	}
}

// scoreAndPersist scores one article-symbol pair and persists the result.
func (coordinator *Coordinator) scoreAndPersist(
	ctx context.Context,
	title string,
	currencyCode string,
	tradingSymbol string,
	articleID int64,
) {
	scored, err := coordinator.scorer.ScoreArticleForSymbol(ctx, title, currencyCode)
	if err != nil {
		coordinator.logger.Error("failed to score article",
			"article_id", articleID,
			"trading_symbol", tradingSymbol,
			"currency_code", currencyCode,
			"error", err,
		)
		return
	}

	scoreID, err := coordinator.sentimentRepo.InsertScore(ctx, domain.SentimentScore{
		ArticleID:  articleID,
		Symbol:     tradingSymbol,
		Direction:  scored.Direction,
		Confidence: scored.Confidence,
		RawScore:   scored.RawScore,
		ModelUsed:  coordinator.scorer.model,
	})
	if err != nil {
		coordinator.logger.Error("failed to insert sentiment score",
			"article_id", articleID,
			"trading_symbol", tradingSymbol,
			"error", err,
		)
		return
	}

	coordinator.logger.Info("scored and persisted sentiment",
		"score_id", scoreID,
		"article_id", articleID,
		"trading_symbol", tradingSymbol,
		"direction", scored.Direction,
		"confidence", scored.Confidence.String(),
		"raw_score", scored.RawScore.String(),
	)
}

// LatestSentimentForSymbol returns the aggregated sentiment for a symbol
// over the past N hours (typically 24). Result in [-1, 1].
// Used by the decision engine.
func (coordinator *Coordinator) LatestSentimentForSymbol(ctx context.Context, symbol string, lookback time.Duration) (decimal.Decimal, error) {
	since := time.Now().Add(-lookback)

	score, err := coordinator.sentimentRepo.AggregateSentimentForSymbol(ctx, symbol, since)
	if err != nil {
		return decimal.Zero, fmt.Errorf("aggregate sentiment for symbol %q lookback %v: %w", symbol, lookback, err)
	}

	return score, nil
}
