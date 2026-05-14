import type { AuditLog } from '../../lib/api/types'
import { Table } from '../ui/Table'

type AuditLogTableProps = {
  rows: AuditLog[]
}

export function AuditLogTable({ rows }: AuditLogTableProps) {
  return (
    <Table
      columns={[
        { key: 'occurredAt', header: 'Occurred At' },
        { key: 'actorId', header: 'Actor' },
        { key: 'tenantId', header: 'Tenant' },
        { key: 'action', header: 'Action' },
        { key: 'resource', header: 'Resource' },
        { key: 'resourceId', header: 'Resource ID' },
      ]}
      rows={rows}
      rowKey={(row) => row.id}
    />
  )
}
