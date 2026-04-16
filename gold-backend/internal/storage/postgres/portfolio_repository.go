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

// PortfolioRepository defines persistence operations for portfolio metric snapshots.
type PortfolioRepository interface {
	InsertSnapshot(ctx context.Context, metrics domain.PortfolioMetrics) (int64, error)
	FindLatestSnapshot(ctx context.Context) (*domain.PortfolioMetrics, error)
	FindSnapshotsByRange(ctx context.Context, from, to time.Time) ([]domain.PortfolioMetrics, error)
}

type portfolioRepository struct {
	pool *pgxpool.Pool
}

// NewPortfolioRepository returns a PortfolioRepository backed by the given connection pool.
func NewPortfolioRepository(pool *pgxpool.Pool) PortfolioRepository {
	return &portfolioRepository{pool: pool}
}

// InsertSnapshot persists a portfolio metrics snapshot and returns its generated ID.
func (r *portfolioRepository) InsertSnapshot(ctx context.Context, m domain.PortfolioMetrics) (int64, error) {
	const query = `
		INSERT INTO portfolio_snapshots
			(balance, peak_balance, drawdown_percent, total_pnl,
			 win_count, loss_count, total_trades, win_rate,
			 profit_factor, average_win, average_loss, sharpe_ratio,
			 max_drawdown_percent, is_circuit_breaker_active)
		VALUES
			($1::numeric, $2::numeric, $3::numeric, $4::numeric,
			 $5, $6, $7, $8::numeric,
			 $9::numeric, $10::numeric, $11::numeric, $12::numeric,
			 $13::numeric, $14)
		RETURNING id`

	var id int64
	err := r.pool.QueryRow(ctx, query,
		decimalToString(m.Balance), decimalToString(m.PeakBalance),
		decimalToString(m.DrawdownPercent), decimalToString(m.TotalPnl),
		m.WinCount, m.LossCount, m.TotalTrades, decimalToString(m.WinRate),
		decimalToString(m.ProfitFactor), decimalToString(m.AverageWin),
		decimalToString(m.AverageLoss), decimalToString(m.SharpeRatio),
		decimalToString(m.MaxDrawdownPercent), m.IsCircuitBreakerActive,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert portfolio snapshot: %w", err)
	}
	return id, nil
}

// FindLatestSnapshot returns the most recent portfolio snapshot, or nil if none exists.
func (r *portfolioRepository) FindLatestSnapshot(ctx context.Context) (*domain.PortfolioMetrics, error) {
	const query = `
		SELECT id,
		       balance::text, peak_balance::text, drawdown_percent::text, total_pnl::text,
		       win_count, loss_count, total_trades, win_rate::text,
		       COALESCE(profit_factor::text, '0'),
		       COALESCE(average_win::text, '0'),
		       COALESCE(average_loss::text, '0'),
		       COALESCE(sharpe_ratio::text, '0'),
		       max_drawdown_percent::text, is_circuit_breaker_active,
		       snapshot_at
		FROM portfolio_snapshots
		ORDER BY snapshot_at DESC
		LIMIT 1`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query latest portfolio snapshot: %w", err)
	}
	defer rows.Close()

	snapshots, err := scanPortfolioRows(rows)
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, nil
	}
	return &snapshots[0], nil
}

// FindSnapshotsByRange returns portfolio snapshots within [from, to], ordered by snapshot_at ascending.
func (r *portfolioRepository) FindSnapshotsByRange(ctx context.Context, from, to time.Time) ([]domain.PortfolioMetrics, error) {
	const query = `
		SELECT id,
		       balance::text, peak_balance::text, drawdown_percent::text, total_pnl::text,
		       win_count, loss_count, total_trades, win_rate::text,
		       COALESCE(profit_factor::text, '0'),
		       COALESCE(average_win::text, '0'),
		       COALESCE(average_loss::text, '0'),
		       COALESCE(sharpe_ratio::text, '0'),
		       max_drawdown_percent::text, is_circuit_breaker_active,
		       snapshot_at
		FROM portfolio_snapshots
		WHERE snapshot_at >= $1 AND snapshot_at <= $2
		ORDER BY snapshot_at ASC`

	rows, err := r.pool.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("query portfolio snapshots range [%v, %v]: %w", from, to, err)
	}
	defer rows.Close()

	return scanPortfolioRows(rows)
}

func scanPortfolioRows(rows pgx.Rows) ([]domain.PortfolioMetrics, error) {
	var snapshots []domain.PortfolioMetrics
	for rows.Next() {
		var m domain.PortfolioMetrics
		var balance, peakBalance, drawdownPercent, totalPnl string
		var winRate, profitFactor, averageWin, averageLoss, sharpeRatio string
		var maxDrawdownPercent string

		if err := rows.Scan(
			&m.ID,
			&balance, &peakBalance, &drawdownPercent, &totalPnl,
			&m.WinCount, &m.LossCount, &m.TotalTrades, &winRate,
			&profitFactor, &averageWin, &averageLoss, &sharpeRatio,
			&maxDrawdownPercent, &m.IsCircuitBreakerActive,
			&m.SnapshotAt,
		); err != nil {
			return nil, fmt.Errorf("scan portfolio snapshot row: %w", err)
		}

		fields := []struct {
			name string
			raw  string
			dest *decimal.Decimal
		}{
			{"balance", balance, &m.Balance},
			{"peak_balance", peakBalance, &m.PeakBalance},
			{"drawdown_percent", drawdownPercent, &m.DrawdownPercent},
			{"total_pnl", totalPnl, &m.TotalPnl},
			{"win_rate", winRate, &m.WinRate},
			{"profit_factor", profitFactor, &m.ProfitFactor},
			{"average_win", averageWin, &m.AverageWin},
			{"average_loss", averageLoss, &m.AverageLoss},
			{"sharpe_ratio", sharpeRatio, &m.SharpeRatio},
			{"max_drawdown_percent", maxDrawdownPercent, &m.MaxDrawdownPercent},
		}
		for _, f := range fields {
			val, err := decimal.NewFromString(f.raw)
			if err != nil {
				return nil, fmt.Errorf("parse portfolio field %q value %q: %w", f.name, f.raw, err)
			}
			*f.dest = val
		}

		snapshots = append(snapshots, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate portfolio snapshot rows: %w", err)
	}
	return snapshots, nil
}
