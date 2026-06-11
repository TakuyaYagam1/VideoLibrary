package restapi

import (
	"context"
	"net/http"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/controller/httperr"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/controller/httputil"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/openapi"
	"github.com/google/uuid"
)

type VideoUsecase interface {
	ListVideos(ctx context.Context) ([]domain.Video, error)
	IncrementViews(ctx context.Context, id uuid.UUID) (domain.Video, error)
}

type Handler struct {
	openapi.Unimplemented

	video VideoUsecase
}

var _ openapi.ServerInterface = (*Handler)(nil)

func NewHandler(video VideoUsecase) *Handler {
	return &Handler{
		video: video,
	}
}

func (h *Handler) ListVideos(w http.ResponseWriter, r *http.Request) {
	videos, err := h.video.ListVideos(r.Context())
	if err != nil {
		httperr.WriteUsecase(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, mapVideos(videos))
}

func (h *Handler) IncrementVideoViews(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	video, err := h.video.IncrementViews(r.Context(), id)
	if err != nil {
		httperr.WriteUsecase(w, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, openapi.IncrementViewsResponse{
		Id:    video.ID,
		Views: video.Views,
	})
}

func mapVideos(videos []domain.Video) []openapi.Video {
	result := make([]openapi.Video, 0, len(videos))
	for _, video := range videos {
		result = append(result, mapVideo(video))
	}

	return result
}

func mapVideo(video domain.Video) openapi.Video {
	return openapi.Video{
		Id:        video.ID,
		Title:     video.Title,
		FilePath:  video.FilePath,
		Views:     video.Views,
		CreatedAt: video.CreatedAt,
	}
}
