import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { Dashboard } from "./pages/Dashboard/Dashboard";
import { ErrorBoundary } from "./components/ErrorBoundary";

function RootFallback({ error, reset }: { error: Error; reset: () => void }) {
  return (
    <div style={{ padding: "2rem", textAlign: "center" }}>
      <p style={{ color: "#fca5a5" }}>Dashboard error: {error.message}</p>
      <button onClick={() => window.location.reload()} style={{ marginRight: 8 }}>
        Reload
      </button>
      <button onClick={reset}>Retry</button>
    </div>
  );
}

export function App() {
  return (
    <BrowserRouter>
      <ErrorBoundary fallback={(error, reset) => <RootFallback error={error} reset={reset} />}>
        <Routes>
          <Route path="/*" element={<Dashboard />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </ErrorBoundary>
    </BrowserRouter>
  );
}
