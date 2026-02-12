package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

var (
	// ErrEmptyPassword is returned when an empty password is provided.
	ErrEmptyPassword = errors.New("password cannot be empty")
)

// HashPassword hashes a password with bcrypt cost 12.
// Returns an error if the password is empty.
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", ErrEmptyPassword
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}

	return string(hash), nil
}

// VerifyPassword compares a plaintext password with its bcrypt hash.
// Returns nil if the password matches, or an error otherwise.
func VerifyPassword(hashedPassword, password string) error {
	if password == "" {
		return ErrEmptyPassword
	}

	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return fmt.Errorf("invalid password: %w", err)
		}
		return fmt.Errorf("verifying password: %w", err)
	}

	return nil
}
