import { useState } from 'react'
import type { NewGameConfig } from '../../types'
import './NewGameCreator.css'

type Props = {
  instanceId: string
  onSubmit: (cfg: NewGameConfig) => void
  submitting?: boolean
  submitError?: string
}

type Farm = { id: NewGameConfig['farmType']; label: string; asset: string }
type Gender = { id: 'male' | 'female'; label: string; icon: string; preview: string }
type PetPreference = { petType: 'Cat' | 'Dog'; breed: number; label: string; asset: string }

// These files are generated once from the local Stardew runtime and committed with
// the panel image. They deliberately do not depend on a user's game download.
const farms: Farm[] = [
  { id: 'standard', label: '标准农场', asset: '/assets/stardew/new-game/farms/standard.png' },
  { id: 'riverland', label: '河边农场', asset: '/assets/stardew/new-game/farms/riverland.png' },
  { id: 'forest', label: '森林农场', asset: '/assets/stardew/new-game/farms/forest.png' },
  { id: 'hilltop', label: '山顶农场', asset: '/assets/stardew/new-game/farms/hilltop.png' },
  { id: 'wilderness', label: '荒野农场', asset: '/assets/stardew/new-game/farms/wilderness.png' },
  { id: 'fourcorners', label: '四角农场', asset: '/assets/stardew/new-game/farms/fourcorners.png' },
  { id: 'beach', label: '海滩农场', asset: '/assets/stardew/new-game/farms/beach.png' },
  { id: 'meadowlands', label: '草原农场', asset: '/assets/stardew/new-game/farms/meadowlands.png' },
]

const genders: Gender[] = [
  { id: 'male', label: '男', icon: '/assets/stardew/new-game/gender/male-icon.png', preview: '/assets/stardew/new-game/characters/male-preview.png' },
  { id: 'female', label: '女', icon: '/assets/stardew/new-game/gender/female-icon.png', preview: '/assets/stardew/new-game/characters/female-preview.png' },
]

// This order is the game data order exported from the local runtime: every
// selectable cat breed first, followed by every selectable dog breed.
const petPreferences: PetPreference[] = [
  ...[0, 1, 2, 3, 4].map((breed) => ({ petType: 'Cat' as const, breed, label: `猫 ${breed + 1}`, asset: `/assets/stardew/new-game/pets/cat-${breed}.png` })),
  ...[0, 1, 2, 3, 4].map((breed) => ({ petType: 'Dog' as const, breed, label: `狗 ${breed + 1}`, asset: `/assets/stardew/new-game/pets/dog-${breed}.png` })),
]

function defaultConfig(): NewGameConfig {
  return {
    farmName: '',
    farmerName: '',
    favoriteThing: '',
    farmType: 'standard',
    gender: 'male',
    petType: 'Cat',
    petBreedId: '0',
    petBreed: 0,
    startingCabins: 0,
    cabinLayout: 'nearby',
    profitMargin: '100',
    moneyMode: 'shared',
    remixedCommunityCenter: false,
    remixedMineRewards: false,
    spawnMonstersOnFarm: false,
    skipIntro: true,
  }
}

function ArrowButton({ direction, onClick, label }: { direction: 'left' | 'right'; onClick: () => void; label: string }) {
  return (
    <button type="button" className={`ngc-arrow ngc-arrow-${direction}`} onClick={onClick} aria-label={label}>
      {direction === 'left' ? '◀' : '▶'}
    </button>
  )
}

function cycle<T>(value: T, values: T[], direction: -1 | 1): T {
  const current = Math.max(0, values.indexOf(value))
  return values[(current + direction + values.length) % values.length]
}

export function NewGameCreator({ onSubmit, submitting, submitError }: Props) {
  const [cfg, setCfg] = useState<NewGameConfig>(defaultConfig)
  const [advancedOpen, setAdvancedOpen] = useState(false)

  function set<K extends keyof NewGameConfig>(key: K, value: NewGameConfig[K]) {
    setCfg((previous) => ({ ...previous, [key]: value }))
  }

  function updateCabins(direction: -1 | 1) {
    set('startingCabins', Math.max(0, Math.min(7, cfg.startingCabins + direction)))
  }

  function updateFarm(direction: -1 | 1) {
    set('farmType', cycle(cfg.farmType, farms.map((farm) => farm.id), direction))
  }

  function updatePet(direction: -1 | 1) {
    const current = petPreferences.findIndex((pet) => pet.petType === cfg.petType && pet.breed === cfg.petBreed)
    const next = petPreferences[(Math.max(0, current) + direction + petPreferences.length) % petPreferences.length]
    setCfg((previous) => ({ ...previous, petType: next.petType, petBreed: next.breed, petBreedId: String(next.breed) }))
  }

  function updateGender(direction: -1 | 1) {
    set('gender', cycle((cfg.gender ?? 'male') as Gender['id'], genders.map((gender) => gender.id), direction))
  }

  function submit(event: React.FormEvent) {
    event.preventDefault()
    // Intro skipping is intentionally non-optional in the panel UI.
    onSubmit({ ...cfg, skipIntro: true })
  }

  const selectedFarm = farms.find((farm) => farm.id === cfg.farmType) ?? farms[0]
  const selectedGender = genders.find((gender) => gender.id === cfg.gender) ?? genders[0]
  const selectedPet = petPreferences.find((pet) => pet.petType === cfg.petType && pet.breed === cfg.petBreed) ?? petPreferences[0]
  const playerCount = cfg.startingCabins + 1

  return (
    <form className="ngc-game" onSubmit={submit}>
      <aside className="ngc-side-panel ngc-host-panel" aria-label="联机设置">
        <div className="ngc-side-title">初始联机小屋</div>
        <div className="ngc-number-control">
          <ArrowButton direction="left" label="减少联机小屋" onClick={() => updateCabins(-1)} />
          <strong>{playerCount}</strong>
          <ArrowButton direction="right" label="增加联机小屋" onClick={() => updateCabins(1)} />
        </div>

        <div className="ngc-side-label">联机小屋布局</div>
        <div className="ngc-layout-options">
          <button type="button" className={cfg.cabinLayout === 'nearby' ? 'selected' : ''} onClick={() => set('cabinLayout', 'nearby')}>
            <img src="/assets/stardew/new-game/cabins/nearby.png" alt="靠近布局" /><small>靠近</small>
          </button>
          <button type="button" className={cfg.cabinLayout === 'separate' ? 'selected' : ''} onClick={() => set('cabinLayout', 'separate')}>
            <img src="/assets/stardew/new-game/cabins/separate.png" alt="分散布局" /><small>分散</small>
          </button>
        </div>

        <div className="ngc-side-label">利润率</div>
        <div className="ngc-number-control">
          <ArrowButton direction="left" label="降低利润率" onClick={() => set('profitMargin', cycle(cfg.profitMargin, ['100', '75', '50', '25'], -1))} />
          <strong>{cfg.profitMargin === '100' ? '普通' : `${cfg.profitMargin}%`}</strong>
          <ArrowButton direction="right" label="提高利润率" onClick={() => set('profitMargin', cycle(cfg.profitMargin, ['100', '75', '50', '25'], 1))} />
        </div>

        <div className="ngc-side-label">资金管理</div>
        <div className="ngc-number-control">
          <ArrowButton direction="left" label="切换资金管理" onClick={() => set('moneyMode', cfg.moneyMode === 'shared' ? 'separate' : 'shared')} />
          <strong>{cfg.moneyMode === 'shared' ? '共享' : '分开'}</strong>
          <ArrowButton direction="right" label="切换资金管理" onClick={() => set('moneyMode', cfg.moneyMode === 'shared' ? 'separate' : 'shared')} />
        </div>
      </aside>

      <main className="ngc-main-panel">
        <section className="ngc-character-preview" aria-label="角色设置">
          <div className="ngc-farmer-silhouette"><img src={selectedGender.preview} alt={`${selectedGender.label}角色预览`} /></div>
          <div className="ngc-character-switch">
            <ArrowButton direction="left" label="切换性别" onClick={() => updateGender(-1)} />
            <strong><img className="ngc-gender-icon" src={selectedGender.icon} alt="" />{selectedGender.label}</strong>
            <ArrowButton direction="right" label="切换性别" onClick={() => updateGender(1)} />
          </div>
        </section>

        <section className="ngc-fields">
          <label>名字<input required maxLength={100} value={cfg.farmerName ?? ''} onChange={(event) => set('farmerName', event.target.value)} /></label>
          <label>农场名字<input required maxLength={100} value={cfg.farmName} onChange={(event) => set('farmName', event.target.value)} /><span>农场</span></label>
          <label>最喜欢<br />的东西<input maxLength={100} value={cfg.favoriteThing ?? ''} onChange={(event) => set('favoriteThing', event.target.value)} /></label>
          <label className="ngc-pet-line">动物偏好
            <ArrowButton direction="left" label="上一种宠物" onClick={() => updatePet(-1)} />
            <strong className="ngc-pet-choice"><img src={selectedPet.asset} alt="" />{selectedPet.label}</strong>
            <ArrowButton direction="right" label="下一种宠物" onClick={() => updatePet(1)} />
          </label>
        </section>

        <section className="ngc-farm-choice" aria-label="农场类型">
          <div className="ngc-farm-label">农场类型</div>
          <div className="ngc-farm-picker">
            <ArrowButton direction="left" label="上一种农场" onClick={() => updateFarm(-1)} />
            <button type="button" className="ngc-selected-farm" onClick={() => updateFarm(1)} title="切换农场类型">
              <img src={selectedFarm.asset} alt="" onError={(event) => { event.currentTarget.style.display = 'none' }} />
              <span>{selectedFarm.label}</span>
            </button>
            <ArrowButton direction="right" label="下一种农场" onClick={() => updateFarm(1)} />
          </div>
        </section>

        <label className="ngc-check-row ngc-locked-check">
          <input type="checkbox" checked readOnly />
          <span>跳过开场动画</span>
        </label>

        <section className="ngc-advanced">
          <button type="button" className="ngc-advanced-toggle" onClick={() => setAdvancedOpen((open) => !open)} aria-expanded={advancedOpen}>
            高级设置 <span>{advancedOpen ? '▲' : '▼'}</span>
          </button>
          {advancedOpen && (
            <div className="ngc-advanced-options">
              <label className="ngc-check-row"><input type="checkbox" checked={cfg.remixedCommunityCenter} onChange={(event) => set('remixedCommunityCenter', event.target.checked)} />社区中心手机包</label>
              <label className="ngc-check-row"><input type="checkbox" checked={cfg.remixedMineRewards} onChange={(event) => set('remixedMineRewards', event.target.checked)} />矿洞掉落</label>
              <label className="ngc-check-row"><input type="checkbox" checked={cfg.spawnMonstersOnFarm} onChange={(event) => set('spawnMonstersOnFarm', event.target.checked)} />在农场出现怪物</label>
            </div>
          )}
        </section>

        {submitError && <p className="ngc-submit-error">{submitError}</p>}
        <button className="ngc-submit" type="submit" disabled={submitting || !cfg.farmName.trim() || !cfg.farmerName?.trim()}>
          {submitting ? '正在创建…' : '确认并创建存档'}
        </button>
      </main>

      <aside className="ngc-map-panel" aria-label="选择农场">
        {farms.map((farm) => (
          <button key={farm.id} type="button" className={cfg.farmType === farm.id ? 'selected' : ''} onClick={() => set('farmType', farm.id)}>
            <img src={farm.asset} alt="" onError={(event) => { event.currentTarget.style.display = 'none' }} />
            <span>{farm.label}</span>
          </button>
        ))}
      </aside>
    </form>
  )
}
