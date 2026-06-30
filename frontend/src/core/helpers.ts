import { ApiError } from '../api'
import type { JobLog, JobStatus } from '../types'

// Centralized error code → Chinese message mapping.
// Backend error codes are mapped to user-friendly Chinese messages.
// If a code is not in this map, the raw backend message is used.
const errorCodeMap: Record<string, string> = {
  // Auth
  unauthorized: '请先登录',
  forbidden: '权限不足',
  setup_required: '请先初始化管理员账号',
  invalid_credentials: '用户名或密码错误',
  super_admin_required: '只有第一个管理员可以创建管理员或调整管理员权限',
  last_super_admin: '不能移除或降级第一个管理员',
  // Instance
  instance_not_found: '实例不存在',
  driver_not_registered: '实例配置异常，请重新初始化',
  state_reconcile_failed: '实例状态校验失败',
  // Install/Prepare
  prepare_failed: '准备实例目录失败',
  install_failed: '安装任务启动失败',
  env_read_failed: '读取实例配置失败',
  // Lifecycle
  start_failed: '服务器启动失败',
  stop_failed: '服务器停止失败',
  restart_failed: '服务器重启失败',
  server_not_running: '服务器未运行',
  server_running: '服务器正在运行，无法执行此操作',
  // Saves
  list_saves_failed: '读取存档列表失败',
  save_not_found: '存档不存在',
  save_required: '没有可用存档，请先创建或上传存档',
  active_save_required: '没有已选择的启动存档，请先选择一个存档',
  active_save_missing: '上次选择的存档不存在，请重新选择存档',
  active_save_running: '当前启动存档正在被服务器使用，请先停止服务器再删除',
  select_failed: '选择存档失败',
  delete_failed: '删除存档失败',
  export_failed: '导出存档失败',
  import_failed: '导入存档失败',
  invalid_zip: '存档 ZIP 无效',
  invalid_config: '配置参数无效',
  invalid_body: '请求格式错误',
  invalid_field: '参数格式不正确',
  missing_field: '缺少必填参数',
  missing_file: '未找到上传文件',
  parse_form_failed: '解析上传表单失败（文件可能超过大小限制）',
  write_failed: '写入文件失败',
  token_invalid: '上传令牌无效或已过期',
  // Mods
  list_mods_failed: '读取 Mod 列表失败',
  invalid_mod_zip: 'Mod ZIP 无效',
  mod_exists: '已安装相同 ID 的 Mod',
  // Console
  command_failed: '执行命令失败',
  say_failed: '发送喊话失败',
  not_supported: '该功能暂不支持',
  command_not_supported: '该命令暂不支持',
  // Docker
  docker_command_failed: 'Docker 命令执行失败',
  docker_command_timeout: 'Docker 命令执行超时',
  compose_project_not_ready: 'Compose 项目尚未准备就绪',
  // Backup
  list_backups_failed: '读取备份列表失败',
  restore_failed: '恢复备份失败',
  save_exists: '存档已存在，请先删除已有存档或使用覆盖选项',
  delete_backup_failed: '删除备份失败',
  backup_not_found: '备份文件不存在',
  invalid_backup_name: '备份文件名不合法',
  // Jobs
  not_implemented: '功能暂未实现',
  // Generic
  internal_error: '服务器内部错误，请稍后重试',
}

export function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    // Try code-based translation first.
    if (error.code && errorCodeMap[error.code]) {
      return errorCodeMap[error.code]
    }
    return error.message
  }
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
