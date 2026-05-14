# Next MVP Plan (Frontend + Manuals + MCP)

## Scope

This plan covers:

- frontend UI for platform admin and tenant admin
- public useful page
- Nextra manuals (installation, admin, admin tenant)
- MCP server to interact with the app

## Current Baseline

- `frontend/` is already initialized with Vite + React.
- `docs/nextra/` is already initialized with Nextra.
- This plan focuses on feature implementation and hardening, not project bootstrap.

## Sprint 1: Frontend Foundation and App Shell

### Goals

- add routing, auth boundary, shared API client, base layout
- define role-based navigation (`platform_admin`, `tenant_admin`)

### Tasks and Exact Targets

- Create app shell:
  - `frontend/src/App.tsx` and `frontend/src/main.tsx` (or existing entry files)
  - `frontend/src/routes/*` (role-based route setup)
- Create auth and RBAC gate:
  - `frontend/src/lib/auth.ts`
  - `frontend/src/lib/rbac.ts`
- Create API client:
  - `frontend/src/lib/api/client.ts`
  - `frontend/src/lib/api/types.ts`
  - `frontend/src/lib/api/errors.ts`
- Shared UI primitives:
  - `frontend/src/components/ui/Table.tsx`
  - `frontend/src/components/ui/Form.tsx`
  - `frontend/src/components/ui/Modal.tsx`
  - `frontend/src/components/ui/StatusBadge.tsx`
  - `frontend/src/components/ui/EmptyState.tsx`

### Done Criteria

- frontend boots locally
- role-based navigation renders
- protected routes redirect unauthorized users

## Sprint 2: Platform Admin Console MVP

### Goals

- deliver core admin workflows on real backend APIs

### Tasks and Exact Targets

- Admin routes/pages:
  - `frontend/src/app/(admin)/tenants/page.tsx`
  - `frontend/src/app/(admin)/users/page.tsx`
  - `frontend/src/app/(admin)/api-products/page.tsx`
  - `frontend/src/app/(admin)/routes/page.tsx`
  - `frontend/src/app/(admin)/credentials/page.tsx`
  - `frontend/src/app/(admin)/billing/page.tsx`
  - `frontend/src/app/(admin)/audit-logs/page.tsx`
- Admin feature components:
  - `frontend/src/components/admin/TenantForm.tsx`
  - `frontend/src/components/admin/RouteForm.tsx`
  - `frontend/src/components/admin/CredentialRotateDialog.tsx`
  - `frontend/src/components/admin/BillingSummaryTable.tsx`
  - `frontend/src/components/admin/AuditLogTable.tsx`
- Backend API alignment checks and missing endpoint tasks:
  - `backend/internal/controlplane/router.go`
  - `backend/internal/controlplane/store.go`
  - `backend/internal/storage/postgres/controlplane_repository.go`
  - `backend/internal/controlplane/router_test.go`

### Done Criteria

- admin can list/create/update core resources
- billing summary and audit log list are visible
- all changed backend APIs have tests

## Sprint 3: Tenant Admin Console MVP

### Goals

- tenant-scoped management for routes, credentials, usage, transformations

### Tasks and Exact Targets

- Tenant routes/pages:
  - `frontend/src/app/(tenant)/dashboard/page.tsx`
  - `frontend/src/app/(tenant)/routes/page.tsx`
  - `frontend/src/app/(tenant)/credentials/page.tsx`
  - `frontend/src/app/(tenant)/usage/page.tsx`
  - `frontend/src/app/(tenant)/transformations/page.tsx`
- Tenant components:
  - `frontend/src/components/tenant/RouteTable.tsx`
  - `frontend/src/components/tenant/CredentialTable.tsx`
  - `frontend/src/components/tenant/UsageChart.tsx`
  - `frontend/src/components/tenant/TransformationEditor.tsx`
- Backend tenant enforcement and tests:
  - `backend/internal/controlplane/admin_auth.go`
  - `backend/internal/controlplane/router.go`
  - `backend/internal/storage/postgres/controlplane_repository.go`
  - `backend/internal/storage/postgres/controlplane_repository_integration_test.go`

### Done Criteria

- tenant admin only sees own tenant resources
- tenant-scoped actions blocked across tenant boundaries

## Sprint 4: Public Useful Page + Documentation Foundation

### Goals

- provide useful public page
- define Nextra documentation information architecture

### Tasks and Exact Targets

- Public pages:
  - `frontend/src/pages/PublicHome.tsx` (or equivalent route component)
  - `frontend/src/pages/PublicStatus.tsx` (or equivalent route component)
  - `frontend/src/components/public/SystemStatus.tsx`
  - `frontend/src/components/public/OnboardingChecklist.tsx`
- Optional public status API proxy:
  - `frontend/src/lib/api/status.ts`
  - `backend/internal/httpserver/health_handler.go` (only if extra data needed)
- Nextra docs IA/pages:
  - `docs/nextra/pages/index.mdx`
  - `docs/nextra/pages/manuals/index.mdx`

### Done Criteria

- public page is functional (not placeholder-only)
- docs site runs locally

## Sprint 5: User Manuals in Nextra

### Goals

- complete manuals for installation, admin, and admin tenant

### Tasks and Exact Targets

- Installation manual:
  - `docs/nextra/manuals/installation/index.mdx`
  - `docs/nextra/manuals/installation/prerequisites.mdx`
  - `docs/nextra/manuals/installation/local-setup.mdx`
  - `docs/nextra/manuals/installation/deploy-basic.mdx`
  - `docs/nextra/manuals/installation/troubleshooting.mdx`
- Admin manual:
  - `docs/nextra/manuals/admin/index.mdx`
  - `docs/nextra/manuals/admin/tenants.mdx`
  - `docs/nextra/manuals/admin/users-roles.mdx`
  - `docs/nextra/manuals/admin/routes-products.mdx`
  - `docs/nextra/manuals/admin/billing-audit.mdx`
- Admin tenant manual:
  - `docs/nextra/manuals/admin-tenant/index.mdx`
  - `docs/nextra/manuals/admin-tenant/routes.mdx`
  - `docs/nextra/manuals/admin-tenant/credentials.mdx`
  - `docs/nextra/manuals/admin-tenant/usage.mdx`
  - `docs/nextra/manuals/admin-tenant/transformations.mdx`
- Keep architecture references in markdown docs:
  - `docs/markdown/API_SPEC.md`
  - `docs/markdown/SECURITY_DESIGN.md`
  - `docs/markdown/TECHNICAL_DESIGN.md`

### Done Criteria

- manuals are navigable, searchable, and complete for core workflows

## Sprint 6: MCP Server for App Interaction

### Goals

- expose safe MCP tools for operational workflows

### Tasks and Exact Targets

- Create MCP service in backend:
  - `backend/cmd/mcp-server/main.go`
  - `backend/internal/mcp/server.go`
  - `backend/internal/mcp/auth.go`
  - `backend/internal/mcp/tools.go`
  - `backend/internal/mcp/types.go`
- Tool handlers (initial):
  - `backend/internal/mcp/tool_list_tenants.go`
  - `backend/internal/mcp/tool_create_tenant.go`
  - `backend/internal/mcp/tool_list_routes.go`
  - `backend/internal/mcp/tool_create_route.go`
  - `backend/internal/mcp/tool_rotate_api_key.go`
  - `backend/internal/mcp/tool_usage_report.go`
  - `backend/internal/mcp/tool_audit_logs.go`
- Connect MCP handlers to control-plane services:
  - `backend/internal/controlplane/store.go`
  - `backend/internal/controlplane/repository.go`
  - `backend/internal/storage/postgres/controlplane_repository.go`
- Tests:
  - `backend/internal/mcp/server_test.go`
  - `backend/internal/mcp/tools_test.go`

### Done Criteria

- MCP server starts and exposes initial tools
- tool actions respect auth, RBAC, and tenant isolation

## Sprint 7: Hardening and Release Readiness

### Goals

- stabilize critical flows
- finalize docs and operational runbooks

### Tasks and Exact Targets

- Frontend E2E tests:
  - `frontend/tests/e2e/admin-flows.spec.ts`
  - `frontend/tests/e2e/tenant-flows.spec.ts`
  - `frontend/tests/e2e/public-page.spec.ts`
- Backend hardening checks:
  - `backend/internal/controlplane/router_test.go`
  - `backend/internal/httpserver/gateway_handler_test.go`
  - `backend/internal/storage/postgres/runtime_config_source_integration_test.go`
- Operational docs:
  - `backend/runbooks/local-development.md`
  - `backend/runbooks/troubleshooting.md`
  - `backend/runbooks/manual-testing.md`
  - `docs/nextra/manuals/installation/troubleshooting.mdx`

### Done Criteria

- critical user journeys pass E2E
- key backend tests pass
- operator troubleshooting docs are usable

## Cross-Sprint Rules

- Keep tenant isolation explicit in backend APIs and repositories.
- Do not log PAN/CVV/PIN/API keys/tokens/secrets.
- Keep protocol-specific logic behind adapter interfaces.
- Keep docs in sync with shipped behavior each sprint.
