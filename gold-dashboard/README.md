# Gold Dashboard

React + TypeScript + Vite frontend for the Gold Agent trading system.

## Development

```bash
npm install
npm run dev
```

Frontend available at `http://localhost:5173`. Requires the backend running on `http://localhost:8080` — see `INTEGRATION_CHECKLIST.md` for full setup.

## Available commands

| Command | Description |
|---|---|
| `npm run dev` | Start development server with HMR |
| `npm run build` | Type-check and build for production |
| `npm run preview` | Preview the production build locally |
| `npm run lint` | Run ESLint |
| `npm run test` | Run unit tests (Vitest) |
| `npm run test:e2e` | Run end-to-end + accessibility tests (Playwright + axe-core). Requires a live dev server at `http://localhost:5173`. |
| `npm run lighthouse` | Run Lighthouse CI — asserts accessibility score ≥ 0.95 |

## Design system

New components live in `src/design-system/`. The entry point exports all primitives and composition components:

- **Primitives**: Button, Card, Badge, Skeleton, SkeletonContainer, VisuallyHidden, SkipLink, ThemeToggle
- **Composition**: PageShell, AnimatedTabs, MetricCard, LiveNumber

Design tokens (dark/light themes) are in `src/styles/tokens.css`. All components use `var(--token)` references.

Vendored animation libraries are in `src/vendor/` (animate-ui, react-bits) — tracked in git.

## Environment variables

Create `gold-dashboard/.env.local` to override defaults:

```
VITE_API_BASE_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080/ws/v1/stream
```

## Integration

See `INTEGRATION_CHECKLIST.md` for full manual verification steps, API contracts, WebSocket event shapes, and troubleshooting.
