package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

type Cache struct {
	client *goredis.Client
	group  singleflight.Group
}

const cacheLoadTimeout = 5 * time.Second

var (
	_ repo.Cache            = (*Cache)(nil)
	_ repo.CacheInvalidator = (*Cache)(nil)
)

func NewCache(client *goredis.Client) *Cache {
	if client == nil {
		return nil
	}

	return &Cache{client: client}
}

func (c *Cache) GetOrLoadVideos(
	ctx context.Context,
	key string,
	ttl time.Duration,
	loadFn func(context.Context) ([]domain.Video, error),
) ([]domain.Video, error) {
	if c == nil || c.client == nil {
		return nil, errors.New("redis cache is not configured")
	}
	if key == "" {
		return nil, errors.New("redis cache key is empty")
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("redis cache ttl must be greater than 0: %s", ttl)
	}

	cached, err := c.client.Get(ctx, key).Bytes()
	if err == nil {
		var videos []domain.Video
		if unmarshalErr := json.Unmarshal(cached, &videos); unmarshalErr != nil {
			c.group.Forget(key)
			if invalidateErr := c.bumpVersionAndDelete(ctx, key); invalidateErr != nil {
				return nil, fmt.Errorf("invalidate corrupt cached videos: %w", invalidateErr)
			}

			return c.loadAndCacheVideos(ctx, key, ttl, loadFn)
		}

		return videos, nil
	}
	if !errors.Is(err, goredis.Nil) {
		return nil, fmt.Errorf("get cached videos: %w", err)
	}

	value, err, _ := c.group.Do(key, func() (any, error) {
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cacheLoadTimeout)
		defer cancel()

		return c.loadAndCacheVideos(loadCtx, key, ttl, loadFn)
	})
	if err != nil {
		return nil, err
	}

	videos, ok := value.([]domain.Video)
	if !ok {
		return nil, fmt.Errorf("unexpected cached videos type %T", value)
	}

	return videos, nil
}

func (c *Cache) loadAndCacheVideos(
	ctx context.Context,
	key string,
	ttl time.Duration,
	loadFn func(context.Context) ([]domain.Video, error),
) ([]domain.Video, error) {
	version, versionErr := c.version(ctx, key)
	if versionErr != nil {
		return nil, versionErr
	}

	videos, loadErr := loadFn(ctx)
	if loadErr != nil {
		return nil, fmt.Errorf("load videos: %w", loadErr)
	}

	payload, marshalErr := json.Marshal(videos)
	if marshalErr != nil {
		return nil, fmt.Errorf("marshal videos: %w", marshalErr)
	}

	ttlMillis := ttl.Milliseconds()
	if ttlMillis <= 0 {
		ttlMillis = 1
	}

	if _, scriptErr := setIfVersionUnchanged.Run(
		ctx,
		c.client,
		[]string{key, c.versionKey(key)},
		strconv.FormatUint(version, 10),
		payload,
		ttlMillis,
	).Int(); scriptErr != nil {
		return videos, fmt.Errorf("cache loaded videos: %w", scriptErr)
	}

	return videos, nil
}

func (c *Cache) Del(ctx context.Context, keys ...string) error {
	if c == nil || c.client == nil {
		return errors.New("redis cache is not configured")
	}

	for _, key := range keys {
		if key == "" {
			return errors.New("redis cache key is empty")
		}
		c.group.Forget(key)
		if err := c.bumpVersionAndDelete(ctx, key); err != nil {
			return err
		}
	}

	return nil
}

func (c *Cache) version(ctx context.Context, key string) (uint64, error) {
	value, err := c.client.Get(ctx, c.versionKey(key)).Result()
	if errors.Is(err, goredis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get cache version: %w", err)
	}

	version, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse cache version %q: %w", value, err)
	}

	return version, nil
}

func (c *Cache) bumpVersionAndDelete(ctx context.Context, key string) error {
	if _, err := c.client.Pipelined(ctx, func(pipe goredis.Pipeliner) error {
		pipe.Incr(ctx, c.versionKey(key))
		pipe.Del(ctx, key)
		return nil
	}); err != nil {
		return fmt.Errorf("invalidate cache key %q: %w", key, err)
	}

	return nil
}

func (c *Cache) versionKey(key string) string {
	return key + ":version"
}

var setIfVersionUnchanged = goredis.NewScript(`
local current = redis.call("GET", KEYS[2])
if current == false then
  current = "0"
end
if current ~= ARGV[1] then
  return 0
end
redis.call("SET", KEYS[1], ARGV[2], "PX", ARGV[3])
return 1
`)
