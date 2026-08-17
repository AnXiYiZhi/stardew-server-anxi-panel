import { useState } from 'react'
import { getInstanceServerRuntimeSettings, updateInstanceServerRuntimeSettings } from '../../api'
import { errorMessage } from '../../core/helpers'
import type { ServerRuntimeSettings } from '../../types'
import {
  DEFAULT_SERVER_RUNTIME_SETTINGS,
  maxPlayersValidationError,
  normalizeServerRuntimeSettings,
} from './server-runtime-settings-state'

type RuntimeSettingsOptions = {
  isAdmin: boolean
  isRunning: boolean
  refreshPlayers: () => void
}

export function useServerRuntimeSettings({ isAdmin, isRunning, refreshPlayers }: RuntimeSettingsOptions) {
  const [runtimeSettingsOpen, setRuntimeSettingsOpen] = useState(false)
  const [runtimeSettingsDraft, setRuntimeSettingsDraft] = useState<ServerRuntimeSettings>(DEFAULT_SERVER_RUNTIME_SETTINGS)
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
      setRuntimeSettingsDraft(normalizeServerRuntimeSettings(res))
    } catch (e) {
      setRuntimeSettingsError(errorMessage(e))
      setRuntimeSettingsDraft(DEFAULT_SERVER_RUNTIME_SETTINGS)
    } finally {
      setRuntimeSettingsLoading(false)
    }
  }

  function closeRuntimeSettings() {
    setRuntimeSettingsOpen(false)
  }

  function clearRuntimeSettingsFeedback() {
    setRuntimeSettingsError(null)
    setRuntimeSettingsMessage(null)
  }

  async function handleSaveRuntimeSettings() {
    const validationError = maxPlayersValidationError(runtimeSettingsDraft.maxPlayers)
    if (validationError) {
      setRuntimeSettingsError(validationError)
      return
    }
    setRuntimeSettingsSaving(true)
    setRuntimeSettingsError(null)
    setRuntimeSettingsMessage(null)
    try {
      const res = await updateInstanceServerRuntimeSettings(runtimeSettingsDraft)
      setRuntimeSettingsDraft(normalizeServerRuntimeSettings(res))
      await refreshPlayers()
      setRuntimeSettingsMessage(isRunning
        ? '设置已保存；人数上限与高级设置将在重启后生效。'
        : '设置已保存；人数上限与高级设置将在下次启动时生效。')
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
    clearRuntimeSettingsFeedback,
    openRuntimeSettings,
    closeRuntimeSettings,
    handleSaveRuntimeSettings,
  }
}
