import { useCallback, useEffect, useState } from 'react'
import {
  getAuditLogs,
  getUsers,
  createUser,
  updateUserRole,
  updateUserPassword,
  disableUser,
  deleteUserHard,
  getInstanceVNCConfig,
  updateInstanceVNCPort,
} from '../../../api'
import type { AuditLogEntry } from '../../../api'
import type { PanelUser } from '../../../types'
import { errorMessage, formatDate } from '../../../core/helpers'
import type { StardewPageProps } from '../stardew-routes'

// ── 审计日志操作中文映射 ──────────────────────────────────────────────────────

const AUDIT_ACTION_LABELS: Record<string, string> = {
  admin_initialized: '初始化管理员',
  user_login: '用户登录',
  user_logout: '用户登出',
  user_created: '创建用户',
  user_updated: '修改用户',
  user_disabled: '禁用用户',
  user_deleted: '删除用户',
  user_password_changed: '修改密码',
  instance_started: '启动服务器',
  instance_stopped: '停止服务器',
  instance_restarted: '重启服务器',
  instance_installed: '安装游戏',
  instance_game_language_update: '修改服务器游戏语言',
  save_selected: '选择存档',
  save_deleted: '删除存档',
  mod_uploaded: '上传 Mod',
  mod_deleted: '删除 Mod',
  command_executed: '执行命令',
  support_bundle_exported: '导出诊断包',
  instance_vnc_port_update: '修改 VNC 端口',
}

function currentPanelPort(): string {
  if (typeof window === 'undefined') return '—'
  if (window.location.port) return window.location.port
  if (window.location.protocol === 'https:') return '443'
  if (window.location.protocol === 'http:') return '80'
  return '—'
}

function isValidPort(value: string): boolean {
  if (!/^\d+$/.test(value.trim())) return false
  const n = Number.parseInt(value, 10)
  return n >= 1 && n <= 65535
}

function auditActionLabel(action: string): string {
  return AUDIT_ACTION_LABELS[action] ?? action
}

// ── 目标类型中文映射 ──────────────────────────────────────────────────────────

const TARGET_TYPE_LABELS: Record<string, string> = {
  user: '用户',
  instance: '实例',
  save: '存档',
  mod: 'Mod',
  command: '命令',
  system: '系统',
}

function targetTypeLabel(t: string): string {
  return TARGET_TYPE_LABELS[t] ?? t
}

// ── 确认弹窗 ──────────────────────────────────────────────────────────────────

type ConfirmDialogProps = {
  title: string
  body: string
  confirmLabel?: string
  onConfirm: () => void
  onCancel: () => void
  danger?: boolean
}

function ConfirmDialog({ title, body, confirmLabel = '确认', onConfirm, onCancel, danger }: ConfirmDialogProps) {
  return (
    <div className="sd-confirm-overlay" role="dialog" aria-modal>
      <div className="sd-confirm-dialog">
        <h3>{title}</h3>
        <p>{body}</p>
        <div className="sd-confirm-actions">
          <button className="sd-btn-tan" onClick={onCancel}>取消</button>
          <button className="sd-btn-delete" onClick={onConfirm} aria-label={confirmLabel}>
            {danger ? '⚠ ' : ''}{confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}

// ── 版本信息区 ────────────────────────────────────────────────────────────────

function VersionSection({ versionInfo }: { versionInfo: StardewPageProps['dashboardData']['versionInfo'] }) {
  return (
    <section className="sd-settings-section sd-settings-version-section">
      <h3 className="sd-settings-section-title">面板版本</h3>
      <div className="sd-settings-version-body">
        <div className="sd-settings-info-grid">
          <div className="sd-settings-info-row">
            <span className="sd-settings-label">版本号</span>
            <span className="sd-settings-value sd-settings-mono">
              {versionInfo?.version ?? '—'}
            </span>
          </div>
          <div className="sd-settings-info-row">
            <span className="sd-settings-label">构建时间</span>
            <span className="sd-settings-value sd-settings-mono">
              {versionInfo?.buildDate ? formatDate(versionInfo.buildDate) : '—'}
            </span>
          </div>
          <div className="sd-settings-info-row">
            <span className="sd-settings-label">Commit</span>
            <span className="sd-settings-value sd-settings-mono">
              {versionInfo?.commit ?? '—'}
            </span>
          </div>
          <div className="sd-settings-info-row">
            <span className="sd-settings-label">运行模式</span>
            <span className="sd-tag sd-tag-blue">Single Game Mode</span>
          </div>
        </div>
        <div className="sd-settings-version-art" aria-hidden="true">
          <img src="/assets/stardew/ui/install/icon_install_step_box_image2.png" alt="" />
        </div>
      </div>
    </section>
  )
}

// ── 端口信息区 ────────────────────────────────────────────────────────────────

function PortSection({ isAdmin }: { isAdmin: boolean }) {
  const [vncPort, setVNCPort] = useState('')
  const [draftVNCPort, setDraftVNCPort] = useState('')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const panelPort = currentPanelPort()

  const loadVNCPort = useCallback(async () => {
    if (!isAdmin) return
    setLoading(true)
    setError(null)
    setMessage(null)
    try {
      const res = await getInstanceVNCConfig()
      setVNCPort(res.vncPort)
      setDraftVNCPort(res.vncPort)
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setLoading(false)
    }
  }, [isAdmin])

  useEffect(() => {
    void loadVNCPort()
  }, [loadVNCPort])

  async function handleSaveVNCPort() {
    const trimmed = draftVNCPort.trim()
    if (!isValidPort(trimmed)) {
      setError('VNC 端口必须是 1 到 65535 之间的数字')
      setMessage(null)
      return
    }
    setSaving(true)
    setError(null)
    setMessage(null)
    try {
      const res = await updateInstanceVNCPort(trimmed)
      setVNCPort(res.vncPort)
      setDraftVNCPort(res.vncPort)
      setMessage('VNC 端口已保存，重启服务器后生效。')
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setSaving(false)
    }
  }

  const trimmedDraft = draftVNCPort.trim()
  const saveDisabled = loading || saving || !trimmedDraft || trimmedDraft === vncPort

  return (
    <section className="sd-settings-section sd-settings-port-section">
      <h3 className="sd-settings-section-title">端口信息</h3>

      {error && <div className="sd-settings-error">{error}</div>}
      {message && <div className="sd-settings-success">{message}</div>}

      <div className="sd-settings-port-row">
        <label className="sd-settings-port-field">
          <span className="sd-settings-port-label">面板端口</span>
          <input className="sd-input sd-settings-port-input" value={panelPort} readOnly />
        </label>

        <label className="sd-settings-port-field">
          <span className="sd-settings-port-label">VNC 端口</span>
          {isAdmin ? (
            <input
              className="sd-input sd-settings-port-input"
              value={draftVNCPort}
              inputMode="numeric"
              pattern="[0-9]*"
              placeholder={loading ? '读取中…' : '5800'}
              onChange={e => {
                setDraftVNCPort(e.target.value)
                setMessage(null)
              }}
              disabled={loading || saving}
            />
          ) : (
            <input className="sd-input sd-settings-port-input" value="仅管理员" readOnly />
          )}
        </label>

        <div className="sd-settings-port-actions">
          <button className="sd-btn-green" onClick={() => void handleSaveVNCPort()} disabled={!isAdmin || saveDisabled}>
            {saving ? '保存中…' : '保存'}
          </button>
          <button className="sd-btn-tan" onClick={() => void loadVNCPort()} disabled={!isAdmin || loading || saving}>
            {loading ? '读取中…' : '刷新'}
          </button>
        </div>
      </div>

      {!isAdmin && (
        <span className="sd-settings-port-locked">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          仅管理员可查看和修改 VNC 端口。
        </span>
      )}

      <span className="sd-settings-port-desc">修改端口后需要重启面板服务才能生效。</span>
    </section>
  )
}

// ── 用户管理区 ────────────────────────────────────────────────────────────────

type UserManagementSectionProps = {
  currentUserId: number
  isAdmin: boolean
  isSuperAdmin: boolean
}

function UserManagementSection({ currentUserId, isAdmin, isSuperAdmin }: UserManagementSectionProps) {
  const [users, setUsers] = useState<PanelUser[]>([])
  const [loading, setLoading] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)

  // 创建用户表单
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [newUsername, setNewUsername] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [newRole, setNewRole] = useState<'user' | 'admin'>('user')
  const [createBusy, setCreateBusy] = useState(false)
  const [createError, setCreateError] = useState<string | null>(null)

  // 角色变更确认弹窗
  const [roleConfirm, setRoleConfirm] = useState<{ user: PanelUser; toRole: string } | null>(null)
  const [roleBusy, setRoleBusy] = useState(false)

  // 重置密码弹窗
  const [passwordTarget, setPasswordTarget] = useState<PanelUser | null>(null)
  const [passwordDraft, setPasswordDraft] = useState('')
  const [passwordBusy, setPasswordBusy] = useState(false)
  const [passwordDialogError, setPasswordDialogError] = useState<string | null>(null)
  const [passwordSelfChanged, setPasswordSelfChanged] = useState(false)

  // 删除/禁用确认弹窗
  const [deleteConfirm, setDeleteConfirm] = useState<{ user: PanelUser; hard: boolean } | null>(null)
  const [deleteBusy, setDeleteBusy] = useState(false)

  const [actionError, setActionError] = useState<string | null>(null)

  const loadUsers = useCallback(async () => {
    if (!isAdmin) return
    setLoading(true)
    setLoadError(null)
    try {
      const res = await getUsers()
      setUsers(res.users)
    } catch (e) {
      setLoadError(errorMessage(e))
    } finally {
      setLoading(false)
    }
  }, [isAdmin])

  useEffect(() => {
    void loadUsers()
  }, [loadUsers])

  useEffect(() => {
    if (!isSuperAdmin && newRole === 'admin') {
      setNewRole('user')
    }
  }, [isSuperAdmin, newRole])

  async function handleCreate() {
    if (!newUsername.trim() || !newPassword.trim()) return
    setCreateBusy(true)
    setCreateError(null)
    try {
      await createUser(newUsername.trim(), newPassword, newRole)
      setNewUsername('')
      setNewPassword('')
      setNewRole('user')
      setShowCreateForm(false)
      await loadUsers()
    } catch (e) {
      setCreateError(errorMessage(e))
    } finally {
      setCreateBusy(false)
    }
  }

  async function handleRoleChange() {
    if (!roleConfirm) return
    setRoleBusy(true)
    setActionError(null)
    try {
      await updateUserRole(roleConfirm.user.id, roleConfirm.toRole)
      setRoleConfirm(null)
      await loadUsers()
    } catch (e) {
      setActionError(errorMessage(e))
      setRoleConfirm(null)
    } finally {
      setRoleBusy(false)
    }
  }

  function openPasswordDialog(user: PanelUser) {
    setPasswordTarget(user)
    setPasswordDraft('')
    setPasswordDialogError(null)
    setPasswordSelfChanged(false)
  }

  async function handleChangePassword() {
    if (!passwordTarget) return
    if (passwordDraft.length < 6) {
      setPasswordDialogError('密码至少需要 6 位')
      return
    }
    setPasswordBusy(true)
    setPasswordDialogError(null)
    try {
      const isSelf = passwordTarget.id === currentUserId
      await updateUserPassword(passwordTarget.id, passwordDraft)
      setPasswordDraft('')
      if (isSelf) {
        // Changing your own password revokes the current session; keep the
        // dialog open with a notice, then reload to show the login screen.
        setPasswordSelfChanged(true)
        setTimeout(() => window.location.reload(), 1200)
        return
      }
      setPasswordTarget(null)
      await loadUsers()
    } catch (e) {
      setPasswordDialogError(errorMessage(e))
    } finally {
      setPasswordBusy(false)
    }
  }

  async function handleDelete() {
    if (!deleteConfirm) return
    setDeleteBusy(true)
    setActionError(null)
    try {
      if (deleteConfirm.hard) {
        await deleteUserHard(deleteConfirm.user.id)
      } else {
        await disableUser(deleteConfirm.user.id)
      }
      setDeleteConfirm(null)
      await loadUsers()
    } catch (e) {
      setActionError(errorMessage(e))
      setDeleteConfirm(null)
    } finally {
      setDeleteBusy(false)
    }
  }

  if (!isAdmin) {
    return (
      <section className="sd-settings-section sd-settings-users-section">
        <h3 className="sd-settings-section-title">用户管理</h3>
        <div className="sd-settings-locked">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          仅管理员可管理面板用户。
        </div>
      </section>
    )
  }

  return (
    <section className="sd-settings-section sd-settings-users-section">
      <h3 className="sd-settings-section-title">用户管理</h3>

      {actionError && (
        <div className="sd-settings-error">{actionError}</div>
      )}

      <div className="sd-settings-section-toolbar sd-actionbar">
        <button className="sd-btn-green" onClick={() => setShowCreateForm(v => !v)}>
          {showCreateForm ? '收起' : '+ 新建用户'}
        </button>
        <button className="sd-btn-tan" onClick={() => void loadUsers()} disabled={loading}>
          {loading ? '刷新中…' : '刷新'}
        </button>
      </div>

      {showCreateForm && (
        <div className="sd-settings-create-form">
          <div className="sd-settings-form-title">新建用户</div>
          {createError && <div className="sd-settings-error">{createError}</div>}
          <div className="sd-settings-form-row">
            <label className="sd-settings-form-label">用户名</label>
            <input
              className="sd-input"
              value={newUsername}
              onChange={e => setNewUsername(e.target.value)}
              placeholder="用户名"
              disabled={createBusy}
            />
          </div>
          <div className="sd-settings-form-row">
            <label className="sd-settings-form-label">密码</label>
            <input
              className="sd-input"
              type="password"
              value={newPassword}
              onChange={e => setNewPassword(e.target.value)}
              placeholder="至少 6 位"
              disabled={createBusy}
            />
          </div>
          <div className="sd-settings-form-row">
            <label className="sd-settings-form-label">角色</label>
            <select
              className="sd-input"
              value={newRole}
              onChange={e => setNewRole(e.target.value as 'user' | 'admin')}
              disabled={createBusy}
            >
              <option value="user">普通用户</option>
              {isSuperAdmin && <option value="admin">管理员</option>}
            </select>
          </div>
          <div className="sd-settings-form-actions">
            <button className="sd-btn-tan" onClick={() => setShowCreateForm(false)} disabled={createBusy}>
              取消
            </button>
            <button
              className="sd-btn-green"
              onClick={() => void handleCreate()}
              disabled={createBusy || !newUsername.trim() || !newPassword.trim()}
            >
              {createBusy ? '创建中…' : '创建'}
            </button>
          </div>
        </div>
      )}

      {loadError ? (
        <div className="sd-settings-error">
          {loadError}
          <button className="sd-btn-tan sd-btn--sm" style={{ marginLeft: 8 }} onClick={() => void loadUsers()}>重试</button>
        </div>
      ) : loading && users.length === 0 ? (
        <div className="sd-settings-hint">加载用户列表…</div>
      ) : users.length === 0 ? (
        <div className="sd-settings-hint">暂无用户数据。</div>
      ) : (
        <div className="sd-settings-user-list">
          {users.map(u => (
            <div key={u.id} className={`sd-settings-user-row${!u.isActive ? ' sd-settings-user-inactive' : ''}`}>
              {(() => {
                const isSelf = u.id === currentUserId
                const isAdminTarget = u.role === 'admin'
                const canManageTarget = isSelf ? false : (isSuperAdmin || !isAdminTarget)
                const manageTitle = isSelf
                  ? '不能管理自己'
                  : !canManageTarget
                    ? '只有第一个管理员可以管理管理员账号'
                    : undefined
                const canChangePassword = isSuperAdmin || isSelf || !isAdminTarget
                const passwordTitle = canChangePassword ? undefined : '只有第一个管理员可以修改其他管理员的密码'
                return (
                  <>
              <span className="sd-settings-user-name">
                {u.username}
                {isSelf && (
                  <span className="sd-tag sd-tag-blue" style={{ marginLeft: 6 }}>自己</span>
                )}
              </span>
              <span className={`sd-tag ${u.role === 'admin' ? 'sd-tag-green' : 'sd-tag-blue'}`}>
                {u.role === 'admin' ? '管理员' : '普通用户'}
              </span>
              {!u.isActive && <span className="sd-tag sd-tag-red">已禁用</span>}
              <span className="sd-settings-user-login">
                上次登录：{u.lastLoginAt ? formatDate(u.lastLoginAt) : '—'}
              </span>
              <div className="sd-settings-user-actions sd-rowactions">
                <button
                  className="sd-btn-tan sd-btn--sm"
                  disabled={roleBusy || deleteBusy || !canChangePassword}
                  title={passwordTitle}
                  onClick={() => openPasswordDialog(u)}
                >
                  重置密码
                </button>
                {isSuperAdmin && (
                  <button
                    className="sd-btn-tan sd-btn--sm"
                    disabled={roleBusy || deleteBusy || isSelf}
                    title={isSelf ? '不能修改自己的角色' : undefined}
                    onClick={() => setRoleConfirm({ user: u, toRole: u.role === 'admin' ? 'user' : 'admin' })}
                  >
                    {u.role === 'admin' ? '降为普通用户' : '升为管理员'}
                  </button>
                )}
                <button
                  className="sd-btn-delete sd-btn--sm"
                  disabled={roleBusy || deleteBusy || !canManageTarget}
                  title={manageTitle}
                  onClick={() => setDeleteConfirm({ user: u, hard: false })}
                >
                  禁用
                </button>
                <button
                  className="sd-btn-delete sd-btn--sm"
                  disabled={roleBusy || deleteBusy || !canManageTarget}
                  title={manageTitle ?? '永久删除用户（不可恢复）'}
                  onClick={() => setDeleteConfirm({ user: u, hard: true })}
                >
                  删除
                </button>
              </div>
                  </>
                )
              })()}
            </div>
          ))}
        </div>
      )}

      {roleConfirm && (
        <ConfirmDialog
          title="修改用户角色"
          body={`确认将 "${roleConfirm.user.username}" 的角色改为 "${roleConfirm.toRole === 'admin' ? '管理员' : '普通用户'}"？`}
          confirmLabel="确认修改"
          onConfirm={() => void handleRoleChange()}
          onCancel={() => setRoleConfirm(null)}
        />
      )}

      {passwordTarget && (
        <div className="sd-confirm-overlay" role="dialog" aria-modal="true">
          <div className="sd-confirm-dialog">
            <h3>重置密码</h3>
            {passwordSelfChanged ? (
              <p>密码已修改，当前会话已失效，即将跳转到登录页…</p>
            ) : (
              <>
                <p>为用户 "{passwordTarget.username}" 设置新密码：</p>
                <input
                  className="sd-input"
                  type="password"
                  value={passwordDraft}
                  placeholder="至少 6 位"
                  autoFocus
                  onChange={e => {
                    setPasswordDraft(e.target.value)
                    setPasswordDialogError(null)
                  }}
                  disabled={passwordBusy}
                />
                {passwordDialogError && <div className="sd-settings-error" style={{ marginTop: 8 }}>{passwordDialogError}</div>}
                <div className="sd-confirm-actions">
                  <button
                    className="sd-btn-tan"
                    onClick={() => { setPasswordTarget(null); setPasswordDraft(''); setPasswordDialogError(null) }}
                    disabled={passwordBusy}
                  >
                    取消
                  </button>
                  <button
                    className="sd-btn-green"
                    onClick={() => void handleChangePassword()}
                    disabled={passwordBusy || passwordDraft.length < 6}
                  >
                    {passwordBusy ? '保存中…' : '确认修改'}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      )}

      {deleteConfirm && (
        <ConfirmDialog
          title={deleteConfirm.hard ? '永久删除用户' : '禁用用户'}
          body={
            deleteConfirm.hard
              ? `确认永久删除用户 "${deleteConfirm.user.username}"？此操作不可恢复。`
              : `确认禁用用户 "${deleteConfirm.user.username}"？禁用后该用户将无法登录。`
          }
          confirmLabel={deleteConfirm.hard ? '永久删除' : '确认禁用'}
          onConfirm={() => void handleDelete()}
          onCancel={() => setDeleteConfirm(null)}
          danger
        />
      )}
    </section>
  )
}

// ── 审计日志区 ────────────────────────────────────────────────────────────────

const AUDIT_PAGE_SIZE = 7

function AuditLogsSection({ isAdmin }: { isAdmin: boolean }) {
  const [logs, setLogs] = useState<AuditLogEntry[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [hasLoaded, setHasLoaded] = useState(false)

  const loadLogs = useCallback(async (off: number) => {
    setLoading(true)
    setError(null)
    try {
      const res = await getAuditLogs(AUDIT_PAGE_SIZE, off)
      setLogs(res.logs)
      setTotal(res.total)
      setOffset(off)
      setHasLoaded(true)
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (isAdmin) void loadLogs(0)
  }, [isAdmin, loadLogs])

  if (!isAdmin) {
    return (
      <section className="sd-settings-section sd-settings-audit-section">
        <h3 className="sd-settings-section-title">审计日志</h3>
        <div className="sd-settings-locked">
          <span className="sd-dot sd-dot-yellow" aria-hidden="true" />
          仅管理员可查看审计日志。
        </div>
      </section>
    )
  }

  const totalPages = Math.max(1, Math.ceil(total / AUDIT_PAGE_SIZE))
  const currentPage = Math.floor(offset / AUDIT_PAGE_SIZE) + 1

  return (
    <section className="sd-settings-section sd-settings-audit-section">
      <h3 className="sd-settings-section-title">审计日志</h3>

      <div className="sd-settings-section-toolbar sd-actionbar">
        <span className="sd-settings-hint-inline">共 {total} 条记录</span>
        <button className="sd-btn-tan" onClick={() => void loadLogs(offset)} disabled={loading}>
          {loading ? '刷新中…' : '刷新'}
        </button>
      </div>

      {error ? (
        <div className="sd-settings-error">
          {error}
          <button className="sd-btn-tan sd-btn--sm" style={{ marginLeft: 8 }} onClick={() => void loadLogs(offset)}>重试</button>
        </div>
      ) : !hasLoaded || (loading && logs.length === 0) ? (
        <div className="sd-settings-hint">加载审计日志…</div>
      ) : logs.length === 0 ? (
        <div className="sd-settings-hint">暂无审计日志。</div>
      ) : (
        <>
          <div className="sd-settings-audit-table">
            <div className="sd-settings-audit-head">
              <span className="sd-settings-audit-col-time">时间</span>
              <span className="sd-settings-audit-col-actor">操作者</span>
              <span className="sd-settings-audit-col-action">动作</span>
              <span className="sd-settings-audit-col-target">目标</span>
              <span className="sd-settings-audit-col-ip">IP</span>
            </div>
            {logs.map(log => (
              <div key={log.id} className="sd-settings-audit-row">
                <span className="sd-settings-audit-col-time sd-settings-mono">
                  {formatDate(log.createdAt)}
                </span>
                <span className="sd-settings-audit-col-actor">
                  {log.actorName ?? '—'}
                </span>
                <span className="sd-settings-audit-col-action">
                  {auditActionLabel(log.action)}
                </span>
                <span className="sd-settings-audit-col-target">
                  {log.targetType ? targetTypeLabel(log.targetType) : '—'}
                  {log.targetId ? <span className="sd-settings-mono"> #{log.targetId}</span> : null}
                </span>
                <span className="sd-settings-audit-col-ip sd-settings-mono">
                  {log.ipAddress ?? '—'}
                </span>
              </div>
            ))}
          </div>

          {totalPages > 1 && (
            <div className="sd-settings-audit-pagination">
              <button
                className="sd-btn-tan"
                disabled={offset === 0 || loading}
                onClick={() => void loadLogs(Math.max(0, offset - AUDIT_PAGE_SIZE))}
              >
                上一页
              </button>
              <span className="sd-settings-hint-inline">第 {currentPage} / {totalPages} 页</span>
              <button
                className="sd-btn-tan"
                disabled={offset + AUDIT_PAGE_SIZE >= total || loading}
                onClick={() => void loadLogs(offset + AUDIT_PAGE_SIZE)}
              >
                下一页
              </button>
            </div>
          )}
        </>
      )}
    </section>
  )
}

// ── 安全概览区 ──────────────────────────────────────────────────────────────

function SecuritySummarySection() {
  const summaryItems = [
    { label: 'Session 认证', detail: '管理员会话已启用', state: '已启用', level: 'green' },
    { label: '密码哈希', detail: '使用强哈希（bcrypt）', state: '已启用', level: 'green' },
    { label: '操作审计', detail: '记录敏感操作日志', state: '已启用', level: 'green' },
    { label: 'Docker Socket', detail: '未挂载到容器内部', state: '安全', level: 'green' },
    { label: '日志脱敏', detail: '已屏蔽敏感信息输出', state: '已启用', level: 'green' },
  ]

  return (
    <section className="sd-settings-section sd-settings-security-summary-section">
      <h3 className="sd-settings-section-title">安全与权限</h3>
      <div className="sd-settings-security-summary-list">
        {summaryItems.map(item => (
          <div key={item.label} className="sd-settings-security-summary-item">
            <span className={`sd-dot sd-dot-${item.level}`} aria-hidden="true" />
            <span>{item.label}</span>
            <span>{item.detail}</span>
            <strong>{item.state}</strong>
          </div>
        ))}
      </div>
    </section>
  )
}

// ── 安全建议区 ──────────────────────────────────────────────────────────────

function SecuritySection() {
  const adviceItems = [
    {
      level: 'green',
      title: '启用两步验证（2FA）',
      desc: '建议为所有管理员账号启用 2FA，增强账号安全。',
      badge: '良好',
    },
    {
      level: 'yellow',
      title: '限制 Docker Socket 挂载',
      desc: 'Docker Socket 已暴露可能导致容器逃逸，建议禁止使用。',
      badge: '警告',
    },
    {
      level: 'blue',
      title: '日志脱敏检查',
      desc: '建议定期检查日志脱敏配置，避免敏感信息泄露。',
      badge: '提示',
    },
  ]

  return (
    <section className="sd-settings-section sd-settings-security-section">
      <h3 className="sd-settings-section-title">安全建议</h3>
      <div className="sd-settings-security-list">
        {adviceItems.map(item => (
          <div key={item.title} className={`sd-settings-security-item sd-settings-security-item-${item.level}`}>
            <span className={`sd-settings-security-icon sd-settings-security-icon-${item.level}`} aria-hidden="true" />
            <div>
              <div className="sd-settings-security-item-title">{item.title}</div>
              <div className="sd-settings-security-item-desc">{item.desc}</div>
            </div>
            <span className={`sd-settings-security-badge sd-settings-security-badge-${item.level}`}>
              {item.badge}
            </span>
          </div>
        ))}
      </div>
      <div className="sd-settings-security-actions">
        <button className="sd-btn-green" disabled title="安全设置后端待接入">前往安全设置</button>
      </div>
    </section>
  )
}

// ── 待接入设置区 ──────────────────────────────────────────────────────────────

function PendingSettingsSection() {
  const pendingItems = [
    { label: '界面主题', desc: '浅色 / 深色 / 跟随系统' },
    { label: '界面语言', desc: '中文 / English' },
    { label: '多游戏模式', desc: '启用总面板以管理多个游戏实例' },
    { label: '备份策略', desc: '自动备份频率、保留数量、备份路径' },
    { label: '通知设置', desc: '服务器崩溃、Mod 冲突、磁盘告警推送' },
    { label: '会话超时', desc: '自动登出时间（分钟）' },
  ]

  return (
    <section className="sd-settings-section sd-settings-pending-section">
      <h3 className="sd-settings-section-title">其他设置 <span className="sd-settings-pending-badge">后端待接入</span></h3>
      <div className="sd-settings-pending-list">
        {pendingItems.map(item => (
          <div key={item.label} className="sd-settings-pending-item">
            <div className="sd-settings-pending-item-left">
              <span className="sd-settings-pending-item-label">{item.label}</span>
              <span className="sd-settings-pending-item-desc">{item.desc}</span>
            </div>
            <button className="sd-btn-tan" disabled title="后端待接入">待接入</button>
          </div>
        ))}
      </div>
    </section>
  )
}

// ── SettingsPage ──────────────────────────────────────────────────────────────

export function SettingsPage({ user, dashboardData }: StardewPageProps) {
  const isAdmin = user.role === 'admin'

  return (
    <div className="sd-page sd-settings-page">
      <div className="sd-page-header">
        <img
          className="sd-page-icon"
          src="/assets/stardew/ui/icons/icon_nav_settings_gear_image2.png"
          alt=""
        />
        <div>
          <h2 className="sd-page-title">设置与审计</h2>
          <p className="sd-page-desc">账号管理、面板版本、用户权限与操作审计日志。</p>
        </div>
      </div>

      <div className="sd-settings-content-grid">
        <div className="sd-settings-stack">
          <VersionSection versionInfo={dashboardData.versionInfo} />
          <UserManagementSection currentUserId={user.id} isAdmin={isAdmin} isSuperAdmin={user.isSuperAdmin} />
          <PortSection isAdmin={isAdmin} />
          <PendingSettingsSection />
        </div>

        <div className="sd-settings-stack">
          <SecuritySummarySection />
          <AuditLogsSection isAdmin={isAdmin} />
          <SecuritySection />
        </div>
      </div>
    </div>
  )
}
import './SettingsPage.css'
