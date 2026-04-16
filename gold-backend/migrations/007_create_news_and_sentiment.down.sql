DROP INDEX IF EXISTS idx_sentiment_scores_article_id;
DROP INDEX IF EXISTS idx_sentiment_scores_symbol_created_at;
DROP TABLE IF EXISTS sentiment_scores;

DROP INDEX IF EXISTS idx_news_articles_external_id;
DROP INDEX IF EXISTS idx_news_articles_source;
DROP INDEX IF EXISTS idx_news_articles_published_at;
DROP TABLE IF EXISTS news_articles;

DROP TYPE IF EXISTS sentiment_direction;
