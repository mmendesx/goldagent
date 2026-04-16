package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
// All fields use full descriptive names — no abbreviations.
type Config struct {
	// Exchange credentials
	BinanceApiKey             string
	BinanceApiSecret          string
	BinanceWebSocketStreamUrl string // default: wss://stream.binance.com:9443
	BinanceWebSocketApiUrl    string // default: wss://ws-api.binance.com:443/ws-api/v3

	PolymarketApiKey        string
	PolymarketApiSecret     string
	PolymarketApiPassphrase string

	// Trading parameters
	Symbols                   []string // default: [BTCUSDT, ETHUSDT, SOLUSDT, BNBUSDT]
	DefaultInterval           string   // default: 5m
	ConfidenceThreshold       int      // default: 70
	MaxOpenPositions          int      // default: 3
	MaxPositionSizePercent    float64  // default: 10.0
	MaxDrawdownPercent        float64  // default: 15.0
	IsDryRunEnabled           bool     // default: true
	SentimentWeight           float64  // default: 0.3
	TrailingStopAtrMultiplier float64  // default: 1.0

	// Indicator parameters
	RsiPeriod                  int     // default: 14
	MacdFastPeriod             int     // default: 12
	MacdSlowPeriod             int     // default: 26
	MacdSignalPeriod           int     // default: 9
	BollingerPeriod            int     // default: 20
	BollingerStandardDeviation float64 // default: 2.0
	EmaPeriods                 []int   // default: [9, 21, 50, 200]
	AtrPeriod                  int     // default: 14

	// Infrastructure
	HttpPort    int    // default: 8080
	DatabaseUrl string // required
	RedisUrl    string // required

	// External APIs
	CryptoPanicApiKey string
	AnthropicApiKey   string
}

// LoadConfiguration reads configuration from environment variables and .env file.
// Returns an error if required values are missing.
func LoadConfiguration() (*Config, error) {
	// Try to load .env file — non-fatal if missing or malformed.
	// godotenv does not override variables already set in the environment.
	_ = godotenv.Load()

	cfg := &Config{
		BinanceApiKey:             getEnvString("BINANCE_API_KEY", ""),
		BinanceApiSecret:          getEnvString("BINANCE_API_SECRET", ""),
		BinanceWebSocketStreamUrl: getEnvString("BINANCE_WEBSOCKET_STREAM_URL", "wss://stream.binance.com:9443"),
		BinanceWebSocketApiUrl:    getEnvString("BINANCE_WEBSOCKET_API_URL", "wss://ws-api.binance.com:443/ws-api/v3"),

		PolymarketApiKey:        getEnvString("POLYMARKET_API_KEY", ""),
		PolymarketApiSecret:     getEnvString("POLYMARKET_API_SECRET", ""),
		PolymarketApiPassphrase: getEnvString("POLYMARKET_API_PASSPHRASE", ""),

		Symbols:                   getEnvStringSlice("GOLD_SYMBOLS", []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"}),
		DefaultInterval:           getEnvString("GOLD_DEFAULT_INTERVAL", "5m"),
		ConfidenceThreshold:       getEnvInt("GOLD_CONFIDENCE_THRESHOLD", 70),
		MaxOpenPositions:          getEnvInt("GOLD_MAX_POSITIONS", 3),
		MaxPositionSizePercent:    getEnvFloat64("GOLD_MAX_POSITION_SIZE_PERCENT", 10.0),
		MaxDrawdownPercent:        getEnvFloat64("GOLD_MAX_DRAWDOWN_PERCENT", 15.0),
		IsDryRunEnabled:           getEnvBool("GOLD_DRY_RUN", true),
		SentimentWeight:           getEnvFloat64("GOLD_SENTIMENT_WEIGHT", 0.3),
		TrailingStopAtrMultiplier: getEnvFloat64("GOLD_TRAILING_STOP_ATR_MULTIPLIER", 1.0),

		RsiPeriod:                  getEnvInt("GOLD_RSI_PERIOD", 14),
		MacdFastPeriod:             getEnvInt("GOLD_MACD_FAST", 12),
		MacdSlowPeriod:             getEnvInt("GOLD_MACD_SLOW", 26),
		MacdSignalPeriod:           getEnvInt("GOLD_MACD_SIGNAL", 9),
		BollingerPeriod:            getEnvInt("GOLD_BOLLINGER_PERIOD", 20),
		BollingerStandardDeviation: getEnvFloat64("GOLD_BOLLINGER_STDDEV", 2.0),
		EmaPeriods:                 getEnvIntSlice("GOLD_EMA_PERIODS", []int{9, 21, 50, 200}),
		AtrPeriod:                  getEnvInt("GOLD_ATR_PERIOD", 14),

		HttpPort:    getEnvInt("GOLD_HTTP_PORT", 8080),
		DatabaseUrl: getEnvString("GOLD_DATABASE_URL", ""),
		RedisUrl:    getEnvString("GOLD_REDIS_URL", ""),

		CryptoPanicApiKey: getEnvString("CRYPTOPANIC_API_KEY", ""),
		AnthropicApiKey:   getEnvString("ANTHROPIC_API_KEY", ""),
	}

	if err := validateRequiredFields(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validateRequiredFields checks that all required configuration fields are populated.
// Collects all missing fields into a single error so the caller can fix everything at once.
func validateRequiredFields(cfg *Config) error {
	var missing []string

	if cfg.DatabaseUrl == "" {
		missing = append(missing, "GOLD_DATABASE_URL")
	}

	if cfg.RedisUrl == "" {
		missing = append(missing, "GOLD_REDIS_URL")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}

// getEnvString returns the value of the named environment variable,
// or the provided default if the variable is unset or empty.
func getEnvString(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}

// getEnvInt parses the named environment variable as an integer.
// Returns the default if the variable is unset, empty, or not a valid integer.
func getEnvInt(key string, defaultValue int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}

	return parsed
}

// getEnvFloat64 parses the named environment variable as a float64.
// Returns the default if the variable is unset, empty, or not a valid float.
func getEnvFloat64(key string, defaultValue float64) float64 {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return defaultValue
	}

	return parsed
}

// getEnvBool parses the named environment variable as a boolean.
// Accepts the values understood by strconv.ParseBool: 1, t, T, TRUE, true, True,
// 0, f, F, FALSE, false, False.
// Returns the default if the variable is unset, empty, or not a valid boolean.
func getEnvBool(key string, defaultValue bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return defaultValue
	}

	return parsed
}

// getEnvStringSlice parses the named environment variable as a comma-separated list of strings.
// Trims whitespace from each element. Returns the default if the variable is unset or empty.
func getEnvStringSlice(key string, defaultValue []string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}

	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return defaultValue
	}

	return result
}

// getEnvIntSlice parses the named environment variable as a comma-separated list of integers.
// Skips elements that cannot be parsed as integers.
// Returns the default if the variable is unset, empty, or yields no valid integers.
func getEnvIntSlice(key string, defaultValue []int) []int {
	raw := os.Getenv(key)
	if raw == "" {
		return defaultValue
	}

	parts := strings.Split(raw, ",")
	result := make([]int, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}

		parsed, err := strconv.Atoi(trimmed)
		if err != nil {
			continue
		}

		result = append(result, parsed)
	}

	if len(result) == 0 {
		return defaultValue
	}

	return result
}
