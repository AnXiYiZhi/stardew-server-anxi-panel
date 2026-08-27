import { useCallback, useEffect, useRef, useState } from 'react'
import {
  createJobEventSource,
  getLatestJobLogs,
  getHealthDiagnostics,
  getInstancePlayers,
  getInviteCode,
  getJobs,
  getMods,
  getSaves,
  getStardewState,
} from '../../api'
import type { HealthDiagnosticsResponse } from '../../api'
import type { InstanceState, Job, JobLog, ModsListResult, PublicIPResult, SavesListResult, StardewPlayersResponse, SteamInviteStatus } from '../../types'
import { errorMessage } from '../../core/helpers'
import type { StardewDashboardData } from './stardew-routes'
import { usePanelUpdate } from './PanelUpdateProvider'
import {
  isCurrentSteamInviteProjection,
  isCurrentSteamInviteRequest,
  preserveSteamInviteProjection,
  shouldPollSteamInvite,
  shouldResumeSteamInviteAfterRuntimeReset,
  shouldRestartSteamInvitePolling,
  steamInviteIsEnabled,
  steamInvitePollBudgetExhausted,
} from './steam-invite-state'

const STEAM_INVITE_POLL_INTERVAL_MS = 5_000
const STEAM_INVITE_POLL_MAX_ATTEMPTS = 125

function resolvePanelAccessHost(): PublicIPResult | null {
  const host = window.location.hostname.trim()
  if (!host) return null
  return {
    ip: host,
    checkedAt: new Date().toISOString(),
    source: 'panel-access-host',
    cached: false,
  }
}

export function useStardewDashboardData(): StardewDashboardData {
  const panelUpdate = usePanelUpdate()
  const [instanceState, setInstanceState] = useState<InstanceState | null>(null)
  const [saves, setSaves] = useState<SavesListResult | null>(null)
  const [mods, setMods] = useState<ModsListResult | null>(null)
  const [players, setPlayers] = useState<StardewPlayersResponse | null>(null)
  const [jobs, setJobs] = useState<Job[]>([])
  const [jobLogsByJobId, setJobLogsByJobId] = useState<Record<string, JobLog[]>>({})
  const [health, setHealth] = useState<HealthDiagnosticsResponse | null>(null)
  const [inviteCode, setInviteCode] = useState<string | null>(null)
  const [inviteCodeStatus, setInviteCodeStatus] = useState<SteamInviteStatus | null>(null)
  const [publicIP, setPublicIP] = useState<PublicIPResult | null>(null)

  const [savesError, setSavesError] = useState<string | null>(null)
  const [modsError, setModsError] = useState<string | null>(null)
  const [playersError, setPlayersError] = useState<string | null>(null)
  const [healthError, setHealthError] = useState<string | null>(null)
  const [inviteCodeError, setInviteCodeError] = useState<string | null>(null)
  const [publicIPError, setPublicIPError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [playersLoading, setPlayersLoading] = useState(false)
  const [inviteCodeLoading, setInviteCodeLoading] = useState(false)
  const [publicIPRefreshing, setPublicIPRefreshing] = useState(false)

  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const playersPollRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const invitePollRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const delayedJobRefreshRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const dashboardMountedRef = useRef(true)
  const instanceStateRef = useRef<InstanceState | null>(null)
  const inviteCodeRef = useRef<string | null>(null)
  const steamInviteEnabledRef = useRef(false)
  const instanceStateRequestGenerationRef = useRef(0)
  const inviteRequestGenerationRef = useRef(0)
  const inviteProjectionGenerationRef = useRef(0)
  const staleInviteCodeRef = useRef<string | null>(null)
  const invitePollAttemptsRef = useRef(0)
  const invitePollBudgetExhaustedRef = useRef(false)
  const invitePollLastStatusRef = useRef<SteamInviteStatus | null>(null)
  const invitePollRuntimeStateRef = useRef<string | null>(null)
  const jobStreamsRef = useRef<Map<string, EventSource>>(new Map())
  const activeSaveNameRef = useRef<string | null>(null)
  const [invitePollRequested, setInvitePollRequested] = useState(false)
  const steamInviteEnabled = steamInviteIsEnabled(instanceState)
  const updateInviteCode = useCallback((value: string | null) => {
    inviteCodeRef.current = value
    setInviteCode(value)
  }, [])

  const refreshInstanceState = useCallback(async () => {
    const stateRequestGeneration = ++instanceStateRequestGenerationRef.current
    const inviteProjectionGeneration = ++inviteProjectionGenerationRef.current
    try {
      const s = await getStardewState()
      if (stateRequestGeneration !== instanceStateRequestGenerationRef.current) return

      const previousRuntimeState = invitePollRuntimeStateRef.current
      const nextRuntimeActive = s.state === 'starting' || s.state === 'running'
      const inviteEnabled = steamInviteIsEnabled(s)
      const shouldResetRuntimeGeneration = shouldRestartSteamInvitePolling(
        previousRuntimeState,
        s.state,
        invitePollLastStatusRef.current,
      )
      const canProjectInvite = isCurrentSteamInviteProjection(
        inviteProjectionGeneration,
        inviteProjectionGenerationRef.current,
      )
      const stateWithCurrentReadyCode = canProjectInvite
        && inviteEnabled
        && s.state === 'running'
        && !s.inviteCode?.trim()
        && invitePollLastStatusRef.current === 'ready'
        && inviteCodeRef.current
        ? { ...s, inviteCode: inviteCodeRef.current }
        : s
      const nextState = canProjectInvite
        ? stateWithCurrentReadyCode
        : preserveSteamInviteProjection(stateWithCurrentReadyCode, instanceStateRef.current)

      instanceStateRef.current = nextState
      invitePollRuntimeStateRef.current = s.state
      setInstanceState(nextState)
      if (shouldResetRuntimeGeneration) {
        // runtime 代际独立于 invite projection：即使本次 /state 的邀请视图已被
        // 更新的 invite 请求取代，starting→running 仍必须获得一份新预算。
        invitePollAttemptsRef.current = 0
        invitePollBudgetExhaustedRef.current = false
        const latestInviteStatus = invitePollLastStatusRef.current
        if (!canProjectInvite && shouldResumeSteamInviteAfterRuntimeReset(
          steamInviteEnabledRef.current,
          inviteCodeRef.current,
          latestInviteStatus,
        )) {
          // 新 invite 响应若已证明终态/ready，上述条件不会命中；这里只恢复被
          // starting 旧预算抢先关闭的、仍为 generating 的新运行代轮询。
          setInviteCodeStatus('generating')
          setInvitePollRequested(true)
        }
      }
      if (!canProjectInvite) return

      steamInviteEnabledRef.current = inviteEnabled

      if (!inviteEnabled) {
        inviteRequestGenerationRef.current += 1
        staleInviteCodeRef.current = null
        invitePollAttemptsRef.current = 0
        invitePollBudgetExhaustedRef.current = false
        invitePollLastStatusRef.current = 'disabled'
        setInviteCodeLoading(false)
        setInvitePollRequested(false)
        updateInviteCode(null)
        setInviteCodeStatus('disabled')
        setInviteCodeError(null)
        return
      }
      const recordedInviteCode = s.inviteCode?.trim() ?? ''
      const stateExposesInviteCode = s.state === 'running'
      if (!stateExposesInviteCode) {
        // starting/stopped 不得继续展示上一运行代的邀请码。
        updateInviteCode(null)
      } else if (!recordedInviteCode) {
        // 直接 invite GET 可能先于 /state 的 payload 持久化返回 ready；这时保留
        // 已被最新请求确认的码，其余状态仍丢弃本地旧值。
        if (invitePollLastStatusRef.current !== 'ready') updateInviteCode(null)
      } else if (recordedInviteCode) {
        if (staleInviteCodeRef.current && recordedInviteCode === staleInviteCodeRef.current) {
          updateInviteCode(null)
        } else {
          inviteRequestGenerationRef.current += 1
          staleInviteCodeRef.current = null
          invitePollAttemptsRef.current = 0
          invitePollBudgetExhaustedRef.current = false
          invitePollLastStatusRef.current = 'ready'
          setInviteCodeLoading(false)
          updateInviteCode(recordedInviteCode)
          setInviteCodeStatus('ready')
          setInviteCodeError(null)
          setInvitePollRequested(false)
          return
        }
      }

      if (!nextRuntimeActive) {
        inviteRequestGenerationRef.current += 1
        invitePollAttemptsRef.current = 0
        invitePollBudgetExhaustedRef.current = false
        setInviteCodeLoading(false)
        setInvitePollRequested(false)
        setInviteCodeError(null)
        if (s.steamInviteAuthState === 'failed') {
          invitePollLastStatusRef.current = 'authorization_failed'
          setInviteCodeStatus('authorization_failed')
        } else if (s.steamInviteAuthState === 'pending') {
          invitePollLastStatusRef.current = 'waiting_authorization'
          setInviteCodeStatus('waiting_authorization')
        } else if (s.steamInviteAuthState === 'authorizing' || s.steamInviteAuthState === 'cleanup_pending') {
          invitePollLastStatusRef.current = 'generating'
          setInviteCodeStatus('generating')
        } else {
          invitePollLastStatusRef.current = 'server_stopped'
          setInviteCodeStatus('server_stopped')
        }
        return
      }

      if (s.steamInviteAuthState !== 'ready') {
        inviteRequestGenerationRef.current += 1
        setInviteCodeLoading(false)
        setInvitePollRequested(false)
        setInviteCodeError(null)
        if (s.steamInviteAuthState === 'failed') {
          invitePollLastStatusRef.current = 'authorization_failed'
          setInviteCodeStatus('authorization_failed')
        } else if (s.steamInviteAuthState === 'pending') {
          invitePollLastStatusRef.current = 'waiting_authorization'
          setInviteCodeStatus('waiting_authorization')
        } else {
          invitePollLastStatusRef.current = 'generating'
          setInviteCodeStatus('generating')
        }
        return
      }

      const lastStatus = invitePollLastStatusRef.current
      const needsInitialPolling = lastStatus === null
        || lastStatus === 'disabled'
        || lastStatus === 'server_stopped'
        || lastStatus === 'waiting_authorization'
        || lastStatus === 'authorization_failed'
      const shouldStartPolling = needsInitialPolling
        || shouldResetRuntimeGeneration
      const shouldStartPollingWithoutCurrentCode = shouldStartPolling && !inviteCodeRef.current
      if (shouldStartPollingWithoutCurrentCode) {
        inviteRequestGenerationRef.current += 1
        invitePollAttemptsRef.current = 0
        invitePollBudgetExhaustedRef.current = false
        invitePollLastStatusRef.current = 'generating'
        setInviteCodeLoading(false)
        setInviteCodeStatus('generating')
        setInviteCodeError(null)
        setInvitePollRequested(true)
      }
    } catch {
      // 保留上次已知状态，不向用户暴露错误
    }
  }, [updateInviteCode])

  const refreshSaves = useCallback(async () => {
    setSavesError(null)
    try {
      const res = await getSaves()
      setSaves(res)
    } catch (e) {
      setSavesError(errorMessage(e))
    }
  }, [])

  const refreshMods = useCallback(async () => {
    setModsError(null)
    try {
      const res = await getMods()
      setMods(res)
    } catch (e) {
      setModsError(errorMessage(e))
    }
  }, [])

  const refreshPlayers = useCallback(async () => {
    setPlayersLoading(true)
    setPlayersError(null)
    try {
      const res = await getInstancePlayers()
      setPlayers(res)
    } catch (e) {
      setPlayersError(errorMessage(e))
    } finally {
      setPlayersLoading(false)
    }
  }, [])

  const refreshJobs = useCallback(async () => {
    try {
      const res = await getJobs()
      setJobs(res.jobs)
    } catch {
      // 保留上次已知任务列表
    }
  }, [])

  const appendJobLogs = useCallback((jobId: string, entries: JobLog[]) => {
    if (entries.length === 0) return
    setJobLogsByJobId((prev) => {
      const current = prev[jobId] ?? []
      const seen = new Set(current.map((entry) => entry.sequence))
      const next = [...current]
      for (const entry of entries) {
        if (seen.has(entry.sequence)) continue
        seen.add(entry.sequence)
        next.push({ ...entry, jobId })
      }
      if (next.length === current.length) return prev
      next.sort((a, b) => a.sequence - b.sequence)
      return { ...prev, [jobId]: next.slice(-200) }
    })
  }, [])

  const refreshHealth = useCallback(async () => {
    setHealthError(null)
    try {
      const res = await getHealthDiagnostics()
      setHealth(res)
    } catch (e) {
      setHealthError(errorMessage(e))
    }
  }, [])

  const applyHealthDiagnostics = useCallback((res: HealthDiagnosticsResponse) => {
    setHealth(res)
    setHealthError(null)
  }, [])

  const refreshInviteCode = useCallback(async () => {
    if (!steamInviteEnabledRef.current) {
      inviteRequestGenerationRef.current += 1
      invitePollBudgetExhaustedRef.current = false
      invitePollLastStatusRef.current = 'disabled'
      updateInviteCode(null)
      setInviteCodeStatus('disabled')
      setInviteCodeError(null)
      setInviteCodeLoading(false)
      setInvitePollRequested(false)
      return
    }
    if (invitePollBudgetExhaustedRef.current) return
    // 后端权威终态保持粘性；手动刷新和新 runtime 会先把 lastStatus 重置为
    // generating，因此仍能显式开启一次新的查询。
    if (invitePollLastStatusRef.current === 'auth_unavailable') return

    const inviteRequestGeneration = ++inviteRequestGenerationRef.current
    const inviteProjectionGeneration = ++inviteProjectionGenerationRef.current
    setInviteCodeLoading(true)
    setInviteCodeError(null)
    try {
      const res = await getInviteCode()
      if (!isCurrentSteamInviteRequest(
        inviteRequestGeneration,
        inviteRequestGenerationRef.current,
        steamInviteEnabledRef.current,
      ) || !isCurrentSteamInviteProjection(
        inviteProjectionGeneration,
        inviteProjectionGenerationRef.current,
      )) return

      if (!res.steamInviteEnabled || res.status === 'disabled') {
        // 该权威 disabled 响应同时使更早的 /state 与 invite 请求失效。
        inviteProjectionGenerationRef.current += 1
        inviteRequestGenerationRef.current += 1
        steamInviteEnabledRef.current = false
        const currentState = instanceStateRef.current
        if (currentState) {
          const disabledState = { ...currentState, steamInviteEnabled: false, inviteCode: '' }
          instanceStateRef.current = disabledState
          setInstanceState(disabledState)
        }
        staleInviteCodeRef.current = null
        invitePollAttemptsRef.current = 0
        invitePollBudgetExhaustedRef.current = false
        invitePollLastStatusRef.current = 'disabled'
        updateInviteCode(null)
        setInviteCodeStatus('disabled')
        setInviteCodeError(null)
        setInviteCodeLoading(false)
        setInvitePollRequested(false)
        return
      }
      // invite 响应已赢得共享 projection；阻止更早开始但更晚返回的 /state
      // 覆盖本次 ready/generating/terminal 结果。
      inviteProjectionGenerationRef.current += 1
      setInviteCodeStatus(res.status)
      invitePollLastStatusRef.current = res.status
      const nextCode = res.inviteCode.trim()
      const staleCode = Boolean(staleInviteCodeRef.current && nextCode === staleInviteCodeRef.current)
      const projectedCode = nextCode && nextCode.toLowerCase() !== 'n/a' && !staleCode ? nextCode : ''
      const currentState = instanceStateRef.current
      steamInviteEnabledRef.current = true
      if (currentState) {
        const enabledState = { ...currentState, steamInviteEnabled: true, inviteCode: projectedCode }
        instanceStateRef.current = enabledState
        setInstanceState(enabledState)
      }
      if (!nextCode || nextCode.toLowerCase() === 'n/a') {
        updateInviteCode(null)
        const shouldContinuePolling = shouldPollSteamInvite(
          instanceStateRef.current,
          true,
          null,
          res.status,
        )
        setInvitePollRequested(shouldContinuePolling)
        return
      }
      if (staleCode) {
        updateInviteCode(null)
        setInviteCodeStatus('generating')
        invitePollLastStatusRef.current = 'generating'
        setInvitePollRequested(true)
        return
      }
      staleInviteCodeRef.current = null
      invitePollAttemptsRef.current = 0
      invitePollBudgetExhaustedRef.current = false
      updateInviteCode(nextCode)
      setInviteCodeStatus('ready')
      setInvitePollRequested(false)
      invitePollLastStatusRef.current = 'ready'
      return
    } catch (e) {
      if (!isCurrentSteamInviteRequest(
        inviteRequestGeneration,
        inviteRequestGenerationRef.current,
        steamInviteEnabledRef.current,
      ) || !isCurrentSteamInviteProjection(
        inviteProjectionGeneration,
        inviteProjectionGenerationRef.current,
      )) return

      inviteProjectionGenerationRef.current += 1
      updateInviteCode(null)
      setInviteCodeError(errorMessage(e))
      const currentState = instanceStateRef.current
      if (currentState) {
        const failedState = { ...currentState, inviteCode: '' }
        instanceStateRef.current = failedState
        setInstanceState(failedState)
      }
      const runtimeMayStillBeWarming = currentState?.state === 'starting'
        || (currentState?.state === 'running' && currentState.steamInviteAuthState === 'ready')
      const failureStatus = runtimeMayStillBeWarming ? 'generating' : 'auth_unavailable'
      setInviteCodeStatus(failureStatus)
      invitePollLastStatusRef.current = failureStatus
      setInvitePollRequested(runtimeMayStillBeWarming)
      return
    } finally {
      if (isCurrentSteamInviteRequest(
        inviteRequestGeneration,
        inviteRequestGenerationRef.current,
        steamInviteEnabledRef.current,
      )) {
        setInviteCodeLoading(false)
      }
    }
  }, [updateInviteCode])

  const refreshPublicIP = useCallback(async (_force = false) => {
    setPublicIPRefreshing(true)
    setPublicIPError(null)
    try {
      const res = resolvePanelAccessHost()
      if (!res) {
        throw new Error('无法读取当前面板访问地址')
      }
      setPublicIP(res)
    } catch (e) {
      setPublicIP(null)
      setPublicIPError(errorMessage(e))
    } finally {
      setPublicIPRefreshing(false)
    }
  }, [])

  const clearInviteCode = useCallback(() => {
    inviteProjectionGenerationRef.current += 1
    inviteRequestGenerationRef.current += 1
    staleInviteCodeRef.current = null
    invitePollAttemptsRef.current = 0
    invitePollBudgetExhaustedRef.current = false
    invitePollLastStatusRef.current = steamInviteEnabledRef.current ? 'server_stopped' : 'disabled'
    setInviteCodeLoading(false)
    setInvitePollRequested(false)
    const currentState = instanceStateRef.current
    if (currentState) {
      const clearedState = { ...currentState, inviteCode: '' }
      instanceStateRef.current = clearedState
      setInstanceState(clearedState)
    }
    updateInviteCode(null)
    setInviteCodeError(null)
    setInviteCodeStatus(invitePollLastStatusRef.current)
  }, [updateInviteCode])

  const requestInviteCodeRefresh = useCallback(() => {
    if (!steamInviteEnabledRef.current) return
    inviteProjectionGenerationRef.current += 1
    inviteRequestGenerationRef.current += 1
    staleInviteCodeRef.current = inviteCodeRef.current
    invitePollAttemptsRef.current = 0
    invitePollBudgetExhaustedRef.current = false
    invitePollLastStatusRef.current = 'generating'
    setInviteCodeLoading(false)
    setInvitePollRequested(true)
    const currentState = instanceStateRef.current
    if (currentState) {
      const refreshingState = { ...currentState, inviteCode: '' }
      instanceStateRef.current = refreshingState
      setInstanceState(refreshingState)
    }
    updateInviteCode(null)
    setInviteCodeStatus('generating')
    setInviteCodeError(null)
  }, [updateInviteCode])

  const refreshAll = useCallback(() => {
    void (async () => {
      await refreshInstanceState()
      if (steamInviteEnabledRef.current) await refreshInviteCode()
    })()
    void refreshSaves()
    void refreshMods()
    void refreshPlayers()
    void refreshJobs()
    void refreshPublicIP()
  }, [
    refreshInstanceState,
    refreshSaves,
    refreshMods,
    refreshPlayers,
    refreshJobs,
    refreshInviteCode,
    refreshPublicIP,
  ])

  const refreshAfterJobFinished = useCallback(() => {
    void refreshJobs()
    void (async () => {
      await refreshInstanceState()
      if (dashboardMountedRef.current
        && document.visibilityState === 'visible'
        && steamInviteEnabledRef.current) {
        await refreshInviteCode()
      }
    })()
    void refreshSaves()
    void refreshMods()
    if (document.visibilityState === 'visible') {
      void refreshPlayers()
    }
    if (delayedJobRefreshRef.current !== null) clearTimeout(delayedJobRefreshRef.current)
    delayedJobRefreshRef.current = window.setTimeout(() => {
      delayedJobRefreshRef.current = null
      if (!dashboardMountedRef.current) return
      void (async () => {
        await refreshInstanceState()
        if (dashboardMountedRef.current
          && document.visibilityState === 'visible'
          && steamInviteEnabledRef.current) {
          await refreshInviteCode()
        }
      })()
      if (document.visibilityState === 'visible') {
        void refreshPlayers()
      }
    }, 1000)
  }, [refreshInstanceState, refreshInviteCode, refreshJobs, refreshMods, refreshPlayers, refreshSaves])

  useEffect(() => {
    dashboardMountedRef.current = true
    const init = async () => {
      setLoading(true)
      // 并发加载所有数据，单个失败不阻塞其他
      await Promise.allSettled([
        refreshInstanceState(),
        refreshSaves(),
        refreshMods(),
        refreshPlayers(),
        refreshJobs(),
        refreshPublicIP(),
      ])
      setLoading(false)
    }
    void init()

    // 每 30s 轮询实例状态和任务列表（任务列表兜底调度器触发的 job，SSE 只覆盖已知任务）
    pollRef.current = setInterval(() => {
      void refreshInstanceState()
      void refreshJobs()
    }, 30_000)

    return () => {
      dashboardMountedRef.current = false
      instanceStateRequestGenerationRef.current += 1
      inviteRequestGenerationRef.current += 1
      inviteProjectionGenerationRef.current += 1
      if (pollRef.current !== null) clearInterval(pollRef.current)
      if (playersPollRef.current !== null) clearTimeout(playersPollRef.current)
      if (invitePollRef.current !== null) clearTimeout(invitePollRef.current)
      if (delayedJobRefreshRef.current !== null) clearTimeout(delayedJobRefreshRef.current)
      for (const es of jobStreamsRef.current.values()) {
        es.close()
      }
      jobStreamsRef.current.clear()
    }
  }, [
    refreshInstanceState,
    refreshSaves,
    refreshMods,
    refreshPlayers,
    refreshJobs,
    refreshPublicIP,
  ])

  useEffect(() => {
    const recovered = () => refreshAll()
    window.addEventListener('panel-update-recovered', recovered)
    return () => window.removeEventListener('panel-update-recovered', recovered)
  }, [refreshAll])

  useEffect(() => {
    const activeJobIds = new Set(
      jobs
        .filter((job) => job.status === 'queued' || job.status === 'running')
        .map((job) => job.id),
    )

    for (const [jobId, es] of jobStreamsRef.current) {
      if (!activeJobIds.has(jobId)) {
        es.close()
        jobStreamsRef.current.delete(jobId)
      }
    }

    for (const jobId of activeJobIds) {
      if (jobStreamsRef.current.has(jobId)) continue
      void getLatestJobLogs(jobId, 200)
        .then((res) => appendJobLogs(jobId, res.logs))
        .catch(() => {
          // 实时流仍会继续写入后续日志；初始日志拉取失败不阻塞右栏显示任务。
        })
      const es = createJobEventSource(jobId)
      jobStreamsRef.current.set(jobId, es)
      es.addEventListener('log', (ev) => {
        try {
          const entry = JSON.parse((ev as MessageEvent<string>).data) as JobLog
          appendJobLogs(jobId, [entry])
        } catch {
          // Ignore malformed SSE payloads; the full job page remains the source of truth.
        }
      })
      es.addEventListener('finished', () => {
        es.close()
        jobStreamsRef.current.delete(jobId)
        refreshAfterJobFinished()
      })
      es.onerror = () => {
        es.close()
        jobStreamsRef.current.delete(jobId)
        void refreshJobs()
        void refreshInstanceState()
      }
    }
  }, [appendJobLogs, jobs, refreshAfterJobFinished, refreshInstanceState, refreshJobs])

  useEffect(() => {
    if (!instanceState?.state) return
    if (!steamInviteEnabled) {
      updateInviteCode(null)
      setInviteCodeStatus('disabled')
      setInviteCodeError(null)
      setInvitePollRequested(false)
    } else if (document.visibilityState === 'visible') {
      void refreshInviteCode()
    }
    if (instanceState.state === 'running' || instanceState.state === 'starting') {
      if (document.visibilityState === 'visible') {
        if (instanceState.state === 'running') void refreshPlayers()
      }
      return
    }
    updateInviteCode(null)
    if (steamInviteEnabled) setInviteCodeStatus('server_stopped')
    setInviteCodeError(null)
    // 服务器一旦不再是 running，就把在线玩家列表清空，避免下一次启动时
    // ServerControlPage 用上一轮运行时残留的"主机在线"快照误判为已就绪——
    // refreshPlayers() 请求可能因为容器还没起来而失败，失败分支不会清空
    // players，如果不在这里主动清空，旧快照会一直挂着直到请求成功为止。
    setPlayers(null)
    if (document.visibilityState === 'visible') {
      void refreshPlayers()
    }
    setPlayersError(null)
  }, [instanceState?.state, refreshInviteCode, refreshPlayers, steamInviteEnabled, updateInviteCode])

  useEffect(() => {
    const activeSaveName = saves?.activeSaveName ?? ''
    if (activeSaveNameRef.current === null) {
      activeSaveNameRef.current = activeSaveName
      return
    }
    if (activeSaveNameRef.current === activeSaveName) return
    activeSaveNameRef.current = activeSaveName
    void refreshMods()
  }, [saves?.activeSaveName, refreshMods])

  useEffect(() => {
    if (playersPollRef.current !== null) {
      clearTimeout(playersPollRef.current)
      playersPollRef.current = null
    }
    if (instanceState?.state !== 'running') return

    let cancelled = false
    const pollPlayers = async () => {
      if (document.visibilityState !== 'visible') return
      await refreshPlayers()
      if (cancelled) return
      playersPollRef.current = window.setTimeout(() => {
        void pollPlayers()
      }, 5_000)
    }
    const schedulePlayers = () => {
      if (cancelled || document.visibilityState !== 'visible') return
      playersPollRef.current = window.setTimeout(() => {
        void pollPlayers()
      }, 5_000)
    }
    const handleVisibilityChange = () => {
      if (document.visibilityState !== 'visible') {
        if (playersPollRef.current !== null) {
          clearTimeout(playersPollRef.current)
          playersPollRef.current = null
        }
        return
      }
      schedulePlayers()
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)
    schedulePlayers()
    return () => {
      cancelled = true
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      if (playersPollRef.current !== null) {
        clearTimeout(playersPollRef.current)
        playersPollRef.current = null
      }
    }
  }, [instanceState?.state, refreshPlayers])

  useEffect(() => {
    if (invitePollRef.current !== null) {
      clearTimeout(invitePollRef.current)
      invitePollRef.current = null
    }

    const shouldPollInvite = shouldPollSteamInvite(
      instanceState,
      invitePollRequested,
      inviteCode,
      inviteCodeStatus,
    )
    if (!shouldPollInvite) return

    let cancelled = false
    const pollInviteCode = async () => {
      if (document.visibilityState !== 'visible') return
      await refreshInstanceState()
      if (cancelled || !steamInviteEnabledRef.current) return
      const currentRuntimeState = instanceStateRef.current?.state
      if (currentRuntimeState !== 'starting' && currentRuntimeState !== 'running') return
      if (steamInvitePollBudgetExhausted(invitePollAttemptsRef.current, STEAM_INVITE_POLL_MAX_ATTEMPTS)) {
        invitePollBudgetExhaustedRef.current = true
        if (inviteCodeStatus === null || inviteCodeStatus === 'generating' || inviteCodeStatus === 'ready') {
          // 这里只改变展示终态，不覆盖最后一次请求状态：若 runtime 随后从
          // starting 进入 running，后端 generating 证据仍会开启新的运行代预算。
          setInviteCodeStatus('auth_unavailable')
        }
        setInvitePollRequested(false)
        return
      }
      invitePollAttemptsRef.current += 1
      await refreshInviteCode()
      const pollStatus = invitePollLastStatusRef.current
      if (cancelled) return
      if (steamInvitePollBudgetExhausted(invitePollAttemptsRef.current, STEAM_INVITE_POLL_MAX_ATTEMPTS)) {
        if (pollStatus === null || pollStatus === 'generating') {
          invitePollBudgetExhaustedRef.current = true
          // 本地预算终态与后端权威 auth_unavailable 分开保存，避免前者阻断
          // starting → running 后的新一轮暖机查询。
          setInviteCodeStatus('auth_unavailable')
          setInvitePollRequested(false)
        }
        return
      }
      if (pollStatus !== null && pollStatus !== 'generating') return
      invitePollRef.current = window.setTimeout(() => {
        void pollInviteCode()
      }, STEAM_INVITE_POLL_INTERVAL_MS)
    }

    const scheduleInvite = () => {
      if (cancelled || document.visibilityState !== 'visible') return
      invitePollRef.current = window.setTimeout(() => {
        void pollInviteCode()
      }, STEAM_INVITE_POLL_INTERVAL_MS)
    }
    const handleVisibilityChange = () => {
      if (document.visibilityState !== 'visible') {
        if (invitePollRef.current !== null) {
          clearTimeout(invitePollRef.current)
          invitePollRef.current = null
        }
        return
      }
      scheduleInvite()
    }
    document.addEventListener('visibilitychange', handleVisibilityChange)
    scheduleInvite()

    return () => {
      cancelled = true
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      if (invitePollRef.current !== null) {
        clearTimeout(invitePollRef.current)
        invitePollRef.current = null
      }
    }
  }, [
    instanceState?.state,
    steamInviteEnabled,
    inviteCode,
    inviteCodeStatus,
    invitePollRequested,
    refreshInstanceState,
    refreshInviteCode,
  ])

  return {
    ...panelUpdate,
    instanceState,
    saves,
    mods,
    players,
    jobs,
    jobLogsByJobId,
    health,
    inviteCode,
    inviteCodeStatus,
    publicIP,
    savesError,
    modsError,
    playersError,
    healthError,
    inviteCodeError,
    publicIPError,
    loading,
    playersLoading,
    inviteCodeRefreshing: steamInviteEnabled && (invitePollRequested || inviteCodeLoading),
    publicIPRefreshing,
    refreshAll,
    refreshInstanceState,
    refreshSaves,
    refreshMods,
    refreshPlayers,
    refreshJobs,
    refreshHealth,
    applyHealthDiagnostics,
    refreshInviteCode,
    refreshPublicIP,
    clearInviteCode,
    requestInviteCodeRefresh,
  }
}
