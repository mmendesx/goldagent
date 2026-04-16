package sentiment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewsFetcher_FetchOnce_Success(t *testing.T) {
	body := `{
		"results": [
			{
				"id": 12345,
				"kind": "news",
				"title": "Bitcoin hits new ATH",
				"published_at": "2025-04-16T12:00:00Z",
				"url": "https://example.com/article",
				"source": {"title": "CoinDesk"},
				"currencies": [{"code": "BTC", "title": "Bitcoin"}, {"code": "ETH", "title": "Ethereum"}]
			},
			{
				"id": 67890,
				"kind": "news",
				"title": "Ethereum upgrade announced",
				"published_at": "2025-04-16T11:00:00Z",
				"url": "https://example.com/eth",
				"source": {"title": "Decrypt"},
				"currencies": [{"code": "ETH", "title": "Ethereum"}]
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("auth_token") == "" {
			t.Error("expected auth_token query param to be set")
		}
		if r.URL.Query().Get("public") != "true" {
			t.Error("expected public=true query param")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	fetcher := &NewsFetcher{
		apiKey:     "test-key",
		httpClient: server.Client(),
	}

	// Override base URL by using a custom request approach — we need to patch the URL.
	// Since the fetcher hardcodes the URL, we test via a wrapper approach using the httptest client's transport.
	// Re-create using a transport that redirects to our test server.
	fetcher.httpClient = &http.Client{
		Transport: &redirectTransport{target: server.URL},
	}

	articles, err := fetcher.FetchOnce(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}

	first := articles[0]
	if first.ExternalID != "12345" {
		t.Errorf("expected ExternalID %q, got %q", "12345", first.ExternalID)
	}
	if first.Title != "Bitcoin hits new ATH" {
		t.Errorf("expected title %q, got %q", "Bitcoin hits new ATH", first.Title)
	}
	if first.URL != "https://example.com/article" {
		t.Errorf("expected URL %q, got %q", "https://example.com/article", first.URL)
	}
	if first.Source != "CoinDesk" {
		t.Errorf("expected source %q, got %q", "CoinDesk", first.Source)
	}
	if len(first.CurrencyCodes) != 2 {
		t.Errorf("expected 2 currency codes, got %d", len(first.CurrencyCodes))
	}
	if first.CurrencyCodes[0] != "BTC" {
		t.Errorf("expected first currency code %q, got %q", "BTC", first.CurrencyCodes[0])
	}

	expectedTime, _ := time.Parse(time.RFC3339, "2025-04-16T12:00:00Z")
	if !first.PublishedAt.Equal(expectedTime) {
		t.Errorf("expected published_at %v, got %v", expectedTime, first.PublishedAt)
	}

	second := articles[1]
	if second.ExternalID != "67890" {
		t.Errorf("expected ExternalID %q, got %q", "67890", second.ExternalID)
	}
	if len(second.CurrencyCodes) != 1 || second.CurrencyCodes[0] != "ETH" {
		t.Errorf("expected [ETH], got %v", second.CurrencyCodes)
	}
}

func TestNewsFetcher_FetchOnce_EmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results": []}`))
	}))
	defer server.Close()

	fetcher := &NewsFetcher{
		apiKey: "test-key",
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	articles, err := fetcher.FetchOnce(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(articles) != 0 {
		t.Errorf("expected empty articles, got %d", len(articles))
	}
}

func TestNewsFetcher_FetchOnce_Non200Response(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	fetcher := &NewsFetcher{
		apiKey: "test-key",
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	_, err := fetcher.FetchOnce(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 response, got nil")
	}
}

func TestNewsFetcher_FetchOnce_MissingApiKey(t *testing.T) {
	fetcher := &NewsFetcher{
		apiKey:     "",
		httpClient: &http.Client{},
	}

	_, err := fetcher.FetchOnce(context.Background())
	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
}

func TestNewsFetcher_FetchOnce_MalformedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{not valid json}`))
	}))
	defer server.Close()

	fetcher := &NewsFetcher{
		apiKey: "test-key",
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	_, err := fetcher.FetchOnce(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestNewsFetcher_FetchOnce_ArticleWithNoCurrencies(t *testing.T) {
	body := `{
		"results": [
			{
				"id": 99999,
				"kind": "news",
				"title": "Crypto market overview",
				"published_at": "2025-04-16T10:00:00Z",
				"url": "https://example.com/overview",
				"source": {"title": "CryptoNews"},
				"currencies": []
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	fetcher := &NewsFetcher{
		apiKey: "test-key",
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	articles, err := fetcher.FetchOnce(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(articles) != 1 {
		t.Fatalf("expected 1 article, got %d", len(articles))
	}
	if len(articles[0].CurrencyCodes) != 0 {
		t.Errorf("expected empty currency codes, got %v", articles[0].CurrencyCodes)
	}
}

func TestNewNewsFetcher_DefaultsHttpClient(t *testing.T) {
	fetcher := NewNewsFetcher(NewsFetcherConfig{
		CryptoPanicApiKey: "test-key",
	})

	if fetcher.httpClient == nil {
		t.Error("expected httpClient to be set")
	}
	if fetcher.httpClient.Timeout != defaultFetcherTimeout {
		t.Errorf("expected timeout %v, got %v", defaultFetcherTimeout, fetcher.httpClient.Timeout)
	}
}

func TestNewNewsFetcher_UsesProvidedHttpClient(t *testing.T) {
	custom := &http.Client{Timeout: 5 * time.Second}
	fetcher := NewNewsFetcher(NewsFetcherConfig{
		CryptoPanicApiKey: "test-key",
		HttpClient:        custom,
	})

	if fetcher.httpClient != custom {
		t.Error("expected provided httpClient to be used")
	}
}

// redirectTransport replaces the host of all outgoing requests with the target server URL.
type redirectTransport struct {
	target string
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Parse target to extract host.
	newReq := req.Clone(req.Context())
	newReq.URL.Scheme = "http"
	// Extract host from target URL (strip scheme).
	host := t.target
	if len(host) > 7 && host[:7] == "http://" {
		host = host[7:]
	}
	newReq.URL.Host = host
	return http.DefaultTransport.RoundTrip(newReq)
}
