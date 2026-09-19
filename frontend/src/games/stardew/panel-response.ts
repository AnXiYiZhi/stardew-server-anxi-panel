export const PANEL_RESPONSE_INTERVAL_MS = 5000
export const PANEL_RESPONSE_TIMEOUT_MS = 3000

export type PanelResponseSample =
  | { status: 'ok'; milliseconds: number }
  | { status: 'loading' | 'timeout' | 'error' | 'paused' }

export function panelResponsePresentation(sample: PanelResponseSample) {
  if (sample.status !== 'ok') {
    const text = { loading: '测量中', timeout: '超时', error: '连接失败', paused: '已暂停' }[sample.status]
    return { text, level: 'unknown' as const }
  }
  const ms = Math.round(sample.milliseconds)
  return {
    text: sample.milliseconds < 1 ? '<1 ms' : `${ms} ms`,
    level: ms >= 1000 ? 'crit' as const : ms >= 400 ? 'warn' as const : ms >= 200 ? 'info' as const : 'ok' as const,
  }
}

// Measure the complete, uncached HTTP response from this browser to the Panel.
// The existing version endpoint reads only in-memory build metadata.
export async function measurePanelResponse(
  signal: AbortSignal,
  fetcher: typeof fetch = fetch,
  clock: () => number = () => performance.now(),
): Promise<number> {
  const url = `/api/version?panel-response=${Date.now()}`
  const start = clock()
  const response = await fetcher(url, {
    signal, cache: 'no-store', credentials: 'same-origin', redirect: 'error',
    headers: { Accept: 'application/json' },
  })
  if (response.status !== 200 || response.redirected) throw new Error('Panel response unavailable')
  const body = await response.text()
  const elapsed = clock() - start
  const version: unknown = JSON.parse(body)
  if (!version || typeof version !== 'object' || !('version' in version)
    || typeof version.version !== 'string' || !version.version.trim()
    || !Number.isFinite(elapsed) || elapsed < 0) {
    throw new Error('Invalid Panel response')
  }
  signal.throwIfAborted()
  return elapsed
}
