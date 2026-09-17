type Listener<T> = { data: (value: T) => void; error?: (error: unknown) => void }
type Poll<T> = {
  listeners: Set<Listener<T>>
  stop: () => void
  latest?: T
  lastError?: unknown
}
const polls = new Map<string, Poll<unknown>>()

// One timer and one in-flight read per endpoint, shared by all visible views.
// Critical update/recovery state machines deliberately do not use this helper.
export function subscribeVisiblePoll<T>(key: string, intervalMs: number, read: (signal: AbortSignal) => Promise<T>, listener: Listener<T>, immediate = true): () => void {
  let poll = polls.get(key) as Poll<T> | undefined
  if (!poll) {
    const listeners = new Set<Listener<T>>()
    let stopped = false
    let busy = false
    let resumePending = false
    let timer: ReturnType<typeof setTimeout> | undefined
    let controller: AbortController | undefined
    const clear = () => { clearTimeout(timer); timer = undefined }
    const visible = () => document.visibilityState !== 'hidden'
    const schedule = () => { clear(); if (!stopped && visible()) timer = setTimeout(() => void run(), intervalMs) }
    const run = async () => {
      if (stopped || busy || !visible()) return
      clear()
      busy = true
      resumePending = false
      controller = new AbortController()
      const active = controller
      try {
        const value = await read(active.signal)
        if (!stopped && !active.signal.aborted) {
          poll!.latest = value
          poll!.lastError = undefined
          for (const subscriber of listeners) subscriber.data(value)
        }
      } catch (error) {
        if (!stopped && !active.signal.aborted) {
          poll!.lastError = error
          for (const subscriber of listeners) subscriber.error?.(error)
        }
      } finally {
        busy = false
        if (resumePending && !stopped && visible()) { resumePending = false; queueMicrotask(() => void run()) }
        else schedule()
      }
    }
    const visibility = () => {
      if (visible()) { if (busy) resumePending = true; else void run() }
      else { resumePending = false; clear(); controller?.abort() }
    }
    document.addEventListener('visibilitychange', visibility)
    poll = { listeners, stop: () => { stopped = true; clear(); controller?.abort(); document.removeEventListener('visibilitychange', visibility) } }
    polls.set(key, poll as Poll<unknown>)
    // Defer until the first listener has been installed.
    if (immediate) queueMicrotask(() => void run())
    else schedule()
  }
  poll.listeners.add(listener)
  if (poll.latest !== undefined) listener.data(poll.latest)
  if (poll.lastError !== undefined) listener.error?.(poll.lastError)
  const current = poll
  return () => {
    current.listeners.delete(listener)
    if (current.listeners.size === 0) { current.stop(); polls.delete(key) }
  }
}
