package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestHandlerServesIndexAndFallsBackForClientRoutes(t *testing.T) {
	h := Handler()
	for _, path := range []string{"/", "/riwayat", "/token/abc"} {
		rec := get(t, h, path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d", path, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "<!doctype html>") {
			t.Fatalf("%s did not serve index.html: %s", path, rec.Body.String())
		}
	}
	if ct := get(t, h, "/").Header().Get("Cache-Control"); ct != "no-store" {
		t.Fatalf("index must not be cached, got %q", ct)
	}
}

func TestHandlerDoesNotServeIndexForMissingFiles(t *testing.T) {
	rec := get(t, Handler(), "/assets/missing.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("static miss falls back to the SPA shell, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<!doctype html>") {
		t.Fatal("static miss must still return the shell")
	}
}