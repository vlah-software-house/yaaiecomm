package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/forgecommerce/api/internal/auth"
)

const (
	// CustomerIDKey is the context key for the authenticated customer's UUID.
	CustomerIDKey contextKey = "customer_id"
	// CustomerEmailKey is the context key for the authenticated customer's email.
	CustomerEmailKey contextKey = "customer_email"
)

// RequireCustomerAuth returns middleware that validates a JWT Bearer token
// and injects the customer ID and email into the request context.
// Unauthenticated requests receive a 401 JSON response.
func RequireCustomerAuth(jwtMgr *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				writeJSONError(w, http.StatusUnauthorized, "invalid authorization format")
				return
			}

			claims, err := jwtMgr.ValidateToken(parts[1])
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), CustomerIDKey, claims.CustomerID)
			ctx = context.WithValue(ctx, CustomerEmailKey, claims.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CustomerFromContext extracts the customer ID from the request context.
// Returns uuid.Nil and false if no customer is authenticated.
func CustomerFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(CustomerIDKey).(uuid.UUID)
	return id, ok
}

// writeJSONError writes a JSON error response. This avoids importing
// the handlers/api package to prevent circular dependencies.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Simple manual JSON to avoid circular imports.
	w.Write([]byte(`{"error":"` + msg + `"}`))
}
