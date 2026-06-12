-- name: InsertOutboxEvent :one
INSERT INTO outbox_events (
    id,
    event_type,
    payload
)
VALUES (
    sqlc.arg(id),
    sqlc.arg(event_type),
    sqlc.arg(payload)
)
RETURNING
    id,
    event_type,
    payload,
    created_at,
    processed_at;

-- name: FetchUnprocessedOutbox :many
WITH claimed AS (
    SELECT id
    FROM outbox_events
    WHERE processed_at IS NULL
      AND failed_at IS NULL
      AND (locked_until IS NULL OR locked_until < now())
    ORDER BY created_at, id
    LIMIT sqlc.arg(limit_count)
    FOR UPDATE SKIP LOCKED
)
UPDATE outbox_events AS event
SET processing_at = now(),
    locked_until = now() + interval '30 seconds',
    attempts = event.attempts + 1
FROM claimed
WHERE event.id = claimed.id
RETURNING
    event.id,
    event.event_type,
    event.payload,
    event.created_at,
    event.processed_at,
    event.processing_at,
    event.locked_until,
    event.failed_at,
    event.attempts,
    event.failure_error;

-- name: MarkOutboxProcessed :exec
UPDATE outbox_events
SET processed_at = now(),
    processing_at = NULL,
    locked_until = NULL,
    failure_error = NULL
WHERE id = sqlc.arg(id)
  AND processed_at IS NULL;

-- name: ReleaseOutbox :exec
UPDATE outbox_events
SET processing_at = NULL,
    locked_until = sqlc.arg(locked_until),
    failure_error = sqlc.arg(failure_error)
WHERE id = sqlc.arg(id)
  AND processed_at IS NULL
  AND failed_at IS NULL;

-- name: MarkOutboxFailed :exec
UPDATE outbox_events
SET failed_at = now(),
    processing_at = NULL,
    locked_until = NULL,
    failure_error = sqlc.arg(failure_error)
WHERE id = sqlc.arg(id)
  AND processed_at IS NULL
  AND failed_at IS NULL;
