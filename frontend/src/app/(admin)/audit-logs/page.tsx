import { useState } from 'react'
import { AuditLogTable } from '../../../components/admin/AuditLogTable'
import { EmptyState } from '../../../components/ui/EmptyState'
import { apiRequest } from '../../../lib/api/client'
import type { ApiListResponse, AuditLog } from '../../../lib/api/types'
import { getActiveTenantId } from '../../../lib/adminContext'

export function AdminAuditLogsPage() {
  const [rows, setRows] = useState<AuditLog[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [tenantScoped, setTenantScoped] = useState(false)

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      const tenantId = getActiveTenantId()
      const path =
        tenantScoped && tenantId
          ? `/admin/v1/tenants/${tenantId}/audit-logs`
          : '/admin/v1/audit-logs'
      const response = await apiRequest<ApiListResponse<AuditLog>>(path, {
        headers: { Authorization: 'Bearer dev-admin-token' },
      })
      setRows(response.data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load audit logs')
    } finally {
      setLoading(false)
    }
  }

  return (
    <section>
      <h2>Audit Logs</h2>
      <label>
        <input
          type="checkbox"
          checked={tenantScoped}
          onChange={(event) => setTenantScoped(event.target.checked)}
        />
        Scope to active tenant
      </label>
      <div className="ui-form-actions">
        <button type="button" onClick={() => void load()} disabled={loading}>
          {loading ? 'Loading...' : 'Load Audit Logs'}
        </button>
      </div>
      {loading ? <p>Loading audit logs...</p> : null}
      {error ? <p>{error}</p> : null}
      {!loading && !error && rows.length === 0 ? (
        <EmptyState title="No audit events" description="Actions will appear here after admin operations." />
      ) : null}
      {rows.length > 0 ? <AuditLogTable rows={rows} /> : null}
    </section>
  )
}
