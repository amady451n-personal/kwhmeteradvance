package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterBlocksOverLimit(t *testing.T) {
	rl := newRateLimiter(2, time.Minute)
	now := time.Now()
	if !rl.allow("1.2.3.4", now) || !rl.allow("1.2.3.4", now) {
		t.Fatal("first two requests must pass")
	}
	if rl.allow("1.2.3.4", now) {
		t.Fatal("third request must be blocked")
	}
	if !rl.allow("5.6.7.8", now) {
		t.Fatal("another IP has its own bucket")
	}
	if !rl.allow("1.2.3.4", now.Add(2*time.Minute)) {
		t.Fatal("bucket resets after the window")
	}
}

func TestRateLimitMiddlewareReturns429(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	h := rl.wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("X-Forwarded-For", "9.9.9.9")
	if rec := httptest.NewRecorder(); true {
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("first = %d", rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second = %d", rec.Code)
	}
}
