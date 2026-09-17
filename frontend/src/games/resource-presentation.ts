import type { ResourceMetricSample } from '../types'

export function resourceBytes(value?: number | null) {
  if (value == null || !Number.isFinite(value) || value < 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) { value /= 1024; unit++ }
  return `${Number(value.toFixed(1))} ${units[unit]}`
}

export function resourcePercent(value?: number | null) {
  return value == null || !Number.isFinite(value) || value < 0 ? '—' : `${Number(value.toFixed(1))}%`
}

export function resourceRing(value?: number | null) {
  return value == null || !Number.isFinite(value) || value < 0 ? 0 : Math.min(100, value)
}

export function resourceShare(used?: number | null, total?: number | null) {
  if (used == null || total == null || !Number.isFinite(used) || !Number.isFinite(total) || used < 0 || total <= 0 || used > total) return null
  return used / total * 100
}

export function resourceShareText(value?: number | null) {
  return value != null && value > 0 && value < 0.1 ? '<0.1%' : resourcePercent(value)
}

export type ResourceKind = 'cpu' | 'memory' | 'storage'

export function resourceCores(value?: number | null) {
  if (value == null || !Number.isFinite(value) || value < 0) return '—'
  return `${value > 0 && value < 0.01 ? '<0.01' : Number(value.toFixed(2))} 核`
}

export function resourceAmount(kind: ResourceKind, sample?: ResourceMetricSample, machine = sample) {
  if (!sample) return '—'
  if (kind === 'cpu') {
    if (sample.cpuPercent == null) return '—'
    const cores = sample.scope !== 'machine' && sample.cpuCores != null
      ? sample.cpuCores : machine?.cpuCount ? sample.cpuPercent / 100 * machine.cpuCount : null
    return resourceCores(cores)
  }
  if (kind === 'memory') return sample.memoryPercent == null ? '—' : resourceBytes(sample.memoryUsedBytes ?? 0)
  return sample.scope === 'machine' ? sample.diskPercent == null ? '—' : resourceBytes(sample.diskUsedBytes ?? 0) : resourceBytes(sample.storageUsedBytes)
}

export function resourceComparison(kind: ResourceKind, sample?: ResourceMetricSample, machine?: ResourceMetricSample, scope = sample?.scope) {
  const percent = kind === 'cpu' ? sample?.cpuPercent : kind === 'memory' ? sample?.memoryPercent : resourceShare(sample?.storageUsedBytes, machine?.diskTotalBytes)
  const machinePercent = kind === 'cpu' ? machine?.cpuPercent : kind === 'memory' ? machine?.memoryPercent : machine?.diskPercent
  const primary = resourceShareText(percent)
  const secondary = resourceShareText(machinePercent)
  const scopeLabel = scope === 'world' ? '当前世界' : '游戏合计'
  const label = kind === 'cpu' ? 'CPU' : kind === 'memory' ? '内存' : '存储'
  const capacity = kind === 'cpu' ? resourceCores(machine?.cpuCount) : resourceBytes(kind === 'memory' ? machine?.memoryTotalBytes : machine?.diskTotalBytes)
  const storageTime = kind === 'storage' && sample?.storageTimestamp
    ? `\n存储采样：${new Date(sample.storageTimestamp).toLocaleTimeString('zh-CN', { hour12: false })}` : ''
  return {
    percent: primary === '—' ? null : percent ?? null,
    machinePercent: secondary === '—' ? null : machinePercent ?? null,
    primary: primary.replace('%', ''),
    secondary: secondary.replace('%', ''),
    unit: primary === '—' && secondary === '—' ? '' : '%',
    accessible: `${scopeLabel}${label} ${primary}，整机当前占用 ${secondary}`,
    detail: `${label}\n${scopeLabel}：${resourceAmount(kind, sample, machine)}\n整机已用：${resourceAmount(kind, machine)}\n${kind === 'storage' ? '数据盘容量' : '整机总量'}：${capacity}${storageTime}`,
  }
}
