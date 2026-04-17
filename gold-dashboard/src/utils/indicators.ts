import type { ChartCandle } from '../types';

export interface LinePoint {
  time: number;
  value: number;
}

/**
 * Simple Moving Average over `period` candles.
 * Returns one point per candle starting from index `period - 1`.
 */
export function sma(candles: ChartCandle[], period: number): LinePoint[] {
  if (period < 1 || candles.length < period) return [];
  const result: LinePoint[] = [];
  for (let i = period - 1; i < candles.length; i++) {
    let sum = 0;
    for (let j = i - period + 1; j <= i; j++) sum += candles[j].close;
    result.push({ time: candles[i].time, value: sum / period });
  }
  return result;
}

/**
 * VWAP computed from typical price (H+L+C)/3 weighted by volume.
 * When session === 'day', resets at UTC midnight (candle.time is Unix seconds).
 */
export function vwap(candles: ChartCandle[], session?: 'day'): LinePoint[] {
  if (candles.length === 0) return [];
  const result: LinePoint[] = [];
  let cumTypicalPriceVolume = 0;
  let cumVolume = 0;
  let sessionDay = -1;

  for (const candle of candles) {
    if (session === 'day') {
      const dayUtc = Math.floor(candle.time / 86400); // seconds → UTC day number
      if (dayUtc !== sessionDay) {
        cumTypicalPriceVolume = 0;
        cumVolume = 0;
        sessionDay = dayUtc;
      }
    }
    const typicalPrice = (candle.high + candle.low + candle.close) / 3;
    cumTypicalPriceVolume += typicalPrice * candle.volume;
    cumVolume += candle.volume;
    if (cumVolume > 0) {
      result.push({ time: candle.time, value: cumTypicalPriceVolume / cumVolume });
    }
  }
  return result;
}
