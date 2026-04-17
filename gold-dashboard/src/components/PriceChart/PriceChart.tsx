import { useEffect, useReducer, useRef } from "react";
import {
  createChart,
  createSeriesMarkers,
  CandlestickSeries,
  HistogramSeries,
  LineStyle,
  type IChartApi,
  type ISeriesApi,
  type IPriceLine,
  type ISeriesMarkersPluginApi,
  type SeriesMarker,
  type Time,
  ColorType,
} from "lightweight-charts";
import { useDashboardStore, candleKey, candleToChartCandle } from "../../store";
import { restClient } from "../../api";
import { useLiveTicker } from "../../hooks/useLiveTicker";
import { Skeleton } from "../Skeleton/Skeleton";
import { SymbolSelector } from "../SymbolSelector/SymbolSelector";
import { IntervalButtons } from "../IntervalButtons/IntervalButtons";
import type { ChartCandle } from "../../types";
import "./PriceChart.css";

const EMPTY_CANDLES: ChartCandle[] = [];

type FetchState = { isLoading: boolean; errorMessage: string | null };
type FetchAction =
  | { type: "start" }
  | { type: "success" }
  | { type: "error"; message: string };

function fetchReducer(_state: FetchState, action: FetchAction): FetchState {
  switch (action.type) {
    case "start": return { isLoading: true, errorMessage: null };
    case "success": return { isLoading: false, errorMessage: null };
    case "error": return { isLoading: false, errorMessage: action.message };
  }
}

// Dark theme hex values — lightweight-charts cannot read CSS variables
const CHART_THEME = {
  background: "#111318",
  textColor: "#9aa0ac",
  gridColor: "#1e2230",
  borderColor: "#2c3040",
  upColor: "#22c55e",
  downColor: "#ef4444",
  volumeUpColor: "rgba(34, 197, 94, 0.3)",
  volumeDownColor: "rgba(239, 68, 68, 0.3)",
  priceLineColor: "#f59e0b",
};

export function PriceChart() {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<IChartApi | null>(null);
  const candlestickSeriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const volumeSeriesRef = useRef<ISeriesApi<"Histogram"> | null>(null);
  const priceLineRef = useRef<IPriceLine | null>(null);
  const markersApiRef = useRef<ISeriesMarkersPluginApi<Time> | null>(null);
  // Tracks the symbol:interval key for which setData was last called.
  // When this matches current key we skip setData and use update() for WS ticks.
  const seededKeyRef = useRef<string | null>(null);

  const [{ isLoading, errorMessage }, dispatch] = useReducer(fetchReducer, {
    isLoading: true,
    errorMessage: null,
  });

  const selectedSymbol = useDashboardStore((state) => state.selectedSymbol);
  const selectedInterval = useDashboardStore((state) => state.selectedInterval);
  const setCandlesForKey = useDashboardStore((state) => state.setCandlesForKey);
  const openPositions = useDashboardStore((state) => state.openPositions);
  const closedPositions = useDashboardStore((state) => state.closedPositions);

  const key = candleKey(selectedSymbol, selectedInterval);
  const candles = useDashboardStore((state) => state.candlesByKey[key]) ?? EMPTY_CANDLES;

  const ticker = useLiveTicker(selectedSymbol);

  // Initialize chart once on mount
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

    // Create dashed price line for live ticker — updated via applyOptions, never recreated
    const priceLine = candlestickSeries.createPriceLine({
      price: 0,
      color: CHART_THEME.priceLineColor,
      lineWidth: 1,
      lineStyle: LineStyle.Dashed,
      axisLabelVisible: false,
      title: "Live",
    });

    const markersApi = createSeriesMarkers(candlestickSeries, []);

    chartRef.current = chart;
    candlestickSeriesRef.current = candlestickSeries;
    volumeSeriesRef.current = volumeSeries;
    priceLineRef.current = priceLine;
    markersApiRef.current = markersApi;

    return () => {
      markersApi.detach();
      chart.remove();
      chartRef.current = null;
      candlestickSeriesRef.current = null;
      volumeSeriesRef.current = null;
      priceLineRef.current = null;
      markersApiRef.current = null;
      seededKeyRef.current = null;
    };
  }, []);

  // Fetch historical candles when symbol or interval changes
  useEffect(() => {
    let isCancelled = false;
    dispatch({ type: "start" });
    // Clear stale data immediately so old candles don't persist while loading
    seededKeyRef.current = null;
    candlestickSeriesRef.current?.setData([]);
    volumeSeriesRef.current?.setData([]);

    restClient
      .fetchCandles({
        symbol: selectedSymbol,
        interval: selectedInterval,
        limit: 500,
      })
      .then((response) => {
        if (isCancelled) return;
        const chartCandles = response.items.map(candleToChartCandle);
        chartCandles.sort((a, b) => a.time - b.time);
        // Invalidate here so the upcoming store update triggers a full setData,
        // not just update(lastCandle). Moving this to fetch-start would cause
        // cached-symbol revisits to miss 499 candles on the incremental path.
        seededKeyRef.current = null;
        setCandlesForKey(candleKey(selectedSymbol, selectedInterval), chartCandles);
        dispatch({ type: "success" });
      })
      .catch((error: unknown) => {
        if (isCancelled) return;
        const message =
          error instanceof Error ? error.message : "Failed to load candles";
        dispatch({ type: "error", message });
      });

    return () => {
      isCancelled = true;
    };
  }, [selectedSymbol, selectedInterval, setCandlesForKey]);

  // Push candle data to chart — full seed on key change, incremental update on WS tick
  useEffect(() => {
    const candlestickSeries = candlestickSeriesRef.current;
    const volumeSeries = volumeSeriesRef.current;
    if (!candlestickSeries || !volumeSeries) return;
    if (candles.length === 0) return;

    const lastCandle = candles[candles.length - 1];

    if (seededKeyRef.current !== key) {
      // Full seed after symbol/interval change or initial fetch
      const candlestickData = candles.map((c) => ({
        time: c.time as Time,
        open: c.open,
        high: c.high,
        low: c.low,
        close: c.close,
      }));

      const volumeData = candles.map((c) => ({
        time: c.time as Time,
        value: c.volume,
        color: c.close >= c.open ? CHART_THEME.volumeUpColor : CHART_THEME.volumeDownColor,
      }));

      candlestickSeries.setData(candlestickData);
      volumeSeries.setData(volumeData);
      seededKeyRef.current = key;
    } else {
      // Incremental update from WS candle_update — only push the last candle
      candlestickSeries.update({
        time: lastCandle.time as Time,
        open: lastCandle.open,
        high: lastCandle.high,
        low: lastCandle.low,
        close: lastCandle.close,
      });

      volumeSeries.update({
        time: lastCandle.time as Time,
        value: lastCandle.volume,
        color:
          lastCandle.close >= lastCandle.open
            ? CHART_THEME.volumeUpColor
            : CHART_THEME.volumeDownColor,
      });
    }
  }, [candles, key]);

  // Reset price line when symbol changes to avoid showing the previous symbol's price
  // on the new symbol's candles before the first tick arrives.
  useEffect(() => {
    priceLineRef.current?.applyOptions({ price: 0, axisLabelVisible: false });
  }, [selectedSymbol]);

  // Update live price line when ticker changes — does NOT mutate candle data
  useEffect(() => {
    const priceLine = priceLineRef.current;
    if (!priceLine) return;

    if (ticker === null) {
      // No ticker for this symbol yet — hide the label but keep the line object alive
      priceLine.applyOptions({ price: 0, axisLabelVisible: false });
      return;
    }

    priceLine.applyOptions({ price: ticker.price, axisLabelVisible: true });
  }, [ticker]);

  // Rebuild trade markers whenever positions or selected symbol change
  useEffect(() => {
    if (!markersApiRef.current) return;

    const markers: SeriesMarker<Time>[] = [];

    const symbolOpenPositions = openPositions.filter((p) => p.symbol === selectedSymbol);
    const symbolClosedPositions = closedPositions.filter((p) => p.symbol === selectedSymbol);

    for (const position of symbolOpenPositions) {
      const openedAtTime = Math.floor(new Date(position.openedAt).getTime() / 1000) as Time;
      markers.push({
        time: openedAtTime,
        position: "aboveBar",
        color: "#eab308",
        shape: "arrowDown",
        text: "OPEN",
      });
    }

    for (const position of symbolClosedPositions) {
      const openedAtTime = Math.floor(new Date(position.openedAt).getTime() / 1000) as Time;

      markers.push({
        time: openedAtTime,
        position: "aboveBar",
        color: "#ef4444",
        shape: "arrowDown",
        text: "SHORT",
      });

      if (position.closedAt && position.closeReason) {
        const closedAtTime = Math.floor(new Date(position.closedAt).getTime() / 1000) as Time;

        if (position.closeReason === "TAKE_PROFIT") {
          markers.push({
            time: closedAtTime,
            position: "belowBar",
            color: "#22c55e",
            shape: "arrowUp",
            text: "TAKE_PROFIT",
          });
        } else if (
          position.closeReason === "STOP_LOSS" ||
          position.closeReason === "TRAILING_STOP"
        ) {
          markers.push({
            time: closedAtTime,
            position: "aboveBar",
            color: "#ef4444",
            shape: "circle",
            text: position.closeReason,
          });
        }
      }
    }

    markers.sort((a, b) => (a.time as number) - (b.time as number));
    markersApiRef.current.setMarkers(markers);
  }, [openPositions, closedPositions, selectedSymbol]);

  return (
    <div className="price-chart">
      <div className="price-chart-controls">
        <SymbolSelector />
        <IntervalButtons />
      </div>
      <div className="price-chart-container" ref={containerRef}>
        {isLoading && (
          <div className="price-chart-skeleton">
            <Skeleton variant="block" height="400px" />
          </div>
        )}
        {errorMessage && (
          <div className="price-chart-overlay price-chart-error" role="alert">
            {errorMessage}
          </div>
        )}
      </div>
    </div>
  );
}
