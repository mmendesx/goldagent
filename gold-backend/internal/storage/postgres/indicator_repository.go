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

// IndicatorRepository defines persistence operations for computed technical indicators.
type IndicatorRepository interface {
	InsertIndicator(ctx context.Context, indicator domain.Indicator) (int64, error)
	FindLatestIndicator(ctx context.Context, symbol, interval string) (*domain.Indicator, error)
	FindIndicatorsByRange(ctx context.Context, symbol, interval string, from, to time.Time) ([]domain.Indicator, error)
}

type indicatorRepository struct {
	pool *pgxpool.Pool
}

// NewIndicatorRepository returns an IndicatorRepository backed by the given connection pool.
func NewIndicatorRepository(pool *pgxpool.Pool) IndicatorRepository {
	return &indicatorRepository{pool: pool}
}

// InsertIndicator persists a full indicator record and returns its generated ID.
func (r *indicatorRepository) InsertIndicator(ctx context.Context, ind domain.Indicator) (int64, error) {
	const query = `
		INSERT INTO indicators
			(candle_id, symbol, interval, timestamp,
			 rsi, macd_line, macd_signal, macd_histogram,
			 bollinger_upper, bollinger_middle, bollinger_lower,
			 ema_9, ema_21, ema_50, ema_200, vwap, atr)
		VALUES
			($1, $2, $3, $4,
			 $5::numeric, $6::numeric, $7::numeric, $8::numeric,
			 $9::numeric, $10::numeric, $11::numeric,
			 $12::numeric, $13::numeric, $14::numeric, $15::numeric,
			 $16::numeric, $17::numeric)
		RETURNING id`

	var id int64
	err := r.pool.QueryRow(ctx, query,
		ind.CandleID, ind.Symbol, ind.Interval, ind.Timestamp,
		decimalToString(ind.Rsi), decimalToString(ind.MacdLine),
		decimalToString(ind.MacdSignal), decimalToString(ind.MacdHistogram),
		decimalToString(ind.BollingerUpper), decimalToString(ind.BollingerMiddle),
		decimalToString(ind.BollingerLower),
		decimalToString(ind.Ema9), decimalToString(ind.Ema21),
		decimalToString(ind.Ema50), decimalToString(ind.Ema200),
		decimalToString(ind.Vwap), decimalToString(ind.Atr),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert indicator for symbol %q interval %q candle_id %d: %w",
			ind.Symbol, ind.Interval, ind.CandleID, err)
	}
	return id, nil
}

// FindLatestIndicator returns the most recent indicator record for a symbol+interval,
// or nil if none exists.
func (r *indicatorRepository) FindLatestIndicator(ctx context.Context, symbol, interval string) (*domain.Indicator, error) {
	const query = `
		SELECT id, candle_id, symbol, interval, timestamp,
		       COALESCE(rsi::text, '0'), COALESCE(macd_line::text, '0'),
		       COALESCE(macd_signal::text, '0'), COALESCE(macd_histogram::text, '0'),
		       COALESCE(bollinger_upper::text, '0'), COALESCE(bollinger_middle::text, '0'),
		       COALESCE(bollinger_lower::text, '0'),
		       COALESCE(ema_9::text, '0'), COALESCE(ema_21::text, '0'),
		       COALESCE(ema_50::text, '0'), COALESCE(ema_200::text, '0'),
		       COALESCE(vwap::text, '0'), COALESCE(atr::text, '0')
		FROM indicators
		WHERE symbol = $1 AND interval = $2
		ORDER BY timestamp DESC
		LIMIT 1`

	rows, err := r.pool.Query(ctx, query, symbol, interval)
	if err != nil {
		return nil, fmt.Errorf("query latest indicator for symbol %q interval %q: %w", symbol, interval, err)
	}
	defer rows.Close()

	indicators, err := scanIndicatorRows(rows)
	if err != nil {
		return nil, err
	}
	if len(indicators) == 0 {
		return nil, nil
	}
	return &indicators[0], nil
}

// FindIndicatorsByRange returns indicators for a symbol+interval within [from, to],
// ordered by timestamp ascending.
func (r *indicatorRepository) FindIndicatorsByRange(
	ctx context.Context,
	symbol, interval string,
	from, to time.Time,
) ([]domain.Indicator, error) {
	const query = `
		SELECT id, candle_id, symbol, interval, timestamp,
		       COALESCE(rsi::text, '0'), COALESCE(macd_line::text, '0'),
		       COALESCE(macd_signal::text, '0'), COALESCE(macd_histogram::text, '0'),
		       COALESCE(bollinger_upper::text, '0'), COALESCE(bollinger_middle::text, '0'),
		       COALESCE(bollinger_lower::text, '0'),
		       COALESCE(ema_9::text, '0'), COALESCE(ema_21::text, '0'),
		       COALESCE(ema_50::text, '0'), COALESCE(ema_200::text, '0'),
		       COALESCE(vwap::text, '0'), COALESCE(atr::text, '0')
		FROM indicators
		WHERE symbol = $1 AND interval = $2 AND timestamp >= $3 AND timestamp <= $4
		ORDER BY timestamp ASC`

	rows, err := r.pool.Query(ctx, query, symbol, interval, from, to)
	if err != nil {
		return nil, fmt.Errorf("query indicators for symbol %q interval %q range [%v, %v]: %w",
			symbol, interval, from, to, err)
	}
	defer rows.Close()

	return scanIndicatorRows(rows)
}

func scanIndicatorRows(rows pgx.Rows) ([]domain.Indicator, error) {
	var indicators []domain.Indicator
	for rows.Next() {
		var ind domain.Indicator
		var rsi, macdLine, macdSignal, macdHistogram string
		var bollingerUpper, bollingerMiddle, bollingerLower string
		var ema9, ema21, ema50, ema200, vwap, atr string

		if err := rows.Scan(
			&ind.ID, &ind.CandleID, &ind.Symbol, &ind.Interval, &ind.Timestamp,
			&rsi, &macdLine, &macdSignal, &macdHistogram,
			&bollingerUpper, &bollingerMiddle, &bollingerLower,
			&ema9, &ema21, &ema50, &ema200,
			&vwap, &atr,
		); err != nil {
			return nil, fmt.Errorf("scan indicator row: %w", err)
		}

		fields := []struct {
			name string
			raw  string
			dest *decimal.Decimal
		}{
			{"rsi", rsi, &ind.Rsi},
			{"macd_line", macdLine, &ind.MacdLine},
			{"macd_signal", macdSignal, &ind.MacdSignal},
			{"macd_histogram", macdHistogram, &ind.MacdHistogram},
			{"bollinger_upper", bollingerUpper, &ind.BollingerUpper},
			{"bollinger_middle", bollingerMiddle, &ind.BollingerMiddle},
			{"bollinger_lower", bollingerLower, &ind.BollingerLower},
			{"ema_9", ema9, &ind.Ema9},
			{"ema_21", ema21, &ind.Ema21},
			{"ema_50", ema50, &ind.Ema50},
			{"ema_200", ema200, &ind.Ema200},
			{"vwap", vwap, &ind.Vwap},
			{"atr", atr, &ind.Atr},
		}
		for _, f := range fields {
			d, err := decimal.NewFromString(f.raw)
			if err != nil {
				return nil, fmt.Errorf("parse indicator field %q value %q: %w", f.name, f.raw, err)
			}
			*f.dest = d
		}

		indicators = append(indicators, ind)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate indicator rows: %w", err)
	}
	return indicators, nil
}
