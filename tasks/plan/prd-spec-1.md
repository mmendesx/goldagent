# PRD: Gold Trading Agent

**Spec**: `tasks/specs/spec-1-gold-trading-agent.md`
**Date**: 2026-04-16
**Status**: Ready for implementation

---

## BDD Scenarios

### Domain: Market Data Ingestion

#### S-1: Binance WebSocket connection and kline streaming
```
Given the Gold backend is running with BTCUSDT configured
When the Binance WebSocket stream connects successfully
Then the backend receives kline updates for BTCUSDT at the configured interval
And each kline is cached in Redis with key "candle:BTCUSDT:5m:latest"
And each closed kline is persisted to Postgres
```

#### S-2: Automatic reconnection on WebSocket disconnect
```
Given the Binance WebSocket connection is active
When the connection drops unexpectedly
Then the backend retries with exponential backoff (1s, 2s, 4s, 8s... max 60s)
And resumes data ingestion from the reconnection point
And logs a warning with the disconnection reason
```

#### S-3: Multi-symbol subscription
```
Given symbols BTCUSDT, ETHUSDT, SOLUSDT, BNBUSDT are configured
When the backend starts
Then it subscribes to kline and trade streams for all four symbols
And each symbol's data flows independently through the pipeline
```

#### S-4: Polymarket real-time data connection
```
Given valid Polymarket CLOB credentials are configured
When the backend connects to the Polymarket real-time data client
Then it receives activity and crypto price updates
And stores relevant market events in Postgres
```

#### S-5: Redis hot cache reflects latest market state
```
Given the backend is receiving kline data for BTCUSDT:5m
When a new candle update arrives
Then Redis key "candle:BTCUSDT:5m:latest" is updated within 10ms
And the ticker cache reflects the latest price within 10ms
```

---

### Domain: Technical Analysis

#### S-6: Indicator computation on candle close
```
Given 200 historical candles exist for BTCUSDT:5m
When a new 5m candle closes
Then RSI(14), MACD(12,26,9), Bollinger Bands(20,2), EMA(9,21,50,200), VWAP, and ATR(14) are computed
And the computed values are stored alongside the candle in Postgres
And the computation completes within 20ms
```

#### S-7: Candlestick pattern detection
```
Given the last 5 candles for BTCUSDT:5m are available
When the pattern detector runs
Then it identifies any matching patterns (engulfing, doji, hammer, shooting star, morning/evening star)
And returns a list of detected patterns with confidence scores
```

#### S-8: Support and resistance detection
```
Given at least 100 historical candles for a symbol
When the chart pattern analyzer runs
Then it identifies support and resistance levels based on swing highs/lows
And detects breakouts when price crosses these levels with volume confirmation
```

---

### Domain: Sentiment Analysis

#### S-9: News ingestion and sentiment scoring
```
Given a CryptoPanic API key is configured
When new crypto news articles are fetched
Then each article is scored for sentiment (positive/negative/neutral) with a confidence value
And the raw article and computed score are stored in Postgres
And the sentiment is associated with relevant asset symbols
```

#### S-10: Sentiment influence on decisions
```
Given the sentiment weight is configured at 0.3
When the decision engine evaluates BTCUSDT
Then the sentiment score contributes 30% to the composite signal
And a strongly negative sentiment can suppress a technical BUY signal below the confidence threshold
```

---

### Domain: Decision Engine

#### S-11: Composite signal generation
```
Given technical indicators, pattern detections, and sentiment scores are available for BTCUSDT
When the decision engine evaluates the current state
Then it produces a BUY, SELL, or HOLD decision with a confidence score (0-100)
And logs the decision with all contributing signal values to Postgres
```

#### S-12: Confidence threshold enforcement
```
Given the minimum confidence threshold is 70
When the decision engine produces a BUY with confidence 65
Then no trade is executed
And the decision is still logged with reason "below_confidence_threshold"
```

#### S-13: Position limit enforcement
```
Given max concurrent positions is 3 and 3 positions are already open
When the decision engine produces a BUY signal with confidence 90
Then no trade is executed
And the decision is logged with reason "max_positions_reached"
```

#### S-14: Dry-run mode
```
Given dry-run mode is enabled in configuration
When the decision engine produces a BUY signal above threshold
Then the decision is logged as "dry_run"
And no order is sent to any exchange
And the position is tracked as a simulated position
```

---

### Domain: Trade Execution

#### S-15: Binance spot market order execution
```
Given a BUY decision for BTCUSDT with confidence 85
When the execution engine places the order
Then a signed market order is sent via Binance WebSocket API
And the order response (filled price, quantity, fees) is recorded in Postgres
And the position is created with computed TP and SL levels
```

#### S-16: Take-profit exit
```
Given an open LONG position for BTCUSDT with TP at $65,000
When the current price reaches $65,000
Then a SELL market order is executed automatically
And the position is closed with status "TAKE_PROFIT"
And realized P&L is calculated and stored
```

#### S-17: Stop-loss exit
```
Given an open LONG position for BTCUSDT with SL at $60,000
When the current price drops to $60,000
Then a SELL market order is executed automatically
And the position is closed with status "STOP_LOSS"
And realized P&L is calculated and stored
```

#### S-18: Trailing stop adjustment
```
Given an open LONG position for BTCUSDT with trail distance of 1x ATR ($500)
And the current SL is at $62,000
When the price rises to $63,500
Then the SL is moved up to $63,000 ($63,500 - $500)
And the SL adjustment is logged
```

#### S-19: Trailing stop trigger
```
Given an open LONG position with trailing SL at $63,000
When the price drops from $63,400 to $63,000
Then a SELL market order is executed
And the position is closed with status "TRAILING_STOP"
```

#### S-20: Order rejection handling
```
Given the execution engine sends an order to Binance
When Binance rejects the order (insufficient balance, invalid symbol, etc.)
Then the rejection reason is logged
And the position is NOT created
And the decision is updated with execution status "rejected"
```

---

### Domain: Portfolio and Risk Management

#### S-21: Real-time balance tracking
```
Given the backend is running with open positions
When a position is closed with profit
Then the balance is updated immediately
And peak balance is updated if current > previous peak
And drawdown is recalculated as percentage from peak
```

#### S-22: Drawdown circuit breaker
```
Given the max drawdown threshold is 15%
When the current drawdown reaches 15%
Then all new trade execution is halted
And a critical alert is logged
And existing positions retain their TP/SL (not forcefully closed)
```

#### S-23: Portfolio metrics calculation
```
Given 50 closed trades exist in the database
When the metrics endpoint is called
Then it returns: win rate, profit factor, average win, average loss, Sharpe ratio, total P&L, max drawdown
And all monetary values use decimal precision
```

---

### Domain: REST API

#### S-24: Historical candles endpoint
```
Given 1000 candles exist for BTCUSDT:5m in Postgres
When GET /api/v1/candles?symbol=BTCUSDT&interval=5m&from=2026-04-15T00:00:00Z&to=2026-04-16T00:00:00Z is called
Then it returns candles within the time range in chronological order
And each candle includes OHLCV data and computed indicator values
And the response is paginated
```

#### S-25: Open positions endpoint
```
Given 2 open positions exist
When GET /api/v1/positions is called
Then it returns both positions with: symbol, side, entry price, current price, unrealized P&L, SL, TP
And unrealized P&L is computed using the latest cached price from Redis
```

#### S-26: Dashboard WebSocket stream
```
Given the dashboard connects to /ws/v1/stream
When new candle data, position updates, or metric changes occur
Then the dashboard receives real-time JSON messages categorized by type
And messages include: candle_update, position_update, metric_update, trade_executed, decision_made
```

---

### Domain: Dashboard

#### S-27: Metrics bar real-time updates
```
Given the dashboard is connected via WebSocket
When a trade closes and metrics change
Then the metrics bar updates Balance, Peak Balance, Drawdown, Win Rate, Total Trades, Open Positions
And Drawdown text is green (<5%), yellow (5-10%), or red (>10%)
And updates appear without page refresh
```

#### S-28: Candlestick chart with trade markers
```
Given historical candles and trades exist for BTCUSDT:5m
When the user views the chart for BTCUSDT at 5m interval
Then candlesticks render with OHLC data and volume
And SHORT entries show as red down arrows
And TAKE_PROFIT exits show as green up arrows
And STOP_LOSS exits show as red circles
And TRAILING_STOP exits show as red circles
And OPEN positions show as yellow down arrows
```

#### S-29: Symbol switching
```
Given the dashboard shows BTCUSDT chart
When the user selects ETHUSDT from the symbol selector
Then the chart loads ETHUSDT historical candles
And real-time updates switch to ETHUSDT
And trade markers update to show ETHUSDT trades only
```

#### S-30: Interval switching
```
Given the dashboard shows BTCUSDT at 5m interval
When the user clicks the "1h" interval button
Then historical candles reload at 1h interval
And real-time updates switch to 1h kline stream
And the "1h" button appears selected/active
```

#### S-31: Open positions panel with live P&L
```
Given 2 positions are open (BTCUSDT LONG, ETHUSDT LONG)
When the dashboard renders the Open Positions tab
Then each row shows: Symbol, Side, Entry Price, Current Price, Unrealized P&L, SL, TP
And Unrealized P&L updates in real time as prices change
And positive P&L is green, negative P&L is red
```

#### S-32: Trade history with filtering
```
Given 100 closed trades exist
When the user views the Trade History tab
Then trades are listed in reverse chronological order
And the user can filter by symbol
And each row shows: Date, Symbol, Side, Entry Price, Exit Price, P&L, Exit Reason
```

#### S-33: Tab navigation
```
Given the dashboard is loaded
When the user clicks between Chart, Open Positions, Trade History, and Decision Log tabs
Then the corresponding panel renders without page reload
And only one tab is active at a time
```

---

## Implementation Tasks (ICT)

### Phase 1: Foundation (Infrastructure + Data Layer)

#### ICT-1: Project scaffolding and Docker Compose
**Size**: M
**Depends on**: None
**Scenarios**: —
**Description**: Initialize Go module (`gold-backend`), create directory structure per spec section 4.1. Initialize Vite + React + TypeScript project (`gold-dashboard`). Create `docker-compose.yml` with Postgres 16, Redis 7, and backend/frontend services. Add `.env.example` with all required configuration variables.

**Acceptance**:
- `docker-compose up` starts all services
- Go backend compiles and starts (empty HTTP server on :8080)
- Frontend dev server runs on :3000
- Postgres and Redis are accessible from backend

---

#### ICT-2: Database schema and migrations
**Size**: M
**Depends on**: ICT-1
**Scenarios**: S-6, S-9, S-11, S-15, S-21, S-24
**Description**: Create migration files for all Postgres tables: `candles` (partitioned by symbol), `indicators`, `decisions`, `orders`, `positions`, `portfolio_snapshots`, `news_articles`, `sentiment_scores`. Use `golang-migrate`. Include indexes for common query patterns (symbol+interval+timestamp, position status, trade date).

**Acceptance**:
- Migrations run cleanly on fresh database
- Migrations are reversible (up/down)
- Schema supports all data described in the spec

---

#### ICT-3: Configuration and environment management
**Size**: S
**Depends on**: ICT-1
**Scenarios**: S-10, S-12, S-13, S-14
**Description**: Create `internal/config/` package. Load configuration from environment variables with sensible defaults. Struct fields: Binance API key/secret, Polymarket credentials, symbol list, intervals, indicator parameters, confidence threshold, max positions, max position size percentage, max drawdown, dry-run flag, sentiment weight, database/Redis URLs.

**Acceptance**:
- Config loads from `.env` file and environment variables
- Missing required values (API keys) cause a clear error at startup
- All configurable values from the spec are represented

---

#### ICT-4: Redis cache layer
**Size**: S
**Depends on**: ICT-1
**Scenarios**: S-5, S-7, S-25
**Description**: Create `internal/storage/redis/` package. Implement cache operations: set/get latest candle per symbol+interval, set/get current positions, set/get portfolio metrics. Use structured key naming: `candle:{symbol}:{interval}:latest`, `positions:open`, `metrics:portfolio`.

**Acceptance**:
- Cache reads return within 1ms
- Cache entries have configurable TTL
- Serialization uses JSON

---

#### ICT-5: Postgres repository layer
**Size**: L
**Depends on**: ICT-2
**Scenarios**: S-6, S-9, S-11, S-15, S-21, S-24, S-25
**Description**: Create `internal/storage/postgres/` package. Implement repository interfaces and pgx implementations for: `CandleRepository`, `IndicatorRepository`, `DecisionRepository`, `OrderRepository`, `PositionRepository`, `PortfolioRepository`, `NewsRepository`, `SentimentRepository`. Use `shopspring/decimal` for all monetary/price fields. Batch insert support for candles.

**Acceptance**:
- All repositories implement their interfaces
- Monetary values use decimal types
- Batch inserts work for candle data
- Queries use prepared statements

---

### Phase 2: Market Data Pipeline

#### ICT-6: Binance WebSocket stream client
**Size**: L
**Depends on**: ICT-3, ICT-4
**Scenarios**: S-1, S-2, S-3, S-5, S-6
**Description**: Create `internal/exchange/binance/` package. Implement WebSocket client that: connects to `wss://stream.binance.com:9443`, subscribes to `{symbol}@kline_{interval}` and `{symbol}@trade` for configured symbols, parses kline and trade messages, publishes parsed data to Go channels, handles ping/pong, implements reconnection with exponential backoff (1s to 60s). Respect rate limits (max 5 messages/sec, max 1024 streams/connection).

**Acceptance**:
- Connects and receives real-time kline data
- Reconnects automatically on disconnect
- Parsed candles flow through Go channels
- Ping/pong keeps connection alive
- Logs connection events

---

#### ICT-7: Polymarket real-time data client
**Size**: M
**Depends on**: ICT-3, ICT-4
**Scenarios**: S-4
**Description**: Create `internal/exchange/polymarket/` package. Implement WebSocket client that connects to Polymarket's real-time data service, subscribes to activity and crypto price topics, parses incoming messages, publishes to Go channels. Handle authentication with CLOB API credentials for user-specific streams.

**Acceptance**:
- Connects and receives market activity data
- Authenticates for user-specific streams
- Parsed events flow through Go channels
- Reconnection on disconnect

---

#### ICT-8: Candle aggregation and storage service
**Size**: M
**Depends on**: ICT-5, ICT-6
**Scenarios**: S-1, S-5, S-6
**Description**: Create `internal/market/candle/` package. Service that: receives raw kline updates from Binance channel, detects candle close events, persists closed candles to Postgres via repository, updates Redis hot cache with latest candle, emits candle-close events for the analysis engine. Support multiple concurrent symbols without blocking.

**Acceptance**:
- Closed candles are persisted to Postgres
- Latest candle is always in Redis
- Handles multiple symbols concurrently
- Candle close events trigger downstream processing

---

### Phase 3: Analysis Engine

#### ICT-9: Technical indicator computation
**Size**: L
**Depends on**: ICT-8
**Scenarios**: S-6
**Description**: Create `internal/analysis/indicator/` package. Implement pure functions for: RSI(period), MACD(fast, slow, signal), Bollinger Bands(period, stddev), EMA(period), VWAP, ATR(period). Each function takes a slice of candles and returns computed values. Indicator computer service subscribes to candle-close events and computes all indicators, storing results via repository.

**Acceptance**:
- Each indicator function is pure (deterministic given inputs)
- Unit tests validate indicator calculations against known values
- Indicator values are stored in Postgres linked to candle timestamps
- Computation of all indicators completes within 20ms for 200 candles

---

#### ICT-10: Candlestick pattern detection
**Size**: M
**Depends on**: ICT-8
**Scenarios**: S-7
**Description**: Create `internal/analysis/pattern/` package with candlestick sub-package. Detect patterns from recent candles: bullish/bearish engulfing, doji, hammer, inverted hammer, shooting star, morning star, evening star. Return detected patterns with pattern name and confidence score (0-100).

**Acceptance**:
- Detects all listed patterns correctly
- Pure functions with unit tests against known candle sequences
- Returns structured results (pattern name, direction, confidence)

---

#### ICT-11: Chart pattern and support/resistance detection
**Size**: L
**Depends on**: ICT-8
**Scenarios**: S-8
**Description**: Create chart pattern sub-package under `internal/analysis/pattern/`. Detect: support/resistance levels from swing highs/lows (pivot points), trend lines, breakout events (price crossing S/R with volume spike), double top/bottom formations. Use a lookback window of configurable candle count.

**Acceptance**:
- Identifies S/R levels from pivot points
- Detects breakouts with volume confirmation
- Pure functions with tests against known chart data

---

#### ICT-12: News ingestion and sentiment analysis
**Size**: L
**Depends on**: ICT-3, ICT-5
**Scenarios**: S-9, S-10
**Description**: Create `internal/analysis/sentiment/` package. News fetcher polls CryptoPanic API on configurable interval (default 60s). Sentiment scorer uses Claude API (Anthropic SDK) to score each article for sentiment per asset (positive/neutral/negative with confidence 0-1). Store raw articles and scores in Postgres. Expose latest sentiment score per symbol for the decision engine.

**Acceptance**:
- Fetches news from configured API
- Scores each article using LLM
- Stores articles and scores in Postgres
- Returns aggregated sentiment per symbol

---

### Phase 4: Decision and Execution

#### ICT-13: Decision engine
**Size**: L
**Depends on**: ICT-9, ICT-10, ICT-11, ICT-12, ICT-3
**Scenarios**: S-11, S-12, S-13, S-14
**Description**: Create `internal/engine/` package. The decision engine: collects latest indicator values, pattern detections, and sentiment scores for each symbol; applies weighted scoring to produce a composite signal; generates BUY/SELL/HOLD with confidence (0-100); enforces confidence threshold; checks position limits; respects dry-run mode; logs every decision to Postgres. The engine runs on each candle-close event.

**Acceptance**:
- Produces deterministic decisions given the same inputs (pure function core)
- Respects confidence threshold
- Respects max position limits
- Logs all decisions regardless of execution
- Dry-run mode prevents trade execution but still logs

---

#### ICT-14: Binance order execution
**Size**: L
**Depends on**: ICT-6, ICT-5
**Scenarios**: S-15, S-16, S-17, S-19, S-20
**Description**: Create `internal/execution/` package. Order executor: sends signed market orders via Binance WebSocket API (`order.place` method); parses fill responses; creates position records with computed TP/SL levels (based on ATR from indicators or fixed percentage fallback); handles order rejections and partial fills; records all order events in Postgres.

**Acceptance**:
- Places signed orders via Binance WS API
- Records order fills with price, quantity, fees
- Creates positions with TP and SL levels
- Handles rejections gracefully (no orphaned positions)

---

#### ICT-15: Position monitor with TP/SL/trailing stop
**Size**: L
**Depends on**: ICT-14, ICT-4
**Scenarios**: S-16, S-17, S-18, S-19
**Description**: Extend execution package with position monitor goroutine. For each open position: compare current price (from Redis) against TP, SL, and trailing stop; trigger exit order when any condition is met; adjust trailing SL when price moves favorably (new_sl = current_price - trail_distance, only if > current_sl). Update position status in Postgres and Redis on every change.

**Acceptance**:
- TP triggers sell when price >= TP level
- SL triggers sell when price <= SL level
- Trailing stop moves SL up as price rises
- Trailing stop triggers sell when price reverses to trailing SL
- Position closed with correct status (TAKE_PROFIT, STOP_LOSS, TRAILING_STOP)

---

#### ICT-16: Portfolio manager and risk controls
**Size**: M
**Depends on**: ICT-14, ICT-5, ICT-4
**Scenarios**: S-21, S-22, S-23
**Description**: Create `internal/portfolio/` package. Track: current balance, peak balance, drawdown percentage, win rate, profit factor, average win/loss, Sharpe ratio, total P&L. Update on every trade close. Store snapshots in Postgres. Cache current metrics in Redis. Implement circuit breaker: halt new trades when drawdown >= configured max.

**Acceptance**:
- Balance updates on every closed trade
- Drawdown calculates correctly from peak
- Circuit breaker halts trading at max drawdown
- All metrics compute correctly
- Metrics cached in Redis for fast API access

---

### Phase 5: API Layer

#### ICT-17: REST API endpoints
**Size**: M
**Depends on**: ICT-5, ICT-4, ICT-16
**Scenarios**: S-24, S-25, S-23
**Description**: Create `internal/api/rest/` package using `go-chi/chi`. Implement all endpoints from FR-8: candles (with indicator data), positions (open + history), trades, metrics, decisions. Paginate list endpoints. Use JSON responses. Add CORS headers for dashboard access.

**Acceptance**:
- All 6 REST endpoints respond correctly
- Pagination works with page+limit parameters
- Monetary values serialize as strings (decimal precision)
- CORS allows dashboard origin

---

#### ICT-18: Dashboard WebSocket hub
**Size**: M
**Depends on**: ICT-8, ICT-14, ICT-16
**Scenarios**: S-26, S-27
**Description**: Create `internal/api/websocket/` package. WebSocket hub that: accepts dashboard connections on `/ws/v1/stream`, broadcasts real-time events (candle updates, position changes, metric updates, trade executions, decisions), supports subscription filtering by symbol, handles multiple concurrent dashboard clients.

**Acceptance**:
- Dashboard connects and receives real-time events
- Events are categorized by type in JSON messages
- Multiple clients receive broadcasts simultaneously
- Clean disconnect handling

---

### Phase 6: Dashboard

#### ICT-19: Dashboard scaffolding and layout
**Size**: M
**Depends on**: ICT-1
**Scenarios**: S-33, S-27
**Description**: Set up `gold-dashboard` with Vite + React + TypeScript (ES2022 target, strict mode). Install dependencies: `lightweight-charts`, `animate-ui`, `react-bits`, state management library. Create main layout: MetricsBar at top, tab navigation below (Chart, Open Positions, Trade History, Decision Log), content area. Dark theme. Responsive.

**Acceptance**:
- App renders with dark theme
- Tab navigation works (URL-based routing)
- MetricsBar component renders placeholder values
- Responsive on desktop and tablet widths

---

#### ICT-20: WebSocket client and state management
**Size**: M
**Depends on**: ICT-18, ICT-19
**Scenarios**: S-26, S-27, S-31
**Description**: Create `src/api/` with WebSocket client that connects to backend `/ws/v1/stream`. Create state store (zustand/jotai) for: candles, positions, metrics, trades, decisions. WebSocket messages dispatch to store. Create `useWebSocket` hook for connection lifecycle. Create REST client for initial data fetching.

**Acceptance**:
- WebSocket connects and receives updates
- State store updates on each message
- Components re-render on state changes
- REST client fetches historical data on mount

---

#### ICT-21: Metrics bar component
**Size**: S
**Depends on**: ICT-20
**Scenarios**: S-27
**Description**: Create `MetricsBar` component. Display: Balance (formatted currency), Peak Balance, Drawdown (percentage with color coding: green <5%, yellow 5-10%, red >10%), Win Rate (percentage), Total Trades (count), Open Positions (count). All values from state store, updated in real time.

**Acceptance**:
- All 6 metrics display correctly
- Drawdown color changes based on severity
- Values update in real time without flicker
- Responsive layout (wraps on smaller screens)

---

#### ICT-22: Price chart with TradingView Lightweight Charts
**Size**: L
**Depends on**: ICT-20
**Scenarios**: S-28, S-29, S-30
**Description**: Create `PriceChart` component wrapping `lightweight-charts`. Render candlestick series with volume histogram. Load historical candles via REST API on mount. Subscribe to real-time candle updates via state store. Implement `SymbolSelector` dropdown and `IntervalButtons` (1m, 5m, 15m, 1h, 4h, 1D). On symbol/interval change, fetch new history and resubscribe.

**Acceptance**:
- Candlestick chart renders with OHLCV data
- Real-time candles append/update on the chart
- Symbol selector switches data source
- Interval buttons switch timeframe
- Chart auto-scrolls to latest candle

---

#### ICT-23: Trade markers on chart
**Size**: M
**Depends on**: ICT-22
**Scenarios**: S-28
**Description**: Extend `PriceChart` to overlay trade markers using Lightweight Charts marker API. Map position data to markers: SHORT entry = red down arrow (`▼ SHORT`), TAKE_PROFIT exit = green up arrow (`▲ TAKE_PROFIT`), STOP_LOSS exit = red circle (`● STOP_LOSS`), TRAILING_STOP exit = red circle (`● TRAILING_STOP`), OPEN position = yellow down arrow (`▼ OPEN`). Markers update in real time as positions open/close.

**Acceptance**:
- All 5 marker types render at correct candle timestamps
- Colors match spec (red, green, yellow)
- Markers update when new trades/exits occur
- Markers filter to selected symbol

---

#### ICT-24: Open positions panel
**Size**: M
**Depends on**: ICT-20
**Scenarios**: S-31
**Description**: Create `OpenPositions` component. Table with columns: Symbol, Side, Entry Price, Current Price, Unrealized P&L, SL, TP. Current Price and Unrealized P&L update in real time from state store. P&L is green when positive, red when negative. Empty state message when no positions.

**Acceptance**:
- Table renders all open positions
- P&L updates in real time
- Color coding for positive/negative P&L
- Empty state displays when no positions

---

#### ICT-25: Trade history panel
**Size**: M
**Depends on**: ICT-20
**Scenarios**: S-32
**Description**: Create `TradeHistory` component. Table with columns: Date, Symbol, Side, Entry Price, Exit Price, P&L, Exit Reason. Load from REST API with pagination. Filter by symbol. Sort by date (newest first by default). P&L color-coded green/red.

**Acceptance**:
- Loads trade history with pagination
- Symbol filter works
- Chronological sorting (newest first)
- P&L color coding

---

#### ICT-26: Decision log panel
**Size**: S
**Depends on**: ICT-20
**Scenarios**: S-33
**Description**: Create `DecisionLog` component. Table with columns: Timestamp, Symbol, Decision (BUY/SELL/HOLD), Confidence, Top Signals, Executed (yes/no), Reason (if not executed). Load from REST API with pagination. Color-code decisions: BUY green, SELL red, HOLD grey.

**Acceptance**:
- Loads decisions with pagination
- Decision color coding
- Shows execution status and reason
- Real-time updates for new decisions

---

### Phase 7: Integration and Polish

#### ICT-27: End-to-end integration testing
**Size**: L
**Depends on**: ICT-13, ICT-14, ICT-15, ICT-16, ICT-17, ICT-18
**Scenarios**: S-1 through S-26
**Description**: Write integration tests that verify the full pipeline: mock Binance WebSocket → candle ingestion → indicator computation → decision engine → order execution (mocked) → position management → portfolio metrics → API responses. Use Docker Compose test environment with real Postgres and Redis.

**Acceptance**:
- Full pipeline executes end-to-end with mocked exchange
- Decisions produce expected trades for known market scenarios
- Portfolio metrics are accurate after a series of trades
- API endpoints return correct data

---

#### ICT-28: Dashboard integration testing
**Size**: M
**Depends on**: ICT-19 through ICT-26
**Scenarios**: S-27 through S-33
**Description**: Verify dashboard against running backend: metrics bar updates, chart renders with markers, positions panel shows live P&L, trade history loads and filters, decision log updates. Manual verification checklist + automated component tests.

**Acceptance**:
- All dashboard features work against live backend
- Real-time updates propagate within 1 second
- No console errors
- Dark theme consistent across all components

---

#### ICT-29: Documentation and deployment guide
**Size**: S
**Depends on**: ICT-27, ICT-28
**Scenarios**: —
**Description**: Update README with: architecture overview, setup instructions, configuration reference (all env vars), development workflow, deployment instructions. Add inline code comments only where logic is non-obvious.

**Acceptance**:
- A new developer can set up and run the project following the README
- All configuration options are documented
- Architecture diagram matches implementation

---

## Task Summary

| Phase | Tasks | Sizes |
|-------|-------|-------|
| 1. Foundation | ICT-1 through ICT-5 | 1L, 2M, 2S |
| 2. Market Data | ICT-6 through ICT-8 | 1L, 2M |
| 3. Analysis | ICT-9 through ICT-12 | 3L, 1M |
| 4. Decision & Execution | ICT-13 through ICT-16 | 3L, 1M |
| 5. API | ICT-17, ICT-18 | 2M |
| 6. Dashboard | ICT-19 through ICT-26 | 1L, 5M, 2S |
| 7. Integration | ICT-27 through ICT-29 | 1L, 1M, 1S |
| **Total** | **29 tasks** | **10L, 13M, 5S** |

## Dependency Graph (Critical Path)

```
ICT-1 ──▶ ICT-2 ──▶ ICT-5 ──▶ ICT-8 ──▶ ICT-9 ──┐
  │         │                     │        ICT-10 ──┤
  │         │                     │        ICT-11 ──┼──▶ ICT-13 ──▶ ICT-14 ──▶ ICT-15 ──▶ ICT-27
  │         │                     │                 │                  │
  ├──▶ ICT-3 ──▶ ICT-6 ──────────┘     ICT-12 ────┘       ICT-16 ───┘
  │         │                                                  │
  │         ├──▶ ICT-7                                    ICT-17 ──▶ ICT-18 ──▶ ICT-28
  │         │                                                              │
  ├──▶ ICT-4                                                          ICT-20 ──▶ ICT-21
  │                                                                     │   ──▶ ICT-22 ──▶ ICT-23
  └──▶ ICT-19 ─────────────────────────────────────────────────────────┘   ──▶ ICT-24
                                                                            ──▶ ICT-25
                                                                            ──▶ ICT-26
```

**Critical path**: ICT-1 → ICT-2 → ICT-5 → ICT-8 → ICT-9 → ICT-13 → ICT-14 → ICT-15 → ICT-27
