package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	t.Run("hash and verify match", func(t *testing.T) {
		password := "correcthorsebatterystaple"

		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword returned error: %v", err)
		}

		if hash == "" {
			t.Fatal("HashPassword returned empty hash")
		}

		// Hash should be a valid bcrypt hash.
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
			t.Fatalf("bcrypt.CompareHashAndPassword failed: %v", err)
		}

		// VerifyPassword should succeed.
		if err := VerifyPassword(hash, password); err != nil {
			t.Fatalf("VerifyPassword returned error for correct password: %v", err)
		}
	})

	t.Run("verify with wrong password fails", func(t *testing.T) {
		password := "correcthorsebatterystaple"
		wrongPassword := "wrongpassword"

		hash, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword returned error: %v", err)
		}

		err = VerifyPassword(hash, wrongPassword)
		if err == nil {
			t.Fatal("VerifyPassword should return error for wrong password")
		}
	})

	t.Run("empty password returns error", func(t *testing.T) {
		_, err := HashPassword("")
		if err == nil {
			t.Fatal("HashPassword should return error for empty password")
		}

		if err != ErrEmptyPassword {
			t.Fatalf("expected ErrEmptyPassword, got: %v", err)
		}
	})

	t.Run("verify empty password returns error", func(t *testing.T) {
		hash, err := HashPassword("somepassword")
		if err != nil {
			t.Fatalf("HashPassword returned error: %v", err)
		}

		err = VerifyPassword(hash, "")
		if err == nil {
			t.Fatal("VerifyPassword should return error for empty password")
		}

		if err != ErrEmptyPassword {
			t.Fatalf("expected ErrEmptyPassword, got: %v", err)
		}
	})

	t.Run("different passwords produce different hashes", func(t *testing.T) {
		hash1, err := HashPassword("password1")
		if err != nil {
			t.Fatalf("HashPassword returned error: %v", err)
		}

		hash2, err := HashPassword("password2")
		if err != nil {
			t.Fatalf("HashPassword returned error: %v", err)
		}

		if hash1 == hash2 {
			t.Fatal("different passwords should produce different hashes")
		}
	})

	t.Run("same password produces different hashes (salting)", func(t *testing.T) {
		password := "samepassword"

		hash1, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword returned error: %v", err)
		}

		hash2, err := HashPassword(password)
		if err != nil {
			t.Fatalf("HashPassword returned error: %v", err)
		}

		if hash1 == hash2 {
			t.Fatal("same password should produce different hashes due to random salt")
		}

		// Both should still verify against the original password.
		if err := VerifyPassword(hash1, password); err != nil {
			t.Fatalf("VerifyPassword failed for hash1: %v", err)
		}
		if err := VerifyPassword(hash2, password); err != nil {
			t.Fatalf("VerifyPassword failed for hash2: %v", err)
		}
	})

	t.Run("bcrypt cost is 12", func(t *testing.T) {
		hash, err := HashPassword("testpassword")
		if err != nil {
			t.Fatalf("HashPassword returned error: %v", err)
		}

		cost, err := bcrypt.Cost([]byte(hash))
		if err != nil {
			t.Fatalf("bcrypt.Cost returned error: %v", err)
		}

		if cost != 12 {
			t.Fatalf("expected bcrypt cost 12, got %d", cost)
		}
	})
}
