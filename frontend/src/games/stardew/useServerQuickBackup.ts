import { useState } from 'react'
import { createSaveBackup } from '../../api'
import { errorMessage } from '../../core/helpers'

type QuickBackupOptions = {
  activeSaveName: string
  isAdmin: boolean
}

export function useServerQuickBackup({ activeSaveName, isAdmin }: QuickBackupOptions) {
  const [quickBackupBusy, setQuickBackupBusy] = useState(false)
  const [quickBackupMessage, setQuickBackupMessage] = useState<string | null>(null)
  const [quickBackupError, setQuickBackupError] = useState(false)

  async function handleQuickBackup() {
    if (!activeSaveName || !isAdmin) return
    setQuickBackupBusy(true)
    setQuickBackupMessage(null)
    setQuickBackupError(false)
    try {
      const result = await createSaveBackup(activeSaveName)
      setQuickBackupMessage(`已为 ${activeSaveName} 创建手动备份：${result.backupName}`)
    } catch (e) {
      setQuickBackupError(true)
      setQuickBackupMessage(errorMessage(e))
    } finally {
      setQuickBackupBusy(false)
    }
  }

  return {
    quickBackupBusy,
    quickBackupMessage,
    quickBackupError,
    handleQuickBackup,
  }
}
