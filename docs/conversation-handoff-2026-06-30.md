# Conversation Handoff 2026-06-30

## UI-PROT-1: Stardew 主面板页面级 image2 原型
### 目标

用户要求根据项目目标和现有前端页面重新优化规划项目原型图，不需要 HTML，要求“漂亮为主、不维护感、很符合星露谷主题感”，并且所有页面的左侧栏和顶部总栏统一，只有页面内容不同。本轮生成登录后 Stardew 主面板 9 个页面的独立静态原型图。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/*.png` | 新增 9 张 image2 页面原型图：总览、服务器、存档、任务日志、玩家、模组、诊断、安装、设置。 |
| `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/README.md` | 新增页面清单、统一视觉约束和验证记录。 |
| `docs/handoff-roadmap.md` | 新增 `UI-PROT-1` 已完成条目。 |

### 影响接口/文件

- 不改前端 React 代码、不改后端 API、不生成 HTML。
- 原型图对应现有 `StardewRoute` 9 个登录后主面板页面：
  - `/instances/stardew/overview`
  - `/instances/stardew/server`
  - `/instances/stardew/saves`
  - `/instances/stardew/jobs`
  - `/instances/stardew/players`
  - `/instances/stardew/mods`
  - `/instances/stardew/diagnostics`
  - `/instances/stardew/install`
  - `/instances/stardew/settings`
- 登录/初始化页本轮未生成，因为它们没有用户要求统一的左侧栏和顶部总栏。

### 如何验证

```powershell
Get-ChildItem E:\stardew-server-anxi-panel\docs\prototypes\stardew-page-prototypes-image2-2026-06-30 -Filter *.png

Add-Type -AssemblyName System.Drawing
Get-ChildItem E:\stardew-server-anxi-panel\docs\prototypes\stardew-page-prototypes-image2-2026-06-30 -Filter *.png |
  Sort-Object Name |
  ForEach-Object {
    $img=[System.Drawing.Image]::FromFile($_.FullName)
    [pscustomobject]@{Name=$_.Name; Width=$img.Width; Height=$img.Height}
    $img.Dispose()
  }
```

验证结果：9 张图均存在，尺寸均为 `1672 x 941`。已人工查看 `01-overview.png` 和 `09-settings.png`，页面非空、外壳统一、主内容符合对应页面功能。

### 下一步注意事项

- 这些图是视觉原型，不是可交互实现；后续落地仍应基于 `frontend/src/games/stardew` 现有组件拆分。
- image2 生成图中的细小中文可能存在不稳定，正式实现时以现有真实中文文案为准。
- 后续 UI 重构可优先抽象统一外壳：左侧木导航、顶部状态栏、右侧 OpsRail、羊皮纸内容面板、像素按钮和状态灯。
- 示例数据只服务视觉表达，不应倒推接口字段或后端契约。

## UI-R21: 设置页新增面板端口与 VNC 端口区域

### 目标

用户要求在设置页增加一个区域，用来显示面板端口和 VNC 端口。目标是让用户能在“设置与审计”里直接看到当前面板访问端口，并能查看/修改 Stardew 实例的 VNC 端口。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/SettingsPage.tsx` | 新增 `PortSection`：面板端口从 `window.location` 推导；VNC 端口通过 `getInstanceVNCConfig()` 读取，管理员可修改并调用 `updateInstanceVNCPort()` 保存；普通用户显示锁定提示。 |
| `frontend/src/games/stardew/pages/SettingsPage.tsx` | 新增端口号前端校验，非法输入提示“VNC 端口必须是 1 到 65535 之间的数字”；审计日志中文映射增加 `instance_vnc_port_update`。 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增 `.sd-settings-port-*` 样式；桌面端两列显示，移动端单列显示，输入框和按钮在 390px 宽度下不溢出。 |
| `backend/internal/web/ports_handlers.go` | `readInstanceVNCPort` 在 `.env` 缺失时返回默认 `5800`；保存 VNC 端口前 `MkdirAll(instance.DataDir)`，避免新实例目录尚未创建时写入失败。 |
| `backend/internal/web/docker_handlers_test.go` | 新增 `TestInstanceVNCConfigReturnsDefaultWhenEnvMissing` 和 `TestInstanceVNCConfigUpdatesPort`。 |

### 影响接口/文件

- 复用既有接口：`GET /api/instances/:id/config/vnc-port` 和 `PUT /api/instances/:id/config/vnc-port`，仍为 admin-only。
- 本轮没有新增面板端口后端接口；面板端口显示的是当前浏览器访问端口，适配 Vite、容器端口映射和反向代理访问。
- 保存 VNC 端口只修改实例 `.env`，不会立即重建或重启容器；需要后续重启服务器才会应用 Docker Compose 映射。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
go test ./internal/web -run "TestInstanceVNCConfig|TestDocker|TestInstanceMetrics"
go test ./...

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

浏览器验证：
- `http://127.0.0.1:5174/instances/stardew/settings` 可打开，标题为 `Stardew Anxi Panel`。
- 设置页出现“端口信息”，显示面板端口 `5174`，VNC 端口显示当前配置 `18600`。
- 输入非法 VNC 端口 `abc` 并点保存，会显示“VNC 端口必须是 1 到 65535 之间的数字”，不会提交保存。
- 390px 移动端端口区为单列，`bodyScrollWidth=375`、`viewportWidth=390`，无横向溢出。
- Browser console `error/warn` 为空。

### 下一步注意事项

- 如果未来需要显示容器内监听端口或 `PANEL_ADDR`，需要新增后端配置读取接口；当前“面板端口”是用户实际访问入口，更适合在 UI 上解释“我现在从哪个端口打开面板”。
- VNC 端口保存后仍需重启服务器；这点已在保存成功提示里说明。

## UI-R20: 浏览器标签页图标替换为 Anxi Panel 字标

### 目标

用户要求把打开面板网页时展示的默认 Vue/Vite 图标替换为刚生成的 `anxi panel` 小图标，并希望图标方向保持“饥荒那种风格”。本轮使用最新 imagegen 生成图作为来源，接入项目 favicon。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/favicon.png` | 新增 512x512 PNG favicon，保留 `anxi panel` 两个词的旧纸手绘黑色线稿风格。 |
| `frontend/index.html` | 在 `<head>` 中新增 `rel="icon"` 和 `apple-touch-icon`，均指向 `/favicon.png`。 |
| `frontend/dist/favicon.png` | `npm.cmd run build` 后由 Vite 从 public 同步到生产产物。 |
| `frontend/dist/index.html` | 构建产物中已包含 `/favicon.png` 引用。 |

### 影响接口/文件

- 不影响后端 API、认证、路由和业务逻辑。
- 只影响浏览器标签页图标、移动端添加到主屏幕时的触摸图标，以及生产静态产物。
- 生成图原文件仍保留在 `C:\Users\anxi\.codex\generated_images\019f17d5-a08e-79d2-8369-de1d1be57e59\`，项目使用的是复制缩放后的 `frontend/public/favicon.png`。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0
```

补充检查：
- `frontend/dist/index.html` 含有 `<link rel="icon" type="image/png" href="/favicon.png" />`。
- `frontend/dist/favicon.png` 与 `frontend/public/favicon.png` 文件大小一致。

### 下一步注意事项

- 浏览器 favicon 缓存很顽固，如果页面仍显示旧图标，先强刷新或清理站点数据。
- 如果将来需要进一步减小体积，可以只压缩 `frontend/public/favicon.png`，再重新 build 同步到 `dist`。

## DIAG-1: 诊断页资源趋势接入
### 目标

用户要求把诊断页“资源趋势”的待接入占位改成真实可用的图表，并且视觉上要做成好看的线型图和圆圈型占用率。目标是在不造假数据的前提下接入后端资源指标：CPU/内存来自 Junimo Compose 容器 stats，磁盘来自实例目录所在磁盘，并在前端展示最近趋势。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `backend/internal/docker/types.go` | 新增 `ComposeStatsResult`、`ComposeServiceStats`、`Timeouts.Stats`。 |
| `backend/internal/docker/compose.go` | 新增 `ComposeStats()`，执行 `docker compose stats --no-stream --format json`；解析 CPU%、内存%、`MemUsage` 字节数，兼容 JSON、单对象和 JSONL。 |
| `backend/internal/docker/compose_test.go` | 扩展 Docker 命令测试，覆盖 stats 命令参数和解析结果。 |
| `backend/internal/web/resource_metrics.go` | 新增资源指标 handler：优先选 `server` 服务；找不到明确 server stats 时不再 fallback 到第一个容器，容器未运行时返回磁盘数据和提示，不伪造 CPU/内存；CPU 保留 Docker 原始百分比，内存/磁盘占用率仍限制在 0-100。 |
| `backend/internal/web/disk_usage_unix.go` / `backend/internal/web/disk_usage_windows.go` | 新增跨平台磁盘用量读取。 |
| `backend/internal/web/handler.go` | `DockerService` 增加 `ComposeStats()`。 |
| `backend/internal/web/instance_handlers.go` | 接入 `GET /api/instances/:id/metrics`。 |
| `backend/internal/web/docker_handlers_test.go` | fake Docker 增加 stats；新增 `TestInstanceMetricsReturnsStatsAndDisk`，并补充非 server stats 不应冒充 server running 的回归测试。 |
| `frontend/src/types.ts` | 新增 `ResourceMetricSample` / `ResourceMetricsResponse`。 |
| `frontend/src/api.ts` | 新增 `getInstanceMetrics()`。 |
| `frontend/src/games/stardew/pages/DiagnosticsPage.tsx` | “资源趋势”区改为轮询 metrics API，保存最近 24 个样本，渲染 CPU/内存/磁盘环形卡片和 SVG 折线图；轮询改为请求完成后再等待 5 秒，避免慢 stats 请求重叠；CPU 超过 100% 时趋势图自动抬高纵轴。 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增资源面板、环形占用率、趋势图、图例、提示条和移动端单列样式。 |

### 影响接口/文件

- 新接口：`GET /api/instances/:id/metrics`，登录用户可读。
- 返回 `sample.cpuPercent` / `memoryPercent` / `diskPercent` 均为百分比；CPU 可超过 100% 以表达多核占用，内存/磁盘是 0-100 的占用率；CPU/内存在容器未运行时为 `null`，磁盘通常仍可返回。
- 只有明确匹配 `server` 服务或容器名包含 `server` 的 stats 才会被当成服务器资源；`steam-auth`、`discord-bot` 等非 server stats 不会触发 `containerRunning=true`。
- 前端折线历史只保存在当前诊断页组件内，刷新后重新累计。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./internal/docker
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./internal/web -run "TestDocker|TestInstanceMetrics"
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./...

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

浏览器验证记录：
- `http://127.0.0.1:5174/instances/stardew/diagnostics` 可加载 Vite 应用；使用本地账号登录后，诊断页真实渲染成功，页面无 framework overlay、无 console error。
- 桌面端可见三张圆圈占用率卡片和下方“实时趋势”折线图区域。
- 390px 移动端验证：`bodyScrollWidth=375`、`viewportWidth=390`，无横向溢出；三张圆圈卡片纵向单列排列，趋势图存在。
- 当前 Vite 代理到的 8090 后端进程仍是旧版本，`/api/instances/stardew/metrics` 返回 `resource not found`，所以图表提示条显示该错误；重启/替换为本轮后端后该接口才会返回真实指标。

### 下一步注意事项

- `docker compose stats --format json` 在不同 Compose 版本字段名可能有差异；当前解析已覆盖常见 `CPUPerc`、`MemPerc`、`MemUsage`，如遇新字段名，在 `parseComposeStats` 增加 alias 即可。
- 前端 metrics 轮询是“请求完成后再等待 5 秒”，不要改回固定 `setInterval`，否则 Docker stats 慢于轮询间隔时会堆叠请求。
- 如果将来需要刷新后仍保留历史曲线，可把样本缓存放到 dashboard 数据层或后端短期内存缓存。
- 如果要更贴近“容器内磁盘”，可再用 Junimo `server` 容器内 `df` 采样；本轮磁盘占用表示实例目录所在磁盘，服务器停机时也可用。

## JOBS-1: 任务日志中心支持清空错误日志
### 目标

用户要求增加“清空错误日志”按钮。目标是在不删除任务历史的前提下，让管理员可以清理任务日志里的错误内容：删除 error 级别日志行，并清空任务详情顶部显示的 `errorMessage`。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `backend/internal/storage/jobs.go` | 新增 `ClearJobErrorLogs()`，在事务里删除 `job_logs.level = error` 的日志行，并把 `jobs.error_message` 置空；返回 `deleted` 和 `messagesCleared`。 |
| `backend/internal/jobs/manager.go` | 新增 `ClearErrorLogs()`，供 Web handler 调用。 |
| `backend/internal/web/jobs_handlers.go` | 新增 `handleClearJobErrorLogs`，仅管理员可调用；成功后写审计日志 `job_error_logs_cleared`。 |
| `backend/internal/web/handler.go` | 新增路由 `DELETE /api/jobs/error-logs`。 |
| `backend/internal/web/jobs_handlers_test.go` | 新增回归测试：普通用户 403；管理员清空后 job detail 的 `errorMessage` 为 null，日志列表里不再有 `level=error` 行。 |
| `frontend/src/api.ts` | 新增 `clearJobErrorLogs()`。 |
| `frontend/src/games/stardew/pages/JobsLogsPage.tsx` | 工具栏新增“清空错误日志”按钮；点击后弹确认框；清空后刷新任务列表、当前任务详情和日志窗口。 |

### 影响接口/文件

- 新接口：`DELETE /api/jobs/error-logs`，admin-only。
- 返回：`{ ok: true, deleted: number, messagesCleared: number }`。
- 不删除 `jobs` 记录，不改变任务 `status`，只删除 error 日志行和清空任务错误详情文本。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./internal/web -run "TestJobs|TestAdminCanClearJob"
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./...

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

### 下一步注意事项

- 清空错误日志后失败任务仍显示 failed，这是预期行为；该按钮只清错误文本，不修正历史任务状态。
- 如果未来想做“只清当前任务错误日志”，需要新增带 `job_id` 条件的后端接口，并复用 `loadReadableJob` 做权限判断。

## AUTH-1: 首个管理员拥有隐藏超级管理员权限
### 目标

第一个初始化/注册的管理员应拥有后台最高管理权限；普通管理员仍然叫“管理员”，但只能管理普通用户。只有首个管理员可以创建管理员、把普通用户升为管理员、把管理员降级为普通用户，以及禁用/彻底删除管理员账号。界面不要展示“超级管理员”这个新角色名，统一仍显示“管理员”。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `backend/migrations/005_super_admin.sql` | 新增 `users.is_super_admin` 隐藏能力位；旧库升级时把最早的管理员回填为超级管理员，并增加 `idx_users_super_admin_active`。 |
| `backend/internal/auth/types.go` | `PublicUser` 增加 `isSuperAdmin`，用于登录态和前端权限判断。 |
| `backend/internal/storage/auth.go` | 首个管理员创建时写入 `IsSuperAdmin=true`；所有用户查询、会话查询、创建/更新返回值补齐该字段；新增 `ErrSuperAdminRequired` 和 `ErrLastSuperAdmin`；普通管理员不能修改管理员账号或执行任何角色升降级；禁止移除最后一个启用的超级管理员。 |
| `backend/internal/web/users_handlers.go` | 创建管理员前显式检查当前会话是否 `IsSuperAdmin`；用户管理错误码新增 `super_admin_required` / `last_super_admin`。 |
| `backend/internal/web/auth_handlers_test.go` | 覆盖首次管理员 `isSuperAdmin=true`、普通管理员越权失败、普通管理员仍能禁用普通用户、超级管理员可创建/升降管理员。 |
| `frontend/src/types.ts` | `CurrentUser` 增加 `isSuperAdmin`，`PanelUser` 继承该字段。 |
| `frontend/src/games/stardew/pages/SettingsPage.tsx` | 普通管理员创建用户时只显示“普通用户”；升降级按钮仅超级管理员可见；禁用/删除管理员账号对普通管理员禁用；普通用户管理保持可用。 |
| `frontend/src/games/stardew/StardewPanel.tsx` | 顶部账号角色显示中文“管理员/普通用户”，超级管理员不展示额外名称。 |
| `frontend/src/core/helpers.ts` | 增加新权限错误码中文文案。 |

### 影响接口/文件

- `/api/setup/admin`、`/api/auth/login`、`/api/auth/me`、`GET /api/users`、用户创建/更新响应都会带 `isSuperAdmin`。
- `POST /api/users`：普通管理员创建 `role=admin` 返回 403 `super_admin_required`。
- `PATCH /api/users/:id`：普通管理员执行任何角色变更，或修改管理员账号，返回 403 `super_admin_required`。
- `DELETE /api/users/:id`：普通管理员禁用/彻底删除管理员账号返回 403 `super_admin_required`。
- 角色名称协议不变，仍只有 `admin` / `user`；`is_super_admin` 不作为新 role。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./internal/web -run "TestSetupLoginLogoutAndUserPermissions|TestLastAdminCannotBeDisabledOrDowngraded|TestAdminCanEnableAndHardDeleteUser|TestSuperAdminControlsAdminRoleManagement"
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./internal/storage ./internal/auth
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./...

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

### 下一步注意事项

- UI 不要新增“超级管理员”标签或角色选项；用户看到的管理员名称保持一致。
- 后续若补“修改密码/启用用户/批量操作”等入口，需要继续按同一规则：普通管理员只能操作普通用户，管理员账号和角色升降级必须 `isSuperAdmin=true`。
- 迁移只自动指定最早管理员为超级管理员；若用户已有多个管理员，其他管理员升级后会成为普通管理员。

## MVP-UX-9: 备份条目支持彻底删除
### 目标

用户要求“备份与恢复”的每一个备份存档也要有“彻底删除”按钮。目标是让管理员能清理不再需要的备份 ZIP，同时避免误删正式存档：删除动作只作用于备份目录，不需要停服，但必须二次确认不可撤销。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `backend/internal/games/stardew_junimo/saves.go` | 新增 `validateBackupName()`，统一校验备份文件名：拒绝空名、路径分隔符、冒号、`..` 和非 `.zip`。 |
| `backend/internal/games/stardew_junimo/saves.go` | 新增 `DeleteBackup(dataDir, backupName)`，只删除 `<dataDir>/.local-container/backups/saves/<backupName>` 下的单个 ZIP 文件。`RestoreBackup` 也改为复用同一校验。 |
| `backend/internal/games/stardew_junimo/saves_test.go` | 新增删除备份成功和非法文件名测试。 |
| `backend/internal/web/instance_handlers.go` | 新增 `DELETE /api/instances/:id/saves/backups/:backupName` 路由。 |
| `backend/internal/web/lifecycle_handlers.go` | 新增 `handleSavesBackupDelete`，admin-only，删除成功后记录审计日志 `save_backup_delete`。 |
| `frontend/src/api.ts` | 新增 `deleteSaveBackup()`。 |
| `frontend/src/core/helpers.ts` | 新增 `delete_backup_failed`、`backup_not_found`、`invalid_backup_name` 错误文案。 |
| `frontend/src/games/stardew/SavesSection.tsx` | 备份行增加“彻底删除”按钮；点击后弹出确认弹窗，说明只删除备份 ZIP、不删除当前存档、删除后无法恢复；确认后刷新备份列表。 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增 `.sd-save-backup-actions`，让“恢复/彻底删除”按钮保持面板式纵向动作区。 |

### 影响接口/文件

- 新增接口：`DELETE /api/instances/:id/saves/backups/:backupName`，admin-only。
- 删除备份不要求服务器停止，因为不触碰正式 `Saves/<存档>` 目录，只删除备份 ZIP。
- 删除备份不会生成新的备份，属于不可撤销清理操作。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./internal/games/stardew_junimo -run "TestBackup|TestListBackups|TestDeleteBackup|TestRestoreBackup"
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./internal/web -run "TestRunningProtection|TestSaveDelete"
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./...
# exit 0

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，CSS 92.11 kB，JS 337.99 kB
```

建议真实页面复测：
1. 生成至少一个备份，进入存档页“备份与恢复”。
2. 点击备份行“彻底删除”，确认弹窗应说明不可撤销。
3. 确认后备份行消失，刷新备份列表后仍不出现。
4. 服务器运行中也可以删除备份，但恢复按钮仍应禁用。

### 下一步注意

- 当前没有“批量删除备份”或“保留最近 N 个备份”的自动清理策略；如果用户后续备份很多，可以再加批量清理入口。
- URL 路由使用备份文件名作为最后一段，前端已 `encodeURIComponent`；后端仍会拒绝任何路径穿越或非 zip 文件名。

## MVP-UX-8: 备份列表展示存档详情
### 目标

用户反馈备份列表也应该展示存档信息，而不是只展示 ZIP 文件名和创建时间。目标是让“备份与恢复”区块能像普通存档卡一样显示农场、农民、游戏日期、地图等基础信息，帮助用户判断要恢复哪一个备份。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `backend/internal/games/stardew_junimo/saves.go` | 把普通存档 XML 解析逻辑抽成 `fillSaveInfoFromXML`；新增 `readWhichFarmFromData` 供 ZIP 内主存档内容解析地图。 |
| `backend/internal/games/stardew_junimo/saves.go` | `BackupInfo` 增加 `farmerName`、`farmName`、`gameYear`、`gameSeason`、`gameDay`、`farmType`、`fileSizeBytes`、`parseError`。 |
| `backend/internal/games/stardew_junimo/saves.go` | `ListBackups` 现在会打开每个备份 ZIP，校验条目并读取 `SaveGameInfo` / `SaveGameInfo.xml` / 主存档文件，best-effort 填充备份详情；单个备份解析失败只写入该行 `parseError`。 |
| `backend/internal/games/stardew_junimo/saves_test.go` | 扩展备份列表测试，验证备份返回农民、农场、年份季节日期和地图。 |
| `frontend/src/types.ts` | `BackupInfo` 类型补齐存档详情字段。 |
| `frontend/src/games/stardew/SavesSection.tsx` | 备份行展示农场、农民、年季日、地图、存档大小、备份 ZIP 大小；解析失败时显示“解析失败”。 |

### 影响接口/文件

- `GET /api/instances/:id/saves/backups` 返回字段向后兼容地增加存档详情字段。
- 前端“备份与恢复”区块不需要额外请求，刷新备份列表即可获得详情。
- 解析失败不会阻断列表，避免一个坏备份导致整个备份页不可用。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./internal/games/stardew_junimo -run "TestBackup|TestListBackups|TestRestoreBackup"
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./...
# exit 0

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，CSS 92.01 kB，JS 336.56 kB
```

建议真实页面复测：
1. 删除一个带完整 `SaveGameInfo` 的存档生成备份。
2. 进入存档页“备份与恢复”，确认该备份显示农场名、农民、游戏日期和地图。
3. 准备一个损坏备份或缺少 `SaveGameInfo` 的 ZIP 时，对应行显示解析失败，但备份列表整体仍可打开。

### 下一步注意

- 目前没有解析备份内玩家列表、小屋等高级信息；如后续要展示“已有玩家”，可以继续在后端 ZIP 解析阶段扩展字段。
- `ListBackups` 会打开每个备份 ZIP；如果将来备份数量很多，可以考虑分页或缓存解析结果。

## MVP-UX-7: 存档页接入备份与恢复区块
### 目标

用户看到前端删除确认提示“删除前会自动备份”，追问备份在哪里，并要求在“存档”页增加“备份与恢复”区块。目标是在不新造后端协议的前提下，接入已有备份列表/恢复接口，让用户能看到备份并恢复；同名存档存在时必须二次确认覆盖，且恢复在服务器运行中继续禁用。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/src/types.ts` | 新增 `BackupInfo`、`BackupsListResult`、`RestoreBackupResult`。 |
| `frontend/src/api.ts` | 新增 `getSaveBackups()`：`GET /api/instances/:id/saves/backups`；新增 `restoreSaveBackup()`：`POST /api/instances/:id/saves/backups/restore`。 |
| `frontend/src/games/stardew/SavesSection.tsx` | 新增备份状态、加载函数、恢复确认弹窗；在存档列表下方显示“备份与恢复”区块，列出备份名、原存档名、大小、创建时间和同名冲突标记。 |
| `frontend/src/games/stardew/SavesSection.tsx` | 恢复按钮在 `running` / `starting` 时禁用；同名存档存在时，普通“确认恢复”禁用，只显示可用的“覆盖恢复”，弹窗说明覆盖前会先备份当前存档。删除存档后同步刷新备份列表。 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增 `.sd-save-backups-*` 和 `.sd-confirm-dialog-wide` 样式，保持 Stardew 面板木框/羊皮纸/警告条风格；为 `#saves-section` 增加 grid gap。 |

### 影响接口/文件

- 后端 API 未新增，本轮复用已有：
  - `GET /api/instances/:id/saves/backups`
  - `POST /api/instances/:id/saves/backups/restore`
- 备份文件实际位于 `<instance.DataDir>/.local-container/backups/saves/`；默认 Docker 部署中是 `/data/instances/stardew/.local-container/backups/saves/`。
- 备份恢复仍由后端 `ensureInstanceNotRunning` 保护，运行中无法恢复。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，CSS 92.01 kB，JS 336.00 kB
```

建议真实页面复测：
1. 删除一个非当前启动存档，确认备份与恢复区块出现对应 ZIP。
2. 点击该备份“恢复”，如果同名存档不存在，应可直接确认恢复。
3. 如果同名存档存在，弹窗应说明“覆盖前会先备份当前存档”，并通过“覆盖恢复”执行。
4. 服务器运行中，备份列表仍可查看，但恢复按钮应禁用。

### 下一步注意

- 当前前端只做列表和恢复，没有“下载备份 ZIP”或“删除备份 ZIP”。如果用户后续需要清理空间，可以补后端 delete/download backup API 后再加入口。
- `saveName` 来自后端按备份文件名解析；如果未来备份文件名规则变化，前端无需修改，但后端 `parseBackupSaveName` 要保持兼容。

## MVP-UX-6: 运行中删除存档只保护当前启动存档
### 目标

用户要求调整删除存档逻辑：服务器运行或启动中时，只有当前启动/正在使用的存档应该受保护；其他存档不再被硬性禁止删除，只需要在前端弹出明确警告并由用户确认。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `backend/internal/web/lifecycle_handlers.go` | `handleSaveDelete` 不再调用通用 `ensureInstanceNotRunning`；改为 reconcile 后读取 `sj.GetActiveSaveName(instance.DataDir)`，仅当待删名称等于 active save 且实例为 `running` / `starting` 时返回 409 `active_save_running`。 |
| `backend/internal/web/saves_handlers_test.go` | 从运行中全局保护测试里移除 `delete-save`；新增运行中删除测试，验证 active save 被保护、non-active save 可删除且 active save 配置保持不变。 |
| `frontend/src/games/stardew/SavesSection.tsx` | 删除按钮拥有独立禁用逻辑：`busy || !isAdmin || (isRunning && isActive)`；确认弹窗中，运行中删除 active save 会禁用确认，运行中删除非 active save 会显示警告但允许确认。 |
| `frontend/src/core/helpers.ts` | 新增 `active_save_running` 错误码映射，避免后端保护触发时显示泛化错误。 |

### 影响接口/文件

- `DELETE /api/instances/:id/saves/:name` 行为变更：运行中删除非当前启动存档现在允许成功删除并自动备份；运行中删除当前启动存档返回 409 `active_save_running`。
- 其他存档写操作不变：创建、上传、切换、选择并启动仍然在 `running` / `starting` 时禁用或返回 409。
- Mod 删除/上传保护不变，仍要求先停止服务器。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./internal/web -run "TestRunningProtection|TestSaveDelete"
# exit 0

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，CSS 91.10 kB，JS 331.87 kB
```

建议真实页面复测：
1. 准备至少两个存档，选择其中一个作为启动存档并启动服务器。
2. 进入存档页，当前启动存档的删除按钮应不可用。
3. 另一个非当前存档的删除按钮应可用；点击后弹出运行中删除警告，确认后应删除成功并刷新列表。

### 下一步注意

- 当前保护依据是 Junimo gameloader config 的 `SaveNameToLoad`。如果未来增加“当前实际加载存档”的 Junimo API 读取能力，可考虑优先用运行时真实值，再回退到 gameloader 配置。
- 删除非当前存档仍会先自动备份；如果未来做备份中心入口，需要把这类运行中删除的备份也展示出来。

## MVP-UX-5: 启动任务完成后自动同步运行状态和邀请码
### 目标

用户反馈：服务器开启成功后，任务与日志中心已经输出邀请码，但总览页和服务器控制页仍然显示“服务器未运行，邀请码不可用”，需要手动刷新页面才更新。根因是启动接口只返回 jobId，页面立即刷新时服务仍在 `starting`；真正任务完成时没有全局 dashboard 订阅任务完成事件，且任务日志页自身的 finished 回调也没有刷新邀请码。

### 改了什么
| 文件/范围 | 修改 |
|------|------|
| `frontend/src/games/stardew/useStardewDashboardData.ts` | 引入 `createJobEventSource`，对 dashboard 数据中的 `queued/running` 任务建立全局 SSE 订阅。 |
| `frontend/src/games/stardew/useStardewDashboardData.ts` | 收到任意活跃任务 `finished` 后自动刷新 jobs、instance state、saves、mods、invite code，并在 1 秒后再补刷 state/invite code。 |
| `frontend/src/games/stardew/useStardewDashboardData.ts` | `refreshInviteCode()` 失败时清空旧邀请码；实例状态不是 `running` 时清空邀请码和错误状态，避免停服后残留旧邀请码。 |
| `frontend/src/games/stardew/pages/JobsLogsPage.tsx` | 任务详情页 SSE finished 回调同步调用 `refreshInviteCode()`，避免在任务日志页完成时只刷新状态不刷新邀请码。 |

### 影响接口/文件

- 后端 API 未变。
- 前端 dashboard 数据层现在会为活跃任务持有一个全局 `EventSource`，任务终态或组件卸载时关闭。
- 总览页、服务器控制页、玩家页等所有使用 `dashboardData.instanceState` / `dashboardData.inviteCode` 的页面都会受益。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0；CSS 91.10 kB，JS 331.09 kB
```

建议真实页面复测：
1. 在总览页或服务器控制页点击启动服务器。
2. 不手动刷新页面，不切到任务日志页也应自动订阅该启动任务。
3. 任务完成并输出邀请码后，顶部状态、总览页、服务器控制页应自动变为运行中，并显示邀请码。
4. 停止服务器后邀请码应清空，不再显示旧码。

### 下一步注意

- 如果后续任务列表做清空/取消，需要确认全局 `EventSource` cleanup 不留下已终态任务订阅。
- 如果未来要做全局任务 toast，可以复用本轮全局任务 finished 监听，但不要在每个页面重复创建同一个 job 的 SSE。

---

## MVP-UX-4: VNC 端口占用启动失败引导
### 目标

用户启动服务器时遇到 Docker Compose 报错：`ports are not available ... 0.0.0.0:5800 ... forbidden by its access permissions`。此前任务日志只显示 `docker compose up: docker command failed`，用户无法判断该改什么。本轮目标是把该错误类型固定为用户向流程：启动失败任务日志明确说明 VNC 端口被占用或被系统保留；任务日志详情上方出现“更换 VNC 端口”按钮；点击后打开统一 Stardew 面板风格弹窗，显示当前端口和要更改的端口号。

### 改了什么
| 文件/范围 | 修改 |
|------|------|
| `backend/internal/games/stardew_junimo/lifecycle.go` | `ComposeUp` 失败时识别端口绑定失败，读取 `.env` 的 `VNC_PORT`，写入 `VNC 端口 <port> 被占用或被系统保留，请更换 VNC 端口后重试。`，同时把实例 phase 设为 `vnc_port_unavailable`。 |
| `backend/internal/games/stardew_junimo/lifecycle_test.go` | 新增端口绑定失败识别测试，覆盖 Windows 系统保留端口和 `port is already allocated`。 |
| `backend/internal/web/ports_handlers.go` | 新增 VNC 端口配置 handler，支持读取/更新 `.env` 中 `VNC_PORT`，校验端口范围 `1-65535`。 |
| `backend/internal/web/instance_handlers.go` | 新增路由 `GET/PUT /api/instances/:id/config/vnc-port`。 |
| `frontend/src/types.ts` / `frontend/src/api.ts` | 新增 `InstanceVNCConfig` 和 `getInstanceVNCConfig()` / `updateInstanceVNCPort()`。 |
| `frontend/src/games/stardew/pages/JobsLogsPage.tsx` | 根据 job error/logs 检测 VNC 端口错误，显示“更换 VNC 端口”修复条；弹窗读取当前端口、预填下一端口、保存新端口。 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增 `.sd-jobs-vnc-fix`、`.sd-vnc-port-*` 样式，复用现有 Stardew 弹窗、按钮和输入框风格。 |

### 影响接口/文件

- 新接口：`GET /api/instances/:id/config/vnc-port`，返回 `{ "vncPort": "5800" }`。
- 新接口：`PUT /api/instances/:id/config/vnc-port`，body `{ "port": "5801" }`，返回更新后的 `{ "vncPort": "5801" }`。
- 新接口为 admin-only，会写审计日志 `instance_vnc_port_update`。
- 端口更新只修改实例目录 `.env`，不会立即重建容器；需要再次启动服务器生效。
- 后端启动失败 phase 新增 `vnc_port_unavailable`，目前前端主要通过任务日志/错误文案识别是否显示修复按钮。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'; go test ./...
# exit 0

cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0；CSS 91.10 kB，JS 330.39 kB
```

建议真实页面复测：
1. 保持/制造 VNC 端口 5800 被占用或被系统保留，再点击启动服务器。
2. 任务日志应出现 `VNC 端口 5800 被占用或被系统保留，请更换 VNC 端口后重试。`
3. 任务日志详情上方应出现“更换 VNC 端口”按钮。
4. 点击按钮后弹窗显示目前端口号和要更改的端口号，保存 5801 后 `.env` 中 `VNC_PORT=5801`。

### 下一步注意

- 旧失败任务只有泛化错误文案，不会触发按钮；需要下一次启动失败写入新日志后前端才会显示修复入口。
- 如果后续要支持 GAME_PORT、QUERY_PORT、API_PORT 等端口冲突，建议先扩展后端错误分类结构，再让前端根据明确 code/phase 显示对应按钮，避免把所有 Docker 启动失败都归因到 VNC。
- 端口变更弹窗当前不自动重新启动服务器，保持用户显式控制。

---

## UI-R19: 新建游戏界面主面板风格微调

### 目标

用户确认新建游戏界面的布局和素材引用已经满意，但仍与主面板整体风格略有违和。本轮目标是小范围收口视觉风格，并补齐用户向默认值与选项展示：名字默认 `host`，最喜欢的东西给默认值且可修改；左右切换按钮改为 image2 生成的 Stardew 风格位图；性别和宠物选择不再显示“男/女/猫 1/狗 1”这类文字；宠物图标固定居中在左右按钮之间。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/src/games/stardew/NewGameCreator.tsx` | `defaultConfig()` 中 `farmerName` 改为 `host`，`favoriteThing` 改为 `星露谷` |
| `frontend/src/games/stardew/NewGameCreator.tsx` | `Gender` / `PetPreference` 类型和数据移除可见 `label`；性别切换只显示性别 icon，宠物切换只显示宠物图标 |
| `frontend/src/games/stardew/NewGameCreator.tsx` | `ArrowButton` 不再渲染字符箭头，改为留空 span，由 CSS 使用 PNG 背景 |
| `frontend/src/games/stardew/NewGameCreator.css` | 面板底色改为主面板同源羊皮纸 tile + 木框色系，降低原先偏亮橙色的违和感 |
| `frontend/src/games/stardew/NewGameCreator.css` | `.ngc-arrow-left/right` 引用新 PNG；`.ngc-pet-line` 固定为 label / left / center / right 四列，`.ngc-pet-choice` 居中显示 |
| `frontend/public/assets/stardew/new-game/buttons/arrow-left.png` | 新增 image2 生成源图后本地去绿幕、裁切、缩放得到的 64x64 透明 PNG |
| `frontend/public/assets/stardew/new-game/buttons/arrow-right.png` | 新增 image2 生成源图后本地去绿幕、裁切、缩放得到的 64x64 透明 PNG |

### 影响接口/文件

- 后端 API、Junimo 通信、新建存档提交协议均未改。
- `NewGameConfig` 类型未改，只是默认值和前端展示方式调整。
- 新素材位于 `frontend/public/assets/stardew/new-game/buttons/`，不覆盖旧 `new-game` 已有素材。
- 使用了 image2 生成左右箭头按钮底稿，最终项目内只保留裁切后的两个透明 PNG；原始/中间图留在 `tmp/imagegen/` 方便本地复查。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 90.43 kB，JS 327.12 kB
```

补充验证：

1. Python/Pillow 检查 `arrow-left.png` 和 `arrow-right.png`：尺寸均为 `64x64`，RGBA，四角 alpha 均为 0。
2. 用 `http://localhost:5173` 的真实 CSS/资源渲染临时新建游戏预览：名字默认 `host`，最喜欢的东西默认 `星露谷`。
3. 临时预览正文中没有出现“男/女/猫 N/狗 N”文字；宠物图标在左右按钮之间居中。
4. 使用用户提供的 `anxi / 123456.` 登录真实面板，进入 `/instances/stardew/saves` 并打开“创建存档”弹窗；真实页面确认默认值、箭头 PNG、隐藏“男/女/猫 N/狗 N”文字和宠物居中均生效。

### 下一步注意

- 真实页面复测时密码包含末尾句号：`123456.`。
- 若继续微调风格，优先在 `NewGameCreator.css` 里改颜色/边框/间距，不要重排当前用户已满意的布局结构。

---

## MVP-UX-3: 无存档引导兼容 stopped + saves=0 状态

### 目标

用户截图反馈：页面顶部状态是“已停止”，存档指标卡显示 `0 / 暂无激活存档`，但“启动”按钮旁仍没有出现“当前没有存档，请点击此按钮去创建/上传存档。”和“创建/上传存档”按钮。上一轮只监听 `save_required` 状态或启动请求返回 `save_required`，没有覆盖当前截图里的 `stopped + saves.length === 0`。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | 新增 `noSavesDetected`，当 `dashboardData.saves` 已加载且 `saves.length === 0` 时也显示无存档引导 |
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | 引导显示条件现在是 `save_required` / 本次启动检测到 `save_required` / 存档列表为 0，且状态不是 `running` 或 `starting` |
| `frontend/src/games/stardew/pages/ServerControlPage.tsx` | 同步使用 `dashboardData.saves.saves.length === 0` 触发无存档引导，覆盖服务器控制页 |

### 影响接口/文件

- 后端 API 未改。
- 存档列表接口读取失败时不会显示该引导，避免误把读取失败当成无存档。
- 截图中的 `stopped + saves=0` 状态现在会直接显示提示和按钮，不需要先点击启动。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.96 kB，JS 327.17 kB
```

建议真实页面复测：

1. 进入截图同样状态：顶部“已停止”，存档卡 `0 / 暂无激活存档`。
2. 总览页“启动”按钮旁应出现无存档提示和“创建/上传存档”按钮。
3. 点击按钮进入存档页。
4. 上传或创建存档后，回到总览页，该提示应消失。

### 下一步注意

- 如果后续后端能够在 reconcile 时把 `stopped + saves=0` 自动收敛成 `save_required`，这段前端兼容仍然安全，因为两个条件等价显示。
- 若未来支持非管理员账号，可能需要给非管理员点击“创建/上传存档”后增加权限提示。

---

## MVP-UX-2: 无存档启动时在启动按钮旁显示创建/上传引导

### 目标

用户继续做 MVP 用户向验证后调整期望：没有存档时点击“启动服务器”，不应只有弹提示，也不应直接替用户跳进创建存档弹窗；应在启动服务器按钮旁边出现一个去创建/上传存档的按钮，并用文字提示“当前没有存档，请点击此按钮去创建/上传存档”。该文字和按钮只在服务器检测没有存档时出现。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | 新增 `saveRequiredDetected` 本地状态；当实例状态是 `save_required` 或本次启动请求收到 `save_required` 时，在启动按钮旁显示无存档引导 |
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | `save_required` 状态下保留禁用的“启动”按钮，不再把生命周期主按钮直接替换成“创建存档” |
| `frontend/src/games/stardew/pages/ServerControlPage.tsx` | 生命周期控制区同样在启动按钮旁显示“当前没有存档，请点击此按钮去创建/上传存档。”和“创建/上传存档”按钮 |
| `frontend/src/games/stardew/pages/ServerControlPage.tsx` | 启动接口返回 `save_required` 时就地展示引导并刷新状态/存档列表，不再自动导航到存档页 |
| `frontend/src/games/stardew/StardewPanel.css` | 新增 `.sd-start-save-required` 样式，提示条用金色警告语义，支持换行，避免窄屏挤压 |

### 影响接口/文件

- 后端 API、Junimo 通信、存档创建/上传接口均未改。
- 仅当前端状态为 `save_required`，或启动请求刚收到 `save_required` 错误码时显示提示和按钮。
- “创建/上传存档”按钮只导航到存档页，不带 `saveAction`，让用户自己选择创建或上传。
- `MVP-UX-1` 新增的 `saveActionRequest` 机制仍保留，但本轮启动失败流程不再使用它自动打开新建弹窗。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.96 kB，JS 327.04 kB
```

建议真实页面补测：

1. 无任何有效存档时进入总览页，状态为 `save_required` 后，启动按钮旁应显示无存档提示和“创建/上传存档”按钮。
2. 点击“创建/上传存档”应进入 `/instances/stardew/saves`，不自动打开新建弹窗。
3. 在服务器控制页重复同样检查。
4. 有存档、`ready_to_start`、`stopped`、`running` 状态下不应显示该提示条。

### 下一步注意

- 如果真实后端返回 `save_required` 但没有及时把 instance state 更新为 `save_required`，本轮本地 `saveRequiredDetected` 会先显示引导；状态恢复到其他值后会自动隐藏。
- 文案里统一使用“存档”，没有沿用用户消息里的“文档”字样。

---

## MVP-UX-1: 无存档启动时直达创建存档界面

### 目标

用户向功能验证发现：MVP 基本完成后，在没有存档的情况下点击“启动服务器”，前端只弹出“没有可用存档，请先创建或上传存档”的提示，没有跳转到创建存档界面。目标是让这条首次启动路径自动进入下一步，而不是让用户自己找入口。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/src/games/stardew/stardew-routes.ts` | 新增 `StardewNavigateOptions`、`StardewSaveActionRequest`，让页面导航可以携带一次性的存档操作意图 |
| `frontend/src/games/stardew/StardewPanel.tsx` | 保存 `saveActionRequest` 并传给当前页面；`navigate('saves', { saveAction: 'new' })` 即可触发存档页动作 |
| `frontend/src/games/stardew/pages/OverviewPage.tsx` | `startInstance()` 返回 `save_required` 时刷新状态/存档并跳到存档页，同时自动打开新建游戏弹窗；已处于 `save_required` 状态时按钮改为“创建存档” |
| `frontend/src/games/stardew/pages/ServerControlPage.tsx` | 生命周期启动失败遇到 `save_required` 时执行同样跳转和弹窗打开逻辑 |
| `frontend/src/games/stardew/pages/SavesPage.tsx` | 将 `saveActionRequest` 传入 `SavesSection` |
| `frontend/src/games/stardew/SavesSection.tsx` | 收到 `saveActionRequest.action === 'new'` 时滚动到存档区域并打开“新建游戏”弹窗；保留 `upload` 动作扩展入口 |

### 影响接口/文件

- 前端内部页面导航签名从 `onNavigate(route)` 扩展为 `onNavigate(route, options?)`；现有只传 route 的调用保持兼容。
- 后端 API、Junimo 通信、存档创建接口、上传接口均未改动。
- `save_required` 会直达新建存档弹窗；`active_save_required` / `active_save_missing` 只跳转存档页，避免已有存档时误导用户新建。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 326.33 kB
```

建议真实页面补测：

1. 准备一个已安装但没有任何有效存档的实例。
2. 在总览页点击“启动”。
3. 预期跳转到 `/instances/stardew/saves`，并直接显示“新建游戏”弹窗。
4. 在服务器控制页重复同样操作，预期一致。
5. 如果后端返回 `active_save_required` 或 `active_save_missing`，预期只进入存档页让用户选择已有存档。

### 下一步注意

- 这次只修 UX 串联，不改变启动状态机；如果真实联调中发现后端没有返回 `save_required` code，而是包在 `start_failed` message 里，需要继续收口后端错误码。
- `saveAction: 'upload'` 已预留并在 `SavesSection` 支持，但当前启动失败默认选择新建存档，因为用户反馈的是“跳转到创建存档界面”。

---

## UI-R18: Stardew wood strip 背景按 image2 风格重新生成

### 目标

用户要求对 `E:\stardew-server-anxi-panel\frontend\dist\assets\stardew\ui\backgrounds\background_frame_wood_strip.png` 用 image2 风格重新生成，明确要求不要简单重绘，而是按满意参考图的高级 Stardew 像素质感重新做对应素材。实际执行时修改 `frontend/public/assets/stardew/ui/backgrounds/background_frame_wood_strip.png` 源素材，再通过 `npm.cmd run build` 同步到 dist。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/backgrounds/background_frame_wood_strip.png` | 按 image2 木条参考重新生成 256x14 不透明 wood strip |
| `frontend/dist/assets/stardew/ui/backgrounds/background_frame_wood_strip.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-background-frame-wood-strip-readable-contact-sheet.png` | 新增本轮 wood strip 可读性 contact sheet |

### 视觉实现

- 先用 image2 生成高阶 Stardew 横向木条参考，再本地按原尺寸重新导出最终 PNG。
- 保留原文件名、原尺寸 `256x14`、原目录结构。
- 保持不透明位图，因为该素材在 `--sd-img-bg-wood-strip` 中作为结构性背景使用。
- 根据页面截图反馈，第一版过亮且细节过多，导致 topbar 上白字不清晰；已改为更暗、更克制的深胡桃木条。
- 设计为可横向 `repeat-x` 的低干扰木条：深棕描边、低对比金色顶光、轻量木纹颗粒、少量木板接缝和像素级阴影，优先保证 topbar 文案可读。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
# public/dist background_frame_wood_strip.png hash 一致
# alpha 扫描确认 HasAlpha=False，尺寸仍为 256x14
# git diff --name-only | Select-String 'frontend/public/assets/stardew/new-game|frontend/dist/assets/stardew/new-game' 无匹配
```

### 下一步注意

- 真实页面里重点看 topbar 背景重复后的木板接缝是否仍然过密；如果显得太花，继续单独调低接缝和颗粒密度即可。
- 这个文件被 `stardew-theme.css` 的 `--sd-img-bg-wood-strip` 引用，后续不要改路径；如要继续微调，优先保持 `256x14` 和不透明结构。

---

## UI-R17: Stardew panels 按 image2 风格重新生成

### 目标

用户要求对 `E:\stardew-server-anxi-panel\frontend\dist\assets\stardew\ui\panels` 下的 panels 素材用 image2 重绘，明确要求不要简单重绘，而是按文件名一一对应生成更好看、更高级的 Stardew 像素面板皮肤。实际执行时修改 `frontend/public/assets/stardew/ui/panels/` 源素材，再通过 `npm.cmd run build` 同步到 dist。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/panels/*.png` | 从 image2 panel sheet 参考重新生成 6 个 panel |
| `frontend/dist/assets/stardew/ui/panels/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-panels-image2-generated-contact-sheet.png` | 新增本轮 panels contact sheet |

### 视觉实现

- 先用 image2 生成高阶 Stardew panel sheet 参考，再按每个文件名裁切和重采样到原尺寸。
- 6 个文件均保留原文件名、原尺寸、原目录结构。
- 6 个文件均保持不透明位图，因为它们作为结构性背景皮肤使用。
- `panel_metric_card_blank.png`：小型指标卡片，木框 + 羊皮纸内底。
- `panel_mod_card_blank.png`：中型模组卡片，紧凑木框面板。
- `panel_parchment_form_blank.png`：表单面板，羊皮纸底和四角铆钉。
- `panel_parchment_section_blank.png`：宽 section 面板，适合横向内容区。
- `panel_table_area_blank.png`：表格区域，保留浅色网格行列。
- `panel_warning_row_blank.png`：warning row，暖红/琥珀警告底色。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
# public/dist panels PNG hash 一致
# alpha 扫描确认 6 个 panels PNG 均 HasAlpha=False
# git diff --name-only | Select-String 'frontend/public/assets/stardew/new-game|frontend/dist/assets/stardew/new-game' 无匹配
```

### 下一步注意

- panels 是结构性背景皮肤，后续微调时优先保持原尺寸和不透明背景。
- 如果真实页面中某个大面板拉伸后边框或角钉显重，单独微调对应 PNG，不要整批回退。

---

## UI-R16: Stardew sprites 按 image2 风格重新生成

### 目标

用户要求对 `E:\stardew-server-anxi-panel\frontend\dist\assets\stardew\ui\sprites` 下的 sprites 素材用 image2 重绘，明确要求不要简单重绘，而是按文件名一一对应生成更好看、更高级的 Stardew 像素 sprite。实际执行时修改 `frontend/public/assets/stardew/ui/sprites/` 源素材，再通过 `npm.cmd run build` 同步到 dist。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/sprites/*.png` | 从 image2 sprite sheet 参考重新生成 8 个 sprite |
| `frontend/dist/assets/stardew/ui/sprites/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-sprites-image2-generated-contact-sheet.png` | 新增本轮 sprites contact sheet |

### 视觉实现

- 先用 image2 生成高阶 Stardew sprite sheet 参考，再按每个文件名裁切、抠图和重采样到原尺寸。
- 8 个文件均保留原文件名、原尺寸、原目录结构。
- 7 个小 sprite 保持透明背景：`sprite_blue_device.png`、`sprite_blue_gem.png`、`sprite_chest.png`、`sprite_cloud_left.png`、`sprite_cloud_right.png`、`sprite_fence.png`、`sprite_tree.png`。
- `sprite_farmhouse_scene.png` 保持不透明 158x92 场景图；它被 Overview banner 作为背景图使用，不适合透明黑底裁切。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
# public/dist sprites PNG hash 一致
# alpha 扫描确认 7 个小 sprite HasAlpha=True，sprite_farmhouse_scene.png HasAlpha=False
# git diff --name-only | Select-String 'frontend/public/assets/stardew/new-game|frontend/dist/assets/stardew/new-game' 无匹配
```

### 下一步注意

- 真实页面里重点看 Overview 顶部横幅的农舍背景是否与文字 overlay 对比足够。
- 小 sprite 当前按原尺寸保守输出；如果后续要让它们在页面里大尺寸展示，可以单独为对应使用处加 CSS 显示尺寸，而不是改变所有源图尺寸。

---

## UI-R15: Stardew navigation 按 image2 风格重新生成

### 目标

用户要求对 `E:\stardew-server-anxi-panel\frontend\dist\assets\stardew\ui\navigation` 下的 navigation 素材用 image2 重绘，明确要求不要简单重绘，而是按文件名一一对应生成更好看、更高级的 Stardew 像素 UI 皮肤。实际执行时修改 `frontend/public/assets/stardew/ui/navigation/` 源素材，再通过 `npm.cmd run build` 同步到 dist。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/navigation/*.png` | 重新生成 7 个 navigation PNG |
| `frontend/dist/assets/stardew/ui/navigation/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-navigation-image2-generated-contact-sheet.png` | 新增本轮 navigation contact sheet |

### 视觉实现

- 先用 image2 生成高阶 Stardew 导航皮肤方向，再按每个文件原尺寸本地导出，避免破坏现有 CSS 背景拉伸。
- 7 个文件均保留原文件名、原尺寸、原目录结构。
- 7 个文件仍保持不透明位图，因为它们作为背景皮肤使用，不应改成透明大图。
- `nav_item_default_blank.png`：深木纹默认导航条。
- `nav_item_active_green_blank.png`：绿色 active 导航条，带厚木框、角钉、高光和内阴影。
- `nav_item_active_saves_blank.png`：存档专用 active，羊皮纸内底 + 绿色下划强调。
- `nav_quick_help_blank.png`：小型木质帮助按钮。
- `tab_content_active_blank.png` / `tab_content_inactive_blank.png`：active/inactive tab 内容皮肤区分。
- `tab_top_green_blank.png`：绿色顶部 tab，带 raised tab 结构。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
# public/dist navigation PNG hash 一致
# alpha 扫描确认 7 个 navigation PNG 均 HasAlpha=False
# git diff --name-only | Select-String 'frontend/public/assets/stardew/new-game|frontend/dist/assets/stardew/new-game' 无匹配
```

### 下一步注意

- navigation 是结构性背景皮肤，后续微调时优先保持原尺寸和不透明背景。
- 如果真实页面中某个 tab 或左侧导航拉伸后边框显重，单独微调对应 PNG，不要整批回退。

---

## UI-R14: Stardew 图标按满意参考图直接裁切

### 目标

用户明确确认 `C:\Users\anxi\AppData\Local\Temp\codex-clipboard-b6edf34e-1046-4b35-b2ac-4b3dd6d502b7.png` 这张 4x4 图标图满意，要求“就按照这张图来”，且像素大小可以和图中元素一致。本轮目标是停止继续风格再生成，直接使用这张参考图作为最终视觉源。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/icons/*.png` | 从参考图手工框裁切并抠透明，输出 16 个大尺寸 PNG 图标 |
| `frontend/dist/assets/stardew/ui/icons/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-icons-reference-crop-contact-sheet.png` | 新增本轮裁切版 contact sheet |

### 映射关系

参考图按 4x4 从左到右、从上到下对应：

1. `icon_button_play.png`
2. `icon_button_restart.png`
3. `icon_button_stop.png`
4. `icon_nav_diagnostics.png`
5. `icon_nav_mods.png`
6. `icon_nav_overview_home.png`
7. `icon_nav_players.png`
8. `icon_nav_saves.png`
9. `icon_nav_server_control.png`
10. `icon_nav_settings.png`
11. `icon_nav_tasks.png`
12. `icon_sidebar_chicken.png`
13. `icon_top_summary_players.png`
14. `icon_top_summary_save.png`
15. `icon_top_summary_time.png`
16. `icon_top_summary_version.png`

### 实现细节

- 直接按参考图元素裁切，不再压缩回原来的 13x13/16x17 小尺寸。
- 本地抠图保留高光、深棕描边、像素阴影和主体细节，输出 PNG 透明背景。
- CSS 已对导航、页面标题、顶部摘要和按钮内图标做显示尺寸约束，所以大尺寸源图不会撑坏布局。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
# public/dist icons PNG hash 一致
# 16 个图标四角 alpha 均为 0
# git diff --name-only | Select-String 'frontend/public/assets/stardew/new-game|frontend/dist/assets/stardew/new-game' 无匹配
```

### 下一步注意

- 如果真实页面里某些图标被 CSS 缩到 13px 后细节过密，可以只针对对应使用处调整显示尺寸或单独微调对应 PNG。
- 不建议再用生成模型重跑整批；当前用户已明确满意这张参考图风格。

---

## UI-R13: Stardew 图标按文件名重新生成

### 目标

用户反馈 UI-R12 的图标仍像简单重绘，要求“不要简单重绘”，而是按参考图风格和图片名一一对应重新生成一批更好看、更高级的图标。实际仍遵循项目资产规则：修改 `frontend/public/assets/stardew/ui/icons/` 源文件，再通过 `npm.cmd run build` 同步到 `frontend/dist/assets/stardew/ui/icons/`。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/icons/*.png` | 重新生成 16 个透明 PNG 图标 |
| `frontend/dist/assets/stardew/ui/icons/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-icons-image2-generated-contact-sheet.png` | 新增本轮图标 contact sheet |

### 视觉实现

- 使用 imagegen 生成高阶 Stardew 像素风图标参考，但最终导出仍按每个原始 PNG 的尺寸和透明边界本地生成，避免破坏现有 CSS 对齐。
- 16 个图标按文件名语义一一对应重新设计：
  - `icon_button_play.png`、`icon_button_restart.png`、`icon_button_stop.png`
  - `icon_nav_diagnostics.png`、`icon_nav_mods.png`、`icon_nav_overview_home.png`、`icon_nav_players.png`
  - `icon_nav_saves.png`、`icon_nav_server_control.png`、`icon_nav_settings.png`、`icon_nav_tasks.png`
  - `icon_sidebar_chicken.png`
  - `icon_top_summary_players.png`、`icon_top_summary_save.png`、`icon_top_summary_time.png`、`icon_top_summary_version.png`
- 风格上使用暖金高光、深棕描边、局部阴影和更明确的图标轮廓；13x13 小图标优先保证真实页面里的识别度。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
# public/dist icons PNG hash 一致
# 16 个 icons alpha 扫描均 HasAlpha=True
# git diff --name-only | Select-String 'new-game' 无匹配
```

### 下一步注意

- 真实页面里重点看 13x13 导航图标在左侧按钮上的可读性。
- 如果某个极小图标仍不够清楚，建议只微调对应 PNG，不要整组回退。

---

## UI-R12: Stardew 图标位图高级重绘

### 目标

用户要求对 `E:\stardew-server-anxi-panel\frontend\dist\assets\stardew\ui\icons` 下图标素材用 image2 风格重绘，要求更高级、更好看。实际执行时修改 `frontend/public/assets/stardew/ui/icons/` 源文件，再运行 build 同步到 dist。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/icons/*.png` | 重绘 16 个透明图标 PNG |
| `frontend/dist/assets/stardew/ui/icons/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-icons-image2-premium-contact-sheet.png` | 新增 5 倍放大图标 contact sheet |

### 视觉实现

- 保留每个图标原尺寸、原文件名、原目录结构。
- 使用高级像素图标风格：深色投影、暖金主体、浅色高光。
- 保留透明背景，适配现有按钮/导航/摘要区域。
- 13x13 小导航图标优先保证语义可读；玩家图标已单独加强为多人轮廓。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
git diff --name-only -- frontend/public/assets/stardew/ui/icons | Measure-Object
# 16
```

- public/dist icons PNG hash 一致。
- alpha 扫描确认 16 个图标均保留透明背景。
- `new-game` 无变更。

### 下一步注意

- 真实页面里重点看 13x13 导航图标在左侧按钮上的识别度。
- 如果某个图标语义不够清楚，建议单独微调该图标，不要回退整组。

---

## UI-R11: Stardew 输入框位图高级重绘

### 目标

用户要求对 `E:\stardew-server-anxi-panel\frontend\dist\assets\stardew\ui\fields` 下输入框素材用 image2 风格重绘，要求更高级、更好看。实际执行时修改 `frontend/public/assets/stardew/ui/fields/` 源文件，再运行 build 同步到 dist。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/fields/*.png` | 重绘 4 个输入框 PNG |
| `frontend/dist/assets/stardew/ui/fields/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-fields-image2-premium-contact-sheet.png` | 新增输入框 contact sheet |

### 视觉实现

- 保留每个输入框原尺寸、原文件名、原目录结构。
- 使用高级像素输入框皮肤：深木边框、羊皮纸内底、内高光、底部阴影、细纸纹。
- 搜索框保留右侧像素放大镜；下拉框保留右侧控制区和像素箭头。
- 4 个输入框全部保持不透明，避免 CSS 背景拉伸漏底。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
git diff --name-only -- frontend/public/assets/stardew/ui/fields | Measure-Object
# 4
```

- public/dist fields PNG hash 一致。
- alpha 扫描确认 4 个输入框均不透明。
- `new-game` 无变更。

### 下一步注意

- 真实页面里重点看输入框文字、placeholder 和右侧图标的对比度。
- 如果某个输入框在 CSS 拉伸后边框太重，优先单独微调该 PNG，不要回退整组。

---

## UI-R10: Stardew 按钮位图高级重绘

### 目标

用户要求对 `E:\stardew-server-anxi-panel\frontend\dist\assets\stardew\ui\buttons` 下按钮用 image2 风格重绘，要求更高级、更好看。实际执行时修改 `frontend/public/assets/stardew/ui/buttons/` 源文件，再运行 build 同步到 dist。

### 改了什么

| 文件/范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/buttons/*.png` | 重绘 12 个按钮 PNG |
| `frontend/dist/assets/stardew/ui/buttons/*.png` | 通过 `npm.cmd run build` 同步 |
| `tmp/stardew-buttons-image2-premium-contact-sheet.png` | 新增按钮 contact sheet |

### 视觉实现

- 保留每个按钮原尺寸、原文件名、原目录结构。
- 使用高级像素按钮皮肤：外层木框硬边、内高光、底部阴影、细颗粒纹理。
- 按语义重绘 green/red/tan/gold/wood 变体。
- 12 个按钮全部保持不透明，避免 CSS 背景拉伸漏底。
- 未触碰 `new-game`、业务逻辑、API、CSS 路径。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

补充检查：

```powershell
git diff --name-only -- frontend/public/assets/stardew/ui/buttons | Measure-Object
# 12
```

- public/dist buttons PNG hash 一致。
- alpha 扫描确认 12 个按钮均不透明。
- `new-game` 无变更。

### 下一步注意

- 真实页面里重点看按钮文字在绿色/红色按钮上的对比度。
- 如果某个按钮因 CSS 拉伸显得边框过重，优先单独微调该按钮，不要回退整组。

---

## UI-R9: 左侧导航点击态尺寸统一

### 目标

用户反馈左侧栏除“服务器”外，其他导航按钮点击后的效果尺寸长宽不一致。目标是让所有桌面端左侧导航按钮的点击/激活态视觉尺寸与“服务器”项一致。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/StardewPanel.css` | 将 `.sd-sidebar .sd-nav-item` 默认尺寸改为服务器项尺寸：`--sd-nav-w: 110`、`--sd-nav-active-w: 106`、`--sd-nav-active-h: 29` |
| `frontend/src/games/stardew/StardewPanel.css` | 删除 `overview/server/saves/jobs/players/mods/diagnostics/install/settings` 各自单独设置 `--sd-nav-w` / `--sd-nav-active-w` / `--sd-nav-active-h` 的规则 |
| `frontend/src/games/stardew/StardewPanel.css` | `saves` 继续使用 `nav_item_active_saves_blank.png` 专用激活贴图，但不再覆盖激活态尺寸 |

### 影响范围

- 只影响 Stardew 面板左侧导航的桌面端按钮宽度与激活态贴图尺寸。
- 不改导航 PNG 素材、不改路由、不改页面组件、不改后端。
- 移动端侧栏规则保持原样：图标按钮仍为 `min-width: 36px; height: 30px`。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 modules，CSS 89.68 kB，JS 325.25 kB
```

### 下一步注意

- 当前工作区已有较多 UI PNG 与 CSS 改动，`StardewPanel.css` 的 diff 会混入之前的状态语义色等改动；本轮只新增/调整左侧导航尺寸规则。
- 如果后续继续微调左侧栏，优先改 CSS 尺寸变量，避免为了尺寸问题再次改导航贴图素材。

---

## UI-R8: Stardew UI 位图资产按参考图重制（已完成）

### 目标

按用户要求只优化 `frontend/public/assets/stardew/ui/` 目录下的 UI 位图素材，解决原素材截图感、压缩感、缺失/破损感明显的问题。不要改 `new-game` 目录，不改业务逻辑、不改 API、不改 CSS 路径。

### 改了什么

按用户提供的参考图 `codex-clipboard-da9ce68b-ffb8-448e-bd80-030206b9aa24.png` 重制 `frontend/public/assets/stardew/ui/` 下 56 个 tracked PNG。实现方式不是 imagegen 大图硬缩，而是从参考图中裁切对应 UI 元素，按项目原尺寸重采样导出。`background_sidebar_wood_tile.png` 已按用户要求单独回退到原始版本，并通过 build 同步到 dist。

| 范围 | 修改 |
|------|------|
| `frontend/public/assets/stardew/ui/backgrounds` | 重制黑底、木条、羊皮纸 tile；保留 `background_login_farm_generated.png`；`background_sidebar_wood_tile.png` 单独回退 |
| `frontend/public/assets/stardew/ui/buttons` | 按参考图重制绿色、红色、tan、gold、wood 和方形按钮 |
| `frontend/public/assets/stardew/ui/fields` | 按参考图重制邀请码框、路径输入框、搜索框、下拉框 |
| `frontend/public/assets/stardew/ui/navigation` | 重制默认/激活导航和 tab 皮肤 |
| `frontend/public/assets/stardew/ui/panels` | 重制大羊皮纸面板、表格区域、表单卡、warning row、metric/mod 卡 |
| `frontend/public/assets/stardew/ui/icons` | 从参考图按钮网格裁切图标并扣透明背景 |
| `frontend/public/assets/stardew/ui/sprites` | 从参考图裁切蓝宝石、箱子、云、农舍场景、栅栏、树等 sprite |
| `tmp/stardew-ui-assets-reference-crop-contact-sheet.png` | 新增本轮视觉总览图 |

### 实现原则

- 只处理 `frontend/public/assets/stardew/ui/`，未触碰 `frontend/public/assets/stardew/new-game/`。
- 保留所有旧文件名、原尺寸、原目录结构。
- 结构性 UI 资产强制不透明：backgrounds/buttons/fields/navigation/panels。
- 透明图只保留在 icons 和部分 sprites；`sprite_farmhouse_scene.png` 按不透明场景图处理。
- `background_login_farm_generated.png` 是 UI-R7 登录页背景，本轮保留，不纳入 tracked 旧资产重制。
- `background_sidebar_wood_tile.png` 已单独回退；当前 UI diff 计数因此为 56。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# exit 0，39 模块，JS 325.25 kB，CSS 90.39 kB
```

补充检查：

```powershell
git diff --name-only -- frontend/public/assets/stardew/ui | Measure-Object
# 56

git diff --name-only -- frontend/public/assets/stardew/ui | Select-String new-game
# 无输出
```

视觉检查：
- `tmp/stardew-ui-assets-reference-crop-contact-sheet.png` 已生成。
- contact sheet 中结构件尺寸保持原样，透明图未变黑底。

### 下一步注意事项

- 重点看真实页面里的按钮、导航、面板背景和 13x13 小图标语义。
- 如果某几个图标不够清楚，建议单独微调，不要整批回退。
- `new-game` 下农场/宠物/角色预览图本轮未处理。

---

## UI-R7: 登录首页视觉重构（Stardew 风格统一）

### 目标

将登录/初始化页面从原有的普通后台 SaaS 风格完全重构为 Stardew Valley 像素风，使其与面板内部 (`StardewPanel`) 视觉系统归属同一套产品体系。不改动登录 API、认证逻辑、后端，只调整组件结构、className 和 CSS。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/App.css` | 末尾新增 `/* ── UI-R7: Stardew Auth Shell ──` 块，约 230 行，包含 `.sd-auth-shell`、`.sd-auth-card`、`.sd-auth-eyebrow`、`.sd-auth-title`、`.sd-auth-version`、`.sd-auth-error`、`.sd-auth-loading`，及 `.sd-auth-card .form-grid/field/field input/password-input/password-toggle/button/form-hint` 上下文覆盖样式，加 `@media (max-width: 430px)` 和 `@media (max-width: 340px)` 断点 |
| `frontend/src/App.tsx` | 非 stardew 视图的 return 从 `main.shell > section.panel-card` 改为 `main.sd-auth-shell > section.sd-auth-card`；`p.eyebrow` → `p.sd-auth-eyebrow`；`h1` 加 `className="sd-auth-title"`；`p.version-info` → `p.sd-auth-version`；`div.error-banner` → `div.sd-auth-error`；`p.summary` → `p.sd-auth-loading`（仅 booting 状态） |
| `frontend/src/core/LoginPanel.tsx` | 移除 `<p className="summary">请输入面板账号登录……</p>` 段落，表单直接从字段开始，符合"不使用大段说明文字"要求 |
| `frontend/public/assets/stardew/ui/backgrounds/background_login_farm_generated.png` | 新增登录页原创像素农场背景图，替代直接拉伸 `sprite_farmhouse_scene.png` |

**SetupPanel.tsx 未改动**（保留初始化说明文字，对首次使用者有必要性）。

### 视觉设计实现细节

**背景：**
- `.sd-auth-shell::before`：`background-image: url('/assets/stardew/ui/backgrounds/background_login_farm_generated.png')`，`background-size: cover`，`background-position: center`
- 不再把 `sprite_farmhouse_scene.png` 直接拉伸成整页背景，避免低分辨率素材放大后发糊
- `.sd-auth-shell::after`：`rgba(12, 6, 1, 0.48)` 半透明暗色蒙层，保证面板可读性而不遮挡背景氛围
- 新背景图已在 `frontend/public/assets/stardew/ui/backgrounds/`，build 后以 `/assets/stardew/ui/backgrounds/background_login_farm_generated.png` 访问

**面板：**
- 桌面端 `.sd-auth-shell` 使用 `justify-content: flex-end`，并通过 `padding-right: clamp(36px, 11vw, 180px)` 将卡片放到右侧背景留白区，避免遮挡左侧农舍主视觉
- 900px 以下回到居中，避免窄桌面/平板卡片贴边
- 5px 深棕边框 `#5b2f18`，零圆角（像素风）
- `background-image` 使用暖色渐变叠加 `background_parchment_tile.png`，`background-color: #e7b96f`，降低白纸感
- `box-shadow` 使用内描边 + 短木质底边 + 柔和投影，让卡片更接近背景里的木框告示牌
- 宽度 `min(420px, 100%)`，比旧版略窄

**表单覆盖（`.sd-auth-card .xxx`）：**
- 输入框：34px 高，2px 棕色边框，无圆角，`#fff0c7` 底色，13px 正文字号
- 密码切换：同上高度，`#ffe6b6` 底色
- 登录按钮：高 36px，使用 `button_primary_small_green_blank.png` PNG 底图（`background-size: 100% 100%`），无 `box-shadow`，颜色白色文字
- 错误条 `.sd-auth-error`：`border: 3px solid #b54837`，`background: #ffe0d6`，`color: #681111`（与全站 `sd-notice--error` 语义一致）

**CSS 变量依赖：** 无，`sd-auth-*` 全部使用硬编码颜色值，避免 stardew-theme.css 加载顺序依赖问题（尽管 stardew-theme.css 已全局导入）。

### 影响的接口/文件

- `frontend/src/App.css`：新增末尾样式块，不影响已有规则
- `frontend/src/App.tsx`：只改 login/setup/booting 三个视图的 className，stardew 视图路径 (`view === 'stardew'`) 完全不动
- `frontend/src/core/LoginPanel.tsx`：减少一个 `<p>` 段落，不影响 form 提交逻辑
- 旧的 `.shell`、`.panel-card`、`.eyebrow`、`.version-info` CSS 类在 App.css 中保留（不删），但不再被任何 JSX 引用（死规则，后续可按需清理）

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，39 模块，JS ~325 kB，CSS ~90 kB
```

**视觉验证（Playwright 已执行）：**
- `localhost:5173`（Vite dev server）- 1280px：原创像素农场背景全屏，面板位于右侧留白区，所有元素渲染正常
- 390px：面板 100% 宽，背景图下方可见，无横向滚动
- 320px：面板紧凑，无横向滚动
- 填入账号密码后：输入框 focus 绿色 outline，密码 toggle 可用
- 错误状态：显示 Stardew 风格红色错误条

**手动检查（如需重现）：**
1. 启动 Vite dev server（已在运行）：访问 `http://localhost:5173`
2. 登录页应第一眼显示原创像素农场背景，不再显示被拉伸的 farmhouse sprite
3. 使用浏览器 DevTools，将宽度调至 390px / 320px，确认无横向滚动条
4. 错误状态：输入错误密码后查看错误条样式

### 下一步注意事项

**UI-R8（如需）：进一步精细化方向**
- 登录按钮 PNG 在某些显示器/缩放下可能偏浅，可考虑在按钮上叠加一个 `color: rgba(0,0,0,0.15)` 文字阴影增强可读性
- SetupPanel 的「创建管理员」按钮同样通过 `.sd-auth-card .button` 覆盖为 Stardew 绿色按钮，视觉一致
- 如果未来需要在登录页加 logo 图（如 farmhouse icon），可在 `.sd-auth-card` 顶部加一个 `img.sd-auth-logo`，推荐复用 `sprite_chest.png` 或 `sprite_blue_gem.png`

**清理（可选，低优先级）：**
- App.css 中 `.shell`、`.panel-card`、`.eyebrow`、`.version-info` 已无 JSX 引用，可择机删除

**后端嵌入 frontend：**
- 当前 Vite dev server (5173) 与 Go backend (8090) 分离运行
- `npm.cmd run build` 生成的 `frontend/dist/` 需要重新编译 Go binary 才能被嵌入生效
- 如果需要测试完整部署效果，需 `go build` 后重启 backend
