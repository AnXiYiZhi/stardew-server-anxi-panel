import type { InstanceState } from '../../types'
import { StatusBadge } from '../../core/StatusBadge'
import { formatDate } from '../../core/helpers'

export function InstanceStateCard({ state, onRefresh }: { state: InstanceState | null; onRefresh: () => void }) {
  return (
    <div className="status-card instance-state-card">
      <div>
        <span>{state?.name ?? 'Stardew Valley'} 实例状态</span>
        <strong>{state?.state ?? 'unknown'}</strong>
        <small>{state?.driverId ? `Driver: ${state.driverId}` : 'Driver: stardew_junimo'}</small>
        <small>{state?.stateMessage ?? '尚未读取实例状态。'}</small>
      </div>
      <StatusBadge status={state?.state ?? 'unknown'} />
      <small>更新时间：{state?.updatedAt ? formatDate(state.updatedAt) : '未读取'}</small>
      <button className="button button-small button-secondary" onClick={onRefresh} type="button">刷新状态</button>
    </div>
  )
}
