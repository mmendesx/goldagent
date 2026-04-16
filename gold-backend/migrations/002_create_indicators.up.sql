CREATE TABLE indicators (
    id BIGSERIAL PRIMARY KEY,
    candle_id BIGINT NOT NULL,
    symbol VARCHAR(20) NOT NULL,
    interval VARCHAR(5) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    rsi NUMERIC(10, 4),
    macd_line NUMERIC(20, 8),
    macd_signal NUMERIC(20, 8),
    macd_histogram NUMERIC(20, 8),
    bollinger_upper NUMERIC(20, 8),
    bollinger_middle NUMERIC(20, 8),
    bollinger_lower NUMERIC(20, 8),
    ema_9 NUMERIC(20, 8),
    ema_21 NUMERIC(20, 8),
    ema_50 NUMERIC(20, 8),
    ema_200 NUMERIC(20, 8),
    vwap NUMERIC(20, 8),
    atr NUMERIC(20, 8),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_indicators_symbol_interval_timestamp ON indicators (symbol, interval, timestamp DESC);
CREATE INDEX idx_indicators_candle_id ON indicators (candle_id);
