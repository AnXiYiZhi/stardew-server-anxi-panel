import { useCallback, useEffect, useRef, useState } from 'react'
import {
  createJobEventSource,
  getHealthDiagnostics,
  getInviteCode,
  getJobs,
  getMods,
  getSaves,
  getStardewState,
  getVersion,
} from '../../api'
import type { HealthDiagnosticsResponse, VersionInfo } from '../../api'
import type { InstanceState, Job, ModsListResult, SavesListResult } from '../../types'
import { errorMessage } from '../../core/helpers'
import type { StardewDashboardData } from './stardew-routes'

export function useStardewDashboardData(): StardewDashboardData {
  const [instanceState, setInstanceState] = useState<InstanceState | null>(null)
  const [saves, setSaves] = useState<SavesListResult | null>(null)
  const [mods, setMods] = useState<ModsListResult | null>(null)
  const [jobs, setJobs] = useState<Job[]>([])
  const [health, setHealth] = useState<HealthDiagnosticsResponse | null>(null)
  const [versionInfo, setVersionInfo] = useState<VersionInfo | null>(null)
  const [inviteCode, setInviteCode] = useState<string | null>(null)

  const [savesError, setSavesError] = useState<string | null>(null)
  const [modsError, setModsError] = useState<string | null>(null)
  const [healthError, setHealthError] = useState<string | null>(null)
  const [inviteCodeError, setInviteCodeError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const jobStreamsRef = useRef<Map<string, EventSource>>(new Map())

  const refreshInstanceState = useCallback(async () => {
    try {
      const s = await getStardewState()
      setInstanceState(s)
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

  const refreshJobs = useCallback(async () => {
    try {
      const res = await getJobs()
      setJobs(res.jobs)
    } catch {
      // 保留上次已知任务列表
    }
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

  const refreshInviteCode = useCallback(async () => {
    setInviteCodeError(null)
    try {
      const res = await getInviteCode()
      setInviteCode(res.inviteCode)
    } catch (e) {
      setInviteCode(null)
      setInviteCodeError(errorMessage(e))
    }
  }, [])

  // 版本信息只在初始化时加载一次，不对外暴露刷新函数
  const fetchVersion = useCallback(async () => {
    try {
      const res = await getVersion()
      setVersionInfo(res)
    } catch {
      // 静默失败
    }
  }, [])

  const refreshAll = useCallback(() => {
    void refreshInstanceState()
    void refreshSaves()
    void refreshMods()
    void refreshJobs()
    void refreshHealth()
    void refreshInviteCode()
  }, [
    refreshInstanceState,
    refreshSaves,
    refreshMods,
    refreshJobs,
    refreshHealth,
    refreshInviteCode,
  ])

  const refreshAfterJobFinished = useCallback(() => {
    void refreshJobs()
    void refreshInstanceState()
    void refreshSaves()
    void refreshMods()
    void refreshInviteCode()
    window.setTimeout(() => {
      void refreshInstanceState()
      void refreshInviteCode()
    }, 1000)
  }, [refreshInstanceState, refreshInviteCode, refreshJobs, refreshMods, refreshSaves])

  useEffect(() => {
    const init = async () => {
      setLoading(true)
      // 并发加载所有数据，单个失败不阻塞其他
      await Promise.allSettled([
        refreshInstanceState(),
        refreshSaves(),
        refreshMods(),
        refreshJobs(),
        refreshHealth(),
        refreshInviteCode(),
        fetchVersion(),
      ])
      setLoading(false)
    }
    void init()

    // 每 30s 轮询实例状态
    pollRef.current = setInterval(() => {
      void refreshInstanceState()
    }, 30_000)

    return () => {
      if (pollRef.current !== null) clearInterval(pollRef.current)
      for (const es of jobStreamsRef.current.values()) {
        es.close()
      }
      jobStreamsRef.current.clear()
    }
  }, [
    refreshInstanceState,
    refreshSaves,
    refreshMods,
    refreshJobs,
    refreshHealth,
    refreshInviteCode,
    fetchVersion,
  ])

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
      const es = createJobEventSource(jobId)
      jobStreamsRef.current.set(jobId, es)
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
  }, [jobs, refreshAfterJobFinished, refreshInstanceState, refreshJobs])

  useEffect(() => {
    if (!instanceState?.state) return
    if (instanceState.state === 'running') {
      void refreshInviteCode()
      return
    }
    setInviteCode(null)
    setInviteCodeError(null)
  }, [instanceState?.state, refreshInviteCode])

  return {
    instanceState,
    saves,
    mods,
    jobs,
    health,
    versionInfo,
    inviteCode,
    savesError,
    modsError,
    healthError,
    inviteCodeError,
    loading,
    refreshAll,
    refreshInstanceState,
    refreshSaves,
    refreshMods,
    refreshJobs,
    refreshHealth,
    refreshInviteCode,
  }
}
