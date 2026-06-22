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

## 下一步注意事项

- Milestone 2 应聚焦 Storage and Auth：管理员初始化、登录、登出、当前用户、session、角色权限。
- Auth 表建议放到后续新的 migration，例如 `002_auth.sql`，不要改写已应用的 `001_foundation.sql`。
- 不要在 API handler 中直接写 Stardew 或 Junimo 具体逻辑。
- Docker / Compose 控制应放在 `backend/internal/docker`，并由后续 GameDriver 使用。
- Stardew + Junimo 相关逻辑应放在 `backend/internal/games/stardew_junimo`。
- 当前还没有用户系统、Junimo 工作目录准备、Docker 控制、Steam Auth、存档、Mod 或控制台逻辑。
