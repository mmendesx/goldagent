package rest

import (
	"log/slog"
	"net/http"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
)

type decisionHandler struct {
	decisionRepository postgres.DecisionRepository
	logger             *slog.Logger
}

func newDecisionHandler(deps HandlerDependencies) *decisionHandler {
	return &decisionHandler{
		decisionRepository: deps.DecisionRepository,
		logger:             deps.Logger,
	}
}

// handleListDecisions handles GET /api/v1/decisions.
// Optional query param: symbol — filters results to that symbol.
// Supports pagination via limit and offset.
func (h *decisionHandler) handleListDecisions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pagination := ParsePagination(r)
	symbol := r.URL.Query().Get("symbol")

	var decisions []domain.Decision
	var err error

	if symbol != "" {
		decisions, err = h.decisionRepository.FindDecisionsBySymbol(ctx, symbol, pagination.Limit, pagination.Offset)
	} else {
		decisions, err = h.decisionRepository.FindRecentDecisions(ctx, pagination.Limit, pagination.Offset)
	}

	if err != nil {
		h.logger.Error("decision handler: failed to fetch decisions",
			"path", r.URL.Path,
			"symbol", symbol,
			"limit", pagination.Limit,
			"offset", pagination.Offset,
			"error", err,
		)
		WriteError(w, http.StatusInternalServerError, "failed to fetch decisions", "INTERNAL_ERROR")
		return
	}

	// Ensure JSON serializes as [] not null for empty results.
	if decisions == nil {
		decisions = []domain.Decision{}
	}

	WriteJSON(w, http.StatusOK, PaginatedResponse{
		Items:   decisions,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Count:   len(decisions),
		HasMore: len(decisions) == pagination.Limit,
	})
}
