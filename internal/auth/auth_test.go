package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/t0mer/SwitchDeck/internal/auth"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := auth.HashPassword("correct-horse")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "argon2id:") {
		t.Errorf("hash should start with argon2id:")
	}
	if !auth.VerifyPassword(hash, "correct-horse") {
		t.Error("VerifyPassword should return true for correct password")
	}
	if auth.VerifyPassword(hash, "wrong") {
		t.Error("VerifyPassword should return false for wrong password")
	}
}

func TestSessionRoundtrip(t *testing.T) {
	secret := []byte("test-secret-32-bytes-exactly!!!!!")
	token, err := auth.SignSession(secret, "admin", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("SignSession: %v", err)
	}
	user, err := auth.ParseSession(secret, token)
	if err != nil {
		t.Fatalf("ParseSession: %v", err)
	}
	if user != "admin" {
		t.Errorf("username: got %q, want admin", user)
	}
}

func TestSessionExpired(t *testing.T) {
	secret := []byte("test-secret-32-bytes-exactly!!!!!")
	token, _ := auth.SignSession(secret, "admin", time.Now().Add(-time.Second))
	if _, err := auth.ParseSession(secret, token); err == nil {
		t.Error("expected error for expired session")
	}
}

func TestSessionTampered(t *testing.T) {
	secret := []byte("test-secret-32-bytes-exactly!!!!!")
	token, _ := auth.SignSession(secret, "admin", time.Now().Add(time.Hour))
	tampered := token[:len(token)-4] + "XXXX"
	if _, err := auth.ParseSession(secret, tampered); err == nil {
		t.Error("expected error for tampered token")
	}
}
