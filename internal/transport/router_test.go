package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/PakSerg/NotiHub/internal/repository"
	"github.com/PakSerg/NotiHub/internal/service"
)

type denyingLimiter struct{}

func (denyingLimiter) Allow(key string) bool { return false }

func newTestHandler(limiter RateLimiter) *Handler {
	svc := service.NewNotificationService(repository.NewMemoryRepository())
	return NewHandler(svc, limiter)
}

func TestRouterWithoutLimiterAllowsRequests(t *testing.T) {
	h := newTestHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/notifications/missing", nil)
	rec := httptest.NewRecorder()
	h.Router().ServeHTTP(rec, req)

	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf("expected no rate limiting without a limiter, got %d", rec.Code)
	}
}

func TestRateLimitMiddlewareRejectsWhenDenied(t *testing.T) {
	h := newTestHandler(denyingLimiter{})

	req := httptest.NewRequest(http.MethodGet, "/notifications/missing", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	h.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header to be set")
	}
}

func TestRateLimitMiddlewareNeverThrottlesHealth(t *testing.T) {
	h := newTestHandler(denyingLimiter{})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	h.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected health check to bypass rate limiting, got %d", rec.Code)
	}
}
