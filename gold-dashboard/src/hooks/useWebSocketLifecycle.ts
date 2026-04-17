import { useEffect } from "react";
import { webSocketClient } from "../api";
import { useDashboardStore, candleKey, candleToChartCandle } from "../store";
import { TickBuffer, chartSeriesRegistry } from "../utils";
import type { WebSocketMessage } from "../types";

export function useWebSocketLifecycle(): void {
  const setConnectionState = useDashboardStore((s) => s.setConnectionState);
  const appendOrUpdateCandle = useDashboardStore((s) => s.appendOrUpdateCandle);
  const setLastPrice = useDashboardStore((s) => s.setLastPrice);
  const upsertOpenPosition = useDashboardStore((s) => s.upsertOpenPosition);
  const removeOpenPosition = useDashboardStore((s) => s.removeOpenPosition);
  const setMetrics = useDashboardStore((s) => s.setMetrics);
  const prependDecision = useDashboardStore((s) => s.prependDecision);

  useEffect(() => {
    const tickBuffer = new TickBuffer((candleUpdates, priceUpdates) => {
      for (const [key, candle] of candleUpdates) {
        // Always keep the store in sync for cache consistency
        appendOrUpdateCandle(key, candle);
        // Also imperatively update the visible series when key matches — bypasses React re-render
        chartSeriesRegistry.tryUpdate(key, candle);
      }
      for (const [symbol, { price, time }] of priceUpdates) {
        setLastPrice(symbol, price, time);
      }
    });

    const unsubscribeState = webSocketClient.onConnectionStateChange((state) => {
      setConnectionState(state);
    });

    const unsubscribeMessage = webSocketClient.subscribe((message: WebSocketMessage) => {
      switch (message.type) {
        case "candle_update": {
          const candle = message.payload;
          const key = candleKey(candle.symbol, candle.interval);
          const chartCandle = candleToChartCandle(candle);
          tickBuffer.pushCandle(key, chartCandle);
          tickBuffer.pushPrice(candle.symbol, parseFloat(candle.closePrice), chartCandle.time);
          break;
        }
        case "trade_executed":
        case "position_update": {
          const position = message.payload;
          upsertOpenPosition({
            ...position,
            currentPrice: position.entryPrice,
            unrealizedPnl: "0",
          });
          break;
        }
        case "position_closed": {
          const position = message.payload;
          removeOpenPosition(position.id);
          break;
        }
        case "metric_update": {
          setMetrics(message.payload);
          break;
        }
        case "decision_made": {
          prependDecision(message.payload);
          break;
        }
      }
    });

    webSocketClient.connect();

    return () => {
      unsubscribeMessage();
      unsubscribeState();
      tickBuffer.destroy();
      webSocketClient.disconnect();
    };
  }, [
    setConnectionState,
    appendOrUpdateCandle,
    setLastPrice,
    upsertOpenPosition,
    removeOpenPosition,
    setMetrics,
    prependDecision,
  ]);
}
