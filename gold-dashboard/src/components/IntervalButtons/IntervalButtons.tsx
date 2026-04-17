import type { ChartInterval } from "../../types";
import "./IntervalButtons.css";

const AVAILABLE_INTERVALS: ChartInterval[] = ["1m", "5m", "15m", "1h"];

interface IntervalButtonsProps {
  interval: ChartInterval;
  onIntervalChange: (interval: ChartInterval) => void;
}

export function IntervalButtons({ interval, onIntervalChange }: IntervalButtonsProps) {
  return (
    <div className="interval-buttons" role="radiogroup" aria-label="Chart interval">
      {AVAILABLE_INTERVALS.map((iv) => (
        <button
          key={iv}
          type="button"
          role="radio"
          aria-checked={iv === interval}
          className={iv === interval ? "interval-button active" : "interval-button"}
          onClick={() => onIntervalChange(iv)}
        >
          {iv}
        </button>
      ))}
    </div>
  );
}
