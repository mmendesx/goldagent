import React from "react";
import type { AsyncState } from "../../hooks";
import { Skeleton } from "../Skeleton";
import "./AsyncBoundary.css";

export interface AsyncBoundaryProps {
  state: AsyncState;
  onRetry?: () => void;
  emptyCopy?: string;
  loadingSkeleton?: React.ReactNode;
  errorMessage?: string;
  children: React.ReactNode;
}

export function AsyncBoundary({
  state,
  onRetry,
  emptyCopy = "No data available",
  loadingSkeleton,
  errorMessage = "Something went wrong",
  children,
}: AsyncBoundaryProps) {
  if (state === "loading") {
    return (
      <>
        {loadingSkeleton ?? <Skeleton variant="block" height="200px" />}
      </>
    );
  }

  if (state === "empty") {
    return (
      <div className="async-boundary-empty">
        {emptyCopy}
      </div>
    );
  }

  if (state === "error") {
    return (
      <div className="async-boundary-error" role="alert">
        <p className="async-boundary-error-message">{errorMessage}</p>
        {onRetry !== undefined && (
          <button
            type="button"
            className="async-boundary-retry-button"
            onClick={onRetry}
          >
            Retry
          </button>
        )}
      </div>
    );
  }

  return <>{children}</>;
}
