import { useEffect, useReducer, useState } from "react";
import { restClient } from "../../api";
import { formatPrice } from "../../utils";
import type { Position, TradingSymbol } from "../../types";
import "./TradeHistory.css";

const PAGE_SIZE = 50;
const AVAILABLE_SYMBOLS: (TradingSymbol | "ALL")[] = ["ALL", "BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"];

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

export function TradeHistory() {
  const [positions, setPositions] = useState<Position[]>([]);
  const [{ isLoading, errorMessage }, dispatch] = useReducer(fetchReducer, { isLoading: true, errorMessage: null });
  const [symbolFilter, setSymbolFilter] = useState<TradingSymbol | "ALL">("ALL");
  const [currentPage, setCurrentPage] = useState(0);
  const [hasMore, setHasMore] = useState(false);

  useEffect(() => {
    let isCancelled = false;
    dispatch({ type: "start" });

    restClient
      .fetchClosedPositions(PAGE_SIZE, currentPage * PAGE_SIZE)
      .then((response) => {
        if (isCancelled) return;
        setPositions(response.items ?? []);
        setHasMore(response.hasMore);
        dispatch({ type: "success" });
      })
      .catch((error: unknown) => {
        if (isCancelled) return;
        const message = error instanceof Error ? error.message : "Failed to load trade history";
        dispatch({ type: "error", message });
      });

    return () => {
      isCancelled = true;
    };
  }, [currentPage]);

  // Filter client-side by symbol (the backend endpoint for positions/history doesn't support symbol filter)
  const filteredPositions = symbolFilter === "ALL"
    ? positions
    : positions.filter((position) => position.symbol === symbolFilter);

  return (
    <div className="trade-history">
      <div className="trade-history-controls">
        <label className="trade-history-filter">
          <span className="trade-history-filter-label">Symbol</span>
          <select
            value={symbolFilter}
            onChange={(event) => setSymbolFilter(event.target.value as TradingSymbol | "ALL")}
            className="trade-history-select"
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
            onClick={() => setCurrentPage((page) => Math.max(0, page - 1))}
            disabled={currentPage === 0 || isLoading}
          >
            ← Previous
          </button>
          <span className="trade-history-page-info">Page {currentPage + 1}</span>
          <button
            type="button"
            className="trade-history-page-button"
            onClick={() => setCurrentPage((page) => page + 1)}
            disabled={!hasMore || isLoading}
          >
            Next →
          </button>
        </div>
      </div>

      {errorMessage && (
        <div className="trade-history-error" role="alert">{errorMessage}</div>
      )}

      {isLoading && positions.length === 0 ? (
        <div className="trade-history-empty">Loading…</div>
      ) : filteredPositions.length === 0 ? (
        <div className="trade-history-empty">
          {symbolFilter === "ALL"
            ? "No closed trades yet. History will appear after the first exit."
            : `No closed trades for ${symbolFilter}.`}
        </div>
      ) : (
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
              {filteredPositions.map((position) => {
                const realizedPnl = position.realizedPnl ?? "0";
                const realizedPnlNumeric = parseFloat(realizedPnl);
                const pnlClassName = realizedPnlNumeric > 0 ? "pnl-positive" : realizedPnlNumeric < 0 ? "pnl-negative" : "pnl-neutral";
                const sideClassName = position.side === "LONG" ? "side-long" : "side-short";

                return (
                  <tr key={position.id}>
                    <td>{position.closedAt ? formatDateTime(position.closedAt) : "—"}</td>
                    <td className="symbol-cell">{position.symbol}</td>
                    <td>
                      <span className={`side-badge ${sideClassName}`}>{position.side}</span>
                    </td>
                    <td className="numeric-cell">{formatPrice(position.entryPrice)}</td>
                    <td className="numeric-cell">{position.exitPrice ? formatPrice(position.exitPrice) : "—"}</td>
                    <td className="numeric-cell">{formatPrice(position.quantity)}</td>
                    <td className={`numeric-cell ${pnlClassName}`}>
                      {realizedPnlNumeric >= 0 ? "+" : ""}{formatPrice(realizedPnl)}
                    </td>
                    <td>
                      {position.closeReason && (
                        <span className={`close-reason close-reason--${position.closeReason.toLowerCase().replace(/_/g, "-")}`}>
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
      )}
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
