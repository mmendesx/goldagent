# Spec: Python Backend Migration with LLM-Driven Trading Decisions

## Overview
Migrate the trading agent backend from Go to Python, replacing hand-coded signal scoring with an LLM-driven decision layer. The Python backend uses `py-clob-client` for Polymarket integration and `binance-connector-python` for Binance, and routes aggregated market data to an Anthropic Claude model that decides whether to place a book order.

## Actors
- **Binance Exchange** — Real-time kline/ticker streams and REST account API via `binance-connector-python`
- **Polymarket CLOB** — Order book streams and authenticated balance/order API via `py-clob-client`
- **Claude LLM (Anthropic)** — Receives structured market snapshot and returns a structured trading decision
- **PostgreSQL** — Persistent store for candles, decisions, orders, positions, portfolio metrics
- **Redis** — Candle cache and short-lived state
- **React Dashboard** — Existing frontend; continues consuming the same REST and WebSocket API contracts
- **System Operator** — Configures via `.env`, monitors via dashboard

---

## Functional requirements

### FR-1: Binance Market Data Ingestion
The backend subscribes to Binance kline and best-ticker WebSocket streams for all configured symbols using `binance-connector-python`'s async `WebsocketClient`. On each closed candle, the system aggregates OHLCV data, persists the candle to Postgres, and fans it out for LLM evaluation.

### FR-2: Polymarket Market Data Ingestion
The backend connects to Polymarket CLOB WebSocket (`py-clob-client`) and subscribes to price feeds and order book updates for configured markets. Updates are stored and made available to the LLM decision context.

### FR-3: Technical Indicator Computation
On each closed candle, the backend computes: RSI (14), MACD (12/26/9), Bollinger Bands (20, 2σ), EMA (9, 21, 50, 200), VWAP, ATR (14). Results are persisted to Postgres and included in the LLM context payload.

### FR-4: LLM Decision Engine
When a candle closes, the system assembles a structured JSON context object containing:
- Last N candles (configurable, default 50)
- All computed indicators for the latest candle
- Open positions for the symbol
- Portfolio metrics (balance, drawdown, open position count)
- Recent decisions (last 5) to prevent thrashing
- Polymarket correlated market snapshot (optional, if available)

This context is submitted as a user message to Claude (claude-sonnet-4-6 or configured model) with a system prompt defining the agent's role, risk constraints, and output schema. The LLM returns a structured JSON decision matching the Decision domain type.

### FR-5: LLM Output Schema
The LLM must return a JSON object with:
```json
{
  "action": "BUY" | "SELL" | "HOLD",
  "confidence": 0-100,
  "reasoning": "string (free text explanation)",
  "suggested_entry": number | null,
  "suggested_take_profit": number | null,
  "suggested_stop_loss": number | null
}
```
If the LLM response cannot be parsed, the decision defaults to HOLD.

### FR-6: Risk Gating
Before a decision is acted upon, the system applies rule-based risk gates:
- Confidence must exceed `GOLD_CONFIDENCE_THRESHOLD` (default 70)
- Open positions must be below `GOLD_MAX_POSITIONS` (default 3)
- Portfolio drawdown must be below `GOLD_MAX_DRAWDOWN_PERCENT` (default 15%)
- If any gate fails, the decision's `execution_status` is set to the appropriate rejection reason; no order is placed

### FR-7: Book Order Placement
When a BUY or SELL decision passes all risk gates and `GOLD_DRY_RUN=false`:
- For Binance symbols: place a limit order via `binance-connector-python` Spot REST client at the LLM-suggested entry price with computed position size
- For Polymarket markets: place a limit order via `py-clob-client` `post_order()` at the LLM-suggested price with computed size
- The placed order is persisted to the `orders` table
- On fill confirmation, a Position record is created

### FR-8: Position Monitoring
Open positions are monitored for take-profit, stop-loss, and trailing stop conditions. On trigger, the system places a closing order and marks the position as closed with the appropriate close reason.

### FR-9: Portfolio Metrics
After each order fill or position close, portfolio metrics are recomputed and stored. Circuit breaker trips when drawdown exceeds the maximum; no new positions are opened until reset.

### FR-10: Exchange Balance Fetching
The `/api/v1/exchange/balances` REST endpoint returns live Binance USDT balance (via `binance-connector-python` REST) and Polymarket USDC balance (via `py-clob-client` `get_balance_allowance()`).

### FR-11: REST and WebSocket API Compatibility
The Python backend exposes the identical REST and WebSocket API contracts as the current Go backend. The existing React dashboard must work without changes.

### FR-12: Dry-Run Mode
When `GOLD_DRY_RUN=true`, all LLM decisions are logged and persisted but no orders are placed. The LLM still runs on every candle close.

---

## Technical requirements

### Architecture

```
binance-connector-python (WS)  ──► CandleAggregator ──► IndicatorComputer
                                          │                       │
                                          ▼                       ▼
py-clob-client (WS + REST)   ──► ContextBuilder ◄──── PortfolioManager
                                          │
                                          ▼
                              LLMDecisionEngine (Claude API)
                                          │
                                     ┌────▼────┐
                                     │ RiskGate│
                                     └────┬────┘
                                          │
                            ┌─────────────▼─────────────┐
                            │         Executor           │
                            │ (Binance REST / py-clob)   │
                            └─────────────┬──────────────┘
                                          │
                               PositionMonitor
                                          │
                               PortfolioManager
                                          │
                              WebSocket Hub (FastAPI)
                                          │
                              REST API    ◄── Dashboard
```

**Tech stack:**
- Python 3.12+
- **FastAPI** — async HTTP server + WebSocket hub
- **asyncio** — concurrency model (replaces Go goroutines / errgroup)
- **binance-connector-python** — Binance SDK (WebSocket streams + REST)
- **py-clob-client** — Polymarket CLOB SDK (WebSocket + REST)
- **anthropic** — Claude API SDK (with prompt caching)
- **asyncpg** — async PostgreSQL driver
- **redis.asyncio** — async Redis client
- **pydantic v2** — domain models and LLM response validation
- **python-dotenv** — .env loading

### Data model
Same Postgres schema as Go backend (no migrations required). Python models use Pydantic and map 1:1 to existing tables:
- `candles`, `indicators`, `positions`, `orders`, `decisions`, `portfolio_snapshots`, `news_articles`, `sentiment_scores`

### API contracts

The Python backend preserves all existing REST endpoints unchanged:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/candles` | Paginated candle history |
| GET | `/api/v1/positions` | Open + closed positions |
| GET | `/api/v1/trades` | Closed trade history |
| GET | `/api/v1/decisions` | Decision log |
| GET | `/api/v1/metrics` | Portfolio metrics |
| GET | `/api/v1/exchange/balances` | Live exchange balances |
| WS | `/ws/v1/stream` | Real-time push stream |

WebSocket message types preserved: `candle_update`, `position_update`, `position_closed`, `trade_executed`, `metric_update`, `decision_made`

### Infrastructure

**New Python service structure:**
```
gold-agent/               # New Python backend (replaces gold-backend/)
├── main.py               # Application entry point
├── config.py             # Pydantic settings from env
├── domain/
│   └── types.py          # Pydantic domain models
├── exchange/
│   ├── binance_stream.py # binance-connector-python WS wrapper
│   ├── binance_rest.py   # binance-connector-python REST wrapper
│   ├── polymarket_stream.py  # py-clob-client WS wrapper
│   └── polymarket_rest.py    # py-clob-client REST wrapper
├── market/
│   └── aggregator.py     # Candle aggregation + persistence
├── analysis/
│   └── indicators.py     # Technical indicator computation (pandas-ta or manual)
├── engine/
│   ├── context_builder.py # Assemble LLM context payload
│   ├── llm_engine.py      # Claude API call + response parsing
│   ├── risk.py            # Risk gate logic
│   └── prompts.py         # System prompt and output schema definition
├── execution/
│   ├── executor.py        # Order placement router
│   ├── position_monitor.py
│   └── portfolio_manager.py
├── storage/
│   ├── postgres.py        # asyncpg pool + repositories
│   └── redis_client.py    # Redis cache
├── api/
│   ├── router.py          # FastAPI router
│   ├── handlers.py        # REST endpoint handlers
│   └── websocket_hub.py   # WebSocket connection manager
├── requirements.txt
├── Dockerfile
└── .env (inherited)
```

**Dependencies (requirements.txt additions):**
```
fastapi>=0.111
uvicorn[standard]>=0.29
binance-connector-python>=3.7
py-clob-client>=0.18
anthropic>=0.25
asyncpg>=0.29
redis>=5.0
pydantic>=2.7
pydantic-settings>=2.2
pandas>=2.2
pandas-ta>=0.3.14b
python-dotenv>=1.0
httpx>=0.27
```

### LLM Integration

**System prompt (stored in `engine/prompts.py`):**
```
You are a professional cryptocurrency trading agent. You receive real-time market data 
and must decide whether to place a book order, hold, or exit a position.

You have access to:
- Recent OHLCV candle data
- Technical indicators (RSI, MACD, Bollinger Bands, EMA, VWAP, ATR)
- Current open positions and portfolio risk metrics
- Correlated Polymarket prediction market prices (when available)

Your response MUST be valid JSON matching this exact schema:
{
  "action": "BUY" | "SELL" | "HOLD",
  "confidence": <integer 0-100>,
  "reasoning": "<concise explanation>",
  "suggested_entry": <number or null>,
  "suggested_take_profit": <number or null>,
  "suggested_stop_loss": <number or null>
}

Risk constraints you must respect:
- Never suggest confidence > 80 without strong multi-indicator confluence
- suggested_stop_loss distance must not exceed 2× ATR from entry
- suggested_take_profit must yield risk/reward ≥ 1.5
```

**Prompt caching:** The system prompt is marked with Anthropic's cache_control breakpoint to reduce token costs on repeated calls.

**Context payload structure (user message):**
```json
{
  "symbol": "BTCUSDT",
  "interval": "5m",
  "timestamp": "ISO-8601",
  "candles": [...],          // last 50 candles, OHLCV
  "indicators": {...},        // latest computed indicators
  "open_positions": [...],    // positions for this symbol
  "portfolio": {
    "balance": "...",
    "drawdown_percent": "...",
    "open_position_count": N
  },
  "recent_decisions": [...],  // last 5 decisions for this symbol
  "polymarket_snapshot": {...} // optional
}
```

## Non-functional requirements

### Performance
- LLM decision latency: < 5 seconds per candle close (non-blocking; decision published asynchronously)
- REST endpoint p99: < 200ms
- WebSocket fan-out: < 50ms for connected clients
- Indicator computation: < 100ms per candle

### Security
- API keys loaded from environment only; never logged
- Anthropic API key never included in LLM messages
- Polymarket credentials (L2 auth) handled by py-clob-client SDK; no custom signing code needed

### Reliability
- WebSocket streams auto-reconnect with exponential backoff (max 60s)
- LLM call failures default to HOLD decision; never crash the pipeline
- asyncio structured concurrency with graceful shutdown (SIGINT/SIGTERM)

### Compatibility
- Dashboard REST and WebSocket contracts unchanged; zero frontend changes required
- Postgres schema unchanged; no migrations required
- `.env.example` gains two new variables: `GOLD_LLM_MODEL` and `GOLD_LLM_CONTEXT_CANDLES`

---

## Dependencies

| Dependency | Status |
|-----------|--------|
| Anthropic API key (`ANTHROPIC_API_KEY`) | Available in .env.example |
| Binance API credentials | Available in .env.example |
| Polymarket API credentials | Available in .env.example |
| PostgreSQL (existing schema) | Available via docker-compose |
| Redis | Available via docker-compose |
| `binance-connector-python` PyPI | Available |
| `py-clob-client` PyPI | Available |
| `anthropic` PyPI SDK | Available |

---

## Constraints

- The React dashboard (`gold-dashboard/`) must not be modified
- The Postgres schema must not require new migrations for core functionality
- The Go backend (`gold-backend/`) can be removed or left in place; no shared code
- The existing `.env` variable names must be preserved (additive changes only)
- Python 3.12+ required (asyncio.TaskGroup, match statements)
- No synchronous blocking calls in async paths (use `asyncio.to_thread` if needed)

---

## Open questions

None — proceeding to plan.

---

## Glossary

- **Book order** — A limit order placed on an exchange order book (as opposed to a market order)
- **L2 auth** — Polymarket's HMAC-SHA256 request-level authentication (handled by py-clob-client)
- **Prompt caching** — Anthropic feature that caches the system prompt token cost across repeated calls
- **CLOB** — Central Limit Order Book (Polymarket's matching engine)
