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
