import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { ApiError, getVersion, request, setDefaultInstanceId } from './api'
import type { VersionInfo } from './api'
import { parseAppRoute } from './app-routes'
import { subscribeSessionExpired } from './auth-session-events'
import type { CurrentUser, OKResponse, SetupStatus, UserResponse } from './types'

import { SetupPanel, emptySetupForm } from './core/SetupPanel'
import type { SetupFormState } from './core/SetupPanel'
import { LoginPanel, emptyLoginForm } from './core/LoginPanel'
import type { LoginFormState } from './core/LoginPanel'
import { errorMessage } from './core/helpers'

import { StardewPanel } from './games/stardew/StardewPanel'
import { StardewMobileShell } from './games/stardew/StardewMobileShell'
import { PanelUpdateProvider } from './games/stardew/PanelUpdateProvider'
import { GamesPage } from './games/GameLibrary'
import {
  COMPACT_SHELL_MEDIA_QUERY,
  shouldForceCompactShell,
} from './games/stardew/responsive-layout'
import { useMediaQuery } from './hooks/useMediaQuery'

type View = 'booting' | 'setup' | 'login' | 'authenticated'

type BrowserLocation = {
  pathname: string
  search: string
}

function App() {
  const automaticallyUsesCompactShell = useMediaQuery(COMPACT_SHELL_MEDIA_QUERY)
  const [forceCompactShell] = useState(() => shouldForceCompactShell(window.location.search))
  const usesCompactShell = forceCompactShell || automaticallyUsesCompactShell
  const [desktopShellRequested, setDesktopShellRequested] = useState(false)
  const [view, setView] = useState<View>('booting')
  const [browserLocation, setBrowserLocation] = useState<BrowserLocation>(() => ({
    pathname: window.location.pathname,
    search: window.location.search,
  }))
  const [configuredDefaultInstanceId, setConfiguredDefaultInstanceId] = useState('stardew')
  const [currentUser, setCurrentUser] = useState<CurrentUser | null>(null)
  const [setupForm, setSetupForm] = useState<SetupFormState>({ ...emptySetupForm })
  const [loginForm, setLoginForm] = useState<LoginFormState>({ ...emptyLoginForm })
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const [versionInfo, setVersionInfo] = useState<VersionInfo | null>(null)

  useEffect(() => {
    if (view !== 'authenticated') return
    return subscribeSessionExpired(() => {
      setCurrentUser(null)
      setDesktopShellRequested(false)
      setLoginForm({ ...emptyLoginForm })
      setMessage('登录已失效，请重新登录后继续。')
      setView('login')
    })
  }, [view])

  useEffect(() => {
    boot()
    void getVersion().then(setVersionInfo).catch(() => {})
  }, [])

  useEffect(() => {
    const onPopState = () => {
      setBrowserLocation({ pathname: window.location.pathname, search: window.location.search })
    }
    window.addEventListener('popstate', onPopState)
    return () => window.removeEventListener('popstate', onPopState)
  }, [])

  const appRoute = parseAppRoute(
    browserLocation.pathname,
    browserLocation.search,
    configuredDefaultInstanceId,
  )

  useEffect(() => {
    if (view !== 'authenticated') {
      document.title = 'Anxi Game Panel'
      return
    }
    if (appRoute.kind === 'games') {
      document.title = '游戏库 · Anxi Game Panel'
      return
    }
    if (appRoute.kind === 'stardew-worlds' || appRoute.kind === 'stardew-new-world') {
      document.title = '星露谷世界 · Anxi Game Panel'
      return
    }
    if (appRoute.kind === 'stardew-install') {
      document.title = '安装星露谷 · Anxi Game Panel'
      return
    }
    document.title = forceCompactShell ? 'Stardew Anxi Panel · 手机端' : 'Stardew Anxi Panel'
  }, [appRoute.kind, forceCompactShell, view])

  function navigate(path: string, replace = false) {
    const url = new URL(path, window.location.origin)
    if (!url.pathname.startsWith('/instances/')) setDesktopShellRequested(false)
    if (replace) window.history.replaceState(null, '', `${url.pathname}${url.search}`)
    else window.history.pushState(null, '', `${url.pathname}${url.search}`)
    setBrowserLocation({ pathname: url.pathname, search: url.search })
  }

  function openAuthenticatedLanding(preserveRequestedRoute: boolean) {
    const isProtectedRoute = browserLocation.pathname === '/games'
      || browserLocation.pathname.startsWith('/games/')
      || browserLocation.pathname.startsWith('/instances/')
    if (!preserveRequestedRoute || !isProtectedRoute) navigate('/games', true)
    setView('authenticated')
  }

  async function boot() {
    setMessage('')
    try {
      const status = await request<SetupStatus>('/api/setup/status')
      const resolvedDefaultInstanceId = setDefaultInstanceId(status.defaultInstanceId)
      setConfiguredDefaultInstanceId(resolvedDefaultInstanceId)
      if (!status.initialized) {
        setView('setup')
        return
      }
      try {
        const me = await request<UserResponse>('/api/auth/me')
        setCurrentUser(me.user)
        openAuthenticatedLanding(true)
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
      openAuthenticatedLanding(false)
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
      openAuthenticatedLanding(true)
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
      setDesktopShellRequested(false)
      setLoginForm({ ...emptyLoginForm })
      setView('login')
      setBusy(false)
    }
  }

  if (view === 'authenticated' && currentUser) {
    const routedInstanceId = appRoute.kind === 'stardew-instance' || appRoute.kind === 'stardew-install'
      ? appRoute.instanceId
      : configuredDefaultInstanceId
    setDefaultInstanceId(routedInstanceId)

    return (
      <PanelUpdateProvider user={currentUser}>
        {appRoute.kind === 'games' || appRoute.kind === 'stardew-worlds' || appRoute.kind === 'stardew-new-world' || appRoute.kind === 'stardew-install' ? (
          <GamesPage
            user={currentUser}
            defaultInstanceId={configuredDefaultInstanceId}
            worldsOpen={appRoute.kind === 'stardew-worlds' || appRoute.kind === 'stardew-new-world'}
            createWorldOpen={appRoute.kind === 'stardew-new-world'}
            installOpen={appRoute.kind === 'stardew-install'}
            installTargetId={appRoute.kind === 'stardew-install' ? appRoute.instanceId : undefined}
            requestedInstallJobId={appRoute.kind === 'stardew-install'
              ? new URLSearchParams(browserLocation.search).get('jobId') ?? undefined
              : undefined}
            onNavigate={navigate}
            onLogout={logout}
          />
        ) : usesCompactShell && !desktopShellRequested ? (
          <StardewMobileShell
            key={appRoute.instanceId}
            user={currentUser}
            instanceId={appRoute.instanceId}
            onLogout={logout}
            onUseDesktop={() => setDesktopShellRequested(true)}
            onBackToWorlds={() => navigate('/games/stardew')}
          />
        ) : (
          <StardewPanel
            key={appRoute.instanceId}
            user={currentUser}
            instanceId={appRoute.instanceId}
            onLogout={logout}
            onUseCompact={usesCompactShell && desktopShellRequested ? () => setDesktopShellRequested(false) : undefined}
            onBackToWorlds={() => navigate('/games/stardew')}
          />
        )}
      </PanelUpdateProvider>
    )
  }

  const authShellClass = [
    'sd-auth-shell',
    view === 'login' || view === 'setup' ? 'sd-auth-shell--image-login' : '',
    view === 'login' ? 'sd-auth-shell--login' : '',
    view === 'setup' ? 'sd-auth-shell--setup' : '',
    message ? 'sd-auth-shell--has-message' : '',
  ].filter(Boolean).join(' ')

  return (
    <main className={authShellClass}>
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
