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
  options?: { signal?: AbortSignal; timeoutMs?: number },
): Promise<T> {
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
    return withRetry(() => request("/api/v1/candles", queryParams));
  },

  fetchOpenPositions(): Promise<
    (Position & { currentPrice: string; unrealizedPnl: string })[]
  > {
    return withRetry(() => request("/api/v1/positions"));
  },

  fetchClosedPositions(
    limit = 100,
    offset = 0,
  ): Promise<PaginatedResponse<Position>> {
    return withRetry(() =>
      request("/api/v1/positions/history", { limit, offset }),
    );
  },

  fetchTrades(
    symbol?: string,
    limit = 100,
    offset = 0,
  ): Promise<PaginatedResponse<TradeRecord>> {
    const params: Record<string, string | number> = { limit, offset };
    if (symbol) params.symbol = symbol;
    return withRetry(() => request("/api/v1/trades", params));
  },

  fetchMetrics(): Promise<PortfolioMetrics> {
    return withRetry(() => request("/api/v1/metrics"));
  },

  fetchDecisions(
    symbol?: string,
    limit = 100,
    offset = 0,
  ): Promise<PaginatedResponse<Decision>> {
    const params: Record<string, string | number> = { limit, offset };
    if (symbol) params.symbol = symbol;
    return withRetry(() => request("/api/v1/decisions", params));
  },

  fetchExchangeBalances(): Promise<ExchangeBalances> {
    return withRetry(() => request("/api/v1/exchange/balances"));
  },
};
