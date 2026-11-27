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
	if ok, _ := rl.allow("1.2.3.4", now); !ok {
		t.Fatal("first request must pass")
	}
	if ok, _ := rl.allow("1.2.3.4", now); !ok {
		t.Fatal("second request must pass")
	}
	if ok, _ := rl.allow("1.2.3.4", now); ok {
		t.Fatal("third request must be blocked")
	}
	if ok, _ := rl.allow("5.6.7.8", now); !ok {
		t.Fatal("another IP has its own bucket")
	}
	if ok, _ := rl.allow("1.2.3.4", now.Add(2*time.Minute)); !ok {
		t.Fatal("bucket resets after the window")
	}
}

func TestRateLimitExemptsSessionAndHealth(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	h := rl.wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for _, path := range []string{"/api/health", "/api/session"} {
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.Header.Set("X-Forwarded-For", "7.7.7.7")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s request %d = %d", path, i, rec.Code)
			}
		}
	}
}

func TestRateLimitSkipsStaticAssets(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	h := rl.wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/assets/index.js", nil)
	req.Header.Set("X-Forwarded-For", "8.8.8.8")
	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("static request %d = %d", i, rec.Code)
		}
	}
}

func TestRateLimitMiddlewareReturns429(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	h := rl.wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/readings", nil)
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
