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
import { useSaveBackups } from './useSaveBackups'
import { useSaveRestore } from './useSaveRestore'

const seasonLabel: Record<string, string> = {
  spring: '春', summer: '夏', fall: '秋', winter: '冬',
}
const farmTypeLabel: Record<string, string> = {
  standard: '标准农场', riverland: '河畔农场', forest: '森林农场',
  hilltop: '山顶农场', wilderness: '荒野农场', fourcorners: '四角农场',
  beach: '海滩农场', meadowlands: '草地农场',
}
const farmTypeAlias: Record<string, string> = {
  标准农场: 'standard',
  河畔农场: 'riverland',
  河边农场: 'riverland',
  森林农场: 'forest',
  山顶农场: 'hilltop',
  荒野农场: 'wilderness',
  四角农场: 'fourcorners',
  海滩农场: 'beach',
  草地农场: 'meadowlands',
}

// ── SaveCard ─────────────────────────────────────────────────────────────────

// "游戏日回档"（kind === 'auto'）之外的备份类型标签；latest/daily/scheduled 是
// 已取消的旧机制留下的历史文件，不再产生新的，仅归入"历史备份"展示。
const backupKindLabel: Record<string, string> = {
  manual: '手动备份',
  predelete: '删除存档前备份',
  prerestore: '回档前保护备份',
  latest: '历史备份',
  daily: '历史备份',
  scheduled: '历史备份',
}

function saveFarmMapSrc(save: SaveInfo): string | null {
  if (!save.farmType) return null
  const farmTypeKey = farmTypeLabel[save.farmType] ? save.farmType : farmTypeAlias[save.farmType]
  if (farmTypeKey) {
    return `/assets/stardew/new-game/farms/${farmTypeKey}.png`
  }
  return null
}

function saveProgressText(save: SaveInfo): string | null {
  if (!save.gameYear) return null
  const season = seasonLabel[save.gameSeason ?? ''] ?? save.gameSeason ?? ''
  return `第 ${save.gameYear} 年 · ${season}季 · ${save.gameDay} 日`
}

function SaveCard({
  save,
  busy,
  isRunning,
  isAdmin,
  onSelect,
  onDelete,
}: {
  save: SaveInfo
  busy: boolean
  isRunning: boolean
  isAdmin: boolean
  onSelect: () => void
  onDelete: () => void
}) {
  const writeDisabled = busy || isRunning || !isAdmin
  const deleteDisabled = busy || !isAdmin
  const writeTitle = !isAdmin
    ? '仅管理员可执行此操作'
    : isRunning
      ? '服务器运行中，请先停止后操作'
      : '设为启动存档'
  const deleteTitle = !isAdmin
    ? '仅管理员可执行此操作'
    : isRunning
      ? '服务器运行中，仅允许删除非当前启动存档；删除前会再次确认'
      : undefined
  const mapSrc = saveFarmMapSrc(save)
  const progress = saveProgressText(save)

  return (
    <div className="sd-save-card">
      <div className="sd-save-card-thumb" aria-hidden="true">
        {mapSrc ? <img src={mapSrc} alt="" /> : <span className="sd-save-card-thumb-empty">?</span>}
      </div>
      <div className="sd-save-card-info">
        <div className="sd-save-card-name">
          <span>{save.farmName || save.name}</span>
        </div>
        {save.parseError ? (
          <div className="sd-save-card-error">解析失败：{save.parseError}</div>
        ) : (
          <>
            <div className="sd-save-card-line">
              {progress ?? <span className="sd-save-meta-muted">进度未知</span>}
            </div>
            <div className="sd-save-card-line sd-save-card-line-muted">
              {save.farmType ? (farmTypeLabel[save.farmType] ?? save.farmType) : '地图未知'}
              {save.fileSizeBytes ? ` · ${formatBytes(save.fileSizeBytes)}` : ''}
            </div>
          </>
        )}
      </div>
      <div className="sd-save-card-actions sd-rowactions">
        <button
          className="sd-btn-green sd-btn--sm"
          disabled={writeDisabled}
          title={writeTitle}
          onClick={onSelect}
          type="button"
        >
          选择
        </button>
        <button
          className="sd-btn-delete sd-btn--sm"
          disabled={deleteDisabled}
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

  const saves = data?.saves ?? []

  const {
    backupsLoading,
    backupMessage,
    backupPolicyDraft,
    setBackupPolicyDraft,
    backupPolicyBusy,
    backupPolicyChanged,
    autoBackups,
    otherBackups,
    showAllBackups,
    setShowAllBackups,
    deleteBackupTarget,
    openDeleteBackupDialog,
    cancelDeleteBackupDialog,
    loadBackups,
    handleManualBackup,
    handleBackupPolicySave,
    handleBackupDeleteConfirmed,
    clearBackupMessage,
  } = useSaveBackups({ isAdmin, setBusy })

  const {
    restoreBackup,
    restoreNeedsOverwrite,
    restoreError,
    restoreSaveExists,
    restoreBlocked,
    openRestoreDialog,
    cancelRestoreDialog,
    handleRestoreConfirmed,
  } = useSaveRestore({
    saves,
    isAdmin,
    isRunning,
    busy,
    setBusy,
    onJobStarted,
    onStateRefresh,
    onSavesChanged,
    loadSaves,
    loadBackups,
    clearBackupMessage,
  })

  useEffect(() => {
    void loadSaves()
    void loadBackups()
  }, [loadSaves, loadBackups])

  // refreshTrigger 变化时重新加载（如任务完成后）
  useEffect(() => {
    if (refreshTrigger && refreshTrigger > 0) {
      void loadSaves()
      void loadBackups()
    }
  }, [refreshTrigger, loadSaves, loadBackups])

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
      await loadBackups()
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

  const hasSaves = saves.length > 0
  const activeSave = data?.activeSaveName
    ? saves.find((save) => save.isActive || save.name === data.activeSaveName) ?? null
    : null
  // 存档库网格只展示非激活存档；激活存档在上方重点卡展示
  const libRestSaves = saves.filter((save) => !(save.isActive || data?.activeSaveName === save.name))
  const confirmDeleteSave = saves.find((save) => save.name === confirmDeleteName)
  const confirmDeleteIsActive = Boolean(confirmDeleteSave?.isActive || data?.activeSaveName === confirmDeleteName)
  const confirmDeleteIsLastSave = confirmDeleteName !== null && saves.length === 1
  const confirmDeleteBlocked = busy || !isAdmin || (isRunning && confirmDeleteIsActive)

  return (
    <section id="saves-section">
      {/* ── 运行中提示 / 全局操作结果 ── */}
      {isRunning && (
        <div className="sd-saves-running-hint">
          ⚠ 服务器运行中，创建 / 上传 / 切换存档已暂时禁用；当前启动存档受保护，其他存档删除前会再次确认
        </div>
      )}
      {message ? <div className="sd-saves-error">{message}</div> : null}

      {data?.activeSaveName ? (
        <>
          <div className="sd-saves-eyebrow">
            <span className="sd-saves-eyebrow-ico" aria-hidden="true">⭐</span>
            <span>当前激活存档</span>
          </div>
          <div className="sd-saves-active-row">
            <div className="sd-saves-active-card">
              <div className="sd-saves-active-art" aria-hidden="true">
                {activeSave && saveFarmMapSrc(activeSave) ? (
                  <img
                    className="sd-saves-active-art-map"
                    src={saveFarmMapSrc(activeSave) as string}
                    alt=""
                  />
                ) : null}
              </div>
              <div className="sd-saves-active-main">
                <div className="sd-saves-active-title">
                  <span>{activeSave?.farmName || data.activeSaveName}</span>
                  <span className="sd-save-active-tag">当前激活</span>
                  <span className="sd-saves-active-star" aria-hidden="true">⭐</span>
                </div>
                <div className="sd-saves-active-fields">
                  <div className="sd-saves-field">
                    <span className="sd-saves-field-ico" aria-hidden="true">🧑‍🌾</span>
                    <span className="sd-saves-field-label">农场主</span>
                    <span className="sd-saves-field-value">{activeSave?.farmerName || '未知'}</span>
                  </div>
                  <div className="sd-saves-field">
                    <span className="sd-saves-field-ico" aria-hidden="true">🕐</span>
                    <span className="sd-saves-field-label">最后游玩</span>
                    <span className="sd-saves-field-value">
                      {activeSave?.modifiedAt ? new Date(activeSave.modifiedAt).toLocaleString() : '未知'}
                    </span>
                  </div>
                  <div className="sd-saves-field">
                    <span className="sd-saves-field-ico" aria-hidden="true">📅</span>
                    <span className="sd-saves-field-label">日期</span>
                    <span className="sd-saves-field-value">
                      {activeSave ? (saveProgressText(activeSave) ?? '未知') : '未知'}
                    </span>
                  </div>
                  <div className="sd-saves-field">
                    <span className="sd-saves-field-ico" aria-hidden="true">💾</span>
                    <span className="sd-saves-field-label">文件大小</span>
                    <span className="sd-saves-field-value">
                      {activeSave?.fileSizeBytes ? formatBytes(activeSave.fileSizeBytes) : '未知'}
                    </span>
                  </div>
                  <div className="sd-saves-field">
                    <span className="sd-saves-field-ico" aria-hidden="true">🌱</span>
                    <span className="sd-saves-field-label">农场类型</span>
                    <span className="sd-saves-field-value">
                      {activeSave?.farmType ? (farmTypeLabel[activeSave.farmType] ?? activeSave.farmType) : '未知'}
                    </span>
                  </div>
                  <div className="sd-saves-field">
                    <span className="sd-saves-field-ico" aria-hidden="true">📁</span>
                    <span className="sd-saves-field-label">存档目录</span>
                    <span className="sd-saves-field-value">{data.activeSaveName}</span>
                  </div>
                </div>
              </div>
            </div>
            <div className="sd-saves-active-action-card">
              <button
                className="sd-btn-green sd-btn--lg"
                disabled={busy || isRunning || !isAdmin}
                title={
                  !isAdmin
                    ? '仅管理员可执行此操作'
                    : isRunning
                      ? '服务器运行中，请先停止后再启动'
                      : undefined
                }
                onClick={() => void handleSelectAndStart(data.activeSaveName)}
                type="button"
              >
                启动此存档
              </button>
              {activeSave ? (
                <button className="sd-btn-tan" disabled={busy} onClick={() => void handleExport(activeSave.name)} type="button">
                  导出存档
                </button>
              ) : null}
              {activeSave ? (
                <button
                  className="sd-btn-tan"
                  disabled={busy || !isAdmin}
                  title={!isAdmin ? '仅管理员可执行此操作' : undefined}
                  onClick={() => void handleManualBackup(activeSave.name)}
                  type="button"
                >
                  手动备份
                </button>
              ) : null}
            </div>
          </div>
        </>
      ) : null}

      {/* ── 存档库 ── */}
      {hasSaves ? (
        <>
          <div className="sd-saves-eyebrow">
            <span className="sd-saves-eyebrow-ico" aria-hidden="true">🍀</span>
            <span>存档库</span>
            <div className="sd-saves-eyebrow-actions sd-actionbar sd-actionbar--end">
              {isAdmin ? (
                <>
                  <button
                    className="sd-btn-green"
                    disabled={busy || isRunning}
                    title={isRunning ? '服务器运行中，请先停止后再创建存档' : undefined}
                    onClick={() => setShowNewGameModal(true)}
                    type="button"
                  >
                    新建游戏
                  </button>
                  <button
                    className="sd-btn-tan"
                    disabled={busy || isRunning}
                    title={isRunning ? '服务器运行中，请先停止后再上传存档' : '上传本地 Stardew Valley 存档 ZIP'}
                    onClick={() => setShowUploadModal(true)}
                    type="button"
                  >
                    上传存档
                  </button>
                </>
              ) : null}
              <button
                className="sd-btn-tan"
                disabled={loading}
                onClick={() => void loadSaves()}
                type="button"
              >
                {loading ? '刷新中…' : '刷新'}
              </button>
            </div>
          </div>
          <div className="sd-saves-list">
            {libRestSaves.map((save) => (
              <SaveCard
                key={save.name}
                save={save}
                busy={busy || loading}
                isRunning={isRunning}
                isAdmin={isAdmin}
                onSelect={() => void handleSelect(save.name)}
                onDelete={() => setConfirmDeleteName(save.name)}
              />
            ))}
          </div>
        </>
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
                创建并启动
              </button>
              <button
                className="sd-btn-tan"
                disabled={busy || isRunning}
                title={isRunning ? '服务器运行中，请先停止后再上传存档' : undefined}
                onClick={() => setShowUploadModal(true)}
                type="button"
              >
                上传并启动
              </button>
            </div>
          ) : null}
        </div>
      )}

      {/* ── 上传存档：天空横条入口 ── */}
      {hasSaves && isAdmin ? (
        <div className="sd-saves-upload-strip">
          <span className="sd-saves-upload-ico" aria-hidden="true">📮</span>
          <div className="sd-saves-upload-copy">
            <div className="sd-saves-upload-title">拖拽存档文件到此处或点击上传</div>
            <div className="sd-saves-upload-hint">支持 Stardew Valley 存档 ZIP 文件</div>
          </div>
          <button
            type="button"
            className="sd-btn-green"
            disabled={busy || isRunning}
            title={isRunning ? '服务器运行中，请先停止后再上传存档' : '上传本地 Stardew Valley 存档 ZIP'}
            onClick={() => setShowUploadModal(true)}
          >
            选择文件
          </button>
        </div>
      ) : null}

      {/* ── 游戏日回档 ── */}
      {isAdmin ? (
        <section className="sd-save-backups-section" aria-label="游戏日回档">
          <div className="sd-save-backup-policy-card">
            <div className="sd-save-backup-card-title">自动备份策略</div>
            <div className="sd-save-backup-policy">
              <label
                className="sd-save-backup-toggle"
                title="睡觉存档成功、存档已经落盘后，为当前游戏日创建/覆盖一个回档点"
              >
                <input
                  type="checkbox"
                  checked={backupPolicyDraft.gameSaveBackups}
                  onChange={(e) => setBackupPolicyDraft((policy) => ({ ...policy, gameSaveBackups: e.target.checked }))}
                />
                <span className="sd-save-backup-toggle-label">睡觉存档后创建回档点</span>
              </label>
              <label
                className="sd-save-backup-slider"
                title="按游戏内日期保留，与现实时间无关；同一游戏日多次保存会覆盖同一个回档点"
              >
                <span className="sd-save-backup-slider-label">
                  <span>保留最近</span>
                  <strong>{backupPolicyDraft.retainGameDays} 个游戏日</strong>
                </span>
                <input
                  type="range"
                  min={1}
                  max={14}
                  value={backupPolicyDraft.retainGameDays}
                  onChange={(e) => {
                    const value = Math.max(1, Math.min(14, Number(e.target.value) || 5))
                    setBackupPolicyDraft((policy) => ({ ...policy, retainGameDays: value }))
                  }}
                />
              </label>
              <button
                className="sd-btn-green"
                type="button"
                disabled={backupPolicyBusy || !backupPolicyChanged}
                onClick={() => void handleBackupPolicySave()}
              >
                {backupPolicyBusy ? '保存中…' : '保存设置'}
              </button>
            </div>
          </div>
          <div className="sd-save-backup-list-card">
            <div className="sd-save-backups-header">
              <div className="sd-save-backup-card-title">游戏日回档</div>
              <button
                className="sd-btn-tan"
                type="button"
                disabled={backupsLoading}
                onClick={() => void loadBackups()}
              >
                {backupsLoading ? '刷新中…' : '刷新'}
              </button>
            </div>
            {backupMessage ? <div className="sd-saves-error">{backupMessage}</div> : null}
            {backupsLoading ? (
              <div className="sd-srv-empty">读取回档列表中…</div>
            ) : autoBackups.length > 0 ? (
              <div className="sd-save-gameday-table" role="table" aria-label="游戏日回档">
                <div className="sd-save-backups-thead" role="row">
                  <span>游戏内日期</span>
                  <span>农场</span>
                  <span>农场主</span>
                  <span>创建时间</span>
                  <span>大小</span>
                  <span>操作</span>
                </div>
                {autoBackups.map((backup) => (
                  <div className="sd-save-backup-row" role="row" key={backup.name}>
                    <span className="sd-save-backup-cell">
                      {backup.gameYear
                        ? `第 ${backup.gameYear} 年 ${seasonLabel[backup.gameSeason ?? ''] ?? backup.gameSeason ?? ''}季 ${backup.gameDay} 日`
                        : <span className="sd-save-meta-muted">未知</span>}
                    </span>
                    <span className="sd-save-backup-cell">{backup.farmName || backup.saveName || '未知'}</span>
                    <span className="sd-save-backup-cell">{backup.farmerName || '未知'}</span>
                    <span className="sd-save-backup-cell">{new Date(backup.createdAt).toLocaleString()}</span>
                    <span className="sd-save-backup-cell">{formatBytes(backup.size)}</span>
                    <span className="sd-save-backup-actions sd-rowactions">
                      <button
                        className="sd-btn-green sd-btn--sm"
                        type="button"
                        disabled={restoreBlocked}
                        title={!isAdmin ? '仅管理员可执行此操作' : undefined}
                        onClick={() => openRestoreDialog(backup)}
                      >
                        回档到此日
                      </button>
                      <button
                        className="sd-btn-delete sd-btn--sm"
                        type="button"
                        disabled={busy}
                        title="彻底删除这个回档点，不影响当前存档"
                        onClick={() => openDeleteBackupDialog(backup)}
                      >
                        删除
                      </button>
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <div className="sd-srv-empty">暂无游戏日回档点。游戏内睡觉存档成功后会自动生成。</div>
            )}
          </div>
        </section>
      ) : null}

      {/* ── 其他备份：手动备份 / 删除存档前备份 / 回档前保护备份 / 历史备份 ── */}
      {isAdmin ? (
        <section className="sd-save-backups-section" aria-label="其他备份">
          <div className="sd-save-backup-list-card sd-save-backup-list-card--full">
            <div className="sd-save-backups-header">
              <div className="sd-save-backup-card-title">其他备份</div>
            </div>
            {otherBackups.length > 0 ? (
              <div className="sd-save-backups-table" role="table" aria-label="其他备份">
              <div className="sd-save-backups-thead" role="row">
                <span>备份文件</span>
                <span>所属农场</span>
                <span>创建时间</span>
                <span>大小</span>
                <span>状态</span>
                <span>操作</span>
              </div>
              {(showAllBackups ? otherBackups : otherBackups.slice(0, 5)).map((backup) => {
                const sameNameExists = saves.some((save) => save.name === backup.saveName)
                const rowTitle = [
                  backupKindLabel[backup.kind] ?? backup.kind,
                  backup.farmerName ? `农民：${backup.farmerName}` : null,
                  backup.gameYear
                    ? `第 ${backup.gameYear} 年 ${seasonLabel[backup.gameSeason ?? ''] ?? backup.gameSeason} 第 ${backup.gameDay} 天`
                    : null,
                  backup.farmType ? `地图：${farmTypeLabel[backup.farmType] ?? backup.farmType}` : null,
                ]
                  .filter(Boolean)
                  .join(' · ')
                return (
                  <div className="sd-save-backup-row" role="row" key={backup.name} title={rowTitle}>
                    <span className="sd-save-backup-file">
                      <span className="sd-save-backup-zip" aria-hidden="true">ZIP</span>
                      <span className="sd-save-backup-file-text">
                        <span className="sd-save-backup-name">{backup.name}</span>
                        <small className="sd-save-backup-kind">{backupKindLabel[backup.kind] ?? backup.kind}</small>
                      </span>
                    </span>
                    <span className="sd-save-backup-cell">
                      {backup.farmName || backup.saveName || '未知'}
                      {backup.farmerName ? `（${backup.farmerName}）` : ''}
                    </span>
                    <span className="sd-save-backup-cell">{new Date(backup.createdAt).toLocaleString()}</span>
                    <span className="sd-save-backup-cell">{formatBytes(backup.size)}</span>
                    <span className="sd-save-backup-cell">
                      {backup.parseError ? (
                        <span className="sd-badge sd-badge-red">解析失败</span>
                      ) : sameNameExists ? (
                        <span className="sd-badge sd-badge-yellow">同名冲突</span>
                      ) : (
                        <span className="sd-badge sd-badge-green">正常</span>
                      )}
                    </span>
                    <span className="sd-save-backup-actions sd-rowactions">
                      <button
                        className="sd-btn-green sd-btn--sm"
                        type="button"
                        disabled={restoreBlocked}
                        title={!isAdmin ? '仅管理员可执行此操作' : undefined}
                        onClick={() => openRestoreDialog(backup)}
                      >
                        回档到此日
                      </button>
                      <button
                        className="sd-btn-delete sd-btn--sm"
                        type="button"
                        disabled={busy}
                        title="彻底删除备份 ZIP，不影响当前存档"
                        onClick={() => openDeleteBackupDialog(backup)}
                      >
                        删除
                      </button>
                    </span>
                  </div>
                )
              })}
              {otherBackups.length > 5 ? (
                <button
                  type="button"
                  className="sd-save-backups-more"
                  onClick={() => setShowAllBackups((v) => !v)}
                >
                  {showAllBackups ? '收起备份 ︿' : `查看更多备份（${otherBackups.length - 5}） ﹀`}
                </button>
              ) : null}
            </div>
            ) : (
              <div className="sd-srv-empty">暂无其他备份。手动备份、删除存档前备份和回档前保护备份会显示在这里。</div>
            )}
          </div>
        </section>
      ) : null}

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
            {isRunning && confirmDeleteIsActive ? (
              <div className="sd-confirm-warning">
                服务器正在使用这个存档，必须先停止服务器才能删除。
              </div>
            ) : null}
            {isRunning && !confirmDeleteIsActive ? (
              <div className="sd-confirm-warning">
                服务器正在运行。此存档不是当前启动存档，可以删除；删除前会自动备份，但请确认没有玩家正在使用它。
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
                disabled={confirmDeleteBlocked}
                onClick={() => void handleDeleteConfirmed()}
              >
                {busy ? '删除中…' : '确认删除'}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {/* ── 彻底删除备份确认对话框 ── */}
      {deleteBackupTarget ? (
        <div className="sd-confirm-overlay">
          <div className="sd-confirm-dialog sd-confirm-dialog-wide">
            <h3>彻底删除备份</h3>
            <p>
              确定彻底删除备份 <strong>"{deleteBackupTarget.name}"</strong> 吗？
              这个操作只删除备份 ZIP，不会删除当前存档，但删除后无法从这个备份恢复。
            </p>
            <div className="sd-confirm-warning">
              这是不可撤销操作。请确认你已经不需要这个备份。
            </div>
            <div className="sd-confirm-actions">
              <button
                className="sd-btn-tan"
                type="button"
                disabled={busy}
                onClick={cancelDeleteBackupDialog}
              >
                取消
              </button>
              <button
                className="sd-btn-delete"
                type="button"
                disabled={busy}
                onClick={() => void handleBackupDeleteConfirmed()}
              >
                {busy ? '删除中…' : '彻底删除'}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {/* ── 回档确认对话框 ── */}
      {restoreBackup ? (
        <div className="sd-confirm-overlay">
          <div className="sd-confirm-dialog sd-confirm-dialog-wide">
            <h3>回档到此日</h3>
            <p>
              确定回档到 <strong>"{restoreBackup.name}"</strong> 吗？
              回档后会生成存档 <strong>"{restoreBackup.saveName}"</strong>。
            </p>
            {isRunning ? (
              <div className="sd-confirm-warning">
                服务器正在运行中。确认后将自动停止服务器、完成回档，并重新启动服务器；整个过程可能需要几分钟，请勿在此期间反复点击。
              </div>
            ) : null}
            {restoreNeedsOverwrite || restoreSaveExists ? (
              <div className="sd-confirm-warning">
                同名存档已存在。选择覆盖回档时，系统会先备份当前存档，再用这个回档点覆盖它。
              </div>
            ) : null}
            {restoreError ? <div className="sd-saves-error">{restoreError}</div> : null}
            <div className="sd-confirm-actions">
              <button
                className="sd-btn-tan"
                type="button"
                disabled={busy}
                onClick={cancelRestoreDialog}
              >
                取消
              </button>
              <button
                className="sd-btn-green"
                type="button"
                disabled={restoreBlocked || restoreNeedsOverwrite || restoreSaveExists}
                onClick={() => void handleRestoreConfirmed(false)}
              >
                {busy ? (isRunning ? '正在停止服务器…' : '回档中…') : isRunning ? '确认回档（自动重启服务器）' : '确认回档'}
              </button>
              {(restoreNeedsOverwrite || restoreSaveExists) ? (
                <button
                  className="sd-btn-delete"
                  type="button"
                  disabled={restoreBlocked}
                  onClick={() => void handleRestoreConfirmed(true)}
                >
                  {busy ? (isRunning ? '正在停止服务器…' : '回档中…') : isRunning ? '覆盖回档（自动重启服务器）' : '覆盖回档'}
                </button>
              ) : null}
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
                    className="sd-btn-tan"
                    disabled={uploadBusy}
                    type="button"
                    onClick={handleUploadCancel}
                  >
                    取消
                  </button>
                  <button
                    className="sd-btn-green"
                    disabled={uploadBusy || !uploadFile}
                    onClick={() => void handleUploadPreview()}
                    type="button"
                  >
                    {uploadBusy ? '解析中…' : '预览存档'}
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
                    className="sd-btn-tan"
                    disabled={uploadBusy}
                    type="button"
                    onClick={handleUploadCancel}
                  >
                    取消
                  </button>
                  <button
                    className="sd-btn-green"
                    disabled={uploadBusy}
                    onClick={() => void handleUploadCommit()}
                    type="button"
                  >
                    {uploadBusy ? '导入中…' : '导入并启动'}
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
