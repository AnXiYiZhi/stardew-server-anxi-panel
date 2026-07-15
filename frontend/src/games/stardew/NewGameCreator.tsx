import { useEffect, useRef, useState } from 'react'
import { getFarmTypeCatalog, prepareFarmTypeMods } from '../../api'
import type { FarmTypeCatalogItem, NewGameConfig } from '../../types'
import { canSelectModFarm, farmCatalogIconSource, farmComponentsToEnable, farmDependencyStatusText, initialFarmCatalogViewState, isSafeManualFarmTypeId, modFarmPlaceholder, moddedCompatibilityShortcut, startFarmCatalogLoad } from './farm-catalog-state'
import { builtinFarms, isBuiltinFarmType } from './new-game-farms'
import './NewGameCreator.css'

type Props = {
  instanceId: string
  onSubmit: (cfg: NewGameConfig) => void
  submitting?: boolean
  submitError?: string
}

type Gender = { id: 'male' | 'female'; icon: string; preview: string }
type PetPreference = { petType: 'Cat' | 'Dog'; breed: number; asset: string }

const genders: Gender[] = [
  { id: 'male', icon: '/assets/stardew/new-game/gender/male-icon.png', preview: '/assets/stardew/new-game/characters/male-preview.png' },
  { id: 'female', icon: '/assets/stardew/new-game/gender/female-icon.png', preview: '/assets/stardew/new-game/characters/female-preview.png' },
]

// This order is the game data order exported from the local runtime: every
// selectable cat breed first, followed by every selectable dog breed.
const petPreferences: PetPreference[] = [
  ...[0, 1, 2, 3, 4].map((breed) => ({ petType: 'Cat' as const, breed, asset: `/assets/stardew/new-game/pets/cat-${breed}.png` })),
  ...[0, 1, 2, 3, 4].map((breed) => ({ petType: 'Dog' as const, breed, asset: `/assets/stardew/new-game/pets/dog-${breed}.png` })),
]

function defaultConfig(): NewGameConfig {
  return {
    farmName: '',
    farmerName: 'host',
    favoriteThing: '星露谷',
    farmType: 'standard',
    gender: 'male',
    petType: 'Cat',
    petBreedId: '0',
    petBreed: 0,
    startingCabins: 0,
    maxPlayers: 10,
    cabinLayout: 'nearby',
    cabinMode: 'recommended',
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
      <span aria-hidden="true" />
    </button>
  )
}

function cycle<T>(value: T, values: T[], direction: -1 | 1): T {
  const current = Math.max(0, values.indexOf(value))
  return values[(current + direction + values.length) % values.length]
}

export function NewGameCreator({ instanceId, onSubmit, submitting, submitError }: Props) {
  const [cfg, setCfg] = useState<NewGameConfig>(defaultConfig)
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [farmCatalog, setFarmCatalog] = useState(initialFarmCatalogViewState)
  const [failedFarmIcons, setFailedFarmIcons] = useState<Set<string>>(() => new Set())
  const [catalogReload, setCatalogReload] = useState(0)
  const [prepareTarget, setPrepareTarget] = useState<FarmTypeCatalogItem | null>(null)
  const [prepareBusy, setPrepareBusy] = useState(false)
  const [prepareError, setPrepareError] = useState('')
  const [prepareMessage, setPrepareMessage] = useState('')
  const [manualFarmTypeId, setManualFarmTypeId] = useState('')
  const prepareController = useRef<AbortController | null>(null)

  useEffect(() => {
    setFarmCatalog(initialFarmCatalogViewState)
    setFailedFarmIcons(new Set())
    return startFarmCatalogLoad(instanceId, setFarmCatalog, getFarmTypeCatalog)
  }, [instanceId, catalogReload])

  useEffect(() => () => prepareController.current?.abort(), [])

  function openPrepare(farm: FarmTypeCatalogItem) {
    if (farm.modSelection?.readiness !== 'needs_enable') return
    setPrepareError('')
    setPrepareTarget(farm)
  }

  function closePrepare() {
    if (prepareBusy) return
    setPrepareTarget(null)
    setPrepareError('')
  }

  async function confirmPrepare() {
    if (!prepareTarget) return
    prepareController.current?.abort()
    const controller = new AbortController()
    prepareController.current = controller
    setPrepareBusy(true)
    setPrepareError('')
    try {
      const result = await prepareFarmTypeMods(instanceId, prepareTarget.id, controller.signal)
      if (controller.signal.aborted) return
      const changed = result.changedModKeys?.length ?? 0
      setPrepareMessage(changed > 0 ? `已启用 ${changed} 个创建所需组件，服务器不会自动启动。` : '所需组件已经启用。')
      setPrepareTarget(null)
      setCatalogReload((value) => value + 1)
    } catch (error) {
      if (controller.signal.aborted) return
      setPrepareError(error instanceof Error ? error.message : '准备 Mod 失败')
    } finally {
      if (!controller.signal.aborted) setPrepareBusy(false)
    }
  }

  function set<K extends keyof NewGameConfig>(key: K, value: NewGameConfig[K]) {
    setCfg((previous) => ({ ...previous, [key]: value }))
  }

  function updateCabins(direction: -1 | 1) {
    setCfg((previous) => {
      const nextCabins = Math.max(0, Math.min(7, previous.startingCabins + direction))
      return {
        ...previous,
        startingCabins: nextCabins,
        maxPlayers: Math.max(previous.maxPlayers ?? 10, nextCabins + 1),
      }
    })
  }

  function updateMaxPlayers(direction: -1 | 1) {
    setCfg((previous) => ({
      ...previous,
      maxPlayers: Math.max(previous.startingCabins + 1, Math.min(100, (previous.maxPlayers ?? 10) + direction)),
    }))
  }

  function updateFarm(direction: -1 | 1) {
    set('farmType', cycle(cfg.farmType, builtinFarms.map((farm) => farm.id), direction))
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
    if (!isBuiltinFarmType(cfg.farmType) && (!farmCatalog.moddedCreationEnabled || !isSafeManualFarmTypeId(cfg.farmType))) return
    // Intro skipping is intentionally non-optional in the panel UI.
    onSubmit({ ...cfg, farmType: cfg.farmType, skipIntro: true })
  }

  const selectedBuiltinFarm = builtinFarms.find((farm) => farm.id === cfg.farmType)
  const selectedModFarm = farmCatalog.modded.find((farm) => farm.id === cfg.farmType)
  const selectedFarmLabel = selectedBuiltinFarm?.label ?? selectedModFarm?.label ?? cfg.farmType
  const selectedFarmAsset = selectedBuiltinFarm?.asset ?? (selectedModFarm ? farmCatalogIconSource(selectedModFarm, failedFarmIcons) : modFarmPlaceholder)
  const selectedFarmAllowed = isBuiltinFarmType(cfg.farmType)
    || (selectedModFarm ? canSelectModFarm(selectedModFarm, farmCatalog.moddedCreationEnabled) : farmCatalog.moddedCreationEnabled && isSafeManualFarmTypeId(cfg.farmType))
  const compatibilityShortcut = moddedCompatibilityShortcut(farmCatalog.modded, farmCatalog.moddedCreationEnabled)
  const selectedGender = genders.find((gender) => gender.id === cfg.gender) ?? genders[0]
  const selectedPet = petPreferences.find((pet) => pet.petType === cfg.petType && pet.breed === cfg.petBreed) ?? petPreferences[0]
  const cabinCount = cfg.startingCabins

  return (
    <>
      <form className="ngc-game" onSubmit={submit}>
      <aside className="ngc-side-panel ngc-host-panel" aria-label="联机设置">
        <div className="ngc-side-title">初始联机小屋</div>
        <div className="ngc-number-control">
          <ArrowButton direction="left" label="减少联机小屋" onClick={() => updateCabins(-1)} />
          <strong>{cabinCount}</strong>
          <ArrowButton direction="right" label="增加联机小屋" onClick={() => updateCabins(1)} />
        </div>

        <div className="ngc-side-label">联机人数上限</div>
        <div className="ngc-number-control">
          <ArrowButton direction="left" label="降低联机人数上限" onClick={() => updateMaxPlayers(-1)} />
          <strong>{cfg.maxPlayers ?? 10}人</strong>
          <ArrowButton direction="right" label="提高联机人数上限" onClick={() => updateMaxPlayers(1)} />
        </div>

        <div className="ngc-side-label">小屋模式</div>
        <div className="ngc-number-control">
          <ArrowButton
            direction="left"
            label="切换小屋模式"
            onClick={() => set('cabinMode', cfg.cabinMode === 'vanilla' ? 'recommended' : 'vanilla')}
          />
          <strong>{cfg.cabinMode === 'vanilla' ? '原版' : '推荐'}</strong>
          <ArrowButton
            direction="right"
            label="切换小屋模式"
            onClick={() => set('cabinMode', cfg.cabinMode === 'vanilla' ? 'recommended' : 'vanilla')}
          />
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
          <div className="ngc-farmer-silhouette"><img src={selectedGender.preview} alt="角色预览" /></div>
          <div className="ngc-character-switch">
            <ArrowButton direction="left" label="切换性别" onClick={() => updateGender(-1)} />
            <strong><img className="ngc-gender-icon" src={selectedGender.icon} alt="" /></strong>
            <ArrowButton direction="right" label="切换性别" onClick={() => updateGender(1)} />
          </div>
        </section>

        <section className="ngc-fields">
          <label>名字<input required maxLength={100} value={cfg.farmerName ?? ''} onChange={(event) => set('farmerName', event.target.value)} /></label>
          <label>农场名字<input required maxLength={100} value={cfg.farmName} onChange={(event) => set('farmName', event.target.value)} /><span>农场</span></label>
          <label>最喜欢<br />的东西<input maxLength={100} value={cfg.favoriteThing ?? ''} onChange={(event) => set('favoriteThing', event.target.value)} /></label>
          <label className="ngc-pet-line">动物偏好
            <ArrowButton direction="left" label="上一种宠物" onClick={() => updatePet(-1)} />
            <strong className="ngc-pet-choice"><img src={selectedPet.asset} alt="宠物预览" /></strong>
            <ArrowButton direction="right" label="下一种宠物" onClick={() => updatePet(1)} />
          </label>
        </section>

        <section className="ngc-farm-choice" aria-label="农场类型">
          <div className="ngc-farm-label">农场类型</div>
          <div className="ngc-farm-picker">
            <ArrowButton direction="left" label="上一种农场" onClick={() => updateFarm(-1)} />
            <button type="button" className="ngc-selected-farm" onClick={() => updateFarm(1)} title="切换农场类型">
              <img src={selectedFarmAsset} alt="" onError={(event) => { event.currentTarget.src = modFarmPlaceholder }} />
              <span>{selectedFarmLabel}</span>
            </button>
            <ArrowButton direction="right" label="下一种农场" onClick={() => updateFarm(1)} />
          </div>
        </section>

        <section className="ngc-modded-catalog" aria-label="检测到的模组农场">
          <div className="ngc-modded-heading">
            <strong>检测到的模组农场</strong>
            <span>{farmCatalog.moddedCreationEnabled ? '创建时会启动并再次验证' : '创建功能未启用'}</span>
          </div>
          {farmCatalog.loading ? <p className="ngc-catalog-note">正在读取已安装 Mod…</p> : null}
          {farmCatalog.error ? <p className="ngc-catalog-warning">模组农场目录暂时无法读取，官方农场仍可正常创建。</p> : null}
          {prepareMessage ? <p className="ngc-catalog-note">{prepareMessage}</p> : null}
          {!farmCatalog.loading && !farmCatalog.error && farmCatalog.modded.length === 0 ? (
            <p className="ngc-catalog-note">当前没有检测到模组农场。</p>
          ) : null}
          {farmCatalog.modded.length > 0 ? (
            <div className="ngc-modded-grid">
              {farmCatalog.modded.map((farm, index) => {
                const cardKey = `${farm.id}:${farm.providerModId ?? ''}:${index}`
                const iconSource = farmCatalogIconSource(farm, failedFarmIcons)
                return (
                  <article className={`ngc-modded-card${farm.enabled ? '' : ' is-disabled'}${farm.conflict ? ' has-conflict' : ''}${cfg.farmType === farm.id ? ' is-selected' : ''}`} key={cardKey} data-selectable={canSelectModFarm(farm, farmCatalog.moddedCreationEnabled)}>
                    <img
                      src={iconSource}
                      alt=""
                      onError={() => {
                        if (!farm.iconUrl || iconSource !== farm.iconUrl) return
                        setFailedFarmIcons((previous) => new Set(previous).add(farm.iconUrl!))
                      }}
                    />
                    <div className="ngc-modded-card-body">
                      <div className="ngc-modded-title"><strong>{farm.label}</strong><span>MOD</span></div>
                      <code>FarmType: {farm.id}</code>
                      <small>{farm.providerName || farm.providerModId || '未知 Mod'}{farm.providerVersion ? ` · v${farm.providerVersion}` : ''}</small>
                      <div className="ngc-modded-statuses">
                        <span>{farm.enabled ? '已启用' : '未启用'}</span>
                        {farm.conflict ? <span className="is-conflict">ID 冲突</span> : null}
                        <span className={farm.dependenciesReady ? 'is-ready' : 'is-pending'}>{farmDependencyStatusText(farm)}</span>
                      </div>
                      {farm.modSelection?.missingRequiredModKeys.length ? (
                        <p className="ngc-modded-missing">缺少必需 Mod：{farm.modSelection.missingRequiredModKeys.join('、')}</p>
                      ) : null}
                      {farm.description ? <p>{farm.description}</p> : null}
                      {farm.modSelection?.readiness === 'needs_enable' ? (
                        <button className="ngc-prepare-button" type="button" onClick={() => openPrepare(farm)}>一键准备</button>
                      ) : null}
                      {canSelectModFarm(farm, farmCatalog.moddedCreationEnabled) ? (
                        <button className="ngc-select-modded-button" type="button" onClick={() => set('farmType', farm.id)}>选择该农场</button>
                      ) : null}
                      <p className="ngc-modded-lock">{farmCatalog.moddedCreationEnabled ? '创建时会启动服务器，并用本次运行时目录再次验证该 FarmType。' : '已检测到该模组农场；当前功能开关未启用。'}</p>
                    </div>
                  </article>
                )
              })}
            </div>
          ) : null}
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
              {farmCatalog.moddedCreationEnabled ? (
                <div className="ngc-manual-farm">
                  <label>手动 FarmType Id
                    <input
                      maxLength={128}
                      value={manualFarmTypeId}
                      placeholder="例如 FrontierFarm"
                      onChange={(event) => setManualFarmTypeId(event.target.value)}
                    />
                  </label>
                  <p>必须填写 `Data/AdditionalFarms` 中的 Id；未知 Id 不会回落标准农场，创建时仍会进行运行时验证。</p>
                  <button type="button" disabled={!isSafeManualFarmTypeId(manualFarmTypeId)} onClick={() => set('farmType', manualFarmTypeId.trim())}>使用该 Id</button>
                  {compatibilityShortcut ? <small>兼容值 `modded` 当前只对应一个可创建农场：{compatibilityShortcut.label}；仍推荐使用明确 Id。</small> : null}
                  {farmCatalog.modded.filter((farm) => canSelectModFarm(farm, true)).length > 1 ? <small>检测到多个可用模组农场，加载顺序不稳定，必须显式选择 Id。</small> : null}
                </div>
              ) : null}
            </div>
          )}
        </section>

        {submitError && <p className="ngc-submit-error">{submitError}</p>}
        <button className="ngc-submit" type="submit" disabled={submitting || !selectedFarmAllowed || !cfg.farmName.trim() || !cfg.farmerName?.trim()}>
          {submitting ? '正在创建…' : '确认并创建存档'}
        </button>
      </main>

      <aside className="ngc-map-panel" aria-label="选择农场">
        {builtinFarms.map((farm) => (
          <button key={farm.id} type="button" className={cfg.farmType === farm.id ? 'selected' : ''} onClick={() => set('farmType', farm.id)}>
            <img src={farm.asset} alt="" onError={(event) => { event.currentTarget.style.display = 'none' }} />
            <span>{farm.label}</span>
          </button>
        ))}
      </aside>
      </form>
      {prepareTarget?.modSelection ? (
        <div className="ngc-prepare-overlay" role="presentation">
          <section className="ngc-prepare-dialog" role="dialog" aria-modal="true" aria-labelledby="ngc-prepare-title">
            <h3 id="ngc-prepare-title">准备“{prepareTarget.label}”所需 Mod</h3>
            <p>将启用以下组件。此操作不会创建存档，也不会启动服务器：</p>
            <ul>
              {farmComponentsToEnable(prepareTarget).map((component) => (
                <li key={component.key}>
                  <strong>{component.name || component.uniqueId || component.folderName}</strong>
                  {component.version ? ` · v${component.version}` : ''}
                  {component.uniqueId ? <code>{component.uniqueId}</code> : null}
                </li>
              ))}
            </ul>
            {prepareError ? <p className="ngc-submit-error">{prepareError}</p> : null}
            <div className="ngc-prepare-actions">
              <button type="button" onClick={closePrepare} disabled={prepareBusy}>取消</button>
              <button type="button" onClick={() => void confirmPrepare()} disabled={prepareBusy}>{prepareBusy ? '正在准备…' : '确认启用'}</button>
            </div>
          </section>
        </div>
      ) : null}
    </>
  )
}
