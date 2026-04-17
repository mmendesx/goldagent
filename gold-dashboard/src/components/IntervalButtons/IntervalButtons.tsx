import { useDashboardStore } from "../../store";
import type { ChartInterval } from "../../types";
import "./IntervalButtons.css";

const AVAILABLE_INTERVALS: ChartInterval[] = ["1m", "5m", "15m", "1h"];

export function IntervalButtons() {
  const selectedInterval = useDashboardStore((state) => state.selectedInterval);
  const setSelectedInterval = useDashboardStore((state) => state.setSelectedInterval);

  return (
    <div className="interval-buttons" role="radiogroup" aria-label="Chart interval">
      {AVAILABLE_INTERVALS.map((interval) => (
        <button
          key={interval}
          type="button"
          role="radio"
          aria-checked={interval === selectedInterval}
          className={interval === selectedInterval ? "interval-button active" : "interval-button"}
          onClick={() => setSelectedInterval(interval)}
        >
          {interval}
        </button>
      ))}
    </div>
  );
}
