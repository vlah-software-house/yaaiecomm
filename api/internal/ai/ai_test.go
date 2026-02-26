package ai

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/forgecommerce/api/internal/config"
)

// --------------------------------------------------------------------------
// Registry tests
// --------------------------------------------------------------------------

func TestNewRegistry_RegistersConfiguredProviders(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg := config.AIConfig{
		OpenAI:  config.AIProviderConfig{APIKey: "sk-test", Model: "gpt-4o"},
		Gemini:  config.AIProviderConfig{APIKey: "gem-test", Model: "gemini-2.0-flash"},
		Mistral: config.AIProviderConfig{APIKey: "mis-test", Model: "mistral-large"},
	}

	r := NewRegistry(cfg, logger)

	if !r.HasProviders() {
		t.Fatal("expected HasProviders=true")
	}

	avail := r.Available()
	if len(avail) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(avail))
	}

	// Get each by name
	for _, name := range []string{"openai", "gemini", "mistral"} {
		p, err := r.Get(name)
		if err != nil {
			t.Errorf("Get(%q) returned error: %v", name, err)
		}
		if p.Name() != name {
			t.Errorf("Get(%q).Name() = %q", name, p.Name())
		}
	}
}

func TestNewRegistry_EmptyConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := NewRegistry(config.AIConfig{}, logger)

	if r.HasProviders() {
		t.Fatal("expected HasProviders=false for empty config")
	}
	if len(r.Available()) != 0 {
		t.Fatal("expected no available providers")
	}

	_, err := r.Default()
	if err == nil {
		t.Fatal("expected error from Default() with no providers")
	}
}

func TestRegistry_GetUnknownProvider(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := NewRegistry(config.AIConfig{}, logger)

	_, err := r.Get("anthropic")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestRegistry_DefaultPreference(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Only gemini + mistral configured — default should be gemini (2nd in preference)
	cfg := config.AIConfig{
		Gemini:  config.AIProviderConfig{APIKey: "gem-test", Model: "gemini-2.0-flash"},
		Mistral: config.AIProviderConfig{APIKey: "mis-test", Model: "mistral-large"},
	}

	r := NewRegistry(cfg, logger)
	p, err := r.Default()
	if err != nil {
		t.Fatalf("Default() error: %v", err)
	}
	if p.Name() != "gemini" {
		t.Errorf("Default() = %q, want gemini (second in preference order)", p.Name())
	}
}

func TestRegistry_DefaultPrefersOpenAI(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg := config.AIConfig{
		OpenAI:  config.AIProviderConfig{APIKey: "sk-test", Model: "gpt-4o"},
		Gemini:  config.AIProviderConfig{APIKey: "gem-test", Model: "gemini-2.0-flash"},
		Mistral: config.AIProviderConfig{APIKey: "mis-test", Model: "mistral-large"},
	}

	r := NewRegistry(cfg, logger)
	p, err := r.Default()
	if err != nil {
		t.Fatalf("Default() error: %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("Default() = %q, want openai (first in preference order)", p.Name())
	}
}

// --------------------------------------------------------------------------
// Provider tests (using httptest mock servers)
// --------------------------------------------------------------------------

// newOpenAIServer returns a test server that mimics the OpenAI API.
func newOpenAIServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("missing Authorization header")
		}

		var reqBody openAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(reqBody.Messages) < 2 {
			t.Error("expected at least 2 messages (system + user)")
		}

		resp := openAIChatResponse{
			Model: reqBody.Model,
			Choices: []struct {
				Message openAIMessage `json:"message"`
			}{
				{Message: openAIMessage{Role: "assistant", Content: content}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestOpenAI_Generate(t *testing.T) {
	srv := newOpenAIServer(t, "A beautifully crafted leather bag.")
	defer srv.Close()

	provider := &OpenAI{
		apiKey: "sk-test",
		cfg:    config.AIProviderConfig{Model: "gpt-4o", ModelContent: "gpt-4o-mini"},
		client: srv.Client(),
	}
	// Override endpoint by wrapping the transport
	provider.client.Transport = rewriteTransport{base: srv.Client().Transport, url: srv.URL}

	resp, err := provider.Generate(context.Background(), Request{
		SystemPrompt: "You are a copywriter.",
		UserPrompt:   "Write a description for Leather Bag",
		MaxTokens:    256,
		Temperature:  0.7,
	})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if resp.Content != "A beautifully crafted leather bag." {
		t.Errorf("Content = %q", resp.Content)
	}
	if resp.Provider != "openai" {
		t.Errorf("Provider = %q, want openai", resp.Provider)
	}
}

func TestOpenAI_ModelFallback(t *testing.T) {
	var receivedModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openAIChatRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedModel = req.Model
		resp := openAIChatResponse{
			Model: req.Model,
			Choices: []struct {
				Message openAIMessage `json:"message"`
			}{
				{Message: openAIMessage{Role: "assistant", Content: "test"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// No ModelContent set — should fall back to Model
	provider := &OpenAI{
		apiKey: "sk-test",
		cfg:    config.AIProviderConfig{Model: "gpt-4o"},
		client: &http.Client{Transport: rewriteTransport{base: srv.Client().Transport, url: srv.URL}},
	}

	provider.Generate(context.Background(), Request{
		SystemPrompt: "sys",
		UserPrompt:   "usr",
	})
	if receivedModel != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o (fallback from empty ModelContent)", receivedModel)
	}
}

func TestOpenAI_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "Rate limit exceeded"}}`))
	}))
	defer srv.Close()

	provider := &OpenAI{
		apiKey: "sk-test",
		cfg:    config.AIProviderConfig{Model: "gpt-4o"},
		client: &http.Client{Transport: rewriteTransport{base: srv.Client().Transport, url: srv.URL}},
	}

	_, err := provider.Generate(context.Background(), Request{
		SystemPrompt: "sys",
		UserPrompt:   "usr",
	})
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
}

func TestGemini_Generate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var req geminiRequest
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Contents) == 0 || len(req.Contents[0].Parts) == 0 {
			t.Error("expected contents with parts")
		}
		if req.SystemInstruction == nil {
			t.Error("expected system instruction")
		}

		resp := geminiResponse{
			Candidates: []struct {
				Content geminiContent `json:"content"`
			}{
				{Content: geminiContent{Parts: []geminiPart{{Text: "Gemini generated text."}}}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	provider := &Gemini{
		apiKey: "gem-test",
		cfg:    config.AIProviderConfig{Model: "gemini-2.0-flash"},
		client: &http.Client{Transport: rewriteTransport{base: srv.Client().Transport, url: srv.URL}},
	}

	resp, err := provider.Generate(context.Background(), Request{
		SystemPrompt: "You are a copywriter.",
		UserPrompt:   "Write a description",
		MaxTokens:    256,
		Temperature:  0.5,
	})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if resp.Content != "Gemini generated text." {
		t.Errorf("Content = %q", resp.Content)
	}
	if resp.Provider != "gemini" {
		t.Errorf("Provider = %q, want gemini", resp.Provider)
	}
}

func TestMistral_Generate(t *testing.T) {
	srv := newOpenAIServer(t, "Mistral generated text.")
	defer srv.Close()

	provider := &Mistral{
		apiKey: "mis-test",
		cfg:    config.AIProviderConfig{Model: "mistral-large"},
		client: &http.Client{Transport: rewriteTransport{base: srv.Client().Transport, url: srv.URL}},
	}

	resp, err := provider.Generate(context.Background(), Request{
		SystemPrompt: "You are a copywriter.",
		UserPrompt:   "Write a description",
	})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if resp.Content != "Mistral generated text." {
		t.Errorf("Content = %q", resp.Content)
	}
	if resp.Provider != "mistral" {
		t.Errorf("Provider = %q, want mistral", resp.Provider)
	}
}

// rewriteTransport redirects all requests to the test server URL.
type rewriteTransport struct {
	base http.RoundTripper
	url  string
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.url[len("http://"):]
	if t.base != nil {
		return t.base.RoundTrip(req)
	}
	return http.DefaultTransport.RoundTrip(req)
}

// --------------------------------------------------------------------------
// Service tests
// --------------------------------------------------------------------------

// mockProvider is a test provider that returns configured content.
type mockProvider struct {
	name    string
	content string
	err     error
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Generate(_ context.Context, _ Request) (Response, error) {
	if m.err != nil {
		return Response{}, m.err
	}
	return Response{Content: m.content, Model: "test-model", Provider: m.name}, nil
}

func newTestService(providers ...Provider) *Service {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := &Registry{
		providers: make(map[string]Provider),
		logger:    logger,
	}
	for _, p := range providers {
		r.providers[p.Name()] = p
	}
	return &Service{registry: r, logger: logger}
}

func TestService_Generate_DefaultProvider(t *testing.T) {
	svc := newTestService(&mockProvider{name: "openai", content: "Generated description"})

	resp, err := svc.Generate(context.Background(), GenerateParams{
		Task:        TaskDescription,
		ProductName: "Leather Bag",
	})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if resp.Content != "Generated description" {
		t.Errorf("Content = %q", resp.Content)
	}
	if resp.Provider != "openai" {
		t.Errorf("Provider = %q", resp.Provider)
	}
}

func TestService_Generate_SpecificProvider(t *testing.T) {
	svc := newTestService(
		&mockProvider{name: "openai", content: "OpenAI text"},
		&mockProvider{name: "gemini", content: "Gemini text"},
	)

	resp, err := svc.Generate(context.Background(), GenerateParams{
		Provider:    "gemini",
		Task:        TaskShortDescription,
		ProductName: "Canvas Tote",
	})
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if resp.Content != "Gemini text" {
		t.Errorf("Content = %q, want Gemini text", resp.Content)
	}
}

func TestService_Generate_UnknownProvider(t *testing.T) {
	svc := newTestService(&mockProvider{name: "openai", content: "text"})

	_, err := svc.Generate(context.Background(), GenerateParams{
		Provider:    "anthropic",
		Task:        TaskDescription,
		ProductName: "Test",
	})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestService_Generate_NoProviders(t *testing.T) {
	svc := newTestService()

	_, err := svc.Generate(context.Background(), GenerateParams{
		Task:        TaskDescription,
		ProductName: "Test",
	})
	if err == nil {
		t.Fatal("expected error when no providers configured")
	}
}

func TestService_HasProviders(t *testing.T) {
	empty := newTestService()
	if empty.HasProviders() {
		t.Error("expected HasProviders=false for empty service")
	}

	with := newTestService(&mockProvider{name: "openai", content: "x"})
	if !with.HasProviders() {
		t.Error("expected HasProviders=true")
	}
}

func TestService_Available(t *testing.T) {
	svc := newTestService(
		&mockProvider{name: "openai"},
		&mockProvider{name: "gemini"},
	)
	avail := svc.Available()
	if len(avail) != 2 {
		t.Errorf("Available() returned %d providers, want 2", len(avail))
	}
}

// --------------------------------------------------------------------------
// Prompt building tests
// --------------------------------------------------------------------------

func TestBuildPrompt_AllTasks(t *testing.T) {
	tasks := []struct {
		task        Task
		expectSys   bool // system prompt should not be empty
		expectUser  bool // user prompt should contain product name
	}{
		{TaskDescription, true, true},
		{TaskShortDescription, true, true},
		{TaskSEOTitle, true, true},
		{TaskSEODescription, true, true},
		{TaskSuggestAttrs, true, true},
		{TaskAltText, true, true},
	}

	for _, tt := range tasks {
		t.Run(string(tt.task), func(t *testing.T) {
			sys, usr := buildPrompt(GenerateParams{
				Task:        tt.task,
				ProductName: "Test Product",
				Category:    "Accessories",
				Context: map[string]string{
					"description":       "A great product.",
					"short_description": "Great stuff.",
					"variant":           "Black/Large",
					"filename":          "test.jpg",
				},
			})
			if sys == "" {
				t.Error("system prompt is empty")
			}
			if usr == "" {
				t.Error("user prompt is empty")
			}
			// All user prompts should reference the product name
			if !contains(usr, "Test Product") {
				t.Errorf("user prompt does not contain product name: %s", usr)
			}
		})
	}
}

func TestBuildPrompt_CategoryContext(t *testing.T) {
	_, usr := buildPrompt(GenerateParams{
		Task:        TaskDescription,
		ProductName: "Canvas Bag",
		Category:    "Bags & Accessories",
	})
	if !contains(usr, "Bags & Accessories") {
		t.Error("user prompt should include category context")
	}
}

func TestBuildPrompt_NoCategory(t *testing.T) {
	_, usr := buildPrompt(GenerateParams{
		Task:        TaskDescription,
		ProductName: "Canvas Bag",
	})
	if contains(usr, "category") {
		t.Error("user prompt should not mention category when not provided")
	}
}

// --------------------------------------------------------------------------
// Clean response tests
// --------------------------------------------------------------------------

func TestCleanResponse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text", "Hello world", "Hello world"},
		{"with whitespace", "  Hello world  ", "Hello world"},
		{"json code fence", "```json\n[{\"name\":\"color\"}]\n```", `[{"name":"color"}]`},
		{"generic code fence", "```\nsome code\n```", "some code"},
		{"surrounding quotes", `"A great product."`, "A great product."},
		{"no surrounding quotes on longer text", `Some "quoted" words inside.`, `Some "quoted" words inside.`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanResponse(tt.input, TaskDescription)
			if got != tt.want {
				t.Errorf("cleanResponse(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --------------------------------------------------------------------------
// Task config tests
// --------------------------------------------------------------------------

func TestMaxTokensForTask(t *testing.T) {
	if got := maxTokensForTask(TaskDescription); got != 1024 {
		t.Errorf("description max_tokens = %d, want 1024", got)
	}
	if got := maxTokensForTask(TaskSEOTitle); got != 256 {
		t.Errorf("seo_title max_tokens = %d, want 256", got)
	}
	if got := maxTokensForTask(TaskSuggestAttrs); got != 1024 {
		t.Errorf("suggest_attributes max_tokens = %d, want 1024", got)
	}
	if got := maxTokensForTask("unknown"); got != 512 {
		t.Errorf("unknown task max_tokens = %d, want 512", got)
	}
}

func TestTemperatureForTask(t *testing.T) {
	if got := temperatureForTask(TaskDescription); got != 0.7 {
		t.Errorf("description temp = %f, want 0.7", got)
	}
	if got := temperatureForTask(TaskSEOTitle); got != 0.5 {
		t.Errorf("seo_title temp = %f, want 0.5", got)
	}
	if got := temperatureForTask(TaskSuggestAttrs); got != 0.3 {
		t.Errorf("suggest_attributes temp = %f, want 0.3", got)
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
