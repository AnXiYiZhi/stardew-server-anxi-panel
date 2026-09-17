import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import './ResourceHint.css'

export function ResourceHint({ children, detail, label, className = '', style, focusable = true }: {
  children: ReactNode; detail: string; label: string; className?: string; style?: CSSProperties; focusable?: boolean
}) {
  const [open, setOpen] = useState(false)
  const [position, setPosition] = useState({ left: 8, top: 8 })
  const anchor = useRef<HTMLSpanElement>(null)
  const tooltip = useRef<HTMLSpanElement>(null)
  const id = useId()
  const rotated = Boolean(open && anchor.current?.closest('.game-hub') && window.matchMedia('(max-width: 700px) and (orientation: portrait)').matches)
  const place = useCallback(() => {
    if (!anchor.current || !tooltip.current) return
    const a = anchor.current.getBoundingClientRect()
    const t = tooltip.current.getBoundingClientRect()
    if (a.bottom < 0 || a.top > window.innerHeight || a.right < 0 || a.left > window.innerWidth) { setOpen(false); return }
    const x = rotated ? a.right + t.width + 16 <= window.innerWidth ? a.right + 8 : a.left - t.width - 8 : a.left + a.width / 2 - t.width / 2
    const y = rotated ? a.top + a.height / 2 - t.height / 2 : a.top >= t.height + 16 ? a.top - t.height - 8 : a.bottom + 8
    setPosition({ left: Math.max(8, Math.min(window.innerWidth - t.width - 8, x)) + (rotated ? t.width : 0), top: Math.max(8, Math.min(window.innerHeight - t.height - 8, y)) })
  }, [rotated])
  useLayoutEffect(() => { if (open) place() }, [open, detail, place])
  useEffect(() => {
    if (!open) return
    const close = () => setOpen(false)
    const escape = (event: KeyboardEvent) => { if (event.key === 'Escape') close() }
    const outside = (event: PointerEvent) => { if (!anchor.current?.contains(event.target as Node)) close() }
    window.addEventListener('resize', close)
    window.addEventListener('scroll', place, true)
    document.addEventListener('keydown', escape)
    document.addEventListener('pointerdown', outside)
    return () => { window.removeEventListener('resize', close); window.removeEventListener('scroll', place, true); document.removeEventListener('keydown', escape); document.removeEventListener('pointerdown', outside) }
  }, [open, place])
  return <span ref={anchor} className={`resource-hint ${className}`} style={style} tabIndex={focusable ? 0 : undefined} aria-label={label} aria-describedby={open ? id : undefined}
    onMouseEnter={() => setOpen(true)} onMouseLeave={() => setOpen(false)} onFocus={() => setOpen(true)} onBlur={() => setOpen(false)}
    onClick={event => { event.stopPropagation(); setOpen(true) }}>
    {children}
    {open && createPortal(<span ref={tooltip} id={id} role="tooltip" className={`resource-hint-popup${rotated ? ' is-rotated' : ''}`} style={position}>{detail}</span>, document.body)}
  </span>
}

export function ResourcePair({ primary, secondary, unit, accessible }: { primary: string; secondary: string; unit: string; accessible: string }) {
  return <span className={`resource-pair${primary.length + secondary.length > 6 ? ' is-long' : ''}`} aria-label={accessible}>
    <span>{primary}</span><span className="resource-pair-secondary"><span className="resource-pair-slash">/</span>{secondary}</span>{unit && <small>{unit}</small>}
  </span>
}
