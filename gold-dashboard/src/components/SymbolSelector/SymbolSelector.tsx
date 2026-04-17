import type { TradingSymbol } from "../../types";
import "./SymbolSelector.css";

const AVAILABLE_SYMBOLS: TradingSymbol[] = ["BTCUSDT", "ETHUSDT", "SOLUSDT", "BNBUSDT"];

interface SymbolSelectorProps {
  symbol: TradingSymbol;
  onSymbolChange: (symbol: TradingSymbol) => void;
}

export function SymbolSelector({ symbol, onSymbolChange }: SymbolSelectorProps) {
  return (
    <label className="symbol-selector">
      <span className="symbol-selector-label">Symbol</span>
      <select
        className="symbol-selector-select"
        value={symbol}
        onChange={(event) => onSymbolChange(event.target.value as TradingSymbol)}
      >
        {AVAILABLE_SYMBOLS.map((s) => (
          <option key={s} value={s}>{s}</option>
        ))}
      </select>
    </label>
  );
}
