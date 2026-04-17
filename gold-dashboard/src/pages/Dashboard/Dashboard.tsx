import { useEffect } from "react";
import { NavLink, Routes, Route, Navigate } from "react-router-dom";
import { useWebSocketLifecycle } from "../../hooks";
import { useDashboardStore } from "../../store";
import { restClient } from "../../api";
import type { ConnectionState } from "../../api";
import { MetricsBar } from "../../components/MetricsBar";
import { ErrorBoundary } from "../../components/ErrorBoundary";
import { BinanceView } from "./BinanceView";
import { PolymarketView } from "./PolymarketView";
import "./Dashboard.css";

export function Dashboard() {
  useWebSocketLifecycle();

  const connectionState = useDashboardStore((s) => s.connectionState);
  const setMetrics = useDashboardStore((state) => state.setMetrics);
  const setOpenPositions = useDashboardStore((state) => state.setOpenPositions);
  const setClosedPositions = useDashboardStore((state) => state.setClosedPositions);

  useEffect(() => {
    restClient
      .fetchMetrics()
      .then(setMetrics)
      .catch((error) => console.warn("Failed to bootstrap metrics", error));
  }, [setMetrics]);

  useEffect(() => {
    restClient
      .fetchOpenPositions()
      .then((positions) => setOpenPositions(positions ?? []))
      .catch((error) => console.warn("Failed to fetch open positions", error));
  }, [setOpenPositions]);

  useEffect(() => {
    restClient
      .fetchClosedPositions()
      .then((res) => setClosedPositions(res.items ?? []))
      .catch((error) => console.warn("Failed to fetch closed positions", error));
  }, [setClosedPositions]);

  return (
    <div className="dashboard">
      <header className="dashboard-metrics">
        <ErrorBoundary>
          <MetricsBar />
        </ErrorBoundary>
        <ConnectionBadge state={connectionState} />
      </header>

      <nav className="exchange-tabs" aria-label="Exchange">
        <NavLink to="/binance" className={({ isActive }) => (isActive ? "exchange-tab active" : "exchange-tab")}>
          Binance
        </NavLink>
        <NavLink to="/polymarket" className={({ isActive }) => (isActive ? "exchange-tab active" : "exchange-tab")}>
          Polymarket
        </NavLink>
      </nav>

      <Routes>
        <Route path="/binance/*" element={<BinanceView />} />
        <Route path="/polymarket/*" element={<PolymarketView />} />
        <Route path="/" element={<Navigate to="/binance/chart" replace />} />
      </Routes>
    </div>
  );
}

interface ConnectionBadgeProps {
  state: ConnectionState;
}

function ConnectionBadge({ state }: ConnectionBadgeProps) {
  const label =
    state === "open"
      ? "Live"
      : state === "connecting" || state === "reconnecting"
      ? "Connecting\u2026"
      : "Offline";

  return (
    <div className={`connection-badge connection-badge--${state}`} aria-live="polite" aria-label={`Connection status: ${label}`}>
      <span className="connection-badge__dot" aria-hidden="true" />
      <span className="connection-badge__label">{label}</span>
    </div>
  );
}
