package ai

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/forgecommerce/api/internal/config"
)

// Provider is implemented by each AI backend (OpenAI, Gemini, Mistral).
type Provider interface {
	Name() string
	Generate(ctx context.Context, req Request) (Response, error)
}

// Request is the input to an AI generation call.
type Request struct {
	SystemPrompt string
	UserPrompt   string
	Model        string  // model override; empty = use provider default
	MaxTokens    int     // 0 = provider default
	Temperature  float64 // 0 = provider default (usually 0.7)
}

// Response is the output from an AI generation call.
type Response struct {
	Content  string
	Model    string
	Provider string
}

// Registry holds all configured AI providers.
type Registry struct {
	providers map[string]Provider
	logger    *slog.Logger
}

// NewRegistry creates a Registry from the application AI config.
func NewRegistry(cfg config.AIConfig, logger *slog.Logger) *Registry {
	r := &Registry{
		providers: make(map[string]Provider),
		logger:    logger,
	}

	if cfg.OpenAI.APIKey != "" {
		r.providers["openai"] = NewOpenAI(cfg.OpenAI)
		logger.Info("AI provider registered", "provider", "openai", "model", cfg.OpenAI.Model)
	}
	if cfg.Gemini.APIKey != "" {
		r.providers["gemini"] = NewGemini(cfg.Gemini)
		logger.Info("AI provider registered", "provider", "gemini", "model", cfg.Gemini.Model)
	}
	if cfg.Mistral.APIKey != "" {
		r.providers["mistral"] = NewMistral(cfg.Mistral)
		logger.Info("AI provider registered", "provider", "mistral", "model", cfg.Mistral.Model)
	}
	if cfg.Anthropic.APIKey != "" {
		r.providers["anthropic"] = NewAnthropic(cfg.Anthropic)
		logger.Info("AI provider registered", "provider", "anthropic", "model", cfg.Anthropic.Model)
	}

	return r
}

// Get returns a provider by name, or an error if not found.
func (r *Registry) Get(name string) (Provider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("AI provider %q not configured", name)
	}
	return p, nil
}

// Default returns the first available provider, preferring openai > gemini > mistral.
func (r *Registry) Default() (Provider, error) {
	for _, name := range []string{"openai", "anthropic", "gemini", "mistral"} {
		if p, ok := r.providers[name]; ok {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no AI providers configured")
}

// Available returns the names of all configured providers.
func (r *Registry) Available() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// HasProviders returns true if at least one provider is registered.
func (r *Registry) HasProviders() bool {
	return len(r.providers) > 0
}
