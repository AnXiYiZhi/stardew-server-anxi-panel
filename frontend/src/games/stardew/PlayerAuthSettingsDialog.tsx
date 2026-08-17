import { useEffect, useState } from 'react'
import {
  getInstancePasswordStatus,
  getInstancePlayerAuthConfig,
  updateInstancePlayerAuthConfig,
} from '../../api'
import { errorMessage } from '../../core/helpers'
import { ModalPortal } from '../../core/ModalPortal'
import type {
  InstancePasswordStatus,
  InstancePlayerAuthConfig,
  PlayerAuthMode,
} from '../../types'
import { playerAuthPasswordError } from './player-auth-password'
import './PlayerAuthSettingsDialog.css'

type PlayerAuthSettingsDialogProps = {
  isRunning: boolean
  mobile?: boolean
  onClose: () => void
  onRestart: () => Promise<void>
}

const modeOptions: Array<{
  mode: PlayerAuthMode
  title: string
  summary: string
  marker: string
}> = [
  { mode: 'none', title: '不设密码', summary: '知道地址即可加入，适合可信内网。', marker: '开放' },
  { mode: 'global', title: '全服统一密码', summary: '所有玩家输入同一个 !login 密码。', marker: '共用' },
  { mode: 'role', title: '角色独立密码', summary: '每个存档角色使用自己的密码，不能串号。', marker: '独立' },
]

const minAuthTimeoutSeconds = 0
const maxAuthTimeoutSeconds = 3600
const minLoginAttempts = 1
const maxLoginAttempts = 20

function modeLabel(mode?: PlayerAuthMode): string {
  if (mode === 'global') return '全服统一密码'
  if (mode === 'role') return '角色独立密码'
  if (mode === 'none') return '不设密码'
  return '尚未读取'
}

function policyLabel(timeoutSeconds: number, maxAttempts: number): string {
  const timeout = timeoutSeconds === 0 ? '不限制超时' : `${timeoutSeconds} 秒超时`
  return `${timeout} · 最多失败 ${maxAttempts} 次`
}

export function PlayerAuthSettingsDialog({ isRunning, mobile = false, onClose, onRestart }: PlayerAuthSettingsDialogProps) {
  const [config, setConfig] = useState<InstancePlayerAuthConfig | null>(null)
  const [mode, setMode] = useState<PlayerAuthMode>('none')
  const [globalPassword, setGlobalPassword] = useState('')
  const [rolePasswords, setRolePasswords] = useState<Record<string, string>>({})
  const [rolePasswordRemovals, setRolePasswordRemovals] = useState<Record<string, boolean>>({})
  const [timeoutSecondsInput, setTimeoutSecondsInput] = useState('120')
  const [maxAttemptsInput, setMaxAttemptsInput] = useState('3')
  const [passwordVisible, setPasswordVisible] = useState(false)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [status, setStatus] = useState<InstancePasswordStatus | null>(null)
  const [statusLoading, setStatusLoading] = useState(false)
  const [statusError, setStatusError] = useState<string | null>(null)
  const [restartPromptOpen, setRestartPromptOpen] = useState(false)
  const [restarting, setRestarting] = useState(false)
  const [restartError, setRestartError] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    setLoading(true)
    setError(null)
    getInstancePlayerAuthConfig()
      .then((result) => {
        if (!active) return
        setConfig(result)
        setMode(result.mode)
        setGlobalPassword(result.globalPassword ?? '')
        setTimeoutSecondsInput(String(result.timeoutSeconds))
        setMaxAttemptsInput(String(result.maxAttempts))
      })
      .catch((reason: unknown) => {
        if (active) setError(errorMessage(reason))
      })
      .finally(() => {
        if (active) setLoading(false)
      })

    if (isRunning) {
      setStatusLoading(true)
      setStatusError(null)
      getInstancePasswordStatus()
        .then((result) => {
          if (active) setStatus(result)
        })
        .catch((reason: unknown) => {
          if (!active) return
          setStatus(null)
          setStatusError(errorMessage(reason))
        })
        .finally(() => {
          if (active) setStatusLoading(false)
        })
    }

    return () => {
      active = false
    }
  }, [isRunning])

  function selectMode(nextMode: PlayerAuthMode) {
    setMode(nextMode)
    setError(null)
    setMessage(null)
  }

  function updateRolePassword(roleId: string, password: string) {
    setRolePasswords((current) => ({ ...current, [roleId]: password }))
    if (password.length > 0) {
      setRolePasswordRemovals((current) => ({ ...current, [roleId]: false }))
    }
    setError(null)
    setMessage(null)
  }

  function toggleRolePasswordRemoval(roleId: string) {
    setRolePasswordRemovals((current) => ({ ...current, [roleId]: !current[roleId] }))
    setRolePasswords((current) => ({ ...current, [roleId]: '' }))
    setError(null)
    setMessage(null)
  }

  async function refreshStatus() {
    if (!isRunning) return
    setStatusLoading(true)
    setStatusError(null)
    try {
      setStatus(await getInstancePasswordStatus())
    } catch (reason) {
      setStatus(null)
      setStatusError(errorMessage(reason))
    } finally {
      setStatusLoading(false)
    }
  }

  function postponeRestart() {
    setRestartPromptOpen(false)
    setRestartError(null)
    setMessage('设置已保存。服务器仍在使用旧配置，请稍后手动重启。')
  }

  async function restartNow() {
    setRestarting(true)
    setRestartError(null)
    try {
      await onRestart()
      setRestartPromptOpen(false)
      onClose()
    } catch (reason) {
      setRestartError(errorMessage(reason))
    } finally {
      setRestarting(false)
    }
  }

  async function save() {
    if (!config) return
    const updates = config.roles
      .map((role) => ({ roleId: role.roleId, password: rolePasswords[role.roleId] ?? '' }))
      .filter((update) => update.password.length > 0 && !rolePasswordRemovals[update.roleId])
    const removals = config.roles
      .filter((role) => rolePasswordRemovals[role.roleId])
      .map((role) => role.roleId)

    const globalPasswordError = mode === 'global' ? playerAuthPasswordError(globalPassword, '全服密码') : null
    if (globalPasswordError) {
      setError(globalPasswordError)
      setMessage(null)
      return
    }
    const invalidRolePassword = updates
      .map((update) => playerAuthPasswordError(update.password, '角色密码'))
      .find((validationError): validationError is string => validationError !== null)
    if (invalidRolePassword) {
      setError(invalidRolePassword)
      setMessage(null)
      return
    }
    const timeoutSeconds = Number(timeoutSecondsInput)
    if (timeoutSecondsInput.trim() === '' || !Number.isInteger(timeoutSeconds) || timeoutSeconds < minAuthTimeoutSeconds || timeoutSeconds > maxAuthTimeoutSeconds) {
      setError('认证超时时间必须为 0 到 3600 秒，0 表示不限制。')
      setMessage(null)
      return
    }
    const maxAttempts = Number(maxAttemptsInput)
    if (maxAttemptsInput.trim() === '' || !Number.isInteger(maxAttempts) || maxAttempts < minLoginAttempts || maxAttempts > maxLoginAttempts) {
      setError('最大失败次数必须为 1 到 20 次。')
      setMessage(null)
      return
    }
    const timeoutChanged = timeoutSeconds !== config.timeoutSeconds
    const maxAttemptsChanged = maxAttempts !== config.maxAttempts
    const modeChanged = mode !== config.mode
    const globalPasswordChanged = mode === 'global' && globalPassword !== (config.globalPassword ?? '')
    const restartSettingChanged = modeChanged || globalPasswordChanged || timeoutChanged || maxAttemptsChanged
    const roleCredentialsChanged = updates.length > 0 || removals.length > 0
    if (!restartSettingChanged && !roleCredentialsChanged) {
      setError(null)
      setMessage('没有需要保存的修改。')
      return
    }
    setSaving(true)
    setError(null)
    setMessage(null)
    setRestartPromptOpen(false)
    setRestartError(null)
    try {
      const next = await updateInstancePlayerAuthConfig({
        expectedRevision: config.revision,
        mode,
        ...(timeoutChanged ? { timeoutSeconds } : {}),
        ...(maxAttemptsChanged ? { maxAttempts } : {}),
        ...(mode === 'global' ? { globalPassword } : {}),
        ...(updates.length ? { rolePasswordUpdates: updates } : {}),
        ...(removals.length ? { rolePasswordRemovals: removals } : {}),
      })
      setConfig(next)
      setMode(next.mode)
      setGlobalPassword(next.globalPassword ?? '')
      setTimeoutSecondsInput(String(next.timeoutSeconds))
      setMaxAttemptsInput(String(next.maxAttempts))
      setRolePasswords({})
      setRolePasswordRemovals({})
      const revisionChanged = next.revision !== config.revision
      if (revisionChanged && next.restartRequired && isRunning) {
        setMessage('玩家加入保护已保存，需要重启服务器后生效。')
        setRestartPromptOpen(true)
      } else if (!isRunning && next.revision !== config.revision) {
        setMessage('玩家加入保护已保存，将在下次启动服务器时生效。')
      } else if (updates.length || removals.length) {
        setMessage('角色密码已保存，后续登录立即使用新规则，无需重启服务器。')
      } else {
        setMessage('玩家加入保护设置已保存。')
      }
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setSaving(false)
    }
  }

  const dialogTitleId = mobile ? 'mobile-player-auth-settings-title' : 'player-auth-settings-title'
  const overlayClass = mobile ? 'sd-mctrl-dialog-overlay' : 'sd-confirm-overlay'
  const dialogClass = mobile
    ? 'sd-panel sd-mctrl-dialog sd-player-auth-dialog sd-player-auth-dialog--mobile'
    : 'sd-confirm-dialog sd-confirm-dialog-wide sd-player-auth-dialog'
  const buttonSuffix = mobile ? ' sd-mctrl-dialog-btn' : ''
  const restartDialogTitleId = mobile ? 'mobile-player-auth-restart-title' : 'player-auth-restart-title'
  const restartDialogClass = mobile
    ? 'sd-panel sd-mctrl-dialog sd-player-auth-restart-dialog'
    : 'sd-confirm-dialog sd-player-auth-restart-dialog'

  return (
    <>
      <ModalPortal
        className={overlayClass}
        ariaLabelledBy={dialogTitleId}
        onEscape={saving || restarting ? undefined : onClose}
      >
      <div className={dialogClass}>
        <div className="sd-player-auth-heading">
          <div>
            <span className="sd-player-auth-kicker">IP 直连保护</span>
            <h3 id={dialogTitleId}>玩家加入保护</h3>
          </div>
          {config?.restartRequired ? <span className="sd-player-auth-restart-chip">待重启</span> : null}
        </div>

        {loading ? <p>正在读取玩家加入保护配置…</p> : null}

        {!loading && !config ? (
          <>
            <div role="alert" className={mobile ? 'sd-notice sd-notice--error sd-mctrl-notice' : 'sd-ov-error'}>
              {error ?? '未能读取玩家加入保护配置'}
            </div>
            <div className={`sd-player-auth-actions${mobile ? ' sd-mctrl-dialog-actions' : ''}`}>
              <button type="button" className={`sd-btn-tan${buttonSuffix}`} onClick={onClose}>关闭</button>
            </div>
          </>
        ) : null}

        {!loading && config ? (
          <>
            <div className="sd-player-auth-modes" role="group" aria-label="玩家加入保护模式">
              {modeOptions.map((option) => (
                <button
                  key={option.mode}
                  type="button"
                  className={`sd-player-auth-mode${mode === option.mode ? ' is-selected' : ''}`}
                  aria-pressed={mode === option.mode}
                  onClick={() => selectMode(option.mode)}
                  disabled={saving}
                >
                  <span className="sd-player-auth-mode-marker">{option.marker}</span>
                  <strong>{option.title}</strong>
                  <span>{option.summary}</span>
                </button>
              ))}
            </div>

            {mode === 'global' ? (
              <section className="sd-player-auth-config-card" aria-labelledby="global-password-label">
                <div className="sd-player-auth-section-title">
                  <div>
                    <strong><label id="global-password-label" htmlFor="global-player-auth-password">全服统一密码</label></strong>
                    <span>玩家选择任意未认领角色后，都输入这一密码。</span>
                  </div>
                </div>
                <div className="sd-player-auth-password-row">
                  <input
                    id="global-player-auth-password"
                    className="sd-input"
                    type={passwordVisible ? 'text' : 'password'}
                    value={globalPassword}
                    placeholder="输入 1–128 个字符"
                    maxLength={256}
                    autoComplete="new-password"
                    onChange={(event) => {
                      setGlobalPassword(event.target.value)
                      setError(null)
                      setMessage(null)
                    }}
                    disabled={saving}
                  />
                  <button
                    type="button"
                    className={`sd-btn-tan${buttonSuffix}`}
                    onClick={() => setPasswordVisible((visible) => !visible)}
                    disabled={saving}
                  >
                    {passwordVisible ? '隐藏' : '显示'}
                  </button>
                </div>
              </section>
            ) : null}

            {mode === 'role' ? (
              <section className="sd-player-auth-config-card" aria-labelledby="role-passwords-label">
                <div className="sd-player-auth-section-title">
                  <div>
                    <strong id="role-passwords-label">角色密码</strong>
                    <span>新角色和无密码记录的老角色，都会把第一次 !login 输入的密码设为自己的密码。</span>
                  </div>
                  <span className="sd-player-auth-count">
                    {config.configuredRoleCount} 已设置 · {config.unconfiguredRoleCount} 待设置
                  </span>
                </div>
                {!config.roleCredentialStoreReady ? (
                  <div className="sd-player-auth-store-error">
                    {config.roleCredentialStoreDetail ?? '角色凭据异常；为防止串号，相关登录已拒绝。'}
                  </div>
                ) : null}
                {config.roles.length ? (
                  <div className="sd-player-auth-role-list">
                    {config.roles.map((role) => {
                      const credentialStatus = role.credentialStatus ?? (role.configured ? 'configured' : 'waiting')
                      const pendingRemoval = Boolean(rolePasswordRemovals[role.roleId])
                      const inputId = `role-password-${role.roleId}`
                      const statusText = pendingRemoval
                        ? '保存后等待首次设置'
                        : credentialStatus === 'configured'
                          ? '已设置 · 输入新密码可重置'
                          : credentialStatus === 'error'
                            ? '凭据异常 · 登录已拒绝'
                            : '等待首次设置 · 玩家可自行认领'
                      return (
                        <div className="sd-player-auth-role" key={role.roleId}>
                          <label className="sd-player-auth-role-name" htmlFor={inputId}>
                            <strong>{role.name}</strong>
                            <span className={`is-${pendingRemoval ? 'waiting' : credentialStatus}`}>{statusText}</span>
                          </label>
                          <div className="sd-player-auth-role-control">
                            <input
                              id={inputId}
                              className="sd-input"
                              type={passwordVisible ? 'text' : 'password'}
                              value={rolePasswords[role.roleId] ?? ''}
                              placeholder={role.configured ? '输入新密码可重置' : '管理员可代为设置'}
                              maxLength={256}
                              autoComplete="new-password"
                              onChange={(event) => updateRolePassword(role.roleId, event.target.value)}
                              disabled={saving || credentialStatus === 'error'}
                            />
                            {role.configured || pendingRemoval ? (
                              <button
                                type="button"
                                className={`sd-btn-tan sd-player-auth-clear${buttonSuffix}`}
                                onClick={() => toggleRolePasswordRemoval(role.roleId)}
                                disabled={saving || credentialStatus === 'error'}
                              >
                                {pendingRemoval ? '撤销' : '清除'}
                              </button>
                            ) : null}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                ) : (
                  <div className="sd-player-auth-empty">
                    当前存档还没有非主机角色。可以先启用此模式；新角色或以后导入的无密码老角色，第一次 !login 会自行设置密码。
                  </div>
                )}
                {config.roles.length ? (
                  <button
                    type="button"
                    className={`sd-btn-tan sd-player-auth-visibility${buttonSuffix}`}
                    onClick={() => setPasswordVisible((visible) => !visible)}
                    disabled={saving}
                  >
                    {passwordVisible ? '隐藏本次输入' : '显示本次输入'}
                  </button>
                ) : null}
              </section>
            ) : null}

            {mode === 'none' ? (
              <div className="sd-player-auth-open-warning">
                关闭保护后，任何知道 IP 和端口的人都可以选择未认领角色。仅建议在可信内网使用。
              </div>
            ) : (
              <div className="sd-player-auth-login-tip">
                玩家加入后在游戏聊天框输入 <code>!login 密码</code>。
                {mode === 'role'
                  ? ' 未设置角色的第一次输入会设密并立即登录；角色密码修改无需重启。'
                  : ' 全服密码或保护模式修改需要按重启状态生效。'}
              </div>
            )}

            {error ? (
              <div role="alert" className={mobile ? 'sd-notice sd-notice--error sd-mctrl-notice' : 'sd-ov-error'}>{error}</div>
            ) : null}
            {message ? (
              <div role="status" aria-live="polite" className={mobile ? 'sd-notice sd-notice--ok sd-mctrl-notice' : 'sd-srv-result'}>{message}</div>
            ) : null}

            <section className="sd-player-auth-runtime" aria-label="当前生效状态">
              <div className="sd-player-auth-runtime-head">
                <div>
                  <strong>当前生效状态</strong>
                  <span>{isRunning ? '来自 JunimoServer 与 Control Mod' : '服务器未运行'}</span>
                </div>
                <button
                  type="button"
                  className={`sd-btn-tan${buttonSuffix}`}
                  onClick={() => void refreshStatus()}
                  disabled={!isRunning || statusLoading}
                >
                  {statusLoading ? '读取中…' : '刷新'}
                </button>
              </div>
              {mode !== 'none' ? <fieldset className="sd-player-auth-policy-fields" disabled={saving || restarting}>
                <legend>登录保护规则</legend>
                <div className="sd-player-auth-policy-field">
                  <label htmlFor={`${dialogTitleId}-timeout`}>认证超时时间</label>
                  <div className="sd-player-auth-number-input">
                    <input
                      id={`${dialogTitleId}-timeout`}
                      className="sd-input"
                      type="number"
                      inputMode="numeric"
                      min={minAuthTimeoutSeconds}
                      max={maxAuthTimeoutSeconds}
                      step={1}
                      value={timeoutSecondsInput}
                      aria-describedby={`${dialogTitleId}-timeout-help`}
                      onChange={(event) => {
                        setTimeoutSecondsInput(event.target.value)
                        setError(null)
                        setMessage(null)
                      }}
                    />
                    <span>秒</span>
                  </div>
                  <small id={`${dialogTitleId}-timeout-help`}>0 表示关闭超时，最多 3600 秒</small>
                </div>
                <div className="sd-player-auth-policy-field">
                  <label htmlFor={`${dialogTitleId}-attempts`}>最大失败次数</label>
                  <div className="sd-player-auth-number-input">
                    <input
                      id={`${dialogTitleId}-attempts`}
                      className="sd-input"
                      type="number"
                      inputMode="numeric"
                      min={minLoginAttempts}
                      max={maxLoginAttempts}
                      step={1}
                      value={maxAttemptsInput}
                      aria-describedby={`${dialogTitleId}-attempts-help`}
                      onChange={(event) => {
                        setMaxAttemptsInput(event.target.value)
                        setError(null)
                        setMessage(null)
                      }}
                    />
                    <span>次</span>
                  </div>
                  <small id={`${dialogTitleId}-attempts-help`}>输入错误达到次数后断开连接</small>
                </div>
              </fieldset> : null}
              <div className="sd-player-auth-runtime-grid">
                <span>已保存</span><strong>{modeLabel(config.mode)}</strong>
                <span>运行中</span><strong>{modeLabel(status?.runtimeMode ?? config.runtimeMode)}</strong>
                <span>重启状态</span><strong>{config.restartRequired || status?.restartRequired ? '需要重启' : '配置已同步'}</strong>
                <span>已保存规则</span><strong>{policyLabel(config.timeoutSeconds, config.maxAttempts)}</strong>
                <span>运行中规则</span>
                <strong>
                  {!isRunning
                    ? '服务器未运行'
                    : status
                      ? status.enabled
                        ? policyLabel(status.timeoutSeconds, status.maxAttempts)
                        : '保护未启用'
                      : '尚未读取'}
                </strong>
                {mode === 'role' ? (
                  <>
                    <span>角色密码补丁</span>
                    <strong>
                      {!isRunning
                        ? '启动后校验'
                        : status?.rolePasswordPatchReady ?? config.rolePasswordPatchReady
                          ? '已就绪'
                          : '未就绪'}
                    </strong>
                  </>
                ) : null}
              </div>
              {!isRunning ? <div className="sd-player-auth-runtime-note">下次启动服务器后可读取实际生效状态。</div> : null}
              {statusError ? <div role="alert" className="sd-player-auth-runtime-note is-error">{statusError}</div> : null}
              {isRunning && status ? (
                <div className="sd-player-auth-runtime-note">
                  已认证 {status.authenticatedCount} 人 · 待认证 {status.pendingCount} 人
                </div>
              ) : null}
            </section>

            <div className={`sd-player-auth-actions${mobile ? ' sd-mctrl-dialog-actions' : ''}`}>
              <button type="button" className={`sd-btn-tan${buttonSuffix}`} onClick={onClose} disabled={saving}>
                关闭
              </button>
              <button type="button" className={`sd-btn-green${buttonSuffix}`} onClick={() => void save()} disabled={saving}>
                {saving ? '保存中…' : '保存设置'}
              </button>
            </div>
          </>
        ) : null}
        </div>
      </ModalPortal>

      {restartPromptOpen ? (
        <ModalPortal
          className={overlayClass}
          role="alertdialog"
          ariaLabelledBy={restartDialogTitleId}
          onEscape={restarting ? undefined : postponeRestart}
        >
          <div className={restartDialogClass}>
            <h3 id={restartDialogTitleId}>重启服务器以应用设置</h3>
            <p>玩家加入保护配置已经保存。当前服务器仍在使用旧配置，立即重启后会自动切换到新设置。</p>
            <p className="sd-player-auth-restart-warning">重启会让在线玩家暂时断开，请先确认当前游戏进度已经保存。</p>
            {restartError ? (
              <div role="alert" className={mobile ? 'sd-notice sd-notice--error sd-mctrl-notice' : 'sd-ov-error'}>
                重启失败：{restartError}。配置已保存，可以重试或稍后手动重启。
              </div>
            ) : null}
            <div className={mobile ? 'sd-mctrl-dialog-actions' : 'sd-confirm-actions'}>
              <button type="button" className={`sd-btn-tan${buttonSuffix}`} onClick={postponeRestart} disabled={restarting}>
                稍后重启
              </button>
              <button type="button" className={`sd-btn-green${buttonSuffix}`} onClick={() => void restartNow()} disabled={restarting}>
                {restarting ? '正在重启…' : '立即重启'}
              </button>
            </div>
          </div>
        </ModalPortal>
      ) : null}
    </>
  )
}
