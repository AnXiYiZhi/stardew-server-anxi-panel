import { useState } from 'react'
import { getInstanceServerRuntimeSettings, updateInstanceServerRuntimeSettings } from '../../api'
import { errorMessage } from '../../core/helpers'
import type { ServerRuntimeSettings } from '../../types'

const defaultRuntimeSettings: ServerRuntimeSettings = {
  cabinStrategy: 'CabinStack',
  existingCabinBehavior: 'KeepExisting',
  networkBroadcastPeriod: 1,
}

type RuntimeSettingsOptions = {
  isAdmin: boolean
}

export function useServerRuntimeSettings({ isAdmin }: RuntimeSettingsOptions) {
  const [runtimeSettingsOpen, setRuntimeSettingsOpen] = useState(false)
  const [runtimeSettingsDraft, setRuntimeSettingsDraft] = useState<ServerRuntimeSettings>(defaultRuntimeSettings)
  const [runtimeSettingsLoading, setRuntimeSettingsLoading] = useState(false)
  const [runtimeSettingsSaving, setRuntimeSettingsSaving] = useState(false)
  const [runtimeSettingsError, setRuntimeSettingsError] = useState<string | null>(null)
  const [runtimeSettingsMessage, setRuntimeSettingsMessage] = useState<string | null>(null)

  async function openRuntimeSettings() {
    if (!isAdmin) return
    setRuntimeSettingsOpen(true)
    setRuntimeSettingsLoading(true)
    setRuntimeSettingsSaving(false)
    setRuntimeSettingsError(null)
    setRuntimeSettingsMessage(null)
    try {
      const res = await getInstanceServerRuntimeSettings()
      setRuntimeSettingsDraft(res)
    } catch (e) {
      setRuntimeSettingsError(errorMessage(e))
      setRuntimeSettingsDraft(defaultRuntimeSettings)
    } finally {
      setRuntimeSettingsLoading(false)
    }
  }

  function closeRuntimeSettings() {
    setRuntimeSettingsOpen(false)
  }

  async function handleSaveRuntimeSettings() {
    setRuntimeSettingsSaving(true)
    setRuntimeSettingsError(null)
    setRuntimeSettingsMessage(null)
    try {
      const res = await updateInstanceServerRuntimeSettings(runtimeSettingsDraft)
      setRuntimeSettingsDraft(res)
      setRuntimeSettingsMessage('设置已保存，需要重启服务器容器后才会生效。')
    } catch (e) {
      setRuntimeSettingsError(errorMessage(e))
    } finally {
      setRuntimeSettingsSaving(false)
    }
  }

  return {
    runtimeSettingsOpen,
    runtimeSettingsDraft,
    setRuntimeSettingsDraft,
    runtimeSettingsLoading,
    runtimeSettingsSaving,
    runtimeSettingsError,
    runtimeSettingsMessage,
    setRuntimeSettingsMessage,
    openRuntimeSettings,
    closeRuntimeSettings,
    handleSaveRuntimeSettings,
  }
}
