import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import {
  ApiError,
  createJobEventSource,
  getComposePs,
  getDockerStatus,
  getJob,
  getJobLogs,
  getJobs,
  getStardewState,
  request,
  startFailingTestJob,
  startTestJob,
} from './api'
import type {
  ComposePsResponse,
  CurrentUser,
  DockerStatusResponse,
  InstanceState,
  Job,
  JobLog,
  JobStatus,
  OKResponse,
  PanelUser,
  PanelUserResponse,
  SetupStatus,
  UserResponse,
  UsersResponse,
} from './types'

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

const emptySetupForm: SetupFormState = {
  username: '',
  password: '',
  confirmPassword: '',
}

const emptyLoginForm: LoginFormState = {
  username: '',
  password: '',
}

const emptyNewUserForm: NewUserFormState = {
  username: '',
  password: '',
  role: 'user',
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
      setNewUserForm({ ...emptyNewUserForm })
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
      setNewUserForm({ ...emptyNewUserForm })
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
      setNewUserForm({ ...emptyNewUserForm })
      setView('login')
      setBusy(false)
    }
  }

  async function createUser(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setBusy(true)
    setMessage('')
    try {
      await request<PanelUserResponse>('/api/users', {
        method: 'POST',
        body: newUserForm,
      })
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
      await request<PanelUserResponse>(`/api/users/${user.id}`, {
        method: 'PATCH',
        body: { role },
      })
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
      await request<PanelUserResponse>(`/api/users/${user.id}`, {
        method: 'PATCH',
        body: { isActive },
      })
      await loadUsers()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function deleteUser(user: PanelUser) {
    if (!window.confirm(`确认永久删除用户“${user.username}”？此操作不可恢复。`)) {
      return
    }
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
        <p className="eyebrow">里程碑 5 · GameDriver Registry</p>
        <h1>Stardew Anxi Panel</h1>
        {message ? <div className="error-banner">{message}</div> : null}
        {view === 'booting' ? <p className="summary">正在读取面板状态……</p> : null}
        {view === 'setup' ? (
          <SetupPanel
            form={setupForm}
            busy={busy}
            onChange={setSetupForm}
            onSubmit={submitSetup}
          />
        ) : null}
        {view === 'login' ? (
          <LoginPanel
            form={loginForm}
            busy={busy}
            onChange={setLoginForm}
            onSubmit={submitLogin}
          />
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

type SetupPanelProps = {
  form: SetupFormState
  busy: boolean
  onChange: (form: SetupFormState) => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
}

function SetupPanel({ form, busy, onChange, onSubmit }: SetupPanelProps) {
  const [showPassword, setShowPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)

  return (
    <form className="form-grid" onSubmit={onSubmit} autoComplete="off">
      <p className="summary">
        当前数据库里还没有管理员。请创建第一个管理员账号，完成后会自动登录。
      </p>
      <Field label="管理员用户名">
        <input
          value={form.username}
          onChange={(event) => onChange({ ...form, username: event.target.value })}
          autoComplete="username"
          required
        />
      </Field>
      <Field label="管理员密码">
        <PasswordInput
          value={form.password}
          visible={showPassword}
          autoComplete="new-password"
          onChange={(password) => onChange({ ...form, password })}
          onToggle={() => setShowPassword((value) => !value)}
        />
      </Field>
      <Field label="确认密码">
        <PasswordInput
          value={form.confirmPassword}
          visible={showConfirmPassword}
          autoComplete="new-password"
          onChange={(confirmPassword) => onChange({ ...form, confirmPassword })}
          onToggle={() => setShowConfirmPassword((value) => !value)}
        />
      </Field>
      <p className="form-hint">密码至少 6 位。</p>
      <button className="button" disabled={busy} type="submit">
        {busy ? '正在创建……' : '创建管理员'}
      </button>
    </form>
  )
}

type LoginPanelProps = {
  form: LoginFormState
  busy: boolean
  onChange: (form: LoginFormState) => void
  onSubmit: (event: FormEvent<HTMLFormElement>) => void
}

function LoginPanel({ form, busy, onChange, onSubmit }: LoginPanelProps) {
  const [showPassword, setShowPassword] = useState(false)

  return (
    <form className="form-grid" onSubmit={onSubmit} autoComplete="on">
      <p className="summary">请输入面板账号登录。登录状态会通过 HttpOnly Cookie 保存。</p>
      <Field label="用户名">
        <input
          value={form.username}
          onChange={(event) => onChange({ ...form, username: event.target.value })}
          autoComplete="username"
          required
        />
      </Field>
      <Field label="密码">
        <PasswordInput
          value={form.password}
          visible={showPassword}
          autoComplete="current-password"
          onChange={(password) => onChange({ ...form, password })}
          onToggle={() => setShowPassword((value) => !value)}
        />
      </Field>
      <button className="button" disabled={busy} type="submit">
        {busy ? '正在登录……' : '登录'}
      </button>
    </form>
  )
}

type DashboardProps = {
  user: CurrentUser
  users: PanelUser[]
  busy: boolean
  newUserForm: NewUserFormState
  onNewUserChange: (form: NewUserFormState) => void
  onCreateUser: (event: FormEvent<HTMLFormElement>) => void
  onUpdateRole: (user: PanelUser, role: 'admin' | 'user') => void
  onSetUserActive: (user: PanelUser, isActive: boolean) => void
  onDeleteUser: (user: PanelUser) => void
  onRefreshUsers: () => void
  onLogout: () => void
}

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
}: DashboardProps) {
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

  useEffect(() => {
    void refreshState()
    void refreshJobs()
  }, [])

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
        if (!closed) {
          setJobMessage(errorMessage(error))
        }
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
      source.close()
    })
    source.onerror = () => {
      source.close()
      if (!closed && activeJob.status === 'running') {
        setStreamFailed(true)
      }
    }

    return () => {
      closed = true
      source.close()
    }
  }, [selectedJob?.id])

  useEffect(() => {
    if (!selectedJob || !streamFailed || isTerminalJobStatus(selectedJob.status)) {
      return
    }
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
        }
      } catch (error) {
        setJobMessage(errorMessage(error))
      }
    }, 2500)
    return () => window.clearInterval(timer)
  }, [selectedJob, streamFailed, jobLogs])

  async function checkDocker() {
    setDockerBusy(true)
    setDockerMessage('')
    try {
      const response = await getDockerStatus()
      setDockerStatus(response)
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
      const response = await getComposePs()
      setComposePs(response)
    } catch (error) {
      setDockerMessage(errorMessage(error))
    } finally {
      setDockerBusy(false)
    }
  }

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

  return (
    <div className="dashboard-grid">
      <div className="status-card">
        <span>当前用户</span>
        <strong>{user.username}</strong>
        <small>{user.role === 'admin' ? '管理员' : '普通用户'}</small>
      </div>
      <div className="status-card">
        <span>用户体系</span>
        <strong>已启用</strong>
        <small>Junimo 安装与 Docker 控制将在后续里程碑接入。</small>
      </div>
      <InstanceStateCard state={instanceState} onRefresh={refreshState} />
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
            <button className="button button-small" disabled={busy} onClick={onRefreshUsers} type="button">
              刷新
            </button>
          </div>

          <form className="create-user-form" onSubmit={onCreateUser} autoComplete="off">
            <input
              aria-label="新用户用户名"
              name="new-panel-username"
              placeholder="用户名"
              value={newUserForm.username}
              autoComplete="off"
              onChange={(event) => onNewUserChange({ ...newUserForm, username: event.target.value })}
              required
            />
            <PasswordInput
              value={newUserForm.password}
              visible={showNewPassword}
              placeholder="密码"
              autoComplete="new-password"
              inputName="new-panel-password"
              onChange={(password) => onNewUserChange({ ...newUserForm, password })}
              onToggle={() => setShowNewPassword((value) => !value)}
            />
            <select
              aria-label="新用户角色"
              value={newUserForm.role}
              onChange={(event) =>
                onNewUserChange({ ...newUserForm, role: event.target.value as 'admin' | 'user' })
              }
            >
              <option value="user">普通用户</option>
              <option value="admin">管理员</option>
            </select>
            <button className="button" disabled={busy} type="submit">
              创建用户
            </button>
          </form>

          <div className="user-table" role="table" aria-label="面板用户列表">
            <div className="user-row user-row-head" role="row">
              <span>用户名</span>
              <span>角色</span>
              <span>状态</span>
              <span>操作</span>
            </div>
            {users.map((panelUser) => (
              <div className="user-row" key={panelUser.id} role="row">
                <span>{panelUser.username}</span>
                <select
                  aria-label={`${panelUser.username} 角色`}
                  value={panelUser.role}
                  disabled={busy || !panelUser.isActive}
                  onChange={(event) => onUpdateRole(panelUser, event.target.value as 'admin' | 'user')}
                >
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
                    onClick={() => onSetUserActive(panelUser, !panelUser.isActive)}
                    type="button"
                  >
                    {panelUser.isActive ? '禁用' : '启用'}
                  </button>
                  <button
                    className="button button-small button-danger"
                    disabled={busy || panelUser.id === user.id}
                    onClick={() => onDeleteUser(panelUser)}
                    type="button"
                  >
                    删除
                  </button>
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

type InstanceStateCardProps = {
  state: InstanceState | null
  onRefresh: () => void
}

function InstanceStateCard({ state, onRefresh }: InstanceStateCardProps) {
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
      <button className="button button-small button-secondary" onClick={onRefresh} type="button">
        刷新状态
      </button>
    </div>
  )
}

type JobsSectionProps = {
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
}

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
}: JobsSectionProps) {
  return (
    <section className="jobs-section">
      <div className="section-heading">
        <div>
          <h2>任务中心</h2>
          <p>本阶段只提供固定测试任务，用于验证长期任务、状态与实时日志。</p>
        </div>
        <div className="job-actions">
          <button className="button button-small button-secondary" disabled={busy} onClick={onRefresh} type="button">
            刷新任务
          </button>
          {user.role === 'admin' ? (
            <>
              <button className="button button-small" disabled={busy} onClick={onRunTestJob} type="button">
                启动测试任务
              </button>
              <button className="button button-small button-danger" disabled={busy} onClick={onRunFailingTestJob} type="button">
                启动失败测试任务
              </button>
            </>
          ) : null}
        </div>
      </div>
      {user.role !== 'admin' ? <p className="form-hint">普通用户只能查看自己有权限的任务，不能创建测试任务。</p> : null}
      {message ? <div className="error-banner docker-error">{message}</div> : null}
      <div className="jobs-layout">
        <div className="jobs-list" role="table" aria-label="最近任务列表">
          <div className="job-row job-row-head" role="row">
            <span>ID</span>
            <span>类型</span>
            <span>状态</span>
            <span>创建</span>
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
              <div className="job-log-window" aria-label="任务日志">
                {logs.length === 0 ? <p>暂无日志。</p> : null}
                {logs.map((log) => (
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

function StatusBadge({ status }: { status: string }) {
  return <span className={`status-badge ${statusClass(status)}`}>{status}</span>
}

function statusClass(status: string) {
  if (status === 'succeeded' || status === 'running') {
    return status
  }
  if (status === 'failed' || status === 'error' || status === 'steam_auth_failed') {
    return 'failed'
  }
  if (status === 'canceled') {
    return 'canceled'
  }
  return 'queued'
}

function isTerminalJobStatus(status: JobStatus) {
  return status === 'succeeded' || status === 'failed' || status === 'canceled'
}

function appendUniqueLog(current: JobLog[], next: JobLog) {
  if (current.some((log) => log.jobId === next.jobId && log.sequence === next.sequence)) {
    return current
  }
  return [...current, next]
}

function shortJobID(id: string) {
  return id.length > 14 ? `${id.slice(0, 10)}…` : id
}

function formatDate(value: string) {
  if (!value) {
    return '-'
  }
  return new Date(value).toLocaleString()
}

type DockerSectionProps = {
  status: DockerStatusResponse | null
  composePs: ComposePsResponse | null
  checkedAt: string
  message: string
  busy: boolean
  onCheckDocker: () => void
  onLoadComposePs: () => void
}

function DockerSection({
  status,
  composePs,
  checkedAt,
  message,
  busy,
  onCheckDocker,
  onLoadComposePs,
}: DockerSectionProps) {
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
                <span>服务</span>
                <span>容器</span>
                <span>状态</span>
                <span>健康</span>
              </div>
              {composePs.services.map((service, index) => (
                <div className="compose-row" key={`${service.name}-${service.service}-${index}`} role="row">
                  <span>{service.service || '-'}</span>
                  <span>{service.name || '-'}</span>
                  <span>{service.state || service.status || '-'}</span>
                  <span>{service.health || '-'}</span>
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

type StatusPillProps = {
  label: string
  ok: boolean | undefined
  emptyLabel: string
}

function StatusPill({ label, ok, emptyLabel }: StatusPillProps) {
  const text = ok === undefined ? emptyLabel : ok ? '可用' : '不可用'
  const className = ok === undefined ? 'docker-status-pill' : ok ? 'docker-status-pill ok' : 'docker-status-pill bad'
  return (
    <div className={className}>
      <span>{label}</span>
      <strong>{text}</strong>
    </div>
  )
}

type CommandOutputProps = {
  title: string
  result?: { stdout: string; stderr: string; exitCode: number; durationMs: number; timedOut: boolean }
}

function CommandOutput({ title, result }: CommandOutputProps) {
  if (!result) {
    return null
  }
  return (
    <div className="compose-output">
      <h3>{title}</h3>
      <p>
        退出码：{result.exitCode}；耗时：{result.durationMs}ms；超时：{result.timedOut ? '是' : '否'}
      </p>
      {result.stdout ? <pre>{result.stdout}</pre> : null}
      {result.stderr ? <pre className="stderr-output">{result.stderr}</pre> : null}
    </div>
  )
}

type FieldProps = {
  label: string
  children: React.ReactNode
}

function Field({ label, children }: FieldProps) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
    </label>
  )
}

type PasswordInputProps = {
  value: string
  visible: boolean
  placeholder?: string
  autoComplete: string
  inputName?: string
  onChange: (value: string) => void
  onToggle: () => void
}

function PasswordInput({
  value,
  visible,
  placeholder,
  autoComplete,
  inputName,
  onChange,
  onToggle,
}: PasswordInputProps) {
  return (
    <div className="password-input">
      <input
        name={inputName}
        type={visible ? 'text' : 'password'}
        value={value}
        placeholder={placeholder}
        autoComplete={autoComplete}
        onChange={(event) => onChange(event.target.value)}
        required
      />
      <button
        className="password-toggle"
        type="button"
        aria-label={visible ? '隐藏密码' : '显示密码'}
        onClick={onToggle}
      >
        {visible ? '隐藏' : '显示'}
      </button>
    </div>
  )
}

function errorMessage(error: unknown) {
  if (error instanceof ApiError) {
    return error.message
  }
  if (error instanceof Error) {
    return error.message
  }
  return '请求失败，请稍后重试。'
}

export default App
