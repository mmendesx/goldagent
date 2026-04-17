"use client";
import * as React from "react";
import { motion } from "motion/react";

// ---------------------------------------------------------------------------
// Context
// ---------------------------------------------------------------------------

interface HighlightContextValue {
  activeValue: string | null;
  setActiveValue: (value: string | null) => void;
  hover: boolean;
  layoutId: string;
}

const HighlightCtx = React.createContext<HighlightContextValue | null>(null);

function useHighlightCtx(): HighlightContextValue {
  const ctx = React.useContext(HighlightCtx);
  if (!ctx) throw new Error("HighlightItem must be used inside Highlight");
  return ctx;
}

// ---------------------------------------------------------------------------
// Highlight
// ---------------------------------------------------------------------------

let instanceCounter = 0;

export interface HighlightProps {
  children: React.ReactNode;
  hover?: boolean;
  className?: string;
  controlledItems?: string | null;
}

export function Highlight({
  children,
  hover = false,
  className,
  controlledItems,
}: HighlightProps): React.ReactElement {
  // Stable per-instance layoutId to avoid collisions across multiple Highlights
  const layoutId = React.useRef<string>(`highlight-${++instanceCounter}`).current;
  const [activeValue, setActiveValue] = React.useState<string | null>(
    controlledItems !== undefined ? controlledItems : null,
  );

  // Sync controlled value
  React.useEffect(() => {
    if (controlledItems !== undefined) {
      setActiveValue(controlledItems);
    }
  }, [controlledItems]);

  return (
    <HighlightCtx.Provider value={{ activeValue, setActiveValue, hover, layoutId }}>
      <div className={className} style={{ position: "relative" }}>
        {children}
      </div>
    </HighlightCtx.Provider>
  );
}

// ---------------------------------------------------------------------------
// HighlightItem
// ---------------------------------------------------------------------------

export interface HighlightItemProps {
  children: React.ReactNode;
  value: string;
  className?: string;
}

export function HighlightItem({
  children,
  value,
  className,
}: HighlightItemProps): React.ReactElement {
  const { activeValue, setActiveValue, hover, layoutId } = useHighlightCtx();
  const isActive = activeValue === value;

  return (
    <span
      className={className}
      style={{ position: "relative", display: "inline-flex", cursor: "pointer" }}
      onMouseEnter={() => {
        if (hover) setActiveValue(value);
      }}
      onMouseLeave={() => {
        if (hover) setActiveValue(null);
      }}
      onClick={() => {
        if (!hover) setActiveValue(value);
      }}
    >
      {isActive && (
        <motion.span
          layoutId={layoutId}
          aria-hidden="true"
          style={{
            position: "absolute",
            inset: 0,
            borderRadius: "inherit",
            zIndex: 0,
            pointerEvents: "none",
          }}
          transition={{ type: "spring", bounce: 0.15, duration: 0.3 }}
        />
      )}
      <span style={{ position: "relative", zIndex: 1 }}>{children}</span>
    </span>
  );
}
