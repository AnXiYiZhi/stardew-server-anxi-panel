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
