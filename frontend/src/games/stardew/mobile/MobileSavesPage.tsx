import { useEffect, useRef, useState } from 'react'
import { ApiError, cancelSaveUploadPreview, exportSave, getSaveBackups, restoreSaveBackup, uploadSavePreview, uploadSaveCommitAndStart } from '../../../api'
import type { BackupInfo, SaveInfo, UploadPreviewResult } from '../../../types'
import { errorMessage, formatBytes, formatDate } from '../../../core/helpers'
import type { StardewPageProps } from '../stardew-routes'
import {
  findActiveSaveImportJob,
  findLatestSaveImportJob,
  saveImportErrorPresentation,
  saveImportJobStage,
  saveImportRuntimeUnsupported,
  saveImportSubmissionDisabled,
  type SaveImportDecisionDraft,
  validateSaveImportDecision,
} from '../save-import'
import './MobileSavesPage.css'

type MobileSavesPageProps = Pick<StardewPageProps, 'user' | 'instanceState' | 'dashboardData'>

type SaveStatus = 'active' | 'available' | 'missing'

// 地图类型 -> 农场原画映射（和 SavesSection.tsx 里的 farmTypeLabel/farmTypeAlias/
// saveFarmMapSrc 同构，这里按仓库“各页面自带小工具函数”的既有风格独立维护一份，
// 不共享模块）。以后如果 SaveInfo 新增更精确的地图字段，只需要改这里的映射函数。
const farmTypeLabel: Record<string, string> = {
  standard: '标准农场',
  riverland: '河畔农场',
  forest: '森林农场',
  hilltop: '山顶农场',
  wilderness: '荒野农场',
  fourcorners: '四角农场',
  beach: '海滩农场',
  meadowlands: '草地农场',
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

// 没有可识别地图类型时的兜底原画：仓库已有的纯像素农场场景素材（无假 UI 元素），
// 不从外部拉取图片，也不新增素材。
const DEFAULT_MAP_SRC = '/assets/stardew/ui/backgrounds/background_login_farm_generated.png'

function saveFarmMapSrc(save: SaveInfo | null): string {
  const farmType = save?.farmType
  if (farmType) {
    const key = farmTypeLabel[farmType] ? farmType : farmTypeAlias[farmType]
    if (key) return `/assets/stardew/new-game/farms/${key}.png`
  }
  return DEFAULT_MAP_SRC
}

function saveFarmTypeText(save: SaveInfo): string {
  if (!save.farmType) return '地图未知'
  const label = save.farmTypeLabel ?? farmTypeLabel[save.farmType] ?? save.farmType
  return label !== save.farmType && !farmTypeLabel[save.farmType] ? `${label} (${save.farmType})` : label
}

const SEASON_ZH: Record<string, string> = {
  spring: '春',
  summer: '夏',
  fall: '秋',
  winter: '冬',
}

function saveDateText(save: SaveInfo): string {
  if (!save.gameYear) return '—'
  const season = SEASON_ZH[save.gameSeason?.toLowerCase() ?? ''] ?? save.gameSeason ?? '?'
  return `第 ${save.gameYear} 年${season}季${save.gameDay ?? '?'} 日`
}

function saveStatusLabel(status: SaveStatus): string {
  if (status === 'active') return '当前使用中'
  if (status === 'available') return '可用'
  return '未找到'
}

function saveStatusTagClass(status: SaveStatus): string {
  if (status === 'active') return 'sd-tag sd-tag-green sd-msave-status-tag'
  if (status === 'available') return 'sd-tag sd-tag-gold sd-msave-status-tag'
  return 'sd-tag sd-msave-status-tag'
}

export function MobileSavesPage({ user, instanceState, dashboardData }: MobileSavesPageProps) {
  const isAdmin = user.role === 'admin'
  const state = instanceState?.state ?? null
  // 和桌面 SavesSection.tsx 的 isRunning 判断逐字一致：starting 阶段也算“运行中”，
  // 上传/导入按钮的禁用逻辑要和桌面保持同一套口径。
  const isRunning = state === 'running' || state === 'starting'

  const [refreshBusy, setRefreshBusy] = useState(false)
  const [mapImgFailed, setMapImgFailed] = useState(false)

  const [exportBusy, setExportBusy] = useState(false)
  const [exportError, setExportError] = useState<string | null>(null)

  // 游戏日回档：读法和桌面 SavesSection.tsx 一致，仅管理员可见，独立于共享
  // dashboardData 轮询按需拉取（这个卡片不需要跟随 30s 轮询刷新）。
  const [backups, setBackups] = useState<BackupInfo[]>([])
  const [backupsLoading, setBackupsLoading] = useState(false)
  const [backupsError, setBackupsError] = useState<string | null>(null)
  const [restoreTarget, setRestoreTarget] = useState<BackupInfo | null>(null)
  const [restoreNeedsOverwrite, setRestoreNeedsOverwrite] = useState(false)
  const [restoreBusy, setRestoreBusy] = useState(false)
  const [restoreError, setRestoreError] = useState<string | null>(null)

  // ── 导入存档（上传）：逻辑照抄桌面 SavesSection.tsx 的 handleUploadPreview/
  // handleUploadCommit/handleUploadCancel，只是把 onJobStarted/onSavesChanged 回调
  // 换成移动端读得到的 dashboardData 刷新函数，弹窗视觉重新用 sd-msave-dialog 排布。
  const [uploadOpen, setUploadOpen] = useState(false)
  const [uploadFile, setUploadFile] = useState<File | null>(null)
  const [uploadPreview, setUploadPreview] = useState<UploadPreviewResult | null>(null)
  const [uploadBusy, setUploadBusy] = useState(false)
  const [uploadMessage, setUploadMessage] = useState<string | null>(null)
  const [uploadMessageTone, setUploadMessageTone] = useState<'error' | 'warning'>('error')
  const [uploadRetryBlocked, setUploadRetryBlocked] = useState(false)
  const [importDecision, setImportDecision] = useState<SaveImportDecisionDraft>({ mode: null, platformId: '', takeoverAcknowledged: false })
  const uploadSubmittingRef = useRef(false)

  const savesResult = dashboardData.saves
  const savesLoading = dashboardData.loading && savesResult === null
  const saveRows = savesResult?.saves ?? []
  const activeSaveName = savesResult?.activeSaveName?.trim() ?? ''
  const activeImportJob = findActiveSaveImportJob(dashboardData.jobs)
  const latestImportJob = findLatestSaveImportJob(dashboardData.jobs)
  const importJobStage = latestImportJob
    ? saveImportJobStage(latestImportJob, dashboardData.jobLogsByJobId[latestImportJob.id] ?? [])
    : null
  const runtimeUnsupported = saveImportRuntimeUnsupported(instanceState)
  const importSubmitDisabled = saveImportSubmissionDisabled({
    draft: importDecision,
    uploadBusy,
    generalBusy: uploadRetryBlocked || isRunning,
    runtimeUnsupported,
    activeImport: activeImportJob !== null,
  })

  const activeSave =
    saveRows.find((save) => save.isActive || (activeSaveName !== '' && save.name === activeSaveName)) ?? null
  const displaySave = activeSave ?? saveRows[0] ?? null
  const saveStatus: SaveStatus = !displaySave ? 'missing' : displaySave === activeSave ? 'active' : 'available'

  const mapSrc = mapImgFailed ? DEFAULT_MAP_SRC : saveFarmMapSrc(displaySave)

  // 只展示自动回档点（kind === 'auto'），按游戏日序号倒序——和桌面"游戏日回档"
  // 卡片同一套排序口径，不看现实创建时间。后端已经按策略把数量限制到 N 个。
  const autoBackups = [...backups]
    .filter((b) => b.kind === 'auto')
    .sort((a, b) => (b.gameDayOrdinal ?? 0) - (a.gameDayOrdinal ?? 0))

  async function loadBackups() {
    if (!isAdmin) return
    setBackupsLoading(true)
    setBackupsError(null)
    try {
      const result = await getSaveBackups()
      setBackups(result.backups)
    } catch (e) {
      setBackupsError(errorMessage(e))
    } finally {
      setBackupsLoading(false)
    }
  }

  useEffect(() => {
    void loadBackups()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isAdmin])

  function openRestoreDialog(backup: BackupInfo) {
    setRestoreTarget(backup)
    setRestoreNeedsOverwrite(saveRows.some((save) => save.name === backup.saveName))
    setRestoreError(null)
  }

  function closeRestoreDialog() {
    setRestoreTarget(null)
    setRestoreNeedsOverwrite(false)
    setRestoreError(null)
  }

  async function handleRestoreConfirmed(overwrite: boolean) {
    if (!restoreTarget) return
    setRestoreBusy(true)
    setRestoreError(null)
    try {
      const result = await restoreSaveBackup(restoreTarget.name, overwrite, isRunning)
      setRestoreTarget(null)
      setRestoreNeedsOverwrite(false)
      if (result.jobId) {
        // Server was running: stop -> restore -> start submitted as one job.
        // Refresh the shared job list so the dashboard hook picks it up and
        // polls it (same SSE mechanism as any other lifecycle job); it will
        // refresh saves/state itself once the job finishes.
        dashboardData.refreshJobs()
      } else {
        await Promise.all([dashboardData.refreshSaves(), loadBackups()])
      }
    } catch (e) {
      if (e instanceof ApiError && e.code === 'save_exists') {
        setRestoreNeedsOverwrite(true)
        setRestoreError('同名存档已存在。确认覆盖后，系统会先备份当前存档再恢复此回档点。')
      } else {
        setRestoreError(errorMessage(e))
      }
    } finally {
      setRestoreBusy(false)
    }
  }

  async function handleRefresh() {
    setRefreshBusy(true)
    setMapImgFailed(false)
    try {
      await Promise.all([dashboardData.refreshSaves(), loadBackups()])
    } finally {
      setRefreshBusy(false)
    }
  }

  async function handleExport() {
    if (!displaySave) return
    setExportBusy(true)
    setExportError(null)
    try {
      const { blob, filename } = await exportSave(displaySave.name)
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

  function openUpload() {
    if (!isAdmin || isRunning || runtimeUnsupported || activeImportJob) return
    setUploadOpen(true)
    setUploadFile(null)
    setUploadPreview(null)
    setUploadMessage(null)
    setUploadMessageTone('error')
    setUploadRetryBlocked(false)
    setImportDecision({ mode: null, platformId: '', takeoverAcknowledged: false })
  }

  async function handleUploadPreview() {
    if (!uploadFile) return
    setUploadBusy(true)
    setUploadMessage(null)
    try {
      const res = await uploadSavePreview(uploadFile)
      setUploadPreview(res)
      setImportDecision({ mode: null, platformId: '', takeoverAcknowledged: false })
      setUploadRetryBlocked(false)
    } catch (e) {
      setUploadMessage(errorMessage(e))
    } finally {
      setUploadBusy(false)
    }
  }

  async function handleUploadCommit() {
    if (!uploadPreview || uploadSubmittingRef.current) return
    const validation = validateSaveImportDecision(importDecision)
    if (!validation.valid || !validation.hostHandling || importSubmitDisabled) return
    uploadSubmittingRef.current = true
    setUploadBusy(true)
    setUploadMessage(null)
    try {
      await uploadSaveCommitAndStart(uploadPreview.token, validation.hostHandling)
      setUploadOpen(false)
      setUploadPreview(null)
      setUploadFile(null)
      dashboardData.requestInviteCodeRefresh()
      dashboardData.refreshInstanceState()
      dashboardData.refreshJobs()
      void dashboardData.refreshSaves()
    } catch (e) {
      const presentation = saveImportErrorPresentation(e)
      setUploadMessage(presentation.message)
      setUploadMessageTone(presentation.tone)
      setUploadRetryBlocked(presentation.retryBlocked)
    } finally {
      uploadSubmittingRef.current = false
      setUploadBusy(false)
    }
  }

  function handleUploadCancel() {
    if (uploadPreview) {
      // 尽力清理挂起的 token，和桌面版同一处理，失败静默忽略
      void cancelSaveUploadPreview(uploadPreview.token).catch(() => {})
    }
    setUploadOpen(false)
    setUploadPreview(null)
    setUploadFile(null)
    setUploadMessage(null)
    setUploadMessageTone('error')
    setUploadRetryBlocked(false)
    setImportDecision({ mode: null, platformId: '', takeoverAcknowledged: false })
  }

  return (
    <div className="sd-msave-wrap">
      <div className="sd-msave-page-header">
        <div className="sd-msave-page-title">
          <img src="/assets/stardew/ui/icons/icon_nav_saves_chest_image2.png" alt="" />
          存档
        </div>
        <button
          type="button"
          className="sd-btn-tan sd-msave-refresh-btn"
          onClick={() => void handleRefresh()}
          disabled={refreshBusy}
        >
          {refreshBusy ? '刷新中…' : '刷新'}
        </button>
      </div>

      {dashboardData.savesError ? (
        <div className="sd-notice sd-notice--error sd-msave-page-notice">读取存档信息失败：{dashboardData.savesError}</div>
      ) : null}

      {savesLoading ? (
        <section className="sd-panel sd-msave-card">
          <div className="sd-msave-empty">
            <div className="sd-msave-empty-title">正在读取存档信息</div>
          </div>
        </section>
      ) : (
        <>
          {!displaySave ? (
            <section className="sd-panel sd-msave-card">
              <div className="sd-msave-empty">
                <div className="sd-msave-empty-title">暂无存档</div>
                <div className="sd-msave-empty-desc">创建新存档或上传已有存档后，这里会显示存档地图与详细信息。</div>
              </div>
            </section>
          ) : (
            <>
              <section className="sd-panel sd-msave-card sd-msave-summary-card">
                <div className="sd-msave-card-title">
                  <img src="/assets/stardew/ui/icons/icon_top_summary_save.png" alt="" />
                  核心信息
                </div>
                <div className="sd-msave-summary-body">
                  <div className="sd-msave-map-thumb-block">
                    <span className={saveStatusTagClass(saveStatus)}>{saveStatusLabel(saveStatus)}</span>
                    <div className="sd-msave-map-thumb-frame">
                      <img
                        className="sd-msave-map-thumb-img"
                        src={mapSrc}
                        alt=""
                        onError={() => setMapImgFailed(true)}
                      />
                    </div>
                  </div>
                  <div className="sd-msave-info-grid sd-msave-summary-grid">
                    <div className="sd-msave-info-item">
                      <span className="sd-msave-info-label">存档名称</span>
                      <span className="sd-msave-info-value">{displaySave.name}</span>
                    </div>
                    <div className="sd-msave-info-item">
                      <span className="sd-msave-info-label">农场名称</span>
                      <span className="sd-msave-info-value">{displaySave.farmName || '—'}</span>
                    </div>
                    <div className="sd-msave-info-item">
                      <span className="sd-msave-info-label">农场主</span>
                      <span className="sd-msave-info-value">{displaySave.farmerName || '—'}</span>
                    </div>
                    <div className="sd-msave-info-item">
                      <span className="sd-msave-info-label">游戏日期</span>
                      <span className="sd-msave-info-value">{saveDateText(displaySave)}</span>
                    </div>
                  </div>
                </div>
              </section>

              <section className="sd-panel sd-msave-card">
                <div className="sd-msave-card-title">
                  <img src="/assets/stardew/ui/icons/icon_right_rail_in_progress_clock_image2.png" alt="" />
                  更多信息
                </div>
                <div className="sd-msave-info-grid">
                  <div className="sd-msave-info-item">
                    <span className="sd-msave-info-label">地图类型</span>
                    <span className="sd-msave-info-value">{saveFarmTypeText(displaySave)}</span>
                  </div>
                  <div className="sd-msave-info-item">
                    <span className="sd-msave-info-label">存档大小</span>
                    <span className="sd-msave-info-value">
                      {displaySave.fileSizeBytes ? formatBytes(displaySave.fileSizeBytes) : '—'}
                    </span>
                  </div>
                  <div className="sd-msave-info-item">
                    <span className="sd-msave-info-label">最后保存时间</span>
                    <span className="sd-msave-info-value">
                      {displaySave.modifiedAt ? formatDate(displaySave.modifiedAt) : '—'}
                    </span>
                  </div>
                  <div className="sd-msave-info-item">
                    <span className="sd-msave-info-label">存档状态</span>
                    <span className="sd-msave-info-value">{saveStatusLabel(saveStatus)}</span>
                  </div>
                </div>
                {displaySave.parseError ? (
                  <div className="sd-notice sd-notice--error sd-msave-notice">存档解析失败：{displaySave.parseError}</div>
                ) : null}
              </section>
            </>
          )}

          {isAdmin ? (
            <section className="sd-panel sd-msave-card">
              <div className="sd-msave-card-title">
                <img src="/assets/stardew/ui/icons/icon_nav_tasks_scroll_image2.png" alt="" />
                游戏日回档
              </div>
              {backupsError ? <div className="sd-notice sd-notice--error sd-msave-notice">{backupsError}</div> : null}
              {backupsLoading ? (
                <div className="sd-msave-empty">
                  <div className="sd-msave-empty-title">正在读取回档列表</div>
                </div>
              ) : autoBackups.length > 0 ? (
                <div className="sd-msave-gameday-list">
                  {autoBackups.map((backup) => (
                    <div className="sd-msave-gameday-row" key={backup.name}>
                      <div className="sd-msave-gameday-main">
                        <span className="sd-msave-gameday-date">{saveDateText(backup)}</span>
                        <span className="sd-msave-gameday-meta">
                          {backup.farmName || backup.saveName || '未知'}
                          {backup.farmerName ? ` · ${backup.farmerName}` : ''}
                        </span>
                        <span className="sd-msave-gameday-meta sd-msave-gameday-meta-muted">
                          {new Date(backup.createdAt).toLocaleString()} · {formatBytes(backup.size)}
                        </span>
                      </div>
                      <button
                        type="button"
                        className="sd-btn-green sd-msave-gameday-btn"
                        disabled={restoreBusy}
                        onClick={() => openRestoreDialog(backup)}
                      >
                        回档到此日
                      </button>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="sd-msave-empty">
                  <div className="sd-msave-empty-desc">暂无游戏日回档点。游戏内睡觉存档成功后会自动生成。</div>
                </div>
              )}
            </section>
          ) : null}

          <section className="sd-panel sd-msave-card">
            <div className="sd-msave-card-title">
              <img src="/assets/stardew/ui/icons/icon_nav_saves_chest_image2.png" alt="" />
              存档操作
            </div>
            <div className="sd-msave-op-list">
              <button
                type="button"
                className="sd-btn-tan sd-msave-op-btn"
                disabled={exportBusy || !displaySave}
                title={!displaySave ? '暂无可导出的存档' : '将当前存档导出为 ZIP 文件'}
                onClick={() => void handleExport()}
              >
                {exportBusy ? '导出中…' : '导出存档'}
              </button>
              <button
                type="button"
                className="sd-btn-tan sd-msave-op-btn"
                disabled={!isAdmin || isRunning || runtimeUnsupported || activeImportJob !== null}
                title={
                  !isAdmin
                    ? '仅管理员可执行此操作'
                    : runtimeUnsupported
                      ? '当前 Junimo 运行版本不支持安全导入'
                      : activeImportJob
                        ? '已有导入任务正在进行'
                    : isRunning
                      ? '服务器运行中，请先停止后再上传存档'
                      : '上传本地 Stardew Valley 存档 ZIP'
                }
                onClick={openUpload}
              >
                导入存档
              </button>
            </div>
            {exportError ? <div className="sd-notice sd-notice--error sd-msave-notice">{exportError}</div> : null}
          </section>
        </>
      )}

      {latestImportJob && importJobStage ? (
        <section className={`sd-panel sd-msave-import-job sd-msave-import-job--${importJobStage.tone}`} role="status">
          <strong>{importJobStage.label}</strong>
          <span>任务 {latestImportJob.id.slice(0, 10)} · {latestImportJob.status}</span>
          <small>{activeImportJob ? '关闭弹窗不会取消后端事务' : '状态已从任务记录恢复'}</small>
        </section>
      ) : null}

      {uploadOpen ? (
        <div className="sd-msave-dialog-overlay" role="dialog" aria-modal="true">
          <div className="sd-panel sd-msave-dialog">
            <h3>导入存档</h3>

            {uploadMessage ? <div className={`sd-notice ${uploadMessageTone === 'warning' ? 'sd-msave-import-warning' : 'sd-notice--error'} sd-msave-notice`}>{uploadMessage}</div> : null}

            {!uploadPreview ? (
              <>
                <p className="sd-msave-dialog-text">上传一个包含 Stardew Valley 存档的 ZIP 文件（最大 100 MB）。</p>
                <label className="sd-msave-field">
                  <span>选择 ZIP 文件</span>
                  <input
                    className="sd-input"
                    type="file"
                    accept=".zip"
                    onChange={(e) => setUploadFile(e.target.files?.[0] ?? null)}
                    disabled={uploadBusy}
                  />
                </label>
                <div className="sd-msave-dialog-actions">
                  <button
                    type="button"
                    className="sd-btn-tan sd-msave-dialog-btn"
                    onClick={handleUploadCancel}
                    disabled={uploadBusy}
                  >
                    取消
                  </button>
                  <button
                    type="button"
                    className="sd-btn-green sd-msave-dialog-btn"
                    onClick={() => void handleUploadPreview()}
                    disabled={uploadBusy || !uploadFile}
                  >
                    {uploadBusy ? '解析中…' : '预览存档'}
                  </button>
                </div>
              </>
            ) : (
              <>
                <div className="sd-msave-preview-table">
                  <div className="sd-msave-preview-row">
                    <span className="sd-msave-preview-label">存档目录名</span>
                    <strong>{uploadPreview.saveName}</strong>
                  </div>
                  {uploadPreview.preview.farmName ? (
                    <div className="sd-msave-preview-row">
                      <span className="sd-msave-preview-label">农场名</span>
                      <span>{uploadPreview.preview.farmName}</span>
                    </div>
                  ) : null}
                  {uploadPreview.preview.farmerName ? (
                    <div className="sd-msave-preview-row">
                      <span className="sd-msave-preview-label">农场主</span>
                      <span>{uploadPreview.preview.farmerName}</span>
                    </div>
                  ) : null}
                  {uploadPreview.preview.gameYear ? (
                    <div className="sd-msave-preview-row">
                      <span className="sd-msave-preview-label">游戏时间</span>
                      <span>{saveDateText(uploadPreview.preview)}</span>
                    </div>
                  ) : null}
                  <div className="sd-msave-preview-row">
                    <span className="sd-msave-preview-label">地图类型</span>
                    <span>{saveFarmTypeText(uploadPreview.preview)}</span>
                  </div>
                  {uploadPreview.preview.fileSizeBytes ? (
                    <div className="sd-msave-preview-row">
                      <span className="sd-msave-preview-label">文件大小</span>
                      <span>{formatBytes(uploadPreview.preview.fileSizeBytes)}</span>
                    </div>
                  ) : null}
                  {uploadPreview.preview.modifiedAt ? (
                    <div className="sd-msave-preview-row">
                      <span className="sd-msave-preview-label">修改时间</span>
                      <span>{formatDate(uploadPreview.preview.modifiedAt)}</span>
                    </div>
                  ) : null}
                  {uploadPreview.preview.parseError ? (
                    <div className="sd-msave-preview-row">
                      <span className="sd-msave-preview-label">解析状态</span>
                      <span className="sd-msave-preview-error">{uploadPreview.preview.parseError}</span>
                    </div>
                  ) : null}
                </div>
                <fieldset className="sd-msave-import-options">
                  <legend>原主机角色处理方式</legend>
                  <label className={`sd-msave-import-option ${importDecision.mode === 'swap_to_player' ? 'is-selected' : ''}`}>
                    <input
                      type="radio"
                      name="mobile-save-import-host-mode"
                      checked={importDecision.mode === 'swap_to_player'}
                      onChange={() => setImportDecision((current) => ({ ...current, mode: 'swap_to_player', takeoverAcknowledged: false }))}
                      disabled={uploadBusy || activeImportJob !== null}
                    />
                    <span><strong>保留原主机角色给玩家使用</strong><small>推荐。用于把原角色绑定回原玩家；技能、物品、关系、住宅和家庭迁移由 Junimo 处理。</small></span>
                  </label>
                  {importDecision.mode === 'swap_to_player' ? (
                    <label className="sd-msave-import-platform">
                      <span>原主机平台 ID</span>
                      <input
                        className="sd-input"
                        type="text"
                        inputMode="numeric"
                        autoComplete="off"
                        value={importDecision.platformId}
                        onChange={(event) => setImportDecision((current) => ({ ...current, platformId: event.target.value }))}
                        placeholder="Steam64 / GOG ID"
                        disabled={uploadBusy || activeImportJob !== null}
                      />
                      <small>支持 Steam64/GOG ID，仅允许十进制数字，并始终按字符串提交。</small>
                    </label>
                  ) : null}
                  <label className={`sd-msave-import-option sd-msave-import-option--danger ${importDecision.mode === 'virtual_host_takeover' ? 'is-selected' : ''}`}>
                    <input
                      type="radio"
                      name="mobile-save-import-host-mode"
                      checked={importDecision.mode === 'virtual_host_takeover'}
                      onChange={() => setImportDecision((current) => ({ ...current, mode: 'virtual_host_takeover', platformId: '', takeoverAcknowledged: false }))}
                      disabled={uploadBusy || activeImportJob !== null}
                    />
                    <span><strong>由 Junimo 虚拟主机接管原角色</strong><small>高风险：原玩家之后可能无法再选择该角色。</small></span>
                  </label>
                  {importDecision.mode === 'virtual_host_takeover' ? (
                    <label className="sd-msave-import-ack">
                      <input
                        type="checkbox"
                        checked={importDecision.takeoverAcknowledged}
                        onChange={(event) => setImportDecision((current) => ({ ...current, takeoverAcknowledged: event.target.checked }))}
                        disabled={uploadBusy || activeImportJob !== null}
                      />
                      <span>我明白原主机角色将由虚拟主机接管，原玩家可能无法再选择该角色。</span>
                    </label>
                  ) : null}
                </fieldset>
                {runtimeUnsupported ? <div className="sd-notice sd-notice--error sd-msave-notice">当前 Junimo 运行版本不支持安全导入，请先升级运行组件。</div> : null}
                {activeImportJob ? <div className="sd-notice sd-notice--error sd-msave-notice">已有导入任务正在进行，不能重复提交。</div> : null}
                <div className="sd-msave-dialog-actions">
                  <button
                    type="button"
                    className="sd-btn-tan sd-msave-dialog-btn"
                    onClick={handleUploadCancel}
                    disabled={uploadBusy}
                  >
                    取消
                  </button>
                  <button
                    type="button"
                    className="sd-btn-green sd-msave-dialog-btn"
                    onClick={() => void handleUploadCommit()}
                    disabled={importSubmitDisabled}
                  >
                    {uploadBusy ? '正在创建导入任务…' : '确认并开始导入'}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      ) : null}

      {restoreTarget ? (
        <div className="sd-msave-dialog-overlay" role="dialog" aria-modal="true">
          <div className="sd-panel sd-msave-dialog">
            <h3>回档到此日</h3>
            <p className="sd-msave-dialog-text">
              确定回档到 "{restoreTarget.name}" 吗？回档后会生成存档 "{restoreTarget.saveName}"。
            </p>
            {isRunning ? (
              <div className="sd-notice sd-notice--error sd-msave-notice">
                服务器正在运行中。确认后将自动停止服务器、完成回档，并重新启动服务器；整个过程可能需要几分钟，请勿在此期间反复点击。
              </div>
            ) : null}
            {restoreNeedsOverwrite ? (
              <div className="sd-notice sd-notice--error sd-msave-notice">
                同名存档已存在。选择覆盖回档时，系统会先备份当前存档，再用这个回档点覆盖它。
              </div>
            ) : null}
            {restoreError ? <div className="sd-notice sd-notice--error sd-msave-notice">{restoreError}</div> : null}
            <div className="sd-msave-dialog-actions">
              <button
                type="button"
                className="sd-btn-tan sd-msave-dialog-btn"
                onClick={closeRestoreDialog}
                disabled={restoreBusy}
              >
                取消
              </button>
              {restoreNeedsOverwrite ? (
                <button
                  type="button"
                  className="sd-btn-delete sd-msave-dialog-btn"
                  onClick={() => void handleRestoreConfirmed(true)}
                  disabled={!isAdmin || restoreBusy}
                >
                  {restoreBusy ? (isRunning ? '正在停止服务器…' : '回档中…') : isRunning ? '覆盖回档（自动重启）' : '覆盖回档'}
                </button>
              ) : (
                <button
                  type="button"
                  className="sd-btn-green sd-msave-dialog-btn"
                  onClick={() => void handleRestoreConfirmed(false)}
                  disabled={!isAdmin || restoreBusy}
                >
                  {restoreBusy ? (isRunning ? '正在停止服务器…' : '回档中…') : isRunning ? '确认回档（自动重启）' : '确认回档'}
                </button>
              )}
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
