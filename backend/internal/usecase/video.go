package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

type VideoUsecase interface {
	ListVideos(ctx context.Context) ([]domain.Video, error)
	GetVideo(ctx context.Context, id uuid.UUID) (domain.Video, error)
	IncrementViews(ctx context.Context, id uuid.UUID) (domain.Video, error)
}

// VideoListCacheKey is the Redis key used for cached ListVideos responses.
const VideoListCacheKey = "videos:list"

const (
	videoListLoadTimeout       = 5 * time.Second
	videoListInvalidateTimeout = 5 * time.Second
)

var (
	errVideoRepositoryRequired = errors.New("video repository is required")
	errVideoCacheRequired      = errors.New("video cache is required")
	errVideoListTTLRequired    = errors.New("video list ttl must be greater than 0")
)

// VideoService coordinates video use cases.
type VideoService struct {
	repository   repo.VideoRepository
	cache        repo.Cache
	videoListTTL time.Duration
	listGroup    singleflight.Group
}

var _ VideoUsecase = (*VideoService)(nil)

// NewVideoService creates a video use case service.
func NewVideoService(repository repo.VideoRepository, cache repo.Cache, videoListTTL time.Duration) (*VideoService, error) {
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	value, err, _ := s.listGroup.Do(VideoListCacheKey, func() (any, error) {
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), videoListLoadTimeout)
		defer cancel()

		return s.cache.GetOrLoadVideos(loadCtx, VideoListCacheKey, s.videoListTTL, func(loadCtx context.Context) ([]domain.Video, error) {
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

func (s *VideoService) GetVideo(ctx context.Context, id uuid.UUID) (domain.Video, error) {
	video, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return domain.Video{}, fmt.Errorf("get video: %w", err)
	}

	return video, nil
}

// IncrementViews records a view, invalidates the list cache synchronously, and keeps the outbox as retry fallback.
func (s *VideoService) IncrementViews(ctx context.Context, id uuid.UUID) (domain.Video, error) {
	video, err := s.repository.IncrementViews(ctx, id)
	if err != nil {
		return domain.Video{}, fmt.Errorf("increment video views: %w", err)
	}

	invalidateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), videoListInvalidateTimeout)
	defer cancel()
	if err := s.cache.Del(invalidateCtx, VideoListCacheKey); err != nil {
		return domain.Video{}, fmt.Errorf("invalidate video list cache: %w", err)
	}

	return video, nil
}
