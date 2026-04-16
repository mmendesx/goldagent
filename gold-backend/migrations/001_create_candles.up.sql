CREATE TABLE candles (
    id BIGSERIAL,
    symbol VARCHAR(20) NOT NULL,
    interval VARCHAR(5) NOT NULL,
    open_time TIMESTAMPTZ NOT NULL,
    close_time TIMESTAMPTZ NOT NULL,
    open_price NUMERIC(20, 8) NOT NULL,
    high_price NUMERIC(20, 8) NOT NULL,
    low_price NUMERIC(20, 8) NOT NULL,
    close_price NUMERIC(20, 8) NOT NULL,
    volume NUMERIC(20, 8) NOT NULL,
    quote_volume NUMERIC(20, 8) NOT NULL,
    trade_count INTEGER NOT NULL DEFAULT 0,
    is_closed BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, symbol)
) PARTITION BY LIST (symbol);

-- Create partitions for each supported symbol
CREATE TABLE candles_btcusdt PARTITION OF candles FOR VALUES IN ('BTCUSDT');
CREATE TABLE candles_ethusdt PARTITION OF candles FOR VALUES IN ('ETHUSDT');
CREATE TABLE candles_solusdt PARTITION OF candles FOR VALUES IN ('SOLUSDT');
CREATE TABLE candles_bnbusdt PARTITION OF candles FOR VALUES IN ('BNBUSDT');

CREATE INDEX idx_candles_symbol_interval_open_time ON candles (symbol, interval, open_time DESC);
CREATE INDEX idx_candles_symbol_interval_close_time ON candles (symbol, interval, close_time DESC);
CREATE UNIQUE INDEX idx_candles_unique ON candles (symbol, interval, open_time);
