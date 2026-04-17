import { PriceChart } from "../../components/PriceChart/PriceChart";
import { PolymarketOverview } from "../../components/PolymarketOverview";
import { DecisionLog } from "../../components/DecisionLog/DecisionLog";
import { ErrorBoundary } from "../../components/ErrorBoundary";
import { AnimatedTabs } from "../../design-system";

const polymarketTabs = [
  {
    value: "chart",
    label: "Chart",
    content: (
      <ErrorBoundary>
        <PriceChart exchange="polymarket" />
      </ErrorBoundary>
    ),
  },
  {
    value: "overview",
    label: "Overview",
    content: (
      <ErrorBoundary>
        <PolymarketOverview />
      </ErrorBoundary>
    ),
  },
  {
    value: "decisions",
    label: "Decision Log",
    content: (
      <ErrorBoundary>
        <DecisionLog />
      </ErrorBoundary>
    ),
  },
];

export function PolymarketView() {
  return (
    <AnimatedTabs
      tabs={polymarketTabs}
      defaultValue="chart"
      variant="highlight"
    />
  );
}
