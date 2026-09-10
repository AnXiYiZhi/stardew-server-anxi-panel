import { useCallback, useEffect, useRef, useState } from 'react'
import { getFarmhandDeleteIntent } from '../../api'
import { errorMessage } from '../../core/helpers'
import type { FarmhandDeleteIntent } from '../../types'

export function useFarmhandDeleteIntent(instanceId: string, enabled: boolean) {
  const [intent, setIntent] = useState<FarmhandDeleteIntent | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const sequence = useRef({ next: 0, applied: 0 })

  const refresh = useCallback(async (accepted?: FarmhandDeleteIntent) => {
    const requestId = ++sequence.current.next
    if (!enabled) {
      sequence.current.applied = requestId
      setIntent(null)
      setError(null)
      setLoading(false)
      return
    }
    if (accepted) {
      sequence.current.applied = requestId
      setIntent(accepted)
      setError(null)
      setLoading(false)
      return
    }
    setLoading(true)
    try {
      const result = await getFarmhandDeleteIntent(instanceId)
      if (requestId < sequence.current.applied) return
      sequence.current.applied = requestId
      setIntent(result.intent)
      setError(null)
    } catch (reason) {
      if (requestId < sequence.current.applied) return
      sequence.current.applied = requestId
      setError(errorMessage(reason))
      throw reason
    } finally {
      if (requestId === sequence.current.next) setLoading(false)
    }
  }, [enabled, instanceId])

  useEffect(() => {
    void refresh().catch(() => undefined)
    return () => { sequence.current.applied = ++sequence.current.next }
  }, [refresh])

  useEffect(() => {
    if (!enabled) return
    const timer = window.setInterval(() => void refresh().catch(() => undefined), 3000)
    return () => window.clearInterval(timer)
  }, [enabled, refresh])

  return { intent, loading, error, refresh }
}
