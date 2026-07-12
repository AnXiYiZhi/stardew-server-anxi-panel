import assert from 'node:assert/strict'
import { classifyPlayerCommandOutcome, hasCommandResultCapability } from '../src/games/stardew/player-command-results.ts'

const base = { commandId: '0123456789abcdef0123456789abcdef', updatedAt: new Date().toISOString() }

assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'queued' }, 'kick', 'Leah').kind, 'processing')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'running' }, 'kick', 'Leah').message, '处理中…')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'succeeded', errorCode: 'ok' }, 'warp-home', 'Leah').kind, 'succeeded')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'failed', errorCode: 'player_not_online' }, 'kick', 'Leah').message, '玩家已经离线，操作未执行。')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'unknown' }, 'approve-auth', 'Leah').kind, 'unconfirmed')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'expired' }, 'approve-auth', 'Leah').message, '无法确认最终结果，请先检查当前游戏状态再决定是否重试。')
assert.equal(hasCommandResultCapability({ command: 'kick', output: '指令已提交', exitCode: 0, durationMs: 1 }), false)
assert.equal(hasCommandResultCapability({ command: 'kick', commandId: base.commandId, status: 'queued', exitCode: 0, durationMs: 1 }), true)
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'succeeded', errorCode: 'ok' }, 'broadcast', '').kind, 'succeeded')
assert.match(classifyPlayerCommandOutcome({ ...base, status: 'succeeded', errorCode: 'ok' }, 'broadcast', '').message, /不保证每个客户端/)
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'succeeded', errorCode: 'ok' }, 'ban', 'Leah').message, '已封禁 Leah。')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'dispatched' }, 'ban', 'Leah').kind, 'dispatched')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'failed', errorCode: 'ambiguous_player' }, 'ban', 'Leah').kind, 'failed')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'unknown' }, 'ban', 'Leah').message, '无法确认最终结果，请先检查当前游戏状态再决定是否重试。')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'dispatched' }, 'trigger-event', '').kind, 'dispatched')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'dispatched' }, 'enable-joja', '').message, '指令已发送，等待游戏处理或需结合游戏状态确认。')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'failed', errorCode: 'no_festival_today' }, 'trigger-event', '').kind, 'failed')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'failed', errorCode: 'admin_promotion_failed' }, 'enable-joja', '').message, '主机的 JunimoServer 管理员权限提升失败，Joja 指令未派发。')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'running' }, 'save-now', '').kind, 'processing')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'succeeded', errorCode: 'ok' }, 'save-now', '').message, '已确认游戏内保存完成。')
assert.equal(classifyPlayerCommandOutcome({ ...base, status: 'failed', errorCode: 'save_timeout' }, 'save-now', '').kind, 'failed')

console.log('player command result state tests passed')
