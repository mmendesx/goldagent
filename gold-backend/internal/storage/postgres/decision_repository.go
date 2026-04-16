package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

// DecisionRepository defines persistence operations for AI trading decisions.
type DecisionRepository interface {
	InsertDecision(ctx context.Context, decision domain.Decision) (int64, error)
	UpdateDecisionExecutionStatus(ctx context.Context, id int64, status domain.DecisionExecutionStatus, rejectionReason string) error
	FindDecisionsBySymbol(ctx context.Context, symbol string, limit, offset int) ([]domain.Decision, error)
	FindRecentDecisions(ctx context.Context, limit, offset int) ([]domain.Decision, error)
}

type decisionRepository struct {
	pool *pgxpool.Pool
}

// NewDecisionRepository returns a DecisionRepository backed by the given connection pool.
func NewDecisionRepository(pool *pgxpool.Pool) DecisionRepository {
	return &decisionRepository{pool: pool}
}

// InsertDecision persists a decision record and returns its generated ID.
func (r *decisionRepository) InsertDecision(ctx context.Context, d domain.Decision) (int64, error) {
	const query = `
		INSERT INTO decisions
			(symbol, action, confidence, execution_status, rejection_reason,
			 rsi_signal, macd_signal, bollinger_signal, ema_signal,
			 pattern_signal, sentiment_signal, support_resistance_signal,
			 composite_score, is_dry_run)
		VALUES
			($1, $2::decision_action, $3, $4::decision_execution_status, $5,
			 $6::numeric, $7::numeric, $8::numeric, $9::numeric,
			 $10::numeric, $11::numeric, $12::numeric,
			 $13::numeric, $14)
		RETURNING id`

	var id int64
	err := r.pool.QueryRow(ctx, query,
		d.Symbol, string(d.Action), d.Confidence, string(d.ExecutionStatus), nullableText(d.RejectionReason),
		decimalToString(d.RsiSignal), decimalToString(d.MacdSignal),
		decimalToString(d.BollingerSignal), decimalToString(d.EmaSignal),
		decimalToString(d.PatternSignal), decimalToString(d.SentimentSignal),
		decimalToString(d.SupportResistanceSignal),
		decimalToString(d.CompositeScore), d.IsDryRun,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert decision for symbol %q action %q: %w", d.Symbol, d.Action, err)
	}
	return id, nil
}

// UpdateDecisionExecutionStatus updates the execution_status and optional rejection_reason
// for the given decision ID.
func (r *decisionRepository) UpdateDecisionExecutionStatus(
	ctx context.Context,
	id int64,
	status domain.DecisionExecutionStatus,
	rejectionReason string,
) error {
	const query = `
		UPDATE decisions
		SET execution_status = $2::decision_execution_status,
		    rejection_reason = $3
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query, id, string(status), nullableText(rejectionReason))
	if err != nil {
		return fmt.Errorf("update decision execution status for id %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update decision execution status: decision with id %d not found", id)
	}
	return nil
}

// FindDecisionsBySymbol returns paginated decisions for a specific symbol,
// ordered by created_at descending.
func (r *decisionRepository) FindDecisionsBySymbol(ctx context.Context, symbol string, limit, offset int) ([]domain.Decision, error) {
	const query = `
		SELECT id, symbol, action, confidence, execution_status,
		       COALESCE(rejection_reason, ''),
		       COALESCE(rsi_signal::text, '0'), COALESCE(macd_signal::text, '0'),
		       COALESCE(bollinger_signal::text, '0'),
		       COALESCE(ema_signal::text, '0'), COALESCE(pattern_signal::text, '0'),
		       COALESCE(sentiment_signal::text, '0'),
		       COALESCE(support_resistance_signal::text, '0'), composite_score::text,
		       is_dry_run, created_at
		FROM decisions
		WHERE symbol = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, symbol, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query decisions for symbol %q limit %d offset %d: %w", symbol, limit, offset, err)
	}
	defer rows.Close()

	return scanDecisionRows(rows)
}

// FindRecentDecisions returns the most recent decisions across all symbols,
// ordered by created_at descending, with pagination.
func (r *decisionRepository) FindRecentDecisions(ctx context.Context, limit, offset int) ([]domain.Decision, error) {
	const query = `
		SELECT id, symbol, action, confidence, execution_status,
		       COALESCE(rejection_reason, ''),
		       COALESCE(rsi_signal::text, '0'), COALESCE(macd_signal::text, '0'),
		       COALESCE(bollinger_signal::text, '0'),
		       COALESCE(ema_signal::text, '0'), COALESCE(pattern_signal::text, '0'),
		       COALESCE(sentiment_signal::text, '0'),
		       COALESCE(support_resistance_signal::text, '0'), composite_score::text,
		       is_dry_run, created_at
		FROM decisions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query recent decisions limit %d offset %d: %w", limit, offset, err)
	}
	defer rows.Close()

	return scanDecisionRows(rows)
}

func scanDecisionRows(rows pgx.Rows) ([]domain.Decision, error) {
	var decisions []domain.Decision
	for rows.Next() {
		var d domain.Decision
		var action, executionStatus string
		var rsiSignal, macdSignal, bollingerSignal, emaSignal string
		var patternSignal, sentimentSignal, supportResistanceSignal, compositeScore string

		if err := rows.Scan(
			&d.ID, &d.Symbol, &action, &d.Confidence, &executionStatus,
			&d.RejectionReason,
			&rsiSignal, &macdSignal, &bollingerSignal,
			&emaSignal, &patternSignal, &sentimentSignal,
			&supportResistanceSignal, &compositeScore,
			&d.IsDryRun, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan decision row: %w", err)
		}

		d.Action = domain.DecisionAction(action)
		d.ExecutionStatus = domain.DecisionExecutionStatus(executionStatus)

		fields := []struct {
			name string
			raw  string
			dest *decimal.Decimal
		}{
			{"rsi_signal", rsiSignal, &d.RsiSignal},
			{"macd_signal", macdSignal, &d.MacdSignal},
			{"bollinger_signal", bollingerSignal, &d.BollingerSignal},
			{"ema_signal", emaSignal, &d.EmaSignal},
			{"pattern_signal", patternSignal, &d.PatternSignal},
			{"sentiment_signal", sentimentSignal, &d.SentimentSignal},
			{"support_resistance_signal", supportResistanceSignal, &d.SupportResistanceSignal},
			{"composite_score", compositeScore, &d.CompositeScore},
		}
		for _, f := range fields {
			val, err := decimal.NewFromString(f.raw)
			if err != nil {
				return nil, fmt.Errorf("parse decision field %q value %q: %w", f.name, f.raw, err)
			}
			*f.dest = val
		}

		decisions = append(decisions, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate decision rows: %w", err)
	}
	return decisions, nil
}

// nullableText returns nil for an empty string so that optional TEXT columns
// store NULL rather than an empty string.
func nullableText(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
