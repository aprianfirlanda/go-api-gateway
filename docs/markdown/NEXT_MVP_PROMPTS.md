# Next MVP Prompts (Split for Codex)

Use prompts in order.  
Run each prompt in a separate task/turn.

## Prompt 0: Ground Rules (Run First)

```text
You are working in this repository:
- backend/
- frontend/
- docs/

Read first:
- docs/markdown/NEXT_MVP_PLAN.md
- docs/markdown/TECHNOLOGY_DECISIONS.md
- docs/markdown/SECURITY_DESIGN.md
- docs/markdown/API_SPEC.md

Rules:
- Keep implementation inside backend/, frontend/, and docs/.
- Keep tenant isolation explicit in APIs, repositories, and tests.
- Never log PAN/CVV/PIN/API keys/tokens/secrets.
- Add or update tests for every behavior change.
- After backend changes run: go test ./... from backend/.
- After frontend changes run project tests/lint/build and report exact commands + results.
- If scope is too large, finish only the exact sprint requested and stop.
```

## Prompt 1: Sprint 1 (Frontend Foundation)

```text
Implement Sprint 1 from docs/markdown/NEXT_MVP_PLAN.md.

Scope:
- Frontend foundation and app shell only.
- Routing, auth boundary, RBAC gate, API client, shared UI primitives.

Exact targets to create/update:
- frontend/package.json
- frontend/tsconfig.json
- frontend/next.config.* or frontend/vite.config.* (choose one and document)
- frontend/.env.example
- frontend/src/app/layout.tsx (or equivalent)
- frontend/src/app/page.tsx
- frontend/src/app/(admin)/layout.tsx
- frontend/src/app/(tenant)/layout.tsx
- frontend/src/lib/auth.ts
- frontend/src/lib/rbac.ts
- frontend/src/middleware.ts (if using Next)
- frontend/src/lib/api/client.ts
- frontend/src/lib/api/types.ts
- frontend/src/lib/api/errors.ts
- frontend/src/components/ui/Table.tsx
- frontend/src/components/ui/Form.tsx
- frontend/src/components/ui/Modal.tsx
- frontend/src/components/ui/StatusBadge.tsx
- frontend/src/components/ui/EmptyState.tsx

Requirements:
- Use clean, minimal, production-oriented structure.
- No fake marketing landing page.
- Route guards must enforce role boundary.

Validation:
- Run frontend install + test/lint/build commands.
- Report results and changed files.
```

## Prompt 2: Sprint 2 (Platform Admin Console MVP)

```text
Implement Sprint 2 from docs/markdown/NEXT_MVP_PLAN.md.

Scope:
- Platform admin pages and core CRUD workflows.
- Align with existing backend control plane APIs.

Frontend targets:
- frontend/src/app/(admin)/tenants/page.tsx
- frontend/src/app/(admin)/users/page.tsx
- frontend/src/app/(admin)/api-products/page.tsx
- frontend/src/app/(admin)/routes/page.tsx
- frontend/src/app/(admin)/credentials/page.tsx
- frontend/src/app/(admin)/billing/page.tsx
- frontend/src/app/(admin)/audit-logs/page.tsx
- frontend/src/components/admin/TenantForm.tsx
- frontend/src/components/admin/RouteForm.tsx
- frontend/src/components/admin/CredentialRotateDialog.tsx
- frontend/src/components/admin/BillingSummaryTable.tsx
- frontend/src/components/admin/AuditLogTable.tsx

Backend alignment targets (only if required by frontend):
- backend/internal/controlplane/router.go
- backend/internal/controlplane/store.go
- backend/internal/storage/postgres/controlplane_repository.go
- backend/internal/controlplane/router_test.go

Requirements:
- Admin flows must use real APIs (not permanent mocks).
- Handle empty/loading/error states.

Validation:
- Run backend go test ./... from backend/ (if backend changed).
- Run frontend tests/lint/build.
- Report results and changed files.
```

## Prompt 3: Sprint 3 (Tenant Admin Console MVP)

```text
Implement Sprint 3 from docs/markdown/NEXT_MVP_PLAN.md.

Scope:
- Tenant-scoped dashboard and management pages.
- Strict tenant isolation in UI and backend access.

Frontend targets:
- frontend/src/app/(tenant)/dashboard/page.tsx
- frontend/src/app/(tenant)/routes/page.tsx
- frontend/src/app/(tenant)/credentials/page.tsx
- frontend/src/app/(tenant)/usage/page.tsx
- frontend/src/app/(tenant)/transformations/page.tsx
- frontend/src/components/tenant/RouteTable.tsx
- frontend/src/components/tenant/CredentialTable.tsx
- frontend/src/components/tenant/UsageChart.tsx
- frontend/src/components/tenant/TransformationEditor.tsx

Backend targets (if needed for enforcement/fixes):
- backend/internal/controlplane/admin_auth.go
- backend/internal/controlplane/router.go
- backend/internal/storage/postgres/controlplane_repository.go
- backend/internal/storage/postgres/controlplane_repository_integration_test.go

Requirements:
- Tenant admin cannot access cross-tenant resources.
- Show clear authorization failures.

Validation:
- Run backend go test ./... from backend/ (if backend changed).
- Run frontend tests/lint/build.
- Report results and changed files.
```

## Prompt 4: Sprint 4 (Public Useful Page + Nextra Foundation)

```text
Implement Sprint 4 from docs/markdown/NEXT_MVP_PLAN.md.

Scope:
- Build useful public pages.
- Initialize Nextra docs app skeleton.

Frontend targets:
- frontend/src/app/page.tsx
- frontend/src/app/status/page.tsx
- frontend/src/components/public/SystemStatus.tsx
- frontend/src/components/public/OnboardingChecklist.tsx
- frontend/src/app/api/status/route.ts (optional proxy)

Backend target (only if necessary for richer status):
- backend/internal/httpserver/health_handler.go

Docs/Nextra targets:
- docs/nextra/package.json
- docs/nextra/next.config.*
- docs/nextra/theme.config.*
- docs/nextra/pages/index.mdx
- docs/nextra/pages/manuals/index.mdx

Requirements:
- Public page must be operationally useful, not marketing fluff.
- Keep design simple and readable.

Validation:
- Run frontend checks.
- Run backend go test ./... if backend changed.
- Report results and changed files.
```

## Prompt 5: Sprint 5 (Nextra Manuals)

```text
Implement Sprint 5 from docs/markdown/NEXT_MVP_PLAN.md.

Scope:
- Write user manuals in Nextra for installation, admin, and admin tenant.

Targets:
- docs/nextra/manuals/installation/index.mdx
- docs/nextra/manuals/installation/prerequisites.mdx
- docs/nextra/manuals/installation/local-setup.mdx
- docs/nextra/manuals/installation/deploy-basic.mdx
- docs/nextra/manuals/installation/troubleshooting.mdx
- docs/nextra/manuals/admin/index.mdx
- docs/nextra/manuals/admin/tenants.mdx
- docs/nextra/manuals/admin/users-roles.mdx
- docs/nextra/manuals/admin/routes-products.mdx
- docs/nextra/manuals/admin/billing-audit.mdx
- docs/nextra/manuals/admin-tenant/index.mdx
- docs/nextra/manuals/admin-tenant/routes.mdx
- docs/nextra/manuals/admin-tenant/credentials.mdx
- docs/nextra/manuals/admin-tenant/usage.mdx
- docs/nextra/manuals/admin-tenant/transformations.mdx

Requirements:
- Manuals must be procedural and task-driven.
- Include prerequisites, step-by-step, expected results, and troubleshooting.
- Keep content aligned with actual backend/frontend behavior.

Validation:
- Run docs app checks/build.
- Report results and changed files.
```

## Prompt 6: Sprint 6 (MCP Server)

```text
Implement Sprint 6 from docs/markdown/NEXT_MVP_PLAN.md.

Scope:
- Add MCP server for safe operational interaction with app/control-plane.

Targets:
- backend/cmd/mcp-server/main.go
- backend/internal/mcp/server.go
- backend/internal/mcp/auth.go
- backend/internal/mcp/tools.go
- backend/internal/mcp/types.go
- backend/internal/mcp/tool_list_tenants.go
- backend/internal/mcp/tool_create_tenant.go
- backend/internal/mcp/tool_list_routes.go
- backend/internal/mcp/tool_create_route.go
- backend/internal/mcp/tool_rotate_api_key.go
- backend/internal/mcp/tool_usage_report.go
- backend/internal/mcp/tool_audit_logs.go
- backend/internal/controlplane/store.go
- backend/internal/controlplane/repository.go
- backend/internal/storage/postgres/controlplane_repository.go
- backend/internal/mcp/server_test.go
- backend/internal/mcp/tools_test.go

Requirements:
- Enforce auth + RBAC + tenant isolation.
- Add audit trail for MCP actions.
- Return structured, safe error responses.

Validation:
- Run go test ./... from backend/.
- Report results and changed files.
```

## Prompt 7: Sprint 7 (Hardening + Release Readiness)

```text
Implement Sprint 7 from docs/markdown/NEXT_MVP_PLAN.md.

Scope:
- End-to-end hardening for admin, tenant, public page, and MCP flows.

Targets:
- frontend/tests/e2e/admin-flows.spec.ts
- frontend/tests/e2e/tenant-flows.spec.ts
- frontend/tests/e2e/public-page.spec.ts
- backend/internal/controlplane/router_test.go
- backend/internal/httpserver/gateway_handler_test.go
- backend/internal/storage/postgres/runtime_config_source_integration_test.go
- backend/runbooks/local-development.md
- backend/runbooks/troubleshooting.md
- backend/runbooks/manual-testing.md
- docs/nextra/manuals/installation/troubleshooting.mdx

Requirements:
- Focus on critical path reliability and security.
- No new feature scope beyond hardening.

Validation:
- Run backend go test ./... from backend/.
- Run frontend tests/lint/build (including E2E).
- Report results and changed files.
```

## Prompt 8: Optional Split Prompts for Sprint 6 (MCP Too Large)

If Sprint 6 is too large in one run, split into 3 prompts:

### 8A: MCP Bootstrap

```text
Implement only MCP bootstrap:
- backend/cmd/mcp-server/main.go
- backend/internal/mcp/server.go
- backend/internal/mcp/auth.go
- backend/internal/mcp/types.go

Add minimal tests for server startup and auth guard.
Run go test ./... from backend/.
```

### 8B: MCP Tools CRUD

```text
Implement MCP tools:
- listTenants, createTenant, listRoutes, createRoute

Targets:
- backend/internal/mcp/tools.go
- backend/internal/mcp/tool_list_tenants.go
- backend/internal/mcp/tool_create_tenant.go
- backend/internal/mcp/tool_list_routes.go
- backend/internal/mcp/tool_create_route.go
- related controlplane/repository updates
- tests

Run go test ./... from backend/.
```

### 8C: MCP Security + Reporting Tools

```text
Implement remaining MCP tools:
- rotateApiKey, usageReport, auditLogs

Targets:
- backend/internal/mcp/tool_rotate_api_key.go
- backend/internal/mcp/tool_usage_report.go
- backend/internal/mcp/tool_audit_logs.go
- audit + RBAC enforcement in handlers
- tests

Run go test ./... from backend/.
```

