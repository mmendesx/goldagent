# Spec: Gold Dashboard — Ultimate Trading UI Redesign

## Overview

Ground-up redesign of `gold-dashboard` to deliver a production-grade trading UI. Current dashboard has broken pagination (frontend expects `{items,hasMore}` envelopes; backend returns plain arrays), a missing `/api/v1/positions/history` endpoint, synthetic `currentPrice`/`unrealizedPnl` on open positions, a non-live price tick channel, and a thin design system. This spec aligns frontend and backend contracts, adds a live ticker WS channel, introduces a proper design-token system, rewrites key views for density + clarity, and standardizes loading / empty / error states across every surface.

## Actors

- **Trader** — monitors the agent's live activity and portfolio health in real time
- **Backend agent** — emits REST responses + WebSocket events consumed by the dashboard
- **Frontend dashboard** — single-page React app rendering charts, tables, and metrics

---

## Functional Requirements

### FR-1: Unified pagination envelope on list endpoints

All list endpoints return a consistent pagination envelope instead of bare arrays:

```json
{ "items": [...], "limit": 50, "offset": 0, "count": <totalKnown>, "hasMore": <bool> }
```

Applies to: `/api/v1/candles`, `/api/v1/trades`, `/api/v1/decisions`, `/api/v1/positions/history` (new), `/api/v1/positions` (when `status` absent).

### FR-2: `/api/v1/positions/history` endpoint

New endpoint returning closed positions with pagination. Query params: `symbol?`, `limit` (default 50, max 500), `offset` (default 0). Response uses FR-1 envelope with `Position[]`. Orders DESC by `closedAt`.

### FR-3: Live ticker WebSocket channel

Backend publishes `ticker_update` events from the miniTicker stream already consumed by `CandleAggregator`. Payload: `{ symbol: string, price: string, timestamp: string (ISO8601) }`. Emitted per miniTicker callback, throttled to ≤1 Hz per symbol to cap fan-out. Frontend uses these ticks to compute live `currentPrice` and `unrealizedPnl` on open positions.

### FR-4: Decision payload includes `reasoning`

`decision_made` WebSocket event and `GET /api/v1/decisions` include the `reasoning` field (nullable string). Backend already stores it; dashboard renders it in an expandable row.

### FR-5: Dashboard design token system

Centralized token layer in `src/styles/tokens.css` covering color (bg, surface, text, accent, status, chart), spacing (4/8/12/16/24/32/48), radius, shadow, font scale, z-index. All component CSS consumes tokens — no hard-coded hex values. Dark theme only (v1); structure allows light theme swap in future.

### FR-6: Global loading / empty / error states

Every async surface (charts, tables, metric cards, balance cards, decision rows) exposes three explicit states:

- **Loading**: skeleton placeholder sized to final content, not a bare spinner
- **Empty**: domain-specific copy + recommended action ("No open positions. Agent is scanning…")
- **Error**: inline error with retry button; retry re-runs the originating fetch

A shared `<AsyncBoundary>` component enforces this contract.

### FR-7: Real-time P&L on open positions

`OpenPositions` computes `currentPrice` from the live ticker store (FR-3) and derives `unrealizedPnl` client-side:

- LONG: `(currentPrice - entryPrice) * quantity - fees`
- SHORT: `(entryPrice - currentPrice) * quantity - fees`

No 3 s REST polling. Updates on every `ticker_update` event. Color code positive/negative.

### FR-8: Synchronized price chart

`PriceChart` seeds from `/api/v1/candles?limit=500` on mount / symbol / interval change, then:

1. On `candle_update` for active symbol+interval → `appendOrUpdateCandle`
2. On `ticker_update` for active symbol → update live price line (dashed horizontal overlay) without mutating the last candle
3. Volume histogram below main chart, shares X axis

Intervals: `1m`, `5m`, `15m`, `1h`. Time zone: UTC with local tooltip formatting.

### FR-9: Exchange-scoped navigation

Top-level routes: `/binance/*`, `/polymarket/*`, default `/` → `/binance/chart`. Each view owns its sub-tab layout (per spec-4). Sub-tabs:

- Binance: `chart`, `positions`, `history`, `decisions`
- Polymarket: `chart`, `overview`, `decisions`

Active route persists across reloads (deep-linkable).

### FR-10: Metrics bar — always visible, exchange-agnostic

Header metrics bar displays 8 cards: **Balance**, **Peak**, **Drawdown %**, **Total P&L**, **Win Rate**, **Trades**, **Binance USDT**, **Polymarket USDC**. Balances animate on change (number transition 200 ms). Drawdown ≥10 % → warning color; ≥15 % → negative color + circuit-breaker badge.

### FR-11: Order entry surface (read-only v1)

Chart view shows a compact "Latest signal" panel rendering the most recent Decision with: symbol, action, confidence bar, composite score, dry-run badge, execution status, timestamp, reasoning (truncated w/ expand). Read-only — no order submission in v1.

### FR-12: Dashboard error boundary + offline banner

Top-level React error boundary catches render errors and shows a reload CTA. A global offline banner renders when `connectionState ∈ {closed, reconnecting}` for >3 s, with reconnect attempt count.

---

## Technical Requirements

### Architecture

```
gold-dashboard/src/
├── styles/
│   ├── tokens.css         NEW  — design tokens (colors, spacing, radius, etc.)
│   ├── reset.css          NEW  — minimal CSS reset
│   └── globals.css        NEW  — base typography, scrollbar, utility classes
├── api/
│   ├── restClient.ts      MOD  — pagination envelope, typed endpoints
│   └── websocketClient.ts MOD  — add `ticker_update` handler registry
├── store/
│   └── index.ts           MOD  — add tickersBySymbol, derive unrealizedPnl selector
├── hooks/
│   ├── useLiveTicker.ts   NEW  — subscribes to ticker_update for symbol
│   ├── useAsyncResource.ts NEW — wraps fetch → loading/error/data + retry
│   └── useWebSocketLifecycle.ts MOD — register ticker handler, drop 3s poll
├── components/
│   ├── AsyncBoundary/     NEW  — loading/empty/error wrapper
│   ├── Skeleton/          NEW  — skeleton primitives (text, block, table row)
│   ├── PriceChart/        REWRITE — live ticker overlay, volume pane, loading skeleton
│   ├── OpenPositions/     REWRITE — client-side P&L, live ticker binding
│   ├── TradeHistory/      REWRITE — pagination against new envelope
│   ├── DecisionLog/       REWRITE — expandable reasoning row
│   ├── LatestSignal/      NEW     — compact signal panel on chart view
│   ├── MetricsBar/        MOD     — animated number + drawdown severity
│   ├── ConnectionBadge/   MOD     — reconnect attempt count
│   └── OfflineBanner/     NEW     — top-bar offline/reconnecting strip
├── pages/Dashboard/       MOD  — error boundary wrap, offline banner slot
└── types/index.ts         MOD  — Paginated<T>, TickerUpdate, remove synthetic fields
```

Backend (`gold-agent/`):

```
api/
├── handlers.py            MOD  — paginate trades/decisions/candles/positions; add /positions/history
├── websocket_hub.py       MOD  — add publish_ticker(symbol, price, ts)
└── router.py              — no change
exchange/binance_stream.py MOD  — on_ticker callback publishes ticker_update via hub (throttled)
main.py                    MOD  — wire hub.publish_ticker as on_ticker callback
```

### Data model

No DB migrations. Store changes only:

```typescript
interface DashboardState {
  // ... existing
  tickersBySymbol: Record<string, { price: number; timestamp: string }>
  setTicker: (symbol: string, price: number, timestamp: string) => void
}
```

`OpenPositionWithLive` derived client-side from `Position` + `tickersBySymbol[symbol]`.

### API contracts

All responses remain `by_alias=True` camelCase.

| Method | Path | Query | Response |
|---|---|---|---|
| GET | `/api/v1/candles` | `symbol, interval, limit=100, offset=0` | `Paginated<Candle>` |
| GET | `/api/v1/positions` | `symbol?, status?` | `Position[]` (unchanged — no pagination needed; open set is small) |
| GET | `/api/v1/positions/history` **(new)** | `symbol?, limit=50, offset=0` | `Paginated<Position>` |
| GET | `/api/v1/trades` | `symbol?, limit=50, offset=0` | `Paginated<Position>` |
| GET | `/api/v1/decisions` | `symbol?, limit=50, offset=0` | `Paginated<Decision>` (with `reasoning`) |
| GET | `/api/v1/metrics` | — | `PortfolioMetrics` |
| GET | `/api/v1/exchange/balances` | — | `ExchangeBalances` |
| WS | `/ws/v1/stream` | — | events (see below) |

`Paginated<T>`: `{ items: T[], limit: number, offset: number, count: number, hasMore: boolean }`.

WebSocket event types (`type` field):

- `candle_update` → `Candle`
- `ticker_update` **(new)** → `{ symbol, price, timestamp }`
- `position_update` → `Position`
- `position_closed` → `Position`
- `metric_update` → `PortfolioMetrics`
- `decision_made` → `Decision` (includes `reasoning`)

### UI structure

Layout tree:

```
<App>
  <ErrorBoundary>
    <BrowserRouter>
      <Dashboard>
        <OfflineBanner />                 // conditional
        <Header>
          <MetricsBar />
          <ConnectionBadge />
        </Header>
        <ExchangeTabs />                  // /binance | /polymarket
        <main>
          <Routes>
            <Route path="/binance/*"   element={<BinanceView />} />
            <Route path="/polymarket/*" element={<PolymarketView />} />
          </Routes>
        </main>
      </Dashboard>
    </BrowserRouter>
  </ErrorBoundary>
</App>
```

Component contracts:

- `<AsyncBoundary state={loading|empty|error|ready} onRetry={fn}>…</AsyncBoundary>` — renders the appropriate shell; children render only in `ready`.
- `<Skeleton variant="text|block|row" w h />`
- `<LatestSignal decision={Decision|null} />`

### Infrastructure

No new external services. Existing Redis/Postgres/WebSocket stack sufficient.

---

## Non-functional Requirements

- **Latency**: ticker → chart live-price line repaint <100 ms from WS receipt
- **Smoothness**: no layout jank during updates — numeric cells fixed-width, monospace, pre-reserved space for sign + digits
- **A11y**: all interactive elements keyboard-reachable, focus ring consumes `--color-accent`; aria-live="polite" on connection badge and offline banner; table rows navigable with ↑/↓
- **Types**: strict TS; no `any`; paginated responses fully generic; remove synthetic fields from `OpenPositionWithLive`
- **Bundle**: no new heavyweight deps. Keep `lightweight-charts`. No Tailwind / CSS-in-JS additions
- **Error handling**: every REST call wrapped in typed error; UI shows inline retry, never a blank screen
- **Backwards compatibility**: old clients hitting bare-array endpoints break — acceptable; single consumer

---

## Dependencies

| Dependency | Status |
|-----------|--------|
| spec-4 exchange-tab layout | Merged |
| `lightweight-charts` for chart | Installed |
| `react-router-dom` v7 | Installed |
| `zustand` store | Installed |
| Binance miniTicker stream (already subscribed) | Available |

---

## Constraints

- No new REST framework or state lib
- No light theme in v1 (tokens must support it; UI renders dark only)
- No mobile breakpoint work in v1 — desktop trading-desk target (≥1280 px)
- No order submission UI in v1 (read-only dashboard)
- Existing symbol universe fixed: BTCUSDT / ETHUSDT / SOLUSDT / BNBUSDT

---

## Open Questions

1. **Ticker throttle rate** — 1 Hz per symbol proposed; acceptable, or finer (e.g. 4 Hz for active-symbol)? **Default**: 1 Hz server-side.
2. **Reasoning truncation length** — 240 chars then expand? **Default**: 240.
3. **INTEGRATION_CHECKLIST.md** references Go backend and obsolete endpoints. Rewrite in same PR, or track separately? **Default**: rewrite in this scope as final ICT.

---

## Glossary

- **Ticker** — Binance miniTicker event carrying latest trade price for a symbol
- **Paginated envelope** — `{ items, limit, offset, count, hasMore }` response shape
- **AsyncBoundary** — React component enforcing loading/empty/error/ready display contract
