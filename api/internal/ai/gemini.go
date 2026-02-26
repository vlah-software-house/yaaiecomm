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

// Gemini implements Provider for the Google Gemini API.
type Gemini struct {
	apiKey string
	cfg    config.AIProviderConfig
	client *http.Client
}

func NewGemini(cfg config.AIProviderConfig) *Gemini {
	return &Gemini{
		apiKey: cfg.APIKey,
		cfg:    cfg,
		client: &http.Client{},
	}
}

func (g *Gemini) Name() string { return "gemini" }

func (g *Gemini) Generate(ctx context.Context, req Request) (Response, error) {
	model := req.Model
	if model == "" {
		model = g.cfg.Model
	}

	temp := req.Temperature
	if temp == 0 {
		temp = 0.7
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}

	body := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: req.UserPrompt},
				},
			},
		},
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{
				{Text: req.SystemPrompt},
			},
		},
		GenerationConfig: &geminiGenConfig{
			Temperature:     temp,
			MaxOutputTokens: maxTokens,
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return Response{}, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, g.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("gemini API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var gemResp geminiResponse
	if err := json.Unmarshal(respBody, &gemResp); err != nil {
		return Response{}, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(gemResp.Candidates) == 0 || len(gemResp.Candidates[0].Content.Parts) == 0 {
		return Response{}, fmt.Errorf("gemini returned no content")
	}

	return Response{
		Content:  gemResp.Candidates[0].Content.Parts[0].Text,
		Model:    model,
		Provider: "gemini",
	}, nil
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiGenConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}

type geminiRequest struct {
	Contents          []geminiContent  `json:"contents"`
	SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenConfig `json:"generationConfig,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
}
