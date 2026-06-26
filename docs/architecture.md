# stardew-server-anxi-panel Architecture

本文档记录 `stardew-server-anxi-panel` 的初始架构规划，供后续 Codex、Claude 或人工维护者快速理解项目方向。

## Project Goal

`stardew-server-anxi-panel` 的近期目标是基于 JunimoServer 构建一个 Stardew Valley 专用服务器 Web 管理面板，让用户通过浏览器完成 Stardew Valley 专用服务器的安装、Steam 认证、启动、状态查看、存档管理、Mod 管理和面板用户管理。

长期目标不是把所有游戏塞进同一个 Stardew 页面，而是演进成一个通用的游戏开服总面板：总面板展示所有游戏服务器实例的状态，用户选择某个游戏实例后进入该游戏自己的专属管理面板。每个游戏由独立的后端 `GameDriver` 和前端 game module 接入对应成熟的开源服务端容器。

首个可上线版本应使用 **Single Game Mode**：用户登录后直接进入 Stardew 面板，不显示总面板和游戏列表。代码内部仍按 `instances + driver_id + GameDriver` 设计，等第二个游戏面板开发时再开启 **Multi Game Mode** 并显示总面板。

## Selected Stack

本项目选择：

- Backend: Go
- Frontend: React + TypeScript + Vite
- Database: SQLite
- Deployment: single Panel Docker image + Docker Socket
- Architecture: Panel core + GameDriver plugin-style abstraction

选择 Go 的主要原因：

- 项目核心是本地运维控制面板，需要稳定控制 Docker、Compose、文件系统、长期任务和日志流。
- Go 适合构建长期运行的单体服务，部署简单，运行时依赖少。
- Go 的并发模型适合处理安装任务、日志流、WebSocket/SSE、容器状态轮询。
- 后续做多游戏、多实例管理时，Go 后端更容易保持清晰边界。

选择 React + TypeScript 的主要原因：

- 安装向导、控制台、状态页、存档页、Mod 页都需要较强交互。
- TypeScript 便于维护前后端 API 类型边界。
- xterm.js、TanStack Query、Zustand 等生态适合面板类产品。

选择 SQLite 的主要原因：

- MVP 阶段不需要外部数据库。
- 单机面板更容易部署、备份和迁移。
- 后续如果要支持集中式多节点平台，可以再抽象存储层并迁移到 PostgreSQL。

## Deployment Model

推荐使用 Docker-outside-of-Docker，而不是完整 DinD。

Panel 容器通过挂载宿主机 Docker Socket 控制 JunimoServer 容器：

```bash
docker run -d \
  --name anxi-panel \
  -p 8090:8090 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v anxi-panel-data:/data \
  ghcr.io/yourname/stardew-server-anxi-panel:latest
```

Panel 镜像内应包含：

- Go 后端服务
- React 构建后的静态文件
- docker CLI
- docker compose plugin
- 必要的系统工具

后端负责在 `/data` 下创建并管理游戏实例目录。

## Product Model: Single Now, Multi Later

本项目的长期结构分为四层：

```text
Global Panel
  -> Game Instance List
  -> Game Panel Frontend Module
  -> GameDriver
  -> Game Server Containers
```

但产品显示策略分两个阶段。

### Single Game Mode

当前上线版本默认使用单游戏直达模式：

```text
PANEL_MODE=single
DEFAULT_INSTANCE_ID=stardew
DEFAULT_DRIVER_ID=stardew_junimo
```

用户体验：

```text
打开面板
  -> 初始化管理员 / 登录
  -> 直接进入 Stardew 面板
```

不显示：

- 总面板游戏列表。
- 选择游戏页面。
- 多游戏入口。

内部仍创建并使用一条 instance：

```text
instances
  id: stardew
  driver_id: stardew_junimo
  name: Stardew Valley
  data_dir: /data/instances/stardew
```

前端路由可以内部保留：

```text
/                  single 模式下自动进入 Stardew 面板
/instances/stardew Stardew 面板
```

### Multi Game Mode

等开发第二个游戏面板时再开启：

```text
PANEL_MODE=multi
```

此时才显示总面板：

```text
/                    总面板游戏实例列表
/instances/stardew   Stardew 面板
/instances/minecraft Minecraft 面板
```

推荐规则：

```text
if PANEL_MODE == single and only one instance:
    登录后直接进入该实例面板

if PANEL_MODE == multi or instances > 1:
    登录后进入总面板实例列表
```

### Global Panel

总面板是未来 Multi Game Mode 下显示的页面，负责所有游戏都共用的能力：

- 面板用户、登录、权限。
- 所有游戏实例列表。
- 所有游戏实例的运行状态摘要。
- 全局任务中心和 job logs。
- 全局 Docker / Compose 健康检查。
- 全局审计日志、备份记录和基础设置。

示例：

```text
游戏        实例名      状态      玩家       操作
Stardew     朋友农场    running   2 / 8      进入面板
Minecraft   生存服      stopped   0 / 20     进入面板
DST         洞穴服      running   3 / 6      进入面板
```

### Game Panel Frontend Module

进入某个游戏实例后，不同游戏可以加载不同的前端页面模块。不要强行让所有游戏共用 Stardew 的页面。

建议前端结构：

```text
frontend/src/core
frontend/src/games/stardew
frontend/src/games/minecraft
frontend/src/games/dont_starve_together
frontend/src/games/terraria
frontend/src/games/palworld
```

通用页面骨架可以复用，例如概览、任务、日志、启动/停止/重启、备份入口。但游戏专属页面必须允许差异化：

- Stardew: Steam Guard、邀请码、农场设置、小屋、Junimo `attach-cli`。
- Minecraft: `server.properties`、EULA、RCON、白名单、OP、世界管理、插件/Mod。
- Don't Starve Together: cluster token、Master/Caves、玩家白名单。
- Terraria: world 文件、难度、tModLoader。
- Palworld: `PalWorldSettings.ini`、管理员密码、玩家上限。

### GameDriver

后端 `GameDriver` 与前端 game module 对应。总面板只通过 registry 找到当前实例的 driver，然后调用统一方法。具体命令、文件路径、容器名、配置格式和控制台协议都属于 driver 内部细节。

### Current Scope

当前仓库第一阶段只实现：

```text
Single Game Mode
  + Global Panel 基础能力
  + Stardew game panel 直接显示
  + games/stardew_junimo driver
```

首版不要为了未来多游戏而强制用户看到总面板。后续接入 Minecraft、饥荒、泰拉瑞亚、幻兽帕鲁时，应新增对应 `frontend/src/games/<game>` 和 `backend/internal/games/<game>`，并把 `PANEL_MODE` 切到 `multi` 或根据实例数量自动显示总面板。不要把逻辑写进 Stardew 模块，也不要在 API handler 中用大量 `if game == ...` 分支堆业务。

## Suggested Directory Layout

```text
stardew-server-anxi-panel
├─ backend
│  ├─ cmd
│  │  └─ panel
│  ├─ internal
│  │  ├─ auth
│  │  ├─ docker
│  │  ├─ jobs
│  │  ├─ games
│  │  │  ├─ registry
│  │  │  └─ stardew_junimo
│  │  │     ├─ install
│  │  │     ├─ lifecycle
│  │  │     ├─ saves
│  │  │     ├─ mods
│  │  │     ├─ console
│  │  │     └─ config
│  │  ├─ storage
│  │  └─ web
│  └─ migrations
│
├─ frontend
│  └─ src
│
├─ docs
│  └─ architecture.md
│
└─ data
   ├─ panel.db
   ├─ instances
   │  └─ stardew
   │     ├─ docker-compose.yml
   │     ├─ .env
   │     ├─ .local-container
   │     ├─ saves
   │     └─ mods
   └─ backups
```

实际代码目录可以随着实现调整，但建议保持以下边界：

- `auth`: 面板初始化注册、登录、session、权限。
- `docker`: 通用 Docker / Compose 控制层，封装 `pull`、`up`、`down`、`restart`、`ps`、`logs`、`exec` 等能力。
- `jobs`: 安装、认证、启动等长期任务和任务日志。
- `games`: 不同游戏 driver 的注册和调度。
- `games/registry`: 游戏 driver 注册表和统一调度入口。
- `games/stardew_junimo`: Stardew Valley + JunimoServer 的具体实现。
- `games/stardew_junimo/install`: 下载/生成 Junimo 配置、写入 `.env`、拉取镜像、执行 Steam Auth。
- `games/stardew_junimo/lifecycle`: 启动、停止、重启、状态检查。
- `games/stardew_junimo/saves`: 通过 Junimo 挂载目录和 Junimo CLI 管理存档。
- `games/stardew_junimo/mods`: 通过 Junimo 挂载目录管理 Mod。
- `games/stardew_junimo/console`: 通过 `attach-cli`、SMAPI 命令和日志流与 Junimo 容器通信。
- `games/stardew_junimo/config`: 管理 `.env`、`server-settings.json` 等 Junimo 相关配置。
- `storage`: 面板自己的 SQLite、迁移、持久化模型，例如用户、session、任务、实例状态和审计日志。

注意：`saves`、`mods`、`console` 这些能力不应该作为顶层通用模块直接实现 Stardew 逻辑。它们应优先放在具体游戏 driver 内部，通过 Docker / Compose、容器 `exec`、Junimo `attach-cli`、HTTP status、日志流和挂载目录完成。Panel 后端负责鉴权、编排、状态记录和 API 转发，不替代 JunimoServer 本身。

## Core State Machine

不要把后端做成简单的“按钮触发 shell 命令”。核心流程应建模为状态机，前端根据状态决定按钮是否可用、下一步该展示什么。

建议初始状态：

```text
uninitialized
admin_created
junimo_scaffolded
credentials_required
steam_auth_running
steam_auth_failed
steam_auth_done
game_installed
save_required
ready_to_start
starting
running
stopped
error
```

含义：

- `uninitialized`: 面板首次启动，还没有管理员账号。
- `admin_created`: 管理员已初始化。
- `junimo_scaffolded`: 已创建 Junimo 工作目录，并下载/生成 `docker-compose.yml` 和 `.env`。
- `credentials_required`: 需要用户填写 Steam 账号、Steam 密码、VNC 密码。
- `steam_auth_running`: 正在执行 Steam 认证流程。
- `steam_auth_failed`: Steam 认证失败，例如密码错误或 Guard 失败。
- `steam_auth_done`: Steam 认证完成。
- `game_installed`: 游戏文件安装完成。
- `save_required`: 启动前需要选择上传存档、已有存档或新建存档。
- `ready_to_start`: 已满足启动条件。
- `starting`: 正在启动 JunimoServer。
- `running`: 服务器正在运行。
- `stopped`: 服务器已停止。
- `error`: 出现需要人工处理的错误。

## MVP Flow

第一阶段 MVP 应聚焦跑通完整闭环：

1. 用户打开面板。
2. 如果没有管理员账号，进入初始化注册页。
3. 登录后进入安装向导。
4. 后端创建 Junimo 工作目录。
5. 后端下载或生成 `docker-compose.yml` 和 `.env`。
6. 用户输入 Steam 账号、Steam 密码、VNC 密码。
7. 后端写入 `.env`。
8. 后端执行 `docker compose pull`。
9. 后端执行 `docker compose run --rm -it steam-auth setup`。
10. 前端实时展示 Steam Guard 交互信息。
11. 用户在前端输入验证码、确认手机应用，或扫描二维码。
12. Steam 认证成功后，进入可启动状态。
13. 用户选择存档：上传存档、读取已有存档或新建存档。
14. 后端执行 `docker compose up -d`。
15. 后端通过 `docker compose exec server attach-cli` 或 HTTP status 获取邀请码和服务器信息。
16. 前端展示服务器状态、邀请码、日志和控制台入口。

## Panel Product Workflow

本节是产品流程的实现准绳。后续原型、前端页面、后端 API 和状态机都应以这里为准。

### 0. User Runs Panel Image

用户的第一步不是手动创建 Junimo 目录，而是拉取并运行本项目的面板镜像：

```bash
docker run -d \
  --name anxi-panel \
  -p 8090:8090 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v anxi-panel-data:/data \
  ghcr.io/yourname/stardew-server-anxi-panel:latest
```

面板容器启动后，后端自动做这些事：

- 创建 `/data/instances/stardew`。
- 下载或写入 JunimoServer `docker-compose.yml`。
- 下载或写入 `.env.example` 并生成 `.env`。
- 初始化 SQLite。
- 检查是否已有管理员账号。

此时还不拉 Junimo 镜像，不执行 Steam Auth，不启动服务器。用户打开 `http://host:8090` 后先进入管理员初始化。

### 1. Admin Initialization

如果数据库中没有管理员：

- 前端显示管理员初始化注册页。
- 用户输入面板管理员账号和密码。
- 后端创建管理员用户，密码使用 Argon2id。
- 状态进入 `admin_created`。
- 用户进入登录态并跳转主界面。

这一阶段只操作面板自身 SQLite，不调用 Junimo 容器。

### 2. Main Dashboard Before Install

主界面显示：

- `安装游戏` 按钮可用。
- `启动服务器` 按钮不可用。
- 如果用户点击启动，提示：`请先安装游戏`。
- 显示 Junimo 配置已准备，但游戏未安装。

这对应状态：

```text
admin_created or junimo_scaffolded or credentials_required
```

### 3. Install Game Modal

用户点击 `安装游戏` 后弹出安装弹窗：

- Steam 账号。
- Steam 密码。
- VNC 密码。

用户确认后：

- 后端把输入写入实例目录下的 `.env`。
- 写入字段至少包括 `STEAM_USERNAME`、`STEAM_PASSWORD`、`VNC_PASSWORD`。
- 后端执行 `docker compose pull`。
- 后端执行 `docker compose run --rm -it steam-auth setup`。
- 状态进入 `steam_auth_running`。

日志必须脱敏，不允许输出 Steam 密码和 VNC 密码。

### 4. Steam Password and Steam Guard Loop

Steam 认证不是一次性线性流程，而是一个可重试循环。

如果 `steam-auth setup` 输出密码错误或认证失败：

- 后端标记 `steam_auth_failed` 或回到 `credentials_required`。
- 前端重新弹出账号/密码修改界面。
- 用户修改后，后端重新写入 `.env`。
- 再次执行 `docker compose run --rm -it steam-auth setup`。
- 直到密码正确或用户取消。

如果 Steam Guard 需要用户操作：

- 后端通过 PTY 实时读取输出。
- 前端展示当前提示。
- 如果是邮箱验证码，前端显示验证码输入框。
- 如果是手机 App 确认，前端显示等待确认状态。
- 如果是二维码，前端完整展示二维码或二维码链接。
- 用户在前端输入验证码后，后端写入 PTY stdin。

认证成功后：

- 状态进入 `steam_auth_done`。
- 后端确认游戏文件已准备好后进入 `game_installed`。
- 主界面 `安装游戏` 显示已完成或可重新安装。
- `启动服务器` 按钮变为可用。

### 5. Start Server Requires Save Selection

用户点击 `启动服务器` 后，不应立即执行 `docker compose up -d`。必须先弹出存档选择界面。

存档选择界面提供三种入口：

- 上传存档。
- 读取已有存档并展示。
- 新建存档。

用户确认后：

- 如果上传存档，后端校验并写入 Junimo 使用的挂载目录。
- 如果选择已有存档，后端记录 active save，必要时调用 Junimo `saves select <name> --confirm`。
- 如果新建存档，后端设置新游戏策略，必要时调用 `settings newgame --confirm` 或编辑 `server-settings.json`。
- 状态进入 `ready_to_start`。

然后后端执行：

```bash
docker compose up -d
```

状态进入：

```text
starting -> running
```

### 6. Invite Code Fetch

服务器启动完成后，面板必须自动获取邀请码并展示在前端。

推荐路径：

```bash
docker compose exec server attach-cli
```

然后发送：

```text
invitecode
```

或发送：

```text
info
```

如果 Junimo HTTP status/API 已启用且可用，也可以从 HTTP 状态中读取邀请码和服务器信息。

前端展示：

- 邀请码。
- 当前服务器状态。
- 玩家数。
- 当前存档。
- 复制按钮。

### 7. Daily Management Pages

安装和启动闭环完成后，面板提供这些页面：

- 状态页：服务器运行状态、玩家状态、容器状态、游戏内日期、运行时间。
- 指令/喊话页：常用 Junimo/SMAPI 指令、服务器喊话、命令输出。
- 存档页：上传、切换、删除、备份存档。
- Mod 页：上传、删除、导出 Mod，提示是否需要重启。
- 用户管理页：管理面板控制权，区分普通用户和管理员。

这些功能都应通过 `games/stardew_junimo` driver 与 Junimo 容器通信。Junimo 已暴露的能力优先使用；Junimo 没有暴露或能力不足的地方，面板再补充自己的逻辑。

## JunimoServer Flow Mapping

本项目不是重新实现 Stardew 服务端，而是把 JunimoServer 官方命令流程封装成面板状态机、任务队列和可视化交互。

官方流程参考：

- Admins overview: `https://stardew-valley-dedicated-server.github.io/server/admins/`
- Installation: `https://stardew-valley-dedicated-server.github.io/server/admins/quick-start/installation.html`
- First setup: `https://stardew-valley-dedicated-server.github.io/server/admins/quick-start/first-setup.html`
- Environment variables: `https://stardew-valley-dedicated-server.github.io/server/admins/configuration/environment.html`
- Server settings: `https://stardew-valley-dedicated-server.github.io/server/admins/configuration/server-settings.html`
- Commands: `https://stardew-valley-dedicated-server.github.io/server/admins/operations/commands.html`

### Command Ownership

所有 JunimoServer 命令都由后端 `games/stardew_junimo` driver 调用。前端只发业务请求，例如“安装游戏”“提交 Steam Guard 验证码”“启动服务器”“获取邀请码”，不能直接提交 shell 命令。

```text
React Frontend
  -> Go API
  -> jobs state machine
  -> games/registry
  -> games/stardew_junimo driver
  -> docker compose / mounted files / attach-cli / Junimo HTTP API
  -> JunimoServer containers
```

### Panel Step to Junimo Command Matrix

| Panel step | State transition | Files touched | Command or communication |
| --- | --- | --- | --- |
| 管理员初始化 | `uninitialized` -> `admin_created` | `panel.db` | 面板自身 SQLite，不调用 Junimo |
| 创建实例目录 | `admin_created` -> `junimo_scaffolded` | `/data/instances/stardew` | `mkdir` equivalent in Go filesystem APIs |
| 获取 Junimo 配置 | `junimo_scaffolded` | `docker-compose.yml`, `.env` | 下载官方 `docker-compose.yml` 和 `.env.example`，或使用内置模板写入 |
| 写入凭据 | `junimo_scaffolded` -> `credentials_required` -> `steam_auth_running` | `.env` | 写入 `STEAM_USERNAME`, `STEAM_PASSWORD`, `VNC_PASSWORD`，可选写入 `API_PORT`, `SERVER_FPS`, `SERVER_PASSWORD`, `API_KEY` |
| 拉取镜像 | `steam_auth_running` | job logs | `docker compose pull` |
| 首次 Steam 认证 | `steam_auth_running` -> `steam_auth_done` or `steam_auth_failed` | Steam auth volume/session | `docker compose run --rm -it steam-auth setup` through PTY |
| 启动服务器 | `ready_to_start` -> `starting` -> `running` | `.local-container/`, settings volume | `docker compose up -d` |
| 停止服务器 | `running` -> `stopped` | none | `docker compose down` |
| 重启服务器 | `running` -> `starting` -> `running` | none | `docker compose restart` |
| 状态检查 | no state change or reconcile state | none | `docker compose ps`, `docker compose logs`, Junimo HTTP status |
| 获取邀请码 | `running` | none | `docker compose exec server attach-cli`, then send `invitecode` or `info`; or Junimo HTTP status if available |
| 查看日志 | no state change | job logs | `docker compose logs -f`, optionally service-specific logs |
| 存档管理 | `game_installed` -> `save_required` -> `ready_to_start` | mounted save/settings dirs | Prefer `attach-cli` commands: `saves`, `saves info <name>`, `saves select <name> --confirm`; file upload uses mounted dirs |
| 新游戏设置 | before restart/start | `server-settings.json` | Edit `/data/settings/server-settings.json` through mounted file or `docker compose cp`; alternatively use `settings newgame --confirm` when appropriate |
| Mod 管理 | usually `restart_required` side state | mounted Mods dir | File operations on mounted mod directory; verify with `docker compose logs server`; restart to apply |
| 控制台命令 | no state change | none | `docker compose exec server attach-cli`, allowlisted commands only |

### Container Communication Channels

`stardew_junimo` driver should use these channels, in this order:

1. **Mounted files**  
   Used for `.env`, `docker-compose.yml`, uploaded saves, uploaded mods, backups, and local metadata. This is the safest path for panel-owned files.

2. **Docker Compose lifecycle commands**  
   Used for `pull`, `up -d`, `down`, `restart`, `ps`, `logs`, `exec`, and `run`.

3. **TTY for Steam Auth**  
   Used for `steam-auth setup` and `steam-auth download`. The backend owns stdin/stdout and streams output to the frontend; Windows uses Docker Engine API with `Tty:true`, while Unix-like hosts use Docker Compose through a PTY.

4. **Junimo CLI through `attach-cli`**  
   Used for `info`, `invitecode`, `settings`, `saves`, `cabins`, `rendering`, `host-auto`, and `host-visibility`. The driver should keep an allowlist of commands exposed to each panel role.

5. **Junimo HTTP API / status endpoint**  
   Used for status and monitoring when enabled and available. `.env` can configure `API_PORT`, `API_ENABLED`, and `API_KEY`. Do not expose Junimo API externally without an API key.

6. **VNC**  
   VNC is not part of normal panel control. It is only for advanced debugging. The panel may expose a link/status for VNC, but normal management should rely on CLI, HTTP status, logs, and mounted files.

### State Reconciliation

The panel state machine is the source of UI flow, but Docker and Junimo are the source of runtime truth. On backend startup and before major actions, reconcile state using:

```text
docker compose ps
docker compose logs --tail
Junimo HTTP /health or /status if enabled
attach-cli info when server is running
presence of .env, docker-compose.yml, .local-container
presence of selected save metadata
```

For example:

- If panel state says `running` but `docker compose ps` shows the server stopped, set state to `stopped` or `error`.
- If `steam-auth setup` succeeds but the server is not started yet, keep state at `game_installed` or `save_required`.
- If the user clicks start before installation, return a structured error: `请先安装游戏`。
- If the user clicks start before choosing save/new-game strategy, return `save_required` and route frontend to save selection.

## Steam Guard Interaction

Steam 认证是本项目早期最关键的技术点。

后端不能只用普通 `exec.Command` 等待命令结束。应使用 PTY 运行交互命令：

```bash
docker compose run --rm -it steam-auth setup
```

推荐行为：

- 后端启动 PTY 进程。
- 后端实时读取 stdout/stderr。
- 日志通过 WebSocket 或 SSE 推送给前端。
- 前端展示终端输出、二维码、验证码输入框和当前提示。
- 用户输入验证码后，前端通过 WebSocket/API 发给后端。
- 后端写入 PTY stdin。
- 命令退出码为 0 时标记认证成功。
- 检测到密码错误、Guard 失败或超时时，状态回到 `credentials_required` 或 `steam_auth_failed`。

如果 Junimo 输出二维码 URL，前端可直接生成二维码。如果输出 ASCII QR，则可原样在终端区域展示。

### Junimo Steam Auth 重要实现细节

后续维护者必须按 Junimo 上游行为理解 Steam Auth，而不是只按面板 UI 的两个按钮理解：

- 官方 compose 使用 `IMAGE_VERSION` 控制 Junimo server 镜像版本，不使用面板早期自定义的 `JUNIMO_IMAGE_TAG`。
- `steam-auth` sidecar 由 `STEAM_SERVICE_IMAGE` 独立控制，当前默认使用 `anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`。该镜像基于 JunimoServer `tools/steam-service`，只修补 SteamClient 连接等待和认证会话重试，保留上游 HTTP API、ticket、lobby、download 等行为。
- 新实例 `.env` 会写入 `STEAM_CLIENT_CONNECT_TIMEOUT_SECONDS=60`、`STEAM_CLIENT_CONNECT_RETRIES=5`、`STEAM_AUTH_SESSION_RETRIES=3`、`STEAM_AUTH_SESSION_RETRY_DELAY_SECONDS=5`。本地联调 fork 镜像时，可把 `STEAM_SERVICE_IMAGE` 覆盖成 `junimo-steam-service-cn:auth-retry-test`。
- 官方关键服务名是 `steam-auth`、`server`、`discord-bot`；后续 `docker compose exec server attach-cli`、认证和状态检查都依赖这些名字。
- 官方关键存储是 Docker named volumes：`steam-session:/data/steam-session` 保存 Steam session，`game-data:/data/game` 保存下载的游戏文件，`saves:/config/xdg/config/StardewValley` 保存 Stardew 存档；本地绑定目录 `./.local-container/settings:/data/settings` 保存 server settings。
- `steam-auth` 和 `server` 都应保留 `stdin_open: true` 与 `tty: true`，因为上游认证和后续 CLI 都是交互式流程。
- 面板主动提供第一层 Steam 登录方式选择：`auth_method_required` 显示“扫码登录 / 账号密码/验证码登录”。选择扫码时启动 `steam-auth setup` 并自动向上游第一层 `Choose authentication method` 输入 `2`；选择账号密码/验证码时启动 `steam-auth download`。账号密码路径触发的 `Steam Guard Authentication` 才是第二层 `steam_guard_choice_required`，面板显示“手机 App 批准 / 输入验证码”。
- 上游 `steam-auth setup` 输出 `Choose authentication method` 后，选择 `[1] Username & Password` 并不会自动使用 `.env`；它还会继续从 stdin 读取 Steam 用户名和 Steam 密码，且密码读取使用 `Console.ReadKey()`。在面板后台任务的 stdin pipe 环境中，这会报 `Cannot read keys when either application does not have a console or when console input has been redirected`。因此面板账号密码安装路径应优先使用 `steam-auth download`，让上游通过 `EnsureLoggedInAsync(LoginConfig)` 使用 `.env` 中的账号密码完成非交互登录和游戏下载。
- 选择 `[2] QR Code` 时，未修补的上游 QR 登录可能在生成二维码前报 `QR authentication failed: The SteamClient instance must be connected`。CN sidecar 已改为等待 `ConnectedCallback` 后再开始 QR / credentials / token 登录，并对 `TryAnotherCM`、`AsyncJobFailedException`、认证阶段断线和超时做认证会话重试。
- 当前实现使用跨平台 TTY 跑 `steam-auth setup` 的扫码路径，使用 `steam-auth download` 处理账号密码安装路径；不要用普通 stdin 重定向去跑 `setup` 的账号密码分支。

## GameDriver Abstraction

为了后续支持多个游戏，Stardew + JunimoServer 不应写死在主业务逻辑里。`GameDriver` 是“总面板调度不同游戏”的后端边界，不是要求所有游戏使用相同页面或相同容器命令。

`GameDriver` 不是让 Panel 重新实现游戏服务端能力，而是定义“总面板如何向某个游戏实例下达通用意图”。具体到 Stardew，`stardew_junimo` driver 应通过 JunimoServer 已暴露的能力工作：Docker Compose、容器 `exec`、`attach-cli`、HTTP status、日志流、挂载目录和配置文件。具体到 Minecraft，将来可以通过 Minecraft 镜像、`server.properties`、RCON、白名单文件和世界目录完成同样的面板意图。

示例接口：

```go
type GameDriver interface {
    ID() string
    Name() string

    Prepare(ctx context.Context, instance Instance) error
    Install(ctx context.Context, req InstallRequest) (*Job, error)
    Start(ctx context.Context, req StartRequest) (*Job, error)
    Stop(ctx context.Context, instance Instance) error
    Restart(ctx context.Context, instance Instance) error

    Status(ctx context.Context, instance Instance) (*ServerStatus, error)
    Logs(ctx context.Context, instance Instance) (<-chan LogLine, error)
    ExecCommand(ctx context.Context, cmd string) (*CommandResult, error)

    ListSaves(ctx context.Context, instance Instance) ([]SaveInfo, error)
    UploadSave(ctx context.Context, file UploadedFile) error
    SelectSave(ctx context.Context, name string) error
    DeleteSave(ctx context.Context, name string) error

    ListMods(ctx context.Context, instance Instance) ([]ModInfo, error)
    UploadMod(ctx context.Context, file UploadedFile) error
    DeleteMod(ctx context.Context, id string) error
}
```

`stardew_junimo` 是第一个 driver，也是当前唯一要做扎实的 driver。

后续可以增加：

- Minecraft driver
- Don't Starve Together driver
- Terraria driver
- Palworld driver
- Valheim driver

每个 driver 负责自己游戏的 Compose 模板、安装流程、配置文件、状态解析、存档和 Mod 规则。

其中存档、Mod、控制台等能力应尽量走该游戏服务端容器已有的通信方式。只有 JunimoServer 没有暴露、或暴露能力不足的地方，Panel 才补充自己的逻辑。

前端也应遵守同样边界：总面板可以复用布局、权限、任务中心和状态卡片，但游戏专属交互应放在对应 game module 中。例如 Stardew 的 Steam Guard 和邀请码不应出现在 Minecraft 面板里；Minecraft 的 RCON、白名单和 OP 管理也不应污染 Stardew 模块。

## Jobs and Instance State Baseline

Milestone 4 已落地通用 jobs/state 基础能力，供后续 Junimo 安装、Steam Auth、服务器启动和日志流复用。

当前持久化模型：

```text
jobs
  id
  type
  status: queued / running / succeeded / failed / canceled
  target_type
  target_id
  created_by
  created_at
  started_at
  finished_at
  error_message

job_logs
  id
  job_id
  level: info / warn / error / debug
  message
  created_at
  sequence

instance_state
  instance_id
  driver_id
  state
  state_message
  last_job_id
  updated_at
  updated_by
```

当前流式协议选择 SSE：

```text
GET /api/jobs/:id/stream
```

事件约定：

```text
event: log       data: job log line, id = job_logs.sequence
event: finished  data: final job payload
event: ping      heartbeat
```

当前单实例临时入口：

```text
GET /api/instances/stardew/state
```

该入口用于 Single Game Mode 的本机联调。Milestone 5 应将它收口到通用 instance API：

```text
GET /api/instances/:instance_id/state
```

Job Manager 是通用基础设施，不应包含 Stardew/Junimo 专属业务。真实安装、Steam Auth、启动等流程应由后续 `games/stardew_junimo` driver 创建 job，并通过 job context 写入脱敏日志和更新 instance state。

## Migration Notes After Milestones 0-4

Milestone 0、1、2、3、4 都属于总面板基础设施，已经完成或正在进行的方向不需要推翻重做。

这些能力本来就是所有游戏共享的：

- 项目骨架。
- 后端配置。
- SQLite。
- 用户、登录、权限。
- Docker / Compose 通用执行层。
- jobs、job logs、SSE/WebSocket 日志流。
- instance state 基础能力。

当前 0-4 阶段允许存在一些 Stardew 单实例临时约定，例如：

```text
$PANEL_DATA_DIR/instances/stardew
GET /api/instances/stardew/state
默认 Compose 项目目录指向 stardew
```

这些临时约定不是最终架构。必须在 Milestone 5 之后逐步收进实例模型和 GameDriver 边界。

推荐补救迁移：

```text
临时写法:
GET /api/docker/ps -> $PANEL_DATA_DIR/instances/stardew

目标写法:
GET /api/instances/:instance_id/docker/ps
  -> load instance
  -> find instance.driver_id
  -> registry.Get(driver_id)
  -> driver returns compose/project dir or status implementation
  -> docker layer executes allowlisted operation
```

状态也要分层：

```text
通用状态:
installing
installed
starting
running
stopped
error

Stardew 专属状态或阶段:
steam_auth_running
steam_guard_required
steam_auth_failed
invite_code_ready
save_required
```

如果 Milestone 4 已经使用 `steam_auth_running`、`save_required` 等 Stardew 状态，也可以保留，但后续应把它们标记为 driver-specific phase，避免 Minecraft、DST 等游戏被迫继承 Stardew 语义。

结论：0-4 不需要返工；Milestone 5 负责把临时单实例写法收进 `instances + GameDriver registry`，Milestone 8 负责把前端调整成“Single Game Mode 下直达 Stardew game module，Multi Game Mode 下显示总面板”。

## Frontend Pages

MVP 推荐页面：

- 初始化注册页：首次打开面板时创建管理员账号。
- 登录页：管理员和普通用户登录。
- Single Game Mode 入口：登录后直接进入 Stardew 专属面板。
- Multi Game Mode 总面板：首页展示所有游戏实例的状态摘要、任务概览和入口；首版默认隐藏。
- 安装向导页：输入 Steam/VNC 信息，展示 pull、install、Steam Guard 的实时状态。
- 首页/控制台页：启动、停止、重启、邀请码、当前状态、玩家数量。
- 状态页：服务器运行状态、容器状态、玩家状态、游戏内日期、运行时间。
- 指令页：执行 Junimo/SMAPI 命令，支持服务器喊话。
- 存档页：上传、读取已有存档、切换、新建、删除、备份。
- Mod 页：上传、删除、启用/禁用、导出 Mod 包。
- 用户管理页：管理面板用户、管理员、普通用户和权限。

## Database Model

SQLite 初始表建议：

```text
users
sessions
instances
game_drivers
jobs
job_logs
server_state
panel_settings
audit_logs
```

建议原则：

- 用户密码使用 Argon2id 哈希。
- session 使用 HttpOnly Cookie。
- 敏感配置不要进入普通日志。
- Steam 密码默认不长期保存。
- 如果必须保存 Steam 密码，应使用面板级 `PANEL_SECRET` 加密。
- 所有关键操作写入 `audit_logs`，方便排查问题。

## Security Notes

挂载 Docker Socket 等同于给 Panel 容器很高权限，必须在文档和 UI 中明确提示。

需要特别注意：

- 不要在日志中输出 Steam 密码、VNC 密码、session token。
- `.env` 文件权限应尽量收紧。
- 上传存档和 Mod 时要做文件类型、大小和路径穿越检查。
- 不允许用户通过前端执行任意 shell 命令。
- 控制台命令应经过 driver 层白名单或明确的权限控制。
- 管理员和普通用户权限要分离。

## Development Phases

### Phase 1: Stardew MVP

- 管理员初始化。
- 登录。
- 创建 Junimo 工作目录。
- 写入 `docker-compose.yml` 和 `.env`。
- 执行 `docker compose pull`。
- Steam Auth 交互。
- 启动、停止、重启服务器。
- 获取并展示邀请码。
- 展示基础日志。

### Phase 2: Management Features

- 存档上传、切换、删除、备份。
- Mod 上传、删除、导出。
- 状态页。
- 控制台命令。
- 服务器喊话。
- 用户权限。
- 操作审计。

### Phase 3: General Game Panel

- 总面板实例列表。
- 前端 game module registry。
- 多游戏 driver 注册机制。
- 多实例管理。
- 游戏市场/模板列表。
- 统一日志。
- 统一备份恢复。
- 跨游戏权限模型。
- 更完整的插件化扩展能力。

## Architecture Decision Summary

本项目应从第一天起按“通用游戏开服面板”的方向设计，但第一版只把 Stardew Valley + JunimoServer 做扎实。

核心架构决策：

- 使用 Go 作为后端。
- 使用 React + TypeScript + Vite 作为前端。
- 使用 SQLite 作为本地数据库。
- 使用 Docker Socket 控制宿主机 Docker。
- 使用 GameDriver 抽象隔离不同游戏。
- 使用状态机管理安装、认证、启动、运行流程。
- 使用 WebSocket/SSE 处理日志流和 Steam Guard 交互。
- 先做单实例 Stardew MVP，再扩展多游戏、多实例平台。
