import { useDashboardStore } from "../store";

type Theme = "light" | "dark" | "system";

const CYCLE: Theme[] = ["dark", "light", "system"];

const LABELS: Record<Theme, string> = {
  dark: "Dark",
  light: "Light",
  system: "System",
};

const ICONS: Record<Theme, string> = {
  dark: "🌙",
  light: "☀️",
  system: "⚙",
};

export function ThemeToggle() {
  const theme = useDashboardStore((s) => s.theme);
  const setTheme = useDashboardStore((s) => s.setTheme);

  function handleClick() {
    const currentIndex = CYCLE.indexOf(theme);
    const nextIndex = (currentIndex + 1) % CYCLE.length;
    setTheme(CYCLE[nextIndex]);
  }

  return (
    <button
      type="button"
      onClick={handleClick}
      aria-label={`Theme: ${LABELS[theme]}. Click to cycle theme.`}
      aria-pressed={theme !== "system"}
      className="theme-toggle"
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: "0.375rem",
        padding: "0.375rem 0.625rem",
        borderRadius: "var(--radius-sm, 6px)",
        border: "1px solid var(--color-border, rgba(255,255,255,0.12))",
        background: "var(--color-bg-elevated, rgba(255,255,255,0.06))",
        color: "var(--color-text-primary, inherit)",
        fontSize: "0.8125rem",
        fontFamily: "inherit",
        cursor: "pointer",
        userSelect: "none",
        transition: "background 120ms ease, border-color 120ms ease",
      }}
    >
      <span aria-hidden="true" style={{ fontSize: "1rem", lineHeight: 1 }}>
        {ICONS[theme]}
      </span>
      <span>{LABELS[theme]}</span>
    </button>
  );
}
