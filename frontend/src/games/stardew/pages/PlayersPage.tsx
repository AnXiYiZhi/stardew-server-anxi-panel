import { useState } from 'react'
import { runCommand } from '../../../api'
import { errorMessage, stateLabel, formatDate } from '../../../core/helpers'
import type { StardewPageProps } from '../stardew-routes'

export function PlayersPage({ user, instanceState, dashboardData }: StardewPageProps) {
  const [serverInfo, setServerInfo] = useState<string | null>(null)
  const [serverInfoLoading, setServerInfoLoading] = useState(false)
  const [serverInfoError, setServerInfoError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [copyError, setCopyError] = useState(false)

  const isAdmin = user.role === 'admin'
  const state = instanceState?.state ?? null
  const isRunning = state === 'running'
  const isStarting = state === 'starting'

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

  // Active save info
  const activeSaveName = dashboardData.saves?.activeSaveName ?? null
  const activeSave = dashboardData.saves?.saves.find(
    (s) => s.isActive || s.name === activeSaveName,
  ) ?? null

  async function fetchServerInfo() {
    if (!isRunning) return
    setServerInfoLoading(true)
    setServerInfoError(null)
    try {
      const result = await runCommand('info')
      setServerInfo(result.output || result.error || '（无输出）')
    } catch (e) {
      setServerInfoError(errorMessage(e))
    } finally {
      setServerInfoLoading(false)
    }
  }

  function handleCopyInvite() {
    const code = dashboardData.inviteCode
    if (!code) return
    setCopyError(false)
    navigator.clipboard.writeText(code).then(
      () => {
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      },
      () => {
        setCopyError(true)
        setTimeout(() => setCopyError(false), 3000)
      },
    )
  }

  // ── Season / date helper ───────────────────────────────────────────────────
  const SEASON_ZH: Record<string, string> = {
    spring: '春',
    summer: '夏',
    fall: '秋',
    winter: '冬',
  }
  function saveDate(save: NonNullable<typeof activeSave>): string {
    if (!save.gameYear) return '—'
    const season = SEASON_ZH[save.gameSeason?.toLowerCase() ?? ''] ?? save.gameSeason ?? '?'
    return `第 ${save.gameYear} 年${season}季第 ${save.gameDay ?? '?'} 天`
  }

  return (
    <div className="sd-page">
      {/* ── 页头 ──────────────────────────────────────────────────────────── */}
      <div className="sd-page-header">
        <img
          className="sd-page-icon"
          src="/assets/stardew/ui/icons/icon_nav_players.png"
          alt=""
        />
        <div>
          <h2 className="sd-page-title">玩家管理</h2>
          <p className="sd-page-desc">
            查看在线玩家、邀请码和服务器信息；踢出 / 白名单等管理功能待后端接入。
          </p>
        </div>
      </div>

      {/* ── 区域 1：玩家概览 ──────────────────────────────────────────────── */}
      <div className="sd-srv-section">
        <div className="sd-srv-section-title">玩家概览</div>

        <div className="sd-players-overview-grid">
          {/* 服务器状态 */}
          <div className="sd-players-stat">
            <span className="sd-players-stat-label">服务器状态</span>
            <span className="sd-players-stat-value">
              <span className={dotClass} aria-hidden="true" />
              {stateLabelText}
            </span>
          </div>

          {/* 在线人数 — 待接入 */}
          <div className="sd-players-stat">
            <span className="sd-players-stat-label">在线人数</span>
            <span className="sd-players-stat-value sd-players-stat-pending">
              {isRunning ? '—' : '—'}
              <span className="sd-srv-badge-pending">待接入</span>
            </span>
          </div>

          {/* 最大人数 — 待接入 */}
          <div className="sd-players-stat">
            <span className="sd-players-stat-label">最大人数</span>
            <span className="sd-players-stat-value sd-players-stat-pending">
              —
              <span className="sd-srv-badge-pending">待接入</span>
            </span>
          </div>

          {/* 活跃存档 / 农场 */}
          <div className="sd-players-stat">
            <span className="sd-players-stat-label">当前农场</span>
            <span className="sd-players-stat-value">
              {activeSave
                ? activeSave.farmName
                  ? activeSave.farmName
                  : activeSave.name
                : activeSaveName ?? '—'}
            </span>
          </div>

          {/* 农民名 */}
          {activeSave?.farmerName && (
            <div className="sd-players-stat">
              <span className="sd-players-stat-label">主机农民</span>
              <span className="sd-players-stat-value">{activeSave.farmerName}</span>
            </div>
          )}

          {/* 游戏内时间 */}
          {activeSave?.gameYear && (
            <div className="sd-players-stat">
              <span className="sd-players-stat-label">游戏日期</span>
              <span className="sd-players-stat-value">{saveDate(activeSave)}</span>
            </div>
          )}
        </div>

        {/* 邀请码 */}
        <div className="sd-players-invite-row">
          <span className="sd-players-invite-label">邀请码</span>
          {isRunning ? (
            dashboardData.inviteCode ? (
              <span className="sd-players-invite-code">{dashboardData.inviteCode}</span>
            ) : dashboardData.inviteCodeError ? (
              <span className="sd-players-invite-error">获取失败</span>
            ) : (
              <span className="sd-players-invite-loading">获取中…</span>
            )
          ) : (
            <span className="sd-players-invite-empty">服务器未运行</span>
          )}
          {isRunning && dashboardData.inviteCode && (
            <button
              className="sd-btn-tan sd-btn-xs"
              onClick={handleCopyInvite}
              disabled={!dashboardData.inviteCode}
              title="复制邀请码"
            >
              {copied ? '已复制 ✓' : '复制'}
            </button>
          )}
          {isRunning && (
            <button
              className="sd-btn-tan sd-btn-xs"
              onClick={() => { void dashboardData.refreshInviteCode() }}
              title="刷新邀请码"
            >
              刷新
            </button>
          )}
        </div>
        {copyError && (
          <div className="sd-srv-hint" style={{ color: '#b94040', marginTop: 4 }}>
            复制失败，请手动选取邀请码文字。
          </div>
        )}
      </div>

      {/* ── 区域 2：服务器信息（via info 命令）──────────────────────────── */}
      <div className="sd-srv-section">
        <div className="sd-srv-section-title">
          服务器信息
          {isRunning && (
            <button
              className="sd-btn-tan sd-btn-xs"
              style={{ marginLeft: 8 }}
              onClick={() => { void fetchServerInfo() }}
              disabled={serverInfoLoading}
            >
              {serverInfoLoading ? '获取中…' : '刷新'}
            </button>
          )}
        </div>

        {!isRunning && !isStarting ? (
          <div className="sd-srv-empty">服务器未运行，暂无服务器信息。</div>
        ) : serverInfoLoading ? (
          <div className="sd-srv-empty">正在获取服务器信息…</div>
        ) : serverInfoError ? (
          <div className="sd-players-info-error">
            获取服务器信息失败：{serverInfoError}
          </div>
        ) : serverInfo ? (
          <pre className="sd-players-info-terminal">{serverInfo}</pre>
        ) : (
          <div className="sd-srv-empty">
            {isRunning
              ? '服务器已运行，点击「刷新」获取服务器信息。'
              : '服务器启动中，请稍候…'}
          </div>
        )}

        <div className="sd-srv-hint" style={{ marginTop: 6 }}>
          <span>↑ 上方内容来自 JunimoServer </span>
          <code style={{ fontSize: 9 }}>info</code>
          <span> 命令的原始输出，包括玩家数、存档和状态信息。</span>
        </div>
      </div>

      {/* ── 区域 3：在线玩家列表 ─────────────────────────────────────────── */}
      <div className="sd-srv-section">
        <div className="sd-srv-section-title">
          在线玩家列表
          <span className="sd-srv-badge-pending">待接入</span>
        </div>

        {!isRunning && !isStarting ? (
          <div className="sd-players-empty">
            <img
              className="sd-players-empty-icon"
              src="/assets/stardew/ui/icons/icon_top_summary_players.png"
              alt=""
            />
            <div className="sd-players-empty-title">服务器未运行</div>
            <div className="sd-players-empty-desc">服务器停止时没有在线玩家。</div>
          </div>
        ) : (
          <div className="sd-players-empty">
            <img
              className="sd-players-empty-icon"
              src="/assets/stardew/ui/icons/icon_top_summary_players.png"
              alt=""
            />
            <div className="sd-players-empty-title">玩家列表接口待接入</div>
            <div className="sd-players-empty-desc">
              JunimoServer 暂未提供玩家列表 API。后续版本将接入在线玩家名、角色名、位置和在线时长。
            </div>
          </div>
        )}

        {/* 列表结构占位（真实接入后替换） */}
        <div className="sd-players-table-placeholder">
          <div className="sd-players-table-header">
            <span>玩家名</span>
            <span>角色</span>
            <span>位置</span>
            <span>在线时长</span>
            <span>状态</span>
            <span>操作</span>
          </div>
          <div className="sd-players-table-empty-row">
            暂无在线玩家数据（待接入）
          </div>
        </div>
      </div>

      {/* ── 区域 4：玩家活动历史 ─────────────────────────────────────────── */}
      <div className="sd-srv-section">
        <div className="sd-srv-section-title">
          玩家活动 / 最近事件
          <span className="sd-srv-badge-pending">待接入</span>
        </div>
        <div className="sd-players-empty sd-players-empty-small">
          <div className="sd-players-empty-title">事件历史待接入</div>
          <div className="sd-players-empty-desc">
            玩家加入、离开、断线等事件将在后端接入日志解析后显示。
          </div>
        </div>
      </div>

      {/* ── 区域 5：管理操作 ─────────────────────────────────────────────── */}
      <div className="sd-srv-section">
        <div className="sd-srv-section-title">
          管理操作
          {!isAdmin && (
            <span className="sd-srv-badge-pending" style={{ background: 'rgba(180,80,0,0.12)', color: '#7a3c00' }}>
              仅管理员
            </span>
          )}
        </div>

        {!isAdmin && (
          <div className="sd-srv-hint" style={{ marginBottom: 8 }}>
            管理操作仅管理员可用。
          </div>
        )}

        <div className="sd-players-actions-grid">
          {/* 踢出玩家 */}
          <div className="sd-players-action-item">
            <button
              className="sd-btn-tan"
              disabled
              title={!isAdmin ? '仅管理员可用' : '玩家列表 API 待接入'}
            >
              踢出玩家
            </button>
            <span className="sd-srv-badge-pending">待接入</span>
          </div>

          {/* 封禁玩家 */}
          <div className="sd-players-action-item">
            <button
              className="sd-btn-tan"
              disabled
              title={!isAdmin ? '仅管理员可用' : '封禁 API 待接入'}
            >
              封禁玩家
            </button>
            <span className="sd-srv-badge-pending">待接入</span>
          </div>

          {/* 白名单 */}
          <div className="sd-players-action-item">
            <button
              className="sd-btn-tan"
              disabled
              title={!isAdmin ? '仅管理员可用' : '白名单 API 待接入'}
            >
              白名单管理
            </button>
            <span className="sd-srv-badge-pending">待接入</span>
          </div>

          {/* 权限设置 */}
          <div className="sd-players-action-item">
            <button
              className="sd-btn-tan"
              disabled
              title={!isAdmin ? '仅管理员可用' : '权限设置 API 待接入'}
            >
              权限设置
            </button>
            <span className="sd-srv-badge-pending">待接入</span>
          </div>
        </div>

        <div className="sd-srv-hint" style={{ marginTop: 8 }}>
          上述操作需要 JunimoServer 提供对应 API 或日志解析支持后才能启用。
        </div>
      </div>

      {/* ── 版本信息（如有）────────────────────────────────────────────── */}
      {dashboardData.versionInfo?.version && (
        <div className="sd-srv-hint">
          面板版本：{dashboardData.versionInfo.version}
          {dashboardData.versionInfo.buildDate
            ? `  ·  构建：${formatDate(dashboardData.versionInfo.buildDate)}`
            : ''}
        </div>
      )}

    </div>
  )
}
