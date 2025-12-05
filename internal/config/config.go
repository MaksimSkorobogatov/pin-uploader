package config

import (
	"encoding/base64"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ServerConfig represents configuration for the server component.
type ServerConfig struct {
	Port          int     `yaml:"port"`
	EncryptionKey string  `yaml:"encryption_key"`
	PublicBaseURL string  `yaml:"public_base_url"`
	DatabasePath  string  `yaml:"database_path"`
	BlackboxKey   string  `yaml:"blackbox_api_key"`
	BlackboxURL   string  `yaml:"blackbox_base_url"`
	Model         string  `yaml:"llm_model"`
	Temperature   float64 `yaml:"llm_temperature"`
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

	var cfg ServerConfig
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
