import { useCallback, useEffect, useState } from 'react'
import type { ModInfo, ModsListResult } from '../../types'
import { getMods, uploadMods, deleteMod, exportMods } from '../../api'
import { errorMessage } from '../../core/helpers'

// ── ModRow ────────────────────────────────────────────────────────────────────

function ModRow({
  mod,
  busy,
  onDelete,
}: {
  mod: ModInfo
  busy: boolean
  onDelete: () => void
}) {
  return (
    <div className="mod-row">
      <div className="mod-row-info">
        <div className="mod-row-name">
          <strong>{mod.name ?? mod.folderName}</strong>
          {mod.version ? <span className="mod-version">v{mod.version}</span> : null}
        </div>
        {mod.parseError ? (
          <div className="mod-row-error">解析失败：{mod.parseError}</div>
        ) : (
          <div className="mod-row-meta">
            {mod.uniqueId ? <span>ID：{mod.uniqueId}</span> : null}
            {mod.author ? <span>作者：{mod.author}</span> : null}
            {mod.description ? <span className="mod-description">{mod.description}</span> : null}
          </div>
        )}
      </div>
      <div className="mod-row-actions">
        <button
          className="button button-small button-danger"
          disabled={busy}
          onClick={onDelete}
          type="button"
        >
          删除
        </button>
      </div>
    </div>
  )
}

// ── ModsSection ───────────────────────────────────────────────────────────────

export function ModsSection({
  state,
  isAdmin,
  refreshTrigger,
}: {
  state: string
  isAdmin: boolean
  refreshTrigger?: number
}) {
  const [data, setData] = useState<ModsListResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  // Upload state
  const [showUploadModal, setShowUploadModal] = useState(false)
  const [uploadFiles, setUploadFiles] = useState<File[]>([])
  const [uploadBusy, setUploadBusy] = useState(false)
  const [uploadMessage, setUploadMessage] = useState('')

  const loadMods = useCallback(async () => {
    setLoading(true)
    setMessage('')
    try {
      const result = await getMods()
      setData(result)
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadMods()
  }, [loadMods])

  useEffect(() => {
    if (refreshTrigger && refreshTrigger > 0) {
      void loadMods()
    }
  }, [refreshTrigger, loadMods])

  async function handleUpload() {
    if (uploadFiles.length === 0) return
    setUploadBusy(true)
    setUploadMessage('')
    try {
      await uploadMods(uploadFiles)
      setShowUploadModal(false)
      setUploadFiles([])
      await loadMods()
    } catch (error) {
      setUploadMessage(errorMessage(error))
    } finally {
      setUploadBusy(false)
    }
  }

  async function handleDelete(mod: ModInfo) {
    const displayName = mod.name ?? mod.folderName
    if (!window.confirm(`确定删除 Mod"${displayName}"吗？此操作不可恢复。`)) return
    setBusy(true)
    setMessage('')
    try {
      await deleteMod(mod.id)
      await loadMods()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleExport() {
    setBusy(true)
    setMessage('')
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
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  const mods = data?.mods ?? []
  const hasMods = mods.length > 0
  const isRunning = state === 'running' || state === 'starting'
  const restartRequired = data?.restartRequired ?? false

  return (
    <section className="mods-section">
      <div className="section-heading">
        <div>
          <h2>Mod 管理</h2>
          <p>管理服务器 Mod：上传、删除、导出。</p>
        </div>
        <div className="mods-heading-actions">
          <button className="button button-small button-secondary" disabled={loading} onClick={loadMods} type="button">
            {loading ? '刷新中...' : '刷新列表'}
          </button>
          {isAdmin ? (
            <>
              <button
                className="button button-small"
                disabled={busy || isRunning}
                onClick={() => setShowUploadModal(true)}
                type="button"
                title={isRunning ? '请先停止服务器再上传 Mod' : ''}
              >
                上传 Mod
              </button>
              <button
                className="button button-small button-secondary"
                disabled={busy || !hasMods}
                onClick={handleExport}
                type="button"
              >
                导出 Mod
              </button>
            </>
          ) : null}
        </div>
      </div>

      {restartRequired ? (
        <div className="mods-restart-banner">
          ⚠️ Mod 变更需要重启服务器生效
        </div>
      ) : null}

      {message ? <div className="error-banner">{message}</div> : null}

      {isRunning ? (
        <div className="mods-running-hint">
          服务器运行中，Mod 上传和删除功能已禁用。请先停止服务器。
        </div>
      ) : null}

      {hasMods ? (
        <div className="mods-list">
          {mods.map((mod) => (
            <ModRow
              key={mod.id}
              mod={mod}
              busy={busy || loading || isRunning}
              onDelete={() => handleDelete(mod)}
            />
          ))}
        </div>
      ) : !loading ? (
        <div className="empty-mods">
          <p className="empty-mods-title">当前没有安装 Mod</p>
          <p className="empty-mods-hint">
            上传包含 SMAPI Mod 的 ZIP 文件来安装 Mod。
            <br />
            Mod 安装后需要重启服务器才能生效。
          </p>
        </div>
      ) : null}

      {/* Upload modal */}
      {showUploadModal ? (
        <div className="modal-overlay">
          <div className="modal-card" style={{ maxWidth: 500 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
              <h3 style={{ margin: 0 }}>上传 Mod</h3>
              <button
                className="button button-small button-secondary"
                type="button"
                onClick={() => { setShowUploadModal(false); setUploadFiles([]); setUploadMessage('') }}
                disabled={uploadBusy}
              >
                关闭
              </button>
            </div>
            {uploadMessage ? <div className="error-banner">{uploadMessage}</div> : null}
            <div className="form-grid">
              <p className="form-hint">
                上传一个包含 SMAPI Mod 的 ZIP 文件。
                <br />
                ZIP 内应包含 Mod 文件夹，每个文件夹中有 manifest.json。
              </p>
              <label className="field-label">
                选择 ZIP 文件
                <input
                  type="file"
                  accept=".zip"
                  multiple
                  onChange={(e) => setUploadFiles(Array.from(e.target.files ?? []))}
                />
              </label>
              {uploadFiles.length > 0 ? (
                <div className="form-hint">
                  已选择 {uploadFiles.length} 个 ZIP
                </div>
              ) : null}
              <div className="modal-actions">
                <button
                  className="button"
                  disabled={uploadBusy || uploadFiles.length === 0}
                  onClick={handleUpload}
                  type="button"
                >
                  {uploadBusy ? '上传中...' : '上传并安装'}
                </button>
                <button
                  className="button button-secondary"
                  disabled={uploadBusy}
                  type="button"
                  onClick={() => { setShowUploadModal(false); setUploadFiles([]); setUploadMessage('') }}
                >
                  取消
                </button>
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </section>
  )
}
