import type { Dispatch, SetStateAction } from 'react'
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
  error: string | null
  message: string | null
  isRunning: boolean
  currentMaxPlayers: number | null
  onlineCount: number | null
  onClearFeedback: () => void
  onClose: () => void
  onSave: () => void
}

export function ServerRuntimeSettingsDialog({
  mobile = false,
  draft,
  setDraft,
  loading,
  saving,
  error,
  message,
  isRunning,
  currentMaxPlayers,
  onlineCount,
  onClearFeedback,
  onClose,
  onSave,
}: ServerRuntimeSettingsDialogProps) {
  const titleId = mobile ? 'mobile-runtime-settings-title' : 'server-runtime-settings-title'
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

  return (
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
                  <option value="CabinStack">CabinStack（隐藏小屋堆叠，最适合大多数服务器）</option>
                  <option value="FarmhouseStack" hidden>FarmhouseStack（兼容已有配置）</option>
                  <option value="None">None（原版行为，小屋放置在真实农场位置）</option>
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

            <div className="sd-runtime-restart-note">
              仅保存配置，不会直接重启服务器。JunimoServer 会在容器下次启动时读取这些设置。
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
                {saving ? '保存中…' : '仅保存'}
              </button>
            </div>
          </>
        )}
      </div>
    </ModalPortal>
  )
}
