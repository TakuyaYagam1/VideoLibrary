package usecase

import (
	"context"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/wahrwelt-kit/go-cachekit"
)

type CacheKitVideoCache struct {
	cache *cachekit.Cache
}

func NewCacheKitVideoCache(cache *cachekit.Cache) VideoCache {
	if cache == nil {
		return nil
	}

	return &CacheKitVideoCache{cache: cache}
}

func (c *CacheKitVideoCache) GetOrLoadVideos(
	ctx context.Context,
	key string,
	ttl time.Duration,
	loadFn func(context.Context) ([]domain.Video, error),
) ([]domain.Video, error) {
	return cachekit.GetOrLoad(c.cache, ctx, key, ttl, loadFn)
}
