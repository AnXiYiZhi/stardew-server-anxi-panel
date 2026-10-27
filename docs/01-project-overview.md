# 项目总纲

本文是 `stardew-server-anxi-panel` 的长期入口文档。它只保留项目方向、边界和日常维护索引，细节分别放入后端、前端、联调、优化、路线和镜像构建文档。

## 项目目标

当前目标是在已经完整工作的 Stardew Valley 专用服务器面板外建立可扩展的游戏入口：用户通过浏览器选择游戏与服务器实例，再进入安装、Steam 认证、启动停止、邀请码、存档、Mod、玩家、诊断和面板用户管理。

通用游戏开服面板已经进入前端第一阶段，当前产品形态为“多游戏入口、Stardew 单 driver 完整可用”：

```text
当前：Multi Game Entry / Stardew Worlds
打开面板 -> 初始化/登录 -> 游戏库 -> Stardew 世界（服务器实例）列表 -> 实例专属面板

后续：Multi Driver Management
接入新的 GameDriver -> 为对应游戏提供真实安装、实例目录与实例创建契约
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

2026-10-27正式发布准备：用户已授权按0.7.3→0.8.0发布和官网上线。先从当前main完成073干净同步候选，含072及最老032真实Web代表链；同digest真实专项封存后才允许tag和三仓提升。073正式完成后才启用080目标并显式minor候选，080最老/上一来源均073。发布workflow增加candidate与专项双重证明，公开升级指引先更新，各版本首页与变更记录在正式Release后上线。当前尚未产生本轮正式候选，实际状态见镜像文档顶部。

2026-10-27完成状态：073全局更新中心、旧版强制先073再080、后端前置/提示校验与正式Auth140对接已实现并完成本地验收；真实072→073→080、unhealthy回滚、有效邀请码和原数据/功能验证通过，详见镜像文档V4记录。后续顶栏像素图标与更新中心展示收尾已完成前端/原App样例预览验证，属于新的候选输入。Auth代码库和阿里云等三仓固定版本已正式发布；Panel代码仍在本地main未提交/推送，073/080须分别通过干净同步main的正式候选流程。19172/games/stardew加载真实App与原背景卡片，仅隔离API数据；原临时DinD运行盘在Docker关闭后清空，历史验收证据保留。

2026-10-26后续状态：独立Auth已按上游完整名称 `1.5.0-preview.140-anxi.1` 发布到GitHub及三仓，正式digest4649205b…，阿里云回拉冒烟通过。Panel仍未发布；用户最新方向是073全局升级中心与强制前置升级，旧版只能先看到073、073后再提供080，同时把更新内容/重大提醒直接呈现给用户。这些新要求与正式Auth对接待讨论实施，下面的full-v6仅代表上一轮允许直升的本地工程证明。

2026-10-26本地增量：.140/Auth3升级闭环完成最终898输入工程快照full-v6验收，2281项Linux测试/vet/build通过。真实旧072 Web更新Panel后，另作实例范围确认，成功迁移140/Auth3并取得有效邀请码；升级后的备份恢复/导出、Mod开关、游戏启动与Panel重启保持通过，原1档/3角色/用户/Mod/专用会话/非目标资源完整。健康等待门槛、可信镜像别名范围比较及140无passwd的数字UID校验均已补齐，旧Web unhealthy和Auth目标失败整组恢复证据保留。用户明确以取得邀请码为本轮标准；默认仍`.125/Auth anxi.2/SMAPI4.5.2/Control0.3.9`，正式Auth身份与清洁main候选待另行发布授权，未提交、推送或发布，精确工程证明见镜像文档顶部。

当前正式版为 `v0.7.2`（commit `c8ec1db143e451b458c158bc63c7c9f38fffc834`）。本版纳入直连端口独立读取、紧凑登录/初始化卡片、总览与响应式布局、共享纸色，以及失效素材/代码清理和构建输入减负。升级来源为 `v0.7.1`，本版没有新增数据库迁移或运行栈版本变化，嵌入 Control 保持 `0.3.8`。

不可变候选 `35429187345`、独立兼容 `35429187386`、自动 annotated tag `35429987690` 与正式提升 `35429996287` 均成功；tag `v0.7.2` 指向上述 commit，三仓 `0.7.2/latest` 使用唯一 digest=`sha256:3ea2c86054c02673874413d00bfe05e13a17017551b6042ed26f87fb7954e655`。真实 Web 升级、unhealthy 回滚、全新/升级后直连专项和正式镜像 health/version/重启、Release 资产校验均通过，完整矩阵见镜像构建文档。[GitHub Release v0.7.2](https://github.com/AnXiYiZhi/stardew-server-anxi-panel/releases/tag/v0.7.2)

当前仓库已经围绕 Stardew driver 与动态实例路由形成完整闭环：

- 管理员初始化、登录、session、用户权限。
- SQLite 迁移、jobs、job logs、instance state、审计日志。
- Docker / Compose 控制层。
- Stardew Junimo 准备、安装、Steam Auth、生命周期控制。
- 存档新建、上传预览、选择、删除、导出、备份与恢复。
- Mod 上传、删除、导出和重启提示。
- 控制台 allowlist 命令与服务器喊话入口。
- 健康检查、支持包导出、版本信息。
- 面板正式版本检测，以及基于 Docker/Compose 自容器识别的独立 updater；管理员可全程在 Web 面板触发一键全栈升级，页面负责阶段展示、预期断线重连和结果恢复。Panel 重建包含 SQLite 一致性备份、三项健康验收和自动回滚；新 Panel 启动后会逐实例校验 Control 版本与 DLL hash，并以“通告、保存、整档备份、停服、更新、启动、SMAPI 实载验证”的受控事务完成运行栈同步。
- 中性游戏库 `/games`、Stardew 世界列表/全局安装入口，以及每个实例内保留的像素风 Shell、9 个路由和 UI 素材体系；DST、Minecraft 当前只做不可操作的未来入口。
- 玩家信息读取、位置中文映射、邀请码刷新和单人菜单暂停相关修复。

## 文档入口

| 文档 | 用途 |
| --- | --- |
| `docs/01-project-overview.md` | 项目总纲、边界、索引 |
| `docs/02-backend.md` | 后端架构、模块、API、测试 |
| `docs/03-frontend.md` | 前端架构、路由、UI、构建 |
| `docs/backend-handoff/`（最新：`backend-handoff-2026-07-16.md`） | 后端接手重点 |
| `docs/frontend-handoff/`（最新：`frontend-handoff-2026-07-11.md`） | 前端接手重点 |
| `docs/06-integration.md` | 前后端联调、API 契约、验证流程 |
| `docs/07-later-optimizations.md` | 后期优化池 |
| `docs/08-future-roadmap.md` | 已完成里程碑与未来路线 |
| `docs/09-image-build.md` | 镜像构建、部署、发布检查；顶部含 2026-10-15 Junimo `.125 → .140` 更新替换交接 |
| `docs/11-docs-portal.md` | 公开文档门户网站（VitePress + GitHub Pages）方案与实施步骤 |

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
