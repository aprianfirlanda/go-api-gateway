import { getAuthSession } from '../../../lib/auth'
import { EmptyState } from '../../../components/ui/EmptyState'

export function TenantDashboardPage() {
  const session = getAuthSession()

  if (session.role !== 'tenant_admin' || !session.tenantId) {
    return (
      <EmptyState
        title="Unauthorized"
        description="Tenant admin session is required for dashboard access."
      />
    )
  }

  return (
    <section>
      <h2>Tenant Dashboard</h2>
      <p>Tenant ID: {session.tenantId}</p>
      <p>Use tenant routes to manage routes, credentials, usage, and transformations.</p>
    </section>
  )
}
