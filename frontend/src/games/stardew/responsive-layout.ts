export const SHELL_DESIGN_WIDTH = 1536
export const SHELL_DESIGN_HEIGHT = 1024
export const SHELL_MIN_UI_SCALE = 0.72
export const COMPACT_PHONE_MAX_WIDTH = 768
export const COMPACT_TOUCH_MAX_WIDTH = 1366
export const OPS_RAIL_COLLAPSE_MAIN_WIDTH = 400
export const OPS_RAIL_EXPAND_MAIN_WIDTH = 460

export const COMPACT_SHELL_MEDIA_QUERY =
  `(max-width: ${COMPACT_PHONE_MAX_WIDTH}px), ` +
  `(max-width: ${COMPACT_TOUCH_MAX_WIDTH}px) and (hover: none) and (pointer: coarse)`

export type ShellViewport = {
  scale: number
  layoutWidth: number
  layoutHeight: number
}

function positiveFiniteOr(value: number, fallback: number): number {
  return Number.isFinite(value) && value > 0 ? value : fallback
}

function clampNumber(min: number, value: number, max: number): number {
  return Math.max(min, Math.min(max, value))
}

export function shouldUseCompactShell(
  viewportWidth: number,
  hoverNone: boolean,
  pointerCoarse: boolean,
): boolean {
  const width = positiveFiniteOr(viewportWidth, SHELL_DESIGN_WIDTH)
  return (
    width <= COMPACT_PHONE_MAX_WIDTH ||
    (width <= COMPACT_TOUCH_MAX_WIDTH && hoverNone && pointerCoarse)
  )
}

export function shouldForceCompactShell(search: string): boolean {
  return new URLSearchParams(search).get('shell') === 'mobile'
}

export function calculateShellViewport(viewportWidth: number, viewportHeight: number): ShellViewport {
  const width = positiveFiniteOr(viewportWidth, SHELL_DESIGN_WIDTH)
  const height = positiveFiniteOr(viewportHeight, SHELL_DESIGN_HEIGHT)
  const scale = Math.max(
    SHELL_MIN_UI_SCALE,
    Math.min(width / SHELL_DESIGN_WIDTH, height / SHELL_DESIGN_HEIGHT),
  )

  return {
    scale,
    layoutWidth: width / scale,
    layoutHeight: height / scale,
  }
}

export function expandedMainWidthForShell(shellWidth: number, shellHeight: number): number {
  const width = positiveFiniteOr(shellWidth, SHELL_DESIGN_WIDTH)
  const scale = calculateShellViewport(width, shellHeight).scale
  const sidebarWidth = clampNumber(196, width * 0.14, 216) * scale
  const opsRailWidth = clampNumber(268, width * 0.19, 300) * scale
  return width - sidebarWidth - opsRailWidth
}

export function shouldAutoCollapseOpsRail(
  shellWidth: number,
  shellHeight: number,
  currentlyCollapsed: boolean,
): boolean {
  const width = positiveFiniteOr(shellWidth, SHELL_DESIGN_WIDTH)
  if (width <= 720) return false
  const expandedMainWidth = expandedMainWidthForShell(width, shellHeight)
  const threshold = currentlyCollapsed
    ? OPS_RAIL_EXPAND_MAIN_WIDTH
    : OPS_RAIL_COLLAPSE_MAIN_WIDTH
  return expandedMainWidth < threshold
}
