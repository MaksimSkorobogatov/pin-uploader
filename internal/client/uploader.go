package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	appcrypto "github.com/MaksimSkorobogatov/pin-uploader/internal/crypto"
)

// UploadResult represents a single upload outcome.
type UploadResult struct {
	Path  string
	Error error
	Meta  *UploadResponse
}

// UploadResponse mirrors the server response.
type UploadResponse struct {
	ID          int64    `json:"id"`
	GUID        string   `json:"guid"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// UploadPayload is encrypted and sent to server.
type UploadPayload struct {
	Filename   string `json:"filename"`
	DataBase64 string `json:"data"`
}

// Uploader handles encrypted uploads.
type Uploader struct {
	serverURL string
	key       []byte
	client    *http.Client
	logger    *zap.Logger
}

// NewUploader creates a new uploader instance.
func NewUploader(serverURL string, key []byte, logger *zap.Logger) *Uploader {
	return &Uploader{
		serverURL: trimTrailingSlash(serverURL),
		key:       key,
		logger:    logger,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ExpandGlobs expands glob patterns and deduplicates paths.
func ExpandGlobs(patterns []string) ([]string, error) {
	seen := make(map[string]struct{})
	var paths []string
	for _, p := range patterns {
		matches, err := filepath.Glob(p)
		if err != nil {
			return nil, fmt.Errorf("glob %s: %w", p, err)
		}
		if len(matches) == 0 {
			matches = []string{p} // treat as literal path
		}
		for _, m := range matches {
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			paths = append(paths, m)
		}
	}
	return paths, nil
}

// UploadFile uploads a single file.
func (u *Uploader) UploadFile(ctx context.Context, path string) UploadResult {
	data, err := os.ReadFile(path)
	if err != nil {
		return UploadResult{Path: path, Error: fmt.Errorf("read file: %w", err)}
	}

	payload := UploadPayload{
		Filename:   filepath.Base(path),
		DataBase64: base64.StdEncoding.EncodeToString(data),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return UploadResult{Path: path, Error: fmt.Errorf("encode payload: %w", err)}
	}

	cipher, err := appcrypto.Encrypt(u.key, raw)
	if err != nil {
		return UploadResult{Path: path, Error: fmt.Errorf("encrypt: %w", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.serverURL+"/upload", bytes.NewReader(cipher))
	if err != nil {
		return UploadResult{Path: path, Error: fmt.Errorf("build request: %w", err)}
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := u.client.Do(req)
	if err != nil {
		return UploadResult{Path: path, Error: fmt.Errorf("send request: %w", err)}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return UploadResult{Path: path, Error: fmt.Errorf("server returned %s: %s", resp.Status, string(body))}
	}

	var meta UploadResponse
	if err := json.Unmarshal(body, &meta); err != nil {
		return UploadResult{Path: path, Error: fmt.Errorf("decode response: %w", err)}
	}

	return UploadResult{Path: path, Meta: &meta}
}

func trimTrailingSlash(url string) string {
	return strings.TrimRight(url, "/")
}
