package analytics

import (
	"net/http"
	"os"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

var (
	// OrdersTotal counts all orders by type and side.
	OrdersTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "market_orders_total",
			Help: "Total number of orders received",
		},
		[]string{"type", "side"},
	)

	// TradesTotal counts all executed trades.
	TradesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "market_trades_total",
			Help: "Total number of trades executed",
		},
	)

	// ActiveConnections tracks current WebSocket connections.
	ActiveConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "market_active_connections",
			Help: "Number of active WebSocket connections",
		},
	)

	// OrderBookDepth tracks order book depth for bids and asks.
	OrderBookDepth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "market_orderbook_depth",
			Help: "Current order book depth",
		},
		[]string{"side"},
	)

	// MatchingLatency tracks order matching latency.
	MatchingLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "market_matching_latency_seconds",
			Help:    "Order matching latency in seconds",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
		},
	)
)

func init() {
	prometheus.MustRegister(
		OrdersTotal,
		TradesTotal,
		ActiveConnections,
		OrderBookDepth,
		MatchingLatency,
	)
}

// StartMetricsServer starts the Prometheus metrics HTTP server on the given port.
// Returns immediately; the server runs in a goroutine.
func StartMetricsServer(port int, logger *zap.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	addr := ":" + strconv.Itoa(port)
	logger.Info("starting metrics server", zap.String("addr", addr))

	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			logger.Fatal("metrics server failed", zap.Error(err))
		}
	}()
}

// GetMetricsPort returns the port from METRICS_PORT env var, or the default.
func GetMetricsPort() int {
	if v := os.Getenv("METRICS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			return port
		}
	}
	return 9090
}
