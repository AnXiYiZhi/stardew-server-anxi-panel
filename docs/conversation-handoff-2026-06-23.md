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
