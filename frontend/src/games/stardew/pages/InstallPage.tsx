import { useCallback, useEffect, useRef, useState } from 'react'
import type { CSSProperties, FormEvent } from 'react'
import type { ImageTagOption, Job, JobLog } from '../../../types'
import type { StardewPageProps } from '../stardew-routes'
import {
  createJobEventSource,
  getInstallOptions,
  getJob,
  getJobLogs,
  installInstance,
  submitSteamGuardInput,
} from '../../../api'
import {
  appendUniqueLog,
  errorMessage,
  isTerminalJobStatus,
} from '../../../core/helpers'

// ── 进度工具 ──────────────────────────────────────────────────────────────────

const PULL_PROGRESS_RE = /^\[pull:progress:(\d+):(\d+)\]$/

function extractPullProgress(logs: JobLog[]): { done: number; total: number; percent: number } | null {
  let latest: { done: number; total: number } | null = null
  for (const log of logs) {
    const m = log.message.match(PULL_PROGRESS_RE)
    if (m) latest = { done: parseInt(m[1], 10), total: parseInt(m[2], 10) }
  }
  if (!latest || latest.total === 0) return null
  return { ...latest, percent: Math.round((latest.done / latest.total) * 100) }
}

function extractQrText(logs: JobLog[]): string {
  // QR code is always the most recent steam-auth output, so take the last 80 [steam] lines.
  // This avoids mixing earlier auth menus/errors with the actual QR block.
  const steamLines = logs
    .filter((l) => l.message.startsWith('[steam] '))
    .map((l) => l.message.slice('[steam] '.length))
  return steamLines.slice(-80).join('\n')
}

function qrCodeFontSize(text: string): number {
  const lines = text.split('\n')
  const longest = lines.reduce((max, line) => Math.max(max, line.length), 0)
  if (lines.length > 42 || longest > 92) return 9
  if (lines.length > 36 || longest > 82) return 10
  if (lines.length > 30 || longest > 72) return 11
  return 12
}

// ── 阶段→步骤状态 ──────────────────────────────────────────────────────────────

type StepStatus = 'pending' | 'active' | 'done' | 'error'

const AUTH_FAILED_PHASES = [
  'steam_auth_failed', 'credentials_required', 'qr_auth_failed',
  'steam_auth_console_failed', 'steam_auth_connection_failed',
]

function calcStepStatuses(
  state: string,
  phase: string,
  authFailed: boolean,
  isInstalling: boolean,
): [StepStatus, StepStatus, StepStatus, StepStatus, StepStatus] {
  const installed = ['game_installed', 'save_required', 'ready_to_start', 'starting', 'running', 'stopped'].includes(state)
  // Phases where Steam authentication is actively happening (not yet done)
  const authPhases = [
    'steam_auth_running', 'auth_method_required', 'steam_guard_choice_required',
    'steam_guard_required', 'steam_guard_mobile_required', 'steam_qr_required',
    'steam_auth_done',
  ]
  // Phases where auth is already complete and game download is in progress
  const downloadPhases = ['game_downloading', 'steam_sdk_downloading']

  const isAuthPhase = authPhases.includes(phase)
  const isDownloadPhase = downloadPhases.includes(phase)
  const started = isInstalling || installed || authFailed
    || phase === 'pull_failed' || phase === 'install_timeout'

  const s1: StepStatus = started ? 'done' : 'pending'
  const s2: StepStatus =
    installed || isAuthPhase || isDownloadPhase || phase === 'install_timeout' ? 'done'
    : phase === 'pull_failed' ? 'error'
    : isInstalling ? 'active'
    : 'pending'
  const s3: StepStatus =
    installed || isDownloadPhase ? 'done'        // auth done, now downloading
    : authFailed || phase === 'install_timeout' ? 'error'
    : isAuthPhase ? 'active'
    : 'pending'
  const s4: StepStatus =
    installed ? 'done'
    : isDownloadPhase ? 'active'
    : phase === 'download_failed' ? 'error'
    : 'pending'
  const s5: StepStatus = installed ? 'done' : 'pending'
  return [s1, s2, s3, s4, s5]
}

function phaseLabel(phase: string, isInstalling: boolean, authFailed: boolean, state: string): string {
  if (['game_installed', 'save_required', 'ready_to_start', 'starting', 'running', 'stopped'].includes(state)) return '安装完成'
  if (phase === 'download_failed') return '游戏文件下载失败，请检查网络/磁盘后重试'
  if (phase === 'qr_auth_failed') return '二维码登录失败，请改用账号密码或 Steam Guard'
  if (phase === 'credentials_required' && authFailed) return 'Steam 认证失败，账号或密码错误'
  if (authFailed) return 'Steam 认证失败，请查看任务日志'
  if (phase === 'pull_failed') return '镜像拉取失败，请检查网络后重试'
  if (phase === 'install_timeout') return '安装任务超时，请重试安装'
  if (phase === 'steam_auth_connection_failed') return 'Steam 连接建立超时，请检查网络后重试'
  if (phase === 'steam_auth_retrying') return 'Steam 连接较慢，正在自动重试认证...'
  if (!isInstalling) return ''
  const labels: Record<string, string> = {
    junimo_scaffolded: '目录已准备，正在拉取镜像...',
    pull_running: '正在拉取 JunimoServer 镜像...',
    steam_auth_running: '正在使用 Steam 凭据认证并下载游戏...',
    auth_method_required: '等待选择 Steam 登录方式...',
    steam_guard_choice_required: '等待选择 Steam Guard 验证方式...',
    steam_guard_required: '等待 Steam Guard 验证码...',
    steam_guard_mobile_required: '请在手机 App 批准登录...',
    steam_qr_required: '请扫描 Steam 二维码...',
    game_downloading: '正在下载游戏文件（约 10–30 分钟）...',
    steam_sdk_downloading: '游戏文件已下载，正在下载 Steam SDK 运行文件...',
    steam_auth_done: 'Steam 认证成功，即将完成...',
  }
  return labels[phase] ?? '正在准备安装环境...'
}

const STEP_ICON: Record<StepStatus, string> = {
  done: '✓', error: '✗', active: '↻', pending: '○',
}
const STEPS = ['准备环境', '拉取镜像', 'Steam 认证', '下载游戏', '完成'] as const
const STEP_ICON_SRC = [
  '/assets/stardew/ui/install/icon_install_step_seed_image2.png',
  '/assets/stardew/ui/install/icon_install_step_box_image2.png',
  '/assets/stardew/ui/install/icon_install_step_steam_image2.png',
  '/assets/stardew/ui/install/icon_install_step_download_image2.png',
  '/assets/stardew/ui/install/icon_install_step_star_image2.png',
] as const

// ── 组件 ──────────────────────────────────────────────────────────────────────

export function InstallPage({ user, instanceState, dashboardData, onNavigate }: StardewPageProps) {
  const state = instanceState?.state ?? ''
  const phase = instanceState?.driverPhase ?? ''
  const stateMessage = instanceState?.stateMessage ?? ''

  const isAdmin = user.role === 'admin'
  // Any state after game_installed means the game is installed (server may be running/stopped).
  const INSTALLED_STATES = ['game_installed', 'save_required', 'ready_to_start', 'starting', 'running', 'stopped']
  const isInstalled = INSTALLED_STATES.includes(state)
  const authFailed = AUTH_FAILED_PHASES.includes(phase)
  const isQrAuthError = phase === 'qr_auth_failed'
  const needsAuthMethodChoice = phase === 'auth_method_required'
  const needsGuardChoice = phase === 'steam_guard_choice_required'
  const needsGuard = phase === 'steam_guard_required' || phase === 'steam_guard_mobile_required'
  const needsQrCode = phase === 'steam_qr_required'
  const canDirectRetry = state === 'error'
    || ['pull_failed', 'install_timeout', 'steam_auth_connection_failed'].includes(phase)

  // ── 镜像选项 ──────────────────────────────────────────────────────────────────
  const [imageTagOptions, setImageTagOptions] = useState<ImageTagOption[]>([])
  const [optionsLoading, setOptionsLoading] = useState(true)
  const [imageTag, setImageTag] = useState('')

  useEffect(() => {
    setOptionsLoading(true)
    getInstallOptions()
      .then((res) => {
        setImageTagOptions(res.imageTagOptions)
        setImageTag((prev) => {
          if (prev) return prev
          const rec = res.imageTagOptions.find((o) => o.recommended) ?? res.imageTagOptions[0]
          return rec?.tag ?? ''
        })
      })
      .catch(() => { /* 静默失败，镜像列表为空时不显示选择 */ })
      .finally(() => setOptionsLoading(false))
  }, [])

  // ── 安装 Job ──────────────────────────────────────────────────────────────────
  const [installJobId, setInstallJobId] = useState<string | null>(null)
  const [installJob, setInstallJob] = useState<Job | null>(null)
  const [logs, setLogs] = useState<JobLog[]>([])
  const [sseError, setSseError] = useState('')
  const logEndRef = useRef<HTMLDivElement>(null)

  // Phases that indicate an install is actively running, even before installJob loads from async effect
  const INSTALLING_PHASES = [
    'pull_running', 'steam_auth_running', 'auth_method_required',
    'steam_guard_choice_required', 'steam_guard_required', 'steam_guard_mobile_required',
    'steam_qr_required', 'steam_auth_retrying', 'steam_auth_done',
    'game_downloading', 'steam_sdk_downloading',
  ]
  const isInstalling = (installJob !== null && !isTerminalJobStatus(installJob.status))
    || (!isInstalled && INSTALLING_PHASES.includes(phase))

  // 当 dashboardData.jobs 变化时，自动拾取活跃的安装任务
  useEffect(() => {
    if (installJobId) return
    const active = dashboardData.jobs.find(
      (j) => j.type === 'stardew_install' && !isTerminalJobStatus(j.status),
    )
    if (active) setInstallJobId(active.id)
  }, [dashboardData.jobs, installJobId])

  // 当 installJobId 变化时加载详情 + 日志，并连接 SSE
  useEffect(() => {
    if (!installJobId) return
    let cancelled = false
    let es: EventSource | null = null

    const load = async () => {
      try {
        const [jobRes, logsRes] = await Promise.all([
          getJob(installJobId),
          getJobLogs(installJobId, 0, 1000),
        ])
        if (cancelled) return
        setInstallJob(jobRes.job)
        setLogs(logsRes.logs)

        if (isTerminalJobStatus(jobRes.job.status)) return

        const lastSeq = logsRes.logs.length > 0 ? logsRes.logs[logsRes.logs.length - 1].sequence : 0
        const currentJobId = installJobId
        es = createJobEventSource(currentJobId, lastSeq)

        es.addEventListener('log', (ev) => {
          if (cancelled) { es?.close(); return }
          try {
            const entry = JSON.parse((ev as MessageEvent<string>).data) as JobLog
            setLogs((prev) => appendUniqueLog(prev, { ...entry, jobId: currentJobId }))
          } catch { /* 忽略格式错误 */ }
        })

        es.addEventListener('finished', () => {
          es?.close()
          if (cancelled) return
          void getJob(currentJobId).then((r) => {
            if (!cancelled) setInstallJob(r.job)
          })
          dashboardData.refreshJobs()
          dashboardData.refreshInstanceState()
        })

        es.onerror = () => {
          if (!cancelled) setSseError('实时日志连接已断开，可手动刷新查看最新日志。')
          es?.close()
        }
      } catch {
        if (!cancelled) setInstallJob(null)
      }
    }

    void load()
    return () => {
      cancelled = true
      es?.close()
    }
  }, [installJobId]) // dashboardData.refresh* 是稳定引用，intentionally omitted

  // 日志自动滚动
  useEffect(() => {
    logEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs.length])

  // ── 表单 ──────────────────────────────────────────────────────────────────────
  const [showForm, setShowForm] = useState(false)
  const [steamUsername, setSteamUsername] = useState('')
  const [steamPassword, setSteamPassword] = useState('')
  const [vncPassword, setVncPassword] = useState('')
  const [showSteamPwd, setShowSteamPwd] = useState(false)
  const [showVncPwd, setShowVncPwd] = useState(false)
  const [installBusy, setInstallBusy] = useState(false)
  const [installError, setInstallError] = useState('')

  const handleInstallSubmit = useCallback(async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!isAdmin) return
    setInstallError('')
    setInstallBusy(true)
    try {
      const body = canDirectRetry
        ? { reuseCredentials: true, imageTag }
        : { steamUsername, steamPassword, vncPassword, imageTag }
      const res = await installInstance(body)
      setInstallJobId(res.jobId)
      setLogs([])
      setInstallJob(null)
      setSseError('')
      setShowForm(false)
      dashboardData.refreshJobs()
      dashboardData.refreshInstanceState()
    } catch (err) {
      setInstallError(errorMessage(err))
    } finally {
      setInstallBusy(false)
    }
  }, [isAdmin, canDirectRetry, imageTag, steamUsername, steamPassword, vncPassword, dashboardData])

  // ── Steam Guard ───────────────────────────────────────────────────────────────
  const [guardInput, setGuardInput] = useState('')
  const [guardBusy, setGuardBusy] = useState(false)
  const [guardError, setGuardError] = useState('')

  const handleGuardSubmit = useCallback(async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!installJobId) return
    setGuardError('')
    setGuardBusy(true)
    try {
      await submitSteamGuardInput(installJobId, guardInput)
      setGuardInput('')
    } catch (err) {
      setGuardError(errorMessage(err))
    } finally {
      setGuardBusy(false)
    }
  }, [installJobId, guardInput])

  const handleAuthMethodSelect = useCallback(async (choice: string) => {
    if (!installJobId) return
    setGuardBusy(true)
    setGuardError('')
    try {
      await submitSteamGuardInput(installJobId, choice)
    } catch (err) {
      setGuardError(errorMessage(err))
    } finally {
      setGuardBusy(false)
    }
  }, [installJobId])

  // ── QR 弹窗 ───────────────────────────────────────────────────────────────────
  const [showQrModal, setShowQrModal] = useState(false)

  // ── 计算值 ───────────────────────────────────────────────────────────────────
  const pullProgress = extractPullProgress(logs)
  const qrText = extractQrText(logs)
  const stepStatuses = calcStepStatuses(state, phase, authFailed, isInstalling)
  const showProgress = isInstalling || isInstalled || authFailed
    || ['pull_failed', 'install_timeout', 'download_failed'].includes(phase)
  const progressLabel = phaseLabel(phase, isInstalling, authFailed, state)
  const selectedOption = imageTagOptions.find((o) => o.tag === imageTag)

  const visibleLogs = logs.slice(-50)
  const finishedStepCount = stepStatuses.filter((status) => status === 'done').length
  const hasActiveStep = stepStatuses.some((status) => status === 'active')
  const overallProgress = isInstalled
    ? 100
    : showProgress
      ? Math.min(96, (finishedStepCount * 20) + (hasActiveStep ? 8 : 0))
      : 0

  return (
    <div className="sd-page sd-install-page">
      {/* ── 页面头部 + 状态横幅 ───────────────────────────────────────────── */}
      <section className="sd-install-hero" aria-labelledby="sd-install-title">
        <div className="sd-install-title-strip">
          <img
            className="sd-page-icon"
            src="/assets/stardew/ui/icons/icon_nav_install_package_image2.png"
            alt=""
          />
          <h2 id="sd-install-title" className="sd-page-title">首次安装向导</h2>
        </div>

        <div className="sd-install-status-banner">
          <div className="sd-install-seed-art" aria-hidden="true">
            <img
              className="sd-install-seed-img"
              src="/assets/stardew/ui/install/icon_install_status_seed_image2.png"
              alt=""
            />
          </div>
          <div className="sd-state-card">
            <div className="sd-state-row">
              <span className="sd-state-label">安装状态</span>
              {isInstalled ? (
                <><span className="sd-dot sd-dot-green" aria-hidden="true" /><span className="sd-state-value">已安装</span></>
              ) : isInstalling ? (
                <><span className="sd-dot sd-dot-yellow" aria-hidden="true" /><span className="sd-state-value">安装中...</span></>
              ) : authFailed ? (
                <><span className="sd-dot sd-dot-red" aria-hidden="true" /><span className="sd-state-value">认证失败</span></>
              ) : (
                <><span className="sd-dot sd-dot-gray" aria-hidden="true" /><span className="sd-state-value">未安装</span></>
              )}
            </div>
            <div className="sd-state-row">
              <span className="sd-state-label">当前阶段</span>
              {phase && phase !== 'empty' ? (
                <span className="sd-install-phase-tag">{phase}</span>
              ) : (
                <span className="sd-install-state-msg">等待开始</span>
              )}
            </div>
            <div className="sd-state-row">
              <span className="sd-state-label">状态说明</span>
              <span className="sd-install-state-msg">
                {stateMessage || (isInstalled
                  ? 'Stardew Valley Dedicated Server 已成功安装并可运行！'
                  : '配置 Steam 凭据并安装 Stardew Valley 服务器（含 SMAPI + JunimoServer）。')}
              </span>
            </div>
          </div>
          <div className="sd-install-farm-scene" aria-hidden="true">
            <img src="/assets/stardew/ui/sprites/sprite_farmhouse_scene.png" alt="" />
          </div>
        </div>
      </section>

      {/* ── 安装进度区 ──────────────────────────────────────────────────── */}
      <section className="sd-install-progress-section">
        <div className="sd-install-section-title">安装进度</div>

        {/* 步骤条 */}
        <ol
          className="sd-install-steps"
          style={{ '--sd-install-progress-line': `${overallProgress}%` } as CSSProperties}
        >
          {STEPS.map((label, i) => (
            <li key={i} className={`sd-install-step sd-install-step-${stepStatuses[i]}`}>
              <span className="sd-install-step-number">{i + 1}</span>
              <img className="sd-install-step-art" src={STEP_ICON_SRC[i]} alt="" aria-hidden="true" />
              <span className="sd-install-step-label">{label}</span>
              <span className="sd-install-step-icon">{STEP_ICON[stepStatuses[i]]}</span>
            </li>
          ))}
        </ol>

        <div className="sd-install-overall-progress">
          <div className="sd-install-overall-track">
            <div className="sd-install-overall-fill" style={{ width: `${overallProgress}%` }} />
          </div>
          <span>{overallProgress}%</span>
        </div>

        {/* 进度说明 */}
        {progressLabel ? (
          <div className="sd-install-progress-label">{progressLabel}</div>
        ) : null}
      </section>

      <div className="sd-install-workbench">
        <section className="sd-install-column sd-install-config-panel">
          <div className="sd-install-column-title">安装配置</div>

          {/* ── 已安装成功卡 ───────────────────────────────────────────────── */}
          {isInstalled ? (
            <div className="sd-install-complete-card">
              <span className="sd-install-complete-icon">✓</span>
              <div className="sd-install-complete-body">
                <div className="sd-install-complete-title">Stardew Valley 已安装</div>
                <div className="sd-install-complete-desc">服务器已就绪，可以前往「服务器控制」启动游戏。</div>
              </div>
              <button className="sd-btn-green" type="button" onClick={() => onNavigate('server')}>
                前往服务器控制
              </button>
              {isAdmin ? (
                <button
                  className="sd-btn-tan"
                  type="button"
                  onClick={() => { setShowForm(true); setInstallError('') }}
                >
                  重新安装 / 修复
                </button>
              ) : null}
            </div>
          ) : null}

          {/* ── 非 admin 提示 ──────────────────────────────────────────────── */}
          {!isAdmin ? (
            <div className="sd-install-info-bar">
              仅管理员可以启动安装。请联系管理员完成 Steam 认证和游戏安装。
            </div>
          ) : null}

          {/* ── QR 认证失败提示 ────────────────────────────────────────────── */}
          {isQrAuthError ? (
            <div className="sd-install-error-bar">
              二维码登录失败：steam-auth 容器无法连接 SteamClient。请点击下方"改用账号密码重试"，后续如需验证码会再提示。
            </div>
          ) : null}

          {/* ── 操作按钮区 ────────────────────────────────────────────────── */}
          {isAdmin && !isInstalling && !showForm ? (
            <div className="sd-install-actions">
              {!isInstalled || authFailed ? (
                <button
                  className="sd-btn-green"
                  type="button"
                  onClick={() => { setShowForm(true); setInstallError('') }}
                >
                  {isQrAuthError
                    ? '改用账号密码重试'
                    : authFailed
                      ? '重新安装（凭据错误）'
                      : canDirectRetry
                        ? '重试安装'
                        : '安装游戏'}
                </button>
              ) : null}
            </div>
          ) : null}

          {isAdmin && isInstalling && !showForm ? (
            <div className="sd-install-config-placeholder">
              安装配置已提交，当前任务正在执行。需要 Steam 交互时请在中间区域完成认证。
            </div>
          ) : null}

          {/* ── 安装配置表单 ───────────────────────────────────────────────── */}
          {showForm && isAdmin ? (
            <div className="sd-install-form-card">
              <div className="sd-install-form-title">
                {isQrAuthError || (authFailed && !isInstalled)
                  ? '重新输入 Steam 凭据'
                  : isInstalled
                    ? '重新安装 / 修复安装'
                    : canDirectRetry
                      ? '确认重试安装'
                      : '安装配置'}
              </div>
              <p className="sd-install-form-hint">
                {canDirectRetry && !isInstalled
                  ? '将使用已保存的 Steam 凭据重新安装，只需确认镜像版本。'
                  : '请输入 Steam 账号信息和 VNC 密码。密码不会出现在任何日志中。'}
              </p>

              <form onSubmit={(e) => void handleInstallSubmit(e)} autoComplete="off">
                {!optionsLoading && imageTagOptions.length > 0 ? (
                  <div className="sd-install-field">
                    <label className="sd-install-field-label">JunimoServer 镜像版本</label>
                    <select
                      className="sd-install-select"
                      value={imageTag}
                      onChange={(e) => setImageTag(e.target.value)}
                    >
                      {imageTagOptions.map((opt) => (
                        <option key={opt.tag} value={opt.tag}>
                          {opt.label}{opt.recommended ? ' ★' : ''}{opt.isLatest ? ' 已是最新版' : ''}
                        </option>
                      ))}
                    </select>
                    {selectedOption?.warning ? (
                      <p className="sd-install-version-warn">{selectedOption.warning}</p>
                    ) : null}
                  </div>
                ) : null}

                {!canDirectRetry ? (
                  <>
                    <div className="sd-install-field">
                      <label className="sd-install-field-label">Steam 用户名</label>
                      <input
                        className="sd-install-input"
                        type="text"
                        value={steamUsername}
                        autoComplete="steam-account"
                        required
                        onChange={(e) => setSteamUsername(e.target.value)}
                      />
                    </div>
                    <div className="sd-install-field">
                      <label className="sd-install-field-label">Steam 密码</label>
                      <div className="sd-install-pwd-row">
                        <input
                          className="sd-install-input"
                          type={showSteamPwd ? 'text' : 'password'}
                          value={steamPassword}
                          autoComplete="new-password"
                          required
                          onChange={(e) => setSteamPassword(e.target.value)}
                        />
                        <button
                          className="sd-btn-tan"
                          type="button"
                          onClick={() => setShowSteamPwd((v) => !v)}
                        >
                          {showSteamPwd ? '隐藏' : '显示'}
                        </button>
                      </div>
                    </div>
                    <div className="sd-install-field">
                      <label className="sd-install-field-label">VNC 密码</label>
                      <div className="sd-install-pwd-row">
                        <input
                          className="sd-install-input"
                          type={showVncPwd ? 'text' : 'password'}
                          value={vncPassword}
                          autoComplete="new-password"
                          required
                          onChange={(e) => setVncPassword(e.target.value)}
                        />
                        <button
                          className="sd-btn-tan"
                          type="button"
                          onClick={() => setShowVncPwd((v) => !v)}
                        >
                          {showVncPwd ? '隐藏' : '显示'}
                        </button>
                      </div>
                    </div>
                    <p className="sd-install-form-hint" style={{ marginTop: 2 }}>
                      密码不会打印到任何日志或浏览器控制台。
                    </p>
                  </>
                ) : null}

                {installError ? (
                  <div className="sd-install-error-bar" style={{ marginTop: 8 }}>{installError}</div>
                ) : null}

                <div className="sd-install-form-actions">
                  <button className="sd-btn-green" type="submit" disabled={installBusy}>
                    {installBusy ? '正在启动安装...' : canDirectRetry && !isInstalled ? '确认重试' : '确认安装'}
                  </button>
                  <button
                    className="sd-btn-tan"
                    type="button"
                    disabled={installBusy}
                    onClick={() => { setShowForm(false); setInstallError('') }}
                  >
                    取消
                  </button>
                </div>
              </form>
            </div>
          ) : null}
        </section>

        <section className="sd-install-column sd-install-auth-panel">
          <div className="sd-install-column-title sd-install-column-title-steam">Steam 认证</div>

          {/* 拉取镜像进度卡 */}
          {phase === 'pull_running' && isInstalling ? (
            <div className="sd-install-pull-card">
              <div className="sd-install-pull-header">
                <span className="sd-install-pull-icon">↓</span>
                <div>
                  <div className="sd-install-pull-title">正在下载 JunimoServer 镜像</div>
                  <div className="sd-install-pull-sub">{stateMessage || '正在准备拉取镜像，请稍候...'}</div>
                </div>
              </div>
              {pullProgress ? (
                <div className="sd-install-prog-wrap">
                  <div className="sd-install-prog-track">
                    <div
                      className={`sd-install-prog-fill${pullProgress.done === pullProgress.total ? ' done' : ''}`}
                      style={{ width: `${pullProgress.percent}%` }}
                    />
                  </div>
                  <span className="sd-install-prog-pct">{pullProgress.done}/{pullProgress.total} 个镜像</span>
                </div>
              ) : (
                <div className="sd-install-waiting">等待 Docker 开始下载...</div>
              )}
              <p className="sd-install-pull-hint">首次下载约需 10–30 分钟，取决于网络速度。</p>
            </div>
          ) : null}

          {/* 游戏/SDK 下载提示 */}
          {(phase === 'game_downloading' || phase === 'steam_sdk_downloading') && isInstalling ? (
            <div className="sd-install-download-card">
              <div className="sd-install-dl-title">
                {phase === 'steam_sdk_downloading'
                  ? '下载 Steam SDK 运行文件中...'
                  : '下载 Stardew Valley 游戏文件中...'}
              </div>
              <div className="sd-install-dl-hint">大文件下载中，请耐心等待（约 10–30 分钟）。下载完成后面板会自动继续。</div>
            </div>
          ) : null}

          {/* ── Steam 认证交互区 ───────────────────────────────────────────── */}
          {(needsAuthMethodChoice || needsGuardChoice || needsGuard || needsQrCode) && !isAdmin ? (
            <div className="sd-install-guard-section">
              <div className="sd-install-guard-block">
                <div className="sd-install-guard-desc">Steam 认证正在进行中，请等待管理员完成验证。</div>
              </div>
            </div>
          ) : (needsAuthMethodChoice || needsGuardChoice || needsGuard || needsQrCode) ? (
            <div className="sd-install-guard-section">
              {/* 选择登录方式 */}
              {needsAuthMethodChoice ? (
                <div className="sd-install-guard-block">
                  <div className="sd-install-guard-title">选择 Steam 登录方式</div>
                  <p className="sd-install-guard-desc">
                    请选择扫码登录（Steam 手机 App），或使用已填写的账号密码继续。
                    账号密码方式如触发二次验证，会再提示选择 Steam Guard 方式。
                  </p>
                  <div className="sd-install-guard-actions">
                    <button
                      className="sd-btn-green"
                      type="button"
                      disabled={guardBusy}
                      onClick={() => void handleAuthMethodSelect('2')}
                    >
                      {guardBusy ? '提交中...' : '扫码登录'}
                    </button>
                    <button
                      className="sd-btn-tan"
                      type="button"
                      disabled={guardBusy}
                      onClick={() => void handleAuthMethodSelect('1')}
                    >
                      {guardBusy ? '提交中...' : '账号密码 / 验证码登录'}
                    </button>
                  </div>
                  {guardError ? <div className="sd-install-guard-error">{guardError}</div> : null}
                </div>
              ) : null}

              {/* 选择 Guard 方式 */}
              {needsGuardChoice ? (
                <div className="sd-install-guard-block">
                  <div className="sd-install-guard-title">选择 Steam Guard 验证方式</div>
                  <p className="sd-install-guard-desc">Steam 要求二步验证，请选择与任务日志菜单一致的方式。</p>
                  <div className="sd-install-guard-actions">
                    <button
                      className="sd-btn-green"
                      type="button"
                      disabled={guardBusy}
                      onClick={() => void handleAuthMethodSelect('1')}
                    >
                      {guardBusy ? '提交中...' : '手机 App 批准'}
                    </button>
                    <button
                      className="sd-btn-tan"
                      type="button"
                      disabled={guardBusy}
                      onClick={() => void handleAuthMethodSelect('2')}
                    >
                      {guardBusy ? '提交中...' : '输入验证码'}
                    </button>
                  </div>
                  {guardError ? <div className="sd-install-guard-error">{guardError}</div> : null}
                </div>
              ) : null}

              {/* Guard 验证码输入 / 手机批准 */}
              {needsGuard ? (
                <div className="sd-install-guard-block">
                  <div className="sd-install-guard-title">Steam Guard 验证</div>
                  {phase === 'steam_guard_required' ? (
                    <>
                      <p className="sd-install-guard-desc">Steam 已发送验证码，请在下方输入：</p>
                      <form
                        className="sd-install-guard-form"
                        onSubmit={(e) => void handleGuardSubmit(e)}
                      >
                        <input
                          className="sd-install-input"
                          type="text"
                          placeholder="输入 Steam Guard 验证码"
                          value={guardInput}
                          onChange={(e) => setGuardInput(e.target.value)}
                          autoComplete="off"
                          required
                        />
                        <button className="sd-btn-green" type="submit" disabled={guardBusy}>
                          {guardBusy ? '提交中...' : '提交验证码'}
                        </button>
                      </form>
                      {guardError ? <div className="sd-install-guard-error">{guardError}</div> : null}
                    </>
                  ) : null}
                  {phase === 'steam_guard_mobile_required' ? (
                    <div className="sd-install-guard-mobile">
                      <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
                      <span>请打开 Steam 手机 App，批准此次登录请求后面板会自动继续。</span>
                    </div>
                  ) : null}
                </div>
              ) : null}

              {/* QR 扫码 */}
              {needsQrCode ? (
                <div className="sd-install-guard-block">
                  <div className="sd-install-guard-title">Steam 手机扫码</div>
                  <p className="sd-install-guard-desc">
                    请使用 Steam 手机 App 扫描日志中输出的二维码。如二维码还未出现，请稍等几秒。
                  </p>
                  <div className="sd-install-guard-actions">
                    <button
                      className="sd-btn-green"
                      type="button"
                      disabled={!qrText}
                      onClick={() => setShowQrModal(true)}
                    >
                      打开扫码窗口
                    </button>
                  </div>
                  {!qrText ? (
                    <p className="sd-install-guard-desc" style={{ marginTop: 4 }}>
                      正在等待容器输出二维码...
                    </p>
                  ) : null}
                </div>
              ) : null}
            </div>
          ) : (
            <div className="sd-install-auth-placeholder">
              <span className="sd-install-auth-orb" aria-hidden="true" />
              <p>
                {isInstalled
                  ? 'Steam 认证已完成。后续如需修复安装，可在左侧重新进入安装流程。'
                  : isInstalling
                    ? '安装流程运行中，认证交互会在需要时显示在这里。'
                    : '启动安装后，这里会显示扫码登录、Steam Guard 或验证码输入。'}
              </p>
            </div>
          )}

          {/* ── SSE 断线提示 ─────────────────────────────────────────────── */}
          {sseError ? (
            <div className="sd-install-sse-warn">{sseError}</div>
          ) : null}
        </section>

        <section className="sd-install-column sd-install-log-panel">
          {/* ── 安装日志预览 ───────────────────────────────────────────────── */}
          <div className="sd-install-log-section">
            <div className="sd-install-section-title">
              安装日志
              {isInstalling ? (
                <span className="sd-jobs-sse-dot" aria-label="实时接收中" />
              ) : null}
            </div>
            {!installJobId ? (
              <div className="sd-install-log-empty">等待安装任务启动...</div>
            ) : logs.length === 0 ? (
              <div className="sd-install-log-empty">等待日志输出...</div>
            ) : (
              <div className="sd-install-log-window">
                {visibleLogs.map((log) => (
                  <div
                    key={`${log.jobId ?? ''}-${log.sequence}`}
                    className={`sd-install-log-line sd-install-log-${log.level}`}
                  >
                    <span className="sd-install-log-seq">{log.sequence}</span>
                    <span className="sd-install-log-level">{log.level}</span>
                    <span className="sd-install-log-msg">{log.message}</span>
                  </div>
                ))}
                <div ref={logEndRef} />
              </div>
            )}
            {logs.length >= 50 ? (
              <p className="sd-install-log-hint">
                仅显示最近 50 条。查看完整日志请前往{' '}
                <button
                  className="sd-inline-nav"
                  type="button"
                  onClick={() => onNavigate('jobs')}
                >
                  任务与日志
                </button>
                。
              </p>
            ) : null}
          </div>
        </section>
      </div>

      {/* ── QR 弹窗 ──────────────────────────────────────────────────────── */}
      {showQrModal ? (
        <div className="sd-install-qr-overlay" role="dialog" aria-modal="true">
          <div className="sd-install-qr-card">
            <div className="sd-install-qr-header">
              <span className="sd-install-qr-title">Steam 手机扫码</span>
              <button className="sd-btn-tan" type="button" onClick={() => setShowQrModal(false)}>
                关闭
              </button>
            </div>
            {qrText ? (
              <pre className="sd-install-qr-pre" style={{ fontSize: `${qrCodeFontSize(qrText)}px` }}>
                {qrText}
              </pre>
            ) : (
              <p className="sd-install-guard-desc">正在等待容器输出二维码...</p>
            )}
          </div>
        </div>
      ) : null}

    </div>
  )
}
