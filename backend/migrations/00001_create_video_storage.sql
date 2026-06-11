-- +goose Up
CREATE TABLE videos (
    id UUID PRIMARY KEY,
    title TEXT NOT NULL CHECK (btrim(title) <> ''),
    file_path TEXT NOT NULL CHECK (btrim(file_path) <> ''),
    views INTEGER NOT NULL DEFAULT 0 CHECK (views >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    event_type TEXT NOT NULL CHECK (btrim(event_type) <> ''),
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ NULL
);

CREATE INDEX idx_outbox_events_processing
    ON outbox_events (processed_at NULLS FIRST, created_at);

-- +goose Down
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS videos;
