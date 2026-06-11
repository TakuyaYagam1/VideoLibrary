package restapi

import (
	"context"
	"net/http"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/controller/restapi/errmap"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/controller/restapi/response"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/openapi"
	"github.com/google/uuid"
	httpkit "github.com/wahrwelt-kit/go-httpkit/httputil"
)

type VideoUsecase interface {
	ListVideos(ctx context.Context) ([]domain.Video, error)
	IncrementViews(ctx context.Context, id uuid.UUID) (domain.Video, error)
}

type HandlerOption func(*Handler)

type Handler struct {
	openapi.Unimplemented

	video  VideoUsecase
	health http.HandlerFunc
}

var _ openapi.ServerInterface = (*Handler)(nil)

func NewHandler(video VideoUsecase, opts ...HandlerOption) *Handler {
	handler := &Handler{
		video: video,
	}
	handler.health = httpkit.HealthHandler(nil, httpkit.HealthHideDetails())
	for _, opt := range opts {
		opt(handler)
	}

	return handler
}

func WithHealthCheckers(checkers map[string]httpkit.Checker, timeout time.Duration) HandlerOption {
	return func(h *Handler) {
		h.health = httpkit.HealthHandler(checkers, httpkit.HealthTimeout(timeout), httpkit.HealthHideDetails())
	}
}

func (h *Handler) ListVideos(w http.ResponseWriter, r *http.Request) {
	videos, err := h.video.ListVideos(r.Context())
	if err != nil {
		errmap.WriteUsecase(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, mapVideos(videos))
}

func (h *Handler) IncrementVideoViews(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	video, err := h.video.IncrementViews(r.Context(), id)
	if err != nil {
		errmap.WriteUsecase(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, openapi.IncrementViewsResponse{
		Id:    video.ID,
		Views: video.Views,
	})
}

func (h *Handler) GetHealthz(w http.ResponseWriter, r *http.Request) {
	h.health(w, r)
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
