import { describe, it, expect, beforeEach } from "vitest";
import { candleToChartCandle, useDashboardStore, selectOpenPositionsWithLivePnl } from "./index";
import type { Candle } from "../types";

const initialState = useDashboardStore.getState();

beforeEach(() => {
  useDashboardStore.setState(initialState, true);
});

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

describe("chartSelection", () => {
  it("binance and polymarket selections are independent", () => {
    const store = useDashboardStore.getState();
    store.setChartSelection("binance", { symbol: "ETHUSDT" });
    const state = useDashboardStore.getState();
    expect(state.chartSelection.binance.symbol).toBe("ETHUSDT");
    expect(state.chartSelection.polymarket.symbol).toBe("BTCUSDT"); // unchanged
  });

  it("setChartSelection partial update preserves other fields", () => {
    const store = useDashboardStore.getState();
    store.setChartSelection("binance", { interval: "1h" });
    const state = useDashboardStore.getState();
    expect(state.chartSelection.binance.interval).toBe("1h");
    expect(state.chartSelection.binance.symbol).toBe("BTCUSDT"); // default symbol preserved
  });
});

describe("selectOpenPositionsWithLivePnl", () => {
  const basePosition = {
    id: 1,
    symbol: "BTCUSDT",
    side: "LONG" as const,
    entryPrice: "40000",
    quantity: "0.5",
    takeProfitPrice: "45000",
    stopLossPrice: "38000",
    status: "open" as const,
    openedAt: "2026-01-01T00:00:00Z",
    currentPrice: "40000",
    unrealizedPnl: "0",
  };

  it("returns position unchanged when no tick for symbol", () => {
    useDashboardStore.setState({ openPositions: [basePosition], lastPrice: {} });
    const state = useDashboardStore.getState();
    const result = selectOpenPositionsWithLivePnl(state);
    expect(result[0]).toEqual(basePosition);
    expect(result[0].unrealizedPnl).toBe("0");
  });

  it("computes LONG PnL: (lastPrice - entry) * qty * +1", () => {
    useDashboardStore.setState({
      openPositions: [{ ...basePosition, side: "LONG" }],
      lastPrice: { BTCUSDT: { price: 41000, time: 1000 } },
    });
    const state = useDashboardStore.getState();
    const result = selectOpenPositionsWithLivePnl(state);
    // (41000 - 40000) * 0.5 * 1 = 500
    expect(result[0].unrealizedPnl).toBe("500.0000");
    expect(result[0].currentPrice).toBe("41000");
  });

  it("computes SHORT PnL: (lastPrice - entry) * qty * -1", () => {
    useDashboardStore.setState({
      openPositions: [{ ...basePosition, side: "SHORT" }],
      lastPrice: { BTCUSDT: { price: 41000, time: 1000 } },
    });
    const state = useDashboardStore.getState();
    const result = selectOpenPositionsWithLivePnl(state);
    // (41000 - 40000) * 0.5 * -1 = -500
    expect(result[0].unrealizedPnl).toBe("-500.0000");
    expect(result[0].currentPrice).toBe("41000");
  });
});
