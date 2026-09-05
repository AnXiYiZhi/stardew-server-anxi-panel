import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { CSSProperties, FormEvent, KeyboardEvent as ReactKeyboardEvent, PointerEvent as ReactPointerEvent, ReactNode } from 'react'
import { ApiError, createInstance, getInstances, getInstancePublicIP, getInstanceState, getJobs, getStardewGameInstallation, startInstance, stopInstance } from '../api'
import { stardewInstallPath } from '../app-routes'
import { errorMessage, stateLabel } from '../core/helpers'
import type { CurrentUser, GameInstallation } from '../types'
import { routeToPath } from './stardew/stardew-routes'
import { classifyInstallationState } from './stardew/installation-state'
import { canonicalInstallJobs } from './stardew/install-state'
import { shouldClearPendingStartupAction } from './stardew/lifecycle-action-state'
import { copyText } from './stardew/copy-text'
import { GameInstallRail } from './GameInstallRail'
import type { GameInstallProgressPresentation } from './stardew/install-progress-presentation'
import {
  canCreateWorld,
  gameCardNavigationIndex,
  gameLibraryBackgroundForDate,
  gameLibraryManualBackgroundPreference,
  gameLibraryNextBackgroundBoundary,
  initialStardewCatalogItem,
  stardewCatalogItems,
  stardewCreateInstanceRequest,
  stardewGameCardAriaLabel,
  stardewGameDestination,
  stardewInstallCardState,
  stardewInstanceDestination,
  stardewJoinAddress,
  stardewJoinAddressValue,
  stardewRequiresSave,
  stardewShouldLoadConnection,
  stardewWorldLifecycleControl,
  type GameLibraryBackground,
  type StardewCatalogItem,
  type StardewWorldLifecycleIntent,
} from './game-library-state'
import './GameLibrary.css'
import { canDeleteWorld } from './world-delete-gesture'
import { useWorldDeletePress, WorldDeleteDialog } from './WorldDeleteControl'
import { getSaves, getFarmTypeCatalog, renameInstance } from '../api'
import { builtinFarms } from './stardew/new-game-farms'
const defaultWorldFarmIcon = '/assets/stardew/new-game/farms/standard.png'

type Navigate = (path: string, replace?: boolean) => void

type GameHubShellProps = {
  user: CurrentUser
  title?: string
  eyebrow?: string
  variant?: 'library' | 'stardew'
  background?: GameLibraryBackground
  onToggleBackground?: () => void
  onNavigate: Navigate
  onLogout: () => void
  children: ReactNode
}

type CatalogState = {
  items: StardewCatalogItem[]
  loading: boolean
  error: string | null
}

const EMPTY_CATALOG: CatalogState = { items: [], loading: true, error: null }

function useStardewCatalog(): CatalogState & {
  refresh: () => void
  refreshInstance: (instanceId: string) => void
  removeInstance: (instanceId: string) => void
} {
  const [catalog, setCatalog] = useState<CatalogState>(EMPTY_CATALOG)
  const [refreshNonce, setRefreshNonce] = useState(0)
  const mountedRef = useRef(true)
  const instanceRefreshesRef = useRef(new Set<string>())

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      instanceRefreshesRef.current.clear()
    }
  }, [])

  useEffect(() => {
    let active = true
    const connectionRequests = new Set<string>()

    function updateItem(instanceId: string, patch: Partial<StardewCatalogItem>) {
      if (!active) return
      setCatalog((current) => ({
        ...current,
        items: current.items.map((item) => item.instance.id === instanceId ? { ...item, ...patch } : item),
      }))
    }

    function loadConnection(instanceId: string) {
      if (connectionRequests.has(instanceId)) return
      connectionRequests.add(instanceId)
      updateItem(instanceId, { connectionLoading: true, connectionError: null })
      void getInstancePublicIP(instanceId)
        .then((connection) => updateItem(instanceId, { connection, connectionLoading: false, connectionError: null }))
        .catch((error) => updateItem(instanceId, {
          connection: null,
          connectionLoading: false,
          connectionError: errorMessage(error),
        }))
    }

    async function load() {
      setCatalog((current) => ({ ...current, loading: current.items.length === 0, error: null }))
      const jobsPromise = getJobs().catch(() => ({ jobs: [] }))
      try {
        const response = await getInstances()
        const instances = stardewCatalogItems(response.instances)
        const items = instances.map(initialStardewCatalogItem)
        if (!active) return
        setCatalog({ items, loading: false, error: null })

        void jobsPromise.then((jobsResponse) => {
          if (!active) return
          setCatalog((current) => ({
            ...current,
            items: current.items.map((item) => {
              const instanceJobs = jobsResponse.jobs.filter(
                (job) => job.targetType === 'instance' && job.targetId === item.instance.id,
              )
              return {
                ...item,
                hasActiveInstallJob: canonicalInstallJobs(instanceJobs, null).active !== null,
                activeLifecycleJob: instanceJobs.find(
                  (job) => job.type === 'stardew_lifecycle' && (job.status === 'running' || job.status === 'queued'),
                ) ?? null,
              }
            }),
          }))
        })

        for (const item of items) {
          if (item.connectionLoading) loadConnection(item.instance.id)
          void getInstanceState(item.instance.id)
            .then((state) => {
              updateItem(item.instance.id, { state, stateLoading: false, stateError: null })
              if (stardewShouldLoadConnection({ ...item, state })) loadConnection(item.instance.id)
            })
            .catch((error) => updateItem(item.instance.id, {
              stateLoading: false,
              stateError: errorMessage(error),
            }))
        }
      } catch (error) {
        if (active) {
          setCatalog((current) => ({ ...current, loading: false, error: errorMessage(error) }))
        }
      }
    }

    void load()
    return () => {
      active = false
    }
  }, [refreshNonce])

  const refresh = useCallback(() => setRefreshNonce((value) => value + 1), [])
  const refreshInstance = useCallback((instanceId: string) => {
    if (instanceRefreshesRef.current.has(instanceId)) return
    instanceRefreshesRef.current.add(instanceId)

    const updateItem = (patch: Partial<StardewCatalogItem>) => {
      if (!mountedRef.current) return
      setCatalog((current) => ({
        ...current,
        items: current.items.map((item) => item.instance.id === instanceId ? { ...item, ...patch } : item),
      }))
    }
    const stateRequest = getInstanceState(instanceId)
      .then((state) => updateItem({ state, stateLoading: false, stateError: null }))
      .catch((error) => updateItem({ stateLoading: false, stateError: errorMessage(error) }))
    const jobsRequest = getJobs()
      .then((jobsResponse) => {
        const instanceJobs = jobsResponse.jobs.filter(
          (job) => job.targetType === 'instance' && job.targetId === instanceId,
        )
        updateItem({
          hasActiveInstallJob: canonicalInstallJobs(instanceJobs, null).active !== null,
          activeLifecycleJob: instanceJobs.find(
            (job) => job.type === 'stardew_lifecycle' && (job.status === 'running' || job.status === 'queued'),
          ) ?? null,
        })
      })
      .catch(() => undefined)

    void Promise.allSettled([stateRequest, jobsRequest]).finally(() => {
      instanceRefreshesRef.current.delete(instanceId)
    })
  }, [])

  const removeInstance = useCallback((instanceId: string) => setCatalog(current => ({ ...current, items: current.items.filter(item => item.instance.id !== instanceId) })), [])
  return { ...catalog, refresh, refreshInstance, removeInstance }
}

type InstallationState = {
  data: GameInstallation | null
  loading: boolean
  error: string | null
}

function useStardewInstallation(): InstallationState {
  const [state, setState] = useState<InstallationState>({ data: null, loading: true, error: null })
  useEffect(() => {
    let active = true
    getStardewGameInstallation()
      .then((data) => {
        if (active) setState({ data, loading: false, error: null })
      })
      .catch((error) => {
        if (active) setState({ data: null, loading: false, error: errorMessage(error) })
      })
    return () => {
      active = false
    }
  }, [])
  return state
}

function GameHubShell({
  user,
  title,
  eyebrow,
  variant = 'library',
  background = 'day',
  onToggleBackground,
  onNavigate,
  onLogout,
  children,
}: GameHubShellProps) {
  const backgroundClass = variant === 'library' ? ` game-hub--background-${background}` : ''
  const hasHeading = Boolean(title || eyebrow)
  return (
    <div className={`game-hub game-hub--${variant}${backgroundClass}`}>
      <header className="game-hub-topbar">
        <button type="button" className="game-hub-brand" onClick={() => onNavigate('/games')}>
          <strong>ANXI PANEL</strong>
        </button>
        <div className="game-hub-account" aria-label="账号区域">
          <strong className="game-hub-account-name">{user.username}</strong>
          <button type="button" className="game-hub-logout" onClick={onLogout}>退出</button>
          {variant === 'library' && onToggleBackground ? (
            <button
              type="button"
              className="game-hub-theme-toggle"
              aria-label={background === 'day' ? '切换到静夜背景' : '切换到暖昼背景'}
              aria-pressed={background === 'night'}
              onClick={onToggleBackground}
            >
              {background === 'day' ? (
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <circle cx="12" cy="12" r="3.5" />
                  <path d="M12 2v2.2M12 19.8V22M4.93 4.93l1.56 1.56M17.51 17.51l1.56 1.56M2 12h2.2M19.8 12H22M4.93 19.07l1.56-1.56M17.51 6.49l1.56-1.56" />
                </svg>
              ) : (
                <svg viewBox="0 0 24 24" aria-hidden="true">
                  <path d="M20.2 15.25A8.4 8.4 0 0 1 8.75 3.8 8.5 8.5 0 1 0 20.2 15.25Z" />
                </svg>
              )}
            </button>
          ) : null}
        </div>
      </header>

      <main className={`game-hub-main${hasHeading ? '' : ' game-hub-main--library'}`}>
        {hasHeading ? (
          <header className="game-hub-heading">
            {eyebrow ? <p>{eyebrow}</p> : null}
            {title ? <h1>{title}</h1> : null}
          </header>
        ) : null}
        {children}
      </main>
    </div>
  )
}

function CatalogNotice({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="game-hub-notice" role="alert">
      <span>{message}</span>
      <button type="button" onClick={onRetry}>重新读取</button>
    </div>
  )
}

function instanceStatus(
  item: StardewCatalogItem,
  lifecycle: ReturnType<typeof stardewWorldLifecycleControl>,
): { label: string; tone: string } {
  if (lifecycle.busy) return { label: lifecycle.label.replace(/…$/, ''), tone: 'working' }
  if (stardewRequiresSave(item)) return { label: '需要存档', tone: 'error' }
  if (item.stateError) return { label: '状态读取失败', tone: 'error' }
  if (!item.state) return { label: '状态未知', tone: 'unknown' }
  const installation = classifyInstallationState(item.state, item.hasActiveInstallJob)
  if (installation.kind === 'installing') return { label: '安装中', tone: 'working' }
  if (installation.kind === 'not_installed') return { label: '未安装', tone: 'idle' }
  if (installation.kind === 'repair_required' || installation.kind === 'install_failed') {
    return { label: '需要修复', tone: 'error' }
  }
  if (item.state.state === 'running') return { label: '运行中', tone: 'running' }
  if (item.state.state === 'stopped') return { label: '已停止', tone: 'stopped' }
  return { label: stateLabel(item.state.state), tone: 'unknown' }
}

type WorldRailClosePhase = 'idle' | 'closing'

type WorldRailProps = {
  catalog: CatalogState & {
    refresh: () => void
    refreshInstance: (instanceId: string) => void
    removeInstance: (instanceId: string) => void
  }
  user: CurrentUser
  open: boolean
  installOpen: boolean
  installInstanceId: string
  requestedInstallJobId?: string
  createWorldOpen: boolean
  closePhase: WorldRailClosePhase
  onNavigate: Navigate
  onInstallPresentationChange: (presentation: GameInstallProgressPresentation | null) => void
}

const WORLD_RAIL_OPEN_MS = 360
const WORLD_RAIL_CLOSE_MS = 360

function InlineWorldCreateCard({ onNavigate }: { onNavigate: Navigate }) {
  const installation = useStardewInstallation()
  const nameInputRef = useRef<HTMLInputElement | null>(null)
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const installationReady = installation.data?.installed === true
  const canSubmit = installationReady && !installation.loading && !busy && name.trim().length > 0

  useLayoutEffect(() => {
    nameInputRef.current?.focus({ preventScroll: true })
  }, [])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!canSubmit) return
    setBusy(true)
    setSubmitError(null)
    try {
      const response = await createInstance(stardewCreateInstanceRequest(name))
      onNavigate(routeToPath('saves', undefined, response.instance.id))
    } catch (error) {
      setSubmitError(errorMessage(error))
    } finally {
      setBusy(false)
    }
  }

  const statusMessage = submitError
    ?? installation.error
    ?? (installation.loading ? '正在确认游戏安装状态…' : !installationReady ? '需要先安装游戏' : null)

  return (
    <form className="world-create-card world-create-card--form" onSubmit={submit} aria-busy={busy}>
      <strong>请输入世界名称</strong>
      <label className="world-create-field">
        <span>世界名称</span>
        <input
          ref={nameInputRef}
          value={name}
          onChange={(event) => setName(event.target.value)}
          maxLength={40}
          placeholder="例如：河湾农场"
          autoComplete="off"
          disabled={busy}
        />
      </label>
      {statusMessage ? (
        <span className={`world-create-form-message${submitError || installation.error ? ' is-error' : ''}`} role={submitError || installation.error ? 'alert' : 'status'}>
          {statusMessage}
        </span>
      ) : null}
      <span className="world-create-form-actions">
        <button type="button" onClick={() => onNavigate('/games/stardew')} disabled={busy}>取消</button>
        <button type="submit" disabled={!canSubmit}>{busy ? '正在创建…' : '创建世界'}</button>
      </span>
    </form>
  )
}

type CopyFeedback = 'idle' | 'copied' | 'failed'

function WorldChoiceCard({
  item,
  user,
  onOpen,
  onNavigate,
  onRefresh,
  onDeleted,
}: {
  item: StardewCatalogItem
  index: number
  user: CurrentUser
  onOpen: (item: StardewCatalogItem) => void
  onNavigate: Navigate
  onRefresh: (instanceId: string) => void
  onDeleted: () => void
}) {
  const destination = stardewInstanceDestination(item)
  const [worldName, setWorldName] = useState(item.instance.name)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [deleted, setDeleted] = useState(false)
  const deletionPending = item.instance.driverPhase === 'instance_deleting'
  const deletionAllowed = canDeleteWorld(user.role, item.instance.isDefault)
  const openButton = useRef<HTMLButtonElement>(null)
  const { pressing, gesture } = useWorldDeletePress(() => setDeleteOpen(true), deletionAllowed && !deleteOpen)
  function closeDelete() { setDeleteOpen(false); window.setTimeout(() => openButton.current?.focus({ preventScroll: true }), 0) }
  const actionLabel = destination === 'install'
    ? '查看安装流程'
    : destination === 'saves'
      ? '前往创建或上传存档'
      : `进入${worldName}`
  const panelAccessHost = window.location.hostname
  const joinAddress = stardewJoinAddress(item, panelAccessHost)
  const copyValue = stardewJoinAddressValue(item, panelAccessHost)
  const [copyFeedback, setCopyFeedback] = useState<CopyFeedback>('idle')
  const [editingName, setEditingName] = useState(false)
  const [nameDraft, setNameDraft] = useState('')
  const [savingName, setSavingName] = useState(false)
  const [nameError, setNameError] = useState<string | null>(null)
  const [farmIcon, setFarmIcon] = useState(defaultWorldFarmIcon)
  useEffect(() => { setWorldName(item.state?.name ?? item.instance.name) }, [item.state?.name, item.instance.name])
  useEffect(() => {
    let canceled = false
    async function loadFarmIcon() {
      try {
        const result = await getSaves(item.instance.id)
        const active = result.saves.find((save) => save.name === result.activeSaveName || save.isActive)
        const builtinFarm = builtinFarms.find((farm) => farm.id === active?.farmType || farm.label === active?.farmType)
        let icon = builtinFarm?.asset ?? defaultWorldFarmIcon
        if (active?.farmType && !builtinFarm && user.role === 'admin') {
          const catalog = await getFarmTypeCatalog(item.instance.id)
          icon = catalog.farmTypes.find((farm) => farm.id === active.farmType)?.iconUrl || defaultWorldFarmIcon
        }
        if (!canceled) setFarmIcon(icon)
      } catch { if (!canceled) setFarmIcon(defaultWorldFarmIcon) }
    }
    void loadFarmIcon()
    return () => { canceled = true }
  }, [item.instance.id, item.state?.updatedAt, user.role])

  async function saveName(event: FormEvent) {
    event.preventDefault()
    if (savingName || !nameDraft.trim()) return
    setSavingName(true)
    setNameError(null)
    try {
      const response = await renameInstance(item.instance.id, nameDraft.trim())
      setWorldName(response.instance.name)
      setEditingName(false)
      onRefresh(item.instance.id)
    } catch (error) { setNameError(errorMessage(error)) }
    finally { setSavingName(false) }
  }
  const copyResetTimerRef = useRef<number | null>(null)
  const [pendingLifecycle, setPendingLifecycle] = useState<StardewWorldLifecycleIntent>(null)
  const [pendingStartupSawActiveJob, setPendingStartupSawActiveJob] = useState(false)
  const [lifecycleError, setLifecycleError] = useState<string | null>(null)
  const lifecycle = stardewWorldLifecycleControl(item, pendingLifecycle, user.role)
  const status = instanceStatus(item, lifecycle)
  const isRunning = item.state?.state === 'running' || item.state?.uiStatus === 'ready'

  useEffect(() => () => {
    if (copyResetTimerRef.current !== null) window.clearTimeout(copyResetTimerRef.current)
  }, [])

  useEffect(() => {
    if (pendingLifecycle === 'start' && item.activeLifecycleJob) {
      setPendingStartupSawActiveJob(true)
    }
  }, [item.activeLifecycleJob, pendingLifecycle])

  useEffect(() => {
    if (shouldClearPendingStartupAction({
      action: pendingLifecycle === 'start' ? 'start' : null,
      hasActiveLifecycleJob: Boolean(item.activeLifecycleJob),
      isRunning,
      sawActiveLifecycleJob: pendingStartupSawActiveJob,
    })) {
      setPendingLifecycle(null)
      setPendingStartupSawActiveJob(false)
    }
  }, [isRunning, item.activeLifecycleJob, pendingLifecycle, pendingStartupSawActiveJob])

  useEffect(() => {
    if (pendingLifecycle !== 'stop') return
    const state = item.state?.state ?? item.instance.state
    if (state === 'stopped' || state === 'ready_to_start' || state === 'game_installed' || state === 'save_required' || state === 'error') {
      setPendingLifecycle(null)
    }
  }, [item.instance.state, item.state?.state, pendingLifecycle])

  useEffect(() => {
    if (!pendingLifecycle && !lifecycle.busy) return
    const interval = window.setInterval(() => onRefresh(item.instance.id), 1_500)
    return () => window.clearInterval(interval)
  }, [item.instance.id, lifecycle.busy, onRefresh, pendingLifecycle])

  async function handleCopy() {
    if (!copyValue) return
    const copied = await copyText(copyValue)
    setCopyFeedback(copied ? 'copied' : 'failed')
    if (copyResetTimerRef.current !== null) window.clearTimeout(copyResetTimerRef.current)
    copyResetTimerRef.current = window.setTimeout(() => {
      copyResetTimerRef.current = null
      setCopyFeedback('idle')
    }, 1_800)
  }

  async function handleLifecycle() {
    if (!lifecycle.action || lifecycle.disabled) return
    const intent = lifecycle.action
    setPendingLifecycle(intent)
    setPendingStartupSawActiveJob(false)
    setLifecycleError(null)
    try {
      if (intent === 'start') await startInstance(item.instance.id)
      else await stopInstance(item.instance.id)
      onRefresh(item.instance.id)
    } catch (error) {
      setPendingLifecycle(null)
      if (error instanceof ApiError && ['save_required', 'active_save_required', 'active_save_missing'].includes(error.code)) {
        onNavigate(routeToPath('saves', undefined, item.instance.id))
        return
      }
      setLifecycleError(errorMessage(error))
    }
  }

  const lifecycleAriaLabel = lifecycle.action
    ? `${lifecycle.label}${worldName}服务器${lifecycle.disabled ? '，仅管理员可操作' : ''}`
    : `${worldName}服务器，${lifecycle.label}`
  const copyAriaLabel = copyFeedback === 'copied'
    ? '加入地址已复制'
    : copyFeedback === 'failed'
      ? '复制失败，请重试'
      : `复制${worldName}加入地址`

  if (deleted) return null
  return (
    <article className={`world-choice${pressing ? ' is-delete-pressing' : ''}`}>
      <button
        ref={openButton}
        type="button"
        className="world-choice-open"
        title={worldName}
        aria-label={`${worldName}，${status.label}，加入地址 ${joinAddress}，${actionLabel}`}
        aria-keyshortcuts={deletionAllowed ? 'Delete' : undefined}
        onKeyDown={(event) => { if (deletionAllowed && event.key === 'Delete') { event.preventDefault(); setDeleteOpen(true) } }}
        onPointerDown={(event) => { if (deletionAllowed && !deleteOpen && event.button === 0 && event.isPrimary) gesture.start(event.pointerId, event.clientX, event.clientY) }}
        onPointerMove={(event) => gesture.move(event.pointerId, event.clientX, event.clientY)}
        onPointerUp={() => gesture.release()}
        onPointerLeave={() => gesture.cancel()}
        onPointerCancel={() => gesture.cancel()}
        onLostPointerCapture={() => gesture.cancel()}
        onContextMenu={(event) => { if (deletionAllowed) event.preventDefault() }}
        onClick={(event) => { if (event.detail !== 0 && gesture.consumeClick()) return; if (!deleteOpen) { if (deletionPending && deletionAllowed) setDeleteOpen(true); else if (!deletionPending) onOpen(item) } }}
      />
      {deletionAllowed ? <span className="world-delete-hint" aria-hidden="true">{pressing ? '继续按住，松开取消' : '长按封面删除 · 键盘 Delete'}</span> : null}
      {pressing ? <span className="world-delete-progress" role="status" aria-label="继续按住以打开删除确认" /> : null}
      {deleteOpen ? <WorldDeleteDialog id={item.instance.id} name={worldName} onClose={closeDelete} onDeleted={() => { setDeleteOpen(false); setDeleted(true); onDeleted() }} /> : null}
      <img className="world-choice-scene" src={farmIcon} alt="" onError={() => setFarmIcon(defaultWorldFarmIcon)} />
      <span className="world-choice-content">
        <span className="world-choice-heading">
          {editingName ? (
            <form className="world-name-editor" onSubmit={saveName}>
              <input autoFocus aria-label="世界名称" value={nameDraft} maxLength={40} disabled={savingName}
                onChange={(event) => setNameDraft(event.target.value)}
                onKeyDown={(event) => { if (event.key === 'Escape' && !savingName) { setEditingName(false); setNameError(null) } }} />
              <button type="submit" disabled={savingName || !nameDraft.trim()}>{savingName ? '保存中…' : '保存'}</button>
              <button type="button" disabled={savingName} onClick={() => { setEditingName(false); setNameError(null) }}>取消</button>
            </form>
          ) : (
            <span className="world-name-row"><span className="world-choice-name">{worldName}</span>
              {user.role === 'admin' ? <button type="button" disabled={deletionPending} className="world-name-edit" aria-label={`修改${worldName}的名称`} title="修改名称"
                onClick={() => { setNameDraft(worldName); setNameError(null); setEditingName(true) }}>
                <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m15 5 4 4M4 20l4-1L20 7a2.8 2.8 0 0 0-4-4L4 15z" /></svg>
              </button> : null}
            </span>
          )}
          <span className={`world-status world-status--${status.tone}`}>
            <span aria-hidden="true" />
            {deletionPending ? '删除未完成，请重试' : status.label}
          </span>
        </span>
        <span className="world-choice-details">
          {nameError ? <span className="world-choice-action-error" role="alert">{nameError}</span> : null}
          <span className="world-choice-address">
            <span>
              <small>加入地址</small>
              <code>{joinAddress}</code>
            </span>
            <button
              type="button"
              className={`world-copy-button${copyFeedback !== 'idle' ? ` is-${copyFeedback}` : ''}`}
              aria-label={copyAriaLabel}
              title={copyAriaLabel}
              disabled={!copyValue}
              onClick={() => void handleCopy()}
            >
              {copyFeedback === 'copied' ? (
                <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m5.5 12.5 4 4 9-10" /></svg>
              ) : (
                <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 8h10v10H8zM5 15H4V4h11v1" /></svg>
              )}
            </button>
          </span>
          <button
            type="button"
            className={`world-lifecycle-button world-lifecycle-button--${lifecycle.tone}`}
            aria-label={lifecycleAriaLabel}
            aria-busy={lifecycle.busy}
            disabled={lifecycle.disabled || deletionPending}
            onClick={() => void handleLifecycle()}
          >
            <span aria-hidden="true" />
            {lifecycle.label}
          </button>
          {lifecycleError ? <span className="world-choice-action-error" role="alert">{lifecycleError}</span> : null}
        </span>
      </span>
    </article>
  )
}

function WorldRail({
  catalog,
  user,
  open,
  installOpen,
  installInstanceId,
  requestedInstallJobId,
  createWorldOpen,
  closePhase,
  onNavigate,
  onInstallPresentationChange,
}: WorldRailProps) {
  const contentRef = useRef<HTMLDivElement | null>(null)
  const createButtonRef = useRef<HTMLButtonElement | null>(null)
  const previousCreateWorldOpenRef = useRef(createWorldOpen)
  const [contentWidth, setContentWidth] = useState(0)

  useEffect(() => {
    if (open && previousCreateWorldOpenRef.current && !createWorldOpen) {
      window.setTimeout(() => createButtonRef.current?.focus({ preventScroll: true }), 0)
    }
    previousCreateWorldOpenRef.current = createWorldOpen
  }, [createWorldOpen, open])

  useLayoutEffect(() => {
    const content = contentRef.current
    if (!content) return
    const measure = () => {
      const nextWidth = Math.ceil(content.scrollWidth)
      setContentWidth((current) => current === nextWidth ? current : nextWidth)
    }
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(content)
    return () => observer.disconnect()
  }, [catalog.error, catalog.items.length, catalog.loading, createWorldOpen, installOpen, user.role])

  useLayoutEffect(() => {
    const content = contentRef.current
    const picker = content?.closest<HTMLElement>('.game-picker')
    if (!content || !picker) return
    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    let timeout = 0
    let animationFrame = 0

    const animatePickerTo = (targetLeft: number, duration: number, easing: (progress: number) => number) => {
      window.cancelAnimationFrame(animationFrame)
      if (reducedMotion) {
        picker.scrollLeft = Math.max(0, targetLeft)
        return
      }
      const startLeft = picker.scrollLeft
      const distance = Math.max(0, targetLeft) - startLeft
      if (Math.abs(distance) < 0.5) return
      const startedAt = performance.now()
      const animate = (now: number) => {
        const progress = Math.min(1, (now - startedAt) / duration)
        picker.scrollLeft = startLeft + distance * easing(progress)
        if (progress < 1) animationFrame = window.requestAnimationFrame(animate)
      }
      animationFrame = window.requestAnimationFrame(animate)
    }

    const scrollPickerToStart = () => {
      animatePickerTo(0, WORLD_RAIL_CLOSE_MS, (progress) => progress * progress * (3 - 2 * progress))
    }

    const scrollPickerOpenTo = (targetLeft: number) => {
      animatePickerTo(targetLeft, WORLD_RAIL_OPEN_MS, (progress) => progress * progress * (3 - 2 * progress))
    }

    const alignRail = () => {
      if (!open) {
        picker.scrollLeft = 0
        return
      }
      if (closePhase === 'closing') {
        scrollPickerToStart()
        return
      }
      if (contentWidth === 0) return

      const primary = picker.querySelector<HTMLElement>('.game-picker-item--primary')
      const firstChoice = content.querySelector<HTMLElement>('.game-install-inline, .world-choice, .world-create-card')
      if (!primary || !firstChoice) return

      const createForm = content.querySelector<HTMLElement>('.world-create-card--form')
      if (!installOpen && createWorldOpen && createForm) {
        const rail = content.parentElement
        if (!rail) return
        const inset = 20
        const formStart = rail.offsetLeft + createForm.offsetLeft
        const formEnd = formStart + createForm.offsetWidth
        const viewportStart = picker.scrollLeft
        const viewportEnd = viewportStart + picker.clientWidth
        const targetLeft = formEnd > viewportEnd - inset
          ? formEnd - picker.clientWidth + inset
          : formStart < viewportStart + inset
            ? Math.max(0, formStart - inset)
            : viewportStart
        scrollPickerOpenTo(targetLeft)
        return
      }

      const pickerRect = picker.getBoundingClientRect()
      const primaryRect = primary.getBoundingClientRect()
      const contentRect = content.getBoundingClientRect()
      const firstChoiceRect = firstChoice.getBoundingClientRect()
      const clusterStart = primaryRect.left - pickerRect.left + picker.scrollLeft
      const fullClusterWidth = primary.offsetWidth + contentWidth
      const leadingClusterWidth = primary.offsetWidth + firstChoiceRect.right - contentRect.left
      const clusterWidth = fullClusterWidth <= picker.clientWidth
        ? fullClusterWidth
        : leadingClusterWidth <= picker.clientWidth
          ? leadingClusterWidth
          : null

      if (clusterWidth !== null) {
        const clusterCenter = clusterStart + clusterWidth / 2
        scrollPickerOpenTo(Math.max(0, clusterCenter - picker.clientWidth / 2))
        return
      }

      const narrowViewport = window.matchMedia('(max-width: 700px)').matches
      const startLeft = picker.scrollLeft
      firstChoice.scrollIntoView({
        behavior: 'auto',
        block: 'nearest',
        inline: narrowViewport ? 'start' : 'nearest',
      })
      const targetLeft = picker.scrollLeft
      picker.scrollLeft = startLeft
      scrollPickerOpenTo(targetLeft)
    }

    const scheduleAlignment = () => {
      window.clearTimeout(timeout)
      timeout = window.setTimeout(alignRail, 0)
    }

    alignRail()
    window.addEventListener('resize', scheduleAlignment)
    return () => {
      window.clearTimeout(timeout)
      window.cancelAnimationFrame(animationFrame)
      window.removeEventListener('resize', scheduleAlignment)
    }
  }, [closePhase, contentWidth, createWorldOpen, installOpen, open])

  function openInstance(item: StardewCatalogItem) {
    const destination = stardewInstanceDestination(item)
    if (destination === 'install') {
      onNavigate(routeToPath('install', undefined, item.instance.id))
      return
    }
    if (destination === 'saves') {
      onNavigate(routeToPath('saves', undefined, item.instance.id))
      return
    }
    onNavigate(routeToPath('overview', undefined, item.instance.id))
  }

  return (
    <li
      id="stardew-detail-rail"
      className={`game-world-rail${open ? ' is-open' : ''}${closePhase === 'closing' ? ' is-closing' : ''}`}
      style={{ width: open && closePhase === 'idle' ? contentWidth : 0 }}
      aria-hidden={!open || closePhase !== 'idle'}
      inert={!open || closePhase !== 'idle'}
    >
      <div ref={contentRef} className="game-world-rail-content" aria-label={installOpen ? '星露谷安装' : '星露谷服务器实例'}>
        {installOpen ? (
          <GameInstallRail
            key={installInstanceId}
            user={user}
            instanceId={installInstanceId}
            sourceState={catalog.items.find((item) => item.instance.id === installInstanceId)?.state ?? null}
            requestedInstallJobId={requestedInstallJobId}
            onNavigate={onNavigate}
            onCatalogRefresh={catalog.refresh}
            onPresentationChange={onInstallPresentationChange}
          />
        ) : catalog.error ? (
          <div className="world-rail-message world-rail-message--error" role="alert">
            <span>世界读取失败</span>
            <button type="button" onClick={catalog.refresh}>重新读取</button>
          </div>
        ) : null}

        {!installOpen && catalog.loading && catalog.items.length === 0 ? (
          <div className="world-rail-message" aria-live="polite">正在读取世界…</div>
        ) : null}

        {!installOpen && !catalog.loading && catalog.items.length === 0 && !catalog.error ? (
          <div className="world-rail-message">还没有世界</div>
        ) : null}

        {!installOpen && catalog.items.length > 0 ? (
          <ul className="world-choice-list">
            {catalog.items.map((item, index) => (
                <li key={item.instance.id} style={{ transitionDelay: `${index * 34}ms` }}>
                  <WorldChoiceCard
                    item={item}
                    index={index}
                    user={user}
                    onOpen={openInstance}
                    onNavigate={onNavigate}
                    onRefresh={catalog.refreshInstance}
                    onDeleted={() => catalog.removeInstance(item.instance.id)}
                  />
                </li>
              ))}
          </ul>
        ) : null}

        {!installOpen && canCreateWorld(user.role) ? createWorldOpen ? (
          <InlineWorldCreateCard onNavigate={onNavigate} />
        ) : (
          <button
            ref={createButtonRef}
            type="button"
            className="world-create-card"
            onClick={() => onNavigate('/games/stardew/new')}
          >
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" /></svg>
            <strong>新建世界</strong>
          </button>
        ) : null}
      </div>
    </li>
  )
}

const GAME_CARD_ENABLED = [true, false] as const
const GAME_LIBRARY_BACKGROUND_STORAGE_KEY = 'anxi.game-library-background.v1'
const GAME_LIBRARY_BACKGROUND_MANUAL_UNTIL_STORAGE_KEY = 'anxi.game-library-background-manual-until.v1'

type GameLibraryBackgroundSelection = {
  background: GameLibraryBackground
  manualUntil: number | null
}

function storedGameLibraryBackground(): GameLibraryBackgroundSelection {
  const now = new Date()
  const automaticBackground = gameLibraryBackgroundForDate(now)
  if (typeof window === 'undefined') return { background: automaticBackground, manualUntil: null }
  try {
    const manual = gameLibraryManualBackgroundPreference(
      window.localStorage.getItem(GAME_LIBRARY_BACKGROUND_STORAGE_KEY),
      window.localStorage.getItem(GAME_LIBRARY_BACKGROUND_MANUAL_UNTIL_STORAGE_KEY),
      now.getTime(),
    )
    return manual
      ? { background: manual.background, manualUntil: manual.expiresAt }
      : { background: automaticBackground, manualUntil: null }
  } catch {
    return { background: automaticBackground, manualUntil: null }
  }
}

type TouchGesture = {
  pointerId: number
  startX: number
  startY: number
  moved: boolean
}

export function GamesPage({
  user,
  defaultInstanceId,
  worldsOpen = false,
  createWorldOpen = false,
  installOpen = false,
  installTargetId,
  requestedInstallJobId,
  onNavigate,
  onLogout,
}: {
  user: CurrentUser
  defaultInstanceId: string
  worldsOpen?: boolean
  createWorldOpen?: boolean
  installOpen?: boolean
  installTargetId?: string
  requestedInstallJobId?: string
  onNavigate: Navigate
  onLogout: () => void
}) {
  const catalog = useStardewCatalog()
  const destination = stardewGameDestination(catalog.items, defaultInstanceId)
  const installCard = stardewInstallCardState(catalog.items, defaultInstanceId, catalog.loading, catalog.error)
  const railOpen = worldsOpen || installOpen
  const installInstanceId = installTargetId ?? defaultInstanceId
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [backgroundSelection, setBackgroundSelection] = useState<GameLibraryBackgroundSelection>(storedGameLibraryBackground)
  const background = backgroundSelection.background
  const [worldClosePhase, setWorldClosePhase] = useState<WorldRailClosePhase>('idle')
  const worldsClosing = worldClosePhase !== 'idle'
  const [installPresentation, setInstallPresentation] = useState<GameInstallProgressPresentation | null>(null)
  const cardRefs = useRef<Array<HTMLButtonElement | null>>([])
  const touchGestureRef = useRef<TouchGesture | null>(null)
  const suppressClickUntilRef = useRef(0)
  const closeTimerRef = useRef<number | null>(null)

  const completeWorldClose = useCallback(() => {
    onNavigate('/games')
    window.setTimeout(() => cardRefs.current[0]?.focus({ preventScroll: true }), 0)
  }, [onNavigate])

  const closeWorlds = useCallback(() => {
    if (!railOpen || worldClosePhase !== 'idle') return
    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    if (reducedMotion) {
      completeWorldClose()
      return
    }
    setWorldClosePhase('closing')
    if (closeTimerRef.current !== null) window.clearTimeout(closeTimerRef.current)
    closeTimerRef.current = window.setTimeout(() => {
      closeTimerRef.current = null
      completeWorldClose()
    }, WORLD_RAIL_CLOSE_MS)
  }, [completeWorldClose, railOpen, worldClosePhase])

  const handleStardewAction = useCallback(() => {
    if (railOpen) {
      closeWorlds()
      return
    }
    if (installCard.actionDisabled || destination.kind === 'unavailable') return
    if (destination.kind === 'install') {
      onNavigate(stardewInstallPath())
      return
    }
    onNavigate('/games/stardew')
  }, [closeWorlds, destination.kind, installCard.actionDisabled, onNavigate, railOpen])

  useEffect(() => () => {
    if (closeTimerRef.current !== null) window.clearTimeout(closeTimerRef.current)
  }, [])

  useEffect(() => {
    let clockTimer: number | null = null

    const synchronizeBackgroundWithClock = () => {
      if (clockTimer !== null) window.clearTimeout(clockTimer)
      const now = new Date()
      const nowValue = now.getTime()
      setBackgroundSelection((current) => current.manualUntil !== null && current.manualUntil > nowValue
        ? current
        : { background: gameLibraryBackgroundForDate(now), manualUntil: null })
      try {
        const storedManual = gameLibraryManualBackgroundPreference(
          window.localStorage.getItem(GAME_LIBRARY_BACKGROUND_STORAGE_KEY),
          window.localStorage.getItem(GAME_LIBRARY_BACKGROUND_MANUAL_UNTIL_STORAGE_KEY),
          nowValue,
        )
        if (!storedManual) {
          window.localStorage.removeItem(GAME_LIBRARY_BACKGROUND_STORAGE_KEY)
          window.localStorage.removeItem(GAME_LIBRARY_BACKGROUND_MANUAL_UNTIL_STORAGE_KEY)
        }
      } catch {
        // Clock-following remains available when browser storage is unavailable.
      }
      const delay = Math.max(1_000, gameLibraryNextBackgroundBoundary(now) - nowValue + 100)
      clockTimer = window.setTimeout(synchronizeBackgroundWithClock, delay)
    }

    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') synchronizeBackgroundWithClock()
    }

    synchronizeBackgroundWithClock()
    document.addEventListener('visibilitychange', handleVisibilityChange)
    window.addEventListener('focus', synchronizeBackgroundWithClock)
    return () => {
      if (clockTimer !== null) window.clearTimeout(clockTimer)
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      window.removeEventListener('focus', synchronizeBackgroundWithClock)
    }
  }, [])

  useEffect(() => {
    if (railOpen) return
    if (closeTimerRef.current !== null) {
      window.clearTimeout(closeTimerRef.current)
      closeTimerRef.current = null
    }
    setWorldClosePhase('idle')
  }, [railOpen])

  useEffect(() => {
    if (railOpen) return
    const selectedCard = cardRefs.current[selectedIndex]
    if (!selectedCard) return
    const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    selectedCard.scrollIntoView({
      behavior: reducedMotion ? 'auto' : 'smooth',
      block: 'nearest',
      inline: 'nearest',
    })
  }, [railOpen, selectedIndex])

  useEffect(() => {
    if (!railOpen) return
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      if (createWorldOpen) {
        onNavigate('/games/stardew')
        return
      }
      closeWorlds()
    }
    window.addEventListener('keydown', handleEscape)
    return () => window.removeEventListener('keydown', handleEscape)
  }, [closeWorlds, createWorldOpen, onNavigate, railOpen])

  useEffect(() => {
    if (createWorldOpen && !canCreateWorld(user.role)) onNavigate('/games/stardew', true)
  }, [createWorldOpen, onNavigate, user.role])

  function handleCardKeyDown(event: ReactKeyboardEvent<HTMLButtonElement>, currentIndex: number) {
    if ((event.key === 'Enter' || event.key === ' ') && currentIndex === 0) {
      event.preventDefault()
      handleStardewAction()
      return
    }
    const intent = event.key === 'ArrowLeft'
      ? 'previous'
      : event.key === 'ArrowRight'
        ? 'next'
        : event.key === 'Home'
          ? 'first'
          : event.key === 'End'
            ? 'last'
            : null
    if (!intent) return
    event.preventDefault()
    const nextIndex = gameCardNavigationIndex(currentIndex, GAME_CARD_ENABLED, intent)
    setSelectedIndex(nextIndex)
    cardRefs.current[nextIndex]?.focus({ preventScroll: true })
  }

  function handleTouchStart(event: ReactPointerEvent<HTMLButtonElement>) {
    if (event.pointerType !== 'touch') return
    touchGestureRef.current = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      moved: false,
    }
  }

  function handleTouchMove(event: ReactPointerEvent<HTMLButtonElement>) {
    const gesture = touchGestureRef.current
    if (!gesture || gesture.pointerId !== event.pointerId) return
    if (Math.hypot(event.clientX - gesture.startX, event.clientY - gesture.startY) > 10) {
      gesture.moved = true
    }
  }

  function handleTouchEnd(event: ReactPointerEvent<HTMLButtonElement>) {
    const gesture = touchGestureRef.current
    if (!gesture || gesture.pointerId !== event.pointerId) return
    if (gesture.moved) suppressClickUntilRef.current = performance.now() + 450
    touchGestureRef.current = null
  }

  function activateStardew() {
    if (performance.now() < suppressClickUntilRef.current) return
    handleStardewAction()
  }

  function chooseBackground(nextBackground: GameLibraryBackground) {
    const manualUntil = gameLibraryNextBackgroundBoundary(new Date())
    setBackgroundSelection({ background: nextBackground, manualUntil })
    try {
      window.localStorage.setItem(GAME_LIBRARY_BACKGROUND_STORAGE_KEY, nextBackground)
      window.localStorage.setItem(GAME_LIBRARY_BACKGROUND_MANUAL_UNTIL_STORAGE_KEY, String(manualUntil))
    } catch {
      // Storage can be unavailable in hardened/private browser contexts; the in-memory choice still lasts to the next boundary.
    }
  }

  function toggleBackground() {
    chooseBackground(background === 'day' ? 'night' : 'day')
  }

  const installProgressLabel = installOpen && installPresentation
    ? `${installPresentation.mode === 'complete' ? '安装完成' : installPresentation.mode === 'failed' ? '安装未完成' : installPresentation.mode === 'idle' ? '等待安装' : '安装中'} · 总进度${installPresentation.mode === 'complete' ? ' ' : '约 '}${installPresentation.overallPercent}%`
    : installCard.label

  return (
    <GameHubShell
      user={user}
      background={background}
      onToggleBackground={toggleBackground}
      onNavigate={onNavigate}
      onLogout={onLogout}
    >
      <ul className={`game-picker${railOpen ? ' is-worlds-open' : ''}${installOpen ? ' is-install-open' : ''}${worldsClosing ? ' is-worlds-closing' : ''}`} aria-label="可管理的游戏">
        <li className="game-picker-item game-picker-item--primary">
          <div className={`game-card-frame${installOpen ? ' has-install-panel' : ''}`}>
            {installOpen && installPresentation ? (
              <span
                className={`game-card-progress-ring is-${installPresentation.mode}`}
                style={{ '--game-install-progress': `${installPresentation.overallPercent * 3.6}deg` } as CSSProperties}
                role="progressbar"
                aria-label="总体安装进度"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={installPresentation.overallPercent}
                aria-valuetext={`${installPresentation.mode === 'complete' ? '' : '约 '}${installPresentation.overallPercent}%`}
              />
            ) : null}
            <button
              ref={(element) => { cardRefs.current[0] = element }}
              type="button"
              className={`game-carousel-card game-carousel-card--stardew game-carousel-card--${installCard.kind}${selectedIndex === 0 ? ' is-selected' : ''}`}
              tabIndex={selectedIndex === 0 ? 0 : -1}
              aria-label={worldsClosing
                ? `星露谷物语，${installProgressLabel}，正在收起`
                : installOpen
                  ? `星露谷物语，${installProgressLabel}，点击收起安装`
                  : worldsOpen
                    ? `星露谷物语，${installCard.label}，点击收起世界`
                    : stardewGameCardAriaLabel(installCard)}
              aria-disabled={installCard.actionDisabled}
              aria-expanded={railOpen && !worldsClosing}
              aria-busy={worldsClosing}
              aria-controls="stardew-detail-rail"
              onFocus={() => setSelectedIndex(0)}
              onKeyDown={(event) => handleCardKeyDown(event, 0)}
              onClick={activateStardew}
              onPointerDown={handleTouchStart}
              onPointerMove={handleTouchMove}
              onPointerUp={handleTouchEnd}
              onPointerCancel={handleTouchEnd}
            >
              <span className="game-carousel-cover" aria-hidden="true">
                <span className={`game-card-signal game-card-signal--${installCard.tone}`} />
              </span>
              <span className="game-carousel-copy">
                <span className="game-carousel-title">星露谷物语</span>
                <span className={`game-carousel-status game-carousel-status--${installCard.tone}`}>
                  {installProgressLabel}
                </span>
                <span className={`game-carousel-action${railOpen && !worldsClosing ? ' is-open' : ''}`}>
                  {worldsClosing ? '正在收起…' : installOpen ? '收起安装' : worldsOpen ? '收起世界' : installCard.actionLabel}
                  <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 5 7 7-7 7" /></svg>
                </span>
              </span>
            </button>
          </div>
        </li>

        <WorldRail
          catalog={catalog}
          user={user}
          open={railOpen}
          installOpen={installOpen}
          installInstanceId={installInstanceId}
          requestedInstallJobId={requestedInstallJobId}
          createWorldOpen={createWorldOpen}
          closePhase={worldClosePhase}
          onNavigate={onNavigate}
          onInstallPresentationChange={setInstallPresentation}
        />

        <li className="game-picker-item game-picker-item--soon">
          <button
            ref={(element) => { cardRefs.current[1] = element }}
            type="button"
            className="game-carousel-card game-carousel-card--soon"
            tabIndex={-1}
            aria-label="其他游戏，暂未接入"
            disabled
          >
            <span className="game-carousel-cover" aria-hidden="true"><span>•••</span></span>
            <span className="game-carousel-copy">
              <span className="game-carousel-title">其他游戏</span>
              <span className="game-carousel-status">暂未接入</span>
              <span className="game-carousel-action">等待后续接入</span>
            </span>
          </button>
        </li>
      </ul>

      {catalog.error && !railOpen ? <CatalogNotice message={`无法读取实例：${catalog.error}`} onRetry={catalog.refresh} /> : null}
    </GameHubShell>
  )
}
