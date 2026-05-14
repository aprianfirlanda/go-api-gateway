# Technical Design

## Runtime Model

The platform is split into two planes:

- Data plane: handles live protocol traffic
- Control plane: manages tenant configuration and policy

## Data Plane Flow

```text
Listener
  -> Tenant resolution
  -> Authentication / authorization
  -> Policy checks
  -> Route match
  -> Inbound protocol adapter
  -> Canonical transformation
  -> Outbound protocol adapter / upstream call
  -> Metrics + audit + usage event
```

## Adapter Strategy

- Keep protocol logic behind adapter interfaces.
- MVP adapters: REST and ISO8583.
- Future adapters: SOAP/XML, gRPC, GraphQL facade, webhooks, queues, files, proprietary TCP.

## System Boundaries

- No protocol-specific logic in shared middleware.
- Billing is based on usage events, not logs.
- Tenant-aware filters are mandatory in repositories and runtime config lookup.

## Implementation Baseline

- Go `1.25.9`
- `net/http` + `chi`
- `slog`
- PostgreSQL + `pgx`
- Redis for runtime state/cache (where configured)
- Hand-written SQL + repository interfaces
