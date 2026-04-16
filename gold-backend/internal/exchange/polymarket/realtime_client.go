package polymarket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

const (
	channelBufferSize    = 256
	reconnectBaseDelay   = 1 * time.Second
	reconnectMaxDelay    = 60 * time.Second
	reconnectMultiplier  = 2
)

// RealtimeClientConfig configures the Polymarket real-time WebSocket client.
type RealtimeClientConfig struct {
	// BaseUrl is the WebSocket endpoint, e.g. "wss://ws-live-data.polymarket.com".
	BaseUrl string

	// ApiKey, ApiSecret and ApiPassphrase are optional credentials for authenticated
	// topic subscriptions (clob_user). Leave empty for public-only topics.
	ApiKey        string
	ApiSecret     string
	ApiPassphrase string

	// SubscribeToActivity enables the "activity" topic (trades and order matches).
	SubscribeToActivity bool

	// SubscribeToCryptoPrices enables the "crypto_prices" topic.
	SubscribeToCryptoPrices bool
}

// RealtimeClient maintains a persistent WebSocket connection to Polymarket and
// publishes parsed events to typed channels. Call Run to start the connection
// loop; it blocks until the context is cancelled.
type RealtimeClient struct {
	config          RealtimeClientConfig
	activityChannel chan domain.PolymarketActivity
	cryptoChannel   chan domain.PolymarketCryptoPrice
	logger          *slog.Logger
}

// NewRealtimeClient constructs a RealtimeClient ready to be started with Run.
func NewRealtimeClient(config RealtimeClientConfig, logger *slog.Logger) *RealtimeClient {
	return &RealtimeClient{
		config:          config,
		activityChannel: make(chan domain.PolymarketActivity, channelBufferSize),
		cryptoChannel:   make(chan domain.PolymarketCryptoPrice, channelBufferSize),
		logger:          logger,
	}
}

// ActivityChannel returns a read-only channel of parsed activity events (trades
// and order matches). The channel is closed when Run returns.
func (client *RealtimeClient) ActivityChannel() <-chan domain.PolymarketActivity {
	return client.activityChannel
}

// CryptoPriceChannel returns a read-only channel of parsed crypto price updates.
// The channel is closed when Run returns.
func (client *RealtimeClient) CryptoPriceChannel() <-chan domain.PolymarketCryptoPrice {
	return client.cryptoChannel
}

// Run connects to Polymarket, subscribes to the configured topics, and publishes
// parsed messages to the typed channels. It reconnects automatically on
// disconnection using exponential backoff (1 s to 60 s). The backoff resets to
// 1 s whenever a connection succeeds and receives at least one message. Blocks
// until ctx is cancelled, then closes both channels.
func (client *RealtimeClient) Run(ctx context.Context) error {
	defer close(client.activityChannel)
	defer close(client.cryptoChannel)

	delay := reconnectBaseDelay

	for {
		if err := ctx.Err(); err != nil {
			client.logger.Info("polymarket realtime client stopping", "reason", err.Error())
			return nil
		}

		receivedMessages, err := client.runOnce(ctx)
		if err == nil {
			// Context was cancelled cleanly inside runOnce.
			return nil
		}

		// Reset backoff when the connection was healthy enough to receive data.
		// This prevents a brief network blip from keeping the delay at maximum
		// for the next reconnect attempt after an otherwise-stable session.
		if receivedMessages {
			delay = reconnectBaseDelay
		}

		client.logger.Warn("polymarket websocket disconnected, reconnecting",
			"error", err.Error(),
			"delay", delay.String(),
		)

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay):
		}

		delay = min(delay*reconnectMultiplier, reconnectMaxDelay)
	}
}

// runOnce establishes one WebSocket connection, subscribes, and reads messages
// until either the context is cancelled (returns false, nil) or an error occurs
// (returns whether at least one message was received before the error, plus the
// error itself). The caller uses the boolean to decide whether to reset backoff.
func (client *RealtimeClient) runOnce(ctx context.Context) (receivedMessages bool, err error) {
	conn, _, dialErr := websocket.DefaultDialer.DialContext(ctx, client.config.BaseUrl, nil)
	if dialErr != nil {
		return false, fmt.Errorf("dial %s: %w", client.config.BaseUrl, dialErr)
	}
	defer conn.Close()

	client.logger.Info("polymarket websocket connected", "url", client.config.BaseUrl)

	if subscribeErr := client.sendSubscriptions(conn); subscribeErr != nil {
		return false, fmt.Errorf("send subscriptions: %w", subscribeErr)
	}

	// Ensure the read loop exits when the context is cancelled.
	go func() {
		<-ctx.Done()
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		conn.Close()
	}()

	for {
		_, raw, readErr := conn.ReadMessage()
		if readErr != nil {
			if ctx.Err() != nil {
				return receivedMessages, nil // clean shutdown
			}
			return receivedMessages, fmt.Errorf("read message: %w", readErr)
		}

		receivedMessages = true
		client.handleRawMessage(raw)
	}
}

// sendSubscriptions builds and sends the subscription request for all configured topics.
func (client *RealtimeClient) sendSubscriptions(conn *websocket.Conn) error {
	subscriptions := client.buildSubscriptions()
	if len(subscriptions) == 0 {
		client.logger.Warn("polymarket realtime client has no topics configured; no subscriptions sent")
		return nil
	}

	request := subscribeRequest{
		Action:        "subscribe",
		Subscriptions: subscriptions,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal subscribe request: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return fmt.Errorf("write subscribe request: %w", err)
	}

	client.logger.Info("polymarket subscriptions sent", "topics", topicNames(subscriptions))
	return nil
}

// buildSubscriptions constructs the slice of subscriptionTopic entries based on
// the client configuration.
func (client *RealtimeClient) buildSubscriptions() []subscriptionTopic {
	var subscriptions []subscriptionTopic

	hasCredentials := client.config.ApiKey != "" &&
		client.config.ApiSecret != "" &&
		client.config.ApiPassphrase != ""

	var credentials *clobAuthCredentials
	if hasCredentials {
		credentials = &clobAuthCredentials{
			ApiKey:     client.config.ApiKey,
			Secret:     client.config.ApiSecret,
			Passphrase: client.config.ApiPassphrase,
		}
	}

	if client.config.SubscribeToActivity {
		topic := subscriptionTopic{
			Topic:   "activity",
			Type:    "*",
			Filters: "{}",
		}
		if hasCredentials {
			topic.ClobAuth = credentials
		}
		subscriptions = append(subscriptions, topic)
	}

	if client.config.SubscribeToCryptoPrices {
		subscriptions = append(subscriptions, subscriptionTopic{
			Topic:   "crypto_prices",
			Type:    "*",
			Filters: "{}",
		})
	}

	return subscriptions
}

// handleRawMessage parses an incoming WebSocket message and routes it to the
// appropriate channel. Unrecognised topics are logged and silently dropped.
func (client *RealtimeClient) handleRawMessage(raw []byte) {
	var message incomingMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		client.logger.Warn("polymarket failed to parse incoming message envelope",
			"error", err.Error(),
			"raw", truncate(string(raw), 200),
		)
		return
	}

	switch message.Topic {
	case "activity":
		client.handleActivityMessage(message)
	case "crypto_prices":
		client.handleCryptoPriceMessage(message)
	default:
		client.logger.Debug("polymarket received unhandled topic",
			"topic", message.Topic,
			"type", message.Type,
		)
	}
}

// handleActivityMessage parses an activity payload and pushes it onto the
// activity channel. Drops the event with a warning if the channel is full.
func (client *RealtimeClient) handleActivityMessage(message incomingMessage) {
	var payload activityPayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		client.logger.Warn("polymarket failed to parse activity payload",
			"error", err.Error(),
			"payload", truncate(string(message.Payload), 200),
		)
		return
	}

	price, err := decimal.NewFromString(payload.Price)
	if err != nil {
		client.logger.Warn("polymarket activity has unparseable price",
			"price", payload.Price,
			"market_slug", payload.MarketSlug,
			"error", err.Error(),
		)
		return
	}

	size, err := decimal.NewFromString(payload.Size)
	if err != nil {
		client.logger.Warn("polymarket activity has unparseable size",
			"size", payload.Size,
			"market_slug", payload.MarketSlug,
			"error", err.Error(),
		)
		return
	}

	event := domain.PolymarketActivity{
		EventType:  payload.EventType,
		MarketSlug: payload.MarketSlug,
		Side:       payload.Side,
		Price:      price,
		Size:       size,
		Timestamp:  time.UnixMilli(payload.Timestamp).UTC(),
		User:       payload.User,
	}

	select {
	case client.activityChannel <- event:
	default:
		client.logger.Warn("polymarket activity channel full, dropping event",
			"market_slug", event.MarketSlug,
			"event_type", event.EventType,
		)
	}
}

// handleCryptoPriceMessage parses a crypto_prices payload and pushes it onto the
// crypto channel. Drops the event with a warning if the channel is full.
func (client *RealtimeClient) handleCryptoPriceMessage(message incomingMessage) {
	var payload cryptoPricePayload
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		client.logger.Warn("polymarket failed to parse crypto_prices payload",
			"error", err.Error(),
			"payload", truncate(string(message.Payload), 200),
		)
		return
	}

	value, err := decimal.NewFromString(payload.Value)
	if err != nil {
		client.logger.Warn("polymarket crypto_prices has unparseable value",
			"value", payload.Value,
			"symbol", payload.Symbol,
			"error", err.Error(),
		)
		return
	}

	event := domain.PolymarketCryptoPrice{
		Symbol:    payload.Symbol,
		Value:     value,
		Timestamp: time.UnixMilli(payload.Timestamp).UTC(),
	}

	select {
	case client.cryptoChannel <- event:
	default:
		client.logger.Warn("polymarket crypto price channel full, dropping event",
			"symbol", event.Symbol,
		)
	}
}

// topicNames extracts the topic name from each subscription for logging.
func topicNames(subscriptions []subscriptionTopic) []string {
	names := make([]string, len(subscriptions))
	for index, subscription := range subscriptions {
		names[index] = subscription.Topic
	}
	return names
}

// truncate returns the first n characters of s, appending "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
