export interface Candle {
  openTime: number;
  closeTime: number;
  openPrice: string;
  highPrice: string;
  lowPrice: string;
  closePrice: string;
  volume: string;
  quoteVolume: string;
  tradeCount: number;
  isClosed: boolean;
}

export interface Position {
  id: number;
  symbol: string;
  side: "LONG" | "SHORT";
  entryPrice: string;
  exitPrice?: string;
  quantity: string;
  takeProfitPrice: string;
  stopLossPrice: string;
  trailingStopDistance?: string;
  trailingStopPrice?: string;
  realizedPnl?: string;
  status: "open" | "closed";
  closeReason?: "TAKE_PROFIT" | "STOP_LOSS" | "TRAILING_STOP" | "MANUAL" | "CIRCUIT_BREAKER";
  openedAt: string;
  closedAt?: string;
}

export interface PortfolioMetrics {
  balance: string;
  peakBalance: string;
  drawdownPercent: string;
  totalPnl: string;
  winCount: number;
  lossCount: number;
  totalTrades: number;
  winRate: string;
  profitFactor?: string;
  averageWin?: string;
  averageLoss?: string;
  sharpeRatio?: string;
  maxDrawdownPercent: string;
  isCircuitBreakerActive: boolean;
}

export interface TradeRecord {
  id: number;
  symbol: string;
  side: "LONG" | "SHORT";
  entryPrice: string;
  exitPrice: string;
  quantity: string;
  realizedPnl: string;
  closeReason: string;
  openedAt: string;
  closedAt: string;
}

export interface Decision {
  id: number;
  symbol: string;
  action: "BUY" | "SELL" | "HOLD";
  confidence: number;
  executionStatus: string;
  rejectionReason?: string;
  compositeScore: string;
  isDryRun: boolean;
  createdAt: string;
}

export type TradingSymbol = "BTCUSDT" | "ETHUSDT" | "SOLUSDT" | "BNBUSDT";
export type ChartInterval = "1m" | "5m" | "15m" | "1h" | "4h" | "1D";

export interface WebSocketMessage {
  type: "candle_update" | "position_update" | "metric_update" | "trade_executed" | "decision_made";
  payload: unknown;
}
