package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo/persistent"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/usecase"
	"github.com/google/uuid"
	logkit "github.com/wahrwelt-kit/go-logkit"
)

const (
	outboxEventInvalidateVideosList = "cache.invalidate_videos_list"
	outboxBatchSize                 = int32(16)
	outboxPollInterval              = time.Second
	outboxOperationTimeout          = 5 * time.Second
)

var (
	errOutboxRepositoryRequired = errors.New("outbox repository is required")
	errOutboxCacheRequired      = errors.New("outbox cache is required")
)

type outboxRepository interface {
	FetchUnprocessed(ctx context.Context, limit int32) ([]persistent.OutboxEvent, error)
	MarkProcessed(ctx context.Context, id uuid.UUID) error
}

type cacheInvalidator interface {
	Del(ctx context.Context, keys ...string) error
}

type OutboxWorker struct {
	repository       outboxRepository
	cache            cacheInvalidator
	logger           logkit.Logger
	batchSize        int32
	pollInterval     time.Duration
	operationTimeout time.Duration
}

func NewOutboxWorker(repository outboxRepository, cache cacheInvalidator, logger logkit.Logger) (*OutboxWorker, error) {
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

func (w *OutboxWorker) fetch(ctx context.Context) ([]persistent.OutboxEvent, error) {
	operationCtx, cancel := context.WithTimeout(ctx, w.operationTimeout)
	defer cancel()

	events, err := w.repository.FetchUnprocessed(operationCtx, w.batchSize)
	if err != nil {
		return nil, fmt.Errorf("fetch outbox events: %w", err)
	}

	return events, nil
}

func (w *OutboxWorker) processEvent(ctx context.Context, event persistent.OutboxEvent) error {
	operationCtx, cancel := context.WithTimeout(ctx, w.operationTimeout)
	defer cancel()

	switch event.EventType {
	case outboxEventInvalidateVideosList:
		if err := w.cache.Del(operationCtx, usecase.VideoListCacheKey); err != nil {
			return fmt.Errorf("invalidate video list cache: %w", err)
		}
	default:
		return fmt.Errorf("unsupported outbox event type %q", event.EventType)
	}

	if err := w.repository.MarkProcessed(operationCtx, event.ID); err != nil {
		return fmt.Errorf("mark event processed: %w", err)
	}

	return nil
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
