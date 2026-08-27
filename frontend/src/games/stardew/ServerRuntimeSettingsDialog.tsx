import { useState, type Dispatch, type SetStateAction } from 'react'
import { ModalPortal } from '../../core/ModalPortal'
import type { ServerRuntimeSettings } from '../../types'
import {
  maxPlayersValidationError,
  runtimeSettingsEffectText,
  runtimeSettingsOnlineWarning,
} from './server-runtime-settings-state'
import './ServerRuntimeSettingsDialog.css'

type ServerRuntimeSettingsDialogProps = {
  mobile?: boolean
  draft: ServerRuntimeSettings
  setDraft: Dispatch<SetStateAction<ServerRuntimeSettings>>
  loading: boolean
  saving: boolean
  savingAction: 'save' | 'save_restart' | null
  error: string | null
  message: string | null
  isRunning: boolean
  currentMaxPlayers: number | null
  onlineCount: number | null
  onClearFeedback: () => void
  onClose: () => void
  onSave: () => void
  onSaveAndRestart: () => void
}

export function ServerRuntimeSettingsDialog({
  mobile = false,
  draft,
  setDraft,
  loading,
  saving,
  savingAction,
  error,
  message,
  isRunning,
  currentMaxPlayers,
  onlineCount,
  onClearFeedback,
  onClose,
  onSave,
  onSaveAndRestart,
}: ServerRuntimeSettingsDialogProps) {
  const [restartConfirmOpen, setRestartConfirmOpen] = useState(false)
  const titleId = mobile ? 'mobile-runtime-settings-title' : 'server-runtime-settings-title'
  const restartTitleId = mobile ? 'mobile-runtime-settings-restart-title' : 'server-runtime-settings-restart-title'
  const validationError = maxPlayersValidationError(draft.maxPlayers)
  const onlineWarning = validationError ? null : runtimeSettingsOnlineWarning(draft.maxPlayers, onlineCount)
  const effectText = validationError ? null : runtimeSettingsEffectText({
    isRunning,
    currentMaxPlayers,
    configuredMaxPlayers: draft.maxPlayers,
  })
  const updateDraft = (update: Partial<ServerRuntimeSettings>) => {
    setDraft((current) => ({ ...current, ...update }))
    onClearFeedback()
  }
  const stepMaxPlayers = (delta: -1 | 1) => {
    const next = Math.min(100, Math.max(1, draft.maxPlayers + delta))
    updateDraft({ maxPlayers: next })
  }

  return (
    <>
      <ModalPortal
        className={mobile ? 'sd-mctrl-dialog-overlay' : 'sd-confirm-overlay'}
        ariaLabelledBy={titleId}
        onEscape={saving ? undefined : onClose}
      >
        <div className={mobile ? 'sd-panel sd-mctrl-dialog sd-runtime-dialog' : 'sd-confirm-dialog sd-confirm-dialog-wide sd-runtime-dialog'}>
        <h3 id={titleId}>联机人数与小屋设置</h3>

        {loading ? (
          <p>正在读取当前配置...</p>
        ) : (
          <>
            <section className="sd-runtime-primary" aria-labelledby={`${titleId}-player-limit`}>
              <label className="sd-runtime-field" htmlFor={`${titleId}-max-players`}>
                <span id={`${titleId}-player-limit`}>最大同时在线人数</span>
                <span className="sd-runtime-number-stepper">
                  <input
                    id={`${titleId}-max-players`}
                    className="sd-input sd-runtime-number-input"
                    type="number"
                    inputMode="numeric"
                    autoComplete="off"
                    min={1}
                    max={100}
                    step={1}
                    value={String(draft.maxPlayers)}
                    disabled={saving}
                    aria-invalid={validationError ? 'true' : undefined}
                    aria-describedby={`${titleId}-max-help${validationError ? ` ${titleId}-max-error` : ''}`}
                    onChange={(event) => updateDraft({ maxPlayers: Number(event.target.value) })}
                  />
                  <button
                    type="button"
                    className="sd-runtime-stepper-btn"
                    onClick={() => stepMaxPlayers(-1)}
                    disabled={saving || draft.maxPlayers <= 1}
                    aria-label="减少最大同时在线人数"
                    title="减少 1 人"
                  >
                    −
                  </button>
                  <button
                    type="button"
                    className="sd-runtime-stepper-btn"
                    onClick={() => stepMaxPlayers(1)}
                    disabled={saving || draft.maxPlayers >= 100}
                    aria-label="增加最大同时在线人数"
                    title="增加 1 人"
                  >
                    +
                  </button>
                </span>
              </label>
              <p id={`${titleId}-max-help`} className="sd-runtime-help">
                范围 1~100，包含主机位。降低上限不会删除已有角色或小屋。
              </p>
              {validationError ? (
                <div id={`${titleId}-max-error`} className="sd-runtime-inline-error" role="alert">{validationError}</div>
              ) : null}
              {effectText ? <div className="sd-runtime-effect">{effectText}</div> : null}
              {onlineWarning ? <div className="sd-runtime-online-warning">{onlineWarning}</div> : null}
            </section>

            <fieldset className="sd-runtime-advanced">
              <legend>高级设置</legend>

              <label className="sd-runtime-field">
                <span>小屋策略（CabinStrategy）</span>
                <select
                  className="sd-input"
                  value={draft.cabinStrategy}
                  disabled={saving}
                  onChange={(event) => updateDraft({ cabinStrategy: event.target.value })}
                >
                  <option value="None">None（原版行为，小屋放置在真实农场位置）</option>
                  <option value="CabinStack">CabinStack（堆叠模式，隐藏小屋）</option>
                  <option value="FarmhouseStack" hidden>FarmhouseStack（兼容已有配置）</option>
                </select>
              </label>

              <label className="sd-runtime-field">
                <span>已有小屋处理方式（ExistingCabinBehavior）</span>
                <select
                  className="sd-input"
                  value={draft.existingCabinBehavior}
                  disabled={saving}
                  onChange={(event) => updateDraft({ existingCabinBehavior: event.target.value })}
                >
                  <option value="KeepExisting">KeepExisting（保留已有小屋位置）</option>
                  <option value="MoveToStack">MoveToStack（把已有小屋迁移到策略指定位置）</option>
                </select>
              </label>

              <label className="sd-runtime-field">
                <span>网络广播频率（NetworkBroadcastPeriod，单位：刻）</span>
                <select
                  className="sd-input"
                  value={draft.networkBroadcastPeriod}
                  disabled={saving}
                  onChange={(event) => updateDraft({ networkBroadcastPeriod: Number(event.target.value) })}
                >
                  <option value={1}>1（每刻广播，最实时）</option>
                  <option value={2}>2</option>
                  <option value={3}>3（原版频率）</option>
                </select>
              </label>
            </fieldset>

            <div id={`${titleId}-restart-note`} className="sd-runtime-restart-note">
              {isRunning
                ? '“仅保存”会等待下次重启生效；“保存并重启”会先确认在线玩家断线风险，再通过现有生命周期重启。'
                : '服务器当前已停止；请仅保存，JunimoServer 会在下次启动时读取这些设置。'}
            </div>

            {error ? <div className="sd-runtime-error" role="alert">{error}</div> : null}
            {message ? <div className="sd-runtime-success" role="status" aria-live="polite">{message}</div> : null}

            <div className="sd-runtime-actions">
              <button type="button" className="sd-btn-tan sd-runtime-dialog-btn" onClick={onClose} disabled={saving}>
                关闭
              </button>
              <button
                type="button"
                className="sd-btn-green sd-runtime-dialog-btn"
                onClick={onSave}
                disabled={saving || Boolean(validationError)}
              >
                {savingAction === 'save' ? '保存中…' : '仅保存'}
              </button>
              <button
                type="button"
                className="sd-btn-restart sd-runtime-dialog-btn sd-runtime-save-restart-btn"
                onClick={() => setRestartConfirmOpen(true)}
                disabled={saving || Boolean(validationError) || !isRunning}
                aria-describedby={`${titleId}-restart-note`}
                title={isRunning ? '保存设置并重启服务器' : '服务器当前已停止，请仅保存'}
              >
                {savingAction === 'save_restart' ? '处理中…' : '保存并重启'}
              </button>
            </div>
          </>
        )}
        </div>
      </ModalPortal>

      {restartConfirmOpen ? (
        <ModalPortal
          className={mobile ? 'sd-mctrl-dialog-overlay' : 'sd-confirm-overlay'}
          role="alertdialog"
          ariaLabelledBy={restartTitleId}
          onEscape={() => setRestartConfirmOpen(false)}
        >
          <div className={mobile ? 'sd-panel sd-mctrl-dialog sd-runtime-restart-dialog' : 'sd-confirm-dialog sd-runtime-restart-dialog'}>
            <h3 id={restartTitleId}>确认保存并重启</h3>
            <p>设置保存成功后会立即提交服务器重启，新的联机人数与小屋设置将在本次重启后生效。</p>
            <p className="sd-runtime-online-warning">
              {onlineCount != null && onlineCount > 0
                ? `当前有 ${onlineCount} 名玩家在线，重启会使他们暂时断开连接。请先确认游戏进度已经保存。`
                : '重启会暂时中断服务器连接，请先确认游戏进度已经保存。'}
            </p>
            <div className={mobile ? 'sd-mctrl-dialog-actions' : 'sd-confirm-actions'}>
              <button type="button" className="sd-btn-tan sd-runtime-dialog-btn" onClick={() => setRestartConfirmOpen(false)}>
                取消
              </button>
              <button
                type="button"
                className="sd-btn-green sd-runtime-dialog-btn"
                onClick={() => {
                  setRestartConfirmOpen(false)
                  onSaveAndRestart()
                }}
              >
                确认保存并重启
              </button>
            </div>
          </div>
        </ModalPortal>
      ) : null}
    </>
  )
}
