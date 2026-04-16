package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

// NewsRepository defines persistence operations for news articles.
type NewsRepository interface {
	InsertArticle(ctx context.Context, article domain.NewsArticle) (int64, error)
	FindArticleByExternalID(ctx context.Context, externalID string) (*domain.NewsArticle, error)
	FindRecentArticles(ctx context.Context, limit int) ([]domain.NewsArticle, error)
}

type newsRepository struct {
	pool *pgxpool.Pool
}

// NewNewsRepository returns a NewsRepository backed by the given connection pool.
func NewNewsRepository(pool *pgxpool.Pool) NewsRepository {
	return &newsRepository{pool: pool}
}

// InsertArticle persists a news article and returns its generated ID.
func (r *newsRepository) InsertArticle(ctx context.Context, a domain.NewsArticle) (int64, error) {
	const query = `
		INSERT INTO news_articles
			(external_id, source, title, url, published_at, raw_content)
		VALUES
			($1, $2, $3, $4, $5, $6)
		RETURNING id`

	var id int64
	err := r.pool.QueryRow(ctx, query,
		nullableText(a.ExternalID), a.Source, a.Title,
		nullableText(a.URL), a.PublishedAt, nullableText(a.RawContent),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert news article from source %q title %q: %w", a.Source, a.Title, err)
	}
	return id, nil
}

// FindArticleByExternalID returns the article with the given external ID, or nil if not found.
func (r *newsRepository) FindArticleByExternalID(ctx context.Context, externalID string) (*domain.NewsArticle, error) {
	const query = `
		SELECT id, COALESCE(external_id, ''), source, title,
		       COALESCE(url, ''), published_at, COALESCE(raw_content, ''), created_at
		FROM news_articles
		WHERE external_id = $1`

	rows, err := r.pool.Query(ctx, query, externalID)
	if err != nil {
		return nil, fmt.Errorf("query news article by external_id %q: %w", externalID, err)
	}
	defer rows.Close()

	articles, err := scanNewsArticleRows(rows)
	if err != nil {
		return nil, err
	}
	if len(articles) == 0 {
		return nil, nil
	}
	return &articles[0], nil
}

// FindRecentArticles returns the most recent articles ordered by published_at descending.
func (r *newsRepository) FindRecentArticles(ctx context.Context, limit int) ([]domain.NewsArticle, error) {
	const query = `
		SELECT id, COALESCE(external_id, ''), source, title,
		       COALESCE(url, ''), published_at, COALESCE(raw_content, ''), created_at
		FROM news_articles
		ORDER BY published_at DESC
		LIMIT $1`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent news articles limit %d: %w", limit, err)
	}
	defer rows.Close()

	return scanNewsArticleRows(rows)
}

func scanNewsArticleRows(rows pgx.Rows) ([]domain.NewsArticle, error) {
	var articles []domain.NewsArticle
	for rows.Next() {
		var a domain.NewsArticle
		if err := rows.Scan(
			&a.ID, &a.ExternalID, &a.Source, &a.Title,
			&a.URL, &a.PublishedAt, &a.RawContent, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan news article row: %w", err)
		}
		articles = append(articles, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate news article rows: %w", err)
	}
	return articles, nil
}
