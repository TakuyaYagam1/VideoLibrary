package redis

import (
	"context"
	"fmt"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	goredis "github.com/redis/go-redis/v9"
	"github.com/wahrwelt-kit/go-cachekit"
)

func NewCache(ctx context.Context, cfg config.Redis) (*goredis.Client, *cachekit.Cache, error) {
	client, err := cachekit.NewRedisClient(ctx, &cachekit.RedisConfig{
		Host:         cfg.Host,
		Port:         cfg.Port,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create redis client: %w", err)
	}

	return client, cachekit.New(client), nil
}
