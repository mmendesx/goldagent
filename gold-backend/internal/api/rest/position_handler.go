package rest

import (
	"log/slog"
	"net/http"

	"github.com/shopspring/decimal"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
)

type positionHandler struct {
	positionRepository postgres.PositionRepository
	cache              PriceCache
	logger             *slog.Logger
}

func newPositionHandler(deps HandlerDependencies) *positionHandler {
	return &positionHandler{
		positionRepository: deps.PositionRepository,
		cache:              deps.Cache,
		logger:             deps.Logger,
	}
}

// OpenPositionResponse adds live current price and unrealized P&L to a position.
type OpenPositionResponse struct {
	domain.Position
	CurrentPrice  decimal.Decimal `json:"currentPrice"`
	UnrealizedPnl decimal.Decimal `json:"unrealizedPnl"`
}

// handleListOpenPositions handles GET /api/v1/positions.
// Returns all open positions enriched with live price and unrealized P&L.
// If the price cache is unavailable for a symbol, currentPrice and unrealizedPnl are zero.
func (h *positionHandler) handleListOpenPositions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	positions, err := h.positionRepository.FindOpenPositions(ctx)
	if err != nil {
		h.logger.Error("position handler: failed to fetch open positions",
			"path", r.URL.Path,
			"error", err,
		)
		WriteError(w, http.StatusInternalServerError, "failed to fetch open positions", "INTERNAL_ERROR")
		return
	}

	items := make([]OpenPositionResponse, 0, len(positions))
	for _, pos := range positions {
		resp := OpenPositionResponse{Position: pos}

		ticker, err := h.cache.GetTickerPrice(ctx, pos.Symbol)
		if err != nil {
			h.logger.Error("position handler: failed to get ticker price from cache",
				"symbol", pos.Symbol,
				"error", err,
			)
			// Degrade gracefully: emit zero values rather than failing the whole response.
		} else if ticker != nil {
			resp.CurrentPrice = ticker.Price
			resp.UnrealizedPnl = computeUnrealizedPnl(pos.Side, pos.EntryPrice, ticker.Price, pos.Quantity)
		}

		items = append(items, resp)
	}

	WriteJSON(w, http.StatusOK, items)
}

// handleListClosedPositions handles GET /api/v1/positions/history.
// Returns paginated closed positions ordered by closed_at descending.
func (h *positionHandler) handleListClosedPositions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pagination := ParsePagination(r)

	positions, err := h.positionRepository.FindClosedPositions(ctx, pagination.Limit, pagination.Offset)
	if err != nil {
		h.logger.Error("position handler: failed to fetch closed positions",
			"path", r.URL.Path,
			"limit", pagination.Limit,
			"offset", pagination.Offset,
			"error", err,
		)
		WriteError(w, http.StatusInternalServerError, "failed to fetch closed positions", "INTERNAL_ERROR")
		return
	}

	WriteJSON(w, http.StatusOK, PaginatedResponse{
		Items:   positions,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Count:   len(positions),
		HasMore: len(positions) == pagination.Limit,
	})
}

// computeUnrealizedPnl calculates unrealized P&L based on position side.
// LONG:  (currentPrice - entryPrice) * quantity
// SHORT: (entryPrice - currentPrice) * quantity
func computeUnrealizedPnl(side string, entryPrice, currentPrice, quantity decimal.Decimal) decimal.Decimal {
	switch side {
	case "LONG", "BUY":
		return currentPrice.Sub(entryPrice).Mul(quantity)
	case "SHORT", "SELL":
		return entryPrice.Sub(currentPrice).Mul(quantity)
	default:
		return decimal.Zero
	}
}
