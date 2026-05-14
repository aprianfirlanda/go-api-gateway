# Technology Decisions

Locked stack unless explicitly changed:

- Go: `1.25.9`
- HTTP server: `net/http`
- Router: `github.com/go-chi/chi/v5`
- Logger: `log/slog`
- Config: environment variables + internal config package
- Database: PostgreSQL
- PostgreSQL driver: `github.com/jackc/pgx/v5`
- SQL style: hand-written SQL with repository interfaces
- Migrations: `github.com/pressly/goose/v3`
- Redis: `github.com/redis/go-redis/v9`
- API key hashing: `golang.org/x/crypto/argon2`
- IDs: `github.com/google/uuid`
- Tests: `testing`, `httptest`, `testify`
- Integration tests: `testcontainers-go`
- Metrics: `prometheus/client_golang`
- Tracing: OpenTelemetry
- ISO8583: internal codec interface first
- XML: `encoding/xml`
- JSON: `encoding/json`
- Transformation expressions: restricted internal evaluator
