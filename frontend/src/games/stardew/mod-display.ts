import type { ModInfo } from '../../types'

const CONTENT_PACK_PREFIXES: Record<string, string> = {
  'pathoschild.contentpatcher': '[CP]',
  'esca.farmtypemanager': '[FTM]',
}

// SMAPI manifests usually keep the technical content-pack marker in the
// folder name, while Name is the human-readable title. Preserve that useful
// distinction in the panel without changing the manifest-derived name.
export function modDisplayName(mod: ModInfo) {
  const name = mod.name || mod.folderName || mod.uniqueId || mod.id
  if (!mod.isContentPack) return name

  const contentPackFor = (mod.contentPackFor ?? '').trim().toLowerCase()
  const folderPrefix = mod.folderName.match(/^\[([a-z0-9-]{1,8})\]\s*/i)?.[0]?.trim()
  const prefix = CONTENT_PACK_PREFIXES[contentPackFor] ?? folderPrefix
  if (!prefix || name.toLowerCase().startsWith(prefix.toLowerCase())) return name
  return `${prefix} ${name}`
}
