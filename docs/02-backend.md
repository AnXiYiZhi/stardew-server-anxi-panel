# 后端文档

## 总体结构

后端是 Go 单体服务，负责面板鉴权、任务编排、Docker/Compose 控制、Stardew Junimo driver、SQLite 持久化、审计和静态前端托管。

建议边界如下：

```text
backend/cmd/panel              启动入口
backend/internal/auth          登录、密码、session
backend/internal/docker        Docker / Compose 封装与脱敏
backend/internal/jobs          长任务、日志、SSE
backend/internal/storage       SQLite、迁移、用户、状态、审计
backend/internal/static        嵌入 frontend/dist
backend/internal/web           HTTP handler、鉴权、路由
backend/internal/games         GameDriver 与游戏实现
backend/internal/games/stardew_junimo
```

Stardew 业务逻辑优先放在 `backend/internal/games/stardew_junimo`，不要散落到通用 handler 或 Docker 层。

## GameDriver 边界

`GameDriver` 表示“面板对某个游戏实例下达通用意图”的后端接口。Stardew 当前 driver ID 是 `stardew_junimo`，默认实例 ID 是 `stardew`。

driver 应负责：

- 实例目录准备与 compose 模板。
- 安装流程与 Steam Auth。
- 启动、停止、重启、状态校准。
- 邀请码读取。
- 存档、Mod、控制台命令、玩家信息。
- 与 JunimoServer 容器通信。

通用 web 层只做鉴权、参数解析、调用 driver、写审计、返回结构化错误。

## Junimo 通信优先级

1. 挂载文件：`.env`、`docker-compose.yml`、settings、saves、mods、backups、control JSON。
2. Docker Compose：`pull`、`up -d`、`down`、`restart`、`ps`、`logs`、`exec`、`run`。
3. Steam Auth TTY：扫码登录走 `steam-auth setup`，账号密码安装优先走 `steam-auth download`。
4. Junimo / SMAPI 控制：`attach-cli` 或当前 FIFO / output log 通信。
5. Junimo HTTP API：仅在启用并具备 API key 时用于状态或监控。
6. VNC：只做高级调试，不作为常规控制通道。

## 状态与任务

核心状态包括：

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

`jobs` 和 `job_logs` 用于安装、认证、启动等长任务。日志通过：

```text
GET /api/jobs/:id/stream
```

推送 `log`、`finished`、`ping` 事件。写日志前必须脱敏。

生命周期任务必须可取消。同一实例的 `stardew_lifecycle` 任务采用“最后操作生效”：新的启动、停止或重启请求会先取消该实例仍在 queued/running 的旧生命周期任务。被取消的任务状态必须落到 `canceled`，不能继续挂在 running，也不能在取消后把实例状态重新写回 running。

## 主要 API 分组

| 分组 | 代表接口 |
| --- | --- |
| 健康与版本 | `GET /health`, `GET /api/version`, `GET /api/health/diagnostics` |
| 认证 | `GET /api/auth/status`, `POST /api/auth/setup`, `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me` |
| 用户 | `/api/users` 管理员接口 |
| 实例状态 | `GET /api/instances/:id/state` |
| 安装 | `POST /api/instances/:id/prepare`, `GET /api/instances/:id/install-options`, `POST /api/instances/:id/install`, Steam Guard 输入接口 |
| 生命周期 | `POST /api/instances/:id/start`, `stop`, `restart` |
| 邀请码 | `GET /api/instances/:id/invite-code` |
| 存档 | `GET /api/instances/:id/saves`, 上传预览、提交启动、选择、删除、导出、备份恢复 |
| Mods | `GET /api/instances/:id/mods`, 上传、删除、导出 |
| 命令 | `GET /api/instances/:id/commands`, `POST /commands/run`, `POST /commands/say` |
| 玩家 | `GET /api/instances/:id/players` |
| Docker 诊断 | compose ps/logs/status 相关调试接口 |
| 审计与支持包 | `GET /api/audit-logs`, `POST /api/instances/:id/support-bundle` |

新增接口时必须返回结构化错误码，前端通过错误码映射中文提示。

## Stardew 关键实现

### Steam Auth

- 官方服务名保留 `steam-auth`、`server`、`discord-bot`。
- 官方镜像版本变量是 `IMAGE_VERSION`。
- `steam-auth` sidecar 当前默认使用 `anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`。
- `.env` 会写入 Steam 连接等待和认证重试相关变量。
- 不要用普通 stdin 重定向跑 `steam-auth setup` 的账号密码分支；该分支会用 `Console.ReadKey()` 读密码，后台 pipe 会失败。

### 邀请码

Junimo 会把邀请码写入容器内 `/tmp/invite-code.txt`。`docker compose restart` 可能保留旧文件，因此重启前后要清理或过滤旧码，前端启动/重启后也要等待非旧码的新邀请码。

Stardew 生命周期里的“重启服务器”必须只重启 Compose 的 `server` 服务，不要重启 `steam-auth`。`steam-auth` 重启会重新登录 Steam，短时间多次尝试可能触发 `RateLimitExceeded`，导致 Junimo 启动时 30 秒内无法访问 steam-auth，最终日志显示 `Steam-auth service not ready` 且邀请码为 `n/a`。

### 玩家信息

当前通过 SMAPI 控制模组写 `.local-container/control/players.json`，后端合并 `players-cache.json` 输出玩家名册。前端 5 秒轮询，成本低且稳定。

收入口径由 SMAPI mod 写入并保持固定含义：`farmIncome` 始终表示农场/团队累计收入，`personalIncome` 始终表示玩家 `individualMoneyEarned` 个人累计收入；`totalMoneyEarned` 仅保留为旧字段兼容。修改嵌入 DLL 后必须同步 `embedded/smapi-mod` 和已部署实例，并重启 Stardew server 容器才会生效。

最近事件由后端根据当前在线快照和 `players-cache.json` 中上一轮状态差分生成，并写入 `.local-container/control/players-events.json`。当前支持首次记录、加入和离开事件，最多保留最近 50 条，并按 `saveId` 隔离，避免切换存档后显示旧存档活动。

`players-cache.json` 必须按存档隔离：缓存文件带 `saveId`，只有 `cache.saveId == players.json.saveId` 时才合并历史离线玩家。切换/新建存档后，旧存档名册不能混入新存档玩家列表；旧版无 `saveId` 的缓存遇到当前存档 ID 时应被忽略并在下一次读取后重写。

### 单人菜单暂停

`StardewAnxiPanel.Control` SMAPI mod 支持真人玩家菜单暂停。当前逻辑仍在 Junimo 服务端读取远端 `Farmer.hasMenuOpen || Farmer.requestingTimePause` 来触发背包/菜单暂停；实测服务端能通过该字段看到背包打开。AUTOPAUSE-7 的规则是：只有 1 个真人玩家时，该玩家打开菜单即暂停；有 2 个及以上真人玩家时，必须所有真人玩家都打开菜单才暂停，避免单个玩家翻背包影响其他正在行动的人。时钟控制沿用 AUTOPAUSE-6 参考 `Pause Time in Multiplayer` 的 1.6 分支做法，不再靠 `IsTimePaused/pauseTime` 维持暂停，而是在 `UpdateTicking` 中保存 `Game1.gameTimeInterval` 并写入 `-100` 哨兵冻结时钟；关闭背包后发现哨兵则恢复保存的 interval，同时清理旧版本可能残留的 `Game1.netWorldState.Value.IsTimePaused`、`Game1.netWorldState.Value.IsPaused`、`Game1.isTimePaused` 和 `Game1.pauseTime`。如果暂停 tick 中抛异常，mod 会先释放时钟，避免再次永久卡住。

## 安全要求

- 控制台命令必须 allowlist，普通用户和管理员可见命令不同。
- 存档 ZIP 解压必须防 zip-slip、绝对路径和符号链接。
- Mod 上传必须限制文件类型、大小和运行中危险操作。
- 错误响应不暴露内部路径、堆栈、token、密码。
- 支持包不包含完整数据库、完整存档、完整 Mod 或 Steam session。
- Docker Socket 权限风险必须在部署和 UI 中提示。

## 后端验证

常用命令：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
```

SMAPI mod 编译：

```powershell
docker run --rm `
  -v "E:\stardew-server-anxi-panel\backend\internal\games\stardew_junimo\embedded\smapi-mod-src:/src" `
  -v "E:\stardew-anxi-panel\runtime\game:/game" `
  -w /src mcr.microsoft.com/dotnet/sdk:6.0 `
  dotnet build -c Release /p:GamePath=/game
```

改动后如果影响嵌入 DLL，必须说明是否需要重启 Stardew server 容器。
