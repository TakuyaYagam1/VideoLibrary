package persistent

import (
	"context"
	"fmt"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo/persistent/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxEvent struct {
	ID        uuid.UUID
	EventType string
	Payload   []byte
}

type OutboxRepository struct {
	query *sqlc.Queries
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{
		query: sqlc.New(pool),
	}
}

func (r *OutboxRepository) FetchUnprocessed(ctx context.Context, limit int32) ([]OutboxEvent, error) {
	rows, err := r.query.FetchUnprocessedOutbox(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch unprocessed outbox: %w", err)
	}

	events := make([]OutboxEvent, 0, len(rows))
	for _, row := range rows {
		events = append(events, OutboxEvent{
			ID:        uuid.UUID(row.ID.Bytes),
			EventType: row.EventType,
			Payload:   row.Payload,
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
