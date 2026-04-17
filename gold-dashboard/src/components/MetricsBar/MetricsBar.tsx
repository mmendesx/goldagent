import { useCallback, useEffect, useMemo, useState } from "react";
import { useDashboardStore } from "../../store";
import { formatCurrency, formatPercent, getDrawdownSeverity, dailyPnl, cumulativePnl, winRate as computeWinRate, maxDrawdown } from "../../utils";
import { restClient } from "../../api";
import { MetricItem } from "./MetricItem";
import "./MetricsBar.css";

function formatSignedPnl(value: number): string {
  const prefix = value >= 0 ? '+' : '';
  return `${prefix}$${Math.abs(value).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

const EXCHANGE_BALANCE_POLL_MS = 30_000;

export function MetricsBar() {
  const metrics = useDashboardStore((state) => state.metrics);
  const openPositionCount = useDashboardStore((state) => state.openPositions.length);
  const exchangeBalances = useDashboardStore((state) => state.exchangeBalances);
  const setExchangeBalances = useDashboardStore((state) => state.setExchangeBalances);
  const closedPositions = useDashboardStore((state) => state.closedPositions);
  const [balanceError, setBalanceError] = useState<string | null>(null);

  const analytics = useMemo(() => ({
    daily: dailyPnl(closedPositions),
    cumulative: cumulativePnl(closedPositions),
    winRateResult: computeWinRate(closedPositions),
    drawdownResult: maxDrawdown(closedPositions),
  }), [closedPositions]);

  const fetchBalances = useCallback(() => {
    restClient.fetchExchangeBalances()
      .then((b) => { setExchangeBalances(b); setBalanceError(null); })
      .catch((err: unknown) => {
        setBalanceError(err instanceof Error ? err.message : "Failed to load balances");
      });
  }, [setExchangeBalances]);

  useEffect(() => {
    fetchBalances();
    const intervalId = setInterval(fetchBalances, EXCHANGE_BALANCE_POLL_MS);
    return () => clearInterval(intervalId);
  }, [fetchBalances]);

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
        label="Daily P&L"
        value={formatSignedPnl(analytics.daily)}
        severity={analytics.daily < 0 ? 'high' : undefined}
      />
      <MetricItem
        label="Cumul. P&L"
        value={formatSignedPnl(analytics.cumulative)}
        severity={analytics.cumulative < 0 ? 'medium' : undefined}
      />
      <MetricItem
        label="Win Rate (live)"
        value={
          analytics.winRateResult
            ? `${(analytics.winRateResult.rate * 100).toFixed(1)}% (${analytics.winRateResult.wins}W/${analytics.winRateResult.losses}L)`
            : '—'
        }
      />
      <MetricItem
        label="Max Drawdown"
        value={
          analytics.drawdownResult.absolute > 0
            ? analytics.drawdownResult.relative != null
              ? `$${analytics.drawdownResult.absolute.toFixed(2)} (${(analytics.drawdownResult.relative * 100).toFixed(1)}%)`
              : `$${analytics.drawdownResult.absolute.toFixed(2)}`
            : '—'
        }
        severity={analytics.drawdownResult.absolute > 0 ? 'medium' : undefined}
      />
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
      {balanceError && (
        <div className="metrics-balance-error" role="alert" title={balanceError}>
          Balance error
          <button type="button" onClick={fetchBalances} style={{ marginLeft: 6, cursor: "pointer" }}>↺</button>
        </div>
      )}
    </section>
  );
}
