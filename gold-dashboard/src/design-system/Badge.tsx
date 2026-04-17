import React from "react";

export type BadgeVariant = "success" | "warning" | "danger" | "info" | "neutral";

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?: BadgeVariant;
  icon?: React.ReactNode;
}

if (typeof document !== "undefined" && !document.getElementById("ds-badge-styles")) {
  const el = document.createElement("style");
  el.id = "ds-badge-styles";
  el.textContent = `
    .ds-badge {
      display: inline-flex;
      align-items: center;
      gap: var(--space-1);
      padding: 2px var(--space-2);
      border-radius: var(--radius-sm);
      font-family: var(--font-sans);
      font-size: var(--font-size-xs);
      font-weight: var(--font-weight-medium);
      line-height: 1.5;
      white-space: nowrap;
    }
    .ds-badge--success {
      color: var(--color-accent-success);
      background: color-mix(in srgb, var(--color-accent-success) 15%, transparent);
    }
    .ds-badge--warning {
      color: var(--color-accent-warning);
      background: color-mix(in srgb, var(--color-accent-warning) 15%, transparent);
    }
    .ds-badge--danger {
      color: var(--color-accent-danger);
      background: color-mix(in srgb, var(--color-accent-danger) 15%, transparent);
    }
    .ds-badge--info {
      color: var(--color-accent-info);
      background: color-mix(in srgb, var(--color-accent-info) 15%, transparent);
    }
    .ds-badge--neutral {
      color: var(--color-text-secondary);
      background: var(--color-border-subtle);
    }
  `;
  document.head.appendChild(el);
}

export function Badge({
  variant = "neutral",
  icon,
  children,
  className = "",
  ...rest
}: BadgeProps) {
  const classes = ["ds-badge", `ds-badge--${variant}`, className]
    .filter(Boolean)
    .join(" ");

  return (
    <span className={classes} {...rest}>
      {icon !== undefined && (
        <span aria-hidden="true" style={{ display: "inline-flex", alignItems: "center" }}>
          {icon}
        </span>
      )}
      {children}
    </span>
  );
}
