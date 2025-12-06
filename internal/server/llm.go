package server

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"go.uber.org/zap"

	"github.com/MaksimSkorobogatov/pin-uploader/internal/config"
)

const (
	defaultTemperature = 0.7
	defaultTimeout     = 30 * time.Second
	defaultPinDescLang = "English"
)

// LLMClient wraps langchaingo model for metadata generation.
type LLMClient struct {
	llm          llms.Model
	model        string
	temperature  float64
	logger       *zap.Logger
	timeout      time.Duration
	promptParams config.PromptParams
}

//go:embed prompt_template.md
var llmPromptTemplate string

// Metadata describes generated AI metadata.
type Metadata struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// NewLLMClient constructs a client using an OpenAI-compatible LLM endpoint.
func NewLLMClient(apiKey, baseURL, model string, temperature float64, promptParams config.PromptParams, logger *zap.Logger) (*LLMClient, error) {
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
		temperature = defaultTemperature
	}

	return &LLMClient{
		llm:          llm,
		model:        model,
		temperature:  temperature,
		logger:       logger,
		timeout:      defaultTimeout,
		promptParams: promptParams,
	}, nil
}

func getPrompt(params config.PromptParams) (string, error) {
	if params.PinDescriptionLanguage == "" {
		params.PinDescriptionLanguage = defaultPinDescLang
	}

	tpl, err := template.New("prompt").Parse(llmPromptTemplate)
	if err != nil {
		return "", fmt.Errorf("parse prompt template: %w", err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("execute prompt template: %w", err)
	}

	return buf.String(), nil
}

// GenerateMetadata asks the model for structured metadata. Falls back to deterministic output on failure.
func (c *LLMClient) GenerateMetadata(ctx context.Context, filename string, preview []byte) (Metadata, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	prompt, err := getPrompt(c.promptParams)
	if err != nil {
		c.logger.Fatal("build prompt failed", zap.Error(err))
	}

	mime := http.DetectContentType(preview)
	dataURL := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(preview)

	messages := []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextPart(prompt),
				llms.ImageURLPart(dataURL),
			},
		},
	}

	resp, err := c.llm.GenerateContent(ctx, messages, llms.WithTemperature(c.temperature))
	if err != nil {
		c.logger.Warn("llm generation failed, using fallback", zap.Error(err))
		return c.fallback(), err
	}

	if len(resp.Choices) == 0 || resp.Choices[0] == nil {
		c.logger.Warn("llm returned empty choices, using fallback")
		return c.fallback(), errors.New("empty llm response")
	}

	var meta Metadata
	if err := json.Unmarshal([]byte(resp.Choices[0].Content), &meta); err != nil {
		c.logger.Warn("failed to parse llm json, using fallback", zap.Error(err))
		return c.fallback(), err
	}

	return meta, nil
}

func (c *LLMClient) fallback() Metadata {
	return Metadata{
		Title:       "photography",
		Description: "",
	}
}
