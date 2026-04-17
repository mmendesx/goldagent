import { useDashboardStore } from "../store";

export function useLiveTicker(symbol: string): { price: number; timestamp: string } | null {
  return useDashboardStore((s) => {
    const tick = s.lastPrice[symbol];
    if (!tick) return null;
    return { price: tick.price, timestamp: new Date(tick.time * 1000).toISOString() };
  });
}
