import { useEffect, useState } from 'react'
import { ResourceHint } from '../ResourceHint'
import {
  measurePanelResponse, panelResponsePresentation, PANEL_RESPONSE_INTERVAL_MS,
  PANEL_RESPONSE_TIMEOUT_MS, type PanelResponseSample,
} from './panel-response'

export function PanelResponse() {
  const [sample, setSample] = useState<PanelResponseSample>({ status: 'loading' })

  useEffect(() => {
    let disposed = false
    let timer: number | undefined
    let active: AbortController | null = null

    function stop() {
      window.clearTimeout(timer)
      active?.abort()
      active = null
    }

    async function measure() {
      if (disposed || active || document.visibilityState !== 'visible') return
      const controller = new AbortController()
      active = controller
      let timedOut = false
      const timeout = window.setTimeout(() => {
        timedOut = true
        controller.abort()
      }, PANEL_RESPONSE_TIMEOUT_MS)
      try {
        const milliseconds = await measurePanelResponse(controller.signal)
        if (!disposed && active === controller) setSample({ status: 'ok', milliseconds })
      } catch {
        if (!disposed && active === controller) setSample({ status: timedOut ? 'timeout' : 'error' })
      } finally {
        window.clearTimeout(timeout)
        if (!disposed && active === controller) {
          active = null
          timer = window.setTimeout(() => { void measure() }, PANEL_RESPONSE_INTERVAL_MS)
        }
      }
    }

    function visibilityChanged() {
      stop()
      if (document.visibilityState === 'visible') {
        setSample({ status: 'loading' })
        void measure()
      } else {
        setSample({ status: 'paused' })
      }
    }

    document.addEventListener('visibilitychange', visibilityChanged)
    visibilityChanged()
    return () => {
      disposed = true
      stop()
      document.removeEventListener('visibilitychange', visibilityChanged)
    }
  }, [])

  const { text, level } = panelResponsePresentation(sample)
  const detail = [
    '当前浏览器 ↔ 面板的 HTTP 往返耗时，包含网络传输与面板处理时间。',
    '每 5 秒采样一次，页面隐藏时暂停。',
    sample.status === 'ok' ? `最近一次：${text}`
      : sample.status === 'timeout' ? '本次请求超过 3 秒，将自动重试。'
      : sample.status === 'error' ? '暂时未收到有效响应，将自动重试。'
      : sample.status === 'paused' ? '返回页面后重新采样。' : '正在等待首次响应。',
  ].join('\n')

  return <div className={`sd-opsrail-hstat sd-panel-response sd-opsrail-hstat--${level}`}>
    <ResourceHint className="sd-opsrail-hstat-row" label={`面板响应：${text}`} detail={detail}>
      <span className="sd-opsrail-hstat-orb" aria-hidden="true" />
      <span className="sd-opsrail-hstat-label">面板响应</span>
      <span className="sd-opsrail-hstat-value">{text}</span>
    </ResourceHint>
  </div>
}
