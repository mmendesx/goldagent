package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

// PositionRepository defines persistence operations for trading positions.
type PositionRepository interface {
	InsertPosition(ctx context.Context, position domain.Position) (int64, error)
	UpdatePositionTrailingStop(ctx context.Context, id int64, trailingStopPrice decimal.Decimal) error
	ClosePosition(ctx context.Context, id int64, exitOrderID int64, exitPrice, realizedPnl decimal.Decimal, closeReason string) error
	FindOpenPositions(ctx context.Context) ([]domain.Position, error)
	CountOpenPositions(ctx context.Context) (int, error)
	FindClosedPositions(ctx context.Context, limit, offset int) ([]domain.Position, error)
	FindPositionByID(ctx context.Context, id int64) (*domain.Position, error)
}

type positionRepository struct {
	pool *pgxpool.Pool
}

// NewPositionRepository returns a PositionRepository backed by the given connection pool.
func NewPositionRepository(pool *pgxpool.Pool) PositionRepository {
	return &positionRepository{pool: pool}
}

// InsertPosition persists a new position and returns its generated ID.
func (r *positionRepository) InsertPosition(ctx context.Context, p domain.Position) (int64, error) {
	const query = `
		INSERT INTO positions
			(symbol, side, entry_order_id, entry_price, quantity,
			 take_profit_price, stop_loss_price,
			 trailing_stop_distance, trailing_stop_price,
			 fee_total, status)
		VALUES
			($1, $2::position_side, $3, $4::numeric, $5::numeric,
			 $6::numeric, $7::numeric,
			 $8::numeric, $9::numeric,
			 $10::numeric, $11::position_status)
		RETURNING id`

	var id int64
	err := r.pool.QueryRow(ctx, query,
		p.Symbol, p.Side, p.EntryOrderID,
		decimalToString(p.EntryPrice), decimalToString(p.Quantity),
		decimalToString(p.TakeProfitPrice), decimalToString(p.StopLossPrice),
		decimalToString(p.TrailingStopDistance), decimalToString(p.TrailingStopPrice),
		decimalToString(p.FeeTotal), p.Status,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert position for symbol %q side %q: %w", p.Symbol, p.Side, err)
	}
	return id, nil
}

// UpdatePositionTrailingStop updates the trailing stop price for the given position ID.
func (r *positionRepository) UpdatePositionTrailingStop(ctx context.Context, id int64, trailingStopPrice decimal.Decimal) error {
	const query = `
		UPDATE positions
		SET trailing_stop_price = $2::numeric,
		    updated_at          = NOW()
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query, id, decimalToString(trailingStopPrice))
	if err != nil {
		return fmt.Errorf("update trailing stop for position id %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update trailing stop: position with id %d not found", id)
	}
	return nil
}

// ClosePosition marks a position as closed, recording the exit order, prices, P&L, and reason.
func (r *positionRepository) ClosePosition(
	ctx context.Context,
	id int64,
	exitOrderID int64,
	exitPrice, realizedPnl decimal.Decimal,
	closeReason string,
) error {
	const query = `
		UPDATE positions
		SET status       = 'closed'::position_status,
		    exit_order_id = $2,
		    exit_price   = $3::numeric,
		    realized_pnl = $4::numeric,
		    close_reason = $5::position_close_reason,
		    closed_at    = NOW(),
		    updated_at   = NOW()
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query,
		id, exitOrderID,
		decimalToString(exitPrice), decimalToString(realizedPnl),
		closeReason,
	)
	if err != nil {
		return fmt.Errorf("close position id %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("close position: position with id %d not found", id)
	}
	return nil
}

// FindOpenPositions returns all positions with status 'open', ordered by opened_at ascending.
func (r *positionRepository) FindOpenPositions(ctx context.Context) ([]domain.Position, error) {
	const query = `
		SELECT id, symbol, side, entry_order_id, exit_order_id,
		       entry_price::text, COALESCE(exit_price::text, '0'),
		       quantity::text,
		       COALESCE(take_profit_price::text, '0'),
		       COALESCE(stop_loss_price::text, '0'),
		       COALESCE(trailing_stop_distance::text, '0'),
		       COALESCE(trailing_stop_price::text, '0'),
		       COALESCE(realized_pnl::text, '0'),
		       fee_total::text,
		       status, COALESCE(close_reason, ''),
		       opened_at, closed_at, updated_at
		FROM positions
		WHERE status = 'open'
		ORDER BY opened_at ASC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query open positions: %w", err)
	}
	defer rows.Close()

	return scanPositionRows(rows)
}

// CountOpenPositions returns the count of positions currently with status 'open'.
func (r *positionRepository) CountOpenPositions(ctx context.Context) (int, error) {
	const query = `SELECT COUNT(*) FROM positions WHERE status = 'open'`

	var count int
	if err := r.pool.QueryRow(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("count open positions: %w", err)
	}
	return count, nil
}

// FindClosedPositions returns paginated closed positions, ordered by closed_at descending.
func (r *positionRepository) FindClosedPositions(ctx context.Context, limit, offset int) ([]domain.Position, error) {
	const query = `
		SELECT id, symbol, side, entry_order_id, exit_order_id,
		       entry_price::text, COALESCE(exit_price::text, '0'),
		       quantity::text,
		       COALESCE(take_profit_price::text, '0'),
		       COALESCE(stop_loss_price::text, '0'),
		       COALESCE(trailing_stop_distance::text, '0'),
		       COALESCE(trailing_stop_price::text, '0'),
		       COALESCE(realized_pnl::text, '0'),
		       fee_total::text,
		       status, COALESCE(close_reason, ''),
		       opened_at, closed_at, updated_at
		FROM positions
		WHERE status = 'closed'
		ORDER BY closed_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query closed positions limit %d offset %d: %w", limit, offset, err)
	}
	defer rows.Close()

	return scanPositionRows(rows)
}

// FindPositionByID returns the position with the given ID, or nil if it does not exist.
func (r *positionRepository) FindPositionByID(ctx context.Context, id int64) (*domain.Position, error) {
	const query = `
		SELECT id, symbol, side, entry_order_id, exit_order_id,
		       entry_price::text, COALESCE(exit_price::text, '0'),
		       quantity::text,
		       COALESCE(take_profit_price::text, '0'),
		       COALESCE(stop_loss_price::text, '0'),
		       COALESCE(trailing_stop_distance::text, '0'),
		       COALESCE(trailing_stop_price::text, '0'),
		       COALESCE(realized_pnl::text, '0'),
		       fee_total::text,
		       status, COALESCE(close_reason, ''),
		       opened_at, closed_at, updated_at
		FROM positions
		WHERE id = $1`

	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("query position by id %d: %w", id, err)
	}
	defer rows.Close()

	positions, err := scanPositionRows(rows)
	if err != nil {
		return nil, err
	}
	if len(positions) == 0 {
		return nil, nil
	}
	return &positions[0], nil
}

func scanPositionRows(rows pgx.Rows) ([]domain.Position, error) {
	var positions []domain.Position
	for rows.Next() {
		var p domain.Position
		var side string
		var entryPrice, exitPrice, quantity string
		var takeProfitPrice, stopLossPrice string
		var trailingStopDistance, trailingStopPrice, realizedPnl, feeTotal string
		var status, closeReason string

		if err := rows.Scan(
			&p.ID, &p.Symbol, &side, &p.EntryOrderID, &p.ExitOrderID,
			&entryPrice, &exitPrice,
			&quantity,
			&takeProfitPrice, &stopLossPrice,
			&trailingStopDistance, &trailingStopPrice,
			&realizedPnl,
			&feeTotal,
			&status, &closeReason,
			&p.OpenedAt, &p.ClosedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan position row: %w", err)
		}

		p.Side = side
		p.Status = status
		p.CloseReason = closeReason

		fields := []struct {
			name string
			raw  string
			dest *decimal.Decimal
		}{
			{"entry_price", entryPrice, &p.EntryPrice},
			{"exit_price", exitPrice, &p.ExitPrice},
			{"quantity", quantity, &p.Quantity},
			{"take_profit_price", takeProfitPrice, &p.TakeProfitPrice},
			{"stop_loss_price", stopLossPrice, &p.StopLossPrice},
			{"trailing_stop_distance", trailingStopDistance, &p.TrailingStopDistance},
			{"trailing_stop_price", trailingStopPrice, &p.TrailingStopPrice},
			{"realized_pnl", realizedPnl, &p.RealizedPnl},
			{"fee_total", feeTotal, &p.FeeTotal},
		}
		for _, f := range fields {
			val, err := decimal.NewFromString(f.raw)
			if err != nil {
				return nil, fmt.Errorf("parse position field %q value %q: %w", f.name, f.raw, err)
			}
			*f.dest = val
		}

		positions = append(positions, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate position rows: %w", err)
	}
	return positions, nil
}
