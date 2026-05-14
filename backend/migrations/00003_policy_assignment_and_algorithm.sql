-- +goose Up
ALTER TABLE api_products
    ADD COLUMN IF NOT EXISTS rate_limit_policy_id UUID NULL REFERENCES rate_limit_policies(id),
    ADD COLUMN IF NOT EXISTS quota_policy_id UUID NULL REFERENCES quota_policies(id);

ALTER TABLE rate_limit_policies
    ADD COLUMN IF NOT EXISTS algorithm TEXT NOT NULL DEFAULT 'fixed_window';

-- +goose Down
ALTER TABLE rate_limit_policies
    DROP COLUMN IF EXISTS algorithm;

ALTER TABLE api_products
    DROP COLUMN IF EXISTS quota_policy_id,
    DROP COLUMN IF EXISTS rate_limit_policy_id;
