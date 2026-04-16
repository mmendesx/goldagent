CREATE TABLE portfolio_snapshots (
    id BIGSERIAL PRIMARY KEY,
    balance NUMERIC(20, 8) NOT NULL,
    peak_balance NUMERIC(20, 8) NOT NULL,
    drawdown_percent NUMERIC(10, 4) NOT NULL DEFAULT 0,
    total_pnl NUMERIC(20, 8) NOT NULL DEFAULT 0,
    win_count INTEGER NOT NULL DEFAULT 0,
    loss_count INTEGER NOT NULL DEFAULT 0,
    total_trades INTEGER NOT NULL DEFAULT 0,
    win_rate NUMERIC(10, 4) NOT NULL DEFAULT 0,
    profit_factor NUMERIC(10, 4),
    average_win NUMERIC(20, 8),
    average_loss NUMERIC(20, 8),
    sharpe_ratio NUMERIC(10, 4),
    max_drawdown_percent NUMERIC(10, 4) NOT NULL DEFAULT 0,
    is_circuit_breaker_active BOOLEAN NOT NULL DEFAULT false,
    snapshot_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_portfolio_snapshots_snapshot_at ON portfolio_snapshots (snapshot_at DESC);
