import { useReducedMotion } from "motion/react";
import CountUp from "../vendor/react-bits/CountUp";
import { VisuallyHidden } from "./VisuallyHidden";

export interface LiveNumberProps {
  value: number;
  from?: number;
  format?: (n: number) => string;
  duration?: number;
  decimalPlaces?: number;
  className?: string;
}

export function LiveNumber({
  value,
  from = 0,
  format = (n) => n.toFixed(2),
  duration = 1,
  decimalPlaces = 2,
  className,
}: LiveNumberProps) {
  const prefersReducedMotion = useReducedMotion();

  if (prefersReducedMotion) {
    return (
      <span aria-live="polite" className={className}>
        {format(value)}
      </span>
    );
  }

  return (
    <span className={className}>
      {/*
       * CountUp renders intermediate animated values that would spam
       * screen readers if aria-live were on the animated span directly.
       * We hide the animation from AT and announce only the final
       * formatted value through a visually-hidden polite region.
       */}
      <span aria-hidden="true">
        <CountUp
          from={from}
          to={value}
          duration={duration}
          decimalPlaces={decimalPlaces}
        />
      </span>
      <VisuallyHidden aria-live="polite">{format(value)}</VisuallyHidden>
    </span>
  );
}
