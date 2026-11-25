package httpapi

import (
	"log"
	"net/http"
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

func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
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