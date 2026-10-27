import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { createServer } from 'vite'
import { fixturePort } from './fixture-port.mjs'
import { fileURLToPath } from 'node:url'
import { mkdir } from 'node:fs/promises'
import path from 'node:path'

const require = createRequire(import.meta.url)
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const contextModule = "import{createContext,useContext}from'react';const C=createContext(null);export const FixtureProvider=C.Provider;export const usePanelUpdate=()=>useContext(C)"
function textContrast(a, b) {
  const luminance = color => { const rgb = color.match(/[0-9.]+/g).slice(0, 3).map(Number).map(v => { v /= 255; return v <= .04045 ? v / 12.92 : ((v + .055) / 1.055) ** 2.4 }); return .2126 * rgb[0] + .7152 * rgb[1] + .0722 * rgb[2] }
  const values = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return (values[0] + .05) / (values[1] + .05)
}
const entry = [
  "import React from 'react'",
  "import {createRoot} from 'react-dom/client'",
  "import {UpdateDetailsDialog} from '/src/games/stardew/UpdateDetailsDialog.tsx'",
  "import {FixtureProvider} from '/fixture-context'",
  "const f=window.fixture,noop=()=>{},data={updateTheme:f.theme||'day',versionInfo:{version:'0.7.3',runtimeUpgrade:{requiresScopeConfirmation:f.game,serverVersion:f.game?'1.5.0-preview.140':'1.5.0-preview.125',authVersion:f.game?'1.5.0-preview.140-anxi.1':'1.5.0-anxi.2'}},updateDialogOpen:true,updateStatus:{currentVersion:'0.7.3',latestVersion:'v0.8.0',updateAvailable:true,checkStatus:'ok',releaseUrl:'https://example.test/release'},updateDryRun:{phase:'succeeded',targetVersion:'0.8.0',logs:[],capability:{supported:true,conversionRequired:false}},updateApply:null,players:{players:[]},fullStackUpgrade:null,fullStackBusy:false,closeUpdateDialog:noop,refreshUpdateStatus:noop,runUpdateDryRun:noop,prepareFullStack:()=>f.prepared++,applyUpdate:()=>f.applied++,continueReconnect:noop};",
  "if(f.knownTarget)Object.assign(data.versionInfo.runtimeUpgrade,{targetPanelVersion:f.knownTarget,serverVersion:'1.5.0-preview.140',authVersion:'1.5.0-preview.140-anxi.1'});if(f.nextVersion){data.updateStatus.latestVersion='v'+f.nextVersion;data.updateDryRun.targetVersion=f.nextVersion}",
  "data.updateStatus.content={schemaVersion:1,title:'升级准备与更新中心',summary:'先准备好升级，再放心更新游戏实例。',changes:['游戏库和世界列表新增更新入口','直接查看重要提示与升级影响','保留现有存档、Mod 和认证会话'],notices:[],blocked:!!f.blocked};if(f.notice){data.updateStatus.noticeDigest='fixture-notice';data.updateStatus.content.notices=[{level:'warning',title:'请预留维护时间',message:'运行中的世界会保存、备份并重新启动。',requiresConfirmation:true}]}if(f.history)data.updateApply={updateId:'previous',fromVersion:'0.7.2',toVersion:'0.7.3',phase:'succeeded',progress:100,result:'面板升级已完成',logs:[]};",
  "if(f.finishedGame){data.versionInfo.version='0.8.0';Object.assign(data.versionInfo.runtimeUpgrade,{requiresScopeConfirmation:true,targetPanelVersion:'0.8.0'});data.updateStatus.currentVersion='0.8.0';data.updateStatus.latestVersion=f.nextPatch?'v0.8.1':'v0.8.0';data.updateStatus.updateAvailable=!!f.nextPatch;data.updateDryRun.targetVersion=f.nextPatch?'0.8.1':'0.8.0';data.updateApply={updateId:'previous-game',fromVersion:'0.7.3',toVersion:'0.8.0',phase:'succeeded',progress:100,result:'全部世界升级已完成',logs:[]};data.fullStackUpgrade={operationId:'stack_complete',phase:'succeeded',authorizedAt:'2026-10-26T00:00:00Z',panel:{version:'0.8.0'},groups:[]}}",
  "if(f.authorized)data.fullStackUpgrade={operationId:'stack_fixture',phase:f.recovery?'manual_action':'waiting_panel',authorizedAt:'2026-10-25T00:00:00Z',panel:{version:'0.8.0'},groups:[]};data.recoverFullStack=()=>f.recovered++;const {fullStackUpgrade,...dashboardData}=data;",
  "if(f.scopePrepared){data.fullStackUpgrade={operationId:'stack_prepared',phase:'awaiting_confirmation',panel:{version:'0.8.0'},groups:[]};data.confirmFullStack=digest=>{f.confirmed++;f.confirmDigest=digest};data.cancelFullStack=()=>f.cancelled++}",
  "createRoot(document.getElementById('root')).render(React.createElement(FixtureProvider,{value:data},React.createElement(UpdateDetailsDialog,{user:{id:41,role:'admin',username:'admin'},dashboardData})))",
].join('\n')

const server = await createServer({
  root: fileURLToPath(new URL('..', import.meta.url)),
  configFile: false,
  logLevel: 'error',
  server: { host: '127.0.0.1', port: await fixturePort(), strictPort: true },
  plugins: [{
    name: 'upgrade-entry',
    enforce: 'pre',
    resolveId(id, importer) {
      if (id === '/fixture.tsx') return '\0upgrade-entry.tsx'
      if (id === '/fixture-context' || id === './PanelUpdateProvider' && /FullStackUpgradePanel|UpdateDetailsDialog/.test(importer || '')) return '\0upgrade-context'
    },
    load(id) {
      if (id === '\0upgrade-entry.tsx') return entry
      if (id === '\0upgrade-context') return contextModule
    },
    configureServer(s) {
      s.middlewares.use((req, res, next) => {
        if (req.url !== '/upgrade-entry') return next()
        res.setHeader('Content-Type', 'text/html')
        res.end('<!doctype html><meta name="viewport" content="width=device-width,initial-scale=1"><title>Upgrade entry</title><div id="root"></div><script type="module" src="/fixture.tsx"></script>')
      })
    },
  }],
})
await server.listen()
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_CHANNEL || 'chrome', headless: true })
const results = []
try {
  for (const scenario of [{ name: 'ordinary Panel patch', game: false }, { name: 'junimo140', game: true }, { name: 'bridge before major', game: false, knownTarget: '0.8.0', nextVersion: '0.7.4' }, { name: 'bridge to declared major', game: false, knownTarget: '0.8.0', expectGame: true }, { name: 'unknown future release', game: true, knownTarget: '0.8.0', nextVersion: '0.8.1', expectGame: false }, { name: 'authorized reconnect', game: false, authorized: true }, { name: 'important notice', game: false, notice: true }, { name: 'blocked release', game: false, blocked: true }, { name: 'successful history', game: false, history: true }, { name: 'completed game history', game: false, finishedGame: true, notice: true }, { name: 'patch after game upgrade', game: false, finishedGame: true, nextPatch: true }, { name: 'blocked release retains authorized recovery', game: false, authorized: true, blocked: true, recovery: true }, { name: 'prepared scope requires notice confirmation', game: true, notice: true, scopePrepared: true }, { name: 'night important notice', game: false, notice: true, theme: 'night' }]) {
    const { game, authorized = false, expectGame = game } = scenario
    const context = await browser.newContext({ viewport: { width: 390, height: 844 } })
    const page = await context.newPage()
    page.setDefaultTimeout(10000)
    const errors = []
    page.on('pageerror', error => errors.push(error.message))
    await page.addInitScript(fixture => { window.fixture = { ...fixture, applied: 0, prepared: 0, recovered: 0, confirmed: 0, cancelled: 0 } }, scenario)
    await page.goto('http://127.0.0.1:' + server.httpServer.address().port + '/upgrade-entry', { waitUntil: 'domcontentloaded' })
    await page.getByRole('dialog').waitFor()
    assert.equal(await page.title(), 'Upgrade entry')
    assert.equal(await page.getByRole('heading', { name: '更新中心', exact: true }).isVisible(), true)
    assert.equal(await page.locator('.sd-update-timeline').count(), 0)
    if (scenario.theme === 'night') {
      assert.equal(await page.getByRole('dialog').getAttribute('data-theme'), 'night')
      const colors = await page.locator('.sd-update-release-notes a').evaluate(e => ({ foreground: getComputedStyle(e).color, background: getComputedStyle(e.closest('.sd-update-center')).backgroundColor }))
      assert.ok(textContrast(colors.foreground, colors.background) >= 4.5, 'Night release link remains readable')
    }
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth), true)
    if (scenario.notice && !scenario.finishedGame) {
      assert.equal(await page.getByRole('button', { name: scenario.scopePrepared ? '确认范围并升级' : '立即升级', exact: true }).isDisabled(), true)
      await page.getByRole('checkbox', { name: '我已阅读并理解以上重要提示' }).check()
      assert.equal(await page.getByRole('button', { name: scenario.scopePrepared ? '确认范围并升级' : '立即升级', exact: true }).isEnabled(), true)
    }
    if (scenario.history) assert.equal(await page.locator('.sd-update-history').getAttribute('open'), null)
    if (process.env.QA_ARTIFACT_DIR && ['important notice','blocked release','successful history','completed game history'].includes(scenario.name)) {
      assert.ok(path.isAbsolute(process.env.QA_ARTIFACT_DIR))
      await mkdir(process.env.QA_ARTIFACT_DIR, { recursive: true })
      const name = scenario.name.replaceAll(' ', '-')
      await page.screenshot({ path: path.join(process.env.QA_ARTIFACT_DIR, name + '-mobile.png'), fullPage: false })
      await page.setViewportSize({ width: 1440, height: 1000 })
      await page.screenshot({ path: path.join(process.env.QA_ARTIFACT_DIR, name + '-desktop.png'), fullPage: false })
    }
    if (scenario.scopePrepared) {
      assert.equal(await page.locator('.sd-update-center-footer .sd-update-apply-button').count(), 1)
      assert.equal(await page.locator('.sd-update-center-body .sd-update-actions').count(), 0)
      await page.getByRole('button', { name: '确认范围并升级', exact: true }).click()
      assert.deepEqual(await page.evaluate(() => ({ confirmed: window.fixture.confirmed, digest: window.fixture.confirmDigest, applied: window.fixture.applied })), { confirmed: 1, digest: 'fixture-notice', applied: 0 })
    } else if (scenario.recovery) {
      await page.getByRole('button', { name: '继续恢复', exact: true }).click()
      assert.equal(await page.evaluate(() => window.fixture.recovered), 1)
      assert.equal(await page.evaluate(() => window.fixture.applied + window.fixture.prepared), 0)
    } else if (scenario.finishedGame && !scenario.nextPatch) {
      assert.equal(await page.getByRole('checkbox').count(), 0)
      assert.equal(await page.getByText('已是最新版本', { exact: true }).isVisible(), true)
      assert.equal(await page.locator('.sd-update-history').getAttribute('open'), null)
      assert.equal(await page.getByRole('button', { name: '检查升级范围', exact: true }).count(), 0)
      assert.equal(await page.getByRole('button', { name: '立即升级', exact: true }).count(), 0)
    } else if (scenario.blocked) {
      assert.equal(await page.getByRole('button', { name: '立即升级', exact: true }).count(), 0)
      assert.equal(await page.getByRole('alert').isVisible(), true)
    } else if (authorized) {
      assert.equal(await page.getByRole('button', { name: '立即升级', exact: true }).count(), 0)
      assert.equal(await page.getByRole('button', { name: '升级中…', exact: true }).isDisabled(), true)
      assert.deepEqual(await page.evaluate(() => ({ applied: window.fixture.applied, prepared: window.fixture.prepared })), { applied: 0, prepared: 0 })
    } else if (expectGame) {
      assert.equal(await page.getByRole('button', { name: '立即升级', exact: true }).count(), 0)
      await page.getByRole('button', { name: '检查升级范围', exact: true }).click()
      assert.deepEqual(await page.evaluate(() => ({ applied: window.fixture.applied, prepared: window.fixture.prepared })), { applied: 0, prepared: 1 })
    } else {
      assert.equal(await page.getByRole('button', { name: '检查升级范围', exact: true }).count(), 0)
      await page.getByRole('button', { name: '立即升级', exact: true }).click()
      await page.getByRole('alertdialog').getByRole('button', { name: '确认升级', exact: true }).click()
      assert.deepEqual(await page.evaluate(() => ({ applied: window.fixture.applied, prepared: window.fixture.prepared })), { applied: 1, prepared: 0 })
    }
    assert.deepEqual(errors, [])
    results.push({ profile: scenario.name, passed: true })
    await context.close()
  }
  console.log(JSON.stringify({ browser: 'Browser plugin not available; Playwright', results }))
} finally {
  await browser.close()
  await server.close()
}
