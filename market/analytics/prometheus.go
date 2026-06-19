package analytics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type PrometheusSnapshot struct {
	ActiveConnections float64
	BidDepth          float64
	AskDepth          float64
}

type PrometheusExporter struct {
	registry          *prometheus.Registry
	orders            *prometheus.CounterVec
	trades            prometheus.Counter
	activeConnections prometheus.Gauge
	orderBookDepth    *prometheus.GaugeVec
	matchingLatency   prometheus.Histogram
	snapshot          func() PrometheusSnapshot
	server            *http.Server
	mu                sync.Mutex
}

func NewPrometheusExporter(snapshot func() PrometheusSnapshot) *PrometheusExporter {
	exporter := &PrometheusExporter{
		registry: prometheus.NewRegistry(),
		orders: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "market_orders_total",
			Help: "Total market orders by order type and side.",
		}, []string{"type", "side"}),
		trades: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "market_trades_total",
			Help: "Total market trades matched by the engine.",
		}),
		activeConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "market_active_connections",
			Help: "Current active market gateway WebSocket connections.",
		}),
		orderBookDepth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "market_orderbook_depth",
			Help: "Current aggregate order book depth by side.",
		}, []string{"side"}),
		matchingLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "market_matching_latency_seconds",
			Help:    "Latency for market matching engine order placement.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
		}),
		snapshot: snapshot,
	}

	exporter.registry.MustRegister(
		exporter.orders,
		exporter.trades,
		exporter.activeConnections,
		exporter.orderBookDepth,
		exporter.matchingLatency,
	)

	for _, orderType := range []string{"limit", "market", "stop_limit", "stop_market", "trailing_stop", "iceberg"} {
		for _, side := range []string{"buy", "sell"} {
			exporter.orders.WithLabelValues(orderType, side)
		}
	}
	for _, side := range []string{"bids", "asks"} {
		exporter.orderBookDepth.WithLabelValues(side)
	}

	return exporter
}

func (e *PrometheusExporter) Handler() http.Handler {
	handler := promhttp.HandlerFor(e.registry, promhttp.HandlerOpts{})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		e.syncSnapshot()
		handler.ServeHTTP(w, r)
	})
}

func (e *PrometheusExporter) Start(port int) error {
	e.mu.Lock()
	e.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           e.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	server := e.server
	e.mu.Unlock()

	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (e *PrometheusExporter) Stop(ctx context.Context) error {
	e.mu.Lock()
	server := e.server
	e.mu.Unlock()
	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
}

func (e *PrometheusExporter) RecordOrder(orderType string, side string) {
	if orderType == "" || side == "" {
		return
	}
	e.orders.WithLabelValues(orderType, side).Inc()
}

func (e *PrometheusExporter) RecordTrades(count int) {
	if count <= 0 {
		return
	}
	e.trades.Add(float64(count))
}

func (e *PrometheusExporter) ObserveMatchingLatency(duration time.Duration) {
	if duration < 0 {
		return
	}
	e.matchingLatency.Observe(duration.Seconds())
}

func (e *PrometheusExporter) syncSnapshot() {
	if e.snapshot == nil {
		return
	}
	snapshot := e.snapshot()
	e.activeConnections.Set(snapshot.ActiveConnections)
	e.orderBookDepth.WithLabelValues("bids").Set(snapshot.BidDepth)
	e.orderBookDepth.WithLabelValues("asks").Set(snapshot.AskDepth)
}
