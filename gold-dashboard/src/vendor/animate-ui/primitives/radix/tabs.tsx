"use client";
import * as React from "react";
import * as TabsPrimitive from "@radix-ui/react-tabs";
import { motion, AnimatePresence } from "motion/react";

// ---------------------------------------------------------------------------
// TabsList
// ---------------------------------------------------------------------------

export const TabsList = React.forwardRef<
  React.ElementRef<typeof TabsPrimitive.List>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.List>
>(({ className, ...props }, ref) => (
  <TabsPrimitive.List
    ref={ref}
    role="tablist"
    className={className}
    {...props}
  />
));
TabsList.displayName = "TabsList";

// ---------------------------------------------------------------------------
// TabsTrigger
// ---------------------------------------------------------------------------

export const TabsTrigger = React.forwardRef<
  React.ElementRef<typeof TabsPrimitive.Trigger>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.Trigger>
>(({ className, onKeyDown, ...props }, ref) => {
  function handleKeyDown(event: React.KeyboardEvent<HTMLButtonElement>) {
    const tablist = (event.currentTarget as HTMLElement).closest('[role="tablist"]');
    if (!tablist) {
      onKeyDown?.(event);
      return;
    }
    const tabs = Array.from(tablist.querySelectorAll<HTMLElement>('[role="tab"]'));
    const currentIndex = tabs.indexOf(event.currentTarget as HTMLElement);

    switch (event.key) {
      case "ArrowRight": {
        event.preventDefault();
        const next = tabs[(currentIndex + 1) % tabs.length];
        next?.focus();
        break;
      }
      case "ArrowLeft": {
        event.preventDefault();
        const prev = tabs[(currentIndex - 1 + tabs.length) % tabs.length];
        prev?.focus();
        break;
      }
      case "Home": {
        event.preventDefault();
        tabs[0]?.focus();
        break;
      }
      case "End": {
        event.preventDefault();
        tabs[tabs.length - 1]?.focus();
        break;
      }
      default:
        onKeyDown?.(event);
    }
  }

  return (
    <TabsPrimitive.Trigger
      ref={ref}
      role="tab"
      className={className}
      onKeyDown={handleKeyDown}
      {...props}
    />
  );
});
TabsTrigger.displayName = "TabsTrigger";

// ---------------------------------------------------------------------------
// TabsContent
// ---------------------------------------------------------------------------

export const TabsContent = React.forwardRef<
  React.ElementRef<typeof TabsPrimitive.Content>,
  React.ComponentPropsWithoutRef<typeof TabsPrimitive.Content>
>(({ className, ...props }, ref) => (
  <TabsPrimitive.Content
    ref={ref}
    role="tabpanel"
    className={className}
    {...props}
  />
));
TabsContent.displayName = "TabsContent";

// ---------------------------------------------------------------------------
// TabsContents — animated wrapper using AnimatePresence
// ---------------------------------------------------------------------------

export interface TabsContentsProps {
  children: React.ReactNode;
  activeValue: string;
  className?: string;
}

export function TabsContents({ children, activeValue, className }: TabsContentsProps): React.ReactElement {
  return (
    <AnimatePresence mode="wait">
      {React.Children.map(children, (child) => {
        if (!React.isValidElement(child)) return null;
        const childValue = (child.props as { value?: string }).value;
        if (childValue !== activeValue) return null;
        return (
          <motion.div
            key={activeValue}
            className={className}
            initial={{ opacity: 0, y: 4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{
              duration: 0.12,
              ease: [0, 0, 0.2, 1],
            }}
          >
            {child}
          </motion.div>
        );
      })}
    </AnimatePresence>
  );
}

// ---------------------------------------------------------------------------
// HighlightContext — shared between TabsHighlight and TabsHighlightItem
// ---------------------------------------------------------------------------

interface HighlightContextValue {
  activeValue: string;
  hoveredValue: string | null;
  setHoveredValue: (value: string | null) => void;
  layoutId: string;
}

const HighlightContext = React.createContext<HighlightContextValue | null>(null);

function useHighlightContext(): HighlightContextValue {
  const ctx = React.useContext(HighlightContext);
  if (!ctx) throw new Error("TabsHighlightItem must be used inside TabsHighlight");
  return ctx;
}

// ---------------------------------------------------------------------------
// TabsHighlight — TabsList variant with sliding highlight indicator
// ---------------------------------------------------------------------------

export interface TabsHighlightProps {
  children: React.ReactNode;
  value: string;
  layoutId?: string;
  className?: string;
}

export function TabsHighlight({
  children,
  value,
  layoutId = "tab-highlight",
  className,
}: TabsHighlightProps): React.ReactElement {
  const [hoveredValue, setHoveredValue] = React.useState<string | null>(null);

  return (
    <HighlightContext.Provider value={{ activeValue: value, hoveredValue, setHoveredValue, layoutId }}>
      <TabsPrimitive.List role="tablist" className={className} style={{ position: "relative" }}>
        {children}
      </TabsPrimitive.List>
    </HighlightContext.Provider>
  );
}

// ---------------------------------------------------------------------------
// TabsHighlightItem — wraps a trigger with hover tracking + highlight span
// ---------------------------------------------------------------------------

export interface TabsHighlightItemProps {
  children: React.ReactNode;
  value: string;
  className?: string;
}

export function TabsHighlightItem({
  children,
  value,
  className,
}: TabsHighlightItemProps): React.ReactElement {
  const { activeValue, hoveredValue, setHoveredValue, layoutId } = useHighlightContext();
  const isActive = activeValue === value;
  const isHovered = hoveredValue === value;
  const showHighlight = isActive || isHovered;

  return (
    <span
      className={className}
      style={{ position: "relative", display: "inline-flex" }}
      onMouseEnter={() => setHoveredValue(value)}
      onMouseLeave={() => setHoveredValue(null)}
    >
      {showHighlight && (
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

// ---------------------------------------------------------------------------
// Root re-export
// ---------------------------------------------------------------------------

export const Tabs = TabsPrimitive.Root;
