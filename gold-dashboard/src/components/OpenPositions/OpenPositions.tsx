import { useEffect } from "react";
import { useDashboardStore } from "../../store";
import { restClient } from "../../api";
import { useAsyncResource } from "../../hooks";
import { useLiveTicker } from "../../hooks/useLiveTicker";
import { AsyncBoundary } from "../AsyncBoundary";
import { formatPrice } from "../../utils";
import type { Position } from "../../types";
import "./OpenPositions.css";

function PositionRow({ position }: { position: Position }) {
  const ticker = useLiveTicker(position.symbol);
  const currentPrice = ticker?.price ?? null;

  let unrealizedPnl: number | null = null;
  if (currentPrice !== null) {
    const entry = parseFloat(position.entryPrice);
    const qty = parseFloat(position.quantity);
    // fees field not present on Position type — default to 0
    const isLong = position.side === "LONG";
    unrealizedPnl = isLong
      ? (currentPrice - entry) * qty
      : (entry - currentPrice) * qty;
  }

  const pnlClassName =
    unrealizedPnl === null
      ? ""
      : unrealizedPnl >= 0
      ? "pnl-positive"
      : "pnl-negative";

  const sideClassName = position.side === "LONG" ? "side-long" : "side-short";

  return (
    <tr>
      <td className="symbol-cell">{position.symbol}</td>
      <td>
        <span className={`side-badge ${sideClassName}`}>{position.side}</span>
      </td>
      <td className="numeric-cell">{formatPrice(position.entryPrice)}</td>
      <td className="numeric-cell">
        {currentPrice !== null ? formatPrice(String(currentPrice)) : "—"}
      </td>
      <td className={`numeric-cell ${pnlClassName}`}>
        {unrealizedPnl !== null
          ? (unrealizedPnl >= 0 ? "+" : "") + unrealizedPnl.toFixed(2)
          : "—"}
      </td>
      <td className="numeric-cell">{formatPrice(position.stopLossPrice)}</td>
      <td className="numeric-cell">{formatPrice(position.takeProfitPrice)}</td>
      <td className="numeric-cell">{formatPrice(position.quantity)}</td>
    </tr>
  );
}

function PositionsTable({ positions }: { positions: Position[] }) {
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
          {positions.map((position) => (
            <PositionRow key={position.id} position={position} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function LoadingSkeleton() {
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
          {Array.from({ length: 3 }, (_, i) => (
            <tr key={i} className="skeleton-row">
              {Array.from({ length: 8 }, (_, j) => (
                <td key={j}>
                  <span className="skeleton-cell" />
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function OpenPositions() {
  const setOpenPositions = useDashboardStore((state) => state.setOpenPositions);
  const openPositions = useDashboardStore((state) => state.openPositions);

  const resource = useAsyncResource(
    () => restClient.fetchOpenPositions(),
    (data) => data.length === 0,
  );

  // Seed the store when fetch resolves so WebSocket updates
  // (upsertOpenPosition / removeOpenPosition) continue to flow through.
  useEffect(() => {
    if (resource.state === "ready" || resource.state === "empty") {
      const positions = resource.data ?? [];
      setOpenPositions(
        positions.map((p) => ({
          ...p,
          currentPrice: p.entryPrice,
          unrealizedPnl: "0",
        })),
      );
    }
  }, [resource.state, resource.data, setOpenPositions]);

  return (
    <AsyncBoundary
      state={resource.state}
      onRetry={resource.retry}
      emptyCopy="No open positions. Agent is scanning…"
      errorMessage={resource.error ?? "Failed to load open positions"}
      loadingSkeleton={<LoadingSkeleton />}
    >
      <PositionsTable positions={openPositions} />
    </AsyncBoundary>
  );
}
