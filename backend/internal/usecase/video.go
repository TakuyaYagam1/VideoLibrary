package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/google/uuid"
	"github.com/wahrwelt-kit/go-cachekit"
	"golang.org/x/sync/singleflight"
)

// VideoListCacheKey is the Redis key used for cached ListVideos responses.
const VideoListCacheKey = "videos:list"

var (
	errVideoRepositoryRequired = errors.New("video repository is required")
	errVideoCacheRequired      = errors.New("video cache is required")
	errVideoListTTLRequired    = errors.New("video list ttl must be greater than 0")
)

type VideoRepository interface {
	ListVideos(ctx context.Context) ([]domain.Video, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Video, error)
	IncrementViews(ctx context.Context, id uuid.UUID) (domain.Video, error)
}

// VideoService coordinates video use cases.
type VideoService struct {
	repository   VideoRepository
	cache        *cachekit.Cache
	videoListTTL time.Duration
	listGroup    singleflight.Group
}

// NewVideoService creates a video use case service.
func NewVideoService(repository VideoRepository, cache *cachekit.Cache, videoListTTL time.Duration) (*VideoService, error) {
	if repository == nil {
		return nil, errVideoRepositoryRequired
	}
	if cache == nil {
		return nil, errVideoCacheRequired
	}
	if videoListTTL <= 0 {
		return nil, errVideoListTTLRequired
	}

	return &VideoService{
		repository:   repository,
		cache:        cache,
		videoListTTL: videoListTTL,
	}, nil
}

func (s *VideoService) ListVideos(ctx context.Context) ([]domain.Video, error) {
	value, err, _ := s.listGroup.Do(VideoListCacheKey, func() (any, error) {
		return cachekit.GetOrLoad(s.cache, ctx, VideoListCacheKey, s.videoListTTL, func(loadCtx context.Context) ([]domain.Video, error) {
			videos, err := s.repository.ListVideos(loadCtx)
			if err != nil {
				return nil, fmt.Errorf("load videos: %w", err)
			}

			return videos, nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}

	videos, ok := value.([]domain.Video)
	if !ok {
		return nil, fmt.Errorf("list videos: unexpected cache value type %T", value)
	}

	return videos, nil
}
