package sentiment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

// --- In-memory fakes ---

type fakeNewsRepository struct {
	mu       sync.Mutex
	articles []domain.NewsArticle
	nextID   int64
}

func newFakeNewsRepository() *fakeNewsRepository {
	return &fakeNewsRepository{nextID: 1}
}

func (r *fakeNewsRepository) InsertArticle(_ context.Context, a domain.NewsArticle) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a.ID = r.nextID
	a.CreatedAt = time.Now()
	r.nextID++
	r.articles = append(r.articles, a)
	return a.ID, nil
}

func (r *fakeNewsRepository) FindArticleByExternalID(_ context.Context, externalID string) (*domain.NewsArticle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, a := range r.articles {
		if a.ExternalID == externalID {
			return &r.articles[i], nil
		}
	}
	return nil, nil
}

func (r *fakeNewsRepository) FindRecentArticles(_ context.Context, limit int) ([]domain.NewsArticle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit > len(r.articles) {
		limit = len(r.articles)
	}
	result := make([]domain.NewsArticle, limit)
	copy(result, r.articles[len(r.articles)-limit:])
	return result, nil
}

type fakeSentimentRepository struct {
	mu     sync.Mutex
	scores []domain.SentimentScore
	nextID int64
}

func newFakeSentimentRepository() *fakeSentimentRepository {
	return &fakeSentimentRepository{nextID: 1}
}

func (r *fakeSentimentRepository) InsertScore(_ context.Context, s domain.SentimentScore) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s.ID = r.nextID
	s.CreatedAt = time.Now()
	r.nextID++
	r.scores = append(r.scores, s)
	return s.ID, nil
}

func (r *fakeSentimentRepository) FindLatestScoresBySymbol(_ context.Context, symbol string, limit int) ([]domain.SentimentScore, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []domain.SentimentScore
	for _, s := range r.scores {
		if s.Symbol == symbol {
			result = append(result, s)
		}
	}
	if limit > len(result) {
		limit = len(result)
	}
	return result[:limit], nil
}

func (r *fakeSentimentRepository) AggregateSentimentForSymbol(_ context.Context, symbol string, since time.Time) (decimal.Decimal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var totalWeight decimal.Decimal
	var weightedSum decimal.Decimal

	for _, s := range r.scores {
		if s.Symbol != symbol {
			continue
		}
		if s.CreatedAt.Before(since) {
			continue
		}
		var multiplier decimal.Decimal
		switch s.Direction {
		case domain.SentimentDirectionPositive:
			multiplier = decimal.NewFromInt(1)
		case domain.SentimentDirectionNegative:
			multiplier = decimal.NewFromInt(-1)
		default:
			multiplier = decimal.Zero
		}
		weightedSum = weightedSum.Add(multiplier.Mul(s.Confidence))
		totalWeight = totalWeight.Add(s.Confidence)
	}

	if totalWeight.IsZero() {
		return decimal.Zero, nil
	}
	return weightedSum.Div(totalWeight), nil
}

// --- Test helpers ---

const positiveAnthropicResponse = `{
	"content": [
		{
			"type": "text",
			"text": "{\"direction\": \"positive\", \"confidence\": 0.9, \"raw_score\": 0.8}"
		}
	]
}`

const neutralAnthropicResponse = `{
	"content": [
		{
			"type": "text",
			"text": "{\"direction\": \"neutral\", \"confidence\": 0.5, \"raw_score\": 0.0}"
		}
	]
}`

func cryptoPanicBody(id int, title string, currencies ...string) string {
	currenciesJSON := "["
	for i, c := range currencies {
		if i > 0 {
			currenciesJSON += ","
		}
		currenciesJSON += `{"code": "` + c + `", "title": "` + c + `"}`
	}
	currenciesJSON += "]"

	return `{
		"results": [{
			"id": ` + itoa(id) + `,
			"kind": "news",
			"title": "` + title + `",
			"published_at": "2025-04-16T12:00:00Z",
			"url": "https://example.com/article",
			"source": {"title": "CoinDesk"},
			"currencies": ` + currenciesJSON + `
		}]
	}`
}

func itoa(n int) string {
	s := ""
	if n == 0 {
		return "0"
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// setupCoordinator wires a real Coordinator with real fetcher/scorer backed by httptest servers.
func setupCoordinator(
	t *testing.T,
	cpBody string,
	anthropicBody string,
	symbols []string,
) (*Coordinator, *fakeNewsRepository, *fakeSentimentRepository) {
	t.Helper()

	cpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(cpBody))
	}))
	t.Cleanup(cpServer.Close)

	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(anthropicBody))
	}))
	t.Cleanup(aiServer.Close)

	newsRepo := newFakeNewsRepository()
	sentimentRepo := newFakeSentimentRepository()

	fetcher := NewNewsFetcher(NewsFetcherConfig{
		CryptoPanicApiKey: "test-key",
		HttpClient: &http.Client{
			Transport: &redirectTransport{target: cpServer.URL},
		},
	})

	scorer := NewScorer(ScorerConfig{
		AnthropicApiKey: "test-key",
		HttpClient: &http.Client{
			Transport: &redirectTransport{target: aiServer.URL},
		},
	})

	coord := NewCoordinator(CoordinatorConfig{
		Fetcher:             fetcher,
		Scorer:              scorer,
		NewsRepository:      newsRepo,
		SentimentRepository: sentimentRepo,
		Symbols:             symbols,
		PollInterval:        time.Minute,
	})

	return coord, newsRepo, sentimentRepo
}

// --- Tests ---

func TestCoordinator_DuplicateArticlesAreSkipped(t *testing.T) {
	body := cryptoPanicBody(111, "Bitcoin surges", "BTC")
	coord, newsRepo, sentimentRepo := setupCoordinator(t, body, positiveAnthropicResponse, []string{"BTCUSDT"})

	ctx := context.Background()

	// First cycle: article should be inserted + scored.
	coord.runCycle(ctx)

	if len(newsRepo.articles) != 1 {
		t.Fatalf("expected 1 article after first cycle, got %d", len(newsRepo.articles))
	}
	if len(sentimentRepo.scores) != 1 {
		t.Fatalf("expected 1 sentiment score after first cycle, got %d", len(sentimentRepo.scores))
	}

	// Second cycle: same article must be skipped.
	coord.runCycle(ctx)

	if len(newsRepo.articles) != 1 {
		t.Errorf("expected still 1 article after second cycle (duplicate skipped), got %d", len(newsRepo.articles))
	}
	if len(sentimentRepo.scores) != 1 {
		t.Errorf("expected still 1 sentiment score after second cycle, got %d", len(sentimentRepo.scores))
	}
}

func TestCoordinator_ScoresInsertedPerSymbol(t *testing.T) {
	// Article mentions both BTC and ETH.
	body := cryptoPanicBody(222, "BTC and ETH both rally", "BTC", "ETH")
	coord, newsRepo, sentimentRepo := setupCoordinator(t, body, positiveAnthropicResponse, []string{"BTCUSDT", "ETHUSDT"})

	coord.runCycle(context.Background())

	if len(newsRepo.articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(newsRepo.articles))
	}

	// Should have 2 scores: one for BTCUSDT, one for ETHUSDT.
	if len(sentimentRepo.scores) != 2 {
		t.Fatalf("expected 2 sentiment scores (one per symbol), got %d", len(sentimentRepo.scores))
	}

	scoredSymbols := make(map[string]bool)
	for _, s := range sentimentRepo.scores {
		scoredSymbols[s.Symbol] = true
	}

	if !scoredSymbols["BTCUSDT"] {
		t.Error("expected score for BTCUSDT")
	}
	if !scoredSymbols["ETHUSDT"] {
		t.Error("expected score for ETHUSDT")
	}
}

func TestCoordinator_OnlyConfiguredSymbolsGetScored(t *testing.T) {
	// Article mentions BTC, ETH, and SOL — but coordinator is only configured for BTCUSDT.
	body := cryptoPanicBody(333, "Crypto market roundup", "BTC", "ETH", "SOL")
	coord, newsRepo, sentimentRepo := setupCoordinator(t, body, neutralAnthropicResponse, []string{"BTCUSDT"})

	coord.runCycle(context.Background())

	if len(newsRepo.articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(newsRepo.articles))
	}

	if len(sentimentRepo.scores) != 1 {
		t.Fatalf("expected 1 score (only for BTCUSDT), got %d", len(sentimentRepo.scores))
	}
	if sentimentRepo.scores[0].Symbol != "BTCUSDT" {
		t.Errorf("expected score symbol BTCUSDT, got %q", sentimentRepo.scores[0].Symbol)
	}
}

func TestCoordinator_ArticleWithNoRelevantCurrencies_NoScores(t *testing.T) {
	// DOGE is not in SymbolToCurrencyMap at all, so no configured symbol maps to it.
	body := `{
		"results": [{
			"id": 444,
			"kind": "news",
			"title": "Dogecoin goes to the moon",
			"published_at": "2025-04-16T12:00:00Z",
			"url": "https://example.com",
			"source": {"title": "CryptoNews"},
			"currencies": [{"code": "DOGE", "title": "Dogecoin"}]
		}]
	}`
	coord, newsRepo, sentimentRepo := setupCoordinator(t, body, neutralAnthropicResponse, []string{"BTCUSDT", "ETHUSDT"})

	coord.runCycle(context.Background())

	// Article should still be persisted.
	if len(newsRepo.articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(newsRepo.articles))
	}
	// But no scores since no configured symbol matches.
	if len(sentimentRepo.scores) != 0 {
		t.Errorf("expected 0 scores for unconfigured currency, got %d", len(sentimentRepo.scores))
	}
}

func TestCoordinator_MultipleArticles_AllProcessed(t *testing.T) {
	body := `{
		"results": [
			{
				"id": 501,
				"kind": "news",
				"title": "Bitcoin breaks 100k",
				"published_at": "2025-04-16T12:00:00Z",
				"url": "https://example.com/1",
				"source": {"title": "CoinDesk"},
				"currencies": [{"code": "BTC", "title": "Bitcoin"}]
			},
			{
				"id": 502,
				"kind": "news",
				"title": "Ethereum upgrade live",
				"published_at": "2025-04-16T11:00:00Z",
				"url": "https://example.com/2",
				"source": {"title": "Decrypt"},
				"currencies": [{"code": "ETH", "title": "Ethereum"}]
			}
		]
	}`
	coord, newsRepo, sentimentRepo := setupCoordinator(t, body, positiveAnthropicResponse, []string{"BTCUSDT", "ETHUSDT"})

	coord.runCycle(context.Background())

	if len(newsRepo.articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(newsRepo.articles))
	}
	if len(sentimentRepo.scores) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(sentimentRepo.scores))
	}
}

func TestCoordinator_ScoreStoredWithTradingSymbol(t *testing.T) {
	body := cryptoPanicBody(600, "Solana ecosystem grows", "SOL")
	coord, _, sentimentRepo := setupCoordinator(t, body, positiveAnthropicResponse, []string{"SOLUSDT"})

	coord.runCycle(context.Background())

	if len(sentimentRepo.scores) != 1 {
		t.Fatalf("expected 1 score, got %d", len(sentimentRepo.scores))
	}

	// Score must be stored with the trading symbol (SOLUSDT), not the currency code (SOL).
	if sentimentRepo.scores[0].Symbol != "SOLUSDT" {
		t.Errorf("expected score symbol SOLUSDT, got %q", sentimentRepo.scores[0].Symbol)
	}
}

func TestCoordinator_LatestSentimentForSymbol(t *testing.T) {
	sentimentRepo := newFakeSentimentRepository()

	// Pre-populate a score.
	_, _ = sentimentRepo.InsertScore(context.Background(), domain.SentimentScore{
		ArticleID:  1,
		Symbol:     "BTCUSDT",
		Direction:  domain.SentimentDirectionPositive,
		Confidence: decimal.NewFromFloat(0.8),
		RawScore:   decimal.NewFromFloat(0.7),
	})

	coord := &Coordinator{
		sentimentRepo: sentimentRepo,
	}

	score, err := coord.LatestSentimentForSymbol(context.Background(), "BTCUSDT", 24*time.Hour)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// One positive score with confidence 0.8 => aggregate = 0.8/0.8 = 1.0
	expected := decimal.NewFromFloat(1.0)
	if !score.Equal(expected) {
		t.Errorf("expected aggregate score %s, got %s", expected.String(), score.String())
	}
}

func TestCoordinator_LatestSentimentForSymbol_NoScores_ReturnsZero(t *testing.T) {
	sentimentRepo := newFakeSentimentRepository()

	coord := &Coordinator{
		sentimentRepo: sentimentRepo,
	}

	score, err := coord.LatestSentimentForSymbol(context.Background(), "BTCUSDT", 24*time.Hour)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !score.Equal(decimal.Zero) {
		t.Errorf("expected zero score when no data, got %s", score.String())
	}
}

func TestSymbolToCurrencyMap_ContainsExpectedMappings(t *testing.T) {
	cases := map[string]string{
		"BTCUSDT": "BTC",
		"ETHUSDT": "ETH",
		"SOLUSDT": "SOL",
		"BNBUSDT": "BNB",
	}

	for symbol, expectedCode := range cases {
		code, ok := SymbolToCurrencyMap[symbol]
		if !ok {
			t.Errorf("SymbolToCurrencyMap missing entry for %q", symbol)
			continue
		}
		if code != expectedCode {
			t.Errorf("SymbolToCurrencyMap[%q] = %q, want %q", symbol, code, expectedCode)
		}
	}
}
