package usecase

import (
	"context"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/google/uuid"
)

type VideoRepository interface {
	ListVideos(ctx context.Context) ([]domain.Video, error)
	GetVideo(ctx context.Context, id uuid.UUID) (domain.Video, error)
	IncrementViews(ctx context.Context, id uuid.UUID) (domain.Video, error)
}
