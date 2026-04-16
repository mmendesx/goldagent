import { create } from "zustand";
import type {
  Candle,
  ChartCandle,
  Position,
  PortfolioMetrics,
  TradeRecord,
  Decision,
  TradingSymbol,
  ChartInterval,
} from "../types";

interface OpenPositionWithLive extends Position {
  currentPrice: string;
  unrealizedPnl: string;
}

interface DashboardState {
  // Connection state
  connectionState: "connecting" | "open" | "closed" | "reconnecting";
  setConnectionState: (state: DashboardState["connectionState"]) => void;

  // Selected symbol/interval (chart context)
  selectedSymbol: TradingSymbol;
  selectedInterval: ChartInterval;
  setSelectedSymbol: (symbol: TradingSymbol) => void;
  setSelectedInterval: (interval: ChartInterval) => void;

  // Candle data — keyed by `${symbol}:${interval}` so multiple symbols can be cached
  candlesByKey: Record<string, ChartCandle[]>;
  setCandlesForKey: (key: string, candles: ChartCandle[]) => void;
  appendOrUpdateCandle: (key: string, candle: ChartCandle) => void;

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

export const useDashboardStore = create<DashboardState>((set) => ({
  connectionState: "closed",
  setConnectionState: (state) => set({ connectionState: state }),

  selectedSymbol: "BTCUSDT",
  selectedInterval: "5m",
  setSelectedSymbol: (symbol) => set({ selectedSymbol: symbol }),
  setSelectedInterval: (interval) => set({ selectedInterval: interval }),

  candlesByKey: {},
  setCandlesForKey: (key, candles) =>
    set((state) => ({ candlesByKey: { ...state.candlesByKey, [key]: candles } })),
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
}));
