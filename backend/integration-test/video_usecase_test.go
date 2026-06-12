//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	rediscache "github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo/redis"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/usecase"
	redisconnector "github.com/TakuyaYagam1/VideoLibrary/backend/pkg/redis"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

func TestVideoServiceListVideosReloadsAfterTTL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, cache, err := newIntegrationRedisCache(t, ctx)
	if err != nil {
		t.Fatalf("newIntegrationRedisCache() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	})
	if delErr := cache.Del(ctx, usecase.VideoListCacheKey); delErr != nil {
		t.Fatalf("cache.Del() before test error = %v", delErr)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if delErr := cache.Del(cleanupCtx, usecase.VideoListCacheKey); delErr != nil {
			t.Fatalf("cache.Del() cleanup error = %v", delErr)
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

func TestVideoServiceListVideosReloadsCorruptCache(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, cache, err := newIntegrationRedisCache(t, ctx)
	if err != nil {
		t.Fatalf("newIntegrationRedisCache() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	})
	if delErr := cache.Del(ctx, usecase.VideoListCacheKey); delErr != nil {
		t.Fatalf("cache.Del() before test error = %v", delErr)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if delErr := cache.Del(cleanupCtx, usecase.VideoListCacheKey); delErr != nil {
			t.Fatalf("cache.Del() cleanup error = %v", delErr)
		}
	})

	if setErr := client.Set(ctx, usecase.VideoListCacheKey, []byte("{not-json"), time.Minute).Err(); setErr != nil {
		t.Fatalf("seed corrupt cache error = %v", setErr)
	}

	repository := &fakeVideoRepository{
		videos: []domain.Video{{
			ID:        uuid.MustParse("01978a7a-8a40-7a0d-9b2f-6f0c1e544444"),
			Title:     "corrupt-cache-reload",
			FilePath:  "http://localhost:8888/videos/corrupt-cache.mp4",
			Views:     3,
			CreatedAt: time.Date(2026, 6, 11, 9, 2, 0, 0, time.UTC),
		}},
	}
	service, err := usecase.NewVideoService(repository, cache, time.Minute)
	if err != nil {
		t.Fatalf("NewVideoService() error = %v", err)
	}

	got, err := service.ListVideos(ctx)
	if err != nil {
		t.Fatalf("ListVideos() error = %v", err)
	}
	if len(got) != 1 || got[0].Title != "corrupt-cache-reload" {
		t.Fatalf("ListVideos() = %#v, want corrupt-cache-reload", got)
	}
	if calls := repository.listCalls.Load(); calls != 1 {
		t.Fatalf("repository ListVideos calls = %d, want 1", calls)
	}

	cached, err := client.Get(ctx, usecase.VideoListCacheKey).Bytes()
	if err != nil {
		t.Fatalf("read repaired cache error = %v", err)
	}
	var repaired []domain.Video
	if err := json.Unmarshal(cached, &repaired); err != nil {
		t.Fatalf("repaired cache is not valid JSON: %v", err)
	}
	if len(repaired) != 1 || repaired[0].Title != "corrupt-cache-reload" {
		t.Fatalf("repaired cache = %#v, want corrupt-cache-reload", repaired)
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

func (r *fakeVideoRepository) IncrementViews(context.Context, uuid.UUID) (domain.Video, error) {
	return domain.Video{}, nil
}

func newIntegrationRedisCache(t *testing.T, ctx context.Context) (*goredis.Client, *rediscache.Cache, error) {
	t.Helper()

	client, _, err := redisconnector.NewCache(ctx, startRedisContainer(t, ctx))
	if err != nil {
		return nil, nil, err
	}

	return client, rediscache.NewCache(client), nil
}
