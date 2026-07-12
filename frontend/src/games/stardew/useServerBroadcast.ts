import { useState } from 'react'
import { sendSay } from '../../api'
import { errorMessage } from '../../core/helpers'
import { submitAndWaitForPlayerCommand } from './player-command-results'

export function useServerBroadcast() {
  const [sayMessage, setSayMessage] = useState('')
  const [sayBusy, setSayBusy] = useState(false)
  const [sayResult, setSayResult] = useState<string | null>(null)
  const [sayError, setSayError] = useState<string | null>(null)
  const [sayConfirmed, setSayConfirmed] = useState(false)

  async function handleSay() {
    if (!sayMessage.trim()) return
    setSayBusy(true)
    setSayResult(null)
    setSayError(null)
    try {
      const feedback = await submitAndWaitForPlayerCommand(
        () => sendSay(sayMessage.trim()),
        'broadcast',
        '',
        (next) => {
          if (next.kind === 'failed') {
            setSayResult(null)
            setSayError(next.message)
            setSayConfirmed(false)
          } else {
            setSayError(null)
            setSayResult(next.message)
            setSayConfirmed(next.kind === 'succeeded')
          }
        },
      )
      if (feedback.kind === 'succeeded' || feedback.kind === 'legacy') setSayMessage('')
    } catch (e) {
      setSayError(errorMessage(e))
    } finally {
      setSayBusy(false)
    }
  }

  return {
    sayMessage,
    setSayMessage,
    sayBusy,
    sayResult,
    sayError,
    sayConfirmed,
    handleSay,
  }
}
