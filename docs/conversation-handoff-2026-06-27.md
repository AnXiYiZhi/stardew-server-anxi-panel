# Conversation Handoff 2026-06-27

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
