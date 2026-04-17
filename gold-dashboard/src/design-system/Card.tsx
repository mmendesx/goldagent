import React from "react";

export interface CardProps extends React.HTMLAttributes<HTMLElement> {
  children: React.ReactNode;
  as?: React.ElementType;
}

if (typeof document !== "undefined" && !document.getElementById("ds-card-styles")) {
  const el = document.createElement("style");
  el.id = "ds-card-styles";
  el.textContent = `
    .ds-card {
      background: var(--color-bg-elevated);
      border-radius: var(--radius-md);
      box-shadow: var(--shadow-md);
      border: 1px solid var(--color-border-subtle);
      overflow: hidden;
    }
  `;
  document.head.appendChild(el);
}

export function Card({ children, as, className = "", ...rest }: CardProps) {
  const Tag = as ?? "div";
  const classes = ["ds-card", className].filter(Boolean).join(" ");
  return (
    <Tag className={classes} {...rest}>
      {children}
    </Tag>
  );
}
