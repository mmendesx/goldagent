import { describe, it, expect } from "vitest";
import { candleToChartCandle } from "./index";
import type { Candle } from "../types";

describe("candleToChartCandle", () => {
  const sampleCandle: Candle = {
    symbol: "BTCUSDT",
    interval: "5m",
    openTime: "2026-01-15T12:00:00Z",
    closeTime: "2026-01-15T12:05:00Z",
    openPrice: "42000.00",
    highPrice: "42500.50",
    lowPrice: "41900.25",
    closePrice: "42300.75",
    volume: "1234.567",
    quoteVolume: "52100000.00",
    tradeCount: 500,
    isClosed: true,
  };

  it("converts openTime RFC3339 string to unix seconds", () => {
    const chart = candleToChartCandle(sampleCandle);
    const expectedTime = Math.floor(new Date("2026-01-15T12:00:00Z").getTime() / 1000);
    expect(chart.time).toBe(expectedTime);
  });

  it("parses decimal string prices to numbers", () => {
    const chart = candleToChartCandle(sampleCandle);
    expect(chart.open).toBe(42000.0);
    expect(chart.high).toBe(42500.5);
    expect(chart.low).toBe(41900.25);
    expect(chart.close).toBe(42300.75);
  });

  it("parses volume to a number", () => {
    const chart = candleToChartCandle(sampleCandle);
    expect(chart.volume).toBeCloseTo(1234.567);
  });
});
