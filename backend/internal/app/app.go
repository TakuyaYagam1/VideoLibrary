package app

import (
	"context"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

type App struct {
	config config.Config
}

func New(cfg config.Config) *App {
	return &App{
		config: cfg,
	}
}

func (a *App) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	logger := logkit.FromContext(ctx)
	logger.DebugContext(ctx, "logger configured",
		logkit.Component("app"),
		logkit.Fields{
			"log_level":  a.config.Log.Level,
			"log_output": a.config.Log.Output,
		},
	)
	logger.InfoContext(ctx, "application started",
		logkit.Component("app"),
		logkit.Fields{
			"app": a.config.App.Name,
			"env": a.config.App.Env,
		},
	)

	return nil
}
