# AGENTS.md

## Project

This is the `backend` Go service template for API gateway work.

## Locked Choices

- HTTP server: `net/http`
- Router: `github.com/go-chi/chi/v5`
- Logger: `log/slog`
- Config: environment variables plus `internal/config`
- Database: PostgreSQL
- PostgreSQL driver: `github.com/jackc/pgx/v5`
- SQL: hand-written SQL behind repository interfaces
- Migrations: `github.com/pressly/goose/v3`
- IDs: `github.com/google/uuid`
- Tests: `testing`, `net/http/httptest`, `github.com/stretchr/testify`

## Rules

- Keep domain and application code independent from HTTP and storage adapters.
- Keep tenant isolation explicit in future models, repositories, routes, and tests.
- Keep protocol-specific gateway logic behind adapter interfaces.
- Do not log sensitive data.
- Run `go test ./...` after code changes.
