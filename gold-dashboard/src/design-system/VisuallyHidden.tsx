import React from "react";

if (typeof document !== "undefined" && !document.getElementById("ds-visually-hidden-styles")) {
  const el = document.createElement("style");
  el.id = "ds-visually-hidden-styles";
  el.textContent = `
    .ds-visually-hidden {
      position: absolute;
      width: 1px;
      height: 1px;
      padding: 0;
      margin: -1px;
      overflow: hidden;
      clip: rect(0, 0, 0, 0);
      white-space: nowrap;
      border: 0;
    }
  `;
  document.head.appendChild(el);
}

export interface VisuallyHiddenProps extends React.HTMLAttributes<HTMLElement> {
  children: React.ReactNode;
  as?: React.ElementType;
}

export function VisuallyHidden({
  children,
  as,
  className = "",
  ...props
}: VisuallyHiddenProps) {
  const Tag = as ?? "span";
  const classes = ["ds-visually-hidden", className].filter(Boolean).join(" ");
  return (
    <Tag className={classes} {...props}>
      {children}
    </Tag>
  );
}
