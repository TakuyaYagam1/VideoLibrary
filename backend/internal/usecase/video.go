package usecase

import (
	"context"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
)

type VideoRepository interface {
	ListVideos(ctx context.Context) ([]domain.Video, error)
	GetVideo(ctx context.Context, id domain.VideoID) (domain.Video, error)
	IncrementViews(ctx context.Context, id domain.VideoID) (domain.Video, error)
}
