package persistent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/domain"
	"github.com/TakuyaYagam1/VideoLibrary/backend/internal/repo/persistent/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wahrwelt-kit/go-pgkit/pgutil"
)

const (
	incrementViewsTxTimeout       = 5 * time.Second
	incrementViewsRollbackTimeout = 2 * time.Second
)

// VideoRepository persists videos in PostgreSQL through sqlc queries.
type VideoRepository struct {
	pool              *pgxpool.Pool
	query             *sqlc.Queries
	publicFileBaseURL string
}

// NewVideoRepository creates a PostgreSQL-backed video repository.
func NewVideoRepository(pool *pgxpool.Pool, publicFileBaseURL ...string) *VideoRepository {
	fileBaseURL := ""
	if len(publicFileBaseURL) > 0 {
		fileBaseURL = strings.TrimRight(publicFileBaseURL[0], "/")
	}

	return &VideoRepository{
		pool:              pool,
		query:             sqlc.New(pool),
		publicFileBaseURL: fileBaseURL,
	}
}

func (r *VideoRepository) ListVideos(ctx context.Context) ([]domain.Video, error) {
	rows, err := r.query.ListVideos(ctx)
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}

	videos := make([]domain.Video, 0, len(rows))
	for _, row := range rows {
		videos = append(videos, r.mapVideo(row))
	}

	return videos, nil
}

func (r *VideoRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Video, error) {
	row, err := r.query.GetVideoByID(ctx, pgUUID(id))
	if err != nil {
		return domain.Video{}, mapVideoError("get video", err)
	}

	return r.mapVideo(row), nil
}

func (r *VideoRepository) IncrementViews(ctx context.Context, id uuid.UUID) (domain.Video, error) {
	if err := ctx.Err(); err != nil {
		return domain.Video{}, err
	}

	eventID, err := uuid.NewV7()
	if err != nil {
		return domain.Video{}, fmt.Errorf("create outbox event id: %w", err)
	}

	txCtx, cancel := context.WithTimeout(ctx, incrementViewsTxTimeout)
	defer cancel()

	tx, err := r.pool.BeginTx(txCtx, pgx.TxOptions{})
	if err != nil {
		return domain.Video{}, fmt.Errorf("begin increment views transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), incrementViewsRollbackTimeout)
			defer rollbackCancel()
			_ = tx.Rollback(rollbackCtx)
		}
	}()

	query := r.query.WithTx(tx)
	if _, incrementErr := query.IncrementViews(txCtx, sqlc.IncrementViewsParams{
		VideoID:       pgUUID(id),
		OutboxEventID: pgUUID(eventID),
	}); incrementErr != nil {
		return domain.Video{}, mapVideoError("increment video views", incrementErr)
	}

	row, err := query.GetVideoByID(txCtx, pgUUID(id))
	if err != nil {
		return domain.Video{}, mapVideoError("get incremented video", err)
	}

	if err := tx.Commit(txCtx); err != nil {
		return domain.Video{}, fmt.Errorf("commit increment views transaction: %w", err)
	}
	committed = true

	return r.mapVideo(row), nil
}

func (r *VideoRepository) mapVideo(row sqlc.Video) domain.Video {
	return domain.Video{
		ID:        uuid.UUID(row.ID.Bytes),
		Title:     row.Title,
		FilePath:  r.publicFilePath(row.FilePath),
		Views:     int64(row.Views),
		CreatedAt: pgutil.TimestamptzToTimeZero(row.CreatedAt),
	}
}

func (r *VideoRepository) publicFilePath(filePath string) string {
	if r.publicFileBaseURL == "" || strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
		return filePath
	}
	if strings.HasPrefix(filePath, "/") {
		return r.publicFileBaseURL + filePath
	}

	return r.publicFileBaseURL + "/" + filePath
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
