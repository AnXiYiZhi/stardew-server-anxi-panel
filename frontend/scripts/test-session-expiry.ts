import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { currentSessionGeneration, markSessionAuthenticated, notifySessionExpired, subscribeSessionExpired } from '../src/auth-session-events.ts'
import { ApiError, request } from '../src/api.ts'

let expirations = 0
const unsubscribe = subscribeSessionExpired(() => { expirations += 1 })
const original = currentSessionGeneration()
notifySessionExpired(original)
notifySessionExpired(original)
assert.equal(expirations, 1, 'simultaneous 401 responses notify once')
const beforeLogin = currentSessionGeneration()
markSessionAuthenticated()
notifySessionExpired(beforeLogin)
assert.equal(expirations, 1, 'an old request cannot invalidate a new login')
notifySessionExpired(currentSessionGeneration())
assert.equal(expirations, 2, 'the new session can expire normally')
unsubscribe()
notifySessionExpired(currentSessionGeneration())
assert.equal(expirations, 2, 'unmounted listeners are removed')

const originalFetch = globalThis.fetch
let protectedExpirations = 0
const stopObserving = subscribeSessionExpired(() => { protectedExpirations += 1 })
try {
  globalThis.fetch = async () => new Response(JSON.stringify({ code: 'unauthorized', message: '请先登录' }), { status: 401 })
  for (const path of ['/api/auth/login', '/api/auth/me', '/api/setup/admin']) {
    await assert.rejects(request(path), (error: unknown) => error instanceof ApiError && error.status === 401)
  }
  assert.equal(protectedExpirations, 0, 'authentication errors remain local to the form')
  await assert.rejects(request('/api/instances'), ApiError)
  assert.equal(protectedExpirations, 1, 'a protected request invalidates the displayed session')

  let completeOldRequest: (response: Response) => void = () => { throw new Error('request not started') }
  globalThis.fetch = async (input) => String(input) === '/api/auth/login'
    ? new Response('{}', { status: 200 })
    : new Promise<Response>((resolve) => { completeOldRequest = resolve })
  const pending = request('/api/instances')
  await request('/api/auth/login', { method: 'POST' })
  completeOldRequest(new Response('{}', { status: 401 }))
  await assert.rejects(pending, ApiError)
  assert.equal(protectedExpirations, 1, 'delayed pre-login responses do not expire the successful login')
} finally {
  globalThis.fetch = originalFetch
  stopObserving()
}

const api = readFileSync(new URL('../src/api.ts', import.meta.url), 'utf8')
assert.ok(api.includes("response.status === 401 && !path.startsWith('/api/auth/') && !path.startsWith('/api/setup/')"))
assert.ok(api.includes('notifySessionExpired(sessionGeneration)'))
assert.ok(api.includes("path === '/api/auth/login' || path === '/api/setup/admin'"))
const app = readFileSync(new URL('../src/App.tsx', import.meta.url), 'utf8')
assert.match(app, /return subscribeSessionExpired\([\s\S]*?setCurrentUser\(null\)[\s\S]*?setView\('login'\)/)
assert.ok(app.includes('登录已失效，请重新登录后继续。'))
console.log('Session expiry regression checks passed.')
