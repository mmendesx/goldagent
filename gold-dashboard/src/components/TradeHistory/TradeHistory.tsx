import { useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { restClient } from "../../api";
import { useListFetch } from "../../hooks/useListFetch";
import { formatPrice } from "../../utils";
import { VisuallyHidden } from "../../design-system";
import type { TradingSymbol } from "../../types";
import "./TradeHistory.css";

const PAGE_SIZE = 50;
const AVAILABLE_SYMBOLS: (TradingSymbol | "ALL")[] = ["ALL", "BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"];

export function TradeHistory() {
  const [symbolFilter, setSymbolFilter] = useState<TradingSymbol | "ALL">("ALL");
  const [currentPage, setCurrentPage] = useState(0);

  const fetchKey = `trades-${currentPage}-${symbolFilter}`;
  const { data, error, loading, refetch } = useListFetch(
    fetchKey,
    () => restClient.fetchClosedPositions(PAGE_SIZE, currentPage * PAGE_SIZE),
    [currentPage],
  );
  const positions = data?.items ?? [];
  const hasMore = data?.hasMore ?? false;

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
            disabled={currentPage === 0 || loading}
          >
            ← Previous
          </button>
          <span className="trade-history-page-info">Page {currentPage + 1}</span>
          <button
            type="button"
            className="trade-history-page-button"
            onClick={() => setCurrentPage((page) => page + 1)}
            disabled={!hasMore || loading}
          >
            Next →
          </button>
        </div>
      </div>

      {positions.length > 0 && (
        <div className="trade-history-table-container">
          <table className="trade-history-table">
            <caption><VisuallyHidden>Trade History</VisuallyHidden></caption>
            <thead>
              <tr>
                <th scope="col">Closed At</th>
                <th scope="col">Symbol</th>
                <th scope="col">Side</th>
                <th scope="col">Entry</th>
                <th scope="col">Exit</th>
                <th scope="col">Quantity</th>
                <th scope="col">P&amp;L</th>
                <th scope="col">Reason</th>
              </tr>
            </thead>
            <tbody>
              <AnimatePresence>
                {filteredPositions.map((position, i) => {
                  const realizedPnl = position.realizedPnl ?? "0";
                  const realizedPnlNumeric = parseFloat(realizedPnl);
                  const pnlClassName = realizedPnlNumeric > 0 ? "pnl-positive" : realizedPnlNumeric < 0 ? "pnl-negative" : "pnl-neutral";
                  const sideClassName = position.side === "LONG" ? "side-long" : "side-short";

                  return (
                    <motion.tr
                      key={position.id ?? `${position.symbol}-${i}`}
                      initial={{ opacity: 0, y: 4 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0 }}
                      transition={{ duration: 0.15, delay: i * 0.04 }}
                    >
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
                    </motion.tr>
                  );
                })}
              </AnimatePresence>
            </tbody>
          </table>
        </div>
      )}

      {error && (
        <div className="trade-history-error" role="alert">
          <span>{error}</span>
          <button type="button" onClick={refetch} className="trade-history-retry">Retry</button>
        </div>
      )}

      {!error && loading && positions.length === 0 && (
        <div className="trade-history-empty">Loading…</div>
      )}

      {!error && !loading && filteredPositions.length === 0 && (
        <div className="trade-history-empty">
          {symbolFilter === "ALL"
            ? "No closed trades yet. History will appear after the first exit."
            : `No closed trades for ${symbolFilter}.`}
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
