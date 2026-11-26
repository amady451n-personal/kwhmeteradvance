package push

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"meteran/internal/store"
)

type vapidFixture struct {
	VAPID *VAPID
	pub   []byte
	priv  []byte
}

func newVAPIDFixture(t *testing.T) vapidFixture {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	priv := key.D.FillBytes(make([]byte, 32))
	pub := elliptic.Marshal(elliptic.P256(), key.PublicKey.X, key.PublicKey.Y)
	v, err := NewVAPID(b64(pub), b64(priv))
	if err != nil {
		t.Fatalf("NewVAPID: %v", err)
	}
	return vapidFixture{VAPID: v, pub: pub, priv: priv}
}

func b64(raw []byte) string { return base64.RawURLEncoding.EncodeToString(raw) }

func TestNewVAPIDRejectsBadKeys(t *testing.T) {
	f := newVAPIDFixture(t)
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherPub := elliptic.Marshal(elliptic.P256(), other.PublicKey.X, other.PublicKey.Y)

	cases := []struct {
		name            string
		public, private string
	}{
		{"empty", "", ""},
		{"short private", b64(f.pub), b64(f.priv[:16])},
		{"short public", b64(f.pub[:32]), b64(f.priv)},
		{"mismatch", b64(otherPub), b64(f.priv)},
	}
	for _, tc := range cases {
		if _, err := NewVAPID(tc.public, tc.private); err == nil {
			t.Errorf("%s: expected error", tc.name)
		}
	}
}

func TestVAPIDTokenVerifies(t *testing.T) {
	f := newVAPIDFixture(t)
	issued := time.Unix(1_800_000_000, 0)
	token, err := f.VAPID.Token("https://push.example/send/abc", issued)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("jwt parts: %d", len(parts))
	}
	claims, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(claims, &got); err != nil {
		t.Fatal(err)
	}
	if got["aud"] != "https://push.example" {
		t.Errorf("aud = %v", got["aud"])
	}
	if want := float64(issued.Add(12 * time.Hour).Unix()); got["exp"] != want {
		t.Errorf("exp = %v want %v", got["exp"], want)
	}

	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(sig) != 64 {
		t.Fatalf("sig decode: %v len %d", err, len(sig))
	}
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(&f.VAPID.Private.PublicKey, digest[:], r, s) {
		t.Fatal("signature does not verify")
	}
}

type uaFixture struct {
	p256dh string
	auth   string
	priv   *ecdh.PrivateKey
}

func newUA(t *testing.T) uaFixture {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	seed := make([]byte, 16)
	if _, err := rand.Read(seed); err != nil {
		t.Fatal(err)
	}
	return uaFixture{p256dh: b64(key.PublicKey().Bytes()), auth: b64(seed), priv: key}
}

// decryptUA replays the receiving side of RFC 8291.
func decryptUA(t *testing.T, ua uaFixture, body []byte) []byte {
	t.Helper()
	if len(body) < 21+65 {
		t.Fatalf("body too short: %d", len(body))
	}
	salt := body[:16]
	idlen := int(body[20])
	asPub := body[21 : 21+idlen]
	ct := body[21+idlen:]

	asKey, err := ecdh.P256().NewPublicKey(asPub)
	if err != nil {
		t.Fatal(err)
	}
	shared, err := ua.priv.ECDH(asKey)
	if err != nil {
		t.Fatal(err)
	}
	authSecret, err := base64.RawURLEncoding.DecodeString(ua.auth)
	if err != nil {
		t.Fatal(err)
	}
	authInfo := append([]byte("WebPush: info\x00"), ua.priv.PublicKey().Bytes()...)
	authInfo = append(authInfo, asPub...)
	ikm, err := hkdf.Key(sha256.New, shared, authSecret, string(authInfo), 32)
	if err != nil {
		t.Fatal(err)
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		t.Fatal(err)
	}
	cek, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		t.Fatal(err)
	}
	nonce, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return plain
}

func TestEncryptRoundTrip(t *testing.T) {
	ua := newUA(t)
	payload := []byte(`{"title":"Waktunya catat meter","body":"tes"}`)
	body, err := Encrypt(ua.p256dh, ua.auth, payload)
	if err != nil {
		t.Fatal(err)
	}
	if rs := binary.BigEndian.Uint32(body[16:20]); rs != recordSize {
		t.Errorf("record size = %d", rs)
	}
	plain := decryptUA(t, ua, body)
	if plain[len(plain)-1] != 0x02 {
		t.Errorf("missing padding delimiter, got %d", plain[len(plain)-1])
	}
	if string(plain[:len(plain)-1]) != string(payload) {
		t.Errorf("payload = %q", plain[:len(plain)-1])
	}
}

func TestEncryptRejectsBadSecrets(t *testing.T) {
	ua := newUA(t)
	if _, err := Encrypt("not base64!", ua.auth, []byte("x")); err == nil {
		t.Error("expected p256dh error")
	}
	if _, err := Encrypt(ua.p256dh, b64([]byte("short")), []byte("x")); err == nil {
		t.Error("expected auth length error")
	}
	if _, err := Encrypt(ua.p256dh, ua.auth, make([]byte, recordSize)); err == nil {
		t.Error("expected payload size error")
	}
}

func TestReminderDue(t *testing.T) {
	now := time.Date(2026, 11, 26, 8, 30, 0, 0, wib)
	old := now.Add(-30 * time.Hour).UnixMilli()
	fresh := now.Add(-2 * time.Hour).UnixMilli()
	cases := []struct {
		name     string
		hour     int
		latest   *int64
		lastPush string
		want     bool
	}{
		{"due", 8, &old, "", true},
		{"before hour", 9, &old, "", false},
		{"already sent", 8, &old, "2026-11-26", false},
		{"fresh reading", 8, &fresh, "", false},
		{"no readings", 8, nil, "", true},
		{"disabled", -1, &old, "", false},
	}
	for _, tc := range cases {
		if got := ReminderDue(now, tc.hour, tc.latest, tc.lastPush); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

func TestPayloadMentionsDate(t *testing.T) {
	var got map[string]string
	if err := json.Unmarshal(Payload("2026-11-22T15:20:00+07:00"), &got); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got["body"], "22 Nov 2026") {
		t.Errorf("body = %q", got["body"])
	}
	if got["title"] == "" || got["url"] == "" {
		t.Errorf("payload = %v", got)
	}
}

func TestTickSendsOnceAndDropsGone(t *testing.T) {
	db := testDB(t)
	ua := newUA(t)

	var posts int32
	var gotBody []byte
	status := int32(http.StatusCreated)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&posts, 1)
		if !strings.HasPrefix(r.Header.Get("Authorization"), "vapid t=") {
			t.Error("missing vapid auth header")
		}
		if r.Header.Get("Content-Encoding") != "aes128gcm" {
			t.Errorf("encoding = %q", r.Header.Get("Content-Encoding"))
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(int(atomic.LoadInt32(&status)))
	}))
	defer server.Close()

	if _, err := db.Subscribe(server.URL+"/sub", ua.p256dh, ua.auth); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkPushDay("2026-11-25"); err != nil {
		t.Fatal(err)
	}

	client := &Client{VAPID: newVAPIDFixture(t).VAPID, HTTP: server.Client(), TTL: 60}
	now := time.Date(2026, 11, 26, 9, 0, 0, 0, wib)
	if err := client.Tick(context.Background(), db, now); err != nil {
		t.Fatalf("tick: %v", err)
	}
	if atomic.LoadInt32(&posts) == 0 {
		t.Fatal("no push sent")
	}
	if day, _ := db.LastPushDay(); day != "2026-11-26" {
		t.Errorf("last push day = %q", day)
	}
	plain := decryptUA(t, ua, gotBody)
	if !strings.Contains(string(plain), "Waktunya catat meter") {
		t.Errorf("payload = %q", plain)
	}

	atomic.StoreInt32(&posts, 0)
	if err := client.Tick(context.Background(), db, now.Add(10*time.Minute)); err != nil {
		t.Fatalf("second tick: %v", err)
	}
	if n := atomic.LoadInt32(&posts); n != 0 {
		t.Errorf("second tick sent %d pushes", n)
	}

	atomic.StoreInt32(&status, http.StatusGone)
	if err := client.Tick(context.Background(), db, now.Add(24*time.Hour)); err != nil {
		t.Fatalf("third tick: %v", err)
	}
	if n, _ := db.CountSubscriptions(); n != 0 {
		t.Errorf("subscriptions after 410 = %d", n)
	}
	if settings, _ := db.GetSettings(); settings.PushEnabled {
		t.Error("push_enabled still true without subscriptions")
	}
}

func TestTickSkipsBeforeReminderHour(t *testing.T) {
	db := testDB(t)
	ua := newUA(t)
	var posts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&posts, 1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()
	if _, err := db.Subscribe(server.URL+"/sub", ua.p256dh, ua.auth); err != nil {
		t.Fatal(err)
	}
	settings, err := db.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	client := &Client{VAPID: newVAPIDFixture(t).VAPID, HTTP: server.Client()}
	early := time.Date(2026, 11, 26, settings.ReminderHour-1, 0, 0, 0, wib)
	if err := client.Tick(context.Background(), db, early); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&posts); n != 0 {
		t.Errorf("sent %d pushes before reminder hour", n)
	}
}

func TestLoadOrCreatePersistsKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "vapid.json")
	first, err := LoadOrCreate("", "", path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(first.public) != 65 || first.public[0] != 0x04 {
		t.Fatalf("generated public key looks wrong: %d bytes", len(first.public))
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("cache file mode = %v, %v", info, err)
	}
	second, err := LoadOrCreate("", "", path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if second.Public != first.Public {
		t.Fatalf("key rotated on restart: %s vs %s", first.Public, second.Public)
	}

	// Env wins even when a cache file exists.
	env, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	got, err := LoadOrCreate(env.Public, env.private, path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Public != env.Public {
		t.Fatalf("env key ignored: %s", got.Public)
	}
	if _, err := LoadOrCreate("only-public", "", path); err == nil {
		t.Error("expected error for half-configured env keys")
	}
}

func testDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
