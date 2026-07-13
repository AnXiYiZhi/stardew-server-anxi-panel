import { useCallback, useEffect, useRef, useState } from 'react'
import {
  createJobEventSource,
  getJobLogs,
  getHealthDiagnostics,
  getInstancePlayers,
  getInviteCode,
  getJobs,
  getMods,
  getSaves,
  getStardewState,
} from '../../api'
import type { HealthDiagnosticsResponse } from '../../api'
import type { InstanceState, Job, JobLog, ModsListResult, PublicIPResult, SavesListResult, StardewPlayersResponse } from '../../types'
import { errorMessage } from '../../core/helpers'
import type { StardewDashboardData } from './stardew-routes'
import { usePanelUpdate } from './PanelUpdateProvider'

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
  const [publicIP, setPublicIP] = useState<PublicIPResult | null>(null)

  const [savesError, setSavesError] = useState<string | null>(null)
  const [modsError, setModsError] = useState<string | null>(null)
  const [playersError, setPlayersError] = useState<string | null>(null)
  const [healthError, setHealthError] = useState<string | null>(null)
  const [inviteCodeError, setInviteCodeError] = useState<string | null>(null)
  const [publicIPError, setPublicIPError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [playersLoading, setPlayersLoading] = useState(false)
  const [publicIPRefreshing, setPublicIPRefreshing] = useState(false)

  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const playersPollRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const invitePollRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const staleInviteCodeRef = useRef<string | null>(null)
  const invitePollAttemptsRef = useRef(0)
  const jobStreamsRef = useRef<Map<string, EventSource>>(new Map())
  const activeSaveNameRef = useRef<string | null>(null)
  const [invitePollRequested, setInvitePollRequested] = useState(false)

  const refreshInstanceState = useCallback(async () => {
    try {
      const s = await getStardewState()
      setInstanceState(s)
      const recordedInviteCode = s.inviteCode?.trim() ?? ''
      const stateExposesInviteCode = s.state === 'running' || s.state === 'starting'
      if (!stateExposesInviteCode) {
        // 服务器未运行时，后端为了保留历史元数据不会清空 invite_code 字段，
        // 这里必须主动丢弃，否则每次轮询都会把停止前的旧邀请码重新展示出来。
        setInviteCode(null)
      } else if (recordedInviteCode) {
        if (staleInviteCodeRef.current && recordedInviteCode === staleInviteCodeRef.current) {
          setInviteCode(null)
        } else {
          staleInviteCodeRef.current = null
          invitePollAttemptsRef.current = 0
          setInviteCode(recordedInviteCode)
          setInviteCodeError(null)
          setInvitePollRequested(false)
        }
      }
    } catch {
      // 保留上次已知状态，不向用户暴露错误
    }
  }, [])

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
    setInviteCodeError(null)
    try {
      const res = await getInviteCode()
      if (staleInviteCodeRef.current && res.inviteCode === staleInviteCodeRef.current) {
        setInviteCode(null)
        return
      }
      staleInviteCodeRef.current = null
      setInviteCode(res.inviteCode)
      setInvitePollRequested(false)
    } catch (e) {
      setInviteCode(null)
      setInviteCodeError(errorMessage(e))
    }
  }, [])

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
    staleInviteCodeRef.current = null
    invitePollAttemptsRef.current = 0
    setInvitePollRequested(false)
    setInviteCode(null)
    setInviteCodeError(null)
  }, [])

  const requestInviteCodeRefresh = useCallback(() => {
    staleInviteCodeRef.current = inviteCode
    invitePollAttemptsRef.current = 0
    setInvitePollRequested(true)
    setInviteCode(null)
    setInviteCodeError(null)
  }, [inviteCode])

  const refreshAll = useCallback(() => {
    void refreshInstanceState()
    void refreshSaves()
    void refreshMods()
    void refreshPlayers()
    void refreshJobs()
    void refreshInviteCode()
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
    void refreshInstanceState()
    void refreshSaves()
    void refreshMods()
    void refreshPlayers()
    void refreshInviteCode()
    window.setTimeout(() => {
      void refreshInstanceState()
      void refreshInviteCode()
      void refreshPlayers()
    }, 1000)
  }, [refreshInstanceState, refreshInviteCode, refreshJobs, refreshMods, refreshPlayers, refreshSaves])

  useEffect(() => {
    const init = async () => {
      setLoading(true)
      // 并发加载所有数据，单个失败不阻塞其他
      await Promise.allSettled([
        refreshInstanceState(),
        refreshSaves(),
        refreshMods(),
        refreshPlayers(),
        refreshJobs(),
        refreshInviteCode(),
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
      if (pollRef.current !== null) clearInterval(pollRef.current)
      if (playersPollRef.current !== null) clearTimeout(playersPollRef.current)
      if (invitePollRef.current !== null) clearTimeout(invitePollRef.current)
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
    refreshInviteCode,
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
      void getJobLogs(jobId)
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
    if (instanceState.state === 'running') {
      void refreshInviteCode()
      void refreshPlayers()
      return
    }
    setInviteCode(null)
    setInviteCodeError(null)
    // 服务器一旦不再是 running，就把在线玩家列表清空，避免下一次启动时
    // ServerControlPage 用上一轮运行时残留的"主机在线"快照误判为已就绪——
    // refreshPlayers() 请求可能因为容器还没起来而失败，失败分支不会清空
    // players，如果不在这里主动清空，旧快照会一直挂着直到请求成功为止。
    setPlayers(null)
    void refreshPlayers()
    setPlayersError(null)
  }, [instanceState?.state, refreshInviteCode, refreshPlayers])

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
      await refreshPlayers()
      if (cancelled) return
      playersPollRef.current = window.setTimeout(() => {
        void pollPlayers()
      }, 5_000)
    }
    playersPollRef.current = window.setTimeout(() => {
      void pollPlayers()
    }, 5_000)
    return () => {
      cancelled = true
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

    const stateCanExposeInvite =
      instanceState?.state === 'running' || instanceState?.state === 'starting'
    const shouldPollInvite = stateCanExposeInvite && invitePollRequested && !inviteCode
    if (!shouldPollInvite) return

    let cancelled = false
    const pollInviteCode = async () => {
      if (invitePollAttemptsRef.current >= 20) {
        setInvitePollRequested(false)
        return
      }
      invitePollAttemptsRef.current += 1
      await refreshInviteCode()
      if (cancelled) return
      if (invitePollAttemptsRef.current >= 20) {
        setInvitePollRequested(false)
        return
      }
      invitePollRef.current = window.setTimeout(() => {
        void refreshInstanceState()
        void pollInviteCode()
      }, 5_000)
    }

    invitePollRef.current = window.setTimeout(() => {
      void pollInviteCode()
    }, 5_000)

    return () => {
      cancelled = true
      if (invitePollRef.current !== null) {
        clearTimeout(invitePollRef.current)
        invitePollRef.current = null
      }
    }
  }, [
    instanceState?.state,
    inviteCode,
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
    publicIP,
    savesError,
    modsError,
    playersError,
    healthError,
    inviteCodeError,
    publicIPError,
    loading,
    playersLoading,
    inviteCodeRefreshing: invitePollRequested,
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
