# Data Model

## Core Rules

- Every tenant-owned table includes `tenant_id`.
- Every tenant-scoped query filters by `tenant_id`.
- API keys are stored hashed, never plaintext.
- Usage events are append-only.
- Audit logs are append-only.

## Core Entities

- `tenants`
- `tenant_users`, `roles`, `role_bindings`
- `api_products`, `routes`, `upstreams`
- `credentials`
- `protocol_adapter_configs`
- `transformation_templates`
- `rate_limit_policies`, `quota_policies`
- `usage_events`, `billing_summaries`
- `audit_logs`

## Common Columns

```text
id UUID PRIMARY KEY
tenant_id UUID (for tenant-owned tables)
created_at TIMESTAMPTZ NOT NULL DEFAULT now()
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```

## Isolation Constraints

- Tenant-owned uniqueness should include `tenant_id`.
- Foreign keys between tenant-owned records should stay within the same tenant boundary.
