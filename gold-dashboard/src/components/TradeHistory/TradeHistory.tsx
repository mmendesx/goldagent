import { useEffect, useReducer, useState } from "react";
import { restClient } from "../../api";
import { formatPrice } from "../../utils";
import { AsyncBoundary } from "../AsyncBoundary";
import type { AsyncState } from "../../hooks";
import type { Position, TradingSymbol } from "../../types";
import "./TradeHistory.css";

const PAGE_LIMIT = 50;
const AVAILABLE_SYMBOLS: (TradingSymbol | "ALL")[] = ["ALL", "BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"];

type FetchState = {
  asyncState: AsyncState;
  errorMessage: string | null;
};

type FetchAction =
  | { type: "start" }
  | { type: "success"; hasItems: boolean }
  | { type: "error"; message: string };

function fetchReducer(_state: FetchState, action: FetchAction): FetchState {
  switch (action.type) {
    case "start":
      return { asyncState: "loading", errorMessage: null };
    case "success":
      return { asyncState: action.hasItems ? "ready" : "empty", errorMessage: null };
    case "error":
      return { asyncState: "error", errorMessage: action.message };
  }
}

export function TradeHistory() {
  const [positions, setPositions] = useState<Position[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [offset, setOffset] = useState(0);
  const [symbolFilter, setSymbolFilter] = useState<TradingSymbol | "ALL">("ALL");
  const [{ asyncState, errorMessage }, dispatch] = useReducer(fetchReducer, {
    asyncState: "loading",
    errorMessage: null,
  });

  useEffect(() => {
    let cancelled = false;
    dispatch({ type: "start" });

    const symbol = symbolFilter === "ALL" ? undefined : symbolFilter;

    restClient
      .fetchPositionsHistory({ symbol, limit: PAGE_LIMIT, offset })
      .then((response) => {
        if (cancelled) return;
        const items = response.items ?? [];
        setPositions(items);
        setHasMore(response.hasMore);
        dispatch({ type: "success", hasItems: items.length > 0 });
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        const message = error instanceof Error ? error.message : "Failed to load trade history";
        dispatch({ type: "error", message });
      });

    return () => {
      cancelled = true;
    };
  }, [offset, symbolFilter]);

  function handleSymbolChange(value: TradingSymbol | "ALL") {
    setSymbolFilter(value);
    setOffset(0);
  }

  const isLoading = asyncState === "loading";
  const currentPage = Math.floor(offset / PAGE_LIMIT) + 1;

  return (
    <div className="trade-history">
      <div className="trade-history-controls">
        <label className="trade-history-filter">
          <span className="trade-history-filter-label">Symbol</span>
          <select
            value={symbolFilter}
            onChange={(event) => handleSymbolChange(event.target.value as TradingSymbol | "ALL")}
            className="trade-history-select"
            disabled={isLoading}
          >
            {AVAILABLE_SYMBOLS.map((symbol) => (
              <option key={symbol} value={symbol}>{symbol}</option>
            ))}
          </select>
        </label>

        <div className="trade-history-pagination">
          <button
            type="button"
            className="trade-history-page-button"
            onClick={() => setOffset((prev) => Math.max(0, prev - PAGE_LIMIT))}
            disabled={offset === 0 || isLoading}
          >
            ← Previous
          </button>
          <span className="trade-history-page-info">Page {currentPage}</span>
          <button
            type="button"
            className="trade-history-page-button"
            onClick={() => setOffset((prev) => prev + PAGE_LIMIT)}
            disabled={!hasMore || isLoading}
          >
            Next →
          </button>
        </div>
      </div>

      <AsyncBoundary
        state={asyncState}
        onRetry={() => {
          setOffset(0);
          setSymbolFilter("ALL");
        }}
        emptyCopy={
          symbolFilter === "ALL"
            ? "No trades found. History will appear after the first closed position."
            : `No trades found for ${symbolFilter}.`
        }
        errorMessage={errorMessage ?? "Failed to load trade history"}
      >
        <div className="trade-history-table-container">
          <table className="trade-history-table">
            <thead>
              <tr>
                <th>Closed At</th>
                <th>Symbol</th>
                <th>Side</th>
                <th>Entry</th>
                <th>Exit</th>
                <th>Quantity</th>
                <th>P&amp;L</th>
                <th>Reason</th>
              </tr>
            </thead>
            <tbody>
              {positions.map((position) => {
                const realizedPnl = position.realizedPnl ?? "0";
                const realizedPnlNumeric = parseFloat(realizedPnl);
                const pnlClassName =
                  realizedPnlNumeric > 0
                    ? "pnl-positive"
                    : realizedPnlNumeric < 0
                    ? "pnl-negative"
                    : "pnl-neutral";
                const sideClassName = position.side === "LONG" ? "side-long" : "side-short";

                return (
                  <tr key={position.id}>
                    <td>{position.closedAt ? formatDateTime(position.closedAt) : "—"}</td>
                    <td className="symbol-cell">{position.symbol}</td>
                    <td>
                      <span className={`side-badge ${sideClassName}`}>{position.side}</span>
                    </td>
                    <td className="numeric-cell">{formatPrice(position.entryPrice)}</td>
                    <td className="numeric-cell">
                      {position.exitPrice ? formatPrice(position.exitPrice) : "—"}
                    </td>
                    <td className="numeric-cell">{formatPrice(position.quantity)}</td>
                    <td className={`numeric-cell ${pnlClassName}`}>
                      {realizedPnlNumeric >= 0 ? "+" : ""}{formatPrice(realizedPnl)}
                    </td>
                    <td>
                      {position.closeReason && (
                        <span
                          className={`close-reason close-reason--${position.closeReason
                            .toLowerCase()
                            .replace(/_/g, "-")}`}
                        >
                          {position.closeReason.replace(/_/g, " ")}
                        </span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </AsyncBoundary>
    </div>
  );
}

function formatDateTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleString("en-US", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}
