import { useEffect, useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { useDashboardStore } from "../../store";
import { restClient } from "../../api";
import { useListFetch } from "../../hooks/useListFetch";
import { Skeleton, SkeletonContainer, VisuallyHidden } from "../../design-system";
import type { Decision, TradingSymbol } from "../../types";
import "./DecisionLog.css";

const PAGE_SIZE = 100;
const AVAILABLE_SYMBOLS: (TradingSymbol | "ALL")[] = ["ALL", "BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"];

export function DecisionLog() {
  const decisions = useDashboardStore((state) => state.decisions);
  const setDecisions = useDashboardStore((state) => state.setDecisions);
  const [symbolFilter, setSymbolFilter] = useState<TradingSymbol | "ALL">("ALL");

  const symbolParam = symbolFilter === "ALL" ? "" : symbolFilter;
  const fetchKey = `decisions-${symbolParam}`;
  const { data, error, loading, refetch } = useListFetch(
    fetchKey,
    () => restClient.fetchDecisions(symbolParam || undefined, PAGE_SIZE, 0),
    [symbolFilter],
  );

  // Sync REST results into the store so live WS decisions augment the initial load
  useEffect(() => {
    if (data?.items) setDecisions(data.items);
  }, [data, setDecisions]);

  // Filter live-updated decisions client-side by symbol
  const filteredDecisions = symbolFilter === "ALL"
    ? decisions
    : decisions.filter((decision) => decision.symbol === symbolFilter);

  // Show skeleton only on the initial load when there's nothing in the store yet
  const isInitialLoading = loading && decisions.length === 0;

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

      <SkeletonContainer busy={isInitialLoading}>
        {isInitialLoading ? (
          <Skeleton lines={5} className="decision-log-skeleton" />
        ) : error ? (
          <div className="decision-log-error" role="alert">
            <span>{error}</span>
            <button type="button" onClick={refetch} className="decision-log-retry">Retry</button>
          </div>
        ) : filteredDecisions.length === 0 ? (
          <div className="decision-log-empty">
            No decisions yet. The engine logs every evaluation, including HOLDs.
          </div>
        ) : (
          <div className="decision-log-table-container">
            <table className="decision-log-table">
              <caption><VisuallyHidden>Decision Log</VisuallyHidden></caption>
              <thead>
                <tr>
                  <th scope="col">Time</th>
                  <th scope="col">Symbol</th>
                  <th scope="col">Action</th>
                  <th scope="col">Confidence</th>
                  <th scope="col">Score</th>
                  <th scope="col">Status</th>
                  <th scope="col">Reason</th>
                </tr>
              </thead>
              <tbody>
                <AnimatePresence>
                  {filteredDecisions.map((decision, i) => (
                    <DecisionRow key={decision.id ?? `${decision.symbol}-${i}`} decision={decision} index={i} />
                  ))}
                </AnimatePresence>
              </tbody>
            </table>
          </div>
        )}
      </SkeletonContainer>
    </div>
  );
}

function DecisionRow({ decision, index }: { decision: Decision; index: number }) {
  const actionClassName = getActionClassName(decision.action);
  const statusClassName = getStatusClassName(decision.executionStatus);
  const compositeScoreNumeric = parseFloat(decision.compositeScore ?? "0");
  const scoreClassName = compositeScoreNumeric > 0 ? "score-positive" : compositeScoreNumeric < 0 ? "score-negative" : "score-neutral";

  return (
    <motion.tr
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.15, delay: index * 0.04 }}
    >
      <td>{decision.createdAt ? formatDateTime(decision.createdAt) : "—"}</td>
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
    </motion.tr>
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
