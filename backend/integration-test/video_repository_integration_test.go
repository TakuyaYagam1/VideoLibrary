//go:build integration

package integration_test

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	repopostgres "github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo/persistent"
	pgconnector "github.com/TakuyaYagam1/VideoLibrary/backend/pkg/postgres"
	"github.com/google/uuid"
)

func TestVideoRepositoryIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://videolibrary:videolibrary@localhost:5432/videolibrary?sslmode=disable"
	}

	cfg := config.PostgreSQL{
		DSN:            dsn,
		MaxConns:       4,
		MinConns:       1,
		RetryTimeout:   time.Second,
		ConnectTimeout: 5 * time.Second,
		MigrationsPath: "../migrations",
	}
	if err := pgconnector.RunMigrations(ctx, cfg); err != nil {
		t.Fatalf("Run migrations error = %v", err)
	}

	pool, err := pgconnector.NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)

	repository := repopostgres.NewVideoRepository(pool)
	videoID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7() error = %v", err)
	}
	_, err = pool.Exec(ctx, `
INSERT INTO videos (id, title, file_path, views, created_at)
VALUES ($1, $2, $3, $4, now())
`, videoID.String(), "Integration video", "http://localhost:8888/videos/integration.mp4", int32(7))
	if err != nil {
		t.Fatalf("insert test video error = %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM outbox_events WHERE payload->>'video_id' = $1`, videoID.String()); err != nil {
			t.Fatalf("delete test outbox events error = %v", err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM videos WHERE id = $1`, videoID.String()); err != nil {
			t.Fatalf("delete test video error = %v", err)
		}
	})

	videos, err := repository.ListVideos(ctx)
	if err != nil {
		t.Fatalf("ListVideos() error = %v", err)
	}
	if !slices.ContainsFunc(videos, func(video domain.Video) bool { return video.ID == videoID }) {
		t.Fatalf("ListVideos() does not contain inserted video id %s", videoID)
	}

	video, err := repository.GetByID(ctx, videoID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if video.Views != 7 {
		t.Fatalf("GetByID().Views = %d, want 7", video.Views)
	}

	missingID, err := uuid.NewV7()
	if err != nil {
		t.Fatalf("uuid.NewV7() error = %v", err)
	}
	if _, err := repository.GetByID(ctx, missingID); !errors.Is(err, domain.ErrVideoNotFound) {
		t.Fatalf("GetByID() error = %v, want ErrVideoNotFound", err)
	}

	incremented, err := repository.IncrementViews(ctx, videoID)
	if err != nil {
		t.Fatalf("IncrementViews() error = %v", err)
	}
	if incremented.Views != video.Views+1 {
		t.Fatalf("IncrementViews().Views = %d, want %d", incremented.Views, video.Views+1)
	}

	var outboxCount int
	err = pool.QueryRow(ctx, `
SELECT count(*)
FROM outbox_events
WHERE event_type = 'cache.invalidate_videos_list'
  AND payload->>'video_id' = $1
`, videoID.String()).Scan(&outboxCount)
	if err != nil {
		t.Fatalf("count outbox events error = %v", err)
	}
	if outboxCount != 1 {
		t.Fatalf("outbox event count = %d, want 1", outboxCount)
	}
}
