import { useEffect, useState } from 'react'
import {
  cancelFarmhandDeleteIntent,
  deleteFarmhand,
  retryFarmhandDeletePersistence,
} from '../../api'
import { errorMessage } from '../../core/helpers'
import { ModalPortal } from '../../core/ModalPortal'
import type { FarmhandDeleteIntent, FarmhandDeleteMode } from '../../types'
import {
  farmhandDeleteConfirmationReady,
  farmhandDeleteExpiryText,
  farmhandDeleteIntentCancelable,
  farmhandDeleteIntentNeedsRecovery,
  farmhandDeleteStatusLabel,
} from './farmhand-delete-state'
import './FarmhandDeleteFlow.css'

export type FarmhandDeleteTarget = {
  uniqueMultiplayerId: string
  name: string
}

type FarmhandDeleteFlowProps = {
  instanceId: string
  activeSaveId?: string
  intent: FarmhandDeleteIntent | null
  intentError: string | null
  intentLoading: boolean
  mobile?: boolean
  onlineHumanCount: number
  onClose: () => void
  onIntentRefresh: (accepted?: FarmhandDeleteIntent) => Promise<void>
  onJobsRefresh: () => Promise<void>
  onOpenBackups: () => void
  target: FarmhandDeleteTarget | null
}

export function FarmhandDeleteFlow({
  instanceId,
  activeSaveId,
  intent,
  intentError,
  intentLoading,
  mobile = false,
  onlineHumanCount,
  onClose,
  onIntentRefresh,
  onJobsRefresh,
  onOpenBackups,
  target,
}: FarmhandDeleteFlowProps) {
  const [mode, setMode] = useState<FarmhandDeleteMode>('wait')
  const [riskAcknowledged, setRiskAcknowledged] = useState(false)
  const [confirmationName, setConfirmationName] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [actionBusy, setActionBusy] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  useEffect(() => {
    setMode('wait')
    setRiskAcknowledged(false)
    setConfirmationName('')
    setActionError(null)
  }, [target?.uniqueMultiplayerId])

  async function submitDeletion() {
    if (!target || !activeSaveId) return
    if (!farmhandDeleteConfirmationReady(mode, riskAcknowledged, confirmationName, target.name)) {
      setActionError('立即维护删除需要确认当天未同步进度可能丢失，并准确输入目标人物名称。')
      return
    }
    setSubmitting(true)
    setActionError(null)
    setMessage(null)
    try {
      const submission = await deleteFarmhand(
        target.uniqueMultiplayerId,
        target.name,
        activeSaveId,
        mode,
        mode === 'maintenance_now' && riskAcknowledged,
        mode === 'maintenance_now' ? confirmationName : '',
        instanceId,
      )
      setMessage(
        mode === 'wait'
          ? `已登记 ${target.name} 的等待删除；无人在线时会自动进入维护。`
          : `已提交 ${target.name} 的立即维护删除；60 秒倒计时已经开始。`,
      )
      onClose()
      await Promise.all([onIntentRefresh(submission.intent), onJobsRefresh()])
    } catch (reason) {
      setActionError(errorMessage(reason))
      await onIntentRefresh().catch(() => undefined)
    } finally {
      setSubmitting(false)
    }
  }

  async function cancelWaiting() {
    if (!intent) return
    setActionBusy(true)
    setActionError(null)
    try {
      await cancelFarmhandDeleteIntent(intent.operationId, instanceId)
      setMessage(`已取消 ${intent.expectedName} 的${intent.status === 'countdown' ? '维护倒计时' : '等待删除'}。`)
      await onIntentRefresh()
    } catch (reason) {
      setActionError(errorMessage(reason))
    } finally {
      setActionBusy(false)
    }
  }

  async function retryFinalSave() {
    if (!intent) return
    setActionBusy(true)
    setActionError(null)
    setMessage(null)
    try {
      await retryFarmhandDeletePersistence(intent.operationId, instanceId)
      setMessage('已提交最终保存重试；联机入口会在保存和磁盘验证成功后恢复。')
      await Promise.all([onIntentRefresh(), onJobsRefresh()])
    } catch (reason) {
      setActionError(errorMessage(reason))
    } finally {
      setActionBusy(false)
    }
  }

  const dialogTitleId = mobile ? 'mobile-farmhand-delete-title' : 'farmhand-delete-title'
  const overlayClass = mobile ? 'sd-mplay-confirm-overlay' : 'sd-confirm-overlay'
  const dialogClass = mobile
    ? 'sd-panel sd-mplay-confirm-dialog sd-farmhand-delete-dialog is-mobile'
    : 'sd-confirm-dialog sd-farmhand-delete-dialog'
  const buttonClass = mobile ? ' sd-mplay-confirm-btn' : ''
  const immediateReady = target
    ? farmhandDeleteConfirmationReady(mode, riskAcknowledged, confirmationName, target.name)
    : false

  return (
    <>
      {intent || intentError || intentLoading || message || actionError ? (
        <section className={`sd-farmhand-delete-status${intent ? ` is-${intent.status}` : ''}`} aria-label="人物删除状态">
          <div className="sd-farmhand-delete-status-head">
            <div>
              <span>人物删除</span>
              <strong>{intent ? farmhandDeleteStatusLabel(intent.status) : intentLoading ? '正在读取状态' : '状态读取失败'}</strong>
            </div>
            <button type="button" className={`sd-btn-tan${buttonClass}`} onClick={() => void onIntentRefresh().catch(() => undefined)} disabled={intentLoading || actionBusy}>
              {intentLoading ? '刷新中…' : '刷新'}
            </button>
          </div>
          {intent ? (
            <div className="sd-farmhand-delete-status-body">
              <span>{intent.expectedName || '未命名人物'} · {intent.mode === 'wait' ? '等待无人在线' : '立即维护'}</span>
              {intent.status === 'waiting' ? <span>{farmhandDeleteExpiryText(intent)}</span> : null}
              {intent.backupName ? <span>保护备份：{intent.backupName}</span> : null}
              {intent.lastError ? <span className="is-error">{intent.lastError}</span> : null}
            </div>
          ) : null}
          {message ? <div className="sd-farmhand-delete-feedback is-ok" role="status">{message}</div> : null}
          {intentError || actionError ? <div className="sd-farmhand-delete-feedback is-error" role="alert">{actionError ?? intentError}</div> : null}
          {farmhandDeleteIntentCancelable(intent) || farmhandDeleteIntentNeedsRecovery(intent) ? (
            <div className="sd-farmhand-delete-status-actions">
              {farmhandDeleteIntentCancelable(intent) ? (
                <button type="button" className={`sd-btn-tan${buttonClass}`} onClick={() => void cancelWaiting()} disabled={actionBusy}>
                  {actionBusy ? '处理中…' : intent?.status === 'countdown' ? '取消倒计时' : '取消等待'}
                </button>
              ) : null}
              {farmhandDeleteIntentNeedsRecovery(intent) ? (
                <>
                  <button type="button" className={`sd-btn-green${buttonClass}`} onClick={() => void retryFinalSave()} disabled={actionBusy}>
                    {actionBusy ? '提交中…' : '重试最终保存'}
                  </button>
                  <button type="button" className={`sd-btn-tan${buttonClass}`} onClick={onOpenBackups} disabled={actionBusy}>
                    前往保护备份
                  </button>
                </>
              ) : null}
            </div>
          ) : null}
        </section>
      ) : null}

      {target ? (
        <ModalPortal
          className={overlayClass}
          role="alertdialog"
          ariaLabelledBy={dialogTitleId}
          onEscape={submitting ? undefined : onClose}
        >
          <div className={dialogClass}>
            <h3 id={dialogTitleId}>删除存档人物</h3>
            <p>将永久删除人物 {target.name}、人物进度、背包、对应小屋和小屋内容。系统会先保存并创建整档保护备份。</p>

            <div className="sd-farmhand-delete-modes" role="group" aria-label="删除执行方式">
              <button
                type="button"
                className={mode === 'wait' ? 'is-selected' : ''}
                aria-pressed={mode === 'wait'}
                onClick={() => setMode('wait')}
                disabled={submitting}
              >
                <strong>等待无人在线</strong>
                <span>推荐 · 最多等待 24 小时</span>
              </button>
              <button
                type="button"
                className={mode === 'maintenance_now' ? 'is-selected is-danger' : ''}
                aria-pressed={mode === 'maintenance_now'}
                onClick={() => setMode('maintenance_now')}
                disabled={submitting}
              >
                <strong>立即进入维护</strong>
                <span>不推荐 · 会断开在线玩家</span>
              </button>
            </div>

            {mode === 'wait' ? (
              <div className="sd-farmhand-delete-mode-note">
                玩家可以继续游玩。系统检测到无人在线后，会关闭新连接，再保存、备份并删除；目标人物重新上线、切换存档或超过 24 小时都会自动取消。
              </div>
            ) : (
              <div className="sd-farmhand-delete-risk">
                <p>当前有 {onlineHumanCount} 名真人玩家在线。游戏内会每 10 秒通告一次，60 秒后关闭新连接并断开所有玩家。尚未同步的当天进度可能丢失。</p>
                <p>倒计时或删除前只要有人开始睡觉，或游戏进入日结算，本次操作就会取消。</p>
                <label className="sd-farmhand-delete-risk-check">
                  <input
                    type="checkbox"
                    checked={riskAcknowledged}
                    onChange={(event) => setRiskAcknowledged(event.target.checked)}
                    disabled={submitting}
                  />
                  <span>我确认在线玩家可能丢失当天尚未同步的进度</span>
                </label>
                <label className="sd-farmhand-delete-confirm-name">
                  <span>输入人物名称 <strong>{target.name}</strong> 以继续</span>
                  <input
                    className="sd-input"
                    type="text"
                    value={confirmationName}
                    onChange={(event) => setConfirmationName(event.target.value)}
                    autoComplete="off"
                    disabled={submitting}
                  />
                </label>
              </div>
            )}

            {actionError ? <div className="sd-farmhand-delete-feedback is-error" role="alert">{actionError}</div> : null}
            <div className={mobile ? 'sd-mplay-confirm-actions' : 'sd-confirm-actions'}>
              <button type="button" className={`sd-btn-tan${buttonClass}`} onClick={onClose} disabled={submitting}>取消</button>
              <button
                type="button"
                className={`sd-btn-delete${buttonClass}`}
                onClick={() => void submitDeletion()}
                disabled={submitting || !activeSaveId || (mode === 'maintenance_now' && !immediateReady)}
              >
                {submitting ? '正在提交…' : mode === 'wait' ? '登记等待删除' : '开始 60 秒倒计时'}
              </button>
            </div>
          </div>
        </ModalPortal>
      ) : null}
    </>
  )
}
