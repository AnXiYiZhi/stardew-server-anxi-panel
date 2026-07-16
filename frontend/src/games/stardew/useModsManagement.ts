import { useCallback, useEffect, useRef, useState } from 'react'
import {
  deleteMod,
  exportMods,
  exportModSyncPack,
  exportModSyncUpdatePack,
  getMods,
  updateAllModsEnabled,
  updateModEnabled,
  updateModSyncClassification,
  uploadMods,
} from '../../api'
import { errorMessage } from '../../core/helpers'
import type { ModInfo, ModsListResult, ModSyncKind, ModUploadSummary } from '../../types'
import type { StardewPageProps } from './stardew-routes'

type UseModsManagementOptions = {
  dashboardData: StardewPageProps['dashboardData']
  activeSaveName: string
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  document.body.appendChild(anchor)
  anchor.click()
  document.body.removeChild(anchor)
  URL.revokeObjectURL(url)
}

export function useModsManagement({ dashboardData, activeSaveName }: UseModsManagementOptions) {
  const [data, setData] = useState<ModsListResult | null>(dashboardData.mods)
  const [loading, setLoading] = useState(false)
  const [listError, setListError] = useState<string | null>(null)
  const [showUpload, setShowUpload] = useState(false)
  const [uploadFiles, setUploadFiles] = useState<File[]>([])
  const [uploadBusy, setUploadBusy] = useState(false)
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [uploadSuccess, setUploadSuccess] = useState<ModUploadSummary | null>(null)
  const [confirmDelete, setConfirmDelete] = useState<ModInfo | null>(null)
  const [deleteBusy, setDeleteBusy] = useState(false)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [exportBusy, setExportBusy] = useState(false)
  const [exportError, setExportError] = useState<string | null>(null)
  const [syncUpdating, setSyncUpdating] = useState<string | null>(null)
  const [syncError, setSyncError] = useState<string | null>(null)
  const [syncPackBusy, setSyncPackBusy] = useState<'full' | 'update' | null>(null)
  const [syncPackError, setSyncPackError] = useState<string | null>(null)
  const [enableUpdating, setEnableUpdating] = useState<string | null>(null)
	const [enableAllUpdating, setEnableAllUpdating] = useState<boolean | null>(null)
  const [enableError, setEnableError] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const loadMods = useCallback(async () => {
    setLoading(true)
    setListError(null)
    try {
      const result = await getMods()
      setData(result)
      return result
    } catch (error) {
      setListError(errorMessage(error))
      return null
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!dashboardData.mods) void loadMods()
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (dashboardData.mods) setData(dashboardData.mods)
  }, [dashboardData.mods])

  async function handleUpload() {
    if (uploadFiles.length === 0) return
    setUploadBusy(true)
    setUploadError(null)
    setUploadSuccess(null)
    try {
      const result = await uploadMods(uploadFiles)
      await loadMods()
      void dashboardData.refreshMods()
      setShowUpload(false)
      setUploadFiles([])
      if (fileInputRef.current) fileInputRef.current.value = ''
      setUploadSuccess(result.upload ?? {
        archiveCount: uploadFiles.length,
        discoveredCount: result.mods.length,
        importedCount: result.mods.length,
        enabledCount: result.mods.length,
      })
      window.setTimeout(() => setUploadSuccess(null), 8000)
    } catch (error) {
      setUploadError(errorMessage(error))
    } finally {
      setUploadBusy(false)
    }
  }

  function openUpload() {
    setUploadError(null)
    setShowUpload(true)
  }

  function closeUpload() {
    if (uploadBusy) return
    setShowUpload(false)
    setUploadFiles([])
    if (fileInputRef.current) fileInputRef.current.value = ''
    setUploadError(null)
  }

  function openDeleteConfirm(mod: ModInfo) {
    setDeleteError(null)
    setConfirmDelete(mod)
  }

  function closeDeleteConfirm() {
    if (deleteBusy) return
    setConfirmDelete(null)
    setDeleteError(null)
  }

  async function handleDeleteConfirm() {
    if (!confirmDelete) return
    setDeleteBusy(true)
    setDeleteError(null)
    try {
      await deleteMod(confirmDelete.id)
      void dashboardData.refreshMods()
      await loadMods()
      setConfirmDelete(null)
    } catch (error) {
      setDeleteError(errorMessage(error))
    } finally {
      setDeleteBusy(false)
    }
  }

  async function handleExport() {
    setExportBusy(true)
    setExportError(null)
    try {
      const { blob, filename } = await exportMods()
      downloadBlob(blob, filename)
    } catch (error) {
      setExportError(errorMessage(error))
    } finally {
      setExportBusy(false)
    }
  }

  async function handleSyncChange(mod: ModInfo, syncKind: ModSyncKind) {
    setSyncError(null)
    setSyncUpdating(mod.id)
    try {
      const result = await updateModSyncClassification(mod.id, syncKind)
      const updates = new Map(result.mods.map((item) => [item.folderName, item]))
      setData((previous) => previous ? {
        ...previous,
        mods: previous.mods.map((modItem) => {
          const updated = updates.get(modItem.folderName)
          return updated ? { ...modItem, syncKind: updated.syncKind, syncNote: updated.syncNote } : modItem
        }),
      } : previous)
      void dashboardData.refreshMods()
    } catch (error) {
      setSyncError(errorMessage(error))
    } finally {
      setSyncUpdating(null)
    }
  }

  async function handleEnabledChange(mod: ModInfo, enabled: boolean) {
    setEnableError(null)
    setEnableUpdating(mod.id)
    try {
      const result = await updateModEnabled(mod.id, enabled, activeSaveName || undefined)
      const updates = new Map(result.mods.map((item) => [item.folderName, item]))
      setData((previous) => previous ? {
        ...previous,
        mods: previous.mods.map((modItem) => {
          const updated = updates.get(modItem.folderName)
          return updated ? {
            ...modItem,
            enabled: updated.enabled,
            canToggle: updated.canToggle,
            enableNote: updated.enableNote,
            dependencies: updated.dependencies ?? modItem.dependencies,
          } : modItem
        }),
      } : previous)
      void dashboardData.refreshMods()
    } catch (error) {
      setEnableError(errorMessage(error))
    } finally {
      setEnableUpdating(null)
    }
  }

  async function handleAllEnabledChange(enabled: boolean) {
    setEnableError(null)
    setEnableAllUpdating(enabled)
    try {
      await updateAllModsEnabled(enabled, activeSaveName || undefined)
      await loadMods()
      void dashboardData.refreshMods()
    } catch (error) {
      setEnableError(errorMessage(error))
    } finally {
      setEnableAllUpdating(null)
    }
  }

  async function handleSyncPackExport(kind: 'full' | 'update') {
    setSyncPackBusy(kind)
    setSyncPackError(null)
    try {
      const { blob, filename } = kind === 'update' ? await exportModSyncUpdatePack() : await exportModSyncPack()
      downloadBlob(blob, filename)
    } catch (error) {
      setSyncPackError(errorMessage(error))
    } finally {
      setSyncPackBusy(null)
    }
  }

  return {
    data, loading, listError, showUpload, uploadFiles, setUploadFiles, uploadBusy, uploadError,
    uploadSuccess, confirmDelete, deleteBusy, deleteError, exportBusy, exportError, syncUpdating, syncError,
    syncPackBusy, syncPackError, enableUpdating, enableAllUpdating, enableError, fileInputRef, loadMods, openUpload, closeUpload,
    handleUpload, openDeleteConfirm, closeDeleteConfirm, handleDeleteConfirm, handleExport, handleSyncChange,
    handleEnabledChange, handleAllEnabledChange, handleSyncPackExport,
  }
}
