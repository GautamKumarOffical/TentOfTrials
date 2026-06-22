# Fix for Issue #11: Bounty Claim: Gateway rate limiting middleware ($65)

// market/gateway/middleware.go
package gateway

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

type RateLimiter struct {
	mu           sync.RWMutex
	requests     map[string][]time.Time
	limit        int
	burst        int
	windowSize   time.Duration
	cleanupTick  *time.Ticker
	stopCleanup  chan struct{}
}

type RateLimitError struct {
	Error      string `json:"error"`
	RetryAfter int    `json:"retry_after"`
}

func NewRateLimiter() *RateLimiter {
	limit := 120
	burst := 10

	if envLimit := os.Getenv("MARKET_RATE_LIMIT_PER_MINUTE"); envLimit != "" {
		if parsed, err := strconv.Atoi(envLimit); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if envBurst := os.Getenv("MARKET_RATE_LIMIT_BURST"); envBurst != "" {
		if parsed, err := strconv.Atoi(envBurst); err == nil && parsed > 0 {
			burst = parsed
		}
	}

	rl := &RateLimiter{
		requests:    make(map[string][]time.Time),
		limit:       limit,
		burst:       burst,
		windowSize:  time.Minute,
		cleanupTick: time.NewTicker(time.Minute * 5),
		stopCleanup: make(chan struct{}),
	}

	go rl.cleanup()

	return rl
}

func (rl *RateLimiter) cleanup() {
	for {
		select {
		case <-rl.cleanupTick.C:
			rl.mu.Lock()
			now := time.Now()
			for ip, times := range rl.requests {
				var valid []time.Time
				for _, t := range times {
					if now.Sub(t) <= rl.windowSize {
						valid = append(valid, t)
					}
				}
				if len(valid) == 0 {
					delete(rl.requests, ip)
				} else {
					rl.requests[ip] = valid
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCleanup:
			rl.cleanupTick.Stop()
			return
		}
	}
}

func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
}

func (rl *RateLimiter) Allow(ip string) (bool, int, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.windowSize)

	// Filter requests within the sliding window
	var validRequests []time.Time
	for _, t := range rl.requests[ip] {
		if t.After(windowStart) {
			validRequests = append(validRequests, t)
		}
	}

	currentCount := len(validRequests)
	remaining := rl.limit - currentCount

	if currentCount >= rl.limit {
		// Calculate retry after based on oldest request in window
		if len(validRequests) > 0 {
			oldestInWindow := validRequests[0]
			retryAfter := int(rl.windowSize.Seconds() - now.Sub(oldestInWindow).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			return false, 0, retryAfter
		}
		return false, 0, 60
	}

	// Add current request
	validRequests = append(validRequests, now)
	rl.requests[ip] = validRequests

	return true, remaining - 1, 0
}

func (rl *RateLimiter) GetLimit() int {
	return rl.limit
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	return r.RemoteAddr
}

func RateLimitMiddleware(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			allowed, remaining, retryAfter := rl.Allow(ip)

			// Set rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.GetLimit()))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)

				errResp := RateLimitError{
					Error:      "rate limit exceeded",
					RetryAfter: retryAfter,
				}
				json.NewEncoder(w).Encode(errResp)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}