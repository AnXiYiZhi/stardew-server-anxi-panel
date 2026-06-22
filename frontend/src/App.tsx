import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { ApiError, request } from './api'
import type {
  CurrentUser,
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
        <p className="eyebrow">里程碑 2 · 存储与认证</p>
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
                  onChange={(event) =>
                    onUpdateRole(panelUser, event.target.value as 'admin' | 'user')
                  }
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
