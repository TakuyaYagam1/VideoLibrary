package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	if err := app.New(cfg).Run(ctx); err != nil {
		log.Fatal(err)
	}
}
