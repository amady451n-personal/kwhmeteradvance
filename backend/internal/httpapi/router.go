package httpapi

import (
	"net/http"
	"sync"

	"meteran/internal/store"
)

// Server holds everything the handlers need. One instance per process.
type Server struct {
	DB           *store.DB
	Version      string
	Secret       []byte // AES key for token codes, derived from METERAN_SECRET
	VAPIDPublic  string
	CookieSecure bool
	Web          http.Handler // SPA, nil while only the API is wired

	mu      sync.Mutex
	pending map[string]pendingImport
}

// Handler builds the full route table.
func (s *Server) Handler() http.Handler {
	if s.pending == nil {
		s.pending = map[string]pendingImport{}
	}
	mux := http.NewServeMux()

	// Public: FR-110 homepage, health, and session setup/login.
	mux.Handle("GET /api/health", public(s.health))
	mux.Handle("GET /api/homepage", public(s.homepage))
	mux.Handle("GET /api/session", public(s.getSession))
	mux.Handle("POST /api/session", public(s.postSession))
	mux.Handle("PUT /api/session/pin", public(s.putPin))
	mux.Handle("DELETE /api/session", s.auth(s.deleteSession))

	// Readings.
	mux.Handle("GET /api/readings", s.auth(s.listReadings))
	mux.Handle("POST /api/readings", s.auth(s.createReading))
	mux.Handle("GET /api/readings/{id}", s.auth(s.getReading))
	mux.Handle("PUT /api/readings/{id}", s.auth(s.putReading))
	mux.Handle("DELETE /api/readings/{id}", s.auth(s.deleteReading))

	// Tokens.
	mux.Handle("GET /api/tokens", s.auth(s.listTokens))
	mux.Handle("POST /api/tokens", s.auth(s.createToken))
	mux.Handle("GET /api/tokens/{id}", s.auth(s.getToken))
	mux.Handle("PUT /api/tokens/{id}", s.auth(s.putToken))
	mux.Handle("PATCH /api/tokens/{id}", s.auth(s.patchToken))
	mux.Handle("DELETE /api/tokens/{id}", s.auth(s.deleteToken))
	mux.Handle("POST /api/tokens/{id}/mark-used", s.auth(s.markTokenUsed))
	mux.Handle("POST /api/tokens/{id}/cancel", s.auth(s.cancelToken))
	mux.Handle("POST /api/tokens/{id}/reveal", s.auth(s.revealToken))

	// Derived views.
	mux.Handle("GET /api/usage", s.auth(s.usage))
	mux.Handle("GET /api/summary", s.auth(s.summary))
	mux.Handle("GET /api/settings", s.auth(s.getSettings))
	mux.Handle("PATCH /api/settings", s.auth(s.patchSettings))
	mux.Handle("GET /api/reports/summary.pdf", s.auth(s.reportPDF))

	// Import (FR-100..105).
	mux.Handle("POST /api/import/preview", s.auth(s.importPreview))
	mux.Handle("POST /api/import/commit", s.auth(s.importCommit))

	// Web Push.
	mux.Handle("GET /api/push/public-key", s.auth(s.pushPublicKey))
	mux.Handle("POST /api/push/subscriptions", s.auth(s.subscribePush))
	mux.Handle("DELETE /api/push/subscriptions/{id}", s.auth(s.unsubscribePush))

	// Unknown API paths stay JSON 404 instead of falling through to the SPA.
	mux.Handle("/api/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Fail(w, http.StatusNotFound, CodeNotFound, "Data tidak ditemukan.")
	}))

	// Everything else is the SPA.
	if s.Web != nil {
		mux.Handle("/", s.Web)
	} else {
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend not built into this binary", http.StatusNotFound)
		}))
	}

	return recoverer(logger(mux))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	if err := s.DB.SQL.Ping(); err != nil {
		status = "degraded"
	}
	Data(w, http.StatusOK, map[string]any{
		"status":  status,
		"version": s.Version,
		"time":    store.Now(),
	})
}