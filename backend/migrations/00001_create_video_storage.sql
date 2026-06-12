-- +goose Up
CREATE TABLE videos (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL CHECK (btrim(title) <> ''),
    file_path TEXT NOT NULL CHECK (btrim(file_path) <> ''),
    views BIGINT NOT NULL DEFAULT 0 CHECK (views >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    event_type TEXT NOT NULL CHECK (btrim(event_type) <> ''),
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ NULL,
    processing_at TIMESTAMPTZ NULL,
    locked_until TIMESTAMPTZ NULL,
    failed_at TIMESTAMPTZ NULL,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    failure_error TEXT NULL
);

CREATE INDEX idx_outbox_events_processing
    ON outbox_events (processed_at, failed_at, locked_until, created_at);

-- +goose Down
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS videos;
