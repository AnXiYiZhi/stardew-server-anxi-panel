export const WORLD_DELETE_HOLD_MS = 1_200
export function canDeleteWorld(role: string, isDefault: boolean | undefined): boolean {
  return role === 'admin' && isDefault === false
}

// The clock and scheduler are injected so pointer cancellation is exercised
// with deterministic time, including callbacks already queued when canceled.
export function worldDeleteGesture(schedule: (run: () => void, ms: number) => () => void, ready: () => void, progress: (active: boolean) => void, now = Date.now) {
  let pointer: { id: number; x: number; y: number; cancel: () => void } | null = null
  let suppressed = false
  let generation = 0
  let started = 0
  const cancel = (suppress = true) => {
    generation++
    if (pointer) { pointer.cancel(); pointer = null; progress(false); if (suppress) suppressed = true }
  }
  return {
    start(id: number, x: number, y: number) {
      cancel()
      suppressed = false
      started = now()
      const token = ++generation
      pointer = { id, x, y, cancel: schedule(() => {
        if (token !== generation || !pointer) return
        suppressed = true
        cancel()
        ready()
      }, WORLD_DELETE_HOLD_MS) }
      progress(true)
    },
    move(id: number, x: number, y: number) {
      if (pointer?.id === id && Math.hypot(x - pointer.x, y - pointer.y) > 10) cancel()
    },
    cancel,
    release() { cancel(now() - started >= 180) },
    // An ordinary click is still an entry point. Any aborted hold/scroll is
    // consumed; keyboard clicks have detail=0 and are handled independently.
    consumeClick() { const value = suppressed; suppressed = false; return value },
  }
}
