# Runbook: Troubleshooting

## `/readyz` is not ready

1. Check process logs:
```sh
docker compose -f compose.prod.yaml logs gateway control-plane --tail=200
```
2. Check database and redis health:
```sh
docker compose -f compose.prod.yaml ps
```
3. Check runtime config load status by forcing reload interval wait, then re-check gateway readiness.

## Gateway returns `upstream_not_found`

1. Confirm route is published.
2. Confirm referenced upstream is active.
3. Wait for one `CONFIG_RELOAD_INTERVAL` and retry.

## Control plane admin auth fails

1. Verify `CONTROL_PLANE_ADMIN_TOKEN`.
2. Use header:
```sh
Authorization: Bearer <token>
```

## MCP auth fails (`401 unauthorized`)

1. Verify `MCP_AUTH_TOKEN` is set for `cmd/mcp-server`.
2. Send either:
```sh
X-MCP-Token: <token>
```
or:
```sh
Authorization: Bearer <token>
```
3. Check MCP health endpoint first:
```sh
curl -i http://localhost:8082/healthz
```

## Tenant admin MCP access denied (`403 forbidden`)

1. Confirm role and tenant context headers:
```sh
X-MCP-Role: tenant_admin
X-MCP-Tenant-ID: <tenant_id>
```
2. Ensure the requested tool tenant matches `X-MCP-Tenant-ID`.
3. Use platform admin role only for global/list-all operations.

## Shutdown handling

Graceful shutdown waits for in-flight requests up to `SHUTDOWN_TIMEOUT`. Increase timeout for long-running upstream calls.
