import { useEffect } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { MotionConfig } from "motion/react";
import { Dashboard } from "./pages/Dashboard/Dashboard";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { useDashboardStore } from "./store";
import { Button } from "./design-system";

function RootFallback({ error, reset }: { error: Error; reset: () => void }) {
  return (
    <div style={{ padding: "2rem", textAlign: "center" }}>
      <p style={{ color: "var(--color-accent-danger)" }}>Dashboard error: {error.message}</p>
      <Button onClick={() => window.location.reload()} style={{ marginRight: 8 }}>
        Reload
      </Button>
      <Button onClick={reset}>Retry</Button>
    </div>
  );
}

function resolveTheme(theme: "light" | "dark" | "system"): "light" | "dark" {
  if (theme !== "system") return theme;
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function App() {
  const theme = useDashboardStore((s) => s.theme);

  useEffect(() => {
    const apply = () => {
      document.documentElement.setAttribute("data-theme", resolveTheme(theme));
    };
    apply();

    if (theme === "system") {
      const mq = window.matchMedia("(prefers-color-scheme: dark)");
      mq.addEventListener("change", apply);
      return () => mq.removeEventListener("change", apply);
    }
  }, [theme]);

  return (
    <MotionConfig reducedMotion="user">
      <BrowserRouter>
        <ErrorBoundary fallback={(error, reset) => <RootFallback error={error} reset={reset} />}>
          <Routes>
            <Route path="/*" element={<Dashboard />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </ErrorBoundary>
      </BrowserRouter>
    </MotionConfig>
  );
}
