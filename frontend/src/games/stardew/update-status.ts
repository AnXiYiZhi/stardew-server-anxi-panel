import type { PanelUpdateStatus } from '../../api'

export type UpdateDisplayKind = 'available' | 'latest' | 'checking' | 'error' | 'unavailable'

export function withVersionPrefix(version: string | null | undefined): string {
  const value = version?.trim() ?? ''
  if (!value) return 'v--'
  return /^[vV]/.test(value) ? `v${value.slice(1)}` : `v${value}`
}

export function withoutVersionPrefix(version: string | null | undefined): string {
  return (version?.trim() ?? '').replace(/^[vV]/, '')
}

export function updateDisplayKind(status: PanelUpdateStatus | null): UpdateDisplayKind {
  if (!status) return 'checking'
  if (status.updateAvailable && status.latestVersion) return 'available'
  if (status.checkStatus === 'ok') return 'latest'
  if (status.checkStatus === 'checking' || status.checkStatus === 'pending') return 'checking'
  if (status.checkStatus === 'unavailable') return 'unavailable'
  return 'error'
}

export function updateSummaryText(status: PanelUpdateStatus | null): string {
  switch (updateDisplayKind(status)) {
    case 'available':
      return `发现新版本 ${withVersionPrefix(status?.latestVersion)}`
    case 'latest':
      return '✓ 最新'
    case 'checking':
      return '检查中…'
    case 'unavailable':
      return '开发版本'
    case 'error':
      return '检查失败'
  }
}
