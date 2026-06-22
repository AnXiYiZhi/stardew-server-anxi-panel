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
