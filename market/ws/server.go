package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tent-of-trials/market/matching"
	"github.com/tent-of-trials/market/types"
	"go.uber.org/zap"
)

const (
	defaultHeartbeatIntervalSecs = 30
	defaultIdleMultiplier        = 2
)

func getHeartbeatInterval() time.Duration {
	val := os.Getenv("WS_HEARTBEAT_INTERVAL_SECS")
	if val != "" {
		if secs, err := strconv.Atoi(val); err == nil && secs > 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return time.Duration(defaultHeartbeatIntervalSecs) * time.Second
}

func getReadDeadline(interval time.Duration) time.Duration {
	return interval * defaultIdleMultiplier
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	subs      map[types.Symbol]struct{}
	remote    string
	mu        sync.Mutex
	lastPong  time.Time
	connected time.Time
}

type Hub struct {
	clients    map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	logger     *zap.Logger
	mu         sync.RWMutex
	activeCount int
}

func (h *Hub) ActiveCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.activeCount
}

type Server struct {
	hub    *Hub
	engine *matching.MatchingEngine
	logger *zap.Logger
	port   int
	srv    *http.Server
}

func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
		logger:     logger,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			h.activeCount = len(h.clients)
			count := h.activeCount
			h.mu.Unlock()
			h.logger.Info("client connected",
				zap.String("remote", client.remote),
				zap.Int("total", count),
			)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.activeCount = len(h.clients)
			}
			count := h.activeCount
			h.mu.Unlock()
			h.logger.Info("client disconnected",
				zap.String("remote", client.remote),
				zap.Int("total", count),
			)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func NewServer(hub *Hub, engine *matching.MatchingEngine, logger *zap.Logger, port int) *Server {
	return &Server{
		hub:    hub,
		engine: engine,
		logger: logger,
		port:   port,
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/v1/trades", s.handleGetTrades)
	mux.HandleFunc("/api/v1/depth", s.handleGetDepth)

	s.srv = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s.srv.ListenAndServe()
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.srv.Shutdown(ctx)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", zap.Error(err))
		return
	}

	now := time.Now()
	client := &Client{
		hub:       s.hub,
		conn:      conn,
		send:      make(chan []byte, 256),
		subs:      make(map[types.Symbol]struct{}),
		remote:    r.RemoteAddr,
		lastPong:  now,
		connected: now,
	}

	s.hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "ok",
		"service":        "tent-market",
		"time":           time.Now().Unix(),
		"active_ws":      s.hub.ActiveCount(),
		"heartbeat_secs": int(getHeartbeatInterval().Seconds()),
	})
}

func (s *Server) handleGetTrades(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	trades := s.engine.GetRecentTrades(100)
	json.NewEncoder(w).Encode(trades)
}

func (s *Server) handleGetDepth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "depth endpoint"})
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	heartbeatInterval := getHeartbeatInterval()
	idleDeadline := getReadDeadline(heartbeatInterval)

	c.conn.SetReadLimit(65536)
	c.conn.SetReadDeadline(time.Now().Add(idleDeadline))
	c.conn.SetPongHandler(func(string) error {
		c.mu.Lock()
		c.lastPong = time.Now()
		c.mu.Unlock()
		c.conn.SetReadDeadline(time.Now().Add(idleDeadline))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var event map[string]interface{}
		if err := json.Unmarshal(message, &event); err != nil {
			continue
		}

		c.mu.Lock()

		c.mu.Unlock()
	}
}

func (c *Client) writePump() {
	heartbeatInterval := getHeartbeatInterval()
	ticker := time.NewTicker(heartbeatInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
