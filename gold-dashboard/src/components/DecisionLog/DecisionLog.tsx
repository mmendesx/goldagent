import { useEffect, useReducer, useState } from "react";
import { restClient } from "../../api";
import { AsyncBoundary } from "../AsyncBoundary";
import type { AsyncState } from "../../hooks";
import type { Decision, TradingSymbol } from "../../types";
import "./DecisionLog.css";

const PAGE_LIMIT = 50;
const TRUNCATE_LENGTH = 240;
const AVAILABLE_SYMBOLS: (TradingSymbol | "ALL")[] = ["ALL", "BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"];

type FetchState = {
  asyncState: AsyncState;
  errorMessage: string | null;
};

type FetchAction =
  | { type: "start" }
  | { type: "success"; hasItems: boolean }
  | { type: "error"; message: string };

function fetchReducer(_state: FetchState, action: FetchAction): FetchState {
  switch (action.type) {
    case "start":
      return { asyncState: "loading", errorMessage: null };
    case "success":
      return { asyncState: action.hasItems ? "ready" : "empty", errorMessage: null };
    case "error":
      return { asyncState: "error", errorMessage: action.message };
  }
}

export function DecisionLog() {
  const [decisions, setDecisions] = useState<Decision[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [offset, setOffset] = useState(0);
  const [symbolFilter, setSymbolFilter] = useState<TradingSymbol | "ALL">("ALL");
  const [{ asyncState, errorMessage }, dispatch] = useReducer(fetchReducer, {
    asyncState: "loading",
    errorMessage: null,
  });

  useEffect(() => {
    let cancelled = false;
    dispatch({ type: "start" });

    const symbol = symbolFilter === "ALL" ? undefined : symbolFilter;

    restClient
      .fetchDecisions(symbol, PAGE_LIMIT, offset)
      .then((response) => {
        if (cancelled) return;
        const items = response.items ?? [];
        setDecisions(items);
        setHasMore(response.hasMore);
        dispatch({ type: "success", hasItems: items.length > 0 });
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        const message = error instanceof Error ? error.message : "Failed to load decisions";
        dispatch({ type: "error", message });
      });

    return () => {
      cancelled = true;
    };
  }, [offset, symbolFilter]);

  function handleSymbolChange(value: TradingSymbol | "ALL") {
    setSymbolFilter(value);
    setOffset(0);
  }

  const isLoading = asyncState === "loading";
  const currentPage = Math.floor(offset / PAGE_LIMIT) + 1;

  return (
    <div className="decision-log">
      <div className="decision-log-controls">
        <label className="decision-log-filter">
          <span className="decision-log-filter-label">Symbol</span>
          <select
            value={symbolFilter}
            onChange={(event) => handleSymbolChange(event.target.value as TradingSymbol | "ALL")}
            className="decision-log-select"
            disabled={isLoading}
          >
            {AVAILABLE_SYMBOLS.map((symbol) => (
              <option key={symbol} value={symbol}>{symbol}</option>
            ))}
          </select>
        </label>

        <div className="decision-log-pagination">
          <button
            type="button"
            className="decision-log-page-button"
            onClick={() => setOffset((prev) => Math.max(0, prev - PAGE_LIMIT))}
            disabled={offset === 0 || isLoading}
          >
            ← Previous
          </button>
          <span className="decision-log-page-info">Page {currentPage}</span>
          <button
            type="button"
            className="decision-log-page-button"
            onClick={() => setOffset((prev) => prev + PAGE_LIMIT)}
            disabled={!hasMore || isLoading}
          >
            Next →
          </button>
        </div>
      </div>

      <AsyncBoundary
        state={asyncState}
        onRetry={() => {
          setOffset(0);
          setSymbolFilter("ALL");
        }}
        emptyCopy="No decisions found. The engine logs every evaluation, including HOLDs."
        errorMessage={errorMessage ?? "Failed to load decisions"}
      >
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
                <th>Reasoning</th>
              </tr>
            </thead>
            <tbody>
              {decisions.map((decision) => (
                <DecisionRow key={decision.id} decision={decision} />
              ))}
            </tbody>
          </table>
        </div>
      </AsyncBoundary>
    </div>
  );
}

function DecisionRow({ decision }: { decision: Decision }) {
  const actionClassName = getActionClassName(decision.action);
  const statusClassName = getStatusClassName(decision.executionStatus);
  const compositeScoreNumeric =
    decision.compositeScore != null ? parseFloat(decision.compositeScore) : null;
  const scoreClassName =
    compositeScoreNumeric !== null && compositeScoreNumeric > 0
      ? "score-positive"
      : compositeScoreNumeric !== null && compositeScoreNumeric < 0
      ? "score-negative"
      : "score-neutral";

  return (
    <tr>
      <td>{formatDateTime(decision.createdAt)}</td>
      <td className="symbol-cell">{decision.symbol}</td>
      <td>
        <span className={`action-badge ${actionClassName}`}>{decision.action}</span>
        {decision.isDryRun && (
          <span className="dry-run-badge" title="Dry run — no real order placed">DRY</span>
        )}
      </td>
      <td className="numeric-cell">
        <ConfidenceBar confidence={decision.confidence} />
      </td>
      <td className={`numeric-cell ${scoreClassName}`}>
        {compositeScoreNumeric !== null && !Number.isNaN(compositeScoreNumeric)
          ? `${compositeScoreNumeric >= 0 ? "+" : ""}${compositeScoreNumeric.toFixed(1)}`
          : "—"}
      </td>
      <td>
        <span className={`status-badge ${statusClassName}`}>{formatStatus(decision.executionStatus)}</span>
      </td>
      <td className="reason-cell">{decision.rejectionReason ?? "—"}</td>
      <td className="reasoning-cell">
        <ReasoningCell reasoning={decision.reasoning} />
      </td>
    </tr>
  );
}

function ReasoningCell({ reasoning }: { reasoning?: string | null }) {
  const [expanded, setExpanded] = useState(false);

  if (!reasoning) return <span className="muted">—</span>;

  const isTruncated = reasoning.length > TRUNCATE_LENGTH;
  const displayText =
    !expanded && isTruncated ? reasoning.slice(0, TRUNCATE_LENGTH) + "…" : reasoning;

  return (
    <span className="reasoning-text">
      {displayText}
      {isTruncated && (
        <button
          type="button"
          onClick={() => setExpanded((prev) => !prev)}
          className="expand-toggle"
        >
          {expanded ? "Show less" : "Show more"}
        </button>
      )}
    </span>
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
