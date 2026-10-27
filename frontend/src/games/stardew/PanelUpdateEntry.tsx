import { usePanelUpdate } from './PanelUpdateProvider'
import { isPanelUpdateActive } from './panel-update-machine'

export function PanelUpdateEntry() {
  const data = usePanelUpdate()
  const active = isPanelUpdateActive(data.updateApply)
  const available = data.updateStatus?.updateAvailable
  const attention = data.updateApply?.phase === 'rollback_failed' || data.updateApply?.fullStack?.phase === 'manual_action' || ['manual_action', 'scope_changed'].includes(data.fullStackUpgrade?.phase || '')
  const label = attention ? '升级需要处理' : active ? '升级中' : available ? '发现新版本' : '检查更新'
  return <button type="button" className={`game-hub-update${attention ? ' has-attention' : ''}`} onClick={data.openUpdateDialog} aria-label={`${label}，打开更新中心`} title={label}>
    <svg viewBox="0 0 20 20" aria-hidden="true" focusable="false">
      <path d="M9 2h2v2h2v2h2v2h-4v6H9V8H5V6h2V4h2ZM3 13h2v3h10v-3h2v5H3Z" />
    </svg>
    {(available || attention) && !active ? <span className="game-hub-update-dot" aria-hidden="true" /> : null}
  </button>
}
