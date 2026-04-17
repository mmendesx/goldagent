import React from "react";

export interface SkeletonProps extends React.HTMLAttributes<HTMLDivElement> {
  width?: string | number;
  height?: string | number;
  lines?: number;
}

export interface SkeletonContainerProps
  extends React.HTMLAttributes<HTMLDivElement> {
  children: React.ReactNode;
  busy: boolean;
}

if (typeof document !== "undefined" && !document.getElementById("ds-skeleton-styles")) {
  const el = document.createElement("style");
  el.id = "ds-skeleton-styles";
  el.textContent = `
    @keyframes ds-shimmer {
      0% {
        background-position: -200% 0;
      }
      100% {
        background-position: 200% 0;
      }
    }
    .ds-skeleton {
      display: block;
      border-radius: var(--radius-sm);
      background: linear-gradient(
        90deg,
        var(--color-bg-elevated) 25%,
        var(--color-bg-overlay) 50%,
        var(--color-bg-elevated) 75%
      );
      background-size: 200% 100%;
      animation: ds-shimmer 1.4s var(--motion-ease-in-out) infinite;
    }
    .ds-skeleton-lines {
      display: flex;
      flex-direction: column;
      gap: var(--space-2);
    }
    .ds-skeleton-lines .ds-skeleton:last-child {
      width: 60%;
    }
  `;
  document.head.appendChild(el);
}

export function Skeleton({ width, height, lines, className = "", style, ...rest }: SkeletonProps) {
  const resolvedWidth = width !== undefined ? (typeof width === "number" ? `${width}px` : width) : undefined;
  const resolvedHeight = height !== undefined ? (typeof height === "number" ? `${height}px` : height) : "1rem";

  if (lines !== undefined && lines > 1) {
    return (
      <div className={`ds-skeleton-lines${className ? ` ${className}` : ""}`} {...rest}>
        {Array.from({ length: lines }).map((_, i) => (
          <span
            key={i}
            className="ds-skeleton"
            style={{ width: resolvedWidth ?? "100%", height: resolvedHeight }}
          />
        ))}
      </div>
    );
  }

  const classes = ["ds-skeleton", className].filter(Boolean).join(" ");
  return (
    <span
      className={classes}
      style={{
        width: resolvedWidth ?? "100%",
        height: resolvedHeight,
        ...style,
      }}
      {...rest}
    />
  );
}

export function SkeletonContainer({
  children,
  busy,
  ...rest
}: SkeletonContainerProps) {
  return (
    <div aria-busy={busy} {...rest}>
      {children}
    </div>
  );
}
