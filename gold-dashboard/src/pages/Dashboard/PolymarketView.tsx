import { NavLink, Routes, Route, Navigate } from "react-router-dom";
import { PriceChart } from "../../components/PriceChart/PriceChart";
import { PolymarketOverview } from "../../components/PolymarketOverview";
import { DecisionLog } from "../../components/DecisionLog/DecisionLog";

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
          <Route path="chart" element={<PriceChart />} />
          <Route path="overview" element={<PolymarketOverview />} />
          <Route path="decisions" element={<DecisionLog />} />
          <Route path="*" element={<Navigate to="/polymarket/chart" replace />} />
        </Routes>
      </main>
    </>
  );
}
