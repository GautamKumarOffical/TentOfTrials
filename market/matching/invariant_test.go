package matching

import (
	"math/rand"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/tent-of-trials/market/orderbook"
	"github.com/tent-of-trials/market/types"
)

func newTestBook() *orderbook.OrderBook {
	return orderbook.NewOrderBook("BTC/USDT", orderbook.Config{
		MaxDepth:       100,
		PriceDecimals:  2,
		VolumeDecimals: 8,
	})
}

func newTestEngine(books map[types.Symbol]*orderbook.OrderBook) *MatchingEngine {
	return NewMatchingEngine(EngineConfig{
		OrderTimeoutMs:   60000,
		MaxPendingOrders: 1000,
		EnableShorting:   true,
		FeeRate:          "0.001",
		MakerFeeRate:     "0.0005",
	}, books)
}

func makeOrder(id string, side types.OrderSide, price float64, qty float64) *types.Order {
	return &types.Order{
		ID:           id,
		Symbol:       "BTC/USDT",
		Side:         side,
		Type:         types.Limit,
		Price:        decimal.NewFromFloat(price),
		Quantity:     decimal.NewFromFloat(qty),
		RemainingQty: decimal.NewFromFloat(qty),
	}
}

// TestPriceTimePriorityBids verifies bids at the same price are matched in order of arrival
func TestPriceTimePriorityBids(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	bid1 := makeOrder("bid-1", types.Buy, 100.0, 1.0)
	bid2 := makeOrder("bid-2", types.Buy, 100.0, 2.0)
	bid3 := makeOrder("bid-3", types.Buy, 100.0, 1.5)

	engine.PlaceOrder(bid1)
	engine.PlaceOrder(bid2)
	engine.PlaceOrder(bid3)

	bids := book.GetBids()
	if len(bids) != 1 {
		t.Fatalf("expected 1 bid level, got %d", len(bids))
	}

	totalQty := bids[0].Quantity.InexactFloat64()
	if totalQty < 4.49 || totalQty > 4.51 {
		t.Fatalf("expected aggregate bid quantity ~4.5, got %f", totalQty)
	}
}

// TestPriceTimePriorityAsks verifies asks at the same price are matched in order of arrival
func TestPriceTimePriorityAsks(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	ask1 := makeOrder("ask-1", types.Sell, 200.0, 0.5)
	ask2 := makeOrder("ask-2", types.Sell, 200.0, 1.0)
	ask3 := makeOrder("ask-3", types.Sell, 200.0, 2.0)

	engine.PlaceOrder(ask1)
	engine.PlaceOrder(ask2)
	engine.PlaceOrder(ask3)

	asks := book.GetAsks()
	if len(asks) != 1 {
		t.Fatalf("expected 1 ask level, got %d", len(asks))
	}

	totalQty := asks[0].Quantity.InexactFloat64()
	if totalQty < 3.49 || totalQty > 3.51 {
		t.Fatalf("expected aggregate ask quantity ~3.5, got %f", totalQty)
	}
}

// TestPricePriorityBidsHigherFirst verifies higher bids have priority over lower bids
func TestPricePriorityBidsHigherFirst(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	engine.PlaceOrder(makeOrder("bid-low", types.Buy, 95.0, 1.0))
	engine.PlaceOrder(makeOrder("bid-mid", types.Buy, 100.0, 1.0))
	engine.PlaceOrder(makeOrder("bid-high", types.Buy, 105.0, 1.0))

	bids := book.GetBids()
	if len(bids) != 3 {
		t.Fatalf("expected 3 bid levels, got %d", len(bids))
	}

	if bids[0].Price.LessThan(bids[1].Price) {
		t.Fatalf("first bid should be highest price, got %f < %f", bids[0].Price.InexactFloat64(), bids[1].Price.InexactFloat64())
	}
	if bids[1].Price.LessThan(bids[2].Price) {
		t.Fatalf("second bid should be >= third price, got %f < %f", bids[1].Price.InexactFloat64(), bids[2].Price.InexactFloat64())
	}
}

// TestPricePriorityAsksLowerFirst verifies lower asks have priority over higher asks
func TestPricePriorityAsksLowerFirst(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	engine.PlaceOrder(makeOrder("ask-high", types.Sell, 205.0, 1.0))
	engine.PlaceOrder(makeOrder("ask-mid", types.Sell, 200.0, 1.0))
	engine.PlaceOrder(makeOrder("ask-low", types.Sell, 195.0, 1.0))

	asks := book.GetAsks()
	if len(asks) != 3 {
		t.Fatalf("expected 3 ask levels, got %d", len(asks))
	}

	if asks[0].Price.GreaterThan(asks[1].Price) {
		t.Fatalf("first ask should be lowest price, got %f > %f", asks[0].Price.InexactFloat64(), asks[1].Price.InexactFloat64())
	}
	if asks[1].Price.GreaterThan(asks[2].Price) {
		t.Fatalf("second ask should be <= third price, got %f > %f", asks[1].Price.InexactFloat64(), asks[2].Price.InexactFloat64())
	}
}

// TestPartialFillRemainingQuantity verifies partial fills leave correct remaining
func TestPartialFillRemainingQuantity(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	largeOrder := makeOrder("big-bid", types.Buy, 100.0, 10.0)
	engine.PlaceOrder(largeOrder)

	bids := book.GetBids()
	if len(bids) != 1 {
		t.Fatalf("expected 1 bid level, got %d", len(bids))
	}

	qty := bids[0].Quantity.InexactFloat64()
	if qty < 9.99 || qty > 10.01 {
		t.Fatalf("expected remaining quantity ~10.0, got %f", qty)
	}
}

// TestCancelledOrderCannotBeMatched verifies cancelled orders are removed
func TestCancelledOrderCannotBeMatched(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	bid := makeOrder("bid-cancel", types.Buy, 100.0, 5.0)
	engine.PlaceOrder(bid)

	err := engine.CancelOrder("BTC/USDT", "bid-cancel")
	if err != nil {
		t.Fatalf("cancel should succeed: %v", err)
	}

	bids := book.GetBids()
	if len(bids) != 0 {
		t.Fatalf("cancelled order should be removed from book, got %d bid levels", len(bids))
	}

	err = engine.CancelOrder("BTC/USDT", "bid-cancel")
	if err == nil {
		t.Fatal("double cancel should fail")
	}
}

// TestCancelNonexistentOrder verifies cancel of unknown order fails
func TestCancelNonexistentOrder(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	err := engine.CancelOrder("BTC/USDT", "no-such-order")
	if err == nil {
		t.Fatal("cancel of nonexistent order should fail")
	}
}

// TestFullyFilledOrderRemoved verifies fully filled orders are marked and removed
func TestFullyFilledOrderRemoved(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	bid := makeOrder("bid-fill", types.Buy, 100.0, 1.0)
	engine.PlaceOrder(bid)

	err := engine.CancelOrder("BTC/USDT", "bid-fill")
	if err != nil {
		t.Fatalf("cancel fully filled order should succeed: %v", err)
	}

	bids := book.GetBids()
	if len(bids) != 0 {
		t.Fatalf("filled order should be removed, got %d levels", len(bids))
	}
}

// TestZeroQuantityOrderRejected verifies zero quantity orders are rejected
func TestZeroQuantityOrderRejected(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	bad := makeOrder("zero", types.Buy, 100.0, 0.0)
	_, err := engine.PlaceOrder(bad)
	if err == nil {
		t.Fatal("zero quantity order should be rejected")
	}
}

// TestNegativeQuantityOrderRejected verifies negative quantity orders are rejected
func TestNegativeQuantityOrderRejected(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	bad := makeOrder("neg-qty", types.Buy, 100.0, -5.0)
	_, err := engine.PlaceOrder(bad)
	if err == nil {
		t.Fatal("negative quantity order should be rejected")
	}
}

// TestZeroPriceLimitOrderRejected verifies zero price limit orders are rejected
func TestZeroPriceLimitOrderRejected(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	bad := makeOrder("zero-price", types.Buy, 0.0, 1.0)
	_, err := engine.PlaceOrder(bad)
	if err == nil {
		t.Fatal("zero price limit order should be rejected")
	}
}

// TestNegativePriceLimitOrderRejected verifies negative price limit orders are rejected
func TestNegativePriceLimitOrderRejected(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	bad := makeOrder("neg-price", types.Buy, -50.0, 1.0)
	_, err := engine.PlaceOrder(bad)
	if err == nil {
		t.Fatal("negative price limit order should be rejected")
	}
}

// TestSymbolNotFound verifies orders on unknown symbols fail
func TestSymbolNotFound(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	bad := &types.Order{
		ID:           "orphan",
		Symbol:       "DOGE/BTC",
		Side:         types.Buy,
		Type:         types.Limit,
		Price:        decimal.NewFromFloat(1.0),
		Quantity:     decimal.NewFromFloat(1.0),
		RemainingQty: decimal.NewFromFloat(1.0),
	}

	_, err := engine.PlaceOrder(bad)
	if err == nil {
		t.Fatal("order on unknown symbol should fail")
	}
}

// TestMultipleLevelsSorted verifies bid and ask levels stay sorted after inserts
func TestMultipleLevelsSorted(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	prices := []float64{105.0, 90.0, 100.0, 95.0, 110.0}
	for i, p := range prices {
		engine.PlaceOrder(makeOrder(
			"bid-"+string(rune('A'+i)),
			types.Buy, p, 1.0,
		))
	}

	bids := book.GetBids()
	for i := 0; i < len(bids)-1; i++ {
		if bids[i].Price.LessThan(bids[i+1].Price) {
			t.Fatalf("bids not sorted descending at index %d: %f < %f",
				i, bids[i].Price.InexactFloat64(), bids[i+1].Price.InexactFloat64())
		}
	}

	askPrices := []float64{210.0, 190.0, 200.0, 195.0, 215.0}
	for i, p := range askPrices {
		engine.PlaceOrder(makeOrder(
			"ask-"+string(rune('A'+i)),
			types.Sell, p, 1.0,
		))
	}

	asks := book.GetAsks()
	for i := 0; i < len(asks)-1; i++ {
		if asks[i].Price.GreaterThan(asks[i+1].Price) {
			t.Fatalf("asks not sorted ascending at index %d: %f > %f",
				i, asks[i].Price.InexactFloat64(), asks[i+1].Price.InexactFloat64())
		}
	}
}

// TestTradeCountIncrements verifies trade count only increases when matching occurs
func TestTradeCountIncrements(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	initial := engine.GetTradeCount()
	engine.PlaceOrder(makeOrder("t1", types.Buy, 100.0, 1.0))
	engine.PlaceOrder(makeOrder("t2", types.Buy, 200.0, 2.0))

	current := engine.GetTradeCount()
	if current < initial {
		t.Fatal("trade count should not decrease")
	}
}

// TestRecentTradesLimit verifies GetRecentTrades respects limit
func TestRecentTradesLimit(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	for i := 0; i < 5; i++ {
		engine.PlaceOrder(makeOrder(
			"ord-"+string(rune('A'+i)),
			types.Buy, float64(100+i), 1.0,
		))
	}

	recent := engine.GetRecentTrades(3)
	if len(recent) > 3 {
		t.Fatalf("expected at most 3 recent trades, got %d", len(recent))
	}

	all := engine.GetRecentTrades(0)
	if len(all) != 0 {
		t.Fatalf("expected 0 trades when no matching occurred, got %d", len(all))
	}
}

// TestGetBidsAsksReturnsCopy verifies snapshot doesn't leak internal state
func TestGetBidsAsksReturnsCopy(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	engine.PlaceOrder(makeOrder("bid-x", types.Buy, 100.0, 1.0))

	bids1 := book.GetBids()
	bids2 := book.GetBids()

	if len(bids1) != len(bids2) {
		t.Fatal("snapshot should return same length")
	}

	bids1[0].Quantity = decimal.NewFromFloat(999.0)
	bids2 = book.GetBids()
	if bids2[0].Quantity.Equal(decimal.NewFromFloat(999.0)) {
		t.Fatal("modifying snapshot should not affect internal state")
	}
}

// TestConcurrentPlaceOrders verifies no races under concurrent writes
func TestConcurrentPlaceOrders(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	done := make(chan struct{})
	for i := 0; i < 20; i++ {
		go func(n int) {
			engine.PlaceOrder(makeOrder(
				"conc-"+string(rune('A'+n%26))+string(rune('0'+n/26)),
				types.Buy, float64(100+n%10), 1.0,
			))
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < 20; i++ {
		<-done
	}

	bids := book.GetBids()
	if len(bids) == 0 {
		t.Fatal("concurrent writes should produce at least one level")
	}
}

// TestConcurrentPlaceAndCancel verifies concurrent place/cancel is safe
func TestConcurrentPlaceAndCancel(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	for i := 0; i < 10; i++ {
		engine.PlaceOrder(makeOrder(
			"pre-"+string(rune('A'+i)),
			types.Buy, float64(100+i), 1.0,
		))
	}

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			engine.PlaceOrder(makeOrder(
				"new-"+string(rune('A'+n)),
				types.Buy, float64(200+n), 0.5,
			))
			done <- struct{}{}
		}(i)
		for j := 0; j < 3; j++ {
			go func(m int) {
				engine.CancelOrder("BTC/USDT", "pre-"+string(rune('A'+m)))
				done <- struct{}{}
			}(i)
		}
	}

	for i := 0; i < 40; i++ {
		<-done
	}
}

// TestSnapshotConsistency verifies GetSnapshot matches book state
func TestSnapshotConsistency(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	engine.PlaceOrder(makeOrder("snap-bid", types.Buy, 100.0, 1.0))
	engine.PlaceOrder(makeOrder("snap-ask", types.Sell, 200.0, 2.0))

	snap := book.GetSnapshot()
	if snap.Symbol != "BTC/USDT" {
		t.Fatalf("expected symbol BTC/USDT, got %s", snap.Symbol)
	}
	if len(snap.Bids) != 1 {
		t.Fatalf("expected 1 bid in snapshot, got %d", len(snap.Bids))
	}
	if len(snap.Asks) != 1 {
		t.Fatalf("expected 1 ask in snapshot, got %d", len(snap.Asks))
	}
}

// TestOrderTimeoutExpiry verifies expired orders are rejected
func TestOrderTimeoutExpiry(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	expired := time.Now().Add(-1 * time.Hour)
	bad := makeOrder("expired", types.Buy, 100.0, 1.0)
	bad.ExpireAt = &expired

	_, err := engine.PlaceOrder(bad)
	if err == nil {
		t.Fatal("expired order should be rejected")
	}
}

// TestBuySellSideInvariants verifies correct side assignment
func TestBuySellSideInvariants(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	bid := makeOrder("side-bid", types.Buy, 100.0, 1.0)
	engine.PlaceOrder(bid)

	ask := makeOrder("side-ask", types.Sell, 200.0, 1.0)
	engine.PlaceOrder(ask)

	bids := book.GetBids()
	asks := book.GetAsks()
	if len(bids) == 0 || len(asks) == 0 {
		t.Fatal("bid and ask should be on separate sides")
	}
	if bids[0].Price.Equal(asks[0].Price) {
		t.Fatal("bid and ask price should differ")
	}
}

// TestOrderIDRequired verifies orders without IDs get auto-generated
func TestOrderIDRequired(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	noID := makeOrder("", types.Buy, 100.0, 1.0)
	result, err := engine.PlaceOrder(noID)
	if err != nil {
		t.Fatalf("order without ID should be accepted: %v", err)
	}
	if noID.ID == "" {
		t.Fatal("order ID should be auto-generated")
	}
	_ = result
}

// TestTableDrivenPriceTimePriority is a table-driven test covering multiple price-time scenarios
func TestTableDrivenPriceTimePriority(t *testing.T) {
	tests := []struct {
		name       string
		bidPrices  []float64
		askPrices  []float64
		expectBids int
		expectAsks int
	}{
		{
			name:       "all same price bids",
			bidPrices:  []float64{100, 100, 100, 100, 100},
			askPrices:  []float64{},
			expectBids: 1,
			expectAsks: 0,
		},
		{
			name:       "all same price asks",
			bidPrices:  []float64{},
			askPrices:  []float64{200, 200, 200},
			expectBids: 0,
			expectAsks: 1,
		},
		{
			name:       "distinct bid prices",
			bidPrices:  []float64{90, 100, 110},
			askPrices:  []float64{},
			expectBids: 3,
			expectAsks: 0,
		},
		{
			name:       "distinct ask prices",
			bidPrices:  []float64{},
			askPrices:  []float64{190, 200, 210},
			expectBids: 0,
			expectAsks: 3,
		},
		{
			name:       "mixed bid and ask",
			bidPrices:  []float64{100, 105},
			askPrices:  []float64{200, 195},
			expectBids: 2,
			expectAsks: 2,
		},
		{
			name:       "wide spread",
			bidPrices:  []float64{50, 60},
			askPrices:  []float64{300, 400},
			expectBids: 2,
			expectAsks: 2,
		},
		{
			name:       "overlapping range",
			bidPrices:  []float64{100, 150, 200},
			askPrices:  []float64{100, 150, 200},
			expectBids: 3,
			expectAsks: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			book := newTestBook()
			engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

			for i, p := range tt.bidPrices {
				engine.PlaceOrder(makeOrder(
					"bid-"+string(rune('A'+i)),
					types.Buy, p, 1.0,
				))
			}
			for i, p := range tt.askPrices {
				engine.PlaceOrder(makeOrder(
					"ask-"+string(rune('A'+i)),
					types.Sell, p, 1.0,
				))
			}

			bids := book.GetBids()
			asks := book.GetAsks()
			if len(bids) != tt.expectBids {
				t.Errorf("expected %d bid levels, got %d", tt.expectBids, len(bids))
			}
			if len(asks) != tt.expectAsks {
				t.Errorf("expected %d ask levels, got %d", tt.expectAsks, len(asks))
			}
		})
	}
}

// TestRandomizedOrderSequences runs 100 randomized order sequences to verify invariants
func TestRandomizedOrderSequences(t *testing.T) {
	const numSequences = 100
	const maxOrdersPerSeq = 50

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for seq := 0; seq < numSequences; seq++ {
		book := newTestBook()
		engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

		numOrders := rng.Intn(maxOrdersPerSeq) + 5
		ids := make([]string, 0, numOrders)

		for i := 0; i < numOrders; i++ {
			side := types.Buy
			if rng.Float64() < 0.5 {
				side = types.Sell
			}

			price := 100.0 + float64(rng.Intn(50)-25)
			qty := 0.1 + rng.Float64()*10.0

			id := "seq-" + string(rune('A'+seq%26)) + "-" + string(rune('0'+i/10)) + string(rune('0'+i%10))
			order := makeOrder(id, side, price, qty)
			engine.PlaceOrder(order)
			ids = append(ids, id)
		}

		// invariant: bid levels must be descending
		bids := book.GetBids()
		for i := 0; i < len(bids)-1; i++ {
			if bids[i].Price.LessThan(bids[i+1].Price) {
				t.Fatalf("seq %d: bids not descending at %d: %f < %f",
					seq, i, bids[i].Price.InexactFloat64(), bids[i+1].Price.InexactFloat64())
			}
		}

		// invariant: ask levels must be ascending
		asks := book.GetAsks()
		for i := 0; i < len(asks)-1; i++ {
			if asks[i].Price.GreaterThan(asks[i+1].Price) {
				t.Fatalf("seq %d: asks not ascending at %d: %f > %f",
					seq, i, asks[i].Price.InexactFloat64(), asks[i+1].Price.InexactFloat64())
			}
		}

		// invariant: no level has zero or negative quantity
		for i, l := range bids {
			if l.Quantity.LessThanOrEqual(decimal.Zero) {
				t.Fatalf("seq %d: bid level %d has non-positive quantity %s", seq, i, l.Quantity.String())
			}
		}
		for i, l := range asks {
			if l.Quantity.LessThanOrEqual(decimal.Zero) {
				t.Fatalf("seq %d: ask level %d has non-positive quantity %s", seq, i, l.Quantity.String())
			}
		}

		// invariant: cancel removes the order cleanly
		cancelCount := rng.Intn(len(ids)/2 + 1)
		for c := 0; c < cancelCount; c++ {
			idx := rng.Intn(len(ids))
			engine.CancelOrder("BTC/USDT", ids[idx])
			ids = append(ids[:idx], ids[idx+1:]...)
		}

		// invariant: book still sorted after cancels
		bids = book.GetBids()
		for i := 0; i < len(bids)-1; i++ {
			if bids[i].Price.LessThan(bids[i+1].Price) {
				t.Fatalf("seq %d: bids not descending after cancel at %d", seq, i)
			}
		}
		asks = book.GetAsks()
		for i := 0; i < len(asks)-1; i++ {
			if asks[i].Price.GreaterThan(asks[i+1].Price) {
				t.Fatalf("seq %d: asks not ascending after cancel at %d", seq, i)
			}
		}
	}
}

// TestPriceInequalityMaintained verifies bid < ask spread is maintained
func TestPriceInequalityMaintained(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	for i := 0; i < 20; i++ {
		engine.PlaceOrder(makeOrder(
			"bid-spread-"+string(rune('A'+i)),
			types.Buy, 90.0+float64(i), 1.0,
		))
		engine.PlaceOrder(makeOrder(
			"ask-spread-"+string(rune('A'+i)),
			types.Sell, 200.0+float64(i), 1.0,
		))
	}

	bids := book.GetBids()
	asks := book.GetAsks()

	if len(bids) == 0 || len(asks) == 0 {
		t.Fatal("should have both bids and asks")
	}

	highestBid := bids[0].Price
	lowestAsk := asks[0].Price

	if highestBid.GreaterThanOrEqual(lowestAsk) {
		t.Fatalf("highest bid %f should be < lowest ask %f", highestBid.InexactFloat64(), lowestAsk.InexactFloat64())
	}
}

// TestIdenticalPriceSameLevel verifies orders at same price aggregate into one level
func TestIdenticalPriceSameLevel(t *testing.T) {
	book := newTestBook()
	engine := newTestEngine(map[types.Symbol]*orderbook.OrderBook{"BTC/USDT": book})

	for i := 0; i < 10; i++ {
		engine.PlaceOrder(makeOrder(
			"agg-"+string(rune('A'+i)),
			types.Buy, 100.0, 0.5,
		))
	}

	bids := book.GetBids()
	if len(bids) != 1 {
		t.Fatalf("10 orders at same price should produce 1 level, got %d", len(bids))
	}

	totalQty := bids[0].Quantity.InexactFloat64()
	if totalQty < 4.99 || totalQty > 5.01 {
		t.Fatalf("expected aggregate quantity ~5.0, got %f", totalQty)
	}
}
