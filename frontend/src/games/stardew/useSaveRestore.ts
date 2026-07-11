import { useState } from 'react'
import { ApiError, restoreSaveBackup } from '../../api'
import { errorMessage } from '../../core/helpers'
import type { BackupInfo, SaveInfo } from '../../types'

type SaveRestoreOptions = {
  saves: SaveInfo[]
  isAdmin: boolean
  isRunning: boolean
  busy: boolean
  setBusy: (busy: boolean) => void
  onJobStarted: (jobId: string) => void
  onStateRefresh: () => void
  onSavesChanged?: () => void
  loadSaves: () => Promise<void>
  loadBackups: () => Promise<void>
  clearBackupMessage: () => void
}

export function useSaveRestore({
  saves,
  isAdmin,
  isRunning,
  busy,
  setBusy,
  onJobStarted,
  onStateRefresh,
  onSavesChanged,
  loadSaves,
  loadBackups,
  clearBackupMessage,
}: SaveRestoreOptions) {
  const [restoreBackup, setRestoreBackup] = useState<BackupInfo | null>(null)
  const [restoreNeedsOverwrite, setRestoreNeedsOverwrite] = useState(false)
  const [restoreError, setRestoreError] = useState('')

  function openRestoreDialog(backup: BackupInfo) {
    setRestoreBackup(backup)
    setRestoreNeedsOverwrite(saves.some((save) => save.name === backup.saveName))
    setRestoreError('')
  }

  function cancelRestoreDialog() {
    setRestoreBackup(null)
    setRestoreNeedsOverwrite(false)
    setRestoreError('')
  }

  async function handleRestoreConfirmed(overwrite: boolean) {
    if (!restoreBackup) return
    setBusy(true)
    setRestoreError('')
    clearBackupMessage()
    try {
      const result = await restoreSaveBackup(restoreBackup.name, overwrite, isRunning)
      setRestoreBackup(null)
      setRestoreNeedsOverwrite(false)
      if (result.jobId) {
        // Server was running: stop -> restore -> start submitted as one job.
        // Let the existing job-polling/SSE machinery drive save/state refresh
        // once it finishes, instead of reloading immediately (the restore
        // itself hasn't happened yet at this point).
        onJobStarted(result.jobId)
      } else {
        await loadSaves()
        await loadBackups()
        onStateRefresh()
        onSavesChanged?.()
      }
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

  const restoreSaveExists = restoreBackup ? saves.some((save) => save.name === restoreBackup.saveName) : false
  // 回档相关按钮（列表行入口和弹窗内提交）都不再因服务器运行中禁用——确认后
  // 会自动停止服务器、完成回档，再重新启动服务器（见 handleRestoreConfirmed
  // 的 autoRestart），而不是给一个只有 hover 才看得到说明的禁用按钮。
  const restoreBlocked = busy || !isAdmin

  return {
    restoreBackup,
    restoreNeedsOverwrite,
    restoreError,
    restoreSaveExists,
    restoreBlocked,
    openRestoreDialog,
    cancelRestoreDialog,
    handleRestoreConfirmed,
  }
}
