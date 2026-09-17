export const CONTROL_ACTION_IDS = ['language', 'schedule', 'auth', 'runtime', 'save', 'festival', 'joja'] as const
export type ControlActionId = typeof CONTROL_ACTION_IDS[number]
export const CONTROL_ACTION_HOLD_MS = 450

export function normalizeControlActionOrder(value: unknown): ControlActionId[] {
  const saved = Array.isArray(value) ? value : []
  return [...new Set([...saved.filter((id): id is ControlActionId => CONTROL_ACTION_IDS.includes(id)), ...CONTROL_ACTION_IDS])]
}

export function readControlActionOrder(raw: string | null): ControlActionId[] {
  try {
    const saved: unknown = JSON.parse(raw ?? 'null')
    if (saved && typeof saved === 'object' && 'version' in saved && saved.version === 1 && 'order' in saved) {
      return normalizeControlActionOrder(saved.order)
    }
  } catch { /* Invalid browser preferences fall back to the complete default list. */ }
  return [...CONTROL_ACTION_IDS]
}

export function controlActionOrderKey(userId: number, instanceId: string): string {
  return `anxipanel:control-order:v1:${userId}:${encodeURIComponent(instanceId)}`
}

export function moveControlAction(order: readonly ControlActionId[], id: ControlActionId, index: number): ControlActionId[] {
  if (!order.includes(id) || !Number.isFinite(index)) return [...order]
  const next = order.filter((item) => item !== id)
  next.splice(Math.max(0, Math.min(next.length, Math.trunc(index))), 0, id)
  return next
}

// The scheduler is injected to test cancellation of a callback already queued by the browser.
export function controlReorderGesture(
  schedule: (run: () => void, ms: number) => () => void,
  activate: () => void,
  finish: (cancelled: boolean, wasActive: boolean) => void,
) {
  let pointer: { id: number; x: number; y: number; cancelTimer: () => void } | null = null
  let active = false
  let generation = 0

  function end(cancelled: boolean) {
    generation++
    if (!pointer) return
    pointer.cancelTimer()
    pointer = null
    const wasActive = active
    active = false
    finish(cancelled, wasActive)
  }

  return {
    start(id: number, x: number, y: number) {
      end(true)
      const token = ++generation
      pointer = { id, x, y, cancelTimer: schedule(() => {
        if (!pointer || generation !== token || active) return
        active = true
        activate()
      }, CONTROL_ACTION_HOLD_MS) }
    },
    move(id: number, x: number, y: number): boolean {
      if (pointer?.id !== id) return false
      if (!active && Math.hypot(x - pointer.x, y - pointer.y) > 8) end(true)
      return active
    },
    release(id: number) { if (pointer?.id === id) end(false) },
    cancel() { end(true) },
    isActive() { return active },
  }
}
