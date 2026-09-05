import { normalizeInstanceId } from './instance-id.ts'
import type { StardewRoute } from './games/stardew/stardew-routes'

export type AppRoute =
  | { kind: 'games' }
  | { kind: 'stardew-worlds' }
  | { kind: 'stardew-new-world' }
  | { kind: 'stardew-install'; instanceId: string }
  | { kind: 'stardew-instance'; instanceId: string; route: StardewRoute }

const STARDEW_ROUTES = new Set<StardewRoute>([
  'install',
  'overview',
  'server',
  'saves',
  'jobs',
  'players',
  'player-mods',
  'mods',
  'diagnostics',
  'settings',
])

function stardewRoute(value: string | undefined): StardewRoute {
  return value && STARDEW_ROUTES.has(value as StardewRoute)
    ? value as StardewRoute
    : 'overview'
}

export function parseAppRoute(pathname: string, _search: string, defaultInstanceId: string): AppRoute {
  if (pathname === '/games/stardew/new') return { kind: 'stardew-new-world' }
  if (pathname === '/games/stardew/install') {
    return { kind: 'stardew-install', instanceId: normalizeInstanceId(defaultInstanceId) }
  }
  if (pathname === '/games/stardew') return { kind: 'stardew-worlds' }
  if (pathname === '/games' || pathname === '/' || pathname === '/index.html') return { kind: 'games' }

  const match = /^\/instances\/([^/]+)(?:\/([^/]+))?$/.exec(pathname)
  if (match) {
    let decoded = match[1]
    try {
      decoded = decodeURIComponent(match[1])
    } catch {
      // normalizeInstanceId below fails closed to the configured default.
    }
    const instanceId = normalizeInstanceId(decoded)
    const route = stardewRoute(match[2])
    if (route === 'install') return { kind: 'stardew-install', instanceId }
    return { kind: 'stardew-instance', instanceId, route }
  }

  return { kind: 'games' }
}

export function stardewInstallPath(): string {
  return '/games/stardew/install'
}
