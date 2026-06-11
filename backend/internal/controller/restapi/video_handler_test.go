package restapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/controller/restapi/errmap"
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
		ErrorHandlerFunc: errmap.WriteOpenAPI,
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/videos/not-a-uuid/view", nil)

	handler.ServeHTTP(recorder, request)

	assertErrorResponse(t, recorder, http.StatusBadRequest, errmap.CodeInvalidRequest)
}

func TestHandlerIncrementVideoViewsNotFound(t *testing.T) {
	handler := openapi.Handler(NewHandler(&fakeVideoUsecase{
		incrementErr: domain.ErrVideoNotFound,
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/videos/01978a7a-8a40-7a0d-9b2f-6f0c1e577777/view", nil)

	handler.ServeHTTP(recorder, request)

	assertErrorResponse(t, recorder, http.StatusNotFound, errmap.CodeVideoNotFound)
}

func TestHandlerListVideosInternalError(t *testing.T) {
	handler := openapi.Handler(NewHandler(&fakeVideoUsecase{
		listErr: errors.New("storage unavailable"),
	}))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/videos", nil)

	handler.ServeHTTP(recorder, request)

	assertErrorResponse(t, recorder, http.StatusInternalServerError, errmap.CodeInternal)
}
