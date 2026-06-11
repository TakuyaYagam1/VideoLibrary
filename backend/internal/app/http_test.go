package app

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/controller/httperr"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/controller/restapi"
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
		restapi.NewHealthCheckers(
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

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	var got openapi.ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&got); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got.Code != httperr.CodeInvalidRequest {
		t.Fatalf("error code = %q, want %q", got.Code, httperr.CodeInvalidRequest)
	}
}

func TestNewHTTPServerAppliesConfiguredTimeouts(t *testing.T) {
	cfg := config.HTTP{
		Addr:              "127.0.0.1:0",
		ReadHeaderTimeout: 1500 * time.Millisecond,
		WriteTimeout:      3 * time.Second,
		ShutdownTimeout:   2 * time.Second,
	}

	server := NewHTTPServer(cfg, http.NotFoundHandler(), logkit.Noop())

	if server.server.ReadHeaderTimeout != cfg.ReadHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout = %s, want %s", server.server.ReadHeaderTimeout, cfg.ReadHeaderTimeout)
	}
	if server.server.WriteTimeout != cfg.WriteTimeout {
		t.Fatalf("WriteTimeout = %s, want %s", server.server.WriteTimeout, cfg.WriteTimeout)
	}
	if server.shutdownTimeout != cfg.ShutdownTimeout {
		t.Fatalf("shutdownTimeout = %s, want %s", server.shutdownTimeout, cfg.ShutdownTimeout)
	}
}

func TestHTTPServerServeShutsDownOnContextCancel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}

	cfg := config.HTTP{
		Addr:              listener.Addr().String(),
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      time.Second,
		ShutdownTimeout:   time.Second,
	}
	server := NewHTTPServer(cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), logkit.Noop())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- server.serve(ctx, listener)
	}()

	assertHTTPStatus(t, "http://"+listener.Addr().String(), http.StatusNoContent)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server did not shut down")
	}
}

type fakeVideoUsecase struct {
	videos []domain.Video
}

func (u *fakeVideoUsecase) ListVideos(context.Context) ([]domain.Video, error) {
	return u.videos, nil
}

func (u *fakeVideoUsecase) IncrementViews(_ context.Context, id uuid.UUID) (domain.Video, error) {
	return domain.Video{ID: id, Views: 1}, nil
}

func assertHTTPStatus(t *testing.T, url string, status int) {
	t.Helper()

	client := http.Client{Timeout: time.Second}
	var lastErr error
	for range 50 {
		resp, err := client.Get(url)
		if err == nil {
			defer func() {
				_ = resp.Body.Close()
			}()
			if resp.StatusCode != status {
				t.Fatalf("status = %d, want %d", resp.StatusCode, status)
			}

			return
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("GET %s did not succeed: %v", url, lastErr)
}
