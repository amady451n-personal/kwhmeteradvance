package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"meteran/internal/store"
)

func testServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	s := &Server{DB: db, Version: "test", Secret: []byte("0123456789abcdef0123456789abcdef")}
	return s, s.Handler()
}

func do(t *testing.T, h http.Handler, method, path string, body any, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeData(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("body %s: %v", rec.Body.String(), err)
	}
	if err := json.Unmarshal(env.Data, dst); err != nil {
		t.Fatalf("data %s: %v", string(env.Data), err)
	}
}

func setupPin(t *testing.T, h http.Handler) *http.Cookie {
	t.Helper()
	rec := do(t, h, "PUT", "/api/session/pin", map[string]any{"newPin": "1234"}, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("set pin: %d %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("setting the PIN must create a session")
	}
	return cookies[0]
}

func past(d time.Duration) string {
	return store.FormatTime(time.Now().Add(-d))
}

func TestHealthAndHomepageArePublic(t *testing.T) {
	_, h := testServer(t)
	rec := do(t, h, "GET", "/api/health", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("health: %d", rec.Code)
	}
	var health struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	decodeData(t, rec, &health)
	if health.Status != "ok" || health.Version != "test" {
		t.Fatalf("health = %+v", health)
	}

	rec = do(t, h, "GET", "/api/homepage", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("homepage: %d %s", rec.Code, rec.Body.String())
	}
	var home struct {
		Label   string `json:"label"`
		Targets []struct {
			Name      string `json:"name"`
			Formatted string `json:"formatted"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &home); err != nil {
		t.Fatal(err)
	}
	if home.Label != "kWh Tracker" || len(home.Targets) != 4 || home.Targets[3].Formatted != "N/A" {
		t.Fatalf("homepage = %+v", home)
	}
}

func TestAuthGateAndSessionLifecycle(t *testing.T) {
	_, h := testServer(t)
	if rec := do(t, h, "GET", "/api/readings", nil, nil); rec.Code != 200 {
		t.Fatalf("before the first PIN the API is open for setup: %d", rec.Code)
	}
	cookie := setupPin(t, h)
	if rec := do(t, h, "GET", "/api/readings", nil, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after PIN, no cookie must be 401, got %d", rec.Code)
	}
	if rec := do(t, h, "GET", "/api/readings", nil, cookie); rec.Code != 200 {
		t.Fatalf("with cookie: %d %s", rec.Code, rec.Body.String())
	}

	// Wrong PIN is rejected and counted.
	bad := do(t, h, "POST", "/api/session", map[string]string{"pin": "9999"}, nil)
	if bad.Code != http.StatusUnauthorized {
		t.Fatalf("wrong pin: %d", bad.Code)
	}
	good := do(t, h, "POST", "/api/session", map[string]string{"pin": "1234"}, nil)
	if good.Code != http.StatusNoContent || len(good.Result().Cookies()) == 0 {
		t.Fatalf("login: %d", good.Code)
	}

	rec := do(t, h, "GET", "/api/session", nil, cookie)
	var state struct {
		Authenticated bool    `json:"authenticated"`
		PinSet        bool    `json:"pinSet"`
		ExpiresAt     *string `json:"expiresAt"`
	}
	decodeData(t, rec, &state)
	if !state.Authenticated || !state.PinSet || state.ExpiresAt == nil {
		t.Fatalf("session = %+v", state)
	}

	logout := do(t, h, "DELETE", "/api/session", nil, cookie)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", logout.Code)
	}
	if rec := do(t, h, "GET", "/api/readings", nil, cookie); rec.Code != http.StatusUnauthorized {
		t.Fatalf("after logout: %d", rec.Code)
	}
}

func TestReadingSummaryUsageAndSettings(t *testing.T) {
	_, h := testServer(t)
	cookie := setupPin(t, h)

	// First reading.
	rec := do(t, h, "POST", "/api/readings", map[string]any{
		"reading": 100, "timestamp": past(48 * time.Hour),
	}, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created store.Reading
	decodeData(t, rec, &created)
	if created.ID == "" || created.Source != "manual" || created.MeterName != "main" {
		t.Fatalf("created = %+v", created)
	}
	if loc := rec.Header().Get("Location"); loc != "/api/readings/"+created.ID {
		t.Fatalf("location = %q", loc)
	}

	// Growth without a top-up needs explicit confirmation.
	grow := map[string]any{"reading": 180, "timestamp": past(24 * time.Hour)}
	rec = do(t, h, "POST", "/api/readings", grow, cookie)
	if rec.Code != http.StatusConflict {
		t.Fatalf("soft violation: %d %s", rec.Code, rec.Body.String())
	}
	var conflict struct {
		Code    string `json:"code"`
		Details []struct {
			Code string `json:"code"`
		} `json:"details"`
	}
	var errEnv struct {
		Error json.RawMessage `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &errEnv)
	json.Unmarshal(errEnv.Error, &conflict)
	if conflict.Code != CodeConfirmationRequired || conflict.Details[0].Code != ConfirmIncreasedWithoutTopup {
		t.Fatalf("conflict = %+v", conflict)
	}
	grow["confirm"] = []string{ConfirmIncreasedWithoutTopup}
	if rec = do(t, h, "POST", "/api/readings", grow, cookie); rec.Code != http.StatusCreated {
		t.Fatalf("confirmed create: %d %s", rec.Code, rec.Body.String())
	}

	// Upsert by client id is idempotent.
	put := do(t, h, "PUT", "/api/readings/offline-1", map[string]any{
		"reading": 170, "timestamp": past(2 * time.Hour), "confirm": []string{ConfirmIncreasedWithoutTopup, ConfirmDuplicateTimestamp},
	}, cookie)
	if put.Code != http.StatusOK {
		t.Fatalf("upsert: %d %s", put.Code, put.Body.String())
	}
	if again := do(t, h, "PUT", "/api/readings/offline-1", map[string]any{
		"reading": 170, "timestamp": past(2 * time.Hour), "confirm": []string{ConfirmIncreasedWithoutTopup, ConfirmDuplicateTimestamp},
	}, cookie); again.Code != http.StatusOK {
		t.Fatalf("replay: %d", again.Code)
	}

	rec = do(t, h, "GET", "/api/readings?limit=2", nil, cookie)
	var page struct {
		Data []store.Reading `json:"data"`
		Meta struct {
			NextCursor *string `json:"nextCursor"`
			HasMore    bool    `json:"hasMore"`
		} `json:"meta"`
	}
	json.Unmarshal(rec.Body.Bytes(), &page)
	if len(page.Data) != 2 || !page.Meta.HasMore || page.Meta.NextCursor == nil {
		t.Fatalf("page = %+v", page)
	}
	if page.Data[0].Reading != 170 {
		t.Fatalf("newest first, got %v", page.Data[0].Reading)
	}

	rec = do(t, h, "GET", "/api/summary?window=7", nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("summary: %d %s", rec.Code, rec.Body.String())
	}
	var sum struct {
		WindowDays    int      `json:"windowDays"`
		DataQuality   string   `json:"dataQuality"`
		AvailableKwh  float64  `json:"availableKwh"`
		LatestReading *struct {
			Reading float64 `json:"reading"`
		} `json:"latestReading"`
		Chart []struct {
			Key string `json:"key"`
		} `json:"chart"`
	}
	decodeData(t, rec, &sum)
	if sum.WindowDays != 7 || sum.DataQuality != "ok" || sum.AvailableKwh != 170 || len(sum.Chart) != 7 {
		t.Fatalf("summary = %+v", sum)
	}
	if sum.LatestReading == nil || sum.LatestReading.Reading != 170 {
		t.Fatalf("latest = %+v", sum.LatestReading)
	}

	rec = do(t, h, "GET", "/api/usage?group=day", nil, cookie)
	if rec.Code != 200 {
		t.Fatalf("usage: %d %s", rec.Code, rec.Body.String())
	}

	rec = do(t, h, "PATCH", "/api/settings", map[string]any{"buffer": 25, "denominations": []int{50000}}, cookie)
	var settings store.Settings
	decodeData(t, rec, &settings)
	if settings.Buffer != 25 || len(settings.Denominations) != 1 || !settings.PinSet {
		t.Fatalf("settings = %+v", settings)
	}
}

func TestTokenFlowAndErrors(t *testing.T) {
	_, h := testServer(t)
	cookie := setupPin(t, h)

	// Cost far from any denomination is a soft violation.
	tok := map[string]any{
		"code": "1234-5678-9012", "kwh": 66, "cost": 12345,
		"purchasedAt": past(5 * time.Hour), "type": "additional",
	}
	rec := do(t, h, "POST", "/api/tokens", tok, cookie)
	if rec.Code != http.StatusConflict {
		t.Fatalf("denomination: %d %s", rec.Code, rec.Body.String())
	}
	tok["confirm"] = []string{ConfirmDenominationOutOfTol}
	rec = do(t, h, "POST", "/api/tokens", tok, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create token: %d %s", rec.Code, rec.Body.String())
	}
	var token store.Token
	decodeData(t, rec, &token)
	if token.Status != "unused" || !token.HasCode || token.CanDelete {
		t.Fatalf("token = %+v", token)
	}

	rec = do(t, h, "POST", "/api/tokens/"+token.ID+"/reveal", nil, cookie)
	var code struct {
		Code string `json:"code"`
	}
	decodeData(t, rec, &code)
	if code.Code != "1234-5678-9012" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("reveal = %+v cc=%q", code, rec.Header().Get("Cache-Control"))
	}

	mark := do(t, h, "POST", "/api/tokens/"+token.ID+"/mark-used", map[string]any{
		"reading": 166, "at": past(4 * time.Hour),
	}, cookie)
	if mark.Code != 200 {
		t.Fatalf("mark used: %d %s", mark.Code, mark.Body.String())
	}
	var used struct {
		Token   store.Token   `json:"token"`
		Reading store.Reading `json:"reading"`
	}
	decodeData(t, mark, &used)
	if used.Token.Status != "used" || used.Reading.TokenID == nil || *used.Reading.TokenID != token.ID {
		t.Fatalf("mark = %+v", used)
	}

	cancel := do(t, h, "POST", "/api/tokens/"+token.ID+"/cancel", nil, cookie)
	if cancel.Code != 200 {
		t.Fatalf("cancel: %d %s", cancel.Code, cancel.Body.String())
	}
	var after struct {
		Token              store.Token `json:"token"`
		CancelledReadingID *string     `json:"cancelledReadingId"`
	}
	decodeData(t, cancel, &after)
	if after.Token.Status != "unused" || after.CancelledReadingID == nil {
		t.Fatalf("cancel = %+v", after)
	}

	// Unknown API path stays JSON, the SPA is not involved.
	rec = do(t, h, "GET", "/api/nope", nil, cookie)
	if rec.Code != http.StatusNotFound || rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("404 = %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
}