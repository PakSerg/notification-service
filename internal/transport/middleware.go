package transport

import (
	"net"
	"net/http"
)

// RateLimiter is satisfied by *ratelimit.Limiter. It's declared here rather
// than imported so this package doesn't need to know how limiting works.
type RateLimiter interface {
	Allow(key string) bool
}

// rateLimitMiddleware throttles requests per client IP, leaving /health
// unthrottled so liveness/readiness probes are never rejected.
func rateLimitMiddleware(limiter RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		if !limiter.Allow(clientKey(r)) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// clientKey derives the rate-limit bucket key from the connecting peer's
// address. It intentionally ignores X-Forwarded-For: trusting it here would
// let a client bypass its own limit by sending an arbitrary value.
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
