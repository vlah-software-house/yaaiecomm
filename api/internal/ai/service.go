package ai

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
)

// Task identifies what kind of content to generate.
type Task string

const (
	TaskDescription      Task = "description"
	TaskShortDescription Task = "short_description"
	TaskSEOTitle         Task = "seo_title"
	TaskSEODescription   Task = "seo_description"
	TaskSuggestAttrs     Task = "suggest_attributes"
	TaskAltText          Task = "alt_text"
)

// GenerateParams is the input for content generation.
type GenerateParams struct {
	Provider    string            // provider name (empty = default)
	Task        Task              // what to generate
	ProductName string            // required context
	Category    string            // optional category
	Context     map[string]string // extra context (existing description, etc.)
}

// Service orchestrates AI content generation for products.
type Service struct {
	registry *Registry
	logger   *slog.Logger
}

// NewService creates an AI service from a provider registry.
func NewService(registry *Registry, logger *slog.Logger) *Service {
	return &Service{
		registry: registry,
		logger:   logger,
	}
}

// Generate produces content for the given task and product context.
func (s *Service) Generate(ctx context.Context, params GenerateParams) (Response, error) {
	var provider Provider
	var err error

	if params.Provider != "" {
		provider, err = s.registry.Get(params.Provider)
	} else {
		provider, err = s.registry.Default()
	}
	if err != nil {
		return Response{}, err
	}

	system, user := buildPrompt(params)

	s.logger.Info("AI generation requested",
		slog.String("provider", provider.Name()),
		slog.String("task", string(params.Task)),
		slog.String("product", params.ProductName),
	)

	resp, err := provider.Generate(ctx, Request{
		SystemPrompt: system,
		UserPrompt:   user,
		MaxTokens:    maxTokensForTask(params.Task),
		Temperature:  temperatureForTask(params.Task),
	})
	if err != nil {
		return Response{}, fmt.Errorf("generate %s: %w", params.Task, err)
	}

	// Clean up common AI artifacts (markdown code fences, quotes, etc.)
	resp.Content = cleanResponse(resp.Content, params.Task)

	return resp, nil
}

// Available returns the names of configured providers.
func (s *Service) Available() []string {
	return s.registry.Available()
}

// HasProviders returns true if at least one provider is configured.
func (s *Service) HasProviders() bool {
	return s.registry.HasProviders()
}

const systemBase = `You are a professional e-commerce copywriter for a European online store.
You write compelling, accurate product content that drives conversions.
Write in English unless instructed otherwise.
Never include markdown formatting, code fences, or quotation marks around your output.
Return ONLY the requested content, nothing else.`

func buildPrompt(p GenerateParams) (system, user string) {
	system = systemBase

	categoryCtx := ""
	if p.Category != "" {
		categoryCtx = fmt.Sprintf("\nProduct category: %s", p.Category)
	}

	existingDesc := ""
	if d, ok := p.Context["description"]; ok && d != "" {
		existingDesc = fmt.Sprintf("\nExisting product description:\n%s", d)
	}

	existingShort := ""
	if d, ok := p.Context["short_description"]; ok && d != "" {
		existingShort = fmt.Sprintf("\nExisting short description: %s", d)
	}

	switch p.Task {
	case TaskDescription:
		user = fmt.Sprintf(`Write a compelling product description for: "%s"%s%s

Requirements:
- 2-4 paragraphs, 150-300 words
- Highlight key benefits and features
- Use persuasive but honest language
- Include sensory details where appropriate
- End with a subtle call-to-action
- Do NOT include the product name as a heading`, p.ProductName, categoryCtx, existingShort)

	case TaskShortDescription:
		user = fmt.Sprintf(`Write a short product description (1-2 sentences, max 160 characters) for: "%s"%s%s

This appears as a tagline or summary on product listing pages.
Be concise, compelling, and highlight the main value proposition.`, p.ProductName, categoryCtx, existingDesc)

	case TaskSEOTitle:
		user = fmt.Sprintf(`Write an SEO-optimized page title for this product: "%s"%s

Requirements:
- Maximum 60 characters (strict limit)
- Include the product name or key terms
- Make it compelling for search results
- Do NOT include brand name or site name
- Return only the title text`, p.ProductName, categoryCtx)

	case TaskSEODescription:
		user = fmt.Sprintf(`Write an SEO meta description for this product: "%s"%s%s

Requirements:
- Maximum 155 characters (strict limit)
- Include a call-to-action
- Mention key selling points
- Make it compelling for search result clicks
- Return only the description text`, p.ProductName, categoryCtx, existingDesc)

	case TaskSuggestAttrs:
		system = systemBase + `
When suggesting attributes, return a JSON array of objects with this structure:
[{"name": "color", "display_name": "Color", "type": "color_swatch", "options": ["Black", "White", "Red"]},
 {"name": "size", "display_name": "Size", "type": "button_group", "options": ["S", "M", "L", "XL"]}]
Valid types: select, color_swatch, button_group, image_swatch
Return ONLY valid JSON, no other text.`

		user = fmt.Sprintf(`Suggest product attributes for: "%s"%s

Consider what a customer would want to choose when purchasing this product.
Suggest 2-5 relevant attributes with common options.
Only suggest attributes that make sense for this specific product type.`, p.ProductName, categoryCtx)

	case TaskAltText:
		variantInfo := ""
		if v, ok := p.Context["variant"]; ok && v != "" {
			variantInfo = fmt.Sprintf("\nThis image shows the %s variant.", v)
		}
		filename := ""
		if f, ok := p.Context["filename"]; ok && f != "" {
			filename = fmt.Sprintf("\nImage filename: %s", f)
		}

		user = fmt.Sprintf(`Write alt text for a product image of: "%s"%s%s

Requirements:
- Maximum 125 characters
- Descriptive and specific
- Include product name and relevant details
- Useful for accessibility and SEO
- Return only the alt text`, p.ProductName, variantInfo, filename)

	default:
		user = fmt.Sprintf(`Generate content for: "%s"`, p.ProductName)
	}

	return system, user
}

func maxTokensForTask(task Task) int {
	switch task {
	case TaskDescription:
		return 1024
	case TaskShortDescription, TaskSEOTitle, TaskSEODescription, TaskAltText:
		return 256
	case TaskSuggestAttrs:
		return 1024
	default:
		return 512
	}
}

func temperatureForTask(task Task) float64 {
	switch task {
	case TaskSEOTitle, TaskSEODescription, TaskAltText:
		return 0.5 // more deterministic for SEO/structured content
	case TaskSuggestAttrs:
		return 0.3 // low temp for structured JSON
	default:
		return 0.7
	}
}

func cleanResponse(content string, task Task) string {
	content = strings.TrimSpace(content)
	// Remove markdown code fences
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	// Remove surrounding quotes
	if len(content) >= 2 && content[0] == '"' && content[len(content)-1] == '"' {
		content = content[1 : len(content)-1]
	}
	return content
}
