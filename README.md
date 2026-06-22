# Stardew Anxi Panel

[English](README.en.md)

`stardew-server-anxi-panel` 是一个围绕 [JunimoServer](https://stardew-valley-dedicated-server.github.io/server/) 构建的 Stardew Valley 专用服务器 Web 管理面板。

目标是让用户只需要运行一个 Anxi Panel Docker 镜像，打开浏览器，初始化管理员账号，安装 Stardew 服务器，完成 Steam 认证，选择存档，启动服务器，查看邀请码，监控状态，管理存档和 Mod，发送服务器命令，并管理面板用户。

> 当前状态：**Milestone 1: Backend Foundation**。后端已包含配置加载、SQLite 初始化、最小迁移运行器、增强健康检查、基础结构化日志和统一 JSON 错误响应。Docker 控制、用户认证、Junimo 安装、Steam Auth、存档、Mod 和控制台功能仍在计划中，尚未实现。

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

后端配置：

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `PANEL_ADDR` | `:8090` | HTTP listen address. |
| `PANEL_DATA_DIR` | `/data` | Panel data directory, created on startup. |
| `PANEL_DB_PATH` | `$PANEL_DATA_DIR/panel.db` | SQLite database path, created on startup. |
| `PANEL_SECRET` | empty | Reserved for future auth/session features. |
| `PANEL_VERSION` | `dev` | Version string returned by `/health`. |

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

当前前端仍是 Milestone 0 的基础占位实现。

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

Milestone 1 不包含：

- Docker / Compose 控制逻辑
- 管理员初始化和登录
- 完整 SQLite 迁移
- Junimo 工作目录准备
- Steam Auth 交互
- 服务器启动、停止、重启
- 邀请码获取
- 存档管理
- Mod 管理
- 控制台命令
- 面板用户管理

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
