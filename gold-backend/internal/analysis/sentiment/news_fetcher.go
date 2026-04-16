package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const (
	cryptoPanicBaseURL      = "https://cryptopanic.com/api/developer/v2/posts/"
	defaultFetcherTimeout   = 15 * time.Second
	defaultFetchPollInterval = 60 * time.Second
)

// NewsFetcherConfig configures news fetching from CryptoPanic.
type NewsFetcherConfig struct {
	CryptoPanicApiKey string
	PollInterval      time.Duration // default 60s
	HttpClient        *http.Client  // optional; defaults to http.Client{Timeout: 15s}
}

// NewsFetcher polls CryptoPanic for recent crypto news articles.
type NewsFetcher struct {
	apiKey     string
	httpClient *http.Client
}

// FetchedArticle is the parsed CryptoPanic article (intermediate type;
// distinct from domain.NewsArticle which is the persisted shape).
type FetchedArticle struct {
	ExternalID    string
	Title         string
	URL           string
	Source        string
	PublishedAt   time.Time
	CurrencyCodes []string // e.g., ["BTC", "ETH"]
}

// cryptoPanicResponse is the top-level CryptoPanic API response shape.
type cryptoPanicResponse struct {
	Results []cryptoPanicPost `json:"results"`
}

// cryptoPanicPost is a single article from CryptoPanic.
type cryptoPanicPost struct {
	ID          int64                    `json:"id"`
	Kind        string                   `json:"kind"`
	Title       string                   `json:"title"`
	PublishedAt time.Time                `json:"published_at"`
	URL         string                   `json:"url"`
	Source      cryptoPanicSource        `json:"source"`
	Currencies  []cryptoPanicCurrency    `json:"currencies"`
}

type cryptoPanicSource struct {
	Title string `json:"title"`
}

type cryptoPanicCurrency struct {
	Code  string `json:"code"`
	Title string `json:"title"`
}

// NewNewsFetcher constructs a NewsFetcher.
func NewNewsFetcher(config NewsFetcherConfig) *NewsFetcher {
	client := config.HttpClient
	if client == nil {
		client = &http.Client{Timeout: defaultFetcherTimeout}
	}

	return &NewsFetcher{
		apiKey:     config.CryptoPanicApiKey,
		httpClient: client,
	}
}

// FetchOnce performs a single fetch and returns parsed articles.
// Each article includes the list of mentioned currency codes (e.g., ["BTC", "ETH"]).
func (fetcher *NewsFetcher) FetchOnce(ctx context.Context) ([]FetchedArticle, error) {
	if fetcher.apiKey == "" {
		return nil, fmt.Errorf("CryptoPanic API key is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cryptoPanicBaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build CryptoPanic request: %w", err)
	}

	query := req.URL.Query()
	query.Set("auth_token", fetcher.apiKey)
	query.Set("public", "true")
	req.URL.RawQuery = query.Encode()

	resp, err := fetcher.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute CryptoPanic request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("CryptoPanic API returned non-200 status: %d", resp.StatusCode)
	}

	var apiResp cryptoPanicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode CryptoPanic response: %w", err)
	}

	articles := make([]FetchedArticle, 0, len(apiResp.Results))
	for _, post := range apiResp.Results {
		currencyCodes := make([]string, 0, len(post.Currencies))
		for _, c := range post.Currencies {
			if c.Code != "" {
				currencyCodes = append(currencyCodes, c.Code)
			}
		}

		articles = append(articles, FetchedArticle{
			ExternalID:    strconv.FormatInt(post.ID, 10),
			Title:         post.Title,
			URL:           post.URL,
			Source:        post.Source.Title,
			PublishedAt:   post.PublishedAt,
			CurrencyCodes: currencyCodes,
		})
	}

	return articles, nil
}
