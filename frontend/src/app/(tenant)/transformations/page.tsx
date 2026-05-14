import { useState } from 'react'
import { TransformationEditor } from '../../../components/tenant/TransformationEditor'
import { EmptyState } from '../../../components/ui/EmptyState'
import { Table } from '../../../components/ui/Table'
import { apiRequest } from '../../../lib/api/client'
import { controlPlaneAuthHeaders, getAuthSession } from '../../../lib/auth'
import type { ApiListResponse, TransformationTemplate } from '../../../lib/api/types'

export function TenantTransformationsPage() {
  const session = getAuthSession()
  const [rows, setRows] = useState<TransformationTemplate[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  if (session.role !== 'tenant_admin' || !session.tenantId) {
    return (
      <EmptyState
        title="Unauthorized"
        description="Tenant admin session is required for transformation access."
      />
    )
  }

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      const response = await apiRequest<ApiListResponse<TransformationTemplate>>(
        `/admin/v1/tenants/${session.tenantId}/transformation-templates`,
        { headers: controlPlaneAuthHeaders(session) },
      )
      setRows(response.data)
    } catch (err) {
      setRows([])
      setError(err instanceof Error ? err.message : 'Failed to load templates')
    } finally {
      setLoading(false)
    }
  }

  return (
    <section>
      <h2>Tenant Transformations</h2>
      <p>Tenant ID: {session.tenantId}</p>
      <TransformationEditor
        onCreate={async (payload) => {
          await apiRequest(`/admin/v1/tenants/${session.tenantId}/transformation-templates`, {
            method: 'POST',
            headers: controlPlaneAuthHeaders(session),
            body: payload,
          })
          await load()
        }}
      />
      <div className="ui-form-actions">
        <button type="button" onClick={() => void load()} disabled={loading}>
          {loading ? 'Loading...' : 'Load Templates'}
        </button>
      </div>
      {error ? <p>{error}</p> : null}
      {!loading && !error && rows.length === 0 ? (
        <EmptyState title="No templates" description="Create or load transformation templates." />
      ) : null}
      {rows.length > 0 ? (
        <Table
          columns={[
            { key: 'id', header: 'ID' },
            { key: 'name', header: 'Name' },
            { key: 'sourceProtocol', header: 'Source' },
            { key: 'targetProtocol', header: 'Target' },
            { key: 'version', header: 'Version' },
            { key: 'status', header: 'Status' },
          ]}
          rows={rows}
          rowKey={(row) => row.id}
        />
      ) : null}
    </section>
  )
}
