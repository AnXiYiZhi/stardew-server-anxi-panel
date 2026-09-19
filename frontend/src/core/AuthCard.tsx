import { createContext, useContext } from 'react'
import type { ReactNode } from 'react'
import type { VersionInfo } from '../api'
import { useMediaQuery } from '../hooks/useMediaQuery'
import './AuthCard.css'

export const COMPACT_AUTH_MEDIA_QUERY = '(max-width: 1106px), (max-aspect-ratio: 8 / 5), (min-aspect-ratio: 5 / 2), (max-width: 1366px) and (hover: none) and (pointer: coarse)'
const CompactAuthContext = createContext(false)

export function useCompactAuthCard() {
  return useContext(CompactAuthContext)
}

type AuthCardProps = {
  booting: boolean
  setup?: boolean
  message: string
  versionInfo: VersionInfo | null
  children: ReactNode
}

export function AuthCard({ booting, setup = false, message, versionInfo, children }: AuthCardProps) {
  const compactViewport = useMediaQuery(COMPACT_AUTH_MEDIA_QUERY)
  const compact = compactViewport || booting
  const shellClass = [
    'sd-auth-shell',
    compact ? 'sd-auth-shell--compact' : 'sd-auth-shell--image-login',
    booting ? 'sd-auth-shell--booting' : setup ? 'sd-auth-shell--setup' : 'sd-auth-shell--login',
    message ? 'sd-auth-shell--has-message' : '',
  ].filter(Boolean).join(' ')
  return (
    <CompactAuthContext.Provider value={compact}>
      <main className={shellClass}>
        <section className="sd-auth-card" aria-labelledby="auth-title">
          {compact ? <img className="sd-auth-mascot" src="/assets/stardew/ui/topbar/icon_topbar_chicken_image2_v2.png" alt="" /> : null}
          {compact && versionInfo ? <span className="sd-auth-version">v{versionInfo.version}</span> : null}
          <header className="sd-auth-heading">
            <p className="sd-auth-eyebrow">Stardew Valley 管理面板</p>
            <h1 id="auth-title" className="sd-auth-title">Stardew Anxi Panel</h1>
          </header>
          {compact ? <div className="sd-auth-divider" aria-hidden="true"><span /></div> : null}
          {!compact && versionInfo ? (
            <p className="sd-auth-version">
              v{versionInfo.version}
              {versionInfo.commit ? ` · ${versionInfo.commit}` : ''}
              {versionInfo.buildDate ? ` · ${versionInfo.buildDate}` : ''}
            </p>
          ) : null}
          {message ? <div className="sd-auth-error" role="alert">{message}</div> : null}
          {booting ? <p className="sd-auth-loading" role="status">正在读取面板状态……</p> : children}
        </section>
      </main>
    </CompactAuthContext.Provider>
  )
}
