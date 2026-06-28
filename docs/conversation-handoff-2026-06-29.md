# Conversation Handoff 2026-06-29

## FE-R8: PlayersPage 玩家管理页真实化

### 目标

把 `/instances/stardew/players` 从占位页改造为真实可用的玩家管理页，接入所有现有后端能力，对无后端 API 的功能保持清晰的"待接入"空状态，不写死演示数据。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/PlayersPage.tsx` | 完全重写 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增约 160 行 `sd-players-*` 样式 |
| `docs/handoff-roadmap.md` | Current Context 顶部新增 FE-R8 完成节 |

### 代码探查结论

探查后确认：
1. **无玩家列表 API**：后端无 `/api/instances/:id/players` 接口，Junimo 也无玩家列表 HTTP API。
2. **可接入的真实能力**：`instanceState`（状态）、`dashboardData.saves`（存档/农场信息）、`dashboardData.inviteCode`（邀请码）、`runCommand('info')`（JunimoServer `info` 命令原始输出）。
3. **`info` 命令**：在 commandDefs allowlist 中已存在，描述"查看服务器当前状态、玩家数、存档信息"。服务器运行时调用会返回 attach-cli 输出，含玩家数等信息（输出格式由 Junimo 决定）。

### 各区域说明

**区域 1：玩家概览**
- 服务器状态：从 `instanceState.state` 读取，彩色状态点。
- 在线人数 / 最大人数：当前无 API，显示"—"+ 待接入徽章；解析 `info` 输出太脆弱，不尝试。
- 当前农场 / 主机农民 / 游戏日期：从 `dashboardData.saves.saves` 找活跃存档，展示 `farmName`、`farmerName`、`gameYear/gameSeason/gameDay`。
- 邀请码行：来自 `dashboardData.inviteCode`，带复制和刷新按钮（刷新调 `dashboardData.refreshInviteCode()`）。

**区域 2：服务器信息**
- 自动在服务器首次进入 running 状态时调用 `runCommand('info')`。
- 原始输出显示在深色 `sd-players-info-terminal` 终端框中（像素风暗色背景）。
- 提供"刷新"按钮手动重新获取；服务器未运行时禁用。
- 服务器未运行显示"服务器未运行，暂无服务器信息"。

**区域 3：在线玩家列表**
- 服务器未运行：空状态"服务器未运行，暂无在线玩家"。
- 服务器运行：空状态"玩家列表接口待接入 — JunimoServer 暂未提供玩家列表 API"。
- 占位表头展示未来接入后的列结构（玩家名 / 角色 / 位置 / 在线时长 / 状态 / 操作）。

**区域 4：玩家活动历史**
- 待接入空状态，说明需要日志解析 API 支持。

**区域 5：管理操作**
- 踢出玩家、封禁玩家、白名单管理、权限设置：全部 `disabled` + 待接入徽章。
- 非 admin 用户显示"仅管理员"提示，所有按钮 title 说明原因。
- 未来接入时只需替换 disabled 属性和 click handler。

### 真实接入 API 汇总

| API / 数据 | 接入情况 |
|-----------|---------|
| `instanceState.state` | ✅ 接入 — 控制状态点和区域可用性 |
| `dashboardData.saves.saves` | ✅ 接入 — 活跃存档的农场/农民/日期信息 |
| `dashboardData.inviteCode` | ✅ 接入 — 邀请码展示和复制 |
| `dashboardData.refreshInviteCode()` | ✅ 接入 — 刷新邀请码 |
| `runCommand('info')` | ✅ 接入 — 服务器原始信息输出 |
| 玩家列表 API | ❌ 后端不存在 |
| 踢出/封禁/白名单 API | ❌ 后端不存在 |
| 玩家事件历史 API | ❌ 后端不存在 |

### 影响的接口/文件

- 无新增后端接口，无改动 API 签名。
- `PlayersPage.tsx` 不依赖 `dashboardData.jobs` 或 `dashboardData.mods`。
- `StardewPanel.css` 追加约 160 行，不影响已有类。
- 未改动：`OverviewPage`、`ServerControlPage`、`SavesPage`、`JobsLogsPage`。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，39 模块，JS ~278 kB，CSS ~62 kB
```

已验证通过（exit 0），JS 278.02 kB，CSS 62.53 kB。

手动验证点：
- `/instances/stardew/players` 不再是占位页，显示 5 个区域。
- 服务器停止时：状态点灰色 / 邀请码显示"服务器未运行" / 服务器信息区显示提示 / 玩家列表显示停止状态空态。
- 服务器运行时：状态点绿色脉冲 / 邀请码显示可复制 / 自动触发 `info` 命令并展示原始输出。
- 管理操作按钮全部 disabled，非 admin 有仅管理员提示。
- 左侧导航、Overview、ServerControlPage、SavesPage、JobsLogsPage 不受影响。

### 下一步注意事项

**FE-R9 建议：ModsPage 真实化**
- 已有完整后端 API：`getMods`、`uploadMod`、`deleteMod`、`exportMods`。
- 旧 `ModsSection.tsx` 已实现逻辑（来自 App.tsx 重构前），可迁移为像素风。

**PlayersPage 后续接入建议（给后端开发者）**

若要接入真实玩家列表，后端需要：
1. 解析 JunimoServer 日志或调用 `info` 命令并结构化解析输出（玩家名、在线时长等）。
2. 或接入 JunimoServer HTTP API（需 `API_ENABLED=true`，端口 `API_PORT`，`API_KEY`）。
3. 提供 `GET /api/instances/:id/players` 返回 `{ players: PlayerInfo[] }`。
4. 提供踢出 `POST /api/instances/:id/players/:name/kick`，封禁 `POST /api/instances/:id/players/:name/ban`。
5. 前端只需移除 disabled 属性并补 click handler，UI 入口已就位。

---

## FE-R7a: JobsLogsPage review follow-up

### 目标

补齐 FE-R7 任务日志页提交前 review 发现的细节：避免日志详情仍被后端默认 200 行限制截断，同时让详情加载失败有明确 UI 反馈。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/api.ts` | `getJobLogs(id, after, limit)` 新增 `limit` 参数，默认 1000，并把 `limit` 写入 query |
| `frontend/src/games/stardew/pages/JobsLogsPage.tsx` | 详情加载失败时显示错误条；日志达到 1000 行时显示截断提示；刷新和清空任务时清理详情错误/截断状态 |
| `docs/handoff-roadmap.md` | 新增 FE-R7a 完成记录 |

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 通过
```

## FE-R6a: 存档删除交互修正

### 目标

修正“选中的存档不能删除”的不合理前端逻辑。后端 `DeleteSave`/`DeleteSaveWithBackup` 已支持删除当前启动存档，并会在删除 active save 后清理 gameloader 配置，因此前端只需要做风险提示，不应该隐藏删除入口。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/SavesSection.tsx` | `SaveCard` 不再对 active save 隐藏“删除”按钮；新增 `deleteTitle`；删除确认弹窗根据 `confirmDeleteIsActive` 和 `confirmDeleteIsLastSave` 显示风险提示 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增 `sd-confirm-warning`，用于删除确认弹窗里的黄色警告块 |
| `docs/handoff-roadmap.md` | 新增 FE-R6a 完成记录 |

### 行为说明

- 当前启动存档可以删除。
- 删除当前启动存档时弹窗提示：删除后需要重新选择、创建或上传存档才能再次启动。
- 如果这是最后一个存档，弹窗额外提示：删除后存档列表会变空。
- admin、running/starting、busy 的禁用规则不变。
- 后端接口没有改动。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 通过：tsc -b && vite build
```

## FE-R6: SavesPage 存档管理页真实化（像素视觉迁移）

### 目标

把 `/instances/stardew/saves` 存档管理页从旧样式（`button`、`modal-card` 等 App.css 类）完整迁移为 Stardew 像素主题视觉，保留并强化所有已有功能，补上运行中保护的明确提示。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/SavesSection.tsx` | 全面重写 CSS 类为像素主题；删除确认从 `window.confirm` 改为内联对话框；新增 `onSavesChanged` prop；`selectSaveAndStart`/`createNewGame`/`uploadSaveCommitAndStart` 成功后均先 `await loadSaves()` 再通知上层 |
| `frontend/src/games/stardew/pages/SavesPage.tsx` | `onJobStarted` 新增 `refreshInstanceState` 调用；传 `onSavesChanged={dashboardData.refreshSaves}` |
| `frontend/src/games/stardew/StardewPanel.css` | 新增约 160 行存档页专用样式（`sd-saves-*`、`sd-save-*`、`sd-saves-modal-*`、`sd-saves-preview-*`） |
| `docs/handoff-roadmap.md` | 在 Current Context 顶部新增 FE-R6 完成节 |

### 各改动细节

**SavesSection.tsx 视觉迁移**

- 页头：`sd-saves-header`（flex 两端对齐）+ 左侧标题/警示横条 + 右侧操作按钮组。
- 操作按钮：`sd-btn-green`（创建/启动）、`sd-btn-tan`（刷新/上传/设为启动/取消）、`sd-btn-delete`（删除）。
- 运行保护：服务器 `running`/`starting` 时创建/上传/删除/切换按钮保持可见但 `disabled`，顶部显示金色警示条 `sd-saves-running-hint`，`title` tooltip 说明原因，不再隐藏按钮。
- 存档卡片：`sd-save-card`（活跃存档加 `active` class，绿色边框背景）+ `sd-save-active-tag` 徽章 + `sd-save-meta` 元数据行 + `sd-save-card-actions` 纵向按钮列。
- 删除确认：`window.confirm()` 替换为 `confirmDeleteName` state 驱动的 `sd-confirm-overlay` + `sd-confirm-dialog`，视觉风格与 ServerControlPage 一致。
- 弹窗外壳：新游戏弹窗和上传弹窗都使用 `sd-saves-modal-overlay` + `sd-saves-modal-card`（宽版加 `sd-saves-modal-card-wide`），`NewGameCreator` 内部样式不改。
- 上传预览：`sd-saves-preview-table` + `sd-saves-preview-row` + `sd-saves-preview-label` 替代旧的 `upload-preview-*` 类。
- 空状态：`sd-saves-empty`（虚线边框卡片）+ `sd-saves-empty-title` + `sd-saves-empty-hint`，空状态下也提供创建/上传按钮。

**SavesPage.tsx 更新**

- `onJobStarted` 回调增加 `dashboardData.refreshInstanceState()`，让实例状态立即更新（之前只有 `refreshJobs`）。
- 新增 `onSavesChanged={dashboardData.refreshSaves}`，操作完成后同步刷新公共数据层的存档数据（影响 OpsRail/Overview 中的存档摘要）。

**StardewPanel.css 新增类**

31 个新 CSS 类，全部以 `sd-saves-` 或 `sd-save-` 开头，不影响其他页面。

### 影响的接口/文件

- 无新增后端接口，无改动 API 签名。
- `SavesSection` 新增 `onSavesChanged?: () => void` 可选 prop，向后兼容（不传时静默跳过）。
- 未改动 `NewGameCreator.tsx`、`NewGameCreator.css`、`useStardewDashboardData.ts`、`stardew-routes.ts`。

### 已接入的存档 API

| API 函数 | 状态 |
|----------|------|
| `getSaves` | ✅ 接入 |
| `selectSave` | ✅ 接入 |
| `selectSaveAndStart` | ✅ 接入 |
| `deleteSave` | ✅ 接入 |
| `exportSave` | ✅ 接入 |
| `createNewGame` | ✅ 接入 |
| `uploadSavePreview` | ✅ 接入 |
| `uploadSaveCommitAndStart` | ✅ 接入（含取消/清理 token） |
| `getSavesPreflight` | ❌ 未接入（如后端 API 可用可补） |

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，39 模块，JS ~261 kB
```

已验证通过（exit 0），JS bundle 261.21 kB，CSS 54.56 kB。

手动验证点：
- `/instances/stardew/saves` 显示像素风存档列表（或像素风空状态）。
- 活跃存档卡片绿色边框 + 「当前」绿色徽章。
- 每张存档卡片展示：存档名、农场名、农民名、游戏时间（年/季/日）、地图类型、文件大小、修改时间，解析失败时显示红色错误条。
- 点击「创建存档」弹出完整 `NewGameCreator` 自定义存档弹窗（三列农场选择、角色/宠物/小屋等字段全保留）。
- 点击「上传存档」弹出 ZIP 上传 → 预览确认流程。
- 点击「删除」弹出像素风二次确认对话框（`sd-confirm-dialog`），取消/确认均正常。
- 服务器运行中时，创建/上传/删除/切换按钮全部 disabled，顶部显示金色警示条。
- 导出存档正常触发浏览器下载。
- 左侧导航、顶部栏、Overview、ServerControlPage 不受影响。

### 下一步注意事项

- `JobsLogsPage` 已完成（FE-R7），见下一节。
- `SavesSection` 内部有自己的 `loadSaves()`，与 `dashboardData.saves` 并行维护，属于可接受的轻微重复请求。后续可通过 `refreshTrigger` 或传入 `dashboardData.saves` 替代内部请求。
- 备份恢复 UI 尚未接入（后端 `GET /api/instances/:id/backups` 等 API 已存在），可作为存档页后续增强。

---

## FE-R7: JobsLogsPage 任务与日志页真实化

### 目标

把 `/instances/stardew/jobs` 从占位页改造为完整可用的任务与日志页，支持 SSE 实时日志、任务列表选择、状态展示，视觉完全融入 Stardew 像素风。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/JobsLogsPage.tsx` | 完全重写：任务列表 + 详情 + SSE 日志流 + 清空确认 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增约 220 行 `sd-jobs-*` 任务页专用样式 |
| `docs/handoff-roadmap.md` | Current Context 顶部新增 FE-R7 完成节 |

### 各改动细节

**JobsLogsPage.tsx 重写**

- **任务列表（左列 220px）**：`sd-jobs-list`，每行显示任务类型（`TYPE_LABELS` 中文映射）、创建时间、状态徽章，点击切换选中任务，活跃行左侧金色 3px 边框。
- **任务详情（右列 1fr）**：`sd-jobs-detail`，显示类型、ID 缩略、创建/开始/结束时间。
- **错误展示**：`failed` 任务的 `errorMessage` 以 `sd-jobs-error-banner-prominent`（红色双边框）突出，不干扰整体布局。
- **日志终端**：`sd-jobs-log-window`（深色 `#1e1209` 背景，等宽字体），每行显示序号/level/message，按 level 着色（info=绿/warn=金/error=红/debug=蓝）。
- **安装进度条**：`stardew_install` 类型任务解析 `[pull:progress:done:total]` 日志，渲染进度条，与旧 `JobsSection` 逻辑一致。
- **状态标签中文**：`queued=排队中 / running=运行中 / succeeded=已完成 / failed=失败 / canceled=已取消`。
- **工具栏**：刷新按钮（所有用户）+ 清空任务历史（admin 且有任务时显示，`sd-confirm-overlay` 像素风二次确认）。
- **三态 UI**：加载中（`sd-jobs-loading`）、空任务（`sd-jobs-empty`）、选中（详情区）、未选中（`sd-jobs-select-hint`）。
- **默认自动选中最近任务**：初始加载后用 `autoSelectedRef` 防止重复触发，首次加载选中 `jobs[0]`。

**SSE 实时日志实现**

- 在 `selectedJobId` 变化的 `useEffect` 中，加载详情+日志后判断是否终态：
  - 非终态 → `createJobEventSource(id, lastSeq)` 开启 SSE。
  - `log` 事件 → `appendUniqueLog`（按 `jobId+sequence` 去重）追加到 `logs` state。
  - `finished` 事件 → `es.close()` → `getJob(id)` 刷新详情 → `loadJobs()` 刷新本地列表 → `dashRefreshJobs()` 刷新 OpsRail → `refreshInstanceState()` 同步全局实例状态。
  - `onerror` → 显示金色警示条 `sd-jobs-sse-notice-warn`，关闭 EventSource，保留手动刷新。
- `useEffect` cleanup 函数：`cancelled = true` + `es.close()`，防止组件卸载/任务切换后仍追加 stale 日志。
- 日志区底部 `ref={logEndRef}`，`logs.length` 变化时 `scrollIntoView`。

**StardewPanel.css 新增类（约 220 行）**

布局类：`sd-jobs-toolbar`、`sd-jobs-toolbar-actions`、`sd-jobs-layout`（grid 220px/1fr）、`sd-jobs-list`、`sd-jobs-list-row`（active 左金 3px 边框）、`sd-jobs-detail`

样式类：`sd-jobs-detail-head/title/id/meta`、`sd-jobs-error-banner/prominent`、`sd-jobs-sse-notice/warn`、`sd-jobs-sse-dot`（脉冲绿点动画）、`sd-jobs-pull-progress`、`sd-jobs-progress-wrap/track/fill/pct`、`sd-jobs-log-window`、`sd-jobs-log-line/seq/level/msg`（level 着色）、`sd-jobs-empty/loading/select-hint`

状态徽章：`sd-jobs-status` + `sd-jobs-status-queued/running/succeeded/failed/canceled`（5 色）

### 影响的接口/文件

- 无新增后端接口，无改动 API 签名。
- `JobsLogsPage.tsx` 不再依赖 `dashboardData.jobs`（独立维护本地 `jobs` state），但 SSE `finished` 时同步调用 `dashRefreshJobs()` 保持 OpsRail 数据一致。
- `StardewPanel.css` 追加约 220 行，不影响已有类。

### 接入的 API

| API 函数 | 路径 | 用途 |
|----------|------|------|
| `getJobs()` | `GET /api/jobs` | 任务列表 |
| `getJob(id)` | `GET /api/jobs/:id` | 单任务详情 |
| `getJobLogs(id, 0)` | `GET /api/jobs/:id/logs?after=0` | 全量日志加载 |
| `createJobEventSource(id, seq)` | `GET /api/jobs/:id/stream?after=seq` | SSE 实时日志流 |
| `clearJobs()` | `DELETE /api/jobs` | 清空任务历史 |

### FE-R6 小修复确认

本轮代码审查确认以下三项 FE-R6 要求均已在上一轮实现，本轮无需改动：

1. **非 admin 按钮保护**：`SaveCard` 中 `writeDisabled = busy || isRunning || !isAdmin`，「设为启动存档 / 选择并启动 / 删除」按钮非 admin 时 disabled，不可点击至 403。
2. **空状态按钮 running 时可见 disabled**：`sd-saves-empty-actions` 中按钮 `disabled={busy || isRunning}`，服务器运行时 disabled+title，不隐藏。
3. **删除确认受 isAdmin 约束**：`sd-confirm-dialog` 中确认按钮 `disabled={busy || isRunning || !isAdmin}`。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，39 模块，JS ~269 kB，CSS ~59 kB
```

已验证通过（exit 0），JS 269.14 kB，CSS 59.57 kB。

手动验证点：
- `/instances/stardew/jobs` 不再是占位页，显示任务列表（左）+ 详情（右）两列布局。
- 刚从 SavesPage/ServerControlPage 触发任务后跳转，能立即看到最新任务（自动选中第一条）。
- 选中 `running` 任务后日志区底部显示「实时接收日志中…」绿色脉冲点，SSE 日志追加。
- `finished` 事件后状态徽章更新为「已完成」或「失败」。
- `failed` 任务的错误信息以红色双边框横条展示。
- 安装任务显示「拉取镜像」进度条。
- admin 用户可见「清空任务历史」按钮，确认弹框像素风，取消/确认均正常。
- 非 admin 用户不显示「清空任务历史」按钮。
- SSE 断线后金色提示条出现，手动刷新可更新日志。
- 左侧导航、Overview、ServerControlPage、SavesPage 不受影响。

### 下一步建议

- **FE-R8（或后续）**：`PlayersPage` 真实化——接入玩家列表（需确认后端 API 是否已有）。
- **FE-R9**：`ModsPage` 真实化——接入已有的 `getMods/uploadMod/deleteMod/exportMods` API，迁移旧 `ModsSection`。
- **JobsSection 旧组件**：`JobsSection.tsx` 已被本页取代，后续可考虑删除或归档（仍有 App.css 依赖，谨慎删除）。
- **存档备份恢复 UI**：后端 `GET /api/instances/:id/backups` 已存在，可在 `SavesPage` 增加备份列表和恢复入口。
- **InstallPage 安装向导**：安装流程产生的任务（`stardew_install`）需要在 `JobsLogsPage` 的 SSE 里正确展示 Steam Guard 提示——当前页面显示进度条和日志，但 Steam Guard 交互仍在 `InstallPage`，两者互不干扰。
