# Fix for Issue #11: Bounty Claim: Gateway rate limiting middleware ($65)

// market/gateway/api.go
package gateway

import (
	"net/http"
)

type Gateway struct {
	rateLimiter *RateLimiter
	mux         *http.ServeMux
}

func NewGateway() *Gateway {
	g := &Gateway{
		rateLimiter: NewRateLimiter(),
		mux:         http.NewServeMux(),
	}
	g.setupRoutes()
	return g
}

func (g *Gateway) setupRoutes() {
	g.mux.HandleFunc("/health", g.healthHandler)
}

func (g *Gateway) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy"}`))
}

func (g *Gateway) Handler() http.Handler {
	return RateLimitMiddleware(g.rateLimiter)(g.mux)
}

func (g *Gateway) Stop() {
	if g.rateLimiter != nil {
		g.rateLimiter.Stop()
	}
}