import type { BillingSummary } from '../../lib/api/types'
import { Table } from '../ui/Table'

type BillingSummaryTableProps = {
  summary: BillingSummary | null
}

export function BillingSummaryTable({ summary }: BillingSummaryTableProps) {
  if (!summary) {
    return null
  }

  return (
    <Table
      columns={[
        { key: 'billingPeriod', header: 'Period' },
        { key: 'totalRequests', header: 'Total Requests' },
        { key: 'billableRequests', header: 'Billable Requests' },
        { key: 'overageRequests', header: 'Overage Requests' },
        { key: 'estimatedAmount', header: 'Estimated Amount' },
        { key: 'currency', header: 'Currency' },
        { key: 'status', header: 'Status' },
      ]}
      rows={[summary]}
      rowKey={() => summary.tenantId + summary.billingPeriod}
    />
  )
}
