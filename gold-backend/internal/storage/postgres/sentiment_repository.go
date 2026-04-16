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

// SentimentRepository defines persistence operations for AI-derived sentiment scores.
type SentimentRepository interface {
	InsertScore(ctx context.Context, score domain.SentimentScore) (int64, error)
	FindLatestScoresBySymbol(ctx context.Context, symbol string, limit int) ([]domain.SentimentScore, error)
	AggregateSentimentForSymbol(ctx context.Context, symbol string, since time.Time) (decimal.Decimal, error)
}

type sentimentRepository struct {
	pool *pgxpool.Pool
}

// NewSentimentRepository returns a SentimentRepository backed by the given connection pool.
func NewSentimentRepository(pool *pgxpool.Pool) SentimentRepository {
	return &sentimentRepository{pool: pool}
}

// InsertScore persists a sentiment score record and returns its generated ID.
func (r *sentimentRepository) InsertScore(ctx context.Context, s domain.SentimentScore) (int64, error) {
	const query = `
		INSERT INTO sentiment_scores
			(article_id, symbol, direction, confidence, raw_score, model_used)
		VALUES
			($1, $2, $3::sentiment_direction, $4::numeric, $5::numeric, $6)
		RETURNING id`

	var id int64
	err := r.pool.QueryRow(ctx, query,
		s.ArticleID, s.Symbol, string(s.Direction),
		decimalToString(s.Confidence), decimalToString(s.RawScore),
		nullableText(s.ModelUsed),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert sentiment score for symbol %q article_id %d: %w",
			s.Symbol, s.ArticleID, err)
	}
	return id, nil
}

// FindLatestScoresBySymbol returns the most recent sentiment scores for a symbol,
// ordered by created_at descending.
func (r *sentimentRepository) FindLatestScoresBySymbol(ctx context.Context, symbol string, limit int) ([]domain.SentimentScore, error) {
	const query = `
		SELECT id, article_id, symbol, direction,
		       confidence::text, COALESCE(raw_score::text, '0'), COALESCE(model_used, ''),
		       created_at
		FROM sentiment_scores
		WHERE symbol = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, query, symbol, limit)
	if err != nil {
		return nil, fmt.Errorf("query latest sentiment scores for symbol %q limit %d: %w", symbol, limit, err)
	}
	defer rows.Close()

	return scanSentimentScoreRows(rows)
}

// AggregateSentimentForSymbol computes a weighted average sentiment score in [-1, 1] for a
// symbol since the given timestamp. Positive direction scores +confidence, negative
// scores -confidence, neutral scores 0. Returns zero when no scores exist.
func (r *sentimentRepository) AggregateSentimentForSymbol(ctx context.Context, symbol string, since time.Time) (decimal.Decimal, error) {
	// Weighted average: map direction to a signed multiplier, weight by confidence.
	// Result is in [-1, 1]: SUM(multiplier * confidence) / SUM(confidence)
	const query = `
		SELECT
			COALESCE(
				SUM(
					CASE direction
						WHEN 'positive' THEN  confidence
						WHEN 'negative' THEN -confidence
						ELSE 0
					END
				) / NULLIF(SUM(confidence), 0),
			0)::text
		FROM sentiment_scores
		WHERE symbol = $1 AND created_at >= $2`

	var result string
	if err := r.pool.QueryRow(ctx, query, symbol, since).Scan(&result); err != nil {
		return decimal.Zero, fmt.Errorf("aggregate sentiment for symbol %q since %v: %w", symbol, since, err)
	}

	score, err := decimal.NewFromString(result)
	if err != nil {
		return decimal.Zero, fmt.Errorf("parse aggregate sentiment result %q: %w", result, err)
	}
	return score, nil
}

func scanSentimentScoreRows(rows pgx.Rows) ([]domain.SentimentScore, error) {
	var scores []domain.SentimentScore
	for rows.Next() {
		var s domain.SentimentScore
		var direction string
		var confidence, rawScore string

		if err := rows.Scan(
			&s.ID, &s.ArticleID, &s.Symbol, &direction,
			&confidence, &rawScore, &s.ModelUsed,
			&s.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan sentiment score row: %w", err)
		}

		s.Direction = domain.SentimentDirection(direction)

		var err error
		if s.Confidence, err = decimal.NewFromString(confidence); err != nil {
			return nil, fmt.Errorf("parse sentiment confidence %q: %w", confidence, err)
		}
		if s.RawScore, err = decimal.NewFromString(rawScore); err != nil {
			return nil, fmt.Errorf("parse sentiment raw_score %q: %w", rawScore, err)
		}

		scores = append(scores, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sentiment score rows: %w", err)
	}
	return scores, nil
}
