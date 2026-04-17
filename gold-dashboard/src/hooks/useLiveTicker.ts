import { useDashboardStore } from "../store";

export function useLiveTicker(symbol: string): { price: number; timestamp: string } | null {
  return useDashboardStore((s) => s.tickersBySymbol[symbol] ?? null);
}
