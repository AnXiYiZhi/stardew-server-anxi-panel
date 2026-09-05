// Notifications contain no credentials. A new login invalidates responses from
// older requests, so a delayed 401 cannot discard a freshly authenticated user.
let sessionGeneration = 0
const listeners = new Set<() => void>()

export function currentSessionGeneration(): number { return sessionGeneration }
export function markSessionAuthenticated(): void { sessionGeneration += 1 }
export function subscribeSessionExpired(listener: () => void): () => void {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}
export function notifySessionExpired(generation: number): void {
  if (generation !== sessionGeneration) return
  sessionGeneration += 1
  for (const listener of listeners) listener()
}
