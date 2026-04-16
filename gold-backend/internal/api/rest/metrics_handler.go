package rest

import (
	"log/slog"
	"net/http"
)

type metricsHandler struct {
	portfolioManager PortfolioMetricsProvider
	logger           *slog.Logger
}

func newMetricsHandler(deps HandlerDependencies) *metricsHandler {
	return &metricsHandler{
		portfolioManager: deps.PortfolioManager,
		logger:           deps.Logger,
	}
}

// handleGetMetrics handles GET /api/v1/metrics.
// Returns the current in-memory portfolio metrics snapshot.
func (h *metricsHandler) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := h.portfolioManager.CurrentMetrics()
	WriteJSON(w, http.StatusOK, metrics)
}
