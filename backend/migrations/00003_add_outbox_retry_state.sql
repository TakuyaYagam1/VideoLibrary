-- +goose Up
ALTER TABLE outbox_events
    ADD COLUMN IF NOT EXISTS processing_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS locked_until TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS failed_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS attempts INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS failure_error TEXT NULL;

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conrelid = 'outbox_events'::regclass
          AND conname = 'outbox_events_attempts_check'
    ) THEN
        ALTER TABLE outbox_events
            ADD CONSTRAINT outbox_events_attempts_check CHECK (attempts >= 0);
    END IF;
END $$;
-- +goose StatementEnd

CREATE INDEX IF NOT EXISTS idx_outbox_events_processing
    ON outbox_events (processed_at, failed_at, locked_until, created_at);

-- +goose Down
-- Compatibility migration: 00001 owns these columns for fresh installs, so rollback is intentionally non-destructive.
SELECT 1;
