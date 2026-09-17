import { Children, cloneElement, isValidElement, useEffect, useId, useLayoutEffect, useRef, useState, type HTMLAttributes, type ReactNode } from 'react'
import { controlReorderGesture, moveControlAction, readControlActionOrder, type ControlActionId } from './control-action-order'

type ActionProps = HTMLAttributes<HTMLDivElement> & { 'data-control-action': ControlActionId }
type Drag = {
  id: ControlActionId
  pointerId: number
  kind: 'touch' | 'pointer'
  row: HTMLElement
  y: number
  grabOffset: number
  original: ControlActionId[]
}

export function SortableControlActions({ storageKey, title, children }: { storageKey: string; title: ReactNode; children: ReactNode }) {
  const [order, setOrder] = useState(() => {
    try { return readControlActionOrder(localStorage.getItem(storageKey)) }
    catch { return readControlActionOrder(null) }
  })
  const [dragging, setDragging] = useState<ControlActionId | null>(null)
  const [announcement, setAnnouncement] = useState('')
  const listRef = useRef<HTMLDivElement>(null)
  const orderRef = useRef(order)
  const syncDragRef = useRef(() => {})
  const hintId = useId()
  const items = Children.toArray(children).filter(isValidElement<ActionProps>)

  useLayoutEffect(() => { syncDragRef.current() }, [order, dragging])

  useEffect(() => {
    const list = listRef.current
    if (!list) return
    let drag: Drag | null = null
    let frame = 0
    let lastFrame = 0
    let suppressClickUntil = 0
    let mounted = true
    const scroller = list.closest<HTMLElement>('.sd-mshell-scroll')
    const tabbar = list.closest('.sd-mshell')?.querySelector('.sd-mshell-tabbar')

    function applyOrder(next: ControlActionId[]) {
      if (next.every((id, i) => id === orderRef.current[i])) return
      orderRef.current = next
      setOrder(next)
    }

    function saveOrder(label: string) {
      try {
        localStorage.setItem(storageKey, JSON.stringify({ version: 1, order: orderRef.current }))
        setAnnouncement(`${label}，顺序已保存。`)
      } catch { setAnnouncement(`${label}，当前浏览器无法保存顺序。`) }
    }

    function syncPosition() {
      if (!drag || !gesture.isActive()) return
      const rect = drag.row.getBoundingClientRect()
      const scale = rect.height / drag.row.offsetHeight || 1
      const previous = parseFloat(drag.row.style.getPropertyValue('--sd-drag-offset')) || 0
      drag.row.style.setProperty('--sd-drag-offset', `${previous + (drag.y - drag.grabOffset - rect.top) / scale}px`)
    }
    syncDragRef.current = syncPosition

    function tick(time: number) {
      if (!drag || !gesture.isActive()) return
      const elapsed = lastFrame ? Math.min(time - lastFrame, 32) : 16
      lastFrame = time
      if (scroller) {
        const rect = scroller.getBoundingClientRect()
        const bottom = Math.min(rect.bottom, tabbar?.getBoundingClientRect().top ?? window.innerHeight)
        const speed = drag.y < rect.top + 48
          ? -Math.min(1, (rect.top + 48 - drag.y) / 48)
          : drag.y > bottom - 48 ? Math.min(1, (drag.y - bottom + 48) / 48) : 0
        scroller.scrollTop += speed * elapsed * 0.5
      }
      const others = Array.from(list!.querySelectorAll<HTMLElement>('[data-control-action]'))
        .filter((row) => row !== drag!.row)
      const index = others.filter((row) => {
        const rect = row.getBoundingClientRect()
        return drag!.y > rect.top + rect.height / 2
      }).length
      applyOrder(moveControlAction(orderRef.current, drag.id, index))
      syncPosition()
      frame = requestAnimationFrame(tick)
    }

    const gesture = controlReorderGesture((run, ms) => {
      const timer = window.setTimeout(run, ms)
      return () => window.clearTimeout(timer)
    }, () => {
      if (!drag) return
      setDragging(drag.id)
      setAnnouncement('已拿起卡片，上下拖动，松手保存。')
      drag.row.focus({ preventScroll: true })
      lastFrame = 0
      frame = requestAnimationFrame(tick)
    }, (cancelled, wasActive) => {
      cancelAnimationFrame(frame)
      const ended = drag
      drag = null
      ended?.row.style.removeProperty('--sd-drag-offset')
      if (ended?.kind === 'pointer' && ended.row.hasPointerCapture(ended.pointerId)) {
        ended.row.releasePointerCapture(ended.pointerId)
      }
      if (!mounted) return
      setDragging(null)
      if (!wasActive || !ended) return
      suppressClickUntil = Date.now() + 500
      if (cancelled) {
        applyOrder(ended.original)
        setAnnouncement('已取消移动。')
      } else if (!ended.original.every((id, i) => id === orderRef.current[i])) {
        saveOrder(`已移到第 ${orderRef.current.indexOf(ended.id) + 1} 位`)
      } else setAnnouncement('位置未改变。')
    })

    function start(target: EventTarget | null, pointerId: number, x: number, y: number, kind: Drag['kind']) {
      if (!(target instanceof Element) || target.closest('button, a, input, select, textarea')) return
      const row = target.closest<HTMLElement>('[data-control-action]')
      if (!row || !list!.contains(row)) return
      gesture.cancel()
      drag = {
        id: row.dataset.controlAction as ControlActionId, pointerId, kind, row, y,
        grabOffset: y - row.getBoundingClientRect().top, original: [...orderRef.current],
      }
      gesture.start(pointerId, x, y)
    }

    function pointerDown(event: PointerEvent) {
      if (event.pointerType === 'touch') return
      if (!event.isPrimary || event.button !== 0) { gesture.cancel(); return }
      start(event.target, event.pointerId, event.clientX, event.clientY, 'pointer')
      if (drag) drag.row.setPointerCapture(event.pointerId)
    }
    function pointerMove(event: PointerEvent) {
      if (!drag || drag.kind !== 'pointer' || drag.pointerId !== event.pointerId) return
      drag.y = event.clientY
      if (gesture.move(event.pointerId, event.clientX, event.clientY)) event.preventDefault()
    }
    function pointerUp(event: PointerEvent) {
      if (drag?.kind === 'pointer') gesture.release(event.pointerId)
    }
    function pointerCancel(event: PointerEvent) {
      if (drag?.kind === 'pointer' && drag.pointerId === event.pointerId) gesture.cancel()
    }
    function touchStart(event: TouchEvent) {
      if (event.touches.length !== 1) { gesture.cancel(); return }
      const touch = event.touches[0]
      start(event.target, touch.identifier, touch.clientX, touch.clientY, 'touch')
    }
    function touchMove(event: TouchEvent) {
      if (!drag || drag.kind !== 'touch') return
      if (event.touches.length !== 1) { gesture.cancel(); return }
      const touch = Array.from(event.touches).find((item) => item.identifier === drag!.pointerId)
      if (!touch) return
      drag.y = touch.clientY
      if (gesture.move(touch.identifier, touch.clientX, touch.clientY)) {
        if (event.cancelable) event.preventDefault()
        else gesture.cancel()
      }
    }
    function touchEnd(event: TouchEvent) {
      if (!drag || drag.kind !== 'touch') return
      const touch = Array.from(event.changedTouches).find((item) => item.identifier === drag!.pointerId)
      if (!touch) return
      if (gesture.isActive() && event.cancelable) event.preventDefault()
      gesture.release(touch.identifier)
    }
    function cancel() { gesture.cancel() }
    function scroll() { if (!gesture.isActive()) gesture.cancel() }
    function visibility() { if (document.hidden) gesture.cancel() }
    function click(event: MouseEvent) {
      if (event.detail !== 0 && (gesture.isActive() || Date.now() < suppressClickUntil)) {
        event.preventDefault()
        event.stopImmediatePropagation()
      }
    }
    function contextMenu(event: Event) {
      if (drag) event.preventDefault()
    }
    function nativeDrag(event: Event) { event.preventDefault() }
    function keyDown(event: KeyboardEvent) {
      if (gesture.isActive()) {
        if (event.key === 'Escape') gesture.cancel()
        event.preventDefault()
        return
      }
      const row = event.target instanceof HTMLElement ? event.target : null
      const id = row?.dataset.controlAction as ControlActionId | undefined
      if (!id || !event.altKey || !['ArrowUp', 'ArrowDown'].includes(event.key)) return
      event.preventDefault()
      gesture.cancel()
      const next = moveControlAction(orderRef.current, id, orderRef.current.indexOf(id) + (event.key === 'ArrowUp' ? -1 : 1))
      if (next.every((item, i) => item === orderRef.current[i])) return
      applyOrder(next)
      saveOrder(`已移到第 ${next.indexOf(id) + 1} 位`)
      requestAnimationFrame(() => {
        if (!mounted || !row?.isConnected) return
        row.focus({ preventScroll: true })
        row.scrollIntoView({ block: 'nearest' })
      })
    }

    list.addEventListener('pointerdown', pointerDown)
    window.addEventListener('pointermove', pointerMove)
    window.addEventListener('pointerup', pointerUp)
    window.addEventListener('pointercancel', pointerCancel)
    window.addEventListener('touchstart', touchStart, { passive: true })
    // React delegates touchmove passively. A native listener may stop scrolling only after the hold succeeds.
    window.addEventListener('touchmove', touchMove, { passive: false })
    window.addEventListener('touchend', touchEnd, { passive: false })
    window.addEventListener('touchcancel', cancel)
    window.addEventListener('blur', cancel)
    window.addEventListener('scroll', scroll, true)
    document.addEventListener('visibilitychange', visibility)
    list.addEventListener('click', click, true)
    list.addEventListener('contextmenu', contextMenu)
    list.addEventListener('dragstart', nativeDrag)
    list.addEventListener('keydown', keyDown)
    return () => {
      mounted = false
      gesture.cancel()
      syncDragRef.current = () => {}
      list.removeEventListener('pointerdown', pointerDown)
      window.removeEventListener('pointermove', pointerMove)
      window.removeEventListener('pointerup', pointerUp)
      window.removeEventListener('pointercancel', pointerCancel)
      window.removeEventListener('touchstart', touchStart)
      window.removeEventListener('touchmove', touchMove)
      window.removeEventListener('touchend', touchEnd)
      window.removeEventListener('touchcancel', cancel)
      window.removeEventListener('blur', cancel)
      window.removeEventListener('scroll', scroll, true)
      document.removeEventListener('visibilitychange', visibility)
      list.removeEventListener('click', click, true)
      list.removeEventListener('contextmenu', contextMenu)
      list.removeEventListener('dragstart', nativeDrag)
      list.removeEventListener('keydown', keyDown)
    }
  }, [storageKey])

  return (
    <>
      <div className="sd-mctrl-sort-heading">
        <div className="sd-mctrl-card-title">{title}</div>
        <p className="sd-mctrl-sort-hint" id={hintId}>
          长按卡片拖动排序
          <span className="sd-mctrl-sort-status">；键盘聚焦卡片后，按 Alt 加上下方向键移动。</span>
        </p>
      </div>
      <div ref={listRef} className={`sd-mctrl-action-list${dragging ? ' is-sorting' : ''}`} role="list" aria-label="快捷操作排序">
        {order.map((id, index) => {
          const item = items.find((child) => child.props['data-control-action'] === id)
          return item ? cloneElement(item, {
            key: id,
            className: `${item.props.className}${dragging === id ? ' is-dragging' : ''}`,
            role: 'listitem', tabIndex: 0, 'aria-describedby': hintId, 'aria-posinset': index + 1, 'aria-setsize': order.length,
          }) : null
        })}
      </div>
      <span className="sd-mctrl-sort-status" role="status" aria-live="polite">{announcement}</span>
    </>
  )
}
