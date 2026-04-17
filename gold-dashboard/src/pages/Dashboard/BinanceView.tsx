import { PriceChart } from "../../components/PriceChart/PriceChart";
import { OpenPositions } from "../../components/OpenPositions/OpenPositions";
import { TradeHistory } from "../../components/TradeHistory";
import { DecisionLog } from "../../components/DecisionLog/DecisionLog";
import { ErrorBoundary } from "../../components/ErrorBoundary";
import { AnimatedTabs } from "../../design-system";

const binanceTabs = [
  {
    value: "chart",
    label: "Chart",
    content: (
      <ErrorBoundary>
        <PriceChart exchange="binance" />
      </ErrorBoundary>
    ),
  },
  {
    value: "positions",
    label: "Open Positions",
    content: (
      <ErrorBoundary>
        <OpenPositions />
      </ErrorBoundary>
    ),
  },
  {
    value: "history",
    label: "Trade History",
    content: (
      <ErrorBoundary>
        <TradeHistory />
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

export function BinanceView() {
  return (
    <AnimatedTabs
      tabs={binanceTabs}
      defaultValue="chart"
      variant="highlight"
    />
  );
}
