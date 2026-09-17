type LifecycleIconProps = { action: 'start' | 'stop' | 'restart' }

const paths = {
  start: 'M3 2h3v2h3v2h3v2h3v2h3v2h-3v2h-3v2H9v2H6v2H3Z',
  stop: 'M3 3h16v16H3Z',
  restart: 'M6 2h8v2h2v2h2V2h2v8h-8V8h3V6h-2V4H7v2H5v2H3v6h2v2h2v2h6v-2h3v-2h2v4h-3v2H6v-2H3v-2H1V6h2V4h3Z',
}

export function LifecycleIcon({ action }: LifecycleIconProps) {
  return (
    <svg className="sd-lifecycle-icon" viewBox="0 0 22 22" aria-hidden="true" focusable="false" shapeRendering="crispEdges">
      <path d={paths[action]} transform="translate(1 1)" fill="currentColor" />
    </svg>
  )
}
