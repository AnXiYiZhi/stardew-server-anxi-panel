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
