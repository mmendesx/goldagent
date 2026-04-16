import { useDashboardStore } from "../../store";
import { formatCurrency, formatPercent, getDrawdownSeverity } from "../../utils";
import { MetricItem } from "./MetricItem";
import "./MetricsBar.css";

export function MetricsBar() {
  const metrics = useDashboardStore((state) => state.metrics);
  const openPositionCount = useDashboardStore((state) => state.openPositions.length);

  const balance = metrics?.balance ?? "0";
  const peakBalance = metrics?.peakBalance ?? "0";
  const drawdownPercent = metrics?.drawdownPercent ?? "0";
  const winRate = metrics?.winRate ?? "0";
  const totalTrades = metrics?.totalTrades ?? 0;
  const isCircuitBreakerActive = metrics?.isCircuitBreakerActive ?? false;

  const drawdownNumeric = parseFloat(drawdownPercent);
  const drawdownSeverity = getDrawdownSeverity(drawdownNumeric);

  return (
    <section className="metrics-bar" aria-label="Portfolio metrics">
      <MetricItem label="Balance" value={formatCurrency(balance)} />
      <MetricItem label="Peak Balance" value={formatCurrency(peakBalance)} />
      <MetricItem
        label="Drawdown"
        value={formatPercent(drawdownPercent)}
        severity={drawdownSeverity}
      />
      <MetricItem label="Win Rate" value={formatPercent(winRate, 1)} />
      <MetricItem label="Total Trades" value={totalTrades.toString()} />
      <MetricItem label="Open Positions" value={openPositionCount.toString()} />
      {isCircuitBreakerActive && (
        <div className="circuit-breaker-alert" role="alert">
          ⚠ Circuit breaker active
        </div>
      )}
    </section>
  );
}
