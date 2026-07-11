import { useState } from 'react'
import { sendSay } from '../../api'
import { errorMessage } from '../../core/helpers'

export function useServerBroadcast() {
  const [sayMessage, setSayMessage] = useState('')
  const [sayBusy, setSayBusy] = useState(false)
  const [sayResult, setSayResult] = useState<string | null>(null)
  const [sayError, setSayError] = useState<string | null>(null)

  async function handleSay() {
    if (!sayMessage.trim()) return
    setSayBusy(true)
    setSayResult(null)
    setSayError(null)
    try {
      const res = await sendSay(sayMessage.trim())
      setSayResult(res.output?.trim() || '消息已发送')
      setSayMessage('')
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
    handleSay,
  }
}
