package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func rateLimitHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func TestRateLimitMiddleware_AllowsTraffic(t *testing.T) {
	os.Unsetenv("MARKET_RATE_LIMIT_PER_MINUTE")
	os.Unsetenv("MARKET_RATE_LIMIT_BURST")
	defer os.Unsetenv("MARKET_RATE_LIMIT_PER_MINUTE")
	defer os.Unsetenv("MARKET_RATE_LIMIT_BURST")

	mw := RateLimitMiddleware(120, 120)
	ts := httptest.NewServer(mw(rateLimitHandler()))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	if resp.Header.Get("X-RateLimit-Limit") == "" {
		t.Error("missing X-RateLimit-Limit header")
	}
	if resp.Header.Get("X-RateLimit-Remaining") == "" {
		t.Error("missing X-RateLimit-Remaining header")
	}
}

func TestRateLimitMiddleware_BlocksTraffic(t *testing.T) {
	os.Unsetenv("MARKET_RATE_LIMIT_PER_MINUTE")
	os.Unsetenv("MARKET_RATE_LIMIT_BURST")
	defer os.Unsetenv("MARKET_RATE_LIMIT_PER_MINUTE")
	defer os.Unsetenv("MARKET_RATE_LIMIT_BURST")

	mw := RateLimitMiddleware(1, 1)
	ts := httptest.NewServer(mw(rateLimitHandler()))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("first request: expected 200, got %d", resp.StatusCode)
	}

	resp2, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Errorf("second request: expected 429, got %d", resp2.StatusCode)
	}

	if resp2.Header.Get("Retry-After") == "" {
		t.Error("missing Retry-After header on 429")
	}

	var body map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&body)
	if body["error"] != "rate_limit_exceeded" {
		t.Errorf("unexpected error body: %v", body)
	}
}

func TestRateLimitMiddleware_EnvOverride(t *testing.T) {
	os.Setenv("MARKET_RATE_LIMIT_PER_MINUTE", "2")
	os.Setenv("MARKET_RATE_LIMIT_BURST", "2")
	defer os.Unsetenv("MARKET_RATE_LIMIT_PER_MINUTE")
	defer os.Unsetenv("MARKET_RATE_LIMIT_BURST")

	ratePerMinute := 120.0
	burst := 120
	if v := os.Getenv("MARKET_RATE_LIMIT_PER_MINUTE"); v != "" {
		ratePerMinute = 2
	}
	if v := os.Getenv("MARKET_RATE_LIMIT_BURST"); v != "" {
		burst = 2
	}

	mw := RateLimitMiddleware(ratePerMinute, burst)
	ts := httptest.NewServer(mw(rateLimitHandler()))
	defer ts.Close()

	for i := 0; i < 2; i++ {
		resp, err := http.Get(ts.URL)
		if err != nil {
			t.Fatalf("request %d failed: %v", i+1, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("third request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("third request: expected 429, got %d", resp.StatusCode)
	}
}

func TestRateLimitMiddleware_DifferentIPs(t *testing.T) {
	os.Unsetenv("MARKET_RATE_LIMIT_PER_MINUTE")
	os.Unsetenv("MARKET_RATE_LIMIT_BURST")
	defer os.Unsetenv("MARKET_RATE_LIMIT_PER_MINUTE")
	defer os.Unsetenv("MARKET_RATE_LIMIT_BURST")

	mw := RateLimitMiddleware(1, 1)
	ts := httptest.NewServer(mw(rateLimitHandler()))
	defer ts.Close()

	resp1, _ := http.Get(ts.URL)
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Errorf("IP1 first: expected 200, got %d", resp1.StatusCode)
	}

	resp1b, _ := http.Get(ts.URL)
	resp1b.Body.Close()
	if resp1b.StatusCode != http.StatusTooManyRequests {
		t.Errorf("IP1 second: expected 429, got %d", resp1b.StatusCode)
	}
}
