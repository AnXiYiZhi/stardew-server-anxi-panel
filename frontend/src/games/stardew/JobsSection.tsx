import type { CurrentUser, Job, JobLog } from '../../types'
import { StatusBadge } from '../../core/StatusBadge'
import { shortJobID, formatDate } from '../../core/helpers'

const pullProgressRe = /^\[pull:progress:(\d+):(\d+)\]$/

function extractPullProgress(logs: JobLog[], jobType: string | undefined): { done: number; total: number; percent: number } | null {
  if (jobType !== 'stardew_install') return null
  let latest: { done: number; total: number } | null = null
  for (const log of logs) {
    const m = log.message.match(pullProgressRe)
    if (m) latest = { done: parseInt(m[1], 10), total: parseInt(m[2], 10) }
  }
  if (!latest || latest.total === 0) return null
  return { ...latest, percent: Math.round((latest.done / latest.total) * 100) }
}

export function JobsSection({
  user,
  jobs,
  selectedJob,
  logs,
  busy,
  message,
  onRefresh,
  onSelectJob,
  onRunTestJob,
  onRunFailingTestJob,
  onClearJobs,
}: {
  user: CurrentUser
  jobs: Job[]
  selectedJob: Job | null
  logs: JobLog[]
  busy: boolean
  message: string
  onRefresh: () => void
  onSelectJob: (job: Job) => void
  onRunTestJob: () => void
  onRunFailingTestJob: () => void
  onClearJobs: () => void
}) {
  return (
    <section className="jobs-section">
      <div className="section-heading">
        <div>
          <h2>任务中心</h2>
          <p>安装任务、失败原因和实时日志都在这里显示。</p>
        </div>
        <div className="job-actions">
          <button className="button button-small button-secondary" disabled={busy} onClick={onRefresh} type="button">刷新任务</button>
          {user.role === 'admin' && import.meta.env.VITE_SHOW_DEV_TOOLS === 'true' ? (
            <>
              <button className="button button-small" disabled={busy} onClick={onRunTestJob} type="button">启动测试任务</button>
              <button className="button button-small button-danger" disabled={busy} onClick={onRunFailingTestJob} type="button">启动失败测试任务</button>
            </>
          ) : null}
          {user.role === 'admin' ? (
            <button className="button button-small button-danger" disabled={busy || jobs.length === 0} onClick={onClearJobs} type="button">清空任务中心</button>
          ) : null}
        </div>
      </div>
      {user.role !== 'admin' ? <p className="form-hint">普通用户只能查看自己有权限的任务，不能创建测试任务。</p> : null}
      {message ? <div className="error-banner docker-error">{message}</div> : null}
      <div className="jobs-layout">
        <div className="jobs-list" role="table" aria-label="最近任务列表">
          <div className="job-row job-row-head" role="row">
            <span>ID</span><span>类型</span><span>状态</span><span>创建</span>
          </div>
          {jobs.length === 0 ? <p className="summary compact">暂无任务。</p> : null}
          {jobs.map((job) => (
            <button
              className={selectedJob?.id === job.id ? 'job-row selected' : 'job-row'}
              key={job.id}
              onClick={() => onSelectJob(job)}
              type="button"
            >
              <span title={job.id}>{shortJobID(job.id)}</span>
              <span>{job.type}</span>
              <StatusBadge status={job.status} />
              <span>{formatDate(job.createdAt)}</span>
            </button>
          ))}
        </div>
        <div className="job-detail">
          {selectedJob ? (
            <>
              <div className="job-detail-head">
                <div>
                  <h3>{selectedJob.type}</h3>
                  <p>{selectedJob.id}</p>
                </div>
                <StatusBadge status={selectedJob.status} />
              </div>
              <p>
                创建：{formatDate(selectedJob.createdAt)}；完成：
                {selectedJob.finishedAt ? formatDate(selectedJob.finishedAt) : '尚未完成'}
              </p>
              {selectedJob.errorMessage ? <div className="error-banner docker-error">{selectedJob.errorMessage}</div> : null}
              {(() => {
                const pp = extractPullProgress(logs, selectedJob.type)
                if (!pp) return null
                return (
                  <div className="pull-progress-container">
                    <div className="pull-progress-header">
                      <span>拉取镜像</span>
                      <span>{pp.done}/{pp.total} 服务</span>
                    </div>
                    <div className="progress-bar-wrap">
                      <div className="progress-bar-track">
                        <div
                          className={`progress-bar-fill${pp.done === pp.total ? ' done' : ''}`}
                          style={{ width: `${pp.percent}%` }}
                        />
                      </div>
                      <span className="progress-bar-percent">{pp.percent}%</span>
                    </div>
                  </div>
                )
              })()}
              <div className="job-log-window" aria-label="任务日志">
                {logs.length === 0 ? <p>暂无日志。</p> : null}
                {logs.filter((log) => !pullProgressRe.test(log.message)).map((log) => (
                  <div className={`job-log-line ${log.level}`} key={`${log.jobId}-${log.sequence}`}>
                    <span>{String(log.sequence).padStart(3, '0')}</span>
                    <strong>{log.level}</strong>
                    <p>{log.message}</p>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <p className="summary compact">选择一个任务查看详情和日志。</p>
          )}
        </div>
      </div>
    </section>
  )
}
