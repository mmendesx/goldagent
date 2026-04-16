package sentiment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

func TestScorer_ScoreArticleForSymbol_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") == "" {
			t.Error("expected x-api-key header")
		}
		if r.Header.Get("anthropic-version") != anthropicVersion {
			t.Errorf("expected anthropic-version %q, got %q", anthropicVersion, r.Header.Get("anthropic-version"))
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"content": [
				{
					"type": "text",
					"text": "{\"direction\": \"positive\", \"confidence\": 0.9, \"raw_score\": 0.8}"
				}
			]
		}`))
	}))
	defer server.Close()

	scorer := &Scorer{
		apiKey: "test-key",
		model:  defaultScorerModel,
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	result, err := scorer.ScoreArticleForSymbol(context.Background(), "Bitcoin hits new ATH", "BTC")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Direction != domain.SentimentDirectionPositive {
		t.Errorf("expected direction %q, got %q", domain.SentimentDirectionPositive, result.Direction)
	}

	expectedConfidence := decimal.NewFromFloat(0.9)
	if !result.Confidence.Equal(expectedConfidence) {
		t.Errorf("expected confidence %s, got %s", expectedConfidence.String(), result.Confidence.String())
	}

	expectedRawScore := decimal.NewFromFloat(0.8)
	if !result.RawScore.Equal(expectedRawScore) {
		t.Errorf("expected raw_score %s, got %s", expectedRawScore.String(), result.RawScore.String())
	}
}

func TestScorer_ScoreArticleForSymbol_NegativeSentiment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"content": [
				{
					"type": "text",
					"text": "{\"direction\": \"negative\", \"confidence\": 0.85, \"raw_score\": -0.75}"
				}
			]
		}`))
	}))
	defer server.Close()

	scorer := &Scorer{
		apiKey: "test-key",
		model:  defaultScorerModel,
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	result, err := scorer.ScoreArticleForSymbol(context.Background(), "SEC sues major crypto exchange", "BTC")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if result.Direction != domain.SentimentDirectionNegative {
		t.Errorf("expected direction %q, got %q", domain.SentimentDirectionNegative, result.Direction)
	}
}

func TestScorer_ScoreArticleForSymbol_MalformedResponse_ReturnsNeutral(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"content": [
				{
					"type": "text",
					"text": "I cannot classify this headline."
				}
			]
		}`))
	}))
	defer server.Close()

	scorer := &Scorer{
		apiKey: "test-key",
		model:  defaultScorerModel,
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	// Malformed response should not return an error — it falls back to neutral with low confidence.
	result, err := scorer.ScoreArticleForSymbol(context.Background(), "Some headline", "BTC")
	if err != nil {
		t.Fatalf("expected no error for malformed response, got: %v", err)
	}

	if result.Direction != domain.SentimentDirectionNeutral {
		t.Errorf("expected neutral direction, got %q", result.Direction)
	}

	lowConfidenceThreshold := decimal.NewFromFloat(0.2)
	if result.Confidence.GreaterThan(lowConfidenceThreshold) {
		t.Errorf("expected low confidence for fallback (<=0.2), got %s", result.Confidence.String())
	}
}

func TestScorer_ScoreArticleForSymbol_InvalidDirection_ReturnsNeutral(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"content": [
				{
					"type": "text",
					"text": "{\"direction\": \"bullish\", \"confidence\": 0.9, \"raw_score\": 0.8}"
				}
			]
		}`))
	}))
	defer server.Close()

	scorer := &Scorer{
		apiKey: "test-key",
		model:  defaultScorerModel,
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	result, err := scorer.ScoreArticleForSymbol(context.Background(), "Bitcoin surges", "BTC")
	if err != nil {
		t.Fatalf("expected no error for invalid direction, got: %v", err)
	}
	if result.Direction != domain.SentimentDirectionNeutral {
		t.Errorf("expected neutral direction fallback, got %q", result.Direction)
	}
}

func TestScorer_ScoreArticleForSymbol_ApiError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	scorer := &Scorer{
		apiKey: "test-key",
		model:  defaultScorerModel,
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	_, err := scorer.ScoreArticleForSymbol(context.Background(), "Some headline", "BTC")
	if err == nil {
		t.Fatal("expected error for API 500 response, got nil")
	}
}

func TestScorer_ScoreArticleForSymbol_EmptyContentList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"content": []}`))
	}))
	defer server.Close()

	scorer := &Scorer{
		apiKey: "test-key",
		model:  defaultScorerModel,
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	_, err := scorer.ScoreArticleForSymbol(context.Background(), "Some headline", "BTC")
	if err == nil {
		t.Fatal("expected error for empty content list, got nil")
	}
}

func TestNewScorer_DefaultsModel(t *testing.T) {
	scorer := NewScorer(ScorerConfig{
		AnthropicApiKey: "test-key",
	})

	if scorer.model != defaultScorerModel {
		t.Errorf("expected model %q, got %q", defaultScorerModel, scorer.model)
	}
	if scorer.httpClient == nil {
		t.Error("expected httpClient to be set")
	}
	if scorer.httpClient.Timeout != defaultScorerTimeout {
		t.Errorf("expected timeout %v, got %v", defaultScorerTimeout, scorer.httpClient.Timeout)
	}
}

func TestNewScorer_UsesProvidedModel(t *testing.T) {
	scorer := NewScorer(ScorerConfig{
		AnthropicApiKey: "test-key",
		Model:           "claude-custom-model",
	})

	if scorer.model != "claude-custom-model" {
		t.Errorf("expected model %q, got %q", "claude-custom-model", scorer.model)
	}
}

func TestBuildScorerPrompt_ContainsSymbolAndTitle(t *testing.T) {
	prompt := buildScorerPrompt("Bitcoin hits ATH", "BTC")

	if !strings.Contains(prompt, "BTC") {
		t.Error("prompt should contain symbol 'BTC'")
	}
	if !strings.Contains(prompt, "Bitcoin hits ATH") {
		t.Error("prompt should contain the article title")
	}
	if strings.Contains(prompt, "{SYMBOL}") {
		t.Error("prompt should not contain unreplaced {SYMBOL} placeholder")
	}
	if strings.Contains(prompt, "{TITLE}") {
		t.Error("prompt should not contain unreplaced {TITLE} placeholder")
	}
}

func TestParseSentimentClassification_ValidInput(t *testing.T) {
	cases := []struct {
		name           string
		input          string
		wantDirection  string
		wantConfidence float64
		wantRawScore   float64
	}{
		{
			name:           "positive",
			input:          `{"direction": "positive", "confidence": 0.9, "raw_score": 0.8}`,
			wantDirection:  "positive",
			wantConfidence: 0.9,
			wantRawScore:   0.8,
		},
		{
			name:           "negative",
			input:          `{"direction": "negative", "confidence": 0.7, "raw_score": -0.6}`,
			wantDirection:  "negative",
			wantConfidence: 0.7,
			wantRawScore:   -0.6,
		},
		{
			name:           "neutral",
			input:          `{"direction": "neutral", "confidence": 0.5, "raw_score": 0.0}`,
			wantDirection:  "neutral",
			wantConfidence: 0.5,
			wantRawScore:   0.0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseSentimentClassification(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Direction != tc.wantDirection {
				t.Errorf("expected direction %q, got %q", tc.wantDirection, result.Direction)
			}
			if result.Confidence != tc.wantConfidence {
				t.Errorf("expected confidence %f, got %f", tc.wantConfidence, result.Confidence)
			}
			if result.RawScore != tc.wantRawScore {
				t.Errorf("expected raw_score %f, got %f", tc.wantRawScore, result.RawScore)
			}
		})
	}
}

func TestParseSentimentClassification_InvalidCases(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{name: "invalid json", input: "not json"},
		{name: "invalid direction", input: `{"direction": "bullish", "confidence": 0.9, "raw_score": 0.8}`},
		{name: "confidence out of range high", input: `{"direction": "positive", "confidence": 1.5, "raw_score": 0.8}`},
		{name: "raw_score out of range high", input: `{"direction": "positive", "confidence": 0.9, "raw_score": 2.0}`},
		{name: "negative confidence", input: `{"direction": "neutral", "confidence": -0.1, "raw_score": 0.0}`},
		{name: "raw_score out of range low", input: `{"direction": "negative", "confidence": 0.8, "raw_score": -1.5}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSentimentClassification(tc.input)
			if err == nil {
				t.Errorf("expected error for input %q, got nil", tc.input)
			}
		})
	}
}
