import { useDashboardStore } from "../../store";
import type { TradingSymbol } from "../../types";
import "./SymbolSelector.css";

const AVAILABLE_SYMBOLS: TradingSymbol[] = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"];

export function SymbolSelector() {
  const selectedSymbol = useDashboardStore((state) => state.selectedSymbol);
  const setSelectedSymbol = useDashboardStore((state) => state.setSelectedSymbol);

  return (
    <label className="symbol-selector">
      <span className="symbol-selector-label">Symbol</span>
      <select
        className="symbol-selector-select"
        value={selectedSymbol}
        onChange={(event) => setSelectedSymbol(event.target.value as TradingSymbol)}
      >
        {AVAILABLE_SYMBOLS.map((symbol) => (
          <option key={symbol} value={symbol}>{symbol}</option>
        ))}
      </select>
    </label>
  );
}
