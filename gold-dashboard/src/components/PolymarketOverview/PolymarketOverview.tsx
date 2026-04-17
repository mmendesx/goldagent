import { useDashboardStore } from "../../store";
import { OpenPositions } from "../OpenPositions/OpenPositions";
import "./PolymarketOverview.css";

export function PolymarketOverview() {
  const exchangeBalances = useDashboardStore((s) => s.exchangeBalances);
  const pm = exchangeBalances?.polymarket;

  const statusLabel = pm?.status === "ok" ? "Connected" : pm?.status === "error" ? "Error" : "Not configured";
  const statusClass = pm?.status === "ok" ? "status--ok" : pm?.status === "error" ? "status--error" : "status--inactive";

  return (
    <div className="polymarket-overview">
      <div className="balance-card">
        <h3 className="balance-card__title">Polymarket Balance</h3>
        <div className="balance-card__row">
          <span className="balance-card__amount">
            {pm?.status === "ok" ? `${parseFloat(pm.balance).toFixed(2)} USDC` : "—"}
          </span>
          <span className={`balance-card__status ${statusClass}`}>{statusLabel}</span>
        </div>
      </div>

      <div className="polymarket-overview__positions">
        <h3 className="section-title">Open Positions</h3>
        <OpenPositions />
      </div>
    </div>
  );
}
