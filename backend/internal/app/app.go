package app

import (
	"context"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
)

type App struct {
	config config.Config
}

func New() *App {
	return &App{
		config: config.Default(),
	}
}

func (a *App) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	return nil
}
