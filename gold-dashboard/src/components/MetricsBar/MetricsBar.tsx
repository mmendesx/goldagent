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

  const balance = parseFloat(metrics?.balance ?? "0");
  const peakBalance = parseFloat(metrics?.peakBalance ?? "0");
  const drawdownPercent = parseFloat(metrics?.drawdownPercent ?? "0");
  const winRate = parseFloat(metrics?.winRate ?? "0");
  const totalTrades = metrics?.totalTrades ?? 0;
  const isCircuitBreakerActive = metrics?.isCircuitBreakerActive ?? false;

  const drawdownSeverity = getDrawdownSeverity(drawdownPercent);
  const showCircuitBreaker = isCircuitBreakerActive || drawdownPercent >= 15;

  const circuitBreakerBadge = showCircuitBreaker ? (
    <span className="circuit-breaker-badge" role="status" aria-label="Circuit breaker active">
      CIRCUIT BREAKER
    </span>
  ) : undefined;

  const binanceValue =
    exchangeBalances == null
      ? "—"
      : exchangeBalances.binance.status === "ok"
        ? parseFloat(exchangeBalances.binance.balance)
        : null;

  const polymarketValue =
    exchangeBalances == null
      ? "—"
      : exchangeBalances.polymarket.status === "ok"
        ? parseFloat(exchangeBalances.polymarket.balance)
        : null;

  return (
    <section className="metrics-bar" aria-label="Portfolio metrics">
      <MetricItem
        label="Balance"
        numericValue={balance}
        format={(n) => formatCurrency(n)}
      />
      <MetricItem
        label="Peak Balance"
        numericValue={peakBalance}
        format={(n) => formatCurrency(n)}
      />
      <MetricItem
        label="Drawdown"
        numericValue={drawdownPercent}
        format={(n) => formatPercent(n)}
        severity={drawdownSeverity}
        badge={circuitBreakerBadge}
      />
      <MetricItem
        label="Win Rate"
        numericValue={winRate}
        format={(n) => formatPercent(n, 1)}
      />
      <MetricItem label="Total Trades" value={totalTrades.toString()} />
      <MetricItem label="Open Positions" value={openPositionCount.toString()} />
      <MetricItem
        label="Binance USDT"
        {...(typeof binanceValue === "number"
          ? {
              numericValue: binanceValue,
              format: (n: number) =>
                "$" +
                n.toLocaleString("en-US", {
                  minimumFractionDigits: 2,
                  maximumFractionDigits: 2,
                }),
            }
          : {
              value:
                exchangeBalances?.binance.status === "not_configured" || exchangeBalances == null
                  ? "—"
                  : "Error",
            })}
        severity={exchangeBalances?.binance.status === "error" ? "high" : undefined}
      />
      <MetricItem
        label="Polymarket USDC"
        {...(typeof polymarketValue === "number"
          ? {
              numericValue: polymarketValue,
              format: (n: number) =>
                "$" +
                n.toLocaleString("en-US", {
                  minimumFractionDigits: 2,
                  maximumFractionDigits: 2,
                }),
            }
          : {
              value:
                exchangeBalances?.polymarket.status === "not_configured" || exchangeBalances == null
                  ? "—"
                  : "Error",
            })}
        severity={exchangeBalances?.polymarket.status === "error" ? "high" : undefined}
      />
    </section>
  );
}
