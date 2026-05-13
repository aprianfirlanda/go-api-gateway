-- +goose Up
ALTER TABLE routes
    ADD COLUMN IF NOT EXISTS required_scopes TEXT[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS hmac_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS hmac_secret TEXT NULL,
    ADD COLUMN IF NOT EXISTS replay_window_sec INT NOT NULL DEFAULT 300,
    ADD COLUMN IF NOT EXISTS idempotency_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS idempotency_ttl_sec INT NOT NULL DEFAULT 86400;

-- +goose Down
ALTER TABLE routes
    DROP COLUMN IF EXISTS idempotency_ttl_sec,
    DROP COLUMN IF EXISTS idempotency_enabled,
    DROP COLUMN IF EXISTS replay_window_sec,
    DROP COLUMN IF EXISTS hmac_secret,
    DROP COLUMN IF EXISTS hmac_enabled,
    DROP COLUMN IF EXISTS required_scopes;
