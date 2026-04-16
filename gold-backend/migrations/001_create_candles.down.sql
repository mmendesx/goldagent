DROP INDEX IF EXISTS idx_candles_unique;
DROP INDEX IF EXISTS idx_candles_symbol_interval_close_time;
DROP INDEX IF EXISTS idx_candles_symbol_interval_open_time;

DROP TABLE IF EXISTS candles_bnbusdt;
DROP TABLE IF EXISTS candles_solusdt;
DROP TABLE IF EXISTS candles_ethusdt;
DROP TABLE IF EXISTS candles_btcusdt;
DROP TABLE IF EXISTS candles;
