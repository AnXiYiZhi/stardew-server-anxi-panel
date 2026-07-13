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
  const updatePhases = [
    'backing_up', 'pulling', 'recreating', 'waiting_health',
    ...apply && ['rolling_back', 'failed_rolled_back', 'rollback_failed'].includes(apply.phase) ? ['rolling_back'] : [],
  ]
  const currentPhaseIndex = apply ? updatePhases.indexOf(apply.phase) : -1
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
          <strong>游戏服务器不会停止。</strong>升级只会重启 Web 面板，页面会短暂离线；若新版本未通过健康检查，系统会自动恢复原版本。
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

        {isAdmin && dashboardData.updateDryRunError ? (
          <div className="sd-update-error">{dashboardData.updateDryRunError}</div>
        ) : null}

        {isAdmin && apply ? (
          <section className={`sd-update-apply sd-update-apply--${apply.phase}`} aria-live="polite">
            <div><strong>{panelUpdatePhaseLabel(apply.phase)}</strong><span>{apply.progress}%</span></div>
            <p>{withVersionPrefix(apply.fromVersion)} → {withVersionPrefix(apply.toVersion)}</p>
            <progress max="100" value={apply.progress} />
            <ol className="sd-update-timeline">
              {updatePhases.map((phase, index) => (
                <li
                  key={phase}
                  className={index < currentPhaseIndex ? 'is-done' : index === currentPhaseIndex ? 'is-active' : apply.phase === 'succeeded' ? 'is-done' : ''}
                >
                  {panelUpdatePhaseLabel(phase)}
                </li>
              ))}
            </ol>
            {apply.result ? <p>{apply.result}</p> : null}
            {apply.error ? <div className="sd-update-error">{apply.error}</div> : null}
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
                {applyActive || dashboardData.updateApplyStarting ? '升级处理中…' : '立即升级并重启面板'}
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
            <p>将从 {withVersionPrefix(currentVersion)} 升级到 {withVersionPrefix(latestVersion)}。游戏服务器不会停止，但此页面会短暂离线，失败时自动恢复。</p>
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
