package app

import (
	"context"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	"github.com/jackc/pgx/v5/pgxpool"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

type App struct {
	config       config.Config
	postgresPool *pgxpool.Pool
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	logger := logkit.FromContext(ctx)

	pool, err := NewPostgresPool(ctx, cfg.PostgreSQL)
	if err != nil {
		logger.ErrorContext(ctx, "connect postgres", logkit.Component("app"), logkit.Error(err))
		return nil, err
	}

	if err := RunMigrations(ctx, cfg.PostgreSQL); err != nil {
		logger.ErrorContext(ctx, "run postgres migrations", logkit.Component("app"), logkit.Error(err))
		pool.Close()
		return nil, err
	}

	return &App{
		config:       cfg,
		postgresPool: pool,
	}, nil
}

func (a *App) Close() {
	if a.postgresPool != nil {
		a.postgresPool.Close()
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
