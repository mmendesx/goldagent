CREATE TYPE position_side AS ENUM ('LONG', 'SHORT');
CREATE TYPE position_status AS ENUM ('open', 'closed');
CREATE TYPE position_close_reason AS ENUM ('TAKE_PROFIT', 'STOP_LOSS', 'TRAILING_STOP', 'MANUAL', 'CIRCUIT_BREAKER');

CREATE TABLE positions (
    id BIGSERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    side position_side NOT NULL,
    entry_order_id BIGINT REFERENCES orders(id),
    exit_order_id BIGINT REFERENCES orders(id),
    entry_price NUMERIC(20, 8) NOT NULL,
    exit_price NUMERIC(20, 8),
    quantity NUMERIC(20, 8) NOT NULL,
    take_profit_price NUMERIC(20, 8),
    stop_loss_price NUMERIC(20, 8),
    trailing_stop_distance NUMERIC(20, 8),
    trailing_stop_price NUMERIC(20, 8),
    realized_pnl NUMERIC(20, 8),
    fee_total NUMERIC(20, 8) NOT NULL DEFAULT 0,
    status position_status NOT NULL DEFAULT 'open',
    close_reason position_close_reason,
    opened_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_positions_status ON positions (status);
CREATE INDEX idx_positions_symbol_status ON positions (symbol, status);
CREATE INDEX idx_positions_opened_at ON positions (opened_at DESC);
CREATE INDEX idx_positions_closed_at ON positions (closed_at DESC);
