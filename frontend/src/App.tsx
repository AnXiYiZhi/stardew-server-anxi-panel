import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import {
  ApiError,
  clearJobs,
  createJobEventSource,
  createNewGame,
  defaultInstanceId,
  getComposePs,
  getDockerStatus,
  getInstallOptions,
  getInviteCode,
  getJob,
  getJobLogs,
  getJobs,
  getStardewState,
  installInstance,
  request,
  restartInstance,
  startFailingTestJob,
  startInstance,
  startTestJob,
  stopInstance,
  submitSteamGuardInput,
  uploadSaveCommitAndStart,
  uploadSavePreview,
} from './api'
import type {
  ComposePsResponse,
  CurrentUser,
  DockerStatusResponse,
  ImageTagOption,
  InstanceState,
  Job,
  JobLog,
  JobStatus,
  NewGameConfig,
  OKResponse,
  PanelUser,
  PanelUserResponse,
  SaveInfo,
  SetupStatus,
  UploadPreviewResult,
  UserResponse,
  UsersResponse,
} from './types'
import { NewGameCreator } from './games/stardew/NewGameCreator'

type View = 'booting' | 'setup' | 'login' | 'dashboard'

type SetupFormState = {
  username: string
  password: string
  confirmPassword: string
}

type LoginFormState = {
  username: string
  password: string
}

type NewUserFormState = {
  username: string
  password: string
  role: 'admin' | 'user'
}

type InstallFormState = {
  steamUsername: string
  steamPassword: string
  vncPassword: string
  imageTag: string
}

const emptySetupForm: SetupFormState = { username: '', password: '', confirmPassword: '' }
const emptyLoginForm: LoginFormState = { username: '', password: '' }
const emptyNewUserForm: NewUserFormState = { username: '', password: '', role: 'user' }
const emptyInstallForm: InstallFormState = { steamUsername: '', steamPassword: '', vncPassword: '', imageTag: '' }

type StepStatus = 'pending' | 'active' | 'done' | 'error'
type ProgressInfo = { percent: number; label: string; status: 'idle' | 'active' | 'done' | 'error' }
type DownloadProgress = {
  filesDone: number
  filesTotal: number
  percent: number
  bytesDone: string
  bytesTotal: string
}

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
        <p className="eyebrow">里程碑 7 · Stardew Junimo Lifecycle</p>
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

// ── Setup / Login panels ──────────────────────────────────────────────────────

function SetupPanel({
  form,
  busy,
  onChange,
  onSubmit,
}: {
  form: SetupFormState
  busy: boolean
  onChange: (f: SetupFormState) => void
  onSubmit: (e: FormEvent<HTMLFormElement>) => void
}) {
  const [showPwd, setShowPwd] = useState(false)
  const [showConfirm, setShowConfirm] = useState(false)
  return (
    <form className="form-grid" onSubmit={onSubmit} autoComplete="off">
      <p className="summary">当前数据库里还没有管理员。请创建第一个管理员账号，完成后会自动登录。</p>
      <Field label="管理员用户名">
        <input value={form.username} autoComplete="username" required
          onChange={(e) => onChange({ ...form, username: e.target.value })} />
      </Field>
      <Field label="管理员密码">
        <PasswordInput value={form.password} visible={showPwd} autoComplete="new-password"
          onChange={(p) => onChange({ ...form, password: p })} onToggle={() => setShowPwd((v) => !v)} />
      </Field>
      <Field label="确认密码">
        <PasswordInput value={form.confirmPassword} visible={showConfirm} autoComplete="new-password"
          onChange={(p) => onChange({ ...form, confirmPassword: p })} onToggle={() => setShowConfirm((v) => !v)} />
      </Field>
      <p className="form-hint">密码至少 6 位。</p>
      <button className="button" disabled={busy} type="submit">{busy ? '正在创建……' : '创建管理员'}</button>
    </form>
  )
}

function LoginPanel({
  form,
  busy,
  onChange,
  onSubmit,
}: {
  form: LoginFormState
  busy: boolean
  onChange: (f: LoginFormState) => void
  onSubmit: (e: FormEvent<HTMLFormElement>) => void
}) {
  const [showPwd, setShowPwd] = useState(false)
  return (
    <form className="form-grid" onSubmit={onSubmit} autoComplete="on">
      <p className="summary">请输入面板账号登录。登录状态会通过 HttpOnly Cookie 保存。</p>
      <Field label="用户名">
        <input value={form.username} autoComplete="username" required
          onChange={(e) => onChange({ ...form, username: e.target.value })} />
      </Field>
      <Field label="密码">
        <PasswordInput value={form.password} visible={showPwd} autoComplete="current-password"
          onChange={(p) => onChange({ ...form, password: p })} onToggle={() => setShowPwd((v) => !v)} />
      </Field>
      <button className="button" disabled={busy} type="submit">{busy ? '正在登录……' : '登录'}</button>
    </form>
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
      // Pre-select the recommended tag when the form is still at default empty value.
      const recommended = res.imageTagOptions.find((o) => o.recommended)
      if (recommended) {
        setInstallForm((prev) => (prev.imageTag === '' ? { ...prev, imageTag: recommended.tag } : prev))
      }
    } catch {
      // Non-fatal; install modal falls back to the backend default.
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
  // canDirectRetry: credentials already in .env, no need to re-enter them.
  // Only credentials_required should ask for Steam/VNC fields again; network/CM failures,
  // timeouts, QR failures, and generic auth failures should reuse saved .env credentials.
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

  return (
    <div className="dashboard-grid">
      <div className="status-card">
        <span>当前用户</span>
        <strong>{user.username}</strong>
        <small>{user.role === 'admin' ? '管理员' : '普通用户'}</small>
      </div>

      {/* Stardew 实例状态卡 */}
      <InstanceStateCard state={displayInstanceState} onRefresh={refreshState} />

      {/* 安装区域（仅 admin） */}
      {user.role === 'admin' ? (
        <InstallSection
          state={state}
          phase={phase}
          stateMessage={instanceState?.stateMessage ?? ''}
          pullProgress={extractPullProgress(jobLogs, selectedJob?.type)}
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

      {/* 生命周期区域：game_installed 之后全部用户可见（非安装状态） */}
      {isInstalled ? (
        <LifecycleSection
          state={state}
          isAdmin={user.role === 'admin'}
          onJobStarted={(jobId) => {
            void getJob(jobId).then((r) => setSelectedJob(r.job))
            void refreshJobs()
          }}
          onStateRefresh={refreshState}
        />
      ) : null}

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
      ) : (
        <div className="status-card">
          <span>Docker / Compose</span>
          <strong>仅管理员可用</strong>
          <small>Docker 状态检查和 Compose 调试接口需要管理员权限。</small>
        </div>
      )}

      <button className="button button-secondary" disabled={busy} onClick={onLogout} type="button">
        登出
      </button>

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
      ) : (
        <p className="summary">当前账号没有用户管理权限。</p>
      )}
    </div>
  )
}

// ── Install Section ───────────────────────────────────────────────────────────

function InstallSection({
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
              <small>启动服务器将在下一阶段实现。</small>
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

// ── Instance State Card ───────────────────────────────────────────────────────

function InstanceStateCard({ state, onRefresh }: { state: InstanceState | null; onRefresh: () => void }) {
  return (
    <div className="status-card instance-state-card">
      <div>
        <span>{state?.name ?? 'Stardew Valley'} 实例状态</span>
        <strong>{state?.state ?? 'unknown'}</strong>
        <small>{state?.driverId ? `Driver: ${state.driverId}` : 'Driver: stardew_junimo'}</small>
        <small>{state?.stateMessage ?? '尚未读取实例状态。'}</small>
      </div>
      <StatusBadge status={state?.state ?? 'unknown'} />
      <small>更新时间：{state?.updatedAt ? formatDate(state.updatedAt) : '未读取'}</small>
      <button className="button button-small button-secondary" onClick={onRefresh} type="button">刷新状态</button>
    </div>
  )
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

// ── Jobs Section ──────────────────────────────────────────────────────────────

function JobsSection({
  user,
  jobs,
  selectedJob,
  logs,
  busy,
  message,
  onRefresh,
  onSelectJob,
  onRunTestJob,
  onRunFailingTestJob,
  onClearJobs,
}: {
  user: CurrentUser
  jobs: Job[]
  selectedJob: Job | null
  logs: JobLog[]
  busy: boolean
  message: string
  onRefresh: () => void
  onSelectJob: (job: Job) => void
  onRunTestJob: () => void
  onRunFailingTestJob: () => void
  onClearJobs: () => void
}) {
  return (
    <section className="jobs-section">
      <div className="section-heading">
        <div>
          <h2>任务中心</h2>
          <p>安装任务、失败原因和实时日志都在这里显示。</p>
        </div>
        <div className="job-actions">
          <button className="button button-small button-secondary" disabled={busy} onClick={onRefresh} type="button">刷新任务</button>
          {user.role === 'admin' ? (
            <>
              <button className="button button-small" disabled={busy} onClick={onRunTestJob} type="button">启动测试任务</button>
              <button className="button button-small button-danger" disabled={busy} onClick={onRunFailingTestJob} type="button">启动失败测试任务</button>
              <button className="button button-small button-danger" disabled={busy || jobs.length === 0} onClick={onClearJobs} type="button">清空任务中心</button>
            </>
          ) : null}
        </div>
      </div>
      {user.role !== 'admin' ? <p className="form-hint">普通用户只能查看自己有权限的任务，不能创建测试任务。</p> : null}
      {message ? <div className="error-banner docker-error">{message}</div> : null}
      <div className="jobs-layout">
        <div className="jobs-list" role="table" aria-label="最近任务列表">
          <div className="job-row job-row-head" role="row">
            <span>ID</span><span>类型</span><span>状态</span><span>创建</span>
          </div>
          {jobs.length === 0 ? <p className="summary compact">暂无任务。</p> : null}
          {jobs.map((job) => (
            <button
              className={selectedJob?.id === job.id ? 'job-row selected' : 'job-row'}
              key={job.id}
              onClick={() => onSelectJob(job)}
              type="button"
            >
              <span title={job.id}>{shortJobID(job.id)}</span>
              <span>{job.type}</span>
              <StatusBadge status={job.status} />
              <span>{formatDate(job.createdAt)}</span>
            </button>
          ))}
        </div>
        <div className="job-detail">
          {selectedJob ? (
            <>
              <div className="job-detail-head">
                <div>
                  <h3>{selectedJob.type}</h3>
                  <p>{selectedJob.id}</p>
                </div>
                <StatusBadge status={selectedJob.status} />
              </div>
              <p>
                创建：{formatDate(selectedJob.createdAt)}；完成：
                {selectedJob.finishedAt ? formatDate(selectedJob.finishedAt) : '尚未完成'}
              </p>
              {selectedJob.errorMessage ? <div className="error-banner docker-error">{selectedJob.errorMessage}</div> : null}
              {(() => {
                const pp = extractPullProgress(logs, selectedJob.type)
                if (!pp) return null
                return (
                  <div className="pull-progress-container">
                    <div className="pull-progress-header">
                      <span>拉取镜像</span>
                      <span>{pp.done}/{pp.total} 服务</span>
                    </div>
                    <div className="progress-bar-wrap">
                      <div className="progress-bar-track">
                        <div
                          className={`progress-bar-fill${pp.done === pp.total ? ' done' : ''}`}
                          style={{ width: `${pp.percent}%` }}
                        />
                      </div>
                      <span className="progress-bar-percent">{pp.percent}%</span>
                    </div>
                  </div>
                )
              })()}
              <div className="job-log-window" aria-label="任务日志">
                {logs.length === 0 ? <p>暂无日志。</p> : null}
                {logs.filter((log) => !pullProgressRe.test(log.message)).map((log) => (
                  <div className={`job-log-line ${log.level}`} key={`${log.jobId}-${log.sequence}`}>
                    <span>{String(log.sequence).padStart(3, '0')}</span>
                    <strong>{log.level}</strong>
                    <p>{log.message}</p>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <p className="summary compact">选择一个任务查看详情和日志。</p>
          )}
        </div>
      </div>
    </section>
  )
}

// ── Docker Section ────────────────────────────────────────────────────────────

function DockerSection({
  status,
  composePs,
  checkedAt,
  message,
  busy,
  onCheckDocker,
  onLoadComposePs,
}: {
  status: DockerStatusResponse | null
  composePs: ComposePsResponse | null
  checkedAt: string
  message: string
  busy: boolean
  onCheckDocker: () => void
  onLoadComposePs: () => void
}) {
  return (
    <section className="docker-section">
      <div className="section-heading">
        <div>
          <h2>Docker 状态</h2>
          <p>本区域只做本机联调检查，不会拉取镜像或启动 Junimo 容器。</p>
        </div>
        <div className="docker-actions">
          <button className="button button-small" disabled={busy} onClick={onCheckDocker} type="button">
            {busy ? '正在检查……' : '检查 Docker'}
          </button>
          <button className="button button-small button-secondary" disabled={busy} onClick={onLoadComposePs} type="button">
            {busy ? '正在读取……' : '查看 Compose PS'}
          </button>
        </div>
      </div>
      {message ? <div className="error-banner docker-error">{message}</div> : null}
      <div className="docker-status-grid">
        <StatusPill label="Docker" ok={status?.docker.available} emptyLabel="未检查" />
        <StatusPill label="Compose" ok={status?.compose.available} emptyLabel="未检查" />
        <StatusPill label="Compose 目录" ok={status?.composeProject.ready} emptyLabel="未检查" />
        <div className="docker-status-pill">
          <span>最近检查</span>
          <strong>{checkedAt || '未检查'}</strong>
        </div>
      </div>
      {status ? (
        <div className="compose-output-grid">
          <CommandOutput title="Docker version" result={status.docker.result} />
          <CommandOutput title="Compose version" result={status.compose.result} />
          <div className="compose-output">
            <h3>Compose 工作目录</h3>
            <p>{status.composeProject.workDir}</p>
            <p>
              目录：{status.composeProject.workDirExists ? '存在' : '不存在'}；Compose 文件：
              {status.composeProject.composeFileExists ? '存在' : '不存在'}。
            </p>
          </div>
        </div>
      ) : null}
      {composePs ? (
        <div className="compose-output">
          <h3>Compose PS</h3>
          <p>工作目录：{composePs.workDir}</p>
          {composePs.services.length > 0 ? (
            <div className="compose-table" role="table" aria-label="Compose 服务列表">
              <div className="compose-row compose-row-head" role="row">
                <span>服务</span><span>容器</span><span>状态</span><span>健康</span>
              </div>
              {composePs.services.map((svc, i) => (
                <div className="compose-row" key={`${svc.name}-${svc.service}-${i}`} role="row">
                  <span>{svc.service || '-'}</span>
                  <span>{svc.name || '-'}</span>
                  <span>{svc.state || svc.status || '-'}</span>
                  <span>{svc.health || '-'}</span>
                </div>
              ))}
            </div>
          ) : (
            <p>当前没有 Compose 服务，或当前 Compose 版本没有返回结构化服务列表。</p>
          )}
          <CommandOutput title="原始输出" result={composePs.result} />
        </div>
      ) : null}
    </section>
  )
}

// ── Shared UI primitives ──────────────────────────────────────────────────────

function StatusBadge({ status }: { status: string }) {
  return <span className={`status-badge ${statusClass(status)}`}>{status}</span>
}

function statusClass(status: string) {
  if (status === 'succeeded' || status === 'running' || status === 'game_installed' || status === 'ready_to_start') {
    return 'succeeded'
  }
  if (status === 'failed' || status === 'error' || status === 'steam_auth_failed') return 'failed'
  if (status === 'canceled') return 'canceled'
  if (status === 'steam_auth_running' || status === 'installing') return 'running'
  return 'queued'
}

function isTerminalJobStatus(status: JobStatus) {
  return status === 'succeeded' || status === 'failed' || status === 'canceled'
}

function appendUniqueLog(current: JobLog[], next: JobLog) {
  if (current.some((l) => l.jobId === next.jobId && l.sequence === next.sequence)) return current
  return [...current, next]
}

const pullProgressRe = /^\[pull:progress:(\d+):(\d+)\]$/
const steamDownloadProgressRe = /\[steam\].*Progress:\s*(\d+)\/(\d+)\s+files\s+-\s+([^/()]+?)\/([^()]+?)\s+\((\d+(?:\.\d+)?)%\)/i

function extractPullProgress(logs: JobLog[], jobType: string | undefined): { done: number; total: number; percent: number } | null {
  if (jobType !== 'stardew_install') return null
  let latest: { done: number; total: number } | null = null
  for (const log of logs) {
    const m = log.message.match(pullProgressRe)
    if (m) latest = { done: parseInt(m[1], 10), total: parseInt(m[2], 10) }
  }
  if (!latest || latest.total === 0) return null
  return { ...latest, percent: Math.round((latest.done / latest.total) * 100) }
}

function extractSteamDownloadProgress(logs: JobLog[], jobType: string | undefined, kind: 'game' | 'sdk'): DownloadProgress | null {
  if (jobType !== 'stardew_install') return null
  let currentKind: 'game' | 'sdk' | null = null
  let sawSdk = false
  let latest: DownloadProgress | null = null
  for (const log of logs) {
    const lower = log.message.toLowerCase()
    if (lower.includes('[steam]') && lower.includes('downloading app 413150')) {
      currentKind = 'game'
    } else if (
      lower.includes('[steam]') &&
      (lower.includes('downloading app 1007') || lower.includes('.steam-sdk'))
    ) {
      currentKind = 'sdk'
      sawSdk = true
    }

    const m = log.message.match(steamDownloadProgressRe)
    if (m) {
      const progressKind = currentKind ?? (sawSdk ? 'sdk' : 'game')
      if (progressKind === kind) {
        latest = {
          filesDone: parseInt(m[1], 10),
          filesTotal: parseInt(m[2], 10),
          bytesDone: m[3].trim(),
          bytesTotal: m[4].trim(),
          percent: fileCountPercent(parseInt(m[1], 10), parseInt(m[2], 10)),
        }
      }
    } else if (lower.includes('[steam]') && lower.includes('download complete')) {
      if (currentKind === kind) latest = completeSteamDownloadProgress(latest)
    }
  }
  return latest
}

function hasSteamSdkDownloadStarted(logs: JobLog[], jobType: string | undefined) {
  if (jobType !== 'stardew_install') return false
  return logs.some((log) => {
    const lower = log.message.toLowerCase()
    return lower.includes('[steam]') && (lower.includes('downloading app 1007') || lower.includes('.steam-sdk'))
  })
}

function hasSteamSdkDownloadCompleted(logs: JobLog[], jobType: string | undefined) {
  if (jobType !== 'stardew_install') return false
  return logs.some((log) => {
    const lower = log.message.toLowerCase()
    return lower.includes('[steam]') && lower.includes('app installed to:') && lower.includes('/data/game/.steam-sdk')
  })
}

function fileCountPercent(done: number, total: number) {
  if (total <= 0) return 0
  return roundPercent((done / total) * 100)
}

function roundPercent(value: number) {
  return Math.min(100, Math.max(0, Math.round(value * 10) / 10))
}

function formatPercent(value: number) {
  const rounded = roundPercent(value)
  return `${Number.isInteger(rounded) ? rounded.toFixed(0) : rounded.toFixed(1)}%`
}

function completeSteamDownloadProgress(progress: DownloadProgress | null): DownloadProgress | null {
  if (!progress) return null
  return {
    filesDone: progress.filesTotal,
    filesTotal: progress.filesTotal,
    bytesDone: progress.bytesTotal,
    bytesTotal: progress.bytesTotal,
    percent: 100,
  }
}

function calcSteamDownloadTaskProgress(
  phase: string,
  gameProgress: DownloadProgress | null,
  sdkProgress: DownloadProgress | null,
) {
  if (phase !== 'game_downloading' && phase !== 'steam_sdk_downloading') return null
  if (phase === 'steam_sdk_downloading') {
    return {
      done: sdkProgress?.percent === 100 ? 2 : 1,
      total: 2,
      percent: sdkProgress?.percent === 100 ? 100 : roundPercent(50 + (sdkProgress?.percent ?? 0) / 2),
      label: sdkProgress?.percent === 100
        ? '游戏文件和 Steam SDK 均已下载完成。'
        : '游戏文件已下载完成，正在下载 Steam SDK 运行文件。',
    }
  }
  return {
    done: gameProgress?.percent === 100 ? 1 : 0,
    total: 2,
    percent: roundPercent((gameProgress?.percent ?? 0) / 2),
    label: '正在校验/下载 Stardew Valley 游戏文件；已存在且校验通过的文件会自动跳过。',
  }
}

function extractRecentSteamQrText(logs: JobLog[]) {
  const steamLines = logs
    .filter((log) => log.message.startsWith('[steam] '))
    .map((log) => log.message.replace(/^\[steam\] /, ''))
  let qrIndex = -1
  for (let i = steamLines.length - 1; i >= 0; i -= 1) {
    if (steamLines[i].toLowerCase().includes('qr')) {
      qrIndex = i
      break
    }
  }
  if (qrIndex < 0) return steamLines.slice(-20).join('\n')
  return steamLines.slice(qrIndex, qrIndex + 40).join('\n')
}

function qrCodeFontSize(text: string) {
  const lines = text.split('\n')
  const longest = lines.reduce((max, line) => Math.max(max, line.length), 0)
  if (lines.length > 42 || longest > 92) return 9
  if (lines.length > 36 || longest > 82) return 10
  if (lines.length > 30 || longest > 72) return 11
  return 12
}

function shortJobID(id: string) {
  return id.length > 14 ? `${id.slice(0, 10)}…` : id
}

function formatDate(value: string) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function StatusPill({ label, ok, emptyLabel }: { label: string; ok: boolean | undefined; emptyLabel: string }) {
  const text = ok === undefined ? emptyLabel : ok ? '可用' : '不可用'
  const className = ok === undefined ? 'docker-status-pill' : ok ? 'docker-status-pill ok' : 'docker-status-pill bad'
  return (
    <div className={className}>
      <span>{label}</span>
      <strong>{text}</strong>
    </div>
  )
}

function CommandOutput({ title, result }: {
  title: string
  result?: { stdout: string; stderr: string; exitCode: number; durationMs: number; timedOut: boolean }
}) {
  if (!result) return null
  return (
    <div className="compose-output">
      <h3>{title}</h3>
      <p>退出码：{result.exitCode}；耗时：{result.durationMs}ms；超时：{result.timedOut ? '是' : '否'}</p>
      {result.stdout ? <pre>{result.stdout}</pre> : null}
      {result.stderr ? <pre className="stderr-output">{result.stderr}</pre> : null}
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
    </label>
  )
}

function PasswordInput({
  value,
  visible,
  placeholder,
  autoComplete,
  inputName,
  onChange,
  onToggle,
}: {
  value: string
  visible: boolean
  placeholder?: string
  autoComplete: string
  inputName?: string
  onChange: (v: string) => void
  onToggle: () => void
}) {
  return (
    <div className="password-input">
      <input
        name={inputName}
        type={visible ? 'text' : 'password'}
        value={value}
        placeholder={placeholder}
        autoComplete={autoComplete}
        onChange={(e) => onChange(e.target.value)}
        required
      />
      <button className="password-toggle" type="button"
        aria-label={visible ? '隐藏密码' : '显示密码'}
        onClick={onToggle}>{visible ? '隐藏' : '显示'}</button>
    </div>
  )
}

function installFailureDisplayMessage(
  state: string,
  phase: string,
  stateMessage: string,
  latestInstallJob: Job | undefined,
  selectedJob: Job | null,
  logs: JobLog[],
) {
  const failedInstallJob = latestInstallJob?.type === 'stardew_install' && latestInstallJob.status === 'failed'
    ? latestInstallJob
    : null
  const failedSelectedInstallJob = selectedJob?.type === 'stardew_install' && selectedJob.status === 'failed'
    ? selectedJob
    : null
  const failedJob = failedInstallJob ?? failedSelectedInstallJob
  const errorPhase = [
    'pull_failed',
    'install_timeout',
    'steam_auth_connection_failed',
    'steam_auth_failed',
    'credentials_required',
    'qr_auth_failed',
    'download_failed',
    'steam_auth_console_failed',
  ].includes(phase)
  const isFailureState = state === 'steam_auth_failed' || state === 'error' || errorPhase || !!failedJob
  if (!isFailureState || state === 'game_installed') return ''

  const lastErrorLog = failedJob && selectedJob?.id === failedJob.id
    ? [...logs].reverse().find((log) => log.level === 'error')?.message ?? ''
    : ''
  const rawText = [stateMessage, failedJob?.errorMessage ?? '', lastErrorLog].filter(Boolean).join(' ')
  const lower = rawText.toLowerCase()

  if (phase === 'install_timeout' || lower.includes('任务超时') || lower.includes('timed out')) {
    return '安装任务超时：Steam 认证或下载没有在限定时间内完成，请重试安装。'
  }
  if (
    phase === 'steam_auth_connection_failed' ||
    lower.includes('tryanothercm') ||
    lower.includes('steam client not connected') ||
    lower.includes('steamclient') ||
    lower.includes('cm')
  ) {
    return 'Steam CM 连接失败或超时：当前网络连接 Steam 会话不稳定，请稍后重试；如果一直失败，建议改用扫码登录或先在可用网络完成一次 refresh token。'
  }
  if (phase === 'credentials_required' || lower.includes('invalid password') || lower.includes('incorrect password')) {
    return 'Steam 账号或密码认证失败，请重新输入凭据后再试。'
  }
  if (phase === 'qr_auth_failed') {
    return 'Steam 二维码登录失败：当前 steam-auth 容器未能连接 SteamClient，请改用账号密码/验证码登录。'
  }
  if (phase === 'download_failed' || lower.includes('download failed')) {
    return '游戏文件下载失败：Steam 认证可能已经成功，但下载阶段失败，请检查网络、磁盘空间后重试。'
  }
  if (phase === 'pull_failed') {
    return 'Junimo 镜像拉取失败，请检查 Docker 网络或镜像地址后重试。'
  }

  if (stateMessage && !stateMessage.includes('正在') && !stateMessage.includes('请稍候')) return stateMessage
  if (lastErrorLog) return lastErrorLog.replace(/^\[steam\]\s*/, '')
  if (failedJob?.errorMessage) return failedJob.errorMessage
  return '安装任务失败，请查看任务中心日志后重试。'
}

function errorMessage(error: unknown) {
  if (error instanceof ApiError) return error.message
  if (error instanceof Error) return error.message
  return '请求失败，请稍后重试。'
}

// ── Lifecycle Section ─────────────────────────────────────────────────────────

function LifecycleSection({
  state,
  isAdmin,
  onJobStarted,
  onStateRefresh,
}: {
  state: string
  isAdmin: boolean
  onJobStarted: (jobId: string) => void
  onStateRefresh: () => void
}) {
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const [inviteCode, setInviteCode] = useState('')
  const [showNewGameModal, setShowNewGameModal] = useState(false)
  const [newGameError, setNewGameError] = useState('')
  const [showUploadModal, setShowUploadModal] = useState(false)
  const [uploadFile, setUploadFile] = useState<File | null>(null)
  const [uploadPreview, setUploadPreview] = useState<UploadPreviewResult | null>(null)
  const [uploadBusy, setUploadBusy] = useState(false)
  const [uploadMessage, setUploadMessage] = useState('')

  const canStart = state === 'game_installed' || state === 'save_required' || state === 'ready_to_start' || state === 'stopped'
  const isRunning = state === 'running'
  const isStarting = state === 'starting'

  async function handleStart() {
    setBusy(true)
    setMessage('')
    try {
      const res = await startInstance()
      onJobStarted(res.jobId)
      onStateRefresh()
    } catch (error) {
      if (error instanceof ApiError && error.code === 'save_required') {
        setMessage('没有可用存档。请使用下方“创建存档并启动”或“上传存档并启动”。')
        document.getElementById('save-start-panel')?.scrollIntoView({ behavior: 'smooth', block: 'center' })
        return
      }
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleStop() {
    if (!window.confirm('确定停止服务器吗？')) return
    setBusy(true)
    setMessage('')
    try {
      await stopInstance()
      onStateRefresh()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleRestart() {
    if (!window.confirm('确定重启服务器吗？')) return
    setBusy(true)
    setMessage('')
    try {
      await restartInstance()
      onStateRefresh()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleGetInviteCode() {
    setBusy(true)
    setMessage('')
    try {
      const res = await getInviteCode()
      setInviteCode(res.inviteCode)
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleNewGameSubmit(cfg: NewGameConfig) {
    setBusy(true)
    setNewGameError('')
    try {
      const res = await createNewGame(cfg)
      setShowNewGameModal(false)
      onJobStarted(res.jobId)
      onStateRefresh()
    } catch (error) {
      setNewGameError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleUploadPreview() {
    if (!uploadFile) return
    setUploadBusy(true)
    setUploadMessage('')
    try {
      const res = await uploadSavePreview(uploadFile)
      setUploadPreview(res)
    } catch (error) {
      setUploadMessage(errorMessage(error))
    } finally {
      setUploadBusy(false)
    }
  }

  async function handleUploadCommit() {
    if (!uploadPreview) return
    setUploadBusy(true)
    setUploadMessage('')
    try {
      const res = await uploadSaveCommitAndStart(uploadPreview.token)
      setShowUploadModal(false)
      setUploadPreview(null)
      setUploadFile(null)
      onJobStarted(res.jobId)
      onStateRefresh()
    } catch (error) {
      setUploadMessage(errorMessage(error))
    } finally {
      setUploadBusy(false)
    }
  }

  async function handleUploadCancel() {
    if (uploadPreview) {
      try {
        await uploadSaveCommitAndStart(uploadPreview.token, true)
      } catch { /* best effort */ }
    }
    setShowUploadModal(false)
    setUploadPreview(null)
    setUploadFile(null)
    setUploadMessage('')
  }

  return (
    <section className="lifecycle-section">
      <div className="section-heading">
        <div>
          <h2>服务器生命周期</h2>
          <p>启动、停止、重启 Stardew Junimo 服务器。</p>
        </div>
      </div>

      {message ? <div className="error-banner">{message}</div> : null}

      {/* 状态标签 */}
      <div className="lifecycle-state">
        <span className="lifecycle-state-label">当前状态：</span>
        <span className={`lifecycle-state-badge lifecycle-state-${state}`}>{stateLabel(state)}</span>
      </div>

      {/* 邀请码 */}
      {inviteCode ? (
        <div className="invite-code-display">
          <span>邀请码：</span>
          <strong className="invite-code">{inviteCode}</strong>
        </div>
      ) : null}

      {/* 独立存档启动面板：始终提供显式创建/上传路径。 */}
      {isAdmin && !isRunning && !isStarting ? (
        <div id="save-start-panel" className="preflight-result">
          <p className="preflight-heading">存档启动</p>
          <p className="form-hint">创建或上传存档后会自动启动服务器。</p>
          <div className="lifecycle-actions">
            <button className="button" disabled={busy} onClick={() => setShowNewGameModal(true)} type="button">
              创建存档并启动
            </button>
            <button className="button button-secondary" disabled={busy} onClick={() => setShowUploadModal(true)} type="button">
              上传存档并启动
            </button>
          </div>
        </div>
      ) : null}

      {/* 启动服务器独立于创建/上传：默认由 Junimo 继续加载上次使用的可用存档。 */}
      {isAdmin && (canStart || isRunning || isStarting) ? (
        <div className="lifecycle-actions">
          {canStart ? (
            <button className="button" disabled={busy} onClick={handleStart} type="button">
              {busy ? '启动中...' : '启动服务器（使用上次存档）'}
            </button>
          ) : null}
          {isRunning ? (
            <>
              <button className="button button-secondary" disabled={busy} onClick={handleRestart} type="button">
                重启
              </button>
              <button className="button button-danger" disabled={busy} onClick={handleStop} type="button">
                停止
              </button>
              <button className="button button-secondary" disabled={busy} onClick={handleGetInviteCode} type="button">
                获取邀请码
              </button>
            </>
          ) : null}
          {isStarting ? (
            <p className="summary">服务器正在启动，请稍候...</p>
          ) : null}
        </div>
      ) : null}

      {/* 新建游戏 Modal */}
      {showNewGameModal ? (
        <div className="modal-overlay">
          <div className="modal-card" style={{ maxWidth: 720, width: '95vw' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
              <h3 style={{ margin: 0 }}>新建游戏</h3>
              <button className="button button-small button-secondary" type="button"
                onClick={() => { setShowNewGameModal(false); setNewGameError('') }}>
                关闭
              </button>
            </div>
            <NewGameCreator
              instanceId={defaultInstanceId}
              onSubmit={handleNewGameSubmit}
              submitting={busy}
              submitError={newGameError}
            />
          </div>
        </div>
      ) : null}

      {/* 上传存档 Modal */}
      {showUploadModal ? (
        <div className="modal-overlay">
          <div className="modal-card">
            <h3>上传存档</h3>
            {uploadMessage ? <div className="error-banner">{uploadMessage}</div> : null}
            {!uploadPreview ? (
              <div className="form-grid">
                <p className="form-hint">上传一个包含 Stardew Valley 存档的 ZIP 文件（最大 100 MB）。</p>
                <Field label="选择 ZIP 文件">
                  <input type="file" accept=".zip"
                    onChange={(e) => setUploadFile(e.target.files?.[0] ?? null)} />
                </Field>
                <div className="modal-actions">
                  <button className="button" disabled={uploadBusy || !uploadFile} onClick={handleUploadPreview} type="button">
                    {uploadBusy ? '解析中...' : '预览存档'}
                  </button>
                  <button className="button button-secondary" disabled={uploadBusy} type="button"
                    onClick={handleUploadCancel}>
                    取消
                  </button>
                </div>
              </div>
            ) : (
              <div>
                <p className="preflight-heading">存档预览：</p>
                <SaveCard save={uploadPreview.preview} />
                <p className="form-hint">确认后将导入并启动服务器。</p>
                <div className="modal-actions">
                  <button className="button" disabled={uploadBusy} onClick={handleUploadCommit} type="button">
                    {uploadBusy ? '导入中...' : '确认导入并启动'}
                  </button>
                  <button className="button button-secondary" disabled={uploadBusy} type="button"
                    onClick={handleUploadCancel}>
                    取消
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      ) : null}
    </section>
  )
}

function SaveCard({ save }: { save: SaveInfo }) {
  return (
    <div className="save-card">
      <div className="save-card-name">{save.name}</div>
      {save.parseError ? (
        <div className="save-card-hint">解析失败：{save.parseError}</div>
      ) : (
        <div className="save-card-meta">
          {save.farmerName ? <span>农民：{save.farmerName}</span> : <span className="muted">农民名：未读取到</span>}
          {save.farmName ? <span>农场：{save.farmName}</span> : <span className="muted">农场名：未读取到</span>}
          {save.gameYear ? <span>第 {save.gameYear} 年 {save.gameSeason} 第 {save.gameDay} 天</span> : null}
          {save.farmType ? <span>地图：{save.farmType}</span> : null}
          {save.fileSizeBytes ? <span>大小：{formatBytes(save.fileSizeBytes)}</span> : null}
        </div>
      )}
    </div>
  )
}

function stateLabel(state: string): string {
  const labels: Record<string, string> = {
    game_installed: '游戏已安装',
    save_required: '需要选择存档',
    ready_to_start: '准备启动',
    starting: '启动中',
    running: '运行中',
    stopped: '已停止',
    error: '错误',
  }
  return labels[state] ?? state
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

export default App
