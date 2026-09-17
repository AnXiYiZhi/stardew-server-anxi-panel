import { useEffect, useState, type CSSProperties } from 'react'
import { getResourceOverview } from '../api'
import type { ResourceMetricSample, ResourceOverview } from '../types'
import { resourceAmount, resourceBytes, resourceCores, resourcePercent, resourceRing, resourceComparison } from './resource-presentation'
import { ResourceHint, ResourcePair } from './ResourceHint'
import './ResourceReadouts.css'

export function useResourceOverview() {
  const [data, setData] = useState<ResourceOverview | null>(null)
  const [error, setError] = useState(false)
  useEffect(() => {
    let active = true
    let busy = false
    let timer: ReturnType<typeof setTimeout> | undefined
    let controller: AbortController | undefined
    async function load() {
      if (!active || busy || document.visibilityState === 'hidden') return
      clearTimeout(timer)
      busy = true
      controller = new AbortController()
      const deadline = setTimeout(() => controller?.abort(), 30_000)
      try {
        const response = await getResourceOverview(controller.signal)
        if (active) { setData(response); setError(false) }
      } catch {
        if (active) setError(true)
      } finally {
        clearTimeout(deadline)
        busy = false
        if (active) timer = setTimeout(() => void load(), 8_000)
      }
    }
    const visible = () => { if (document.visibilityState === 'visible') void load() }
    void load()
    document.addEventListener('visibilitychange', visible)
    return () => { active = false; controller?.abort(); clearTimeout(timer); document.removeEventListener('visibilitychange', visible) }
  }, [])
  return { data: error ? null : data, error }
}

type ResourceKind = 'cpu' | 'memory' | 'storage'
const resources = [
  { kind: 'cpu', label: 'CPU', key: 'cpuPercent', color: '#52783e' },
  { kind: 'memory', label: '内存', key: 'memoryPercent', color: '#a26b24' },
  { kind: 'storage', label: '磁盘', key: 'diskPercent', color: '#347985' },
] as const

export function ResourceIcon({ kind }: { kind: ResourceKind }) {
  return <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" aria-hidden="true">
    {kind === 'cpu' ? <><rect x="6" y="6" width="12" height="12" rx="1" /><path d="M9 2v4m6-4v4M9 18v4m6-4v4M2 9h4m-4 6h4m12-6h4m-4 6h4" /><path d="M10 10h4v4h-4z" /></>
      : kind === 'memory' ? <><rect x="3" y="6" width="18" height="12" rx="1" /><path d="M7 10v4m5-4v4m5-4v4M7 18v3m5-3v3m5-3v3" /></>
        : <><rect x="5" y="3" width="14" height="18" rx="1" /><circle cx="12" cy="10" r="3" /><path d="M8 17h8" /></>}
  </svg>
}

export function MachineResources({ sample, error }: { sample?: ResourceMetricSample; error: boolean }) {
  return <div className="hub-machine-resources" aria-label="整机资源">
    <span className="hub-machine-label">整机</span>
    {resources.map(({ kind, key, label, color }) => {
      const value = sample?.[key]
      const detail = value == null ? error ? '暂不可用，正在重试' : sample ? '暂不可用' : '正在读取'
        : kind === 'cpu' ? `${resourceAmount(kind, sample)} / ${resourceCores(sample?.cpuCount)} · ${resourcePercent(value)}`
          : kind === 'memory' ? `${resourceBytes(sample?.memoryUsedBytes)} / ${resourceBytes(sample?.memoryTotalBytes)}`
            : `${resourceBytes(sample?.diskUsedBytes)} / ${resourceBytes(sample?.diskTotalBytes)} · 数据所在磁盘`
      return <ResourceHint className="hub-machine-metric" key={kind} style={{ '--resource-accent': color } as CSSProperties} focusable={false} label={`整机${label}：${detail}`} detail={`${label} ${detail}`}>
        <button type="button" aria-label={`整机${label}：${detail}`}>
          <span className="hub-machine-ring" aria-hidden="true">
            <svg viewBox="0 0 28 28"><circle className="hub-machine-track" cx="14" cy="14" r="12" /><circle className="hub-machine-fill" cx="14" cy="14" r="12" pathLength="100" strokeDasharray={`${resourceRing(value)} 100`} /></svg>
            <ResourceIcon kind={kind} />
          </span>
          <span className="hub-machine-percent">{resourcePercent(value)}</span>
        </button>
      </ResourceHint>
    })}
  </div>
}

export function GameResources({ sample, machine, error }: { sample?: ResourceMetricSample; machine?: ResourceMetricSample; error: boolean }) {
  return <span id="stardew-game-resource-details" className="hub-game-resources" aria-label="游戏资源合计">
    {resources.map(({ kind, label, color }) => {
      const reading = resourceComparison(kind, error ? undefined : sample, error ? undefined : machine)
      return <ResourceHint className={`hub-game-resource${reading.primary === '—' ? ' is-unavailable' : ''}`} key={kind} label={`${reading.accessible}；${reading.detail}`} detail={reading.detail} focusable={false} style={{ '--resource-accent': color } as CSSProperties}>
        <span className="hub-game-resource-label"><ResourceIcon kind={kind} /><span>{kind === 'storage' ? '存储' : label}</span></span>
        <strong className="hub-game-resource-value"><ResourcePair {...reading} /></strong>
      </ResourceHint>
    })}
  </span>
}
