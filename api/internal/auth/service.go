package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrInvalidCredentials is returned when email/password authentication fails.
	ErrInvalidCredentials = errors.New("invalid email or password")

	// ErrUserNotFound is returned when an admin user is not found.
	ErrUserNotFound = errors.New("admin user not found")

	// ErrUserInactive is returned when an admin user account is disabled.
	ErrUserInactive = errors.New("admin user account is inactive")

	// ErrInvalidTOTPCode is returned when a TOTP code is invalid.
	ErrInvalidTOTPCode = errors.New("invalid TOTP code")

	// ErrTOTPAlreadySetup is returned when 2FA is already configured for a user.
	ErrTOTPAlreadySetup = errors.New("two-factor authentication is already set up")

	// ErrTOTPNotSetup is returned when 2FA has not been set up for a user.
	ErrTOTPNotSetup = errors.New("two-factor authentication is not set up")

	// ErrInvalidRecoveryCode is returned when a recovery code is invalid.
	ErrInvalidRecoveryCode = errors.New("invalid recovery code")
)

// AdminUser represents an admin user from the database.
type AdminUser struct {
	ID            uuid.UUID
	Email         string
	Name          string
	PasswordHash  string
	Role          string
	Permissions   []string
	TOTPSecret    string
	TOTPVerified  bool
	RecoveryCodes []string // hashed codes stored in the database
	Force2FASetup bool
	IsActive      bool
	LastLoginAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Service handles admin authentication flows.
type Service struct {
	pool    *pgxpool.Pool
	session *SessionManager
	logger  *slog.Logger
	issuer  string // TOTP issuer name (e.g., "ForgeCommerce")
}

// NewService creates a new auth service.
func NewService(pool *pgxpool.Pool, session *SessionManager, logger *slog.Logger, issuer string) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		pool:    pool,
		session: session,
		logger:  logger,
		issuer:  issuer,
	}
}

// Login authenticates an admin user with email and password.
// Returns the user if credentials are valid, but does NOT create a session
// (caller must check 2FA first).
func (s *Service) Login(ctx context.Context, email, password string) (*AdminUser, error) {
	user, err := s.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// Don't leak whether the email exists. Still do a dummy hash comparison
			// to prevent timing attacks.
			_ = VerifyPassword("$2a$12$000000000000000000000000000000000000000000000000000000", password)
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("looking up user: %w", err)
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	if err := VerifyPassword(user.PasswordHash, password); err != nil {
		s.logger.Warn("failed login attempt",
			slog.String("email", email),
			slog.String("error", "invalid password"),
		)
		return nil, ErrInvalidCredentials
	}

	return user, nil
}

// CompleteTwoFactor validates the TOTP code and creates a session.
// Returns the session token on success.
func (s *Service) CompleteTwoFactor(ctx context.Context, userID uuid.UUID, code, ipAddress, userAgent string) (string, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("fetching user for 2FA: %w", err)
	}

	if !user.TOTPVerified || user.TOTPSecret == "" {
		return "", ErrTOTPNotSetup
	}

	if !ValidateTOTPCode(code, user.TOTPSecret) {
		s.logger.Warn("failed 2FA attempt",
			slog.String("user_id", userID.String()),
		)
		return "", ErrInvalidTOTPCode
	}

	token, err := s.session.CreateSession(ctx, userID, ipAddress, userAgent)
	if err != nil {
		return "", fmt.Errorf("creating session after 2FA: %w", err)
	}

	if err := s.UpdateLastLogin(ctx, userID); err != nil {
		s.logger.Error("failed to update last login",
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()),
		)
	}

	s.logger.Info("admin login successful",
		slog.String("user_id", userID.String()),
		slog.String("email", user.Email),
		slog.String("ip", ipAddress),
	)

	return token, nil
}

// Setup2FA generates a new TOTP secret for a user who hasn't set up 2FA yet.
// Returns the TOTP setup containing the secret, OTP URL, and QR code PNG bytes.
func (s *Service) Setup2FA(ctx context.Context, userID uuid.UUID) (*TOTPSetup, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching user for 2FA setup: %w", err)
	}

	if user.TOTPVerified {
		return nil, ErrTOTPAlreadySetup
	}

	setup, err := GenerateTOTPSecret(s.issuer, user.Email)
	if err != nil {
		return nil, fmt.Errorf("generating TOTP secret: %w", err)
	}

	// Store the secret temporarily (not yet verified).
	_, err = s.pool.Exec(ctx, `
		UPDATE admin_users
		SET totp_secret = $1, updated_at = $2
		WHERE id = $3
	`, setup.Secret, time.Now().UTC(), userID)
	if err != nil {
		return nil, fmt.Errorf("storing TOTP secret: %w", err)
	}

	return setup, nil
}

// Confirm2FA verifies the initial TOTP setup code and enables 2FA for the user.
// Stores the TOTP secret and recovery codes. Sets totp_verified=true, force_2fa_setup=false.
// Returns the plaintext recovery codes (to show the user once).
func (s *Service) Confirm2FA(ctx context.Context, userID uuid.UUID, code string) ([]string, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching user for 2FA confirmation: %w", err)
	}

	if user.TOTPVerified {
		return nil, ErrTOTPAlreadySetup
	}

	if user.TOTPSecret == "" {
		return nil, ErrTOTPNotSetup
	}

	if !ValidateTOTPCode(code, user.TOTPSecret) {
		return nil, ErrInvalidTOTPCode
	}

	// Generate recovery codes.
	recovery, err := GenerateRecoveryCodes()
	if err != nil {
		return nil, fmt.Errorf("generating recovery codes: %w", err)
	}

	// Enable 2FA: set verified, store hashed recovery codes, clear force setup flag.
	_, err = s.pool.Exec(ctx, `
		UPDATE admin_users
		SET totp_verified = true,
		    force_2fa_setup = false,
		    recovery_codes = $1,
		    updated_at = $2
		WHERE id = $3
	`, recovery.Hashed, time.Now().UTC(), userID)
	if err != nil {
		return nil, fmt.Errorf("enabling 2FA: %w", err)
	}

	s.AuditLog(ctx, userID, "2fa_enabled", "admin_user", userID, nil)

	return recovery.Plaintext, nil
}

// UseRecoveryCode validates a recovery code and burns it (removes from stored codes).
// Creates a session on success. Returns the session token.
func (s *Service) UseRecoveryCode(ctx context.Context, userID uuid.UUID, code, ipAddress, userAgent string) (string, error) {
	user, err := s.GetUserByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("fetching user for recovery code: %w", err)
	}

	idx := ValidateRecoveryCode(code, user.RecoveryCodes)
	if idx == -1 {
		s.logger.Warn("invalid recovery code attempt",
			slog.String("user_id", userID.String()),
		)
		return "", ErrInvalidRecoveryCode
	}

	// Burn the used code by removing it from the slice.
	remaining := make([]string, 0, len(user.RecoveryCodes)-1)
	for i, c := range user.RecoveryCodes {
		if i != idx {
			remaining = append(remaining, c)
		}
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE admin_users
		SET recovery_codes = $1, updated_at = $2
		WHERE id = $3
	`, remaining, time.Now().UTC(), userID)
	if err != nil {
		return "", fmt.Errorf("burning recovery code: %w", err)
	}

	token, err := s.session.CreateSession(ctx, userID, ipAddress, userAgent)
	if err != nil {
		return "", fmt.Errorf("creating session after recovery: %w", err)
	}

	if err := s.UpdateLastLogin(ctx, userID); err != nil {
		s.logger.Error("failed to update last login",
			slog.String("user_id", userID.String()),
			slog.String("error", err.Error()),
		)
	}

	s.AuditLog(ctx, userID, "recovery_code_used", "admin_user", userID, map[string]any{
		"codes_remaining": len(remaining),
	})

	s.logger.Info("admin login via recovery code",
		slog.String("user_id", userID.String()),
		slog.Int("codes_remaining", len(remaining)),
	)

	return token, nil
}

// GetUserByID fetches an admin user by ID.
func (s *Service) GetUserByID(ctx context.Context, id uuid.UUID) (*AdminUser, error) {
	return s.scanUser(s.pool.QueryRow(ctx, `
		SELECT id, email, name, password_hash, role, permissions,
		       totp_secret, totp_verified, recovery_codes, force_2fa_setup,
		       is_active, last_login_at, created_at, updated_at
		FROM admin_users
		WHERE id = $1
	`, id))
}

// GetUserByEmail fetches an admin user by email.
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*AdminUser, error) {
	return s.scanUser(s.pool.QueryRow(ctx, `
		SELECT id, email, name, password_hash, role, permissions,
		       totp_secret, totp_verified, recovery_codes, force_2fa_setup,
		       is_active, last_login_at, created_at, updated_at
		FROM admin_users
		WHERE email = $1
	`, email))
}

// CreateUser creates a new admin user with the given email, name, and password.
// The user is created with force_2fa_setup=true, requiring 2FA setup on first login.
func (s *Service) CreateUser(ctx context.Context, email, name, password, role string, permissions []string) (*AdminUser, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hashing password for new user: %w", err)
	}

	id := uuid.New()
	now := time.Now().UTC()

	_, err = s.pool.Exec(ctx, `
		INSERT INTO admin_users (
			id, email, name, password_hash, role, permissions,
			totp_secret, totp_verified, recovery_codes, force_2fa_setup,
			is_active, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, '', false, '{}', true, true, $7, $7)
	`, id, email, name, hash, role, permissions, now)
	if err != nil {
		return nil, fmt.Errorf("inserting admin user: %w", err)
	}

	return s.GetUserByID(ctx, id)
}

// UpdateLastLogin updates the last_login_at timestamp for a user.
func (s *Service) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE admin_users SET last_login_at = $1 WHERE id = $2
	`, time.Now().UTC(), userID)
	if err != nil {
		return fmt.Errorf("updating last login: %w", err)
	}

	return nil
}

// AuditLog creates an audit log entry for an admin action.
func (s *Service) AuditLog(ctx context.Context, adminUserID uuid.UUID, action, entityType string, entityID uuid.UUID, changes map[string]any) {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO admin_audit_log (id, admin_user_id, action, entity_type, entity_id, changes, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, uuid.New(), adminUserID, action, entityType, entityID, changes, time.Now().UTC())
	if err != nil {
		s.logger.Error("failed to write audit log",
			slog.String("action", action),
			slog.String("entity_type", entityType),
			slog.String("entity_id", entityID.String()),
			slog.String("error", err.Error()),
		)
	}
}

// Logout deletes the session for the given token.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.session.DeleteSession(ctx, token)
}

// ValidateSession retrieves and validates a session by token.
// Returns the associated admin user if the session is valid.
func (s *Service) ValidateSession(ctx context.Context, token string) (*AdminUser, *Session, error) {
	sess, err := s.session.GetSession(ctx, token)
	if err != nil {
		return nil, nil, err
	}

	user, err := s.GetUserByID(ctx, sess.AdminUserID)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching user for session: %w", err)
	}

	if !user.IsActive {
		// User was deactivated while session was still valid. Kill the session.
		_ = s.session.DeleteSession(ctx, token)
		return nil, nil, ErrUserInactive
	}

	return user, sess, nil
}

// scanUser scans a single admin user row from a query result.
func (s *Service) scanUser(row pgx.Row) (*AdminUser, error) {
	user := &AdminUser{}
	var totpSecret *string
	var lastLogin *time.Time

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.PasswordHash,
		&user.Role,
		&user.Permissions,
		&totpSecret,
		&user.TOTPVerified,
		&user.RecoveryCodes,
		&user.Force2FASetup,
		&user.IsActive,
		&lastLogin,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("scanning admin user: %w", err)
	}

	if totpSecret != nil {
		user.TOTPSecret = *totpSecret
	}
	user.LastLoginAt = lastLogin

	return user, nil
}
