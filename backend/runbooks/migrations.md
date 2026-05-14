# Runbook: Migrations

## Run once from host

```sh
export DATABASE_URL='postgres://app:app@localhost:5432/app?sslmode=disable'
go run ./cmd/migrate
```

## Run with Docker

```sh
docker compose -f compose.prod.yaml run --rm migrate
```

## Validate migration version

```sh
psql 'postgres://app:app@localhost:5432/app?sslmode=disable' -c "SELECT version_id, is_applied FROM goose_db_version ORDER BY id DESC LIMIT 5;"
```

## Rollback guidance

Use a forward-fix migration by default. Avoid down migrations on shared environments unless explicitly approved.
