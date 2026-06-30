# 后端接手文档 2026-06-30

## 本次文档归并

本次把原 `architecture.md`、`handoff-roadmap.md`、历史 `conversation-handoff-*`、`deployment.md`、`release-checklist.md`、`later-optimizations.md` 等后端相关内容压缩进九份长期文档。后端接手入口改为本文和 `docs/02-backend.md`。

影响文件：

- 新增 `docs/02-backend.md`
- 新增 `docs/backend-handoff/backend-handoff-2026-06-30.md`
- 新增 `docs/06-integration.md`
- 新增 `docs/08-future-roadmap.md`
- 新增 `docs/09-image-build.md`
- 删除旧的流水式 handoff/roadmap/deployment/release checklist 文档
- 更新 `AGENTS.md` 与 `CLAUDE.md` 的阅读规则

验证方式：

```powershell
rg --files docs -g "*.md" -g "*.txt"
```

预期只出现九份维护文档。

## 后端当前状态

后端主线已经完成：

- Go 服务、SQLite、迁移、用户、session、权限。
- Docker / Compose 封装、命令脱敏。
- Jobs / job logs / SSE / instance state。
- GameDriver registry 与 `stardew_junimo` driver。
- Junimo compose 准备、安装、Steam Auth、生命周期控制。
- 存档管理：新建、上传预览、提交启动、选择、删除、导出、备份恢复。
- Mod 管理：上传、删除、导出、重启提示。
- 控制台命令：allowlist、权限、FIFO/output log 通信。
- 审计日志、健康检查、支持包导出、版本信息。
- 玩家信息 API 与 SMAPI 控制文件读取。
- 邀请码旧码清理与状态校准。
- 单真人玩家菜单暂停 SMAPI mod 修复。

## 最近重点改动

### LIFECYCLE-JOBS-1: 生命周期任务取消修复

问题：启动服务器遇到错误或长时间等待时，如果用户随后点击停止，停止任务会完成，但旧启动任务仍保持 running，导致任务列表和实例状态误导用户。

当前处理：

- `jobs.Manager.Cancel()` 已实现，运行中任务会触发 context cancel，最终状态写为 `canceled`。
- 新增 `jobs.Manager.CancelActive()` 和 `storage.ListActiveJobs()`，可按 target/type 取消仍在 queued/running 的任务。
- Stardew `Start` / `Stop` / `Restart` 在创建新的 `stardew_lifecycle` 任务前，会取消同实例旧生命周期任务。
- 启动/重启任务在等待邀请码期间如果收到取消，不再把实例写回 `running`。

涉及：

- `backend/internal/jobs/manager.go`
- `backend/internal/jobs/manager_test.go`
- `backend/internal/storage/jobs.go`
- `backend/internal/games/stardew_junimo/lifecycle.go`

验证：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
```

### 邀请码重启刷新

问题：`docker compose restart` 会复用同一容器，容器内 `/tmp/invite-code.txt` 可能保留旧邀请码。

当前处理：

- 重启前尝试删除旧 `invite-code.txt`。
- 重启后如果文件内容等于 `driver_payload.invite_code` 中记录的旧码，则删除并继续等待新码。
- 前端启动/重启成功后不会立刻显示旧码，而是进入等待新码轮询。
- 后端重启 Stardew 时只执行 `docker compose restart server`，不再裸跑 `docker compose restart`。不要重启 `steam-auth`，否则 sidecar 会重新登录 Steam，短时间内可能触发 `RateLimitExceeded`，导致 Junimo 日志出现 `Steam-auth service not ready` 和 `Invite Code: n/a`。

涉及：

- `backend/internal/docker/compose.go`
- `backend/internal/games/stardew_junimo/lifecycle.go`
- `backend/internal/games/stardew_junimo/lifecycle_test.go`

### 玩家信息

SMAPI mod 输出玩家快照，后端合并在线快照和缓存名册，返回玩家状态、位置、tile/pixel 字段。

最新修复：`players-cache.json` 已按 `saveId` 隔离。新建或切换存档后，只有缓存的 `saveId` 与当前 `players.json.saveId` 一致时才会合并历史离线玩家；旧版无 `saveId` 的缓存会在有当前存档 ID 时被忽略，避免上一存档玩家污染新存档玩家列表。

收入字段修复：SMAPI mod 现在同时写出 `farmIncome` 和 `personalIncome`。`farmIncome` 固定来自 `farmer.totalMoneyEarned`，表示农场/团队累计收入；`personalIncome` 固定来自 `farmer.stats.Get("individualMoneyEarned")`，表示玩家个人累计收入；`totalMoneyEarned` 只作为旧字段兼容。新 DLL 已同步到嵌入目录和当前 `stardew` 实例目录，验证时必须重启 Stardew server 容器并等待新的 `players.json` 写出。

最近事件：`ListPlayers` 会根据当前在线快照和缓存里的上一轮 `status` 生成 `seen`、`joined`、`left` 事件，写入 `.local-container/control/players-events.json`，最多保留 50 条。事件和玩家缓存一样按 `saveId` 隔离；服务器停止时如果缓存里仍有在线玩家，会标记为离线并补一条 `left` 事件。

涉及：

- `backend/internal/games/stardew_junimo/players.go`
- `backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ModEntry.cs`
- `backend/internal/games/stardew_junimo/embedded/smapi-mod/StardewAnxiPanel.Control.dll`
- `backend/internal/web/players_handlers.go`
- `frontend/src/games/stardew/pages/PlayersPage.tsx`

注意：如果要看到新 SMAPI DLL 行为，必须重启 Stardew server 容器。

### 单人菜单暂停

最新状态：AUTOPAUSE-7 保持服务端读取 `hasMenuOpen || requestingTimePause` 触发暂停，不引入客户端配套 mod。规则是：只有 1 个真人玩家时，该玩家打开菜单即暂停；有 2 个及以上真人玩家时，必须所有真人玩家都打开菜单才暂停。时钟控制继续参考 `Pause Time in Multiplayer` 1.6 分支，从 `IsTimePaused/pauseTime` 切到 `Game1.gameTimeInterval` 哨兵方案：暂停时保存当前 interval 并写 `-100`，恢复时发现哨兵就写回保存值。释放时仍清理旧版本可能残留的 `Game1.netWorldState.Value.IsTimePaused`、`Game1.netWorldState.Value.IsPaused`、`Game1.isTimePaused` 和 `Game1.pauseTime`；异常时也会先释放时钟，避免背包关闭后时间永久不流动。

涉及：

- `backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ModEntry.cs`
- `backend/internal/games/stardew_junimo/embedded/smapi-mod/StardewAnxiPanel.Control.dll`

验证：

```powershell
docker run --rm `
  -v "E:\stardew-server-anxi-panel\backend\internal\games\stardew_junimo\embedded\smapi-mod-src:/src" `
  -v "E:\stardew-anxi-panel\runtime\game:/game" `
  -w /src mcr.microsoft.com/dotnet/sdk:6.0 `
  dotnet build -c Release /p:GamePath=/game
```

下一步注意：必须重启 Stardew server 容器，新 DLL 才会加载。验证时只需要服务器端 mod；不需要客户端额外安装同名 mod。多人验证时，一个玩家打开菜单不应暂停；所有真人玩家都打开菜单才应暂停。如果仍出现“打开能暂停但关闭不恢复”，优先检查 `Game1.gameTimeInterval` 是否停在 `-100`，以及新 DLL 是否真的加载。

## 后端验证建议

通用：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
```

聚焦 Stardew driver：

```powershell
go test ./internal/games/stardew_junimo
go test ./internal/web -run "TestInstance|TestDocker|TestPermission|TestAudit|TestHealth"
```

SMAPI mod：

```powershell
docker run --rm `
  -v "E:\stardew-server-anxi-panel\backend\internal\games\stardew_junimo\embedded\smapi-mod-src:/src" `
  -v "E:\stardew-anxi-panel\runtime\game:/game" `
  -w /src mcr.microsoft.com/dotnet/sdk:6.0 `
  dotnet build -c Release /p:GamePath=/game
```

## 下一步注意事项

- 联机角色槽异常仍需单独排查，不要做破坏性存档修改工具。
- 当前玩家页仍是 5 秒轮询；事件驱动更新放在后期优化。
- Unix 平台 TTY 实现和真实 Docker 联调仍要持续验证。
- 新增后端接口时同步更新 `docs/02-backend.md` 和 `docs/06-integration.md`。
- 影响发布、镜像或运行命令时同步更新 `docs/09-image-build.md`。
