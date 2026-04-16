CREATE TYPE decision_action AS ENUM ('BUY', 'SELL', 'HOLD');
CREATE TYPE decision_execution_status AS ENUM ('executed', 'below_confidence_threshold', 'max_positions_reached', 'circuit_breaker_active', 'dry_run', 'rejected', 'pending');

CREATE TABLE decisions (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    action decision_action NOT NULL,
    confidence INTEGER NOT NULL CHECK (confidence >= 0 AND confidence <= 100),
    execution_status decision_execution_status NOT NULL DEFAULT 'pending',
    rejection_reason TEXT,
    rsi_signal NUMERIC(10, 4),
    macd_signal NUMERIC(10, 4),
    bollinger_signal NUMERIC(10, 4),
    ema_signal NUMERIC(10, 4),
    pattern_signal NUMERIC(10, 4),
    sentiment_signal NUMERIC(10, 4),
    support_resistance_signal NUMERIC(10, 4),
    composite_score NUMERIC(10, 4) NOT NULL,
    is_dry_run BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_decisions_symbol_created_at ON decisions (symbol, created_at DESC);
CREATE INDEX idx_decisions_execution_status ON decisions (execution_status);
