import { useState } from 'react'
import type { FormEvent } from 'react'
import type { ImageTagOption } from '../../types'
import { StatusBadge } from '../../core/StatusBadge'
import { Field } from '../../core/Field'
import { PasswordInput } from '../../core/PasswordInput'
import { formatPercent } from '../../core/helpers'

export type InstallFormState = {
  steamUsername: string
  steamPassword: string
  vncPassword: string
  imageTag: string
}

type StepStatus = 'pending' | 'active' | 'done' | 'error'
type ProgressInfo = { percent: number; label: string; status: 'idle' | 'active' | 'done' | 'error' }
type DownloadProgress = {
  filesDone: number
  filesTotal: number
  percent: number
  bytesDone: string
  bytesTotal: string
}

export const emptyInstallForm: InstallFormState = { steamUsername: '', steamPassword: '', vncPassword: '', imageTag: '' }

function calcInstallProgress(state: string, phase: string, isInstalling: boolean, authFailed: boolean): ProgressInfo {
  if (state === 'game_installed') return { percent: 100, label: '安装完成', status: 'done' }
  if (phase === 'download_failed') return { percent: 88, label: '游戏文件下载失败，请检查网络/磁盘后重试', status: 'error' }
  if (authFailed || phase === 'qr_auth_failed') return { percent: 75, label: phase === 'qr_auth_failed' ? '二维码登录失败，请改用账号密码或 Steam Guard' : phase === 'credentials_required' ? 'Steam 认证失败，账号或密码错误' : 'Steam 认证失败，请查看任务日志', status: 'error' }
  if (phase === 'pull_failed') return { percent: 20, label: '镜像拉取失败，请检查网络后重试', status: 'error' }
  if (phase === 'install_timeout') return { percent: 75, label: '安装任务超时，请重试安装', status: 'error' }
  if (phase === 'steam_auth_connection_failed') return { percent: 75, label: 'Steam 连接建立超时，请检查网络后重试', status: 'error' }
  if (phase === 'steam_auth_retrying') return { percent: 68, label: 'Steam 连接较慢，正在自动重试认证...', status: 'active' }
  if (!isInstalling) return { percent: 0, label: '', status: 'idle' }
  switch (phase) {
    case 'junimo_scaffolded': return { percent: 15, label: '目录已准备，正在拉取镜像...', status: 'active' }
    case 'pull_running': return { percent: 35, label: '正在拉取 JunimoServer 镜像...', status: 'active' }
    case 'steam_auth_running': return { percent: 65, label: '正在使用 Steam 凭据认证并下载游戏...', status: 'active' }
    case 'auth_method_required': return { percent: 70, label: '等待选择 Steam 登录方式...', status: 'active' }
    case 'steam_guard_choice_required': return { percent: 75, label: '等待选择 Steam Guard 验证方式...', status: 'active' }
    case 'steam_guard_required': return { percent: 75, label: '等待 Steam Guard 验证码...', status: 'active' }
    case 'steam_guard_mobile_required': return { percent: 75, label: '请在手机 App 批准登录...', status: 'active' }
    case 'steam_qr_required': return { percent: 75, label: '请扫描 Steam 二维码...', status: 'active' }
    case 'game_downloading': return { percent: 88, label: '正在下载游戏文件（约10-30分钟）...', status: 'active' }
    case 'steam_sdk_downloading': return { percent: 94, label: '游戏文件已下载，正在下载 Steam SDK 运行文件...', status: 'active' }
    case 'steam_auth_done': return { percent: 92, label: 'Steam 认证成功，即将完成...', status: 'active' }
    default: return { percent: 8, label: '正在准备安装环境...', status: 'active' }
  }
}

function calcStepStatuses(
  state: string, phase: string, authFailed: boolean, isInstalling: boolean,
): [StepStatus, StepStatus, StepStatus, StepStatus] {
  const installed = state === 'game_installed'
  const isAuthPhase = ['steam_auth_running', 'auth_method_required', 'steam_guard_choice_required', 'steam_guard_required', 'steam_guard_mobile_required', 'steam_qr_required', 'steam_auth_done', 'game_downloading', 'steam_sdk_downloading'].includes(phase)
  const started = isInstalling || installed || authFailed || phase === 'pull_failed' || phase === 'install_timeout'

  const s1: StepStatus = started ? 'done' : 'pending'
  const s2: StepStatus = installed || isAuthPhase || phase === 'install_timeout' ? 'done' : phase === 'pull_failed' ? 'error' : isInstalling ? 'active' : 'pending'
  const s3: StepStatus = installed ? 'done' : authFailed || phase === 'install_timeout' ? 'error' : isAuthPhase ? 'active' : 'pending'
  const s4: StepStatus = installed ? 'done' : 'pending'
  return [s1, s2, s3, s4]
}

function DownloadProgressBody({ progress, waitingText }: { progress: DownloadProgress | null; waitingText: string }) {
  if (!progress) {
    return (
      <>
        <div className="progress-bar-wrap">
          <div className="progress-bar-track">
            <div className="progress-bar-fill" style={{ width: '0%' }} />
          </div>
          <span className="progress-bar-percent">0%</span>
        </div>
        <p className="game-download-detail">{waitingText}</p>
      </>
    )
  }
  return (
    <>
      <div className="progress-bar-wrap">
        <div className="progress-bar-track">
          <div
            className={`progress-bar-fill${progress.percent >= 100 ? ' done' : ''}`}
            style={{ width: `${progress.percent}%` }}
          />
        </div>
        <span className="progress-bar-percent">{formatPercent(progress.percent)}</span>
      </div>
      <p className="game-download-detail">
        {progress.filesDone} / {progress.filesTotal} 个文件
        {progress.bytesDone ? ` · ${progress.bytesDone} / ${progress.bytesTotal}` : ''}
      </p>
    </>
  )
}

export function InstallSection({
  state,
  phase,
  stateMessage,
  pullProgress,
  gameDownloadProgress,
  steamSdkDownloadProgress,
  steamDownloadTaskProgress,
  needsInstall,
  isInstalling,
  isInstalled,
  authFailed,
  isQrAuthError,
  needsAuthMethodChoice,
  needsGuard,
  needsGuardChoice,
  needsQrCode,
  steamQrLogText,
  installForm,
  installBusy,
  installMessage,
  installFailureMessage,
  showInstallModal,
  guardInput,
  guardBusy,
  guardMessage,
  imageTagOptions,
  canDirectRetry,
  onInstallClick,
  onInstallFormChange,
  onShowInstallModal,
  onInstallSubmit,
  onGuardInputChange,
  onGuardSubmit,
  onAuthMethodSelect,
}: {
  state: string
  phase: string
  stateMessage: string
  pullProgress: { done: number; total: number; percent: number } | null
  gameDownloadProgress: DownloadProgress | null
  steamSdkDownloadProgress: DownloadProgress | null
  steamDownloadTaskProgress: { done: number; total: number; percent: number; label: string } | null
  needsInstall: boolean
  isInstalling: boolean
  isInstalled: boolean
  authFailed: boolean
  isQrAuthError: boolean
  needsAuthMethodChoice: boolean
  needsGuard: boolean
  needsGuardChoice: boolean
  needsQrCode: boolean
  steamQrLogText: string
  installForm: InstallFormState
  installBusy: boolean
  installMessage: string
  installFailureMessage: string
  showInstallModal: boolean
  guardInput: string
  guardBusy: boolean
  guardMessage: string
  imageTagOptions: ImageTagOption[]
  canDirectRetry: boolean
  onInstallClick: () => void
  onInstallFormChange: (f: InstallFormState) => void
  onShowInstallModal: (v: boolean) => void
  onInstallSubmit: (e: FormEvent<HTMLFormElement>) => void
  onGuardInputChange: (v: string) => void
  onGuardSubmit: (e: FormEvent<HTMLFormElement>) => void
  onAuthMethodSelect: (choice: string) => void
}) {
  const [showSteamPwd, setShowSteamPwd] = useState(false)
  const [showVncPwd, setShowVncPwd] = useState(false)
  const [showQrModal, setShowQrModal] = useState(false)

  const selectedOption = imageTagOptions.find((o) => o.tag === installForm.imageTag)
  const selectedWarning = selectedOption?.warning ?? ''

  const progress = calcInstallProgress(state, phase, isInstalling, authFailed)
  const stepStatuses = calcStepStatuses(state, phase, authFailed, isInstalling)
  const showProgress = isInstalling || isInstalled || authFailed || phase === 'pull_failed' || phase === 'install_timeout'

  return (
    <section className="install-section">
      <div className="section-heading">
        <div>
          <h2>Stardew Valley 安装</h2>
          <p>管理员通过此区域安装游戏并完成 Steam 认证。</p>
        </div>
      </div>

      {installMessage ? <div className="error-banner">{installMessage}</div> : null}
      {!installMessage && installFailureMessage ? <div className="error-banner">{installFailureMessage}</div> : null}

      {/* 状态和操作 */}
      <div className="install-status-row">
        <div className="install-state-info">
          <span>当前状态：</span>
          <StatusBadge status={state || 'unknown'} />
          {phase && phase !== 'empty' ? <small>阶段：{phase}</small> : null}
        </div>
        <div className="install-actions">
          {/* 安装游戏 / 重试安装按钮 */}
          {needsInstall || authFailed ? (
            <button className="button" disabled={installBusy || isInstalling}
              onClick={onInstallClick} type="button">
              {isQrAuthError ? '二维码失败，改用账号密码重试' : authFailed ? '重新安装（凭据错误）' : (canDirectRetry || state === 'error') ? '重试安装' : '安装游戏'}
            </button>
          ) : null}

          {/* 已安装 */}
          {isInstalled ? (
            <div className="install-complete">
              <span className="status-badge succeeded">已安装</span>
            </div>
          ) : null}
        </div>
      </div>

      {/* 进度条 */}
      {showProgress ? (
        <div className="install-progress-section">
          <ol className="install-steps">
            {(['准备环境', '拉取镜像', 'Steam 认证', '完成'] as const).map((label, i) => (
              <li key={i} className={`install-step ${stepStatuses[i]}`}>
                <span className="step-icon">
                  {stepStatuses[i] === 'done' ? '✓' : stepStatuses[i] === 'error' ? '✗' : stepStatuses[i] === 'active' ? '↻' : '○'}
                </span>
                <span className="step-label">{label}</span>
              </li>
            ))}
          </ol>
          <div className="progress-bar-wrap">
            <div className="progress-bar-track">
              <div
                className={`progress-bar-fill ${progress.status}`}
                style={{ width: `${progress.percent}%` }}
              />
            </div>
            <span className="progress-bar-percent">{progress.percent}%</span>
          </div>
          {phase !== 'pull_running' && progress.label ? <p className="progress-bar-label">{progress.label}</p> : null}

          {/* pull 阶段专用用户状态卡 */}
          {phase === 'pull_running' && isInstalling ? (
            <div className="pull-status-card">
              <div className="pull-status-header">
                <span className="pull-status-spinner">↓</span>
                <div className="pull-status-text">
                  <strong>正在下载 JunimoServer 镜像</strong>
                  <p>{stateMessage || '正在准备拉取镜像，请稍候...'}</p>
                </div>
              </div>
              {pullProgress ? (
                <div className="pull-images-bar">
                  <div className="pull-images-bar-track">
                    <div
                      className={`pull-images-bar-fill${pullProgress.done === pullProgress.total ? ' done' : ''}`}
                      style={{ width: `${pullProgress.percent}%` }}
                    />
                  </div>
                  <span className="pull-images-bar-label">{pullProgress.done} / {pullProgress.total} 个镜像</span>
                </div>
              ) : (
                <div className="pull-images-waiting">等待 Docker 开始下载...</div>
              )}
              <p className="pull-status-hint">
                首次下载约需 10–30 分钟，取决于网络速度。如果超过 15 分钟仍无变化，请检查网络连接后点击"重试安装"。
              </p>
            </div>
          ) : null}
          {(phase === 'game_downloading' || phase === 'steam_sdk_downloading') && isInstalling ? (
            <div className="game-download-card">
              <div className="game-download-header">
                <strong>下载任务进度</strong>
                <span>{steamDownloadTaskProgress ? `${steamDownloadTaskProgress.done}/${steamDownloadTaskProgress.total}` : '等待下载'}</span>
              </div>
              {steamDownloadTaskProgress ? (
                <>
                  <div className="progress-bar-wrap">
                    <div className="progress-bar-track">
                      <div
                        className={`progress-bar-fill${steamDownloadTaskProgress.percent >= 100 ? ' done' : ''}`}
                        style={{ width: `${steamDownloadTaskProgress.percent}%` }}
                      />
                    </div>
                    <span className="progress-bar-percent">{formatPercent(steamDownloadTaskProgress.percent)}</span>
                  </div>
                  <p className="game-download-detail">{steamDownloadTaskProgress.label}</p>
                </>
              ) : (
                <p className="game-download-detail">正在校验已有文件并连接 Steam 下载服务器...</p>
              )}
            </div>
          ) : null}
          {(phase === 'game_downloading' || phase === 'steam_sdk_downloading') && isInstalling ? (
            <div className="game-download-card">
              <div className="game-download-header">
                <strong>{phase === 'steam_sdk_downloading' ? 'Stardew Valley 游戏文件' : '正在下载 Stardew Valley 游戏文件'}</strong>
                <span>{gameDownloadProgress ? formatPercent(gameDownloadProgress.percent) : '等待进度'}</span>
              </div>
              <DownloadProgressBody
                progress={gameDownloadProgress}
                waitingText="正在等待 steam-auth 输出游戏文件下载百分比..."
              />
            </div>
          ) : null}
          {phase === 'steam_sdk_downloading' && isInstalling ? (
            <div className="game-download-card">
              <div className="game-download-header">
                <strong>正在下载 Steam SDK 运行文件</strong>
                <span>{steamSdkDownloadProgress ? formatPercent(steamSdkDownloadProgress.percent) : '等待进度'}</span>
              </div>
              <DownloadProgressBody
                progress={steamSdkDownloadProgress}
                waitingText="正在与 Steam 下载服务器建立连接中..."
              />
            </div>
          ) : null}
        </div>
      ) : null}

      {isQrAuthError ? (
        <div className="error-banner">
          二维码登录失败：当前 Junimo steam-auth 容器在生成二维码前无法连接 SteamClient。请点击下方重试，并改用账号密码登录；如果 Steam 需要二次验证，再按提示输入 Steam Guard。
        </div>
      ) : null}

      {/* Steam 认证交互区域 */}
      {(needsAuthMethodChoice || needsGuardChoice || needsGuard || needsQrCode) ? (
        <div className="steam-guard-section">
          {needsAuthMethodChoice ? (
            <>
              <h3>选择 Steam 登录方式</h3>
              <p>请选择本次认证使用扫码登录，还是使用已填写的 Steam 账号密码继续；后者如果触发二次验证，会再选择手机 App 或验证码。</p>
              <div className="auth-method-actions">
                <button
                  className="button"
                  disabled={guardBusy}
                  onClick={() => onAuthMethodSelect('2')}
                  type="button"
                >
                  {guardBusy ? '提交中...' : '扫码登录'}
                </button>
                <button
                  className="button button-secondary"
                  disabled={guardBusy}
                  onClick={() => onAuthMethodSelect('1')}
                  type="button"
                >
                  {guardBusy ? '提交中...' : '账号密码/验证码登录'}
                </button>
              </div>
              {guardMessage ? <p className="form-hint">{guardMessage}</p> : null}
            </>
          ) : null}
          {needsGuardChoice ? (
            <>
              <h3>选择 Steam Guard 验证方式</h3>
              <p>Steam 要求二步验证，请选择和日志菜单对应的验证方式。</p>
              <div className="auth-method-actions">
                <button
                  className="button"
                  disabled={guardBusy}
                  onClick={() => onAuthMethodSelect('1')}
                  type="button"
                >
                  {guardBusy ? '提交中...' : '手机 App 批准'}
                </button>
                <button
                  className="button button-secondary"
                  disabled={guardBusy}
                  onClick={() => onAuthMethodSelect('2')}
                  type="button"
                >
                  {guardBusy ? '提交中...' : '输入验证码'}
                </button>
              </div>
              {guardMessage ? <p className="form-hint">{guardMessage}</p> : null}
            </>
          ) : null}
          {needsGuard ? (
            <>
              <h3>Steam Guard 验证</h3>
              {phase === 'steam_guard_required' ? (
                <>
                  <p>Steam 发送了邮箱验证码，请在下方输入：</p>
                  <form className="guard-form" onSubmit={onGuardSubmit}>
                    <input
                      type="text"
                      placeholder="输入 Steam Guard 验证码"
                      value={guardInput}
                      onChange={(e) => onGuardInputChange(e.target.value)}
                      autoComplete="off"
                      required
                    />
                    <button className="button" disabled={guardBusy} type="submit">
                      {guardBusy ? '提交中...' : '提交验证码'}
                    </button>
                  </form>
                  {guardMessage ? <p className="form-hint">{guardMessage}</p> : null}
                </>
              ) : null}
              {phase === 'steam_guard_mobile_required' ? (
                <p>请打开 Steam 手机 App，批准此次登录请求后继续。</p>
              ) : null}
            </>
          ) : null}

          {needsQrCode ? (
            <>
              <h3>Steam 手机扫码</h3>
              <p>请使用 Steam 手机 App 扫描弹窗中的二维码。如果二维码还没出现，请稍等几秒。</p>
              <button
                className="button"
                disabled={!steamQrLogText}
                onClick={() => setShowQrModal(true)}
                type="button"
              >
                打开扫码窗口
              </button>
              {!steamQrLogText ? <p className="form-hint">正在等待容器输出二维码...</p> : null}
            </>
          ) : null}
        </div>
      ) : null}

      {showQrModal ? (
        <div className="modal-overlay qr-modal-overlay" role="dialog" aria-modal="true">
          <div className="modal-card steam-qr-modal-card">
            <div className="qr-modal-heading">
              <h2>Steam 手机扫码</h2>
              <button className="button button-small button-secondary" type="button" onClick={() => setShowQrModal(false)}>
                关闭
              </button>
            </div>
            {steamQrLogText ? (
              <pre className="steam-qr-modal-code" style={{ fontSize: `${qrCodeFontSize(steamQrLogText)}px` }}>
                {steamQrLogText}
              </pre>
            ) : (
              <p className="form-hint">正在等待容器输出二维码...</p>
            )}
          </div>
        </div>
      ) : null}

      {/* 安装 Modal */}
      {showInstallModal ? (
        <div className="modal-overlay" role="dialog" aria-modal="true">
          <div className="modal-card">
            <h2>
              {isQrAuthError ? '改用账号密码登录' : authFailed ? '重新输入 Steam 凭据' : canDirectRetry ? '选择镜像版本重试' : '安装 Stardew Valley'}
            </h2>
            <p className="summary">
              {isQrAuthError
                ? '二维码登录已失败，请改用账号密码登录。若 Steam 需要二次验证，后续会提示输入 Steam Guard。'
                : canDirectRetry
                  ? '将使用已保存的 Steam 凭据重新安装，只需确认镜像版本。'
                  : '请输入 Steam 账号信息和 VNC 密码。这些信息将被写入实例目录的 .env 文件，不会出现在任何日志中。'}
            </p>
            <form className="form-grid" onSubmit={onInstallSubmit} autoComplete="off">
              {imageTagOptions.length > 0 ? (
                <Field label="JunimoServer 镜像版本">
                  <select
                    value={installForm.imageTag}
                    onChange={(e) => onInstallFormChange({ ...installForm, imageTag: e.target.value })}
                  >
                    {imageTagOptions.map((opt) => (
                      <option key={opt.tag + opt.label} value={opt.tag}>
                        {opt.label}{opt.recommended ? ' ★' : ''}{opt.isLatest ? ' 已是最新版' : ''}
                      </option>
                    ))}
                  </select>
                  {selectedWarning ? (
                    <p className="version-warning">{selectedWarning}</p>
                  ) : null}
                </Field>
              ) : null}
              {!canDirectRetry ? (
                <>
                  <Field label="Steam 用户名">
                    <input
                      type="text"
                      value={installForm.steamUsername}
                      autoComplete="steam-account"
                      required
                      onChange={(e) => onInstallFormChange({ ...installForm, steamUsername: e.target.value })}
                    />
                  </Field>
                  <Field label="Steam 密码">
                    <PasswordInput
                      value={installForm.steamPassword}
                      visible={showSteamPwd}
                      autoComplete="new-password"
                      onChange={(p) => onInstallFormChange({ ...installForm, steamPassword: p })}
                      onToggle={() => setShowSteamPwd((v) => !v)}
                    />
                  </Field>
                  <Field label="VNC 密码">
                    <PasswordInput
                      value={installForm.vncPassword}
                      visible={showVncPwd}
                      autoComplete="new-password"
                      onChange={(p) => onInstallFormChange({ ...installForm, vncPassword: p })}
                      onToggle={() => setShowVncPwd((v) => !v)}
                    />
                  </Field>
                  <p className="form-hint">密码不会打印到任何日志或浏览器控制台。</p>
                </>
              ) : null}
              <div className="modal-actions">
                <button className="button" disabled={installBusy} type="submit">
                  {installBusy ? '正在启动安装...' : canDirectRetry ? '确认重试' : '确认安装'}
                </button>
                <button className="button button-secondary" disabled={installBusy} type="button"
                  onClick={() => onShowInstallModal(false)}>取消</button>
              </div>
              {installMessage ? <div className="error-banner">{installMessage}</div> : null}
            </form>
          </div>
        </div>
      ) : null}
    </section>
  )
}

function qrCodeFontSize(text: string) {
  const lines = text.split('\n')
  const longest = lines.reduce((max, line) => Math.max(max, line.length), 0)
  if (lines.length > 42 || longest > 92) return 9
  if (lines.length > 36 || longest > 82) return 10
  if (lines.length > 30 || longest > 72) return 11
  return 12
}
