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
import './PlayerAuthSettingsDialog.css'

type PlayerAuthSettingsDialogProps = {
  isRunning: boolean
  mobile?: boolean
  onClose: () => void
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

function modeLabel(mode?: PlayerAuthMode): string {
  if (mode === 'global') return '全服统一密码'
  if (mode === 'role') return '角色独立密码'
  if (mode === 'none') return '不设密码'
  return '尚未读取'
}

export function PlayerAuthSettingsDialog({ isRunning, mobile = false, onClose }: PlayerAuthSettingsDialogProps) {
  const [config, setConfig] = useState<InstancePlayerAuthConfig | null>(null)
  const [mode, setMode] = useState<PlayerAuthMode>('none')
  const [globalPassword, setGlobalPassword] = useState('')
  const [rolePasswords, setRolePasswords] = useState<Record<string, string>>({})
  const [passwordVisible, setPasswordVisible] = useState(false)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [status, setStatus] = useState<InstancePasswordStatus | null>(null)
  const [statusLoading, setStatusLoading] = useState(false)
  const [statusError, setStatusError] = useState<string | null>(null)

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

  async function save() {
    if (!config) return
    const updates = config.roles
      .map((role) => ({ roleId: role.roleId, password: rolePasswords[role.roleId] ?? '' }))
      .filter((update) => update.password.length > 0)

    if (mode === 'global' && (globalPassword.length === 0 || [...globalPassword].length > 128)) {
      setError('全服密码必须为 1 到 128 个字符')
      setMessage(null)
      return
    }
    const invalidRolePassword = updates.find((update) => [...update.password].length > 128)
    if (invalidRolePassword) {
      setError('角色密码不能超过 128 个字符')
      setMessage(null)
      return
    }
    if (mode === 'role') {
      if (config.roles.length === 0) {
        setError('当前存档没有可配置的非主机角色，暂时不能启用角色独立密码')
        setMessage(null)
        return
      }
      const missingRole = config.roles.find(
        (role) => !role.configured && !(rolePasswords[role.roleId] ?? '').length,
      )
      if (missingRole) {
        setError(`请先为角色“${missingRole.name}”设置密码`)
        setMessage(null)
        return
      }
    }

    setSaving(true)
    setError(null)
    setMessage(null)
    try {
      const next = await updateInstancePlayerAuthConfig({
        expectedRevision: config.revision,
        mode,
        ...(mode === 'global' ? { globalPassword } : {}),
        ...(updates.length ? { rolePasswordUpdates: updates } : {}),
      })
      setConfig(next)
      setMode(next.mode)
      setGlobalPassword(next.globalPassword ?? '')
      setRolePasswords({})
      setMessage(
        next.restartRequired || isRunning
          ? '玩家加入保护已保存。请重启服务器容器，让新规则生效。'
          : '玩家加入保护已保存，将在下次启动服务器时生效。',
      )
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

  return (
    <ModalPortal
      className={overlayClass}
      ariaLabelledBy={dialogTitleId}
      onEscape={saving ? undefined : onClose}
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
            <div className={mobile ? 'sd-notice sd-notice--error sd-mctrl-notice' : 'sd-ov-error'}>
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
                    <strong id="global-password-label">全服统一密码</strong>
                    <span>玩家选择任意未认领角色后，都输入这一密码。</span>
                  </div>
                </div>
                <div className="sd-player-auth-password-row">
                  <input
                    className="sd-input"
                    type={passwordVisible ? 'text' : 'password'}
                    value={globalPassword}
                    placeholder="输入 1–128 个字符"
                    maxLength={128}
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
                    <span>密码和角色身份绑定；角色名仅用于显示，改名不会失效。</span>
                  </div>
                  <span className="sd-player-auth-count">
                    {config.configuredRoleCount}/{config.roles.length} 已设置
                  </span>
                </div>
                {config.roles.length ? (
                  <div className="sd-player-auth-role-list">
                    {config.roles.map((role) => (
                      <label className="sd-player-auth-role" key={role.roleId}>
                        <span className="sd-player-auth-role-name">
                          <strong>{role.name}</strong>
                          <span className={role.configured ? 'is-configured' : 'is-missing'}>
                            {role.configured ? '已设置 · 留空保持不变' : '未设置 · 启用前必须填写'}
                          </span>
                        </span>
                        <input
                          className="sd-input"
                          type={passwordVisible ? 'text' : 'password'}
                          value={rolePasswords[role.roleId] ?? ''}
                          placeholder={role.configured ? '输入新密码可重置' : '为此角色设置密码'}
                          maxLength={128}
                          autoComplete="new-password"
                          onChange={(event) => updateRolePassword(role.roleId, event.target.value)}
                          disabled={saving}
                        />
                      </label>
                    ))}
                  </div>
                ) : (
                  <div className="sd-player-auth-empty">
                    当前存档还没有可配置的非主机角色。先让角色进入过存档，再回来设置。
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
                玩家加入后在游戏聊天框输入 <code>!login 密码</code>。密码仅在服务器启动时载入。
              </div>
            )}

            {error ? (
              <div className={mobile ? 'sd-notice sd-notice--error sd-mctrl-notice' : 'sd-ov-error'}>{error}</div>
            ) : null}
            {message ? (
              <div className={mobile ? 'sd-notice sd-notice--ok sd-mctrl-notice' : 'sd-srv-result'}>{message}</div>
            ) : null}

            <div className={`sd-player-auth-actions${mobile ? ' sd-mctrl-dialog-actions' : ''}`}>
              <button type="button" className={`sd-btn-tan${buttonSuffix}`} onClick={onClose} disabled={saving}>
                关闭
              </button>
              <button type="button" className={`sd-btn-green${buttonSuffix}`} onClick={() => void save()} disabled={saving}>
                {saving ? '保存中…' : '保存设置'}
              </button>
            </div>

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
              <div className="sd-player-auth-runtime-grid">
                <span>已保存</span><strong>{modeLabel(config.mode)}</strong>
                <span>运行中</span><strong>{modeLabel(status?.runtimeMode ?? config.runtimeMode)}</strong>
                <span>重启状态</span><strong>{config.restartRequired || status?.restartRequired ? '需要重启' : '配置已同步'}</strong>
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
              {statusError ? <div className="sd-player-auth-runtime-note is-error">{statusError}</div> : null}
              {isRunning && status ? (
                <div className="sd-player-auth-runtime-note">
                  已认证 {status.authenticatedCount} 人 · 待认证 {status.pendingCount} 人 · 最多失败 {status.maxAttempts} 次
                </div>
              ) : null}
            </section>
          </>
        ) : null}
      </div>
    </ModalPortal>
  )
}
