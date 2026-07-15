import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  exportMods,
  getMods,
  searchNexusMods,
  updateModEnabled,
  uploadMods,
} from '../../../api'
import { errorMessage, formatDate } from '../../../core/helpers'
import type { ModInfo, ModsListResult, NexusModSearchResult, NexusRequiredMod } from '../../../types'
import { modIsPanelControl, modIsSmapi, modIsSystemRuntime } from '../mod-visibility'
import { modDisplayName } from '../mod-display'
import type { StardewPageProps } from '../stardew-routes'
import './MobileModsPage.css'

type MobileModsPageProps = Pick<StardewPageProps, 'user' | 'instanceState' | 'dashboardData'>
type MobileModsTab = 'search' | 'installed'

type NexusSearchSessionState = {
  query: string
  results: NexusModSearchResult[] | null
  page: number
  total: number
  hasMore: boolean
  pageInput: string
  updatedAt: number
}

const NEXUS_PAGE_SIZE = 4
const NEXUS_SEARCH_SESSION_KEY = 'stardew-anxi:mobile-nexus-search-state:v2'
const NEXUS_QUICK_TAGS = ['UI Info', 'Fishing Mod', 'Tractor']
const SYNC_KIND_LABELS: Record<string, string> = {
  server_only: '服务器专用',
  client_required: '玩家需同步',
  unknown: '待确认',
}

const DEPENDENCY_LABELS: Record<string, string> = {
  'Pathoschild.ContentPatcher': 'Content Patcher',
  'spacechase0.GenericModConfigMenu': 'Generic Mod Config Menu',
  'Cherry.ShopTileFramework': 'Shop Tile Framework',
  'tlitookilakin.HDPortraits': 'HD Portraits',
}

function readNexusSearchSessionState(): NexusSearchSessionState | null {
  try {
    const raw = window.sessionStorage.getItem(NEXUS_SEARCH_SESSION_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<NexusSearchSessionState>
    return {
      query: typeof parsed.query === 'string' ? parsed.query : '',
      results: Array.isArray(parsed.results) ? parsed.results as NexusModSearchResult[] : null,
      page: Number.isFinite(parsed.page) ? Number(parsed.page) : 1,
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

function dependencyIssueText(dep: NonNullable<ModInfo['dependencies']>[number]) {
  const name = dependencyLabel(dep.uniqueId)
  const minimum = dep.minimumVersion ? ` >= ${dep.minimumVersion}` : ''
  const current = dep.installedVersion ? `（当前 ${dep.installedVersion}）` : ''
  switch (dep.status) {
    case 'missing':
      return `缺失：${name}${minimum}${current}`
    case 'disabled':
      return `未启用：${name}${minimum}${current}`
    case 'version_mismatch':
      return `版本不足：${name}${minimum}${current}`
    case 'unknown_version':
      return `版本待确认：${name}${minimum}${current}`
    default:
      return `${name}${minimum}${current}`
  }
}

function dependencyDisplay(mod: ModInfo) {
  const required = (mod.dependencies ?? []).filter((dep) => dep.required && dep.uniqueId)
  if (required.length === 0) return null
  const issues = required.filter((dep) => dep.status && dep.status !== 'satisfied')
  if (issues.length > 0) {
    const names = issues.slice(0, 2).map((dep) => dependencyLabel(dep.uniqueId))
    const label = `${issues.some((dep) => dep.status === 'missing') ? '缺失前置' : '前置异常'}：${names.join('、')}${issues.length > 2 ? ` 等 ${issues.length} 个` : ''}`
    return { label, className: 'sd-tag-red', title: `前置依赖检查：${issues.map(dependencyIssueText).join('、')}` }
  }
  const names = required.slice(0, 2).map((dep) => dependencyLabel(dep.uniqueId))
  return {
    label: `前置：${names.join('、')}${required.length > 2 ? ` 等 ${required.length} 个` : ''}`,
    className: 'sd-tag-gold',
    title: `需要前置依赖：${required.map((dep) => dependencyLabel(dep.uniqueId)).join('、')}`,
  }
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

function missingNexusRequiredMods(result: NexusModSearchResult) {
  return (result.requiredMods ?? []).filter((required) => !required.installed || required.installedEnabled === false)
}

function modNexusId(mod: ModInfo): number {
  if ((mod.nexusModId ?? 0) > 0) return mod.nexusModId ?? 0
  if (mod.originSource === 'nexus' && (mod.originNexusModId ?? 0) > 0) return mod.originNexusModId ?? 0
  return 0
}

function modExternalUrl(mod: ModInfo) {
  if (mod.nexusUrl) return mod.nexusUrl
  if (mod.originModUrl) return mod.originModUrl
  const nexusId = modNexusId(mod)
  return nexusId > 0 ? `https://www.nexusmods.com/stardewvalley/mods/${nexusId}` : ''
}

function sortInstalledMods(mods: ModInfo[]) {
  return [...mods].sort((a, b) => {
    if (a.builtIn !== b.builtIn) return a.builtIn ? -1 : 1
    return modDisplayName(a).localeCompare(modDisplayName(b), 'zh-Hans')
  })
}

function nexusModInstalledMatchInList(modList: ModInfo[] | undefined, modId: number) {
  if (!modList || modId <= 0) return null
  const match = modList.find((mod) => modNexusId(mod) === modId)
  if (!match) return null
  return {
    installed: true,
    installedEnabled: match.enabled,
    installedFolderName: match.folderName,
    installedVersion: match.version,
  }
}

function NexusRequiredModsBadge({
  requiredMods,
  open,
  onToggle,
}: {
  requiredMods: NexusRequiredMod[]
  open: boolean
  onToggle: () => void
}) {
  if (requiredMods.length === 0) return <span className="sd-tag sd-tag-gold">前置：无</span>
  const missing = missingNexusRequiredMods({ requiredMods } as NexusModSearchResult)
  return (
    <div className="sd-mmods-required">
      <button
        type="button"
        className={`sd-tag ${missing.length > 0 ? 'sd-tag-red' : 'sd-tag-green'} sd-mmods-required-summary`}
        onClick={onToggle}
        aria-expanded={open}
      >
        {missing.length > 0 ? '缺少前置mod' : '前置已满足'}
      </button>
      {open ? (
        <div className="sd-mmods-required-list">
          {requiredMods.map((required) => (
            <div className="sd-mmods-required-row" key={required.modId}>
              <span>{required.name}</span>
              <span>Nexus:{required.modId}</span>
              <span className={`sd-tag ${nexusRequiredStatusClass(required)}`}>{nexusRequiredStatusLabel(required)}</span>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  )
}

export function MobileModsPage({ user, instanceState, dashboardData }: MobileModsPageProps) {
  const restoredNexusSearch = readNexusSearchSessionState()
  const [activeTab, setActiveTab] = useState<MobileModsTab>('search')
  const [data, setData] = useState<ModsListResult | null>(dashboardData.mods)
  const [loading, setLoading] = useState(false)
  const [listError, setListError] = useState<string | null>(null)

  const [exportBusy, setExportBusy] = useState(false)
  const [exportError, setExportError] = useState<string | null>(null)
  const [showUpload, setShowUpload] = useState(false)
  const [uploadFiles, setUploadFiles] = useState<File[]>([])
  const [uploadBusy, setUploadBusy] = useState(false)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [uploadSuccess, setUploadSuccess] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const [enableUpdating, setEnableUpdating] = useState<string | null>(null)
  const [enableError, setEnableError] = useState<string | null>(null)
  const [enableMessage, setEnableMessage] = useState<string | null>(null)

  const [nexusQuery, setNexusQuery] = useState(restoredNexusSearch?.query ?? '')
  const [nexusLoading, setNexusLoading] = useState(false)
  const [nexusError, setNexusError] = useState<string | null>(null)
  const [nexusResults, setNexusResults] = useState<NexusModSearchResult[] | null>(restoredNexusSearch?.results ?? null)
  const [nexusPage, setNexusPage] = useState(restoredNexusSearch?.page ?? 1)
  const [nexusTotal, setNexusTotal] = useState(restoredNexusSearch?.total ?? 0)
  const [nexusHasMore, setNexusHasMore] = useState(restoredNexusSearch?.hasMore ?? false)
  const [nexusPageInput, setNexusPageInput] = useState(restoredNexusSearch?.pageInput ?? '1')
  const [openRequiredModId, setOpenRequiredModId] = useState<number | null>(null)

  const defaultNexusLoadedRef = useRef(Boolean(restoredNexusSearch?.results))
  const installedLoadedRef = useRef(Boolean(dashboardData.mods))

  const isAdmin = user.role === 'admin'
  const state = instanceState?.state ?? null
  const isRunning = state === 'running' || state === 'starting'
  const writeDisabled = !isAdmin || isRunning
  const writeTitle = !isAdmin ? '仅管理员可用' : isRunning ? '服务器运行中，请先停止后操作' : ''
  const activeSaveName = dashboardData.saves?.activeSaveName ?? ''

  const installedMods = useMemo(() => (
    sortInstalledMods(data?.mods ?? []).filter((mod) => !mod.builtIn && !modIsSystemRuntime(mod))
  ), [data?.mods])
  const visibleModCount = installedMods.length
  const nexusTotalPages = Math.max(1, Math.ceil(nexusTotal / NEXUS_PAGE_SIZE))

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
      installedLoadedRef.current = true
      setData(dashboardData.mods)
      syncNexusResultsFromInstalledMods(dashboardData.mods.mods)
    }
  }, [dashboardData.mods]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (dashboardData.modsError) {
      setListError(dashboardData.modsError)
    }
  }, [dashboardData.modsError])

  useEffect(() => {
    writeNexusSearchSessionState({
      query: nexusQuery,
      results: nexusResults,
      page: nexusPage,
      total: nexusTotal,
      hasMore: nexusHasMore,
      pageInput: nexusPageInput,
      updatedAt: Date.now(),
    })
  }, [nexusQuery, nexusResults, nexusPage, nexusTotal, nexusHasMore, nexusPageInput])

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
        return { ...result, ...(match ?? {}), ...(requiredMods ? { requiredMods } : {}) }
      })
    })
  }

  const handleNexusSearch = useCallback(async (page = 1, queryOverride?: string) => {
    const query = (queryOverride ?? nexusQuery).trim()
    setNexusLoading(true)
    setNexusError(null)
    try {
      const result = await searchNexusMods(query, page, NEXUS_PAGE_SIZE)
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
    if (activeTab !== 'search' || defaultNexusLoadedRef.current) return
    defaultNexusLoadedRef.current = true
    void handleNexusSearch(1, '')
  }, [activeTab, handleNexusSearch])

  useEffect(() => {
    if (activeTab !== 'installed') return
    if (!installedLoadedRef.current || !data) {
      installedLoadedRef.current = true
      void loadMods()
      return
    }
    void loadMods()
  }, [activeTab]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (openRequiredModId === null || !nexusResults?.some((result) => result.modId === openRequiredModId)) {
      setOpenRequiredModId(null)
    }
  }, [openRequiredModId, nexusResults])

  async function handleRefresh() {
    if (activeTab === 'search') {
      await loadMods()
      await handleNexusSearch(nexusResults ? nexusPage : 1)
    } else {
      await loadMods()
      await dashboardData.refreshMods()
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

  async function handleUpload() {
    if (uploadFiles.length === 0) return
    setUploadBusy(true)
    setUploadError(null)
    setUploadSuccess(null)
    try {
      const result = await uploadMods(uploadFiles)
      await loadMods()
      dashboardData.refreshMods()
      setShowUpload(false)
      setUploadFiles([])
      if (fileInputRef.current) fileInputRef.current.value = ''
      const summary = result.upload
      setUploadSuccess(summary
        ? `已解析 ${summary.archiveCount} 个 ZIP，发现 ${summary.discoveredCount} 个 Mod，安装 ${summary.importedCount} 个，已启用 ${summary.enabledCount} 个。${(summary.skippedBuiltInCount ?? 0) > 0 ? `已跳过 ${summary.skippedBuiltInCount} 个 SMAPI 重复内置组件。` : ''}下次启动服务器时会自动加载。`
        : `已安装并启用 ${result.mods.length} 个 Mod，下次启动服务器时会自动加载。`)
      window.setTimeout(() => setUploadSuccess(null), 8000)
    } catch (e) {
      setUploadError(errorMessage(e))
    } finally {
      setUploadBusy(false)
    }
  }

  async function handleEnabledChange(mod: ModInfo, enabled: boolean) {
    setEnableError(null)
    setEnableMessage(null)
    setEnableUpdating(mod.id)
    try {
      const result = await updateModEnabled(mod.id, enabled, activeSaveName || undefined)
      const updates = new Map(result.mods.map((item) => [item.folderName, item]))
      setData((prev) => prev ? {
        ...prev,
        mods: prev.mods.map((current) => {
          const updated = updates.get(current.folderName)
          return updated ? {
            ...current,
            enabled: updated.enabled,
            canToggle: updated.canToggle,
            enableNote: updated.enableNote,
            dependencies: updated.dependencies ?? current.dependencies,
          } : current
        }),
      } : prev)
      setEnableMessage(enabled ? '已启用 Mod。' : '已禁用 Mod。')
      await loadMods()
      dashboardData.refreshMods()
    } catch (e) {
      setEnableError(errorMessage(e))
    } finally {
      setEnableUpdating(null)
    }
  }

  function closeUpload() {
    if (uploadBusy) return
    setShowUpload(false)
    setUploadFiles([])
    setUploadError(null)
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  function jumpToNexusPage(page: number) {
    const target = Math.min(Math.max(1, Math.trunc(page)), nexusTotalPages)
    setNexusPageInput(String(target))
    if (target !== nexusPage) void handleNexusSearch(target)
  }

  function handleNexusPageInputJump() {
    const parsed = Number.parseInt(nexusPageInput, 10)
    if (!Number.isFinite(parsed)) {
      setNexusPageInput(String(nexusPage))
      return
    }
    jumpToNexusPage(parsed)
  }

  return (
    <div className="sd-mmods-wrap">
      <div className="sd-mmods-page-header">
        <div className="sd-mmods-page-title">
          <img src="/assets/stardew/ui/icons/icon_nav_mods_crystal_image2.png" alt="" />
          模组
        </div>
        <div className="sd-mmods-header-actions">
          <button type="button" className="sd-btn-tan sd-mmods-icon-btn" onClick={() => void handleRefresh()} disabled={loading || nexusLoading} title="刷新">
            刷新
          </button>
          <button
            type="button"
            className="sd-btn-tan sd-mmods-icon-btn"
            onClick={() => void handleExport()}
            disabled={exportBusy || visibleModCount === 0}
            title={visibleModCount === 0 ? '暂无 Mod 可导出' : '导出全部 Mod 为 ZIP'}
          >
            {exportBusy ? '导出中' : '导出'}
          </button>
          <button
            type="button"
            className="sd-btn-green sd-mmods-icon-btn"
            onClick={() => setShowUpload(true)}
            disabled={writeDisabled}
            title={writeTitle || '上传 ZIP 包安装 Mod'}
          >
            上传
          </button>
        </div>
      </div>

      <div className="sd-mmods-subtabs" role="tablist" aria-label="模组子页">
        <button
          type="button"
          className={`sd-mmods-subtab${activeTab === 'search' ? ' active' : ''}`}
          onClick={() => setActiveTab('search')}
          role="tab"
          aria-selected={activeTab === 'search'}
        >
          搜索
        </button>
        <button
          type="button"
          className={`sd-mmods-subtab${activeTab === 'installed' ? ' active' : ''}`}
          onClick={() => setActiveTab('installed')}
          role="tab"
          aria-selected={activeTab === 'installed'}
        >
          服务器模组
        </button>
      </div>

      {listError ? <div className="sd-notice sd-notice--error sd-mmods-notice">{listError}</div> : null}
      {exportError ? <div className="sd-notice sd-notice--error sd-mmods-notice">{exportError}</div> : null}
      {uploadSuccess ? <div className="sd-notice sd-notice--ok sd-mmods-notice">{uploadSuccess}</div> : null}
      {(data?.compatibilityWarnings ?? []).map((warning) => (
        <div className="sd-notice sd-notice--warn sd-mmods-notice" key={`${warning.code}:${warning.saveName ?? ''}`}>
          <strong>{warning.title}</strong><br />{warning.message}
          {warning.saveName ? <><br /><small>当前存档：{warning.saveName}</small></> : null}
        </div>
      ))}

      {activeTab === 'search' ? (
        <section className="sd-panel sd-mmods-card">
          <div className="sd-mmods-search-row">
            <input
              className="sd-input sd-mmods-search-input"
              type="text"
              placeholder="英文名称、ID 或关键词"
              value={nexusQuery}
              onChange={(e) => setNexusQuery(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') void handleNexusSearch(1) }}
            />
            <button type="button" className="sd-btn-green sd-mmods-search-btn" onClick={() => void handleNexusSearch(1)} disabled={nexusLoading}>
              {nexusLoading ? '搜索中' : '搜索'}
            </button>
          </div>
          <div className="sd-mmods-quick-tags">
            <span>热门标签</span>
            {NEXUS_QUICK_TAGS.map((tag) => (
              <button key={tag} type="button" className="sd-mmods-quick-tag" disabled={nexusLoading} onClick={() => { setNexusQuery(tag); void handleNexusSearch(1, tag) }}>
                {tag}
              </button>
            ))}
          </div>

          {nexusError ? <div className="sd-notice sd-notice--error sd-mmods-notice">{nexusError}</div> : null}

          {nexusLoading ? (
            <div className="sd-mmods-empty">正在搜索 Nexus Mods...</div>
          ) : nexusResults ? (
            nexusResults.length === 0 ? (
              <div className="sd-mmods-empty">{nexusQuery.trim() ? '未找到匹配的 Mod，换个关键词试试。' : '暂时没有读取到 N 站热门模组。'}</div>
            ) : (
              <>
                <div className="sd-mmods-search-total">
                  <span>搜索结果：{nexusTotal.toLocaleString()} 个</span>
                  <span>第 {nexusPage} / {nexusTotalPages} 页</span>
                </div>
                <div className="sd-mmods-search-list">
                  {nexusResults.map((result) => {
                    const requiredOpen = openRequiredModId === result.modId
                    return (
                      <article className="sd-mmods-search-card" key={result.modId}>
                        <div className="sd-mmods-search-card-top">
                          {result.pictureUrl ? (
                            <img className="sd-mmods-search-thumb" src={result.pictureUrl} alt="" />
                          ) : (
                            <div className="sd-mmods-search-thumb sd-mmods-search-thumb-empty">NEXUS</div>
                          )}
                          <div className="sd-mmods-search-main">
                            <div className="sd-mmods-search-name">{result.name}</div>
                            <div className="sd-mmods-tags">
                              <span className="sd-tag sd-tag-blue">Nexus:{result.modId}</span>
                              {result.installed ? (
                                <span className={`sd-tag ${result.installedEnabled === false ? 'sd-tag-gold' : 'sd-tag-green'}`}>
                                  {result.installedEnabled === false ? '已安装未启用' : '已安装'}
                                </span>
                              ) : null}
                            </div>
                          </div>
                        </div>
                        <div className="sd-mmods-info-grid">
                          <span><b>版本</b>{result.version || '—'}</span>
                          <span><b>更新</b>{result.updatedAt ? formatDate(result.updatedAt) : '—'}</span>
                        </div>
                        {result.summary ? <p className="sd-mmods-desc">{result.summary}</p> : null}
                        <div className="sd-mmods-search-footer">
                          <NexusRequiredModsBadge
                            requiredMods={result.requiredMods ?? []}
                            open={requiredOpen}
                            onToggle={() => setOpenRequiredModId((current) => current === result.modId ? null : result.modId)}
                          />
                          <div className="sd-mmods-card-actions">
                            <button
                              type="button"
                              className="sd-btn-tan sd-mmods-nexus-link-btn"
                              disabled={!result.nexusUrl}
                              onClick={() => result.nexusUrl && window.open(result.nexusUrl, '_blank', 'noopener,noreferrer')}
                            >
                              跳转 N站
                            </button>
                          </div>
                        </div>
                      </article>
                    )
                  })}
                </div>
                <div className="sd-mmods-pager">
                  <button type="button" className="sd-btn-tan" disabled={nexusLoading || nexusPage <= 1} onClick={() => jumpToNexusPage(nexusPage - 1)}>上一页</button>
                  <div className="sd-mmods-page-jump">
                    <input
                      className="sd-input"
                      type="number"
                      min={1}
                      max={nexusTotalPages}
                      value={nexusPageInput}
                      disabled={nexusLoading}
                      onChange={(e) => setNexusPageInput(e.target.value)}
                      onKeyDown={(e) => { if (e.key === 'Enter') handleNexusPageInputJump() }}
                      aria-label="跳转页码"
                    />
                    <button type="button" className="sd-btn-tan" disabled={nexusLoading} onClick={handleNexusPageInputJump}>跳转</button>
                  </div>
                  <button type="button" className="sd-btn-tan" disabled={nexusLoading || nexusPage >= nexusTotalPages || !nexusHasMore} onClick={() => jumpToNexusPage(nexusPage + 1)}>下一页</button>
                </div>
              </>
            )
          ) : (
            <div className="sd-mmods-empty">输入名称或 ID 后开始搜索；空关键词会显示近期热门模组。</div>
          )}
        </section>
      ) : (
        <section className="sd-panel sd-mmods-card">
          <div className="sd-mmods-installed-summary">
            <span className="sd-tag sd-tag-blue">已安装 {visibleModCount} 个</span>
            <span className={`sd-tag ${isRunning ? 'sd-tag-green' : 'sd-tag-gold'}`}>{isRunning ? '运行中' : '已停止'}</span>
            {data?.restartRequired ? <span className="sd-tag sd-tag-red">需要重启</span> : null}
            {activeSaveName ? <span className="sd-tag sd-tag-gold">{activeSaveName}</span> : null}
          </div>
          {enableError ? <div className="sd-notice sd-notice--error sd-mmods-notice">{enableError}</div> : null}
          {enableMessage ? <div className="sd-notice sd-notice--ok sd-mmods-notice">{enableMessage}</div> : null}
          {loading && !data ? (
            <div className="sd-mmods-empty">正在读取服务器模组...</div>
          ) : installedMods.length === 0 ? (
            <div className="sd-mmods-empty">当前没有可展示的服务器模组。</div>
          ) : (
            <div className="sd-mmods-installed-list">
              {installedMods.map((mod) => {
                const busy = enableUpdating === mod.id
                const toggleDisabled = writeDisabled || !activeSaveName || !mod.canToggle || busy
                const title = mod.enableNote || writeTitle || (mod.enabled ? '禁用此 Mod' : '启用此 Mod')
                const dependency = dependencyDisplay(mod)
                const externalUrl = modExternalUrl(mod)
                const isBuiltIn = mod.builtIn === true
                const nexusId = modNexusId(mod)
                return (
                  <article className="sd-mmods-installed-card" key={mod.id}>
                    <div className="sd-mmods-installed-head">
                      {mod.pictureUrl ? (
                        <img className="sd-mmods-installed-thumb" src={mod.pictureUrl} alt="" />
                      ) : (
                        <div className="sd-mmods-installed-thumb sd-mmods-installed-thumb-empty">{nexusId > 0 ? 'NEXUS' : 'MOD'}</div>
                      )}
                      <div className="sd-mmods-installed-main">
                        <div className="sd-mmods-installed-name-row">
                          <span className="sd-mmods-installed-name">{modDisplayName(mod)}</span>
                          <span className={`sd-tag ${mod.enabled ? 'sd-tag-green' : 'sd-tag-gold'} sd-mmods-status-tag`}>{mod.enabled ? '已启用' : '已禁用'}</span>
                        </div>
                        <div className="sd-mmods-info-grid sd-mmods-installed-grid">
                          <span><b>版本</b>{mod.version || '—'}</span>
                          <span><b>文件夹</b>{mod.folderName || '—'}</span>
                          <span><b>更新</b>{mod.updatedAt ? formatDate(mod.updatedAt) : '—'}</span>
                          <span><b>同步</b>{SYNC_KIND_LABELS[mod.syncKind] ?? mod.syncKind}</span>
                        </div>
                      </div>
                    </div>
                    <div className="sd-mmods-installed-main">
                      {mod.description || mod.nexusSummary ? <p className="sd-mmods-desc">{mod.description || mod.nexusSummary}</p> : null}
                      <div className="sd-mmods-tags">
                        {isBuiltIn ? <span className="sd-tag sd-tag-blue">内置</span> : null}
                        {modIsSmapi(mod) ? <span className="sd-tag sd-tag-gold">玩家需先安装</span> : null}
                        {modIsPanelControl(mod) ? <span className="sd-tag sd-tag-blue">服务端控制</span> : null}
                        {dependency ? <span className={`sd-tag ${dependency.className}`} title={dependency.title}>{dependency.label}</span> : null}
                        {mod.parseError ? <span className="sd-tag sd-tag-red">解析失败</span> : null}
                        {externalUrl ? (
                          <button type="button" className="sd-tag sd-mmods-link-chip" onClick={() => window.open(externalUrl, '_blank', 'noopener,noreferrer')}>
                            {nexusId > 0 ? '跳转N站' : '链接'}
                          </button>
                        ) : null}
                        <label className={`sd-mmods-toggle${mod.enabled ? ' on' : ''}${toggleDisabled ? ' disabled' : ''}`} title={title}>
                          <input
                            type="checkbox"
                            checked={mod.enabled}
                            disabled={toggleDisabled}
                            onChange={(e) => void handleEnabledChange(mod, e.currentTarget.checked)}
                            aria-label={mod.enabled ? '禁用此 Mod' : '启用此 Mod'}
                          />
                          <span className="sd-mmods-toggle-track" aria-hidden="true">
                            <span className="sd-mmods-toggle-thumb" />
                          </span>
                        </label>
                      </div>
                    </div>
                  </article>
                )
              })}
            </div>
          )}
        </section>
      )}

      {showUpload ? (
        <div className="sd-mmods-dialog-overlay" role="dialog" aria-modal="true">
          <div className="sd-panel sd-mmods-dialog">
            <h3>上传 Mod</h3>
            {uploadError ? <div className="sd-notice sd-notice--error sd-mmods-notice">{uploadError}</div> : null}
            <p>上传一个或多个包含 SMAPI Mod 的 ZIP 文件。服务器停止时才能安装。</p>
            <label className="sd-mmods-field">
              <span>选择 ZIP 文件</span>
              <input
                ref={fileInputRef}
                className="sd-input"
                type="file"
                accept=".zip"
                multiple
                disabled={uploadBusy}
                onChange={(e) => setUploadFiles(Array.from(e.target.files ?? []))}
              />
            </label>
            {uploadFiles.length > 0 ? (
              <div className="sd-mmods-upload-files">
                已选择 {uploadFiles.length} 个 ZIP{uploadFiles.length <= 3 ? `：${uploadFiles.map((file) => file.name).join('、')}` : ''}
              </div>
            ) : null}
            <div className="sd-mmods-dialog-actions">
              <button type="button" className="sd-btn-tan" onClick={closeUpload} disabled={uploadBusy}>取消</button>
              <button type="button" className="sd-btn-green" onClick={() => void handleUpload()} disabled={uploadBusy || uploadFiles.length === 0}>
                {uploadBusy ? '上传中...' : '上传并安装'}
              </button>
            </div>
          </div>
        </div>
      ) : null}

    </div>
  )
}
