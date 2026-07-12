# MOBILE-SHELL-SAVES-REFINE-1 手机端背景、顶栏与存档卡片优化

## 背景

用户继续反馈移动端整体视觉：服务器模组不要展示内置模组；存档页农场原画素材过大，需要缩小并和核心信息放在同一卡片；移动端上横栏需要更接近 PC 端风格；整个手机端背景要改成 PC 端页面背景样式。当前实现优先复用仓库已有 image2/PC 素材，没有新增或重新生成图片。

## 改了什么

- `MobileModsPage.tsx`：
  - 服务器模组列表恢复过滤 `builtIn` 和 `modIsSystemRuntime()`，即不展示 SMAPI、`StardewAnxiPanel.Control`、`JunimoServer` / `JunimoHost.Server` 这类内置/系统运行组件。
  - 搜索结果页不受影响，仍按 Nexus 搜索结果展示已安装状态。
- `MobileSavesPage.tsx` / `MobileSavesPage.css`：
  - 删除独立的大农场原画卡。
  - “核心信息”卡新增 `sd-msave-summary-body`：左侧是约 72px 宽的 16:9 农场原画缩略图（仍复用原 `saveFarmMapSrc()` 映射），右侧是存档名称、农场名称、农场主、游戏日期。
  - 存档状态徽章从缩略图叠层里移出，放到缩略图上方，避免“当前使用中”和农场图重叠。
  - 原“更多信息”和“存档操作”卡保持独立，导出/导入逻辑不变。
- `StardewMobileShell.css`：
  - `.sd-mshell` 背景改为 PC 端同款 `background_app_black.png` 深色纹理。
  - `.sd-mshell-body` 复用 PC 端 `.sd-main` 的 image2 页面框素材：四角、四边、中心 tile，移动端尺寸使用更小的 frame corner/edge 变量。
  - `.sd-mshell-topbar` 改为轻量 PC 顶栏风格，复用 `icon_topbar_chicken_image2_v2.png`；状态徽章改为浅棕像素按钮样式。后续按用户反馈不再拼接 PC 顶栏左右端图，而是使用新生成的手机端横栏素材，避免手机宽度下背景出现接缝/撕裂感。
  - 注意：`StardewMobileShell.tsx` 的 header 已移除旧的 `sd-bg-wood-strip` 类；如果保留该类，全局木纹工具类可能在打包后覆盖 `mobile_topbar_generated_image2.png`，导致手机上看不到新横栏。
  - 最终滚动模型：`.sd-mshell` 固定为 `height: 100dvh` 且 `overflow: hidden`，`.sd-mshell-body` 负责固定 image2 页面框背景与上下裁切，`.sd-mshell-scroll` 设置 `min-height: 0; overflow-y: auto; overscroll-behavior: contain` 作为唯一纵向滚动容器。因此手机端上下滑动时只有主体内容滚动，顶栏、底栏和 image2 页面框背景保持不动。
  - 按最新反馈，主体滚动模型调整为“背景框固定 + 内层滚动裁切”：`.sd-mshell-body` 继续承载 PC image2 页面框背景并设置 `overflow: hidden` 与上下边框安全区；页面内容包进 `.sd-mshell-scroll`，由它作为唯一纵向滚动容器。这样中间组件滑过背景自带的上/下木纹框线时会被裁掉，不再遮挡框线本身。
  - 底部 Tab 占位从 `.sd-mshell` 的底部 padding 挪到 `.sd-mshell-scroll` 的底部 padding：外壳不再在底部留黑色空白，`.sd-mshell-body` 的页面框背景可以铺满到屏幕底部；内容列表末尾仍能滚过悬浮 Tab，不会被底栏遮住。
  - 上方裁切区继续由 `.sd-mshell-body` 的上边框安全区保护背景木纹线，但上边距已减半为 `calc(var(--sd-mobile-main-frame-edge-thickness) / 2)`，`.sd-mshell-scroll` 的顶部额外 padding 保持 0，让页面组件更贴着裁切区内侧顶格摆放；底部安全区仍保留完整 frame 厚度并叠加滚动内容底部 Tab 留位。
- `frontend/public/assets/stardew/ui/topbar/mobile_topbar_framed_generated_image2.png`：
  - 使用内置 imagegen 生成一张移动端顶栏木纹素材。第一版裁取横条后木纹视觉偏大；第二版仍缺少四周边框。最终 framed 版明确要求“完整四边像素木框、边框贴边、无文字/图标/按钮”。生成器返回的图不是目标尺寸，所以没有裁切，而是把整张图缩放到 1170×174，保留完整四边框。
  - CSS 使用 `background-size: 100% 100%` 填充 `.sd-mshell-topbar`，URL 带 `?v=mobile-topbar-4` 防手机缓存。
  - 生成提示词要点：complete four-sided pixel-art wooden frame border、fills entire canvas edge-to-edge、Stardew-inspired image2 mobile top header bar、no black outer margin、no text/logos/icons/buttons。

## 影响文件

- `frontend/src/games/stardew/StardewMobileShell.css`
- `frontend/public/assets/stardew/ui/topbar/mobile_topbar_framed_generated_image2.png`
- `frontend/src/games/stardew/mobile/MobileSavesPage.tsx`
- `frontend/src/games/stardew/mobile/MobileSavesPage.css`
- `frontend/src/games/stardew/mobile/MobileModsPage.tsx`

未新增后端接口，未改桌面端页面。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过。
- `cd frontend; npm run build`：通过，Vite 仍提示主 chunk 超过 500 kB。

# MOBILE-MODS-M7-1 手机端模组页

## 背景

用户要求在手机端底部 Tab 的“玩家”和“存档”之间新增“模组”，并把手机端模组页拆成“搜索 / 服务器模组”两个子页。搜索页要尽量复用 PC 端 Nexus 搜索展示能力；服务器模组页要把“已安装模组”和“启用状态”合并到同一张详情卡片，并把真实可点击的启用开关放到卡片右下角。实现后用户追加多轮视觉调整：搜索框以上全部删除、去掉安装按钮、继续缩小卡片和按钮、去掉来源/作者/下载/认可、跳转按钮和前置状态按钮平齐、搜索结果一页展示 4 个、分页控件缩到一行、热门标签去掉 Backpack；服务器模组卡去掉作者/来源字段，改用 N 站缩略图，启用开关去掉文字和外框，放在底部标签行右侧且和“内置”等标签平齐。因此最终手机搜索页不展示 Nexus Key / 扩展连接区，不提供安装按钮，搜索结果卡只保留更轻量的信息，底部为左侧前置状态 + 右侧小号 N 站跳转的一行，分页为上一页 / 页码输入 / 跳转 / 下一页单行。

## 改了什么

- `StardewMobileShell.tsx`：
  - 新增 `MobileModsPage` 挂载分支，底部 Tab 从 5 个改为 6 个：总览 / 控制 / 玩家 / 模组 / 存档 / 更多。
  - “模组”Tab 使用现有 image2 风格图标 `icon_nav_mods_crystal_image2.png`，位置固定在“玩家”和“存档”之间。
  - 仅影响移动壳；桌面 `StardewPanel` 导航未改。
- `StardewMobileShell.css`：
  - 底部导航 grid 调整为 6 列，缩小 tab 文案字号，继续保留浮动胶囊底栏和 safe-area 底部占位。
  - `.sd-mshell-body` 从垂直居中改为顶部对齐，只保留横向居中，避免“搜索 / 服务器模组”内容高度不同导致页头和二级 Tab 在切换时上下跳动。
- 新增 `mobile/MobileModsPage.tsx`：
  - 顶部右侧全局操作：刷新、导出、上传；上传弹窗复用 `uploadMods`，导出复用 `exportMods`。
  - 二级 Tab：搜索 / 服务器模组。
  - 搜索页复用 `searchNexusMods`，结果状态用 `getMods` 返回的已安装列表校正；`NEXUS_PAGE_SIZE=4`，并把 session key 升到 `v2`，避免旧手机会话继续恢复 8 个结果。热门标签为 `UI Info`、`Fishing Mod`、`Tractor`。单列卡片展示缩略图、名称、Nexus ID、版本、更新时间、简介、前置依赖状态、已安装/已安装未启用状态和“跳转 N站”。按用户最终反馈，搜索框上方的 Nexus Key/扩展连接区已删除，卡片里的安装/会员安装/一键安装按钮已删除，来源/作者/下载/认可四项也已移除；前置状态按钮和小号 N 站跳转按钮放在同一行，顶部平齐。
  - 服务器模组页复用 `getMods` 和 `updateModEnabled`。最终口径改为隐藏 `builtIn` 与 `modIsSystemRuntime()` 命中的内置/系统组件，只展示用户安装 Mod。每张卡融合详情和状态：左侧使用 `mods[].pictureUrl` 渲染约 74px 方形 N 站缩略图（无图时用 NEXUS/MOD 占位），右侧展示名称、启用状态、版本、文件夹、更新时间、同步类型；作者/来源字段按用户反馈移除。启用状态徽章固定在名称行右侧；底部标签行右侧是真实 `<input type="checkbox">` 开关，样式为无文字、无外框的绿色小开关。进入“服务器模组”子页时主动刷新一次 `GET /mods`，避免只依赖移动壳初始化数据；切换启用状态时按单个 Mod 锁定，成功后刷新当前列表并同步公共 `dashboardData.refreshMods()`，失败展示 `sd-notice--error`。
- 新增 `mobile/MobileModsPage.css`：
  - class 前缀 `sd-mmods-`，只作用于移动端模组页。
  - 搜索结果和服务器模组均为单列卡片；搜索卡片进一步压缩了内边距、缩略图、信息区和描述字号，底部新增 `.sd-mmods-search-footer` 让前置状态与 N 站跳转按钮同排平齐；分页 `.sd-mmods-pager` 固定为上一页 / 页码输入+跳转 / 下一页单行；搜索框、分页输入和服务器模组开关仍保持移动端可点击热区，卡片内长文本用 `overflow-wrap:anywhere`，避免 390px 宽度横向溢出。
- `qa-layout-main.tsx`：
  - 补了移动端 Nexus 搜索和上传/导出所需 mock 路由，便于 `qa-layout.html?shell=mobile` 下继续渲染页面。

## 影响文件

- `frontend/src/games/stardew/StardewMobileShell.tsx`
- `frontend/src/games/stardew/StardewMobileShell.css`
- `frontend/src/games/stardew/mobile/MobileModsPage.tsx`
- `frontend/src/games/stardew/mobile/MobileModsPage.css`
- `frontend/src/qa-layout-main.tsx`

未新增后端接口；未改桌面端 `ModsPage.tsx` 的搜索、安装、上传、导出和启用禁用逻辑。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过。
- `cd frontend; npm run build`：通过，Vite 仍提示主 chunk 超过 500 kB，这是既有体积提示。

## 下一步注意事项

- 本轮没有做真机点击验证。建议下一位维护者用 `npm run dev -- --host 0.0.0.0` 后在手机访问局域网地址，或在浏览器用 `qa-layout.html?shell=mobile&state=running` 检查 390×844、393×852、430×932：底部 6 Tab 不遮挡内容，搜索卡/分页不横向溢出，切换“搜索 / 服务器模组”时页头和二级 Tab 不上下跳动，服务器模组开关位于卡片右下角且可点击。
- 手机搜索页当前按用户最终口径只做精简展示与跳转 N 站，不提供安装按钮，也不展示来源/作者/下载/认可四个统计项。以后如果要恢复移动端安装入口或更完整的统计密度，应先重新确认交互位置，避免把 Nexus Key/扩展连接区放回搜索框上方。

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

# MOBILE-SHELL-M0-1 移动端基础入口（同日追加）

## 背景

用户要求在同一个前端工程内加移动端入口，不新建独立项目。M0 范围明确限定为"识别手机端并切到占位壳"：不重做具体页面、不改后端、不改登录/权限，只在 Stardew 面板入口处按视口宽度分流。

## 改了什么

- 新增 `frontend/src/hooks/useMediaQuery.ts`：通用 `useMediaQuery(query: string): boolean` hook。内部用 `window.matchMedia(query)` 取初始值，`useEffect` 里订阅 `change` 事件同步更新；SSR/无 `matchMedia` 环境下降级返回 `false`。不写死具体断点，纯工具 hook，供以后其它响应式判断复用。
- 新增 `frontend/src/games/stardew/StardewMobileShell.tsx`：M0 占位壳组件，不接收 props，内部直接调用既有 `useStardewDashboardData()` 拿 `instanceState.state` + `loading`：
  - 顶部品牌栏复用 `stardew-theme.css` 的 `.sd-bg-wood-strip` 木纹条背景，左侧文字 `Stardew Anxi Panel`，右侧状态徽章按 `loading→初始化中`、`running→运行中`、`stopped→已停止`、其它状态复用 `core/helpers.ts` 的 `stateLabel()` 兜底，状态点复用 `.sd-dot`/`.sd-dot-green`/`.sd-dot-red`/`.sd-dot-yellow` 语义色。
  - 中间是一张复用 `.sd-panel`（`stardew-theme.css` 里已有的羊皮纸面板工具类：3px 棕边框 + 米色底）的占位卡片，文案"移动端面板建设中"。
  - 底部固定 5 个 Tab（总览/控制/玩家/任务/更多），本地 `useState<MobileTabKey>` 控制 `active` class 高亮，点击只切本地 state，不触发路由跳转或数据请求；`active` 态用 `--sd-green-bg`/`--sd-green-text` 变量，和桌面端"绿色 active"语义一致。
- 新增 `frontend/src/games/stardew/StardewMobileShell.css`：class 前缀统一 `sd-mshell-`，与 `StardewPanel.css` 的桌面 Shell 类名（`sd-shell`/`sd-topbar`/`sd-sidebar` 等）完全不重叠，只从 `stardew-theme.css` 读颜色变量和 `.sd-bg-wood-strip`/`.sd-panel`/`.sd-dot-*` 工具类，没有新增图片素材、没有引入新 UI 库。根容器 `.sd-mshell` 用 `overflow-x: hidden` + 全局既有的 `box-sizing: border-box`（`App.css` 顶部 `* { box-sizing: border-box }`）防止横向溢出；底部 Tab 用 `position: fixed`，`.sd-mshell` 用 `padding-bottom` 让内容不被遮挡；顶部/底部都叠加 `env(safe-area-inset-*)` 安全区（沿用 `FE-MOBILE-FIXES-1` 的既有做法）。
- `frontend/src/App.tsx`：新增 `const isMobile = useMediaQuery('(max-width: 768px)')`（无条件调用，遵守 hooks 规则）；渲染分支从 `return <StardewPanel .../>` 改为 `return isMobile ? <StardewMobileShell /> : <StardewPanel user={currentUser} onLogout={logout} />`，只在这一处分流，其余 `booting`/`setup`/`login` 视图未改。
- `frontend/src/qa-layout-main.tsx`（既有 QA mock 入口，`frontend/qa-layout.html` 引用）新增 `shell` query 参数：`?shell=mobile` 渲染 `StardewMobileShell`，不带该参数或 `?shell=desktop` 保持原有渲染 `StardewPanel` 的行为不变。方便后续迭代移动端布局时用同一套 mock fetch 数据回归对比，不用每次连真实后端。

## 影响文件

- 新增：`frontend/src/hooks/useMediaQuery.ts`、`frontend/src/games/stardew/StardewMobileShell.tsx`、`frontend/src/games/stardew/StardewMobileShell.css`。
- 修改：`frontend/src/App.tsx`、`frontend/src/qa-layout-main.tsx`。
- 未改：后端任何文件、`frontend/src/games/stardew/StardewPanel.tsx`、`StardewPanel.css`、任何登录/权限/API 逻辑、`useStardewDashboardData.ts` 内部实现（只是在新组件里多调用了一次，桌面端和移动端二选一渲染，不会同时挂载两份轮询）。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过。
- `cd frontend; npm run build`（`tsc -b && vite build`）：通过，构建产物体积无明显变化（新增代码量很小）。
- **未做的验证**：没有用真实浏览器窗口缩放或移动设备实测视觉效果——当前环境没有可用的浏览器自动化/截图工具，只能靠阅读 `StardewPanel.css`/`stardew-theme.css` 已有断点和变量定义、以及 `App.css` 全局 `box-sizing: border-box` 规则推理没有横向溢出。建议下一位维护者用 `frontend/qa-layout.html?shell=mobile` 在真实浏览器里分别打开 768px/390px/320px 宽度，确认：无横向滚动、底部 Tab 贴底、状态徽章颜色随 `?state=running/stopped` 变化、点击 5 个 Tab 高亮态正确切换。也没有连真实后端在手机浏览器上验证 768px 分流阈值本身（`useMediaQuery` 逻辑简单，风险低，但建议补一次真机确认）。

## 下一步 M1 注意事项

- `StardewMobileShell` 目前是完全独立组件，不复用 `StardewPanel.tsx` 里的路由状态、`navigate()` 或页面组件；M1 如果要接入真实页面内容，需要决定是"移动端也走同一套 `stardew-routes.ts` route + 页面组件"还是"移动端维护一套独立的精简页面"。当前 5 个 Tab（总览/控制/玩家/任务/更多）和桌面 9 路由不是一一对应，`更多` 大概率要收纳存档/模组/诊断/设置这几个桌面路由，M1 需要先定 Tab 到路由的映射关系。
- `StardewMobileShell` 现在不接收 `user`/`onLogout`，没有登出入口；`App.tsx` 里 `currentUser` 和 `logout` 都还在但只传给了桌面分支。M1 做真实内容时大概率需要把这两个 prop 传回移动壳（比如"更多" Tab 里放登出按钮），到时候在 `App.tsx` 分流处顺手补上就行，不需要现在预先加。
- M0 的 5 个 Tab 是纯 `useState` 本地高亮，刷新页面或从桌面态切回移动态都会重置到"总览"；M1 如果要让 Tab 状态和 URL/History API 同步（比如支持浏览器后退），需要参考 `StardewPanel.tsx` 里 `parseRoute`/`routeToPath`/`popstate` 监听的既有实现方式，不要另起一套机制。
- 768px 阈值目前只在 `frontend/src/hooks/useMediaQuery.ts` 调用处硬编码在 `App.tsx` 里（`'(max-width: 768px)'` 字符串字面量），没有做成常量导出；如果 M1 需要在其它组件里判断"当前是否移动端"（例如页面内部想要展示不同内容），建议把这个断点字符串提成一个共享常量，避免多处复制粘贴导致断点不一致。
- 桌面 `StardewPanel.css` 里 640px/720px/960px 的既有响应式断点没有删除，只是因为 `<=768px` 现在会被分流到 `StardewMobileShell`，这些断点在实际使用中很难再被触发（除非以后改分流阈值）。如果后续要做"移动端也能强制切回桌面视图"这类调试/兼容开关，需要注意这些旧断点规则依然可用，是个现成的回退方案。

# LOGIN-MOBILE-FIX-1 移动端登录/初始化页布局修复（同日追加）

## 背景

用户反馈手机浏览器打开登录页布局崩坏，并附了真机截图：一条半透明米色横条写着"Stardew Anxi Panel"，硬生生压在木框顶部的鸡形徽标上，登录卡片和背景图的比例明显不对。用户特别指出"没有输入法的时候也有突出，只是没有这么严重"，说明这不只是软键盘弹出时的挤压问题，静态状态下就已经错位。

排查后定位到根因：`App.tsx` 在 `view==='login'`/`view==='setup'` 时永远会给 `.sd-auth-shell` 加上 `sd-auth-shell--image-login` 修饰类（`App.css` 里那套"非 image2"的旧版羊皮纸登录卡样式，也就是 M0/移动端相关文档之前提到的 `.sd-auth-card` 基础规则，实际上只有 `booting` 这个一闪而过的中间态会用到，登录/注册页面本身从未使用过它）。`.sd-auth-shell--image-login .sd-auth-card` 的实现方式是：把整张桌面原型图 `background_login_home_image2.png` 当成卡片本体的背景图，卡片自身用 `position:absolute` + `width:max(100vw, 100dvh*1.7779), height:max(100vh, 100vw*0.5628)`（16:9 反算）撑成和原型图相同纵横比的一个大盒子，再用 `translate(-50%,-50%)` 居中；输入框、密码框、按钮全部是绝对定位的透明热区，坐标写死成相对这个大盒子的百分比（例如 `left:61.6%; top:40.8%; width:26.62%`），指望这个大盒子和原型图完全重合。

这套方案只在盒子恰好是 16:9 横向比例时成立。手机竖屏视口（比如 390×844）算出来的盒子宽度是 `max(390, 844*1.7779)=1500px`，比屏幕宽近 4 倍；外层 `.sd-auth-shell` 又有 `overflow:hidden` 和固定 `min-height:100vh`（绝对定位的卡片不参与父级流内高度计算，父级永远刚好 100vh 高，不会随内容变高），于是超出视口的部分被直接裁掉而不是滚动出去。仓库里已经有一版 `@media (max-width:700px)` 补丁试图靠 `transform:translate(-77%,-50%)` 把大盒子推到"露出右侧表单区域"的位置，并且新增了一条 `position:absolute` 的全屏标题横条盖住原图里对应区域——但这个位移量是按某一个参考机型手算出来的百分比，换一个宽高比（390 vs 393 vs 430，或者随手机型号原生 DPI 微调）就会跟着错位，这正是用户截图里标题横条盖住鸡形徽标、卡片被压扁的直接原因。

## 改了什么

只改了一个文件：`frontend/src/App.css`，在文件末尾新增一段 `@media (max-width: 768px)` 覆盖，作用域全部限定在 `.sd-auth-shell`/`.sd-auth-shell--image-login` 极其子选择器上，不改登录组件的 TSX、不改 API、不改认证逻辑：

- `.sd-auth-shell`：`min-height` 补 `100dvh`（`100vh` 之后再声明 `100dvh` 兜底，不支持的浏览器自动忽略第二条，和仓库里 `body:has(.sd-shell)` 已有的 `100vh`/`100dvh` 双声明写法一致）；`height:auto` 让容器可以随内容变高而不是被钉死在一屏；`overflow-x:hidden` 继续防横向溢出，`overflow-y:auto` 松开纵向裁切。
- `.sd-auth-shell--image-login`：`display:block` 改回 `flex` + `align-items:center; justify-content:center`（和最早的非 image2 版 `.sd-auth-shell` 用的是同一套居中写法），`padding:20px 0 calc(28px + safe-area-inset-bottom)` 兼顾键盘/手势条安全区。背景不再让卡片自己去顶原型图比例，改成 `background-size:cover; background-position:center` 铺满 shell（叠一层半透明暗色蒙层帮衬卡片可读性）。**背景素材本身有过一轮返工**：第一版直接复用 `background_login_home_image2.png`（桌面版那张"整张画死的假窗口"原图，木框/鸡形徽标/字段标签全部是图里的美术，不是真实 DOM），铺满裁切后用户反馈"背景还是 PC 端的登陆窗口，很违和"——因为这张图本来就是设计给"整张图当卡片本体"用的，手机端卡片已经改回真实 DOM 渲染后，背景里再铺一张假窗口截图，就会变成背景和前景各画一套 UI、互相打架。改用仓库里另一张已有素材 `background_login_farm_generated.png`（非 image2 版 `.sd-auth-shell::before` 用的纯像素农场场景，没有任何假 UI 元素，本来就是设计给"背景 + 悬浮真实卡片"这种组合用的），把同一种搭配方式从非 image2 版搬到 image2 移动端分支，视觉上背景和前景卡片不再互相冲突。两张图都是仓库已有素材，没有新增或替换图片文件，桌面端（>768px）依然使用 `background_login_home_image2.png` 原样不变。
- **第二轮视觉返工**：换完背景素材后用户反馈卡片本体还是"太丑"，想要接近 PC 端右侧那块木框招牌的质感。没有再去裁 `background_login_home_image2.png` 里的招牌区域当卡片背景——那样等于把"整张大图固定坐标"的老问题换了个更小的坐标系重新做一遍，换机型宽高比一样会裁歪或错位，得不偿失。改成给 `.sd-auth-card` 加了一个 `::before` 伪元素，摆放固定尺寸（52×59px，不参与任何百分比坐标换算）的鸡形徽标 `icon_topbar_chicken_image2_v2.png`——这张图不是新素材，是顶栏 `sd-topbar-brand-icon` 一直在用的同一张小图标，`position:absolute; left:50%; top:0; transform:translate(-50%,-62%)` 让它骑在卡片顶部边框上，视觉上呼应桌面大图里"鸡+木框"的招牌形象，但因为是固定像素尺寸的小图标、不依赖容器宽高比，不会重新引入之前那种绝对定位坐标错位的问题。配合把卡片 `padding-top` 从 22px 提到 30px（给徽标留出下坠空间）、`margin-top:30px`（给徽标向上探出的部分留出和 shell 顶部的间距）、`overflow:visible`（允许徽标探出卡片边框），以及把外层投影从 `0 14px 30px rgba(0,0,0,.5)` 加深到 `0 14px 30px rgba(0,0,0,.55)` 让卡片更有"雕刻木框"的立体感。
- `.sd-auth-shell--image-login .sd-auth-card`：放弃绝对定位和固定纵横比，改成 `position:relative; width:min(100% - 24px, 420px); height:auto`，正常参与文档流；背景改用现有的 `background_parchment_tile.png`（原本非 image2 版 `.sd-auth-card` 就是用这张贴图，这里是复用，不是新素材），配色/描边沿用同一套棕色木框数值，只是把桌面版的 5px 边框缩到 4px、`padding` 从 `26px 26px 30px` 收到 `22px 18px 26px`，对应第 6 条"缩小边距和装饰占用空间"的要求。
- 品牌文案（`.sd-auth-eyebrow`/`.sd-auth-title`/`.sd-auth-version`/`.sd-auth-loading`）在桌面 image2 版里是用"视觉隐藏"技巧（`position:absolute; width:1px; height:1px; clip:rect(0 0 0 0)`）藏起来的——因为桌面版指望原型图本身画出标题；手机端没有大图承载标题了，这段在移动端覆盖里改回 `position:static` 正常显示。
- 表单区域（`.form-grid`/`.field`/`.password-input`/输入框/按钮/`.form-hint`/`.sd-auth-error`）全部从绝对定位坐标改回正常网格布局。**这里有一处需要格外注意的 specificity 坑**：桌面版给用户名和密码两个 `.field` 的定位坐标不是写在 `.sd-auth-shell--image-login .field` 这个 2 类选择器上，而是额外用 `.sd-auth-shell--login .field:nth-of-type(1)` 这种 3 类选择器（`:nth-of-type` 在 specificity 计算里等同一个类）单独覆盖 `left/top/width`，selector 权重比 `.field` 基础规则更高。如果只重置 `.sd-auth-shell--image-login .field` 而不把这几条 `nth-of-type` 选择器也一起列进重置规则里，桌面坐标会原封不动地覆盖回来。这次是把两组选择器写在同一条分组规则里一起重置，规则本身在文件里排在最后，靠"选择器权重相等、源码顺序更靠后"生效，不是靠 `!important`。同样的坑也出现在 `@media (max-width:700px)` 那条旧手机补丁上（它用同样权重的选择器改坐标），新规则因为在文件更靠后，会覆盖回去。
- 密码显示/隐藏按钮和登录/注册按钮桌面版都是"透明背景 + 只画文字"，指望原型图本身画出按钮的木框/绿色底——手机端卡片不再用原型图做背景，所以这两个按钮补回真实可见的背景：密码切换按钮复用非 image2 版 `.sd-auth-card .password-toggle` 的棕边框/浅黄底配色；登录/注册按钮复用 `stardew-theme.css` 里到处都在用的 `button_primary_small_green_blank.png` 绿色像素按钮素材（同一张已有素材，仓库里存档页、服务器控制页等大量按钮都在用它，不是新引入的图）。
- `.sd-auth-error` 的重置里特意保留 `position:relative`（不是 `static`），是因为它的 `::before` 伪元素画错误图标 `!` 用了 `position:absolute; left:2%; top:50%`，如果祖先变成 `static`，这个百分比定位会失去参照物、跳去找更外层的 `.sd-auth-card` 当基准，图标位置会错乱；保留 `relative` 让它继续以错误提示条自身为基准。
- 输入框统一 `min-height:40px`、字号锁定 `16px`（防 iOS Safari 聚焦时整页自动放大，这是仓库里 `stardew-theme.css` `.sd-input` 移动端断点已经用过的同一个技巧）；登录/注册按钮 `min-height:44px`，对应第 7 条触控热区要求。
- 没有额外加 JS 处理"聚焦滚动到输入框"——改成正常文档流后，卡片和 `.sd-auth-shell` 都不再固定死高度，`overflow-y:auto` 生效，浏览器聚焦输入框时的原生 `scrollIntoView` 行为可以正常工作，不需要手写滚动逻辑。

## 影响文件

- `frontend/src/App.css`（唯一改动文件，新增约 200 行 `@media (max-width:768px)` 覆盖，全部在文件末尾追加，没有改动任何既有规则本身）。

未改：`frontend/src/App.tsx`、`frontend/src/core/LoginPanel.tsx`、`frontend/src/core/SetupPanel.tsx`、`frontend/src/core/Field.tsx`、`frontend/src/core/PasswordInput.tsx`、任何登录/初始化/session API、`frontend/src/games/stardew/StardewPanel.css`、`StardewMobileShell.*`。桌面端（>768px）视觉路径完全没有被这段新规则触碰到，选择器全部限定在这个媒体查询断点内。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过（本次只改 CSS，不影响类型检查，此处仅按流程复核）。
- `cd frontend; npm run build`（`tsc -b && vite build`）：通过。
- `cd frontend; npm run dev -- --host 0.0.0.0` 启动本地开发服务器（本机 5173 端口占用时会自动切到 5174），局域网地址例如 `http://192.168.0.5:5174/`。
- **未做的验证**：没有连接真实手机在 5174 端口重新打开页面确认横条问题已消失——用户提供的截图是修复前的状态，本次改动之后需要用户或下一位维护者重新在真机上刷新页面确认：390×844/393×852/430×932 几个尺寸下标题不再压住鸡形图标、卡片不再横向撑爆、软键盘弹出后登录按钮可以通过滚动看到。也没有验证 `booting`（不带 `--image-login`，用的是仓库里更早的非 image2 版 `.sd-auth-card`）这个一闪而过的中间态在手机端的表现，理论上不受本次改动影响（本次只碰了 `.sd-auth-shell--image-login` 相关选择器），但没有专门截图确认。

## 下一步 M0/M1 移动端 Shell 适配注意事项

- 这次修复只碰登录/初始化页（`App.tsx` 里 `view!=='stardew'` 的分支），和 `MOBILE-SHELL-M0-1` 的 `StardewMobileShell`（`view==='stardew'` 之后的分支）是完全独立的两套代码，样式作用域也不重叠（`sd-auth-*` vs `sd-mshell-*`）。下一位维护者不要把两者混为一谈：登录页永远走 `App.tsx` 顶层的 `sd-auth-shell` 家族样式，和 `useMediaQuery('(max-width:768px)')` 分流到 `StardewMobileShell` 的桌面/移动端 Shell 选择完全是两件事——即使桌面浏览器把窗口缩到 768px 以下，登录页此时用的也是本次新增的 `.sd-auth-shell--image-login` 移动端覆盖，和 Stardew 主面板用不用 `StardewMobileShell` 无关。
- 这次刻意没有触碰 `.sd-auth-shell--image-login` 在 `>768px` 桌面宽度下的任何规则，也没有删除 `@media (max-width:700px)` 那段旧手机补丁（虽然它现在在 `<=768px` 范围内已经被新规则的"源码顺序更靠后"盖过，实际不再生效，但按仓库"不删除无关既有代码，先提及"的约定保留在原地）。如果以后要彻底清理，需要注意 `@media (max-width:900px)` 那条（针对**非 image2** 版 `.sd-auth-shell` 的居中处理）目前依然在 `booting` 中间态下有效，不能一起删掉。
- 这套"桌面用绝对坐标叠在原型大图上、移动端整体放弃坐标改真实文档流"的修复思路，如果以后登录页原型图重新设计（比如真的要出一版专门为竖屏手机做的 image2 素材），可以直接把这段 `@media (max-width:768px)` 覆盖整体替换成"引用新素材 + 少量坐标微调"，不需要伤筋动骨；但只要还在复用桌面这张 16:9 原型图，就不建议再回头尝试"给移动端也算一套百分比坐标"，本次踩坑已经证明这条路对任意宽高比手机都不稳，正常文档流 + 复用棕框/羊皮纸素材是更稳的方向。
- 输入框 `font-size:16px` 防自动缩放、`env(safe-area-inset-*)` 安全区这两个技巧和 `FE-MOBILE-FIXES-1`（Stardew 主面板的移动端修复）用的是同一套思路，如果后续 `StardewMobileShell` 在 M1 长出真实表单（比如设置页的登录相关表单），可以直接复用这两个技巧，不需要重新发明。

# PLAYERS-BAN-1 封禁玩家 + 玩家行操作按钮精简（同日追加）

> 后续结论（2026-07-12）：用户已人工验证封禁记录在服务器容器重启后丢失；前端确认文案已从“可能失效”改为“重启后会丢失，需要重新操作”。

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

# MOBILE-HOME-M2-1 移动端总览页 M2（同日追加）

## 背景

`MOBILE-SHELL-M0-1` 只做了移动端占位壳（顶部品牌/状态、羊皮纸占位卡、5 个静态 Tab）。这次要求把“总览”Tab 换成真正可用的手机应急运维首页：看服务器是否运行、看在线人数、复制邀请码/局域网地址、启停重启服务器、批准待认证玩家。明确要求：不是桌面总览页缩小版，不改后端 API，不改桌面 `OverviewPage` 行为和布局。

## 改了什么

- 新增目录 `frontend/src/games/stardew/mobile/`，放 `MobileHomePage.tsx` + `MobileHomePage.css`（为后续 M3 移动端页面预留同级位置，不是把所有移动端代码都堆进 `StardewMobileShell.tsx`）。
- `StardewMobileShell.tsx`：
  - 新增 `user: CurrentUser` prop（M0 遗留的“不接收 user，没有登出入口”限制，这次先把 `user` 补上，登出入口仍留给后续“更多”Tab）。
  - `activeTab === 'overview'` 时渲染 `<MobileHomePage user={user} instanceState={dashboardData.instanceState} dashboardData={dashboardData} />`；其余四个 Tab（控制/玩家/任务/更多）保持 M0 的“移动端面板建设中”占位卡不变。
  - `App.tsx`：`<StardewMobileShell user={currentUser} />`（`view==='stardew'` 分支已确保 `currentUser` 非空）。
  - `frontend/src/qa-layout-main.tsx`：同步传 `user={mockUser}` 给 `StardewMobileShell`；补了 `[/\/password-status$/, passwordStatus]` mock 路由（`enabled:true, passwordBridgeAvailable:true`）和一名 `status:'online', isAuthenticated:false` 的玩家 `PendingGuest`，方便在 `qa-layout.html?shell=mobile` 下看到待认证卡片的真实列表态而不是空态。
- `MobileHomePage.tsx` 按单列卡片流渲染四张卡片，`Pick<StardewPageProps, 'user'|'instanceState'|'dashboardData'>` 作为 props（复用现有类型，没有新定义一套 props 类型）：
  1. **状态摘要卡**：存档名取 `dashboardData.saves?.activeSaveName`（空则显示“暂无存档”）；服务器状态本地写了 `serverStatusText()`/`serverStatusDotClass()`，逻辑和 `StardewPanel.tsx` 里模块级 `topbarStatusText`/`topbarStatusDotClassName` 一致（区分运行中/已停止/启动中/停止中/异常），但没有导出复用那两个函数——它们是 `StardewPanel.tsx` 内部私有函数不对外导出，按仓库“各页面自带小工具函数”的既有风格在本文件内重写了一份，而不是新建共享 helper 模块；在线玩家取 `players.onlineCount/maxPlayers`，逻辑抄的是 `OverviewPage.tsx` 的 `playerSummary`；版本取 `versionInfo.version`。四项都有兜底文案（“—”/“暂无存档”/“识别中”），不会渲染 `undefined`/`null`。
  2. **邀请信息卡**：**没有**直接复用 `InviteCodeCard.tsx` 组件——它的 JSX 用的类名（`sd-players-invite-row`/`sd-btn-green` 里那套桌面尺寸/`sd-players-invite-code` 等）定义在 `StardewPanel.css` 里，而 `StardewPanel.css` 只在挂载 `StardewPanel.tsx` 时才被 Vite 加载（组件级 CSS import），移动端走 `StardewMobileShell` 完全不会加载这个文件，直接复用组件会导致大部分样式丢失、退化成无样式纯文本。按用户在需求里给的口径（“如果直接复用组件导致手机端布局挤压，可以拆出移动端轻量展示组件，但不要复制 API 请求逻辑”），改成在 `MobileHomePage.tsx` 内部写了 `inviteInfo()`/`hostInfo()` 两个纯函数，只读 `dashboardData.inviteCode`/`steamAuthLoggedIn`/`inviteCodeError`/`publicIP`/`publicIPError`/`publicIPRefreshing`（和 `InviteCodeCard` 判断优先级完全一致：有码显示码、需要 Steam 授权显示提示、running/starting 显示获取中/失败、否则显示“服务器未运行”），配合本地 `copyText()`（和 `InviteCodeCard.tsx` 里的实现逐字一致，处理非安全上下文下 `navigator.clipboard` 不可用的降级）。**没有**给移动端加“登录授权”按钮和“刷新”按钮——需求只要“复制邀请码/局域网地址”，登录授权流程不在这次范围内，跳过是按最小实现处理，不是遗漏。
  3. **快捷控制卡**：`handleStart`/`handleStop`/`handleRestart` 直接调 `startInstance`/`stopInstance`/`restartInstance`（`api.ts` 现有函数），状态判断（`hasActiveLifecycleJob`/`activeLifecycleIsStopping`/`waitingForStartup`/`waitingForStop`/`canStart`）抄的是 `OverviewPage.tsx` 的 `renderLifecycleButtons()` 同一套变量命名和计算方式。和桌面端“按状态切换只显示相关按钮”不同，移动端**固定渲染三个按钮**（启动/停止/重启都常驻），用 `disabled`+`title` 表达当前不可操作——这是有意的设计取舍：手机卡片布局更适合固定三个大按钮，靠 disabled 态而不是按钮增减来表达状态，用户截图/需求里也是这么描述的（“如果状态不可用…按钮禁用，要有明确 title”）。停止/重启点击后走 `confirm` 弹窗二次确认（和桌面 `OverviewPage` 一样的安全考虑），启动不需要确认（和桌面一致）。按钮和弹窗按钮都叠加 `min-height:44px`（`MobileHomePage.css` 的 `.sd-mhome-lifecycle-btn`/`.sd-mhome-confirm-btn` 类），利用 CSS `min-height` 会把 `.sd-btn-start`/`.sd-btn-stop`/`.sd-btn-restart`（`stardew-theme.css` 里定义为固定 `height:40px`）撑到至少 44px 而不产生样式冲突，**没有修改 `stardew-theme.css` 里这三个按钮类本身的高度**，桌面端按钮尺寸不受影响。
  4. **待认证玩家批准卡**：`useEffect` 在 `isRunning` 为真时调用 `getInstancePasswordStatus()`（`api.ts` 现有函数），和 `PlayersPage.tsx` 里的同名 `useEffect` 逐字一致（“页面自己按需拉取，不进 `useStardewDashboardData` 全局轮询”这个既有决定延续到移动端）；`pendingAuthPlayers` 过滤条件（`status==='online' && isAuthenticated===false`）也和 `PlayersPage.tsx` 一致。批准动作复用 `approvePlayerAuth()`，点击后走 `confirm` 弹窗（文案和 `PlayersPage.tsx` 的批准确认弹窗一致），成功后 `dashboardData.refreshPlayers()`。卡片对“未运行/识别中/未开启密码认证/桥接不可用/暂无待认证/正常列表”五种状态都有独立文案，不会出现空白或 `undefined`。
- 三个确认场景（停止/重启/批准）合并成一个 `confirm: ConfirmState | null`（联合类型 `{kind:'stop'}|{kind:'restart'}|{kind:'approve',target}`）+ 一个共享的弹窗 JSX 块，而不是像桌面端 `OverviewPage`/`PlayersPage` 分别维护独立的 `confirmAction`/`approveConfirmTarget` 状态各自渲染一个弹窗——移动端三种确认的视觉结构完全一样（标题+说明+取消/确认两个按钮），合并成一个联合类型判断分支能省掉两份几乎相同的弹窗 JSX，不算过度抽象。
- `MobileHomePage.css`（class 前缀 `sd-mhome-`）只定义卡片壳的间距/网格/长文本换行等布局细节；卡片本体用全局 `.sd-panel`（`stardew-theme.css`），按钮用全局 `.sd-btn-start/-stop/-restart/-green/-tan/-delete`，提示条用全局 `.sd-notice--ok/-warn/-error/-info`，徽章用全局 `.sd-tag`/`.sd-tag-gold`——这些类在 `main.tsx` 顶层 `import './games/stardew/stardew-theme.css'` 全局加载，和只在 `StardewPanel.tsx` 挂载时才加载的 `StardewPanel.css` 是两个完全不同的加载时机，这也是为什么邀请信息卡不能直接借 `InviteCodeCard` 的用法但可以放心借 `.sd-btn-green` 等基础按钮类。未新增图片素材，四张卡片头部图标复用 `OverviewPage.tsx`/生命周期按钮已有的 PNG（`icon_top_summary_save.png`、`icon_nav_server_rack_image2.png`、`icon_button_play/stop/restart.png`、`icon_nav_players_avatar_image2.png`）。

## 影响文件

- 新增：`frontend/src/games/stardew/mobile/MobileHomePage.tsx`、`frontend/src/games/stardew/mobile/MobileHomePage.css`。
- 修改：`frontend/src/games/stardew/StardewMobileShell.tsx`、`frontend/src/App.tsx`、`frontend/src/qa-layout-main.tsx`。
- 未改：任何后端文件、`frontend/src/games/stardew/pages/OverviewPage.tsx`、`StardewPanel.tsx`、`StardewPanel.css`、`InviteCodeCard.tsx`、`useStardewDashboardData.ts` 内部实现、`api.ts`（只是调用了其中已有的导出函数，没有新增/修改任何函数签名）。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过。
- `cd frontend; npm run build`（`tsc -b && vite build`）：通过。

**未做的验证**：本次环境没有可用的浏览器自动化/截图工具，没有做真实浏览器缩放或真机截图确认 390×844/393×852/430×932 下的实际渲染效果，也没有连一个真实运行中的实例走一遍“启动→查看在线人数→复制邀请码→停止→批准待认证玩家”完整链路。建议下一位维护者：
1. `cd frontend; npm run dev -- --host 0.0.0.0`，手机访问局域网地址（如 `http://192.168.0.5:5173/`）登录后确认总览 Tab 四张卡片正常显示、无横向滚动。
2. 或用 `frontend/qa-layout.html?shell=mobile&state=running`（已补好 mock 数据，包含一名待认证玩家）在浏览器里直接缩放窗口到上述三个宽度确认布局。
3. 真实连一个运行中实例，验证复制按钮的剪贴板反馈、启停重启后 `dashboardData` 刷新是否及时反映到卡片、非管理员账号登录后四个操作按钮是否正确禁用并显示 title 提示。

## 下一步 M3 注意事项

- “控制”Tab 目前仍是 M0 占位卡，M3 大概率要把桌面 `ServerControlPage.tsx` 的能力（密码设置、计划重启、触发节日活动、Joja 路线、小屋策略等）挑一个精简子集搬到移动端。复用点参考本轮经验：**不要**直接复用桌面页面组件（它们的样式都挂在 `StardewPanel.css`，移动端不加载），但**可以**放心复用桌面页面里已经抽出来的纯函数/状态判断逻辑（比如 `renderLifecycleButtons()` 的变量命名）和 `api.ts`/`useStardewDashboardData()` 里的现有函数；只有跨页面共享的视觉基础件（按钮/面板/提示条/徽章）才应该去 `stardew-theme.css` 找全局类，页面级专属结构（表格、复杂弹窗）还是照抄逻辑后用移动端自己的 CSS 类重排布局。
- 本轮的“三个确认场景合并成一个联合类型 `confirm` state”的模式如果在“控制”Tab 里还会出现类似的多重确认弹窗，可以直接照搬这个模式，不需要重新发明。
- 待认证玩家批准卡目前只在“总览”Tab 出现；如果 M3“玩家”Tab 也要接入玩家管理能力，需要决定“待认证批准”这个功能到底该保留在总览卡里、搬到玩家 Tab、还是两处都留（两处都读同一份 `dashboardData.players`，不会有数据不一致问题，但要避免用户对“在哪里操作”感到困惑）。
- `StardewMobileShell` 现在有 `user` 但仍没有 `onLogout`；“更多” Tab 如果要放登出入口，需要在 `App.tsx` 分流处再传一个 `onLogout={logout}` 并在 `StardewMobileShellProps` 里补上，这次没有做（不在“总览”Tab 的范围内）。

## 追加调整（同日，用户截图反馈后）

用户看了首版效果后陆续提了几点反馈，已在 `MobileHomePage.tsx`/`MobileHomePage.css` 里调整：

1. **邀请码/局域网地址视觉要和 PC 端一致，码值本身要有框**：首版把码值渲染成一个纯文本 `<span>`，没有边框，视觉上比桌面 `InviteCodeCard` 的 `.sd-players-invite-code` 弱很多。桌面那个类定义在 `StardewPanel.css`（双边框 + 羊皮纸渐变 + 绿色等宽字 + 角落绿色小圆点），移动端不会加载这个文件，所以没法直接引用类名，改成在 `MobileHomePage.css` 里按同一套视觉参数（`border:3px double`、`linear-gradient(180deg,#fff7cf,#ffe7aa)`、`color:#1c7c29`、等宽字体）重新画了一份 `.sd-mhome-invite-box`（有值时）/`.sd-mhome-invite-box--muted`（占位文案时的虚线框，对应桌面 `.sd-players-invite-empty/-loading/-error`）。
2. **按钮和实际码要同一行，390px 放得下**：第一版把标签/码框拆成了“标签+按钮一行，码框单独一行”两行布局，用户反馈这个放不下的判断是错的、一行放得下。改回和桌面一致的单行布局：`.sd-mhome-invite-row` 里标签（固定 54px）+ 码框（`flex:1; min-width:0`）+ 复制按钮（`flex-shrink:0`）三者同一行，码框字号收到 `clamp(13px,4vw,17px)`（桌面是 `clamp(22px,3.4cqi,34px)`，桌面码框列宽够宽；390px 单行放不下同等字号，缩小字号后配合 `word-break:break-all` 保证一行能容纳常见邀请码长度，超长时再允许换行而不是横向溢出）。
3. **待认证玩家卡挪到快捷控制卡上面**：单纯调整 JSX 里两个 `<section>` 的先后顺序，卡片内部逻辑、状态判断、API 调用都没有改。当前四卡顺序变成：状态摘要 → 邀请信息 → 待认证玩家批准 → 快捷控制。
4. **两个复制按钮颜色可以不同，但尺寸必须一样**：邀请码复制按钮用 `.sd-btn-green`、局域网地址复制按钮用 `.sd-btn-tan`（配色故意区分，和桌面 `InviteCodeCard.tsx` 的用色一致），但这两个基础类各自默认 `min-width` 不同（58px vs 50px），且按钮文案在“复制”/“已复制”之间切换时字数不同，仅用 `min-width` 会让两个按钮看起来一大一小或点击后跳动。`.sd-mhome-copy-btn` 改成固定 `width:64px`（不再是 `min-width`），两个按钮无论文案长度还是颜色差异，渲染尺寸都强制一致。
5. **版本改成游戏内日期**：状态摘要卡的“版本”字段（`versionInfo.version`）换成“游戏日期”，取 `dashboardData.saves?.saves.find(save => save.isActive || save.name === activeSaveName)` 找到当前激活存档后用 `saveDate()` 格式化——`SEASON_ZH`/`saveDate()` 直接照抄 `ServerSummaryCard.tsx` 里同名实现（“第 X 年春季 Y 日”），没有另外发明格式。`versionInfo` 从组件里彻底移除，不再被引用。
6. **邀请码/局域网地址两个复制按钮颜色互换**：邀请码复制按钮从 `.sd-btn-green` 改成 `.sd-btn-tan`，局域网地址复制按钮从 `.sd-btn-tan` 改成 `.sd-btn-green`，纯粹是配色对调，尺寸逻辑（第 4 点的 `.sd-mhome-copy-btn` 固定宽度）不受影响。
7. **快捷控制卡的按钮显隐逻辑要复用桌面总览页**：早期版本固定常驻渲染启动/停止/重启三个按钮，靠 `disabled` 表达“当前不可操作”，这是这次任务开始时的主动设计取舍（详见前面 M2 首版记录）。用户反馈要求改成和桌面 `OverviewPage.tsx` 的 `renderLifecycleButtons()` 完全一致的分支逻辑：**没运行时只显示启动按钮，运行中才切换成停止+重启**，不是三个按钮一直都在。改动点：
   - 新增组件内函数 `renderLifecycleButtons()`，把原来铺平写在 JSX 里的三个 `<button>` 收进一个按状态分支返回不同 JSX 的函数，分支顺序和桌面版逐条对应：`save_required` → 禁用的“启动”按钮 + title 提示；`waitingForStartup` → 单个“启动中…”spinner 按钮；`waitingForStop` → 单个“停止中…”spinner 按钮；`canStart`（`ready_to_start`/`stopped`/`game_installed`）→ 单个“启动”按钮；`state==='running'` → “停止”+“重启”两个按钮一起出现；`state==='error'` → 不渲染按钮，改成一条 `sd-notice--error` 提示“服务器异常，请到电脑端查看诊断信息”（移动端目前没有诊断 Tab，桌面版这里是跳转诊断页的按钮，移动端暂时只提示去 PC 端看，不是遗漏）。
   - 原来铺平计算的 `startDisabled`/`startTitle`/`stopRestartDisabled`/`stopTitle`/`restartTitle` 五个变量全部删除，`disabled`/`title` 逻辑收回到各分支内联判断（`actionBusy || !isAdmin`，和桌面 `OverviewPage.tsx` 一模一样），不再需要单独处理“三个按钮同时存在时各自的禁用文案”这种只有旧版才有的复杂度。
   - `.sd-mhome-lifecycle-list`/`.sd-mhome-lifecycle-btn` 这两个 CSS 类未改，单按钮或双按钮场景都能正常撑满/纵向排列。

验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。同样没有做真实浏览器/真机视觉复核，建议和上面“未做的验证”一起补，第 7 点尤其需要真实切换 running/stopped/starting/stopping/error 几种状态确认按钮显隐符合预期。

# MOBILE-CONTROL-M3-1 移动端控制页 M3（同日追加）

## 背景

`MOBILE-HOME-M2-1` 只做了移动端“总览”Tab。这次要求把“控制”Tab 从占位卡换成真正可用的手机服务器控制页：展示桌面 `ServerControlPage.tsx` 的“全服消息”卡片和“快捷操作”卡片（去掉手动备份和 VNC 显示相关按钮），不新增后端接口，不改鉴权逻辑，不影响桌面端；生命周期启停不在这次范围内（`MobileHomePage` 的“快捷控制”卡已经提供，两个 Tab 读同一份 `dashboardData`，避免重复维护同一组状态判断）。

## 改了什么

- 新增 `frontend/src/games/stardew/mobile/MobileControlPage.tsx` + `MobileControlPage.css`（和 `MobileHomePage` 同级目录，class 前缀 `sd-mctrl-`）：
  - 顶部一条紧凑状态条（不是完整卡片）：状态点 + 文案 + 当前存档名，复用和 `MobileHomePage.tsx` 里逐字一致的私有函数 `serverStatusText`/`serverStatusDotClass`（按仓库“各页面自带小工具函数”的既有风格各自实现一份）。
  - **全服消息卡**：输入框 + 发送按钮，状态/逻辑（`sayMessage`/`sayBusy`/`sayResult`/`sayError`/`handleSay`）与桌面 `ServerControlPage.tsx` 的 `handleSay`/`sendSay` 完全同构；未运行时展示 `sd-notice--info` 提示，不渲染输入框。
  - **快捷操作卡**：单列纵向按钮列表（不是桌面的多列网格，符合“不要照搬桌面大表格/网格布局”的要求），5 个按钮，状态和处理函数都是从 `ServerControlPage.tsx` 逐条搬过来的（状态命名、`disabled`/`title` 门控逻辑完全对齐，只是去掉了 `quickBackupBusy`/`vncPort`/`vncRenderingEnabled` 等手动备份和 VNC 相关的状态与按钮）：
    1. 计划重启（`scheduleOpen`/`scheduleDraft`/...，`getRestartSchedule`/`updateRestartSchedule`）。
    2. 服务器密码设置（`passwordOpen`/`passwordDraft`/...，`getInstanceServerPassword`/`updateInstanceServerPassword`/`getInstancePasswordStatus`）。
    3. 小屋与联机高级设置（`runtimeSettingsOpen`/...，`getInstanceServerRuntimeSettings`/`updateInstanceServerRuntimeSettings`）。
    4. 触发节日活动（`festivalBusy`/...，`triggerFestivalEvent`，无二次确认，和桌面一致）。
    5. 永久启用 Joja 路线（`jojaOpen`/`jojaConfirmInput`/...，`enableJojaRoute`，逐字输入 `IRREVERSIBLY_ENABLE_JOJA_RUN` 才能点亮红色确认按钮，弹窗结构和桌面版一致）。
  - 4 个表单类操作（计划重启/密码设置/小屋高级设置/Joja）各自弹出一个全屏 `.sd-mctrl-dialog-overlay` + `.sd-mctrl-dialog`（`.sd-panel` 羊皮纸壳 + `max-height:88vh; overflow-y:auto`，防止计划重启这类字段较多的表单在小屏下把弹窗撑出视口）。**没有**像 `MobileHomePage.tsx` 那样把多个弹窗合并成一个联合类型 `confirm` state——那个模式只适用于视觉结构完全相同的简单“标题+说明+取消/确认”弹窗，这里 4 个弹窗内部表单结构（复选框组/下拉选择/文本输入）互不相同，合并没有意义，继续沿用桌面 `ServerControlPage.tsx` 每个弹窗一组独立 state 的写法。
  - **没有**直接复用 `ServerControlPage.tsx` 组件或它用到的桌面 CSS 类（`sd-confirm-overlay`/`sd-confirm-dialog`/`sd-schedule-field`/`sd-schedule-check`/`sd-srv-section`/`sd-server-quick-grid`/`sd-server-message-row` 等全部只在 `StardewPanel.css` 里定义，只有挂载 `StardewPanel.tsx` 时才会被 Vite 加载，移动端走 `StardewMobileShell` 不会加载这个文件）。只复用了桌面页面里的**状态判断逻辑和 `api.ts`/`types.ts` 里已有的函数与类型**（`RestartSchedule`/`InstancePasswordStatus`/`ServerRuntimeSettings`、`getRestartSchedule`/`updateRestartSchedule`/`getInstanceServerPassword`/`updateInstanceServerPassword`/`getInstancePasswordStatus`/`getInstanceServerRuntimeSettings`/`updateInstanceServerRuntimeSettings`/`triggerFestivalEvent`/`enableJojaRoute`/`sendSay`，没有新增或修改任何 `api.ts` 函数签名），弹窗/按钮/提示条在 `MobileControlPage.css` 里重新按移动端布局排布。视觉基础件继续用全局 `stardew-theme.css` 的 `.sd-panel`/`.sd-input`/`.sd-btn-*`/`.sd-notice--*`（`main.tsx` 全局加载，移动端可以放心用）。
  - 表单控件触控热区：`.sd-mctrl-field .sd-input { min-height: 44px }` 覆盖全局 `.sd-input` 默认的 26px 高度（仅在 `.sd-mctrl-field` 作用域内覆盖，不影响其它页面的 `.sd-input`）；所有按钮（发送/操作行/弹窗内的取消保存）都通过 `min-height: 44px` 满足触控热区要求。
  - 未新增图片素材：按钮图标复用桌面 `ServerControlPage.tsx` 里 `SERVER_PAGE_ICONS` 已经在用的几张 PNG（`icon_nav_diagnostics_monitor_image2.png`/`icon_nav_settings_gear_image2.png`/`icon_right_rail_in_progress_clock_image2.png`/`icon_nav_tasks_scroll_image2.png`/`icon_players_action_permission_image2.png`）。
  - `StardewMobileShell.tsx`：`activeTab === 'server'` 时渲染 `<MobileControlPage user={user} instanceState={dashboardData.instanceState} dashboardData={dashboardData} />`，其余“玩家”“更多”两个 Tab 仍是 M0 占位卡。
  - `qa-layout-main.tsx`：新增 `/config/server-password`（`{ serverPassword: '' }`）和 `/config/server-runtime-settings`（`{ cabinStrategy: 'CabinStack', existingCabinBehavior: 'KeepExisting', networkBroadcastPeriod: 1 }`）两个 GET 路由 mock——此前这两个接口没有专门的 mock，落到默认的 `jsonRes({}, 200)` 兜底，打开对应弹窗时会把 `undefined` 塞进受控输入框（React "changing controlled input to uncontrolled" 警告），也看不出真实的表单初始值；新增 mock 后这两个弹窗在 `qa-layout.html?shell=mobile` 下可以看到正常初始值。POST 类操作（`festival/event`/`joja/enable`）沿用仓库既有做法不单独加 mock，落到默认 `jsonRes({}, 200)` 兜底，`result.output?.trim()` 对 `undefined` 有兜底文案，不会报错。
- 顺手清理：**全服消息卡片里过时的提示文案**"通过 SMAPI say 命令发送全服公告。注：该命令当前版本可能返回‘命令不支持’。"——用户确认 SMAPI say 命令现在已经正常支持，这句提示不再准确，PC 端 `ServerControlPage.tsx` 和这次新增的移动端 `MobileControlPage.tsx` 一起删除，只删文案本身，`sd-srv-hint`/`sd-notice--info` 外层结构如果因此变成空壳就顺带删掉（PC 端这句本来单独一个 `<div className="sd-srv-hint">`，移动端本来单独一个 `<div className="sd-notice sd-notice--info">`，两处整个 div 一起删）。
- 顺手调整：移动端“发送”按钮从 `sd-btn-green` 改成 `sd-btn-restart`（棕色，和“重启”按钮同一个视觉样式），用户明确要求“手机端的按钮改为重启按钮的样式颜色”；PC 端发送按钮保持 `sd-btn-green` 不变，两端故意保留颜色差异，不是要统一成一个颜色。
- **快捷操作 4 个非危险按钮最终改成了 PC 端同款“羊皮纸卡片”样式，不是按钮贴图色**：用户看截图反馈快捷操作按钮不该是黄色像素按钮贴图，去读 `StardewPanel.css` 源码后发现 PC 端 `.sd-server-quick .sd-server-quick-grid > button` 这条文件尾部最终覆盖规则其实完全不用 `background-image`，是纯 CSS 画的边框+渐变+内阴影卡片（`border:2px solid rgba(160,108,44,.58)`、`background:linear-gradient(...),#f4d79a`、`box-shadow:inset...`、`color:#3a250e`），且这条规则对网格里所有按钮一视同仁生效，不区分原本挂的是 `sd-btn-tan` 还是 `sd-btn-delete`。移动端新增修饰类 `.sd-mctrl-action-btn--card` 照抄同一套视觉参数，只加在计划重启/密码设置/小屋高级设置/触发节日活动这 4 个按钮上；“永久启用 Joja 路线”按钮按用户明确要求（“joja按钮样式不动”）继续保留独立的红色 `sd-btn-delete` 像素贴图，不跟着改成金棕色卡片（尽管 PC 端实际上 Joja 也是同一套金棕色卡片，这是移动端刻意做的差异化危险提示，不是照抄疏漏）。
- **踩坑记录，务必读**：第一版把 `.sd-mctrl-action-btn--card` 写成单类选择器，构建后打包 CSS 里核对发现完全没生效——`stardew-theme.css` 里 `.sd-btn-tan` 的 `background-image` 规则在 Vite 打包后的最终 CSS 顺序里实际排在这个组件样式**之后**（和"先加载的全局样式排在前面、后加载的组件样式排在后面覆盖它"的直觉相反，Vite/Rollup 对 CSS chunk 的拼接顺序不完全遵循 import 语句的源码顺序）。两条规则选择器优先级相同（都是单类），层叠顺序里更靠后的 `.sd-btn-tan` 赢，我加的卡片背景被悄悄覆盖回了原来拉伸的黄色像素贴图——这正是用户反馈"改了但看起来还是一样"的根因，不是视觉参数抄错。修复方式是把选择器改成两个类叠加的复合选择器 `.sd-mctrl-action-btn.sd-mctrl-action-btn--card`（优先级 (0,2,0)，无论打包顺序如何都稳赢单类的 `.sd-btn-tan` (0,1,0)），并且**用 `node -e "..." ` 直接读构建产物 `dist/assets/index-*.css` 里两条规则的字节偏移位置做了实际验证，不是靠猜**。同一类问题排查后顺手修了这个页面里另外两处同构风险：`.sd-mctrl-say-input`/`.sd-mctrl-say-btn` 的 `min-width`（原来是单类，改成 `.sd-input.sd-mctrl-say-input`/`.sd-btn-restart.sd-mctrl-say-btn`）、`.sd-mctrl-dialog-btn` 的 `min-width`（原来是单类，改成对 `sd-btn-tan`/`sd-btn-green`/`sd-btn-delete` 三种颜色分别写复合选择器）。**`min-height` 不受影响，没有改**：基类设的是 `height`，我设的是 `min-height`，这是两个不同属性，浏览器盒模型固定取两者较大值生效，不受层叠顺序影响，不属于这个坑。
- **遗留风险，未修**：`MobileHomePage.css` 里 `MOBILE-HOME-M2-1` 那批的 `.sd-mhome-copy-btn`（覆盖 `.sd-btn-green`/`.sd-btn-tan` 的 `padding`/`width`）用的也是单类选择器，很可能有同样的层叠顺序风险（`padding:0` 大概率被基类 `padding:0 10px` 覆盖回去，`width:64px` 因为基类没设 `width` 只设 `min-width` 应该不受影响还是安全的）。这次没有动这个文件——不在“控制”Tab 的任务范围内，而且没有找该页面维护者确认，怕改了引入新的视觉差异。下一位维护者如果要修，参考这次的验证方法：改完直接读 `npm run build` 产物 CSS 里两条规则的实际先后顺序确认，不要只凭直觉判断。

## 影响文件

- 新增：`frontend/src/games/stardew/mobile/MobileControlPage.tsx`、`frontend/src/games/stardew/mobile/MobileControlPage.css`。
- 修改：`frontend/src/games/stardew/StardewMobileShell.tsx`、`frontend/src/qa-layout-main.tsx`、`frontend/src/games/stardew/pages/ServerControlPage.tsx`（仅删除过时提示文案，其余逻辑未改）。
- 未改：任何后端文件、`StardewPanel.tsx`、`StardewPanel.css`（除上面提到的一处文案删除外）、`MobileHomePage.tsx`/`.css`、`api.ts`（只调用了其中已有的导出函数，未新增/修改任何函数签名）、`useStardewDashboardData.ts` 内部实现、任何登录/权限/鉴权逻辑。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过。
- `cd frontend; npm run build`（`tsc -b && vite build`）：通过。

**未做的验证**：本次环境没有可用的浏览器自动化/截图工具，没有做真实浏览器缩放或真机截图确认 390×844/393×852/430×932 下的实际渲染效果（尤其是计划重启弹窗字段较多，需要确认 `max-height:88vh; overflow-y:auto` 在实际小屏设备上纵向滚动顺畅、无横向溢出），也没有连一个真实运行中的实例走一遍“发送全服消息 → 设置计划重启 → 设置服务器密码 → 调整小屋策略 → 触发节日活动 → 确认启用 Joja 路线”完整链路。建议下一位维护者：
1. `cd frontend; npm run dev -- --host 0.0.0.0`，手机访问局域网地址（如 `http://192.168.0.5:5173/`）登录后确认“控制”Tab 两张卡片、5 个快捷操作按钮和 4 个弹窗正常显示、无横向滚动、弹窗可纵向滚动到底部按钮。
2. 或用 `frontend/qa-layout.html?shell=mobile&state=running` 在浏览器里直接缩放窗口到上述三个宽度确认布局，重点看计划重启弹窗（字段最多）和 Joja 确认弹窗（危险操作红色按钮）。
3. 真实连一个运行中实例，验证全服消息发送后游戏内是否收到广播、5 个快捷操作按钮的实际效果、非管理员账号登录后按钮是否正确禁用并显示 title 提示。

## 下一步注意事项

- “控制”Tab 目前不包含控制台命令执行（桌面 `ServerControlPage.tsx` 的“控制台命令”卡片）——这次任务范围明确只要“全服消息 + 快捷操作（去手动备份/VNC）”，控制台命令是单独一块桌面能力，没有被要求搬到移动端，如果以后需要，可以参考这次的搬迁模式（复制状态判断逻辑 + 新写移动端 CSS，不直接复用桌面组件）。
- “玩家”Tab 仍是占位卡；`MobileHomePage.tsx` 的“待认证玩家批准”卡和这次“控制”Tab 是两个独立入口，都读同一份 `dashboardData.players`，不会数据不一致，但如果以后“玩家”Tab 也要接入玩家管理能力，需要注意避免用户对“在哪里操作”感到困惑（这一点在 `MOBILE-HOME-M2-1` 的注意事项里已经提过一次，这里不是新问题）。
- 密码设置弹窗里的“密码保护状态（来自 JunimoServer）”块依赖 `isRunning` 才能读取，和桌面版行为一致；如果管理员在服务器未运行时打开这个弹窗，会看到“服务器未运行，无法读取密码保护状态”文案，这是预期行为，不是 bug。
- `MobileControlPage.tsx` 的 4 个表单弹窗状态命名和桌面 `ServerControlPage.tsx` 逐条对应（`scheduleXxx`/`passwordXxx`/`runtimeSettingsXxx`/`jojaXxx`），如果桌面这几块以后新增字段或改校验逻辑，需要记得同步搬一份到这里，两边目前没有共享状态或共享 hook，是两份独立实现。

# MOBILE-PLAYERS-M4-1 移动端玩家页 M4（同日追加）

## 背景

`MOBILE-CONTROL-M3-1` 之后“玩家”Tab 仍是 M0 占位卡。这次要求把它换成真正可用的手机端玩家页：查看在线玩家名单、状态、最近活动、角色/主机信息，并处理待授权玩家（同意/拒绝）。明确要求：不新增后端接口，不改鉴权逻辑，不影响桌面端 `PlayersPage.tsx`。

本节记录了两轮实现：**首版**（顶部统计卡 + 独立待授权玩家卡 + 只读在线玩家列表）→ 用户反馈"踢出和封禁的按钮呢"后**补了踢出/封禁**→ 用户又反馈"待授权玩家卡片删除掉，最上面的玩家卡片也删除掉，位置信息放在线玩家卡片的左下角，刷新按钮放在在线玩家的右上角"后**大幅简化成当前的单卡结构**。下面直接描述最终态，中间态不再展开（历史过程见本文件 git 历史）。

## 改了什么（最终态）

- 新增 `frontend/src/games/stardew/mobile/MobilePlayersPage.tsx` + `MobilePlayersPage.css`（和 `MobileHomePage`/`MobileControlPage` 同级目录，class 前缀 `sd-mplay-`）：
  - 页面只有**一张卡片**——“在线玩家”。**没有**顶部统计卡（在线人数/待授权数量），**没有**独立的“待授权玩家”卡（同意/拒绝密码认证），这两块用户明确要求删除；待认证玩家的同意/拒绝能力继续保留在“总览”Tab 的 `MobileHomePage.tsx`（`MOBILE-HOME-M2-1` 已有），这次不重复实现。
  - 卡片头部：左侧标题“在线玩家”，右上角一个“刷新”按钮（`min-height:44px`），点击调用 `dashboardData.refreshPlayers()`。读取失败展示 `dashboardData.playersError`。
  - 玩家列表渲染 `playerRows` 全量，**不是**只筛 `status==='online'`——需求要同时展示在线/离线/等待/未知状态徽章，预先过滤会让离线/未知状态永远不出现，所以这里语义上是“玩家名册”（含历史记录的离线玩家），和桌面 `PlayersPage.tsx` 的“在线玩家”表格同一个语义。
  - 每张卡片自上而下三行：
    1. 姓名（粗体，`overflow:hidden;text-overflow:ellipsis` 防溢出）+ 状态徽章（`sd-tag-green` 在线 / `sd-tag-gold` 等待 / 默认 `sd-tag` 离线或未知，文案：在线/等待/离线/未知）。
    2. 次要信息行：`isHost` 显示“主机”徽章、`player.role` 存在时显示角色徽章、活动文案 `playerActivityText()`（在线显示 `onlineFor` 有值时的原文或“在线中”；离线显示 `lastSeen` 有值时的 `最近活动：${formatDate(lastSeen)}`；都没有显示“—”，**没有**照抄桌面更复杂的 `onlineSinceLabel`/`eventTimeLabel` 时间反推逻辑，现成字段已经满足“如果现有数据里已有”的要求）。
    3. 底部一行 `justify-content:space-between`：左侧位置信息 `playerLocationText()`（取 `locationDisplayName`/`locationName`/`location` 里第一个非空值，有 `tileX`/`tileY` 时附加坐标；都没有时“—”；**没有**翻译成中文地名，桌面 `PlayersPage.tsx` 那份 `LOCATION_ZH` 字典有 200 多行，为这个精简页面整份搬过来不划算，属于按需求最小实现），右侧“踢出”“封禁”两个按钮（`min-height:44px`）。
  - 踢出/封禁直接复用桌面 `kickPlayer()`/`banPlayer()`，未新增接口。`disabled`/`title` 门控条件逐条对齐桌面 `PlayersPage.tsx` 行内图标按钮——踢出要求 `player.status==='online'`，封禁不要求在线（`banTargetPlayers` 的既有设计：离线/从未上线过的玩家也可以提前封禁），两者都排除 `isHost`（无法踢出/封禁主机）和缺少 `uniqueMultiplayerId` 的行。忙碌态用 `kickBusyId`/`banBusyId`（存目标 `uniqueMultiplayerId` 而非单一 boolean），`rosterActionBusy = kickBusyId!==null || banBusyId!==null` 联合锁定其它玩家行的踢出/封禁按钮，避免并发误触。确认弹窗文案逐字复用桌面版（封禁弹窗保留“如果之后重启了服务器容器，这条封禁可能会失效”的提示，不夸大成“永久封禁”）。
  - 空状态：`playerRows.length===0` 时显示“暂无在线玩家” + 一句补充说明，不留白。
  - 未新增图片素材：卡片标题图标复用现有 `icon_nav_players_avatar_image2.png`。视觉基础件（`.sd-panel`/`.sd-tag*`/`.sd-notice--*`/`.sd-btn-delete`/`.sd-btn-tan`）全部走全局 `stardew-theme.css`，没有复用 `StardewPanel.css` 里桌面玩家表格的类名（只在挂载 `StardewPanel` 时才加载，移动端不可用）。
- `StardewMobileShell.tsx`：`activeTab === 'players'` 时渲染 `<MobilePlayersPage user={user} instanceState={dashboardData.instanceState} dashboardData={dashboardData} />`，只剩“更多”Tab 还是 M0 占位卡。
- **没有**修改 `qa-layout-main.tsx`——现有 mock 数据（`players` 数组里已有 `status:'online'`/`'waiting'` 混合、多名带 `locationDisplayName`/`tileX`/`tileY` 的玩家）已经覆盖本次需要的场景，不需要新增 mock 路由。

## 影响文件

- 新增：`frontend/src/games/stardew/mobile/MobilePlayersPage.tsx`、`frontend/src/games/stardew/mobile/MobilePlayersPage.css`。
- 修改：`frontend/src/games/stardew/StardewMobileShell.tsx`（新增 import + 一个 Tab 分支）。
- 未改：任何后端文件、`frontend/src/games/stardew/pages/PlayersPage.tsx`、`StardewPanel.css`、`MobileHomePage.tsx`/`.css`、`MobileControlPage.tsx`/`.css`、`api.ts`（只调用了其中已有的 `kickPlayer`/`banPlayer`，未新增/修改任何函数签名）、`types.ts`、`useStardewDashboardData.ts` 内部实现、`qa-layout-main.tsx`、任何登录/权限/鉴权逻辑。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过。
- `cd frontend; npm run build`（`tsc -b && vite build`）：通过。

**未做的验证**：本次环境没有可用的浏览器自动化/截图工具，没有做真实浏览器缩放或真机截图确认 390×844/393×852/430×932 下的实际渲染效果（尤其是卡片底部“位置信息（左）+ 踢出/封禁按钮（右）”这一行在长地名或双按钮同时出现时是否会挤压换行、换行后视觉是否合理），也没有连一个真实运行中的实例走一遍“查看在线玩家名册 → 踢出/封禁某个玩家 → 列表刷新”完整链路。建议下一位维护者：
1. `cd frontend; npm run dev -- --host 0.0.0.0`，手机访问局域网地址（如 `http://192.168.0.5:5173/`）登录后确认“玩家”Tab 卡片头部刷新按钮、玩家卡片列表正常显示、无横向滚动。
2. 或用 `frontend/qa-layout.html?shell=mobile&state=running` 在浏览器里直接缩放窗口到上述三个宽度确认布局。
3. 真实连一个运行中实例，验证踢出/封禁按钮点击后玩家是否真的被踢出/无法重新加入、非管理员账号登录后按钮是否正确禁用并显示 title 提示。

## 下一步注意事项

- 待认证玩家的同意/拒绝**只**在“总览”Tab 的 `MobileHomePage.tsx` 里，这次“玩家”Tab 删掉了同一功能的重复入口；如果以后用户反馈“玩家”Tab 也需要处理待认证玩家，需要重新加回一张独立卡片，可以参考 `MobileHomePage.tsx` 里 `passwordStatus`/`pendingAuthPlayers`/`approvePlayerAuth` 那一段实现，不需要从零设计。
- `playerLocationText()` 目前只输出英文/原始场景 ID（例如 `Farm`、`FarmHouse1`），没有做中文翻译；如果用户后续反馈需要，可以把桌面 `PlayersPage.tsx` 的 `LOCATION_ZH` 字典和 `translateLocationName()` 抽成共享模块（例如 `frontend/src/games/stardew/location-labels.ts`）供桌面和移动端共用，比在移动端重复一份 200 行字典更合理，这次按最小实现处理没有做。
- “更多”Tab 是唯一剩下的 M0 占位卡；`StardewMobileShell` 仍没有 `onLogout` 入口，留给“更多”Tab 实现时一并补上。

# MOBILE-SAVES-M5-1 移动端存档页 M5，底部导航“任务”改“存档”（同日追加）

## 背景

用户要求不再做手机端“任务”页，把底部导航第 4 个 Tab 从“任务”改成“存档”，用于查看当前服务器的存档信息并展示对应的存档地图原画。明确要求：复用现有数据/接口/资源和 Stardew/image2 UI 风格，不影响桌面端，不重写后端逻辑，不新增后端接口；如果接口没有地图类型字段就先展示默认图并留好映射入口——但实际检查发现 `SaveInfo` 早就有 `farmType` 字段（桌面新建存档页、`SavesSection.tsx` 都在用），所以这次不是“字段缺失占位”，而是复用已有字段 + 已有的桌面地图原画映射逻辑。

## 改了什么

- `StardewMobileShell.tsx`：本地私有类型 `MobileTabKey` 的第 4 个值从 `'jobs'` 改成 `'saves'`，`MOBILE_TABS` 对应 label 从“任务”改成“存档”。**这个改动只影响移动端 Shell 内部的本地 Tab 状态**，和桌面 `stardew-routes.ts` 里 `StardewRoute` 联合类型的 `'jobs'`（任务日志路由，`JobsLogsPage.tsx` 用）是两个完全独立的类型/命名空间，没有任何引用关系，改名不会影响桌面任务日志页、`routeToPath('jobs')` 或任何桌面路由跳转。新增 `activeTab === 'saves'` 分支渲染新组件，“更多”Tab 仍是 M0 占位卡。
- 新增 `frontend/src/games/stardew/mobile/MobileSavesPage.tsx` + `MobileSavesPage.css`（和 `MobileHomePage`/`MobileControlPage`/`MobilePlayersPage` 同级目录，class 前缀 `sd-msave-`）：
  - Props 沿用既有约定 `Pick<StardewPageProps, 'user'|'instanceState'|'dashboardData'>`，实际只用到 `dashboardData`（存档信息展示不区分管理员/普通用户权限，和桌面 `SavesPage` 的只读展示部分一致，刷新按钮所有登录用户都能点）。
  - 数据完全来自现有 `dashboardData.saves`（`SavesListResult { saves: SaveInfo[]; activeSaveName: string }`）/`dashboardData.savesError`/`dashboardData.refreshSaves()`，没有新增 API 函数、没有新增类型字段。`activeSave = saveRows.find(s => s.isActive || s.name === activeSaveName)`；如果没有显式激活的存档但列表非空，回退取列表第一个存档展示（状态徽章标“可用”而不是“当前使用中”，区分“这是服务器正在用的存档”和“这只是列表里随便挑一个展示的存档”两种语义）；列表为空时整卡替换成空状态文案，不留白。
  - 页面顶部一行：标题“存档”（配 `icon_nav_saves_chest_image2.png`，和桌面 `SavesPage.tsx` 页头同一张图标）+ 右侧“刷新”按钮。刷新按钮没有复用某个共享 loading 字段——`StardewDashboardData` 类型里没有 `savesLoading`（这一点和 `players`/`playersLoading` 不一样），改成本地 `refreshBusy` state 包一层 `await dashboardData.refreshSaves()`（`refreshSaves` 类型签名是 `() => void` 但运行时实现是 `async` 函数，`await` 一个声明为 `void` 的调用在这个仓库里已经是既有写法，`MobilePlayersPage.tsx`/`MobileHomePage.tsx` 的 `await dashboardData.refreshPlayers()` 就是同样模式，编译器不会报错）。首次加载态判断借用 `dashboardData.loading && dashboardData.saves === null`。
  - **地图原画卡**（第一块，大图展示）：`.sd-msave-map-frame` 固定 `aspect-ratio:16/9` 容器 + `object-fit:contain`——农场地图素材原生尺寸只有 88×80 像素画（用 `node -e` 读了 PNG IHDR 实测确认），用 `cover` 会裁掉边缘看不全整张农场布局，`contain` 完整展示、容器背景铺 `background_parchment_tile.png`（羊皮纸贴图，仓库已有素材）填充左右留白，图片本身叠 `image-rendering:pixelated` 保持像素锐利不糊。容器右上角绝对定位状态徽章（`sd-tag-green` 当前使用中 / `sd-tag-gold` 可用 / 默认灰 `sd-tag` 未找到），图片下方一行居中说明当前地图类型文字。
  - **地图匹配入口** `saveFarmMapSrc(save)`：按 `save.farmType` 查两个映射表 `farmTypeLabel`（英文 key→中文名）/`farmTypeAlias`（中文名→英文 key，兼容 QA mock 数据里直接用中文存 `farmType` 的情况，例如 `qa-layout-main.tsx` 的 mock 存档就是 `farmType: '标准农场'`），命中后拼 `/assets/stardew/new-game/farms/{key}.png`（桌面 `NewGameCreator.tsx`/`SavesSection.tsx` 已经在用的 6 张农场原画素材：`standard`/`riverland`/`forest`/`hilltop`/`wilderness`/`beach`，没有新增图片）。这两个映射表和桌面 `SavesSection.tsx` 里的同名表逐字一致，按仓库“各页面自带小工具函数”的既有风格（`MobileControlPage.tsx`/`MobileHomePage.tsx` 都是这么做的）在这个文件里独立维护一份，没有抽共享模块。未命中映射表、或图片加载失败（`<img onError>` 触发一个 `mapImgFailed` state）都统一回退到 `DEFAULT_MAP_SRC = /assets/stardew/ui/backgrounds/background_login_farm_generated.png`——这张图是 `LOGIN-MOBILE-FIX-1` 已经在用的纯像素农场背景素材（没有任何假 UI 元素），复用同一张图当“地图未知”兜底，不从外部拉图、不新增素材。**这次不是“接口没有地图类型字段所以先占位”**——`SaveInfo.farmType` 后端早就有并且桌面已经在用同一套映射逻辑；“映射入口清晰、以后接入真实字段”这条要求已经通过 `saveFarmMapSrc()` 这个独立函数自然满足（以后如果 `farmType` 的取值范围变化，只需要改这一个函数）。
  - **核心信息卡**（第二块）：存档名称、农场名称、农场主（`farmerName`）、游戏日期，两列网格布局，游戏日期复用和 `MobileHomePage.tsx` 逐字一致的 `SEASON_ZH` 字典 + 拼接函数（按同样的“各页面自带小工具函数”风格重写一份，不共享）。字段缺失统一显示“—”，不渲染空字符串或 `undefined`。
  - **更多信息卡**（第三块）：地图类型（文字版，和地图原画卡的图说是同一个 `saveFarmTypeText()` 函数）、存档大小（`formatBytes(fileSizeBytes)`）、最后保存时间（`formatDate(modifiedAt)`）、存档状态文字版；`save.parseError` 存在时在卡片底部追加一条 `.sd-notice--error` 提示，不会让整卡崩溃或空白。
  - **存档操作卡**（第四块，用户后续追加的要求）：**没有**放在 `!displaySave` 空状态分支内部——空状态下（暂无任何存档）这张卡仍然渲染，因为“导入存档”本来就要覆盖“服务器还没有任何存档，需要先导入一个”这种起始场景，只有地图原画/核心信息/更多信息三张卡才依赖有存档才展示。
    - **导出存档**：`handleExport()` 逐字照抄桌面 `SavesSection.tsx` 的同名函数——`exportSave(displaySave.name)` 拿到 `{blob, filename}` 后用临时 `<a download>` 元素触发浏览器下载、`URL.revokeObjectURL` 收尾，没有新增 API。`disabled={exportBusy || !displaySave}`，不要求管理员权限、不要求服务器停止运行——和桌面按钮的门控（`disabled={busy}`，无 `isAdmin`/`isRunning` 限制）完全一致，这是刻意保持的行为对齐，不是遗漏。
    - **导入存档**：三段状态机逐字照抄桌面 `handleUploadPreview`/`handleUploadCommit`/`handleUploadCancel`——选文件→`uploadSavePreview(file)`拿预览→确认后`uploadSaveCommitAndStart(token)`导入并启动，取消时如果已经生成过 token 会调用 `uploadSaveCommitAndStart(token, true)` 尽力清理挂起的上传（失败静默忽略，和桌面版一致）。预览信息展示复用本文件已有的 `saveDateText()`/`saveFarmTypeText()`/`formatBytes()`/`formatDate()`，没有重复写一遍格式化逻辑。**和桌面版唯一的差异**：桌面 `handleUploadCommit` 成功后调用 `onJobStarted(jobId)`/`onSavesChanged?.()`（这两个是 `SavesSection` 的 props 回调，触发跳转总览页 + 父组件联动刷新），移动端页面 props 里没有这两个（`Pick<StardewPageProps,'user'|'instanceState'|'dashboardData'>` 不包含 `onNavigate`/`saveActionRequest`），改成直接调用 `dashboardData.requestInviteCodeRefresh()`/`refreshInstanceState()`/`refreshJobs()`/`refreshSaves()` 四个刷新函数——这四个函数本来就是 `onJobStarted`/`onSavesChanged` 桌面回调链最终触达的同一批底层刷新函数，效果等价，只是跳过了"导入后跳转到总览页"这一步（移动端本来就是 Tab 切换而不是路由跳转，没有对应的"总览"跳转语义）。弹窗 UI（`.sd-msave-dialog-*`）重新按移动端排版，未复用桌面 `.sd-saves-modal-*` 类名。`disabled={!isAdmin || isRunning}`，`isRunning` 判定 `state==='running'||state==='starting'`，和桌面 `SavesSection.tsx` 的 `isRunning` 定义逐字一致（这也是这个文件第一次引入 `isAdmin`/`isRunning`，此前"只读展示"阶段没有用到 `user`/`instanceState` 两个 prop，这次为了门控导入按钮重新把它们加回 props 解构）。
    - **回档**：纯占位——按钮恒定 `disabled`，`title` 和按钮下方一行 `.sd-msave-op-hint` 说明文字都明确写"暂不支持手机浏览器，请前往桌面端存档管理页使用备份与恢复功能"。没有接入 `restoreSaveBackup()`、没有读取备份列表（`getSaveBackups()`）——移动端目前完全不展示备份数据，这是这次用户明确要求的范围（"回档的占位按钮，标明目前不支持手机浏览器"），不是漏做，之后如果要真做，需要先决定"是否要把备份列表也搬到移动端"这个更大的范围问题。
  - 视觉基础件（`.sd-panel`/`.sd-tag*`/`.sd-notice--*`/`.sd-btn-tan`/`.sd-btn-green`）全部走全局 `stardew-theme.css`（`main.tsx` 顶层加载，移动端可以放心用），没有复用 `StardewPanel.css` 里桌面存档卡片（`.sd-save-card*`）或上传弹窗（`.sd-saves-modal-*`）的任何类名——那批类只在挂载 `StardewPanel.tsx` 时才会被 Vite 加载，移动端走 `StardewMobileShell` 不会加载这个文件，这是这一整轮移动端页面（M2/M3/M4）都遵守的既有约定，这次延续。
  - 页面信息展示部分（地图原画/核心信息/更多信息三张卡）仍然只展示"当前存档"（单份），不像桌面 `SavesPage`/`SavesSection.tsx` 那样列出全部存档、支持创建/切换/删除/备份——这些能力仍然只在桌面端；这次追加的导出/导入是用户明确要求补的两个操作入口，不是要把整个存档管理搬过来，"新建游戏"、"存档库"网格、"备份与恢复"区块依然只在桌面端。

## 影响文件

- 新增：`frontend/src/games/stardew/mobile/MobileSavesPage.tsx`、`frontend/src/games/stardew/mobile/MobileSavesPage.css`。
- 修改：`frontend/src/games/stardew/StardewMobileShell.tsx`（Tab key/label 改名 + 新增一个渲染分支）。
- 未改：任何后端文件、`frontend/src/games/stardew/pages/SavesPage.tsx`、`SavesSection.tsx`、`StardewPanel.css`、`types.ts`（`SaveInfo`/`SavesListResult`/`UploadPreviewResult` 字段未变）、`api.ts`（未新增/修改任何函数签名，只调用了已有的 `exportSave`/`uploadSavePreview`/`uploadSaveCommitAndStart`/`dashboardData.refreshSaves()` 等已有导出）、`useStardewDashboardData.ts` 内部实现、`qa-layout-main.tsx`（现有 mock 存档数据已经覆盖多种 `farmType`/`isActive` 场景，导出/导入走真实 `fetch`，QA mock 的 `/saves/upload-preview`/`/saves/*/export`/`/saves/upload-commit-and-start` 路由沿用既有兜底 `jsonRes({}, 200)`，不需要新增专门 mock）、任何登录/权限/鉴权逻辑、桌面 `stardew-routes.ts` 里的 `StardewRoute`/`'jobs'` 路由、`restoreSaveBackup()`/`getSaveBackups()`（回档占位没有接入这两个函数）。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过。
- `cd frontend; npm run build`（`tsc -b && vite build`）：通过。

**未做的验证**：本次环境没有可用的浏览器自动化/截图工具，没有做真实浏览器缩放或真机截图确认 390×844/393×852/430×932 下的实际渲染效果（尤其是地图原画卡 `aspect-ratio:16/9` + `object-fit:contain` 在不同素材尺寸下的留白观感、状态徽章绝对定位是否和地图内容重叠、存档操作卡三个按钮在窄屏下是否需要换行），也没有连一个真实运行中的实例走一遍"查看当前激活存档 → 点击刷新 → 确认字段更新 → 导出存档确认下载 → 导入存档确认预览与导入并启动"的完整链路。建议下一位维护者：
1. `cd frontend; npm run dev -- --host 0.0.0.0`，手机访问局域网地址登录后确认"存档"Tab 正常显示、无横向滚动，重点验证手机浏览器上点击"导出存档"是否能正常触发下载（不同手机浏览器对 `<a download>` blob 下载的支持程度可能不同，桌面版这段代码经过验证，但没有专门在移动端浏览器测过）。
2. 或用 `frontend/qa-layout.html?shell=mobile&state=running` 在浏览器里直接缩放窗口到上述三个宽度确认布局（mock 数据里有 4 个不同 `farmType` 的存档，可以顺手确认地图匹配逻辑对"标准农场"/"河边农场"/"森林农场"这几个中文 `farmType` 值都能正确命中）；导入存档弹窗在 QA mock 环境下由于 `/saves/upload-preview` 落到默认 `jsonRes({}, 200)` 兜底，预览步骤可能拿不到真实字段，建议连真实后端验证完整导入链路。
3. 真实连一个存档解析失败（`parseError` 非空）或 `farmType` 为空/未知值的场景，确认地图卡正确回退到默认原画、核心信息卡不崩溃。
4. 非管理员账号登录后确认"导入存档"按钮正确禁用并显示"仅管理员可执行此操作"；"导出存档"按钮不受管理员权限限制，非管理员也应该能点。

## 下一步注意事项

- 页面信息展示部分目前只展示"一个存档"（激活的，或列表第一个），如果以后用户反馈需要在移动端也能看到多份存档或切换存档，需要重新设计成列表结构（可以参考桌面 `SavesSection.tsx` 的 `SaveCard` 数据字段用法，但视觉和交互要按移动端单列卡片风格重排，不要直接复用桌面组件——这是这一整轮 M2/M3/M4 反复验证过的教训）。
- `farmTypeLabel`/`farmTypeAlias`/`saveFarmMapSrc()` 这套地图映射逻辑目前在 `SavesSection.tsx` 和 `MobileSavesPage.tsx` 里各维护一份（逐字相同）；如果以后地图类型枚举变化需要同步改两处，尚未抽成共享模块，属于按仓库既有风格的重复，不是遗漏。
- "回档"目前是纯占位禁用按钮，如果以后要在移动端真正实现，需要先决定数据来源——桌面 `getSaveBackups()`/`BackupInfo` 已经有完整字段，可以直接复用 API，但需要新写一个移动端备份列表 UI（选哪个备份、覆盖确认、`restoreSaveBackup()` 的 `save_exists` 冲突处理，参考 `SavesSection.tsx` 的 `handleRestoreConfirmed`），工作量接近再做一个 M6，不是简单加一个按钮就能完成。
- "更多"Tab 仍是唯一剩下的 M0 占位卡；`StardewMobileShell` 仍没有 `onLogout` 入口，留给"更多"Tab 实现时一并补上（这一点和 `MOBILE-PLAYERS-M4-1` 记录的一致，不是新问题）。

# MOBILE-VISUAL-UNIFY-M6-1 手机端卡片与底部 Tab 视觉统一优化（同日追加）

## 背景

手机端各页面卡片使用全局 `.sd-panel` 类（`stardew-theme.css`），默认是直角方块（`border-radius:0`）。PC 总览页经过多轮视觉迭代已经有成熟的圆角卡片风格（9px 圆角、棕色边框、三层内/外阴影）和统一羊皮纸渐变背景色。底部 Tab 栏也偏硬（贴底直角横条，纯文字，无图标，`active` 态不够显眼）。这次目标是把手机端视觉拉到和 PC 总览页一致的成熟度：

- 卡片背景色：使用 PC 总览页"存档/模组"指标卡（`.sd-mc`）的背景色。
- 卡片圆角/边框/阴影/间距：参考 PC 总览页"在线玩家"卡片（`.sd-ov-card`）的样式。
- 底部 Tab 栏：更圆润、有图标、active 态更明显、触控反馈、安全区适配。

## 改了什么

### 1. 卡片统一样式

`StardewMobileShell.css` 新增 `:root` CSS 变量和一条祖先限定覆盖规则：

```css
:root {
  --stardew-mobile-card-bg:
    linear-gradient(180deg, rgba(255, 245, 214, 0.96), rgba(248, 226, 174, 0.94)),
    #f7e3ad;
  --stardew-mobile-card-border: 2px solid #a06c2c;
  --stardew-mobile-card-radius: 9px;
  --stardew-mobile-card-shadow:
    inset 0 0 0 2px rgba(255, 243, 210, 0.55),
    inset 0 -5px 9px rgba(160, 108, 44, 0.12),
    0 2px 0 rgba(90, 55, 18, 0.25);
}

.sd-mshell .sd-panel {
  background: var(--stardew-mobile-card-bg);
  border: var(--stardew-mobile-card-border);
  border-radius: var(--stardew-mobile-card-radius);
  box-shadow: var(--stardew-mobile-card-shadow);
}
```

选择器用 `.sd-mshell .sd-panel`（优先级 (0,2,0)）稳赢全局 `.sd-panel` 单类规则 (0,1,0)，和 `MOBILE-CONTROL-M3-1` 踩过的同一类 Vite 打包层叠顺序坑用同样方法规避。作用域限定在 `.sd-mshell` 内部，桌面端 `StardewPanel.tsx`（不渲染在 `.sd-mshell` 下）和登录页 `.sd-auth-shell` 不受影响。

这条规则覆盖了：
- 总览页 4 张卡片（`.sd-panel.sd-mhome-card`）
- 控制页状态条 + 2 张卡片（`.sd-panel.sd-mctrl-status-strip` / `.sd-panel.sd-mctrl-card`）
- 控制页 4 个弹窗（`.sd-panel.sd-mctrl-dialog`）
- 玩家页主卡片（`.sd-panel.sd-mplay-card`）+ 确认弹窗（`.sd-panel.sd-mplay-confirm-dialog`）
- 存档页 4 张卡片（`.sd-panel.sd-msave-card` / `.sd-panel.sd-msave-map-card`）
- "更多"Tab 占位卡（`.sd-panel.sd-mshell-card`）
- 总览页确认弹窗（`.sd-panel.sd-mhome-confirm-dialog`）

### 2. 玩家行卡片

`MobilePlayersPage.css` 的 `.sd-mplay-player-card`（每个玩家条目的小卡片）也同步引用这组变量，从扁平的 `border:1px solid #dcc898; border-radius:2px` 升级为和外层容器卡片一致的圆角样式。

### 3. 底部 Tab 栏重做

- 从贴底满宽直角硬条改为两侧各缩进 `10px` 的悬浮胶囊式导航条（`border-radius:20px`）。
- `bottom: calc(10px + env(safe-area-inset-bottom, 0px))` 适配 iPhone Home Indicator 区域。
- 每个 Tab 从纯文字改为纵向 `图标 + 文字` 结构（复用桌面导航的 5 张 image2 icon PNG），`min-height:48px` 满足 44px+ 触控热区要求。
- `active` 态：`background:var(--sd-green-bg); color:var(--sd-green-text)` 绿色高亮 pill。
- `:active` 态：`transform:scale(0.92)` 按压缩放反馈。
- 文案：`text-overflow:ellipsis; white-space:nowrap` 防溢出换行。
- Tab 按钮圆角 14px，内间距均匀。
- `.sd-mshell` 的 `padding-bottom` 从 `56px` 提到 `84px`，匹配更高的浮动底栏占位。

### 4. TSX 变更

`StardewMobileShell.tsx`：
- `MOBILE_TABS` 每项新增 `icon` 字段。
- Tab `<button>` 内部从纯 `{tab.label}` 改为 `<img className="sd-mshell-tab-icon">` + `<span className="sd-mshell-tab-label">`。
- 无业务逻辑变更。

## 影响文件

- `frontend/src/games/stardew/StardewMobileShell.css`（主要改动：变量 + 卡片覆盖 + Tab 栏重写）
- `frontend/src/games/stardew/StardewMobileShell.tsx`（Tab 结构补图标）
- `frontend/src/games/stardew/mobile/MobilePlayersPage.css`（玩家行卡片样式升级）

未改：任何后端文件、`StardewPanel.css`、`StardewPanel.tsx`、各 Mobile*Page.tsx 文件的业务逻辑、`App.css`（登录页）、`stardew-theme.css`、`api.ts`、`types.ts`、hooks、store。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过。
- `cd frontend; npm run build`（`tsc -b && vite build`）：通过。
- 构建产物 CSS 确认 `.sd-mshell .sd-panel{...border-radius:...9px...}` 和 `.sd-mshell-tabbar{...border-radius:20px...}` 规则存在且生效。

**未做的验证**：本次环境没有可用的浏览器自动化/截图工具，没有做真实浏览器缩放或真机截图。建议下一位维护者：
1. `cd frontend; npm run dev -- --host 0.0.0.0`，手机访问局域网地址（如 `http://192.168.0.5:5173/`）确认：
   - 所有页面卡片都是圆角（9px），不再是直角方块
   - 底部 Tab 栏是浮动的圆润胶囊条，有图标+文字
   - active Tab 有明显绿色高亮
   - 点击 Tab 有缩放动画反馈
   - iPhone 底部 Home Indicator 区域有适当间距
2. 确认桌面端（>768px）打开面板时视觉完全不变。
3. 确认登录页（手机端/PC 端）不受影响。
4. 390×844、393×852、430×932 三个宽度下无横向滚动、Tab 文案不溢出。

## 下一步注意事项

- `--stardew-mobile-card-*` 变量定义在 `:root` 上而非 `.sd-mshell` 上，这是因为 `MobilePlayersPage.css` 里 `.sd-mplay-player-card` 不在 `.sd-mshell .sd-panel` 选择器作用范围内（它不用 `.sd-panel` 类），需要直接引用这些变量。如果以后有其它非 `.sd-panel` 的手机端元素需要同一组卡片样式，直接引用这些变量即可。
- `MOBILE-CONTROL-M3-1` 记录的 `MobileHomePage.css` 里 `.sd-mhome-copy-btn` 单类选择器可能存在的打包层叠顺序风险仍未修（不在本次范围）。如果以后修那个问题，直接改成复合选择器即可（如 `.sd-btn-tan.sd-mhome-copy-btn`、`.sd-btn-green.sd-mhome-copy-btn`）。
- 底栏高度变化（56px→约 70px 含 padding，加浮动 gap 约 84px 占位）可能导致部分页面底部"最后一张卡片"距离底栏太近或太远，需要真机确认；如果太近可以把 `.sd-mshell` 的 `padding-bottom` 从 84px 微调到 90px。

# PLAYERS-WARP-HOME-1 玩家回家按钮

## 改了什么

- 桌面玩家管理页 `PlayersPage.tsx` 的在线玩家行新增“回家”图标按钮，位置在“踢出”左侧。点击后弹确认框，确认后调用 `warpPlayerHome(uniqueMultiplayerId, name)`，成功/失败消息和刷新玩家列表流程沿用现有踢出/封禁模式。
- 手机玩家页 `MobilePlayersPage.tsx` 同步新增“回家”按钮，位置同样在“踢出”左侧；按钮禁用条件、确认弹窗、busy 状态和错误提示与桌面端一致。
- `api.ts` 新增 `warpPlayerHome()`，请求 `POST /api/instances/:id/players/warp-home`。
- 新增 image2 风格图标 `frontend/public/assets/stardew/ui/icons/icon_players_action_home_image2.png`，尺寸 192x192，`StardewPanel.css` 通过 `.sd-players-icon-home::before` 引用。
- `MobilePlayersPage.css` 轻微收窄玩家操作按钮宽度，确保手机端“回家 / 踢出 / 封禁”三按钮在卡片内更稳。

## 影响文件

- `frontend/src/api.ts`
- `frontend/src/games/stardew/pages/PlayersPage.tsx`
- `frontend/src/games/stardew/mobile/MobilePlayersPage.tsx`
- `frontend/src/games/stardew/mobile/MobilePlayersPage.css`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/public/assets/stardew/ui/icons/icon_players_action_home_image2.png`

## 交互约束

- 按钮禁用条件：非管理员、服务器未运行、目标离线、目标是 host、缺少 `uniqueMultiplayerId`、当前回家操作处理中。
- 这是在线 rescue 功能，不处理离线玩家；实际调用的是后端 `PLAYERS-WARP-HOME-1`，最终由控制模组调用 JunimoServer 的 `farmer.WarpHome()`。
- HTTP 成功只表示“命令已提交”，前端没有精确的游戏内落点回执。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .` 通过。
- `cd frontend; npm run build` 通过。
- 后端相关包测试和 SMAPI DLL 构建见 backend handoff 同名小节。

## 下一步注意事项

- 仍需真机/真实服务器验证：手机端玩家卡片三个按钮不横向溢出，点击“回家”后确认弹窗可操作，farmhand 实际传送回家。
- 新图标目前只用于桌面行内图标按钮；手机端按现有文字按钮风格处理，没有额外放图标，避免窄屏按钮拥挤。
# FE-INSTALL-STEAM-AUTH-BUTTON-1 安装页授权按钮统一

- 改动：安装页删除“更换 Steam 账号 / 重新认证”及其强制重认证表单入口，改为常驻显示“登录授权”。该按钮不检查 `steamAuthLoggedIn`，所以认证完成后也继续显示。
- 共用逻辑：新建 `frontend/src/games/stardew/useSteamAuthLogin.ts`，总览页 `InviteCodeCard` 与安装页均使用它；接口仍为 `POST /api/instances/:id/steam-auth/login`，成功跳转安装页，运行/启动状态要求先停服，错误原位显示。
- 影响文件：`frontend/src/games/stardew/useSteamAuthLogin.ts`、`frontend/src/games/stardew/InviteCodeCard.tsx`、`frontend/src/games/stardew/pages/InstallPage.tsx`。
- 验证：`cd frontend; npm.cmd run build` 通过；仅有既有的 chunk 大小提示。
- 下一步注意：若以后调整 Steam 登录授权的文案、禁用条件或跳转行为，应只修改 `useSteamAuthLogin`，避免总览页与安装页再次分叉。
# REAL-INSTANCE-MULTIPLAYER-VERIFIED-1 多人管理真实实例验证补记

- 用户已确认认证批准、踢出、封禁、回家四条多人操作链路均已在真实实例验证通过。本标记取代相关小节的“未做真机联机验证”说明。
- 未单独确认的移动端像素级布局、触控热区等视觉验证状态不随本标记改变。
