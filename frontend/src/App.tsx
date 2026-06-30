import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { ApiError, getVersion, request } from './api'
import type { VersionInfo } from './api'
import type { CurrentUser, OKResponse, SetupStatus, UserResponse } from './types'

import { SetupPanel, emptySetupForm } from './core/SetupPanel'
import type { SetupFormState } from './core/SetupPanel'
import { LoginPanel, emptyLoginForm } from './core/LoginPanel'
import type { LoginFormState } from './core/LoginPanel'
import { errorMessage } from './core/helpers'

import { StardewPanel } from './games/stardew/StardewPanel'

type View = 'booting' | 'setup' | 'login' | 'stardew'

function App() {
  const [view, setView] = useState<View>('booting')
  const [currentUser, setCurrentUser] = useState<CurrentUser | null>(null)
  const [setupForm, setSetupForm] = useState<SetupFormState>({ ...emptySetupForm })
  const [loginForm, setLoginForm] = useState<LoginFormState>({ ...emptyLoginForm })
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [versionInfo, setVersionInfo] = useState<VersionInfo | null>(null)

  useEffect(() => {
    boot()
    void getVersion().then(setVersionInfo).catch(() => {})
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
        setView('stardew')
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
      setView('stardew')
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
      setView('stardew')
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
      setLoginForm({ ...emptyLoginForm })
      setView('login')
      setBusy(false)
    }
  }

  if (view === 'stardew' && currentUser) {
    return <StardewPanel user={currentUser} onLogout={logout} />
  }

  return (
    <main className="sd-auth-shell">
      <section className="sd-auth-card">
        <p className="sd-auth-eyebrow">Stardew Valley 管理面板</p>
        <h1 className="sd-auth-title">Stardew Anxi Panel</h1>
        {versionInfo ? (
          <p className="sd-auth-version">
            v{versionInfo.version}
            {versionInfo.commit ? ` · ${versionInfo.commit}` : ''}
            {versionInfo.buildDate ? ` · ${versionInfo.buildDate}` : ''}
          </p>
        ) : null}
        {message ? <div className="sd-auth-error">{message}</div> : null}
        {view === 'booting' ? <p className="sd-auth-loading">正在读取面板状态……</p> : null}
        {view === 'setup' ? (
          <SetupPanel form={setupForm} busy={busy} onChange={setSetupForm} onSubmit={submitSetup} />
        ) : null}
        {view === 'login' ? (
          <LoginPanel form={loginForm} busy={busy} onChange={setLoginForm} onSubmit={submitLogin} />
        ) : null}
      </section>
    </main>
  )
}

export default App
