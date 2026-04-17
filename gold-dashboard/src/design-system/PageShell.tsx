import React from "react";
import { SkipLink } from "./SkipLink";

if (typeof document !== "undefined" && !document.getElementById("ds-page-shell-styles")) {
  const el = document.createElement("style");
  el.id = "ds-page-shell-styles";
  el.textContent = `
    .page-shell {
      display: flex;
      flex-direction: column;
      min-height: 100dvh;
      min-height: 100vh;
      background: var(--color-bg-base);
      color: var(--color-text-primary);
    }

    .page-shell__header {
      position: sticky;
      top: 0;
      z-index: var(--z-sticky, 10);
      flex-shrink: 0;
      background: var(--color-bg-elevated);
      border-bottom: 1px solid var(--color-border);
    }

    .page-shell__main {
      flex: 1 1 auto;
      overflow-y: auto;
      outline: none;
    }
  `;
  document.head.appendChild(el);
}

export interface PageShellProps {
  header: React.ReactNode;
  children: React.ReactNode;
}

export function PageShell({ header, children }: PageShellProps) {
  return (
    <>
      <SkipLink href="#main" />
      <div className="page-shell">
        <header role="banner" className="page-shell__header">
          {header}
        </header>
        <main id="main" role="main" tabIndex={-1} className="page-shell__main">
          {children}
        </main>
      </div>
    </>
  );
}
