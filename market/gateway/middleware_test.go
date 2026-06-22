# Fix for Issue #11: Bounty Claim: Gateway rate limiting middleware ($65)

// market/gateway/middleware_test.go
package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
)

func TestRateLimitAllowedTraffic(t *testing.T) {
	rl := NewRateLimiter()
	defer rl.Stop()

	handler := RateLimitMiddleware(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// First request should be allowed
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if rr.Header().Get("X-RateLimit-Limit") != "120" {
		t.Errorf("Expected X-RateLimit-Limit to be 120, got %s", rr.Header().Get("X-RateLimit-Limit"))
	}

	remaining := rr.Header().Get("X-RateLimit-Remaining")
	if remaining == "" {
		t.Error("Expected X-RateLimit-Remaining header to be set")
	}
}

func TestRateLimitBlockedTraffic(t *testing.T) {
	// Set low limit for testing
	os.Setenv("MARKET_RATE_LIMIT_PER_MINUTE", "5")
	defer os.Unsetenv("MARKET_RATE_LIMIT_PER_MINUTE")

	rl := NewRateLimiter()
	defer rl.Stop()

	handler := RateLimitMiddleware(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip := "192.168.1.100:12345"

	// Make requests up to the limit
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, rr.Code)
		}
	}

	// Next request should be blocked
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = ip
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", rr.Code)
	}

	if rr.Header().Get("Retry-After") == "" {
		t.Error("Expected Retry-After header to be set")
	}

	var errResp RateLimitError
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Errorf("Failed to decode error response: %v", err)
	}

	if errResp.Error != "rate limit exceeded" {
		t.Errorf("Expected error message 'rate limit exceeded', got '%s'", errResp.Error)
	}
}

func TestRateLimitEnvOverride(t *testing.T) {
	os.Setenv("MARKET_RATE_LIMIT_PER_MINUTE", "200")
	os.Setenv("MARKET_RATE_LIMIT_BURST", "20")
	defer os.Unsetenv("MARKET_RATE_LIMIT_PER_MINUTE")
	defer os.Unsetenv("MARKET_RATE_LIMIT_BURST")

	rl := NewRateLimiter()
	defer rl.Stop()

	if rl.GetLimit() != 200 {
		t.Errorf("Expected limit to be 200, got %d", rl.GetLimit())
	}

	handler := RateLimitMiddleware(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	limitHeader := rr.Header().Get("X-RateLimit-Limit")
	if limitHeader != "200" {
		t.Errorf("Expected X-RateLimit-Limit to be 200, got %s", limitHeader)
	}
}

func TestRateLimitPerIPIsolation(t *testing.T) {
	os.Setenv("MARKET_RATE_LIMIT_PER_MINUTE", "3")
	defer os.Unsetenv("MARKET_RATE_LIMIT_PER_MINUTE")

	rl := NewRateLimiter()
	defer rl.Stop()

	handler := RateLimitMiddleware(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ip1 := "192.168.1.1:12345"
	ip2 := "192.168.1.2:12345"

	// Exhaust limit for IP1
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/api/test", nil)
		req.RemoteAddr = ip1
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}

	// IP1 should be blocked
	req1 := httptest.NewRequest("GET", "/api/test", nil)
	req1.RemoteAddr = ip1
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusTooManyRequests {
		t.Errorf("IP1: Expected status 429, got %d", rr1.Code)
	}

	// IP2 should still be allowed
	req2 := httptest.NewRequest("GET", "/api/test", nil)
	req2.RemoteAddr = ip2
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("IP2: Expected status 200, got %d", rr2.Code)
	}

	remaining, _ := strconv.Atoi(rr2.Header().Get("X-RateLimit-Remaining"))
	if remaining != 2 {
		t.Errorf("IP2: Expected 2 remaining, got %d", remaining)
	}
}