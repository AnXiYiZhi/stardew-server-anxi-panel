import { useState } from 'react'
import { getInstanceServerPassword, updateInstanceServerPassword, getInstancePasswordStatus } from '../../api'
import { errorMessage } from '../../core/helpers'
import type { InstancePasswordStatus } from '../../types'

type ServerPasswordOptions = {
  isAdmin: boolean
}

export function useServerPassword({ isAdmin }: ServerPasswordOptions) {
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [passwordDraft, setPasswordDraft] = useState('')
  const [passwordVisible, setPasswordVisible] = useState(false)
  const [passwordLoading, setPasswordLoading] = useState(false)
  const [passwordSaving, setPasswordSaving] = useState(false)
  const [passwordError, setPasswordError] = useState<string | null>(null)
  const [passwordMessage, setPasswordMessage] = useState<string | null>(null)
  const [passwordStatus, setPasswordStatus] = useState<InstancePasswordStatus | null>(null)
  const [passwordStatusLoading, setPasswordStatusLoading] = useState(false)
  const [passwordStatusError, setPasswordStatusError] = useState<string | null>(null)

  async function loadPasswordStatus() {
    setPasswordStatusLoading(true)
    setPasswordStatusError(null)
    try {
      const res = await getInstancePasswordStatus()
      setPasswordStatus(res)
    } catch (e) {
      setPasswordStatus(null)
      setPasswordStatusError(errorMessage(e))
    } finally {
      setPasswordStatusLoading(false)
    }
  }

  async function openPasswordSettings() {
    if (!isAdmin) return
    setPasswordOpen(true)
    setPasswordVisible(false)
    setPasswordLoading(true)
    setPasswordSaving(false)
    setPasswordError(null)
    setPasswordMessage(null)
    try {
      const res = await getInstanceServerPassword()
      setPasswordDraft(res.serverPassword)
    } catch (e) {
      setPasswordError(errorMessage(e))
      setPasswordDraft('')
    } finally {
      setPasswordLoading(false)
    }
    void loadPasswordStatus()
  }

  function closePasswordSettings() {
    setPasswordOpen(false)
  }

  function togglePasswordVisible() {
    setPasswordVisible((v) => !v)
  }

  function updatePasswordDraft(value: string) {
    setPasswordDraft(value)
    setPasswordMessage(null)
  }

  async function handleSaveServerPassword() {
    if (passwordDraft.length > 128) {
      setPasswordError('服务器密码不能超过 128 个字符')
      setPasswordMessage(null)
      return
    }
    setPasswordSaving(true)
    setPasswordError(null)
    setPasswordMessage(null)
    try {
      const res = await updateInstanceServerPassword(passwordDraft)
      setPasswordDraft(res.serverPassword)
      setPasswordMessage('密码已保存，需要重启服务器容器后才会生效。')
    } catch (e) {
      setPasswordError(errorMessage(e))
    } finally {
      setPasswordSaving(false)
    }
  }

  return {
    passwordOpen,
    passwordDraft,
    passwordVisible,
    passwordLoading,
    passwordSaving,
    passwordError,
    passwordMessage,
    passwordStatus,
    passwordStatusLoading,
    passwordStatusError,
    openPasswordSettings,
    closePasswordSettings,
    togglePasswordVisible,
    updatePasswordDraft,
    loadPasswordStatus,
    handleSaveServerPassword,
  }
}
