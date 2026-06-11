package restapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/controller/httperr"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/openapi"
	"github.com/google/uuid"
	httpkit "github.com/wahrwelt-kit/go-httpkit/httputil"
)

func TestHandlerListVideos(t *testing.T) {
	video := testVideo("list", 4)
	handler := openapi.Handler(NewHandler(&fakeVideoUsecase{
		videos: []domain.Video{video},
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/videos", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var got []openapi.Video
	if err := json.NewDecoder(recorder.Body).Decode(&got); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if len(got) != 1 || got[0].Id != video.ID || got[0].Title != video.Title || got[0].Views != video.Views {
		t.Fatalf("response = %#v, want video %#v", got, video)
	}
}

func TestHandlerIncrementVideoViews(t *testing.T) {
	videoID := uuid.MustParse("01978a7a-8a40-7a0d-9b2f-6f0c1e566666")
	video := testVideo("increment", 9)
	video.ID = videoID
	usecase := &fakeVideoUsecase{incrementedVideo: video}
	handler := openapi.Handler(NewHandler(usecase))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/videos/"+videoID.String()+"/view", nil)

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if usecase.incrementID != videoID {
		t.Fatalf("increment id = %s, want %s", usecase.incrementID, videoID)
	}
	var got openapi.IncrementViewsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&got); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got.Id != videoID || got.Views != 9 {
		t.Fatalf("response = %#v, want id=%s views=9", got, videoID)
	}
}

func TestHandlerIncrementVideoViewsInvalidUUID(t *testing.T) {
	handler := openapi.HandlerWithOptions(NewHandler(&fakeVideoUsecase{}), openapi.ChiServerOptions{
		ErrorHandlerFunc: httperr.WriteOpenAPI,
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/videos/not-a-uuid/view", nil)

	handler.ServeHTTP(recorder, request)

	assertErrorResponse(t, recorder, http.StatusBadRequest, httperr.CodeInvalidRequest)
}

func TestHandlerIncrementVideoViewsNotFound(t *testing.T) {
	handler := openapi.Handler(NewHandler(&fakeVideoUsecase{
		incrementErr: domain.ErrVideoNotFound,
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/videos/01978a7a-8a40-7a0d-9b2f-6f0c1e577777/view", nil)

	handler.ServeHTTP(recorder, request)

	assertErrorResponse(t, recorder, http.StatusNotFound, httperr.CodeVideoNotFound)
}

func TestHandlerListVideosInternalError(t *testing.T) {
	handler := openapi.Handler(NewHandler(&fakeVideoUsecase{
		listErr: errors.New("storage unavailable"),
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/videos", nil)

	handler.ServeHTTP(recorder, request)

	assertErrorResponse(t, recorder, http.StatusInternalServerError, httperr.CodeInternal)
}

func TestHandlerGetHealthzOK(t *testing.T) {
	handler := openapi.Handler(NewHandler(&fakeVideoUsecase{}, WithHealthCheckers(NewHealthCheckers(
		func(context.Context) error { return nil },
		func(context.Context) error { return nil },
	), time.Second)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	handler.ServeHTTP(recorder, request)

	assertHealthResponse(t, recorder, http.StatusOK, openapi.Ok)
}

func TestHandlerGetHealthzDegraded(t *testing.T) {
	handler := openapi.Handler(NewHandler(&fakeVideoUsecase{}, WithHealthCheckers(NewHealthCheckers(
		func(context.Context) error { return nil },
		func(context.Context) error { return errors.New("redis unavailable") },
	), time.Second)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	handler.ServeHTTP(recorder, request)

	assertHealthResponse(t, recorder, http.StatusServiceUnavailable, openapi.Degraded)
}

func TestHandlerGetHealthzUsesTimeout(t *testing.T) {
	const timeout = 25 * time.Millisecond
	var deadline time.Time
	handler := openapi.Handler(NewHandler(&fakeVideoUsecase{}, WithHealthCheckers(map[string]httpkit.Checker{
		"db": checkerFunc(func(ctx context.Context) error {
			var ok bool
			deadline, ok = ctx.Deadline()
			if !ok {
				return errors.New("missing deadline")
			}

			return nil
		}),
	}, timeout)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	startedAt := time.Now()

	handler.ServeHTTP(recorder, request)

	assertHealthResponse(t, recorder, http.StatusOK, openapi.Ok)
	if deadline.Before(startedAt) || deadline.After(startedAt.Add(timeout+25*time.Millisecond)) {
		t.Fatalf("deadline = %s, want within configured timeout from %s", deadline, startedAt)
	}
}

func TestHandlerGetHealthzRunsChecksInParallel(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	handler := openapi.Handler(NewHandler(&fakeVideoUsecase{}, WithHealthCheckers(map[string]httpkit.Checker{
		"db": checkerFunc(func(context.Context) error {
			started <- "db"
			<-release
			return nil
		}),
		"redis": checkerFunc(func(context.Context) error {
			started <- "redis"
			<-release
			return nil
		}),
	}, time.Second)))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	done := make(chan struct{})

	go func() {
		defer close(done)
		handler.ServeHTTP(recorder, request)
	}()

	waitForStartedChecks(t, started, 2)
	close(release)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("health handler did not finish")
	}

	assertHealthResponse(t, recorder, http.StatusOK, openapi.Ok)
}

type fakeVideoUsecase struct {
	videos           []domain.Video
	incrementedVideo domain.Video
	incrementID      uuid.UUID
	listErr          error
	incrementErr     error
}

func (u *fakeVideoUsecase) ListVideos(context.Context) ([]domain.Video, error) {
	if u.listErr != nil {
		return nil, u.listErr
	}

	return u.videos, nil
}

func (u *fakeVideoUsecase) IncrementViews(_ context.Context, id uuid.UUID) (domain.Video, error) {
	u.incrementID = id
	if u.incrementErr != nil {
		return domain.Video{}, u.incrementErr
	}

	return u.incrementedVideo, nil
}

func testVideo(title string, views int64) domain.Video {
	return domain.Video{
		ID:        uuid.MustParse("01978a7a-8a40-7a0d-9b2f-6f0c1e511111"),
		Title:     title,
		FilePath:  "http://localhost:8888/videos/test.mp4",
		Views:     views,
		CreatedAt: time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC),
	}
}

func assertErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()

	if recorder.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, status, recorder.Body.String())
	}
	var got openapi.ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&got); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got.Code != code {
		t.Fatalf("error code = %q, want %q", got.Code, code)
	}
	if got.Message == "" {
		t.Fatal("error message is empty")
	}
}

func assertHealthResponse(t *testing.T, recorder *httptest.ResponseRecorder, status int, want openapi.HealthResponseStatus) {
	t.Helper()

	if recorder.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, status, recorder.Body.String())
	}
	body := recorder.Body.Bytes()
	var got openapi.HealthResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got.Status != want {
		t.Fatalf("health status = %q, want %q", got.Status, want)
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, ok := raw["checks"]; ok {
		t.Fatal("health response includes check details")
	}
}

type checkerFunc func(context.Context) error

func (f checkerFunc) Check(ctx context.Context) error {
	return f(ctx)
}

func waitForStartedChecks(t *testing.T, started <-chan string, want int) {
	t.Helper()

	seen := make(map[string]struct{}, want)
	for len(seen) < want {
		select {
		case name := <-started:
			seen[name] = struct{}{}
		case <-time.After(time.Second):
			t.Fatalf("started checks = %d, want %d", len(seen), want)
		}
	}
}
