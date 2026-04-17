import { useState } from "react";
import type { Decision } from "../../types";
import "./LatestSignal.css";

const REASONING_TRUNCATE_LENGTH = 240;

interface LatestSignalProps {
  decision: Decision | null;
}

export function LatestSignal({ decision }: LatestSignalProps) {
  if (decision === null) {
    return (
      <section className="latest-signal latest-signal--empty" aria-label="Latest signal">
        <p className="latest-signal__awaiting">Awaiting signal…</p>
      </section>
    );
  }

  return (
    <section className="latest-signal" aria-label="Latest signal">
      <header className="latest-signal__header">
        <div className="latest-signal__action-group">
          <span className={`latest-signal__action-badge latest-signal__action-badge--${decision.action.toLowerCase()}`}>
            {decision.action}
          </span>
          <span className="latest-signal__symbol">{decision.symbol}</span>
          {decision.isDryRun && (
            <span className="latest-signal__dry-run-badge" title="Dry run — no real order placed">
              DRY-RUN
            </span>
          )}
        </div>
        <time
          className="latest-signal__timestamp"
          dateTime={decision.createdAt}
          title={new Date(decision.createdAt).toLocaleString()}
        >
          {formatRelativeTime(decision.createdAt)}
        </time>
      </header>

      <div className="latest-signal__body">
        <ConfidenceRow confidence={decision.confidence} />
        <CompositeScoreRow compositeScore={decision.compositeScore} />
        <StatusRow executionStatus={decision.executionStatus} />
        <ReasoningRow reasoning={decision.reasoning} />
      </div>
    </section>
  );
}

function ConfidenceRow({ confidence }: { confidence: number }) {
  const clamped = Math.min(100, Math.max(0, confidence));
  const severity = clamped >= 70 ? "high" : clamped >= 40 ? "medium" : "low";

  return (
    <div className="latest-signal__row">
      <span className="latest-signal__label">Confidence</span>
      <div className="latest-signal__confidence">
        <div
          className="latest-signal__confidence-track"
          role="progressbar"
          aria-valuenow={clamped}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-label={`Confidence: ${clamped}%`}
        >
          <div
            className={`latest-signal__confidence-fill latest-signal__confidence-fill--${severity}`}
            style={{ width: `${clamped}%` }}
          />
        </div>
        <span className="latest-signal__confidence-value">{clamped}</span>
      </div>
    </div>
  );
}

function CompositeScoreRow({ compositeScore }: { compositeScore: string }) {
  const numeric = parseFloat(compositeScore);
  if (Number.isNaN(numeric)) return null;

  const scoreClass =
    numeric > 0
      ? "latest-signal__score--positive"
      : numeric < 0
      ? "latest-signal__score--negative"
      : "latest-signal__score--neutral";

  return (
    <div className="latest-signal__row">
      <span className="latest-signal__label">Score</span>
      <span className={`latest-signal__score ${scoreClass}`}>
        {numeric >= 0 ? "+" : ""}{numeric.toFixed(2)}
      </span>
    </div>
  );
}

function StatusRow({ executionStatus }: { executionStatus: string }) {
  return (
    <div className="latest-signal__row">
      <span className="latest-signal__label">Status</span>
      <span className="latest-signal__status">{formatStatus(executionStatus)}</span>
    </div>
  );
}

function ReasoningRow({ reasoning }: { reasoning?: string | null }) {
  const [expanded, setExpanded] = useState(false);

  if (!reasoning) return null;

  const isTruncated = reasoning.length > REASONING_TRUNCATE_LENGTH;
  const displayText =
    !expanded && isTruncated
      ? reasoning.slice(0, REASONING_TRUNCATE_LENGTH) + "…"
      : reasoning;

  return (
    <div className="latest-signal__row latest-signal__row--reasoning">
      <span className="latest-signal__label">Reasoning</span>
      <span className="latest-signal__reasoning">
        {displayText}
        {isTruncated && (
          <button
            type="button"
            className="latest-signal__expand-toggle"
            onClick={() => setExpanded((prev) => !prev)}
          >
            {expanded ? "Show less" : "Show more"}
          </button>
        )}
      </span>
    </div>
  );
}

function formatRelativeTime(timestamp: string): string {
  const diff = Date.now() - new Date(timestamp).getTime();
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  return `${hours}h ago`;
}

function formatStatus(status: string): string {
  return status.replace(/_/g, " ");
}
