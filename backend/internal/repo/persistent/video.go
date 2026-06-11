package persistent

import (
	"context"
	"fmt"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo/persistent/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wahrwelt-kit/go-pgkit/pgutil"
)

// VideoRepository persists videos in PostgreSQL through sqlc queries.
type VideoRepository struct {
	pool  *pgxpool.Pool
	query *sqlc.Queries
}

// NewVideoRepository creates a PostgreSQL-backed video repository.
func NewVideoRepository(pool *pgxpool.Pool) *VideoRepository {
	return &VideoRepository{
		pool:  pool,
		query: sqlc.New(pool),
	}
}

func (r *VideoRepository) ListVideos(ctx context.Context) ([]domain.Video, error) {
	rows, err := r.query.ListVideos(ctx)
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}

	videos := make([]domain.Video, 0, len(rows))
	for _, row := range rows {
		videos = append(videos, mapVideo(row))
	}

	return videos, nil
}

func (r *VideoRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Video, error) {
	row, err := r.query.GetVideoByID(ctx, pgUUID(id))
	if err != nil {
		return domain.Video{}, mapVideoError("get video", err)
	}

	return mapVideo(row), nil
}

func (r *VideoRepository) IncrementViews(ctx context.Context, id uuid.UUID) (domain.Video, error) {
	return r.IncrementViewsWithOutbox(ctx, id)
}

func (r *VideoRepository) IncrementViewsWithOutbox(ctx context.Context, id uuid.UUID) (domain.Video, error) {
	eventID, err := uuid.NewV7()
	if err != nil {
		return domain.Video{}, fmt.Errorf("create outbox event id: %w", err)
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Video{}, fmt.Errorf("begin increment views transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	query := r.query.WithTx(tx)
	if _, incrementErr := query.IncrementViewsWithOutbox(ctx, sqlc.IncrementViewsWithOutboxParams{
		VideoID:       pgUUID(id),
		OutboxEventID: pgUUID(eventID),
	}); incrementErr != nil {
		return domain.Video{}, mapVideoError("increment video views", incrementErr)
	}

	row, err := query.GetVideoByID(ctx, pgUUID(id))
	if err != nil {
		return domain.Video{}, mapVideoError("get incremented video", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Video{}, fmt.Errorf("commit increment views transaction: %w", err)
	}
	committed = true

	return mapVideo(row), nil
}

func mapVideo(row sqlc.Video) domain.Video {
	return domain.Video{
		ID:        uuid.UUID(row.ID.Bytes),
		Title:     row.Title,
		FilePath:  row.FilePath,
		Views:     int64(row.Views),
		CreatedAt: pgutil.TimestamptzToTimeZero(row.CreatedAt),
	}
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: [16]byte(id),
		Valid: true,
	}
}

func mapVideoError(operation string, err error) error {
	if pgutil.IsNoRows(err) {
		return fmt.Errorf("%s: %w", operation, domain.ErrVideoNotFound)
	}

	return fmt.Errorf("%s: %w", operation, err)
}
