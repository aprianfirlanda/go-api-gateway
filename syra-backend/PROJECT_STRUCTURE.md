# Project Structure

This template uses a small ports-and-adapters layout. Domain and application code should stay independent from HTTP, PostgreSQL, Redis, and other infrastructure packages.

```text
cmd/
  gateway/
    main.go

internal/
  app/
    gateway/        dependency wiring
  config/           env-based config loader
  health/           health/readiness use case
  httpserver/       net/http + chi adapter
  ports/
    input/          inbound use case contracts
    output/         outbound dependency contracts
  storage/
    postgres/       pgx-based PostgreSQL adapter

pkg/
  ids/              small reusable helpers

migrations/         Goose SQL migrations
```

## Rules

- Use `net/http` handlers and middleware.
- Use chi only as the router.
- Use `log/slog` for structured logs.
- Keep config environment-based until there is a clear need for another source.
- Use `pgx` and hand-written SQL for persistence.
- Keep repository interfaces tenant-aware when adding multitenant data.
- Do not put protocol-specific gateway logic in shared HTTP middleware.
- Do not log API keys, tokens, PAN, CVV, PIN blocks, or secrets.
