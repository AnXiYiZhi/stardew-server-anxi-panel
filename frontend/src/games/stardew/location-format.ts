export type StardewLocationLike = {
  location?: string
  locationName?: string
  locationDisplayName?: string
  tileX?: number
  tileY?: number
}

const UNIQUE_LOCATION_SUFFIX = /(?:_?\d+|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$/i

const CORE_LOCATION_ZH: Record<string, string> = {
  Farm: '农场', FarmHouse: '农舍', Cabin: '小屋', Cellar: '地窖', Shed: '小屋', Barn: '畜棚', Coop: '鸡舍',
  Town: '鹈鹕镇', Beach: '海滩', Forest: '煤矿森林', Mountain: '山区', Mine: '矿井', UndergroundMine: '矿井', VolcanoDungeon: '火山地牢',
}

const INSTANCE_LOCATION_BASES = ['FarmHouse', 'Cabin', 'Cellar', 'Shed', 'Barn', 'Coop', 'VolcanoDungeon', 'UndergroundMine']

export function normalizeStardewLocationKey(value?: string): string {
  const raw = (value ?? '').trim()
  if (!raw) return ''
  for (const base of INSTANCE_LOCATION_BASES) {
    if (raw.toLowerCase() === base.toLowerCase()) return base
    const suffix = raw.slice(base.length)
    if (raw.slice(0, base.length).toLowerCase() === base.toLowerCase() && suffix && UNIQUE_LOCATION_SUFFIX.test(suffix)) return base
  }
  return raw
}

export function rawStardewLocation(location: StardewLocationLike): string {
  return location.locationName || location.location || location.locationDisplayName || '—'
}

export function readableStardewLocation(location: StardewLocationLike, labels: Record<string, string> = CORE_LOCATION_ZH, fallback = '—'): string {
  for (const raw of [location.locationDisplayName, location.locationName, location.location]) {
    const exact = (raw ?? '').trim()
    if (exact && (labels[exact] || CORE_LOCATION_ZH[exact])) return labels[exact] ?? CORE_LOCATION_ZH[exact]
    const key = normalizeStardewLocationKey(raw)
    if (!key) continue
    const mapped = labels[key] ?? CORE_LOCATION_ZH[key]
    if (mapped) return mapped
    if (raw === location.locationDisplayName) return key
  }
  return normalizeStardewLocationKey(location.locationName || location.location) || fallback
}

export function formatStardewLocation(location: StardewLocationLike, options: { labels?: Record<string, string>; fallback?: string; coordinates?: boolean } = {}): string {
  const name = readableStardewLocation(location, options.labels, options.fallback)
  if (options.coordinates !== false && typeof location.tileX === 'number' && typeof location.tileY === 'number') return `${name} (${location.tileX}, ${location.tileY})`
  return name
}
