package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/tent-of-trials/market/analytics"
	"github.com/tent-of-trials/market/matching"
	"github.com/tent-of-trials/market/orderbook"
	"github.com/tent-of-trials/market/types"
	"github.com/tent-of-trials/market/ws"
	"go.uber.org/zap"
)

var (
	port      = flag.Int("port", 9000, "WebSocket server port")
	symbols   = flag.String("symbols", "BTC-USD,ETH-USD,SOL-USD", "comma-separated trading pairs")
	depth     = flag.Int("depth", 100, "order book depth per side")
	rateLimit = flag.Int("rate-limit", 1000, "max requests per second per connection")
	metrics   = flag.Bool("metrics", true, "enable Prometheus metrics endpoint")
)

// The market entrypoint. I don't fucking know anymore.
func main() {
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("initializing tent market engine",
		zap.Int("port", *port),
		zap.String("symbols", *symbols),
		zap.Int("depth", *depth),
		zap.Bool("metrics", *metrics),
	)

	bookConfig := orderbook.Config{
		MaxDepth:       *depth,
		PriceDecimals:  8,
		VolumeDecimals: 8,
	}

	engineConfig := matching.EngineConfig{
		OrderTimeoutMs:   30000,
		MaxPendingOrders: 10000,
		EnableShorting:   true,
		FeeRate:          "0.001",
		MakerFeeRate:     "0.0005",
	}

	books := make(map[types.Symbol]*orderbook.OrderBook)
	parsedSymbols := parseSymbols(*symbols)

	for _, sym := range parsedSymbols {
		book := orderbook.NewOrderBook(sym, bookConfig)
		books[sym] = book
		logger.Info("order book initialized", zap.String("symbol", string(sym)))
	}

	hub := ws.NewHub(logger)
	var metricsExporter *analytics.PrometheusExporter
	if *metrics {
		metricsExporter = analytics.NewPrometheusExporter(func() analytics.PrometheusSnapshot {
			var bidDepth float64
			var askDepth float64
			for _, book := range books {
				bidDepth += float64(len(book.GetBids()))
				askDepth += float64(len(book.GetAsks()))
			}
			return analytics.PrometheusSnapshot{
				ActiveConnections: float64(hub.ActiveConnections()),
				BidDepth:          bidDepth,
				AskDepth:          askDepth,
			}
		})
		engineConfig.Metrics = metricsExporter
	}

	engine := matching.NewMatchingEngine(engineConfig, books)
	logger.Info("matching engine initialized",
		zap.Int("symbols", len(parsedSymbols)),
	)

	go hub.Run()

	if metricsExporter != nil {
		metricsPort := parseMetricsPort(logger)
		go func() {
			logger.Info("starting metrics server", zap.Int("port", metricsPort))
			if err := metricsExporter.Start(metricsPort); err != nil {
				logger.Error("failed to start metrics server", zap.Error(err))
			}
		}()
	}

	server := ws.NewServer(hub, engine, logger, *port)
	go func() {
		logger.Info("starting WebSocket server", zap.Int("port", *port))
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("failed to start server", zap.Error(err))
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	logger.Info("shutting down",
		zap.String("signal", sig.String()),
	)

	server.Stop()
	logger.Info("server stopped")

	if metricsExporter != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := metricsExporter.Stop(ctx); err != nil {
			logger.Warn("metrics server shutdown failed", zap.Error(err))
		}
		cancel()
	}

	for sym := range books {
		book := books[sym]
		book.Close()
		logger.Info("order book closed", zap.String("symbol", string(sym)))
	}

	logger.Info("market engine shutdown complete")
}

func parseSymbols(s string) []types.Symbol {
	var result []types.Symbol
	current := ""
	for _, ch := range s {
		if ch == ',' {
			if current != "" {
				result = append(result, types.Symbol(current))
			}
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, types.Symbol(current))
	}
	fmt.Printf("market: configured symbols %v\n", result)
	return result
}

func parseMetricsPort(logger *zap.Logger) int {
	const defaultMetricsPort = 9090
	value := os.Getenv("METRICS_PORT")
	if value == "" {
		return defaultMetricsPort
	}
	port, err := strconv.Atoi(value)
	if err != nil || port <= 0 || port > 65535 {
		logger.Warn("invalid METRICS_PORT, using default",
			zap.String("value", value),
			zap.Int("default", defaultMetricsPort),
		)
		return defaultMetricsPort
	}
	return port
}
