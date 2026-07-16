import { useState } from 'react'
import { getInstanceGameLanguage, restartInstance, updateInstanceGameLanguage } from '../../api'
import { errorMessage } from '../../core/helpers'

type GameLanguageOptions = {
  isAdmin: boolean
  isRunning: boolean
}

export function useGameLanguage({ isAdmin, isRunning }: GameLanguageOptions) {
  const [gameLanguageOpen, setGameLanguageOpen] = useState(false)
  const [gameLanguageCode, setGameLanguageCode] = useState('zh')
  const [gameLanguageLoading, setGameLanguageLoading] = useState(false)
  const [gameLanguageSaving, setGameLanguageSaving] = useState(false)
  const [gameLanguageError, setGameLanguageError] = useState<string | null>(null)
  const [gameLanguageMessage, setGameLanguageMessage] = useState<string | null>(null)

  async function openGameLanguage() {
    if (!isAdmin) return
    setGameLanguageOpen(true)
    setGameLanguageLoading(true)
    setGameLanguageError(null)
    setGameLanguageMessage(null)
    try {
      const result = await getInstanceGameLanguage()
      setGameLanguageCode(result.languageCode)
    } catch (error) {
      setGameLanguageError(errorMessage(error))
      setGameLanguageCode('zh')
    } finally {
      setGameLanguageLoading(false)
    }
  }

  async function saveGameLanguage(restartAfterSave = false) {
    setGameLanguageSaving(true)
    setGameLanguageError(null)
    setGameLanguageMessage(null)
    try {
      const result = await updateInstanceGameLanguage({ languageCode: gameLanguageCode })
      setGameLanguageCode(result.languageCode)
      if (restartAfterSave && isRunning) {
        await restartInstance()
        setGameLanguageMessage('语言已保存，服务器正在重启，重启完成后生效。')
      } else {
        setGameLanguageMessage(isRunning ? '语言已保存，将在下次重启服务器后生效。' : '语言已保存，将在下次启动服务器时生效。')
      }
    } catch (error) {
      setGameLanguageError(errorMessage(error))
    } finally {
      setGameLanguageSaving(false)
    }
  }

  return {
    gameLanguageOpen,
    setGameLanguageOpen,
    gameLanguageCode,
    setGameLanguageCode,
    gameLanguageLoading,
    gameLanguageSaving,
    gameLanguageError,
    gameLanguageMessage,
    setGameLanguageMessage,
    openGameLanguage,
    saveGameLanguage,
  }
}
