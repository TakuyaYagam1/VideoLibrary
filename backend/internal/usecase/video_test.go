package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/go-redis/redismock/v9"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/wahrwelt-kit/go-cachekit"
)

func TestVideoServiceListVideosUsesCacheAside(t *testing.T) {
	ctx := context.Background()
	ttl := time.Minute
	client, mock := redismock.NewClientMock()
	cache := cachekit.New(client)
	repository := &fakeVideoRepository{
		videos: []domain.Video{testVideo("cache-aside", 1)},
	}

	cachedBytes, err := json.Marshal(repository.videos)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	mock.ExpectGet(VideoListCacheKey).SetErr(redis.Nil)
	mock.ExpectSet(VideoListCacheKey, cachedBytes, ttl).SetVal("OK")
	mock.ExpectGet(VideoListCacheKey).SetVal(string(cachedBytes))

	service, err := NewVideoService(repository, cache, ttl)
	if err != nil {
		t.Fatalf("NewVideoService() error = %v", err)
	}

	first, err := service.ListVideos(ctx)
	if err != nil {
		t.Fatalf("ListVideos() first error = %v", err)
	}
	second, err := service.ListVideos(ctx)
	if err != nil {
		t.Fatalf("ListVideos() second error = %v", err)
	}

	if len(first) != 1 || first[0].Title != "cache-aside" {
		t.Fatalf("first ListVideos() = %#v", first)
	}
	if len(second) != 1 || second[0].Title != "cache-aside" {
		t.Fatalf("second ListVideos() = %#v", second)
	}
	if got := repository.listCalls.Load(); got != 1 {
		t.Fatalf("repository ListVideos calls = %d, want 1", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("redis expectations error = %v", err)
	}
}

func TestVideoServiceListVideosSingleflightMiss(t *testing.T) {
	ctx := context.Background()
	ttl := time.Minute
	client, mock := redismock.NewClientMock()
	cache := cachekit.New(client)
	videos := []domain.Video{testVideo("singleflight", 3)}
	cachedBytes, err := json.Marshal(videos)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	mock.ExpectGet(VideoListCacheKey).SetErr(redis.Nil)
	mock.ExpectSet(VideoListCacheKey, cachedBytes, ttl).SetVal("OK")

	releaseLoad := make(chan struct{})
	repository := &fakeVideoRepository{
		videos:      videos,
		releaseLoad: releaseLoad,
	}
	service, err := NewVideoService(repository, cache, ttl)
	if err != nil {
		t.Fatalf("NewVideoService() error = %v", err)
	}

	const goroutines = 8
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	start := make(chan struct{})
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := service.ListVideos(ctx)
			errs <- err
		}()
	}

	close(start)
	waitForRepositoryCall(t, repository, 1)
	close(releaseLoad)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("ListVideos() error = %v", err)
		}
	}
	if got := repository.listCalls.Load(); got != 1 {
		t.Fatalf("repository ListVideos calls = %d, want 1", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("redis expectations error = %v", err)
	}
}

func TestVideoServiceIncrementViewsUsesOutboxRepository(t *testing.T) {
	ctx := context.Background()
	client, mock := redismock.NewClientMock()
	cache := cachekit.New(client)
	videoID := uuid.MustParse("01978a7a-8a40-7a0d-9b2f-6f0c1e544444")
	want := testVideo("increment", 8)
	want.ID = videoID
	repository := &fakeVideoRepository{
		incrementedVideo: want,
	}
	service, err := NewVideoService(repository, cache, time.Minute)
	if err != nil {
		t.Fatalf("NewVideoService() error = %v", err)
	}

	got, err := service.IncrementViews(ctx, videoID)
	if err != nil {
		t.Fatalf("IncrementViews() error = %v", err)
	}

	if got != want {
		t.Fatalf("IncrementViews() = %#v, want %#v", got, want)
	}
	if repository.incrementID != videoID {
		t.Fatalf("repository increment id = %s, want %s", repository.incrementID, videoID)
	}
	if got := repository.incrementCalls.Load(); got != 1 {
		t.Fatalf("repository IncrementViewsWithOutbox calls = %d, want 1", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("redis expectations error = %v", err)
	}
}

func TestVideoServiceIncrementViewsPreservesNotFound(t *testing.T) {
	ctx := context.Background()
	client, mock := redismock.NewClientMock()
	cache := cachekit.New(client)
	videoID := uuid.MustParse("01978a7a-8a40-7a0d-9b2f-6f0c1e555555")
	repository := &fakeVideoRepository{
		incrementError: domain.ErrVideoNotFound,
	}
	service, err := NewVideoService(repository, cache, time.Minute)
	if err != nil {
		t.Fatalf("NewVideoService() error = %v", err)
	}

	_, err = service.IncrementViews(ctx, videoID)
	if !errors.Is(err, domain.ErrVideoNotFound) {
		t.Fatalf("IncrementViews() error = %v, want ErrVideoNotFound", err)
	}
	if got := repository.incrementCalls.Load(); got != 1 {
		t.Fatalf("repository IncrementViewsWithOutbox calls = %d, want 1", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("redis expectations error = %v", err)
	}
}

type fakeVideoRepository struct {
	videos           []domain.Video
	incrementedVideo domain.Video
	incrementID      uuid.UUID
	incrementError   error
	releaseLoad      <-chan struct{}
	listCalls        atomic.Int64
	incrementCalls   atomic.Int64
}

func (r *fakeVideoRepository) ListVideos(context.Context) ([]domain.Video, error) {
	r.listCalls.Add(1)
	if r.releaseLoad != nil {
		<-r.releaseLoad
	}

	return r.videos, nil
}

func (r *fakeVideoRepository) GetByID(context.Context, uuid.UUID) (domain.Video, error) {
	return domain.Video{}, nil
}

func (r *fakeVideoRepository) IncrementViewsWithOutbox(_ context.Context, id uuid.UUID) (domain.Video, error) {
	r.incrementCalls.Add(1)
	r.incrementID = id
	if r.incrementError != nil {
		return domain.Video{}, r.incrementError
	}

	return r.incrementedVideo, nil
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

func waitForRepositoryCall(t *testing.T, repository *fakeVideoRepository, want int64) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("repository ListVideos calls = %d, want %d", repository.listCalls.Load(), want)
		case <-ticker.C:
			if repository.listCalls.Load() == want {
				return
			}
		}
	}
}
