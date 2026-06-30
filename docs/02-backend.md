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
| Mods | `GET /api/instances/:id/mods`, 上传、删除、导出、玩家同步分类(`mods/sync-plan`、`mods/:modId/sync-classification`、`mods/sync-pack/export`)、Nexus 只读搜索(`mods/nexus/search`) |
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

### Mod 玩家同步包

`stardew_junimo` 提供 Mod 同步分类能力，逻辑全部在 `backend/internal/games/stardew_junimo/mod_sync.go`，不绕过 driver。

- `ModInfo` 新增 `syncKind`(`server_only` | `client_required` | `unknown`) 和可选 `syncNote`。`GET /api/instances/:id/mods` 在返回前会调用 `ApplyModSyncClassification` 补全这两个字段，前端不需要单独再拉一次分类。
- 分类持久化在面板自有文件 `.local-container/control/mod-sync.json`，绝不写入 Mod 自身的 `manifest.json`。未分类 Mod 默认 `unknown`；面板自带的 `StardewAnxiPanel.Control` 默认 `server_only`。
- `GET /api/instances/:id/mods/sync-plan`：返回全部已装 Mod 及分类统计（`serverOnly`/`clientRequired`/`unknown`）。
- `PUT /api/instances/:id/mods/:modId/sync-classification`：管理员专用，只写面板元数据，不受服务器运行状态限制。`:modId` 可以是文件夹名或 `UniqueID`（复用 `ResolveModFolder`，同 `DeleteMod` 的查找顺序）。
- `POST /api/instances/:id/mods/sync-pack/export`：导出全部 `syncKind == client_required` 的 Mod 为 ZIP，附带面板生成的 `player-sync-manifest.json`（导出时间 + uniqueId/name/version/folderName/syncKind 列表）。服务器运行中也允许导出。`StardewAnxiPanel.Control` 无论分类如何，始终被排除在导出之外（双重保险：默认分类 + 导出时强制跳过）。没有任何 Mod 命中 `client_required` 时返回 `400 no_sync_mods`。

涉及：`backend/internal/games/registry/types.go`、`backend/internal/games/stardew_junimo/mod_sync.go`、`backend/internal/games/stardew_junimo/mods.go`（抽出共用的 `addModDirToZip`）、`backend/internal/web/lifecycle_handlers.go`、`backend/internal/web/instance_handlers.go`。

测试覆盖：分类文件读写、默认 unknown/server_only、导出只含 client_required、导出排除控制 Mod、`ResolveModFolder` 路径安全，见 `backend/internal/games/stardew_junimo/mod_sync_test.go`。

`mods.go` 的 `ExportModsZip` 原本在循环失败但 `w.Close()` 成功时会因 `if err := w.Close(); err != nil` 遮蔽外层 `err`，把失败误判为成功；已修复为循环内失败直接 `return`、`Close()` 结果直接赋值给外层 `err`，与 `ExportModSyncPackZip` 写法一致。

`ExportModSyncPackZip` 原本固定写入 `%TEMP%\stardew-player-sync-pack.zip`，两个并发导出请求会互相覆盖/截断对方的 ZIP，且一个请求失败后的 `defer` 清理可能删掉另一个请求正在 `ServeFile` 的文件；已改为 `os.CreateTemp("", "stardew-player-sync-pack-*.zip")` 生成唯一临时路径，对外 `Content-Disposition` 文件名固定用新增的 `PlayerSyncPackFileName` 常量，与实际磁盘路径解耦。

`SetModSyncClassification` 原本对 `mod-sync.json` 是无锁的 load-modify-save，两个几乎同时的分类更新请求会让后写入的覆盖先写入的；已加上按 `dataDir` 维度的 `sync.Mutex`（`modSyncLockFor`）包住整个读改写流程，并把 `saveModSyncStore` 改成临时文件 + `os.Rename` 的原子写入，避免中途崩溃留下半截 JSON。

### Nexus Mods 只读搜索（第二阶段）

`backend/internal/games/stardew_junimo/nexus.go` 提供 Stardew Valley（`stardewvalley` game domain）只读搜索，不做下载/安装，只读 `NEXUS_API_KEY` 环境变量、不持久化任何配置。

- **API Key 缺失**：`SearchNexusMods` 在调用前先检查 `NEXUS_API_KEY`，未配置时返回哨兵错误 `ErrNexusAPIKeyMissing`，handler 据此返回 `503 nexus_api_key_missing`。
- **两种查询方式**：
  - 查询关键词若是纯数字，按精确 Mod ID 查询走官方文档化的 v1 REST 接口 `GET https://api.nexusmods.com/v1/games/stardewvalley/mods/{id}.json`（`apikey` 请求头鉴权）。
  - 其余关键词走 GraphQL v2（`https://api.nexusmods.com/v2/graphql`，与 nexusmods.com 网站搜索框同源）做关键词搜索。**注意**：Nexus 官方 v1 REST API 文档中没有任何关键词全文搜索接口（只有按 ID 查询、最近更新列表、MD5 查询），GraphQL v2 的具体字段名（`nexusGraphQLSearchQuery`/`nexusGraphQLResponse`，见 `nexus.go`）是根据公开资料推测的，**未用真实 API Key 验证过**；接入真实 Key 后如果解析失败，需要对照 Nexus 最新 GraphQL schema 调整这两处。
  - 结果数量上限 `nexusMaxResults = 20`。
- **请求安全**：固定 10 秒超时（`nexusRequestTimeout`）、固定 `User-Agent`；非 2xx 响应一律不读取/转发响应体，只保留状态码包成 `*NexusAPIError`，避免上游错误页（可能回显请求细节）泄露给前端；API Key 只通过请求头发送，从不出现在 URL 或错误信息里。
- **query 校验**：空查询（trim 后）返回 `ErrInvalidNexusQuery`，handler 映射为 `400 invalid_query`。
- **已安装匹配**：`ApplyNexusInstalledMatch(dataDir, results)` 读取本地已装 Mod，按 manifest `UpdateKeys` 中 `Nexus:<id>` 解析出的 `NexusModID` 做匹配，命中则把 `installed`/`installedFolderName`/`installedVersion` 填上；本阶段只判断"已安装"，不做版本新旧比较。
- **manifest 解析扩展**：`modManifest`/`registry.ModInfo` 新增 `UpdateKeys []string` 和 `NexusModID int`（由 `parseNexusModIDFromUpdateKeys` 从 `UpdateKeys` 里挑出 `Nexus:` 前缀的条目解析），`readModInfo` 在解析时一并填充，所以 `GET .../mods` 现有列表也会带上这两个新字段（向后兼容，新字段都是 `omitempty`）。

API：`GET /api/instances/:id/mods/nexus/search?q=关键词`，`requireAuth`（任意登录用户，普通玩家也能用，不需要管理员权限）。错误码映射：

| 场景 | HTTP | code |
| --- | --- | --- |
| 未配置 `NEXUS_API_KEY` | 503 | `nexus_api_key_missing` |
| 空查询 | 400 | `invalid_query` |
| Nexus 返回 404（按 ID 查询未命中） | 404 | `nexus_mod_not_found` |
| Nexus 返回 401/403 | 502 | `nexus_unauthorized` |
| Nexus 返回 429 | 429 | `nexus_rate_limited` |
| 其他非 2xx / 网络错误 | 502 | `nexus_request_failed` |

涉及文件：`backend/internal/games/registry/types.go`（`ModInfo` 新增字段）、`backend/internal/games/stardew_junimo/mods.go`（manifest 解析扩展）、`backend/internal/games/stardew_junimo/nexus.go`（新文件）、`backend/internal/web/instance_handlers.go`（路由）、`backend/internal/web/lifecycle_handlers.go`（`handleModNexusSearch`/`writeNexusError`）。

测试覆盖（`backend/internal/games/stardew_junimo/nexus_test.go`）：API Key 缺失、空 query、ID 查询结果解析、关键词搜索结果解析、结果数量上限裁剪、非 2xx 状态码映射为 `*NexusAPIError`（ID 查询和关键词搜索两条路径都覆盖）、API Key 不泄露（用 httptest mock 一个把 Key 回显到错误响应体里的恶意/有 bug 的上游，断言最终错误信息不包含该 Key）、`UpdateKeys`/`NexusModID` 解析与 `installed` 匹配。

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
