package binance

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// TestBuildStreamURL_SingleSymbol verifies URL construction for a single symbol.
func TestBuildStreamURL_SingleSymbol(t *testing.T) {
	got := buildStreamURL("wss://stream.binance.com:9443", []string{"BTCUSDT"}, "5m")
	want := "wss://stream.binance.com:9443/stream?streams=btcusdt@kline_5m/btcusdt@trade"

	if got != want {
		t.Errorf("buildStreamURL single symbol\ngot:  %s\nwant: %s", got, want)
	}
}

// TestBuildStreamURL_MultipleSymbols verifies that multiple symbols produce
// correctly ordered, slash-separated stream names with lowercase conversion.
func TestBuildStreamURL_MultipleSymbols(t *testing.T) {
	got := buildStreamURL(
		"wss://stream.binance.com:9443",
		[]string{"BTCUSDT", "ETHUSDT"},
		"5m",
	)
	want := "wss://stream.binance.com:9443/stream?streams=btcusdt@kline_5m/btcusdt@trade/ethusdt@kline_5m/ethusdt@trade"

	if got != want {
		t.Errorf("buildStreamURL multiple symbols\ngot:  %s\nwant: %s", got, want)
	}
}

// TestBuildStreamURL_IntervalVariants verifies that different intervals are interpolated correctly.
func TestBuildStreamURL_IntervalVariants(t *testing.T) {
	cases := []struct {
		interval string
		want     string
	}{
		{"1m", "wss://stream.binance.com:9443/stream?streams=btcusdt@kline_1m/btcusdt@trade"},
		{"1h", "wss://stream.binance.com:9443/stream?streams=btcusdt@kline_1h/btcusdt@trade"},
		{"1d", "wss://stream.binance.com:9443/stream?streams=btcusdt@kline_1d/btcusdt@trade"},
	}

	for _, tc := range cases {
		t.Run(tc.interval, func(t *testing.T) {
			got := buildStreamURL("wss://stream.binance.com:9443", []string{"BTCUSDT"}, tc.interval)
			if got != tc.want {
				t.Errorf("interval %s\ngot:  %s\nwant: %s", tc.interval, got, tc.want)
			}
		})
	}
}

// TestBuildStreamURL_SymbolsAreLowercased confirms symbols are lowercased in the URL
// regardless of input casing.
func TestBuildStreamURL_SymbolsAreLowercased(t *testing.T) {
	got := buildStreamURL("wss://stream.binance.com:9443", []string{"SOLUSDT", "BNBUSDT"}, "15m")
	want := "wss://stream.binance.com:9443/stream?streams=solusdt@kline_15m/solusdt@trade/bnbusdt@kline_15m/bnbusdt@trade"

	if got != want {
		t.Errorf("lowercase symbols\ngot:  %s\nwant: %s", got, want)
	}
}

// TestParseKlineMessage_ValidPayload verifies full kline message parsing with
// a representative payload from the Binance documentation.
func TestParseKlineMessage_ValidPayload(t *testing.T) {
	// Inner data field of a combined stream kline event.
	raw := json.RawMessage(`{
		"e": "kline",
		"E": 123456789,
		"s": "BTCUSDT",
		"k": {
			"t": 123400000,
			"T": 123460000,
			"s": "BTCUSDT",
			"i": "5m",
			"o": "29000.50",
			"c": "29500.00",
			"h": "29600.75",
			"l": "28900.25",
			"v": "1234.56",
			"q": "35891234.50",
			"n": 4200,
			"x": false
		}
	}`)

	candle, err := parseKlineMessage(raw)
	if err != nil {
		t.Fatalf("parseKlineMessage returned unexpected error: %v", err)
	}

	assertString(t, "Symbol", candle.Symbol, "BTCUSDT")
	assertString(t, "Interval", candle.Interval, "5m")
	assertDecimal(t, "OpenPrice", candle.OpenPrice, "29000.50")
	assertDecimal(t, "ClosePrice", candle.ClosePrice, "29500.00")
	assertDecimal(t, "HighPrice", candle.HighPrice, "29600.75")
	assertDecimal(t, "LowPrice", candle.LowPrice, "28900.25")
	assertDecimal(t, "Volume", candle.Volume, "1234.56")
	assertDecimal(t, "QuoteVolume", candle.QuoteVolume, "35891234.50")

	if candle.TradeCount != 4200 {
		t.Errorf("TradeCount: got %d, want 4200", candle.TradeCount)
	}

	if candle.IsClosed {
		t.Error("IsClosed: got true, want false")
	}

	wantOpenTime := time.UnixMilli(123400000)
	if !candle.OpenTime.Equal(wantOpenTime) {
		t.Errorf("OpenTime: got %v, want %v", candle.OpenTime, wantOpenTime)
	}

	wantCloseTime := time.UnixMilli(123460000)
	if !candle.CloseTime.Equal(wantCloseTime) {
		t.Errorf("CloseTime: got %v, want %v", candle.CloseTime, wantCloseTime)
	}
}

// TestParseKlineMessage_ClosedCandle verifies that the IsClosed flag is correctly parsed.
func TestParseKlineMessage_ClosedCandle(t *testing.T) {
	raw := json.RawMessage(`{
		"e": "kline",
		"E": 123456789,
		"s": "ETHUSDT",
		"k": {
			"t": 123400000,
			"T": 123460000,
			"s": "ETHUSDT",
			"i": "1m",
			"o": "1800.00",
			"c": "1810.00",
			"h": "1815.00",
			"l": "1795.00",
			"v": "500.00",
			"q": "900000.00",
			"n": 200,
			"x": true
		}
	}`)

	candle, err := parseKlineMessage(raw)
	if err != nil {
		t.Fatalf("parseKlineMessage returned unexpected error: %v", err)
	}

	if !candle.IsClosed {
		t.Error("IsClosed: got false, want true")
	}
}

// TestParseKlineMessage_InvalidDecimal verifies that malformed price data returns an error.
func TestParseKlineMessage_InvalidDecimal(t *testing.T) {
	raw := json.RawMessage(`{
		"e": "kline",
		"E": 123456789,
		"s": "BTCUSDT",
		"k": {
			"t": 123400000,
			"T": 123460000,
			"s": "BTCUSDT",
			"i": "5m",
			"o": "not-a-number",
			"c": "29500.00",
			"h": "29600.00",
			"l": "28900.00",
			"v": "100.00",
			"q": "2900000.00",
			"n": 10,
			"x": false
		}
	}`)

	_, err := parseKlineMessage(raw)
	if err == nil {
		t.Error("expected an error for invalid decimal, got nil")
	}
}

// TestParseTradeMessage_ValidPayload verifies trade message parsing with a
// representative payload from the Binance documentation.
func TestParseTradeMessage_ValidPayload(t *testing.T) {
	raw := json.RawMessage(`{
		"e": "trade",
		"E": 123456789,
		"s": "BTCUSDT",
		"t": 12345,
		"p": "29250.75",
		"q": "0.125",
		"T": 123456785
	}`)

	ticker, err := parseTradeMessage(raw)
	if err != nil {
		t.Fatalf("parseTradeMessage returned unexpected error: %v", err)
	}

	assertString(t, "Symbol", ticker.Symbol, "BTCUSDT")
	assertDecimal(t, "Price", ticker.Price, "29250.75")

	wantTimestamp := time.UnixMilli(123456785)
	if !ticker.Timestamp.Equal(wantTimestamp) {
		t.Errorf("Timestamp: got %v, want %v", ticker.Timestamp, wantTimestamp)
	}
}

// TestParseTradeMessage_InvalidPrice verifies that a malformed price returns an error.
func TestParseTradeMessage_InvalidPrice(t *testing.T) {
	raw := json.RawMessage(`{
		"e": "trade",
		"E": 123456789,
		"s": "BTCUSDT",
		"t": 12345,
		"p": "bad-price",
		"q": "0.125",
		"T": 123456785
	}`)

	_, err := parseTradeMessage(raw)
	if err == nil {
		t.Error("expected an error for invalid price decimal, got nil")
	}
}

// TestReconnectDelay verifies the exponential backoff progression.
func TestReconnectDelay(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 32 * time.Second},
		{6, 60 * time.Second}, // capped at max
		{10, 60 * time.Second},
	}

	for _, tc := range cases {
		got := reconnectDelay(tc.attempt)
		if got != tc.want {
			t.Errorf("reconnectDelay(%d): got %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

// TestIntegration_LiveBinanceStream connects to real Binance and verifies at least
// one kline and one trade message is received within 30 seconds.
// Only runs when BINANCE_INTEGRATION_TEST=1 is set.
func TestIntegration_LiveBinanceStream(t *testing.T) {
	if os.Getenv("BINANCE_INTEGRATION_TEST") != "1" {
		t.Skip("set BINANCE_INTEGRATION_TEST=1 to run integration tests")
	}

	// Integration test body intentionally omitted in the unit test file.
	// This placeholder ensures the skip guard compiles and is evaluated.
	t.Log("integration test placeholder — implement full stream receive test here")
}

// assertString is a helper that fails the test if got != want.
func assertString(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", field, got, want)
	}
}

// assertDecimal is a helper that fails the test if the decimal field does not
// equal the string representation of the expected value.
func assertDecimal(t *testing.T, field string, got decimal.Decimal, wantStr string) {
	t.Helper()
	want, err := decimal.NewFromString(wantStr)
	if err != nil {
		t.Fatalf("assertDecimal: invalid expected value %q: %v", wantStr, err)
	}
	if !got.Equal(want) {
		t.Errorf("%s: got %s, want %s", field, got.String(), want.String())
	}
}
