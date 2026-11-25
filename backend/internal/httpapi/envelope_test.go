package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestFailEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	Fail(rec, 409, CodeConfirmationRequired, "Perlu konfirmasi.",
		Detail{Field: "reading", Code: ConfirmIncreasedWithoutTopup, Message: "x"})
	var body struct {
		Error struct {
			Code    string
			Details []struct{ Field, Code string }
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != CodeConfirmationRequired {
		t.Fatalf("code = %q", body.Error.Code)
	}
	if body.Error.Details[0].Code != ConfirmIncreasedWithoutTopup || body.Error.Details[0].Field != "reading" {
		t.Fatalf("details = %+v", body.Error.Details)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", ct)
	}
}

func TestDataEnvelopeOmitsMeta(t *testing.T) {
	rec := httptest.NewRecorder()
	Data(rec, 200, map[string]string{"status": "ok"})
	if got := rec.Body.String(); got != "{\"data\":{\"status\":\"ok\"}}\n" {
		t.Fatalf("body = %s", got)
	}
}

func TestFailOmitsEmptyDetails(t *testing.T) {
	rec := httptest.NewRecorder()
	Fail(rec, 404, CodeNotFound, "x")
	if got := rec.Body.String(); got != "{\"error\":{\"code\":\"not_found\",\"message\":\"x\"}}\n" {
		t.Fatalf("body = %s", got)
	}
}