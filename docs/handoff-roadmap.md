# stardew-server-anxi-panel Handoff Roadmap

本文档用于给后续 Codex、Claude 或人工开发者接手项目时快速进入状态。

项目架构以 [architecture.md](architecture.md) 为准：Go 后端、React + TypeScript 前端、SQLite、本地 Docker Socket、GameDriver 插件化抽象。本文只负责把大目标切成可执行的小目标，并说明每一步应该做什么、怎么做、做到什么程度算完成。

## Current Context

项目目标：

- 基于 JunimoServer 做 Stardew Valley 专用服务器 Web 管理面板。
- 用户通过浏览器完成管理员初始化、Steam 认证、Junimo 安装、服务器启动、邀请码展示、状态查看、存档管理、Mod 管理、控制台指令、面板用户管理。
- 长期演进为多游戏开服总面板：总面板展示所有游戏实例状态，点击某个游戏实例后进入该游戏自己的专属管理面板。第一版只做好 Stardew + JunimoServer，并默认使用 Single Game Mode，登录后直接进入 Stardew 面板，不显示总面板游戏列表。

当前已有文档：

- `docs/architecture.md`: 技术架构和模块边界。
- `docs/prototypes/stardew-anxi-panel-product-prototype.html`: 产品原型 HTML。
- `docs/prototypes/stardew-anxi-panel-product-prototype.png`: 产品原型图。
- `docs/prototypes/stardew-anxi-panel-prototype-notes.md`: 原型说明。

## Development Principles

后续实现时请遵守这些约束：

- 不要把 Stardew 专属逻辑放到顶层 `saves`、`mods`、`console` 模块里。
- 顶层只保留通用能力：`auth`、`docker`、`jobs`、`games/registry`、`storage`、`web`。
- Stardew 相关能力放在 `games/stardew_junimo` driver 内。
- 前端也要分层：总面板、通用游戏面板骨架、各游戏自己的 game module。不要把 Minecraft、饥荒、泰拉瑞亚等未来页面塞进 Stardew 页面里。
- 面板后端不替代 JunimoServer，优先通过 Docker Compose、容器 `exec`、`attach-cli`、HTTP status、日志流和挂载目录与 JunimoServer 通信。
- 所有长任务必须有状态、日志、错误信息和可恢复策略。
- 不要把 Steam 密码、VNC 密码、session token 打到日志里。

## Product Model: Single Now, Multi Later

接手者必须理解：本项目最终不是“一个 Stardew 页面兼容所有游戏”，而是“一个总面板 + 多个游戏专属面板”。但首个可上线版本不要提前显示总面板，应该先使用单游戏直达体验。

当前产品模式：

```text
Single Game Mode
  -> 登录后直接进入 Stardew 面板
  -> 不显示总面板游戏列表
  -> 内部仍使用 instance + driver 架构
```

未来产品模式：

```text
Multi Game Mode
  -> 登录后进入总面板
  -> 展示多个游戏实例
  -> 点击进入对应游戏面板
```

建议配置：

```text
PANEL_MODE=single
DEFAULT_INSTANCE_ID=stardew
DEFAULT_DRIVER_ID=stardew_junimo
```

推荐路由行为：

```text
if PANEL_MODE == single and only one instance:
    / -> /instances/stardew

if PANEL_MODE == multi or instances > 1:
    / -> 总面板实例列表
```

```text
Global Panel
  ├─ Stardew Instance -> Stardew Panel -> stardew_junimo driver -> JunimoServer containers
  ├─ Minecraft Instance -> Minecraft Panel -> minecraft driver -> Minecraft containers
  ├─ DST Instance -> DST Panel -> dont_starve_together driver -> DST containers
  ├─ Terraria Instance -> Terraria Panel -> terraria driver -> Terraria containers
  └─ Palworld Instance -> Palworld Panel -> palworld driver -> Palworld containers
```

总面板在 Multi Game Mode 下显示，负责：

- 登录、用户、权限。
- 所有游戏实例列表。
- 所有游戏实例的状态摘要。
- 全局 Docker 状态。
- 全局任务中心。
- 审计日志和基础设置。

游戏专属面板负责：

- 该游戏自己的安装向导。
- 该游戏自己的配置项。
- 该游戏自己的控制台协议。
- 该游戏自己的存档/世界规则。
- 该游戏自己的 Mod/插件规则。
- 该游戏自己的特殊 UI，例如 Stardew 的 Steam Guard / 邀请码，Minecraft 的 RCON / 白名单 / OP。

当前第一版只实现 Stardew，UI 默认隐藏总面板，但代码和文档要按这个模型留边界。

## Core Abstraction: GameDriver

`GameDriver` 是本项目最重要的后端长期抽象。这个面板后面会支持多个游戏，所以从第一版开始就不能把 Stardew 写死在主业务、API handler、jobs 或 docker 层里。

主业务只知道“当前实例使用哪个 driver”。具体游戏怎么准备、安装、启动、读取状态、管理存档、管理 Mod、执行命令，都由对应 driver 实现。`GameDriver` 不代表所有游戏共用同一套页面或命令，它只代表总面板调用各游戏后端能力的统一边界。

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

第一版 driver 是：

```text
games/stardew_junimo
```

后续可以增加：

```text
games/minecraft
games/dont_starve_together
games/terraria
games/palworld
```

每个 driver 自己负责：

- Compose 模板或容器模板。
- 安装流程。
- 配置文件。
- 状态解析。
- 日志读取。
- 控制台命令。
- 存档规则。
- Mod 规则。

`auth`、`jobs`、`docker`、`storage`、`web` 都是通用基础设施，不应该出现 Stardew 专属业务判断。API handler 应通过 `games/registry` 找到 driver，再调用 driver 方法。

前端对应也应有 game module 边界：

```text
frontend/src/core
frontend/src/games/stardew
frontend/src/games/minecraft
frontend/src/games/dont_starve_together
frontend/src/games/terraria
frontend/src/games/palworld
```

第一版可以只有 `frontend/src/games/stardew`，并在 Single Game Mode 下直接显示它；但不要把未来 Minecraft / DST / Terraria / Palworld 的页面逻辑放进 Stardew 模块。

## JunimoServer Integration Plan

接手者必须先理解：本面板不是另写一个 Stardew 服务端，而是把 JunimoServer 官方流程变成可视化、可恢复、可审计的 Web 工作流。

官方流程来源：

- `https://stardew-valley-dedicated-server.github.io/server/admins/`
- `https://stardew-valley-dedicated-server.github.io/server/admins/quick-start/installation.html`
- `https://stardew-valley-dedicated-server.github.io/server/admins/quick-start/first-setup.html`
- `https://stardew-valley-dedicated-server.github.io/server/admins/configuration/environment.html`
- `https://stardew-valley-dedicated-server.github.io/server/admins/configuration/server-settings.html`
- `https://stardew-valley-dedicated-server.github.io/server/admins/operations/commands.html`

### Official Flow to Panel Flow

| Official Junimo step | Panel UI step | Backend owner | Command / action |
| --- | --- | --- | --- |
| `mkdir junimoserver && cd junimoserver` | 创建 Stardew 实例 | `games/stardew_junimo/install` | Go filesystem creates `/data/instances/stardew` |
| Download `docker-compose.yml` | 准备 Junimo 配置 | `games/stardew_junimo/install` | Download official file or write embedded template |
| Download `.env.example`, rename `.env` | 准备 `.env` | `games/stardew_junimo/config` | Write `.env` under instance dir |
| Set `STEAM_USERNAME`, `STEAM_PASSWORD`, `VNC_PASSWORD` | 输入 Steam/VNC 信息 | `games/stardew_junimo/config` | Rewrite `.env`; redact values in logs |
| `docker compose pull` | 拉取 Junimo 镜像 | `internal/docker` through driver | `docker compose pull` |
| `docker compose run --rm -it steam-auth setup` | Steam Guard 认证 | `jobs` + `stardew_junimo/install` | Run through PTY, stream stdout, accept frontend stdin |
| `docker compose up -d` | 启动服务器 | `stardew_junimo/lifecycle` | `docker compose up -d` |
| `docker compose down` | 停止服务器 | `stardew_junimo/lifecycle` | `docker compose down` |
| `docker compose restart` | 重启服务器 | `stardew_junimo/lifecycle` | `docker compose restart` |
| `docker compose ps` | 状态页 | `stardew_junimo/lifecycle` | `docker compose ps` |
| `docker compose logs -f` | 日志页 | `stardew_junimo/console` | `docker compose logs -f`, service logs |
| `docker compose exec server attach-cli` | 控制台 / 邀请码 / 指令 | `stardew_junimo/console` | Attach and send allowlisted commands |
| `invitecode` | 展示邀请码 | `stardew_junimo/console` | Send command through `attach-cli` |
| `info` | 展示农场状态 | `stardew_junimo/console` | Send command through `attach-cli` |
| `settings show/validate/newgame` | 设置页 / 新游戏 | `stardew_junimo/config` | Prefer `attach-cli`; direct file edit only where safer |
| `saves`, `saves info`, `saves select --confirm` | 存档页 | `stardew_junimo/saves` | Prefer Junimo CLI; file upload uses mounted dirs |
| Mod file operations | Mod 页 | `stardew_junimo/mods` | Manage mounted Mods dir; restart if needed |

### Communication Rules

所有和 JunimoServer 的通信都要藏在 `games/stardew_junimo` driver 后面。

优先级：

1. Mounted files: `.env`, `docker-compose.yml`, saves, mods, backups, `server-settings.json`。
2. Docker Compose: `pull`, `up`, `down`, `restart`, `ps`, `logs`, `exec`, `run`。
3. PTY: 仅用于 `steam-auth setup` 这种交互命令。
4. `attach-cli`: 用于 `info`、`invitecode`、`settings`、`saves`、`cabins`、`rendering`、`host-auto` 等 Junimo 命令。
5. HTTP status/API: 用于状态轮询，只有启用并配置 API key 后使用。

禁止：

- 禁止前端传入任意 shell。
- 禁止 API handler 直接拼接 `docker compose` 命令。
- 禁止把 Junimo 的存档、Mod、控制台逻辑写到顶层通用模块。
- 禁止在日志里输出 Steam 密码、VNC 密码、session token。

## User Journey Implementation Contract

这一节是实现时的产品流程合同。后续开发者做页面、API、状态机和任务队列时，必须按这个顺序落地。

### Step 1: User Runs This Panel Image

用户只需要拉取并运行本面板镜像。面板容器启动时自动准备 Junimo 工作目录和配置文件。

后台自动执行：

```text
create /data/instances/stardew
write or download docker-compose.yml
write .env from .env.example
open panel port 8090
init SQLite panel.db
check whether admin exists
```

注意：

- 这一步不要求用户手动运行 Junimo 官方命令。
- 这一步不拉 Junimo 镜像。
- 这一步不执行 Steam Auth。
- 这一步不启动 Stardew 服务器。

完成后用户访问面板端口，看到管理员初始化注册界面。

### Step 2: Admin Initialization Page

用户输入：

```text
admin username
admin password
confirm password
```

后端执行：

```text
create first admin user
hash password with Argon2id
create HttpOnly session
write audit log
```

完成后进入主界面。

### Step 3: Main Dashboard Before Install

主界面必须展示：

```text
安装游戏按钮: enabled
启动服务器按钮: disabled
disabled reason: 请先安装游戏
Junimo 配置状态: 已准备
游戏安装状态: 未安装
```

如果用户直接点击启动服务器：

```text
return structured error: 请先安装游戏
route frontend to install prompt
```

### Step 4: Install Game Modal

点击 `安装游戏` 弹出输入框：

```text
Steam username
Steam password
VNC password
```

确认后后端执行：

```text
rewrite /data/instances/stardew/.env
docker compose pull
docker compose run --rm -it steam-auth setup
```

实现位置：

```text
games/stardew_junimo/config
games/stardew_junimo/install
internal/docker
internal/jobs
```

注意：

- 所有日志必须脱敏。
- `.env` 写入要用结构化方法，不要简单拼接不可信字符串。
- `steam-auth setup` 必须通过 PTY，因为要交互。

### Step 5: Password Error Retry Loop

如果 Steam 返回密码错误或认证失败：

```text
set state = steam_auth_failed or credentials_required
show password correction modal
rewrite .env
rerun docker compose run --rm -it steam-auth setup
```

这个循环一直持续到：

```text
Steam auth succeeds
or user cancels install
or job timeout/error
```

前端不应该让用户重新走完整安装向导，只需要回到凭据修改弹窗。

### Step 6: Steam Guard Frontend Interaction

如果 Steam Guard 需要操作，后端把 PTY 输出实时推给前端。

前端根据输出展示：

```text
auth method choice: QR scan / username password
Steam Guard choice: mobile app approval / enter code
email code input
mobile app confirmation waiting state
QR code full display
raw terminal output fallback
```

用户输入验证码后：

```text
frontend -> POST/WebSocket steam guard input
backend -> write to PTY stdin
job log -> append sanitized line
```

认证成功后：

```text
set state = steam_auth_done
verify game files/install result
set state = game_installed
enable start server button
```

### Step 7: Separate Save And Start Actions

界面始终提供独立的存档启动面板，包含“创建存档并启动”和“上传存档并启动”。普通 `启动服务器` 是单独动作，默认让 Junimo 加载上次使用的可用存档；仅在后端检查到零有效存档时返回 `save_required` 并引导到该面板。

界面提供：

```text
上传存档
读取已有存档展示
新建存档
```

用户确认后：

```text
upload save -> validate -> write mounted save dir
existing save -> select active save
new save -> mark new game strategy
set state = ready_to_start
docker compose up -d
```

如果用户在没有任何有效存档时点击普通启动：

```text
return state = save_required
scroll to create/upload-and-start panel
```

### Step 8: Fetch Invite Code After Start

`docker compose up -d` 完成后，后端自动运行：

```bash
docker compose exec server attach-cli
```

然后发送：

```text
invitecode
```

必要时发送：

```text
info
```

前端展示：

```text
invite code
copy button
server status
players
active save
```

### Step 9: Daily Management Pages

启动闭环后再做这些页面：

```text
状态页: 服务器运行状态、用户状态、容器状态
指令页: Junimo/SMAPI 指令、服务器喊话
存档页: 上传、切换、删除、备份
Mod 页: 上传、删除、导出、重启提示
用户管理页: 普通用户、管理员、权限
```

实现规则：

- 状态页通过 `docker compose ps`、Junimo HTTP status、`attach-cli info` 综合获取。
- 指令页通过 `attach-cli`，只允许 allowlist 命令。
- 存档页优先使用 Junimo `saves` 命令和挂载目录。
- Mod 页主要管理挂载目录，Junimo 未暴露的能力由面板补充。
- 用户管理页只操作面板 SQLite，不操作 Junimo。

### State to Command Rules

状态和命令必须一一对应，前端按钮只根据状态启用。

```text
uninitialized       -> only setup admin
admin_created       -> prepare Junimo instance
junimo_scaffolded   -> accept Steam/VNC credentials
credentials_required -> write .env, then pull/setup
steam_auth_running  -> stream PTY and accept guard input
steam_auth_failed   -> ask user to re-enter password or guard code
steam_auth_done     -> mark game installed or continue install verification
game_installed      -> ask for save strategy
save_required       -> upload/select/new save
ready_to_start      -> allow docker compose up -d
starting            -> poll compose ps and Junimo status
running             -> allow stop/restart/console/status
stopped             -> allow start/restart if installed and save-ready
error               -> show job logs and recovery action
```

## Milestone 0: Repo Skeleton ✅ 已完成（2026-06-22）

目标：建立项目骨架，让后续开发有稳定落点。

已完成：

- 已创建 `backend` Go 项目并初始化 `go.mod`。
- 已创建 `frontend` React + TypeScript + Vite 项目。
- 已建立基础目录结构。
- 已准备本地开发脚本。
- 后端已提供 `GET /health`，默认监听 `:8090`，支持 `PANEL_ADDR` 覆盖。
- README 已写明 backend/frontend 启动方式。

建议目录：

```text
backend
├─ cmd/panel
├─ internal/auth
├─ internal/docker
├─ internal/jobs
├─ internal/games/registry
├─ internal/games/stardew_junimo
├─ internal/storage
├─ internal/web
└─ migrations

frontend
└─ src
```

怎么做：

- 后端初始化 `go.mod`。
- 前端初始化 Vite React TS。
- 后端先提供 `/health`。
- 前端先能访问一个空壳页面。
- 暂时不要急着写 Docker 逻辑。

完成标准：

- `go test ./...` 可以跑。
- 前端 dev server 可以启动。
- 后端 `/health` 返回健康状态。
- README 或开发说明里写明本地启动方式。

## Milestone 1: Backend Foundation ✅ 已完成（2026-06-22）

目标：搭好 Go 后端的基础能力。

已完成：

- HTTP API server 保持标准库 `net/http`，并保留清晰 web 层入口。
- 已新增环境变量配置加载：`PANEL_ADDR`、`PANEL_DATA_DIR`、`PANEL_DB_PATH`、`PANEL_SECRET`、`PANEL_VERSION`。
- 已使用标准库 `log/slog` 输出基础结构化日志。
- 已实现统一 JSON 错误响应。
- 已使用 `database/sql` + `modernc.org/sqlite` 连接 SQLite。
- 已实现嵌入式 SQL 最小迁移机制。
- 已预留静态文件服务边界，当前未接入前端构建产物。

要做什么：

- HTTP API server。
- 配置加载。
- 日志。
- 错误响应格式。
- SQLite 连接。
- 数据库迁移机制。
- 静态文件服务预留。

建议实现：

- HTTP router 可用 `chi`。
- SQLite 可用标准 `database/sql` + `modernc.org/sqlite` 或 `mattn/go-sqlite3`。
- 迁移可以先用简单 SQL 文件执行，不必一开始引入复杂框架。
- 配置来源：环境变量 + 默认值。

建议配置项：

```text
PANEL_ADDR=:8090
PANEL_DATA_DIR=/data
PANEL_DB_PATH=/data/panel.db
PANEL_SECRET=
```

完成标准：

- 后端启动时自动创建 data 目录和 SQLite 数据库。
- `/health` 返回服务状态、版本、数据库可用性。
- 统一 JSON 错误结构。
- 代码中有清晰的 internal 包边界。

## Milestone 2: Storage and Auth ✅ 已完成（2026-06-22）

目标：实现面板自己的用户体系。

已完成：

- 新增 `users`、`sessions`、`audit_logs`、`panel_settings` 数据表迁移，迁移文件为 `backend/migrations/002_auth.sql`。
- 新增管理员初始化状态接口和初始化管理员接口。
- 密码使用 Argon2id 哈希保存，不保存明文密码，当前最小长度为 6 位。
- Session 使用 HttpOnly Cookie，数据库只保存 session token hash。
- 新增登录、登出、当前用户接口。
- 新增 `admin` / `user` 角色。
- 新增 admin-only 用户管理接口：列表、创建、更新、启用、禁用和真正删除。
- 新增关键操作 audit log：初始化管理员、登录、登出、用户创建、用户更新、用户禁用。
- 无 active admin 时，只允许访问 `GET /health`、`GET /api/setup/status`、`POST /api/setup/admin`。
- 前端已实现管理员初始化页、登录页、基础主界面和最小用户管理区域。

已实现 API：

```text
GET  /api/setup/status
POST /api/setup/admin
POST /api/auth/login
POST /api/auth/logout
GET  /api/auth/me
GET  /api/users
POST /api/users
PATCH /api/users/:id
DELETE /api/users/:id
```

安全和权限规则：

- 普通用户不能访问用户管理接口。
- 最后一个 active admin 不能被禁用或降级。
- 当前登录 admin 不能禁用自己。
- 不把密码、password hash、session token 或 token hash 写入日志、响应或 audit metadata。
- 所有数据库操作使用参数化 SQL。

完成标准：

- 无管理员时只能访问初始化和健康检查。
- 初始化后可以登录。
- 普通用户不能管理其他用户。
- Cookie session 可刷新页面保持登录。
- `go test ./...` 通过。
- `npm run build` 通过。

## Milestone 3: Docker / Compose Control Layer ✅ 已完成（2026-06-22）

目标：建立通用 Docker 操作层，供 GameDriver 使用。

已完成：

- 新增 `backend/internal/docker` 通用 Docker / Compose CLI 控制层。
- 封装 `docker version`、`docker compose version`、`ps`、`logs`、`pull`、`up -d`、`down`、`restart`。
- 所有命令通过 `exec.CommandContext` 和参数数组执行，不经过 shell。
- 所有命令明确工作目录、超时、输出大小上限。
- 结构化返回 stdout、stderr、exit code、duration、timeout 和输出截断状态。
- 命令参数和输出会脱敏 password、token、secret、`STEAM_PASSWORD`、`VNC_PASSWORD` 等敏感字段。
- 新增 admin-only Docker / Compose 状态 API，供当前前端基础状态区使用。
- Docker API 固定使用 `$PANEL_DATA_DIR/instances/stardew` 作为默认 Compose 工作目录，前端不能传入任意工作目录或任意命令。

后续补救说明：

- 这里写死 `$PANEL_DATA_DIR/instances/stardew` 是 Milestone 3 为了本机联调 Docker 状态而保留的临时单实例入口。
- 不需要返工 Milestone 3 的 Docker 执行层；`backend/internal/docker` 本身仍应保持通用。
- Milestone 5 必须把 API 层从“默认 Stardew 工作目录”迁移到“根据 instance_id 找 driver，再由 driver 提供工作目录或状态实现”。
- 目标 API 形态应逐步靠近 `GET /api/instances/:instance_id/docker/ps` 或 `GET /api/instances/:instance_id/status`，而不是永久保留只面向 Stardew 的 Docker API。

已实现能力：

```text
DockerVersion(ctx, workDir)
ComposeVersion(ctx, workDir)
ComposePull(ctx, dir)
ComposeUp(ctx, dir)
ComposeDown(ctx, dir)
ComposeRestart(ctx, dir)
ComposePs(ctx, dir)
ComposeLogs(ctx, dir, opts)
```

已实现 API：

```text
GET /api/docker/status
GET /api/docker/ps
GET /api/docker/logs?service=&tail=100
```

API 行为说明：

- Docker API 需要已完成管理员初始化，并且当前 session 用户角色必须是 `admin`。
- 无 active admin 时，除初始化白名单外仍返回 `setup_required`。
- 普通 `user` 访问 Docker API 返回 403。
- `GET /api/docker/status` 返回 Docker CLI 可用性、Docker Compose 可用性，以及默认 Compose 项目目录状态。
- `GET /api/docker/ps` 在 `$PANEL_DATA_DIR/instances/stardew` 执行 `docker compose ps --format json`，并解析服务名、service、state、status、health、exit code。
- `GET /api/docker/ps` 如果默认工作目录或 compose 文件不存在，返回 409 `compose_project_not_ready`。
- `GET /api/docker/logs` 返回非流式日志快照，不是 SSE/WebSocket；`tail` 默认 100，允许范围 1 到 1000。
- `GET /api/docker/logs` 的 `service` 参数可选，只允许字母、数字、点、下划线和短横线；非法时返回 400 `invalid_service`。
- Docker 命令失败返回 502 `docker_command_failed`，超时返回 504 `docker_command_timeout`，错误 details 中包含已脱敏的结构化命令结果。

完成标准：

- 后端能在指定目录执行 `docker compose ps`。
- 命令失败时能把 exit code 和错误信息记录到结构化命令结果中，后续 jobs 可直接写入 job log。
- 不存在前端任意命令执行入口。
- `go test ./...` 通过。
- `npm run build` 通过。

## Milestone 4: Jobs and State Machine ✅ 已完成（2026-06-22）

目标：让安装、认证、启动等长任务可观察、可恢复。

已完成：

- 新增 `backend/migrations/003_jobs_state.sql`。
- 新增 `jobs`、`job_logs`、`instance_state` 数据表。
- 新增 `backend/internal/storage/jobs.go`，支持创建 job、启动、成功、失败、取消标记、查询最近 jobs、查询 logs、追加 logs、恢复中断任务。
- 新增 `backend/internal/storage/instance_state.go`，支持默认 Stardew 单实例状态、状态查询、状态更新和保守状态转换校验。
- 新增 `backend/internal/jobs` 通用 Job Manager，支持异步执行、context timeout、日志追加、SSE 事件发布、panic 捕获并标记 failed。
- 后端启动时会确保默认 `stardew` instance state 存在，并把重启前遗留的 `queued/running` job 标记为 `failed`。
- 管理员初始化成功后，默认实例状态进入 `admin_created`。
- 新增登录后可读的 jobs/state API 和 admin-only 测试任务 API。
- 新增 SSE 任务日志流。
- 前端新增 Stardew 实例状态卡片、任务中心、任务详情和实时日志窗口。
- 普通 user 不能创建测试任务；admin 可查看全部任务，普通 user 只能查看自己有权限的任务。

新增表：

```text
jobs
job_logs
instance_state
```

`jobs` 关键字段：

```text
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
updated_at
```

`job_logs` 关键字段：

```text
id
job_id
level: info / warn / error / debug
message
created_at
sequence
```

`instance_state` 关键字段：

```text
instance_id
driver_id
state
state_message
last_job_id
updated_at
updated_by
```

已实现 API：

```text
GET  /api/jobs
GET  /api/jobs/:id
GET  /api/jobs/:id/logs?after=0&limit=200
GET  /api/jobs/:id/stream
POST /api/jobs/:id/cancel
POST /api/jobs/test
POST /api/jobs/test-fail
GET  /api/instances/stardew/state
```

API 行为说明：

- jobs 查询、详情、logs 和 SSE stream 都必须登录。
- admin 可以查看全部 job。
- 普通 user 只能查看 `created_by` 是自己的 job。
- 测试 job 创建必须 admin。
- `POST /api/jobs/:id/cancel` 当前返回 501 `not_implemented`，后续接真实任务取消。
- `GET /api/jobs/:id/stream` 使用 SSE，按 `job_logs.sequence` 作为事件 id；job 完成时发送 `finished` 事件并结束。
- `POST /api/jobs/test` 创建约 5 秒的模拟成功任务。
- `POST /api/jobs/test-fail` 创建模拟失败任务，最终状态为 `failed` 并保存错误原因。

核心状态仍按 architecture 文档保留：

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

补救边界：

- Milestone 4 以 `/api/instances/stardew/state` 做 Stardew 单实例联调入口。
- Milestone 5 需要新增通用形态：`GET /api/instances/:instance_id/state`。
- jobs 是通用基础设施，不应写入 Stardew 专属业务判断。
- 当前状态表暂时直接保存上述状态；Milestone 5 之后可逐步拆分通用 `state` 和 driver-specific `driver_phase` / `driver_payload`。

完成标准验证：

- 管理员登录后可以点击“启动测试任务”。
- 页面能实时看到日志追加。
- 成功测试任务最终变为 `succeeded`。
- 失败测试任务最终变为 `failed` 并显示错误原因。
- jobs、job_logs、instance_state 持久化在 SQLite，后端重启后仍可查询。
- 普通用户不能创建测试任务。
- Job Manager 没有前端任意命令执行入口。
- `go test ./...` 通过。
- `npm run build` 通过。

## Milestone 5: GameDriver Registry ✅ 已完成（2026-06-23）

目标：建立实例模型和 GameDriver registry。首版仍然是 Single Game Mode，不显示总面板，但后端已经具备未来 Multi Game Mode 的实例/driver 边界。

已完成：

- 新增配置项：`PANEL_MODE`、`DEFAULT_INSTANCE_ID`、`DEFAULT_DRIVER_ID`，默认分别为 `single`、`stardew`、`stardew_junimo`。
- 新增 `backend/migrations/004_instances.sql`，创建 `instances` 表。
- 新增 `backend/internal/storage/instances.go`，支持默认 instance 创建、查询、列表和状态更新。
- 后端启动时会确保默认 Stardew instance 存在：
  - `id = stardew`
  - `driver_id = stardew_junimo`
  - `name = Stardew Valley`
  - `data_dir = $PANEL_DATA_DIR/instances/stardew`
- 兼容旧 `instance_state` 表：新 `instances` 为空时会从旧默认状态迁移 state/state_message；旧表不删除。
- 新增 `backend/internal/games/registry`，定义完整 `GameDriver` 接口和 MVP 类型，并实现 `Register`、`Get`、`List`。
- 新增 `backend/internal/games/stardew_junimo` driver 骨架：
  - `ID() = stardew_junimo`
  - `Name() = Stardew Valley / JunimoServer`
  - `Prepare` 仅确保实例目录存在。
  - `Status` 通过通用 Docker Compose PS 能力返回基础 runtime 状态。
  - 其他安装、启动、存档、Mod、命令能力返回 `not_implemented`。
- 新增 instance-based API：

```text
GET /api/instances
GET /api/instances/:instance_id
GET /api/instances/:instance_id/state
GET /api/instances/:instance_id/status
GET /api/instances/:instance_id/docker/ps
```

- `/api/instances/:instance_id/status` 通过 `instance.driver_id -> registry.Get -> driver.Status` 获取状态。
- `/api/instances/:instance_id/docker/ps` 使用 `instance.data_dir`，不再硬编码 `$PANEL_DATA_DIR/instances/stardew`。
- 旧 `/api/docker/status`、`/api/docker/ps`、`/api/docker/logs` 保留为 admin-only 兼容/调试入口，其中默认 Compose 目录也改为读取默认 instance。
- 前端仍保持 Single Game Mode：登录后直达 Stardew 当前主界面，不显示总面板/游戏列表。
- 前端内部新增默认实例概念，状态和 Compose PS 主路径切到 `/api/instances/stardew/...`。
- 状态卡显示当前 instance 名称和 driver id。

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

权限规则：

- `GET /api/instances*` 基础查询需要登录。
- `/api/instances/:instance_id/status` 登录用户可读基础状态。
- `/api/instances/:instance_id/docker/ps` admin-only。
- 旧 `/api/docker/*` 仍 admin-only。
- 前端不允许提交任意工作目录、任意 shell 或任意 compose 参数。

保留兼容：

- `/api/instances/stardew/state` 仍可用，但现在由通用 `/api/instances/:instance_id/state` 路由处理。
- `/api/docker/*` 仍可作为开发调试入口，但产品主路径应优先使用 `/api/instances/stardew/...`。

本阶段明确未实现：

- Junimo prepare/install 的真实配置写入。
- `docker compose pull`。
- Steam Auth。
- 服务器 start/stop/restart 真实流程。
- 存档、Mod、控制台命令。
- Multi Game Mode 总面板。

完成标准验证：

```bash
cd backend
go test ./...
```

本次结果：通过。

```bash
cd frontend
npm run build
```

本次结果：通过。

后续注意事项：

- Milestone 6 开始做 Junimo 工作目录和 install 时，必须通过 `games/stardew_junimo` driver 创建 job，不要把 Stardew 业务写进 web/docker/jobs 顶层。
- `Prepare` 当前只创建目录，不代表 Junimo 配置、镜像或游戏文件已经安装。
- 真实任务写 job log 前继续脱敏 Steam 密码、VNC 密码、session token、secret。

## Milestone 6: Stardew Junimo Prepare and Install ✅ 已完成（2026-06-23）

目标：跑通 Junimo 工作目录准备和安装流程。

已完成：

- 新增 `.env` 文件管理模块 `backend/internal/games/stardew_junimo/config/env.go`，支持安全读写、合并和未知字段保留，文件权限 0600。
- 新增 `compose_template.go`，嵌入更贴近 JunimoServer 官方 `docker-compose.yml` 的模板（services: `steam-auth`、`server`、`discord-bot`；official volumes: `steam-session`、`game-data`、`saves`、`settings`；使用 `IMAGE_VERSION`；保留 `stdin_open: true` / `tty: true`）。
- `steam-auth` sidecar 已从固定 `sdvd/steam-service:${IMAGE_VERSION}` 改为 `STEAM_SERVICE_IMAGE`，默认 `anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`；`.env` 同步写入 SteamClient 连接等待和认证会话重试参数，支持本地覆盖为 `junimo-steam-service-cn:auth-retry-test` 联调 fork 镜像。
- `Prepare()` 创建实例目录（含 saves/mods/.local-container），首次写入 compose 和 .env，已有文件不覆盖（保留用户修改）。
- `Install()` 校验凭据、加载 instance、创建 `installRunner`、通过 Job Manager 启动 30 分钟超时任务。
- `installRunner.run()` 四步：检查/补齐 Junimo 工作目录（如果 `docker-compose.yml` 或 `.env` 被删会重新生成并写 job 日志）→ 写 .env（凭据写入，不记录到日志；使用 Junimo 官方 `IMAGE_VERSION`）→ 检查本地 Junimo 镜像，缺失时 `docker compose pull` → 等待面板选择 Steam 登录方式。
- 安装 job 会在日志中输出当前实例目录和实际使用的 `docker-compose.yml` 绝对路径，避免本地开发时 `data\instances\stardew` 与 `backend\data\instances\stardew` 两套历史目录混淆。
- Steam Auth 使用跨平台 TTY：Linux/macOS 通过 Docker Compose + PTY 运行，Windows 通过 Docker Engine API 创建 `Tty:true` 容器并 attach stdin/stdout，避免 Docker CLI 的本机终端检查。
- Docker TTY 输出读取器已支持没有换行结尾的交互提示，例如 `Enter Steam Guard code sent to qq.com:`；账号密码/验证码分支能在上游等待验证码时立即切到前端输入阶段，而不是等 60 秒超时后才看到同一行日志。
- `lineHandler` 检测 Steam Guard、认证方式选择、二维码提示和登录失败关键字，更新 `driver_phase`；只有看到明确登录成功关键字才进入安装完成，所有 docker 输出通过 `paneldocker.RedactString` 脱敏。
- `SendSteamGuardInput(jobID, input)` 实现 `registry.SteamGuardSender` 接口，写入活跃 job 的 guard channel。
- `registry.InstallRequest` 新增 `SteamUsername`、`SteamPassword`（never log）、`VNCPassword`（never log）字段。
- `registry.SteamGuardSender` 接口用 type assertion 模式实现可选能力。
- 新增后端路由：
  - `POST /api/instances/:id/prepare`
  - `POST /api/instances/:id/install`
  - `POST /api/instances/:id/steam-guard/input`
- 新增前端 API 函数：`prepareInstance`、`installInstance`、`submitSteamGuardInput`。
- 任务中心支持 admin 清空已结束任务记录和日志；若存在 `queued` / `running` 任务，清空接口返回 409，避免删除进行中的安装日志。
- 前端安装 UI 完整实现：
  - Prepare 按钮 / 安装游戏按钮按状态显示。
  - InstallSection 含 Modal（三个密码字段，`type=password`，不打印到控制台，不存 localStorage）。
  - 安装期间 SSE 日志实时接入 job log viewer。
  - Steam 登录方式选择区域（`auth_method_required`）：镜像检查完成后先让管理员选择扫码登录或账号密码/验证码登录。
  - Steam Guard 选择/输入区域（`steam_guard_choice_required`、`steam_guard_required` 或 `steam_guard_mobile_required`）：账号密码路径触发二次验证后，再选择手机 App 批准或输入验证码。
  - 选择扫码登录时运行 `steam-auth setup`，面板自动向上游第一层 `Choose authentication method` 输入 `2`，并展示 QR 输出。
  - Steam QR 使用独立弹窗展示，避免管理员靠滚动日志判断二维码是否完整。
  - 选择账号密码/验证码登录时运行 `steam-auth download`，让 Junimo 使用 `.env` 中的 `STEAM_USERNAME` / `STEAM_PASSWORD` 登录并下载游戏文件。不要用普通 stdin pipe 跑 `setup` 的账号密码分支；上游该分支使用 `Console.ReadKey()` 读取密码，在无 console / stdin 重定向时会崩溃。
  - 游戏文件下载阶段解析 steam-auth `Progress: done/total files ... (xx.x%)` 日志，并在安装区显示独立百分比进度条；前端百分比按文件数 `done/total` 计算（例如 `100/1470 = 6.8%`），不使用上游括号里的字节百分比。Steamworks SDK app `1007` 下载会切到 `steam_sdk_downloading`，前端单独显示 SDK 下载进度和 2 阶段下载任务进度；只要日志出现 `Downloading app 1007` / `.steam-sdk` 就立刻切到 SDK 阶段，未收到 SDK `Progress` 前显示 0% 和“正在与 Steam 下载服务器建立连接中”。Stardew 安装 job 超时已延长到 2 小时，且 SDK 已安装完成时不会再被 deadline 误判为超时。下载失败/超时/CM 网络错误后的“重试安装”会复用 `.env` 凭据并直接运行 `steam-auth download`，跳过扫码/账号密码选择；已有文件由 steam-auth 校验，已存在且校验通过的文件自动跳过。
  - Steam 二维码展示区域（`steam_qr_required`）：从任务日志提取并用等宽文本完整显示二维码输出；若上游容器在生成二维码前报 `qr_auth_failed`，明确提示改用账号密码/Steam Guard。
  - 安装任务失败后会清理前端活跃安装 job 标记，并把 failed job / error log / instance state message 转成安装区错误提示；`TryAnotherCM`、SteamClient/CM 连接失败、超时、凭据错误、二维码失败、下载失败不再只停留在任务日志里。
  - 认证失败时重显凭据 Modal。
  - 安装成功后展示"已安装"徽标和 disabled 启动按钮占位。
  - 安装期间 2.5s 轮询实例状态；若旧安装任务已超时但 instance 状态未同步，前端会按 `install_timeout` 兜底显示重试按钮。
- 安装状态读取时会通过 `stardew_junimo` driver 校验 `.local-container` 是否存在安装产物；若数据库误留 `game_installed` 但目录为空，会纠正为 `error/install_missing` 并显示重试。
- `go test ./...` 通过；`npm run build` 通过。

实例目录：

```text
/data/instances/stardew
├─ docker-compose.yml  （首次 Prepare 写入，不覆盖）
├─ .env                （首次 Prepare 写空模板；Install 写凭据；不覆盖用户已改字段）
├─ .local-container
├─ saves
└─ mods
```

已实现 API：

```text
POST /api/instances/:id/prepare
POST /api/instances/:id/install
POST /api/instances/:id/steam-guard/input
```

安全规则（已落实）：

- STEAM_PASSWORD、VNC_PASSWORD 不写入任何 job log、audit log、响应 JSON。
- 前端密码框 `type=password`，不打印到 console，不存 localStorage。
- 前端不允许传入任意 shell 命令、任意 compose 参数或任意工作目录。
- `paneldocker.RedactString` 对所有 docker 输出脱敏。

完成标准验证：

- 密码错误时状态进入 `steam_auth_failed`；前端重显凭据 Modal。
- Steam Guard 需要输入时，前端显示验证码输入框，后端写入 stdin channel。
- 认证成功后状态进入 `game_installed`。
- job log 不出现 Steam 密码和 VNC 密码。
- `go test ./...` 通过。
- `npm run build` 通过。

## Milestone 7: Server Lifecycle ✅ 已完成（2026-06-26）

目标：完成启动、停止、重启、状态和邀请码展示。

### 完成内容（2026-06-26）

**后端新增/修改文件：**
- `backend/internal/docker/compose.go`: 新增 `ComposeExecPipe`，用于向容器 stdin 管道输入（attach-cli invitecode）
- `backend/internal/games/stardew_junimo/compose_template.go`: saves 改为 bind mount (`./.local-container/saves:/config/xdg/config/StardewValley`)，删除 named volume `saves:`
- `backend/internal/games/stardew_junimo/installer.go`: 新增 `migrateSavesVolume`（迁移已有实例），在安装流程中自动执行
- `backend/internal/games/stardew_junimo/driver.go`: `Prepare` 新增 `.local-container/saves/Saves` 和 `saves-templates` 目录创建；`updatePhase` 修复为保留 `DriverPayload`
- `backend/internal/games/stardew_junimo/saves.go`: 新建，实现 `ListSaves`、`PreviewSaveZip`、`ImportSaveToVolume`、`WriteServerSettings`、`HasTemplates`；完整 ZIP 安全检查（zip-slip、绝对路径、解压大小限制）
- `backend/internal/games/stardew_junimo/lifecycle.go`: 新建，实现 `Start`/`Stop`/`Restart`/`GetInviteCode`；`LifecycleDockerService` 接口扩展 Docker 操作；`parseInviteCode` 解析 attach-cli 输出
- `backend/internal/games/registry/types.go`: 新增 `NewGameConfig`、`PreflightResult`、`UploadPreviewResult`、`InviteCodeResult`；扩展 `SaveInfo`（元数据字段）
- `backend/internal/web/lifecycle_handlers.go`: 新建，包含 `pendingUploadStore`（token 绑定实例、短 TTL、一次性）；实现全部 8 个生命周期 handler
- `backend/internal/web/handler.go`: server 新增 `pendingUploads` 字段
- `backend/internal/web/instance_handlers.go`: 注册 8 条新路由

**前端修改：**
- `frontend/src/types.ts`: 新增 `SaveInfo`、`NewGameConfig`、`PreflightResult`、`UploadPreviewResult`、`InviteCodeResult`、`LifecycleJobResponse`
- `frontend/src/api.ts`: 新增 `getSavesPreflight`、`createNewGame`、`uploadSavePreview`、`uploadSaveCommitAndStart`、`startInstance`、`stopInstance`、`restartInstance`、`getInviteCode`
- `frontend/src/App.tsx`: 新增 `LifecycleSection`、`SaveCard` 组件；新建游戏 modal（farmName/farmType/小屋/利润/宠物/金钱）；上传存档 modal（两阶段：preview→confirm）；preflight 检查流程
- `frontend/src/App.css`: 新增 lifecycle section、modal、save-card、invite-code 样式

**测试：**
- `backend/internal/games/stardew_junimo/saves_test.go`: 18 个测试（ZIP 安全、存档解析、migrateSavesVolume 幂等、ImportSaveToVolume）
- `backend/internal/games/stardew_junimo/lifecycle_test.go`: 邀请码解析、payload merge
- `backend/internal/games/stardew_junimo/driver_test.go`: 更新已有测试（bind mount 路径、新子目录）

**阻塞点（已按需求文档记录）：**
真正的自定义新建存档（FarmerName/FavoriteThing/外貌）需要预置 save template（`.local-container/saves-templates/<SaveDir>/`）。不支持 SMAPI `loadForNewGame(false)`，当前 M7 通过 `server-settings.json` 配置 Junimo 支持的字段（FarmName/FarmType/利润/小屋/宠物），Junimo 首次启动自动创建存档。以上限制已在前端 modal 提示用户。

### Milestone 7.5: 可视化新建存档创建器 ✅ 已完成（2026-06-26）

**目标：** 在 React 前端实现真实游戏素材驱动的可视化新建存档创建器；通过 SMAPI mod 机制提供农场类型、宠物品种等真实图片；角色字段（FarmerName/Gender/PetType/PetBreed/外貌）通过 server-init.json 由 SMAPI mod 在 SaveCreating 事件中应用。

**后端新增/修改文件：**
- `backend/internal/games/stardew_junimo/embedded/smapi-mod/manifest.json`: SMAPI mod 元数据（嵌入进 Go binary）
- `backend/internal/games/stardew_junimo/embedded/smapi-mod/StardewAnxiPanel.Control.dll`: 预编译 SMAPI mod（14848 字节，嵌入进 Go binary）
- `backend/internal/games/stardew_junimo/smapi_mod.go`: `//go:embed` 指令 + `installSMAPIMod()`，幂等写入 `.local-container/mods/StardewAnxiPanel.Control/`
- `backend/internal/games/stardew_junimo/catalog.go`: `PanelOptionItem`、`CatalogResponse` 类型；`ReadCatalog()`（读 options.json，有 mtime 缓存）；`DefaultCatalog()`（SVG 占位图 fallback，source="fallback"）；`InvalidateCatalogCache()`
- `backend/internal/games/stardew_junimo/compose_template.go`: server service 新增 `SAP_CONTROL_DIR=/data/control` 环境变量；新增两个 bind mount：`.local-container/control:/data/control`、`.local-container/mods/StardewAnxiPanel.Control:/data/Mods/StardewAnxiPanel.Control`
- `backend/internal/games/stardew_junimo/driver.go`: `Prepare()` 新增 `.local-container/control/`、`.local-container/control/commands/`、`.local-container/mods/` 目录创建；调用 `installSMAPIMod()`（非致命错误）
- `backend/internal/games/stardew_junimo/saves.go`: `NewGameConfig` 扩展字段（Gender/PetType/PetBreedID/Skin/Hair/Shirt/Pants/Accessory/EyeColor/HairColor/PantsColor）；`WriteInitConfig()` 写 server-init.json；`profitMargin` 改为数字字符串 ("100"|"75"|"50"|"25")；`moneyMode` 改为 "shared"|"separate"；`WriteServerSettings()` 内部调用 `WriteInitConfig()`
- `backend/internal/games/stardew_junimo/catalog_test.go`: 12 个测试（DefaultCatalog、ReadCatalog 解析/缓存/实例隔离/fallback、normalizeProfitMarginIDs）
- `backend/internal/games/stardew_junimo/saves_test.go`: 更新 profitMargin/moneyMode 测试用例为新格式
- `backend/internal/games/registry/types.go`: `RgbColor` 类型；`NewGameConfig` 扩展新角色字段
- `backend/internal/web/lifecycle_handlers.go`: `handleCustomNewGameCatalog`（GET，需认证）、`handleCustomNewGameCatalogRefresh`（POST，需管理员）
- `backend/internal/web/instance_handlers.go`: 注册 `GET/POST /api/instances/:id/custom-new-game/catalog` 路由

**前端新增/修改文件：**
- `frontend/src/games/stardew/NewGameCreator.tsx`: 可视化创建器（图片网格选农场/宠物品种、Chip 选性别/小屋/利润/资金、骨架屏加载、fallback banner、错误重试）
- `frontend/src/games/stardew/NewGameCreator.css`: 创建器样式（ImageCard、BreedCard、PetTypeCard、Chips、骨架动画、fallback banner）
- `frontend/src/types.ts`: `RgbColor`、`CatalogItem`、`CatalogResponse`；`NewGameConfig` 扩展角色字段
- `frontend/src/api.ts`: `getCustomNewGameCatalog()`、`refreshCustomNewGameCatalog()`
- `frontend/src/App.tsx`: `LifecycleSection` 替换旧内联 modal 为 `<NewGameCreator>`；引入 `defaultInstanceId`

**SMAPI Mod 机制：**
- SMAPI mod 安装在 `/data/Mods/StardewAnxiPanel.Control/`（bind mount 进容器）
- 游戏启动时 SMAPI 读取 `/data/control/options.json`（mod 写入真实游戏素材 data URL）
- 面板 Catalog API 返回 options.json 内容（四态：ready/generating/failed/unavailable）
- server-init.json 写入 `/data/control/`，SMAPI 在 SaveCreating 事件中应用完整角色定制（在 Junimo runtime 下有效）

**验证：**
- `go test ./...` 全部通过
- `npm run build` 通过（19 modules，无 TypeScript 错误）

### Milestone 7.5 续篇：Install 阶段自动导出 catalog ✅ 已完成（2026-06-26）

**目标：** Steam 安装完成后、服务器首次启动前，自动从游戏文件导出真实素材，用户打开"新建存档"应立即看到真实农场/宠物图片，无需先启动正式服务器。

**核心机制：**
- Install job 最后一步 `runCatalogExportPhase()` 启动一次性 `docker run` Junimo 容器（无端口、无 steam-auth）
- 挂载 game-data 命名卷（只读可读）+ `.local-container/control` bind mount（写）
- 轮询宿主侧 `control/options.json` 出现后 `docker stop`，最长等 10 分钟
- 锁文件 `catalog_export.lock`：存在 → status=generating；options.json 存在 → status=ready；error 文件 → status=failed

**新增文件：** `catalog_exporter.go`（`AcquireCatalogLock`/`ReleaseCatalogLock`/`ExportCatalogContent`/`WriteCatalogExportError`/`GetInstanceImageTag`）

**修改文件：** `catalog.go`（四态 CatalogResponse）、`installer.go`（新增 export 阶段）、`lifecycle_handlers.go`（POST 触发后台 goroutine）、`catalog_test.go`（更新断言）、`types.ts`（status 字段）、`NewGameCreator.tsx`（四态 UI + 5 秒轮询）、`NewGameCreator.css`（状态横幅 + spinner）

**验证：**
- `go test ./...` 全部通过
- `npm run build` 通过（19 modules，无 TypeScript 错误）

### 素材导出器启动参数修复（2026-06-26）

首次联调中，素材 exporter 仅报告 `export container exited with error: exit status 1`。修复 `backend/internal/games/stardew_junimo/catalog_exporter.go`：临时容器传入 `ALLOW_INSECURE_SETUP=true`，避免没有 `steam-auth` sidecar 时阻塞离线初始化；`SETTINGS_PATH` 修正为 Compose server 使用的 `/data/settings/server-settings.json`；同时将容器退出前未换行的 stdout/stderr 刷入 job 日志。

验证已通过：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./internal/games/stardew_junimo ./internal/web
```

尚需 Docker 联调：点击“重新生成素材”，确认没有 `steam-auth download`、没有游戏文件重新下载、没有常驻正式 server；若仍失败，优先查看任务中心末尾新增的真实容器错误，而不是只按 exit code 猜测。

### 素材导出 Compose 依赖修复（2026-06-26）

联调日志先后出现裸容器 `exit status 1` 与 10 分钟没有任何 Junimo/SMAPI 输出的超时。后者表明 `sdvd/server` 作为裸 `docker run` 容器没有获得 Junimo Compose 的 `steam-auth` 服务和网络依赖，不能到达 SMAPI `GameLaunched`。

`catalog_exporter.go` 已改用受控的：

```text
docker compose run --name sap-catalog-export-... --rm --no-ports \
  -v <instance>/.local-container/catalog-export-saves:/config/xdg/config/StardewValley \
  server
```

它复用实例 Compose 的 `server`、`steam-auth`、`game-data` 和网络；不会调用 `steam-auth download`。真实 saves 挂载会被临时目录覆盖，导出结束后删除该目录并停止 sidecar。相关 Go 单测已通过；下一次联调应在任务日志看到 `[catalog-export] 通过 Compose 启动临时 server...`，若仍超时则查看 Compose/SMAPI 的实际输出。

### Milestone 7 存档启动策略补充（2026-06-26）

本次检索了当前仓库中“自定义新建存档”和“上传存档解析”的前后端实现状态，结论如下：

- 当前代码里尚未实现可直接复用的自定义新建存档业务代码。
- 当前前端可复用的是 `frontend/src/App.tsx` 里的安装弹窗、任务日志、错误提示、状态轮询、按钮状态和 Modal 样式。
- 当前前端 API 层 `frontend/src/api.ts` 还没有 saves/new-game/upload/parse 相关函数。
- 当前类型层 `frontend/src/types.ts` 还没有自定义新建存档表单、存档解析预览、上传确认响应类型。
- 当前后端 `backend/internal/games/registry/types.go` 只有 `SaveInfo` / `UploadedFile` 占位，字段不足以表达自定义新建存档或解析后的存档数据。
- 当前 `backend/internal/games/stardew_junimo/driver.go` 中 `ListSaves` / `UploadSave` / `SelectSave` / `DeleteSave` 仍返回 `not_implemented`。
- 当前 `backend/internal/web/instance_handlers.go` 只有 state/status/docker/prepare/install/steam-guard 等路由，没有 saves 路由。
- 原型文档只描述“启动前选择存档”，不是可复用代码。

### 外部参考项目复用结论（2026-06-26）

已检索 `E:\stardew-anxi-panel` 中已完成的自定义新建存档和上传解析实现。Milestone 7 应复用其经过验证的产品与协议设计，但必须按本仓库的 `instances + GameDriver + stardew_junimo` 架构重新落位，不能直接复制其旧的 Go/Vue 路由层。

可复用的前后端设计：

- `api/internal/control/control.go` 的 `InitConfig` 是自定义农场表单的完整字段基线：玩家名、农场名、喜欢的东西、性别、宠物/品种、外观、农场类型、小屋数量/布局、资金模式、利润率和跳过开场等；其 `NormalizeInitConfig`、`ValidateInitConfig`、颜色校验和限定枚举应移植为 Stardew driver 的结构化 DTO/校验。
- `web/src/components/FarmInit.vue` 提供农场地图、性别、宠物、小屋、利润率、资金模式的成熟交互结构。新前端可按 React/现有 `App.tsx` 风格重写为启动前 Modal，不要把 Vue 组件或旧页面框架搬入本项目。
- 旧项目上传采用可靠的两阶段协议：`POST /api/saves/upload-preview` 接收受限大小的 ZIP，解压到临时目录并验证真实 Stardew 存档；服务器返回 token 和预览；`POST /api/saves/upload-commit` 才以 token 导入并选中存档。预览元数据至少含存档 ID、大小、修改时间、游戏年/季/日、农场类型/显示名、角色/已有玩家列表。此协议应以本项目的 instance 路由和短期暂存记录实现。
- ZIP 处理必须保留旧项目的安全原则：仅 ZIP、请求体和解压总量限制、拒绝路径穿越/绝对路径/符号链接、寻找并校验 Stardew 主存档文件、临时目录与最终存档目录均做路径边界检查；确认或过期后清理暂存文件。

不能误复用的部分：

- 旧项目的 `ModEntry.StartNativeCreate()` 是 SMAPI 模组内调用 Stardew 原生 `loadForNewGame(false)`/保存链路的执行器，不是普通后端写配置即可替代的能力；它在检测到 Junimo runtime 时还会跳过这条创建分支。
- `WriteJunimoSettings()` 只能映射农场、利润率、小屋、资金等 Junimo `server-settings.json` 设置，不能单独生成完整自定义角色存档。
- 所以 M7 若承诺“自定义新建存档并启动”，必须在 `stardew_junimo` driver 下接入一个经验证的真实存档生成执行器（移植/打包兼容的 SMAPI 原生创建能力，或等价的受控 helper），生成后再用 Junimo 的挂载存档目录和 `saves select --confirm`/既有加载路径选中。不得只写 `driver_payload`、metadata 或 `server-settings.json` 后就宣称已创建。

建议把 M7 的最小 API 收口到实例路由：

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

`upload-commit-and-start` 可以内部创建 lifecycle job，但确认前绝不能写正式 saves volume；自定义创建同样应由 job 记录生成、选中、启动和邀请码获取日志。`driver_payload` 只持久化 `save_strategy`、active save ID 和不敏感摘要，真实存档留在 Junimo 管理的存储中。

因此 Milestone 7 不能写成“复用现成自定义新建存档实现”，而应明确新增一个最小存档启动闭环。用户点击 `启动服务器` 时：

1. 后端先检测服务器侧是否已有可用存档。
2. 如果没有检测到已有存档，前端弹出两条路径：
   - `新建存档`：点击后打开自定义新建存档窗口。由面板前端收集农场名、玩家名、地图类型、初始设置等字段；后端校验后生成可被 Stardew/Junimo 读取的真实初始存档，并写入 `driver_payload.save_strategy=custom_new_game` 摘要。上游 Junimo 不支持完整自定义创建，所以不要把这一步写成简单调用上游 `settings newgame` 就结束，也不能只写 metadata 而不生成存档。
   - `从本机上传存档`：点击后打开上传存档界面。上传后先进入临时解析阶段，解析并完整展示游戏时间、地图、已有玩家名称、农场/角色基础信息等；用户确认无误后再点击“上传到服务器并启动”，后端才写入正式存档位置并启动服务器。
3. 如果已有存档，Milestone 7 可以先允许选择已有存档并启动，完整存档管理留给 Milestone 9。

要做什么：

- 启动前检查安装状态。
- 启动前检查存档选择状态；没有已有存档时按“自定义新建存档 / 本机上传存档并解析确认”两条路径处理。
- 执行 `docker compose up -d`。
- 停止、重启。
- 读取容器状态。
- 获取邀请码。

建议 API：

```text
POST /api/games/stardew/start
POST /api/games/stardew/stop
POST /api/games/stardew/restart
GET  /api/games/stardew/status
GET  /api/games/stardew/invite-code
GET  /api/games/stardew/logs/stream
```

怎么做：

- 如果没有完成安装，启动接口返回“请先安装游戏”。
- 如果没有选择存档，启动接口返回 `save_required`。
- 启动后通过 `docker compose ps`、HTTP status、`attach-cli` 组合判断状态。
- 邀请码优先通过 Junimo 已暴露能力获取。

完成标准：

- 未安装时不能启动。
- 未选存档时不能启动。
- 启动成功后前端显示 `running`、邀请码、玩家数。
- 停止后状态变为 `stopped`。

## Milestone 8: Frontend MVP

目标：用 React 实现 MVP 可用界面。首版上线体验是 Stardew 单面板直达，不强制显示总面板。

Milestone 8 是前端补救点：如果前面 0-4 做出的前端主界面直接等同于 Stardew 面板，这里不要强行加一个空总面板，而是调整为“Single Game Mode 直达 Stardew game module；Multi Game Mode 才显示总面板”。

页面：

- 初始化注册页。
- 登录页。
- Single Game Mode 入口：登录后直接进入 Stardew 面板。
- Stardew 游戏面板入口：内部路由建议使用 `/instances/stardew` 或 `/instances/:instance_id`。
- Multi Game Mode 总面板：预留但默认隐藏；等第二个游戏面板出现后再展示。
- 安装向导页。
- 首页/控制台页。
- 存档选择页。
- 基础日志页。

怎么做：

- 使用 React + TypeScript + Vite。
- 使用 TanStack Query 管理 API 请求。
- 使用 Zustand 或 Context 管理当前用户和实例状态。
- 使用 xterm.js 或轻量日志窗口展示安装输出。
- 预留 `frontend/src/core` 和 `frontend/src/games/stardew` 分层。
- 视觉参考 `docs/prototypes`，但先保证流程闭环，不追求一次做完全部美术。

前端迁移示例：

```text
Before:
/dashboard
  -> 直接显示 Stardew 安装、启动、存档、Mod

After:
/
  -> PANEL_MODE=single 时自动进入 Stardew 面板
  -> PANEL_MODE=multi 时显示总面板实例列表

/instances/:instance_id
  -> 根据 instance.driver_id 加载对应 game module

/instances/stardew
  -> Stardew 专属安装、Steam Guard、邀请码、存档、Mod
```

第一版只有 Stardew 一个实例时，用户不应看到多余的选择游戏页面。代码结构要像多实例/多游戏，但产品体验要像一个完整的 Stardew 面板。

关键交互：

- 没有管理员时强制进入初始化。
- 未安装时首页按钮引导到安装向导。
- 安装时展示任务日志。
- Steam Guard 出现时展示二维码/验证码输入。
- 启动按钮根据状态禁用或可用。

完成标准：

- 普通用户能完整走完登录后查看状态。
- 管理员能走完安装、认证、选择存档、启动服务器。
- 前端按钮状态与后端状态机一致。

## Milestone 9: Saves

目标：实现存档上传、已有存档读取、选择、新建和备份。

要做什么：

- 列出 Junimo 挂载目录下的存档。
- 上传存档 zip。
- 校验存档结构。
- 选择已有存档。
- 新建存档策略。
- 备份当前存档。

建议 API：

```text
GET    /api/games/stardew/saves
POST   /api/games/stardew/saves/upload
POST   /api/games/stardew/saves/select
POST   /api/games/stardew/saves/new-game
POST   /api/games/stardew/saves/backup
DELETE /api/games/stardew/saves/:name
```

怎么做：

- 文件上传必须限制大小。
- 解压时防路径穿越。
- 优先调用 Junimo `saves` 相关 CLI 能力。
- 删除和切换前做备份提示。

完成标准：

- 前端能上传一个存档并选择为启动存档。
- 切换存档后状态进入 `ready_to_start`。
- 删除存档不会误删实例目录外文件。

## Milestone 10: Mods

目标：实现 Mod 上传、删除、启用状态提示和导出。

要做什么：

- 上传 Mod zip。
- 解压到实例 mod 目录。
- 列出已安装 Mod。
- 删除 Mod。
- 导出 Mod 包。
- 标记“需要重启生效”。

建议 API：

```text
GET    /api/games/stardew/mods
POST   /api/games/stardew/mods/upload
DELETE /api/games/stardew/mods/:id
POST   /api/games/stardew/mods/export
```

怎么做：

- 上传和解压必须检查路径穿越。
- 不解析复杂 Mod 语义也可以，MVP 先管理文件。
- 修改 Mod 后设置 `restart_required`。

完成标准：

- 上传、列出、删除、导出可用。
- 修改 Mod 后前端提示需要重启。

## Milestone 11: Console and Commands

目标：提供常用命令、控制台输出和服务器喊话。

要做什么：

- 常用命令按钮。
- 自定义命令输入。
- 服务器喊话。
- 日志流。
- 权限控制。

建议命令：

```text
info
invitecode
settings show
settings validate
rendering status
host-auto
host-visibility
```

怎么做：

- 第一版只允许 allowlist 命令。
- 管理员可执行更多命令，普通用户只读或只能喊话。
- 通过 `attach-cli` 或 Junimo 暴露接口执行。

完成标准：

- 前端可执行 `info` 和 `invitecode`。
- 命令输出显示在前端。
- 未授权用户不能执行管理命令。

## Milestone 12: Packaging

目标：让用户可以拉取一个镜像运行面板。

要做什么：

- 多阶段 Dockerfile。
- 前端构建后嵌入或复制到 Go 服务。
- 镜像内包含 docker CLI 和 compose plugin。
- 数据目录 `/data`。
- 暴露 `8090`。

建议运行方式：

```bash
docker run -d \
  --name anxi-panel \
  -p 8090:8090 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v anxi-panel-data:/data \
  ghcr.io/yourname/stardew-server-anxi-panel:latest
```

完成标准：

- 新机器只需要 Docker Engine 20+ 和 Compose V2。
- 启动面板后能初始化管理员。
- 面板能创建 Junimo 实例目录并执行 compose 命令。

## Milestone 13: Hardening

目标：把 MVP 从“能用”提高到“可交付”。

要做什么：

- 操作审计。
- 错误恢复。
- 更完整权限。
- 上传安全。
- 日志脱敏。
- 备份恢复。
- 健康检查。
- 文档。

完成标准：

- 常见错误有明确 UI 提示。
- 敏感信息不会出现在 job logs、audit logs、浏览器控制台。
- 关键操作都有审计记录。
- README 能指导用户安装和排错。

## Suggested First Three Tasks

如果下一个接手者不知道从哪里开始，建议按这个顺序做：

1. 创建 Go 后端骨架和 `/health`。
2. 实现 SQLite + 管理员初始化 + 登录。
3. 实现 Docker Compose 控制层的 `ps`、`pull`、`up`、`down` 基础封装。

这三步完成后，项目就有了“面板自身可运行 + 可鉴权 + 能控制 Docker”的核心地基。

## Do Not Do Yet

这些事情不要太早做：

- 不要一开始做多游戏市场。
- 不要一开始做复杂插件系统。
- 不要一开始支持多节点。
- 不要先做大而全的 UI 组件库。
- 不要绕过 GameDriver 直接在 API 层写 Stardew 逻辑。
- 不要把未来 Minecraft / DST / Terraria / Palworld 的页面硬塞进 Stardew 面板。
- 不要为了省事允许前端执行任意 shell 命令。

## Handoff Checklist

接手前先读：

- `docs/architecture.md`
- `docs/handoff-roadmap.md`
- `docs/prototypes/stardew-anxi-panel-prototype-notes.md`

接手时先确认：

- 当前仓库是否已经创建 `backend` 和 `frontend`。
- 当前是否已有数据库迁移。
- 当前是否已有管理员初始化流程。
- 当前 Docker 命令是否只是封装层调用，而不是散落在 handler 里。
- 当前 Stardew 逻辑是否位于 `games/stardew_junimo` 下。

每完成一个 milestone 后建议更新：

- 本文档对应 milestone 的完成状态。
- README 的启动方式。
- API 文档或接口清单。
- 已知问题和下一步。
