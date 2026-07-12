import { useState } from 'react'
import { triggerFestivalEvent } from '../../api'
import { errorMessage } from '../../core/helpers'
import { submitAndWaitForPlayerCommand } from './player-command-results'

type ServerFestivalOptions = {
  isAdmin: boolean
  isRunning: boolean
}

export function useServerFestival({ isAdmin, isRunning }: ServerFestivalOptions) {
  const [festivalBusy, setFestivalBusy] = useState(false)
  const [festivalMessage, setFestivalMessage] = useState<string | null>(null)
  const [festivalError, setFestivalError] = useState(false)

  async function handleTriggerFestivalEvent() {
    if (!isAdmin || !isRunning) return
    setFestivalBusy(true)
    setFestivalMessage(null)
    setFestivalError(false)
    try {
      await submitAndWaitForPlayerCommand(
        () => triggerFestivalEvent(),
        'trigger-event',
        '',
        (feedback) => {
          setFestivalError(feedback.kind === 'failed')
          setFestivalMessage(feedback.message)
        },
      )
    } catch (e) {
      setFestivalError(true)
      setFestivalMessage(errorMessage(e))
    } finally {
      setFestivalBusy(false)
    }
  }

  return {
    festivalBusy,
    festivalMessage,
    festivalError,
    handleTriggerFestivalEvent,
  }
}
