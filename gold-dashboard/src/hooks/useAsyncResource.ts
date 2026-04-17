import { useState, useEffect, useRef, useCallback } from "react";

export type AsyncState = "loading" | "empty" | "error" | "ready";

export interface AsyncResource<T> {
  state: AsyncState;
  data: T | null;
  error: string | null;
  retry: () => void;
}

export function useAsyncResource<T>(
  fetcher: () => Promise<T>,
  isEmpty?: (data: T) => boolean,
): AsyncResource<T> {
  const [state, setState] = useState<AsyncState>("loading");
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retryCount, setRetryCount] = useState(0);

  // Keep fetcher and isEmpty in refs so the effect only re-runs on retry,
  // not every render when callers pass inline functions.
  const fetcherRef = useRef(fetcher);
  const isEmptyRef = useRef(isEmpty);

  useEffect(() => {
    fetcherRef.current = fetcher;
  });

  useEffect(() => {
    isEmptyRef.current = isEmpty;
  });

  useEffect(() => {
    let cancelled = false;

    setState("loading");
    setError(null);

    fetcherRef.current()
      .then((result) => {
        if (cancelled) return;

        const checkEmpty = isEmptyRef.current;
        if (checkEmpty !== undefined && checkEmpty(result)) {
          setData(result);
          setState("empty");
        } else {
          setData(result);
          setState("ready");
        }
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : String(err));
        setState("error");
      });

    return () => {
      cancelled = true;
    };
  }, [retryCount]);

  const retry = useCallback(() => {
    setRetryCount((c) => c + 1);
  }, []);

  return { state, data, error, retry };
}
