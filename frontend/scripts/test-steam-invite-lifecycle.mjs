import assert from 'node:assert/strict'
import { createRequire } from 'node:module'
import { readFileSync } from 'node:fs'
import { mkdir } from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createServer } from 'vite'
import { fixturePort } from './fixture-port.mjs'

// Runs the real hook in React StrictMode and a real DOM. PLAYWRIGHT_MODULE may
// point to an existing workspace runtime; the application gains no dependency.
const require = createRequire(import.meta.url)
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const root = fileURLToPath(new URL('..', import.meta.url))
const entry = [
  "import React, { StrictMode, useState } from 'react'",
  "import { createRoot } from 'react-dom/client'",
  "import { useStardewDashboardData } from '/src/games/stardew/useStardewDashboardData.ts'",
  "import { markSessionAuthenticated } from '/src/auth-session-events.ts'",
  "function Probe({ id }) { const data = useStardewDashboardData(id); return React.createElement('output',null,JSON.stringify({code:data.inviteCode,status:data.inviteCodeStatus})) }",
  "function App() { const [id,setId]=useState('a'); const [on,setOn]=useState(true); window.fixture.switch=setId; window.fixture.unmount=()=>setOn(false); window.fixture.logout=()=>{markSessionAuthenticated();setOn(false)}; return on ? React.createElement(Probe,{id}) : React.createElement('output',null,'unmounted') }",
  "createRoot(document.getElementById('root')).render(React.createElement(StrictMode,null,React.createElement(App)))",
].join('\n')
const server = await createServer({
  root, configFile: false, logLevel: 'error', server: { host: '127.0.0.1', port: await fixturePort(), strictPort: true },
  plugins: [{
    name: 'invite-lifecycle-fixture',
    enforce: 'pre',
    resolveId(id, importer) {
      if (id === '/fixture.tsx') return '\0invite-entry.tsx'
      if (id === './PanelUpdateProvider' && importer?.includes('useStardewDashboardData')) return '\0panel-update-stub'
    },
    load(id) {
      if (process.env.INVITE_TEST_SOURCE && id.endsWith('/src/games/stardew/useStardewDashboardData.ts')) return readFileSync(process.env.INVITE_TEST_SOURCE, 'utf8')
      if (id === '\0invite-entry.tsx') return entry
      if (id === '\0panel-update-stub') return 'export const usePanelUpdate = () => ({})'
    },
    configureServer(s) {
      s.middlewares.use((req, res, next) => {
        if (req.url !== '/invite-test') return next()
        res.setHeader('Content-Type', 'text/html')
        res.end('<!doctype html><title>Invite lifecycle</title><div id="root"></div><script type="module" src="/fixture.tsx"></script>')
      })
    },
  }],
})
await server.listen()
const address = server.httpServer.address()
const browser = await chromium.launch({ channel: process.env.PLAYWRIGHT_CHANNEL || 'chrome', headless: true })
const results = []
try {
  for (const scenario of ['hidden cached first load', 'hide before state', 'hide during invite', 'live revoke and recover', 'switch stale response', 'logout stale response', 'unmount stale response', 'legacy ready', 'strict visible repeats']) {
    const context = await browser.newContext({ viewport: scenario === 'live revoke and recover' ? {width:390,height:844} : {width:1473,height:1033} })
    const page = await context.newPage()
    const errors = []
    page.on('pageerror', e => errors.push(e.message))
    let stateGets = 0, inviteGets = 0, active = 0, maxActive = 0
    let snapshot = { steamInviteEnabled: true, status: scenario === 'legacy ready' ? 'ready' : 'connection_ready', inviteCode: 'SLIVE' }
    let releaseState, releaseInvite
    const stateGate = scenario === 'hide before state' ? new Promise(r => { releaseState = r }) : Promise.resolve()
    const inviteGate = scenario.includes('stale response') || scenario === 'hide during invite' ? new Promise(r => { releaseInvite = r }) : Promise.resolve()
    await page.addInitScript(({ hidden }) => {
      let visibility = hidden ? 'hidden' : 'visible'
      window.fixture = { visibility: value => { visibility = value; document.dispatchEvent(new Event('visibilitychange')) } }
      Object.defineProperty(document, 'visibilityState', { get: () => visibility })
      const original = window.setTimeout
      window.setTimeout = (fn, ms, ...args) => original(fn, ms === 5000 ? 80 : ms === 30000 ? 500 : ms, ...args)
    }, { hidden: scenario === 'hidden cached first load' })
    await page.route('**/api/**', async route => {
      const path = new URL(route.request().url()).pathname
      let body = {}
      if (path.endsWith('/state')) {
        stateGets++
        await stateGate
        body = { state:'running', steamInviteEnabled:true, steamInviteAuthState:'ready', inviteCode:'SCACHED' }
      } else if (path.endsWith('/invite-code')) {
        inviteGets++; active++; maxActive = Math.max(maxActive, active)
        if (inviteGets === 1) await inviteGate
        body = { ...snapshot, inviteCode: path.includes('/b/') ? 'SB' : snapshot.inviteCode }
        active--
      } else if (path.endsWith('/jobs')) body = { jobs:[] }
      else if (path.endsWith('/saves')) body = { saves:[],activeSaveName:'' }
      else if (path.endsWith('/mods')) body = { mods:[] }
      else if (path.endsWith('/players')) body = { players:[] }
      else if (path.endsWith('/direct-connect')) body = { gamePort:24642 }
      await route.fulfill({ status:200, contentType:'application/json', body:JSON.stringify(body) }).catch(() => {})
    })
    await page.goto('http://127.0.0.1:' + address.port + '/invite-test', { waitUntil:'domcontentloaded' })
    if (scenario === 'hide before state') {
      await page.waitForFunction(() => Boolean(window.fixture.switch))
      await page.evaluate(() => window.fixture.visibility('hidden'))
      releaseState()
    }
    if (scenario.startsWith('hidden') || scenario === 'hide before state') {
      await page.waitForFunction(() => document.querySelector('output')?.textContent.includes('status'))
      await page.waitForTimeout(250)
      assert.equal(inviteGets, 0, 'hidden initial load must pause invite requests')
      assert.ok(!await page.locator('output').textContent().then(t => t.includes('"status":"ready"')), 'cached code is not live readiness')
      await page.evaluate(() => window.fixture.visibility('visible'))
    } else if (releaseInvite) {
      await page.waitForFunction(() => Boolean(window.fixture.switch))
      for (let n=0; inviteGets===0 && n<100; n++) await page.waitForTimeout(10)
      assert.ok(inviteGets > 0, 'first invite request started')
      if (scenario === 'hide during invite') await page.evaluate(() => window.fixture.visibility('hidden'))
      if (scenario === 'switch stale response') await page.evaluate(() => window.fixture.switch('b'))
      if (scenario === 'logout stale response') await page.evaluate(() => window.fixture.logout())
      if (scenario === 'unmount stale response') await page.evaluate(() => window.fixture.unmount())
      await page.waitForTimeout(100)
      releaseInvite()
      if (scenario === 'hide during invite') {
        await page.waitForTimeout(180)
        assert.ok(!await page.locator('output').textContent().then(t => t.includes('SLIVE')), 'hidden response must not project')
        await page.evaluate(() => window.fixture.visibility('visible'))
      }
    }
    if (scenario === 'logout stale response' || scenario === 'unmount stale response') {
      await page.waitForTimeout(250)
      assert.equal(await page.locator('output').textContent(), 'unmounted')
      const stopped = inviteGets
      await page.evaluate(() => window.fixture.visibility('visible'))
      await page.waitForTimeout(200)
      assert.equal(inviteGets, stopped)
    } else {
      await page.waitForFunction(expected => document.querySelector('output')?.textContent.includes(expected), scenario === 'switch stale response' ? 'SB' : 'SLIVE', { timeout:4000 })
      if (scenario === 'strict visible repeats') assert.equal(inviteGets, 1, 'StrictMode initial live request is deduplicated')
      if (scenario === 'live revoke and recover') {
        snapshot = { ...snapshot, status:'steam_session_down',inviteCode:'' }
        await page.waitForFunction(() => document.querySelector('output')?.textContent.includes('steam_session_down'))
        assert.ok(!await page.locator('output').textContent().then(t => t.includes('SLIVE')))
        snapshot = { ...snapshot, status:'invite_unavailable' }
        await page.waitForFunction(() => document.querySelector('output')?.textContent.includes('invite_unavailable'))
        snapshot = { ...snapshot, status:'steam_relay_pending',inviteCode:'SNEW' }
        await page.waitForFunction(() => document.querySelector('output')?.textContent.includes('SNEW'))
        snapshot = { ...snapshot, status:'connection_ready' }
        await page.waitForFunction(() => document.querySelector('output')?.textContent.includes('connection_ready'))
      }
      const before = inviteGets
      if (scenario === 'legacy ready') {
        await page.waitForTimeout(400)
        assert.equal(inviteGets, before, '.125 confirmed ready stops network polling')
      } else {
        await page.evaluate(() => { for(let n=0;n<8;n++) window.fixture.visibility('visible') })
        await page.waitForTimeout(200)
        assert.ok(inviteGets - before <= 4, 'repeated visible events must not multiply pollers')
      }
      assert.equal(maxActive, 1, 'one invite request at a time')
    }
    assert.deepEqual(errors, [], 'real React/DOM has no runtime error')
    assert.equal(await page.title(), 'Invite lifecycle')
    if (process.env.INVITE_EVIDENCE_DIR && (scenario === 'hidden cached first load' || scenario === 'live revoke and recover')) {
      await mkdir(process.env.INVITE_EVIDENCE_DIR, {recursive:true})
      await page.screenshot({path:path.join(process.env.INVITE_EVIDENCE_DIR, scenario.replaceAll(' ','-')+'.png'),fullPage:false})
    }
    results.push({ scenario, stateGets, inviteGets, maxActive, passed:true })
    console.log(JSON.stringify(results.at(-1)))
    await context.close()
  }
} finally {
  await browser.close()
  await server.close()
}
console.log('Real React StrictMode invite visibility lifecycle passed: ' + results.length)
