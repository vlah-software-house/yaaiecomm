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

// Anthropic implements Provider for the Anthropic Messages API.
type Anthropic struct {
	apiKey string
	cfg    config.AIProviderConfig
	client *http.Client
}

func NewAnthropic(cfg config.AIProviderConfig) *Anthropic {
	return &Anthropic{
		apiKey: cfg.APIKey,
		cfg:    cfg,
		client: &http.Client{},
	}
}

func (a *Anthropic) Name() string { return "anthropic" }

func (a *Anthropic) Generate(ctx context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = a.cfg.ModelContent
		if model == "" {
			model = a.cfg.Model
		}
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}

	temp := req.Temperature
	if temp == 0 {
		temp = 0.7
	}

	body := anthropicRequest{
		Model:       model,
		MaxTokens:   maxTokens,
		Temperature: temp,
		System:      req.SystemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: req.UserPrompt},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var msgResp anthropicResponse
	if err := json.Unmarshal(respBody, &msgResp); err != nil {
		return Response{}, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(msgResp.Content) == 0 {
		return Response{}, fmt.Errorf("anthropic returned no content")
	}

	return Response{
		Content:  msgResp.Content[0].Text,
		Model:    msgResp.Model,
		Provider: "anthropic",
	}, nil
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
}

type anthropicResponse struct {
	Model   string `json:"model"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}
