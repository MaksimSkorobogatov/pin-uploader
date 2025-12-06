package server

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"go.uber.org/zap"

	"github.com/MaksimSkorobogatov/pin-uploader/internal/config"
)

const (
	defaultTemperature = 0.7
	defaultTimeout     = time.Minute
	defaultPinDescLang = "English"
)

// LLMClient wraps langchaingo model for metadata generation.
type LLMClient struct {
	llm         llms.Model
	model       string
	temperature float64
	logger      *zap.Logger
	timeout     time.Duration
	prompt      string
}

//go:embed prompt_template.md
var llmPromptTemplate string

// Metadata describes generated AI metadata.
type Metadata struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// NewLLMClient constructs a client using an OpenAI-compatible LLM endpoint.
func NewLLMClient(apiKey, baseURL, model string, temperature float64, promptParams config.PromptParams, timeout time.Duration, logger *zap.Logger) (*LLMClient, error) {
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

	if timeout <= 0 {
		timeout = defaultTimeout
	}

	prompt, err := getPrompt(promptParams)
	if err != nil {
		logger.Fatal("build prompt failed", zap.Error(err))
	}

	return &LLMClient{
		llm:         llm,
		model:       model,
		temperature: temperature,
		logger:      logger,
		timeout:     timeout,
		prompt:      prompt,
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
func (c *LLMClient) GenerateMetadata(ctx context.Context, mimeType string, fileData []byte) (Metadata, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	fileDataBase64 := base64.StdEncoding.EncodeToString(fileData)

	const (
		dataIdent   = "data:"
		base64Ident = ";base64,"
	)
	dataURL := strings.Builder{}
	dataURL.Grow(len(dataIdent) + len(base64Ident) + len(fileDataBase64) + len(mimeType))
	dataURL.WriteString(dataIdent)
	dataURL.WriteString(mimeType)
	dataURL.WriteString(base64Ident)
	dataURL.WriteString(fileDataBase64)

	messages := []llms.MessageContent{
		{
			Role: llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{
				llms.TextPart(c.prompt),
				llms.ImageURLPart(dataURL.String()),
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

	if len(resp.Choices[0].Content) == 0 {
		c.logger.Warn("llm returned empty content, using fallback")
		return c.fallback(), errors.New("empty llm response")
	}

	var meta Metadata
	if err := json.Unmarshal([]byte(resp.Choices[0].Content), &meta); err != nil {
		c.logger.Warn("failed to parse llm json, using fallback", zap.Error(err), zap.Any("resp", resp))
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
