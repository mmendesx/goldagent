import { useEffect, useRef, useState } from "react";
import { useDashboardStore } from "../../store";
import "./OfflineBanner.css";

const SHOW_DELAY_MS = 3000;

export function OfflineBanner() {
  const connectionState = useDashboardStore((s) => s.connectionState);
  const reconnectAttempts = useDashboardStore((s) => s.reconnectAttempts);
  const [showBanner, setShowBanner] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    const isUnhealthy = connectionState === "closed" || connectionState === "reconnecting";

    if (isUnhealthy) {
      if (timerRef.current === null) {
        timerRef.current = setTimeout(() => {
          setShowBanner(true);
        }, SHOW_DELAY_MS);
      }
    } else {
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
      setShowBanner(false);
    }

    return () => {
      if (timerRef.current !== null) {
        clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    };
  }, [connectionState]);

  if (!showBanner) {
    return null;
  }

  return (
    <div className="offline-banner" role="status" aria-live="polite">
      <span className="offline-banner__icon" aria-hidden="true" />
      <span className="offline-banner__text">
        Reconnecting&hellip;
        {reconnectAttempts > 0 && (
          <span className="offline-banner__attempts">
            {" "}(attempt {reconnectAttempts})
          </span>
        )}
      </span>
    </div>
  );
}
