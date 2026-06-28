import { useState } from 'react'
import { getHealthDiagnostics, downloadSupportBundle } from '../../../api'
import type { HealthCheck } from '../../../api'
import { errorMessage } from '../../../core/helpers'
import type { StardewPageProps } from '../stardew-routes'

// ── 检查项名称中文映射 ─────────────────────────────────────────────────────────

const CHECK_NAME_LABELS: Record<string, string> = {
  docker_daemon: 'Docker 服务',
  docker_compose: 'Docker Compose',
  data_dir: '数据目录',
  instance_dir: '实例目录',
  compose_file: 'Compose 文件',
  active_save: '启动存档',
}

function checkNameLabel(name: string): string {
  return CHECK_NAME_LABELS[name] ?? name
}

// ── 状态点 ────────────────────────────────────────────────────────────────────

function StatusDot({ status }: { status: 'ok' | 'warning' | 'error' }) {
  const cls =
    status === 'ok'
      ? 'sd-dot sd-dot-green'
      : status === 'warning'
        ? 'sd-dot sd-dot-yellow'
        : 'sd-dot sd-dot-red'
  return <span className={cls} aria-hidden="true" />
}

// ── CheckRow ──────────────────────────────────────────────────────────────────

function CheckRow({ check }: { check: HealthCheck }) {
  return (
    <div className={`sd-diag-check-row sd-diag-check-${check.status}`}>
      <StatusDot status={check.status} />
      <span className="sd-diag-check-name">{checkNameLabel(check.name)}</span>
      <span className="sd-diag-check-msg">{check.message}</span>
    </div>
  )
}

// ── DiagnosticsPage ───────────────────────────────────────────────────────────

export function DiagnosticsPage({ user, dashboardData }: StardewPageProps) {
  const isAdmin = user.role === 'admin'

  // 本地状态：允许重新检查时独立维护 loading/error，不污染公共层
  const [localData, setLocalData] = useState(dashboardData.health)
  const [localError, setLocalError] = useState<string | null>(null)
  // 用户手动触发过一次刷新后，不再 fallback 到公共层的过期 healthError
  const [hasLocalAttempt, setHasLocalAttempt] = useState(false)
  const [refreshing, setRefreshing] = useState(false)

  // 导出状态
  const [exportBusy, setExportBusy] = useState(false)
  const [exportError, setExportError] = useState<string | null>(null)

  // 以 localData 为准（重新检查后更新），dashboardData.health 只作为初始值
  const data = localData ?? dashboardData.health
  // 手动刷新后以本次结果为准，不再读取公共层可能过期的错误
  const displayError = hasLocalAttempt ? localError : dashboardData.healthError

  const checks: HealthCheck[] = data?.checks ?? []
  const overallStatus = data?.status ?? null
  const alerts = checks.filter((c) => c.status !== 'ok')
  const okCount = checks.filter((c) => c.status === 'ok').length
  const warnCount = checks.filter((c) => c.status === 'warning').length
  const errorCount = checks.filter((c) => c.status === 'error').length

  async function handleRefresh() {
    setRefreshing(true)
    setLocalError(null)
    setHasLocalAttempt(true)
    try {
      const res = await getHealthDiagnostics()
      setLocalData(res)
      dashboardData.refreshHealth()
    } catch (e) {
      setLocalError(errorMessage(e))
    } finally {
      setRefreshing(false)
    }
  }

  async function handleExportBundle() {
    setExportBusy(true)
    setExportError(null)
    try {
      const { blob, filename } = await downloadSupportBundle()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
    } catch (e) {
      setExportError(errorMessage(e))
    } finally {
      setExportBusy(false)
    }
  }

  // ── render ──────────────────────────────────────────────────────────────────

  return (
    <div className="sd-page">
      {/* 页头 */}
      <div className="sd-diag-header">
        <div className="sd-diag-header-left">
          <img
            className="sd-page-icon"
            src="/assets/stardew/ui/icons/icon_nav_diagnostics.png"
            alt=""
          />
          <div>
            <h2 className="sd-page-title">诊断与健康检查</h2>
            <p className="sd-page-desc">Docker 服务、数据目录、实例状态、支持包导出</p>
          </div>
        </div>
        <div className="sd-diag-header-actions">
          <button
            className="sd-btn-tan"
            disabled={refreshing}
            onClick={handleRefresh}
            type="button"
          >
            {refreshing ? '检查中…' : '重新检查'}
          </button>
          <button
            className="sd-btn-green"
            disabled={exportBusy || !isAdmin}
            onClick={handleExportBundle}
            type="button"
            title={!isAdmin ? '仅管理员可导出诊断包' : '导出含系统信息、日志、状态的诊断包'}
          >
            {exportBusy ? '导出中…' : '导出诊断包'}
          </button>
        </div>
      </div>

      {/* 加载中状态 */}
      {dashboardData.loading && !data && (
        <div className="sd-diag-loading">正在加载健康检查数据…</div>
      )}

      {/* 错误条 */}
      {displayError && (
        <div className="sd-diag-error-banner">
          {displayError}
          <button
            className="sd-btn-tan"
            style={{ marginLeft: 8, padding: '1px 8px', fontSize: '10px' }}
            disabled={refreshing}
            onClick={handleRefresh}
            type="button"
          >
            重试
          </button>
        </div>
      )}

      {/* 导出错误 */}
      {exportError && (
        <div className="sd-diag-error-banner">{exportError}</div>
      )}

      {/* 总状态面板 */}
      {data && (
        <div className={`sd-diag-status-panel sd-diag-status-${overallStatus}`}>
          <div className="sd-diag-status-icon-wrap">
            {overallStatus === 'ok' && <span className="sd-diag-status-big-dot sd-dot-green" />}
            {overallStatus === 'warning' && <span className="sd-diag-status-big-dot sd-dot-yellow" />}
            {overallStatus === 'error' && <span className="sd-diag-status-big-dot sd-dot-red" />}
          </div>
          <div className="sd-diag-status-info">
            <div className="sd-diag-status-label">
              {overallStatus === 'ok' && '系统正常'}
              {overallStatus === 'warning' && '存在警告'}
              {overallStatus === 'error' && '存在错误'}
              {!overallStatus && '未知'}
            </div>
            <div className="sd-diag-status-counts">
              <span className="sd-diag-count sd-diag-count-ok">✓ {okCount} 正常</span>
              {warnCount > 0 && (
                <span className="sd-diag-count sd-diag-count-warn">⚠ {warnCount} 警告</span>
              )}
              {errorCount > 0 && (
                <span className="sd-diag-count sd-diag-count-error">✕ {errorCount} 错误</span>
              )}
            </div>
          </div>
        </div>
      )}

      {/* 检查项列表 */}
      {checks.length > 0 && (
        <>
          <div className="sd-diag-section-title">检查项明细</div>
          <div className="sd-diag-checks">
            {checks.map((c) => (
              <CheckRow key={c.name} check={c} />
            ))}
          </div>
        </>
      )}

      {/* 告警与建议 */}
      <div className="sd-diag-section-title" style={{ marginTop: 14 }}>
        告警与建议
      </div>
      {alerts.length === 0 ? (
        <div className="sd-diag-alert-empty">
          <span className="sd-dot sd-dot-green" aria-hidden="true" />
          {data ? '暂无告警，所有检查项均正常' : '暂无数据，请先点击「重新检查」'}
        </div>
      ) : (
        <div className="sd-diag-alerts">
          {alerts.map((c) => (
            <div key={c.name} className={`sd-diag-alert-row sd-diag-alert-${c.status}`}>
              <StatusDot status={c.status} />
              <div className="sd-diag-alert-content">
                <span className="sd-diag-alert-name">{checkNameLabel(c.name)}</span>
                <span className="sd-diag-alert-msg">{c.message}</span>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* 资源趋势（待接入） */}
      <div className="sd-diag-section-title" style={{ marginTop: 14 }}>
        资源趋势
        <span className="sd-diag-pending-badge">待接入</span>
      </div>
      <div className="sd-diag-resource-pending">
        <div className="sd-diag-resource-pending-desc">
          CPU / 内存 / 磁盘实时趋势图表尚无后端数据源，待接入后在此区域渲染。
        </div>
      </div>
    </div>
  )
}
