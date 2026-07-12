import { useState } from 'react'
import { requestGameSave } from '../../api'
import { errorMessage } from '../../core/helpers'
import { submitAndWaitForPlayerCommand } from './player-command-results'

type ServerSaveNowOptions = {
  isAdmin: boolean
  isRunning: boolean
}

const SAVE_RESULT_TIMEOUT_MS = 125_000

export function useServerSaveNow({ isAdmin, isRunning }: ServerSaveNowOptions) {
  const [saveNowBusy, setSaveNowBusy] = useState(false)
  const [saveNowMessage, setSaveNowMessage] = useState<string | null>(null)
  const [saveNowError, setSaveNowError] = useState(false)

  async function handleSaveNow() {
    if (!isAdmin || !isRunning || saveNowBusy) return
    setSaveNowBusy(true)
    setSaveNowMessage(null)
    setSaveNowError(false)
    try {
      await submitAndWaitForPlayerCommand(
        () => requestGameSave(),
        'save-now',
        '',
        (feedback) => {
          setSaveNowError(feedback.kind === 'failed')
          setSaveNowMessage(feedback.message)
        },
        SAVE_RESULT_TIMEOUT_MS,
      )
    } catch (e) {
      setSaveNowError(true)
      setSaveNowMessage(errorMessage(e))
    } finally {
      setSaveNowBusy(false)
    }
  }

  return { saveNowBusy, saveNowMessage, saveNowError, handleSaveNow }
}
