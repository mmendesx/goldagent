package rest

import (
	"log/slog"
	"net/http"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
)

type tradeHandler struct {
	orderRepository postgres.OrderRepository
	logger          *slog.Logger
}

func newTradeHandler(deps HandlerDependencies) *tradeHandler {
	return &tradeHandler{
		orderRepository: deps.OrderRepository,
		logger:          deps.Logger,
	}
}

// handleListTrades handles GET /api/v1/trades.
// Optional query param: symbol — filters results to that symbol.
// Supports pagination via limit and offset.
func (h *tradeHandler) handleListTrades(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pagination := ParsePagination(r)
	symbol := r.URL.Query().Get("symbol")

	var orders []domain.Order
	var err error

	if symbol != "" {
		orders, err = h.orderRepository.FindOrdersBySymbol(ctx, symbol, pagination.Limit, pagination.Offset)
	} else {
		orders, err = h.orderRepository.FindRecentOrders(ctx, pagination.Limit, pagination.Offset)
	}

	if err != nil {
		h.logger.Error("trade handler: failed to fetch orders",
			"path", r.URL.Path,
			"symbol", symbol,
			"limit", pagination.Limit,
			"offset", pagination.Offset,
			"error", err,
		)
		WriteError(w, http.StatusInternalServerError, "failed to fetch trades", "INTERNAL_ERROR")
		return
	}

	// Ensure JSON serializes as [] not null for empty results.
	if orders == nil {
		orders = []domain.Order{}
	}

	WriteJSON(w, http.StatusOK, PaginatedResponse{
		Items:   orders,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Count:   len(orders),
		HasMore: len(orders) == pagination.Limit,
	})
}
