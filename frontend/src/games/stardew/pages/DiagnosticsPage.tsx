import { useEffect, useMemo, useState } from 'react'
import type { CSSProperties } from 'react'
import { getHealthDiagnostics, downloadSupportBundle, getInstanceMetrics } from '../../../api'
import type { HealthCheck } from '../../../api'
import { errorMessage } from '../../../core/helpers'
import type { StardewPageProps } from '../stardew-routes'
import type { ResourceMetricSample } from '../../../types'

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

function checkIconClass(name: string): string {
  return `sd-diag-check-icon-${name.replace(/[^a-z0-9_-]/gi, '-')}`
}

// ── CheckRow ──────────────────────────────────────────────────────────────────

function CheckRow({ check }: { check: HealthCheck }) {
  return (
    <div className={`sd-diag-check-row sd-diag-check-${check.status}`}>
      <span className={`sd-diag-check-icon ${checkIconClass(check.name)}`} aria-hidden="true" />
      <span className="sd-diag-check-name">{checkNameLabel(check.name)}</span>
      <span className="sd-diag-check-status">
        <StatusDot status={check.status} />
      </span>
      <span className="sd-diag-check-msg">{check.message}</span>
    </div>
  )
}

function CountCard({
  type,
  label,
  count,
}: {
  type: 'ok' | 'warn' | 'error'
  label: string
  count: number
}) {
  return (
    <div className={`sd-diag-count-card sd-diag-count-card-${type}`}>
      <span className="sd-diag-count-gem" aria-hidden="true" />
      <span className="sd-diag-count-card-label">{label}</span>
      <span className="sd-diag-count-card-value">{count}</span>
    </div>
  )
}

function formatGaugeNumber(value: number | null | undefined): string {
  if (value == null) return '—'
  return `${Math.round(value * 10) / 10}`
}

function formatBytes(value: number | undefined): string {
  if (value == null || value < 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = value
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }
  const digits = size >= 10 || unit === 0 ? 0 : 1
  return `${size.toFixed(digits)} ${units[unit]}`
}

function hasByteValue(value: number | undefined): value is number {
  return value != null && value >= 0
}

function formatTrendTime(timestamp: string): string {
  const date = new Date(timestamp)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

function GaugeCard({
  label,
  value,
  sub,
  color,
}: {
  label: string
  value: number | null | undefined
  sub: string
  color: string
}) {
  const percent = value == null ? 0 : Math.max(0, Math.min(100, value))
  const gaugeStyle = {
    '--sd-diag-gauge-color': color,
    '--sd-diag-gauge-angle': `${percent * 3.6}deg`,
  } as CSSProperties
  return (
    <div className="sd-diag-gauge-card">
      <div className="sd-diag-gauge-ring" style={gaugeStyle}>
        <div className="sd-diag-gauge-core">
          <span className="sd-diag-gauge-number">{formatGaugeNumber(value)}</span>
          {value != null ? <span className="sd-diag-gauge-unit">%</span> : null}
        </div>
      </div>
      <div className="sd-diag-gauge-meta">
        <span className="sd-diag-gauge-label">{label}</span>
        <span className="sd-diag-gauge-sub">{sub}</span>
      </div>
    </div>
  )
}

function ResourceTrendChart({ samples }: { samples: ResourceMetricSample[] }) {
  const width = 560
  const height = 176
  const padX = 28
  const padTop = 16
  const padBottom = 26
  const chartW = width - padX * 2
  const chartH = height - padTop - padBottom
  const series = [
    { key: 'cpu', label: 'CPU (%)', color: '#3f8f2c', get: (s: ResourceMetricSample) => s.cpuPercent },
    { key: 'memory', label: '内存 (%)', color: '#d87916', get: (s: ResourceMetricSample) => s.memoryPercent },
    { key: 'disk', label: '磁盘 (%)', color: '#1f68b5', get: (s: ResourceMetricSample) => s.diskPercent },
  ]
  const maxValue = samples.reduce((max, sample) => {
    return series.reduce((seriesMax, item) => {
      const value = item.get(sample)
      return value == null ? seriesMax : Math.max(seriesMax, value)
    }, max)
  }, 100)
  const yMax = Math.max(100, Math.ceil(maxValue / 25) * 25)
  const ticks = [0, 0.25, 0.5, 0.75, 1].map((ratio) => Math.round(yMax * ratio))
  const xLabels =
    samples.length < 2
      ? []
      : [0, 0.25, 0.5, 0.75, 1].map((ratio) => {
          const index = Math.round((samples.length - 1) * ratio)
          return {
            key: `${ratio}-${samples[index]?.timestamp ?? ''}`,
            x: padX + chartW * ratio,
            label: formatTrendTime(samples[index]?.timestamp ?? ''),
          }
        })

  function pointsFor(getValue: (s: ResourceMetricSample) => number | null): string {
    if (samples.length < 2) return ''
    return samples
      .map((sample, index) => {
        const value = getValue(sample)
        if (value == null) return null
        const x = padX + (chartW * index) / Math.max(1, samples.length - 1)
        const y = padTop + chartH - (chartH * Math.max(0, Math.min(yMax, value))) / yMax
        return `${x.toFixed(1)},${y.toFixed(1)}`
      })
      .filter(Boolean)
      .join(' ')
  }

  return (
    <div className="sd-diag-trend-card">
      <div className="sd-diag-trend-head">
        <span>实时趋势</span>
        <div className="sd-diag-trend-legend">
          {series.map((item) => (
            <span key={item.key}>
              <i style={{ background: item.color }} />
              {item.label}
            </span>
          ))}
        </div>
      </div>
      <svg className="sd-diag-trend-svg" viewBox={`0 0 ${width} ${height}`} role="img" aria-label="CPU、内存、磁盘趋势折线图">
        {ticks.map((tick) => {
          const y = padTop + chartH - (chartH * tick) / yMax
          return (
            <g key={tick}>
              <line x1={padX} y1={y} x2={width - padX} y2={y} className="sd-diag-grid-line" />
              <text x={8} y={y + 4} className="sd-diag-axis-label">{tick}</text>
            </g>
          )
        })}
        {xLabels.map((item) => (
          <g key={item.key}>
            <line x1={item.x} y1={padTop} x2={item.x} y2={padTop + chartH} className="sd-diag-grid-line sd-diag-grid-line-vertical" />
            {item.label && (
              <text x={item.x} y={height - 7} className="sd-diag-axis-label sd-diag-axis-label-x">
                {item.label}
              </text>
            )}
          </g>
        ))}
        {series.map((item) => {
          const points = pointsFor(item.get)
          if (!points) return null
          return (
            <polyline
              key={item.key}
              className="sd-diag-trend-line"
              points={points}
              stroke={item.color}
            />
          )
        })}
        {samples.length < 2 && (
          <text x={width / 2} y={height / 2} className="sd-diag-chart-empty">
            等待更多采样数据
          </text>
        )}
      </svg>
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
  const [metricSamples, setMetricSamples] = useState<ResourceMetricSample[]>([])
  const [metricError, setMetricError] = useState<string | null>(null)
  const [metricService, setMetricService] = useState('server')

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
  const latestMetric = metricSamples[metricSamples.length - 1]
  const overallText =
    overallStatus === 'ok'
      ? '系统正常'
      : overallStatus === 'warning'
        ? '存在警告'
        : overallStatus === 'error'
          ? '存在错误'
          : '状态未知'
  const overallMessage =
    overallStatus === 'ok'
      ? '所有关键服务运行良好'
      : overallStatus === 'warning'
        ? '部分检查项需要关注'
        : overallStatus === 'error'
          ? '发现需要立即处理的问题'
          : '点击重新检查获取最新状态'

  useEffect(() => {
    let alive = true
    let timer: number | undefined

    async function loadMetrics() {
      try {
        const res = await getInstanceMetrics()
        if (!alive) return
        setMetricError(null)
        setMetricService(res.service || 'server')
        setMetricSamples((prev) => [...prev, res.sample].slice(-24))
      } catch (e) {
        if (!alive) return
        setMetricError(errorMessage(e))
      } finally {
        if (alive) {
          timer = window.setTimeout(() => {
            void loadMetrics()
          }, 5000)
        }
      }
    }

    void loadMetrics()
    return () => {
      alive = false
      if (timer != null) {
        window.clearTimeout(timer)
      }
    }
  }, [])

  const metricSubtitles = useMemo(() => {
    const memory =
      hasByteValue(latestMetric?.memoryUsedBytes) && hasByteValue(latestMetric?.memoryLimitBytes)
        ? `${formatBytes(latestMetric.memoryUsedBytes)} / ${formatBytes(latestMetric.memoryLimitBytes)}`
        : latestMetric?.containerRunning
          ? '容器内存'
          : '启动后显示'
    const disk =
      hasByteValue(latestMetric?.diskUsedBytes) && hasByteValue(latestMetric?.diskTotalBytes)
        ? `${formatBytes(latestMetric.diskUsedBytes)} / ${formatBytes(latestMetric.diskTotalBytes)}`
        : '实例磁盘'
    return {
      cpu: latestMetric?.containerRunning ? metricService : '启动后显示',
      memory,
      disk,
    }
  }, [latestMetric, metricService])

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
    <div className="sd-page sd-diag-page">
      {/* 页头 */}
      <div className="sd-diag-header">
        <div className="sd-diag-header-left">
          <img className="sd-page-icon" src="/assets/stardew/ui/icons/icon_nav_diagnostics_monitor_image2.png" alt="" />
          <div>
            <h2 className="sd-page-title">诊断与健康检查</h2>
          </div>
        </div>
        <div className="sd-diag-header-actions">
          <button
            className="sd-btn-green"
            disabled={refreshing}
            onClick={handleRefresh}
            type="button"
          >
            {refreshing ? '检查中…' : '重新检查'}
          </button>
          <button
            className="sd-btn-blue"
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
        <div className={`sd-diag-status-panel sd-diag-status-${overallStatus ?? 'unknown'}`}>
          <div className="sd-diag-status-main">
            <div className="sd-diag-status-icon-wrap" aria-hidden="true">
              <span className="sd-diag-status-shield" />
            </div>
            <div className="sd-diag-status-info">
              <div className="sd-diag-status-label">{overallText}</div>
              <div className="sd-diag-status-subtitle">{overallMessage}</div>
            </div>
          </div>
          <div className="sd-diag-count-strip" aria-label="健康检查统计">
            <CountCard type="ok" label="正常" count={okCount} />
            <CountCard type="warn" label="警告" count={warnCount} />
            <CountCard type="error" label="错误" count={errorCount} />
          </div>
        </div>
      )}

      <div className="sd-diag-main-grid">
        <div className="sd-diag-primary">
          {/* 检查项列表 */}
          {checks.length > 0 && (
            <div className="sd-diag-check-panel">
              <div className="sd-diag-check-head" aria-hidden="true">
                <span>检查项</span>
                <span>状态</span>
                <span>信息</span>
              </div>
              <div className="sd-diag-checks">
                {checks.map((c) => (
                  <CheckRow key={c.name} check={c} />
                ))}
              </div>
            </div>
          )}
        </div>

        <div className="sd-diag-resource-wrap">
          {/* 资源趋势 */}
          <div className="sd-diag-resource-panel">
            <div className="sd-diag-resource-head">
              <div className="sd-diag-section-title">资源趋势</div>
              <span className={latestMetric?.containerRunning ? 'sd-diag-live-badge' : 'sd-diag-idle-badge'}>
                {latestMetric?.containerRunning ? '实时' : '待运行'}
              </span>
            </div>
            <div className="sd-diag-gauge-grid">
              <GaugeCard label="CPU" value={latestMetric?.cpuPercent} sub={metricSubtitles.cpu} color="#3f8f2c" />
              <GaugeCard label="内存" value={latestMetric?.memoryPercent} sub={metricSubtitles.memory} color="#d87916" />
              <GaugeCard label="磁盘" value={latestMetric?.diskPercent} sub={metricSubtitles.disk} color="#1f68b5" />
            </div>
            <ResourceTrendChart samples={metricSamples} />
            {(metricError || latestMetric?.message) && (
              <div className="sd-diag-resource-note">
                {metricError ?? latestMetric?.message}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* 告警与建议 */}
      <div className="sd-diag-advice-panel">
        <div className="sd-diag-section-title">告警与建议</div>
        {alerts.length === 0 ? (
          <>
            <div className="sd-diag-advice-row sd-diag-advice-good">
              <span className="sd-diag-advice-icon" aria-hidden="true" />
              <span className="sd-diag-advice-label">良好</span>
              <span className="sd-diag-advice-msg">
                {data ? '当前系统状态良好，暂无需要处理的问题。' : '暂无数据，请先点击「重新检查」。'}
              </span>
            </div>
            <div className="sd-diag-advice-row sd-diag-advice-tip">
              <span className="sd-diag-advice-icon" aria-hidden="true" />
              <span className="sd-diag-advice-label">建议</span>
              <span className="sd-diag-advice-msg">定期备份存档以防止数据丢失。</span>
            </div>
            <div className="sd-diag-advice-row sd-diag-advice-note">
              <span className="sd-diag-advice-icon" aria-hidden="true" />
              <span className="sd-diag-advice-label">提示</span>
              <span className="sd-diag-advice-msg">可在“设置”中配置资源使用告警阈值。</span>
            </div>
          </>
        ) : (
          <>
            {alerts.map((c) => (
              <div key={c.name} className={`sd-diag-advice-row sd-diag-advice-${c.status}`}>
                <span className="sd-diag-advice-icon" aria-hidden="true" />
                <span className="sd-diag-advice-label">{checkNameLabel(c.name)}</span>
                <span className="sd-diag-advice-msg">{c.message}</span>
              </div>
            ))}
            <div className="sd-diag-advice-row sd-diag-advice-tip">
              <span className="sd-diag-advice-icon" aria-hidden="true" />
              <span className="sd-diag-advice-label">建议</span>
              <span className="sd-diag-advice-msg">处理问题后点击“重新检查”刷新诊断结果。</span>
            </div>
          </>
        )}
      </div>
    </div>
  )
}
