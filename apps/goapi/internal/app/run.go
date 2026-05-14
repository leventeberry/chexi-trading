package app

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	openapi "goapi/api/openapi"
	"goapi/container"
	"goapi/initializers"
	"goapi/internal/events"
	authinfra "goapi/internal/infra/auth"
	"goapi/internal/marketdata/coinbase"
	"goapi/internal/marketdata/state"
	"goapi/internal/queue"
	httpmiddleware "goapi/internal/transport/http/middleware"
	httpserver "goapi/internal/transport/httpserver"
	"goapi/logger"
)

// Run bootstraps dependencies and starts the HTTP server.
func Run() {
	host := os.Getenv("SWAGGER_HOST")
	if host == "" {
		host = "localhost:8080"
	}
	openapi.SwaggerInfo.Host = host
	openapi.SwaggerInfo.BasePath = "/"
	openapi.SwaggerInfo.Schemes = []string{"http", "https"}

	deps := initializers.Bootstrap()

	jwtMgr := authinfra.NewManager(deps.Config.JWT.Secret, deps.Config.JWT.AccessTokenMinutes)
	recorder := events.NewPostgresRecorder(deps.DB)
	webhookSync := queue.WebhookDeliverWithoutWorker(deps.Config, deps.RedisClient)
	tickerStore := state.New()
	appContainer := container.NewContainer(deps.DB, deps.Cache, jwtMgr, recorder, deps.Config, deps.QueueEnqueue, deps.RedisQueue, webhookSync, tickerStore)

	workerCtx, workerCancel := context.WithCancel(context.Background())

	coinbaseCtx, coinbaseCancel := context.WithCancel(context.Background())
	var coinbaseWG sync.WaitGroup
	if deps.Config.CoinbaseExchangeWS.Enabled {
		logger.Log.Info().
			Str("component", "coinbase_ws").
			Str("url", deps.Config.CoinbaseExchangeWS.URL).
			Int("product_count", len(deps.Config.CoinbaseExchangeWS.Products)).
			Strs("products", deps.Config.CoinbaseExchangeWS.Products).
			Msg("coinbase public websocket ticker ingestion enabled")
	} else {
		logger.Log.Info().
			Str("component", "coinbase_ws").
			Msg("coinbase public websocket ticker ingestion disabled (set COINBASE_WS_ENABLED=true to enable)")
	}
	if deps.Config.CoinbaseExchangeWS.Enabled {
		coinbaseWG.Add(1)
		go func() {
			defer coinbaseWG.Done()
			cfg := coinbase.DefaultTickerAdapterConfig(
				deps.Config.CoinbaseExchangeWS.URL,
				deps.Config.CoinbaseExchangeWS.Products,
			)
			ad := coinbase.NewTickerAdapter(cfg, func(ev coinbase.MarketTickerEvent) {
				tickerStore.UpsertTicker(ev)
				logger.Log.Debug().
					Str("product_id", ev.ProductID).
					Float64("price", ev.Price).
					Msg("coinbase ticker")
			})
			ad.SetLogger(func(format string, args ...any) {
				logger.Log.Warn().Str("component", "coinbase_ws").Msgf(format, args...)
			})
			ad.Run(coinbaseCtx)
		}()
	}

	var workerWG sync.WaitGroup
	if deps.QueueWorker != nil {
		workerWG.Add(1)
		go func() {
			defer workerWG.Done()
			deps.QueueWorker.Start(workerCtx)
		}()
	}

	router := httpserver.NewEngine(deps.Cache, deps.Config, appContainer)
	httpserver.MountSwaggerRoutes(router, deps.Config, jwtMgr)

	port := deps.Config.Server.Port

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           router,
		ReadHeaderTimeout: deps.Config.Server.ReadHeaderTimeout,
		ReadTimeout:       deps.Config.Server.ReadTimeout,
		WriteTimeout:      deps.Config.Server.WriteTimeout,
		IdleTimeout:       deps.Config.Server.IdleTimeout,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Log.Info().Str("port", port).Msg("Server is running")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	<-quit
	logger.Log.Info().Msg("Shutting down server...")

	coinbaseCancel()
	coinbaseWG.Wait()

	httpmiddleware.ShutdownRateLimiter()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error().Err(err).Msg("Server forced to shutdown")
	} else {
		logger.Log.Info().Msg("Server shutdown gracefully")
	}

	workerCancel()
	workerWG.Wait()

	initializers.CloseRedis(deps.RedisClient)
	logger.Log.Info().Msg("Server exited")
}
