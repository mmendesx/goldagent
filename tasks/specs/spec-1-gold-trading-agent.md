# Spec-1: Gold Trading Agent

**Feature**: Autonomous trading agent for Binance spot day-trading and Polymarket event markets, with a real-time dashboard.

**Status**: Draft
**Date**: 2026-04-16

---

## 1. Problem Statement

Manual crypto day-trading is emotionally biased, slow to react, and impossible to sustain 24/7. Polymarket event-driven markets add another dimension that humans struggle to correlate with price action in real time.

Gold is an autonomous trading agent that:
- Ingests real-time market data via WebSocket (zero polling).
- Applies technical indicators, pattern recognition, and news sentiment analysis to generate trading signals.
- Executes trades on Binance (spot) and Polymarket with full position management (TP/SL/trailing stop).
- Persists every decision, trade, and metric for auditability.
- Exposes a real-time dashboard for monitoring performance, positions, and price charts.

---

## 2. Functional Requirements

### FR-1: Real-Time Market Data Ingestion

| ID | Requirement |
|----|-------------|
| FR-1.1 | Connect to Binance WebSocket Streams (`wss://stream.binance.com:9443`) for kline, trade, and ticker data. |
| FR-1.2 | Support configurable symbols: BTCUSDT, ETHUSDT, SOLUSDT, BNBUSDT (extensible). |
| FR-1.3 | Support configurable kline intervals: 1m, 5m, 15m, 1h, 4h, 1D. Default focus: 5m. |
| FR-1.4 | Connect to Polymarket real-time data client WebSocket for activity (trades) and crypto price streams. |
| FR-1.5 | Maintain persistent WebSocket connections with automatic reconnection on disconnect (exponential backoff, max 60s). |
| FR-1.6 | Respond to Binance ping frames with pong frames to keep connections alive. |
| FR-1.7 | Cache latest candle, ticker, and orderbook snapshot in Redis for sub-millisecond reads by the decision engine. |

### FR-2: Technical Analysis Engine

| ID | Requirement |
|----|-------------|
| FR-2.1 | Compute indicators in real time as new candles close: RSI (14), MACD (12,26,9), Bollinger Bands (20,2), EMA (9,21,50,200), VWAP, ATR (14). |
| FR-2.2 | Detect candlestick patterns: engulfing, doji, hammer, shooting star, morning/evening star. |
| FR-2.3 | Detect chart patterns: support/resistance levels, trend lines, breakouts, double top/bottom. |
| FR-2.4 | Store computed indicators alongside raw candle data in Postgres for backtesting. |
| FR-2.5 | Allow indicator parameters to be configurable via environment variables or config file. |

### FR-3: News and Sentiment Analysis

| ID | Requirement |
|----|-------------|
| FR-3.1 | Ingest news from configurable RSS/API sources (CryptoPanic, CoinGecko news, general financial feeds). |
| FR-3.2 | Score sentiment per asset using an LLM or pre-trained NLP model (positive/negative/neutral with confidence). |
| FR-3.3 | Weight sentiment signals into the decision engine with configurable influence (0.0-1.0 multiplier). |
| FR-3.4 | Store raw news items and computed sentiment scores in Postgres with timestamps. |

### FR-4: Decision Engine

| ID | Requirement |
|----|-------------|
| FR-4.1 | Combine technical signals, pattern detections, and sentiment scores into a composite trading signal per symbol. |
| FR-4.2 | Generate BUY, SELL, or HOLD decisions with a confidence score (0-100). |
| FR-4.3 | Enforce a minimum confidence threshold (configurable, default 70) before executing a trade. |
| FR-4.4 | Respect position limits: max concurrent open positions (configurable, default 3), max position size as percentage of balance (configurable, default 10%). |
| FR-4.5 | Log every decision (action, confidence, contributing signals, timestamp) to Postgres regardless of whether a trade is executed. |
| FR-4.6 | Support a dry-run mode that logs decisions without executing trades. |

### FR-5: Trade Execution

| ID | Requirement |
|----|-------------|
| FR-5.1 | Execute spot market orders on Binance via WebSocket API (`wss://ws-api.binance.com:443/ws-api/v3`) with HMAC-SHA256 signed requests. |
| FR-5.2 | Place orders on Polymarket using CLOB API with API key authentication. |
| FR-5.3 | Set take-profit (TP) and stop-loss (SL) levels automatically based on ATR or configurable fixed percentages. |
| FR-5.4 | Implement trailing stop: move SL up as price moves favorably, with configurable trail distance (default: 1x ATR). |
| FR-5.5 | Monitor open positions in real time and trigger TP/SL/trailing stop exits automatically. |
| FR-5.6 | Handle partial fills and order rejections gracefully with retry logic. |
| FR-5.7 | Record every order (placed, filled, cancelled, rejected) with full details in Postgres. |

### FR-6: Portfolio and Risk Management

| ID | Requirement |
|----|-------------|
| FR-6.1 | Track current balance, peak balance, and drawdown in real time. |
| FR-6.2 | Enforce a max drawdown circuit breaker (configurable, default 15% from peak). When triggered, halt all new trades and alert. |
| FR-6.3 | Calculate and store: win rate, profit factor, average win/loss, Sharpe ratio, total P&L. |
| FR-6.4 | Expose portfolio metrics via REST API endpoint for dashboard consumption. |

### FR-7: Data Persistence

| ID | Requirement |
|----|-------------|
| FR-7.1 | Use PostgreSQL for all persistent data: candles, indicators, decisions, trades, portfolio snapshots, news/sentiment. |
| FR-7.2 | Use Redis for hot data: latest candle per symbol/interval, current positions, real-time metrics, WebSocket subscription state. |
| FR-7.3 | Implement database migrations with versioning (golang-migrate or similar). |
| FR-7.4 | Partition candle data by symbol and time range for query performance. |

### FR-8: REST API (Backend-to-Dashboard)

| ID | Requirement |
|----|-------------|
| FR-8.1 | `GET /api/v1/candles?symbol=X&interval=Y&from=T1&to=T2` — Historical candle data with indicators. |
| FR-8.2 | `GET /api/v1/positions` — All open positions with current unrealized P&L. |
| FR-8.3 | `GET /api/v1/positions/history` — Closed positions with realized P&L. |
| FR-8.4 | `GET /api/v1/trades` — Trade execution log with filters. |
| FR-8.5 | `GET /api/v1/metrics` — Portfolio metrics (balance, drawdown, win rate, etc.). |
| FR-8.6 | `GET /api/v1/decisions` — Decision log with confidence scores and contributing signals. |
| FR-8.7 | `WebSocket /ws/v1/stream` — Real-time push of candles, ticks, position updates, metrics to dashboard. |

### FR-9: Dashboard (Frontend)

| ID | Requirement |
|----|-------------|
| FR-9.1 | Metrics bar at the top: Balance, Peak Balance, Drawdown (color-coded green/yellow/red by severity), Win Rate, Total Trades, Open Positions count. All values update in real time via WebSocket. |
| FR-9.2 | Price chart using TradingView Lightweight Charts (`lightweight-charts` npm package): candlestick series with volume. |
| FR-9.3 | Symbol selector dropdown for configured symbols (BTCUSDT, ETHUSDT, SOLUSDT, BNBUSDT). |
| FR-9.4 | Interval buttons: 1m, 5m, 15m, 1h, 4h, 1D. Switching loads historical data and subscribes to new real-time stream. |
| FR-9.5 | Trade markers on chart: `SHORT` (red down arrow) on entry, `TAKE_PROFIT` (green up arrow), `STOP_LOSS` (red circle), `TRAILING_STOP` (red circle) on exit, `OPEN` (yellow down arrow) for live positions. |
| FR-9.6 | Open Positions panel: table with Symbol, Side, Entry Price, Current Price, Unrealized P&L (live, color-coded), SL level, TP level. |
| FR-9.7 | Trade History panel: table with all closed trades, sortable/filterable by symbol, date, P&L. |
| FR-9.8 | Decision Log panel: table showing agent decisions with confidence, signals, and outcome. |
| FR-9.9 | Tab navigation to switch between: Chart, Open Positions, Trade History, Decision Log. |
| FR-9.10 | Responsive layout. Dark theme by default. |

---

## 3. Non-Functional Requirements

| ID | Requirement |
|----|-------------|
| NFR-1 | Latency: candle/tick data must reach the decision engine within 50ms of WebSocket receipt. |
| NFR-2 | Latency: trade execution order must be sent within 100ms of decision. |
| NFR-3 | Uptime: auto-restart on crash. Systemd/Docker health checks. |
| NFR-4 | Observability: structured JSON logging with correlation IDs per decision cycle. |
| NFR-5 | Security: API keys stored in environment variables, never logged or persisted in DB. TLS for all external connections. |
| NFR-6 | Testability: decision engine must be pure-functional (given inputs, deterministic output) for unit testing. |
| NFR-7 | Configuration: all thresholds, indicator parameters, and feature flags via `.env` or config file. No hardcoded magic numbers. |
| NFR-8 | Deployment: Docker Compose for local dev (Go backend, Postgres, Redis, frontend dev server). |

---

## 4. Technical Architecture

### 4.1 Backend (Go)

```
gold-backend/
  cmd/
    gold/              # Main entrypoint
  internal/
    config/            # Configuration loading
    exchange/
      binance/         # Binance WebSocket + REST client
      polymarket/      # Polymarket CLOB + real-time client
    market/
      candle/          # Candle aggregation and storage
      ticker/          # Ticker processing
    analysis/
      indicator/       # RSI, MACD, BB, EMA, VWAP, ATR
      pattern/         # Candlestick and chart pattern detection
      sentiment/       # News ingestion and sentiment scoring
    engine/            # Decision engine — combines all signals
    execution/         # Order placement, position monitoring, TP/SL
    portfolio/         # Balance tracking, risk management, metrics
    storage/
      postgres/        # Repository implementations
      redis/           # Cache implementations
    api/
      rest/            # HTTP handlers (Gin or Chi)
      websocket/       # Dashboard WebSocket hub
  migrations/          # SQL migration files
  go.mod
  go.sum
```

**Key libraries**:
- `gorilla/websocket` — WebSocket client/server
- `jackc/pgx/v5` — PostgreSQL driver
- `redis/go-redis/v9` — Redis client
- `go-chi/chi` or `gin-gonic/gin` — HTTP router
- `shopspring/decimal` — Precise decimal arithmetic for financial calculations

### 4.2 Frontend (React + TypeScript)

```
gold-dashboard/
  src/
    api/               # REST + WebSocket client
    components/
      MetricsBar/      # Top metrics strip
      PriceChart/      # TradingView chart wrapper
      OpenPositions/   # Positions table
      TradeHistory/    # History table
      DecisionLog/     # Decisions table
      SymbolSelector/  # Symbol dropdown
      IntervalButtons/ # Timeframe switcher
    hooks/             # useWebSocket, useCandles, usePositions
    pages/
      Dashboard/       # Main layout with tabs
    types/             # Shared TypeScript interfaces
    utils/             # Formatters, color helpers
  index.html
  vite.config.ts
  tsconfig.json
  package.json
```

**Key libraries**:
- `lightweight-charts` — TradingView charting
- `animate-ui` + `react-bits` — UI components and animations
- `react-router` — Tab routing
- `zustand` or `jotai` — Lightweight state management

### 4.3 Data Flow

```
Binance WS ──────┐
                  ├──▶ Market Data Service ──▶ Redis (hot cache)
Polymarket WS ───┘          │                      │
                            ▼                      ▼
                    Analysis Engine ◀───── Indicator Computer
                            │
                            ▼
                    Decision Engine ──▶ Postgres (decisions)
                            │
                            ▼
                    Execution Engine ──▶ Binance Order API
                            │              Polymarket CLOB
                            ▼
                    Portfolio Manager ──▶ Postgres (trades, metrics)
                            │
                            ▼
                    REST API + WS Hub ──▶ Dashboard
```

### 4.4 Infrastructure

| Component | Image/Runtime | Port |
|-----------|--------------|------|
| Gold Backend | Go binary | 8080 |
| PostgreSQL 16 | postgres:16-alpine | 5432 |
| Redis 7 | redis:7-alpine | 6379 |
| Dashboard | Vite dev / nginx | 3000 |

---

## 5. External Dependencies

| Dependency | Purpose | Auth |
|------------|---------|------|
| Binance WebSocket Streams | Real-time klines, trades, tickers | None (public) |
| Binance WebSocket API | Order placement | HMAC-SHA256 API key + secret |
| Polymarket Real-Time Client | Event market data | CLOB API key (for user-specific streams) |
| Polymarket CLOB API | Order placement | API key + secret + passphrase |
| News APIs (CryptoPanic, etc.) | Sentiment data | API key per provider |
| LLM API (optional) | Advanced sentiment analysis | API key |

---

## 6. Constraints

- **Real-time means real-time**: No HTTP polling for price data. WebSocket only.
- **Financial precision**: All monetary values use `decimal` types, never floating point.
- **Rate limits**: Binance allows max 300 WS connections per 5 min per IP, max 5 incoming messages/sec per connection, max 1024 streams per connection. Code must respect these limits.
- **Go conventions**: Full, descriptive names (no abbreviations). SOLID principles. KISS.
- **TypeScript conventions**: ES2022 target. Strict mode. No `any` types.

---

## 7. Open Questions (Non-Blocking)

| # | Question | Default if unanswered |
|---|----------|-----------------------|
| 1 | Which Polymarket markets to trade? | Top 5 by volume in crypto category |
| 2 | Preferred news API provider? | CryptoPanic (free tier) |
| 3 | LLM for sentiment analysis? | Claude API via Anthropic SDK |
| 4 | Initial balance allocation split between Binance and Polymarket? | 80% Binance / 20% Polymarket |
| 5 | Dashboard authentication? | None for v1 (local network only) |
