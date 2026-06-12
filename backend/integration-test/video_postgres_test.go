//go:build integration

package integration_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	repopostgres "github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo/persistent"
	pgconnector "github.com/TakuyaYagam1/VideoLibrary/backend/pkg/postgres"
	"github.com/google/uuid"
)

func TestVideoRepositoryIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg := startPostgresContainer(t, ctx)
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
		if _, execErr := pool.Exec(cleanupCtx, `DELETE FROM outbox_events WHERE payload->>'video_id' = $1`, videoID.String()); execErr != nil {
			t.Fatalf("delete test outbox events error = %v", execErr)
		}
		if _, execErr := pool.Exec(cleanupCtx, `DELETE FROM videos WHERE id = $1`, videoID.String()); execErr != nil {
			t.Fatalf("delete test video error = %v", execErr)
		}
	})

	videos, err := repository.ListVideos(ctx)
	if err != nil {
		t.Fatalf("ListVideos() error = %v", err)
	}
	if !slices.ContainsFunc(videos, func(video domain.Video) bool { return video.ID == videoID }) {
		t.Fatalf("ListVideos() does not contain inserted video id %s", videoID)
	}

	seedVideos := map[string]struct {
		title    string
		filePath string
	}{
		"01978a7a-8a40-7a0d-9b2f-6f0c1e5f1001": {
			title:    "Planet 1.5 MB",
			filePath: "/videos/planet_1.5mb.mp4",
		},
		"01978a7a-8a40-7a0d-9b2f-6f0c1e5f1002": {
			title:    "Planet 3 MB",
			filePath: "/videos/planet_3mb.mp4",
		},
		"01978a7a-8a40-7a0d-9b2f-6f0c1e5f1003": {
			title:    "Planet 10 MB",
			filePath: "/videos/planet_10mb.mp4",
		},
		"01978a7a-8a40-7a0d-9b2f-6f0c1e5f1004": {
			title:    "Planet 18 MB",
			filePath: "/videos/planet_18mb.mp4",
		},
	}
	for _, video := range videos {
		want, ok := seedVideos[video.ID.String()]
		if !ok {
			continue
		}
		if video.Title != want.title || video.FilePath != want.filePath {
			t.Fatalf(
				"seed video %s = (%q, %q), want (%q, %q)",
				video.ID,
				video.Title,
				video.FilePath,
				want.title,
				want.filePath,
			)
		}
		delete(seedVideos, video.ID.String())
	}
	if len(seedVideos) != 0 {
		t.Fatalf("missing seed videos: %v", seedVideos)
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
	if _, getErr := repository.GetByID(ctx, missingID); !errors.Is(getErr, domain.ErrVideoNotFound) {
		t.Fatalf("GetByID() error = %v, want ErrVideoNotFound", getErr)
	}

	incremented, err := repository.IncrementViews(ctx, videoID)
	if err != nil {
		t.Fatalf("IncrementViews() error = %v", err)
	}
	if incremented.Views != video.Views+1 {
		t.Fatalf("IncrementViews().Views = %d, want %d", incremented.Views, video.Views+1)
	}
	if _, incrementErr := repository.IncrementViews(ctx, missingID); !errors.Is(incrementErr, domain.ErrVideoNotFound) {
		t.Fatalf("IncrementViews() error = %v, want ErrVideoNotFound", incrementErr)
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
