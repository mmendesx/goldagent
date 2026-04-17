import React from "react";
import {
  Tabs,
  TabsList,
  TabsTrigger,
  TabsContent,
  TabsContents,
  TabsHighlight,
  TabsHighlightItem,
} from "../vendor/animate-ui/primitives/radix/tabs";

export interface AnimatedTabsProps {
  tabs: Array<{
    value: string;
    label: string;
    content: React.ReactNode;
  }>;
  defaultValue?: string;
  value?: string;
  onValueChange?: (value: string) => void;
  variant?: "highlight" | "plain";
  orientation?: "horizontal" | "vertical";
  className?: string;
  tabsListClassName?: string;
  contentClassName?: string;
}

export function AnimatedTabs({
  tabs,
  defaultValue,
  value,
  onValueChange,
  variant = "highlight",
  orientation = "horizontal",
  className,
  tabsListClassName,
  contentClassName,
}: AnimatedTabsProps) {
  // Determine whether this is controlled or uncontrolled.
  const isControlled = value !== undefined;
  const [internalValue, setInternalValue] = React.useState(
    defaultValue ?? tabs[0]?.value ?? ""
  );
  const activeValue = isControlled ? value! : internalValue;

  function handleValueChange(next: string) {
    if (!isControlled) setInternalValue(next);
    onValueChange?.(next);
  }

  const triggerList =
    variant === "highlight" ? (
      <TabsHighlight value={activeValue} className={tabsListClassName}>
        {tabs.map((tab) => (
          <TabsHighlightItem key={tab.value} value={tab.value}>
            <TabsTrigger
              value={tab.value}
              id={`tab-${tab.value}`}
              aria-controls={`panel-${tab.value}`}
            >
              {tab.label}
            </TabsTrigger>
          </TabsHighlightItem>
        ))}
      </TabsHighlight>
    ) : (
      <TabsList className={tabsListClassName}>
        {tabs.map((tab) => (
          <TabsTrigger
            key={tab.value}
            value={tab.value}
            id={`tab-${tab.value}`}
            aria-controls={`panel-${tab.value}`}
          >
            {tab.label}
          </TabsTrigger>
        ))}
      </TabsList>
    );

  return (
    <Tabs
      value={activeValue}
      onValueChange={handleValueChange}
      orientation={orientation}
      className={className}
    >
      {triggerList}

      <TabsContents activeValue={activeValue} className={contentClassName}>
        {tabs.map((tab) => (
          <TabsContent
            key={tab.value}
            value={tab.value}
            id={`panel-${tab.value}`}
            aria-labelledby={`tab-${tab.value}`}
          >
            {tab.content}
          </TabsContent>
        ))}
      </TabsContents>
    </Tabs>
  );
}
