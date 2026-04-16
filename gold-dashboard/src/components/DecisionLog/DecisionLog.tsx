import { useEffect, useState } from "react";
import { useDashboardStore } from "../../store";
import { restClient } from "../../api";
import type { Decision, TradingSymbol } from "../../types";
import "./DecisionLog.css";

const PAGE_SIZE = 100;
const AVAILABLE_SYMBOLS: (TradingSymbol | "ALL")[] = ["ALL", "BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"];

export function DecisionLog() {
  const decisions = useDashboardStore((state) => state.decisions);
  const setDecisions = useDashboardStore((state) => state.setDecisions);
  const [isLoading, setIsLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [symbolFilter, setSymbolFilter] = useState<TradingSymbol | "ALL">("ALL");

  // Initial load from REST
  useEffect(() => {
    let isCancelled = false;
    setIsLoading(true);
    setErrorMessage(null);

    const symbolParam = symbolFilter === "ALL" ? undefined : symbolFilter;

    restClient
      .fetchDecisions(symbolParam, PAGE_SIZE, 0)
      .then((response) => {
        if (!isCancelled) setDecisions(response.items);
      })
      .catch((error: unknown) => {
        if (isCancelled) return;
        const message = error instanceof Error ? error.message : "Failed to load decisions";
        setErrorMessage(message);
      })
      .finally(() => {
        if (!isCancelled) setIsLoading(false);
      });

    return () => {
      isCancelled = true;
    };
  }, [symbolFilter, setDecisions]);

  // Filter live-updated decisions client-side by symbol
  const filteredDecisions = symbolFilter === "ALL"
    ? decisions
    : decisions.filter((decision) => decision.symbol === symbolFilter);

  return (
    <div className="decision-log">
      <div className="decision-log-controls">
        <label className="decision-log-filter">
          <span className="decision-log-filter-label">Symbol</span>
          <select
            value={symbolFilter}
            onChange={(event) => setSymbolFilter(event.target.value as TradingSymbol | "ALL")}
            className="decision-log-select"
          >
            {AVAILABLE_SYMBOLS.map((symbol) => (
              <option key={symbol} value={symbol}>{symbol}</option>
            ))}
          </select>
        </label>
        <div className="decision-log-meta">
          {filteredDecisions.length} decision{filteredDecisions.length === 1 ? "" : "s"}
        </div>
      </div>

      {errorMessage && (
        <div className="decision-log-error" role="alert">{errorMessage}</div>
      )}

      {isLoading && filteredDecisions.length === 0 ? (
        <div className="decision-log-empty">Loading…</div>
      ) : filteredDecisions.length === 0 ? (
        <div className="decision-log-empty">
          No decisions yet. The engine logs every evaluation, including HOLDs.
        </div>
      ) : (
        <div className="decision-log-table-container">
          <table className="decision-log-table">
            <thead>
              <tr>
                <th>Time</th>
                <th>Symbol</th>
                <th>Action</th>
                <th>Confidence</th>
                <th>Score</th>
                <th>Status</th>
                <th>Reason</th>
              </tr>
            </thead>
            <tbody>
              {filteredDecisions.map((decision) => (
                <DecisionRow key={decision.id} decision={decision} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function DecisionRow({ decision }: { decision: Decision }) {
  const actionClassName = getActionClassName(decision.action);
  const statusClassName = getStatusClassName(decision.executionStatus);
  const compositeScoreNumeric = parseFloat(decision.compositeScore);
  const scoreClassName = compositeScoreNumeric > 0 ? "score-positive" : compositeScoreNumeric < 0 ? "score-negative" : "score-neutral";

  return (
    <tr>
      <td>{formatDateTime(decision.createdAt)}</td>
      <td className="symbol-cell">{decision.symbol}</td>
      <td>
        <span className={`action-badge ${actionClassName}`}>{decision.action}</span>
        {decision.isDryRun && <span className="dry-run-badge" title="Dry run — no real order placed">DRY</span>}
      </td>
      <td className="numeric-cell">
        <ConfidenceBar confidence={decision.confidence} />
      </td>
      <td className={`numeric-cell ${scoreClassName}`}>
        {compositeScoreNumeric >= 0 ? "+" : ""}{compositeScoreNumeric.toFixed(1)}
      </td>
      <td>
        <span className={`status-badge ${statusClassName}`}>{formatStatus(decision.executionStatus)}</span>
      </td>
      <td className="reason-cell">{decision.rejectionReason ?? "—"}</td>
    </tr>
  );
}

function ConfidenceBar({ confidence }: { confidence: number }) {
  const severity = confidence >= 70 ? "high" : confidence >= 40 ? "medium" : "low";
  return (
    <div className="confidence-bar">
      <div className="confidence-bar-track">
        <div
          className={`confidence-bar-fill confidence-bar-fill--${severity}`}
          style={{ width: `${Math.min(100, Math.max(0, confidence))}%` }}
        />
      </div>
      <span className="confidence-bar-value">{confidence}</span>
    </div>
  );
}

function getActionClassName(action: Decision["action"]): string {
  if (action === "BUY") return "action-buy";
  if (action === "SELL") return "action-sell";
  return "action-hold";
}

function getStatusClassName(status: string): string {
  if (status === "executed") return "status-executed";
  if (status === "dry_run") return "status-dry-run";
  return "status-rejected";
}

function formatStatus(status: string): string {
  return status.replace(/_/g, " ");
}

function formatDateTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleTimeString("en-US", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}
