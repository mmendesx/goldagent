import React from "react";

if (typeof document !== "undefined" && !document.getElementById("ds-skip-link-styles")) {
  const el = document.createElement("style");
  el.id = "ds-skip-link-styles";
  el.textContent = `
    .ds-skip-link {
      position: fixed;
      top: var(--space-3);
      left: var(--space-3);
      z-index: var(--z-overlay);
      padding: var(--space-2) var(--space-4);
      background: var(--color-accent-brand);
      color: var(--color-text-inverse);
      font-family: var(--font-sans);
      font-size: var(--font-size-base);
      font-weight: var(--font-weight-semibold);
      border-radius: var(--radius-md);
      border: 2px solid transparent;
      text-decoration: none;
      /* Hidden until focused */
      transform: translateY(-200%);
      transition: transform var(--motion-duration-fast) var(--motion-ease-out);
    }
    .ds-skip-link:focus-visible {
      transform: translateY(0);
      outline: 2px solid var(--color-focus-ring);
      outline-offset: 2px;
    }
  `;
  document.head.appendChild(el);
}

export interface SkipLinkProps {
  href?: string;
}

export function SkipLink({ href = "#main" }: SkipLinkProps) {
  const targetId = href.startsWith("#") ? href.slice(1) : href;

  function handleClick(e: React.MouseEvent<HTMLAnchorElement>) {
    e.preventDefault();
    const target = document.getElementById(targetId);
    if (target !== null) {
      // Ensure the target is focusable — consumers should set tabindex="-1" on <main>.
      // We set it defensively here if not already set.
      if (!target.hasAttribute("tabindex")) {
        target.setAttribute("tabindex", "-1");
      }
      target.focus();
    }
  }

  return (
    <a href={href} className="ds-skip-link" onClick={handleClick}>
      Skip to main content
    </a>
  );
}
