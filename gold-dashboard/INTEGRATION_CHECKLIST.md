# Integration Checklist — Gold Dashboard

Manual verification guide for validating the dashboard end-to-end against the live backend.

All commands run from the **monorepo root** (`/Users/mmendesx/workspace/goldagent`) unless noted.

---

## Quick Start

### 1. Environment setup

Copy the example env file and fill in your credentials:

```bash
cp .env.example .env
```

Required variables:

| Variable | Description |
|---|---|
| `BINANCE_API_KEY` | Binance API key |
| `BINANCE_API_SECRET` | Binance API secret |
| `POLYMARKET_API_KEY` | Polymarket API key |
| `POLYMARKET_API_SECRET` | Polymarket API secret |
| `POLYMARKET_API_PASSPHRASE` | Polymarket passphrase |
| `POLYMARKET_PRIVATE_KEY` | Polymarket private key |
| `POLYMARKET_WALLET_ADDRESS` | Polymarket wallet address |
| `GOLD_LLM_API_KEY` | Anthropic API key |

DB and Redis: the default Docker Compose values work out of the box — no changes needed unless you're running a custom setup.

### 2. Start all services

```bash
docker compose up -d
```

This starts the Python backend (FastAPI), PostgreSQL, and Redis in one command. Wait ~5 seconds for the backend to finish initializing.

### 3. Verify backend health

```bash
curl http://localhost:8080/api/v1/metrics
```

Expected: `200 OK` with a JSON body containing portfolio metrics fields. Any error here means the backend is not ready — check `docker compose logs` for details.

### 4. Start frontend dev server

```bash
cd gold-dashboard && npm run dev
```

Frontend available at `http://localhost:5173`.

---

## API Contracts

### Base URL

`http://localhost:8080`

### Paginated response envelope

Endpoints that return lists use this shape:

```json
{
  "items": [],
  "limit": 50,
  "offset": 0,
  "count": 0,
  "hasMore": false
}
```

### REST endpoints

| Method | Path | Query params | Response shape |
|--------|------|--------------|----------------|
| GET | `/api/v1/candles` | `symbol`, `interval`, `limit`, `offset` | `Paginated<Candle>` |
| GET | `/api/v1/positions` | `status` (e.g. `OPEN`) | `Position[]` |
| GET | `/api/v1/positions/history` | `limit`, `offset` | `Paginated<Position>` |
| GET | `/api/v1/trades` | `limit`, `offset` | `Paginated<Position>` |
| GET | `/api/v1/decisions` | `limit`, `offset` | `Paginated<Decision>` |
| GET | `/api/v1/metrics` | — | `PortfolioMetrics` |
| GET | `/api/v1/exchange/balances` | — | `ExchangeBalances` |

#### Example candles request

```
GET /api/v1/candles?symbol=BTCUSDT&interval=5m&limit=500&offset=0
```

#### Example positions request

```
GET /api/v1/positions?status=OPEN
```

Response is a bare array (`Position[]`), not paginated.

#### Example paginated positions history request

```
GET /api/v1/positions/history?limit=50&offset=0
```

---

## WebSocket Events

### Connection

```
ws://localhost:8080/ws/v1/stream
```

All messages are JSON with a `type` field that identifies the event.

### Event types

| `type` value | Payload shape | Description |
|---|---|---|
| `candle_update` | `Candle` | New or updated candlestick |
| `ticker_update` | `{ symbol, price, timestamp }` | Real-time price tick |
| `position_update` | `Position` | Open position changed |
| `position_closed` | `Position` | Position has been closed |
| `metric_update` | `PortfolioMetrics` | Portfolio metrics recalculated |
| `decision_made` | `Decision` | Agent decision, includes reasoning field |

### Parsing example

```ts
socket.onmessage = (event) => {
  const msg = JSON.parse(event.data)
  switch (msg.type) {
    case 'ticker_update':
      // msg.payload: { symbol: string, price: number, timestamp: string }
      break
    case 'candle_update':
      // msg.payload: Candle
      break
    // ...
  }
}
```

---

## Frontend Integration

### Environment variables

Controlled by `gold-dashboard/.env.local` (create if it doesn't exist):

```
VITE_API_BASE_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080/ws/v1/stream
```

These default to the above values in development. Only override if the backend runs on a different host or port.

### Manual checks — Network tab (DevTools)

On dashboard load, confirm these fire with 200:

- [ ] `GET /api/v1/metrics` → `200`
- [ ] `GET /api/v1/positions?status=OPEN` → `200`
- [ ] `GET /api/v1/positions/history?limit=50&offset=0` → `200`

WebSocket tab: confirm a connection to `ws://localhost:8080/ws/v1/stream` with status `101 Switching Protocols`.

Console: no uncaught errors or exceptions on startup.

### Tab navigation checks

- [ ] **Chart** tab → URL `/chart`, chart renders
- [ ] **Open Positions** tab → URL `/positions`, table or empty state renders
- [ ] **Trade History** tab → URL `/history`, pagination visible (Previous disabled on page 1)
- [ ] **Decision Log** tab → URL `/decisions`, symbol filter dropdown present

### Chart tab

- [ ] Symbol selector shows: BTCUSDT, ETHUSDT, SOLUSDT, BNBUSDT
- [ ] Interval buttons visible: 1m, 5m, 15m, 1h — active one highlighted
- [ ] Switching symbol or interval fires a new `GET /api/v1/candles` request

Network check: `GET /api/v1/candles?symbol=BTCUSDT&interval=5m&limit=500&offset=0` returns `200` with `{ items: [], ... }` on a fresh database.

### Open Positions tab

- [ ] Renders without errors
- [ ] Empty state message shown when no positions exist

Network check: `GET /api/v1/positions?status=OPEN` returns `[]`.

### Trade History tab

- [ ] Renders without errors
- [ ] Empty state shown when no trades exist
- [ ] Pagination controls render; Previous is disabled on page 1

Network check: `GET /api/v1/positions/history?limit=50&offset=0` returns `{ items: [], hasMore: false, ... }`.

### Decision Log tab

- [ ] Renders without errors
- [ ] Symbol filter dropdown shows ALL, BTCUSDT, ETHUSDT, SOLUSDT, BNBUSDT

Network check: `GET /api/v1/decisions?limit=50&offset=0` returns `200`.

### Connection badge

- [ ] Shows green "Live" dot when backend is reachable
- [ ] Stop the backend (`docker compose stop`) → badge changes to "Offline" (red) or "Connecting…" (amber) within ~5 seconds
- [ ] Restart the backend (`docker compose start`) → badge returns to "Live" within ~30 seconds (exponential backoff), no page refresh required

---

## Troubleshooting

### Backend not responding on port 8080

Check container status and logs:

```bash
docker compose ps
docker compose logs --tail=50
```

Common causes: missing `.env` variables, port already in use, DB migration failed on first boot.

### WebSocket connection not establishing

1. Confirm backend is healthy: `curl http://localhost:8080/api/v1/metrics`
2. Check `VITE_WS_URL` in `gold-dashboard/.env.local` matches the backend address
3. Inspect the WS tab in DevTools for the close code and reason

### Candles endpoint returns empty items on a live system

The backend only stores candles it has observed since startup. Wait for a few minutes of uptime, or check that the Binance API key is valid and not rate-limited.

### Frontend shows stale data after backend restart

The WebSocket reconnects automatically (exponential backoff, up to ~30 seconds). REST data refreshes on the next poll or user-triggered navigation. A manual page refresh forces a clean state.

### Paginated endpoint returns unexpected shape

All list endpoints except `GET /api/v1/positions` return the paginated envelope. If a consumer expects a bare array from `/api/v1/positions/history` or `/api/v1/trades`, it will receive an object — read `items` from the response.

---

## Spec-6: Dashboard Pro Redesign + WCAG 2.1 AA

### Design System
- **Tokens**: `src/styles/tokens.css` — two-theme (dark/light) CSS custom properties. All components use `var(--token)` references. Backward-compat aliases keep existing components working during migration.
- **Primitives**: `src/design-system/` — Button, Card, Badge, Skeleton/SkeletonContainer, VisuallyHidden, SkipLink, ThemeToggle
- **Composition**: `src/design-system/` — PageShell, AnimatedTabs, MetricCard, LiveNumber

### Vendored Libraries
- **animate-ui**: `src/vendor/animate-ui/` — Tabs (Radix), Fade/Fades, Highlight, SlidingNumber. License: MIT.
- **react-bits**: `src/vendor/react-bits/` — CountUp, GradientText. License: MIT.
- Vendor dir is tracked in git (added negation to root .gitignore).

### Accessibility
- **Automated**: `npm run test:e2e` (Playwright + axe-core). Runs against live dev server at `http://localhost:5173`.
- **Lighthouse**: `npm run lighthouse` — asserts accessibility score ≥0.95.
- **Manual VoiceOver smoke (macOS Safari)** — required before production deploy:
  - [ ] Navigation landmarks announced: banner, main, navigation
  - [ ] Exchange tabs (Binance/Polymarket) announced as links in navigation
  - [ ] Inner tab list (Chart/Overview/Decisions) announced as tablist with selected state
  - [ ] Arrow-key tab cycling works with VoiceOver active
  - [ ] Skip link announced and activates focus on main region
  - [ ] MetricsBar balance announces via live region on data update
  - [ ] ConnectionBadge state changes announced via polite live region
  - [ ] Error boundary fallback announced via alert role
  - [ ] ChartSettings dialog: announced as dialog, focus trapped, ESC closes
  - [ ] Table captions read on table entry (Open Positions, Trade History, Decision Log)
  - Result: ⬜ PASS / ⬜ FAIL (fill in after manual pass)

### Reduced Motion
- JS animations: `MotionConfig reducedMotion="user"` in App.tsx handles motion/react animations.
- CSS animations: `@media (prefers-reduced-motion: reduce)` in globals.css collapses all durations.

### Spec-5 Status
- Superseded by spec-6. Archived to `tasks/specs/_archive/spec-5-dashboard-ultimate-redesign.md`.
