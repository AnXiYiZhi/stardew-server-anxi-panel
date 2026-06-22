# Conversation Handoff 2026-06-22

## Milestone 0: Repo Skeleton

完成 Milestone 0: Repo Skeleton。

- 新增 `backend/` Go 项目骨架。
- 新增 `frontend/` React + TypeScript + Vite 项目骨架。
- 新增后端 `GET /health` 健康检查接口。
- 新增 `README.md`，记录后端和前端本地启动方式。
- 更新 `docs/handoff-roadmap.md`，标记 Milestone 0 已完成。

### 影响的接口和文件

后端：

- `backend/go.mod`
- `backend/cmd/panel/main.go`
- `backend/internal/web/handler.go`
- `backend/internal/auth/doc.go`
- `backend/internal/docker/doc.go`
- `backend/internal/jobs/doc.go`
- `backend/internal/games/registry/doc.go`
- `backend/internal/games/stardew_junimo/doc.go`
- `backend/internal/storage/doc.go`
- `backend/migrations/.gitkeep`

前端：

- `frontend/package.json`
- `frontend/index.html`
- `frontend/vite.config.ts`
- `frontend/tsconfig.json`
- `frontend/tsconfig.app.json`
- `frontend/tsconfig.node.json`
- `frontend/src/main.tsx`
- `frontend/src/App.tsx`
- `frontend/src/App.css`

文档：

- `README.md`
- `docs/handoff-roadmap.md`
- `docs/conversation-handoff-2026-06-22.md`

## Milestone 1: Backend Foundation

完成 Milestone 1: Backend Foundation。

- 新增后端配置加载，支持 `PANEL_ADDR`、`PANEL_DATA_DIR`、`PANEL_DB_PATH`、`PANEL_SECRET`、`PANEL_VERSION`。
- 新增 SQLite 连接初始化，启动时自动创建 data 目录和 SQLite 数据库文件。
- 新增嵌入式 SQL 最小迁移机制。
- 新增 `schema_migrations` 迁移记录表和 `panel_metadata` 最小元信息表。
- 增强 `GET /health`，返回服务状态、版本和数据库可用性。
- 新增基础结构化日志，使用 Go 标准库 `log/slog`。
- 新增统一 JSON 错误响应，未匹配路由返回 JSON 404。

### 影响的接口和文件

后端：

- `backend/go.mod`
- `backend/go.sum`
- `backend/cmd/panel/main.go`
- `backend/internal/config/config.go`
- `backend/internal/storage/doc.go`
- `backend/internal/storage/db.go`
- `backend/internal/storage/migrations.go`
- `backend/internal/web/handler.go`
- `backend/migrations/migrations.go`
- `backend/migrations/001_foundation.sql`

文档：

- `README.md`
- `docs/handoff-roadmap.md`
- `docs/conversation-handoff-2026-06-22.md`

### 接口变化

`GET /health` 成功时返回：

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

数据库不可用时返回 HTTP 503，并将 `status` 设为 `degraded`。

未知路径返回统一 JSON 错误：

```json
{
  "error": {
    "code": "not_found",
    "message": "resource not found"
  }
}
```

### 如何验证

后端：

```bash
cd backend
go test ./...
go run ./cmd/panel
```

建议本地开发时显式指定 data 目录，避免写入系统 `/data`：

```bash
PANEL_DATA_DIR=./data PANEL_DB_PATH=./data/panel.db go run ./cmd/panel
```

启动后访问：

```text
http://localhost:8090/health
```

再访问一个不存在路径，确认返回统一 JSON 404：

```text
http://localhost:8090/not-found
```

连续启动两次，确认迁移幂等，不出现 `table already exists`。

前端：

```bash
cd frontend
npm install
npm run dev
```

如只验证构建：

```bash
npm run build
```

## Milestone 2: Storage and Auth

完成 Milestone 2: Storage and Auth。

- 新增 `users`、`sessions`、`audit_logs`、`panel_settings` SQLite 表迁移。
- 新增 Argon2id 密码哈希和校验能力。
- 新增 session token 生成、hash 和 HttpOnly Cookie 设置。
- 新增管理员初始化、登录、登出、当前用户接口。
- 新增 admin/user 角色和 admin-only 用户管理接口。
- 新增 setup gate：无 active admin 时只允许健康检查、初始化状态和初始化管理员接口。
- 新增关键操作 audit log。
- 前端从占位页改为初始化页、登录页、基础主界面和最小用户管理区域。
- Vite 开发服务器新增 `/api` 和 `/health` 代理到 `http://localhost:8090`。

### 影响的接口和文件

后端：

- `backend/go.mod`
- `backend/go.sum`
- `backend/migrations/002_auth.sql`
- `backend/internal/auth/types.go`
- `backend/internal/auth/password.go`
- `backend/internal/auth/session.go`
- `backend/internal/auth/password_test.go`
- `backend/internal/storage/auth.go`
- `backend/internal/web/handler.go`
- `backend/internal/web/middleware.go`
- `backend/internal/web/auth_handlers.go`
- `backend/internal/web/users_handlers.go`
- `backend/internal/web/auth_handlers_test.go`

前端：

- `frontend/vite.config.ts`
- `frontend/src/types.ts`
- `frontend/src/api.ts`
- `frontend/src/App.tsx`
- `frontend/src/App.css`

文档：

- `README.md`
- `docs/handoff-roadmap.md`
- `docs/conversation-handoff-2026-06-22.md`

### 接口变化

新增：

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

行为说明：

- 无 active admin 时，除 `GET /health`、`GET /api/setup/status`、`POST /api/setup/admin` 外，其余接口返回 `setup_required`。
- `POST /api/setup/admin` 创建第一个管理员后会自动 Set-Cookie 并写 `setup_admin_created` audit log。
- `POST /api/auth/login` 成功后 Set-Cookie 并写 `auth_login` audit log。
- `POST /api/auth/logout` 撤销 session、清 Cookie 并写 `auth_logout` audit log。
- `/api/users` 系列接口需要 admin 角色。
- `DELETE /api/users/:id` 默认是禁用用户；`DELETE /api/users/:id?hard=true` 会真正删除用户。
- 最后一个 active admin 不能被禁用、删除或降级，当前登录 admin 不能禁用或删除自己。

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
3. 刷新页面后仍保持登录。
4. 点击登出后进入登录页。
5. 管理员重新登录后可创建普通用户。
6. 普通用户登录后不显示用户管理入口，直接请求用户管理 API 会被后端拒绝。

## 下一步注意事项

- Milestone 3 应聚焦 Docker / Compose Control Layer。
- 后续游戏管理 API 应复用当前 auth/admin guard，不要让前端绕过权限。
- 不要在 auth handler 中写 Stardew 或 Junimo 具体逻辑。
- Docker / Compose 控制应放在 `backend/internal/docker`，并由后续 GameDriver 使用。
- Stardew + Junimo 相关逻辑应放在 `backend/internal/games/stardew_junimo`。
- 不要把密码、Steam 密码、VNC 密码、session token 或 token hash 写入日志、响应、job log 或 audit metadata。
- 当前仍未实现 Junimo 工作目录准备、Docker 控制、Steam Auth、存档、Mod 或控制台逻辑。

## Milestone 2 Follow-up: Auth UI Feedback Fixes

根据用户反馈补充修复：

- 后端密码最小长度从 10 位调整为 6 位，并返回中文错误提示。
- 前端管理员初始化、登录、创建用户密码框增加“显示/隐藏”按钮。
- 前端提示词和用户状态文案改为中文显示。
- 创建用户表单提交后重置为空，并通过独立字段名和 `autocomplete` 避免浏览器把管理员账号密码自动填入创建用户栏。
- 用户管理中禁用后会显示“启用”按钮，可重新启用用户。
- 用户管理中新增“删除”按钮，调用 `DELETE /api/users/:id?hard=true` 执行真正删除。

验证：

```bash
go test ./...
npm run build
```

本次运行结果：后端测试通过，前端构建通过。

## Milestone 3: Docker / Compose Control Layer

完成 Milestone 3: Docker / Compose Control Layer。

- 新增 `backend/internal/docker` 通用 Docker / Compose CLI 控制层。
- 新增固定 allowlist 方法：Docker version、Compose version、Compose ps、Compose logs、Compose pull、Compose up/down/restart。
- Docker 命令通过 `exec.CommandContext` 和参数数组执行，不经过 shell。
- 命令执行统一记录工作目录、参数、stdout、stderr、exit code、duration、timeout 和输出截断状态。
- 命令输出和参数会脱敏 password、token、secret、`STEAM_PASSWORD`、`VNC_PASSWORD` 等敏感字段。
- 新增 admin-only Docker API：状态检查、Compose ps、Compose logs 快照。
- 前端登录后 admin 可查看 Docker / Compose 状态、Compose ps 和 logs 快照；普通 user 不显示入口且后端拒绝访问。

### 影响的接口和文件

后端：

- `backend/internal/docker/types.go`
- `backend/internal/docker/runner.go`
- `backend/internal/docker/compose.go`
- `backend/internal/docker/redact.go`
- `backend/internal/docker/compose_test.go`
- `backend/internal/docker/redact_test.go`
- `backend/internal/web/handler.go`
- `backend/internal/web/docker_handlers.go`
- `backend/internal/web/docker_handlers_test.go`
- `backend/cmd/panel/main.go`

前端：

- `frontend/src/types.ts`
- `frontend/src/api.ts`
- `frontend/src/App.tsx`
- `frontend/src/App.css`

文档：

- `README.md`
- `docs/handoff-roadmap.md`
- `docs/conversation-handoff-2026-06-22.md`

### 接口变化

新增：

```text
GET /api/docker/status
GET /api/docker/ps
GET /api/docker/logs?service=&tail=100
```

行为说明：

- Docker API 需要已初始化管理员，并且当前 session 用户角色必须为 `admin`。
- 普通 `user` 访问 Docker API 返回 403。
- 默认 Compose 工作目录固定为 `$PANEL_DATA_DIR/instances/stardew`，前端不能传入任意工作目录。
- `GET /api/docker/status` 返回 Docker CLI 可用性、Compose 可用性和默认 Compose 项目目录状态。
- `GET /api/docker/ps` 执行 `docker compose ps --format json` 并解析服务列表；默认工作目录或 compose 文件不存在时返回 409 `compose_project_not_ready`。
- `GET /api/docker/logs` 返回非流式日志快照，`tail` 默认 100，最大 1000；`service` 只允许字母、数字、点、下划线和短横线。
- Docker 命令失败返回 502 `docker_command_failed`；超时返回 504 `docker_command_timeout`；错误 details 中包含已脱敏的结构化命令结果。

### 如何验证

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

验证流程：

1. 管理员登录后可点击“检查 Docker”查看 Docker 和 Compose 状态。
2. 如果 `./data/instances/stardew` 不存在 compose 文件，点击“查看 Compose PS”应显示“Compose 工作目录尚未准备”的明确提示。
3. 如手动准备 compose 文件，`GET /api/docker/ps` 应返回结构化 services。
4. `GET /api/docker/logs?tail=100` 返回非流式 logs 快照。
5. 普通 user 登录后不显示 Docker 状态区域，直接请求 `/api/docker/status` 应返回 403。
6. 输出中出现 password、token、secret、`STEAM_PASSWORD` 或 `VNC_PASSWORD` 时应被替换为 `[REDACTED]`。

### 下一步注意事项

- Milestone 4 应把 Docker 命令结果接入 jobs 表和 job logs；当前 Milestone 3 只返回结构化结果，尚未持久化 job log。
- 当前 logs API 是快照，不是流式；实时日志应在 Milestone 4 用 SSE 或 WebSocket 实现。
- 当前尚未自动创建 Junimo 工作目录，也不会自动拉镜像或启动容器；这些应由后续 `games/stardew_junimo` driver 编排。
- 后续 API 层不要直接拼接 Docker 命令，继续通过 `backend/internal/docker` 的固定方法调用。
- 后续不要把 Stardew 存档、Mod、控制台逻辑写进顶层 Docker 或 Web handler；应放进 `games/stardew_junimo` driver。

## Architecture Correction: Global Panel + Game Panels

根据产品方向补充修正文档：

- 长期产品不是“一个 Stardew 页面兼容所有游戏”，而是“总面板 + 多个游戏专属面板”。
- 总面板负责登录、用户、权限、所有游戏实例列表、全局 Docker 状态、全局任务中心和审计。
- 点击游戏实例后进入该游戏自己的 game panel。
- 后端每个游戏有自己的 `GameDriver`，例如 `stardew_junimo`、`minecraft`、`dont_starve_together`、`terraria`、`palworld`。
- 前端每个游戏也应有自己的 game module，例如 `frontend/src/games/stardew`、`frontend/src/games/minecraft`。
- Stardew 的 Steam Guard、邀请码、农场设置、小屋管理等只属于 Stardew game module。
- Minecraft 的 RCON、白名单、OP、世界管理等未来应属于 Minecraft game module。
- `auth`、`jobs`、`docker`、`storage`、`web` 是通用基础设施，不应该出现 Stardew 或 Minecraft 专属业务判断。
- API handler 应通过 `games/registry` 找到当前实例的 driver，不要堆 `if game == ...` 分支。

已同步更新：

- `README.md`
- `README.en.md`
- `docs/architecture.md`
- `docs/handoff-roadmap.md`
- `docs/prototypes/stardew-anxi-panel-prototype-notes.md`

当前进度记录：

- Milestone 0、1、2、3 已完成。
- Milestone 4: Jobs and State Machine 正在进行。

## Product Display Correction: Single Game Mode First

最新产品决策：

- 首个可上线版本只显示 Stardew 面板。
- 登录后直接进入 Stardew game module。
- 不提前展示总面板游戏列表。
- 内部仍保留 `instances + driver_id + GameDriver` 架构。
- 等开发第二个游戏面板时，再开启 Multi Game Mode 并显示总面板。

建议配置：

```text
PANEL_MODE=single
DEFAULT_INSTANCE_ID=stardew
DEFAULT_DRIVER_ID=stardew_junimo
```

路由策略：

```text
PANEL_MODE=single:
  / -> /instances/stardew

PANEL_MODE=multi:
  / -> 总面板游戏实例列表
  /instances/stardew -> Stardew 面板
  /instances/minecraft -> Minecraft 面板
```

对 Milestone 的影响：

- Milestone 5 仍然要做 `instances`、`driver_id`、`GameDriver registry`。
- Milestone 8 不要强制显示总面板，而是实现 Single Game Mode 直达 Stardew。
- Multi Game Mode 可以预留，但默认隐藏。

## Remediation Plan for Milestones 0-4

结论：当前按 Milestone 0、1、2、3、4 的实现方向不需要推翻。

原因：

- Milestone 0 项目骨架是通用基础。
- Milestone 1 配置、SQLite、迁移、错误响应是通用基础。
- Milestone 2 用户、登录、权限是总面板基础。
- Milestone 3 Docker / Compose 执行层是通用基础。
- Milestone 4 jobs、job logs、状态机、日志流是通用基础。

需要后续补救的点：

- Milestone 3 当前 Docker API 默认使用 `$PANEL_DATA_DIR/instances/stardew`，这是临时单实例联调入口。
- Milestone 4 如果已经使用 `/api/instances/stardew/state` 或 Stardew 专属状态，也可以暂留。
- Milestone 5 必须新增/完善 `instances` 模型和 `driver_id`，让 API 根据 `instance_id` 找到 driver，而不是永久写死 Stardew。
- Milestone 5 需要把 `GET /api/docker/ps` 这类开发调试接口迁移到产品主路径：`GET /api/instances/:instance_id/status` 或 `GET /api/instances/:instance_id/docker/ps`。
- Milestone 4 的状态应逐步拆分为通用 `state` 和 driver-specific `driver_phase` / `driver_payload`。
- Milestone 8 需要把前端从“直接写死 Stardew 管理页”调整为“Single Game Mode 直达 Stardew game module；Multi Game Mode 才显示总面板实例列表”。

建议目标：

```text
instances
  id
  driver_id
  name
  data_dir
  state
  driver_phase
  driver_payload
```

```text
GET /api/instances
GET /api/instances/:instance_id
GET /api/instances/:instance_id/state
GET /api/instances/:instance_id/status
POST /api/instances/:instance_id/actions/start
POST /api/instances/:instance_id/actions/stop
```

当前 0-4 的成果保留；Milestone 5 和 Milestone 8 是主要修正节点。

## Milestone 4: Jobs and State Machine

完成 Milestone 4: Jobs and State Machine。

- 新增 `jobs`、`job_logs`、`instance_state` SQLite 表迁移。
- 新增通用 Job Manager，支持创建 job、异步执行、追加日志、成功/失败标记、panic 捕获和启动恢复。
- 新增 SSE 任务日志流，前端可实时看到新增 job logs。
- 新增 Stardew 单实例状态存储和查询接口。
- 新增 admin-only 测试任务 API：成功测试任务和失败测试任务。
- 前端新增 Stardew 实例状态卡片、任务中心、任务详情和日志窗口。
- 普通用户不能创建测试任务；admin 可看全部任务，普通用户只能看自己有权限的任务。

### 影响的接口和文件

后端：

- `backend/migrations/003_jobs_state.sql`
- `backend/internal/storage/jobs.go`
- `backend/internal/storage/instance_state.go`
- `backend/internal/jobs/types.go`
- `backend/internal/jobs/manager.go`
- `backend/internal/web/jobs_handlers.go`
- `backend/internal/web/instance_handlers.go`
- `backend/internal/web/handler.go`
- `backend/internal/web/auth_handlers.go`
- `backend/cmd/panel/main.go`
- `backend/internal/storage/jobs_test.go`
- `backend/internal/storage/instance_state_test.go`
- `backend/internal/jobs/manager_test.go`
- `backend/internal/web/jobs_handlers_test.go`

前端：

- `frontend/src/types.ts`
- `frontend/src/api.ts`
- `frontend/src/App.tsx`
- `frontend/src/App.css`

文档：

- `README.md`
- `docs/handoff-roadmap.md`
- `docs/conversation-handoff-2026-06-22.md`

### 接口变化

新增：

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

行为说明：

- jobs 查询、详情、logs 和 stream 都要求已登录。
- admin 可查看全部 job。
- 普通 user 只能查看 `created_by` 为自己的 job。
- 测试 job 创建必须 admin。
- `GET /api/jobs/:id/stream` 使用 SSE，日志事件为 `event: log`，完成事件为 `event: finished`。
- `POST /api/jobs/test` 创建约 5 秒的模拟成功任务，每秒写入日志，最终 `succeeded`。
- `POST /api/jobs/test-fail` 创建模拟失败任务，写入日志后最终 `failed`，并保存 `error_message`。
- `POST /api/jobs/:id/cancel` 当前返回 501 `not_implemented`，后续真实长期任务接入时再完善取消。
- `GET /api/instances/stardew/state` 返回默认实例 `stardew` 的 `driver_id`、`state`、`state_message` 和更新时间。

### 数据库变化

新增迁移：`backend/migrations/003_jobs_state.sql`。

新增表：

```text
jobs
job_logs
instance_state
```

当前 `instance_state` 默认使用：

```text
instance_id = stardew
driver_id = stardew_junimo
```

后端启动会确保默认实例状态存在：

- 无 active admin 时为 `uninitialized`。
- 有 active admin 且无状态行时为 `admin_created`。
- 已有状态行不会覆盖。

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

1. 用管理员登录；新库则先创建管理员。
2. 登录后查看“Stardew 实例状态”，应能显示 `admin_created` 或当前数据库保存的状态。
3. 点击“启动测试任务”。
4. 点击新任务，确认日志窗口每秒追加日志，最终状态变为 `succeeded`。
5. 点击“启动失败测试任务”，确认任务最终为 `failed`，并显示错误原因。
6. 创建普通用户并登录，确认普通用户看不到测试任务按钮。
7. 普通用户直接请求 `POST /api/jobs/test` 应返回 403。
8. 重启后端后，历史 jobs、job logs 和 `stardew` instance state 仍可查询。

### 下一步注意事项

- Milestone 5 应把当前 `/api/instances/stardew/state` 收口为通用 `/api/instances/:instance_id/state`。
- Milestone 5 应新增/完善 `instances` 模型和 GameDriver registry，减少 API 层直接写死 Stardew。
- 当前测试 job 只用于联调 jobs/SSE，不代表真实 Junimo 业务。
- 后续 Junimo 安装、Steam Auth、服务器启动都应通过 Job Manager 创建 job 并写 job_logs。
- 真实任务写日志前必须继续脱敏，不要输出 Steam 密码、VNC 密码、session token、secret。
- 当前 cancel API 明确返回 not implemented；后续接入真实长期任务时再连接 Manager cancel map 和 runner context。
