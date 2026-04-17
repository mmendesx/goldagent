import { useEffect } from "react";
import { webSocketClient } from "../api";
import { useDashboardStore, candleKey, candleToChartCandle } from "../store";
import type { WebSocketMessage } from "../types";

export function useWebSocketLifecycle(): void {
  const setConnectionState = useDashboardStore((s) => s.setConnectionState);
  const appendOrUpdateCandle = useDashboardStore((s) => s.appendOrUpdateCandle);
  const upsertOpenPosition = useDashboardStore((s) => s.upsertOpenPosition);
  const removeOpenPosition = useDashboardStore((s) => s.removeOpenPosition);
  const setMetrics = useDashboardStore((s) => s.setMetrics);
  const prependDecision = useDashboardStore((s) => s.prependDecision);
  const setTicker = useDashboardStore((s) => s.setTicker);

  useEffect(() => {
    const unsubscribeState = webSocketClient.onConnectionStateChange((state) => {
      setConnectionState(state);
    });

    const unsubscribeMessage = webSocketClient.subscribe((message: WebSocketMessage) => {
      switch (message.type) {
        case "candle_update": {
          const candle = message.payload;
          const key = candleKey(candle.symbol, candle.interval);
          appendOrUpdateCandle(key, candleToChartCandle(candle));
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
        case "ticker_update": {
          const { symbol, price, timestamp } = message.payload;
          setTicker(symbol, parseFloat(price), timestamp);
          break;
        }
      }
    });

    webSocketClient.connect();

    return () => {
      unsubscribeMessage();
      unsubscribeState();
      webSocketClient.disconnect();
    };
  }, [
    setConnectionState,
    appendOrUpdateCandle,
    upsertOpenPosition,
    removeOpenPosition,
    setMetrics,
    prependDecision,
    setTicker,
  ]);
}
