import { describe, it, expect, vi, afterEach } from "vitest";
import { TickBuffer } from "./tickBuffer";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("TickBuffer", () => {
  it("batches multiple pushCandle calls into one flush callback", () => {
    let rafCallback: FrameRequestCallback | null = null;
    vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => {
      rafCallback = cb;
      return 1;
    });
    vi.stubGlobal("cancelAnimationFrame", vi.fn());

    const flushSpy = vi.fn();
    const buffer = new TickBuffer(flushSpy);

    const candle1 = { time: 1000, open: 1, high: 2, low: 0.5, close: 1.5, volume: 10 };
    const candle2 = { time: 1000, open: 1, high: 3, low: 0.5, close: 2.5, volume: 20 }; // same key, newer

    buffer.pushCandle("BTCUSDT:1m", candle1);
    buffer.pushCandle("BTCUSDT:1m", candle2); // overwrites candle1

    expect(flushSpy).not.toHaveBeenCalled(); // not yet — RAF hasn't fired

    rafCallback!(performance.now()); // simulate RAF fire

    expect(flushSpy).toHaveBeenCalledOnce();
    const [candles] = flushSpy.mock.calls[0];
    expect(candles.get("BTCUSDT:1m")).toEqual(candle2); // latest wins
  });

  it("schedules RAF only once for multiple pushCandle calls", () => {
    const rafSpy = vi.fn((cb: FrameRequestCallback) => {
      void cb; // capture but don't call
      return 1;
    });
    vi.stubGlobal("requestAnimationFrame", rafSpy);
    vi.stubGlobal("cancelAnimationFrame", vi.fn());

    const buffer = new TickBuffer(vi.fn());
    buffer.pushCandle("X:1m", { time: 1, open: 1, high: 1, low: 1, close: 1, volume: 0 });
    buffer.pushCandle("Y:1m", { time: 2, open: 2, high: 2, low: 2, close: 2, volume: 0 });

    // RAF should only be requested once despite two pushes
    expect(rafSpy).toHaveBeenCalledOnce();
  });

  it("destroy cancels pending RAF", () => {
    const cancelSpy = vi.fn();
    vi.stubGlobal("requestAnimationFrame", () => 42);
    vi.stubGlobal("cancelAnimationFrame", cancelSpy);

    const buffer = new TickBuffer(vi.fn());
    buffer.pushCandle("X:1m", { time: 1, open: 1, high: 1, low: 1, close: 1, volume: 0 });
    buffer.destroy();

    expect(cancelSpy).toHaveBeenCalledWith(42);
  });

  it("pushPrice is batched and flushed alongside candles", () => {
    let rafCallback: FrameRequestCallback | null = null;
    vi.stubGlobal("requestAnimationFrame", (cb: FrameRequestCallback) => {
      rafCallback = cb;
      return 1;
    });
    vi.stubGlobal("cancelAnimationFrame", vi.fn());

    const flushSpy = vi.fn();
    const buffer = new TickBuffer(flushSpy);

    buffer.pushPrice("BTCUSDT", 42000, 1000);

    rafCallback!(performance.now());

    expect(flushSpy).toHaveBeenCalledOnce();
    const [_candles, prices] = flushSpy.mock.calls[0];
    expect(prices.get("BTCUSDT")).toEqual({ price: 42000, time: 1000 });
  });
});
