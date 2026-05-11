# Implementation Plan: Go-Based Multitenant API Gateway

## 1. Purpose

This document converts the product and technical designs into a practical build plan.

The goal is to build a Go-based, multitenant, finance-focused API gateway without depending on gateway runtimes such as Kong, APISIX, Tyk, KrakenD, or Envoy.

The gateway should start with REST and ISO8583 support, but the implementation must use a protocol adapter model so SOAP/XML, gRPC, GraphQL, webhooks, message queues, files, and proprietary TCP protocols can be added later.

Technology choices are locked in [TECHNOLOGY_DECISIONS.md](TECHNOLOGY_DECISIONS.md). Copy-paste implementation prompts are listed in [SPRINT_PROMPTS.md](SPRINT_PROMPTS.md).

The active Go implementation module lives in `syra-backend/`.

All sprint implementation work must be performed inside `syra-backend/` unless explicitly requested otherwise. The repository root is reserved for design and planning documentation. Do not create a root-level `go.mod`, `cmd/`, `internal/`, or `pkg/` implementation tree.

## 2. Implementation Principles

- Build the gateway runtime in Go.
- Keep the data plane independent from the control plane.
- Keep request processing fast and predictable.
- Use tenant-aware configuration everywhere.
- Use protocol adapters instead of hard-coded protocol pairs.
- Use a canonical message model between protocol adapters and transformations.
- Treat billing events as first-class data, not logs.
- Mask sensitive financial data by default.
- Prefer simple, testable interfaces before adding advanced features.

## 3. Target Repository Structure

```text
syra-backend/
  cmd/
  gateway/
    main.go
  control-plane/
    main.go
  billing-worker/
    main.go

  internal/
  app/
    gateway/
    controlplane/
    billingworker/

  config/
  storage/
  tenant/
  auth/
  rbac/
  gateway/
    listener/
    middleware/
    router/
    policy/
    upstream/
  protocol/
    adapter.go
    canonical.go
    registry.go
    rest/
    iso8583/
    soap/
  transform/
  ratelimit/
  quota/
  billing/
  audit/
  observability/

  pkg/
  errors/
  ids/
  masking/
```

## 4. Milestone 1: Project Foundation

The milestone sections describe capability groups. For implementation work, use the sprint roadmap in section 20 as the source of truth.

Goal: create a clean Go service foundation.

Tasks:

- Initialize Go module.
- Initialize or update the Go module inside `syra-backend/`.
- Add basic `syra-backend/cmd/gateway/main.go`.
- Add configuration loader.
- Add structured logger.
- Add graceful shutdown.
- Add health endpoint.
- Add readiness endpoint.
- Add request ID middleware.
- Add basic test setup.

Expected result:

- `go test ./...` runs from inside `syra-backend/`.
- `go run ./cmd/gateway` starts an HTTP server from inside `syra-backend/`.
- `GET /healthz` returns healthy status.

## 5. Milestone 2: Tenant-Aware Routing

Goal: route REST traffic using tenant-aware route configuration.

Tasks:

- Create tenant model.
- Create API product model.
- Create route model.
- Implement in-memory route registry.
- Match route by tenant, host, method, and path.
- Add route status: draft, active, disabled.
- Add route timeout configuration.

Initial structs:

```go
type Tenant struct {
    ID     string
    Name   string
    Status string
}

type Route struct {
    ID               string
    TenantID         string
    APIProductID     string
    InboundProtocol  string
    OutboundProtocol string
    Host             string
    Method           string
    Path             string
    UpstreamRef      string
    TemplateRef      string
    TimeoutMs        int
    Status           string
}
```

Expected result:

- Gateway can resolve a route for a tenant.
- Unknown routes return `404`.
- Disabled routes return `404` or `403` based on policy.
- Early sprint route statuses may start with `draft`, `active`, and `disabled`; the full data model also includes `deprecated`.

## 6. Milestone 3: Authentication and Tenant Resolution

Goal: resolve tenant from credentials and reject unauthenticated traffic.

Tasks:

- Implement API key model.
- Store only hashed API keys.
- Add API key lookup.
- Resolve tenant from API key.
- Attach tenant ID, consumer ID, and credential ID to request context.
- Add credential statuses: active, suspended, revoked.

Authentication behavior:

- Missing credential returns `401`.
- Invalid credential returns `401`.
- Suspended credential returns `403`.
- Revoked credential returns `403`.

Expected result:

- REST requests cannot reach routes without valid credentials.
- Tenant cannot access another tenant route using its credential.

## 7. Milestone 4: REST Proxy Runtime

Goal: support REST inbound to REST outbound.

Tasks:

- Implement REST protocol adapter.
- Implement REST upstream client.
- Forward allowed headers.
- Strip hop-by-hop headers.
- Apply route timeout.
- Return upstream response body and status.
- Add upstream latency metric.

REST adapter responsibilities:

- Decode HTTP request into canonical message.
- Encode canonical response into HTTP response.
- Call REST upstream from canonical message.

Expected result:

- Gateway can proxy REST requests to a REST backend.
- Gateway emits request logs and metrics.

## 8. Milestone 5: Protocol Adapter Interface

Goal: make protocol support extensible before adding more protocols.

Tasks:

- Create `ProtocolAdapter` interface.
- Create `UpstreamAdapter` interface.
- Create adapter registry.
- Register REST adapter.
- Add source and target protocol fields to routes.
- Make gateway route execution use adapters instead of direct REST logic.

Interfaces:

```go
type ProtocolAdapter interface {
    Name() string
    Decode(ctx context.Context, req InboundRequest) (CanonicalMessage, error)
    Encode(ctx context.Context, msg CanonicalMessage) (OutboundResponse, error)
}

type UpstreamAdapter interface {
    Name() string
    Call(ctx context.Context, target UpstreamTarget, msg CanonicalMessage) (CanonicalMessage, error)
}
```

Expected result:

- REST to REST still works.
- Adding ISO8583 does not require rewriting gateway middleware.

## 9. Milestone 6: Canonical Message Model

Goal: normalize protocol payloads into a shared internal message.

Tasks:

- Create canonical message struct.
- Add metadata fields: tenant, route, consumer, source protocol, target protocol.
- Add generic fields map.
- Add raw payload reference.
- Add sensitive key list.
- Add helper functions for masking.

Canonical message:

```go
type CanonicalMessage struct {
    TenantID       string
    ConsumerID     string
    APIProductID   string
    RouteID        string
    SourceProtocol string
    TargetProtocol string
    Operation      string
    Headers        map[string]string
    Fields         map[string]any
    Metadata       map[string]any
    RawRef         string
    SensitiveKeys  []string
}
```

Expected result:

- REST adapter can convert requests and responses through canonical messages.
- Transformation engine can work against canonical messages.

## 10. Milestone 7: Transformation Engine

Goal: transform canonical messages using tenant-owned templates.

Tasks:

- Define transformation template model.
- Support template versioning.
- Support published and draft status.
- Implement field mapping.
- Implement default values.
- Implement simple functions: amount formatting, currency conversion, date formatting.
- Implement validation before publishing a template.
- Implement dry-run transformation test.

Template example:

```yaml
name: card-authorization-v1
sourceProtocol: rest
targetProtocol: iso8583
request:
  fields:
    transactionType: "$.fields.transactionType"
    amount: "$.fields.amount"
    currency: "$.fields.currency"
    terminalId: "$.fields.terminalId"
response:
  fields:
    responseCode: "$.fields.responseCode"
    authorizationCode: "$.fields.authorizationCode"
```

Expected result:

- Gateway can apply a configured transformation before calling upstream.
- Invalid templates are rejected before publish.

## 11. Milestone 8: ISO8583 Adapter

Goal: support REST to ISO8583 and ISO8583 to REST.

Tasks:

- Create ISO8583 profile model.
- Implement field definitions.
- Support fixed fields, LLVAR, and LLLVAR.
- Support bitmap handling.
- Support length header configuration.
- Implement pack and unpack logic.
- Implement ISO8583 TCP upstream client.
- Implement ISO8583 inbound TCP listener.
- Map ISO8583 messages to canonical messages.
- Map canonical messages to ISO8583 messages.

Expected result:

- REST request can be transformed into ISO8583 and sent to a TCP upstream.
- ISO8583 response can be transformed back into REST JSON.
- ISO8583 inbound message can be transformed to REST upstream call.

## 12. Milestone 9: Policy Engine

Goal: execute shared gateway policies consistently across protocols.

Tasks:

- Define request pipeline.
- Implement IP allowlist.
- Implement request size limit.
- Implement rate limit policy.
- Implement quota policy.
- Implement schema validation placeholder.
- Implement policy error mapping.

Policy order:

1. Request ID.
2. Tenant resolution.
3. Authentication.
4. Authorization.
5. IP allowlist.
6. Request size validation.
7. Rate limit.
8. Quota check.
9. Protocol decode.
10. Transformation.
11. Upstream call.
12. Response transformation.
13. Protocol encode.
14. Metrics.
15. Billing event.

Expected result:

- REST and ISO8583 routes use the same policy pipeline.
- Rate limit and quota behavior is tenant-aware.

## 13. Milestone 10: Billing Metering

Goal: produce reliable tenant-scoped usage events.

Tasks:

- Define usage event schema.
- Emit usage event for every request attempt.
- Include tenant, consumer, API, route, protocol, status, latency, and billable flag.
- Store usage events.
- Add durable outbox if direct write fails.
- Build billing aggregation worker.
- Generate billing summaries per tenant and billing period.

Usage event fields:

```text
event_id
tenant_id
consumer_id
api_product_id
route_id
source_protocol
target_protocol
status
http_status
upstream_status
latency_ms
billable
occurred_at
```

Expected result:

- Billing summaries can be generated from raw usage events.
- Billing aggregation can be replayed.

## 14. Milestone 11: Control Plane API

Goal: manage tenants, routes, credentials, templates, adapters, and billing configuration.

Tasks:

- Create admin HTTP service.
- Add tenant CRUD.
- Add API product CRUD.
- Add route CRUD.
- Add credential create, rotate, revoke.
- Add transformation template CRUD and publish.
- Add ISO8583 profile CRUD.
- Add protocol adapter config CRUD.
- Add billing plan CRUD.
- Add usage and billing summary read APIs.
- Add audit log read API.

Expected result:

- Gateway configuration can be managed through APIs instead of hard-coded files.
- Every admin change writes an audit event.

## 15. Milestone 12: Storage Layer

Goal: persist configuration, audit logs, and billing data.

Tasks:

- Add PostgreSQL migrations.
- Add repository interfaces.
- Add repository implementations.
- Add transaction handling.
- Add tenant-scoped query helpers.
- Add indexes for route lookup and billing aggregation.
- Add integration tests with PostgreSQL.

Initial tables:

- `tenants`
- `api_products`
- `routes`
- `credentials`
- `protocol_adapter_configs`
- `iso8583_profiles`
- `transformation_templates`
- `rate_limit_policies`
- `quota_policies`
- `billing_plans`
- `usage_events`
- `billing_summaries`
- `audit_logs`
- `config_versions`

Expected result:

- Control plane persists configuration.
- Gateway can load active configuration.

## 16. Milestone 13: Config Reload

Goal: update gateway runtime configuration without restarting the gateway.

Tasks:

- Add config snapshot model.
- Add config version tracking.
- Add periodic config reload.
- Keep last known good config.
- Reject invalid config snapshots.
- Add route/template/profile version references.

Expected result:

- Admin updates can be picked up by gateway instances.
- Bad config does not break live traffic.

## 17. Milestone 14: Observability and Audit

Goal: make the gateway operable in finance environments.

Tasks:

- Add structured JSON logs.
- Add OpenTelemetry tracing.
- Add Prometheus metrics.
- Add tenant ID and route ID to logs.
- Add sensitive data masking.
- Add audit event writer.
- Add dashboard metric names.

Required metrics:

- Request count.
- Request latency.
- Upstream latency.
- Transformation latency.
- Authentication failures.
- Rate limit rejects.
- Quota rejects.
- Protocol adapter errors.
- ISO8583 timeouts.
- Billing event failures.

Expected result:

- Operators can trace traffic by tenant, request ID, route, and protocol.

## 18. Milestone 15: SOAP/XML Proof Adapter

Goal: prove the gateway is not limited to REST and ISO8583.

Tasks:

- Add SOAP/XML outbound adapter.
- Support SOAP envelope template.
- Support XML namespace configuration.
- Support XML response path extraction.
- Add REST to SOAP/XML integration test.

Expected result:

- One non-REST, non-ISO8583 adapter works through the same route, policy, transformation, billing, and observability pipeline.

## 19. Testing Plan

Unit tests:

- Route matching.
- Tenant resolution.
- API key hashing.
- Credential status handling.
- Adapter registry.
- Canonical message mapping.
- Transformation templates.
- ISO8583 packing and unpacking.
- PAN masking.
- Billing event creation.

Integration tests:

- REST to REST.
- REST to ISO8583.
- ISO8583 to REST.
- REST to SOAP/XML.
- Rate limit exceeded.
- Quota exceeded.
- Tenant isolation.
- Billing aggregation.
- Config reload.

Load tests:

- REST proxy throughput.
- REST to ISO8583 latency.
- ISO8583 TCP connection behavior.
- Billing event write throughput.

## 20. Sprint Roadmap

Use these sprint names when asking for implementation work.

Example prompt:

```text
Implement Sprint 1 from IMPLEMENTATION_PLAN.md. Add tests and run them.
```

### Sprint 1: Gateway Foundation

Goal: create the smallest working Go gateway service.

Scope:

1. Initialize Go module.
2. Create `syra-backend/cmd/gateway`.
3. Add health endpoint.
4. Add readiness endpoint.
5. Add graceful shutdown.
6. Add request ID middleware.
7. Add basic config loading.
8. Add structured logging.
9. Add unit tests for health, readiness, and request ID behavior.

Sprint done criteria:

- Gateway starts locally.
- Health endpoint works.
- Readiness endpoint works.
- Request ID is assigned when missing.
- `go test ./...` passes.

### Sprint 2: Tenant Routing and Authentication

Goal: add tenant-aware route matching and API key authentication.

Scope:

1. Add tenant model.
2. Add API product model.
3. Add route model.
4. Add in-memory route registry.
5. Match route by tenant, host, method, and path.
6. Add API key credential model.
7. Hash and verify API keys.
8. Resolve tenant and consumer from API key.
9. Reject unauthorized requests.
10. Add unit tests for route matching and authentication.

Sprint done criteria:

- A configured API key maps to a tenant.
- Unknown API key returns `401`.
- Suspended or revoked credential returns `403`.
- Tenant A credential cannot access Tenant B route.
- Active route can be matched by host, method, and path.
- `go test ./...` passes.

### Sprint 3: REST Proxy

Goal: support REST inbound to REST upstream routing.

Scope:

1. Add upstream model.
2. Add REST upstream client.
3. Add REST proxy handler.
4. Forward allowed headers.
5. Strip hop-by-hop headers.
6. Apply route timeout.
7. Return upstream status and body.
8. Add integration test with mock REST upstream.

Sprint done criteria:

- A REST request proxies to a mock upstream.
- Route timeout is enforced.
- Unauthorized requests do not reach upstream.
- Hop-by-hop headers are stripped.
- `go test ./...` passes.

### Sprint 4: Protocol Adapter Foundation

Goal: introduce adapter interfaces and canonical messages before adding more protocols.

Scope:

1. Add `ProtocolAdapter` interface.
2. Add `UpstreamAdapter` interface.
3. Add adapter registry.
4. Add canonical message model.
5. Move REST handling behind REST adapter.
6. Keep REST to REST behavior working.
7. Add tests for adapter registry and canonical mapping.

Sprint done criteria:

- REST adapter is registered.
- Gateway route execution uses adapter interfaces.
- REST to REST still works.
- Canonical message can represent REST request and response.
- `go test ./...` passes.

### Sprint 5: Transformation Engine MVP

Goal: add the first template-driven transformation engine.

Scope:

1. Add transformation template model.
2. Add field mapping.
3. Add static values.
4. Add basic functions: `formatAmount`, `currencyNumeric`, `nowMMddHHmmss`, `generateStan`, `maskPan`.
5. Add template validation.
6. Add dry-run execution.
7. Add masking for sensitive fields.

Sprint done criteria:

- Template can transform canonical request fields.
- Invalid template returns validation errors.
- Dry-run output masks sensitive fields.
- Transformation tests cover missing fields and function errors.
- `go test ./...` passes.

### Sprint 6: ISO8583 Outbound

Goal: support REST to ISO8583.

Scope:

1. Add ISO8583 profile model.
2. Add ISO8583 packer for fixed, LLVAR, and LLLVAR fields.
3. Add bitmap handling.
4. Add length header handling.
5. Add ISO8583 TCP upstream client.
6. Transform REST canonical message to ISO8583 canonical shape.
7. Send ISO8583 request and parse response.
8. Add mock ISO8583 upstream integration test.

Sprint done criteria:

- REST request can produce ISO8583 message.
- ISO8583 message respects profile field definitions.
- Mock ISO8583 upstream receives expected fields.
- ISO8583 response returns REST JSON.
- `go test ./...` passes.

### Sprint 7: ISO8583 Inbound

Goal: support ISO8583 to REST.

Scope:

1. Add TCP listener for ISO8583 inbound messages.
2. Resolve tenant from listener profile.
3. Decode ISO8583 request.
4. Transform ISO8583 canonical message to REST canonical shape.
5. Call REST upstream.
6. Transform REST response to ISO8583 response.
7. Add integration test for ISO8583 inbound flow.

Sprint done criteria:

- Gateway accepts ISO8583 TCP request.
- Gateway calls REST upstream.
- Gateway returns ISO8583 response.
- Malformed ISO8583 message is rejected safely.
- `go test ./...` passes.

### Sprint 8: Policy Engine

Goal: add shared policies for runtime traffic.

Scope:

1. Add policy pipeline.
2. Add IP allowlist.
3. Add request size limit.
4. Add rate limit interface.
5. Add in-memory rate limiter for MVP.
6. Add quota policy interface.
7. Add policy error mapping.

Sprint done criteria:

- Policies run in deterministic order.
- Rate-limited request returns `429`.
- Blocked IP is rejected.
- Oversized request is rejected.
- REST and ISO8583 routes use the same policy pipeline.
- `go test ./...` passes.

### Sprint 9: Billing Metering

Goal: emit usage events and generate basic billing summaries.

Scope:

1. Add usage event model.
2. Emit usage event for every request attempt.
3. Add billable flag calculation.
4. Store usage events in memory or initial storage abstraction.
5. Add billing plan model.
6. Add billing summary aggregation.
7. Add tests for overage calculation.

Sprint done criteria:

- Successful request emits usage event.
- Rejected request emits non-billable usage event.
- Timeout event can be marked billable when upstream was called.
- Billing summary calculates overage.
- `go test ./...` passes.

### Sprint 10: Control Plane MVP

Goal: add admin APIs for managing runtime configuration.

Scope:

1. Add `cmd/control-plane`.
2. Add tenant APIs.
3. Add API product APIs.
4. Add route APIs.
5. Add upstream APIs.
6. Add credential APIs.
7. Add transformation template APIs.
8. Add audit event writes for admin changes.

Sprint done criteria:

- Tenant can be created through API.
- Route can be created through API.
- Credential can be created and returned once.
- Admin changes write audit events.
- `go test ./...` passes.

### Sprint 11: PostgreSQL Storage

Goal: replace in-memory configuration with PostgreSQL-backed repositories.

Scope:

1. Add migration tool.
2. Add initial migrations.
3. Add repository interfaces.
4. Add PostgreSQL repository implementations.
5. Add tenant-scoped query helpers.
6. Add integration tests for repositories.

Sprint done criteria:

- Control plane persists tenants, routes, credentials, templates, and upstreams.
- Tenant-scoped queries are enforced.
- Repository tests pass against PostgreSQL.
- `go test ./...` passes.

### Sprint 12: Config Reload and Observability

Goal: make gateway configuration reloadable and observable.

Scope:

1. Add runtime config snapshot.
2. Add config version tracking.
3. Add gateway config reload.
4. Keep last known good config.
5. Add structured JSON logs.
6. Add Prometheus metrics.
7. Add trace hooks.

Sprint done criteria:

- Gateway reloads config without restart.
- Invalid config does not replace last known good config.
- Metrics endpoint exposes gateway metrics.
- Logs include request ID, tenant ID, route ID, and protocol.
- `go test ./...` passes.

### Sprint 13: SOAP/XML Proof Adapter

Goal: prove the gateway is not limited to REST and ISO8583.

Scope:

1. Add SOAP/XML adapter config.
2. Add SOAP envelope generation.
3. Add XML response extraction.
4. Add REST to SOAP/XML transformation.
5. Add mock SOAP upstream integration test.

Sprint done criteria:

- REST request can call SOAP/XML upstream.
- SOAP/XML response maps back to REST JSON.
- SOAP/XML route uses the same auth, policy, billing, and observability pipeline.
- `go test ./...` passes.

### Sprint 14: Control Plane to Gateway Config Sync

Goal: make the locally running control plane and gateway work together for end-to-end manual demos.

Scope:

1. Add a PostgreSQL-backed runtime config loader for the gateway.
2. Convert active tenants, API products, routes, upstreams, credentials, templates, policies, and adapter configs into runtime config snapshots.
3. Wire gateway startup to load an initial snapshot from storage when `DATABASE_URL` is configured.
4. Wire periodic reload to the PostgreSQL config source.
5. Increment config version when admin-managed runtime resources change.
6. Keep last-known-good behavior when database config is incomplete or invalid.
7. Add integration tests that create config through repositories and execute a gateway request after reload.
8. Update local run documentation with a real control plane to gateway flow.

Sprint done criteria:

- Admin-created active routes can be loaded by the gateway without hard-coded test config.
- Published templates are required for routes that reference transformations.
- Disabled routes, upstreams, credentials, and tenants are excluded or rejected.
- Bad database config does not replace the current gateway snapshot.
- `go test ./...` passes.

### Sprint 15: Redis Runtime State Foundation

Goal: introduce Redis as the shared runtime state store before features depend on it.

Scope:

1. Add `github.com/redis/go-redis/v9`.
2. Add an internal Redis client package using `REDIS_ADDR` and timeout configuration.
3. Add a small runtime state store interface for short-lived keys, counters, TTLs, and compare-and-set style operations where needed.
4. Add Redis-backed implementation for the runtime state store.
5. Add in-memory implementation for tests and single-process development.
6. Wire Redis readiness checks when Redis-backed features are enabled.
7. Add Redis connection metrics and structured logs.
8. Add testcontainers-go Redis integration tests.
9. Update local manual testing docs to explain when Redis is required.

Sprint done criteria:

- Redis-backed runtime state store can set, get, increment, expire, and delete tenant-scoped keys.
- In-memory runtime state store can be used by unit tests without Redis.
- Readiness reports Redis health only when Redis-backed runtime features are enabled.
- Redis keys are namespaced by environment, tenant, feature, and version where applicable.
- `go test ./...` passes.

### Sprint 16: Runtime Authorization and Security Hardening

Goal: enforce finance-grade runtime access rules beyond basic API key authentication.

Scope:

1. Enforce tenant status at runtime.
2. Enforce consumer status at runtime.
3. Enforce credential status, expiration, and scopes.
4. Add API product or route scope requirements.
5. Add optional HMAC request signature verification for selected routes.
6. Add replay protection primitives using nonce, timestamp, and the Redis runtime state store.
7. Add idempotency key handling for configured unsafe methods.
8. Strengthen sensitive data masking for logs, audit events, usage events, and errors.
9. Add tests for disabled tenants, disabled consumers, expired credentials, missing scopes, invalid signatures, replayed requests, and masking.

Sprint done criteria:

- Disabled tenant traffic is rejected.
- Revoked, suspended, or expired credentials cannot call routes.
- Missing required scopes return `403`.
- HMAC-protected routes reject invalid or replayed requests.
- Sensitive request and response payload values do not appear in logs, audit records, billing records, or error responses.
- `go test ./...` passes.

### Sprint 17: Billing Admin APIs and Usage Reporting

Goal: make usage-based billing inspectable through admin APIs.

Scope:

1. Add billing plan CRUD APIs.
2. Add usage event query APIs with tenant, route, consumer, status, and time filters.
3. Add billing summary query APIs by tenant and billing period.
4. Add billing summary generation endpoint or worker command.
5. Add overage and billable unit breakdowns to billing summary responses.
6. Add CSV or JSON export for usage summaries.
7. Add pagination for usage event reads.
8. Add audit events for billing plan and billing summary admin actions.
9. Add tests for billing APIs, tenant isolation, pagination, overage calculation, and audit behavior.

Sprint done criteria:

- Admins can create and update billing plans.
- Admins can query usage events without seeing sensitive payload data.
- Billing summaries can be generated and read by period.
- Overage calculation is visible in API responses.
- Tenant-scoped usage queries cannot leak another tenant's events.
- `go test ./...` passes.

### Sprint 18: Admin Audit, RBAC, and Operator APIs

Goal: make the control plane safer for multiple operators.

Scope:

1. Replace the single development admin token with an admin identity abstraction.
2. Add platform admin and tenant admin roles.
3. Add role checks for tenant-scoped and platform-scoped admin APIs.
4. Add audit log read APIs with tenant, actor, action, resource, and time filters.
5. Add immutable audit event storage behavior.
6. Add admin API key or bootstrap admin credential support.
7. Add tests for role authorization, audit log reads, and denied admin actions.

Sprint done criteria:

- Platform admins can manage all tenants.
- Tenant admins can manage only their assigned tenant.
- Denied admin actions do not mutate state.
- Audit logs can be queried without exposing secrets.
- Audit records cannot be updated or deleted through repositories.
- `go test ./...` passes.

### Sprint 19: Policy Persistence and Distributed Enforcement

Goal: make policies durable and usable across multiple gateway instances.

Scope:

1. Add rate limit policy CRUD APIs.
2. Add quota policy CRUD APIs.
3. Persist policy assignments on routes or API products.
4. Add Redis-backed rate limiter implementation using the runtime state store.
5. Add Redis-backed quota counter implementation using the runtime state store.
6. Add fixed-window or sliding-window behavior based on policy configuration.
7. Add tests for policy persistence, distributed counters, route-level policies, API-product-level policies, and Redis fallback behavior.

Sprint done criteria:

- Admin-created policies are loaded into gateway runtime config.
- Rate limit behavior is consistent across gateway instances using Redis.
- Quota counters are tenant-scoped and period-aware.
- Gateway behavior is explicit when Redis is unavailable.
- `go test ./...` passes.

### Sprint 20: Advanced Adapter Expansion

Goal: add the next set of protocol proofs without weakening the adapter architecture.

Scope:

1. Add gRPC outbound proof adapter.
2. Add GraphQL facade proof adapter.
3. Add webhook outbound delivery proof.
4. Add file-based ingestion or delivery proof.
5. Keep all protocol-specific behavior behind adapter interfaces.
6. Add route configuration validation for each new adapter type.
7. Add integration tests that verify auth, policy, transformation, billing, and observability still run through the shared pipeline.

Sprint done criteria:

- At least two new adapter proofs work through the same gateway route execution model.
- Adapter-specific configs are validated before publish or reload.
- Failed adapter calls emit usage events and metrics.
- No protocol-specific shortcut bypasses auth, policy, billing, or observability.
- `go test ./...` passes.

### Sprint 21: Production Readiness and Deployment

Goal: prepare the MVP for realistic deployment and operations.

Scope:

1. Add Dockerfiles for gateway, control plane, and optional workers.
2. Add production-oriented compose file for local multi-service testing.
3. Add Kubernetes manifests or Helm chart skeleton.
4. Add migration command or startup migration documentation.
5. Add readiness checks for PostgreSQL, Redis, and config load status.
6. Add graceful shutdown tests for in-flight requests.
7. Add load test scripts for REST and REST to ISO8583 paths.
8. Add runbooks for local development, manual testing, migrations, and troubleshooting.

Sprint done criteria:

- Services can be built into containers.
- Local compose can run gateway, control plane, PostgreSQL, and Redis together.
- Readiness accurately reports dependency and config status.
- Operators have documented commands for migrations, startup, shutdown, and troubleshooting.
- `go test ./...` passes.

## 21. Build Order Summary

Recommended order:

1. Gateway foundation.
2. Tenant-aware routing.
3. Authentication.
4. REST proxy.
5. Protocol adapter interface.
6. Canonical message model.
7. Transformation engine.
8. ISO8583 adapter.
9. Shared policy engine.
10. Billing metering.
11. Control plane API.
12. PostgreSQL storage.
13. Config reload.
14. Observability and audit.
15. SOAP/XML proof adapter.
16. Control plane to gateway config sync.
17. Redis runtime state foundation.
18. Runtime authorization and security hardening.
19. Billing admin APIs and usage reporting.
20. Admin audit, RBAC, and operator APIs.
21. Policy persistence and distributed enforcement.
22. Advanced adapter expansion.
23. Production readiness and deployment.

## 22. Definition of MVP Complete

The MVP is complete when:

- A platform admin can create a tenant.
- A tenant can create credentials.
- A tenant can configure an API product and route.
- REST to REST routing works.
- REST to ISO8583 transformation works.
- ISO8583 to REST transformation works.
- At least one additional adapter proof works, preferably REST to SOAP/XML.
- Tenant isolation is enforced.
- Rate limits and quotas are enforced.
- Usage events are recorded.
- Billing summaries are generated.
- Sensitive fields are masked.
- Audit logs are written for admin changes.
- Gateway config can reload without restart.
- Core integration tests pass.
