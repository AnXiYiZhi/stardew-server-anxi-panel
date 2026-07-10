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

# APPROVE-PENDING-AUTH-1 批准待认证玩家（同日追加）

## 背景

用户提出方案：让面板自带的 `StardewAnxiPanel.Control` SMAPI 模组反射调用 JunimoServer 内部单例 `PasswordProtectionService.TryAuthenticate(long playerId, string password)`，用服务器自己配置的真实密码代替玩家完成一次认证，从而在面板上"批准"一个正卡在隔离小屋里的待认证玩家。

先确认了这确实是唯一可行路径：`docs/10-junimo-rest-api.md` 3.5 节里 JunimoServer 官方 REST API 关于密码保护只有 `GET /auth`（五个计数字段）和 `POST /auth/timeout`，没有批准/待认证玩家名单接口；`FESTIVAL-EVENT-1`/`JOJA-ROUTE-1` 用过的"模拟游戏内聊天指令"这条路对本功能也不适用——读了上游 `LoginCommand.cs` 确认 `!login` 是按`passwordProtectionService.TryAuthenticate(msg.SourceFarmer, password)` 鉴权，即认证目标是"这条聊天消息的发送者"，主机在聊天框输入 `!login` 只能给自己认证（而主机本来就完全绕过认证检查），无法代替其他玩家完成认证。

读了 JunimoServer 上游源码 `E:\codex\junimo-server-upstream\mod\JunimoServer\Services\PasswordProtection\PasswordProtectionService.cs` 确认关键事实：`TryAuthenticate`/`IsPlayerAuthenticated` 是 public 方法，密码校验成功后内部会处理"传送出隔离小屋、处理跨日过渡"等全部逻辑，模组侧不需要重新实现；单例存放在 `private static PasswordProtectionService _instance` 字段，没有公开 getter，必须反射拿实例；密码来自 `Environment.GetEnvironmentVariable("SERVER_PASSWORD")`，控制模组和 JunimoServer 运行在同一容器/进程，可以直接读同一环境变量而不需要额外反射密码本身。这是控制模组第一次真正反射进 JunimoServer 的私有实现（不是公开契约），存在"JunimoServer 升级后悄悄改字段名/方法签名导致反射静默失效"的固有风险，因此用 `AskUserQuestion` 向用户确认了两个设计决策：

1. 是否要在模组启动时做一次性"反射能力自检"，结果写入 `status.json`，供面板据此提前禁用"批准"按钮——用户选择"加自检标志（推荐）"。
2. 待认证玩家在 UI 上怎么呈现——用户选择"新增独立待认证玩家卡片"，不复用/不改动现有在线玩家表。

## 改了什么

### 1. 新增反射桥（`embedded/smapi-mod-src/PasswordProtectionBridge.cs`）

独立文件隔离反射逻辑（参考 `ControlContract.cs` 已经把数据契约拆开的先例）。`Initialize(IMonitor)` 只在 `ModEntry.OnGameLaunched` 里、`isJunimoRuntime` 判定为真之后调用一次：遍历 `AppDomain.CurrentDomain.GetAssemblies()` 按程序集名 `JunimoServer` 定位已加载程序集（不新增编译期引用，纯运行时反射）→ `GetType("JunimoServer.Services.PasswordProtection.PasswordProtectionService")` → `GetField("_instance", NonPublic|Static)` → `GetMethod("TryAuthenticate", (long,string))`/`GetMethod("IsPlayerAuthenticated", (long))` → `AuthenticationResult` 的 `Success`/`Message`/`ShouldKick` 三个 `PropertyInfo`。每一步失败都记录具体哪一步失败到 `_detail`，全程 try/catch，异常绝不上抛导致模组崩溃。`Available`/`Detail` 供外部读取自检结果——**这只能证明"启动时反射链路建立成功"，不保证"每次调用都成功"或"JunimoServer 内部行为语义不变"**，这一点在代码注释和本文档里都写清楚了。

### 2. `ModEntry.cs` 接入反射桥

- 新增 `passwordBridge` 字段，`OnGameLaunched` 里 `isJunimoRuntime` 判定之后调用 `passwordBridge.Initialize(Monitor)`。
- `WriteStatus` 构造 `RuntimeStatus` 时追加 `PasswordBridgeAvailable`/`PasswordBridgeDetail`（不改 `WriteStatus` 方法签名，所有现有调用点自动带上最新自检结果，这是最小侵入的做法）。
- `WritePlayers()`/`BuildPlayerInfo` 为每个在线玩家追加 `IsAuthenticated`（`bool?`）：host 直接视为 `true`（host 本就完全绕过认证），其余玩家反射调用 `passwordBridge.IsPlayerAuthenticated(...)`，桥不可用或单次查询失败时为 `null`（序列化时因 `ContractJson.Options` 的 `WhenWritingNull` 自动省略该字段，旧前端/旧数据不受影响）。
- `HandleCommand` 新增 `case "approve-auth"` → 新方法 `ApproveAuth(string uniqueMultiplayerId)`，完全模仿现有 `KickPlayer`（`ModEntry.cs` 第 715 行附近）的校验结构：`Context.IsWorldReady` → 桥可用性（不可用时提前返回并 `Monitor.Log` 警告）→ `long.TryParse` → `Game1.getOnlineFarmers()` 在线查找 → 排除 host → try/catch 调用 `passwordBridge.TryAuthenticate(targetId, 环境变量里的真实密码)` → `WriteStatus` 反馈成功/失败文案。`AuthenticationResult.ShouldKick`（超限踢出）字段有意丢弃不处理，因为 JunimoServer 内部的 `TryAuthenticate` 在超限时会自己调用 `Game1.server.kick(...)`，模组侧不需要重复实现。

### 3. `ControlContract.cs` 新增字段

`RuntimeStatus.PasswordBridgeAvailable`(bool)/`PasswordBridgeDetail`(string)；`PlayerInfo.IsAuthenticated`(`bool?`)。

### 4. Go 后端

- `players.go`：`PlayerInfo`/`playerCacheItem`/`controlPlayersFile` 新增 `IsAuthenticated *bool`。用指针而非 `bool`：区分"旧版本 DLL 没有这个字段"（`nil`）和"确实未认证"（`false`），避免旧实例的玩家被误判为全员待认证。`mergePlayerRoster` 从本次在线快照透传该字段到 roster 条目；`playerInfoFromCacheItem` 在 `status != "online"` 时把 `isAuthenticated` 强制置 `nil`——"待认证"是纯粹的瞬时在线状态，不应该进入离线花名册的长期缓存语义（否则玩家下线后再上线前，`isAuthenticated` 会残留上一次在线时的旧值，具有误导性）。
- 新增 `player_auth.go`：
  - `readPasswordBridgeStatus(dataDir) PasswordBridgeStatus`：只读 `control/status.json` 里模组写入的 `passwordBridgeAvailable`/`passwordBridgeDetail`，文件缺失/字段缺失/解析失败都返回 `Available:false`，不报错。独立于 `lifecycle.go` 现有的 `readSMAPIStatus`（后者只关心 `state` 字段，用于生命周期启动进度日志），两者职责不同不应该合并。
  - `approveAuth(instance, uniqueMultiplayerID) (*CommandRunResult, error)`：完全模仿 `console.go` 的 `kickPlayer`——trim 校验非空 → 校验 `instance.State == running` → 校验 `readPasswordBridgeStatus(...).Available`（不可用则返回 `CommandError{Code:"password_bridge_unavailable"}`，这是防御性校验，不能只依赖前端提前禁用按钮，否则绕过前端直接调 API 时后端毫无防护）→ `writePanelCommand(..., "approve-auth", {"uniqueMultiplayerId":...})` → 返回和 kick/say 一致的 fire-and-forget 乐观提示。
  - `Driver.ApproveAuth` 薄封装，供 web 层接口断言调用。
- `auth_status.go`：`AuthStatusResult` 新增 `PasswordBridgeAvailable`/`PasswordBridgeDetail`（`omitempty`）。`GetAuthStatus` 只在 `GET /auth` REST 调用成功路径的末尾追加一行读取 `readPasswordBridgeStatus`，**不改动现有错误分支的语义**——服务器未运行/JunimoServer REST 未就绪时整体请求仍然失败，前端按"待认证功能不可用"处理，这和产品预期一致（服务器没跑起来时本来就不该显示待认证列表），避免为了传递一个诊断字段而大改现有稳定的错误处理路径。

### 5. Web 层

`players_handlers.go` 新增 `playerAuthApprover` 接口和 `handlePlayerApproveAuth`（`POST /api/instances/:id/players/approve-auth`），完全照抄 `handlePlayerKick`（`requireAdmin` → `loadInstance` → `reconcileInstanceState` → `loadDriver` → 接口断言 → 解析 body → 调用 → `CommandError` 按 `Code` 映射 HTTP 状态：`server_not_running`/`password_bridge_unavailable` → 409，`not_supported` → 501 → `auditLog` → `writeJSON`）。审计日志 action 名 `player_approve_auth`（metadata: `uniqueMultiplayerId`）。路由接入 `instance_handlers.go`，紧邻现有 `players/kick` 分支（用 `perl -0777 -pi` 精确字节匹配插入的，因为 `Edit` 工具在这个文件上连续几次因为缩进空白字符不匹配而失败，改用脚本插入后一次成功，供后续遇到同类问题时参考）。

## 影响文件

- `backend/internal/games/stardew_junimo/embedded/smapi-mod-src/PasswordProtectionBridge.cs`（新增）
- `backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ControlContract.cs`
- `backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ModEntry.cs`
- `backend/internal/games/stardew_junimo/embedded/smapi-mod/StardewAnxiPanel.Control.dll`（**已重新编译替换**）
- `backend/internal/games/stardew_junimo/players.go`
- `backend/internal/games/stardew_junimo/player_auth.go`（新增）
- `backend/internal/games/stardew_junimo/auth_status.go`
- `backend/internal/web/players_handlers.go`
- `backend/internal/web/instance_handlers.go`

前端改动见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `APPROVE-PENDING-AUTH-1` 小节。

## 如何验证

- SMAPI Mod 重新编译（本机已有 Docker 和挂载的游戏目录）：
  ```powershell
  docker run --rm `
    -v "E:\stardew-server-anxi-panel\backend\internal\games\stardew_junimo\embedded\smapi-mod-src:/src" `
    -v "E:\stardew-anxi-panel\runtime\game:/game" `
    -w /src mcr.microsoft.com/dotnet/sdk:6.0 `
    dotnet build -c Release /p:GamePath=/game
  ```
  `Build succeeded. 0 Errors`（1 个历次已知的 ModBuildConfig analyzer 版本 warning，无关；纯反射不新增 JunimoServer.dll 引用，编译期完全不依赖真实 JunimoServer 类型是否存在）。编译产物已用 md5 校验（编译前后 DLL 哈希确认不同）复制覆盖 `embedded/smapi-mod/StardewAnxiPanel.Control.dll`。
- 新增测试：`player_auth_test.go`（`TestReadPasswordBridgeStatus_MissingFile`/`_ParsesFields`/`_UnavailableWithDetail`、`TestApproveAuth_RequiresPlayerID`/`_ServerNotRunning`/`_RequiresPasswordBridgeAvailable`/`_WritesCommandFile`）、`auth_status_test.go`（`TestGetAuthStatus_MergesPasswordBridgeStatus`/`_PasswordBridgeUnavailableWhenStatusMissing`）、`players_test.go` 新增 `TestReadPlayersFromControlParsesIsAuthenticated`（true/false/字段缺失三种输入）、`TestListPlayersOfflinePlayersHideIsAuthenticated`（玩家下线后 `isAuthenticated` 从 `false` 强制变回 `nil`，验证不会残留旧值）。
- `cd backend; go build ./... && go vet ./... && go test ./internal/games/stardew_junimo/... ./internal/web/...` 全绿；`go test ./...` 全仓库回归也全绿。
- 前端 `cd frontend; npx tsc --noEmit -p . && npm run build` 通过。

**未做的验证**：没有在开启 `SERVER_PASSWORD` 的真实运行实例上做端到端联机测试。反射桥的自检机制只能验证"启动时类型/字段/方法能被反射定位到"，不能验证"调用后行为是否符合预期"（例如 `TryAuthenticate` 内部的跨日过渡处理、`WarpToDestination` 的小屋查找逻辑是否在这个特定环境下正常工作）。强烈建议下一位维护者找测试实例走一遍完整链路：开密码 → 客户端连接但不 `!login` → 确认面板"待认证玩家"卡片显示该玩家且反射桥自检为可用 → 点击批准 → 确认玩家立即被传送出隔离小屋、`isAuthenticated` 变为 `true`；并顺带验证"批准已离线/不存在的联机 ID"和"重复批准已认证玩家"两个边界不会导致模组异常或崩溃。

## 下一步注意事项

- 反射目标是 JunimoServer **私有静态字段**和未公开为稳定契约的内部服务，不是公共 API。JunimoServer 后续版本一旦重命名/重构该单例、改方法签名，反射会静默失效——`PasswordBridgeAvailable` 只能捕捉"启动时反射链路建立失败"这一种情况。如果以后这个功能突然不生效了，第一步应该看 `control/status.json` 的 `passwordBridgeDetail` 诊断信息（会写清楚是类型找不到、字段找不到还是方法签名不匹配），而不是凭空猜测。
- 和 kick/say/`!event`/`!joja` 一样是 fire-and-forget，没有做"命令文件带 id + `command-results/<id>.json`"精确反馈通道（历史文档提过这个方案但一直没实现）。这次评估后决定继续不做：批准操作的实际失败面比 kick 更窄，基本只有"反射桥不可用"这一种业务失败模式，而这一种已经被启动期自检提前拦截并禁用按钮，不需要逐次反馈来定位失败原因。
- 待认证玩家能否被面板看到，依赖控制模组已经运行过至少一次 `WritePlayers()`（`OnUpdateTicked` 每 120 tick、约 2 秒一次，`Context.IsWorldReady` 为真时才写）；玩家刚连接的几秒钟内，"待认证玩家"卡片可能还没显示出来，这是正常的轮询延迟，不需要额外处理。
- 如果以后要给"批准"加二次密码强度校验、审批理由记录等更复杂的审批流程，现在的实现是最小化的"一键批准"，没有为这些假设中的需求预留任何字段或抽象。

# PLAYERS-BAN-1 封禁玩家 + 玩家行操作按钮精简（同日追加）

## 背景

用户看着玩家管理页"在线玩家"表格每行的图标按钮截图，要求精简：原本 3 个图标（恒禁用的"发送消息"占位、可用的"踢出"、恒禁用的"更多操作"占位）只保留"踢出"，图标换成"管理操作"卡片"踢出玩家"用的那张真实 PNG（`icon_players_action_boot_image2.png`），旁边加一个新按钮，图标复用"封禁玩家"的图标（`icon_players_action_ban_image2.png`）。

第一轮讨论把新按钮定为"取消认证"（`APPROVE-PENDING-AUTH-1` 批准认证的反操作）。深入调研 JunimoServer 上游 `PasswordProtectionService.cs` 后向用户澄清了一个关键事实：认证状态是纯内存运行时字典（`_playerAuthData`），玩家每次连接（`OnFarmhandRequest`）都会被无条件创建一条全新的 `Unauthenticated` 记录，断线重连或容器重启都会自动重置——也就是说认证根本不持久化，"取消认证"在语义上唯一有意义的实现方式就是让当前连接结束（也就是踢出）。额外做一个"原地撤销认证但不踢人"的反射写操作，不仅要新增对 JunimoServer 私有可变字典（`_playerAuthData`）的写操作（比已实现的 `TryAuthenticate` 只读/单向调用风险高得多），实际运行效果还很差：如果服务器配置了 `AuthTimeoutSeconds > 0`（常见配置），JunimoServer 自己的 `OnUpdateTicked` 逻辑会在下一次 tick 几乎立刻把这个玩家自动踢出，效果等同踢出但绕了个大弯；如果 `AuthTimeoutSeconds = 0`，玩家会静默卡在原地无法移动/聊天（消息被过滤但没有传送回隔离小屋，也收不到"请重新登录"提示），体验比直接踢出更差。用户确认放弃这个方向，转而问"上游有没有拉黑功能，改成拉黑玩家，拒绝他的加入"。

调研 JunimoServer 上游源码找到了真正的封禁能力：`Services/Commands/BanCommand.cs`/`UnbanCommand.cs`/`ListBansCommand.cs` 三个聊天指令 `!ban <名字>`/`!unban <id|名字>`/`!listbans`，和已实现的 `!joja` 是同一套模式——要求触发者持有 admin 角色（`RoleService.IsPlayerAdmin`），底层调用 vanilla `Game1.server.ban(uniqueMultiplayerID)`，写入 `Game1.bannedUsers`；`BanCommand.cs` 里 `IsServerHost(targetFarmer.UniqueMultiplayerID)` 会被上游自己拒绝并提示"You can't ban the server host."，host 保护由上游保证，不需要我们重复实现。`ModHelperExtensions.FindPlayerIdByFarmerNameOrUserName` 用 `Game1.getAllFarmers().FirstOrDefault(f => f.Name == name || GetFarmerUserNameById(id) == name)` 按名字匹配，`getAllFarmers()` 同时包含在线和离线的 farmhand——离线玩家也能被封禁，这正好对应玩家管理页"管理操作"卡片里已经存在但一直禁用、标着"待接入"的"封禁玩家"卡片，只是没接后端。

唯一没能确认的关键事实：`Game1.bannedUsers` 是否随存档持久化，还是纯进程内存（容器重启即丢失）——全仓库搜索确认 JunimoServer 自己的代码里，`bannedUsers` 只在这三个命令文件里被引用，没有任何写盘持久化逻辑；这本身是 vanilla Stardew Valley 的字段，要彻底确认是否被 `SaveGame.cs` 序列化/反序列化，需要反编译游戏本体 DLL（用 `FESTIVAL-EVENT-1`/`JOJA-ROUTE-1` 时验证过的 `MetadataLoadContext`+`ilspycmd` 方法），这一步在规划阶段无法执行（不能跑 docker），需要放到实现阶段做。就这个不确定性征询了用户意见：面板自己做持久化补偿（新增数据库表记录封禁名单，每次启动后自动重新执行 `!ban`）还是先做简单接通接受重启可能失效——**用户选择了后者**："先做简单接通，接受重启可能失效"，不做面板侧持久化，只在 UI 确认弹窗里如实提示这个限制。

## 改了什么

### 1. `embedded/smapi-mod-src/ModEntry.cs`

`HandleCommand` 新增 `case "ban":`，解析 `payload["name"]`，调用新方法 `BanPlayer(string name)`：

```csharp
private void BanPlayer(string name)
{
    if (!Context.IsWorldReady || Game1.chatBox is null)
    {
        WriteStatus("command", "Ban command ignored because the world is not ready.");
        return;
    }

    name = SanitizeChatText(name, 60);
    if (string.IsNullOrWhiteSpace(name))
    {
        WriteStatus("command", "Ban command ignored because the target player name was empty.");
        return;
    }

    Game1.chatBox.textBoxEnter($"!ban {name}");
    WriteStatus("command", $"Ban command sent for player {name}.");
    Monitor.Log($"Ban command sent for player {name}.", LogLevel.Info);
}
```

结构完全模仿 `EnableJojaRoute()`（模拟聊天指令），不是模仿 `KickPlayer`（直接调用游戏 API）——因为 `!ban` 和 `!joja` 一样需要 admin 权限，且都是通过 `Game1.chatBox.textBoxEnter(...)` 触发 JunimoServer 挂在 `ChatBox.receiveChatMessage` 上的 Harmony 补丁命令解析链路。模组侧不做权限提升，权限提升由 Go 后端在写命令文件之前先完成（和 `EnableJojaRoute` 的分工一致）。复用了已有的 `SanitizeChatText` 辅助方法做防御性清理。

不需要改 `ControlContract.cs`：`PanelCommand.Payload` 已经是通用的 `Dictionary<string, JsonElement>`。

### 2. 新增 `backend/internal/games/stardew_junimo/ban.go`

`Driver.BanPlayer(ctx, instance, name, uniqueMultiplayerID)` → 核心 `banPlayer`：校验名字非空 → 校验实例 running → **完全复用** `joja.go` 已有的 `findHostPlayerID`/`callRolesAdminAPI`（这两个函数本来就是包级别可复用，不是 Joja 专属）把主机提升为 admin → `writePanelCommand(dataDir, "ban", {"name":...})`。返回文案明确写"如果服务器容器重启，此封禁可能失效，需要重新操作"，不是遗漏。

### 3. Web 层（`players_handlers.go`/`instance_handlers.go`）

新增接口 `playerBanner` 和请求体 `banPlayerRequest{Name, UniqueMultiplayerID}`（`UniqueMultiplayerID` 只用于审计日志，不参与实际封禁逻辑，因为 `!ban` 本身按名字匹配）；`handlePlayerBan` 完全照抄 `handlePlayerKick` 骨架，`CommandError.Code` 映射 `server_not_running`/`host_unknown`/`junimo_api_unavailable` → 409，`invalid_player` → 400，`not_supported` → 501；审计日志 `player_ban`（metadata: `name`、`uniqueMultiplayerId`）。路由接入 `instance_handlers.go`，紧邻 `players/kick`/`players/approve-auth` 分支（**同样用 `perl -0777 -pi` 按精确字节内容插入**，因为 `Edit` 工具在这个文件上又一次因缩进空白字符匹配问题失败，这是继 `APPROVE-PENDING-AUTH-1` 之后第二次在这个文件上踩这个坑，下次遇到同样问题应该直接用脚本化插入，不要反复尝试 `Edit`）。

### 4. 前端（`frontend/src/`）详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `PLAYERS-BAN-1` 小节

## 影响文件

- `backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ModEntry.cs`
- `backend/internal/games/stardew_junimo/embedded/smapi-mod/StardewAnxiPanel.Control.dll`（已重新编译替换）
- `backend/internal/games/stardew_junimo/ban.go`（新增）
- `backend/internal/games/stardew_junimo/ban_test.go`（新增）
- `backend/internal/web/players_handlers.go`
- `backend/internal/web/instance_handlers.go`
- 前端文件见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md`

## 如何验证

- SMAPI Mod 重新编译（沿用文档命令），`Build succeeded. 0 Errors`，编译产物已用 md5 校验（编译前后哈希确认不同）复制覆盖嵌入 DLL。
- 新增 `ban_test.go`：`TestBanPlayer_RequiresName`、`TestBanPlayer_ServerNotRunning`、`TestBanPlayer_HostUnknown`、`TestBanPlayer_PromotesHostThenWritesCommandFile`（mock `ComposeExecPipe` 验证请求打到 `/roles/admin`，再验证 `ban` 命令文件的 `payload.name` 正确）。
- `cd backend; go build ./... && go vet ./... && go test ./internal/games/stardew_junimo/... ./internal/web/...` 全绿；`go test ./...` 全仓库回归全绿。
- 前端 `cd frontend; npx tsc --noEmit -p . && npm run build` 通过。

**未做的验证**：没有在真实运行实例上实际封禁过一个在线/离线玩家、确认其真的无法重新加入。**没有反编译 vanilla 游戏 DLL 确认 `Game1.bannedUsers` 是否跨容器重启持久化**——这是本次特意推迟到下一步的验证项：需要用已验证过的方法反编译 `Stardew Valley.dll`，检查 `SaveGame.cs` 的存档序列化/反序列化流程是否读写了这个字段。确认结果出来后，需要相应地修正前端确认弹窗文案和这两份文档里"可能失效"这个不确定性用词——如果确认持久化，去掉"可能"；如果确认不持久化，可以考虑评估是否要补一版面板侧持久化（新增数据库表 + 启动后自动重新执行 `!ban`）。

## 下一步注意事项

- 上游 `FindPlayerIdByFarmerNameOrUserName` 用 `FirstOrDefault` 按名字匹配，两个玩家重名时 `!ban`/`!kick`/`!admin` 都可能误伤，这是 JunimoServer 自身已知限制（其仓库 `.claude/plans/audit-security.md` 有记录），面板侧无法修复，只能在管理员使用时提醒留意重名情况。
- 没有实现 `!unban`/`!listbans` 对应的"解封"管理界面——如果封禁操作失误，目前只能等服务器重启（如果确认不持久化）或者手动进入游戏聊天框输入 `!unban <id|名字>` 解决。这是本次按"先做简单接通"明确排除的范围，如果后续用户反馈需要撤销入口，是一个自然的后续迭代，不在这次实现范围。
- 和 kick/say/`!event`/`!joja` 一样是 fire-and-forget，前端只能提示"指令已提交"，拿不到"目标是否真的被封禁成功"这类精确反馈。
- 玩家管理页"管理操作"卡片里的 select+button 目前被一条较晚的 CSS 规则（`.sd-players-action-select, .sd-players-action-item > button { display: none; }`，`StardewPanel.css` 约 15131 行附近）整体隐藏，这是现有布局迭代遗留的既有状态（"踢出玩家"卡片本来就是这样，本次为了保持一致，"封禁玩家"卡片的 select+button 照抄同一结构，同样会被这条规则隐藏）——真正生效的交互入口是"在线玩家"表格每行新增的封禁图标按钮。如果以后要重新启用这些卡片内的 select+button 可见，需要专门评估这条 CSS 规则的意图，不要贸然删除。

# PLAYERS-WARP-HOME-1 玩家回家传送

## 改了什么

- 新增后端接口 `POST /api/instances/:id/players/warp-home`，管理员专用，请求体 `{ "uniqueMultiplayerId": string, "name"?: string }`。
- 新增 `backend/internal/games/stardew_junimo/player_warp.go`：校验目标玩家 ID、实例 running 状态和控制模组 `warpHomeBridgeAvailable` 自检结果，然后写入 `warp-home` panel command。
- 新增 `embedded/smapi-mod-src/WarpHomeBridge.cs`：在 JunimoServer 程序集中反射 `JunimoServer.Util.FarmerExtensions.WarpHome(Farmer)`。这条路是本次确认可走的“回家”路径，区别于 `TryAuthenticate`。
- `ModEntry.cs` 新增 `warp-home` command 消费逻辑：按 `uniqueMultiplayerId` 找在线玩家，拒绝 host，调用 `warpHomeBridge.WarpHome(target)`，并把成功/失败写入 `status.json` 的 command 文案。
- `ControlContract.RuntimeStatus` 新增 `WarpHomeBridgeAvailable` / `WarpHomeBridgeDetail`，供 Go 后端启动前置校验。
- 已重新编译并覆盖 `embedded/smapi-mod/StardewAnxiPanel.Control.dll`。

## 为什么不是 TryAuthenticate

`PasswordProtectionService.TryAuthenticate` 不适合当“回家”按钮：已认证玩家会直接返回 `Already authenticated`，不会再次触发传送；认证内部 `WarpToDestination` 是 private，且绑定认证 lobby 清理语义。本次改用 JunimoServer 已有的公共扩展方法 `farmer.WarpHome()`，它也是上游处理“非主机进入主机屋子自动送回自己小屋”时调用的逻辑。

## 影响文件

- `backend/internal/games/stardew_junimo/player_warp.go`
- `backend/internal/games/stardew_junimo/player_warp_test.go`
- `backend/internal/games/stardew_junimo/embedded/smapi-mod-src/WarpHomeBridge.cs`
- `backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ControlContract.cs`
- `backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ModEntry.cs`
- `backend/internal/games/stardew_junimo/embedded/smapi-mod/StardewAnxiPanel.Control.dll`
- `backend/internal/web/players_handlers.go`
- `backend/internal/web/instance_handlers.go`

## 如何验证

- `cd backend; go test ./internal/games/stardew_junimo ./internal/web` 通过。
- SMAPI 控制模组 Docker 构建通过：`dotnet build -c Release /p:GamePath=/game`，只有既有 ModBuildConfig analyzer 版本 warning，0 errors；新 DLL 已复制覆盖嵌入目录。
- 前端验证见同日 frontend handoff 的 `PLAYERS-WARP-HOME-1`。

## 下一步注意事项

- 仍需真实多人联机验证：在线 farmhand 点击“回家”后，应被传送到自己小屋入口；host 按钮应禁用，因为 host 没有 farmhand cabin。
- `farmer.WarpHome()` 依赖 `Game1.getFarm().GetCabin(farmer.UniqueMultiplayerID)` 找到玩家小屋。正常 farmhand 应可用；如果存档 cabin ownership 已损坏，调用可能无法生效，这属于上游逻辑限制。
- 该接口和 kick/say/ban 一样是 fire-and-forget：HTTP 成功仅代表命令已提交，不代表游戏内动作一定已经完成。
