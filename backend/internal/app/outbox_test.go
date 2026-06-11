package app

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/config"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo/persistent"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/usecase"
	"github.com/google/uuid"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

func TestOutboxWorkerProcessBatchInvalidatesCacheAndMarksProcessed(t *testing.T) {
	eventID := uuid.MustParse("01978a7a-8a40-7a0d-9b2f-6f0c1e588888")
	repository := &fakeOutboxRepository{
		events: []persistent.OutboxEvent{{
			ID:        eventID,
			EventType: outboxEventInvalidateVideosList,
		}},
	}
	cache := &fakeCacheInvalidator{}
	worker := newTestOutboxWorker(t, repository, cache)

	processed, err := worker.ProcessBatch(context.Background())

	if err != nil {
		t.Fatalf("ProcessBatch() error = %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}
	if !slices.Equal(cache.deletedKeys, []string{usecase.VideoListCacheKey}) {
		t.Fatalf("deleted keys = %#v, want %q", cache.deletedKeys, usecase.VideoListCacheKey)
	}
	if !slices.Equal(repository.marked, []uuid.UUID{eventID}) {
		t.Fatalf("marked events = %#v, want %s", repository.marked, eventID)
	}
}

func TestOutboxWorkerDoesNotMarkWhenCacheInvalidationFails(t *testing.T) {
	eventID := uuid.MustParse("01978a7a-8a40-7a0d-9b2f-6f0c1e599999")
	repository := &fakeOutboxRepository{
		events: []persistent.OutboxEvent{{
			ID:        eventID,
			EventType: outboxEventInvalidateVideosList,
		}},
	}
	cache := &fakeCacheInvalidator{err: errors.New("redis unavailable")}
	worker := newTestOutboxWorker(t, repository, cache)

	processed, err := worker.ProcessBatch(context.Background())

	if err != nil {
		t.Fatalf("ProcessBatch() error = %v", err)
	}
	if processed != 0 {
		t.Fatalf("processed = %d, want 0", processed)
	}
	if len(repository.marked) != 0 {
		t.Fatalf("marked events = %#v, want none", repository.marked)
	}
}

func TestAppRunStartsHTTPAndOutboxWorkerUntilContextCancel(t *testing.T) {
	repository := &fakeOutboxRepository{}
	cache := &fakeCacheInvalidator{}
	worker := newTestOutboxWorker(t, repository, cache)
	worker.pollInterval = 10 * time.Millisecond
	cfg := testConfig()
	cfg.HTTP = config.HTTP{
		Addr:              "127.0.0.1:0",
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      time.Second,
		ShutdownTimeout:   time.Second,
	}
	application := &App{
		config:       cfg,
		httpServer:   NewHTTPServer(cfg.HTTP, http.NotFoundHandler(), logkit.Noop()),
		outboxWorker: worker,
	}
	ctx, cancel := context.WithCancel(logkit.IntoContext(context.Background(), logkit.Noop()))
	done := make(chan error, 1)

	go func() {
		done <- application.Run(ctx)
	}()

	waitForFetchCall(t, repository)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not stop")
	}
}

func newTestOutboxWorker(t *testing.T, repository *fakeOutboxRepository, cache *fakeCacheInvalidator) *OutboxWorker {
	t.Helper()

	worker, err := NewOutboxWorker(repository, cache, logkit.Noop())
	if err != nil {
		t.Fatalf("NewOutboxWorker() error = %v", err)
	}
	worker.operationTimeout = time.Second

	return worker
}

type fakeOutboxRepository struct {
	mu         sync.Mutex
	events     []persistent.OutboxEvent
	marked     []uuid.UUID
	fetchCalls int
	fetchErr   error
	markErr    error
}

func (r *fakeOutboxRepository) FetchUnprocessed(ctx context.Context, _ int32) ([]persistent.OutboxEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fetchCalls++
	if r.fetchErr != nil {
		return nil, r.fetchErr
	}

	return slices.Clone(r.events), nil
}

func (r *fakeOutboxRepository) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.markErr != nil {
		return r.markErr
	}
	r.marked = append(r.marked, id)

	return nil
}

func waitForFetchCall(t *testing.T, repository *fakeOutboxRepository) {
	t.Helper()

	deadline := time.After(time.Second)
	for {
		repository.mu.Lock()
		calls := repository.fetchCalls
		repository.mu.Unlock()
		if calls > 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatal("outbox worker did not fetch")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

type fakeCacheInvalidator struct {
	deletedKeys []string
	err         error
}

func (c *fakeCacheInvalidator) Del(_ context.Context, keys ...string) error {
	c.deletedKeys = append(c.deletedKeys, keys...)
	return c.err
}
