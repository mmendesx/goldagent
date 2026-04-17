import type {
  Candle,
  Position,
  TradeRecord,
  Decision,
  PortfolioMetrics,
  Paginated,
  ExchangeBalances,
} from "../types";

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";

class HttpError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(path: string, params?: Record<string, string | number>): Promise<T> {
  const url = new URL(`${BASE_URL}${path}`);
  if (params) {
    for (const [key, value] of Object.entries(params)) {
      if (value !== undefined) url.searchParams.set(key, String(value));
    }
  }
  const response = await fetch(url.toString());
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
  async fetchCandles(
    params: CandleQueryParams,
  ): Promise<Paginated<Candle & { indicator?: unknown }>> {
    const queryParams: Record<string, string | number> = {
      symbol: params.symbol,
      interval: params.interval,
    };
    if (params.from !== undefined) queryParams.from = params.from;
    if (params.to !== undefined) queryParams.to = params.to;
    if (params.limit !== undefined) queryParams.limit = params.limit;
    if (params.offset !== undefined) queryParams.offset = params.offset;
    return request("/api/v1/candles", queryParams);
  },

  async fetchOpenPositions(): Promise<Position[]> {
    return request("/api/v1/positions");
  },

  async fetchClosedPositions(limit = 100, offset = 0): Promise<Paginated<Position>> {
    return request("/api/v1/positions/history", { limit, offset });
  },

  async fetchPositionsHistory(params?: {
    symbol?: string;
    limit?: number;
    offset?: number;
  }): Promise<Paginated<Position>> {
    const queryParams: Record<string, string | number> = {
      limit: params?.limit ?? 100,
      offset: params?.offset ?? 0,
    };
    if (params?.symbol !== undefined) queryParams.symbol = params.symbol;
    return request("/api/v1/positions/history", queryParams);
  },

  async fetchTrades(
    symbol?: string,
    limit = 100,
    offset = 0,
  ): Promise<Paginated<TradeRecord>> {
    const params: Record<string, string | number> = { limit, offset };
    if (symbol) params.symbol = symbol;
    return request("/api/v1/trades", params);
  },

  async fetchMetrics(): Promise<PortfolioMetrics> {
    return request("/api/v1/metrics");
  },

  async fetchDecisions(
    symbol?: string,
    limit = 100,
    offset = 0,
  ): Promise<Paginated<Decision>> {
    const params: Record<string, string | number> = { limit, offset };
    if (symbol) params.symbol = symbol;
    return request("/api/v1/decisions", params);
  },

  async fetchExchangeBalances(): Promise<ExchangeBalances> {
    return request("/api/v1/exchange/balances");
  },
};

export { HttpError };
