package httpapi

import (
	"net/http"
	"strings"

	"meteran/internal/store"
)

func (s *Server) pushPublicKey(w http.ResponseWriter, r *http.Request) {
	Data(w, http.StatusOK, map[string]string{"publicKey": s.VAPIDPublic})
}

func (s *Server) subscribePush(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Endpoint string        `json:"endpoint"`
		Keys     store.PushKeys `json:"keys"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !isHTTPS(in.Endpoint) || in.Keys.P256dh == "" || in.Keys.Auth == "" {
		Fail(w, http.StatusUnprocessableEntity, CodeValidationError, "Data tidak valid.",
			Detail{Field: "endpoint", Code: "invalid_subscription", Message: "Endpoint push tidak valid."})
		return
	}
	sub, err := s.DB.Subscribe(in.Endpoint, in.Keys.P256dh, in.Keys.Auth)
	if err != nil {
		failStore(w, err)
		return
	}
	_ = s.DB.SetPushEnabled(true)
	Data(w, http.StatusCreated, sub)
}

func (s *Server) unsubscribePush(w http.ResponseWriter, r *http.Request) {
	if err := s.DB.Unsubscribe(r.PathValue("id")); err != nil {
		failStore(w, err)
		return
	}
	if n, err := s.DB.CountSubscriptions(); err == nil && n == 0 {
		_ = s.DB.SetPushEnabled(false)
	}
	w.WriteHeader(http.StatusNoContent)
}

func isHTTPS(endpoint string) bool {
	return len(endpoint) > 8 && strings.HasPrefix(endpoint, "https://")
}