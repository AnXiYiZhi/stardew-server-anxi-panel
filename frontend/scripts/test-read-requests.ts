import assert from 'node:assert/strict'
import { clearReadRequests, peekReadRequest, sharedRead } from '../src/core/read-requests.ts'
import { subscribeVisiblePoll } from '../src/core/visible-polling.ts'

const sleep = (ms: number) => new Promise(resolve => setTimeout(resolve, ms))
clearReadRequests()
let calls = 0
let complete!: (value: number) => void
let underlying!: AbortSignal
const read = (signal: AbortSignal) => { calls++; underlying = signal; return new Promise<number>(resolve => { complete = resolve }) }
const one = new AbortController(), two = new AbortController()
const a = sharedRead('same', read, { signal: one.signal, maxAge: 1000 })
const b = sharedRead('same', read, { signal: two.signal, maxAge: 1000 })
await sleep(0)
assert.equal(calls, 1)
one.abort()
await assert.rejects(a, { name: 'AbortError' })
assert.equal(underlying.aborted, false, 'one consumer cannot cancel another')
complete(42)
assert.equal(await b, 42)
assert.equal(await sharedRead('same', read, { maxAge: 1000 }), 42)
assert.equal(peekReadRequest('same'), 42)
assert.equal(calls, 1)
clearReadRequests()
assert.equal(peekReadRequest('same'), undefined)

let staleComplete!: (value: number) => void
const stale = sharedRead('mutation', () => new Promise<number>(resolve => { staleComplete = resolve }), { maxAge: 1000 })
await sleep(0)
clearReadRequests()
assert.equal(await sharedRead('mutation', async () => 2, { maxAge: 1000 }), 2)
staleComplete(1)
await stale
assert.equal(peekReadRequest('mutation'), 2, 'pre-mutation response cannot overwrite a fresh sample')

const never = (signal: AbortSignal) => new Promise<number>((_, reject) => {
  if (signal.aborted) reject(signal.reason)
  else signal.addEventListener('abort', () => reject(signal.reason), { once: true })
})
await assert.rejects(sharedRead('timeout', never, { timeoutMs: 10 }), { name: 'TimeoutError' })
const unmount = new AbortController()
const abandoned = sharedRead('unmount', never, { signal: unmount.signal })
await sleep(0); unmount.abort()
await assert.rejects(abandoned, { name: 'AbortError' })

class FakeDocument extends EventTarget { visibilityState = 'visible' }
const doc = new FakeDocument()
Object.defineProperty(globalThis, 'document', { value: doc, configurable: true })
let polls = 0, active = 0, maxActive = 0, deliveries = 0
const poll = async (signal: AbortSignal) => {
  polls++; active++; maxActive = Math.max(maxActive, active)
  await sleep(5); active--
  if (signal.aborted) throw signal.reason
  return polls
}
const offA = subscribeVisiblePoll('metrics', 15, poll, { data: () => deliveries++ })
const offB = subscribeVisiblePoll('metrics', 15, poll, { data: () => deliveries++ })
await sleep(12)
assert.equal(polls, 1)
assert.equal(deliveries, 2)
doc.visibilityState = 'hidden'; doc.dispatchEvent(new Event('visibilitychange'))
await sleep(45)
assert.equal(polls, 1, 'hidden document must stop display polling')
doc.visibilityState = 'visible'; doc.dispatchEvent(new Event('visibilitychange'))
await sleep(12)
assert.equal(polls, 2)
assert.equal(maxActive, 1)
offA(); offB()
const stoppedAt = polls
await sleep(30)
assert.equal(polls, stoppedAt, 'last unsubscribe must stop the timer')
console.log('read requests: deduplication, cancellation, timeout, invalidation, shared polling and visibility passed')
