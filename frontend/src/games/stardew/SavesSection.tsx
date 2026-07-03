import { useCallback, useEffect, useState } from 'react'
import type { BackupInfo, BackupPolicy, NewGameConfig, SaveInfo, SavesListResult, UploadPreviewResult } from '../../types'
import {
  ApiError,
  defaultInstanceId,
  getSaves,
  getSaveBackups,
  createSaveBackup,
  updateSaveBackupPolicy,
  selectSave,
  selectSaveAndStart,
  deleteSave,
  deleteSaveBackup,
  exportSave,
  restoreSaveBackup,
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

const defaultBackupPolicy: BackupPolicy = {
  gameSaveBackups: true,
  dailySnapshots: true,
  dailyRetentionDays: 3,
  scheduledBackups: false,
  scheduledHour: 4,
}

function normalizeBackupPolicy(policy: BackupPolicy): BackupPolicy {
  const rawHour = (policy as Partial<BackupPolicy>).scheduledHour
  const scheduledHour = Math.max(
    0,
    Math.min(23, typeof rawHour === 'number' && Number.isFinite(rawHour) ? rawHour : defaultBackupPolicy.scheduledHour),
  )
  const { scheduledIntervalHours: _legacyScheduledIntervalHours, ...normalized } = {
    ...defaultBackupPolicy,
    ...policy,
    scheduledHour,
  }
  return normalized
}

const backupKindLabel: Record<string, string> = {
  manual: '手动备份',
  latest: '最新备份',
  daily: '每日快照',
  scheduled: '定时备份',
}

function SaveCard({
  save,
  isActive,
  busy,
  isRunning,
  isAdmin,
  onSelect,
  onSelectAndStart,
  onBackup,
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
  onBackup: () => void
  onDelete: () => void
  onExport: () => void
}) {
  const writeDisabled = busy || isRunning || !isAdmin
  const deleteDisabled = busy || !isAdmin || (isRunning && isActive)
  const writeTitle = !isAdmin
    ? '仅管理员可执行此操作'
    : isRunning
      ? '服务器运行中，请先停止后操作'
      : undefined
  const deleteTitle = !isAdmin
    ? '仅管理员可执行此操作'
    : isRunning && isActive
      ? '当前启动存档正在被服务器使用，请先停止服务器再删除'
      : isRunning
        ? '服务器运行中，仅允许删除非当前启动存档；删除前会再次确认'
        : isActive
          ? '这是当前启动存档，删除后需要重新选择启动存档'
          : undefined

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
          className="sd-btn-tan"
          disabled={busy || !isAdmin}
          title={!isAdmin ? '仅管理员可执行此操作' : undefined}
          onClick={onBackup}
          type="button"
        >
          手动备份
        </button>
        <button
          className="sd-btn-delete"
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
  const [backups, setBackups] = useState<BackupInfo[]>([])
  const [backupsLoading, setBackupsLoading] = useState(false)
  const [backupMessage, setBackupMessage] = useState('')
  const [backupPolicy, setBackupPolicy] = useState<BackupPolicy>(defaultBackupPolicy)
  const [backupPolicyDraft, setBackupPolicyDraft] = useState<BackupPolicy>(defaultBackupPolicy)
  const [backupPolicyBusy, setBackupPolicyBusy] = useState(false)
  const [restoreBackup, setRestoreBackup] = useState<BackupInfo | null>(null)
  const [restoreNeedsOverwrite, setRestoreNeedsOverwrite] = useState(false)
  const [restoreError, setRestoreError] = useState('')
  const [deleteBackupTarget, setDeleteBackupTarget] = useState<BackupInfo | null>(null)
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

  const loadBackups = useCallback(async () => {
    if (!isAdmin) {
      setBackups([])
      return
    }
    setBackupsLoading(true)
    setBackupMessage('')
    try {
      const result = await getSaveBackups()
      setBackups([...result.backups].sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt)))
      if (result.policy) {
        const normalizedPolicy = normalizeBackupPolicy(result.policy)
        setBackupPolicy(normalizedPolicy)
        setBackupPolicyDraft(normalizedPolicy)
      }
    } catch (error) {
      setBackupMessage(errorMessage(error))
    } finally {
      setBackupsLoading(false)
    }
  }, [isAdmin])

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

  async function handleManualBackup(name: string) {
    setBusy(true)
    setBackupMessage('')
    try {
      await createSaveBackup(name)
      await loadBackups()
      setBackupMessage('手动备份已创建。')
    } catch (error) {
      setBackupMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleBackupPolicySave() {
    setBackupPolicyBusy(true)
    setBackupMessage('')
    try {
      const result = await updateSaveBackupPolicy(normalizeBackupPolicy(backupPolicyDraft))
      const normalizedPolicy = normalizeBackupPolicy(result.policy)
      setBackupPolicy(normalizedPolicy)
      setBackupPolicyDraft(normalizedPolicy)
      await loadBackups()
      setBackupMessage('备份设置已保存。')
    } catch (error) {
      setBackupMessage(errorMessage(error))
    } finally {
      setBackupPolicyBusy(false)
    }
  }

  function openRestoreDialog(backup: BackupInfo) {
    setRestoreBackup(backup)
    setRestoreNeedsOverwrite(saves.some((save) => save.name === backup.saveName))
    setRestoreError('')
  }

  async function handleRestoreConfirmed(overwrite: boolean) {
    if (!restoreBackup) return
    setBusy(true)
    setRestoreError('')
    setBackupMessage('')
    try {
      await restoreSaveBackup(restoreBackup.name, overwrite)
      setRestoreBackup(null)
      setRestoreNeedsOverwrite(false)
      await loadSaves()
      await loadBackups()
      onStateRefresh()
      onSavesChanged?.()
    } catch (error) {
      if (error instanceof ApiError && error.code === 'save_exists') {
        setRestoreNeedsOverwrite(true)
        setRestoreError('同名存档已存在。确认覆盖后，系统会先备份当前存档再恢复此备份。')
      } else {
        setRestoreError(errorMessage(error))
      }
    } finally {
      setBusy(false)
    }
  }

  async function handleBackupDeleteConfirmed() {
    if (!deleteBackupTarget) return
    setBusy(true)
    setBackupMessage('')
    try {
      await deleteSaveBackup(deleteBackupTarget.name)
      setDeleteBackupTarget(null)
      await loadBackups()
    } catch (error) {
      setBackupMessage(errorMessage(error))
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
  const activeSave = data?.activeSaveName
    ? saves.find((save) => save.isActive || save.name === data.activeSaveName) ?? null
    : null
  const confirmDeleteSave = saves.find((save) => save.name === confirmDeleteName)
  const confirmDeleteIsActive = Boolean(confirmDeleteSave?.isActive || data?.activeSaveName === confirmDeleteName)
  const confirmDeleteIsLastSave = confirmDeleteName !== null && saves.length === 1
  const confirmDeleteBlocked = busy || !isAdmin || (isRunning && confirmDeleteIsActive)
  const restoreSaveExists = restoreBackup ? saves.some((save) => save.name === restoreBackup.saveName) : false
  const restoreBlocked = busy || isRunning || !isAdmin
  const backupPolicyChanged = JSON.stringify(backupPolicyDraft) !== JSON.stringify(backupPolicy)

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
              ⚠ 服务器运行中，创建 / 上传 / 切换存档已暂时禁用；当前启动存档受保护，其他存档删除前会再次确认
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

      {data?.activeSaveName ? (
        <div className="sd-saves-active-card">
          <div className="sd-saves-active-art" aria-hidden="true" />
          <div className="sd-saves-active-main">
            <div className="sd-saves-active-eyebrow">当前激活存档</div>
            <div className="sd-saves-active-title">
              {activeSave?.farmName || data.activeSaveName}
              <span className="sd-save-active-tag">当前激活</span>
            </div>
            <div className="sd-save-meta">
              <span>目录：{data.activeSaveName}</span>
              {activeSave?.farmerName ? <span>农民：{activeSave.farmerName}</span> : null}
              {activeSave?.gameYear ? (
                <span>
                  第 {activeSave.gameYear} 年{' '}
                  {seasonLabel[activeSave.gameSeason ?? ''] ?? activeSave.gameSeason}{' '}
                  第 {activeSave.gameDay} 天
                </span>
              ) : null}
              {activeSave?.farmType ? <span>地图：{farmTypeLabel[activeSave.farmType] ?? activeSave.farmType}</span> : null}
              {activeSave?.fileSizeBytes ? <span>大小：{formatBytes(activeSave.fileSizeBytes)}</span> : null}
              {activeSave?.modifiedAt ? <span>修改：{new Date(activeSave.modifiedAt).toLocaleString()}</span> : null}
            </div>
          </div>
          <div className="sd-saves-active-actions">
            <button
              className="sd-btn-green"
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
              使用此存档启动
            </button>
            {activeSave ? (
              <button className="sd-btn-tan" disabled={busy} onClick={() => void handleExport(activeSave.name)} type="button">
                导出
              </button>
            ) : null}
          </div>
        </div>
      ) : null}

      {/* ── 存档列表 ── */}
      {hasSaves ? (
        <div className="sd-saves-list">
          {saves.map((save) => (
            <SaveCard
              key={save.name}
              save={save}
              isActive={Boolean(save.isActive || data?.activeSaveName === save.name)}
              busy={busy || loading}
              isRunning={isRunning}
              isAdmin={isAdmin}
              onSelect={() => void handleSelect(save.name)}
              onSelectAndStart={() => void handleSelectAndStart(save.name)}
              onBackup={() => void handleManualBackup(save.name)}
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

      {/* ── 备份与恢复 ── */}
      {isAdmin ? (
        <section className="sd-save-backups-section" aria-label="备份与恢复">
          <div className="sd-save-backups-header">
            <div>
              <div className="sd-srv-section-title" style={{ borderBottom: 'none', paddingBottom: 0, marginBottom: 0 }}>
                备份与恢复
              </div>
              {isRunning ? (
                <div className="sd-saves-running-hint">
                  ⚠ 服务器运行中，备份可以查看，恢复需要先停止服务器
                </div>
              ) : null}
            </div>
            <button
              className="sd-btn-tan"
              type="button"
              disabled={backupsLoading}
              onClick={() => void loadBackups()}
            >
              {backupsLoading ? '刷新中…' : '刷新备份'}
            </button>
          </div>
          <div className="sd-save-backup-policy">
            <div className="sd-save-backup-policy-head">
              <strong>自动备份规则</strong>
              <span>手动备份会单独保留；下面这些只控制自动备份。</span>
            </div>
            <label className="sd-save-backup-toggle sd-save-backup-option">
              <input
                type="checkbox"
                checked={backupPolicyDraft.gameSaveBackups}
                onChange={(e) => setBackupPolicyDraft((policy) => ({ ...policy, gameSaveBackups: e.target.checked }))}
              />
              <span>
                <strong>游戏保存后更新“最新备份”</strong>
                <small>玩家睡觉完成保存后，覆盖同一份最新备份，方便回到最近一次保存。</small>
              </span>
            </label>
            <label className="sd-save-backup-toggle sd-save-backup-option">
              <input
                type="checkbox"
                checked={backupPolicyDraft.scheduledBackups}
                onChange={(e) => setBackupPolicyDraft((policy) => ({ ...policy, scheduledBackups: e.target.checked }))}
              />
              <span>
                <strong>每天固定时间更新“定时备份”</strong>
                <small>到你选的时间后覆盖同一份定时备份，适合每天留一份固定保险。</small>
              </span>
            </label>
            <label className="sd-save-backup-field">
              <span>每天</span>
              <select
                value={backupPolicyDraft.scheduledHour}
                onChange={(e) => {
                  const value = Math.max(0, Math.min(23, Number(e.target.value) || 0))
                  setBackupPolicyDraft((policy) => ({ ...policy, scheduledHour: value }))
                }}
              >
                {Array.from({ length: 24 }, (_, hour) => (
                  <option key={hour} value={hour}>
                    {String(hour).padStart(2, '0')}:00
                  </option>
                ))}
              </select>
              <span>执行一次</span>
            </label>
            <label className="sd-save-backup-slider">
              <span>
                <strong>每日快照最多保留 {backupPolicyDraft.dailyRetentionDays} 天</strong>
                <small>每天只留一份；同一天再次备份会覆盖，超过天数会自动删旧快照。</small>
              </span>
              <input
                type="range"
                min={1}
                max={14}
                value={backupPolicyDraft.dailyRetentionDays}
                onChange={(e) => {
                  const value = Math.max(1, Math.min(14, Number(e.target.value) || 3))
                  setBackupPolicyDraft((policy) => ({ ...policy, dailySnapshots: true, dailyRetentionDays: value }))
                }}
              />
            </label>
            <button
              className="sd-btn-green"
              type="button"
              disabled={backupPolicyBusy || !backupPolicyChanged}
              onClick={() => void handleBackupPolicySave()}
            >
              {backupPolicyBusy ? '保存中…' : '保存备份设置'}
            </button>
          </div>
          {backupMessage ? <div className="sd-saves-error">{backupMessage}</div> : null}
          {backupsLoading ? (
            <div className="sd-srv-empty">读取备份列表中…</div>
          ) : backups.length > 0 ? (
            <div className="sd-save-backups-list">
              {backups.map((backup) => {
                const sameNameExists = saves.some((save) => save.name === backup.saveName)
                return (
                  <div className="sd-save-backup-row" key={backup.name}>
                    <div className="sd-save-backup-main">
                      <div className="sd-save-backup-name">{backup.name}</div>
                      <div className="sd-save-backup-meta">
                        <span className="sd-save-backup-kind">{backupKindLabel[backup.kind] ?? backup.kind}</span>
                        <span>原存档：{backup.saveName || '未知'}</span>
                        {backup.farmName ? <span>农场：{backup.farmName}</span> : null}
                        {backup.farmerName ? <span>农民：{backup.farmerName}</span> : null}
                        {backup.gameYear ? (
                          <span>
                            第 {backup.gameYear} 年{' '}
                            {seasonLabel[backup.gameSeason ?? ''] ?? backup.gameSeason}{' '}
                            第 {backup.gameDay} 天
                          </span>
                        ) : null}
                        {backup.farmType ? <span>地图：{farmTypeLabel[backup.farmType] ?? backup.farmType}</span> : null}
                        {backup.fileSizeBytes ? <span>存档：{formatBytes(backup.fileSizeBytes)}</span> : null}
                        <span>备份：{formatBytes(backup.size)}</span>
                        <span>创建：{new Date(backup.createdAt).toLocaleString()}</span>
                        {sameNameExists ? <span className="sd-save-backup-conflict">同名存档存在</span> : null}
                      </div>
                      {backup.parseError ? (
                        <div className="sd-save-card-error">解析失败：{backup.parseError}</div>
                      ) : null}
                    </div>
                    <div className="sd-save-backup-actions">
                      <button
                        className="sd-btn-green"
                        type="button"
                        disabled={restoreBlocked}
                        title={isRunning ? '服务器运行中，请先停止后再恢复备份' : undefined}
                        onClick={() => openRestoreDialog(backup)}
                      >
                        恢复
                      </button>
                      <button
                        className="sd-btn-delete"
                        type="button"
                        disabled={busy}
                        onClick={() => setDeleteBackupTarget(backup)}
                      >
                        彻底删除
                      </button>
                    </div>
                  </div>
                )
              })}
            </div>
          ) : (
            <div className="sd-srv-empty">暂无备份。删除存档前会自动创建备份，覆盖恢复前也会先备份当前存档。</div>
          )}
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
                onClick={() => setDeleteBackupTarget(null)}
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

      {/* ── 恢复备份确认对话框 ── */}
      {restoreBackup ? (
        <div className="sd-confirm-overlay">
          <div className="sd-confirm-dialog sd-confirm-dialog-wide">
            <h3>恢复备份</h3>
            <p>
              确定恢复备份 <strong>"{restoreBackup.name}"</strong> 吗？
              恢复后会生成存档 <strong>"{restoreBackup.saveName}"</strong>。
            </p>
            {isRunning ? (
              <div className="sd-confirm-warning">
                服务器正在运行。请先停止服务器，再恢复备份。
              </div>
            ) : null}
            {restoreNeedsOverwrite || restoreSaveExists ? (
              <div className="sd-confirm-warning">
                同名存档已存在。选择覆盖恢复时，系统会先备份当前存档，再用这个备份覆盖它。
              </div>
            ) : null}
            {restoreError ? <div className="sd-saves-error">{restoreError}</div> : null}
            <div className="sd-confirm-actions">
              <button
                className="sd-btn-tan"
                type="button"
                disabled={busy}
                onClick={() => { setRestoreBackup(null); setRestoreNeedsOverwrite(false); setRestoreError('') }}
              >
                取消
              </button>
              <button
                className="sd-btn-green"
                type="button"
                disabled={restoreBlocked || restoreNeedsOverwrite || restoreSaveExists}
                onClick={() => void handleRestoreConfirmed(false)}
              >
                {busy ? '恢复中…' : '确认恢复'}
              </button>
              {(restoreNeedsOverwrite || restoreSaveExists) ? (
                <button
                  className="sd-btn-delete"
                  type="button"
                  disabled={restoreBlocked}
                  onClick={() => void handleRestoreConfirmed(true)}
                >
                  {busy ? '恢复中…' : '覆盖恢复'}
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
