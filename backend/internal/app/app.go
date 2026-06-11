package app

import (
	"context"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo/persistent"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/usecase"
	pgconnector "github.com/TakuyaYagam1/VideoLibrary/backend/pkg/postgres"
	redisconnector "github.com/TakuyaYagam1/VideoLibrary/backend/pkg/redis"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/wahrwelt-kit/go-cachekit"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

type App struct {
	config       config.Config
	postgresPool *pgxpool.Pool
	redisClient  *redis.Client
	cache        *cachekit.Cache
	videoService *usecase.VideoService
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	logger := logkit.FromContext(ctx)

	pool, err := pgconnector.NewPool(ctx, cfg.PostgreSQL)
	if err != nil {
		logger.ErrorContext(ctx, "connect postgres", logkit.Component("app"), logkit.Error(err))
		return nil, err
	}

	if err := pgconnector.RunMigrations(ctx, cfg.PostgreSQL); err != nil {
		logger.ErrorContext(ctx, "run postgres migrations", logkit.Component("app"), logkit.Error(err))
		pool.Close()
		return nil, err
	}

	redisClient, cache, err := redisconnector.NewCache(ctx, cfg.Redis)
	if err != nil {
		logger.ErrorContext(ctx, "connect redis", logkit.Component("app"), logkit.Error(err))
		pool.Close()
		return nil, err
	}

	videoService, err := usecase.NewVideoService(
		persistent.NewVideoRepository(pool),
		cache,
		cfg.Cache.VideoListTTL,
	)
	if err != nil {
		logger.ErrorContext(ctx, "create video usecase", logkit.Component("app"), logkit.Error(err))
		_ = redisClient.Close()
		pool.Close()
		return nil, err
	}

	return &App{
		config:       cfg,
		postgresPool: pool,
		redisClient:  redisClient,
		cache:        cache,
		videoService: videoService,
	}, nil
}

func (a *App) Close() {
	if a.redisClient != nil {
		_ = a.redisClient.Close()
	}
	if a.postgresPool != nil {
		a.postgresPool.Close()
	}
}

func (a *App) Cache() *cachekit.Cache {
	return a.cache
}

func (a *App) VideoService() *usecase.VideoService {
	return a.videoService
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
