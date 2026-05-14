import { useState } from 'react'
import { fetchSystemStatus, type ServiceStatus } from '../../lib/api/status'
import { StatusBadge } from '../ui/StatusBadge'
import { Table } from '../ui/Table'

export function SystemStatus() {
  const [rows, setRows] = useState<ServiceStatus[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const load = async () => {
    setLoading(true)
    setError('')
    try {
      const result = await fetchSystemStatus()
      setRows(result)
    } catch (err) {
      setRows([])
      setError(err instanceof Error ? err.message : 'Failed to load system status')
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className="panel">
      <div className="panel-header">
        <h3>System Status</h3>
        <button type="button" onClick={() => void load()} disabled={loading}>
          {loading ? 'Checking...' : 'Check Status'}
        </button>
      </div>
      {error ? <p>{error}</p> : null}
      {rows.length > 0 ? (
        <Table
          columns={[
            { key: 'service', header: 'Service' },
            {
              key: 'health',
              header: 'Health',
              render: (row) => <StatusBadge status={row.health === 'healthy' ? 'active' : 'error'} />,
            },
            {
              key: 'ready',
              header: 'Readiness',
              render: (row) => <StatusBadge status={row.ready === 'ready' ? 'active' : 'pending'} />,
            },
            { key: 'checkedAt', header: 'Checked At' },
            { key: 'error', header: 'Error' },
          ]}
          rows={rows}
          rowKey={(row) => row.service}
        />
      ) : (
        <p>No status loaded yet.</p>
      )}
    </section>
  )
}
