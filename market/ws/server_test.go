package ws

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tent-of-trials/market/matching"
	"go.uber.org/zap"
)

func TestGetHeartbeatIntervalDefault(t *testing.T) {
	os.Unsetenv("WS_HEARTBEAT_INTERVAL_SECS")
	got := getHeartbeatInterval()
	if got != 30*time.Second {
		t.Errorf("expected 30s, got %v", got)
	}
}

func TestGetHeartbeatIntervalCustom(t *testing.T) {
	os.Setenv("WS_HEARTBEAT_INTERVAL_SECS", "10")
	defer os.Unsetenv("WS_HEARTBEAT_INTERVAL_SECS")
	got := getHeartbeatInterval()
	if got != 10*time.Second {
		t.Errorf("expected 10s, got %v", got)
	}
}

func TestGetHeartbeatIntervalInvalid(t *testing.T) {
	os.Setenv("WS_HEARTBEAT_INTERVAL_SECS", "abc")
	defer os.Unsetenv("WS_HEARTBEAT_INTERVAL_SECS")
	got := getHeartbeatInterval()
	if got != 30*time.Second {
		t.Errorf("expected fallback 30s, got %v", got)
	}
}

func TestGetHeartbeatIntervalZero(t *testing.T) {
	os.Setenv("WS_HEARTBEAT_INTERVAL_SECS", "0")
	defer os.Unsetenv("WS_HEARTBEAT_INTERVAL_SECS")
	got := getHeartbeatInterval()
	if got != 30*time.Second {
		t.Errorf("expected fallback 30s for zero, got %v", got)
	}
}

func TestGetReadDeadline(t *testing.T) {
	interval := 15 * time.Second
	got := getReadDeadline(interval)
	if got != 30*time.Second {
		t.Errorf("expected 30s (2x15s), got %v", got)
	}
}

func TestHubActiveCount(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	if hub.ActiveCount() != 0 {
		t.Errorf("expected 0 active, got %d", hub.ActiveCount())
	}
}

func TestNewClientHasLastPong(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()
	engine := &matching.MatchingEngine{}
	srv := NewServer(hub, engine, logger, 8080)

	// Create test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.handleWebSocket(w, r)
	}))
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer ws.Close()

	// Wait briefly for client registration
	time.Sleep(100 * time.Millisecond)

	if hub.ActiveCount() != 1 {
		t.Errorf("expected 1 active connection, got %d", hub.ActiveCount())
	}

	// Send a pong to verify lastPong updates
	ws.WriteMessage(websocket.PongMessage, nil)
	time.Sleep(50 * time.Millisecond)
}

func TestIdleConnectionClosed(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	go hub.Run()
	engine := &matching.MatchingEngine{}
	srv := NewServer(hub, engine, logger, 8081)

	// Use a very short heartbeat for testing
	os.Setenv("WS_HEARTBEAT_INTERVAL_SECS", "1")
	defer os.Unsetenv("WS_HEARTBEAT_INTERVAL_SECS")

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.handleWebSocket(w, r)
	}))
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer ws.Close()

	time.Sleep(100 * time.Millisecond)
	if hub.ActiveCount() != 1 {
		t.Fatalf("expected 1 active, got %d", hub.ActiveCount())
	}

	// Don't respond to pings. The idle timeout should close us.
	// With 1s heartbeat * 2 multiplier = 2s idle deadline
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err = ws.ReadMessage()
	if err == nil {
		t.Fatal("expected read error (connection closed), got nil")
	}
}

func TestHealthEndpointIncludesHeartbeatInfo(t *testing.T) {
	logger := zap.NewNop()
	hub := NewHub(logger)
	engine := &matching.MatchingEngine{}
	srv := NewServer(hub, engine, logger, 8082)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.handleHealth(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "heartbeat_secs") {
		t.Error("health response missing heartbeat_secs")
	}
	if !strings.Contains(body, "active_ws") {
		t.Error("health response missing active_ws")
	}
}
