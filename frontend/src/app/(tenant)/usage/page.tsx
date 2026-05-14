import { useState } from 'react'
import { BillingSummaryTable } from '../../../components/admin/BillingSummaryTable'
import { UsageChart } from '../../../components/tenant/UsageChart'
import { EmptyState } from '../../../components/ui/EmptyState'
import { apiRequest } from '../../../lib/api/client'
import { controlPlaneAuthHeaders, getAuthSession } from '../../../lib/auth'
import type { BillingSummary } from '../../../lib/api/types'

function defaultPeriod() {
  const now = new Date()
  const month = String(now.getUTCMonth() + 1).padStart(2, '0')
  return `${now.getUTCFullYear()}-${month}`
}

export function TenantUsagePage() {
  const session = getAuthSession()
  const [period, setPeriod] = useState(defaultPeriod())
  const [summary, setSummary] = useState<BillingSummary | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  if (session.role !== 'tenant_admin' || !session.tenantId) {
    return (
      <EmptyState
        title="Unauthorized"
        description="Tenant admin session is required for usage access."
      />
    )
  }

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      const data = await apiRequest<BillingSummary>(
        `/admin/v1/tenants/${session.tenantId}/billing-summaries/${period}`,
        { headers: controlPlaneAuthHeaders(session) },
      )
      setSummary(data)
    } catch (err) {
      setSummary(null)
      setError(err instanceof Error ? err.message : 'Failed to load usage summary')
    } finally {
      setLoading(false)
    }
  }

  return (
    <section>
      <h2>Tenant Usage</h2>
      <p>Tenant ID: {session.tenantId}</p>
      <div className="toolbar">
        <label>
          Billing period (YYYY-MM)
          <input value={period} onChange={(event) => setPeriod(event.target.value)} />
        </label>
        <button type="button" onClick={() => void load()} disabled={loading}>
          {loading ? 'Loading...' : 'Load Usage'}
        </button>
      </div>
      {error ? <p>{error}</p> : null}
      {!error && !summary ? <EmptyState title="No usage data" description="Load a billing period first." /> : null}
      <UsageChart summary={summary} />
      <BillingSummaryTable summary={summary} />
    </section>
  )
}
