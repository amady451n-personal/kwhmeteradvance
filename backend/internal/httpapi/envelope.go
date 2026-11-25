package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"meteran/internal/store"
)

// Detail is one machine-readable reason attached to an error.
type Detail = store.Detail

type envelope struct {
	Data  any `json:"data"`
	Meta  any `json:"meta,omitempty"`
	Links any `json:"links,omitempty"`
}

type errEnvelope struct {
	Error errObject `json:"error"`
}

type errObject struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []Detail `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

// Data writes {"data": v}.
func Data(w http.ResponseWriter, status int, v any) {
	writeJSON(w, status, envelope{Data: v})
}

// DataMeta writes {"data": v, "meta": meta}.
func DataMeta(w http.ResponseWriter, status int, v, meta any) {
	writeJSON(w, status, envelope{Data: v, Meta: meta})
}

// Fail writes {"error": {...}}.
func Fail(w http.ResponseWriter, status int, code, message string, details ...Detail) {
	writeJSON(w, status, errEnvelope{Error: errObject{Code: code, Message: message, Details: details}})
}

// failStore maps store errors onto the contract's error shape.
func failStore(w http.ResponseWriter, err error) {
	var conf store.ErrConfirmation
	var val store.ErrValidation
	switch {
	case errors.As(err, &conf):
		Fail(w, http.StatusConflict, CodeConfirmationRequired, "Perlu konfirmasi sebelum menyimpan.", conf.Details...)
	case errors.As(err, &val):
		Fail(w, http.StatusUnprocessableEntity, CodeValidationError, "Data tidak valid.", val.Details...)
	case errors.Is(err, store.ErrNotFound):
		Fail(w, http.StatusNotFound, CodeNotFound, "Data tidak ditemukan.")
	case errors.Is(err, store.ErrReadingLocked):
		Fail(w, http.StatusConflict, CodeReadingLocked, "Pembacaan tertaut token terkunci dan tidak dapat dihapus.")
	case errors.Is(err, store.ErrTokenLocked):
		Fail(w, http.StatusConflict, CodeTokenLocked, "Token terkunci dan tidak dapat diubah.")
	case errors.Is(err, store.ErrTokenNotDeletable):
		Fail(w, http.StatusConflict, CodeTokenNotDeletable, "Token baru dapat dihapus setelah 50 pengisian berikutnya.")
	case errors.Is(err, store.ErrImportNotAllowed):
		Fail(w, http.StatusConflict, CodeImportNotAllowed, "Impor hanya tersedia saat database masih kosong.")
	default:
		log.Printf("internal error: %v", err)
		Fail(w, http.StatusInternalServerError, CodeInternal, "Terjadi kesalahan pada server.")
	}
}

// decode reads a JSON body. It rejects unknown fields so contract drift shows up
// as a 422 instead of a silently ignored field.
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if ct := r.Header.Get("Content-Type"); ct != "" && !isJSON(ct) {
		Fail(w, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Isi permintaan harus application/json.")
		return false
	}
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		Fail(w, http.StatusUnprocessableEntity, CodeValidationError, "Data tidak valid.")
		return false
	}
	return true
}

func isJSON(ct string) bool {
	for i := 0; i < len(ct); i++ {
		if ct[i] == ';' {
			ct = ct[:i]
			break
		}
	}
	return ct == "application/json"
}