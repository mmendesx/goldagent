import { useDashboardStore } from "../../store";
import "./ConnectionBadge.css";

export function ConnectionBadge() {
  const connectionState = useDashboardStore((s) => s.connectionState);
  const reconnectAttempts = useDashboardStore((s) => s.reconnectAttempts);

  const label =
    connectionState === "open"
      ? "Live"
      : connectionState === "reconnecting"
      ? reconnectAttempts > 0
        ? `Reconnecting (${reconnectAttempts})\u2026`
        : "Reconnecting\u2026"
      : connectionState === "connecting"
      ? "Connecting\u2026"
      : "Offline";

  return (
    <div
      className={`connection-badge connection-badge--${connectionState}`}
      aria-live="polite"
      aria-atomic="true"
    >
      <span className="connection-badge__dot" aria-hidden="true" />
      <span className="connection-badge__label">{label}</span>
    </div>
  );
}
