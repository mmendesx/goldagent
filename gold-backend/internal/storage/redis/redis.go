package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

const (
	candleTTL = 5 * time.Minute
	tickerTTL = 30 * time.Second
)

// CacheClient wraps Redis operations for the Gold trading agent.
type CacheClient struct {
	client *goredis.Client
}

// NewCacheClient creates a new Redis cache client from a Redis URL string.
func NewCacheClient(redisURL string) (*CacheClient, error) {
	options, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parsing redis URL: %w", err)
	}

	client := goredis.NewClient(options)
	return &CacheClient{client: client}, nil
}

// Close closes the Redis connection.
func (c *CacheClient) Close() error {
	return c.client.Close()
}

// Ping verifies the Redis connection is alive.
func (c *CacheClient) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}
	return nil
}

// SetLatestCandle stores the latest candle for a symbol+interval pair.
// Key format: "candle:{symbol}:{interval}:latest"
// TTL: 5 minutes
func (c *CacheClient) SetLatestCandle(ctx context.Context, candle domain.Candle) error {
	key := fmt.Sprintf("candle:%s:%s:latest", candle.Symbol, candle.Interval)
	data, err := json.Marshal(candle)
	if err != nil {
		return fmt.Errorf("serializing candle for key %q: %w", key, err)
	}

	if err := c.client.Set(ctx, key, data, candleTTL).Err(); err != nil {
		return fmt.Errorf("writing candle to cache key %q: %w", key, err)
	}
	return nil
}

// GetLatestCandle retrieves the latest cached candle for a symbol+interval.
// Returns nil (not an error) on cache miss.
func (c *CacheClient) GetLatestCandle(ctx context.Context, symbol string, interval string) (*domain.Candle, error) {
	key := fmt.Sprintf("candle:%s:%s:latest", symbol, interval)
	data, err := c.client.Get(ctx, key).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading candle from cache key %q: %w", key, err)
	}

	var candle domain.Candle
	if err := json.Unmarshal(data, &candle); err != nil {
		return nil, fmt.Errorf("deserializing candle from cache key %q: %w", key, err)
	}
	return &candle, nil
}

// SetTickerPrice stores the latest ticker price for a symbol.
// Key format: "ticker:{symbol}:latest"
// TTL: 30 seconds
func (c *CacheClient) SetTickerPrice(ctx context.Context, ticker domain.TickerPrice) error {
	key := fmt.Sprintf("ticker:%s:latest", ticker.Symbol)
	data, err := json.Marshal(ticker)
	if err != nil {
		return fmt.Errorf("serializing ticker for key %q: %w", key, err)
	}

	if err := c.client.Set(ctx, key, data, tickerTTL).Err(); err != nil {
		return fmt.Errorf("writing ticker to cache key %q: %w", key, err)
	}
	return nil
}

// GetTickerPrice retrieves the latest ticker price for a symbol.
// Returns nil (not an error) on cache miss.
func (c *CacheClient) GetTickerPrice(ctx context.Context, symbol string) (*domain.TickerPrice, error) {
	key := fmt.Sprintf("ticker:%s:latest", symbol)
	data, err := c.client.Get(ctx, key).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading ticker from cache key %q: %w", key, err)
	}

	var ticker domain.TickerPrice
	if err := json.Unmarshal(data, &ticker); err != nil {
		return nil, fmt.Errorf("deserializing ticker from cache key %q: %w", key, err)
	}
	return &ticker, nil
}

// SetOpenPositions replaces the full set of open positions.
// Key: "positions:open"
// No TTL — managed explicitly.
func (c *CacheClient) SetOpenPositions(ctx context.Context, positions []domain.Position) error {
	const key = "positions:open"
	data, err := json.Marshal(positions)
	if err != nil {
		return fmt.Errorf("serializing open positions for key %q: %w", key, err)
	}

	if err := c.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("writing open positions to cache key %q: %w", key, err)
	}
	return nil
}

// GetOpenPositions retrieves all cached open positions.
// Returns nil (not an error) on cache miss.
func (c *CacheClient) GetOpenPositions(ctx context.Context) ([]domain.Position, error) {
	const key = "positions:open"
	data, err := c.client.Get(ctx, key).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading open positions from cache key %q: %w", key, err)
	}

	var positions []domain.Position
	if err := json.Unmarshal(data, &positions); err != nil {
		return nil, fmt.Errorf("deserializing open positions from cache key %q: %w", key, err)
	}
	return positions, nil
}

// SetPortfolioMetrics stores the current portfolio metrics.
// Key: "metrics:portfolio"
// No TTL — managed explicitly.
func (c *CacheClient) SetPortfolioMetrics(ctx context.Context, metrics domain.PortfolioMetrics) error {
	const key = "metrics:portfolio"
	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("serializing portfolio metrics for key %q: %w", key, err)
	}

	if err := c.client.Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("writing portfolio metrics to cache key %q: %w", key, err)
	}
	return nil
}

// GetPortfolioMetrics retrieves the cached portfolio metrics.
// Returns nil (not an error) on cache miss.
func (c *CacheClient) GetPortfolioMetrics(ctx context.Context) (*domain.PortfolioMetrics, error) {
	const key = "metrics:portfolio"
	data, err := c.client.Get(ctx, key).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading portfolio metrics from cache key %q: %w", key, err)
	}

	var metrics domain.PortfolioMetrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, fmt.Errorf("deserializing portfolio metrics from cache key %q: %w", key, err)
	}
	return &metrics, nil
}
