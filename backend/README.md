# Syra Backend

Syra Backend is a clean Go service template aligned with the API gateway technology choices:

- `net/http`
- `github.com/go-chi/chi/v5`
- `log/slog`
- Go 1.25.9
- environment-based configuration
- PostgreSQL through `github.com/jackc/pgx/v5`
- hand-written SQL behind repository interfaces
- Goose SQL migrations

The template keeps business code independent from framework details. HTTP, storage, and infrastructure packages adapt external systems into application ports.

## Run

```sh
go run ./cmd/gateway
```

Default address:

```text
:8080
```

Health endpoints:

```text
GET /healthz
GET /readyz
```

Control plane:

```sh
go run ./cmd/control-plane
```

Run migrations:

```sh
go run ./cmd/migrate
```

## Configuration

Configuration is loaded from environment variables:

```text
GATEWAY_ADDR=:8080
DATABASE_URL=postgres://user:pass@localhost:5432/app?sslmode=disable
LOG_LEVEL=info
HTTP_READ_TIMEOUT=5s
HTTP_WRITE_TIMEOUT=30s
HTTP_IDLE_TIMEOUT=60s
REQUEST_BODY_LIMIT_BYTES=1048576
RUNTIME_STATE_BACKEND=memory
RUNTIME_STATE_ENV=dev
RUNTIME_STATE_VERSION=v1
REDIS_TIMEOUT=2s
```

Redis is optional by default (`RUNTIME_STATE_BACKEND=memory`). Set
`RUNTIME_STATE_BACKEND=redis` to enable Redis-backed runtime state storage and
readiness checks:

```text
REDIS_ADDR=localhost:6379
RUNTIME_STATE_BACKEND=redis
```

When `DATABASE_URL` is set, the gateway loads runtime config from PostgreSQL at
startup and reloads it on the `CONFIG_RELOAD_INTERVAL` schedule.

Gateway readiness depends on:

- PostgreSQL (if configured)
- Redis (when `RUNTIME_STATE_BACKEND=redis`)
- runtime config load success status

## Structure

```text
cmd/gateway              service entrypoint
cmd/control-plane        control plane entrypoint
cmd/migrate              migration command
internal/app/gateway     dependency wiring
internal/config          environment configuration
internal/health          health use case
internal/httpserver      chi router, handlers, middleware
internal/ports           input and output interfaces
internal/storage         database adapters
pkg                      small reusable helpers
migrations               Goose SQL migrations
deploy/k8s               Kubernetes deployment skeleton
runbooks                 operator runbooks
scripts/load             load test scripts
```

## Verify

```sh
go test ./...
```

## Container Build

```sh
docker build -f Dockerfile.gateway -t syra/gateway:local .
docker build -f Dockerfile.control-plane -t syra/control-plane:local .
docker build -f Dockerfile.migrator -t syra/migrator:local .
```

## Local Multi-Service Compose

```sh
docker compose -f compose.prod.yaml up --build
```
