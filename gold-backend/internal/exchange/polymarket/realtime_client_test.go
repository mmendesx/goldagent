package polymarket

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// newTestLogger returns a discard logger suitable for use in tests.
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// newTestClient creates a RealtimeClient with the given config wired to a test logger.
func newTestClient(config RealtimeClientConfig) *RealtimeClient {
	return NewRealtimeClient(config, newTestLogger())
}

// --- Subscription building ---

func TestBuildSubscriptions_ActivityOnly(t *testing.T) {
	client := newTestClient(RealtimeClientConfig{
		SubscribeToActivity:     true,
		SubscribeToCryptoPrices: false,
	})

	subscriptions := client.buildSubscriptions()

	if len(subscriptions) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subscriptions))
	}
	if subscriptions[0].Topic != "activity" {
		t.Errorf("expected topic 'activity', got %q", subscriptions[0].Topic)
	}
	if subscriptions[0].ClobAuth != nil {
		t.Error("expected no clob_auth when credentials are empty")
	}
}

func TestBuildSubscriptions_CryptoPricesOnly(t *testing.T) {
	client := newTestClient(RealtimeClientConfig{
		SubscribeToActivity:     false,
		SubscribeToCryptoPrices: true,
	})

	subscriptions := client.buildSubscriptions()

	if len(subscriptions) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subscriptions))
	}
	if subscriptions[0].Topic != "crypto_prices" {
		t.Errorf("expected topic 'crypto_prices', got %q", subscriptions[0].Topic)
	}
}

func TestBuildSubscriptions_BothTopics(t *testing.T) {
	client := newTestClient(RealtimeClientConfig{
		SubscribeToActivity:     true,
		SubscribeToCryptoPrices: true,
	})

	subscriptions := client.buildSubscriptions()

	if len(subscriptions) != 2 {
		t.Fatalf("expected 2 subscriptions, got %d", len(subscriptions))
	}
}

func TestBuildSubscriptions_NoTopics(t *testing.T) {
	client := newTestClient(RealtimeClientConfig{
		SubscribeToActivity:     false,
		SubscribeToCryptoPrices: false,
	})

	subscriptions := client.buildSubscriptions()

	if len(subscriptions) != 0 {
		t.Fatalf("expected 0 subscriptions, got %d", len(subscriptions))
	}
}

func TestBuildSubscriptions_ActivityWithCredentials_AttachesClobAuth(t *testing.T) {
	client := newTestClient(RealtimeClientConfig{
		ApiKey:              "test-key",
		ApiSecret:           "test-secret",
		ApiPassphrase:       "test-passphrase",
		SubscribeToActivity: true,
	})

	subscriptions := client.buildSubscriptions()

	if len(subscriptions) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subscriptions))
	}

	auth := subscriptions[0].ClobAuth
	if auth == nil {
		t.Fatal("expected clob_auth to be set when credentials are provided")
	}
	if auth.ApiKey != "test-key" {
		t.Errorf("expected api_key 'test-key', got %q", auth.ApiKey)
	}
	if auth.Secret != "test-secret" {
		t.Errorf("expected secret 'test-secret', got %q", auth.Secret)
	}
	if auth.Passphrase != "test-passphrase" {
		t.Errorf("expected passphrase 'test-passphrase', got %q", auth.Passphrase)
	}
}

func TestBuildSubscriptions_CryptoPrices_NeverAttachesClobAuth(t *testing.T) {
	client := newTestClient(RealtimeClientConfig{
		ApiKey:                  "test-key",
		ApiSecret:               "test-secret",
		ApiPassphrase:           "test-passphrase",
		SubscribeToCryptoPrices: true,
	})

	subscriptions := client.buildSubscriptions()

	if subscriptions[0].ClobAuth != nil {
		t.Error("crypto_prices topic should never carry clob_auth credentials")
	}
}

// --- Message parsing ---

func TestHandleRawMessage_CryptoPrice_ParsesCorrectly(t *testing.T) {
	client := newTestClient(RealtimeClientConfig{SubscribeToCryptoPrices: true})

	payload := cryptoPricePayload{
		Symbol:    "BTC",
		Value:     "65432.10",
		Timestamp: 1713312000000, // 2024-04-17 00:00:00 UTC in ms
	}
	payloadBytes, _ := json.Marshal(payload)

	message := incomingMessage{
		Topic:   "crypto_prices",
		Type:    "price_update",
		Payload: json.RawMessage(payloadBytes),
	}
	raw, _ := json.Marshal(message)

	client.handleRawMessage(raw)

	select {
	case event := <-client.cryptoChannel:
		if event.Symbol != "BTC" {
			t.Errorf("expected symbol 'BTC', got %q", event.Symbol)
		}
		if event.Value.String() != "65432.1" {
			t.Errorf("expected value '65432.1', got %q", event.Value.String())
		}
		expectedTime := time.UnixMilli(1713312000000).UTC()
		if !event.Timestamp.Equal(expectedTime) {
			t.Errorf("expected timestamp %v, got %v", expectedTime, event.Timestamp)
		}
	default:
		t.Fatal("expected an event on cryptoChannel but channel was empty")
	}
}

func TestHandleRawMessage_Activity_ParsesCorrectly(t *testing.T) {
	client := newTestClient(RealtimeClientConfig{SubscribeToActivity: true})

	payload := activityPayload{
		EventType:  "TRADE",
		MarketSlug: "will-btc-hit-100k",
		Side:       "BUY",
		Price:      "0.75",
		Size:       "100",
		Timestamp:  1713312000000,
		User:       "0xdeadbeef",
	}
	payloadBytes, _ := json.Marshal(payload)

	message := incomingMessage{
		Topic:   "activity",
		Type:    "trade",
		Payload: json.RawMessage(payloadBytes),
	}
	raw, _ := json.Marshal(message)

	client.handleRawMessage(raw)

	select {
	case event := <-client.activityChannel:
		if event.EventType != "TRADE" {
			t.Errorf("expected event_type 'TRADE', got %q", event.EventType)
		}
		if event.MarketSlug != "will-btc-hit-100k" {
			t.Errorf("expected market_slug 'will-btc-hit-100k', got %q", event.MarketSlug)
		}
		if event.Side != "BUY" {
			t.Errorf("expected side 'BUY', got %q", event.Side)
		}
		if event.Price.String() != "0.75" {
			t.Errorf("expected price '0.75', got %q", event.Price.String())
		}
		if event.Size.String() != "100" {
			t.Errorf("expected size '100', got %q", event.Size.String())
		}
		if event.User != "0xdeadbeef" {
			t.Errorf("expected user '0xdeadbeef', got %q", event.User)
		}
	default:
		t.Fatal("expected an event on activityChannel but channel was empty")
	}
}

func TestHandleRawMessage_InvalidEnvelope_DropsWithoutPanic(t *testing.T) {
	client := newTestClient(RealtimeClientConfig{SubscribeToActivity: true})

	client.handleRawMessage([]byte(`{invalid json`))

	if len(client.activityChannel) != 0 {
		t.Error("expected no events on channel after invalid envelope")
	}
}

func TestHandleRawMessage_InvalidPrice_DropsEvent(t *testing.T) {
	client := newTestClient(RealtimeClientConfig{SubscribeToActivity: true})

	payload := activityPayload{
		EventType:  "TRADE",
		MarketSlug: "some-market",
		Side:       "BUY",
		Price:      "not-a-number",
		Size:       "100",
		Timestamp:  1713312000000,
	}
	payloadBytes, _ := json.Marshal(payload)

	message := incomingMessage{
		Topic:   "activity",
		Type:    "trade",
		Payload: json.RawMessage(payloadBytes),
	}
	raw, _ := json.Marshal(message)

	client.handleRawMessage(raw)

	if len(client.activityChannel) != 0 {
		t.Error("expected no events on channel when price is unparseable")
	}
}

func TestHandleRawMessage_UnknownTopic_DropsWithoutPanic(t *testing.T) {
	client := newTestClient(RealtimeClientConfig{})

	message := incomingMessage{
		Topic:   "equity_prices",
		Type:    "*",
		Payload: json.RawMessage(`{}`),
	}
	raw, _ := json.Marshal(message)

	// Should not panic.
	client.handleRawMessage(raw)
}

// --- Channel full: non-blocking send ---

func TestHandleRawMessage_FullChannel_DropsWithoutBlocking(t *testing.T) {
	client := newTestClient(RealtimeClientConfig{SubscribeToCryptoPrices: true})

	// Fill the channel to capacity.
	for i := 0; i < channelBufferSize; i++ {
		payload := cryptoPricePayload{Symbol: "ETH", Value: "3000", Timestamp: 1713312000000}
		payloadBytes, _ := json.Marshal(payload)
		msg := incomingMessage{Topic: "crypto_prices", Type: "*", Payload: json.RawMessage(payloadBytes)}
		raw, _ := json.Marshal(msg)
		client.handleRawMessage(raw)
	}

	// One more message on a full channel should not block.
	done := make(chan struct{})
	go func() {
		payload := cryptoPricePayload{Symbol: "ETH", Value: "3001", Timestamp: 1713312001000}
		payloadBytes, _ := json.Marshal(payload)
		msg := incomingMessage{Topic: "crypto_prices", Type: "*", Payload: json.RawMessage(payloadBytes)}
		raw, _ := json.Marshal(msg)
		client.handleRawMessage(raw)
		close(done)
	}()

	select {
	case <-done:
		// Passed: did not block.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handleRawMessage blocked on a full channel")
	}
}

// --- Run loop test against a local mock server ---

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func TestRun_ConnectsSubscribesAndReceivesMessage(t *testing.T) {
	// Spin up a local test WebSocket server.
	cryptoPayload := cryptoPricePayload{Symbol: "BTC", Value: "70000", Timestamp: 1713312000000}
	cryptoPayloadBytes, _ := json.Marshal(cryptoPayload)
	serverMessage := incomingMessage{
		Topic:   "crypto_prices",
		Type:    "*",
		Payload: json.RawMessage(cryptoPayloadBytes),
	}
	serverMessageBytes, _ := json.Marshal(serverMessage)

	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(responseWriter, request, nil)
		if err != nil {
			t.Errorf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		// Read the subscription request.
		_, _, err = conn.ReadMessage()
		if err != nil {
			t.Errorf("read subscription: %v", err)
			return
		}

		// Send one crypto price message.
		_ = conn.WriteMessage(websocket.TextMessage, serverMessageBytes)

		// Hold open until client disconnects.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	// Convert http:// to ws://.
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	client := newTestClient(RealtimeClientConfig{
		BaseUrl:                 wsURL,
		SubscribeToCryptoPrices: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go func() {
		_ = client.Run(ctx)
	}()

	select {
	case event, ok := <-client.CryptoPriceChannel():
		if !ok {
			t.Fatal("channel closed before receiving an event")
		}
		if event.Symbol != "BTC" {
			t.Errorf("expected symbol 'BTC', got %q", event.Symbol)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for crypto price event")
	}
}
