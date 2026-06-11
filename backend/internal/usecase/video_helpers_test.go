package usecase

import (
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/google/uuid"
)

func newTestVideoService(t *testing.T, repository VideoRepository, cache VideoCache, ttl time.Duration) *VideoService {
	t.Helper()

	service, err := NewVideoService(repository, cache, ttl)
	if err != nil {
		t.Fatalf("NewVideoService() error = %v", err)
	}

	return service
}

func testVideo(title string, views int64) domain.Video {
	return domain.Video{
		ID:        uuid.MustParse("01978a7a-8a40-7a0d-9b2f-6f0c1e5f1111"),
		Title:     title,
		FilePath:  "http://localhost:8888/videos/test.mp4",
		Views:     views,
		CreatedAt: time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC),
	}
}

func requireVideos(t *testing.T, got, want []domain.Video) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("videos = %#v, want %#v", got, want)
	}
}

func waitForAtomic(t *testing.T, value *atomic.Int64, want int64) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("value = %d, want %d", value.Load(), want)
		case <-ticker.C:
			if value.Load() == want {
				return
			}
		}
	}
}

func waitForChannel(t *testing.T, ch <-chan struct{}, message string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal(message)
	}
}
