import type { ModInfo } from '../../types'

function normalized(value?: string) {
  return value?.trim().toLowerCase() ?? ''
}

export function modIsSmapi(mod: ModInfo) {
  const uniqueId = normalized(mod.uniqueId)
  const folderName = normalized(mod.folderName)
  const name = normalized(mod.name)
  return mod.id === '__smapi_runtime' ||
    uniqueId === 'pathoschild.smapi' ||
    folderName === 'smapi' ||
    name === 'smapi'
}

export function modIsPanelControl(mod: ModInfo) {
  const uniqueId = normalized(mod.uniqueId)
  const folderName = normalized(mod.folderName)
  const name = normalized(mod.name)
  return folderName === 'stardewanxipanel.control' ||
    uniqueId === 'anxiyizhi.stardewanxipanel.control' ||
    name === 'stardew anxi panel control' ||
    name === 'stardew anxi panel'
}

export function modIsJunimoServer(mod: ModInfo) {
  const uniqueId = normalized(mod.uniqueId)
  const folderName = normalized(mod.folderName)
  const name = normalized(mod.name)
  return uniqueId === 'junimohost.server' ||
    folderName === 'junimoserver' ||
    folderName === 'junimohost.server' ||
    name === 'junimoserver' ||
    name === 'junimo server' ||
    name === 'junimohost.server'
}

export function modIsSystemRuntime(mod: ModInfo) {
  return modIsSmapi(mod) || modIsPanelControl(mod) || modIsJunimoServer(mod)
}
