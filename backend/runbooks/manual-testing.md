# Runbook: Manual Testing

Primary guide: [`MANUAL_TESTING.md`](../MANUAL_TESTING.md).

Quick checks:

```sh
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
curl -i http://localhost:8081/healthz
curl -i http://localhost:8081/readyz
```

Load tests:

```sh
chmod +x scripts/load/rest_load.sh scripts/load/rest_to_iso8583_load.sh
API_KEY='<tenant-api-key>' URL='http://localhost:8080/accounts' scripts/load/rest_load.sh
API_KEY='<tenant-api-key>' URL='http://localhost:8080/cards/authorization' scripts/load/rest_to_iso8583_load.sh
```

MCP quick checks:

```sh
curl -i http://localhost:8082/healthz
curl -i -H 'X-MCP-Token: dev-mcp-token' http://localhost:8082/mcp
curl -i -H 'X-MCP-Token: dev-mcp-token' -H 'X-MCP-Role: platform_admin' http://localhost:8082/mcp/tools/list-tenants
```

MCP tenant-scope check:

```sh
curl -i \
  -H 'X-MCP-Token: dev-mcp-token' \
  -H 'X-MCP-Role: tenant_admin' \
  -H 'X-MCP-Tenant-ID: tenant_1' \
  'http://localhost:8082/mcp/tools/list-routes?tenantId=tenant_2'
```

Expected result: `403 forbidden` (cross-tenant access blocked).
