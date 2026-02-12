package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultSessionTTL = 8 * time.Hour
	tokenBytes        = 32 // 32 bytes = 64 hex chars
)

var (
	// ErrSessionNotFound is returned when a session token is not found or expired.
	ErrSessionNotFound = errors.New("session not found or expired")
)

// Session represents an active admin session.
type Session struct {
	ID          uuid.UUID
	Token       string
	AdminUserID uuid.UUID
	IPAddress   string
	UserAgent   string
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// SessionManager manages admin sessions in PostgreSQL.
type SessionManager struct {
	pool       *pgxpool.Pool
	sessionTTL time.Duration
}

// NewSessionManager creates a new session manager with the given connection pool.
// If sessionTTL is 0, it defaults to 8 hours.
func NewSessionManager(pool *pgxpool.Pool, sessionTTL time.Duration) *SessionManager {
	if sessionTTL == 0 {
		sessionTTL = defaultSessionTTL
	}

	return &SessionManager{
		pool:       pool,
		sessionTTL: sessionTTL,
	}
}

// CreateSession creates a new session for an admin user.
// Generates a cryptographically random 32-byte token (hex encoded = 64 chars).
// Stores in admin_sessions table with admin_user_id, ip_address, user_agent, expires_at.
// Returns the session token.
func (sm *SessionManager) CreateSession(ctx context.Context, adminUserID uuid.UUID, ipAddress, userAgent string) (string, error) {
	token, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("generating session token: %w", err)
	}

	expiresAt := time.Now().UTC().Add(sm.sessionTTL)

	_, err = sm.pool.Exec(ctx, `
		INSERT INTO admin_sessions (id, token, admin_user_id, ip_address, user_agent, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, uuid.New(), token, adminUserID, ipAddress, userAgent, time.Now().UTC(), expiresAt)
	if err != nil {
		return "", fmt.Errorf("inserting session: %w", err)
	}

	return token, nil
}

// GetSession retrieves a session by token.
// Returns the session data if the token is valid and not expired.
// Extends expiration (sliding window) on each valid access.
func (sm *SessionManager) GetSession(ctx context.Context, token string) (*Session, error) {
	session := &Session{}
	err := sm.pool.QueryRow(ctx, `
		SELECT id, token, admin_user_id, ip_address, user_agent, created_at, expires_at
		FROM admin_sessions
		WHERE token = $1 AND expires_at > NOW()
	`, token).Scan(
		&session.ID,
		&session.Token,
		&session.AdminUserID,
		&session.IPAddress,
		&session.UserAgent,
		&session.CreatedAt,
		&session.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("querying session: %w", err)
	}

	// Sliding window: extend expiration on valid access.
	newExpiry := time.Now().UTC().Add(sm.sessionTTL)
	_, err = sm.pool.Exec(ctx, `
		UPDATE admin_sessions SET expires_at = $1 WHERE id = $2
	`, newExpiry, session.ID)
	if err != nil {
		// Log but don't fail the request if we can't extend the session.
		return session, nil
	}

	session.ExpiresAt = newExpiry
	return session, nil
}

// DeleteSession removes a session by token (logout).
func (sm *SessionManager) DeleteSession(ctx context.Context, token string) error {
	result, err := sm.pool.Exec(ctx, `
		DELETE FROM admin_sessions WHERE token = $1
	`, token)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrSessionNotFound
	}

	return nil
}

// DeleteUserSessions removes all sessions for a user (force logout everywhere).
func (sm *SessionManager) DeleteUserSessions(ctx context.Context, adminUserID uuid.UUID) error {
	_, err := sm.pool.Exec(ctx, `
		DELETE FROM admin_sessions WHERE admin_user_id = $1
	`, adminUserID)
	if err != nil {
		return fmt.Errorf("deleting user sessions: %w", err)
	}

	return nil
}

// CleanupExpired removes all expired sessions. Should be called periodically.
func (sm *SessionManager) CleanupExpired(ctx context.Context) (int64, error) {
	result, err := sm.pool.Exec(ctx, `
		DELETE FROM admin_sessions WHERE expires_at <= NOW()
	`)
	if err != nil {
		return 0, fmt.Errorf("cleaning up expired sessions: %w", err)
	}

	return result.RowsAffected(), nil
}

// generateToken generates a cryptographically secure random hex token.
func generateToken() (string, error) {
	bytes := make([]byte, tokenBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}

	return hex.EncodeToString(bytes), nil
}
