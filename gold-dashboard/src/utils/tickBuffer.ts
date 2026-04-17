import type { ChartCandle } from '../types';

type TickFlushCallback = (
  candleUpdates: Map<string, ChartCandle>,
  priceUpdates: Map<string, { price: number; time: number }>
) => void;

export class TickBuffer {
  private pendingCandles = new Map<string, ChartCandle>();
  private pendingPrices = new Map<string, { price: number; time: number }>();
  private rafHandle: number | null = null;
  private callback: TickFlushCallback;

  constructor(callback: TickFlushCallback) {
    this.callback = callback;
  }

  pushCandle(key: string, candle: ChartCandle): void {
    this.pendingCandles.set(key, candle); // latest wins
    this.scheduleFlush();
  }

  pushPrice(symbol: string, price: number, time: number): void {
    this.pendingPrices.set(symbol, { price, time });
    this.scheduleFlush();
  }

  private scheduleFlush(): void {
    if (this.rafHandle !== null) return; // already scheduled
    this.rafHandle = requestAnimationFrame(() => {
      this.rafHandle = null;
      this.flush();
    });
  }

  private flush(): void {
    if (this.pendingCandles.size === 0 && this.pendingPrices.size === 0) return;
    const candles = new Map(this.pendingCandles);
    const prices = new Map(this.pendingPrices);
    this.pendingCandles.clear();
    this.pendingPrices.clear();
    this.callback(candles, prices);
  }

  destroy(): void {
    if (this.rafHandle !== null) {
      cancelAnimationFrame(this.rafHandle);
      this.rafHandle = null;
    }
    this.pendingCandles.clear();
    this.pendingPrices.clear();
  }
}
