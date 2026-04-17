import { BrowserRouter, Routes, Route } from "react-router-dom";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { Dashboard } from "./pages/Dashboard/Dashboard";

export function App() {
  return (
    <ErrorBoundary>
      <BrowserRouter>
        <Routes>
          <Route path="/*" element={<Dashboard />} />
        </Routes>
      </BrowserRouter>
    </ErrorBoundary>
  );
}
