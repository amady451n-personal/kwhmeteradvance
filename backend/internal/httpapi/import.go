package httpapi

import (
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"meteran/internal/importer"
	"meteran/internal/store"
)

type pendingImport struct {
	preview importer.Preview
	expires time.Time
}

func (s *Server) importPreview(w http.ResponseWriter, r *http.Request) {
	if !isDBEmpty(s.DB) {
		Fail(w, http.StatusConflict, CodeImportNotAllowed, "Impor hanya tersedia saat database masih kosong.")
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		Fail(w, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Unggahan harus multipart/form-data.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	file, _, err := r.FormFile("file")
	if err != nil {
		Fail(w, http.StatusUnprocessableEntity, CodeValidationError, "Data tidak valid.",
			Detail{Field: "file", Code: "required", Message: "Berkas database wajib diunggah."})
		return
	}
	defer file.Close()
	tmp, err := os.CreateTemp("", "meteran-legacy-*.db")
	if err != nil {
		failStore(w, err)
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		failStore(w, err)
		return
	}
	tmp.Close()

	preview, err := importer.Scan(tmp.Name())
	if err != nil {
		Fail(w, http.StatusUnprocessableEntity, CodeValidationError, "Data tidak valid.",
			Detail{Field: "file", Code: "invalid_database", Message: "Berkas bukan database meteran basic yang valid."})
		return
	}

	id := store.NewID("import")
	expires := time.Now().Add(10 * time.Minute)
	s.mu.Lock()
	s.pruneImportsLocked()
	s.pending[id] = pendingImport{preview: preview, expires: expires}
	s.mu.Unlock()

	Data(w, http.StatusOK, map[string]any{
		"previewId": id,
		"expiresAt": store.FormatTime(expires),
		"source": map[string]int{
			"rows": preview.Rows, "mainRows": preview.MainRows, "otherMeterRows": preview.OtherMeterRows,
		},
		"groups": map[string]any{
			"valid":             preview.Valid,
			"needsConfirmation": preview.NeedsConfirm,
			"rejected":          preview.Rejected,
		},
		"stats": preview.Stats,
	})
}

func (s *Server) importCommit(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PreviewID string `json:"previewId"`
		Confirm   bool   `json:"confirm"`
	}
	if !decode(w, r, &in) {
		return
	}
	s.mu.Lock()
	pending, ok := s.pending[in.PreviewID]
	delete(s.pending, in.PreviewID)
	s.mu.Unlock()
	if !ok || time.Now().After(pending.expires) {
		Fail(w, http.StatusConflict, CodeConflict, "Pratinjau impor kedaluwarsa. Ulangi pratinjau.")
		return
	}
	if !in.Confirm {
		Data(w, http.StatusOK, map[string]any{
			"imported": importer.Stats{},
			"verification": map[string]any{
				"rowsMatch": true, "kwhMatch": true, "costMatch": true, "source": pending.preview.Stats,
			},
		})
		return
	}
	imported, err := importer.Commit(s.DB, pending.preview)
	if err != nil {
		failStore(w, err)
		return
	}
	if err := s.DB.MarkImported(); err != nil {
		failStore(w, err)
		return
	}
	src := pending.preview.Stats
	Data(w, http.StatusOK, map[string]any{
		"imported": imported,
		"verification": map[string]any{
			"rowsMatch": imported.Rows == src.Rows,
			"kwhMatch":  abs(imported.TopupKwh-src.TopupKwh) < 0.01,
			"costMatch": imported.TopupCost == src.TopupCost,
			"source":    src,
		},
	})
}

func (s *Server) pruneImportsLocked() {
	now := time.Now()
	for id, p := range s.pending {
		if now.After(p.expires) {
			delete(s.pending, id)
		}
	}
}

func isDBEmpty(db *store.DB) bool {
	var n int
	if err := db.SQL.QueryRow(`select count(*) from readings`).Scan(&n); err != nil {
		return false
	}
	if n > 0 {
		return false
	}
	if err := db.SQL.QueryRow(`select count(*) from tokens`).Scan(&n); err != nil {
		return false
	}
	return n == 0
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}