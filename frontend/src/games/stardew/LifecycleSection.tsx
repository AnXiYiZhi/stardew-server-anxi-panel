import { useState } from 'react'
import { ApiError } from '../../api'
import { startInstance, stopInstance, restartInstance, getInviteCode } from '../../api'
import { errorMessage, stateLabel } from '../../core/helpers'

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
      if (error instanceof ApiError && (error.code === 'save_required' || error.code === 'active_save_required' || error.code === 'active_save_missing')) {
        if (error.code === 'save_required') {
          setMessage('没有可用存档。请在下方存档管理区域创建或上传存档。')
        } else if (error.code === 'active_save_required') {
          setMessage('没有已选择的启动存档，请先创建、上传或选择一个存档。')
        } else {
          setMessage('上次选择的存档不存在，请重新选择存档。')
        }
        document.getElementById('saves-section')?.scrollIntoView({ behavior: 'smooth', block: 'center' })
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

  return (
    <section className="lifecycle-section">
      <div className="section-heading">
        <div>
          <h2>服务器控制</h2>
          <p>启动、停止、重启 Stardew Junimo 服务器。</p>
        </div>
      </div>

      {message ? <div className="error-banner">{message}</div> : null}

      <div className="lifecycle-state">
        <span className="lifecycle-state-label">当前状态：</span>
        <span className={`lifecycle-state-badge lifecycle-state-${state}`}>{stateLabel(state)}</span>
      </div>

      {inviteCode ? (
        <div className="invite-code-display">
          <span>邀请码：</span>
          <strong className="invite-code">{inviteCode}</strong>
        </div>
      ) : null}

      {isAdmin ? (
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
    </section>
  )
}
