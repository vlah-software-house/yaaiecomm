package admin

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/forgecommerce/api/internal/auth"
	"github.com/forgecommerce/api/internal/middleware"
	admin "github.com/forgecommerce/api/templates/admin"
)

// UserHandler handles admin user management endpoints.
type UserHandler struct {
	authSvc *auth.Service
	logger  *slog.Logger
}

// NewUserHandler creates a new user handler.
func NewUserHandler(authSvc *auth.Service, logger *slog.Logger) *UserHandler {
	return &UserHandler{
		authSvc: authSvc,
		logger:  logger,
	}
}

// RegisterRoutes registers user management routes on the given mux.
func (h *UserHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/users", h.ListUsers)
	mux.HandleFunc("GET /admin/users/new", h.NewUserForm)
	mux.HandleFunc("POST /admin/users", h.CreateUser)
	mux.HandleFunc("GET /admin/users/{id}", h.EditUserForm)
	mux.HandleFunc("POST /admin/users/{id}", h.UpdateUser)
	mux.HandleFunc("POST /admin/users/{id}/toggle-active", h.ToggleActive)
}

// ListUsers handles GET /admin/users.
func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.authSvc.ListUsers(r.Context())
	if err != nil {
		h.logger.Error("failed to list users", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	csrfToken := middleware.CSRFToken(r)
	data := admin.UserListData{
		Users:     users,
		CSRFToken: csrfToken,
	}

	admin.UserListPage(data).Render(r.Context(), w)
}

// NewUserForm handles GET /admin/users/new.
func (h *UserHandler) NewUserForm(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)
	data := admin.UserFormData{
		User:      nil,
		IsEdit:    false,
		CSRFToken: csrfToken,
	}

	admin.UserFormPage(data).Render(r.Context(), w)
}

// CreateUser handles POST /admin/users.
func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		h.renderUserFormWithError(w, r, nil, false, csrfToken, "Invalid form data.")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	role := r.FormValue("role")

	// Validate required fields.
	if name == "" || email == "" || password == "" || role == "" {
		h.renderUserFormWithError(w, r, nil, false, csrfToken, "All fields are required.")
		return
	}

	if len(password) < 8 {
		h.renderUserFormWithError(w, r, nil, false, csrfToken, "Password must be at least 8 characters.")
		return
	}

	if role != "admin" && role != "super_admin" {
		h.renderUserFormWithError(w, r, nil, false, csrfToken, "Invalid role selected.")
		return
	}

	_, err := h.authSvc.CreateUser(r.Context(), email, name, password, role, []string{})
	if err != nil {
		h.logger.Error("failed to create user", "error", err)
		h.renderUserFormWithError(w, r, nil, false, csrfToken, "Failed to create user: "+err.Error())
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// EditUserForm handles GET /admin/users/{id}.
func (h *UserHandler) EditUserForm(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := h.authSvc.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get user", "error", err, "user_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := admin.UserFormData{
		User:      user,
		IsEdit:    true,
		CSRFToken: csrfToken,
	}

	admin.UserFormPage(data).Render(r.Context(), w)
}

// UpdateUser handles POST /admin/users/{id}.
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.logger.Error("failed to parse form", "error", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	// Fetch the existing user so we can re-render the form on error.
	user, err := h.authSvc.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get user", "error", err, "user_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	role := r.FormValue("role")
	isActive := r.FormValue("is_active") == "on"

	if name == "" {
		h.renderUserFormWithError(w, r, user, true, csrfToken, "Name is required.")
		return
	}

	if role != "admin" && role != "super_admin" {
		h.renderUserFormWithError(w, r, user, true, csrfToken, "Invalid role selected.")
		return
	}

	_, err = h.authSvc.UpdateUser(r.Context(), id, name, role, []string{}, isActive)
	if err != nil {
		h.logger.Error("failed to update user", "error", err, "user_id", id)
		h.renderUserFormWithError(w, r, user, true, csrfToken, "Failed to update user.")
		return
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// ToggleActive handles POST /admin/users/{id}/toggle-active.
// Returns an HTMX fragment: a single table row for the updated user.
func (h *UserHandler) ToggleActive(w http.ResponseWriter, r *http.Request) {
	csrfToken := middleware.CSRFToken(r)

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := h.authSvc.GetUserByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, auth.ErrUserNotFound) {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		h.logger.Error("failed to get user for toggle", "error", err, "user_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	newActive := !user.IsActive
	if err := h.authSvc.SetUserActive(r.Context(), id, newActive); err != nil {
		h.logger.Error("failed to toggle user active", "error", err, "user_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Refresh the user to get the updated state.
	user, err = h.authSvc.GetUserByID(r.Context(), id)
	if err != nil {
		h.logger.Error("failed to reload user after toggle", "error", err, "user_id", id)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Render just the table row as an HTMX fragment.
	admin.UserTableRow(user, csrfToken).Render(r.Context(), w)
}

// renderUserFormWithError renders the user form with a 422 status and an error message.
func (h *UserHandler) renderUserFormWithError(w http.ResponseWriter, r *http.Request, user *auth.AdminUser, isEdit bool, csrfToken, errMsg string) {
	data := admin.UserFormData{
		User:      user,
		IsEdit:    isEdit,
		CSRFToken: csrfToken,
		Error:     errMsg,
	}
	w.WriteHeader(http.StatusUnprocessableEntity)
	admin.UserFormPage(data).Render(r.Context(), w)
}
