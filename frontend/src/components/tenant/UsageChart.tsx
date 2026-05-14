import type { BillingSummary } from '../../lib/api/types'

type UsageChartProps = {
  summary: BillingSummary | null
}

function ratio(part: number, total: number) {
  if (total <= 0) {
    return 0
  }
  return Math.round((part / total) * 100)
}

export function UsageChart({ summary }: UsageChartProps) {
  if (!summary) {
    return null
  }

  const billablePct = ratio(summary.billableRequests, summary.totalRequests)
  const overagePct = ratio(summary.overageRequests, summary.totalRequests)

  return (
    <section className="usage-chart">
      <h3>Usage Distribution</h3>
      <div className="usage-row">
        <span>Billable</span>
        <div className="usage-bar">
          <div className="usage-fill active" style={{ width: `${billablePct}%` }} />
        </div>
        <span>{billablePct}%</span>
      </div>
      <div className="usage-row">
        <span>Overage</span>
        <div className="usage-bar">
          <div className="usage-fill warning" style={{ width: `${overagePct}%` }} />
        </div>
        <span>{overagePct}%</span>
      </div>
    </section>
  )
}
