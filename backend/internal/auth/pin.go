// Package auth implements PIN hashing and the persisted PIN rate limiter.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"

	"meteran/internal/store"
)

// Params are the Argon2id cost parameters. Fixed at build time; changing them
// only affects newly written hashes because the old hash encodes its own.
type Params struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

// Default is tuned for a 2 GB arm64 board: ~50ms per verify.
var Default = Params{Memory: 64 * 1024, Iterations: 3, Parallelism: 2, SaltLen: 16, KeyLen: 32}

// HashPin returns an encoded argon2id hash.
func HashPin(pin string) (string, error) { return HashPinWith(pin, Default) }

func HashPinWith(pin string, p Params) (string, error) {
	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pin), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLen)
	return fmt.Sprintf("argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPin checks pin against an encoded hash in constant time.
func VerifyPin(encoded, pin string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" {
		return false
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	for _, kv := range strings.Split(parts[2], ",") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return false
		}
		n, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return false
		}
		switch k {
		case "m":
			memory = uint32(n)
		case "t":
			iterations = uint32(n)
		case "p":
			parallelism = uint8(n)
		}
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(pin), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// MaxAttempts and Window are fixed by the contract.
const (
	MaxAttempts = 5
	Window      = 15 * time.Minute
)

// Limiter counts failed PIN attempts in SQLite so a restart cannot reset the
// lockout.
type Limiter struct{ DB *store.DB }

// Allow reports whether key may attempt now, and how long until it may retry.
func (l Limiter) Allow(key string) (bool, time.Duration) {
	cutoff := store.FormatTime(time.Now().Add(-Window))
	var n int
	var oldest string
	if err := l.DB.SQL.QueryRow(`select count(*), coalesce(min(at), '') from auth_attempts where key = ? and at > ?`,
		key, cutoff).Scan(&n, &oldest); err != nil {
		return false, Window
	}
	if n < MaxAttempts {
		return true, 0
	}
	if t, err := store.ParseTime(oldest); err == nil {
		return false, time.Until(t.Add(Window))
	}
	return false, Window
}

// Fail records one failed attempt.
func (l Limiter) Fail(key string) error {
	_, err := l.DB.SQL.Exec(`insert into auth_attempts (key, at) values (?, ?)`, key, store.Now())
	return err
}

// Reset clears attempts after a success.
func (l Limiter) Reset(key string) error {
	_, err := l.DB.SQL.Exec(`delete from auth_attempts where key = ?`, key)
	return err
}

// Prune drops rows older than the window.
func (l Limiter) Prune() {
	_, _ = l.DB.SQL.Exec(`delete from auth_attempts where at <= ?`, store.FormatTime(time.Now().Add(-Window)))
}