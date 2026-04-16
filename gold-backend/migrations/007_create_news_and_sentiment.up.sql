CREATE TYPE sentiment_direction AS ENUM ('positive', 'negative', 'neutral');

CREATE TABLE news_articles (
    id BIGSERIAL PRIMARY KEY,
    external_id VARCHAR(255),
    source VARCHAR(100) NOT NULL,
    title TEXT NOT NULL,
    url TEXT,
    published_at TIMESTAMPTZ NOT NULL,
    raw_content TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_news_articles_published_at ON news_articles (published_at DESC);
CREATE INDEX idx_news_articles_source ON news_articles (source);
CREATE UNIQUE INDEX idx_news_articles_external_id ON news_articles (external_id) WHERE external_id IS NOT NULL;

CREATE TABLE sentiment_scores (
    id BIGSERIAL PRIMARY KEY,
    article_id BIGINT NOT NULL REFERENCES news_articles(id),
    symbol VARCHAR(20) NOT NULL,
    direction sentiment_direction NOT NULL,
    confidence NUMERIC(5, 4) NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    raw_score NUMERIC(10, 4),
    model_used VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sentiment_scores_symbol_created_at ON sentiment_scores (symbol, created_at DESC);
CREATE INDEX idx_sentiment_scores_article_id ON sentiment_scores (article_id);
