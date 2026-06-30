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
import type { StardewSaveActionRequest } from './stardew-routes'

const seasonLabel: Record<string, string> = {
  spring: '春', summer: '夏', fall: '秋', winter: '冬',
}
const farmTypeLabel: Record<string, string> = {
  standard: '标准农场', riverland: '河畔农场', forest: '森林农场',
  hilltop: '山顶农场', wilderness: '荒野农场', fourcorners: '四角农场',
  beach: '海滩农场', meadowlands: '草地农场',
}

// ── SaveCard ─────────────────────────────────────────────────────────────────

function SaveCard({
  save,
  isActive,
  busy,
  isRunning,
  isAdmin,
  onSelect,
  onSelectAndStart,
  onDelete,
  onExport,
}: {
  save: SaveInfo
  isActive: boolean
  busy: boolean
  isRunning: boolean
  isAdmin: boolean
  onSelect: () => void
  onSelectAndStart: () => void
  onDelete: () => void
  onExport: () => void
}) {
  const writeDisabled = busy || isRunning || !isAdmin
  const writeTitle = !isAdmin
    ? '仅管理员可执行此操作'
    : isRunning
      ? '服务器运行中，请先停止后操作'
      : undefined
  const deleteTitle = writeTitle ?? (isActive ? '这是当前启动存档，删除后需要重新选择启动存档' : undefined)

  return (
    <div className={`sd-save-card${isActive ? ' active' : ''}`}>
      <div className="sd-save-card-info">
        <div className="sd-save-card-name">
          {isActive ? <span className="sd-save-active-tag">当前</span> : null}
          <span>{save.name}</span>
        </div>
        {save.parseError ? (
          <div className="sd-save-card-error">解析失败：{save.parseError}</div>
        ) : (
          <div className="sd-save-meta">
            {save.farmName
              ? <span>农场：{save.farmName}</span>
              : <span className="sd-save-meta-muted">农场名未知</span>}
            {save.farmerName
              ? <span>农民：{save.farmerName}</span>
              : <span className="sd-save-meta-muted">农民名未知</span>}
            {save.gameYear ? (
              <span>
                第 {save.gameYear} 年{' '}
                {seasonLabel[save.gameSeason ?? ''] ?? save.gameSeason}{' '}
                第 {save.gameDay} 天
              </span>
            ) : null}
            {save.farmType
              ? <span>地图：{farmTypeLabel[save.farmType] ?? save.farmType}</span>
              : <span className="sd-save-meta-muted">地图未知</span>}
            {save.fileSizeBytes ? <span>大小：{formatBytes(save.fileSizeBytes)}</span> : null}
            {save.modifiedAt
              ? <span>修改：{new Date(save.modifiedAt).toLocaleString()}</span>
              : null}
          </div>
        )}
      </div>
      <div className="sd-save-card-actions">
        {/* 写操作按钮：管理员可见，非管理员禁用（始终可见，避免用户困惑） */}
        {!isActive ? (
          <button
            className="sd-btn-tan"
            disabled={writeDisabled}
            title={writeTitle}
            onClick={onSelect}
            type="button"
          >
            设为启动存档
          </button>
        ) : null}
        <button
          className="sd-btn-green"
          disabled={writeDisabled}
          title={writeTitle}
          onClick={onSelectAndStart}
          type="button"
        >
          {isActive ? '使用此存档启动' : '选择并启动'}
        </button>
        {/* 导出：所有登录用户均可操作 */}
        <button className="sd-btn-tan" disabled={busy} onClick={onExport} type="button">
          导出
        </button>
        <button
          className="sd-btn-delete"
          disabled={writeDisabled}
          title={deleteTitle}
          onClick={onDelete}
          type="button"
        >
          删除
        </button>
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
  onSavesChanged,
  refreshTrigger,
  saveActionRequest,
}: {
  state: string
  isAdmin: boolean
  onJobStarted: (jobId: string) => void
  onStateRefresh: () => void
  onSavesChanged?: () => void
  refreshTrigger?: number
  saveActionRequest?: StardewSaveActionRequest | null
}) {
  const [data, setData] = useState<SavesListResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)
  const isRunning = state === 'running' || state === 'starting'

  // 删除确认（内联对话框，替代 window.confirm）
  const [confirmDeleteName, setConfirmDeleteName] = useState<string | null>(null)

  // 新建游戏弹窗
  const [showNewGameModal, setShowNewGameModal] = useState(false)
  const [newGameError, setNewGameError] = useState('')

  // 上传存档弹窗
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

  // refreshTrigger 变化时重新加载（如任务完成后）
  useEffect(() => {
    if (refreshTrigger && refreshTrigger > 0) {
      void loadSaves()
    }
  }, [refreshTrigger, loadSaves])

  // save_required 状态时自动滚动到本区域
  useEffect(() => {
    if (state === 'save_required') {
      document.getElementById('saves-section')?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    }
  }, [state])

  useEffect(() => {
    if (!saveActionRequest || !isAdmin || isRunning) return
    document.getElementById('saves-section')?.scrollIntoView({ behavior: 'smooth', block: 'center' })
    setMessage('')
    if (saveActionRequest.action === 'new') {
      setShowUploadModal(false)
      setShowNewGameModal(true)
      return
    }
    setShowNewGameModal(false)
    setShowUploadModal(true)
  }, [saveActionRequest?.nonce, saveActionRequest?.action, isAdmin, isRunning])

  async function handleSelect(name: string) {
    setBusy(true)
    setMessage('')
    try {
      await selectSave(name)
      await loadSaves()
      onStateRefresh()
      onSavesChanged?.()
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
      await loadSaves()
      onJobStarted(res.jobId)
      onStateRefresh()
      onSavesChanged?.()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleDeleteConfirmed() {
    const name = confirmDeleteName
    setConfirmDeleteName(null)
    if (!name) return
    setBusy(true)
    setMessage('')
    try {
      await deleteSave(name)
      await loadSaves()
      onStateRefresh()
      onSavesChanged?.()
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
      await loadSaves()
      onJobStarted(res.jobId)
      onStateRefresh()
      onSavesChanged?.()
    } catch (error) {
      setNewGameError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

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
      await loadSaves()
      onJobStarted(res.jobId)
      onStateRefresh()
      onSavesChanged?.()
    } catch (error) {
      setUploadMessage(errorMessage(error))
    } finally {
      setUploadBusy(false)
    }
  }

  function handleUploadCancel() {
    if (uploadPreview) {
      // 尽力清理挂起的 token
      void uploadSaveCommitAndStart(uploadPreview.token, true).catch(() => {})
    }
    setShowUploadModal(false)
    setUploadPreview(null)
    setUploadFile(null)
    setUploadMessage('')
  }

  const saves = data?.saves ?? []
  const hasSaves = saves.length > 0
  const confirmDeleteSave = saves.find((save) => save.name === confirmDeleteName)
  const confirmDeleteIsActive = Boolean(confirmDeleteSave?.isActive || data?.activeSaveName === confirmDeleteName)
  const confirmDeleteIsLastSave = confirmDeleteName !== null && saves.length === 1

  return (
    <section id="saves-section">
      {/* ── 页头 ── */}
      <div className="sd-saves-header">
        <div className="sd-saves-header-left">
          <div className="sd-srv-section-title" style={{ borderBottom: 'none', paddingBottom: 0, marginBottom: 0 }}>
            存档列表
          </div>
          {isRunning && (
            <div className="sd-saves-running-hint">
              ⚠ 服务器运行中，创建 / 上传 / 删除 / 切换存档已暂时禁用
            </div>
          )}
        </div>
        <div className="sd-saves-header-actions">
          <button
            className="sd-btn-tan"
            disabled={loading}
            onClick={() => void loadSaves()}
            type="button"
          >
            {loading ? '刷新中…' : '刷新列表'}
          </button>
          {isAdmin && (
            <>
              <button
                className="sd-btn-green"
                disabled={busy || isRunning}
                title={isRunning ? '服务器运行中，请先停止后再创建存档' : undefined}
                onClick={() => setShowNewGameModal(true)}
                type="button"
              >
                创建存档
              </button>
              <button
                className="sd-btn-tan"
                disabled={busy || isRunning}
                title={isRunning ? '服务器运行中，请先停止后再上传存档' : undefined}
                onClick={() => setShowUploadModal(true)}
                type="button"
              >
                上传存档
              </button>
            </>
          )}
        </div>
      </div>

      {/* ── 全局操作结果 ── */}
      {message ? <div className="sd-saves-error">{message}</div> : null}

      {/* ── 活跃存档提示 ── */}
      {data?.activeSaveName ? (
        <div className="sd-saves-active-hint">
          <span>下次启动将加载：</span>
          <strong>{data.activeSaveName}</strong>
        </div>
      ) : null}

      {/* ── 存档列表 ── */}
      {hasSaves ? (
        <div className="sd-saves-list">
          {saves.map((save) => (
            <SaveCard
              key={save.name}
              save={save}
              isActive={save.isActive ?? false}
              busy={busy || loading}
              isRunning={isRunning}
              isAdmin={isAdmin}
              onSelect={() => void handleSelect(save.name)}
              onSelectAndStart={() => void handleSelectAndStart(save.name)}
              onDelete={() => setConfirmDeleteName(save.name)}
              onExport={() => void handleExport(save.name)}
            />
          ))}
        </div>
      ) : loading ? (
        <div className="sd-srv-empty">加载存档列表中…</div>
      ) : (
        <div className="sd-saves-empty">
          <div className="sd-saves-empty-title">当前没有存档</div>
          <p className="sd-saves-empty-hint">
            你可以创建一个新存档，或从本地上传已有的 Stardew Valley 存档（ZIP）。
            <br />
            Junimo 首次生成世界可能需要 5–15 分钟。
          </p>
          {isAdmin ? (
            <div className="sd-saves-empty-actions">
              <button
                className="sd-btn-green"
                disabled={busy || isRunning}
                title={isRunning ? '服务器运行中，请先停止后再创建存档' : undefined}
                onClick={() => setShowNewGameModal(true)}
                type="button"
              >
                创建存档并启动
              </button>
              <button
                className="sd-btn-tan"
                disabled={busy || isRunning}
                title={isRunning ? '服务器运行中，请先停止后再上传存档' : undefined}
                onClick={() => setShowUploadModal(true)}
                type="button"
              >
                上传存档并启动
              </button>
            </div>
          ) : null}
        </div>
      )}

      {/* ── 删除确认对话框 ── */}
      {confirmDeleteName ? (
        <div className="sd-confirm-overlay">
          <div className="sd-confirm-dialog">
            <h3>删除存档</h3>
            <p>
              确定删除存档 <strong>"{confirmDeleteName}"</strong> 吗？
              删除前会自动备份，但操作本身不可直接撤销。
            </p>
            {confirmDeleteIsActive ? (
              <div className="sd-confirm-warning">
                这是当前启动存档。删除后服务器将没有已选择的启动存档，下一次启动前需要重新选择、创建或上传存档。
              </div>
            ) : null}
            {confirmDeleteIsLastSave ? (
              <div className="sd-confirm-warning">
                这是当前最后一个存档。删除后存档列表会变空，服务器启动前会要求先准备一个新存档。
              </div>
            ) : null}
            <div className="sd-confirm-actions">
              <button
                className="sd-btn-tan"
                type="button"
                onClick={() => setConfirmDeleteName(null)}
              >
                取消
              </button>
              <button
                className="sd-btn-delete"
                type="button"
                disabled={busy || isRunning || !isAdmin}
                onClick={() => void handleDeleteConfirmed()}
              >
                {busy ? '删除中…' : '确认删除'}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {/* ── 新建游戏弹窗 ── */}
      {showNewGameModal ? (
        <div className="sd-saves-modal-overlay">
          <div className="sd-saves-modal-card sd-saves-modal-card-wide">
            <div className="sd-saves-modal-header">
              <h3 className="sd-saves-modal-title">新建游戏</h3>
              <button
                className="sd-btn-tan"
                type="button"
                onClick={() => { setShowNewGameModal(false); setNewGameError('') }}
              >
                关闭
              </button>
            </div>
            <NewGameCreator
              instanceId={defaultInstanceId}
              onSubmit={(cfg) => void handleNewGameSubmit(cfg)}
              submitting={busy}
              submitError={newGameError}
            />
          </div>
        </div>
      ) : null}

      {/* ── 上传存档弹窗 ── */}
      {showUploadModal ? (
        <div className="sd-saves-modal-overlay">
          <div className="sd-saves-modal-card">
            <div className="sd-saves-modal-header">
              <h3 className="sd-saves-modal-title">上传存档</h3>
              <button
                className="sd-btn-tan"
                type="button"
                onClick={handleUploadCancel}
                disabled={uploadBusy}
              >
                关闭
              </button>
            </div>

            {uploadMessage ? <div className="sd-saves-error">{uploadMessage}</div> : null}

            {!uploadPreview ? (
              <div className="sd-saves-upload-form">
                <p className="sd-saves-hint">
                  上传一个包含 Stardew Valley 存档的 ZIP 文件（最大 100 MB）。
                </p>
                <Field label="选择 ZIP 文件">
                  <input
                    type="file"
                    accept=".zip"
                    onChange={(e) => setUploadFile(e.target.files?.[0] ?? null)}
                  />
                </Field>
                <div className="sd-saves-modal-actions">
                  <button
                    className="sd-btn-green"
                    disabled={uploadBusy || !uploadFile}
                    onClick={() => void handleUploadPreview()}
                    type="button"
                  >
                    {uploadBusy ? '解析中…' : '预览存档'}
                  </button>
                  <button
                    className="sd-btn-tan"
                    disabled={uploadBusy}
                    type="button"
                    onClick={handleUploadCancel}
                  >
                    取消
                  </button>
                </div>
              </div>
            ) : (
              <div>
                <div className="sd-srv-section-title" style={{ marginBottom: 8 }}>存档预览</div>
                <div className="sd-saves-preview-table">
                  <div className="sd-saves-preview-row">
                    <span className="sd-saves-preview-label">存档目录名</span>
                    <strong>{uploadPreview.saveName}</strong>
                  </div>
                  {uploadPreview.preview.farmName ? (
                    <div className="sd-saves-preview-row">
                      <span className="sd-saves-preview-label">农场名</span>
                      <span>{uploadPreview.preview.farmName}</span>
                    </div>
                  ) : null}
                  {uploadPreview.preview.farmerName ? (
                    <div className="sd-saves-preview-row">
                      <span className="sd-saves-preview-label">农民名</span>
                      <span>{uploadPreview.preview.farmerName}</span>
                    </div>
                  ) : null}
                  {uploadPreview.preview.gameYear ? (
                    <div className="sd-saves-preview-row">
                      <span className="sd-saves-preview-label">游戏时间</span>
                      <span>
                        第 {uploadPreview.preview.gameYear} 年{' '}
                        {{ spring: '春', summer: '夏', fall: '秋', winter: '冬' }[uploadPreview.preview.gameSeason ?? ''] ?? uploadPreview.preview.gameSeason}{' '}
                        第 {uploadPreview.preview.gameDay} 天
                      </span>
                    </div>
                  ) : null}
                  <div className="sd-saves-preview-row">
                    <span className="sd-saves-preview-label">地图类型</span>
                    <span>
                      {uploadPreview.preview.farmType
                        ? (farmTypeLabel[uploadPreview.preview.farmType] ?? uploadPreview.preview.farmType)
                        : <span className="sd-save-meta-muted">未知</span>}
                    </span>
                  </div>
                  {uploadPreview.preview.fileSizeBytes ? (
                    <div className="sd-saves-preview-row">
                      <span className="sd-saves-preview-label">文件大小</span>
                      <span>{formatBytes(uploadPreview.preview.fileSizeBytes)}</span>
                    </div>
                  ) : null}
                  {uploadPreview.preview.modifiedAt ? (
                    <div className="sd-saves-preview-row">
                      <span className="sd-saves-preview-label">修改时间</span>
                      <span>{new Date(uploadPreview.preview.modifiedAt).toLocaleString()}</span>
                    </div>
                  ) : null}
                  {uploadPreview.preview.parseError ? (
                    <div className="sd-saves-preview-row">
                      <span className="sd-saves-preview-label">解析状态</span>
                      <span className="sd-save-card-error">{uploadPreview.preview.parseError}</span>
                    </div>
                  ) : null}
                </div>
                <p className="sd-saves-hint" style={{ marginTop: 10 }}>
                  确认后将导入存档并启动服务器。
                </p>
                <div className="sd-saves-modal-actions">
                  <button
                    className="sd-btn-green"
                    disabled={uploadBusy}
                    onClick={() => void handleUploadCommit()}
                    type="button"
                  >
                    {uploadBusy ? '导入中…' : '确认导入并启动'}
                  </button>
                  <button
                    className="sd-btn-tan"
                    disabled={uploadBusy}
                    type="button"
                    onClick={handleUploadCancel}
                  >
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
