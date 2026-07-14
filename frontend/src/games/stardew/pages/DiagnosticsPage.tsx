import { useEffect, useMemo, useState } from 'react'
import { getHealthDiagnostics, downloadSupportBundle, getInstanceMetrics, getComposePs, getJunimoUpdate, getJunimoUpdateDryRun, startJunimoUpdateDryRun, getJunimoUpdateApply, startJunimoUpdateApply, getRuntimeComponents, getRuntimeComponentsPreflight, startRuntimeComponentsPreflight, getSMAPIUpdate, getSMAPIUpdateDryRun, startSMAPIUpdateDryRun, getSMAPIUpdateApply, startSMAPIUpdateApply } from '../../../api'
import type { HealthCheck } from '../../../api'
import { errorMessage } from '../../../core/helpers'
import type { StardewPageProps } from '../stardew-routes'
import type { ComposeService, JunimoUpdateApplyStatus, JunimoUpdateDryRunStatus, JunimoUpdateInfo, ResourceMetricSample, RuntimeComponentsInfo, RuntimeComponentsPreflight, SMAPIUpdateInfo, SMAPIUpdateWorkflowStatus } from '../../../types'
import { junimoApplyActive, junimoApplyPhaseLabel, junimoDryRunActive, junimoDryRunPhaseLabel, junimoPairMatches, junimoUpdateStatusLabel } from '../junimo-update-status'
import { runtimeComponentsStatusLabel } from '../runtime-components-status'
import { shouldShowSMAPIUpdate, smapiPhaseActive, smapiPhaseLabel, smapiStatusLabel } from '../smapi-update-status'

const RESOURCE_METRICS_REFRESH_MS = 8000
const CONTROL_FRESH_MS = 30_000

function freshnessLabel(updatedAt?: string): string {
  if (!updatedAt) return '无数据'
  const age = Date.now() - new Date(updatedAt).getTime()
  if (!Number.isFinite(age)) return '时间无效'
  const seconds = Math.max(0, Math.round(age / 1000))
  return `${seconds} 秒前 · ${age <= CONTROL_FRESH_MS ? '新鲜' : '已过期'}`
}

function eventAgeLabel(updatedAt?: string): string {
  if (!updatedAt) return '无数据'
  const age = Date.now() - new Date(updatedAt).getTime()
  if (!Number.isFinite(age)) return '时间无效'
  return `${Math.max(0, Math.round(age / 1000))} 秒前 · 阶段事件`
}

function durationLabel(value?: number): string {
  if (value == null) return '暂无样本'
  return value >= 60_000 ? `${(value / 60_000).toFixed(1)} 分钟` : `${(value / 1000).toFixed(1)} 秒`
}

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

function compatibilityMatrixStatusLabel(status?: string): string {
  switch (status) {
    case 'recommended': return '正式推荐'
    case 'withdrawn': return '已撤回，禁止新安装和升级'
    default: return '矩阵状态未知'
  }
}

function recommendedImage(component?: { images?: string[]; image?: string }): string {
  return component?.images?.[0] || component?.image || '—'
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

// 圆环参照 Tomik23 circular-progress-bar 样式：
// 渐变描边（yellow -> #ff0000）+ 圆头端帽 + #e6e6e6 底环，SVG 实现无额外依赖
const GAUGE_RADIUS = 52
const GAUGE_CIRCUMFERENCE = 2 * Math.PI * GAUGE_RADIUS

function GaugeCard({
  label,
  value,
  sub,
  gradientId,
}: {
  label: string
  value: number | null | undefined
  sub: string
  gradientId: string
}) {
  const percent = value == null ? 0 : Math.max(0, Math.min(100, value))
  return (
    <div className="sd-diag-gauge-card">
      <span className="sd-diag-gauge-label">{label}</span>
      <div className="sd-diag-gauge-ring">
        <svg
          className="sd-diag-gauge-svg"
          viewBox="0 0 120 120"
          role="img"
          aria-label={value == null ? `${label} 暂无数据` : `${label} ${formatGaugeNumber(value)}%`}
        >
          <defs>
            <linearGradient id={gradientId} x1="0%" y1="0%" x2="100%" y2="100%">
              <stop offset="0%" stopColor="yellow" />
              <stop offset="100%" stopColor="#ff0000" />
            </linearGradient>
          </defs>
          <circle className="sd-diag-gauge-track" cx="60" cy="60" r={GAUGE_RADIUS} />
          {percent > 0 ? (
            <circle
              className="sd-diag-gauge-arc"
              cx="60"
              cy="60"
              r={GAUGE_RADIUS}
              stroke={`url(#${gradientId})`}
              strokeDasharray={GAUGE_CIRCUMFERENCE}
              strokeDashoffset={GAUGE_CIRCUMFERENCE * (1 - percent / 100)}
              transform="rotate(-90 60 60)"
            />
          ) : null}
        </svg>
        <div className="sd-diag-gauge-core">
          <span className="sd-diag-gauge-number">{formatGaugeNumber(value)}</span>
          {value != null ? <span className="sd-diag-gauge-unit">%</span> : null}
        </div>
      </div>
      <span className="sd-diag-gauge-sub">{sub}</span>
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
        <span>资源使用趋势（24小时）</span>
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

export function DiagnosticsPage({ user, dashboardData, instanceState }: StardewPageProps) {
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
  const [composeServices, setComposeServices] = useState<ComposeService[]>([])
  const [composeError, setComposeError] = useState<string | null>(null)
  const [junimoUpdate, setJunimoUpdate] = useState<JunimoUpdateInfo | null>(null)
  const [junimoUpdateError, setJunimoUpdateError] = useState<string | null>(null)
  const [junimoDryRun, setJunimoDryRun] = useState<JunimoUpdateDryRunStatus | null>(null)
  const [junimoDryRunError, setJunimoDryRunError] = useState<string | null>(null)
  const [junimoDryRunBusy, setJunimoDryRunBusy] = useState(false)
  const [junimoApply, setJunimoApply] = useState<JunimoUpdateApplyStatus | null>(null)
  const [junimoApplyError, setJunimoApplyError] = useState<string | null>(null)
  const [junimoApplyBusy, setJunimoApplyBusy] = useState(false)
  const [runtimeComponents, setRuntimeComponents] = useState<RuntimeComponentsInfo | null>(null)
  const [runtimeComponentsError, setRuntimeComponentsError] = useState<string | null>(null)
  const [runtimePreflight, setRuntimePreflight] = useState<RuntimeComponentsPreflight | null>(null)
  const [runtimePreflightBusy, setRuntimePreflightBusy] = useState(false)
  const [smapiUpdate, setSMAPIUpdate] = useState<SMAPIUpdateInfo | null>(null)
  const [smapiError, setSMAPIError] = useState<string | null>(null)
  const [smapiDryRun, setSMAPIDryRun] = useState<SMAPIUpdateWorkflowStatus | null>(null)
  const [smapiDryRunBusy, setSMAPIDryRunBusy] = useState(false)
  const [smapiApply, setSMAPIApply] = useState<SMAPIUpdateWorkflowStatus | null>(null)
  const [smapiApplyBusy, setSMAPIApplyBusy] = useState(false)
  const [maintenanceOpen, setMaintenanceOpen] = useState(false)
  const { applyHealthDiagnostics } = dashboardData

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
  const junimoUpdateAvailable = junimoUpdate?.available === true
    || instanceState?.runtimeDiagnostic?.junimoUpdateStatus === 'update_available'
  const gameUpdateAvailable = runtimeComponents?.recommended?.tested === true
    && runtimeComponents.status === 'update_available'
  const smapiUpdateAvailable = shouldShowSMAPIUpdate(smapiUpdate)
  const maintenanceCount = [junimoUpdateAvailable, gameUpdateAvailable, smapiUpdateAvailable].filter(Boolean).length

  function showMaintenanceDetail(anchor: string) {
    setMaintenanceOpen(true)
    window.requestAnimationFrame(() => {
      document.getElementById(anchor)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    })
  }

  useEffect(() => {
    let alive = true
    getComposePs().then((res) => { if (alive) setComposeServices(res.services ?? []) }).catch((e) => { if (alive) setComposeError(errorMessage(e)) })
    return () => { alive = false }
  }, [])

  useEffect(() => {
    if (!isAdmin) {
      setJunimoUpdate(null)
      setJunimoUpdateError(null)
      setJunimoDryRun(null)
      setJunimoDryRunError(null)
      setJunimoApply(null)
      setJunimoApplyError(null)
      setRuntimeComponents(null)
      setRuntimeComponentsError(null)
      setRuntimePreflight(null)
      setSMAPIUpdate(null)
      setSMAPIDryRun(null)
      setSMAPIApply(null)
      return
    }
    let alive = true
    getJunimoUpdate().then((result) => {
      if (!alive) return
      setJunimoUpdate(result)
      setJunimoUpdateError(null)
    }).catch((e) => {
      if (!alive) return
      setJunimoUpdateError(errorMessage(e))
    })
    getJunimoUpdateDryRun().then((result) => {
      if (alive) setJunimoDryRun(result)
    }).catch((e) => {
      if (alive) setJunimoDryRunError(errorMessage(e))
    })
    getJunimoUpdateApply().then((result) => { if (alive) setJunimoApply(result) }).catch((e) => { if (alive) setJunimoApplyError(errorMessage(e)) })
    getRuntimeComponents().then((result) => { if (alive) { setRuntimeComponents(result); if (result.smapi) setSMAPIUpdate(result.smapi); setRuntimeComponentsError(null) } }).catch((e) => { if (alive) setRuntimeComponentsError(errorMessage(e)) })
    getRuntimeComponentsPreflight().then((result) => { if (alive) setRuntimePreflight(result) }).catch(() => undefined)
    getSMAPIUpdate().then((result) => { if (alive) { setSMAPIUpdate(result); setSMAPIError(null) } }).catch((e) => { if (alive) setSMAPIError(errorMessage(e)) })
    getSMAPIUpdateDryRun().then((result) => { if (alive) setSMAPIDryRun(result) }).catch((e) => { if (alive) setSMAPIError(errorMessage(e)) })
    getSMAPIUpdateApply().then((result) => { if (alive) setSMAPIApply(result) }).catch((e) => { if (alive) setSMAPIError(errorMessage(e)) })
    return () => { alive = false }
  }, [isAdmin])

  useEffect(() => {
    if (!isAdmin || !junimoDryRunActive(junimoDryRun?.phase)) return
    const timer = window.setInterval(() => {
      getJunimoUpdateDryRun().then((result) => {
        setJunimoDryRun(result)
        setJunimoDryRunError(null)
      }).catch((e) => setJunimoDryRunError(errorMessage(e)))
    }, 1800)
    return () => window.clearInterval(timer)
  }, [isAdmin, junimoDryRun?.phase])

  useEffect(() => {
    if (!isAdmin || !junimoApplyActive(junimoApply?.phase)) return
    const timer = window.setInterval(() => {
      getJunimoUpdateApply().then((result) => { setJunimoApply(result); setJunimoApplyError(null) }).catch((e) => setJunimoApplyError(errorMessage(e)))
    }, 1800)
    return () => window.clearInterval(timer)
  }, [isAdmin, junimoApply?.phase])

  useEffect(() => {
    if (!isAdmin || !smapiPhaseActive(smapiApply?.phase)) return
    const timer = window.setInterval(() => {
      getSMAPIUpdateApply().then((result) => {
        setSMAPIApply(result)
        setSMAPIError(null)
        if (!smapiPhaseActive(result.phase)) void getSMAPIUpdate().then(setSMAPIUpdate)
      }).catch((e) => setSMAPIError(errorMessage(e)))
    }, 1800)
    return () => window.clearInterval(timer)
  }, [isAdmin, smapiApply?.phase])

  useEffect(() => {
    if (localData) return
    let alive = true

    async function loadInitialHealth() {
      setRefreshing(true)
      setLocalError(null)
      setHasLocalAttempt(true)
      try {
        const res = await getHealthDiagnostics()
        if (!alive) return
        setLocalData(res)
        applyHealthDiagnostics(res)
      } catch (e) {
        if (!alive) return
        setLocalError(errorMessage(e))
      } finally {
        if (alive) {
          setRefreshing(false)
        }
      }
    }

    void loadInitialHealth()
    return () => {
      alive = false
    }
  }, [applyHealthDiagnostics, localData])

  useEffect(() => {
    let alive = true
    let timer: number | undefined

    function clearTimer() {
      if (timer != null) {
        window.clearTimeout(timer)
        timer = undefined
      }
    }

    function scheduleNext() {
      if (!alive || document.visibilityState !== 'visible') return
      clearTimer()
      timer = window.setTimeout(() => {
        void loadMetrics()
      }, RESOURCE_METRICS_REFRESH_MS)
    }

    async function loadMetrics() {
      if (document.visibilityState !== 'visible') return
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
        scheduleNext()
      }
    }

    function handleVisibilityChange() {
      if (document.visibilityState === 'visible') {
        void loadMetrics()
        return
      }
      clearTimer()
    }

    document.addEventListener('visibilitychange', handleVisibilityChange)
    if (document.visibilityState === 'visible') {
      void loadMetrics()
    }
    return () => {
      alive = false
      clearTimer()
      document.removeEventListener('visibilitychange', handleVisibilityChange)
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
      const [res, compose] = await Promise.all([getHealthDiagnostics(), getComposePs()])
      setLocalData(res)
      setComposeServices(compose.services ?? [])
      setComposeError(null)
      applyHealthDiagnostics(res)
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

  async function handleJunimoDryRun() {
    setJunimoDryRunBusy(true)
    setJunimoDryRunError(null)
    try {
      setJunimoDryRun(await startJunimoUpdateDryRun())
    } catch (e) {
      setJunimoDryRunError(errorMessage(e))
    } finally {
      setJunimoDryRunBusy(false)
    }
  }

  async function handleJunimoApply() {
    if (!junimoDryRun || junimoDryRun.phase !== 'succeeded') return
    const runningNote = junimoDryRun.serverRunning
      ? '更新期间服务器会安全停服，成功后恢复运行。'
      : '实例当前停止；验证时会临时启动，完成后恢复停止。'
    const confirmed = window.confirm(`将把 server 与 steam-auth-cn 作为一个不可拆分版本对升级。\n\n当前：server ${junimoDryRun.current.server.tag || '—'} + steam-auth-cn ${junimoDryRun.current.steamAuth.tag || '—'}\n目标：server ${junimoDryRun.target.server.tag} + steam-auth-cn ${junimoDryRun.target.steamAuth.tag}\n\n${runningNote}\nSteam 授权预计通过受控认证卷快照保留。确认继续吗？`)
    if (!confirmed) return
    setJunimoApplyBusy(true); setJunimoApplyError(null)
    try { setJunimoApply(await startJunimoUpdateApply()) } catch (e) { setJunimoApplyError(errorMessage(e)) } finally { setJunimoApplyBusy(false) }
  }

  async function handleRuntimePreflight() {
    setRuntimePreflightBusy(true)
    setRuntimeComponentsError(null)
    try {
      const result = await startRuntimeComponentsPreflight()
      setRuntimePreflight(result)
      setRuntimeComponents(await getRuntimeComponents())
    } catch (e) {
      setRuntimeComponentsError(errorMessage(e))
    } finally {
      setRuntimePreflightBusy(false)
    }
  }

  async function handleSMAPIDryRun() {
    setSMAPIDryRunBusy(true)
    setSMAPIError(null)
    try { setSMAPIDryRun(await startSMAPIUpdateDryRun()) } catch (e) { setSMAPIError(errorMessage(e)) } finally { setSMAPIDryRunBusy(false) }
  }

  async function handleSMAPIApply() {
    if (smapiDryRun?.phase !== 'succeeded' || !smapiUpdate) return
    const confirmed = window.confirm(`将从可信清单把 SMAPI ${smapiUpdate.current.version || '未安装'} 升级到 ${smapiUpdate.recommended.version}。\n\nPanel 会复制当前 game-data 到 staging volume，在 staging 上运行官方 Linux 安装器；验收失败会切回旧 volume。玩家随后可能需要重新下载完整同步包。确认继续吗？`)
    if (!confirmed) return
    setSMAPIApplyBusy(true)
    setSMAPIError(null)
    try { setSMAPIApply(await startSMAPIUpdateApply()) } catch (e) { setSMAPIError(errorMessage(e)) } finally { setSMAPIApplyBusy(false) }
  }

  // ── render ──────────────────────────────────────────────────────────────────

  return (
    <div className="sd-page sd-diag-page">
      {/* 页头 */}
      <div className="sd-diag-header sd-page-header">
        <div className="sd-diag-header-left">
          <img className="sd-page-icon" src="/assets/stardew/ui/icons/icon_nav_diagnostics_monitor_image2.png" alt="" />
          <div>
            <h2 className="sd-page-title">服务器健康</h2>
          </div>
        </div>
        <div className="sd-diag-header-actions sd-actionbar sd-actionbar--end">
          <button
            className="sd-btn-green sd-btn--lg"
            disabled={refreshing}
            onClick={handleRefresh}
            type="button"
          >
            {refreshing ? '检查中…' : '重新检查'}
          </button>
        </div>
      </div>

      {/* 加载中状态 */}
      {(dashboardData.loading || refreshing) && !data && (
        <div className="sd-diag-loading">正在加载健康检查数据…</div>
      )}

      {/* 错误条 */}
      {displayError && (
        <div className="sd-diag-error-banner">
          {displayError}
          <button
            className="sd-btn-tan sd-btn--sm"
            style={{ marginLeft: 8 }}
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

      <details
        className="sd-diag-maintenance-details"
        open={maintenanceOpen}
        onToggle={(event) => setMaintenanceOpen(event.currentTarget.open)}
      >
        <summary>
          <span>
            <strong>维护与技术详情</strong>
            <small>版本、镜像、运行来源、预检和升级日志</small>
          </span>
          <span className="sd-diag-maintenance-summary-count">{maintenanceCount ? `${maintenanceCount} 项可处理` : '按需查看'}</span>
        </summary>
        <div className="sd-diag-maintenance-tools">
          <p>这里用于升级和故障排查。日常查看服务器状态不需要理解下面的技术字段。</p>
          <button
            className="sd-btn-tan sd-btn--sm sd-diag-export-btn"
            disabled={exportBusy || !isAdmin}
            onClick={handleExportBundle}
            type="button"
            title={!isAdmin ? '仅管理员可导出诊断包' : '导出含系统信息、日志、状态的诊断包'}
          >
            {exportBusy ? '导出中…' : '导出诊断包'}
          </button>
        </div>
      <section className="sd-card sd-diag-source-panel" aria-label="生命周期状态来源">
        <h3>服务器状态来源</h3>
        <div className="sd-diag-check-list">
          <div className="sd-diag-check-row"><span className="sd-diag-check-name">UI 标准状态</span><span className="sd-diag-check-msg">{instanceState?.uiStatus ?? '未知'} · {instanceState?.uiStatusUpdatedAt ?? '无更新时间'}</span></div>
          <div className="sd-diag-check-row"><span className="sd-diag-check-name">实例 / Driver</span><span className="sd-diag-check-msg">{instanceState?.state ?? '未知'} / {instanceState?.driverPhase ?? '未知'} · {instanceState?.updatedAt ?? '无更新时间'}</span></div>
          <div className="sd-diag-check-row"><span className="sd-diag-check-name">SMAPI status.json</span><span className="sd-diag-check-msg">{instanceState?.statusSource?.state ?? '无数据'} · saveId {instanceState?.statusSource?.saveId || '—'} · {eventAgeLabel(instanceState?.statusSource?.updatedAt)}</span></div>
          <div className="sd-diag-check-row"><span className="sd-diag-check-name">SMAPI players.json</span><span className="sd-diag-check-msg">saveId {instanceState?.playersSource?.saveId || '—'} · {freshnessLabel(instanceState?.playersSource?.updatedAt)}</span></div>
          <div className="sd-diag-check-row"><span className="sd-diag-check-name">当前存档 / 玩家快照身份</span><span className="sd-diag-check-msg">{instanceState?.runtimeDiagnostic?.activeSaveId || '—'} / {instanceState?.runtimeDiagnostic?.cacheSaveId || '—'} · {!instanceState?.runtimeDiagnostic?.cacheSaveId ? '快照未生成' : instanceState.runtimeDiagnostic.cacheMatchesActive ? '匹配' : '不匹配'}</span></div>
          <div className="sd-diag-check-row"><span className="sd-diag-check-name">存档目录</span><span className="sd-diag-check-msg">{instanceState?.runtimeDiagnostic?.saveDirectory || '—'}</span></div>
          <div className="sd-diag-check-row"><span className="sd-diag-check-name">Compose 容器</span><span className="sd-diag-check-msg">{composeServices.length ? composeServices.map((service) => `${service.service || service.name}: ${service.state || 'unknown'}${service.health ? ` (${service.health})` : ''}`).join(' · ') : composeError || '无数据'}</span></div>
          <div className="sd-diag-check-row"><span className="sd-diag-check-name">最近启动阶段耗时</span><span className="sd-diag-check-msg">容器→存档 {durationLabel(instanceState?.runtimeDiagnostic?.containerToSaveMs)} · 存档→主机 {durationLabel(instanceState?.runtimeDiagnostic?.saveToHostMs)}</span></div>
          <div className="sd-diag-check-row"><span className="sd-diag-check-name">控制模组版本</span><span className="sd-diag-check-msg">{instanceState?.runtimeDiagnostic?.controlModVersion || '未安装'} / 期望 {instanceState?.runtimeDiagnostic?.expectedControlModVersion || '—'} · {instanceState?.runtimeDiagnostic?.controlModMatches ? '匹配' : '不匹配'}</span></div>
          <div className="sd-diag-check-row"><span className="sd-diag-check-name">命令回执协议</span><span className="sd-diag-check-msg">commandResultVersion {instanceState?.runtimeDiagnostic?.commandProtocol?.commandResultVersion ?? 0}</span></div>
          <div className="sd-diag-check-row"><span className="sd-diag-check-name">命令队列</span><span className="sd-diag-check-msg">待消费 {instanceState?.runtimeDiagnostic?.commandProtocol?.pendingCommandCount ?? 0} · 未入库结果 {instanceState?.runtimeDiagnostic?.commandProtocol?.unimportedResultCount ?? 0}</span></div>
          <div className="sd-diag-check-row"><span className="sd-diag-check-name">最老待处理命令</span><span className="sd-diag-check-msg">{instanceState?.runtimeDiagnostic?.commandProtocol?.oldestPendingAt ? freshnessLabel(instanceState.runtimeDiagnostic.commandProtocol.oldestPendingAt) : '无'}</span></div>
          <div className="sd-diag-check-row"><span className="sd-diag-check-name">最近控制模组消费</span><span className="sd-diag-check-msg">{freshnessLabel(instanceState?.runtimeDiagnostic?.commandProtocol?.lastControlModConsumeAt)}</span></div>
          <div className="sd-diag-check-row"><span className="sd-diag-check-name">控制目录可写</span><span className="sd-diag-check-msg">commands {instanceState?.runtimeDiagnostic?.commandProtocol?.commandsWritable ? '正常' : '不可写'} · command-results {instanceState?.runtimeDiagnostic?.commandProtocol?.commandResultsWritable ? '正常' : '不可写'}</span></div>
          {(instanceState?.runtimeDiagnostic?.commandProtocol?.warnings ?? []).map((warning) => <div className="sd-diag-check-row" key={warning}><span className="sd-diag-check-name">命令协议警告</span><span className="sd-diag-check-msg">{warning}</span></div>)}
        </div>
      </section>

      <section className="sd-card sd-diag-source-panel sd-diag-runtime-matrix" aria-label="统一运行环境版本">
        <div className="sd-diag-runtime-matrix-head">
          <div>
            <h3>运行环境版本</h3>
            <p>统一矩阵只描述经过审查的精确组合；不会把所有组件无条件更新到 latest。</p>
          </div>
          <span className={`sd-diag-matrix-badge sd-diag-matrix-badge--${runtimeComponents?.recommended.status || junimoUpdate?.recommended.status || 'unknown'}`}>
            {compatibilityMatrixStatusLabel(runtimeComponents?.recommended.status || junimoUpdate?.recommended.status)}
          </span>
        </div>
        <div className="sd-diag-runtime-matrix-meta">
          <span>stack {runtimeComponents?.recommended.stackVersion || junimoUpdate?.recommended.stackVersion || '—'}</span>
          <span>通道 {runtimeComponents?.recommended.channel || junimoUpdate?.recommended.channel || '—'}</span>
          <span>最低 Panel {runtimeComponents?.recommended.minimumPanelVersion || junimoUpdate?.recommended.minimumPanelVersion || '—'}</span>
        </div>
		{junimoUpdate?.recommended.withdrawal ? <div className="sd-diag-dry-run-error">撤回原因：{junimoUpdate.recommended.withdrawal.reason}；建议人工确认后回退到 {junimoUpdate.recommended.withdrawal.fallbackStackVersion}。不会远程静默回退。</div> : null}
        <div className="sd-diag-runtime-matrix-groups">
          <a href="#junimo-update" className="sd-diag-runtime-matrix-group">
            <strong>Junimo server / auth</strong>
            <span>当前 {junimoUpdate?.current.server.tag || '—'} / {junimoUpdate?.current.steamAuth.tag || '—'}</span>
            <span>推荐 {junimoUpdate?.recommended.server.tag || '—'} / {junimoUpdate?.recommended.steamAuth.tag || '—'}</span>
          </a>
          <a href="#runtime-components-update" className="sd-diag-runtime-matrix-group">
            <strong>游戏 / Steamworks SDK</strong>
            <span>当前 {runtimeComponents?.current.game.buildId || '—'} / {runtimeComponents?.current.sdk.buildId || '—'}</span>
            <span>推荐 {runtimeComponents?.recommended.game.buildId || '—'} / {runtimeComponents?.recommended.sdk.buildId || '—'}</span>
          </a>
          <a href="#smapi-update" className="sd-diag-runtime-matrix-group">
            <strong>SMAPI / 控制 Mod</strong>
            <span>当前 {smapiUpdate?.current.version || '—'} / {instanceState?.runtimeDiagnostic?.controlModVersion || '—'}</span>
            <span>推荐 {smapiUpdate?.recommended.version || '—'} / {smapiUpdate?.recommended.compatibility.controlVersion || '—'}</span>
          </a>
        </div>
        <p className="sd-diag-junimo-note">推荐顺序：先更新 Junimo server/auth，再更新游戏/SDK，最后更新 SMAPI/控制 Mod。每一阶段都独立预检、确认、停服验收和回滚；SMAPI 变化后玩家需要重新获取完整同步包。</p>
      </section>

      {isAdmin ? (
        <section id="runtime-components-update" className="sd-card sd-diag-source-panel sd-diag-junimo-panel" aria-label="游戏运行文件版本">
          <h3>游戏运行文件版本</h3>
          <div className="sd-diag-check-list">
            <div className="sd-diag-check-row"><span className="sd-diag-check-name">整体状态</span><span className="sd-diag-check-msg">{runtimeComponentsStatusLabel(runtimeComponents?.status)} · {runtimeComponents?.reason || '读取固定 ACF 后显示'}</span></div>
            <div className="sd-diag-check-row"><span className="sd-diag-check-name">游戏版本</span><span className="sd-diag-check-msg sd-diag-image-ref">当前 buildid {runtimeComponents?.current.game.buildId || '—'} / 推荐 {runtimeComponents?.recommended.game.buildId || '—'}<br />App {runtimeComponents?.current.game.appId || '413150'} · {runtimeComponents?.current.game.manifestPath || 'steamapps/appmanifest_413150.acf'} · StateFlags {runtimeComponents?.current.game.stateFlags || '—'}</span></div>
            <div className="sd-diag-check-row"><span className="sd-diag-check-name">联机运行库</span><span className="sd-diag-check-msg sd-diag-image-ref">当前 buildid {runtimeComponents?.current.sdk.buildId || '—'} / 推荐 {runtimeComponents?.recommended.sdk.buildId || '—'}<br />App {runtimeComponents?.current.sdk.appId || '1007'} · {runtimeComponents?.current.sdk.manifestPath || '.steam-sdk/steamapps/appmanifest_1007.acf'} · StateFlags {runtimeComponents?.current.sdk.stateFlags || '—'}</span></div>
            {runtimeComponents?.current.game.installDir ? <div className="sd-diag-check-row"><span className="sd-diag-check-name">游戏安装目录标记</span><span className="sd-diag-check-msg sd-diag-image-ref">{runtimeComponents.current.game.installDir}</span></div> : null}
            {runtimeComponents?.current.sdk.installDir ? <div className="sd-diag-check-row"><span className="sd-diag-check-name">运行库安装目录标记</span><span className="sd-diag-check-msg sd-diag-image-ref">{runtimeComponents.current.sdk.installDir}</span></div> : null}
            <div className="sd-diag-check-row"><span className="sd-diag-check-name">推荐矩阵</span><span className="sd-diag-check-msg">{runtimeComponents?.recommended.tested ? '已通过发布前验证' : '尚未验证，不提示更新'} · {runtimeComponents?.recommended.game.manifestVersion || '—'} / {runtimeComponents?.recommended.sdk.manifestVersion || '—'}</span></div>
            {(runtimeComponents?.recommended.game.notes ?? []).map((note) => <div className="sd-diag-check-row" key={`game-${note}`}><span className="sd-diag-check-name">游戏版本说明</span><span className="sd-diag-check-msg">{note}</span></div>)}
            {(runtimeComponents?.recommended.sdk.notes ?? []).map((note) => <div className="sd-diag-check-row" key={`sdk-${note}`}><span className="sd-diag-check-name">联机运行库说明</span><span className="sd-diag-check-msg">{note}</span></div>)}
            {runtimeComponentsError ? <div className="sd-diag-check-row sd-diag-check-warning"><span className="sd-diag-check-name">检测错误</span><span className="sd-diag-check-msg">{runtimeComponentsError}</span></div> : null}
          </div>
          <div className="sd-diag-dry-run" aria-label="游戏运行文件只读预检">
            <div className="sd-diag-dry-run-actions"><button className="sd-btn-green sd-btn--sm" type="button" disabled={runtimePreflightBusy} onClick={handleRuntimePreflight}>{runtimePreflightBusy ? '预检中…' : '运行只读预检'}</button><span>仅检查，不提供升级按钮</span></div>
            <div className="sd-diag-dry-run-head"><strong>{runtimePreflight?.phase === 'succeeded' ? '预检通过' : runtimePreflight?.phase === 'failed' ? '预检未通过' : '尚未运行'}</strong><span>{runtimePreflight?.progress ?? 0}%</span></div>
            <progress className="sd-diag-dry-run-progress" max={100} value={runtimePreflight?.progress ?? 0} />
            {runtimePreflight?.requiredBytes ? <div className="sd-diag-dry-run-meta">空间保守估算 {formatBytes(runtimePreflight.requiredBytes)} · 当前可用 {runtimePreflight.freeBytes ? formatBytes(runtimePreflight.freeBytes) : '无法可靠取得'}</div> : null}
            {(runtimePreflight?.checks ?? []).map((check, index) => <div className={`sd-diag-dry-run-check sd-diag-dry-run-check--${check.status}`} key={`runtime-${check.name}-${index}`}><span>{check.status === 'ok' ? '通过' : check.status === 'warning' ? '警告' : '失败'}</span><strong>{check.name}</strong><p>{check.message}</p></div>)}
            {(runtimePreflight?.warnings ?? []).map((warning, index) => <div className="sd-diag-dry-run-warning" key={`${warning}-${index}`}>警告：{warning}</div>)}
            {runtimePreflight?.error ? <div className="sd-diag-dry-run-error">{runtimePreflight.errorCode ? `${runtimePreflight.errorCode}：` : ''}{runtimePreflight.error}</div> : null}
          </div>
        </section>
      ) : null}

      {isAdmin ? (
        <section id="smapi-update" className="sd-card sd-diag-source-panel sd-diag-junimo-panel" aria-label="SMAPI 安全升级">
          <h3>SMAPI 推荐版本与安全升级</h3>
          <div className="sd-diag-check-list">
            <div className="sd-diag-check-row"><span className="sd-diag-check-name">检测状态</span><span className="sd-diag-check-msg">{smapiStatusLabel(smapiUpdate?.status)} · {smapiUpdate?.reason || '正在从实际游戏目录读取'}</span></div>
            <div className="sd-diag-check-row"><span className="sd-diag-check-name">当前 / 推荐</span><span className="sd-diag-check-msg">{smapiUpdate?.current.version || (smapiUpdate?.current.present ? '无法识别' : '未安装')} / {smapiUpdate?.recommended.version || '—'}</span></div>
            <div className="sd-diag-check-row"><span className="sd-diag-check-name">版本来源</span><span className="sd-diag-check-msg sd-diag-image-ref">{smapiUpdate?.current.versionSource || 'SMAPI 程序集元数据与固定安装产物'}；不以 .env 为事实来源</span></div>
            <div className="sd-diag-check-row"><span className="sd-diag-check-name">可信安装器</span><span className="sd-diag-check-msg sd-diag-image-ref">SHA256 {smapiUpdate?.recommended.sha256 || '—'} · {formatBytes(smapiUpdate?.recommended.archiveBytes)}</span></div>
            <div className="sd-diag-check-row"><span className="sd-diag-check-name">Stardew / SDK</span><span className="sd-diag-check-msg">buildid {smapiUpdate?.recommended.compatibility.gameBuildId || '—'} / {smapiUpdate?.recommended.compatibility.sdkBuildId || '—'} <a href="#runtime-components-update">查看前置入口</a></span></div>
            <div className="sd-diag-check-row"><span className="sd-diag-check-name">Junimo / auth</span><span className="sd-diag-check-msg">{smapiUpdate?.recommended.compatibility.junimoVersion || '—'} / {smapiUpdate?.recommended.compatibility.steamAuthVersion || '—'} <a href="#junimo-update">查看前置入口</a></span></div>
            <div className="sd-diag-check-row"><span className="sd-diag-check-name">Control Mod</span><span className="sd-diag-check-msg">{smapiUpdate?.recommended.compatibility.controlVersion || '—'} · commandResultVersion {smapiUpdate?.recommended.compatibility.commandResultVersion ?? '—'}</span></div>
            {smapiError ? <div className="sd-diag-check-row sd-diag-check-warning"><span className="sd-diag-check-name">读取或执行错误</span><span className="sd-diag-check-msg">{smapiError}</span></div> : null}
          </div>
          <div className="sd-diag-dry-run" aria-label="SMAPI 升级执行状态">
            <div className="sd-diag-dry-run-actions">
              <button className="sd-btn-green sd-btn--sm" type="button" disabled={smapiDryRunBusy || smapiPhaseActive(smapiApply?.phase) || !smapiUpdate?.available || !smapiUpdate?.supported} onClick={handleSMAPIDryRun}>{smapiDryRunBusy ? '预检中…' : '运行 SMAPI dry-run'}</button>
              <button className="sd-btn-tan sd-btn--sm" type="button" disabled={smapiApplyBusy || smapiPhaseActive(smapiApply?.phase) || smapiDryRun?.phase !== 'succeeded'} onClick={handleSMAPIApply}>{smapiApplyBusy || smapiPhaseActive(smapiApply?.phase) ? '升级进行中…' : '安全升级 SMAPI'}</button>
            </div>
            {!smapiUpdate?.supported || !smapiUpdate?.available ? <div className="sd-diag-dry-run-warning">升级入口已禁用：{smapiUpdate?.reason || '尚未完成实际版本检测'}。请先从上方游戏运行文件或下方 Junimo 版本对入口修复前置组件；本流程不会顺便更新它们。</div> : null}
            <div className="sd-diag-dry-run-head"><strong>dry-run：{smapiPhaseLabel(smapiDryRun?.phase)}</strong><span>{smapiDryRun?.progress ?? 0}%</span></div>
            <progress className="sd-diag-dry-run-progress" max={100} value={smapiDryRun?.progress ?? 0} />
            {smapiDryRun?.target.version ? <div className="sd-diag-dry-run-meta">只读目标 SMAPI {smapiDryRun.target.version} · 不创建 volume、不下载、不停服</div> : null}
            {smapiDryRun?.requiredBytes ? <div className="sd-diag-dry-run-meta">staging 保守空间 {formatBytes(smapiDryRun.requiredBytes)} · game-data 可用 {smapiDryRun.freeBytes ? formatBytes(smapiDryRun.freeBytes) : '无法可靠取得'}</div> : null}
            {(smapiDryRun?.checks ?? []).map((check, index) => <div className={`sd-diag-dry-run-check sd-diag-dry-run-check--${check.status}`} key={`smapi-dry-${check.name}-${index}`}><span>{check.status === 'ok' ? '通过' : check.status === 'warning' ? '警告' : '失败'}</span><strong>{check.name}</strong><p>{check.message}</p></div>)}
            {(smapiDryRun?.warnings ?? []).map((warning, index) => <div className="sd-diag-dry-run-warning" key={`smapi-warning-${index}`}>警告：{warning}</div>)}
            {smapiDryRun?.error ? <div className="sd-diag-dry-run-error">{smapiDryRun.errorCode ? `${smapiDryRun.errorCode}：` : ''}{smapiDryRun.error}</div> : null}
            <div className={`sd-diag-apply sd-diag-apply--${smapiApply?.phase ?? 'idle'}`}>
              <div className="sd-diag-dry-run-head"><strong>apply：{smapiPhaseLabel(smapiApply?.phase)}</strong><span>{smapiApply?.progress ?? 0}%</span></div>
              <progress className="sd-diag-dry-run-progress" max={100} value={smapiApply?.progress ?? 0} />
              {smapiApply?.updateId ? <div className="sd-diag-dry-run-meta">updateId {smapiApply.updateId} · jobId {smapiApply.jobId || '—'} · 升级前服务器{smapiApply.serverWasRunning ? '运行' : '停止'}</div> : null}
              {smapiApply?.phase === 'succeeded' ? <div className="sd-diag-dry-run-check sd-diag-dry-run-check--ok"><span>完成</span><strong>staging 验收通过</strong><p>新 volume 已启用，旧 game-data 仍保留为恢复材料。</p></div> : null}
              {smapiApply?.phase === 'failed_rolled_back' ? <div className="sd-diag-dry-run-warning">新版本未通过完整验收，已切回旧 GAME_DATA_VOLUME 并恢复原运行状态。</div> : null}
              {smapiApply?.phase === 'rollback_failed' ? <div className="sd-diag-dry-run-error">自动回滚失败；恢复材料已保留。{smapiApply.manualAction || '请停止自动重试并人工检查。'}</div> : null}
              {(smapiApply?.checks ?? []).map((check, index) => <div className={`sd-diag-dry-run-check sd-diag-dry-run-check--${check.status}`} key={`smapi-apply-${check.name}-${index}`}><span>{check.status === 'ok' ? '通过' : check.status === 'warning' ? '警告' : '失败'}</span><strong>{check.name}</strong><p>{check.message}</p></div>)}
              {smapiApply?.error && smapiApply.phase !== 'rollback_failed' ? <div className="sd-diag-dry-run-error">{smapiApply.errorCode ? `${smapiApply.errorCode}：` : ''}{smapiApply.error}</div> : null}
              {(smapiApply?.logs ?? []).length ? <details className="sd-diag-dry-run-logs"><summary>脱敏升级日志</summary>{smapiApply!.logs.map((log, index) => <div key={`${log.at}-${index}`}>{log.at} [{log.level}] {log.message}</div>)}</details> : null}
            </div>
          </div>
          <p className="sd-diag-junimo-note">完整玩家同步包会记录并携带当前推荐 SMAPI 安装器；仅含 Mod 的增量包不会携带 SMAPI。升级后请提醒玩家重新导出完整同步包，并保持客户端 SMAPI 与服务器推荐版本一致。</p>
        </section>
      ) : null}

      <section id="junimo-update" className="sd-card sd-diag-source-panel sd-diag-junimo-panel" aria-label="Junimo 运行组件版本对">
        <h3>Junimo 运行组件版本对</h3>
        <div className="sd-diag-check-list">
          <div className="sd-diag-check-row">
            <span className="sd-diag-check-name">整体状态</span>
            <span className="sd-diag-check-msg">{junimoUpdateStatusLabel(junimoUpdate?.status ?? instanceState?.runtimeDiagnostic?.junimoUpdateStatus)}</span>
          </div>
          <div className="sd-diag-check-row">
            <span className="sd-diag-check-name">当前 server 镜像 / tag</span>
            <span className="sd-diag-check-msg sd-diag-image-ref">{isAdmin ? (junimoUpdate?.current.server.image || '未配置') : '仓库信息仅管理员可见'} · tag {junimoUpdate?.current.server.tag || instanceState?.runtimeDiagnostic?.serverVersion || '—'}</span>
          </div>
          <div className="sd-diag-check-row">
            <span className="sd-diag-check-name">当前 steam-auth-cn 镜像 / tag</span>
            <span className="sd-diag-check-msg sd-diag-image-ref">{isAdmin ? (junimoUpdate?.current.steamAuth.image || '未配置') : '仓库信息仅管理员可见'} · tag {junimoUpdate?.current.steamAuth.tag || instanceState?.runtimeDiagnostic?.steamAuthVersion || '—'}</span>
          </div>
          <div className="sd-diag-check-row">
            <span className="sd-diag-check-name">推荐版本对</span>
            <span className="sd-diag-check-msg">{junimoUpdate?.recommended.stackVersion || instanceState?.runtimeDiagnostic?.junimoStackVersion || '—'} · server {junimoUpdate?.recommended.server.tag || instanceState?.runtimeDiagnostic?.expectedServerVersion || '—'} + steam-auth-cn {junimoUpdate?.recommended.steamAuth.tag || instanceState?.runtimeDiagnostic?.expectedSteamAuthVersion || '—'}</span>
          </div>
          {isAdmin && junimoUpdate ? (
            <>
              <div className="sd-diag-check-row"><span className="sd-diag-check-name">推荐 server 镜像</span><span className="sd-diag-check-msg sd-diag-image-ref">{recommendedImage(junimoUpdate.recommended.server)} · tag {junimoUpdate.recommended.server.tag}</span></div>
              <div className="sd-diag-check-row"><span className="sd-diag-check-name">推荐 steam-auth-cn 镜像</span><span className="sd-diag-check-msg sd-diag-image-ref">{recommendedImage(junimoUpdate.recommended.steamAuth)} · tag {junimoUpdate.recommended.steamAuth.tag}</span></div>
            </>
          ) : null}
          <div className="sd-diag-check-row">
            <span className="sd-diag-check-name">是否匹配</span>
            <span className="sd-diag-check-msg">{junimoPairMatches(junimoUpdate?.status ?? instanceState?.runtimeDiagnostic?.junimoUpdateStatus) ? 'server 与 steam-auth-cn 版本对完全匹配' : '版本对不匹配或无法判断'}</span>
          </div>
          {!(junimoUpdate?.supported ?? instanceState?.runtimeDiagnostic?.junimoUpdateSupported ?? false) ? (
            <div className="sd-diag-check-row sd-diag-check-warning">
              <span className="sd-diag-check-name">unsupported 原因</span>
              <span className="sd-diag-check-msg">{junimoUpdate?.reason || instanceState?.runtimeDiagnostic?.junimoUpdateReason || '当前配置不受支持'}</span>
            </div>
          ) : null}
          {junimoUpdateError ? <div className="sd-diag-check-row sd-diag-check-warning"><span className="sd-diag-check-name">详情读取</span><span className="sd-diag-check-msg">{junimoUpdateError}</span></div> : null}
          {(junimoUpdate?.releaseNotes ?? []).map((note) => <div className="sd-diag-check-row" key={note}><span className="sd-diag-check-name">版本说明</span><span className="sd-diag-check-msg">{note}</span></div>)}
        </div>
        {isAdmin ? (
          <div className="sd-diag-dry-run" aria-label="Junimo 运行组件升级预检">
            <div className="sd-diag-dry-run-actions">
              <button
                className="sd-btn-green sd-btn--sm"
                disabled={junimoDryRunBusy || junimoDryRunActive(junimoDryRun?.phase) || junimoUpdate?.supported === false}
                onClick={handleJunimoDryRun}
                type="button"
              >
                {junimoDryRunBusy || junimoDryRunActive(junimoDryRun?.phase) ? '预检进行中…' : '运行升级预检'}
              </button>
              <button
                className="sd-btn-tan sd-btn--sm"
                disabled={junimoApplyBusy || junimoApplyActive(junimoApply?.phase) || junimoDryRun?.phase !== 'succeeded'}
                onClick={handleJunimoApply}
                type="button"
                title={junimoDryRun?.phase !== 'succeeded' ? '必须先完成当前推荐版本对的升级预检' : 'server 与 steam-auth-cn 将同时升级'}
              >
                {junimoApplyBusy || junimoApplyActive(junimoApply?.phase) ? '升级进行中…' : '更新运行组件'}
              </button>
            </div>
            <div className="sd-diag-dry-run-head">
              <strong>{junimoDryRunPhaseLabel(junimoDryRun?.phase)}</strong>
              <span>{junimoDryRun?.progress ?? 0}%</span>
            </div>
            <progress className="sd-diag-dry-run-progress" max={100} value={junimoDryRun?.progress ?? 0} />
            {junimoDryRun?.dryRunId ? <div className="sd-diag-dry-run-meta">dryRunId {junimoDryRun.dryRunId} · jobId {junimoDryRun.jobId || '—'} · server {junimoDryRun.serverRunning ? '运行中（不会停服）' : '未运行'}</div> : null}
            {junimoDryRun?.target.stackVersion ? <div className="sd-diag-dry-run-pair"><strong>目标版本对</strong><span>{junimoDryRun.target.stackVersion} · server {junimoDryRun.target.server.tag} + steam-auth-cn {junimoDryRun.target.steamAuth.tag}</span></div> : null}
            {junimoDryRun?.selected.server.image ? <div className="sd-diag-dry-run-pair"><strong>选中 server</strong><span>{junimoDryRun.selected.server.image}<br />digest {junimoDryRun.selected.server.digest || '无法确认'}</span></div> : null}
            {junimoDryRun?.selected.steamAuth.image ? <div className="sd-diag-dry-run-pair"><strong>选中 steam-auth-cn</strong><span>{junimoDryRun.selected.steamAuth.image}<br />digest {junimoDryRun.selected.steamAuth.digest || '无法确认'}</span></div> : null}
            {(junimoDryRun?.checks ?? []).map((check, index) => (
              <div className={`sd-diag-dry-run-check sd-diag-dry-run-check--${check.status}`} key={`${check.name}-${index}`}>
                <span>{check.status === 'ok' ? '通过' : check.status === 'warning' ? '警告' : '失败'}</span><strong>{check.name}</strong><p>{check.message}</p>
              </div>
            ))}
            {(junimoDryRun?.warnings ?? []).map((warning, index) => <div className="sd-diag-dry-run-warning" key={`${warning}-${index}`}>警告：{warning}</div>)}
            {junimoDryRun?.error ? <div className="sd-diag-dry-run-error">{junimoDryRun.errorCode ? `${junimoDryRun.errorCode}：` : ''}{junimoDryRun.error}</div> : null}
            {junimoDryRunError ? <div className="sd-diag-dry-run-error">{junimoDryRunError}</div> : null}
            {(junimoDryRun?.logs ?? []).length ? <details className="sd-diag-dry-run-logs"><summary>脱敏预检日志</summary>{junimoDryRun!.logs.map((log, index) => <div key={`${log.at}-${index}`}>{log.at} [{log.level}] {log.message}</div>)}</details> : null}
            <div className={`sd-diag-apply sd-diag-apply--${junimoApply?.phase ?? 'idle'}`} aria-label="Junimo 运行组件升级执行状态">
              <div className="sd-diag-dry-run-head"><strong>{junimoApplyPhaseLabel(junimoApply?.phase)}</strong><span>{junimoApply?.progress ?? 0}%</span></div>
              <progress className="sd-diag-dry-run-progress" max={100} value={junimoApply?.progress ?? 0} />
              {junimoApply?.applyId ? <div className="sd-diag-dry-run-meta">applyId {junimoApply.applyId} · jobId {junimoApply.jobId || '—'} · 升级前 {junimoApply.serverWasRunning ? '运行' : '停止'}</div> : null}
              {junimoApply?.target.stackVersion ? <div className="sd-diag-dry-run-pair"><strong>成对目标</strong><span>{junimoApply.target.stackVersion} · server {junimoApply.target.server.tag} + steam-auth-cn {junimoApply.target.steamAuth.tag}</span></div> : null}
              {junimoApply?.phase === 'succeeded' ? <div className="sd-diag-dry-run-check sd-diag-dry-run-check--ok"><span>完成</span><strong>成对升级成功</strong><p>Steam 认证与运行链路已验证，实例已恢复升级前运行状态。</p></div> : null}
              {junimoApply?.phase === 'failed_rolled_back' ? <div className="sd-diag-dry-run-warning">升级未通过验收，但 server/auth、认证卷与运行状态已自动回滚。</div> : null}
              {junimoApply?.phase === 'rollback_failed' ? <div className="sd-diag-dry-run-error">自动回滚失败。{junimoApply.manualAction || '请保留恢复材料并联系管理员人工处理。'} 不提供自动破坏性重试。</div> : null}
              {(junimoApply?.checks ?? []).map((check, index) => <div className={`sd-diag-dry-run-check sd-diag-dry-run-check--${check.status}`} key={`apply-${check.name}-${index}`}><span>{check.status === 'ok' ? '通过' : check.status === 'warning' ? '警告' : '失败'}</span><strong>{check.name}</strong><p>{check.message}</p></div>)}
              {(junimoApply?.warnings ?? []).map((warning, index) => <div className="sd-diag-dry-run-warning" key={`${warning}-${index}`}>警告：{warning}</div>)}
              {junimoApply?.error && junimoApply.phase !== 'rollback_failed' ? <div className="sd-diag-dry-run-error">{junimoApply.errorCode ? `${junimoApply.errorCode}：` : ''}{junimoApply.error}</div> : null}
              {junimoApplyError ? <div className="sd-diag-dry-run-error">{junimoApplyError}</div> : null}
              {(junimoApply?.logs ?? []).length ? <details className="sd-diag-dry-run-logs"><summary>脱敏升级日志</summary>{junimoApply!.logs.map((log, index) => <div key={`${log.at}-${index}`}>{log.at} [{log.level}] {log.message}</div>)}</details> : null}
            </div>
          </div>
        ) : null}
        <p className="sd-diag-junimo-note">执行升级只使用 Panel 内置且 tested=true 的版本对；不会接受自定义目标，不会执行 down -v，也不会删除 game-data 或 steam-session。</p>
      </section>
      </details>

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

      <section className="sd-diag-maintenance-panel" aria-label="版本维护建议">
        <div className="sd-diag-maintenance-head">
          <div>
            <div className="sd-diag-section-title">版本维护</div>
            <p>{maintenanceCount ? `发现 ${maintenanceCount} 项推荐维护，均由管理员确认后执行。` : '当前组件已匹配推荐版本，无需操作。'}</p>
          </div>
          <span className={`sd-diag-maintenance-badge ${maintenanceCount ? 'is-pending' : 'is-ok'}`}>
            {maintenanceCount ? '建议处理' : '已是推荐版本'}
          </span>
        </div>
        <div className="sd-diag-maintenance-list">
          {junimoUpdateAvailable ? (
            <div className="sd-diag-maintenance-item">
              <span className="sd-diag-maintenance-item-icon" aria-hidden="true">↑</span>
              <div>
                <strong>Junimo 服务组件有推荐更新</strong>
                <p>{junimoUpdate?.current.server.tag || instanceState?.runtimeDiagnostic?.serverVersion || '当前版本'} → {junimoUpdate?.recommended.server.tag || instanceState?.runtimeDiagnostic?.expectedServerVersion || '推荐版本'}。不升级仍可继续使用。</p>
              </div>
              {isAdmin ? <button className="sd-btn-green sd-btn--sm" type="button" onClick={() => showMaintenanceDetail('junimo-update')}>查看并预检</button> : <span className="sd-diag-maintenance-role-note">请联系管理员</span>}
            </div>
          ) : null}
          {gameUpdateAvailable ? (
            <div className="sd-diag-maintenance-item">
              <span className="sd-diag-maintenance-item-icon" aria-hidden="true">↑</span>
              <div><strong>游戏运行文件有推荐更新</strong><p>先进行只读预检，确认版本与磁盘空间。</p></div>
              <button className="sd-btn-green sd-btn--sm" type="button" onClick={() => showMaintenanceDetail('runtime-components-update')}>查看详情</button>
            </div>
          ) : null}
          {smapiUpdateAvailable ? (
            <div className="sd-diag-maintenance-item">
              <span className="sd-diag-maintenance-item-icon" aria-hidden="true">↑</span>
              <div><strong>SMAPI 有经过验证的更新</strong><p>{smapiUpdate?.current.version || '当前版本'} → {smapiUpdate?.recommended.version || '推荐版本'}，升级前会先执行安全预检。</p></div>
              <button className="sd-btn-green sd-btn--sm" type="button" onClick={() => showMaintenanceDetail('smapi-update')}>查看并预检</button>
            </div>
          ) : null}
          {!maintenanceCount ? (
            <div className="sd-diag-maintenance-empty"><span aria-hidden="true">✓</span><div><strong>不用做任何事</strong><p>游戏服务、运行文件和模组框架均未发现推荐更新。</p></div></div>
          ) : null}
        </div>
      </section>

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
              <GaugeCard label="CPU" value={latestMetric?.cpuPercent} sub={metricSubtitles.cpu} gradientId="sd-gauge-grad-cpu" />
              <GaugeCard label="内存" value={latestMetric?.memoryPercent} sub={metricSubtitles.memory} gradientId="sd-gauge-grad-memory" />
              <GaugeCard label="磁盘" value={latestMetric?.diskPercent} sub={metricSubtitles.disk} gradientId="sd-gauge-grad-disk" />
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
import './DiagnosticsPage.css'
