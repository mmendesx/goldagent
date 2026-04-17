# Gold Agent

A real-time crypto trading agent that ingests live market data from Binance, computes technical indicators, and asks an LLM (OpenAI `gpt-5.4-nano`) to decide whether to place a book order. Streams everything to a live React dashboard. Designed to run safely in dry-run mode by default — no real orders until you explicitly flip the switch.

---

## What it does

- Streams real-time candlestick data from Binance WebSocket for all configured symbols simultaneously
- Computes RSI, MACD, Bollinger Bands, EMAs (9/21/50/200), VWAP, and ATR on every closed candle via `pandas-ta`
- Optionally ingests Polymarket prediction market prices as a correlated sentiment signal
- Sends a structured market snapshot (candles + indicators + positions + portfolio state) to `gpt-5.4-nano` on every candle close
- The LLM returns a BUY/SELL/HOLD decision with confidence score, reasoning, and suggested entry/TP/SL prices
- Applies rule-based risk gates (confidence threshold, max open positions, max drawdown) before acting
- Executes live limit orders via `binance-connector-python` (Binance) or `py-clob-client` (Polymarket) — or logs decisions without placing orders in dry-run mode
- Monitors open positions for take-profit, stop-loss, and trailing-stop triggers
- Tracks portfolio metrics (balance, drawdown, win rate, Sharpe ratio) and trips a circuit breaker if max drawdown is exceeded
- Serves all data over a REST API and streams live events to a React dashboard over WebSocket

---

## Architecture overview

```
Binance WS Stream (binance-connector-python)
       |
       v
  CandleAggregator ──────────────────────────────────────────┐
       |                                                       |
       v                                                       |
  IndicatorComputer (pandas-ta)                               |
  RSI / MACD / Bollinger / EMA / VWAP / ATR                  |
       |                                                       |
       v                                                       |
  ContextBuilder                                              |
  (candles + indicators + positions + portfolio + Polymarket) |
       |                                                       |
       v                                                       |
  LLMDecisionEngine ──── OpenAI gpt-5.4-nano                 |
       |                                                       |
       v                                                       |
  RiskGate (confidence / max-positions / drawdown)            |
       |                                                       v
       v                                              WebSocketHub (FastAPI)
  Executor                                                     |
  (BinanceRestClient / PolymarketRestClient)                   v
       |                                          Dashboard (React + Lightweight Charts)
       v
  PositionMonitor (TP / SL / trailing stop)
       |
       v
  PortfolioManager ──── Postgres (candles, indicators, decisions, positions, orders, metrics)
                   └──── Redis (ticker price cache)

REST API (/api/v1/*) ──── FastAPI + asyncpg
```

**Dry-run mode**: the LLM runs on every candle close and decisions are persisted, but no orders are placed and the position monitor is not started. Full observability, zero real-money risk.

---

## Tech stack

**Backend** (`gold-agent/`)
- Python 3.12+
- FastAPI + Uvicorn — async HTTP server and WebSocket hub
- `asyncio.TaskGroup` — structured concurrency
- `binance-connector-python` — Binance market data WebSocket + REST order API
- `py-clob-client` — Polymarket CLOB WebSocket + REST order API
- `openai` — LLM decision engine (gpt-5.4-nano)
- `pandas` + `pandas-ta` — indicator computation
- `asyncpg` — async PostgreSQL driver
- `redis.asyncio` — async Redis client
- `pydantic v2` + `pydantic-settings` — domain models and config

**Frontend** (`gold-dashboard/`)
- React 19
- Vite
- TypeScript (ES2022 strict)
- Zustand — global state
- TradingView `lightweight-charts` v5 — candlestick chart
- `react-router-dom` v7

**Infrastructure**
- PostgreSQL 16
- Redis 7

---

## Project structure

```
goldagent/
├── docker-compose.yml          # Postgres + Redis + gold-agent + dashboard
├── .env.example                # All environment variables with defaults
├── gold-agent/                 # Python trading backend
│   ├── main.py                 # Entry point — wires all components via asyncio.TaskGroup
│   ├── config.py               # pydantic-settings config from environment
│   ├── requirements.txt
│   ├── Dockerfile
│   ├── domain/types.py         # Pydantic domain models (Candle, Position, Decision, ...)
│   ├── exchange/
│   │   ├── binance_stream.py   # Binance WebSocket kline/ticker stream
│   │   ├── binance_rest.py     # Binance REST — balance query + limit order placement
│   │   ├── polymarket_stream.py # Polymarket WebSocket price feed
│   │   └── polymarket_rest.py  # Polymarket REST — balance + order placement
│   ├── market/aggregator.py    # Candle fan-out: Postgres + Redis + decision pipeline
│   ├── analysis/indicators.py  # pandas-ta indicator computation
│   ├── engine/
│   │   ├── context_builder.py  # Assembles LLM context payload
│   │   ├── prompts.py          # System prompt + build_messages()
│   │   ├── llm_engine.py       # OpenAI API call + JSON response parsing
│   │   └── risk.py             # Rule-based risk gates
│   ├── execution/
│   │   ├── executor.py         # Order routing (Binance vs Polymarket)
│   │   ├── position_monitor.py # TP / SL / trailing stop loop
│   │   └── portfolio_manager.py # Metrics snapshot + circuit breaker
│   ├── storage/
│   │   ├── postgres.py         # asyncpg pool + all repository functions
│   │   └── redis_client.py     # Ticker price cache
│   └── api/
│       ├── websocket_hub.py    # FastAPI WebSocket connection manager
│       ├── handlers.py         # REST endpoint handlers
│       └── router.py           # FastAPI app factory + CORS
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
| Python | 3.12+ | `python3 --version` |
| Node | 22+ | `node --version` |
| Docker + Docker Compose | any recent | for Postgres and Redis |
| Binance API key + secret | — | optional; required for live trading |
| OpenAI API key | — | required; used by the LLM decision engine |
| Polymarket API credentials | — | optional; enables Polymarket order placement |

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
# Edit .env — set GOLD_DATABASE_URL, GOLD_REDIS_URL, OPENAI_API_KEY, and Binance credentials
```

**3. Start infrastructure**
```bash
docker compose up -d postgres redis
```

**4. Run database migrations**

The schema is managed via the SQL migration files in the original repo history. Apply them manually once against your Postgres instance:

```bash
# Example using psql — adapt the connection string to your .env
psql postgres://gold:gold@localhost:5432/gold < /path/to/migrations.sql
```

Or start the full stack with Docker Compose, which brings up a fresh Postgres with the correct default credentials; then connect and apply any schema you need.

**5. Install Python dependencies**
```bash
cd gold-agent
pip install -r requirements.txt
```

**6. Start the backend**
```bash
cd gold-agent
python main.py
```

The server starts on `http://localhost:8080`. Structured logs go to stdout.

**7. Start the dashboard**
```bash
cd gold-dashboard
npm install
npm run dev
```

Open http://localhost:3000.

---

## Configuration reference

All configuration is read from environment variables. The `.env` file is loaded automatically if present; variables set in the shell take precedence.

**Required**

| Variable | Description |
|----------|-------------|
| `GOLD_DATABASE_URL` | PostgreSQL connection string |
| `GOLD_REDIS_URL` | Redis connection string |
| `BINANCE_API_KEY` | Binance API key |
| `BINANCE_API_SECRET` | Binance API secret |
| `BINANCE_REST_API_URL` | Binance REST base URL |
| `BINANCE_WEBSOCKET_STREAM_URL` | Binance market data WebSocket URL |
| `GOLD_SYMBOLS` | Comma-separated trading symbols |

**LLM**

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENAI_API_KEY` | _(empty)_ | OpenAI API key; required for the decision engine |
| `GOLD_LLM_MODEL` | `gpt-5.4-nano` | OpenAI model used for trading decisions |
| `GOLD_LLM_CONTEXT_CANDLES` | `50` | Number of recent candles included in each LLM prompt |

**Exchange**

| Variable | Default | Description |
|----------|---------|-------------|
| `BINANCE_WEBSOCKET_API_URL` | _(empty)_ | Binance order WebSocket API URL |
| `POLYMARKET_API_KEY` | _(empty)_ | Polymarket API key; optional |
| `POLYMARKET_API_SECRET` | _(empty)_ | Polymarket API secret |
| `POLYMARKET_API_PASSPHRASE` | _(empty)_ | Polymarket API passphrase |
| `POLYMARKET_PRIVATE_KEY` | _(empty)_ | Hex private key for on-chain order signing |
| `POLYMARKET_WALLET_ADDRESS` | _(empty)_ | Wallet address for Polymarket CLOB auth |

**Trading**

| Variable | Default | Description |
|----------|---------|-------------|
| `GOLD_DEFAULT_INTERVAL` | `5m` | Candlestick interval (1m, 5m, 15m, 1h, 4h) |
| `GOLD_CONFIDENCE_THRESHOLD` | `70` | Minimum LLM confidence (0–100) required to act |
| `GOLD_MAX_POSITIONS` | `3` | Maximum concurrent open positions |
| `GOLD_MAX_POSITION_SIZE_PERCENT` | `10` | Max position size as % of portfolio balance |
| `GOLD_MAX_DRAWDOWN_PERCENT` | `15` | Drawdown % at which the circuit breaker activates |
| `GOLD_DRY_RUN` | `false` | `true` = log decisions only, no real orders placed |

**Infrastructure**

| Variable | Default | Description |
|----------|---------|-------------|
| `GOLD_HTTP_PORT` | `8080` | Port the HTTP + WebSocket server listens on |

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
| gold-agent | 8080 | REST API + WebSocket |
| Dashboard | 3000 | React app |

The backend waits for healthy Postgres and Redis before starting (Docker Compose health checks).

Pass credentials via a `.env` file in the project root:
```bash
cp .env.example .env
# Fill in OPENAI_API_KEY, BINANCE_API_KEY, BINANCE_API_SECRET
docker compose up
```

---

## Operating modes

### Dry-run (safe)

```
GOLD_DRY_RUN=true
```

The LLM runs on every candle close and all decisions are persisted. No orders are placed and the position monitor is not started. Use this mode for evaluation and observation before going live. All decisions are tagged `isDryRun: true` in the database and on the dashboard.

### Live trading

```
GOLD_DRY_RUN=false
BINANCE_API_KEY=<your key>
BINANCE_API_SECRET=<your secret>
OPENAI_API_KEY=<your key>
```

Limit orders are placed via the Binance REST API. Real money is at risk. The position monitor checks all open positions every 5 seconds and closes them when take-profit, stop-loss, or trailing-stop conditions are met.

### Circuit breaker

When portfolio drawdown reaches `GOLD_MAX_DRAWDOWN_PERCENT`, the circuit breaker activates automatically. While active, the LLM still runs but the risk gate rejects all non-HOLD decisions — no new orders are placed. The `isCircuitBreakerActive` field in the metrics response and the dashboard metrics bar reflect the current state.

---

## API reference

All REST endpoints are read-only. The server listens on `GOLD_HTTP_PORT` (default 8080).

### REST

| Method | Path | Query parameters | Description |
|--------|------|-----------------|-------------|
| `GET` | `/api/v1/candles` | `symbol`, `interval`, `limit` (default 100) | Recent candles in ascending time order |
| `GET` | `/api/v1/positions` | `symbol`, `status` (open/closed) | Positions with optional filters |
| `GET` | `/api/v1/trades` | `symbol`, `limit` (default 50) | Closed position history |
| `GET` | `/api/v1/decisions` | `symbol`, `limit` (default 50) | LLM decision log, newest first |
| `GET` | `/api/v1/metrics` | — | Latest portfolio snapshot |
| `GET` | `/api/v1/exchange/balances` | — | Live Binance USDT and Polymarket USDC balances |

All responses use camelCase JSON keys matching the TypeScript types in `gold-dashboard/src/types/index.ts`.

### WebSocket

```
WS /ws/v1/stream
```

Connect to receive a real-time stream of all events. The connection is read-only — incoming messages are ignored.

**Outbound event types**:

| `type` | `payload` | Description |
|--------|-----------|-------------|
| `candle_update` | `Candle` | Emitted on every candle tick (open and closed) |
| `position_update` | `Position` | Emitted when a position is modified |
| `position_closed` | `Position` | Emitted when a position closes |
| `metric_update` | `PortfolioMetrics` | Emitted after each portfolio snapshot |
| `trade_executed` | `Position` | Emitted when a live order is filled |
| `decision_made` | `Decision` | Emitted after every LLM evaluation |

All messages share the envelope:
```json
{
  "type": "<event_type>",
  "payload": { ... }
}
```

---

## Development workflow

- Task specs and BDD scenarios live in `tasks/`. Read `spec-2` and `prd-spec-2.md` to understand the design intent behind every backend component.
- The backend uses Python's `logging` module with structured context fields. Every decision, component start/stop, and error logs with context. In production, pipe stdout to your log aggregator.
- All domain models are Pydantic v2. API-facing models use a camelCase `alias_generator` so `.model_dump(by_alias=True)` produces dashboard-compatible JSON automatically.
- The LLM system prompt lives in `gold-agent/engine/prompts.py`. Tune it there — no other files need to change for prompt adjustments.

---

## Safety and disclaimers

This software is provided for educational and research purposes. It is not financial advice.

- **Cryptocurrency trading carries substantial risk of loss.** Markets are volatile and unpredictable. Past performance of any strategy does not guarantee future results.
- **LLM-based trading decisions are non-deterministic.** The model may produce unexpected outputs. Always review the decision log before enabling live trading.
- **Run in dry-run mode until you fully understand every component.** Review the system prompt, the risk gate configuration, and the position sizing logic before enabling live trading.
- **You are responsible for your own funds.** The authors provide no warranty and no support guarantees. Use at your own risk.
- **Protect your API keys.** Never commit them. Use environment variables or a secrets manager. Restrict Binance keys to spot trading only and disable withdrawal permissions.

---

## License

MIT License. See LICENSE file.

---

## References

- [Binance Spot API docs](https://github.com/binance/binance-spot-api-docs)
- [binance-connector-python](https://github.com/binance/binance-connector-python)
- [py-clob-client](https://github.com/Polymarket/py-clob-client)
- [Polymarket real-time data client](https://github.com/Polymarket/real-time-data-client)
- [OpenAI API reference](https://platform.openai.com/docs/api-reference)
- [TradingView Lightweight Charts](https://tradingview.github.io/lightweight-charts/)
