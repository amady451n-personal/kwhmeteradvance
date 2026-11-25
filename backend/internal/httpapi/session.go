package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"net"
	"net/http"
	"strconv"

	"meteran/internal/auth"
)

func (s *Server) sessionValid(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	return s.DB.SessionValid(c.Value)
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	pinSet, err := s.DB.HasPin()
	if err != nil {
		failStore(w, err)
		return
	}
	valid := s.sessionValid(r)
	var expiresAt *string
	if valid {
		if c, err := r.Cookie(sessionCookie); err == nil {
			if exp, err := s.DB.SessionExpiry(c.Value); err == nil && exp.Valid {
				v := exp.String
				expiresAt = &v
			}
		}
	}
	authenticated := valid && pinSet
	Data(w, http.StatusOK, map[string]any{
		"authenticated": authenticated,
		"pinSet":        pinSet,
		"expiresAt":     expiresAt,
	})
}

func (s *Server) postSession(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Pin string `json:"pin"`
	}
	if !decode(w, r, &in) {
		return
	}
	ip := clientIP(r)
	limiter := auth.Limiter{DB: s.DB}
	if ok, wait := limiter.Allow(ip); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		Fail(w, http.StatusTooManyRequests, CodeRateLimited, "Terlalu banyak percobaan. Coba lagi nanti.")
		return
	}
	hash, err := s.DB.PinHash()
	if err != nil {
		_ = limiter.Fail(ip)
		Fail(w, http.StatusUnauthorized, CodePinInvalid, "PIN belum diatur.")
		return
	}
	if !auth.VerifyPin(hash, in.Pin) {
		_ = limiter.Fail(ip)
		Fail(w, http.StatusUnauthorized, CodePinInvalid, "PIN salah.")
		return
	}
	_ = limiter.Reset(ip)
	token, err := newSessionToken()
	if err != nil {
		failStore(w, err)
		return
	}
	if err := s.DB.CreateSession(token); err != nil {
		failStore(w, err)
		return
	}
	s.setSessionCookie(w, token)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		_ = s.DB.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.CookieSecure,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) putPin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CurrentPin *string `json:"currentPin"`
		NewPin     string  `json:"newPin"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.NewPin) < 4 || len(in.NewPin) > 128 {
		Fail(w, http.StatusUnprocessableEntity, CodeValidationError, "Data tidak valid.",
			Detail{Field: "newPin", Code: "invalid_length", Message: "PIN harus 4 sampai 128 karakter."})
		return
	}
	pinSet, err := s.DB.HasPin()
	if err != nil {
		failStore(w, err)
		return
	}
	ip := clientIP(r)
	limiter := auth.Limiter{DB: s.DB}
	if pinSet {
		if !s.sessionValid(r) {
			Fail(w, http.StatusUnauthorized, CodeUnauthorized, "Sesi tidak aktif. Masukkan PIN.")
			return
		}
		if ok, wait := limiter.Allow(ip); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
			Fail(w, http.StatusTooManyRequests, CodeRateLimited, "Terlalu banyak percobaan. Coba lagi nanti.")
			return
		}
		hash, err := s.DB.PinHash()
		if err != nil {
			failStore(w, err)
			return
		}
		if in.CurrentPin == nil || !auth.VerifyPin(hash, *in.CurrentPin) {
			_ = limiter.Fail(ip)
			Fail(w, http.StatusUnauthorized, CodePinInvalid, "PIN lama salah.")
			return
		}
		_ = limiter.Reset(ip)
	}

	newHash, err := auth.HashPin(in.NewPin)
	if err != nil {
		failStore(w, err)
		return
	}
	if err := s.DB.SetPin(newHash); err != nil {
		failStore(w, err)
		return
	}
	// Changing the PIN invalidates every existing session.
	if _, err := s.DB.SQL.Exec(`delete from sessions`); err != nil {
		failStore(w, err)
		return
	}
	token, err := newSessionToken()
	if err != nil {
		failStore(w, err)
		return
	}
	if err := s.DB.CreateSession(token); err != nil {
		failStore(w, err)
		return
	}
	s.setSessionCookie(w, token)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.CookieSecure,
	})
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// clientIP prefers the proxy header because the STB sits behind a reverse proxy.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}