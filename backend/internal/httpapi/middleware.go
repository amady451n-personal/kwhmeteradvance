package httpapi

import (
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const sessionCookie = "meteran_session"

// recoverer turns a panic into a contract-shaped 500 instead of a dropped
// connection.
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Printf("panic %s %s: %v", r.Method, r.URL.Path, v)
				Fail(w, http.StatusInternalServerError, CodeInternal, "Terjadi kesalahan pada server.")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// logger only writes when something is wrong or slow. Logging every healthy
// poll filled the container's JSON log with tens of megabytes of I/O.
func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		elapsed := time.Since(start)
		if sw.status >= 400 || elapsed > 500*time.Millisecond {
			log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, elapsed.Round(time.Millisecond))
		}
	})
}

type rateBucket struct {
	count int
	reset time.Time
}

type rateLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	buckets   map[string]rateBucket
	lastClean time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:     limit,
		window:    window,
		buckets:   make(map[string]rateBucket),
		lastClean: time.Now(),
	}
}

func (rl *rateLimiter) allow(key string, now time.Time) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	bucket, ok := rl.buckets[key]
	if !ok || !now.Before(bucket.reset) {
		bucket = rateBucket{reset: now.Add(rl.window)}
	}
	if bucket.count >= rl.limit {
		return false
	}
	bucket.count++
	rl.buckets[key] = bucket
	if now.Sub(rl.lastClean) > rl.window {
		for k, v := range rl.buckets {
			if !now.Before(v.reset) {
				delete(rl.buckets, k)
			}
		}
		rl.lastClean = now
	}
	return true
}

func (rl *rateLimiter) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Static shell and service worker must always load; only API calls are
		// worth throttling. Otherwise a hard reload trips the limit itself.
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		key := r.Header.Get("X-Real-IP")
		if key == "" {
			key = r.Header.Get("X-Forwarded-For")
			if comma := strings.IndexByte(key, ','); comma >= 0 {
				key = key[:comma]
			}
			key = strings.TrimSpace(key)
		}
		if key == "" {
			host, _, err := net.SplitHostPort(r.RemoteAddr)
			if err == nil {
				key = host
			} else {
				key = r.RemoteAddr
			}
		}
		if !rl.allow(key, time.Now()) {
			log.Printf("rate limit: ip=%s path=%s", key, r.URL.Path)
			w.Header().Set("Retry-After", "60")
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
			Fail(w, http.StatusTooManyRequests, CodeRateLimited, "Terlalu banyak permintaan. Coba lagi nanti.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// public marks a route that does not require a session.
func public(h http.HandlerFunc) http.Handler { return h }

// auth rejects unauthenticated calls with 401. Setup routes stay public
// because they are self-authorizing (first PIN, session login).
func (s *Server) auth(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authenticated(r) {
			Fail(w, http.StatusUnauthorized, CodeUnauthorized, "Sesi tidak aktif. Masukkan PIN.")
			return
		}
		h(w, r)
	})
}

func (s *Server) authenticated(r *http.Request) bool {
	// Before the first PIN exists the app has nothing to protect. Session
	// handlers still work, but every data route is reachable for setup.
	hasPin, err := s.DB.HasPin()
	if err != nil {
		return false
	}
	if !hasPin {
		return true
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	return s.DB.SessionValid(c.Value)
}
