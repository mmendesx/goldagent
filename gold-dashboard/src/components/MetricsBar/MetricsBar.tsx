import { useEffect } from "react";
import { useDashboardStore } from "../../store";
import { formatCurrency, formatPercent, getDrawdownSeverity } from "../../utils";
import { restClient } from "../../api";
import { MetricItem } from "./MetricItem";
import "./MetricsBar.css";

const EXCHANGE_BALANCE_POLL_MS = 30_000;

export function MetricsBar() {
  const metrics = useDashboardStore((state) => state.metrics);
  const openPositionCount = useDashboardStore((state) => state.openPositions.length);
  const exchangeBalances = useDashboardStore((state) => state.exchangeBalances);
  const setExchangeBalances = useDashboardStore((state) => state.setExchangeBalances);

  useEffect(() => {
    const fetchBalances = () => {
      restClient.fetchExchangeBalances().then(setExchangeBalances).catch(() => {});
    };
    fetchBalances();
    const intervalId = setInterval(fetchBalances, EXCHANGE_BALANCE_POLL_MS);
    return () => clearInterval(intervalId);
  }, [setExchangeBalances]);

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
      <MetricItem
        label="Binance USDT"
        value={
          exchangeBalances == null
            ? "—"
            : exchangeBalances.binance.status === "ok"
              ? "$" +
                parseFloat(exchangeBalances.binance.balance).toLocaleString("en-US", {
                  minimumFractionDigits: 2,
                  maximumFractionDigits: 2,
                })
              : exchangeBalances.binance.status === "not_configured"
                ? "—"
                : "Error"
        }
        severity={exchangeBalances?.binance.status === "error" ? "high" : undefined}
      />
      <MetricItem
        label="Polymarket USDC"
        value={
          exchangeBalances == null
            ? "—"
            : exchangeBalances.polymarket.status === "ok"
              ? "$" +
                parseFloat(exchangeBalances.polymarket.balance).toLocaleString("en-US", {
                  minimumFractionDigits: 2,
                  maximumFractionDigits: 2,
                })
              : exchangeBalances.polymarket.status === "not_configured"
                ? "—"
                : "Error"
        }
        severity={exchangeBalances?.polymarket.status === "error" ? "high" : undefined}
      />
      {isCircuitBreakerActive && (
        <div className="circuit-breaker-alert" role="alert">
          ⚠ Circuit breaker active
        </div>
      )}
    </section>
  );
}
