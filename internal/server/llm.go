package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"go.uber.org/zap"
)

// LLMClient wraps langchaingo model for metadata generation.
type LLMClient struct {
	llm         llms.Model
	model       string
	temperature float64
	logger      *zap.Logger
	timeout     time.Duration
}

// Metadata describes generated AI metadata.
type Metadata struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// NewLLMClient constructs a client using OpenAI-compatible Blackbox endpoint.
func NewLLMClient(apiKey, baseURL, model string, temperature float64, logger *zap.Logger) (*LLMClient, error) {
	opts := []openai.Option{
		openai.WithToken(apiKey),
		openai.WithBaseURL(baseURL),
	}
	if model != "" {
		opts = append(opts, openai.WithModel(model))
	}

	llm, err := openai.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("init llm: %w", err)
	}

	if temperature <= 0 {
		temperature = 0.7
	}

	return &LLMClient{
		llm:         llm,
		model:       model,
		temperature: temperature,
		logger:      logger,
		timeout:     30 * time.Second,
	}, nil
}

// GenerateMetadata asks the model for structured metadata. Falls back to deterministic output on failure.
func (c *LLMClient) GenerateMetadata(ctx context.Context, filename string, preview []byte) (Metadata, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	prompt := fmt.Sprintf(`You are an assistant that writes Pinterest pin metadata.
Generate a short, catchy title, a 2-3 sentence description, and 3-6 tags.
Use the file name and context to infer subject. Reply strictly in JSON with fields "title", "description", "tags".
Filename: %s
ImageBytesSize: %d
`, filename, len(preview))

	resp, err := llms.GenerateFromSinglePrompt(ctx, c.llm, prompt, llms.WithTemperature(c.temperature))
	if err != nil {
		c.logger.Warn("llm generation failed, using fallback", zap.Error(err))
		return c.fallback(filename), err
	}

	var meta Metadata
	if err := json.Unmarshal([]byte(resp), &meta); err != nil {
		c.logger.Warn("failed to parse llm json, using fallback", zap.Error(err))
		return c.fallback(filename), err
	}

	if len(meta.Tags) == 0 {
		meta.Tags = []string{"pinterest", "ai-generated"}
	}

	return meta, nil
}

func (c *LLMClient) fallback(filename string) Metadata {
	title := fmt.Sprintf("Pin for %s", filename)
	desc := "AI generated placeholder description for uploaded image."
	return Metadata{
		Title:       title,
		Description: desc,
		Tags:        []string{"placeholder", "ai", "pin"},
	}
}
