package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"

	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/indicator"
	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/pattern/candlestick"
	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/pattern/chart"
	"github.com/mmendesx/goldagent/gold-backend/internal/analysis/sentiment"
	"github.com/mmendesx/goldagent/gold-backend/internal/api/rest"
	websockethub "github.com/mmendesx/goldagent/gold-backend/internal/api/websocket"
	"github.com/mmendesx/goldagent/gold-backend/internal/config"
	"github.com/mmendesx/goldagent/gold-backend/internal/domain"
	"github.com/mmendesx/goldagent/gold-backend/internal/engine"
	"github.com/mmendesx/goldagent/gold-backend/internal/exchange/binance"
	"github.com/mmendesx/goldagent/gold-backend/internal/exchange/polymarket"
	"github.com/mmendesx/goldagent/gold-backend/internal/execution"
	"github.com/mmendesx/goldagent/gold-backend/internal/market/candle"
	"github.com/mmendesx/goldagent/gold-backend/internal/portfolio"
	"github.com/mmendesx/goldagent/gold-backend/internal/storage/postgres"
	redisstore "github.com/mmendesx/goldagent/gold-backend/internal/storage/redis"
)

const (
	// historyLimit is the number of recent candles fetched per orchestrator cycle.
	// 300 candles covers the 200-period EMA warm-up period with buffer.
	historyLimit = 300

	// sentimentLookback is the window for aggregating sentiment scores.
	sentimentLookback = 24 * time.Hour

	// shutdownTimeout is the maximum time allowed for graceful shutdown.
	shutdownTimeout = 30 * time.Second

	// closedCandleFanOutBuffer is the per-subscriber channel buffer for fanned-out candles.
	closedCandleFanOutBuffer = 256
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.LoadConfiguration()
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("configuration loaded",
		"dry_run", cfg.IsDryRunEnabled,
		"symbols", cfg.Symbols,
		"http_port", cfg.HttpPort,
	)

	// Root context cancelled by OS signal. The errgroup derives from this.
	rootCtx, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		sig := <-quit
		slog.Info("shutdown signal received", "signal", sig.String())
		cancelRoot()
	}()

	// --- Infrastructure ---

	pool, err := postgres.NewConnectionPool(rootCtx, cfg.DatabaseUrl)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("postgres connected")

	cacheClient, err := redisstore.NewCacheClient(cfg.RedisUrl)
	if err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}
	defer cacheClient.Close()

	if err := cacheClient.Ping(rootCtx); err != nil {
		slog.Error("redis ping failed", "error", err)
		os.Exit(1)
	}
	slog.Info("redis connected")

	// --- Repositories ---

	candleRepo := postgres.NewCandleRepository(pool)
	indicatorRepo := postgres.NewIndicatorRepository(pool)
	positionRepo := postgres.NewPositionRepository(pool)
	orderRepo := postgres.NewOrderRepository(pool)
	decisionRepo := postgres.NewDecisionRepository(pool)
	portfolioRepo := postgres.NewPortfolioRepository(pool)
	newsRepo := postgres.NewNewsRepository(pool)
	sentimentRepo := postgres.NewSentimentRepository(pool)

	// --- Portfolio Manager ---

	portfolioManager := portfolio.NewManager(portfolio.ManagerConfig{
		PortfolioRepository: portfolioRepo,
		PositionRepository:  positionRepo,
		Cache:               cacheClient,
		InitialBalance:      decimal.NewFromFloat(10000.0),
		MaxDrawdownPercent:  decimal.NewFromFloat(cfg.MaxDrawdownPercent),
		Logger:              logger,
	})

	if err := portfolioManager.Bootstrap(rootCtx); err != nil {
		slog.Error("portfolio manager bootstrap failed", "error", err)
		os.Exit(1)
	}
	slog.Info("portfolio manager bootstrapped")

	// --- Exchange Stream Clients ---

	streamClient := binance.NewStreamClient(binance.StreamClientConfig{
		BaseUrl:  cfg.BinanceWebSocketStreamUrl,
		Symbols:  cfg.Symbols,
		Interval: cfg.DefaultInterval,
	}, logger)

	// --- Candle Aggregator ---

	aggregator := candle.NewAggregator(candle.AggregatorConfig{
		InputChannel:       streamClient.CandleChannel(),
		PostgresRepository: candleRepo,
		RedisCache:         cacheClient,
		Logger:             logger,
	})

	// Fan out closed candles to three subscribers:
	//   1. Indicator computer
	//   2. Decision orchestrator
	//   3. WebSocket hub
	indicatorComputerCh := make(chan domain.Candle, closedCandleFanOutBuffer)
	orchestratorCh := make(chan domain.Candle, closedCandleFanOutBuffer)
	wsCandleCh := make(chan domain.Candle, closedCandleFanOutBuffer)

	// --- Indicator Computer ---

	indicatorComputer := indicator.NewComputer(indicator.ComputerConfig{
		InputChannel:        indicatorComputerCh,
		CandleRepository:    candleRepo,
		IndicatorRepository: indicatorRepo,
		RsiPeriod:           cfg.RsiPeriod,
		MacdFast:            cfg.MacdFastPeriod,
		MacdSlow:            cfg.MacdSlowPeriod,
		MacdSignalPeriod:    cfg.MacdSignalPeriod,
		BollingerPeriod:     cfg.BollingerPeriod,
		BollingerStdDev:     cfg.BollingerStandardDeviation,
		EmaPeriods:          cfg.EmaPeriods,
		AtrPeriod:           cfg.AtrPeriod,
		HistoryLimit:        historyLimit,
		Logger:              logger,
	})

	// --- Pattern Detectors ---

	candlestickDetector := candlestick.NewDetector()
	chartAnalyzer := chart.NewAnalyzer(chart.AnalyzerConfig{})

	// --- Sentiment Coordinator (conditional) ---

	var sentimentCoordinator *sentiment.Coordinator
	if cfg.CryptoPanicApiKey != "" && cfg.AnthropicApiKey != "" {
		fetcher := sentiment.NewNewsFetcher(sentiment.NewsFetcherConfig{
			CryptoPanicApiKey: cfg.CryptoPanicApiKey,
		})
		scorer := sentiment.NewScorer(sentiment.ScorerConfig{
			AnthropicApiKey: cfg.AnthropicApiKey,
		})
		sentimentCoordinator = sentiment.NewCoordinator(sentiment.CoordinatorConfig{
			Fetcher:             fetcher,
			Scorer:              scorer,
			NewsRepository:      newsRepo,
			SentimentRepository: sentimentRepo,
			Symbols:             cfg.Symbols,
			PollInterval:        5 * time.Minute,
			Logger:              logger,
		})
		slog.Info("sentiment coordinator enabled")
	} else {
		slog.Info("sentiment coordinator disabled: CRYPTOPANIC_API_KEY or ANTHROPIC_API_KEY not set")
	}

	// --- Decision Engine ---

	decisionEngine := engine.NewDecisionEngine(engine.DecisionEngineConfig{
		DecisionRepository:        decisionRepo,
		PositionRepository:        positionRepo,
		PortfolioCache:            cacheClient,
		ConfidenceThreshold:       cfg.ConfidenceThreshold,
		MaxOpenPositions:          cfg.MaxOpenPositions,
		MaxDrawdownPercent:        decimal.NewFromFloat(cfg.MaxDrawdownPercent),
		SentimentWeight:           cfg.SentimentWeight,
		PositionSizePercent:       decimal.NewFromFloat(cfg.MaxPositionSizePercent),
		TakeProfitAtrMultiplier:   decimal.NewFromFloat(2.0),
		StopLossAtrMultiplier:     decimal.NewFromFloat(1.0),
		TrailingStopAtrMultiplier: decimal.NewFromFloat(cfg.TrailingStopAtrMultiplier),
		InitialBalance:            decimal.NewFromFloat(10000.0),
		IsDryRunEnabled:           cfg.IsDryRunEnabled,
		Logger:                    logger,
	})

	// --- Executor and Position Monitor (conditional) ---

	var (
		orderClient     *execution.BinanceOrderClient
		executor        *execution.Executor
		positionMonitor *execution.PositionMonitor

		// Position fan-out channels: executor emits to one channel; we fan out to
		// both the position monitor and the WebSocket hub.
		positionOpenedForHub       <-chan domain.Position
		positionOpenedFanOutSource <-chan domain.Position
		positionOpenedFanOutSink1  chan domain.Position
		positionOpenedFanOutSink2  chan domain.Position
	)

	hasApiKeys := cfg.BinanceApiKey != "" && cfg.BinanceApiSecret != ""
	if hasApiKeys && !cfg.IsDryRunEnabled {
		orderClient = execution.NewBinanceOrderClient(execution.BinanceOrderClientConfig{
			WebSocketApiUrl: cfg.BinanceWebSocketApiUrl,
			ApiKey:          cfg.BinanceApiKey,
			ApiSecret:       cfg.BinanceApiSecret,
			Logger:          logger,
		})

		if err := orderClient.Connect(rootCtx); err != nil {
			slog.Error("failed to connect binance order client", "error", err)
			os.Exit(1)
		}
		slog.Info("binance order client connected")

		executor = execution.NewExecutor(execution.ExecutorConfig{
			TradeIntentChannel: decisionEngine.TradeIntentChannel(),
			OrderClient:        orderClient,
			OrderRepository:    orderRepo,
			PositionRepository: positionRepo,
			DecisionRepository: decisionRepo,
			Logger:             logger,
		})

		// Fan out position-opened events to both the position monitor and the WebSocket hub.
		// A single channel has one reader — without fan-out the monitor would miss events
		// consumed by the hub and silently fail to track positions for TP/SL.
		const positionFanOutBuffer = 64
		positionForMonitorCh := make(chan domain.Position, positionFanOutBuffer)
		positionForHubCh := make(chan domain.Position, positionFanOutBuffer)

		positionMonitor = execution.NewPositionMonitor(execution.PositionMonitorConfig{
			PositionOpenedChannel: positionForMonitorCh,
			PriceReader:           cacheClient,
			OrderClient:           orderClient,
			OrderRepository:       orderRepo,
			PositionRepository:    positionRepo,
			PortfolioManager:      portfolioManager,
			Logger:                logger,
		})

		if err := positionMonitor.LoadOpenPositionsFromDatabase(rootCtx); err != nil {
			slog.Error("failed to load open positions for monitor", "error", err)
			os.Exit(1)
		}

		// Register the position fan-out in the errgroup below; store channels for hub wiring.
		positionOpenedForHub = positionForHubCh
		positionOpenedFanOutSource = executor.PositionOpenedChannel()
		positionOpenedFanOutSink1 = positionForMonitorCh
		positionOpenedFanOutSink2 = positionForHubCh

		slog.Info("executor and position monitor enabled")
	} else {
		if cfg.IsDryRunEnabled {
			slog.Info("executor disabled: dry-run mode enabled")
		} else {
			slog.Info("executor disabled: Binance API keys not configured")
		}
	}

	// --- Polymarket Client (conditional) ---

	var polymarketClient *polymarket.RealtimeClient
	if cfg.PolymarketApiKey != "" {
		polymarketClient = polymarket.NewRealtimeClient(polymarket.RealtimeClientConfig{
			BaseUrl:                 "wss://ws-live-data.polymarket.com",
			ApiKey:                  cfg.PolymarketApiKey,
			ApiSecret:               cfg.PolymarketApiSecret,
			ApiPassphrase:           cfg.PolymarketApiPassphrase,
			SubscribeToActivity:     true,
			SubscribeToCryptoPrices: true,
		}, logger)
		slog.Info("polymarket client enabled")
	} else {
		slog.Info("polymarket client disabled: POLYMARKET_API_KEY not set")
	}

	// --- WebSocket Hub ---

	hub := websockethub.NewHub(websockethub.HubConfig{
		CandleChannel:         wsCandleCh,
		PositionOpenedChannel: positionOpenedForHub, // nil when executor is disabled — hub handles nil
		MetricsProvider:       portfolioManager,
		// DecisionChannel: nil — engine does not expose a decision channel (only TradeIntentChannel)
		// PositionClosedChannel: nil — position monitor does not expose a closed channel
		Logger: logger,
	})

	// --- HTTP Router ---

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	router.Get("/health", handleHealth)

	rest.RegisterRoutes(router, rest.HandlerDependencies{
		CandleRepository:    candleRepo,
		IndicatorRepository: indicatorRepo,
		PositionRepository:  positionRepo,
		OrderRepository:     orderRepo,
		DecisionRepository:  decisionRepo,
		Cache:               cacheClient,
		PortfolioManager:    portfolioManager,
		Logger:              logger,
	}, "http://localhost:3000")

	router.Get("/ws/v1/stream", hub.HandleWebSocket)

	serverAddress := fmt.Sprintf(":%d", cfg.HttpPort)
	server := &http.Server{
		Addr:         serverAddress,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// --- Start all components with errgroup ---

	g, gctx := errgroup.WithContext(rootCtx)

	// Binance stream client
	g.Go(func() error {
		slog.Info("component started", "component", "binance_stream_client")
		if err := streamClient.Run(gctx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("binance stream client: %w", err)
		}
		slog.Info("component stopped", "component", "binance_stream_client")
		return nil
	})

	// Ticker-to-cache bridge: keeps redis ticker prices current for the position monitor.
	g.Go(func() error {
		slog.Info("component started", "component", "ticker_cache_bridge")
		for {
			select {
			case <-gctx.Done():
				slog.Info("component stopped", "component", "ticker_cache_bridge")
				return nil
			case ticker, ok := <-streamClient.TickerChannel():
				if !ok {
					slog.Info("component stopped", "component", "ticker_cache_bridge")
					return nil
				}
				if err := cacheClient.SetTickerPrice(gctx, ticker); err != nil {
					slog.Warn("ticker cache bridge: failed to cache ticker",
						"symbol", ticker.Symbol,
						"error", err,
					)
				}
			}
		}
	})

	// Fan-out: distributes closed-candle events to all three subscribers.
	g.Go(func() error {
		slog.Info("component started", "component", "candle_fan_out")
		fanOutCandles(gctx, aggregator.ClosedCandleChannel(), []chan<- domain.Candle{
			indicatorComputerCh,
			orchestratorCh,
			wsCandleCh,
		})
		slog.Info("component stopped", "component", "candle_fan_out")
		return nil
	})

	// Candle aggregator
	g.Go(func() error {
		slog.Info("component started", "component", "candle_aggregator")
		if err := aggregator.Run(gctx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("candle aggregator: %w", err)
		}
		slog.Info("component stopped", "component", "candle_aggregator")
		return nil
	})

	// Indicator computer
	g.Go(func() error {
		slog.Info("component started", "component", "indicator_computer")
		if err := indicatorComputer.Run(gctx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("indicator computer: %w", err)
		}
		slog.Info("component stopped", "component", "indicator_computer")
		return nil
	})

	// Decision orchestrator: reads orchestratorCh, fetches context, feeds engine.
	g.Go(func() error {
		slog.Info("component started", "component", "decision_orchestrator")
		if err := runDecisionOrchestrator(
			gctx,
			orchestratorCh,
			candleRepo,
			indicatorRepo,
			candlestickDetector,
			chartAnalyzer,
			sentimentCoordinator,
			decisionEngine,
			historyLimit,
			sentimentLookback,
			logger,
		); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("decision orchestrator: %w", err)
		}
		slog.Info("component stopped", "component", "decision_orchestrator")
		return nil
	})

	// Sentiment coordinator (conditional)
	if sentimentCoordinator != nil {
		g.Go(func() error {
			slog.Info("component started", "component", "sentiment_coordinator")
			if err := sentimentCoordinator.Run(gctx); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("sentiment coordinator: %w", err)
			}
			slog.Info("component stopped", "component", "sentiment_coordinator")
			return nil
		})
	}

	// Executor (conditional)
	if executor != nil {
		g.Go(func() error {
			slog.Info("component started", "component", "executor")
			if err := executor.Run(gctx); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("executor: %w", err)
			}
			slog.Info("component stopped", "component", "executor")
			return nil
		})
	}

	// Position fan-out: distributes opened-position events to monitor and hub (conditional).
	if positionOpenedFanOutSource != nil {
		g.Go(func() error {
			slog.Info("component started", "component", "position_fan_out")
			defer close(positionOpenedFanOutSink1)
			defer close(positionOpenedFanOutSink2)
			for {
				select {
				case <-gctx.Done():
					slog.Info("component stopped", "component", "position_fan_out")
					return nil
				case pos, ok := <-positionOpenedFanOutSource:
					if !ok {
						slog.Info("component stopped", "component", "position_fan_out")
						return nil
					}
					for _, sink := range []chan domain.Position{positionOpenedFanOutSink1, positionOpenedFanOutSink2} {
						select {
						case sink <- pos:
						default:
							slog.Warn("position fan-out: subscriber channel full, dropping event",
								"symbol", pos.Symbol,
							)
						}
					}
				}
			}
		})
	}

	// Position monitor (conditional)
	if positionMonitor != nil {
		g.Go(func() error {
			slog.Info("component started", "component", "position_monitor")
			if err := positionMonitor.Run(gctx); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("position monitor: %w", err)
			}
			slog.Info("component stopped", "component", "position_monitor")
			return nil
		})
	}

	// Polymarket client (conditional)
	if polymarketClient != nil {
		g.Go(func() error {
			slog.Info("component started", "component", "polymarket_client")
			if err := polymarketClient.Run(gctx); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("polymarket client: %w", err)
			}
			slog.Info("component stopped", "component", "polymarket_client")
			return nil
		})
	}

	// Portfolio manager snapshot loop
	g.Go(func() error {
		slog.Info("component started", "component", "portfolio_manager")
		if err := portfolioManager.Run(gctx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("portfolio manager: %w", err)
		}
		slog.Info("component stopped", "component", "portfolio_manager")
		return nil
	})

	// WebSocket hub
	g.Go(func() error {
		slog.Info("component started", "component", "websocket_hub")
		if err := hub.Run(gctx); err != nil && !errors.Is(err, context.Canceled) {
			return fmt.Errorf("websocket hub: %w", err)
		}
		slog.Info("component stopped", "component", "websocket_hub")
		return nil
	})

	// HTTP server: shuts down when gctx is cancelled (by signal or first component error).
	g.Go(func() error {
		slog.Info("server starting", "port", cfg.HttpPort)

		serverErrCh := make(chan error, 1)
		go func() {
			if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErrCh <- fmt.Errorf("http server: %w", err)
			}
			close(serverErrCh)
		}()

		select {
		case <-gctx.Done():
			// Context cancelled — shut down HTTP server gracefully.
			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				slog.Error("http server shutdown error", "error", err)
			}
			slog.Info("component stopped", "component", "http_server")
			return nil
		case err := <-serverErrCh:
			return err
		}
	})

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("application exited with error", "error", err)
		os.Exit(1)
	}

	slog.Info("application shutdown complete")
}

// handleHealth returns a 200 OK with a JSON status body.
func handleHealth(responseWriter http.ResponseWriter, _ *http.Request) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(responseWriter).Encode(map[string]string{"status": "ok"}); err != nil {
		slog.Error("failed to write health response", "error", err)
	}
}

// fanOutCandles distributes closed-candle events to multiple subscribers.
// Closes all subscriber channels when the source closes or ctx is cancelled.
func fanOutCandles(ctx context.Context, source <-chan domain.Candle, subscribers []chan<- domain.Candle) {
	defer func() {
		for _, sub := range subscribers {
			close(sub)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case candle, ok := <-source:
			if !ok {
				return
			}
			for _, sub := range subscribers {
				select {
				case sub <- candle:
				default:
					slog.Warn("candle fan-out: subscriber channel full, dropping candle",
						"symbol", candle.Symbol,
						"interval", candle.Interval,
					)
				}
			}
		}
	}
}

// runDecisionOrchestrator subscribes to closed candles, fetches the latest indicator,
// runs pattern detection, and feeds the decision engine.
// This composes the analysis pipeline that the decision engine consumes.
func runDecisionOrchestrator(
	ctx context.Context,
	closedCandles <-chan domain.Candle,
	candleRepository postgres.CandleRepository,
	indicatorRepository postgres.IndicatorRepository,
	candlestickDetector *candlestick.Detector,
	chartAnalyzer *chart.Analyzer,
	sentimentCoordinator *sentiment.Coordinator,
	decisionEngine *engine.DecisionEngine,
	historyLimit int,
	sentimentLookback time.Duration,
	logger *slog.Logger,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case candle, ok := <-closedCandles:
			if !ok {
				return nil
			}

			history, err := candleRepository.FindLatestCandles(ctx, candle.Symbol, candle.Interval, historyLimit)
			if err != nil {
				logger.Error("orchestrator: fetch candles failed",
					"symbol", candle.Symbol,
					"error", err,
				)
				continue
			}

			// FindLatestCandles returns newest-first; reverse to chronological order.
			reverseCandles(history)

			// The indicator computer may still be mid-flight for this same candle.
			// Retry once with a short delay to absorb the race.
			var ind *domain.Indicator
			for attempt := 0; attempt < 2; attempt++ {
				ind, err = indicatorRepository.FindLatestIndicator(ctx, candle.Symbol, candle.Interval)
				if err != nil {
					logger.Error("orchestrator: fetch indicator failed",
						"symbol", candle.Symbol,
						"attempt", attempt+1,
						"error", err,
					)
					break
				}
				if ind != nil {
					break
				}
				if attempt == 0 {
					// Indicator not persisted yet — give the computer 100ms.
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(100 * time.Millisecond):
					}
				}
			}

			if ind == nil {
				logger.Warn("orchestrator: no indicator available, skipping decision cycle",
					"symbol", candle.Symbol,
					"interval", candle.Interval,
				)
				continue
			}

			patterns := candlestickDetector.DetectPatterns(history)
			chartResult := chartAnalyzer.Analyze(history)

			var sentimentScore decimal.Decimal
			if sentimentCoordinator != nil {
				sentimentScore, err = sentimentCoordinator.LatestSentimentForSymbol(ctx, candle.Symbol, sentimentLookback)
				if err != nil {
					logger.Warn("orchestrator: sentiment fetch failed",
						"symbol", candle.Symbol,
						"error", err,
					)
					sentimentScore = decimal.Zero
				}
			}

			input := engine.AnalysisInput{
				Symbol:          candle.Symbol,
				LatestIndicator: *ind,
				LatestCandle:    candle,
				Patterns:        patterns,
				ChartAnalysis:   chartResult,
				Sentiment:       sentimentScore,
			}

			if err := decisionEngine.ProcessAnalysisInput(ctx, input); err != nil {
				logger.Error("orchestrator: decision engine processing failed",
					"symbol", candle.Symbol,
					"error", err,
				)
			}
		}
	}
}

// reverseCandles reverses a slice of candles in-place.
func reverseCandles(candles []domain.Candle) {
	for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
		candles[i], candles[j] = candles[j], candles[i]
	}
}
