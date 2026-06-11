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
