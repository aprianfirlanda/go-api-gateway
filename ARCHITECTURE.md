# Architecture: Go-Based Multitenant API Gateway

## 1. Purpose

This document describes the system architecture for the Go-based multitenant API gateway.

The architecture supports:

- Finance-grade API gateway runtime.
- Multitenant control plane.
- Protocol adapter model.
- REST, ISO8583, SOAP/XML, and future protocols.
- Transformation engine.
- Usage metering and billing.
- Audit logging.
- Secure tenant isolation.

## 2. Architecture Summary

The system is split into two major planes:

- Data plane: handles live API and protocol traffic.
- Control plane: manages configuration, tenants, routes, credentials, templates, billing plans, and audit logs.

```text
                        +----------------------+
                        | Admin / Dev Portal   |
                        +----------+-----------+
                                   |
                                   v
                        +----------------------+
                        | Control Plane API    |
                        +----------+-----------+
                                   |
                  +----------------+----------------+
                  |                                 |
                  v                                 v
          +---------------+                 +---------------+
          | PostgreSQL    |                 | Secret Store   |
          +-------+-------+                 +---------------+
                  |
                  v
          +---------------+
          | Config Cache  |
          | Redis         |
          +-------+-------+
                  |
          config snapshots
                  |
                  v
+-----------------+-------------------------------------------+
|                    Gateway Data Plane                       |
|                                                             |
| Listener -> Tenant Resolver -> Auth -> Policy -> Router     |
|    -> Inbound Adapter -> Transform -> Upstream Adapter      |
|    -> Response Transform -> Outbound Adapter -> Response    |
|    -> Metrics -> Audit -> Usage Event                       |
+-----------------+-------------------------------------------+
                  |
       +----------+----------+----------------+
       |                     |                |
       v                     v                v
 REST / SOAP / gRPC     ISO8583 / TCP     Queue / File
 Backend Services       Switches          Integrations
```

## 3. Main Components

### 3.1 Gateway Data Plane

The data plane handles live traffic.

Responsibilities:

- Accept inbound traffic.
- Resolve tenant.
- Authenticate credentials.
- Authorize consumer access.
- Apply policies.
- Match routes.
- Decode inbound protocol.
- Transform request.
- Call upstream.
- Transform response.
- Encode outbound response.
- Emit metrics.
- Emit usage events.

The data plane must avoid database queries in the hot request path whenever possible.

### 3.2 Control Plane API

The control plane manages configuration.

Responsibilities:

- Tenant management.
- User and role management.
- API product management.
- Route management.
- Upstream management.
- Protocol adapter config management.
- ISO8583 profile management.
- Transformation template management.
- Credential management.
- Billing plan management.
- Runtime config publishing.
- Audit logging.

### 3.3 Billing Worker

The billing worker aggregates usage events.

Responsibilities:

- Read raw usage events.
- Aggregate usage by tenant and billing period.
- Apply pricing plans.
- Generate billing summaries.
- Support recalculation.
- Support exports.

### 3.4 Config Publisher

The config publisher creates validated runtime snapshots.

Responsibilities:

- Load tenant configuration from PostgreSQL.
- Validate route references.
- Validate published templates.
- Validate adapter configs.
- Build runtime config snapshot.
- Write config version.
- Publish config change event.

### 3.5 Protocol Adapters

Protocol adapters isolate protocol-specific behavior.

Adapters:

- REST.
- ISO8583.
- SOAP/XML.
- gRPC.
- GraphQL facade.
- Webhook.
- Message queue.
- File/batch.
- Proprietary TCP.

Adapter responsibilities:

- Decode native payload into canonical message.
- Encode canonical message into native response.
- Call upstream protocol where applicable.
- Handle protocol-specific timeouts and errors.

Adapters must not own tenant authorization, billing, or shared policy logic.

### 3.6 Transformation Engine

The transformation engine maps canonical messages.

Responsibilities:

- Execute transformation templates.
- Resolve field paths.
- Execute approved built-in functions.
- Validate templates.
- Run dry-run tests.
- Mask sensitive values.

### 3.7 Storage Layer

The storage layer abstracts PostgreSQL and Redis access.

Responsibilities:

- Tenant-scoped repositories.
- Transaction handling.
- Config reads and writes.
- Usage event writes.
- Billing aggregation reads.
- Audit writes.

## 4. Runtime Request Flow

### 4.1 REST to ISO8583

```text
HTTP request
  -> assign request ID
  -> authenticate API key or JWT
  -> resolve tenant
  -> authorize consumer
  -> match route
  -> apply rate limit and quota
  -> decode REST payload to canonical message
  -> transform canonical message to ISO8583 canonical shape
  -> encode ISO8583 message
  -> send TCP request to switch
  -> receive ISO8583 response
  -> decode ISO8583 response
  -> transform response to REST canonical shape
  -> encode JSON response
  -> emit metrics
  -> emit usage event
```

### 4.2 ISO8583 to REST

```text
TCP message
  -> parse length header
  -> decode ISO8583 message
  -> resolve tenant from listener profile
  -> match route
  -> apply policy
  -> convert ISO8583 to canonical message
  -> transform canonical message to REST shape
  -> call REST upstream
  -> transform REST response to ISO8583 shape
  -> encode ISO8583 response
  -> write TCP response
  -> emit metrics
  -> emit usage event
```

### 4.3 REST to SOAP/XML

```text
HTTP request
  -> authenticate and resolve tenant
  -> match REST route
  -> decode JSON to canonical message
  -> transform canonical message to SOAP request shape
  -> encode SOAP envelope
  -> call SOAP upstream
  -> parse SOAP response
  -> transform XML response to REST canonical shape
  -> return JSON response
  -> emit metrics
  -> emit usage event
```

## 5. Control Plane Flow

Example: publish a new REST-to-ISO8583 route.

```text
Admin creates upstream
  -> Admin creates ISO8583 profile
  -> Admin creates transformation template
  -> Admin dry-runs template
  -> Admin publishes template
  -> Admin creates route
  -> Admin publishes route
  -> Control plane validates config
  -> Config publisher creates snapshot
  -> Gateway reloads snapshot
```

## 6. Config Architecture

The gateway should not query PostgreSQL for every request.

Recommended config flow:

```text
PostgreSQL
  -> Control Plane validation
  -> Runtime config snapshot
  -> Config version
  -> Redis/cache or direct polling
  -> Gateway in-memory config
```

Rules:

- Runtime config must be immutable after loaded.
- Gateway should keep last known good config.
- Bad config snapshots must be rejected.
- Routes must reference published templates only.
- Active routes must reference active upstreams.
- Tenant status must be included in runtime config.

## 7. Data Plane Runtime State

Gateway instances should keep these in memory:

- Active tenants.
- Active credentials or credential lookup cache.
- Active routes.
- Active upstreams.
- Published transformation templates.
- ISO8583 profiles.
- Protocol adapter configs.
- Rate limit and quota policy references.

Redis or another shared store should be used for:

- Distributed rate limiting.
- Quota counters.
- Nonce replay protection.
- Idempotency records.
- Optional config cache.

## 8. Storage Architecture

Primary storage:

- PostgreSQL.

Supporting storage:

- Redis for runtime counters and short-lived state.
- Secret manager for sensitive secrets.
- Optional durable queue for usage events.

PostgreSQL stores:

- Tenants.
- Users and roles.
- API products.
- Routes.
- Upstreams.
- Adapter configs.
- ISO8583 profiles.
- Transformation templates.
- Credentials metadata.
- Billing plans.
- Usage events.
- Billing summaries.
- Audit logs.
- Config versions.

## 9. Deployment Architecture

MVP deployment:

```text
gateway service
control-plane service
billing-worker service
PostgreSQL
Redis
```

Production deployment:

```text
multiple gateway replicas
multiple control-plane replicas
billing-worker replicas
PostgreSQL primary + replica
Redis cluster or managed Redis
secret manager
metrics stack
log aggregation
distributed tracing
```

The gateway data plane should scale horizontally.

## 10. Network Boundaries

Public zone:

- Gateway public API listeners.

Private application zone:

- Control plane API.
- Billing worker.
- Internal service APIs.

Data zone:

- PostgreSQL.
- Redis.
- Secret manager.

Partner connectivity zone:

- ISO8583 TCP links.
- SOAP/XML partner links.
- mTLS partner HTTP links.
- SFTP or file exchange links.

## 11. Scalability

Scale data plane by adding gateway replicas.

Scalability requirements:

- Stateless request handling where possible.
- Shared Redis for rate limits and quotas.
- Shared config version source.
- Connection pooling for REST upstreams.
- Managed connection pools for ISO8583 upstreams.
- Async billing aggregation.

Potential bottlenecks:

- ISO8583 persistent connection pools.
- Redis rate limit counters.
- Usage event writes.
- Transformation CPU cost.
- XML parsing.
- Billing aggregation over large event volume.

## 12. Availability

Gateway availability requirements:

- Gateway should continue serving with last known good config if control plane is unavailable.
- Gateway should fail closed for authentication and authorization.
- Gateway should fail predictably for upstream timeouts.
- Billing event failure should not block customer response, but must trigger durable retry and alert.

Control plane availability:

- Control plane outage should not stop existing gateway traffic.
- Control plane outage prevents config changes until recovered.

## 13. Failure Modes

### 13.1 PostgreSQL Unavailable

Impact:

- Control plane cannot persist changes.
- Billing worker cannot aggregate.
- Gateway may continue if it already has config.

Expected behavior:

- Gateway uses last known good config.
- Admin APIs return service unavailable.
- Billing worker retries.

### 13.2 Redis Unavailable

Impact:

- Distributed rate limiting affected.
- Quota counters affected.
- Replay protection affected.

Expected behavior:

- Fail closed or degraded based on tenant policy.
- For finance-critical APIs, prefer fail closed.
- Emit alert.

### 13.3 Upstream REST Unavailable

Expected behavior:

- Return configured timeout or upstream error.
- Emit usage event.
- Emit metric.
- Circuit breaker may open.

### 13.4 ISO8583 Switch Unavailable

Expected behavior:

- Return mapped timeout response.
- Common response code may be `91` depending on tenant config.
- Emit timeout metric.
- Emit usage event if request was sent.

### 13.5 Config Snapshot Invalid

Expected behavior:

- Reject new snapshot.
- Keep last known good config.
- Emit alert.
- Write audit or config error event.

## 14. Observability Architecture

Logs:

- Structured JSON.
- Include request ID, tenant ID, route ID, protocol, status, latency.
- Mask sensitive values.

Metrics:

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

Traces:

- Gateway request span.
- Authentication span.
- Route match span.
- Transformation span.
- Upstream call span.
- Billing event span.

## 15. Security Architecture

Security controls:

- API key hashing.
- OAuth2/JWT validation.
- Tenant-scoped authorization.
- RBAC for admin APIs.
- TLS for public APIs.
- mTLS for selected partners.
- HMAC request signing for selected partners.
- Secret manager integration.
- Sensitive data masking.
- Audit logging.

Cross-tenant protection must be tested at:

- Repository layer.
- Control plane API layer.
- Gateway route matching layer.
- Billing aggregation layer.

## 16. Billing Architecture

Billing flow:

```text
Gateway request
  -> usage event
  -> usage_events storage
  -> billing worker aggregation
  -> billing summary
  -> export
```

Rules:

- Usage events are append-only.
- Billing summaries are derived data.
- Draft summaries can be recalculated.
- Finalized summaries require explicit audit-tracked recalculation.
- Billing records must not contain sensitive payload data.

## 17. Package Architecture

Recommended Go package boundaries:

```text
internal/app/gateway
internal/app/controlplane
internal/app/billingworker

internal/gateway/listener
internal/gateway/router
internal/gateway/policy
internal/gateway/upstream

internal/protocol
internal/protocol/rest
internal/protocol/iso8583
internal/protocol/soap

internal/transform
internal/auth
internal/tenant
internal/billing
internal/audit
internal/config
internal/storage
internal/observability
```

Dependency rule:

- App packages compose dependencies.
- Protocol packages should not depend on control plane packages.
- Transformation should not depend on HTTP server details.
- Billing should depend on usage events, not logs.
- Storage repositories should enforce tenant-scoped access.

## 18. MVP Architecture Scope

MVP must include:

- Gateway data plane service.
- Control plane API service.
- Billing worker service.
- PostgreSQL.
- Redis.
- REST adapter.
- ISO8583 adapter.
- Protocol adapter interface.
- Canonical message model.
- Transformation engine.
- Usage event metering.
- Audit logging.

MVP should prove:

- REST to REST route.
- REST to ISO8583 route.
- ISO8583 to REST route.
- One additional adapter proof, preferably REST to SOAP/XML.
- Tenant isolation.
- Billing summary generation.

## 19. Future Architecture Enhancements

Future enhancements:

- Multi-region deployment.
- Dedicated tenant runtime.
- Physical tenant isolation.
- Event streaming for usage events.
- API marketplace.
- Advanced developer portal.
- HSM integration.
- Full mTLS partner management.
- GraphQL facade.
- gRPC adapter.
- File and SFTP adapter.
- Queue adapters.
- Policy plugin system with strong sandboxing.

## 20. Open Decisions

Open architecture decisions:

- Whether config updates use polling, Redis pub/sub, or durable event stream.
- Whether usage events go directly to PostgreSQL or through a queue.
- Whether Redis outage should fail open or fail closed per policy.
- Whether dedicated tenants get separate gateway deployments.
- Whether transformation templates are stored only as JSONB or also as YAML.
- Whether ISO8583 connection pools are shared per tenant or per route.

