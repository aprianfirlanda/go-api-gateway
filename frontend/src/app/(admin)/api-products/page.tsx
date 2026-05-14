import { useState } from 'react'
import { EmptyState } from '../../../components/ui/EmptyState'
import { Table } from '../../../components/ui/Table'
import { apiRequest } from '../../../lib/api/client'
import type { APIProduct, ApiListResponse } from '../../../lib/api/types'
import { getActiveTenantId } from '../../../lib/adminContext'

export function AdminAPIProductsPage() {
  const [tenantId, setTenantId] = useState(getActiveTenantId())
  const [products, setProducts] = useState<APIProduct[]>([])
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const load = async (currentTenantId: string) => {
    if (!currentTenantId) {
      setProducts([])
      return
    }
    setLoading(true)
    setError('')
    try {
      const response = await apiRequest<ApiListResponse<APIProduct>>(
        `/admin/v1/tenants/${currentTenantId}/api-products`,
        { headers: { Authorization: 'Bearer dev-admin-token' } },
      )
      setProducts(response.data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load API products')
    } finally {
      setLoading(false)
    }
  }

  return (
    <section>
      <h2>API Products</h2>
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
          {loading ? 'Loading...' : 'Load API Products'}
        </button>
      </div>
      {!tenantId ? <EmptyState title="No tenant selected" description="Go to Tenants and set an active tenant first." /> : null}
      {tenantId ? (
        <form
          className="ui-form"
          onSubmit={async (event) => {
            event.preventDefault()
            await apiRequest(`/admin/v1/tenants/${tenantId}/api-products`, {
              method: 'POST',
              headers: { Authorization: 'Bearer dev-admin-token' },
              body: { name, slug },
            })
            setName('')
            setSlug('')
            await load(tenantId)
          }}
        >
          <h3>Create API Product</h3>
          <label>
            Name
            <input value={name} onChange={(event) => setName(event.target.value)} />
          </label>
          <label>
            Slug
            <input value={slug} onChange={(event) => setSlug(event.target.value)} />
          </label>
          <div className="ui-form-actions">
            <button type="submit" disabled={!name || !slug}>
              Create
            </button>
          </div>
        </form>
      ) : null}
      {loading ? <p>Loading API products...</p> : null}
      {error ? <p>{error}</p> : null}
      {!loading && !error && tenantId && products.length === 0 ? (
        <EmptyState title="No API products" description="Create an API product for route assignment." />
      ) : null}
      {products.length > 0 ? (
        <Table
          columns={[
            { key: 'id', header: 'ID' },
            { key: 'name', header: 'Name' },
            { key: 'slug', header: 'Slug' },
            { key: 'status', header: 'Status' },
          ]}
          rows={products}
          rowKey={(row) => row.id}
        />
      ) : null}
    </section>
  )
}
