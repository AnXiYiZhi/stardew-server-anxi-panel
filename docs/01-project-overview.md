# 项目总纲

本文是 `stardew-server-anxi-panel` 的长期入口文档。它只保留项目方向、边界和日常维护索引，细节分别放入后端、前端、联调、优化、路线和镜像构建文档。

## 项目目标

当前目标是基于 JunimoServer 构建 Stardew Valley 专用服务器 Web 管理面板，让用户通过浏览器完成安装、Steam 认证、启动停止、邀请码查看、存档管理、Mod 管理、玩家信息、诊断和面板用户管理。

长期目标是通用游戏开服面板，但当前产品形态必须保持单游戏直达：

```text
当前：Single Game Mode
打开面板 -> 初始化/登录 -> 直接进入 Stardew 面板

未来：Multi Game Mode
打开面板 -> 总面板实例列表 -> 进入某个游戏专属面板
```

内部代码仍按 `instances + driver_id + GameDriver` 设计，避免后续接入 Minecraft、DST、Terraria、Palworld 时返工。

## 技术栈

| 层 | 技术 |
| --- | --- |
| 后端 | Go |
| 前端 | React + TypeScript + Vite |
| 数据库 | SQLite |
| 部署 | 单 Panel Docker 镜像 + Docker Socket |
| 游戏接入 | `GameDriver` 抽象 + Stardew Junimo driver |

## 核心原则

- 优先通过 JunimoServer 已有能力通信：挂载文件、Docker Compose、`steam-auth`、`server`、`attach-cli`、HTTP status、SMAPI 控制文件。
- 不在前端暴露任意 shell；控制台命令必须走后端 allowlist 和权限检查。
- 不把未来多游戏逻辑硬塞进 Stardew 页面。
- 面板负责鉴权、编排、状态记录、诊断和 API 转发，不重新实现 Stardew 服务端。
- Docker Socket 权限很高，默认只建议可信内网运行。
- 日志、错误响应、支持包必须脱敏 Steam 密码、VNC 密码、session、token、邀请码等敏感信息。

## 当前产品范围

当前仓库已经围绕单实例 Stardew 面板形成完整闭环：

- 管理员初始化、登录、session、用户权限。
- SQLite 迁移、jobs、job logs、instance state、审计日志。
- Docker / Compose 控制层。
- Stardew Junimo 准备、安装、Steam Auth、生命周期控制。
- 存档新建、上传预览、选择、删除、导出、备份与恢复。
- Mod 上传、删除、导出和重启提示。
- 控制台 allowlist 命令与服务器喊话入口。
- 健康检查、支持包导出、版本信息。
- Stardew 像素风前端 Shell、9 个路由和 UI 素材体系。
- 玩家信息读取、位置中文映射、邀请码刷新和单人菜单暂停相关修复。

## 文档入口

| 文档 | 用途 |
| --- | --- |
| `docs/01-project-overview.md` | 项目总纲、边界、索引 |
| `docs/02-backend.md` | 后端架构、模块、API、测试 |
| `docs/03-frontend.md` | 前端架构、路由、UI、构建 |
| `docs/backend-handoff/`（最新：`backend-handoff-2026-07-06.md`） | 后端接手重点 |
| `docs/frontend-handoff/`（最新：`frontend-handoff-2026-07-06.md`） | 前端接手重点 |
| `docs/06-integration.md` | 前后端联调、API 契约、验证流程 |
| `docs/07-later-optimizations.md` | 后期优化池 |
| `docs/08-future-roadmap.md` | 已完成里程碑与未来路线 |
| `docs/09-image-build.md` | 镜像构建、部署、发布检查 |

## 工作开始前必读

每次开始工作先读：

1. `docs/01-project-overview.md`
2. 与任务相关的 `docs/02-backend.md` 或 `docs/03-frontend.md`
3. 对应接手文档：`docs/backend-handoff/` 或 `docs/frontend-handoff/` 中最新文件
4. 涉及联调、发布或路线时，再读 `docs/06-integration.md`、`docs/08-future-roadmap.md`、`docs/09-image-build.md`

## 维护规则

- 做后端功能：更新后端文档、后端接手文档和未来路线中对应状态。
- 做前端功能：更新前端文档、前端接手文档和未来路线中对应状态。
- 做跨端联调：更新前后端联调文档。
- 做部署发布：更新镜像构建文档。
- 后续如需新增长期文档，必须先判断能否并入九份文档之一；默认不再新增零散流水文档。
