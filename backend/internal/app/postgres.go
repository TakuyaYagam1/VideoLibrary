package app

import (
	"context"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	"github.com/jackc/pgx/v5/pgxpool"
	pgkitgoose "github.com/wahrwelt-kit/go-pgkit/migrator/goose"
	pgkitpostgres "github.com/wahrwelt-kit/go-pgkit/postgres"
)

func NewPostgresPool(ctx context.Context, cfg config.PostgreSQL) (*pgxpool.Pool, error) {
	return pgkitpostgres.New(ctx, &pgkitpostgres.Config{
		URL:            cfg.DSN,
		MaxConns:       cfg.MaxConns,
		MinConns:       cfg.MinConns,
		RetryTimeout:   cfg.RetryTimeout,
		ConnectTimeout: cfg.ConnectTimeout,
	})
}

func RunMigrations(ctx context.Context, cfg config.PostgreSQL) error {
	return pgkitgoose.Run(ctx, cfg.DSN, cfg.MigrationsPath)
}
