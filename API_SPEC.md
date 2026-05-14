# API Spec

## Control Plane

- Base path: `/admin/v1`
- Content type: JSON
- IDs: UUID
- Timestamps: RFC3339 UTC

## Required Capabilities

- Tenant and user management
- API product, route, and upstream management
- Credential lifecycle
- Adapter and transformation config
- Policy management (authz, rate/quotas)
- Billing usage and summary read APIs
- Audit log query APIs

## Runtime Endpoints

- `GET /healthz`
- `GET /readyz`

## Security Baseline

- Admin APIs require authenticated operator/admin context.
- Tenant-scoped reads/writes must enforce tenant boundaries.
- Sensitive fields must be redacted in responses and logs.

## Pagination Convention

```json
{
  "data": [],
  "nextCursor": null
}
```
