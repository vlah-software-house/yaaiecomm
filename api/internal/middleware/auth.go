package middleware

import (
	"context"
	"net/http"

	"github.com/forgecommerce/api/internal/auth"
	"github.com/google/uuid"
)

const sessionCookieName = "admin_session"

// RequireAuth checks for a valid admin session cookie.
// If invalid, redirects to /admin/login.
// On success, stores the admin user ID and session token in context.
func RequireAuth(authService *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || cookie.Value == "" {
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}

			user, _, err := authService.ValidateSession(r.Context(), cookie.Value)
			if err != nil {
				// Clear invalid cookie
				http.SetCookie(w, &http.Cookie{
					Name:     sessionCookieName,
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
					Secure:   true,
					SameSite: http.SameSiteStrictMode,
				})
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}

			// Check if user needs to set up 2FA
			if user.Force2FASetup {
				http.Redirect(w, r, "/admin/setup-2fa", http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), AdminUserIDKey, user.ID.String())
			ctx = context.WithValue(ctx, SessionTokenKey, cookie.Value)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SetSessionCookie sets the admin session cookie on the response.
func SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   8 * 60 * 60, // 8 hours
	})
}

// ClearSessionCookie clears the admin session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

// AdminUserIDFromContext returns the admin user ID from the request context.
func AdminUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	idStr, ok := ctx.Value(AdminUserIDKey).(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}
