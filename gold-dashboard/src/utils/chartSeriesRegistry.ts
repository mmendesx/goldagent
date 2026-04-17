import type { ISeriesApi, Time } from 'lightweight-charts';

let activeSeries: ISeriesApi<'Candlestick'> | null = null;
let activeKey: string | null = null;

export const chartSeriesRegistry = {
  register(key: string, series: ISeriesApi<'Candlestick'>): void {
    activeSeries = series;
    activeKey = key;
  },
  unregister(): void {
    activeSeries = null;
    activeKey = null;
  },
  tryUpdate(key: string, candle: { time: number; open: number; high: number; low: number; close: number }): boolean {
    if (activeKey !== key || activeSeries === null) return false;
    try {
      activeSeries.update({ ...candle, time: candle.time as Time });
      return true;
    } catch {
      return false;
    }
  },
};
