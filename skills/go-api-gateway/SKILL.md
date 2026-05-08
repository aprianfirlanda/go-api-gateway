---
name: go-api-gateway
description: Use for all work in this repository: designing, implementing, testing, reviewing, or documenting the Go-based multitenant finance API gateway with protocol adapters, ISO8583/REST transformation, billing, tenant isolation, and security. Use when implementing sprint work from IMPLEMENTATION_PLAN.md or following SPRINT_PROMPTS.md.
---

# Go API Gateway Project Skill

## When To Use

Use this skill for any task in this repository that changes or reviews:

- Gateway data plane.
- Control plane APIs.
- Protocol adapters.
- REST, ISO8583, SOAP/XML, gRPC, webhooks, queues, files, or custom TCP support.
- Transformation engine.
- Tenant isolation.
- Authentication and authorization.
- Billing metering.
- Security and audit behavior.
- Sprint implementation.

## First Step

Read only the docs needed for the request:

- Sprint coding: `IMPLEMENTATION_PLAN.md`, `SPRINT_PROMPTS.md`, `TECHNOLOGY_DECISIONS.md`.
- Architecture changes: `ARCHITECTURE.md`, `TECHNICAL_DESIGN.md`.
- Database work: `DATA_MODEL.md`.
- API work: `API_SPEC.md`, `SECURITY_DESIGN.md`.
- Transformation work: `TRANSFORMATION_DESIGN.md`.
- Billing work: `BILLING_DESIGN.md`.
- Security work: `SECURITY_DESIGN.md`.
- Product scope questions: `PRODUCT_DESIGN.md`.

## Locked Technology Choices

Follow `TECHNOLOGY_DECISIONS.md`.

Default choices:

- `net/http` for HTTP server.
- `chi` for routing.
- `slog` for logging.
- Environment variables for MVP config.
- PostgreSQL with `pgx`.
- Hand-written SQL with repository interfaces.
- `goose` for migrations.
- Redis with `go-redis`.
- Argon2 for API key secret hashing.
- `testing`, `httptest`, and `testify` for tests.
- Testcontainers for PostgreSQL/Redis integration tests.
- Prometheus and OpenTelemetry for observability.
- Internal ISO8583 codec interface first.
- Restricted internal expression evaluator for transformations.

## Project Rules

- Build in Go.
- Do not depend on Kong, APISIX, Tyk, KrakenD, Envoy, or another gateway runtime.
- Keep data plane and control plane separated.
- Keep protocol-specific behavior behind adapter interfaces.
- Use a canonical message model for transformations.
- Keep tenant isolation explicit and tested.
- Use usage events for billing, not logs.
- Mask sensitive financial data.
- Do not log full PAN, CVV, PIN block, API keys, tokens, or secrets.
- Work sprint by sprint unless the user asks otherwise.

## Sprint Workflow

When the user asks to implement a sprint:

1. Read the matching sprint section in `IMPLEMENTATION_PLAN.md`.
2. Read the matching prompt in `SPRINT_PROMPTS.md`.
3. Implement only that sprint's scope.
4. Add or update tests for the sprint.
5. Run `go test ./...`.
6. Report what changed and any blockers.

If a sprint is too large, finish the smallest coherent slice and state what remains in that sprint.

## Review Workflow

When reviewing code:

1. Check tenant isolation.
2. Check auth and authorization boundaries.
3. Check sensitive data masking.
4. Check billing event correctness.
5. Check protocol adapter boundaries.
6. Check tests for cross-tenant and failure behavior.

Lead with findings and file/line references.

