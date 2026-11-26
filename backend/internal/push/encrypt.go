package push

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

const recordSize = 4096

// Encrypt builds an aes128gcm body (RFC 8188) for one push subscription,
// keyed with the user agent's p256dh and auth secrets (RFC 8291).
func Encrypt(p256dh, auth string, payload []byte) ([]byte, error) {
	uaPub, err := base64.RawURLEncoding.DecodeString(p256dh)
	if err != nil {
		return nil, fmt.Errorf("p256dh: %w", err)
	}
	authSecret, err := base64.RawURLEncoding.DecodeString(auth)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	if len(authSecret) != 16 {
		return nil, fmt.Errorf("auth secret must be 16 bytes, got %d", len(authSecret))
	}
	uaKey, err := ecdh.P256().NewPublicKey(uaPub)
	if err != nil {
		return nil, fmt.Errorf("p256dh: %w", err)
	}
	asKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	shared, err := asKey.ECDH(uaKey)
	if err != nil {
		return nil, err
	}
	asPub := asKey.PublicKey().Bytes()

	authInfo := append([]byte("WebPush: info\x00"), uaPub...)
	authInfo = append(authInfo, asPub...)
	ikm, err := hkdf.Key(sha256.New, shared, authSecret, string(authInfo), 32)
	if err != nil {
		return nil, err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	prk, err := hkdf.Extract(sha256.New, ikm, salt)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Expand(sha256.New, prk, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}

	// Single record: append the last-record padding delimiter.
	plaintext := append(append([]byte{}, payload...), 0x02)
	if len(plaintext) > recordSize {
		return nil, fmt.Errorf("payload too large: %d bytes", len(payload))
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nil, nonce, plaintext, nil)

	var rs [4]byte
	binary.BigEndian.PutUint32(rs[:], recordSize)
	body := append([]byte{}, salt...)
	body = append(body, rs[:]...)
	body = append(body, byte(len(asPub)))
	body = append(body, asPub...)
	return append(body, sealed...), nil
}
