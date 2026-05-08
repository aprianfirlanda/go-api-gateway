# Data Model: Multitenant API Gateway

## 1. Purpose

This document defines the initial database model for the Go-based multitenant API gateway.

The schema supports:

- Multitenancy.
- API products and routes.
- Protocol adapter configuration.
- REST, ISO8583, SOAP/XML, and future protocols.
- Transformation templates.
- Credentials and access control.
- Rate limits and quotas.
- Billing usage events and billing summaries.
- Audit logs.
- Runtime configuration versioning.

PostgreSQL is the recommended primary database for the MVP.

## 2. Data Modeling Principles

- Every tenant-owned table must include `tenant_id`.
- Every tenant-scoped query must filter by `tenant_id`.
- Unique constraints for tenant-owned resources should usually include `tenant_id`.
- API keys must be hashed, not stored in plain text.
- Sensitive config values should be stored through a secret reference, not directly in normal tables.
- Usage events should be append-only.
- Audit logs should be append-only.
- Billing summaries should be reproducible from raw usage events.
- Runtime configuration should be versioned.

## 3. Common Columns

Most tables should use these columns:

```text
id UUID PRIMARY KEY
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Tenant-owned tables should include:

```text
tenant_id UUID NOT NULL REFERENCES tenants(id)
```

Soft-deletable configuration tables may include:

```text
deleted_at TIMESTAMPTZ NULL
```

Status fields should use constrained values at application level or database enum/check constraints.

## 4. Entity Relationship Overview

```text
Tenant
  -> TenantUser
  -> ApiProduct
      -> Route
          -> Upstream
          -> TransformationTemplate
          -> RateLimitPolicy
          -> QuotaPolicy
  -> Credential
  -> ProtocolAdapterConfig
  -> ISO8583Profile
  -> BillingPlan assignment
  -> UsageEvent
  -> BillingSummary
  -> AuditLog
```

## 5. Tables

### 5.1 tenants

Stores tenant organizations.

```text
id UUID PRIMARY KEY
name TEXT NOT NULL
slug TEXT NOT NULL UNIQUE
status TEXT NOT NULL
billing_plan_id UUID NULL REFERENCES billing_plans(id)
metadata JSONB NOT NULL DEFAULT '{}'
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Status values:

- `active`
- `suspended`
- `disabled`

Indexes:

```text
UNIQUE (slug)
INDEX (status)
```

### 5.2 users

Stores platform and tenant users.

```text
id UUID PRIMARY KEY
email TEXT NOT NULL UNIQUE
name TEXT NOT NULL
status TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Status values:

- `active`
- `invited`
- `suspended`
- `disabled`

### 5.3 tenant_users

Maps users to tenants and roles.

```text
id UUID PRIMARY KEY
tenant_id UUID NOT NULL REFERENCES tenants(id)
user_id UUID NOT NULL REFERENCES users(id)
role TEXT NOT NULL
status TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Roles:

- `tenant_admin`
- `api_operator`
- `billing_viewer`
- `developer`
- `auditor`

Indexes:

```text
UNIQUE (tenant_id, user_id)
INDEX (tenant_id, role)
INDEX (user_id)
```

### 5.4 api_products

Stores tenant-owned API products.

```text
id UUID PRIMARY KEY
tenant_id UUID NOT NULL REFERENCES tenants(id)
name TEXT NOT NULL
slug TEXT NOT NULL
description TEXT NULL
status TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
deleted_at TIMESTAMPTZ NULL
```

Status values:

- `draft`
- `active`
- `deprecated`
- `disabled`

Indexes:

```text
UNIQUE (tenant_id, slug)
INDEX (tenant_id, status)
```

### 5.5 routes

Stores route definitions.

```text
id UUID PRIMARY KEY
tenant_id UUID NOT NULL REFERENCES tenants(id)
api_product_id UUID NOT NULL REFERENCES api_products(id)
name TEXT NOT NULL
inbound_protocol TEXT NOT NULL
outbound_protocol TEXT NOT NULL
host TEXT NULL
method TEXT NULL
path TEXT NULL
listener_ref TEXT NULL
upstream_id UUID NOT NULL REFERENCES upstreams(id)
transformation_template_id UUID NULL REFERENCES transformation_templates(id)
rate_limit_policy_id UUID NULL REFERENCES rate_limit_policies(id)
quota_policy_id UUID NULL REFERENCES quota_policies(id)
priority INT NOT NULL DEFAULT 100
timeout_ms INT NOT NULL DEFAULT 5000
status TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
deleted_at TIMESTAMPTZ NULL
```

Protocol values:

- `rest`
- `iso8583`
- `soap_xml`
- `grpc`
- `graphql`
- `webhook`
- `message_queue`
- `file`
- `tcp_custom`

Status values:

- `draft`
- `active`
- `deprecated`
- `disabled`

Indexes:

```text
INDEX (tenant_id, api_product_id)
INDEX (tenant_id, inbound_protocol, host, method, path)
INDEX (tenant_id, listener_ref)
INDEX (tenant_id, status)
INDEX (tenant_id, priority)
```

Notes:

- REST routes use `host`, `method`, and `path`.
- TCP, ISO8583, queue, and file routes may use `listener_ref`.
- Route matching must always include `tenant_id`.

### 5.6 upstreams

Stores target backend configuration.

```text
id UUID PRIMARY KEY
tenant_id UUID NOT NULL REFERENCES tenants(id)
name TEXT NOT NULL
protocol TEXT NOT NULL
config JSONB NOT NULL
secret_ref TEXT NULL
status TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
deleted_at TIMESTAMPTZ NULL
```

Example REST config:

```json
{
  "baseUrl": "https://core.example.com",
  "tls": {
    "verify": true
  }
}
```

Example ISO8583 config:

```json
{
  "host": "10.10.10.20",
  "port": 5000,
  "connectionMode": "persistent",
  "profileId": "profile_uuid"
}
```

Indexes:

```text
UNIQUE (tenant_id, name)
INDEX (tenant_id, protocol)
INDEX (tenant_id, status)
```

### 5.7 protocol_adapter_configs

Stores protocol-specific adapter settings.

```text
id UUID PRIMARY KEY
tenant_id UUID NOT NULL REFERENCES tenants(id)
name TEXT NOT NULL
protocol TEXT NOT NULL
direction TEXT NOT NULL
config JSONB NOT NULL
status TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
deleted_at TIMESTAMPTZ NULL
```

Direction values:

- `inbound`
- `outbound`
- `both`

Example SOAP/XML config:

```json
{
  "soapVersion": "1.1",
  "namespaces": {
    "soapenv": "http://schemas.xmlsoap.org/soap/envelope/",
    "bank": "http://example.com/bank"
  }
}
```

Indexes:

```text
UNIQUE (tenant_id, name)
INDEX (tenant_id, protocol, direction)
INDEX (tenant_id, status)
```

### 5.8 iso8583_profiles

Stores ISO8583 dialect definitions.

```text
id UUID PRIMARY KEY
tenant_id UUID NOT NULL REFERENCES tenants(id)
name TEXT NOT NULL
encoding TEXT NOT NULL
length_header_enabled BOOLEAN NOT NULL DEFAULT true
length_header_size_bytes INT NOT NULL DEFAULT 2
length_header_byte_order TEXT NOT NULL DEFAULT 'big_endian'
bitmap_encoding TEXT NOT NULL
fields JSONB NOT NULL
status TEXT NOT NULL
version INT NOT NULL DEFAULT 1
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
deleted_at TIMESTAMPTZ NULL
```

Indexes:

```text
UNIQUE (tenant_id, name, version)
INDEX (tenant_id, status)
```

### 5.9 transformation_templates

Stores mapping templates.

```text
id UUID PRIMARY KEY
tenant_id UUID NOT NULL REFERENCES tenants(id)
api_product_id UUID NULL REFERENCES api_products(id)
name TEXT NOT NULL
source_protocol TEXT NOT NULL
target_protocol TEXT NOT NULL
version INT NOT NULL
template_body JSONB NOT NULL
status TEXT NOT NULL
created_by UUID NULL REFERENCES users(id)
published_at TIMESTAMPTZ NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
deleted_at TIMESTAMPTZ NULL
```

Status values:

- `draft`
- `published`
- `archived`
- `disabled`

Indexes:

```text
UNIQUE (tenant_id, name, version)
INDEX (tenant_id, source_protocol, target_protocol)
INDEX (tenant_id, status)
```

### 5.10 consumers

Stores consumer applications.

```text
id UUID PRIMARY KEY
tenant_id UUID NOT NULL REFERENCES tenants(id)
name TEXT NOT NULL
slug TEXT NOT NULL
owner_user_id UUID NULL REFERENCES users(id)
status TEXT NOT NULL
metadata JSONB NOT NULL DEFAULT '{}'
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
deleted_at TIMESTAMPTZ NULL
```

Indexes:

```text
UNIQUE (tenant_id, slug)
INDEX (tenant_id, status)
```

### 5.11 credentials

Stores credential metadata. Secret values are not stored in plain text.

```text
id UUID PRIMARY KEY
tenant_id UUID NOT NULL REFERENCES tenants(id)
consumer_id UUID NOT NULL REFERENCES consumers(id)
type TEXT NOT NULL
key_prefix TEXT NULL
secret_hash TEXT NULL
secret_ref TEXT NULL
scopes TEXT[] NOT NULL DEFAULT '{}'
status TEXT NOT NULL
expires_at TIMESTAMPTZ NULL
last_used_at TIMESTAMPTZ NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
revoked_at TIMESTAMPTZ NULL
```

Credential types:

- `api_key`
- `oauth2_client`
- `mtls_certificate`
- `hmac`

Status values:

- `active`
- `suspended`
- `revoked`
- `expired`

Indexes:

```text
INDEX (tenant_id, consumer_id)
INDEX (tenant_id, type, status)
INDEX (key_prefix)
```

Notes:

- API key lookup should use `key_prefix` first, then verify the full key against `secret_hash`.
- Full API keys must never be stored.

### 5.12 consumer_api_access

Maps consumers to API products.

```text
id UUID PRIMARY KEY
tenant_id UUID NOT NULL REFERENCES tenants(id)
consumer_id UUID NOT NULL REFERENCES consumers(id)
api_product_id UUID NOT NULL REFERENCES api_products(id)
status TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Indexes:

```text
UNIQUE (tenant_id, consumer_id, api_product_id)
INDEX (tenant_id, api_product_id)
```

### 5.13 rate_limit_policies

Stores rate limit settings.

```text
id UUID PRIMARY KEY
tenant_id UUID NOT NULL REFERENCES tenants(id)
name TEXT NOT NULL
scope TEXT NOT NULL
limit_count INT NOT NULL
window_seconds INT NOT NULL
burst_count INT NOT NULL DEFAULT 0
status TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
deleted_at TIMESTAMPTZ NULL
```

Scope values:

- `tenant`
- `consumer`
- `api_product`
- `route`

Indexes:

```text
UNIQUE (tenant_id, name)
INDEX (tenant_id, scope)
INDEX (tenant_id, status)
```

### 5.14 quota_policies

Stores quota settings.

```text
id UUID PRIMARY KEY
tenant_id UUID NOT NULL REFERENCES tenants(id)
name TEXT NOT NULL
scope TEXT NOT NULL
period TEXT NOT NULL
quota_count BIGINT NOT NULL
exceeded_behavior TEXT NOT NULL
status TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
deleted_at TIMESTAMPTZ NULL
```

Period values:

- `daily`
- `monthly`

Exceeded behavior values:

- `reject`
- `allow_overage`
- `allow_grace`

Indexes:

```text
UNIQUE (tenant_id, name)
INDEX (tenant_id, scope, period)
INDEX (tenant_id, status)
```

### 5.15 billing_plans

Stores pricing plans.

```text
id UUID PRIMARY KEY
name TEXT NOT NULL
slug TEXT NOT NULL UNIQUE
monthly_fee NUMERIC(18, 4) NOT NULL DEFAULT 0
included_requests BIGINT NOT NULL DEFAULT 0
overage_price NUMERIC(18, 8) NOT NULL DEFAULT 0
currency TEXT NOT NULL
status TEXT NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
deleted_at TIMESTAMPTZ NULL
```

Indexes:

```text
UNIQUE (slug)
INDEX (status)
```

### 5.16 usage_events

Stores raw usage events for billing and analytics.

```text
id UUID PRIMARY KEY
event_id TEXT NOT NULL UNIQUE
tenant_id UUID NOT NULL REFERENCES tenants(id)
consumer_id UUID NULL REFERENCES consumers(id)
api_product_id UUID NULL REFERENCES api_products(id)
route_id UUID NULL REFERENCES routes(id)
source_protocol TEXT NOT NULL
target_protocol TEXT NOT NULL
transformation_type TEXT NULL
status TEXT NOT NULL
http_status INT NULL
upstream_status TEXT NULL
latency_ms INT NOT NULL
billable BOOLEAN NOT NULL DEFAULT true
occurred_at TIMESTAMPTZ NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Status values:

- `success`
- `failed`
- `rejected`
- `timeout`

Indexes:

```text
INDEX (tenant_id, occurred_at)
INDEX (tenant_id, api_product_id, occurred_at)
INDEX (tenant_id, consumer_id, occurred_at)
INDEX (tenant_id, route_id, occurred_at)
INDEX (tenant_id, billable, occurred_at)
```

Partitioning recommendation:

- Partition by `occurred_at` monthly when usage volume grows.

### 5.17 billing_summaries

Stores aggregated billing data.

```text
id UUID PRIMARY KEY
tenant_id UUID NOT NULL REFERENCES tenants(id)
billing_period TEXT NOT NULL
billing_plan_id UUID NULL REFERENCES billing_plans(id)
request_count BIGINT NOT NULL DEFAULT 0
success_count BIGINT NOT NULL DEFAULT 0
failure_count BIGINT NOT NULL DEFAULT 0
rejected_count BIGINT NOT NULL DEFAULT 0
timeout_count BIGINT NOT NULL DEFAULT 0
billable_count BIGINT NOT NULL DEFAULT 0
included_quota BIGINT NOT NULL DEFAULT 0
billable_overage BIGINT NOT NULL DEFAULT 0
monthly_fee NUMERIC(18, 4) NOT NULL DEFAULT 0
overage_amount NUMERIC(18, 4) NOT NULL DEFAULT 0
estimated_amount NUMERIC(18, 4) NOT NULL DEFAULT 0
currency TEXT NOT NULL
status TEXT NOT NULL
calculated_at TIMESTAMPTZ NOT NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Status values:

- `draft`
- `finalized`
- `exported`

Indexes:

```text
UNIQUE (tenant_id, billing_period)
INDEX (billing_period)
INDEX (tenant_id, status)
```

### 5.18 audit_logs

Stores admin and configuration changes.

```text
id UUID PRIMARY KEY
tenant_id UUID NULL REFERENCES tenants(id)
actor_user_id UUID NULL REFERENCES users(id)
actor_role TEXT NULL
action TEXT NOT NULL
resource_type TEXT NOT NULL
resource_id TEXT NOT NULL
before JSONB NULL
after JSONB NULL
ip_address INET NULL
user_agent TEXT NULL
occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Indexes:

```text
INDEX (tenant_id, occurred_at)
INDEX (actor_user_id, occurred_at)
INDEX (resource_type, resource_id)
INDEX (action, occurred_at)
```

Notes:

- Audit logs should be append-only.
- Sensitive fields must be masked before writing `before` or `after`.

### 5.19 config_versions

Tracks runtime configuration versions.

```text
id UUID PRIMARY KEY
tenant_id UUID NULL REFERENCES tenants(id)
scope TEXT NOT NULL
version BIGINT NOT NULL
checksum TEXT NOT NULL
status TEXT NOT NULL
published_at TIMESTAMPTZ NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

Scope examples:

- `global`
- `tenant`
- `routes`
- `templates`
- `adapters`
- `iso8583_profiles`
- `billing`

Indexes:

```text
UNIQUE (tenant_id, scope, version)
INDEX (scope, status)
INDEX (tenant_id, scope, status)
```

### 5.20 outbox_events

Stores durable events for async processing.

```text
id UUID PRIMARY KEY
event_type TEXT NOT NULL
aggregate_type TEXT NOT NULL
aggregate_id TEXT NOT NULL
payload JSONB NOT NULL
status TEXT NOT NULL
attempt_count INT NOT NULL DEFAULT 0
next_attempt_at TIMESTAMPTZ NULL
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
processed_at TIMESTAMPTZ NULL
```

Use cases:

- Billing usage event delivery.
- Config version publishing.
- Audit export.

Indexes:

```text
INDEX (status, next_attempt_at)
INDEX (event_type, created_at)
INDEX (aggregate_type, aggregate_id)
```

## 6. Tenant Isolation Rules

Application rules:

- Never load a route without checking `tenant_id`.
- Never load a credential without checking `tenant_id`.
- Never load a transformation template without checking `tenant_id`.
- Never aggregate billing usage across tenants unless the caller is a platform admin.
- Admin users must be checked through `tenant_users`.

Database rules:

- Use foreign keys where practical.
- Include `tenant_id` in tenant-owned unique constraints.
- Add indexes that start with `tenant_id` for tenant-scoped lookups.

Optional production hardening:

- PostgreSQL Row-Level Security for tenant-owned tables.
- Separate database schemas for high-value dedicated tenants.
- Separate database clusters for dedicated enterprise tenants.

## 7. Billing Aggregation Logic

Billing worker should aggregate from `usage_events`.

Monthly summary logic:

```text
request_count = count(*)
success_count = count(status = 'success')
failure_count = count(status = 'failed')
rejected_count = count(status = 'rejected')
timeout_count = count(status = 'timeout')
billable_count = count(billable = true)
billable_overage = max(billable_count - included_quota, 0)
overage_amount = billable_overage * overage_price
estimated_amount = monthly_fee + overage_amount
```

Aggregation must be replayable:

- Delete or replace draft summary for period.
- Recalculate from raw usage events.
- Finalized summaries require explicit admin action to recalculate.

## 8. Retention Policy

Recommended MVP retention:

```text
usage_events: 13 months
billing_summaries: 7 years
audit_logs: 7 years
request logs: 30-90 days
metrics: 13 months
configuration history: 2 years
```

Retention should be configurable per tenant for enterprise customers.

## 9. Migration Order

Recommended migration order:

1. `billing_plans`
2. `tenants`
3. `users`
4. `tenant_users`
5. `api_products`
6. `upstreams`
7. `protocol_adapter_configs`
8. `iso8583_profiles`
9. `transformation_templates`
10. `rate_limit_policies`
11. `quota_policies`
12. `routes`
13. `consumers`
14. `credentials`
15. `consumer_api_access`
16. `usage_events`
17. `billing_summaries`
18. `audit_logs`
19. `config_versions`
20. `outbox_events`

## 10. Open Decisions

These decisions can be finalized during implementation:

- Whether to use PostgreSQL enums or application-level status validation.
- Whether usage events should first land in PostgreSQL, Kafka, NATS, or another durable stream.
- Whether API product access should be route-level for more granular authorization.
- Whether transformation templates should be JSONB, YAML stored as text, or both.
- Whether high-value tenants need physical database isolation.

