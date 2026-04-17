"use client";
import * as React from "react";
import {
  useSpring,
  useMotionValue,
  useTransform,
  motion,
  useReducedMotion,
} from "motion/react";

export interface CountUpProps {
  from?: number;
  to: number;
  direction?: "up" | "down";
  separator?: string;
  duration?: number;
  delay?: number;
  className?: string;
  startWhen?: boolean;
  onStart?: () => void;
  onEnd?: () => void;
  decimalPlaces?: number;
}

function formatValue(
  value: number,
  separator: string,
  decimalPlaces: number,
): string {
  const fixed = value.toFixed(decimalPlaces);

  if (!separator) return fixed;

  const [intPart, decPart] = fixed.split(".");
  const formattedInt = intPart.replace(/\B(?=(\d{3})+(?!\d))/g, separator);

  return decPart !== undefined ? `${formattedInt}.${decPart}` : formattedInt;
}

export function CountUp({
  from = 0,
  to,
  direction = "up",
  separator = "",
  duration = 1.5,
  delay = 0,
  className,
  startWhen = true,
  onStart,
  onEnd,
  decimalPlaces = 0,
}: CountUpProps): React.ReactElement {
  const prefersReducedMotion = useReducedMotion();

  const initialValue = direction === "down" ? to : from;
  const targetValue = direction === "down" ? from : to;

  const motionValue = useMotionValue(initialValue);
  const spring = useSpring(motionValue, { duration, bounce: 0 });

  const displayText = useTransform(spring, (v) =>
    formatValue(v, separator, decimalPlaces),
  );

  React.useEffect(() => {
    if (prefersReducedMotion || !startWhen) return;

    const timer = setTimeout(() => {
      onStart?.();

      const unsubscribeComplete = spring.on("animationComplete", () => {
        onEnd?.();
        unsubscribeComplete();
      });

      motionValue.set(targetValue);
    }, delay * 1000);

    return () => {
      clearTimeout(timer);
    };
  }, [
    startWhen,
    prefersReducedMotion,
    delay,
    targetValue,
    motionValue,
    spring,
    onStart,
    onEnd,
  ]);

  if (prefersReducedMotion) {
    return (
      <span className={className}>
        {formatValue(targetValue, separator, decimalPlaces)}
      </span>
    );
  }

  return <motion.span className={className}>{displayText}</motion.span>;
}

export default CountUp;
