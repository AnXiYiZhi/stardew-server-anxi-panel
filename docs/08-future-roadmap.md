# 未来路线

## 当前路线判断

当前产品继续保持：

```text
Single Game Mode now
Multi Game Mode later
```

也就是说，用户体验上只看到 Stardew 面板；代码内部保留 `instances + driver_id + GameDriver`，后续新增第二个游戏时再显示总面板。

## 已完成里程碑摘要

| 阶段 | 状态 | 摘要 |
| --- | --- | --- |
| M0 Repo Skeleton | completed | 仓库、目录、基础文档 |
| M1 Backend Foundation | completed | Go 后端、配置、健康检查 |
| M2 Storage/Auth | completed | SQLite、用户、session、登录 |
| M3 Docker/Compose | completed | Docker 封装和调试接口 |
| M4 Jobs/State | completed | 长任务、日志、SSE、实例状态 |
| M5 GameDriver Registry | completed | driver 注册和实例模型 |
| M6 Junimo Prepare/Install | completed | Junimo compose、Steam Auth、安装 |
| M7 Lifecycle | completed | 启动、停止、重启、邀请码 |
| M7.5 New Game | completed | 自定义新建存档和素材 catalog |
| M8 Frontend MVP | completed | 登录、安装、主面板基础 |
| M9 Saves | completed | 存档管理、上传、删除、备份 |
| M10 Mods | completed | Mod 上传、删除、导出 |
| M11 Console | completed | allowlist 命令、喊话入口 |
| M12 Packaging | completed | Dockerfile、静态前端嵌入、部署 |
| M13 Hardening | completed | 审计、脱敏、权限、诊断、支持包 |
| M14 Release Candidate | completed | 发布检查、版本信息、支持包 |
| FE-R1 至 FE-R13 | completed | Stardew 像素风 Shell 与 9 路由 |
| UI-R7 至 UI-R12 | completed | 登录页和 UI 位图高级重绘 |
| PLAYERS-4 至 PLAYERS-6 | completed | 玩家精确位置与中文映射 |
| PLAYERS-7 | completed | 玩家页拆分农场收入与个人收入字段 |
| PLAYERS-8 | completed | 玩家活动最近事件，基于快照差分记录首次记录、加入和离开 |
| STATE-INVITE-1 至 4 | completed | 状态校准、重启后新邀请码等待与 server-only restart |
| AUTOPAUSE-1 至 7 | completed | 真人玩家菜单暂停、多人全员菜单共识暂停与 gameTimeInterval 哨兵时钟冻结 |
| DOCS-1 | completed | 文档归并为九份长期维护文档 |
| LIFECYCLE-JOBS-1 | completed | 停止/重启/再次启动会取消同实例旧生命周期任务，避免旧启动任务长期 running |
| FE-SHELL-SCROLL-1 | completed | Stardew Shell 固定视口高度，长页面仅中间内容区滚动，左右栏保持固定 |
| FE-LOGIN-IMAGE2-1 | completed | 登录/首次注册首页切换为 image2 原型图整页背景，前端覆盖绘制账号、密码区域和登录/注册按钮 |
| MODSYNC-1 | completed | Mod 玩家同步包第一阶段：`syncKind` 分类、面板自有 `mod-sync.json`、sync-plan/sync-classification/sync-pack 导出接口、ModsPage 玩家同步区域 |
| NEXUS-2 | completed | Mod 管理第二阶段：Nexus Mods 只读搜索（`GET mods/nexus/search`，ID 精确查询走官方 v1 REST、关键词走 GraphQL v2）、`UpdateKeys`/`NexusModID` manifest 解析、已安装匹配、ModsPage 在线搜索区域；不做下载/安装 |
| FE-MODS-WORKBENCH-1 | completed | ModsPage 参考 EMP Mod 管理台改为“下载模组 / 添加模组 / 配置模组”三段式工作台，Nexus 搜索卡片化，已安装与玩家同步归入添加页，配置页预留 SMAPI 配置入口 |

## 近期优先级

0. 玩家缓存按 `saveId` 隔离已修复；真实新建/切换存档后确认上一存档玩家不再出现在当前玩家列表。
1. 真实运行环境验证邀请码重启刷新、SMAPI DLL 加载，以及玩家页 `farmIncome`/`personalIncome` 显示。
2. 验证玩家页在真实多人场景下的位置、在线状态、中文地图名和最近事件。
3. 继续排查联机角色槽异常，保持只诊断不破坏存档。
4. 做一次完整 release checklist 冒烟测试。
5. 清理 UI 中已无 JSX 引用的旧 CSS 规则。
6. 用真实 `NEXUS_API_KEY` 验证 Nexus 关键词搜索的 GraphQL v2 返回结构；当前 `nexus.go` 里的字段名是根据公开资料推测的，未经真实账号验证，按 ID 精确查询的 v1 REST 路径已按官方文档实现、可信度更高。
7. 为 ModsPage 的“安装待接入 / 启用禁用 / 依赖检查 / SMAPI 配置”补齐后端能力，再把当前禁用入口改成真实操作。

## 中期路线

- 玩家事件驱动 SSE。
- 完整服务器日志 tail。
- 更完善备份策略。
- Mod 依赖和启用/禁用。
- Nexus Mods 下载安装（在当前只读搜索基础上接入下载/校验/导入，注意 Nexus 下载链接需要 premium 或 OAuth 流程，不能直接复用只读搜索的 `apikey` 调用方式）。
- 设置页中的审计过滤、会话管理、安全策略。
- 更完整的移动端导航和表格卡片化。

## 长期路线

### Multi Game Mode

启用条件：

- 至少新增第二个可用游戏 driver。
- 前端具备 game module registry。
- 总面板能展示实例列表、状态摘要和入口。

建议未来游戏：

- Minecraft
- Don't Starve Together
- Terraria
- Palworld
- Valheim

### 插件化

长期可以把 driver、前端模块、Compose 模板和文档模板进一步插件化，但不要在 Stardew MVP 阶段提前做复杂市场系统。

## 不要过早做

- 不要一开始做多游戏市场。
- 不要把未来游戏页面硬塞进 Stardew 模块。
- 不要绕过 GameDriver 在 handler 里堆游戏分支。
- 不要允许前端任意 shell。
- 不要用截图/OCR/VNC 解析游戏状态。
- 不要做会破坏存档的自动修复工具。
