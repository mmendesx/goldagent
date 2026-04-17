import { useEffect } from "react";
import { NavLink, Routes, Route, Navigate } from "react-router-dom";
import { useWebSocketLifecycle } from "../../hooks";
import { useDashboardStore } from "../../store";
import { restClient } from "../../api";
import { MetricsBar } from "../../components/MetricsBar";
import { ConnectionBadge } from "../../components/ConnectionBadge/ConnectionBadge";
import { ErrorBoundary } from "../../components/ErrorBoundary";
import { PageShell, ThemeToggle } from "../../design-system";
import { BinanceView } from "./BinanceView";
import { PolymarketView } from "./PolymarketView";
import "./Dashboard.css";

export function Dashboard() {
  useWebSocketLifecycle();

  const setMetrics = useDashboardStore((state) => state.setMetrics);
  const setOpenPositions = useDashboardStore((state) => state.setOpenPositions);
  const setClosedPositions = useDashboardStore((state) => state.setClosedPositions);
  const reconnectAttempts = useDashboardStore((state) => state.reconnectAttempts);

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
    <PageShell
      header={
        <>
          <ErrorBoundary resetKeys={[reconnectAttempts]}>
            <MetricsBar />
          </ErrorBoundary>
          <div className="dashboard-header-right">
            <ThemeToggle />
            <ConnectionBadge />
          </div>
        </>
      }
    >
      <nav className="exchange-tabs" role="navigation" aria-label="Exchange">
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
    </PageShell>
  );
}
