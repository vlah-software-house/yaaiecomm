package auth_test

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"

	"github.com/forgecommerce/api/internal/auth"
	"github.com/forgecommerce/api/internal/testutil"
)

var testDB *testutil.TestDB

func TestMain(m *testing.M) {
	var code int
	defer func() { os.Exit(code) }()

	db, err := testutil.SetupTestDB()
	if err != nil {
		log.Fatalf("setting up test database: %v", err)
	}
	defer db.Close()
	testDB = db

	code = m.Run()
}

func newSessionManager() *auth.SessionManager {
	return auth.NewSessionManager(testDB.Pool, 0) // default 8h TTL
}

func newService() *auth.Service {
	sm := newSessionManager()
	return auth.NewService(testDB.Pool, sm, nil, "ForgeCommerce")
}

// createTestUser creates an admin user via the service and returns it.
func createTestUser(t *testing.T, svc *auth.Service, email, password string) *auth.AdminUser {
	t.Helper()
	user, err := svc.CreateUser(context.Background(), email, "Test User", password, "admin", []string{"all"})
	if err != nil {
		t.Fatalf("creating test user: %v", err)
	}
	return user
}

// --------------------------------------------------------------------------
// JWT tests (no DB needed)
// --------------------------------------------------------------------------

func TestJWT_AccessToken_RoundTrip(t *testing.T) {
	mgr := auth.NewJWTManager("test-secret-minimum-length-32-chars")
	id := uuid.New()

	token, err := mgr.GenerateAccessToken(id, "alice@example.com")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := mgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.CustomerID != id {
		t.Errorf("customer_id: got %s, want %s", claims.CustomerID, id)
	}
	if claims.Email != "alice@example.com" {
		t.Errorf("email: got %q, want %q", claims.Email, "alice@example.com")
	}
}

func TestJWT_RefreshToken_RoundTrip(t *testing.T) {
	mgr := auth.NewJWTManager("test-secret-minimum-length-32-chars")
	id := uuid.New()

	token, err := mgr.GenerateRefreshToken(id, "bob@example.com")
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	claims, err := mgr.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.CustomerID != id {
		t.Errorf("customer_id: got %s, want %s", claims.CustomerID, id)
	}
}

func TestJWT_WrongSecret(t *testing.T) {
	mgr1 := auth.NewJWTManager("secret-one-minimum-length-32-chars")
	mgr2 := auth.NewJWTManager("secret-two-minimum-length-32-chars")

	token, _ := mgr1.GenerateAccessToken(uuid.New(), "x@x.com")
	_, err := mgr2.ValidateToken(token)
	if err != auth.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestJWT_GarbageToken(t *testing.T) {
	mgr := auth.NewJWTManager("test-secret-minimum-length-32-chars")
	_, err := mgr.ValidateToken("not.a.valid.jwt")
	if err != auth.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestJWT_EmptyToken(t *testing.T) {
	mgr := auth.NewJWTManager("test-secret-minimum-length-32-chars")
	_, err := mgr.ValidateToken("")
	if err != auth.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

// --------------------------------------------------------------------------
// Session Manager (needs DB)
// --------------------------------------------------------------------------

func TestSessionManager_CreateAndGet(t *testing.T) {
	testDB.Truncate(t)
	sm := newSessionManager()
	ctx := context.Background()

	// Need an admin user for the FK.
	svc := newService()
	user := createTestUser(t, svc, "sess@example.com", "password123!")

	token, err := sm.CreateSession(ctx, user.ID, "127.0.0.1", "TestAgent/1.0")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if len(token) != 64 {
		t.Errorf("token length: got %d, want 64", len(token))
	}

	sess, err := sm.GetSession(ctx, token)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess.AdminUserID != user.ID {
		t.Errorf("admin_user_id: got %s, want %s", sess.AdminUserID, user.ID)
	}
	if sess.IPAddress != "127.0.0.1" {
		t.Errorf("ip: got %q, want %q", sess.IPAddress, "127.0.0.1")
	}
}

func TestSessionManager_GetNotFound(t *testing.T) {
	testDB.Truncate(t)
	sm := newSessionManager()
	ctx := context.Background()

	_, err := sm.GetSession(ctx, "nonexistent-token-aaaa1111222233334444")
	if err != auth.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestSessionManager_Delete(t *testing.T) {
	testDB.Truncate(t)
	sm := newSessionManager()
	ctx := context.Background()

	svc := newService()
	user := createTestUser(t, svc, "del@example.com", "password123!")

	token, _ := sm.CreateSession(ctx, user.ID, "127.0.0.1", "TestAgent")

	err := sm.DeleteSession(ctx, token)
	if err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	_, err = sm.GetSession(ctx, token)
	if err != auth.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound after delete, got %v", err)
	}
}

func TestSessionManager_DeleteNotFound(t *testing.T) {
	testDB.Truncate(t)
	sm := newSessionManager()
	ctx := context.Background()

	err := sm.DeleteSession(ctx, "nonexistent-token-aaaa1111222233334444")
	if err != auth.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestSessionManager_DeleteUserSessions(t *testing.T) {
	testDB.Truncate(t)
	sm := newSessionManager()
	ctx := context.Background()

	svc := newService()
	user := createTestUser(t, svc, "multi@example.com", "password123!")

	sm.CreateSession(ctx, user.ID, "127.0.0.1", "Browser1")
	sm.CreateSession(ctx, user.ID, "127.0.0.1", "Browser2")

	err := sm.DeleteUserSessions(ctx, user.ID)
	if err != nil {
		t.Fatalf("DeleteUserSessions: %v", err)
	}
}

func TestSessionManager_CleanupExpired(t *testing.T) {
	testDB.Truncate(t)
	ctx := context.Background()

	// Use a very short TTL so sessions expire immediately.
	sm := auth.NewSessionManager(testDB.Pool, 1*time.Millisecond)

	svc := auth.NewService(testDB.Pool, sm, nil, "ForgeCommerce")
	user := createTestUser(t, svc, "expire@example.com", "password123!")

	sm.CreateSession(ctx, user.ID, "127.0.0.1", "ExpireMe")
	time.Sleep(10 * time.Millisecond) // ensure it's expired

	cleaned, err := sm.CleanupExpired(ctx)
	if err != nil {
		t.Fatalf("CleanupExpired: %v", err)
	}
	if cleaned < 1 {
		t.Errorf("expected at least 1 cleaned session, got %d", cleaned)
	}
}

// --------------------------------------------------------------------------
// Auth Service — User CRUD
// --------------------------------------------------------------------------

func TestCreateUser(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user, err := svc.CreateUser(ctx, "admin@forge.com", "Admin", "securePass123!", "admin", []string{"products", "orders"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if user.Email != "admin@forge.com" {
		t.Errorf("email: got %q, want %q", user.Email, "admin@forge.com")
	}
	if user.Name != "Admin" {
		t.Errorf("name: got %q, want %q", user.Name, "Admin")
	}
	if user.Role != "admin" {
		t.Errorf("role: got %q, want %q", user.Role, "admin")
	}
	if !user.IsActive {
		t.Error("expected is_active=true")
	}
	if !user.Force2FASetup {
		t.Error("expected force_2fa_setup=true for new user")
	}
	if user.TOTPVerified {
		t.Error("expected totp_verified=false for new user")
	}
}

func TestGetUserByID(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created := createTestUser(t, svc, "getid@forge.com", "pass123!")

	got, err := svc.GetUserByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.Email != "getid@forge.com" {
		t.Errorf("email: got %q, want %q", got.Email, "getid@forge.com")
	}
}

func TestGetUserByID_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetUserByID(ctx, uuid.New())
	if err != auth.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestGetUserByEmail(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	createTestUser(t, svc, "getemail@forge.com", "pass123!")

	got, err := svc.GetUserByEmail(ctx, "getemail@forge.com")
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if got.Email != "getemail@forge.com" {
		t.Errorf("email: got %q, want %q", got.Email, "getemail@forge.com")
	}
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.GetUserByEmail(ctx, "nosuch@forge.com")
	if err != auth.ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestListUsers(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	createTestUser(t, svc, "u1@forge.com", "pass123!")
	createTestUser(t, svc, "u2@forge.com", "pass123!")

	users, err := svc.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("count: got %d, want 2", len(users))
	}
}

func TestUpdateUser(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	created := createTestUser(t, svc, "upd@forge.com", "pass123!")

	updated, err := svc.UpdateUser(ctx, created.ID, "New Name", "editor", []string{"products"}, true)
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("name: got %q, want %q", updated.Name, "New Name")
	}
	if updated.Role != "editor" {
		t.Errorf("role: got %q, want %q", updated.Role, "editor")
	}
}

func TestSetUserActive(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "active@forge.com", "pass123!")

	err := svc.SetUserActive(ctx, user.ID, false)
	if err != nil {
		t.Fatalf("SetUserActive: %v", err)
	}

	got, _ := svc.GetUserByID(ctx, user.ID)
	if got.IsActive {
		t.Error("expected is_active=false")
	}
}

func TestUpdateLastLogin(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "login@forge.com", "pass123!")
	if user.LastLoginAt != nil {
		t.Error("expected nil LastLoginAt for new user")
	}

	err := svc.UpdateLastLogin(ctx, user.ID)
	if err != nil {
		t.Fatalf("UpdateLastLogin: %v", err)
	}

	got, _ := svc.GetUserByID(ctx, user.ID)
	if got.LastLoginAt == nil {
		t.Error("expected non-nil LastLoginAt after update")
	}
}

func TestAuditLog(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "audit@forge.com", "pass123!")

	// AuditLog doesn't return an error; it logs internally.
	svc.AuditLog(ctx, user.ID, "test_action", "admin_user", user.ID, map[string]any{"key": "value"})

	// Verify audit log was created.
	var count int
	err := testDB.Pool.QueryRow(ctx, `SELECT count(*) FROM admin_audit_log WHERE admin_user_id = $1`, user.ID).Scan(&count)
	if err != nil {
		t.Fatalf("querying audit log: %v", err)
	}
	if count != 1 {
		t.Errorf("audit log count: got %d, want 1", count)
	}
}

// --------------------------------------------------------------------------
// Auth Service — Login
// --------------------------------------------------------------------------

func TestLogin_HappyPath(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	createTestUser(t, svc, "login@forge.com", "correctPassword123!")

	user, err := svc.Login(ctx, "login@forge.com", "correctPassword123!")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if user.Email != "login@forge.com" {
		t.Errorf("email: got %q, want %q", user.Email, "login@forge.com")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	createTestUser(t, svc, "login@forge.com", "correctPassword123!")

	_, err := svc.Login(ctx, "login@forge.com", "wrongPassword")
	if err != auth.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_NonexistentEmail(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, err := svc.Login(ctx, "nosuch@forge.com", "anyPassword")
	if err != auth.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_InactiveUser(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "inactive@forge.com", "pass123!")
	svc.SetUserActive(ctx, user.ID, false)

	_, err := svc.Login(ctx, "inactive@forge.com", "pass123!")
	if err != auth.ErrUserInactive {
		t.Errorf("expected ErrUserInactive, got %v", err)
	}
}

// --------------------------------------------------------------------------
// Auth Service — Session-based flows
// --------------------------------------------------------------------------

func TestCreateSessionDirect(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "direct@forge.com", "pass123!")

	token, err := svc.CreateSessionDirect(ctx, user.ID, "10.0.0.1", "Firefox/1.0")
	if err != nil {
		t.Fatalf("CreateSessionDirect: %v", err)
	}
	if len(token) != 64 {
		t.Errorf("token length: got %d, want 64", len(token))
	}
}

func TestValidateSession(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "validate@forge.com", "pass123!")
	token, _ := svc.CreateSessionDirect(ctx, user.ID, "10.0.0.1", "Chrome")

	gotUser, sess, err := svc.ValidateSession(ctx, token)
	if err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
	if gotUser.ID != user.ID {
		t.Errorf("user_id mismatch: got %s, want %s", gotUser.ID, user.ID)
	}
	if sess == nil {
		t.Fatal("expected non-nil session")
	}
}

func TestValidateSession_InvalidToken(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	_, _, err := svc.ValidateSession(ctx, "nonexistent-token")
	if err != auth.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestValidateSession_InactiveUser(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "deact@forge.com", "pass123!")
	token, _ := svc.CreateSessionDirect(ctx, user.ID, "10.0.0.1", "Chrome")

	// Deactivate the user after session was created.
	svc.SetUserActive(ctx, user.ID, false)

	_, _, err := svc.ValidateSession(ctx, token)
	if err != auth.ErrUserInactive {
		t.Errorf("expected ErrUserInactive, got %v", err)
	}
}

func TestLogout(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "logout@forge.com", "pass123!")
	token, _ := svc.CreateSessionDirect(ctx, user.ID, "10.0.0.1", "Chrome")

	err := svc.Logout(ctx, token)
	if err != nil {
		t.Fatalf("Logout: %v", err)
	}

	_, _, err = svc.ValidateSession(ctx, token)
	if err != auth.ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound after logout, got %v", err)
	}
}

// --------------------------------------------------------------------------
// Auth Service — 2FA Setup & Verification
// --------------------------------------------------------------------------

func TestSetup2FA(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "2fa@forge.com", "pass123!")

	setup, err := svc.Setup2FA(ctx, user.ID)
	if err != nil {
		t.Fatalf("Setup2FA: %v", err)
	}
	if setup.Secret == "" {
		t.Error("expected non-empty TOTP secret")
	}
	if setup.URL == "" {
		t.Error("expected non-empty OTP URL")
	}
	if len(setup.QRCode) == 0 {
		t.Error("expected non-empty QR code")
	}
}

func TestSetup2FA_AlreadySetup(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "2fadup@forge.com", "pass123!")

	// Set up and confirm 2FA.
	setup, _ := svc.Setup2FA(ctx, user.ID)
	code, _ := totp.GenerateCode(setup.Secret, time.Now())
	svc.Confirm2FA(ctx, user.ID, code)

	// Try setting up again.
	_, err := svc.Setup2FA(ctx, user.ID)
	if err != auth.ErrTOTPAlreadySetup {
		t.Errorf("expected ErrTOTPAlreadySetup, got %v", err)
	}
}

func TestConfirm2FA(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "confirm2fa@forge.com", "pass123!")

	setup, _ := svc.Setup2FA(ctx, user.ID)
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatalf("generating TOTP code: %v", err)
	}

	recoveryCodes, err := svc.Confirm2FA(ctx, user.ID, code)
	if err != nil {
		t.Fatalf("Confirm2FA: %v", err)
	}
	if len(recoveryCodes) != 8 {
		t.Errorf("recovery codes: got %d, want 8", len(recoveryCodes))
	}

	// User should now have totp_verified=true.
	got, _ := svc.GetUserByID(ctx, user.ID)
	if !got.TOTPVerified {
		t.Error("expected totp_verified=true after confirmation")
	}
	if got.Force2FASetup {
		t.Error("expected force_2fa_setup=false after confirmation")
	}
}

func TestConfirm2FA_InvalidCode(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "badcode@forge.com", "pass123!")
	svc.Setup2FA(ctx, user.ID)

	_, err := svc.Confirm2FA(ctx, user.ID, "000000")
	if err != auth.ErrInvalidTOTPCode {
		t.Errorf("expected ErrInvalidTOTPCode, got %v", err)
	}
}

func TestConfirm2FA_NoSetup(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "nosetup@forge.com", "pass123!")

	_, err := svc.Confirm2FA(ctx, user.ID, "123456")
	if err != auth.ErrTOTPNotSetup {
		t.Errorf("expected ErrTOTPNotSetup, got %v", err)
	}
}

func TestCompleteTwoFactor(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "complete2fa@forge.com", "pass123!")

	// Set up and confirm 2FA.
	setup, _ := svc.Setup2FA(ctx, user.ID)
	code, _ := totp.GenerateCode(setup.Secret, time.Now())
	svc.Confirm2FA(ctx, user.ID, code)

	// Now complete a 2FA login.
	code2, _ := totp.GenerateCode(setup.Secret, time.Now())
	token, err := svc.CompleteTwoFactor(ctx, user.ID, code2, "10.0.0.1", "Chrome")
	if err != nil {
		t.Fatalf("CompleteTwoFactor: %v", err)
	}
	if len(token) != 64 {
		t.Errorf("token length: got %d, want 64", len(token))
	}
}

func TestCompleteTwoFactor_InvalidCode(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "bad2fa@forge.com", "pass123!")

	// Set up and confirm 2FA.
	setup, _ := svc.Setup2FA(ctx, user.ID)
	code, _ := totp.GenerateCode(setup.Secret, time.Now())
	svc.Confirm2FA(ctx, user.ID, code)

	_, err := svc.CompleteTwoFactor(ctx, user.ID, "000000", "10.0.0.1", "Chrome")
	if err != auth.ErrInvalidTOTPCode {
		t.Errorf("expected ErrInvalidTOTPCode, got %v", err)
	}
}

func TestCompleteTwoFactor_NotSetup(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "no2fa@forge.com", "pass123!")

	_, err := svc.CompleteTwoFactor(ctx, user.ID, "123456", "10.0.0.1", "Chrome")
	if err != auth.ErrTOTPNotSetup {
		t.Errorf("expected ErrTOTPNotSetup, got %v", err)
	}
}

// --------------------------------------------------------------------------
// Auth Service — Recovery codes
// --------------------------------------------------------------------------

func TestUseRecoveryCode(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "recovery@forge.com", "pass123!")

	// Set up and confirm 2FA (which generates recovery codes).
	setup, _ := svc.Setup2FA(ctx, user.ID)
	code, _ := totp.GenerateCode(setup.Secret, time.Now())
	recoveryCodes, _ := svc.Confirm2FA(ctx, user.ID, code)

	// Use the first recovery code.
	token, err := svc.UseRecoveryCode(ctx, user.ID, recoveryCodes[0], "10.0.0.1", "Chrome")
	if err != nil {
		t.Fatalf("UseRecoveryCode: %v", err)
	}
	if len(token) != 64 {
		t.Errorf("token length: got %d, want 64", len(token))
	}

	// Using the same recovery code again should fail (it was burned).
	_, err = svc.UseRecoveryCode(ctx, user.ID, recoveryCodes[0], "10.0.0.1", "Chrome")
	if err != auth.ErrInvalidRecoveryCode {
		t.Errorf("expected ErrInvalidRecoveryCode for burned code, got %v", err)
	}
}

func TestUseRecoveryCode_InvalidCode(t *testing.T) {
	testDB.Truncate(t)
	svc := newService()
	ctx := context.Background()

	user := createTestUser(t, svc, "badrec@forge.com", "pass123!")

	// Set up and confirm 2FA.
	setup, _ := svc.Setup2FA(ctx, user.ID)
	code, _ := totp.GenerateCode(setup.Secret, time.Now())
	svc.Confirm2FA(ctx, user.ID, code)

	_, err := svc.UseRecoveryCode(ctx, user.ID, "ZZZZ-ZZZZ", "10.0.0.1", "Chrome")
	if err != auth.ErrInvalidRecoveryCode {
		t.Errorf("expected ErrInvalidRecoveryCode, got %v", err)
	}
}
