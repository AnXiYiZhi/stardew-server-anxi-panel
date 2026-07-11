import { useEffect, useState } from 'react'
import { getInstanceVNCConfig, getInstanceRenderingFPS, setInstanceRenderingFPS } from '../../api'
import { errorMessage } from '../../core/helpers'

const vncDisplayFPS = 15

function buildVNCControlURL(port: string) {
  const host = window.location.hostname.includes(':')
    ? `[${window.location.hostname}]`
    : window.location.hostname
  return `http://${host}:${port}/`
}

type VNCSettingsOptions = {
  isAdmin: boolean
  isRunning: boolean
}

export function useServerVNCSettings({ isAdmin, isRunning }: VNCSettingsOptions) {
  const [vncPort, setVNCPort] = useState('')
  const [vncPortLoading, setVNCPortLoading] = useState(false)
  const [vncDisplayBusy, setVNCDisplayBusy] = useState(false)
  const [vncRenderingEnabled, setVNCRenderingEnabled] = useState(false)
  const [vncRenderingStatusLoading, setVNCRenderingStatusLoading] = useState(false)
  const [vncMessage, setVNCMessage] = useState<string | null>(null)
  const [vncError, setVNCError] = useState<string | null>(null)

  useEffect(() => {
    if (!isRunning) {
      setVNCRenderingEnabled(false)
      setVNCRenderingStatusLoading(false)
    }
  }, [isRunning])

  useEffect(() => {
    if (!isAdmin || !isRunning) return
    let canceled = false
    setVNCRenderingStatusLoading(true)
    getInstanceRenderingFPS()
      .then((res) => {
        if (canceled) return
        setVNCRenderingEnabled(res.fps > 0)
      })
      .catch((e) => {
        if (canceled) return
        setVNCError(`读取 VNC 显示状态失败：${errorMessage(e)}`)
      })
      .finally(() => {
        if (!canceled) setVNCRenderingStatusLoading(false)
      })
    return () => {
      canceled = true
    }
  }, [isAdmin, isRunning])

  useEffect(() => {
    if (!isAdmin) {
      setVNCPort('')
      return
    }
    let canceled = false
    setVNCPortLoading(true)
    getInstanceVNCConfig()
      .then((res) => {
        if (canceled) return
        setVNCPort(res.vncPort)
      })
      .catch((e) => {
        if (canceled) return
        setVNCError(`读取 VNC 端口失败：${errorMessage(e)}`)
      })
      .finally(() => {
        if (!canceled) setVNCPortLoading(false)
      })
    return () => {
      canceled = true
    }
  }, [isAdmin])

  async function handleToggleVNCDisplay() {
    if (!isAdmin || !isRunning) return
    const nextEnabled = !vncRenderingEnabled
    const nextFPS = nextEnabled ? vncDisplayFPS : 0
    setVNCDisplayBusy(true)
    setVNCMessage(null)
    setVNCError(null)
    try {
      const result = await setInstanceRenderingFPS(nextFPS)
      setVNCRenderingEnabled(nextEnabled)
      setVNCMessage(
        nextEnabled
          ? `VNC 显示已打开（${result.fps} FPS），现在可以跳转到 VNC 控制。`
          : 'VNC 显示已关闭。'
      )
    } catch (e) {
      setVNCError(errorMessage(e))
    } finally {
      setVNCDisplayBusy(false)
    }
  }

  function handleOpenVNCControl() {
    if (!isAdmin || !isRunning || !vncPort) return
    setVNCError(null)
    const opened = window.open(buildVNCControlURL(vncPort), '_blank')
    if (!opened) {
      setVNCError('浏览器拦截了 VNC 控制窗口，请允许弹出窗口后重试。')
      return
    }
    opened.opener = null
    setVNCMessage(`已打开 VNC 控制页面（端口 ${vncPort}）。`)
  }

  return {
    vncPort,
    vncPortLoading,
    vncDisplayBusy,
    vncRenderingEnabled,
    vncRenderingStatusLoading,
    vncMessage,
    vncError,
    vncDisplayFPS,
    buildVNCControlURL,
    handleToggleVNCDisplay,
    handleOpenVNCControl,
  }
}
