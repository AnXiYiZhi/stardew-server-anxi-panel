import { useState } from 'react'
import type { NewGameConfig, SaveInfo, UploadPreviewResult } from '../../types'
import { ApiError } from '../../api'
import { defaultInstanceId, createNewGame, uploadSavePreview, uploadSaveCommitAndStart, startInstance, stopInstance, restartInstance, getInviteCode } from '../../api'
import { errorMessage, stateLabel, formatBytes } from '../../core/helpers'
import { Field } from '../../core/Field'
import { NewGameCreator } from './NewGameCreator'

function SaveCard({ save }: { save: SaveInfo }) {
  return (
    <div className="save-card">
      <div className="save-card-name">{save.name}</div>
      {save.parseError ? (
        <div className="save-card-hint">解析失败：{save.parseError}</div>
      ) : (
        <div className="save-card-meta">
          {save.farmerName ? <span>农民：{save.farmerName}</span> : <span className="muted">农民名：未读取到</span>}
          {save.farmName ? <span>农场：{save.farmName}</span> : <span className="muted">农场名：未读取到</span>}
          {save.gameYear ? <span>第 {save.gameYear} 年 {save.gameSeason} 第 {save.gameDay} 天</span> : null}
          {save.farmType ? <span>地图：{save.farmType}</span> : null}
          {save.fileSizeBytes ? <span>大小：{formatBytes(save.fileSizeBytes)}</span> : null}
        </div>
      )}
    </div>
  )
}

export function LifecycleSection({
  state,
  isAdmin,
  onJobStarted,
  onStateRefresh,
}: {
  state: string
  isAdmin: boolean
  onJobStarted: (jobId: string) => void
  onStateRefresh: () => void
}) {
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const [inviteCode, setInviteCode] = useState('')
  const [showNewGameModal, setShowNewGameModal] = useState(false)
  const [newGameError, setNewGameError] = useState('')
  const [showUploadModal, setShowUploadModal] = useState(false)
  const [uploadFile, setUploadFile] = useState<File | null>(null)
  const [uploadPreview, setUploadPreview] = useState<UploadPreviewResult | null>(null)
  const [uploadBusy, setUploadBusy] = useState(false)
  const [uploadMessage, setUploadMessage] = useState('')

  const canStart = state === 'game_installed' || state === 'save_required' || state === 'ready_to_start' || state === 'stopped'
  const isRunning = state === 'running'
  const isStarting = state === 'starting'

  async function handleStart() {
    setBusy(true)
    setMessage('')
    try {
      const res = await startInstance()
      onJobStarted(res.jobId)
      onStateRefresh()
    } catch (error) {
      if (error instanceof ApiError && error.code === 'save_required') {
        setMessage('没有可用存档。请使用下方"创建存档并启动"或"上传存档并启动"。')
        document.getElementById('save-start-panel')?.scrollIntoView({ behavior: 'smooth', block: 'center' })
        return
      }
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleStop() {
    if (!window.confirm('确定停止服务器吗？')) return
    setBusy(true)
    setMessage('')
    try {
      await stopInstance()
      onStateRefresh()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleRestart() {
    if (!window.confirm('确定重启服务器吗？')) return
    setBusy(true)
    setMessage('')
    try {
      await restartInstance()
      onStateRefresh()
    } catch (error) {
      setMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleGetInviteCode() {
    setBusy(true)
    setMessage('')
    try {
      const res = await getInviteCode()
      setInviteCode(res.inviteCode)
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

  async function handleUploadCancel() {
    if (uploadPreview) {
      try {
        await uploadSaveCommitAndStart(uploadPreview.token, true)
      } catch { /* best effort */ }
    }
    setShowUploadModal(false)
    setUploadPreview(null)
    setUploadFile(null)
    setUploadMessage('')
  }

  return (
    <section className="lifecycle-section">
      <div className="section-heading">
        <div>
          <h2>服务器生命周期</h2>
          <p>启动、停止、重启 Stardew Junimo 服务器。</p>
        </div>
      </div>

      {message ? <div className="error-banner">{message}</div> : null}

      {/* 状态标签 */}
      <div className="lifecycle-state">
        <span className="lifecycle-state-label">当前状态：</span>
        <span className={`lifecycle-state-badge lifecycle-state-${state}`}>{stateLabel(state)}</span>
      </div>

      {/* 邀请码 */}
      {inviteCode ? (
        <div className="invite-code-display">
          <span>邀请码：</span>
          <strong className="invite-code">{inviteCode}</strong>
        </div>
      ) : null}

      {/* 独立存档启动面板：始终提供显式创建/上传路径。 */}
      {isAdmin && !isRunning && !isStarting ? (
        <div id="save-start-panel" className="preflight-result">
          <p className="preflight-heading">存档启动</p>
          <p className="form-hint">创建或上传存档后会自动启动服务器。</p>
          <div className="lifecycle-actions">
            <button className="button" disabled={busy} onClick={() => setShowNewGameModal(true)} type="button">
              创建存档并启动
            </button>
            <button className="button button-secondary" disabled={busy} onClick={() => setShowUploadModal(true)} type="button">
              上传存档并启动
            </button>
          </div>
        </div>
      ) : null}

      {/* 启动服务器独立于创建/上传：默认由 Junimo 继续加载上次使用的可用存档。 */}
      {isAdmin && (canStart || isRunning || isStarting) ? (
        <div className="lifecycle-actions">
          {canStart ? (
            <button className="button" disabled={busy} onClick={handleStart} type="button">
              {busy ? '启动中...' : '启动服务器（使用上次存档）'}
            </button>
          ) : null}
          {isRunning ? (
            <>
              <button className="button button-secondary" disabled={busy} onClick={handleRestart} type="button">
                重启
              </button>
              <button className="button button-danger" disabled={busy} onClick={handleStop} type="button">
                停止
              </button>
              <button className="button button-secondary" disabled={busy} onClick={handleGetInviteCode} type="button">
                获取邀请码
              </button>
            </>
          ) : null}
          {isStarting ? (
            <p className="summary">服务器正在启动，请稍候...</p>
          ) : null}
        </div>
      ) : null}

      {/* 新建游戏 Modal */}
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

      {/* 上传存档 Modal */}
      {showUploadModal ? (
        <div className="modal-overlay">
          <div className="modal-card">
            <h3>上传存档</h3>
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
                <p className="preflight-heading">存档预览：</p>
                <SaveCard save={uploadPreview.preview} />
                <p className="form-hint">确认后将导入并启动服务器。</p>
                <div className="modal-actions">
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
