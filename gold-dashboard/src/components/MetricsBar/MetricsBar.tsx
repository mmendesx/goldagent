import { useCallback, useEffect, useMemo, useState } from "react";
import { useDashboardStore } from "../../store";
import { getDrawdownSeverity, dailyPnl, cumulativePnl, winRate as computeWinRate, maxDrawdown } from "../../utils";
import { restClient } from "../../api";
import { MetricCard, LiveNumber, Badge } from "../../design-system";
import { GradientText } from "../../vendor/react-bits/GradientText";
import "./MetricsBar.css";

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
  const drawdownBadge = drawdownSeverity === "high" ? (
    <Badge variant="danger" icon={<span aria-hidden="true">⚠</span>}>HIGH</Badge>
  ) : drawdownSeverity === "medium" ? (
    <Badge variant="warning" icon={<span aria-hidden="true">▲</span>}>MED</Badge>
  ) : null;

  const balanceLiveNumber = (
    <LiveNumber
      value={parseFloat(balance)}
      format={(v) => `$${v.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`}
      decimalPlaces={2}
    />
  );

  return (
    <section className="metrics-bar" aria-label="Portfolio metrics">
      {/* Hero balance — conditional on circuit breaker */}
      {isCircuitBreakerActive ? (
        <MetricCard
          label="Balance"
          severity="high"
          value={balanceLiveNumber}
          badge={
            <Badge variant="danger" icon={<span aria-hidden="true">🔒</span>}>BREAKER</Badge>
          }
        />
      ) : (
        <MetricCard
          label="Balance"
          value={
            <GradientText colors={["#f0b429", "#ffd580", "#f0b429"]} animationSpeed={8}>
              {balanceLiveNumber}
            </GradientText>
          }
        />
      )}

      <MetricCard
        label="Peak Balance"
        value={
          <LiveNumber
            value={parseFloat(peakBalance)}
            format={(v) => `$${v.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`}
            decimalPlaces={2}
          />
        }
      />

      <MetricCard
        label="Drawdown"
        severity={drawdownSeverity}
        badge={drawdownBadge}
        value={
          <LiveNumber
            value={drawdownNumeric}
            format={(v) => `${v.toFixed(2)}%`}
            decimalPlaces={2}
          />
        }
      />

      <MetricCard
        label="Win Rate"
        value={
          <LiveNumber
            value={parseFloat(winRate)}
            format={(v) => `${v.toFixed(1)}%`}
            decimalPlaces={1}
          />
        }
      />

      <MetricCard
        label="Total Trades"
        value={totalTrades.toString()}
      />

      <MetricCard
        label="Open Positions"
        value={openPositionCount.toString()}
      />

      <MetricCard
        label="Daily P&L"
        severity={analytics.daily < 0 ? "high" : undefined}
        value={
          <LiveNumber
            value={analytics.daily}
            format={(v) => {
              const prefix = v >= 0 ? "+" : "";
              return `${prefix}$${Math.abs(v).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
            }}
            decimalPlaces={2}
          />
        }
      />

      <MetricCard
        label="Cumul. P&L"
        severity={analytics.cumulative < 0 ? "medium" : undefined}
        value={
          <LiveNumber
            value={analytics.cumulative}
            format={(v) => {
              const prefix = v >= 0 ? "+" : "";
              return `${prefix}$${Math.abs(v).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
            }}
            decimalPlaces={2}
          />
        }
      />

      <MetricCard
        label="Win Rate (live)"
        value={
          analytics.winRateResult
            ? `${(analytics.winRateResult.rate * 100).toFixed(1)}% (${analytics.winRateResult.wins}W/${analytics.winRateResult.losses}L)`
            : "—"
        }
      />

      <MetricCard
        label="Max Drawdown"
        severity={analytics.drawdownResult.absolute > 0 ? "medium" : undefined}
        value={
          analytics.drawdownResult.absolute > 0
            ? analytics.drawdownResult.relative != null
              ? `$${analytics.drawdownResult.absolute.toFixed(2)} (${(analytics.drawdownResult.relative * 100).toFixed(1)}%)`
              : `$${analytics.drawdownResult.absolute.toFixed(2)}`
            : "—"
        }
      />

      <MetricCard
        label="Binance USDT"
        severity={exchangeBalances?.binance.status === "error" ? "high" : undefined}
        value={
          exchangeBalances == null
            ? "—"
            : exchangeBalances.binance.status === "ok"
              ? (
                <LiveNumber
                  value={parseFloat(exchangeBalances.binance.balance)}
                  format={(v) => `$${v.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`}
                  decimalPlaces={2}
                />
              )
              : exchangeBalances.binance.status === "not_configured"
                ? "—"
                : "Error"
        }
      />

      <MetricCard
        label="Polymarket USDC"
        severity={exchangeBalances?.polymarket.status === "error" ? "high" : undefined}
        value={
          exchangeBalances == null
            ? "—"
            : exchangeBalances.polymarket.status === "ok"
              ? (
                <LiveNumber
                  value={parseFloat(exchangeBalances.polymarket.balance)}
                  format={(v) => `$${v.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`}
                  decimalPlaces={2}
                />
              )
              : exchangeBalances.polymarket.status === "not_configured"
                ? "—"
                : "Error"
        }
      />

      {balanceError && (
        <div className="metrics-balance-error" role="alert" title={balanceError}>
          Balance error
          <button type="button" onClick={fetchBalances} style={{ marginLeft: 6, cursor: "pointer" }}>↺</button>
        </div>
      )}
    </section>
  );
}
