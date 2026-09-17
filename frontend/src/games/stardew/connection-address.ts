const DEFAULT_STARDEW_PORT = 24642

export function formatStardewAddress(hostname: string | undefined, port: number | undefined): string | null {
  const host = hostname?.trim().replace(/^\[([^\]]+)\]$/, '$1') ?? ''
  if (!host || port === undefined || !Number.isInteger(port) || port < 1 || port > 65535) return null
  if (port === DEFAULT_STARDEW_PORT) return host
  return `${host.includes(':') ? `[${host}]` : host}:${port}`
}
