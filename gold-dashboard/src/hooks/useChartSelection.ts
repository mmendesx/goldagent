import { useDashboardStore } from "../store";
import type { TradingSymbol, ChartInterval } from "../types";

export type Exchange = "binance" | "polymarket";

export function useChartSelection(exchange: Exchange) {
  const selection = useDashboardStore((s) => s.chartSelection[exchange]);
  const setChartSelection = useDashboardStore((s) => s.setChartSelection);

  const setSymbol = (symbol: TradingSymbol) => setChartSelection(exchange, { symbol });
  const setInterval = (interval: ChartInterval) => setChartSelection(exchange, { interval });

  return { symbol: selection.symbol, interval: selection.interval, setSymbol, setInterval };
}
