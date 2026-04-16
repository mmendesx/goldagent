# Integration Checklist — Gold Dashboard

Manual verification guide for validating the dashboard end-to-end against the live backend.

---

## Prerequisites

All commands are run from the **monorepo root** (`/workspace/goldagent`) unless otherwise noted.

---

## 1. Start Infrastructure

```bash
docker compose up -d postgres redis
```

DevTools check: nothing yet — just confirm the command exits cleanly.

---

## 2. Run Backend Migrations

Using [golang-migrate](https://github.com/golang-migrate/migrate) CLI:

```bash
migrate -path gold-backend/migrations \
        -database "postgres://postgres:postgres@localhost:5432/goldagent?sslmode=disable" \
        up
```

Adjust the DSN to match your local `docker-compose.yml` credentials. On first run you should see one line per migration file with no errors.

---

## 3. Start Backend (Dry-Run Mode)

```bash
cd gold-backend
GOLD_DRY_RUN=true go run ./cmd/gold
```

No Binance API keys are required in dry-run mode. The backend exposes:
- REST at `http://localhost:8080/api/v1/*`
- WebSocket at `ws://localhost:8080/ws/v1/stream`

DevTools check: the terminal should emit structured JSON log lines, e.g. `{"level":"info","msg":"server started","addr":":8080"}`.

---

## 4. Start Frontend

```bash
cd gold-dashboard
npm run dev
```

The dev server starts at `http://localhost:3000`.

---

## 5. Open the Dashboard

Navigate to `http://localhost:3000` in a browser.

**DevTools — Network tab**: you should see three initial REST calls fire immediately on page load:
- `GET /api/v1/metrics` → 200
- `GET /api/v1/positions` → 200
- `GET /api/v1/positions/history?limit=100&offset=0` → 200

**DevTools — WS tab**: a WebSocket connection to `ws://localhost:8080/ws/v1/stream` should appear with status `101 Switching Protocols`.

**DevTools — Console**: no errors or uncaught exceptions at startup.

---

## 6. Visual Checks

- [ ] Dashboard loads with a **dark theme** (near-black background `#12121a`)
- [ ] **Connection badge** in the header shows a green dot labeled **"Live"**
- [ ] **Metrics bar** is visible at the top (values will be zeros on a fresh database)

---

## 7. Tab Navigation

- [ ] Click **Chart** tab → URL changes to `/chart`, PriceChart renders — no full page reload
- [ ] Click **Open Positions** → URL changes to `/positions`
- [ ] Click **Trade History** → URL changes to `/history`
- [ ] Click **Decision Log** → URL changes to `/decisions`
- [ ] Clicking back and forward in the browser navigates between tabs correctly

DevTools check: no additional full-page navigation events — only `fetch`/`xhr` requests for data.

---

## 8. Chart Tab

- [ ] Chart container renders (dark candlestick chart area, even if empty)
- [ ] **Symbol selector** dropdown shows all 4 options: BTCUSDT, ETHUSDT, SOLUSDT, BNBUSDT
- [ ] **Interval buttons** are visible: 1m, 5m, 15m, 1h, 4h, 1D — the active one is highlighted
- [ ] Switching symbol or interval triggers a new `GET /api/v1/candles` request (visible in Network tab)
- [ ] No JavaScript errors appear in Console after switching symbol/interval

DevTools — Network: `GET /api/v1/candles?symbol=BTCUSDT&interval=5m&limit=500` should return a `200` with a `{ items: [], ... }` body on a fresh database.

---

## 9. Open Positions Tab

- [ ] Page renders without errors
- [ ] **Empty state message** is displayed (no positions on first run)

DevTools — Network: `GET /api/v1/positions` returns `[]`.

---

## 10. Trade History Tab

- [ ] Page renders without errors
- [ ] **Empty state message** is displayed
- [ ] Pagination controls (Previous / Next) are visible and Previous is disabled on page 1

DevTools — Network: `GET /api/v1/positions/history?limit=50&offset=0` returns `{ items: [], hasMore: false, ... }`.

---

## 11. Decision Log Tab

- [ ] Page renders without errors
- [ ] **Empty state message** is displayed, or a table of decisions appears if the backend has been running long enough in dry-run mode
- [ ] Symbol filter dropdown shows ALL, BTCUSDT, ETHUSDT, SOLUSDT, BNBUSDT

DevTools — Network: `GET /api/v1/decisions?limit=100&offset=0` returns `200`.

---

## 12. Backend Log Verification

In the terminal running the backend, confirm:
- Logs are structured JSON (one JSON object per line)
- Each incoming request logs method, path, and status code
- No `ERROR`-level entries during normal idle operation

---

## 13. Connection Badge — Offline State

- [ ] Stop the backend (`Ctrl+C` in its terminal)
- [ ] Within ~5 seconds the connection badge changes to show **"Offline"** (red dot) or **"Connecting…"** (amber) while attempting to reconnect

DevTools — WS tab: the WebSocket connection should show as closed/failed.

---

## 14. Connection Badge — Reconnect

- [ ] Restart the backend: `GOLD_DRY_RUN=true go run ./cmd/gold`
- [ ] Within ~30 seconds (exponential backoff) the badge returns to **"Live"** (green dot) without a page refresh

---

## Notes

- All REST and WebSocket URLs are controlled by environment variables: `VITE_API_BASE_URL` and `VITE_WS_URL`. In development these default to `http://localhost:8080` and `ws://localhost:8080/ws/v1/stream` respectively.
- If the backend is on a different port, create `gold-dashboard/.env.local` with the correct values before running `npm run dev`.
- The frontend gracefully handles all backend errors — failed fetches log a `console.warn` and display an error message in the relevant component; they do not crash the page.
