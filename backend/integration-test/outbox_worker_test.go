//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	appinternal "github.com/TakuyaYagam1/VideoLibrary/backend/internal/app"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	repopostgres "github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo/persistent"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/usecase"
	pgconnector "github.com/TakuyaYagam1/VideoLibrary/backend/pkg/postgres"
	redisconnector "github.com/TakuyaYagam1/VideoLibrary/backend/pkg/redis"
	"github.com/google/uuid"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

func TestOutboxWorkerIntegrationInvalidatesVideoListCache(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pgCfg := startPostgresContainer(t, ctx)
	if err := pgconnector.RunMigrations(ctx, pgCfg); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}
	pool, err := pgconnector.NewPool(ctx, pgCfg)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)

	redisClient, cache, err := redisconnector.NewCache(ctx, startRedisContainer(t, ctx))
	if err != nil {
		t.Fatalf("NewCache() error = %v", err)
	}
	t.Cleanup(func() {
		if err := redisClient.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	videoID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7() error = %v", err)
	}
	_, err = pool.Exec(ctx, `
INSERT INTO videos (id, title, file_path, views, created_at)
VALUES ($1, $2, $3, $4, now())
`, videoID.String(), "Outbox integration video", "http://localhost:8888/videos/outbox-integration.mp4", int32(41))
	if err != nil {
		t.Fatalf("insert test video error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if err := cache.Del(cleanupCtx, usecase.VideoListCacheKey); err != nil {
			t.Fatalf("cache.Del() cleanup error = %v", err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM outbox_events WHERE payload->>'video_id' = $1`, videoID.String()); err != nil {
			t.Fatalf("delete test outbox events error = %v", err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM videos WHERE id = $1`, videoID.String()); err != nil {
			t.Fatalf("delete test video error = %v", err)
		}
	})

	cachedVideos := []domain.Video{{
		ID:        videoID,
		Title:     "stale cached video",
		FilePath:  "http://localhost:8888/videos/outbox-integration.mp4",
		Views:     41,
		CreatedAt: time.Now().UTC(),
	}}
	if err := cache.Set(ctx, usecase.VideoListCacheKey, cachedVideos, time.Minute); err != nil {
		t.Fatalf("cache.Set() error = %v", err)
	}
	if exists, err := redisClient.Exists(ctx, usecase.VideoListCacheKey).Result(); err != nil {
		t.Fatalf("redis Exists() before worker error = %v", err)
	} else if exists != 1 {
		t.Fatalf("redis cache key exists before worker = %d, want 1", exists)
	}

	videoRepository := repopostgres.NewVideoRepository(pool)
	outboxRepository := repopostgres.NewOutboxRepository(pool)
	incremented, err := videoRepository.IncrementViewsWithOutbox(ctx, videoID)
	if err != nil {
		t.Fatalf("IncrementViewsWithOutbox() error = %v", err)
	}
	if incremented.Views != 42 {
		t.Fatalf("IncrementViewsWithOutbox().Views = %d, want 42", incremented.Views)
	}

	events, err := outboxRepository.FetchUnprocessed(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnprocessed() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("FetchUnprocessed() returned %d events, want 1", len(events))
	}
	event := events[0]
	if event.EventType != "cache.invalidate_videos_list" {
		t.Fatalf("event type = %q, want cache.invalidate_videos_list", event.EventType)
	}
	var payload struct {
		VideoID uuid.UUID `json:"video_id"`
		Views   int64     `json:"views"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("unmarshal outbox payload error = %v", err)
	}
	if payload.VideoID != videoID {
		t.Fatalf("payload video_id = %s, want %s", payload.VideoID, videoID)
	}
	if payload.Views != 42 {
		t.Fatalf("payload views = %d, want 42", payload.Views)
	}

	worker, err := appinternal.NewOutboxWorker(outboxRepository, cache, logkit.Noop())
	if err != nil {
		t.Fatalf("NewOutboxWorker() error = %v", err)
	}
	processed, err := worker.ProcessBatch(ctx)
	if err != nil {
		t.Fatalf("ProcessBatch() error = %v", err)
	}
	if processed != 1 {
		t.Fatalf("ProcessBatch() processed = %d, want 1", processed)
	}

	if exists, err := redisClient.Exists(ctx, usecase.VideoListCacheKey).Result(); err != nil {
		t.Fatalf("redis Exists() after worker error = %v", err)
	} else if exists != 0 {
		t.Fatalf("redis cache key exists after worker = %d, want 0", exists)
	}
	events, err = outboxRepository.FetchUnprocessed(ctx, 10)
	if err != nil {
		t.Fatalf("FetchUnprocessed() after worker error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("FetchUnprocessed() after worker returned %d events, want 0", len(events))
	}

	var processedAtSet bool
	if err := pool.QueryRow(ctx, `
SELECT processed_at IS NOT NULL
FROM outbox_events
WHERE id = $1
`, event.ID.String()).Scan(&processedAtSet); err != nil {
		t.Fatalf("query processed_at error = %v", err)
	}
	if !processedAtSet {
		t.Fatal("processed_at is not set after worker")
	}
}
