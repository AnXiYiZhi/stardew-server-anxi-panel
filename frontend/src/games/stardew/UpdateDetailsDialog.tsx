import { useState } from 'react'
import type { CurrentUser } from '../../types'
import type { StardewDashboardData } from './stardew-routes'
import { updateDisplayKind, updateSummaryText, withVersionPrefix } from './update-status'
import { canStartPanelUpdate, isPanelUpdateActive, panelUpdatePhaseLabel, panelUpdateSurface } from './panel-update-machine'
import './UpdateDetailsDialog.css'

type UpdateDetailsDialogProps = {
  user: CurrentUser
  dashboardData: StardewDashboardData
}

function formatDate(value: string | null | undefined): string {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(date)
}

function dryRunPhaseLabel(phase: string): string {
  switch (phase) {
    case 'starting': return '正在启动演练 helper'
    case 'running': return '正在检查镜像与 Compose 配置'
    case 'succeeded': return '升级环境演练通过'
    case 'unsupported': return '当前部署模式不支持安全升级'
    case 'failed': return '升级环境演练未通过'
    default: return phase
  }
}

export function UpdateDetailsDialog({ user, dashboardData }: UpdateDetailsDialogProps) {
  const [confirmApply, setConfirmApply] = useState(false)
  if (!dashboardData.updateDialogOpen) return null
  const status = dashboardData.updateStatus
  const displayKind = updateDisplayKind(status)
  const apply = dashboardData.updateApply
  const surface = panelUpdateSurface(status, apply, dashboardData.versionInfo)
  const currentVersion = surface.currentVersion
  const latestVersion = surface.targetVersion
  const isAdmin = user.role === 'admin'
  const dryRun = dashboardData.updateDryRun
  const dryRunBusy = dashboardData.updateDryRunChecking || dryRun?.phase === 'starting' || dryRun?.phase === 'running'
  const applyActive = isPanelUpdateActive(apply)
	const canApply = canStartPanelUpdate(user, status, dryRun, apply)
	const onlineHumans = dashboardData.players?.players.filter((player) => player.status === 'online' && !player.isHost).length
	const updatePhases = [
		'backing_up', 'pulling', 'recreating', 'waiting_health',
		'checking_runtime', 'notifying_players', 'saving_game', 'backing_up_save', 'updating_runtime', 'verifying_runtime', 'restoring_server',
		...apply && ['rolling_back', 'failed_rolled_back', 'rollback_failed'].includes(apply.phase) ? ['rolling_back'] : [],
	]
	const effectivePhase = apply?.fullStack?.phase || apply?.phase || ''
	const effectiveProgress = apply?.fullStack?.progress ?? apply?.progress ?? 0
	const currentPhaseIndex = updatePhases.indexOf(effectivePhase)
  const closeDialog = () => {
    setConfirmApply(false)
    dashboardData.closeUpdateDialog()
  }

  return (
    <div className="sd-update-overlay" role="presentation" onMouseDown={closeDialog}>
      <section
        className="sd-update-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="sd-update-dialog-title"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="sd-update-dialog-head">
          <div>
            <p className="sd-update-dialog-kicker">PANEL UPDATE</p>
            <h2 id="sd-update-dialog-title">面板版本详情</h2>
          </div>
          <button type="button" className="sd-update-close" onClick={closeDialog} aria-label="关闭版本详情">×</button>
        </div>

        <div className={`sd-update-state sd-update-state--${apply ? surface.tone : displayKind}`}>
          <span className="sd-update-state-dot" aria-hidden="true" />
          <span>{apply ? surface.topbarText : updateSummaryText(status)}</span>
        </div>

        <dl className="sd-update-details">
          <div><dt>当前版本</dt><dd>{withVersionPrefix(currentVersion)}</dd></div>
          <div><dt>最新版本</dt><dd>{latestVersion ? withVersionPrefix(latestVersion) : '—'}</dd></div>
          <div><dt>发布时间</dt><dd>{formatDate(status?.publishedAt)}</dd></div>
          <div><dt>检查时间</dt><dd>{formatDate(status?.checkedAt)}</dd></div>
          <div><dt>检查状态</dt><dd>{status?.checkStatus ?? 'pending'}</dd></div>
        </dl>

        {status?.checkError || dashboardData.updateError ? (
          <div className="sd-update-error">{status?.checkError || dashboardData.updateError}</div>
        ) : null}

		<div className="sd-update-boundary">
			<strong>这是一键全栈升级。</strong>Panel 更新后会检查全部实例的 Control 版本与 DLL hash；不匹配的运行中实例将保存、整档备份、停服、同步 Control、重启并验证 SMAPI 实际加载版本。Panel 健康失败会自动恢复旧容器。
        </div>

        <section className="sd-update-release-notes">
          <strong>更新说明</strong>
          <p>详细变更记录由正式 Release 提供。升级前可打开 Release 页面查看本版本修复与兼容说明。</p>
          {status?.releaseUrl ? <a href={status.releaseUrl} target="_blank" rel="noreferrer">查看 {withVersionPrefix(latestVersion)} Release 说明</a> : null}
        </section>

        {isAdmin && dryRun ? (
          <section className={`sd-update-dry-run sd-update-dry-run--${dryRun.phase}`} aria-live="polite">
            <div className="sd-update-dry-run-title">
              <strong>{dryRunPhaseLabel(dryRun.phase)}</strong>
              <code>{dryRun.capability.code || dryRun.errorCode}</code>
            </div>
            <p>{dryRun.capability.reason || dryRun.error}</p>
            {dryRun.capability.composeProject || dryRun.logs.length > 0 ? (
              <details className="sd-update-environment-details">
                <summary>查看环境详情</summary>
                {dryRun.capability.composeProject ? (
                  <dl>
                    <div><dt>Compose 项目</dt><dd>{dryRun.capability.composeProject}</dd></div>
                    <div><dt>Compose 服务</dt><dd>{dryRun.capability.composeService || '—'}</dd></div>
                    <div><dt>当前容器</dt><dd>{dryRun.capability.currentContainer || '—'}</dd></div>
                    <div><dt>当前镜像</dt><dd>{dryRun.capability.currentImage || '—'}</dd></div>
                    <div><dt>目标镜像</dt><dd>{dryRun.targetImage || '检查中…'}</dd></div>
                  </dl>
                ) : null}
                {dryRun.logs.length > 0 ? (
                  <div className="sd-update-dry-run-logs">
                    {dryRun.logs.map((entry, index) => (
                      <div key={`${entry.at}-${index}`} className={`sd-update-dry-run-log sd-update-dry-run-log--${entry.level}`}>
                        <time>{formatDate(entry.at)}</time><span>{entry.message}</span>
                      </div>
                    ))}
                  </div>
                ) : null}
              </details>
            ) : null}
          </section>
        ) : null}

        <section className={`sd-update-capability sd-update-capability--${dryRun?.phase ?? 'unknown'}`}>
          <strong>升级环境</strong>
          <span>
            {!isAdmin ? '部署能力仅管理员可检查' : dryRun?.phase === 'succeeded' ? '✓ 支持安全升级' : dryRun?.phase === 'unsupported' ? '当前部署不支持 Web 升级' : dryRun?.phase === 'failed' ? '环境检查未通过' : dryRunBusy ? '正在检查…' : '尚未检查'}
          </span>
        </section>

        {isAdmin && dryRun?.phase === 'succeeded' && dryRun.capability.conversionRequired ? (
          <div className="sd-update-boundary">
            <strong>将先转换为标准部署。</strong> 系统会反查当前容器、镜像、数据挂载与旧配置，备份 Compose、环境变量和旧镜像 digest，再由独立 helper 重建 Panel；新 Panel 健康检查失败会自动恢复旧容器。
          </div>
        ) : null}

        {isAdmin && dashboardData.updateDryRunError ? (
          <div className="sd-update-error">{dashboardData.updateDryRunError}</div>
        ) : null}

        {isAdmin && apply ? (
          <section className={`sd-update-apply sd-update-apply--${apply.phase}`} aria-live="polite">
			<div><strong>{panelUpdatePhaseLabel(effectivePhase)}</strong><span>{effectiveProgress}%</span></div>
            <p>{withVersionPrefix(apply.fromVersion)} → {withVersionPrefix(apply.toVersion)}</p>
			<progress max="100" value={effectiveProgress} />
            <ol className="sd-update-timeline">
              {updatePhases.map((phase, index) => (
                <li
                  key={phase}
				className={index < currentPhaseIndex ? 'is-done' : index === currentPhaseIndex ? 'is-active' : effectivePhase === 'succeeded' || effectivePhase === 'not_needed' ? 'is-done' : ''}
                >
                  {panelUpdatePhaseLabel(phase)}
                </li>
              ))}
            </ol>
			{apply.fullStack?.result || apply.result ? <p>{apply.fullStack?.result || apply.result}</p> : null}
			{apply.fullStack?.error || apply.error ? <div className="sd-update-error">{apply.fullStack?.error || apply.error}</div> : null}
			{apply.fullStack?.instances?.length ? (
				<details className="sd-update-environment-details">
					<summary>查看全部实例同步状态</summary>
					<dl>
						{apply.fullStack.instances.map((instance) => (
							<div key={instance.instanceId}>
								<dt>{instance.instanceId}</dt>
								<dd>{panelUpdatePhaseLabel(instance.phase)} · {instance.progress}%{instance.error ? ` · ${instance.error}` : ''}</dd>
							</div>
						))}
					</dl>
				</details>
			) : null}
          </section>
        ) : null}

        {isAdmin && dashboardData.updateApplyError ? <div className="sd-update-error">{dashboardData.updateApplyError}</div> : null}

        <div className="sd-update-actions">
          {isAdmin ? (
            <>
              <button
                type="button"
                className="sd-update-environment"
                onClick={() => void dashboardData.runUpdateDryRun(latestVersion)}
                disabled={!latestVersion || !status?.updateAvailable || dryRunBusy || applyActive}
              >
                {dryRunBusy ? '环境检查中…' : '检查升级环境'}
              </button>
              <button
                type="button"
                className="sd-update-apply-button"
                disabled={!canApply || dashboardData.updateApplyStarting}
                onClick={() => setConfirmApply(true)}
              >
                {applyActive || dashboardData.updateApplyStarting ? '升级处理中…' : dryRun?.capability.conversionRequired ? '转换为标准部署并升级' : '立即升级并重启面板'}
              </button>
              <button
                type="button"
                className="sd-update-refresh"
                onClick={() => void dashboardData.refreshUpdateStatus(true)}
                disabled={dashboardData.updateChecking}
              >
                {dashboardData.updateChecking ? '检查中…' : '手动检查版本'}
              </button>
            </>
          ) : (
            <span className="sd-update-user-note">仅管理员可手动刷新</span>
          )}
          <button type="button" className="sd-update-dismiss" onClick={closeDialog}>关闭</button>
        </div>

      </section>
      {confirmApply ? (
        <div className="sd-update-confirm" role="alertdialog" aria-modal="true" aria-labelledby="sd-update-confirm-title" onMouseDown={(event) => event.stopPropagation()}>
          <div className="sd-update-confirm-card">
            <p className="sd-update-dialog-kicker">SECOND CONFIRMATION</p>
            <h3 id="sd-update-confirm-title">确认升级并重启面板？</h3>
			<p>{onlineHumans !== undefined ? `当前有 ${onlineHumans} 名真人玩家在线。` : '当前在线真人玩家数量暂时无法确认。'} 本次升级会先保存并创建整档保护备份；若 Control 或运行栈不匹配，游戏服务器会停止并重启，在线玩家将断开连接。新 Panel 健康失败时自动恢复旧容器；Control 同步或实载验证失败时禁止实例继续带旧 DLL 启动。</p>
            <div>
              <button type="button" className="sd-update-dismiss" onClick={() => setConfirmApply(false)}>取消</button>
              <button
                type="button"
                className="sd-update-apply-button"
                onClick={() => {
                  setConfirmApply(false)
                  dashboardData.closeUpdateDialog()
                  void dashboardData.applyUpdate()
                }}
              >
                确认升级
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
