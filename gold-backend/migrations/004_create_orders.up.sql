CREATE TYPE order_side AS ENUM ('BUY', 'SELL');
CREATE TYPE order_status AS ENUM ('pending', 'filled', 'partially_filled', 'cancelled', 'rejected', 'expired');
CREATE TYPE order_exchange AS ENUM ('binance', 'polymarket');

CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    exchange order_exchange NOT NULL,
    external_order_id VARCHAR(100),
    decision_id BIGINT REFERENCES decisions(id),
    symbol VARCHAR(20) NOT NULL,
    side order_side NOT NULL,
    quantity NUMERIC(20, 8) NOT NULL,
    price NUMERIC(20, 8),
    filled_quantity NUMERIC(20, 8) NOT NULL DEFAULT 0,
    filled_price NUMERIC(20, 8),
    fee NUMERIC(20, 8) NOT NULL DEFAULT 0,
    fee_asset VARCHAR(20),
    status order_status NOT NULL DEFAULT 'pending',
    raw_response JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_symbol_created_at ON orders (symbol, created_at DESC);
CREATE INDEX idx_orders_decision_id ON orders (decision_id);
CREATE INDEX idx_orders_status ON orders (status);
