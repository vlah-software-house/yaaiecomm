package admin

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/forgecommerce/api/internal/ai"
	"github.com/forgecommerce/api/internal/config"
)

func newTestAIHandler(t *testing.T, providers ...config.AIProviderConfig) *AIHandler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg := config.AIConfig{}
	for _, p := range providers {
		// Use the APIKey suffix to determine which provider slot to fill
		if p.APIKey == "openai" {
			cfg.OpenAI = p
		} else if p.APIKey == "gemini" {
			cfg.Gemini = p
		} else if p.APIKey == "mistral" {
			cfg.Mistral = p
		}
	}

	registry := ai.NewRegistry(cfg, logger)
	svc := ai.NewService(registry, logger)
	return NewAIHandler(svc, logger)
}

func TestAIHandler_ListProviders_Empty(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	registry := ai.NewRegistry(config.AIConfig{}, logger)
	svc := ai.NewService(registry, logger)
	h := NewAIHandler(svc, logger)

	req := httptest.NewRequest("GET", "/admin/ai/providers", nil)
	rr := httptest.NewRecorder()
	h.ListProviders(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp providersResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Providers) != 0 {
		t.Errorf("expected empty providers list, got %v", resp.Providers)
	}
}

func TestAIHandler_ListProviders_WithProviders(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg := config.AIConfig{
		OpenAI: config.AIProviderConfig{APIKey: "sk-test", Model: "gpt-4o"},
	}
	registry := ai.NewRegistry(cfg, logger)
	svc := ai.NewService(registry, logger)
	h := NewAIHandler(svc, logger)

	req := httptest.NewRequest("GET", "/admin/ai/providers", nil)
	rr := httptest.NewRecorder()
	h.ListProviders(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var resp providersResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if len(resp.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(resp.Providers))
	}
	if resp.Providers[0] != "openai" {
		t.Errorf("provider = %q, want openai", resp.Providers[0])
	}
}

func TestAIHandler_Generate_NoProviders(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	registry := ai.NewRegistry(config.AIConfig{}, logger)
	svc := ai.NewService(registry, logger)
	h := NewAIHandler(svc, logger)

	body := `{"task":"description","product_name":"Test"}`
	req := httptest.NewRequest("POST", "/admin/ai/generate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}

func TestAIHandler_Generate_InvalidBody(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg := config.AIConfig{
		OpenAI: config.AIProviderConfig{APIKey: "sk-test", Model: "gpt-4o"},
	}
	registry := ai.NewRegistry(cfg, logger)
	svc := ai.NewService(registry, logger)
	h := NewAIHandler(svc, logger)

	req := httptest.NewRequest("POST", "/admin/ai/generate", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestAIHandler_Generate_MissingProductName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg := config.AIConfig{
		OpenAI: config.AIProviderConfig{APIKey: "sk-test", Model: "gpt-4o"},
	}
	registry := ai.NewRegistry(cfg, logger)
	svc := ai.NewService(registry, logger)
	h := NewAIHandler(svc, logger)

	body := `{"task":"description","product_name":""}`
	req := httptest.NewRequest("POST", "/admin/ai/generate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}

	var resp errorResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "Product name is required" {
		t.Errorf("error = %q", resp.Error)
	}
}

func TestAIHandler_Generate_MissingTask(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg := config.AIConfig{
		OpenAI: config.AIProviderConfig{APIKey: "sk-test", Model: "gpt-4o"},
	}
	registry := ai.NewRegistry(cfg, logger)
	svc := ai.NewService(registry, logger)
	h := NewAIHandler(svc, logger)

	body := `{"task":"","product_name":"Leather Bag"}`
	req := httptest.NewRequest("POST", "/admin/ai/generate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.Generate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}

	var resp errorResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "Task is required" {
		t.Errorf("error = %q", resp.Error)
	}
}

func TestAIHandler_WriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, http.StatusCreated, map[string]string{"foo": "bar"})

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var result map[string]string
	json.NewDecoder(rr.Body).Decode(&result)
	if result["foo"] != "bar" {
		t.Errorf("response body = %v", result)
	}
}
