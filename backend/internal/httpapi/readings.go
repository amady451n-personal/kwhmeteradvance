package httpapi

import (
	"net/http"
	"strconv"

	"meteran/internal/store"
)

func (s *Server) listReadings(w http.ResponseWriter, r *http.Request) {
	from, to, err := store.NormalizeRange(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		failStore(w, err)
		return
	}
	rows, meta, err := s.DB.ListReadings(store.ReadingQuery{
		From: from, To: to, Limit: atoiDefault(r.URL.Query().Get("limit"), 20), Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		failStore(w, err)
		return
	}
	DataMeta(w, http.StatusOK, rows, meta)
}

func (s *Server) getReading(w http.ResponseWriter, r *http.Request) {
	row, err := s.DB.GetReading(r.PathValue("id"))
	if err != nil {
		failStore(w, err)
		return
	}
	Data(w, http.StatusOK, row)
}

func (s *Server) createReading(w http.ResponseWriter, r *http.Request) {
	var in store.ReadingInput
	if !decode(w, r, &in) {
		return
	}
	if in.ID == "" {
		in.ID = store.NewID("reading")
	}
	row, err := s.DB.UpsertReading(in)
	if err != nil {
		failStore(w, err)
		return
	}
	w.Header().Set("Location", "/api/readings/"+row.ID)
	Data(w, http.StatusCreated, row)
}

func (s *Server) putReading(w http.ResponseWriter, r *http.Request) {
	var in store.ReadingInput
	if !decode(w, r, &in) {
		return
	}
	in.ID = r.PathValue("id")
	row, err := s.DB.UpsertReading(in)
	if err != nil {
		failStore(w, err)
		return
	}
	Data(w, http.StatusOK, row)
}

func (s *Server) deleteReading(w http.ResponseWriter, r *http.Request) {
	if err := s.DB.DeleteReading(r.PathValue("id")); err != nil {
		failStore(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func atoiDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}