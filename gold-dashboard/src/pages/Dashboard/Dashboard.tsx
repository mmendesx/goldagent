import { NavLink, Routes, Route, Navigate } from "react-router-dom";
import { useWebSocketLifecycle } from "../../hooks";
import { useDashboardStore } from "../../store";
import type { ConnectionState } from "../../api";
import "./Dashboard.css";

export function Dashboard() {
  useWebSocketLifecycle();

  const connectionState = useDashboardStore((s) => s.connectionState);

  return (
    <div className="dashboard">
      <header className="dashboard-metrics">
        <div className="metrics-bar-placeholder">
          <MetricItem label="Balance" value="$0.00" />
          <MetricItem label="Peak Balance" value="$0.00" />
          <MetricItem label="Drawdown" value="0.00%" severity="low" />
          <MetricItem label="Win Rate" value="0.0%" />
          <MetricItem label="Total Trades" value="0" />
          <MetricItem label="Open Positions" value="0" />
        </div>
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
          <Route path="/chart" element={<div className="placeholder-panel">Chart — coming soon</div>} />
          <Route path="/positions" element={<div className="placeholder-panel">Open Positions — coming soon</div>} />
          <Route path="/history" element={<div className="placeholder-panel">Trade History — coming soon</div>} />
          <Route path="/decisions" element={<div className="placeholder-panel">Decision Log — coming soon</div>} />
          <Route path="/" element={<Navigate to="/chart" replace />} />
        </Routes>
      </main>
    </div>
  );
}

interface MetricItemProps {
  label: string;
  value: string;
  severity?: "low" | "medium" | "high";
}

function MetricItem({ label, value, severity }: MetricItemProps) {
  const severityColor =
    severity === "high"
      ? "var(--color-negative)"
      : severity === "medium"
      ? "var(--color-warning)"
      : "var(--color-positive)";

  return (
    <div className="metric-item">
      <span className="metric-label">{label}</span>
      <span
        className="metric-value"
        style={severity ? { color: severityColor } : undefined}
      >
        {value}
      </span>
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
