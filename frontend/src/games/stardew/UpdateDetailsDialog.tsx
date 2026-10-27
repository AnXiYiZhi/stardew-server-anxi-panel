import { useEffect, useRef, useState } from 'react'
import type { CurrentUser } from '../../types'
import { usePanelUpdate } from './PanelUpdateProvider'
import { FullStackUpgradeActions, FullStackUpgradePanel } from './FullStackUpgradePanel'
import { canStartPanelUpdate, hasGameUpgradeFlow, isPanelUpdateActive, panelUpdatePhaseLabel, panelUpdateSurface } from './panel-update-machine'
import { withVersionPrefix } from './update-status'
import './UpdateDetailsDialog.css'
import './UpdateCenter.css'

function date(value?: string | null) {
  return value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '尚未检查'
}

export function UpdateDetailsDialog({ user }: { user: CurrentUser }) {
  const data = usePanelUpdate()
  const theme = data.updateTheme ?? 'day'
  const card = useRef<HTMLElement>(null)
  const [confirm, setConfirm] = useState(false)
  const [acknowledged, setAcknowledged] = useState('')
  const status = data.updateStatus
  const apply = data.updateApply
  const surface = panelUpdateSurface(status, apply, data.versionInfo)
  const admin = user.role === 'admin'
  const active = isPanelUpdateActive(apply)
  const gameUpgrade = hasGameUpgradeFlow(data.versionInfo, data.fullStackUpgrade, status)
  const content = status?.content
  const noticesAccepted = !status?.noticeDigest || acknowledged === status.noticeDigest
  const ready = status?.checkStatus === 'ok' && !content?.blocked
  const dryRun = data.updateDryRun
  const scopedUpgrade = gameUpgrade && !dryRun?.capability.conversionRequired
  const environmentReady = dryRun?.phase === 'succeeded' && dryRun.targetVersion.replace(/^v/, '') === surface.targetVersion.replace(/^v/, '')
  const busy = data.updateDryRunChecking || dryRun?.phase === 'starting' || dryRun?.phase === 'running' || active || data.updateApplyStarting
  const canApply = ready && noticesAccepted && canStartPanelUpdate(user, status, dryRun, apply)
  const phase = apply?.fullStack?.phase || apply?.phase || ''
  const progress = apply?.fullStack?.progress ?? apply?.progress ?? 0
  const successfulHistory = apply && ['succeeded', 'not_needed'].includes(phase)
  const completedGameUpgrade = data.fullStackUpgrade?.phase === 'succeeded'
  const authorizedGameUpgrade = Boolean(data.fullStackUpgrade?.authorizedAt && !['succeeded', 'failed_safe', 'panel_failed_rolled_back'].includes(data.fullStackUpgrade.phase))
  const newAuthorization = !authorizedGameUpgrade && Boolean(status?.updateAvailable || scopedUpgrade && !completedGameUpgrade)
  const visibleNotices = content?.notices?.filter(notice => newAuthorization || content?.blocked || notice.level === 'danger' || notice.level === 'warning' && !notice.requiresConfirmation) || []
  const showScope = admin && scopedUpgrade && !completedGameUpgrade && (authorizedGameUpgrade || !content?.blocked && (!status?.updateAvailable || environmentReady))
  const showPanelAction = admin && status?.updateAvailable && !active && !content?.blocked && (!scopedUpgrade || !environmentReady)
  const releaseTitle = content?.title?.replace(/^v?\d+\.\d+\.\d+\s*[·：:-]\s*/, '')
  const close = data.closeUpdateDialog

  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    card.current?.focus()
    const keyboard = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { event.preventDefault(); close() }
      if (event.key !== 'Tab') return
      const controls = Array.from(card.current?.querySelectorAll<HTMLElement>('button:not(:disabled), a[href], input:not(:disabled), summary, [tabindex="0"]') || [])
      const first = controls[0]
      const last = controls[controls.length - 1]
      if (event.shiftKey && (document.activeElement === first || document.activeElement === card.current)) { event.preventDefault(); last?.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
    }
    document.addEventListener('keydown', keyboard)
    return () => { document.body.style.overflow = previousOverflow; document.removeEventListener('keydown', keyboard); previous?.focus() }
  }, [close])

  const progressCard = apply ? <section className={'sd-update-apply sd-update-apply--' + apply.phase} aria-live="polite">
    <div><strong>{panelUpdatePhaseLabel(phase)}</strong><span>{progress}%</span></div>
    <p>{withVersionPrefix(apply.fromVersion)} → {withVersionPrefix(apply.toVersion)}</p>
    <progress max="100" value={progress} />
    {apply.fullStack?.result || apply.result ? <p>{apply.fullStack?.result || apply.result}</p> : null}
    {apply.fullStack?.error || apply.error ? <p className="sd-update-error">{apply.fullStack?.error || apply.error}</p> : null}
  </section> : null

  return <div className={`sd-update-overlay is-${theme}`} onMouseDown={event => { if (event.target === event.currentTarget) close() }}>
    <section ref={card} tabIndex={-1} className={`sd-update-dialog sd-update-center is-${theme}`} data-theme={theme} role="dialog" aria-modal="true" aria-labelledby="sd-update-dialog-title">
      <header className="sd-update-dialog-head">
        <div className="sd-update-title"><svg viewBox="0 0 20 20" aria-hidden="true"><path d="M9 2h2v2h2v2h2v2h-4v6H9V8H5V6h2V4h2ZM3 13h2v3h10v-3h2v5H3Z" /></svg><h2 id="sd-update-dialog-title">更新中心</h2></div>
        <button type="button" className="sd-update-close" onClick={close} aria-label="关闭更新中心"><svg viewBox="0 0 20 20" aria-hidden="true"><path d="M4 2h2v2h2v2h2V4h2V2h2v2h-2v2h-2v2h2v2h2v2h-2v-2h-2v-2H8v2H6v2H4v-2h2v-2h2V6H6V4H4Z" /></svg></button>
      </header>
      <div className="sd-update-center-body">
      <div className="sd-update-version-path">
        <div><span>当前版本</span><strong>{withVersionPrefix(surface.currentVersion)}</strong></div>
        {status?.updateAvailable ? <><span aria-hidden="true"><svg viewBox="0 0 20 20"><path d="M10 3h2v2h2v2h2v2h2v2h-2v2h-2v2h-2v2h-2v-6H2V9h8Z" /></svg></span><div><span>{content?.blocked ? '暂缓升级' : '可升级版本'}</span><strong>{withVersionPrefix(surface.targetVersion)}</strong></div></> : <span className="sd-update-current-state">{content?.blocked ? '暂缓升级' : status?.checkStatus === 'ok' ? '已是最新版本' : '等待检查'}</span>}
      </div>
      <section className="sd-update-release-notes" aria-label="本次更新">
        <h3>{releaseTitle || (status?.updateAvailable ? '本次更新' : status?.checkStatus === 'ok' ? '当前版本更新内容' : '检查可用更新')}</h3>
        {content?.summary ? <p>{content.summary}</p> : null}
        {content?.changes?.length ? <ul>{content.changes.map((change, index) => <li key={index}>{change}</li>)}</ul> : content?.notes ? <p className="sd-update-notes-text">{content.notes}</p> : <p>此版本暂未提供详细更新说明。</p>}
        {status?.updateAvailable && surface.targetVersion.replace(/^v/, '') === '0.7.3' ? <p className="sd-update-route">完成此升级准备版本后，可继续更新游戏大版本。</p> : null}
        {status?.releaseUrl ? <a href={status.releaseUrl} target="_blank" rel="noreferrer">查看完整更新说明 ↗</a> : null}
      </section>
      {visibleNotices.length ? <section className="sd-update-notices" aria-label="重要提示">
        <h3>升级提醒</h3>
        {visibleNotices.map((notice, index) => <div key={index} className={'sd-update-notice sd-update-notice--' + notice.level}><strong>{notice.title}</strong><p>{notice.message}</p></div>)}
        {admin && newAuthorization && status?.noticeDigest ? <label className="sd-update-ack"><input type="checkbox" checked={noticesAccepted} onChange={event => setAcknowledged(event.target.checked ? status.noticeDigest || '' : '')} />我已阅读并理解以上重要提示</label> : null}
      </section> : null}
      {content?.blocked ? <p className="sd-update-error" role="alert">此版本暂缓升级，请按照重要提示处理并等待后续通知。</p> : null}
      {status?.checkError || data.updateError ? <p className="sd-update-error" role="alert">{status?.checkError || data.updateError}</p> : null}
      {active || apply && !successfulHistory ? progressCard : null}
      {showScope ? <FullStackUpgradePanel compact hideActions /> : null}
      {environmentReady && status?.updateAvailable && !active ? <p className="sd-update-environment-result">升级环境检查通过</p> : null}
      {data.updateDryRunError || data.updateApplyError ? <p className="sd-update-error" role="alert">{data.updateDryRunError || data.updateApplyError}</p> : null}
      {successfulHistory ? <details className="sd-update-history"><summary>上次升级已完成 · {withVersionPrefix(apply.toVersion)}</summary>{progressCard}{completedGameUpgrade ? <><FullStackUpgradePanel readOnly compact />{admin && !status?.updateAvailable && data.versionInfo?.runtimeUpgrade?.requiresScopeConfirmation ? <button type="button" className="sd-update-environment" disabled={data.fullStackBusy} onClick={() => void data.prepareFullStack(data.versionInfo?.version || '')}>重新检查游戏组件更新</button> : null}</> : null}</details> : null}
      <details className="sd-update-technical"><summary>版本与诊断详情</summary>
        <dl className="sd-update-details"><div><dt>发布时间</dt><dd>{date(status?.publishedAt)}</dd></div><div><dt>最近检查</dt><dd>{date(status?.checkedAt)}</dd></div><div><dt>构建提交</dt><dd>{status?.currentCommit || data.versionInfo?.commit || '—'}</dd></div><div><dt>升级环境</dt><dd>{dryRun?.capability.reason || '尚未检查'}</dd></div></dl>
        {dryRun?.error ? <p>{dryRun.error}</p> : null}
        {dryRun?.logs.map((log, index) => <p key={index}>{log.message}</p>)}
        {apply?.logs.map((log, index) => <p key={index}>{log.message}</p>)}
      </details>
      </div>
      <footer className="sd-update-center-footer">
        {admin ? <button type="button" className="sd-update-refresh" disabled={data.updateChecking || active} onClick={() => void data.refreshUpdateStatus(true)}>{data.updateChecking ? '检查中…' : '重新检查'}</button> : <span>请联系管理员执行升级</span>}
        {showScope ? <FullStackUpgradeActions compact noticeDigest={noticesAccepted ? status?.noticeDigest : undefined} noticesAccepted={noticesAccepted} /> : showPanelAction ? confirm ? <div role="alertdialog" aria-labelledby="sd-update-confirm-title" className="sd-update-footer-confirm">
          <p id="sd-update-confirm-title">确认升级并重启面板？</p>
          <div className="sd-update-actions"><button type="button" className="sd-update-dismiss" onClick={() => setConfirm(false)}>取消</button><button type="button" className="sd-update-apply-button" disabled={!canApply || busy} onClick={() => { setConfirm(false); void data.applyUpdate(status?.noticeDigest) }}>确认升级</button></div>
        </div> : <button type="button" className="sd-update-apply-button" disabled={!ready || !noticesAccepted || busy || environmentReady && !canApply} onClick={() => { if (!environmentReady) void data.runUpdateDryRun(surface.targetVersion); else setConfirm(true) }}>
          {busy ? '正在检查升级环境…' : environmentReady ? '立即升级' : '检查并准备升级'}
        </button> : active ? <span>升级进行中…</span> : null}
      </footer>
    </section>
  </div>
}
