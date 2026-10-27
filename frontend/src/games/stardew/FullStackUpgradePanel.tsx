import { usePanelUpdate } from './PanelUpdateProvider'
import { fullStackPhaseLabel } from './full-stack-state'
import { hasGameUpgradeFlow } from './panel-update-machine'

type UpgradeActionsProps = { noticeDigest?: string; noticesAccepted?: boolean; compact?: boolean }

export function FullStackUpgradeActions({ noticeDigest, noticesAccepted = true, compact = false }: UpgradeActionsProps) {
  const data=usePanelUpdate()
  const op=data.fullStackUpgrade
  const target=data.updateStatus?.updateAvailable ? data.updateStatus.latestVersion : data.versionInfo?.version
  const preparing=op?.phase==='preparing'
  const authorized=Boolean(op?.authorizedAt && !['succeeded','failed_safe','panel_failed_rolled_back'].includes(op.phase))
  const future=target?.replace(/^v/,'')!==data.versionInfo?.version.replace(/^v/,'')
  const canPrepare=Boolean(target) && (!future || data.updateDryRun?.phase==='succeeded') && !preparing && !authorized
  if (!hasGameUpgradeFlow(data.versionInfo, op, data.updateStatus)) return null
  return <div className="sd-update-actions">
    {op?.phase==='awaiting_confirmation' || op?.phase==='scope_changed' ? <button type="button" className="sd-update-dismiss" disabled={data.fullStackBusy} onClick={()=>void data.cancelFullStack()}>{compact ? '取消本次准备' : '撤销本次授权'}</button> : null}
    {op?.phase==='awaiting_confirmation' ? <button type="button" className="sd-update-apply-button" disabled={data.fullStackBusy || !noticesAccepted} onClick={()=>void data.confirmFullStack(noticeDigest)}>{compact ? '确认范围并升级' : '确认上述目标和世界范围，开始升级'}</button> : op?.phase==='manual_action' ? <button type="button" className="sd-update-apply-button" disabled={data.fullStackBusy} onClick={()=>void data.recoverFullStack()}>{compact ? '继续恢复' : '恢复本次游戏组事务'}</button> : op?.phase!=='scope_changed' ? <button type="button" className="sd-update-apply-button" disabled={!canPrepare || data.fullStackBusy} onClick={()=>{if(target) void data.prepareFullStack(target)}}>{compact ? authorized ? '升级中…' : preparing ? '正在检查…' : '检查升级范围' : preparing ? '下载预检中…' : '下载并预检全栈升级'}</button> : null}
  </div>
}

export function FullStackUpgradePanel({ noticeDigest, noticesAccepted = true, readOnly = false, compact = false, hideActions = false }: UpgradeActionsProps & { readOnly?: boolean; hideActions?: boolean }) {
  const data=usePanelUpdate()
  const op=data.fullStackUpgrade
  const target=data.updateStatus?.updateAvailable ? data.updateStatus.latestVersion : data.versionInfo?.version
  const future=target?.replace(/^v/,'')!==data.versionInfo?.version.replace(/^v/,'')
  if (!hasGameUpgradeFlow(data.versionInfo, op, data.updateStatus)) return null
  return <section className="sd-update-dry-run" aria-label="Panel 与 Junimo 统一升级" aria-live="polite">
    <strong>{compact ? '升级范围' : 'Panel 与 Junimo .140 统一升级'}</strong>
    {!compact ? <p>下载与预检先完成。确认后会备份 Panel 数据、更新 Panel，再逐组保存并停服、完整备份、迁移和验收；原来运行的世界恢复运行，停止世界与空模板保持原选择。</p> : !op ? <p>先检查需要更新的世界，确认范围后再升级。</p> : null}
    {!future && !compact ? <p>当前 Panel 已就绪；这里将继续完成游戏运行栈的范围预检与明确确认。</p> : null}
    {op ? <>
      <p><strong>{compact && op.phase==='awaiting_confirmation' ? '确认以下世界后开始更新' : fullStackPhaseLabel(op.phase)}</strong></p>
      {!compact ? <dl className="sd-update-details">
        <div><dt>目标 Panel</dt><dd>{op.panel.version}</dd></div>
        <div><dt>游戏运行栈</dt><dd>{op.runtimeVersion} · Control {op.controlVersion}</dd></div>
        <div><dt>影响范围</dt><dd>{op.groups.length} 个共享组 · {op.groups.reduce((n,g)=>n+g.members.length,0)} 个世界</dd></div>
        <div><dt>运行世界</dt><dd>{op.groups.reduce((n,g)=>n+g.runningWorlds,0)} 个将保存并重启</dd></div>
      </dl> : <p className="sd-update-scope-summary">{op.groups.reduce((n,g)=>n+g.members.length,0)} 个世界{op.groups.some(g=>g.runningWorlds>0) ? ` · ${op.groups.reduce((n,g)=>n+g.runningWorlds,0)} 个运行中` : ''}</p>}
      {op.groups.map(group=><div key={group.applyId}>
        <strong>{group.members.map(id=>group.memberNames[id]||id).join('、')}</strong>
        <p>{compact ? group.runningWorlds>0 ? `${group.runningWorlds} 个运行中将保存并重启${group.members.length>group.runningWorlds ? '，其余保持原选择' : ''}` : '升级后保持当前停止或未启动状态' : `${group.members.length} 个世界 · 原有 ${group.runningWorlds} 个运行中 · ${group.coldTemplates} 个空模板 · ${fullStackPhaseLabel(group.phase)}`}</p>
        {op.authorizedAt ? <p>已保存 {group.savedWorlds} · 已静止 {group.stoppedWorlds} · 已替换 {group.updatedWorlds}</p> : null}
      </div>)}
      <details className="sd-update-environment-details"><summary>{compact ? '组件与校验信息' : '查看精确制品与确认身份'}</summary>
        <dl>
          <div><dt>Panel 内容</dt><dd><code>{op.panel.imageId}</code></dd></div>
          <div><dt>Panel commit</dt><dd><code>{op.panel.revision}</code></dd></div>
          <div><dt>构建时间</dt><dd>{op.panel.buildDate}</dd></div>
          <div><dt>Junimo digest</dt><dd><code>{op.runtimeDigest}</code></dd></div>
          {op.authVersion ? <div><dt>Steam/GOG 认证</dt><dd>{op.authVersion}{op.authDigest ? <> · <code>{op.authDigest}</code></> : null}</dd></div> : null}
          <div><dt>游戏 / SDK build</dt><dd>{op.gameBuildId} / {op.sdkBuildId}</dd></div>
          <div><dt>SMAPI</dt><dd>{op.smapiIdentity}</dd></div>
          <div><dt>Control SHA256</dt><dd><code>{op.controlSha256}</code></dd></div>
          <div><dt>操作</dt><dd><code>{op.operationId}</code></dd></div>
          <div><dt>范围与目标证明</dt><dd><code>{op.planSha256}</code></dd></div>
        </dl>
      </details>
      {op.error ? <div className="sd-update-error">{op.error}</div> : null}
    </> : null}
    {data.fullStackError ? <div className="sd-update-error">{data.fullStackError}</div> : null}
    {!readOnly && !hideActions ? <FullStackUpgradeActions noticeDigest={noticeDigest} noticesAccepted={noticesAccepted} compact={compact} /> : null}
    {!compact ? <p>Panel 连接恢复后会继续显示同一操作。游戏组全部达到持久成功终态后，才显示整次升级完成。</p> : null}
  </section>
}
