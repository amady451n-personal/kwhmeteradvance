// Package push sends Web Push (RFC 8291) notifications with a VAPID (RFC 8292)
// JWT. Everything is stdlib: P-256, AES-128-GCM, HKDF and ECDH.
package push

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// VAPID holds the application server key pair. Both halves are the base64url
// strings printed by `web-push generate-vapid-keys`.
type VAPID struct {
	Private *ecdsa.PrivateKey
	Public  string // base64url, 65-byte uncompressed point
	private string // base64url, 32-byte scalar
	public  []byte
}

// LoadOrCreate returns the key pair: env values win, otherwise a generated pair
// is cached at path so existing browser subscriptions keep working after a
// restart (a rotating key would make every push fail).
func LoadOrCreate(public, private, path string) (*VAPID, error) {
	if strings.TrimSpace(public) != "" || strings.TrimSpace(private) != "" {
		return NewVAPID(public, private)
	}
	if raw, err := os.ReadFile(path); err == nil {
		var kp struct{ Public, Private string }
		if json.Unmarshal(raw, &kp) == nil {
			if v, err := NewVAPID(kp.Public, kp.Private); err == nil {
				return v, nil
			}
		}
	}
	v, err := Generate()
	if err != nil {
		return nil, err
	}
	blob, _ := json.Marshal(struct{ Public, Private string }{v.Public, v.private})
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, blob, 0o600); err != nil {
		return nil, err
	}
	return v, nil
}

// Generate mints a fresh P-256 key pair.
func Generate() (*VAPID, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	pub := elliptic.Marshal(elliptic.P256(), key.PublicKey.X, key.PublicKey.Y)
	priv := key.D.FillBytes(make([]byte, 32))
	return NewVAPID(base64.RawURLEncoding.EncodeToString(pub), base64.RawURLEncoding.EncodeToString(priv))
}

func decodeKey(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
}

// NewVAPID parses the environment keys. A missing or malformed key is an error
// so push stays off instead of failing at subscribe time.
func NewVAPID(public, private string) (*VAPID, error) {
	raw, err := decodeKey(private)
	if err != nil {
		return nil, fmt.Errorf("vapid private key: %w", err)
	}
	if len(raw) != 32 {
		return nil, fmt.Errorf("vapid private key must be 32 bytes, got %d", len(raw))
	}
	d := new(big.Int).SetBytes(raw)
	if d.Sign() == 0 || d.Cmp(elliptic.P256().Params().N) >= 0 {
		return nil, errors.New("vapid private key out of range")
	}
	priv := &ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: elliptic.P256()}, D: d}
	priv.PublicKey.X, priv.PublicKey.Y = elliptic.P256().ScalarBaseMult(raw)

	pub, err := decodeKey(public)
	if err != nil {
		return nil, fmt.Errorf("vapid public key: %w", err)
	}
	if len(pub) != 65 || pub[0] != 0x04 {
		return nil, errors.New("vapid public key must be a 65-byte uncompressed P-256 point")
	}
	derived := elliptic.Marshal(elliptic.P256(), priv.PublicKey.X, priv.PublicKey.Y)
	if !equal(derived, pub) {
		return nil, errors.New("vapid public key does not match the private key")
	}
	return &VAPID{Private: priv, Public: strings.TrimSpace(public), private: strings.TrimSpace(private), public: pub}, nil
}

func equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Audience is the scheme plus host of the endpoint, as RFC 8292 requires.
func audience(endpoint string) string {
	dsi := strings.Index(endpoint, "://")
	if dsi < 0 {
		return endpoint
	}
	rest := endpoint[dsi+3:]
	if slash := strings.IndexAny(rest, "/?#"); slash >= 0 {
		return endpoint[:dsi+3+slash]
	}
	return endpoint
}

// Token signs a VAPID JWT valid for 12 hours.
func (v *VAPID) Token(endpoint string, now time.Time) (string, error) {
	header, _ := json.Marshal(map[string]string{"typ": "JWT", "alg": "ES256"})
	claims, _ := json.Marshal(map[string]any{
		"aud": audience(endpoint),
		"exp": now.Add(12 * time.Hour).Unix(),
		"sub": "mailto:meteran@localhost",
	})
	signing := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(signing))
	r, s, err := ecdsa.Sign(rand.Reader, v.Private, digest[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}
