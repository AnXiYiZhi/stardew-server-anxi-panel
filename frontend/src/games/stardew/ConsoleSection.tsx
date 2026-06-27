import { useCallback, useEffect, useState } from 'react'
import type { ConsoleCommandDef, CommandRunResult } from '../../types'
import { getCommands, runCommand, sendSay } from '../../api'
import { errorMessage } from '../../core/helpers'

type HistoryEntry = {
  id: number
  command: string
  name: string
  time: Date
  result: CommandRunResult | null
  error: string | null
  expanded: boolean
}

type Props = {
  state: string
  isAdmin: boolean
  onStateRefresh?: () => void
}

export function ConsoleSection({ state, isAdmin: _isAdmin }: Props) {
  const [commands, setCommands] = useState<ConsoleCommandDef[]>([])
  const [history, setHistory] = useState<HistoryEntry[]>([])
  const [sayText, setSayText] = useState('')
  const [loading, setLoading] = useState(false)
  const [busy, setBusy] = useState<string | null>(null) // command ID being executed
  const [message, setMessage] = useState('')
  const [sayBusy, setSayBusy] = useState(false)
  const [sayMessage, setSayMessage] = useState('')
  const [historyCounter, setHistoryCounter] = useState(0)

  const isRunning = state === 'running'

  const loadCommands = useCallback(async () => {
    setLoading(true)
    setMessage('')
    try {
      const data = await getCommands()
      setCommands(data.commands)
    } catch (err) {
      setMessage(errorMessage(err))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadCommands()
  }, [loadCommands])

  const handleRunCommand = useCallback(async (cmd: ConsoleCommandDef) => {
    if (!isRunning) return
    setBusy(cmd.id)
    setMessage('')
    try {
      const result = await runCommand(cmd.id)
      const entry: HistoryEntry = {
        id: historyCounter,
        command: cmd.id,
        name: cmd.name,
        time: new Date(),
        result,
        error: null,
        expanded: true,
      }
      setHistory((prev) => [entry, ...prev].slice(0, 20))
      setHistoryCounter((c) => c + 1)
    } catch (err) {
      const isTimeout = err instanceof DOMException && err.name === 'AbortError'
      const entry: HistoryEntry = {
        id: historyCounter,
        command: cmd.id,
        name: cmd.name,
        time: new Date(),
        result: null,
        error: isTimeout ? '命令执行超时，请稍后重试' : errorMessage(err),
        expanded: true,
      }
      setHistory((prev) => [entry, ...prev].slice(0, 20))
      setHistoryCounter((c) => c + 1)
    } finally {
      setBusy(null)
    }
  }, [isRunning, historyCounter])

  const handleSay = useCallback(async () => {
    if (!isRunning || !sayText.trim()) return
    setSayBusy(true)
    setSayMessage('')
    try {
      const result = await sendSay(sayText.trim())
      const entry: HistoryEntry = {
        id: historyCounter,
        command: 'say',
        name: '服务器喊话',
        time: new Date(),
        result,
        error: null,
        expanded: true,
      }
      setHistory((prev) => [entry, ...prev].slice(0, 20))
      setHistoryCounter((c) => c + 1)
      setSayText('')
      setSayMessage('喊话已发送')
    } catch (err) {
      const isTimeout = err instanceof DOMException && err.name === 'AbortError'
      setSayMessage(isTimeout ? '喊话执行超时，请稍后重试' : errorMessage(err))
    } finally {
      setSayBusy(false)
    }
  }, [isRunning, sayText, historyCounter])

  const toggleExpand = useCallback((id: number) => {
    setHistory((prev) =>
      prev.map((e) => (e.id === id ? { ...e, expanded: !e.expanded } : e))
    )
  }, [])

  function formatTime(d: Date): string {
    return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  }

  return (
    <section className="console-section">
      <div className="section-heading">
        <div>
          <h2>控制台 / 命令</h2>
          <p className="section-desc">执行常用 Junimo 服务器命令</p>
        </div>
      </div>

      {message ? <div className="error-banner">{message}</div> : null}

      {/* Command buttons */}
      {!isRunning ? (
        <div className="console-offline-hint">
          服务器未运行，命令不可用。请先启动服务器。
        </div>
      ) : loading ? (
        <div className="console-loading">加载命令列表...</div>
      ) : (
        <div className="command-btn-grid">
          {commands.map((cmd) => (
            <button
              key={cmd.id}
              className="command-btn"
              disabled={busy !== null}
              onClick={() => void handleRunCommand(cmd)}
              title={cmd.description}
            >
              {busy === cmd.id ? '执行中...' : cmd.name}
            </button>
          ))}
        </div>
      )}

      {/* Say input */}
      <div className="console-say-area">
        <h3>服务器喊话</h3>
        <div className="console-say-row">
          <input
            type="text"
            className="console-say-input"
            placeholder="输入喊话内容（最多 200 字符）"
            value={sayText}
            maxLength={200}
            disabled={!isRunning || sayBusy}
            onChange={(e) => setSayText(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && sayText.trim() && isRunning && !sayBusy) {
                void handleSay()
              }
            }}
          />
          <button
            className="btn btn-primary console-say-btn"
            disabled={!isRunning || sayBusy || !sayText.trim()}
            onClick={() => void handleSay()}
          >
            {sayBusy ? '发送中...' : '发送'}
          </button>
        </div>
        {sayMessage ? <div className="console-say-msg">{sayMessage}</div> : null}
      </div>

      {/* Command history */}
      {history.length > 0 && (
        <div className="console-history">
          <h3>命令历史</h3>
          {history.map((entry) => (
            <div key={entry.id} className="console-history-entry">
              <div
                className="console-history-header"
                onClick={() => toggleExpand(entry.id)}
              >
                <span className="console-history-name">{entry.name}</span>
                <span className="console-history-time">{formatTime(entry.time)}</span>
                {entry.error ? (
                  <span className="console-history-status console-status-error">失败</span>
                ) : (
                  <span className="console-history-status console-status-ok">
                    退出码 {entry.result?.exitCode ?? '?'}
                  </span>
                )}
                <span className="console-history-toggle">
                  {entry.expanded ? '▼' : '▶'}
                </span>
              </div>
              {entry.expanded && (
                <div className="console-history-output">
                  {entry.error ? (
                    <div className="console-output-error">{entry.error}</div>
                  ) : (
                    <>
                      {entry.result?.output && (
                        <pre className="console-output-text">{entry.result.output}</pre>
                      )}
                      {entry.result?.error && (
                        <pre className="console-output-stderr">{entry.result.error}</pre>
                      )}
                      {!entry.result?.output && !entry.result?.error && (
                        <div className="console-output-empty">（无输出）</div>
                      )}
                      <div className="console-output-meta">
                        耗时 {entry.result?.durationMs ?? 0}ms
                      </div>
                    </>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </section>
  )
}
