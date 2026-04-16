# Gold Agent

A real-time crypto trading agent that ingests live market data from Binance, computes technical indicators, detects chart patterns, weighs sentiment, makes autonomous trading decisions, and streams everything to a live dashboard. Designed to run safely in dry-run mode by default — no real orders until you explicitly flip the switch.

---

## What it does

- Streams real-time candlestick data from Binance WebSocket for up to N configured symbols simultaneously
- Computes RSI, MACD, Bollinger Bands, EMAs (9/21/50/200), VWAP, and ATR on every closed candle
- Detects candlestick and chart patterns; optionally incorporates news sentiment via CryptoPanic + Anthropic
- Runs a decision engine that weighs all signals into a composite score and emits BUY/SELL/HOLD decisions
- Executes live orders via the Binance WebSocket Order API — or logs decisions without placing orders in dry-run mode
- Monitors open positions for take-profit, stop-loss, and trailing-stop triggers
- Tracks portfolio metrics (balance, drawdown, win rate, Sharpe ratio) and trips a circuit breaker if max drawdown is exceeded
- Serves all data over a REST API and streams live events to a React dashboard over WebSocket

---

## Architecture overview

```
Binance WS Stream
       |
       v
  Candle Aggregator ──────────────────────────────────────────────┐
       |                                                           |
       ├──> Indicator Computer (RSI, MACD, BB, EMA, VWAP, ATR)   |
       |                                                           |
       └──> Decision Orchestrator                                  |
                 |                                                 |
                 ├── Candlestick Detector                         |
                 ├── Chart Pattern Analyzer                       |
                 └── Sentiment Coordinator (optional)             |
                           |                                       |
                           v                                       |
                   Decision Engine ──── Postgres (decisions)       |
                           |                                       |
                           v (TradeIntent)                         v
                       Executor ──── Binance WS Order API    WebSocket Hub
                           |                                       |
                           v                                       v
                   Position Monitor              Dashboard (React + Lightweight Charts)
                           |
                           v
                   Portfolio Manager ──── Postgres (positions, orders, metrics)
                                    └──── Redis (ticker prices, circuit breaker)

REST API (/api/v1/*) ──── Postgres read-only
```

**Dry-run mode**: the Decision Engine makes all decisions and persists them, but no `TradeIntent` is ever emitted. The Executor and Position Monitor are not started. Everything else runs normally — you get full observability without real money at risk.

---

## Tech stack

**Backend** (`gold-backend`)
- Go 1.23+
- PostgreSQL 16
- Redis 7
- `chi` v5 — HTTP router
- `gorilla/websocket` — WebSocket transport
- `pgx` v5 — PostgreSQL driver
- `shopspring/decimal` — exact decimal arithmetic for prices and quantities

**Frontend** (`gold-dashboard`)
- React 19
- Vite
- TypeScript (ES2022 strict)
- Zustand — global state
- TradingView `lightweight-charts` v5 — candlestick chart
- `react-router-dom` v7

---

## Project structure

```
goldagent/
├── docker-compose.yml          # Postgres + Redis + backend + dashboard
├── .env.example                # All environment variables with defaults
├── gold-backend/
│   ├── cmd/gold/
│   │   └── main.go             # Entry point — wires all components together
│   ├── internal/
│   │   ├── config/             # Environment variable loading and validation
│   │   ├── domain/             # Shared types (Candle, Position, Decision, Order, ...)
│   │   ├── analysis/
│   │   │   ├── indicator/      # RSI, MACD, Bollinger, EMA, VWAP, ATR computation
│   │   │   ├── pattern/
│   │   │   │   ├── candlestick/  # Candlestick pattern detector
│   │   │   │   └── chart/        # Chart pattern analyzer (S/R, trend)
│   │   │   └── sentiment/      # CryptoPanic news fetcher + Anthropic scorer
│   │   ├── api/
│   │   │   ├── rest/           # chi router, all GET handlers, response helpers
│   │   │   └── websocket/      # Hub, per-client goroutines, outbound message types
│   │   ├── engine/             # Decision engine — weights signals, persists decisions
│   │   ├── exchange/
│   │   │   ├── binance/        # Stream client (kline/ticker WS) + Order API client
│   │   │   └── polymarket/     # Realtime WebSocket client
│   │   ├── execution/          # Executor, BinanceOrderClient, PositionMonitor
│   │   ├── market/candle/      # Candle aggregator — buffers partial candles
│   │   ├── portfolio/          # Portfolio manager — balance, drawdown, circuit breaker
│   │   └── storage/
│   │       ├── postgres/       # Repository implementations for every domain entity
│   │       └── redis/          # Ticker price cache, circuit breaker state
│   └── migrations/             # golang-migrate SQL files (7 pairs up/down)
└── gold-dashboard/
    ├── src/
    │   ├── api/                # REST client, WebSocket client
    │   ├── components/         # DecisionLog, MetricsBar, OpenPositions, PriceChart,
    │   │                       # SymbolSelector, IntervalButtons, TradeHistory
    │   ├── hooks/              # useWebSocketLifecycle
    │   ├── pages/Dashboard/    # Main dashboard page
    │   ├── store/              # Zustand store
    │   └── types/              # Shared TypeScript types
    └── vite.config.ts
```

---

## Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.23+ | `go version` |
| Node | 22+ | `node --version` |
| Docker + Docker Compose | any recent | for Postgres and Redis |
| golang-migrate CLI | latest | for running migrations |
| Binance API key + secret | — | optional; required only for live trading |
| CryptoPanic API key | — | optional; enables news fetching |
| Anthropic API key | — | optional; enables sentiment scoring via Claude |

Install golang-migrate:
```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

---

## Setup (local dev)

**1. Clone the repo**
```bash
git clone <repo-url>
cd goldagent
```

**2. Configure environment**
```bash
cp .env.example .env
# Edit .env — at minimum set GOLD_DATABASE_URL and GOLD_REDIS_URL
# (the defaults in .env.example match the Docker Compose service names for local dev)
```

**3. Start infrastructure**
```bash
docker compose up -d postgres redis
```

**4. Run migrations**
```bash
migrate -path gold-backend/migrations -database "$GOLD_DATABASE_URL" up
```

The Docker setup does not auto-run migrations. Run them manually after any new deployment.

**5. Start the backend**
```bash
cd gold-backend
go run ./cmd/gold
```

The server starts on `http://localhost:8080`. Structured JSON logs go to stdout.

**6. Start the dashboard**
```bash
cd gold-dashboard
npm install
npm run dev
```

Open http://localhost:3000.

---

## Configuration reference

All configuration is read from environment variables. The `.env` file is loaded automatically if present; environment variables already set in the shell take precedence.

**Required**

| Variable | Description |
|----------|-------------|
| `GOLD_DATABASE_URL` | PostgreSQL connection string |
| `GOLD_REDIS_URL` | Redis connection string |

**Exchange**

| Variable | Default | Description |
|----------|---------|-------------|
| `BINANCE_API_KEY` | _(empty)_ | Binance API key; required for live trading |
| `BINANCE_API_SECRET` | _(empty)_ | Binance API secret; required for live trading |
| `BINANCE_WEBSOCKET_STREAM_URL` | `wss://stream.binance.com:9443` | Binance market data WebSocket |
| `BINANCE_WEBSOCKET_API_URL` | `wss://ws-api.binance.com:443/ws-api/v3` | Binance order WebSocket API |
| `POLYMARKET_API_KEY` | _(empty)_ | Polymarket API key; optional |
| `POLYMARKET_API_SECRET` | _(empty)_ | Polymarket API secret |
| `POLYMARKET_API_PASSPHRASE` | _(empty)_ | Polymarket API passphrase |

**Trading**

| Variable | Default | Description |
|----------|---------|-------------|
| `GOLD_SYMBOLS` | `BTCUSDT,ETHUSDT,SOLUSDT,BNBUSDT` | Comma-separated list of symbols to trade |
| `GOLD_DEFAULT_INTERVAL` | `5m` | Candlestick interval (1m, 5m, 15m, 1h, 4h, 1d) |
| `GOLD_CONFIDENCE_THRESHOLD` | `70` | Minimum composite score (0–100) to act on a decision |
| `GOLD_MAX_POSITIONS` | `3` | Maximum number of concurrent open positions |
| `GOLD_MAX_POSITION_SIZE_PERCENT` | `10` | Maximum size of each position as % of portfolio balance |
| `GOLD_MAX_DRAWDOWN_PERCENT` | `15` | Drawdown % at which the circuit breaker halts new trades |
| `GOLD_DRY_RUN` | `true` | `true` = log decisions only, no real orders placed |
| `GOLD_SENTIMENT_WEIGHT` | `0.3` | Weight of sentiment signal in composite score (0.0–1.0) |
| `GOLD_TRAILING_STOP_ATR_MULTIPLIER` | `1.0` | Trailing stop distance = ATR × this multiplier (0 = disabled) |

**Indicators**

| Variable | Default | Description |
|----------|---------|-------------|
| `GOLD_RSI_PERIOD` | `14` | RSI lookback period |
| `GOLD_MACD_FAST` | `12` | MACD fast EMA period |
| `GOLD_MACD_SLOW` | `26` | MACD slow EMA period |
| `GOLD_MACD_SIGNAL` | `9` | MACD signal line period |
| `GOLD_BOLLINGER_PERIOD` | `20` | Bollinger Bands SMA period |
| `GOLD_BOLLINGER_STDDEV` | `2` | Bollinger Bands standard deviation multiplier |
| `GOLD_EMA_PERIODS` | `9,21,50,200` | Comma-separated EMA periods to compute |
| `GOLD_ATR_PERIOD` | `14` | ATR lookback period |

**Infrastructure**

| Variable | Default | Description |
|----------|---------|-------------|
| `GOLD_HTTP_PORT` | `8080` | Port the HTTP server listens on |

**External APIs**

| Variable | Default | Description |
|----------|---------|-------------|
| `CRYPTOPANIC_API_KEY` | _(empty)_ | CryptoPanic key; enables news fetching for sentiment |
| `ANTHROPIC_API_KEY` | _(empty)_ | Anthropic key; enables Claude-based sentiment scoring |

**Frontend (Vite)**

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_API_BASE_URL` | `http://localhost:8080` | Backend base URL for REST calls |
| `VITE_WS_URL` | `ws://localhost:8080/ws/v1/stream` | Backend WebSocket URL |

---

## Running in Docker

Start everything — Postgres, Redis, backend, and dashboard — in one command:

```bash
docker compose up
```

| Service | Port | Notes |
|---------|------|-------|
| Postgres | 5432 | Username/password/database: `gold` |
| Redis | 6379 | No auth by default |
| Backend | 8080 | REST API + WebSocket |
| Dashboard | 3000 | React app served by Nginx |

The backend waits for healthy Postgres and Redis before starting (Docker Compose health checks). Migrations are **not** run automatically — run them manually before starting the backend for the first time:

```bash
migrate -path gold-backend/migrations \
  -database "postgres://gold:gold@localhost:5432/gold?sslmode=disable" up
```

Pass exchange credentials and trading parameters via environment variables or a `.env` file in the project root. Docker Compose forwards all relevant variables from the host environment.

---

## Operating modes

### Dry-run (default, safe)

```
GOLD_DRY_RUN=true
```

The decision engine runs fully — it streams data, computes indicators, detects patterns, scores signals, and persists every decision to the database. No `TradeIntent` is ever emitted. The order executor and position monitor are not started. No API calls are made to Binance's order endpoint.

Use this mode for evaluation, backtesting observation, and extended paper trading before going live. All decisions are tagged `isDryRun: true` in the database and on the dashboard.

### Live trading

```
GOLD_DRY_RUN=false
BINANCE_API_KEY=<your key>
BINANCE_API_SECRET=<your secret>
```

Real orders are placed via the Binance WebSocket Order API. Real money is at risk. The position monitor watches each open position and closes it when price hits the take-profit, stop-loss, or trailing-stop target.

### Circuit breaker

When portfolio drawdown reaches `GOLD_MAX_DRAWDOWN_PERCENT`, the circuit breaker activates automatically. While active, the decision engine continues to analyze and log decisions, but no new trade intents are emitted. The `isCircuitBreakerActive` field in the metrics response and the dashboard metrics bar reflect the current state.

---

## API reference

All endpoints are read-only. The server listens on `GOLD_HTTP_PORT` (default 8080).

### REST

| Method | Path | Query parameters | Description |
|--------|------|-----------------|-------------|
| `GET` | `/health` | — | Returns `{"status":"ok"}` |
| `GET` | `/api/v1/candles` | `symbol` (required), `interval` (required), `from` (RFC3339), `to` (RFC3339), `limit`, `offset` | Paginated candles with merged indicator values. Defaults to last 7 days when `from`/`to` are omitted. |
| `GET` | `/api/v1/positions` | — | All open positions enriched with live price from Redis and computed unrealized P&L |
| `GET` | `/api/v1/positions/history` | `limit`, `offset` | Paginated closed positions ordered by `closed_at` descending |
| `GET` | `/api/v1/trades` | `symbol` (optional), `limit`, `offset` | Paginated order history, optionally filtered by symbol |
| `GET` | `/api/v1/metrics` | — | Current in-memory portfolio snapshot (balance, drawdown, win rate, Sharpe, circuit breaker state) |
| `GET` | `/api/v1/decisions` | `symbol` (optional), `limit`, `offset` | Paginated decision log, optionally filtered by symbol |

**Pagination defaults**: `limit=100`, `offset=0`. Maximum `limit` is 1000.

**Paginated response shape**:
```json
{
  "items": [...],
  "limit": 100,
  "offset": 0,
  "count": 42,
  "hasMore": false
}
```

**Error response shape**:
```json
{
  "error": "human-readable message",
  "code": "MACHINE_READABLE_CODE"
}
```

### WebSocket

```
WS /ws/v1/stream
```

Connect and optionally send a subscribe message to filter by symbol:
```json
{
  "action": "subscribe",
  "symbols": ["BTCUSDT", "ETHUSDT"]
}
```

Omit `symbols` or send an empty array to receive all events.

**Outbound event types**:

| `type` | `payload` | Description |
|--------|-----------|-------------|
| `candle_update` | `domain.Candle` | Emitted on every closed candle |
| `position_update` | `domain.Position` | Emitted when a position is modified |
| `position_closed` | `domain.Position` | Emitted when a position closes (TP, SL, or trailing stop) |
| `metric_update` | `domain.PortfolioMetrics` | Emitted periodically with current portfolio state |
| `trade_executed` | `domain.Order` | Emitted when a real order is filled (live mode only) |
| `decision_made` | `domain.Decision` | Emitted on every decision produced by the engine |

All messages share the envelope:
```json
{
  "type": "<event_type>",
  "payload": { ... }
}
```

---

## Testing

**Backend**
```bash
cd gold-backend
go test ./...
```

Most tests run without any external services. Integration tests are gated behind environment variables — they are skipped unless the relevant variable is set:

| Environment variable | Effect |
|---------------------|--------|
| `GOLD_DATABASE_URL` | Enables repository integration tests against real Postgres |
| `REDIS_URL` | Enables cache integration tests against real Redis |
| `BINANCE_INTEGRATION_TEST=1` | Enables Binance WebSocket connectivity test |
| `POLYMARKET_INTEGRATION_TEST=1` | Enables Polymarket client connectivity test |
| `GOLD_INTEGRATION_TEST=1` | Enables full-pipeline integration test |

To run with full integration coverage:
```bash
GOLD_DATABASE_URL="postgres://gold:gold@localhost:5432/gold?sslmode=disable" \
REDIS_URL="redis://localhost:6379/0" \
go test ./...
```

**Frontend — confirm the build compiles**
```bash
cd gold-dashboard
npm run build
```

---

## Development workflow

- Task specs and BDD scenarios live in `tasks/`. Read `spec-1` and `prd-spec-1.md` to understand the design intent behind every component.
- Code conventions: full descriptive names throughout (no abbreviations), single-responsibility functions under 40 lines, SOLID, KISS. See the task spec for the full style guide.
- The backend uses structured JSON logging (`log/slog`). Every decision, component start/stop, and error includes context fields. In production, pipe stdout to your log aggregator.
- Schema changes must ship as a new migration file pair in `gold-backend/migrations/` following the existing `NNN_description.up.sql` / `NNN_description.down.sql` naming convention.

---

## Safety and disclaimers

This software is provided for educational and research purposes. It is not financial advice.

- **Cryptocurrency trading carries substantial risk of loss.** Markets are volatile and unpredictable. Past performance of any strategy does not guarantee future results.
- **Run in dry-run mode until you fully understand every component.** Read the source, understand the signal weights, review the position sizing logic, and confirm the circuit breaker is configured for your risk tolerance before enabling live trading.
- **You are responsible for your own funds.** The authors provide no warranty and no support guarantees. Use at your own risk.
- **Review your exchange fee structure.** Frequent short-interval trading generates significant fees that the current P&L accounting does not fully model.
- **Protect your API keys.** Never commit them. Use environment variables or a secrets manager. Restrict the key to the minimum permissions Binance requires (spot trading only; disable withdrawals).

---

## License

MIT License. See LICENSE file. You can change this later.

---

## References

- [Binance WebSocket API docs](https://github.com/binance/binance-spot-api-docs/blob/master/web-socket-api.md)
- [Binance WebSocket Stream docs](https://github.com/binance/binance-spot-api-docs/blob/master/web-socket-streams.md)
- [Polymarket real-time data client](https://github.com/Polymarket/real-time-data-client)
- [TradingView Lightweight Charts](https://tradingview.github.io/lightweight-charts/)
