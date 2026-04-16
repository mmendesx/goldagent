package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
)

// OrderRepository defines persistence operations for exchange orders.
type OrderRepository interface {
	InsertOrder(ctx context.Context, order domain.Order) (int64, error)
	UpdateOrderStatus(ctx context.Context, id int64, status domain.OrderStatus, filledQty, filledPrice, fee decimal.Decimal, feeAsset string, rawResponse []byte) error
	FindOrderByID(ctx context.Context, id int64) (*domain.Order, error)
	FindOrdersBySymbol(ctx context.Context, symbol string, limit, offset int) ([]domain.Order, error)
	FindRecentOrders(ctx context.Context, limit, offset int) ([]domain.Order, error)
}

type orderRepository struct {
	pool *pgxpool.Pool
}

// NewOrderRepository returns an OrderRepository backed by the given connection pool.
func NewOrderRepository(pool *pgxpool.Pool) OrderRepository {
	return &orderRepository{pool: pool}
}

// InsertOrder persists an order record and returns its generated ID.
func (r *orderRepository) InsertOrder(ctx context.Context, o domain.Order) (int64, error) {
	const query = `
		INSERT INTO orders
			(exchange, external_order_id, decision_id, symbol, side,
			 quantity, price, filled_quantity, filled_price,
			 fee, fee_asset, status, raw_response)
		VALUES
			($1::order_exchange, $2, $3, $4, $5::order_side,
			 $6::numeric, $7::numeric, $8::numeric, $9::numeric,
			 $10::numeric, $11, $12::order_status, $13)
		RETURNING id`

	var id int64
	err := r.pool.QueryRow(ctx, query,
		string(o.Exchange), nullableText(o.ExternalOrderID), o.DecisionID,
		o.Symbol, string(o.Side),
		decimalToString(o.Quantity), decimalToString(o.Price),
		decimalToString(o.FilledQuantity), decimalToString(o.FilledPrice),
		decimalToString(o.Fee), nullableText(o.FeeAsset),
		string(o.Status), nullableBytes(o.RawResponse),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert order for symbol %q side %q exchange %q: %w",
			o.Symbol, o.Side, o.Exchange, err)
	}
	return id, nil
}

// UpdateOrderStatus updates the fill details, fee, status, and raw exchange response
// for the given order ID, and refreshes updated_at.
func (r *orderRepository) UpdateOrderStatus(
	ctx context.Context,
	id int64,
	status domain.OrderStatus,
	filledQty, filledPrice, fee decimal.Decimal,
	feeAsset string,
	rawResponse []byte,
) error {
	const query = `
		UPDATE orders
		SET status          = $2::order_status,
		    filled_quantity = $3::numeric,
		    filled_price    = $4::numeric,
		    fee             = $5::numeric,
		    fee_asset       = $6,
		    raw_response    = $7,
		    updated_at      = NOW()
		WHERE id = $1`

	tag, err := r.pool.Exec(ctx, query,
		id, string(status),
		decimalToString(filledQty), decimalToString(filledPrice),
		decimalToString(fee), nullableText(feeAsset),
		nullableBytes(rawResponse),
	)
	if err != nil {
		return fmt.Errorf("update order status for id %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update order status: order with id %d not found", id)
	}
	return nil
}

// FindOrderByID returns the order with the given ID, or nil if it does not exist.
func (r *orderRepository) FindOrderByID(ctx context.Context, id int64) (*domain.Order, error) {
	const query = `
		SELECT id, exchange, COALESCE(external_order_id, ''), decision_id, symbol, side,
		       quantity::text, COALESCE(price::text, '0'), filled_quantity::text,
		       COALESCE(filled_price::text, '0'), fee::text, COALESCE(fee_asset, ''),
		       status, raw_response, created_at, updated_at
		FROM orders
		WHERE id = $1`

	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("query order by id %d: %w", id, err)
	}
	defer rows.Close()

	orders, err := scanOrderRows(rows)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return nil, nil
	}
	return &orders[0], nil
}

// FindOrdersBySymbol returns paginated orders for a symbol, ordered by created_at descending.
func (r *orderRepository) FindOrdersBySymbol(ctx context.Context, symbol string, limit, offset int) ([]domain.Order, error) {
	const query = `
		SELECT id, exchange, COALESCE(external_order_id, ''), decision_id, symbol, side,
		       quantity::text, COALESCE(price::text, '0'), filled_quantity::text,
		       COALESCE(filled_price::text, '0'), fee::text, COALESCE(fee_asset, ''),
		       status, raw_response, created_at, updated_at
		FROM orders
		WHERE symbol = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, query, symbol, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query orders for symbol %q limit %d offset %d: %w", symbol, limit, offset, err)
	}
	defer rows.Close()

	return scanOrderRows(rows)
}

// FindRecentOrders returns paginated orders across all symbols, ordered by created_at descending.
func (r *orderRepository) FindRecentOrders(ctx context.Context, limit, offset int) ([]domain.Order, error) {
	const query = `
		SELECT id, exchange, COALESCE(external_order_id, ''), decision_id, symbol, side,
		       quantity::text, COALESCE(price::text, '0'), filled_quantity::text,
		       COALESCE(filled_price::text, '0'), fee::text, COALESCE(fee_asset, ''),
		       status, raw_response, created_at, updated_at
		FROM orders
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query recent orders limit %d offset %d: %w", limit, offset, err)
	}
	defer rows.Close()

	return scanOrderRows(rows)
}

func scanOrderRows(rows pgx.Rows) ([]domain.Order, error) {
	var orders []domain.Order
	for rows.Next() {
		var o domain.Order
		var exchange, side, status string
		var quantity, price, filledQuantity, filledPrice, fee string

		if err := rows.Scan(
			&o.ID, &exchange, &o.ExternalOrderID, &o.DecisionID, &o.Symbol, &side,
			&quantity, &price, &filledQuantity,
			&filledPrice, &fee, &o.FeeAsset,
			&status, &o.RawResponse, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan order row: %w", err)
		}

		o.Exchange = domain.OrderExchange(exchange)
		o.Side = domain.OrderSide(side)
		o.Status = domain.OrderStatus(status)

		fields := []struct {
			name string
			raw  string
			dest *decimal.Decimal
		}{
			{"quantity", quantity, &o.Quantity},
			{"price", price, &o.Price},
			{"filled_quantity", filledQuantity, &o.FilledQuantity},
			{"filled_price", filledPrice, &o.FilledPrice},
			{"fee", fee, &o.Fee},
		}
		for _, f := range fields {
			val, err := decimal.NewFromString(f.raw)
			if err != nil {
				return nil, fmt.Errorf("parse order field %q value %q: %w", f.name, f.raw, err)
			}
			*f.dest = val
		}

		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate order rows: %w", err)
	}
	return orders, nil
}

// nullableBytes returns nil for an empty/nil byte slice so that optional JSONB
// columns store NULL rather than an empty JSON value.
func nullableBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	return b
}
