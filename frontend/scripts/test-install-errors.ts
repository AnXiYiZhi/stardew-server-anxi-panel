import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import catalog from '../../backend/internal/games/installerrors/catalog.json' with { type: 'json' }
import { installationFailureMessage, installationRequestErrorMessage, matchInstallationError } from '../src/core/install-error.ts'
import type { Job, JobLog } from '../src/types.ts'
import { reconcileJobSnapshots } from '../src/games/stardew/install-state.ts'

const samples = JSON.parse(readFileSync(new URL('../../backend/internal/games/installerrors/testdata/samples.json', import.meta.url), 'utf8')) as { id: string; line: string }[]
for (const { id, line } of samples) {
  assert.equal(matchInstallationError(line), catalog.rules.find(rule => rule.id === id)?.message, id)
}
assert.equal(samples.length, catalog.rules.length)
const job = { id: 'current', type: 'stardew_install', status: 'failed', targetId: 'world-b', errorMessage: 'SteamCMD install exited with code 5' } as Job
const emptySnapshot = { ...job, errorMessage: null }
assert.equal(reconcileJobSnapshots(emptySnapshot, job).errorMessage, job.errorMessage, 'terminal detail enriches an equal-time list snapshot')
assert.equal(reconcileJobSnapshots(job, emptySnapshot).errorMessage, job.errorMessage, 'late list responses retain the diagnosis')
assert.equal(reconcileJobSnapshots(emptySnapshot, { ...job, id: 'another-job' }).errorMessage, null, 'no cross-job enrichment')
const log = (message: string, sequence = 1, jobId = job.id): JobLog => ({ jobId, sequence, level: 'info', message, createdAt: '' })
assert.equal(installationFailureMessage({ job, logs: [log('Invalid Password')] }), catalog.rules[0].message)
assert.match(installationFailureMessage({ job, logs: [log('Invalid Password', 1, 'previous')] }), /无法确定原因/)
assert.match(installationFailureMessage({ job, logs: [log('Invalid Password'), log('正在自动切换为账号密码完整登录', 2), log('no space left on device', 3)] }), /存储空间/)
assert.match(installationFailureMessage({ job, logs: [log('That Steam Guard code was invalid.'), log('Logged in OK', 2)] }), /无法确定原因/)
assert.match(installationFailureMessage({ job, logs: [log('no space left on device'), log("App '413150' state is 0x402 after update job", 2)] }), /存储空间/)
assert.match(installationFailureMessage({ job, logs: [log('No subscription'), log("Success! App '1007' fully installed", 2)] }), /下载许可/, 'SDK success cannot erase a failed game download')
assert.match(installationFailureMessage({ job: { ...job, errorMessage: 'SteamCMD install finished without success marker' }, logs: [log('No subscription')] }), /下载许可/, 'specific cause wins over missing completion marker')
assert.equal(installationFailureMessage({ job: { ...job, errorMessage: catalog.rules.find(rule => rule.id === 'disk_full')!.message }, logs: [log('Invalid Password')] }), catalog.rules.find(rule => rule.id === 'disk_full')!.message)
assert.equal(installationFailureMessage({ job, stateMessage: 'Steam 验证码不正确，请重新验证。' }), 'Steam 验证码不正确，请重新验证。')
assert.match(installationFailureMessage({ phase: 'credentials_required' }), /账号、密码或验证码/)
assert.match(installationFailureMessage({ job: { ...job, status: 'canceled' }, logs: [log('Invalid Password')] }), /已取消/)
assert.equal(installationFailureMessage({ requiredFilesMissing: true, job, logs: [log('Invalid Password')] }), '游戏运行文件不完整，请重新安装或修复。')
assert.match(installationFailureMessage({ job: { ...job, type: 'stardew_steam_auth' }, stateMessage: '基础安装曾出现错误', logs: [log('Invalid Password')] }), /账号或密码错误/)
assert.match(installationFailureMessage({ job: { ...job, type: 'stardew_steam_auth' }, stateMessage: '基础安装曾出现错误' }), /无法确定原因/)
for (const code of ['5', '126', '127', '137', '139', '143', '254']) {
  assert.match(installationFailureMessage({ error: `container exited with code ${code}` }), new RegExp(`退出码 ${code}`))
}
assert.equal(installationFailureMessage({ error: 'unrecognized failure token=do-not-display' }), catalog.unknown)
assert.match(installationRequestErrorMessage('Failed to fetch'), /无法连接面板服务/)
assert.match(installationRequestErrorMessage('opaque request error token=do-not-display'), /信息不足以确定原因/)
assert.equal(installationRequestErrorMessage('只有管理员可以提交验证码。'), '只有管理员可以提交验证码。')
assert.match(installationFailureMessage({ error: '任务超时。', phase: 'install_timeout' }), /授权或下载未在限定时间内完成/)
console.log(`Install errors: ${samples.length} shared causes, retries, job isolation, cancellation, state precedence and exit codes passed`)
