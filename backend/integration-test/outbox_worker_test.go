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
	rediscache "github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo/redis"
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
		if closeErr := redisClient.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
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
		if delErr := cache.Del(cleanupCtx, usecase.VideoListCacheKey); delErr != nil {
			t.Fatalf("cache.Del() cleanup error = %v", delErr)
		}
		if _, execErr := pool.Exec(cleanupCtx, `DELETE FROM outbox_events WHERE payload->>'video_id' = $1`, videoID.String()); execErr != nil {
			t.Fatalf("delete test outbox events error = %v", execErr)
		}
		if _, execErr := pool.Exec(cleanupCtx, `DELETE FROM videos WHERE id = $1`, videoID.String()); execErr != nil {
			t.Fatalf("delete test video error = %v", execErr)
		}
	})

	cachedVideos := []domain.Video{{
		ID:        videoID,
		Title:     "stale cached video",
		FilePath:  "http://localhost:8888/videos/outbox-integration.mp4",
		Views:     41,
		CreatedAt: time.Now().UTC(),
	}}
	if setErr := cache.Set(ctx, usecase.VideoListCacheKey, cachedVideos, time.Minute); setErr != nil {
		t.Fatalf("cache.Set() error = %v", setErr)
	}
	if exists, existsErr := redisClient.Exists(ctx, usecase.VideoListCacheKey).Result(); existsErr != nil {
		t.Fatalf("redis Exists() before worker error = %v", existsErr)
	} else if exists != 1 {
		t.Fatalf("redis cache key exists before worker = %d, want 1", exists)
	}

	videoRepository := repopostgres.NewVideoRepository(pool)
	outboxRepository := repopostgres.NewOutboxRepository(pool)
	incremented, err := videoRepository.IncrementViews(ctx, videoID)
	if err != nil {
		t.Fatalf("IncrementViews() error = %v", err)
	}
	if incremented.Views != 42 {
		t.Fatalf("IncrementViews().Views = %d, want 42", incremented.Views)
	}

	var eventID uuid.UUID
	var eventType string
	var eventPayload []byte
	if queryErr := pool.QueryRow(ctx, `
SELECT id, event_type, payload
FROM outbox_events
WHERE payload->>'video_id' = $1
`, videoID.String()).Scan(&eventID, &eventType, &eventPayload); queryErr != nil {
		t.Fatalf("query outbox event error = %v", queryErr)
	}
	if eventType != "cache.invalidate_videos_list" {
		t.Fatalf("event type = %q, want cache.invalidate_videos_list", eventType)
	}
	var payload struct {
		VideoID uuid.UUID `json:"video_id"`
		Views   int64     `json:"views"`
	}
	if unmarshalErr := json.Unmarshal(eventPayload, &payload); unmarshalErr != nil {
		t.Fatalf("unmarshal outbox payload error = %v", unmarshalErr)
	}
	if payload.VideoID != videoID {
		t.Fatalf("payload video_id = %s, want %s", payload.VideoID, videoID)
	}
	if payload.Views != 42 {
		t.Fatalf("payload views = %d, want 42", payload.Views)
	}

	worker, err := appinternal.NewOutboxWorker(outboxRepository, rediscache.NewCache(redisClient), logkit.Noop())
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

	if exists, existsErr := redisClient.Exists(ctx, usecase.VideoListCacheKey).Result(); existsErr != nil {
		t.Fatalf("redis Exists() after worker error = %v", existsErr)
	} else if exists != 0 {
		t.Fatalf("redis cache key exists after worker = %d, want 0", exists)
	}
	events, err := outboxRepository.FetchUnprocessed(ctx, 10)
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
`, eventID.String()).Scan(&processedAtSet); err != nil {
		t.Fatalf("query processed_at error = %v", err)
	}
	if !processedAtSet {
		t.Fatal("processed_at is not set after worker")
	}
}
