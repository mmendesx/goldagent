import type {
  Candle,
  Position,
  TradeRecord,
  Decision,
  PortfolioMetrics,
  PaginatedResponse,
  ExchangeBalances,
} from "../types";

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";

export class HttpError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "HttpError";
    this.status = status;
  }
}

export class TimeoutError extends Error {
  constructor(url: string) {
    super(`Request timed out: ${url}`);
    this.name = "TimeoutError";
  }
}

export class NetworkError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "NetworkError";
  }
}

const inFlight = new Map<
  string,
  { controller: AbortController; promise: Promise<unknown> }
>();

function requestKey(
  path: string,
  params?: Record<string, string | number>,
): string {
  if (!params) return path;
  const sorted = Object.keys(params)
    .sort()
    .map((k) => `${k}=${params[k]}`)
    .join("&");
  return `${path}?${sorted}`;
}

export function abortRequest(key: string): void {
  const entry = inFlight.get(key);
  if (entry) {
    entry.controller.abort();
    inFlight.delete(key);
  }
}

async function withRetry<T>(
  fn: () => Promise<T>,
  {
    retries = 2,
    baseDelayMs = 300,
    factor = 3,
  }: { retries?: number; baseDelayMs?: number; factor?: number } = {},
): Promise<T> {
  let lastError: unknown;
  for (let attempt = 0; attempt <= retries; attempt++) {
    try {
      return await fn();
    } catch (error) {
      lastError = error;
      // Do not retry on 4xx
      if (error instanceof HttpError && error.status < 500) throw error;
      if (attempt < retries) {
        await new Promise((resolve) =>
          setTimeout(resolve, baseDelayMs * factor ** attempt),
        );
      }
    }
  }
  throw lastError;
}

async function request<T>(
  path: string,
  params?: Record<string, string | number>,
  options?: { signal?: AbortSignal; timeoutMs?: number; dedupKey?: string },
): Promise<T> {
  const dedupKey = options?.dedupKey;

  if (dedupKey && inFlight.has(dedupKey)) {
    return inFlight.get(dedupKey)!.promise as Promise<T>;
  }

  const url = new URL(`${BASE_URL}${path}`);
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined) url.searchParams.set(key, String(value));
    }
  }

  const controller = new AbortController();

  // Forward any external signal — abort our controller when the external signal fires
  if (options?.signal) {
    if (options.signal.aborted) {
      controller.abort();
    } else {
      options.signal.addEventListener("abort", () => controller.abort(), {
        once: true,
      });
    }
  }

  const timeoutId = setTimeout(
    () => controller.abort(),
    options?.timeoutMs ?? 10_000,
  );

  const fetchPromise: Promise<T> = (async () => {
    let response: Response;
    try {
      response = await fetch(url.toString(), { signal: controller.signal });
    } catch (error) {
      clearTimeout(timeoutId);
      if (
        controller.signal.aborted ||
        (error instanceof DOMException && error.name === "AbortError")
      ) {
        throw new TimeoutError(url.toString());
      }
      throw new NetworkError(
        error instanceof Error ? error.message : String(error),
      );
    }

    clearTimeout(timeoutId);

    if (!response.ok) {
      throw new HttpError(response.status, await response.text());
    }
    return (await response.json()) as T;
  })().finally(() => {
    if (dedupKey) {
      inFlight.delete(dedupKey);
    }
  });

  if (dedupKey) {
    inFlight.set(dedupKey, { controller, promise: fetchPromise });
  }

  return fetchPromise;
}

export interface CandleQueryParams {
  symbol: string;
  interval: string;
  from?: string;
  to?: string;
  limit?: number;
  offset?: number;
}

export const restClient = {
  fetchCandles(
    params: CandleQueryParams,
  ): Promise<PaginatedResponse<Candle & { indicator?: unknown }>> {
    const queryParams: Record<string, string | number> = {
      symbol: params.symbol,
      interval: params.interval,
    };
    if (params.from !== undefined) queryParams.from = params.from;
    if (params.to !== undefined) queryParams.to = params.to;
    if (params.limit !== undefined) queryParams.limit = params.limit;
    if (params.offset !== undefined) queryParams.offset = params.offset;
    const key = requestKey("/api/v1/candles", queryParams);
    return withRetry(() =>
      request("/api/v1/candles", queryParams, { dedupKey: key }),
    );
  },

  fetchOpenPositions(): Promise<
    (Position & { currentPrice: string; unrealizedPnl: string })[]
  > {
    const key = requestKey("/api/v1/positions");
    return withRetry(() =>
      request("/api/v1/positions", undefined, { dedupKey: key }),
    );
  },

  fetchClosedPositions(
    limit = 100,
    offset = 0,
  ): Promise<PaginatedResponse<Position>> {
    const queryParams = { limit, offset };
    const key = requestKey("/api/v1/positions/history", queryParams);
    return withRetry(() =>
      request("/api/v1/positions/history", queryParams, { dedupKey: key }),
    );
  },

  fetchTrades(
    symbol?: string,
    limit = 100,
    offset = 0,
  ): Promise<PaginatedResponse<TradeRecord>> {
    const params: Record<string, string | number> = { limit, offset };
    if (symbol) params.symbol = symbol;
    const key = requestKey("/api/v1/trades", params);
    return withRetry(() =>
      request("/api/v1/trades", params, { dedupKey: key }),
    );
  },

  fetchMetrics(): Promise<PortfolioMetrics> {
    const key = requestKey("/api/v1/metrics");
    return withRetry(() =>
      request("/api/v1/metrics", undefined, { dedupKey: key }),
    );
  },

  fetchDecisions(
    symbol?: string,
    limit = 100,
    offset = 0,
  ): Promise<PaginatedResponse<Decision>> {
    const params: Record<string, string | number> = { limit, offset };
    if (symbol) params.symbol = symbol;
    const key = requestKey("/api/v1/decisions", params);
    return withRetry(() =>
      request("/api/v1/decisions", params, { dedupKey: key }),
    );
  },

  fetchExchangeBalances(): Promise<ExchangeBalances> {
    const key = requestKey("/api/v1/exchange/balances");
    return withRetry(() =>
      request("/api/v1/exchange/balances", undefined, { dedupKey: key }),
    );
  },
};
