import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { webcrypto } from 'node:crypto'
import vm from 'node:vm'

const extensionRoot = new URL('../../browser-extensions/nexus-slow-installer/', import.meta.url)
const sharedSource = readFileSync(new URL('shared.js', extensionRoot), 'utf8')
const backgroundSource = readFileSync(new URL('background.js', extensionRoot), 'utf8')
const panelBridgeSource = readFileSync(new URL('panel-bridge.js', extensionRoot), 'utf8')
const manifest = JSON.parse(readFileSync(new URL('manifest.json', extensionRoot), 'utf8'))

const CONFIG_KEY = 'anxiNexusInstallerConfig'
const STATE_KEY = 'anxiNexusInstallerState'
const downloadURL = 'https://supporter-files.nexus-cdn.com/mods/1303/123/example.zip?expires=1&key=secret'

function clone(value) {
  return value === undefined ? undefined : structuredClone(value)
}

function jsonResponse(body, status = 202) {
  return {
    ok: status >= 200 && status < 300,
    status,
    async text() {
      return JSON.stringify(body)
    },
  }
}

function storageArea(seed) {
  const values = clone(seed)
  return {
    values,
    api: {
      async get(keys) {
        if (typeof keys === 'string') return { [keys]: clone(values[keys]) }
        if (Array.isArray(keys)) {
          return Object.fromEntries(keys.map((key) => [key, clone(values[key])]))
        }
        return clone(values)
      },
      async set(patch) {
        Object.assign(values, clone(patch))
      },
    },
  }
}

function extensionConfig() {
  return {
    panelBaseUrl: 'https://panel.example.test',
    instanceId: 'stardew',
    autoStartOnNexusFilePage: true,
    autoClickSlowDownload: true,
    cancelBrowserDownload: true,
  }
}

function captureState({ key = 'tab:1', requestId = 'request-1', modId = 100, fileId = 10 } = {}) {
  const capture = {
    active: true,
    captureKey: key,
    requestId,
    batchId: '',
    itemId: '',
    autoSubmit: false,
    closeTabOnComplete: false,
    createdAt: 1_700_000_000_000,
    expiresAt: Date.now() + 10 * 60 * 1000,
    tabId: Number(key.split(':')[1] || 1),
    gameDomain: 'stardewvalley',
    modId,
    fileId,
    modName: 'Example',
    pageUrl: `https://www.nexusmods.com/stardewvalley/mods/${modId}?file_id=${fileId}`,
    pendingUrl: downloadURL,
  }
  return { captures: { [key]: capture }, capture }
}

function backgroundHarness({ seedState = captureState(), fetchImpl, tabMessageImpl } = {}) {
  const storage = storageArea({
    [CONFIG_KEY]: extensionConfig(),
    [STATE_KEY]: seedState,
  })
  const runtimeListeners = []
  const context = {
    AbortController,
    URL,
    Uint8Array,
    console,
    crypto: webcrypto,
    fetch: (...args) => fetchImpl(...args),
    setTimeout,
    clearTimeout,
    importScripts() {},
    chrome: {
      storage: { local: storage.api },
      notifications: { async create() {} },
      downloads: {
        async cancel() {},
        async erase() {},
        onCreated: { addListener() {} },
      },
      tabs: {
        async create() { return { id: 99 } },
        async remove() {},
        async sendMessage(...args) {
          if (tabMessageImpl) return tabMessageImpl(...args)
          return { ok: true }
        },
      },
      runtime: {
        onInstalled: { addListener() {} },
        onMessage: { addListener(listener) { runtimeListeners.push(listener) } },
      },
    },
  }
  context.globalThis = context
  vm.createContext(context)
  vm.runInContext(sharedSource, context, { filename: 'shared.js' })
  vm.runInContext(backgroundSource, context, { filename: 'background.js' })
  return { context, runtimeListeners, storage: storage.values }
}

async function waitFor(predicate, message) {
  const deadline = Date.now() + 2000
  while (Date.now() < deadline) {
    if (predicate()) return
    await new Promise((resolve) => setTimeout(resolve, 0))
  }
  throw new Error(message)
}

async function testBackgroundSingleflight() {
  const requests = []
  let releaseFetch
  const fetchGate = new Promise((resolve) => { releaseFetch = resolve })
  const harness = backgroundHarness({
    fetchImpl: async (url, options) => {
      requests.push({ url, options })
      await fetchGate
      return jsonResponse({ jobId: 'job-singleflight' })
    },
  })

  const calls = Array.from({ length: 20 }, () => harness.context.finishInstall(downloadURL, 'tab:1'))
  await waitFor(() => requests.length === 1, 'background did not start its first POST')
  assert.equal(requests.length, 1, 'concurrent finishInstall calls must share one POST')
  releaseFetch()
  const results = await Promise.all(calls)

  assert.ok(results.every((result) => result.jobId === 'job-singleflight'))
  assert.ok(results.some((result) => result.deduped), 'followers should report that the POST was shared')
  assert.equal(requests[0].options.headers['Idempotency-Key'], 'request-1')
  assert.equal(requests[0].options.headers['X-Anxi-Nexus-Installer'], '0.1.3')
  assert.equal(harness.storage[STATE_KEY].captures['tab:1'].active, false)
}

async function testFailureRetryAndWorkerRestart() {
  const failedHeaders = []
  const firstWorker = backgroundHarness({
    fetchImpl: async (_url, options) => {
      failedHeaders.push(options.headers)
      throw new Error('simulated connection loss')
    },
  })
  await assert.rejects(
    firstWorker.context.finishInstall(downloadURL, 'tab:1'),
    /simulated connection loss/,
  )
  const failedCapture = firstWorker.storage[STATE_KEY].captures['tab:1']
  assert.equal(failedCapture.active, true, 'a failed POST must remain retryable')
  assert.equal(failedCapture.autoSubmitting, false)
  assert.equal(failedCapture.requestId, 'request-1')

  let sameWorkerRetries = 0
  firstWorker.context.fetch = async (_url, options) => {
    sameWorkerRetries++
    failedHeaders.push(options.headers)
    return jsonResponse({ jobId: 'job-same-worker-retry', deduped: true })
  }
  const sameWorkerRetry = await firstWorker.context.finishInstall(downloadURL, 'tab:1')
  assert.equal(sameWorkerRetry.jobId, 'job-same-worker-retry')
  assert.equal(sameWorkerRetries, 1, 'a rejected singleflight must be removed immediately')
  assert.equal(failedHeaders[1]['Idempotency-Key'], failedHeaders[0]['Idempotency-Key'])

  const restartSeed = captureState({ requestId: failedCapture.requestId })
  const retryHeaders = []
  const restartedWorker = backgroundHarness({
    seedState: restartSeed,
    fetchImpl: async (_url, options) => {
      retryHeaders.push(options.headers)
      return jsonResponse({ jobId: 'job-recovered', deduped: true })
    },
  })
  const retry = await restartedWorker.context.finishInstall(downloadURL, 'tab:1')
  assert.equal(retry.jobId, 'job-recovered')
  assert.equal(retry.deduped, true)
  assert.equal(retryHeaders[0]['Idempotency-Key'], failedHeaders[0]['Idempotency-Key'])
}

async function testCaptureIdentityRotation() {
  const harness = backgroundHarness({
    seedState: { captures: {}, capture: null },
    fetchImpl: async () => jsonResponse({ jobId: 'unused' }),
  })
  const sender = { tab: { id: 7, url: 'https://www.nexusmods.com/stardewvalley/mods/100' } }
  const first = await harness.context.startCapture({ modId: 100, fileId: 10 }, sender)
  const reinjected = await harness.context.startCapture({ modId: 100, fileId: 10 }, sender)
  const differentFile = await harness.context.startCapture({ modId: 100, fileId: 11 }, sender)
  const unknownFile = await harness.context.startCapture({ modId: 100, fileId: 0 }, sender)
  const repeatedUnknownFile = await harness.context.startCapture({ modId: 100, fileId: 0 }, sender)
  assert.equal(reinjected.requestId, first.requestId, 'same active capture must survive page reinjection')
  assert.notEqual(differentFile.requestId, first.requestId, 'different Nexus files must be separate actions')
  assert.notEqual(repeatedUnknownFile.requestId, unknownFile.requestId, 'unknown file identities must not be merged')
}

function panelBridgeHarness(fetchImpl) {
  const runtimeListeners = []
  const context = {
    AbortController,
    URL,
    Uint8Array,
    console,
    crypto: webcrypto,
    fetch: (...args) => fetchImpl(...args),
    setTimeout,
    clearTimeout,
    chrome: {
      runtime: {
        async sendMessage(message) {
          if (message.type === 'GET_CONFIG') return { ok: true, config: extensionConfig(), state: {} }
          return { ok: true, config: extensionConfig() }
        },
        onMessage: { addListener(listener) { runtimeListeners.push(listener) } },
      },
    },
    window: {
      location: { href: 'https://panel.example.test/instances/stardew/mods', origin: 'https://panel.example.test' },
      setTimeout,
      clearTimeout,
      addEventListener() {},
      postMessage() {},
    },
  }
  context.globalThis = context
  vm.createContext(context)
  vm.runInContext(sharedSource, context, { filename: 'shared.js' })
  vm.runInContext(panelBridgeSource, context, { filename: 'panel-bridge.js' })
  return { context, runtimeListeners }
}

function sendPanelInstall(listener, payload) {
  return new Promise((resolve) => {
    const keepAlive = listener({ type: 'PANEL_REMOTE_INSTALL', payload }, {}, resolve)
    assert.equal(keepAlive, true)
  })
}

async function testPanelBridgeSingleflightAndFileIdentity() {
  const requests = []
  let releaseFetch
  const fetchGate = new Promise((resolve) => { releaseFetch = resolve })
  const harness = panelBridgeHarness(async (url, options) => {
    requests.push({ url, options })
    await fetchGate
    return jsonResponse({ jobId: 'job-panel' })
  })
  await waitFor(() => harness.runtimeListeners.length === 1, 'panel bridge did not initialize')
  const listener = harness.runtimeListeners[0]
  const payload = {
    url: downloadURL,
    instanceId: 'stardew',
    requestId: 'panel-request-1',
    fileId: 10,
    mod: { modId: 100, name: 'Example' },
  }
  const first = sendPanelInstall(listener, payload)
  const duplicate = sendPanelInstall(listener, payload)
  await waitFor(() => requests.length === 1, 'panel bridge did not start its POST')
  assert.equal(requests.length, 1, 'panel bridge must share the same requestId POST')
  releaseFetch()
  const [firstResult, duplicateResult] = await Promise.all([first, duplicate])
  assert.equal(firstResult.result.jobId, 'job-panel')
  assert.equal(duplicateResult.result.jobId, 'job-panel')
  assert.equal(requests[0].options.headers['Idempotency-Key'], 'panel-request-1')

  const distinctRequests = []
  const distinctHarness = panelBridgeHarness(async (url, options) => {
    distinctRequests.push({ url, options })
    return jsonResponse({ jobId: `job-${distinctRequests.length}` })
  })
  await waitFor(() => distinctHarness.runtimeListeners.length === 1, 'second panel bridge did not initialize')
  const distinctListener = distinctHarness.runtimeListeners[0]
  await Promise.all([
    sendPanelInstall(distinctListener, { ...payload, requestId: '', fileId: 10 }),
    sendPanelInstall(distinctListener, { ...payload, requestId: '', fileId: 11 }),
  ])
  assert.equal(distinctRequests.length, 2, 'different fileIds must not be merged by the panel bridge')
}

assert.equal(manifest.version, '0.1.3')
await testBackgroundSingleflight()
await testFailureRetryAndWorkerRestart()
await testCaptureIdentityRotation()
await testPanelBridgeSingleflightAndFileIdentity()

console.log('Nexus extension idempotency tests passed')
