# AGENTS.md

## Project

This repository contains a Go-based multitenant API gateway for finance companies.

The gateway must be built in Go without depending on gateway runtimes such as Kong, APISIX, Tyk, KrakenD, or Envoy.

Core product requirements:

- Multitenant API gateway.
- Protocol adapter model.
- REST and ISO8583 as first MVP adapters.
- Future support for SOAP/XML, gRPC, GraphQL facade, webhooks, message queues, files, and proprietary TCP protocols.
- Usage-based billing.
- Finance-grade security, audit logging, and sensitive data masking.

## Required Reading

Before implementation work, read the relevant docs:

- `PRODUCT_DESIGN.md`: product scope and MVP.
- `TECHNICAL_DESIGN.md`: Go runtime and protocol adapter architecture.
- `ARCHITECTURE.md`: system architecture and deployment model.
- `IMPLEMENTATION_PLAN.md`: milestones and sprint roadmap.
- `SPRINT_PROMPTS.md`: copy-paste sprint prompts.
- `TECHNOLOGY_DECISIONS.md`: locked library and tooling choices.
- `DATA_MODEL.md`: PostgreSQL schema and tenant isolation rules.
- `API_SPEC.md`: control plane and runtime API spec.
- `TRANSFORMATION_DESIGN.md`: canonical message and transformation model.
- `SECURITY_DESIGN.md`: finance security requirements.
- `BILLING_DESIGN.md`: usage metering and billing design.

For sprint implementation, always start with `IMPLEMENTATION_PLAN.md`, `SPRINT_PROMPTS.md`, and `TECHNOLOGY_DECISIONS.md`.

## Technology Decisions

Use these locked choices unless the user explicitly changes them:

- Go version: `1.25.9`
- HTTP server: `net/http`
- Router: `github.com/go-chi/chi/v5`
- Logger: `log/slog`
- Config: environment variables plus internal config package
- Database: PostgreSQL
- PostgreSQL driver: `github.com/jackc/pgx/v5`
- SQL: hand-written SQL with repository interfaces
- Migrations: `github.com/pressly/goose/v3`
- Redis: `github.com/redis/go-redis/v9`
- API key hashing: `golang.org/x/crypto/argon2`
- IDs: `github.com/google/uuid`
- Tests: `testing`, `net/http/httptest`, `github.com/stretchr/testify`
- Integration tests: `github.com/testcontainers/testcontainers-go`
- Metrics: `github.com/prometheus/client_golang`
- Tracing: OpenTelemetry
- ISO8583: internal codec interface first
- SOAP/XML: `encoding/xml`
- JSON: `encoding/json`
- Transformation expressions: restricted internal evaluator

## Template Foundation

- `syra-backend/` is the cleaned Go backend template foundation.
- Keep it aligned with the locked technology choices above.
- Do not reintroduce Fiber, GORM, Logrus, Viper, Cobra, RabbitMQ, or Swagger into the template unless explicitly requested.

## Implementation Rules

- Work sprint by sprint unless the user explicitly asks otherwise.
- Do not implement later sprint scope early unless needed to keep current sprint coherent.
- Keep data plane and control plane boundaries clear.
- Keep protocol-specific code behind adapter interfaces.
- Keep billing based on usage events, not logs.
- Keep tenant isolation explicit in models, routes, repositories, and tests.
- Do not log full PAN, CVV, PIN block, API keys, tokens, or secrets.
- Prefer simple interfaces and tests before advanced abstractions.

## Validation

For code changes:

- Run `go test ./...`.
- Add or update focused tests for the sprint.
- If tests cannot run, state the exact blocker.

For documentation-only changes:

- No tests are required.
- Keep docs consistent with existing design files.

## Useful Prompts

Start implementation:

```text
Implement Sprint 1 from IMPLEMENTATION_PLAN.md. Add tests and run them.
```

Continue implementation:

```text
Implement the next sprint from IMPLEMENTATION_PLAN.md. Add tests and run them.
```

Fix current sprint only:

```text
Fix the failing tests from the current sprint. Do not add new scope. Run go test ./...
```
