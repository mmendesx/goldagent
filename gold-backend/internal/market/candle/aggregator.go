package candle

import (
	"context"
	"log/slog"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
)

const closedCandleChannelBuffer = 256

// CandleCache is the subset of Redis cache operations required by the Aggregator.
// The real *redisstore.CacheClient satisfies this interface implicitly.
type CandleCache interface {
	SetLatestCandle(ctx context.Context, candle domain.Candle) error
}

// AggregatorConfig holds dependencies and tuning parameters.
type AggregatorConfig struct {
	InputChannel       <-chan domain.Candle
	PostgresRepository postgres.CandleRepository
	RedisCache         CandleCache
	Logger             *slog.Logger
}

// Aggregator processes raw candle updates: caches latest in Redis,
// persists closed candles to Postgres, and emits candle-close events
// for downstream analysis.
type Aggregator struct {
	config              AggregatorConfig
	closedCandleChannel chan domain.Candle
}

// NewAggregator constructs an Aggregator with a buffered closed-candle channel.
func NewAggregator(config AggregatorConfig) *Aggregator {
	return &Aggregator{
		config:              config,
		closedCandleChannel: make(chan domain.Candle, closedCandleChannelBuffer),
	}
}

// ClosedCandleChannel returns a read-only channel that emits each candle
// at the moment it closes. Downstream services (indicators, decision
// engine) subscribe to this.
func (aggregator *Aggregator) ClosedCandleChannel() <-chan domain.Candle {
	return aggregator.closedCandleChannel
}

// Run drains the input channel, processing each candle update:
//  1. Update Redis hot cache (always, including non-closed updates)
//  2. If IsClosed: upsert to Postgres and emit on closedCandleChannel
//  3. Continue concurrently across all symbols (one goroutine total
//     reads the channel; per-candle work is small and sequential)
//
// Blocks until ctx is cancelled or input channel closes.
func (aggregator *Aggregator) Run(ctx context.Context) error {
	// Close closedCandleChannel exactly once when Run exits, covering
	// both the ctx-cancelled and input-channel-closed exit paths.
	defer close(aggregator.closedCandleChannel)

	for {
		select {
		case candle, ok := <-aggregator.config.InputChannel:
			if !ok {
				aggregator.config.Logger.Info("candle aggregator: input channel closed, shutting down")
				return nil
			}
			aggregator.processCandle(ctx, candle)

		case <-ctx.Done():
			aggregator.config.Logger.Info("candle aggregator: context cancelled, shutting down",
				"reason", ctx.Err(),
			)
			return ctx.Err()
		}
	}
}

// processCandle handles a single candle update:
//   - always writes to the Redis hot cache (best-effort)
//   - if the candle is closed: upserts to Postgres and emits on closedCandleChannel
func (aggregator *Aggregator) processCandle(ctx context.Context, candle domain.Candle) {
	if err := aggregator.config.RedisCache.SetLatestCandle(ctx, candle); err != nil {
		aggregator.config.Logger.Warn("candle aggregator: failed to update redis cache",
			"symbol", candle.Symbol,
			"interval", candle.Interval,
			"openTime", candle.OpenTime,
			"error", err,
		)
		// Cache is best-effort — continue processing.
	}

	if !candle.IsClosed {
		return
	}

	id, err := aggregator.config.PostgresRepository.UpsertCandle(ctx, candle)
	if err != nil {
		aggregator.config.Logger.Error("candle aggregator: failed to persist closed candle",
			"symbol", candle.Symbol,
			"interval", candle.Interval,
			"openTime", candle.OpenTime,
			"error", err,
		)
		// Missing one candle is recoverable — do not crash the pipeline.
		return
	}

	aggregator.config.Logger.Info("candle aggregator: closed candle persisted",
		"symbol", candle.Symbol,
		"interval", candle.Interval,
		"openTime", candle.OpenTime,
		"id", id,
	)

	// Emit non-blocking so a slow downstream consumer cannot stall the aggregator.
	select {
	case aggregator.closedCandleChannel <- candle:
	default:
		aggregator.config.Logger.Warn("candle aggregator: closed candle channel full, dropping event",
			"symbol", candle.Symbol,
			"interval", candle.Interval,
			"openTime", candle.OpenTime,
		)
	}
}
