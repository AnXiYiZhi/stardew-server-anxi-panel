import { useCallback, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { getMods, uploadMods, deleteMod, exportMods, updateModSyncClassification, updateModEnabled, exportModSyncPack, exportModSyncUpdatePack, searchNexusMods, installRemoteMod, getNexusSettings, saveNexusAPIKey, deleteNexusAPIKey, createJobEventSource, getJob } from '../../../api'
import { errorMessage, formatDate } from '../../../core/helpers'
import type { JobLog, ModInfo, ModsListResult, ModSearchResult, ModSyncKind, NexusModSearchResult, NexusRequiredMod, NexusSettingsStatus } from '../../../types'
import type { StardewPageProps } from '../stardew-routes'

type ModWorkbenchTab = 'download' | 'installed' | 'settings'

const NEXUS_SEARCH_PAGE_SIZE = 20
const SMAPI_NEXUS_MOD_ID = 2400
const SMAPI_NEXUS_URL = `https://www.nexusmods.com/stardewvalley/mods/${SMAPI_NEXUS_MOD_ID}`

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

function ModSearchResultCard({
  result,
  actionSlot,
  footerSlot,
}: {
  result: ModSearchResult
  actionSlot?: ReactNode
  footerSlot?: ReactNode
}) {
  return (
    <div className="sd-mods-nexus-card">
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

function nexusRequiredModToNexusResult(required: NexusRequiredMod): NexusModSearchResult {
  return {
    modId: required.modId,
    name: required.name,
    summary: required.notes,
    endorsementCount: 0,
    downloadCount: 0,
    nexusUrl: required.nexusUrl,
    installed: required.installed,
    installedEnabled: required.installedEnabled,
    installedFolderName: required.installedFolderName,
    installedVersion: required.installedVersion,
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
      name: mod.name ?? mod.folderName,
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
    name: mod.name ?? mod.folderName,
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

function modHasNexusPresentation(mod: ModInfo) {
  if (modIsSmapi(mod)) return true
  if ((mod.nexusModId ?? 0) > 0) return true
  return mod.originSource === 'nexus' && (mod.originNexusModId ?? 0) > 0
}

function modDisplayName(mod: ModInfo) {
  return mod.name ?? mod.folderName
}

function modIsSmapi(mod: ModInfo) {
  const uniqueId = mod.uniqueId?.trim().toLowerCase()
  const folderName = mod.folderName?.trim().toLowerCase()
  const name = mod.name?.trim().toLowerCase()
  return mod.id === '__smapi_runtime' ||
    uniqueId === 'pathoschild.smapi' ||
    folderName === 'smapi' ||
    name === 'smapi'
}

function modIsPanelControl(mod: ModInfo) {
  return mod.folderName === 'StardewAnxiPanel.Control' ||
    mod.uniqueId === 'AnXiYiZhi.StardewAnxiPanel.Control'
}

function modCountsForPlayerSync(mod: ModInfo) {
  if (modIsSmapi(mod)) return true
  return !mod.builtIn
}

function builtInRank(mod: ModInfo) {
  if (modIsSmapi(mod)) return 0
  if (modIsPanelControl(mod)) return 1
  return 2
}

function modBundleKey(mod: ModInfo) {
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
  const [activeTab, setActiveTab] = useState<ModWorkbenchTab>('download')
  const [data, setData] = useState<ModsListResult | null>(dashboardData.mods)
  const [loading, setLoading] = useState(false)
  const [listError, setListError] = useState<string | null>(null)

  const [showUpload, setShowUpload] = useState(false)
  const [uploadFiles, setUploadFiles] = useState<File[]>([])
  const [uploadBusy, setUploadBusy] = useState(false)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [uploadSuccess, setUploadSuccess] = useState(false)

  const [confirmDelete, setConfirmDelete] = useState<ModInfo | null>(null)
  const [deleteBusy, setDeleteBusy] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  const [exportBusy, setExportBusy] = useState(false)
  const [exportError, setExportError] = useState<string | null>(null)

  const [syncUpdating, setSyncUpdating] = useState<string | null>(null)
  const [syncError, setSyncError] = useState<string | null>(null)
  const [syncPackBusy, setSyncPackBusy] = useState<'full' | 'update' | null>(null)
  const [syncPackError, setSyncPackError] = useState<string | null>(null)
  const [enableUpdating, setEnableUpdating] = useState<string | null>(null)
  const [enableError, setEnableError] = useState<string | null>(null)

  const [nexusQuery, setNexusQuery] = useState('')
  const [nexusLoading, setNexusLoading] = useState(false)
  const [nexusError, setNexusError] = useState<string | null>(null)
  const [nexusResults, setNexusResults] = useState<NexusModSearchResult[] | null>(null)
  const [nexusPage, setNexusPage] = useState(1)
  const [nexusTotal, setNexusTotal] = useState(0)
  const [nexusHasMore, setNexusHasMore] = useState(false)
  const [nexusPageInput, setNexusPageInput] = useState('1')
  const [nexusSettings, setNexusSettings] = useState<NexusSettingsStatus | null>(null)
  const [nexusSettingsLoading, setNexusSettingsLoading] = useState(false)
  const [showNexusKeyModal, setShowNexusKeyModal] = useState(false)
  const [nexusKeyInput, setNexusKeyInput] = useState('')
  const [nexusKeyBusy, setNexusKeyBusy] = useState(false)
  const [nexusKeyError, setNexusKeyError] = useState<string | null>(null)
  const [nexusKeyMessage, setNexusKeyMessage] = useState<string | null>(null)
  const [nexusInstallingModId] = useState<string | null>(null)
  const [nexusInstallJobId, setNexusInstallJobId] = useState<string | null>(null)
  const [nexusInstallLogs, setNexusInstallLogs] = useState<JobLog[]>([])
  const [nexusInstallError, setNexusInstallError] = useState<string | null>(null)
  const [showRemoteInstallModal, setShowRemoteInstallModal] = useState(false)
  const [remoteInstallURL, setRemoteInstallURL] = useState('')
  const [remoteInstallBusy, setRemoteInstallBusy] = useState(false)

  const fileInputRef = useRef<HTMLInputElement>(null)
  const nexusInstallEventSourceRef = useRef<EventSource | null>(null)
  const defaultNexusLoadedRef = useRef(false)

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

  const mods = sortInstalledMods(data?.mods ?? [])
  const displayedInstalledMods = mods.filter(modHasNexusPresentation)
  const hiddenLocalMods = mods.filter((mod) => !modHasNexusPresentation(mod))
  const syncableMods = mods.filter(modCountsForPlayerSync)
  const restartRequired = data?.restartRequired ?? false
  const parseErrorCount = mods.filter((m) => m.parseError).length
  const deleteBundle = confirmDelete ? modBundleMembers(mods, confirmDelete) : []
  const deleteBundleCompanions = confirmDelete
    ? deleteBundle.filter((mod) => mod.folderName !== confirmDelete.folderName)
    : []
  const syncSummary = {
    serverOnly: syncableMods.filter((m) => m.syncKind === 'server_only').length,
    clientRequired: syncableMods.filter((m) => m.syncKind === 'client_required').length,
    unknown: syncableMods.filter((m) => m.syncKind !== 'server_only' && m.syncKind !== 'client_required').length,
  }
  const syncPackagedClientRequired = syncableMods.filter((m) => m.syncKind === 'client_required' && !m.builtIn).length
  const nexusTotalPages = Math.max(1, Math.ceil(nexusTotal / NEXUS_SEARCH_PAGE_SIZE))

  const tabItems: Array<{ id: ModWorkbenchTab; label: string; hint: string }> = [
    { id: 'download', label: '下载模组', hint: '搜索 N 站并准备安装' },
    { id: 'installed', label: '添加模组', hint: '本服已安装与玩家同步' },
    { id: 'settings', label: '配置模组', hint: '启用、依赖与配置入口' },
  ]

  const loadMods = useCallback(async () => {
    setLoading(true)
    setListError(null)
    try {
      const result = await getMods()
      setData(result)
      return result
    } catch (e) {
      setListError(errorMessage(e))
      return null
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!dashboardData.mods) {
      void loadMods()
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (dashboardData.mods) {
      setData(dashboardData.mods)
    }
  }, [dashboardData.mods])

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
      nexusInstallEventSourceRef.current?.close()
      nexusInstallEventSourceRef.current = null
    }
  }, [])

  async function handleUpload() {
    if (uploadFiles.length === 0) return
    setUploadBusy(true)
    setUploadError(null)
    setUploadSuccess(false)
    try {
      await uploadMods(uploadFiles)
      await loadMods()
      dashboardData.refreshMods()
      setShowUpload(false)
      setUploadFiles([])
      if (fileInputRef.current) fileInputRef.current.value = ''
      setUploadSuccess(true)
      setTimeout(() => setUploadSuccess(false), 4000)
    } catch (e) {
      setUploadError(errorMessage(e))
    } finally {
      setUploadBusy(false)
    }
  }

  function openDeleteConfirm(mod: ModInfo) {
    setDeleteError(null)
    setConfirmDelete(mod)
  }

  async function handleDeleteConfirm() {
    if (!confirmDelete) return
    setDeleteBusy(true)
    setDeleteError(null)
    try {
      await deleteMod(confirmDelete.id)
      dashboardData.refreshMods()
      await loadMods()
      setConfirmDelete(null)
    } catch (e) {
      setDeleteError(errorMessage(e))
    } finally {
      setDeleteBusy(false)
    }
  }

  async function handleExport() {
    setExportBusy(true)
    setExportError(null)
    try {
      const { blob, filename } = await exportMods()
      downloadBlob(blob, filename)
    } catch (e) {
      setExportError(errorMessage(e))
    } finally {
      setExportBusy(false)
    }
  }

  async function handleSyncChange(mod: ModInfo, syncKind: ModSyncKind) {
    setSyncError(null)
    setSyncUpdating(mod.id)
    try {
      const result = await updateModSyncClassification(mod.id, syncKind)
      const updates = new Map(result.mods.map((item) => [item.folderName, item]))
      setData((prev) =>
        prev ? {
          ...prev,
          mods: prev.mods.map((m) => {
            const updated = updates.get(m.folderName)
            return updated
              ? { ...m, syncKind: updated.syncKind, syncNote: updated.syncNote }
              : m
          }),
        } : prev,
      )
      dashboardData.refreshMods()
    } catch (e) {
      setSyncError(errorMessage(e))
    } finally {
      setSyncUpdating(null)
    }
  }

  async function handleEnabledChange(mod: ModInfo, enabled: boolean) {
    setEnableError(null)
    setEnableUpdating(mod.id)
    try {
      const result = await updateModEnabled(mod.id, enabled, activeSaveName || undefined)
      const updates = new Map(result.mods.map((item) => [item.folderName, item]))
      setData((prev) =>
        prev ? {
          ...prev,
          mods: prev.mods.map((m) => {
            const updated = updates.get(m.folderName)
            return updated
              ? {
                  ...m,
                  enabled: updated.enabled,
                  canToggle: updated.canToggle,
                  enableNote: updated.enableNote,
                  dependencies: updated.dependencies ?? m.dependencies,
                }
              : m
          }),
        } : prev,
      )
      dashboardData.refreshMods()
    } catch (e) {
      setEnableError(errorMessage(e))
    } finally {
      setEnableUpdating(null)
    }
  }

  async function handleSyncPackExport(kind: 'full' | 'update') {
    setSyncPackBusy(kind)
    setSyncPackError(null)
    try {
      const { blob, filename } = kind === 'update'
        ? await exportModSyncUpdatePack()
        : await exportModSyncPack()
      downloadBlob(blob, filename)
    } catch (e) {
      setSyncPackError(errorMessage(e))
    } finally {
      setSyncPackBusy(null)
    }
  }

  const handleNexusSearch = useCallback(async (page = 1, queryOverride?: string) => {
    const query = (queryOverride ?? nexusQuery).trim()
    setNexusLoading(true)
    setNexusError(null)
    try {
      const result = await searchNexusMods(query, page, NEXUS_SEARCH_PAGE_SIZE)
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
  }, [nexusQuery])

  useEffect(() => {
    if (activeTab !== 'download' || defaultNexusLoadedRef.current) return
    defaultNexusLoadedRef.current = true
    void handleNexusSearch(1, '')
  }, [activeTab, handleNexusSearch])

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

  function renderNexusPager(position: 'top' | 'bottom') {
    const atFirstPage = nexusPage <= 1
    const atLastPage = nexusPage >= nexusTotalPages || !nexusHasMore
    return (
      <div className={`sd-mods-nexus-total sd-mods-nexus-total-${position}`}>
        <span>共 {nexusTotal.toLocaleString()} 个结果 · 第 {nexusPage} / {nexusTotalPages} 页</span>
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

  function installFailureMessage(message: string | null | undefined) {
    const text = message || 'Mod 安装失败'
    if (text.includes('status 403') || text.includes('403')) {
      return `${text}。非 Premium 账号请粘贴 Nexus 生成的 nxm:// 链接，或浏览器下载得到的 nexus-cdn .zip 临时链接继续安装。`
    }
    return text
  }

  function subscribeInstallJob(jobId: string, result: NexusModSearchResult | null, onDone: () => void) {
    setNexusInstallJobId(jobId)
    const es = createJobEventSource(jobId)
    nexusInstallEventSourceRef.current = es

    es.addEventListener('log', (ev) => {
      try {
        const entry = JSON.parse((ev as MessageEvent<string>).data) as JobLog
        setNexusInstallLogs((prev) => (
          prev.some((item) => item.sequence === entry.sequence)
            ? prev
            : [...prev, { ...entry, jobId }]
        ))
      } catch {
        // Ignore malformed SSE payloads; the job page remains the source of truth.
      }
    })

    es.addEventListener('finished', () => {
      es.close()
      if (nexusInstallEventSourceRef.current === es) {
        nexusInstallEventSourceRef.current = null
      }
      void getJob(jobId).then((jobResponse) => {
        if (jobResponse.job.status === 'failed') {
          setNexusInstallError(installFailureMessage(jobResponse.job.errorMessage))
        } else if (jobResponse.job.status === 'succeeded') {
          if (result) {
            setNexusResults((prev) => prev?.map((item) => (
              item.modId === result.modId
                ? { ...item, installed: true, installedEnabled: true, installedVersion: result.version }
                : item
            )) ?? prev)
          }
          setActiveTab('installed')
          void loadMods().then(() => dashboardData.refreshMods())
        }
      }).finally(onDone)
    })

    es.onerror = () => {
      es.close()
      if (nexusInstallEventSourceRef.current === es) {
        nexusInstallEventSourceRef.current = null
      }
      setNexusInstallError('安装进度连接已断开，可以在任务页查看最新状态')
      onDone()
    }
  }

  function handleNexusInstall(result: NexusModSearchResult) {
    if (nexusInstallingModId !== null || remoteInstallBusy) return
    setNexusInstallError(null)
    setNexusInstallLogs([])
    setNexusInstallJobId(null)
    nexusInstallEventSourceRef.current?.close()
    nexusInstallEventSourceRef.current = null
    window.location.assign(nexusExtensionInstallURL(result))
  }

  async function handleRemoteInstall() {
    const url = remoteInstallURL.trim()
    if (!url || remoteInstallBusy) return
    setRemoteInstallBusy(true)
    setNexusInstallError(null)
    setNexusInstallLogs([])
    setNexusInstallJobId(null)
    nexusInstallEventSourceRef.current?.close()
    nexusInstallEventSourceRef.current = null

    try {
      const response = await installRemoteMod({ url })
      setShowRemoteInstallModal(false)
      setRemoteInstallURL('')
      subscribeInstallJob(response.jobId, null, () => setRemoteInstallBusy(false))
    } catch (e) {
      setNexusInstallError(errorMessage(e))
      setRemoteInstallBusy(false)
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

  function closeUpload() {
    if (uploadBusy) return
    setShowUpload(false)
    setUploadFiles([])
    if (fileInputRef.current) fileInputRef.current.value = ''
    setUploadError(null)
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

  function searchedModCanInstall(result: NexusModSearchResult) {
    if (!isAdmin || isRunning || result.installed || nexusInstallingModId !== null || remoteInstallBusy) {
      return false
    }
    return true
  }

  function searchedModInstallTitle(result: NexusModSearchResult) {
    if (result.installed && result.installedEnabled === false) return '该 Mod 已安装，但当前存档未启用，可到配置模组中启用'
    if (result.installed) return '该 Mod 已安装'
    if (!isAdmin) return '仅管理员可以安装 Mod'
    if (isRunning) return '服务器运行中，请先停止后安装 Mod'
    if (nexusInstallingModId !== null || remoteInstallBusy) return '已有安装任务正在进行'
    return '打开 Nexus 下载页，浏览器扩展会自动获取 ZIP 链接'
  }

  function searchedModInstallLabel(result: NexusModSearchResult, installing: boolean) {
    if (installing) return '打开中...'
    if (result.installed && result.installedEnabled === false) return '已安装未启用'
    if (result.installed) return '已安装'
    return '一键安装'
  }

  return (
    <div className="sd-page">
      <div className="sd-mods-header">
        <div className="sd-mods-header-left">
          <img
            className="sd-page-icon"
            src="/assets/stardew/ui/icons/icon_nav_mods.png"
            alt=""
          />
          <div>
            <h2 className="sd-page-title">模组管理</h2>
            <p className="sd-page-desc">搜索、安装、同步和配置 SMAPI 模组</p>
          </div>
        </div>
        <div className="sd-mods-header-actions">
          <button
            className="sd-btn-tan"
            disabled={loading}
            onClick={loadMods}
            type="button"
            title="刷新 Mod 列表"
          >
            {loading ? '刷新中…' : '刷新列表'}
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
            onClick={() => { setUploadError(null); setShowUpload(true); setActiveTab('installed') }}
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
              <span className="sd-mods-tab-label">{tab.label}</span>
              <span className="sd-mods-tab-hint">{tab.hint}</span>
            </button>
          ))}
        </div>

        <div className="sd-mods-tab-panel" role="tabpanel">
          {activeTab === 'download' ? (
            <>
              <div className="sd-mods-panel-head">
                <div>
                  <div className="sd-mods-section-title">搜索 Nexus Mods</div>
                  <p className="sd-mods-sync-hint">
                    默认展示 N 站近期热门 20 个模组，也可以输入名称或 ID 搜索。
                  </p>
                </div>
                <div className="sd-mods-panel-actions">
                  {isAdmin && (
                    <>
                      <span className={`sd-tag ${nexusSettings?.configured ? 'sd-tag-green' : 'sd-tag-gold'}`}>
                        {nexusSettingsLoading
                          ? 'Nexus Key 读取中'
                          : nexusSettings?.configured
                            ? `Nexus Key 已配置${nexusSettings.last4 ? ` · ${nexusSettings.last4}` : ''}`
                            : 'Nexus Key 未配置'}
                      </span>
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
                        className="sd-btn-green"
                        type="button"
                        onClick={() => { setRemoteInstallURL(''); setShowRemoteInstallModal(true) }}
                        disabled={isRunning || remoteInstallBusy || nexusInstallingModId !== null}
                        title={isRunning ? '服务器运行中，请先停止后安装 Mod' : '粘贴 Nexus nxm:// 或 nexus-cdn .zip 临时链接安装'}
                      >
                        {remoteInstallBusy ? '安装中...' : '粘贴链接安装'}
                      </button>
                    </>
                  )}
                  <span className="sd-mods-pending-badge">当前 N站 GraphQL v2 可直接搜索</span>
                </div>
              </div>

              <div className="sd-mods-nexus-search-row">
                <select className="sd-mods-search-type" value="text" disabled title="按 Nexus 关键词或数字 ID 搜索">
                  <option value="text">名称 / ID</option>
                </select>
                <input
                  className="sd-input"
                  type="text"
                  placeholder="输入 Mod 名称、唯一 ID 或站点 ID 搜索；留空刷新热门..."
                  value={nexusQuery}
                  onChange={(e) => setNexusQuery(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') void handleNexusSearch(1) }}
                />
                <button
                  className="sd-btn-tan"
                  disabled={nexusLoading}
                  onClick={() => void handleNexusSearch(1)}
                  type="button"
                  title={nexusQuery.trim() ? '搜索 Nexus Mods' : '刷新近期热门 Nexus Mods'}
                >
                  {nexusLoading ? '搜索中...' : nexusQuery.trim() ? '搜索' : '刷新热门'}
                </button>
              </div>

              {nexusError && <div className="sd-mods-list-error">{nexusError}</div>}
              {nexusInstallError && <div className="sd-mods-list-error">{nexusInstallError}</div>}
              {(nexusInstallJobId || nexusInstallLogs.length > 0) && (
                <div className="sd-mods-install-log">
                  <div className="sd-mods-install-log-head">
                    <span>安装进度</span>
                    {nexusInstallJobId ? <span className="sd-mods-install-job">#{nexusInstallJobId}</span> : null}
                  </div>
                  {nexusInstallLogs.length > 0 ? (
                    <div className="sd-mods-install-log-lines">
                      {nexusInstallLogs.slice(-8).map((log) => (
                        <div className={`sd-mods-install-log-line sd-mods-install-log-${log.level}`} key={log.sequence}>
                          <span>{log.message}</span>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="sd-mods-install-log-empty">等待安装任务开始...</div>
                  )}
                </div>
              )}
              {nexusLoading ? (
                <div className="sd-mods-nexus-skeleton-grid">
                  {Array.from({ length: 6 }, (_, i) => (
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
                    <div className="sd-mods-nexus-list">
                      {nexusResults.map((r) => {
                        const installing = nexusInstallingModId === String(r.modId)
                        const canInstall = searchedModCanInstall(r)
                        const requiredMods = r.requiredMods ?? []
                        const missingRequiredMods = missingNexusRequiredMods(r)
                        return (
                          <ModSearchResultCard
                            key={r.modId}
                            result={nexusResultToSearchResult(r)}
                            actionSlot={(
                              <button
                                className="sd-btn-green"
                                type="button"
                                disabled={!canInstall}
                                title={searchedModInstallTitle(r)}
                                onClick={() => void handleNexusInstall(r)}
                              >
                                {searchedModInstallLabel(r, installing)}
                              </button>
                            )}
                            footerSlot={requiredMods.length > 0 ? (
                              <div className="sd-mods-installed-footer">
                                {missingRequiredMods.length > 0 ? (
                                  <span className="sd-tag sd-tag-red sd-mods-dependency-tag" title={`缺少前置：${missingRequiredMods.map((dep) => dep.name).join('、')}`}>
                                    缺少前置 {missingRequiredMods.length} 个
                                  </span>
                                ) : (
                                  <span className="sd-tag sd-tag-green sd-mods-dependency-tag">
                                    前置已满足
                                  </span>
                                )}
                                {requiredMods.map((required) => {
                                  const requiredResult = nexusRequiredModToNexusResult(required)
                                  const canInstallRequired = searchedModCanInstall(requiredResult)
                                  return (
                                    <span className="sd-mods-required-pill" key={`${r.modId}-required-${required.modId}`}>
                                      <span
                                        className={`sd-tag ${nexusRequiredStatusClass(required)} sd-mods-dependency-tag`}
                                        title={required.notes || required.name}
                                      >
                                        {nexusRequiredStatusLabel(required)}：{required.name}
                                      </span>
                                      {!required.installed ? (
                                        <button
                                          className="sd-btn-tan sd-mods-required-install"
                                          type="button"
                                          disabled={!canInstallRequired}
                                          title={searchedModInstallTitle(requiredResult)}
                                          onClick={() => void handleNexusInstall(requiredResult)}
                                        >
                                          安装前置
                                        </button>
                                      ) : null}
                                    </span>
                                  )
                                })}
                              </div>
                            ) : undefined}
                          />
                        )
                      })}
                    </div>
                    {renderNexusPager('bottom')}
                  </>
                )
              ) : (
                <div className="sd-mods-nexus-empty">输入名称或 ID 后开始搜索。本页会显示来源、跳转入口和可用的一键安装方式。</div>
              )}
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
                  ✔ Mod 上传成功 - 下次启动服务器时会自动加载
                </div>
              )}

              <div className="sd-mods-overview">
                <div className="sd-mods-stat">
                  <span className="sd-mods-stat-label">已安装</span>
                  <span className="sd-mods-stat-value">{loading ? '-' : `${mods.length} 个`}</span>
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
                  {syncPackBusy === 'full' ? '导出中...' : '导出完整同步包'}
                </button>
                <button
                  className="sd-btn-tan"
                  disabled={syncPackBusy !== null || syncPackagedClientRequired === 0}
                  onClick={() => handleSyncPackExport('update')}
                  type="button"
                  title={syncPackagedClientRequired === 0 ? '暂无可打包的玩家 Mod' : '已运行过同步包的玩家使用，不包含 SMAPI'}
                >
                  {syncPackBusy === 'update' ? '导出中...' : '导出模组更新包'}
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
                已安装 Nexus 模组
                {loading && <span className="sd-mods-loading-tag">加载中...</span>}
              </div>

              {!loading && mods.length === 0 ? (
                <div className="sd-mods-empty">
                  <img
                    className="sd-mods-empty-icon"
                    src="/assets/stardew/ui/icons/icon_nav_mods.png"
                    alt=""
                  />
                  <div className="sd-mods-empty-title">当前没有安装 Mod</div>
                  <div className="sd-mods-empty-desc">
                    上传包含 SMAPI Mod 的 ZIP 文件来安装模组。
                    每个 Mod 文件夹中应包含 manifest.json。
                  </div>
                  <button
                    className="sd-btn-green"
                    disabled={writeDisabled}
                    title={writeTitle || '上传 ZIP 包安装 Mod'}
                    onClick={() => { setUploadError(null); setShowUpload(true) }}
                    type="button"
                  >
                    上传 Mod
                  </button>
                </div>
              ) : !loading && displayedInstalledMods.length === 0 ? (
                <div className="sd-mods-empty">
                  <img
                    className="sd-mods-empty-icon"
                    src="/assets/stardew/ui/icons/icon_nav_mods.png"
                    alt=""
                  />
                  <div className="sd-mods-empty-title">暂无 Nexus 来源数据</div>
                  <div className="sd-mods-empty-desc">
                    通过 N 站安装，或上传带 Nexus UpdateKeys 的模组后会显示在这里。
                  </div>
                </div>
              ) : (
                <>
                  {hiddenLocalMods.length > 0 ? (
                    <div className="sd-mods-nexus-only-note">
                      已隐藏 {hiddenLocalMods.length} 个本地文件项
                    </div>
                  ) : null}
                  <div className="sd-mods-nexus-list">
                    {displayedInstalledMods.map((mod) => {
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
                            {syncBusy && <span className="sd-mods-loading-tag">更新中...</span>}
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
                ) : mods.length === 0 ? (
                  <div className="sd-mods-empty sd-mods-settings-empty">
                    <img className="sd-mods-empty-icon" src="/assets/stardew/ui/icons/icon_nav_mods.png" alt="" />
                    <div className="sd-mods-empty-title">当前没有 Mod</div>
                    <div className="sd-mods-empty-desc">安装 Mod 后可以在这里为当前存档启用或禁用。</div>
                  </div>
                ) : (
                  <div className="sd-mods-enable-list">
                    {mods.map((mod) => {
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
              <button
                className="sd-btn-tan"
                type="button"
                disabled={uploadBusy}
                onClick={closeUpload}
              >
                关闭
              </button>
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
                className="sd-btn-green"
                disabled={uploadBusy || uploadFiles.length === 0}
                onClick={handleUpload}
                type="button"
              >
                {uploadBusy ? '上传中...' : '上传并安装'}
              </button>
              <button
                className="sd-btn-tan"
                disabled={uploadBusy}
                type="button"
                onClick={closeUpload}
              >
                取消
              </button>
            </div>
          </div>
        </div>
      )}

      {showRemoteInstallModal && (
        <div className="sd-mods-modal-overlay" onClick={() => { if (!remoteInstallBusy) setShowRemoteInstallModal(false) }}>
          <div
            className="sd-mods-modal-card"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="sd-mods-modal-header">
              <span className="sd-mods-modal-title">粘贴链接安装</span>
              <button
                className="sd-btn-tan"
                type="button"
                disabled={remoteInstallBusy}
                onClick={() => setShowRemoteInstallModal(false)}
              >
                关闭
              </button>
            </div>

            <p className="sd-mods-upload-hint">
              支持 Nexus 网站生成的 nxm:// 链接，也支持浏览器下载时得到的 Nexus CDN HTTPS .zip 临时链接。链接只用于当前安装任务，不会写入审计日志。
            </p>

            <input
              className="sd-input"
              type="text"
              value={remoteInstallURL}
              autoComplete="off"
              placeholder="nxm://... 或 https://moddrop.com/...zip "
              disabled={remoteInstallBusy}
              onChange={(e) => setRemoteInstallURL(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') void handleRemoteInstall() }}
            />

            <div className="sd-mods-modal-actions">
              <button
                className="sd-btn-green"
                disabled={remoteInstallBusy || !remoteInstallURL.trim()}
                onClick={handleRemoteInstall}
                type="button"
              >
                {remoteInstallBusy ? '安装中...' : '开始安装'}
              </button>
              <button
                className="sd-btn-tan"
                disabled={remoteInstallBusy}
                type="button"
                onClick={() => setShowRemoteInstallModal(false)}
              >
                取消
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
              <button
                className="sd-btn-tan"
                type="button"
                disabled={nexusKeyBusy}
                onClick={closeNexusKeyModal}
              >
                关闭
              </button>
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
                className="sd-btn-green"
                disabled={nexusKeyBusy || !nexusKeyInput.trim()}
                onClick={handleNexusKeySave}
                type="button"
              >
                {nexusKeyBusy ? '保存中...' : '保存并生效'}
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
                className="sd-btn-tan"
                disabled={nexusKeyBusy}
                type="button"
                onClick={closeNexusKeyModal}
              >
                取消
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
                className="sd-btn-delete"
                disabled={deleteBusy}
                onClick={handleDeleteConfirm}
                type="button"
              >
                {deleteBusy ? '删除中...' : '确认删除'}
              </button>
              <button
                className="sd-btn-tan"
                disabled={deleteBusy}
                onClick={() => { setConfirmDelete(null); setDeleteError(null) }}
                type="button"
              >
                取消
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
