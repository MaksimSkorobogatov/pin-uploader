package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"

	"github.com/MaksimSkorobogatov/pin-uploader/internal/client"
	"github.com/MaksimSkorobogatov/pin-uploader/internal/config"
)

func main() {
	defaultConfig := defaultClientConfigPath()
	configPath := flag.String("config", defaultConfig, "path to client config")
	flag.Parse()

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.LoadClientConfig(*configPath)
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}

	key, err := config.DecodeKey(cfg.EncryptionKey)
	if err != nil {
		logger.Fatal("decode key", zap.Error(err))
	}

	paths, err := client.ExpandGlobs(flag.Args())
	if err != nil {
		logger.Fatal("expand file paths", zap.Error(err))
	}
	if len(paths) == 0 {
		fmt.Println("no files provided")
		os.Exit(1)
	}

	uploader := client.NewUploader(cfg.ServerAddress, key, logger)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()

	for _, p := range paths {
		res := uploader.UploadFile(ctx, p)
		if res.Error != nil {
			fmt.Printf("❌ %s: %v\n", p, res.Error)
			continue
		}
		fmt.Printf("✅ %s -> ID %d | GUID %s | Title: %s\n", p, res.Meta.ID, res.Meta.GUID, res.Meta.Title)
	}
}

func defaultClientConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "configs/client.yaml"
	}
	return filepath.Join(home, ".config", "pin-uploader", "config.yaml")
}
