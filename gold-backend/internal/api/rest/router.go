package rest

import (
	"context"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
)

// PriceCache is a narrow interface for reading the latest ticker price.
// *redis.CacheClient satisfies this interface.
type PriceCache interface {
	GetTickerPrice(ctx context.Context, symbol string) (*domain.TickerPrice, error)
}

// PortfolioMetricsProvider is a narrow interface for reading current portfolio metrics.
// *portfolio.Manager satisfies this interface.
type PortfolioMetricsProvider interface {
	CurrentMetrics() domain.PortfolioMetrics
}

// HandlerDependencies aggregates all dependencies the REST handlers need.
type HandlerDependencies struct {
	CandleRepository    postgres.CandleRepository
	IndicatorRepository postgres.IndicatorRepository
	PositionRepository  postgres.PositionRepository
	OrderRepository     postgres.OrderRepository
	DecisionRepository  postgres.DecisionRepository
	Cache               PriceCache
	PortfolioManager    PortfolioMetricsProvider
	Logger              *slog.Logger
}

// RegisterRoutes mounts all REST API v1 endpoints onto the given router.
// A dedicated CORS middleware is applied to the /api/v1 group allowing the
// dashboard origin and localhost:3000.
//
// NOTE: main.go already applies a wildcard CORS middleware globally. This
// group-level middleware restricts the API routes more precisely. ICT-27
// should consolidate the two CORS policies when wiring the full app.
func RegisterRoutes(router chi.Router, deps HandlerDependencies, dashboardOrigin string) {
	candleHandler := newCandleHandler(deps)
	positionHandler := newPositionHandler(deps)
	tradeHandler := newTradeHandler(deps)
	metricsHandler := newMetricsHandler(deps)
	decisionHandler := newDecisionHandler(deps)

	router.Route("/api/v1", func(r chi.Router) {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins: []string{dashboardOrigin, "http://localhost:3000"},
			AllowedMethods: []string{"GET", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type"},
			MaxAge:         300,
		}))

		r.Get("/candles", candleHandler.handleListCandles)
		r.Get("/positions", positionHandler.handleListOpenPositions)
		r.Get("/positions/history", positionHandler.handleListClosedPositions)
		r.Get("/trades", tradeHandler.handleListTrades)
		r.Get("/metrics", metricsHandler.handleGetMetrics)
		r.Get("/decisions", decisionHandler.handleListDecisions)
	})
}
