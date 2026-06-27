import { ApiError } from '../api'
import type { JobLog, JobStatus } from '../types'

export function errorMessage(error: unknown): string {
  if (error instanceof ApiError) return error.message
  if (error instanceof Error) return error.message
  return '请求失败，请稍后重试。'
}

export function formatDate(value: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

export function shortJobID(id: string): string {
  return id.length > 14 ? `${id.slice(0, 10)}…` : id
}

export function statusClass(status: string): string {
  if (status === 'succeeded' || status === 'running' || status === 'game_installed' || status === 'ready_to_start') {
    return 'succeeded'
  }
  if (status === 'failed' || status === 'error' || status === 'steam_auth_failed') return 'failed'
  if (status === 'canceled') return 'canceled'
  if (status === 'steam_auth_running' || status === 'installing') return 'running'
  return 'queued'
}

export function isTerminalJobStatus(status: JobStatus): boolean {
  return status === 'succeeded' || status === 'failed' || status === 'canceled'
}

export function appendUniqueLog(current: JobLog[], next: JobLog): JobLog[] {
  if (current.some((l) => l.jobId === next.jobId && l.sequence === next.sequence)) return current
  return [...current, next]
}

export function stateLabel(state: string): string {
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

export function roundPercent(value: number): number {
  return Math.min(100, Math.max(0, Math.round(value * 10) / 10))
}

export function formatPercent(value: number): string {
  const rounded = roundPercent(value)
  return `${Number.isInteger(rounded) ? rounded.toFixed(0) : rounded.toFixed(1)}%`
}
