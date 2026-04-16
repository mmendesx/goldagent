package config

import (
	"strings"
	"testing"
)

// requiredEnvVars sets the required environment variables so tests that are
// focused on other behaviour are not blocked by the required-field validation.
func setRequiredEnvVars(t *testing.T) {
	t.Helper()
	t.Setenv("GOLD_DATABASE_URL", "postgres://localhost:5432/goldagent_test")
	t.Setenv("GOLD_REDIS_URL", "redis://localhost:6379")
}

func TestLoadConfiguration_DefaultsAreApplied(t *testing.T) {
	setRequiredEnvVars(t)

	cfg, err := LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration() returned unexpected error: %v", err)
	}

	// Exchange credentials — defaults are empty strings
	if cfg.BinanceApiKey != "" {
		t.Errorf("BinanceApiKey: want %q, got %q", "", cfg.BinanceApiKey)
	}
	if cfg.BinanceApiSecret != "" {
		t.Errorf("BinanceApiSecret: want %q, got %q", "", cfg.BinanceApiSecret)
	}
	if cfg.BinanceWebSocketStreamUrl != "wss://stream.binance.com:9443" {
		t.Errorf("BinanceWebSocketStreamUrl: want %q, got %q", "wss://stream.binance.com:9443", cfg.BinanceWebSocketStreamUrl)
	}
	if cfg.BinanceWebSocketApiUrl != "wss://ws-api.binance.com:443/ws-api/v3" {
		t.Errorf("BinanceWebSocketApiUrl: want %q, got %q", "wss://ws-api.binance.com:443/ws-api/v3", cfg.BinanceWebSocketApiUrl)
	}

	// Trading parameters
	expectedSymbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"}
	if len(cfg.Symbols) != len(expectedSymbols) {
		t.Errorf("Symbols: want %v, got %v", expectedSymbols, cfg.Symbols)
	} else {
		for i, s := range expectedSymbols {
			if cfg.Symbols[i] != s {
				t.Errorf("Symbols[%d]: want %q, got %q", i, s, cfg.Symbols[i])
			}
		}
	}

	if cfg.DefaultInterval != "5m" {
		t.Errorf("DefaultInterval: want %q, got %q", "5m", cfg.DefaultInterval)
	}
	if cfg.ConfidenceThreshold != 70 {
		t.Errorf("ConfidenceThreshold: want %d, got %d", 70, cfg.ConfidenceThreshold)
	}
	if cfg.MaxOpenPositions != 3 {
		t.Errorf("MaxOpenPositions: want %d, got %d", 3, cfg.MaxOpenPositions)
	}
	if cfg.MaxPositionSizePercent != 10.0 {
		t.Errorf("MaxPositionSizePercent: want %v, got %v", 10.0, cfg.MaxPositionSizePercent)
	}
	if cfg.MaxDrawdownPercent != 15.0 {
		t.Errorf("MaxDrawdownPercent: want %v, got %v", 15.0, cfg.MaxDrawdownPercent)
	}
	if !cfg.IsDryRunEnabled {
		t.Errorf("IsDryRunEnabled: want true, got false")
	}
	if cfg.SentimentWeight != 0.3 {
		t.Errorf("SentimentWeight: want %v, got %v", 0.3, cfg.SentimentWeight)
	}
	if cfg.TrailingStopAtrMultiplier != 1.0 {
		t.Errorf("TrailingStopAtrMultiplier: want %v, got %v", 1.0, cfg.TrailingStopAtrMultiplier)
	}

	// Indicator parameters
	if cfg.RsiPeriod != 14 {
		t.Errorf("RsiPeriod: want %d, got %d", 14, cfg.RsiPeriod)
	}
	if cfg.MacdFastPeriod != 12 {
		t.Errorf("MacdFastPeriod: want %d, got %d", 12, cfg.MacdFastPeriod)
	}
	if cfg.MacdSlowPeriod != 26 {
		t.Errorf("MacdSlowPeriod: want %d, got %d", 26, cfg.MacdSlowPeriod)
	}
	if cfg.MacdSignalPeriod != 9 {
		t.Errorf("MacdSignalPeriod: want %d, got %d", 9, cfg.MacdSignalPeriod)
	}
	if cfg.BollingerPeriod != 20 {
		t.Errorf("BollingerPeriod: want %d, got %d", 20, cfg.BollingerPeriod)
	}
	if cfg.BollingerStandardDeviation != 2.0 {
		t.Errorf("BollingerStandardDeviation: want %v, got %v", 2.0, cfg.BollingerStandardDeviation)
	}

	expectedEmaPeriods := []int{9, 21, 50, 200}
	if len(cfg.EmaPeriods) != len(expectedEmaPeriods) {
		t.Errorf("EmaPeriods: want %v, got %v", expectedEmaPeriods, cfg.EmaPeriods)
	} else {
		for i, p := range expectedEmaPeriods {
			if cfg.EmaPeriods[i] != p {
				t.Errorf("EmaPeriods[%d]: want %d, got %d", i, p, cfg.EmaPeriods[i])
			}
		}
	}

	if cfg.AtrPeriod != 14 {
		t.Errorf("AtrPeriod: want %d, got %d", 14, cfg.AtrPeriod)
	}

	// Infrastructure
	if cfg.HttpPort != 8080 {
		t.Errorf("HttpPort: want %d, got %d", 8080, cfg.HttpPort)
	}

	// Required fields set by setRequiredEnvVars
	if cfg.DatabaseUrl != "postgres://localhost:5432/goldagent_test" {
		t.Errorf("DatabaseUrl: want %q, got %q", "postgres://localhost:5432/goldagent_test", cfg.DatabaseUrl)
	}
	if cfg.RedisUrl != "redis://localhost:6379" {
		t.Errorf("RedisUrl: want %q, got %q", "redis://localhost:6379", cfg.RedisUrl)
	}

	// External APIs — defaults are empty strings
	if cfg.CryptoPanicApiKey != "" {
		t.Errorf("CryptoPanicApiKey: want %q, got %q", "", cfg.CryptoPanicApiKey)
	}
	if cfg.AnthropicApiKey != "" {
		t.Errorf("AnthropicApiKey: want %q, got %q", "", cfg.AnthropicApiKey)
	}
}

func TestLoadConfiguration_EnvVarsOverrideDefaults(t *testing.T) {
	setRequiredEnvVars(t)

	t.Setenv("GOLD_HTTP_PORT", "9090")
	t.Setenv("GOLD_DEFAULT_INTERVAL", "15m")
	t.Setenv("GOLD_CONFIDENCE_THRESHOLD", "85")
	t.Setenv("GOLD_DRY_RUN", "false")
	t.Setenv("GOLD_SYMBOLS", "BTCUSDT, ETHUSDT")
	t.Setenv("GOLD_EMA_PERIODS", "10,20,30")

	cfg, err := LoadConfiguration()
	if err != nil {
		t.Fatalf("LoadConfiguration() returned unexpected error: %v", err)
	}

	if cfg.HttpPort != 9090 {
		t.Errorf("HttpPort: want 9090, got %d", cfg.HttpPort)
	}
	if cfg.DefaultInterval != "15m" {
		t.Errorf("DefaultInterval: want %q, got %q", "15m", cfg.DefaultInterval)
	}
	if cfg.ConfidenceThreshold != 85 {
		t.Errorf("ConfidenceThreshold: want 85, got %d", cfg.ConfidenceThreshold)
	}
	if cfg.IsDryRunEnabled {
		t.Errorf("IsDryRunEnabled: want false, got true")
	}

	expectedSymbols := []string{"BTCUSDT", "ETHUSDT"}
	if len(cfg.Symbols) != len(expectedSymbols) {
		t.Errorf("Symbols: want %v, got %v", expectedSymbols, cfg.Symbols)
	}

	expectedEmaPeriods := []int{10, 20, 30}
	if len(cfg.EmaPeriods) != len(expectedEmaPeriods) {
		t.Errorf("EmaPeriods: want %v, got %v", expectedEmaPeriods, cfg.EmaPeriods)
	}
}

func TestLoadConfiguration_MissingRequiredFields_ReturnsError(t *testing.T) {
	// Explicitly clear required vars to simulate a fresh environment
	t.Setenv("GOLD_DATABASE_URL", "")
	t.Setenv("GOLD_REDIS_URL", "")

	_, err := LoadConfiguration()
	if err == nil {
		t.Fatal("LoadConfiguration() expected an error for missing required fields, got nil")
	}

	errorMessage := err.Error()
	if !strings.Contains(errorMessage, "GOLD_DATABASE_URL") {
		t.Errorf("error message should mention GOLD_DATABASE_URL, got: %q", errorMessage)
	}
	if !strings.Contains(errorMessage, "GOLD_REDIS_URL") {
		t.Errorf("error message should mention GOLD_REDIS_URL, got: %q", errorMessage)
	}
}

func TestLoadConfiguration_OnlyDatabaseUrlMissing_ReturnsError(t *testing.T) {
	t.Setenv("GOLD_DATABASE_URL", "")
	t.Setenv("GOLD_REDIS_URL", "redis://localhost:6379")

	_, err := LoadConfiguration()
	if err == nil {
		t.Fatal("LoadConfiguration() expected an error for missing GOLD_DATABASE_URL, got nil")
	}

	if !strings.Contains(err.Error(), "GOLD_DATABASE_URL") {
		t.Errorf("error message should mention GOLD_DATABASE_URL, got: %q", err.Error())
	}
}

func TestLoadConfiguration_OnlyRedisUrlMissing_ReturnsError(t *testing.T) {
	t.Setenv("GOLD_DATABASE_URL", "postgres://localhost:5432/goldagent_test")
	t.Setenv("GOLD_REDIS_URL", "")

	_, err := LoadConfiguration()
	if err == nil {
		t.Fatal("LoadConfiguration() expected an error for missing GOLD_REDIS_URL, got nil")
	}

	if !strings.Contains(err.Error(), "GOLD_REDIS_URL") {
		t.Errorf("error message should mention GOLD_REDIS_URL, got: %q", err.Error())
	}
}
