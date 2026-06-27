import { statusClass } from './helpers'

export function StatusBadge({ status }: { status: string }) {
  return <span className={`status-badge ${statusClass(status)}`}>{status}</span>
}
