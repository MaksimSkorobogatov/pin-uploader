package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	appcrypto "github.com/MaksimSkorobogatov/pin-uploader/internal/crypto"
)

// Server aggregates HTTP handlers and dependencies.
type Server struct {
	logger         *zap.Logger
	storage        *Storage
	llm            *LLMClient
	key            []byte
	baseURL        string
	defaultPinLink string
	httpSrv        *http.Server
	shutdown       chan struct{}
}

// UploadPayload is the decrypted request payload.
type UploadPayload struct {
	Filename   string  `json:"filename"`
	DataBase64 string  `json:"data"`
	PinLink    *string `json:"pin_link"`
}

// UploadResponse contains metadata echoed back to the client.
type UploadResponse struct {
	ID          int64    `json:"id"`
	GUID        string   `json:"guid"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// NewServer constructs the HTTP server.
func NewServer(addr, baseURL, defaultPinLink string, key []byte, storage *Storage, llm *LLMClient, logger *zap.Logger) *Server {
	mux := http.NewServeMux()
	s := &Server{
		logger:         logger,
		storage:        storage,
		llm:            llm,
		key:            key,
		baseURL:        baseURL,
		defaultPinLink: defaultPinLink,
		shutdown:       make(chan struct{}),
	}

	mux.HandleFunc("/rss", s.handleRSS)
	mux.HandleFunc("/rss/image/", s.handleImage)
	mux.HandleFunc("/rss/upload", s.handleUpload)

	s.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  90 * time.Second,
	}
	return s
}

// Start runs the HTTP server.
func (s *Server) Start() error {
	s.logger.Info("server starting", zap.String("addr", s.httpSrv.Addr))
	return s.httpSrv.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	defer close(s.shutdown)
	return s.httpSrv.Shutdown(ctx)
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 5<<20)) // 5MB cap
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("read body: %w", err))
		return
	}
	defer r.Body.Close()

	plaintext, err := appcrypto.Decrypt(s.key, body)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("decrypt payload: %w", err))
		return
	}

	var payload UploadPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("decode payload: %w", err))
		return
	}

	data, err := base64.StdEncoding.DecodeString(payload.DataBase64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("decode data: %w", err))
		return
	}

	hash := computeHash(data)
	seen, err := s.storage.ExistsHash(r.Context(), hash)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	if seen {
		s.logger.Warn("duplicate image upload skipped", zap.String("filename", payload.Filename), zap.String("hash", hash))
		s.writeError(w, http.StatusConflict, fmt.Errorf("image already uploaded"))
		return
	}

	mimeType := http.DetectContentType(data)
	if mimeType == "application/octet-stream" {
		mimeType = mime.TypeByExtension(filepath.Ext(payload.Filename))
	}

	switch mimeType {
	case "image/jpeg", "image/jpg", "image/png":
	default:
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("unsupported media type: %s", mimeType))
		return
	}

	meta, err := s.llm.GenerateMetadata(r.Context(), mimeType, data)
	if err != nil {
		s.logger.Info("llm error, served fallback", zap.Error(err))
	}

	now := time.Now()
	guid := uuid.NewString()

	pin := Pin{
		Filename:    filepath.Base(payload.Filename),
		UploadedAt:  now,
		Title:       meta.Title,
		Description: meta.Description,
		GUID:        guid,
		PubDate:     now,
		MimeType:    mimeType,
		Data:        data,
		Hash:        hash,
		Link:        payload.PinLink,
	}

	id, err := s.storage.InsertPin(r.Context(), pin)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Errorf("store pin: %w", err))
		return
	}

	resp := UploadResponse{
		ID:          id,
		GUID:        guid,
		Title:       meta.Title,
		Description: meta.Description,
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRSS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pins, err := s.storage.ListPins(r.Context(), 200)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Errorf("fetch pins: %w", err))
		return
	}

	feed, err := BuildRSS(pins, s.baseURL, s.defaultPinLink)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Errorf("build rss: %w", err))
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(feed))
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/rss/image/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("invalid id: %w", err))
		return
	}

	mimeType, data, err := s.storage.GetPinData(r.Context(), id)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, sql.ErrNoRows) || strings.Contains(strings.ToLower(err.Error()), "no rows") {
			http.NotFound(w, r)
			return
		}
		s.writeError(w, http.StatusInternalServerError, fmt.Errorf("fetch image: %w", err))
		return
	}

	w.Header().Set("Content-Type", mimeType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (s *Server) writeError(w http.ResponseWriter, code int, err error) {
	s.logger.Warn("request failed", zap.Int("status", code), zap.Error(err))
	s.writeJSON(w, code, map[string]string{"error": err.Error()})
}

func (s *Server) writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	if err := enc.Encode(v); err != nil {
		s.logger.Error("encode json", zap.Error(err))
	}
}

func computeHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
