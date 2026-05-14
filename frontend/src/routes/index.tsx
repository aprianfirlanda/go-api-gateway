import { NavLink, Navigate, Outlet, Route, Routes } from 'react-router-dom'
import type { ReactElement } from 'react'
import { getAuthSession } from '../lib/auth'
import { canAccessRole } from '../lib/rbac'
import { AdminTenantsPage } from '../app/(admin)/tenants/page'
import { AdminUsersPage } from '../app/(admin)/users/page'
import { AdminAPIProductsPage } from '../app/(admin)/api-products/page'
import { AdminRoutesPage } from '../app/(admin)/routes/page'
import { AdminCredentialsPage } from '../app/(admin)/credentials/page'
import { AdminBillingPage } from '../app/(admin)/billing/page'
import { AdminAuditLogsPage } from '../app/(admin)/audit-logs/page'
import { setLocalRole } from '../lib/auth'
import { TenantDashboardPage } from '../app/(tenant)/dashboard/page'
import { TenantRoutesPage } from '../app/(tenant)/routes/page'
import { TenantCredentialsPage } from '../app/(tenant)/credentials/page'
import { TenantUsagePage } from '../app/(tenant)/usage/page'
import { TenantTransformationsPage } from '../app/(tenant)/transformations/page'
import { PublicHome } from '../pages/PublicHome'
import { PublicStatus } from '../pages/PublicStatus'

function RequireRole({
  role,
  children,
}: {
  role: 'platform_admin' | 'tenant_admin'
  children: ReactElement
}) {
  const session = getAuthSession()
  if (!canAccessRole(session, role)) {
    return <Navigate to="/" replace />
  }
  return children
}

function ShellLayout() {
  const session = getAuthSession()
  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="brand">Gateway Console</div>
        <div className="meta">
          <label>
            Role:{' '}
            <select
              value={session.role}
              onChange={(event) => {
                setLocalRole(event.target.value as 'public' | 'platform_admin' | 'tenant_admin')
                window.location.reload()
              }}
            >
              <option value="public">public</option>
              <option value="platform_admin">platform_admin</option>
              <option value="tenant_admin">tenant_admin</option>
            </select>
          </label>
        </div>
      </header>
      <div className="body">
        <aside className="sidebar">
          <nav>
            <NavLink to="/">Public</NavLink>
            <NavLink to="/status">Status</NavLink>
            <NavLink to="/admin/tenants">Admin</NavLink>
            <NavLink to="/tenant/dashboard">Tenant</NavLink>
          </nav>
        </aside>
        <main className="content">
          <Outlet />
        </main>
      </div>
    </div>
  )
}

function AdminHome() {
  return (
    <section>
      <h2>Platform Admin</h2>
      <p>Select an admin module:</p>
      <div className="admin-links">
        <NavLink to="/admin/tenants">Tenants</NavLink>
        <NavLink to="/admin/users">Users</NavLink>
        <NavLink to="/admin/api-products">API Products</NavLink>
        <NavLink to="/admin/routes">Routes</NavLink>
        <NavLink to="/admin/credentials">Credentials</NavLink>
        <NavLink to="/admin/billing">Billing</NavLink>
        <NavLink to="/admin/audit-logs">Audit Logs</NavLink>
      </div>
    </section>
  )
}

function TenantHome() {
  return (
    <section>
      <h2>Tenant Admin</h2>
      <p>Select a tenant module:</p>
      <div className="admin-links">
        <NavLink to="/tenant/dashboard">Dashboard</NavLink>
        <NavLink to="/tenant/routes">Routes</NavLink>
        <NavLink to="/tenant/credentials">Credentials</NavLink>
        <NavLink to="/tenant/usage">Usage</NavLink>
        <NavLink to="/tenant/transformations">Transformations</NavLink>
      </div>
    </section>
  )
}

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<ShellLayout />}>
        <Route path="/" element={<PublicHome />} />
        <Route path="/status" element={<PublicStatus />} />
        <Route
          path="/admin"
          element={
            <RequireRole role="platform_admin">
              <AdminHome />
            </RequireRole>
          }
        />
        <Route
          path="/admin/tenants"
          element={
            <RequireRole role="platform_admin">
              <AdminTenantsPage />
            </RequireRole>
          }
        />
        <Route
          path="/admin/users"
          element={
            <RequireRole role="platform_admin">
              <AdminUsersPage />
            </RequireRole>
          }
        />
        <Route
          path="/admin/api-products"
          element={
            <RequireRole role="platform_admin">
              <AdminAPIProductsPage />
            </RequireRole>
          }
        />
        <Route
          path="/admin/routes"
          element={
            <RequireRole role="platform_admin">
              <AdminRoutesPage />
            </RequireRole>
          }
        />
        <Route
          path="/admin/credentials"
          element={
            <RequireRole role="platform_admin">
              <AdminCredentialsPage />
            </RequireRole>
          }
        />
        <Route
          path="/admin/billing"
          element={
            <RequireRole role="platform_admin">
              <AdminBillingPage />
            </RequireRole>
          }
        />
        <Route
          path="/admin/audit-logs"
          element={
            <RequireRole role="platform_admin">
              <AdminAuditLogsPage />
            </RequireRole>
          }
        />
        <Route
          path="/tenant"
          element={
            <RequireRole role="tenant_admin">
              <TenantHome />
            </RequireRole>
          }
        />
        <Route
          path="/tenant/dashboard"
          element={
            <RequireRole role="tenant_admin">
              <TenantDashboardPage />
            </RequireRole>
          }
        />
        <Route
          path="/tenant/routes"
          element={
            <RequireRole role="tenant_admin">
              <TenantRoutesPage />
            </RequireRole>
          }
        />
        <Route
          path="/tenant/credentials"
          element={
            <RequireRole role="tenant_admin">
              <TenantCredentialsPage />
            </RequireRole>
          }
        />
        <Route
          path="/tenant/usage"
          element={
            <RequireRole role="tenant_admin">
              <TenantUsagePage />
            </RequireRole>
          }
        />
        <Route
          path="/tenant/transformations"
          element={
            <RequireRole role="tenant_admin">
              <TenantTransformationsPage />
            </RequireRole>
          }
        />
      </Route>
    </Routes>
  )
}
