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

## 3. Sprint 1 Prompt

```text
Implement Sprint 1: Gateway Foundation from IMPLEMENTATION_PLAN.md.

Follow all existing design docs, especially TECHNOLOGY_DECISIONS.md.

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

Run go test ./... and fix failures.
Do not implement Sprint 2 yet.
```

## 4. Sprint 2 Prompt

```text
Implement Sprint 2: Tenant Routing and Authentication from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 1.

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

Run go test ./... and fix failures.
Do not implement Sprint 3 yet.
```

## 5. Sprint 3 Prompt

```text
Implement Sprint 3: REST Proxy from IMPLEMENTATION_PLAN.md.

Build on the existing code from Sprint 2.

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

Run go test ./... and fix failures.
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

Run go test ./... and fix failures.
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

Run go test ./... and fix failures.
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

Run go test ./... and fix failures.
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

Run go test ./... and fix failures.
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

Run go test ./... and fix failures.
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

Run go test ./... and fix failures.
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

Run go test ./... and fix failures.
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

Run go test ./... and fix failures.
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

Run go test ./... and fix failures.
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

Run go test ./... and fix failures.
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

## 17. Prompt If a Sprint Gets Too Large

If a sprint becomes too large, use:

```text
Continue the current sprint only. Finish the smallest coherent slice, add tests, run them, and summarize what remains for this sprint.
```

## 18. Prompt For Bug Fixing

Use this when tests fail after a sprint:

```text
Fix the failing tests from the current sprint. Do not add new scope. Run go test ./... and summarize the fix.
```

