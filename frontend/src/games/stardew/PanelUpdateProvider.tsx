import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import {
  ApiError,
  applyPanelUpdate,
  checkPanelUpdate,
  getPanelUpdateApplyStatus,
  getPanelUpdateDryRunStatus,
  getPanelUpdateStatus,
  getVersion,
  runPanelUpdateDryRun,
} from '../../api'
import type { PanelUpdateApplyStatus, PanelUpdateDryRunStatus, PanelUpdateStatus, VersionInfo } from '../../api'
import type { CurrentUser } from '../../types'
import { errorMessage } from '../../core/helpers'
import { isPanelUpdateActive, isPanelUpdateTerminal, panelUpdatePhaseLabel, reconnectDelay } from './panel-update-machine'
import './PanelUpdateProvider.css'

type ReconnectMode = 'idle' | 'offline' | 'timeout'

export type PanelUpdateContextValue = {
  versionInfo: VersionInfo | null
  updateStatus: PanelUpdateStatus | null
  updateDryRun: PanelUpdateDryRunStatus | null
  updateApply: PanelUpdateApplyStatus | null
  updateError: string | null
  updateDryRunError: string | null
  updateApplyError: string | null
  updateChecking: boolean
  updateDryRunChecking: boolean
  updateApplyStarting: boolean
  updateDialogOpen: boolean
  reconnectMode: ReconnectMode
  reconnectAttempts: number
  refreshUpdateStatus: (manual?: boolean) => Promise<void>
  runUpdateDryRun: (targetVersion: string) => Promise<void>
  applyUpdate: () => Promise<void>
  openUpdateDialog: () => void
  closeUpdateDialog: () => void
  continueReconnect: () => void
}

const PanelUpdateContext = createContext<PanelUpdateContextValue | null>(null)
const RECONNECT_TIMEOUT_MS = 180_000

async function probePanel(expectedVersion: string): Promise<boolean> {
  const controller = new AbortController()
  const timeout = window.setTimeout(() => controller.abort(), 5_000)
  try {
    const [healthResponse, versionResponse] = await Promise.all([
      fetch('/health', { cache: 'no-store', credentials: 'include', signal: controller.signal }),
      fetch('/api/version', { cache: 'no-store', credentials: 'include', signal: controller.signal }),
    ])
    if (!healthResponse.ok || !versionResponse.ok) return false
    const health = await healthResponse.json() as { status?: string }
    const version = await versionResponse.json() as VersionInfo
    const normalizedExpected = expectedVersion.replace(/^[vV]/, '')
    const normalizedActual = version.version.replace(/^[vV]/, '')
    return health.status === 'ok' && Boolean(normalizedActual) && (
      !normalizedExpected || normalizedActual === normalizedExpected
    )
  } catch {
    return false
  } finally {
    window.clearTimeout(timeout)
  }
}

export function PanelUpdateProvider({ user, children }: { user: CurrentUser; children: ReactNode }) {
  const isAdmin = user.role === 'admin'
  const [versionInfo, setVersionInfo] = useState<VersionInfo | null>(null)
  const [updateStatus, setUpdateStatus] = useState<PanelUpdateStatus | null>(null)
  const [updateDryRun, setUpdateDryRun] = useState<PanelUpdateDryRunStatus | null>(null)
  const [updateApply, setUpdateApply] = useState<PanelUpdateApplyStatus | null>(null)
  const [updateError, setUpdateError] = useState<string | null>(null)
  const [updateDryRunError, setUpdateDryRunError] = useState<string | null>(null)
  const [updateApplyError, setUpdateApplyError] = useState<string | null>(null)
  const [updateChecking, setUpdateChecking] = useState(false)
  const [updateDryRunChecking, setUpdateDryRunChecking] = useState(false)
  const [updateApplyStarting, setUpdateApplyStarting] = useState(false)
  const [updateDialogOpen, setUpdateDialogOpen] = useState(false)
  const [reconnectMode, setReconnectMode] = useState<ReconnectMode>('idle')
  const [reconnectAttempts, setReconnectAttempts] = useState(0)
  const [reconnectGeneration, setReconnectGeneration] = useState(0)
  const reconnectStartedRef = useRef(0)
  const reconnectGenerationRef = useRef(0)

  const refreshUpdateStatus = useCallback(async (manual = false) => {
    if (manual && !isAdmin) return
    if (manual) setUpdateChecking(true)
    setUpdateError(null)
    try {
      setUpdateStatus(manual ? await checkPanelUpdate() : await getPanelUpdateStatus())
    } catch (error) {
      setUpdateError(errorMessage(error))
    } finally {
      if (manual) setUpdateChecking(false)
    }
  }, [isAdmin])

  const runUpdateDryRun = useCallback(async (targetVersion: string) => {
    if (!isAdmin) return
    setUpdateDryRunChecking(true)
    setUpdateDryRunError(null)
    try {
      setUpdateDryRun(await runPanelUpdateDryRun(targetVersion))
    } catch (error) {
      setUpdateDryRunError(errorMessage(error))
    } finally {
      setUpdateDryRunChecking(false)
    }
  }, [isAdmin])

  const applyUpdate = useCallback(async () => {
    if (!isAdmin) return
    setUpdateApplyStarting(true)
    setUpdateApplyError(null)
    try {
      const apply = await applyPanelUpdate()
      setUpdateApply(apply)
      reconnectStartedRef.current = Date.now()
      setReconnectAttempts(0)
    } catch (error) {
      if (error instanceof ApiError) {
        setUpdateApplyError(errorMessage(error))
      } else {
        const now = new Date().toISOString()
        setUpdateApply({
          updateId: 'pending-response', phase: 'checking', progress: 5,
          fromVersion: updateStatus?.currentVersion || versionInfo?.version || '',
          toVersion: updateStatus?.latestVersion || '',
          originalImage: '', originalDigest: '', selectedImage: '', selectedDigest: '',
          errorCode: '', error: '', result: '', logs: [], startedAt: now, updatedAt: now, finishedAt: null,
        })
        reconnectStartedRef.current = Date.now()
        setReconnectMode('offline')
      }
    } finally {
      setUpdateApplyStarting(false)
    }
  }, [isAdmin, updateStatus, versionInfo])

  const continueReconnect = useCallback(() => {
    reconnectStartedRef.current = Date.now()
    reconnectGenerationRef.current += 1
    setReconnectGeneration((value) => value + 1)
    setReconnectAttempts(0)
    setReconnectMode('offline')
  }, [])

  useEffect(() => {
    let cancelled = false
    void Promise.allSettled([
      getVersion().then((value) => { if (!cancelled) setVersionInfo(value) }),
      getPanelUpdateStatus().then((value) => { if (!cancelled) setUpdateStatus(value) }),
      ...(isAdmin ? [
        getPanelUpdateApplyStatus().then((value) => { if (!cancelled) setUpdateApply(value) }).catch((error) => {
          if (!(error instanceof ApiError && error.status === 404) && !cancelled) setUpdateApplyError(errorMessage(error))
        }),
        getPanelUpdateDryRunStatus().then((value) => { if (!cancelled) setUpdateDryRun(value) }).catch((error) => {
          if (!(error instanceof ApiError && error.status === 404) && !cancelled) setUpdateDryRunError(errorMessage(error))
        }),
      ] : []),
    ])
    return () => { cancelled = true }
  }, [isAdmin])

  useEffect(() => {
    const waiting = !updateStatus || updateStatus.checkStatus === 'pending' || updateStatus.checkStatus === 'checking'
    const timer = window.setTimeout(() => void refreshUpdateStatus(), waiting ? 5_000 : 15 * 60_000)
    return () => window.clearTimeout(timer)
  }, [refreshUpdateStatus, updateStatus])

  useEffect(() => {
    if (!isAdmin || (updateDryRun?.phase !== 'starting' && updateDryRun?.phase !== 'running')) return
    const timer = window.setTimeout(() => {
      void getPanelUpdateDryRunStatus().then((value) => {
        setUpdateDryRun(value)
        setUpdateDryRunError(null)
      }).catch((error) => setUpdateDryRunError(errorMessage(error)))
    }, 2_000)
    return () => window.clearTimeout(timer)
  }, [isAdmin, updateDryRun])

  useEffect(() => {
    if (!isAdmin || !isPanelUpdateActive(updateApply)) return
    let cancelled = false
    let timer = 0
    const generation = reconnectGenerationRef.current

    const finish = async (status: PanelUpdateApplyStatus) => {
      setUpdateApply(status)
      setReconnectMode('idle')
      setReconnectAttempts(0)
      setUpdateDialogOpen(true)
      const [versionResult, updateResult] = await Promise.allSettled([getVersion(), getPanelUpdateStatus()])
      if (versionResult.status === 'fulfilled') setVersionInfo(versionResult.value)
      if (updateResult.status === 'fulfilled') setUpdateStatus(updateResult.value)
      window.dispatchEvent(new CustomEvent('panel-update-recovered'))
    }

    const poll = async (attempt: number) => {
      if (cancelled || generation !== reconnectGenerationRef.current) return
      try {
        const status = await getPanelUpdateApplyStatus()
        if (cancelled) return
        setUpdateApply(status)
        setUpdateApplyError(null)
        if (isPanelUpdateTerminal(status)) {
          await finish(status)
          return
        }
        timer = window.setTimeout(() => void poll(0), 1_500)
      } catch (error) {
        if (cancelled) return
        if (error instanceof ApiError && error.status === 404 && updateApply?.updateId === 'pending-response') {
          if (attempt >= 3) {
            setUpdateApply(null)
            setReconnectMode('idle')
            setUpdateApplyError('未确认升级任务是否创建成功，请重新打开版本详情检查后再试。')
            return
          }
          timer = window.setTimeout(() => void poll(attempt + 1), 1_500)
          return
        }
        if (!reconnectStartedRef.current) reconnectStartedRef.current = Date.now()
        const elapsed = Date.now() - reconnectStartedRef.current
        setReconnectMode(elapsed >= RECONNECT_TIMEOUT_MS ? 'timeout' : 'offline')
        setReconnectAttempts(attempt + 1)
        if (elapsed >= RECONNECT_TIMEOUT_MS) return

        const expected = updateApply?.toVersion || updateStatus?.latestVersion || ''
        if (await probePanel(expected)) {
          try {
            const status = await getPanelUpdateApplyStatus()
            if (isPanelUpdateTerminal(status)) {
              await finish(status)
              return
            }
            setUpdateApply(status)
          } catch {
            // 新进程可能已经健康，但 helper 尚未写入终态；继续退避读取。
          }
        }
        timer = window.setTimeout(() => void poll(attempt + 1), reconnectDelay(attempt))
      }
    }
    void poll(0)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [isAdmin, reconnectGeneration, updateApply?.phase, updateApply?.toVersion, updateStatus?.latestVersion])

  const value = useMemo<PanelUpdateContextValue>(() => ({
    versionInfo, updateStatus, updateDryRun, updateApply,
    updateError, updateDryRunError, updateApplyError,
    updateChecking, updateDryRunChecking, updateApplyStarting,
    updateDialogOpen, reconnectMode, reconnectAttempts,
    refreshUpdateStatus, runUpdateDryRun, applyUpdate,
    openUpdateDialog: () => setUpdateDialogOpen(true),
    closeUpdateDialog: () => setUpdateDialogOpen(false),
    continueReconnect,
  }), [
    versionInfo, updateStatus, updateDryRun, updateApply,
    updateError, updateDryRunError, updateApplyError,
    updateChecking, updateDryRunChecking, updateApplyStarting,
    updateDialogOpen, reconnectMode, reconnectAttempts,
    refreshUpdateStatus, runUpdateDryRun, applyUpdate, continueReconnect,
  ])

	const overlayPhase = panelUpdatePhaseLabel(updateApply?.fullStack?.phase ?? updateApply?.phase ?? '')
  return (
    <PanelUpdateContext.Provider value={value}>
      {children}
      {reconnectMode !== 'idle' ? (
        <div className="sd-panel-update-reconnect" role="alert" aria-live="assertive">
          <div className="sd-panel-update-reconnect-card">
            <div className="sd-panel-update-reconnect-sprite" aria-hidden="true">🌱</div>
            <p className="sd-panel-update-reconnect-kicker">PANEL UPDATE</p>
            <h1>{reconnectMode === 'timeout' ? '面板恢复时间比预期更长' : '面板正在升级'}</h1>
            <p className="sd-panel-update-reconnect-phase">{overlayPhase}</p>
            {reconnectMode === 'timeout' ? (
			  <p>全栈升级恢复时间比预期更长。游戏实例可能正在保存、备份、重启或回滚；请勿手动启动旧 Control。你可以继续等待，或稍后重新打开面板查看持久化状态。</p>
            ) : (
			  <p>Panel 会短暂离线；新 Panel 恢复后将继续校验全部实例，必要时保存、整档备份并重启游戏服务器。请不要重复点击或手动重启容器。</p>
            )}
            <div className="sd-panel-update-reconnect-progress" aria-hidden="true"><span /></div>
            <p className="sd-panel-update-reconnect-attempt">已自动尝试连接 {reconnectAttempts} 次</p>
            {reconnectMode === 'timeout' ? (
              <button type="button" onClick={continueReconnect}>继续自动等待</button>
            ) : null}
          </div>
        </div>
      ) : null}
    </PanelUpdateContext.Provider>
  )
}

export function usePanelUpdate(): PanelUpdateContextValue {
  const value = useContext(PanelUpdateContext)
  if (!value) throw new Error('usePanelUpdate must be used inside PanelUpdateProvider')
  return value
}
