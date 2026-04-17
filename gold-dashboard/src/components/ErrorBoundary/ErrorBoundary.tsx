import React from "react";
import "./ErrorBoundary.css";

interface ErrorBoundaryProps {
  children: React.ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error };
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="error-boundary" role="alert">
          <h2 className="error-boundary__heading">Something went wrong</h2>
          <p className="error-boundary__message">{this.state.error?.message}</p>
          <button
            className="error-boundary__reload"
            onClick={() => window.location.reload()}
          >
            Reload dashboard
          </button>
        </div>
      );
    }

    return this.props.children;
  }
}
