package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

// CandleRepository defines persistence operations for OHLCV candle data.
type CandleRepository interface {
	InsertCandle(ctx context.Context, candle domain.Candle) (int64, error)
	InsertCandlesBatch(ctx context.Context, candles []domain.Candle) error
	UpsertCandle(ctx context.Context, candle domain.Candle) (int64, error)
	FindCandlesByRange(ctx context.Context, symbol, interval string, from, to time.Time, limit int) ([]domain.Candle, error)
	FindLatestCandles(ctx context.Context, symbol, interval string, limit int) ([]domain.Candle, error)
}

type candleRepository struct {
	pool *pgxpool.Pool
}

// NewCandleRepository returns a CandleRepository backed by the given connection pool.
func NewCandleRepository(pool *pgxpool.Pool) CandleRepository {
	return &candleRepository{pool: pool}
}

// InsertCandle inserts a single candle and returns its generated ID.
func (r *candleRepository) InsertCandle(ctx context.Context, candle domain.Candle) (int64, error) {
	const query = `
		INSERT INTO candles
			(symbol, interval, open_time, close_time, open_price, high_price, low_price,
			 close_price, volume, quote_volume, trade_count, is_closed)
		VALUES
			($1, $2, $3, $4, $5::numeric, $6::numeric, $7::numeric,
			 $8::numeric, $9::numeric, $10::numeric, $11, $12)
		RETURNING id`

	var id int64
	err := r.pool.QueryRow(ctx, query,
		candle.Symbol, candle.Interval, candle.OpenTime, candle.CloseTime,
		decimalToString(candle.OpenPrice), decimalToString(candle.HighPrice),
		decimalToString(candle.LowPrice), decimalToString(candle.ClosePrice),
		decimalToString(candle.Volume), decimalToString(candle.QuoteVolume),
		candle.TradeCount, candle.IsClosed,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert candle for symbol %q interval %q open_time %v: %w",
			candle.Symbol, candle.Interval, candle.OpenTime, err)
	}
	return id, nil
}

// InsertCandlesBatch bulk-inserts candles using pgx CopyFrom for maximum throughput.
// The partitioned table routes each row to the correct partition automatically.
func (r *candleRepository) InsertCandlesBatch(ctx context.Context, candles []domain.Candle) error {
	if len(candles) == 0 {
		return nil
	}

	columns := []string{
		"symbol", "interval", "open_time", "close_time",
		"open_price", "high_price", "low_price", "close_price",
		"volume", "quote_volume", "trade_count", "is_closed",
	}

	rows := make([][]interface{}, 0, len(candles))
	for _, c := range candles {
		rows = append(rows, []interface{}{
			c.Symbol, c.Interval, c.OpenTime, c.CloseTime,
			decimalToNumeric(c.OpenPrice), decimalToNumeric(c.HighPrice),
			decimalToNumeric(c.LowPrice), decimalToNumeric(c.ClosePrice),
			decimalToNumeric(c.Volume), decimalToNumeric(c.QuoteVolume),
			c.TradeCount, c.IsClosed,
		})
	}

	_, err := r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"candles"},
		columns,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return fmt.Errorf("batch insert %d candles: %w", len(candles), err)
	}
	return nil
}

// UpsertCandle inserts a candle or updates it on conflict with the unique
// (symbol, interval, open_time) index, returning the row's ID.
func (r *candleRepository) UpsertCandle(ctx context.Context, candle domain.Candle) (int64, error) {
	const query = `
		INSERT INTO candles
			(symbol, interval, open_time, close_time, open_price, high_price, low_price,
			 close_price, volume, quote_volume, trade_count, is_closed)
		VALUES
			($1, $2, $3, $4, $5::numeric, $6::numeric, $7::numeric,
			 $8::numeric, $9::numeric, $10::numeric, $11, $12)
		ON CONFLICT (symbol, interval, open_time) DO UPDATE SET
			close_time   = EXCLUDED.close_time,
			open_price   = EXCLUDED.open_price,
			high_price   = EXCLUDED.high_price,
			low_price    = EXCLUDED.low_price,
			close_price  = EXCLUDED.close_price,
			volume       = EXCLUDED.volume,
			quote_volume = EXCLUDED.quote_volume,
			trade_count  = EXCLUDED.trade_count,
			is_closed    = EXCLUDED.is_closed
		RETURNING id`

	var id int64
	err := r.pool.QueryRow(ctx, query,
		candle.Symbol, candle.Interval, candle.OpenTime, candle.CloseTime,
		decimalToString(candle.OpenPrice), decimalToString(candle.HighPrice),
		decimalToString(candle.LowPrice), decimalToString(candle.ClosePrice),
		decimalToString(candle.Volume), decimalToString(candle.QuoteVolume),
		candle.TradeCount, candle.IsClosed,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("upsert candle for symbol %q interval %q open_time %v: %w",
			candle.Symbol, candle.Interval, candle.OpenTime, err)
	}
	return id, nil
}

// FindCandlesByRange returns candles for a symbol+interval within [from, to],
// ordered by open_time ascending. Results are capped by limit.
func (r *candleRepository) FindCandlesByRange(
	ctx context.Context,
	symbol, interval string,
	from, to time.Time,
	limit int,
) ([]domain.Candle, error) {
	const query = `
		SELECT id, symbol, interval, open_time, close_time,
		       open_price::text, high_price::text, low_price::text, close_price::text,
		       volume::text, quote_volume::text, trade_count, is_closed
		FROM candles
		WHERE symbol = $1 AND interval = $2 AND open_time >= $3 AND open_time <= $4
		ORDER BY open_time ASC
		LIMIT $5`

	rows, err := r.pool.Query(ctx, query, symbol, interval, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("query candles for symbol %q interval %q range [%v, %v]: %w",
			symbol, interval, from, to, err)
	}
	defer rows.Close()

	return scanCandleRows(rows)
}

// FindLatestCandles returns the most recent candles for a symbol+interval,
// ordered by open_time descending.
func (r *candleRepository) FindLatestCandles(ctx context.Context, symbol, interval string, limit int) ([]domain.Candle, error) {
	const query = `
		SELECT id, symbol, interval, open_time, close_time,
		       open_price::text, high_price::text, low_price::text, close_price::text,
		       volume::text, quote_volume::text, trade_count, is_closed
		FROM candles
		WHERE symbol = $1 AND interval = $2
		ORDER BY open_time DESC
		LIMIT $3`

	rows, err := r.pool.Query(ctx, query, symbol, interval, limit)
	if err != nil {
		return nil, fmt.Errorf("query latest candles for symbol %q interval %q limit %d: %w",
			symbol, interval, limit, err)
	}
	defer rows.Close()

	return scanCandleRows(rows)
}

func scanCandleRows(rows pgx.Rows) ([]domain.Candle, error) {
	var candles []domain.Candle
	for rows.Next() {
		var c domain.Candle
		var openPrice, highPrice, lowPrice, closePrice, volume, quoteVolume string
		if err := rows.Scan(
			&c.ID, &c.Symbol, &c.Interval, &c.OpenTime, &c.CloseTime,
			&openPrice, &highPrice, &lowPrice, &closePrice,
			&volume, &quoteVolume, &c.TradeCount, &c.IsClosed,
		); err != nil {
			return nil, fmt.Errorf("scan candle row: %w", err)
		}

		var err error
		if c.OpenPrice, err = decimal.NewFromString(openPrice); err != nil {
			return nil, fmt.Errorf("parse open_price %q: %w", openPrice, err)
		}
		if c.HighPrice, err = decimal.NewFromString(highPrice); err != nil {
			return nil, fmt.Errorf("parse high_price %q: %w", highPrice, err)
		}
		if c.LowPrice, err = decimal.NewFromString(lowPrice); err != nil {
			return nil, fmt.Errorf("parse low_price %q: %w", lowPrice, err)
		}
		if c.ClosePrice, err = decimal.NewFromString(closePrice); err != nil {
			return nil, fmt.Errorf("parse close_price %q: %w", closePrice, err)
		}
		if c.Volume, err = decimal.NewFromString(volume); err != nil {
			return nil, fmt.Errorf("parse volume %q: %w", volume, err)
		}
		if c.QuoteVolume, err = decimal.NewFromString(quoteVolume); err != nil {
			return nil, fmt.Errorf("parse quote_volume %q: %w", quoteVolume, err)
		}

		candles = append(candles, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate candle rows: %w", err)
	}
	return candles, nil
}
