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

# APPROVE-PENDING-AUTH-1 批准待认证玩家（同日追加）

## 背景

后端已实现"批准待认证玩家"能力（详见 `docs/backend-handoff/backend-handoff-2026-07-10.md` 的 `APPROVE-PENDING-AUTH-1` 小节）：新增 `POST /api/instances/:id/players/approve-auth`，`GET /players` 返回的每个玩家新增 `isAuthenticated`（`boolean | null`），`GET /password-status` 新增 `passwordBridgeAvailable`/`passwordBridgeDetail` 自检字段。这次做前端接线。用户明确要求：待认证玩家在 UI 上新增一个**独立卡片**，不要合并进现有"在线玩家"表格。

## 改了什么

- `types.ts`：`StardewPlayerInfo` 新增 `isAuthenticated?: boolean | null`；`InstancePasswordStatus` 新增 `passwordBridgeAvailable?: boolean`/`passwordBridgeDetail?: string`。
- `api.ts` 新增 `approvePlayerAuth(uniqueMultiplayerId, instanceId?)`，`POST /players/approve-auth`，写法和已有的 `kickPlayer` 完全同构。
- `PlayersPage.tsx`：
  - 新增本地状态 `passwordStatus`（`InstancePasswordStatus | null`）+ 一个 `useEffect`（`isRunning` 为真时调用 `getInstancePasswordStatus()`，失败时静默置 `null`，不打断页面渲染）。这是页面第一次引入 `useEffect`——此前整页数据完全由外部 `dashboardData` prop 驱动（一个共享轮询 hook），没有把 `passwordStatus` 塞进那个共享数据层，因为它只有这一个页面需要，参考 `ServerControlPage.tsx` 里 `getInstancePasswordStatus` 已经是"页面自己按需拉取，不进全局轮询"的既有做法。
  - `pendingAuthPlayers = playerRows.filter(p => p.status === 'online' && p.isAuthenticated === false)`：严格用 `=== false` 而不是 `!p.isAuthenticated`，因为 `undefined`/`null`（旧版本控制模组没有这个字段，或反射查询失败）不代表"待认证"，只有明确拿到 `false` 才展示。
  - 新增独立卡片区块 `sd-players-pending-auth-section`，插在"在线玩家"表格和"玩家活动/最近事件"区块之间。只在 `passwordStatus?.enabled` 为真时渲染整个卡片（密码保护没开就没有"待认证"这个概念，不显示空卡片占位）；`passwordBridgeAvailable === false` 时卡片顶部展示提示文案（反射桥不可用，模组版本过旧或不兼容，暂时无法批准，需要玩家自己 `!login`），并把 `passwordBridgeDetail` 放进这段提示的 `title` 属性里，供管理员 hover 查看具体诊断信息而不占用可见空间。
  - 每行显示玩家名 + 联机 ID（复用现有 `shortId` 帮助函数）+ "批准"按钮。按钮禁用条件 `!isAdmin || !isRunning || !passwordStatus?.passwordBridgeAvailable || !player.uniqueMultiplayerId || approveBusy`，完全对齐现有踢出按钮的 `disabled`/`title` 写法风格。
  - 复用踢出功能的"确认弹窗 → busy → 调用 API → 成功/失败提示 → `dashboardData.refreshPlayers()`"整套状态模式，新增同构的 `approveConfirmTarget`/`approveBusy`/`approveError`/`approveMessage`（复用已有的 `KickTarget` 类型，字段完全一样，没有重复定义类型）。确认弹窗文案强调"该操作会立即让玩家进入正式农场，等同于服务器替其正确输入了一次密码"，保留二次确认——虽然批准不是破坏性操作，但会立即改变玩家的游戏状态，不是纯只读操作。
- 没有新增 CSS：卡片和行内元素全部复用现有 `sd-srv-section`/`sd-players-table-*`/`sd-players-badge-waiting`/`sd-players-empty*`/`sd-btn-green`/`sd-confirm-*` 等既有类名，没有为这个卡片写任何新样式规则。

## 影响文件

- `frontend/src/types.ts`
- `frontend/src/api.ts`
- `frontend/src/games/stardew/pages/PlayersPage.tsx`

未新增图片素材；未改踢出玩家、密码设置弹窗、生命周期 API、权限判断（除新增按钮自身的禁用条件外）或轮询逻辑。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过。
- `cd frontend; npm run build`（`tsc -b && vite build`）：通过。

**未做的验证**：没有连一个真实开启 `SERVER_PASSWORD` 的运行实例走一遍完整流程（真实玩家连接卡在待认证 → 面板卡片显示 → 点击批准 → 玩家真的进入农场），也没有截图确认卡片在移动端窄屏下的布局表现。后端侧已有单元测试覆盖数据管线和命令写入，但没有端到端联调；这一层验证依赖后端 handoff 文档里提到的真机测试，建议下一位维护者一起完成。

## 下一步注意事项

- 页面里已经存在但从未被后端赋值过的 `isWaitingPlayerStatus`（`status === 'waiting'|'pending'|'joining'`）和"等待加入: N"徽章，是历史遗留的预留扩展点（全仓库搜索确认后端从未产出这些 status 值）。这次的"待认证"概念**没有**复用这套逻辑，而是走独立的 `isAuthenticated` 字段 + 独立卡片，两者是完全不同的概念，不要混淆或试图合并。
- `passwordStatus` 只在页面挂载且 `isRunning` 为真时拉取一次，不会随 `dashboardData` 的轮询周期自动刷新；如果管理员在打开玩家页之后才把密码保护打开/关闭，需要手动切换路由或刷新页面才会看到卡片显示/隐藏状态变化。如果以后反馈这个延迟造成困扰，可以考虑加一个定时轮询或"刷新"按钮，这次按最小实现处理。

# PLAYERS-BAN-1 封禁玩家 + 玩家行操作按钮精简（同日追加）

## 背景

用户看着玩家管理页"在线玩家"表格每行的图标按钮截图，要求精简为只保留"踢出"，图标换成"管理操作"卡片同款真实 PNG，旁边加一个新按钮。讨论过程详见 `docs/backend-handoff/backend-handoff-2026-07-10.md` 的 `PLAYERS-BAN-1` 小节——最初讨论的"取消认证"方向被放弃（JunimoServer 认证状态纯内存不持久化，"踢出"本身已经语义等价），转而调研到 JunimoServer 真正有 `!ban`/`!unban`/`!listbans` 聊天指令，对应玩家管理页"管理操作"卡片里一直禁用、标着"待接入"的"封禁玩家"。后端新增 `POST /players/ban`，这次做前端接线。

## 改了什么

- `api.ts` 新增 `banPlayer(name, uniqueMultiplayerId, instanceId?)`，`POST /players/ban`，写法和 `kickPlayer` 完全同构。
- `PlayersPage.tsx`：
  - 新增状态 `banConfirmTarget`/`banSelectId`/`banBusy`/`banError`/`banMessage`（复用已有的 `KickTarget` 类型），新增 `banTargetPlayers = playerRows.filter(p => !p.isHost && p.uniqueMultiplayerId)`（不按在线状态过滤，理由：封禁本来就该支持针对离线/曾经离开的玩家，`!ban <name>` 按名字匹配、`Game1.getAllFarmers()` 本来就包含离线 farmhand）。
  - "在线玩家"表格行内操作区（`sd-players-row-actions`）：删除恒禁用的"发送消息"和"更多操作"两个占位按钮，只保留"踢出"图标按钮，旁边新增"封禁"图标按钮（`sd-players-icon-button sd-players-icon-ban`），禁用条件 `!isAdmin || !isRunning || player.isHost || !player.uniqueMultiplayerId || banBusy`。
  - "管理操作"卡片"封禁玩家"从恒禁用改为真实可用：`<select>` 用 `banTargetPlayers` 填充选项，"封禁"按钮按 `!isAdmin || !isRunning || !banSelectId || banBusy` 判断，移除 `<span className="sd-srv-badge-pending">待接入</span>`。
  - 新增 `handleConfirmBan()`，完全复用 `handleConfirmKick`/`handleConfirmApprove` 的"busy → 调用 API → 成功/失败提示 → `dashboardData.refreshPlayers()`"结构。
  - 新增确认弹窗（复用 `sd-confirm-overlay`/`sd-confirm-dialog`），文案**必须包含重启可能失效的提示**："封禁玩家 {name}？该玩家会被立即断开且暂时无法重新加入服务器；如果之后重启了服务器容器，这条封禁可能会失效，需要重新操作。"——这是用户明确要求的措辞取舍，不要简化成"永久封禁"这类过度承诺的说法。
  - `sd-srv-hint` 提示文案同步更新，说明封禁和踢出一样通过控制模组发送、无法针对主机，且封禁会临时提升主机为管理员。
- `StardewPanel.css`：`.sd-players-icon-boot::before`（行内小按钮唯一生效的定义，之前是纯 `linear-gradient`/`radial-gradient` 画的靴子矢量图形，不引用任何图片）改为直接复用 `icon_players_action_boot_image2.png`；新增 `.sd-players-icon-ban::before` 复用 `icon_players_action_ban_image2.png`；删除因移除"更多操作"按钮而变成孤儿样式的 `.sd-players-icon-more::before` 规则，以及只被"发送消息"占位按钮用过、现在也孤儿的 `.sd-players-icon-button::before` 默认消息图标规则（改成 20×20 通用尺寸留给 `-boot`/`-ban` 两个具体图标覆盖）。这是本次改动直接导致的孤儿样式清理，不是清理无关的历史遗留代码。

## 影响文件

- `frontend/src/api.ts`
- `frontend/src/games/stardew/pages/PlayersPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`

未新增图片素材（直接复用"管理操作"卡片已有的两张 PNG）；未改踢出玩家、批准认证、密码设置弹窗或轮询逻辑。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过。
- `cd frontend; npm run build`（`tsc -b && vite build`）：通过。

**未做的验证**：没有连一个真实运行实例实际点一遍行内封禁图标和"管理操作"卡片的封禁按钮，确认目标玩家真的被断开且无法重新加入；也没有截图验证移动端窄屏下两个行内图标按钮（踢出+封禁）的间距和触控热区是否合适。这部分依赖后端 `PLAYERS-BAN-1` 的真机联机验证一起完成，建议下一位维护者一起补。

## 下一步注意事项

- 玩家管理页"管理操作"卡片里的 select+button 目前被一条较晚的 CSS 规则（`StardewPanel.css` 约 15131 行 `.sd-players-action-select, .sd-players-action-item > button { display: none; }`）整体隐藏——这是既有状态，"踢出玩家"卡片早就是这样，本次"封禁玩家"卡片照抄同一结构保持一致，不是本次引入的新问题。真正生效的交互入口是行内图标按钮。如果以后要让卡片内的 select/button 重新可见，需要先弄清楚这条 CSS 规则当初为什么要隐藏它们，不要贸然删除。
- 封禁确认弹窗的措辞（"可能会失效"）依赖后端反编译验证 `Game1.bannedUsers` 是否跨容器重启持久化的结果（见 `docs/backend-handoff/backend-handoff-2026-07-10.md` `PLAYERS-BAN-1` 小节"未做的验证"）。确认结果出来后需要同步修正这段措辞，不要让前端文案和实际行为脱节。
- 没有实现"解封"（`!unban`）对应的 UI 入口，封禁操作目前无法从面板撤销。这是本次按"先做简单接通"明确排除的范围，如果后续用户反馈需要撤销入口，是自然的后续迭代。
