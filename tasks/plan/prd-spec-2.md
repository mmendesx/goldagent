# PRD: Python Backend Migration with LLM-Driven Trading Decisions

**Spec**: tasks/specs/spec-2-python-backend-migration.md

## Summary

Migrate the trading agent backend from Go to Python, replacing the hand-coded signal scoring engine with Claude LLM reasoning. The Python service uses `binance-connector-python` and `py-clob-client` for exchange connectivity, `pandas-ta` for indicator computation, and the Anthropic SDK to submit structured market snapshots to Claude. The existing React dashboard and Postgres schema are preserved unchanged.

---

## Behavior scenarios

### Feature: Project Scaffolding and Configuration

#### Scenario: Config loads all required env vars
  Given a valid `.env` file with all required variables
  When the application starts
  Then `Settings` resolves all values without raising a validation error

#### Scenario: Config raises on missing required var
  Given an `.env` file missing `BINANCE_API_KEY`
  When the application starts
  Then a `ValidationError` is raised and the process exits with a non-zero code

#### Scenario: New LLM env vars have defaults
  Given an `.env` file that does not set `GOLD_LLM_MODEL` or `GOLD_LLM_CONTEXT_CANDLES`
  When `Settings` is instantiated
  Then `llm_model` defaults to `"claude-sonnet-4-6"` and `llm_context_candles` defaults to `50`

---

### Feature: Domain Models

#### Scenario: Candle model validates required fields
  Given a dict with all required OHLCV fields
  When a `Candle` Pydantic model is constructed
  Then the model is valid and field values are accessible

#### Scenario: LLMDecisionResponse rejects invalid action
  Given a dict with `"action": "MAYBE"`
  When an `LLMDecisionResponse` model is constructed
  Then a `ValidationError` is raised

#### Scenario: Decision model serialises to JSON matching API contract
  Given a `Decision` object with action `"BUY"`, confidence `85`
  When `.model_dump()` is called
  Then the output keys match the dashboard's expected TypeScript `Decision` interface

---

### Feature: Storage — Postgres

#### Scenario: Candle is upserted without duplicate key error
  Given an open connection to Postgres
  When the same candle is inserted twice (same symbol, interval, open_time)
  Then only one row exists and no exception is raised

#### Scenario: Fetch last N candles returns in ascending time order
  Given 100 candles for `BTCUSDT/5m` in the database
  When `fetch_candles(symbol="BTCUSDT", interval="5m", limit=50)` is called
  Then 50 rows are returned ordered by `open_time ASC`

#### Scenario: Decision is persisted with all signal fields
  Given a `Decision` object with action `"HOLD"` and a reasoning string
  When `save_decision(decision)` is called
  Then a row exists in the `decisions` table with the correct `action` and `reasoning`

#### Scenario: Open positions query returns only status='open' rows
  Given 2 open and 3 closed positions for `BTCUSDT`
  When `fetch_open_positions(symbol="BTCUSDT")` is called
  Then exactly 2 positions are returned

---

### Feature: Storage — Redis

#### Scenario: Candle cache stores and retrieves by key
  Given a connected Redis client
  When a `Candle` is cached under key `"candle:BTCUSDT:5m:latest"`
  Then a subsequent `get` returns the same candle data

#### Scenario: Cache miss returns None
  Given a connected Redis client with no data
  When `get("nonexistent_key")` is called
  Then `None` is returned

---

### Feature: Binance Stream Client

#### Scenario: Closed candle is emitted on kline close event
  Given a connected `BinanceStreamClient` subscribed to `BTCUSDT@kline_5m`
  When a kline WebSocket message arrives with `x: true` (candle closed)
  Then a `Candle` with `is_closed=True` is placed on the output queue

#### Scenario: Open (in-progress) candle updates are emitted
  Given a connected `BinanceStreamClient`
  When a kline message arrives with `x: false`
  Then a `Candle` with `is_closed=False` is placed on the output queue

#### Scenario: Stream reconnects after connection drop
  Given a connected `BinanceStreamClient`
  When the WebSocket connection closes unexpectedly
  Then the client reconnects within 60 seconds using exponential backoff

#### Scenario: Stream subscribes to all configured symbols
  Given `GOLD_SYMBOLS=BTCUSDT,ETHUSDT`
  When `BinanceStreamClient` starts
  Then WebSocket subscriptions are created for both `BTCUSDT@kline_*` and `ETHUSDT@kline_*`

---

### Feature: Polymarket Stream Client

#### Scenario: Price update is placed on output queue
  Given a connected `PolymarketStreamClient`
  When a price update WebSocket message arrives
  Then a `PolymarketCryptoPrice` is placed on the output queue

#### Scenario: Stream reconnects after disconnect
  Given a connected `PolymarketStreamClient`
  When the connection drops
  Then the client reconnects within 60 seconds

#### Scenario: Stream client handles missing credentials gracefully
  Given `POLYMARKET_API_KEY` is not set
  When `PolymarketStreamClient` initialises
  Then it logs a warning and skips connection without crashing

---

### Feature: Candle Aggregation

#### Scenario: Closed candle is persisted to Postgres
  Given the `CandleAggregator` is running
  When a `Candle` with `is_closed=True` arrives on the input queue
  Then a row is upserted to the `candles` table

#### Scenario: Closed candle is placed on the fan-out queue
  Given the `CandleAggregator` is running
  When a closed candle arrives
  Then the same candle appears on the `closed_candle_queue`

#### Scenario: Open candle is not persisted
  Given the `CandleAggregator` is running
  When a `Candle` with `is_closed=False` arrives
  Then no database write occurs and the candle is not placed on `closed_candle_queue`

---

### Feature: Technical Indicator Computation

#### Scenario: All indicators are computed for a closed candle
  Given 200 closed candles for `BTCUSDT/5m` in Postgres
  When `compute_indicators(symbol, interval, candle)` is called
  Then an `Indicator` object is returned with non-null RSI, MACD, Bollinger, EMA-9/21/50/200, VWAP, ATR fields

#### Scenario: Indicators are persisted after computation
  Given a valid `Indicator` object
  When `save_indicators(indicator)` is called
  Then a row exists in the `indicators` table for the candle's `open_time`

#### Scenario: Insufficient candle history returns None
  Given fewer than 20 candles in Postgres (below Bollinger warm-up period)
  When `compute_indicators` is called
  Then `None` is returned and no database write occurs

---

### Feature: Exchange REST Clients

#### Scenario: Binance balance returns USDT free balance
  Given valid `BINANCE_API_KEY` and `BINANCE_API_SECRET`
  When `BinanceRestClient.fetch_usdt_balance()` is called
  Then a string representation of the USDT free balance is returned

#### Scenario: Binance balance returns 'not_configured' when keys absent
  Given `BINANCE_API_KEY` is empty
  When `BinanceRestClient.fetch_usdt_balance()` is called
  Then an `ExchangeBalance(balance="0", status="not_configured")` is returned

#### Scenario: Polymarket balance returns USDC collateral
  Given valid Polymarket credentials
  When `PolymarketRestClient.fetch_usdc_balance()` is called
  Then a string USDC balance is returned

#### Scenario: Polymarket balance returns 'not_configured' when credentials absent
  Given `POLYMARKET_API_KEY` is empty
  When `PolymarketRestClient.fetch_usdc_balance()` is called
  Then `ExchangeBalance(balance="0", status="not_configured")` is returned

---

### Feature: LLM Context Builder

#### Scenario: Context payload includes last N candles
  Given `GOLD_LLM_CONTEXT_CANDLES=50` and 100 candles in Postgres for `BTCUSDT/5m`
  When `build_context(symbol="BTCUSDT", interval="5m", candle=latest)` is called
  Then the returned payload's `candles` list contains exactly 50 items

#### Scenario: Context payload includes latest indicators
  Given a computed `Indicator` row in Postgres
  When `build_context` is called
  Then the payload's `indicators` field contains the RSI, MACD, and ATR values

#### Scenario: Context payload includes Polymarket snapshot when available
  Given a recent `PolymarketCryptoPrice` for `BTCUSDT` in the in-memory cache
  When `build_context` is called
  Then `polymarket_snapshot` in the payload is non-null

#### Scenario: Context payload omits Polymarket when unavailable
  Given no Polymarket data in cache
  When `build_context` is called
  Then `polymarket_snapshot` in the payload is `null`

---

### Feature: LLM Decision Engine

#### Scenario: LLM returns valid BUY decision
  Given a context payload and a Claude API response containing `{"action":"BUY","confidence":82,...}`
  When `LLMDecisionEngine.evaluate(context)` is called
  Then a `Decision` with `action="BUY"` and `confidence=82` is returned

#### Scenario: Malformed LLM response defaults to HOLD
  Given the Claude API returns non-JSON text
  When `LLMDecisionEngine.evaluate(context)` is called
  Then a `Decision` with `action="HOLD"` and `confidence=0` is returned and the error is logged

#### Scenario: LLM API error defaults to HOLD
  Given the Anthropic API raises a network error
  When `LLMDecisionEngine.evaluate(context)` is called
  Then a `Decision` with `action="HOLD"` is returned without raising

#### Scenario: System prompt uses prompt caching
  Given a configured `LLMDecisionEngine`
  When two consecutive `evaluate` calls are made
  Then the second API call includes a `cache_control` breakpoint on the system prompt

---

### Feature: Risk Gating

#### Scenario: Decision below confidence threshold is rejected
  Given `GOLD_CONFIDENCE_THRESHOLD=70` and a BUY decision with `confidence=65`
  When `RiskGate.check(decision, portfolio)` is called
  Then `is_allowed=False` and `rejection_reason="LOW_CONFIDENCE"`

#### Scenario: Decision rejected when max positions reached
  Given `GOLD_MAX_POSITIONS=3` and 3 open positions
  When `RiskGate.check(buy_decision, portfolio)` is called
  Then `is_allowed=False` and `rejection_reason="MAX_POSITIONS_REACHED"`

#### Scenario: Decision rejected when circuit breaker active
  Given portfolio drawdown is 16% and `GOLD_MAX_DRAWDOWN_PERCENT=15`
  When `RiskGate.check(buy_decision, portfolio)` is called
  Then `is_allowed=False` and `rejection_reason="MAX_DRAWDOWN_REACHED"`

#### Scenario: HOLD decision is never blocked by risk gates
  Given a HOLD decision with confidence 30
  When `RiskGate.check(hold_decision, portfolio)` is called
  Then `is_allowed=True` regardless of open positions or drawdown

#### Scenario: Valid BUY decision passes all gates
  Given confidence=80, 1 open position, drawdown=5%
  When `RiskGate.check(buy_decision, portfolio)` is called
  Then `is_allowed=True`

---

### Feature: Order Execution

#### Scenario: Binance limit order is placed on valid BUY decision
  Given `GOLD_DRY_RUN=false`, a BUY TradeIntent for `BTCUSDT`, and valid Binance credentials
  When `Executor.execute(intent)` is called
  Then a limit order is submitted via `binance-connector-python` Spot REST
  And the order is persisted to the `orders` table

#### Scenario: Polymarket limit order is placed on valid BUY decision
  Given `GOLD_DRY_RUN=false`, a BUY TradeIntent for a Polymarket market, and valid credentials
  When `Executor.execute(intent)` is called
  Then `py_clob_client.post_order()` is invoked
  And the order is persisted to the `orders` table

#### Scenario: No order is placed in dry-run mode
  Given `GOLD_DRY_RUN=true` and a BUY decision that passed risk gates
  When `Executor.execute(intent)` is called
  Then no external API call is made
  And the decision's `execution_status` is set to `"DRY_RUN"`

#### Scenario: Order placement failure is logged without crashing
  Given the Binance API returns a 400 error
  When `Executor.execute(intent)` is called
  Then the error is logged
  And no position is created
  And the pipeline continues running

---

### Feature: Position Monitoring

#### Scenario: Position is closed at take-profit price
  Given an open position with `take_profit_price=50000` and current price reaches `50001`
  When `PositionMonitor` runs its check loop
  Then a closing order is placed
  And the position's `close_reason` is set to `"TAKE_PROFIT"`

#### Scenario: Position is closed at stop-loss price
  Given an open position with `stop_loss_price=45000` and current price drops to `44999`
  When `PositionMonitor` runs its check loop
  Then a closing order is placed
  And the position's `close_reason` is set to `"STOP_LOSS"`

#### Scenario: Trailing stop updates as price moves in favour
  Given an open LONG position with current price rising
  When `PositionMonitor` detects a new high above the previous trailing reference
  Then `trailing_stop_price` is updated upward in the `positions` table

#### Scenario: Position monitor does not run in dry-run mode
  Given `GOLD_DRY_RUN=true`
  When the application starts
  Then `PositionMonitor` is not started and no position checks occur

---

### Feature: Portfolio Metrics

#### Scenario: Balance and drawdown update after position close
  Given a closed profitable position with realized PnL of $200
  When `PortfolioManager.snapshot()` is called
  Then the new `PortfolioMetrics.balance` reflects the gain
  And `total_pnl` increases by $200

#### Scenario: Circuit breaker trips when drawdown exceeds limit
  Given portfolio peak balance is $10,000 and current balance drops to $8,400
  When `PortfolioManager.snapshot()` is called
  Then `is_circuit_breaker_active=True`
  And subsequent `RiskGate.check` calls reject all non-HOLD decisions

---

### Feature: REST API

#### Scenario: GET /api/v1/candles returns paginated candles
  Given 100 candles for `BTCUSDT/5m` in Postgres
  When `GET /api/v1/candles?symbol=BTCUSDT&interval=5m&limit=20` is called
  Then a 200 response is returned with exactly 20 candle objects

#### Scenario: GET /api/v1/positions returns open positions
  Given 2 open and 3 closed positions
  When `GET /api/v1/positions?status=open` is called
  Then 2 position objects are returned

#### Scenario: GET /api/v1/exchange/balances returns both exchange statuses
  Given configured Binance and Polymarket credentials
  When `GET /api/v1/exchange/balances` is called
  Then the response contains `{"binance": {...}, "polymarket": {...}}`
  And both have `"status": "ok"`

#### Scenario: Unconfigured exchange returns 'not_configured' status
  Given `BINANCE_API_KEY` is empty
  When `GET /api/v1/exchange/balances` is called
  Then `response.binance.status == "not_configured"`

#### Scenario: GET /api/v1/decisions returns decision log
  Given 10 persisted decisions
  When `GET /api/v1/decisions?limit=5` is called
  Then 5 decisions are returned ordered by `created_at DESC`

---

### Feature: WebSocket Hub

#### Scenario: Connected client receives candle_update on closed candle
  Given a dashboard client connected to `/ws/v1/stream`
  When a candle closes and is processed
  Then the client receives a `{"type":"candle_update","payload":{...}}` message

#### Scenario: Connected client receives decision_made after LLM evaluates
  Given a dashboard client connected to `/ws/v1/stream`
  When `LLMDecisionEngine` completes evaluation
  Then the client receives a `{"type":"decision_made","payload":{...}}` message

#### Scenario: Client disconnection is handled gracefully
  Given a client connected to `/ws/v1/stream`
  When the client disconnects
  Then the hub removes it from the active set without error

#### Scenario: Multiple clients receive the same broadcast
  Given 3 clients connected to `/ws/v1/stream`
  When a `metric_update` event is published
  Then all 3 clients receive the message within 50ms

---

### Feature: Application Lifecycle

#### Scenario: Application starts all components successfully
  Given valid configuration and reachable Postgres and Redis
  When `main.py` is executed
  Then all services start without error and the HTTP server listens on `GOLD_HTTP_PORT`

#### Scenario: SIGTERM triggers graceful shutdown
  Given a running application with active WebSocket connections
  When SIGTERM is received
  Then open connections are closed, Postgres pool is released, and the process exits cleanly

#### Scenario: Postgres connection failure prevents startup
  Given an unreachable Postgres host
  When the application starts
  Then an error is logged and the process exits with code 1

---

## Tasks

### ICT-1: Project Scaffold and Configuration
- **What**: Create the `gold-agent/` directory with `__init__.py` files, `requirements.txt` (all deps from spec), and `config.py` using `pydantic-settings`. Config must load all existing `.env` variables plus `GOLD_LLM_MODEL` (default `claude-sonnet-4-6`) and `GOLD_LLM_CONTEXT_CANDLES` (default `50`). Update `.env.example` with the two new vars.
- **Where**: `gold-agent/config.py`, `gold-agent/requirements.txt`, `.env.example`
- **Validated by**: Config loads all required env vars, Config raises on missing required var, New LLM env vars have defaults
- **Estimate**: S

### ICT-2: Domain Models
- **What**: Implement all Pydantic v2 domain models in `gold-agent/domain/types.py`: `Candle`, `Indicator`, `Position`, `Order`, `Decision`, `PortfolioMetrics`, `TradeIntent`, `ExchangeBalance`, `ExchangeBalances`, `LLMDecisionResponse`, `PolymarketCryptoPrice`. Field names and types must match the existing TypeScript `types/index.ts` interface.
- **Where**: `gold-agent/domain/types.py`
- **Validated by**: Candle model validates required fields, LLMDecisionResponse rejects invalid action, Decision model serialises to JSON matching API contract
- **Estimate**: S

### ICT-3: Storage — Postgres Repositories
- **What**: Implement `gold-agent/storage/postgres.py` with asyncpg connection pool and repositories: `save_candle` (upsert), `fetch_candles` (limit + order), `save_indicator`, `fetch_indicator`, `save_decision`, `fetch_decisions`, `save_order`, `fetch_open_positions`, `close_position`, `save_portfolio_snapshot`, `fetch_latest_portfolio`. Pool init/teardown as async context managers.
- **Where**: `gold-agent/storage/postgres.py`
- **Validated by**: Candle is upserted without duplicate key error, Fetch last N candles returns in ascending time order, Decision is persisted with all signal fields, Open positions query returns only status='open' rows
- **Estimate**: M

### ICT-4: Storage — Redis Client
- **What**: Implement `gold-agent/storage/redis_client.py` with `redis.asyncio` pool: `set(key, value, ttl)`, `get(key)`, `delete(key)`. Keys for candle cache: `"candle:{symbol}:{interval}:latest"`. Serialize with JSON.
- **Where**: `gold-agent/storage/redis_client.py`
- **Validated by**: Candle cache stores and retrieves by key, Cache miss returns None
- **Estimate**: S

### ICT-5: Binance Stream Client
- **What**: Implement `gold-agent/exchange/binance_stream.py` wrapping `binance-connector-python`'s `WebsocketClient`. Subscribe to `{symbol}@kline_{interval}` and `{symbol}@bookTicker` for all configured symbols. Parse kline messages into `Candle` objects and put them on an `asyncio.Queue`. Implement exponential backoff reconnection (max 60s delay). Use `asyncio.to_thread` to bridge the sync SDK callbacks into the async event loop.
- **Where**: `gold-agent/exchange/binance_stream.py`
- **Validated by**: Closed candle is emitted on kline close event, Open candle updates are emitted, Stream reconnects after connection drop, Stream subscribes to all configured symbols
- **Estimate**: M

### ICT-6: Polymarket Stream Client
- **What**: Implement `gold-agent/exchange/polymarket_stream.py` wrapping `py-clob-client`'s WebSocket subscription for price feeds. Parse updates into `PolymarketCryptoPrice` objects and put them on an `asyncio.Queue`. Skip gracefully when credentials are absent. Implement reconnect with backoff.
- **Where**: `gold-agent/exchange/polymarket_stream.py`
- **Validated by**: Price update is placed on output queue, Stream reconnects after disconnect, Stream client handles missing credentials gracefully
- **Estimate**: M

### ICT-7: Exchange REST Clients
- **What**: Implement `gold-agent/exchange/binance_rest.py` (uses `binance-connector-python` Spot REST client) with `fetch_usdt_balance() -> ExchangeBalance`. Implement `gold-agent/exchange/polymarket_rest.py` (uses `py-clob-client`'s `ClobClient`) with `fetch_usdc_balance() -> ExchangeBalance`. Both return `status="not_configured"` when credentials are absent and `status="error"` on API failures.
- **Where**: `gold-agent/exchange/binance_rest.py`, `gold-agent/exchange/polymarket_rest.py`
- **Validated by**: Binance balance returns USDT free balance, Binance balance returns 'not_configured' when keys absent, Polymarket balance returns USDC collateral, Polymarket balance returns 'not_configured' when credentials absent
- **Estimate**: S

### ICT-8: Candle Aggregator
- **What**: Implement `gold-agent/market/aggregator.py`. Consumes from the Binance stream queue. For closed candles: upsert to Postgres via repo, cache in Redis, put on `closed_candle_queue`. For open candles: update Redis only (no DB write). Also fan out to the WebSocket hub as `candle_update`.
- **Where**: `gold-agent/market/aggregator.py`
- **Validated by**: Closed candle is persisted to Postgres, Closed candle is placed on the fan-out queue, Open candle is not persisted
- **Estimate**: S

### ICT-9: Technical Indicator Computation
- **What**: Implement `gold-agent/analysis/indicators.py` using `pandas-ta`. `compute_indicators(symbol, interval, candle, candle_repo) -> Indicator | None`. Fetches last 200 candles from Postgres, builds a DataFrame, computes RSI(14), MACD(12,26,9), Bollinger(20,2), EMA(9,21,50,200), VWAP, ATR(14). Returns `None` if fewer than 20 candles. Persists the result via indicator repo.
- **Where**: `gold-agent/analysis/indicators.py`
- **Validated by**: All indicators are computed for a closed candle, Indicators are persisted after computation, Insufficient candle history returns None
- **Estimate**: M

### ICT-10: LLM Context Builder
- **What**: Implement `gold-agent/engine/context_builder.py`. `build_context(symbol, interval, candle, repos, polymarket_cache) -> dict`. Fetches last `GOLD_LLM_CONTEXT_CANDLES` candles, latest indicator, open positions, latest portfolio snapshot, last 5 decisions. Attaches Polymarket snapshot from in-memory dict if available. Returns serialized dict matching the spec's context payload schema.
- **Where**: `gold-agent/engine/context_builder.py`
- **Validated by**: Context payload includes last N candles, Context payload includes latest indicators, Context payload includes Polymarket snapshot when available, Context payload omits Polymarket when unavailable
- **Estimate**: S

### ICT-11: LLM Prompts
- **What**: Implement `gold-agent/engine/prompts.py` with the system prompt string (from spec). Mark it for Anthropic prompt caching with `{"type": "text", "text": SYSTEM_PROMPT, "cache_control": {"type": "ephemeral"}}`. Provide a `format_user_message(context: dict) -> str` helper that serializes context to compact JSON.
- **Where**: `gold-agent/engine/prompts.py`
- **Validated by**: System prompt uses prompt caching
- **Estimate**: S

### ICT-12: LLM Decision Engine
- **What**: Implement `gold-agent/engine/llm_engine.py` using the `anthropic` SDK. `LLMDecisionEngine.evaluate(context: dict, symbol: str) -> Decision`. Submits context as user message with cached system prompt to `GOLD_LLM_MODEL`. Parses JSON response into `LLMDecisionResponse` via Pydantic. On any failure (network, parse, validation), returns a HOLD decision with confidence 0 and logs the error. Saves the decision to Postgres.
- **Where**: `gold-agent/engine/llm_engine.py`
- **Validated by**: LLM returns valid BUY decision, Malformed LLM response defaults to HOLD, LLM API error defaults to HOLD, System prompt uses prompt caching
- **Estimate**: M

### ICT-13: Risk Gate
- **What**: Implement `gold-agent/engine/risk.py`. `RiskGate.check(decision: Decision, portfolio: PortfolioMetrics, open_position_count: int) -> RiskCheckResult`. Applies three gates: confidence threshold, max positions (only for BUY/SELL), max drawdown (only for BUY/SELL). Returns `RiskCheckResult(is_allowed: bool, rejection_reason: str | None)`. HOLD always passes.
- **Where**: `gold-agent/engine/risk.py`
- **Validated by**: Decision below confidence threshold is rejected, Decision rejected when max positions reached, Decision rejected when circuit breaker active, HOLD decision is never blocked by risk gates, Valid BUY decision passes all gates
- **Estimate**: S

### ICT-14: Order Executor
- **What**: Implement `gold-agent/execution/executor.py`. `Executor.execute(intent: TradeIntent) -> Order | None`. Routes by symbol prefix: Binance symbols use `binance_rest.py` limit order placement; Polymarket markets use `polymarket_rest.py` `post_order()`. In dry-run mode, logs intent and returns without placing. Persists the resulting order. On API error, logs and returns None.
- **Where**: `gold-agent/execution/executor.py`
- **Validated by**: Binance limit order is placed on valid BUY decision, Polymarket limit order is placed on valid BUY decision, No order is placed in dry-run mode, Order placement failure is logged without crashing
- **Estimate**: M

### ICT-15: Position Monitor
- **What**: Implement `gold-agent/execution/position_monitor.py`. Async loop that runs every 5 seconds. Fetches all open positions from Postgres. For each, checks current price (from Redis ticker cache) against `take_profit_price`, `stop_loss_price`, and trailing stop. On trigger, places closing order via `Executor` and updates position with `close_reason` and `closed_at`. Does not start in dry-run mode.
- **Where**: `gold-agent/execution/position_monitor.py`
- **Validated by**: Position is closed at take-profit price, Position is closed at stop-loss price, Trailing stop updates as price moves in favour, Position monitor does not run in dry-run mode
- **Estimate**: M

### ICT-16: Portfolio Manager
- **What**: Implement `gold-agent/execution/portfolio_manager.py`. `PortfolioManager.snapshot() -> PortfolioMetrics`. Queries all closed positions for PnL aggregation (win count, loss count, total PnL, profit factor, average win/loss). Reads current balance from latest order fills. Computes drawdown from peak. Persists snapshot. Sets `is_circuit_breaker_active=True` when drawdown exceeds limit.
- **Where**: `gold-agent/execution/portfolio_manager.py`
- **Validated by**: Balance and drawdown update after position close, Circuit breaker trips when drawdown exceeds limit
- **Estimate**: M

### ICT-17: WebSocket Hub
- **What**: Implement `gold-agent/api/websocket_hub.py`. `WebSocketHub` class with `connect(ws)`, `disconnect(ws)`, `broadcast(message: dict)`. Uses `asyncio.Lock` for thread-safe set management. `broadcast` serializes to JSON and sends to all active connections; silently removes disconnected clients. Exposes typed publish methods: `publish_candle`, `publish_decision`, `publish_position`, `publish_metrics`.
- **Where**: `gold-agent/api/websocket_hub.py`
- **Validated by**: Connected client receives candle_update on closed candle, Connected client receives decision_made after LLM evaluates, Client disconnection is handled gracefully, Multiple clients receive the same broadcast
- **Estimate**: S

### ICT-18: REST API Handlers
- **What**: Implement `gold-agent/api/handlers.py` and `gold-agent/api/router.py` using FastAPI. Endpoints: `GET /api/v1/candles`, `GET /api/v1/positions`, `GET /api/v1/trades`, `GET /api/v1/decisions`, `GET /api/v1/metrics`, `GET /api/v1/exchange/balances`, `WS /ws/v1/stream`. All REST endpoints accept query params matching Go backend (symbol, interval, limit, status). Responses serialized from Pydantic models. Add CORS middleware permitting the dashboard origin.
- **Where**: `gold-agent/api/handlers.py`, `gold-agent/api/router.py`
- **Validated by**: GET /api/v1/candles returns paginated candles, GET /api/v1/positions returns open positions, GET /api/v1/exchange/balances returns both exchange statuses, Unconfigured exchange returns 'not_configured' status, GET /api/v1/decisions returns decision log
- **Estimate**: M

### ICT-19: Application Entry Point
- **What**: Implement `gold-agent/main.py`. Uses `asyncio.TaskGroup` (Python 3.12) to start all components concurrently: Postgres pool init, Redis init, BinanceStreamClient, PolymarketStreamClient, CandleAggregator, IndicatorComputer loop, LLM decision loop, Executor + PositionMonitor (non-dry-run only), PortfolioManager snapshot loop, WebSocketHub, Uvicorn FastAPI server. Register SIGINT/SIGTERM handlers for graceful shutdown (cancel TaskGroup, close pools).
- **Where**: `gold-agent/main.py`
- **Validated by**: Application starts all components successfully, SIGTERM triggers graceful shutdown, Postgres connection failure prevents startup
- **Estimate**: M

### ICT-20: Dockerfile and docker-compose Update
- **What**: Create `gold-agent/Dockerfile` (Python 3.12-slim, install requirements, copy source, `CMD ["python", "main.py"]`). Update `docker-compose.yml` to add a `gold-agent` service replacing (or alongside) `gold-backend`, using the same env file and depending on `postgres` and `redis`.
- **Where**: `gold-agent/Dockerfile`, `docker-compose.yml`
- **Validated by**: Application starts all components successfully
- **Estimate**: S

---

## Open questions

None carried from spec.

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
