import { useState } from 'react'
import {
  sendSay,
  getRestartSchedule,
  updateRestartSchedule,
  getInstanceServerPassword,
  updateInstanceServerPassword,
  getInstancePasswordStatus,
  getInstanceServerRuntimeSettings,
  updateInstanceServerRuntimeSettings,
  triggerFestivalEvent,
  enableJojaRoute,
  requestGameSave,
} from '../../../api'
import type { InstancePasswordStatus, RestartSchedule, ServerRuntimeSettings } from '../../../types'
import { errorMessage, stateLabel, formatDate } from '../../../core/helpers'
import type { StardewPageProps } from '../stardew-routes'
import './MobileControlPage.css'
import { submitAndWaitForPlayerCommand } from '../player-command-results'

type MobileControlPageProps = Pick<StardewPageProps, 'user' | 'instanceState' | 'dashboardData'>

const ICONS = {
  broadcast: '/assets/stardew/ui/icons/icon_nav_diagnostics_monitor_image2.png',
  quick: '/assets/stardew/ui/icons/icon_nav_settings_gear_image2.png',
  schedule: '/assets/stardew/ui/icons/icon_right_rail_in_progress_clock_image2.png',
  settings: '/assets/stardew/ui/icons/icon_nav_settings_gear_image2.png',
  festival: '/assets/stardew/ui/icons/icon_nav_tasks_scroll_image2.png',
  joja: '/assets/stardew/ui/icons/icon_players_action_permission_image2.png',
} as const

const JOJA_CONFIRM_TEXT = 'IRREVERSIBLY_ENABLE_JOJA_RUN'

const defaultRestartSchedule: RestartSchedule = {
  instanceId: 'stardew',
  enabled: false,
  shutdownTime: '04:00',
  startupTime: '04:20',
  timezone: 'Asia/Shanghai',
  warningMinutes: [10, 5, 1],
  backupBeforeShutdown: true,
  skipIfPlayersOnline: false,
}

const defaultRuntimeSettings: ServerRuntimeSettings = {
  cabinStrategy: 'CabinStack',
  existingCabinBehavior: 'KeepExisting',
  networkBroadcastPeriod: 1,
}

function serverStatusText(state: string | null, loading: boolean): string {
  if (!state) return loading ? '读取中' : '未知'
  if (state === 'stopping') return '停止中'
  return stateLabel(state)
}

function serverStatusDotClass(state: string | null, loading: boolean): string {
  if (state === 'running') return 'sd-dot sd-dot-green sd-dot-pulse'
  if (state === 'starting' || state === 'stopping' || (loading && !state)) return 'sd-dot sd-dot-yellow sd-dot-pulse'
  if (state === 'stopped' || state === 'error') return 'sd-dot sd-dot-red'
  return 'sd-dot sd-dot-gray'
}

export function MobileControlPage({ user, instanceState, dashboardData }: MobileControlPageProps) {
  const isAdmin = user.role === 'admin'
  const state = instanceState?.state ?? null
  const isRunning = state === 'running'
  const activeSaveName = dashboardData.saves?.activeSaveName?.trim() || '暂无存档'

  // ── 全服喊话 ────────────────────────────────────────────────────────────
  const [sayMessage, setSayMessage] = useState('')
  const [sayBusy, setSayBusy] = useState(false)
  const [sayResult, setSayResult] = useState<string | null>(null)
  const [sayError, setSayError] = useState<string | null>(null)
  const [sayConfirmed, setSayConfirmed] = useState(false)

  // ── 计划重启 ────────────────────────────────────────────────────────────
  const [scheduleOpen, setScheduleOpen] = useState(false)
  const [scheduleDraft, setScheduleDraft] = useState<RestartSchedule>(defaultRestartSchedule)
  const [scheduleLoading, setScheduleLoading] = useState(false)
  const [scheduleSaving, setScheduleSaving] = useState(false)
  const [scheduleError, setScheduleError] = useState<string | null>(null)
  const [scheduleSaved, setScheduleSaved] = useState<string | null>(null)

  // ── 服务器密码设置 ──────────────────────────────────────────────────────
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [passwordDraft, setPasswordDraft] = useState('')
  const [passwordVisible, setPasswordVisible] = useState(false)
  const [passwordLoading, setPasswordLoading] = useState(false)
  const [passwordSaving, setPasswordSaving] = useState(false)
  const [passwordError, setPasswordError] = useState<string | null>(null)
  const [passwordMessage, setPasswordMessage] = useState<string | null>(null)
  const [passwordStatus, setPasswordStatus] = useState<InstancePasswordStatus | null>(null)
  const [passwordStatusLoading, setPasswordStatusLoading] = useState(false)
  const [passwordStatusError, setPasswordStatusError] = useState<string | null>(null)

  // ── 小屋与联机高级设置 ──────────────────────────────────────────────────
  const [runtimeSettingsOpen, setRuntimeSettingsOpen] = useState(false)
  const [runtimeSettingsDraft, setRuntimeSettingsDraft] = useState<ServerRuntimeSettings>(defaultRuntimeSettings)
  const [runtimeSettingsLoading, setRuntimeSettingsLoading] = useState(false)
  const [runtimeSettingsSaving, setRuntimeSettingsSaving] = useState(false)
  const [runtimeSettingsError, setRuntimeSettingsError] = useState<string | null>(null)
  const [runtimeSettingsMessage, setRuntimeSettingsMessage] = useState<string | null>(null)

  // ── 触发节日活动 ────────────────────────────────────────────────────────
  const [festivalBusy, setFestivalBusy] = useState(false)
  const [festivalMessage, setFestivalMessage] = useState<string | null>(null)
  const [festivalError, setFestivalError] = useState(false)
  const [saveNowBusy, setSaveNowBusy] = useState(false)
  const [saveNowMessage, setSaveNowMessage] = useState<string | null>(null)
  const [saveNowError, setSaveNowError] = useState(false)

  // ── 永久启用 Joja 路线 ──────────────────────────────────────────────────
  const [jojaOpen, setJojaOpen] = useState(false)
  const [jojaConfirmInput, setJojaConfirmInput] = useState('')
  const [jojaBusy, setJojaBusy] = useState(false)
  const [jojaMessage, setJojaMessage] = useState<string | null>(null)
  const [jojaError, setJojaError] = useState(false)

  const statusText = serverStatusText(state, dashboardData.loading)
  const statusDotClass = serverStatusDotClass(state, dashboardData.loading)

  async function handleSay() {
    if (!sayMessage.trim()) return
    setSayBusy(true)
    setSayResult(null)
    setSayError(null)
    try {
      const feedback = await submitAndWaitForPlayerCommand(
        () => sendSay(sayMessage.trim()),
        'broadcast',
        '',
        (next) => {
          if (next.kind === 'failed') {
            setSayResult(null)
            setSayError(next.message)
            setSayConfirmed(false)
          } else {
            setSayError(null)
            setSayResult(next.message)
            setSayConfirmed(next.kind === 'succeeded')
          }
        },
      )
      if (feedback.kind === 'succeeded' || feedback.kind === 'legacy') setSayMessage('')
    } catch (e) {
      setSayError(errorMessage(e))
    } finally {
      setSayBusy(false)
    }
  }

  async function openRestartSchedule() {
    if (!isAdmin) return
    setScheduleOpen(true)
    setScheduleLoading(true)
    setScheduleSaving(false)
    setScheduleError(null)
    setScheduleSaved(null)
    try {
      const result = await getRestartSchedule()
      setScheduleDraft(result.schedule)
    } catch (e) {
      setScheduleError(errorMessage(e))
      setScheduleDraft(defaultRestartSchedule)
    } finally {
      setScheduleLoading(false)
    }
  }

  async function handleSaveRestartSchedule() {
    setScheduleSaving(true)
    setScheduleError(null)
    setScheduleSaved(null)
    try {
      const result = await updateRestartSchedule(scheduleDraft)
      setScheduleDraft(result.schedule)
      setScheduleSaved('计划重启已保存。')
      dashboardData.refreshJobs()
    } catch (e) {
      setScheduleError(errorMessage(e))
    } finally {
      setScheduleSaving(false)
    }
  }

  function toggleScheduleWarning(minute: number) {
    setScheduleDraft((draft) => {
      const exists = draft.warningMinutes.includes(minute)
      const next = exists
        ? draft.warningMinutes.filter((value) => value !== minute)
        : [...draft.warningMinutes, minute]
      next.sort((a, b) => b - a)
      return { ...draft, warningMinutes: next }
    })
  }

  async function loadPasswordStatus() {
    setPasswordStatusLoading(true)
    setPasswordStatusError(null)
    try {
      const res = await getInstancePasswordStatus()
      setPasswordStatus(res)
    } catch (e) {
      setPasswordStatus(null)
      setPasswordStatusError(errorMessage(e))
    } finally {
      setPasswordStatusLoading(false)
    }
  }

  async function openPasswordSettings() {
    if (!isAdmin) return
    setPasswordOpen(true)
    setPasswordVisible(false)
    setPasswordLoading(true)
    setPasswordSaving(false)
    setPasswordError(null)
    setPasswordMessage(null)
    try {
      const res = await getInstanceServerPassword()
      setPasswordDraft(res.serverPassword)
    } catch (e) {
      setPasswordError(errorMessage(e))
      setPasswordDraft('')
    } finally {
      setPasswordLoading(false)
    }
    void loadPasswordStatus()
  }

  async function handleSaveServerPassword() {
    if (passwordDraft.length > 128) {
      setPasswordError('服务器密码不能超过 128 个字符')
      setPasswordMessage(null)
      return
    }
    setPasswordSaving(true)
    setPasswordError(null)
    setPasswordMessage(null)
    try {
      const res = await updateInstanceServerPassword(passwordDraft)
      setPasswordDraft(res.serverPassword)
      setPasswordMessage('密码已保存，需要重启服务器容器后才会生效。')
    } catch (e) {
      setPasswordError(errorMessage(e))
    } finally {
      setPasswordSaving(false)
    }
  }

  async function openRuntimeSettings() {
    if (!isAdmin) return
    setRuntimeSettingsOpen(true)
    setRuntimeSettingsLoading(true)
    setRuntimeSettingsSaving(false)
    setRuntimeSettingsError(null)
    setRuntimeSettingsMessage(null)
    try {
      const res = await getInstanceServerRuntimeSettings()
      setRuntimeSettingsDraft(res)
    } catch (e) {
      setRuntimeSettingsError(errorMessage(e))
      setRuntimeSettingsDraft(defaultRuntimeSettings)
    } finally {
      setRuntimeSettingsLoading(false)
    }
  }

  async function handleSaveRuntimeSettings() {
    setRuntimeSettingsSaving(true)
    setRuntimeSettingsError(null)
    setRuntimeSettingsMessage(null)
    try {
      const res = await updateInstanceServerRuntimeSettings(runtimeSettingsDraft)
      setRuntimeSettingsDraft(res)
      setRuntimeSettingsMessage('设置已保存，需要重启服务器容器后才会生效。')
    } catch (e) {
      setRuntimeSettingsError(errorMessage(e))
    } finally {
      setRuntimeSettingsSaving(false)
    }
  }

  async function handleTriggerFestivalEvent() {
    if (!isAdmin || !isRunning || festivalBusy) return
    setFestivalBusy(true)
    setFestivalMessage(null)
    setFestivalError(false)
    try {
      await submitAndWaitForPlayerCommand(
        () => triggerFestivalEvent(),
        'trigger-event',
        '',
        (feedback) => {
          setFestivalError(feedback.kind === 'failed')
          setFestivalMessage(feedback.message)
        },
      )
    } catch (e) {
      setFestivalError(true)
      setFestivalMessage(errorMessage(e))
    } finally {
      setFestivalBusy(false)
    }
  }

  async function handleSaveNow() {
    if (!isAdmin || !isRunning || saveNowBusy) return
    setSaveNowBusy(true)
    setSaveNowMessage(null)
    setSaveNowError(false)
    try {
      await submitAndWaitForPlayerCommand(
        () => requestGameSave(),
        'save-now',
        '',
        (feedback) => {
          setSaveNowError(feedback.kind === 'failed')
          setSaveNowMessage(feedback.message)
        },
        125_000,
      )
    } catch (e) {
      setSaveNowError(true)
      setSaveNowMessage(errorMessage(e))
    } finally {
      setSaveNowBusy(false)
    }
  }

  function openJojaConfirm() {
    if (!isAdmin || !isRunning) return
    setJojaConfirmInput('')
    setJojaMessage(null)
    setJojaError(false)
    setJojaOpen(true)
  }

  async function handleEnableJoja() {
    if (jojaConfirmInput !== JOJA_CONFIRM_TEXT) return
    setJojaBusy(true)
    setJojaMessage(null)
    setJojaError(false)
    try {
      await submitAndWaitForPlayerCommand(
        () => enableJojaRoute(jojaConfirmInput),
        'enable-joja',
        '',
        (feedback) => {
          setJojaError(feedback.kind === 'failed')
          setJojaMessage(feedback.message)
        },
      )
    } catch (e) {
      setJojaError(true)
      setJojaMessage(errorMessage(e))
    } finally {
      setJojaBusy(false)
    }
  }

  return (
    <div className="sd-mctrl-wrap">
      <div className="sd-mctrl-status-strip sd-panel">
        <span className="sd-mctrl-status-main">
          <span className={statusDotClass} aria-hidden="true" />
          {statusText}
        </span>
        <span className="sd-mctrl-status-save">存档：{activeSaveName}</span>
      </div>

      {/* ── 全服消息 ─────────────────────────────────────────────────────── */}
      <section className="sd-panel sd-mctrl-card">
        <div className="sd-mctrl-card-title">
          <img src={ICONS.broadcast} alt="" />
          全服消息
        </div>
        {isRunning ? (
          <>
            <div className="sd-mctrl-say-row">
              <input
                className="sd-input sd-mctrl-say-input"
                type="text"
                placeholder="向所有在线玩家发送消息…"
                value={sayMessage}
                onChange={(e) => setSayMessage(e.target.value)}
                disabled={sayBusy}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') void handleSay()
                }}
              />
              <button
                type="button"
                className="sd-btn-restart sd-mctrl-say-btn"
                onClick={() => void handleSay()}
                disabled={sayBusy || !sayMessage.trim()}
              >
                {sayBusy ? '发送中…' : '发送'}
              </button>
            </div>
            <div className="sd-mctrl-say-count">{sayMessage.length} 字</div>
            {sayResult ? <div className={`sd-notice ${sayConfirmed ? 'sd-notice--ok' : ''} sd-mctrl-notice`}>{sayResult}</div> : null}
            {sayError ? <div className="sd-notice sd-notice--error sd-mctrl-notice">{sayError}</div> : null}
          </>
        ) : (
          <div className="sd-notice sd-notice--info sd-mctrl-notice">服务器运行时可向在线玩家发送全服消息。</div>
        )}
      </section>

      {/* ── 快捷操作 ─────────────────────────────────────────────────────── */}
      <section className="sd-panel sd-mctrl-card">
        <div className="sd-mctrl-card-title">
          <img src={ICONS.quick} alt="" />
          快捷操作
        </div>
        <div className="sd-mctrl-action-list">
          <button
            type="button"
            className="sd-btn-tan sd-mctrl-action-btn sd-mctrl-action-btn--card"
            disabled={!isAdmin}
            title={isAdmin ? '设置每天几点关闭、几点开启服务器' : '仅管理员可设置计划重启'}
            onClick={() => void openRestartSchedule()}
          >
            <img className="sd-mctrl-action-icon" src={ICONS.schedule} alt="" />
            <span className="sd-mctrl-action-copy">
              <strong>计划重启</strong>
              <span>设置定时重启</span>
            </span>
          </button>

          <button
            type="button"
            className="sd-btn-tan sd-mctrl-action-btn sd-mctrl-action-btn--card"
            disabled={!isAdmin}
            title={isAdmin ? '设置玩家加入服务器所需的密码' : '仅管理员可设置服务器密码'}
            onClick={() => void openPasswordSettings()}
          >
            <img className="sd-mctrl-action-icon" src={ICONS.settings} alt="" />
            <span className="sd-mctrl-action-copy">
              <strong>服务器密码设置</strong>
              <span>配置玩家加入密码</span>
            </span>
          </button>

          <button
            type="button"
            className="sd-btn-tan sd-mctrl-action-btn sd-mctrl-action-btn--card"
            disabled={!isAdmin}
            title={isAdmin ? '配置小屋策略与联机广播频率' : '仅管理员可配置小屋与联机高级设置'}
            onClick={() => void openRuntimeSettings()}
          >
            <img className="sd-mctrl-action-icon" src={ICONS.settings} alt="" />
            <span className="sd-mctrl-action-copy">
              <strong>小屋与联机高级设置</strong>
              <span>小屋策略 / 广播频率</span>
            </span>
          </button>

          <button
            type="button"
            className="sd-btn-tan sd-mctrl-action-btn sd-mctrl-action-btn--card"
            disabled={!isAdmin || !isRunning || saveNowBusy}
            title={
              !isAdmin
                ? '仅管理员可执行此操作'
                : !isRunning
                  ? '服务器运行后才能请求游戏内保存'
                  : '设置游戏内保存请求，并等待 GameLoop.Saved 确认；这不是创建 ZIP 备份'
            }
            onClick={() => void handleSaveNow()}
          >
            <img className="sd-mctrl-action-icon" src={ICONS.quick} alt="" />
            <span className="sd-mctrl-action-copy">
              <strong>{saveNowBusy ? '等待保存…' : '请求游戏内保存'}</strong>
              <span>以 Saved 事件确认完成</span>
            </span>
          </button>

          <button
            type="button"
            className="sd-btn-tan sd-mctrl-action-btn sd-mctrl-action-btn--card"
            disabled={!isAdmin || !isRunning || festivalBusy}
            title={
              !isAdmin
                ? '仅管理员可执行此操作'
                : !isRunning
                  ? '服务器运行后才能触发节日活动'
                  : '模拟游戏内 !event 指令，强制开始当天节日的主活动（若当天没有节日则不会生效）'
            }
            onClick={() => void handleTriggerFestivalEvent()}
          >
            <img className="sd-mctrl-action-icon" src={ICONS.festival} alt="" />
            <span className="sd-mctrl-action-copy">
              <strong>{festivalBusy ? '触发中…' : '触发节日活动'}</strong>
              <span>卡住时强制开始</span>
            </span>
          </button>

          <button
            type="button"
            className="sd-btn-delete sd-mctrl-action-btn"
            disabled={!isAdmin || !isRunning}
            title={
              !isAdmin
                ? '仅管理员可执行此操作'
                : !isRunning
                  ? '服务器运行后才能启用 Joja 路线'
                  : '永久启用 Joja 路线并禁用标准社区中心，此操作不可撤销'
            }
            onClick={openJojaConfirm}
          >
            <img className="sd-mctrl-action-icon" src={ICONS.joja} alt="" />
            <span className="sd-mctrl-action-copy">
              <strong>永久启用 Joja 路线</strong>
              <span>不可撤销，请谨慎操作</span>
            </span>
          </button>
        </div>
        {festivalMessage ? (
          <div className={`sd-notice sd-mctrl-notice ${festivalError ? 'sd-notice--error' : 'sd-notice--ok'}`}>
            {festivalMessage}
          </div>
        ) : null}
        {saveNowMessage ? (
          <div className={`sd-notice sd-mctrl-notice ${saveNowError ? 'sd-notice--error' : 'sd-notice--ok'}`}>
            {saveNowMessage}
          </div>
        ) : null}
      </section>

      {/* ── 计划重启弹窗 ─────────────────────────────────────────────────── */}
      {scheduleOpen ? (
        <div className="sd-mctrl-dialog-overlay" role="dialog" aria-modal="true">
          <div className="sd-panel sd-mctrl-dialog">
            <h3>计划重启</h3>
            {scheduleLoading ? (
              <p>正在读取计划重启配置...</p>
            ) : (
              <>
                <label className="sd-mctrl-check">
                  <input
                    type="checkbox"
                    checked={scheduleDraft.enabled}
                    onChange={(e) => setScheduleDraft({ ...scheduleDraft, enabled: e.target.checked })}
                  />
                  启用每日计划维护
                </label>

                <label className="sd-mctrl-field">
                  <span>关闭时间</span>
                  <input
                    className="sd-input"
                    type="time"
                    value={scheduleDraft.shutdownTime}
                    onChange={(e) => setScheduleDraft({ ...scheduleDraft, shutdownTime: e.target.value })}
                  />
                </label>

                <label className="sd-mctrl-field">
                  <span>开启时间</span>
                  <input
                    className="sd-input"
                    type="time"
                    value={scheduleDraft.startupTime}
                    onChange={(e) => setScheduleDraft({ ...scheduleDraft, startupTime: e.target.value })}
                  />
                </label>

                <label className="sd-mctrl-field">
                  <span>时区</span>
                  <input
                    className="sd-input"
                    value={scheduleDraft.timezone}
                    onChange={(e) => setScheduleDraft({ ...scheduleDraft, timezone: e.target.value })}
                  />
                </label>

                <div className="sd-mctrl-field">
                  <span>关服前提醒</span>
                  <div className="sd-mctrl-check-group">
                    {[10, 5, 1].map((minute) => (
                      <label key={minute} className="sd-mctrl-check">
                        <input
                          type="checkbox"
                          checked={scheduleDraft.warningMinutes.includes(minute)}
                          onChange={() => toggleScheduleWarning(minute)}
                        />
                        {minute} 分钟
                      </label>
                    ))}
                  </div>
                </div>

                <label className="sd-mctrl-check">
                  <input
                    type="checkbox"
                    checked={scheduleDraft.backupBeforeShutdown}
                    onChange={(e) => setScheduleDraft({ ...scheduleDraft, backupBeforeShutdown: e.target.checked })}
                  />
                  关闭前备份当前已保存进度
                </label>

                <label className="sd-mctrl-check">
                  <input
                    type="checkbox"
                    checked={scheduleDraft.skipIfPlayersOnline}
                    onChange={(e) => setScheduleDraft({ ...scheduleDraft, skipIfPlayersOnline: e.target.checked })}
                  />
                  如果仍有玩家在线则跳过本次关闭
                </label>

                <div className="sd-mctrl-warning">
                  关闭时间到达后，面板会先按配置发送提醒、备份当前已经落盘的存档，再提交停止任务；开启时间到达后会按当前激活存档提交启动任务。
                </div>

                <div className="sd-mctrl-summary">
                  <div>下次关闭：{scheduleDraft.nextShutdownAt ? formatDate(scheduleDraft.nextShutdownAt) : '未启用'}</div>
                  <div>下次开启：{scheduleDraft.nextStartupAt ? formatDate(scheduleDraft.nextStartupAt) : '未启用'}</div>
                  <div>上次状态：{scheduleDraft.lastStatus ?? '暂无记录'}</div>
                  {scheduleDraft.lastMessage ? <div>说明：{scheduleDraft.lastMessage}</div> : null}
                </div>
              </>
            )}

            {scheduleError ? <div className="sd-notice sd-notice--error sd-mctrl-notice">{scheduleError}</div> : null}
            {scheduleSaved ? <div className="sd-notice sd-notice--ok sd-mctrl-notice">{scheduleSaved}</div> : null}

            <div className="sd-mctrl-dialog-actions">
              <button
                type="button"
                className="sd-btn-tan sd-mctrl-dialog-btn"
                onClick={() => setScheduleOpen(false)}
                disabled={scheduleSaving}
              >
                取消
              </button>
              <button
                type="button"
                className="sd-btn-green sd-mctrl-dialog-btn"
                onClick={() => void handleSaveRestartSchedule()}
                disabled={scheduleLoading || scheduleSaving}
              >
                {scheduleSaving ? '保存中…' : '保存'}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {/* ── 服务器密码设置弹窗 ───────────────────────────────────────────── */}
      {passwordOpen ? (
        <div className="sd-mctrl-dialog-overlay" role="dialog" aria-modal="true">
          <div className="sd-panel sd-mctrl-dialog">
            <h3>服务器密码设置</h3>

            {passwordLoading ? (
              <p>正在读取当前密码配置...</p>
            ) : (
              <>
                <label className="sd-mctrl-field">
                  <span>加入密码</span>
                  <div className="sd-mctrl-inline-row">
                    <input
                      className="sd-input"
                      type={passwordVisible ? 'text' : 'password'}
                      value={passwordDraft}
                      placeholder="留空表示不设置密码"
                      maxLength={128}
                      onChange={(e) => {
                        setPasswordDraft(e.target.value)
                        setPasswordMessage(null)
                      }}
                      disabled={passwordSaving}
                    />
                    <button
                      type="button"
                      className="sd-btn-tan sd-mctrl-inline-btn"
                      onClick={() => setPasswordVisible((v) => !v)}
                    >
                      {passwordVisible ? '隐藏' : '显示'}
                    </button>
                  </div>
                </label>

                <div className="sd-mctrl-warning">
                  该密码仅在服务器容器启动时生效（JunimoServer 不支持运行时热改）。保存后需要重启服务器容器才会真正生效；玩家加入时需要在游戏内输入 !login 密码。
                </div>

                {passwordError ? <div className="sd-notice sd-notice--error sd-mctrl-notice">{passwordError}</div> : null}
                {passwordMessage ? <div className="sd-notice sd-notice--ok sd-mctrl-notice">{passwordMessage}</div> : null}

                <div className="sd-mctrl-dialog-actions">
                  <button
                    type="button"
                    className="sd-btn-tan sd-mctrl-dialog-btn"
                    onClick={() => setPasswordOpen(false)}
                    disabled={passwordSaving}
                  >
                    关闭
                  </button>
                  <button
                    type="button"
                    className="sd-btn-green sd-mctrl-dialog-btn"
                    onClick={() => void handleSaveServerPassword()}
                    disabled={passwordSaving}
                  >
                    {passwordSaving ? '保存中…' : '保存'}
                  </button>
                </div>

                <div className="sd-mctrl-summary sd-mctrl-summary-block">
                  <div className="sd-mctrl-summary-head">
                    <strong>密码保护状态（来自 JunimoServer）</strong>
                    <button
                      type="button"
                      className="sd-btn-tan sd-mctrl-inline-btn"
                      onClick={() => void loadPasswordStatus()}
                      disabled={passwordStatusLoading || !isRunning}
                    >
                      {passwordStatusLoading ? '读取中…' : '刷新'}
                    </button>
                  </div>
                  {!isRunning ? (
                    <div>服务器未运行，无法读取密码保护状态。</div>
                  ) : passwordStatusError ? (
                    <div className="sd-notice sd-notice--error sd-mctrl-notice">{passwordStatusError}</div>
                  ) : passwordStatus ? (
                    <>
                      <div>是否启用：{passwordStatus.enabled ? '已启用' : '未启用'}</div>
                      <div>已认证玩家：{passwordStatus.authenticatedCount}　待认证玩家：{passwordStatus.pendingCount}</div>
                      <div>认证超时：{passwordStatus.timeoutSeconds} 秒　最大失败次数：{passwordStatus.maxAttempts}</div>
                    </>
                  ) : (
                    <div>暂无数据。</div>
                  )}
                </div>
              </>
            )}
          </div>
        </div>
      ) : null}

      {/* ── 小屋与联机高级设置弹窗 ───────────────────────────────────────── */}
      {runtimeSettingsOpen ? (
        <div className="sd-mctrl-dialog-overlay" role="dialog" aria-modal="true">
          <div className="sd-panel sd-mctrl-dialog">
            <h3>小屋与联机高级设置</h3>

            {runtimeSettingsLoading ? (
              <p>正在读取当前配置...</p>
            ) : (
              <>
                <label className="sd-mctrl-field">
                  <span>小屋策略（CabinStrategy）</span>
                  <select
                    className="sd-input"
                    value={runtimeSettingsDraft.cabinStrategy}
                    disabled={runtimeSettingsSaving}
                    onChange={(e) => {
                      setRuntimeSettingsDraft((draft) => ({ ...draft, cabinStrategy: e.target.value }))
                      setRuntimeSettingsMessage(null)
                    }}
                  >
                    <option value="CabinStack">CabinStack（隐藏小屋堆叠，最适合大多数服务器）</option>
                    <option value="FarmhouseStack">FarmhouseStack（隐藏小屋，从主农舍共用入口出）</option>
                    <option value="None">None（原版行为，小屋放置在真实农场位置）</option>
                  </select>
                </label>

                <label className="sd-mctrl-field">
                  <span>已有小屋处理方式（ExistingCabinBehavior）</span>
                  <select
                    className="sd-input"
                    value={runtimeSettingsDraft.existingCabinBehavior}
                    disabled={runtimeSettingsSaving}
                    onChange={(e) => {
                      setRuntimeSettingsDraft((draft) => ({ ...draft, existingCabinBehavior: e.target.value }))
                      setRuntimeSettingsMessage(null)
                    }}
                  >
                    <option value="KeepExisting">KeepExisting（保留已有小屋位置）</option>
                    <option value="MoveToStack">MoveToStack（把已有小屋迁移到策略指定位置）</option>
                  </select>
                </label>

                <label className="sd-mctrl-field">
                  <span>网络广播频率（NetworkBroadcastPeriod，单位：刻）</span>
                  <select
                    className="sd-input"
                    value={runtimeSettingsDraft.networkBroadcastPeriod}
                    disabled={runtimeSettingsSaving}
                    onChange={(e) => {
                      setRuntimeSettingsDraft((draft) => ({ ...draft, networkBroadcastPeriod: Number(e.target.value) }))
                      setRuntimeSettingsMessage(null)
                    }}
                  >
                    <option value={1}>1（每刻广播，最实时）</option>
                    <option value={2}>2</option>
                    <option value={3}>3（原版频率）</option>
                  </select>
                </label>

                <div className="sd-mctrl-warning">
                  这些设置写入 server-settings.json，JunimoServer 只在容器启动时读取。保存后需要重启服务器容器才会生效，对已有存档同样适用。
                </div>

                {runtimeSettingsError ? <div className="sd-notice sd-notice--error sd-mctrl-notice">{runtimeSettingsError}</div> : null}
                {runtimeSettingsMessage ? <div className="sd-notice sd-notice--ok sd-mctrl-notice">{runtimeSettingsMessage}</div> : null}

                <div className="sd-mctrl-dialog-actions">
                  <button
                    type="button"
                    className="sd-btn-tan sd-mctrl-dialog-btn"
                    onClick={() => setRuntimeSettingsOpen(false)}
                    disabled={runtimeSettingsSaving}
                  >
                    关闭
                  </button>
                  <button
                    type="button"
                    className="sd-btn-green sd-mctrl-dialog-btn"
                    onClick={() => void handleSaveRuntimeSettings()}
                    disabled={runtimeSettingsSaving}
                  >
                    {runtimeSettingsSaving ? '保存中…' : '保存'}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      ) : null}

      {/* ── 永久启用 Joja 路线弹窗 ───────────────────────────────────────── */}
      {jojaOpen ? (
        <div className="sd-mctrl-dialog-overlay" role="dialog" aria-modal="true">
          <div className="sd-panel sd-mctrl-dialog">
            <h3>永久启用 Joja 路线</h3>

            <div className="sd-mctrl-warning">
              此操作会模拟游戏内 !joja IRREVERSIBLY_ENABLE_JOJA_RUN 指令，永久禁用标准社区中心路线，改为 Joja 路线。<strong>此操作不可撤销</strong>，对本存档的剩余游玩时间永久生效。请仅在你确实需要切换路线时使用。
            </div>

            <label className="sd-mctrl-field">
              <span>请输入 {JOJA_CONFIRM_TEXT} 以确认</span>
              <div className="sd-mctrl-inline-row">
                <input
                  className="sd-input"
                  type="text"
                  value={jojaConfirmInput}
                  placeholder={JOJA_CONFIRM_TEXT}
                  onChange={(e) => {
                    setJojaConfirmInput(e.target.value)
                    setJojaMessage(null)
                  }}
                  disabled={jojaBusy}
                />
                <button
                  type="button"
                  className="sd-btn-tan sd-mctrl-inline-btn"
                  onClick={() => {
                    setJojaConfirmInput(JOJA_CONFIRM_TEXT)
                    setJojaMessage(null)
                  }}
                  disabled={jojaBusy}
                >
                  填入
                </button>
              </div>
            </label>

            {jojaMessage ? (
              <div className={`sd-notice sd-mctrl-notice ${jojaError ? 'sd-notice--error' : 'sd-notice--ok'}`}>
                {jojaMessage}
              </div>
            ) : null}

            <div className="sd-mctrl-dialog-actions">
              <button type="button" className="sd-btn-tan sd-mctrl-dialog-btn" onClick={() => setJojaOpen(false)} disabled={jojaBusy}>
                取消
              </button>
              <button
                type="button"
                className="sd-btn-delete sd-mctrl-dialog-btn"
                onClick={() => void handleEnableJoja()}
                disabled={jojaBusy || jojaConfirmInput !== JOJA_CONFIRM_TEXT}
              >
                {jojaBusy ? '提交中…' : '确认永久启用'}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  )
}
