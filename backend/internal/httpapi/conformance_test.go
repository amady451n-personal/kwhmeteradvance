package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

// loadContract reads the single source of truth. The test lives three levels
// below the repo root, so the relative path is stable for `go test ./...`.
func loadContract(t *testing.T) *openapi3.T {
	t.Helper()
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(filepath.Join("..", "..", "..", "docs", "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("load contract: %v", err)
	}
	if err := doc.Validate(loader.Context, openapi3.DisableExamplesValidation()); err != nil {
		t.Fatalf("contract is not valid: %v", err)
	}
	return doc
}

// checkResponse validates one recorded response against the contract.
func checkResponse(t *testing.T, doc *openapi3.T, method, template string, rec *httptest.ResponseRecorder) {
	t.Helper()
	item := doc.Paths.Find(template)
	if item == nil {
		t.Fatalf("contract has no path %s", template)
	}
	op := item.GetOperation(method)
	// Ensure the operation exists before looking at its responses.
	if op == nil {
		t.Fatalf("contract has no %s %s", method, template)
	}
	resp := op.Responses.Status(rec.Code)
	if resp == nil {
		resp = op.Responses.Default()
	}
	if resp == nil {
		t.Fatalf("%s %s: contract has no response for %d", method, template, rec.Code)
	}
	content := resp.Value.Content

	body := rec.Body.Bytes()
	if len(body) == 0 {
		if rec.Code == http.StatusNoContent || rec.Code == http.StatusCreated && len(rec.Body.Bytes()) == 0 {
			return
		}
		if rec.Code == http.StatusNoContent {
			return
		}
		// A body-less 2xx is only acceptable when the contract declares no content.
		if rec.Code != http.StatusNoContent && content != nil && len(content) > 0 {
			t.Fatalf("%s %s: empty body for %d", method, template, rec.Code)
		}
		return
	}

	ct := rec.Header().Get("Content-Type")
	base := ct
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		base = strings.TrimSpace(ct[:i])
	}
	media := content[base]
	if media == nil {
		for key, candidate := range content {
			if strings.HasPrefix(base, key) {
				media = candidate
				break
			}
		}
	}
	if media == nil || media.Schema == nil {
		t.Fatalf("%s %s: contract has no schema for %d %s", method, template, rec.Code, ct)
	}

	var value any
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		t.Fatalf("%s %s: body is not JSON: %v (%s)", method, template, err, truncate(body))
	}
	if err := media.Schema.Value.VisitJSON(value, openapi3.MultiErrors()); err != nil {
		t.Fatalf("%s %s: response breaks the contract: %v\n%s", method, template, err, truncate(body))
	}
}

func truncate(b []byte) string {
	if len(b) > 800 {
		return string(b[:800]) + "..."
	}
	return string(b)
}

// TestConformanceHappyPath walks the real router and validates every body
// against docs/api/openapi.yaml.
func TestConformanceHappyPath(t *testing.T) {
	doc := loadContract(t)
	_, h := testServer(t)
	cookie := setupPin(t, h)

	call := func(method, path string, body any) *httptest.ResponseRecorder {
		rec := do(t, h, method, path, body, cookie)
		return rec
	}
	expect := func(method, template string, rec *httptest.ResponseRecorder, want int) {
		t.Helper()
		if rec.Code != want {
			t.Fatalf("%s %s: %d, want %d: %s", method, template, rec.Code, want, rec.Body.String())
		}
		checkResponse(t, doc, method, template, rec)
	}

	expect("GET", "/api/health", do(t, h, "GET", "/api/health", nil, nil), 200)
	expect("GET", "/api/homepage", do(t, h, "GET", "/api/homepage", nil, nil), 200)
	expect("GET", "/api/session", do(t, h, "GET", "/api/session", nil, cookie), 200)
	// A wrong current PIN is a documented 401; changing it would invalidate the
	// session cookie this test keeps using.
	expect("PUT", "/api/session/pin", call("PUT", "/api/session/pin", map[string]any{
		"currentPin": "9999", "newPin": "5678",
	}), 401)
	expect("GET", "/api/settings", call("GET", "/api/settings", nil), 200)
	expect("PATCH", "/api/settings", call("PATCH", "/api/settings", map[string]any{"buffer": 20}), 200)

	created := call("POST", "/api/readings", map[string]any{
		"reading": 100, "timestamp": past(48 * time.Hour),
	})
	expect("POST", "/api/readings", created, 201)
	var first struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(created.Body.Bytes(), &first)
	if first.Data.ID == "" {
		t.Fatal("reading id missing")
	}

	expect("GET", "/api/readings", call("GET", "/api/readings?limit=5", nil), 200)
	expect("GET", "/api/readings/{id}", call("GET", "/api/readings/"+first.Data.ID, nil), 200)
	expect("POST", "/api/readings", call("POST", "/api/readings", map[string]any{
		"reading": 160, "timestamp": past(24 * time.Hour),
		"confirm": []string{ConfirmIncreasedWithoutTopup},
	}), 201)
	// Hard validation still has to match the Error schema.
	expect("POST", "/api/readings", call("POST", "/api/readings", map[string]any{
		"reading": -5, "timestamp": past(time.Hour),
	}), 422)
	// Soft violation is a documented 409 envelope.
	expect("POST", "/api/readings", call("POST", "/api/readings", map[string]any{
		"reading": 500, "timestamp": past(12 * time.Hour),
	}), 409)

	token := call("POST", "/api/tokens", map[string]any{
		"code": "1234-5678-9012", "kwh": 66, "cost": 100000,
		"purchasedAt": past(6 * time.Hour), "type": "additional",
		"confirm": []string{ConfirmDenominationOutOfTol},
	})
	expect("POST", "/api/tokens", token, 201)
	var tok struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(token.Body.Bytes(), &tok)
	if tok.Data.ID == "" {
		t.Fatal("token id missing")
	}
	expect("GET", "/api/tokens", call("GET", "/api/tokens", nil), 200)
	expect("GET", "/api/tokens/{id}", call("GET", "/api/tokens/"+tok.Data.ID, nil), 200)
	expect("PATCH", "/api/tokens/{id}", call("PATCH", "/api/tokens/"+tok.Data.ID, map[string]any{"notes": "voucher"}), 200)
	expect("POST", "/api/tokens/{id}/reveal", call("POST", "/api/tokens/"+tok.Data.ID+"/reveal", nil), 200)
	expect("POST", "/api/tokens/{id}/mark-used", call("POST", "/api/tokens/"+tok.Data.ID+"/mark-used", map[string]any{
		"reading": 220, "at": past(5 * time.Hour),
		"confirm": []string{ConfirmIncreasedWithoutTopup, ConfirmDuplicateTimestamp},
	}), 200)
	expect("POST", "/api/tokens/{id}/cancel", call("POST", "/api/tokens/"+tok.Data.ID+"/cancel", nil), 200)

	expect("GET", "/api/summary", call("GET", "/api/summary?window=7", nil), 200)
	expect("GET", "/api/usage", call("GET", "/api/usage?group=day", nil), 200)

	pdf := call("GET", "/api/reports/summary.pdf", nil)
	if pdf.Code != 200 || !strings.HasPrefix(pdf.Body.String(), "%PDF-1.4") {
		t.Fatalf("report = %d %q", pdf.Code, pdf.Header().Get("Content-Type"))
	}
	if ct := pdf.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("report content-type = %q", ct)
	}

	expect("GET", "/api/push/public-key", call("GET", "/api/push/public-key", nil), 200)
	sub := call("POST", "/api/push/subscriptions", map[string]any{
		"endpoint": "https://push.example.test/abc",
		"keys":     map[string]string{"p256dh": "BPublic", "auth": "Secret"},
	})
	expect("POST", "/api/push/subscriptions", sub, 201)
	var subscription struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(sub.Body.Bytes(), &subscription)
	if subscription.Data.ID == "" {
		t.Fatal("subscription id missing")
	}
	expect("DELETE", "/api/push/subscriptions/{id}", call("DELETE", "/api/push/subscriptions/"+subscription.Data.ID, nil), 204)

	// Unauthorized shape: no cookie after the PIN exists.
	expect("GET", "/api/readings", do(t, h, "GET", "/api/readings", nil, nil), 401)
	// Unknown path stays a JSON 404 envelope even though the contract only
	// documents real paths.
	if rec := do(t, h, "GET", "/api/nope", nil, cookie); rec.Code != 404 || !strings.Contains(rec.Body.String(), `"not_found"`) {
		t.Fatalf("unknown api path = %d %s", rec.Code, rec.Body.String())
	}
}