package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ServerConfig represents configuration for the server component.
type ServerConfig struct {
	ListenAddress string        `yaml:"listen_address"`
	EncryptionKey string        `yaml:"encryption_key"`
	PublicBaseURL string        `yaml:"public_base_url"`
	DatabasePath  string        `yaml:"database_path"`
	LLMAPIKey     string        `yaml:"llm_api_key"`
	LLMBaseURL    string        `yaml:"llm_base_url"`
	Model         string        `yaml:"llm_model"`
	Temperature   float64       `yaml:"llm_temperature"`
	LLMTimeout    time.Duration `yaml:"llm_timeout"`
	PromptParams  PromptParams  `yaml:"prompt_params"`
}

type PromptParams struct {
	PinDescriptionLanguage string `yaml:"pin_description_language"`
}

func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		ListenAddress: "localhost:8080",
		DatabasePath:  "~/.local/share/pin-uploader/db.sqlite",
		PromptParams: PromptParams{
			PinDescriptionLanguage: "English",
		},
	}
}

// ClientConfig represents configuration for the CLI uploader.
type ClientConfig struct {
	ServerAddress string `yaml:"server_address"`
	EncryptionKey string `yaml:"encryption_key"`
}

// LoadServerConfig reads and parses the server configuration from disk.
func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read server config: %w", err)
	}

	cfg := DefaultServerConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse server config: %w", err)
	}

	return &cfg, nil
}

// LoadClientConfig reads and parses the client configuration from disk.
func LoadClientConfig(path string) (*ClientConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read client config: %w", err)
	}

	var cfg ClientConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse client config: %w", err)
	}

	return &cfg, nil
}

// DecodeKey accepts a base64-encoded key and returns its bytes. Keys must be 32 bytes.
func DecodeKey(encoded string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode encryption key: %w", err)
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("invalid encryption key length %d, expected 32 bytes", len(key))
	}

	return key, nil
}
