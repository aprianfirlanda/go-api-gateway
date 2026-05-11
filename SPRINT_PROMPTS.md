# Sprint Prompts: From Start to MVP Complete

## 1. Purpose

This document gives copy-paste prompts for implementing the gateway sprint by sprint.

Use these prompts in order. Each prompt asks the coding agent to follow the design docs, make pragmatic implementation decisions, add tests, and run them.

## 2. General Prompt Rules

Use this rule for every sprint:

```text
Follow PRODUCT_DESIGN.md, TECHNICAL_DESIGN.md, ARCHITECTURE.md, IMPLEMENTATION_PLAN.md, DATA_MODEL.md, API_SPEC.md, TRANSFORMATION_DESIGN.md, SECURITY_DESIGN.md, BILLING_DESIGN.md, and TECHNOLOGY_DECISIONS.md.
```

When a sprint is complete, review the result, then continue to the next sprint.

Implementation location:

```text
Run all sprint implementation commands from syra-backend/.
Do not create root-level Go implementation files unless explicitly requested.
The repository root is for design and planning docs.
```

## 3. Sprint 1 Prompt

```text
Implement Sprint 1: Gateway Foundation from IMPLEMENTATION_PLAN.md.

Follow all existing design docs, especially TECHNOLOGY_DECISIONS.md.

Work inside syra-backend/.

Build the smallest working Go gateway service:
- initialize the Go module if needed
- create cmd/gateway
- add environment-based config loading
- use net/http, chi, and slog
- add graceful shutdown
- add /healthz
- add /readyz
- add request ID middleware
- add unit tests for health, readiness, and request ID behavior

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 2 yet.
```

## 4. Sprint 2 Prompt

```text
Implement Sprint 2: Tenant Routing and Authentication from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 1.
Work inside syra-backend/.

Add:
- tenant model
- API product model
- route model
- in-memory route registry
- route matching by tenant, host, method, and path
- API key credential model
- API key hashing and verification using argon2
- tenant and consumer resolution from API key
- unauthorized and forbidden responses
- tests for route matching, credential verification, and cross-tenant isolation

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 3 yet.
```

## 5. Sprint 3 Prompt

```text
Implement Sprint 3: REST Proxy from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 2.
Work inside syra-backend/.

Add:
- upstream model
- REST upstream client
- REST proxy handler
- allowed header forwarding
- hop-by-hop header stripping
- route timeout handling
- integration test with a mock REST upstream

Verify:
- authenticated request reaches upstream
- unauthorized request does not reach upstream
- timeout behavior works
- hop-by-hop headers are stripped

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 4 yet.
```

## 6. Sprint 4 Prompt

```text
Implement Sprint 4: Protocol Adapter Foundation from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 3.

Add:
- ProtocolAdapter interface
- UpstreamAdapter interface
- adapter registry
- canonical message model
- REST protocol adapter
- REST upstream adapter
- gateway route execution through adapter interfaces
- tests for adapter registry and canonical REST mapping

Keep REST to REST proxy behavior working.

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 5 yet.
```

## 7. Sprint 5 Prompt

```text
Implement Sprint 5: Transformation Engine MVP from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 4.

Add:
- transformation template model
- field mapping
- static values
- restricted expression evaluator
- built-in functions: formatAmount, currencyNumeric, nowMMddHHmmss, generateStan, maskPan
- template validation
- dry-run execution
- sensitive field masking
- tests for mapping, functions, validation, dry-run, and masking

Do not allow arbitrary scripting.

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 6 yet.
```

## 8. Sprint 6 Prompt

```text
Implement Sprint 6: ISO8583 Outbound from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 5.

Add:
- ISO8583 profile model
- internal ISO8583 codec interface
- packer for fixed, LLVAR, and LLLVAR fields
- bitmap handling
- length header handling
- ISO8583 TCP upstream client
- REST to ISO8583 transformation flow
- mock ISO8583 upstream integration test

Verify:
- REST request produces expected ISO8583 fields
- ISO8583 message respects profile definitions
- mock ISO8583 upstream receives message
- ISO8583 response maps back to REST JSON

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 7 yet.
```

## 9. Sprint 7 Prompt

```text
Implement Sprint 7: ISO8583 Inbound from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 6.

Add:
- TCP listener for ISO8583 inbound messages
- tenant resolution from listener profile
- ISO8583 request decode
- ISO8583 to REST transformation flow
- REST upstream call
- REST response to ISO8583 response mapping
- integration test for ISO8583 inbound flow
- malformed ISO8583 rejection test

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 8 yet.
```

## 10. Sprint 8 Prompt

```text
Implement Sprint 8: Policy Engine from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 7.

Add:
- shared policy pipeline
- IP allowlist policy
- request size limit policy
- rate limit interface
- in-memory rate limiter for MVP
- quota policy interface
- policy error mapping
- tests for policy order, blocked IP, oversized request, rate limit, and shared REST/ISO8583 pipeline behavior

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 9 yet.
```

## 11. Sprint 9 Prompt

```text
Implement Sprint 9: Billing Metering from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 8.

Add:
- usage event model
- usage event emission for every request attempt
- billable flag calculation
- initial in-memory usage event store or storage abstraction
- billing plan model
- billing summary aggregation
- tests for successful, rejected, failed, and timeout events
- tests for overage calculation

Ensure billing events do not include sensitive payload values.

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 10 yet.
```

## 12. Sprint 10 Prompt

```text
Implement Sprint 10: Control Plane MVP from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 9.

Add:
- cmd/control-plane
- admin API server using net/http and chi
- tenant APIs
- API product APIs
- route APIs
- upstream APIs
- credential APIs
- transformation template APIs
- audit event writes for admin changes
- tests for admin APIs and audit behavior

Follow API_SPEC.md and SECURITY_DESIGN.md.

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 11 yet.
```

## 13. Sprint 11 Prompt

```text
Implement Sprint 11: PostgreSQL Storage from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 10.

Use TECHNOLOGY_DECISIONS.md:
- PostgreSQL
- pgx
- goose migrations
- hand-written SQL with repository interfaces
- testcontainers-go for integration tests

Add:
- migrations directory
- initial SQL migrations based on DATA_MODEL.md
- repository interfaces
- PostgreSQL repository implementations
- tenant-scoped query helpers
- integration tests for core repositories

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 12 yet.
```

## 14. Sprint 12 Prompt

```text
Implement Sprint 12: Config Reload and Observability from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 11.

Add:
- runtime config snapshot model
- config version tracking
- gateway config reload
- last known good config behavior
- structured JSON logs with slog
- Prometheus metrics
- OpenTelemetry trace hooks where practical
- tests for config reload and invalid config rejection

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 13 yet.
```

## 15. Sprint 13 Prompt

```text
Implement Sprint 13: SOAP/XML Proof Adapter from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 12.

Add:
- SOAP/XML adapter config
- SOAP envelope generation using encoding/xml
- XML response extraction
- REST to SOAP/XML transformation flow
- mock SOAP upstream integration test

Verify:
- REST request calls SOAP/XML upstream
- SOAP/XML response maps back to REST JSON
- SOAP/XML route uses the same auth, policy, billing, and observability pipeline

Run go test ./... from inside syra-backend/ and fix failures.
```

## 16. Final MVP Verification Prompt

Use this after Sprint 13.

```text
Verify the MVP against IMPLEMENTATION_PLAN.md, PRODUCT_DESIGN.md, TECHNICAL_DESIGN.md, SECURITY_DESIGN.md, BILLING_DESIGN.md, and TRANSFORMATION_DESIGN.md.

Run the full test suite.
Identify missing MVP acceptance criteria.
Fix any gaps that are reasonable to complete now.
Produce a concise summary of what works, what remains, and how to run the gateway locally.
```

## 17. Sprint 14 Prompt

```text
Implement Sprint 14: Control Plane to Gateway Config Sync from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 13 and the MVP verification work.
Work inside syra-backend/.

Goal:
- Make the locally running control plane and gateway work together for end-to-end manual demos.

Add:
- PostgreSQL-backed runtime config loader for the gateway
- conversion from active control plane records into runtime config snapshots
- initial gateway snapshot load from storage when DATABASE_URL is configured
- periodic gateway reload from the PostgreSQL config source
- config version increment when admin-managed runtime resources change
- last-known-good behavior when database config is incomplete or invalid
- tests that create config through repositories and execute a gateway request after reload
- documentation updates for the local control plane to gateway flow

Verify:
- admin-created active routes can be loaded by the gateway
- published templates are required for routes that reference transformations
- disabled routes, upstreams, credentials, and tenants are excluded or rejected
- invalid database config does not replace the current runtime snapshot

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 15 yet.
```

## 18. Sprint 15 Prompt

```text
Implement Sprint 15: Redis Runtime State Foundation from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 14.
Work inside syra-backend/.

Use TECHNOLOGY_DECISIONS.md:
- Redis through github.com/redis/go-redis/v9
- testcontainers-go for Redis integration tests

Add:
- github.com/redis/go-redis/v9 dependency
- internal Redis client package using REDIS_ADDR and timeout configuration
- runtime state store interface for short-lived keys, counters, TTLs, and compare-and-set style operations where needed
- Redis-backed runtime state store implementation
- in-memory runtime state store implementation for tests and single-process development
- Redis readiness checks when Redis-backed features are enabled
- Redis connection metrics and structured logs
- testcontainers-go Redis integration tests
- manual testing documentation updates explaining when Redis is required

Verify:
- Redis-backed runtime state store can set, get, increment, expire, and delete tenant-scoped keys
- in-memory runtime state store can be used by unit tests without Redis
- readiness reports Redis health only when Redis-backed runtime features are enabled
- Redis keys are namespaced by environment, tenant, feature, and version where applicable

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 16 yet.
```

## 19. Sprint 16 Prompt

```text
Implement Sprint 16: Runtime Authorization and Security Hardening from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 15.
Work inside syra-backend/.

Follow SECURITY_DESIGN.md closely.

Add:
- runtime tenant status enforcement
- runtime consumer status enforcement
- credential status, expiration, and scope enforcement
- API product or route scope requirements
- optional HMAC request signature verification for selected routes
- replay protection primitives using nonce, timestamp, and the Redis runtime state store
- idempotency key handling for configured unsafe methods
- stronger sensitive data masking for logs, audit events, usage events, and errors
- tests for disabled tenants, disabled consumers, expired credentials, missing scopes, invalid signatures, replayed requests, idempotency behavior, and masking

Verify:
- disabled tenant traffic is rejected
- revoked, suspended, or expired credentials cannot call routes
- missing required scopes return 403
- HMAC-protected routes reject invalid or replayed requests
- sensitive request and response payload values do not appear in logs, audit records, billing records, or error responses

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 17 yet.
```

## 20. Sprint 17 Prompt

```text
Implement Sprint 17: Billing Admin APIs and Usage Reporting from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 16.
Work inside syra-backend/.

Follow BILLING_DESIGN.md and API_SPEC.md.

Add:
- billing plan CRUD APIs
- usage event query APIs with tenant, route, consumer, status, and time filters
- billing summary query APIs by tenant and billing period
- billing summary generation endpoint or worker command
- overage and billable unit breakdowns in billing summary responses
- CSV or JSON export for usage summaries
- pagination for usage event reads
- audit events for billing plan and billing summary admin actions
- tests for billing APIs, tenant isolation, pagination, overage calculation, and audit behavior

Verify:
- admins can create and update billing plans
- admins can query usage events without seeing sensitive payload data
- billing summaries can be generated and read by period
- overage calculation is visible in API responses
- tenant-scoped usage queries cannot leak another tenant's events

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 18 yet.
```

## 21. Sprint 18 Prompt

```text
Implement Sprint 18: Admin Audit, RBAC, and Operator APIs from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 17.
Work inside syra-backend/.

Follow SECURITY_DESIGN.md and API_SPEC.md.

Add:
- admin identity abstraction to replace the single development admin token internally
- platform admin and tenant admin roles
- role checks for tenant-scoped and platform-scoped admin APIs
- audit log read APIs with tenant, actor, action, resource, and time filters
- immutable audit event repository behavior
- admin API key or bootstrap admin credential support
- tests for role authorization, audit log reads, denied admin actions, and immutable audit records

Verify:
- platform admins can manage all tenants
- tenant admins can manage only their assigned tenant
- denied admin actions do not mutate state
- audit logs can be queried without exposing secrets
- audit records cannot be updated or deleted through repositories

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 19 yet.
```

## 22. Sprint 19 Prompt

```text
Implement Sprint 19: Policy Persistence and Distributed Enforcement from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 18.
Work inside syra-backend/.

Use TECHNOLOGY_DECISIONS.md:
- Redis through github.com/redis/go-redis/v9
- hand-written SQL with repository interfaces
- testcontainers-go for integration tests where useful

Add:
- rate limit policy CRUD APIs
- quota policy CRUD APIs
- persisted policy assignments on routes or API products
- Redis-backed rate limiter implementation using the runtime state store
- Redis-backed quota counter implementation using the runtime state store
- fixed-window or sliding-window behavior based on policy configuration
- tests for policy persistence, distributed counters, route-level policies, API-product-level policies, and Redis fallback behavior

Verify:
- admin-created policies are loaded into gateway runtime config
- rate limit behavior is consistent across gateway instances using Redis
- quota counters are tenant-scoped and period-aware
- gateway behavior is explicit when Redis is unavailable

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 20 yet.
```

## 23. Sprint 20 Prompt

```text
Implement Sprint 20: Advanced Adapter Expansion from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 19.
Work inside syra-backend/.

Keep protocol-specific code behind adapter interfaces.
Do not bypass the shared auth, policy, transformation, billing, and observability pipeline.

Add at least two of these adapter proofs:
- gRPC outbound proof adapter
- GraphQL facade proof adapter
- webhook outbound delivery proof
- file-based ingestion or delivery proof

Also add:
- route configuration validation for each new adapter type
- integration tests for the selected adapters
- tests proving failed adapter calls emit usage events and metrics
- documentation showing how to manually test the selected adapters

Verify:
- the selected adapter proofs work through the same gateway route execution model
- adapter-specific configs are validated before publish or reload
- failed adapter calls emit usage events and metrics
- no protocol-specific shortcut bypasses auth, policy, billing, or observability

Run go test ./... from inside syra-backend/ and fix failures.
Do not implement Sprint 21 yet.
```

## 24. Sprint 21 Prompt

```text
Implement Sprint 21: Production Readiness and Deployment from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 20.
Work inside syra-backend/ unless updating root-level documentation.

Add:
- Dockerfiles for gateway, control plane, and optional workers
- production-oriented compose file for local multi-service testing
- Kubernetes manifests or Helm chart skeleton
- migration command or startup migration documentation
- readiness checks for PostgreSQL, Redis, and config load status
- graceful shutdown tests for in-flight requests
- load test scripts for REST and REST to ISO8583 paths
- runbooks for local development, manual testing, migrations, and troubleshooting

Verify:
- services can be built into containers
- local compose can run gateway, control plane, PostgreSQL, and Redis together
- readiness accurately reports dependency and config status
- operators have documented commands for migrations, startup, shutdown, and troubleshooting

Run go test ./... from inside syra-backend/ and fix failures.
Do not add new product scope beyond Sprint 21.
```

## 25. Multi-Feature Planning Prompt

Use this when you want to update the roadmap again before implementing code.

```text
Review IMPLEMENTATION_PLAN.md, SPRINT_PROMPTS.md, PRODUCT_DESIGN.md, TECHNICAL_DESIGN.md, SECURITY_DESIGN.md, BILLING_DESIGN.md, and TRANSFORMATION_DESIGN.md.

Propose the next multiple-feature roadmap after the current last sprint.
Update IMPLEMENTATION_PLAN.md with sprint goals, scope, and done criteria.
Update SPRINT_PROMPTS.md with copy-paste prompts for each new sprint.

Do not implement code.
Keep all implementation work scoped to syra-backend/ in the prompts.
```

## 26. Prompt If a Sprint Gets Too Large

If a sprint becomes too large, use:

```text
Continue the current sprint only. Finish the smallest coherent slice, add tests, run them, and summarize what remains for this sprint.
```

## 27. Prompt For Bug Fixing

Use this when tests fail after a sprint:

```text
Fix the failing tests from the current sprint inside syra-backend/. Do not add new scope. Run go test ./... from inside syra-backend/ and summarize the fix.
```
