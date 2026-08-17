import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { webcrypto } from 'node:crypto'
import vm from 'node:vm'

const extensionRoot = new URL('../../browser-extensions/nexus-slow-installer/', import.meta.url)
const sharedSource = readFileSync(new URL('shared.js', extensionRoot), 'utf8')
const backgroundSource = readFileSync(new URL('background.js', extensionRoot), 'utf8')
const contentSource = readFileSync(new URL('content.js', extensionRoot), 'utf8')
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
    expectedVersion: '2.9.1',
    modName: 'Example',
    pageUrl: `https://www.nexusmods.com/stardewvalley/mods/${modId}?file_id=${fileId}`,
    pendingUrl: downloadURL,
  }
  return { captures: { [key]: capture }, capture }
}

function backgroundHarness({ seedState = captureState(), fetchImpl, tabMessageImpl, setTimeoutImpl = setTimeout } = {}) {
  const storage = storageArea({
    [CONFIG_KEY]: extensionConfig(),
    [STATE_KEY]: seedState,
  })
  const runtimeListeners = []
  const createdTabs = []
  let nextTabId = 99
  const context = {
    AbortController,
    URL,
    Uint8Array,
    console,
    crypto: webcrypto,
    fetch: (...args) => fetchImpl(...args),
    setTimeout: setTimeoutImpl,
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
        async create(options) {
          const tab = { id: nextTabId++, url: options.url, active: options.active }
          createdTabs.push(tab)
          return tab
        },
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
  return { context, runtimeListeners, storage: storage.values, createdTabs }
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
  assert.equal(requests[0].options.headers['X-Anxi-Nexus-Installer'], '0.1.8')
  assert.deepEqual(JSON.parse(requests[0].options.body), {
    url: downloadURL,
    mod: {
      modId: 100,
      name: 'Example',
      version: '2.9.1',
      nexusUrl: 'https://www.nexusmods.com/stardewvalley/mods/100?file_id=10',
    },
    expectedVersion: '2.9.1',
    nexusFileId: 10,
  })
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

async function testUpdateContextReachesPanelRequest() {
  const seedState = captureState({ requestId: 'update-request-1' })
  seedState.capture.operation = 'update'
  seedState.capture.replaceUniqueId = 'Pathoschild.ContentPatcher'
  seedState.captures['tab:1'] = seedState.capture
  const requests = []
  const harness = backgroundHarness({
    seedState,
    fetchImpl: async (url, options) => {
      requests.push({ url, options })
      return jsonResponse({ jobId: 'job-update' })
    },
  })

  await harness.context.finishInstall(downloadURL, 'tab:1')
  assert.equal(requests.length, 1)
  assert.equal(JSON.parse(requests[0].options.body).replaceUniqueId, 'Pathoschild.ContentPatcher')
}

async function testCaptureIdentityRotation() {
  const harness = backgroundHarness({
    seedState: { captures: {}, capture: null },
    fetchImpl: async () => jsonResponse({ jobId: 'unused' }),
  })
  const sender = { tab: { id: 7, url: 'https://www.nexusmods.com/stardewvalley/mods/100' } }
  const first = await harness.context.startCapture({ modId: 100, fileId: 10, expectedVersion: '2.9.0' }, sender)
  const reinjected = await harness.context.startCapture({ modId: 100, fileId: 10, expectedVersion: '2.9.0' }, sender)
  const differentVersion = await harness.context.startCapture({ modId: 100, fileId: 10, expectedVersion: '2.9.1' }, sender)
  const differentFile = await harness.context.startCapture({ modId: 100, fileId: 11, expectedVersion: '2.9.1' }, sender)
  const unknownFile = await harness.context.startCapture({ modId: 100, fileId: 0 }, sender)
  const repeatedUnknownFile = await harness.context.startCapture({ modId: 100, fileId: 0 }, sender)
  assert.equal(reinjected.requestId, first.requestId, 'same active capture must survive page reinjection')
  assert.notEqual(differentVersion.requestId, first.requestId, 'a new target version must rotate the install identity')
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
    expectedVersion: '2.9.1',
    mod: { modId: 100, name: 'Example', version: '2.9.1' },
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
  assert.equal(requests[0].options.headers['X-Anxi-Nexus-Installer'], '0.1.8')
  assert.deepEqual(JSON.parse(requests[0].options.body), {
    url: downloadURL,
    mod: { modId: 100, name: 'Example', version: '2.9.1' },
    expectedVersion: '2.9.1',
    nexusFileId: 10,
  })

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

function testVersionAwareFileSelection() {
  const context = { URL, console }
  context.globalThis = context
  vm.createContext(context)
  vm.runInContext(sharedSource, context, { filename: 'shared.js' })
  const oldFirst = [
    { fileId: 100, contextText: 'Content Patcher 2.9.0 Manual download', visible: true, order: 0 },
    { fileId: 101, contextText: 'Content Patcher 2.9.1 Manual download', visible: true, order: 1 },
  ]
  assert.equal(context.chooseNexusFileCandidate(oldFirst, '2.9.1').fileId, 101)
  assert.equal(context.chooseNexusFileCandidate(oldFirst, 'v2.9.1').fileId, 101)
  assert.equal(context.chooseNexusFileCandidate(oldFirst, '2.9.10'), null)
  assert.equal(context.nexusVersionMatchesText('Content Patcher 2.9.10', '2.9.1'), false)

  const restored = context.mergeNexusAutomationParams(
    { batchId: 'batch-1', itemId: 'target-1915', autoSubmit: true, expectedVersion: '' },
    { batchId: 'batch-1', itemId: 'target-1915', autoSubmit: true, expectedVersion: '2.9.1' },
  )
  assert.equal(restored.expectedVersion, '2.9.1', 'Nexus redirects must not drop the selected latest version')
  const unrelated = context.mergeNexusAutomationParams(
    { batchId: 'batch-2', itemId: 'target-1915', autoSubmit: true, expectedVersion: '' },
    { batchId: 'batch-1', itemId: 'target-1915', autoSubmit: true, expectedVersion: '2.9.1' },
  )
  assert.equal(unrelated.expectedVersion, '', 'a different batch must not inherit stale version state')
}

async function testBatchCarriesLatestVersionAndAllowsLegacyInference() {
  const harness = backgroundHarness({
    fetchImpl: async () => jsonResponse({ jobId: 'unused' }),
  })
  const pageURL = harness.context.withBatchParams(
    'https://www.nexusmods.com/stardewvalley/mods/1915?tab=files',
    'batch-1',
    { id: 'target-1915', expectedVersion: '2.9.1' },
  )
  assert.equal(new URL(pageURL).searchParams.get('anxi_version'), '2.9.1')
  const legacyHarness = backgroundHarness({
    fetchImpl: async () => jsonResponse({ jobId: 'unused' }),
    setTimeoutImpl: (callback, delay, ...args) => delay >= 180000 ? 0 : setTimeout(callback, delay, ...args),
  })
  const legacyBatch = await legacyHarness.context.startBatchInstall({
    batchId: 'batch-missing-version',
    targets: [{ id: 'target-1915', modId: 1915, name: 'Content Patcher', url: pageURL }],
  }, { tab: { id: 1 } })
  assert.equal(legacyBatch.items[0].expectedVersion, '')
  assert.equal(legacyBatch.items[0].status, 'opening', 'legacy Panel targets should defer latest-version discovery to Nexus')

  const updateBatch = await legacyHarness.context.startBatchInstall({
    batchId: 'batch-update',
    targets: [{
      id: 'update-1915',
      operation: 'update',
      replaceUniqueId: 'Pathoschild.ContentPatcher',
      modId: 1915,
      name: 'Content Patcher',
      url: pageURL,
      expectedVersion: '2.9.1',
    }],
  }, { tab: { id: 1 } })
  assert.equal(updateBatch.items[0].operation, 'update')
  assert.equal(updateBatch.items[0].replaceUniqueId, 'Pathoschild.ContentPatcher')
  const updateCapture = await legacyHarness.context.startCapture({
    batchId: 'batch-update',
    itemId: 'update-1915',
    modId: 1915,
    fileId: 101,
    expectedVersion: '2.9.1',
  }, { tab: { id: 99 } })
  assert.equal(updateCapture.operation, 'update')
  assert.equal(updateCapture.replaceUniqueId, 'Pathoschild.ContentPatcher')
}

async function testBatchTargetsOpenSequentially() {
  const harness = backgroundHarness({
    seedState: { captures: {}, capture: null },
    fetchImpl: async () => jsonResponse({ jobId: 'unused' }),
    tabMessageImpl: async (_tabId, message) => {
      if (message.type === 'PANEL_REMOTE_INSTALL') {
        return { ok: true, result: { jobId: 'job-required' } }
      }
      return { ok: true }
    },
    setTimeoutImpl: (callback, delay, ...args) => delay >= 180000 ? 0 : setTimeout(callback, delay, ...args),
  })
  const batchId = 'batch-sequential'
  const batch = await harness.context.startBatchInstall({
    batchId,
    targets: [
      {
        id: 'required-1915',
        role: 'required',
        modId: 1915,
        name: 'Content Patcher',
        url: 'https://www.nexusmods.com/stardewvalley/mods/1915?tab=files',
        expectedVersion: '2.9.1',
      },
      {
        id: 'target-4626',
        role: 'target',
        modId: 4626,
        name: 'Dialogue translation',
        url: 'https://www.nexusmods.com/stardewvalley/mods/4626?tab=files',
        expectedVersion: '1.2.11',
      },
    ],
  }, { tab: { id: 1 } })

  assert.equal(harness.createdTabs.length, 1, 'only the first batch target may open initially')
  assert.equal(batch.items[0].status, 'opening')
  assert.equal(batch.items[1].status, 'pending')
  assert.match(harness.createdTabs[0].url, /\/mods\/1915/)

  const firstTab = harness.createdTabs[0]
  await harness.context.startCapture({
    batchId,
    itemId: 'required-1915',
    modId: 1915,
    fileId: 160463,
    expectedVersion: '2.9.1',
    autoSubmit: true,
    closeTabOnComplete: true,
    pageUrl: firstTab.url,
  }, { tab: { id: firstTab.id, url: firstTab.url } })
  await harness.context.finishInstall(downloadURL, `${batchId}:required-1915`)

  assert.equal(harness.createdTabs.length, 2, 'the next target must open only after the first POST is accepted')
  const targetURL = new URL(harness.createdTabs[1].url)
  assert.equal(targetURL.pathname, '/stardewvalley/mods/4626')
  assert.equal(targetURL.searchParams.get('anxi_item'), 'target-4626')
  assert.equal(targetURL.searchParams.get('anxi_version'), '1.2.11')
  assert.equal(targetURL.searchParams.get('file_id'), null, 'the dependency file id must not leak into the target page')
  const storedBatch = harness.storage[STATE_KEY].batches[batchId]
  assert.equal(storedBatch.items[0].status, 'queued')
  assert.equal(storedBatch.items[1].status, 'opening')
}

const legacySlowGuard = contentSource.indexOf('if (clickSlow && (document.querySelector("mod-file-download") || findSlowDownloadButton()))')
const versionAwareDiscovery = contentSource.indexOf('if (!pageInfo.fileId || batch.expectedVersion)', legacySlowGuard)
assert.ok(legacySlowGuard >= 0, 'legacy Nexus slow-download pages must have an explicit auto-click guard')
assert.ok(versionAwareDiscovery > legacySlowGuard, 'the slow-download interstitial guard must run before file discovery')
assert.equal(manifest.version, '0.1.8')
assert.match(contentSource, /current\.previousElementSibling/)
assert.match(contentSource, /label\.tagName\.toLowerCase\(\) === "dt"/)
testVersionAwareFileSelection()
await testBatchCarriesLatestVersionAndAllowsLegacyInference()
await testBatchTargetsOpenSequentially()
await testBackgroundSingleflight()
await testUpdateContextReachesPanelRequest()
await testFailureRetryAndWorkerRestart()
await testCaptureIdentityRotation()
await testPanelBridgeSingleflightAndFileIdentity()

console.log('Nexus extension idempotency tests passed')
