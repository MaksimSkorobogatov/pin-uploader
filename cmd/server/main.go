package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/MaksimSkorobogatov/pin-uploader/internal/config"
	"github.com/MaksimSkorobogatov/pin-uploader/internal/server"
)

func main() {
	configPath := flag.String("config", "/etc/pin-uploader.yaml", "path to server config")
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.LoadServerConfig(*configPath)
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}

	key, err := config.DecodeKey(cfg.EncryptionKey)
	if err != nil {
		logger.Fatal("decode encryption key", zap.Error(err))
	}

	addr := cfg.ListenAddress
	baseURL := cfg.PublicBaseURL
	if baseURL == "" {
		baseURL = "https://" + addr
	}

	storage, err := server.NewStorage(cfg.DatabasePath, logger)
	if err != nil {
		logger.Fatal("init storage", zap.Error(err))
	}
	defer storage.Close()

	llm, err := server.NewLLMClient(cfg.LLMAPIKey, cfg.LLMBaseURL, cfg.Model, cfg.Temperature, cfg.PromptParams, cfg.LLMTimeout, logger)
	if err != nil {
		logger.Fatal("init llm", zap.Error(err))
	}

	srv := server.NewServer(addr, baseURL, cfg.DefaultPinLink, cfg.PinDescription, key, storage, llm, cfg.MaxConcurrentUploads, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		logger.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", zap.Error(err))
		}
	}()

	if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatal("server failed", zap.Error(err))
	}
}
