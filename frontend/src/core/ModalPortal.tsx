import { useEffect, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

const focusableSelector = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

const modalStack: symbol[] = []
let bodyOverflowBeforeModal = ''

type BackgroundElementState = {
  ariaHidden: string | null
  element: HTMLElement
  inert: boolean
}

type ModalPortalProps = {
  ariaLabel?: string
  ariaLabelledBy?: string
  children: ReactNode
  className: string
  onEscape?: () => void
  role?: 'dialog' | 'alertdialog'
}

function visibleFocusableElements(container: HTMLElement): HTMLElement[] {
  return Array.from(container.querySelectorAll<HTMLElement>(focusableSelector)).filter((element) => (
    element.getClientRects().length > 0 && element.getAttribute('aria-hidden') !== 'true'
  ))
}

export function ModalPortal({
  ariaLabel,
  ariaLabelledBy,
  children,
  className,
  onEscape,
  role = 'dialog',
}: ModalPortalProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const onEscapeRef = useRef(onEscape)
  const returnFocusRef = useRef<HTMLElement | null>(
    typeof document !== 'undefined' && document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null,
  )
  const stackTokenRef = useRef(Symbol('modal'))

  onEscapeRef.current = onEscape

  useEffect(() => {
    const mountedContainer = containerRef.current
    if (!mountedContainer) return
    const container: HTMLDivElement = mountedContainer

    const stackToken = stackTokenRef.current
    if (!returnFocusRef.current && document.activeElement instanceof HTMLElement) {
      returnFocusRef.current = document.activeElement
    }
    container.focus()
    const backgroundElements: BackgroundElementState[] = Array.from(document.body.children)
      .filter((element): element is HTMLElement => element instanceof HTMLElement && element !== container)
      .map((element) => ({
        ariaHidden: element.getAttribute('aria-hidden'),
        element,
        inert: element.inert,
      }))
    backgroundElements.forEach(({ element }) => {
      element.inert = true
      element.setAttribute('aria-hidden', 'true')
    })
    if (modalStack.length === 0) {
      bodyOverflowBeforeModal = document.body.style.overflow
      document.body.style.overflow = 'hidden'
    }
    modalStack.push(stackToken)

    const focusFrame = window.requestAnimationFrame(() => {
      const requestedTarget = container.querySelector<HTMLElement>('[data-modal-initial-focus]')
      const firstTarget = visibleFocusableElements(container)[0]
      ;(requestedTarget ?? firstTarget ?? container).focus()
    })

    function handleKeyDown(event: KeyboardEvent) {
      if (modalStack.at(-1) !== stackToken) return

      if (event.key === 'Escape' && onEscapeRef.current) {
        event.preventDefault()
        event.stopPropagation()
        onEscapeRef.current()
        return
      }

      if (event.key !== 'Tab') return
      const focusable = visibleFocusableElements(container)
      if (focusable.length === 0) {
        event.preventDefault()
        container.focus()
        return
      }

      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      const active = document.activeElement
      if (!container.contains(active)) {
        event.preventDefault()
        first.focus()
      } else if (event.shiftKey && active === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && active === last) {
        event.preventDefault()
        first.focus()
      }
    }

    document.addEventListener('keydown', handleKeyDown, true)
    return () => {
      window.cancelAnimationFrame(focusFrame)
      document.removeEventListener('keydown', handleKeyDown, true)
      const stackIndex = modalStack.lastIndexOf(stackToken)
      if (stackIndex >= 0) modalStack.splice(stackIndex, 1)
      if (modalStack.length === 0) document.body.style.overflow = bodyOverflowBeforeModal
      backgroundElements.forEach(({ ariaHidden, element, inert }) => {
        element.inert = inert
        if (ariaHidden === null) element.removeAttribute('aria-hidden')
        else element.setAttribute('aria-hidden', ariaHidden)
      })
      const returnTarget = returnFocusRef.current
      if (returnTarget?.isConnected) window.requestAnimationFrame(() => returnTarget.focus())
    }
  }, [])

  return createPortal(
    <div
      ref={containerRef}
      className={className}
      role={role}
      aria-modal="true"
      aria-label={ariaLabel}
      aria-labelledby={ariaLabelledBy}
      tabIndex={-1}
    >
      {children}
    </div>,
    document.body,
  )
}
