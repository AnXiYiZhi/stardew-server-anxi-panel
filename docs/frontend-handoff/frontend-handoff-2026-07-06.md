# FE-PANEL-ACCESS-HOST-INVITE-1 局域网邀请地址来源

## 背景
- NAS/云服/FRP 场景下，用户希望“局域网邀请”显示的就是当前访问面板使用的地址，而不是后端探测到的公网出口 IP。
- 旧逻辑调用 `/api/instances/:id/public-ip`，在局域网、反代、FRP、域名访问场景下可能与用户实际进入面板的地址不一致。

## 改了什么
- `useStardewDashboardData.refreshPublicIP()` 改为读取 `window.location.hostname` 并写入现有 `publicIP` 状态，`source` 标记为 `panel-access-host`。
- `InviteCodeCard` 的复制/同步按钮文案改为“复制当前面板访问地址”“同步当前面板访问地址”，按钮显示“同步”而不是“刷新”。
- 保留现有 `publicIP` 类型和属性名以减少改动面；语义已经从“公网 IP 探测结果”转为“当前面板访问 host”。

## 影响文件
- `frontend/src/games/stardew/useStardewDashboardData.ts`
- `frontend/src/games/stardew/InviteCodeCard.tsx`

## 如何验证
- `cd frontend; npm.cmd run build`
- 分别用 `http://192.168.x.x:8090`、公网 IP、FRP 域名进入面板，邀请卡下方 host 应跟浏览器地址栏 hostname 一致。

## 下一步注意事项
- 如果用户用 `localhost:8090` 打开面板，局域网邀请也会显示 `localhost`；这是本次“用什么地址进来就显示什么地址”的规则结果。
- 后端 `/public-ip` 接口还存在，但当前邀请卡不再调用。

# FE-STEAM-AUTH-RUNTIME-READY-1 邀请码授权失效入口

## 背景
- `steamAuthLoggedIn=true` 只表示历史上 steam-auth 成功过一次；真实运行时可能仍出现 `Steam-auth service has no logged-in accounts`，导致邀请码一直 `n/a`。
- 旧 `InviteCodeCard` 只在 `steamAuthLoggedIn===false` 时显示【登录授权】，所以历史认证存在但当前授权失效时，用户只能看到【刷新】，找不到重新登录入口。

## 改了什么
- `types.ts` 的 `InstanceState` 增加 `steamAuthReady?: boolean`，并修正 `steamAuthLoggedIn` 注释为历史标志。
- `InviteCodeCard` 新增 `currentSteamAuthUnavailable` 判断：`steamAuthLoggedIn=true && steamAuthReady=false` 且当前无邀请码/邀请码错误时，显示“需重新 Steam 授权”。
- 服务器运行中按钮显示“停服后重新授权”并禁用，提示先停止服务器；停服后显示“重新登录授权”，继续调用既有 `steamAuthLogin()` 并跳转安装页查看认证日志。

## 影响文件
- `frontend/src/types.ts`
- `frontend/src/games/stardew/InviteCodeCard.tsx`

## 如何验证
- `cd frontend; npm.cmd run build`
- 手动联调：`/state` 返回 `steamAuthLoggedIn=true, steamAuthReady=false` 且邀请码为空/获取失败时，总览和服务器页的邀请码卡应显示重新授权提示，而不是只显示刷新按钮。

## 下一步注意事项
- `steamAuthReady=false` 在服务器未运行时不一定代表异常；前端只在邀请码缺失或错误上下文中展示重新授权入口，避免停止态一直催用户授权。

# FE-INSTALL-SMAPI-PREINSTALL-1 安装页 SMAPI 子状态

## 改了什么
- `InstallPage.tsx` 新增 `smapi_installing` 日志/phase 识别；任务日志出现 `[smapi]` 时，前端会切到 SMAPI 安装状态。
- `install-helpers.ts` 的 `calcSteamDownloadTaskProgress()` 把“下载游戏”子任务从 2 段扩展为 3 段：游戏文件、Steam SDK、SMAPI。
- 新增 SMAPI 专属下载卡片，提示“安装 SMAPI 运行环境中...”。`smapi_install_failed` 属于后置安装失败，不算 Steam 认证失败；重试仍复用保存凭据。

## 影响文件
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- `frontend/src/games/stardew/install-helpers.ts`

## 如何验证
- `cd frontend; npm.cmd run build`
- 手动联调：后端 phase/log 进入 `smapi_installing` 时，第 4 步“下载游戏”保持 active，进度标签显示 SMAPI 子任务；失败时第 4 步标红且按钮走重试下载/继续安装。

## 下一步注意事项
- SMAPI 是 SDK 完成后的最后一个子状态；不要把它显示成新的第 6 步，也不要归类为 Steam 认证失败。

# FE-STEAM-GUARD-SUBMITTED-FEEDBACK-1 验证码提交后等待态

## 改了什么
- `InstallPage.tsx` 新增 `guardSubmittedKind`，区分普通 Steam Guard 和 SteamCMD 验证码提交后的本地等待态。
- 验证码提交接口成功返回后会清空输入框并显示“验证码已提交，正在等待 Steam/SteamCMD 响应”；如果上游长时间没有推进，管理员可以点“重新输入”再次提交。
- 当 `effectivePhase` 离开对应验证码/手机批准阶段时，等待态自动清除，避免下载/失败/完成状态仍残留“已提交”提示。
- `StardewPanel.css` 补充等待态行内按钮布局，避免窄屏下说明文字和“重新输入”按钮挤压。

## 影响文件
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`

## 如何验证
- `cd frontend; npm.cmd run build`
- 手动联调：在 SteamCMD 验证码输入框提交后，右侧认证区应显示已提交等待态；点“重新输入”应回到验证码输入框。

## 下一步注意事项
- 不要把接口成功返回理解为 Steam 验证已完成；真正完成仍以后续 job 日志、`driverPhase` 和实例状态为准。

# FE-STEAMCMD-EMAIL-GUARD-PROMPT-1 SteamCMD 邮箱验证码分行提示

## 改了什么
- `inferLatestSteamAuthLogPhase()` 的 SteamCMD 分支新增识别 SteamCMD 原生邮箱 Guard 分行提示：`this computer has not been authenticated`、`please check your email`、`enter the steam guard`、`code from that message`、`set_steam_guard_code`。
- 日志必须带 `[steamcmd]` 才触发，命中后切到 `steamcmd_guard_required`，让安装页显示 SteamCMD 验证码输入框，而不是继续停留在 `steamcmd_downloading` 或客户端自更新进度。

## 影响文件
- `frontend/src/games/stardew/pages/InstallPage.tsx`

## 如何验证
- `cd frontend; npm.cmd run build`
- 手动联调：SteamCMD 日志出现 `Please check your email... enter the Steam Guard` 后，右侧认证区应显示验证码输入框。

## 下一步注意事项
- 这些提示属于 SteamCMD fallback 授权，不要用普通 steam-auth 的 `steam_guard_required` 文案覆盖。

# 前端接手文档 2026-07-06

## FE-INSTALL-CHANGE-ACCOUNT-1 更换 Steam 账号 / 强制重新认证入口

### 改了什么
后端把安装路由收敛到按持久化 driverPhase/state 决策后，前端**核心失败重试逻辑无需改动**（`canDirectRetry` 现有 phase 列表仍照发 `reuseCredentials`）。本次只新增“更换账号”能力：

- 新增 `forceReauth` 状态（`InstallPage.tsx`）。
- 「更换 Steam 账号 / 重新认证」按钮两处：
  - 已安装卡片，与“重新安装 / 修复”并列。
  - 操作区，仅 `!isInstalled && canDirectRetry && !authFailed` 时展示（未安装但已有保存凭据的复用态）；已安装态只使用卡片内按钮，避免重复渲染。
  - 点击 ⇒ `setForceReauth(true); setShowForm(true)`。
- 表单凭据输入显示条件：`!canDirectRetry` → `!canDirectRetry || forceReauth`（换号时即便复用态也显示账号/密码/VNC 输入框）。
- 表单标题“更换 Steam 账号 / 重新认证”、提示“将清除已保存的 Steam / SteamCMD 授权缓存并用新账号密码重新认证；已下载的游戏文件会保留。”、提交按钮“确认更换账号并重新认证”。
- `handleInstallSubmit`：`forceReauth` ⇒ `{ steamUsername, steamPassword, vncPassword, imageTag, forceReauth: true }`；成功或“取消”复位 `forceReauth`；“安装/重试”“重新安装/修复”按钮点击时 `setForceReauth(false)` 防残留。
- `api.ts` `installInstance` body 类型新增 `forceReauth?: boolean`。

### 影响哪些接口 / 文件
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- `frontend/src/api.ts`
- 接口：`POST /api/instances/:id/install` 新增可选 `forceReauth`。

### 如何验证
- `cd frontend; npm.cmd run build`。
- 手动：已安装态点“更换 Steam 账号 / 重新认证” ⇒ 弹出账密表单；提交后任务日志出现清授权卷提示并进入完整认证流程。

### 下一步注意事项
- 未改既有语义：镜像拉取失败/认证前超时的复用重试仍“不弹凭据表单、自动账号密码继续”；只有 `credentials_required` 才要求重输凭据；`install_timeout`/`steam_auth_connection_failed` 不算账密错误。这些都靠后端路由 + 现有 `canDirectRetry`，不要为这些入口额外弹凭据表单。
- SteamCMD 认证超时重试后，右侧认证区应重新出现 SteamCMD guard 选择/验证码/批准提示（`steamcmd_guard_*` phase），现有 `needsSteamCMDGuardChoice`/`needsSteamCMDGuard` 已覆盖，无需改动。
