package persistent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo/persistent/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxRepository struct {
	query *sqlc.Queries
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{
		query: sqlc.New(pool),
	}
}

func (r *OutboxRepository) FetchUnprocessed(ctx context.Context, limit int32) ([]repo.OutboxEvent, error) {
	rows, err := r.query.FetchUnprocessedOutbox(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch unprocessed outbox: %w", err)
	}

	events := make([]repo.OutboxEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, repo.OutboxEvent{
			ID:        uuid.UUID(row.ID.Bytes),
			EventType: row.EventType,
			Payload:   row.Payload,
			Attempts:  row.Attempts,
		})
	}

	return events, nil
}

func (r *OutboxRepository) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	if err := r.query.MarkOutboxProcessed(ctx, pgUUID(id)); err != nil {
		return fmt.Errorf("mark outbox processed: %w", err)
	}

	return nil
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id uuid.UUID, reason string) error {
	if err := r.query.MarkOutboxFailed(ctx, sqlc.MarkOutboxFailedParams{
		ID:           pgUUID(id),
		FailureError: outboxErrorText(reason),
	}); err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}

	return nil
}

func (r *OutboxRepository) Release(ctx context.Context, id uuid.UUID, reason string, retryAt time.Time) error {
	if err := r.query.ReleaseOutbox(ctx, sqlc.ReleaseOutboxParams{
		ID:           pgUUID(id),
		LockedUntil:  pgtype.Timestamptz{Time: retryAt.UTC(), Valid: true},
		FailureError: outboxErrorText(reason),
	}); err != nil {
		return fmt.Errorf("release outbox event: %w", err)
	}

	return nil
}

func trimOutboxError(reason string) string {
	reason = strings.TrimSpace(reason)
	const maxLen = 1024
	if len(reason) > maxLen {
		return reason[:maxLen]
	}

	return reason
}

func outboxErrorText(reason string) pgtype.Text {
	reason = trimOutboxError(reason)
	return pgtype.Text{
		String: reason,
		Valid:  reason != "",
	}
}
