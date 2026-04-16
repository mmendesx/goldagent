DROP INDEX IF EXISTS idx_positions_closed_at;
DROP INDEX IF EXISTS idx_positions_opened_at;
DROP INDEX IF EXISTS idx_positions_symbol_status;
DROP INDEX IF EXISTS idx_positions_status;
DROP TABLE IF EXISTS positions;
DROP TYPE IF EXISTS position_close_reason;
DROP TYPE IF EXISTS position_status;
DROP TYPE IF EXISTS position_side;
