-- Gold Agent — full database schema
-- Apply with: psql $GOLD_DATABASE_URL < migrations/schema.sql
-- Idempotent: uses CREATE TYPE IF NOT EXISTS / CREATE TABLE IF NOT EXISTS

-- ---------------------------------------------------------------------------
-- Enum types
-- ---------------------------------------------------------------------------

DO $$ BEGIN
    CREATE TYPE decision_action AS ENUM ('BUY', 'SELL', 'HOLD');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE decision_execution_status AS ENUM (
        'pending', 'executed', 'rejected', 'dry_run', 'error'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE position_side AS ENUM ('LONG', 'SHORT');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE position_status AS ENUM ('open', 'closed');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE position_close_reason AS ENUM (
        'TAKE_PROFIT', 'STOP_LOSS', 'TRAILING_STOP', 'MANUAL', 'CIRCUIT_BREAKER'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE order_exchange AS ENUM ('binance', 'polymarket');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE order_side AS ENUM ('BUY', 'SELL');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE order_status AS ENUM (
        'pending', 'filled', 'partially_filled', 'cancelled', 'rejected', 'expired'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ---------------------------------------------------------------------------
-- candles
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS candles (
    id            SERIAL PRIMARY KEY,
    symbol        VARCHAR(20)   NOT NULL,
    interval      VARCHAR(10)   NOT NULL,
    open_time     TIMESTAMPTZ   NOT NULL,
    close_time    TIMESTAMPTZ   NOT NULL,
    open_price    NUMERIC(24,8) NOT NULL,
    high_price    NUMERIC(24,8) NOT NULL,
    low_price     NUMERIC(24,8) NOT NULL,
    close_price   NUMERIC(24,8) NOT NULL,
    volume        NUMERIC(24,8) NOT NULL,
    quote_volume  NUMERIC(24,8) NOT NULL DEFAULT 0,
    trade_count   INTEGER       NOT NULL DEFAULT 0,
    is_closed     BOOLEAN       NOT NULL DEFAULT FALSE,
    CONSTRAINT candles_symbol_interval_open_time_key
        UNIQUE (symbol, interval, open_time)
);

CREATE INDEX IF NOT EXISTS candles_symbol_interval_open_time_idx
    ON candles (symbol, interval, open_time DESC);

-- ---------------------------------------------------------------------------
-- indicators
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS indicators (
    id               SERIAL PRIMARY KEY,
    candle_id        INTEGER       NOT NULL REFERENCES candles(id) ON DELETE CASCADE,
    symbol           VARCHAR(20)   NOT NULL,
    interval         VARCHAR(10)   NOT NULL,
    timestamp        TIMESTAMPTZ   NOT NULL,
    rsi              NUMERIC(10,4),
    macd_line        NUMERIC(20,8),
    macd_signal      NUMERIC(20,8),
    macd_histogram   NUMERIC(20,8),
    bollinger_upper  NUMERIC(24,8),
    bollinger_middle NUMERIC(24,8),
    bollinger_lower  NUMERIC(24,8),
    ema_9            NUMERIC(24,8),
    ema_21           NUMERIC(24,8),
    ema_50           NUMERIC(24,8),
    ema_200          NUMERIC(24,8),
    vwap             NUMERIC(24,8),
    atr              NUMERIC(24,8)
);

CREATE INDEX IF NOT EXISTS indicators_symbol_interval_timestamp_idx
    ON indicators (symbol, interval, timestamp DESC);

-- ---------------------------------------------------------------------------
-- decisions
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS decisions (
    id               SERIAL PRIMARY KEY,
    symbol           VARCHAR(20)                NOT NULL,
    action           decision_action            NOT NULL,
    confidence       INTEGER                    NOT NULL CHECK (confidence BETWEEN 0 AND 100),
    reasoning        TEXT,
    execution_status decision_execution_status  NOT NULL DEFAULT 'pending',
    rejection_reason VARCHAR(100),
    composite_score  NUMERIC(10,4),
    is_dry_run       BOOLEAN                    NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ                NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS decisions_symbol_created_at_idx
    ON decisions (symbol, created_at DESC);

-- Drift correction: ensure reasoning column exists on pre-existing DBs
ALTER TABLE decisions ADD COLUMN IF NOT EXISTS reasoning TEXT;

-- ---------------------------------------------------------------------------
-- positions
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS positions (
    id                     SERIAL PRIMARY KEY,
    symbol                 VARCHAR(20)           NOT NULL,
    side                   position_side         NOT NULL,
    entry_price            NUMERIC(24,8)         NOT NULL,
    exit_price             NUMERIC(24,8),
    quantity               NUMERIC(24,8)         NOT NULL,
    take_profit_price      NUMERIC(24,8),
    stop_loss_price        NUMERIC(24,8),
    trailing_stop_distance NUMERIC(24,8),
    trailing_stop_price    NUMERIC(24,8),
    realized_pnl           NUMERIC(24,8),
    fee_total              NUMERIC(24,8)         NOT NULL DEFAULT 0,
    status                 position_status       NOT NULL DEFAULT 'open',
    close_reason           position_close_reason,
    opened_at              TIMESTAMPTZ           NOT NULL DEFAULT NOW(),
    closed_at              TIMESTAMPTZ,
    updated_at             TIMESTAMPTZ           NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS positions_status_idx ON positions (status);
CREATE INDEX IF NOT EXISTS positions_symbol_status_idx ON positions (symbol, status);

-- ---------------------------------------------------------------------------
-- orders
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS orders (
    id                SERIAL PRIMARY KEY,
    decision_id       INTEGER         REFERENCES decisions(id) ON DELETE SET NULL,
    exchange          order_exchange  NOT NULL,
    external_order_id VARCHAR(100),
    symbol            VARCHAR(20)     NOT NULL,
    side              order_side      NOT NULL,
    quantity          NUMERIC(24,8)   NOT NULL,
    price             NUMERIC(24,8),
    filled_quantity   NUMERIC(24,8)   NOT NULL DEFAULT 0,
    filled_price      NUMERIC(24,8),
    fee               NUMERIC(24,8)   NOT NULL DEFAULT 0,
    fee_asset         VARCHAR(20),
    status            order_status    NOT NULL DEFAULT 'pending',
    created_at        TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS orders_symbol_created_at_idx ON orders (symbol, created_at DESC);

-- ---------------------------------------------------------------------------
-- portfolio_snapshots
-- ---------------------------------------------------------------------------

CREATE TABLE IF NOT EXISTS portfolio_snapshots (
    id                       SERIAL PRIMARY KEY,
    balance                  NUMERIC(24,8) NOT NULL,
    peak_balance             NUMERIC(24,8) NOT NULL,
    drawdown_percent         NUMERIC(10,4) NOT NULL,
    total_pnl                NUMERIC(24,8) NOT NULL,
    win_count                INTEGER       NOT NULL DEFAULT 0,
    loss_count               INTEGER       NOT NULL DEFAULT 0,
    total_trades             INTEGER       NOT NULL DEFAULT 0,
    win_rate                 NUMERIC(10,4) NOT NULL DEFAULT 0,
    profit_factor            NUMERIC(10,4) NOT NULL DEFAULT 0,
    average_win              NUMERIC(24,8) NOT NULL DEFAULT 0,
    average_loss             NUMERIC(24,8) NOT NULL DEFAULT 0,
    sharpe_ratio             NUMERIC(10,4) NOT NULL DEFAULT 0,
    max_drawdown_percent     NUMERIC(10,4) NOT NULL DEFAULT 0,
    is_circuit_breaker_active BOOLEAN      NOT NULL DEFAULT FALSE,
    snapshot_at              TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);
