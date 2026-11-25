package httpapi

import (
	"log"
	"net/http"

	"meteran/internal/store"
)

func (s *Server) listTokens(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && status != "unused" && status != "used" && status != "locked" {
		Fail(w, http.StatusUnprocessableEntity, CodeValidationError, "Data tidak valid.",
			Detail{Field: "status", Code: "invalid_status", Message: "Status token tidak dikenal."})
		return
	}
	rows, meta, err := s.DB.ListTokens(store.TokenQuery{
		Status: status, Limit: atoiDefault(r.URL.Query().Get("limit"), 20), Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		failStore(w, err)
		return
	}
	DataMeta(w, http.StatusOK, rows, meta)
}

func (s *Server) getToken(w http.ResponseWriter, r *http.Request) {
	t, err := s.DB.GetToken(r.PathValue("id"))
	if err != nil {
		failStore(w, err)
		return
	}
	Data(w, http.StatusOK, t)
}

func (s *Server) createToken(w http.ResponseWriter, r *http.Request) {
	var in store.TokenInput
	if !decode(w, r, &in) {
		return
	}
	t, err := s.DB.CreateToken(in)
	if err != nil {
		failStore(w, err)
		return
	}
	w.Header().Set("Location", "/api/tokens/"+t.ID)
	Data(w, http.StatusCreated, t)
}

func (s *Server) putToken(w http.ResponseWriter, r *http.Request) {
	var in store.TokenInput
	if !decode(w, r, &in) {
		return
	}
	in.ID = r.PathValue("id")
	t, err := s.DB.UpsertToken(in, false)
	if err != nil {
		failStore(w, err)
		return
	}
	Data(w, http.StatusOK, t)
}

func (s *Server) patchToken(w http.ResponseWriter, r *http.Request) {
	var in store.TokenPatch
	if !decode(w, r, &in) {
		return
	}
	t, err := s.DB.PatchToken(r.PathValue("id"), in)
	if err != nil {
		failStore(w, err)
		return
	}
	Data(w, http.StatusOK, t)
}

func (s *Server) deleteToken(w http.ResponseWriter, r *http.Request) {
	if err := s.DB.DeleteToken(r.PathValue("id")); err != nil {
		failStore(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) markTokenUsed(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Reading *float64 `json:"reading"`
		At      string   `json:"at"`
		Notes   *string  `json:"notes"`
		Confirm []string `json:"confirm"`
	}
	if !decode(w, r, &in) {
		return
	}
	t, reading, err := s.DB.MarkTokenUsed(r.PathValue("id"), store.ReadingInput{
		Reading: in.Reading, Timestamp: in.At, Notes: in.Notes, Confirm: in.Confirm,
	})
	if err != nil {
		failStore(w, err)
		return
	}
	Data(w, http.StatusOK, map[string]any{"token": t, "reading": reading})
}

func (s *Server) cancelToken(w http.ResponseWriter, r *http.Request) {
	t, cancelled, err := s.DB.CancelToken(r.PathValue("id"))
	if err != nil {
		failStore(w, err)
		return
	}
	Data(w, http.StatusOK, map[string]any{"token": t, "cancelledReadingId": cancelled})
}

func (s *Server) revealToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	code, err := s.DB.RevealTokenCode(id)
	if err != nil {
		failStore(w, err)
		return
	}
	log.Printf("token code revealed: id=%s ip=%s", id, clientIP(r))
	w.Header().Set("Cache-Control", "no-store")
	Data(w, http.StatusOK, map[string]string{"code": code})
}