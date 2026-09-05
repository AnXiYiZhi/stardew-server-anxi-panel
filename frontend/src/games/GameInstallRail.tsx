import { useCallback, useEffect, useId, useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import {
  ApiError,
  getInstanceState,
  getJob,
  getJobs,
  getLatestJobLogs,
  installInstance,
  steamAuthLogin,
  submitSteamGuardInput,
} from '../api'
import { errorMessage, isTerminalJobStatus } from '../core/helpers'
import type { CurrentUser, InstanceState, Job, JobLog } from '../types'
import {
  GAME_INSTALL_STEPS,
  gameInstallProgressPresentation,
  gameInstallStepProgressLabel,
  type GameInstallProgressPresentation,
} from './stardew/install-progress-presentation'
import { classifyInstallationState } from './stardew/installation-state'
import { canonicalInstallPageJobs, reconcileJobSnapshots } from './stardew/install-state'
import { routeToPath } from './stardew/stardew-routes'

type Navigate = (path: string, replace?: boolean) => void

type GameInstallRailProps = {
  user: CurrentUser
  instanceId: string
  sourceState: InstanceState | null
  requestedInstallJobId?: string
  onNavigate: Navigate
  onCatalogRefresh: () => void
  onPresentationChange: (presentation: GameInstallProgressPresentation | null) => void
}

function inProgressJobId(error: ApiError): string {
  if (error.code !== 'install_in_progress' || typeof error.details !== 'object' || error.details === null) return ''
  if (!('jobId' in error.details)) return ''
  return String((error.details as { jobId?: unknown }).jobId ?? '')
}

function PasswordVisibilityIcon({ visible }: { visible: boolean }) {
  return (
    <svg className="game-install-password-eye" viewBox="0 0 24 24" aria-hidden="true">
      <path d="M2.7 12s3.4-5.3 9.3-5.3 9.3 5.3 9.3 5.3-3.4 5.3-9.3 5.3S2.7 12 2.7 12Z" />
      <circle cx="12" cy="12" r="2.5" />
      {visible ? <path className="game-install-password-eye-slash" d="m4.2 4.2 15.6 15.6" /> : null}
    </svg>
  )
}

export function GameInstallRail({
  user,
  instanceId,
  sourceState,
  requestedInstallJobId,
  onNavigate,
  onCatalogRefresh,
  onPresentationChange,
}: GameInstallRailProps) {
  const firstInputRef = useRef<HTMLInputElement | null>(null)
  const steamPasswordId = useId()
  const vncPasswordId = useId()
  const [liveState, setLiveState] = useState<InstanceState | null>(sourceState)
  const [jobId, setJobId] = useState<string | null>(requestedInstallJobId ?? null)
  const [job, setJob] = useState<Job | null>(null)
  const [logs, setLogs] = useState<JobLog[]>([])
  const [steamUsername, setSteamUsername] = useState('')
  const [steamPassword, setSteamPassword] = useState('')
  const [vncPassword, setVncPassword] = useState('')
  const [steamPasswordVisible, setSteamPasswordVisible] = useState(false)
  const [vncPasswordVisible, setVncPasswordVisible] = useState(false)
  const [guardInput, setGuardInput] = useState('')
  const [editingCredentials, setEditingCredentials] = useState(false)
  const [busy, setBusy] = useState(false)
  const [loading, setLoading] = useState(true)
  const [message, setMessage] = useState<string | null>(null)
  const [pollError, setPollError] = useState<string | null>(null)
  const [guardSubmitted, setGuardSubmitted] = useState(false)
  const automaticChoiceRef = useRef('')
  const terminalRefreshRef = useRef('')
  const targetId = instanceId
  const authOnly = job?.type === 'stardew_steam_auth'
  const requestGeneration = useRef(0)
  const pollingRequest = useRef<symbol | null>(null)

  useEffect(() => {
    if (requestedInstallJobId && requestedInstallJobId !== jobId) {
      setEditingCredentials(false)
      setJobId(requestedInstallJobId)
      setJob(null)
      setLogs([])
    }
  }, [jobId, requestedInstallJobId])

  useEffect(() => {
    let canceled = false
    setLoading(true)
    setMessage(null)
    void Promise.all([getInstanceState(targetId), getJobs()])
      .then(([stateResponse, jobsResponse]) => {
        if (canceled) return
        setLiveState(stateResponse)
        const selected = canonicalInstallPageJobs(
          jobsResponse.jobs.filter((candidate) => candidate.targetType === 'instance' && candidate.targetId === targetId),
          null,
          requestedInstallJobId ?? null,
        )
        const candidate = requestedInstallJobId ? selected.selected : selected.active
        if (candidate) {
          setJob(current => current
            ? current.id === candidate.id ? reconcileJobSnapshots(current, candidate) : current
            : candidate)
          setJobId(current => current ?? candidate.id)
        }
      })
      .catch((reason) => {
        if (!canceled) setMessage(errorMessage(reason))
      })
      .finally(() => {
        if (!canceled) setLoading(false)
      })
    return () => {
      canceled = true
    }
  }, [requestedInstallJobId, targetId])

  useEffect(() => {
    let canceled = false
    void getInstanceState(targetId)
      .then((nextState) => {
        if (!canceled) setLiveState(nextState)
      })
      .catch(() => {
        if (!canceled) setLiveState(sourceState)
      })
    return () => {
      canceled = true
    }
  }, [sourceState, targetId])

  const refreshTask = useCallback(async () => {
    if (!jobId || pollingRequest.current) return
    const request = Symbol()
    pollingRequest.current = request
    const generation = ++requestGeneration.current
    try {
      const [jobResponse, logsResponse, stateResponse] = await Promise.all([
        getJob(jobId),
        getLatestJobLogs(jobId, 1000),
        getInstanceState(targetId),
      ])
      if (generation !== requestGeneration.current) return
      if (jobResponse.job.targetType !== 'instance' || jobResponse.job.targetId !== targetId
        || !['stardew_install', 'stardew_steam_auth'].includes(jobResponse.job.type)) {
        throw new Error('该任务不属于当前世界，请返回世界列表重新进入。')
      }
      setJob(current => current?.id === jobResponse.job.id ? reconcileJobSnapshots(current, jobResponse.job) : jobResponse.job)
      setLogs(logsResponse.logs)
      setLiveState(stateResponse)
      setPollError(null)
      if (isTerminalJobStatus(jobResponse.job.status) && terminalRefreshRef.current !== jobId) {
        terminalRefreshRef.current = jobId
        onCatalogRefresh()
      }
    } catch (reason) {
      if (generation === requestGeneration.current) setPollError(errorMessage(reason))
    } finally {
      if (pollingRequest.current === request) pollingRequest.current = null
    }
  }, [jobId, onCatalogRefresh, targetId])

  useEffect(() => {
    if (!jobId) return
    if (terminalRefreshRef.current === jobId) return
    void refreshTask()
    const interval = window.setInterval(() => void refreshTask(), 1_500)
    return () => {
      window.clearInterval(interval)
      requestGeneration.current += 1
      pollingRequest.current = null
    }
  }, [jobId, job?.status, refreshTask])

  const sourceInstallation = useMemo(
    () => classifyInstallationState(sourceState),
    [sourceState],
  )
  const requiredFilesMissing = jobId === null && (
    sourceInstallation.reason === 'required_files_missing'
    || sourceState?.driverPhase === 'install_verification_failed'
    || liveState?.installationDiagnostic?.requiredFiles === 'missing'
  )
  const presentation = useMemo(
    () => gameInstallProgressPresentation(
      liveState ?? sourceState,
      job,
      logs,
      requiredFilesMissing,
    ),
    [job, liveState, logs, requiredFilesMissing, sourceState],
  )
  const showTask = !editingCredentials
    && (jobId !== null || presentation.mode !== 'idle')

  // Retain the current form in component memory for an editable retry.
  // Completion clears it; leaving this mounted installation view discards it.
  useEffect(() => {
    if (presentation.mode !== 'complete') return
    setSteamUsername('')
    setSteamPassword('')
    setVncPassword('')
    setGuardInput('')
  }, [presentation.mode])

  useEffect(() => {
    onPresentationChange(showTask ? presentation : null)
    return () => onPresentationChange(null)
  }, [onPresentationChange, presentation, showTask])

  useLayoutEffect(() => {
    if (!showTask) firstInputRef.current?.focus({ preventScroll: true })
  }, [showTask])

  async function submitInstall(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (user.role !== 'admin' || busy) return
    setBusy(true)
    setMessage(null)
    try {
      const response = await installInstance({ steamUsername, steamPassword, vncPassword }, targetId)
      setEditingCredentials(false)
      setSteamPasswordVisible(false)
      setVncPasswordVisible(false)
      setJobId(response.jobId)
      setJob(null)
      setLogs([])
      onNavigate(routeToPath('install', { installJobId: response.jobId }, targetId), true)
      onCatalogRefresh()
    } catch (reason) {
      if (reason instanceof ApiError) {
        const existingJobId = inProgressJobId(reason)
        if (existingJobId) {
          const existingJob = await getJob(existingJobId).catch(() => null)
          if (!existingJob) {
            setMessage('无法读取正在进行的安装任务，请稍后重试。')
            return
          }
          if (existingJob.job.targetType !== 'instance' || existingJob.job.targetId !== targetId) {
            setMessage('另一个世界正在安装，请等待该任务结束后重试。')
            return
          }
          setJobId(existingJobId)
          setJob(null)
          setLogs([])
          onNavigate(routeToPath('install', { installJobId: existingJobId }, targetId), true)
          return
        }
      }
      setMessage(errorMessage(reason))
    } finally {
      setBusy(false)
    }
  }

  async function submitGuard(value: string) {
    if (user.role !== 'admin' || !jobId || busy || !value.trim()) return
    setBusy(true)
    setMessage(null)
    setGuardSubmitted(false)
    try {
      await submitSteamGuardInput(jobId, value.trim(), targetId)
      setGuardInput('')
      setGuardSubmitted(true)
      await refreshTask()
    } catch (reason) {
      setMessage(errorMessage(reason))
    } finally {
      setBusy(false)
    }
  }

  async function retryWithCredentials() {
    if (user.role !== 'admin' || busy) return
    if (authOnly) {
      setBusy(true)
      setMessage(null)
      try {
        const response = await steamAuthLogin(targetId)
        setJobId(response.jobId)
        setJob(null)
        setLogs([])
        onNavigate(routeToPath('install', { installJobId: response.jobId }, targetId), true)
      } catch (reason) { setMessage(errorMessage(reason)) }
      finally { setBusy(false) }
      return
    }
    setEditingCredentials(true)
    setGuardInput('')
    setSteamPasswordVisible(false)
    setVncPasswordVisible(false)
    setJobId(null)
    setJob(null)
    setLogs([])
    setMessage(null)
    onNavigate(routeToPath('install', undefined, targetId), true)
  }

  const phase = (liveState ?? sourceState)?.driverPhase ?? ''
  const taskActive = presentation.mode === 'active'
  const needsGuardCode = taskActive && (phase === 'steam_guard_required' || phase === 'steamcmd_guard_required')
  const waitsForMobile = taskActive && (phase === 'steam_guard_mobile_required' || phase === 'steamcmd_guard_mobile_required')
  const automaticChoice = phase === 'auth_method_required'
    || phase === 'steam_guard_choice_required'
    || phase === 'steamcmd_guard_choice_required'

  useEffect(() => {
    if (user.role !== 'admin' || !jobId || !taskActive || !automaticChoice) return
    const key = `${jobId}:${phase}`
    if (automaticChoiceRef.current === key) return
    automaticChoiceRef.current = key
    setMessage(null)
    void submitSteamGuardInput(jobId, '1', targetId)
      .then(() => refreshTask())
      .catch((reason) => {
        automaticChoiceRef.current = ''
        setMessage(errorMessage(reason))
      })
  }, [automaticChoice, jobId, phase, refreshTask, targetId, taskActive, user.role])
  const progressLabel = gameInstallStepProgressLabel(presentation)
  const indeterminate = taskActive && presentation.percent === null

  if (!showTask) {
    return (
      <form className="game-install-inline game-install-inline--form" onSubmit={submitInstall} aria-busy={busy}>
        <strong>安装星露谷物语</strong>
        <label className="game-install-field">
          <span>Steam 账号</span>
          <input
            ref={firstInputRef}
            value={steamUsername}
            onChange={(event) => setSteamUsername(event.target.value)}
            autoComplete="username"
            spellCheck={false}
            required
            disabled={busy || loading}
          />
        </label>
        <div className="game-install-field">
          <label htmlFor={steamPasswordId}>Steam 密码</label>
          <span className="game-install-secret-control">
            <input
              id={steamPasswordId}
              type={steamPasswordVisible ? 'text' : 'password'}
              value={steamPassword}
              onChange={(event) => setSteamPassword(event.target.value)}
              autoComplete="new-password"
              required
              disabled={busy || loading}
            />
            <button
              className="game-install-password-toggle"
              type="button"
              aria-label={`${steamPasswordVisible ? '隐藏' : '显示'} Steam 密码`}
              aria-pressed={steamPasswordVisible}
              onClick={() => setSteamPasswordVisible((visible) => !visible)}
              disabled={busy || loading}
            >
              <PasswordVisibilityIcon visible={steamPasswordVisible} />
            </button>
          </span>
        </div>
        <div className="game-install-field">
          <label htmlFor={vncPasswordId}>VNC 密码</label>
          <span className="game-install-secret-control">
            <input
              id={vncPasswordId}
              type={vncPasswordVisible ? 'text' : 'password'}
              value={vncPassword}
              onChange={(event) => setVncPassword(event.target.value)}
              autoComplete="new-password"
              required
              disabled={busy || loading}
            />
            <button
              className="game-install-password-toggle"
              type="button"
              aria-label={`${vncPasswordVisible ? '隐藏' : '显示'} VNC 密码`}
              aria-pressed={vncPasswordVisible}
              onClick={() => setVncPasswordVisible((visible) => !visible)}
              disabled={busy || loading}
            >
              <PasswordVisibilityIcon visible={vncPasswordVisible} />
            </button>
          </span>
        </div>
        {user.role !== 'admin' ? (
          <span className="game-install-message" role="status">仅管理员可以安装游戏。</span>
        ) : message ? (
          <span className="game-install-message is-error" role="alert">{message}</span>
        ) : (
          <span className="game-install-message" role="status">凭据只用于现有安全安装流程，不会写入日志。</span>
        )}
        <span className="game-install-actions">
          <button type="button" onClick={() => onNavigate('/games')} disabled={busy}>取消</button>
          <button
            type="submit"
            disabled={user.role !== 'admin' || busy || loading || !steamUsername.trim() || !steamPassword || !vncPassword}
          >
            {busy ? '正在启动…' : '开始安装'}
          </button>
        </span>
      </form>
    )
  }

  return (
    <section
      className={`game-install-inline game-install-inline--progress is-${presentation.mode}`}
      aria-live="polite"
    >
      <header className="game-install-progress-head">
        <span>当前步骤</span>
        <strong>{presentation.title}</strong>
        <b>{progressLabel}</b>
      </header>
      <div
        className={`game-install-progress-bar${indeterminate ? ' is-indeterminate' : ''}`}
        role="progressbar"
        aria-label="当前安装步骤进度"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={presentation.percent ?? undefined}
        aria-valuetext={progressLabel}
      >
        <span style={{ width: `${presentation.percent ?? (indeterminate ? 26 : 0)}%` }} />
      </div>
      {!authOnly ? <ol className="game-install-step-list">
        {GAME_INSTALL_STEPS.map((label, index) => (
          <li key={label} className={`is-${presentation.steps[index]}`}>
            <span aria-hidden="true" />
            <strong>{label}</strong>
          </li>
        ))}
      </ol> : null}
      <p className="game-install-progress-detail">{presentation.detail}</p>

      {needsGuardCode ? (
        <form className="game-install-guard" onSubmit={(event) => { event.preventDefault(); void submitGuard(guardInput) }}>
          <label>
            <span>Steam Guard 验证码</span>
            <input value={guardInput} onChange={(event) => setGuardInput(event.target.value)} autoComplete="one-time-code" required />
          </label>
          <button type="submit" disabled={user.role !== 'admin' || busy || !guardInput.trim()}>{busy ? '正在提交…' : '提交验证'}</button>
        </form>
      ) : waitsForMobile ? (
        <span className="game-install-mobile-wait" role="status">请在 Steam 手机 App 中批准登录。</span>
      ) : taskActive && phase === 'steam_qr_required' ? (
        <span className="game-install-mobile-wait" role="status">当前旧登录任务无法继续，请取消后使用 Steam 账号密码重新开始。</span>
      ) : null}

      {needsGuardCode && guardSubmitted ? <span className="game-install-message" role="status">验证码已提交，请等待 Steam 验证；若提示无效，请输入最新验证码重新提交。</span> : null}
      {message || pollError ? <span className="game-install-message is-error" role="alert">{message || pollError}</span> : null}

      {presentation.mode === 'failed' ? (
        <button type="button" className="game-install-retry" disabled={busy || user.role !== 'admin'} onClick={() => void retryWithCredentials()}>{authOnly ? '重新授权' : '重新填写并重试'}</button>
      ) : presentation.mode === 'complete' ? (
        <button type="button" className="game-install-retry" onClick={() => onNavigate('/games/stardew')}>选择世界</button>
      ) : null}
    </section>
  )
}
