import { create } from "zustand";
import { persist } from "zustand/middleware";
import type {
  Candle,
  ChartCandle,
  Position,
  PortfolioMetrics,
  TradeRecord,
  Decision,
  TradingSymbol,
  ChartInterval,
  ExchangeBalances,
} from "../types";

export interface ChartIndicatorSettings {
  ma: { enabled: boolean; periods: [number, number] };
  vwap: { enabled: boolean };
  volume: { enabled: boolean };
}

export const DEFAULT_INDICATOR_SETTINGS: ChartIndicatorSettings = {
  ma: { enabled: true, periods: [20, 50] },
  vwap: { enabled: false },
  volume: { enabled: true },
};

interface OpenPositionWithLive extends Position {
  currentPrice: string;
  unrealizedPnl: string;
}

interface ChartSelectionEntry {
  symbol: TradingSymbol;
  interval: ChartInterval;
}

export interface DashboardState {
  // Connection state
  connectionState: "connecting" | "open" | "closed" | "reconnecting";
  setConnectionState: (state: DashboardState["connectionState"]) => void;
  reconnectAttempts: number;
  setReconnectAttempts: (attempts: number) => void;

  // Per-exchange chart selection
  chartSelection: {
    binance: ChartSelectionEntry;
    polymarket: ChartSelectionEntry;
  };
  setChartSelection: (
    exchange: "binance" | "polymarket",
    partial: Partial<ChartSelectionEntry>
  ) => void;

  // Last price per symbol
  lastPrice: Record<string, { price: number; time: number }>;
  setLastPrice: (symbol: string, price: number, time: number) => void;

  // Candle data — keyed by `${symbol}:${interval}` so multiple symbols can be cached
  candlesByKey: Record<string, ChartCandle[]>;
  setCandlesForKey: (key: string, candles: ChartCandle[]) => void;
  appendOrUpdateCandle: (key: string, candle: ChartCandle) => void;
  candleLoading: Record<string, boolean>;
  setCandleLoading: (key: string, loading: boolean) => void;
  clearCandles: (key: string) => void;

  // Open positions (with live data)
  openPositions: OpenPositionWithLive[];
  setOpenPositions: (positions: OpenPositionWithLive[]) => void;
  upsertOpenPosition: (position: OpenPositionWithLive) => void;
  removeOpenPosition: (positionId: number) => void;

  // Closed positions (history)
  closedPositions: Position[];
  setClosedPositions: (positions: Position[]) => void;

  // Trade records
  trades: TradeRecord[];
  setTrades: (trades: TradeRecord[]) => void;

  // Decisions
  decisions: Decision[];
  setDecisions: (decisions: Decision[]) => void;
  prependDecision: (decision: Decision) => void;

  // Portfolio metrics (live)
  metrics: PortfolioMetrics | null;
  setMetrics: (metrics: PortfolioMetrics) => void;

  // Exchange balances (polled)
  exchangeBalances: ExchangeBalances | null;
  setExchangeBalances: (balances: ExchangeBalances) => void;

  // Chart indicator settings — persisted per chart key
  chartIndicators: Record<string, ChartIndicatorSettings>;
  setChartIndicators: (key: string, settings: Partial<ChartIndicatorSettings>) => void;

  // Theme
  theme: "light" | "dark" | "system";
  setTheme: (t: "light" | "dark" | "system") => void;
}

export const candleKey = (symbol: string, interval: string): string => `${symbol}:${interval}`;

export function candleToChartCandle(candle: Candle): ChartCandle {
  return {
    time: Math.floor(new Date(candle.openTime).getTime() / 1000),
    open: parseFloat(candle.openPrice),
    high: parseFloat(candle.highPrice),
    low: parseFloat(candle.lowPrice),
    close: parseFloat(candle.closePrice),
    volume: parseFloat(candle.volume),
  };
}

export const useDashboardStore = create<DashboardState>()(
  persist(
    (set) => ({
      connectionState: "closed",
      setConnectionState: (state) => set({ connectionState: state }),
      reconnectAttempts: 0,
      setReconnectAttempts: (attempts) => set({ reconnectAttempts: attempts }),

      chartSelection: {
        binance: { symbol: "BTCUSDT", interval: "5m" },
        polymarket: { symbol: "BTCUSDT", interval: "5m" },
      },
      setChartSelection: (exchange, partial) =>
        set((state) => ({
          chartSelection: {
            ...state.chartSelection,
            [exchange]: { ...state.chartSelection[exchange], ...partial },
          },
        })),

      lastPrice: {},
      setLastPrice: (symbol, price, time) =>
        set((state) => ({
          lastPrice: { ...state.lastPrice, [symbol]: { price, time } },
        })),

      candlesByKey: {},
      setCandlesForKey: (key, candles) =>
        set((state) => ({ candlesByKey: { ...state.candlesByKey, [key]: candles } })),
      candleLoading: {},
      setCandleLoading: (key, loading) =>
        set((state) => ({ candleLoading: { ...state.candleLoading, [key]: loading } })),
      clearCandles: (key) =>
        set((state) => {
          const next = { ...state.candlesByKey };
          delete next[key];
          return { candlesByKey: next };
        }),
      appendOrUpdateCandle: (key, candle) =>
        set((state) => {
          const existing = state.candlesByKey[key] ?? [];
          const lastIndex = existing.length - 1;
          const last = existing[lastIndex];
          let next: ChartCandle[];
          if (last && last.time === candle.time) {
            next = [...existing.slice(0, lastIndex), candle];
          } else if (last && candle.time < last.time) {
            return state;
          } else {
            next = [...existing, candle];
          }
          return { candlesByKey: { ...state.candlesByKey, [key]: next } };
        }),

      openPositions: [],
      setOpenPositions: (positions) => set({ openPositions: positions }),
      upsertOpenPosition: (position) =>
        set((state) => {
          const existingIndex = state.openPositions.findIndex((p) => p.id === position.id);
          if (existingIndex === -1) return { openPositions: [...state.openPositions, position] };
          const next = [...state.openPositions];
          next[existingIndex] = position;
          return { openPositions: next };
        }),
      removeOpenPosition: (positionId) =>
        set((state) => ({
          openPositions: state.openPositions.filter((p) => p.id !== positionId),
        })),

      closedPositions: [],
      setClosedPositions: (positions) => set({ closedPositions: positions }),

      trades: [],
      setTrades: (trades) => set({ trades }),

      decisions: [],
      setDecisions: (decisions) => set({ decisions }),
      prependDecision: (decision) =>
        set((state) => ({ decisions: [decision, ...state.decisions].slice(0, 500) })),

      metrics: null,
      setMetrics: (metrics) => set({ metrics }),

      exchangeBalances: null,
      setExchangeBalances: (balances) => set({ exchangeBalances: balances }),

      chartIndicators: {},
      setChartIndicators: (key, settings) =>
        set((state) => ({
          chartIndicators: {
            ...state.chartIndicators,
            [key]: { ...(state.chartIndicators[key] ?? DEFAULT_INDICATOR_SETTINGS), ...settings },
          },
        })),

      theme: "dark",
      setTheme: (t) => set({ theme: t }),
    }),
    {
      name: "gold-dashboard-indicators",
      partialize: (state) => ({ chartIndicators: state.chartIndicators, theme: state.theme }),
    }
  )
);

export function selectChartIndicators(key: string) {
  return (state: DashboardState): ChartIndicatorSettings =>
    state.chartIndicators[key] ?? DEFAULT_INDICATOR_SETTINGS;
}

export function computeOpenPositionsWithLivePnl(
  openPositions: OpenPositionWithLive[],
  lastPriceMap: Record<string, { price: number; time: number }>
): OpenPositionWithLive[] {
  return openPositions.map((position) => {
    const lastTick = lastPriceMap[position.symbol];
    if (!lastTick) return position;

    const lastPrice = lastTick.price;
    const entryPrice = parseFloat(position.entryPrice);
    const quantity = parseFloat(position.quantity);
    const sideMultiplier = position.side === "LONG" ? 1 : -1;
    const pnl = (lastPrice - entryPrice) * quantity * sideMultiplier;

    return {
      ...position,
      currentPrice: String(lastPrice),
      unrealizedPnl: String(pnl.toFixed(4)),
    };
  });
}

/** @deprecated use computeOpenPositionsWithLivePnl with useMemo to avoid unstable snapshots */
export function selectOpenPositionsWithLivePnl(state: DashboardState) {
  return computeOpenPositionsWithLivePnl(state.openPositions, state.lastPrice);
}
