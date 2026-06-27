import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import {
  ApiError,
  clearJobs,
  createJobEventSource,
  getComposePs,
  getDockerStatus,
  getInstallOptions,
  getJob,
  getJobLogs,
  getJobs,
  getStardewState,
  installInstance,
  request,
  startFailingTestJob,
  startTestJob,
  submitSteamGuardInput,
} from './api'
import type {
  ComposePsResponse,
  CurrentUser,
  DockerStatusResponse,
  ImageTagOption,
  InstanceState,
  Job,
  JobLog,
  OKResponse,
  PanelUser,
  PanelUserResponse,
  SetupStatus,
  UserResponse,
  UsersResponse,
} from './types'

// Core components
import { SetupPanel, emptySetupForm } from './core/SetupPanel'
import type { SetupFormState } from './core/SetupPanel'
import { LoginPanel, emptyLoginForm } from './core/LoginPanel'
import type { LoginFormState } from './core/LoginPanel'
import { PasswordInput } from './core/PasswordInput'
import { errorMessage, isTerminalJobStatus, appendUniqueLog } from './core/helpers'

// Stardew game components
import { InstanceStateCard } from './games/stardew/InstanceStateCard'
import { InstallSection, emptyInstallForm } from './games/stardew/InstallSection'
import type { InstallFormState } from './games/stardew/InstallSection'
import { LifecycleSection } from './games/stardew/LifecycleSection'
import { SavesSection } from './games/stardew/SavesSection'
import { JobsSection } from './games/stardew/JobsSection'
import { DockerSection } from './games/stardew/DockerSection'
import {
  extractSteamDownloadProgress,
  hasSteamSdkDownloadStarted,
  hasSteamSdkDownloadCompleted,
  calcSteamDownloadTaskProgress,
  extractRecentSteamQrText,
  installFailureDisplayMessage,
} from './games/stardew/install-helpers'

type View = 'booting' | 'setup' | 'login' | 'dashboard'

type NewUserFormState = {
  username: string
  password: string
  role: 'admin' | 'user'
}

const emptyNewUserForm: NewUserFormState = { username: '', password: '', role: 'user' }

function App() {
  const [view, setView] = useState<View>('booting')
  const [currentUser, setCurrentUser] = useState<CurrentUser | null>(null)
  const [setupForm, setSetupForm] = useState<SetupFormState>({ ...emptySetupForm })
  const [loginForm, setLoginForm] = useState<LoginFormState>({ ...emptyLoginForm })
  const [newUserForm, setNewUserForm] = useState<NewUserFormState>({ ...emptyNewUserForm })
  const [users, setUsers] = useState<PanelUser[]>([])
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    boot()
  }, [])

  async function boot() {
    setMessage('')
    try {
      const status = await request<SetupStatus>('/api/setup/status')
      if (!status.initialized) {
        setView('setup')
        return
      }
      try {
        const me = await request<UserResponse>('/api/auth/me')
        setCurrentUser(me.user)
        setView('dashboard')
        if (me.user.role === 'admin') {
          void loadUsers()
        }
      } catch (error) {
        if (error instanceof ApiError && error.status === 401) {
          setView('login')
          return
        }
        throw error
      }
    } catch (error) {
      setMessage(errorMessage(error))
      setView('login')
    }
  }

  async function loadUsers() {
    try {
      const response = await request<UsersResponse>('/api/users')
      setUsers(response.users)
    } catch (error) {
      setMessage(errorMessage(error))
    }
  }

  async function submitSetup(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setBusy(true)
    setMessage('')
    try {
      const response = await request<UserResponse>('/api/setup/admin', {
        method: 'POST',
        body: setupForm,
      })
      setCurrentUser(response.user)
      setSetupForm({ ...emptySetupForm })
      setView('dashboard')
      void loadUsers()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function submitLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setBusy(true)
    setMessage('')
    try {
      const response = await request<UserResponse>('/api/auth/login', {
        method: 'POST',
        body: loginForm,
      })
      setCurrentUser(response.user)
      setLoginForm({ ...emptyLoginForm })
      setView('dashboard')
      if (response.user.role === 'admin') {
        void loadUsers()
      } else {
        setUsers([])
      }
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function logout() {
    setBusy(true)
    setMessage('')
    try {
      await request<OKResponse>('/api/auth/logout', { method: 'POST' })
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setCurrentUser(null)
      setUsers([])
      setLoginForm({ ...emptyLoginForm })
      setView('login')
      setBusy(false)
    }
  }

  async function createUser(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setBusy(true)
    setMessage('')
    try {
      await request<PanelUserResponse>('/api/users', { method: 'POST', body: newUserForm })
      setNewUserForm({ ...emptyNewUserForm })
      await loadUsers()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function updateRole(user: PanelUser, role: 'admin' | 'user') {
    setBusy(true)
    setMessage('')
    try {
      await request<PanelUserResponse>(`/api/users/${user.id}`, { method: 'PATCH', body: { role } })
      await loadUsers()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function setUserActive(user: PanelUser, isActive: boolean) {
    setBusy(true)
    setMessage('')
    try {
      await request<PanelUserResponse>(`/api/users/${user.id}`, { method: 'PATCH', body: { isActive } })
      await loadUsers()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function deleteUser(user: PanelUser) {
    if (!window.confirm(`确认永久删除用户"${user.username}"？此操作不可恢复。`)) return
    setBusy(true)
    setMessage('')
    try {
      await request<OKResponse>(`/api/users/${user.id}?hard=true`, { method: 'DELETE' })
      await loadUsers()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="shell">
      <section className="panel-card">
        <p className="eyebrow">Stardew Valley 管理面板</p>
        <h1>Stardew Anxi Panel</h1>
        {message ? <div className="error-banner">{message}</div> : null}
        {view === 'booting' ? <p className="summary">正在读取面板状态……</p> : null}
        {view === 'setup' ? (
          <SetupPanel form={setupForm} busy={busy} onChange={setSetupForm} onSubmit={submitSetup} />
        ) : null}
        {view === 'login' ? (
          <LoginPanel form={loginForm} busy={busy} onChange={setLoginForm} onSubmit={submitLogin} />
        ) : null}
        {view === 'dashboard' && currentUser ? (
          <Dashboard
            user={currentUser}
            users={users}
            busy={busy}
            newUserForm={newUserForm}
            onNewUserChange={setNewUserForm}
            onCreateUser={createUser}
            onUpdateRole={updateRole}
            onSetUserActive={setUserActive}
            onDeleteUser={deleteUser}
            onRefreshUsers={loadUsers}
            onLogout={logout}
          />
        ) : null}
      </section>
    </main>
  )
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

function Dashboard({
  user,
  users,
  busy,
  newUserForm,
  onNewUserChange,
  onCreateUser,
  onUpdateRole,
  onSetUserActive,
  onDeleteUser,
  onRefreshUsers,
  onLogout,
}: {
  user: CurrentUser
  users: PanelUser[]
  busy: boolean
  newUserForm: NewUserFormState
  onNewUserChange: (f: NewUserFormState) => void
  onCreateUser: (e: FormEvent<HTMLFormElement>) => void
  onUpdateRole: (u: PanelUser, r: 'admin' | 'user') => void
  onSetUserActive: (u: PanelUser, a: boolean) => void
  onDeleteUser: (u: PanelUser) => void
  onRefreshUsers: () => void
  onLogout: () => void
}) {
  const [showNewPassword, setShowNewPassword] = useState(false)
  const [dockerStatus, setDockerStatus] = useState<DockerStatusResponse | null>(null)
  const [composePs, setComposePs] = useState<ComposePsResponse | null>(null)
  const [dockerCheckedAt, setDockerCheckedAt] = useState('')
  const [dockerMessage, setDockerMessage] = useState('')
  const [dockerBusy, setDockerBusy] = useState(false)
  const [instanceState, setInstanceState] = useState<InstanceState | null>(null)
  const [jobs, setJobs] = useState<Job[]>([])
  const [selectedJob, setSelectedJob] = useState<Job | null>(null)
  const [jobLogs, setJobLogs] = useState<JobLog[]>([])
  const [jobMessage, setJobMessage] = useState('')
  const [jobBusy, setJobBusy] = useState(false)
  const [streamFailed, setStreamFailed] = useState(false)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [savesRefreshKey, setSavesRefreshKey] = useState(0)

  // Install flow state
  const [showInstallModal, setShowInstallModal] = useState(false)
  const [installForm, setInstallForm] = useState<InstallFormState>({ ...emptyInstallForm })
  const [installBusy, setInstallBusy] = useState(false)
  const [installMessage, setInstallMessage] = useState('')
  const [activeInstallJobId, setActiveInstallJobId] = useState<string | null>(null)
  const [guardInput, setGuardInput] = useState('')
  const [guardBusy, setGuardBusy] = useState(false)
  const [guardMessage, setGuardMessage] = useState('')
  const [imageTagOptions, setImageTagOptions] = useState<ImageTagOption[]>([])
  const [lastTriedCredentials, setLastTriedCredentials] = useState<{
    steamUsername: string
    steamPassword: string
    vncPassword: string
  } | null>(null)

  // Poll instance state when install job is active
  const statePollerRef = useRef<number | null>(null)

  useEffect(() => {
    void refreshState()
    void refreshJobs()
    if (user.role === 'admin') void loadInstallOptions()
  }, [])

  // Start/stop state polling during install
  useEffect(() => {
    if (activeInstallJobId || instanceState?.state === 'steam_auth_running') {
      statePollerRef.current = window.setInterval(() => {
        void refreshState()
      }, 2500)
    }
    return () => {
      if (statePollerRef.current !== null) {
        window.clearInterval(statePollerRef.current)
        statePollerRef.current = null
      }
    }
  }, [activeInstallJobId, instanceState?.state])

  // SSE for selected job
  useEffect(() => {
    if (!selectedJob) {
      setJobLogs([])
      return
    }
    const activeJob = selectedJob
    let closed = false
    setStreamFailed(false)

    async function loadSelectedJob() {
      try {
        const [jobResponse, logsResponse] = await Promise.all([
          getJob(activeJob.id),
          getJobLogs(activeJob.id),
        ])
        if (!closed) {
          setSelectedJob(jobResponse.job)
          setJobLogs(logsResponse.logs)
        }
      } catch (error) {
        if (!closed) setJobMessage(errorMessage(error))
      }
    }

    void loadSelectedJob()
    const source = createJobEventSource(activeJob.id)
    source.addEventListener('log', (event) => {
      const nextLog = JSON.parse((event as MessageEvent).data) as JobLog
      setJobLogs((current) => appendUniqueLog(current, nextLog))
    })
    source.addEventListener('finished', (event) => {
      const nextJob = JSON.parse((event as MessageEvent).data) as Job
      setSelectedJob(nextJob)
      void refreshJobs()
      void refreshState()
      // Trigger saves list refresh after any job completes (create/upload/select-and-start).
      setSavesRefreshKey((k) => k + 1)
      setActiveInstallJobId((current) => current === nextJob.id ? null : current)
      source.close()
    })
    source.onerror = () => {
      source.close()
      if (!closed && activeJob.status === 'running') setStreamFailed(true)
    }
    return () => {
      closed = true
      source.close()
    }
  }, [selectedJob?.id])

  useEffect(() => {
    if (!activeInstallJobId) return
    const activeJob = jobs.find((job) => job.id === activeInstallJobId)
    if (!activeJob || !isTerminalJobStatus(activeJob.status)) return
    setActiveInstallJobId(null)
    void refreshState()
  }, [activeInstallJobId, jobs])

  // Poll fallback when SSE fails
  useEffect(() => {
    if (!selectedJob || !streamFailed || isTerminalJobStatus(selectedJob.status)) return
    const timer = window.setInterval(async () => {
      const lastSeq = jobLogs.at(-1)?.sequence ?? 0
      try {
        const [jobResponse, logsResponse] = await Promise.all([
          getJob(selectedJob.id),
          getJobLogs(selectedJob.id, lastSeq),
        ])
        setSelectedJob(jobResponse.job)
        setJobLogs((current) => logsResponse.logs.reduce(appendUniqueLog, current))
        if (isTerminalJobStatus(jobResponse.job.status)) {
          await refreshJobs()
          await refreshState()
          if (jobResponse.job.id === activeInstallJobId) setActiveInstallJobId(null)
        }
      } catch (error) {
        setJobMessage(errorMessage(error))
      }
    }, 2500)
    return () => window.clearInterval(timer)
  }, [selectedJob, streamFailed, jobLogs])

  async function refreshState() {
    try {
      setInstanceState(await getStardewState())
    } catch (error) {
      setJobMessage(errorMessage(error))
    }
  }

  async function refreshJobs() {
    try {
      const response = await getJobs()
      setJobs(response.jobs)
    } catch (error) {
      setJobMessage(errorMessage(error))
    }
  }

  async function runTestJob(shouldFail: boolean) {
    setJobBusy(true)
    setJobMessage('')
    try {
      const response = shouldFail ? await startFailingTestJob() : await startTestJob()
      setSelectedJob(response.job)
      await refreshJobs()
    } catch (error) {
      setJobMessage(errorMessage(error))
    } finally {
      setJobBusy(false)
    }
  }

  async function clearJobCenter() {
    if (!window.confirm('确定清空任务中心吗？任务记录和日志会被删除。')) return
    setJobBusy(true)
    setJobMessage('')
    try {
      const response = await clearJobs()
      setJobs([])
      setSelectedJob(null)
      setJobLogs([])
      setJobMessage(`已清空 ${response.deleted} 个任务。`)
    } catch (error) {
      setJobMessage(errorMessage(error))
    } finally {
      setJobBusy(false)
    }
  }

  async function checkDocker() {
    setDockerBusy(true)
    setDockerMessage('')
    try {
      setDockerStatus(await getDockerStatus())
      setDockerCheckedAt(new Date().toLocaleString())
    } catch (error) {
      setDockerMessage(errorMessage(error))
    } finally {
      setDockerBusy(false)
    }
  }

  async function loadComposePs() {
    setDockerBusy(true)
    setDockerMessage('')
    try {
      setComposePs(await getComposePs())
    } catch (error) {
      setDockerMessage(errorMessage(error))
    } finally {
      setDockerBusy(false)
    }
  }

  // ── Install flow ─────────────────────────────────────────────────────────

  async function loadInstallOptions() {
    try {
      const res = await getInstallOptions()
      setImageTagOptions(res.imageTagOptions)
      const recommended = res.imageTagOptions.find((o) => o.recommended)
      if (recommended) {
        setInstallForm((prev) => (prev.imageTag === '' ? { ...prev, imageTag: recommended.tag } : prev))
      }
    } catch {
      // Non-fatal
    }
  }

  async function handleInstallSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setInstallBusy(true)
    setInstallMessage('')
    try {
      let response
      if (canDirectRetry) {
        response = await installInstance({ reuseCredentials: true, imageTag: installForm.imageTag })
      } else {
        setLastTriedCredentials({
          steamUsername: installForm.steamUsername,
          steamPassword: installForm.steamPassword,
          vncPassword: installForm.vncPassword,
        })
        response = await installInstance({
          steamUsername: installForm.steamUsername,
          steamPassword: installForm.steamPassword,
          vncPassword: installForm.vncPassword,
          imageTag: installForm.imageTag,
        })
      }
      setActiveInstallJobId(response.jobId)
      setShowInstallModal(false)
      const jobResponse = await getJob(response.jobId)
      setSelectedJob(jobResponse.job)
      await refreshJobs()
      setInstallForm({ ...emptyInstallForm })
    } catch (error) {
      setInstallMessage(errorMessage(error))
    } finally {
      setInstallBusy(false)
    }
  }

  async function handleGuardSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const jobId = currentInstallJobID()
    if (!jobId) {
      setGuardMessage('找不到进行中的安装任务 ID，请刷新页面后重试。')
      return
    }
    if (!guardInput) return
    setGuardBusy(true)
    setGuardMessage('')
    try {
      await submitSteamGuardInput(jobId, guardInput)
      setGuardInput('')
      setGuardMessage('验证码已提交，请等待 Steam 响应。')
    } catch (error) {
      setGuardMessage(errorMessage(error))
    } finally {
      setGuardBusy(false)
    }
  }

  async function handleAuthMethodSelect(choice: string) {
    const jobId = currentInstallJobID()
    if (!jobId) {
      setGuardMessage('找不到进行中的安装任务 ID，请刷新页面后重试。')
      return
    }
    setGuardBusy(true)
    setGuardMessage('')
    try {
      await submitSteamGuardInput(jobId, choice)
      if (phase === 'auth_method_required') {
        if (choice === '1') setGuardMessage('已选择账号密码登录，正在等待 Steam 响应。')
        if (choice === '2') setGuardMessage('已选择二维码登录，正在等待二维码输出。')
      } else {
        if (choice === '1') setGuardMessage('已选择手机 App 批准，请打开 Steam App 确认登录。')
        if (choice === '2') setGuardMessage('已选择验证码方式，正在等待验证码输入框...')
      }
    } catch (error) {
      setGuardMessage(errorMessage(error))
    } finally {
      setGuardBusy(false)
    }
  }

  const rawState = instanceState?.state ?? ''
  const rawPhase = instanceState?.driverPhase ?? ''
  const selectedInstallJob = selectedJob?.type === 'stardew_install' ? selectedJob : null
  const installJobs = selectedInstallJob
    ? [
      selectedInstallJob,
      ...jobs.filter((job) => job.id !== selectedInstallJob.id),
    ]
    : jobs
  const runningInstallJobId = installJobs.find((job) => job.type === 'stardew_install' && !isTerminalJobStatus(job.status))?.id ?? null
  const latestInstallJob = installJobs.find((job) => job.type === 'stardew_install')
  const activeInstallJob = activeInstallJobId ? installJobs.find((job) => job.id === activeInstallJobId) : null
  const activeInstallJobIsRunning = activeInstallJobId !== null && (!activeInstallJob || !isTerminalJobStatus(activeInstallJob.status))
  const hasFailedInstallJob = !runningInstallJobId && latestInstallJob?.status === 'failed'
  const hasTimedOutInstallJob = hasFailedInstallJob && latestInstallJob?.errorMessage?.includes('超时')
  const steamSdkStartedFromLogs = hasSteamSdkDownloadStarted(jobLogs, selectedJob?.type)
  const installSucceededFromLogs = hasSteamSdkDownloadCompleted(jobLogs, selectedJob?.type)
  const effectiveRawPhase = steamSdkStartedFromLogs && (rawPhase === 'game_downloading' || rawPhase === 'steam_auth_running')
    ? 'steam_sdk_downloading'
    : rawPhase
  const staleAuthState = rawState === 'steam_auth_running' || effectiveRawPhase === 'steam_guard_mobile_required' ||
    effectiveRawPhase === 'steam_guard_required' || effectiveRawPhase === 'steam_auth_retrying' || effectiveRawPhase === 'steam_guard_choice_required' ||
    effectiveRawPhase === 'auth_method_required' || effectiveRawPhase === 'steam_qr_required' || effectiveRawPhase === 'game_downloading' ||
    effectiveRawPhase === 'steam_sdk_downloading'
  const state = installSucceededFromLogs ? 'game_installed' : hasFailedInstallJob && staleAuthState ? 'steam_auth_failed' : rawState
  const phase = installSucceededFromLogs ? 'steam_auth_done'
    : hasTimedOutInstallJob && staleAuthState ? 'install_timeout'
      : hasFailedInstallJob && staleAuthState ? 'steam_auth_failed'
        : effectiveRawPhase


  const needsInstall = state === 'admin_created' || state === 'junimo_scaffolded' ||
    state === 'credentials_required' || state === 'steam_auth_failed' || state === 'uninitialized' || state === '' ||
    state === 'error'
  const isInstalling = state === 'steam_auth_running' || activeInstallJobIsRunning || runningInstallJobId !== null
  const isInstalled = state === 'game_installed' || state === 'save_required' ||
    state === 'ready_to_start' || state === 'starting' || state === 'running' || state === 'stopped'
  const authFailed = state === 'steam_auth_failed'
  const isQrAuthError = phase === 'qr_auth_failed'
  const needsCodeInput = phase === 'steam_guard_required'
  const needsMobileApproval = phase === 'steam_guard_mobile_required'
  const needsGuard = needsCodeInput || needsMobileApproval
  const needsAuthMethodChoice = phase === 'auth_method_required' && isInstalling
  const needsGuardChoice = phase === 'steam_guard_choice_required' && isInstalling
  const needsQrCode = phase === 'steam_qr_required'
  const steamQrLogText = needsQrCode ? extractRecentSteamQrText(jobLogs) : ''
  const gameDownloadProgress = extractSteamDownloadProgress(jobLogs, selectedJob?.type, 'game')
  const steamSdkDownloadProgress = extractSteamDownloadProgress(jobLogs, selectedJob?.type, 'sdk')
  const steamDownloadTaskProgress = calcSteamDownloadTaskProgress(phase, gameDownloadProgress, steamSdkDownloadProgress)
  const installFailureMessage = installFailureDisplayMessage(
    state,
    phase,
    instanceState?.stateMessage ?? '',
    latestInstallJob,
    selectedJob,
    jobLogs,
  )
  const isCredentialError = phase === 'credentials_required'
  const canDirectRetry =
    !isCredentialError && (
      (state === 'junimo_scaffolded' && phase === 'pull_failed') ||
      state === 'error' ||
      state === 'steam_auth_failed' ||
      phase === 'install_timeout' ||
      phase === 'steam_auth_connection_failed' ||
      phase === 'steam_auth_failed' ||
      phase === 'qr_auth_failed' ||
      phase === 'download_failed'
    )
  const displayInstanceState = instanceState && installSucceededFromLogs
    ? {
      ...instanceState,
      state: 'game_installed',
      stateMessage: 'Steam 认证成功，游戏和 Steam SDK 已安装。',
      driverPhase: 'steam_auth_done',
    }
    : instanceState

  function currentInstallJobID() {
    if (activeInstallJobId) return activeInstallJobId
    return runningInstallJobId
  }

  useEffect(() => {
    if (!runningInstallJobId) return
    if (selectedJob?.id === runningInstallJobId) return
    const runningInstallJob = jobs.find((job) => job.id === runningInstallJobId)
    if (runningInstallJob) setSelectedJob(runningInstallJob)
  }, [selectedJob?.id, runningInstallJobId, jobs])

  function openInstallModal() {
    const recommendedTag = imageTagOptions.find((o) => o.recommended)?.tag ?? ''
    if (isCredentialError && lastTriedCredentials) {
      setInstallForm({
        steamUsername: lastTriedCredentials.steamUsername,
        steamPassword: lastTriedCredentials.steamPassword,
        vncPassword: lastTriedCredentials.vncPassword,
        imageTag: recommendedTag,
      })
    } else {
      setInstallForm({ ...emptyInstallForm, imageTag: recommendedTag })
    }
    setInstallMessage('')
    setShowInstallModal(true)
  }

  async function handleInstallClick() {
    if (canDirectRetry) {
      setInstallBusy(true)
      setInstallMessage('')
      try {
        const recommendedTag = imageTagOptions.find((o) => o.recommended)?.tag ?? installForm.imageTag
        const response = await installInstance({ reuseCredentials: true, imageTag: recommendedTag })
        setActiveInstallJobId(response.jobId)
        const jobResponse = await getJob(response.jobId)
        setSelectedJob(jobResponse.job)
        await refreshJobs()
      } catch (error) {
        setInstallMessage(errorMessage(error))
      } finally {
        setInstallBusy(false)
      }
      return
    }
    openInstallModal()
  }

  function handleExtractPullProgress(logs: JobLog[], jobType: string | undefined) {
    if (jobType !== 'stardew_install') return null
    const pullProgressRe = /^\[pull:progress:(\d+):(\d+)\]$/
    let latest: { done: number; total: number } | null = null
    for (const log of logs) {
      const m = log.message.match(pullProgressRe)
      if (m) latest = { done: parseInt(m[1], 10), total: parseInt(m[2], 10) }
    }
    if (!latest || latest.total === 0) return null
    return { ...latest, percent: Math.round((latest.done / latest.total) * 100) }
  }

  const jobStarted = (jobId: string) => {
    void getJob(jobId).then((r) => setSelectedJob(r.job))
    void refreshJobs()
  }

  return (
    <div className="dashboard-grid">
      {/* ── 顶部状态摘要 ──────────────────────────────────── */}
      <div className="dashboard-status-row">
        <div className="status-card">
          <span>当前用户</span>
          <strong>{user.username}</strong>
          <small>{user.role === 'admin' ? '管理员' : '普通用户'}</small>
        </div>
        <InstanceStateCard state={displayInstanceState} onRefresh={refreshState} />
      </div>

      {/* ── 安装区域（仅 admin，未安装时显示） ──────────────── */}
      {user.role === 'admin' ? (
        <InstallSection
          state={state}
          phase={phase}
          stateMessage={instanceState?.stateMessage ?? ''}
          pullProgress={handleExtractPullProgress(jobLogs, selectedJob?.type)}
          gameDownloadProgress={gameDownloadProgress}
          steamSdkDownloadProgress={steamSdkDownloadProgress}
          steamDownloadTaskProgress={steamDownloadTaskProgress}
          needsInstall={needsInstall}
          isInstalling={isInstalling}
          isInstalled={isInstalled}
          authFailed={authFailed}
          isQrAuthError={isQrAuthError}
          needsAuthMethodChoice={needsAuthMethodChoice}
          needsGuard={needsGuard}
          needsGuardChoice={needsGuardChoice}
          needsQrCode={needsQrCode}
          steamQrLogText={steamQrLogText}
          installForm={installForm}
          installBusy={installBusy}
          installMessage={installMessage}
          installFailureMessage={installFailureMessage}
          showInstallModal={showInstallModal}
          guardInput={guardInput}
          guardBusy={guardBusy}
          guardMessage={guardMessage}
          imageTagOptions={imageTagOptions}
          canDirectRetry={canDirectRetry}
          onInstallClick={handleInstallClick}
          onInstallFormChange={setInstallForm}
          onShowInstallModal={setShowInstallModal}
          onInstallSubmit={handleInstallSubmit}
          onGuardInputChange={setGuardInput}
          onGuardSubmit={handleGuardSubmit}
          onAuthMethodSelect={handleAuthMethodSelect}
        />
      ) : null}

      {/* ── 主操作区：生命周期 + 存档管理 | 任务日志 ──────── */}
      {isInstalled ? (
        <div className="dashboard-main">
          <div className="dashboard-main-left">
            <LifecycleSection
              state={state}
              isAdmin={user.role === 'admin'}
              onJobStarted={jobStarted}
              onStateRefresh={refreshState}
            />
            <SavesSection
              state={state}
              isAdmin={user.role === 'admin'}
              onJobStarted={jobStarted}
              onStateRefresh={refreshState}
              refreshTrigger={savesRefreshKey}
            />
          </div>
          <div className="dashboard-main-right">
            <JobsSection
              user={user}
              jobs={jobs}
              selectedJob={selectedJob}
              logs={jobLogs}
              busy={jobBusy}
              message={jobMessage}
              onRefresh={refreshJobs}
              onSelectJob={setSelectedJob}
              onRunTestJob={() => runTestJob(false)}
              onRunFailingTestJob={() => runTestJob(true)}
              onClearJobs={clearJobCenter}
            />
          </div>
        </div>
      ) : (
        <JobsSection
          user={user}
          jobs={jobs}
          selectedJob={selectedJob}
          logs={jobLogs}
          busy={jobBusy}
          message={jobMessage}
          onRefresh={refreshJobs}
          onSelectJob={setSelectedJob}
          onRunTestJob={() => runTestJob(false)}
          onRunFailingTestJob={() => runTestJob(true)}
          onClearJobs={clearJobCenter}
        />
      )}

      {/* ── 高级区域（折叠） ──────────────────────────────── */}
      <div className="dashboard-advanced">
        <button
          className="collapsible-header"
          onClick={() => setShowAdvanced((v) => !v)}
          type="button"
        >
          <span>{showAdvanced ? '▾' : '▸'} 高级设置</span>
          <span className="collapsible-hint">Docker 调试、用户管理</span>
        </button>
        {showAdvanced ? (
          <div className="collapsible-content">
            {user.role === 'admin' ? (
              <DockerSection
                status={dockerStatus}
                composePs={composePs}
                checkedAt={dockerCheckedAt}
                message={dockerMessage}
                busy={dockerBusy}
                onCheckDocker={checkDocker}
                onLoadComposePs={loadComposePs}
              />
            ) : null}

            {user.role === 'admin' ? (
              <section className="users-section">
                <div className="section-heading">
                  <div>
                    <h2>用户管理</h2>
                    <p>管理员可以创建、启用、禁用、删除用户并调整角色。</p>
                  </div>
                  <button className="button button-small" disabled={busy} onClick={onRefreshUsers} type="button">刷新</button>
                </div>
                <form className="create-user-form" onSubmit={onCreateUser} autoComplete="off">
                  <input aria-label="新用户用户名" name="new-panel-username" placeholder="用户名"
                    value={newUserForm.username} autoComplete="off"
                    onChange={(e) => onNewUserChange({ ...newUserForm, username: e.target.value })} required />
                  <PasswordInput value={newUserForm.password} visible={showNewPassword} placeholder="密码"
                    autoComplete="new-password" inputName="new-panel-password"
                    onChange={(p) => onNewUserChange({ ...newUserForm, password: p })}
                    onToggle={() => setShowNewPassword((v) => !v)} />
                  <select aria-label="新用户角色" value={newUserForm.role}
                    onChange={(e) => onNewUserChange({ ...newUserForm, role: e.target.value as 'admin' | 'user' })}>
                    <option value="user">普通用户</option>
                    <option value="admin">管理员</option>
                  </select>
                  <button className="button" disabled={busy} type="submit">创建用户</button>
                </form>
                <div className="user-table" role="table" aria-label="面板用户列表">
                  <div className="user-row user-row-head" role="row">
                    <span>用户名</span><span>角色</span><span>状态</span><span>操作</span>
                  </div>
                  {users.map((panelUser) => (
                    <div className="user-row" key={panelUser.id} role="row">
                      <span>{panelUser.username}</span>
                      <select aria-label={`${panelUser.username} 角色`} value={panelUser.role}
                        disabled={busy || !panelUser.isActive}
                        onChange={(e) => onUpdateRole(panelUser, e.target.value as 'admin' | 'user')}>
                        <option value="user">普通用户</option>
                        <option value="admin">管理员</option>
                      </select>
                      <span className={panelUser.isActive ? 'role-badge' : 'role-badge muted'}>
                        {panelUser.isActive ? '已启用' : '已禁用'}
                      </span>
                      <div className="user-actions">
                        <button
                          className={panelUser.isActive ? 'button button-small button-danger' : 'button button-small'}
                          disabled={busy || panelUser.id === user.id}
                          onClick={() => onSetUserActive(panelUser, !panelUser.isActive)} type="button">
                          {panelUser.isActive ? '禁用' : '启用'}
                        </button>
                        <button className="button button-small button-danger"
                          disabled={busy || panelUser.id === user.id}
                          onClick={() => onDeleteUser(panelUser)} type="button">删除</button>
                      </div>
                    </div>
                  ))}
                </div>
              </section>
            ) : null}
          </div>
        ) : null}
      </div>

      {/* ── 登出 ────────────────────────────────────────────── */}
      <div className="dashboard-footer">
        <button className="button button-secondary" disabled={busy} onClick={onLogout} type="button">
          登出
        </button>
      </div>
    </div>
  )
}

export default App
