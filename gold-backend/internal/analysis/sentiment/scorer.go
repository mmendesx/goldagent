package sentiment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

const (
	anthropicMessagesURL  = "https://api.anthropic.com/v1/messages"
	anthropicVersion      = "2023-06-01"
	defaultScorerModel    = "claude-haiku-4-5-20251001"
	defaultScorerTimeout  = 30 * time.Second
	scorerMaxTokens       = 200
)

// ScorerConfig configures the LLM-based sentiment scorer.
type ScorerConfig struct {
	AnthropicApiKey string
	Model           string       // default: "claude-haiku-4-5-20251001"
	HttpClient      *http.Client // optional; defaults to http.Client{Timeout: 30s}
}

// Scorer uses Claude to classify article sentiment per asset.
type Scorer struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// ScoredSentiment is the LLM's classification.
type ScoredSentiment struct {
	Direction  domain.SentimentDirection
	Confidence decimal.Decimal // 0.0 - 1.0
	RawScore   decimal.Decimal // signed [-1, 1]: -1 strongly negative, +1 strongly positive
}

// anthropicRequest is the request body sent to the Anthropic API.
type anthropicRequest struct {
	Model     string              `json:"model"`
	MaxTokens int                 `json:"max_tokens"`
	Messages  []anthropicMessage  `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse is the top-level Anthropic API response.
type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// sentimentClassification is the expected JSON shape from the LLM.
type sentimentClassification struct {
	Direction  string  `json:"direction"`
	Confidence float64 `json:"confidence"`
	RawScore   float64 `json:"raw_score"`
}

// NewScorer constructs a Scorer.
func NewScorer(config ScorerConfig) *Scorer {
	client := config.HttpClient
	if client == nil {
		client = &http.Client{Timeout: defaultScorerTimeout}
	}

	model := config.Model
	if model == "" {
		model = defaultScorerModel
	}

	return &Scorer{
		apiKey:     config.AnthropicApiKey,
		model:      model,
		httpClient: client,
	}
}

// ScoreArticleForSymbol returns a sentiment classification for the given
// article relative to the given symbol (e.g., "BTC").
func (scorer *Scorer) ScoreArticleForSymbol(ctx context.Context, articleTitle string, symbol string) (ScoredSentiment, error) {
	prompt := buildScorerPrompt(articleTitle, symbol)

	reqBody := anthropicRequest{
		Model:     scorer.model,
		MaxTokens: scorerMaxTokens,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return neutralSentiment(), fmt.Errorf("marshal Anthropic request for symbol %q: %w", symbol, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicMessagesURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return neutralSentiment(), fmt.Errorf("build Anthropic request for symbol %q: %w", symbol, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", scorer.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := scorer.httpClient.Do(req)
	if err != nil {
		return neutralSentiment(), fmt.Errorf("execute Anthropic request for symbol %q: %w", symbol, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return neutralSentiment(), fmt.Errorf("Anthropic API returned non-200 status %d for symbol %q", resp.StatusCode, symbol)
	}

	var apiResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return neutralSentiment(), fmt.Errorf("decode Anthropic response for symbol %q: %w", symbol, err)
	}

	if len(apiResp.Content) == 0 {
		return neutralSentiment(), fmt.Errorf("empty content in Anthropic response for symbol %q", symbol)
	}

	rawText := apiResp.Content[0].Text
	classification, err := parseSentimentClassification(rawText)
	if err != nil {
		slog.Warn("failed to parse LLM sentiment response; returning neutral",
			"symbol", symbol,
			"title", articleTitle,
			"raw_response", rawText,
			"error", err,
		)
		return neutralSentiment(), nil
	}

	return ScoredSentiment{
		Direction:  domain.SentimentDirection(classification.Direction),
		Confidence: decimal.NewFromFloat(classification.Confidence),
		RawScore:   decimal.NewFromFloat(classification.RawScore),
	}, nil
}

// buildScorerPrompt formats the sentiment classification prompt with the given symbol and title.
func buildScorerPrompt(articleTitle, symbol string) string {
	prompt := `Classify the sentiment of this crypto news headline regarding {SYMBOL}.

Headline: "{TITLE}"

Respond with EXACTLY this JSON format and nothing else:
{"direction": "positive" | "negative" | "neutral", "confidence": 0.0-1.0, "raw_score": -1.0 to 1.0}

- "direction": the sentiment toward {SYMBOL}
- "confidence": how confident you are in this classification (0=uncertain, 1=very confident)
- "raw_score": signed sentiment strength where -1 is strongly negative for {SYMBOL}, 0 is neutral, +1 is strongly positive`

	prompt = strings.ReplaceAll(prompt, "{SYMBOL}", symbol)
	prompt = strings.ReplaceAll(prompt, "{TITLE}", articleTitle)
	return prompt
}

// parseSentimentClassification decodes the LLM text into a structured classification.
// Returns an error if parsing or validation fails.
func parseSentimentClassification(text string) (sentimentClassification, error) {
	// Trim whitespace before attempting parse.
	text = strings.TrimSpace(text)

	var c sentimentClassification
	if err := json.Unmarshal([]byte(text), &c); err != nil {
		return sentimentClassification{}, fmt.Errorf("unmarshal sentiment JSON %q: %w", text, err)
	}

	switch domain.SentimentDirection(c.Direction) {
	case domain.SentimentDirectionPositive, domain.SentimentDirectionNegative, domain.SentimentDirectionNeutral:
		// valid
	default:
		return sentimentClassification{}, fmt.Errorf("invalid sentiment direction %q", c.Direction)
	}

	if c.Confidence < 0.0 || c.Confidence > 1.0 {
		return sentimentClassification{}, fmt.Errorf("confidence %f out of range [0, 1]", c.Confidence)
	}

	if c.RawScore < -1.0 || c.RawScore > 1.0 {
		return sentimentClassification{}, fmt.Errorf("raw_score %f out of range [-1, 1]", c.RawScore)
	}

	return c, nil
}

// neutralSentiment returns a neutral ScoredSentiment with low confidence.
func neutralSentiment() ScoredSentiment {
	return ScoredSentiment{
		Direction:  domain.SentimentDirectionNeutral,
		Confidence: decimal.NewFromFloat(0.1),
		RawScore:   decimal.Zero,
	}
}
