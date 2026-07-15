import type { FarmTypeCatalogItem, FarmTypeCatalogResponse } from '../../types'

export const modFarmPlaceholder = '/assets/stardew/ui/icons/icon_nav_mods_crystal_image2.png'

export type FarmCatalogViewState = {
  loading: boolean
  modded: FarmTypeCatalogItem[]
  warnings: string[]
  error: string
  moddedCreationEnabled: boolean
}

export const initialFarmCatalogViewState: FarmCatalogViewState = {
  loading: true,
  modded: [],
  warnings: [],
  error: '',
  moddedCreationEnabled: false,
}

type FarmCatalogLoader = (instanceId: string, signal: AbortSignal) => Promise<FarmTypeCatalogResponse>

export function catalogResponseState(response: FarmTypeCatalogResponse): FarmCatalogViewState {
  return {
    loading: false,
    modded: response.farmTypes.filter((farm) => farm.kind === 'modded'),
    warnings: response.catalogWarnings,
    error: '',
    // Older backends don't return this field. Treat absence as disabled so a
    // mixed-version deployment can never open modded creation accidentally.
    moddedCreationEnabled: response.moddedCreationEnabled === true,
  }
}

export function startFarmCatalogLoad(
  instanceId: string,
  onResult: (state: FarmCatalogViewState) => void,
  loader: FarmCatalogLoader,
): () => void {
  const controller = new AbortController()
  void loader(instanceId, controller.signal)
    .then((response) => {
      if (!controller.signal.aborted) onResult(catalogResponseState(response))
    })
    .catch((error: unknown) => {
      if (controller.signal.aborted) return
      const message = error instanceof Error ? error.message : '农场目录读取失败'
      onResult({ loading: false, modded: [], warnings: [], error: message, moddedCreationEnabled: false })
    })
  return () => controller.abort()
}

export function farmCatalogIconSource(farm: FarmTypeCatalogItem, failedURLs: ReadonlySet<string>): string {
  if (!farm.iconUrl || failedURLs.has(farm.iconUrl)) return modFarmPlaceholder
  return farm.iconUrl
}

export function farmDependencyStatusText(farm: FarmTypeCatalogItem): string {
  if (farm.conflict || farm.modSelection?.readiness === 'conflict') return 'FarmType ID 存在冲突'
  const selection = farm.modSelection
  if (!selection) return '依赖状态暂不可用'
  if (selection.missingRequiredModKeys.length > 0) return `缺少：${selection.missingRequiredModKeys.join('、')}`
  if (selection.disabledRequiredModKeys.length > 0) return `有 ${selection.disabledRequiredModKeys.length} 个依赖尚未启用`
  return '依赖完整'
}

export function farmComponentsToEnable(farm: FarmTypeCatalogItem) {
  return farm.modSelection?.components.filter((component) => !component.enabled) ?? []
}

export function canSelectModFarm(farm: FarmTypeCatalogItem, featureEnabled: boolean): boolean {
  return featureEnabled && farm.kind === 'modded' && farm.enabled && farm.dependenciesReady === true
    && !farm.conflict && farm.confidence === 'explicit' && farm.selectable
}

export function isSafeManualFarmTypeId(value: string): boolean {
  const trimmed = value.trim()
  return trimmed.length > 0 && new TextEncoder().encode(trimmed).length <= 128 && !/[\u0000-\u001f\u007f]/.test(trimmed)
}

export function moddedCompatibilityShortcut(farms: FarmTypeCatalogItem[], featureEnabled: boolean): FarmTypeCatalogItem | null {
  const selectable = farms.filter((farm) => canSelectModFarm(farm, featureEnabled))
  return selectable.length === 1 ? selectable[0] : null
}
