import { useEffect, useMemo, useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useDashboardStore, computeOpenPositionsWithLivePnl } from "../../store";
import { restClient } from "../../api";
import { formatPrice } from "../../utils";
import { Skeleton, SkeletonContainer, VisuallyHidden } from "../../design-system";
import "./OpenPositions.css";

const REFRESH_INTERVAL_MILLISECONDS = 3000;

export function OpenPositions() {
  const prefersReducedMotion = useReducedMotion();
  const rawOpenPositions = useDashboardStore((s) => s.openPositions);
  const lastPrice = useDashboardStore((s) => s.lastPrice);
  const setOpenPositions = useDashboardStore((state) => state.setOpenPositions);
  const [isInitialLoading, setIsInitialLoading] = useState(true);

  const openPositions = useMemo(
    () => computeOpenPositionsWithLivePnl(rawOpenPositions, lastPrice),
    [rawOpenPositions, lastPrice]
  );

  // Periodic refetch for live P&L (WebSocket doesn't send price ticks)
  useEffect(() => {
    let isCancelled = false;

    async function refresh(): Promise<void> {
      try {
        const positions = await restClient.fetchOpenPositions();
        if (!isCancelled) {
          setOpenPositions(positions ?? []);
          setIsInitialLoading(false);
        }
      } catch (error) {
        if (!isCancelled) {
          console.warn("Failed to refresh open positions", error);
          setIsInitialLoading(false);
        }
      }
    }

    refresh();
    const intervalId = window.setInterval(refresh, REFRESH_INTERVAL_MILLISECONDS);

    return () => {
      isCancelled = true;
      window.clearInterval(intervalId);
    };
  }, [setOpenPositions]);

  return (
    <SkeletonContainer busy={isInitialLoading}>
      {isInitialLoading ? (
        <Skeleton lines={5} className="open-positions-skeleton" />
      ) : openPositions.length === 0 ? (
        <div className="open-positions-empty">
          <p>No open positions</p>
          <p className="open-positions-empty-hint">Positions will appear here when Gold opens a trade.</p>
        </div>
      ) : (
        <div className="open-positions">
          <table className="open-positions-table">
            <caption><VisuallyHidden>Open Positions</VisuallyHidden></caption>
            <thead>
              <tr>
                <th scope="col">Symbol</th>
                <th scope="col">Side</th>
                <th scope="col">Entry Price</th>
                <th scope="col">Current Price</th>
                <th scope="col">Unrealized P&amp;L</th>
                <th scope="col">SL</th>
                <th scope="col">TP</th>
                <th scope="col">Quantity</th>
              </tr>
            </thead>
            <tbody>
              <AnimatePresence>
                {openPositions.map((position, i) => {
                  const unrealizedPnlNumeric = parseFloat(position.unrealizedPnl);
                  const pnlClassName =
                    unrealizedPnlNumeric > 0
                      ? "pnl-positive"
                      : unrealizedPnlNumeric < 0
                      ? "pnl-negative"
                      : "pnl-neutral";
                  const sideClassName = position.side === "LONG" ? "side-long" : "side-short";

                  return (
                    <motion.tr
                      key={position.id ?? `${position.symbol}-${i}`}
                      initial={prefersReducedMotion ? false : { opacity: 0, y: 4 }}
                      animate={prefersReducedMotion ? undefined : { opacity: 1, y: 0 }}
                      exit={prefersReducedMotion ? { opacity: 0 } : { opacity: 0 }}
                      transition={prefersReducedMotion ? { duration: 0 } : { duration: 0.15, delay: i * 0.04 }}
                    >
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
                    </motion.tr>
                  );
                })}
              </AnimatePresence>
            </tbody>
          </table>
        </div>
      )}
    </SkeletonContainer>
  );
}
