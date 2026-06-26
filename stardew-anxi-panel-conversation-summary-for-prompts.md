# stardew-server-anxi-panel Conversation Summary For Future Prompts

本文档总结本轮关于 `stardew-server-anxi-panel` 的关键产品、架构、里程碑和提示词约束，方便后续继续询问 Milestone 8、9、10、11、12、13、14 的提示词时保持一致。

## Project Identity

项目名：

```text
stardew-server-anxi-panel
```

当前项目目录：

```text
E:\stardew-server-anxi-panel
```

JunimoServer 参考源码：

```text
E:\源码\JunimoServer_源码
```

EMP 参考源码：

```text
E:\源码\emp_源码
```

项目近期目标：

```text
做一个基于 JunimoServer 的 Stardew Valley 专用服务器 Web 管理面板。
```

长期目标：

```text
演进成一个多游戏开服面板。
但首版上线时只显示 Stardew 面板，不提前显示总面板/游戏列表。
```

## Current Product Decision

最终确定的产品模式是：

```text
Single Game Mode now
Multi Game Mode later
```

也就是说：

- 当前上线版本只显示 Stardew 面板。
- 用户登录后直接进入 Stardew 管理界面。
- 不显示总面板。
- 不显示游戏列表。
- 不让用户多点一次“选择 Stardew”。
- 内部代码仍然按多游戏架构预留。
- 等第二个游戏面板，例如 Minecraft，开始开发时再开启 Multi Game Mode。

建议配置：

```text
PANEL_MODE=single
DEFAULT_INSTANCE_ID=stardew
DEFAULT_DRIVER_ID=stardew_junimo
```

Single Game Mode 路由：

```text
/ -> /instances/stardew
/instances/stardew -> Stardew 面板
```

Multi Game Mode 路由，未来才启用：

```text
/ -> 总面板游戏实例列表
/instances/stardew -> Stardew 面板
/instances/minecraft -> Minecraft 面板
```

推荐规则：

```text
if PANEL_MODE == single and only one instance:
    登录后直接进入默认实例面板

if PANEL_MODE == multi or instances > 1:
    登录后进入总面板实例列表
```

## Core Stack

已确定技术栈：

```text
Backend: Go
Frontend: React + TypeScript + Vite
Database: SQLite
Runtime control: Docker Socket + Docker Compose V2
Architecture: GameDriver-style abstraction
First driver: games/stardew_junimo
```

## Architecture Boundary

后端分层原则：

```text
auth      -> 面板用户、初始化注册、登录、权限
storage   -> SQLite、迁移、持久化模型
docker    -> 通用 Docker / Compose allowlist 控制层
jobs      -> 长任务、job logs、SSE/WebSocket 日志流
games     -> 游戏 driver registry
web       -> HTTP API
```

Stardew / Junimo 专属逻辑必须放在：

```text
backend/internal/games/stardew_junimo
```

不要把 Stardew 专属逻辑写进：

```text
auth
docker
jobs
storage
web handler
```

前端也要分层：

```text
frontend/src/core
frontend/src/games/stardew
frontend/src/games/minecraft       # future
frontend/src/games/dst             # future
frontend/src/games/terraria        # future
frontend/src/games/palworld        # future
```

首版可以只有：

```text
frontend/src/games/stardew
```

但不要把未来 Minecraft、DST、Terraria、Palworld 的页面逻辑塞进 Stardew 模块。

## GameDriver Meaning

`GameDriver` 不是要求所有游戏共用同一个页面，也不是要求所有游戏使用同一种容器命令。

它的意义是：

```text
总面板和通用后端只知道当前 instance 使用哪个 driver。
具体游戏如何安装、启动、停止、读取状态、管理存档、管理 Mod、执行命令，由对应 driver 自己负责。
```

Stardew driver：

```text
games/stardew_junimo
```

未来可能新增：

```text
games/minecraft
games/dont_starve_together
games/terraria
games/palworld
```

GameDriver 接口方向：

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

## Instance Model

Milestone 5 后应有 instance 概念。

默认 Stardew instance：

```text
id: stardew
driver_id: stardew_junimo
name: Stardew Valley
data_dir: $PANEL_DATA_DIR/instances/stardew
```

建议 instance 表字段：

```text
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

状态分层：

```text
state          通用状态，例如 installed / starting / running / stopped / error
driver_phase   游戏 driver 自己的阶段，例如 steam_auth_running / save_required
driver_payload driver 自己的 JSON 元数据，例如 invite_code / save_strategy
```

## JunimoServer Official Flow

用户最初提供的 Junimo 官方流程：

```bash
mkdir junimoserver && cd junimoserver
curl -O https://raw.githubusercontent.com/stardew-valley-dedicated-server/server/master/docker-compose.yml
curl -O https://raw.githubusercontent.com/stardew-valley-dedicated-server/server/master/.env.example
mv .env.example .env
```

配置：

```text
STEAM_USERNAME="your_steam_username"
STEAM_PASSWORD="your_steam_password"
VNC_PASSWORD="your_secure_password"
```

拉取镜像：

```bash
docker compose pull
```

首次 Steam 认证：

```bash
docker compose run --rm -it steam-auth setup
```

启动服务器：

```bash
docker compose up -d
```

获取邀请码：

```bash
docker compose exec server attach-cli
invitecode
```

状态和管理命令：

```text
info
settings show
settings validate
settings newgame
settings newgame --confirm
saves
saves info <name>
saves select <name>
saves select <name> --confirm
rendering <fps>
rendering status
invitecode
host-auto
host-visibility
```

面板职责是把这些命令封装为可视化、可恢复、可审计的 Web 工作流。

## Port Decision

曾讨论过 Junimo 源码里 `8090` 的问题。后来用户要求把相关记忆视为未发生，并恢复之前端口状态。

当前文档和代码继续以已有项目状态为准，不在提示词里强行改端口。

如果未来要重新讨论端口，应先检查：

```text
E:\源码\JunimoServer_源码\docker-compose.yml
```

但不要在后续 Milestone 提示词里主动改端口，除非用户重新要求。

## Milestone Status

根据对话，当前进度：

```text
Milestone 0 已完成
Milestone 1 已完成
Milestone 2 已完成
Milestone 3 已完成
Milestone 4 已完成
Milestone 5 已完成
Milestone 6 已完成
Milestone 7 的提示词已给出
```

用户现在后续可能会问：

```text
8 的提示词怎么写
9 的提示词怎么写
10 的提示词怎么写
11 的提示词怎么写
12 的提示词怎么写
13 的提示词怎么写
14 的提示词怎么写
```

回答时必须保持本文档中的产品模式和架构边界。

## Completed Milestones Summary

### Milestone 0: Repo Skeleton

目标：

```text
建立 backend / frontend / docs 基础骨架。
```

已完成能力：

- Go backend skeleton。
- React + TypeScript + Vite frontend skeleton。
- 基础 `/health`。
- 初始文档。

### Milestone 1: Backend Foundation

目标：

```text
后端基础能力。
```

已完成能力：

- 配置加载。
- SQLite 初始化。
- 迁移机制。
- 统一 JSON 错误响应。
- 增强 `/health`。
- 基础日志。

### Milestone 2: Storage and Auth

目标：

```text
面板用户体系。
```

已完成能力：

- 管理员初始化。
- 登录。
- 登出。
- 当前用户。
- admin / user 角色。
- HttpOnly Cookie session。
- Argon2id 密码哈希。
- 用户管理。

### Milestone 3: Docker / Compose Control Layer

目标：

```text
通用 Docker / Compose allowlist 控制层。
```

已完成能力：

- Docker version。
- Compose version。
- Compose ps。
- Compose logs snapshot。
- Compose pull/up/down/restart 封装。
- 命令超时。
- 结构化结果。
- 敏感输出脱敏。
- admin-only Docker API。

临时点：

```text
可能写死 $PANEL_DATA_DIR/instances/stardew
这是 0-4 阶段允许存在的临时单实例联调入口。
Milestone 5 已要求收口进 instances + driver_id。
```

### Milestone 4: Jobs and State Machine

目标：

```text
通用长任务、job logs、状态机和日志流。
```

应完成能力：

- jobs 表。
- job_logs 表。
- instance_state / server_state。
- SSE 或 WebSocket 日志流。
- 测试任务。
- 失败任务。
- job 历史。

注意：

```text
jobs 是通用基础设施。
不要写 Stardew 专属业务。
Stardew 专属阶段放 driver_phase / driver_payload。
```

### Milestone 5: GameDriver Registry + Instance Model

目标：

```text
把临时 Stardew 单实例路径收口进 instances + driver_id + GameDriver registry。
```

应完成能力：

- `instances` 表。
- `PANEL_MODE=single`。
- `DEFAULT_INSTANCE_ID=stardew`。
- `DEFAULT_DRIVER_ID=stardew_junimo`。
- 自动创建默认 Stardew instance。
- GameDriver 接口。
- games registry。
- stardew_junimo driver skeleton。
- instance-based API。

重要：

```text
仍然不显示总面板。
仍然 Single Game Mode。
登录后直接进入 Stardew 面板。
```

### Milestone 6: Stardew Junimo Prepare and Install

目标：

```text
在 games/stardew_junimo driver 内实现 Junimo prepare / install / steam-auth。
```

应完成能力：

- 创建 `$PANEL_DATA_DIR/instances/stardew`。
- 写入或下载 `docker-compose.yml`。
- 写入 `.env`。
- 用户输入 Steam username / password / VNC password。
- 写入 `.env`，不泄漏密码。
- `docker compose pull`。
- PTY 执行 `docker compose run --rm -it steam-auth setup`。
- Steam Guard 输出展示到前端。
- Steam Guard input 写回 PTY stdin。
- 密码错误可重试。
- 安装成功后更新 `state / driver_phase`。

本阶段不做：

- 不启动服务器。
- 不获取邀请码。
- 不做存档管理。
- 不做 Mod 管理。
- 不显示总面板。

### Milestone 7: Server Lifecycle

目标：

```text
在 games/stardew_junimo driver 内实现 start / stop / restart / status / invite-code。
```

提示词已给出，应要求：

- 启动前检查安装状态。
- 未安装提示 `请先安装游戏`。
- 启动前要求最小存档策略，至少支持 `new_game`。
- `docker compose up -d`。
- `docker compose down`。
- `docker compose restart` 或 down + up。
- `docker compose ps` 查询状态。
- 启动后通过 `attach-cli` 固定发送 `invitecode` / `info`。
- 保存并展示邀请码。
- lifecycle job logs。
- 前端显示启动、停止、重启、刷新状态、刷新邀请码、复制邀请码。

本阶段不做：

- 完整存档管理。
- 完整 Mod 管理。
- 任意控制台。
- 总面板。

## Future Milestone Direction

下面是后续 8-14 提示词应该遵守的方向。

## Milestone 7 Save-Start Clarification

在 2026-06-26 重新检索当前仓库后，确认当前项目里尚未实现可直接复用的“自定义新建存档”或“上传存档解析预览”业务代码：

- `frontend/src/App.tsx` 可复用安装 Modal、任务日志、SSE、错误展示和按钮状态结构。
- `frontend/src/api.ts` 尚无 saves/new-game/upload/parse/confirm API。
- `frontend/src/types.ts` 尚无自定义新建存档表单或存档解析预览类型。
- `backend/internal/games/registry/types.go` 的 `SaveInfo` / `UploadedFile` 仍是占位。
- `backend/internal/games/stardew_junimo/driver.go` 的 saves 方法仍是 `not_implemented`。
- `backend/internal/web/instance_handlers.go` 尚无 saves 相关路由。

因此 Milestone 7 提示词必须明确：点击启动服务器时，如果未检测到已有存档，弹出两个按钮：

```text
新建存档
从本机上传存档
```

`新建存档` 需要弹出自定义新建存档窗口，由面板前后端收集和保存农场名、玩家名、地图类型、初始设置等字段，并生成可被 Stardew/Junimo 读取的真实初始存档；上游 Junimo 不支持完整自定义创建，不能假设调用上游即可完成，`driver_payload` 也不能替代真实存档。

`从本机上传存档` 需要先上传到临时区并解析，展示游戏时间、地图、已有玩家名称、农场/角色基础信息等；用户确认无误后再点击“上传到服务器并启动”。

完整存档管理、删除、备份、切换仍放到 Milestone 9，Milestone 7 只做启动前所需的最小闭环。

### Milestone 8: Frontend MVP

核心目标：

```text
把前端做成可用的 Stardew Single Game Mode MVP。
```

必须保持：

- 登录后直接进入 Stardew 面板。
- 不显示总面板。
- 不显示游戏列表。
- 内部使用 `/instances/stardew` 或 `/api/instances/stardew/...`。
- UI 风格贴近 Stardew。
- 不要提前做 Minecraft / DST / Terraria / Palworld 页面。

应覆盖页面：

- 初始化注册。
- 登录。
- Stardew 面板主页。
- 安装向导。
- Steam Guard 交互。
- 启动/停止/重启。
- 邀请码展示。
- 基础状态。
- lifecycle job logs。
- 基础设置入口。

提示词里要强调：

```text
Milestone 8 是前端 MVP，不是 Multi Game Mode。
总面板入口可预留，但默认隐藏。
```

### Milestone 9: Saves

核心目标：

```text
在 stardew_junimo driver 内实现 Stardew 存档管理。
```

应包含：

- 上传存档。
- 读取已有存档。
- 选择存档。
- 新建存档策略。
- 删除存档。
- 备份存档。
- 校验 zip / folder。
- 防路径穿越。
- 优先使用 Junimo `saves` CLI。
- 必要时管理挂载目录。

不要做：

- Minecraft 世界管理。
- 通用多游戏存档市场。
- 任意 shell。

### Milestone 10: Mods

核心目标：

```text
在 stardew_junimo driver 内实现 Stardew Mod 管理。
```

应包含：

- 上传 Mod zip。
- 解压到 Mods 目录。
- 列出 Mod。
- 删除 Mod。
- 导出 Mod 包。
- 修改后提示需要重启。
- 文件大小限制。
- 防路径穿越。

不要做：

- Minecraft 插件。
- tModLoader。
- Palworld mods。

### Milestone 11: Console and Commands

核心目标：

```text
在 stardew_junimo driver 内实现 allowlist 控制台命令和喊话。
```

应包含：

- 常用命令按钮。
- `info`。
- `invitecode`。
- `settings show`。
- `settings validate`。
- `rendering status`。
- `host-auto`。
- `host-visibility`。
- 服务器喊话。
- 命令输出。
- 权限控制。

禁止：

- 前端提交任意 shell。
- 前端提交任意 compose command。
- 未授权用户执行管理命令。

### Milestone 12: Packaging

核心目标：

```text
打包成用户可直接 docker run 的单镜像面板。
```

应包含：

- 多阶段 Dockerfile。
- 前端 build 产物嵌入或复制到 Go 服务。
- 镜像内包含 docker CLI。
- 镜像内包含 docker compose plugin。
- `/data` 数据目录。
- Docker Socket 挂载说明。
- Single Game Mode 默认配置。
- 健康检查。
- README 安装命令。

运行方式示例：

```bash
docker run -d \
  --name anxi-panel \
  -p 8090:8090 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v anxi-panel-data:/data \
  ghcr.io/yourname/stardew-server-anxi-panel:latest
```

注意：

```text
不要默认显示总面板。
启动后仍然直达 Stardew 面板。
```

### Milestone 13: Hardening

核心目标：

```text
把 MVP 从能用提升到可交付。
```

应包含：

- 操作审计。
- 错误恢复。
- 权限完善。
- 上传安全。
- 日志脱敏。
- 备份恢复。
- 健康检查。
- 配置校验。
- 失败任务重试。
- 数据库备份。
- 安全文档。

重点：

- Docker Socket 高权限提示。
- Steam 密码、VNC 密码、session token 绝不进日志。
- `.env` 权限收紧。
- 上传 zip 防路径穿越。
- 控制台命令 allowlist。

### Milestone 14: Multi Game Mode Prep Or Release Polish

Milestone 14 尚未在早期路线里明确固定。后续如果用户问 14，可以按两种方向之一生成提示词，先判断用户意图。

方向 A：Release Polish

```text
完善首版 Stardew 面板上线体验。
```

可能包括：

- 安装文档。
- 故障排查。
- UI polish。
- 首次启动向导优化。
- Docker 镜像发布。
- GitHub Actions。
- 版本号。
- changelog。

方向 B：Multi Game Mode Preparation

```text
为第二个游戏面板做总面板准备。
```

可能包括：

- `PANEL_MODE=multi`。
- 显示总面板游戏实例列表。
- game module registry。
- 创建第二个 dummy driver。
- 创建 Minecraft driver skeleton。
- 不实现 Minecraft 完整功能，只打通架构。

但除非用户明确说要做第二个游戏，否则 14 更建议作为 Release Polish。

## Prompt Style Rules For Future Answers

以后用户问：

```text
8的提示词怎么写
9的提示词怎么写
10的提示词怎么写
11的提示词怎么写
12的提示词怎么写
13的提示词怎么写
14的提示词怎么写
```

回答应遵守：

1. 用中文。
2. 给一段可直接复制给 Codex / Claude 的完整提示词。
3. 开头要求读取：

```text
E:\stardew-server-anxi-panel\docs\architecture.md
E:\stardew-server-anxi-panel\docs\handoff-roadmap.md
E:\stardew-server-anxi-panel\README.md
最新 conversation-handoff 文档
```

4. 明确当前已完成到前一个 Milestone。
5. 明确当前仍是 Single Game Mode。
6. 明确不要显示总面板，除非用户进入 Multi Game Mode 阶段。
7. 明确 Stardew 逻辑必须在 `games/stardew_junimo` driver 内。
8. 明确不要前端任意 shell。
9. 明确不要泄漏 Steam 密码、VNC 密码、session token。
10. 每个提示词都包含：

```text
目标
后端要求
前端要求
权限要求
安全要求
测试要求
完成标准
完成后更新文档
本机测试说明
防跑偏提醒
```

## Current Documentation Files

项目中已有重要文档：

```text
README.md
README.en.md
docs/architecture.md
docs/handoff-roadmap.md
docs/conversation-handoff-2026-06-22.md
docs/prototypes/stardew-anxi-panel-product-prototype.html
docs/prototypes/stardew-anxi-panel-product-prototype.png
docs/prototypes/stardew-anxi-panel-prototype-notes.md
```

本摘要文档应复制到：

```text
E:\stardew-anxi-panel-conversation-summary-for-prompts.md
```

## Final Guardrails

最重要的几条不要变：

```text
首版上线只显示 Stardew 面板。
不要提前显示总面板。
内部架构仍然按 instances + driver_id + GameDriver。
Stardew 逻辑必须进 games/stardew_junimo。
前端 Stardew 页面必须进 games/stardew module。
后续第二个游戏出现后再启用 Multi Game Mode。
```

