const FALLBACK_INSTANCE_ID = 'stardew'
const INSTANCE_ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/

export let activeInstanceId = FALLBACK_INSTANCE_ID

export function normalizeInstanceId(value: unknown): string {
  const candidate = typeof value === 'string' ? value.trim() : ''
  return INSTANCE_ID_PATTERN.test(candidate) ? candidate : FALLBACK_INSTANCE_ID
}

export function setActiveInstanceId(value: unknown): string {
  activeInstanceId = normalizeInstanceId(value)
  return activeInstanceId
}
