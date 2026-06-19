package metrics

import (
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tent-of-trials/market/matching"
	"github.com/tent-of-trials/market/orderbook"
	"github.com/tent-of-trials/market/types"
)

const defaultVersion = "0.1.0"

// Exporter renders a small Prometheus-compatible snapshot for the market gateway.
// It intentionally avoids dumping environment variables or process details so the
// endpoint can be exposed to operators without leaking secrets.
type Exporter struct {
	engine            *matching.MatchingEngine
	books             map[types.Symbol]*orderbook.OrderBook
	started           time.Time
	version           string
	commit            string
	features          []string
	activeConnections func() int
}

// Config contains the runtime data needed to render market metrics.
type Config struct {
	Engine            *matching.MatchingEngine
	Books             map[types.Symbol]*orderbook.OrderBook
	Started           time.Time
	Version           string
	Commit            string
	Features          []string
	ActiveConnections func() int
}

func NewExporter(cfg Config) *Exporter {
	version := cfg.Version
	if version == "" {
		version = defaultVersion
	}
	commit := cfg.Commit
	if commit == "" {
		commit = os.Getenv("GIT_COMMIT")
	}
	if commit == "" {
		commit = "unknown"
	}
	started := cfg.Started
	if started.IsZero() {
		started = time.Now()
	}
	features := append([]string(nil), cfg.Features...)
	if len(features) == 0 {
		features = []string{"websocket", "rest", "prometheus"}
	}
	sort.Strings(features)

	return &Exporter{
		engine:            cfg.Engine,
		books:             cfg.Books,
		started:           started,
		version:           version,
		commit:            commit,
		features:          features,
		activeConnections: cfg.ActiveConnections,
	}
}

func (e *Exporter) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = w.Write([]byte(e.Render()))
	}
}

func (e *Exporter) Render() string {
	var b strings.Builder

	b.WriteString("# HELP market_gateway_info Static build and feature information for the market gateway.\n")
	b.WriteString("# TYPE market_gateway_info gauge\n")
	fmt.Fprintf(&b, "market_gateway_info{version=%q,commit=%q,features=%q} 1\n", e.version, e.commit, strings.Join(e.features, ","))

	b.WriteString("# HELP market_gateway_uptime_seconds Seconds since the market gateway started.\n")
	b.WriteString("# TYPE market_gateway_uptime_seconds gauge\n")
	fmt.Fprintf(&b, "market_gateway_uptime_seconds %.0f\n", time.Since(e.started).Seconds())

	orderCounts := map[string]int64{}
	latency := matching.LatencyMetrics{}
	tradeCount := int64(0)
	if e.engine != nil {
		orderCounts = e.engine.GetOrderMetrics()
		latency = e.engine.GetMatchingLatencyMetrics()
		tradeCount = e.engine.GetTradeCount()
	}

	b.WriteString("# HELP market_orders_total Total orders submitted to the matching engine.\n")
	b.WriteString("# TYPE market_orders_total counter\n")
	for _, key := range sortedKeys(orderCounts) {
		parts := strings.SplitN(key, ":", 2)
		orderType, side := parts[0], "unknown"
		if len(parts) == 2 {
			side = parts[1]
		}
		fmt.Fprintf(&b, "market_orders_total{type=%q,side=%q} %d\n", orderType, side, orderCounts[key])
	}
	if len(orderCounts) == 0 {
		fmt.Fprintf(&b, "market_orders_total{type=%q,side=%q} 0\n", "unknown", "unknown")
	}

	b.WriteString("# HELP market_trades_total Total trades recorded by the matching engine.\n")
	b.WriteString("# TYPE market_trades_total counter\n")
	fmt.Fprintf(&b, "market_trades_total %d\n", tradeCount)

	b.WriteString("# HELP market_active_connections Current active WebSocket client connections.\n")
	b.WriteString("# TYPE market_active_connections gauge\n")
	fmt.Fprintf(&b, "market_active_connections %d\n", e.activeConnectionCount())

	b.WriteString("# HELP market_orderbook_depth Number of price levels currently held per symbol and side.\n")
	b.WriteString("# TYPE market_orderbook_depth gauge\n")
	for _, symbol := range sortedSymbols(e.books) {
		book := e.books[symbol]
		if book == nil {
			continue
		}
		fmt.Fprintf(&b, "market_orderbook_depth{symbol=%q,side=%q} %d\n", string(symbol), "bid", len(book.GetBids()))
		fmt.Fprintf(&b, "market_orderbook_depth{symbol=%q,side=%q} %d\n", string(symbol), "ask", len(book.GetAsks()))
	}

	b.WriteString("# HELP market_matching_latency_seconds Matching engine order placement latency in seconds.\n")
	b.WriteString("# TYPE market_matching_latency_seconds summary\n")
	fmt.Fprintf(&b, "market_matching_latency_seconds_count %d\n", latency.Count)
	fmt.Fprintf(&b, "market_matching_latency_seconds_sum %.9f\n", latency.SumSeconds)

	return b.String()
}

func (e *Exporter) activeConnectionCount() int {
	if e.activeConnections == nil {
		return 0
	}
	return e.activeConnections()
}

func sortedKeys(m map[string]int64) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedSymbols(books map[types.Symbol]*orderbook.OrderBook) []types.Symbol {
	symbols := make([]types.Symbol, 0, len(books))
	for symbol := range books {
		symbols = append(symbols, symbol)
	}
	sort.Slice(symbols, func(i, j int) bool { return symbols[i] < symbols[j] })
	return symbols
}
