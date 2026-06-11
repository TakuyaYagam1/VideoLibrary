//go:build integration

package integration_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/usecase"
	redisconnector "github.com/TakuyaYagam1/VideoLibrary/backend/pkg/redis"
	"github.com/google/uuid"
	"github.com/wahrwelt-kit/go-cachekit"
)

func TestVideoServiceListVideosReloadsAfterTTL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, cache, err := newIntegrationRedisCache(t, ctx)
	if err != nil {
		t.Fatalf("newIntegrationRedisCache() error = %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	if err := cache.Del(ctx, usecase.VideoListCacheKey); err != nil {
		t.Fatalf("cache.Del() before test error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if err := cache.Del(cleanupCtx, usecase.VideoListCacheKey); err != nil {
			t.Fatalf("cache.Del() cleanup error = %v", err)
		}
	})

	repository := &fakeVideoRepository{
		videos: []domain.Video{{
			ID:        uuid.MustParse("01978a7a-8a40-7a0d-9b2f-6f0c1e5f2222"),
			Title:     "ttl-first",
			FilePath:  "http://localhost:8888/videos/ttl.mp4",
			Views:     1,
			CreatedAt: time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC),
		}},
	}
	service, err := usecase.NewVideoService(repository, cache, 100*time.Millisecond)
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
	if first[0].Title != "ttl-first" || second[0].Title != "ttl-first" {
		t.Fatalf("unexpected cached titles: first=%q second=%q", first[0].Title, second[0].Title)
	}
	if got := repository.listCalls.Load(); got != 1 {
		t.Fatalf("repository ListVideos calls before ttl = %d, want 1", got)
	}

	repository.videos = []domain.Video{{
		ID:        uuid.MustParse("01978a7a-8a40-7a0d-9b2f-6f0c1e5f3333"),
		Title:     "ttl-second",
		FilePath:  "http://localhost:8888/videos/ttl.mp4",
		Views:     2,
		CreatedAt: time.Date(2026, 6, 11, 9, 1, 0, 0, time.UTC),
	}}
	time.Sleep(150 * time.Millisecond)

	third, err := service.ListVideos(ctx)
	if err != nil {
		t.Fatalf("ListVideos() third error = %v", err)
	}
	if third[0].Title != "ttl-second" {
		t.Fatalf("third ListVideos title = %q, want ttl-second", third[0].Title)
	}
	if got := repository.listCalls.Load(); got != 2 {
		t.Fatalf("repository ListVideos calls after ttl = %d, want 2", got)
	}
}

type fakeVideoRepository struct {
	videos    []domain.Video
	listCalls atomic.Int64
}

func (r *fakeVideoRepository) ListVideos(context.Context) ([]domain.Video, error) {
	r.listCalls.Add(1)

	return r.videos, nil
}

func (r *fakeVideoRepository) GetByID(context.Context, uuid.UUID) (domain.Video, error) {
	return domain.Video{}, nil
}

func (r *fakeVideoRepository) IncrementViewsWithOutbox(context.Context, uuid.UUID) (domain.Video, error) {
	return domain.Video{}, nil
}

func newIntegrationRedisCache(t *testing.T, ctx context.Context) (interface{ Close() error }, *cachekit.Cache, error) {
	t.Helper()

	client, cache, err := redisconnector.NewCache(ctx, startRedisContainer(t, ctx))
	if err != nil {
		return nil, nil, err
	}

	return client, cache, nil
}
