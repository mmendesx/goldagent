import { useState, useEffect, useRef } from "react";

export function useAnimatedNumber(target: number, duration = 200): number {
  const [displayed, setDisplayed] = useState(target);
  const startRef = useRef({ from: target, to: target, startTime: 0 });

  useEffect(() => {
    if (target === startRef.current.to) return;
    // Snapshot `displayed` at tween start — intentionally not in deps to avoid
    // re-triggering the effect mid-animation when displayed updates each frame.
    startRef.current = { from: displayed, to: target, startTime: performance.now() };

    let raf: number;
    function tick(now: number) {
      const elapsed = now - startRef.current.startTime;
      const progress = Math.min(elapsed / duration, 1);
      const eased = progress * (2 - progress); // ease-out quad
      setDisplayed(startRef.current.from + (startRef.current.to - startRef.current.from) * eased);
      if (progress < 1) raf = requestAnimationFrame(tick);
    }
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [target, duration]);

  return displayed;
}
