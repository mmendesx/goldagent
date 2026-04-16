# Gold Agent

Our mission is to create an agent that should handle operations in Polymarket and Binance. This agent should be trained, oriented and driven by results, since its best behavior is binance day trading and specialist in polymarket binance 5 minutes charts.

The agent must have the best documentation of how handle graphs, trace patterns, templates, get histories and compare then to make the best decision. It must also analise news from market, global about every companies and countries that can move the prices. The agent can also use indicators and extensions to help prove and strengthen its decision.

This agent should run in a backend server, getting realtime prices (do not accept updates with duration, REAL TIME is REAL TIME) and handling to make operations. Look up at websockets from binance and polymarket, both have SDKs to help with. 

Once the agent has a decision to make, do it and store into its own database. 

We should have a dashboard with graph, separated by switch tabs, with must contain the agent books, tp/sl, history, etc. The dashboard also must get the realtime prices. 


Use line indicators on graphs to identify BUYs and SELLs booked along its TP and SL.

Docs: 
- https://github.com/Polymarket/real-time-data-client
- https://github.com/binance/binance-spot-api-docs/blob/master/web-socket-api.md

API keys will be provided for both binance and polymarket.


Metrics bar** — Balance, Peak Balance, Drawdown (color-coded), Win Rate, Total Trades, Open Positions

**Price chart** (TradingView Lightweight Charts)
- Candlestick chart for any of your configured symbols (BTCUSDT, ETHUSDT, SOLUSDT, BNBUSDT)
- `▼ SHORT` marker in red when a position opens
- `▲ TAKE_PROFIT` / `● STOP_LOSS` / `● TRAILING_STOP` markers in green/red when it closes
- `▼ OPEN` marker in yellow for any currently live position
- Interval buttons: 1m / 5m /15m / 1h / 4h / 1D

Open Positions panel** — Entry price, unrealized P&L (live), SL and TP levels.


Prefered backend(agent) stacks:
- Language: GO
- Postgres
- Redis

Prefered frontend(dashboard) stack:
- React + Vite+
- AnimateUI and ReactBits
- Typescript
- ES2022
