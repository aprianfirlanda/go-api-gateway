import { useState } from 'react'
import { TenantForm } from '../../../components/admin/TenantForm'
import { EmptyState } from '../../../components/ui/EmptyState'
import { Table } from '../../../components/ui/Table'
import { apiRequest } from '../../../lib/api/client'
import type { ApiListResponse, Tenant } from '../../../lib/api/types'
import { getActiveTenantId, setActiveTenantId } from '../../../lib/adminContext'

export function AdminTenantsPage() {
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [activeTenantId, setActiveTenant] = useState(getActiveTenantId())

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      const response = await apiRequest<ApiListResponse<Tenant>>('/admin/v1/tenants', {
        headers: { Authorization: 'Bearer dev-admin-token' },
      })
      setTenants(response.data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load tenants')
    } finally {
      setLoading(false)
    }
  }

  return (
    <section>
      <h2>Tenants</h2>
      <div className="ui-form-actions">
        <button type="button" onClick={() => void load()} disabled={loading}>
          {loading ? 'Loading...' : 'Load Tenants'}
        </button>
      </div>
      <TenantForm
        onCreate={async (payload) => {
          await apiRequest('/admin/v1/tenants', {
            method: 'POST',
            headers: { Authorization: 'Bearer dev-admin-token' },
            body: payload,
          })
          await load()
        }}
      />
      {loading ? <p>Loading tenants...</p> : null}
      {error ? <p>{error}</p> : null}
      {!loading && !error && tenants.length === 0 ? (
        <EmptyState title="No tenants" description="Create a tenant to continue with admin workflows." />
      ) : null}
      {tenants.length > 0 ? (
        <Table
          columns={[
            { key: 'name', header: 'Name' },
            { key: 'slug', header: 'Slug' },
            { key: 'status', header: 'Status' },
            {
              key: 'actions',
              header: 'Actions',
              render: (row) => (
                <button
                  type="button"
                  onClick={() => {
                    setActiveTenantId(row.id)
                    setActiveTenant(row.id)
                  }}
                >
                  {activeTenantId === row.id ? 'Selected' : 'Set Active'}
                </button>
              ),
            },
          ]}
          rows={tenants}
          rowKey={(row) => row.id}
        />
      ) : null}
    </section>
  )
}
