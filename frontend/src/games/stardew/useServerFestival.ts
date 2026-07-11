import { useState } from 'react'
import { triggerFestivalEvent } from '../../api'
import { errorMessage } from '../../core/helpers'

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
      const result = await triggerFestivalEvent()
      setFestivalMessage(result.output?.trim() || '触发节日活动指令已提交。')
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
