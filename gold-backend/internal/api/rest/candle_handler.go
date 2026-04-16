package rest

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
)

type candleHandler struct {
	candleRepository    postgres.CandleRepository
	indicatorRepository postgres.IndicatorRepository
	logger              *slog.Logger
}

func newCandleHandler(deps HandlerDependencies) *candleHandler {
	return &candleHandler{
		candleRepository:    deps.CandleRepository,
		indicatorRepository: deps.IndicatorRepository,
		logger:              deps.Logger,
	}
}

// CandleResponse combines a Candle with its matching Indicator values.
type CandleResponse struct {
	domain.Candle
	Indicator *domain.Indicator `json:"indicator,omitempty"`
}

// handleListCandles handles GET /api/v1/candles.
// Required query params: symbol, interval.
// Optional: from (RFC3339), to (RFC3339), limit, offset.
// Defaults to the last 7 days when from/to are omitted.
func (h *candleHandler) handleListCandles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	symbol := q.Get("symbol")
	if symbol == "" {
		WriteError(w, http.StatusBadRequest, "query parameter 'symbol' is required", "MISSING_PARAM")
		return
	}

	interval := q.Get("interval")
	if interval == "" {
		WriteError(w, http.StatusBadRequest, "query parameter 'interval' is required", "MISSING_PARAM")
		return
	}

	now := time.Now().UTC()
	from := now.AddDate(0, 0, -7)
	to := now

	if rawFrom := q.Get("from"); rawFrom != "" {
		parsed, err := time.Parse(time.RFC3339, rawFrom)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "'from' must be a valid RFC3339 timestamp", "INVALID_PARAM")
			return
		}
		from = parsed
	}

	if rawTo := q.Get("to"); rawTo != "" {
		parsed, err := time.Parse(time.RFC3339, rawTo)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "'to' must be a valid RFC3339 timestamp", "INVALID_PARAM")
			return
		}
		to = parsed
	}

	if from.After(to) {
		WriteError(w, http.StatusBadRequest, "'from' must be before 'to'", "INVALID_PARAM")
		return
	}

	pagination := ParsePagination(r)

	candles, err := h.candleRepository.FindCandlesByRangePaginated(ctx, symbol, interval, from, to, pagination.Limit, pagination.Offset)
	if err != nil {
		h.logger.Error("candle handler: failed to fetch candles",
			"path", r.URL.Path,
			"symbol", symbol,
			"interval", interval,
			"error", err,
		)
		WriteError(w, http.StatusInternalServerError, "failed to fetch candles", "INTERNAL_ERROR")
		return
	}

	indicators, err := h.indicatorRepository.FindIndicatorsByRange(ctx, symbol, interval, from, to)
	if err != nil {
		h.logger.Error("candle handler: failed to fetch indicators",
			"path", r.URL.Path,
			"symbol", symbol,
			"interval", interval,
			"error", err,
		)
		WriteError(w, http.StatusInternalServerError, "failed to fetch indicators", "INTERNAL_ERROR")
		return
	}

	// Build a map from indicator timestamp -> indicator for O(1) merge.
	indicatorByTimestamp := make(map[time.Time]*domain.Indicator, len(indicators))
	for i := range indicators {
		indicatorByTimestamp[indicators[i].Timestamp] = &indicators[i]
	}

	items := make([]CandleResponse, 0, len(candles))
	for _, c := range candles {
		resp := CandleResponse{Candle: c}
		if ind, ok := indicatorByTimestamp[c.OpenTime]; ok {
			resp.Indicator = ind
		}
		items = append(items, resp)
	}

	WriteJSON(w, http.StatusOK, PaginatedResponse{
		Items:   items,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Count:   len(items),
		HasMore: len(items) == pagination.Limit,
	})
}
