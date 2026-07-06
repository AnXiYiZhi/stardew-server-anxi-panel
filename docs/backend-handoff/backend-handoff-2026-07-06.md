# INVITE-CODE-DECOUPLE-AUTHSTATUS-1 启动不卡邀请码 + 暴露 auth 登录状态

## 背景
- 启动服务器时 `doStart`/`doRestart` 在 `waitForReadyState`（最长 20 分钟）里阻塞轮询邀请码，导致「启动」job 迟迟不完成、前端一直转圈。
- 邀请码走 Steam SDR/Galaxy P2P，需要**真实 Steam 账号登录**（`STEAM_REFRESH_TOKEN`）。用户环境 token 为空（游戏文件走了 SteamCMD 兜底、steam-auth 下载失败），所以邀请码永远 `n/a`，却一直显示「获取中」。

## 改了什么（本次已完成 ①②③提示）
- **①（`lifecycle.go`）**：`doStart`/`doRestart` 在服务器就绪、置 `Running` 后**立即完成 job**（前端显示「启动完成」）；新增 `pollInviteCodeBackground()`——后台协程用 `context.Background()` 轮询邀请码，拿到后 `updateDriverPayloadInviteCode`，并在实例非 `Running`（停服/重启）时自动退出，不泄漏。
- **②（`config/env.go` + `web/instance_handlers.go`）**：新增 `config.SteamAuthLoggedIn(dataDir)` = **`STEAM_AUTH_COMPLETED=="true"`**（认证成功过一次即算登录）。**更正**：早先误用 `STEAM_REFRESH_TOKEN` 非空判定——错的，正常可用环境该字段也是空（用户本地实测空 token 照样出邀请码），登录持久化不靠它，按「日志认证成功」即 `STEAM_AUTH_COMPLETED` 判定。`instanceStateResponse.steamAuthLoggedIn` 返回给前端。
- **③前端（`InviteCodeCard.tsx` + `types.ts` + `OverviewPage.tsx`）**：`steamAuthLoggedIn===false`（从未认证）时邀请码显示「需登录 Steam 授权」+【登录授权】按钮，点击 **`onNavigate('install')` 跳转安装页**去完成认证；已认证过则正常显示/获取邀请码。

## 影响文件
- `backend/internal/games/stardew_junimo/lifecycle.go`（`waitForReadyState`/`tailServerLogs`/`readSMAPIStatus` 现已无调用者——保留未删以缩小改动面，后续可清理）
- `backend/internal/games/stardew_junimo/config/env.go`
- `backend/internal/web/instance_handlers.go`
- `frontend/src/types.ts`、`frontend/src/games/stardew/InviteCodeCard.tsx`

## 如何验证
- `cd backend; go build ./... && go test ./internal/games/stardew_junimo/... ./internal/web/...`
- `cd frontend; npx tsc --noEmit -p tsconfig.app.json`
- 真机：启动服务器应很快显示「启动完成」；未登录 auth 时邀请码区显示「需登录 Steam 授权」。

## 「登录 Steam 授权」按钮 —— 改为跳转安装页（撤掉了 AuthLoginOnly 端点）
- **撤销**：先前基于「只跑 steam-auth 拿 token」的 `AuthLoginOnly` 端点/标志已全部回退（`registry/types.go`、`driver.go`、`install_handlers.go`、`instance_handlers.go`、`api.ts` steamAuthLogin 均删除）。原因：`runSteamAuthAttempt` 登录后会尝试下载，国内下载失败 → 掉 `runSteamCMDFallback`（又要批准），且 token 也没写进 .env——这条路在国内走不通，且判定本就不该看 token。
- **现方案**：`InviteCodeCard` 的【登录授权】按钮点击 `onNavigate('install')` **跳转安装页**，由现有安装/认证流程处理（guard 也在那儿）。按钮通过新增可选 prop `onNavigate` 从 `OverviewPage` 传入。

## 前端启动判定解耦邀请码（本次同批修复）
- 现象：后端 job 已完成、`state=running`（用户已能进游戏），但前端「启动」按钮一直「启动中…」，且**没邀请码时停止/重启按钮被禁用**。
- 根因：`OverviewPage`/`ServerControlPage` 里 `waitingForInvite` 含 `inviteCodeRefreshing && !inviteCode`，且 `pendingStartupAction` 只在**出现邀请码**时清；`canStop/canRestart` 还要求 `Boolean(inviteCode)`。
- 改为：`waitingForInvite` 只看 `starting || pendingStartupAction`；`pendingStartupAction` 在 `state==='running'` 即清；`canStop/canRestart` 去掉邀请码要求。即**「running 就算启动完成」，邀请码是后台可选**。
- 影响文件补充：`frontend/src/games/stardew/pages/OverviewPage.tsx`、`ServerControlPage.tsx`。

# IP-DIRECT-CONNECT-DEFAULT-ON-1 默认开启 IP 直连

## 背景（现场实证）
- 服务器启动加载存档后日志出现 `IP connections disabled (default). Players must use invite codes to join.`，而邀请码走 Steam SDR / Galaxy P2P 一直 `Invite Code: n/a`（`SDR relay status: k_ESteamNetworkingAvailability_Waiting`）→ 玩家既没邀请码、又没 IP 直连，根本进不去。
- 根因：`WriteServerSettings()` 写的 `server-settings.json` 的 `Server` 段**没有 `AllowIpConnections` 字段**，JunimoServer 回落到默认「关闭」。参考项目 `StardewValleyServerKit` 默认就是 `Server.AllowIpConnections=true`（sdv-server.sh、admin-panel.js），并注明邀请码与 IP 直连相互独立、邀请码可能单独失败。

## 改了什么
- `saves.go WriteServerSettings()`：`Server` 段新增 `"AllowIpConnections": true`（新建存档默认开 IP 直连）。
- 新增 `saves.go EnsureServerSettingsDefaults(dataDir)`：幂等地确保 `server-settings.json` 的 `Server.AllowIpConnections` 存在（缺失才补 true，已显式设置则尊重不覆盖），并合并保留其它键。
- `lifecycle.go doStart()`：`ComposeUp` 前调用 `EnsureServerSettingsDefaults`（best-effort），使**已有存档**（如本次 test2，建时还没这默认）重启后也能拿到 IP 直连。

## 影响文件
- `backend/internal/games/stardew_junimo/saves.go`
- `backend/internal/games/stardew_junimo/lifecycle.go`
- `backend/internal/games/stardew_junimo/saves_test.go`（新增 `TestEnsureServerSettingsDefaults`，`TestWriteServerSettings_ValidConfig` 加 `AllowIpConnections` 断言）

## 如何验证
- `cd backend; go test ./internal/games/stardew_junimo/ -run ServerSettings`
- 真实服务器：重新发版后重启服务器，日志应变为「IP connections enabled」类；玩家可用 `服务器IP:24642` 直连（**需在安全组放行 UDP 24642**），不再必须等邀请码。

## 下一步注意事项
- IP 直连要真正可用，云安全组必须放行 **UDP 24642**（Stardew）/ 视情况 27015。邀请码（Steam SDR）卡 `n/a` 是独立的 Steam 中继/网络问题，本改动不修它，只是提供更可靠的 IP 直连通道绕过它。
- 后续如需前端可切换 IP 直连开关，再把它提升为 `NewGameConfig` 字段 + UI；当前是默认 true。

# NEWGAME-TIMEOUT-WRONG-SAVE-1 新建存档超时导致回退到旧存档

## 背景（现场实证）
- 全新安装后启动服务器建新存档（test / 海滩 / 3 联机小屋），日志出现 `创建新存档失败（服务器将继续加载已有存档）：POST /newgame: docker compose exec: docker command timed out`，最终**加载了删游戏文件之前残留的旧存档 1111**，而不是新建的 test。
- 根因：`sendNewGameCommand()` 里 `POST /newgame` 只给 **30 秒**超时。JunimoServer 的 `/newgame` 是**同步阻塞**——要把整个世界生成完才返回；全新首次冷启动 + 2 核小机器满载，生成要 1~3 分钟，30 秒必然超时。
- 超时后代码直接 `return err` 判失败，**没进入后面 5 分钟「等存档落盘」轮询**；上层只 Warn 不阻断，于是回退加载已有存档。存档目录在持久化 bind mount（`instances/stardew/saves/`），删 `game-data` 卷不会删存档，所以旧 1111 还在、`gameloader.json` 仍指向它 → 被加载。

## 改了什么（`lifecycle.go` `sendNewGameCommand()`）
- `POST /newgame` 超时 30s → **4 分钟**，避免生成中途 curl 被杀。
- 超时/出错**不再 `return err`**，改为 Warn 后继续走「等新存档落盘」轮询（服务器可能仍在后台生成）。
- 发请求前先记下 `gameloader.json` 现有 `SaveNameToLoad`（`prevSave`）；轮询要求检测到的存档名 **`!= prevSave`**，避免把残留旧存档误报成新建存档。

## 影响文件
- `backend/internal/games/stardew_junimo/lifecycle.go`

## 如何验证
- `cd backend; go build ./... && go test ./internal/games/stardew_junimo/...`
- 真实服务器：重新发版后停服 → 再次「新建存档」，应能等到 test 存档生成并加载，不再回退到旧存档；日志不再是 30 秒即超时判失败。

## 下一步注意事项
- 若 2 核机器世界生成仍超过 4 分钟 POST 超时，靠后面 5 分钟轮询兜底（总计约 9 分钟）；如仍不够可再调大。
- 旧存档 1111 仍在 `saves/Saves/`，`prevSave` 机制不会误认它，但用户如需干净测试可手动删除。
- SMAPI 下载 curl 已加传输超时：`--speed-limit 1024 --speed-time 30`（30s 低于 1KB/s 就放弃、换下一个镜像源），解决慢镜像「连上却不传」长时间不换源的隐患；正常慢速源（本次约 70KB/s）远高于阈值不受影响。改在 `installer.go` 的 SMAPI 预安装脚本。

# STEAMCMD-ANON-SDK-FULL-LOGIN-1 游戏段完整登录 + SDK 段匿名，废弃只用户名缓存模式

> 说明：本条取代同日先前两版草稿（SDK 只用户名复用登录 / 139 清缓存后强制重登）。那两版基于「游戏段登录会在容器里缓存出可复用令牌」的错误假设——现场实证该环境根本不持久化可复用凭据，故 SDK 只用户名登录同样会挂。以本条为准。

## 背景（现场实证，服务器 121.40.29.22）
- SteamCMD 下载反复卡死在 `Loading Steam API...OK` / `Cached credentials not found.`，容器不是崩溃而是**挂在交互式密码提示**（`+login <用户名>` 无密码、无缓存 ⇒ SteamCMD 等 stdin 输密码，one-shot 容器无 stdin ⇒ 永久挂起）。
- 关键实证：即便一次 Steam Guard 批准 + 登录成功 + 下载成功，`config.vdf` 里也**没有任何登录令牌键**（`ConnectCache`/`Accounts`/`MachineAuth`/`RefreshToken` 计数全 0），全卷也没有 `ssfn*`/`loginusers.vdf`。即**这套 SteamCMD 容器环境没有把可复用凭据持久化下来**。
- 完整登录时游戏段 413150、SDK 段 1007 各自 `+login <账密>`，Steam Guard 手机批准被要求了**两次**。
- 参考 `E:\源码\StardewValleyServerKit`：游戏段始终 `+login <账密>`，**SDK（1007）用 `+login anonymous`**，没有「只用户名缓存」这种模式。

## 改了什么（`buildSteamCMDOpts()` + `run()`）
- **游戏段 413150 恒为完整登录** `+login "$STEAM_USERNAME" "$STEAM_PASSWORD"`。`run()` 里 `r.steamCMDUseCache` **恒置 false**（不再从 `STEAMCMD_AUTH_COMPLETED` 推导），彻底废弃「只用户名缓存登录」——它在本环境必然命中 `Cached credentials not found.` 挂死。`steamCMDUseCache=false` 也让 `lineHandler` 把 guard 提示当作正常再认证展示给用户，而非「缓存不可用」报错。
- **SDK 段 1007 恒为匿名登录** `+login anonymous`。SDK Redist 公开可匿名下载，不需要账号、不触发 Guard、永不挂死、也不用二次批准。
- 回退了本条取代的两版草稿对 `clearSteamCMDRuntimeCache()` 的改动（该函数恢复为仅清卷）。

## 影响文件
- `backend/internal/games/stardew_junimo/installer.go`（`run()` 的 `steamCMDUseCache=false`；`buildSteamCMDOpts()`）
- `backend/internal/games/stardew_junimo/driver_test.go`（新增 `TestBuildSteamCMDOptsGameFullLoginSDKAnonymous`；`TestDriverInstallRepairUsesSteamCMDCacheLogin` 改名为 `TestDriverInstallRepairUsesFullLoginAndAnonymousSDK` 并改断言为完整登录 + 匿名 SDK）

## 如何验证
- `cd backend; go test ./internal/games/stardew_junimo/... ./internal/web`
- 真实服务器全新安装：游戏段 413150 只弹一次手机批准；SDK 段 1007 日志出现 `Logging in user anonymous` 直接下载，不再第二次 `Please confirm the login`，也不再停在 `Cached credentials not found.`。

## 下一步注意事项（PART 2，待实测）
- **「记住登录、下次免 2FA」**：靠持久化登录哨兵（Steam config 目录），不是靠只用户名登录。用户 Windows Docker Desktop 同镜像能记住、服务器记不住，疑似服务器触发过 `139` → `clearSteamCMDRuntimeCache()` 删掉 `steamcmd-root-local`（含哨兵）。
- 新流程（分段 + 匿名 SDK）后 139 概率大幅下降。发版后需实测：全新安装一次 → SSH 查 `steamcmd-root-local`/其它卷是否落下哨兵/令牌 → 再装一次看游戏段是否免批准。
- 若实测发现哨兵确实落盘、但被某次 139 清理删掉，再补 PART 2：把 `clearSteamCMDRuntimeCache()` 收窄为只删 `appcache`/`depotcache`/临时目录，**保留 `config/`**；或给凭据单独挂一个永不清理的嵌套卷。

# STEAMCMD-HOME-CACHE-CLEANUP-1 SteamCMD HOME 与缓存清理加固

## 改了什么
- SteamCMD fallback 以 `steam` 用户运行时显式设置 `HOME=/home/steam USER=steam LOGNAME=steam`，避免 `su -m steam` 保留 root HOME 后继续使用 `/root/.local/share/Steam` 自更新缓存。
- `docker.Client.RemoveVolumes()` 改为逐个删除 volume，并忽略缺失卷；真实失败仍返回错误。
- 新增 `docker.Client.RemoveContainersByVolume()`，在 139 后删除 runtime cache volume 前，会先按 `steamcmd-user-local` / `steamcmd-root-local` 查找并强制删除残留 one-shot SteamCMD 容器，解决 `volume is in use - [container_id]`。
- `clearSteamCMDRuntimeCache()` 在清理失败时把 Docker stderr/stdout 脱敏后写进任务日志，便于现场判断是卷不存在、被占用还是 Docker 权限问题。

## 影响文件
- `backend/internal/games/stardew_junimo/installer.go`
- `backend/internal/docker/compose.go`
- `backend/internal/games/stardew_junimo/driver.go`
- `backend/internal/games/stardew_junimo/driver_test.go`
- `backend/internal/docker/compose_test.go`
- `backend/internal/web/docker_handlers_test.go`

## 如何验证
- `cd backend; go test ./internal/docker ./internal/games/stardew_junimo`
- `cd backend; go test ./...`
- 真实服务器重试后，SteamCMD 日志应优先写入 `/home/steam/.local/share/Steam`；若仍出现 139，应先看到 runtime cache 清理，然后再自动重试一次。

## 下一步注意事项
- 如果真实日志仍提示 volume 被占用，优先检查 `docker ps -a --filter volume=<volume>` 是否仍有 Docker 未释放的容器，以及 Docker daemon 是否异常。
- 不要把 `413150` 与 `1007` 合回同一个 SteamCMD 会话；两段独立进程仍是当前更稳的结构。

# STEAMCMD-SPLIT-SDK-1 SteamCMD 游戏与 SDK 分段下载

## 改了什么
- `buildSteamCMDOpts()` 不再把 `413150` 与 `1007` 放在同一个 SteamCMD 会话里连续执行。
- 新流程仍是一个 Docker one-shot 容器，但容器内先运行一次 SteamCMD 下载/校验 `413150` 到 `/data/game`，退出后再运行第二次 SteamCMD 下载/校验 `1007` 到 `/data/game/.steam-sdk`。
- 这样保证每次 `+force_install_dir` 都在对应 `+login` 前设置，避免真实云服日志中的 `Please use force_install_dir before logon!` 后 SteamCMD `Segmentation fault (exit 139)`。
- `appendSteamCMDImageRef()` 过滤旧 `docker.m.daocloud.io/steamcmd/steamcmd:latest`，避免旧实例因为本地已有 daocloud 镜像而继续选中它。
- `runSteamCMDFallback()` 遇到 exit code `139` 时，会删除 `<project>_steamcmd-user-local` 和 `<project>_steamcmd-root-local` 后自动重试一次；不会删除 `steamcmd-login` / `steamcmd-home`，因此 SteamCMD 授权缓存仍保留。

## 影响文件
- `backend/internal/games/stardew_junimo/installer.go`
- `backend/internal/games/stardew_junimo/driver_test.go`

## 如何验证
- `cd backend; go test ./internal/games/stardew_junimo ./internal/web`
- 真实服务器重试安装时，日志应出现 `Running SteamCMD app_update 413150...` 和 `Running SteamCMD app_update 1007...`，且不应再出现 `Please use force_install_dir before logon!`。
- 如果首次仍出现 `exit code 139`，应看到自动清理 runtime cache 并再次启动 SteamCMD；第二次不应继续使用 `docker.m.daocloud.io/steamcmd/steamcmd:latest`。

## 下一步注意事项
- 不要为了减少进程数把两个 `app_update` 合并回同一条 SteamCMD 命令；SteamCMD 对 `force_install_dir` 的位置敏感，SDK 阶段容易崩溃。

# INSTALL-SMAPI-PREINSTALL-1 SDK 后置 SMAPI 预安装

## 改了什么
- `installer.go` 在所有“游戏文件 + Steam SDK 已完成”的成功出口前统一调用 `completeInstall()`；该方法先执行 `ensureSMAPIInstalled()`，成功后才 `markInstallSucceeded()`。
- `ensureSMAPIInstalled()` 把实例 phase 更新为 `smapi_installing`，用当前 `.env` 的 `SERVER_IMAGE`（缺省时为 `sdvd/server:<tag>`）启动一次性 JunimoServer 容器，挂载 `<project>_game-data:/data/game`，下载并运行 SMAPI Linux installer。
- `.env` 默认新增 `SMAPI_VERSION` / `SMAPI_DOWNLOAD_URLS`。默认 URL 顺序是 `gh.llkk.cc`、`github.dpik.top`、`ghfast.top`、GitHub 官方源；脚本会校验 zip，坏包继续尝试下一个候选。
- 已存在 `/data/game/StardewModdingAPI` 时跳过安装。失败时 phase 为 `smapi_install_failed`，job 失败，用户可复用已保存凭据重试。

## 影响文件
- `backend/internal/games/stardew_junimo/config/env.go`
- `backend/internal/games/stardew_junimo/driver.go`
- `backend/internal/games/stardew_junimo/installer.go`
- `backend/internal/games/stardew_junimo/driver_test.go`

## 如何验证
- `cd backend; go test ./internal/games/stardew_junimo`
- 真实安装联调：Steam SDK 完成后应看到 `[smapi]` 日志；已装 SMAPI 时应输出 skip。

## 下一步注意事项
- 该容器是 `docker run --rm` 一次性容器，不是 compose 常驻服务。不要在 README 中要求用户维护它，也不要开放额外端口。
- SMAPI 安装必须保持在 Steam SDK 完成之后；不要提前到游戏文件下载结束时执行。

# STEAMCMD-EMAIL-GUARD-PROMPT-1 SteamCMD 邮箱验证码分行提示

## 改了什么
- `isSteamCMDGuardCodePrompt()` 新增 SteamCMD 原生邮箱 Guard 分行提示识别：`This computer has not been authenticated...`、`Please check your email...`、`enter the Steam Guard`、`code from that message`、`set_steam_guard_code`。
- 这些日志命中后仍走现有 `runSteamCMDFallback()` 分支，更新实例为 `steamcmd_guard_required`，等待用户通过现有 Steam Guard 输入接口提交验证码。
- 新增 matcher 测试 `TestSteamCMDGuardCodePromptMatchesSplitEmailPrompt`。

## 影响文件
- `backend/internal/games/stardew_junimo/installer.go`
- `backend/internal/games/stardew_junimo/driver_test.go`

## 如何验证
- `cd backend; go test ./internal/games/stardew_junimo -run "SteamCMDGuardCodePrompt|SteamGuardCodePrompt"`

## 下一步注意事项
- 该识别仅用于 SteamCMD fallback，不要把这些邮箱提示映射到普通 `steam_guard_required`；前端应继续显示 SteamCMD 专属兜底授权文案。

# 后端接手文档 2026-07-06

## INSTALL-ROUTING-SPLIT-1 安装路由三决策拆分 + 更换账号

### 改了什么
把安装流程里由 `reuseCredentials` 粗暴驱动、且单一 `installRunner.steamCMDRetry` 兼管两职的路由，拆成三个正交决策：

- `reuse`：复用已存账密、不再弹表单输入。
- `steamCMDDirect`：是否跳过拉镜像 + steam-auth 直达 SteamCMD。在 `driver.Install` 计算：
  `reuse && !forceReauth && (shouldResumeSteamCMD(phase) || authAlreadySucceeded(state, phase))`。
- `steamCMDUseCache`：SteamCMD 用“仅用户名缓存登录”还是“账号密码完整登录”。在 `installRunner.run()` 从 `.env` 的 `STEAMCMD_AUTH_COMPLETED` 读出（`forceReauth` 时强制 false）。
- `forceReauth`：清授权卷 + 重置标志 + 重新走完整认证。

新增 `authAlreadySucceeded(state, phase)`（driver.go）：phase ∈ {download_failed,post_auth_failed,game_downloading,steam_sdk_downloading,game_installed} 或 state 属已安装态（game_installed/save_required/ready_to_start/starting/running/stopped）时为真。因为这些 phase/state 只在认证成功之后出现，driverPhase 又已存库，所以它是一个天然跨会话的“已过认证”判据——无需额外持久 steam-auth 标志。

### 修复的三个问题
1. **镜像拉取失败重试误跳 SteamCMD**：`pull_failed` 不满足直达条件 ⇒ `steamCMDDirect=false` ⇒ 重新 `ensureJunimoImages` + `runSteamAuth`；`reuse` ⇒ `runSteamAuth` 走 autoMode（自动账号密码，跳过登录方式选择）。
2. **SteamCMD 认证超时重试秒报“授权缓存不可用”**：超时时从未 `logged in ok`，`STEAMCMD_AUTH_COMPLETED` 未置位 ⇒ `steamCMDUseCache=false` ⇒ SteamCMD 完整登录并正常展示 guard 选择/验证码/批准提示。
3. **认证成功一次后跨会话跳过**（steam-auth 与 SteamCMD 对称）：
   - `STEAM_AUTH_COMPLETED`：`runSteamAuthAttempt` 在 `RunSteamAuthTTY` 返回后，若 `authSucceeded || sdkDownloaded || downloadFailed || currentApp != ""`（即已过登录），调用 `markSteamAuthCompleted` 写 `.env`。`driver.Install` 读取该标志并纳入 `steamCMDDirect` 判据，作为 `authAlreadySucceeded` 相位推断的兜底——修复了“下载中途面板重启 → phase 被 `markInterruptedInstallInstance` 重置为 `install_interrupted` → 相位推断误判为未认证 → 重试又跑一遍 steam-auth”的健壮性缺口。
   - `STEAMCMD_AUTH_COMPLETED`：SteamCMD `logged in ok` 后 `run` 尾部 `UpdateEnvFile` 写入（即使随后下载失败也记录，授权本身有效）。**只写自己这个标志**——steam-auth 与 SteamCMD 是两套独立凭证/会话，SteamCMD 登录成功不代表 steam-auth 也成功，不得顺带置位 `STEAM_AUTH_COMPLETED`。
   - 语义：`STEAM_AUTH_COMPLETED` 决定“是否还要跑 steam-auth 登录步骤”，是“steam-auth 成功一次后所有重试/重装/更新一律走 cmd”的开关；`STEAMCMD_AUTH_COMPLETED` 只决定 SteamCMD 用缓存还是完整登录。正常路径下每个到达 cmd 兜底的场景，steam-auth 都已先行认证成功并置位 `STEAM_AUTH_COMPLETED`，所以移除 cmd 处对该标志的写入不会造成“认证成功却没记录”。

### 更换账号 / 强制重新认证
- `POST /api/instances/:id/install` 新增 `forceReauth`。为真时 handler 置 `ReuseCredentials=false`（要求提供新账密），透传 `InstallRequest.ForceReauth`。
- `run()` 开头：`forceReauth` ⇒ `.env` 写 `STEAM_REFRESH_TOKEN=""`、`STEAMCMD_AUTH_COMPLETED=""`、`STEAM_AUTH_COMPLETED=""`，并调用 `clearAuthVolumes` 删除 `<project>_steam-session`、`<project>_steamcmd-login/-home/-user-local/-root-local`（保留 `game-data`）。
- 新增 `docker.Client.RemoveVolumes(ctx, workDir, names)`：`docker volume rm -f`（`-f` 让缺失卷成为 no-op）。best-effort，占用中失败仅 `jobCtx.Warn` 不阻断。`DockerService` 接口同步新增该方法。

### 影响的接口 / 文件
- `backend/internal/web/install_handlers.go`：`installRequestBody.ForceReauth`；去掉 `SteamCMDRetry: reuseCredentials`，改传 `AutoDownload`/`ForceReauth`。
- `backend/internal/games/stardew_junimo/driver.go`：`Install` 路由计算、`authAlreadySucceeded`、`DockerService` 增 `RemoveVolumes`。
- `backend/internal/games/stardew_junimo/installer.go`：`installRunner` 字段（reuse/steamCMDDirect/steamCMDUseCache/forceReauth）、run() 路由、写标志、`clearAuthVolumes`；fallback 内 `r.steamCMDRetry`→`r.steamCMDUseCache`。
- `backend/internal/games/stardew_junimo/config/env.go`：新增 `STEAMCMD_AUTH_COMPLETED`。
- `backend/internal/docker/compose.go`：`RemoveVolumes`。
- `backend/internal/games/registry/types.go`：`InstallRequest.ForceReauth`（`SteamCMDRetry` 保留为兼容字段）。
- 测试：`driver_test.go` 新增 `TestDriverInstallReRunsSteamAuthAfterPullFailureRetry`；repair 用例先写 `STEAMCMD_AUTH_COMPLETED=true` 才断言仅用户名登录；两处 fake docker 新增 `RemoveVolumes`。

### 如何验证
- `cd backend; go test ./...`（全绿）。
- 真实 Docker 场景按 `docs/06-integration.md` 的路由表逐条走。

### 下一步注意事项
- SteamCMD 与 steam-auth 是两套独立授权缓存（不同卷）。因此“steam-auth 认证成功但 steamcmd 从未跑过”的实例第一次落到 SteamCMD（下载失败/重装）时，会做一次完整 SteamCMD 登录（可能弹一次 guard），成功后写标志、以后命中缓存——这是预期，不是回归。
- `RemoveVolumes` 依赖 CLI `docker volume rm`；若卷正被运行中的 server 容器占用会删除失败（仅告警）。如需在服务器运行时强制换号，未来可考虑先停容器再清卷。
