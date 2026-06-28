# Frontend UI Refactor Implementation Plan

本文档把 `docs/prototypes/stardew-anxi-panel-ui-refactor-prototype.html` 拆成真实前端路由与渐进式实施目标。原型图不是四个最终页面，而是把多个业务页面压缩展示在四个大区块里；真实 React 前端必须按左侧导航和业务任务拆路由，并先接入已有后端能力，暂时没有后端支持的功能保留 UI 位置和空状态，后续再补接口。

## 核心判断

原型图展示的是这些真实路由：

```text
/instances/stardew/install       首次安装向导
/instances/stardew/overview      概览
/instances/stardew/server        服务器控制
/instances/stardew/saves         存档管理
/instances/stardew/jobs          任务与日志
/instances/stardew/players       玩家管理
/instances/stardew/mods          模组管理
/instances/stardew/diagnostics   诊断与健康检查
```

原型左侧还有“设置”。当前项目已有面板用户、审计日志、版本信息和登出能力，但它们不是 Stardew 游戏玩家管理。建议补一个内部路由：

```text
/instances/stardew/settings      面板设置 / 用户 / 审计 / 版本
```

`players` 路由用于未来 Stardew 联机玩家管理。当前后端没有完整玩家列表、踢出、权限、白名单等 API，所以第一版只保留 UI 和占位，并尽量从已有状态中显示可用摘要。

## 输入材料

| 材料 | 用途 |
|------|------|
| `docs/prototypes/stardew-anxi-panel-ui-refactor-prototype.html` | 视觉、密度、导航和交互意图来源 |
| `docs/prototypes/assets/ui-extracted/` | 已拆出的无文字 PNG 素材 |
| `frontend/src/App.tsx` | 当前 setup/login/dashboard 顶层入口 |
| `frontend/src/api.ts` | 当前已有 API 函数 |
| `frontend/src/types.ts` | 当前前后端类型边界 |
| `frontend/src/games/stardew/*` | 现有 Stardew 功能组件 |
| `frontend/src/core/*` | 现有通用组件与 helper |

## 不变原则

- 保持 Single Game Mode：登录后直接进入 Stardew 面板，不显示多游戏总面板。
- 第一阶段不改后端 API；已有功能优先复用 `frontend/src/api.ts`。
- 没有后端支持的原型功能保留 UI 入口、空状态、禁用按钮或“待接入”状态，不硬编码假数据。
- Stardew 专属 UI 放在 `frontend/src/games/stardew`。
- 通用 Shell、导航、按钮、面板、确认框等放在 `frontend/src/core` 或 `frontend/src/core/shell`。
- 每轮重构都必须保持 `npm.cmd run build` 可通过。
- 不把未来 Minecraft/DST/Terraria/Palworld 的逻辑写进 Stardew 模块。

## 路由骨架

当前项目没有 `react-router-dom`。第一轮建议用内部 route + History API，降低依赖和重构风险。

```ts
type AppView = 'booting' | 'setup' | 'login' | 'stardew'

type StardewRoute =
  | 'install'
  | 'overview'
  | 'server'
  | 'saves'
  | 'jobs'
  | 'players'
  | 'mods'
  | 'diagnostics'
  | 'settings'
```

推荐 URL 行为：

```text
/                         未登录显示 setup/login，已登录进入 /instances/stardew/overview
/instances/stardew        切到 overview
/instances/stardew/install
/instances/stardew/overview
/instances/stardew/server
/instances/stardew/saves
/instances/stardew/jobs
/instances/stardew/players
/instances/stardew/mods
/instances/stardew/diagnostics
/instances/stardew/settings
```

后续进入 Multi Game Mode 时，再考虑引入 `react-router-dom` v6+，把当前内部 route 迁移为正式路由。

## 总体组件结构

```text
App
  SetupPanel
  LoginPanel
  StardewPanel
    StardewShell
      StardewLeftNav
      StardewTopStatusBar
      StardewRouteOutlet
        InstallPage
        OverviewPage
        ServerControlPage
        SavesPage
        JobsLogsPage
        PlayersPage
        ModsPage
        DiagnosticsPage
        SettingsPage
      StardewOpsRail
```

建议新增文件：

```text
frontend/src/core/shell/AppShell.tsx
frontend/src/core/shell/LeftNav.tsx
frontend/src/core/shell/TopStatusBar.tsx
frontend/src/core/shell/OpsRail.tsx
frontend/src/core/shell/ConfirmDialog.tsx
frontend/src/games/stardew/StardewPanel.tsx
frontend/src/games/stardew/stardew-routes.ts
frontend/src/games/stardew/pages/InstallPage.tsx
frontend/src/games/stardew/pages/OverviewPage.tsx
frontend/src/games/stardew/pages/ServerControlPage.tsx
frontend/src/games/stardew/pages/SavesPage.tsx
frontend/src/games/stardew/pages/JobsLogsPage.tsx
frontend/src/games/stardew/pages/PlayersPage.tsx
frontend/src/games/stardew/pages/ModsPage.tsx
frontend/src/games/stardew/pages/DiagnosticsPage.tsx
frontend/src/games/stardew/pages/SettingsPage.tsx
```

## 路由功能矩阵

| 路由 | 已有后端优先接入 | 原型保留但后续接入 |
|------|------|------|
| Install | `prepareInstance`、`getInstallOptions`、`installInstance`、`submitSteamGuardInput`、`createJobEventSource` | 更细的环境项、端口/公告/时区配置、安装位置修改 |
| Overview | `getInstanceState`、`getInviteCode`、`getJobs`、`getHealthDiagnostics`、`getSaves`、`getMods` | 在线玩家详细列表、服务器资源曲线、真实游戏内日期 |
| Server | `startInstance`、`stopInstance`、`restartInstance`、`getInviteCode`、`getCommands`、`runCommand`、`sendSay` | 端口配置、服务器公告、可见性、密码策略、计划重启 |
| Saves | `getSaves`、`selectSave`、`selectSaveAndStart`、`deleteSave`、`exportSave`、`createNewGame`、`uploadSavePreview`、`uploadSaveCommitAndStart` | 存档备份列表 UI、恢复备份 UI、存档对比、自动备份策略 |
| Jobs | `getJobs`、`getJob`、`getJobLogs`、`createJobEventSource`、`clearJobs` | 全量实时日志流、过滤、下载日志、任务重试 |
| Players | 暂无完整玩家 API；可从实例状态或邀请码区域显示摘要 | 在线玩家列表、角色槽、踢出、封禁、白名单、权限、玩家事件 |
| Mods | `getMods`、`uploadMod`、`deleteMod`、`exportMods` | 启用/禁用、依赖检查、Mod 更新、兼容性提示 |
| Diagnostics | `getHealthDiagnostics`、`getDockerStatus`、`getComposePs`、`getComposeLogs`、`downloadSupportBundle` | 资源趋势图、网络探测、自动修复、一键清缓存 |
| Settings | `/api/users`、`getAuditLogs`、`getVersion`、`logout` | 面板偏好设置、审计过滤、日志归档、主题配置 |

## 现有组件归位

| 现有组件 / 逻辑 | 新位置 |
|------|------|
| `InstallSection` | `InstallPage` |
| `LifecycleSection` | `ServerControlPage`；其中状态摘要和主按钮可在 `OverviewPage` 复用 |
| `InstanceStateCard` | `OverviewPage` 和 `TopStatusBar` |
| `SavesSection` | `SavesPage` |
| `ModsSection` | `ModsPage` |
| `JobsSection` | `JobsLogsPage` 和 `OpsRail` 最近任务摘要 |
| `ConsoleSection` | `ServerControlPage` 的命令区，或 `DiagnosticsPage` 的工具区 |
| `DockerSection` | `DiagnosticsPage` |
| 用户管理逻辑 | `SettingsPage`，不要放进 Stardew 玩家管理 |
| 健康检查逻辑 | `DiagnosticsPage`，Overview 只显示摘要 |
| 审计日志逻辑 | `SettingsPage` |

## 细化目标

### FE-R0：原型识别与现状保护线

**目标：** 在动前端代码前，把原型里的视觉块拆成真实路由，并保护现有功能。

**任务：**
- 标记原型中每个视觉区域对应哪个真实路由。
- 列出现有 `App.tsx` 中所有业务状态、刷新逻辑、SSE 逻辑和用户管理逻辑。
- 确认哪些功能已有后端，哪些只能做占位。
- 记录当前 `git status --short`，避免误回滚用户已有改动。

**完成标准：**
- 有明确路由清单和组件归位表。
- 不改运行代码或只做文档。

### FE-R1：素材接入与主题基础

**目标：** 让真实前端可以使用原型视觉资产，但不依赖 `docs/` 路径。

**任务：**
- 将需要用于生产前端的素材复制到 `frontend/public/assets/stardew/ui/`。
- 建立统一路径常量或 CSS 变量，避免业务组件散落硬编码图片路径。
- 整理主题变量：木纹、羊皮纸、绿色、红色、金色、墨色、边框、紧凑字号。
- 建立基础样式类：像素按钮、羊皮纸面板、木纹侧栏、输入框、状态点、数据表格。

**完成标准：**
- `npm.cmd run build` 后素材路径可用。
- 当前页面功能不回退。

### FE-R2：StardewShell 与真实左侧导航

**目标：** 建立真实 Stardew 面板壳，让左侧导航成为路由入口。

**任务：**
- `App.tsx` 继续只负责 boot/setup/login/auth。
- 新建 `StardewPanel`，登录后进入 Stardew 面板。
- 新建 `StardewRoute` 和 route parser。
- 左侧导航包含：安装向导、概览、服务器控制、存档管理、任务与日志、玩家管理、模组管理、诊断与健康检查、设置。
- 支持 `pushState` 和 `popstate`。
- 顶部状态栏显示实例状态、当前存档、版本、最近更新时间等已有信息。
- 右侧 `OpsRail` 先显示最近任务、健康摘要、快捷入口。

**完成标准：**
- 9 个路由都能切换。
- 未实现页面显示占位，但保留原型布局和未来功能入口。
- setup/login 不受影响。

### FE-R3：Overview 路由

**目标：** 做可用的日常首页，不承担所有控制功能。

**接入已有功能：**
- 实例状态：`getInstanceState`
- 邀请码摘要：`getInviteCode`
- 最近任务：`getJobs`
- 健康摘要：`getHealthDiagnostics`
- 当前存档摘要：`getSaves`
- Mod 重启提示摘要：`getMods`

**保留未来功能：**
- 资源曲线、玩家详细在线表、游戏内时间趋势、更多事件流。

**完成标准：**
- 首屏能回答：当前能不能启动、当前存档是什么、有没有失败任务、健康是否异常。
- 不把完整存档、完整任务、完整诊断都堆在 Overview。

### FE-R4：Server Control 路由

**目标：** 把服务器控制从概览中拆出来，承载完整生命周期和命令操作。

**接入已有功能：**
- 启动：`startInstance`
- 停止：`stopInstance`
- 重启：`restartInstance`
- 邀请码：`getInviteCode`
- 命令列表：`getCommands`
- 执行命令：`runCommand`
- 喊话：`sendSay`，当前后端可能返回 `command_not_supported`，前端要能优雅显示。

**保留未来功能：**
- 服务器名称、密码、端口、可见性、公告、计划重启、运行策略。

**完成标准：**
- 启动/停止/重启真实可用。
- 危险操作有确认 UI。
- 命令仍走 allowlist，不允许任意 shell。

### FE-R5：Install 路由

**目标：** 把首次安装做成独立向导。

**接入已有功能：**
- 准备实例：`prepareInstance`
- 版本选项：`getInstallOptions`
- 安装任务：`installInstance`
- Steam Guard 输入：`submitSteamGuardInput`
- SSE 日志：`createJobEventSource`

**保留未来功能：**
- 更完整环境检查、端口设置、服务名、时区、安装位置、安装前存档策略。

**完成标准：**
- Steam Guard / QR / job finished 逻辑不回退。
- 原型中的步骤条可用，但缺少后端字段时显示“待接入”。

### FE-R6：Saves 路由

**目标：** 存档管理独立成页。

**接入已有功能：**
- 列表：`getSaves`
- 选择：`selectSave`
- 选择并启动：`selectSaveAndStart`
- 删除：`deleteSave`
- 导出：`exportSave`
- 新建：`createNewGame`
- 上传预览：`uploadSavePreview`
- 上传确认并启动：`uploadSaveCommitAndStart`

**保留未来功能：**
- 备份列表、恢复备份、自动备份策略、存档对比、存档备注。

**完成标准：**
- 当前两阶段上传流程不坏。
- running/starting 时禁止危险写操作的前端规则不丢。
- 列表刷新机制保留。

### FE-R7：Jobs & Logs 路由

**目标：** 任务与日志独立成页，而不是只做右侧摘要。

**接入已有功能：**
- 任务列表：`getJobs`
- 任务详情：`getJob`
- 任务日志：`getJobLogs`
- SSE：`createJobEventSource`
- 清理任务：`clearJobs`

**保留未来功能：**
- 实时服务器日志流、日志过滤、日志下载、任务重试、任务时间线。

**完成标准：**
- 当前 `JobsSection` 能以页面形式工作。
- 最近任务摘要仍可在 `OpsRail` 使用。

### FE-R8：Players 路由

**目标：** 玩家管理独立成页，先保留 UI 和未来能力位。

**接入已有功能：**
- 当前没有完整玩家 API。
- 可显示从实例状态或 Overview 摘要中可获得的玩家数量/状态；如果没有字段，显示“后端未接入玩家列表”。

**保留未来功能：**
- 在线玩家列表。
- 角色槽管理。
- 邀请记录。
- 踢出/封禁。
- 白名单。
- 权限与协作者。
- 玩家事件历史。

**完成标准：**
- 页面存在，导航可达。
- 没有假玩家数据冒充真实后端。
- 所有未来按钮禁用并标注“待接入”。

### FE-R9：Mods 路由

**目标：** 模组管理独立成页。

**接入已有功能：**
- 列表：`getMods`
- 上传：`uploadMod`
- 删除：`deleteMod`
- 导出：`exportMods`
- 重启提示：`restartRequired`

**保留未来功能：**
- 启用/禁用。
- 依赖关系。
- 更新检测。
- 兼容性检查。
- Mod 配置编辑。

**完成标准：**
- 当前上传、删除、导出功能不回退。
- running/starting 时危险操作禁用。
- 解析失败的 Mod 能显示错误。

### FE-R10：Diagnostics 路由

**目标：** 诊断与健康检查独立成页。

**接入已有功能：**
- 健康检查：`getHealthDiagnostics`
- Docker 状态：`getDockerStatus`
- Compose 状态：`getComposePs`
- Compose 日志：`getComposeLogs`
- 支持包：`downloadSupportBundle`

**保留未来功能：**
- 资源趋势图。
- 网络探测。
- 一键修复。
- 清理缓存。
- 存档索引修复。
- 游戏文件校验。

**完成标准：**
- 当前健康检查和支持包导出可用。
- Docker/Compose 技术信息放在折叠或次级区域，不淹没用户。

### FE-R11：Settings 路由

**目标：** 把面板设置、用户管理、审计和版本信息从 Dashboard 中拆出来。

**接入已有功能：**
- 当前用户：`/api/auth/me`
- 登出：`/api/auth/logout`
- 用户列表/创建/改角色/禁用/删除：`/api/users`
- 审计日志：`getAuditLogs`
- 版本信息：`getVersion`

**保留未来功能：**
- 审计过滤。
- 日志归档。
- 主题偏好。
- 面板安全策略。
- 会话管理。

**完成标准：**
- 管理员和普通用户权限不回退。
- 用户管理不要放进 `players`，避免混淆“面板用户”和“游戏玩家”。

### FE-R12：响应式与视觉 QA

**目标：** 让拆分后的多路由 UI 在桌面和移动端都可用。

**任务：**
- 桌面：左导航 + 主内容 + 右 rail。
- 中屏：右 rail 下移或折叠。
- 手机：导航折叠为顶部或抽屉。
- 表格窄屏变列表。
- 检查最长中文按钮和空状态文案。
- 检查原型素材在 build 后路径正确。

**完成标准：**
- 桌面和移动端无文字重叠、按钮溢出、横向滚动。
- `npm.cmd run build` 通过。

### FE-R13：交接收尾

**目标：** 让下一位维护者知道哪些路由已接后端，哪些只是占位。

**任务：**
- 更新 `docs/handoff-roadmap.md`。
- 更新当天 `docs/conversation-handoff-YYYY-MM-DD.md`。
- 每个路由记录：已接 API、保留占位、未完成风险。
- 如果复制素材到 `frontend/public`，记录来源和更新方式。

**完成标准：**
- 文档中清楚区分“已接入”和“待后端接入”。
- 不只更新 README。

## 推荐实施节奏

第一轮：建立结构但不迁复杂业务。

```text
FE-R1 + FE-R2 + FE-R3 + FE-R8(占位)
```

结果：素材可用，Stardew Shell 和 9 个路由可切换，Overview 接真实 API，Players 路由先占位。

第二轮：迁移日常操作核心。

```text
FE-R4 + FE-R7 + FE-R10
```

结果：服务器控制、任务日志、诊断可用。

第三轮：迁移复杂业务流。

```text
FE-R5 + FE-R6 + FE-R9
```

结果：安装向导、存档、模组独立成页，并保留现有复杂流程。

第四轮：设置与 QA。

```text
FE-R11 + FE-R12 + FE-R13
```

结果：用户/审计/版本归位，响应式和交接完成。

## 给 Codex 的第一轮实施提示词

```text
请按 docs/frontend-ui-refactor-implementation-plan.md 实施第一轮前端 UI 重构，只做：
- FE-R1：素材接入与主题基础
- FE-R2：StardewShell 与真实左侧导航
- FE-R3：Overview 路由
- FE-R8：Players 路由占位

关键要求：
- 原型 HTML 不是四个页面，而是多个业务路由平铺展示。请按真实路由拆：
  install、overview、server、saves、jobs、players、mods、diagnostics、settings。
- 第一轮要让 9 个路由都能导航切换，但只要求 Overview 接真实后端。
- Players 当前没有完整后端 API，只做占位和待接入状态，不造假数据。
- 不改后端 API。
- 不破坏 setup/login。
- Stardew 专属逻辑继续放在 frontend/src/games/stardew。
- 当前项目没有 react-router-dom，先用内部 route + History API。
- 素材不要依赖 docs 路径，复制到 frontend/public/assets/stardew/ui/ 或建立明确资源管线。
- 每一步保持 npm.cmd run build 可通过。

完成后更新 docs/handoff-roadmap.md 和当天 docs/conversation-handoff-YYYY-MM-DD.md，记录每个路由哪些功能已接 API、哪些只是未来占位。
```

## 风险清单

| 风险 | 处理方式 |
|------|------|
| 把原型四个展示块误当四个页面 | 明确按左侧导航和业务任务拆真实路由 |
| 玩家管理没有后端 API | 保留路由和 UI，占位显示，不造假玩家数据 |
| 面板用户和游戏玩家混淆 | 面板用户放 `settings`，游戏玩家放 `players` |
| 原型功能多于当前后端 | 每个页面分“已接入”和“待接入” |
| 素材放在 docs 导致 build 后丢失 | 复制到 `frontend/public/assets/stardew/ui/` |
| `App.tsx` 继续膨胀 | 登录/初始化留 App，Stardew 页面移入 `StardewPanel` |
| 一次迁移全部业务导致功能回退 | 按四轮实施，小步 build |
| 危险操作确认分散 | 逐步迁移到统一 `ConfirmDialog` |
| 移动端表格溢出 | 窄屏转列表卡片 |
