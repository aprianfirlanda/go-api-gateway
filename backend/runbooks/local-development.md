# Runbook: Local Development

## 1) Start dependencies

```sh
docker compose -f compose.prod.yaml up -d postgres redis
```

## 2) Run migrations

```sh
export DATABASE_URL='postgres://app:app@localhost:5432/app?sslmode=disable'
go run ./cmd/migrate
```

## 3) Start control plane

```sh
export CONTROL_PLANE_ADMIN_TOKEN='dev-admin-token'
go run ./cmd/control-plane
```

## 4) Start gateway

```sh
export DATABASE_URL='postgres://app:app@localhost:5432/app?sslmode=disable'
export REDIS_ADDR='localhost:6379'
export RUNTIME_STATE_BACKEND='redis'
export CONFIG_RELOAD_INTERVAL='5s'
go run ./cmd/gateway
```

## 5) Check readiness

```sh
curl -i http://localhost:8081/readyz
curl -i http://localhost:8080/readyz
```

## 6) Start MCP server

```sh
export MCP_AUTH_TOKEN='dev-mcp-token'
go run ./cmd/mcp-server
```

## 7) Verify MCP auth + tool health

```sh
curl -i http://localhost:8082/healthz
curl -i -H 'X-MCP-Token: dev-mcp-token' http://localhost:8082/mcp
curl -i -H 'X-MCP-Token: dev-mcp-token' http://localhost:8082/mcp/tools/list-tenants
```
