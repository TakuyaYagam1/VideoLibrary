//go:build e2e

package e2e_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	appinternal "github.com/TakuyaYagam1/VideoLibrary/backend/internal/app"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/openapi"
	"github.com/google/uuid"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

func TestVideoAPIE2EListIncrementAndHealthz(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg := testConfig(t, ctx)
	application, err := appinternal.New(logkit.IntoContext(ctx, logkit.Noop()), cfg)
	if err != nil {
		t.Fatalf("app.New() error = %v", err)
	}
	t.Cleanup(application.Close)

	runCtx, stopApp := context.WithCancel(logkit.IntoContext(context.Background(), logkit.Noop()))
	t.Cleanup(stopApp)
	done := make(chan error, 1)
	go func() {
		done <- application.Run(runCtx)
	}()
	t.Cleanup(func() {
		stopApp()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("application.Run() error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("application did not stop")
		}
	})

	baseURL := "http://" + cfg.HTTP.Addr
	client, err := openapi.NewClientWithResponses(baseURL, openapi.WithHTTPClient(&http.Client{Timeout: 5 * time.Second}))
	if err != nil {
		t.Fatalf("NewClientWithResponses() error = %v", err)
	}
	waitForHealthz(t, ctx, client)

	health, err := client.GetHealthzWithResponse(ctx)
	if err != nil {
		t.Fatalf("GetHealthzWithResponse() error = %v", err)
	}
	if health.StatusCode() != http.StatusOK || health.JSON200 == nil || health.JSON200.Status != openapi.Ok {
		t.Fatalf("healthz status = %d body=%s", health.StatusCode(), health.Body)
	}

	firstList, err := client.ListVideosWithResponse(ctx)
	if err != nil {
		t.Fatalf("ListVideosWithResponse() first error = %v", err)
	}
	if firstList.StatusCode() != http.StatusOK || firstList.JSON200 == nil {
		t.Fatalf("first list status = %d body=%s", firstList.StatusCode(), firstList.Body)
	}
	if len(*firstList.JSON200) == 0 {
		t.Fatal("first list returned no videos")
	}

	video := (*firstList.JSON200)[0]
	increment, err := client.IncrementVideoViewsWithResponse(ctx, uuid.UUID(video.Id))
	if err != nil {
		t.Fatalf("IncrementVideoViewsWithResponse() error = %v", err)
	}
	if increment.StatusCode() != http.StatusOK || increment.JSON200 == nil {
		t.Fatalf("increment status = %d body=%s", increment.StatusCode(), increment.Body)
	}
	if increment.JSON200.Id != video.Id {
		t.Fatalf("increment id = %s, want %s", increment.JSON200.Id, video.Id)
	}
	if increment.JSON200.Views != video.Views+1 {
		t.Fatalf("increment views = %d, want %d", increment.JSON200.Views, video.Views+1)
	}

	waitForListedViews(t, ctx, client, uuid.UUID(video.Id), video.Views+1)
}

func testConfig(t *testing.T, ctx context.Context) config.Config {
	t.Helper()

	return config.Config{
		App: config.App{
			Name: "videolibrary",
			Env:  "e2e",
		},
		HTTP: config.HTTP{
			Addr:              freeTCPAddr(t),
			ReadHeaderTimeout: 2 * time.Second,
			WriteTimeout:      5 * time.Second,
			ShutdownTimeout:   5 * time.Second,
		},
		PostgreSQL: startPostgresContainer(t, ctx),
		Redis:      startRedisContainer(t, ctx),
		Cache: config.Cache{
			VideoListTTL: time.Minute,
		},
		Health: config.Health{
			CheckTimeout: time.Second,
		},
		SeaweedFS: config.SeaweedFS{
			PublicURL: "http://127.0.0.1:8888",
		},
		Log: config.Log{
			Level:  "error",
			Format: "json",
			Output: "console",
		},
	}
}

func waitForHealthz(t *testing.T, ctx context.Context, client *openapi.ClientWithResponses) {
	t.Helper()

	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("wait for healthz context error: %v", ctx.Err())
		case <-deadline:
			t.Fatalf("healthz did not become healthy: %v", lastErr)
		case <-ticker.C:
			response, err := client.GetHealthzWithResponse(ctx)
			if err == nil && response.StatusCode() == http.StatusOK {
				return
			}
			lastErr = err
		}
	}
}

func waitForListedViews(
	t *testing.T,
	ctx context.Context,
	client *openapi.ClientWithResponses,
	id uuid.UUID,
	wantViews int64,
) {
	t.Helper()

	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("wait for listed views context error: %v", ctx.Err())
		case <-deadline:
			t.Fatalf("video %s did not reach views %d in list", id, wantViews)
		case <-ticker.C:
			response, err := client.ListVideosWithResponse(ctx)
			if err != nil || response.StatusCode() != http.StatusOK || response.JSON200 == nil {
				continue
			}
			for _, video := range *response.JSON200 {
				if uuid.UUID(video.Id) == id && video.Views == wantViews {
					return
				}
			}
		}
	}
}
