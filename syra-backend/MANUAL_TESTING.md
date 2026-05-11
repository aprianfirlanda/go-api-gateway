# Syra Backend Manual Testing Guide

This guide covers building the backend, running the automated suite, starting the
gateway and control plane locally, and manually smoke testing the MVP features.

## Prerequisites

- Go 1.25.9
- Docker Desktop or a compatible Docker runtime
- `curl`
- Optional: `jq` for extracting IDs from JSON responses

All commands below run from `syra-backend/`.

## Build and Automated Tests

```sh
cd syra-backend
go test ./...
go build ./cmd/gateway ./cmd/control-plane
```

The PostgreSQL repository integration tests use testcontainers, so Docker must
be running before `go test ./...`.

## Start Local Dependencies

```sh
docker compose up -d postgres redis
export DATABASE_URL='postgres://app:app@localhost:5432/app?sslmode=disable'
export REDIS_ADDR='localhost:6379'
export CONTROL_PLANE_ADMIN_TOKEN='dev-admin-token'
```

The control plane can also run without `DATABASE_URL`; it will use the in-memory
store. Use PostgreSQL for manual testing that should survive a process restart.

## Start Services

Terminal 1:

```sh
cd syra-backend
go run ./cmd/control-plane
```

Terminal 2:

```sh
cd syra-backend
go run ./cmd/gateway
```

Defaults:

- Gateway: `http://localhost:8080`
- Control plane: `http://localhost:8081`
- Admin bearer token: `dev-admin-token`

## Smoke Test the Running Processes

```sh
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
curl -i http://localhost:8080/metrics

curl -i \
  -H 'Authorization: Bearer dev-admin-token' \
  http://localhost:8081/admin/v1/tenants
```

Expected results:

- `/healthz` returns `200`.
- `/readyz` returns `200` when dependencies are healthy.
- `/metrics` returns Prometheus text including `syra_gateway_requests_total`.
- Control plane calls without the bearer token return `401`.

## Manual Control Plane Flow

Create a tenant:

```sh
curl -sS -X POST http://localhost:8081/admin/v1/tenants \
  -H 'Authorization: Bearer dev-admin-token' \
  -H 'Content-Type: application/json' \
  -d '{"name":"Acme Finance","slug":"acme","billingPlanId":"starter"}'
```

Save the returned `id` as `TENANT_ID`.

Create an API product:

```sh
curl -sS -X POST "http://localhost:8081/admin/v1/tenants/$TENANT_ID/api-products" \
  -H 'Authorization: Bearer dev-admin-token' \
  -H 'Content-Type: application/json' \
  -d '{"name":"Payments","slug":"payments","description":"Payments APIs"}'
```

Save the returned `id` as `API_PRODUCT_ID`.

Create an HTTP upstream:

```sh
curl -sS -X POST "http://localhost:8081/admin/v1/tenants/$TENANT_ID/upstreams" \
  -H 'Authorization: Bearer dev-admin-token' \
  -H 'Content-Type: application/json' \
  -d '{"name":"Echo","protocol":"rest","config":{"baseUrl":"http://localhost:9000"},"status":"active"}'
```

Save the returned `id` as `UPSTREAM_ID`.

Create and publish a route:

```sh
curl -sS -X POST "http://localhost:8081/admin/v1/tenants/$TENANT_ID/routes" \
  -H 'Authorization: Bearer dev-admin-token' \
  -H 'Content-Type: application/json' \
  -d "{
    \"apiProductId\":\"$API_PRODUCT_ID\",
    \"name\":\"Create Payment\",
    \"inboundProtocol\":\"rest\",
    \"outboundProtocol\":\"rest\",
    \"host\":\"api.local.test\",
    \"method\":\"POST\",
    \"path\":\"/payments\",
    \"upstreamId\":\"$UPSTREAM_ID\",
    \"timeoutMs\":1000
  }"
```

Save the returned `id` as `ROUTE_ID`, then publish:

```sh
curl -sS -X POST "http://localhost:8081/admin/v1/tenants/$TENANT_ID/routes/$ROUTE_ID/publish" \
  -H 'Authorization: Bearer dev-admin-token'
```

Create a consumer and API key:

```sh
curl -sS -X POST "http://localhost:8081/admin/v1/tenants/$TENANT_ID/consumers" \
  -H 'Authorization: Bearer dev-admin-token' \
  -H 'Content-Type: application/json' \
  -d '{"name":"Mobile App","slug":"mobile-app","ownerUserId":"user-1"}'
```

Save the returned `id` as `CONSUMER_ID`.

```sh
curl -sS -X POST "http://localhost:8081/admin/v1/tenants/$TENANT_ID/consumers/$CONSUMER_ID/credentials" \
  -H 'Authorization: Bearer dev-admin-token' \
  -H 'Content-Type: application/json' \
  -d '{"type":"api_key","scopes":["payments:write"]}'
```

Save the returned `apiKey`. The plaintext API key is only returned once.

## Transformation Template Flow

Create a draft template:

```sh
curl -sS -X POST "http://localhost:8081/admin/v1/tenants/$TENANT_ID/transformation-templates" \
  -H 'Authorization: Bearer dev-admin-token' \
  -H 'Content-Type: application/json' \
  -d "{
    \"apiProductId\":\"$API_PRODUCT_ID\",
    \"name\":\"Payment REST passthrough\",
    \"sourceProtocol\":\"rest\",
    \"targetProtocol\":\"rest\",
    \"templateBody\":{\"request\":{\"body\":{\"amount\":\"$.amount\"}},\"response\":{\"body\":{\"status\":\"$.status\"}}}
  }"
```

Save the returned `id` as `TEMPLATE_ID`, then publish:

```sh
curl -sS -X POST "http://localhost:8081/admin/v1/tenants/$TENANT_ID/transformation-templates/$TEMPLATE_ID/publish" \
  -H 'Authorization: Bearer dev-admin-token'
```

Published templates can be attached to active routes. Draft, disabled, or
archived templates are rejected by the gateway runtime.

## Runtime Gateway Feature Checks

The current MVP has separate control plane storage and gateway runtime config.
Control plane resources are persisted and audited, but they are not yet synced
into the running gateway process as live route config. For full data-plane
behavior, run the focused automated tests:

```sh
go test ./internal/httpserver -run 'Gateway|Billing|SOAP|ISO|Transformation|Policy'
go test ./internal/gateway/listener/iso8583 -run TestISO8583InboundFlow
go test ./internal/protocol/soapxml -run Test
go test ./internal/billing -run Test
go test ./internal/configreload -run Test
```

These tests cover:

- Authenticated REST gateway routing
- REST to REST proxying
- REST to ISO8583 outbound calls
- ISO8583 inbound listener to REST route flow
- REST to SOAP/XML request and SOAP/XML response mapping
- Transformation masking and published-template enforcement
- Billing events for success, rejected, failed, and timeout attempts
- Billing summary and overage calculation
- Rate-limit, quota, IP allowlist, and request-size policy behavior
- Config reload validation and last-known-good fallback
- Prometheus metrics and trace hooks

## PostgreSQL Repository Checks

Run the repository integration suite with Docker running:

```sh
go test ./internal/storage/postgres -v
```

The initial migration lives in `migrations/00001_initial_schema.sql` and is based
on the MVP data model.

## Stop Local Services

```sh
docker compose down
```
