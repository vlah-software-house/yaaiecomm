package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCSRFTokenGeneration_Length(t *testing.T) {
	token := generateCSRFToken()
	// csrfTokenLen = 32 bytes, hex encoded = 64 characters.
	expectedLen := csrfTokenLen * 2
	if len(token) != expectedLen {
		t.Errorf("expected CSRF token length %d, got %d", expectedLen, len(token))
	}
}

func TestCSRFTokenGeneration_Uniqueness(t *testing.T) {
	token1 := generateCSRFToken()
	token2 := generateCSRFToken()
	if token1 == token2 {
		t.Errorf("expected unique tokens, got identical: %s", token1)
	}
}

func TestCSRF_SafeMethodsBypass(t *testing.T) {
	safeMethods := []string{http.MethodGet, http.MethodHead, http.MethodOptions}

	handler := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, method := range safeMethods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/test", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("safe method %s should pass CSRF, got status %d", method, rr.Code)
			}
		})
	}
}

func TestCSRF_SetsCookieOnGET(t *testing.T) {
	handler := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should set a CSRF cookie.
	cookies := rr.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == csrfCookieName {
			found = true
			if !c.HttpOnly {
				t.Error("CSRF cookie should be HttpOnly")
			}
			if !c.Secure {
				t.Error("CSRF cookie should be Secure")
			}
			if c.SameSite != http.SameSiteStrictMode {
				t.Errorf("CSRF cookie SameSite should be Strict, got %v", c.SameSite)
			}
			if len(c.Value) != csrfTokenLen*2 {
				t.Errorf("CSRF cookie value length: want %d, got %d", csrfTokenLen*2, len(c.Value))
			}
		}
	}
	if !found {
		t.Error("expected CSRF cookie to be set on GET request")
	}
}

func TestCSRF_MutationWithoutToken(t *testing.T) {
	handler := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/test", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusForbidden {
				t.Errorf("mutation %s without CSRF token should return 403, got %d", method, rr.Code)
			}
		})
	}
}

func TestCSRF_MutationWithValidHeaderToken(t *testing.T) {
	handler := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	token := generateCSRFToken()

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	req.Header.Set(csrfHeaderName, token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("POST with valid CSRF token should return 200, got %d", rr.Code)
	}
}

func TestCSRF_MutationWithValidFormToken(t *testing.T) {
	handler := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	token := generateCSRFToken()

	body := csrfFormField + "=" + token
	req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("POST with valid form CSRF token should return 200, got %d", rr.Code)
	}
}

func TestCSRF_MutationWithWrongToken(t *testing.T) {
	handler := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	cookieToken := generateCSRFToken()
	wrongToken := generateCSRFToken()

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: cookieToken})
	req.Header.Set(csrfHeaderName, wrongToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("POST with wrong CSRF token should return 403, got %d", rr.Code)
	}
}

func TestCSRF_MutationWithCookieButNoRequestToken(t *testing.T) {
	handler := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	token := generateCSRFToken()

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	// No header or form field set.
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("POST with cookie but no request token should return 403, got %d", rr.Code)
	}
}

func TestCSRFToken_FromContext(t *testing.T) {
	var capturedToken string

	handler := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedToken = CSRFToken(r)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if capturedToken == "" {
		t.Error("expected CSRF token to be available in request context after GET")
	}
}

func TestCSRFToken_MissingContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	token := CSRFToken(req)
	if token != "" {
		t.Errorf("expected empty token when no CSRF middleware, got %q", token)
	}
}

func TestCSRF_ReusesExistingToken(t *testing.T) {
	handler := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	existingToken := "existing-token-value"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: existingToken})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rr.Code)
	}

	// Should NOT set a new cookie since one already exists.
	for _, c := range rr.Result().Cookies() {
		if c.Name == csrfCookieName {
			t.Error("should not set new CSRF cookie when one already exists")
		}
	}
}

func TestCSRF_MutationValidTokenInContext(t *testing.T) {
	var capturedToken string
	handler := CSRF(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedToken = CSRFToken(r)
		w.WriteHeader(http.StatusOK)
	}))

	token := generateCSRFToken()
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	req.Header.Set(csrfHeaderName, token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rr.Code)
	}
	if capturedToken != token {
		t.Errorf("context token: got %q, want %q", capturedToken, token)
	}
}

func TestGetCSRFCookie_NoCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	token := getCSRFCookie(req)
	if token != "" {
		t.Errorf("expected empty string for missing cookie, got %q", token)
	}
}

func TestGetCSRFCookie_WithCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "mytoken123"})
	token := getCSRFCookie(req)
	if token != "mytoken123" {
		t.Errorf("expected 'mytoken123', got %q", token)
	}
}
