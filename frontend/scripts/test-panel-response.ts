import assert from 'node:assert/strict'
import { measurePanelResponse, panelResponsePresentation } from '../src/games/stardew/panel-response.ts'

const controller = new AbortController()
let now = 10
let requested = ''
const fetcher: typeof fetch = async (input, init) => {
  requested = String(input)
  assert.equal(init?.cache, 'no-store')
  assert.equal(init?.credentials, 'same-origin')
  assert.equal(init?.redirect, 'error')
  assert.equal(init?.signal, controller.signal)
  now = 30
  return {
    status: 200, redirected: false,
    text: async () => { now = 155; return JSON.stringify({ version: '0.7.0' }) },
  } as Response
}
assert.equal(await measurePanelResponse(controller.signal, fetcher, () => now), 145)
assert.match(requested, /^\/api\/version\?panel-response=\d+$/)

for (const response of [
  new Response('{}', { status: 503 }),
  new Response('<html>login</html>'),
  new Response('{}'),
  new Response('{"version":null}'),
  new Response('{"version":""}'),
  { status: 200, redirected: true } as Response,
]) {
  await assert.rejects(measurePanelResponse(controller.signal, async () => response))
}
await assert.rejects(measurePanelResponse(controller.signal, async () => { throw new TypeError('offline') }))
controller.abort()
await assert.rejects(measurePanelResponse(controller.signal, async () => new Response('{"version":"dev"}')))

for (const [ms, text, level] of [
  [0, '<1 ms', 'ok'], [0.5, '<1 ms', 'ok'], [42.4, '42 ms', 'ok'],
  [100, '100 ms', 'warn'], [300, '300 ms', 'crit'],
] as const) {
  assert.deepEqual(panelResponsePresentation({ status: 'ok', milliseconds: ms }), { text, level })
}
for (const status of ['loading', 'timeout', 'error', 'paused'] as const) {
  assert.equal(panelResponsePresentation({ status }).level, 'unknown')
  assert.doesNotMatch(panelResponsePresentation({ status }).text, /ms/)
}
console.log('Panel response: complete HTTP timing, cache bypass, validation, cancellation and display states passed')
