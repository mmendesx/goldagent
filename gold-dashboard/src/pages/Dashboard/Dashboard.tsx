import { useEffect } from "react";
import { NavLink, Routes, Route, Navigate } from "react-router-dom";
import { useWebSocketLifecycle } from "../../hooks";
import { useDashboardStore } from "../../store";
import { restClient } from "../../api";
import type { ConnectionState } from "../../api";
import { MetricsBar } from "../../components/MetricsBar";
import { OpenPositions } from "../../components/OpenPositions/OpenPositions";
import { PriceChart } from "../../components/PriceChart/PriceChart";
import { DecisionLog } from "../../components/DecisionLog/DecisionLog";
import { TradeHistory } from "../../components/TradeHistory";
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
      .then(setOpenPositions)
      .catch((error) => console.warn("Failed to fetch open positions", error));
  }, [setOpenPositions]);

  useEffect(() => {
    restClient
      .fetchClosedPositions()
      .then((res) => setClosedPositions(res.items))
      .catch((error) => console.warn("Failed to fetch closed positions", error));
  }, [setClosedPositions]);

  return (
    <div className="dashboard">
      <header className="dashboard-metrics">
        <MetricsBar />
        <ConnectionBadge state={connectionState} />
      </header>

      <nav className="dashboard-tabs" aria-label="Dashboard sections">
        <NavLink
          to="/chart"
          className={({ isActive }) => (isActive ? "tab-link active" : "tab-link")}
        >
          Chart
        </NavLink>
        <NavLink
          to="/positions"
          className={({ isActive }) => (isActive ? "tab-link active" : "tab-link")}
        >
          Open Positions
        </NavLink>
        <NavLink
          to="/history"
          className={({ isActive }) => (isActive ? "tab-link active" : "tab-link")}
        >
          Trade History
        </NavLink>
        <NavLink
          to="/decisions"
          className={({ isActive }) => (isActive ? "tab-link active" : "tab-link")}
        >
          Decision Log
        </NavLink>
      </nav>

      <main className="dashboard-content">
        <Routes>
          <Route path="/chart" element={<PriceChart />} />
          <Route path="/positions" element={<OpenPositions />} />
          <Route path="/history" element={<TradeHistory />} />
          <Route path="/decisions" element={<DecisionLog />} />
          <Route path="/" element={<Navigate to="/chart" replace />} />
        </Routes>
      </main>
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
