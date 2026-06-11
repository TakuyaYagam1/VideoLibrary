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
SELECT
    id,
    event_type,
    payload,
    created_at,
    processed_at
FROM outbox_events
WHERE processed_at IS NULL
ORDER BY created_at, id
LIMIT sqlc.arg(limit_count);

-- name: MarkOutboxProcessed :exec
UPDATE outbox_events
SET processed_at = now()
WHERE id = sqlc.arg(id)
  AND processed_at IS NULL;
