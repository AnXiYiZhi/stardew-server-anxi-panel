import { useCallback, useEffect, useRef, useState } from 'react'
import { getMods, uploadMod, deleteMod, exportMods } from '../../../api'
import { errorMessage } from '../../../core/helpers'
import type { ModInfo, ModsListResult } from '../../../types'
import type { StardewPageProps } from '../stardew-routes'

// ── ModCard ───────────────────────────────────────────────────────────────────

function ModCard({
  mod,
  writeDisabled,
  writeTitle,
  onDelete,
}: {
  mod: ModInfo
  writeDisabled: boolean
  writeTitle: string
  onDelete: (mod: ModInfo) => void
}) {
  const displayName = mod.name ?? mod.folderName

  return (
    <div className={`sd-mods-card${mod.parseError ? ' sd-mods-card-error' : ''}`}>
      <div className="sd-mods-card-main">
        <div className="sd-mods-card-header">
          <span className="sd-mods-card-name">{displayName}</span>
          {mod.version ? <span className="sd-mods-card-version">v{mod.version}</span> : null}
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

// ── ModsPage ──────────────────────────────────────────────────────────────────

export function ModsPage({ user, instanceState, dashboardData }: StardewPageProps) {
  const [data, setData] = useState<ModsListResult | null>(dashboardData.mods)
  const [loading, setLoading] = useState(false)
  const [listError, setListError] = useState<string | null>(null)

  // Upload state
  const [showUpload, setShowUpload] = useState(false)
  const [uploadFile, setUploadFile] = useState<File | null>(null)
  const [uploadBusy, setUploadBusy] = useState(false)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [uploadSuccess, setUploadSuccess] = useState(false)

  // Delete confirm state
  const [confirmDelete, setConfirmDelete] = useState<ModInfo | null>(null)
  const [deleteBusy, setDeleteBusy] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)

  // Export state
  const [exportBusy, setExportBusy] = useState(false)
  const [exportError, setExportError] = useState<string | null>(null)

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

  // 初次挂载加载（如果 dashboardData 已有数据则跳过首次请求）
  useEffect(() => {
    if (!dashboardData.mods) {
      void loadMods()
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  // dashboardData.mods 更新时同步本地 state
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
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
    } catch (e) {
      setExportError(errorMessage(e))
    } finally {
      setExportBusy(false)
    }
  }

  function closeUpload() {
    if (uploadBusy) return
    setShowUpload(false)
    setUploadFile(null)
    setUploadError(null)
  }

  // ── render ─────────────────────────────────────────────────────────────────

  return (
    <div className="sd-page">
      {/* 页头 */}
      <div className="sd-mods-header">
        <div className="sd-mods-header-left">
          <img
            className="sd-page-icon"
            src="/assets/stardew/ui/icons/icon_nav_mods.png"
            alt=""
          />
          <div>
            <h2 className="sd-page-title">模组管理</h2>
            <p className="sd-page-desc">安装、删除和导出 SMAPI 模组</p>
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
            onClick={() => { setUploadError(null); setShowUpload(true) }}
            type="button"
            title={writeTitle || '上传 ZIP 包安装 Mod'}
          >
            上传 Mod
          </button>
        </div>
      </div>

      {/* 运行中警示 */}
      {isRunning && (
        <div className="sd-mods-running-hint">
          ⚠ 服务器运行中，Mod 上传和删除已暂时禁用 — 请先停止服务器
        </div>
      )}

      {/* 上传成功提示 */}
      {uploadSuccess && (
        <div className="sd-mods-success-banner">
          ✔ Mod 上传成功 — 变更需要重启服务器后生效
        </div>
      )}

      {/* 概览卡片 */}
      <div className="sd-mods-overview">
        <div className="sd-mods-stat">
          <span className="sd-mods-stat-label">已安装</span>
          <span className="sd-mods-stat-value">{loading ? '—' : `${mods.length} 个`}</span>
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
            {restartRequired ? '⚠ 需要重启' : '无'}
          </span>
        </div>
        <div className="sd-mods-stat">
          <span className="sd-mods-stat-label">解析失败</span>
          <span className="sd-mods-stat-value">
            {loading ? '—' : `${mods.filter((m) => m.parseError).length} 个`}
          </span>
        </div>
      </div>

      {/* 重启横幅 */}
      {restartRequired && (
        <div className="sd-mods-restart-banner">
          ⚠ Mod 已变更 — 需要重启服务器才能生效
        </div>
      )}

      {/* 列表错误 */}
      {listError && (
        <div className="sd-mods-list-error">{listError}</div>
      )}

      {/* 导出错误 */}
      {exportError && (
        <div className="sd-mods-list-error">{exportError}</div>
      )}

      {/* 模组列表 */}
      <div className="sd-mods-section-title">
        已安装模组
        {loading && <span className="sd-mods-loading-tag">加载中…</span>}
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
            />
          ))}
        </div>
      )}

      {/* 待接入功能区 */}
      <div className="sd-mods-section-title" style={{ marginTop: 16 }}>
        高级功能
        <span className="sd-mods-pending-badge">待接入</span>
      </div>
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

      {/* 上传弹窗 */}
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
                {uploadBusy ? '上传中…' : '上传并安装'}
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

      {/* 删除确认弹窗 */}
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
                {deleteBusy ? '删除中…' : '确认删除'}
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
