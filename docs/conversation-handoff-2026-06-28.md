# Conversation Handoff 2026-06-28 / 2026-06-29

## Bug fix batch: SavesPage P1 + ServerControlPage P2/P3（2026-06-29）

### 改了什么

| 文件 | 改动 |
|------|------|
| `frontend/src/games/stardew/pages/SavesPage.tsx` | 从占位改为渲染 `SavesSection`，props 桥接 StardewPageProps |
| `frontend/src/games/stardew/pages/ServerControlPage.tsx` | 加 `onNavigate` 解构；修复"存档页"按钮；命令加载错误态；`handleStart` 刷新邀请码；剪贴板错误处理 |
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | `handleStart` 刷新邀请码；剪贴板错误处理 |

### 各问题修复细节

**P1 – SavesPage 真实化**

`SavesSection` 原本有 `state / isAdmin / onJobStarted / onStateRefresh / refreshTrigger` 接口。新 `SavesPage` 用 `StardewPageProps` 做桥接：
- `state` ← `instanceState?.state ?? ''`
- `isAdmin` ← `user.role === 'admin'`
- `onJobStarted` ← `(jobId) => { dashboardData.refreshJobs(); onNavigate('jobs') }`（跳到任务日志页查看进度）
- `onStateRefresh` ← `dashboardData.refreshInstanceState`
- `refreshTrigger` 不传（`SavesSection` 内部自己维护存档列表，与 `dashboardData.saves` 各自独立，可接受）

**P2a – "存档页"按钮 onClick**

`onClick={() => void 0}` → `onClick={() => onNavigate('saves')}`，同时补上 `onNavigate` 解构。

**P2b – 命令加载错误态**

新增 `commandsLoading: boolean` 和 `commandsError: string | null`。`loadCommands()` catch 里 `setCommandsError(errorMessage(e))`，finally `setCommandsLoading(false)`。JSX 改为三分支：错误（显示红字 + 重试按钮） → 加载中（显示"正在加载…"） → 已加载（显示命令下拉）。

**P3a – 剪贴板复制**

两处 `handleCopy()`：从 `void .writeText().then(success)` 改为 `.then(success, failure)` 两参数回调，不再 `void` 掉整个 Promise（否则失败分支永远不执行）。失败时 `setCopyError(true)` + 3s 后重置，JSX 显示"复制失败，请手动选取"提示。

**P3b – 启动后刷新邀请码**

`handleStart()` 在 `ServerControlPage` 和 `OverviewPage` 均追加 `dashboardData.refreshInviteCode()`，让启动成功后邀请码区域及时更新，不需要用户手动点刷新。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，39 模块，无 TS 错误
```

已验证通过（exit 0），JS bundle 260.08 kB，CSS 51.38 kB。

手动验证点：
- `/instances/stardew/saves` 页面展示存档列表、创建存档弹窗、上传存档弹窗。
- `ServerControlPage` 快捷操作区"存档页"按钮点击后跳转到存档路由。
- 服务器运行时命令列表加载失败 → 显示"加载命令列表失败"红字 + 重试按钮，不再卡"正在加载"。
- HTTP 环境下复制按钮显示"复制失败，请手动选取"。
- 启动服务器后邀请码区域自动刷新（不需要手动点刷新）。

### 下一步注意事项

- `SavesSection` 在 `SavesPage` 内部仍有自己的 `getSaves()` 请求（与 `dashboardData.saves` 各自独立）；目前是可接受的重复，后续可以用 `refreshTrigger` 机制或把 `dashboardData.saves` 传入替代内部加载。
- `onJobStarted` 目前收到 jobId 后跳转到 jobs 页，但 `JobsLogsPage` 还是占位，用户看不到任务详情；下一步应迁移 `JobsSection`（SSE 日志流）进 `JobsLogsPage`。

## FE-R5: 服务器控制页（ServerControlPage）真实化（2026-06-28）

### 目标

把 `/instances/stardew/server` 路由从"显示状态 + 列出待迁移功能"的占位页面，升级为真实可用的 Stardew 服务器控制页面，视觉继续贴合像素原型风格。

### 改了什么

**修改文件：**

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/ServerControlPage.tsx` | 完整重写（原占位版 ~60 行 → 新版 ~280 行） |
| `frontend/src/games/stardew/StardewPanel.css` | 新增 `sd-srv-*` 前缀 CSS 类（7 个）|

**未改动：** 后端任何 API、`useStardewDashboardData.ts`、`StardewPanel.tsx`、`OverviewPage.tsx`、`stardew-routes.ts`、`stardew-theme.css`。

### 页面结构

```
sd-page
├── 页面标题（图标 + 服务器控制 + 描述）
├── 状态卡片（state 中文标签 + 状态徽章 + 实例名 + driverPhase + stateMessage + 更新时间 + 当前存档 + 版本）
├── 生命周期控制区（sd-srv-section）
│   ├── sd-btn-start / sd-btn-stop / sd-btn-restart
│   ├── actionBusy 指示器
│   ├── actionError 错误提示
│   └── isStarting 提示文字
├── 邀请码区（sd-srv-section）
│   ├── 邀请码 + 复制按钮（sd-btn-copy）
│   ├── 刷新按钮（调用 dashboardData.refreshInviteCode）
│   └── 未运行/加载中/错误的空状态
├── 全服消息区（sd-srv-section）
│   ├── 文本输入 + 发送按钮（调用 sendSay API）
│   └── 未运行时的禁用提示
├── 控制台命令区（sd-srv-section）
│   ├── 命令下拉（从 getCommands 加载 allowlist）
│   ├── 执行按钮（调用 runCommand API）
│   └── 命令描述 + 执行结果
├── 快捷操作区（sd-srv-section）
│   ├── 保存世界（disabled + 待接入标签）
│   ├── 备份存档（disabled + 待接入标签）
│   ├── 计划重启（disabled + 待接入标签）
│   └── 服务器设置（disabled + 待接入标签）
└── 危险操作确认弹框（stop/restart 二步确认）
```

### 已真实接入的 API

| 功能 | API | 备注 |
|------|-----|------|
| 启动服务器 | `startInstance()` | 按状态机控制可用性；操作后刷新 state+jobs |
| 停止服务器 | `stopInstance()` | 需要确认；操作后刷新 state+inviteCode+jobs |
| 重启服务器 | `restartInstance()` | 需要确认；操作后刷新 state+inviteCode+jobs |
| 邀请码展示 | `dashboardData.inviteCode` | 公共数据层；刷新按钮 → `refreshInviteCode()` |
| 邀请码复制 | `navigator.clipboard.writeText` | 复制成功显示 ✓ 反馈 |
| 全服喊话 | `sendSay()` | 优雅处理 command_not_supported |
| 命令列表 | `getCommands()` | 服务器运行时自动加载 |
| 执行命令 | `runCommand()` | 从 allowlist 选择，展示输出 |

### 待接入功能（保留禁用 UI）

| 功能 | 原因 |
|------|------|
| 保存世界 | 后端无手动 world save API |
| 备份存档 | 后端无手动备份触发 API |
| 计划重启 | 功能未实现 |
| 服务器设置（端口/可见性/密码） | 后端无对应 API |

### 影响接口/文件

不影响后端 API，不改业务数据流。

影响文件：

```text
frontend/src/games/stardew/pages/ServerControlPage.tsx
frontend/src/games/stardew/StardewPanel.css
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-28.md
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，无 TS 错误
```

已验证：`tsc -b && vite build` exit 0，JS bundle 239.51 kB，CSS 45.96 kB。

手动验证点：
- 导航到 `/instances/stardew/server`，页面不再是占位。
- 服务器 stopped 状态：启动按钮可用，停止/重启禁用。
- 服务器 running 状态：停止/重启可用，点击有确认弹框。
- 邀请码区：running 时显示邀请码和复制按钮；stopped 时显示"服务器未运行"提示。
- 全服消息：输入后点发送，或按 Enter 发送。
- 命令区：服务器运行时加载命令下拉，执行后显示结果。
- 快捷操作按钮：全部禁用，显示"待接入"标签。
- Overview 页生命周期按钮功能不受影响。
- 左侧导航、顶部栏、背景颜色无改变。

### 下一步注意事项

- **Overview 和 ServerControlPage 的生命周期逻辑目前各自独立**：两者调用相同 API，但 Overview 是"快捷入口"定位，ServerControlPage 是"完整控制"定位，不建议合并为共享组件，否则 props 会复杂化。
- **`sendSay` 当前可能返回 `command_not_supported`**：前端已通过 `errorMessage()` 优雅展示，无需后端修改。
- **命令列表 (`getCommands`) 每次进入页面会重新加载**：如果服务器从 stopped 变为 running，组件内 `isRunning` 变化会触发 `useEffect` 重新加载。
- **邀请码复制**：依赖 `navigator.clipboard` API，在 HTTP（非 HTTPS 且非 localhost）下不可用；当前无降级处理，后续可考虑 `document.execCommand` fallback。
- **FE-R6（SavesPage）**、**FE-R7（JobsLogsPage）** 是下一步的主要目标，参考 `docs/frontend-ui-refactor-implementation-plan.md`。

## FE-R4l: 左侧导航内容组统一长度（2026-06-28）

### 目标

用户澄清“统一长度没让你统一宽度”，因此本次目标是保留每个按钮的独立背景宽度，同时统一图标+文字这组内容的视觉长度和排布节奏。

### 改了什么

修改 `frontend/src/games/stardew/StardewPanel.css`：

- 保留每个 route 的 `--sd-nav-w` / `--sd-nav-active-w` 独立宽度。
- 删除各 route 对 `--sd-nav-pad-left` / `--sd-nav-pad-right` 的单独覆盖。
- 所有 route 共用：
  - `--sd-nav-pad-left: 10`
  - `--sd-nav-pad-right: 7`
  - `--sd-nav-gap: 4`
- 新增 `--sd-nav-text-w: 60`。
- `.sd-sidebar .sd-nav-item span` 固定为 `calc(100cqi * var(--sd-nav-text-w) / 105)`，让文字区域长度统一。

### 影响接口/文件

不影响后端接口，不改业务数据流。

影响文件：

```text
frontend/src/games/stardew/StardewPanel.css
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-28.md
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

已验证通过：`tsc -b && vite build` exit 0。

### 下一步注意事项

- 当前规则：背景宽度独立，内容组长度统一。
- 如果继续微调“长度”，优先改 `--sd-nav-text-w`，不要改 `--sd-nav-w`。

## FE-R4k: 左侧导航每个按钮单独视觉宽度（2026-06-28）

### 目标

用户要求不要统一每个按钮宽度，而是根据每个按钮背景图/视觉形态单独匹配；active 绿色背景也要按自己的大小完整覆盖。

### 改了什么

修改 `frontend/src/games/stardew/StardewPanel.css`：

- 为每个 route 单独设置视觉宽度变量，不再所有按钮共享同一宽度。
- 当前 route 变量：

```text
overview     96 / active 96
server      110 / active 106
saves       102 / active 105, active height 28
jobs        126 / active 122
players      96 / active 96
mods         96 / active 96
diagnostics 106 / active 102
install      96 / active 96
settings     96 / active 96
```

- `saves` 继续使用 `nav_item_active_saves_blank.png`。
- active 状态宽高由对应 route 自己的变量决定。
- 保留源像素内边距变量、水平靠左、垂直居中、字体和图标放大。

### 影响接口/文件

不影响后端接口，不改业务数据流。

影响文件：

```text
frontend/src/games/stardew/StardewPanel.css
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-28.md
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

已验证通过：`tsc -b && vite build` exit 0。

### 下一步注意事项

- 如果后续用户继续做像素级微调，优先改各 route 的 `--sd-nav-w` / `--sd-nav-active-w` / padding 源像素变量。
- 不要再合并回统一按钮宽度。

## FE-R4j: 左侧导航内边距按源 PNG 像素坐标缩放（2026-06-28）

### 目标

用户反馈“又不实际匹配自己的背景宽度了”，指出 FE-R4i 虽然改为靠左并放大，但内边距仍然没有跟随当前 PNG 自己的盒子。

### 改了什么

修改 `frontend/src/games/stardew/StardewPanel.css`：

- 为导航项新增源图像素变量：
  - `--sd-nav-pad-left`
  - `--sd-nav-pad-right`
  - `--sd-nav-gap`
- 普通态按 `105×29` 源 PNG 坐标缩放：默认 `pad-left: 10`、`pad-right: 7`、`gap: 4`。
- active 态按 `99×29` 源 PNG 坐标缩放：`pad-left: 9`、`pad-right: 6`、`gap: 4`。
- padding/gap 从粗略 `clamp(...cqi...)` 改为 `calc(100cqi * sourcePixel / 105)`，与当前 PNG 宽度的缩放比例一致。
- 保留垂直居中、水平靠左、放大的字体和图标。

### 影响接口/文件

不影响后端接口，不改业务数据流。

影响文件：

```text
frontend/src/games/stardew/StardewPanel.css
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-28.md
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

已验证通过：`tsc -b && vite build` exit 0。

### 下一步注意事项

- 后续视觉微调左侧导航时，优先调整源像素变量，而不是直接写侧栏级别的 `cqi` padding。
- 如果为某个 route 增加新的 active PNG，需要同步设置它的源宽高和源内边距。

## FE-R4i: 左侧导航内容靠左且放大（2026-06-28）

### 目标

用户要求左侧导航“上下居中，左右形式是靠左，字体和按钮图标可以适当再大一点”。

### 改了什么

修改 `frontend/src/games/stardew/StardewPanel.css`：

- 保留 FE-R4h 的按 PNG 实际尺寸计算规则。
- `.sd-sidebar .sd-nav-item` 从 `justify-content: center` 改为 `justify-content: flex-start`。
- 水平 padding 调整为 `clamp(12px, 8cqi, 15px)`。
- 图标文字间距调整为 `clamp(6px, 4.4cqi, 8px)`。
- 导航字体提升到 `clamp(11px, 7.6cqi, 13px)`。
- 导航图标提升到 `clamp(17px, 12.5cqi, 20px)`。
- 垂直方向继续依赖 flex `align-items: center` 保持居中。

### 影响接口/文件

不影响后端接口，不改业务数据流。

影响文件：

```text
frontend/src/games/stardew/StardewPanel.css
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-28.md
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

已验证通过：`tsc -b && vite build` exit 0。

### 下一步注意事项

- 当前有效视觉：导航按钮盒子按 PNG 实际尺寸，内容垂直居中、水平靠左。
- 若继续放大字体，需要检查最长中文标签是否溢出；当前仍保留单行省略。

## FE-R4h: 左侧导航按各 PNG 实际尺寸居中（2026-06-28）

### 目标

用户要求：文字居中显示在 PNG 中间；每个 PNG 的宽度不一样，要按实际素材匹配；点击后的绿色 active 图也要完全覆盖它自己的背景图大小。

### 改了什么

修改文件：

```text
frontend/src/games/stardew/StardewPanel.tsx
frontend/src/games/stardew/StardewPanel.css
```

具体改动：

- `StardewPanel.tsx` 给每个左侧导航按钮增加 `data-route={entry.route}`，方便 CSS 对不同 route 使用不同 active 尺寸。
- 普通导航基准尺寸按 `nav_item_default_blank.png`：`105×29`。
- 通用绿色 active 尺寸按 `nav_item_active_green_blank.png`：`99×29`。
- `saves` active 使用专用 `nav_item_active_saves_blank.png`：`105×28`。
- 底部帮助宽高按 `nav_quick_help_blank.png`：`96×28`，但仍保持透明背景以免偏亮素材影响木纹颜色。
- active 状态会改变按钮自身宽高，让绿色 PNG 按自己的尺寸完整覆盖。
- 文本和图标通过 flex 在当前 PNG 盒子里居中。
- 普通态仍透明显示侧栏木纹原色，不铺 `--sd-img-nav-default`。

### 影响接口/文件

不影响后端接口，不改业务数据流。

影响文件：

```text
frontend/src/games/stardew/StardewPanel.tsx
frontend/src/games/stardew/StardewPanel.css
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-28.md
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

已验证通过：`tsc -b && vite build` exit 0。

### 下一步注意事项

- 不要再把所有左侧导航按钮统一为 `width: 100%` 的同一个 PNG 盒子；当前有效规则是按素材实际宽高比缩放。
- 如果新增导航背景素材，应同时补充对应的 `--sd-nav-*-w/h` 或 route-specific CSS。

## FE-R4g: 普通导航底图撤销以恢复木纹原色（2026-06-28）

### 目标

用户指出颜色是在“按素材比例重新校准左侧导航、普通按钮也加了默认底图”那一步之后变淡的。这个时间点说明：真正导致发淡的不是 quick help 单张图，也不是浏览器插值，而是普通导航项被铺上了比侧栏木纹更亮的 `--sd-img-nav-default`。

### 改了什么

修改 `frontend/src/games/stardew/StardewPanel.css`：

- 撤销 `.sd-sidebar .sd-nav-item` 的普通态 `background-image: var(--sd-img-nav-default)`。
- 撤销 `.sd-sidebar-help` 的 `background-image: var(--sd-img-nav-default)`。
- 普通导航项和底部帮助恢复透明背景，直接显示侧栏木纹原色。
- 保留当前侧栏宽度、按比例计算的按钮高度、文字/图标对齐和 active 绿色底图。

### 影响接口/文件

不影响后端接口，不改业务数据流。

影响文件：

```text
frontend/src/games/stardew/StardewPanel.css
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-28.md
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

已验证通过：`tsc -b && vite build` exit 0。

### 下一步注意事项

- 当前有效规则：普通导航项透明显示侧栏木纹；只有 active 项使用绿色 PNG 底图。
- 不要再给普通导航项铺 `--sd-img-nav-default`，它比侧栏木纹亮，会造成用户看到的“发淡”。

## FE-R4f: 底部 quick help 偏亮素材替换（已被 FE-R4g 取代，2026-06-28）

### 目标

用户继续反馈底部背景“还是淡淡的，和下面没有文字的背景颜色不一样”。本次曾判断为 quick help 单张素材偏亮，但后续用户指出颜色是在 FE-R4d 添加普通导航默认底图之后才异常，因此本节方案已被 FE-R4g 取代。

### 改了什么

先对素材取样确认：

```text
background_sidebar_wood_tile.png  平均约 rgba(59,42,30)
nav_item_default_blank.png        平均约 rgba(73,47,29)
nav_quick_help_blank.png          平均约 rgba(130,86,44)
```

也就是说，`nav_quick_help_blank.png` 本身就是偏亮的图，不是 CSS 颜色或 `image-rendering` 问题。

修改 `frontend/src/games/stardew/StardewPanel.css`：

- `.sd-sidebar-help` 背景从 `nav_quick_help_blank.png` 改为 `--sd-img-nav-default`。
- `.sd-sidebar-help` 高度比例同步为 `calc(100cqi * 29 / 105)`，与普通导航按钮一致。

### 影响接口/文件

不影响后端接口，不改业务数据流。

影响文件：

```text
frontend/src/games/stardew/StardewPanel.css
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-28.md
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

已验证通过：`tsc -b && vite build` exit 0。

### 下一步注意事项

- 如果后续想保留 quick help 专用边框形状，需要重新导出一张深色版本的 `nav_quick_help_blank.png`，而不是用当前偏亮素材。

## FE-R4e: 左侧像素背景缩放防发淡（2026-06-28）

### 目标

用户反馈“改完之后底部的背景图颜色变淡”。本次目标是解释并修正左侧栏 PNG 素材放大后的发灰/发淡问题。

### 改了什么

修改 `frontend/src/games/stardew/StardewPanel.css`：

- 没有改颜色变量。
- 为 `.sd-sidebar`、`.sd-sidebar-help`、`.sd-sidebar .sd-nav-item` 和 active 导航项增加 `image-rendering: pixelated`。
- 作用是禁止浏览器对像素 PNG 背景做平滑插值，避免木纹、导航底图和底部 quick help 放大后颜色被采样混合，看起来发淡。

### 影响接口/文件

不影响后端接口，不改业务数据流。

影响文件：

```text
frontend/src/games/stardew/StardewPanel.css
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-28.md
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

已验证通过：`tsc -b && vite build` exit 0。

### 下一步注意事项

- 如果仍觉得底部 quick help 比原型浅，下一步应检查该 PNG 本身的颜色/透明度，而不是继续调 CSS 颜色。
- 像素素材只要被非整数倍缩放，都应该保留 `image-rendering: pixelated`。

## FE-R4d: 左侧导航按钮边界按 PNG 比例校准（2026-06-28）

### 目标

用户反馈当前左侧导航“文字和背景的界限还是有很大偏移”。本次目标是在保留当前侧栏宽度的前提下，用素材真实比例校准按钮边界、文字和图标位置。

### 改了什么

修改 `frontend/src/games/stardew/StardewPanel.css`：

- 保留左侧栏宽度 `clamp(148px, 10vw, 176px)`。
- `.sd-sidebar .sd-nav-item` 仍为当前侧栏 `100%` 宽。
- 导航按钮高度从粗略 `clamp(34px, 27cqi, 44px)` 改为按 PNG 比例计算：`calc(100cqi * 29 / 105)`，对应 `nav_item_default_blank.png` 的 `105×29`。
- 快速帮助按钮高度改为 `calc(100cqi * 28 / 96)`，对应 `nav_quick_help_blank.png` 的 `96×28`。
- 普通导航项也使用默认底图 `--sd-img-nav-default`，不再让文字直接漂在木纹背景上。
- 选中态在 `StardewPanel.css` 中显式使用 `--sd-img-nav-active`，避免被默认底图覆盖。
- 垂直 padding 改为 0，由 flex 居中控制文字/图标基线；文字保留单行省略。

### 影响接口/文件

不影响后端接口，不改业务数据流。

影响文件：

```text
frontend/src/games/stardew/StardewPanel.css
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-28.md
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

已验证通过：`tsc -b && vite build` exit 0。

### 下一步注意事项

- 后续如果继续微调左侧导航，优先按素材比例和实际截图微调，不要再用无依据的固定高度。
- 如果视觉上仍有 1-2px 偏移，可以在 `.sd-sidebar .sd-nav-item span` 上做小幅 `transform: translateY(...)`，但先用截图确认偏移方向。

## FE-R4c: 左侧当前宽度木板按钮适配（2026-06-28）

### 目标

用户澄清：需要保留“现在侧栏这个大小”，但文字和按钮要按当前侧栏里一个木板背景的宽度变化。也就是说，不使用已回退的 `112px` 侧栏 / `105px` 原始按钮方案；当前有效方案是在 FE-R4b 的左侧栏宽度 `clamp(148px, 10vw, 176px)` 上，让按钮、文字、图标随当前侧栏宽度自适应。

### 改了什么

修改 `frontend/src/games/stardew/StardewPanel.css`：

- 保留 Shell 左侧列 `clamp(148px, 10vw, 176px)`。
- `.sd-sidebar` 启用 `container-type: inline-size`，让内部按钮可以使用当前侧栏宽度作为适配依据。
- `.sd-sidebar .sd-nav-item` 宽度为 `100%`，与当前被拉伸后的木板背景等宽。
- 导航按钮高度、间距、左右内边距、字号和图标使用 `cqi` + `clamp()` 随当前侧栏宽度变化。
- 快速帮助按钮同样宽度 `100%`，高度和字号跟随侧栏适配。
- 导航文字开启单行省略，避免长标签撑破按钮。

### 影响接口/文件

不影响后端接口，不改业务数据流。

影响文件：

```text
frontend/src/games/stardew/StardewPanel.css
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-28.md
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

已验证通过：`tsc -b && vite build` exit 0。

### 下一步注意事项

- 当前有效方案：侧栏宽度沿用 FE-R4b，按钮/文字/图标按当前侧栏木板宽度自适应。
- 不要再把左栏改回 `112px` / 按钮 `105px`，那是已回退的误解版本。

## FE-R4b: Shell 三栏比例均衡修复（2026-06-28）

### 目标

根据截图反馈修正 Stardew 面板比例：当前实现中主内容区过宽，左侧导航和右侧 OpsRail 显得过窄，尤其在 1700px 以上宽屏下偏离原型的平衡感。

### 改了什么

修改 `frontend/src/games/stardew/StardewPanel.css`：

- Shell 三栏从固定 `112px/1fr/212px` 改为响应式比例：左侧 `clamp(148px, 10vw, 176px)`，中间 `minmax(0, 1fr)`，右侧 `clamp(280px, 20vw, 360px)`。
- 左侧导航项略放大图标、字号、行高和内边距，让木纹导航不再只是窄条。
- 右侧 OpsRail 增加内边距和文本尺寸，增强右栏存在感。
- Overview 内部右侧事件/资源栏从固定 `212px` 改为 `clamp(260px, 28%, 340px)`，减少主内容区在宽屏下吞掉全部空间的问题。
- 960px 以下仍隐藏右侧 OpsRail，左侧收为 132px，保证较小屏幕不被三栏挤爆。

### 影响接口/文件

不影响后端接口，不改业务数据流。

影响文件：

```text
frontend/src/games/stardew/StardewPanel.css
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-28.md
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

已验证通过：`tsc -b && vite build` exit 0。

### 下一步注意事项

- 这次只修布局比例，没有补齐 install/server/saves 等页面的内容密度；如果某些路由中间仍显空，下一步应按原型把对应业务模块迁移进去，而不是继续扩大侧栏。
- 后续做视觉验收时优先对照 `docs/prototypes/stardew-anxi-panel-ui-refactor-prototype.html` 的三栏重量：左侧导航应有明确存在感，右侧信息栏不应小到只能放摘要，中间主区不应无边界拉满。

## Frontend UI Refactor Implementation Plan（2026-06-28）

### 目标

用户已经完成可交互 HTML 产品原型：

```text
docs/prototypes/stardew-anxi-panel-ui-refactor-prototype.html
```

该原型把多个业务页面压缩展示在四个画布区块中。本次目标不是改前端代码，而是把“如何让 Codex 按这个原型重构真实 React 前端”拆成真实业务路由和细化小目标，并形成接手文档，避免后续一次性大重写导致现有功能回退。

### 改了什么

**新增文档：**

| 文件 | 说明 |
|------|------|
| `docs/frontend-ui-refactor-implementation-plan.md` | 前端 UI 重构实施计划，把原型拆解为可执行小目标和验收标准 |

**更新文档：**

| 文件 | 说明 |
|------|------|
| `docs/handoff-roadmap.md` | 在 Current Context 中新增 2026-06-28 的前端重构计划记录 |

### 文档内容摘要

`docs/frontend-ui-refactor-implementation-plan.md` 覆盖：

- 输入材料：原型 HTML、`ui-extracted` 素材、当前 `App.tsx`、`api.ts`、`types.ts`、`games/stardew` 和 `core` 组件。
- 不变原则：Single Game Mode、优先复用现有 API、Stardew 专属逻辑留在 `frontend/src/games/stardew`。
- 轻量内部路由方案：当前未引入 `react-router-dom`，第一轮建议用 `StardewRoute` union + History API。
- 真实路由识别：`install`、`overview`、`server`、`saves`、`jobs`、`players`、`mods`、`diagnostics`、`settings`。
- API 映射：每个路由对应当前 `frontend/src/api.ts` 中已存在的函数。
- 未来占位：原型中有但当前后端没有的能力保留 UI、禁用按钮或“待接入”空状态。
- 玩家管理边界：`players` 是未来游戏玩家管理；当前没有完整玩家 API，不造假数据。面板用户、审计和版本信息放在 `settings`。
- 小目标：FE-R0 到 FE-R13。
- 推荐实施节奏：第一轮只做素材、Shell、路由、Overview 和 Players 占位；第二轮做 Server/Jobs/Diagnostics；第三轮做 Install/Saves/Mods；第四轮做 Settings/响应式/交接。
- 给 Codex 的第一轮实施提示词。
- 风险清单：误把四个展示块当四个页面、玩家 API 缺失、素材路径、App.tsx 膨胀、危险操作确认、移动端表格溢出等。

### 影响接口/文件

本次不修改前后端运行代码，不影响 API。

影响文件：

```text
docs/frontend-ui-refactor-implementation-plan.md
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-28.md
```

### 如何验证

本次是文档工作，验证方式为：

```powershell
cd E:\stardew-server-anxi-panel
Get-Content .\docs\frontend-ui-refactor-implementation-plan.md -Encoding UTF8
Get-Content .\docs\handoff-roadmap.md -Encoding UTF8
```

并已在编写前读取：

```text
docs/architecture.md
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-27.md
frontend/src/App.tsx
frontend/src/api.ts
frontend/src/games/stardew/*
docs/prototypes/stardew-anxi-panel-ui-refactor-prototype.html
```

未运行 `npm.cmd run build`，因为本次没有修改前端运行代码。

### 下一步注意事项

- 建议下一次实施只做 `FE-R1 + FE-R2 + FE-R3 + FE-R8`：素材接入、StardewShell/内部路由、Overview、Players 占位。
- 第一轮要让九个路由都能导航切换，但只要求 Overview 接真实后端；其他复杂路由可以先占位或继续挂现有组件。
- 当前 `frontend/package.json` 没有 `react-router-dom`，第一轮按文档建议使用内部 route + History API。
- `players` 当前没有完整后端 API，不要用假玩家数据冒充真实数据；只显示可用摘要和”待接入”状态。
- 面板用户、审计日志、版本信息应放在 `settings`，不要和 Stardew 游戏玩家管理混淆。
- 现有工作区已有一些未提交改动和删除文件，本次没有回滚它们；后续实施前应再次查看 `git status --short`。

---

## FE-R1: 前端资产与主题基础（2026-06-28）

### 目标

完成 `docs/frontend-ui-refactor-implementation-plan.md` 中 FE-R1 的全部任务：
让真实前端可以稳定使用原型拆出的 UI 素材，并建立统一 Stardew 视觉主题基础。

### 改了什么

**新增文件：**

| 文件 | 说明 |
|------|------|
| `frontend/public/assets/stardew/ui/backgrounds/` | 4 个背景 PNG（羊皮纸平铺、木纹侧栏、木纹横条、黑底） |
| `frontend/public/assets/stardew/ui/buttons/` | 12 个按钮底图 PNG（启动/停止/重启/复制/快捷工具等） |
| `frontend/public/assets/stardew/ui/fields/` | 4 个表单控件 PNG（输入框、搜索框、下拉框、邀请码框） |
| `frontend/public/assets/stardew/ui/icons/` | 16 个图标 PNG（导航、状态、按钮图标，已透明抠底） |
| `frontend/public/assets/stardew/ui/navigation/` | 7 个导航 PNG（默认/选中导航项、顶部和内容页标签） |
| `frontend/public/assets/stardew/ui/panels/` | 6 个面板底 PNG（表单区、表格区、指标卡、告警条等） |
| `frontend/public/assets/stardew/ui/sprites/` | 8 个贴图 PNG（农舍、云、宝箱、树、栅栏、设备等） |
| `frontend/src/games/stardew/stardew-theme.css` | Stardew 视觉主题变量和基础工具类 |

**修改文件：**

| 文件 | 修改 |
|------|------|
| `frontend/src/main.tsx` | 追加 `import './games/stardew/stardew-theme.css'` |

**未改动：** 任何业务组件（App.tsx、App.css、InstallSection、LifecycleSection、SavesSection、ModsSection、JobsSection 等）和后端 API。

### 素材来源与目录对应

```text
来源：docs/prototypes/assets/ui-extracted/
目标：frontend/public/assets/stardew/ui/

已复制分类（57 个文件）：
  backgrounds/  4 个  →  背景平铺图
  buttons/     12 个  →  像素风按钮底图
  fields/       4 个  →  表单控件底图
  icons/       16 个  →  导航/状态/按钮图标
  navigation/   7 个  →  导航项和标签
  panels/       6 个  →  内容区面板底
  sprites/      8 个  →  装饰贴图

未复制：
  layout/    (整窗壳快速原型用，真实实现拆成 backgrounds+panels+buttons)
  source/    (原始参考图备份，不进生产)
  manifest.json / preview.html / contact-sheet.png  (原型文档)
```

### stardew-theme.css 内容

CSS 变量（`--sd-*` 前缀，避免与 App.css 冲突）：
- `--sd-brown-*` / `--sd-green-*` / `--sd-red-*` / `--sd-gold-*` / `--sd-blue-*`：颜色系
- `--sd-img-bg-*`：背景图片路径
- `--sd-img-nav-*`：导航图片路径
- `--sd-img-btn-*`：按钮底图路径
- `--sd-img-panel-*`：面板底图路径
- `--sd-img-field-*`：表单控件底图路径

基础工具类：
- `.sd-bg-wood-side` / `.sd-bg-wood-strip` / `.sd-bg-black` / `.sd-bg-parchment`：背景类
- `.sd-panel` / `.sd-form-panel` / `.sd-metric-card`：面板类
- `.sd-btn-green` / `.sd-btn-red` / `.sd-btn-gold` / `.sd-btn-tan` / `.sd-btn-icon`：按钮变体
- `.sd-input` / `.sd-input-invite`：输入框类
- `.sd-nav-item` / `.sd-nav-icon`：左侧导航项
- `.sd-dot` + `.sd-dot-green/red/yellow/blue/gray` + `.sd-dot-pulse`：状态点
- `.sd-data-list` / `.sd-data-row` / `.sd-data-row-head`：紧凑数据行
- `.sd-tag` + `.sd-tag-green/red/blue/gold`：徽签
- `.sd-metrics-row` / `.sd-metric` / `.sd-metric-label` / `.sd-metric-value`：指标卡

### 如何引用新素材

生产 build 后，素材通过绝对路径访问：

```
/assets/stardew/ui/icons/icon_nav_overview_home.png
/assets/stardew/ui/backgrounds/background_sidebar_wood_tile.png
/assets/stardew/ui/buttons/button_server_start_green_blank.png
```

CSS 中可直接引用 CSS 变量，例如：

```css
.my-sidebar {
  background-image: var(--sd-img-bg-wood-side);
}
```

React 组件中图片 src：

```tsx
<img src=”/assets/stardew/ui/icons/icon_nav_overview_home.png” className=”sd-nav-icon” alt=”” />
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，dist/ 包含 assets/stardew/ui/ 目录和全部素材
```

已验证：`npm.cmd run build` 通过（exit 0），CSS bundle 36.48 kB（含新主题）。

### 下一步注意事项

- ~~**下一步应执行 FE-R2：StardewShell 与真实左侧导航。**~~ **FE-R2 已完成（见下方）。**
- 如果原型素材更新，重新运行 `scripts/extract-ui-assets.py` 后执行同样的 PowerShell 复制命令即可。

---

## FE-R2: StardewShell 与路由骨架（2026-06-28）

### 目标

建立 StardewShell 和真实路由骨架：登录后进入基于 `window.history.pushState + popstate` 的 9 路由 SPA，左侧木纹导航栏、顶部状态栏、羊皮纸主区域、右侧 OpsRail 任务轨道。

### 改了什么

**新增文件：**

| 文件 | 说明 |
|------|------|
| `frontend/src/games/stardew/stardew-routes.ts` | `StardewRoute` union、URL 解析/生成、`StardewPageProps` 类型 |
| `frontend/src/games/stardew/StardewPanel.css` | Shell 网格布局（topbar / sidebar / main / opsrail）及页面通用组件样式 |
| `frontend/src/games/stardew/StardewPanel.tsx` | Shell 主组件：9 路由导航、30s 状态轮询、OpsRail 任务列表、顶部状态栏 |
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | 总览页 — 实时展示 instanceState（state / phase / message / updatedAt） |
| `frontend/src/games/stardew/pages/InstallPage.tsx` | 安装页占位 |
| `frontend/src/games/stardew/pages/ServerControlPage.tsx` | 服务器控制页占位（含状态读取） |
| `frontend/src/games/stardew/pages/SavesPage.tsx` | 存档页占位 |
| `frontend/src/games/stardew/pages/JobsLogsPage.tsx` | 任务日志页占位 |
| `frontend/src/games/stardew/pages/PlayersPage.tsx` | 玩家页占位（无假数据，明确说明后端待接入） |
| `frontend/src/games/stardew/pages/ModsPage.tsx` | 模组页占位 |
| `frontend/src/games/stardew/pages/DiagnosticsPage.tsx` | 诊断页占位 |
| `frontend/src/games/stardew/pages/SettingsPage.tsx` | 设置页占位（展示当前登录用户信息） |

**重写文件：**

| 文件 | 修改 |
|------|------|
| `frontend/src/App.tsx` | 移除 Dashboard 函数及所有 Dashboard 专用 state/函数/import；`View` 类型新增 `'stardew'`；登录后渲染 `<StardewPanel>` 全屏（在 panel-card 之外） |

**未改动：** backend API、`App.css`、任何现有 Stardew game 组件文件（这些后续由 FE-R3 到 FE-R9 逐步迁移）。

### 架构要点

- 路由：`StardewRoute` union（9 个字符串字面量）+ `parseRoute()` + `routeToPath()`；History API `pushState` 导航，`popstate` 事件同步浏览器前进/后退。
- URL 模式：`/instances/stardew/{route}`，默认 `overview`。
- Shell 布局：`display: grid; grid-template-rows: 52px 1fr; grid-template-columns: 220px 1fr 260px`。
- 数据：`getStardewState()` 每 30s 轮询；`getJobs()` 初始加载填充 OpsRail（不轮询，等 FE-R5 迁移时决定是否加）。
- 玩家页：仅占位，明确标注"后端待接入"，无假数据。
- `stardew-theme.css` 已通过 `main.tsx` 全局引入；`StardewPanel.css` 单独引入，覆盖 Shell 专属布局。

### 影响接口/文件

| 影响点 | 说明 |
|--------|------|
| `App.tsx` | 大幅精简（从 1071 行降至 ~130 行），移除全部 Dashboard 业务逻辑 |
| 新增 10 个 TSX 文件 | 新路由框架和页面占位 |
| `GET /api/instances/stardew/state` | 由 StardewPanel 30s 轮询（之前由 Dashboard 轮询） |
| `GET /api/jobs` | 由 StardewPanel 初始加载（之前由 Dashboard 轮询） |
| `GET /api/version` | 仍由 App.tsx 加载（用于 boot/login 页顶部版本号） |

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，无 TS 错误

# 启动开发服务器验证 9 路由可切换：
npm.cmd run dev
# 浏览器访问 http://localhost:5173，登录后应看到 StardewPanel 木纹侧边栏布局
# 点击 9 个导航项，URL 应切换至 /instances/stardew/{route}
# 浏览器后退/前进应同步路由
```

已验证：`npm.cmd run build` 通过（exit 0），35 个模块，CSS bundle 36.52 kB。

### 下一步注意事项

- **FE-R3（服务器控制页）**：迁移 `LifecycleSection`（启动/停止/重启/邀请码）到 `ServerControlPage.tsx`，以及 `ConsoleSection` 日志流。对应原来 Dashboard 里的 `onStart`, `onStop`, `onRestart`, `onHealthCheck` 等逻辑。
- **FE-R4（任务日志页）**：迁移 `JobsSection`（任务列表、SSE 日志流、clearJobs）到 `JobsLogsPage.tsx`。
- **FE-R5（安装页）**：迁移 `InstallSection`（Steam 账号、SMAPI 版本、安装进度流）到 `InstallPage.tsx`。
- **FE-R6（存档页）**：迁移 `SavesSection` 到 `SavesPage.tsx`。
- **FE-R7（模组页）**：迁移 `ModsSection` 到 `ModsPage.tsx`。
- **FE-R8（诊断页）**：迁移 `DockerSection` + 健康检查 + 支持包导出到 `DiagnosticsPage.tsx`。
- **FE-R9（设置页）**：迁移用户管理（createUser/updateRole/setUserActive/deleteUser）、审计日志、版本信息到 `SettingsPage.tsx`；同时把 `loadUsers` 相关逻辑移回 `StardewPanel` 或 `SettingsPage`。
- `App.tsx` 目前完全干净，不含任何旧 Dashboard 状态；迁移时不要把复杂逻辑放回 App.tsx，只能放在对应页面组件内。
- `PlayersPage` 当前后端没有游戏玩家 API，接入前不要造假数据。
- 旧 Dashboard 函数已被删除；如需参考旧实现，查阅 git log（commit `bc9531e` 之前的版本包含完整 Dashboard）。

## FE-R3: 公共数据层（2026-06-28）

### 目标

在 StardewPanel 层建立公共数据层，统一加载 7 个常用 API，避免各页面重复请求，并把数据通过 `StardewPageProps.dashboardData` 传给所有路由页面。

### 改了什么

**新增文件：**

| 文件 | 说明 |
|------|------|
| `frontend/src/games/stardew/useStardewDashboardData.ts` | 公共数据 hook，集中加载全部共享 API |

**修改文件：**

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/stardew-routes.ts` | 新增 `StardewDashboardData` 类型；`StardewPageProps` 新增 `dashboardData` 字段 |
| `frontend/src/games/stardew/StardewPanel.tsx` | 改为调用 `useStardewDashboardData`；移除旧的内联 fetchState/fetchJobs；TopStatusBar 新增版本号 + 当前存档；OpsRail 新增健康摘要 + Mod 重启提示；pageProps 增加 dashboardData |
| `frontend/src/games/stardew/StardewPanel.css` | 新增 `.sd-topbar-version`、`.sd-topbar-save`、`.sd-topbar-meta-icon` |
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | 接收 `dashboardData`，展示存档名/数量、健康状态、Mod 数量/重启提示、近期任务摘要、邀请码 |

**未改动：** 其余 8 个页面占位（仍接收 `dashboardData` 但不使用）、App.tsx、所有后端 API。

### hook 设计

```text
useStardewDashboardData()
  - 集中调用：getStardewState / getSaves / getMods / getJobs / getHealthDiagnostics / getInviteCode / getVersion
  - 初始化时 Promise.allSettled 并发加载，单个失败不阻塞其他
  - instanceState 每 30s 自动轮询（其余不轮询，由页面操作后手动刷新）
  - 返回独立 error 字段（savesError / modsError / healthError / inviteCodeError），不抛异常
  - 暴露 refreshAll / refreshInstanceState / refreshSaves / refreshMods / refreshJobs / refreshHealth / refreshInviteCode
  - version 只初始化加载一次，不暴露 refreshVersion
```

### 错误降级策略

| API | 失败处理 |
|-----|---------|
| instanceState | 保留上次已知值，TopStatusBar 保持上次状态 |
| saves | `savesError` 非空，页面显示错误文字，不崩溃 |
| mods | `modsError` 非空，页面显示错误文字 |
| jobs | 保留上次已知列表，OpsRail 不变 |
| health | `healthError` 非空，OpsRail 显示"健康检查失败" |
| inviteCode | `inviteCodeError` 非空，Overview 不显示邀请码行 |
| version | 静默失败，TopStatusBar 不显示版本号 |

### 暴露的刷新函数（供后续页面操作后调用）

- `refreshSaves()` → FE-R6 SavesPage 上传/删除后调用
- `refreshMods()` → FE-R9 ModsPage 上传/删除后调用
- `refreshJobs()` → FE-R7 JobsLogsPage 清理后调用；job SSE 完成后 `refreshAll`
- `refreshHealth()` → FE-R10 DiagnosticsPage 手动触发时调用
- `refreshInviteCode()` → FE-R4 ServerControlPage 服务器启动后调用
- `refreshInstanceState()` → FE-R4 ServerControlPage 启动/停止后调用

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，36 个模块，无 TS 错误
```

已验证：`npm.cmd run build` 通过（exit 0），36 模块，JS bundle 224.44 kB。

### 下一步注意事项

- **FE-R4（服务器控制页）**：`ServerControlPage.tsx` 接收 `dashboardData`，解构 `dashboardData.refreshInstanceState` 和 `dashboardData.refreshInviteCode`，在启动/停止/重启完成后调用。迁移 `startInstance / stopInstance / restartInstance / getInviteCode`，移植原 `LifecycleSection` 逻辑。
- **FE-R7（任务日志页）**：`JobsLogsPage.tsx` 接收 `dashboardData.jobs`（初始列表）和 `dashboardData.refreshJobs`，在 SSE job-finished 事件后调用 `refreshAll()`。迁移 `JobsSection` 的 SSE 流和 clearJobs 逻辑。
- **FE-R9（模组页）**：`ModsPage.tsx` 接收 `dashboardData.mods` 和 `dashboardData.refreshMods`，上传/删除 Mod 后调用刷新。
- **FE-R6（存档页）**：`SavesPage.tsx` 接收 `dashboardData.saves` 和 `dashboardData.refreshSaves`，操作后调用刷新。
- `StardewPageProps.instanceState`（顶层字段）和 `dashboardData.instanceState` 指向同一个值，保留两者是为了向后兼容；后续可考虑只用 `dashboardData.instanceState`。
- `inviteCode` 在服务器未运行时（如 stopped 状态）后端可能返回错误，这是预期行为，`inviteCodeError` 会被设置，Overview 不显示邀请码行。

## FE-R4: Overview 真实 UI（2026-06-28）

### 目标

把 `/instances/stardew/overview` 从占位卡片升级为真实可用的日常概览页。展示核心服务器状态、按状态机驱动的 CTA 操作、邀请码（含复制）、各模块摘要、以及玩家/资源占位。

### 改了什么

**修改文件：**

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | 完整重写为真实 UI（原占位版 ~207 行 → 新版 ~330 行） |
| `frontend/src/games/stardew/StardewPanel.css` | 新增 `.sd-overview-cta`、`.sd-overview-cta-hint`、`.sd-overview-cta-buttons`、`.sd-confirm-overlay`、`.sd-confirm-dialog`、`.sd-confirm-actions` |

**未改动：** `useStardewDashboardData.ts`、`stardew-routes.ts`、`StardewPanel.tsx`、其余页面占位、所有后端 API。

### UI 结构

```
sd-page
├── sd-page-header（图标 + 标题）
├── sd-state-card（实例名、状态点+中文标签、驱动阶段、当前存档、更新时间）
├── renderCTA()（根据状态机选 CTA）
│   ├── game_installed → 前往安装配置（导航）
│   ├── save_required  → 前往存档管理（导航）
│   ├── ready/stopped  → 启动服务器（startInstance + refresh）
│   ├── starting       → 禁用按钮，显示动态点
│   ├── running        → 停止/重启按钮（两步确认）
│   └── error          → 查看诊断（导航）
├── 操作错误提示（actionError，独立于 dashboardData 错误）
├── 邀请码卡片（含复制按钮，inviteCodeError 时显示友好提示）
├── sd-placeholder-grid × 4
│   ├── 存档摘要（activeSave + saveCount + 管理按钮）
│   ├── 健康摘要（ok/warn/error 数量 + 诊断按钮）
│   ├── 模组摘要（modCount + restartRequired 徽章 + 管理按钮）
│   └── 近期任务（最多 5 条，状态点 + 类型 + 时间/错误文字）
├── sd-placeholder-grid × 2（玩家占位 + 资源趋势占位）
└── sd-confirm-overlay（stop/restart 危险操作弹框，portal 遮罩）
```

### 状态机 CTA 逻辑

| 状态 | CTA |
|------|-----|
| `game_installed` | `onNavigate('install')` |
| `save_required` | `onNavigate('saves')` |
| `ready_to_start` / `stopped` | `startInstance()` → `refreshInstanceState()` + `refreshJobs()` |
| `starting` | 禁用按钮 |
| `running` | `setConfirmAction('stop')` / `setConfirmAction('restart')` → 确认弹框 → 执行 |
| `error` | `onNavigate('diagnostics')` |

### 本地状态

```typescript
actionBusy: boolean       // API 请求进行中，禁用所有 CTA 按钮
actionError: string|null  // API 失败时显示的用户可见错误
confirmAction: 'stop'|'restart'|null  // 弹框确认
copied: boolean           // 邀请码复制反馈（2s 后自动重置）
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，36 个模块，无 TS 错误
```

已验证：`npm.cmd run build` 通过（exit 0），36 模块，JS bundle 231.14 kB。

### 下一步注意事项

- **FE-R5（ServerControlPage）**：将 `startInstance / stopInstance / restartInstance / getInviteCode` 迁移到 `ServerControlPage.tsx`，提供完整的服务器控制界面（Overview CTA 只是快捷入口）。
- **FE-R6（SavesPage）**：接收 `dashboardData.saves` + `dashboardData.refreshSaves`，实现存档列表、新建、激活、删除功能。
- **FE-R7（JobsLogsPage）**：接收 `dashboardData.jobs` + `dashboardData.refreshJobs`，展示完整任务日志 + SSE 实时日志流。
- **FE-R8（ModsPage）**：接收 `dashboardData.mods` + `dashboardData.refreshMods`，实现模组列表、上传、删除。
- **FE-R9（DiagnosticsPage）**：展示健康检查详情、Docker 状态、支持包导出。
- Overview 的邀请码复制使用 `navigator.clipboard.writeText`（需 HTTPS 或 localhost），部署到 HTTP 环境时该功能不可用，需后续考虑降级（如 `document.execCommand`）。
- `sd-confirm-overlay` 使用 `position: fixed` + `z-index: 200`，正常情况下覆盖整个 viewport；如果将来 StardewPanel 设置了 `overflow: hidden` 或 `transform` 属性，需注意 containing block 变化导致的定位问题。

## FE-R4 修订：P1/P2 像素 UI 回归（2026-06-28）

### 问题

代码审查发现 4 个 P1/P2 视觉回归：Shell 尺寸过大、Overview 为通用卡片堆叠、按钮素材未使用、大量现代圆角。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/stardew-theme.css` | 按钮全部改用 PNG 底图（`button_*_blank.png`）；导航 active 改用 `nav_item_active_green_blank.png`；border-radius 缩至 0–2px；dot 从 10px 缩至 7px；新增 `.sd-btn-start/.stop/.restart/.copy/.delete` 生命周期按钮；新增 `.sd-btn-img` 内联图标类 |
| `frontend/src/games/stardew/StardewPanel.css` | Shell: `220px/260px/52px` → `112px/212px/40px`；`sd-main` padding 改 0 + flex column（页面自管理内边距）；所有卡片/面板圆角降至 0–2px；字体/间距统一到 10.5px 基线；新增全套 `sd-ov-*` 类（ov-wrap/banner/section/body/left/right、bstat、state-badge、metric-grid、mc、ev-list、pack-section 等） |
| `frontend/src/games/stardew/StardewPanel.tsx` | 新增 `stateLabel` import；Topbar 改显示中文状态标签；移除 TopBar driverPhase 展示 |
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | 完整重构为原型水平密集布局：农场横幅（58px）→ 服务器控制横排（生命周期按钮 + 邀请码）→ 双栏主体（左：2×2 指标网格 + 玩家占位；右：事件列表 + 模组摘要） |

### 关键设计决策

- `sd-btn-green`/`sd-btn-tan`/`sd-btn-gold` → 小型 PNG 按钮（24–26px 高），原有 class 名不变，只换实现
- `sd-btn-red` → 映射至 stop 按钮 PNG（语义一致）
- `sd-btn-start`/`stop`/`restart` → 新增大型生命周期 PNG 按钮（固定 76×34 / 88×33 / 84×34）
- `sd-page` 的 `max-width: 900px` 已移除；页面宽度由内容和 Shell 列决定
- Overview 不再使用 `sd-page`，直接用 `sd-ov-wrap` 充满 `sd-main`
- `.sd-main { padding: 0; display: flex; flex-direction: column }` — 其余页面继续用 `sd-page`（自带 12px padding）

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，36 个模块，无 TS 错误
```

已验证：`npm.cmd run build` 通过（exit 0），36 模块，JS bundle 229.21 kB。

### 下一步注意事项

- **FE-R5（ServerControlPage）**：现在有 `sd-btn-start`/`sd-btn-stop`/`sd-btn-restart` 可直接使用。
- **各业务页面**：使用 `.sd-page` 作为外层（带 12px 内边距）；不要用 `sd-ov-wrap`（那是 Overview 专用全高布局）。
- **按钮类对照**：小操作用 `sd-btn-green`（主）/`sd-btn-tan`（次）/`sd-btn-delete`（删除）/`sd-btn-copy`（复制），大生命周期用 `sd-btn-start`/`stop`/`restart`。
- **已移除**的旧 CSS：`sd-overview-cta`、`sd-overview-cta-hint`、`sd-topbar-state-phase`（这些不再存在，不要引用）。
