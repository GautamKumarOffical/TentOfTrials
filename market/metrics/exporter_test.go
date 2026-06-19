package metrics

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/tent-of-trials/market/matching"
	"github.com/tent-of-trials/market/orderbook"
	"github.com/tent-of-trials/market/types"
)

func TestExporterRenderIncludesRequiredPrometheusMetrics(t *testing.T) {
	book := orderbook.NewOrderBook(types.Symbol("BTC-USD"), orderbook.Config{MaxDepth: 10})
	_, err := book.AddOrder(&types.Order{
		Symbol:       types.Symbol("BTC-USD"),
		Side:         types.Buy,
		Type:         types.Limit,
		Price:        decimal.NewFromInt(50000),
		Quantity:     decimal.NewFromInt(1),
		RemainingQty: decimal.NewFromInt(1),
	})
	if err != nil {
		t.Fatalf("seed order book: %v", err)
	}

	books := map[types.Symbol]*orderbook.OrderBook{types.Symbol("BTC-USD"): book}
	engine := matching.NewMatchingEngine(matching.EngineConfig{EnableShorting: true}, books)
	_, err = engine.PlaceOrder(&types.Order{
		Symbol:       types.Symbol("BTC-USD"),
		Side:         types.Sell,
		Type:         types.Market,
		Price:        decimal.NewFromInt(50001),
		Quantity:     decimal.NewFromInt(1),
		RemainingQty: decimal.NewFromInt(1),
	})
	if err != nil {
		t.Fatalf("place order: %v", err)
	}

	exporter := NewExporter(Config{
		Engine:            engine,
		Books:             books,
		Started:           time.Now().Add(-10 * time.Second),
		Version:           "test-version",
		Commit:            "test-commit",
		Features:          []string{"prometheus", "websocket"},
		ActiveConnections: func() int { return 3 },
	})

	body := exporter.Render()
	for _, want := range []string{
		`market_gateway_info{version="test-version",commit="test-commit",features="prometheus,websocket"} 1`,
		`market_gateway_uptime_seconds`,
		`market_orders_total{type="market",side="sell"} 1`,
		`market_trades_total`,
		`market_active_connections 3`,
		`market_orderbook_depth{symbol="BTC-USD",side="bid"} 1`,
		`market_orderbook_depth{symbol="BTC-USD",side="ask"} 1`,
		`market_matching_latency_seconds_count 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics output missing %q\n%s", want, body)
		}
	}
}
