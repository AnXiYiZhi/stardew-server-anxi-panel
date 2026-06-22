# Stardew Anxi Panel

[English](README.en.md)

`stardew-server-anxi-panel` 是一个围绕 [JunimoServer](https://stardew-valley-dedicated-server.github.io/server/) 构建的 Stardew Valley 专用服务器 Web 管理面板。

目标是让用户只需要运行一个 Anxi Panel Docker 镜像，打开浏览器，初始化管理员账号，安装 Stardew 服务器，完成 Steam 认证，选择存档，启动服务器，查看邀请码，监控状态，管理存档和 Mod，发送服务器命令，并管理面板用户。

> 当前状态：**Milestone 2: Storage and Auth**。后端已包含配置加载、SQLite 初始化、嵌入式迁移运行器、增强健康检查、基础结构化日志、统一 JSON 错误响应、管理员初始化、Argon2id 密码哈希、HttpOnly Cookie session、登录/登出、当前用户接口、admin/user 角色和管理员用户管理。前端已支持初始化页、登录页和基础主界面。Docker 控制、Junimo 安装、Steam Auth、存档、Mod 和控制台功能仍在计划中，尚未实现。

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
10. 选择存档：上传、选择已有存档或新建存档。
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

高层流程：

```text
React Frontend
  -> Go API
  -> jobs/state machine
  -> games/stardew_junimo driver
  -> Docker Compose / mounted files / attach-cli / Junimo HTTP status
  -> JunimoServer containers
```

本项目不会替代 JunimoServer，而是在 JunimoServer 官方 Docker 工作流外层提供一个更安全、可见、基于浏览器的管理体验。

## 仓库结构

```text
stardew-server-anxi-panel
├─ backend              Go API 服务
├─ frontend             React + TypeScript 前端
├─ docs
│  ├─ architecture.md   架构决策
│  ├─ handoff-roadmap.md
│  └─ prototypes        产品原型和说明
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
| `PANEL_DATA_DIR` | `/data` | 面板数据目录，启动时自动创建。 |
| `PANEL_DB_PATH` | `$PANEL_DATA_DIR/panel.db` | SQLite 数据库路径，启动时自动创建。 |
| `PANEL_SECRET` | empty | Session token hash secret。本地开发可为空；生产环境必须设置为足够随机的长 secret。 |
| `PANEL_VERSION` | `dev` | `/health` 返回的版本字符串。 |

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
    "message": "invalid username or password"
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
- 最后一个 active admin 不能被禁用或降级，当前登录 admin 不能禁用自己。

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
- admin 可看到最小用户管理区域。
- 普通 user 不显示用户管理入口。

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
6. 登录后主界面会显示当前用户和角色。
7. 点击“登出”后进入登录页。
8. 使用刚创建的管理员账号重新登录。
9. 管理员可以创建普通用户；普通用户登录后不能看到用户管理区域，也不能直接访问用户管理 API。

## 当前里程碑

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

仍未实现：

- Docker / Compose 控制逻辑
- Junimo 工作目录准备
- Steam Auth 交互
- 服务器启动、停止、重启
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

## 许可与第三方声明

本项目以 MIT License 发布，详见 [LICENSE](LICENSE)。

本项目会直接拉取并运行 JunimoServer 容器镜像来提供 Stardew Valley 专用服务器能力。JunimoServer 是独立的第三方项目，其上游仓库为 [stardew-valley-dedicated-server/server](https://github.com/stardew-valley-dedicated-server/server)，上游许可证为 [MIT License](https://github.com/stardew-valley-dedicated-server/server/blob/master/LICENSE)。JunimoServer 容器镜像、镜像内组件及其依赖仍由上游项目及对应第三方许可约束。本仓库不声称拥有 JunimoServer、Stardew Valley、Steam 或相关商标、游戏内容、素材和服务的所有权。

使用者需要自行确认自己拥有运行 Stardew Valley 服务器所需的合法授权，并遵守 JunimoServer、Stardew Valley、Steam 以及相关第三方组件的许可、服务条款和使用规则。
