import { useState } from 'react'
import { RouteForm } from '../../../components/admin/RouteForm'
import { EmptyState } from '../../../components/ui/EmptyState'
import { Table } from '../../../components/ui/Table'
import { apiRequest } from '../../../lib/api/client'
import type { ApiListResponse, Route, Upstream } from '../../../lib/api/types'
import { getActiveTenantId } from '../../../lib/adminContext'

export function AdminRoutesPage() {
  const [tenantId, setTenantId] = useState(getActiveTenantId())
  const [routes, setRoutes] = useState<Route[]>([])
  const [upstreams, setUpstreams] = useState<Upstream[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const [upstreamName, setUpstreamName] = useState('')
  const [upstreamProtocol, setUpstreamProtocol] = useState('rest')

  const load = async (activeTenantId: string) => {
    if (!activeTenantId) {
      setRoutes([])
      setUpstreams([])
      return
    }
    setLoading(true)
    setError('')
    try {
      const [routeResp, upstreamResp] = await Promise.all([
        apiRequest<ApiListResponse<Route>>(`/admin/v1/tenants/${activeTenantId}/routes`, {
          headers: { Authorization: 'Bearer dev-admin-token' },
        }),
        apiRequest<ApiListResponse<Upstream>>(`/admin/v1/tenants/${activeTenantId}/upstreams`, {
          headers: { Authorization: 'Bearer dev-admin-token' },
        }),
      ])
      setRoutes(routeResp.data)
      setUpstreams(upstreamResp.data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load routes')
    } finally {
      setLoading(false)
    }
  }

  return (
    <section>
      <h2>Routes</h2>
      <p>Active tenant: {tenantId || 'Not selected'}</p>
      <div className="ui-form-actions">
        <button
          type="button"
          onClick={() => {
            const id = getActiveTenantId()
            setTenantId(id)
            void load(id)
          }}
          disabled={loading}
        >
          {loading ? 'Loading...' : 'Load Routes and Upstreams'}
        </button>
      </div>
      {!tenantId ? <EmptyState title="No tenant selected" description="Go to Tenants and set an active tenant first." /> : null}
      {tenantId ? (
        <>
          <form
            className="ui-form"
            onSubmit={async (event) => {
              event.preventDefault()
              await apiRequest(`/admin/v1/tenants/${tenantId}/upstreams`, {
                method: 'POST',
                headers: { Authorization: 'Bearer dev-admin-token' },
                body: { name: upstreamName, protocol: upstreamProtocol, config: {} },
              })
              setUpstreamName('')
              setUpstreamProtocol('rest')
              await load(tenantId)
            }}
          >
            <h3>Create Upstream</h3>
            <label>
              Name
              <input value={upstreamName} onChange={(event) => setUpstreamName(event.target.value)} />
            </label>
            <label>
              Protocol
              <input value={upstreamProtocol} onChange={(event) => setUpstreamProtocol(event.target.value)} />
            </label>
            <div className="ui-form-actions">
              <button type="submit" disabled={!upstreamName}>
                Create Upstream
              </button>
            </div>
          </form>

          <RouteForm
            onCreate={async (payload) => {
              await apiRequest(`/admin/v1/tenants/${tenantId}/routes`, {
                method: 'POST',
                headers: { Authorization: 'Bearer dev-admin-token' },
                body: payload,
              })
              await load(tenantId)
            }}
          />
        </>
      ) : null}
      {loading ? <p>Loading routes...</p> : null}
      {error ? <p>{error}</p> : null}
      {!loading && !error && tenantId && routes.length === 0 ? (
        <EmptyState title="No routes" description="Create upstream and route records to start traffic flow." />
      ) : null}
      {upstreams.length > 0 ? (
        <>
          <h3>Upstreams</h3>
          <Table
            columns={[
              { key: 'id', header: 'ID' },
              { key: 'name', header: 'Name' },
              { key: 'protocol', header: 'Protocol' },
              { key: 'status', header: 'Status' },
            ]}
            rows={upstreams}
            rowKey={(row) => row.id}
          />
        </>
      ) : null}
      {routes.length > 0 ? (
        <>
          <h3>Routes</h3>
          <Table
            columns={[
              { key: 'id', header: 'ID' },
              { key: 'name', header: 'Name' },
              { key: 'method', header: 'Method' },
              { key: 'path', header: 'Path' },
              { key: 'inboundProtocol', header: 'Inbound' },
              { key: 'outboundProtocol', header: 'Outbound' },
              { key: 'status', header: 'Status' },
            ]}
            rows={routes}
            rowKey={(row) => row.id}
          />
        </>
      ) : null}
    </section>
  )
}
