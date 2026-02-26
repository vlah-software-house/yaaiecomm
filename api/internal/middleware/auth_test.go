package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/forgecommerce/api/internal/auth"
	"github.com/google/uuid"
)

// mockSessionValidator implements SessionValidator for testing RequireAuth.
type mockSessionValidator struct {
	user *auth.AdminUser
	sess *auth.Session
	err  error
}

func (m *mockSessionValidator) ValidateSession(_ context.Context, _ string) (*auth.AdminUser, *auth.Session, error) {
	return m.user, m.sess, m.err
}

// --- RequireAuth middleware tests ---

func TestRequireAuth_NoCookie_RedirectsToLogin(t *testing.T) {
	validator := &mockSessionValidator{}
	handler := RequireAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when no cookie is present")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusSeeOther)
	}
	loc := rr.Header().Get("Location")
	if loc != "/admin/login" {
		t.Errorf("redirect location: got %q, want %q", loc, "/admin/login")
	}
}

func TestRequireAuth_EmptyCookieValue_RedirectsToLogin(t *testing.T) {
	validator := &mockSessionValidator{}
	handler := RequireAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with empty cookie value")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ""})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusSeeOther)
	}
	loc := rr.Header().Get("Location")
	if loc != "/admin/login" {
		t.Errorf("redirect location: got %q, want %q", loc, "/admin/login")
	}
}

func TestRequireAuth_InvalidSession_ClearsCookieAndRedirects(t *testing.T) {
	validator := &mockSessionValidator{
		err: errors.New("session not found or expired"),
	}
	handler := RequireAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with invalid session")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "expired-token"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusSeeOther)
	}
	loc := rr.Header().Get("Location")
	if loc != "/admin/login" {
		t.Errorf("redirect location: got %q, want %q", loc, "/admin/login")
	}

	// Should have set a clearing cookie.
	var found *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == sessionCookieName {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("expected clearing cookie to be set")
	}
	if found.Value != "" {
		t.Errorf("clearing cookie value: got %q, want empty", found.Value)
	}
	if found.MaxAge != -1 {
		t.Errorf("clearing cookie MaxAge: got %d, want -1", found.MaxAge)
	}
	if !found.HttpOnly {
		t.Error("clearing cookie should be HttpOnly")
	}
	if !found.Secure {
		t.Error("clearing cookie should be Secure")
	}
	if found.SameSite != http.SameSiteStrictMode {
		t.Errorf("clearing cookie SameSite: got %v, want Strict", found.SameSite)
	}
}

func TestRequireAuth_Force2FASetup_RedirectsTo2FA(t *testing.T) {
	userID := uuid.New()
	validator := &mockSessionValidator{
		user: &auth.AdminUser{
			ID:            userID,
			Force2FASetup: true,
			IsActive:      true,
		},
		sess: &auth.Session{},
	}
	handler := RequireAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when Force2FASetup is true")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "valid-token"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusSeeOther)
	}
	loc := rr.Header().Get("Location")
	if loc != "/admin/setup-2fa" {
		t.Errorf("redirect location: got %q, want %q", loc, "/admin/setup-2fa")
	}
}

func TestRequireAuth_ValidSession_SetsContextAndCallsNext(t *testing.T) {
	userID := uuid.New()
	sessionToken := "valid-session-token-abc123"
	validator := &mockSessionValidator{
		user: &auth.AdminUser{
			ID:            userID,
			Force2FASetup: false,
			IsActive:      true,
		},
		sess: &auth.Session{},
	}

	var capturedUserID string
	var capturedToken string
	handler := RequireAuth(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID, _ = r.Context().Value(AdminUserIDKey).(string)
		capturedToken, _ = r.Context().Value(SessionTokenKey).(string)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionToken})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusOK)
	}
	if capturedUserID != userID.String() {
		t.Errorf("admin user ID in context: got %q, want %q", capturedUserID, userID.String())
	}
	if capturedToken != sessionToken {
		t.Errorf("session token in context: got %q, want %q", capturedToken, sessionToken)
	}
}

func TestSetSessionCookie(t *testing.T) {
	rr := httptest.NewRecorder()
	SetSessionCookie(rr, "mytoken123")

	cookies := rr.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("expected session cookie to be set")
	}
	if found.Value != "mytoken123" {
		t.Errorf("value: got %q, want %q", found.Value, "mytoken123")
	}
	if !found.HttpOnly {
		t.Error("expected HttpOnly")
	}
	if !found.Secure {
		t.Error("expected Secure")
	}
	if found.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite: got %v, want Strict", found.SameSite)
	}
	if found.MaxAge != 8*60*60 {
		t.Errorf("MaxAge: got %d, want %d", found.MaxAge, 8*60*60)
	}
	if found.Path != "/" {
		t.Errorf("Path: got %q, want %q", found.Path, "/")
	}
}

func TestClearSessionCookie(t *testing.T) {
	rr := httptest.NewRecorder()
	ClearSessionCookie(rr)

	cookies := rr.Result().Cookies()
	var found *http.Cookie
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			found = c
			break
		}
	}
	if found == nil {
		t.Fatal("expected session cookie to be set")
	}
	if found.Value != "" {
		t.Errorf("value: got %q, want empty", found.Value)
	}
	if found.MaxAge != -1 {
		t.Errorf("MaxAge: got %d, want -1 (delete)", found.MaxAge)
	}
	if !found.HttpOnly {
		t.Error("expected HttpOnly")
	}
	if !found.Secure {
		t.Error("expected Secure")
	}
}

func TestAdminUserIDFromContext_Valid(t *testing.T) {
	expected := uuid.New()
	ctx := context.WithValue(context.Background(), AdminUserIDKey, expected.String())

	id, ok := AdminUserIDFromContext(ctx)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if id != expected {
		t.Errorf("id: got %s, want %s", id, expected)
	}
}

func TestAdminUserIDFromContext_Missing(t *testing.T) {
	_, ok := AdminUserIDFromContext(context.Background())
	if ok {
		t.Error("expected ok=false for missing context value")
	}
}

func TestAdminUserIDFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), AdminUserIDKey, 12345)

	_, ok := AdminUserIDFromContext(ctx)
	if ok {
		t.Error("expected ok=false for wrong type")
	}
}

func TestAdminUserIDFromContext_InvalidUUID(t *testing.T) {
	ctx := context.WithValue(context.Background(), AdminUserIDKey, "not-a-uuid")

	_, ok := AdminUserIDFromContext(ctx)
	if ok {
		t.Error("expected ok=false for invalid UUID string")
	}
}
