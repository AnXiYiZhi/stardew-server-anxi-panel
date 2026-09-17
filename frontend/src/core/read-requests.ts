type Flight = { controller: AbortController; promise: Promise<unknown>; users: number }
type Sample = { value: unknown; expires: number; retainUntil: number }

const flights = new Map<string, Flight>()
const samples = new Map<string, Sample>()
let revision = 0

export function clearReadRequests() {
  revision++
  samples.clear()
  // Keep existing callers alive, but future reads cannot join pre-mutation work.
}

export function peekReadRequest<T>(key: string): T | undefined {
  const sample = samples.get(key)
  if (!sample || Date.now() > sample.retainUntil) return undefined
  return sample.value as T
}

export function sharedRead<T>(key: string, read: (signal: AbortSignal) => Promise<T>, options: { signal?: AbortSignal | null; maxAge?: number; timeoutMs?: number } = {}): Promise<T> {
  if (options.signal?.aborted) return Promise.reject(options.signal.reason ?? new DOMException('Aborted', 'AbortError'))
  const sample = samples.get(key)
  if (sample && sample.expires > Date.now()) return Promise.resolve(sample.value as T)
  const generation = revision
  const flightKey = `${generation}:${key}`
  let flight = flights.get(flightKey)
  if (!flight || flight.controller.signal.aborted) {
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(new DOMException('请求超时，请重试', 'TimeoutError')), options.timeoutMs ?? 20_000)
    flight = { controller, users: 0, promise: Promise.resolve() }
    const current = flight
    current.promise = Promise.resolve().then(() => read(controller.signal)).then(value => {
      if (generation === revision && !controller.signal.aborted && (options.maxAge ?? 0) > 0) {
        for (const [id, old] of samples) if (old.retainUntil < Date.now()) samples.delete(id)
        if (samples.size >= 128) samples.delete(samples.keys().next().value!)
        samples.set(key, { value, expires: Date.now() + (options.maxAge ?? 0), retainUntil: Date.now() + 5 * 60_000 })
      }
      return value
    }).finally(() => {
      clearTimeout(timer)
      if (flights.get(flightKey) === current) flights.delete(flightKey)
    })
    flights.set(flightKey, current)
  }
  const current = flight
  current.users++
  return new Promise<T>((resolve, reject) => {
    let finished = false
    const finish = () => {
      if (finished) return false
      finished = true
      options.signal?.removeEventListener('abort', abort)
      current.users--
      return true
    }
    const abort = () => {
      if (!finish()) return
      reject(options.signal?.reason ?? new DOMException('Aborted', 'AbortError'))
      if (current.users === 0) current.controller.abort()
    }
    options.signal?.addEventListener('abort', abort, { once: true })
    current.promise.then(value => { if (finish()) resolve(value as T) }, error => { if (finish()) reject(error) })
  })
}
