package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	// ErrInvalidToken is returned when a JWT is malformed or expired.
	ErrInvalidToken = errors.New("invalid or expired token")
	// ErrInvalidPassword is returned when authentication fails.
	ErrInvalidPassword = errors.New("invalid email or password")
)

// CustomerClaims holds the JWT claims for a customer token.
type CustomerClaims struct {
	CustomerID uuid.UUID `json:"customer_id"`
	Email      string    `json:"email"`
	jwt.RegisteredClaims
}

// JWTManager handles creation and validation of customer JWTs.
type JWTManager struct {
	secret        []byte
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

// NewJWTManager creates a new JWT manager with the given secret.
// Access tokens expire in 15 minutes; refresh tokens in 7 days.
func NewJWTManager(secret string) *JWTManager {
	return &JWTManager{
		secret:        []byte(secret),
		accessExpiry:  15 * time.Minute,
		refreshExpiry: 7 * 24 * time.Hour,
	}
}

// GenerateAccessToken creates a short-lived access token for the customer.
func (m *JWTManager) GenerateAccessToken(customerID uuid.UUID, email string) (string, error) {
	now := time.Now().UTC()
	claims := CustomerClaims{
		CustomerID: customerID,
		Email:      email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   customerID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessExpiry)),
			Issuer:    "forgecommerce",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("signing access token: %w", err)
	}
	return signed, nil
}

// GenerateRefreshToken creates a long-lived refresh token for the customer.
func (m *JWTManager) GenerateRefreshToken(customerID uuid.UUID, email string) (string, error) {
	now := time.Now().UTC()
	claims := CustomerClaims{
		CustomerID: customerID,
		Email:      email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   customerID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshExpiry)),
			Issuer:    "forgecommerce",
			ID:        uuid.New().String(), // Unique token ID for revocation
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("signing refresh token: %w", err)
	}
	return signed, nil
}

// ValidateToken parses and validates a JWT token string.
// Returns the claims if valid, or ErrInvalidToken if not.
func (m *JWTManager) ValidateToken(tokenStr string) (*CustomerClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomerClaims{}, func(token *jwt.Token) (any, error) {
		// Validate signing method.
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*CustomerClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
