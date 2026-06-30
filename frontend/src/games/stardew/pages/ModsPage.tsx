import { useCallback, useEffect, useRef, useState } from 'react'
import { getMods, uploadMod, deleteMod, exportMods, updateModSyncClassification, exportModSyncPack, searchNexusMods } from '../../../api'
import { errorMessage, formatDate } from '../../../core/helpers'
import type { ModInfo, ModsListResult, ModSyncKind, NexusModSearchResult } from '../../../types'
import type { StardewPageProps } from '../stardew-routes'

type ModWorkbenchTab = 'download' | 'installed' | 'settings'

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

function ModCard({
  mod,
  writeDisabled,
  writeTitle,
  onDelete,
  isAdmin,
  syncBusy,
  onSyncChange,
}: {
  mod: ModInfo
  writeDisabled: boolean
  writeTitle: string
  onDelete: (mod: ModInfo) => void
  isAdmin: boolean
  syncBusy: boolean
  onSyncChange: (mod: ModInfo, syncKind: ModSyncKind) => void
}) {
  const displayName = mod.name ?? mod.folderName

  return (
    <div className={`sd-mods-card${mod.parseError ? ' sd-mods-card-error' : ''}`}>
      <div className="sd-mods-card-main">
        <div className="sd-mods-card-header">
          <span className="sd-mods-card-name">{displayName}</span>
          {mod.version ? <span className="sd-mods-card-version">v{mod.version}</span> : null}
        </div>

        <div className="sd-mods-sync-row">
          <span className={`sd-tag ${SYNC_KIND_TAG_CLASS[mod.syncKind]}`}>
            {SYNC_KIND_LABELS[mod.syncKind]}
          </span>
          {isAdmin && (
            <select
              className="sd-mods-sync-select"
              value={mod.syncKind}
              disabled={syncBusy}
              onChange={(e) => onSyncChange(mod, e.target.value as ModSyncKind)}
            >
              <option value="server_only">服务器专用</option>
              <option value="client_required">玩家需同步</option>
              <option value="unknown">待确认</option>
            </select>
          )}
          {syncBusy && <span className="sd-mods-loading-tag">更新中…</span>}
        </div>

        {mod.parseError ? (
          <div className="sd-mods-parse-error">解析失败：{mod.parseError}</div>
        ) : (
          <div className="sd-mods-card-meta">
            {mod.uniqueId ? <span className="sd-mods-meta-item"><span className="sd-mods-meta-label">ID</span>{mod.uniqueId}</span> : null}
            {mod.author ? <span className="sd-mods-meta-item"><span className="sd-mods-meta-label">作者</span>{mod.author}</span> : null}
            {mod.folderName ? <span className="sd-mods-meta-item"><span className="sd-mods-meta-label">目录</span>{mod.folderName}</span> : null}
            {mod.description ? <span className="sd-mods-meta-desc">{mod.description}</span> : null}
          </div>
        )}
      </div>

      <div className="sd-mods-card-actions">
        <button
          className="sd-btn-delete"
          disabled={writeDisabled}
          title={writeTitle || '删除此 Mod'}
          onClick={() => onDelete(mod)}
          type="button"
        >
          删除
        </button>
      </div>
    </div>
  )
}

function NexusResultCard({ result }: { result: NexusModSearchResult }) {
  return (
    <div className="sd-mods-nexus-card">
      <div className="sd-mods-nexus-card-pic-wrap">
        {result.pictureUrl ? (
          <img className="sd-mods-nexus-card-pic" src={result.pictureUrl} alt="" />
        ) : (
          <img className="sd-mods-nexus-card-pic sd-mods-nexus-card-pic-empty" src="/assets/stardew/ui/icons/icon_nav_mods.png" alt="" />
        )}
      </div>
      <div className="sd-mods-nexus-card-main">
        <div className="sd-mods-nexus-card-header">
          <span className="sd-mods-nexus-card-name">{result.name}</span>
          {result.installed ? (
            <span className="sd-tag sd-tag-green" title={result.installedFolderName ?? ''}>
              已安装{result.installedVersion ? ` v${result.installedVersion}` : ''}
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
          <span>下载 {result.downloadCount.toLocaleString()}</span>
          <span>认可 {result.endorsementCount.toLocaleString()}</span>
        </div>
      </div>
      <div className="sd-mods-card-actions">
        <button
          className="sd-btn-tan"
          type="button"
          onClick={() => window.open(result.nexusUrl, '_blank', 'noopener,noreferrer')}
        >
          打开 N 站
        </button>
        <span className="sd-mods-pending-badge" title="安装功能尚未接入">安装待接入</span>
      </div>
    </div>
  )
}

export function ModsPage({ user, instanceState, dashboardData }: StardewPageProps) {
  const [activeTab, setActiveTab] = useState<ModWorkbenchTab>('download')
  const [data, setData] = useState<ModsListResult | null>(dashboardData.mods)
  const [loading, setLoading] = useState(false)
  const [listError, setListError] = useState<string | null>(null)

  const [showUpload, setShowUpload] = useState(false)
  const [uploadFile, setUploadFile] = useState<File | null>(null)
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
  const [syncPackBusy, setSyncPackBusy] = useState(false)
  const [syncPackError, setSyncPackError] = useState<string | null>(null)

  const [nexusQuery, setNexusQuery] = useState('')
  const [nexusLoading, setNexusLoading] = useState(false)
  const [nexusError, setNexusError] = useState<string | null>(null)
  const [nexusResults, setNexusResults] = useState<NexusModSearchResult[] | null>(null)

  const fileInputRef = useRef<HTMLInputElement>(null)

  const isAdmin = user.role === 'admin'
  const state = instanceState?.state ?? null
  const isRunning = state === 'running' || state === 'starting'
  const writeDisabled = isRunning || !isAdmin
  const writeTitle = !isAdmin
    ? '仅管理员可用'
    : isRunning
      ? '服务器运行中，请先停止后操作'
      : ''

  const mods = data?.mods ?? []
  const restartRequired = data?.restartRequired ?? false
  const parseErrorCount = mods.filter((m) => m.parseError).length
  const syncSummary = {
    serverOnly: mods.filter((m) => m.syncKind === 'server_only').length,
    clientRequired: mods.filter((m) => m.syncKind === 'client_required').length,
    unknown: mods.filter((m) => m.syncKind !== 'server_only' && m.syncKind !== 'client_required').length,
  }

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
    } catch (e) {
      setListError(errorMessage(e))
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

  async function handleUpload() {
    if (!uploadFile) return
    setUploadBusy(true)
    setUploadError(null)
    setUploadSuccess(false)
    try {
      await uploadMod(uploadFile)
      await loadMods()
      dashboardData.refreshMods()
      setShowUpload(false)
      setUploadFile(null)
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
      await updateModSyncClassification(mod.id, syncKind)
      setData((prev) =>
        prev ? { ...prev, mods: prev.mods.map((m) => (m.id === mod.id ? { ...m, syncKind } : m)) } : prev,
      )
      dashboardData.refreshMods()
    } catch (e) {
      setSyncError(errorMessage(e))
    } finally {
      setSyncUpdating(null)
    }
  }

  async function handleSyncPackExport() {
    setSyncPackBusy(true)
    setSyncPackError(null)
    try {
      const { blob, filename } = await exportModSyncPack()
      downloadBlob(blob, filename)
    } catch (e) {
      setSyncPackError(errorMessage(e))
    } finally {
      setSyncPackBusy(false)
    }
  }

  async function handleNexusSearch() {
    const query = nexusQuery.trim()
    if (!query) return
    setNexusLoading(true)
    setNexusError(null)
    try {
      const result = await searchNexusMods(query)
      setNexusResults(result.results)
    } catch (e) {
      setNexusError(errorMessage(e))
      setNexusResults(null)
    } finally {
      setNexusLoading(false)
    }
  }

  function closeUpload() {
    if (uploadBusy) return
    setShowUpload(false)
    setUploadFile(null)
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
                  <div className="sd-mods-section-title">在线搜索（Nexus Mods）</div>
                  <p className="sd-mods-sync-hint">
                    搜索 Stardew Valley 的 Nexus Mods 结果并查看基础信息；本阶段仅支持只读搜索和跳转 N 站，暂不支持安装。
                  </p>
                </div>
                <span className="sd-mods-pending-badge">安装待接入</span>
              </div>

              <div className="sd-mods-nexus-search-row">
                <select className="sd-mods-search-type" value="text" disabled title="当前后端统一按关键词或数字 ID 搜索">
                  <option value="text">名称 / ID</option>
                </select>
                <input
                  className="sd-input"
                  type="text"
                  placeholder="输入 Mod 名称或 Nexus Mod ID 搜索..."
                  value={nexusQuery}
                  onChange={(e) => setNexusQuery(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') void handleNexusSearch() }}
                />
                <button
                  className="sd-btn-tan"
                  disabled={nexusLoading || !nexusQuery.trim()}
                  onClick={handleNexusSearch}
                  type="button"
                  title={!nexusQuery.trim() ? '请输入搜索关键词' : '搜索 Nexus Mods'}
                >
                  {nexusLoading ? '搜索中...' : '搜索'}
                </button>
              </div>

              {nexusError && <div className="sd-mods-list-error">{nexusError}</div>}
              {nexusLoading ? (
                <div className="sd-mods-nexus-skeleton-grid">
                  {Array.from({ length: 6 }, (_, i) => (
                    <div className="sd-mods-nexus-skeleton" key={i} />
                  ))}
                </div>
              ) : nexusResults ? (
                nexusResults.length === 0 ? (
                  <div className="sd-mods-nexus-empty">未找到匹配的 Mod，换个关键词试试。</div>
                ) : (
                  <>
                    <div className="sd-mods-nexus-total">共 {nexusResults.length} 个结果</div>
                    <div className="sd-mods-nexus-list">
                      {nexusResults.map((r) => (
                        <NexusResultCard key={r.modId} result={r} />
                      ))}
                    </div>
                  </>
                )
              ) : (
                <div className="sd-mods-nexus-empty">输入名称或 ID 后开始搜索。本页会显示可预览、可跳转的在线模组卡片。</div>
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
                  ✔ Mod 上传成功 - 变更需要重启服务器后生效
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
                  <span className="sd-mods-stat-label">重启需求</span>
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
                  disabled={syncPackBusy || syncSummary.clientRequired === 0}
                  onClick={handleSyncPackExport}
                  type="button"
                  title={syncSummary.clientRequired === 0 ? '暂无需要玩家同步的 Mod' : '导出玩家需同步的 Mod 为 ZIP'}
                >
                  {syncPackBusy ? '导出中...' : '导出玩家同步包'}
                </button>
              </div>
              <p className="sd-mods-sync-hint">
                为每个 Mod 标记是否需要玩家在客户端同步安装。服务器专用 Mod（如面板控制 Mod）不会包含在导出包中。
              </p>
              {syncError && <div className="sd-mods-list-error">{syncError}</div>}
              {syncPackError && <div className="sd-mods-list-error">{syncPackError}</div>}

              {restartRequired && (
                <div className="sd-mods-restart-banner">
                  ⚠ Mod 已变更 - 需要重启服务器才能生效
                </div>
              )}
              {listError && <div className="sd-mods-list-error">{listError}</div>}
              {exportError && <div className="sd-mods-list-error">{exportError}</div>}

              <div className="sd-mods-section-title">
                已安装模组
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
              ) : (
                <div className="sd-mods-list">
                  {mods.map((mod) => (
                    <ModCard
                      key={mod.id}
                      mod={mod}
                      writeDisabled={writeDisabled}
                      writeTitle={writeTitle}
                      onDelete={openDeleteConfirm}
                      isAdmin={isAdmin}
                      syncBusy={syncUpdating === mod.id}
                      onSyncChange={handleSyncChange}
                    />
                  ))}
                </div>
              )}
            </>
          ) : null}

          {activeTab === 'settings' ? (
            <div className="sd-mods-settings-layout">
              <section className="sd-mods-settings-left">
                <div className="sd-mods-section-title">高级功能</div>
                <div className="sd-mods-pending-grid">
                  <div className="sd-mods-pending-item">
                    <button className="sd-btn-tan" disabled type="button" title="后端暂未支持">
                      启用 / 禁用
                    </button>
                    <span className="sd-mods-pending-desc">按 Mod 单独启用或禁用（后端待接入）</span>
                  </div>
                  <div className="sd-mods-pending-item">
                    <button className="sd-btn-tan" disabled type="button" title="后端暂未支持">
                      依赖检查
                    </button>
                    <span className="sd-mods-pending-desc">检查 Mod 依赖是否完整（后端待接入）</span>
                  </div>
                  <div className="sd-mods-pending-item">
                    <button className="sd-btn-tan" disabled type="button" title="后端暂未支持">
                      更新检查
                    </button>
                    <span className="sd-mods-pending-desc">检查是否有可用更新（后端待接入）</span>
                  </div>
                </div>
              </section>
              <section className="sd-mods-settings-right">
                <div className="sd-mods-section-title">配置面板</div>
                <div className="sd-mods-empty sd-mods-settings-empty">
                  <img
                    className="sd-mods-empty-icon"
                    src="/assets/stardew/ui/icons/icon_nav_mods.png"
                    alt=""
                  />
                  <div className="sd-mods-empty-title">请选择一个可配置的 Mod</div>
                  <div className="sd-mods-empty-desc">
                    后续接入 SMAPI 配置读取后，这里会像参考面板一样展示启用模组列表和配置表单。
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
              上传成功后需要<strong>重启服务器</strong>才能生效。
            </p>

            <label className="sd-mods-upload-label">
              选择 ZIP 文件
              <input
                ref={fileInputRef}
                type="file"
                accept=".zip"
                className="sd-mods-upload-input"
                disabled={uploadBusy}
                onChange={(e) => setUploadFile(e.target.files?.[0] ?? null)}
              />
            </label>

            {uploadFile && (
              <div className="sd-mods-upload-filename">
                已选择：{uploadFile.name}（{(uploadFile.size / 1024).toFixed(1)} KB）
              </div>
            )}

            <div className="sd-mods-modal-actions">
              <button
                className="sd-btn-green"
                disabled={uploadBusy || !uploadFile}
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

      {confirmDelete && (
        <div className="sd-confirm-overlay">
          <div className="sd-confirm-dialog">
            <h3>确认删除 Mod</h3>
            <p>
              确定要删除
              <strong>「{confirmDelete.name ?? confirmDelete.folderName}」</strong>吗？
              此操作不可恢复。
            </p>
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
