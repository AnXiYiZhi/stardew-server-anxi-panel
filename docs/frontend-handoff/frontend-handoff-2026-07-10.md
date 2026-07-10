# FESTIVAL-EVENT-1 触发节日活动 + JOJA-ROUTE-1 永久启用 Joja 路线

## 背景

后端已经实现了 `POST /api/instances/:id/festival/event`（模拟游戏内 `!event` 聊天指令，卡住时强制开始当天节日主活动）和 `POST /api/instances/:id/joja/enable`（模拟 `!joja IRREVERSIBLY_ENABLE_JOJA_RUN`，永久禁用标准社区中心路线，改为 Joja 路线，body 需要 `{"confirm": "..."}` 精确匹配文本）。详见 `docs/backend-handoff/backend-handoff-2026-07-10.md`。这次只做前端接线：把这两个能力做成 `ServerControlPage.tsx` 里的按钮。

## 改了什么

- `api.ts` 新增两个函数，复用已有的 `CommandRunResult` 类型（`runCommand`/`sendSay`/`kickPlayer` 都用这个类型），没有引入新类型：
  ```ts
  export function triggerFestivalEvent(instanceId = defaultInstanceId) {
    return request<CommandRunResult>(`/api/instances/${encodeURIComponent(instanceId)}/festival/event`, { method: 'POST' })
  }
  export function enableJojaRoute(confirm: string, instanceId = defaultInstanceId) {
    return request<CommandRunResult>(`/api/instances/${encodeURIComponent(instanceId)}/joja/enable`, { method: 'POST', body: { confirm } })
  }
  ```
  两者都没有像 `runCommand`/`sendSay` 那样加 40 秒 `AbortController` 超时——那个超时是因为要等 attach-cli 真实命令输出，这两个接口后端是直接写命令文件后立刻返回（fire-and-forget），响应很快，不需要。

- `ServerControlPage.tsx`：
  - 新增图标常量 `SERVER_PAGE_ICONS.festival`（复用 `icon_nav_tasks_scroll_image2.png`）、`SERVER_PAGE_ICONS.joja`（复用 `icon_players_action_permission_image2.png`），没有新增图片资源。
  - 新增常量 `JOJA_CONFIRM_TEXT = 'IRREVERSIBLY_ENABLE_JOJA_RUN'`，前后端各自独立校验同一段文本（前端控制按钮是否可点，后端再校验一次，双重把关，不是只信任前端）。
  - 新增状态：`festivalBusy`/`festivalMessage`/`festivalError`（触发节日活动的忙碌态和结果反馈，和 `quickBackupBusy`/`quickBackupMessage`/`quickBackupError` 同构）；`jojaOpen`/`jojaConfirmInput`/`jojaBusy`/`jojaMessage`/`jojaError`（Joja 强确认弹窗的状态，`jojaOpen` 控制弹窗显隐，`jojaConfirmInput` 是用户在弹窗里输入的确认文本）。
  - 新增 `handleTriggerFestivalEvent()`：直接调用，无二次确认（上游 `!event` 本身没有破坏性副作用，卡住时可以反复点，不需要每次都弹窗打断）。
  - 新增 `openJojaConfirm()` + `handleEnableJoja()`：前者只是打开弹窗并重置状态，后者在 `jojaConfirmInput !== JOJA_CONFIRM_TEXT` 时直接短路不发请求（配合"确认"按钮的 `disabled={jojaBusy || jojaConfirmInput !== JOJA_CONFIRM_TEXT}`，双重防止误触）。
  - "快捷操作"网格（`sd-server-quick-grid`）里紧跟"服务器密码设置"之后新增两个按钮：
    - "触发节日活动"：`sd-btn-tan sd-btn--lg`，`disabled={!isAdmin || !isRunning || festivalBusy}`。
    - "永久启用 Joja 路线"：`sd-btn-delete sd-btn--lg`（红色危险样式，和"确认停止服务器"用的类同一个），`disabled={!isAdmin || !isRunning}`，点击不直接调用 API，而是 `onClick={openJojaConfirm}` 打开弹窗。
  - 新增 Joja 强确认弹窗（紧跟在 `passwordOpen` 弹窗之后，同一个 `sd-confirm-overlay`/`sd-confirm-dialog` 结构）：一段 `sd-confirm-warning` 警告文案说明不可逆后果 + 一个文本输入框要求逐字输入 `IRREVERSIBLY_ENABLE_JOJA_RUN` + 取消/确认两个按钮，"确认"按钮在输入不匹配前保持禁用态。这个"输入确认文本才能点亮按钮"的模式在这个代码库里是第一次出现（之前的危险操作如"停止服务器"只是一个简单的"确认/取消"弹窗），因为上游 `!joja` 命令本身就要求逐字参数匹配，这里是刻意对齐这个交互而不是随手加的复杂度。

## 影响文件

- `frontend/src/api.ts`
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过。
- `cd frontend; npm run build`（`tsc -b && vite build`）：通过。

**未做的验证**：没有连一个真实运行中的 JunimoServer 实例实际点一遍这两个按钮，没有截图确认弹窗在移动端窄屏下的表现，也没有验证点击"触发节日活动"后游戏内聊天记录是否真的出现 `!event` 这条广播消息。建议下一位维护者找一个测试实例走一遍。

## 下一步注意事项

- 两个操作都是 fire-and-forget，`result.output` 只是"指令已提交"这类固定文案，不代表游戏内一定生效（比如当天没有节日时"触发节日活动"不会有任何效果，前端和后端都无法感知这一点，不用当成 bug 去修）。
- 如果用户反馈"点了按钮但游戏里没反应"，优先确认服务器容器是否已经用上了最新编译的 `StardewAnxiPanel.Control.dll`（改嵌入 Mod 源码后必须重启/重新准备 server 容器才会生效，参考 `PLAYERS-KICK-1` 时踩过的同一个坑），而不是先怀疑这次前端改动的逻辑。
- Joja 弹窗目前没有做"倒计时/二次弹窗"这类更重的防误触设计，只有"逐字输入确认文本"这一层，如果以后用户反馈还是有人误触，可以考虑加更多摩擦（比如要求先点开一个"我已了解后果"复选框），但这次没有做，属于按需求最小实现。

# CABIN-STRATEGY-1 小屋策略设置分层（同日追加）

## 背景

后端已实现小屋策略分层（详见 `docs/backend-handoff/backend-handoff-2026-07-10.md` 的 `CABIN-STRATEGY-1` 小节）：新建存档 `NewGameConfig` 新增 `cabinMode`（`recommended|vanilla` 简化二选一），服务器控制页新增 `GET/PUT /api/instances/:id/config/server-runtime-settings` 完整高级设置接口（`CabinStrategy`/`ExistingCabinBehavior`/`NetworkBroadcastPeriod`）。这次做前端接线，按用户给的设计口径落地成两个页面各自的入口。

## 改了什么

- `types.ts`：`NewGameConfig` 新增 `cabinMode?: string`；新增独立类型 `ServerRuntimeSettings{ cabinStrategy, existingCabinBehavior, networkBroadcastPeriod }`（没有复用 `NewGameConfig` 里的字段类型，因为这是两个不同层级的独立配置对象，命名故意保持和后端 JSON 字段一致方便对照）。
- `api.ts` 新增 `getInstanceServerRuntimeSettings(instanceId?)` / `updateInstanceServerRuntimeSettings(settings, instanceId?)`，写法和已有的 `getInstanceServerPassword`/`updateInstanceServerPassword` 完全同构（同一个 URL 做 GET 和 PUT）。
- `NewGameCreator.tsx`：
  - `defaultConfig()` 新增 `cabinMode: 'recommended'` 默认值。
  - 联机设置侧栏"联机小屋布局"上方新增"小屋模式"步进控件，复用现有 `ArrowButton` + 两态切换写法（和"资金管理"共享/分开的实现方式完全一样：左右箭头都触发同一个取反 toggle，不是新增交互模式），显示"推荐"/"原版"。
  - 故意只做二选一，不在这个页面暴露 `FarmhouseStack`——新建存档场景用户只需要决定"要不要隐藏小屋"，更细的三态选择留给服务器控制页。
- `ServerControlPage.tsx`：
  - 新增常量 `defaultRuntimeSettings`（`CabinStack`/`KeepExisting`/`1`，和后端 `ReadServerRuntimeSettings` 的兜底默认值保持一致，读取失败时前端也用同一组默认值兜底，避免两边默认值不同步产生困惑）。
  - 新增状态：`runtimeSettingsOpen`/`runtimeSettingsDraft`/`runtimeSettingsLoading`/`runtimeSettingsSaving`/`runtimeSettingsError`/`runtimeSettingsMessage`，和密码弹窗的一组状态命名风格、职责划分完全一致。
  - 新增 `openRuntimeSettings()`/`handleSaveRuntimeSettings()`，分别对应密码弹窗的 `openPasswordSettings()`/`handleSaveServerPassword()`。
  - "快捷操作"网格里紧跟"服务器密码设置"之后新增"小屋与联机高级设置"按钮：`sd-btn-tan sd-btn--lg`，只按 `!isAdmin` 禁用，**不要求 `isRunning`**——因为这组配置本来就只在容器下次启动时读取，服务器是否在运行不影响能否编辑（这点和密码设置按钮的门控逻辑一致，但和"触发节日活动"这类要求运行中的按钮不同，注意别混淆）。
  - 新增对应弹窗（`runtimeSettingsOpen`，插在 `passwordOpen` 弹窗和 `jojaOpen` 弹窗之间），三个 `<select>` 分别对应 `CabinStrategy`（三选一，选项文案直接写清楚每个策略的效果）、`ExistingCabinBehavior`（二选一）、`NetworkBroadcastPeriod`（`1`/`2`/`3` 三个预设，没有做自定义数字输入——三档预设覆盖常见场景就够了，加输入框是不必要的复杂度）。保存成功提示"设置已保存，需要重启服务器容器后才会生效"，措辞刻意和密码弹窗的提示保持一致。

## 影响文件

- `frontend/src/types.ts`
- `frontend/src/api.ts`
- `frontend/src/games/stardew/NewGameCreator.tsx`
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`

未新增图片素材（弹窗内是纯 `<select>`，沿用 `sd-input`/`sd-schedule-field`/`sd-confirm-overlay` 既有样式类）；未改生命周期 API、密码设置、计划重启、VNC、控制台命令或 Junimo 通信。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过。
- `cd frontend; npm run build`（`tsc -b && vite build`）：通过。

**未做的验证**：没有连一个真实运行中的实例实际打开这两个新入口点一遍（新建存档页切换"小屋模式"提交、服务器控制页打开"小屋与联机高级设置"弹窗读取/保存），没有截图确认弹窗在移动端窄屏下的表现，也没有验证保存后重启服务器容器时 `server-settings.json` 是否真的按预期变化（这部分依赖后端 `UpdateServerRuntimeSettings` 的行为，后端侧已有单元测试覆盖，但没有端到端联调）。

## 下一步注意事项

- `NewGameCreator.tsx` 的"小屋模式"控件和"联机小屋布局"（`cabinLayout`）是两个独立字段，容易混淆：`cabinMode=vanilla` 时小屋才会真的出现在地图上，`cabinLayout`（靠近/分散）在这种情况下才会产生视觉差异；`cabinMode=recommended` 时小屋被隐藏堆叠，`cabinLayout` 字段仍然会被提交但对视觉没有实际影响。如果以后要做"vanilla 模式下才显示布局选项"这类联动 UI，需要注意这是一个可选的体验增强，这次没有做。
- 服务器控制页的弹窗按钮标题和描述文案里没有提到"需要服务器运行中"，因为这组设置本来就不要求；如果以后有用户误以为这是运行时热改配置，可以考虑在按钮 title 里也加一句"重启后生效"提示，这次只在弹窗内部提示，没有在按钮 hover title 上重复。
