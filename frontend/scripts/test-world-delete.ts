import assert from 'node:assert/strict'
import { canDeleteWorld, worldDeleteGesture, WORLD_DELETE_HOLD_MS } from '../src/games/world-delete-gesture.ts'

assert.equal(canDeleteWorld('admin', true), false)
assert.equal(canDeleteWorld('admin', undefined), false)
assert.equal(canDeleteWorld('user', false), false)
assert.equal(canDeleteWorld('admin', false), true)
for (const action of ['release', 'move', 'leave', 'scroll', 'cancel', 'blur']) {
  let now = 0, ready = 0, pressing = false
  let callback = () => {}
  const g = worldDeleteGesture((run, ms) => { assert.equal(ms, WORLD_DELETE_HOLD_MS); callback = run; return () => {} }, () => ready++, active => { pressing = active }, () => now)
  g.start(1, 10, 10)
  assert.equal(pressing, true)
  now = 500
  if (action === 'release') g.release()
  else if (action === 'move') g.move(1, 30, 10)
  else g.cancel()
  callback() // Even an already queued callback cannot open confirmation.
  assert.equal(ready, 0, action)
  assert.equal(pressing, false, action)
  assert.equal(g.consumeClick(), true, action)
}
let ready = 0, run = () => {}, now = 0
const g = worldDeleteGesture(callback => { run = callback; return () => {} }, () => ready++, () => {}, () => now)
g.start(1, 0, 0); now = 50; g.release(); assert.equal(g.consumeClick(), false, 'normal tap enters')
g.start(1, 0, 0); run(); g.release(); assert.equal(ready, 1); assert.equal(g.consumeClick(), true)
g.start(2, 0, 0); const stale = run; g.start(3, 0, 0); stale(); assert.equal(ready, 1); run(); assert.equal(ready, 2)
console.log('World deletion gesture and default/admin protection passed')
