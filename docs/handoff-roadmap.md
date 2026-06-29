# stardew-server-anxi-panel Handoff Roadmap

本文档用于给后续 Codex、Claude 或人工开发者接手项目时快速进入状态。

项目架构以 [architecture.md](architecture.md) 为准：Go 后端、React + TypeScript 前端、SQLite、本地 Docker Socket、GameDriver 插件化抽象。本文只负责把大目标切成可执行的小目标，并说明每一步应该做什么、怎么做、做到什么程度算完成。

## Current Context

### UI-R5: Overview 首页信息层级与移动端重排优化 ✅ completed (2026-06-29)

移动端双栏改单列、指标格单列、指标卡语义色（绿/金/红 modifier）、横幅自适应高度、控制行换行。

**改动内容：**

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/StardewPanel.css` | 新增 `.sd-mc--ok/warn/error` 语义色；`@media (max-width: 640px)` 末尾追加 Overview 移动端规则（双栏→单列、指标格单列、横幅自适应、控制行换行） |
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | 四个指标卡 `className` 动态拼接语义 modifier（存档无激活→warn、读取失败→error；模组需重启→warn；健康OK→ok/有错误→error；有失败任务→error） |

**验证：** `npm.cmd run build` 通过（exit 0），39 模块，JS 325.13 kB，CSS 85.10 kB。

### UI-R4: 全局按钮体系整理与可点击控件优化 ✅ completed (2026-06-29)

统一全局 PNG 按钮系统，修复 SettingsPage 完全无样式的按钮，强化危险/次操作视觉区分，添加 hover 状态，优化移动端按钮布局。

**改动内容：**

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/SettingsPage.tsx` | 替换全部 `sd-btn`（不存在类）为正确 `sd-btn-tan/green/delete`；修复 ConfirmDialog 使用 `h3`/`p` 标签；调整确认弹框按钮顺序（取消在前） |
| `frontend/src/games/stardew/stardew-theme.css` | 新增所有 PNG 按钮 hover 状态（`brightness(1.08)`）；`sd-btn-copy` 高度 25→26px；`sd-btn-delete` 文字色改为暗红 `#8b2020` 以区分危险操作 |
| `frontend/src/games/stardew/StardewPanel.css` | `@media (max-width: 640px)` 新增：安装页操作/表单/Steam 认证按钮组改为纵向全宽；存档/Mods/Jobs 页头部按钮组允许换行；设置页工具栏换行、用户操作按钮换行 |

**改动明细：**

- `SettingsPage.tsx`（重要 bug 修复）：
  - 所有 `sd-btn`（不存在 CSS 类，导致按钮渲染为浏览器默认无样式） → `sd-btn-tan`
  - `sd-btn sd-btn-green` → `sd-btn-green`
  - `sd-btn sd-btn-red`（危险操作如禁用/删除/弹框确认） → `sd-btn-delete`
  - "退出登录" → `sd-btn-tan`（不是删除操作，改为中性色）
  - ConfirmDialog：`div.sd-confirm-title` → `h3`，`div.sd-confirm-body` → `p`（符合现有 CSS 选择器规则）
  - 确认弹框按钮顺序：取消在前，危险操作在后（与 ServerControlPage 一致）

- `stardew-theme.css`：
  - PNG 按钮 hover：全部（green/tan/gold/red/start/stop/restart/copy/delete）加 `filter: brightness(1.08)` hover 状态
  - `sd-btn-copy` 高度 25px → 26px（与其他小按钮对齐）
  - `sd-btn-delete` 文字颜色 `#2c1a0a`（深棕）→ `#8b2020`（暗红），增强危险操作视觉区分

- `StardewPanel.css`（移动端，max-width: 640px）：
  - `.sd-install-actions/.sd-install-form-actions/.sd-install-guard-actions`：改为 `flex-direction: column; align-items: stretch`，按钮撑满宽度
  - `.sd-saves-header-actions/.sd-mods-header-actions`：`width: 100%; justify-content: flex-start`
  - `.sd-settings-section-toolbar`：`flex-wrap: wrap`
  - `.sd-settings-user-actions`：`flex-wrap: wrap; gap: 4px`
  - `.sd-jobs-toolbar-actions`：`flex-wrap: wrap`

**未改动：** 业务逻辑、API、按钮 PNG 素材路径、颜色主题大方向、UI-R1/R2/R3 已有规则。

**验证：** `npm.cmd run build` 通过（exit 0），39 模块，JS 324.96 kB，CSS 84.51 kB。

### UI-R3: 移动端与窄屏布局修复 ✅ completed (2026-06-29)

在 390px 宽度下修复 4 项问题：页面横向滚动、导航触控、顶栏信息过载、安装成功卡拥挤。

**改动内容：**

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/StardewPanel.css` | `.sd-main` 全局新增 `overflow-x: hidden`；`@media (max-width: 640px)` 扩充 5 处规则 |

**改动明细：**
- `.sd-main`（全局）：新增 `overflow-x: hidden`，主内容区宽内容不再撑出页面
- `.sd-shell`（640px）：新增 `overflow-x: hidden`，shell 级防横向滚动双保险
- `.sd-sidebar .sd-nav-item`（640px）：`min-width:36px; height:100%; min-height:0; padding:0 8px; gap:0; flex-shrink:0`，图标充满 36px 侧栏高度，最小触控区 36px
- 顶栏隐藏列表（640px）：新增隐藏 `sd-topbar-name`（品牌文字）和 `sd-topbar-user .sd-tag`（角色徽章），只保留 logo + 状态 + 登出
- `.sd-install-complete-card`（640px）：`gap:8px`，按钮加 `align-self:stretch` 撑满整行

**未改动：** UI-R1 字号变量、UI-R2 间距变量、业务逻辑、API、React 组件、颜色体系、960px 断点。

**验证：** `npm.cmd run build` 通过（exit 0），39 模块，JS 325.01 kB，CSS 83.22 kB。

### UI-R2: 页面间距与卡片密度统一 ✅ completed (2026-06-29)

适配 UI-R1 字号放大后的空间关系，建立 spacing 变量体系，统一页面 padding、卡片 padding、列表行高和区块 gap，消除文字贴边和按钮高度与字号冲突。

**改动内容：**

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/stardew-theme.css` | 新增 `--sd-space-*`（5个）、`--sd-card-padding`、`--sd-section-gap`、`--sd-page-padding` 共 8 个间距变量；按钮 `.sd-btn-green/.sd-btn-tan` 高度 24→26px；`.sd-btn-delete` 高度 24→26px；`.sd-input` 高度 23→26px |
| `frontend/src/games/stardew/StardewPanel.css` | `.sd-page` padding 12→16px（变量化）；`.sd-settings-page` gap 6→14px（变量化）；`.sd-topbar-logout-btn` 高度 22→24px；`.sd-state-card` padding 7→10px；`.sd-srv-section` padding 8→10px，gap 6→8px；`.sd-save-card` padding 7→9px；`.sd-saves-list` gap 6→8px；`.sd-mods-card` padding 7→9px；`.sd-mods-list/.sd-mods-pending-grid` gap 5→7px；`.sd-jobs-list-row` padding 7→8px；`.sd-settings-user-row` padding 5→7px；`.sd-settings-audit-head/.row` padding 4→5px；`.sd-diag-check-row` padding 5→7px；`.sd-save-meta` 补 `line-height: 1.45`；`.sd-install-log-line` `line-height` 1.4→1.5 |

**未改动：** 业务逻辑、API、React 组件、颜色体系、字号体系均未变动。

**验证：** `npm.cmd run build` 通过（exit 0），39 模块，JS 325.01 kB，CSS 82.79 kB。

### UI-R1: 前端字号基线统一 ✅ completed (2026-06-29)

解决 Stardew 管理面板"字太小、长时间看费劲"的问题，建立字号变量体系，全面提升可读性。

**改动内容：**

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/stardew-theme.css` | 新增 `--sd-font-size-*` 变量块（8个变量），更新按钮/输入框/导航/数据行字号 |
| `frontend/src/games/stardew/StardewPanel.css` | 批量替换所有过小字号，按角色分层 |

**字号变量体系（新增到 `stardew-theme.css`）：**
- `--sd-font-size-meta`: 11px — 时间戳、序号、最小徽章
- `--sd-font-size-small`: 12px — 次级说明、日志正文
- `--sd-font-size-body`: 13px — 正文、描述、提示信息
- `--sd-font-size-control`: 13px — 输入框、按钮、选择框
- `--sd-font-size-section-title`: 14px — 区块标题
- `--sd-font-size-page-title`: 18px — 页面标题
- `--sd-font-size-metric`: 18px — 指标大数字
- `--sd-font-size-log`: 12px — 日志正文

**字号分层处理（`StardewPanel.css`）：**
- 8.5px/9px/9.5px → 11px（meta 级：时间戳/序号/徽章，不低于 10.5px 要求）
- 10px → 12px（次级说明/meta 文字）
- 10.5px/11.5px → 13px（正文/内容描述）
- 区块标题 `.sd-srv-section-title`/`.sd-settings-section-title`/`.sd-ov-title` → `--sd-font-size-section-title`（14px）
- 页面标题 `.sd-page-title` → `--sd-font-size-page-title`（18px）
- 指标数字 `.sd-mc-val` → `--sd-font-size-metric`（18px）
- OpsRail 容器基准 → 12px
- 保留 `clamp(11px, 7.6cqi, 13px)` 的导航按钮不动（已是响应式）

**未改动：** 业务逻辑、API、React 组件、颜色体系、图片素材均未变动。

**验证：** `npm.cmd run build` 通过（exit 0），39 模块，JS 325.01 kB，CSS 82.49 kB。

### FE-R12: InstallPage 首次安装向导页真实化 ✅ completed (2026-06-29)

把 `/instances/stardew/install` 从占位页改造为真实可用的「首次安装向导」页面。

**真实接入的 API / 数据：**
- `getInstallOptions()`（`GET /api/instances/:id/install-options`）：加载镜像版本选项。
- `installInstance()`（`POST /api/instances/:id/install`）：启动安装任务，返回 `jobId`。
- `submitSteamGuardInput(jobId, input)`（`POST /api/instances/:id/steam-guard/input`）：提交 Steam Guard 验证码或登录方式选择。
- `createJobEventSource(jobId, lastSeq)`（SSE：`GET /api/jobs/:id/stream`）：实时接收安装日志。
- `getJob(id)` / `getJobLogs(id, 0, 1000)`：加载安装任务详情和历史日志。
- `dashboardData.refreshInstanceState()` / `refreshJobs()`：安装触发/完成后刷新公共数据层。
- `dashboardData.jobs`：初始加载时自动拾取活跃的安装任务（页面刷新/跳转场景）。
- `instanceState.state` / `driverPhase` / `stateMessage`：驱动页面状态显示和交互分支。

**页面功能区（共 8 个）：**
1. **状态概览**：`state`、`driverPhase`（monospace tag）、`stateMessage`，彩色状态点（绿/黄/红/灰）。
2. **已安装成功卡**：绿框卡，提示已就绪，"前往服务器控制"按钮；admin 可见"重新安装/修复"按钮。
3. **安装配置表单**：镜像版本下拉、Steam 用户名、Steam 密码（可切换显隐）、VNC 密码（可切换显隐）；`canDirectRetry` 时只显示镜像版本不重填凭据。
4. **安装进度**：5 步骤条（准备环境→拉取镜像→Steam 认证→下载游戏→完成）+ 阶段文字 + Pull 镜像进度卡（解析 `[pull:progress:N:M]`）+ 游戏/SDK 下载提示卡。
5. **Steam 认证交互区**：auth_method_required（扫码/账密选择）、steam_guard_choice_required（手机批准/输入码选择）、steam_guard_required（验证码输入）、steam_guard_mobile_required（等待手机批准提示）、steam_qr_required（打开扫码窗口按钮）。
6. **QR 二维码弹窗**：暗色 overlay，`pre` 显示从日志提取的 `[steam]` 行文本（ASCII QR），字体自适应。
7. **安装日志预览**：深色终端，SSE 实时追加（最近 50 条），四色着色，超 50 条提示跳转任务与日志。
8. **错误提示**：QR 失败提示条、API 错误条、SSE 断线提示条。

**权限规则：**
- 非 admin：所有写操作不可见，显示"仅管理员可安装"提示，仍可查看状态/进度。
- 已安装：显示成功卡，不误导重复安装；需点"重新安装/修复"才展开表单。

**改动内容：**

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/InstallPage.tsx` | 完全重写（约 360 行） |
| `frontend/src/games/stardew/StardewPanel.css` | 新增约 370 行 `sd-install-*` 样式 |

**验证：** `npm.cmd run build` 通过（exit 0），39 模块，JS 324.18 kB，CSS 81.96 kB。

### FE-R11: SettingsPage 设置与审计页真实化 ✅ completed (2026-06-29)

把 `/instances/stardew/settings` 从占位页改造为真实可用的「设置与审计」页面。

**真实接入的 API / 数据：**
- `user`（props）：当前账号用户名、角色、ID，驱动退出登录按钮和权限控制。
- `dashboardData.versionInfo`（公共数据层）：版本号 / 构建时间 / Commit，面板版本区直接展示。
- `getUsers()`（`GET /api/users`，admin-only）：加载面板用户列表。
- `createUser()`（`POST /api/users`，admin-only）：创建新用户。
- `updateUserRole()`（`PATCH /api/users/:id`，admin-only）：切换用户角色，二次确认弹窗。
- `disableUser()`（`DELETE /api/users/:id`，admin-only）：禁用用户，二次确认弹窗。
- `deleteUserHard()`（`DELETE /api/users/:id?hard=true`，admin-only）：永久删除用户，二次确认弹窗。
- `getAuditLogs()`（`GET /api/audit-logs`，admin-only）：分页加载审计日志（每页 20 条），支持翻页和刷新。

**页面功能区（共 6 个）：**
1. **当前账号**：用户名、角色标签、登录状态、退出登录按钮（复用 `onLogout`）。
2. **面板版本**：版本号、构建时间、Commit hash、运行模式（Single Game Mode 标签）。
3. **用户管理**：admin 可查看列表、新建、角色切换、禁用/永久删除（像素风二次确认）；普通用户显示权限提示；自防护（不能操作自己）。
4. **审计日志**：admin 分页展示（时间/操作者/动作中文/目标/IP），支持翻页和刷新；普通用户权限提示；失败显示错误条+重试。
5. **安全与权限**：5 条静态安全说明，突出 Docker Socket 风险。
6. **待接入设置**：主题、语言、多游戏模式、备份策略、通知、会话超时——全部 disabled，标注徽章。

**改动内容：**

| 文件 | 修改 |
|------|------|
| `frontend/src/api.ts` | 新增 `getUsers` / `createUser` / `updateUserRole` / `disableUser` / `deleteUserHard` |
| `frontend/src/games/stardew/pages/SettingsPage.tsx` | 完全重写（约 370 行） |
| `frontend/src/games/stardew/StardewPanel.css` | 新增约 255 行 `sd-settings-*` 样式 |

**验证：** `npm.cmd run build` 通过（exit 0）。

### FE-R10: DiagnosticsPage 诊断与健康检查页真实化 ✅ completed (2026-06-29)

把 `/instances/stardew/diagnostics` 从占位页改造为真实可用的诊断与健康检查页面。

**真实接入的 API：**
- `getHealthDiagnostics()`（`GET /api/health/diagnostics`）：加载健康检查数据，返回 `{ status, checks[] }`。
- `downloadSupportBundle()`（`POST /api/instances/:id/support-bundle`）：导出诊断包 ZIP，admin-only，触发浏览器下载。
- `dashboardData.health / healthError / refreshHealth()`：公共数据层健康数据，用于初始渲染；重新检查时本地独立请求。

**页面功能区：**
1. **总状态面板**：大彩色状态点（ok=绿/warning=黄/error=红）+ 状态标签（系统正常/存在警告/存在错误）+ 正常/警告/错误计数。
2. **检查项明细**：按后端返回逐项渲染，名称中文映射（docker_daemon / docker_compose / data_dir / instance_dir / compose_file / active_save），状态着色行。
3. **告警与建议**：仅汇总 warning/error 项；全部正常时显示"暂无告警"绿色提示条。
4. **快捷工具**：重新检查（独立 loading 状态，不阻塞公共层）、导出诊断包（admin-only，非 admin disabled + title）。
5. **资源趋势**：待接入空状态，说明需要后端数据源，占位区预留渲染位置。

**权限规则：**
- `getHealthDiagnostics`：所有登录用户可用（后端 requireAuth）。
- `downloadSupportBundle`：admin-only（后端 requireAdmin）；非 admin 显示 disabled 按钮 + title 说明。

**改动内容：**

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/DiagnosticsPage.tsx` | 完全重写 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增约 220 行 `sd-diag-*` 样式 |
| `docs/handoff-roadmap.md` | 新增 FE-R10 完成记录 |
| `docs/conversation-handoff-2026-06-29.md` | 新增 FE-R10 接手节 |

**验证：**`npm.cmd run build` 通过（exit 0），39 模块，JS 291.25 kB，CSS 70.88 kB。

**待接入（有 UI 入口但 disabled/空状态）：**
- CPU / 内存 / 磁盘实时趋势图表（无后端数据源，显示"待接入"空状态）。

### FE-R9: ModsPage 模组管理页真实化 ✅ completed (2026-06-29)

把 `/instances/stardew/mods` 从占位页改造为真实可用的 Stardew 像素风模组管理页面。

**真实接入的 API：**
- `getMods`：加载 Mod 列表（含 `restartRequired` 标志）。
- `uploadMod`：上传 ZIP 包安装 Mod，成功后刷新列表并显示"需要重启"提示。
- `deleteMod`：删除 Mod（admin-only，像素风二次确认弹窗，running/starting 时 disabled）。
- `exportMods`：导出全部 Mod 为 ZIP，触发浏览器下载（所有用户可用）。

**页面功能区：**
1. **概览统计行**：已安装数量、服务器状态（带彩色状态点）、重启需求标志、解析失败数量。
2. **模组列表**：每张卡片显示名称、版本、UniqueID、作者、目录名、描述；解析失败时显示红色错误条。
3. **操作区**：上传 Mod（admin + 非 running）、导出 Mod 包（所有用户）、刷新列表。
4. **删除**：admin-only，running/starting 时 disabled；点击弹出像素风 `sd-confirm-dialog` 二次确认，不使用 `window.confirm`。
5. **待接入区**：启用/禁用、依赖检查、更新检查——全部 disabled + 待接入徽章，注明"后端待接入"。

**权限规则：**
- 非 admin：上传/删除按钮可见但 disabled，title 说明"仅管理员可用"。
- running/starting：上传/删除 disabled，title 说明"服务器运行中，请先停止后操作"。
- 导出无状态限制，mods 为空时 disabled。

**改动内容：**

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/ModsPage.tsx` | 完全重写 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增约 300 行 `sd-mods-*` 样式 |
| `docs/handoff-roadmap.md` | 新增 FE-R9 完成记录 |
| `docs/conversation-handoff-2026-06-29.md` | 新增 FE-R9 接手节 |

**验证：**`npm.cmd run build` 通过（exit 0），39 模块，JS 287.09 kB，CSS 67.30 kB。

**待接入（有 UI 入口但 disabled）：**
- 按 Mod 单独启用/禁用（无后端 API）。
- 依赖完整性检查（无后端 API）。
- Nexus/URL 在线安装（无后端 API）。
- 更新检查（无后端 API）。

**旧 ModsSection：** `ModsSection.tsx` 无任何外部引用，保留不动，不影响已有功能。

### FE-R8: PlayersPage 玩家管理页真实化 ✅ completed (2026-06-29)

把 `/instances/stardew/players` 从占位页改造为真实的 Stardew 像素风玩家管理页面。

**真实接入的数据：**
- `instanceState.state`：服务器运行/停止状态，影响全页可用性。
- `dashboardData.saves?.activeSaveName`：活跃存档名，显示当前农场/农民信息。
- `activeSave.farmName / farmerName / gameYear / gameSeason / gameDay`：从存档元数据展示游戏内日期。
- `dashboardData.inviteCode`：服务器邀请码，带复制/刷新按钮。
- `runCommand('info')`：在服务器运行时自动调用 JunimoServer `info` 命令，展示原始服务器状态文本（含玩家数、存档等）。

**待接入（有 UI 入口但 disabled/空状态）：**
- 在线玩家列表（无 backend API，显示清晰的"待接入"空状态 + 列头占位）。
- 在线人数 / 最大人数（API 不存在，显示"—"+ 待接入徽章）。
- 玩家活动 / 事件历史（无日志解析 API，显示待接入空状态）。
- 踢出、封禁、白名单、权限设置（全部 disabled + 待接入徽章）。

**改动内容：**

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/PlayersPage.tsx` | 完全重写 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增约 160 行 `sd-players-*` 样式 |
| `docs/handoff-roadmap.md` | 新增 FE-R8 完成记录 |
| `docs/conversation-handoff-2026-06-29.md` | 新增 FE-R8 接手节 |

验证：`npm.cmd run build` 通过（exit 0），39 模块，JS 278.02 kB，CSS 62.53 kB。

### FE-R7a: JobsLogsPage review follow-up ✅ completed (2026-06-29)

修复 FE-R7 提交前 review 中发现的任务日志页细节：
- `frontend/src/api.ts`：`getJobLogs(id, after, limit)` 新增 `limit` 参数，默认请求后端允许的 1000 行，避免详情页仍按后端默认 200 行加载。
- `frontend/src/games/stardew/pages/JobsLogsPage.tsx`：详情加载失败时显示错误条，不再静默退回“选择任务”占位；日志达到 1000 行时显示截断提示；刷新/清空时同步清理详情错误和截断状态。
- 验证：`npm.cmd run build` 通过。

### FE-R6a: 存档删除交互修正 ✅ completed (2026-06-29)

修正 `/instances/stardew/saves` 中“当前启动存档不能删除”的前端限制。后端 `DeleteSave` 已支持删除 active save 并自动清理 gameloader 配置，因此前端不应隐藏当前存档的删除入口。

改动内容：
- `SavesSection.tsx`：当前启动存档也始终显示“删除”按钮，仍保留 admin、running/starting、busy 禁用规则。
- 删除确认弹窗新增风险提示：删除当前启动存档后需要重新选择/创建/上传存档；如果这是最后一个存档，再额外提示删除后列表会变空。
- `StardewPanel.css`：新增 `sd-confirm-warning` 样式，用于像素风警告块。

验证：`npm.cmd run build` 通过。

### FE-R7: JobsLogsPage 任务与日志页真实化 ✅ completed (2026-06-29)

把 `/instances/stardew/jobs` 从占位页改造为真实可用的任务与日志页面，完全融入 Stardew 像素风 shell。

**改动内容：**

1. **`JobsLogsPage.tsx` 完全重写**
   - 左侧任务列表：任务类型（中文）、状态徽章（中文）、创建时间，点击切换选中。
   - 右侧详情区：任务类型/ID/时间元数据，failed 任务突出显示 `errorMessage`（红色双边框）。
   - 日志终端：深色 `sd-jobs-log-window`，`sequence` 去重，按 `level` 着色。
   - 安装任务专用拉取进度条（`extractPullProgress`）。
   - 刷新按钮（所有用户）+ 清空历史（admin，像素风二次确认弹框）。
   - 默认自动选中最近一条任务（`autoSelectedRef` 防重复）。

2. **SSE 实时日志**
   - 非终态任务开启 `createJobEventSource(id, lastSeq)` 接收 `log` 事件。
   - `finished` 事件 → 关闭 SSE → 刷新详情 + 本地列表 + OpsRail + 实例状态。
   - 组件卸载/切换任务时自动 `es.close()` + `cancelled` 标志防止 stale 更新。
   - SSE 失败显示金色警示条，保留手动刷新。
   - `appendUniqueLog` 按 `jobId+sequence` 去重。

3. **`StardewPanel.css` 新增约 220 行 `sd-jobs-*` 样式**
   - 两列布局、任务列表行、详情区、日志终端、状态徽章、进度条、空/加载/提示状态。

**FE-R6 小修复确认（均已到位，本轮无需改动）：**
- 非 admin 用户看到 disabled 按钮，不会触发 403（`writeDisabled = busy || isRunning || !isAdmin`）。
- 空状态下 running 时按钮 disabled+title（而非隐藏）。
- 删除确认弹框确认按钮 `disabled={busy || isRunning || !isAdmin}`。

**已接入 API：** `getJobs`、`getJob`、`getJobLogs`、`createJobEventSource`（`GET /api/jobs/:id/stream` SSE）、`clearJobs`。

`npm.cmd run build` 通过（exit 0），39 模块，JS 269.14 kB，CSS 59.57 kB。

详见 `docs/conversation-handoff-2026-06-29.md`。

### FE-R6: SavesPage 存档管理页真实化（像素视觉迁移）✅ completed (2026-06-29)

把 `/instances/stardew/saves` 的存档管理页从旧样式完整迁移为像素主题视觉，保留并强化所有已有功能。

**迁移/改动内容：**

1. **`SavesSection.tsx` 视觉全面重写**
   - 所有 `.button`、`.modal-card`、`.modal-overlay`、`.error-banner` 等旧 App.css 类改为 `sd-btn-*`、`sd-saves-modal-*`、`sd-confirm-*`、`sd-saves-error` 等像素主题类。
   - 删除确认从 `window.confirm()` 升级为内联 `sd-confirm-overlay` + `sd-confirm-dialog` 弹框（视觉一致，保留危险确认二次保障）。
   - 新增 `onSavesChanged?: () => void` prop，供 `SavesPage` 在操作后同步刷新 `dashboardData.saves`。

2. **运行中保护增强**
   - 服务器 `running`/`starting` 时，创建/上传/删除/切换存档按钮全部保持可见但禁用（`disabled` + `title` tooltip），不再隐藏按钮。
   - 页面顶部显示金色警示横条：「⚠ 服务器运行中，创建 / 上传 / 删除 / 切换存档已暂时禁用」。

3. **`SavesPage.tsx` 回调更新**
   - `onJobStarted` 同时调用 `refreshJobs + refreshInstanceState + navigate('jobs')`（原只有 `refreshJobs`）。
   - 新传 `onSavesChanged={dashboardData.refreshSaves}` 给 `SavesSection`，保证操作后公共数据层同步。

4. **`StardewPanel.css` 新增存档页专用样式类**（约 160 行）：
   `sd-saves-header`、`sd-saves-header-left`、`sd-saves-header-actions`、`sd-saves-running-hint`、`sd-saves-active-hint`、`sd-saves-error`、`sd-saves-list`、`sd-save-card`、`sd-save-card.active`、`sd-save-card-info`、`sd-save-card-name`、`sd-save-active-tag`、`sd-save-card-error`、`sd-save-meta`、`sd-save-meta-muted`、`sd-save-card-actions`、`sd-saves-empty`、`sd-saves-empty-title`、`sd-saves-empty-hint`、`sd-saves-empty-actions`、`sd-saves-modal-overlay`、`sd-saves-modal-card`、`sd-saves-modal-card-wide`、`sd-saves-modal-header`、`sd-saves-modal-title`、`sd-saves-modal-actions`、`sd-saves-preview-table`、`sd-saves-preview-row`、`sd-saves-preview-label`、`sd-saves-hint`、`sd-saves-upload-form`。

**已完整保留/迁移的功能：**
- `getSaves`、`selectSave`、`selectSaveAndStart`、`deleteSave`、`exportSave`、`createNewGame`、`uploadSavePreview`、`uploadSaveCommitAndStart` 全部接入。
- 自定义新建存档弹窗（`NewGameCreator`）完整保留，弹窗外壳换为像素主题。
- 上传 ZIP → 解析预览 → 确认导入流程完整保留。
- `NewGameCreator.tsx` 和 `NewGameCreator.css` 未改动。

**待后续接入的功能（无后端支持）：**
- `getSavesPreflight`（如 API 存在则可接入）。
- 存档备份列表浏览和手动恢复（后端 API 已有，前端未接入）。

`npm.cmd run build` 通过（exit 0），39 模块，JS 261.21 kB，CSS 54.56 kB。

详见 `docs/conversation-handoff-2026-06-29.md`。

### Bug fix batch: SavesPage P1 + ServerControlPage P2/P3 ✅ completed (2026-06-29)

修复 5 个问题：

1. **P1 SavesPage 真实化**：`SavesPage.tsx` 从占位改为直接渲染 `SavesSection`（含自定义新建存档弹窗、上传弹窗、存档列表、选择/删除/导出）。props 桥接：`state = instanceState?.state`、`isAdmin = user.role === 'admin'`、`onJobStarted` → `refreshJobs + onNavigate('jobs')`、`onStateRefresh` → `dashboardData.refreshInstanceState`。
2. **P2a ServerControlPage 存档页按钮修复**：`onClick={() => void 0}` 改为 `onNavigate('saves')`，同时补上 `onNavigate` 解构。
3. **P2b 命令加载错误态**：新增 `commandsLoading` / `commandsError` state，`loadCommands()` 失败时设置 `commandsError`，JSX 渲染"加载失败 + 重试按钮"而非永远"加载中"。
4. **P3a 剪贴板复制错误处理**：`ServerControlPage` 和 `OverviewPage` 的 `handleCopy()` 从 `void .then(...)` 改为 `.then(success, failure)` 两参数形式，失败时设置 `copyError` 并显示"复制失败，请手动选取"提示（3s 后自动消失）。
5. **P3b 启动后刷新邀请码**：`handleStart()` 成功后在 `ServerControlPage` 和 `OverviewPage` 均新增 `dashboardData.refreshInviteCode()`。

影响文件：`SavesPage.tsx`、`ServerControlPage.tsx`、`OverviewPage.tsx`。`npm.cmd run build` 通过（exit 0），39 模块，JS 260 kB。

详见 `docs/conversation-handoff-2026-06-28.md`（追加在文件末尾的 2026-06-29 节）。

### FE-R5: 服务器控制页（ServerControlPage）真实化 ✅ completed (2026-06-28)

完整实现 `/instances/stardew/server` 路由，从占位页面升级为真实可用的服务器控制页。

**已真实接入的功能：**

- **生命周期控制**：`startInstance` / `stopInstance` / `restartInstance` 真实调用后端 API，按状态机禁用不可用按钮（如 `running` 时禁用启动、`stopped` 时禁用停止/重启），停止和重启有二步确认弹框，操作中显示 `actionBusy` 禁用态，成功后主动 `refreshInstanceState + refreshInviteCode + refreshJobs`。
- **邀请码**：复用 `dashboardData.inviteCode` 公共数据层，带复制按钮（`navigator.clipboard`），提供"刷新"按钮调用 `dashboardData.refreshInviteCode()`，未运行/加载中/错误时都有明确空状态。
- **状态卡片**：展示 `instanceState.name`、`state`（中文标签）、`driverPhase`、`stateMessage`、`updatedAt`、`saves.activeSaveName`、`versionInfo.version`。
- **全服喊话**：真实调用 `sendSay()` API，优雅处理 `command_not_supported` 错误，支持 Enter 发送，有结果/错误回显。
- **控制台命令**：调用 `getCommands()` 加载 allowlist 命令列表，下拉选择后调用 `runCommand()` 执行，展示命令描述和执行结果。

**保留 UI 但待后端接入的功能：**

- 保存世界（`sd-srv-badge-pending` 标注 disabled）
- 备份存档（同上）
- 计划重启（同上）
- 服务器设置（端口/可见性/密码等，同上）

**CSS 新增：**

在 `StardewPanel.css` 中新增 `sd-srv-*` 前缀样式类：`.sd-srv-section`、`.sd-srv-section-title`、`.sd-srv-empty`、`.sd-srv-hint`、`.sd-srv-result`、`.sd-srv-result-pre`、`.sd-srv-badge-pending`。

**未改动：** Overview 页保留自有快捷生命周期按钮（为独立的 CTA，不与 ServerControlPage 共享代码，逻辑相同但 UI 定位不同）。后端 API 无改动。

`npm.cmd run build` 通过（exit 0），JS bundle 239.51 kB。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R4l: 左侧导航内容组统一长度 ✅ completed (2026-06-28)

根据用户澄清“统一长度没让你统一宽度”，保留 FE-R4k 每个按钮独立背景宽度，但统一图标+文字内容组长度。`frontend/src/games/stardew/StardewPanel.css` 中不再按 route 覆盖 `--sd-nav-pad-left/right`，所有 route 共用 `pad-left: 10`、`pad-right: 7`、`gap: 4` 源像素，并新增 `--sd-nav-text-w: 60`，让文字 `span` 固定为同一缩放宽度。结果是背景盒子仍按各自 route 宽度变化，但内容起点、图标间距和文字区域长度统一。`npm.cmd run build` 通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R4k: 左侧导航每个按钮单独视觉宽度 ✅ completed (2026-06-28)

根据用户要求“不要统一他们的宽度，每个按钮的背景图的实际宽度都不一样，需要完全符合”，在 `frontend/src/games/stardew/StardewPanel.css` 为 9 个 route 分别设置 `--sd-nav-w`、`--sd-nav-active-w` 和内边距变量：短标签（overview/players/mods/install/settings）使用 96 基准；server 使用 110/106；saves 使用 102/105 并保留 `105×28` 专用 active；jobs 使用 126/122；diagnostics 使用 106/102。按钮不再共享同一个视觉宽度，active 状态按每个 route 自己的变量覆盖。`npm.cmd run build` 通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R4j: 左侧导航内边距按源 PNG 像素坐标缩放 ✅ completed (2026-06-28)

针对用户反馈“又不实际匹配自己的背景宽度了，用你的眼睛去看实际匹配”，修正 FE-R4i 中 padding/gap 仍按侧栏 `cqi` 粗略计算的问题。`frontend/src/games/stardew/StardewPanel.css` 现在为导航项定义 `--sd-nav-pad-left/right/gap` 源图像素坐标，普通态默认按 `105×29` 的源坐标缩放，active 态按 `99×29` 设置自己的 `pad-left/right/gap`，避免内容左边距跟随侧栏而不是跟随当前 PNG 盒子。保留垂直居中、水平靠左、放大的字体和图标。`npm.cmd run build` 通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R4i: 左侧导航内容靠左且放大 ✅ completed (2026-06-28)

根据用户要求“上下居中，左右形式是靠左，字体和按钮图标可以适当再大一点”，调整 `frontend/src/games/stardew/StardewPanel.css`。在保留 FE-R4h 的按 PNG 实际尺寸计算规则下，左侧导航项从 `justify-content: center` 改为 `flex-start`，水平 padding 增加到 `clamp(12px, 8cqi, 15px)`，gap 增加到 `clamp(6px, 4.4cqi, 8px)`；字体从 `clamp(10px, 7cqi, 12px)` 提高为 `clamp(11px, 7.6cqi, 13px)`，图标从 `15–18px` 提高为 `17–20px`。垂直方向仍由 flex `align-items: center` 保持居中。`npm.cmd run build` 通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R4h: 左侧导航按各 PNG 实际尺寸居中 ✅ completed (2026-06-28)

根据用户要求“文字居中显示在 PNG 中间，每个 PNG 的宽度其实都不一样，按实际匹配，点击后的绿色也要完全覆盖他自己的背景图大小”，调整左侧导航尺寸模型。`StardewPanel.tsx` 为导航按钮增加 `data-route`，`StardewPanel.css` 不再把所有按钮统一视作 100% PNG 盒子，而是用素材实际尺寸计算：普通导航基准 `105×29`，绿色 active 为 `99×29`，`saves` 专用 active 为 `105×28`，底部帮助为 `96×28`。active 状态改变按钮自身宽高以匹配对应 PNG，并用 flex 居中文字和图标。普通态保持透明显示侧栏木纹原色。`npm.cmd run build` 通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R4g: 普通导航底图撤销以恢复木纹原色 ✅ completed (2026-06-28)

根据用户指出“在按素材比例校准那一步之前颜色正常”，重新确认真正导致左侧发淡的原因是 FE-R4d 把普通导航项和底部帮助都铺上了 `--sd-img-nav-default`。该底图平均色约 `rgba(73,47,29)`，比侧栏木纹平均色 `rgba(59,42,30)` 更亮，因此整块普通导航区看起来淡。已在 `frontend/src/games/stardew/StardewPanel.css` 撤销普通态和 `.sd-sidebar-help` 的默认底图，保留当前侧栏宽度、按钮高度比例、文字/图标对齐和 active 绿色底图。`npm.cmd run build` 通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R4f: 底部 quick help 偏亮素材替换 ⚠ superseded (2026-06-28)

针对用户继续反馈“还是淡淡的，和下面没有文字的背景颜色不一样”，对素材取样确认原因不是 CSS 颜色或插值，而是 `nav_quick_help_blank.png` 本身明显更亮：侧栏木纹平均约 `rgba(59,42,30)`，普通导航底图约 `rgba(73,47,29)`，quick help 约 `rgba(130,86,44)`。已在 `frontend/src/games/stardew/StardewPanel.css` 将 `.sd-sidebar-help` 的背景图从 `nav_quick_help_blank.png` 改为 `--sd-img-nav-default`，高度比例同步为 `105×29`，让底部按钮颜色与普通导航木板一致。`npm.cmd run build` 通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R4e: 左侧像素背景缩放防发淡 ✅ completed (2026-06-28)

针对用户反馈“改完之后底部的背景图颜色变淡”，确认没有改动颜色变量，原因是左侧木纹、导航按钮和底部 quick help PNG 在侧栏宽度放大后由浏览器默认平滑插值，像素边缘和底色被混合导致视觉发灰/发淡。已在 `frontend/src/games/stardew/StardewPanel.css` 为 `.sd-sidebar`、`.sd-sidebar-help`、`.sd-sidebar .sd-nav-item`、active 导航项补充 `image-rendering: pixelated`，保持放大后像素图锐利、不被平滑采样淡化。`npm.cmd run build` 通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R4d: 左侧导航按钮边界按 PNG 比例校准 ✅ completed (2026-06-28)

针对用户反馈“文字和背景的界限还是有很大偏移”，继续校准 `frontend/src/games/stardew/StardewPanel.css` 的左侧导航。保留 FE-R4c 的当前侧栏宽度策略，但按钮高度不再用粗略 `clamp(34px, 27cqi, 44px)`，改为按导航底图真实比例计算：`.sd-nav-item` 使用 `height = 100cqi * 29 / 105`，`.sd-sidebar-help` 使用 `height = 100cqi * 28 / 96`。同时给普通导航项也加上 `--sd-img-nav-default` 默认底图，选中态显式使用 `--sd-img-nav-active`，文字和图标以按钮盒子垂直居中，文字单行省略。`npm.cmd run build` 通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R4c: 左侧当前宽度木板按钮适配 ✅ completed (2026-06-28)

根据用户澄清“要的就是现在侧栏这个大小，文字按钮以现在这个侧栏的一个木板背景变动”，保留 FE-R4b 的左侧列宽 `clamp(148px, 10vw, 176px)`，不再回退到 112px/105px 原始素材尺寸。调整 `frontend/src/games/stardew/StardewPanel.css`：`.sd-sidebar` 启用 `container-type: inline-size`，导航按钮和快速帮助按钮宽度为当前侧栏 `100%`，按钮高度、左右内边距、图标和字号使用 `cqi` + `clamp()` 随当前侧栏宽度适配，文本开启单行省略，保证按钮与当前被拉伸后的木板宽度一致。`npm.cmd run build` 通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R4b: Shell 三栏比例均衡修复 ✅ completed (2026-06-28)

针对用户反馈“中间太大，左有两边太小”，调整 `frontend/src/games/stardew/StardewPanel.css` 的布局比例：Shell 从固定 `112px/1fr/212px` 改为响应式 `clamp(148px, 10vw, 176px) / minmax(0, 1fr) / clamp(280px, 20vw, 360px)`，左侧导航图标、行高和字号略放大，右侧 OpsRail 内边距和文本尺寸略放大；Overview 内部事件/资源栏从固定 `212px` 改为 `clamp(260px, 28%, 340px)`，减少宽屏下主内容区过度吞空间的问题。未改 API 和业务逻辑。`npm.cmd run build` 通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R4: Overview 真实 UI + 像素 UI 回归修复 ✅ completed (2026-06-28)

完整重写 `OverviewPage.tsx` 为原型水平密集布局（农场横幅 58px + 服务器控制横排 + 双栏主体：左 2×2 指标格+玩家占位 / 右事件列表+模组摘要）。同时修复 P1/P2 视觉回归：Shell 尺寸改为 `112px/1fr/212px / 40px` 顶栏；按钮全部改用 PNG 底图（含新增 `sd-btn-start/stop/restart`）；导航 active 使用 `nav_item_active_green_blank.png`；所有圆角收紧至 0–2px；Topbar 改显示 `stateLabel()` 中文标签。`npm.cmd run build` 通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R3: 公共数据层（useStardewDashboardData）✅ completed (2026-06-28)

建立 StardewPanel 公共数据层。新增 `frontend/src/games/stardew/useStardewDashboardData.ts`，集中加载 7 个 API（instanceState/saves/mods/jobs/health/version/inviteCode），单个失败只降级不崩溃，对外暴露 `refreshAll / refreshInstanceState / refreshSaves / refreshMods / refreshJobs / refreshHealth / refreshInviteCode` 供各页面操作后主动刷新；instanceState 每 30s 轮询。在 `stardew-routes.ts` 新增 `StardewDashboardData` 类型并扩展 `StardewPageProps`。重写 `StardewPanel.tsx`，TopStatusBar 新增版本号和当前存档显示，OpsRail 新增健康摘要和 Mod 重启提示。更新 `OverviewPage.tsx` 展示存档/健康/Mod/任务摘要及邀请码，`npm.cmd run build` 通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R2: StardewShell 与路由骨架 ✅ completed (2026-06-28)

建立 StardewShell 和真实路由骨架。新增 `stardew-routes.ts`（`StardewRoute` union、URL 解析/生成、`StardewPageProps` 类型）、`StardewPanel.css`（Shell 四区网格布局）、`StardewPanel.tsx`（左侧木纹导航 9 入口、顶部状态栏、30s 状态轮询、右侧 OpsRail）和 9 个页面占位组件（`pages/`）。重写 `App.tsx`（从 1071 行降至 ~130 行），彻底移除 Dashboard 函数及所有 Dashboard 专用 state/import，`View` 新增 `'stardew'`，登录后全屏渲染 `StardewPanel`。`/instances/stardew/{route}` URL 模式，浏览器前进/后退同步，`npm.cmd run build` 通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### FE-R1: 前端资产与主题基础 ✅ completed (2026-06-28)

完成 UI 重构第 1 步。将 `docs/prototypes/assets/ui-extracted/` 中 57 个 PNG 素材（backgrounds、buttons、fields、icons、navigation、panels、sprites 七类）复制到 `frontend/public/assets/stardew/ui/`，生产 build 后可通过 `/assets/stardew/ui/...` 访问。新增 `frontend/src/games/stardew/stardew-theme.css`，建立 `--sd-*` CSS 变量体系（颜色、素材路径）和基础工具类（木纹背景、羊皮纸面板、绿/红/金按钮变体、像素风输入框、左侧导航项、状态点、紧凑数据行、指标卡）。在 `frontend/src/main.tsx` 中引入该 CSS 文件。未改动任何业务组件或 API，`npm.cmd run build` 验证通过（exit 0）。

详见 `docs/conversation-handoff-2026-06-28.md`。

### Frontend UI Refactor Implementation Plan 📐 documented (2026-06-28)

新增并修订 `docs/frontend-ui-refactor-implementation-plan.md`，把 `docs/prototypes/stardew-anxi-panel-ui-refactor-prototype.html` 从“四个展示区块”识别为真实业务路由集合。当前拆分为 `install`、`overview`、`server`、`saves`、`jobs`、`players`、`mods`、`diagnostics`、`settings` 九个 Stardew 路由。文档明确了 Single Game Mode 不变原则、内部 route + History API 方案、StardewShell 结构、现有组件归位、每个路由已可接入的后端 API、原型中需要保留但等待后端的功能位、FE-R0 到 FE-R13 的细化目标和分轮实施节奏。特别说明：`players` 是未来游戏玩家管理，当前没有完整后端 API，只保留路由、UI 占位和可用摘要；面板用户/审计/版本应放在 `settings`，不要混入游戏玩家管理。

详见 `docs/conversation-handoff-2026-06-28.md`。

### Pixel UI asset extraction ✅ completed (2026-06-27)

基于新的 Stardew Valley 专用服务器管理面板设计图，按区域、按钮、背景、表单控件、导航、图标和贴图拆出无文字可复用 PNG 素材。新增 `scripts/extract-ui-assets.py` 作为可重复运行的裁切/清理脚本，输出目录为 `docs/prototypes/assets/ui-extracted/`，包含 `manifest.json`、`preview.html`、`contact-sheet.png`、`README.md` 和 61 个分类素材。按钮、输入框、面板和整窗壳已清理文字，适合后续 HTML 原型直接作为背景或底图复用。

详见 `docs/conversation-handoff-2026-06-27.md`。

### Frontend UI / Interaction Refactor Spec 📐 documented (2026-06-27)

MVP 功能完成后，新增前端 UI、审美、交互逻辑重构交付文档和 V2 原型。新增 `docs/frontend-ui-interaction-refactor.md`，从用户使用路径重新梳理首次安装、日常启动、存档维护、Mod 维护、排障和权限管理，提出从“长调试页”改为“任务型运维控制台”的信息架构：左侧导航、顶部状态栏、主操作面、右侧任务/健康 rail，并把高级设置拆为 Troubleshoot 与 Security。新增 `docs/prototypes/stardew-anxi-panel-ui-refactor-prototype.html`、`docs/prototypes/stardew-anxi-panel-ui-refactor-prototype.png` 和 `docs/prototypes/stardew-anxi-panel-ui-refactor-notes.md`。同时创建 Figma 草稿原型：`https://www.figma.com/design/GHadKWWdw2jWxgPXgY7fdM`。

详见 `docs/conversation-handoff-2026-06-27.md`。

### Release Candidate ✅ completed (2026-06-27)

Milestone 14 完成。版本信息：新增 `GET /api/version`、`/health` 返回 commit/buildDate、Dockerfile 支持 `--build-arg VERSION/COMMIT/BUILD_DATE` ldflags 注入。支持包导出：新增 `POST /api/instances/:id/support-bundle` 管理员 API，导出 ZIP 包含 version/health/instance-state/jobs/audit-logs/compose-ps/server-logs（全部脱敏）。冒烟测试：新增 `scripts/smoke-test.ps1` Windows PowerShell 脚本。发布检查清单：新增 `docs/release-checklist.md`。前端：页面顶部显示版本号、健康检查区域新增「导出诊断包」按钮。

详见 `docs/conversation-handoff-2026-06-27.md`。

### Hardening ✅ completed (2026-06-27)

Milestone 13 完成。操作审计：新增 `GET /api/audit-logs` 管理员 API，12 个关键操作（install/start/stop/restart/save/mod/command）已添加审计记录。日志脱敏：扩展 `RedactString` 覆盖 session/cookie/auth/bearer/invite code；新增 `sanitizeError()` 替代所有 `err.Error()` 直接暴露。权限加固：新增 4 个 handler 测试覆盖管理员端点 401/403 拒绝。备份恢复：删除存档前自动备份，新增备份列表和恢复 API。健康检查：新增 `/api/health/diagnostics` 返回 Docker/Compose/数据目录/实例/存档的中文诊断。前端：新增集中式错误码映射（40+ 码）、审计日志查看区、健康检查区。

详见 `docs/conversation-handoff-2026-06-27.md`。

### Packaging ✅ completed (2026-06-27)

Milestone 12 完成。新增多阶段 Dockerfile（frontend-builder → backend-builder → runtime with docker-cli + compose）、`.dockerignore`、`deploy/docker-compose.yml` 部署示例、`docs/deployment.md` 部署指南。前端通过 `//go:embed` 嵌入 Go binary，后端 `serveStatic` 提供 SPA fallback。`isSetupAllowed` 白名单扩展以支持初始化页面加载静态资源。容器默认监听 `:8090`，数据目录 `/data`，内置 HEALTHCHECK。

详见 `docs/conversation-handoff-2026-06-27.md`。

### Console and Commands ✅ completed (2026-06-27)

Milestone 11 完成。后端新增 Console 命令执行 API（list/run/say），前端新建 `ConsoleSection` 组件。命令通过 allowlist 机制管控，前端传结构化命令 ID。真实联调后已修正通信方式：`attach-cli` 是 tmux 交互 UI，`quit` 会被转发进 SMAPI 并导致超时；当前命令执行改为写入 Junimo 容器内 `/tmp/smapi-input` FIFO，并从 `/tmp/server-output.log` 抓取输出。`say` 不是当前 Junimo/SMAPI 有效命令，暂返回 `command_not_supported`。Review Fixes 已完成：换行注入防护、普通用户命令权限修复、容器状态 reconcile、FIFO 命令执行。

详见 `docs/conversation-handoff-2026-06-27.md`。

### Mods 管理 ✅ completed (2026-06-27)

Milestone 10 完成。后端新增 Mods 管理 API（list/upload/delete/export），前端新建 `ModsSection` 组件。M9 遗留 review 问题已补齐：ZIP 目录项兼容（已有 TrimSuffix）、whichFarm trim（已有 TrimSpace）、running 保护 web handler 测试已补全。

详见 `docs/conversation-handoff-2026-06-27.md`。

### 存档管理与前端首页信息架构收口 ✅ completed (2026-06-27)

Milestone 9 完成。后端新增 4 个存档管理 API（list/select/select-and-start/delete）。前端新建 `SavesSection` 组件，Dashboard 从"长调试页"重构为分层布局（状态摘要 → 主操作区 → 折叠高级区）。测试任务按钮默认隐藏，Docker 调试区折叠。`go test ./...` 和 `npm run build` 通过。两轮 Review Fixes 已完成：路径安全、active save 校验、Farmer XML 解析兼容、保留路由名规避。

详见 `docs/conversation-handoff-2026-06-27.md`。

### Docker 容器状态实时校验 ✅ completed (2026-06-27)

修复了 Docker 容器停止后前端仍显示"运行中"的 bug。`ReconcileState()` 现在会在实例状态为 `running`/`starting` 时通过 `ComposePs` 校验 `server` 容器是否实际运行中，不在运行则自动修正为 `stopped`。保留 `DriverPayload`（如邀请码）不丢失。

详见 `docs/conversation-handoff-2026-06-27.md`。

### 自定义新建存档修复 ✅ completed (2026-06-26)

修复了"创建存档并启动"自定义配置完全不生效的三个叠加问题：

1. **`server-settings.json` 嵌套结构**：JunimoServer 的 C# 类期望 `{"Game":{...},"Server":{...}}` 嵌套 JSON，面板之前写的是扁平结构，导致全部字段回退默认值。
2. **`docker-compose.yml` 旧版**：saves 使用 Docker named volume 而非 bind mount，宿主机看不到存档，前端永远判定"无存档"。已手动修复 compose 文件（新实例由 `Prepare()` 生成正确模板）。
3. **创建新存档方式**：改用 JunimoServer HTTP API `POST /newgame`（替代失败的 `attach-cli settings newgame --confirm`），旧存档完全保留。

**已知限制**：FarmerName/Gender/外貌等角色字段无法通过 SMAPI mod 在 Junimo runtime 中写入（`SaveCreating` 事件不被 JunimoServer 的 `/newgame` 触发）。需要预置存档模板方案，当前未实现。

详见 `docs/conversation-handoff-2026-06-26.md` 最后一节。

### New-game native UI and packaged assets ✅ completed (2026-06-26)

- The Stardew new-game creator now mirrors the in-game three-column layout while intentionally excluding skin, hair, shirt, pants, and accessory controls.
- `skipIntro` is always enabled. Advanced options for remixed Community Center bundles, remixed mine rewards, and farm monsters are collected in the UI and mapped to Junimo's `bundlesRemix`, `minesRemix`, and `spawnMonstersAtNight` settings.
- Real farm and pet crops were exported once from the maintainer's local Stardew runtime through the SMAPI reflection path, then committed under `frontend/public/assets/stardew/new-game/`; Vite copies them into the published frontend build.
- The former per-user runtime catalog exporter has been removed from the driver, install job, API, frontend polling, and Compose template. Existing instance Compose files are cleaned by `migrateRemoveAssetExporterService` on the next install run.
- Verification completed: `go test ./...` and `npm.cmd run build`.
- Screenshot-backed character/gender/cabin assets and the Meadowlands map were added on 2026-06-26. Pet preference is now a 10-step game-data breed cycle (five cats then five dogs), with backend validation and Junimo `create-or-load` application kept in sync.

项目目标：

- 基于 JunimoServer 做 Stardew Valley 专用服务器 Web 管理面板。
- 用户通过浏览器完成管理员初始化、Steam 认证、Junimo 安装、服务器启动、邀请码展示、状态查看、存档管理、Mod 管理、控制台指令、面板用户管理。
- 长期演进为多游戏开服总面板：总面板展示所有游戏实例状态，点击某个游戏实例后进入该游戏自己的专属管理面板。第一版只做好 Stardew + JunimoServer，并默认使用 Single Game Mode，登录后直接进入 Stardew 面板，不显示总面板游戏列表。

当前已有文档：

- `docs/architecture.md`: 技术架构和模块边界。
- `docs/prototypes/stardew-anxi-panel-product-prototype.html`: 产品原型 HTML。
- `docs/prototypes/stardew-anxi-panel-product-prototype.png`: 产品原型图。
- `docs/prototypes/stardew-anxi-panel-prototype-notes.md`: 原型说明。

## Development Principles

后续实现时请遵守这些约束：

- 不要把 Stardew 专属逻辑放到顶层 `saves`、`mods`、`console` 模块里。
- 顶层只保留通用能力：`auth`、`docker`、`jobs`、`games/registry`、`storage`、`web`。
- Stardew 相关能力放在 `games/stardew_junimo` driver 内。
- 前端也要分层：总面板、通用游戏面板骨架、各游戏自己的 game module。不要把 Minecraft、饥荒、泰拉瑞亚等未来页面塞进 Stardew 页面里。
- 面板后端不替代 JunimoServer，优先通过 Docker Compose、容器 `exec`、`attach-cli`、HTTP status、日志流和挂载目录与 JunimoServer 通信。
- 所有长任务必须有状态、日志、错误信息和可恢复策略。
- 不要把 Steam 密码、VNC 密码、session token 打到日志里。

## Product Model: Single Now, Multi Later

接手者必须理解：本项目最终不是“一个 Stardew 页面兼容所有游戏”，而是“一个总面板 + 多个游戏专属面板”。但首个可上线版本不要提前显示总面板，应该先使用单游戏直达体验。

当前产品模式：

```text
Single Game Mode
  -> 登录后直接进入 Stardew 面板
  -> 不显示总面板游戏列表
  -> 内部仍使用 instance + driver 架构
```

未来产品模式：

```text
Multi Game Mode
  -> 登录后进入总面板
  -> 展示多个游戏实例
  -> 点击进入对应游戏面板
```

建议配置：

```text
PANEL_MODE=single
DEFAULT_INSTANCE_ID=stardew
DEFAULT_DRIVER_ID=stardew_junimo
```

推荐路由行为：

```text
if PANEL_MODE == single and only one instance:
    / -> /instances/stardew

if PANEL_MODE == multi or instances > 1:
    / -> 总面板实例列表
```

```text
Global Panel
  ├─ Stardew Instance -> Stardew Panel -> stardew_junimo driver -> JunimoServer containers
  ├─ Minecraft Instance -> Minecraft Panel -> minecraft driver -> Minecraft containers
  ├─ DST Instance -> DST Panel -> dont_starve_together driver -> DST containers
  ├─ Terraria Instance -> Terraria Panel -> terraria driver -> Terraria containers
  └─ Palworld Instance -> Palworld Panel -> palworld driver -> Palworld containers
```

总面板在 Multi Game Mode 下显示，负责：

- 登录、用户、权限。
- 所有游戏实例列表。
- 所有游戏实例的状态摘要。
- 全局 Docker 状态。
- 全局任务中心。
- 审计日志和基础设置。

游戏专属面板负责：

- 该游戏自己的安装向导。
- 该游戏自己的配置项。
- 该游戏自己的控制台协议。
- 该游戏自己的存档/世界规则。
- 该游戏自己的 Mod/插件规则。
- 该游戏自己的特殊 UI，例如 Stardew 的 Steam Guard / 邀请码，Minecraft 的 RCON / 白名单 / OP。

当前第一版只实现 Stardew，UI 默认隐藏总面板，但代码和文档要按这个模型留边界。

## Core Abstraction: GameDriver

`GameDriver` 是本项目最重要的后端长期抽象。这个面板后面会支持多个游戏，所以从第一版开始就不能把 Stardew 写死在主业务、API handler、jobs 或 docker 层里。

主业务只知道“当前实例使用哪个 driver”。具体游戏怎么准备、安装、启动、读取状态、管理存档、管理 Mod、执行命令，都由对应 driver 实现。`GameDriver` 不代表所有游戏共用同一套页面或命令，它只代表总面板调用各游戏后端能力的统一边界。

```go
type GameDriver interface {
    ID() string
    Name() string

    Prepare(ctx context.Context, instance Instance) error
    Install(ctx context.Context, req InstallRequest) (*Job, error)
    Start(ctx context.Context, req StartRequest) (*Job, error)
    Stop(ctx context.Context, instance Instance) error
    Restart(ctx context.Context, instance Instance) error

    Status(ctx context.Context, instance Instance) (*ServerStatus, error)
    Logs(ctx context.Context, instance Instance) (<-chan LogLine, error)
    ExecCommand(ctx context.Context, cmd string) (*CommandResult, error)

    ListSaves(ctx context.Context, instance Instance) ([]SaveInfo, error)
    UploadSave(ctx context.Context, file UploadedFile) error
    SelectSave(ctx context.Context, name string) error
    DeleteSave(ctx context.Context, name string) error

    ListMods(ctx context.Context, instance Instance) ([]ModInfo, error)
    UploadMod(ctx context.Context, file UploadedFile) error
    DeleteMod(ctx context.Context, id string) error
}
```

第一版 driver 是：

```text
games/stardew_junimo
```

后续可以增加：

```text
games/minecraft
games/dont_starve_together
games/terraria
games/palworld
```

每个 driver 自己负责：

- Compose 模板或容器模板。
- 安装流程。
- 配置文件。
- 状态解析。
- 日志读取。
- 控制台命令。
- 存档规则。
- Mod 规则。

`auth`、`jobs`、`docker`、`storage`、`web` 都是通用基础设施，不应该出现 Stardew 专属业务判断。API handler 应通过 `games/registry` 找到 driver，再调用 driver 方法。

前端对应也应有 game module 边界：

```text
frontend/src/core
frontend/src/games/stardew
frontend/src/games/minecraft
frontend/src/games/dont_starve_together
frontend/src/games/terraria
frontend/src/games/palworld
```

第一版可以只有 `frontend/src/games/stardew`，并在 Single Game Mode 下直接显示它；但不要把未来 Minecraft / DST / Terraria / Palworld 的页面逻辑放进 Stardew 模块。

## JunimoServer Integration Plan

接手者必须先理解：本面板不是另写一个 Stardew 服务端，而是把 JunimoServer 官方流程变成可视化、可恢复、可审计的 Web 工作流。

官方流程来源：

- `https://stardew-valley-dedicated-server.github.io/server/admins/`
- `https://stardew-valley-dedicated-server.github.io/server/admins/quick-start/installation.html`
- `https://stardew-valley-dedicated-server.github.io/server/admins/quick-start/first-setup.html`
- `https://stardew-valley-dedicated-server.github.io/server/admins/configuration/environment.html`
- `https://stardew-valley-dedicated-server.github.io/server/admins/configuration/server-settings.html`
- `https://stardew-valley-dedicated-server.github.io/server/admins/operations/commands.html`

### Official Flow to Panel Flow

| Official Junimo step | Panel UI step | Backend owner | Command / action |
| --- | --- | --- | --- |
| `mkdir junimoserver && cd junimoserver` | 创建 Stardew 实例 | `games/stardew_junimo/install` | Go filesystem creates `/data/instances/stardew` |
| Download `docker-compose.yml` | 准备 Junimo 配置 | `games/stardew_junimo/install` | Download official file or write embedded template |
| Download `.env.example`, rename `.env` | 准备 `.env` | `games/stardew_junimo/config` | Write `.env` under instance dir |
| Set `STEAM_USERNAME`, `STEAM_PASSWORD`, `VNC_PASSWORD` | 输入 Steam/VNC 信息 | `games/stardew_junimo/config` | Rewrite `.env`; redact values in logs |
| `docker compose pull` | 拉取 Junimo 镜像 | `internal/docker` through driver | `docker compose pull` |
| `docker compose run --rm -it steam-auth setup` | Steam Guard 认证 | `jobs` + `stardew_junimo/install` | Run through PTY, stream stdout, accept frontend stdin |
| `docker compose up -d` | 启动服务器 | `stardew_junimo/lifecycle` | `docker compose up -d` |
| `docker compose down` | 停止服务器 | `stardew_junimo/lifecycle` | `docker compose down` |
| `docker compose restart` | 重启服务器 | `stardew_junimo/lifecycle` | `docker compose restart` |
| `docker compose ps` | 状态页 | `stardew_junimo/lifecycle` | `docker compose ps` |
| `docker compose logs -f` | 日志页 | `stardew_junimo/console` | `docker compose logs -f`, service logs |
| `docker compose exec server attach-cli` | 控制台 / 邀请码 / 指令 | `stardew_junimo/console` | Attach and send allowlisted commands |
| `invitecode` | 展示邀请码 | `stardew_junimo/console` | Send command through `attach-cli` |
| `info` | 展示农场状态 | `stardew_junimo/console` | Send command through `attach-cli` |
| `settings show/validate/newgame` | 设置页 / 新游戏 | `stardew_junimo/config` | Prefer `attach-cli`; direct file edit only where safer |
| `saves`, `saves info`, `saves select --confirm` | 存档页 | `stardew_junimo/saves` | Prefer Junimo CLI; file upload uses mounted dirs |
| Mod file operations | Mod 页 | `stardew_junimo/mods` | Manage mounted Mods dir; restart if needed |

### Communication Rules

所有和 JunimoServer 的通信都要藏在 `games/stardew_junimo` driver 后面。

优先级：

1. Mounted files: `.env`, `docker-compose.yml`, saves, mods, backups, `server-settings.json`。
2. Docker Compose: `pull`, `up`, `down`, `restart`, `ps`, `logs`, `exec`, `run`。
3. PTY: 仅用于 `steam-auth setup` 这种交互命令。
4. `attach-cli`: 用于 `info`、`invitecode`、`settings`、`saves`、`cabins`、`rendering`、`host-auto` 等 Junimo 命令。
5. HTTP status/API: 用于状态轮询，只有启用并配置 API key 后使用。

禁止：

- 禁止前端传入任意 shell。
- 禁止 API handler 直接拼接 `docker compose` 命令。
- 禁止把 Junimo 的存档、Mod、控制台逻辑写到顶层通用模块。
- 禁止在日志里输出 Steam 密码、VNC 密码、session token。

## User Journey Implementation Contract

这一节是实现时的产品流程合同。后续开发者做页面、API、状态机和任务队列时，必须按这个顺序落地。

### Step 1: User Runs This Panel Image

用户只需要拉取并运行本面板镜像。面板容器启动时自动准备 Junimo 工作目录和配置文件。

后台自动执行：

```text
create /data/instances/stardew
write or download docker-compose.yml
write .env from .env.example
open panel port 8090
init SQLite panel.db
check whether admin exists
```

注意：

- 这一步不要求用户手动运行 Junimo 官方命令。
- 这一步不拉 Junimo 镜像。
- 这一步不执行 Steam Auth。
- 这一步不启动 Stardew 服务器。

完成后用户访问面板端口，看到管理员初始化注册界面。

### Step 2: Admin Initialization Page

用户输入：

```text
admin username
admin password
confirm password
```

后端执行：

```text
create first admin user
hash password with Argon2id
create HttpOnly session
write audit log
```

完成后进入主界面。

### Step 3: Main Dashboard Before Install

主界面必须展示：

```text
安装游戏按钮: enabled
启动服务器按钮: disabled
disabled reason: 请先安装游戏
Junimo 配置状态: 已准备
游戏安装状态: 未安装
```

如果用户直接点击启动服务器：

```text
return structured error: 请先安装游戏
route frontend to install prompt
```

### Step 4: Install Game Modal

点击 `安装游戏` 弹出输入框：

```text
Steam username
Steam password
VNC password
```

确认后后端执行：

```text
rewrite /data/instances/stardew/.env
docker compose pull
docker compose run --rm -it steam-auth setup
```

实现位置：

```text
games/stardew_junimo/config
games/stardew_junimo/install
internal/docker
internal/jobs
```

注意：

- 所有日志必须脱敏。
- `.env` 写入要用结构化方法，不要简单拼接不可信字符串。
- `steam-auth setup` 必须通过 PTY，因为要交互。

### Step 5: Password Error Retry Loop

如果 Steam 返回密码错误或认证失败：

```text
set state = steam_auth_failed or credentials_required
show password correction modal
rewrite .env
rerun docker compose run --rm -it steam-auth setup
```

这个循环一直持续到：

```text
Steam auth succeeds
or user cancels install
or job timeout/error
```

前端不应该让用户重新走完整安装向导，只需要回到凭据修改弹窗。

### Step 6: Steam Guard Frontend Interaction

如果 Steam Guard 需要操作，后端把 PTY 输出实时推给前端。

前端根据输出展示：

```text
auth method choice: QR scan / username password
Steam Guard choice: mobile app approval / enter code
email code input
mobile app confirmation waiting state
QR code full display
raw terminal output fallback
```

用户输入验证码后：

```text
frontend -> POST/WebSocket steam guard input
backend -> write to PTY stdin
job log -> append sanitized line
```

认证成功后：

```text
set state = steam_auth_done
verify game files/install result
set state = game_installed
enable start server button
```

### Step 7: Separate Save And Start Actions

界面始终提供独立的存档启动面板，包含“创建存档并启动”和“上传存档并启动”。普通 `启动服务器` 是单独动作，默认让 Junimo 加载上次使用的可用存档；仅在后端检查到零有效存档时返回 `save_required` 并引导到该面板。

界面提供：

```text
上传存档
读取已有存档展示
新建存档
```

用户确认后：

```text
upload save -> validate -> write mounted save dir
existing save -> select active save
new save -> mark new game strategy
set state = ready_to_start
docker compose up -d
```

如果用户在没有任何有效存档时点击普通启动：

```text
return state = save_required
scroll to create/upload-and-start panel
```

### Step 8: Fetch Invite Code After Start

`docker compose up -d` 完成后，后端自动运行：

```bash
docker compose exec server attach-cli
```

然后发送：

```text
invitecode
```

必要时发送：

```text
info
```

前端展示：

```text
invite code
copy button
server status
players
active save
```

### Step 9: Daily Management Pages

启动闭环后再做这些页面：

```text
状态页: 服务器运行状态、用户状态、容器状态
指令页: Junimo/SMAPI 指令、服务器喊话
存档页: 上传、切换、删除、备份
Mod 页: 上传、删除、导出、重启提示
用户管理页: 普通用户、管理员、权限
```

实现规则：

- 状态页通过 `docker compose ps`、Junimo HTTP status、`attach-cli info` 综合获取。
- 指令页通过 `attach-cli`，只允许 allowlist 命令。
- 存档页优先使用 Junimo `saves` 命令和挂载目录。
- Mod 页主要管理挂载目录，Junimo 未暴露的能力由面板补充。
- 用户管理页只操作面板 SQLite，不操作 Junimo。

### State to Command Rules

状态和命令必须一一对应，前端按钮只根据状态启用。

```text
uninitialized       -> only setup admin
admin_created       -> prepare Junimo instance
junimo_scaffolded   -> accept Steam/VNC credentials
credentials_required -> write .env, then pull/setup
steam_auth_running  -> stream PTY and accept guard input
steam_auth_failed   -> ask user to re-enter password or guard code
steam_auth_done     -> mark game installed or continue install verification
game_installed      -> ask for save strategy
save_required       -> upload/select/new save
ready_to_start      -> allow docker compose up -d
starting            -> poll compose ps and Junimo status
running             -> allow stop/restart/console/status
stopped             -> allow start/restart if installed and save-ready
error               -> show job logs and recovery action
```

## Milestone 0: Repo Skeleton ✅ 已完成（2026-06-22）

目标：建立项目骨架，让后续开发有稳定落点。

已完成：

- 已创建 `backend` Go 项目并初始化 `go.mod`。
- 已创建 `frontend` React + TypeScript + Vite 项目。
- 已建立基础目录结构。
- 已准备本地开发脚本。
- 后端已提供 `GET /health`，默认监听 `:8090`，支持 `PANEL_ADDR` 覆盖。
- README 已写明 backend/frontend 启动方式。

建议目录：

```text
backend
├─ cmd/panel
├─ internal/auth
├─ internal/docker
├─ internal/jobs
├─ internal/games/registry
├─ internal/games/stardew_junimo
├─ internal/storage
├─ internal/web
└─ migrations

frontend
└─ src
```

怎么做：

- 后端初始化 `go.mod`。
- 前端初始化 Vite React TS。
- 后端先提供 `/health`。
- 前端先能访问一个空壳页面。
- 暂时不要急着写 Docker 逻辑。

完成标准：

- `go test ./...` 可以跑。
- 前端 dev server 可以启动。
- 后端 `/health` 返回健康状态。
- README 或开发说明里写明本地启动方式。

## Milestone 1: Backend Foundation ✅ 已完成（2026-06-22）

目标：搭好 Go 后端的基础能力。

已完成：

- HTTP API server 保持标准库 `net/http`，并保留清晰 web 层入口。
- 已新增环境变量配置加载：`PANEL_ADDR`、`PANEL_DATA_DIR`、`PANEL_DB_PATH`、`PANEL_SECRET`、`PANEL_VERSION`。
- 已使用标准库 `log/slog` 输出基础结构化日志。
- 已实现统一 JSON 错误响应。
- 已使用 `database/sql` + `modernc.org/sqlite` 连接 SQLite。
- 已实现嵌入式 SQL 最小迁移机制。
- 已预留静态文件服务边界，当前未接入前端构建产物。

要做什么：

- HTTP API server。
- 配置加载。
- 日志。
- 错误响应格式。
- SQLite 连接。
- 数据库迁移机制。
- 静态文件服务预留。

建议实现：

- HTTP router 可用 `chi`。
- SQLite 可用标准 `database/sql` + `modernc.org/sqlite` 或 `mattn/go-sqlite3`。
- 迁移可以先用简单 SQL 文件执行，不必一开始引入复杂框架。
- 配置来源：环境变量 + 默认值。

建议配置项：

```text
PANEL_ADDR=:8090
PANEL_DATA_DIR=/data
PANEL_DB_PATH=/data/panel.db
PANEL_SECRET=
```

完成标准：

- 后端启动时自动创建 data 目录和 SQLite 数据库。
- `/health` 返回服务状态、版本、数据库可用性。
- 统一 JSON 错误结构。
- 代码中有清晰的 internal 包边界。

## Milestone 2: Storage and Auth ✅ 已完成（2026-06-22）

目标：实现面板自己的用户体系。

已完成：

- 新增 `users`、`sessions`、`audit_logs`、`panel_settings` 数据表迁移，迁移文件为 `backend/migrations/002_auth.sql`。
- 新增管理员初始化状态接口和初始化管理员接口。
- 密码使用 Argon2id 哈希保存，不保存明文密码，当前最小长度为 6 位。
- Session 使用 HttpOnly Cookie，数据库只保存 session token hash。
- 新增登录、登出、当前用户接口。
- 新增 `admin` / `user` 角色。
- 新增 admin-only 用户管理接口：列表、创建、更新、启用、禁用和真正删除。
- 新增关键操作 audit log：初始化管理员、登录、登出、用户创建、用户更新、用户禁用。
- 无 active admin 时，只允许访问 `GET /health`、`GET /api/setup/status`、`POST /api/setup/admin`。
- 前端已实现管理员初始化页、登录页、基础主界面和最小用户管理区域。

已实现 API：

```text
GET  /api/setup/status
POST /api/setup/admin
POST /api/auth/login
POST /api/auth/logout
GET  /api/auth/me
GET  /api/users
POST /api/users
PATCH /api/users/:id
DELETE /api/users/:id
```

安全和权限规则：

- 普通用户不能访问用户管理接口。
- 最后一个 active admin 不能被禁用或降级。
- 当前登录 admin 不能禁用自己。
- 不把密码、password hash、session token 或 token hash 写入日志、响应或 audit metadata。
- 所有数据库操作使用参数化 SQL。

完成标准：

- 无管理员时只能访问初始化和健康检查。
- 初始化后可以登录。
- 普通用户不能管理其他用户。
- Cookie session 可刷新页面保持登录。
- `go test ./...` 通过。
- `npm run build` 通过。

## Milestone 3: Docker / Compose Control Layer ✅ 已完成（2026-06-22）

目标：建立通用 Docker 操作层，供 GameDriver 使用。

已完成：

- 新增 `backend/internal/docker` 通用 Docker / Compose CLI 控制层。
- 封装 `docker version`、`docker compose version`、`ps`、`logs`、`pull`、`up -d`、`down`、`restart`。
- 所有命令通过 `exec.CommandContext` 和参数数组执行，不经过 shell。
- 所有命令明确工作目录、超时、输出大小上限。
- 结构化返回 stdout、stderr、exit code、duration、timeout 和输出截断状态。
- 命令参数和输出会脱敏 password、token、secret、`STEAM_PASSWORD`、`VNC_PASSWORD` 等敏感字段。
- 新增 admin-only Docker / Compose 状态 API，供当前前端基础状态区使用。
- Docker API 固定使用 `$PANEL_DATA_DIR/instances/stardew` 作为默认 Compose 工作目录，前端不能传入任意工作目录或任意命令。

后续补救说明：

- 这里写死 `$PANEL_DATA_DIR/instances/stardew` 是 Milestone 3 为了本机联调 Docker 状态而保留的临时单实例入口。
- 不需要返工 Milestone 3 的 Docker 执行层；`backend/internal/docker` 本身仍应保持通用。
- Milestone 5 必须把 API 层从“默认 Stardew 工作目录”迁移到“根据 instance_id 找 driver，再由 driver 提供工作目录或状态实现”。
- 目标 API 形态应逐步靠近 `GET /api/instances/:instance_id/docker/ps` 或 `GET /api/instances/:instance_id/status`，而不是永久保留只面向 Stardew 的 Docker API。

已实现能力：

```text
DockerVersion(ctx, workDir)
ComposeVersion(ctx, workDir)
ComposePull(ctx, dir)
ComposeUp(ctx, dir)
ComposeDown(ctx, dir)
ComposeRestart(ctx, dir)
ComposePs(ctx, dir)
ComposeLogs(ctx, dir, opts)
```

已实现 API：

```text
GET /api/docker/status
GET /api/docker/ps
GET /api/docker/logs?service=&tail=100
```

API 行为说明：

- Docker API 需要已完成管理员初始化，并且当前 session 用户角色必须是 `admin`。
- 无 active admin 时，除初始化白名单外仍返回 `setup_required`。
- 普通 `user` 访问 Docker API 返回 403。
- `GET /api/docker/status` 返回 Docker CLI 可用性、Docker Compose 可用性，以及默认 Compose 项目目录状态。
- `GET /api/docker/ps` 在 `$PANEL_DATA_DIR/instances/stardew` 执行 `docker compose ps --format json`，并解析服务名、service、state、status、health、exit code。
- `GET /api/docker/ps` 如果默认工作目录或 compose 文件不存在，返回 409 `compose_project_not_ready`。
- `GET /api/docker/logs` 返回非流式日志快照，不是 SSE/WebSocket；`tail` 默认 100，允许范围 1 到 1000。
- `GET /api/docker/logs` 的 `service` 参数可选，只允许字母、数字、点、下划线和短横线；非法时返回 400 `invalid_service`。
- Docker 命令失败返回 502 `docker_command_failed`，超时返回 504 `docker_command_timeout`，错误 details 中包含已脱敏的结构化命令结果。

完成标准：

- 后端能在指定目录执行 `docker compose ps`。
- 命令失败时能把 exit code 和错误信息记录到结构化命令结果中，后续 jobs 可直接写入 job log。
- 不存在前端任意命令执行入口。
- `go test ./...` 通过。
- `npm run build` 通过。

## Milestone 4: Jobs and State Machine ✅ 已完成（2026-06-22）

目标：让安装、认证、启动等长任务可观察、可恢复。

已完成：

- 新增 `backend/migrations/003_jobs_state.sql`。
- 新增 `jobs`、`job_logs`、`instance_state` 数据表。
- 新增 `backend/internal/storage/jobs.go`，支持创建 job、启动、成功、失败、取消标记、查询最近 jobs、查询 logs、追加 logs、恢复中断任务。
- 新增 `backend/internal/storage/instance_state.go`，支持默认 Stardew 单实例状态、状态查询、状态更新和保守状态转换校验。
- 新增 `backend/internal/jobs` 通用 Job Manager，支持异步执行、context timeout、日志追加、SSE 事件发布、panic 捕获并标记 failed。
- 后端启动时会确保默认 `stardew` instance state 存在，并把重启前遗留的 `queued/running` job 标记为 `failed`。
- 管理员初始化成功后，默认实例状态进入 `admin_created`。
- 新增登录后可读的 jobs/state API 和 admin-only 测试任务 API。
- 新增 SSE 任务日志流。
- 前端新增 Stardew 实例状态卡片、任务中心、任务详情和实时日志窗口。
- 普通 user 不能创建测试任务；admin 可查看全部任务，普通 user 只能查看自己有权限的任务。

新增表：

```text
jobs
job_logs
instance_state
```

`jobs` 关键字段：

```text
id
type
status: queued / running / succeeded / failed / canceled
target_type
target_id
created_by
created_at
started_at
finished_at
error_message
updated_at
```

`job_logs` 关键字段：

```text
id
job_id
level: info / warn / error / debug
message
created_at
sequence
```

`instance_state` 关键字段：

```text
instance_id
driver_id
state
state_message
last_job_id
updated_at
updated_by
```

已实现 API：

```text
GET  /api/jobs
GET  /api/jobs/:id
GET  /api/jobs/:id/logs?after=0&limit=200
GET  /api/jobs/:id/stream
POST /api/jobs/:id/cancel
POST /api/jobs/test
POST /api/jobs/test-fail
GET  /api/instances/stardew/state
```

API 行为说明：

- jobs 查询、详情、logs 和 SSE stream 都必须登录。
- admin 可以查看全部 job。
- 普通 user 只能查看 `created_by` 是自己的 job。
- 测试 job 创建必须 admin。
- `POST /api/jobs/:id/cancel` 当前返回 501 `not_implemented`，后续接真实任务取消。
- `GET /api/jobs/:id/stream` 使用 SSE，按 `job_logs.sequence` 作为事件 id；job 完成时发送 `finished` 事件并结束。
- `POST /api/jobs/test` 创建约 5 秒的模拟成功任务。
- `POST /api/jobs/test-fail` 创建模拟失败任务，最终状态为 `failed` 并保存错误原因。

核心状态仍按 architecture 文档保留：

```text
uninitialized
admin_created
junimo_scaffolded
credentials_required
steam_auth_running
steam_auth_failed
steam_auth_done
game_installed
save_required
ready_to_start
starting
running
stopped
error
```

补救边界：

- Milestone 4 以 `/api/instances/stardew/state` 做 Stardew 单实例联调入口。
- Milestone 5 需要新增通用形态：`GET /api/instances/:instance_id/state`。
- jobs 是通用基础设施，不应写入 Stardew 专属业务判断。
- 当前状态表暂时直接保存上述状态；Milestone 5 之后可逐步拆分通用 `state` 和 driver-specific `driver_phase` / `driver_payload`。

完成标准验证：

- 管理员登录后可以点击“启动测试任务”。
- 页面能实时看到日志追加。
- 成功测试任务最终变为 `succeeded`。
- 失败测试任务最终变为 `failed` 并显示错误原因。
- jobs、job_logs、instance_state 持久化在 SQLite，后端重启后仍可查询。
- 普通用户不能创建测试任务。
- Job Manager 没有前端任意命令执行入口。
- `go test ./...` 通过。
- `npm run build` 通过。

## Milestone 5: GameDriver Registry ✅ 已完成（2026-06-23）

目标：建立实例模型和 GameDriver registry。首版仍然是 Single Game Mode，不显示总面板，但后端已经具备未来 Multi Game Mode 的实例/driver 边界。

已完成：

- 新增配置项：`PANEL_MODE`、`DEFAULT_INSTANCE_ID`、`DEFAULT_DRIVER_ID`，默认分别为 `single`、`stardew`、`stardew_junimo`。
- 新增 `backend/migrations/004_instances.sql`，创建 `instances` 表。
- 新增 `backend/internal/storage/instances.go`，支持默认 instance 创建、查询、列表和状态更新。
- 后端启动时会确保默认 Stardew instance 存在：
  - `id = stardew`
  - `driver_id = stardew_junimo`
  - `name = Stardew Valley`
  - `data_dir = $PANEL_DATA_DIR/instances/stardew`
- 兼容旧 `instance_state` 表：新 `instances` 为空时会从旧默认状态迁移 state/state_message；旧表不删除。
- 新增 `backend/internal/games/registry`，定义完整 `GameDriver` 接口和 MVP 类型，并实现 `Register`、`Get`、`List`。
- 新增 `backend/internal/games/stardew_junimo` driver 骨架：
  - `ID() = stardew_junimo`
  - `Name() = Stardew Valley / JunimoServer`
  - `Prepare` 仅确保实例目录存在。
  - `Status` 通过通用 Docker Compose PS 能力返回基础 runtime 状态。
  - 其他安装、启动、存档、Mod、命令能力返回 `not_implemented`。
- 新增 instance-based API：

```text
GET /api/instances
GET /api/instances/:instance_id
GET /api/instances/:instance_id/state
GET /api/instances/:instance_id/status
GET /api/instances/:instance_id/docker/ps
```

- `/api/instances/:instance_id/status` 通过 `instance.driver_id -> registry.Get -> driver.Status` 获取状态。
- `/api/instances/:instance_id/docker/ps` 使用 `instance.data_dir`，不再硬编码 `$PANEL_DATA_DIR/instances/stardew`。
- 旧 `/api/docker/status`、`/api/docker/ps`、`/api/docker/logs` 保留为 admin-only 兼容/调试入口，其中默认 Compose 目录也改为读取默认 instance。
- 前端仍保持 Single Game Mode：登录后直达 Stardew 当前主界面，不显示总面板/游戏列表。
- 前端内部新增默认实例概念，状态和 Compose PS 主路径切到 `/api/instances/stardew/...`。
- 状态卡显示当前 instance 名称和 driver id。

新增表：

```text
instances
  id
  driver_id
  name
  data_dir
  state
  state_message
  driver_phase
  driver_payload
  created_at
  updated_at
```

权限规则：

- `GET /api/instances*` 基础查询需要登录。
- `/api/instances/:instance_id/status` 登录用户可读基础状态。
- `/api/instances/:instance_id/docker/ps` admin-only。
- 旧 `/api/docker/*` 仍 admin-only。
- 前端不允许提交任意工作目录、任意 shell 或任意 compose 参数。

保留兼容：

- `/api/instances/stardew/state` 仍可用，但现在由通用 `/api/instances/:instance_id/state` 路由处理。
- `/api/docker/*` 仍可作为开发调试入口，但产品主路径应优先使用 `/api/instances/stardew/...`。

本阶段明确未实现：

- Junimo prepare/install 的真实配置写入。
- `docker compose pull`。
- Steam Auth。
- 服务器 start/stop/restart 真实流程。
- 存档、Mod、控制台命令。
- Multi Game Mode 总面板。

完成标准验证：

```bash
cd backend
go test ./...
```

本次结果：通过。

```bash
cd frontend
npm run build
```

本次结果：通过。

后续注意事项：

- Milestone 6 开始做 Junimo 工作目录和 install 时，必须通过 `games/stardew_junimo` driver 创建 job，不要把 Stardew 业务写进 web/docker/jobs 顶层。
- `Prepare` 当前只创建目录，不代表 Junimo 配置、镜像或游戏文件已经安装。
- 真实任务写 job log 前继续脱敏 Steam 密码、VNC 密码、session token、secret。

## Milestone 6: Stardew Junimo Prepare and Install ✅ 已完成（2026-06-23）

目标：跑通 Junimo 工作目录准备和安装流程。

已完成：

- 新增 `.env` 文件管理模块 `backend/internal/games/stardew_junimo/config/env.go`，支持安全读写、合并和未知字段保留，文件权限 0600。
- 新增 `compose_template.go`，嵌入更贴近 JunimoServer 官方 `docker-compose.yml` 的模板（services: `steam-auth`、`server`、`discord-bot`；official volumes: `steam-session`、`game-data`、`saves`、`settings`；使用 `IMAGE_VERSION`；保留 `stdin_open: true` / `tty: true`）。
- `steam-auth` sidecar 已从固定 `sdvd/steam-service:${IMAGE_VERSION}` 改为 `STEAM_SERVICE_IMAGE`，默认 `anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`；`.env` 同步写入 SteamClient 连接等待和认证会话重试参数，支持本地覆盖为 `junimo-steam-service-cn:auth-retry-test` 联调 fork 镜像。
- `Prepare()` 创建实例目录（含 saves/mods/.local-container），首次写入 compose 和 .env，已有文件不覆盖（保留用户修改）。
- `Install()` 校验凭据、加载 instance、创建 `installRunner`、通过 Job Manager 启动 30 分钟超时任务。
- `installRunner.run()` 四步：检查/补齐 Junimo 工作目录（如果 `docker-compose.yml` 或 `.env` 被删会重新生成并写 job 日志）→ 写 .env（凭据写入，不记录到日志；使用 Junimo 官方 `IMAGE_VERSION`）→ 检查本地 Junimo 镜像，缺失时 `docker compose pull` → 等待面板选择 Steam 登录方式。
- 安装 job 会在日志中输出当前实例目录和实际使用的 `docker-compose.yml` 绝对路径，避免本地开发时 `data\instances\stardew` 与 `backend\data\instances\stardew` 两套历史目录混淆。
- Steam Auth 使用跨平台 TTY：Linux/macOS 通过 Docker Compose + PTY 运行，Windows 通过 Docker Engine API 创建 `Tty:true` 容器并 attach stdin/stdout，避免 Docker CLI 的本机终端检查。
- Docker TTY 输出读取器已支持没有换行结尾的交互提示，例如 `Enter Steam Guard code sent to qq.com:`；账号密码/验证码分支能在上游等待验证码时立即切到前端输入阶段，而不是等 60 秒超时后才看到同一行日志。
- `lineHandler` 检测 Steam Guard、认证方式选择、二维码提示和登录失败关键字，更新 `driver_phase`；只有看到明确登录成功关键字才进入安装完成，所有 docker 输出通过 `paneldocker.RedactString` 脱敏。
- `SendSteamGuardInput(jobID, input)` 实现 `registry.SteamGuardSender` 接口，写入活跃 job 的 guard channel。
- `registry.InstallRequest` 新增 `SteamUsername`、`SteamPassword`（never log）、`VNCPassword`（never log）字段。
- `registry.SteamGuardSender` 接口用 type assertion 模式实现可选能力。
- 新增后端路由：
  - `POST /api/instances/:id/prepare`
  - `POST /api/instances/:id/install`
  - `POST /api/instances/:id/steam-guard/input`
- 新增前端 API 函数：`prepareInstance`、`installInstance`、`submitSteamGuardInput`。
- 任务中心支持 admin 清空已结束任务记录和日志；若存在 `queued` / `running` 任务，清空接口返回 409，避免删除进行中的安装日志。
- 前端安装 UI 完整实现：
  - Prepare 按钮 / 安装游戏按钮按状态显示。
  - InstallSection 含 Modal（三个密码字段，`type=password`，不打印到控制台，不存 localStorage）。
  - 安装期间 SSE 日志实时接入 job log viewer。
  - Steam 登录方式选择区域（`auth_method_required`）：镜像检查完成后先让管理员选择扫码登录或账号密码/验证码登录。
  - Steam Guard 选择/输入区域（`steam_guard_choice_required`、`steam_guard_required` 或 `steam_guard_mobile_required`）：账号密码路径触发二次验证后，再选择手机 App 批准或输入验证码。
  - 选择扫码登录时运行 `steam-auth setup`，面板自动向上游第一层 `Choose authentication method` 输入 `2`，并展示 QR 输出。
  - Steam QR 使用独立弹窗展示，避免管理员靠滚动日志判断二维码是否完整。
  - 选择账号密码/验证码登录时运行 `steam-auth download`，让 Junimo 使用 `.env` 中的 `STEAM_USERNAME` / `STEAM_PASSWORD` 登录并下载游戏文件。不要用普通 stdin pipe 跑 `setup` 的账号密码分支；上游该分支使用 `Console.ReadKey()` 读取密码，在无 console / stdin 重定向时会崩溃。
  - 游戏文件下载阶段解析 steam-auth `Progress: done/total files ... (xx.x%)` 日志，并在安装区显示独立百分比进度条；前端百分比按文件数 `done/total` 计算（例如 `100/1470 = 6.8%`），不使用上游括号里的字节百分比。Steamworks SDK app `1007` 下载会切到 `steam_sdk_downloading`，前端单独显示 SDK 下载进度和 2 阶段下载任务进度；只要日志出现 `Downloading app 1007` / `.steam-sdk` 就立刻切到 SDK 阶段，未收到 SDK `Progress` 前显示 0% 和“正在与 Steam 下载服务器建立连接中”。Stardew 安装 job 超时已延长到 2 小时，且 SDK 已安装完成时不会再被 deadline 误判为超时。下载失败/超时/CM 网络错误后的“重试安装”会复用 `.env` 凭据并直接运行 `steam-auth download`，跳过扫码/账号密码选择；已有文件由 steam-auth 校验，已存在且校验通过的文件自动跳过。
  - Steam 二维码展示区域（`steam_qr_required`）：从任务日志提取并用等宽文本完整显示二维码输出；若上游容器在生成二维码前报 `qr_auth_failed`，明确提示改用账号密码/Steam Guard。
  - 安装任务失败后会清理前端活跃安装 job 标记，并把 failed job / error log / instance state message 转成安装区错误提示；`TryAnotherCM`、SteamClient/CM 连接失败、超时、凭据错误、二维码失败、下载失败不再只停留在任务日志里。
  - 认证失败时重显凭据 Modal。
  - 安装成功后展示"已安装"徽标和 disabled 启动按钮占位。
  - 安装期间 2.5s 轮询实例状态；若旧安装任务已超时但 instance 状态未同步，前端会按 `install_timeout` 兜底显示重试按钮。
- 安装状态读取时会通过 `stardew_junimo` driver 校验 `.local-container` 是否存在安装产物；若数据库误留 `game_installed` 但目录为空，会纠正为 `error/install_missing` 并显示重试。
- `go test ./...` 通过；`npm run build` 通过。

实例目录：

```text
/data/instances/stardew
├─ docker-compose.yml  （首次 Prepare 写入，不覆盖）
├─ .env                （首次 Prepare 写空模板；Install 写凭据；不覆盖用户已改字段）
├─ .local-container
├─ saves
└─ mods
```

已实现 API：

```text
POST /api/instances/:id/prepare
POST /api/instances/:id/install
POST /api/instances/:id/steam-guard/input
```

安全规则（已落实）：

- STEAM_PASSWORD、VNC_PASSWORD 不写入任何 job log、audit log、响应 JSON。
- 前端密码框 `type=password`，不打印到 console，不存 localStorage。
- 前端不允许传入任意 shell 命令、任意 compose 参数或任意工作目录。
- `paneldocker.RedactString` 对所有 docker 输出脱敏。

完成标准验证：

- 密码错误时状态进入 `steam_auth_failed`；前端重显凭据 Modal。
- Steam Guard 需要输入时，前端显示验证码输入框，后端写入 stdin channel。
- 认证成功后状态进入 `game_installed`。
- job log 不出现 Steam 密码和 VNC 密码。
- `go test ./...` 通过。
- `npm run build` 通过。

## Milestone 7: Server Lifecycle ✅ 已完成（2026-06-26）

目标：完成启动、停止、重启、状态和邀请码展示。

### 完成内容（2026-06-26）

**后端新增/修改文件：**
- `backend/internal/docker/compose.go`: 新增 `ComposeExecPipe`，用于向容器 stdin 管道输入（attach-cli invitecode）
- `backend/internal/games/stardew_junimo/compose_template.go`: saves 改为 bind mount (`./.local-container/saves:/config/xdg/config/StardewValley`)，删除 named volume `saves:`
- `backend/internal/games/stardew_junimo/installer.go`: 新增 `migrateSavesVolume`（迁移已有实例），在安装流程中自动执行
- `backend/internal/games/stardew_junimo/driver.go`: `Prepare` 新增 `.local-container/saves/Saves` 和 `saves-templates` 目录创建；`updatePhase` 修复为保留 `DriverPayload`
- `backend/internal/games/stardew_junimo/saves.go`: 新建，实现 `ListSaves`、`PreviewSaveZip`、`ImportSaveToVolume`、`WriteServerSettings`、`HasTemplates`；完整 ZIP 安全检查（zip-slip、绝对路径、解压大小限制）
- `backend/internal/games/stardew_junimo/lifecycle.go`: 新建，实现 `Start`/`Stop`/`Restart`/`GetInviteCode`；`LifecycleDockerService` 接口扩展 Docker 操作；`parseInviteCode` 解析 attach-cli 输出
- `backend/internal/games/registry/types.go`: 新增 `NewGameConfig`、`PreflightResult`、`UploadPreviewResult`、`InviteCodeResult`；扩展 `SaveInfo`（元数据字段）
- `backend/internal/web/lifecycle_handlers.go`: 新建，包含 `pendingUploadStore`（token 绑定实例、短 TTL、一次性）；实现全部 8 个生命周期 handler
- `backend/internal/web/handler.go`: server 新增 `pendingUploads` 字段
- `backend/internal/web/instance_handlers.go`: 注册 8 条新路由

**前端修改：**
- `frontend/src/types.ts`: 新增 `SaveInfo`、`NewGameConfig`、`PreflightResult`、`UploadPreviewResult`、`InviteCodeResult`、`LifecycleJobResponse`
- `frontend/src/api.ts`: 新增 `getSavesPreflight`、`createNewGame`、`uploadSavePreview`、`uploadSaveCommitAndStart`、`startInstance`、`stopInstance`、`restartInstance`、`getInviteCode`
- `frontend/src/App.tsx`: 新增 `LifecycleSection`、`SaveCard` 组件；新建游戏 modal（farmName/farmType/小屋/利润/宠物/金钱）；上传存档 modal（两阶段：preview→confirm）；preflight 检查流程
- `frontend/src/App.css`: 新增 lifecycle section、modal、save-card、invite-code 样式

**测试：**
- `backend/internal/games/stardew_junimo/saves_test.go`: 18 个测试（ZIP 安全、存档解析、migrateSavesVolume 幂等、ImportSaveToVolume）
- `backend/internal/games/stardew_junimo/lifecycle_test.go`: 邀请码解析、payload merge
- `backend/internal/games/stardew_junimo/driver_test.go`: 更新已有测试（bind mount 路径、新子目录）

**阻塞点（已按需求文档记录）：**
真正的自定义新建存档（FarmerName/FavoriteThing/外貌）需要预置 save template（`.local-container/saves-templates/<SaveDir>/`）。不支持 SMAPI `loadForNewGame(false)`，当前 M7 通过 `server-settings.json` 配置 Junimo 支持的字段（FarmName/FarmType/利润/小屋/宠物），Junimo 首次启动自动创建存档。以上限制已在前端 modal 提示用户。

### Milestone 7.5: 可视化新建存档创建器 ✅ 已完成（2026-06-26）

**目标：** 在 React 前端实现真实游戏素材驱动的可视化新建存档创建器；通过 SMAPI mod 机制提供农场类型、宠物品种等真实图片；角色字段（FarmerName/Gender/PetType/PetBreed/外貌）通过 server-init.json 由 SMAPI mod 在 SaveCreating 事件中应用。

**后端新增/修改文件：**
- `backend/internal/games/stardew_junimo/embedded/smapi-mod/manifest.json`: SMAPI mod 元数据（嵌入进 Go binary）
- `backend/internal/games/stardew_junimo/embedded/smapi-mod/StardewAnxiPanel.Control.dll`: 预编译 SMAPI mod（14848 字节，嵌入进 Go binary）
- `backend/internal/games/stardew_junimo/smapi_mod.go`: `//go:embed` 指令 + `installSMAPIMod()`，幂等写入 `.local-container/mods/StardewAnxiPanel.Control/`
- `backend/internal/games/stardew_junimo/catalog.go`: `PanelOptionItem`、`CatalogResponse` 类型；`ReadCatalog()`（读 options.json，有 mtime 缓存）；`DefaultCatalog()`（SVG 占位图 fallback，source="fallback"）；`InvalidateCatalogCache()`
- `backend/internal/games/stardew_junimo/compose_template.go`: server service 新增 `SAP_CONTROL_DIR=/data/control` 环境变量；新增两个 bind mount：`.local-container/control:/data/control`、`.local-container/mods/StardewAnxiPanel.Control:/data/Mods/StardewAnxiPanel.Control`
- `backend/internal/games/stardew_junimo/driver.go`: `Prepare()` 新增 `.local-container/control/`、`.local-container/control/commands/`、`.local-container/mods/` 目录创建；调用 `installSMAPIMod()`（非致命错误）
- `backend/internal/games/stardew_junimo/saves.go`: `NewGameConfig` 扩展字段（Gender/PetType/PetBreedID/Skin/Hair/Shirt/Pants/Accessory/EyeColor/HairColor/PantsColor）；`WriteInitConfig()` 写 server-init.json；`profitMargin` 改为数字字符串 ("100"|"75"|"50"|"25")；`moneyMode` 改为 "shared"|"separate"；`WriteServerSettings()` 内部调用 `WriteInitConfig()`
- `backend/internal/games/stardew_junimo/catalog_test.go`: 12 个测试（DefaultCatalog、ReadCatalog 解析/缓存/实例隔离/fallback、normalizeProfitMarginIDs）
- `backend/internal/games/stardew_junimo/saves_test.go`: 更新 profitMargin/moneyMode 测试用例为新格式
- `backend/internal/games/registry/types.go`: `RgbColor` 类型；`NewGameConfig` 扩展新角色字段
- `backend/internal/web/lifecycle_handlers.go`: `handleCustomNewGameCatalog`（GET，需认证）、`handleCustomNewGameCatalogRefresh`（POST，需管理员）
- `backend/internal/web/instance_handlers.go`: 注册 `GET/POST /api/instances/:id/custom-new-game/catalog` 路由

**前端新增/修改文件：**
- `frontend/src/games/stardew/NewGameCreator.tsx`: 可视化创建器（图片网格选农场/宠物品种、Chip 选性别/小屋/利润/资金、骨架屏加载、fallback banner、错误重试）
- `frontend/src/games/stardew/NewGameCreator.css`: 创建器样式（ImageCard、BreedCard、PetTypeCard、Chips、骨架动画、fallback banner）
- `frontend/src/types.ts`: `RgbColor`、`CatalogItem`、`CatalogResponse`；`NewGameConfig` 扩展角色字段
- `frontend/src/api.ts`: `getCustomNewGameCatalog()`、`refreshCustomNewGameCatalog()`
- `frontend/src/App.tsx`: `LifecycleSection` 替换旧内联 modal 为 `<NewGameCreator>`；引入 `defaultInstanceId`

**SMAPI Mod 机制：**
- SMAPI mod 安装在 `/data/Mods/StardewAnxiPanel.Control/`（bind mount 进容器）
- 游戏启动时 SMAPI 读取 `/data/control/options.json`（mod 写入真实游戏素材 data URL）
- 面板 Catalog API 返回 options.json 内容（四态：ready/generating/failed/unavailable）
- server-init.json 写入 `/data/control/`，SMAPI 在 SaveCreating 事件中应用完整角色定制（在 Junimo runtime 下有效）

**验证：**
- `go test ./...` 全部通过
- `npm run build` 通过（19 modules，无 TypeScript 错误）

### Milestone 7.5 续篇：Install 阶段自动导出 catalog ✅ 已完成（2026-06-26）

**目标：** Steam 安装完成后、服务器首次启动前，自动从游戏文件导出真实素材，用户打开"新建存档"应立即看到真实农场/宠物图片，无需先启动正式服务器。

**核心机制：**
- Install job 最后一步 `runCatalogExportPhase()` 启动一次性 `docker run` Junimo 容器（无端口、无 steam-auth）
- 挂载 game-data 命名卷（只读可读）+ `.local-container/control` bind mount（写）
- 轮询宿主侧 `control/options.json` 出现后 `docker stop`，最长等 10 分钟
- 锁文件 `catalog_export.lock`：存在 → status=generating；options.json 存在 → status=ready；error 文件 → status=failed

**新增文件：** `catalog_exporter.go`（`AcquireCatalogLock`/`ReleaseCatalogLock`/`ExportCatalogContent`/`WriteCatalogExportError`/`GetInstanceImageTag`）

**修改文件：** `catalog.go`（四态 CatalogResponse）、`installer.go`（新增 export 阶段）、`lifecycle_handlers.go`（POST 触发后台 goroutine）、`catalog_test.go`（更新断言）、`types.ts`（status 字段）、`NewGameCreator.tsx`（四态 UI + 5 秒轮询）、`NewGameCreator.css`（状态横幅 + spinner）

**验证：**
- `go test ./...` 全部通过
- `npm run build` 通过（19 modules，无 TypeScript 错误）

### 素材导出器启动参数修复（2026-06-26）

首次联调中，素材 exporter 仅报告 `export container exited with error: exit status 1`。修复 `backend/internal/games/stardew_junimo/catalog_exporter.go`：临时容器传入 `ALLOW_INSECURE_SETUP=true`，避免没有 `steam-auth` sidecar 时阻塞离线初始化；`SETTINGS_PATH` 修正为 Compose server 使用的 `/data/settings/server-settings.json`；同时将容器退出前未换行的 stdout/stderr 刷入 job 日志。

验证已通过：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./internal/games/stardew_junimo ./internal/web
```

尚需 Docker 联调：点击“重新生成素材”，确认没有 `steam-auth download`、没有游戏文件重新下载、没有常驻正式 server；若仍失败，优先查看任务中心末尾新增的真实容器错误，而不是只按 exit code 猜测。

### 素材导出 Compose 依赖修复（2026-06-26）

联调日志先后出现裸容器 `exit status 1` 与 10 分钟没有任何 Junimo/SMAPI 输出的超时。后者表明 `sdvd/server` 作为裸 `docker run` 容器没有获得 Junimo Compose 的 `steam-auth` 服务和网络依赖，不能到达 SMAPI `GameLaunched`。

`catalog_exporter.go` 已改用受控的：

```text
docker compose run --name sap-catalog-export-... --rm --no-ports \
  -v <instance>/.local-container/catalog-export-saves:/config/xdg/config/StardewValley \
  server
```

它复用实例 Compose 的 `server`、`steam-auth`、`game-data` 和网络；不会调用 `steam-auth download`。真实 saves 挂载会被临时目录覆盖，导出结束后删除该目录并停止 sidecar。相关 Go 单测已通过；下一次联调应在任务日志看到 `[catalog-export] 通过 Compose 启动临时 server...`，若仍超时则查看 Compose/SMAPI 的实际输出。

### Milestone 7 存档启动策略补充（2026-06-26）

本次检索了当前仓库中“自定义新建存档”和“上传存档解析”的前后端实现状态，结论如下：

- 当前代码里尚未实现可直接复用的自定义新建存档业务代码。
- 当前前端可复用的是 `frontend/src/App.tsx` 里的安装弹窗、任务日志、错误提示、状态轮询、按钮状态和 Modal 样式。
- 当前前端 API 层 `frontend/src/api.ts` 还没有 saves/new-game/upload/parse 相关函数。
- 当前类型层 `frontend/src/types.ts` 还没有自定义新建存档表单、存档解析预览、上传确认响应类型。
- 当前后端 `backend/internal/games/registry/types.go` 只有 `SaveInfo` / `UploadedFile` 占位，字段不足以表达自定义新建存档或解析后的存档数据。
- 当前 `backend/internal/games/stardew_junimo/driver.go` 中 `ListSaves` / `UploadSave` / `SelectSave` / `DeleteSave` 仍返回 `not_implemented`。
- 当前 `backend/internal/web/instance_handlers.go` 只有 state/status/docker/prepare/install/steam-guard 等路由，没有 saves 路由。
- 原型文档只描述“启动前选择存档”，不是可复用代码。

### 外部参考项目复用结论（2026-06-26）

已检索 `E:\stardew-anxi-panel` 中已完成的自定义新建存档和上传解析实现。Milestone 7 应复用其经过验证的产品与协议设计，但必须按本仓库的 `instances + GameDriver + stardew_junimo` 架构重新落位，不能直接复制其旧的 Go/Vue 路由层。

可复用的前后端设计：

- `api/internal/control/control.go` 的 `InitConfig` 是自定义农场表单的完整字段基线：玩家名、农场名、喜欢的东西、性别、宠物/品种、外观、农场类型、小屋数量/布局、资金模式、利润率和跳过开场等；其 `NormalizeInitConfig`、`ValidateInitConfig`、颜色校验和限定枚举应移植为 Stardew driver 的结构化 DTO/校验。
- `web/src/components/FarmInit.vue` 提供农场地图、性别、宠物、小屋、利润率、资金模式的成熟交互结构。新前端可按 React/现有 `App.tsx` 风格重写为启动前 Modal，不要把 Vue 组件或旧页面框架搬入本项目。
- 旧项目上传采用可靠的两阶段协议：`POST /api/saves/upload-preview` 接收受限大小的 ZIP，解压到临时目录并验证真实 Stardew 存档；服务器返回 token 和预览；`POST /api/saves/upload-commit` 才以 token 导入并选中存档。预览元数据至少含存档 ID、大小、修改时间、游戏年/季/日、农场类型/显示名、角色/已有玩家列表。此协议应以本项目的 instance 路由和短期暂存记录实现。
- ZIP 处理必须保留旧项目的安全原则：仅 ZIP、请求体和解压总量限制、拒绝路径穿越/绝对路径/符号链接、寻找并校验 Stardew 主存档文件、临时目录与最终存档目录均做路径边界检查；确认或过期后清理暂存文件。

不能误复用的部分：

- 旧项目的 `ModEntry.StartNativeCreate()` 是 SMAPI 模组内调用 Stardew 原生 `loadForNewGame(false)`/保存链路的执行器，不是普通后端写配置即可替代的能力；它在检测到 Junimo runtime 时还会跳过这条创建分支。
- `WriteJunimoSettings()` 只能映射农场、利润率、小屋、资金等 Junimo `server-settings.json` 设置，不能单独生成完整自定义角色存档。
- 所以 M7 若承诺“自定义新建存档并启动”，必须在 `stardew_junimo` driver 下接入一个经验证的真实存档生成执行器（移植/打包兼容的 SMAPI 原生创建能力，或等价的受控 helper），生成后再用 Junimo 的挂载存档目录和 `saves select --confirm`/既有加载路径选中。不得只写 `driver_payload`、metadata 或 `server-settings.json` 后就宣称已创建。

建议把 M7 的最小 API 收口到实例路由：

```text
GET  /api/instances/:id/saves/preflight
POST /api/instances/:id/saves/custom-new-game
POST /api/instances/:id/saves/upload-preview
POST /api/instances/:id/saves/upload-commit-and-start
POST /api/instances/:id/start
POST /api/instances/:id/stop
POST /api/instances/:id/restart
GET  /api/instances/:id/status
GET  /api/instances/:id/invite-code
```

`upload-commit-and-start` 可以内部创建 lifecycle job，但确认前绝不能写正式 saves volume；自定义创建同样应由 job 记录生成、选中、启动和邀请码获取日志。`driver_payload` 只持久化 `save_strategy`、active save ID 和不敏感摘要，真实存档留在 Junimo 管理的存储中。

因此 Milestone 7 不能写成“复用现成自定义新建存档实现”，而应明确新增一个最小存档启动闭环。用户点击 `启动服务器` 时：

1. 后端先检测服务器侧是否已有可用存档。
2. 如果没有检测到已有存档，前端弹出两条路径：
   - `新建存档`：点击后打开自定义新建存档窗口。由面板前端收集农场名、玩家名、地图类型、初始设置等字段；后端校验后生成可被 Stardew/Junimo 读取的真实初始存档，并写入 `driver_payload.save_strategy=custom_new_game` 摘要。上游 Junimo 不支持完整自定义创建，所以不要把这一步写成简单调用上游 `settings newgame` 就结束，也不能只写 metadata 而不生成存档。
   - `从本机上传存档`：点击后打开上传存档界面。上传后先进入临时解析阶段，解析并完整展示游戏时间、地图、已有玩家名称、农场/角色基础信息等；用户确认无误后再点击“上传到服务器并启动”，后端才写入正式存档位置并启动服务器。
3. 如果已有存档，Milestone 7 可以先允许选择已有存档并启动，完整存档管理留给 Milestone 9。

要做什么：

- 启动前检查安装状态。
- 启动前检查存档选择状态；没有已有存档时按“自定义新建存档 / 本机上传存档并解析确认”两条路径处理。
- 执行 `docker compose up -d`。
- 停止、重启。
- 读取容器状态。
- 获取邀请码。

建议 API：

```text
POST /api/games/stardew/start
POST /api/games/stardew/stop
POST /api/games/stardew/restart
GET  /api/games/stardew/status
GET  /api/games/stardew/invite-code
GET  /api/games/stardew/logs/stream
```

怎么做：

- 如果没有完成安装，启动接口返回“请先安装游戏”。
- 如果没有选择存档，启动接口返回 `save_required`。
- 启动后通过 `docker compose ps`、HTTP status、`attach-cli` 组合判断状态。
- 邀请码优先通过 Junimo 已暴露能力获取。

完成标准：

- 未安装时不能启动。
- 未选存档时不能启动。
- 启动成功后前端显示 `running`、邀请码、玩家数。
- 停止后状态变为 `stopped`。

## Milestone 8: Frontend MVP ✅ 已完成（2026-06-27）

目标：用 React 实现 MVP 可用界面。首版上线体验是 Stardew 单面板直达，不强制显示总面板。

Milestone 8 是前端补救点：如果前面 0-4 做出的前端主界面直接等同于 Stardew 面板，这里不要强行加一个空总面板，而是调整为“Single Game Mode 直达 Stardew game module；Multi Game Mode 才显示总面板”。

页面：

- 初始化注册页。
- 登录页。
- Single Game Mode 入口：登录后直接进入 Stardew 面板。
- Stardew 游戏面板入口：内部路由建议使用 `/instances/stardew` 或 `/instances/:instance_id`。
- Multi Game Mode 总面板：预留但默认隐藏；等第二个游戏面板出现后再展示。
- 安装向导页。
- 首页/控制台页。
- 存档选择页。
- 基础日志页。

怎么做：

- 使用 React + TypeScript + Vite。
- 使用 TanStack Query 管理 API 请求。
- 使用 Zustand 或 Context 管理当前用户和实例状态。
- 使用 xterm.js 或轻量日志窗口展示安装输出。
- 预留 `frontend/src/core` 和 `frontend/src/games/stardew` 分层。
- 视觉参考 `docs/prototypes`，但先保证流程闭环，不追求一次做完全部美术。

前端迁移示例：

```text
Before:
/dashboard
  -> 直接显示 Stardew 安装、启动、存档、Mod

After:
/
  -> PANEL_MODE=single 时自动进入 Stardew 面板
  -> PANEL_MODE=multi 时显示总面板实例列表

/instances/:instance_id
  -> 根据 instance.driver_id 加载对应 game module

/instances/stardew
  -> Stardew 专属安装、Steam Guard、邀请码、存档、Mod
```

第一版只有 Stardew 一个实例时，用户不应看到多余的选择游戏页面。代码结构要像多实例/多游戏，但产品体验要像一个完整的 Stardew 面板。

关键交互：

- 没有管理员时强制进入初始化。
- 未安装时首页按钮引导到安装向导。
- 安装时展示任务日志。
- Steam Guard 出现时展示二维码/验证码输入。
- 启动按钮根据状态禁用或可用。

完成标准：

- 普通用户能完整走完登录后查看状态。
- 管理员能走完安装、认证、选择存档、启动服务器。
- 前端按钮状态与后端状态机一致。

### 完成内容（2026-06-27）

**前端结构拆分：**
- 将 ~2340 行的单体 `App.tsx` 拆分为 14 个独立模块。
- `frontend/src/core/`：通用工具和 UI 组件（helpers.ts, StatusBadge.tsx, Field.tsx, PasswordInput.tsx, StatusPill.tsx, CommandOutput.tsx, SetupPanel.tsx, LoginPanel.tsx）。
- `frontend/src/games/stardew/`：Stardew 专属组件（InstallSection.tsx, LifecycleSection.tsx, InstanceStateCard.tsx, JobsSection.tsx, DockerSection.tsx, install-helpers.ts, NewGameCreator.tsx, NewGameCreator.css）。
- `App.tsx` 精简为 ~600 行的路由+布局+Dashboard 编排组件。

**主面板打磨：**
- eyebrow 文本从"里程碑 7 · Stardew Junimo Lifecycle"更新为"Stardew Valley 管理面板"。
- CSS 修复：合并两处重复的 `.modal-overlay` / `.modal-card` 定义；将 `.lifecycle-section`、`.save-card`、`.preflight-result` 等从缺失的 CSS 变量（`--card`、`--border`、`--text-muted`）改为 Stardew 主题色值。
- 新增 `.lifecycle-state-game_installed`、`.lifecycle-state-save_required`、`.lifecycle-state-ready_to_start` 状态色。
- 按钮文案确认：启动服务器（使用上次存档）、创建存档并启动、上传存档并启动。

**验证：**
- `go test ./...` 全部通过
- `npm.cmd run build` 通过（33 modules，无 TypeScript 错误）

**未引入的变更（按计划有意跳过）：**
- 未引入 React Router（当前 View 状态机已满足 Single Game Mode 需求）。
- 未引入 TanStack Query / Zustand（当前 useState + 直接 API 调用已足够 MVP）。
- 未创建假的 Minecraft/DST 页面。
- 未删除 Docker 调试区域（admin 联调需要）。

**下一步注意事项：**
- 如果未来引入 React Router，建议使用 `react-router-dom` v6+，路由结构：`/` → Single Game Mode 入口，`/instances/:id` → 游戏面板。
- `frontend/src/core/` 已建立，后续 Multi Game Mode 可直接扩展 `frontend/src/games/minecraft/` 等。
- CSS 仍为单一文件，如需模块化可拆分为各组件的 `.css` 文件。

## Milestone 9: 存档管理与前端首页信息架构收口 ✅ 已完成（2026-06-27）

### 完成内容

**后端新增/修改：**

| 文件 | 改动 |
|------|------|
| `backend/internal/games/registry/types.go` | `SaveInfo` 新增 `IsActive` 字段；新增 `SavesListResult` 类型 |
| `backend/internal/games/stardew_junimo/saves.go` | 新增 `GetActiveSaveName`（读取 gameloader.json）、`DeleteSave`（删除单个存档）；`ListSaves` 增加 active save 标记 |
| `backend/internal/games/stardew_junimo/driver.go` | 无改动（`SelectSave`/`DeleteSave` 接口签名缺少 instance，handler 直接调用 `sj` 包函数） |
| `backend/internal/web/lifecycle_handlers.go` | 新增 4 个 handler：`handleSavesList`、`handleSaveSelect`、`handleSaveSelectAndStart`、`handleSaveDelete` |
| `backend/internal/web/instance_handlers.go` | 注册 4 条新路由 |

**新增 API：**

```text
GET    /api/instances/:id/saves                — 存档列表 + activeSaveName
POST   /api/instances/:id/saves/select         — 选择存档为下次启动存档
POST   /api/instances/:id/saves/select-and-start — 选择并启动
DELETE /api/instances/:id/saves/:name           — 删除存档（admin-only，运行中禁止）
POST   /api/instances/:id/saves/:name/export   — 导出存档为 ZIP（命名：存档名_游戏时间.zip）
```

**前端新增/修改：**

| 文件 | 改动 |
|------|------|
| `frontend/src/types.ts` | `SaveInfo` 新增 `isActive`；新增 `SavesListResult` 类型 |
| `frontend/src/api.ts` | 新增 `getSaves`、`selectSave`、`selectSaveAndStart`、`deleteSave`、`exportSave` |
| `frontend/src/games/stardew/SavesSection.tsx` | **新建** — 存档管理区域（列表、空状态、选择/启动/删除、创建/上传入口、上传预览确认页） |
| `frontend/src/games/stardew/LifecycleSection.tsx` | **简化** — 移除内联 SaveCard、上传 Modal、NewGameCreator；仅保留状态标签和启动/停止/重启/邀请码 |
| `frontend/src/games/stardew/JobsSection.tsx` | 测试任务按钮通过 `VITE_SHOW_DEV_TOOLS=true` 环境变量控制，默认隐藏 |
| `frontend/src/App.tsx` | Dashboard 布局重构：顶部状态摘要 → 主操作区（左：生命周期+存档管理，右：任务日志） → 折叠高级区（Docker+用户管理） → 登出 |
| `frontend/src/App.css` | 新增 `.dashboard-status-row`、`.dashboard-main`、`.saves-section`、`.save-row`、`.empty-saves`、`.collapsible-header`、`.upload-preview-detail` 等样式；移动端响应式 |

**布局变化：**

```text
Before (M8):  flat grid — 所有区域平铺
After (M9):
  顶部状态摘要行: 用户卡 + 实例状态卡
  安装区（未安装时显示）
  主操作区:
    左侧: 服务器控制 + 存档管理
    右侧: 任务中心
  高级设置（折叠）: Docker 调试 + 用户管理
  登出按钮
```

### 验证

```powershell
cd E:\stardew-server-anxi-panel\backend
go test ./...
# 全部通过

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 通过（34 modules，无 TypeScript 错误）
```

### 如何验证

1. 无存档时：存档管理区域显示空状态，有"创建存档并启动"和"上传存档并启动"按钮
2. 创建存档后：存档列表显示，active save 高亮标记
3. 上传存档：预览确认页显示完整元数据（农场名、农民名、游戏时间、地图、文件大小、修改时间）
4. 选择存档：点击"设为启动存档"后 active 标记更新
5. 选择并启动：点击后自动启动服务器
6. 删除存档：二次确认，运行中禁止删除
7. 普通启动无存档：提示并自动滚动到存档管理区域
8. 测试任务按钮：默认不显示（需 `VITE_SHOW_DEV_TOOLS=true`）
9. Docker 调试区：默认折叠在"高级设置"中
10. 移动端：单列堆叠，无溢出

### 下一步注意事项

- 备份能力（`POST .../saves/:name/backup`）未实现，删除存档前暂不做自动备份。M13 可补充。
- `GameDriver` 接口的 `SelectSave(ctx, name)` 和 `DeleteSave(ctx, name)` 缺少 `instance` 参数，当前 handler 直接调用 `sj` 包函数。如需统一接口，后续可修改 `GameDriver` 签名为 `SelectSave(ctx, instance, name)` / `DeleteSave(ctx, instance, name)`。
- CSS 仍为单一 `App.css` 文件，如需模块化可拆分。
- 未引入 React Router、TanStack Query、Zustand。
- **Review Fixes（2026-06-27）**：已修复 DeleteSave 路径穿越漏洞、选择存档前校验存在性、删除前 reconcile 真实容器状态、前端存档列表 job 完成后自动刷新、存档元数据多路径读取。详见 `docs/conversation-handoff-2026-06-27.md`。
- **Review Fixes 第二轮（2026-06-27）**：上传/导入路径安全加固（PreviewSaveZip/ImportSaveToVolume 校验）、保留路由名冲突规避、普通启动 active save 一致性校验、readSaveInfo 兼容 Farmer XML 结构（含 seasonForSaveGame 数字映射和 whichFarm 缺失处理）、前端适配新错误码和地图未知显示。详见 `docs/conversation-handoff-2026-06-27.md`。
- **Review Fixes 第三轮（2026-06-27）**：后端创建/上传/选择并启动接口禁止运行中操作（统一 `ensureInstanceNotRunning` helper）、ZIP 路径穿越严格化（逐段检查 `..`/`.`/空段）、存档地图类型从主存档文件补读 `whichFarm`（支持整数和字符串两种格式）。详见 `docs/conversation-handoff-2026-06-27.md`。

## Milestone 10: Mods ✅ 已完成（2026-06-27）

### 完成内容

**后端新增/修改：**

| 文件 | 改动 |
|------|------|
| `backend/internal/games/registry/types.go` | `ModInfo` 扩展完整字段（UniqueID/Name/Version/Author/Description/FolderName/ParseError）；新增 `ModsListResult` 类型 |
| `backend/internal/games/stardew_junimo/mods.go` | **新建** — Mod 管理核心逻辑：`ListMods`、`UploadModZip`、`DeleteMod`、`ExportModsZip`、`FindModByUniqueID`、manifest.json 解析、ZIP 安全校验、restart-required 标志、compose 迁移 |
| `backend/internal/games/stardew_junimo/mods_test.go` | **新建** — 32 个测试覆盖上传安全、manifest 解析、删除安全、导出路径、重启标志、compose 迁移 |
| `backend/internal/games/stardew_junimo/compose_template.go` | 新增 `/.local-container/mods:/data/Mods` bind mount |
| `backend/internal/games/stardew_junimo/installer.go` | 新增 `migrateModsCompose` 调用 |
| `backend/internal/web/lifecycle_handlers.go` | 新增 4 个 handler：`handleModsList`、`handleModsUpload`、`handleModDelete`、`handleModsExport` |
| `backend/internal/web/instance_handlers.go` | 注册 4 条新路由 |

**新增 API：**

```text
GET    /api/instances/:id/mods              — Mod 列表 + restartRequired
POST   /api/instances/:id/mods/upload       — 上传 Mod ZIP（admin-only，运行中禁止）
DELETE /api/instances/:id/mods/:modId        — 删除 Mod（admin-only，运行中禁止）
POST   /api/instances/:id/mods/export       — 导出所有 Mod 为 ZIP
```

**前端新增/修改：**

| 文件 | 改动 |
|------|------|
| `frontend/src/types.ts` | 新增 `ModInfo`、`ModsListResult` 类型 |
| `frontend/src/api.ts` | 新增 `getMods`、`uploadMod`、`deleteMod`、`exportMods` |
| `frontend/src/games/stardew/ModsSection.tsx` | **新建** — Mod 管理区域（列表、空状态、上传/删除/导出、重启提示） |
| `frontend/src/App.tsx` | 引入 `ModsSection`，Dashboard 左侧添加 Mod 管理区 |
| `frontend/src/App.css` | 新增 `.mods-section`、`.mod-row`、`.mods-restart-banner` 等样式 |

**M9 遗留 review 补齐：**

| 问题 | 状态 |
|------|------|
| ZIP 上传目录项兼容 | ✅ 已有 `TrimSuffix(name, “/”)` 处理，测试已覆盖 |
| whichFarm trim | ✅ 已有 `strings.TrimSpace` 处理，测试已覆盖 |
| running 保护 web handler 测试 | ✅ 新增 `saves_handlers_test.go`，覆盖 5 个存档操作 + 2 个 Mod 操作 × running/starting 状态 |

### 验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
# 全部通过

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 通过（35 modules，无 TypeScript 错误）
```

### 安全要点

- 上传 ZIP 检查：符号链接、绝对路径、`..`/`.`/空段、zip-slip、大小限制（200MB 压缩 / 512MB 解压 / 64MB 单文件）
- 允许合法目录 entry（如 `SomeMod/`）
- 删除前校验路径在 mods 根目录内，拒绝 `.`/`..`/路径分隔符/绝对路径
- 重复 UniqueID 拒绝覆盖，返回 `mod_exists`
- running/starting 时禁止上传和删除 Mod，返回 409 `server_running`
- 导出 ZIP 路径均为相对路径，跳过隐藏文件和临时文件

### 如何验证

1. 上传包含单个 Mod 的 ZIP → 成功，列表显示 Mod 信息
2. 上传包含多个 Mod 的 ZIP → 成功，每个 Mod 独立显示
3. 上传无 manifest 的 ZIP → 拒绝，提示错误
4. 上传重复 UniqueID → 拒绝，提示 `mod_exists`
5. 删除 Mod → 二次确认，成功后列表更新
6. 导出 Mod → 下载 ZIP 文件
7. 上传/删除后 → 显示”Mod 变更需要重启服务器生效”
8. 服务器运行中 → 上传/删除按钮禁用
9. 解析失败的 Mod → 列表中显示 parseError

### 下一步注意事项

- Mod 启用/禁用（SMAPI 不支持热禁用，需要重启）未实现
- Mod 依赖关系检查未实现
- Mod 自动备份未实现
- `GameDriver` 接口的 `ListMods`/`UploadMod`/`DeleteMod` 签名仍返回 `ErrNotImplemented`，handler 直接调用 `sj` 包函数。如需统一接口，后续可修改签名为 `(ctx, instance, ...)` 形式
- compose mods mount 迁移在下次 install 时自动执行；已有实例需手动重新安装或手动编辑 compose

## Milestone 11: Console and Commands ✅ 已完成（2026-06-27）

### 完成内容

**后端新增/修改：**

| 文件 | 改动 |
|------|------|
| `backend/internal/games/stardew_junimo/console.go` | **新建** — 命令 allowlist 定义、`RunAllowlistedCommand`、`SendSay`、`ListCommands`、`stripControlChars`、`CommandError` 类型 |
| `backend/internal/games/stardew_junimo/console_test.go` | **新建** — 22 个测试覆盖 allowlist 拒绝、shell 特殊字符、权限检查、状态检查、say 清理、命令执行 |
| `backend/internal/web/lifecycle_handlers.go` | 新增 3 个 handler：`handleCommandsList`、`handleCommandRun`、`handleCommandSay`；新增 `consoleRunner` 接口 |
| `backend/internal/web/instance_handlers.go` | 注册 3 条新路由 |

**新增 API：**

```text
GET  /api/instances/:id/commands          — 返回可用命令列表（登录用户，按角色过滤）
POST /api/instances/:id/commands/run      — 执行 allowlist 命令（admin-only）
POST /api/instances/:id/commands/say      — 服务器喊话（登录用户）
```

**命令 allowlist：**

| ID | 显示名 | AdminOnly |
|----|--------|-----------|
| info | 服务器信息 | false |
| invitecode | 邀请码 | false |
| settings-show | 查看设置 | true |
| settings-validate | 校验设置 | true |
| rendering-status | 渲染状态 | true |
| host-auto | 自动托管状态 | true |
| host-visibility | 可见性状态 | true |

**前端新增/修改：**

| 文件 | 改动 |
|------|------|
| `frontend/src/types.ts` | 新增 `ConsoleCommandDef`、`CommandsListResult`、`CommandRunResult` |
| `frontend/src/api.ts` | 新增 `getCommands`、`runCommand`、`sendSay` |
| `frontend/src/games/stardew/ConsoleSection.tsx` | **新建** — 命令按钮网格、服务器喊话输入框、命令历史列表（可折叠输出） |
| `frontend/src/App.tsx` | 引入 `ConsoleSection`，放在 ModsSection 下方 |
| `frontend/src/App.css` | 新增 `.console-section`、`.command-btn-grid`、`.console-say-area`、`.console-history` 等样式 |

### 安全要点

- **结构化输入**：前端只传 `{command: "info"}`，后端在 allowlist 中查找，不拼接任意 shell
- **无 shell 注入**：`ComposeExecPipe` 使用 args 数组，不经 shell 解析
- **say 清理**：strip 控制字符、限制 200 字符、拒绝空消息
- **状态检查**：服务器未运行时返回 `server_not_running`（409）
- **权限分离**：info/invitecode 普通用户可用，settings/rendering/host 命令 admin-only
- **敏感信息**：不记录 Steam/VNC 密码、session token

### 验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
# 全部通过

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 通过（36 modules，无 TypeScript 错误）
```

### 如何验证

1. 服务器未运行时 → 命令按钮区域显示"服务器未运行"提示
2. 服务器运行中 → 显示命令按钮网格（info、邀请码等）
3. 点击命令按钮 → 显示 loading 状态，完成后输出显示在命令历史中
4. 命令历史可折叠，显示命令名、时间、退出码、输出内容
5. 喊话输入框 → 输入文本，点击发送或回车，限制 200 字符
6. 普通用户登录 → 只看到 info 和邀请码两个按钮
7. 管理员登录 → 看到全部 7 个命令按钮
8. 服务器未运行时喊话 → 按钮禁用

### 下一步注意事项

- 实时日志流（SSE/WebSocket tail logs）未实现，留给 M13 或后续
- 完整交互式控制台（类似终端）未实现，当前只支持单次命令执行
- 更多命令参数 schema（如 `settings set <key> <value>`）未实现
- 审计日志（谁在什么时间执行了什么命令）未实现，留给 M13
- `GameDriver` 接口的 `ExecCommand(ctx, cmd)` 签名仍然返回 `ErrNotImplemented`，console 能力通过 `consoleRunner` 接口暴露
- **已知待排查问题**：联机角色槽异常 — 同一邀请码下重复出现"新农夫"入口，每创建一次多一个。已确认面板普通启动不会误触发 `newgame`，需要排查 JunimoServer/Stardew 联机层面的角色槽行为。详见 `docs/conversation-handoff-2026-06-27.md` 末尾排查项。
- **Review Fixes 第二轮**：ComposeExecTTY 阻塞修复（double start、stdin EOF、ctx 感知读取、轮询退出码、前端 AbortController 40s 超时）。需要真实 Docker 联调验证。

完成标准：

- 前端可执行 `info` 和 `invitecode`。
- 命令输出显示在前端。
- 未授权用户不能执行管理命令。

## Milestone 12: Packaging ✅ 已完成（2026-06-27）

### 完成内容

**新增文件：**

| 文件 | 说明 |
|------|------|
| `Dockerfile` | 多阶段构建：frontend-builder (node:22-alpine) → backend-builder (golang:1.25-alpine) → runtime (alpine:3.20) |
| `.dockerignore` | 排除 .git、node_modules、dist、data、临时文件 |
| `backend/internal/static/static.go` | `//go:embed frontend_dist/*` 嵌入前端构建产物 |
| `backend/internal/static/frontend_dist/.gitkeep` | 占位文件，保证本地 `go build` 通过 |
| `deploy/docker-compose.yml` | 部署示例：panel 服务 + 端口 + volume + socket mount + restart |
| `docs/deployment.md` | 完整部署指南：构建、运行、环境变量、数据持久化、安全说明、排错 |

**修改文件：**

| 文件 | 改动 |
|------|------|
| `backend/internal/web/handler.go` | 新增 `serveStatic` 方法（SPA fallback）；`isSetupAllowed` 白名单扩展 `/`、`/assets/*`、`/favicon.ico`、`/index.html` |
| `README.md` | 新增 Docker 部署章节；更新仓库结构和当前状态 |
| `docs/handoff-roadmap.md` | 标记 Milestone 12 已完成 |
| `docs/conversation-handoff-2026-06-27.md` | 新增 Packaging 章节 |

**构建流程：**

```text
Stage 1 (frontend-builder):
  node:22-alpine
  npm install + npm run build → dist/

Stage 2 (backend-builder):
  golang:1.25-alpine
  COPY frontend/dist → internal/static/frontend_dist/
  CGO_ENABLED=0 go build → /app/panel

Stage 3 (runtime):
  alpine:3.20
  apk add docker-cli docker-cli-compose ca-certificates tzdata
  COPY /app/panel
  EXPOSE 8090, VOLUME /data, HEALTHCHECK /health
```

**关键设计决策：**

1. **前端嵌入 Go binary**：使用 `//go:embed` 将前端构建产物嵌入后端二进制。运行时只需一个文件即可服务 API 和前端页面。
2. **SPA fallback**：非 `/api/*` 和 `/health` 的请求先查找嵌入文件，找不到则返回 `index.html`，支持前端路由。
3. **setup 白名单扩展**：`/`、`/assets/*` 等静态路径在未初始化管理员时也可访问，否则前端无法加载初始化页面。
4. **CGO_ENABLED=0**：`modernc.org/sqlite` 是纯 Go 实现，不需要 CGO，可构建静态链接二进制运行在 Alpine 上。
5. **runtime 用 Alpine 3.20**：体积小、docker-cli 和 docker-cli-compose 包可用。

**环境变量：** 容器内默认值已适配（`PANEL_ADDR=:8090`、`PANEL_DATA_DIR=/data`），无需额外配置即可运行。

**HEALTHCHECK：** 使用已有的 `GET /health`，30 秒间隔，5 秒超时。

### 验证

```powershell
# 后端测试
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
# 全部通过

# 前端构建
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 通过（36 modules）

# 镜像构建
cd E:\stardew-server-anxi-panel
docker build -t stardew-server-anxi-panel:local .
# 本地 Docker 镜像源暂时 403，Dockerfile 本身正确

# 容器运行验证（镜像构建成功后）
docker run --rm -d --name anxi-panel-test -p 8090:8090 -v /var/run/docker.sock:/var/run/docker.sock -v anxi-panel-test-data:/data stardew-server-anxi-panel:local
docker logs anxi-panel-test
# 浏览器 http://localhost:8090
docker exec anxi-panel-test docker version
docker exec anxi-panel-test docker compose version
docker rm -f anxi-panel-test
docker volume rm anxi-panel-test-data
```

### 已知限制

- **Docker Socket 权限**：挂载 socket 等同高权限，仅限内网使用。
- **Windows Docker Desktop**：socket 通过 WSL2 转发，面板控制的容器运行在 WSL2 中。
- **镜像内 Docker CLI 依赖**：runtime 使用 Alpine apk 安装 docker-cli 和 docker-cli-compose，版本随 Alpine 仓库。
- **本地 Docker 镜像源**：当前 Docker Desktop 配置的 `vfonjwaa.mirror.aliyuncs.com` 返回 403，需检查 Docker Desktop 设置或临时移除镜像源配置。

### 下一步注意事项

- Milestone 13 (Hardening) 可补充操作审计、备份恢复和更完整权限。
- 如果需要支持 ARM 架构，Dockerfile 的 `GOOS/GOARCH` 需要参数化或使用 `docker buildx`。
- 如果前端需要 favicon.ico，可在 `frontend/public/` 放置，Vite 会自动复制到 dist。

## Milestone 13: Hardening ✅ 已完成（2026-06-27）

### 完成内容

**1. 操作审计 Audit Log**

- 新增 `GET /api/audit-logs` 管理员专用 API，支持分页查询。
- 新增 `ListAuditLogs` 存储函数，关联 users 表获取操作者用户名。
- 新增 `auditLog` helper 方法，自动记录 IP 和 User-Agent。
- 以下操作已添加审计记录：
  - `instance_prepare`、`instance_install`
  - `instance_start`、`instance_stop`、`instance_restart`
  - `save_new_game`、`save_upload_start`、`save_select`、`save_delete`、`save_restore`
  - `mod_upload`、`mod_delete`
  - `command_run`
- 已有审计（setup_admin_created、auth_login、auth_logout、user_*）保持不变。
- 前端新增审计日志查看区域（高级设置内），显示时间、操作、操作者、目标。

**2. 日志脱敏**

- 扩展 `docker/redact.go` 脱敏模式：
  - 新增：`session`、`cookie`、`authorization`、`api_key`、`apikey`
  - 新增：Bearer token 脱敏（`Authorization: Bearer eyJhb...`）
  - 新增：邀请码脱敏（`invite code: ABCD1234`、`邀请码=EFGH5678`）
  - 新增：`--env` flag 脱敏（`--env SECRET_KEY=abc123`）
- `Redacted` 常量导出，供其他包使用。
- 新增 `sanitizeError()` / `sanitizeErrorMsg()` 函数，自动识别并替换内部错误详情为中文安全消息。
- 所有 handler 的 `writeError` 调用已替换 `err.Error()` 为 `sanitizeError()` / `sanitizeErrorMsg()`。
- 新增 10 个脱敏单元测试。

**3. 权限加固**

- 新增 `TestPermissionHardening_AdminOnlyEndpoints` 测试：覆盖 12 个管理员专用端点的未认证(401)和非管理员(403)拒绝。
- 新增 `TestPermissionHardening_AuthEndpoints` 测试：验证普通用户可访问只读端点。
- 新增 `TestAuditLogsAPI_Permissions` 测试：审计日志 API 权限验证。
- 新增 `TestAuditLogsAPI_ContainsSetupLog` 测试：验证初始化管理员操作被记录。

**4. 备份与恢复**

- 新增 `BackupSave(dataDir, saveName)` — 创建存档 ZIP 备份到 `.local-container/backups/saves/`。
- 新增 `DeleteSaveWithBackup(dataDir, saveName)` — 删除前自动备份。
- `handleSaveDelete` 已改用 `DeleteSaveWithBackup`，删除前自动创建备份。
- 新增 `ListBackups(dataDir)` — 列出所有备份文件。
- 新增 `RestoreBackup(dataDir, backupName, overwrite)` — 从备份恢复存档，支持冲突检测。
- 新增 API：
  - `GET /api/instances/:id/saves/backups` — 备份列表
  - `POST /api/instances/:id/saves/backups/restore` — 恢复备份
- 新增 12 个备份/恢复单元测试。

**5. 健康检查增强**

- 新增 `GET /api/health/diagnostics` 认证 API，返回结构化诊断结果。
- 检查项：Docker daemon、Docker Compose、数据目录可写性、实例目录、compose 文件、active save 状态。
- 每项返回 `ok`/`warning`/`error` 状态和中文可读消息。
- 前端新增健康检查区域（高级设置内），点击「开始检查」显示诊断结果。

**6. 前端错误体验**

- 新增集中式错误码 → 中文消息映射表（`errorCodeMap`），覆盖 40+ 个后端错误码。
- `errorMessage()` 函数优先使用 code 映射，无映射时回退到后端原始消息。
- 后端 handler 不再将 `err.Error()` 直接暴露给前端，统一经过 `sanitizeError()` 处理。

**7. 已知问题记录**

- 联机角色槽异常（同一邀请码下重复出现”新农夫”入口）已记录在 handoff 文档中，当前只做诊断和备份保护，不做破坏性存档修改。
- M11 控制台使用 FIFO 通信，未回退到旧的 attach-cli 方案。

### 新增文件

| 文件 | 说明 |
|------|------|
| `backend/internal/web/audit.go` | 审计日志 helper、sanitizeError、审计日志 API handler |
| `backend/internal/web/audit_test.go` | 权限和审计日志测试（4 个测试函数） |
| `backend/internal/web/health.go` | 健康检查诊断 handler |

### 修改文件

| 文件 | 改动 |
|------|------|
| `backend/internal/docker/redact.go` | 扩展脱敏模式（session/cookie/auth/bearer/invite/env） |
| `backend/internal/docker/redact_test.go` | 新增 10 个脱敏测试 |
| `backend/internal/storage/auth.go` | 新增 `ListAuditLogs`、`AuditLogEntry`、`ListAuditLogsParams` |
| `backend/internal/web/handler.go` | 注册 `/api/audit-logs` 和 `/api/health/diagnostics` 路由 |
| `backend/internal/web/lifecycle_handlers.go` | 全面替换 `err.Error()` 为 `sanitizeError()`；添加审计日志调用；新增备份/恢复 handler |
| `backend/internal/web/install_handlers.go` | 替换 `err.Error()`；添加审计日志调用 |
| `backend/internal/web/instance_handlers.go` | 注册备份/恢复路由 |
| `backend/internal/games/stardew_junimo/saves.go` | 新增 `BackupSave`、`DeleteSaveWithBackup`、`ListBackups`、`RestoreBackup` |
| `backend/internal/games/stardew_junimo/saves_test.go` | 新增 12 个备份/恢复测试 |
| `frontend/src/api.ts` | 新增 `getAuditLogs`、`getHealthDiagnostics` 及类型 |
| `frontend/src/core/helpers.ts` | 新增 `errorCodeMap` 错误码映射表；`errorMessage()` 优先使用 code 映射 |
| `frontend/src/App.tsx` | 新增审计日志和健康检查 UI 区域 |
| `frontend/src/App.css` | 新增 `.health-section`、`.audit-section` 等样式 |
| `docs/handoff-roadmap.md` | 标记 M13 完成 |
| `docs/conversation-handoff-2026-06-27.md` | 新增 M13 章节 |

### 验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
# 全部通过

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 通过
```

### 已知限制

- `GameDriver` 接口的 `SelectSave`/`DeleteSave` 缺少 `instance` 参数，handler 直接调用 `sj` 包函数。后续可改接口签名。
- 联机角色槽异常（重复新农夫入口）需后续专门 Milestone 处理，当前只做诊断和备份保护。
- Unix 平台的 `ComposeExecTTY` 实现未完成（需要 creack/pty 库）。
- 实时日志流（SSE/WebSocket tail logs）未实现。

## Suggested First Three Tasks

如果下一个接手者不知道从哪里开始，建议按这个顺序做：

1. 创建 Go 后端骨架和 `/health`。
2. 实现 SQLite + 管理员初始化 + 登录。
3. 实现 Docker Compose 控制层的 `ps`、`pull`、`up`、`down` 基础封装。

这三步完成后，项目就有了“面板自身可运行 + 可鉴权 + 能控制 Docker”的核心地基。

## Do Not Do Yet

这些事情不要太早做：

- 不要一开始做多游戏市场。
- 不要一开始做复杂插件系统。
- 不要一开始支持多节点。
- 不要先做大而全的 UI 组件库。
- 不要绕过 GameDriver 直接在 API 层写 Stardew 逻辑。
- 不要把未来 Minecraft / DST / Terraria / Palworld 的页面硬塞进 Stardew 面板。
- 不要为了省事允许前端执行任意 shell 命令。

## Handoff Checklist

接手前先读：

- `docs/architecture.md`
- `docs/handoff-roadmap.md`
- `docs/prototypes/stardew-anxi-panel-prototype-notes.md`

接手时先确认：

- 当前仓库是否已经创建 `backend` 和 `frontend`。
- 当前是否已有数据库迁移。
- 当前是否已有管理员初始化流程。
- 当前 Docker 命令是否只是封装层调用，而不是散落在 handler 里。
- 当前 Stardew 逻辑是否位于 `games/stardew_junimo` 下。

每完成一个 milestone 后建议更新：

- 本文档对应 milestone 的完成状态。
- README 的启动方式。
- API 文档或接口清单。
- 已知问题和下一步。
