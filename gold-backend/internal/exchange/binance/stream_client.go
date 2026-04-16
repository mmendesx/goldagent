package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

const (
	channelBufferSize   = 256
	maxReconnectDelay   = 60 * time.Second
	baseReconnectDelay  = 1 * time.Second
	readDeadlineTimeout = 90 * time.Second
	pongWriteTimeout    = 10 * time.Second
	streamSuffixKline   = "@kline_"
	streamSuffixTrade   = "@trade"
)

// StreamClientConfig configures the Binance WebSocket stream client.
type StreamClientConfig struct {
	BaseUrl  string   // e.g., "wss://stream.binance.com:9443"
	Symbols  []string // e.g., ["BTCUSDT", "ETHUSDT"]
	Interval string   // e.g., "5m"
}

// StreamClient maintains a persistent WebSocket connection to Binance and
// publishes parsed market data via channels.
type StreamClient struct {
	config        StreamClientConfig
	candleChannel chan domain.Candle
	tradeChannel  chan domain.TickerPrice
	logger        *slog.Logger
}

// NewStreamClient constructs a StreamClient with buffered output channels.
func NewStreamClient(config StreamClientConfig, logger *slog.Logger) *StreamClient {
	return &StreamClient{
		config:        config,
		candleChannel: make(chan domain.Candle, channelBufferSize),
		tradeChannel:  make(chan domain.TickerPrice, channelBufferSize),
		logger:        logger,
	}
}

// CandleChannel returns a read-only channel of parsed candle updates.
// Includes both intermediate (isClosed=false) and final (isClosed=true) candles.
func (client *StreamClient) CandleChannel() <-chan domain.Candle {
	return client.candleChannel
}

// TickerChannel returns a read-only channel of parsed trade-derived ticker prices.
func (client *StreamClient) TickerChannel() <-chan domain.TickerPrice {
	return client.tradeChannel
}

// Run connects to Binance, subscribes to all configured streams, and begins
// publishing data. Blocks until ctx is cancelled. Reconnects automatically
// with exponential backoff (1s, 2s, 4s, 8s, ..., max 60s).
func (client *StreamClient) Run(ctx context.Context) error {
	attempt := 0

	for {
		if ctx.Err() != nil {
			client.logger.Info("stream client stopping: context cancelled")
			return ctx.Err()
		}

		streamURL := buildStreamURL(client.config.BaseUrl, client.config.Symbols, client.config.Interval)

		client.logger.Info("connecting to binance stream",
			"url", streamURL,
			"attempt", attempt,
		)

		firstMessageReceived, err := client.runConnection(ctx, streamURL)
		if err != nil {
			if ctx.Err() != nil {
				client.logger.Info("stream client stopping after connection closed", "reason", err)
				return ctx.Err()
			}

			client.logger.Warn("binance stream connection lost",
				"error", err,
				"attempt", attempt,
			)
		}

		// Reset backoff only after first message received on the connection.
		// A connection that handshakes but never delivers data does not count as stable.
		if firstMessageReceived {
			attempt = 0
		} else {
			attempt++
		}

		delay := reconnectDelay(attempt)

		client.logger.Info("reconnecting to binance stream",
			"attempt", attempt,
			"delay", delay.String(),
		)

		select {
		case <-ctx.Done():
			client.logger.Info("stream client stopping: context cancelled during backoff")
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

// runConnection opens a single WebSocket connection to streamURL, reads messages
// until the connection fails or ctx is cancelled, and reports whether at least
// one message was successfully parsed. This function owns the lifecycle of conn.
func (client *StreamClient) runConnection(ctx context.Context, streamURL string) (firstMessageReceived bool, err error) {
	dialer := websocket.DefaultDialer
	conn, _, dialErr := dialer.DialContext(ctx, streamURL, nil)
	if dialErr != nil {
		return false, fmt.Errorf("dial failed: %w", dialErr)
	}
	defer func() {
		closeErr := conn.Close()
		if closeErr != nil && err == nil {
			err = fmt.Errorf("connection close: %w", closeErr)
		}
	}()

	client.logger.Info("binance stream connection established", "url", streamURL)

	// Respond to Binance ping frames with a pong. This runs on the read goroutine
	// so there is no concurrent write risk here (no other goroutine writes to conn).
	conn.SetPingHandler(func(data string) error {
		writeErr := conn.WriteControl(
			websocket.PongMessage,
			[]byte(data),
			time.Now().Add(pongWriteTimeout),
		)
		if writeErr != nil {
			client.logger.Warn("failed to send pong", "error", writeErr)
		}
		return nil
	})

	// Extend the read deadline on every successful read so the connection stays
	// alive through brief quiet periods.
	if deadlineErr := conn.SetReadDeadline(time.Now().Add(readDeadlineTimeout)); deadlineErr != nil {
		return false, fmt.Errorf("set initial read deadline: %w", deadlineErr)
	}

	// Drain messages until context is cancelled or the connection fails.
	for {
		if ctx.Err() != nil {
			return firstMessageReceived, ctx.Err()
		}

		_, rawMessage, readErr := conn.ReadMessage()
		if readErr != nil {
			return firstMessageReceived, fmt.Errorf("read message: %w", readErr)
		}

		if deadlineErr := conn.SetReadDeadline(time.Now().Add(readDeadlineTimeout)); deadlineErr != nil {
			client.logger.Warn("failed to reset read deadline", "error", deadlineErr)
		}

		if parseErr := client.dispatchMessage(rawMessage); parseErr != nil {
			client.logger.Warn("failed to dispatch message",
				"error", parseErr,
				"raw", string(rawMessage),
			)
			continue
		}

		// Mark stable after the first successfully parsed message.
		firstMessageReceived = true
	}
}

// dispatchMessage parses the outer combined-stream envelope, determines the stream
// type from the stream name suffix, and routes to the appropriate parser.
func (client *StreamClient) dispatchMessage(rawMessage []byte) error {
	var envelope combinedStreamMessage
	if err := json.Unmarshal(rawMessage, &envelope); err != nil {
		return fmt.Errorf("unmarshal combined stream envelope: %w", err)
	}

	switch {
	case strings.Contains(envelope.Stream, streamSuffixKline):
		return client.handleKlineMessage(envelope.Data)

	case strings.HasSuffix(envelope.Stream, streamSuffixTrade):
		return client.handleTradeMessage(envelope.Data)

	default:
		client.logger.Warn("received unrecognised stream type", "stream", envelope.Stream)
		return nil
	}
}

// handleKlineMessage parses a kline data payload and sends it to the candle channel.
func (client *StreamClient) handleKlineMessage(data json.RawMessage) error {
	candle, err := parseKlineMessage(data)
	if err != nil {
		return err
	}

	select {
	case client.candleChannel <- candle:
	default:
		client.logger.Warn("candle channel full, dropping candle",
			"symbol", candle.Symbol,
			"interval", candle.Interval,
		)
	}

	return nil
}

// handleTradeMessage parses a trade data payload and sends it to the trade channel.
func (client *StreamClient) handleTradeMessage(data json.RawMessage) error {
	ticker, err := parseTradeMessage(data)
	if err != nil {
		return err
	}

	select {
	case client.tradeChannel <- ticker:
	default:
		client.logger.Warn("trade channel full, dropping ticker",
			"symbol", ticker.Symbol,
		)
	}

	return nil
}

// buildStreamURL constructs the combined-stream WebSocket URL from the base URL,
// symbol list, and interval. Symbols are lowercased as required by the Binance API.
// For symbols ["BTCUSDT", "ETHUSDT"] and interval "5m" the result is:
// wss://stream.binance.com:9443/stream?streams=btcusdt@kline_5m/btcusdt@trade/ethusdt@kline_5m/ethusdt@trade
func buildStreamURL(baseURL string, symbols []string, interval string) string {
	streams := make([]string, 0, len(symbols)*2)

	for _, symbol := range symbols {
		lower := strings.ToLower(symbol)
		streams = append(streams, fmt.Sprintf("%s@kline_%s", lower, interval))
		streams = append(streams, fmt.Sprintf("%s@trade", lower))
	}

	return fmt.Sprintf("%s/stream?streams=%s", baseURL, strings.Join(streams, "/"))
}

// parseKlineMessage parses the inner data payload of a kline event into a domain.Candle.
// Returns an error if any required decimal field fails to parse.
func parseKlineMessage(data json.RawMessage) (domain.Candle, error) {
	var msg klineMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return domain.Candle{}, fmt.Errorf("unmarshal kline message: %w", err)
	}

	k := msg.Kline

	openPrice, err := decimal.NewFromString(k.Open)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("parse open price %q: %w", k.Open, err)
	}

	closePrice, err := decimal.NewFromString(k.Close)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("parse close price %q: %w", k.Close, err)
	}

	highPrice, err := decimal.NewFromString(k.High)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("parse high price %q: %w", k.High, err)
	}

	lowPrice, err := decimal.NewFromString(k.Low)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("parse low price %q: %w", k.Low, err)
	}

	volume, err := decimal.NewFromString(k.Volume)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("parse volume %q: %w", k.Volume, err)
	}

	quoteVolume, err := decimal.NewFromString(k.QuoteVolume)
	if err != nil {
		return domain.Candle{}, fmt.Errorf("parse quote volume %q: %w", k.QuoteVolume, err)
	}

	return domain.Candle{
		Symbol:      k.Symbol,
		Interval:    k.Interval,
		OpenTime:    time.UnixMilli(k.OpenTime),
		CloseTime:   time.UnixMilli(k.CloseTime),
		OpenPrice:   openPrice,
		ClosePrice:  closePrice,
		HighPrice:   highPrice,
		LowPrice:    lowPrice,
		Volume:      volume,
		QuoteVolume: quoteVolume,
		TradeCount:  k.TradeCount,
		IsClosed:    k.IsClosed,
	}, nil
}

// parseTradeMessage parses the inner data payload of a trade event into a domain.TickerPrice.
// Returns an error if the price field fails to parse.
func parseTradeMessage(data json.RawMessage) (domain.TickerPrice, error) {
	var msg tradeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return domain.TickerPrice{}, fmt.Errorf("unmarshal trade message: %w", err)
	}

	price, err := decimal.NewFromString(msg.Price)
	if err != nil {
		return domain.TickerPrice{}, fmt.Errorf("parse trade price %q: %w", msg.Price, err)
	}

	return domain.TickerPrice{
		Symbol:    msg.Symbol,
		Price:     price,
		Timestamp: time.UnixMilli(msg.Timestamp),
	}, nil
}

// reconnectDelay computes the backoff duration for the given attempt number.
// Delay = min(1s * 2^attempt, 60s). attempt=0 yields 1s, attempt=5 yields 32s, attempt=6+ yields 60s.
func reconnectDelay(attempt int) time.Duration {
	delay := baseReconnectDelay
	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay >= maxReconnectDelay {
			return maxReconnectDelay
		}
	}

	return delay
}
