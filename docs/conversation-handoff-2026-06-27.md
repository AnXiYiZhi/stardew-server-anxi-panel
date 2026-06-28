# Conversation Handoff 2026-06-27

## Pixel UI Asset Extraction（2026-06-27）

### 目标

把用户提供的 Stardew Valley 管理面板合成设计图，按区域、按钮、背景、控件、图标和贴图拆成无文字、可复用、适合后续 HTML 原型直接引用的最小化 PNG 素材。

### 改了什么

**新增脚本：**

| 文件 | 说明 |
|------|------|
| `scripts/extract-ui-assets.py` | 基于固定坐标的素材裁切脚本；会清理按钮/输入/面板文字，生成透明图标和总览文件 |

**新增素材目录：**

| 路径 | 说明 |
|------|------|
| `docs/prototypes/assets/ui-extracted/backgrounds/` | 羊皮纸、木纹、黑色舞台等背景 |
| `docs/prototypes/assets/ui-extracted/layout/` | 安装、总览、存档模组、健康诊断四个无字整窗壳 |
| `docs/prototypes/assets/ui-extracted/panels/` | 表单、表格、指标卡、告警条、模组卡片等无字面板 |
| `docs/prototypes/assets/ui-extracted/navigation/` | 左导航项、顶部编号标签、内容页标签 |
| `docs/prototypes/assets/ui-extracted/buttons/` | 启动、停止、重启、下一步、复制、快捷工具等无字按钮 |
| `docs/prototypes/assets/ui-extracted/fields/` | 邀请码输入、搜索、路径输入、下拉框等无字控件 |
| `docs/prototypes/assets/ui-extracted/icons/` | 导航、状态摘要、按钮图标，已尽量透明抠底 |
| `docs/prototypes/assets/ui-extracted/sprites/` | 农舍、云、宝箱、树、栅栏、蓝色设备、宝石等贴图 |
| `docs/prototypes/assets/ui-extracted/manifest.json` | 每个素材的分类、文件名、源图坐标和说明 |
| `docs/prototypes/assets/ui-extracted/preview.html` | 浏览器预览页 |
| `docs/prototypes/assets/ui-extracted/contact-sheet.png` | 素材总览图 |
| `docs/prototypes/assets/ui-extracted/README.md` | 使用说明 |

当前共导出 61 个素材。按钮、输入框、面板和整窗壳已去除原图文字；文字/图标建议在 HTML 中重新叠加。

### 影响接口/文件

本次不修改前后端运行代码，不影响 API。只新增原型资产和导出脚本，并更新 `docs/handoff-roadmap.md`。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel
& 'C:\Users\anxi\.cache\codex-runtimes\codex-primary-runtime\dependencies\python\python.exe' .\scripts\extract-ui-assets.py
```

验证结果：

- 脚本输出 `Wrote 61 assets to E:\stardew-server-anxi-panel\docs\prototypes\assets\ui-extracted`
- 已查看 `docs/prototypes/assets/ui-extracted/contact-sheet.png`
- 已查看 `docs/prototypes/assets/ui-extracted/layout/layout_overview_window_shell_blank.png`
- 大窗口文字区、按钮文字和输入框文字已清理；边框、木纹、羊皮纸质感和主要贴图保留。

### 下一步注意事项

- 这些资产来自合成 PNG 原型，不是 Stardew 原版 UI 资源；后续商用或公开发布前需确认素材授权边界。
- `layout/*_shell_blank.png` 适合快速 HTML 背景；真实前端实现建议更多使用 `backgrounds/`、`panels/`、`buttons/` 组合。
- 如果设计图更新，优先修改 `scripts/extract-ui-assets.py` 的坐标和清理策略，再重新生成，保持文件名稳定。
- 部分小图标从深色木纹或羊皮纸上透明抠底，像素级边缘可在最终 HTML 使用场景里再按实际背景微调。

## Frontend UI / Interaction Refactor Spec（2026-06-27）

### 目标

项目 MVP 已完成后，针对下一阶段“前端 UI、审美、交互逻辑重构”输出可落地的产品设计交付。重点不是立刻改代码，而是从用户使用逻辑重新发现问题并给后续实现者明确方向。

### 改了什么

**新增文档：**

| 文件 | 说明 |
|------|------|
| `docs/frontend-ui-interaction-refactor.md` | 前端 UI/交互重构文档，覆盖用户路径诊断、目标信息架构、视觉系统、组件映射、实施分期、验收清单 |
| `docs/prototypes/stardew-anxi-panel-ui-refactor-prototype.html` | V2 静态 HTML 产品原型，展示 Overview、Install Wizard、Saves/Mods、Troubleshoot/Security |
| `docs/prototypes/stardew-anxi-panel-ui-refactor-prototype.png` | 由 HTML 原型渲染出的 PNG 原型图，适合快速预览 |
| `docs/prototypes/stardew-anxi-panel-ui-refactor-notes.md` | 原型说明，记录设计目标、视觉方向、实现映射和限制 |

**Figma：**

- 新建 Figma 设计草稿：`https://www.figma.com/design/GHadKWWdw2jWxgPXgY7fdM`
- 已通过 Figma MCP `use_figma` 写入一个可编辑的简化原型画板，内容与 HTML 原型一致：左导航、顶部状态栏、主操作面、右侧任务/健康 rail、安装向导、维护与排障模块。

### 关键产品判断

当前前端功能完整，但页面仍偏“功能区块堆叠”。下一阶段建议从日常运维路径重构为：

```text
AppShell
  LeftNav
  TopStatusBar
  MainRoute
  OpsRail
```

核心交互规则：

- 首屏必须回答：服务器能不能跑、当前 active save 是什么、下一步主要动作是什么、最近任务是否失败、健康状态是否异常。
- `Start server` 与 `Active save / save_required` 必须在同一视觉上下文中，避免用户先点错再被滚动到存档区。
- `InstallSection` 应重构为向导：Prepare、Pull、Steam、Ready。raw phase 和 job logs 放到技术详情或右侧任务 rail。
- 现有高级设置应拆分：Troubleshoot（健康检查、Docker/Compose、support bundle、失败任务）和 Security（用户、审计日志）。
- 视觉上保留农场/木框/羊皮纸氛围，但减少大标题、厚边框和大阴影，让日常运维更紧凑。

### 影响接口/文件

本次只新增文档和原型，不改前后端运行代码，不改变 API 行为。

后续真实实现可优先复用现有接口：

```text
GET /api/instances/:id/state
POST /api/instances/:id/start
POST /api/instances/:id/stop
POST /api/instances/:id/restart
GET /api/instances/:id/saves
GET /api/instances/:id/mods
GET /api/jobs
GET /api/health/diagnostics
POST /api/instances/:id/support-bundle
GET /api/audit-logs
```

建议后续新增但非本次必需：

- dashboard summary 聚合接口：state、active save、latest job、health summary、restart-required flags 一次返回。
- audit log filters。
- saves summary 中补充 backup metadata。

### 如何验证

本次验证：

```powershell
# HTML 原型渲染 PNG
# 使用 Playwright + 本机 Chrome 打开 file:///E:/stardew-server-anxi-panel/docs/prototypes/stardew-anxi-panel-ui-refactor-prototype.html
# 输出 docs/prototypes/stardew-anxi-panel-ui-refactor-prototype.png
```

已用 `view_image` 检查 PNG 原型图，无明显文字截断、重叠或主体布局错位。Figma 画板已通过 Figma screenshot 接口成功渲染。

### 下一步注意事项

- 进入真实 UI 实现前，先做 Phase UI-1：`AppShell`、左导航、顶部状态栏、Overview、OpsRail，不要一口气重写所有功能组件。
- 重构时不要破坏 Single Game Mode：首版仍然登录后直达 Stardew 面板，不显示多游戏总面板。
- 所有 Stardew 专属交互继续留在 `frontend/src/games/stardew`，不要把未来 Minecraft/DST/Terraria/Palworld 逻辑塞进 Stardew。
- 删除、停止、重启等危险操作建议从 `window.confirm` 迁移到统一 `ConfirmDialog`，但这属于后续代码实现。
- Figma 画板是可编辑蓝图，不是像素级最终视觉稿；HTML/PNG 原型记录了更完整的信息层级。

## 修复：Docker 容器停止后前端仍显示"运行中"

### 问题

Docker 容器被手动关闭（`docker compose down`、容器崩溃、Docker 重启等）后，前端仍显示实例状态为 `running`。原因是 `ReconcileState()` 只检查安装文件是否存在，不检查 Docker 容器是否实际在运行。

### 根因

`driver.go` 中的 `ReconcileState()` 在每次前端查询实例状态时被调用，但：

1. `requiresInstalledFiles()` 始终返回 `false`（注释说明 Junimo 用 Docker named volume 存游戏文件，本地文件检查会误判）
2. 即使 state 为 `running`，也不会通过 `ComposePs` 验证容器是否真的在运行
3. DB 中的 `running` 状态是启动时写入的，容器停止后不会自动更新

### 修改文件

| 文件 | 改动 |
|------|------|
| `backend/internal/games/stardew_junimo/driver.go` | `ReconcileState()` 新增 Docker 容器状态检查；新增 `isRunningState()` 辅助函数 |

### 改动详情

**`ReconcileState()`**：当实例状态为 `running` 或 `starting` 时，通过 `ComposePs` 检查 `server` 容器是否实际运行中：

- 容器已停止 → 自动修正为 `stopped`，保留 `DriverPayload`（如邀请码）
- 容器仍在运行 → 不做任何改动
- `ComposePs` 调用失败 → 不修改状态（可能是 Docker 临时故障）

**`isRunningState()`**：判断实例状态是否表示容器应该在运行（`running` 或 `starting`）。

### 影响接口

- `GET /api/instances/:id/state` — 返回状态现在会实时校验容器
- `GET /api/instances/:id` — 同上

### 如何验证

1. 启动服务器，确认前端显示 `running`
2. 在终端执行 `docker compose down`（在实例目录下）
3. 刷新前端页面，应自动显示 `stopped` 而非 `running`
4. 重新启动服务器，确认恢复正常

```powershell
cd E:\stardew-server-anxi-panel\backend
go test ./...
```

全部通过。

### 下一步注意事项

- 此修复每次前端请求都会调用 `ComposePs`，如果 Docker daemon 响应慢可能影响页面加载速度。后续可考虑加缓存或降低轮询频率。
- 如果未来有更多中间状态（如 `server_initializing`）需要校验，可扩展 `isRunningState()`。

## Milestone 8: Frontend MVP 整理与交付化

### 目标

将当前前端整理为可交付的 Stardew 单游戏面板 MVP。不重做功能，只做结构拆分和体验打磨。

### 改了什么

**前端结构拆分（14 个模块）：**

原 `App.tsx`（~2340 行）拆分为：

| 新文件 | 内容 | 行数 |
|--------|------|------|
| `src/core/helpers.ts` | errorMessage, formatDate, formatBytes, shortJobID, statusClass, isTerminalJobStatus, appendUniqueLog, stateLabel, roundPercent, formatPercent | ~60 |
| `src/core/StatusBadge.tsx` | 状态徽章组件 | ~6 |
| `src/core/Field.tsx` | 表单字段组件 | ~8 |
| `src/core/PasswordInput.tsx` | 密码输入组件 + usePasswordToggle hook | ~40 |
| `src/core/StatusPill.tsx` | Docker 状态指示器 | ~12 |
| `src/core/CommandOutput.tsx` | 命令输出展示 | ~15 |
| `src/core/SetupPanel.tsx` | 管理员初始化面板 | ~40 |
| `src/core/LoginPanel.tsx` | 登录面板 | ~35 |
| `src/games/stardew/InstanceStateCard.tsx` | 实例状态卡片 | ~20 |
| `src/games/stardew/InstallSection.tsx` | 安装区域（含 Steam Guard、QR、进度条） | ~370 |
| `src/games/stardew/LifecycleSection.tsx` | 生命周期（启动/停止/重启、邀请码、存档启动面板、上传存档 Modal） | ~230 |
| `src/games/stardew/JobsSection.tsx` | 任务中心 | ~110 |
| `src/games/stardew/DockerSection.tsx` | Docker 状态区域 | ~85 |
| `src/games/stardew/install-helpers.ts` | 安装流程辅助函数（进度解析、错误消息等） | ~140 |

精简后的 `App.tsx`：~600 行（路由 + Dashboard 编排）。

**CSS 修复：**
- 合并两处重复的 `.modal-overlay` / `.modal-card` 定义
- `.lifecycle-section` 从缺失的 CSS 变量改为 Stardew 主题色
- `.save-card`、`.preflight-result` 等同理
- 新增 `.lifecycle-state-game_installed`、`.lifecycle-state-save_required`、`.lifecycle-state-ready_to_start` 状态色

**主面板打磨：**
- eyebrow: "里程碑 7" → "Stardew Valley 管理面板"
- 安装区 "已安装" 下方删除了过时的 "启动服务器将在下一阶段实现" 提示

### 影响的文件

| 文件 | 变更类型 |
|------|---------|
| `frontend/src/App.tsx` | 重写（精简为路由+Dashboard） |
| `frontend/src/App.css` | 修改（合并重复定义、替换 CSS 变量） |
| `frontend/src/core/*.tsx` | 新建（8 个通用组件/工具） |
| `frontend/src/games/stardew/InstanceStateCard.tsx` | 新建 |
| `frontend/src/games/stardew/InstallSection.tsx` | 新建 |
| `frontend/src/games/stardew/LifecycleSection.tsx` | 新建 |
| `frontend/src/games/stardew/JobsSection.tsx` | 新建 |
| `frontend/src/games/stardew/DockerSection.tsx` | 新建 |
| `frontend/src/games/stardew/install-helpers.ts` | 新建 |
| `docs/handoff-roadmap.md` | 更新 Milestone 8 状态 |

后端文件未修改。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
go test ./...

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

全部通过。浏览器验证：
- 登录后直接进入 Stardew 面板
- 安装/Steam Guard/启动/停止/重启/邀请码/创建存档/上传存档流程正常
- 页面无遮挡和布局错位

### 下一步注意事项

- 如果未来引入 React Router，建议 `react-router-dom` v6+，路由：`/` → Single Game Mode 入口，`/instances/:id` → 游戏面板。
- `frontend/src/core/` 已建立，后续 Multi Game Mode 可扩展 `frontend/src/games/minecraft/` 等。
- CSS 仍为单一文件，如需模块化可拆分为各组件的 `.css` 文件。
- 未引入 TanStack Query / Zustand，当前 MVP 阶段足够。如果后续 API 请求变复杂，可考虑引入。

## Milestone 9: 存档管理与前端首页信息架构收口

### 目标

把当前"长调试页"整理成真正可用的管理面板，补齐存档管理闭环。

### 改了什么

**后端：**

| 文件 | 改动 |
|------|------|
| `backend/internal/games/registry/types.go` | `SaveInfo` 新增 `IsActive bool`；新增 `SavesListResult` 类型 |
| `backend/internal/games/stardew_junimo/saves.go` | 新增 `GetActiveSaveName(dataDir)`（读取 gameloader.json）、`DeleteSave(dataDir, saveName)`；`ListSaves` 增加 active save 标记 |
| `backend/internal/web/lifecycle_handlers.go` | 新增 `handleSavesList`、`handleSaveSelect`、`handleSaveSelectAndStart`、`handleSaveDelete` |
| `backend/internal/web/instance_handlers.go` | 注册 4 条新路由：`GET .../saves`、`POST .../saves/select`、`POST .../saves/select-and-start`、`DELETE .../saves/:name` |

**前端：**

| 文件 | 改动 |
|------|------|
| `frontend/src/types.ts` | `SaveInfo` 新增 `isActive`；新增 `SavesListResult` |
| `frontend/src/api.ts` | 新增 `getSaves`、`selectSave`、`selectSaveAndStart`、`deleteSave` |
| `frontend/src/games/stardew/SavesSection.tsx` | **新建** — 存档列表、空状态、选择/启动/删除、创建/上传入口、上传预览确认页 |
| `frontend/src/games/stardew/LifecycleSection.tsx` | **重写** — 移除内联 SaveCard/上传 Modal/NewGameCreator，仅保留状态+控制按钮 |
| `frontend/src/games/stardew/JobsSection.tsx` | 测试按钮通过 `VITE_SHOW_DEV_TOOLS=true` 控制，默认隐藏 |
| `frontend/src/App.tsx` | Dashboard 重构为分层布局 |
| `frontend/src/App.css` | 新增 dashboard-main、saves-section、collapsible 等样式 |

### 影响接口

```text
GET    /api/instances/:id/saves                — 存档列表 + activeSaveName
POST   /api/instances/:id/saves/select         — 选择存档
POST   /api/instances/:id/saves/select-and-start — 选择并启动
DELETE /api/instances/:id/saves/:name           — 删除存档（admin-only）
```

### 布局变化

```text
Before:  所有区域在 .dashboard-grid 中平铺
After:
  .dashboard-status-row:  用户卡 + 实例状态卡
  InstallSection:         未安装时显示
  .dashboard-main:
    .dashboard-main-left:  LifecycleSection + SavesSection
    .dashboard-main-right: JobsSection
  .dashboard-advanced:    折叠 — DockerSection + 用户管理
  登出按钮
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
go test ./...

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

浏览器验证清单：
- 无存档时看到空状态 + 创建/上传入口
- 已有存档能列出，active save 高亮
- 选择存档、选择并启动、删除存档功能正常
- 上传存档预览信息完整（农场名/农民名/游戏时间/地图/大小/修改时间）
- 普通启动无存档时引导到存档区域
- 测试任务按钮默认不显示
- Docker 调试区折叠在"高级设置"中
- 移动端单列堆叠无溢出

### 下一步注意事项

- 备份能力（`POST .../saves/:name/backup`）未实现，留给 M13。
- `GameDriver` 接口的 `SelectSave`/`DeleteSave` 缺少 `instance` 参数，handler 直接调用 `sj` 包函数。后续可改接口签名。
- 删除存档前暂不做自动备份（后端无备份能力），仅做二次确认。

## Milestone 9 Review Fixes（2026-06-27）

### 修了什么

M9 存档管理 review 发现的安全性和一致性问题修复：

**1. DeleteSave 路径安全加固**

`DeleteSave(saveName)` 之前只检查空名称，`..`、`/`、`\`、绝对路径均可逃逸到 Saves 根目录外。现在新增：
- `validateSaveName()`：拒绝空、`.`、`..`、含路径分隔符、绝对路径、目录穿越
- `resolveSavePath()`：计算绝对路径后用 `filepath.Rel` 确认目标仍在 Saves 根目录下
- `DeleteSave()` 重写：校验目标存在且是目录，才执行删除
- `ValidateSaveExists()`：web 层调用的安全校验 helper，复用上述逻辑

**2. 选择存档前校验存在性**

`handleSaveSelect` 和 `handleSaveSelectAndStart` 之前直接调用 `SetActiveSave`，即使存档不存在也会写入 gameloader 并推进状态。现在在 `SetActiveSave` 前调用 `ValidateSaveExists`，不存在时返回 404 `save_not_found`。

**3. 删除前 reconcile 真实运行状态**

`handleSaveDelete` 之前只看 DB 中的 instance state。如果 DB 显示 `stopped` 但容器实际 `running`，可能删除正在使用的存档。现在在检查前调用 `reconcileInstanceState`，通过 `ComposePs` 获取真实容器状态。

**4. 前端存档列表 job 完成后刷新**

`SavesSection` 之前只在组件挂载时加载一次。创建/上传/选择并启动任务完成后，存档列表不会自动刷新。现在：
- App.tsx 新增 `savesRefreshKey` 状态
- job finished SSE 回调中递增 `savesRefreshKey`
- `SavesSection` 新增 `refreshTrigger` prop，变化时重新加载

**5. 存档元数据读取增强**

`readSaveInfo` 之前只尝试 `SaveGameInfo` 一个文件名。现在依次尝试：
- `SaveGameInfo`（1.5 标准格式）
- `SaveGameInfo.xml`（部分版本）
- 主存档文件（`<saveName>` 同名文件，最后备选）

### 影响文件

| 文件 | 改动 |
|------|------|
| `backend/internal/games/stardew_junimo/saves.go` | 重写 `DeleteSave`；新增 `validateSaveName`、`resolveSavePath`、`ValidateSaveExists`；增强 `readSaveInfo` |
| `backend/internal/web/lifecycle_handlers.go` | `handleSaveSelect`/`handleSaveSelectAndStart` 增加 `ValidateSaveExists` 调用；`handleSaveDelete` 增加 `reconcileInstanceState` |
| `backend/internal/games/stardew_junimo/saves_test.go` | 新增 7 个测试 |
| `frontend/src/App.tsx` | 新增 `savesRefreshKey`；job 完成时触发刷新；传 `refreshTrigger` |
| `frontend/src/games/stardew/SavesSection.tsx` | 新增 `refreshTrigger` prop；监听变化重新加载 |

### 影响接口

无接口变更。现有接口行为更安全：
- `POST .../saves/select`：存档不存在时返回 404（之前会写入无效 gameloader）
- `POST .../saves/select-and-start`：同上
- `DELETE .../saves/:name`：路径穿越被拒绝（之前可逃逸）；删除前检查真实容器状态

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
# 全部通过

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 通过
```

浏览器验证：
1. 选择不存在的存档 → 404 `save_not_found`
2. 选择并启动不存在的存档 → 404，服务器不启动
3. 删除存档时服务器实际运行中 → 409 `server_running`
4. 创建/上传存档任务完成后，存档列表自动刷新
5. 存档列表中农场名/农民名/游戏时间等元数据正常显示（如有 SaveGameInfo 文件）

### 仍没有做的后续事项

- `GameDriver` 接口的 `SelectSave(ctx, name)` / `DeleteSave(ctx, name)` 缺少 `instance` 参数。建议后续改签名为 `SelectSave(ctx, instance, name)` / `DeleteSave(ctx, instance, name)`，让 driver 自己做路径安全校验，web 层不再直接调用 `sj` 包函数。
- 备份能力（`POST .../saves/:name/backup`）未实现，删除前暂不做自动备份。
- `readSaveInfo` 的容错增强基于 Stardew 1.5/1.6 已知格式。如果 Junimo 创建的存档结构有差异，需要实际联调确认 XML 字段映射。

## Milestone 9 Review Fixes 第二轮（2026-06-27）

### 修了什么

**1. 上传/导入存档路径安全加固**

- `PreviewSaveZip()` 在 `detectSaveFolderName()` 后立即调用 `validateSaveName(detectedSaveName)`，拒绝 `.`、`..`、含路径分隔符、绝对路径、保留路由名等危险目录名。
- `ImportSaveToVolume()` 开头新增 `validateSaveName(saveName)` 校验，不再只依赖 preview 阶段。
- `ImportSaveToVolume()` 使用 `resolveSavePath()` 计算绝对路径，确保目标目录仍在 `<dataDir>/.local-container/saves/Saves` 根目录下。明确拒绝目标目录等于 Saves 根目录本身。
- `SetActiveSave()` 也新增 `validateSaveName()` 校验。
- `findSaveDir()` 入口前已通过 `validateSaveName()` 统一校验，安全链路完整。

**2. 保留路由名冲突规避**

`validateSaveName()` 新增保留名称列表：`preflight`、`custom-new-game`、`upload-preview`、`upload-commit-and-start`、`select`、`select-and-start`、`delete`。这些名称会被拒绝，避免与 `DELETE /api/instances/:id/saves/:name` 路由冲突。

**3. 普通启动 active save 一致性校验**

`handleInstanceStart()` 现在：
- 读取 `sj.GetActiveSaveName(instance.DataDir)`
- 如果 active save 为空 → 返回 409 `active_save_required`，中文提示"没有已选择的启动存档，请先创建、上传或选择一个存档"
- 如果 active save 非空但 `sj.ValidateSaveExists()` 失败 → 返回 409 `active_save_missing`，中文提示"上次选择的存档不存在，请重新选择存档"
- 只有 active save 存在且是目录时，才能启动
- 保留原有"没有任何存档时要求创建/上传"的 `save_required` 逻辑

**4. 存档信息解析兼容 Junimo/真实 Stardew SaveGameInfo**

`readSaveInfo()` 重写，同时支持两种 XML 结构：
- 旧/完整结构 `<SaveGame><player>...</player></SaveGame>`：读取 `player>name`、`player>farmName`、`year`、`currentSeason`、`dayOfMonth`、`whichFarm`
- Junimo/真实 `<Farmer>...</Farmer>` 结构：读取 `name`、`farmName`、`yearForSaveGame`、`dayOfMonthForSaveGame`、`seasonForSaveGame`（数字 0-3 映射为 spring/summer/fall/winter）

关键修复：
- `whichFarm` 改为 `*int` 类型，区分"字段缺失"和"字段值为 0"。`<Farmer>` 结构不含 `whichFarm`，`FarmType` 留空（前端显示"地图未知"），不再误显示为 "standard"。
- `seasonForSaveGame` 数字正确映射：0→spring、1→summer、2→fall、3→winter。
- 两种结构都无法解析时才返回 `ParseError`。XML 能解析但部分字段缺失时展示能读到的信息。

**5. 前端适配**

- `LifecycleSection.tsx`：`handleStart()` 新增 `active_save_required` 和 `active_save_missing` 错误码处理，显示中文可读提示并滚动到存档管理区域。
- `SavesSection.tsx`：存档列表中 `farmType` 为空时显示"地图未知"而非隐藏。上传预览中地图类型缺失时也显示"未知"。游戏时间季节显示中文（春/夏/秋/冬）。

### 影响文件

| 文件 | 改动 |
|------|------|
| `backend/internal/games/stardew_junimo/saves.go` | 重写 `readSaveInfo()` 支持 Farmer 结构；`validateSaveName()` 新增保留名称；`PreviewSaveZip()` 新增校验；`ImportSaveToVolume()` 新增校验和路径安全；`SetActiveSave()` 新增校验；新增 `seasonFromInt()` |
| `backend/internal/web/lifecycle_handlers.go` | `handleInstanceStart()` 新增 active save 校验 |
| `backend/internal/games/stardew_junimo/saves_test.go` | 新增 ~20 个测试 |
| `frontend/src/games/stardew/LifecycleSection.tsx` | 处理新错误码 |
| `frontend/src/games/stardew/SavesSection.tsx` | 地图未知显示、季节中文翻译 |

### 影响接口

- `POST /api/instances/:id/start`：新增两种 409 错误码 `active_save_required`、`active_save_missing`
- `POST /api/instances/:id/saves/upload-preview`：危险 ZIP 目录名现在被拒绝
- `POST /api/instances/:id/saves/upload-commit-and-start`：危险 saveName 被拒绝
- `POST /api/instances/:id/saves/select`：保留名称被拒绝
- `DELETE /api/instances/:id/saves/:name`：保留名称被拒绝

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
# 全部通过

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 通过
```

浏览器验证：
1. 对存档 `1111_442155312`：农场名 `1111`、农民名 `Server`、游戏时间 `第 1 年 春 第 1 天`、地图未知
2. 删除 active save 后，直接点"启动服务器" → 409 `active_save_missing`，提示重新选择存档
3. 上传包含 `.` 或 `..` 目录名的 ZIP → 被拒绝
4. 尝试创建名为 `select` 的存档 → 被拒绝

### 仍没有做的后续事项

- `GameDriver` 接口的 `SelectSave(ctx, name)` / `DeleteSave(ctx, name)` 缺少 `instance` 参数。建议后续改签名为 `SelectSave(ctx, instance, name)` / `DeleteSave(ctx, instance, name)`。
- 备份能力（`POST .../saves/:name/backup`）未实现，删除前暂不做自动备份。
- 删除接口路由 body 化（`POST /api/instances/:id/saves/delete`）未实现，当前通过保留名称规避路由冲突。如果未来需要支持这些名称的存档，需要改用 body 传 name。

## Milestone 9 Review Fixes 第三轮（2026-06-27）

### 修了什么

**1. 后端创建/上传/选择并启动接口禁止运行中操作**

新增 `ensureInstanceNotRunning()` helper，内部调用 `reconcileInstanceState` 获取真实 Docker 状态，若 `running` 或 `starting` 则返回 409 `server_running`。

应用于以下 handler（在任何写文件/导入存档/SetActiveSave/Start 之前）：
- `handleSavesCustomNewGame`
- `handleSavesUploadCommitAndStart`（Cancel 分支不受影响，只清理 token）
- `handleSaveSelectAndStart`
- `handleSaveSelect`
- `handleSaveDelete`（改用统一 helper，逻辑不变）

**2. ZIP 路径穿越校验严格化**

`PreviewSaveZip()` 的路径安全检查从 `filepath.Clean(name) + HasPrefix("..")` 改为逐段检查：
- 将路径按 `/` 分段
- 拒绝任何段为 `..`（目录穿越）
- 拒绝任何段为 `.`（当前目录引用）
- 拒绝任何段为空（双斜杠 `foo//bar`）

原有绝对路径、符号链接、单文件大小、总解压大小校验不变。`extractZipSecure()` 保留最终路径边界检查作为第二道防线。

**3. 存档地图类型从主存档文件补读**

`readSaveInfo()` 解析策略改进：

当 `SaveGameInfo` 为 `<Farmer>` 结构且不含 `whichFarm` 时，继续读取主存档文件 `Saves/<saveName>/<saveName>`（完整 `<SaveGame>` XML），从中提取 `<whichFarm>` 并映射为地图类型。

同时支持 `whichFarm` 的两种格式：
- 整数（0-7）：standard/riverland/forest/hilltop/wilderness/fourcorners/beach/meadowlands
- 字符串（如 "MeadowlandsFarm"）：映射为对应类型

新增 `farmTypeLabelFromString()` 和 `readWhichFarmFromMainFile()` 辅助函数。`<SaveGame>` 结构的解析也从 `*int` 改为 `string` 以兼容两种格式。

容错逻辑：
- SaveGameInfo 能解析但主存档缺失/解析失败 → 保留已解析字段，FarmType 留空（前端显示"地图未知"），不设 ParseError
- 只有所有候选 XML 都无法解析时才设 ParseError

### 影响文件

| 文件 | 改动 |
|------|------|
| `backend/internal/web/lifecycle_handlers.go` | 新增 `ensureInstanceNotRunning()`；5 个 handler 使用统一 running 状态保护 |
| `backend/internal/games/stardew_junimo/saves.go` | ZIP 路径逐段检查；`readSaveInfo` 读主存档补 whichFarm；新增 `farmTypeLabelFromString`、`readWhichFarmFromMainFile`、`whichFarmRe`；`<SaveGame>` 的 `whichFarm` 改为 string 类型 |
| `backend/internal/games/stardew_junimo/saves_test.go` | 新增 10 个测试 |

### 影响接口

- `POST /api/instances/:id/saves/custom-new-game`：运行中返回 409（之前可绕过前端直接调用）
- `POST /api/instances/:id/saves/upload-commit-and-start`：运行中返回 409（Cancel 除外）
- `POST /api/instances/:id/saves/select-and-start`：运行中返回 409
- `POST /api/instances/:id/saves/select`：运行中返回 409
- `DELETE /api/instances/:id/saves/:name`：逻辑不变，改用统一 helper
- `POST /api/instances/:id/saves/upload-preview`：ZIP 路径检查更严格（`foo/../bar` 等之前可通过）

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
# 全部通过

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 通过
```

浏览器验证：
1. 对存档 `1111_442155312`：农民名 `Server`、农场名 `1111`、游戏时间 `第 1 年 春 第 1 天`、地图显示 `meadowlands`（从主存档读取）
2. 对存档 `TESTPANELCUSTOM_442153826`：地图显示 `beach`（whichFarm=6）
3. 服务器运行中时，调用创建/上传提交/选择并启动/选择接口 → 409 `server_running`
4. 上传包含 `foo/../bar/SaveGameInfo` 的 ZIP → 被拒绝
5. 上传包含 `./SaveGameInfo` 的 ZIP → 被拒绝
6. 上传包含 `foo//SaveGameInfo` 的 ZIP → 被拒绝

### 仍没有做的后续事项

- `GameDriver` 接口签名缺少 `instance` 参数。
- 备份能力未实现。
- 删除接口路由 body 化未实现。
- `whichFarm` 字符串格式的映射基于已知 Stardew 1.6 农场类型。如果未来出现新的自定义农场类型字符串，需要扩展 `farmTypeLabelFromString`。

## Milestone 10: Mods 管理 + M9 Review 补齐（2026-06-27）

### 修了什么

**1. M9 遗留 review 补齐**

| 问题 | 状态 |
|------|------|
| ZIP 上传目录项兼容 | ✅ 已有 `TrimSuffix(name, "/")` 处理（`saves.go:350`），测试 `TestPreviewSaveZip_AcceptsDirectoryEntry` 已覆盖 |
| whichFarm trim | ✅ 已有 `strings.TrimSpace` 处理（`saves.go:270`），测试 `TestFarmTypeLabelFromString` 已覆盖 |
| running 保护 web handler 测试 | ✅ 新增 `saves_handlers_test.go`，覆盖 7 个操作 × running/starting/stopped 状态 |

**2. Mods 管理后端**

新建 `backend/internal/games/stardew_junimo/mods.go`：
- `ListMods(dataDir)` — 扫描 `.local-container/mods/` 一级目录，读取 `manifest.json`
- `UploadModZip(dataDir, zipPath)` — 安全校验 + 解压 + manifest 校验 + 重复 UniqueID 检查
- `DeleteMod(dataDir, modID)` — 按 folderName 或 UniqueID 删除，路径安全校验
- `ExportModsZip(dataDir)` — 打包所有 Mod 为 ZIP，路径均为相对路径
- `GetModsRestartRequired/SetModsRestartRequired/ClearModsRestartRequired` — 重启标志文件
- `migrateModsCompose` — 为已有实例添加 mods bind mount

`registry/types.go` 的 `ModInfo` 扩展为完整字段，新增 `ModsListResult`。

compose 模板新增 `.local-container/mods:/data/Mods` bind mount，挂载到 SMAPI 的 Mods 目录。

**3. Mods 管理 API**

```text
GET    /api/instances/:id/mods              — Mod 列表 + restartRequired
POST   /api/instances/:id/mods/upload       — 上传 Mod ZIP（admin-only，运行中禁止）
DELETE /api/instances/:id/mods/:modId        — 删除 Mod（admin-only，运行中禁止）
POST   /api/instances/:id/mods/export       — 导出所有 Mod 为 ZIP 下载
```

**4. Mods 管理前端**

新建 `frontend/src/games/stardew/ModsSection.tsx`：
- Mod 列表：名称、UniqueID、版本、作者、描述、解析错误
- 上传按钮：选择 ZIP → 上传并安装
- 删除按钮：二次确认，running 时禁用
- 导出按钮：下载 ZIP
- 重启提示 banner

Dashboard 布局：存档管理下方新增 Mod 管理区域。

### 影响文件

| 文件 | 改动 |
|------|------|
| `backend/internal/games/registry/types.go` | `ModInfo` 扩展；新增 `ModsListResult` |
| `backend/internal/games/stardew_junimo/mods.go` | **新建** |
| `backend/internal/games/stardew_junimo/mods_test.go` | **新建** — 32 个测试 |
| `backend/internal/games/stardew_junimo/compose_template.go` | 新增 mods bind mount |
| `backend/internal/games/stardew_junimo/installer.go` | 新增 `migrateModsCompose` |
| `backend/internal/web/lifecycle_handlers.go` | 新增 4 个 mods handler |
| `backend/internal/web/instance_handlers.go` | 注册 4 条 mods 路由 |
| `backend/internal/web/saves_handlers_test.go` | **新建** — running 保护 handler 测试 |
| `backend/internal/web/auth_handlers_test.go` | 新增 `newTestHandlerWithStore` |
| `frontend/src/types.ts` | 新增 `ModInfo`、`ModsListResult` |
| `frontend/src/api.ts` | 新增 `getMods`、`uploadMod`、`deleteMod`、`exportMods` |
| `frontend/src/games/stardew/ModsSection.tsx` | **新建** |
| `frontend/src/App.tsx` | 引入 `ModsSection` |
| `frontend/src/App.css` | 新增 mods 相关样式 |

### 影响接口

```text
GET    /api/instances/:id/mods              — 新增
POST   /api/instances/:id/mods/upload       — 新增
DELETE /api/instances/:id/mods/:modId        — 新增
POST   /api/instances/:id/mods/export       — 新增
```

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
# 全部通过

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 通过（35 modules，无 TypeScript 错误）
```

浏览器验证：
1. 上传包含单个 Mod 的 ZIP → 成功，列表显示 Mod 信息
2. 上传包含多个 Mod 的 ZIP → 成功
3. 上传无 manifest 的 ZIP → 拒绝
4. 上传重复 UniqueID → 拒绝 `mod_exists`
5. 删除 Mod → 二次确认成功
6. 导出 Mod → 下载 ZIP
7. 上传/删除后 → 显示"Mod 变更需要重启服务器生效"
8. 服务器运行中 → 上传/删除按钮禁用

### 仍没有做的后续事项

- Mod 启用/禁用（SMAPI 不支持热禁用，需重启）
- Mod 依赖关系检查
- Mod 自动备份
- `GameDriver` 接口的 `ListMods`/`UploadMod`/`DeleteMod` 签名仍返回 `ErrNotImplemented`
- compose mods mount 迁移在下次 install 时自动执行；已有实例需重新安装
- running 保护 handler 测试中 `reconcileInstanceState` 因无真实 Docker 而跳过。如有 Docker 联调环境，建议补集成测试确认 reconcile + 409 的完整链路

## M10 Review Fixes（2026-06-27）

### 修了什么

**1. 存档导出功能（新增）**

新增 `POST /api/instances/:id/saves/:name/export` 接口，将单个存档文件夹打包为 ZIP 下载。

- `saves.go` — 新增 `ExportSaveZip(dataDir, saveName)`、`buildSaveZipName(saveName, info)`、`seasonLabelCN(season)`
- ZIP 命名规则：`存档名_游戏时间.zip`（如 `FarmerName_12345_3年_冬_28日.zip`），无游戏时间信息时退化为 `存档名.zip`
- `lifecycle_handlers.go` — 新增 `handleSaveExport`
- `instance_handlers.go` — 注册 `POST /api/instances/:id/saves/:name/export`（4 段路径）
- `api.ts` — 新增 `exportSave(name)` 返回 `{blob, filename}`
- `SavesSection.tsx` — 每个存档行新增「导出」按钮，使用服务端 Content-Disposition 文件名

**2. Mod 导出命名优化**

单个 Mod 导出命名改为 `mod名_作者名.zip`（如 `My_Cool_Mod_AuthorName.zip`），多个 Mod 退化为 `stardew-mods-N.zip`。

- `mods.go` — 新增 `buildModsZipName(root, dirs)`、`sanitizeFileNamePart(s)`

**3. Mod ZIP 导入原子性（P2a → P3c）**

重构 `UploadModZip` 为三阶段：
1. 提取到临时目录
2. 全量预检（manifest 解析、ZIP 内重复 UniqueID、已安装 UniqueID 冲突、目标目录冲突）
3. 全部通过后统一移动，移动失败时回滚已移动目录

**4. 请求体大小硬限制（P2b）**

Mod 上传和存档上传 handler 均加 `http.MaxBytesReader(w, r.Body, maxRequestBody)`（220MB 硬上限），超限请求直接拒绝不落盘。

**5. 启动/重启后清除 restart 标志（P3）**

`doStart` 和 `doRestart` 在进入 `running` 状态后调用 `ClearModsRestartRequired(dataDir)`。

### 影响文件

| 文件 | 改动 |
|------|------|
| `backend/internal/games/stardew_junimo/saves.go` | 新增 `ExportSaveZip`、`buildSaveZipName`、`seasonLabelCN` |
| `backend/internal/games/stardew_junimo/saves_test.go` | 新增 6 个存档导出测试 |
| `backend/internal/games/stardew_junimo/mods.go` | `UploadModZip` 原子性重构 + 移动回滚；新增 `buildModsZipName`、`sanitizeFileNamePart` |
| `backend/internal/games/stardew_junimo/mods_test.go` | 新增 3 个 mods 命名测试 + 1 个原子性测试 |
| `backend/internal/games/stardew_junimo/lifecycle.go` | `doStart`/`doRestart` 清除 restart 标志 |
| `backend/internal/web/lifecycle_handlers.go` | 新增 `handleSaveExport`；Mod/存档上传加 `MaxBytesReader`；新增常量 `maxModFormSize`/`maxRequestBody` |
| `backend/internal/web/instance_handlers.go` | 注册 `POST .../saves/:name/export` |
| `frontend/src/api.ts` | `exportSave` 返回 `{blob, filename}`；`exportMods` 返回 `{blob, filename}` |
| `frontend/src/games/stardew/SavesSection.tsx` | 导出使用服务端文件名 |
| `frontend/src/games/stardew/ModsSection.tsx` | 导出使用服务端文件名 |

### 影响接口

```text
POST /api/instances/:id/saves/:name/export  — 新增
POST /api/instances/:id/mods/export          — 命名变更（单 Mod: mod名_作者名.zip）
```

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

## Milestone 11: Console and Commands（2026-06-27）

### 目标

为 Stardew Junimo 面板提供安全的控制台/命令功能。用户可以在前端执行常用 Junimo/SMAPI 命令，查看命令输出，并支持服务器喊话。

### 改了什么

**后端新增/修改：**

| 文件 | 改动 |
|------|------|
| `backend/internal/games/stardew_junimo/console.go` | **新建** — 命令 allowlist 定义（7 个命令）、`RunAllowlistedCommand`、`SendSay`、`ListCommands`、`stripControlChars`、`CommandError` 结构化错误类型 |
| `backend/internal/games/stardew_junimo/console_test.go` | **新建** — 22 个测试 |
| `backend/internal/web/lifecycle_handlers.go` | 新增 3 个 handler + `consoleRunner` 接口 |
| `backend/internal/web/instance_handlers.go` | 注册 3 条新路由 |

**新增 API：**

```text
GET  /api/instances/:id/commands          — 返回可用命令列表（登录用户，按角色过滤）
POST /api/instances/:id/commands/run      — 执行 allowlist 命令（admin-only）
POST /api/instances/:id/commands/say      — 服务器喊话（登录用户）
```

**命令 allowlist（7 个命令）：**

| ID | 显示名 | attach-cli stdin | AdminOnly |
|----|--------|-----------------|-----------|
| info | 服务器信息 | `info\nquit\n` | false |
| invitecode | 邀请码 | `invitecode\nquit\n` | false |
| settings-show | 查看设置 | `settings show\nquit\n` | true |
| settings-validate | 校验设置 | `settings validate\nquit\n` | true |
| rendering-status | 渲染状态 | `rendering status\nquit\n` | true |
| host-auto | 自动托管状态 | `host-auto\nquit\n` | true |
| host-visibility | 可见性状态 | `host-visibility\nquit\n` | true |

**前端新增/修改：**

| 文件 | 改动 |
|------|------|
| `frontend/src/types.ts` | 新增 `ConsoleCommandDef`、`CommandsListResult`、`CommandRunResult` |
| `frontend/src/api.ts` | 新增 `getCommands`、`runCommand`、`sendSay` |
| `frontend/src/games/stardew/ConsoleSection.tsx` | **新建** — 命令按钮网格、喊话输入框、命令历史（可折叠输出） |
| `frontend/src/App.tsx` | 引入 `ConsoleSection`，放在 ModsSection 下方 |
| `frontend/src/App.css` | 新增 `.console-section`、`.command-btn-grid`、`.console-say-area`、`.console-history` 等样式（绿色主题） |

### 影响接口

```text
GET  /api/instances/:id/commands          — 新增
POST /api/instances/:id/commands/run      — 新增
POST /api/instances/:id/commands/say      — 新增
```

### 安全边界

1. **结构化输入**：前端只传 `{command: "info"}`，后端在 allowlist 中查找，不拼接任意 shell
2. **无 shell 注入**：`ComposeExecPipe` 使用 args 数组，不经 shell 解析
3. **say 清理**：`stripControlChars` 移除控制字符（保留 \n \r \t）、限制 200 字符、拒绝空消息
4. **状态检查**：服务器未运行时返回 409 `server_not_running`
5. **权限分离**：info/invitecode 普通用户可用，settings/rendering/host 命令 admin-only
6. **敏感信息**：不记录 Steam/VNC 密码、session token

### 测试覆盖

`console_test.go`（22 个测试）：
- 非 allowlist 命令被拒绝（含 shell 特殊字符注入尝试）
- AdminOnly 命令对普通用户拒绝、对管理员允许
- 公共命令对非管理员允许
- 服务器未运行（stopped/starting）返回 `server_not_running`
- 命令执行通过 `attach-cli`，stdin 和 args 正确传递
- 非零退出码正确处理
- say 拒绝空消息、拒绝过长消息（>200 字符）
- say 清理控制字符
- say 构建正确 stdin（`say <msg>\nquit\n`）
- `ListCommands` 按角色过滤
- `stripControlChars` 各种输入

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

### 仍没有做的后续事项

- 实时日志流（SSE/WebSocket tail logs）未实现
- 完整交互式控制台（类似终端）未实现，当前只支持单次命令执行
- 更多命令参数 schema（如 `settings set <key> <value>`）未实现
- 审计日志（谁在什么时间执行了什么命令）未实现
- `GameDriver` 接口的 `ExecCommand(ctx, cmd)` 签名仍返回 `ErrNotImplemented`，console 能力通过 `consoleRunner` type-assert 接口暴露

## M11 Review Fixes（2026-06-27）

### 修了什么

**P1a: say 换行注入漏洞**

`stripControlChars` 明确保留了 `\n`、`\r`、`\t`，用户可以通过 say 消息注入多条 attach-cli 命令（如 `hello\nsettings show\nquit`），绕过 allowlist 和 admin-only 边界。

修复：
- 新增 `sanitizeSayMessage()` 函数，将**所有**控制字符（包括 `\n`、`\r`、`\t`）替换为空格，然后用 `strings.Fields` + `strings.Join` 合并连续空格
- `sendSay()` 改用 `sanitizeSayMessage()` 替代 `stripControlChars()`
- 新增 2 个注入防护测试：`TestSendSay_PreventsNewlineInjection`、`TestSendSay_PreventsCarriageReturnInjection`
- 新增 `TestSanitizeSayMessage` 覆盖各种输入

**P2a: 普通用户执行公共命令被 403**

`handleCommandRun` 使用 `requireAdmin`，导致普通用户点击 info/invitecode 时直接 403，到不了 driver 内部的 allowlist 权限判断。

修复：
- `handleCommandRun` 改用 `requireAuth`
- admin-only 命令的权限由 driver 的 `RunAllowlistedCommand` 内部判断（检查 `isAdmin` 参数）
- 普通用户执行 info/invitecode 正常通过

**P2b: 命令/say 没有 reconcile 真实容器状态**

`handleCommandRun` 和 `handleCommandSay` 在 loadInstance 后直接执行，若 DB 仍是 running 但 Docker 容器已停，不会稳定返回 `server_not_running`。

修复：
- 两个 handler 在执行前调用 `reconcileInstanceState`
- reconcile 后检查 `instance.State != running` 时直接返回 409 `server_not_running`

**P1b: attach-cli 需要 TTY，ComposeExecPipe -T 不可用**

`ComposeExecPipe` 使用 `docker compose exec -T` 禁用 TTY，但 JunimoServer 的 `attach-cli` 需要终端（报 "open terminal failed: not a terminal"）。

修复：
- 新增 `ComposeExecTTY` 函数，通过 Docker Engine API 直接创建 exec 实例并分配 TTY（不走 CLI 的 `-T` 路径）
- Windows 实现：通过 `\\.\pipe\docker_engine` named pipe 调用 Docker API（create exec → attach → start → read output）
- Unix 实现：placeholder（返回错误提示，需要后续实现 creack/pty 方案）
- `LifecycleDockerService` 接口新增 `ComposeExecTTY` 方法
- `console.go` 的 `runCommand` 和 `sendSay` 改用 `ComposeExecTTY` 替代 `ComposeExecPipe`

### 影响文件

| 文件 | 改动 |
|------|------|
| `backend/internal/games/stardew_junimo/console.go` | `sanitizeSayMessage()` 替代 `stripControlChars` 用于 say；`runCommand`/`sendSay` 改用 `ComposeExecTTY`；`commandExecutor` 接口改用 `ComposeExecTTY` |
| `backend/internal/games/stardew_junimo/console_test.go` | 新增注入防护测试、`TestSanitizeSayMessage`；fake 改用 `ComposeExecTTYResult` |
| `backend/internal/games/stardew_junimo/lifecycle.go` | `LifecycleDockerService` 接口新增 `ComposeExecTTY` |
| `backend/internal/docker/compose_tty.go` | **新建** — `ComposeExecTTY` 函数定义和 `ComposeExecTTYResult` 类型 |
| `backend/internal/docker/compose_tty_windows.go` | **新建** — Windows Docker Engine API 实现（exec create/attach/start/wait） |
| `backend/internal/docker/compose_tty_unix.go` | **新建** — Unix placeholder |
| `backend/internal/web/lifecycle_handlers.go` | `handleCommandRun` 改用 `requireAuth`；两个 handler 新增 `reconcileInstanceState` + 状态检查 |

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

### 仍没有做的后续事项

- Unix 平台的 `ComposeExecTTY` 实现（需要 creack/pty 库）
- 真实容器联调验证（当前只有 fake 测试，需要在有 Docker 的环境确认 attach-cli + TTY 实际工作）

## 联机角色/邀请码异常排查项（2026-06-27）

### 现象

使用同一个邀请码第一次搜索服务器时，显示一个空的、还没有创建角色的服务器入口。创建角色并退出游戏后，再次用同一个邀请码搜索，会出现两个入口：一个是已创建角色的服务器入口，另一个仍是空的新农夫入口。两个都能进入；进入空入口并创建角色后，会继续多出新的入口。

### 可能原因

1. **服务端没有稳定识别同一个 Steam 用户对应的 farmer** — 退出再进时没有绑定到旧 farmer，继续给"新农夫"入口
2. **初始小屋/玩家槽配置允许多角色** — 可能开了多个 cabin 或服务端默认保留空 farmer slot
3. **服务端重复新建/加载存档逻辑混乱** — 每次启动或连接时误触发 newgame/create-or-load
4. **Stardew 原生行为 + Junimo 表现混合** — 原版多人存档允许多个农民，但"空项创建后继续增加"不正常

### 排查要求

1. 查看当前 active save 的 XML，确认 `<farmhands>`、玩家列表、小屋数量、farmer `UniqueMultiplayerID` / `userID` / `name` 等字段是否每次连接后新增
2. 查看 `.local-container/saves/Saves/<activeSave>/` 下主存档文件和 SaveGameInfo，确认是否有多个 farmer/farmhand
3. 查看 Junimo/SMAPI 日志，确认连接、创建角色、保存时是否有重复 create/load/newgame 行为
4. 确认面板启动服务器时是否误触发 `newgame` 或自定义创建逻辑；普通启动必须只加载 active save，不能创建新存档或新 farmer
5. 确认 `server-settings.json` 中 cabin/多玩家相关配置是否导致每次保留新角色入口
6. 如果这是原生允许多个农民的行为，需要在文档/前端提示说明；如果是重复创建 bug，需要修复并提供清理重复 farmer 的建议或工具
7. 不要直接删除存档内容，先备份再分析

### 一句话判断

同一个农场里出现已创建角色 + 新农夫入口可能是正常多人槽位；但"每进空的创建后又多一个"需要查 XML 和日志，不能当正常放过。

### 与面板代码的关系

- `lifecycle.go` 的 `doStart()` 中 `sendNewGameCommand()` 仅在 `r.newGame == true` 时触发
- `handleInstanceStart`（普通启动）调用 `driver.Start()` 时**不设置** `NewGame: true`，所以普通启动不会触发 `POST /newgame`
- `NewGame: true` 仅在 `handleSavesCustomNewGame`（自定义新建存档并启动）中设置
- 因此面板普通启动逻辑不会误创建新存档或新 farmer
- 需要进一步排查的是 JunimoServer/Stardew 联机层面的角色槽行为，以及 `server-settings.json` 中的 `startingCabins` 配置

## M11 Review Fixes 第二轮：ComposeExecTTY 阻塞修复（2026-06-27）

### 修了什么

**1. Double start — attach 和 start 分两次调用导致挂起**

`dockerExecAttach` 已经通过 `POST /exec/{id}/start` + HTTP hijack 启动了 exec 并拿到双向流，但后面又调用了 `dockerExecStart`，等于对同一个 exec start 两次。

修复：合并为 `dockerExecAttachStart` 一个函数，只调用一次 `POST /exec/{id}/start` + hijack。删除独立的 `dockerExecStart`。

**2. stdin 写完后没有关闭写入方向**

`io.WriteString(conn, stdinData)` 后没有关闭 conn 的写入端，attach-cli 收到 `quit\n` 后可能仍等待更多输入，导致 hijack 连接不结束。

修复：改为写入后依赖 exec 进程自然退出（attach-cli 处理 quit 后退出）。通过 ctx 超时兜底关闭 conn。

**3. reader.Read 不响应 ctx 超时**

`reader.Read(buf)` 是阻塞调用，不监听 `ctx.Done()`，超时后请求一直挂着。

修复：用 goroutine 读取输出，主 goroutine 用 `select` 同时等待读取结果和 `ctx.Done()`。ctx 超时时调用 `closeConn()` 关闭 hijack 连接，解除 reader 阻塞。

**4. dockerExecWait 不是真的 wait**

只查一次 `/exec/{id}/json`，没有循环等 `Running=false`。

修复：改为 `dockerExecPollExit`，200ms 间隔轮询直到 `Running=false` 或超时。

**5. 前端无超时兜底**

`runCommand` 和 `sendSay` 没有 AbortController，后端挂起时前端一直显示"执行中"。

修复：`api.ts` 的 `runCommand` 和 `sendSay` 添加 `AbortController` + 40 秒超时。`ConsoleSection.tsx` 捕获 `AbortError` 显示"命令执行超时，请稍后重试"。

### 影响文件

| 文件 | 改动 |
|------|------|
| `backend/internal/docker/compose_tty_windows.go` | **重写** — `dockerExecAttachStart` 合并 attach+start；ctx 感知的读取；`dockerExecPollExit` 轮询退出码 |
| `frontend/src/api.ts` | `runCommand`/`sendSay` 添加 AbortController + 40s 超时 |
| `frontend/src/games/stardew/ConsoleSection.tsx` | 捕获 `AbortError` 显示超时提示 |

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

### 仍需要的验证

- 真实 Docker 环境联调：点击 info/invitecode 确认 attach-cli + TTY 实际返回输出
- 如果仍然卡住，可能需要进一步调整 stdin EOF 信号（Windows named pipe 不支持 `CloseWrite`，可能需要在写入后立即关闭整个 conn 并用单独的 goroutine 读取缓存的输出）

## M11 超时复查修复：改用 Junimo FIFO 而不是 attach-cli TTY（2026-06-27）

### 问题

上一轮修了 Docker TTY double-start 和 ctx 超时，但前端点击命令/喊话仍然会超时。真实容器联调后确认，问题不在前端 loading 状态，而在通信模型：

- `attach-cli` 是一个 tmux 交互 UI，不是一次性 stdin 命令执行器。
- `quit` 不是 attach-cli 退出命令，会被转发进 SMAPI，日志里出现 `Unknown command 'quit'`。
- `say` 也不是当前 Junimo/SMAPI 注册命令，日志里出现 `Unknown command 'say'`。
- 容器内 `/opt/base/bin/server-command-loop` 的真实输入路径是把命令写入 `/tmp/smapi-input` FIFO。

### 修了什么

- `backend/internal/games/stardew_junimo/console.go`
  - `RunAllowlistedCommand` 不再调用 `ComposeExecTTY` 和 `attach-cli`。
  - 改为用 `docker compose exec -T server tee -a /tmp/smapi-input` 写入单行 allowlist 命令。
  - 写入后从 `/tmp/server-output.log` 读取随后输出，返回给前端命令历史。
  - allowlist 的 stdin 从 `info\nquit\n` 改为单行 `info`、`invitecode`、`settings show` 等，彻底移除错误的 `quit`。
  - `SendSay` 暂时返回 `command_not_supported`，因为真实 `help` 列表没有喊话命令。
- `backend/internal/games/stardew_junimo/console_test.go`
  - fake Docker 改用 `ComposeExecPipe`。
  - 单测改为验证命令写入 `/tmp/smapi-input`，不再保护 `attach-cli + quit` 的错误行为。
  - 喊话测试改为验证合法输入返回 `command_not_supported`，空消息、过长消息、server_not_running 仍然按原规则返回。
- `backend/internal/web/lifecycle_handlers.go`
  - `command_not_supported` 映射为 501。

### 真实联调结论

在本机容器中验证：

```powershell
cd E:\stardew-server-anxi-panel\data\instances\stardew
'info' | docker compose exec -T server tee -a /tmp/smapi-input
docker exec -i stardew-server-1 sh -lc 'tail -n 80 /tmp/server-output.log'
```

日志会出现 `--- Server Info ---`、服务器名、版本、玩家数、邀请码等信息。`invitecode` 同样能通过 FIFO 触发并在日志中输出。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
# 通过

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 通过
```

### 后续注意事项

- `backend/internal/docker/compose_tty*.go` 目前仍留在代码里，但 M11 控制台命令不再依赖它。后续如果没有其他功能需要真实 TTY，可以考虑删除或只保留给 Steam Auth 类交互使用。
- 服务器喊话需要确认 Junimo/SMAPI 是否存在真实广播命令；当前不能继续使用 `say` 这个假设。
- 命令输出现在来自 `/tmp/server-output.log` 的增量尾部，若未来命令输出很慢或日志噪音很大，可以改成更明确的 command marker 或 Junimo HTTP API。

## Milestone 12: Packaging

### 目标

让项目可以构建为一个可交付 Docker 镜像。用户只需 Docker Engine + Compose V2，运行一个 panel 容器即可访问 Web 面板。

### 改了什么

**新增文件：**

| 文件 | 说明 |
|------|------|
| `Dockerfile` | 多阶段构建（frontend-builder → backend-builder → runtime） |
| `.dockerignore` | 排除 .git、node_modules、dist、data 等 |
| `backend/internal/static/static.go` | `//go:embed frontend_dist/*` 嵌入前端产物 |
| `backend/internal/static/frontend_dist/.gitkeep` | 占位文件，本地 go build 不报错 |
| `deploy/docker-compose.yml` | 部署示例 |
| `docs/deployment.md` | 完整部署指南 |

**修改文件：**

| 文件 | 改动 |
|------|------|
| `backend/internal/web/handler.go` | 新增 `serveStatic`（SPA fallback）；`isSetupAllowed` 白名单扩展 |
| `README.md` | 新增 Docker 部署章节、更新仓库结构和状态 |
| `docs/handoff-roadmap.md` | 标记 M12 完成 |

### 关键设计

1. **前端嵌入 Go binary**：Dockerfile 在 backend-builder 阶段将 frontend/dist 复制到 `internal/static/frontend_dist/`，`//go:embed` 将其编译进二进制。运行时只有一个文件 + `/data` 目录。

2. **SPA fallback**：`serveStatic` 先用 `fs.ReadFile` 查找嵌入文件，找不到则返回 `index.html`。前端路由正常工作。

3. **setup 白名单**：`isSetupAllowed` 新增 `/`、`/assets/*`、`/favicon.ico`、`/index.html`。否则未初始化管理员时前端无法加载。

4. **CGO_ENABLED=0**：`modernc.org/sqlite` 是纯 Go，构建静态二进制运行在 Alpine 上。

5. **runtime 包含**：docker-cli、docker-cli-compose、ca-certificates、tzdata。

### 影响接口

- `GET /` — 现在返回前端 index.html（之前返回 404）
- `GET /assets/*` — 返回前端静态资源
- 其他非 `/api/*`、`/health` 路径 — 返回 index.html（SPA fallback）

### 验证

```powershell
# 后端
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
# 全部通过

# 前端
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 通过

# 镜像构建（需 Docker 网络正常）
cd E:\stardew-server-anxi-panel
docker build -t stardew-server-anxi-panel:local .
```

### 已知限制

- Docker Socket 挂载等同高权限，仅限内网。
- Windows Docker Desktop socket 通过 WSL2 转发。
- 当前 Docker 镜像源 `vfonjwaa.mirror.aliyuncs.com` 返回 403，需检查 Docker Desktop 设置。
- 如果需要 ARM 支持，需用 `docker buildx` 或参数化 GOARCH。

### 后续注意事项

- Milestone 13 (Hardening) 可补充审计日志、备份恢复。
- 前端如需 favicon.ico，在 `frontend/public/` 放置即可。
- 本地开发仍使用 `npm run dev`（Vite）+ `go run ./cmd/panel`，不依赖嵌入的静态文件。

## Milestone 13: Hardening（2026-06-27）

### 目标

把 MVP 从"能用"提高到"可交付"：操作审计、日志脱敏、权限加固、备份恢复、健康检查、前端错误体验。

### 改了什么

**1. 操作审计 Audit Log**

新增 `GET /api/audit-logs`（admin-only，分页），存储层新增 `ListAuditLogs` 关联 users 表。

以下操作已添加审计记录（`s.auditLog(r, &actor, action, targetType, targetID, metadata)`）：

| Action | Handler | 触发时机 |
|--------|---------|----------|
| `instance_prepare` | install_handlers.go | 准备实例目录成功 |
| `instance_install` | install_handlers.go | 安装任务启动成功 |
| `instance_start` | lifecycle_handlers.go | 服务器启动成功 |
| `instance_stop` | lifecycle_handlers.go | 服务器停止成功 |
| `instance_restart` | lifecycle_handlers.go | 服务器重启成功 |
| `save_new_game` | lifecycle_handlers.go | 自定义新建存档并启动 |
| `save_upload_start` | lifecycle_handlers.go | 上传存档并启动 |
| `save_select` | lifecycle_handlers.go | 选择存档 |
| `save_delete` | lifecycle_handlers.go | 删除存档（含备份信息） |
| `save_restore` | lifecycle_handlers.go | 从备份恢复存档 |
| `mod_upload` | lifecycle_handlers.go | 上传 Mod |
| `mod_delete` | lifecycle_handlers.go | 删除 Mod |
| `command_run` | lifecycle_handlers.go | 执行控制台命令 |

已有审计（setup_admin_created、auth_login、auth_logout、user_*）保持不变。

前端：高级设置内新增「操作审计」区域，显示时间、操作、操作者、目标。

**2. 日志脱敏**

扩展 `docker/redact.go`：
- 新增 pattern：`session`、`cookie`、`authorization`、`api_key`、`apikey`
- 新增：Bearer token 脱敏（`Authorization: Bearer eyJhb...` → `Authorization: Bearer [REDACTED]`）
- 新增：邀请码脱敏（`invite code: ABCD1234` → `invite code=[REDACTED]`）
- 新增：`--env` flag 脱敏（`--env SECRET_KEY=abc123` → `--env [REDACTED]`）
- `Redacted` 常量导出

新增 `sanitizeError(err, fallback)` / `sanitizeErrorMsg(err, prefix)`：
- 自动识别数据库/Docker/文件系统/网络/ZIP 错误，返回中文安全消息
- 通用 fallback 经过 `RedactString` 处理

所有 handler 的 `writeError` 调用已从 `"前缀: "+err.Error()` 替换为 `sanitizeErrorMsg(err, "前缀")`。

新增 10 个脱敏单元测试（session/cookie/auth/bearer/invite/env/false positives）。

**3. 权限加固**

新增测试文件 `audit_test.go`：
- `TestPermissionHardening_AdminOnlyEndpoints`：12 个端点 × 2 场景（无认证 401、非管理员 403）
- `TestPermissionHardening_AuthEndpoints`：6 个只读端点，非管理员可访问
- `TestAuditLogsAPI_Permissions`：审计日志 API 权限验证
- `TestAuditLogsAPI_ContainsSetupLog`：初始化操作被记录

**4. 备份与恢复**

`saves.go` 新增：
- `BackupSave(dataDir, saveName)` — 创建 ZIP 备份到 `.local-container/backups/saves/`
- `DeleteSaveWithBackup(dataDir, saveName)` — 删除前自动备份（备份失败不阻止删除）
- `ListBackups(dataDir)` — 列出备份
- `RestoreBackup(dataDir, backupName, overwrite)` — 恢复，支持冲突检测

`handleSaveDelete` 改用 `DeleteSaveWithBackup`。

新增 API：
- `GET /api/instances/:id/saves/backups` — 备份列表（admin-only）
- `POST /api/instances/:id/saves/backups/restore` — 恢复备份（admin-only，运行中禁止）

新增 12 个备份/恢复测试。

**5. 健康检查增强**

新增 `GET /api/health/diagnostics`（需认证），返回结构化诊断：

```json
{
  "status": "ok|warning|error",
  "checks": [
    {"name": "docker_daemon", "status": "ok", "message": "Docker 服务正常"},
    {"name": "docker_compose", "status": "ok", "message": "Docker Compose 可用"},
    {"name": "data_dir", "status": "ok", "message": "数据目录可写"},
    {"name": "instance_dir", "status": "ok", "message": "实例目录已就绪"},
    {"name": "compose_file", "status": "ok", "message": "docker-compose.yml 已就绪"},
    {"name": "active_save", "status": "warning", "message": "没有已选择的启动存档"}
  ]
}
```

前端：高级设置内新增「健康检查」区域，点击「开始检查」显示诊断结果。

**6. 前端错误体验**

`helpers.ts` 新增 `errorCodeMap`（40+ 后端错误码 → 中文消息），`errorMessage()` 优先使用 code 映射。

后端 handler 不再将 `err.Error()` 直接暴露给前端。

### 影响接口

```text
GET  /api/audit-logs                    — 新增（admin-only）
GET  /api/health/diagnostics            — 新增（需认证）
GET  /api/instances/:id/saves/backups   — 新增（admin-only）
POST /api/instances/:id/saves/backups/restore — 新增（admin-only）
DELETE /api/instances/:id/saves/:name   — 行为变更（删除前自动备份）
```

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

### 后续注意事项

- `GameDriver` 接口的 `SelectSave`/`DeleteSave` 缺少 `instance` 参数，handler 直接调用 `sj` 包函数。后续可改接口签名。
- 联机角色槽异常（重复新农夫入口）需后续专门 Milestone 处理，当前只做诊断和备份保护。
- Unix 平台的 `ComposeExecTTY` 实现未完成。
- 实时日志流（SSE/WebSocket tail logs）未实现。
- 审计日志目前只有查询 API，没有清理/归档机制。如果操作频繁，后续可加定期清理。

## Milestone 14: Release Candidate（2026-06-27）

### 目标

把项目从"功能完成"推进到"可发布候选版本"。不新增大功能，不重构架构，重点是版本信息、支持包导出、冒烟测试脚本、发布检查清单和文档收口。

### 改了什么

**1. 版本信息**

- `backend/internal/config/config.go`：新增 `Commit`、`BuildDate` 字段；新增 `buildVersion`/`buildCommit`/`buildDate` ldflags 变量，构建时通过 `-ldflags -X` 注入。
- `backend/internal/web/handler.go`：`healthResponse` 新增 `commit`/`buildDate` 字段；新增 `GET /api/version` 端点；`/api/version` 加入 setup 白名单。
- `Dockerfile`：backend-builder 阶段新增 `ARG VERSION/COMMIT/BUILD_DATE`，通过 `-ldflags -X` 注入；runtime 阶段新增 OCI labels。
- `deploy/docker-compose.yml`：文档注释说明如何构建带版本号镜像。
- 前端 `api.ts`：新增 `getVersion()` 函数和 `VersionInfo` 类型。
- 前端 `App.tsx`：启动时加载版本信息，页面顶部显示 `v{version} · {commit} · {buildDate}`。
- 前端 `App.css`：新增 `.version-info` 样式（等宽字体、小字号）。

**2. 支持包导出**

- `backend/internal/web/support_bundle.go`：**新建** — `handleSupportBundle` 收集诊断信息并打包为 ZIP。
- ZIP 包含：version.json、health.json、instance-state.json、jobs.json、audit-logs.json、compose-ps.json、docker-compose.yml、server-logs.txt。
- 所有日志和配置内容经过 `RedactString` 脱敏。
- 只有 admin 可以导出。
- `backend/internal/web/instance_handlers.go`：注册 `POST /api/instances/:id/support-bundle` 路由。
- 前端 `api.ts`：新增 `downloadSupportBundle()` 函数。
- 前端 `App.tsx`：健康检查区域新增「导出诊断包」按钮。
- 前端 `App.css`：新增 `.section-heading-actions` 样式（并排按钮）。

**3. 冒烟测试脚本**

- `scripts/smoke-test.ps1`：**新建** — Windows PowerShell 冒烟测试脚本。
- 检查后端测试（`go test ./...`）、前端构建（`npm run build`）、Docker 镜像构建。
- 可选：启动临时容器检查 `/health` 和 `/api/version` 返回 200。
- 脚本失败时有清晰中文错误。
- 不碰用户真实数据，只清理自己创建的临时容器、镜像和 volume。
- 支持 `-SkipDocker`、`-SkipFrontend`、`-SkipBackend` 参数。

**4. 发布检查清单**

- `docs/release-checklist.md`：**新建** — 17 个验收维度的逐项检查清单。
- 覆盖：构建验证、clean install、existing data upgrade、Docker compose 部署、管理员初始化、登录/登出、游戏安装、存档管理、Mod 管理、控制台命令、审计日志、健康检查、权限、脱敏、版本信息、支持包导出、前端体验。
- 记录已知问题（联机角色槽异常、控制台通信方式）。
- 说明如何运行 smoke test 和构建带版本号镜像。

### 新增文件

| 文件 | 说明 |
|------|------|
| `backend/internal/web/support_bundle.go` | 支持包导出 handler |
| `scripts/smoke-test.ps1` | Windows 冒烟测试脚本 |
| `docs/release-checklist.md` | 发布检查清单 |

### 修改文件

| 文件 | 改动 |
|------|------|
| `backend/internal/config/config.go` | 新增 Commit/BuildDate 字段和 ldflags 变量 |
| `backend/internal/web/handler.go` | health 新增 commit/buildDate；新增 /api/version；setup 白名单 |
| `backend/internal/web/instance_handlers.go` | 注册 support-bundle 路由 |
| `Dockerfile` | 新增 ARG/ldflags/OCI labels |
| `deploy/docker-compose.yml` | 文档注释更新 |
| `frontend/src/api.ts` | 新增 getVersion/downloadSupportBundle |
| `frontend/src/App.tsx` | 版本显示、诊断包按钮 |
| `frontend/src/App.css` | version-info/section-heading-actions 样式 |

### 影响接口

```text
GET  /api/version                       — 新增（无需认证）
POST /api/instances/:id/support-bundle  — 新增（admin-only）
GET  /health                            — 变更（新增 commit/buildDate 字段）
```

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
# 通过

# 冒烟测试
cd E:\stardew-server-anxi-panel
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-test.ps1
```

### 仍没有解决的已知问题

- 联机角色槽异常（重复新农夫入口）：已记录在发布检查清单中，需要排查 JunimoServer/Stardew 联机层面行为。
- Unix 平台的 `ComposeExecTTY` 实现未完成。
- 实时日志流（SSE/WebSocket tail logs）未实现。
- 审计日志没有清理/归档机制。
