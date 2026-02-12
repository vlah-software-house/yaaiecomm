package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
)

const (
	csrfCookieName = "_csrf_token"
	csrfHeaderName = "X-CSRF-Token"
	csrfFormField  = "csrf_token"
	csrfTokenLen   = 32
)

// CSRF protects against cross-site request forgery.
// GET/HEAD/OPTIONS are exempt. All other methods require a valid token.
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Safe methods don't need CSRF validation
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			token := getOrCreateCSRFToken(w, r)
			ctx := context.WithValue(r.Context(), CSRFTokenKey, token)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Validate CSRF token for mutation methods
		cookieToken := getCSRFCookie(r)
		if cookieToken == "" {
			http.Error(w, "CSRF token missing", http.StatusForbidden)
			return
		}

		// Check header first, then form field
		requestToken := r.Header.Get(csrfHeaderName)
		if requestToken == "" {
			requestToken = r.FormValue(csrfFormField)
		}

		if requestToken == "" || subtle.ConstantTimeCompare([]byte(cookieToken), []byte(requestToken)) != 1 {
			http.Error(w, "CSRF token invalid", http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), CSRFTokenKey, cookieToken)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CSRFToken returns the CSRF token from the request context.
func CSRFToken(r *http.Request) string {
	if token, ok := r.Context().Value(CSRFTokenKey).(string); ok {
		return token
	}
	return ""
}

func getOrCreateCSRFToken(w http.ResponseWriter, r *http.Request) string {
	if token := getCSRFCookie(r); token != "" {
		return token
	}
	token := generateCSRFToken()
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	return token
}

func getCSRFCookie(r *http.Request) string {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func generateCSRFToken() string {
	b := make([]byte, csrfTokenLen)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
