# Stardew Anxi Panel

[English](README.en.md)

`stardew-server-anxi-panel` 当前是一个围绕 [JunimoServer](https://stardew-valley-dedicated-server.github.io/server/) 构建的 Stardew Valley 专用服务器 Web 管理面板。

目标是让用户只需要运行一个 Anxi Panel Docker 镜像，打开浏览器，初始化管理员账号，安装 Stardew 服务器，完成 Steam 认证，选择存档，启动服务器，查看邀请码，监控状态，管理存档和 Mod，发送服务器命令，并管理面板用户。

长期目标是演进成一个多游戏开服总面板：总面板展示所有游戏服务器实例状态，用户选择某个游戏后进入该游戏自己的专属面板。Stardew + JunimoServer 是第一个游戏实现，后续可接入 Minecraft、Don't Starve Together、Terraria、Palworld 等。

首个可上线版本默认使用 **Single Game Mode**：用户登录后直接进入 Stardew 面板，不显示总面板和游戏列表。内部仍按 `instances + driver_id + GameDriver` 设计，等开发第二个游戏面板时再开启 **Multi Game Mode**。

> 当前状态：**Milestone 14: Release Candidate 已完成**。Milestone 0-14 已完成。后端已包含配置加载、SQLite、认证与权限、Docker / Compose allowlist、jobs/job_logs/SSE、instances、GameDriver registry、Stardew Junimo 的工作目录准备、Steam 认证、游戏安装、服务器生命周期、存档管理、Mod 管理、控制台命令、多阶段 Docker 镜像构建、操作审计、日志脱敏、备份恢复、健康检查诊断、版本信息、支持包导出和冒烟测试脚本。

## GitHub 描述

```text
基于 JunimoServer 的星露谷物语专用服务器 Web 管理面板，使用 Go、React、SQLite 和 Docker Compose 构建。
```

## 功能目标

预期用户流程：

1. 运行 Anxi Panel Docker 镜像。
2. 后端自动准备 JunimoServer 工作目录和配置文件。
3. 在浏览器中打开面板。
4. 创建第一个管理员账号。
5. 点击 **Install Game**。
6. 输入 Steam 用户名、Steam 密码和 VNC 密码。
7. 后端写入 `.env`，直接拉取 JunimoServer 相关容器镜像，并运行 Steam Auth。
8. 前端显示 Steam Guard 提示，后端完成 PTY 交互。
9. 安装完成后点击 **Start Server**。
10. 如果没有已有存档，选择“自定义新建存档”或“从本机上传存档”：
    - 自定义新建存档由面板收集农场名、玩家名、地图类型和初始设置，并生成可被 Stardew/Junimo 读取的真实初始存档；上游 Junimo 不支持完整自定义创建。
    - 上传存档先解析并展示游戏时间、地图、已有玩家名称等预览，确认后才上传到服务器。
11. 后端运行 `docker compose up -d`。
12. 后端通过 `attach-cli` 获取邀请码，并显示在面板中。
13. 通过 Web UI 管理服务器状态、命令、聊天公告、存档、Mod 和面板用户。

## 架构

计划技术栈：

- 后端：Go
- 前端：React + TypeScript + Vite
- 数据库：SQLite
- 运行时控制：Docker Socket + Docker Compose V2
- 游戏集成：GameDriver 风格抽象
- 首个驱动：通过 JunimoServer 支持 Stardew Valley

长期产品分层：

```text
Global Panel
  -> Game Instance List
  -> Game-specific Frontend Module
  -> GameDriver
  -> Game Server Containers
```

第一版高层流程：

```text
React Frontend
  -> Go API
  -> jobs/state machine
  -> games/stardew_junimo driver
  -> Docker Compose / mounted files / attach-cli / Junimo HTTP status
  -> JunimoServer containers
```

本项目不会替代 JunimoServer，而是在 JunimoServer 官方 Docker 工作流外层提供一个更安全、可见、基于浏览器的管理体验。

当前显示模式：

```text
PANEL_MODE=single
/ -> /instances/stardew
```

未来多游戏模式：

```text
PANEL_MODE=multi
/ -> 总面板游戏实例列表
/instances/stardew -> Stardew 面板
/instances/minecraft -> Minecraft 面板
```

后续接入其他游戏时，不是在 Stardew 页面里继续加分支，而是新增对应 game module 和 driver：

```text
frontend/src/games/stardew        + backend/internal/games/stardew_junimo
frontend/src/games/minecraft      + backend/internal/games/minecraft
frontend/src/games/dst            + backend/internal/games/dont_starve_together
frontend/src/games/terraria       + backend/internal/games/terraria
frontend/src/games/palworld       + backend/internal/games/palworld
```

## 仓库结构

```text
stardew-server-anxi-panel
├─ backend              Go API 服务
├─ frontend             React + TypeScript 前端
├─ deploy               部署示例（docker-compose.yml）
├─ docs
│  ├─ architecture.md   架构决策
│  ├─ deployment.md     部署指南
│  ├─ handoff-roadmap.md
│  └─ prototypes        产品原型和说明
├─ Dockerfile           多阶段构建
├─ .dockerignore
├─ LICENSE
├─ README.en.md
└─ README.md
```

## 后端开发

后端位于 `backend/`。

```bash
cd backend
go test ./...
go run ./cmd/panel
```

默认监听地址：

```text
:8090
```

可通过环境变量覆盖：

```bash
PANEL_ADDR=:8091 go run ./cmd/panel
```

本地开发建议显式指定 data 目录，避免写入系统 `/data`：

```bash
PANEL_DATA_DIR=./data PANEL_DB_PATH=./data/panel.db go run ./cmd/panel
```

后端配置：

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `PANEL_ADDR` | `:8090` | HTTP 监听地址。 |
| `PANEL_DATA_DIR` | `/data` | 面板数据目录，启动时自动创建。默认 instance 的 `data_dir` 为 `$PANEL_DATA_DIR/instances/stardew`。 |
| `PANEL_DB_PATH` | `$PANEL_DATA_DIR/panel.db` | SQLite 数据库路径，启动时自动创建。 |
| `PANEL_SECRET` | empty | Session token hash secret。本地开发可为空；生产环境必须设置为足够随机的长 secret。 |
| `PANEL_VERSION` | `dev` | `/health` 返回的版本字符串。 |
| `PANEL_MODE` | `single` | 产品显示模式。`single` 直达默认游戏面板；`multi` 显示总面板游戏列表。 |
| `DEFAULT_INSTANCE_ID` | `stardew` | Single Game Mode 默认进入的实例。 |
| `DEFAULT_DRIVER_ID` | `stardew_junimo` | 首个默认实例使用的 driver。 |

健康检查：

```text
GET /health
```

示例响应：

```json
{
  "status": "ok",
  "service": "stardew-anxi-panel",
  "version": "dev",
  "database": {
    "status": "ok"
  }
}
```

## Auth API

所有错误响应使用统一 JSON 结构：

```json
{
  "error": {
    "code": "invalid_credentials",
    "message": "用户名或密码错误"
  }
}
```

已实现接口：

```text
GET    /api/setup/status
POST   /api/setup/admin
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/auth/me
GET    /api/users
POST   /api/users
PATCH  /api/users/:id
DELETE /api/users/:id
```

说明：

- 无 active admin 时，只允许访问 `GET /health`、`GET /api/setup/status`、`POST /api/setup/admin`。
- `POST /api/setup/admin` 会创建第一个 admin，写入 audit log，并自动建立 HttpOnly Cookie session。
- 密码使用 Argon2id 哈希保存，不保存明文；当前最小长度为 6 位。
- Session token 只通过 HttpOnly Cookie 返回给浏览器；数据库只保存 token hash。
- `/api/users` 系列接口仅 admin 可访问。
- 普通 user 可以登录、登出和读取 `/api/auth/me`，不能管理其他用户。
- `DELETE /api/users/:id` 默认是软删除/禁用用户；`DELETE /api/users/:id?hard=true` 会真正删除用户。
- 最后一个 active admin 不能被禁用、删除或降级，当前登录 admin 不能禁用或删除自己。

## Docker / Compose API

Docker 细节 API 仅管理员可访问。普通用户直接访问会返回 403。

产品主路径已经切到 instance-based API：

```text
GET /api/instances/:instance_id/status
GET /api/instances/:instance_id/docker/ps
```

兼容/调试接口仍保留：

```text
GET /api/docker/status
GET /api/docker/ps
GET /api/docker/logs?service=&tail=100
```

说明：

- 后端通过 `exec.CommandContext` 和参数数组调用 Docker CLI，不经过 shell。
- 前端不能传入任意命令、任意参数或任意工作目录。
- Compose 工作目录来自 `instances.data_dir`，默认是 `$PANEL_DATA_DIR/instances/stardew`。
- 本阶段只检查 Docker / Compose 状态，不会拉取 Junimo 镜像，不会执行 Steam Auth，不会启动容器。
- `GET /api/docker/status` 返回 Docker 是否可用、Docker version、Compose version、默认 instance 的 Compose 目录状态。
- `GET /api/instances/stardew/docker/ps` 在默认 Stardew instance 的 `data_dir` 执行 `docker compose ps --format json`；没有 compose 文件时返回明确错误。
- `GET /api/docker/ps` 仍可用，但只作为 admin-only 兼容/调试入口。
- `GET /api/docker/logs` 返回非流式 logs 快照，`tail` 默认 100，最大 1000。
- Docker 命令结果包含 stdout、stderr、exit code、duration 和 timeout 状态。
- 命令输出会脱敏 password、token、secret、`STEAM_PASSWORD`、`VNC_PASSWORD` 等敏感字段。

## Instances / Jobs / State API

Instances、Jobs 和状态接口需要登录；测试任务创建仅 admin 可用。

已实现接口：

```text
GET  /api/instances
GET  /api/instances/:instance_id
GET  /api/instances/:instance_id/state
GET  /api/instances/:instance_id/status
GET  /api/instances/:instance_id/docker/ps
GET  /api/jobs
GET  /api/jobs/:id
GET  /api/jobs/:id/logs?after=0&limit=200
GET  /api/jobs/:id/stream
POST /api/jobs/:id/cancel
POST /api/jobs/test
POST /api/jobs/test-fail
```

说明：

- 后端启动后会自动确保默认 Stardew instance 存在。
- 默认 instance 配置为 `id=stardew`、`driver_id=stardew_junimo`、`name=Stardew Valley`、`data_dir=$PANEL_DATA_DIR/instances/stardew`。
- `instances`、`jobs`、`job_logs` 和兼容用 `instance_state` 由 SQLite 持久化，后端重启后仍可查询历史任务和当前实例状态。
- `GET /api/instances/stardew/status` 通过 `stardew_junimo` driver 返回基础状态；当前 driver 只实现骨架和 Compose PS 状态摘要。
- `GET /api/instances/stardew/state` 返回通用 `state` 和 driver-specific `driver_phase`。
- job status 枚举为 `queued`、`running`、`succeeded`、`failed`、`canceled`。
- job log level 枚举为 `info`、`warn`、`error`、`debug`，每个 job 内使用递增 `sequence`。
- `GET /api/jobs/:id/stream` 使用 SSE 推送历史日志和新增日志；job 完成后发送 `finished` 事件并结束。
- `POST /api/jobs/test` 会创建约 5 秒的模拟成功任务，每秒写入一行日志。
- `POST /api/jobs/test-fail` 会创建模拟失败任务，最终状态为 `failed` 并保存 `error_message`。
- `POST /api/jobs/:id/cancel` 目前返回 501 `not_implemented`，取消真实长期任务将在后续里程碑接入。
- 普通 user 不能创建测试任务，只能查看自己有权限的任务。
- 本阶段没有任何前端任意命令执行入口，也不会执行 Junimo 安装、Steam Auth 或 Docker lifecycle job。

## JunimoServer / Steam 认证注意事项

本面板遵循 JunimoServer 官方 Docker Compose 工作流。新实例生成的 `docker-compose.yml` 应尽量贴近官方结构：服务名保留 `steam-auth`、`server`、`discord-bot`，Junimo server 镜像版本使用 `IMAGE_VERSION`，Steam session 和游戏文件分别保存在官方 Docker named volumes `steam-session` 和 `game-data` 中，服务器设置绑定到 `./.local-container/settings:/data/settings`。

`steam-auth` sidecar 默认使用面板维护的 CN 修补镜像：

```env
STEAM_SERVICE_IMAGE=anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2
STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS=60
STEAM_CLIENT_CONNECT_RETRIES=5
STEAM_AUTH_SESSION_RETRIES=3
STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS=5
```

这个镜像基于 JunimoServer `tools/steam-service`，只修补 SteamClient 连接等待和认证会话重试：QR / 账号密码 / refresh token 登录会先等 `ConnectedCallback`，遇到 `TryAnotherCM`、`AsyncJobFailedException`、认证阶段断线或超时时会重连并重试。

本地联合调试还没推 Docker Hub 时，可以先在 fork 仓库构建本地镜像：

```powershell
cd E:\junimo-server-steam-service-cn
docker build --progress=plain -f tools\steam-service\Dockerfile -t junimo-steam-service-cn:auth-retry-test .
```

然后把面板实例 `.env` 里的 `STEAM_SERVICE_IMAGE` 临时改成本地 tag：

```env
STEAM_SERVICE_IMAGE=junimo-steam-service-cn:auth-retry-test
```

Steam Auth 交互有几个容易误解的点：

- 面板安装时优先运行 `docker compose run --rm -i steam-auth download`，让 Junimo 使用 `.env` 中的 `STEAM_USERNAME` / `STEAM_PASSWORD` 走非交互登录并下载游戏文件。这样可以避开上游 `setup` 的账号密码分支：该分支使用 `Console.ReadKey()` 逐字符读取密码，在后台任务的 stdin 重定向环境中会报 `Cannot read keys when either application does not have a console or when console input has been redirected`。敏感内容不会写入任务日志、后端日志或响应 JSON。
- 如果 Steam 需要二次验证，前端会继续显示 Steam Guard 验证码输入或手机 App 确认提示。
- QR 登录、账号密码二次验证和 refresh token 登录都依赖 `steam-auth` 连接 Steam CM。CN 修补镜像会等待连接并对 `TryAnotherCM` 等瞬时错误重试；如果仍然失败，先确认 `.env` 中的 `STEAM_SERVICE_IMAGE` 指向修补镜像，并查看任务日志里的重试信息。
- 已经生成过旧版本 `docker-compose.yml` / `.env` 的本地实例不会被 `Prepare()` 自动覆盖。需要迁移到官方结构时，请先备份实例目录，再删除或重建这两个配置文件。

## 前端开发

前端位于 `frontend/`。

```bash
cd frontend
npm install
npm run dev
```

常用脚本：

```bash
npm run build
npm run preview
```

Vite 开发服务器已配置 `/api` 和 `/health` 代理到 `http://localhost:8090`。本地联调时先启动后端，再启动前端，然后打开 Vite 输出的地址，通常是：

```text
http://localhost:5173
```

当前前端已实现：

- 无管理员时展示管理员初始化注册页。
- 有管理员但未登录时展示登录页。
- 登录后展示基础主界面、当前用户、角色和登出按钮。
- 登录后展示 Stardew 实例状态和任务中心。
- admin 可启动测试任务、启动失败测试任务，并查看实时任务日志。
- admin 可看到最小用户管理区域和 Docker 状态区域。
- 普通 user 不显示用户管理、Docker 控制或测试任务按钮。

## Docker 部署

本项目支持构建为独立 Docker 镜像，用户只需 Docker Engine + Compose V2 即可运行面板。

### 构建镜像

```bash
docker build -t stardew-server-anxi-panel:local .
```

### 运行容器

```bash
docker run -d \
  --name anxi-panel \
  -p 8090:8090 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v anxi-panel-data:/data \
  stardew-server-anxi-panel:local
```

Windows Docker Desktop 用户命令相同（socket 通过 WSL2 转发）。

### 使用 Docker Compose

```bash
cd deploy
docker compose up -d
```

### 首次访问

打开 `http://localhost:8090`，进入管理员初始化注册页。

### 数据持久化

所有面板数据存储在 `/data`，使用 named volume 挂载可在容器重建后保留。

### 安全说明

挂载 Docker Socket 等同于给面板容器宿主机 Docker 控制权。建议仅在受信任的内网环境运行，不要将端口暴露到公网。详见 [部署指南](docs/deployment.md)。

## 本机测试流程

1. 启动后端：

```bash
cd backend
PANEL_DATA_DIR=./data PANEL_DB_PATH=./data/panel.db go run ./cmd/panel
```

2. 启动前端：

```bash
cd frontend
npm run dev
```

3. 打开 Vite 显示地址，通常是：

```text
http://localhost:5173
```

4. 首次打开应进入管理员初始化页。
5. 输入管理员用户名、密码、确认密码，提交后会自动登录。
6. 登录后主界面会显示当前用户和角色，并展示默认 Stardew instance 当前状态、instance 名称和 driver id。
7. 管理员可以点击“启动测试任务”。
8. 在任务中心点击新任务，日志窗口会通过 SSE 每秒追加日志，任务完成后状态变为 `succeeded`。
9. 管理员可以点击“启动失败测试任务”，任务最终变为 `failed`，详情区域显示错误原因。
10. 管理员可以点击“检查 Docker”，查看 Docker 和 Compose 状态。
11. 管理员可以点击“查看 Compose PS”。前端会请求 `/api/instances/stardew/docker/ps`，后端使用默认 instance 的 `data_dir`。如果 `$PANEL_DATA_DIR/instances/stardew` 没有 compose 文件，会看到“Compose 工作目录尚未准备”的清晰提示。
12. 也可以登录后直接请求 `GET /api/instances` 和 `GET /api/instances/stardew`，确认能看到默认 Stardew instance。
13. 管理员可以创建普通用户；普通用户登录后不能看到用户管理区域、Docker 控制区域或测试任务按钮。
14. 可用普通用户 Cookie 或浏览器会话直接请求 `/api/jobs/test`、`/api/docker/status` 或 `/api/instances/stardew/docker/ps`，应返回 403。

## 当前里程碑

Milestone 0 已包含：

- Go 后端骨架
- React + TypeScript + Vite 前端骨架
- 初始目录结构
- 基础 `/health`
- 初始文档

Milestone 1 已包含：

- Go 后端骨架
- 基于环境变量的后端配置
- SQLite 数据库创建和连接
- 最小嵌入式迁移运行器
- 带版本和数据库状态的增强 `/health` 端点
- 基础结构化日志
- 统一 JSON 错误响应
- React + TypeScript + Vite 前端骨架
- 初始文档

Milestone 2 已包含：

- `users`、`sessions`、`audit_logs`、`panel_settings` 数据表迁移
- 管理员初始化状态和初始化接口
- Argon2id 密码哈希
- HttpOnly Cookie session
- 登录、登出、当前用户接口
- admin/user 角色
- admin-only 用户管理接口
- 关键操作 audit log
- 初始化页、登录页和基础主界面

Milestone 3 已包含：

- 通用 Docker / Compose CLI 控制层
- 结构化命令结果：stdout、stderr、exit code、duration、timeout 状态
- Docker 命令超时控制和输出大小限制
- 敏感输出脱敏
- admin-only Docker 状态 API
- 登录后 Docker 状态区域

Milestone 4 已包含：

- `jobs`、`job_logs`、`instance_state` 数据表迁移
- 通用 Job Manager：创建、异步执行、日志追加、成功/失败标记、panic 捕获和启动恢复
- SSE 任务日志流 `GET /api/jobs/:id/stream`
- admin-only 测试任务 `POST /api/jobs/test` 和 `POST /api/jobs/test-fail`
- Stardew 单实例状态接口兼容入口 `GET /api/instances/stardew/state`
- 登录后任务中心、任务详情和实时日志窗口
- 普通用户不能创建测试任务

Milestone 5 已包含：

- 新增 `instances` 表和 storage 模型
- 新增 `PANEL_MODE`、`DEFAULT_INSTANCE_ID`、`DEFAULT_DRIVER_ID` 配置
- 后端启动自动确保默认 Stardew instance 存在
- 新增完整 `GameDriver` 接口和 driver registry
- 新增 `stardew_junimo` driver 骨架
- 新增 instance-based API：`GET /api/instances`、`GET /api/instances/stardew`、`GET /api/instances/stardew/state`、`GET /api/instances/stardew/status`、`GET /api/instances/stardew/docker/ps`
- Compose PS 使用 `instance.data_dir`，不再在产品主路径硬编码 Stardew 工作目录
- 前端保持 Single Game Mode，不显示总面板；内部切到默认 instance API

仍未实现：

- Junimo 工作目录准备
- Steam Auth 交互
- 服务器启动、停止、重启业务流程
- 邀请码获取
- 存档管理
- Mod 管理
- 控制台命令

## 验证命令

后端：

```bash
cd backend
go test ./...
```

前端：

```bash
cd frontend
npm run build
```

冒烟测试：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-test.ps1
```

## 构建带版本号镜像

```powershell
# 获取当前 commit hash
$commit = git rev-parse --short HEAD
$date = (Get-Date -AsUTC -Format 'yyyy-MM-ddTHH:mm:ssZ')

# 构建
docker build -t stardew-server-anxi-panel:1.0.0 `
  --build-arg VERSION=1.0.0 `
  --build-arg COMMIT=$commit `
  --build-arg BUILD_DATE=$date .
```

## 文档

继续开发前建议阅读：

- [Architecture](docs/architecture.md)
- [Handoff Roadmap](docs/handoff-roadmap.md)
- [Prototype Notes](docs/prototypes/stardew-anxi-panel-prototype-notes.md)
- [Product Prototype HTML](docs/prototypes/stardew-anxi-panel-product-prototype.html)

## 设计方向

计划 UI 采用 Stardew 风格的像素农场视觉：木质边框、羊皮纸面板、粗描边、库存式导航，以及高密度服务器管理信息。

原型位于：

```text
docs/prototypes/
```

## 重要边界

所有 Stardew/Junimo 相关逻辑都应位于 `games/stardew_junimo` driver 后面。

不要把存档、Mod 或控制台行为放进顶层通用模块。顶层后端只应提供通用基础设施：认证、Docker 命令封装、任务、存储、Web API 和游戏驱动注册表。

前端也要保持同样边界：总面板负责实例列表、登录、用户、任务中心和全局状态；Stardew 的 Steam Guard、邀请码、农场设置等交互放进 Stardew game module。后续 Minecraft 的 RCON、白名单、OP、世界管理等应放进 Minecraft game module。

Milestone 0-4 的实现不需要返工；其中临时写死的 Stardew 单实例路径应在 Milestone 5 通过 `instances + driver_id + GameDriver registry` 收口。Milestone 8 不要强制显示总面板，而是实现 Single Game Mode：登录后直达 Stardew game module；等第二个游戏面板出现后再切换到 Multi Game Mode。

前端不能提交任意 shell 命令；Docker / Compose 操作必须通过后端 allowlist 的固定方法执行。

## 许可与第三方声明

本项目以 MIT License 发布，详见 [LICENSE](LICENSE)。

本项目会直接拉取并运行 JunimoServer 容器镜像来提供 Stardew Valley 专用服务器能力。JunimoServer 是独立的第三方项目，其上游仓库为 [stardew-valley-dedicated-server/server](https://github.com/stardew-valley-dedicated-server/server)，上游许可证为 [MIT License](https://github.com/stardew-valley-dedicated-server/server/blob/master/LICENSE)。JunimoServer 容器镜像、镜像内组件及其依赖仍由上游项目及对应第三方许可约束。本仓库不声称拥有 JunimoServer、Stardew Valley、Steam 或相关商标、游戏内容、素材和服务的所有权。

使用者需要自行确认自己拥有运行 Stardew Valley 服务器所需的合法授权，并遵守 JunimoServer、Stardew Valley、Steam 以及相关第三方组件的许可、服务条款和使用规则。
