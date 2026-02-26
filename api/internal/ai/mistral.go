package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/forgecommerce/api/internal/config"
)

// Mistral implements Provider using the Mistral API (OpenAI-compatible format).
type Mistral struct {
	apiKey string
	cfg    config.AIProviderConfig
	client *http.Client
}

func NewMistral(cfg config.AIProviderConfig) *Mistral {
	return &Mistral{
		apiKey: cfg.APIKey,
		cfg:    cfg,
		client: &http.Client{},
	}
}

func (m *Mistral) Name() string { return "mistral" }

func (m *Mistral) Generate(ctx context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = m.cfg.Model
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}

	temp := req.Temperature
	if temp == 0 {
		temp = 0.7
	}

	messages := []openAIMessage{
		{Role: "system", Content: req.SystemPrompt},
		{Role: "user", Content: req.UserPrompt},
	}

	body := openAIChatRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temp,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.mistral.ai/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+m.apiKey)

	resp, err := m.client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("mistral request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("mistral API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp openAIChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return Response{}, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return Response{}, fmt.Errorf("mistral returned no choices")
	}

	return Response{
		Content:  chatResp.Choices[0].Message.Content,
		Model:    chatResp.Model,
		Provider: "mistral",
	}, nil
}
