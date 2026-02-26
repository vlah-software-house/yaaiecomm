package admin

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/forgecommerce/api/internal/ai"
)

// AIHandler serves AI content generation endpoints for the admin panel.
type AIHandler struct {
	aiSvc  *ai.Service
	logger *slog.Logger
}

// NewAIHandler creates a new AI handler.
func NewAIHandler(aiSvc *ai.Service, logger *slog.Logger) *AIHandler {
	return &AIHandler{
		aiSvc:  aiSvc,
		logger: logger,
	}
}

// RegisterRoutes registers AI admin routes on the given mux.
func (h *AIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /admin/ai/generate", h.Generate)
	mux.HandleFunc("GET /admin/ai/providers", h.ListProviders)
}

type generateRequest struct {
	Provider    string            `json:"provider"`
	Task        string            `json:"task"`
	ProductName string            `json:"product_name"`
	Category    string            `json:"category"`
	Context     map[string]string `json:"context"`
}

type generateResponse struct {
	Content  string `json:"content"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// Generate handles POST /admin/ai/generate.
func (h *AIHandler) Generate(w http.ResponseWriter, r *http.Request) {
	if !h.aiSvc.HasProviders() {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "No AI providers configured"})
		return
	}

	var req generateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "Invalid request body"})
		return
	}

	if req.ProductName == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "Product name is required"})
		return
	}

	if req.Task == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "Task is required"})
		return
	}

	resp, err := h.aiSvc.Generate(r.Context(), ai.GenerateParams{
		Provider:    req.Provider,
		Task:        ai.Task(req.Task),
		ProductName: req.ProductName,
		Category:    req.Category,
		Context:     req.Context,
	})
	if err != nil {
		h.logger.Error("AI generation failed", "error", err, "task", req.Task, "provider", req.Provider)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "Generation failed: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, generateResponse{
		Content:  resp.Content,
		Provider: resp.Provider,
		Model:    resp.Model,
	})
}

type providersResponse struct {
	Providers []string `json:"providers"`
}

// ListProviders handles GET /admin/ai/providers.
func (h *AIHandler) ListProviders(w http.ResponseWriter, r *http.Request) {
	providers := h.aiSvc.Available()
	if providers == nil {
		providers = []string{}
	}
	writeJSON(w, http.StatusOK, providersResponse{Providers: providers})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
