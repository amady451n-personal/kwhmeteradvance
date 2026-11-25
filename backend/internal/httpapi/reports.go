package httpapi

import (
	"net/http"
	"strings"

	"meteran/internal/domain"
	"meteran/internal/report"
	"meteran/internal/store"
)

func (s *Server) reportPDF(w http.ResponseWriter, r *http.Request) {
	in, err := s.summaryInput(r)
	if err != nil {
		failStore(w, err)
		return
	}
	sum := domain.Summarize(in)
	readings, err := s.DB.AllReadings()
	if err != nil {
		failStore(w, err)
		return
	}
	from, to, err := store.NormalizeRange(r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		failStore(w, err)
		return
	}
	if from != "" || to != "" {
		filtered := make([]store.Reading, 0, len(readings))
		for _, row := range readings {
			if (from == "" || row.Timestamp >= from) && (to == "" || row.Timestamp < to) {
				filtered = append(filtered, row)
			}
		}
		readings = filtered
	}
	name := "meteran"
	if from != "" {
		name += "-" + from[:7]
	}
	pdf := report.SummaryPDF(sum, readings, from, to)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(name, "/", "-")+`.pdf"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}