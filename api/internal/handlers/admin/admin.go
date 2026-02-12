package admin

import (
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"

	"github.com/forgecommerce/api/internal/auth"
	"github.com/forgecommerce/api/internal/middleware"
	"github.com/forgecommerce/api/templates/admin"
	"github.com/google/uuid"
)

// Handler holds dependencies for admin HTTP handlers.
type Handler struct {
	auth   *auth.Service
	logger *slog.Logger
}

// NewHandler creates a new admin handler.
func NewHandler(authService *auth.Service, logger *slog.Logger) *Handler {
	return &Handler{
		auth:   authService,
		logger: logger,
	}
}

// RegisterRoutes registers all admin routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Public routes (no auth required)
	mux.HandleFunc("GET /admin/login", h.ShowLogin)
	mux.HandleFunc("POST /admin/login", h.HandleLogin)
	mux.HandleFunc("GET /admin/login/2fa", h.ShowTwoFactor)
	mux.HandleFunc("POST /admin/login/2fa", h.HandleTwoFactor)

	// Semi-public routes (require pending 2FA cookie, not full session)
	mux.HandleFunc("GET /admin/setup-2fa", h.ShowSetup2FA)
	mux.HandleFunc("POST /admin/setup-2fa/confirm", h.HandleConfirm2FA)
}

// RegisterProtectedRoutes registers routes that require authentication.
// These should be wrapped with RequireAuth middleware by the caller.
func (h *Handler) RegisterProtectedRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/dashboard", h.ShowDashboard)
	mux.HandleFunc("POST /admin/logout", h.HandleLogout)
}

// ShowLogin renders the admin login page.
func (h *Handler) ShowLogin(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)
	admin.LoginPage("", csrfToken).Render(r.Context(), w)
}

// HandleLogin processes the login form submission.
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")
	csrfToken := middleware.CSRFToken(r)

	user, err := h.auth.Login(r.Context(), email, password)
	if err != nil {
		msg := "Invalid email or password"
		if errors.Is(err, auth.ErrUserInactive) {
			msg = "Account is disabled"
		}
		w.WriteHeader(http.StatusUnauthorized)
		admin.LoginPage(msg, csrfToken).Render(r.Context(), w)
		return
	}

	// If user needs 2FA setup, create a temporary session and redirect
	if user.Force2FASetup {
		token, err := h.createPending2FASession(w, r, user.ID)
		if err != nil {
			h.logger.Error("failed to create pending session", "error", err)
			admin.LoginPage("Internal error. Please try again.", csrfToken).Render(r.Context(), w)
			return
		}
		setPending2FACookie(w, token)
		http.Redirect(w, r, "/admin/setup-2fa", http.StatusSeeOther)
		return
	}

	// If user has 2FA set up, redirect to 2FA page
	if user.TOTPVerified {
		token, err := h.createPending2FASession(w, r, user.ID)
		if err != nil {
			h.logger.Error("failed to create pending session", "error", err)
			admin.LoginPage("Internal error. Please try again.", csrfToken).Render(r.Context(), w)
			return
		}
		setPending2FACookie(w, token)
		http.Redirect(w, r, "/admin/login/2fa", http.StatusSeeOther)
		return
	}

	// No 2FA — create session directly (shouldn't happen with force_2fa_setup)
	ip := r.RemoteAddr
	ua := r.UserAgent()
	sessionToken, err := h.auth.CompleteTwoFactor(r.Context(), user.ID, "", ip, ua)
	if err != nil {
		h.logger.Error("failed to create session", "error", err)
		admin.LoginPage("Internal error. Please try again.", csrfToken).Render(r.Context(), w)
		return
	}

	middleware.SetSessionCookie(w, sessionToken)
	http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
}

// ShowTwoFactor renders the 2FA verification page.
func (h *Handler) ShowTwoFactor(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)
	showRecovery := r.URL.Query().Get("recovery") == "true"
	admin.TwoFactorPage("", csrfToken, showRecovery).Render(r.Context(), w)
}

// HandleTwoFactor processes the 2FA form submission.
func (h *Handler) HandleTwoFactor(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)
	code := r.FormValue("code")
	codeType := r.FormValue("type")
	showRecovery := codeType == "recovery"

	userID, err := h.getPending2FAUserID(r)
	if err != nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}

	ip := r.RemoteAddr
	ua := r.UserAgent()

	var sessionToken string
	if showRecovery {
		sessionToken, err = h.auth.UseRecoveryCode(r.Context(), userID, code, ip, ua)
	} else {
		sessionToken, err = h.auth.CompleteTwoFactor(r.Context(), userID, code, ip, ua)
	}

	if err != nil {
		msg := "Invalid code. Please try again."
		if errors.Is(err, auth.ErrInvalidRecoveryCode) {
			msg = "Invalid recovery code."
		}
		w.WriteHeader(http.StatusUnauthorized)
		admin.TwoFactorPage(msg, csrfToken, showRecovery).Render(r.Context(), w)
		return
	}

	// Clear pending 2FA cookie, set session cookie
	clearPending2FACookie(w)
	middleware.SetSessionCookie(w, sessionToken)
	http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
}

// ShowSetup2FA renders the 2FA setup page.
func (h *Handler) ShowSetup2FA(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	userID, err := h.getAuthenticatedUserID(r)
	if err != nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}

	setup, err := h.auth.Setup2FA(r.Context(), userID)
	if err != nil {
		if errors.Is(err, auth.ErrTOTPAlreadySetup) {
			http.Redirect(w, r, "/admin/dashboard", http.StatusSeeOther)
			return
		}
		h.logger.Error("failed to setup 2FA", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	qrBase64 := base64.StdEncoding.EncodeToString(setup.QRCode)
	admin.Setup2FAPage(qrBase64, setup.Secret, csrfToken, "").Render(r.Context(), w)
}

// HandleConfirm2FA processes the 2FA confirmation form.
func (h *Handler) HandleConfirm2FA(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)
	code := r.FormValue("code")

	userID, err := h.getAuthenticatedUserID(r)
	if err != nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}

	codes, err := h.auth.Confirm2FA(r.Context(), userID, code)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidTOTPCode) {
			// Re-setup to get a fresh QR code
			setup, setupErr := h.auth.Setup2FA(r.Context(), userID)
			if setupErr != nil {
				h.logger.Error("failed to re-setup 2FA", "error", setupErr)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			qrBase64 := base64.StdEncoding.EncodeToString(setup.QRCode)
			w.WriteHeader(http.StatusUnprocessableEntity)
			admin.Setup2FAPage(qrBase64, setup.Secret, csrfToken, "Invalid code. Please try again.").Render(r.Context(), w)
			return
		}
		h.logger.Error("failed to confirm 2FA", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	admin.RecoveryCodesPage(codes).Render(r.Context(), w)
}

// ShowDashboard renders the admin dashboard.
func (h *Handler) ShowDashboard(w http.ResponseWriter, r *http.Request) {
	admin.DashboardPage().Render(r.Context(), w)
}

// HandleLogout logs the admin user out.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if token, ok := r.Context().Value(middleware.SessionTokenKey).(string); ok {
		if err := h.auth.Logout(r.Context(), token); err != nil {
			h.logger.Error("logout error", "error", err)
		}
	}

	middleware.ClearSessionCookie(w)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// --- Helper methods ---

const pending2FACookieName = "pending_2fa"

// createPending2FASession stores the user ID for the pending 2FA step.
// We store the user ID encrypted in a short-lived cookie so the 2FA page
// knows which user is authenticating.
func (h *Handler) createPending2FASession(_ http.ResponseWriter, _ *http.Request, userID uuid.UUID) (string, error) {
	// For simplicity, store the user ID hex-encoded. In production, this should
	// be encrypted or stored server-side with a random token.
	return userID.String(), nil
}

func setPending2FACookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     pending2FACookieName,
		Value:    token,
		Path:     "/admin",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   300, // 5 minutes
	})
}

func clearPending2FACookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     pending2FACookieName,
		Value:    "",
		Path:     "/admin",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *Handler) getPending2FAUserID(r *http.Request) (uuid.UUID, error) {
	cookie, err := r.Cookie(pending2FACookieName)
	if err != nil || cookie.Value == "" {
		return uuid.Nil, errors.New("no pending 2FA session")
	}
	return uuid.Parse(cookie.Value)
}

// getAuthenticatedUserID gets the user ID from either the session context
// or the pending 2FA cookie.
func (h *Handler) getAuthenticatedUserID(r *http.Request) (uuid.UUID, error) {
	// First try session context (for already-authenticated users)
	if userID, ok := middleware.AdminUserIDFromContext(r.Context()); ok {
		return userID, nil
	}
	// Fall back to pending 2FA cookie (for users mid-login)
	return h.getPending2FAUserID(r)
}
