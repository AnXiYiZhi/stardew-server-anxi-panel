import { useState, useEffect, useCallback } from 'react'
import {
  startInstance,
  stopInstance,
  restartInstance,
  getCommands,
  runCommand,
  sendSay,
} from '../../../api'
import { errorMessage, stateLabel, formatDate } from '../../../core/helpers'
import type { StardewPageProps } from '../stardew-routes'
import type { ConsoleCommandDef } from '../../../types'

export function ServerControlPage({ instanceState, dashboardData, onNavigate }: StardewPageProps) {
  // ── 生命周期操作状态 ──────────────────────────────────────────────────────
  const [actionBusy, setActionBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [confirmAction, setConfirmAction] = useState<'stop' | 'restart' | null>(null)

  // ── 邀请码 ────────────────────────────────────────────────────────────────
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState(false)

  // ── 控制台命令 ────────────────────────────────────────────────────────────
  const [commands, setCommands] = useState<ConsoleCommandDef[]>([])
  const [commandsLoading, setCommandsLoading] = useState(false)
  const [commandsError, setCommandsError] = useState<string | null>(null)
  const [selectedCommand, setSelectedCommand] = useState('')
  const [commandBusy, setCommandBusy] = useState(false)
  const [commandResult, setCommandResult] = useState<string | null>(null)
  const [commandError, setCommandError] = useState<string | null>(null)

  // ── 全服喊话 ──────────────────────────────────────────────────────────────
  const [sayMessage, setSayMessage] = useState('')
  const [sayBusy, setSayBusy] = useState(false)
  const [sayResult, setSayResult] = useState<string | null>(null)
  const [sayError, setSayError] = useState<string | null>(null)

  // ── 状态推导 ──────────────────────────────────────────────────────────────
  const state = instanceState?.state ?? null
  const isRunning = state === 'running'
  const isStarting = state === 'starting'
  const isStopped = state === 'stopped' || state === 'ready_to_start'
  const canStart = isStopped && !actionBusy
  const canStop = isRunning && !actionBusy
  const canRestart = isRunning && !actionBusy
  const stateLabelText = state
    ? stateLabel(state)
    : dashboardData.loading
      ? '读取中…'
      : '未知'

  const dotClass = isRunning
    ? 'sd-dot sd-dot-green sd-dot-pulse'
    : state === 'error'
      ? 'sd-dot sd-dot-red'
      : isStarting
        ? 'sd-dot sd-dot-yellow sd-dot-pulse'
        : 'sd-dot sd-dot-gray'

  // ── 命令列表：服务器运行时加载一次 ───────────────────────────────────────
  const loadCommands = useCallback(async () => {
    if (!isRunning) {
      setCommands([])
      setCommandsError(null)
      return
    }
    setCommandsLoading(true)
    setCommandsError(null)
    try {
      const res = await getCommands()
      setCommands(res.commands)
      if (res.commands.length > 0 && !selectedCommand) {
        setSelectedCommand(res.commands[0].id)
      }
    } catch (e) {
      setCommandsError(errorMessage(e))
    } finally {
      setCommandsLoading(false)
    }
  }, [isRunning, selectedCommand])

  useEffect(() => {
    void loadCommands()
  }, [loadCommands])

  // ── 生命周期操作 ──────────────────────────────────────────────────────────
  async function handleStart() {
    setActionBusy(true)
    setActionError(null)
    try {
      await startInstance()
      dashboardData.refreshInstanceState()
      dashboardData.refreshJobs()
      dashboardData.refreshInviteCode()
    } catch (e) {
      setActionError(errorMessage(e))
    } finally {
      setActionBusy(false)
    }
  }

  async function handleStop() {
    setActionBusy(true)
    setActionError(null)
    try {
      await stopInstance()
      dashboardData.refreshInstanceState()
      dashboardData.refreshInviteCode()
      dashboardData.refreshJobs()
    } catch (e) {
      setActionError(errorMessage(e))
    } finally {
      setActionBusy(false)
    }
  }

  async function handleRestart() {
    setActionBusy(true)
    setActionError(null)
    try {
      await restartInstance()
      dashboardData.refreshInstanceState()
      dashboardData.refreshInviteCode()
      dashboardData.refreshJobs()
    } catch (e) {
      setActionError(errorMessage(e))
    } finally {
      setActionBusy(false)
    }
  }

  // ── 邀请码复制 ────────────────────────────────────────────────────────────
  function handleCopy() {
    if (!dashboardData.inviteCode) return
    setCopyError(false)
    navigator.clipboard.writeText(dashboardData.inviteCode).then(
      () => {
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      },
      () => {
        // clipboard API 不可用（HTTP 环境或权限被拒），降级提示
        setCopyError(true)
        setTimeout(() => setCopyError(false), 3000)
      },
    )
  }

  // ── 执行控制台命令 ────────────────────────────────────────────────────────
  async function handleRunCommand() {
    if (!selectedCommand) return
    setCommandBusy(true)
    setCommandResult(null)
    setCommandError(null)
    try {
      const res = await runCommand(selectedCommand)
      setCommandResult(res.output?.trim() || '命令已执行（无输出）')
    } catch (e) {
      setCommandError(errorMessage(e))
    } finally {
      setCommandBusy(false)
    }
  }

  // ── 全服喊话 ──────────────────────────────────────────────────────────────
  async function handleSay() {
    if (!sayMessage.trim()) return
    setSayBusy(true)
    setSayResult(null)
    setSayError(null)
    try {
      const res = await sendSay(sayMessage.trim())
      setSayResult(res.output?.trim() || '消息已发送')
      setSayMessage('')
    } catch (e) {
      setSayError(errorMessage(e))
    } finally {
      setSayBusy(false)
    }
  }

  const selectedCommandDef = commands.find((c) => c.id === selectedCommand)

  return (
    <div className="sd-page">
      {/* ── 页面标题 ───────────────────────────────────────────────────────── */}
      <div className="sd-page-header">
        <img
          className="sd-page-icon"
          src="/assets/stardew/ui/icons/icon_nav_server_control.png"
          alt=""
        />
        <div>
          <h2 className="sd-page-title">服务器控制</h2>
          <p className="sd-page-desc">管理服务器生命周期，获取邀请码，发送公告，执行控制台命令。</p>
        </div>
      </div>

      {/* ── 服务器状态卡片 ─────────────────────────────────────────────────── */}
      <div className="sd-state-card">
        <div className="sd-state-row">
          <span className="sd-state-label">当前状态</span>
          <span className={dotClass} aria-hidden="true" />
          <span className="sd-state-value">{stateLabelText}</span>
          <span
            className={`sd-state-badge sd-state-badge-${state ?? 'unknown'}`}
            style={{ marginLeft: 6 }}
          >
            {(isRunning || isStarting) ? (
              <span className="sd-state-badge-dot" aria-hidden="true" />
            ) : null}
            {stateLabelText}
          </span>
        </div>

        {instanceState ? (
          <>
            {instanceState.name ? (
              <div className="sd-state-row">
                <span className="sd-state-label">实例名称</span>
                <span className="sd-state-value" style={{ fontWeight: 400 }}>{instanceState.name}</span>
              </div>
            ) : null}
            {instanceState.driverPhase && instanceState.driverPhase !== state ? (
              <div className="sd-state-row">
                <span className="sd-state-label">驱动阶段</span>
                <span className="sd-state-value" style={{ fontWeight: 400, color: '#5a3c1d' }}>
                  {instanceState.driverPhase}
                </span>
              </div>
            ) : null}
            {instanceState.stateMessage ? (
              <div className="sd-state-row">
                <span className="sd-state-label">状态消息</span>
                <span className="sd-state-value" style={{ fontWeight: 400, color: '#8a7060' }}>
                  {instanceState.stateMessage}
                </span>
              </div>
            ) : null}
            {instanceState.updatedAt ? (
              <div className="sd-state-row">
                <span className="sd-state-label">状态更新</span>
                <span className="sd-state-value" style={{ fontWeight: 400, color: '#8a7060' }}>
                  {formatDate(instanceState.updatedAt)}
                </span>
              </div>
            ) : null}
          </>
        ) : null}

        {dashboardData.saves?.activeSaveName ? (
          <div className="sd-state-row">
            <span className="sd-state-label">当前存档</span>
            <span className="sd-state-value">{dashboardData.saves.activeSaveName}</span>
          </div>
        ) : null}

        {dashboardData.versionInfo ? (
          <div className="sd-state-row">
            <span className="sd-state-label">面板版本</span>
            <span className="sd-state-value" style={{ fontWeight: 400, color: '#5a3c1d' }}>
              {dashboardData.versionInfo.version}
            </span>
          </div>
        ) : null}
      </div>

      {/* ── 生命周期控制 ───────────────────────────────────────────────────── */}
      <div className="sd-srv-section">
        <div className="sd-srv-section-title">生命周期控制</div>
        <div className="sd-ctrl-row">
          <button
            className="sd-btn-start"
            disabled={!canStart}
            onClick={() => void handleStart()}
            title={isRunning ? '服务器已运行' : isStarting ? '服务器启动中' : '启动服务器'}
          >
            <img
              src="/assets/stardew/ui/icons/icon_button_play.png"
              alt=""
              className="sd-btn-img"
              style={{ width: 12, height: 13 }}
            />
            {actionBusy && canStart ? '启动中…' : '启动'}
          </button>

          <button
            className="sd-btn-stop"
            disabled={!canStop}
            onClick={() => setConfirmAction('stop')}
            title={!isRunning ? '服务器未运行' : '停止服务器（需确认）'}
          >
            <img
              src="/assets/stardew/ui/icons/icon_button_stop.png"
              alt=""
              className="sd-btn-img"
              style={{ width: 11, height: 11 }}
            />
            停止
          </button>

          <button
            className="sd-btn-restart"
            disabled={!canRestart}
            onClick={() => setConfirmAction('restart')}
            title={!isRunning ? '服务器未运行' : '重启服务器（需确认）'}
          >
            <img
              src="/assets/stardew/ui/icons/icon_button_restart.png"
              alt=""
              className="sd-btn-img"
              style={{ width: 12, height: 12 }}
            />
            重启
          </button>

          {actionBusy ? (
            <span className="sd-srv-hint" style={{ marginLeft: 6 }}>
              <span className="sd-dot sd-dot-yellow sd-dot-pulse" aria-hidden="true" />
              操作进行中，请稍候…
            </span>
          ) : null}
        </div>

        {actionError ? (
          <div className="sd-ov-error" style={{ marginTop: 6 }}>{actionError}</div>
        ) : null}

        {isStarting ? (
          <div className="sd-srv-hint" style={{ marginTop: 4 }}>
            <span className="sd-dot sd-dot-yellow sd-dot-pulse" aria-hidden="true" />
            &nbsp;服务器正在启动，请等待完成后再操作。
          </div>
        ) : null}

        {state && !isRunning && !isStopped && !isStarting ? (
          <div className="sd-srv-hint" style={{ marginTop: 4 }}>
            当前状态（{stateLabelText}）下无法直接启动服务器，请先完成安装或选择存档。
          </div>
        ) : null}
      </div>

      {/* ── 邀请码 ─────────────────────────────────────────────────────────── */}
      <div className="sd-srv-section">
        <div className="sd-srv-section-title">
          邀请码
          <button
            className="sd-btn-tan"
            style={{ marginLeft: 8, fontSize: 9.5, height: 20, padding: '0 8px', minWidth: 40 }}
            onClick={() => dashboardData.refreshInviteCode()}
            disabled={!isRunning}
            title={isRunning ? '重新获取邀请码' : '服务器未运行时无法获取邀请码'}
          >
            刷新
          </button>
        </div>

        {dashboardData.inviteCode ? (
          <>
            <div className="sd-invite-box">
              <div className="sd-invite-code">{dashboardData.inviteCode}</div>
              <button className="sd-btn-copy" onClick={handleCopy}>
                {copied ? '✓' : '复制'}
              </button>
            </div>
            {copyError ? (
              <div className="sd-srv-hint" style={{ color: '#c02020', marginTop: 3 }}>
                复制失败（需 HTTPS 或 localhost），请手动选择文本复制。
              </div>
            ) : null}
          </>
        ) : !isRunning ? (
          <div className="sd-srv-empty">服务器未运行，邀请码不可用。启动服务器后点击"刷新"获取。</div>
        ) : dashboardData.loading ? (
          <div className="sd-srv-empty">读取邀请码中…</div>
        ) : dashboardData.inviteCodeError ? (
          <div className="sd-srv-empty" style={{ color: '#8a7060' }}>
            获取邀请码失败（服务器可能尚未完全启动），可稍后点击刷新重试。
          </div>
        ) : (
          <div className="sd-srv-empty">暂无邀请码，点击上方"刷新"或等待服务器完全启动。</div>
        )}
      </div>

      {/* ── 全服喊话 ───────────────────────────────────────────────────────── */}
      <div className="sd-srv-section">
        <div className="sd-srv-section-title">全服消息</div>
        {isRunning ? (
          <>
            <div className="sd-ctrl-row" style={{ flexWrap: 'nowrap', gap: 6 }}>
              <input
                className="sd-input"
                type="text"
                placeholder="向所有在线玩家发送消息…"
                value={sayMessage}
                onChange={(e) => setSayMessage(e.target.value)}
                disabled={sayBusy}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') void handleSay()
                }}
                style={{ flex: 1 }}
              />
              <button
                className="sd-btn-green"
                onClick={() => void handleSay()}
                disabled={sayBusy || !sayMessage.trim()}
              >
                {sayBusy ? '发送中…' : '发送'}
              </button>
            </div>
            {sayResult ? (
              <div className="sd-srv-result" style={{ marginTop: 5 }}>{sayResult}</div>
            ) : null}
            {sayError ? (
              <div className="sd-ov-error" style={{ marginTop: 4 }}>{sayError}</div>
            ) : null}
            <div className="sd-srv-hint" style={{ marginTop: 4 }}>
              通过 SMAPI say 命令发送全服公告。注：该命令当前版本可能返回"命令不支持"。
            </div>
          </>
        ) : (
          <div className="sd-srv-empty">服务器运行时可向在线玩家发送全服消息。</div>
        )}
      </div>

      {/* ── 控制台命令 ─────────────────────────────────────────────────────── */}
      <div className="sd-srv-section">
        <div className="sd-srv-section-title">控制台命令</div>
        {isRunning ? (
          commandsError ? (
            <div className="sd-srv-empty" style={{ color: '#c02020' }}>
              加载命令列表失败：{commandsError}
              <button
                className="sd-btn-tan"
                style={{ marginLeft: 8, fontSize: 9.5, height: 20, padding: '0 8px', minWidth: 40 }}
                onClick={() => void loadCommands()}
              >
                重试
              </button>
            </div>
          ) : commandsLoading ? (
            <div className="sd-srv-empty">正在加载可用命令列表…</div>
          ) : commands.length > 0 ? (
            <>
              <div className="sd-ctrl-row" style={{ gap: 6, flexWrap: 'nowrap' }}>
                <select
                  className="sd-input"
                  style={{ flex: 1 }}
                  value={selectedCommand}
                  onChange={(e) => {
                    setSelectedCommand(e.target.value)
                    setCommandResult(null)
                    setCommandError(null)
                  }}
                  disabled={commandBusy}
                >
                  {commands.map((cmd) => (
                    <option key={cmd.id} value={cmd.id}>
                      {cmd.name}{cmd.adminOnly ? ' (仅管理员)' : ''}
                    </option>
                  ))}
                </select>
                <button
                  className="sd-btn-green"
                  onClick={() => void handleRunCommand()}
                  disabled={commandBusy || !selectedCommand}
                >
                  {commandBusy ? '执行中…' : '执行'}
                </button>
              </div>
              {selectedCommandDef?.description ? (
                <div className="sd-srv-hint" style={{ marginTop: 4 }}>
                  {selectedCommandDef.description}
                </div>
              ) : null}
              {commandResult ? (
                <pre className="sd-srv-result sd-srv-result-pre">{commandResult}</pre>
              ) : null}
              {commandError ? (
                <div className="sd-ov-error" style={{ marginTop: 4 }}>{commandError}</div>
              ) : null}
            </>
          ) : (
            <div className="sd-srv-empty">服务器未返回可用命令，可能尚未完全就绪。</div>
          )
        ) : (
          <div className="sd-srv-empty">服务器运行时可执行 SMAPI 控制台命令（allowlist 限制）。</div>
        )}
      </div>

      {/* ── 快捷操作（待接入） ─────────────────────────────────────────────── */}
      <div className="sd-srv-section">
        <div className="sd-srv-section-title">快捷操作</div>
        <div className="sd-ctrl-row" style={{ flexWrap: 'wrap', gap: 6 }}>
          <button
            className="sd-btn-tan"
            disabled
            title="待接入：后端尚无手动世界存档 API"
          >
            保存世界
            <span className="sd-srv-badge-pending">待接入</span>
          </button>
          <button
            className="sd-btn-tan"
            disabled
            title="待接入：后端尚无手动备份触发 API"
          >
            备份存档
            <span className="sd-srv-badge-pending">待接入</span>
          </button>
          <button
            className="sd-btn-tan"
            disabled
            title="待接入：计划重启功能后续实现"
          >
            计划重启
            <span className="sd-srv-badge-pending">待接入</span>
          </button>
          <button
            className="sd-btn-tan"
            disabled
            title="待接入：端口/可见性/密码配置"
          >
            服务器设置
            <span className="sd-srv-badge-pending">待接入</span>
          </button>
        </div>
        <div className="sd-srv-hint" style={{ marginTop: 6 }}>
          快捷操作在后端 API 接入前保持禁用。存档管理请前往
          <button
            className="sd-inline-nav"
            style={{ marginLeft: 2 }}
            onClick={() => onNavigate('saves')}
          >
            存档页
          </button>。
        </div>
      </div>

      {/* ── 危险操作确认弹框 ───────────────────────────────────────────────── */}
      {confirmAction ? (
        <div className="sd-confirm-overlay">
          <div className="sd-confirm-dialog">
            <h3>{confirmAction === 'stop' ? '确认停止服务器' : '确认重启服务器'}</h3>
            <p>
              {confirmAction === 'stop'
                ? '停止服务器将断开所有在线玩家的连接，邀请码将立即失效。此操作不可撤销，确认继续？'
                : '重启服务器将短暂中断所有玩家的连接。重启完成后服务器会自动恢复，确认继续？'}
            </p>
            <div className="sd-confirm-actions">
              <button className="sd-btn-tan" onClick={() => setConfirmAction(null)}>
                取消
              </button>
              <button
                className={confirmAction === 'stop' ? 'sd-btn-stop' : 'sd-btn-restart'}
                onClick={() => {
                  const action = confirmAction
                  setConfirmAction(null)
                  void (action === 'stop' ? handleStop() : handleRestart())
                }}
              >
                确认{confirmAction === 'stop' ? '停止' : '重启'}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
