package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/app"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	logger, err := app.NewLogger(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer closeLogger(logger)

	ctx = logkit.IntoContext(ctx, logger)

	application, err := app.New(ctx, cfg)
	if err != nil {
		logger.Error("initialize application", logkit.Error(err))
		closeLogger(logger)
		os.Exit(1)
	}
	defer application.Close()

	if err := application.Run(ctx); err != nil {
		logger.Error("application failed", logkit.Error(err))
		closeLogger(logger)
		os.Exit(1)
	}
}

func closeLogger(logger logkit.Logger) {
	if err := logger.Close(); err != nil {
		log.Printf("close logger: %v", err)
	}
}
