# Technical Design: Go-Based Multitenant API Gateway

## 1. Decision: Build the Gateway in Go

Yes, the API gateway can be built with Go.

Go is a strong fit because an API gateway needs high-concurrency networking, predictable performance, simple deployment, and good support for HTTP, TCP, TLS, observability, and background workers.

This product should not depend on gateway platforms such as Kong, APISIX, Tyk, KrakenD, or Envoy as the main runtime. The gateway runtime, routing, protocol adapters, policy execution, transformation, billing metering, and tenant isolation should be implemented as first-party Go services.

Allowed dependencies should be normal libraries, not external gateway products. For example:

- HTTP router library.
- PostgreSQL driver.
- Redis client.
- ISO8583 parsing library, if it is evaluated and wrapped behind an internal interface.
- XML, gRPC, GraphQL, message queue, or file parsing libraries when wrapped behind internal adapter interfaces.
- OpenTelemetry SDK.
- JWT/OAuth2 helper library.
- Structured logging library.

## 2. Document Category

This file is the technical design.

Related documents:

- `PRODUCT_DESIGN.md`: product goals, MVP scope, target users, and business requirements.
- `TECHNICAL_DESIGN.md`: Go architecture, gateway components, runtime behavior, data model, and implementation boundaries.

## 3. System Overview

The system should be split into a data plane and a control plane.

The data plane handles live traffic. It must be fast, highly available, and horizontally scalable.

The control plane manages configuration. It can be slower than the data plane but must be safe, auditable, and tenant-aware.

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
                              v
                   +----------------------+
                   | Config Store         |
                   | PostgreSQL / Redis   |
                   +----------+-----------+
                              |
                   config sync / polling / events
                              |
                              v
+-----------------------------+-----------------------------+
|                      Gateway Data Plane                   |
|                                                           |
|  Listener -> Tenant Resolver -> Auth -> Policy -> Router  |
|        -> Protocol Adapter -> Transformer -> Upstream     |
|        -> Response Adapter -> Metrics / Audit / Billing   |
+-----------------------------+-----------------------------+
                              |
              +---------------+----------------+
              |                                |
              v                                v
 REST / SOAP / gRPC / MQ / File     ISO8583 / TCP Upstreams
```

## 4. Core Gateway Capabilities

The gateway should have these capabilities before it is considered production-ready for finance use.

### 4.1 Listener Layer

Required:

- Public HTTP and HTTPS listener.
- Optional private HTTP listener for internal traffic.
- TCP listener for inbound ISO8583 messages.
- TCP listener framework for proprietary financial protocols.
- Optional webhook listener over HTTP.
- Optional message queue consumer worker.
- Optional file ingestion worker for batch channels.
- Health check endpoint.
- Read, write, and idle timeouts.
- Graceful shutdown.
- Request body size limit.
- Connection limit.

Recommended Go packages:

- Standard `net/http` for HTTP server.
- Standard `net` for TCP ISO8583 listener.
- `crypto/tls` for TLS configuration.

### 4.2 Protocol Adapter Layer

The gateway should be protocol-neutral internally. REST and ISO8583 are initial adapters, but the runtime should support adding other adapters without rewriting tenant resolution, policies, billing, or observability.

Adapter categories:

- HTTP/REST adapter.
- ISO8583 TCP adapter.
- SOAP/XML adapter.
- gRPC adapter.
- GraphQL facade adapter.
- Webhook adapter.
- Message queue adapter.
- File/batch adapter.
- Proprietary TCP adapter.

Each adapter should do only protocol-specific work:

- Accept inbound protocol traffic.
- Decode native payloads.
- Encode native responses.
- Extract metadata needed for tenant resolution.
- Convert native payloads into the gateway canonical message.
- Convert canonical responses back into native payloads.

The adapter should not own billing, tenant permissions, rate limiting, audit logging, or shared transformation rules.

Suggested Go interface:

```go
type ProtocolAdapter interface {
    Name() string
    Decode(ctx context.Context, req InboundRequest) (CanonicalMessage, error)
    Encode(ctx context.Context, msg CanonicalMessage) (OutboundResponse, error)
}
```

For outbound calls:

```go
type UpstreamAdapter interface {
    Name() string
    Call(ctx context.Context, target UpstreamTarget, msg CanonicalMessage) (CanonicalMessage, error)
}
```

### 4.3 Canonical Message Model

The gateway should use a canonical internal message so transformations are not hard-coded as direct protocol-to-protocol pairs.

Example:

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

Benefits:

- REST to ISO8583 and ISO8583 to REST remain supported.
- SOAP to REST can reuse the same policy, billing, and transformation engine.
- gRPC to REST can be added as an adapter.
- File records can become canonical messages and reuse existing routing and billing.
- Tenant isolation remains consistent across protocols.

### 4.4 Tenant Resolution

Every request must resolve to one tenant before route execution.

Resolution options:

- API key ownership.
- OAuth2 client ownership.
- Hostname or subdomain.
- `X-Tenant-ID` header for internal environments only.
- ISO8583 listener binding.
- SOAP client credential.
- gRPC metadata.
- Message queue topic or subscription binding.
- File drop location or batch profile.

Recommended MVP:

- Resolve tenant from API key for REST.
- Resolve tenant from listener or network profile for ISO8583.
- Resolve tenant from adapter-specific credential or binding for other protocols.
- Do not trust user-supplied tenant headers on public APIs unless the credential also belongs to the same tenant.

### 4.5 Authentication

Required:

- API key authentication.
- Credential hashing at rest.
- Credential rotation.
- Credential status: active, suspended, revoked.
- Per-tenant credential isolation.

Future:

- OAuth2 client credentials with JWT validation.
- mTLS per tenant or partner.
- HMAC request signing.
- Hardware security module integration for sensitive key operations.

### 4.6 Authorization

Required:

- Tenant-level access.
- Consumer application access.
- API product access.
- Route-level access.
- Admin role-based access control.

Example roles:

- `platform_admin`
- `tenant_admin`
- `api_operator`
- `billing_viewer`
- `developer`
- `auditor`

### 4.7 Routing Engine

Required:

- Match by method and path.
- Match by host.
- Match by tenant.
- Support REST upstream routes.
- Support ISO8583 upstream routes.
- Support SOAP/XML upstream routes.
- Support gRPC upstream routes.
- Support GraphQL facade routes.
- Support webhook target routes.
- Support message queue producer and consumer routes.
- Support file ingestion and file export routes.
- Support proprietary TCP routes through adapter configuration.
- Support route priority.
- Support route status: draft, active, deprecated, disabled.

Route example:

```yaml
tenantId: tenant_bank_a
apiProductId: card_authorization
inboundProtocol: rest
outboundProtocol: iso8583
method: POST
path: /cards/authorization
upstreamRef: switch_primary
transformationTemplateRef: card_authorization_rest_to_iso_v1
timeoutMs: 5000
```

Generic route example:

```yaml
tenantId: tenant_bank_a
apiProductId: account_inquiry
inboundProtocol: rest
outboundProtocol: soap_xml
method: POST
path: /accounts/inquiry
upstreamRef: core_banking_soap
transformationTemplateRef: account_inquiry_rest_to_soap_v1
timeoutMs: 5000
```

### 4.8 Policy Engine

Policies should be executed in a predictable order.

Recommended request pipeline:

1. Request ID.
2. Tenant resolution.
3. Authentication.
4. Authorization.
5. IP allowlist.
6. Request size validation.
7. Rate limit.
8. Quota check.
9. Schema validation.
10. Inbound protocol decoding.
11. Transformation.
12. Upstream protocol call.
13. Response transformation.
14. Outbound protocol encoding.
15. Audit event.
16. Metrics event.
17. Billing event.

MVP policies:

- API key authentication.
- IP allowlist.
- Rate limit.
- Monthly quota.
- Request body size limit.
- Sensitive log masking.

Post-MVP policies:

- mTLS.
- HMAC signature.
- JWT claim-based authorization.
- Payload schema validation.
- Custom tenant policy hooks.

### 4.9 Rate Limiting

Required:

- Tenant-level limit.
- Consumer-level limit.
- API-level limit.
- Route-level limit.
- Burst handling.

Recommended implementation:

- Token bucket or leaky bucket algorithm.
- Redis-backed distributed counters for multi-instance deployments.
- In-memory limiter only for local development or single-node deployment.

Rate limit key format:

```text
rate:{tenantId}:{consumerId}:{apiId}:{routeId}:{window}
```

### 4.10 Quotas

Rate limits protect runtime stability. Quotas protect commercial and contractual limits.

Required:

- Daily request quota.
- Monthly request quota.
- Optional success-only quota.
- Optional billable-only quota.
- Configurable behavior when exceeded.

Quota exceeded behavior:

- Reject request with `429`.
- Allow request but mark as overage.
- Allow request for grace period and notify tenant.

### 4.11 Transformation Engine

The transformation engine is a core product feature and should be owned by this codebase.

Required:

- REST to ISO8583.
- ISO8583 to REST.
- REST to REST.
- SOAP/XML to canonical message.
- Canonical message to SOAP/XML.
- gRPC to canonical message.
- Canonical message to gRPC.
- GraphQL operation to canonical message.
- Webhook payload to canonical message.
- Message queue event to canonical message.
- File record to canonical message.
- JSON field mapping.
- ISO8583 field mapping.
- XML path mapping.
- Protobuf field mapping.
- CSV and fixed-width field mapping.
- Template versioning.
- Template validation.
- Dry-run transformation testing.
- Sensitive data masking.
- Transformation error reporting.

The engine should be interface-driven:

```go
type Transformer interface {
    Transform(ctx context.Context, input TransformInput) (TransformOutput, error)
}
```

Recommended internal modules:

```text
internal/transform/
  engine.go
  template.go
  registry.go
  jsonpath.go
  xmlpath.go
  canonical.go
  masking.go
  errors.go

internal/iso8583/
  codec.go
  spec.go
  message.go
  packager.go
  unpacker.go

internal/protocol/
  adapter.go
  registry.go
  canonical.go
```

### 4.12 Protocol Support Roadmap

Protocol support should be delivered incrementally.

MVP adapters:

- REST inbound.
- REST outbound.
- ISO8583 inbound.
- ISO8583 outbound.

Near-term adapters:

- SOAP/XML outbound for legacy core banking systems.
- SOAP/XML inbound for partner compatibility.
- Webhook outbound for event notification.
- Message queue producer for asynchronous integration.

Later adapters:

- gRPC inbound and outbound.
- GraphQL facade over existing APIs.
- File ingestion for CSV and fixed-width batch processing.
- SFTP batch import and export.
- Proprietary TCP protocol SDK.

### 4.13 ISO8583 Support

ISO8583 is not one universal format in real finance systems. Different switches can use different field definitions, encodings, bitmap handling, length prefixes, and network headers.

MVP should support configurable ISO8583 profiles.

Profile configuration should include:

- MTI support.
- Field definitions.
- Field type: numeric, alphanumeric, binary, amount.
- Fixed or variable length.
- LLVAR and LLLVAR fields.
- BCD or ASCII encoding.
- Bitmap encoding.
- Network length header.
- Request and response MTI mapping.
- Response code field.

Example profile:

```yaml
name: default-iso8583-profile
encoding: ascii
lengthHeader:
  enabled: true
  sizeBytes: 2
  byteOrder: big_endian
fields:
  "2":
    name: pan
    type: numeric
    lengthType: llvar
    maxLength: 19
    sensitive: true
  "3":
    name: processing_code
    type: numeric
    lengthType: fixed
    length: 6
  "4":
    name: amount_transaction
    type: numeric
    lengthType: fixed
    length: 12
  "11":
    name: stan
    type: numeric
    lengthType: fixed
    length: 6
  "37":
    name: rrn
    type: alphanumeric
    lengthType: fixed
    length: 12
  "39":
    name: response_code
    type: alphanumeric
    lengthType: fixed
    length: 2
```

### 4.14 Upstream Clients

REST upstream client requirements:

- Configurable base URL.
- Per-route timeout.
- Retry policy for safe methods only by default.
- Circuit breaker.
- Connection pooling.
- TLS configuration.
- Header forwarding rules.

ISO8583 upstream client requirements:

- TCP connection management.
- Optional persistent connections.
- Connection pool.
- Length header handling.
- Request correlation using STAN, RRN, or configured fields.
- Timeout handling.
- Reversal or timeout event hooks for future payment flows.

SOAP/XML upstream client requirements:

- SOAP envelope generation.
- XML namespace handling.
- XML schema validation where required.
- Basic auth, mTLS, or WS-Security support as future enhancement.
- XML response parsing and fault mapping.

gRPC upstream client requirements:

- Protobuf descriptor or generated client support.
- Deadline propagation.
- Metadata forwarding rules.
- gRPC status mapping to gateway errors.

Message queue upstream requirements:

- Producer and consumer abstraction.
- At-least-once delivery awareness.
- Idempotency key support.
- Dead-letter queue support.
- Offset or acknowledgment tracking.

File upstream requirements:

- CSV and fixed-width parsing.
- Batch correlation ID.
- File checksum.
- Partial failure handling.
- Import and export status tracking.

### 4.15 Billing Metering

Billing must not depend on access logs alone. It should use structured usage events generated by the gateway runtime.

Required:

- Emit one usage event per request attempt.
- Mark whether the event is billable.
- Include tenant, consumer, API, route, transformation type, response status, and latency.
- Include source protocol and target protocol.
- Store raw usage events.
- Aggregate usage into billing summaries.
- Support replaying billing aggregation from raw events.

Usage event example:

```json
{
  "eventId": "evt_01HX000001",
  "tenantId": "tenant_bank_a",
  "consumerId": "mobile_app",
  "apiProductId": "card_authorization",
  "routeId": "route_auth_purchase",
  "direction": "rest_to_iso8583",
  "sourceProtocol": "rest",
  "targetProtocol": "iso8583",
  "status": "success",
  "httpStatus": 200,
  "upstreamStatus": "00",
  "latencyMs": 87,
  "billable": true,
  "occurredAt": "2026-05-08T10:30:00Z"
}
```

Recommended design:

- Gateway writes usage events to a durable queue or database outbox.
- Billing worker aggregates events by tenant and billing period.
- Billing summary can be regenerated from raw usage events.

### 4.16 Observability

Required:

- Structured JSON logs.
- Request ID.
- Correlation ID.
- Tenant ID in every log line.
- API ID and route ID in every log line.
- OpenTelemetry tracing.
- Prometheus-compatible metrics.
- Sensitive data masking.

Important metrics:

- Request count.
- Error count.
- Latency histogram.
- Upstream latency.
- Transformation latency.
- Rate limit rejects.
- Quota rejects.
- Authentication failures.
- ISO8583 timeout count.
- Protocol adapter error count.
- File batch success and failure count.
- Message queue delivery failure count.
- Billing event write failures.

### 4.17 Audit Logging

Audit logs are separate from request logs.

Audit events should be created for:

- Tenant created or updated.
- User invited or removed.
- Credential created, rotated, suspended, or revoked.
- API route created or updated.
- Transformation template created, published, or rolled back.
- Billing plan changed.
- Rate limit changed.
- Quota changed.
- ISO8583 profile changed.
- Protocol adapter configuration changed.
- File batch profile changed.
- Message queue binding changed.

Audit log fields:

```text
id
tenantId
actorUserId
actorRole
action
resourceType
resourceId
before
after
ipAddress
userAgent
occurredAt
```

### 4.18 Configuration Management

Gateway instances should not query PostgreSQL on every request.

Recommended design:

- Control plane writes configuration to PostgreSQL.
- Control plane publishes config version events.
- Gateway data plane loads active config into memory.
- Gateway refreshes config by polling or subscribing to change events.
- Gateway keeps last known good config if reload fails.

Configuration should be versioned:

- Tenant config version.
- Route config version.
- Transformation template version.
- ISO8583 profile version.
- Protocol adapter version.
- Canonical schema version.
- Billing plan version.

### 4.19 Admin API

Required endpoints:

```text
POST   /admin/v1/tenants
GET    /admin/v1/tenants
GET    /admin/v1/tenants/{tenantId}
PATCH  /admin/v1/tenants/{tenantId}

POST   /admin/v1/tenants/{tenantId}/api-products
GET    /admin/v1/tenants/{tenantId}/api-products
POST   /admin/v1/tenants/{tenantId}/routes
GET    /admin/v1/tenants/{tenantId}/routes
PATCH  /admin/v1/tenants/{tenantId}/routes/{routeId}

POST   /admin/v1/tenants/{tenantId}/consumers/{consumerId}/credentials
POST   /admin/v1/tenants/{tenantId}/credentials/{credentialId}/rotate
POST   /admin/v1/tenants/{tenantId}/credentials/{credentialId}/revoke

POST   /admin/v1/tenants/{tenantId}/transformation-templates
POST   /admin/v1/tenants/{tenantId}/transformation-templates/{templateId}/publish
POST   /admin/v1/tenants/{tenantId}/transformation-templates/{templateId}/test

POST   /admin/v1/tenants/{tenantId}/protocol-adapters
GET    /admin/v1/tenants/{tenantId}/protocol-adapters
PATCH  /admin/v1/tenants/{tenantId}/protocol-adapters/{adapterConfigId}

POST   /admin/v1/tenants/{tenantId}/iso8583-profiles
GET    /admin/v1/tenants/{tenantId}/usage
GET    /admin/v1/tenants/{tenantId}/billing-summaries/{billingPeriod}
GET    /admin/v1/tenants/{tenantId}/audit-logs
```

## 5. Suggested Go Project Structure

```text
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
    billing/

  gateway/
    listener/
    middleware/
    router/
    policy/
    protocol/
    upstream/
    response/

  tenant/
  auth/
  ratelimit/
  quota/
  transform/
  iso8583/
  protocol/
  billing/
  audit/
  config/
  observability/
  storage/

pkg/
  sdk/
```

Package responsibilities:

- `cmd/gateway`: starts the data plane.
- `cmd/control-plane`: starts admin APIs.
- `cmd/billing-worker`: aggregates billing usage.
- `internal/gateway`: request pipeline and runtime traffic handling.
- `internal/transform`: template execution and transformation rules.
- `internal/iso8583`: ISO8583 encode and decode support.
- `internal/protocol`: adapter interfaces, registry, and canonical message model.
- `internal/config`: config loading, caching, and versioning.
- `internal/billing`: usage event and billing aggregation.
- `internal/storage`: database repositories.

## 6. Runtime Request Flow

REST to ISO8583 flow:

```text
HTTP request
  -> assign request ID
  -> resolve tenant from credential
  -> authenticate
  -> authorize route
  -> check IP allowlist
  -> apply rate limit
  -> check quota
  -> validate JSON request
  -> transform JSON to ISO8583
  -> send TCP message to ISO8583 upstream
  -> receive ISO8583 response
  -> transform ISO8583 response to JSON
  -> return HTTP response
  -> emit metrics
  -> emit audit if needed
  -> emit billing usage event
```

ISO8583 to REST flow:

```text
TCP message
  -> parse length header
  -> decode ISO8583
  -> resolve tenant from listener profile
  -> validate allowed MTI
  -> apply rate limit
  -> transform ISO8583 to JSON
  -> call REST upstream
  -> transform JSON response to ISO8583
  -> write TCP response
  -> emit metrics
  -> emit billing usage event
```

SOAP/XML to REST flow:

```text
HTTP SOAP request
  -> assign request ID
  -> resolve tenant from credential, host, or SOAP profile
  -> authenticate
  -> authorize route
  -> decode SOAP envelope to canonical message
  -> transform canonical message to REST JSON
  -> call REST upstream
  -> transform REST response to canonical message
  -> encode SOAP response or SOAP fault
  -> emit metrics and billing usage event
```

Message queue to REST flow:

```text
Queue message
  -> resolve tenant from queue binding
  -> decode event payload to canonical message
  -> apply policy and quota
  -> transform canonical message to REST JSON
  -> call REST upstream
  -> acknowledge or retry message based on result
  -> emit metrics and billing usage event
```

## 7. Storage Design

Recommended MVP storage:

- PostgreSQL for durable configuration, tenants, credentials metadata, audit logs, billing plans, usage events, and billing summaries.
- Redis for rate limiting, distributed locks, short-lived counters, and config cache acceleration.

Tables:

```text
tenants
users
tenant_users
api_products
routes
upstreams
protocol_adapter_configs
iso8583_profiles
transformation_templates
credentials
rate_limit_policies
quota_policies
billing_plans
usage_events
billing_summaries
audit_logs
config_versions
```

Important database rules:

- Every tenant-owned table must include `tenant_id`.
- Every query in tenant scope must filter by `tenant_id`.
- Unique constraints should usually include `tenant_id`.
- Secrets should not be stored directly in plain text.
- API keys should be hashed, not encrypted.

## 8. Security Requirements

Required for MVP:

- TLS for public APIs.
- Strong secret generation.
- API key hashing.
- Role-based access control.
- Tenant-scoped authorization checks.
- IP allowlist.
- PAN masking in logs.
- No CVV logging.
- No PIN block logging.
- Audit log for admin changes.
- Secure headers for admin portal APIs.

Recommended before production:

- mTLS for partner connections.
- HSM or KMS integration.
- Secret manager integration.
- Vulnerability scanning.
- Dependency scanning.
- Static analysis in CI.
- Security review of ISO8583 field handling.

## 9. Failure Handling

The gateway must fail predictably.

Failure cases:

- Unknown tenant: return `401` or reject TCP message.
- Invalid credential: return `401`.
- Unauthorized route: return `403`.
- Rate limit exceeded: return `429`.
- Quota exceeded: return `429` or allow as overage based on policy.
- Transformation error: return `400` if client input is invalid, `500` if template/runtime error.
- REST upstream timeout: return `504`.
- ISO8583 upstream timeout: return mapped response, usually response code `91` or tenant-configured equivalent.
- SOAP upstream fault: map to configured HTTP or SOAP fault response.
- gRPC upstream error: map gRPC status to configured gateway response.
- Message queue delivery failure: retry, then dead-letter based on policy.
- File batch validation failure: mark record or batch failed based on tenant policy.
- Billing event write failure: request should not fail, but event must be retried through durable outbox.

## 10. MVP Build Order

Recommended implementation order:

1. Create Go module and service skeleton.
2. Build HTTP gateway listener.
3. Build tenant and route config model.
4. Build API key authentication.
5. Build in-memory route matching.
6. Build REST upstream proxying.
7. Build protocol adapter interfaces and canonical message model.
8. Add REST-to-REST transformation or pass-through using the canonical model.
9. Build REST-to-ISO8583 transformation.
10. Build ISO8583 encode/decode profile support.
11. Build ISO8583-to-REST transformation.
12. Add shared policy execution and in-memory rate limiting.
13. Add usage event metering.
14. Add admin API.
15. Add PostgreSQL storage for tenants, routes, credentials, templates, and upstreams.
16. Add config cache and reload.
17. Add observability and dashboards.
18. Add billing worker and billing summaries.
19. Add audit logs.
20. Add integration tests with mock REST and ISO8583 upstreams.
21. Add one non-REST, non-ISO8583 adapter as proof of extensibility, preferably REST to SOAP/XML outbound.

## 11. Testing Strategy

Required tests:

- Unit tests for route matching.
- Unit tests for tenant resolution.
- Unit tests for API key hashing and validation.
- Unit tests for rate limit key generation.
- Unit tests for ISO8583 packing and unpacking.
- Unit tests for transformation templates.
- Integration tests for REST to REST route.
- Integration tests for REST to ISO8583 route.
- Integration tests for ISO8583 to REST route.
- Integration tests for one additional adapter route.
- Integration tests for billing event generation.
- Integration tests for quota exceeded behavior.

Finance-specific test cases:

- PAN masking.
- Missing required ISO8583 field.
- Invalid amount format.
- Unsupported MTI.
- Upstream timeout.
- Duplicate STAN behavior.
- Response code mapping.
- Tenant isolation between two tenants using similar route paths.

## 12. Production Readiness Checklist

Before production, the gateway should have:

- Horizontal scaling.
- Graceful shutdown.
- Config reload without restart.
- Backpressure controls.
- Distributed rate limiting.
- Durable billing event delivery.
- Audit logging.
- Trace correlation.
- Metrics dashboards.
- Alert rules.
- Tenant data isolation tests.
- Secret rotation process.
- Load test results.
- Disaster recovery plan.
- Database backup and restore process.
- Runbook for ISO8583 upstream outage.

## 13. What Not to Build in MVP

Avoid these until the core gateway is stable:

- Visual drag-and-drop transformation builder.
- API marketplace.
- Advanced payment orchestration.
- Fraud scoring.
- Automated invoice payment.
- Multi-region active-active deployment.
- Custom scripting inside the gateway runtime.
- Plugin marketplace.

Custom runtime scripting is especially risky in finance workloads because it complicates isolation, performance, security review, and auditability.
