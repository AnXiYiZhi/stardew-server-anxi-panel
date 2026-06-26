# Conversation Handoff 2026-06-23

## Milestone 5: GameDriver Registry + Instance Model

完成 Milestone 5。

- 新增实例模型 `instances`，把前面临时写死的 Stardew 单实例路径收口到 `instances + driver_id + GameDriver registry`。
- 新增后端配置项：`PANEL_MODE`、`DEFAULT_INSTANCE_ID`、`DEFAULT_DRIVER_ID`，默认 `single` / `stardew` / `stardew_junimo`。
- 后端启动后会确保默认 Stardew instance 存在，但本阶段不会拉 Junimo 镜像、不会执行 Steam Auth、不会启动服务器。
- 新增 `GameDriver` 接口和 registry，实现 `Register`、`Get`、`List`，重复注册和找不到 driver 都返回明确错误。
- 新增 `stardew_junimo` driver 骨架：`Prepare` 只确保目录存在，`Status` 通过 Compose PS 返回基础容器状态，其他能力返回 `not_implemented`。
- 新增 instance-based API，前端主路径切到默认 Stardew instance。
- 前端继续 Single Game Mode：登录后直接进入当前 Stardew 面板，不显示总面板/游戏列表。

### 影响的接口和文件

后端：

- `backend/internal/config/config.go`
- `backend/migrations/004_instances.sql`
- `backend/internal/storage/instances.go`
- `backend/internal/storage/instances_test.go`
- `backend/internal/games/registry/errors.go`
- `backend/internal/games/registry/types.go`
- `backend/internal/games/registry/registry.go`
- `backend/internal/games/registry/registry_test.go`
- `backend/internal/games/stardew_junimo/driver.go`
- `backend/internal/games/stardew_junimo/driver_test.go`
- `backend/cmd/panel/main.go`
- `backend/internal/web/handler.go`
- `backend/internal/web/instance_handlers.go`
- `backend/internal/web/docker_handlers.go`
- `backend/internal/web/auth_handlers.go`

前端：

- `frontend/src/types.ts`
- `frontend/src/api.ts`
- `frontend/src/App.tsx`

文档：

- `README.md`
- `docs/handoff-roadmap.md`
- `docs/conversation-handoff-2026-06-23.md`

### 数据库变化

新增迁移：`backend/migrations/004_instances.sql`。

新增表：

```text
instances
  id
  driver_id
  name
  data_dir
  state
  state_message
  driver_phase
  driver_payload
  created_at
  updated_at
```

兼容策略：

- 旧 `instance_state` 表保留。
- `EnsureDefaultInstance` 优先读取新 `instances` 表。
- 新表不存在默认记录时，会尝试从旧 `instance_state` 迁移默认 instance 的 `state` 和 `state_message`。
- 默认 instance 已存在时不会覆盖已有状态。

### 接口变化

新增：

```text
GET /api/instances
GET /api/instances/:instance_id
GET /api/instances/:instance_id/state
GET /api/instances/:instance_id/status
GET /api/instances/:instance_id/docker/ps
```

行为说明：

- `GET /api/instances` 和 instance 详情需要登录。
- `GET /api/instances/stardew/state` 返回通用 state、state_message、driver_phase。
- `GET /api/instances/stardew/status` 通过 `stardew_junimo` driver 返回基础状态。
- `GET /api/instances/stardew/docker/ps` admin-only，使用 `instances.data_dir` 作为 Compose 工作目录。
- `/api/docker/status`、`/api/docker/ps`、`/api/docker/logs` 仍保留为 admin-only 兼容/调试入口，其中默认 Compose 目录也读取默认 instance。
- 未找到 instance 返回 404 `instance_not_found`。
- instance 指向未注册 driver 返回 500 `driver_not_registered`。

### 如何验证

后端：

```bash
cd backend
go test ./...
```

本次运行结果：通过。

前端：

```bash
cd frontend
npm run build
```

本次运行结果：通过。

本机联调：

```bash
cd backend
PANEL_DATA_DIR=./data PANEL_DB_PATH=./data/panel.db go run ./cmd/panel
```

另开终端：

```bash
cd frontend
npm run dev
```

打开 Vite 输出地址，通常为：

```text
http://localhost:5173
```

验证流程：

1. 新数据库首次打开应进入管理员初始化页。
2. 创建管理员后自动进入主界面。
3. 登录后仍然直达 Stardew 面板，不显示总面板/游戏列表。
4. 状态卡应显示 `Stardew Valley` 和 `Driver: stardew_junimo`。
5. 登录后可请求 `GET /api/instances`，应看到默认 `stardew` instance。
6. 登录后可请求 `GET /api/instances/stardew`，应看到 instance 详情。
7. 登录后可请求 `GET /api/instances/stardew/state`，应看到通用 `state` 和 `driverPhase`。
8. 登录后可请求 `GET /api/instances/stardew/status`，应走 `stardew_junimo` driver 返回基础状态。
9. 管理员点击“查看 Compose PS”或请求 `GET /api/instances/stardew/docker/ps`，后端会使用 instance 的 `data_dir`。没有 compose 文件时返回 `compose_project_not_ready`。
10. 普通用户不能访问 `/api/instances/stardew/docker/ps` 或 `/api/docker/*`。

### 下一步注意事项

- Milestone 6 应开始实现 Stardew Junimo Prepare and Install。
- 真实 Junimo 安装、Steam Auth、启动流程必须通过 `games/stardew_junimo` driver + Job Manager。
- 不要把 Stardew 业务写进 `web`、`docker`、`jobs` 顶层。
- 当前 `Prepare` 只创建 instance 目录，不代表 Junimo compose/env 已准备。
- 继续禁止日志泄露 Steam 密码、VNC 密码、session token、secret。
- 后续如果新增第二个 driver，应只新增对应 driver 并注册，不应改 auth/jobs/storage/docker 通用层。

---

## Milestone 6: Stardew Junimo Prepare and Install

完成 Milestone 6。

### 改了什么

后端新增/修改文件：

- `backend/internal/games/stardew_junimo/config/env.go` — 新增，管理 .env 文件的安全读写（合并、不覆盖未知键、0600 权限）。
- `backend/internal/games/stardew_junimo/config/env_test.go` — 新增，四个测试覆盖 NotExist/NewFields/UpdateExisting/PreserveUnknown；Windows 下跳过权限断言。
- `backend/internal/games/stardew_junimo/compose_template.go` — 新增，嵌入 JunimoServer docker-compose.yml 常量。
- `backend/internal/games/stardew_junimo/installer.go` — 新增，`installRunner` 三步安装流程（写 .env → docker compose pull → steam-auth download），guard channel 机制，所有密码字段不写日志。
- `backend/internal/games/stardew_junimo/driver.go` — 重写，新增 `Install()`、`SendSteamGuardInput()`、`Prepare()`（写 compose/env，不覆盖已有文件）。`StateStore` 和 `DockerService` 接口化，便于测试 mock。
- `backend/internal/games/stardew_junimo/driver_test.go` — 更新，覆盖 Prepare 不覆盖、Install 无 Job Manager 错误、Install 空凭据校验、无活跃 job 的 SendSteamGuardInput。
- `backend/internal/games/registry/types.go` — 更新，`InstallRequest` 新增 `SteamUsername`、`SteamPassword`、`VNCPassword` 字段；新增 `SteamGuardSender` 接口。
- `backend/internal/web/install_handlers.go` — 新增，三个 handler：`handleInstancePrepare`、`handleInstanceInstall`、`handleInstanceSteamGuardInput`。
- `backend/internal/web/instance_handlers.go` — 更新，三条新路由分支。
- `backend/cmd/panel/main.go` — 更新，`stardew_junimo.New()` 追加 `jobManager`、`store` 两个参数。

前端新增/修改文件：

- `frontend/src/types.ts` — 追加 `InstallJobResponse`、`PrepareResponse` 类型。
- `frontend/src/api.ts` — 追加 `prepareInstance`、`installInstance`、`submitSteamGuardInput` 函数。
- `frontend/src/App.tsx` — 全面更新，新增 `InstallSection` 组件（Prepare 按钮、安装 Modal、Steam Guard 输入区、状态轮询）和对应状态逻辑。
- `frontend/src/App.css` — 追加安装区域、Steam Guard 区域和 Modal 样式。

### 影响的接口

新增：

```text
POST /api/instances/:id/prepare
POST /api/instances/:id/install
POST /api/instances/:id/steam-guard/input
```

`POST /api/instances/:id/prepare`

- 需要 admin。
- 创建实例目录（saves/mods/.local-container）。
- 首次写入 docker-compose.yml（内嵌模板，已有则跳过）。
- 首次写入 .env（空模板，已有则跳过）。
- 从 `admin_created`/`uninitialized` 状态推进到 `junimo_scaffolded`。
- 返回实例状态 JSON（含 state、stateMessage、driverPhase）。

`POST /api/instances/:id/install`

- 需要 admin。
- Body：`{steamUsername, steamPassword, vncPassword}`（均不能为空）。
- 写入 .env 凭据，执行 docker compose pull，执行 steam-auth download。
- 返回 `{jobId}` 202 状态，前端通过 SSE 跟踪 job 日志。
- 密码字段永不出现在日志、响应 JSON 或 audit metadata。

`POST /api/instances/:id/steam-guard/input`

- 需要 admin。
- Body：`{jobId, input}`。
- type assert driver 为 `registry.SteamGuardSender`，写入 guard channel。
- 日志只记录"guard input submitted"，不记录实际验证码。

### 如何验证

```bash
cd backend
go test ./...
```

本次运行结果：通过（含 config/env_test.go 全部四个用例）。

```bash
cd frontend
npm run build
```

本次运行结果：通过，无 TypeScript 错误。

本机联调端到端验证建议：

1. 启动后端和前端开发服务器。
2. 管理员登录后，`InstallSection` 在 `admin_created` 状态下显示"准备 Junimo 目录"按钮。
3. 点击准备后状态变为 `junimo_scaffolded`，出现"安装游戏"按钮。
4. 点击安装游戏，弹出 Modal，填写三项（密码框 `type=password`）。
5. 提交后返回 `jobId`，Modal 关闭，job log 区显示实时安装日志。
6. 若 steam 返回 Guard 提示，页面出现验证码输入框，提交后继续。
7. 安装成功后状态变为 `game_installed`，显示"已安装"徽标和 disabled"启动服务器"按钮。
8. 检查 docker exec 日志，确认无 STEAM_PASSWORD / VNC_PASSWORD 明文出现。

### 下一步注意事项

- Milestone 7 实现 Server Lifecycle（start/stop/restart/status/invite-code）。
- `Start()` 必须检查 `game_installed` 状态和存档选择状态，未满足时返回明确错误。
- 不要把 start/stop 逻辑写进 `web` 顶层，必须通过 `stardew_junimo` driver + Job Manager。
- PTY 当前用 `-i` 模式（非 `-it`），如需真实终端交互（如 QR 码渲染）可后续引入 `creack/pty`，但要先验证 Windows 跨平台兼容性。
- 如果 docker 环境不可用，Prepare 仍会创建本地目录和文件；Install 任务会在 docker compose pull 步骤失败并写入 job log。
- `SendSteamGuardInput` 向 channel 发送时最多等 5 秒，超时返回错误（防止 install job 已结束但前端仍发 guard input 导致阻塞）。

---

## Milestone 6 补丁：安装流程 UX 改进 + 版本检测

### 改了什么

**后端：**

- `backend/internal/games/registry/types.go` — `ImageTagOption` 新增 `IsLatest bool` 字段，表示当前 recommended 版本是否就是 Docker Hub 上的最新版。
- `backend/internal/web/install_handlers.go` — 三项改动：
  1. `installRequestBody` 新增 `ReuseCredentials bool`。若为 `true`，从实例 `.env` 读取已保存凭据，无需前端重传。
  2. `handleInstanceInstall` 逻辑重排：先 `loadInstance` 获取 `DataDir`，再按需读 `.env`，最后校验非空。
  3. `handleInstanceInstallOptions` 在返回前联网查询 Docker Hub `sdvd/steam-service` 的 tags 列表（5 秒超时，失败静默），若 `TestedImageTag` 是最新 non-latest tag，则 recommended option 的 `IsLatest = true`。新增 helper：`checkTestedTagIsLatest`。

**前端：**

- `frontend/src/types.ts` — `ImageTagOption` 新增 `isLatest?: boolean`。
- `frontend/src/api.ts` — `installInstance` 参数全部改为可选，新增 `reuseCredentials?: boolean`。
- `frontend/src/App.tsx` — 五项改动：
  1. `lastTriedCredentials` state：记录上一次填写的 Steam 账号/密码/VNC 密码。
  2. 凭据错误（`authFailed || phase === 'credentials_required'`）时打开 modal，自动预填上次输错的凭据，方便用户核对和修改。
  3. pull 失败（`state === 'junimo_scaffolded' && phase === 'pull_failed'`）时点击"重试安装"直接调用 `reuseCredentials: true` 接口，不弹 modal。
  4. 安装 modal 中 Steam 用户名 `autoComplete="steam-account"`，密码改为 `autoComplete="new-password"`，防止浏览器用面板账号自动填充。
  5. 版本下拉选项：`isLatest` 为真时在 ★ 后追加"已是最新版"。

### 影响的接口

`POST /api/instances/:id/install` body 新增可选字段 `reuseCredentials: true`。后端优先从 `.env` 读凭据，再做非空校验。

`GET /api/instances/:id/install-options` 响应 `imageTagOptions[].isLatest` 新增（可能为 `false` 或缺失，表示网络查询失败或不是最新版）。

### 如何验证

```bash
cd backend && go test ./...   # 通过
cd frontend && npm run build  # 通过
```

**端到端验证：**

1. pull 失败（网络错误 / 镜像不存在）→ 状态变为 `junimo_scaffolded` / phase `pull_failed` → 点击"重试安装"不弹 modal，直接重试。
2. Steam 密码错误 → 状态变为 `steam_auth_failed` → 点击"重新安装（凭据错误）"弹出 modal，上次填的账号和密码已预填。
3. 首次安装（`admin_created`）→ 弹 modal，所有字段为空。
4. 安装 modal 打开时，浏览器不自动填充面板登录的账号密码（通过非标准 `autoComplete` 值阻止）。
5. 打开 `GET /api/instances/stardew/install-options`，若网络正常应能看到 `isLatest: true/false`。

### 下一步注意事项

- `checkTestedTagIsLatest` 使用 Docker Hub 公开 API，无需认证，但有频率限制。目前仅在 install-options 请求时调用（用户操作触发），频率低。若日后需要缓存可在 driver 或 handler 层加 TTL。
- `lastTriedCredentials` 保存在 React state，刷新页面后清空。这是故意设计：密码不应持久化到 `localStorage`。

---

## Milestone 6 补丁：Steam 认证方式选择 + 二维码展示

### 改了什么

**后端：**

- `backend/internal/games/stardew_junimo/installer.go`
  - `lineHandler` 新增检测 `Choose authentication method`，把实例 `driver_phase` 更新为 `auth_method_required`。
  - 检测 `QR Code` / `scan the qr` 等输出后更新为 `steam_qr_required`。
  - stdin forwarder 的日志文案改为通用“Steam 认证输入已提交”，仍不记录原始输入。
- `backend/internal/web/install_handlers.go`
  - `POST /api/instances/:id/steam-guard/input` 继续复用现有 stdin 通道提交认证交互输入。
  - 当当前阶段是 `auth_method_required` 时，只允许提交 `1` 或 `2`，避免变成任意 stdin 入口。
  - handler 日志改为 `steam auth input submitted`，不记录验证码或认证方式值。

**前端：**

- `frontend/src/App.tsx`
  - 安装进度识别 `auth_method_required` 和 `steam_qr_required`。
  - 当阶段为 `auth_method_required` 时，显示“使用账号密码 / 使用手机扫码”选择区。
  - 选择“账号密码”提交 `1`，容器会继续使用已写入 `.env` 的 Steam 账号密码。
  - 选择“手机扫码”提交 `2`，之后阶段进入 `steam_qr_required` 时，从当前安装任务日志提取 Steam 输出并以等宽文本块完整显示，便于扫描。
  - 若页面刷新后仍有运行中的 `stardew_install` 任务，会自动选中该任务并继续轮询状态。
- `frontend/src/App.css`
  - 新增认证方式按钮布局和二维码日志文本块样式。

### 影响的接口

无新增接口。

复用：

```text
POST /api/instances/:id/steam-guard/input
Body: { jobId, input }
```

在 `auth_method_required` 阶段：

- `input: "1"` 表示账号密码。
- `input: "2"` 表示二维码。
- 其他值返回 400 `invalid_field`。

### 如何验证

```bash
cd backend && go test ./...
cd frontend && npm run build
```

端到端验证建议：

1. 启动安装，等待 job log 出现 `Choose authentication method`。
2. 实例状态应变为 `steam_auth_running`，`driverPhase` 应为 `auth_method_required`。
3. 前端安装区显示“选择 Steam 认证方式”。
4. 点击“使用账号密码”后，后端向 steam-auth stdin 写入 `1`，job log 只显示“Steam 认证输入已提交”，不显示密码或输入值。
5. 重新安装并在同一阶段点击“使用手机扫码”，后端写入 `2`。
6. 容器输出二维码后，阶段应进入 `steam_qr_required`，前端显示等宽二维码日志块。
7. 检查 job log / 后端日志 / 响应 JSON，确认没有 STEAM_PASSWORD、VNC_PASSWORD 或验证码明文。

### 下一步注意事项

- 当前二维码展示基于 steam-auth 的文本输出，不引入前端 QR 库；如果上游只输出 URL 而不是 ASCII QR，可后续在前端用该 URL 渲染二维码，但要先确认输出格式。
- `POST /steam-guard/input` 名称历史上是 Guard 输入，当前也承载认证方式选择。若后续交互类型继续增加，可新建更通用的 `steam-auth/input` 路由并保留旧路由兼容。

---

## Milestone 6 补丁：安装超时可重试 + 跳过已有镜像拉取

### 改了什么

**后端：**

- `backend/internal/docker/compose.go`
  - 新增 `ImageInspect(ctx, dir, imageRef)`，封装 `docker image inspect <image>`。
- `backend/internal/games/stardew_junimo/driver.go`
  - `DockerService` 接口新增 `ImageInspect`，供 driver 检查固定 Junimo 镜像是否已存在。
- `backend/internal/games/stardew_junimo/installer.go`
  - 安装 Step 2 改为先检查本地是否已有 `sdvd/steam-service:<tag>` 和 `sdvd/server:<tag>`。
  - 两个镜像都存在时跳过 `docker compose pull`，直接进入 Steam 认证。
  - 缺少任一镜像时才执行 `docker compose pull`。
  - pull 失败和 Steam auth 超时时，使用 `context.Background()` 写回实例状态，避免 job context 已超时/取消导致状态仍停在 `steam_auth_running`。
  - Steam auth 超时后状态写为 `error`，`driver_phase` 写为 `install_timeout`，前端可显示“重试安装”。
- `backend/internal/games/stardew_junimo/driver_test.go`
  - fake docker 补齐 `ImageInspect`。

**前端：**

- `frontend/src/App.tsx`
  - 识别 `install_timeout` phase，进度条显示错误状态和“安装任务超时，请重试安装”。
  - `install_timeout` 下 Steam 认证步骤标红，且实例 `state=error` 时沿用已有“重试安装”按钮逻辑。

### 影响的接口

无新增 HTTP API。

新增后端内部 Docker 能力：

```text
docker image inspect sdvd/steam-service:<tag>
docker image inspect sdvd/server:<tag>
```

镜像名和 tag 都由后端固定逻辑生成，前端不能传任意镜像名或 Docker 参数。

### 如何验证

```bash
cd backend && go test ./...
cd frontend && npm run build
```

本次验证结果：两者通过。

端到端验证建议：

1. 本地已有 `sdvd/steam-service:<tag>` 和 `sdvd/server:<tag>` 时启动安装，job log 应显示“本地已存在 Junimo 镜像，跳过 docker compose pull。”
2. 删除或更换 tag 后启动安装，job log 应显示“本地缺少 Junimo 镜像，正在执行 docker compose pull...”。
3. 让 steam-auth 卡住直到安装任务超时，实例状态应变为 `error`，phase 应为 `install_timeout`。
4. 前端应显示错误进度和“重试安装”按钮，不再停留在 Steam 认证进行中。
5. 点击“重试安装”应使用 `.env` 中已有凭据，不要求重新输入 Steam/VNC，除非之前是凭据错误。

---

## Milestone 6 补丁：二维码登录失败明确提示

### 问题现象

Junimo `steam-auth` 在用户选择二维码登录后，可能在二维码生成前输出：

```text
QR authentication failed: The SteamClient instance must be connected.
Login failed
No accounts logged in, skipping game download
```

这表示上游容器的 QR 登录流程在生成二维码前已经失败，面板没有二维码可以展示。

### 改了什么

**后端：**

- `backend/internal/games/stardew_junimo/installer.go`
  - 新增 `qrAuthFailed` 状态。
  - `QR authentication failed` 单独识别为二维码认证失败。
  - 失败时写入 `steam_auth_failed`，`driver_phase=qr_auth_failed`。
  - job log 输出明确错误：`Steam 二维码登录失败：SteamClient 未连接，请改用账号密码或 Steam Guard。`

**前端：**

- `frontend/src/App.tsx`
  - `qr_auth_failed` 显示明确错误：“二维码登录失败，请改用账号密码或 Steam Guard”。
  - 重试按钮文案改为“二维码失败，改用账号密码重试”。
  - 安装 modal 标题和说明改为引导用户改用账号密码登录，后续如需二次验证再输入 Steam Guard。

### 如何验证

```bash
cd backend && go test ./...
cd frontend && npm run build
```

本次验证结果：两者通过。

---

## Milestone 6 补丁：已安装状态增加本地安装产物校验

### 问题现象

此前 Steam 认证失败被误判成功后，数据库已写入 `game_installed`。即使重新构建或重启面板，前端仍只读数据库状态，所以继续显示“已安装”，但 `.local-container` 里没有实际游戏安装文件。

### 改了什么

**后端：**

- `backend/internal/games/stardew_junimo/driver.go`
  - 新增 `ReconcileState(ctx, instance)`。
  - 当状态是 `game_installed` / `save_required` / `ready_to_start` 时，检查实例目录下 `.local-container` 是否有文件。
  - 如果 `.local-container` 不存在或为空，写回 `error`，`driver_phase=install_missing`，提示“未检测到游戏安装文件，请重新安装”。
- `backend/internal/web/instance_handlers.go`
  - `GET /api/instances/:id` 和 `GET /api/instances/:id/state` 返回前，如果 driver 支持 `ReconcileState`，先让 driver 校验并纠正状态。

### 影响

- 旧数据库里已经误写的 `game_installed` 会在下一次读取实例状态时自动纠正。
- 前端会看到 `state=error`，从而显示“重试安装”按钮。
- 校验逻辑放在 `stardew_junimo` driver 内，web 层只做可选接口调用，不写 Stardew 具体路径规则。

### 如何验证

```bash
cd backend && go test ./...
```

本次验证结果：通过。

---

## Milestone 6 补丁：Steam 认证失败不再误判成功

### 问题现象

用户选择二维码登录后，steam-auth 日志出现：

```text
QR authentication failed: The SteamClient instance must be connected.
Login failed
No accounts logged in, skipping game download
```

但容器退出码可能仍为 0，旧逻辑按退出码 0 写入 `game_installed`，导致前端显示“Steam 认证成功，安装完成”。

### 改了什么

**后端：**

- `backend/internal/games/stardew_junimo/installer.go`
  - 失败关键词新增：`qr authentication failed`、`no accounts logged in`、`skipping game download`。
  - `authFailed` 判定优先于成功判定。
  - 不再仅凭 `exitCode == 0` 判定安装成功；必须看到登录成功关键词（例如 `logged in`、`login successful`）才写入 `game_installed`。
  - 认证失败时写入 `steam_auth_failed` / `credentials_required`，job 返回失败，前端会要求用户重试认证。

### 如何验证

```bash
cd backend && go test ./...
```

本次验证结果：通过。

---

## Milestone 6 补丁：前端兜底识别旧超时安装任务

### 改了什么

**前端：**

- `frontend/src/App.tsx`
  - 新增派生判断：当最近的 `stardew_install` 任务已失败且错误信息包含“超时”，并且当前没有运行中的安装任务，但 instance 仍旧停在 `steam_auth_running` 时，前端临时按 `state=error`、`phase=install_timeout` 展示。
  - 这样旧版本 runner 已经造成的卡死状态也能在前端出现“重试安装”按钮，不需要手动改数据库。
  - 后续新安装任务仍以真实 instance state 为准；只在“任务中心已经失败/超时但 instance 未同步”的不一致状态下兜底。

### 如何验证

```bash
cd frontend && npm run build
```

本次验证结果：通过。

---

## Milestone 6 补丁：同步官方 Junimo compose/env + 修复账号密码认证卡住

### 问题现象

用户在 `steam-auth setup` 输出以下内容后点击“使用账号密码登录”：

```text
Choose authentication method:
[1] Username & Password
[2] QR Code (Steam Mobile App)
Steam 认证输入已提交（原始内容不记录日志）。
```

旧实现曾只向 stdin 写入 `1`，但 Junimo 上游选择 `[1] Username & Password` 后还会继续读取 Steam 用户名和 Steam 密码；进一步验证后发现上游密码读取使用 `Console.ReadKey()`，在面板后台任务的 stdin 重定向环境中会直接报 `Cannot read keys when either application does not have a console or when console input has been redirected`。因此最终修复改为绕过 `setup` 的账号密码交互分支，使用 `steam-auth download` 的非交互 credentials 路径。

同时，旧内嵌 compose 模板是简化版，和 Junimo 官方 compose/env 偏差较大：使用 `JUNIMO_IMAGE_TAG`，缺少官方 `steam-session`、`game-data`、`saves`、`settings` volume 结构，也没有保留 `stdin_open` / `tty`。

### 改了什么

**后端：**

- `backend/internal/games/stardew_junimo/compose_template.go`
  - 模板改为贴近 Junimo 官方结构。
  - 使用 `IMAGE_VERSION`。
  - 保留服务名 `steam-auth`、`server`、`discord-bot`。
  - 补齐 `steam-session:/data/steam-session`、`game-data:/data/game`、`saves:/config/xdg/config/StardewValley`、`./.local-container/settings:/data/settings`。
  - 补齐 `stdin_open: true`、`tty: true`、官方端口和关键环境变量。
- `backend/internal/games/stardew_junimo/config/env.go`
  - `.env` 模板改为官方关键变量集合：`IMAGE_VERSION`、Steam/VNC、端口、API、安全、性能等变量。
  - 读取 `.env` 时会去掉简单引号，便于兼容官方 `.env.example` 风格。
- `backend/internal/games/stardew_junimo/driver.go`
  - `Prepare()` 写入 `IMAGE_VERSION=1.5.0-preview.121`，不再写 `JUNIMO_IMAGE_TAG`。
  - 创建 `.local-container/settings`。
  - 不再用实例目录 `.local-container` 是否为空判断游戏文件是否存在；官方游戏文件在 `game-data` Docker named volume 中，文件系统检查会误判。
- `backend/internal/games/stardew_junimo/installer.go`
  - 写 `.env` 时使用 `IMAGE_VERSION`。
  - 账号密码安装路径改为执行 `docker compose run --rm -i steam-auth download`，让上游通过 `.env` 中的 `STEAM_USERNAME` / `STEAM_PASSWORD` 走 `EnsureLoggedInAsync(LoginConfig)`。
  - stdin pipe 只用于 Steam Guard 等 `Console.ReadLine()` 输入，不再用于 `setup` 的账号密码 `ReadPassword()` 分支。

**前端：**

- `frontend/src/App.tsx`
  - 更新账号密码认证文案，明确安装任务会使用 Steam 凭据认证并下载游戏。
  - QR 失败说明继续提示这是上游 SteamClient 未连接导致的二维码生成失败，建议改用账号密码 / Steam Guard。

**测试：**

- `backend/internal/games/stardew_junimo/config/env_test.go`
  - 覆盖官方 `.env` key 和 `IMAGE_VERSION`。
- `backend/internal/games/stardew_junimo/driver_test.go`
  - 覆盖官方 compose 关键服务、volume、`stdin_open` / `tty`。

### 影响和注意事项

- 已存在实例的 `docker-compose.yml` 和 `.env` 不会被 `Prepare()` 自动覆盖。旧实例如果已经生成过简化 compose，需要手动备份并删除/重建这些文件，或后续新增“重写 Junimo 配置”功能。
- 当前账号密码安装路径使用 `docker compose run --rm -i steam-auth download`，没有引入 PTY。不要用普通 stdin pipe 再切回 `setup` 的账号密码分支；如需完整支持 `setup` 的二维码/菜单交互，下一步应引入跨平台 PTY。
- QR 登录失败 `SteamClient instance must be connected` 属于上游 QR 连接时序问题，面板只做明确失败提示和降级，不再误判成功。

### 如何验证

```bash
cd backend && go test ./...
cd frontend && npm run build
```

端到端建议：

1. 备份并移除旧实例 `docker-compose.yml` / `.env`，重新 Prepare，确认新文件使用官方结构。
2. 安装时确认任务日志显示 `steam-auth download` 路径下的登录/下载输出；job log 不出现 Steam 用户名、Steam 密码、VNC 密码。
3. 选择二维码登录，如上游报 `SteamClient instance must be connected`，确认前端提示改用账号密码 / Steam Guard。

---

## Milestone 6 补丁：SteamClient 连接慢时自动重试 download

### 问题现象

切到 `steam-auth download` 后，用户仍可能看到：

```text
[SteamAuth:A0] Connecting...
[SteamAuth:A0] Authentication failed: The SteamClient instance must be connected.
[SteamService] Game download failed: Account 0: no auth method (no token, saved session, or password)
```

`.env` 已确认有 `STEAM_USERNAME` / `STEAM_PASSWORD`，这个错误不是缺密码，而是上游 `SteamAuthService` 调用 `Connect()` 后固定等待约 2 秒就发起认证；当前网络/Steam 连接慢时，SteamClient 尚未真正 connected。

### 改了什么

- `backend/internal/games/stardew_junimo/installer.go`
  - `runSteamAuth` 增加最多 5 次自动重试。
  - 检测到 `SteamClient instance must be connected` 时，按 6s、9s、12s、15s 等待后重新执行 `steam-auth download`。
  - 多次失败后写入 `driver_phase=steam_auth_connection_failed`，提示检查网络后重试。
  - 保留 Steam Guard stdin 通道，重试期间不记录任何凭据原文。
- `frontend/src/App.tsx`
  - 识别 `steam_auth_retrying`，显示“Steam 连接较慢，正在自动重试认证...”
  - 识别 `steam_auth_connection_failed`，显示“Steam 连接建立超时，请检查网络后重试”。

### 如何验证

```bash
cd backend && go test ./...
cd frontend && npm run build
```

本次验证结果：两者通过。

---

## Milestone 6 补丁：区分 Steam Guard 手机批准、验证码和 CM 网络失败

### 问题现象

`steam-auth download` 第三次重试连接成功后输出：

```text
Steam Guard Authentication
[1] Approve in Steam Mobile App (recommended)
[2] Enter code from Steam Mobile App or Email
Choice [1]: [SteamAuth:A0] Login failed: TryAnotherCM (Extended: Invalid)
```

旧解析看到 `Steam Guard` / `Enter code` 就进入 `steam_guard_required`，导致前端显示“输入验证码”；实际默认选项是 `[1] Approve in Steam Mobile App`，用户应打开手机 App 批准。`TryAnotherCM` 也不是账号密码错误，而是 Steam CM/网络连接失败。

### 改了什么

- `backend/internal/games/stardew_junimo/installer.go`
  - 看到 `Steam Guard Authentication`、`Approve in Steam Mobile App`、`Choice [1]` 时进入 `steam_guard_mobile_required`。
  - 只有看到明确 `enter steam guard code` / `verification code` 等输入验证码提示时才进入 `steam_guard_required`。
  - `TryAnotherCM` 与 `SteamClient instance must be connected` 都归为连接/CM 问题，走自动重试或 `steam_auth_connection_failed`。
  - 只有明确 `invalid password` / `incorrect password` / `wrong password` / `bad password` 才进入 `credentials_required`。
- `frontend/src/App.tsx`
  - `steam_guard_mobile_required` 只显示手机 App 批准等待，不显示验证码输入框。
  - `steam_auth_failed` 不再默认当作账号密码错误；只有 `credentials_required` 才提示重输凭据。

### 如何验证

```bash
cd backend && go test ./...
cd frontend && npm run build
```

本次验证结果：两者通过。

---

## Milestone 6 补丁：修复 Steam Guard 菜单解析顺序

### 问题现象

日志中 `Choice [1]: [SteamAuth:A0] Login failed: TryAnotherCM ...` 同一行同时包含手机批准提示和失败原因。旧解析先匹配 `Choice [1]`，导致状态停在 `steam_guard_mobile_required`；另外菜单行 `[2] Enter code from Steam Mobile App or Email` 被误判为验证码输入提示。

### 改了什么

- `backend/internal/games/stardew_junimo/installer.go`
  - 将失败类判断前置：`TryAnotherCM`、`SteamClient instance must be connected` 优先归类为 Steam CM/连接问题。
  - 新增 `isSteamGuardCodePrompt`，过滤菜单项 `[2] Enter code...`，只有真实验证码输入提示才进入 `steam_guard_required`。
  - `Choice [1]` / `Approve in Steam Mobile App` 仍进入 `steam_guard_mobile_required`，但不会覆盖同一行里的连接失败判断。

### 如何验证

```bash
cd backend && go test ./...
cd frontend && npm run build
```

本次验证结果：两者通过。

---

## Milestone 6 补丁：前端兜底旧失败任务停留在手机确认状态

### 问题现象

旧安装任务已经失败，但实例 `driver_phase` 仍停在 `steam_guard_mobile_required`，前端继续显示“等待手机 App 确认”。这是旧 job 已经写入的过期 instance phase，不代表后端仍在等待手机确认。

### 改了什么

- `frontend/src/App.tsx`
  - 当最近的 `stardew_install` 任务已经 `failed`，且当前没有运行中的安装任务时，如果 instance 仍停在 `steam_auth_running` / `steam_guard_mobile_required` / `steam_guard_required` / `steam_auth_retrying`，前端临时按 `steam_auth_failed` 展示。
  - 超时失败仍优先展示 `install_timeout`。
  - 这样旧 job 不会让 UI 一直卡在“等待手机确认”。

### 如何验证

```bash
cd frontend && npm run build
cd backend && go test ./...
```

本次验证结果：两者通过。

---

## Milestone 6 补丁：非凭据失败重试复用 .env 凭据

### 问题现象

`install_timeout`、`steam_auth_connection_failed`、`TryAnotherCM`/Steam CM 失败都不是账号密码错误，但前端把失败状态归到重新安装表单，导致用户重试时仍要重新输入 Steam 用户名、Steam 密码和 VNC 密码。

### 改了什么

- `frontend/src/App.tsx`
  - `needsInstall` 增加 `steam_auth_failed`，让非凭据认证失败也显示重试入口。
  - `canDirectRetry` 改为：只有 `credentials_required` 才要求重新输入凭据；`install_timeout`、`steam_auth_connection_failed`、`qr_auth_failed`、普通 `steam_auth_failed` 和 pull/generic error 都复用 `.env` 中已保存凭据重试。

### 如何验证

```bash
cd frontend && npm run build
```

本次验证结果：通过。

---

## Milestone 6 补丁：Steam Guard 菜单显示选项按钮

### 问题现象

`steam-auth download` 运行后，Steam 输出 Steam Guard Authentication 两选项菜单：

```text
[steam] ╔═══════════════════════════════════════════════════════════════╗
[steam] ║ Steam Guard Authentication ║
[steam] ╠═══════════════════════════════════════════════════════════════╣
[steam] ║ [1] Approve in Steam Mobile App (recommended) ║
[steam] ║ [2] Enter code from Steam Mobile App or Email ║
[steam] ╚═══════════════════════════════════════════════════════════════╝
[steam] Choice [1]: [SteamAuth:A0] Login failed: TryAnotherCM +60.8s
```

旧逻辑看到 `steam guard authentication` 直接设置 `steam_guard_mobile_required`，前端显示"等待手机批准"，用户看不到选项按钮。60 秒无输入后 Steam 自动选 [1] 并超时。

### 改了什么

**后端：**

- `backend/internal/games/stardew_junimo/installer.go`
  - 将 `steam guard authentication` 单独拆成新 case，phase 更新为 `steam_guard_choice_required`。
  - 将 `choice [1]` 合并到 `waiting for approval`/`open steam app` case，表示 [1] 已选中，进入手机批准等待。
  - 去除 `approve in steam mobile app` 作为手机批准触发词（该字符串是菜单项展示，不是选中确认）。

- `backend/internal/web/install_handlers.go`
  - `POST /api/instances/:id/steam-guard/input` 在 `steam_guard_choice_required` 阶段同样只允许 `1` 或 `2`。

**前端：**

- `frontend/src/App.tsx`
  - 新增 `needsGuardChoice = phase === 'steam_guard_choice_required' && isInstalling`。
  - `calcInstallProgress` 增加 `steam_guard_choice_required` → 75% active。
  - `calcStepStatuses` 的 `isAuthPhase` 增加 `steam_guard_choice_required`。
  - `staleAuthState` 增加 `steam_guard_choice_required`（旧 job 过期 phase 兜底）。
  - `InstallSection` 当 `needsGuardChoice` 时显示两个按钮："使用手机 App 批准"（提交 `1`）和"输入 Steam Guard 验证码"（提交 `2`），并提示 60 秒内选择。
  - `handleAuthMethodSelect` 针对 `steam_guard_choice_required` 阶段显示不同的确认消息。

### 如何验证

```bash
cd backend && go test ./...
cd frontend && npm run build
```

本次验证结果：两者通过。

端到端建议：

1. 启动安装，等待 job log 出现 `Steam Guard Authentication`。
2. 实例 `driverPhase` 应变为 `steam_guard_choice_required`。
3. 前端安装区显示"选择 Steam Guard 验证方式"，有"使用手机 App 批准"和"输入 Steam Guard 验证码"两个按钮，提示 60 秒内操作。
4. 点击"使用手机 App 批准" → 后端向 steam-auth stdin 写入 `1`，phase 变为 `steam_guard_mobile_required`，前端显示"请在手机 App 批准登录"。
5. 点击"输入 Steam Guard 验证码" → 写入 `2`，之后等待 Steam 发验证码，phase 进入 `steam_guard_required`，前端显示验证码输入框。

### 下一步注意事项

- `steam_guard_choice_required` 选项窗口约 60 秒，超时 Steam 自动选 [1]。已有的 TryAnotherCM 重试机制会再次触发此菜单。
- 若后续 steam-auth 版本改变了菜单文本，需同步更新 `installer.go` 的检测关键词。

---

## Milestone 6 补丁：修复选 2 无反应 + AsyncJobFailedException 可重试

### 问题现象

1. 用户在 `steam_guard_choice_required` 阶段选择"输入 Steam Guard 验证码"后，前端没有出现验证码输入框，容器也没有任何反应。

2. 用户在选择选项 1（手机 App 批准）并在手机上同意后，仍然安装失败，日志出现：
   ```text
   [SteamAuth:A0] Authentication failed: Exception of type 'SteamKit2.AsyncJobFailedException' was thrown.
   ```

### 根因分析

**问题1：** 容器收到 "2" 后输出代码提示 `Enter Steam Guard code: `，但该行**末尾没有换行符**。`bufio.Scanner` 按行读取会永远阻塞在该行，不会触发 `lineHandler`，phase 永远不更新，前端看不到验证码输入框。

**问题2：** `SteamKit2.AsyncJobFailedException` 是 Steam CM 层面的网络/连接异常（和 `TryAnotherCM` 性质相同），属于可重试错误。但旧代码检测到 `"authentication failed"` 后标记为 `authFailed=true`，不会重试，直接设为 `steam_auth_failed`。

### 改了什么

**后端：**

- `backend/internal/games/stardew_junimo/installer.go`
  - 在 `tryanothercm` / `steamclient instance must be connected` case 追加 `asyncjobfailedexception`，使其归类为 `connectionFailed`，自动触发最多 5 次重试。

- `backend/internal/web/install_handlers.go`
  - 用户在 `steam_guard_choice_required` 阶段提交 `"2"` 后，**主动**调用 `s.store.UpdateInstanceState` 将 phase 设为 `steam_guard_required`，绕过无换行提示行的检测盲区，确保前端立即显示验证码输入框。

### 如何验证

```bash
cd backend && go test ./...
cd frontend && npm run build
```

本次验证结果：两者通过。

端到端验证：

1. 选 "输入 Steam Guard 验证码" → 前端立即出现验证码输入框（不用等容器输出）。
2. 在验证码输入框输入收到的 Steam Guard 代码，提交后容器应继续认证流程。
3. 手机 App 批准后如遇 `AsyncJobFailedException` → 自动重试，日志显示"SteamClient 尚未连接，等待 Xs 后重试"（用 `steam_auth_retrying` phase）。
4. 重试超过 5 次才设为 `steam_auth_connection_failed`，提示检查网络。

### 下一步注意事项

- 选 "2" 后提交验证码时，后端在 `steam_guard_required` phase 不限制输入格式，任意字符串都会被转发给容器 stdin。
- `AsyncJobFailedException` 加入重试后，手机 App 批准期间发生连接断开仍会重试。重试时会重新启动 `steam-auth download`，手机上可能需要再次批准。

---

## Milestone 6 补丁：阅读上游 Junimo 实现 + 修复下载阶段可见性和下载失败检测

### 背景

阅读了上游 JunimoServer 完整源代码（`ConsoleAuthenticator.cs`、`Logger.cs`、`Program.cs`、`SteamAuthService.cs`、`steam-auth.md`），确认以下关键事实：

1. **我们使用 `download` 命令而不是 `setup` 是正确的。**
   - `setup` 命令若账号在环境变量中则调用 `LoginInteractiveAsync()`，其中 `ReadPassword()` 使用 `Console.ReadKey(intercept:true)`，在无 PTY（stdin 重定向）环境下会抛 `InvalidOperationException: Cannot read keys...`，导致容器崩溃。
   - `download` 命令调用 `EnsureLoggedInAsync(LoginConfig(user, pass, token))`，通过 `.env` 中的 `STEAM_USERNAME/STEAM_PASSWORD` 绕过交互分支，只有 `ConsoleAuthenticator`（Steam Guard）部分使用 `Console.ReadLine()`，不涉及 `Console.ReadKey`，与我们的 stdin pipe 兼容。

2. **`Console.Write`（无换行）在非 PTY 环境下被缓冲，`bufio.Scanner` 无法检测。**
   - `Logger.Log` 最终调用 `Console.WriteLine`（含换行），会出现在我们的 scanner 里。
   - `Console.Write("Choice [1]: ")` / `"Enter Steam Guard code: "` 没有换行，会被 .NET 缓冲，scanner 永远看不到——这正是为何选 2 后无反应，以及我们用主动 phase 更新来绕过。

3. **`AsyncJobFailedException` 是 `PollingWaitForResultAsync` 在手机批准等待期间抛出的，不是凭据错误**，已在上一补丁加入重试。

4. **成功日志路径：** `Logged in as USERNAME` → 开始下载 → `Downloading app 413150...` → （下载过程） → `Download complete!` → `App installed to: /data/game` → 容器退出 0。

### 发现的问题

**问题1（UX）：** Steam 认证成功后到容器退出之间（最长约 30 分钟），前端没有 phase 更新，用户看到的仍是旧的 Steam Guard 阶段（`steam_guard_mobile_required` / `steam_guard_choice_required`）。

**问题2（正确性）：** 若 Steam 认证成功但游戏文件下载失败（CDN 错误、磁盘空间不足），Junimo 记录 `[SteamService] Game download failed: <reason>` 后以退出码 1 退出。旧代码可能因 "logged in" 已设 `authSucceeded=true`，而 "game download failed" 没有检测，导致下载失败时误报"安装成功"。

### 改了什么

**后端 `installer.go`：**

- 新增 `downloadFailed bool` 标志。
- 新增 case：检测 `"downloading app"` → 更新 phase 为 `game_downloading`，message 提示"正在下载游戏文件，请耐心等待"。
- 新增 case：检测 `"game download failed"` / `"download failed:"` → 设 `downloadFailed = true`。
- 结果处理新增 `downloadFailed` 分支（在 `connectionFailed` 之后、`authFailed` 之前）：
  - 写 `steam_auth_failed` 状态，`driver_phase = download_failed`，message 提示"请检查网络和磁盘空间后重试安装"。
  - 直接返回失败，不走 `authSucceeded` 路径，防止误报成功。

**前端 `App.tsx`：**

- `calcInstallProgress` 新增：
  - `download_failed` → 88% error "游戏文件下载失败，请检查网络/磁盘后重试"（在 `authFailed` 检查前）。
  - `game_downloading` → 88% active "正在下载游戏文件（约10-30分钟）..."。
- `isAuthPhase` 列表增加 `game_downloading`（影响步骤状态显示）。
- `staleAuthState` 增加 `game_downloading`（job 失败后清除停留在下载阶段的旧状态）。

### 如何验证

```bash
cd backend && go test ./...
cd frontend && npm run build
```

本次验证结果：两者通过。

端到端建议：

1. 安装成功走到认证后，job log 出现 `[steam] [SteamAuth:A0] Downloading app 413150...` 时，前端应更新进度为 88%"正在下载游戏文件（约10-30分钟）..."，不再停留在 Steam Guard 阶段。
2. 模拟下载失败（可临时修改 docker-compose.yml 让 game-data volume 指向无写权限目录），确认前端显示"游戏文件下载失败"而不是"安装成功"。

### 下一步注意事项

- `game_downloading` 阶段的持续时间取决于网络和磁盘速度，可能 10-30 分钟。此期间后端无进度百分比，前端固定显示 88%。若需要精细进度，需要解析 Junimo 的文件数/字节数日志（`Downloading 1234 files (5.2 GB)`）。
- 选 "2"（Steam Guard 验证码）后到验证成功，下载才开始。若验证码超时（Steam Guard TOTP 每 30 秒轮换），用户需要重新输入，无需重启安装。

---

## Milestone 6 补丁：清理前端 `auth_method_required` 死代码

### 背景

上一补丁确认我们使用 `docker compose run --rm -i steam-auth download` 而不是 `setup`。`auth_method_required` phase 只有 `setup` 命令中的 `LoginInteractiveAsync()` 才会触发，`download` 命令永远不会触发该分支。因此前端中的 `auth_method_required` 处理逻辑及其关联状态都是永远不会执行的死代码。

### 改了什么

**前端 `frontend/src/App.tsx`：**

- 删除 `case 'auth_method_required'` from `calcInstallProgress`（显示进度逻辑）。
- 删除 `'auth_method_required'` from `isAuthPhase` 数组（步骤状态判断）。
- 删除 `const [qrRequested, setQrRequested] = useState(false)` 状态。
- 删除监听 `auth_method_required`/`steam_qr_required` 离开时清除 `qrRequested` 的 `useEffect`。
- 简化 `handleAuthMethodSelect`：删除 `if/else` 分支和所有 `setQrRequested` 调用，直接调用 `submitSteamGuardInput`。
- `needsQrCode` 简化为仅 `phase === 'steam_qr_required'`（删除 `|| qrRequested`）。
- 删除 `needsAuthMethod` 变量。
- 从 `<InstallSection>` JSX 调用中删除 `needsAuthMethod` prop。
- 从 `InstallSection` 函数参数和 TypeScript 类型定义中删除 `needsAuthMethod`。
- 修改外层条件：`(needsAuthMethod || needsGuardChoice || needsGuard || needsQrCode)` → `(needsGuardChoice || needsGuard || needsQrCode)`。
- 删除整个 `{needsAuthMethod ? (...) : null}` UI 块（"选择 Steam 认证方式"/"使用账号密码"/"使用手机扫码"按钮）。

**保留不变：**

- `steam_qr_required` 处理（后端仍可能通过 QR 流程发出此 phase）。
- `needsQrCode = phase === 'steam_qr_required'`。
- `extractRecentSteamQrText` helper。
- `qr_auth_failed` 错误处理。

### 如何验证

```bash
cd frontend && npm run build
```

本次验证结果：通过，17 个模块全部编译，无 TypeScript 错误。

### 下一步注意事项

- `steam_qr_required` 和 `qr_auth_failed` 仍保留，但当前 `download` 路径也未必会触发 QR 流程（需要用户主动在 Steam Guard 菜单选 "2"，再由后端识别 `steam_qr_required` phase）。若日后确认后端也不会发出 `steam_qr_required`，可同步删除前端的 QR 相关代码。
- 当前 Steam 认证失败根本原因是 Docker 容器内网络无法在 2 秒内连接 Steam CM（上游硬编码 `ConnectionEstablishmentDelay`）。这是网络/代理问题，不是代码 bug，需要用户在 Docker Desktop 设置代理或确保容器可访问 Steam CM 服务器。诊断命令：`docker run --rm alpine sh -c "wget -qO- --timeout=5 https://api.steampowered.com/ISteamWebAPIUtil/GetServerInfo/v1/ 2>&1 | head -3"`
