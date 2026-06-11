package restapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/openapi"
	"github.com/google/uuid"
)

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
