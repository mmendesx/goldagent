import { useEffect, useMemo, useRef, useState } from "react";
import {
  createChart,
  createSeriesMarkers,
  CandlestickSeries,
  HistogramSeries,
  LineSeries,
  type IChartApi,
  type ISeriesApi,
  type ISeriesMarkersPluginApi,
  type SeriesMarker,
  type Time,
  ColorType,
} from "lightweight-charts";
import { useDashboardStore, candleKey, candleToChartCandle, selectChartIndicators } from "../../store";
import { chartSeriesRegistry, sma, vwap } from "../../utils";
import { useChartSelection } from "../../hooks/useChartSelection";
import type { Exchange } from "../../hooks/useChartSelection";
import { restClient } from "../../api";
import { SymbolSelector } from "../SymbolSelector/SymbolSelector";
import { IntervalButtons } from "../IntervalButtons/IntervalButtons";
import { ChartSettings } from "../ChartSettings";
import type { ChartCandle } from "../../types";
import "./PriceChart.css";

const EMPTY_CANDLES: ChartCandle[] = [];

// Dark theme colors matching the dashboard design system
const CHART_THEME = {
  background: "#12121a",
  textColor: "#e8e8f0",
  gridColor: "#2a2a3a",
  upColor: "#22c55e",
  downColor: "#ef4444",
  volumeUpColor: "rgba(34, 197, 94, 0.3)",
  volumeDownColor: "rgba(239, 68, 68, 0.3)",
  borderColor: "#2a2a3a",
};

interface PriceChartProps {
  exchange: Exchange;
}

export function PriceChart({ exchange }: PriceChartProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candlestickSeriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const volumeSeriesRef = useRef<ISeriesApi<"Histogram"> | null>(null);
  const ma1SeriesRef = useRef<ISeriesApi<"Line"> | null>(null);
  const ma2SeriesRef = useRef<ISeriesApi<"Line"> | null>(null);
  const vwapSeriesRef = useRef<ISeriesApi<"Line"> | null>(null);
  const markersApiRef = useRef<ISeriesMarkersPluginApi<Time> | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const { symbol, interval, setSymbol, setInterval } = useChartSelection(exchange);
  const setCandlesForKey = useDashboardStore((state) => state.setCandlesForKey);
  const setCandleLoading = useDashboardStore((state) => state.setCandleLoading);
  const clearCandles = useDashboardStore((state) => state.clearCandles);
  const openPositions = useDashboardStore((state) => state.openPositions);
  const closedPositions = useDashboardStore((state) => state.closedPositions);

  const activeKey = candleKey(symbol, interval);
  const settingsKey = `${exchange}|${symbol}|${interval}`;
  const settings = useDashboardStore(selectChartIndicators(settingsKey));
  const candles = useDashboardStore((state) => state.candlesByKey[activeKey]) ?? EMPTY_CANDLES;
  const candleLoading = useDashboardStore((s) => s.candleLoading);
  const isKeyLoading = candleLoading[activeKey] ?? false;

  // Initialize chart once — empty deps so this runs only on mount
  useEffect(() => {
    if (!containerRef.current) return;

    const chart = createChart(containerRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: CHART_THEME.background },
        textColor: CHART_THEME.textColor,
      },
      grid: {
        vertLines: { color: CHART_THEME.gridColor },
        horzLines: { color: CHART_THEME.gridColor },
      },
      rightPriceScale: {
        borderColor: CHART_THEME.borderColor,
      },
      timeScale: {
        borderColor: CHART_THEME.borderColor,
        timeVisible: true,
        secondsVisible: false,
      },
      width: containerRef.current.clientWidth,
      height: containerRef.current.clientHeight,
      autoSize: true,
    });

    const candlestickSeries = chart.addSeries(CandlestickSeries, {
      upColor: CHART_THEME.upColor,
      downColor: CHART_THEME.downColor,
      borderUpColor: CHART_THEME.upColor,
      borderDownColor: CHART_THEME.downColor,
      wickUpColor: CHART_THEME.upColor,
      wickDownColor: CHART_THEME.downColor,
    });

    const volumeSeries = chart.addSeries(HistogramSeries, {
      color: CHART_THEME.volumeUpColor,
      priceFormat: { type: "volume" },
      priceScaleId: "volume",
    });

    chart.priceScale("volume").applyOptions({
      scaleMargins: { top: 0.8, bottom: 0 },
    });

    const ma1Series = chart.addSeries(LineSeries, {
      color: '#f59e0b',
      lineWidth: 1,
      priceScaleId: 'right',
      lastValueVisible: false,
      priceLineVisible: false,
    });

    const ma2Series = chart.addSeries(LineSeries, {
      color: '#8b5cf6',
      lineWidth: 1,
      priceScaleId: 'right',
      lastValueVisible: false,
      priceLineVisible: false,
    });

    const vwapSeries = chart.addSeries(LineSeries, {
      color: '#06b6d4',
      lineWidth: 1,
      priceScaleId: 'right',
      lastValueVisible: false,
      priceLineVisible: false,
    });

    // v5 markers API — must be created via createSeriesMarkers factory
    const markersApi = createSeriesMarkers(candlestickSeries, []);

    chartRef.current = chart;
    candlestickSeriesRef.current = candlestickSeries;
    volumeSeriesRef.current = volumeSeries;
    ma1SeriesRef.current = ma1Series;
    ma2SeriesRef.current = ma2Series;
    vwapSeriesRef.current = vwapSeries;
    markersApiRef.current = markersApi;

    return () => {
      chartSeriesRegistry.unregister();
      markersApi.detach();
      chart.remove();
      chartRef.current = null;
      candlestickSeriesRef.current = null;
      volumeSeriesRef.current = null;
      ma1SeriesRef.current = null;
      ma2SeriesRef.current = null;
      vwapSeriesRef.current = null;
      markersApiRef.current = null;
    };
  }, []);

  // Keep registry in sync when the active key changes (symbol or interval switch)
  useEffect(() => {
    if (candlestickSeriesRef.current) {
      chartSeriesRegistry.register(activeKey, candlestickSeriesRef.current);
    }
  }, [activeKey]);

  // Fetch historical candles when symbol or interval changes
  useEffect(() => {
    let isCancelled = false;
    const fetchKey = candleKey(symbol, interval);

    clearCandles(fetchKey);
    setCandleLoading(fetchKey, true);
    setErrorMessage(null);

    restClient
      .fetchCandles({
        symbol,
        interval,
        limit: 500,
      })
      .then((response) => {
        if (isCancelled) return;
        const chartCandles = response.items.map(candleToChartCandle);
        // Sort ascending by time — backend may return DESC order
        chartCandles.sort((a, b) => a.time - b.time);
        setCandlesForKey(fetchKey, chartCandles);
        setCandleLoading(fetchKey, false);
      })
      .catch((error: unknown) => {
        if (isCancelled) return;
        setCandleLoading(fetchKey, false);
        const message = error instanceof Error ? error.message : "Failed to load candles";
        setErrorMessage(message);
      });

    return () => {
      isCancelled = true;
    };
  }, [symbol, interval, setCandlesForKey, setCandleLoading, clearCandles]);

  // Push candle data to chart whenever the store slice changes
  useEffect(() => {
    if (!candlestickSeriesRef.current || !volumeSeriesRef.current) return;
    if (candles.length === 0) return;

    const candlestickData = candles.map((candle) => ({
      time: candle.time as Time,
      open: candle.open,
      high: candle.high,
      low: candle.low,
      close: candle.close,
    }));

    const volumeData = candles.map((candle) => ({
      time: candle.time as Time,
      value: candle.volume,
      color: candle.close >= candle.open ? CHART_THEME.volumeUpColor : CHART_THEME.volumeDownColor,
    }));

    candlestickSeriesRef.current.setData(candlestickData);
    volumeSeriesRef.current.setData(volumeData);
  }, [candles]);

  // Apply indicator visibility (volume) regardless of candle count
  useEffect(() => {
    if (volumeSeriesRef.current) {
      volumeSeriesRef.current.applyOptions({ visible: settings.volume.enabled });
    }
  }, [settings.volume.enabled]);

  // Compute and apply MA/VWAP from candles whenever candles or settings change
  useEffect(() => {
    if (!ma1SeriesRef.current || !ma2SeriesRef.current || !vwapSeriesRef.current) return;
    if (candles.length === 0) return;

    const { ma, vwap: vwapSettings } = settings;

    if (ma.enabled) {
      const ma1Data = sma(candles, ma.periods[0]).map((p) => ({ time: p.time as Time, value: p.value }));
      const ma2Data = sma(candles, ma.periods[1]).map((p) => ({ time: p.time as Time, value: p.value }));
      ma1SeriesRef.current.setData(ma1Data);
      ma2SeriesRef.current.setData(ma2Data);
      ma1SeriesRef.current.applyOptions({ visible: true });
      ma2SeriesRef.current.applyOptions({ visible: true });
    } else {
      ma1SeriesRef.current.applyOptions({ visible: false });
      ma2SeriesRef.current.applyOptions({ visible: false });
    }

    if (vwapSettings.enabled) {
      // VWAP resets daily for sub-hour intervals (1m, 5m, 15m), continuous for 1h
      const session = interval !== '1h' ? 'day' : undefined;
      const vwapData = vwap(candles, session).map((p) => ({ time: p.time as Time, value: p.value }));
      vwapSeriesRef.current.setData(vwapData);
      vwapSeriesRef.current.applyOptions({ visible: true });
    } else {
      vwapSeriesRef.current.applyOptions({ visible: false });
    }
  }, [candles, settings, interval]);

  // Build marker array — only recomputed when positions or symbol change
  const markers = useMemo<SeriesMarker<Time>[]>(() => {
    const result: SeriesMarker<Time>[] = [];

    // Entry markers for open positions: yellow down arrow above bar
    for (const position of openPositions.filter((p) => p.symbol === symbol)) {
      if (!position.openedAt) continue;
      const openedAtTime = Math.floor(new Date(position.openedAt).getTime() / 1000) as Time;
      result.push({
        time: openedAtTime,
        position: "aboveBar",
        color: "#eab308",
        shape: "arrowDown",
        text: "▼ OPEN",
      });
    }

    // Entry + exit markers for closed positions
    for (const position of closedPositions.filter((p) => p.symbol === symbol)) {
      if (!position.openedAt) continue;
      const openedAtTime = Math.floor(new Date(position.openedAt).getTime() / 1000) as Time;

      // Entry — red down arrow labeled SHORT
      result.push({
        time: openedAtTime,
        position: "aboveBar",
        color: "#ef4444",
        shape: "arrowDown",
        text: "▼ SHORT",
      });

      // Exit — shape and color depend on the close reason
      if (position.closedAt && position.closeReason) {
        const closedAtTime = Math.floor(new Date(position.closedAt).getTime() / 1000) as Time;

        if (position.closeReason === "TAKE_PROFIT") {
          result.push({
            time: closedAtTime,
            position: "belowBar",
            color: "#22c55e",
            shape: "arrowUp",
            text: "▲ TAKE_PROFIT",
          });
        } else if (position.closeReason === "STOP_LOSS") {
          result.push({
            time: closedAtTime,
            position: "aboveBar",
            color: "#ef4444",
            shape: "circle",
            text: "● STOP_LOSS",
          });
        } else if (position.closeReason === "TRAILING_STOP") {
          result.push({
            time: closedAtTime,
            position: "aboveBar",
            color: "#ef4444",
            shape: "circle",
            text: "● TRAILING_STOP",
          });
        }
      }
    }

    // lightweight-charts requires markers sorted ascending by time
    result.sort((a, b) => (a.time as number) - (b.time as number));
    return result;
  }, [openPositions, closedPositions, symbol]);

  // Push stable markers reference to chart — only fires when markers change
  useEffect(() => {
    if (!markersApiRef.current) return;
    markersApiRef.current.setMarkers(markers);
  }, [markers]);

  return (
    <div className="price-chart">
      <div className="price-chart-controls">
        <SymbolSelector symbol={symbol} onSymbolChange={setSymbol} />
        <IntervalButtons interval={interval} onIntervalChange={setInterval} />
        <ChartSettings settingsKey={settingsKey} />
      </div>
      <div className="price-chart-container" ref={containerRef}>
        {isKeyLoading && (
          <div className="price-chart-overlay price-chart-loading">
            <span className="price-chart-spinner" aria-hidden="true" />
            Loading…
          </div>
        )}
        {!isKeyLoading && errorMessage && (
          <div className="price-chart-overlay price-chart-error">{errorMessage}</div>
        )}
      </div>
    </div>
  );
}
