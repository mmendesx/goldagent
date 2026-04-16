package candle

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/shopspring/decimal"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

// fakeCandleRepository is an in-memory CandleRepository for tests.
type fakeCandleRepository struct {
	upsertedCandles []domain.Candle
	mu              sync.Mutex
}

func (f *fakeCandleRepository) UpsertCandle(_ context.Context, c domain.Candle) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upsertedCandles = append(f.upsertedCandles, c)
	return int64(len(f.upsertedCandles)), nil
}

func (f *fakeCandleRepository) InsertCandle(_ context.Context, _ domain.Candle) (int64, error) {
	return 0, nil
}

func (f *fakeCandleRepository) InsertCandlesBatch(_ context.Context, _ []domain.Candle) error {
	return nil
}

func (f *fakeCandleRepository) FindCandlesByRange(
	_ context.Context, _, _ string, _, _ time.Time, _ int,
) ([]domain.Candle, error) {
	return nil, nil
}

func (f *fakeCandleRepository) FindLatestCandles(
	_ context.Context, _, _ string, _ int,
) ([]domain.Candle, error) {
	return nil, nil
}

func (f *fakeCandleRepository) upserted() []domain.Candle {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Candle, len(f.upsertedCandles))
	copy(result, f.upsertedCandles)
	return result
}

// fakeCandleCache is an in-memory CandleCache for tests.
type fakeCandleCache struct {
	candles []domain.Candle
	mu      sync.Mutex
}

func (f *fakeCandleCache) SetLatestCandle(_ context.Context, c domain.Candle) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.candles = append(f.candles, c)
	return nil
}

func (f *fakeCandleCache) latest() []domain.Candle {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]domain.Candle, len(f.candles))
	copy(result, f.candles)
	return result
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func newCandle(symbol string, isClosed bool) domain.Candle {
	return domain.Candle{
		Symbol:     symbol,
		Interval:   "1m",
		OpenTime:   time.Now().Truncate(time.Minute),
		CloseTime:  time.Now().Truncate(time.Minute).Add(time.Minute),
		OpenPrice:  decimal.NewFromFloat(1000),
		HighPrice:  decimal.NewFromFloat(1100),
		LowPrice:   decimal.NewFromFloat(990),
		ClosePrice: decimal.NewFromFloat(1050),
		Volume:     decimal.NewFromFloat(50),
		IsClosed:   isClosed,
	}
}

// buildAggregator returns an Aggregator wired to fakes, plus the input channel.
func buildAggregator() (*Aggregator, chan domain.Candle, *fakeCandleRepository, *fakeCandleCache) {
	input := make(chan domain.Candle, 16)
	repo := &fakeCandleRepository{}
	cache := &fakeCandleCache{}

	agg := NewAggregator(AggregatorConfig{
		InputChannel:       input,
		PostgresRepository: repo,
		RedisCache:         cache,
		Logger:             testLogger(),
	})
	return agg, input, repo, cache
}

// runAggregator starts Run in a goroutine and returns the cancel func plus
// a channel that signals when Run returns.
func runAggregator(agg *Aggregator) (cancel context.CancelFunc, done <-chan error) {
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- agg.Run(ctx)
	}()
	return cancel, errCh
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// Non-closed candles should update cache but not Postgres.
func TestNonClosedCandleUpdatesCacheOnly(t *testing.T) {
	agg, input, repo, cache := buildAggregator()
	cancel, done := runAggregator(agg)
	defer cancel()

	candle := newCandle("BTCUSDT", false)
	input <- candle

	// Give the aggregator time to process.
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	if len(cache.latest()) != 1 {
		t.Fatalf("expected 1 cache write, got %d", len(cache.latest()))
	}
	if len(repo.upserted()) != 0 {
		t.Fatalf("expected 0 postgres writes for non-closed candle, got %d", len(repo.upserted()))
	}
}

// Closed candles should update both cache and Postgres.
func TestClosedCandleUpdatesCacheAndPostgres(t *testing.T) {
	agg, input, repo, cache := buildAggregator()
	cancel, done := runAggregator(agg)
	defer cancel()

	candle := newCandle("BTCUSDT", true)
	input <- candle

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	if len(cache.latest()) != 1 {
		t.Fatalf("expected 1 cache write, got %d", len(cache.latest()))
	}
	if len(repo.upserted()) != 1 {
		t.Fatalf("expected 1 postgres write for closed candle, got %d", len(repo.upserted()))
	}
	if repo.upserted()[0].Symbol != candle.Symbol {
		t.Fatalf("upserted wrong candle: want symbol %q, got %q", candle.Symbol, repo.upserted()[0].Symbol)
	}
}

// closedCandleChannel must emit only closed candles.
func TestClosedCandleChannelEmitsOnlyClosedCandles(t *testing.T) {
	agg, input, _, _ := buildAggregator()
	cancel, done := runAggregator(agg)
	defer cancel()

	// Send a mix: two open, one closed.
	input <- newCandle("BTCUSDT", false)
	input <- newCandle("ETHUSDT", false)
	closedCandle := newCandle("BTCUSDT", true)
	input <- closedCandle

	// Collect what arrives on the closed channel within a timeout.
	var received []domain.Candle
	deadline := time.After(200 * time.Millisecond)
collecting:
	for {
		select {
		case c, ok := <-agg.ClosedCandleChannel():
			if !ok {
				break collecting
			}
			received = append(received, c)
			if len(received) >= 1 {
				// Got the one closed candle we expect; stop collecting.
				break collecting
			}
		case <-deadline:
			break collecting
		}
	}

	cancel()
	<-done

	if len(received) != 1 {
		t.Fatalf("expected 1 closed candle on channel, got %d", len(received))
	}
	if received[0].Symbol != closedCandle.Symbol {
		t.Fatalf("wrong candle emitted: want %q, got %q", closedCandle.Symbol, received[0].Symbol)
	}
	if !received[0].IsClosed {
		t.Fatal("emitted candle is not closed")
	}
}

// A drained closed channel must not block the aggregator from continuing.
func TestDrainedClosedChannelDoesNotBlockAggregator(t *testing.T) {
	// Fill the buffer beyond capacity by sending many closed candles without reading.
	agg, input, repo, _ := buildAggregator()
	cancel, done := runAggregator(agg)
	defer cancel()

	const total = closedCandleChannelBuffer + 10

	for i := 0; i < total; i++ {
		input <- newCandle("BTCUSDT", true)
	}

	// The aggregator must not stall even though the closed channel is never read.
	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	persisted := repo.upserted()
	// At least the buffer size worth should have been persisted regardless of drops.
	if len(persisted) == 0 {
		t.Fatal("expected postgres writes, got none")
	}
}

// Closing the input channel must cause Run to return without error.
func TestInputChannelCloseReturnsGracefully(t *testing.T) {
	agg, input, _, _ := buildAggregator()

	ctx := context.Background()
	errCh := make(chan error, 1)
	go func() {
		errCh <- agg.Run(ctx)
	}()

	// Close the input channel to signal end-of-stream.
	close(input)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected nil error on input close, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after input channel was closed")
	}
}

// Context cancellation must cause Run to return ctx.Err() and close closedCandleChannel.
func TestContextCancelReturnsCtxErr(t *testing.T) {
	agg, _, _, _ := buildAggregator()
	cancel, done := runAggregator(agg)

	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected non-nil error on context cancel, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context was cancelled")
	}

	// closedCandleChannel must be closed so a range loop can drain it.
	_, ok := <-agg.ClosedCandleChannel()
	if ok {
		t.Fatal("expected closedCandleChannel to be closed after Run returns")
	}
}
