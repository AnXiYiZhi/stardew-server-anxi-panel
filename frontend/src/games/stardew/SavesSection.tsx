import { useCallback, useEffect, useState } from 'react'
import type { NewGameConfig, SaveInfo, SavesListResult, UploadPreviewResult } from '../../types'
import {
  defaultInstanceId,
  getSaves,
  selectSave,
  selectSaveAndStart,
  deleteSave,
  exportSave,
  createNewGame,
  uploadSavePreview,
  uploadSaveCommitAndStart,
} from '../../api'
import { errorMessage, formatBytes } from '../../core/helpers'
import { Field } from '../../core/Field'
import { NewGameCreator } from './NewGameCreator'

const seasonLabel: Record<string, string> = {
  spring: '春', summer: '夏', fall: '秋', winter: '冬',
}
const farmTypeLabel: Record<string, string> = {
  standard: '标准农场', riverland: '河畔农场', forest: '森林农场',
  hilltop: '山顶农场', wilderness: '荒野农场', fourcorners: '四角农场',
  beach: '海滩农场', meadowlands: '草地农场',
}

// ── SaveRow ───────────────────────────────────────────────────────────────────

function SaveRow({
  save,
  isActive,
  busy,
  onSelect,
  onSelectAndStart,
  onDelete,
  onExport,
}: {
  save: SaveInfo
  isActive: boolean
  busy: boolean
  onSelect: () => void
  onSelectAndStart: () => void
  onDelete: () => void
  onExport: () => void
}) {
  return (
    <div className={`save-row${isActive ? ' save-row-active' : ''}`}>
      <div className="save-row-info">
        <div className="save-row-name">
          {isActive ? <span className="save-active-badge">当前</span> : null}
          <strong>{save.name}</strong>
        </div>
        {save.parseError ? (
          <div className="save-row-error">解析失败：{save.parseError}</div>
        ) : (
          <div className="save-row-meta">
            {save.farmName ? <span>农场：{save.farmName}</span> : <span className="muted">农场名未知</span>}
            {save.farmerName ? <span>农民：{save.farmerName}</span> : <span className="muted">农民名未知</span>}
            {save.gameYear ? (
              <span>第 {save.gameYear} 年 {seasonLabel[save.gameSeason ?? ''] ?? save.gameSeason} 第 {save.gameDay} 天</span>
            ) : null}
            {save.farmType ? <span>地图：{farmTypeLabel[save.farmType] ?? save.farmType}</span> : <span className="muted">地图未知</span>}
            {save.fileSizeBytes ? <span>大小：{formatBytes(save.fileSizeBytes)}</span> : null}
            {save.modifiedAt ? <span>修改：{new Date(save.modifiedAt).toLocaleString()}</span> : null}
          </div>
        )}
      </div>
      <div className="save-row-actions">
        {!isActive ? (
          <button className="button button-small" disabled={busy} onClick={onSelect} type="button">
            设为启动存档
          </button>
        ) : null}
        <button className="button button-small button-secondary" disabled={busy} onClick={onSelectAndStart} type="button">
          {isActive ? '使用此存档启动' : '选择并启动'}
        </button>
        <button className="button button-small button-secondary" disabled={busy} onClick={onExport} type="button">
          导出
        </button>
        {!isActive ? (
          <button
            className="button button-small button-danger"
            disabled={busy}
            onClick={onDelete}
            type="button"
          >
            删除
          </button>
        ) : null}
      </div>
    </div>
  )
}

// ── SavesSection ──────────────────────────────────────────────────────────────

export function SavesSection({
  state,
  isAdmin,
  onJobStarted,
  onStateRefresh,
  refreshTrigger,
}: {
  state: string
  isAdmin: boolean
  onJobStarted: (jobId: string) => void
  onStateRefresh: () => void
  refreshTrigger?: number
}) {
  const [data, setData] = useState<SavesListResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  // New game modal
  const [showNewGameModal, setShowNewGameModal] = useState(false)
  const [newGameError, setNewGameError] = useState('')

  // Upload modal
  const [showUploadModal, setShowUploadModal] = useState(false)
  const [uploadFile, setUploadFile] = useState<File | null>(null)
  const [uploadPreview, setUploadPreview] = useState<UploadPreviewResult | null>(null)
  const [uploadBusy, setUploadBusy] = useState(false)
  const [uploadMessage, setUploadMessage] = useState('')

  const loadSaves = useCallback(async () => {
    setLoading(true)
    setMessage('')
    try {
      const result = await getSaves()
      setData(result)
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadSaves()
  }, [loadSaves])

  // Refresh saves list when refreshTrigger changes (e.g., after a job completes).
  useEffect(() => {
    if (refreshTrigger && refreshTrigger > 0) {
      void loadSaves()
    }
  }, [refreshTrigger, loadSaves])

  // Auto-scroll to saves when state is save_required
  useEffect(() => {
    if (state === 'save_required') {
      document.getElementById('saves-section')?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }, [state])

  async function handleSelect(name: string) {
    setBusy(true)
    setMessage('')
    try {
      await selectSave(name)
      await loadSaves()
      onStateRefresh()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleSelectAndStart(name: string) {
    setBusy(true)
    setMessage('')
    try {
      const res = await selectSaveAndStart(name)
      onJobStarted(res.jobId)
      onStateRefresh()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleDelete(name: string) {
    if (!window.confirm(`确定删除存档"${name}"吗？此操作不可恢复。`)) return
    setBusy(true)
    setMessage('')
    try {
      await deleteSave(name)
      await loadSaves()
      onStateRefresh()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleExport(name: string) {
    setBusy(true)
    setMessage('')
    try {
      const { blob, filename } = await exportSave(name)
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

  async function handleNewGameSubmit(cfg: NewGameConfig) {
    setBusy(true)
    setNewGameError('')
    try {
      const res = await createNewGame(cfg)
      setShowNewGameModal(false)
      onJobStarted(res.jobId)
      onStateRefresh()
    } catch (error) {
      setNewGameError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  // Upload handlers
  async function handleUploadPreview() {
    if (!uploadFile) return
    setUploadBusy(true)
    setUploadMessage('')
    try {
      const res = await uploadSavePreview(uploadFile)
      setUploadPreview(res)
    } catch (error) {
      setUploadMessage(errorMessage(error))
    } finally {
      setUploadBusy(false)
    }
  }

  async function handleUploadCommit() {
    if (!uploadPreview) return
    setUploadBusy(true)
    setUploadMessage('')
    try {
      const res = await uploadSaveCommitAndStart(uploadPreview.token)
      setShowUploadModal(false)
      setUploadPreview(null)
      setUploadFile(null)
      onJobStarted(res.jobId)
      onStateRefresh()
    } catch (error) {
      setUploadMessage(errorMessage(error))
    } finally {
      setUploadBusy(false)
    }
  }

  function handleUploadCancel() {
    if (uploadPreview) {
      // Best-effort cleanup of pending token
      void uploadSaveCommitAndStart(uploadPreview.token, true).catch(() => {})
    }
    setShowUploadModal(false)
    setUploadPreview(null)
    setUploadFile(null)
    setUploadMessage('')
  }

  const saves = data?.saves ?? []
  const hasSaves = saves.length > 0
  const isRunning = state === 'running' || state === 'starting'

  return (
    <section id="saves-section" className="saves-section">
      <div className="section-heading">
        <div>
          <h2>存档管理</h2>
          <p>管理服务器存档：查看、选择、创建或上传。</p>
        </div>
        <div className="saves-heading-actions">
          <button className="button button-small button-secondary" disabled={loading} onClick={loadSaves} type="button">
            {loading ? '刷新中...' : '刷新列表'}
          </button>
          {isAdmin && !isRunning ? (
            <>
              <button className="button button-small" disabled={busy} onClick={() => setShowNewGameModal(true)} type="button">
                创建存档
              </button>
              <button className="button button-small button-secondary" disabled={busy} onClick={() => setShowUploadModal(true)} type="button">
                上传存档
              </button>
            </>
          ) : null}
        </div>
      </div>

      {message ? <div className="error-banner">{message}</div> : null}

      {data?.activeSaveName ? (
        <div className="saves-active-hint">
          <span>下次启动将加载：</span>
          <strong>{data.activeSaveName}</strong>
        </div>
      ) : null}

      {/* Save list or empty state */}
      {hasSaves ? (
        <div className="saves-list">
          {saves.map((save) => (
            <SaveRow
              key={save.name}
              save={save}
              isActive={save.isActive ?? false}
              busy={busy || loading}
              onSelect={() => handleSelect(save.name)}
              onSelectAndStart={() => handleSelectAndStart(save.name)}
              onDelete={() => handleDelete(save.name)}
              onExport={() => handleExport(save.name)}
            />
          ))}
        </div>
      ) : !loading ? (
        <div className="empty-saves">
          <p className="empty-saves-title">当前没有存档</p>
          <p className="empty-saves-hint">
            你可以创建一个新存档，或从本地上传已有的 Stardew Valley 存档。
            <br />
            Junimo 首次生成世界可能需要 5-15 分钟。
          </p>
          {isAdmin ? (
            <div className="empty-saves-actions">
              <button className="button" disabled={busy} onClick={() => setShowNewGameModal(true)} type="button">
                创建存档并启动
              </button>
              <button className="button button-secondary" disabled={busy} onClick={() => setShowUploadModal(true)} type="button">
                上传存档并启动
              </button>
            </div>
          ) : null}
        </div>
      ) : null}

      {/* New game modal */}
      {showNewGameModal ? (
        <div className="modal-overlay">
          <div className="modal-card" style={{ maxWidth: 1180, width: 'calc(100vw - 32px)', maxHeight: '92vh', overflowX: 'auto' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
              <h3 style={{ margin: 0 }}>新建游戏</h3>
              <button className="button button-small button-secondary" type="button"
                onClick={() => { setShowNewGameModal(false); setNewGameError('') }}>
                关闭
              </button>
            </div>
            <NewGameCreator
              instanceId={defaultInstanceId}
              onSubmit={handleNewGameSubmit}
              submitting={busy}
              submitError={newGameError}
            />
          </div>
        </div>
      ) : null}

      {/* Upload modal */}
      {showUploadModal ? (
        <div className="modal-overlay">
          <div className="modal-card" style={{ maxWidth: 600 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
              <h3 style={{ margin: 0 }}>上传存档</h3>
              <button className="button button-small button-secondary" type="button"
                onClick={handleUploadCancel} disabled={uploadBusy}>
                关闭
              </button>
            </div>
            {uploadMessage ? <div className="error-banner">{uploadMessage}</div> : null}
            {!uploadPreview ? (
              <div className="form-grid">
                <p className="form-hint">上传一个包含 Stardew Valley 存档的 ZIP 文件（最大 100 MB）。</p>
                <Field label="选择 ZIP 文件">
                  <input type="file" accept=".zip"
                    onChange={(e) => setUploadFile(e.target.files?.[0] ?? null)} />
                </Field>
                <div className="modal-actions">
                  <button className="button" disabled={uploadBusy || !uploadFile} onClick={handleUploadPreview} type="button">
                    {uploadBusy ? '解析中...' : '预览存档'}
                  </button>
                  <button className="button button-secondary" disabled={uploadBusy} type="button"
                    onClick={handleUploadCancel}>
                    取消
                  </button>
                </div>
              </div>
            ) : (
              <div>
                <p className="preflight-heading">存档预览</p>
                <div className="upload-preview-detail">
                  <div className="upload-preview-row">
                    <span className="upload-preview-label">存档目录名</span>
                    <strong>{uploadPreview.saveName}</strong>
                  </div>
                  {uploadPreview.preview.farmName ? (
                    <div className="upload-preview-row">
                      <span className="upload-preview-label">农场名</span>
                      <span>{uploadPreview.preview.farmName}</span>
                    </div>
                  ) : null}
                  {uploadPreview.preview.farmerName ? (
                    <div className="upload-preview-row">
                      <span className="upload-preview-label">农民名</span>
                      <span>{uploadPreview.preview.farmerName}</span>
                    </div>
                  ) : null}
                  {uploadPreview.preview.gameYear ? (
                    <div className="upload-preview-row">
                      <span className="upload-preview-label">游戏时间</span>
                      <span>第 {uploadPreview.preview.gameYear} 年 {{spring:'春',summer:'夏',fall:'秋',winter:'冬'}[uploadPreview.preview.gameSeason ?? ''] ?? uploadPreview.preview.gameSeason} 第 {uploadPreview.preview.gameDay} 天</span>
                    </div>
                  ) : null}
                  {uploadPreview.preview.farmType ? (
                    <div className="upload-preview-row">
                      <span className="upload-preview-label">地图类型</span>
                      <span>{farmTypeLabel[uploadPreview.preview.farmType] ?? uploadPreview.preview.farmType}</span>
                    </div>
                  ) : (
                    <div className="upload-preview-row">
                      <span className="upload-preview-label">地图类型</span>
                      <span className="muted">未知</span>
                    </div>
                  )}
                  {uploadPreview.preview.fileSizeBytes ? (
                    <div className="upload-preview-row">
                      <span className="upload-preview-label">文件大小</span>
                      <span>{formatBytes(uploadPreview.preview.fileSizeBytes)}</span>
                    </div>
                  ) : null}
                  {uploadPreview.preview.modifiedAt ? (
                    <div className="upload-preview-row">
                      <span className="upload-preview-label">修改时间</span>
                      <span>{new Date(uploadPreview.preview.modifiedAt).toLocaleString()}</span>
                    </div>
                  ) : null}
                  {uploadPreview.preview.parseError ? (
                    <div className="upload-preview-row">
                      <span className="upload-preview-label">解析状态</span>
                      <span className="save-row-error">{uploadPreview.preview.parseError}</span>
                    </div>
                  ) : null}
                </div>
                <p className="form-hint" style={{ marginTop: 12 }}>确认后将导入存档并启动服务器。</p>
                <div className="modal-actions" style={{ marginTop: 16 }}>
                  <button className="button" disabled={uploadBusy} onClick={handleUploadCommit} type="button">
                    {uploadBusy ? '导入中...' : '确认导入并启动'}
                  </button>
                  <button className="button button-secondary" disabled={uploadBusy} type="button"
                    onClick={handleUploadCancel}>
                    取消
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      ) : null}
    </section>
  )
}
