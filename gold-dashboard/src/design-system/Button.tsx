import React from "react";

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "ghost";
  size?: "sm" | "md";
  ref?: React.Ref<HTMLButtonElement>;
}

if (typeof document !== "undefined" && !document.getElementById("ds-button-styles")) {
  const el = document.createElement("style");
  el.id = "ds-button-styles";
  el.textContent = `
    .ds-btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: var(--space-2);
      border: 1px solid transparent;
      border-radius: var(--radius-md);
      font-family: var(--font-sans);
      font-weight: var(--font-weight-medium);
      cursor: pointer;
      text-decoration: none;
      transition:
        background var(--motion-duration-fast) var(--motion-ease-out),
        border-color var(--motion-duration-fast) var(--motion-ease-out),
        color var(--motion-duration-fast) var(--motion-ease-out),
        opacity var(--motion-duration-fast) var(--motion-ease-out);
      user-select: none;
      white-space: nowrap;
      line-height: 1;
    }
    .ds-btn:focus-visible {
      outline: 2px solid var(--color-focus-ring);
      outline-offset: 2px;
    }
    /* Size: sm */
    .ds-btn--sm {
      padding: var(--space-2) var(--space-3);
      font-size: var(--font-size-sm);
    }
    /* Size: md */
    .ds-btn--md {
      padding: var(--space-2) var(--space-4);
      font-size: var(--font-size-base);
    }
    /* Variant: primary */
    .ds-btn--primary {
      background: var(--color-accent-brand);
      color: var(--color-text-inverse);
      border-color: var(--color-accent-brand);
    }
    .ds-btn--primary:hover:not([disabled]):not([aria-disabled="true"]) {
      background: var(--color-accent-hover, color-mix(in srgb, var(--color-accent-brand) 85%, black));
      border-color: var(--color-accent-hover, color-mix(in srgb, var(--color-accent-brand) 85%, black));
    }
    /* Variant: secondary */
    .ds-btn--secondary {
      background: transparent;
      color: var(--color-text-primary);
      border-color: var(--color-border);
    }
    .ds-btn--secondary:hover:not([disabled]):not([aria-disabled="true"]) {
      background: var(--color-bg-elevated);
      border-color: var(--color-accent-brand);
    }
    /* Variant: ghost */
    .ds-btn--ghost {
      background: transparent;
      color: var(--color-text-secondary);
      border-color: transparent;
    }
    .ds-btn--ghost:hover:not([disabled]):not([aria-disabled="true"]) {
      background: var(--color-bg-elevated);
      color: var(--color-text-primary);
    }
    /* Disabled state */
    .ds-btn[disabled],
    .ds-btn[aria-disabled="true"] {
      opacity: 0.45;
      cursor: not-allowed;
      pointer-events: none;
    }
  `;
  document.head.appendChild(el);
}

export function Button({
  variant = "primary",
  size = "md",
  className = "",
  disabled,
  children,
  ref,
  ...rest
}: ButtonProps) {
  const classes = [
    "ds-btn",
    `ds-btn--${size}`,
    `ds-btn--${variant}`,
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <button
      ref={ref}
      type="button"
      className={classes}
      disabled={disabled}
      aria-disabled={disabled ? true : undefined}
      {...rest}
    >
      {children}
    </button>
  );
}
