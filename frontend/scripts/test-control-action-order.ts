import assert from 'node:assert/strict'
import {
  CONTROL_ACTION_IDS, CONTROL_ACTION_HOLD_MS, controlActionOrderKey,
  controlReorderGesture, moveControlAction, readControlActionOrder,
} from '../src/games/stardew/mobile/control-action-order.ts'

for (const raw of [null, '', '{', 'null', '[]', '{"version":2,"order":["save"]}']) {
  assert.deepEqual(readControlActionOrder(raw), CONTROL_ACTION_IDS, `invalid preference: ${raw}`)
}
assert.deepEqual(readControlActionOrder('{"version":1,"order":["save","save",null,"removed","language"]}'),
  ['save', 'language', 'schedule', 'auth', 'runtime', 'festival', 'joja'])
assert.deepEqual(readControlActionOrder('{"version":1,"order":{}}'), CONTROL_ACTION_IDS)
assert.notEqual(controlActionOrderKey(1, 'stardew'), controlActionOrderKey(2, 'stardew'))
assert.notEqual(controlActionOrderKey(1, 'stardew'), controlActionOrderKey(1, 'river-farm'))
assert.notEqual(controlActionOrderKey(1, 'farm/a'), controlActionOrderKey(1, 'farm%2Fa'))

const original = [...CONTROL_ACTION_IDS]
const moved = moveControlAction(original, 'joja', 0)
assert.deepEqual(moved, ['joja', 'language', 'schedule', 'auth', 'runtime', 'save', 'festival'])
assert.deepEqual(original, CONTROL_ACTION_IDS, 'moving does not mutate the rollback snapshot')
assert.deepEqual(moveControlAction(moved, 'joja', 100), original)
assert.deepEqual(moveControlAction(original, 'language', -1), original)
assert.deepEqual(moveControlAction(original, 'save', NaN), original)
assert.deepEqual(readControlActionOrder(JSON.stringify({ version: 1, order: moved })), moved)

function fixture() {
  let activated = 0
  const callbacks: (() => void)[] = []
  const finished: { cancelled: boolean; active: boolean }[] = []
  const cancelledTimers: number[] = []
  const gesture = controlReorderGesture((run, ms) => {
    assert.equal(ms, CONTROL_ACTION_HOLD_MS)
    const index = callbacks.push(run) - 1
    return () => { cancelledTimers.push(index) }
  }, () => { activated++ }, (cancelled, active) => { finished.push({ cancelled, active }) })
  return { gesture, callbacks, finished, cancelledTimers, activations: () => activated }
}

for (const reason of ['short-tap', 'scroll-movement', 'scroll-event', 'touchcancel', 'blur', 'unmount']) {
  const f = fixture()
  f.gesture.start(1, 20, 20)
  assert.equal(f.gesture.isActive(), false)
  if (reason === 'short-tap') f.gesture.release(1)
  else if (reason === 'scroll-movement') f.gesture.move(1, 29, 20)
  else f.gesture.cancel()
  f.callbacks[0]() // A timer queued before cancellation must stay inert.
  assert.equal(f.activations(), 0, reason)
  assert.equal(f.gesture.isActive(), false, reason)
  assert.deepEqual(f.finished, [{ cancelled: reason !== 'short-tap', active: false }], reason)
  assert.deepEqual(f.cancelledTimers, [0], reason)
}

const f = fixture()
f.gesture.start(1, 20, 20)
f.gesture.move(2, 80, 80)
f.gesture.release(2)
f.gesture.move(1, 28, 20) // Small finger jitter is tolerated.
f.callbacks[0]()
f.callbacks[0]()
assert.equal(f.activations(), 1, 'a held card activates once')
assert.equal(f.gesture.move(1, 20, 500), true, 'active drag can cross multiple cards')
assert.equal(f.gesture.move(2, 20, 500), false, 'unrelated pointer cannot move it')
f.gesture.release(2)
assert.equal(f.gesture.isActive(), true)
f.gesture.release(1)
assert.deepEqual(f.finished, [{ cancelled: false, active: true }], 'release commits an active drag')
f.gesture.cancel()
assert.equal(f.finished.length, 1, 'release cleanup runs once')

const cancelled = fixture()
cancelled.gesture.start(3, 0, 0)
cancelled.callbacks[0]()
cancelled.gesture.cancel()
assert.deepEqual(cancelled.finished, [{ cancelled: true, active: true }], 'active cancel requests rollback')
cancelled.callbacks[0]()
assert.equal(cancelled.activations(), 1)

const replaced = fixture()
replaced.gesture.start(4, 0, 0)
replaced.gesture.start(5, 0, 0)
replaced.callbacks[0]()
assert.equal(replaced.activations(), 0, 'replaced hold cannot activate the next card')
replaced.callbacks[1]()
assert.equal(replaced.activations(), 1)
replaced.gesture.release(5)
assert.deepEqual(replaced.finished, [{ cancelled: true, active: false }, { cancelled: false, active: true }])

console.log('Control action order: persistence, isolation, normalization, hold, scrolling, cancellation and rollback passed')
