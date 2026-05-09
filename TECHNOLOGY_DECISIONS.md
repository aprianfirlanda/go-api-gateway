# Technology Decisions: Go API Gateway

## 1. Purpose

This document locks the initial technology choices for the Go-based multitenant API gateway.

These choices are intended to keep the MVP simple, production-oriented, and compatible with the existing design documents.

## 2. Core Language and Runtime

Decision:

```text
Go stable release
```

Reason:

- Strong standard library for HTTP, TCP, TLS, and concurrency.
- Simple static binary deployment.
- Good fit for gateway and worker services.

## 3. HTTP Server

Decision:

```text
net/http
```

Reason:

- Production-grade standard library.
- No framework lock-in.
- Easy to test with `httptest`.

## 4. HTTP Router

Decision:

```text
github.com/go-chi/chi/v5
```

Reason:

- Small and idiomatic.
- Works cleanly with `net/http`.
- Good middleware support.
- Suitable for both gateway runtime and control plane APIs.

## 5. Logging

Decision:

```text
log/slog
```

Reason:

- Standard library structured logging.
- Avoids extra logging dependency.
- Good enough for JSON logs and request-scoped fields.

## 6. Configuration

Decision:

```text
Environment variables for MVP
Internal config package
```

Reason:

- Simple for local development and deployment.
- Avoids early dependency on a config framework.
- Can evolve later to file-based config or remote config snapshots.

Initial environment variables:

```text
GATEWAY_ADDR=:8080
CONTROL_PLANE_ADDR=:8081
DATABASE_URL=postgres://...
REDIS_ADDR=localhost:6379
LOG_LEVEL=info
```

## 7. Database

Decision:

```text
PostgreSQL
```

Reason:

- Strong relational model.
- Good JSONB support for templates and adapter configs.
- Good fit for tenant-scoped configuration, billing, and audit logs.

## 8. PostgreSQL Driver

Decision:

```text
github.com/jackc/pgx/v5
```

Reason:

- Mature PostgreSQL driver.
- Strong context support.
- Good performance.
- Works well with transactions and connection pools.

## 9. SQL Approach

Decision:

```text
Hand-written SQL with repository interfaces
```

Reason:

- Keeps database access explicit.
- Avoids ORM complexity.
- Easier to enforce tenant-scoped queries.

Optional future addition:

```text
sqlc
```

Use `sqlc` only if hand-written scanning becomes noisy.

## 10. Migrations

Decision:

```text
github.com/pressly/goose/v3
```

Reason:

- Simple migration workflow.
- Works well with Go projects.
- Supports SQL migrations.

Migration directory:

```text
migrations/
```

## 11. Redis Client

Decision:

```text
github.com/redis/go-redis/v9
```

Reason:

- Mature Redis client.
- Good context support.
- Suitable for rate limiting, quota counters, nonce replay protection, and short-lived runtime state.

## 12. Authentication Hashing

Decision:

```text
golang.org/x/crypto/argon2
```

Reason:

- Strong password/API secret hashing primitive.
- Good fit for API key secret verification.

API key format:

```text
gw_live_<prefix>.<secret>
```

Store:

```text
key_prefix
secret_hash
```

## 13. IDs

Decision:

```text
github.com/google/uuid
```

Reason:

- Simple UUID support.
- Compatible with PostgreSQL UUID columns.

## 14. Validation

Decision:

```text
Manual validation in request/model packages for MVP
```

Reason:

- Keeps validation rules explicit.
- Avoids reflection-heavy framework dependency.
- Easier to produce precise API error responses.

## 15. Testing

Decision:

```text
testing
net/http/httptest
```

Additional assertion helper:

```text
github.com/stretchr/testify
```

Reason:

- Standard Go tests remain the foundation.
- `testify/require` reduces repetitive test failure handling.

## 16. Integration Testing

Decision:

```text
testcontainers-go for PostgreSQL and Redis integration tests
```

Package:

```text
github.com/testcontainers/testcontainers-go
```

Reason:

- Real database and Redis behavior in tests.
- Avoids relying on developer machine services.

Use container-backed PostgreSQL and Redis integration tests from Sprint 11 onward. Earlier sprints may still use local `httptest`, mock upstream, or in-process integration tests when they do not require PostgreSQL or Redis containers.

## 17. Observability

Metrics decision:

```text
github.com/prometheus/client_golang
```

Tracing decision:

```text
go.opentelemetry.io/otel
```

Reason:

- Prometheus metrics are common and easy to expose.
- OpenTelemetry is the standard direction for distributed tracing.

## 18. ISO8583

Decision for MVP:

```text
Implement an internal ISO8583 codec interface.
Start with internal pack/unpack support for required fields.
Evaluate external ISO8583 libraries behind the interface only if needed.
```

Reason:

- ISO8583 dialects vary heavily by switch.
- A narrow internal codec keeps the MVP controlled.
- External libraries can be introduced later without changing gateway architecture.

Internal package:

```text
internal/protocol/iso8583
```

## 19. XML and SOAP

Decision:

```text
encoding/xml for MVP SOAP/XML proof adapter
```

Reason:

- Standard library.
- Enough for controlled SOAP envelope generation and parsing.

Security requirement:

- Do not enable unsafe XML entity behavior.
- Enforce payload size limits.

## 20. JSON

Decision:

```text
encoding/json
```

Reason:

- Standard library.
- Adequate for MVP.

## 21. Expression Engine

Decision for MVP:

```text
Implement a small restricted expression evaluator.
```

Reason:

- Transformation expressions must be auditable and predictable.
- Do not allow arbitrary scripting.
- Start with field references, static values, and approved built-in functions.

Supported MVP expression examples:

```text
$.fields.amount
'000000'
formatAmount($.fields.amount)
currencyNumeric($.fields.currency)
generateStan()
maskPan($.fields.pan)
```

## 22. Project Layout

Decision:

```text
cmd/
internal/
pkg/
migrations/
```

Reason:

- Standard Go project layout.
- Keeps application internals private.

## 23. Development Command Targets

Decision:

Use a `Makefile` once code exists.

Initial commands:

```text
make test
make run-gateway
make run-control-plane
make migrate-up
make migrate-down
```

## 24. Summary

Locked choices:

```text
HTTP server: net/http
Router: chi
Logger: slog
Config: env vars + internal config package
Database: PostgreSQL
PostgreSQL driver: pgx
SQL: hand-written SQL with repositories
Migrations: goose
Redis: go-redis
API key hashing: argon2
IDs: google/uuid
Tests: testing, httptest, testify
Integration tests: testcontainers-go
Metrics: prometheus client_golang
Tracing: OpenTelemetry
ISO8583: internal codec interface first
SOAP/XML: encoding/xml
JSON: encoding/json
Expression engine: restricted internal evaluator
```
