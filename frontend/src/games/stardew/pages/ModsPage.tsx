import { useCallback, useEffect, useRef, useState } from 'react'
import { useMemo } from 'react'
import type { CSSProperties, MouseEvent, ReactNode } from 'react'
import { downloadNexusInstallerExtension, searchNexusMods, getNexusSettings, saveNexusAPIKey, deleteNexusAPIKey, getJob } from '../../../api'
import { errorMessage, formatDate } from '../../../core/helpers'
import type { ModInfo, ModSearchResult, ModSyncKind, NexusModSearchResult, NexusRequiredMod, NexusSettingsStatus } from '../../../types'
import { modIsJunimoServer, modIsPanelControl, modIsSmapi, modIsSystemRuntime } from '../mod-visibility'
import { modDisplayName } from '../mod-display'
import { routeToPath, type StardewPageProps } from '../stardew-routes'
import { useModsManagement } from '../useModsManagement'

type ModWorkbenchTab = 'download' | 'installed' | 'settings'

const NEXUS_SEARCH_PAGE_SIZE_DEFAULT = 8
const NEXUS_SEARCH_PAGE_SIZE_MIN = 1
const NEXUS_SEARCH_PAGE_SIZE_MAX = 20
const NEXUS_SEARCH_CARD_HEIGHT = 198
const NEXUS_SEARCH_CARD_GAP = 12
const NEXUS_SEARCH_BOTTOM_RESERVE = 8
const SMAPI_NEXUS_MOD_ID = 2400
const SMAPI_NEXUS_URL = `https://www.nexusmods.com/stardewvalley/mods/${SMAPI_NEXUS_MOD_ID}`
const NEXUS_EXTENSION_PANEL_SOURCE = 'ANXI_PANEL_NEXUS_INSTALL'
const NEXUS_EXTENSION_SOURCE = 'ANXI_NEXUS_INSTALLER'
const NEXUS_EXTENSION_INSTANCE_ID = 'stardew'
const NEXUS_SEARCH_SESSION_KEY = 'stardew-anxi:nexus-search-state:v1'
const NEXUS_EXTENSION_SESSION_KEY = 'stardew-anxi:nexus-extension-install:v1'
const NEXUS_QUICK_TAGS = ['UI Info', 'Fishing Mod', 'Backpack Upgrades', 'Tractor']

type NexusExtensionConnectionStatus = 'unknown' | 'checking' | 'connected' | 'disconnected'

type NexusExtensionConnectionState = {
  status: NexusExtensionConnectionStatus
  message: string
  panelBaseUrl: string
}

type NexusExtensionBatchItemStatus = 'pending' | 'opening' | 'capturing' | 'ready' | 'posting' | 'queued' | 'done' | 'failed'

type NexusExtensionBatchItem = {
  id: string
  role: 'target' | 'required'
  modId: number
  name: string
  url: string
  status: NexusExtensionBatchItemStatus
  message?: string
  jobId?: string
}

type NexusExtensionBatch = {
  id: string
  status: 'running' | 'done' | 'failed'
  progress: number
  items: NexusExtensionBatchItem[]
}

type NexusExtensionInstallState = {
  modId: number
  batchId: string
  status: 'starting' | 'running' | 'done' | 'failed'
  progress: number
  error?: string
  errorItemName?: string
  errorJobId?: string
  items?: NexusExtensionBatchItem[]
}

type NexusSearchSessionState = {
  query: string
  results: NexusModSearchResult[] | null
  page: number
  pageSize: number
  total: number
  hasMore: boolean
  pageInput: string
  updatedAt: number
}

const SYNC_KIND_LABELS: Record<ModSyncKind, string> = {
  server_only: '服务器专用',
  client_required: '玩家需同步',
  unknown: '待确认',
}

const SYNC_KIND_TAG_CLASS: Record<ModSyncKind, string> = {
  server_only: 'sd-tag-blue',
  client_required: 'sd-tag-green',
  unknown: 'sd-tag-gold',
}

const DEPENDENCY_LABELS: Record<string, string> = {
  'Pathoschild.ContentPatcher': 'Content Patcher',
  'spacechase0.GenericModConfigMenu': 'Generic Mod Config Menu',
  'Cherry.ShopTileFramework': 'Shop Tile Framework',
  'tlitookilakin.HDPortraits': 'HD Portraits',
}

function dependencyLabel(uniqueId: string) {
  const knownLabel = DEPENDENCY_LABELS[uniqueId]
  if (knownLabel) return knownLabel

  const parts = uniqueId.split('.').filter(Boolean)
  const nameParts = parts.length > 1 ? parts.slice(1) : parts
  return nameParts
    .join(' ')
    .replace(/[_-]+/g, ' ')
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/\s+/g, ' ')
    .trim() || uniqueId
}

function dependencyRequirementText(dep: NonNullable<ModInfo['dependencies']>[number]) {
  const name = dependencyLabel(dep.uniqueId)
  const minimum = dep.minimumVersion ? ` >= ${dep.minimumVersion}` : ''
  const current = dep.installedVersion ? `（当前 ${dep.installedVersion}）` : ''
  return `${name}${minimum}${current}`
}

function dependencyIssueText(dep: NonNullable<ModInfo['dependencies']>[number]) {
  const requirement = dependencyRequirementText(dep)
  switch (dep.status) {
    case 'missing':
      return `缺失：${requirement}`
    case 'disabled':
      return `未启用：${requirement}`
    case 'version_mismatch':
      return `版本不足：${requirement}`
    case 'unknown_version':
      return `版本待确认：${requirement}`
    case 'optional_missing':
      return `可选缺失：${requirement}`
    case 'optional_disabled':
      return `可选未启用：${requirement}`
    case 'optional_version_mismatch':
      return `可选版本不足：${requirement}`
    case 'optional_unknown_version':
      return `可选版本待确认：${requirement}`
    default:
      return requirement
  }
}

function dependencyDisplay(mod: ModInfo) {
  const required = (mod.dependencies ?? []).filter((dep) => dep.required && dep.uniqueId)
  if (required.length === 0) return null
  const issues = required.filter((dep) => dep.status && dep.status !== 'satisfied')
  if (issues.length > 0) {
    const statusPriority = ['missing', 'disabled', 'version_mismatch', 'unknown_version']
    const primaryStatus = statusPriority.find((status) => issues.some((dep) => dep.status === status)) ?? issues[0].status
    const primaryIssues = issues.filter((dep) => dep.status === primaryStatus)
    const names = primaryIssues.map((dep) => dependencyLabel(dep.uniqueId))
    const labelPrefix = primaryStatus === 'missing'
      ? '缺失前置'
      : primaryStatus === 'disabled'
        ? '前置未启用'
        : primaryStatus === 'version_mismatch'
          ? '前置版本不足'
          : '前置版本待确认'
    return {
      label: names.length <= 2
        ? `${labelPrefix}：${names.join('、')}`
        : `${labelPrefix}：${names.slice(0, 2).join('、')} 等 ${names.length} 个`,
      title: `前置依赖检查：${issues.map(dependencyIssueText).join('、')}`,
      className: primaryStatus === 'unknown_version' ? 'sd-tag-gold' : 'sd-tag-red',
    }
  }
  const names = required.map((dep) => dependencyLabel(dep.uniqueId))
  const titleLabels = required.map((dep) => (
    dependencyRequirementText(dep)
  ))
  if (names.length <= 2) {
    return {
      label: `前置：${names.join('、')}`,
      title: `需要前置依赖：${titleLabels.join('、')}`,
      className: 'sd-tag-gold',
    }
  }
  return {
    label: `前置：${names.slice(0, 2).join('、')} 等 ${names.length} 个`,
    title: `需要前置依赖：${titleLabels.join('、')}`,
    className: 'sd-tag-gold',
  }
}

function installedStatusLabel(result: ModSearchResult) {
  const version = result.installedVersion ? ` v${result.installedVersion}` : ''
  if (result.installedEnabled === false) return `已安装但未启用${version}`
  return `已安装${version}`
}

function installedStatusTitle(result: ModSearchResult) {
  const folder = result.installedFolderName ? `文件夹：${result.installedFolderName}` : ''
  if (result.installedEnabled === false) {
    return folder ? `${folder}；当前存档未启用` : '当前存档未启用'
  }
  return folder
}

function clampNexusPageSize(value: number) {
  if (!Number.isFinite(value)) return NEXUS_SEARCH_PAGE_SIZE_DEFAULT
  return Math.min(NEXUS_SEARCH_PAGE_SIZE_MAX, Math.max(NEXUS_SEARCH_PAGE_SIZE_MIN, Math.trunc(value)))
}

function countGridColumns(gridTemplateColumns: string) {
  if (!gridTemplateColumns || gridTemplateColumns === 'none') return 1
  return Math.max(1, gridTemplateColumns.split(' ').filter((part) => part.trim() && part !== '0px').length)
}

function ModSearchResultCard({
  result,
  actionSlot,
  statsSlot,
  footerSlot,
  className,
}: {
  result: ModSearchResult
  actionSlot?: ReactNode
  statsSlot?: ReactNode
  footerSlot?: ReactNode
  className?: string
}) {
  const cardClassName = ['sd-mods-nexus-card', className].filter(Boolean).join(' ')

  return (
    <div className={cardClassName}>
      <div className="sd-mods-nexus-card-pic-wrap">
        {result.pictureUrl ? (
          <img className="sd-mods-nexus-card-pic" src={result.pictureUrl} alt="" />
        ) : (
          <div className="sd-mods-nexus-card-pic sd-mods-nexus-card-pic-empty" aria-hidden="true">
            {result.source === 'nexus' || result.source === 'nexus_package' ? 'NEXUS' : 'MOD'}
          </div>
        )}
      </div>
      <div className="sd-mods-nexus-card-main">
        <div className="sd-mods-nexus-card-header">
          <span className="sd-mods-nexus-card-name">{result.name}</span>
          <span className="sd-tag sd-tag-blue">来源：{result.sourceName}</span>
          {result.source === 'nexus' && result.sourceModId ? (
            <span className="sd-tag sd-tag-blue">Nexus:{result.sourceModId}</span>
          ) : null}
          {result.sourceDetail ? (
            <span className="sd-tag sd-tag-gold" title={result.sourceDetail}>{result.sourceDetail}</span>
          ) : null}
          {result.installed ? (
            <span
              className={`sd-tag ${result.installedEnabled === false ? 'sd-tag-gold' : 'sd-tag-green'}`}
              title={installedStatusTitle(result)}
            >
              {installedStatusLabel(result)}
            </span>
          ) : null}
        </div>
        <div className="sd-mods-nexus-card-meta">
          {result.author ? <span className="sd-mods-meta-item"><span className="sd-mods-meta-label">作者</span>{result.author}</span> : null}
          {result.version ? <span className="sd-mods-meta-item"><span className="sd-mods-meta-label">版本</span>{result.version}</span> : null}
          {result.updatedAt ? <span className="sd-mods-meta-item"><span className="sd-mods-meta-label">更新于</span>{formatDate(result.updatedAt)}</span> : null}
        </div>
        {result.summary ? <div className="sd-mods-meta-desc">{result.summary}</div> : null}
        <div className="sd-mods-nexus-card-stats">
          <span>下载 {(result.downloadCount ?? 0).toLocaleString()}</span>
          {result.endorsementCount !== undefined ? <span>认可 {result.endorsementCount.toLocaleString()}</span> : null}
          {statsSlot}
        </div>
      </div>
      <div className="sd-mods-card-actions">
        <button
          className="sd-btn-tan"
          type="button"
          disabled={!result.pageUrl}
          onClick={() => result.pageUrl && window.open(result.pageUrl, '_blank', 'noopener,noreferrer')}
        >
          {result.externalLabel}
        </button>
        {actionSlot}
      </div>
      {footerSlot ? <div className="sd-mods-nexus-card-footer">{footerSlot}</div> : null}
    </div>
  )
}

function nexusResultToSearchResult(result: NexusModSearchResult): ModSearchResult {
  return {
    id: `nexus:${result.modId}`,
    source: 'nexus',
    sourceName: 'Nexus',
    sourceModId: String(result.modId),
    name: result.name,
    summary: result.summary,
    author: result.author,
    version: result.version,
    updatedAt: result.updatedAt,
    endorsementCount: result.endorsementCount,
    downloadCount: result.downloadCount,
    pictureUrl: result.pictureUrl,
    pageUrl: result.nexusUrl,
    externalLabel: '跳转 N站',
    installMethod: 'nexus_extension',
    installLabel: '一键安装',
    nexusModId: result.modId,
    installed: result.installed,
    installedEnabled: result.installedEnabled,
    installedFolderName: result.installedFolderName,
    installedVersion: result.installedVersion,
  }
}

function missingNexusRequiredMods(result: NexusModSearchResult) {
  return (result.requiredMods ?? []).filter((required) => !required.installed || required.installedEnabled === false)
}

function nexusRequiredStatusLabel(required: NexusRequiredMod) {
  const version = required.installedVersion ? ` v${required.installedVersion}` : ''
  if (!required.installed) return '缺少前置'
  if (required.installedEnabled === false) return `前置未启用${version}`
  return `前置已安装${version}`
}

function nexusRequiredStatusClass(required: NexusRequiredMod) {
  if (!required.installed) return 'sd-tag-red'
  if (required.installedEnabled === false) return 'sd-tag-gold'
  return 'sd-tag-green'
}

function NexusRequiredModsBadge({
  requiredMods,
  missingRequiredMods,
  isOpen,
  onToggle,
}: {
  requiredMods: NexusRequiredMod[]
  missingRequiredMods: NexusRequiredMod[]
  isOpen: boolean
  onToggle: () => void
}) {
  if (requiredMods.length === 0) return null

  const hasMissing = missingRequiredMods.length > 0
  const label = hasMissing ? '缺少前置mod' : '前置已满足'
  const detailTitle = requiredMods
    .map((required) => `${required.name} (NexusId:${required.modId})`)
    .join('、')

  function handleToggle(event: MouseEvent<HTMLButtonElement>) {
    event.stopPropagation()
    onToggle()
  }

  return (
    <div className={`sd-mods-dependency-details${isOpen ? ' sd-mods-dependency-details-open' : ''}`}>
      <button
        type="button"
        className={`sd-tag ${hasMissing ? 'sd-tag-red' : 'sd-tag-green'} sd-mods-dependency-summary`}
        title={detailTitle}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        onClick={handleToggle}
      >
        {label}
      </button>
      {isOpen ? (
        <div className="sd-mods-dependency-popover" role="dialog" aria-label={label}>
          <div className="sd-mods-dependency-popover-title">{label}</div>
          <ul className="sd-mods-dependency-list">
            {requiredMods.map((required) => (
              <li className="sd-mods-dependency-list-item" key={`required-${required.modId}`}>
                <span className="sd-mods-dependency-name">{required.name}</span>
                <span className="sd-mods-dependency-id">NexusId: {required.modId}</span>
                <span className={`sd-tag ${nexusRequiredStatusClass(required)} sd-mods-dependency-state`}>
                  {nexusRequiredStatusLabel(required)}
                </span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  )
}

function nexusExtensionInstallURL(result: NexusModSearchResult): string {
  const fallback = `https://www.nexusmods.com/stardewvalley/mods/${result.modId}`
  try {
    const url = new URL(result.nexusUrl || fallback)
    url.searchParams.set('tab', 'files')
    url.searchParams.set('anxi_auto', '1')
    return url.toString()
  } catch {
    return `${fallback}?tab=files&anxi_auto=1`
  }
}

function nexusRequiredInstallURL(required: NexusRequiredMod): string {
  const fallback = `https://www.nexusmods.com/stardewvalley/mods/${required.modId}`
  try {
    const url = new URL(required.nexusUrl || fallback)
    url.searchParams.set('tab', 'files')
    url.searchParams.set('anxi_auto', '1')
    return url.toString()
  } catch {
    return `${fallback}?tab=files&anxi_auto=1`
  }
}

function readNexusSearchSessionState(): NexusSearchSessionState | null {
  try {
    const raw = window.sessionStorage.getItem(NEXUS_SEARCH_SESSION_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<NexusSearchSessionState>
    if (!parsed || typeof parsed !== 'object') return null
    return {
      query: typeof parsed.query === 'string' ? parsed.query : '',
      results: Array.isArray(parsed.results) ? parsed.results as NexusModSearchResult[] : null,
      page: Number.isFinite(parsed.page) ? Number(parsed.page) : 1,
      pageSize: clampNexusPageSize(Number(parsed.pageSize ?? NEXUS_SEARCH_PAGE_SIZE_DEFAULT)),
      total: Number.isFinite(parsed.total) ? Number(parsed.total) : 0,
      hasMore: Boolean(parsed.hasMore),
      pageInput: typeof parsed.pageInput === 'string' ? parsed.pageInput : String(parsed.page ?? 1),
      updatedAt: Number.isFinite(parsed.updatedAt) ? Number(parsed.updatedAt) : Date.now(),
    }
  } catch {
    return null
  }
}

function writeNexusSearchSessionState(state: NexusSearchSessionState) {
  try {
    window.sessionStorage.setItem(NEXUS_SEARCH_SESSION_KEY, JSON.stringify(state))
  } catch {
    // Session restore is best-effort only.
  }
}

function readNexusExtensionSessionState(): NexusExtensionInstallState | null {
  try {
    const raw = window.sessionStorage.getItem(NEXUS_EXTENSION_SESSION_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<NexusExtensionInstallState>
    if (!parsed || typeof parsed !== 'object' || !parsed.modId || !parsed.batchId) return null
    return {
      modId: Number(parsed.modId),
      batchId: String(parsed.batchId),
      status: parsed.status === 'done' || parsed.status === 'failed' || parsed.status === 'running' || parsed.status === 'starting'
        ? parsed.status
        : 'running',
      progress: Number.isFinite(parsed.progress) ? Number(parsed.progress) : 0,
      error: typeof parsed.error === 'string' ? parsed.error : undefined,
      items: Array.isArray(parsed.items) ? parsed.items as NexusExtensionBatchItem[] : undefined,
    }
  } catch {
    return null
  }
}

function writeNexusExtensionSessionState(state: NexusExtensionInstallState | null) {
  try {
    if (state) {
      window.sessionStorage.setItem(NEXUS_EXTENSION_SESSION_KEY, JSON.stringify(state))
    } else {
      window.sessionStorage.removeItem(NEXUS_EXTENSION_SESSION_KEY)
    }
  } catch {
    // Session restore is best-effort only.
  }
}

function modToSearchResult(mod: ModInfo): ModSearchResult {
  if (mod.builtIn) {
    const isSmapi = modIsSmapi(mod)
    if (isSmapi) {
      return {
        id: `nexus:${SMAPI_NEXUS_MOD_ID}`,
        source: 'nexus',
        sourceName: 'Nexus',
        sourceModId: String(SMAPI_NEXUS_MOD_ID),
        name: 'SMAPI - Stardew Modding API',
        summary: mod.description,
        author: mod.author,
        version: mod.version,
        updatedAt: mod.updatedAt,
        endorsementCount: mod.endorsementCount ?? 0,
        downloadCount: mod.downloadCount ?? 0,
        pictureUrl: mod.pictureUrl,
        pageUrl: SMAPI_NEXUS_URL,
        externalLabel: '跳转 N站',
        installMethod: 'manual',
        installLabel: '玩家自行安装',
        nexusModId: SMAPI_NEXUS_MOD_ID,
        installed: true,
        installedEnabled: true,
        installedFolderName: mod.folderName,
        installedVersion: mod.version,
      }
    }
    return {
      id: `builtin:${mod.id}`,
      source: 'builtin',
      sourceName: '内置组件',
      sourceModId: mod.uniqueId ?? mod.folderName,
      name: modDisplayName(mod),
      summary: mod.description,
      author: mod.author,
      version: mod.version,
      updatedAt: mod.updatedAt,
      endorsementCount: 0,
      downloadCount: 0,
      pictureUrl: mod.pictureUrl,
      pageUrl: '',
      externalLabel: '内置组件',
      installMethod: 'manual',
      installLabel: isSmapi ? '玩家自行安装' : '内置组件',
      installed: true,
      installedEnabled: true,
      installedFolderName: mod.folderName,
      installedVersion: mod.version,
    }
  }
  const modId = mod.nexusModId ?? 0
  const originNexusModId = mod.originSource === 'nexus' ? (mod.originNexusModId ?? 0) : 0
  const hasNexusPackageOrigin = modId === 0 && originNexusModId > 0
  const ownNexusUrl = modId > 0 ? `https://www.nexusmods.com/stardewvalley/mods/${modId}` : ''
  const originNexusUrl = hasNexusPackageOrigin
    ? mod.originModUrl ?? `https://www.nexusmods.com/stardewvalley/mods/${originNexusModId}`
    : ''
  const pageUrl = (mod.nexusUrl ?? ownNexusUrl) || originNexusUrl
  return {
    id: modId > 0
      ? `nexus:${modId}`
      : (hasNexusPackageOrigin ? `nexus-package:${originNexusModId}:${mod.id}` : `local:${mod.id}`),
    source: modId > 0 ? 'nexus' : (hasNexusPackageOrigin ? 'nexus_package' : 'local'),
    sourceName: modId > 0 ? 'N站' : (hasNexusPackageOrigin ? 'N站包' : '本地'),
    sourceModId: modId > 0 ? String(modId) : (hasNexusPackageOrigin ? String(originNexusModId) : mod.folderName),
    sourceDetail: hasNexusPackageOrigin ? `随 ${mod.originModName || 'Nexus 安装包'} 安装` : undefined,
    name: modDisplayName(mod),
    summary: mod.nexusSummary ?? mod.description,
    author: mod.author,
    version: mod.version,
    updatedAt: mod.updatedAt,
    endorsementCount: mod.endorsementCount ?? 0,
    downloadCount: mod.downloadCount ?? 0,
    pictureUrl: mod.pictureUrl,
    pageUrl,
    externalLabel: modId > 0 ? '跳转 N站' : (hasNexusPackageOrigin ? '跳转 N站包' : '本地 Mod'),
    installMethod: 'none',
    installLabel: '已安装',
    nexusModId: modId || undefined,
    installed: true,
    installedEnabled: mod.enabled,
    installedFolderName: mod.folderName,
    installedVersion: mod.version,
  }
}

function modCountsForPlayerSync(mod: ModInfo) {
  if (modIsSmapi(mod)) return true
  return !mod.builtIn
}

function builtInRank(mod: ModInfo) {
  if (modIsSmapi(mod)) return 0
  if (modIsPanelControl(mod)) return 1
  if (modIsJunimoServer(mod)) return 2
  return 3
}

function modBundleKey(mod: ModInfo) {
  if (mod.packageKey) {
    return `package:${mod.packageKey}`
  }
  if (mod.originSource === 'nexus' && (mod.originNexusModId ?? 0) > 0) {
    return `nexus:${mod.originNexusModId}`
  }
  if ((mod.nexusModId ?? 0) > 0) {
    return `nexus:${mod.nexusModId}`
  }
  return ''
}

function modBundleRank(mod: ModInfo) {
  if ((mod.nexusModId ?? 0) > 0) return 0
  if (mod.originSource === 'nexus' && (mod.originNexusModId ?? 0) > 0) return 1
  return 2
}

function sortInstalledMods(mods: ModInfo[]) {
  return [...mods].sort((a, b) => {
    if (a.builtIn !== b.builtIn) return a.builtIn ? -1 : 1
    if (a.builtIn && b.builtIn) {
      const rankDiff = builtInRank(a) - builtInRank(b)
      if (rankDiff !== 0) return rankDiff
    }
    const bundleA = modBundleKey(a)
    const bundleB = modBundleKey(b)
    if (bundleA && bundleB && bundleA !== bundleB) return bundleA.localeCompare(bundleB)
    if (bundleA && bundleA === bundleB) {
      const rankDiff = modBundleRank(a) - modBundleRank(b)
      if (rankDiff !== 0) return rankDiff
    }
    if (bundleA && !bundleB) return -1
    if (!bundleA && bundleB) return 1
    return modDisplayName(a).localeCompare(modDisplayName(b), 'zh-Hans')
  })
}

function modBundleMembers(mods: ModInfo[], target: ModInfo) {
  const bundleKey = modBundleKey(target)
  if (!bundleKey) return [target]
  const members = mods.filter((mod) => !mod.builtIn && modBundleKey(mod) === bundleKey)
  return members.length > 0 ? members : [target]
}

export function ModsPage({ user, instanceState, dashboardData }: StardewPageProps) {
  const restoredNexusSearchState = readNexusSearchSessionState()
  const restoredNexusExtensionState = readNexusExtensionSessionState()
  const [activeTab, setActiveTab] = useState<ModWorkbenchTab>('download')

  const [nexusQuery, setNexusQuery] = useState(restoredNexusSearchState?.query ?? '')
  const [nexusLoading, setNexusLoading] = useState(false)
  const [nexusError, setNexusError] = useState<string | null>(null)
  const [nexusResults, setNexusResults] = useState<NexusModSearchResult[] | null>(restoredNexusSearchState?.results ?? null)
  const [nexusPage, setNexusPage] = useState(restoredNexusSearchState?.page ?? 1)
  const [nexusPageSize, setNexusPageSize] = useState(clampNexusPageSize(restoredNexusSearchState?.pageSize ?? NEXUS_SEARCH_PAGE_SIZE_DEFAULT))
  const [nexusTotal, setNexusTotal] = useState(restoredNexusSearchState?.total ?? 0)
  const [nexusHasMore, setNexusHasMore] = useState(restoredNexusSearchState?.hasMore ?? false)
  const [nexusPageInput, setNexusPageInput] = useState(restoredNexusSearchState?.pageInput ?? '1')
  const [nexusSettings, setNexusSettings] = useState<NexusSettingsStatus | null>(null)
  const [nexusSettingsLoading, setNexusSettingsLoading] = useState(false)
  const [showNexusKeyModal, setShowNexusKeyModal] = useState(false)
  const [nexusKeyInput, setNexusKeyInput] = useState('')
  const [nexusKeyBusy, setNexusKeyBusy] = useState(false)
  const [nexusKeyError, setNexusKeyError] = useState<string | null>(null)
  const [nexusKeyMessage, setNexusKeyMessage] = useState<string | null>(null)
  const [nexusExtensionPackBusy, setNexusExtensionPackBusy] = useState(false)
  const [nexusInstallError, setNexusInstallError] = useState<string | null>(null)
  const [openNexusRequiredModId, setOpenNexusRequiredModId] = useState<number | null>(null)
  const [nexusExtensionInstall, setNexusExtensionInstall] = useState<NexusExtensionInstallState | null>(restoredNexusExtensionState)
  const [nexusExtensionConnection, setNexusExtensionConnection] = useState<NexusExtensionConnectionState>({
    status: 'unknown',
    message: '尚未检测浏览器扩展',
    panelBaseUrl: window.location.origin,
  })

  const nexusResultsListRef = useRef<HTMLDivElement>(null)
  const lastNexusSearchPageSizeRef = useRef(
    restoredNexusSearchState?.results
      ? clampNexusPageSize(restoredNexusSearchState.results.length || restoredNexusSearchState.pageSize)
      : nexusPageSize,
  )
  const nexusExtensionPollRef = useRef<number | null>(null)
  const nexusExtensionTimeoutRef = useRef<number | null>(null)
  const nexusExtensionInstallRef = useRef<NexusExtensionInstallState | null>(null)
  const nexusExtensionKnownJobIdsRef = useRef<Set<string>>(new Set())
  const defaultNexusLoadedRef = useRef(Boolean(restoredNexusSearchState?.results))

  const isAdmin = user.role === 'admin'
  const state = instanceState?.state ?? null
  const isRunning = state === 'running' || state === 'starting'
  const writeDisabled = isRunning || !isAdmin
  const writeTitle = !isAdmin
    ? '仅管理员可用'
    : isRunning
      ? '服务器运行中，请先停止后操作'
      : ''
  const activeSaveName = dashboardData.saves?.activeSaveName ?? ''
  const {
    data, loading, listError, showUpload, uploadFiles, setUploadFiles, uploadBusy, uploadError,
    uploadSuccess, confirmDelete, deleteBusy, deleteError, exportBusy, exportError, syncUpdating, syncError,
    syncPackBusy, syncPackError, enableUpdating, enableError, fileInputRef, loadMods, openUpload, closeUpload,
    handleUpload, openDeleteConfirm, closeDeleteConfirm, handleDeleteConfirm, handleExport, handleSyncChange,
    handleEnabledChange, handleSyncPackExport,
  } = useModsManagement({ dashboardData, activeSaveName })

  const {
    mods,
    userVisibleMods,
    parseErrorCount,
    syncSummary,
    syncPackagedClientRequired,
  } = useMemo(() => {
    const sortedMods = sortInstalledMods(data?.mods ?? [])
    const visibleMods = sortedMods.filter((mod) => !modIsSystemRuntime(mod))
    let parseErrors = 0
    let serverOnly = 0
    let clientRequired = 0
    let unknown = 0
    let packagedClientRequired = 0

    for (const mod of visibleMods) {
      if (mod.parseError) {
        parseErrors += 1
      }
    }

    for (const mod of sortedMods) {
      if (!modCountsForPlayerSync(mod)) {
        continue
      }
      if (mod.syncKind === 'server_only') {
        serverOnly += 1
      } else if (mod.syncKind === 'client_required') {
        clientRequired += 1
        if (!mod.builtIn) {
          packagedClientRequired += 1
        }
      } else {
        unknown += 1
      }
    }

    return {
      mods: sortedMods,
      userVisibleMods: visibleMods,
      parseErrorCount: parseErrors,
      syncSummary: { serverOnly, clientRequired, unknown },
      syncPackagedClientRequired: packagedClientRequired,
    }
  }, [data?.mods])
  const restartRequired = data?.restartRequired ?? false
  const deleteBundle = useMemo(() => (
    confirmDelete ? modBundleMembers(mods, confirmDelete) : []
  ), [confirmDelete, mods])
  const deleteBundleCompanions = confirmDelete
    ? deleteBundle.filter((mod) => mod.folderName !== confirmDelete.folderName)
    : []
  const nexusTotalPages = Math.max(1, Math.ceil(nexusTotal / nexusPageSize))

  const tabItems: Array<{ id: ModWorkbenchTab; label: string; hint: string; icon: string }> = [
    { id: 'download', label: '下载模组', hint: '搜索 N 站并准备安装', icon: '/assets/stardew/ui/install/icon_install_step_download_image2.png' },
    { id: 'installed', label: '添加模组', hint: '本服已安装与玩家同步', icon: '/assets/stardew/ui/icons/icon_nav_install_package_image2.png' },
    { id: 'settings', label: '配置模组', hint: '启用、依赖与配置入口', icon: '/assets/stardew/ui/icons/icon_nav_settings_gear_image2.png' },
  ]

  const loadNexusSettings = useCallback(async () => {
    if (!isAdmin) return
    setNexusSettingsLoading(true)
    try {
      setNexusSettings(await getNexusSettings())
    } catch (e) {
      setNexusKeyError(errorMessage(e))
    } finally {
      setNexusSettingsLoading(false)
    }
  }, [isAdmin])

  useEffect(() => {
    void loadNexusSettings()
  }, [loadNexusSettings])

  useEffect(() => {
    return () => {
      if (nexusExtensionPollRef.current !== null) {
        window.clearInterval(nexusExtensionPollRef.current)
        nexusExtensionPollRef.current = null
      }
      if (nexusExtensionTimeoutRef.current !== null) {
        window.clearTimeout(nexusExtensionTimeoutRef.current)
        nexusExtensionTimeoutRef.current = null
      }
    }
  }, [])

  useEffect(() => {
    if (openNexusRequiredModId === null) return

    function handlePointerDown(event: PointerEvent) {
      const target = event.target
      if (target instanceof Element && target.closest('.sd-mods-dependency-details')) return
      setOpenNexusRequiredModId(null)
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setOpenNexusRequiredModId(null)
      }
    }

    document.addEventListener('pointerdown', handlePointerDown)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('pointerdown', handlePointerDown)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [openNexusRequiredModId])

  useEffect(() => {
    if (activeTab !== 'download') {
      setOpenNexusRequiredModId(null)
      return
    }
    if (openNexusRequiredModId === null || !nexusResults) return
    if (!nexusResults.some((result) => result.modId === openNexusRequiredModId)) {
      setOpenNexusRequiredModId(null)
    }
  }, [activeTab, nexusResults, openNexusRequiredModId])

  useEffect(() => {
    nexusExtensionInstallRef.current = nexusExtensionInstall
    writeNexusExtensionSessionState(nexusExtensionInstall)
  }, [nexusExtensionInstall])

  useEffect(() => {
    writeNexusSearchSessionState({
      query: nexusQuery,
      results: nexusResults,
      page: nexusPage,
      pageSize: nexusPageSize,
      total: nexusTotal,
      hasMore: nexusHasMore,
      pageInput: nexusPageInput,
      updatedAt: Date.now(),
    })
  }, [nexusQuery, nexusResults, nexusPage, nexusPageSize, nexusTotal, nexusHasMore, nexusPageInput])

  useEffect(() => {
    if (activeTab !== 'download') return

    let frame = 0
    const measureNexusSearchPageSize = () => {
      frame = 0
      const list = nexusResultsListRef.current
      if (!list) return

      const scrollViewport = list.closest('.sd-main-scroll') as HTMLElement | null
      const listRect = list.getBoundingClientRect()
      const viewportBottom = scrollViewport
        ? scrollViewport.getBoundingClientRect().bottom
        : window.innerHeight
      const listStyles = window.getComputedStyle(list)
      const cardHeight = Number.parseFloat(listStyles.getPropertyValue('--sd-mods-nexus-search-card-height')) || NEXUS_SEARCH_CARD_HEIGHT
      const rowGap = Number.parseFloat(listStyles.rowGap) || NEXUS_SEARCH_CARD_GAP
      const columns = countGridColumns(listStyles.gridTemplateColumns)
      const availableHeight = Math.max(cardHeight, viewportBottom - listRect.top - NEXUS_SEARCH_BOTTOM_RESERVE)
      const rows = Math.max(1, Math.floor((availableHeight + rowGap) / (cardHeight + rowGap)))
      const nextPageSize = clampNexusPageSize(rows * columns)

      setNexusPageSize((current) => (current === nextPageSize ? current : nextPageSize))
    }

    const scheduleMeasure = () => {
      if (frame) return
      frame = window.requestAnimationFrame(measureNexusSearchPageSize)
    }

    scheduleMeasure()
    const resizeObserver = new ResizeObserver(scheduleMeasure)
    const list = nexusResultsListRef.current
    const scrollViewport = list?.closest('.sd-main-scroll') as HTMLElement | null
    if (list) resizeObserver.observe(list)
    if (scrollViewport) resizeObserver.observe(scrollViewport)
    window.addEventListener('resize', scheduleMeasure)

    return () => {
      if (frame) window.cancelAnimationFrame(frame)
      resizeObserver.disconnect()
      window.removeEventListener('resize', scheduleMeasure)
    }
  }, [
    activeTab,
    nexusError,
    nexusInstallError,
    nexusExtensionInstall?.status,
    nexusExtensionInstall?.progress,
    nexusResults,
  ])

  function clearNexusExtensionTimers() {
    if (nexusExtensionPollRef.current !== null) {
      window.clearInterval(nexusExtensionPollRef.current)
      nexusExtensionPollRef.current = null
    }
    if (nexusExtensionTimeoutRef.current !== null) {
      window.clearTimeout(nexusExtensionTimeoutRef.current)
      nexusExtensionTimeoutRef.current = null
    }
  }

  function nexusExtensionItemProgress(status: NexusExtensionBatchItemStatus) {
    if (status === 'done') return 100
    if (status === 'queued') return 90
    if (status === 'posting') return 80
    if (status === 'ready') return 65
    if (status === 'capturing') return 35
    if (status === 'opening') return 10
    if (status === 'failed') return 100
    return 0
  }

  function nexusExtensionProgressFromItems(items: NexusExtensionBatchItem[] | undefined, fallback: number) {
    if (!items || items.length === 0) return Math.max(0, Math.min(90, Math.round(fallback || 0)))
    const total = items.reduce((sum, item) => sum + nexusExtensionItemProgress(item.status), 0)
    return Math.max(0, Math.min(90, Math.round(total / items.length)))
  }

  function nexusExtensionFailedItemMessage(items: NexusExtensionBatchItem[] | undefined) {
    const failed = items?.find((item) => item.status === 'failed')
    if (!failed) return '后台页面没有成功提交 ZIP 链接'
    return `${failed.name || `Mod ${failed.modId}`} 获取或提交失败：${failed.message || '请手动安装'}`
  }

  function openJobLogs(jobId: string | undefined) {
    const trimmed = jobId?.trim()
    if (!trimmed) return
    window.history.pushState(null, '', `${routeToPath('jobs')}?jobId=${encodeURIComponent(trimmed)}`)
    window.dispatchEvent(new PopStateEvent('popstate'))
  }

  function nexusModInstalledMatchInList(modList: ModInfo[] | undefined, modId: number) {
    if (!modList || modId <= 0) return null
    const match = modList.find((mod) => (
      (mod.nexusModId ?? 0) === modId ||
      (mod.originSource === 'nexus' && (mod.originNexusModId ?? 0) === modId)
    ))
    if (!match) return null
    return {
      installed: true,
      installedEnabled: match.enabled,
      installedFolderName: match.folderName,
      installedVersion: match.version,
    }
  }

  function nexusModInstalledInList(modList: ModInfo[] | undefined, modId: number) {
    return nexusModInstalledMatchInList(modList, modId) !== null
  }

  function syncNexusResultsFromInstalledMods(modList: ModInfo[] | undefined) {
    if (!modList) return
    setNexusResults((prev) => {
      if (!prev) return prev
      return prev.map((result) => {
        const match = nexusModInstalledMatchInList(modList, result.modId)
        const requiredMods = result.requiredMods?.map((required) => {
          const requiredMatch = nexusModInstalledMatchInList(modList, required.modId)
          return requiredMatch ? { ...required, ...requiredMatch } : required
        })
        return {
          ...result,
          ...(match ?? {}),
          ...(requiredMods ? { requiredMods } : {}),
        }
      })
    })
  }

  async function refreshModsAfterNexusExtensionDone() {
    const latest = await loadMods()
    const latestMods = latest?.mods ?? data?.mods ?? []
    syncNexusResultsFromInstalledMods(latestMods)
    await dashboardData.refreshMods()
  }

  function refreshDashboardForNexusBatchJobs(items: NexusExtensionBatchItem[] | null | undefined) {
    let hasNewJob = false
    for (const item of items ?? []) {
      const jobId = item.jobId?.trim()
      if (!jobId || nexusExtensionKnownJobIdsRef.current.has(jobId)) continue
      nexusExtensionKnownJobIdsRef.current.add(jobId)
      hasNewJob = true
    }
    if (hasNewJob) {
      void dashboardData.refreshJobs()
    }
  }

  async function markInstalledBatchItems(items: NexusExtensionBatchItem[]) {
    const needsLocalCheck = items.some((item) => item.status !== 'done' && item.modId > 0)
    if (!needsLocalCheck) return items
    const latest = await loadMods()
    const latestMods = latest?.mods ?? data?.mods ?? []
    syncNexusResultsFromInstalledMods(latestMods)
    return items.map((item) => {
      if (item.status === 'done' || !nexusModInstalledInList(latestMods, item.modId)) {
        return item
      }
      return {
        ...item,
        status: 'done' as NexusExtensionBatchItemStatus,
        message: item.message || 'installed locally',
      }
    })
  }

  async function reconcileNexusExtensionJobs(batchId: string, items: NexusExtensionBatchItem[]) {
    const reconciledItems = await markInstalledBatchItems(items)
    const jobItems = reconciledItems.filter((item) => item.jobId && item.status !== 'done')
    const doneItems = reconciledItems.filter((item) => item.status === 'done')
    if (jobItems.length === 0) {
      const activeInstall = nexusExtensionInstallRef.current
      if (!activeInstall || activeInstall.batchId !== batchId || activeInstall.status !== 'running') return
      if (doneItems.length === reconciledItems.length && reconciledItems.length > 0) {
        const doneState: NexusExtensionInstallState = {
          ...activeInstall,
          status: 'done',
          progress: 100,
          error: undefined,
          items: reconciledItems,
        }
        setNexusExtensionInstall(doneState)
        nexusExtensionInstallRef.current = doneState
        clearNexusExtensionTimers()
        void refreshModsAfterNexusExtensionDone()
        return
      }
      const waitingState: NexusExtensionInstallState = {
        ...activeInstall,
        status: 'running',
        progress: Math.max(activeInstall.progress, nexusExtensionProgressFromItems(reconciledItems, activeInstall.progress)),
        error: undefined,
        items: reconciledItems,
      }
      setNexusExtensionInstall(waitingState)
      nexusExtensionInstallRef.current = waitingState
      return
    }

    const settled = await Promise.allSettled(jobItems.map((item) => getJob(item.jobId as string)))
    const activeInstall = nexusExtensionInstallRef.current
    if (!activeInstall || activeInstall.batchId !== batchId || activeInstall.status !== 'running') return

    const failedIndex = settled.findIndex((result) => (
      result.status === 'fulfilled' &&
      (result.value.job.status === 'failed' || result.value.job.status === 'canceled')
    ))
    if (failedIndex >= 0) {
      const item = jobItems[failedIndex]
      const result = settled[failedIndex]
      const job = result.status === 'fulfilled' ? result.value.job : null
      const failedState: NexusExtensionInstallState = {
        ...activeInstall,
        status: 'failed',
        progress: 100,
        error: `${item.name || `Mod ${item.modId}`} 安装失败：${job?.errorMessage || job?.status || '请查看任务日志'}`,
        errorItemName: item.name || `Mod ${item.modId}`,
        errorJobId: item.jobId,
        items: reconciledItems,
      }
      setNexusExtensionInstall(failedState)
      nexusExtensionInstallRef.current = failedState
      clearNexusExtensionTimers()
      return
    }

    if (jobItems.length + doneItems.length < reconciledItems.length) {
      const waitingState: NexusExtensionInstallState = {
        ...activeInstall,
        status: 'running',
        progress: Math.max(activeInstall.progress, nexusExtensionProgressFromItems(reconciledItems, activeInstall.progress)),
        error: undefined,
        items: reconciledItems,
      }
      setNexusExtensionInstall(waitingState)
      nexusExtensionInstallRef.current = waitingState
      return
    }

    const succeeded = settled.filter((result) => result.status === 'fulfilled' && result.value.job.status === 'succeeded').length
    const hasPending = settled.some((result) => (
      result.status === 'rejected' ||
      (result.status === 'fulfilled' && (result.value.job.status === 'queued' || result.value.job.status === 'running'))
    ))
    if (jobItems.length + doneItems.length === reconciledItems.length && succeeded === jobItems.length && !hasPending) {
      const doneState: NexusExtensionInstallState = {
        ...activeInstall,
        status: 'done',
        progress: 100,
        error: undefined,
        items: reconciledItems,
      }
      setNexusExtensionInstall(doneState)
      nexusExtensionInstallRef.current = doneState
      clearNexusExtensionTimers()
      void refreshModsAfterNexusExtensionDone()
      return
    }

    const nextProgress = Math.max(activeInstall.progress, Math.round(90 + (succeeded / jobItems.length) * 10))
    const runningState: NexusExtensionInstallState = {
      ...activeInstall,
      status: 'running',
      progress: Math.max(90, Math.min(99, nextProgress)),
      error: undefined,
      items: reconciledItems,
    }
    setNexusExtensionInstall(runningState)
    nexusExtensionInstallRef.current = runningState
  }

  function updateNexusExtensionBatch(batch: NexusExtensionBatch | null | undefined) {
    if (!batch) return
    const activeInstall = nexusExtensionInstallRef.current
    if (!activeInstall || activeInstall.batchId !== batch.id) return
    if (activeInstall.status === 'done' || activeInstall.status === 'failed') return
    refreshDashboardForNexusBatchJobs(batch.items)

    const nextStatus = batch.status === 'failed'
      ? 'failed'
      : 'running'
    const nextProgress = Math.max(activeInstall.progress, nexusExtensionProgressFromItems(batch.items, batch.progress))
    const nextState: NexusExtensionInstallState = {
      ...activeInstall,
      status: nextStatus,
      progress: nextProgress,
      error: nextStatus === 'failed' ? nexusExtensionFailedItemMessage(batch.items) : undefined,
      errorItemName: nextStatus === 'failed' ? batch.items?.find((item) => item.status === 'failed')?.name : undefined,
      errorJobId: nextStatus === 'failed' ? batch.items?.find((item) => item.status === 'failed')?.jobId : undefined,
      items: batch.items,
    }
    setNexusExtensionInstall(nextState)
    nexusExtensionInstallRef.current = nextState

    if (nextStatus === 'failed') {
      clearNexusExtensionTimers()
    }
    if (nextStatus !== 'failed') {
      void reconcileNexusExtensionJobs(batch.id, batch.items || [])
    }
  }

  function requestNexusExtension<T>(type: 'PING' | 'START_BATCH_INSTALL' | 'GET_BATCH_STATUS' | 'CLEAR_STATE', payload: Record<string, unknown>, timeoutMs = 6000): Promise<T> {
    const requestId = `req_${Date.now()}_${Math.random().toString(16).slice(2)}`
    return new Promise((resolve, reject) => {
      const timeout = window.setTimeout(() => {
        window.removeEventListener('message', onMessage)
        reject(new Error('浏览器扩展未响应，请在扩展管理页重新加载 Anxi Nexus Installer 后刷新面板页'))
      }, timeoutMs)

      function onMessage(event: MessageEvent) {
        if (event.source !== window || event.origin !== window.location.origin) return
        const data = event.data as { source?: string; type?: string; requestId?: string; ok?: boolean; error?: string } & T
        if (data.source !== NEXUS_EXTENSION_SOURCE || data.requestId !== requestId || data.type !== `${type}_RESULT`) return
        window.clearTimeout(timeout)
        window.removeEventListener('message', onMessage)
        if (data.ok) {
          resolve(data)
        } else {
          reject(new Error(data.error || '浏览器扩展执行失败'))
        }
      }

      window.addEventListener('message', onMessage)
      window.postMessage({
        source: NEXUS_EXTENSION_PANEL_SOURCE,
        type,
        requestId,
        ...payload,
      }, window.location.origin)
    })
  }

  async function testNexusExtensionConnection(showSuccessMessage = true) {
    const currentPanelBaseUrl = window.location.origin
    setNexusExtensionConnection({
      status: 'checking',
      message: '正在检测浏览器扩展...',
      panelBaseUrl: currentPanelBaseUrl,
    })
    try {
      const response = await requestNexusExtension<{
        config?: { panelBaseUrl?: string; instanceId?: string }
      }>('PING', {
        panelBaseUrl: currentPanelBaseUrl,
        instanceId: NEXUS_EXTENSION_INSTANCE_ID,
      }, 5000)
      const panelBaseUrl = response.config?.panelBaseUrl || ''
      const syncedOrigin = panelBaseUrl ? new URL(panelBaseUrl).origin : ''
      if (syncedOrigin !== currentPanelBaseUrl) {
        throw new Error(`扩展仍指向旧面板地址：${panelBaseUrl || '未配置'}，当前地址是 ${currentPanelBaseUrl}`)
      }
      setNexusExtensionConnection({
        status: 'connected',
        message: showSuccessMessage ? `扩展已连通，面板地址已同步为 ${currentPanelBaseUrl}` : `扩展已连通并已同步：${currentPanelBaseUrl}`,
        panelBaseUrl,
      })
      return true
    } catch (e) {
      const message = errorMessage(e)
      setNexusExtensionConnection({
        status: 'disconnected',
        message: message === 'unsupported_message'
          ? '扩展脚本版本过旧，请在浏览器扩展管理页重新加载 Anxi Nexus Installer，然后刷新面板页'
          : message,
        panelBaseUrl: currentPanelBaseUrl,
      })
      return false
    }
  }

  function resetNexusExtensionInstallState() {
    clearNexusExtensionTimers()
    const previousBatchId = nexusExtensionInstallRef.current?.batchId
    setNexusExtensionInstall(null)
    nexusExtensionInstallRef.current = null
    nexusExtensionKnownJobIdsRef.current.clear()
    setNexusInstallError(null)
    writeNexusExtensionSessionState(null)
    void requestNexusExtension<{ ok?: boolean }>('CLEAR_STATE', previousBatchId ? { batchId: previousBatchId } : {}, 4000)
      .catch(() => {
        // The local panel state is already cleared; extension cleanup is best-effort.
      })
    void loadMods().then(() => dashboardData.refreshMods())
  }

  useEffect(() => {
    function onMessage(event: MessageEvent) {
      if (event.source !== window || event.origin !== window.location.origin) return
      const data = event.data as { source?: string; type?: string; batch?: NexusExtensionBatch | null }
      if (data.source === NEXUS_EXTENSION_SOURCE && data.type === 'BATCH_STATUS_UPDATE') {
        updateNexusExtensionBatch(data.batch)
      }
    }

    window.addEventListener('message', onMessage)
    return () => window.removeEventListener('message', onMessage)
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    const restoredInstall = nexusExtensionInstallRef.current
    if (!restoredInstall || (restoredInstall.status !== 'starting' && restoredInstall.status !== 'running')) return
    const batchId = restoredInstall.batchId

    void requestNexusExtension<{ batch: NexusExtensionBatch }>('GET_BATCH_STATUS', { batchId }, 5000)
      .then((response) => updateNexusExtensionBatch(response.batch))
      .catch(() => {
        const activeInstall = nexusExtensionInstallRef.current
        if (activeInstall?.batchId === batchId && activeInstall.status !== 'done' && activeInstall.status !== 'failed') {
          setNexusExtensionInstall({
            ...activeInstall,
            status: 'failed',
            progress: 100,
            error: '浏览器扩展未返回进度',
          })
          clearNexusExtensionTimers()
        }
      })

    nexusExtensionPollRef.current = window.setInterval(() => {
      void requestNexusExtension<{ batch: NexusExtensionBatch }>('GET_BATCH_STATUS', { batchId }, 4000)
        .then((response) => updateNexusExtensionBatch(response.batch))
        .catch(() => {
          const activeInstall = nexusExtensionInstallRef.current
          if (activeInstall?.batchId === batchId && activeInstall.status !== 'done' && activeInstall.status !== 'failed') {
            setNexusExtensionInstall({
              ...activeInstall,
              status: 'failed',
              progress: 100,
              error: '浏览器扩展未返回进度',
            })
            clearNexusExtensionTimers()
          }
      })
    }, 2500)
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!isAdmin) return
    const timer = window.setTimeout(() => {
      void testNexusExtensionConnection(false)
    }, 500)
    return () => window.clearTimeout(timer)
  }, [isAdmin]) // eslint-disable-line react-hooks/exhaustive-deps


  async function handleNexusExtensionPackDownload() {
    setNexusExtensionPackBusy(true)
    setNexusInstallError(null)
    try {
      const { blob, filename } = await downloadNexusInstallerExtension()
      downloadBlob(blob, filename)
    } catch (e) {
      setNexusInstallError(errorMessage(e))
    } finally {
      setNexusExtensionPackBusy(false)
    }
  }

  const handleNexusSearch = useCallback(async (page = 1, queryOverride?: string, pageSizeOverride?: number) => {
    const query = (queryOverride ?? nexusQuery).trim()
    const searchPageSize = clampNexusPageSize(pageSizeOverride ?? nexusPageSize)
    lastNexusSearchPageSizeRef.current = searchPageSize
    setNexusLoading(true)
    setNexusError(null)
    try {
      const result = await searchNexusMods(query, page, searchPageSize)
      setNexusResults(result.results)
      setNexusPage(result.page)
      setNexusTotal(result.total)
      setNexusHasMore(result.hasMore)
      setNexusPageInput(String(result.page))
    } catch (e) {
      setNexusError(errorMessage(e))
      setNexusResults(null)
      setNexusPage(1)
      setNexusTotal(0)
      setNexusHasMore(false)
      setNexusPageInput('1')
    } finally {
      setNexusLoading(false)
    }
  }, [nexusPageSize, nexusQuery])

  useEffect(() => {
    if (activeTab !== 'download' || defaultNexusLoadedRef.current) return
    defaultNexusLoadedRef.current = true
    void handleNexusSearch(1, '')
  }, [activeTab, handleNexusSearch])

  useEffect(() => {
    if (activeTab !== 'download' || nexusResults === null || nexusLoading) return
    if (lastNexusSearchPageSizeRef.current === nexusPageSize) return
    void handleNexusSearch(1, undefined, nexusPageSize)
  }, [activeTab, handleNexusSearch, nexusLoading, nexusPageSize, nexusResults])

  function clampNexusPage(page: number) {
    if (!Number.isFinite(page)) return nexusPage
    return Math.min(Math.max(1, Math.trunc(page)), nexusTotalPages)
  }

  function jumpToNexusPage(page: number) {
    const target = clampNexusPage(page)
    setNexusPageInput(String(target))
    if (target !== nexusPage) {
      void handleNexusSearch(target)
    }
  }

  function handleNexusPageInputJump() {
    const parsed = Number.parseInt(nexusPageInput, 10)
    if (!Number.isFinite(parsed)) {
      setNexusPageInput(String(nexusPage))
      return
    }
    jumpToNexusPage(parsed)
  }

  function handleNexusQuickTag(tag: string) {
    setNexusQuery(tag)
    void handleNexusSearch(1, tag)
  }

  function renderNexusPager(position: 'top' | 'bottom') {
    const atFirstPage = nexusPage <= 1
    const atLastPage = nexusPage >= nexusTotalPages || !nexusHasMore
    if (position === 'top') {
      return (
        <div className="sd-mods-nexus-total sd-mods-nexus-total-top">
          <span>搜索结果（共 {nexusTotal.toLocaleString()} 个）</span>
          <span>每页 {nexusPageSize} 个</span>
        </div>
      )
    }

    return (
      <div className={`sd-mods-nexus-total sd-mods-nexus-total-${position}`}>
        <span>第 {nexusPage} / {nexusTotalPages} 页</span>
        <div className="sd-mods-nexus-page-actions">
          <button
            className="sd-btn-tan"
            type="button"
            disabled={nexusLoading || atFirstPage}
            onClick={() => jumpToNexusPage(1)}
          >
            首页
          </button>
          <button
            className="sd-btn-tan"
            type="button"
            disabled={nexusLoading || atFirstPage}
            onClick={() => jumpToNexusPage(nexusPage - 1)}
          >
            上一页
          </button>
          <div className="sd-mods-nexus-page-jump">
            <input
              className="sd-input sd-mods-nexus-page-input"
              type="number"
              min={1}
              max={nexusTotalPages}
              value={nexusPageInput}
              disabled={nexusLoading}
              onChange={(e) => setNexusPageInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleNexusPageInputJump() }}
              aria-label="跳转页码"
            />
            <button
              className="sd-btn-tan"
              type="button"
              disabled={nexusLoading}
              onClick={handleNexusPageInputJump}
            >
              跳转
            </button>
          </div>
          <button
            className="sd-btn-tan"
            type="button"
            disabled={nexusLoading || atLastPage}
            onClick={() => jumpToNexusPage(nexusPage + 1)}
          >
            下一页
          </button>
          <button
            className="sd-btn-tan"
            type="button"
            disabled={nexusLoading || atLastPage}
            onClick={() => jumpToNexusPage(nexusTotalPages)}
          >
            末页
          </button>
        </div>
      </div>
    )
  }

  function nexusExtensionInstallTargets(result: NexusModSearchResult) {
    const byModId = new Map<number, NexusExtensionBatchItem>()
    for (const required of result.requiredMods ?? []) {
      if (required.installed) continue
      byModId.set(required.modId, {
        id: `required-${required.modId}`,
        role: 'required',
        modId: required.modId,
        name: required.name,
        url: nexusRequiredInstallURL(required),
        status: 'pending',
      })
    }
    byModId.set(result.modId, {
      id: `target-${result.modId}`,
      role: 'target',
      modId: result.modId,
      name: result.name,
      url: nexusExtensionInstallURL(result),
      status: 'pending',
    })
    return Array.from(byModId.values())
  }

  async function handleNexusInstall(result: NexusModSearchResult) {
    if (nexusExtensionInstall?.status === 'starting' || nexusExtensionInstall?.status === 'running') return
    if (nexusExtensionConnection.status !== 'connected') {
      const connected = await testNexusExtensionConnection(false)
      if (!connected) return
    }
    const batchId = `nexus_${result.modId}_${Date.now()}`
    const installState: NexusExtensionInstallState = {
      modId: result.modId,
      batchId,
      status: 'starting',
      progress: 5,
    }
    clearNexusExtensionTimers()
    setNexusExtensionInstall(installState)
    nexusExtensionInstallRef.current = installState
    nexusExtensionKnownJobIdsRef.current.clear()
    setNexusInstallError(null)

    try {
      const response = await requestNexusExtension<{ batch: NexusExtensionBatch }>('START_BATCH_INSTALL', {
        payload: {
          batchId,
          targets: nexusExtensionInstallTargets(result),
        },
      }, 8000)
      updateNexusExtensionBatch(response.batch)
      nexusExtensionPollRef.current = window.setInterval(() => {
        void requestNexusExtension<{ batch: NexusExtensionBatch }>('GET_BATCH_STATUS', { batchId }, 4000)
          .then((pollResponse) => updateNexusExtensionBatch(pollResponse.batch))
          .catch(() => {
            const activeInstall = nexusExtensionInstallRef.current
            if (activeInstall?.batchId === batchId && activeInstall.status !== 'done' && activeInstall.status !== 'failed') {
              setNexusExtensionInstall({ ...activeInstall, status: 'failed', progress: 100, error: '浏览器扩展未返回进度' })
              clearNexusExtensionTimers()
            }
          })
      }, 2500)
      nexusExtensionTimeoutRef.current = window.setTimeout(() => {
        const activeInstall = nexusExtensionInstallRef.current
        const hasPanelJobs = activeInstall?.items?.some((item) => item.jobId)
        if (activeInstall?.batchId === batchId && activeInstall.status !== 'done' && !hasPanelJobs) {
          setNexusExtensionInstall({ ...activeInstall, status: 'failed', progress: 100, error: '后台页面超时未提交 ZIP 链接' })
          clearNexusExtensionTimers()
        }
      }, 210000)
    } catch (e) {
      const failedState: NexusExtensionInstallState = {
        modId: result.modId,
        batchId,
        status: 'failed',
        progress: 100,
        error: errorMessage(e),
      }
      setNexusExtensionInstall(failedState)
      nexusExtensionInstallRef.current = failedState
      clearNexusExtensionTimers()
    }
  }

  function openNexusKeyModal() {
    setNexusKeyInput('')
    setNexusKeyError(null)
    setNexusKeyMessage(null)
    setShowNexusKeyModal(true)
  }

  function closeNexusKeyModal() {
    if (nexusKeyBusy) return
    setShowNexusKeyModal(false)
    setNexusKeyInput('')
    setNexusKeyError(null)
    setNexusKeyMessage(null)
  }

  async function handleNexusKeySave() {
    const apiKey = nexusKeyInput.trim()
    if (!apiKey) {
      setNexusKeyError('请粘贴 Nexus API Key')
      return
    }
    setNexusKeyBusy(true)
    setNexusKeyError(null)
    setNexusKeyMessage(null)
    try {
      const status = await saveNexusAPIKey(apiKey)
      setNexusSettings(status)
      setNexusKeyInput('')
      setNexusKeyMessage('已保存，当前搜索会立即使用这个 Key')
    } catch (e) {
      setNexusKeyError(errorMessage(e))
    } finally {
      setNexusKeyBusy(false)
    }
  }

  async function handleNexusKeyDelete() {
    setNexusKeyBusy(true)
    setNexusKeyError(null)
    setNexusKeyMessage(null)
    try {
      const status = await deleteNexusAPIKey()
      setNexusSettings(status)
      setNexusKeyInput('')
      setNexusKeyMessage('已移除 Nexus API Key')
    } catch (e) {
      setNexusKeyError(errorMessage(e))
    } finally {
      setNexusKeyBusy(false)
    }
  }

  function downloadBlob(blob: Blob, filename: string) {
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  function searchedModBaseCanInstall(result: NexusModSearchResult) {
    const extensionBusy = nexusExtensionInstall?.status === 'starting' || nexusExtensionInstall?.status === 'running'
    const extensionFailedSameMod = nexusExtensionInstall?.status === 'failed' && nexusExtensionInstall.modId === result.modId
    const extensionDoneSameMod = nexusExtensionInstall?.status === 'done' && nexusExtensionInstall.modId === result.modId
    if (!isAdmin || isRunning || result.installed || extensionBusy || extensionFailedSameMod || extensionDoneSameMod) {
      return false
    }
    return true
  }

  function searchedModCanInstall(result: NexusModSearchResult) {
    return searchedModBaseCanInstall(result) && nexusExtensionConnection.status === 'connected'
  }

  function searchedModCanClickAction(result: NexusModSearchResult) {
    return searchedModCanInstall(result) || (
      nexusExtensionInstall?.status === 'failed' &&
      nexusExtensionInstall.modId === result.modId &&
      !!nexusExtensionInstall.errorJobId
    )
  }

  function handleNexusInstallAction(result: NexusModSearchResult) {
    if (nexusExtensionInstall?.status === 'failed' && nexusExtensionInstall.modId === result.modId && nexusExtensionInstall.errorJobId) {
      openJobLogs(nexusExtensionInstall.errorJobId)
      return
    }
    void handleNexusInstall(result)
  }

  function searchedModInstallTitle(result: NexusModSearchResult) {
    if (result.installed && result.installedEnabled === false) return '该 Mod 已安装，但当前存档未启用，可到配置模组中启用'
    if (result.installed) return '该 Mod 已安装'
    if (!isAdmin) return '仅管理员可以安装 Mod'
    if (isRunning) return '服务器运行中，请先停止后安装 Mod'
    if ((nexusExtensionInstall?.status === 'starting' || nexusExtensionInstall?.status === 'running') && nexusExtensionInstall.modId !== result.modId) {
      return '已有扩展安装流程正在进行'
    }
    if (nexusExtensionInstall?.status === 'done' && nexusExtensionInstall.modId === result.modId) {
      return '扩展安装流程已完成'
    }
    if (nexusExtensionInstall?.status === 'failed' && nexusExtensionInstall.modId === result.modId) {
      if (nexusExtensionInstall.errorJobId) {
        return `${nexusExtensionInstall.error || '安装失败'}；点击查看任务与日志`
      }
      return nexusExtensionInstall.error || '后台页面没有成功提交 ZIP 链接，请手动安装'
    }
    if (nexusExtensionConnection.status !== 'connected') {
      return nexusExtensionConnection.status === 'checking'
        ? '正在检测浏览器扩展连通性'
        : `请先点击“检测扩展”，连通后才能使用普通一键安装。${nexusExtensionConnection.message || ''}`
    }
    return '后台打开 Nexus 下载页，浏览器扩展会自动获取 ZIP 链接并提交到面板'
  }

  function searchedModInstallLabel(result: NexusModSearchResult, installing: boolean) {
    if (installing) return '打开中…'
    if (result.installed && result.installedEnabled === false) return '已安装未启用'
    if (result.installed) return '已安装'
    if (nexusExtensionInstall?.modId === result.modId) {
      if (nexusExtensionInstall.status === 'failed') return `${nexusExtensionInstall.errorItemName || result.name || 'Mod'} 失败`
      if (nexusExtensionInstall.status === 'done') return '安装完成 100%'
      return `安装中 ${Math.max(0, Math.min(100, Math.round(nexusExtensionInstall.progress)))}%`
    }
    return '一键安装'
  }

  function nexusExtensionConnectionLabel() {
    if (nexusExtensionConnection.status === 'checking') return '检测扩展中…'
    if (nexusExtensionConnection.status === 'connected') return '扩展已连通'
    return '检测扩展'
  }

  function nexusExtensionConnectionTitle() {
    if (nexusExtensionConnection.status === 'connected') {
      return `浏览器扩展已连通，当前面板地址：${nexusExtensionConnection.panelBaseUrl}`
    }
    return nexusExtensionConnection.message || '检测浏览器扩展是否已安装、已重新加载，并自动同步当前面板地址'
  }

  return (
    <div className="sd-page sd-mods-page">
      <div className="sd-mods-header sd-page-header">
        <div className="sd-mods-header-left">
          <img
            className="sd-page-icon"
            src="/assets/stardew/ui/icons/icon_nav_mods_crystal_image2.png"
            alt=""
          />
          <div>
            <h2 className="sd-page-title">模组管理</h2>
            <p className="sd-page-desc">搜索、安装、同步和配置 SMAPI 模组</p>
          </div>
        </div>
        <div className="sd-mods-header-actions sd-actionbar sd-actionbar--end">
          <button
            className="sd-btn-tan"
            disabled={loading}
            onClick={loadMods}
            type="button"
            title="刷新 Mod 列表"
          >
            {loading ? '刷新中…' : '刷新'}
          </button>
          <button
            className="sd-btn-tan"
            disabled={exportBusy || mods.length === 0}
            onClick={handleExport}
            type="button"
            title={mods.length === 0 ? '暂无 Mod 可导出' : '导出全部 Mod 为 ZIP'}
          >
            {exportBusy ? '导出中…' : '导出 Mod 包'}
          </button>
          <button
            className="sd-btn-green"
            disabled={writeDisabled}
            onClick={() => { openUpload(); setActiveTab('installed') }}
            type="button"
            title={writeTitle || '上传 ZIP 包安装 Mod'}
          >
            上传 Mod
          </button>
        </div>
      </div>

      {listError && <div className="sd-mods-list-error">{listError}</div>}
      {exportError && <div className="sd-mods-list-error">{exportError}</div>}

      <div className="sd-mods-workbench">
        <div className="sd-mods-tabs" role="tablist" aria-label="模组工作台">
          {tabItems.map((tab) => (
            <button
              key={tab.id}
                className={`sd-mods-tab${activeTab === tab.id ? ' sd-mods-tab-active' : ''}`}
                type="button"
                role="tab"
                aria-selected={activeTab === tab.id}
                onClick={() => setActiveTab(tab.id)}
              >
              <img className="sd-mods-tab-icon" src={tab.icon} alt="" />
              <span className="sd-mods-tab-text">
                <span className="sd-mods-tab-label">{tab.label}</span>
                <span className="sd-mods-tab-hint">{tab.hint}</span>
              </span>
            </button>
          ))}
        </div>

        <div className="sd-mods-tab-panel" role="tabpanel">
          {activeTab === 'download' ? (
            <>
              <div className="sd-mods-nexus-connect-strip">
                <span className="sd-mods-connect-title">Nexus 连接</span>
                <span className="sd-mods-connect-key-label">API Key</span>
                <span className="sd-mods-connect-key">
                  {nexusSettingsLoading
                    ? '读取中...'
                    : nexusSettings?.configured
                      ? `••••••••••••${nexusSettings.last4 ? ` ${nexusSettings.last4}` : ''}`
                      : '未配置'}
                </span>
                <span className={`sd-mods-connect-state ${nexusSettings?.configured ? 'sd-mods-connect-state-ok' : 'sd-mods-connect-state-warn'}`}>
                  {nexusSettings?.configured ? '已配置' : '待配置'}
                </span>
                <span className="sd-mods-connect-divider" aria-hidden="true" />
                <span className="sd-mods-connect-title">扩展连接</span>
                <span className={`sd-mods-extension-status sd-mods-extension-status-${nexusExtensionConnection.status}`}>
                  {nexusExtensionConnection.status === 'connected' ? '已连接' : nexusExtensionConnection.status === 'checking' ? '检测中' : '未连接'}
                </span>
                <div className="sd-mods-panel-actions">
                  {isAdmin && (
                    <>
                      <button
                        className="sd-btn-tan"
                        type="button"
                        onClick={() => openNexusKeyModal()}
                        disabled={nexusSettingsLoading}
                        title="配置 Nexus Mods API Key"
                      >
                        配置 Nexus Key
                      </button>
                      <button
                        className={`sd-btn-tan sd-mods-extension-check sd-mods-extension-check-${nexusExtensionConnection.status}`}
                        type="button"
                        onClick={() => void testNexusExtensionConnection(true)}
                        disabled={nexusExtensionConnection.status === 'checking'}
                        title={nexusExtensionConnectionTitle()}
                      >
                        {nexusExtensionConnectionLabel()}
                      </button>
                      <button
                        className="sd-btn-tan sd-mods-extension-download"
                        type="button"
                        onClick={() => void handleNexusExtensionPackDownload()}
                        disabled={nexusExtensionPackBusy}
                        title="下载面板打包好的浏览器扩展 ZIP，解压后在浏览器扩展管理页加载"
                      >
                        {nexusExtensionPackBusy ? '打包中…' : '下载扩展'}
                      </button>
                    </>
                  )}
                </div>
              </div>

              <div className="sd-mods-search-card">
                <div className="sd-mods-section-title">搜索 Nexus Mods</div>
                <div className="sd-mods-nexus-search-row">
                  <input
                    className="sd-input"
                    type="text"
                    placeholder="输入英文模组名称、ID 或关键词..."
                    value={nexusQuery}
                    onChange={(e) => setNexusQuery(e.target.value)}
                    onKeyDown={(e) => { if (e.key === 'Enter') void handleNexusSearch(1) }}
                  />
                  <button
                    className="sd-btn-green"
                    disabled={nexusLoading}
                    onClick={() => void handleNexusSearch(1)}
                    type="button"
                    title={nexusQuery.trim() ? '搜索 Nexus Mods' : '刷新近期热门 Nexus Mods'}
                  >
                    {nexusLoading ? '搜索中…' : '搜索'}
                  </button>
                </div>
                <div className="sd-mods-quick-tags">
                  <span>热门标签:</span>
                  {NEXUS_QUICK_TAGS.map((tag) => (
                    <button
                      key={tag}
                      type="button"
                      className="sd-mods-quick-tag"
                      disabled={nexusLoading}
                      onClick={() => handleNexusQuickTag(tag)}
                    >
                      {tag}
                    </button>
                  ))}
                </div>
              </div>

              {nexusError && <div className="sd-mods-list-error">{nexusError}</div>}
              {nexusInstallError && <div className="sd-mods-list-error">{nexusInstallError}</div>}
              {nexusLoading ? (
                <div className="sd-mods-nexus-skeleton-grid sd-mods-nexus-search-list">
                  {Array.from({ length: nexusPageSize }, (_, i) => (
                    <div className="sd-mods-nexus-skeleton" key={i} />
                  ))}
                </div>
              ) : nexusResults ? (
                nexusResults.length === 0 ? (
                  <div className="sd-mods-nexus-empty">
                    {nexusQuery.trim() ? '未找到匹配的 Mod，换个关键词试试。' : '暂时没有读取到 N 站热门模组。'}
                  </div>
                ) : (
                  <>
                    {renderNexusPager('top')}
                    <div className="sd-mods-nexus-list sd-mods-nexus-search-list" ref={nexusResultsListRef}>
                      {nexusResults.map((r) => {
                        const canInstall = searchedModCanClickAction(r)
                        const requiredMods = r.requiredMods ?? []
                        const missingRequiredMods = missingNexusRequiredMods(r)
                        const extensionState = nexusExtensionInstall?.modId === r.modId ? nexusExtensionInstall : null
                        const extensionProgress = extensionState ? Math.max(0, Math.min(100, Math.round(extensionState.progress))) : 0
                        const installButtonClass = [
                          'sd-btn-green',
                          extensionState ? 'sd-mods-install-progress-button' : '',
                          extensionState?.status === 'failed' ? 'sd-mods-install-progress-failed' : '',
                        ].filter(Boolean).join(' ')
                        const installButtonStyle = extensionState
                          ? ({ '--sd-install-progress': `${extensionProgress}%` } as CSSProperties)
                          : undefined
                        const requiredDetailsOpen = openNexusRequiredModId === r.modId
                        return (
                          <ModSearchResultCard
                            key={r.modId}
                            className={requiredDetailsOpen ? 'sd-mods-nexus-card-dependency-open' : undefined}
                            result={nexusResultToSearchResult(r)}
                            statsSlot={(
                              requiredMods.length > 0 ? (
                                <NexusRequiredModsBadge
                                  requiredMods={requiredMods}
                                  missingRequiredMods={missingRequiredMods}
                                  isOpen={requiredDetailsOpen}
                                  onToggle={() => setOpenNexusRequiredModId((current) => (
                                    current === r.modId ? null : r.modId
                                  ))}
                                />
                              ) : (
                                <span className="sd-tag sd-tag-gold sd-mods-dependency-tag">前置：无</span>
                              )
                            )}
                            actionSlot={(
                              <button
                                className={installButtonClass}
                                style={installButtonStyle}
                                type="button"
                                disabled={!canInstall}
                                title={searchedModInstallTitle(r)}
                                onClick={() => handleNexusInstallAction(r)}
                              >
                                {searchedModInstallLabel(r, false)}
                              </button>
                            )}
                            footerSlot={extensionState ? (
                              <div className="sd-mods-search-footer">
                                <button
                                  className="sd-btn-tan sd-mods-extension-reset"
                                  type="button"
                                  title="清除当前浏览器保存的一键安装进度，安装已完成的 Mod 不会被删除"
                                  onClick={resetNexusExtensionInstallState}
                                >
                                  重置状态
                                </button>
                              </div>
                            ) : undefined}
                          />
                        )
                      })}
                    </div>
                  </>
                )
              ) : (
                <div className="sd-mods-nexus-empty">输入名称或 ID 后开始搜索。本页会显示来源、跳转入口和可用的一键安装方式。</div>
              )}
              {nexusResults && nexusResults.length > 0 ? renderNexusPager('bottom') : null}
            </>
          ) : null}

          {activeTab === 'installed' ? (
            <>
              {isRunning && (
                <div className="sd-mods-running-hint">
                  ⚠ 服务器运行中，Mod 上传和删除已暂时禁用 - 请先停止服务器
                </div>
              )}

              {uploadSuccess && (
                <div className="sd-mods-success-banner">
                  ✔ 已解析 {uploadSuccess.archiveCount} 个 ZIP，发现 {uploadSuccess.discoveredCount} 个 Mod，
                  安装 {uploadSuccess.importedCount} 个，
                  已启用 {uploadSuccess.enabledCount} 个
                  {uploadSuccess.activeSaveName ? `（当前存档：${uploadSuccess.activeSaveName}）` : ''}。
                  下次启动服务器时会自动加载。
                </div>
              )}

              <div className="sd-mods-overview">
                <div className="sd-mods-stat">
                  <span className="sd-mods-stat-label">已安装</span>
                  <span className="sd-mods-stat-value">{loading ? '-' : `${userVisibleMods.length} 个`}</span>
                </div>
                <div className="sd-mods-stat">
                  <span className="sd-mods-stat-label">服务器状态</span>
                  <span className="sd-mods-stat-value">
                    <span
                      className={
                        isRunning
                          ? 'sd-dot sd-dot-green sd-dot-pulse'
                          : state === 'error'
                            ? 'sd-dot sd-dot-red'
                            : 'sd-dot sd-dot-gray'
                      }
                      aria-hidden="true"
                    />
                    {isRunning ? '运行中' : state === 'stopped' ? '已停止' : state ?? '未知'}
                  </span>
                </div>
                <div className="sd-mods-stat">
                  <span className="sd-mods-stat-label">运行中重启</span>
                  <span className={`sd-mods-stat-value${restartRequired ? ' sd-mods-restart-flag' : ''}`}>
                    {restartRequired ? '需要重启' : '无'}
                  </span>
                </div>
                <div className="sd-mods-stat">
                  <span className="sd-mods-stat-label">解析失败</span>
                  <span className="sd-mods-stat-value">{loading ? '-' : `${parseErrorCount} 个`}</span>
                </div>
              </div>

              <div className="sd-mods-section-title">玩家同步</div>
              <div className="sd-mods-sync-summary">
                <span className="sd-tag sd-tag-blue">服务器专用 {syncSummary.serverOnly}</span>
                <span className="sd-tag sd-tag-green">玩家需同步 {syncSummary.clientRequired}</span>
                <span className="sd-tag sd-tag-gold">待确认 {syncSummary.unknown}</span>
                <button
                  className="sd-btn-tan"
                  disabled={syncPackBusy !== null || syncSummary.clientRequired === 0}
                  onClick={() => handleSyncPackExport('full')}
                  type="button"
                  title={syncSummary.clientRequired === 0 ? '暂无需要玩家同步的 Mod' : '首次加入玩家使用，包含 SMAPI'}
                >
                  {syncPackBusy === 'full' ? '导出中…' : '导出完整同步包'}
                </button>
                <button
                  className="sd-btn-tan"
                  disabled={syncPackBusy !== null || syncPackagedClientRequired === 0}
                  onClick={() => handleSyncPackExport('update')}
                  type="button"
                  title={syncPackagedClientRequired === 0 ? '暂无可打包的玩家 Mod' : '已运行过同步包的玩家使用，不包含 SMAPI'}
                >
                  {syncPackBusy === 'update' ? '导出中…' : '导出模组更新包'}
                </button>
              </div>
              <p className="sd-mods-sync-hint">
                完整同步包用于首次加入玩家；模组更新包不带 SMAPI，适合已经安装过同步包的玩家。客户端会跳过完全相同的 Mod，只备份并覆盖内容不同的同名 Mod。
              </p>
              {syncError && <div className="sd-mods-list-error">{syncError}</div>}
              {syncPackError && <div className="sd-mods-list-error">{syncPackError}</div>}

              {restartRequired && (
                <div className="sd-mods-restart-banner">
                  ⚠ Mod 已变更 - 需要重启服务器才能生效
                </div>
              )}

              <div className="sd-mods-section-title">
                已安装模组
                {loading && <span className="sd-mods-loading-tag">加载中…</span>}
              </div>

              {!loading && userVisibleMods.length === 0 ? (
                <div className="sd-mods-empty">
                  <img
                    className="sd-mods-empty-icon"
                    src="/assets/stardew/ui/icons/icon_nav_mods.png"
                    alt=""
                  />
                  <div className="sd-mods-empty-title">当前没有可展示 Mod</div>
                  <div className="sd-mods-empty-desc">
                    上传包含 SMAPI Mod 的 ZIP 文件来安装模组。
                    每个 Mod 文件夹中应包含 manifest.json。
                  </div>
                  <button
                    className="sd-btn-green"
                    disabled={writeDisabled}
                    title={writeTitle || '上传 ZIP 包安装 Mod'}
                    onClick={openUpload}
                    type="button"
                  >
                    上传 Mod
                  </button>
                </div>
              ) : (
                <>
                  <div className="sd-mods-nexus-list">
                    {userVisibleMods.map((mod) => {
                    const syncBusy = syncUpdating === mod.id
                    const requiredDependency = dependencyDisplay(mod)
                    const isBuiltIn = mod.builtIn === true
                    const isSmapi = modIsSmapi(mod)
                    const isPanelControl = modIsPanelControl(mod)
                    const bundleDeleteCount = isBuiltIn ? 1 : modBundleMembers(mods, mod).length
                    return (
                      <ModSearchResultCard
                        key={mod.id}
                        result={modToSearchResult(mod)}
                        actionSlot={isBuiltIn ? (
                          <span className="sd-mods-pending-badge">内置</span>
                        ) : (
                          <button
                            className="sd-btn-delete"
                            disabled={writeDisabled}
                            title={writeTitle || (bundleDeleteCount > 1 ? `删除同包 ${bundleDeleteCount} 个 Mod` : '删除此 Mod')}
                            onClick={() => openDeleteConfirm(mod)}
                            type="button"
                          >
                            删除
                          </button>
                        )}
                        footerSlot={(
                          <div className="sd-mods-installed-footer">
                            {isBuiltIn ? (
                              <>
                                <span className="sd-tag sd-tag-green">内置</span>
                                {isSmapi ? (
                                  <>
                                    <span className="sd-tag sd-tag-gold">玩家需先安装</span>
                                    <span className="sd-tag sd-tag-blue">写入同步清单</span>
                                  </>
                                ) : null}
                                {isPanelControl ? (
                                  <>
                                    <span className="sd-tag sd-tag-gold">服务端控制</span>
                                    <span className="sd-tag sd-tag-blue">不打包进同步包</span>
                                  </>
                                ) : null}
                              </>
                            ) : (
                              <>
                                <span className={`sd-tag ${mod.enabled ? 'sd-tag-green' : 'sd-tag-gold'}`}>
                                  {mod.enabled ? '已启用' : '已禁用'}
                                </span>
                                <span className={`sd-tag ${SYNC_KIND_TAG_CLASS[mod.syncKind]}`} title={mod.syncNote ?? ''}>
                                  {SYNC_KIND_LABELS[mod.syncKind]}
                                </span>
                              </>
                            )}
                            {!isBuiltIn && (
                              <select
                                className="sd-mods-sync-select"
                                value={mod.syncKind}
                                disabled={syncBusy}
                                onChange={(e) => handleSyncChange(mod, e.target.value as ModSyncKind)}
                              >
                                <option value="server_only">服务器专用</option>
                                <option value="client_required">玩家需同步</option>
                                <option value="unknown">待确认</option>
                              </select>
                            )}
                            {requiredDependency ? (
                              <span className={`sd-tag ${requiredDependency.className} sd-mods-dependency-tag`} title={requiredDependency.title}>
                                {requiredDependency.label}
                              </span>
                            ) : null}
                            {syncBusy && <span className="sd-mods-loading-tag">更新中…</span>}
                            {mod.parseError ? <span className="sd-mods-parse-error">解析失败：{mod.parseError}</span> : null}
                          </div>
                        )}
                      />
                    )
                    })}
                  </div>
                </>
              )}
            </>
          ) : null}

          {activeTab === 'settings' ? (
            <div className="sd-mods-settings-layout">
              <section className="sd-mods-settings-left sd-mods-enable-panel">
                <div className="sd-mods-section-title">
                  当前存档 Mod 启用状态
                  {activeSaveName ? <span className="sd-mods-loading-tag">{activeSaveName}</span> : null}
                </div>
                {enableError ? <div className="sd-mods-list-error">{enableError}</div> : null}
                {!activeSaveName ? (
                  <div className="sd-mods-empty sd-mods-settings-empty">
                    <img className="sd-mods-empty-icon" src="/assets/stardew/ui/icons/icon_nav_saves.png" alt="" />
                    <div className="sd-mods-empty-title">请先选择一个存档</div>
                    <div className="sd-mods-empty-desc">启用状态会按存档保存；没有当前存档时不能切换。</div>
                  </div>
                ) : userVisibleMods.length === 0 ? (
                  <div className="sd-mods-empty sd-mods-settings-empty">
                    <img className="sd-mods-empty-icon" src="/assets/stardew/ui/icons/icon_nav_mods.png" alt="" />
                    <div className="sd-mods-empty-title">当前没有可配置 Mod</div>
                    <div className="sd-mods-empty-desc">安装第三方 Mod 后可以在这里为当前存档启用或禁用。</div>
                  </div>
                ) : (
                  <div className="sd-mods-enable-list">
                    {userVisibleMods.map((mod) => {
                      const busy = enableUpdating === mod.id
                      const toggleDisabled = writeDisabled || !activeSaveName || !mod.canToggle || busy
                      const title = mod.enableNote || writeTitle || (mod.enabled ? '禁用此 Mod' : '启用此 Mod')
                      const requiredDependency = dependencyDisplay(mod)
                      return (
                        <div className="sd-mods-enable-row" key={`enable-${mod.id}`}>
                          <div className="sd-mods-enable-main">
                            <span className="sd-mods-enable-name">{modDisplayName(mod)}</span>
                            <span className="sd-mods-enable-meta">
                              {mod.uniqueId || mod.folderName}
                            </span>
                            {requiredDependency ? (
                              <div className="sd-mods-enable-dependencies">
                                <span className={`sd-tag ${requiredDependency.className} sd-mods-dependency-tag`} title={requiredDependency.title}>
                                  {requiredDependency.label}
                                </span>
                              </div>
                            ) : null}
                          </div>
                          <div className="sd-mods-enable-tags">
                            {mod.builtIn ? <span className="sd-tag sd-tag-blue">内置</span> : null}
                            <span className={`sd-tag ${mod.enabled ? 'sd-tag-green' : 'sd-tag-gold'}`}>
                              {mod.enabled ? '已启用' : '已禁用'}
                            </span>
                          </div>
                          <label className={`sd-mod-toggle${mod.enabled ? ' sd-mod-toggle-on' : ''}${toggleDisabled ? ' sd-mod-toggle-disabled' : ''}`} title={title}>
                            <input
                              type="checkbox"
                              checked={mod.enabled}
                              disabled={toggleDisabled}
                              onChange={(e) => handleEnabledChange(mod, e.currentTarget.checked)}
                            />
                            <span className="sd-mod-toggle-track" aria-hidden="true">
                              <span className="sd-mod-toggle-thumb" />
                            </span>
                          </label>
                        </div>
                      )
                    })}
                  </div>
                )}
              </section>
              <section className="sd-mods-settings-right">
                <div className="sd-mods-section-title">说明</div>
                <div className="sd-mods-empty sd-mods-settings-empty">
                  <img
                    className="sd-mods-empty-icon"
                    src="/assets/stardew/ui/icons/icon_nav_mods.png"
                    alt=""
                  />
                  <div className="sd-mods-empty-title">当前阶段仅切换启用状态</div>
                  <div className="sd-mods-empty-desc">
                    服务器停止时可切换。新建或新导入的存档默认只启用内置组件，第三方 Mod 需要在这里手动打开。
                  </div>
                </div>
              </section>
            </div>
          ) : null}
        </div>
      </div>

      {showUpload && (
        <div className="sd-mods-modal-overlay" onClick={closeUpload}>
          <div
            className="sd-mods-modal-card"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="sd-mods-modal-header">
              <span className="sd-mods-modal-title">上传 Mod</span>
            </div>

            {uploadError && (
              <div className="sd-mods-list-error">{uploadError}</div>
            )}

            <p className="sd-mods-upload-hint">
              上传一个包含 SMAPI Mod 的 ZIP 文件。ZIP 内应包含一个或多个 Mod 文件夹，每个文件夹中有 manifest.json。
              保持服务器停止完成修改，下次启动时会自动加载。
            </p>

            <label className="sd-mods-upload-label">
              选择 ZIP 文件
              <input
                ref={fileInputRef}
                type="file"
                accept=".zip"
                multiple
                className="sd-mods-upload-input"
                disabled={uploadBusy}
                onChange={(e) => setUploadFiles(Array.from(e.target.files ?? []))}
              />
            </label>

            {uploadFiles.length > 0 && (
              <div className="sd-mods-upload-filename">
                已选择：{uploadFiles.length} 个 ZIP（{(uploadFiles.reduce((sum, file) => sum + file.size, 0) / 1024).toFixed(1)} KB）
                {uploadFiles.length <= 5 ? `：${uploadFiles.map((file) => file.name).join('、')}` : ''}
              </div>
            )}

            <div className="sd-mods-modal-actions">
              <button
                className="sd-btn-tan"
                disabled={uploadBusy}
                type="button"
                onClick={closeUpload}
              >
                取消
              </button>
              <button
                className="sd-btn-green"
                disabled={uploadBusy || uploadFiles.length === 0}
                onClick={handleUpload}
                type="button"
              >
                {uploadBusy ? '上传中…' : '上传并安装'}
              </button>
            </div>
          </div>
        </div>
      )}

      {showNexusKeyModal && (
        <div className="sd-mods-modal-overlay" onClick={closeNexusKeyModal}>
          <div
            className="sd-mods-modal-card"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="sd-mods-modal-header">
              <span className="sd-mods-modal-title">
                配置 Nexus API Key
              </span>
            </div>

            {nexusKeyError && <div className="sd-mods-list-error">{nexusKeyError}</div>}
            {nexusKeyMessage && <div className="sd-mods-success-banner">{nexusKeyMessage}</div>}

            <p className="sd-mods-upload-hint">
              粘贴后会保存到面板数据库，立即生效；页面只会显示末 4 位，不会回显完整 Key。
            </p>

            <input
              className="sd-input"
              type="password"
              value={nexusKeyInput}
              autoComplete="off"
              placeholder={nexusSettings?.configured
                ? '输入新的 Key 可覆盖当前配置'
                : '粘贴 Nexus API Key'}
              disabled={nexusKeyBusy}
              onChange={(e) => setNexusKeyInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') void handleNexusKeySave() }}
            />

            <div className="sd-mods-modal-actions">
              <button
                className="sd-btn-tan"
                disabled={nexusKeyBusy}
                type="button"
                onClick={closeNexusKeyModal}
              >
                取消
              </button>
              <button
                className="sd-btn-delete"
                disabled={nexusKeyBusy || !nexusSettings?.configured}
                onClick={handleNexusKeyDelete}
                type="button"
              >
                删除 Key
              </button>
              <button
                className="sd-btn-green"
                disabled={nexusKeyBusy || !nexusKeyInput.trim()}
                onClick={handleNexusKeySave}
                type="button"
              >
                {nexusKeyBusy ? '保存中…' : '保存'}
              </button>
            </div>
          </div>
        </div>
      )}

      {confirmDelete && (
        <div className="sd-confirm-overlay">
          <div className="sd-confirm-dialog">
            <h3>确认删除 Mod</h3>
            {deleteBundleCompanions.length > 0 ? (
              <>
                <p>
                  <strong>「{modDisplayName(confirmDelete)}」</strong>
                  是同一个 Nexus 安装包的一部分，删除时需要和同包 Mod 一起删除。
                </p>
                <div className="sd-mods-delete-bundle">
                  {deleteBundle.map((mod) => (
                    <span className="sd-tag sd-tag-gold" key={mod.folderName}>
                      {modDisplayName(mod)}
                    </span>
                  ))}
                </div>
                <p>是否确认一起删除？此操作不可恢复。</p>
              </>
            ) : (
              <p>
                确定要删除
                <strong>「{modDisplayName(confirmDelete)}」</strong>吗？
                此操作不可恢复。
              </p>
            )}
            {deleteError && (
              <div className="sd-mods-list-error">{deleteError}</div>
            )}
            <div className="sd-confirm-actions">
              <button
                className="sd-btn-tan"
                disabled={deleteBusy}
                onClick={closeDeleteConfirm}
                type="button"
              >
                取消
              </button>
              <button
                className="sd-btn-delete"
                disabled={deleteBusy}
                onClick={handleDeleteConfirm}
                type="button"
              >
                {deleteBusy ? '删除中…' : '确认删除'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
import './ModsPage.css'
