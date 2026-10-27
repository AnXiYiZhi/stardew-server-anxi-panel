import type { CurrentUser, InstanceState } from '../../types'
import type { StardewRoute } from './stardew-routes'
import { usePanelUpdate } from './PanelUpdateProvider'
import { fullStackPhaseLabel } from './full-stack-state'

export function FullStackMaintenanceView({ user,maintenance,onNavigate }:{user:CurrentUser;maintenance:NonNullable<InstanceState['maintenance']>;onNavigate:(route:StardewRoute)=>void}) {
  const updates=usePanelUpdate()
  return <section className="sd-placeholder-card" aria-label="世界升级维护">
    <h2>此世界正在升级或恢复</h2>
    <p>{maintenance.message}</p>
    <p>{fullStackPhaseLabel(maintenance.phase)}</p>
    <p>操作：<code>{maintenance.operationId}</code></p>
    <div className="sd-update-actions">
      {user.role==='admin' ? <button type="button" className="sd-btn-green" onClick={updates.openUpdateDialog}>查看全栈升级进度</button> : <span>详细恢复由管理员处理。</span>}
      <button type="button" className="sd-btn-green" onClick={()=>onNavigate('jobs')}>查看任务日志</button>
      <button type="button" className="sd-btn-green" onClick={()=>onNavigate('diagnostics')}>查看诊断</button>
    </div>
  </section>
}
