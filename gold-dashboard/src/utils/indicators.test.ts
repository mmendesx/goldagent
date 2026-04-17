import { describe, it, expect } from 'vitest';
import { sma, vwap } from './indicators';
import type { ChartCandle } from '../types';

function makeCandle(time: number, close: number, volume = 1, high?: number, low?: number): ChartCandle {
  return {
    time,
    open: close,
    high: high ?? close,
    low: low ?? close,
    close,
    volume,
  };
}

describe('sma', () => {
  it('computes period-3 SMA for 5 candles', () => {
    const candles = [
      makeCandle(1, 10),
      makeCandle(2, 20),
      makeCandle(3, 30),
      makeCandle(4, 40),
      makeCandle(5, 50),
    ];
    const result = sma(candles, 3);
    expect(result).toHaveLength(3);
    expect(result[0]).toEqual({ time: 3, value: 20 }); // (10+20+30)/3
    expect(result[1]).toEqual({ time: 4, value: 30 }); // (20+30+40)/3
    expect(result[2]).toEqual({ time: 5, value: 40 }); // (30+40+50)/3
  });

  it('returns empty array when candle count is less than period', () => {
    const candles = [makeCandle(1, 10), makeCandle(2, 20)];
    expect(sma(candles, 3)).toEqual([]);
  });

  it('returns empty array when candle count equals zero', () => {
    expect(sma([], 3)).toEqual([]);
  });

  it('period 1 returns each candle close as its own average', () => {
    const candles = [makeCandle(1, 15), makeCandle(2, 25), makeCandle(3, 35)];
    const result = sma(candles, 1);
    expect(result).toHaveLength(3);
    expect(result[0]).toEqual({ time: 1, value: 15 });
    expect(result[1]).toEqual({ time: 2, value: 25 });
    expect(result[2]).toEqual({ time: 3, value: 35 });
  });
});

describe('vwap', () => {
  it('returns empty array for empty candles', () => {
    expect(vwap([])).toEqual([]);
  });

  it('accumulates running VWAP across all candles without session reset', () => {
    // Candle 1: typical = (10+8+9)/3 = 9, vol = 2 → cumTPV=18, cumVol=2 → vwap=9
    // Candle 2: typical = (12+10+11)/3 = 11, vol = 3 → cumTPV=18+33=51, cumVol=5 → vwap=10.2
    const candles: ChartCandle[] = [
      { time: 100, open: 9, high: 10, low: 8, close: 9, volume: 2 },
      { time: 200, open: 11, high: 12, low: 10, close: 11, volume: 3 },
    ];
    const result = vwap(candles);
    expect(result).toHaveLength(2);
    expect(result[0].time).toBe(100);
    expect(result[0].value).toBeCloseTo(9, 5);
    expect(result[1].time).toBe(200);
    expect(result[1].value).toBeCloseTo(10.2, 5);
  });

  it('resets accumulator at UTC day boundary when session is day', () => {
    // Day 0 ends at time 86399, day 1 starts at 86400
    // Day 0 candle: typical = (10+8+9)/3 = 9, vol = 2 → vwap = 9
    // Day 1 candle: typical = (20+18+19)/3 = 19, vol = 4 → vwap resets to 19
    const candles: ChartCandle[] = [
      { time: 86399, open: 9, high: 10, low: 8, close: 9, volume: 2 },
      { time: 86400, open: 19, high: 20, low: 18, close: 19, volume: 4 },
    ];
    const result = vwap(candles, 'day');
    expect(result).toHaveLength(2);
    expect(result[0].value).toBeCloseTo(9, 5);
    expect(result[1].value).toBeCloseTo(19, 5);
  });

  it('does not reset mid-day when session is day and candles share the same UTC day', () => {
    // Both candles on day 1 (times 86400 and 86500)
    // Candle 1: typical = 9, vol = 2 → cumTPV=18, cumVol=2 → vwap=9
    // Candle 2: typical = 19, vol = 4 → cumTPV=18+76=94, cumVol=6 → vwap≈15.667
    const candles: ChartCandle[] = [
      { time: 86400, open: 9, high: 10, low: 8, close: 9, volume: 2 },
      { time: 86500, open: 19, high: 20, low: 18, close: 19, volume: 4 },
    ];
    const result = vwap(candles, 'day');
    expect(result).toHaveLength(2);
    expect(result[0].value).toBeCloseTo(9, 5);
    expect(result[1].value).toBeCloseTo(94 / 6, 5);
  });
});
