package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo"
	"github.com/google/uuid"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

func TestOutboxWorkerRetriesFailedCacheInvalidationWithBackoff(t *testing.T) {
	eventID := uuid.MustParse("01978a7a-8a40-7a0d-9b2f-6f0c1e588888")
	repository := &fakeOutboxRepository{
		events: []repo.OutboxEvent{{
			ID:        eventID,
			EventType: outboxEventInvalidateVideosList,
			Attempts:  2,
		}},
	}
	worker := newTestOutboxWorker(t, repository, &failingCacheInvalidator{})

	processed, err := worker.ProcessBatch(context.Background())
	if err != nil {
		t.Fatalf("ProcessBatch() error = %v", err)
	}
	if processed != 0 {
		t.Fatalf("ProcessBatch() processed = %d, want 0", processed)
	}
	if repository.releasedID != eventID {
		t.Fatalf("released id = %s, want %s", repository.releasedID, eventID)
	}
	if repository.failedID != uuid.Nil {
		t.Fatalf("failed id = %s, want nil", repository.failedID)
	}
	if time.Until(repository.retryAt) <= 0 {
		t.Fatalf("retryAt = %s, want future time", repository.retryAt)
	}
}

func TestOutboxWorkerMarksEventFailedAfterRetryLimit(t *testing.T) {
	eventID := uuid.MustParse("01978a7a-8a40-7a0d-9b2f-6f0c1e599999")
	repository := &fakeOutboxRepository{
		events: []repo.OutboxEvent{{
			ID:        eventID,
			EventType: outboxEventInvalidateVideosList,
			Attempts:  outboxMaxAttempts,
		}},
	}
	worker := newTestOutboxWorker(t, repository, &failingCacheInvalidator{})

	processed, err := worker.ProcessBatch(context.Background())
	if err != nil {
		t.Fatalf("ProcessBatch() error = %v", err)
	}
	if processed != 0 {
		t.Fatalf("ProcessBatch() processed = %d, want 0", processed)
	}
	if repository.failedID != eventID {
		t.Fatalf("failed id = %s, want %s", repository.failedID, eventID)
	}
	if repository.releasedID != uuid.Nil {
		t.Fatalf("released id = %s, want nil", repository.releasedID)
	}
}

func TestOutboxRetryBackoffIsCapped(t *testing.T) {
	if got := outboxRetryBackoff(1); got != time.Second {
		t.Fatalf("outboxRetryBackoff(1) = %s, want 1s", got)
	}
	if got := outboxRetryBackoff(99); got != outboxMaxRetryBackoff {
		t.Fatalf("outboxRetryBackoff(99) = %s, want %s", got, outboxMaxRetryBackoff)
	}
}

func newTestOutboxWorker(t *testing.T, repository repo.OutboxRepository, cache repo.CacheInvalidator) *OutboxWorker {
	t.Helper()

	worker, err := NewOutboxWorker(repository, cache, logkit.Noop())
	if err != nil {
		t.Fatalf("NewOutboxWorker() error = %v", err)
	}

	return worker
}

type fakeOutboxRepository struct {
	events     []repo.OutboxEvent
	releasedID uuid.UUID
	failedID   uuid.UUID
	retryAt    time.Time
}

func (r *fakeOutboxRepository) FetchUnprocessed(context.Context, int32) ([]repo.OutboxEvent, error) {
	return r.events, nil
}

func (r *fakeOutboxRepository) MarkProcessed(context.Context, uuid.UUID) error {
	return nil
}

func (r *fakeOutboxRepository) MarkFailed(_ context.Context, id uuid.UUID, _ string) error {
	r.failedID = id
	return nil
}

func (r *fakeOutboxRepository) Release(_ context.Context, id uuid.UUID, _ string, retryAt time.Time) error {
	r.releasedID = id
	r.retryAt = retryAt
	return nil
}

type failingCacheInvalidator struct{}

func (failingCacheInvalidator) Del(context.Context, ...string) error {
	return errors.New("redis unavailable")
}
