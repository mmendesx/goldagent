DROP INDEX IF EXISTS idx_orders_status;
DROP INDEX IF EXISTS idx_orders_decision_id;
DROP INDEX IF EXISTS idx_orders_symbol_created_at;
DROP TABLE IF EXISTS orders;
DROP TYPE IF EXISTS order_exchange;
DROP TYPE IF EXISTS order_status;
DROP TYPE IF EXISTS order_side;
