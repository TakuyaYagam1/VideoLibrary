package restapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/controller/restapi/errmap"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/openapi"
	"github.com/google/uuid"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

func TestNewRouterMountsAPIAndHealthzWithRequestID(t *testing.T) {
	video := domain.Video{
		ID:        uuid.MustParse("01978a7a-8a40-7a0d-9b2f-6f0c1e511111"),
		Title:     "test video",
		FilePath:  "http://localhost:8888/videos/test.mp4",
		Views:     3,
		CreatedAt: time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC),
	}
	router := NewRouter(
		&fakeVideoUsecase{videos: []domain.Video{video}},
		NewHealthCheckers(
			func(context.Context) error { return nil },
			func(context.Context) error { return nil },
		),
		time.Second,
		logkit.Noop(),
	)

	healthRecorder := httptest.NewRecorder()
	router.ServeHTTP(healthRecorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if healthRecorder.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d; body=%s", healthRecorder.Code, http.StatusOK, healthRecorder.Body.String())
	}
	if healthRecorder.Header().Get("X-Request-ID") == "" {
		t.Fatal("health response missing X-Request-ID")
	}

	videosRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/videos", nil)
	request.Header.Set("X-Request-ID", "test-request-id")
	router.ServeHTTP(videosRecorder, request)

	if videosRecorder.Code != http.StatusOK {
		t.Fatalf("videos status = %d, want %d; body=%s", videosRecorder.Code, http.StatusOK, videosRecorder.Body.String())
	}
	if videosRecorder.Header().Get("X-Request-ID") != "test-request-id" {
		t.Fatalf("X-Request-ID = %q, want propagated", videosRecorder.Header().Get("X-Request-ID"))
	}
	var got []openapi.Video
	if err := json.NewDecoder(videosRecorder.Body).Decode(&got); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(got) != 1 || got[0].Id != video.ID {
		t.Fatalf("videos response = %#v, want one video %s", got, video.ID)
	}
}

func TestNewRouterUsesOpenAPIParamErrorHandler(t *testing.T) {
	router := NewRouter(&fakeVideoUsecase{}, nil, time.Second, logkit.Noop())
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/videos/not-a-uuid/view", nil))

	assertErrorResponse(t, recorder, http.StatusBadRequest, errmap.CodeInvalidRequest)
}
