import { useCallback, useEffect, useRef, useState } from 'react'
import type { ControlCommand, Job, JobLog } from '../../../types'
import type { StardewPageProps } from '../stardew-routes'
import {
  clearJobErrorLogs,
  clearJobs,
  createJobEventSource,
  getInstanceVNCConfig,
  getControlCommands,
  getJob,
  getJobLogs,
  getJobs,
  updateInstanceVNCPort,
} from '../../../api'
import {
  appendUniqueLog,
  errorMessage,
  formatDate,
  isTerminalJobStatus,
  jobDisplayName,
  shortJobID,
} from '../../../core/helpers'

// ── 常量表 ────────────────────────────────────────────────────────────────────

const pullProgressRe = /^\[pull:progress:(\d+):(\d+)\]$/

const STATUS_LABELS: Record<string, string> = {
  queued: '排队中',
  running: '运行中',
  succeeded: '已完成',
  failed: '失败',
  canceled: '已取消',
}

const COMMAND_STATUS_LABELS: Record<string, string> = {
  queued: '排队中', running: '处理中', succeeded: '已确认成功', dispatched: '已派发',
  failed: '明确失败', expired: '结果已过期', unknown: '结果未知',
}

const COMMAND_TYPE_LABELS: Record<string, string> = {
  'warp-home': '回家', kick: '踢出', 'approve-auth': '批准认证', broadcast: '喊话', say: '喊话',
  ban: '封禁', 'trigger-event': '触发节日', 'enable-joja': '启用 Joja', 'save-now': '立即保存',
}

const TYPE_LABELS: Record<string, string> = {
  stardew_install: '安装游戏',
  stardew_start: '启动服务器',
  stardew_stop: '停止服务器',
  stardew_restart: '重启服务器',
  stardew_custom_new_game: '新建存档',
  stardew_select_save_and_start: '选档启动',
  stardew_upload_save_and_start: '上传存档启动',
  test: '测试任务',
  test_fail: '失败测试',
}

const TYPE_ICON_CLASSES: Record<string, string> = {
  stardew_install: 'install',
  stardew_start: 'start',
  stardew_stop: 'server',
  stardew_restart: 'server',
  stardew_custom_new_game: 'saves',
  stardew_select_save_and_start: 'start',
  stardew_upload_save_and_start: 'saves',
  mod_remote_install: 'mods',
  mod_nexus_install: 'mods',
  test: 'server',
  test_fail: 'server',
}

function typeLabel(t: string): string {
  return TYPE_LABELS[t] ?? t
}

function statusLabel(s: string): string {
  return STATUS_LABELS[s] ?? s
}

function statusCls(s: string): string {
  return `sd-jobs-status sd-jobs-status-${s}`
}

function typeIconCls(t: string): string {
  return `sd-jobs-type-icon sd-jobs-type-icon-${TYPE_ICON_CLASSES[t] ?? 'server'}`
}

function selectedJobIdFromLocation(): string {
  if (typeof window === 'undefined') return ''
  return new URLSearchParams(window.location.search).get('jobId')?.trim() ?? ''
}

function extractPullProgress(
  logs: JobLog[],
  jobType?: string,
): { done: number; total: number; percent: number } | null {
  if (jobType !== 'stardew_install') return null
  let latest: { done: number; total: number } | null = null
  for (const log of logs) {
    const m = log.message.match(pullProgressRe)
    if (m) latest = { done: parseInt(m[1], 10), total: parseInt(m[2], 10) }
  }
  if (!latest || latest.total === 0) return null
  return { ...latest, percent: Math.round((latest.done / latest.total) * 100) }
}

function isVNCPortProblem(job: Job | null, logs: JobLog[]): boolean {
  const text = [
    job?.errorMessage ?? '',
    ...logs.map((log) => log.message),
  ].join('\n')
  const lower = text.toLowerCase()
  return (
    (text.includes('VNC 端口') || lower.includes('vnc port') || lower.includes('vnc_port')) &&
    (text.includes('占用') ||
      text.includes('系统保留') ||
      lower.includes('forbidden by its access permissions') ||
      lower.includes('port is already allocated') ||
      lower.includes('ports are not available') ||
      lower.includes('address already in use'))
  )
}

function suggestNextPort(port: string): string {
  const n = Number.parseInt(port, 10)
  if (!Number.isFinite(n) || n < 1 || n >= 65535) return ''
  return String(n + 1)
}

// ── 组件 ──────────────────────────────────────────────────────────────────────

export function JobsLogsPage({ user, dashboardData }: StardewPageProps) {
  const [jobs, setJobs] = useState<Job[]>([])
  const [controlCommands, setControlCommands] = useState<ControlCommand[]>([])
  const [controlCommandsError, setControlCommandsError] = useState('')
  const [selectedJobId, setSelectedJobId] = useState<string | null>(null)
  const [selectedJob, setSelectedJob] = useState<Job | null>(null)
  const [logs, setLogs] = useState<JobLog[]>([])
  const [loadingJobs, setLoadingJobs] = useState(true)
  const [loadingDetail, setLoadingDetail] = useState(false)
  const [jobsError, setJobsError] = useState('')
  const [detailError, setDetailError] = useState('')
  const [sseError, setSseError] = useState('')
  const [logsTruncated, setLogsTruncated] = useState(false)
  const [busy, setBusy] = useState(false)
  const [showClearConfirm, setShowClearConfirm] = useState(false)
  const [showClearErrorConfirm, setShowClearErrorConfirm] = useState(false)
  const [showVNCPortModal, setShowVNCPortModal] = useState(false)
  const [vncPortLoading, setVNCPortLoading] = useState(false)
  const [vncPortSaving, setVNCPortSaving] = useState(false)
  const [currentVNCPort, setCurrentVNCPort] = useState('')
  const [newVNCPort, setNewVNCPort] = useState('')
  const [vncPortError, setVNCPortError] = useState('')
  const [vncPortMessage, setVNCPortMessage] = useState('')

  const logWindowRef = useRef<HTMLDivElement | null>(null)
  const autoSelectedRef = useRef(false)

  const { refreshJobs: dashRefreshJobs, refreshInstanceState, refreshInviteCode } = dashboardData

  const loadJobs = useCallback(async (): Promise<Job[]> => {
    try {
      const res = await getJobs()
      setJobs(res.jobs)
      return res.jobs
    } catch (e) {
      setJobsError(errorMessage(e))
      return []
    }
  }, [])

  const loadControlCommands = useCallback(async () => {
    try {
      const res = await getControlCommands()
      setControlCommands(res.commands)
      setControlCommandsError('')
    } catch (e) {
      setControlCommandsError(errorMessage(e))
    }
  }, [])

  // 初始加载任务列表，自动选中最近一条
  useEffect(() => {
    void (async () => {
      setLoadingJobs(true)
      const loaded = await loadJobs()
      if (!autoSelectedRef.current) {
        autoSelectedRef.current = true
        const requestedJobId = selectedJobIdFromLocation()
        if (requestedJobId) {
          setSelectedJobId(requestedJobId)
        } else if (loaded.length > 0) {
          setSelectedJobId(loaded[0].id)
        }
      }
      setLoadingJobs(false)
    })()
  }, [loadJobs])

  useEffect(() => {
    void loadControlCommands()
    const timer = window.setInterval(() => void loadControlCommands(), 5000)
    return () => window.clearInterval(timer)
  }, [loadControlCommands])

  // 选中任务变化时：加载详情 + 日志 + 开启 SSE（非终态任务）
  useEffect(() => {
    if (!selectedJobId) return

    setSseError('')
    setDetailError('')
    setSelectedJob(null)
    setLogs([])
    setLogsTruncated(false)
    setLoadingDetail(true)

    let cancelled = false
    let es: EventSource | null = null

    void (async () => {
      try {
        const [jobRes, logsRes] = await Promise.all([
          getJob(selectedJobId),
          getJobLogs(selectedJobId, 0),
        ])

        if (cancelled) return

        setSelectedJob(jobRes.job)
        setLogs(logsRes.logs)
        setLogsTruncated(logsRes.logs.length >= 1000)
        setLoadingDetail(false)

        // 非终态任务接入 SSE
        if (!isTerminalJobStatus(jobRes.job.status)) {
          const lastSeq =
            logsRes.logs.length > 0
              ? logsRes.logs[logsRes.logs.length - 1].sequence
              : 0
          es = createJobEventSource(selectedJobId, lastSeq)

          es.addEventListener('log', (ev) => {
            if (cancelled) return
            try {
              const log = JSON.parse((ev as MessageEvent<string>).data) as JobLog
              setLogs((prev) => appendUniqueLog(prev, log))
            } catch {
              // 忽略解析失败的事件
            }
          })

          es.addEventListener('finished', () => {
            if (cancelled) return
            es?.close()
            // 刷新任务详情（获取最终状态）
            void getJob(selectedJobId)
              .then((r) => {
                if (!cancelled) setSelectedJob(r.job)
              })
              .catch(() => {})
            // 刷新任务列表（本地 + 公共数据层）
            void loadJobs()
            dashRefreshJobs()
            refreshInstanceState()
            refreshInviteCode()
          })

          es.onerror = () => {
            if (cancelled) return
            setSseError('实时日志连接已断开，可手动点击刷新')
            es?.close()
          }
        }
      } catch (e) {
        if (!cancelled) {
          setDetailError(errorMessage(e))
          setLoadingDetail(false)
        }
      }
    })()

    return () => {
      cancelled = true
      es?.close()
    }
  }, [selectedJobId, loadJobs, dashRefreshJobs, refreshInstanceState, refreshInviteCode])

  // 新日志到来时自动滚动到底部
  useEffect(() => {
    const logWindow = logWindowRef.current
    if (!logWindow) return
    logWindow.scrollTo({ top: logWindow.scrollHeight, behavior: 'smooth' })
  }, [logs.length])

  async function handleRefresh() {
    setBusy(true)
    setJobsError('')
    setDetailError('')
    setSseError('')
    try {
      const loaded = await loadJobs()
      await loadControlCommands()
      dashRefreshJobs()
      if (!selectedJobId && loaded.length > 0) {
        setSelectedJobId(loaded[0].id)
      } else if (selectedJobId) {
        const [jobRes, logsRes] = await Promise.all([
          getJob(selectedJobId).catch(() => null),
          getJobLogs(selectedJobId, 0).catch(() => null),
        ])
        if (jobRes) setSelectedJob(jobRes.job)
        if (logsRes) {
          setLogs(logsRes.logs)
          setLogsTruncated(logsRes.logs.length >= 1000)
        }
      }
    } finally {
      setBusy(false)
    }
  }

  async function handleClearConfirmed() {
    setShowClearConfirm(false)
    setBusy(true)
    setJobsError('')
    try {
      await clearJobs()
      setJobs([])
      setSelectedJobId(null)
      setSelectedJob(null)
      setLogs([])
      setDetailError('')
      setLogsTruncated(false)
      autoSelectedRef.current = false
      dashRefreshJobs()
    } catch (e) {
      setJobsError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  async function handleClearErrorConfirmed() {
    setShowClearErrorConfirm(false)
    setBusy(true)
    setJobsError('')
    setDetailError('')
    try {
      await clearJobErrorLogs()
      const loaded = await loadJobs()
      dashRefreshJobs()
      if (selectedJobId) {
        const [jobRes, logsRes] = await Promise.all([
          getJob(selectedJobId).catch(() => null),
          getJobLogs(selectedJobId, 0).catch(() => null),
        ])
        if (jobRes) setSelectedJob(jobRes.job)
        if (logsRes) {
          setLogs(logsRes.logs)
          setLogsTruncated(logsRes.logs.length >= 1000)
        }
      } else if (loaded.length > 0) {
        setSelectedJobId(loaded[0].id)
      }
    } catch (e) {
      setJobsError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  async function handleOpenVNCPortModal() {
    setShowVNCPortModal(true)
    setVNCPortLoading(true)
    setVNCPortError('')
    setVNCPortMessage('')
    try {
      const res = await getInstanceVNCConfig()
      setCurrentVNCPort(res.vncPort)
      setNewVNCPort(suggestNextPort(res.vncPort))
    } catch (e) {
      setVNCPortError(errorMessage(e))
    } finally {
      setVNCPortLoading(false)
    }
  }

  function handleCloseVNCPortModal() {
    if (vncPortSaving) return
    setShowVNCPortModal(false)
    setVNCPortError('')
    setVNCPortMessage('')
  }

  async function handleSaveVNCPort() {
    const trimmed = newVNCPort.trim()
    const n = Number.parseInt(trimmed, 10)
    if (!/^\d+$/.test(trimmed) || !Number.isFinite(n) || n < 1 || n > 65535) {
      setVNCPortError('VNC 端口必须是 1 到 65535 之间的数字')
      return
    }
    setVNCPortSaving(true)
    setVNCPortError('')
    setVNCPortMessage('')
    try {
      const res = await updateInstanceVNCPort(trimmed)
      setCurrentVNCPort(res.vncPort)
      setNewVNCPort(res.vncPort)
      setVNCPortMessage('VNC 端口已更新，请重新启动服务器。')
      refreshInstanceState()
    } catch (e) {
      setVNCPortError(errorMessage(e))
    } finally {
      setVNCPortSaving(false)
    }
  }

  const visibleLogs = logs.filter((log) => !pullProgressRe.test(log.message))
  const pullProgress = extractPullProgress(logs, selectedJob?.type)
  const showVNCPortFix = user.role === 'admin' && isVNCPortProblem(selectedJob, logs)
  const isLiveStreaming =
    selectedJob !== null && !isTerminalJobStatus(selectedJob.status) && !sseError

  return (
    <div className="sd-page sd-jobs-page">
      {/* ── 页头 ── */}
      <div className="sd-page-header">
        <img
          className="sd-page-icon"
          src="/assets/stardew/ui/icons/icon_nav_tasks_scroll_image2.png"
          alt=""
        />
        <div>
          <h2 className="sd-page-title">任务与日志</h2>
          <p className="sd-page-desc">查看后台任务执行历史，支持实时日志流。</p>
        </div>
      </div>

      {/* ── 工具栏 ── */}
      <div className="sd-jobs-toolbar">
        <div className="sd-jobs-toolbar-actions sd-actionbar">
          <button
            className="sd-btn-tan"
            disabled={busy || loadingJobs}
            onClick={() => void handleRefresh()}
            type="button"
          >
            {busy || loadingJobs ? '刷新中…' : '刷新'}
          </button>
          {user.role === 'admin' && jobs.length > 0 ? (
            <>
              <button
                className="sd-btn-tan"
                disabled={busy}
                onClick={() => setShowClearErrorConfirm(true)}
                type="button"
              >
                清空错误日志
              </button>
              <button
                className="sd-btn-delete"
                disabled={busy}
                onClick={() => setShowClearConfirm(true)}
                type="button"
              >
                清空任务历史
              </button>
            </>
          ) : null}
        </div>
        {jobsError ? <div className="sd-jobs-error-banner">{jobsError}</div> : null}
      </div>

      {/* ── 主体 ── */}
      {loadingJobs ? (
        <div className="sd-jobs-loading">加载任务列表中…</div>
      ) : jobs.length === 0 ? (
        <div className="sd-jobs-empty">
          <div className="sd-jobs-empty-title">暂无任务记录</div>
          <p>安装游戏、启动服务器、创建/上传存档等操作完成后会在此显示任务记录。</p>
        </div>
      ) : (
        <div className="sd-jobs-layout">
          {/* ── 左：任务列表 ── */}
          <div className="sd-jobs-list" role="list" aria-label="任务列表">
            <div className="sd-jobs-list-head">
              <span className="sd-jobs-list-head-title">任务列表</span>
              <span className="sd-jobs-list-head-count">{jobs.length} 个任务</span>
            </div>
            {jobs.map((job) => (
              <button
                key={job.id}
                className={`sd-jobs-list-row${selectedJobId === job.id ? ' active' : ''}`}
                onClick={() => setSelectedJobId(job.id)}
                type="button"
                role="listitem"
                aria-pressed={selectedJobId === job.id}
              >
                <span className={typeIconCls(job.type)} aria-hidden="true" />
                <div className="sd-jobs-list-row-content">
                  <div className="sd-jobs-list-row-type" title={jobDisplayName(job)}>
                    {job.displayName?.trim() || typeLabel(job.type)}
                  </div>
                  <div className="sd-jobs-list-row-id" title={job.id}>
                    ID: {shortJobID(job.id)}
                  </div>
                  <div className="sd-jobs-list-row-date">
                    {formatDate(job.createdAt)}
                  </div>
                </div>
                <span className={statusCls(job.status)} aria-label={`状态：${statusLabel(job.status)}`}>
                  {statusLabel(job.status)}
                </span>
              </button>
            ))}
          </div>

          {/* ── 右：任务详情 ── */}
          <div className="sd-jobs-detail" aria-label="任务详情">
            {loadingDetail ? (
              <div className="sd-jobs-loading">加载任务详情中…</div>
            ) : detailError ? (
              <div className="sd-jobs-error-banner sd-jobs-error-banner-prominent">
                <span className="sd-jobs-error-label">加载失败：</span>
                {detailError}
              </div>
            ) : selectedJob ? (
              <>
                {/* 详情头 */}
                <div className="sd-jobs-detail-head">
                  <div className="sd-jobs-detail-title-wrap">
                    <span className={typeIconCls(selectedJob.type)} aria-hidden="true" />
                    <div className="sd-jobs-detail-title-main">
                      <div className="sd-jobs-detail-title" title={jobDisplayName(selectedJob)}>
                        {selectedJob.displayName?.trim() || typeLabel(selectedJob.type)}
                      </div>
                      <div className="sd-jobs-detail-id" title={selectedJob.id}>
                        {shortJobID(selectedJob.id)}
                      </div>
                    </div>
                  </div>
                  <span className={statusCls(selectedJob.status)}>
                    {statusLabel(selectedJob.status)}
                  </span>
                </div>

                {/* 时间元数据 */}
                <div className="sd-jobs-detail-meta">
                  <span className="sd-jobs-detail-meta-item">
                    <span className="sd-jobs-detail-meta-label">创建：</span>
                    {formatDate(selectedJob.createdAt)}
                  </span>
                  {selectedJob.startedAt ? (
                    <span className="sd-jobs-detail-meta-item">
                      <span className="sd-jobs-detail-meta-label">开始：</span>
                      {formatDate(selectedJob.startedAt)}
                    </span>
                  ) : null}
                  {selectedJob.finishedAt ? (
                    <span className="sd-jobs-detail-meta-item">
                      <span className="sd-jobs-detail-meta-label">结束：</span>
                      {formatDate(selectedJob.finishedAt)}
                    </span>
                  ) : null}
                </div>

                {pullProgress ? (
                  <div className="sd-jobs-pull-progress">
                    <div className="sd-jobs-pull-header">
                      <span>进度</span>
                      <span>
                        {pullProgress.percent}% ({pullProgress.done}/{pullProgress.total})
                      </span>
                    </div>
                    <div className="sd-jobs-progress-wrap">
                      <div className="sd-jobs-progress-track">
                        <div
                          className={`sd-jobs-progress-fill${pullProgress.done === pullProgress.total ? ' done' : ''}`}
                          style={{ width: `${pullProgress.percent}%` }}
                        />
                      </div>
                    </div>
                  </div>
                ) : null}

                <div className="sd-jobs-stream-row">
                  {sseError ? (
                    <div className="sd-jobs-sse-notice sd-jobs-sse-notice-warn">
                      ⚠ {sseError}
                    </div>
                  ) : isLiveStreaming ? (
                    <div className="sd-jobs-sse-notice">
                      <span className="sd-jobs-sse-dot" aria-hidden="true" />
                      实时接收日志中…
                    </div>
                  ) : null}
                  {logsTruncated ? (
                    <div className="sd-jobs-sse-notice sd-jobs-sse-notice-warn">
                      当前仅显示最近加载的 1000 行日志，完整分页加载可在后续里程碑继续补齐。
                    </div>
                  ) : null}
                </div>

                {/* 错误信息（failed 任务） */}
                {selectedJob.errorMessage ? (
                  <div className="sd-jobs-error-banner sd-jobs-error-banner-prominent">
                    <span className="sd-jobs-error-label">错误：</span>
                    {selectedJob.errorMessage}
                  </div>
                ) : null}

                {/* 日志区域 */}
                <div ref={logWindowRef} className="sd-jobs-log-window" aria-label="任务日志">
                  {visibleLogs.length === 0 ? (
                    <span className="sd-jobs-log-empty">暂无日志</span>
                  ) : null}
                  {visibleLogs.map((log) => (
                    <div
                      key={`${log.jobId}-${log.sequence}`}
                      className={`sd-jobs-log-line ${log.level}`}
                    >
                      <span className="sd-jobs-log-seq">
                        {String(log.sequence).padStart(3, '0')}
                      </span>
                      <span className="sd-jobs-log-level">{log.level}</span>
                      <span className="sd-jobs-log-msg">{log.message}</span>
                    </div>
                  ))}
                </div>

                {showVNCPortFix ? (
                  <div className="sd-jobs-vnc-fix">
                    <div className="sd-jobs-vnc-fix-text">
                      <strong>VNC 端口被占用</strong>
                      <span>请更换 VNC 端口后重新启动服务器。</span>
                    </div>
                    <button
                      className="sd-btn-tan"
                      type="button"
                      onClick={() => void handleOpenVNCPortModal()}
                    >
                      更换 VNC 端口
                    </button>
                  </div>
                ) : null}
              </>
            ) : (
              <div className="sd-jobs-select-hint">← 从左侧选择一个任务查看详情</div>
            )}
          </div>
        </div>
      )}

      <section className="sd-card sd-command-history" aria-label="最近控制命令">
        <div className="sd-command-history-head">
          <h3>最近控制命令</h3>
          <span>{controlCommands.length} 条</span>
        </div>
        {controlCommandsError ? <div className="sd-jobs-error-banner">{controlCommandsError}</div> : null}
        {controlCommands.length === 0 && !controlCommandsError ? (
          <div className="sd-command-history-empty">暂无控制命令记录。</div>
        ) : (
          <div className="sd-command-history-table-wrap">
            <table className="sd-command-history-table">
              <thead><tr><th>命令</th><th>目标</th><th>提交人</th><th>状态</th><th>提交 / 完成时间</th><th>结构化结果</th></tr></thead>
              <tbody>{controlCommands.map((command) => (
                <tr key={command.commandId}>
                  <td><strong>{COMMAND_TYPE_LABELS[command.commandType] ?? command.commandType}</strong><small title={command.commandId}>{command.commandId.slice(0, 10)}</small></td>
                  <td>{command.targetLabel || command.targetId || '全服'}</td>
                  <td>{command.actorUsername || '未知'}</td>
                  <td><span className={`sd-command-status sd-command-status-${command.status}`}>{command.resultSupported ? (COMMAND_STATUS_LABELS[command.status] ?? command.status) : '已提交（旧模组）'}</span></td>
                  <td><span>{formatDate(command.submittedAt)}</span><small>{command.completedAt ? formatDate(command.completedAt) : '尚未完成'}</small></td>
                  <td>
                    {command.resultSupported ? (
                      <><span>{command.resultMessage || (command.status === 'dispatched' ? '指令已发送，最终效果需结合游戏状态确认。' : '—')}</span>{command.errorCode ? <small>代码：{command.errorCode}</small> : null}{command.resultDetails && Object.keys(command.resultDetails).length ? <small>{Object.entries(command.resultDetails).map(([key, value]) => `${key}: ${value}`).join(' · ')}</small> : null}</>
                    ) : <span>已提交，无法获取精确结果。</span>}
                  </td>
                </tr>
              ))}</tbody>
            </table>
          </div>
        )}
      </section>

      {/* ── 清空确认弹框 ── */}
      {showClearConfirm ? (
        <div className="sd-confirm-overlay">
          <div className="sd-confirm-dialog">
            <h3>清空任务历史</h3>
            <p>确定清空所有任务记录吗？此操作不可撤销。</p>
            <div className="sd-confirm-actions">
              <button
                className="sd-btn-tan"
                type="button"
                onClick={() => setShowClearConfirm(false)}
              >
                取消
              </button>
              <button
                className="sd-btn-delete"
                type="button"
                disabled={busy}
                onClick={() => void handleClearConfirmed()}
              >
                {busy ? '清空中…' : '确认清空'}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {showClearErrorConfirm ? (
        <div className="sd-confirm-overlay">
          <div className="sd-confirm-dialog">
            <h3>清空错误日志</h3>
            <p>确定清空所有任务中的错误日志和错误详情吗？任务记录和任务状态会保留。</p>
            <div className="sd-confirm-actions">
              <button
                className="sd-btn-tan"
                type="button"
                onClick={() => setShowClearErrorConfirm(false)}
              >
                取消
              </button>
              <button
                className="sd-btn-delete"
                type="button"
                disabled={busy}
                onClick={() => void handleClearErrorConfirmed()}
              >
                {busy ? '清空中…' : '确认清空'}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {showVNCPortModal ? (
        <div className="sd-saves-modal-overlay" role="dialog" aria-modal="true">
          <div className="sd-saves-modal-card sd-vnc-port-modal">
            <div className="sd-saves-modal-header">
              <h3 className="sd-saves-modal-title">更换 VNC 端口</h3>
            </div>

            {vncPortLoading ? (
              <div className="sd-jobs-loading">正在读取当前端口...</div>
            ) : (
              <div className="sd-vnc-port-form">
                <label className="sd-vnc-port-field">
                  <span>目前端口号</span>
                  <input className="sd-input" value={currentVNCPort} readOnly />
                </label>
                <label className="sd-vnc-port-field">
                  <span>要更改的端口号</span>
                  <input
                    className="sd-input"
                    inputMode="numeric"
                    pattern="[0-9]*"
                    maxLength={5}
                    value={newVNCPort}
                    onChange={(event) => setNewVNCPort(event.target.value)}
                    placeholder="例如 5801"
                  />
                </label>
                {vncPortError ? <div className="sd-jobs-error-banner">{vncPortError}</div> : null}
                {vncPortMessage ? (
                  <div className="sd-jobs-sse-notice">{vncPortMessage}</div>
                ) : null}
                <div className="sd-saves-modal-actions">
                  <button
                    className="sd-btn-tan"
                    type="button"
                    disabled={vncPortSaving}
                    onClick={handleCloseVNCPortModal}
                  >
                    取消
                  </button>
                  <button
                    className="sd-btn-green"
                    type="button"
                    disabled={vncPortSaving || vncPortLoading}
                    onClick={() => void handleSaveVNCPort()}
                  >
                    {vncPortSaving ? '保存中…' : '保存'}
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      ) : null}
    </div>
  )
}
import './JobsLogsPage.css'
