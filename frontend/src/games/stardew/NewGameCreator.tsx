import { useEffect, useRef, useState } from 'react'
import { getCustomNewGameCatalog, refreshCustomNewGameCatalog } from '../../api'
import type { CatalogItem, CatalogResponse, NewGameConfig } from '../../types'
import './NewGameCreator.css'

// ── Defaults ──────────────────────────────────────────────────────────────────

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
  }
}

// ── Sub-components ─────────────────────────────────────────────────────────────

function SkeletonGrid({ count, aspect }: { count: number; aspect: string }) {
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: `repeat(auto-fill, minmax(${aspect === 'square' ? '68px' : '110px'}, 1fr))`,
        gap: '8px',
      }}
    >
      {Array.from({ length: count }).map((_, i) => (
        <div
          key={i}
          className="ngc-skeleton"
          style={{
            width: '100%',
            aspectRatio: aspect === 'square' ? '1' : '240 / 132',
          }}
        />
      ))}
    </div>
  )
}

function ImageCard({
  item,
  selected,
  onSelect,
}: {
  item: CatalogItem
  selected: boolean
  onSelect: () => void
}) {
  return (
    <div
      className={`ngc-image-card${selected ? ' selected' : ''}`}
      onClick={onSelect}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => e.key === 'Enter' && onSelect()}
      aria-pressed={selected}
    >
      {item.image ? (
        <img src={item.image} alt={item.label} loading="lazy" />
      ) : null}
      <div className="ngc-image-card-label">{item.label}</div>
    </div>
  )
}

function BreedCard({
  item,
  selected,
  onSelect,
}: {
  item: CatalogItem
  selected: boolean
  onSelect: () => void
}) {
  return (
    <div
      className={`ngc-breed-card${selected ? ' selected' : ''}`}
      onClick={onSelect}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => e.key === 'Enter' && onSelect()}
      aria-pressed={selected}
    >
      {item.image ? (
        <img src={item.image} alt={item.label} loading="lazy" />
      ) : null}
      <div className="ngc-breed-card-label">{item.label}</div>
    </div>
  )
}

function Chips({
  items,
  value,
  onChange,
}: {
  items: CatalogItem[]
  value: string
  onChange: (id: string) => void
}) {
  return (
    <div className="ngc-chips">
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          className={`ngc-chip${value === item.id ? ' selected' : ''}`}
          onClick={() => onChange(item.id)}
          title={item.description}
        >
          {item.label}
        </button>
      ))}
    </div>
  )
}

// ── Status banner ──────────────────────────────────────────────────────────────

function StatusBanner({
  catalog,
  refreshing,
  onRegenerate,
}: {
  catalog: CatalogResponse
  refreshing: boolean
  onRegenerate: () => void
}) {
  if (catalog.status === 'ready') return null

  if (catalog.status === 'generating') {
    return (
      <div className="ngc-status-banner generating">
        <span className="ngc-spinner" />
        <span>正在从游戏文件生成素材目录，请稍候…（页面将自动刷新）</span>
      </div>
    )
  }

  if (catalog.status === 'failed') {
    return (
      <div className="ngc-status-banner failed">
        <span>素材导出失败：{catalog.error || '未知错误'}</span>
        <button type="button" onClick={onRegenerate} disabled={refreshing}>
          {refreshing ? '正在重新生成…' : '重新生成素材'}
        </button>
      </div>
    )
  }

  // unavailable
  return (
    <div className="ngc-status-banner unavailable">
      <span>游戏素材尚未生成。安装完成后素材目录将自动创建；也可点击按钮手动生成。</span>
      <button type="button" onClick={onRegenerate} disabled={refreshing}>
        {refreshing ? '正在生成…' : '生成素材'}
      </button>
    </div>
  )
}

// ── Main Component ─────────────────────────────────────────────────────────────

type Props = {
  instanceId: string
  onSubmit: (cfg: NewGameConfig) => void
  submitting?: boolean
  submitError?: string
}

export function NewGameCreator({ instanceId, onSubmit, submitting, submitError }: Props) {
  const [catalog, setCatalog] = useState<CatalogResponse | null>(null)
  const [fetchError, setFetchError] = useState<string | null>(null)
  const [refreshing, setRefreshing] = useState(false)
  const [cfg, setCfg] = useState<NewGameConfig>(defaultConfig)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // Fetch catalog on mount.
  useEffect(() => {
    getCustomNewGameCatalog(instanceId)
      .then(setCatalog)
      .catch((e) => setFetchError(String(e?.message ?? e)))
  }, [instanceId])

  // Poll every 5 seconds while status is "generating".
  useEffect(() => {
    if (catalog?.status !== 'generating') {
      if (pollRef.current !== null) {
        clearInterval(pollRef.current)
        pollRef.current = null
      }
      return
    }
    if (pollRef.current !== null) return // already polling
    pollRef.current = setInterval(() => {
      getCustomNewGameCatalog(instanceId)
        .then((c) => {
          setCatalog(c)
          if (c.status !== 'generating' && pollRef.current !== null) {
            clearInterval(pollRef.current)
            pollRef.current = null
          }
        })
        .catch(() => {/* ignore poll errors */})
    }, 5000)
    return () => {
      if (pollRef.current !== null) {
        clearInterval(pollRef.current)
        pollRef.current = null
      }
    }
  }, [catalog?.status, instanceId])

  function set<K extends keyof NewGameConfig>(key: K, value: NewGameConfig[K]) {
    setCfg((prev) => ({ ...prev, [key]: value }))
  }

  function handleRegenerate() {
    setRefreshing(true)
    setFetchError(null)
    refreshCustomNewGameCatalog(instanceId)
      .then(setCatalog)
      .catch((e) => setFetchError(String(e?.message ?? e)))
      .finally(() => setRefreshing(false))
  }

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const petBreedInt = parseInt(cfg.petBreedId ?? '0', 10)
    onSubmit({ ...cfg, petBreed: isNaN(petBreedInt) ? 0 : Math.max(0, Math.min(3, petBreedInt)) })
  }

  // Breeds filtered to the selected pet type.
  const visibleBreeds = catalog
    ? catalog.petBreeds.filter((b) => !b.group || b.group === cfg.petType)
    : []

  const assetsReady = catalog?.status === 'ready'

  return (
    <form className="ngc-root" onSubmit={handleSubmit}>
      {/* Status banner (replaces old fallback banner) */}
      {catalog && (
        <StatusBanner
          catalog={catalog}
          refreshing={refreshing}
          onRegenerate={handleRegenerate}
        />
      )}

      {/* Fetch error (network / 5xx) */}
      {fetchError && (
        <div className="ngc-error">
          <span>加载素材失败：{fetchError}</span>
          <button type="button" onClick={handleRegenerate}>
            重试
          </button>
        </div>
      )}

      {/* ── 基本信息 ── */}
      <div className="ngc-section">
        <p className="ngc-section-title">基本信息</p>
        <div className="ngc-row">
          <div className="ngc-field">
            <label htmlFor="ngc-farmName">农场名称 *</label>
            <input
              id="ngc-farmName"
              required
              maxLength={100}
              value={cfg.farmName}
              onChange={(e) => set('farmName', e.target.value)}
              placeholder="例如：阳光农场"
            />
          </div>
          <div className="ngc-field">
            <label htmlFor="ngc-farmerName">农民名称</label>
            <input
              id="ngc-farmerName"
              maxLength={100}
              value={cfg.farmerName ?? ''}
              onChange={(e) => set('farmerName', e.target.value)}
              placeholder="留空则使用默认值"
            />
          </div>
        </div>
        <div className="ngc-field">
          <label htmlFor="ngc-favoriteThing">最喜欢的东西</label>
          <input
            id="ngc-favoriteThing"
            maxLength={100}
            value={cfg.favoriteThing ?? ''}
            onChange={(e) => set('favoriteThing', e.target.value)}
            placeholder="留空则使用默认值"
          />
        </div>
      </div>

      {/* ── 农场类型 ── */}
      <div className="ngc-section">
        <p className="ngc-section-title">农场类型</p>
        {!catalog && !fetchError ? (
          <SkeletonGrid count={7} aspect="farm" />
        ) : assetsReady ? (
          <div className="ngc-image-grid">
            {(catalog?.farmTypes ?? []).map((item) => (
              <ImageCard
                key={item.id}
                item={item}
                selected={cfg.farmType === item.id}
                onSelect={() => set('farmType', item.id)}
              />
            ))}
          </div>
        ) : (
          <Chips
            items={catalog?.farmTypes ?? defaultFarmTypeChips}
            value={cfg.farmType}
            onChange={(id) => set('farmType', id)}
          />
        )}
      </div>

      {/* ── 性别 ── */}
      <div className="ngc-section">
        <p className="ngc-section-title">性别</p>
        {!catalog && !fetchError ? (
          <SkeletonGrid count={2} aspect="square" />
        ) : (
          <Chips
            items={catalog?.genders ?? defaultGenderChips}
            value={cfg.gender ?? 'male'}
            onChange={(id) => set('gender', id)}
          />
        )}
      </div>

      {/* ── 宠物 ── */}
      <div className="ngc-section">
        <p className="ngc-section-title">宠物</p>
        {!catalog && !fetchError ? (
          <SkeletonGrid count={6} aspect="square" />
        ) : (
          <>
            {/* Pet type */}
            {assetsReady ? (
              <div className="ngc-pet-type-row">
                {(catalog?.petTypes ?? []).map((item) => (
                  <div
                    key={item.id}
                    className={`ngc-pet-type-card${cfg.petType === item.id ? ' selected' : ''}`}
                    onClick={() => {
                      set('petType', item.id)
                      const firstBreed = catalog?.petBreeds.find(
                        (b) => !b.group || b.group === item.id,
                      )
                      if (firstBreed) set('petBreedId', firstBreed.id)
                    }}
                    role="button"
                    tabIndex={0}
                    onKeyDown={(e) => e.key === 'Enter' && set('petType', item.id)}
                    aria-pressed={cfg.petType === item.id}
                  >
                    {item.image && (
                      <img src={item.image} alt={item.label} loading="lazy" />
                    )}
                    <span>{item.label}</span>
                  </div>
                ))}
              </div>
            ) : (
              <Chips
                items={catalog?.petTypes ?? defaultPetTypeChips}
                value={cfg.petType ?? 'Cat'}
                onChange={(id) => {
                  set('petType', id)
                  const firstBreed = catalog?.petBreeds.find(
                    (b) => !b.group || b.group === id,
                  )
                  if (firstBreed) set('petBreedId', firstBreed.id)
                }}
              />
            )}
            {/* Breed selection */}
            {visibleBreeds.length > 0 && (
              <>
                <p className="ngc-section-title" style={{ marginTop: 8 }}>
                  品种
                </p>
                {assetsReady ? (
                  <div className="ngc-breed-grid">
                    {visibleBreeds.map((item) => (
                      <BreedCard
                        key={`${item.group}-${item.id}`}
                        item={item}
                        selected={cfg.petBreedId === item.id}
                        onSelect={() => set('petBreedId', item.id)}
                      />
                    ))}
                  </div>
                ) : (
                  <Chips
                    items={visibleBreeds}
                    value={cfg.petBreedId ?? '0'}
                    onChange={(id) => set('petBreedId', id)}
                  />
                )}
              </>
            )}
          </>
        )}
      </div>

      {/* ── 多人设置 ── */}
      <div className="ngc-section">
        <p className="ngc-section-title">多人设置</p>
        <div className="ngc-option-row">
          <label className="ngc-section-title" style={{ fontSize: 12, textTransform: 'none' }}>
            联机小屋数量
          </label>
          <Chips
            items={catalog?.cabinCounts ?? defaultCabinCounts}
            value={String(cfg.startingCabins)}
            onChange={(id) => set('startingCabins', parseInt(id, 10))}
          />
        </div>
        <div className="ngc-option-row">
          <label className="ngc-section-title" style={{ fontSize: 12, textTransform: 'none' }}>
            小屋布局
          </label>
          <Chips
            items={catalog?.cabinLayouts ?? defaultCabinLayouts}
            value={cfg.cabinLayout}
            onChange={(id) => set('cabinLayout', id)}
          />
        </div>
        <div className="ngc-option-row">
          <label className="ngc-section-title" style={{ fontSize: 12, textTransform: 'none' }}>
            利润倍率
          </label>
          <Chips
            items={catalog?.profitMargins ?? defaultProfitMargins}
            value={cfg.profitMargin}
            onChange={(id) => set('profitMargin', id)}
          />
        </div>
        <div className="ngc-option-row">
          <label className="ngc-section-title" style={{ fontSize: 12, textTransform: 'none' }}>
            资金模式
          </label>
          <Chips
            items={catalog?.moneyModes ?? defaultMoneyModes}
            value={cfg.moneyMode}
            onChange={(id) => set('moneyMode', id)}
          />
        </div>
      </div>

      {/* Submit */}
      {submitError && (
        <div className="ngc-error">
          <span>{submitError}</span>
        </div>
      )}
      <button
        type="submit"
        disabled={submitting || !cfg.farmName.trim()}
        style={{
          padding: '10px 24px',
          background: '#4a7c42',
          color: '#fff',
          border: 'none',
          borderRadius: '8px',
          fontSize: '15px',
          fontWeight: 600,
          cursor: 'pointer',
          alignSelf: 'flex-start',
          opacity: submitting || !cfg.farmName.trim() ? 0.6 : 1,
        }}
      >
        {submitting ? '启动中…' : '新建存档并启动'}
      </button>
    </form>
  )
}

// ── Static fallback option lists ───────────────────────────────────────────────

const defaultFarmTypeChips: CatalogItem[] = [
  { id: 'standard', label: '标准农场' },
  { id: 'riverland', label: '河边农场' },
  { id: 'forest', label: '森林农场' },
  { id: 'hilltop', label: '山顶农场' },
  { id: 'wilderness', label: '荒野农场' },
  { id: 'four_corners', label: '四角农场' },
  { id: 'beach', label: '海滩农场' },
]

const defaultGenderChips: CatalogItem[] = [
  { id: 'male', label: '男' },
  { id: 'female', label: '女' },
]

const defaultPetTypeChips: CatalogItem[] = [
  { id: 'Cat', label: '猫' },
  { id: 'Dog', label: '狗' },
]

const defaultCabinCounts: CatalogItem[] = [
  { id: '0', label: '1 人' },
  { id: '1', label: '2 人' },
  { id: '2', label: '3 人' },
  { id: '3', label: '4 人' },
]

const defaultCabinLayouts: CatalogItem[] = [
  { id: 'nearby', label: '靠近' },
  { id: 'separate', label: '分散' },
]

const defaultProfitMargins: CatalogItem[] = [
  { id: '100', label: '100%' },
  { id: '75', label: '75%' },
  { id: '50', label: '50%' },
  { id: '25', label: '25%' },
]

const defaultMoneyModes: CatalogItem[] = [
  { id: 'shared', label: '共享资金' },
  { id: 'separate', label: '分开资金' },
]
