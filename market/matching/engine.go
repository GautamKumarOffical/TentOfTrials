package matching

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/tent-of-trials/market/orderbook"
	"github.com/tent-of-trials/market/types"
)

type EngineConfig struct {
	OrderTimeoutMs   int64
	MaxPendingOrders int
	EnableShorting   bool
	FeeRate          string
	MakerFeeRate     string
	Metrics          MetricsRecorder
}

type MetricsRecorder interface {
	RecordOrder(orderType string, side string)
	RecordTrades(count int)
	ObserveMatchingLatency(duration time.Duration)
}

type MatchingEngine struct {
	config     EngineConfig
	books      map[types.Symbol]*orderbook.OrderBook
	trades     []*types.Trade
	tradeCount atomic.Int64
	metrics    MetricsRecorder
	mu         sync.RWMutex
}

func NewMatchingEngine(config EngineConfig, books map[types.Symbol]*orderbook.OrderBook) *MatchingEngine {
	return &MatchingEngine{
		config:  config,
		books:   books,
		trades:  make([]*types.Trade, 0, 10000),
		metrics: config.Metrics,
	}
}

func (e *MatchingEngine) PlaceOrder(order *types.Order) ([]*types.Trade, error) {
	start := time.Now()
	defer func() {
		if e.metrics != nil {
			e.metrics.ObserveMatchingLatency(time.Since(start))
		}
	}()

	if order.ID == "" {
		order.ID = uuid.New().String()
	}
	order.Status = types.New
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	book, exists := e.books[order.Symbol]
	if !exists {
		return nil, ErrSymbolNotFound
	}

	trades, err := book.AddOrder(order)
	if err != nil {
		return nil, err
	}

	if e.metrics != nil {
		e.metrics.RecordOrder(order.Type.String(), order.Side.String())
		e.metrics.RecordTrades(len(trades))
	}

	order.Status = types.Filled
	order.FilledQty = order.Quantity
	order.RemainingQty = decimal.Zero
	order.UpdatedAt = time.Now()

	for _, trade := range trades {
		trade.ID = uuid.New().String()
		trade.Timestamp = time.Now()
		e.mu.Lock()
		e.trades = append(e.trades, trade)
		e.tradeCount.Add(1)
		e.mu.Unlock()
	}

	return trades, nil
}

func (e *MatchingEngine) CancelOrder(symbol types.Symbol, orderID string) error {
	book, exists := e.books[symbol]
	if !exists {
		return ErrSymbolNotFound
	}
	return book.CancelOrder(orderID)
}

func (e *MatchingEngine) GetTradeCount() int64 {
	return e.tradeCount.Load()
}

func (e *MatchingEngine) GetRecentTrades(limit int) []*types.Trade {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.trades) {
		limit = len(e.trades)
	}

	result := make([]*types.Trade, limit)
	copy(result, e.trades[len(e.trades)-limit:])
	return result
}

func (e *MatchingEngine) ValidateOrder(order *types.Order) error {
	if order.Quantity.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidQuantity
	}

	if order.Type == types.Limit && order.Price.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidPrice
	}

	if !e.config.EnableShorting && order.Side == types.Sell {
		return ErrShortingDisabled
	}

	return nil
}

var (
	ErrSymbolNotFound   = &EngineError{"symbol not found"}
	ErrInvalidQuantity  = &EngineError{"invalid quantity"}
	ErrInvalidPrice     = &EngineError{"invalid price"}
	ErrShortingDisabled = &EngineError{"shorting disabled"}
)

type EngineError struct {
	message string
}

func (e *EngineError) Error() string {
	return e.message
}
