package v1

import (
	"errors"
	"net/http"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/openapi"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/usecase"
	"github.com/google/uuid"
	"github.com/wahrwelt-kit/go-httpkit/httperr"
	httpkit "github.com/wahrwelt-kit/go-httpkit/httputil"
)

type HandlerOption func(*Handler)

type Handler struct {
	openapi.Unimplemented

	video  usecase.VideoUsecase
	health http.HandlerFunc
}

var _ openapi.ServerInterface = (*Handler)(nil)

func NewHandler(video usecase.VideoUsecase, opts ...HandlerOption) *Handler {
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
		httpkit.HandleError(w, r, mapVideoError(err))
		return
	}

	httpkit.RenderOK(w, r, mapVideos(videos))
}

func (h *Handler) GetVideo(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	video, err := h.video.GetVideo(r.Context(), id)
	if err != nil {
		httpkit.HandleError(w, r, mapVideoError(err))
		return
	}

	httpkit.RenderOK(w, r, mapVideo(video))
}

func (h *Handler) IncrementVideoViews(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	video, err := h.video.IncrementViews(r.Context(), id)
	if err != nil {
		httpkit.HandleError(w, r, mapVideoError(err))
		return
	}

	httpkit.RenderOK(w, r, openapi.IncrementViewsResponse{
		Id:    video.ID,
		Views: video.Views,
	})
}

func (h *Handler) GetHealthz(w http.ResponseWriter, r *http.Request) {
	h.health(w, r)
}

func (h *Handler) GetLivez(w http.ResponseWriter, r *http.Request) {
	httpkit.RenderOK(w, r, openapi.HealthResponse{Status: openapi.Ok})
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

func mapVideoError(err error) error {
	if errors.Is(err, domain.ErrVideoNotFound) {
		return httperr.New(err, http.StatusNotFound, httperr.CodeNotFound)
	}

	return err
}
