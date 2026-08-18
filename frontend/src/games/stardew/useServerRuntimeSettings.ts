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
  restartServer: () => Promise<void>
}

export function useServerRuntimeSettings({ isAdmin, isRunning, refreshPlayers, restartServer }: RuntimeSettingsOptions) {
  const [runtimeSettingsOpen, setRuntimeSettingsOpen] = useState(false)
  const [runtimeSettingsDraft, setRuntimeSettingsDraft] = useState<ServerRuntimeSettings>(DEFAULT_SERVER_RUNTIME_SETTINGS)
  const [runtimeSettingsLoading, setRuntimeSettingsLoading] = useState(false)
  const [runtimeSettingsSavingAction, setRuntimeSettingsSavingAction] = useState<'save' | 'save_restart' | null>(null)
  const [runtimeSettingsError, setRuntimeSettingsError] = useState<string | null>(null)
  const [runtimeSettingsMessage, setRuntimeSettingsMessage] = useState<string | null>(null)
  const runtimeSettingsSaving = runtimeSettingsSavingAction !== null

  async function openRuntimeSettings() {
    if (!isAdmin) return
    setRuntimeSettingsOpen(true)
    setRuntimeSettingsLoading(true)
    setRuntimeSettingsSavingAction(null)
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

  async function handleSaveRuntimeSettings(restartAfterSave = false) {
    const validationError = maxPlayersValidationError(runtimeSettingsDraft.maxPlayers)
    if (validationError) {
      setRuntimeSettingsError(validationError)
      return
    }
    setRuntimeSettingsSavingAction(restartAfterSave ? 'save_restart' : 'save')
    setRuntimeSettingsError(null)
    setRuntimeSettingsMessage(null)
    try {
      const res = await updateInstanceServerRuntimeSettings(runtimeSettingsDraft)
      setRuntimeSettingsDraft(normalizeServerRuntimeSettings(res))
      let playersRefreshFailed = false
      try {
        await refreshPlayers()
      } catch {
        playersRefreshFailed = true
      }
      if (restartAfterSave && isRunning) {
        try {
          await restartServer()
          setRuntimeSettingsOpen(false)
        } catch (restartError) {
          setRuntimeSettingsError(`设置已保存，但服务器重启失败：${errorMessage(restartError)}。可以重试或稍后手动重启。`)
        }
        return
      }
      setRuntimeSettingsMessage(isRunning
        ? `设置已保存；人数上限与高级设置将在重启后生效。${playersRefreshFailed ? ' 当前人数刷新暂时失败，面板会自动重试。' : ''}`
        : `设置已保存；人数上限与高级设置将在下次启动时生效。${playersRefreshFailed ? ' 当前人数刷新暂时失败，面板会自动重试。' : ''}`)
    } catch (e) {
      setRuntimeSettingsError(errorMessage(e))
    } finally {
      setRuntimeSettingsSavingAction(null)
    }
  }

  return {
    runtimeSettingsOpen,
    runtimeSettingsDraft,
    setRuntimeSettingsDraft,
    runtimeSettingsLoading,
    runtimeSettingsSaving,
    runtimeSettingsSavingAction,
    runtimeSettingsError,
    runtimeSettingsMessage,
    clearRuntimeSettingsFeedback,
    openRuntimeSettings,
    closeRuntimeSettings,
    handleSaveRuntimeSettings,
  }
}
