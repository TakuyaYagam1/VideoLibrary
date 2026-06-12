package repo

import (
	"context"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/google/uuid"
)

type VideoRepository interface {
	ListVideos(ctx context.Context) ([]domain.Video, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Video, error)
	IncrementViews(ctx context.Context, id uuid.UUID) (domain.Video, error)
}

type Cache interface {
	GetOrLoadVideos(
		ctx context.Context,
		key string,
		ttl time.Duration,
		loadFn func(context.Context) ([]domain.Video, error),
	) ([]domain.Video, error)
	Del(ctx context.Context, keys ...string) error
}

type OutboxEvent struct {
	ID        uuid.UUID
	EventType string
	Payload   []byte
	Attempts  int32
}

type OutboxRepository interface {
	FetchUnprocessed(ctx context.Context, limit int32) ([]OutboxEvent, error)
	MarkProcessed(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, reason string) error
	Release(ctx context.Context, id uuid.UUID, reason string, retryAt time.Time) error
}

type CacheInvalidator interface {
	Del(ctx context.Context, keys ...string) error
}
