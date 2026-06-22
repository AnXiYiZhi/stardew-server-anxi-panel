# Conversation Handoff 2026-06-22

## 本次变更

完成 Milestone 0: Repo Skeleton。

- 新增 `backend/` Go 项目骨架。
- 新增 `frontend/` React + TypeScript + Vite 项目骨架。
- 新增后端 `GET /health` 健康检查接口。
- 新增 `README.md`，记录后端和前端本地启动方式。
- 更新 `docs/handoff-roadmap.md`，标记 Milestone 0 已完成。

## 影响的接口和文件

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

新增接口：

```text
GET /health
```

返回 JSON：

```json
{
  "status": "ok",
  "service": "stardew-anxi-panel"
}
```

## 如何验证

后端：

```bash
cd backend
go test ./...
go run ./cmd/panel
```

默认监听 `:8090`，也可以通过环境变量覆盖：

```bash
PANEL_ADDR=:8091 go run ./cmd/panel
```

启动后访问：

```text
http://localhost:8090/health
```

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

- Milestone 1 应聚焦 Backend Foundation：配置加载、日志、统一错误响应、SQLite 连接、迁移机制、静态文件服务预留。
- 不要在 API handler 中直接写 Stardew 或 Junimo 具体逻辑。
- Docker / Compose 控制应放在 `backend/internal/docker`，并由后续 GameDriver 使用。
- Stardew + Junimo 相关逻辑应放在 `backend/internal/games/stardew_junimo`。
- 当前还没有用户系统、数据库初始化、Junimo 工作目录准备、Steam Auth、存档、Mod 或控制台逻辑。
