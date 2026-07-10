# FESTIVAL-EVENT-1 触发节日活动 + JOJA-ROUTE-1 永久启用 Joja 路线

## 背景

用户提供了 Junimo 容器的"管理命令"表格（`website/docs/guide/getting-started.md` 里贴的一段），要求把其中 `!event`（启动当前的节日活动，卡住时用）和 `!joja IRREVERSIBLY_ENABLE_JOJA_RUN`（永久启用 Joja 路线，不可逆）这两条做成服务器控制页的按钮。

先做了一轮调研（Explore agent，只读），报告了一个关键结论：这两条都是 JunimoServer 的**游戏内聊天指令**，不是 SMAPI 控制台命令，`docs/10-junimo-rest-api.md` 确认没有对应 REST 端点。唯一可行路径是让面板自带的 `StardewAnxiPanel.Control` SMAPI Mod 模拟一条聊天消息触发它们——但这需要修改并重新编译内嵌 C# Mod，而当时的沙盒环境没有 .NET SDK，也不确定 Joja 的权限模型（`!joja` 要求触发者是 admin，服务器主机默认不是）。因此先用 `AskUserQuestion` 向用户确认了两件事：

1. 编译方式：用户回复"docker 里面装了环境，我已经打开了"。
2. Joja 权限方案：用户选择"先用 REST API 提升 host 为 admin（推荐）"。

确认后开始动手实现，实现过程中做了两轮关键验证（不是凭空假设）：

- 用 `System.Reflection.MetadataLoadContext`（.NET 官方包，纯反射不执行任何游戏代码）加载本机 `E:\stardew-anxi-panel\runtime\game\Stardew Valley.dll`，确认 `ChatBox.receiveChatMessage` 的真实签名是 `(Int64 sourceFarmer, Int32 chatKind, LanguageCode language, String message)`，且是**实例方法**。
- 用 `ilspycmd`（在同一个 dotnet SDK 容器里 `dotnet tool install -g ilspycmd --version 7.2.1.6295` 装的）反编译 `ChatBox.textBoxEnter(string)` 的真实源码，确认它内部会 `Game1.multiplayer.sendChatMessage(...)` 广播文本，再本地调用 `receiveChatMessage(Game1.player.UniqueMultiplayerID, 0, ..., text)`——这正是 JunimoServer 的 Harmony postfix 补丁挂载的方法，调用 `Game1.chatBox.textBoxEnter("!event")` 就能天然触发命令解析链路，和 `JunimoTestClient.ChatController.SendMessage` 官方测试客户端用的方式完全一致。
- 反编译 `Game1.chatBox` 的赋值点（`Game1.loadForNewGame`：`onScreenMenus.Add(chatBox = new ChatBox());`），确认它在 `!loadedGame` 判断之外无条件执行，不受 headless/dedicated server 模式影响，所以 `Game1.chatBox` 在 JunimoServer 进程里必然非空（World Ready 之后）。
- 读了上游 `RoleService.cs`：`IsPlayerAdmin` 只查一个显式赋权的字典，注释明确写"没有默认管理员赋权，只能通过 `ADMIN_STEAM_IDS`"；`IsServerHost`/`UnassignAdmin` 两处逻辑说明"dedicated server 的主机不应该是真实玩家"，但**没有**代码阻止给主机 ID 赋予 admin 角色。这证实了"先用 `POST /roles/admin` 提升主机"这条路径是可行且安全的（幂等，只影响面板自己控制的这台服务器进程的虚拟主机身份）。

## 改了什么

### 1. 内嵌 SMAPI Mod（`embedded/smapi-mod-src/ModEntry.cs`）

- `HandleCommand` 的 `switch` 新增两个分支：
  ```csharp
  case "trigger-event":
      TriggerFestivalEvent();
      break;
  case "enable-joja":
      EnableJojaRoute();
      break;
  ```
- 新增 `TriggerFestivalEvent()`：`Context.IsWorldReady && Game1.chatBox is not null` 检查后，`Game1.chatBox.textBoxEnter("!event")`。
- 新增 `EnableJojaRoute()`：同样的检查后，`Game1.chatBox.textBoxEnter("!joja IRREVERSIBLY_ENABLE_JOJA_RUN")`。方法上有注释说明这依赖后端先调用 `POST /roles/admin` 提升主机权限，否则会被 JunimoServer 拒绝（游戏内会提示"Only admins can enable Joja."，但这条提示只会写进聊天记录，Mod 侧和面板都看不到，所以必须提前保证权限，而不是靠事后检测失败）。

### 2. 触发节日活动（`console.go`）

- 新增 `Driver.TriggerFestivalEvent(ctx, instance) (*CommandRunResult, error)` 及可测试核心 `triggerFestivalEvent(instance)`：校验实例 `running`，写 `{"name":"trigger-event"}` 到 `control/commands/`，fire-and-forget，和 kick/say 完全同构，返回口径一致的"指令已提交"提示。

### 3. 永久启用 Joja 路线（新增 `joja.go`）

- `Driver.EnableJojaRoute(ctx, instance, confirm string) (*CommandRunResult, error)`，核心逻辑 `enableJojaRoute`：
  1. `strings.TrimSpace(confirm) != jojaConfirmText`（`"IRREVERSIBLY_ENABLE_JOJA_RUN"`）→ 直接拒绝，`confirm_mismatch`。**这是后端侧的硬校验**，不依赖前端弹窗做把关，即使有人绕过前端直接调 API 也一样要求精确文本。
  2. 校验实例 `running`。
  3. 新增 `findHostPlayerID(dataDir)`：复用已有的 `readPlayersFromControl`（`players.go`，读 `.local-container/control/players.json`），找 `IsHost == true` 的条目取 `UniqueMultiplayerID`；这个字段本来就是 `KickPlayer`/`PlayersPage` 早就在用的既有数据源，没有新增采集逻辑。
  4. 新增 `callRolesAdminAPI(ctx, ld, instance, hostID)`：完全照抄 `rendering.go` 的 `callRenderingAPI` 模式——`docker compose exec server curl -X POST http://localhost:$API_PORT/roles/admin?playerId=<hostID>`，浏览器和面板进程都不会接触 JunimoServer 的 `API_KEY`。复用了 `readJunimoAPIConfig`、`looksLikeJunimoAPIUnavailable` 这两个 `rendering.go` 里已有的辅助函数，没有重复实现。
  5. 提升成功后才写 `{"name":"enable-joja"}` 命令文件，返回明确提示"已将主机提升为管理员并提交 !joja 指令……此操作不可逆"。

### 4. Web 层（新增 `festival_handlers.go`）

- `handleFestivalEventTrigger`（`POST /api/instances/:id/festival/event`）、`handleJojaRouteEnable`（`POST /api/instances/:id/joja/enable`，请求体 `{"confirm": "..."}`），都走 `requireAdmin` → `loadInstance` → `reconcileInstanceState` → `loadDriver` → 接口断言（`festivalEventTrigger`/`jojaRouteEnabler`）的标准链路，和 `players_handlers.go` 的 `handlePlayerKick` 完全同构。
- 路由接入 `instance_handlers.go`：在 `password-status` 分支之前插入 `festival/event`、`joja/enable` 两条，`len(parts) == 3` 判断风格和现有 `players/kick` 一致。
- 审计日志新增 `festival_event_trigger`（metadata `"{}"`）、`joja_route_enable`（metadata `auditMetadata("confirmed", "true")`，故意不记录别的内容，因为没有敏感信息但又想在审计表里能一眼看出这是一次真正确认过的操作）。

## 影响文件

- `backend/internal/games/stardew_junimo/console.go`
- `backend/internal/games/stardew_junimo/joja.go`（新增）
- `backend/internal/web/festival_handlers.go`（新增）
- `backend/internal/web/instance_handlers.go`
- `backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ModEntry.cs`
- `backend/internal/games/stardew_junimo/embedded/smapi-mod/StardewAnxiPanel.Control.dll`（**已重新编译替换，见下方"如何验证"**）

前端改动见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md`。

## 如何验证

- `cd backend; go build ./... && go vet ./... && go test ./internal/games/stardew_junimo/... ./internal/web/...`：全绿。
- SMAPI Mod 重新编译：用户已启动 Docker Desktop，直接用文档命令（`docs/02-backend.md` 594-602 行）：
  ```powershell
  docker run --rm `
    -v "E:\stardew-server-anxi-panel\backend\internal\games\stardew_junimo\embedded\smapi-mod-src:/src" `
    -v "E:\stardew-anxi-panel\runtime\game:/game" `
    -w /src mcr.microsoft.com/dotnet/sdk:6.0 `
    dotnet build -c Release /p:GamePath=/game
  ```
  `Build succeeded. 0 Errors`（1 个历次已知的 ModBuildConfig analyzer 编译器版本 warning，无关）。编译产物 `bin/Release/net6.0/StardewAnxiPanel.Control.dll` 已用 md5 校验复制覆盖 `embedded/smapi-mod/StardewAnxiPanel.Control.dll`；`bin/`/`obj/` 已被 `.gitignore` 排除。
- 覆盖嵌入 DLL 后重新 `go build ./...` 确认新 DLL 能正常被 `//go:embed` 打包。
- 反编译验证（额外做的，不是常规流程）：用同一个 dotnet SDK 容器装 `System.Reflection.MetadataLoadContext` NuGet 包写了一个一次性反射探针程序，加载 `Stardew Valley.dll` 打印 `ChatBox.receiveChatMessage`/`textBoxEnter` 的真实签名；又装 `ilspycmd 7.2.1.6295`（兼容 net6.0 的版本，最新版只支持 net10.0）反编译出 `ChatBox.textBoxEnter`、`Game1.loadForNewGame` 里 `chatBox` 赋值点的真实源码。这两步是为了避免凭空假设 C# 内部实现导致改出一个编译通过但运行时不生效的 Mod。
- 前端 `cd frontend; npx tsc --noEmit -p . && npm run build` 通过。

**未做的验证**：没有在真实运行中的 JunimoServer 实例上做端到端联机测试（当天有节日时点"触发节日活动"→ 观察是否真的提前进入主线剧情倒计时；点"永久启用 Joja 路线"→ 观察 `!joja` 是否真的通过了权限校验、`IsCommunityCenterRun` 是否真的变成 `false`）。反编译验证只能确认"调用链路在理论上应该走通"，不能替代真机验证。强烈建议下一位维护者找一个测试实例走一遍完整流程。

## 下一步注意事项

- 两个操作都是 fire-and-forget，和 kick/say 一样拿不到 Mod 侧的真实执行结果，前端只能提示"指令已提交"。如果以后要做精确反馈，参考 `docs/backend-handoff/backend-handoff-2026-07-09.md` 里提到的方案（命令文件带 id，Mod 处理完写 `command-results/<id>.json`，后端轮询/等待读取），这次为了和已有模式保持一致故意没做。
- `EnableJojaRoute` 每次调用都会先打一次 `POST /roles/admin` 把主机提升为 admin，且 JunimoServer 的 `RoleService` 没有自动撤销机制（`UnassignAdmin` 明确禁止撤销 host 的角色）。如果以后做"管理员列表"这类展示功能，要注意主机会永久出现在名单里，这是预期行为不是 bug。
- `findHostPlayerID` 依赖 `players.json` 里有 `IsHost` 标记的条目，这要求 SMAPI Mod 至少运行过一次 `WritePlayers()`（`OnUpdateTicked` 每 120 tick 一次，`Context.IsWorldReady` 为真时才写）。如果服务器刚启动、世界还没完全加载完成，`findHostPlayerID` 会返回 `false`，`EnableJojaRoute` 会报 `host_unknown`，这是预期的保护行为，不需要额外重试逻辑。
- 如果以后 JunimoServer 上游给 `!event`/`!joja` 加了权限校验或改了参数格式，这里模拟聊天消息的字符串（`"!event"`、`"!joja IRREVERSIBLY_ENABLE_JOJA_RUN"`）需要跟着同步修改；`jojaConfirmText` 常量和上游要求的参数必须逐字一致，不要自行"优化"成别的提示文案。

# CABIN-STRATEGY-1 小屋策略设置分层（同日追加）

## 背景

用户贴出了一段 Junimo 容器"服务器运行时设置"参考表格（`MaxPlayers`/`CabinStrategy`/`SeparateWallets`/`ExistingCabinBehavior`/`VerboseLogging`/`AllowIpConnections`/`LobbyMode`/`ActiveLobbyLayout`/`AdminSteamIds`/`NetworkBroadcastPeriod`），并给出明确的设计判断：`CabinStrategy` 不应该只在新建存档页出现，而是"新建存档页可以选（简化二选一），服务器控制页也能改（完整高级设置）"，两边必须共用同一份配置来源，不能各自为政。用户确认走"直接实现"路径。

调研发现现状：`saves.go` 的 `WriteServerSettings` 此前把 `Server.CabinStrategy` 硬编码为 `"CabinStack"`、`Server.ExistingCabinBehavior` 硬编码为 `"KeepExisting"`，只在新建存档这一次性写入，没有任何运行时可调整入口；`server-settings.json` 只在 JunimoServer `server` 容器启动时读取，此前只有 `EnsureServerSettingsDefaults`（保证 `AllowIpConnections` 默认值，读取合并再写回，不覆盖已有 key）这一个"事后调整已有存档配置"的先例。

## 改了什么

### 1. 新建存档简化开关（`registry/types.go` + `saves.go`）

- `NewGameConfig` 新增 `CabinMode string`（json `cabinMode`），取值 `recommended|vanilla`。`normalizeCfg` 默认 `recommended`；`validateCfg` 新增校验必须是这两个值之一。
- `WriteServerSettings` 不再硬编码 `CabinStrategy`，改为按 `cfg.CabinMode` 派生：`recommended → "CabinStack"`，`vanilla → "None"`。`ExistingCabinBehavior` 仍固定写 `"KeepExisting"`（新建存档场景没有"已有小屋"概念，这个字段只在事后调整时才有意义）。

### 2. 服务器控制页完整高级设置的后端支撑（`saves.go`）

- 新增独立类型 `ServerRuntimeSettings{ CabinStrategy, ExistingCabinBehavior, NetworkBroadcastPeriod }`。
- `ReadServerRuntimeSettings(dataDir)`：读 `server-settings.json` 的 `Server` 段，文件不存在或字段缺失时兜底 `CabinStack`/`KeepExisting`/`1`，不报错。
- `UpdateServerRuntimeSettings(dataDir, settings)`：`validateServerRuntimeSettings` 校验通过后，只覆盖这三个 key，**保留**其它已有字段（`MaxPlayers`、`AllowIpConnections` 等）——这是刻意照抄 `EnsureServerSettingsDefaults` 的"读取合并再写回"模式，不是自己发明的新写法。
- 校验规则：`CabinStrategy` 必须是 `CabinStack`/`FarmhouseStack`/`None`；`ExistingCabinBehavior` 必须是 `KeepExisting`/`MoveToStack`；`NetworkBroadcastPeriod` 必须在 `1~10`（参考表格写"1=每个刻，3=原版"，10 是给自定义场景留的合理上限，不是上游硬限制，纯粹是面板侧的防呆范围）。

### 3. Web 层（新增 `server_runtime_settings_handlers.go`）

- `handleInstanceServerRuntimeSettings` 处理 `GET/PUT /api/instances/:id/config/server-runtime-settings`，结构完全照抄 `server_password_handlers.go` 的 `handleInstanceServerPassword`（`requireAdmin` → `loadInstance` → 读/校验/写 → 审计日志）。PUT 成功写审计 `instance_server_runtime_settings_update`（metadata 记录 `cabinStrategy`/`existingCabinBehavior`，不记录 `networkBroadcastPeriod`，因为审计表历来只记关键枚举字段方便扫描，不是遗漏）。
- 路由接入 `instance_handlers.go`，紧跟在既有的 `config/server-password` 分支之后。

## 影响文件

- `backend/internal/games/registry/types.go`
- `backend/internal/games/stardew_junimo/saves.go`
- `backend/internal/games/stardew_junimo/saves_test.go`
- `backend/internal/web/server_runtime_settings_handlers.go`（新增）
- `backend/internal/web/instance_handlers.go`

前端改动见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `CABIN-STRATEGY-1` 小节。

## 如何验证

- 新增测试：`TestWriteServerSettings_CabinModeRecommendedDefault`、`TestWriteServerSettings_CabinModeVanilla`、`TestWriteServerSettings_CabinModeInvalid`、`TestServerRuntimeSettings_ReadDefaultsWhenMissing`、`TestServerRuntimeSettings_UpdateAndReadRoundTrip`（校验 `MaxPlayers` 等既有字段在更新后仍然保留）、`TestServerRuntimeSettings_UpdateRejectsInvalid`。
- 修正了两处既有测试（`TestValidateNewGameConfig_ProfitMargin`、`TestValidateNewGameConfig_PetPreference`）：它们直接调用 `validateCfg()` 而不经过 `normalizeCfg()`，新增的 `cabinMode` 校验会导致它们失败，已在测试用例里显式补上 `CabinMode: "recommended"`。
- `cd backend; go build ./... && go vet ./... && go test ./...` 全绿。
- `cd frontend; npx tsc --noEmit -p . && npm run build` 通过。

**未做的验证**：没有连一个真实运行中的 JunimoServer 实例走一遍"新建存档选原版→确认小屋出现在真实农场位置→服务器控制页改回推荐→重启→确认小屋重新隐藏堆叠"的完整链路。`server-settings.json` 的字段名/取值都是照抄已有硬编码逻辑和用户提供的参考表格，没有找到 JunimoServer 上游源码逐一交叉验证 `FarmhouseStack`/`MoveToStack`/`NetworkBroadcastPeriod` 的行为细节（这三者此前在这个仓库里从未被写入过，`CabinStack`/`None`/`KeepExisting`/`AllowIpConnections` 之前已有硬编码或测试验证过）。建议下一位维护者用测试实例验证一次，尤其是 `FarmhouseStack`（"所有玩家从主农舍正门共用入口出"）和 `NetworkBroadcastPeriod=2/3`（广播频率降低是否会导致联机同步观感变差）。

## 下一步注意事项

- `ExistingCabinBehavior` 目前只能通过服务器控制页的 `UpdateServerRuntimeSettings` 真正改成 `MoveToStack`；新建存档阶段永远是 `KeepExisting`。如果以后要在新建存档页也暴露这个字段，需要重新评估语义（"已有小屋"在全新档案里不存在，暴露这个选项可能会让用户困惑）。
- `NetworkBroadcastPeriod` 的 `1~10` 范围是面板侧自定义的防呆上限，不是从 JunimoServer 源码或文档里找到的官方硬限制；如果后续找到官方文档给出不同范围，需要同步调整 `validateServerRuntimeSettings` 和前端 `<select>` 选项。
- 这组设置和 `SERVER_PASSWORD` 一样只在 `server` 容器**启动时**生效，`UpdateServerRuntimeSettings` 本身不会重启容器，也不会主动提示用户去重启——前端弹窗文案里写了提示，但如果以后新增"保存后自动询问是否重启"这类交互，需要注意这是一个可选的体验增强，不是本次范围。
