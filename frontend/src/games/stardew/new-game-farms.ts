import type { NewGameConfig } from '../../types'

export type BuiltinFarm = { id: NewGameConfig['farmType']; label: string; asset: string }

// These files are generated once from the local Stardew runtime and committed
// with the panel image. Modded catalog entries never enter this selectable list.
export const builtinFarms: BuiltinFarm[] = [
  { id: 'standard', label: '标准农场', asset: '/assets/stardew/new-game/farms/standard.png' },
  { id: 'riverland', label: '河边农场', asset: '/assets/stardew/new-game/farms/riverland.png' },
  { id: 'forest', label: '森林农场', asset: '/assets/stardew/new-game/farms/forest.png' },
  { id: 'hilltop', label: '山顶农场', asset: '/assets/stardew/new-game/farms/hilltop.png' },
  { id: 'wilderness', label: '荒野农场', asset: '/assets/stardew/new-game/farms/wilderness.png' },
  { id: 'fourcorners', label: '四角农场', asset: '/assets/stardew/new-game/farms/fourcorners.png' },
  { id: 'beach', label: '海滩农场', asset: '/assets/stardew/new-game/farms/beach.png' },
  { id: 'meadowlands', label: '草原农场', asset: '/assets/stardew/new-game/farms/meadowlands.png' },
]

const builtinFarmIDs = new Set(builtinFarms.map((farm) => farm.id))

export function isBuiltinFarmType(value: string): boolean {
  return builtinFarmIDs.has(value)
}
