import { Table } from '../ui/Table'
import { StatusBadge } from '../ui/StatusBadge'
import type { Route } from '../../lib/api/types'

type RouteTableProps = {
  rows: Route[]
}

export function RouteTable({ rows }: RouteTableProps) {
  return (
    <Table
      columns={[
        { key: 'name', header: 'Name' },
        { key: 'method', header: 'Method' },
        { key: 'path', header: 'Path' },
        { key: 'inboundProtocol', header: 'Inbound' },
        { key: 'outboundProtocol', header: 'Outbound' },
        {
          key: 'status',
          header: 'Status',
          render: (row) => {
            const value = row.status.toLowerCase()
            const status =
              value === 'active' || value === 'disabled' || value === 'draft'
                ? value === 'draft'
                  ? 'pending'
                  : value === 'disabled'
                    ? 'inactive'
                    : 'active'
                : 'error'
            return <StatusBadge status={status} />
          },
        },
      ]}
      rows={rows}
      rowKey={(row) => row.id}
    />
  )
}
