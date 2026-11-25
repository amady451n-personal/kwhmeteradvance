package httpapi

import (
	"net/http"

	"meteran/internal/store"
)

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.DB.GetSettings()
	if err != nil {
		failStore(w, err)
		return
	}
	Data(w, http.StatusOK, st)
}

func (s *Server) patchSettings(w http.ResponseWriter, r *http.Request) {
	var in store.SettingsPatch
	if !decode(w, r, &in) {
		return
	}
	st, err := s.DB.PatchSettings(in)
	if err != nil {
		failStore(w, err)
		return
	}
	Data(w, http.StatusOK, st)
}