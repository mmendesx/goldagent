import { useEffect, useRef, useState } from "react";
import { useReducedMotion } from "motion/react";
import CountUp from "../vendor/react-bits/CountUp";
import { VisuallyHidden } from "./VisuallyHidden";

const ANNOUNCE_THROTTLE_MS = 3000;

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
  const formatRef = useRef(format);
  useEffect(() => { formatRef.current = format; });

  const [announcedText, setAnnouncedText] = useState(() => format(value));
  const lastAnnounceRef = useRef<number>(Date.now());
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    const now = Date.now();
    const elapsed = now - lastAnnounceRef.current;

    if (elapsed >= ANNOUNCE_THROTTLE_MS) {
      lastAnnounceRef.current = now;
      setAnnouncedText(formatRef.current(value));
    } else {
      if (timerRef.current) clearTimeout(timerRef.current);
      timerRef.current = setTimeout(() => {
        lastAnnounceRef.current = Date.now();
        setAnnouncedText(formatRef.current(value));
        timerRef.current = null;
      }, ANNOUNCE_THROTTLE_MS - elapsed);
    }

    return () => {
      if (timerRef.current) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
  }, [value]);

  if (prefersReducedMotion) {
    return (
      <span className={className}>
        {format(value)}
        <VisuallyHidden aria-live="polite" aria-atomic="true">
          {announcedText}
        </VisuallyHidden>
      </span>
    );
  }

  return (
    <span className={className}>
      <span aria-hidden="true">
        <CountUp
          from={from}
          to={value}
          duration={duration}
          decimalPlaces={decimalPlaces}
        />
      </span>
      <VisuallyHidden aria-live="polite" aria-atomic="true">
        {announcedText}
      </VisuallyHidden>
    </span>
  );
}
