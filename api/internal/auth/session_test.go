package auth

import (
	"encoding/hex"
	"testing"
)

func TestGenerateToken_Length(t *testing.T) {
	token, err := generateToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// tokenBytes = 32, hex encoded = 64 characters.
	expectedLen := tokenBytes * 2 // 64
	if len(token) != expectedLen {
		t.Errorf("expected token length %d, got %d", expectedLen, len(token))
	}
}

func TestGenerateToken_ValidHex(t *testing.T) {
	token, err := generateToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = hex.DecodeString(token)
	if err != nil {
		t.Errorf("token is not valid hex: %v", err)
	}
}

func TestGenerateToken_Uniqueness(t *testing.T) {
	tokens := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		token, err := generateToken()
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if tokens[token] {
			t.Errorf("duplicate token generated on iteration %d: %s", i, token)
		}
		tokens[token] = true
	}
}

func TestGenerateToken_Is64HexChars(t *testing.T) {
	token, err := generateToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(token) != 64 {
		t.Errorf("expected 64 hex chars, got %d", len(token))
	}

	// Verify all characters are valid hex.
	for i, c := range token {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("invalid hex character at position %d: %c", i, c)
		}
	}
}

func TestSessionConstants(t *testing.T) {
	// Verify exported constants have expected values.
	if tokenBytes != 32 {
		t.Errorf("expected tokenBytes=32, got %d", tokenBytes)
	}

	// Verify default session TTL is 8 hours.
	expectedHours := 8
	if defaultSessionTTL.Hours() != float64(expectedHours) {
		t.Errorf("expected defaultSessionTTL=%d hours, got %v", expectedHours, defaultSessionTTL)
	}
}

func TestErrSessionNotFound(t *testing.T) {
	if ErrSessionNotFound == nil {
		t.Fatal("ErrSessionNotFound should not be nil")
	}
	if ErrSessionNotFound.Error() != "session not found or expired" {
		t.Errorf("unexpected error message: %q", ErrSessionNotFound.Error())
	}
}
