import React from "react";

if (typeof document !== "undefined" && !document.getElementById("ds-metric-card-styles")) {
  const el = document.createElement("style");
  el.id = "ds-metric-card-styles";
  el.textContent = `
    .metric-card {
      display: flex;
      flex-direction: column;
      gap: var(--space-1);
      padding: var(--space-3) var(--space-4);
      background: var(--color-bg-elevated);
      border: 1px solid var(--color-border-subtle);
      border-radius: var(--radius-md);
      min-width: 0;
    }

    .metric-card__label {
      font-family: var(--font-sans);
      font-size: var(--font-size-xs);
      font-weight: var(--font-weight-medium);
      color: var(--color-text-muted);
      text-transform: uppercase;
      letter-spacing: 0.06em;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .metric-card__value {
      font-family: var(--font-mono, var(--font-sans));
      font-size: var(--font-size-lg);
      font-weight: var(--font-weight-semibold);
      color: var(--color-text-primary);
      line-height: 1.2;
    }

    .metric-card--severity-high .metric-card__value {
      color: var(--color-accent-danger);
    }

    .metric-card--severity-medium .metric-card__value {
      color: var(--color-accent-warning);
    }

    .metric-card__badge {
      margin-top: var(--space-1);
    }
  `;
  document.head.appendChild(el);
}

export interface MetricCardProps {
  label: string;
  value: React.ReactNode;
  badge?: React.ReactNode;
  severity?: "medium" | "high";
  className?: string;
}

export function MetricCard({ label, value, badge, severity, className }: MetricCardProps) {
  const severityClass = severity ? ` metric-card--severity-${severity}` : "";
  const classes = ["metric-card", severityClass, className].filter(Boolean).join(" ");

  return (
    <div className={classes} aria-label={`${label}`}>
      <span className="metric-card__label">{label}</span>
      <div className="metric-card__value">{value}</div>
      {badge != null && <div className="metric-card__badge">{badge}</div>}
    </div>
  );
}
