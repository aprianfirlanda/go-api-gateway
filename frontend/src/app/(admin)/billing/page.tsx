import { useState } from 'react'
import { BillingSummaryTable } from '../../../components/admin/BillingSummaryTable'
import { EmptyState } from '../../../components/ui/EmptyState'
import { apiRequest } from '../../../lib/api/client'
import type { BillingSummary } from '../../../lib/api/types'
import { getActiveTenantId } from '../../../lib/adminContext'

function defaultPeriod() {
  const now = new Date()
  const month = String(now.getUTCMonth() + 1).padStart(2, '0')
  return `${now.getUTCFullYear()}-${month}`
}

export function AdminBillingPage() {
  const tenantId = getActiveTenantId()
  const [period, setPeriod] = useState(defaultPeriod())
  const [summary, setSummary] = useState<BillingSummary | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  if (!tenantId) {
    return <EmptyState title="No tenant selected" description="Go to Tenants and set an active tenant first." />
  }

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      const data = await apiRequest<BillingSummary>(
        `/admin/v1/tenants/${tenantId}/billing-summaries/${period}`,
        { headers: { Authorization: 'Bearer dev-admin-token' } },
      )
      setSummary(data)
    } catch (err) {
      setSummary(null)
      setError(err instanceof Error ? err.message : 'Failed to load billing summary')
    } finally {
      setLoading(false)
    }
  }

  return (
    <section>
      <h2>Billing Summaries</h2>
      <p>Active tenant: {tenantId}</p>
      <div className="toolbar">
        <label>
          Billing period (YYYY-MM)
          <input value={period} onChange={(event) => setPeriod(event.target.value)} />
        </label>
        <button type="button" onClick={() => void load()} disabled={loading}>
          {loading ? 'Loading...' : 'Load Summary'}
        </button>
        <button
          type="button"
          disabled={loading}
          onClick={async () => {
            setLoading(true)
            setError('')
            try {
              await apiRequest(`/admin/v1/tenants/${tenantId}/billing-summaries/${period}/recalculate`, {
                method: 'POST',
                headers: { Authorization: 'Bearer dev-admin-token' },
              })
              await load()
            } catch (err) {
              setError(err instanceof Error ? err.message : 'Failed to recalculate summary')
              setLoading(false)
            }
          }}
        >
          Recalculate
        </button>
      </div>
      {error ? <p>{error}</p> : null}
      {!error && !summary ? <EmptyState title="No summary loaded" description="Load or recalculate a billing period summary." /> : null}
      <BillingSummaryTable summary={summary} />
    </section>
  )
}
