import type { ResourceMetricSample } from '../../types'

export const RESOURCE_SERIES = [
  { key: 'cpuPercent', label: 'CPU', color: '#648044' },
  { key: 'memoryPercent', label: '内存', color: '#b47c28' },
  { key: 'diskPercent', label: '磁盘', color: '#4c8582' },
] as const

export type ResourceKey = typeof RESOURCE_SERIES[number]['key']

export function resourceValue(sample: ResourceMetricSample | undefined, key: ResourceKey): number | null {
  if (!sample || (key !== 'diskPercent' && !sample.containerRunning)) return null
  const value = sample[key]
  if (value == null || !Number.isFinite(value) || value < 0) return null
  return key === 'cpuPercent' ? value : Math.min(100, value)
}

export function resourceHistory(samples: ResourceMetricSample[]) {
  const unique = new Map<number, ResourceMetricSample>()
  for (const sample of samples) {
    const time = Date.parse(sample.timestamp)
    if (Number.isFinite(time)) unique.set(time, sample)
  }
  return [...unique].sort(([a], [b]) => a - b).map(([, sample]) => sample)
}

// A missing reading or a long sampling gap must break the line.
export function resourceSegments(samples: ResourceMetricSample[], key: ResourceKey) {
  const segments: { time: number; value: number }[][] = []
  let current: { time: number; value: number }[] = []
  for (const sample of samples) {
    const time = Date.parse(sample.timestamp)
    const value = resourceValue(sample, key)
    if (value == null || (current.length && time - current[current.length - 1].time > 24_000)) {
      if (current.length) segments.push(current)
      current = []
    }
    if (value != null) current.push({ time, value })
  }
  if (current.length) segments.push(current)
  return segments
}
