import { NavLink, Routes, Route, Navigate } from "react-router-dom";
import { PriceChart } from "../../components/PriceChart/PriceChart";
import { OpenPositions } from "../../components/OpenPositions/OpenPositions";
import { TradeHistory } from "../../components/TradeHistory";
import { DecisionLog } from "../../components/DecisionLog/DecisionLog";
import "./Dashboard.css";

export function BinanceView() {
  return (
    <>
      <nav className="view-tabs" aria-label="Binance sections">
        <NavLink
          to="/binance/chart"
          className={({ isActive }) => (isActive ? "tab-link active" : "tab-link")}
        >
          Chart
        </NavLink>
        <NavLink
          to="/binance/positions"
          className={({ isActive }) => (isActive ? "tab-link active" : "tab-link")}
        >
          Open Positions
        </NavLink>
        <NavLink
          to="/binance/history"
          className={({ isActive }) => (isActive ? "tab-link active" : "tab-link")}
        >
          Trade History
        </NavLink>
        <NavLink
          to="/binance/decisions"
          className={({ isActive }) => (isActive ? "tab-link active" : "tab-link")}
        >
          Decision Log
        </NavLink>
      </nav>

      <main className="dashboard-content">
        <Routes>
          <Route path="chart" element={<PriceChart />} />
          <Route path="positions" element={<OpenPositions />} />
          <Route path="history" element={<TradeHistory />} />
          <Route path="decisions" element={<DecisionLog />} />
          <Route path="*" element={<Navigate to="/binance/chart" replace />} />
        </Routes>
      </main>
    </>
  );
}
