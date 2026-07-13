import { useCallback, useState } from 'react'
import { getSaveBackups, createSaveBackup, updateSaveBackupPolicy, deleteSaveBackup } from '../../api'
import { errorMessage } from '../../core/helpers'
import type { BackupInfo, BackupPolicy } from '../../types'

const defaultBackupPolicy: BackupPolicy = {
  gameSaveBackups: true,
  retainGameDays: 5,
}

function normalizeBackupPolicy(policy: BackupPolicy): BackupPolicy {
  const rawDays = (policy as Partial<BackupPolicy>).retainGameDays
  const retainGameDays = Math.max(
    1,
    Math.min(14, typeof rawDays === 'number' && Number.isFinite(rawDays) ? rawDays : defaultBackupPolicy.retainGameDays),
  )
  return { ...defaultBackupPolicy, ...policy, retainGameDays }
}

type SaveBackupsOptions = {
  isAdmin: boolean
  setBusy: (busy: boolean) => void
}

export function useSaveBackups({ isAdmin, setBusy }: SaveBackupsOptions) {
  const [backups, setBackups] = useState<BackupInfo[]>([])
  const [backupsLoading, setBackupsLoading] = useState(false)
  const [backupMessage, setBackupMessage] = useState('')
  const [backupPolicy, setBackupPolicy] = useState<BackupPolicy>(defaultBackupPolicy)
  const [backupPolicyDraft, setBackupPolicyDraft] = useState<BackupPolicy>(defaultBackupPolicy)
  const [backupPolicyBusy, setBackupPolicyBusy] = useState(false)
  const [showAllBackups, setShowAllBackups] = useState(false)
  const [deleteBackupTarget, setDeleteBackupTarget] = useState<BackupInfo | null>(null)

  const loadBackups = useCallback(async () => {
    if (!isAdmin) {
      setBackups([])
      return
    }
    setBackupsLoading(true)
    setBackupMessage('')
    try {
      const result = await getSaveBackups()
      // Older panel versions encoded an empty Go slice as null. Keep this
      // boundary defensive so a malformed/legacy response cannot crash the
      // lazy-loaded saves page during its first render on a fresh server.
      setBackups(Array.isArray(result.backups) ? result.backups : [])
      if (result.policy) {
        const normalizedPolicy = normalizeBackupPolicy(result.policy)
        setBackupPolicy(normalizedPolicy)
        setBackupPolicyDraft(normalizedPolicy)
      }
    } catch (error) {
      setBackupMessage(errorMessage(error))
    } finally {
      setBackupsLoading(false)
    }
  }, [isAdmin])

  async function handleManualBackup(name: string) {
    setBusy(true)
    setBackupMessage('')
    try {
      await createSaveBackup(name)
      await loadBackups()
      setBackupMessage('手动备份已创建。')
    } catch (error) {
      setBackupMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  async function handleBackupPolicySave() {
    setBackupPolicyBusy(true)
    setBackupMessage('')
    try {
      const result = await updateSaveBackupPolicy(normalizeBackupPolicy(backupPolicyDraft))
      const normalizedPolicy = normalizeBackupPolicy(result.policy)
      setBackupPolicy(normalizedPolicy)
      setBackupPolicyDraft(normalizedPolicy)
      await loadBackups()
      setBackupMessage('备份设置已保存。')
    } catch (error) {
      setBackupMessage(errorMessage(error))
    } finally {
      setBackupPolicyBusy(false)
    }
  }

  async function handleBackupDeleteConfirmed() {
    if (!deleteBackupTarget) return
    setBusy(true)
    setBackupMessage('')
    try {
      await deleteSaveBackup(deleteBackupTarget.name)
      setDeleteBackupTarget(null)
      await loadBackups()
    } catch (error) {
      setBackupMessage(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  const backupPolicyChanged = JSON.stringify(backupPolicyDraft) !== JSON.stringify(backupPolicy)
  // 游戏日回档：按游戏内日期序号（不是现实创建时间）排序，后端已经按策略保留了最近 N 个游戏日。
  const autoBackups = [...backups]
    .filter((b) => b.kind === 'auto')
    .sort((a, b) => (b.gameDayOrdinal ?? 0) - (a.gameDayOrdinal ?? 0))
  // 其他备份：手动 / 删除前 / 回档前保护 / 历史遗留（latest、daily、scheduled），按现实创建时间排序。
  const otherBackups = backups
    .filter((b) => b.kind !== 'auto')
    .sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))

  return {
    backups,
    backupsLoading,
    backupMessage,
    backupPolicy,
    backupPolicyDraft,
    setBackupPolicyDraft,
    backupPolicyBusy,
    backupPolicyChanged,
    autoBackups,
    otherBackups,
    showAllBackups,
    setShowAllBackups,
    deleteBackupTarget,
    openDeleteBackupDialog: setDeleteBackupTarget,
    cancelDeleteBackupDialog: () => setDeleteBackupTarget(null),
    loadBackups,
    handleManualBackup,
    handleBackupPolicySave,
    handleBackupDeleteConfirmed,
    clearBackupMessage: () => setBackupMessage(''),
  }
}
