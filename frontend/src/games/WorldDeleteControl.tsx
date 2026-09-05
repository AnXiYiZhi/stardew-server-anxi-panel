import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { deleteInstance } from '../api'
import { errorMessage } from '../core/helpers'
import { worldDeleteGesture } from './world-delete-gesture'

export function useWorldDeletePress(open: () => void, enabled: boolean) {
  const [pressing, setPressing] = useState(false)
  const openRef = useRef(open)
  openRef.current = open
  const [gesture] = useState(() => worldDeleteGesture((run, ms) => {
    const timer = window.setTimeout(run, ms)
    return () => window.clearTimeout(timer)
  }, () => openRef.current(), setPressing))
  useEffect(() => {
    const cancel = () => gesture.cancel()
    window.addEventListener('blur', cancel)
    window.addEventListener('scroll', cancel, true)
    document.addEventListener('visibilitychange', cancel)
    return () => {
      window.removeEventListener('blur', cancel)
      window.removeEventListener('scroll', cancel, true)
      document.removeEventListener('visibilitychange', cancel)
      gesture.cancel()
    }
  }, [gesture])
  useEffect(() => { if (!enabled) gesture.cancel() }, [enabled, gesture])
  return { pressing, gesture }
}

export function WorldDeleteDialog({ id, name, onClose, onDeleted }: {
  id: string; name: string; onClose: () => void; onDeleted: () => void
}) {
  const dialog = useRef<HTMLDialogElement>(null)
  const submitting = useRef(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  useEffect(() => {
    const element = dialog.current!
    element.showModal()
    // Disabling the focused submit button can move focus to body. Capture
    // Escape there too, before the game rail's window shortcut sees it.
    const escape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.preventDefault()
      event.stopPropagation()
      if (!submitting.current) onClose()
    }
    document.addEventListener('keydown', escape, true)
    return () => { document.removeEventListener('keydown', escape, true); element.close() }
  }, [])
  useEffect(() => {
    if (!busy && error) dialog.current?.querySelector<HTMLButtonElement>('.world-delete-confirm')?.focus()
  }, [busy, error])
  async function submit() {
    if (submitting.current) return
    submitting.current = true
    setBusy(true)
    setError(null)
    try { await deleteInstance(id); onDeleted() }
    catch (error) { setError(errorMessage(error)) }
    finally { submitting.current = false; setBusy(false) }
  }
  return createPortal(
    <dialog ref={dialog} className="world-delete-dialog" aria-labelledby="world-delete-title" aria-describedby="world-delete-warning" aria-busy={busy}
      onCancel={(event) => { event.preventDefault(); event.stopPropagation(); if (!submitting.current) onClose() }}
      onKeyDown={(event) => event.stopPropagation()}>
      <h2 id="world-delete-title">彻底删除世界</h2>
      <p className="world-delete-name">{name}</p>
      <p id="world-delete-warning">将永久删除此世界及全部备份，无法恢复。</p>
      {error ? <p role="alert" className="world-delete-error">{error}</p> : null}
      <div className="world-delete-actions">
        <button type="button" autoFocus disabled={busy} onClick={onClose}>取消</button>
        <button type="button" className="world-delete-confirm" disabled={busy} onClick={() => void submit()}>{busy ? '删除中…' : '确认删除'}</button>
      </div>
    </dialog>, document.body,
  )
}
