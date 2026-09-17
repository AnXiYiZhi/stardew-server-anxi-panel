import { useEffect, useRef, useState, type CSSProperties } from 'react'
import type { ResourceMetricSample } from '../../types'
import { RESOURCE_SERIES, resourceHistory, resourceSegments, resourceValue } from './resource-metrics-presentation'
import './ResourceMonitor.css'
import { resourceComparison, resourceRing } from '../resource-presentation'
import { ResourceHint, ResourcePair } from '../ResourceHint'

function bytes(value?: number) {
  if (value == null || !Number.isFinite(value) || value < 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) { value /= 1024; unit++ }
  return `${Number(value.toFixed(1))} ${units[unit]}`
}

function timeLabel(timestamp: string) {
  return new Date(timestamp).toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

export function ResourceMonitor({ samples, error, machine }: { samples: ResourceMetricSample[]; error: string | null; machine?: ResourceMetricSample }) {
  const historyRef = useRef<HTMLDivElement>(null)
  const [chartWidth, setChartWidth] = useState(560)
  useEffect(() => {
    const target = historyRef.current
    if (!target) return
    const observer = new ResizeObserver(([entry]) => setChartWidth(Math.max(240, Math.round(entry.contentRect.width))))
    observer.observe(target)
    return () => observer.disconnect()
  }, [])
  const latest = samples[samples.length - 1]
  const world = latest?.scope === 'world'
  const trendSeries = world ? RESOURCE_SERIES.filter(series => series.key !== 'diskPercent') : RESOURCE_SERIES
  const state = error || latest?.containerState === 'unavailable' ? 'error' : !latest ? 'loading' : latest.containerRunning ? 'live' : 'idle'
  const status = { error: '采样中断', loading: '正在读取', live: '实时采样', idle: '服务器未运行' }[state]
  const history = resourceHistory(samples)
  const hasTrend = history.length >= 2 && RESOURCE_SERIES.some(s => resourceSegments(history, s.key).some(segment => segment.length >= 2))
  const yMax = Math.max(100, Math.ceil(Math.max(0, ...history.map(s => resourceValue(s, 'cpuPercent') ?? 0)) / 50) * 50)
  const start = history[0] ? Date.parse(history[0].timestamp) : 0
  const end = history.length ? Date.parse(history[history.length - 1].timestamp) : start
  const point = (time: number, value: number) => `${(48 + (time - start) / Math.max(1, end - start) * (chartWidth - 60)).toFixed(1)},${(134 - value / yMax * 116).toFixed(1)}`
  const empty = state === 'error' ? '采样暂时中断，正在自动重试' : !latest ? '正在读取资源数据…' : !latest.containerRunning ? '服务器未运行，存储仍可查看' : '已收到采样，趋势将在后续采样后显示'

  return (
    <section className="sd-resource-monitor" aria-label="资源趋势">
      <header className="sd-resource-heading">
        <h3>{world ? '当前世界资源' : '资源趋势'}</h3>
        <span className={`sd-resource-status is-${state}`} role="status">{status}</span>
      </header>
      {world && <div className="sd-resource-meter-key" aria-label="进度条图例"><span><i className="sd-resource-key-world" />当前世界</span><span><i className="sd-resource-key-machine" />整机总占用</span></div>}
      <div className="sd-resource-readings">
        {RESOURCE_SERIES.map(({ key, label, color }) => {
          const storage = world && key === 'diskPercent'
          const reading = resourceComparison(key === 'cpuPercent' ? 'cpu' : key === 'memoryPercent' ? 'memory' : 'storage', latest, machine)
          const value = world ? reading.percent : resourceValue(latest, key)
          const percent = Math.min(100, value ?? 0)
          const machinePercent = world ? resourceRing(reading.machinePercent) : 0
          const missing = value == null
          const warning = key !== 'cpuPercent' && value != null && value >= 90
          const caption = missing
            ? state === 'error' ? '暂时无法读取' : !latest ? '等待首次采样' : key !== 'diskPercent' && !latest.containerRunning ? '启动服务器后显示' : '暂无数据'
            : world ? `当前世界 / 整机已用 · ${storage ? '占数据盘' : '占整机'}`
              : key === 'cpuPercent' ? value > 100 ? '多核 CPU 占用' : '容器 CPU'
                : key === 'memoryPercent' ? `${bytes(latest?.memoryUsedBytes)} / ${bytes(latest?.memoryLimitBytes)}`
                  : `${bytes(latest?.diskUsedBytes)} / ${bytes(latest?.diskTotalBytes)}`
          return (
            <div className={`sd-resource-reading${missing ? ' is-empty' : ''}`} key={key} style={{ '--resource-color': color } as CSSProperties}>
              <div className="sd-resource-reading-head">
                <strong>{storage ? '存储' : label}</strong>
                {world ? <ResourceHint className="sd-resource-value" detail={reading.detail} label={reading.accessible}><ResourcePair {...reading} /></ResourceHint>
                  : <span className="sd-resource-value">{missing ? '—' : value.toFixed(1)}{!missing && <small>%</small>}</span>}
              </div>
              <div className="sd-resource-meter" role={missing ? 'img' : 'meter'} aria-label={world ? reading.accessible : missing ? `${label}：${caption}` : `${label}使用率`} aria-valuemin={missing ? undefined : 0} aria-valuemax={missing ? undefined : Math.max(100, value)} aria-valuenow={missing ? undefined : value} aria-valuetext={missing ? undefined : world ? reading.accessible : `${value.toFixed(1)}%`}>
                {Array.from({ length: 20 }, (_, i) => <span key={i} aria-hidden="true">
                  {world && <i className="sd-resource-meter-machine" style={{ width: `${Math.max(0, Math.min(100, (machinePercent - i * 5) * 20))}%` }} />}
                  <i className="sd-resource-meter-world" style={{ width: `${Math.max(0, Math.min(100, (percent - i * 5) * 20))}%` }} />
                </span>)}
              </div>
              <div className="sd-resource-caption"><span>{caption}</span>{warning && <strong className="sd-resource-warning">占用较高</strong>}{error && !missing && <span>上次采样</span>}</div>
            </div>
          )
        })}
      </div>
      <div className="sd-resource-history" ref={historyRef}>
        <div className="sd-resource-history-head"><strong>{world ? '最近采样 · 当前世界' : '最近采样'}</strong><span>每 8 秒更新</span></div>
        {hasTrend ? (
          <svg viewBox={`0 0 ${chartWidth} 164`} className="sd-resource-chart" role="img" aria-label={world ? '当前世界的 CPU、内存占整机比例，横轴为采样时间' : '最近采样的 CPU、内存和磁盘使用率，横轴为采样时间，纵轴为百分比'}>
            {[0, 0.5, 1].map(ratio => <g key={ratio}><line x1="48" x2={chartWidth - 12} y1={134 - ratio * 116} y2={134 - ratio * 116} /><text x="38" y={138 - ratio * 116} textAnchor="end">{Math.round(yMax * ratio)}%</text></g>)}
            {trendSeries.map(series => resourceSegments(history, series.key).map((segment, i) => segment.length > 1
              ? <polyline key={`${series.key}-${i}`} points={segment.map(p => point(p.time, p.value)).join(' ')} stroke={series.color} />
              : <rect key={`${series.key}-${i}`} x={Number(point(segment[0].time, segment[0].value).split(',')[0]) - 2} y={Number(point(segment[0].time, segment[0].value).split(',')[1]) - 2} width="4" height="4" fill={series.color} />))}
            <text x="48" y="158">{timeLabel(history[0].timestamp)}</text>
            <text x={chartWidth - 12} y="158" textAnchor="end">{timeLabel(history[history.length - 1].timestamp)}</text>
          </svg>
        ) : <div className="sd-resource-empty"><span className="sd-resource-empty-lines" aria-hidden="true"><i /><i /><i /></span><span>{empty}</span></div>}
        <div className="sd-resource-legend">{trendSeries.map(s => <span key={s.key}><i style={{ background: s.color }} />{s.label}</span>)}</div>
      </div>
      {error && <div className="sd-resource-note" role="status">{error}</div>}
      {latest?.message && state !== 'idle' && !error && <div className="sd-resource-note">{latest.message}</div>}
    </section>
  )
}
