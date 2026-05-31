package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

// HashPassword hashes password with argon2id. Format: "argon2id:<salt-hex>:<hash-hex>"
func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("rand: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return "argon2id:" + hex.EncodeToString(salt) + ":" + hex.EncodeToString(hash), nil
}

// VerifyPassword checks password against a hash from HashPassword.
func VerifyPassword(stored, password string) bool {
	parts := strings.SplitN(stored, ":", 3)
	if len(parts) != 3 || parts[0] != "argon2id" {
		return false
	}
	salt, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	return subtle.ConstantTimeCompare(got, expected) == 1
}

// SignSession creates an HMAC-SHA256 signed token: base64url(payload)+"."+base64url(mac)
// payload = "username|unixExpiry"
func SignSession(secret []byte, username string, expiry time.Time) (string, error) {
	payload := username + "|" + strconv.FormatInt(expiry.Unix(), 10)
	payloadB64 := base64.URLEncoding.EncodeToString([]byte(payload))
	mac := computeMAC(secret, payloadB64)
	macB64 := base64.URLEncoding.EncodeToString(mac)
	return payloadB64 + "." + macB64, nil
}

// ParseSession verifies and decodes a session token. Returns username on success.
func ParseSession(secret []byte, token string) (string, error) {
	dot := strings.LastIndex(token, ".")
	if dot < 0 {
		return "", errors.New("malformed token")
	}
	payloadB64, macB64 := token[:dot], token[dot+1:]
	gotMAC, err := base64.URLEncoding.DecodeString(macB64)
	if err != nil {
		return "", errors.New("malformed token mac")
	}
	expected := computeMAC(secret, payloadB64)
	if subtle.ConstantTimeCompare(gotMAC, expected) != 1 {
		return "", errors.New("invalid token signature")
	}
	payloadBytes, err := base64.URLEncoding.DecodeString(payloadB64)
	if err != nil {
		return "", errors.New("malformed token payload")
	}
	parts := strings.SplitN(string(payloadBytes), "|", 2)
	if len(parts) != 2 {
		return "", errors.New("malformed token payload")
	}
	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", errors.New("malformed token expiry")
	}
	if time.Now().Unix() > expiry {
		return "", errors.New("token expired")
	}
	return parts[0], nil
}

func computeMAC(secret []byte, data string) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(data))
	return h.Sum(nil)
}
