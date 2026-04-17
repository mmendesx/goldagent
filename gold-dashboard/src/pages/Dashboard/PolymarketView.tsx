import { NavLink, Routes, Route, Navigate } from "react-router-dom";
import { PriceChart } from "../../components/PriceChart/PriceChart";
import { PolymarketOverview } from "../../components/PolymarketOverview";
import { DecisionLog } from "../../components/DecisionLog/DecisionLog";
import { ErrorBoundary } from "../../components/ErrorBoundary";

export function PolymarketView() {
  return (
    <>
      <nav className="view-tabs" aria-label="Polymarket sections">
        <NavLink to="/polymarket/chart" className={({ isActive }) => isActive ? "tab-link active" : "tab-link"}>
          Chart
        </NavLink>
        <NavLink to="/polymarket/overview" className={({ isActive }) => isActive ? "tab-link active" : "tab-link"}>
          Overview
        </NavLink>
        <NavLink to="/polymarket/decisions" className={({ isActive }) => isActive ? "tab-link active" : "tab-link"}>
          Decision Log
        </NavLink>
      </nav>

      <main className="dashboard-content">
        <Routes>
          <Route path="chart" element={<ErrorBoundary><PriceChart exchange="polymarket" /></ErrorBoundary>} />
          <Route path="overview" element={<ErrorBoundary><PolymarketOverview /></ErrorBoundary>} />
          <Route path="decisions" element={<ErrorBoundary><DecisionLog /></ErrorBoundary>} />
          <Route path="*" element={<Navigate to="/polymarket/chart" replace />} />
        </Routes>
      </main>
    </>
  );
}
