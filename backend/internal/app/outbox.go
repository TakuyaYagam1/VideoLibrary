package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/usecase"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

const (
	outboxEventInvalidateVideosList = "cache.invalidate_videos_list"
	outboxBatchSize                 = int32(16)
	outboxPollInterval              = time.Second
	outboxOperationTimeout          = 5 * time.Second
	outboxCleanupTimeout            = 5 * time.Second
	outboxMaxAttempts               = int32(5)
	outboxMaxRetryBackoff           = 30 * time.Second
)

var (
	errOutboxRepositoryRequired = errors.New("outbox repository is required")
	errOutboxCacheRequired      = errors.New("outbox cache is required")
)

type OutboxWorker struct {
	repository       repo.OutboxRepository
	cache            repo.CacheInvalidator
	logger           logkit.Logger
	batchSize        int32
	pollInterval     time.Duration
	operationTimeout time.Duration
}

func NewOutboxWorker(repository repo.OutboxRepository, cache repo.CacheInvalidator, logger logkit.Logger) (*OutboxWorker, error) {
	if repository == nil {
		return nil, errOutboxRepositoryRequired
	}
	if cache == nil {
		return nil, errOutboxCacheRequired
	}
	if logger == nil {
		logger = logkit.Noop()
	}

	return &OutboxWorker{
		repository:       repository,
		cache:            cache,
		logger:           logger,
		batchSize:        outboxBatchSize,
		pollInterval:     outboxPollInterval,
		operationTimeout: outboxOperationTimeout,
	}, nil
}

func (w *OutboxWorker) Run(ctx context.Context) error {
	for {
		processed, err := w.ProcessBatch(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			w.logger.ErrorContext(ctx, "process outbox batch", logkit.Component("outbox"), logkit.Error(err))
		}

		if processed == 0 {
			if err := w.wait(ctx); err != nil {
				return nil
			}
		}
	}
}

func (w *OutboxWorker) ProcessBatch(ctx context.Context) (int, error) {
	events, err := w.fetch(ctx)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, event := range events {
		if err := w.processEvent(ctx, event); err != nil {
			w.logger.ErrorContext(ctx, "process outbox event", logkit.Component("outbox"), logkit.Error(err), logkit.Fields{
				"event_id":   event.ID.String(),
				"event_type": event.EventType,
			})
			continue
		}
		processed++
	}

	return processed, nil
}

func (w *OutboxWorker) fetch(ctx context.Context) ([]repo.OutboxEvent, error) {
	operationCtx, cancel := context.WithTimeout(ctx, w.operationTimeout)
	defer cancel()

	events, err := w.repository.FetchUnprocessed(operationCtx, w.batchSize)
	if err != nil {
		return nil, fmt.Errorf("fetch outbox events: %w", err)
	}

	return events, nil
}

func (w *OutboxWorker) processEvent(ctx context.Context, event repo.OutboxEvent) error {
	operationCtx, cancel := context.WithTimeout(ctx, w.operationTimeout)
	defer cancel()

	switch event.EventType {
	case outboxEventInvalidateVideosList:
		if err := w.cache.Del(operationCtx, usecase.VideoListCacheKey); err != nil {
			if releaseErr := w.releaseOrFail(ctx, event, err.Error()); releaseErr != nil {
				return fmt.Errorf("release failed cache invalidation event: %w (original error: %w)", releaseErr, err)
			}

			return fmt.Errorf("invalidate video list cache: %w", err)
		}
	default:
		reason := fmt.Sprintf("unsupported outbox event type %q", event.EventType)
		cleanupCtx, cleanupCancel := context.WithTimeout(ctx, outboxCleanupTimeout)
		defer cleanupCancel()
		if err := w.repository.MarkFailed(cleanupCtx, event.ID, reason); err != nil {
			return fmt.Errorf("mark unsupported outbox event failed: %w", err)
		}

		w.logger.ErrorContext(ctx, reason, logkit.Component("outbox"), logkit.Fields{
			"event_id": event.ID.String(),
		})
		return nil
	}

	if err := w.repository.MarkProcessed(operationCtx, event.ID); err != nil {
		if releaseErr := w.releaseOrFail(ctx, event, err.Error()); releaseErr != nil {
			return fmt.Errorf("release failed processed event: %w (original error: %w)", releaseErr, err)
		}

		return fmt.Errorf("mark event processed: %w", err)
	}

	return nil
}

func (w *OutboxWorker) releaseOrFail(ctx context.Context, event repo.OutboxEvent, reason string) error {
	cleanupCtx, cancel := context.WithTimeout(ctx, outboxCleanupTimeout)
	defer cancel()

	if event.Attempts >= outboxMaxAttempts {
		if err := w.repository.MarkFailed(cleanupCtx, event.ID, reason); err != nil {
			return fmt.Errorf("mark outbox event failed after %d attempts: %w", event.Attempts, err)
		}

		w.logger.ErrorContext(ctx, "outbox event reached retry limit", logkit.Component("outbox"), logkit.Fields{
			"event_id": event.ID.String(),
			"attempts": event.Attempts,
		})

		return nil
	}

	return w.repository.Release(cleanupCtx, event.ID, reason, time.Now().Add(outboxRetryBackoff(event.Attempts)))
}

func outboxRetryBackoff(attempts int32) time.Duration {
	if attempts < 1 {
		attempts = 1
	}

	backoff := time.Second
	for i := int32(1); i < attempts; i++ {
		backoff *= 2
		if backoff >= outboxMaxRetryBackoff {
			return outboxMaxRetryBackoff
		}
	}

	return backoff
}

func (w *OutboxWorker) wait(ctx context.Context) error {
	timer := time.NewTimer(w.pollInterval)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
