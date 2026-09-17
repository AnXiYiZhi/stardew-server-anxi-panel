import { useState } from 'react'
import { ResourceMonitor } from './games/stardew/ResourceMonitor'
import type { ResourceMetricSample } from './types'

const states = ['正常运行', '未启动', '高占用', '零占用', '读取中', '读取失败', '采样失败', '采样缺失'] as const
export function ResourceMonitorQA() {
  const [state, setState] = useState<typeof states[number]>('正常运行')
  const history: ResourceMetricSample[] = Array.from({ length: 24 }, (_, i) => ({
    timestamp: new Date(Date.UTC(2026, 8, 15, 6, 20, i * 8)).toISOString(),
    containerRunning: state !== '未启动',
    cpuPercent: state === '零占用' ? 0 : state === '高占用' ? 210 + i : 18 + Math.sin(i) * 5,
    memoryPercent: state === '零占用' ? 0 : state === '高占用' ? 95 : 42 + Math.cos(i) * 3,
    diskPercent: state === '零占用' ? 0 : state === '高占用' ? 99 : 31,
    memoryUsedBytes: 3.4 * 1024 ** 3, memoryLimitBytes: 8 * 1024 ** 3,
    diskUsedBytes: 42.6 * 1024 ** 3, diskTotalBytes: 128 * 1024 ** 3,
  }))
  history.forEach(sample => {
    sample.memoryUsedBytes = sample.memoryPercent! / 100 * sample.memoryLimitBytes!
    sample.diskUsedBytes = sample.diskPercent! / 100 * sample.diskTotalBytes!
  })
  if (state === '采样缺失') history.forEach((s, i) => { if (i > 6 && i < 18) s.cpuPercent = null })
  return <main style={{ padding: 16, background: '#f6e5bb', minHeight: '100dvh', boxSizing: 'border-box', overflow: 'auto', height: '100dvh' }}>
    <nav aria-label="资源状态夹具" style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginBottom: 16 }}>{states.map(s => <button key={s} type="button" className="sd-btn-tan" aria-pressed={state === s} onClick={() => setState(s)}>{s}</button>)}</nav>
    <div style={{ maxWidth: 560, margin: '0 auto' }}><ResourceMonitor samples={state === '读取中' || state === '读取失败' ? [] : state === '未启动' ? history.slice(-1) : history} error={state === '采样失败' || state === '读取失败' ? '无法读取资源，连接恢复后将继续采样。' : null} /></div>
  </main>
}
