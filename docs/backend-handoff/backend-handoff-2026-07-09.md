# PLAYERS-KICK-1 踢出玩家 + PASSWORD-STATUS-1 加入密码设置/认证状态

## 背景

用户要求实现"踢出玩家"和"设置玩家加入密码"这两个此前一直标注"待接入"的玩家管理功能（`frontend/src/games/stardew/pages/PlayersPage.tsx` 里的踢出按钮、`website/docs/handbook/players.md` 的说明）。

先做了一轮调研（读 `E:\codex\junimo-server-upstream` 上游 JunimoServer 源码 + `docs/10-junimo-rest-api.md`），确认：

- **密码**：只有容器启动时读取一次的环境变量 `SERVER_PASSWORD`（`Env.cs`），登录走游戏内聊天指令 `!login <password>`（`PasswordProtectionService`/`LoginCommand.cs`），上游 REST API 只有 `GET /auth`（查看是否启用/在线认证人数）和 `POST /auth/timeout`（改认证超时秒数），**没有运行时改密码的接口**。
- **踢人**：上游只有游戏内聊天指令 `!kick <name>`（`KickCommand.cs`，管理员权限、禁止踢 host、底层 `Game1.server.kick(uniqueId)`），**REST API 完全没有 `/kick` 端点**，全仓库 grep 确认。

因此踢人无法走"代理 JunimoServer REST API"这条路（rendering.go/GetRenderingFPS 那种模式），改为复用面板自己维护的 `StardewAnxiPanel.Control` SMAPI Mod（`backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ModEntry.cs`）——这个 Mod 本来就有一个"命令队列"机制（`ConsumeCommands`/`HandleCommand`，每 120 tick 轮询 `<dataDir>/control/commands/*.json`），喊话（`say`）已经用这套机制实现（`writePanelBroadcastCommand`），这次给它加了一个 `kick` 分支。

密码这块因为上游确实没有运行时改密码的接口，只做了两件事：改 `.env` 的 `SERVER_PASSWORD`（下次启动 server 容器生效）+ 只读代理 `GET /auth` 显示当前认证状态。

## 改了什么

### 1. 踢出玩家

- `console.go`：把原来专属 broadcast 的 `writePanelBroadcastCommand` 拆成通用的 `writePanelCommand(dataDir, name string, payload map[string]string) error`，`writePanelBroadcastCommand` 变成对它的一层薄封装，行为完全不变。
- `console.go` 新增 `Driver.KickPlayer(ctx, instance, uniqueMultiplayerID, name string) (*CommandRunResult, error)` 及其可测试核心 `kickPlayer(...)`：
  - 校验 `uniqueMultiplayerID` 非空、实例必须 `running`。
  - 写入 `{"name":"kick","payload":{"uniqueMultiplayerId":"...","name":"..."},"createdAt":...}` 到 `control/commands/`。
  - **fire-and-forget**：不等待 Mod 侧真实执行结果，立即返回 `Output: "踢出指令已提交，控制模组会在游戏 tick 中处理；无法踢出主机玩家。"`（和 `say` 的 UX 完全一致，Go 层本来就拿不到 Mod 侧的执行反馈）。
- `embedded/smapi-mod-src/ModEntry.cs`：
  - `HandleCommand` 的 `switch` 新增 `case "kick":`，解析 `payload.uniqueMultiplayerId`，调用新方法 `KickPlayer(string uniqueMultiplayerId)`。
  - 新增 `KickPlayer` 方法：`Context.IsWorldReady` 检查 → `long.TryParse` 解析目标 ID → 在 `Game1.getOnlineFarmers()` 里按 `UniqueMultiplayerID` 查找目标 → 目标不存在时忽略（写 `WriteStatus`）→ 目标是 `Game1.MasterPlayer`（host）时忽略并 `Monitor.Log` 警告（**这是踢人唯一的服务端保护**，Go 层和 web 层都没有再做一次 host 判断，因为 Go 层不一定实时知道谁是 host）→ 否则 `try { Game1.server?.kick(targetId); }`，成功/失败都写 `WriteStatus` 和 `Monitor.Log`。

### 2. 加入密码 + 密码保护状态

- 新增 `backend/internal/web/server_password_handlers.go`：
  - `GET/PUT /api/instances/:id/config/server-password`：`handleInstanceServerPassword`，管理员权限，模式完全照抄 `ports_handlers.go` 的 `handleInstanceVNCConfig`（GET 读 `.env`，PUT 用 `sjconfig.UpdateEnvFile` 写；`.env` 不存在时先用 `EmptyEnvTemplate()` 兜底再覆盖）。密码长度上限 128 字符。**GET 会把明文密码原样返回给管理员**（和 `install_handlers.go` 里 `STEAM_PASSWORD`/`VNC_PASSWORD` 的既有惯例一致，不是这次新引入的模式）。
  - `GET /api/instances/:id/password-status`：`handleInstancePasswordStatus`，管理员权限，要求实例 `running`，调用 driver 的 `GetAuthStatus`。
- 新增 `backend/internal/games/stardew_junimo/auth_status.go`：`Driver.GetAuthStatus(ctx, instance)`，完全照抄 `rendering.go` 的 `callRenderingAPI` 模式——**不是从面板后端直接 HTTP 请求 JunimoServer**，而是 `docker compose exec server curl ...` 在 server 容器内部发起请求（`http://localhost:$API_PORT/auth`），这样浏览器和面板进程都不会接触到 Junimo 的 `API_KEY`。返回 `AuthStatusResult{Enabled, AuthenticatedCount, PendingCount, TimeoutSeconds, MaxAttempts}`，字段名和 `docs/10-junimo-rest-api.md` 3.5 节的 `GET /auth` 文档一致。JunimoServer API 未就绪时返回 `junimo_api_unavailable`（502）。
- 路由（`instance_handlers.go`）新增三条：
  - `POST /api/instances/:id/players/kick`（`players_handlers.go` 的 `handlePlayerKick`）
  - `GET/PUT /api/instances/:id/config/server-password`
  - `GET /api/instances/:id/password-status`
- 审计日志：`player_kick`（`uniqueMultiplayerId`、`name`）、`instance_server_password_update`（**只记录 `passwordSet: true/false`，不记录明文密码**——这是刻意的，`auditMetadata` 虽然内部会走 `docker.RedactString`，但保险起见没有把密码原文传进去）。

## 影响文件

- `backend/internal/games/stardew_junimo/console.go`
- `backend/internal/games/stardew_junimo/auth_status.go`（新增）
- `backend/internal/web/players_handlers.go`
- `backend/internal/web/server_password_handlers.go`（新增）
- `backend/internal/web/instance_handlers.go`
- `backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ModEntry.cs`
- `backend/internal/games/stardew_junimo/embedded/smapi-mod/StardewAnxiPanel.Control.dll`（**已重新编译替换，见下方"如何验证"**）

前端改动见 `docs/frontend-handoff/frontend-handoff-2026-07-09.md` 的 `PLAYERS-KICK-1`/`PASSWORD-STATUS-1` 部分。

## 如何验证

- `cd backend; GOCACHE=... go build ./... && go test ./...`：全绿（`internal/games/stardew_junimo`、`internal/web` 等全部 package）。
- SMAPI Mod 重新编译：本机只装了 .NET runtime 没装 SDK，按 `docs/02-backend.md` 文档命令用 Docker 编译（用户手动启动了 Docker Desktop 后执行）：
  ```powershell
  docker run --rm `
    -v "E:\stardew-server-anxi-panel\backend\internal\games\stardew_junimo\embedded\smapi-mod-src:/src" `
    -v "E:\stardew-anxi-panel\runtime\game:/game" `
    -w /src mcr.microsoft.com/dotnet/sdk:6.0 `
    dotnet build -c Release /p:GamePath=/game
  ```
  `Build succeeded. 0 Errors`（1 个历次已知的 ModBuildConfig analyzer 编译器版本 warning，无关）。编译产物 `bin/Release/net6.0/StardewAnxiPanel.Control.dll` 已用 md5 校验复制覆盖 `embedded/smapi-mod/StardewAnxiPanel.Control.dll`；`bin/`/`obj/` 已被 `.gitignore` 排除，`git status` 只看到 `ModEntry.cs` 和嵌入 DLL 两处改动。
- 覆盖嵌入 DLL 后重新 `go build ./...` 确认 Go 二进制能正常打包新 DLL（`//go:embed`）。

**未做的验证**：没有真机部署到真实运行中的 JunimoServer 实例上做端到端联机测试（设密码→重启→玩家 `!login`→管理员踢人→查看认证状态这条完整链路）。`Game1.server?.kick(...)` 的运行时表现、`GET /auth` 响应字段的真实大小写/类型，只依据上游 JunimoServer 源码（`E:\codex\junimo-server-upstream`）和已整理文档确认，未在真实游戏进程里跑过。建议下一位维护者找一个测试实例走一遍完整流程。

## 下一步注意事项

- 踢人是 fire-and-forget，Go 层永远拿不到"踢成功了/目标已离线/因为是主机被拒绝"这类精确结果，只能提示"指令已提交"。如果以后要做精确反馈，需要新增一个命令结果回传通道（例如命令文件带 `id`，Mod 处理完写 `control/command-results/<id>.json`，后端轮询/等待读取），这次为了和已有的 `say` 保持一致故意没做，不要在没有强需求时加这个复杂度。
- 密码保存后不会自动重启 server 容器，也没有在 UI 强提醒"必须重启才生效"之外做更多（比如自动弹出"是否立即重启"）。是否需要更强的提醒/自动化由后续迭代决定。
- 踢人的唯一服务端保护（不能踢 host）是在 SMAPI Mod C# 侧做的，Go/web 层没有重复校验 targetId 是否是 host（因为 Go 层不一定能准确判断谁是当前 host，尤其是 host 中途切换的场景）；如果以后要在 Go 层也做一层保护，需要先确认 `PlayersResult.Players[].IsHost` 的实时性是否够用。
- SMAPI Mod 编译环境依赖本机 `E:\stardew-anxi-panel\runtime\game` 这份游戏文件（用于 `Pathoschild.Stardew.ModBuildConfig` 解析游戏程序集），如果换开发机或该目录被清理，需要重新准备一份游戏安装目录再执行编译命令。
