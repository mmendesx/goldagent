import { useState, useEffect, useRef, useCallback } from 'react';
import { abortRequest } from '../api';

interface UseListFetchResult<T> {
  data: T | null;
  error: string | null;
  loading: boolean;
  refetch: () => void;
}

export function useListFetch<T>(
  fetchKey: string,
  fetchFn: (signal?: AbortSignal) => Promise<T>,
  deps: unknown[]
): UseListFetchResult<T> {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [refetchToken, setRefetchToken] = useState(0);
  const prevKeyRef = useRef<string | null>(null);

  useEffect(() => {
    // Abort the previous request if the key changed
    if (prevKeyRef.current && prevKeyRef.current !== fetchKey) {
      abortRequest(prevKeyRef.current);
    }
    prevKeyRef.current = fetchKey;

    let isCancelled = false;
    setLoading(true);
    setError(null);

    fetchFn()
      .then((result) => {
        if (!isCancelled) {
          setData(result);
          setLoading(false);
        }
      })
      .catch((err: unknown) => {
        if (!isCancelled) {
          setError(err instanceof Error ? err.message : 'Request failed');
          setLoading(false);
        }
      });

    return () => {
      isCancelled = true;
    };
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fetchKey, refetchToken, ...deps]);

  const refetch = useCallback(() => setRefetchToken((t) => t + 1), []);

  return { data, error, loading, refetch };
}
