import { useState } from 'react'
import { RouteTable } from '../../../components/tenant/RouteTable'
import { EmptyState } from '../../../components/ui/EmptyState'
import { apiRequest } from '../../../lib/api/client'
import { getAuthSession } from '../../../lib/auth'
import type { ApiListResponse, Route } from '../../../lib/api/types'

export function TenantRoutesPage() {
  const session = getAuthSession()
  const [rows, setRows] = useState<Route[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  if (session.role !== 'tenant_admin' || !session.tenantId) {
    return (
      <EmptyState
        title="Unauthorized"
        description="Tenant admin session is required for route access."
      />
    )
  }

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      const response = await apiRequest<ApiListResponse<Route>>(
        `/admin/v1/tenants/${session.tenantId}/routes`,
        { headers: { Authorization: 'Bearer dev-admin-token' } },
      )
      setRows(response.data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load routes')
      setRows([])
    } finally {
      setLoading(false)
    }
  }

  return (
    <section>
      <h2>Tenant Routes</h2>
      <p>Tenant ID: {session.tenantId}</p>
      <div className="ui-form-actions">
        <button type="button" onClick={() => void load()} disabled={loading}>
          {loading ? 'Loading...' : 'Load Routes'}
        </button>
      </div>
      {error ? <p>{error}</p> : null}
      {!loading && !error && rows.length === 0 ? (
        <EmptyState title="No routes" description="No routes found for this tenant." />
      ) : null}
      {rows.length > 0 ? <RouteTable rows={rows} /> : null}
    </section>
  )
}
