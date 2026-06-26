# Conversation Handoff 2026-06-26

## 接入 patched steam-service CN 镜像

### 背景

Junimo 上游 `steam-service` 在 SteamClient 认证链路里存在两个国内网络下容易触发的问题：

- QR / credentials / token 登录可能在 SteamClient 尚未真正 connected 时开始认证。
- 已 connected 后，认证会话过程中可能遇到 `TryAnotherCM` / `SteamKit2.AsyncJobFailedException` / 断线 / 超时。

已在 fork 仓库 `E:\junimo-server-steam-service-cn` 修补并推送：

```text
32b4316 fix(steam-service): 等待 SteamClient 连接后再认证
40354c3 fix(steam-service): 重试 TryAnotherCM 认证会话
```

面板本次改动把 Junimo `steam-auth` sidecar 接到这个 patched 镜像。

### 修改文件

| 文件 | 改动 |
|------|------|
| `backend/internal/games/stardew_junimo/driver.go` | 新增默认 CN steam-service 镜像与 Steam 连接/认证重试参数常量 |
| `backend/internal/games/stardew_junimo/compose_template.go` | `steam-auth.image` 改为 `${STEAM_SERVICE_IMAGE:-...}`；注入 `STEAM_CLIENT_CONNECT_*` 和 `STEAM_AUTH_SESSION_*` 环境变量 |
| `backend/internal/games/stardew_junimo/config/env.go` | `.env` 模板新增 `STEAM_SERVICE_IMAGE` 和四个重试配置；写入顺序固定 |
| `backend/internal/games/stardew_junimo/installer.go` | 安装时补齐旧 `.env` 缺失的 CN sidecar 配置；镜像检查和 `RunSteamAuthTTY` 都读取 `STEAM_SERVICE_IMAGE` |
| `README.md` | 新增本地联合调试和 CN sidecar 说明 |
| `docs/architecture.md` | 更新 Junimo Steam Auth 重要实现细节 |
| `docs/handoff-roadmap.md` | Milestone 6 增补 CN sidecar 接入记录 |

### 默认配置

新实例 `.env` 会包含：

```env
STEAM_SERVICE_IMAGE=anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2
STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS=60
STEAM_CLIENT_CONNECT_RETRIES=5
STEAM_AUTH_SESSION_RETRIES=3
STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS=5
```

`IMAGE_VERSION` 仍然只控制 Junimo server 镜像：

```env
IMAGE_VERSION=1.5.0-preview.121
```

### 本地联合调试

如果 CN sidecar 还没有推到 Docker Hub，先在 fork 仓库构建本地镜像：

```powershell
cd E:\junimo-server-steam-service-cn
docker build --progress=plain -f tools\steam-service\Dockerfile -t junimo-steam-service-cn:auth-retry-test .
```

然后在面板实例 `.env` 中覆盖：

```env
STEAM_SERVICE_IMAGE=junimo-steam-service-cn:auth-retry-test
```

---

## Milestone 7 存档启动策略调研

### 用户要求

Milestone 7 的启动服务器流程需要把“启动前选择存档”收口成更明确的两条路径：

1. 点击 `启动服务器` 后，如果没有检测到已有存档，弹出两个按钮：
   - `新建存档`
   - `从本机上传存档`
2. 点击 `新建存档` 后弹出自定义新建存档窗口。上游 Junimo 不支持完整自定义创建，所以不能把该流程简单写成调用上游自定义创建；需要由面板前后端承担自定义字段收集、校验，并生成可被 Stardew/Junimo 读取的真实初始存档。`driver_payload` 只能保存策略摘要，不能替代存档本身。
3. 点击 `从本机上传存档` 后弹出上传界面。上传后先解析，展示游戏时间、地图、已有玩家名称、农场/角色基础信息等完整预览；用户确认后再点击“上传到服务器并启动”。

### 当前代码检索结论

- `frontend/src/App.tsx` 已有安装流程的 Modal、任务日志、SSE、错误提示、状态轮询和按钮状态结构，可作为新建/上传存档弹窗的 UI 参考。
- `frontend/src/api.ts` 目前只有 instances、install、jobs、docker 等 API 函数，没有 saves/new-game/upload/parse/confirm 相关函数。
- `frontend/src/types.ts` 目前没有自定义新建存档表单、存档解析预览、上传确认等类型。
- `backend/internal/games/registry/types.go` 中 `SaveInfo` 和 `UploadedFile` 只是占位，字段不足以表达存档预览和自定义新建配置。
- `backend/internal/games/stardew_junimo/driver.go` 中 `ListSaves`、`UploadSave`、`SelectSave`、`DeleteSave` 仍然返回 `registry.ErrNotImplemented`。
- `backend/internal/web/instance_handlers.go` 目前没有 saves/new-game/upload/parse 相关路由。
- `docs/prototypes/stardew-anxi-panel-product-prototype.html` 和 prototype notes 只描述了启动前选择存档的产品原型，不是可复用业务代码。

### 对 Milestone 7 提示词的影响

Milestone 7 不能写成“复用已实现的自定义新建存档代码”。正确要求应是：

- 新增最小但完整的启动前存档策略 API 和 UI。
- 自定义新建存档由面板实现字段收集与配置准备，记录 `driver_payload.save_strategy=custom_new_game`。
- 上传存档必须先进入临时解析/预览阶段，展示游戏时间、地图、已有玩家名称等；确认后才正式写入服务器存档目录并启动。
- 完整存档管理、删除、备份、切换仍留给 Milestone 9，不要在 Milestone 7 做大。

### 外部项目 `E:\stardew-anxi-panel` 的复核结论

在用户要求下，已额外检索旧项目中真正写好的自定义新建存档前后端代码。此前“当前仓库没有现成实现”的结论仍成立，但 M7 有可靠的参考实现可迁移其设计与校验规则。

旧项目的可迁移资产：

- `api/internal/control/control.go`：`InitConfig`、`NormalizeInitConfig`、`ValidateInitConfig`、颜色/枚举校验；字段覆盖人物、农场、宠物、外观、小屋、资金和利润率。
- `web/src/components/FarmInit.vue`：农场图、性别、宠物、小屋、利润率和资金模式的表单交互。新项目应以 React Modal 复刻，不迁移 Vue 框架。
- `POST /api/saves/upload-preview` + `POST /api/saves/upload-commit`：先安全暂存 ZIP、解析 `SaveInfo` 预览、返回 token，再确认导入；预览含游戏日期、地图、角色列表、大小和更新时间。
- ZIP 安全处理：上传和解压大小限制、路径边界检查、拒绝路径穿越/绝对路径/符号链接、校验真实 Stardew 主存档、确认/过期清理临时文件。

旧项目不能直接视为 Junimo 自定义创建方案：

- `smapi-mod/ModEntry.cs` 的 `StartNativeCreate()` 在 SMAPI 内调用 `Game1.game1.loadForNewGame(false)` 生成真实存档；单纯 Go 后端不能复刻这一效果。
- 该方法遇到 Junimo runtime 会提前返回；`WriteJunimoSettings()` 只负责写 Junimo 设置，不足以生成完整自定义角色存档。
- 所以 M7 必须把“接入/移植兼容 Junimo 的真实存档生成执行器，并验证产物可被 Junimo 加载”列为必做验收。不能只把表单或摘要写入 `driver_payload`、`server-settings.json` 后即认定成功。

建议 M7 使用实例路由：

```text
GET  /api/instances/:id/saves/preflight
POST /api/instances/:id/saves/custom-new-game
POST /api/instances/:id/saves/upload-preview
POST /api/instances/:id/saves/upload-commit-and-start
POST /api/instances/:id/start
POST /api/instances/:id/stop
POST /api/instances/:id/restart
GET  /api/instances/:id/status
GET  /api/instances/:id/invite-code
```

`driver_payload` 仅保存 `save_strategy`、active save ID 与无敏感摘要；真实存档留在 Junimo 使用的 volume/挂载目录。确认上传或自定义创建应产出 lifecycle job，统一记录生成/导入、选中、启动、邀请码获取的日志。

---

## 账号密码分支 Steam Guard 验证码提示修复

### 问题

本地同时存在两套相似目录：

```text
E:\stardew-server-anxi-panel\data\instances\stardew
E:\stardew-server-anxi-panel\backend\data\instances\stardew
```

当前安装日志中的实例目录是：

```text
E:\stardew-server-anxi-panel\data\instances\stardew
```

所以当前面板实际读取的是：

```text
E:\stardew-server-anxi-panel\data\instances\stardew\docker-compose.yml
```

`backend\data\instances\stardew` 是历史本地运行方式留下的旧数据目录，删除那里不会影响当前安装流程。为避免后续继续误判，安装 job 现在会明确打印实际 `Compose 文件：...docker-compose.yml`。

账号密码/验证码登录分支还有一个 TTY 读取问题。上游 `steam-auth download` 会输出：

```text
Enter Steam Guard code sent to qq.com:
```

这行提示没有换行符。原来的读取逻辑使用 `bufio.Scanner`，只有遇到换行才把内容交给 `lineHandler`，导致前端直到 60 秒后上游失败并输出同一行里的 `TryAnotherCM` 才看到日志，无法及时显示验证码输入框。

### 修复

- `backend/internal/docker/streaming.go`
  - 新增 `streamTTYOutput()`，按原始 TTY 字节读取。
  - 完整行仍按换行输出。
  - 对 `Enter Steam Guard code`、`Enter verification code`、`Enter code sent to` 这类无换行交互提示提前输出。
  - 对已提前输出的同一段提示做去重，避免 EOF 时重复上报。
- `backend/internal/docker/tty_run_windows.go`
  - Windows Docker Engine API attach 读取改用 `streamTTYOutput()`。
- `backend/internal/docker/tty_run_unix.go`
  - Linux/macOS PTY 读取改用 `streamTTYOutput()`。
- `backend/internal/games/stardew_junimo/installer.go`
  - 安装日志增加实际 `docker-compose.yml` 路径。
- `backend/internal/docker/streaming_test.go`
  - 覆盖无换行 Steam Guard code prompt 能被立即读取。
- `backend/internal/games/stardew_junimo/driver_test.go`
  - 覆盖 `Enter Steam Guard code sent to qq.com:` 能命中 Steam Guard code prompt 判断。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\backend
go test ./...
```

已通过。

联调预期：

1. 新安装任务日志会出现 `实例目录：...` 和 `Compose 文件：...\docker-compose.yml`。
2. 点击账号密码/验证码登录后，如果 Steam 要求邮箱或 App 验证码，前端应在上游等待输入时立即显示验证码输入框。
3. 不应再等到 `TryAnotherCM (Extended: Invalid)` 后才进入验证码输入阶段。

### 下一步注意

- 如果仍看到面板使用 `backend\data\instances\stardew`，说明后端启动时的工作目录或 `PANEL_DATA_DIR` 又切回了旧配置；优先看安装 job 打印的 `实例目录` 和 `Compose 文件`。
- 项目当前规范文件名是 `docker-compose.yml`，不是 `compose.yml`。除非后续明确要兼容双文件名，不要额外生成 `compose.yml`。

---

## 前端安装失败状态展示闭环

### 问题

steam-auth 已经在任务日志中报错，例如 `TryAnotherCM`、SteamClient/CM 连接失败或超时，但安装区仍可能显示“安装中”。主要原因：

- 前端用 `activeInstallJobId` 参与判断 `isInstalling`，如果 SSE finished 回调没有正确清掉这个 id，UI 会继续把安装流程视为进行中。
- 安装区没有直接展示 failed install job 的 `errorMessage` 或最后一条 error log；错误只能在任务中心里看到。

### 修复

- `frontend/src/App.tsx`
  - SSE `finished` 事件改为使用函数式 `setActiveInstallJobId()`，避免闭包里的旧 job id 导致清理失败。
  - 新增监听 `jobs` 的 effect：只要当前 `activeInstallJobId` 对应的 job 已经进入 `failed/succeeded/canceled`，立即清掉活跃安装标记并刷新实例状态。
  - `isInstalling` 改为只认仍在运行的 active install job，不再因为一个已失败的旧 id 长期为 true。
  - 新增 `installFailureDisplayMessage()`，把 instance state、failed job `errorMessage`、任务 error log 汇总成人能看懂的安装区错误提示。
  - 覆盖 `TryAnotherCM` / `steam client not connected` / `SteamClient` / `CM`、任务超时、凭据错误、二维码失败、下载失败、镜像拉取失败等常见分支。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm run build
```

已通过。

联调预期：

1. 如果 steam-auth 最终失败，安装区进度条进入错误态，不再停留在“安装中”。
2. 安装区顶部直接显示中文错误原因，例如 Steam CM 连接失败/超时。
3. 重试安装按钮恢复可点击；任务中心仍保留原始日志用于排查。

---

## Steam SDK 下载进度拆分

### 问题

Junimo `steam-auth download` 不是只下载 Stardew Valley 游戏文件。上游流程是：

1. `Downloading app 413150...` 下载 Stardew Valley 游戏文件。
2. 游戏文件完成后继续 `Downloading app 1007...` 下载 Steamworks SDK Redistributable 到 `.steam-sdk`。

两段下载共用同一种日志：

```text
Progress: done/total files - doneSize/totalSize (xx.x%)
```

之前前端只做了游戏文件进度条，所以游戏文件 100% 后，在 SDK 下载期间页面看起来像卡在 100%，直到 SDK 也下载完才显示安装完成。

### 修复

- `backend/internal/games/stardew_junimo/installer.go`
  - 识别 `Downloading app 1007` 或 `.steam-sdk`，将 driver phase 切到 `steam_sdk_downloading`。
  - 保留 `Downloading app 413150` / 其他 `Downloading app` 作为 `game_downloading`。
- `frontend/src/App.tsx`
  - 将 steam-auth 下载日志按 appId 归类：`413150` 为游戏文件，`1007` 为 Steam SDK。
  - 新增 `extractSteamDownloadProgress(..., 'game' | 'sdk')`，避免 SDK 的 `Progress:` 覆盖游戏文件进度。
  - 下载百分比按文件数 `done/total` 计算并保留 1 位小数，例如 `100/1470 files` 显示为 `6.8%`；不再使用上游括号里的字节百分比。
  - SDK 阶段显示三块信息：
    - 下载任务进度：1/2 游戏文件完成，2/2 Steam SDK 下载中/完成。
    - Stardew Valley 游戏文件：保留 100% 完成态。
    - Steam SDK 运行文件：单独百分比进度条。
  - 总安装进度在 SDK 阶段显示 94%，文案为“游戏文件已下载，正在下载 Steam SDK 运行文件...”。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm run build

cd E:\stardew-server-anxi-panel\backend
go test ./internal/games/stardew_junimo ./internal/docker
```

均已通过。

联调预期：

1. 游戏文件下载时显示 Stardew Valley 游戏文件进度。
2. 游戏文件到 100% 后，页面进入 SDK 下载阶段，不再误以为卡住。
3. SDK 下载期间显示 Steam SDK 独立进度条；SDK 完成后才进入安装完成。

### 追加修复：SDK 阶段即时切换和完成后误超时

#### 问题

实测日志显示游戏文件完成后会继续下载 SDK：

```text
Progress: 1470/1470 files - 150.50 MB/150.50 MB (100.0%)
Download complete!
App installed to: /data/game
Downloading app 1007...
Target directory: /data/game/.steam-sdk
Downloading 4 files (102.91 MB)
```

前端此前依赖后端轮询到 `steam_sdk_downloading` 或 SDK `Progress` 后才切换，导致页面在 `Downloading app 1007` 到第一条 SDK `Progress` 之间仍停留在“游戏文件下载 100%”。另外 Stardew 安装 job 超时仍是 30 分钟；本次实测游戏下载约 1413 秒、SDK 下载约 823 秒，总计超过 37 分钟，导致 SDK 已经 `App installed to: /data/game/.steam-sdk` 后，任务仍被 job manager 标记为“任务超时”。

#### 修复

- `frontend/src/App.tsx`
  - 新增 `hasSteamSdkDownloadStarted()`：只要任务日志出现 `Downloading app 1007` 或 `.steam-sdk`，即使后端实例状态仍是 `game_downloading` / `steam_auth_running`，前端也立即派生为 `steam_sdk_downloading`。
  - 新增 `hasSteamSdkDownloadCompleted()`：如果日志已出现 `App installed to: /data/game/.steam-sdk`，前端可将当前安装视为完成，避免旧 failed job 的 `任务超时` 继续覆盖页面。
  - 当旧任务日志已证明 SDK 完成时，实例状态卡也会在前端派生显示为 `game_installed/steam_auth_done`，避免安装区已完成但顶部状态仍显示 error。
  - `DownloadProgressBody` 在还没有实际 SDK `Progress` 时也渲染 0% 进度条，并显示“正在与 Steam 下载服务器建立连接中...”。
- `backend/internal/games/stardew_junimo/driver.go`
  - Stardew 安装 job 超时从 30 分钟延长到 2 小时，适配国内 Steam CDN 下载游戏 + SDK 的耗时。
- `backend/internal/games/stardew_junimo/installer.go`
  - 记录当前下载 app：游戏 app `413150` 和 Steam SDK app `1007`。
  - 只有 SDK 下载完成才真正标记安装成功。
  - 如果 SDK 已完成但 context deadline 已到，优先调用 `markInstallSucceeded()`，不再写入 `install_timeout`。

#### 关于重试跳过下载

面板当前不会直接读取 Docker named volume `game-data` 判断文件是否完备；实际下载和校验仍交给 Junimo `steam-auth download`。上游 steam-service 会校验已存在文件/chunk，并输出类似：

```text
Skipped N existing files (already up to date)
```

因此点击“重试安装”会复用 `.env` 凭据并重新运行 `steam-auth download`；已存在且校验通过的文件会被上游跳过，缺失/损坏的 chunk 会被补齐。后续如果要做到“面板启动前主动检测 game-data 是否完备并直接标记 installed”，需要新增一个只读 Docker volume 检查/探针容器，不要直接假设宿主机实例目录里有游戏文件。

#### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm run build

cd E:\stardew-server-anxi-panel\backend
go test ./internal/games/stardew_junimo ./internal/docker ./internal/web
```

均已通过。

### 追加修复：下载失败/超时重试直达下载

#### 问题

下载阶段超时、Steam CM 网络错误、SDK 下载超时等并不代表 Steam 凭据失效。此前点击“重试安装”仍可能打开安装表单，让管理员重新输入 Steam 用户名/密码/VNC 密码，或者再次进入扫码/账号密码登录方式选择，体验不对。

#### 修复

- `backend/internal/games/registry/types.go`
  - `InstallRequest` 新增 `AutoDownload`。
- `backend/internal/web/install_handlers.go`
  - 当前端传 `reuseCredentials=true` 时，从实例 `.env` 读取 `STEAM_USERNAME` / `STEAM_PASSWORD` / `VNC_PASSWORD`。
  - 同时将 `AutoDownload=true` 传给 driver。
- `backend/internal/games/stardew_junimo/driver.go`
  - 将 `AutoDownload` 保存到 install runner。
- `backend/internal/games/stardew_junimo/installer.go`
  - `autoMode=true` 时跳过 `waitSteamAuthMode()`，不再进入 `auth_method_required`。
  - 直接运行 `steam-auth download`，用于校验已有文件并继续下载缺失/损坏内容。
  - job 日志会出现“复用已保存的 Steam 凭据，直接校验并下载游戏文件。”
- `frontend/src/App.tsx`
  - 对 `install_timeout`、`steam_auth_connection_failed`、`steam_auth_failed`、`qr_auth_failed`、`download_failed`、普通 `error` 等非凭据错误，点击“重试安装”会直接调用 `installInstance({ reuseCredentials: true })`。
  - 只有 `credentials_required` 才重新显示 Steam/VNC 凭据表单。
  - 下载等待文案改为“正在校验已有文件并连接 Steam 下载服务器...”，提示这是校验/续传流程。

#### 已有文件跳过逻辑

面板不直接读 Docker named volume `game-data` 做文件完备判断。重试时仍交给 Junimo `steam-auth download`：

- 已存在且校验通过的文件/chunk 会跳过。
- 缺失或损坏的 chunk 会重新下载。
- 上游可能输出 `Skipped N existing files (already up to date)`。

后续如果要“面板主动检测 game-data 完备并直接标记 installed”，应新增只读探针容器挂载 `game-data`，不要读取实例目录，因为 Junimo 官方 compose 把游戏文件放在 Docker named volume 中。

#### 验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm run build

cd E:\stardew-server-anxi-panel\backend
go test ./...
```

均已通过。

---

## Steam Auth 两层认证菜单拆分

### 问题

Junimo `steam-auth setup` 会出现两层都使用 `1/2` 的交互菜单，但面板当前不能把两层都直接暴露给用户，否则会出现“还没选扫码/账号密码，就先显示 Steam Guard 按钮”的错位：

1. 第一层 `Choose authentication method`：
   - `[1] Username & Password`
   - `[2] QR Code (Steam Mobile App)`
2. 账号密码路径后可能出现第二层 `Steam Guard Authentication`：
   - `[1] Approve in Steam Mobile App`
   - `[2] Enter code from Steam Mobile App or Email`

旧逻辑把第一层 `Choose authentication method` 误判成 `steam_guard_choice_required`，导致前端还没让用户选择扫码/账号密码，就直接显示 Steam Guard 分支按钮。随后又发现实际安装路径运行的是 `steam-auth download`，该路径不会输出第一层菜单，因此需要由面板主动提供第一层选择。

### 修复

- `backend/internal/games/stardew_junimo/installer.go`
  - 新增 `isSteamAuthMethodMenu()` 和 `isSteamGuardChoiceMenu()`。
  - 镜像检查完成后先进入 `auth_method_required`，等待面板选择 Steam 登录方式。
  - 选择 `扫码登录` 时运行 `steam-auth setup`，并自动向上游第一层菜单输入 `2`。
  - 选择 `账号密码/验证码登录` 时运行 `steam-auth download`，使用 `.env` 中的账号密码非交互登录并下载游戏。
  - 第二层 Steam Guard 菜单才进入 `steam_guard_choice_required`。
  - 安装 job 开始时会再次执行 `Prepare()`；如果 `docker-compose.yml` 或 `.env` 被手动删除，会重新生成并写入 job 日志。
- `backend/internal/web/install_handlers.go`
  - `auth_method_required + 1` 主动推进到 `steam_auth_running`。
  - `auth_method_required + 2` 主动推进到 `steam_qr_required`。
  - `steam_guard_choice_required + 1` 主动推进到 `steam_guard_mobile_required`。
  - `steam_guard_choice_required + 2` 主动推进到 `steam_guard_required`。
- `frontend/src/App.tsx`
  - `auth_method_required` 显示第一层按钮：`扫码登录` / `账号密码/验证码登录`。
  - `steam_guard_choice_required` 显示第二层按钮：`手机 App 批准` / `输入验证码`。
  - 进度条、超时兜底和任务状态均识别 `auth_method_required`。
- `backend/internal/games/stardew_junimo/driver_test.go`
  - 新增 `TestSteamAuthMenusAreClassifiedSeparately`，防止两个菜单再次串阶段。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\backend
go test ./...

cd E:\stardew-server-anxi-panel\frontend
npm run build
```

联调时预期：

1. 镜像检查完成后，面板阶段为 `auth_method_required`，显示 `扫码登录` / `账号密码/验证码登录`。
2. 点击 `扫码登录` 后启动 `steam-auth setup`，面板阶段为 `steam_qr_required`，等待 QR 输出。
3. 点击 `账号密码/验证码登录` 后启动 `steam-auth download`；如果触发 Steam Guard，阶段为 `steam_guard_choice_required`。
4. 出现 Steam Guard 菜单后，面板显示 `手机 App 批准` / `输入验证码`。
5. 如果用户删除实例目录里的 `docker-compose.yml`，下一次安装 job 日志应出现 `docker-compose.yml 缺失，已重新生成。`。

### 追加修复

- `frontend/src/App.tsx` / `frontend/src/App.css`
  - QR 扫码不再把 ASCII QR 直接塞在安装卡片里，而是提供 `打开扫码窗口` 按钮，独立弹窗完整展示二维码。
  - 新增下载进度解析，解析 steam-auth 日志：
    ```text
    Progress: 100/1234 files - 1.2 GB/5.4 GB (22.4%)
    ```
    并在 `game_downloading` 阶段显示独立的“Stardew Valley 游戏文件下载”百分比进度条；后续已扩展为 `extractSteamDownloadProgress()`，可区分游戏 app `413150` 和 Steam SDK app `1007`。
- `backend/internal/games/stardew_junimo/driver.go`
  - `Prepare()` 对 `docker-compose.yml` / `.env` 的 `os.Stat` 非缺失错误不再静默忽略。
  - 写入文件后立即二次 `Stat` 校验。
- `backend/internal/games/stardew_junimo/installer.go`
  - 安装 job 日志输出实例目录。
  - 安装 job 调用 `Prepare()` 后强校验 `docker-compose.yml` 和 `.env` 确实存在，否则直接失败并给出具体路径。

如果实例已经生成过旧的 `docker-compose.yml`，`Prepare()` 不会覆盖。要让旧实例使用新 compose 模板，需要备份后删除实例内旧 `docker-compose.yml`，再调用 Prepare；或者手动把 `steam-auth.image` 改为：

```yaml
image: ${STEAM_SERVICE_IMAGE:-anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2}
```

并补上四个 environment 变量。

### 验证建议

后端单元测试：

```powershell
cd E:\stardew-server-anxi-panel\backend
go test ./...
```

前端构建：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm run build
```

联调检查：

1. 构建本地 sidecar 镜像 `junimo-steam-service-cn:auth-retry-test`。
2. 面板实例 `.env` 设置 `STEAM_SERVICE_IMAGE=junimo-steam-service-cn:auth-retry-test`。
3. 重新 Prepare 或手动确认 `docker-compose.yml` 的 `steam-auth.image` 使用 `STEAM_SERVICE_IMAGE`。
4. 安装游戏，任务日志中应能看到检查/运行的 steam-auth 镜像为 CN sidecar。
5. QR / Steam Guard 过程若遇到 `TryAnotherCM`，应看到 sidecar 内部重试日志，而不是立即失败。

### 下一步注意事项

- 当前默认 Docker Hub tag `anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2` 需要在发布镜像后才能被新机器拉取；发布前本机联调用 local tag 覆盖。
- `STEAM_SERVICE_IMAGE` 只替换 `steam-auth` sidecar，不替换 Junimo `server` 主镜像。
- 不要把 `STEAM_SERVICE_IMAGE` 做成前端普通用户可输入的任意镜像字段；如果后续开放 UI，应限制为 admin-only 高级设置。
- 老实例的 `docker-compose.yml` 不会被自动覆盖，这是项目既有“不覆盖用户修改”的设计。

---

## 任务中心清空按钮

### 改动

- `backend/internal/storage/jobs.go`
  - 新增 `ClearJobs()`，清空 `jobs` 和 `job_logs`。
  - 如果还有 `queued` / `running` 任务，返回 `ErrActiveJobsExist`，避免安装中删除任务日志。
- `backend/internal/jobs/manager.go`
  - 新增 `Clear()` 转发到 storage。
- `backend/internal/web/jobs_handlers.go`
  - `DELETE /api/jobs`，admin-only。
  - 有进行中任务时返回 409 `active_jobs_exist`。
  - 清空后写 audit log：`jobs_cleared`。
- `frontend/src/api.ts`
  - 新增 `clearJobs()`。
- `frontend/src/App.tsx`
  - 任务中心新增管理员按钮 `清空任务中心`。
  - 点击前有确认弹窗；清空成功后清除当前选中任务和日志。
- `backend/internal/web/jobs_handlers_test.go`
  - 新增 `TestAdminCanClearJobCenterWhenNoActiveJobs`，覆盖进行中任务阻止清空、任务完成后可清空。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\backend
go test ./...

cd E:\stardew-server-anxi-panel\frontend
npm run build
```

---

## 旧 compose 拉错 steam-auth 镜像修复

### 问题

安装日志出现：

```text
[check] anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2 镜像缺失，需要拉取。
需要拉取 1 个服务的镜像：steam-auth
[pull] Image sdvd/steam-service:1.5.0-preview.121 Pulling
[pull] Image sdvd/steam-service:1.5.0-preview.121 Pulled
steam-auth run error: create steam-auth: HTTP 404: {"message":"No such image: anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2"}
```

根因：`missingServices()` 已经按 `.env` 的 `STEAM_SERVICE_IMAGE` 判断 CN 镜像缺失，但 `docker compose pull steam-auth` 读取的是实例目录里已经存在的旧 `docker-compose.yml`，旧 compose 仍然写死 `sdvd/steam-service:${IMAGE_VERSION}`，所以拉取了错误镜像。随后 `RunSteamAuthTTY` 按 `.env` 创建 CN 镜像容器，因本地不存在而 404。

### 修复

`installer.go` 新增 `migrateSteamAuthComposeImage()`：

- 安装写完 `.env` 后、pull 前执行。
- 只替换旧 compose 中的 `image: sdvd/steam-service:...` 行为：

```yaml
image: ${STEAM_SERVICE_IMAGE:-anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2}
```

- 如果旧 compose 缺少 CN sidecar 的四个环境变量，会插入到 `STEAM_KEEP_LANGUAGES` 后面。
- 不整份覆盖 compose，避免破坏用户其他自定义。

新增单测：

```text
TestMigrateSteamAuthComposeImage
```

### 本地联调注意

如果 Docker Hub 还没有 `anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`，仍然需要先在本机提供这个镜像。两种方式任选其一：

```powershell
cd E:\junimo-server-steam-service-cn
docker build --progress=plain -f tools\steam-service\Dockerfile -t anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2 .
```

或使用本地调试 tag，并改实例 `.env`：

```powershell
docker build --progress=plain -f tools\steam-service\Dockerfile -t junimo-steam-service-cn:auth-retry-test .
```

```env
STEAM_SERVICE_IMAGE=junimo-steam-service-cn:auth-retry-test
```

---

## Milestone 7: Stardew Junimo 服务器生命周期 (2026-06-26)

### 概述

实现了 Stardew Junimo 服务器启动前存档检查、新建游戏（server-settings.json 方式）、上传存档（两阶段预览确认）、start/stop/restart/邀请码完整生命周期，前后端均可通过 ?   	github.com/anxi-panel/stardew-server-anxi-panel/backend/cmd/panel	[no test files]
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/auth	(cached)
?   	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config	[no test files]
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker	(cached)
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry	(cached)
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo	(cached)
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config	(cached)
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs	(cached)
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage	(cached)
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/web	(cached)
?   	github.com/anxi-panel/stardew-server-anxi-panel/backend/migrations	[no test files] 和 。

### 架构决策

**1. saves 存储方式变更（named volume → bind mount）**
- 旧：（Docker named volume，面板无法访问）
- 新：（bind mount，面板可读写）
- 路径：
- 已有实例：安装流程中  自动迁移 docker-compose.yml

**2. 自定义新建存档的真实限制**
- Junimo 不暴露 SMAPI  能力，无法从面板后端生成完整自定义存档
- M7 通过写  配置 Junimo 支持的字段（FarmName/FarmType/小屋/利润/宠物/金钱模式）
- FarmerName/FavoriteThing/外貌 **需要** 预置 save template（）
- 前端已在新建游戏 modal 中提示此限制
-  检测是否有可用模板

**3. 邀请码获取**
- 通过 ，stdin 注入 
-  用正则从 stdout 解析  格式

**4. 上传 token 安全**
-  在 web server struct 中，按实例绑定 token，10 分钟 TTL，单次消耗
- 访问时自动清理过期 token 的临时目录
- cancel 接口即时清理

### 新增 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET |  | 检查存档列表 |
| POST |  | 新建游戏配置 + 启动 |
| POST |  | 上传 ZIP 预览（multipart/save） |
| POST |  | 确认导入 + 启动 |
| POST |  | 启动服务器 |
| POST |  | 停止服务器 |
| POST |  | 重启服务器 |
| GET |  | 获取邀请码 |

### 关键文件

| 文件 | 说明 |
|------|------|
|  | 新增 （stdin 管道到容器） |
|  | bind mount 替换 named volume |
|  | ZIP 安全解析/导入/服务器设置写入 |
|  | Start/Stop/Restart/InviteCode 异步 job |
|  | NewGameConfig/SaveInfo/Preflight/UploadPreview |
|  | 全部 8 个 HTTP handler + pendingUploadStore |
|  | 新增生命周期相关 types |
|  | 新增 8 个 API 函数 |
|  | LifecycleSection/SaveCard/NewGameModal/UploadModal |

### 下一步注意事项

- M8： 已初步实现，但 preflight → 有存档路径尚未接选择哪个存档后再启动，当前直接 start（适用于单存档场景）
- 如果要多存档选择，需要在  返回后显示选择列表并调用 （attach-cli）
-  目录预置真实 Stardew 存档模板后， 中需新增patch XML + copy分支（参考  中的 ）
- Compose migrateSavesVolume 已幂等，下次安装自动处理旧实例

### 验证

Windows PowerShell
°æȨËùÓУ¨C£© Microsoft Corporation¡£±£ÁôËùÓÐȨÀû¡£

°²װ×îÐµÄ PowerShell£¬Á˽âÐ¹¦Äܺ͸Ľø£¡https://aka.ms/PSWindows

PS E:\stardew-server-anxi-panel\backend> ?   	github.com/anxi-panel/stardew-server-anxi-panel/backend/cmd/panel	[no test files]
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/auth	(cached)
?   	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/config	[no test files]
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/docker	(cached)
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/registry	(cached)
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo	(cached)
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/games/stardew_junimo/config	(cached)
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/jobs	(cached)
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/storage	(cached)
ok  	github.com/anxi-panel/stardew-server-anxi-panel/backend/internal/web	(cached)
?   	github.com/anxi-panel/stardew-server-anxi-panel/backend/migrations	[no test files]

均已通过

---

## Milestone 7: Stardew Junimo 服务器生命周期 (2026-06-26)

### 概述

实现了 Stardew Junimo 服务器启动前存档检查、新建游戏（server-settings.json 方式）、上传存档（两阶段预览确认）、start/stop/restart/邀请码完整生命周期。

### 架构决策

**1. saves 存储方式变更（named volume → bind mount）**
- 旧：saves:/config/xdg/config/StardewValley（Docker named volume，面板无法访问）
- 新：./.local-container/saves:/config/xdg/config/StardewValley（bind mount）
- 路径：<dataDir>/.local-container/saves/Saves/<SaveFolderName>/
- 已有实例：安装流程中 migrateSavesVolume 自动迁移 docker-compose.yml

**2. 自定义新建存档的真实限制**
- Junimo 不暴露 SMAPI loadForNewGame 能力，无法从面板后端生成完整自定义存档
- M7 通过写 server-settings.json 配置 Junimo 支持的字段（FarmName/FarmType/小屋/利润/宠物/金钱模式）
- FarmerName/FavoriteThing/外貌需要预置 save template（.local-container/saves-templates/）
- 前端在新建游戏 modal 中已提示此限制

**3. 邀请码获取**
- 通过 docker compose exec -T server attach-cli，stdin 注入 invitecode\nquit\n
- parseInviteCode 用正则从 stdout 解析邀请码

**4. 上传 token 安全**
- pendingUploadStore 在 web server struct 中，按实例绑定 token，10 分钟 TTL，单次消耗

### 新增 API

- GET /api/instances/:id/saves/preflight
- POST /api/instances/:id/saves/custom-new-game
- POST /api/instances/:id/saves/upload-preview（multipart/save）
- POST /api/instances/:id/saves/upload-commit-and-start
- POST /api/instances/:id/start
- POST /api/instances/:id/stop
- POST /api/instances/:id/restart
- GET /api/instances/:id/invite-code

### 关键文件

- backend/internal/docker/compose.go: 新增 ComposeExecPipe
- backend/internal/games/stardew_junimo/compose_template.go: bind mount 替换 named volume
- backend/internal/games/stardew_junimo/saves.go: 存档管理、ZIP 安全检查
- backend/internal/games/stardew_junimo/lifecycle.go: Start/Stop/Restart/InviteCode
- backend/internal/games/registry/types.go: 新类型
- backend/internal/web/lifecycle_handlers.go: 8 个 handler + pendingUploadStore
- frontend/src/types.ts: 新增生命周期 types
- frontend/src/api.ts: 新增 8 个 API 函数
- frontend/src/App.tsx: LifecycleSection/SaveCard/NewGameModal/UploadModal

### 下一步注意事项

- 有存档路径当前直接启动，多存档选择需要在 ListSaves 返回后显示列表并调用 attach-cli saves select
- saves-templates 预置真实存档后，WriteServerSettings 需新增 patch XML 分支
- migrateSavesVolume 已幂等，下次安装自动处理旧实例

### 验证

backend: go test ./...  — 全部通过
frontend: npm run build — 构建成功（2026-06-26）
---

## Milestone 7.5: 可视化新建存档创建器（2026-06-26）

### 改了什么

**目标：** 用真实游戏素材替换旧的纯文字 select 下拉框，为 React 前端提供可视化存档创建体验。

**新增文件：**

| 文件 | 说明 |
|------|------|
| `backend/internal/games/stardew_junimo/embedded/smapi-mod/manifest.json` | SMAPI mod 元数据（已嵌入 Go binary） |
| `backend/internal/games/stardew_junimo/embedded/smapi-mod/StardewAnxiPanel.Control.dll` | 预编译 SMAPI mod DLL（已嵌入 Go binary） |
| `backend/internal/games/stardew_junimo/smapi_mod.go` | `//go:embed` + `installSMAPIMod()` |
| `backend/internal/games/stardew_junimo/catalog.go` | Catalog API 类型 + `ReadCatalog()`（有 mtime 缓存）+ `DefaultCatalog()`（SVG fallback） |
| `backend/internal/games/stardew_junimo/catalog_test.go` | 12 个 catalog 测试 |
| `frontend/src/games/stardew/NewGameCreator.tsx` | 可视化创建器组件 |
| `frontend/src/games/stardew/NewGameCreator.css` | 创建器样式 |

**修改文件：**

| 文件 | 改动 |
|------|------|
| `backend/internal/games/stardew_junimo/compose_template.go` | 新增 `SAP_CONTROL_DIR=/data/control` 环境变量；新增 control 和 mods bind mount |
| `backend/internal/games/stardew_junimo/driver.go` | `Prepare()` 新增 control/commands/mods 目录；调用 `installSMAPIMod()` |
| `backend/internal/games/stardew_junimo/saves.go` | profitMargin 改为 "100"\|"75"\|"50"\|"25"；moneyMode 改为 "shared"\|"separate"；新增 `WriteInitConfig()`；`WriteServerSettings()` 内部调用 `WriteInitConfig()` |
| `backend/internal/games/stardew_junimo/saves_test.go` | 更新 profit/money 测试用例 |
| `backend/internal/games/registry/types.go` | `RgbColor` 类型；`NewGameConfig` 新增 Gender/PetType/PetBreedID/Skin/Hair/Shirt/Pants/Accessory/EyeColor/HairColor/PantsColor |
| `backend/internal/web/lifecycle_handlers.go` | `handleCustomNewGameCatalog`（GET）、`handleCustomNewGameCatalogRefresh`（POST） |
| `backend/internal/web/instance_handlers.go` | 路由：`GET/POST /api/instances/:id/custom-new-game/catalog` |
| `frontend/src/types.ts` | `RgbColor`、`CatalogItem`、`CatalogResponse`；`NewGameConfig` 新增角色字段 |
| `frontend/src/api.ts` | `getCustomNewGameCatalog()`、`refreshCustomNewGameCatalog()` |
| `frontend/src/App.tsx` | `LifecycleSection` 的旧 NewGameModal 替换为 `<NewGameCreator>`；引入 `defaultInstanceId` |

### 影响接口/文件

**新 API 端点：**
- `GET  /api/instances/:id/custom-new-game/catalog` — 读取 options.json 或返回 fallback（需认证）
- `POST /api/instances/:id/custom-new-game/catalog` — 清缓存后重读 options.json（需管理员）

**变更的字段格式（breaking for old saves_test values）：**
- `NewGameConfig.profitMargin`: 旧 "normal"\|"75%"\|"50%"\|"25%" → 新 "100"\|"75"\|"50"\|"25"
- `NewGameConfig.moneyMode`: 旧 "normal"\|"shared" → 新 "shared"\|"separate"

**新挂载目录（compose_template.go 生效，现有实例需重建 compose 或手动修改）：**
- `.local-container/control:/data/control`
- `.local-container/mods/StardewAnxiPanel.Control:/data/Mods/StardewAnxiPanel.Control`

### 如何验证

1. `go test ./...` — 全部通过
2. `npm run build` — 19 modules，无错误
3. 前端 "新建游戏" 按钮打开的是可视化创建器，有农场图片网格
4. 首次（SMAPI 未运行）显示 SVG 占位图 + 黄色 fallback banner
5. 服务器启动一次后，banner 出现"刷新素材"按钮，点击可获取真实游戏截图

### 下一步注意事项

1. **现有实例需更新 compose**：老 docker-compose.yml 没有 control/mods bind mount，需手动添加或调用 `Prepare()` 触发重写（当前实现 Prepare 不覆盖已有 compose 文件，需要手动处理老实例）
2. **SMAPI mod 安装**：调用 `Prepare()` 时自动写入 `.local-container/mods/StardewAnxiPanel.Control/`，非致命错误不阻断启动
3. **catalog 缓存**：基于 options.json mtime，内存缓存，重启面板后自动失效。`POST /catalog` 可强制刷新
4. **petBreedId**：前端传字符串 ID，后端转 Junimo 用的 int（0-3），超出范围截断到 [0,3]
5. **server-init.json**：FarmerName 为空时跳过写入，SMAPI 不会 Apply 定制（保留 Junimo 默认值）

### 验证

backend: go test ./...  — 全部通过（含 12 个新 catalog 测试）
frontend: npm run build — 构建成功（2026-06-26）

---

## 素材导出器 exit status 1 修复（2026-06-26）

### 问题

“重新生成素材”启动一次性 `sdvd/server` exporter 后只显示 `export container exited with error: exit status 1`，无法判断是 Steam Auth、设置路径、SMAPI 或游戏启动失败。

### 修复

- `backend/internal/games/stardew_junimo/catalog_exporter.go`
  - 临时容器新增 `ALLOW_INSECURE_SETUP=true`；它不启动 Compose `steam-auth` sidecar，只需要离线运行到 SMAPI `GameLaunched` 导出素材。
  - `SETTINGS_PATH` 从目录 `/data/settings` 修正为 `/data/settings/server-settings.json`，与正式 Compose server 一致。
  - `lineCallbackWriter` 新增 `Flush()`，将容器以无换行 fatal message 退出时的末尾输出写入任务日志。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./internal/games/stardew_junimo ./internal/web
```

已通过。

### 下一步注意

不需要重装游戏。直接点击“重新生成素材”进行 Docker 联调，确认日志不含 `steam-auth download`、`Downloading app 413150` 或 `Downloading app 1007`。若仍失败，任务中心现在应能显示 exporter 最后一段真实错误；据此再定位，不要仅依据 exit status 1 修改。

### 追加修复：裸容器静默超时改为 Compose 临时服务

后续联调没有再次 exit 1，却在 10 分钟内只出现 catalog GET 轮询，没有任何 Junimo/SMAPI 输出，最终为 `catalog export timed out after 10 minutes`。这说明裸 `docker run sdvd/server` 没有拿到 Junimo 必需的 Compose 网络和 `steam-auth` 依赖，不能可靠启动到 SMAPI。

`ExportCatalogContent()` 已改为在实例目录运行 `docker compose run --rm --no-ports server`：

- 复用现有 `game-data`、`steam-auth` service、Compose network 和已安装的 SMAPI mod；不会执行游戏下载命令。
- 以 `.local-container/catalog-export-saves` 覆盖 `server` 的真实 saves 挂载，导出期间不创建或改动用户真实存档；结束后删除临时目录。
- 仍使用固定容器名，在 `options.json` 出现后停止该临时容器，并停止 exporter 为此拉起的 `steam-auth` sidecar。

验证：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./internal/games/stardew_junimo ./internal/web
```

已通过。Docker 实机验证仍待执行：点击“重新生成素材”后，任务日志首行应为“通过 Compose 启动临时 server”，且不应出现 `steam-auth download` 或 Steam app 下载日志。

---

## 存档启动入口拆分（2026-06-26）

### 产品行为

- 页面始终显示独立“存档启动”面板：`创建存档并启动` 与 `上传存档并启动`。
- 普通 `启动服务器` 保持为独立生命周期操作，默认使用 Junimo 上次使用的可用存档。
- 若普通启动时没有有效存档，后端返回 409 `save_required`，前端提示并平滑滚动到存档启动面板；不再先显示“检查存档”按钮，也不让 Junimo 自动创建默认农场。

### 修改

- `backend/internal/web/lifecycle_handlers.go`：`POST /api/instances/:id/start` 在创建 lifecycle job 前调用 driver `ListSaves()`；零存档时返回结构化 `save_required`。
- `frontend/src/App.tsx`：移除以 preflight 驱动的存档选择区，固定显示创建/上传并启动入口；普通启动捕获 `save_required` 后跳转到该面板。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./internal/games/stardew_junimo ./internal/web

cd ..\frontend
npm run build
```

均已通过。

---

## Milestone 7.5 续篇：Install 阶段自动导出 catalog（实现完成）

### 背景

上一轮 M7.5 仅完成了 catalog 的读取和展示，仍以 SVG 占位图作为默认内容。本轮实现核心要求：在 Steam 安装完成后、服务器首次启动前，自动导出真实游戏素材。

### 新增/修改文件

| 文件 | 改动说明 |
|------|----------|
| `backend/internal/games/stardew_junimo/catalog_exporter.go` | **新建** — 锁文件管理、`ExportCatalogContent()`（docker run 轮询方案）、错误持久化 |
| `backend/internal/games/stardew_junimo/catalog.go` | `CatalogResponse` 新增 `Status`/`Error`；`ReadCatalog` 四态（ready/generating/failed/unavailable）；移除 SVG 作为 ready 内容；新增 `noImageCatalog`/`stripImages` |
| `backend/internal/games/stardew_junimo/installer.go` | `markInstallSucceeded` 改为中间态；`run()` 末尾追加 `runCatalogExportPhase()`；export 结束后才写 `game_installed` |
| `backend/internal/web/lifecycle_handlers.go` | GET 返回状态化 catalog；POST 改为触发后台 export goroutine（持有锁后返回 generating） |
| `backend/internal/games/stardew_junimo/catalog_test.go` | 更新断言：source=fallback → status=unavailable；新增无镜像断言 |
| `frontend/src/types.ts` | `CatalogResponse.status: CatalogStatus`；`source` 变 optional |
| `frontend/src/games/stardew/NewGameCreator.tsx` | 移除 fallback 横幅；加入四态状态横幅；status=generating 时 5 秒轮询；非 ready 时 farm/pet 展示为文字 chip 而非 ImageCard |
| `frontend/src/games/stardew/NewGameCreator.css` | 新增 `ngc-status-banner`（generating/failed/unavailable 三色）和 spinner 动画 |

### 工作原理

**导出流程**（install job 内）：
1. Steam 安装完成 → `markInstallSucceeded()` 写 `export_catalog_queued` 中间相
2. `runCatalogExportPhase()` 执行：
   - 写 `catalog_export.lock` 锁文件（frontend GET 此时返回 `status: "generating"`）
   - 运行 `docker run` 启动一次性 Junimo 容器（只有 game-data + control + mods 挂载，无端口，无 steam-auth）
   - 轮询 control/options.json 出现（bind mount 即时可见），出现后 `docker stop` 容器
   - 成功：删除锁和错误文件；失败：删除锁，写 `catalog_export_error.json`
3. 实例状态写 `game_installed`

**手动重新生成**：
- `POST /api/instances/:id/custom-new-game/catalog` 检查锁是否已有
- 没有 → 原子写锁 → 启动 goroutine → 立即返回 `status: "generating"`
- goroutine 完成后锁释放，下次 GET 返回 ready 或 failed

**前端轮询**：
- `status === 'generating'` 时每 5 秒自动 GET，状态变化后停止轮询
- `status !== 'ready'` 时 farmTypes/petTypes/petBreeds 展示为文字 chip（无图片）
- `status === 'ready'` 时展示 SMAPI 真实素材 ImageCard

### 安全约束满足

- ✅ 没有拼接 Docker shell；`docker run` args 全部为字面值
- ✅ 容器无游戏端口暴露
- ✅ 前端不直接访问 Docker volume，通过 bind mount 经由 Go API 读取
- ✅ SVG 占位图不再作为"真实内容"返回，非 ready 状态图片字段为空
- ✅ 用户触发重新生成走受控 goroutine，非任意 exec

### 如何验证

1. `go test ./...` — 全部通过（含更新后的 catalog 测试）
2. `npm run build` — 无 TypeScript 错误
3. 全新实例安装完成后，`GET /api/instances/:id/custom-new-game/catalog` 应返回 `status: "ready"`（若 export 成功）或 `status: "failed"`（若 Junimo 容器启动失败）
4. 期间打开"新建存档"可看到 `status: "generating"` 横幅 + spinner
5. 关闭新建窗口不会启动正式 server

### 下一步注意事项

1. **Junimo 容器启动依赖**：ExportCatalogContent 假设 Junimo 容器在无 steam-auth 时也能成功启动游戏并运行 SMAPI。若 Junimo 的 entrypoint 阻塞在 steam-auth 连接上，export 会在 10 分钟后 timeout → status=failed，用户需点"重新生成"。后续可调查 Junimo 启动脚本以确认无阻塞。
2. **Windows bind mount 路径**：`toDockerPath` 用 `filepath.ToSlash`，Docker Desktop for Windows 应能正确处理（`E:/foo/bar:/container/path` 格式）。若环境异常，可改用 WSL 路径格式。
3. **catalog 缓存失效**：重装后 options.json mtime 会变，缓存自动失效。如有版本跳变需求可改用 game version 做 key。
4. **dotnet 未安装**：本机无 .NET SDK，SMAPI mod 无法重编译。若需加 `SAP_EXPORT_AND_EXIT` 功能，需在有 dotnet 的环境中 `cd E:\stardew-anxi-panel\smapi-mod && dotnet build` 后替换 DLL。

### 验证结果（2026-06-26）

backend: `go test ./...` — 全部通过
frontend: `npm run build` — 19 modules，无错误

---

## Milestone 7.5 续续篇：asset-exporter 改为 Compose service

### 背景

上一轮 ExportCatalogContent 使用裸 `docker run sdvd/server:...` 配合 `-v` 手动挂载。实际运行发现 Junimo server 镜像强依赖 `STEAM_AUTH_URL` 对应的服务可达，在无 steam-auth 网络下会卡住或直接退出，导致 export 超时。

旧项目 smapi-mod 的 `WritePanelOptions()` 在 `OnGameLaunched` 时无条件写 options.json，正确复用的前提是让 SMAPI 能完整启动到 `OnGameLaunched`，这需要 steam-auth 网络可达。

### 修改文件

| 文件 | 改动说明 |
|------|----------|
| `backend/internal/games/stardew_junimo/compose_template.go` | 新增 `asset-exporter` service（profile: catalog-export），与 server 相同镜像、volume、cap、STEAM_AUTH_URL，但不映射端口，挂载 scratch saves，注入 `SAP_EXPORT_ONLY=true`、`ALLOW_INSECURE_SETUP=true` |
| `backend/internal/games/stardew_junimo/catalog_exporter.go` | `ExportCatalogContent` 改为 `docker compose --profile catalog-export up asset-exporter`；删除 `sanitizeContainerName`/`toDockerPath`；stop/kill 均通过 compose 命令 |
| `backend/internal/games/stardew_junimo/installer.go` | 新增 `migrateAssetExporterService(path)`，向旧实例 compose 文件插入 asset-exporter block；在 `run()` 中与 migrateSteamAuthComposeImage / migrateSavesVolume 并列调用 |

### asset-exporter 与 server 的关键区别

| 属性 | server | asset-exporter |
|------|--------|----------------|
| ports | game/query/vnc/api | 无 |
| saves 挂载 | `.local-container/saves` | `.local-container/catalog-export-saves`（scratch，export 后删除）|
| `ALLOW_INSECURE_SETUP` | `${ALLOW_INSECURE_SETUP:-false}` | 硬编码 `true` |
| `SAP_EXPORT_ONLY` | 无 | `true`（SMAPI mod 未来可据此自退出）|
| Compose profiles | 默认（所有启动） | `catalog-export`（只在 export 时激活）|

### 旧实例迁移逻辑

`migrateAssetExporterService` 检查 compose 文件是否已含 `asset-exporter:`，不含时在 `\nvolumes:\n` 行前插入完整 service block。与 `migrateSteamAuthComposeImage`/`migrateSavesVolume` 同模式，在每次安装 job 的 Step 1 阶段执行。

### 安全约束

- asset-exporter 不暴露任何端口（无 game UDP、无 VNC、无 API）
- scratch saves 目录 export 结束后 `defer os.RemoveAll` 清理，防止生成游戏存档
- stop/kill 均通过 `docker compose --profile catalog-export stop/kill asset-exporter`，无裸 container 名拼接

### 如何验证

1. `go test ./...` — 全部通过（2026-06-26 验证）
2. 新实例安装后查看 docker-compose.yml 应含 `asset-exporter:` block
3. 旧实例下次安装时日志应出现"已在 docker-compose.yml 中添加 asset-exporter 服务"
4. catalog export 阶段日志出现 `[catalog-export] 通过 Compose asset-exporter 启动...`

### 下一步注意事项

- `SAP_EXPORT_ONLY=true` 当前 SMAPI mod 并不识别，mod 会继续运行直到 Go driver `docker compose stop`。若未来 dotnet 环境可用，可在 `OnGameLaunched` 末尾检查该环境变量并 `Environment.Exit(0)` 自退出，从而无需 polling + stop。
- steam-auth 的 session token 已在之前安装时写入 `steam-session` volume，export 时 steam-auth 仅需"在线"不需重新认证。

---

## 邀请码解析正则修复（Galaxy P2P 无横杠格式）（2026-06-26）

### 问题

Galaxy P2P 邀请码格式为 `SGCWS0Z572F2`（12 位纯字母数字，无横杠），但原 `parseInviteCode` 两个正则均要求必须含横杠（`XXXX-XXXX`），导致 `attach-cli invitecode` 返回邀请码后始终无法解析，前端一直停留在"服务器正在启动"。

### 修复

**`backend/internal/games/stardew_junimo/lifecycle.go`**

- `inviteCodePattern` 改为接受 8 位以上无横杠或横杠格式：
  ```
  ([A-Z0-9]{8,}|[A-Z0-9]{4,}-[A-Z0-9]{4,}[A-Z0-9-]*)
  ```
- standalone fallback 正则同步更新。

**`backend/internal/games/stardew_junimo/lifecycle_test.go`**

- 新增三个 Galaxy 无横杠邀请码测试用例（`Invite Code: SGCWS0Z572F2`、括号内、裸行）。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\backend
go test ./...
```

全部通过。

---

## 服务器启动等待：SMAPI status.json 阶段感知（2026-06-26）

### 问题

新建存档 (`POST /api/instances/stardew/saves/custom-new-game`) 后点击启动，服务器一直无法获得邀请码。  
根本原因：**JunimoServer 首次新建存档需要完整的世界生成流程，耗时 5~15 分钟**。

旧代码在容器进入 "running" 状态后（秒级），立即开始轮询邀请码，最多等 8 分钟就放弃。  
8 分钟内 JunimoServer 可能还在做世界生成，attach-cli 永远返回失败，最终打印"未能获取邀请码"。

用户见到的现象：创建存档并启动后任务成功但邀请码为空，或 job 日志静默无输出。

### 修复

**`backend/internal/games/stardew_junimo/lifecycle.go`**

1. **新增 `readSMAPIStatus(dataDir)`**：读取 `.local-container/control/status.json` 中 `state` 字段。
   - SMAPI mod 在每个关键阶段写入此文件：`booting` → `launched` → `save-loaded`
   - 文件不存在时返回 `""`（不阻塞）

2. **新增 `waitForSMAPIReady(ctx, jobCtx)`**：在容器 running 后、`pollInviteCode` 前等待 SMAPI 报告 `save-loaded`。
   - 每 5 秒轮询一次
   - 最长等待 20 分钟（`smapiReadyTimeout`）
   - 每次状态变化都写 job 日志，用户能看到 `[SMAPI] 游戏正在启动中...` → `[SMAPI] 存档已加载，准备获取邀请码...`
   - 超时后仍继续尝试 `pollInviteCode`（非致命）

3. **超时调整**：
   | 常量 | 旧值 | 新值 | 原因 |
   |------|------|------|------|
   | `lifecycleJobTimeout` | 20m | 30m | 容纳 SMAPI 等待 + 邀请码轮询 |
   | `smapiReadyTimeout` | 无 | 20m | 世界生成最长 15min |
   | `smapiCheckInterval` | 无 | 5s | 状态变化不频繁，5s 够用 |
   | `inviteCodePollTimeout` | 8m | 3m | save-loaded 后邀请码应秒级可用 |

4. **`doStart` / `doRestart`** 均插入 `waitForSMAPIReady` 调用。

### 影响的接口/文件

- `backend/internal/games/stardew_junimo/lifecycle.go`：常量 + 2 个新函数 + doStart/doRestart 调用
- 无数据库变更；无新 API；无前端改动

### 存档创建原理（厘清给下一位维护者）

`POST /saves/custom-new-game` **不预先创建存档文件**。它只：
1. 写 `.local-container/settings/server-settings.json`（FarmName、FarmType、Cabins 等）
2. 写 `.local-container/control/server-init.json`（SMAPI mod 用于角色定制，mode=create-or-load）
3. 调用 `driver.Start()` → 启动 docker compose up

**JunimoServer 在容器首次启动时读取 `server-settings.json`，自行生成存档**（Stardew Valley 世界生成）。这需要 5~15 分钟。存档文件落到 `.local-container/saves/Saves/<FarmName>/`。

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\backend
go test ./internal/games/stardew_junimo ./internal/docker ./internal/web
```

全部通过。

实机联调预期：
1. 启动后任务日志出现 `[SMAPI] 游戏正在启动中...`（booting 阶段）
2. 约 1~2 分钟后出现 `[SMAPI] 游戏已启动，正在创建或加载存档（首次新建可能需要 5~15 分钟）...`
3. 存档生成完毕后出现 `[SMAPI] 存档已加载，服务器就绪，准备获取邀请码...`
4. 随后取到邀请码，实例状态变为 running

### 下一步注意事项

- 如果 SMAPI mod 没有正确写 `status.json`（旧 DLL、mod 加载失败、control 目录挂载错误），`waitForSMAPIReady` 会等满 20 分钟后超时；仍然尝试 `pollInviteCode`，如果那时存档已创建完毕，邀请码能取到。
- 若想让 SMAPI 在 Junimo runtime 下也能正确写 `status.json`，确保 `.local-container/control` bind mount 存在且 SMAPI mod DLL 是 Debug build（非 CI-build）。
- `server-init.json` 的 `mode: "create-or-load"` 与 Junimo runtime 配合时，SMAPI mod 的字符定制（`ApplyPanelCharacterCustomization`）只在 `mode=native-create` 时运行；Junimo runtime 下角色外观由 Junimo 的默认值决定，不被 SMAPI 覆盖。

---

## 原生新建存档界面与内置素材（2026-06-26）

### 完成内容

- `frontend/src/games/stardew/NewGameCreator.tsx` / `.css` 已改为接近游戏内新建存档的三栏像素界面：左侧联机小屋、布局、利润率和资金管理；中间玩家/农场/喜好/动物偏好、农场切换、固定勾选的“跳过开场动画”；右侧农场缩略图。
- 按产品要求移除了皮肤、头发、上衣、裤子和配饰控件；保留性别、宠物类型、农场与多人设置。
- “高级设置”已接入三个真实字段：`remixedCommunityCenter`、`remixedMineRewards`、`spawnMonstersOnFarm`。后端将它们写入 Junimo `server-settings.json` 的 `bundlesRemix`、`minesRemix`、`spawnMonstersAtNight`；同时写入 `server-init.json` 供控制链路审计。`skipIntro` 强制为 `true`。
- 使用本机正在运行的 Junimo 游戏文件与完整 SMAPI 反射导出模组，一次性导出了真实农场与宠物图块；PNG 已提交到 `frontend/public/assets/stardew/new-game/{farms,pets}/`。Vite 构建确认会把它们复制至 `dist/assets/stardew/new-game/`，不依赖用户机器的 Steam 下载，也未被 `.gitignore` 排除。
- 删除了运行时素材导出链路：`catalog.go`、`catalog_exporter.go`、catalog API、`asset-exporter` Compose 服务、安装阶段 exporter job 和前端轮询均已移除。安装完成会直接进入 `game_installed`。
- 旧实例会在下次安装流程由 `migrateRemoveAssetExporterService()` 自动移除 `asset-exporter` 服务。本机开发实例的旧 exporter 服务、缓存目录和停止容器也已清理。

### 影响的接口/文件

- 请求 DTO：`registry.NewGameConfig` 和前端 `NewGameConfig` 新增上述三个高级布尔设置及 `skipIntro`。
- 创建接口不变：`POST /api/instances/:id/saves/custom-new-game`。
- 移除接口：`GET/POST /api/instances/:id/custom-new-game/catalog`。
- 前端素材位于 `frontend/public/assets/stardew/new-game/`，应与面板镜像一同发布，不应搬回实例数据目录或用户运行时生成。

### 验证

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...

cd ..\frontend
npm.cmd run build
```

两项均已通过；`frontend/dist/assets/stardew/new-game/` 已确认包含农场和宠物 PNG。

### 下一步注意事项

- 若要更新素材，必须在维护者的本地游戏/Junimo 环境一次性重新运行反射导出，再更新 `frontend/public/assets/stardew/new-game/` 并提交；禁止恢复用户侧 `asset-exporter` 或 catalog 轮询。
- 需要真实浏览器截图回归时，可在面板可静态服务前端构建物的运行方式落定后，对新建存档 modal 做桌面和窄屏检查。

---

## 新建存档截图素材与草原农场补齐（2026-06-26）

### 完成内容

- 维护者提供的六张已裁切游戏截图已原样写入 `frontend/public/assets/stardew/new-game/`：`characters/{male-preview,female-preview}.png`、`gender/{male-icon,female-icon}.png`、`cabins/{nearby,separate}.png`。前端直接引用这些 PNG，不再使用 emoji 或重新绘制的替代图。
- `NewGameCreator` 右侧农场列表新增 `meadowlands`（草原农场），对应既有 `farms/meadowlands.png`；后端校验与 `whichFarm=7` 映射同步补齐。
- 删除角色区里重复的猫/狗切换。动物偏好现在只保留与原游戏一致的左右循环，并按本机导出的游戏数据顺序循环 5 个猫品种后 5 个狗品种；每步同时更新 `petType`、`petBreed`、`petBreedId`。
- 后端允许游戏实际的 `0..4` 品种 ID，并拒绝 `petBreed` 与 `petBreedId` 不一致的请求。`server-init.json` 保留性别、宠物类型、宠物品种和小屋布局；控制模组已更新为在 Junimo 的 `create-or-load` 事件也应用这些选择，避免界面选项只停留在表单层。

### 验证

- `go test ./...` 通过（新增草原农场、品种映射及 init 配置断言）。
- `npm.cmd run build` 通过。

### 下一步注意事项

- 新增或替换截图素材时保持与 `NewGameCreator.tsx` 的路径和 ID 一一对应；不需要恢复用户运行时素材导出流程。

---

## 自定义新建存档修复（2026-06-26）

### 问题

点击"创建存档并启动"后，面板填写的自定义配置（农场名、农场类型、利润率等）完全不生效，JunimoServer 始终使用默认值创建存档。存档重启后从前端消失。

### 根因分析

叠加了三个问题：

**1. `server-settings.json` 使用扁平 JSON 结构，JunimoServer 期望嵌套结构**

面板写的格式：
```json
{"FarmName": "xxx", "FarmType": 6, "StartingCabins": 0, ...}
```

JunimoServer 的 C# 类 `ServerSettings` 期望：
```json
{"Game": {"FarmName": "xxx", "FarmType": 6, ...}, "Server": {"SeparateWallets": false}}
```

`JsonConvert.DeserializeObject<ServerSettings>(json)` 对扁平 JSON 无法映射到 `Game`/`Server` 子对象，全部字段回退默认值。

**2. `docker-compose.yml` 是旧版，saves 使用 Docker named volume**

旧 compose 文件：
```yaml
- saves:/config/xdg/config/StardewValley  # named volume，宿主机不可见
```

正确应该是 bind mount：
```yaml
- ./.local-container/saves:/config/xdg/config/StardewValley
```

导致存档写入 Docker 内部 volume，宿主机 `.local-container/saves/Saves/` 永远为空，前端 `ListSaves` 永远返回零存档。

**3. 缺少 control/mods bind mount 和 SAP_CONTROL_DIR 环境变量**

旧 compose 没有以下配置：
```yaml
- ./.local-container/control:/data/control
- ./.local-container/mods/StardewAnxiPanel.Control:/data/Mods/StardewAnxiPanel.Control
```
```yaml
SAP_CONTROL_DIR: /data/control
```

导致 `server-init.json` 和 SMAPI mod 无法进入容器。

### 修复

| 文件 | 改动 |
|------|------|
| `backend/internal/games/stardew_junimo/saves.go` | `WriteServerSettings` 改为嵌套 `Game`/`Server` 结构；字段名改为 JunimoServer 官方 PascalCase（`FarmName`、`FarmType`、`StartingCabins`、`CabinLayoutNearby`、`ProfitMargin`、`PetBreed`、`SeparateWallets`、`RemixBundles`、`RemixMines`、`SpawnMonstersAtNight`）；`SpawnMonstersAtNight` 从 bool 改为 string（`"true"/"false"`）；新增 `DeleteAllSaves`、`SetActiveSave` 函数；`server-init.json` mode 改为 `native-create` |
| `backend/internal/games/stardew_junimo/lifecycle.go` | 新增 `sendNewGameCommand` 方法，通过 JunimoServer HTTP API `POST /newgame` 创建新存档（替代失败的 `attach-cli settings newgame --confirm`）；`doStart` 在 `newGame=true` 时调用 |
| `backend/internal/games/registry/types.go` | `StartRequest` 新增 `NewGame bool` 字段 |
| `backend/internal/web/lifecycle_handlers.go` | `handleSavesCustomNewGame` 不再删除旧存档，改为设置 `NewGame=true` 让 lifecycle job 通过 API 创建新存档；`handleSavesUploadCommitAndStart` 导入后调用 `SetActiveSave` 设置活跃存档 |
| `backend/internal/games/stardew_junimo/saves_test.go` | 更新断言为嵌套结构（`Game.FarmName`、`Server.SeparateWallets` 等） |
| 实例 `docker-compose.yml` | 手动修复：saves 改为 bind mount；新增 control/mods bind mount；新增 `SAP_CONTROL_DIR` 环境变量；移除旧 named volume `saves:`/`settings:` |

### 关键发现

**JunimoServer 不读 `SETTINGS_PATH` 环境变量。** 虽然官方 compose 模板设置了 `SETTINGS_PATH=/data/settings/server-settings.json`，但 JunimoServer 的 SMAPI mod 通过 `IModHelper` 读取配置，`SETTINGS_PATH` 实际被 `ServerSettingsLoader.ResolveSettingsPath()` 使用——它先检查环境变量，不存在则回退到 mod 目录下的 `server-settings.json`。所以 `SETTINGS_PATH` 是生效的，但前提是 JSON 结构必须匹配 C# 类的嵌套格式。

**JunimoServer 有 HTTP API。** `POST /newgame` 可以直接创建新存档，`POST /reload` 可以重新加载配置。比 `attach-cli` 更可靠（不需要 TTY）。

**JunimoServer 用 `junimohost.gameloader.json` 记住上次加载的存档。** 文件位于 `/config/xdg/config/StardewValley/.smapi/mod-data/junimohost.server/junimohost.gameloader.json`，字段 `SaveNameToLoad` 控制启动时加载哪个存档。`SetActiveSave` 函数直接修改此文件来切换存档。

### 存档操作流程

| 操作 | 流程 |
|------|------|
| 创建存档并启动 | 写 `server-settings.json` + `server-init.json` → 启动服务器 → 等待 API 就绪 → `POST /newgame` → 等待新存档出现 → 获取邀请码。旧存档保留。 |
| 上传存档并启动 | `ImportSaveToVolume` 导入 → `SetActiveSave` 写 gameloader 配置 → 启动。旧存档保留。 |
| 普通启动 | 直接启动，JunimoServer 加载 gameloader 记录的存档。 |
| 切换存档 | `SetActiveSave` 修改 gameloader 配置 → 重启。 |

### 已知限制

农夫名字（FarmerName）、性别、外貌等角色字段**无法通过 server-init.json 写入 Junimo runtime 的存档**。SMAPI mod 的 `ApplyPanelCharacterCustomization` 只在原生 Stardew `SaveCreating` 事件中生效，JunimoServer 的 `/newgame` 走自己的存档创建路径，不触发该事件。这些字段需要预置存档模板（`saves-templates`）方案解决，当前未实现。

| 字段 | 来源 | Junimo runtime 是否生效 |
|------|------|------------------------|
| FarmName | `server-settings.json` | ✅ |
| FarmType | `server-settings.json` | ✅ |
| StartingCabins | `server-settings.json` | ✅ |
| ProfitMargin | `server-settings.json` | ✅ |
| PetBreed | `server-settings.json` | ✅ |
| CabinLayoutNearby | `server-settings.json` | ✅ |
| SeparateWallets | `server-settings.json` | ✅ |
| RemixBundles/Mines | `server-settings.json` | ✅ |
| SpawnMonstersAtNight | `server-settings.json` | ✅ |
| FarmerName | `server-init.json` + SMAPI mod | ❌ 需要存档模板 |
| Gender | `server-init.json` + SMAPI mod | ❌ 需要存档模板 |
| FavoriteThing | `server-init.json` + SMAPI mod | ❌ 需要存档模板 |
| 外貌 | `server-init.json` + SMAPI mod | ❌ 需要存档模板 |

### 验证

```powershell
cd E:\stardew-server-anxi-panel\backend
go test ./...
```

全部通过。

### 下一步注意事项

1. **老实例需要手动更新 docker-compose.yml**：当前实例 compose 已手动修复。新实例由 `Prepare()` 生成的模板已包含正确配置。老实例如需自动迁移，需在 `installer.go` 中新增迁移函数。
2. **存档模板方案**：若要支持 FarmerName/外貌等角色字段，需在 `.local-container/saves-templates/` 预置真实 Stardew 存档，创建时复制并 patch XML 中的农场名等字段。参考 `E:\stardew-anxi-panel` 的 `ModEntry.StartNativeCreate()` 实现。
3. **`POST /newgame` 会创建新存档但不会删除旧的**：用户可创建多个存档，通过 `SetActiveSave` 切换。Milestone 9 可扩展完整存档管理 UI。
4. **`junimohost.gameloader.json` 是 JunimoServer 内部文件**：面板通过 bind mount 直接修改它。如果 JunimoServer 未来版本改变此文件格式，需要同步更新 `SetActiveSave`。
