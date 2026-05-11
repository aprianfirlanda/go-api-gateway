-- +goose Up
CREATE TABLE billing_plans (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    monthly_fee NUMERIC(18, 4) NOT NULL DEFAULT 0,
    included_requests BIGINT NOT NULL DEFAULT 0,
    overage_price NUMERIC(18, 8) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL
);
CREATE INDEX billing_plans_status_idx ON billing_plans (status);

CREATE TABLE tenants (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL,
    billing_plan_id UUID NULL REFERENCES billing_plans(id),
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX tenants_status_idx ON tenants (status);

CREATE TABLE users (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tenant_users (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    user_id UUID NOT NULL REFERENCES users(id),
    role TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, user_id)
);
CREATE INDEX tenant_users_tenant_role_idx ON tenant_users (tenant_id, role);
CREATE INDEX tenant_users_user_idx ON tenant_users (user_id);

CREATE TABLE api_products (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    UNIQUE (tenant_id, slug)
);
CREATE INDEX api_products_tenant_status_idx ON api_products (tenant_id, status);

CREATE TABLE upstreams (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    protocol TEXT NOT NULL,
    config JSONB NOT NULL,
    secret_ref TEXT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    UNIQUE (tenant_id, name)
);
CREATE INDEX upstreams_tenant_protocol_idx ON upstreams (tenant_id, protocol);
CREATE INDEX upstreams_tenant_status_idx ON upstreams (tenant_id, status);

CREATE TABLE protocol_adapter_configs (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    protocol TEXT NOT NULL,
    direction TEXT NOT NULL,
    config JSONB NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    UNIQUE (tenant_id, name)
);
CREATE INDEX protocol_adapter_configs_tenant_protocol_direction_idx ON protocol_adapter_configs (tenant_id, protocol, direction);
CREATE INDEX protocol_adapter_configs_tenant_status_idx ON protocol_adapter_configs (tenant_id, status);

CREATE TABLE iso8583_profiles (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    encoding TEXT NOT NULL,
    length_header_enabled BOOLEAN NOT NULL DEFAULT true,
    length_header_size_bytes INT NOT NULL DEFAULT 2,
    length_header_byte_order TEXT NOT NULL DEFAULT 'big_endian',
    bitmap_encoding TEXT NOT NULL,
    fields JSONB NOT NULL,
    status TEXT NOT NULL,
    version INT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    UNIQUE (tenant_id, name, version)
);
CREATE INDEX iso8583_profiles_tenant_status_idx ON iso8583_profiles (tenant_id, status);

CREATE TABLE transformation_templates (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    api_product_id UUID NULL REFERENCES api_products(id),
    name TEXT NOT NULL,
    source_protocol TEXT NOT NULL,
    target_protocol TEXT NOT NULL,
    version INT NOT NULL,
    template_body JSONB NOT NULL,
    status TEXT NOT NULL,
    created_by UUID NULL REFERENCES users(id),
    published_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    UNIQUE (tenant_id, name, version)
);
CREATE INDEX transformation_templates_tenant_protocols_idx ON transformation_templates (tenant_id, source_protocol, target_protocol);
CREATE INDEX transformation_templates_tenant_status_idx ON transformation_templates (tenant_id, status);

CREATE TABLE consumers (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    owner_user_id UUID NULL REFERENCES users(id),
    status TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    UNIQUE (tenant_id, slug)
);
CREATE INDEX consumers_tenant_status_idx ON consumers (tenant_id, status);

CREATE TABLE credentials (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    consumer_id UUID NOT NULL REFERENCES consumers(id),
    type TEXT NOT NULL,
    key_prefix TEXT NULL,
    secret_hash TEXT NULL,
    secret_ref TEXT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    status TEXT NOT NULL,
    expires_at TIMESTAMPTZ NULL,
    last_used_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ NULL
);
CREATE INDEX credentials_tenant_consumer_idx ON credentials (tenant_id, consumer_id);
CREATE INDEX credentials_tenant_type_status_idx ON credentials (tenant_id, type, status);
CREATE INDEX credentials_key_prefix_idx ON credentials (key_prefix);

CREATE TABLE consumer_api_access (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    consumer_id UUID NOT NULL REFERENCES consumers(id),
    api_product_id UUID NOT NULL REFERENCES api_products(id),
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, consumer_id, api_product_id)
);
CREATE INDEX consumer_api_access_tenant_api_product_idx ON consumer_api_access (tenant_id, api_product_id);

CREATE TABLE rate_limit_policies (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    scope TEXT NOT NULL,
    limit_count INT NOT NULL,
    window_seconds INT NOT NULL,
    burst_count INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    UNIQUE (tenant_id, name)
);
CREATE INDEX rate_limit_policies_tenant_scope_idx ON rate_limit_policies (tenant_id, scope);
CREATE INDEX rate_limit_policies_tenant_status_idx ON rate_limit_policies (tenant_id, status);

CREATE TABLE quota_policies (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    scope TEXT NOT NULL,
    period TEXT NOT NULL,
    quota_count BIGINT NOT NULL,
    exceeded_behavior TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL,
    UNIQUE (tenant_id, name)
);
CREATE INDEX quota_policies_tenant_scope_period_idx ON quota_policies (tenant_id, scope, period);
CREATE INDEX quota_policies_tenant_status_idx ON quota_policies (tenant_id, status);

CREATE TABLE routes (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    api_product_id UUID NOT NULL REFERENCES api_products(id),
    name TEXT NOT NULL,
    inbound_protocol TEXT NOT NULL,
    outbound_protocol TEXT NOT NULL,
    host TEXT NULL,
    method TEXT NULL,
    path TEXT NULL,
    listener_ref TEXT NULL,
    upstream_id UUID NOT NULL REFERENCES upstreams(id),
    transformation_template_id UUID NULL REFERENCES transformation_templates(id),
    rate_limit_policy_id UUID NULL REFERENCES rate_limit_policies(id),
    quota_policy_id UUID NULL REFERENCES quota_policies(id),
    priority INT NOT NULL DEFAULT 100,
    timeout_ms INT NOT NULL DEFAULT 5000,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ NULL
);
CREATE INDEX routes_tenant_api_product_idx ON routes (tenant_id, api_product_id);
CREATE INDEX routes_tenant_match_idx ON routes (tenant_id, inbound_protocol, host, method, path);
CREATE INDEX routes_tenant_listener_idx ON routes (tenant_id, listener_ref);
CREATE INDEX routes_tenant_status_idx ON routes (tenant_id, status);
CREATE INDEX routes_tenant_priority_idx ON routes (tenant_id, priority);

CREATE TABLE usage_events (
    id UUID PRIMARY KEY,
    event_id TEXT NOT NULL UNIQUE,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    consumer_id UUID NULL REFERENCES consumers(id),
    api_product_id UUID NULL REFERENCES api_products(id),
    route_id UUID NULL REFERENCES routes(id),
    source_protocol TEXT NOT NULL,
    target_protocol TEXT NOT NULL,
    transformation_type TEXT NULL,
    status TEXT NOT NULL,
    http_status INT NULL,
    upstream_status TEXT NULL,
    latency_ms INT NOT NULL,
    billable BOOLEAN NOT NULL DEFAULT true,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX usage_events_tenant_occurred_at_idx ON usage_events (tenant_id, occurred_at);
CREATE INDEX usage_events_tenant_api_product_occurred_at_idx ON usage_events (tenant_id, api_product_id, occurred_at);
CREATE INDEX usage_events_tenant_consumer_occurred_at_idx ON usage_events (tenant_id, consumer_id, occurred_at);
CREATE INDEX usage_events_tenant_route_occurred_at_idx ON usage_events (tenant_id, route_id, occurred_at);
CREATE INDEX usage_events_tenant_billable_occurred_at_idx ON usage_events (tenant_id, billable, occurred_at);

CREATE TABLE billing_summaries (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    billing_period TEXT NOT NULL,
    billing_plan_id UUID NULL REFERENCES billing_plans(id),
    request_count BIGINT NOT NULL DEFAULT 0,
    success_count BIGINT NOT NULL DEFAULT 0,
    failure_count BIGINT NOT NULL DEFAULT 0,
    rejected_count BIGINT NOT NULL DEFAULT 0,
    timeout_count BIGINT NOT NULL DEFAULT 0,
    billable_count BIGINT NOT NULL DEFAULT 0,
    included_quota BIGINT NOT NULL DEFAULT 0,
    billable_overage BIGINT NOT NULL DEFAULT 0,
    monthly_fee NUMERIC(18, 4) NOT NULL DEFAULT 0,
    overage_amount NUMERIC(18, 4) NOT NULL DEFAULT 0,
    estimated_amount NUMERIC(18, 4) NOT NULL DEFAULT 0,
    currency TEXT NOT NULL,
    status TEXT NOT NULL,
    calculated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, billing_period)
);
CREATE INDEX billing_summaries_billing_period_idx ON billing_summaries (billing_period);
CREATE INDEX billing_summaries_tenant_status_idx ON billing_summaries (tenant_id, status);

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY,
    tenant_id UUID NULL REFERENCES tenants(id),
    actor_user_id UUID NULL REFERENCES users(id),
    actor_role TEXT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    before JSONB NULL,
    after JSONB NULL,
    ip_address INET NULL,
    user_agent TEXT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_tenant_occurred_at_idx ON audit_logs (tenant_id, occurred_at);
CREATE INDEX audit_logs_actor_occurred_at_idx ON audit_logs (actor_user_id, occurred_at);
CREATE INDEX audit_logs_resource_idx ON audit_logs (resource_type, resource_id);
CREATE INDEX audit_logs_action_occurred_at_idx ON audit_logs (action, occurred_at);

CREATE TABLE config_versions (
    id UUID PRIMARY KEY,
    tenant_id UUID NULL REFERENCES tenants(id),
    scope TEXT NOT NULL,
    version BIGINT NOT NULL,
    checksum TEXT NOT NULL,
    status TEXT NOT NULL,
    published_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, scope, version)
);
CREATE INDEX config_versions_scope_status_idx ON config_versions (scope, status);
CREATE INDEX config_versions_tenant_scope_status_idx ON config_versions (tenant_id, scope, status);

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    event_type TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    payload JSONB NOT NULL,
    status TEXT NOT NULL,
    attempt_count INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ NULL
);

-- +goose Down
DROP TABLE IF EXISTS outbox_events;
DROP TABLE IF EXISTS config_versions;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS billing_summaries;
DROP TABLE IF EXISTS usage_events;
DROP TABLE IF EXISTS routes;
DROP TABLE IF EXISTS quota_policies;
DROP TABLE IF EXISTS rate_limit_policies;
DROP TABLE IF EXISTS consumer_api_access;
DROP TABLE IF EXISTS credentials;
DROP TABLE IF EXISTS consumers;
DROP TABLE IF EXISTS transformation_templates;
DROP TABLE IF EXISTS iso8583_profiles;
DROP TABLE IF EXISTS protocol_adapter_configs;
DROP TABLE IF EXISTS upstreams;
DROP TABLE IF EXISTS api_products;
DROP TABLE IF EXISTS tenant_users;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS tenants;
DROP TABLE IF EXISTS billing_plans;
