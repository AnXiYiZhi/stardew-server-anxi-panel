# Conversation Handoff 2026-06-29

## UI-R3: 移动端与窄屏布局修复

### 目标

在 390px 宽度下修复页面横向撑破、导航触控困难、顶栏信息过载、安装成功卡拥挤四个问题。只改 CSS，不改业务逻辑、后端、API，不回退 UI-R1 字号和 UI-R2 间距变量。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/StardewPanel.css` | `.sd-main` 全局新增 `overflow-x: hidden`；`@media (max-width: 640px)` 块 5 处改动（详见下方） |

### 改动明细

| 位置 | 原规则 | 新规则 | 修复问题 |
|------|--------|--------|---------|
| `.sd-main`（全局） | `overflow-y: auto` only | 新增 `overflow-x: hidden` | 主内容区内部宽内容不再撑出页面 |
| `.sd-shell`（640px） | 无 overflow 控制 | 新增 `overflow-x: hidden` | shell 级双保险防横向滚动 |
| `.sd-sidebar .sd-nav-item`（640px） | `width:auto; min-height:auto; padding:4px 6px` | `min-width:36px; height:100%; min-height:0; padding:0 8px; gap:0; flex-shrink:0` | 图标充满侧栏高度，最小触控区 36px |
| 顶栏隐藏列表（640px） | 隐藏 `version/save/username` | 新增隐藏 `sd-topbar-name`（品牌文字）和 `sd-topbar-user .sd-tag`（角色徽章） | 顶栏只剩 logo + 状态点 + 状态文字 + 登出 |
| `.sd-install-complete-card`（640px） | `flex-direction:column; align-items:flex-start` | 同上 + `gap:8px`；按钮新增 `align-self:stretch` | 安装成功卡按钮撑满整行，不贴正文 |

### 保留不动

- UI-R1 字号变量体系、UI-R2 间距变量体系：完全不动
- 桌面（1280px）Shell 三栏布局、顶栏、导航、OpsRail：不受影响
- 960px 断点（隐藏 OpsRail + 任务页单列）：不受影响
- 业务逻辑、API、React 组件、颜色体系：均未改动

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，39 模块，JS 325.01 kB，CSS ~83 kB
```

手动验证（390px 宽度，浏览器 DevTools 模拟手机或直接拖窄）：

- 各页面不出现页面级横向滚动条（可在 body 上检查 `scrollWidth > clientWidth`）
- 左侧导航变为横向图标栏，图标可点击，高度充满侧栏 36px，最小宽度 36px
- 顶栏只显示：鸡形 logo、状态点 + 状态文字（如"运行中"）、"登出"按钮
- 品牌文字"Stardew Anxi Panel"、角色徽章"admin"在移动端不显示
- `/instances/stardew/install` 安装成功卡：✓ 图标 → 标题/说明 → 按钮各占整行，按钮撑满宽度
- 桌面（>1280px）Overview / Install / Jobs / Settings / 导航 / 顶栏无明显错位

### 下一步注意事项

**UI-R4（如需）：全局按钮体系重做**
- 当前按钮依赖 PNG 底图，尺寸受素材限制，移动端仍保持 PNG 尺寸
- 若需更好的移动端按钮体验，需配合 UI-R4 统一改

**若继续做移动端深化（UI-R3b）：**
- Overview 页的双栏主体（指标 2×2 + 事件/模组）在 390px 下可能仍需单列化，可独立评估
- 安装进度步骤条在极窄屏下步骤文字可能溢出，可在 640px 下隐藏阶段文字只留图标

---

## UI-R2: 页面间距与卡片密度统一

### 目标

适配 UI-R1 字号放大后的空间关系，建立 spacing 变量体系，统一所有页面的 padding、卡片密度和行高，消除字号放大后的贴边、拥挤和按钮高度不足问题。只做 CSS 层面调整，不改任何业务逻辑。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/stardew-theme.css` | 新增 section 12 间距变量块（8 个变量）；按钮高度 24→26px（green/tan/delete）；输入框高度 23→26px |
| `frontend/src/games/stardew/StardewPanel.css` | 多处间距、padding、gap、line-height 调整（详见下方） |
| `docs/handoff-roadmap.md` | Current Context 顶部新增 UI-R2 完成节 |
| `docs/conversation-handoff-2026-06-29.md` | 本节 |

### 新增间距变量（stardew-theme.css section 12）

```css
--sd-space-1:        4px;
--sd-space-2:        8px;
--sd-space-3:        12px;
--sd-space-4:        16px;
--sd-space-5:        20px;
--sd-card-padding:   10px 14px;
--sd-section-gap:    14px;
--sd-page-padding:   16px;
```

### StardewPanel.css 调整明细

| 选择器 | 调整内容 |
|--------|---------|
| `.sd-page` | `padding 12px → var(--sd-page-padding)=16px`，`gap 14px → var(--sd-section-gap)=14px`（变量化） |
| `.sd-topbar-logout-btn` | `height 22px → 24px`（13px 字号适配） |
| `.sd-state-card` | `padding 7px 10px → var(--sd-card-padding)=10px 14px`，`gap 5px → 6px` |
| `.sd-srv-section` | `padding 8px 10px → 10px 12px`，`gap 6px → 8px` |
| `.sd-saves-list` | `gap 6px → 8px` |
| `.sd-save-card` | `padding 7px 10px → 9px 12px` |
| `.sd-save-meta` | 补 `line-height: 1.45`（meta 行不低于 1.35 要求） |
| `.sd-mods-list` | `gap 5px → 7px` |
| `.sd-mods-card` | `padding 7px 10px → 9px 12px` |
| `.sd-mods-pending-grid` | `gap 5px → 7px` |
| `.sd-jobs-list-row` | `padding 7px 9px → 8px 10px` |
| `.sd-diag-check-row` | `padding 5px 9px → 7px 10px` |
| `.sd-settings-page` | `gap 6px → var(--sd-section-gap)=14px`（区块间隔过窄修复） |
| `.sd-settings-user-row` | `padding 5px 8px → 7px 10px` |
| `.sd-settings-audit-head/.sd-settings-audit-row` | `padding 4px 6px → 5px 8px` |
| `.sd-install-log-line` | `line-height 1.4 → 1.5`（日志行高 1.45–1.55 范围） |

### 按钮高度修正（stardew-theme.css）

| 按钮 | 原高度 | 新高度 | 原因 |
|------|--------|--------|------|
| `.sd-btn-green` | 24px | 26px | 13px 控件字号需要更多余量 |
| `.sd-btn-tan` | 24px | 26px | 同上 |
| `.sd-btn-delete` | 24px | 26px | 统一小按钮高度基准 |
| `.sd-input` | 23px | 26px | 13px 控件字号在 23px 内过紧 |
| `.sd-topbar-logout-btn` | 22px | 24px | 防止顶栏文字贴边 |

### 保留不动

- `.sd-jobs-log-window` 和 `.sd-install-log-window` 均已使用 `var(--sd-font-size-log)` ✓（UI-R1 review 问题已在上轮修复）
- `.sd-jobs-log-line` 已有 `line-height: 1.45` ✓
- 字号变量体系（UI-R1）完全不动
- 顶栏/侧栏文字无明显挤压（`clamp` 响应式适配已到位）
- 业务逻辑、API、路由、状态流均未改动

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，39 模块，JS 325.01 kB，CSS 82.79 kB
```

手动验证点：
- 各页面外边距感觉从 12px 变宽到 16px，内容不再贴边
- 设置页各区块之间有明显间隔（原 6px 改为 14px）
- 存档卡、Mod 卡、诊断检查行有更舒适的内边距
- 控制台/服务器控制页区块之间留白更清晰
- 小按钮（绿/棕/删除）高度 26px，与 13px 字号匹配，无文字顶边现象
- 输入框高度 26px，控件字号不再显得拥挤
- 设置页的用户行、审计日志行行距更舒适
- 安装日志行高 1.5，可读性提升
- 未引入任何新按钮/UI 组件（仅 CSS 调整）

### 下一步注意事项

**UI-R3（如需）：右侧 OpsRail 收窄/折叠**
- OpsRail 当前 `clamp(280px, 20vw, 360px)`，宽屏下偏宽
- 可考虑折叠/展开交互，但这是独立任务，不要混入本轮

**UI-R4（如需）：全局按钮体系重做**
- 当前按钮依赖 PNG 底图，尺寸受素材限制
- 如需系统化重做，留到 UI-R4，本轮只做最小高度修正

**UI-R5（如需）：Overview 页重排**
- Overview 当前双栏布局较复杂，有可能字号放大后影响指标格
- 独立评估，不在本轮处理

---

## UI-R1: 前端字号基线统一

### 目标

解决 Stardew 管理面板所有页面"字太小、长时间看费劲"的问题。只做字号基线和最小必要适配，不改业务逻辑、不重排页面、不改颜色体系。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/stardew-theme.css` | 新增 `--sd-font-size-*` 变量块（8个变量）；更新 `.sd-btn-green/tan/gold/red/delete/copy/start/stop/restart`、`.sd-input`、`.sd-nav-item`、`.sd-data-row` 字号引用变量 |
| `frontend/src/games/stardew/StardewPanel.css` | 批量+定向替换所有不合理字号 |
| `docs/handoff-roadmap.md` | Current Context 顶部新增 UI-R1 完成节 |
| `docs/conversation-handoff-2026-06-29.md` | 本节 |

**未改动：** 任何 `.tsx`/`.ts`/`.go` 文件、API 接口、状态逻辑、颜色变量、素材路径均未变动。

### 字号变量体系（新增到 stardew-theme.css）

```css
--sd-font-size-meta:           11px;  /* 时间戳、序号、最小徽章 */
--sd-font-size-small:          12px;  /* 次级说明、日志正文 */
--sd-font-size-body:           13px;  /* 正文、描述、提示信息 */
--sd-font-size-control:        13px;  /* 输入框、按钮、选择框 */
--sd-font-size-section-title:  14px;  /* 区块标题 */
--sd-font-size-page-title:     18px;  /* 页面标题 */
--sd-font-size-metric:         18px;  /* 指标大数字 */
--sd-font-size-log:            12px;  /* 日志正文 */
```

### 替换规则（StardewPanel.css）

| 原始字号 | 新字号 | 角色说明 |
|---------|-------|---------|
| 8.5px | 11px | `sd-srv-badge-pending` 标签 |
| 9px | 11px | 全部（时间戳、日志序号级别） |
| 9.5px | 11px | 全部（次级 meta，徽章） |
| 10px | 12px | 全部（次级说明文字） |
| 10.5px | 13px | 全部（正文、提示、表单标签） |
| 11.5px | 13px | 全部（卡片标题、guard 标题） |
| 11px（区块标题） | 14px | `.sd-srv-section-title`, `.sd-settings-section-title`, `.sd-ov-title` |
| 11px（正文） | 13px | 各页面正文内容（save card name 等） |
| 11px（OpsRail base） | 12px | `.sd-opsrail` 容器基准 |
| 11px（按钮/控件） | 通过变量 | `sd-install-input/select` → `var(--sd-font-size-control)` |
| 13px（页面标题） | 18px | `.sd-page-title` → `var(--sd-font-size-page-title)` |
| 15px（指标） | 18px | `.sd-mc-val` → `var(--sd-font-size-metric)` |
| `clamp(11px,...)` | 不动 | 导航按钮已有响应式 clamp，不修改 |

### 如何验证

1. `cd frontend && npm.cmd run build` → 应 exit 0
2. 打开面板各页面（Overview/Jobs/Settings/Mods/Diagnostics/Install）
3. 检查正文字号是否明显比之前大，标题层次是否清晰（页面标题最大，区块标题次之，正文/按钮再次，徽章/序号最小）
4. 确认没有文字溢出或布局错乱

### 下一步注意事项

- 日志区（`sd-jobs-log-window`/`sd-install-log-window`）日志正文已升到 13px（原 10.5px），如果感觉日志区太密，可适当增加 `line-height` 但不建议再改字号。
- OpsRail 右侧栏字号已统一到 12-13px，如果侧栏宽度觉得不够，调整 `clamp(280px, 20vw, 360px)` 即可，不要把字号再降回去。
- 将来新增 CSS 类时，应优先使用上述 `--sd-font-size-*` 变量，不要再硬编码 10px/10.5px 等小字号。

## FE-R12: InstallPage 首次安装向导页真实化

### 目标

把 `/instances/stardew/install` 从占位页改造为真实可用的「首次安装向导」页，接入所有已有安装 API，完整实现 Steam Guard 交互、QR 扫码、安装进度展示和 SSE 实时日志。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/InstallPage.tsx` | 完全重写（约 360 行），含安装向导全部逻辑 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增约 370 行 `sd-install-*` 像素风样式 |
| `docs/handoff-roadmap.md` | Current Context 顶部新增 FE-R12 完成节 |
| `docs/conversation-handoff-2026-06-29.md` | 本节 |

**未改动：** `InstallSection.tsx` 保留不动，无外部引用，不影响任何已有功能。后端无改动。

### 代码探查结论

1. **`getInstallOptions()`** → `GET /api/instances/:id/install-options`，返回 `{ imageTagOptions: ImageTagOption[] }`，每个选项含 `tag / label / recommended / warning / isLatest`。
2. **`installInstance(body)`** → `POST /api/instances/:id/install`，body 可含 `{ steamUsername, steamPassword, vncPassword, imageTag }` 或 `{ reuseCredentials: true, imageTag }`（重试时），返回 `{ jobId }`。
3. **`submitSteamGuardInput(jobId, input)`** → `POST /api/instances/:id/steam-guard/input`，`input` 为验证码字符串或选择数字（"1"/"2"）。
4. **SSE 格式** 同 JobsLogsPage：`log` 事件（`JobLog`）+ `finished` 事件。
5. **阶段驱动**：所有 Steam Guard 交互、进度状态均由 `instanceState.driverPhase` 驱动，无需前端猜测，后端通过 `updatePhase` 实时推送。
6. **QR 文本提取**：从日志中过滤 `[steam] ` 前缀行，拼接后在 `pre` 块中显示 ASCII QR，字体大小按行数/行宽自适应（9–12px）。
7. **日志去重**：复用 `appendUniqueLog`（按 `jobId+sequence` 去重），与 JobsLogsPage 一致。

### 各区域说明

**状态概览（`sd-state-card`）**
- `state`（绿/黄/红/灰点）、`driverPhase`（monospace tag `sd-install-phase-tag`）、`stateMessage`。

**已安装成功卡（`sd-install-complete-card`）**
- `game_installed` 时显示绿框卡：✓ 已安装 + 说明 + "前往服务器控制"（`onNavigate('server')`）。
- admin 额外显示"重新安装/修复"按钮，点击展开安装配置表单。
- 非 admin 看不到写操作，但可见成功卡和进度。

**非 admin 提示条（`sd-install-info-bar`）**
- 仅对非 admin 显示，说明仅管理员可安装。

**安装配置表单（`sd-install-form-card`）**
- 镜像版本 `<select>`（`getInstallOptions` 加载）。
- `canDirectRetry`（pull_failed / install_timeout / steam_auth_connection_failed / state=error）时只显示镜像版本，不重填账密。
- Steam 密码 / VNC 密码均支持显隐切换，`autoComplete="new-password"` 防止浏览器填充敏感字段。
- 表单内独立错误条（`installError`）。

**安装进度（`sd-install-progress-section`）**
- 5 步骤条：`done=✓绿` / `active=↻金/黄` / `error=✗红` / `pending=○灰`。
- 阶段文字标签（`phaseLabel` 函数，覆盖全部后端阶段字符串）。
- `pull_running` 阶段：Pull 镜像进度卡，从日志 `[pull:progress:N:M]` 解析，无数据时显示"等待 Docker..."。
- `game_downloading` / `steam_sdk_downloading` 阶段：下载提示卡（后端不返回结构化百分比，仅展示阶段说明）。

**Steam 认证交互区（`sd-install-guard-section`）**
- `auth_method_required`：扫码（提交 "2"）/ 账号密码（提交 "1"）选择按钮。
- `steam_guard_choice_required`：手机 App 批准（"1"）/ 输入验证码（"2"）选择按钮。
- `steam_guard_required`：验证码输入 + 提交按钮，调用 `submitSteamGuardInput(jobId, code)`。
- `steam_guard_mobile_required`：黄色等待条 + 说明文字（无操作，等待后端状态变化）。
- `steam_qr_required`：打开扫码窗口按钮（`qrText` 非空才 enabled）。

**QR 弹窗（`sd-install-qr-overlay`）**
- 固定全屏暗色 overlay。
- `pre.sd-install-qr-pre`：白色 monospace，显示从日志 `[steam]` 行提取的原始文本（含 ASCII QR 码）。
- 字体大小 `qrCodeFontSize()` 自适应（9–12px）。

**安装日志预览（`sd-install-log-window`）**
- 深色终端，最近 50 条，`info`=绿/`warn`=金/`error`=红/`debug`=蓝 着色。
- SSE 运行中时标题旁显示绿色脉冲点（复用 `sd-jobs-sse-dot`）。
- 超 50 条时提示跳转任务与日志页（`onNavigate('jobs')`）。

**SSE 连接生命周期**
- `installJobId` 变化 → 加载 `getJob` + `getJobLogs` → 非终态时建 `createJobEventSource`。
- `log` 事件 → `appendUniqueLog` 去重追加。
- `finished` 事件 → 关 SSE → `getJob` 刷新详情 → `refreshJobs` + `refreshInstanceState`。
- `onerror` → 关 SSE → 显示警告条。
- 组件卸载/`installJobId` 切换 → `cancelled=true` + `es.close()`。

**自动拾取活跃安装任务**
- `useEffect` 监听 `dashboardData.jobs`：找 `type='stardew_install' && !isTerminalJobStatus` 的任务并设为 `installJobId`。
- 处理刷新页面时已有安装任务在运行的场景。

### 真实接入 API 汇总

| API 函数 | 路径 | 权限 | 状态 |
|----------|------|------|------|
| `getInstallOptions` | `GET /api/instances/:id/install-options` | auth | ✅ 接入 |
| `installInstance` | `POST /api/instances/:id/install` | admin | ✅ 接入 |
| `submitSteamGuardInput` | `POST /api/instances/:id/steam-guard/input` | admin | ✅ 接入 |
| `createJobEventSource` | `GET /api/jobs/:id/stream` | auth | ✅ 接入（SSE） |
| `getJob` | `GET /api/jobs/:id` | auth | ✅ 接入 |
| `getJobLogs` | `GET /api/jobs/:id/logs` | auth | ✅ 接入 |
| `dashboardData.refreshInstanceState` | 公共数据层 | — | ✅ 接入 |
| `dashboardData.refreshJobs` | 公共数据层 | — | ✅ 接入 |

### 影响的接口/文件

- 无新增后端接口，无改动 API 签名，无改动 `api.ts`。
- `StardewPanel.css` 追加约 370 行，全部以 `sd-install-` 开头，不影响已有类。
- 未改动：`InstallSection.tsx`、其他所有页面组件、路由文件、后端。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，39 模块，JS 324.18 kB，CSS 81.96 kB
```

手动验证点：
- `/instances/stardew/install` 不再是占位页，显示完整安装向导。
- 未安装时：显示"未安装"灰点 + admin 看到"安装游戏"按钮，非 admin 看到提示条。
- 点击"安装游戏"展开表单：镜像版本下拉 + Steam 账密 + VNC 密码，密码字段可显隐。
- 提交后：表单消失，出现进度区，日志预览出现并实时追加（SSE 连接中时标题旁绿色脉冲点）。
- Pull 镜像阶段：步骤条第 2 步变金色↻，Pull 进度卡出现。
- Steam Guard 阶段：认证交互区出现对应输入/选择 UI。
- QR 阶段：出现"打开扫码窗口"按钮，点击后弹出暗色 QR 弹窗。
- 安装成功（`game_installed`）：绿框成功卡出现，"前往服务器控制"按钮可点。
- 已安装状态下 admin 点"重新安装/修复"展开表单，普通用户看不到此按钮。
- auth 失败状态：红点 + "重新安装（凭据错误）"按钮。
- 日志超 50 条：提示条出现，点"任务与日志"导航到 jobs 页。

### Bug Fixes（本次会话）

以下 3 个 bug 全部修复并已验证构建通过（JS 324.79 kB，CSS 81.96 kB）：

| Bug | 修复位置 | 说明 |
|-----|---------|------|
| P2 QR 提取范围过宽 | `InstallPage.tsx` `extractQrText` | 改为取最后 80 条 `[steam]` 行，避免菜单/错误混入 |
| P3 下载阶段双步骤同时 active | `InstallPage.tsx` `calcStepStatuses` | 把 `game_downloading` / `steam_sdk_downloading` 移出 `authPhases`，引入 `isPostAuthPhase`；下载中认证步骤显示 `done` |
| P3 下载阶段显示"未安装" | `InstallPage.tsx` `isInstalling` | 增加 `INSTALLING_PHASES` 检查，phase 在列表中时 `isInstalling=true`，无需等 async installJob 加载 |
| P3 非 admin 看到 Steam 认证交互区 | `InstallPage.tsx` Steam Guard 区渲染 | `!isAdmin` 时只显示"等待管理员完成验证"提示，admin 才显示完整交互区 |

### 下一步注意事项

**FE-R13 建议：OverviewPage 深化或整体 Review**
- 当前 OverviewPage 已有基本内容，可按需补充安装引导（未安装时 overview 显示安装入口）。
- 所有 9 个页面均已完成真实化，后续可进行整体 review 或 bug fix。

**旧 InstallSection.tsx**
- `InstallSection.tsx` 无外部引用，可在合适时机删除（不影响功能）。

**游戏下载百分比（待后端支持）**
- 目前 `game_downloading` / `steam_sdk_downloading` 阶段只显示文字提示，无百分比。
- 后端 `installer.go` 目前不发送结构化的游戏下载进度日志（只通过 `driverPhase` 表达阶段）。
- 如需百分比，需后端在 `lineHandler` 中解析 Steam 下载日志并发出 `[sdv:download:...]` 等结构化日志行，前端再对应解析。

---

## FE-R11: SettingsPage 设置与审计页真实化

### 目标

把 `/instances/stardew/settings` 从占位页改造为真实可用的「设置与审计」页，接入已有后端用户管理 API 和审计日志 API，对无后端能力的设置项保留 UI 入口但 disabled。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/api.ts` | 新增 `getUsers` / `createUser` / `updateUserRole` / `disableUser` / `deleteUserHard`；import 新增 `OKResponse` / `PanelUser` / `UsersResponse` |
| `frontend/src/games/stardew/pages/SettingsPage.tsx` | 完全重写（约 370 行），拆为 6 个子区块组件 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增约 255 行 `sd-settings-*` 像素风样式 |
| `docs/handoff-roadmap.md` | Current Context 顶部新增 FE-R11 完成节 |
| `docs/conversation-handoff-2026-06-29.md` | 本文档顶部新增 FE-R11 节 |

### 代码探查结论

1. **`getUsers()`** 对应 `GET /api/users`，后端 `requireAdmin`，返回 `{ users: PanelUser[] }`。
2. **`createUser()`** 对应 `POST /api/users`，admin-only；后端校验用户名/密码/角色，冲突返回 `username_taken`。
3. **`updateUserRole()`** 对应 `PATCH /api/users/:id { role }`，admin-only；不能降级最后一个 admin（`last_admin` 错误）。
4. **`disableUser()`** 对应 `DELETE /api/users/:id`（无 `?hard`），admin-only；不能禁用自己（`self_disable` 错误）。
5. **`deleteUserHard()`** 对应 `DELETE /api/users/:id?hard=true`，admin-only；永久删除。
6. **`getAuditLogs()`** 已存在于 `api.ts`，对应 `GET /api/audit-logs?limit=&offset=`，admin-only，max limit=200。
7. **`dashboardData.versionInfo`** 已由公共数据层 `useStardewDashboardData` 填充，直接使用。

### 各区域说明

**当前账号（`sd-settings-account-card`）**
- 展示 `user.username`、角色标签（管理员/普通用户双色区分）、登录状态绿点。
- 退出登录按钮调用 props `onLogout`，无需额外 API。

**面板版本（`sd-settings-info-grid`）**
- `versionInfo.version` / `buildDate`（`formatDate` 格式化）/ `commit`，全部 monospace 字体。
- 运行模式固定显示 `Single Game Mode` 蓝色标签。

**用户管理（`UserManagementSection`）**
- admin：加载并展示用户列表（含禁用态行 `sd-settings-user-inactive`）；新建用户表单可展开收起；角色切换/禁用/永久删除均弹像素风 `sd-confirm-overlay` 二次确认。
- 自防护：当前用户自己的行，修改/禁用/删除按钮全部 `disabled` + `title` 说明。
- 普通用户：整个区块替换为一行权限锁定提示，不加载 API。

**审计日志（`AuditLogsSection`）**
- 每页 20 条，`AUDIT_PAGE_SIZE = 20`；`total` 超过一页时显示上一页/下一页按钮。
- 操作 `action` 字段通过 `AUDIT_ACTION_LABELS` 映射为中文（22 个常用动作）。
- 目标类型 `targetType` 通过 `TARGET_TYPE_LABELS` 映射为中文（用户/实例/存档/Mod/命令/系统）。
- 加载失败显示红色错误条 + 重试按钮；空结果显示提示文字。
- 普通用户：整个区块替换为权限锁定提示。

**安全与权限（`SecuritySection`）**
- 5 条静态说明：Session 认证、密码存储（Argon2id）、Docker Socket 风险（黄点警告）、操作审计、日志脱敏。
- 纯展示，无 API 调用。

**待接入设置（`PendingSettingsSection`）**
- 主题、语言、多游戏模式、备份策略、通知设置、会话超时——全 disabled 按钮 + 「后端待接入」徽章。

### 真实接入 API 汇总

| API 函数 | 路径 | 权限 | 状态 |
|----------|------|------|------|
| `getUsers` | `GET /api/users` | admin-only | ✅ 接入 |
| `createUser` | `POST /api/users` | admin-only | ✅ 接入 |
| `updateUserRole` | `PATCH /api/users/:id` | admin-only | ✅ 接入 |
| `disableUser` | `DELETE /api/users/:id` | admin-only | ✅ 接入 |
| `deleteUserHard` | `DELETE /api/users/:id?hard=true` | admin-only | ✅ 接入 |
| `getAuditLogs` | `GET /api/audit-logs` | admin-only | ✅ 接入 |
| `dashboardData.versionInfo` | 公共数据层 | 所有用户 | ✅ 接入 |

### 影响的接口/文件

- 无新增后端接口，无改动 API 签名。
- `api.ts` 新增 5 个用户管理函数，已有 `getAuditLogs` / `AuditLogEntry` / `AuditLogsResponse` 不改动。
- `StardewPanel.css` 追加约 255 行，不影响已有类（全部以 `sd-settings-` 开头）。
- 未改动：其他所有页面组件。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0
```

手动验证点：
- `/instances/stardew/settings` 不再是占位页，显示 6 个区块。
- 当前账号区：显示登录用户名、角色标签和退出按钮，点击退出登录有效。
- 面板版本区：显示后端 `/api/version` 真实数据（版本号/构建时间/Commit）。
- 用户管理区（admin）：显示用户列表；新建用户表单展开/提交成功后刷新列表；角色切换弹二次确认；禁用/永久删除弹二次确认；自己的行按钮 disabled。
- 用户管理区（普通用户）：显示权限锁定提示，不展示用户数据。
- 审计日志区（admin）：显示最近操作记录，操作名为中文；超过 20 条时显示翻页按钮；刷新按钮有效；失败时显示错误条 + 重试。
- 审计日志区（普通用户）：显示权限锁定提示。
- 安全区：5 条安全说明，Docker Socket 风险前显示黄点。
- 待接入设置：6 个 disabled 按钮，标注「后端待接入」。

### 下一步注意事项

**FE-R12 建议：InstallPage 安装向导页真实化**
- 后端已有完整安装流程 API：`prepareInstance`、`installInstance`、`submitSteamGuardInput`、SSE 日志流。
- 安装页当前可能仍为旧实现，可评估是否需要完全重写为像素风。

**用户管理后续扩展（如有需求）：**
- 重置密码功能：`PATCH /api/users/:id { password }` 已支持，前端未接入。
- 启用已禁用用户：`PATCH /api/users/:id { isActive: true }` 已支持，前端未接入。

---

## FE-R10: DiagnosticsPage 诊断与健康检查页真实化

### 目标

把 `/instances/stardew/diagnostics` 从占位页改造为真实可用的诊断页，接入已有健康检查和支持包导出 API，对无后端数据的资源趋势区显示清晰"待接入"空状态。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/DiagnosticsPage.tsx` | 完全重写 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增约 220 行 `sd-diag-*` 像素风样式 |
| `docs/handoff-roadmap.md` | Current Context 顶部新增 FE-R10 完成节 |

### 代码探查结论

1. **`getHealthDiagnostics()`** 已在 `api.ts` 中实现，对应 `GET /api/health/diagnostics`，返回 `{ status: string, checks: HealthCheck[] }`，后端 `requireAuth`（所有登录用户可用）。
2. **`downloadSupportBundle()`** 已在 `api.ts` 中实现，对应 `POST /api/instances/:id/support-bundle`，后端 `requireAdmin`（admin-only）。
3. **检查项字段**：后端返回 `name`（`docker_daemon` / `docker_compose` / `data_dir` / `instance_dir` / `compose_file` / `active_save`）、`status`（ok/warning/error）、`message`（中文说明）。
4. **`dashboardData.health / healthError / refreshHealth()`** 均已在公共数据层就绪，DiagnosticsPage 初始渲染直接使用；重新检查时用 `getHealthDiagnostics()` 本地请求以获得独立 loading 状态，成功后同步调用 `refreshHealth()` 更新公共层。

### 各区域说明

**总状态面板（`sd-diag-status-panel`）**
- 大圆点（16px）颜色根据 `overallStatus` 区分：ok=绿/warning=黄/error=红，带轻微发光阴影。
- 状态标签（系统正常/存在警告/存在错误）+ 计数行（✓ N 正常 / ⚠ N 警告 / ✕ N 错误）。
- 面板背景色随状态变化：ok=淡绿/warning=淡黄/error=淡红。

**检查项明细（`sd-diag-checks`）**
- 每行 `sd-diag-check-row`：状态点 + 名称（`sd-diag-check-name`，固定最小宽度 90px）+ 消息（`sd-diag-check-msg`）。
- 行背景/边框颜色随 status 着色（ok/warning/error 三档）。
- 名称通过 `CHECK_NAME_LABELS` 字典中文映射，fallback 显示原始 `name`。

**告警与建议（`sd-diag-alerts`）**
- 过滤 `status !== 'ok'` 项单独渲染。
- 全部正常时显示绿色"暂无告警"提示条；未加载数据时提示"请先点击重新检查"。

**快捷工具（页头右侧）**
- 重新检查：独立 `refreshing` state，避免阻塞公共数据层。加载中显示"检查中…"。
- 导出诊断包：`downloadSupportBundle()` 触发浏览器下载；非 admin 显示 disabled + title 说明"仅管理员可导出"；导出失败显示错误条。

**资源趋势（`sd-diag-resource-pending`）**
- 纯占位：虚线边框 + 说明文字，不显示假数据。

### 真实接入 API 汇总

| API 函数 | 路径 | 权限 | 状态 |
|----------|------|------|------|
| `getHealthDiagnostics` | `GET /api/health/diagnostics` | 所有登录用户 | ✅ 接入 |
| `downloadSupportBundle` | `POST /api/instances/:id/support-bundle` | admin-only | ✅ 接入 |
| 资源趋势 API | — | — | ❌ 后端不存在 |

### 影响的接口/文件

- 无新增后端接口，无改动 API 签名。
- `StardewPanel.css` 追加约 220 行，不影响已有类（全部以 `sd-diag-` 开头）。
- 未改动：`OverviewPage`、`ServerControlPage`、`SavesPage`、`JobsLogsPage`、`PlayersPage`、`ModsPage`。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，39 模块，JS ~291 kB，CSS ~71 kB
```

已验证通过（exit 0），JS 291.25 kB，CSS 70.88 kB。

手动验证点：
- `/instances/stardew/diagnostics` 不再是占位页，显示总状态面板 + 检查项列表 + 告警区 + 快捷工具 + 资源趋势占位。
- 总状态面板颜色与实际 health.status 对应。
- 检查项名称显示为中文（Docker 服务 / Docker Compose / 数据目录 / 实例目录 / Compose 文件 / 启动存档）。
- 点击"重新检查"显示"检查中…"状态，完成后更新数据。
- health 加载失败时显示红色错误条 + 重试按钮。
- admin 用户可点击"导出诊断包"触发浏览器下载。
- 非 admin 用户：导出按钮 disabled，title 说明"仅管理员可导出诊断包"。
- 告警区：有 warning/error 时显示汇总列表；无告警时显示绿色"暂无告警"条。
- 资源趋势区显示"待接入"提示，无假数据。
- 左侧导航、Overview、ServerControlPage、SavesPage、JobsLogsPage、PlayersPage、ModsPage 不受影响。

### 下一步注意事项

**FE-R11 建议：SettingsPage 设置页真实化**
- 后端已有 `/api/users` 用户管理 API（`GET/POST/PATCH/DELETE /api/users/:id`）。
- 审计日志：`GET /api/audit-logs?limit=&offset=` 已实现，可在设置页接入审计日志查看。
- 版本信息：`dashboardData.versionInfo` 已就绪，可在设置页展示版本 / commit / buildDate。

**资源趋势后续接入建议（给后端开发者）**
- 需新增 `GET /api/instances/:id/metrics` 或类似端点，返回 CPU/内存/磁盘使用数据。
- 前端 `DiagnosticsPage` 的"资源趋势"区已预留位置，去掉占位条件渲染即可接入。

---

## FE-R9: ModsPage 模组管理页真实化

### 目标

把 `/instances/stardew/mods` 从占位页改造为真实可用的模组管理页，接入所有已有后端 Mod API，对无后端 API 的功能保持清晰的"待接入"空状态，不写死演示数据。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/ModsPage.tsx` | 完全重写（占位改为真实页面） |
| `frontend/src/games/stardew/StardewPanel.css` | 新增约 300 行 `sd-mods-*` 像素风样式 |
| `docs/handoff-roadmap.md` | Current Context 顶部新增 FE-R9 完成节 |

### 代码探查结论

1. **已有完整 Mod API**：`getMods`、`uploadMod`、`deleteMod`、`exportMods` 均在 `api.ts` 中实现，路径分别为 `GET /mods`、`POST /mods/upload`、`DELETE /mods/:modId`、`POST /mods/export`。
2. **旧 `ModsSection.tsx`**：文件仍存在，但无任何外部引用（已确认），保留不动。
3. **`dashboardData.mods`**：公共数据层已加载 Mod 列表，`ModsPage` 挂载时优先使用此数据，避免重复请求；操作完成后调用 `dashboardData.refreshMods()` 同步。
4. **running/starting 保护**：上传和删除 running/starting 时均禁用，与后端保护一致。

### 各区域说明

**概览统计行（`sd-mods-overview`）**
- 已安装数量 / 服务器状态（彩色状态点）/ 重启需求标志 / 解析失败数量。
- 全部来自真实 API 数据，无硬编码。

**模组列表（`sd-mods-list`）**
- 每张卡片（`sd-mods-card`）：名称 + 版本、UniqueID、作者、目录名、描述。
- 解析失败时（`parseError` 非空）卡片变红色边框 + 错误信息。
- 删除按钮 `writeDisabled`（`isRunning || !isAdmin`）保护，title 说明原因。

**操作区（页头右侧）**
- 刷新列表：所有用户可用，loading 时显示"刷新中…"。
- 导出 Mod 包：调用 `exportMods()` 触发浏览器下载，mods 为空时 disabled。
- 上传 Mod：admin 且非 running/starting 时可用，点击弹出上传弹窗。

**上传流程**
- 像素风弹窗（`sd-mods-modal-overlay + sd-mods-modal-card`），file input 接收 `.zip`。
- 上传中按钮变"上传中…"并禁用，成功后显示全局成功横幅 4 秒，提示"需要重启"。
- 失败时弹窗内显示后端错误。

**删除流程**
- 点击"删除"先打开 `sd-confirm-dialog` 像素风二次确认弹窗，不使用 `window.confirm`。
- 确认后调用 `deleteMod(mod.id)`，成功后同步刷新列表和公共数据层。

**待接入功能区**
- 启用/禁用 / 依赖检查 / 更新检查：全部 disabled 按钮 + 待接入徽章 + 说明文字。

### 真实接入 API 汇总

| API 函数 | 路径 | 状态 |
|----------|------|------|
| `getMods` | `GET /api/instances/:id/mods` | ✅ 接入 |
| `uploadMod` | `POST /api/instances/:id/mods/upload` | ✅ 接入 |
| `deleteMod` | `DELETE /api/instances/:id/mods/:modId` | ✅ 接入 |
| `exportMods` | `POST /api/instances/:id/mods/export` | ✅ 接入 |
| 启用/禁用 API | — | ❌ 后端不存在 |
| 依赖检查 API | — | ❌ 后端不存在 |
| 更新检查 API | — | ❌ 后端不存在 |

### 影响的接口/文件

- 无新增后端接口，无改动 API 签名。
- `StardewPanel.css` 追加约 300 行，不影响已有类（全部以 `sd-mods-` 开头）。
- `ModsSection.tsx` 保留不动，无任何引用，不影响旧入口。
- 未改动：`OverviewPage`、`ServerControlPage`、`SavesPage`、`JobsLogsPage`、`PlayersPage`。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，39 模块，JS ~287 kB，CSS ~67 kB
```

已验证通过（exit 0），JS 287.09 kB，CSS 67.30 kB。

手动验证点：
- `/instances/stardew/mods` 不再是占位页，显示概览统计行 + Mod 列表 + 操作区 + 待接入区。
- Mod 为空时显示像素风空状态（带"上传 Mod"按钮）。
- 非 admin 用户：上传/删除按钮可见但 disabled，title 说明"仅管理员可用"。
- 服务器运行中：顶部显示金色警示条，上传/删除 disabled，title 说明"服务器运行中"。
- 上传弹窗：file input 接收 .zip，上传中禁用，成功显示成功横幅，失败显示错误。
- 删除：弹出 `sd-confirm-dialog` 二次确认（不是 `window.confirm`），确认后删除并刷新。
- 导出：点击"导出 Mod 包"触发浏览器下载（无 Mod 时 disabled）。
- 待接入区三个按钮全部 disabled，显示待接入徽章和说明。
- 左侧导航、Overview、ServerControlPage、SavesPage、JobsLogsPage、PlayersPage 不受影响。

### 下一步注意事项

**FE-R10 建议：DiagnosticsPage 诊断页真实化**
- `dashboardData.health` 已有完整的 `/api/health/diagnostics` 数据（Docker / Compose / 数据目录 / 实例 / 存档）。
- 旧 App.tsx 有诊断显示逻辑可参考迁移。
- 诊断包导出（`/api/instances/:id/support-bundle`）已实现，可接入"导出诊断包"按钮。

**ModsSection 旧组件**
- `ModsSection.tsx` 无外部引用，可在合适时机删除（不影响当前功能）。

**Mod 高级能力**（需后端支持才能接入前端 UI）：
- 按 Mod 单独启用/禁用。
- Mod 依赖图完整性检查。
- Nexus Mods / URL 在线安装。
- SMAPI 兼容性 / 版本更新检查。

---

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

---

## UI-R4: 全局按钮体系整理与可点击控件优化

### 目标

梳理并统一全局 PNG 按钮视觉层级，修复 SettingsPage 因使用不存在的 `sd-btn` 类导致按钮完全无样式的重大 bug，添加 hover 状态，强化危险操作视觉区分，优化移动端按钮布局。只改 CSS 和最小必要 TSX 类名，不改业务逻辑。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/SettingsPage.tsx` | 替换全部无效 `sd-btn` 类；修复 ConfirmDialog 元素类型和按钮顺序 |
| `frontend/src/games/stardew/stardew-theme.css` | 新增 hover 状态；`sd-btn-copy` 高度对齐；`sd-btn-delete` 暗红文字 |
| `frontend/src/games/stardew/StardewPanel.css` | `max-width: 640px` 新增按钮组布局规则 |

### 改动明细

#### SettingsPage.tsx（最重要：修复功能性 bug）

SettingsPage 之前使用了 `sd-btn`（不存在的 CSS 类），导致 **退出登录、新建用户、刷新、创建、取消、升/降权、禁用、删除、翻页** 等所有按钮都渲染为浏览器默认无样式按钮，既破坏像素风，也在不同浏览器下表现不一致。

替换规则：
- `sd-btn` → `sd-btn-tan`（中性/次级操作）
- `sd-btn sd-btn-green` → `sd-btn-green`（主操作）
- `sd-btn sd-btn-red`（危险，如禁用/删除用户/弹框确认） → `sd-btn-delete`（小危险按钮）
- "退出登录" 改为 `sd-btn-tan`（退出不是删除操作，用中性色）

ConfirmDialog 修复：
- `div.sd-confirm-title` → `<h3>`（CSS 规则是 `.sd-confirm-dialog h3`）
- `div.sd-confirm-body` → `<p>`（CSS 规则是 `.sd-confirm-dialog p`）
- 按钮顺序：确认在前→取消在前、危险操作在后（与 ServerControlPage 一致）

#### stardew-theme.css

- **新增 hover 状态**：所有 PNG 按钮（green/tan/gold/red/start/stop/restart/copy/delete）添加 `filter: brightness(1.08)`
- **`sd-btn-copy` 高度**：25px → 26px（与全部小按钮对齐）
- **`sd-btn-delete` 文字颜色**：`#2c1a0a`（深棕）→ `#8b2020`（暗红），让危险小按钮的文字色与普通次操作有视觉区别

#### StardewPanel.css（max-width: 640px）

- 安装页 `.sd-install-actions/.sd-install-form-actions/.sd-install-guard-actions`：改为 `flex-direction: column; align-items: stretch`，按钮全宽排列
- 存档/Mods 页头部操作区：`width: 100%; justify-content: flex-start`，窄屏下不再超宽
- 设置页 `.sd-settings-section-toolbar`：`flex-wrap: wrap`
- 设置页 `.sd-settings-user-actions`：`flex-wrap: wrap; gap: 4px`，用户操作按钮可换行
- Jobs 页 `.sd-jobs-toolbar-actions`：`flex-wrap: wrap`

### 保留不动

- UI-R1 字号变量、UI-R2 间距变量、UI-R3 移动端导航规则
- `sd-btn-start/stop/restart` 固定尺寸（PNG 底图限制，不修改）
- `sd-btn-xs` 的 `!important` 覆盖（仅用于 PlayersPage 待接入禁用按钮，不影响主流程）
- 按钮颜色主题方向
- 业务逻辑、API、React 组件交互

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，39 模块，JS ~325 kB，CSS ~85 kB
```

手动验证要点：
- **SettingsPage（settings 路由）**：退出登录/新建用户/刷新/创建/角色切换/禁用/删除/翻页按钮全部有像素风 PNG 样式，不再是浏览器默认按钮
- **危险操作（禁用/删除按钮）**：文字呈暗红色，与 tan 灰色次按钮有明显区别
- **所有 PNG 按钮 hover**：悬停时有轻微亮度提升反馈
- **390px 宽度**：安装页操作按钮纵向全宽排列，设置页用户操作按钮可换行

### 下一步注意事项

- **UI-R5（如需）**：Overview 2×2 指标格在 390px 下是否需要单列化可独立评估
- **安装步骤条**：极窄屏下步骤文字可能超出，可在 640px 下隐藏步骤文字只留图标
- **sd-btn-xs**：当前用于 PlayersPage 禁用待接入按钮，有 `!important` 覆盖，若后续 PlayersPage 真实化时需要重新设计
