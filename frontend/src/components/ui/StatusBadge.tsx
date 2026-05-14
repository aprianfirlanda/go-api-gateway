type Status = 'active' | 'inactive' | 'error' | 'pending'

const labelByStatus: Record<Status, string> = {
  active: 'Active',
  inactive: 'Inactive',
  error: 'Error',
  pending: 'Pending',
}

type StatusBadgeProps = {
  status: Status
}

export function StatusBadge({ status }: StatusBadgeProps) {
  return <span className={`status-badge ${status}`}>{labelByStatus[status]}</span>
}
