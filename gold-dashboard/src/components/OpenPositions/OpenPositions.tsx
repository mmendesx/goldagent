import { useEffect } from "react";
import { useDashboardStore, selectOpenPositionsWithLivePnl } from "../../store";
import { restClient } from "../../api";
import { formatPrice } from "../../utils";
import "./OpenPositions.css";

const REFRESH_INTERVAL_MILLISECONDS = 3000;

export function OpenPositions() {
  const openPositions = useDashboardStore(selectOpenPositionsWithLivePnl);
  const setOpenPositions = useDashboardStore((state) => state.setOpenPositions);

  // Periodic refetch for live P&L (WebSocket doesn't send price ticks)
  useEffect(() => {
    let isCancelled = false;

    async function refresh(): Promise<void> {
      try {
        const positions = await restClient.fetchOpenPositions();
        if (!isCancelled) {
          setOpenPositions(positions ?? []);
        }
      } catch (error) {
        console.warn("Failed to refresh open positions", error);
      }
    }

    refresh();
    const intervalId = window.setInterval(refresh, REFRESH_INTERVAL_MILLISECONDS);

    return () => {
      isCancelled = true;
      window.clearInterval(intervalId);
    };
  }, [setOpenPositions]);

  if (openPositions.length === 0) {
    return (
      <div className="open-positions-empty">
        <p>No open positions</p>
        <p className="open-positions-empty-hint">Positions will appear here when Gold opens a trade.</p>
      </div>
    );
  }

  return (
    <div className="open-positions">
      <table className="open-positions-table">
        <thead>
          <tr>
            <th>Symbol</th>
            <th>Side</th>
            <th>Entry Price</th>
            <th>Current Price</th>
            <th>Unrealized P&amp;L</th>
            <th>SL</th>
            <th>TP</th>
            <th>Quantity</th>
          </tr>
        </thead>
        <tbody>
          {openPositions.map((position) => {
            const unrealizedPnlNumeric = parseFloat(position.unrealizedPnl);
            const pnlClassName =
              unrealizedPnlNumeric > 0
                ? "pnl-positive"
                : unrealizedPnlNumeric < 0
                ? "pnl-negative"
                : "pnl-neutral";
            const sideClassName = position.side === "LONG" ? "side-long" : "side-short";

            return (
              <tr key={position.id}>
                <td className="symbol-cell">{position.symbol}</td>
                <td>
                  <span className={`side-badge ${sideClassName}`}>{position.side}</span>
                </td>
                <td className="numeric-cell">{formatPrice(position.entryPrice)}</td>
                <td className="numeric-cell">{formatPrice(position.currentPrice)}</td>
                <td className={`numeric-cell ${pnlClassName}`}>
                  {unrealizedPnlNumeric >= 0 ? "+" : ""}
                  {formatPrice(position.unrealizedPnl)}
                </td>
                <td className="numeric-cell">{formatPrice(position.stopLossPrice)}</td>
                <td className="numeric-cell">{formatPrice(position.takeProfitPrice)}</td>
                <td className="numeric-cell">{formatPrice(position.quantity)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
