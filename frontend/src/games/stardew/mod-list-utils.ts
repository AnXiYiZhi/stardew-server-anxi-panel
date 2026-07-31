import type { ModInfo } from '../../types'

export type InstalledModSort = 'installed-desc' | 'name-asc' | 'name-desc'

export const INSTALLED_MOD_SORT_OPTIONS: Array<{ value: InstalledModSort; label: string }> = [
  { value: 'installed-desc', label: '最近安装' },
  { value: 'name-asc', label: '名称 A–Z' },
  { value: 'name-desc', label: '名称 Z–A' },
]

const CONTENT_PACK_PREFIXES: Record<string, string> = {
  'pathoschild.contentpatcher': '[CP]',
  'esca.farmtypemanager': '[FTM]',
}

function modSortDisplayName(mod: ModInfo) {
  const name = mod.name || mod.folderName || mod.uniqueId || mod.id
  if (!mod.isContentPack) return name
  const contentPackFor = (mod.contentPackFor ?? '').trim().toLowerCase()
  const folderPrefix = mod.folderName.match(/^\[([a-z0-9-]{1,8})\]\s*/i)?.[0]?.trim()
  const prefix = CONTENT_PACK_PREFIXES[contentPackFor] ?? folderPrefix
  if (!prefix || name.toLowerCase().startsWith(prefix.toLowerCase())) return name
  return `${prefix} ${name}`
}

function normalizeSearchValue(value: unknown) {
  return String(value ?? '').normalize('NFKC').toLocaleLowerCase('zh-Hans').trim()
}

function compactSearchValue(value: string) {
  return value.replace(/[\s._\-()[\]{}:：/\\]+/g, '')
}

function searchableModText(mod: ModInfo) {
  const nexusIds = [mod.nexusModId, mod.originNexusModId]
    .filter((id): id is number => typeof id === 'number' && id > 0)
  return normalizeSearchValue([
    mod.id,
    mod.uniqueId,
    mod.name,
    mod.folderName,
    mod.author,
    mod.packageName,
    mod.originModName,
    ...nexusIds.flatMap((id) => [String(id), `nexus:${id}`]),
  ].filter(Boolean).join('\n'))
}

export function modMatchesInstalledQuery(mod: ModInfo, query: string) {
  const tokens = normalizeSearchValue(query).split(/\s+/).filter(Boolean)
  if (tokens.length === 0) return true
  const text = searchableModText(mod)
  const compactText = compactSearchValue(text)
  return tokens.every((token) => {
    if (text.includes(token)) return true
    const compactToken = compactSearchValue(token)
    return compactToken.length > 0 && compactText.includes(compactToken)
  })
}

function builtInRank(mod: ModInfo) {
  const uniqueId = normalizeSearchValue(mod.uniqueId)
  if (uniqueId === 'pathoschild.smapi' || mod.id === '__smapi_runtime') return 0
  if (uniqueId === 'stardewanxipanel.control') return 1
  if (uniqueId === 'junimohost.server') return 2
  return 3
}

function installedTime(mod: ModInfo) {
  if (!mod.installedAt) return Number.NEGATIVE_INFINITY
  const parsed = Date.parse(mod.installedAt)
  return Number.isFinite(parsed) ? parsed : Number.NEGATIVE_INFINITY
}

function bundleKey(mod: ModInfo) {
  if (mod.packageKey) return `package:${mod.packageKey}`
  if (mod.originSource === 'nexus' && (mod.originNexusModId ?? 0) > 0) return `nexus:${mod.originNexusModId}`
  if ((mod.nexusModId ?? 0) > 0) return `nexus:${mod.nexusModId}`
  return `single:${normalizeSearchValue(modSortDisplayName(mod))}:${normalizeSearchValue(mod.folderName)}`
}

function bundleRank(mod: ModInfo) {
  if ((mod.nexusModId ?? 0) > 0) return 0
  if (mod.originSource === 'nexus' && (mod.originNexusModId ?? 0) > 0) return 1
  return 2
}

function compareNames(a: ModInfo, b: ModInfo) {
  const nameDiff = modSortDisplayName(a).localeCompare(modSortDisplayName(b), 'zh-Hans', {
    numeric: true,
    sensitivity: 'base',
  })
  if (nameDiff !== 0) return nameDiff
  return a.folderName.localeCompare(b.folderName, 'zh-Hans', { numeric: true, sensitivity: 'base' })
}

export function sortInstalledMods(mods: ModInfo[], sort: InstalledModSort = 'installed-desc') {
  return [...mods].sort((a, b) => {
    if (a.builtIn !== b.builtIn) return a.builtIn ? -1 : 1
    if (a.builtIn && b.builtIn) {
      const rankDiff = builtInRank(a) - builtInRank(b)
      if (rankDiff !== 0) return rankDiff
    }
    if (sort === 'name-asc' || sort === 'name-desc') {
      const nameDiff = compareNames(a, b)
      return sort === 'name-desc' ? -nameDiff : nameDiff
    }

    const timeDiff = installedTime(b) - installedTime(a)
    if (Number.isFinite(timeDiff) && timeDiff !== 0) return timeDiff
    if (installedTime(a) !== installedTime(b)) return installedTime(a) > installedTime(b) ? -1 : 1

    const bundleA = bundleKey(a)
    const bundleB = bundleKey(b)
    const bundleDiff = bundleA.localeCompare(bundleB, 'zh-Hans', { numeric: true, sensitivity: 'base' })
    if (bundleDiff !== 0) return bundleDiff
    const rankDiff = bundleRank(a) - bundleRank(b)
    if (rankDiff !== 0) return rankDiff
    return compareNames(a, b)
  })
}

export function filterAndSortInstalledMods(mods: ModInfo[], query: string, sort: InstalledModSort = 'installed-desc') {
  return sortInstalledMods(mods.filter((mod) => modMatchesInstalledQuery(mod, query)), sort)
}
