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

// OpenAI implements Provider for the OpenAI API.
type OpenAI struct {
	apiKey string
	cfg    config.AIProviderConfig
	client *http.Client
}

func NewOpenAI(cfg config.AIProviderConfig) *OpenAI {
	return &OpenAI{
		apiKey: cfg.APIKey,
		cfg:    cfg,
		client: &http.Client{},
	}
}

func (o *OpenAI) Name() string { return "openai" }

func (o *OpenAI) Generate(ctx context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = o.cfg.ModelContent
		if model == "" {
			model = o.cfg.Model
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

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("openai API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp openAIChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return Response{}, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return Response{}, fmt.Errorf("openai returned no choices")
	}

	return Response{
		Content:  chatResp.Choices[0].Message.Content,
		Model:    chatResp.Model,
		Provider: "openai",
	}, nil
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
}

type openAIChatResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
}
