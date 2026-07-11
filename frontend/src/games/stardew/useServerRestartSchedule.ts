import { useState } from 'react'
import { getRestartSchedule, updateRestartSchedule } from '../../api'
import { errorMessage } from '../../core/helpers'
import type { RestartSchedule } from '../../types'

const defaultRestartSchedule: RestartSchedule = {
  instanceId: 'stardew',
  enabled: false,
  shutdownTime: '04:00',
  startupTime: '04:20',
  timezone: 'Asia/Shanghai',
  warningMinutes: [10, 5, 1],
  backupBeforeShutdown: true,
  skipIfPlayersOnline: false,
}

type RestartScheduleOptions = {
  isAdmin: boolean
  refreshJobs: () => void
}

export function useServerRestartSchedule({ isAdmin, refreshJobs }: RestartScheduleOptions) {
  const [scheduleOpen, setScheduleOpen] = useState(false)
  const [scheduleDraft, setScheduleDraft] = useState<RestartSchedule>(defaultRestartSchedule)
  const [scheduleLoading, setScheduleLoading] = useState(false)
  const [scheduleSaving, setScheduleSaving] = useState(false)
  const [scheduleError, setScheduleError] = useState<string | null>(null)
  const [scheduleSaved, setScheduleSaved] = useState<string | null>(null)

  async function openRestartSchedule() {
    if (!isAdmin) return
    setScheduleOpen(true)
    setScheduleLoading(true)
    setScheduleSaving(false)
    setScheduleError(null)
    setScheduleSaved(null)
    try {
      const result = await getRestartSchedule()
      setScheduleDraft(result.schedule)
    } catch (e) {
      setScheduleError(errorMessage(e))
      setScheduleDraft(defaultRestartSchedule)
    } finally {
      setScheduleLoading(false)
    }
  }

  function closeRestartSchedule() {
    setScheduleOpen(false)
  }

  async function handleSaveRestartSchedule() {
    setScheduleSaving(true)
    setScheduleError(null)
    setScheduleSaved(null)
    try {
      const result = await updateRestartSchedule(scheduleDraft)
      setScheduleDraft(result.schedule)
      setScheduleSaved('计划重启已保存。')
      refreshJobs()
    } catch (e) {
      setScheduleError(errorMessage(e))
    } finally {
      setScheduleSaving(false)
    }
  }

  function toggleScheduleWarning(minute: number) {
    setScheduleDraft((draft) => {
      const exists = draft.warningMinutes.includes(minute)
      const next = exists
        ? draft.warningMinutes.filter((value) => value !== minute)
        : [...draft.warningMinutes, minute]
      next.sort((a, b) => b - a)
      return { ...draft, warningMinutes: next }
    })
  }

  return {
    scheduleOpen,
    scheduleDraft,
    setScheduleDraft,
    scheduleLoading,
    scheduleSaving,
    scheduleError,
    scheduleSaved,
    openRestartSchedule,
    closeRestartSchedule,
    handleSaveRestartSchedule,
    toggleScheduleWarning,
  }
}
