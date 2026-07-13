# 后期优化文档

## PANEL-UPDATE-SAFETY-2 后续安全增强（暂缓）

- 当前以硬编码仓库、精确 tag、拉取后 digest、三项健康验收和自动回滚建立基本信任边界。后续可增加镜像签名/SBOM provenance 验证，并把发布流水线产出的 digest 清单纳入 updater 校验。
- 多 Compose 文件、自定义 service、Docker Swarm/Kubernetes 和无法验证宿主路径的 `docker run` 继续安全禁用；如未来支持，必须设计明确部署描述文件，不以路径猜测放宽检测。
- registry 认证代理、分段拉取重试、历史升级记录清理与备份保留策略可单独规划；任何凭据仍不得进入 updater 状态、日志或支持包。

本文只记录已经讨论过、但当前不急着做的优化。当前优先级仍是保持 Stardew 单实例面板稳定。

## PANEL-UPDATE-HISTORY-2 升级历史与通知增强（暂缓）

- FE-PANEL-UPDATE-1 已完成二次确认、升级阶段、预期断线重连、成功/回滚结果和桌面/移动响应式体验。
- 后续可增加多次升级历史、站内通知归档和更细的无障碍播报；不得放宽后端“只接受已检测最新正式版本、可信镜像和 panel 单服务”的边界，也不得在浏览器自动重试 apply POST。

## 2026-07 体积与性能专项审计建议

本节记录 2026-07-06 对 `stardew-server-anxi-panel` 主仓库和 `junimo-server-steam-service-cn` 配套仓库的体积、构建产物、前端轮询、后端 IO/Docker 调用路径的专项扫描结果。当前目标不是立刻改实现，而是为后续压缩镜像、减少运行时开销和降低维护成本排优先级。

### 审计基线

- Panel 镜像上一轮本地构建基线：`docker image ls` 显示约 194 MB；`docker save` 导出的 tar 约 55.9 MB。两者不是同一个口径：前者更接近本地解包后的虚拟大小，后者更接近可搬运产物大小。
- Panel 运行镜像最大层来自 `apk add docker-cli docker-cli-compose ca-certificates tzdata`；浏览器扩展 zip 已改为构建阶段产物，runtime 不再安装 `zip`。
- 当前前端构建产物 `frontend/dist` 为 155 个文件、约 19.83 MB；其中运行素材占绝大多数，JS 主包约 473.7 KB，CSS 约 286.9 KB，前端代码包体不是当前主要矛盾。
- 当前运行素材 `frontend/public/assets` 为 151 个文件、约 18.56 MB。最大目录是 `panels` 约 9.42 MB、`backgrounds` 约 4.23 MB、`icons` 约 2.98 MB。
- 当前最大运行素材包括：
  - `background_login_home_image2.png` 约 2.46 MB。
  - `background_login_farm_generated.png` 约 1.86 MB。
  - `right_card_health_9slice.png` 约 1.63 MB。
  - `right_rail_shell_middle_tile_seamless.png` 约 1.61 MB。
  - `right_card_recent_9slice.png` 约 1.27 MB。
  - `right_card_progress_9slice.png` 约 1.21 MB。
- `docs/prototypes` 已从 109 个文件、约 71.38 MB 的历史原型截图目录收敛为 3 个文件、约 2.58 MB 的轻量索引和关键基准图目录。完整截图应作为 Release artifact、对象存储或单独设计仓库制品保存。
- `frontend/node_modules` 约 74.19 MB，`frontend/dist` 约 19.83 MB，均已在 `.dockerignore` 中排除，属于本地工作区体积，不属于镜像体积。
- `junimo-server-steam-service-cn` 源码工作区约 7.30 MB，`.dockerignore` 已排除 docs、测试、node_modules、sub_modules 等；它对 Panel 镜像无直接体积贡献。但 Junimo 运行镜像自身包含 Debian GUI/VNC/ffmpeg/tmux 等重依赖，是后续服务器镜像体积优化的大方向。

### P0：先不要做的高风险优化

- 不建议短期内为了省体积直接删除 Panel 镜像内的 `docker-cli` 和 `docker-cli-compose`。这确实是当前 Panel 运行镜像最大层，但后端大量能力依赖 `docker compose ps/stats/logs/pull/up/down/restart/exec`，替换为 Docker Engine API 会影响安装、生命周期、日志、资源指标、Steam Auth、TTY 执行等多条主链路。
- 如果未来要移除 Docker CLI，应单独立项：先抽象 `backend/internal/docker`，用 Engine API 覆盖 `ps/stats/logs/exec`，再逐步替换 `compose up/down/restart/pull`。在完全覆盖之前，保留 CLI fallback。
- 不建议用 UPX 压缩 Go 二进制作为默认发布策略。它可能降低磁盘占用，但会带来启动、杀软误报、调试和崩溃定位成本；除非发布渠道明确需要极限压缩，否则收益不如素材和依赖拆分稳定。

### P1：镜像与发布体积

- 已完成：浏览器扩展 zip 从运行层 `zip -r` 改为 `extension-builder` 构建阶段产物，最终 Alpine runtime 不再安装 `zip`。
- 已完成：`docs/prototypes` 迁出主仓大图，只保留轻量索引和两张关键总览基准图；完整历史截图放外部制品。
- 已完成：超过 300 KB 的运行 PNG 已做一轮无损重压缩，并通过像素等价校验。登录背景因色调变化已回退为 PNG-only；其它背景现代格式需要后续单独做视觉等价校验后再启用。
- 已完成：favicon 改为 `.ico` + 32/64/128 PNG，多尺寸文件，避免继续把 512px 大图直接作为站点图标。
- 仍待后续单独视觉回归：面板/边框类 9-slice 或 tile 素材的切片重导出。当前只做无损压缩，没有改 `border-image` slice 或 tile/cap 结构，避免边缘模糊、alpha 失真和重复平铺接缝。
- 保持 `.dockerignore` 对 `frontend/dist`、`frontend/node_modules`、`docs` 类本地产物的排除。Dockerfile 当前使用精确 `COPY frontend`、`COPY backend`、`COPY browser-extensions`，后续如果改成 `COPY .`，必须同步复核构建上下文，否则 `docs/prototypes` 会重新进入 build context。

### P1：前端数据刷新与渲染

- `frontend/src/games/stardew/useStardewDashboardData.ts` 是当前面板数据刷新中枢。初始化会并发拉取 state、saves、mods、players、jobs、health、inviteCode、version；运行中 30 秒轮询 state/jobs，服务器 running 时 5 秒轮询 players，邀请码缺失或刷新时 5-10 秒轮询 inviteCode。
- 建议新增轻量聚合接口，例如 `GET /api/instances/:id/dashboard-summary`，返回 state、jobs 摘要、players 摘要、inviteCode 状态和关键 health 摘要。前端总览和右侧栏用聚合接口，详情页继续按需拉 saves/mods/health，减少首屏 7 个并发请求和后续重复请求。
- players 已有文件快照来源，后续优先做 `GET /api/instances/:id/players/stream` SSE：首屏走普通 `/players`，running 后订阅 SSE，断线再降级为 30 秒轮询。现有 5 秒轮询成本不高，但在长时间开面板、多用户同时打开时会放大。
- inviteCode 刷新可以改成状态驱动：启动/重启 job finished 或 state 变更后短时间高频轮询，拿到稳定邀请码后停止；不要在 `running && !inviteCode` 时永久 10 秒轮询。
- state/jobs 已有 job SSE，建议让 job finished 事件携带更多后续刷新提示，例如 `refresh: ["state","players","invite"]`，避免前端在 `refreshAfterJobFinished()` 里固定刷新 saves/mods/players/invite。
- `frontend/src/games/stardew/pages/ModsPage.tsx` 文件较大且状态密集，后续可以拆分 Nexus 扩展、远程安装、同步包、表格过滤等子组件。拆分后配合 `React.memo`/`useMemo`，减少输入框、分页、批量任务状态变化时整页重渲染。
- 已完成（阶段一，2026-07-11）：桌面端 9 个路由页面（`StardewPanel.tsx`）和移动端 5 个页面（`StardewMobileShell.tsx`）已改为 `React.lazy` + `Suspense` 按需加载，首屏只加载当前激活 Tab 的代码；构建后主 JS chunk 从约 579 KB 降到约 243 KB，`chunkSizeWarningLimit` 警告已消失。详见 `docs/03-frontend.md` “路由”一节和 `docs/frontend-handoff/frontend-handoff-2026-07-11.md`。
- 已完成（阶段二第一项，2026-07-11）：新增 `useStardewLifecycleActions.ts`，把 `OverviewPage.tsx` 和 `ServerControlPage.tsx` 中重复的启停 action（`handleStart/handleStop/handleRestart`、`saveStartBlocker`、`actionBusy/actionError/saveRequiredDetected/confirmAction/pendingStartupAction/pendingStopAction` 六个 state、三个派生 effect、`showSaveRequiredPrompt`/`canStart/canStop/canRestart` 派生值）合并为一个 hook（内部复用既有 `useStardewLifecycleState.ts` 做状态推导）。两页面改为 `useStardewLifecycleActions({ instanceState, dashboardData, isAdmin })` 一行接入；`OverviewPage.tsx` 555→456 行，`ServerControlPage.tsx` 少了约 90 行重复逻辑。顺带统一了一个行为差异：`ServerControlPage` 原本 `handleStop` 会额外 `refreshJobs()`，`OverviewPage` 没有，合并后两页面都执行 `refreshJobs()`（更完整，行为收敛，不是回归）。详见 `docs/frontend-handoff/frontend-handoff-2026-07-11.md` 的 `FE-LIFECYCLE-ACTIONS-1` 小节。
- 已完成（阶段二第二项，2026-07-11）：`ServerControlPage.tsx` 按业务领域拆成 9 个独立 hook——`useServerQuickBackup`（手动备份）、`useServerRestartSchedule`（计划重启）、`useServerVNCSettings`（VNC 端口/显示渲染）、`useServerPassword`（服务器密码 + JunimoServer 密码保护状态）、`useServerRuntimeSettings`（小屋策略/联机广播频率）、`useServerFestival`（触发节日活动）、`useServerJoja`（永久启用 Joja 路线）、`useServerConsole`（控制台命令列表与执行）、`useServerBroadcast`（全服喊话）。页面文件从 1437 行降到 979 行，JSX 基本只剩渲染，state/effect/handler 全部下沉到对应 hook。详见 `docs/frontend-handoff/frontend-handoff-2026-07-11.md` 的 `FE-SERVER-DOMAIN-HOOKS-1` 小节。
- 阶段二 `ModsPage.tsx` 已完成本服管理领域拆分（2026-07-12）：新增 `useModsManagement.ts`，下沉列表加载、上传/删除/导出、同步分类/同步包和当前存档启用切换，页面 2536→2360 行；Nexus 搜索、Key 与扩展批量安装保留为同一套强耦合状态机，避免拆散轮询/session 恢复/job 对账时序。
- 阶段二 `SavesSection.tsx` 已完成回档领域拆分（2026-07-12）：新增 `useSaveBackups.ts`（备份列表/策略/手动备份/彻底删除备份）和 `useSaveRestore.ts`（回档确认弹窗）两个 hook；存档列表 CRUD（选择/删除/导出）、新建游戏弹窗、上传存档弹窗仍留在页面内——这些不属于"回档"领域，且彼此耦合度低，本次不强行拆分。页面 1236→1131 行。`busy`/`setBusy` 这个跨多个操作共用的忙碌锁没有拆进任何一个 hook，仍留在 `SavesSection.tsx` 顶层并作为参数传给两个 hook，保持"任意一个存档相关写操作进行中，所有相关按钮一起禁用"的原有行为不变。详见 `docs/frontend-handoff/frontend-handoff-2026-07-11.md` 的 `FE-SAVES-DOMAIN-HOOKS-1` 小节。
- 已完成（阶段三，2026-07-12）：`StardewPanel.css` 按页面前缀拆为共享 Shell CSS + 9 个桌面页面 CSS，各页面自行 import 同名文件并随懒加载 chunk 按需加载。共享组件和跨页面合并选择器经二次审计保守留在共享 CSS；共享文件约 16586→4551 行。详见前端接手文档 `FE-CSS-SPLIT-1`。
- 当前多个文件中存在中文注释或用户可见文案乱码。建议单独做一次 UTF-8 编码修复和文案复核，防止错误文案进入 UI、日志和支持包。这是维护性优化，不建议夹在功能变更里做。

### P1：后端 IO、缓存与接口成本

- `ListModsWithState()` 每次读取 active/disabled mods 目录并解析每个 `manifest.json`。对于几十个 Mod 成本可接受；如果用户安装上百个 Mod，建议按目录 mtime、manifest mtime 和启用配置 mtime 做短 TTL 缓存，写操作后主动失效。
- `readSaveInfo()` 会读取 `SaveGameInfo` 或主存档 XML 并解析。存档列表、备份维护、导出命名都会触发相关读取。建议为存档列表增加基于主文件 mtime/size 的元数据缓存，避免总览或保存页反复解析 XML。
- `ListPlayers()` 优先读取 `.local-container/control/players.json`、`players-cache.json`、`players-events.json`，不走 Docker exec，这是正确方向。后续继续推动 Junimo/SMAPI 写结构化事件文件，避免回退到 `junimo info` 命令解析。
- `RunBackupMaintenance()` 通过 `filepath.Glob(save-events/*.json)` 扫事件文件。事件量小问题不大；如果后续事件更多，建议改为单个 JSONL/队列文件并做消费游标，减少目录扫描和大量小文件。
- 支持包 `backend/internal/web/support_bundle.go` 已完成流式 ZIP 导出（`SUPPORT-BUNDLE-STREAM-1`）：直接对 `http.ResponseWriter` 创建 `zip.Writer`，不再用 `bytes.Buffer` 聚合整个压缩包；响应不设置 `Content-Length`，浏览器下载仍可工作。后续新增支持包条目时继续保持“单项失败写入 ZIP 内 error/note”的模式。
- Mod 导出、存档导出、同步包导出当前多处使用 zip writer + 临时文件，这比整包进内存更稳。后续要统一限制单文件大小、总大小、临时文件清理、下载完成后的删除策略，避免大 Mod 包长期堆在系统临时目录。
- `AppendJobLog()` 每条日志通过 `SELECT COALESCE(MAX(sequence), 0) + 1 FROM job_logs WHERE job_id = ?` 获取序号。已有 `(job_id, sequence)` 索引，普通任务够用；高频日志任务可考虑在 jobs 表增加 `next_log_sequence` 或由内存 job context 分配序号，降低每行日志一次 MAX 查询。
- 任务和审计表已经有基础索引。后续如果任务日志长期保留，建议新增日志保留策略：按 job 数量、创建时间或总行数清理，避免 SQLite 文件无限增长。
- SQLite 已启用 `busy_timeout=5000` 和 WAL，这是合理默认。后续如果后台计划任务、多人操作和大量日志同时写入增加，可补充写入队列或批量日志写入，避免频繁小事务。

### P1：Docker 命令与运行时性能

- 当前 `backend/internal/docker` 通过 `exec.CommandContext` 调用 Docker CLI，命令输出有截断和脱敏，安全边界基本清晰。性能瓶颈主要来自进程启动和 Compose 自身延迟，不来自 Go 代码。
- 已完成：`ComposePs` 增加默认 1.5 秒短 TTL 成功结果缓存，`compose up/down/restart` 前后主动失效，吸收同屏短时间重复状态查询。
- 已完成：`ComposeStats --no-stream` 不再由右侧栏全局 5 秒轮询触发；资源指标只在诊断页可见时以 8 秒间隔刷新，后台 tab 隐藏时暂停。
- 已完成：普通 dashboard 初始化不再调用 `/api/health/diagnostics`，因此 `DockerVersion`/`ComposeVersion` 不进入普通总览轮询，只保留在 Diagnostics、Docker 状态页、安装前检查或用户手动刷新路径。
- 长命令 streaming 已用于 pull 等场景，这是正确方向。后续大日志 tail 或安装进度也应保持流式，避免等待命令完成后一次性返回。

### P2：Junimo 配套仓库与服务器镜像

- `junimo-server-steam-service-cn` 仓库源码体积不大，但生产 Dockerfile 基于 `jlesage/baseimage-gui:debian-11`，并安装 GUI、VNC、polybar、ffmpeg、tmux、tcpdump、调试工具等。这些能力对测试/可视化有价值，但对普通 headless 托管不一定都需要。
- 建议长期拆分服务器镜像 profile：
  - `server-headless`：只保留游戏、SMAPI、JunimoServer、必要 X/音频/Steam 依赖和 API。
  - `server-gui`：保留 VNC/桌面/截图/录制能力。
  - `server-test`：再额外包含测试 fixture、录制、调试工具和 test endpoints。
- 当前 Dockerfile 已有 `docker/modern/Dockerfile` 的 Alpine/Zink/SwiftShader 实验路线，目标是减少 GUI/OpenGL 依赖体积。它标注为 WIP，不应直接切生产，但可以作为 headless/modern profile 的技术储备。
- 生产镜像中 `TestFarmMod` 复制到 `/opt/test-fixtures`，默认不加载。为了进一步减小普通镜像，可把测试 fixture 移到 test profile，生产镜像完全不携带。
- `docker/rootfs/data/images/wallpaper-junimo-server.png` 和 `wallpaper-sdv.png` 体积不大，但属于 GUI/VNC 体验资产。headless profile 可不带。
- Junimo mod 已暴露 HTTP API/WebSocket，并在架构上适合事件驱动。Panel 后续玩家、日志、资源状态应优先消费 Junimo 的结构化 API/事件，而不是在 API 层增加 Stardew 逻辑或解析 UI/VNC。

### P2：验证与回归基准

- 体积验证：
  - `npm.cmd run build`，记录 JS/CSS 和 `frontend/dist` 总大小。
  - `docker build -t stardew-anxi-panel:local .`，记录 build context、`docker image ls`、`docker image history`。
  - 如需分发口径，执行 `docker save stardew-anxi-panel:local -o panel.tar` 并记录 tar 大小；不要把它和 `docker image ls` 混为同一指标。
- 前端性能验证：
  - 打开 Overview、Mods、Jobs、Players、Diagnostics，检查 Network 面板 1 分钟内请求数量。
  - running 状态下重点记录 `/players`、`/invite-code`、`/jobs`、`/state` 的请求频率。
  - Mods 页面用 100/300/500 个 mock mod 观察过滤、分页、批量操作的输入延迟。
- 后端性能验证：
  - 构造 100+ Mod、100+ save、100k job_logs 的本地数据集。
  - 对 `/mods`、`/saves`、`/players`、`/jobs/:id/logs`、`/support-bundle` 做响应时间和内存峰值记录。
  - 对 Docker 不可用、Compose 项目不存在、server 容器退出三类场景做降级测试。
- 视觉回归：
  - 素材压缩或格式切换后检查登录页、Overview、右侧栏、导航、Mods、Saves 和移动端。
  - 重点看像素边缘、9-slice 边框、平铺接缝、透明阴影和按钮 hover 状态。

### 建议执行顺序

1. 低风险体积清理已完成第一批：favicon 优化、扩展 zip 移到构建阶段、PNG 无损压缩和 `docs/prototypes` 外置；登录背景现代格式因色调变化已回退。
2. 下一批低风险性能优化：资源指标页面可见时刷新、Docker ps 短 TTL 缓存已落地；支持包流式 zip 已完成，后续继续做更细的缓存和日志保留策略。
3. 中等风险前端优化：dashboard-summary 聚合接口、players/inviteCode 状态驱动刷新、ModsPage 拆分。
4. 中等风险后端优化：mods/saves 元数据缓存、job log 保留策略、日志序号批量化。
5. 高风险架构优化：移除 Panel runtime Docker CLI、Junimo 服务器镜像 profile 拆分、headless/modern 生产化。

## 玩家正式数据模型（PLAYER-ROSTER-SQLITE-1，已完成第一期）

当前 `GET /api/instances/:id/players` 会合并三类来源：SMAPI 控制模组写入的 `players.json` 在线快照、面板维护的 `players-cache.json` 历史缓存，以及 Stardew 存档 XML 中的主机/Farmhand 数据。该链路已能兼容基础 `saveId`（如 `Farm`）与完整存档目录名（如 `Farm_442923526`），但 JSON 缓存同时承担运行时输入和历史数据库职责，身份判定、字段优先级和历史保留会继续变复杂。

第一期已把玩家名册迁入面板 SQLite，并落实以下边界：

- 联合身份使用 `instance_id + stable_save_id + player_id`。`player_id` 优先采用 Stardew `UniqueMultiplayerID`；确实缺失时只能使用带来源标记的临时身份，获取正式 ID 后合并，不能长期以可变玩家名作为主键。
- `stable_save_id` 必须在写库前统一解析。完整目录名中的数字后缀应作为首选稳定标识；基础名只作为别名保存和旧数据匹配入口，不能让 `Farm` 与另一个同名新存档永久共用历史。
- 名册至少保存：首次出现时间、最后在线时间、最后观测位置/坐标、最近收入快照、钱包模式、主机/角色和最后显示名。位置与收入同时记录 `observed_at`/`source`，避免存档睡眠位置覆盖更精确的运行时位置。
- `players.json` 和存档 XML 只作为事实输入；`players-cache.json` 在兼容迁移期只读导入，完成迁移后不再承担历史数据库职责。SMAPI 仍优先通过 Junimo/控制模组输出结构化快照或事件，不把 Stardew 业务逻辑堆到 Web API 层。

第一期表结构为：

- `player_roster`：当前名册快照，主键 `(instance_id, stable_save_id, player_id)`，保存上述首次/最后时间、位置、收入和来源字段。
- `save_identities`：记录完整目录 ID、基础名及最后解析时间，负责旧基础 `saveId` 到稳定 ID 的归一化。
- `player_events`：append-only 保存 `seen/joined/left`，由 `player_roster.current_status` 的状态跃迁生成，继续按最多 50 条返回 UI。

已完成 SQLite migration、storage repository、`/players` upsert/离线补齐，以及 `players-cache.json` / `players-events.json` 幂等导入；成功导入后旧 JSON 立即删除且不再写入。API 的名册与 `recentEvents` 响应结构均保持不变。

第一期自动化验收已覆盖：基础名/完整目录名归一化、同一玩家改名保持身份、运行时字段优先、旧缓存幂等导入与退役、SQLite 离线恢复。真实升级实例和同名新存档隔离仍需生产验证。

## 玩家事件驱动更新

当前玩家页通过 5 秒轮询：

```text
GET /api/instances/:id/players
```

后端读取：

- `.local-container/control/players.json`
- `.local-container/control/players-cache.json`

这个成本很低，不走 Docker exec，不扫大日志，当前足够稳定。

当前已完成第一版“最近事件”：后端通过 `players.json` 与 `players-cache.json` 差分生成首次记录、加入和离开事件，写入 `.local-container/control/players-events.json`，并通过 `/players` 响应的 `recentEvents` 返回。它仍属于轮询快照机制，不是实时推送。

未来如果要做更实时的事件推送：

1. SMAPI mod 继续写 `players.json` 当前快照。
2. SMAPI mod 新增 `player-events.jsonl`，追加 join/leave 事件。
3. 后端保留普通 `/players` 首次加载和兜底。
4. 新增：

```text
GET /api/instances/:id/players/stream
```

5. 前端先请求 `/players`，再订阅 SSE；断线后降级 30 秒低频轮询。

注意：

- 不要解析黄色弹窗、截图、VNC 或 OCR。
- 文件 watcher 在 Windows/WSL2/Docker bind mount 下可能不可靠，要有 mtime 兜底。
- 事件日志要轮转或截断，不能无限增长。

## 实时服务器日志流

当前任务日志和部分 command 输出已经可用，但完整 server log tail 仍可独立优化：

- 后端统一封装 compose logs tail。
- 前端 Jobs/Diagnostics 提供过滤、下载、暂停滚动。
- 支持包继续只导出尾部日志并脱敏。

## 计划重启设计

目标：给管理员一个可预期、可取消、有玩家提示的“维护窗口”能力。第一阶段不做复杂 cron，而是让用户配置每天几点关闭服务器、几点重新开启服务器。

### 产品语义

- “关闭时间”：到点开始维护，面板先广播提醒、创建当前激活存档备份，然后停止服务器。
- “开启时间”：到点结束维护，面板自动启动服务器，并按现有启动流程等待新邀请码。
- 如果关闭时间和开启时间相同，不允许保存；如果开启时间早于关闭时间，表示跨天维护窗口，例如 23:30 关闭、06:00 开启。
- 计划只负责服务器生命周期，不承诺强制保存游戏内实时进度；关闭前备份只备份已落盘存档。
- 第一阶段只支持单实例 Stardew 的一条计划，后续 Multi Game Mode 再泛化。

### 前端弹窗

入口：服务器控制页“快捷操作”的“计划重启”按钮。点击打开弹窗，不直接执行。

弹窗字段：

- 启用计划：开关。
- 每天关闭：`HH:mm` 时间输入，24 小时制。
- 每天开启：`HH:mm` 时间输入，24 小时制。
- 关闭前提醒：多选或固定标签，默认 `10 / 5 / 1` 分钟。
- 关闭前备份：默认开启，文案写“备份已保存进度”。
- 在线玩家策略：第一阶段建议只提示并继续执行；后续可加“有玩家在线则跳过本次维护”。
- 下次关闭时间、下次开启时间、最近执行结果：只读摘要。

弹窗操作：

- 保存计划：`PUT /api/instances/:id/restart-schedule`。
- 暂停计划：把 `enabled=false` 保存即可；可以不用单独 pause 接口。
- 取消下一次维护：可选第一阶段暂不做；如做则只跳过最近一组关闭/开启窗口。
- 立即关闭并按计划开启：不建议第一阶段加入，避免绕过现有停止确认。

### 后端 API

建议契约：

```text
GET /api/instances/:id/restart-schedule
PUT /api/instances/:id/restart-schedule
POST /api/instances/:id/restart-schedule/skip-next
```

请求体：

```json
{
  "enabled": true,
  "shutdownTime": "04:00",
  "startupTime": "04:20",
  "timezone": "Asia/Shanghai",
  "warningMinutes": [10, 5, 1],
  "backupBeforeShutdown": true,
  "skipIfPlayersOnline": false
}
```

响应体：

```json
{
  "schedule": {
    "enabled": true,
    "shutdownTime": "04:00",
    "startupTime": "04:20",
    "timezone": "Asia/Shanghai",
    "warningMinutes": [10, 5, 1],
    "backupBeforeShutdown": true,
    "skipIfPlayersOnline": false,
    "nextShutdownAt": "2026-07-03T04:00:00+08:00",
    "nextStartupAt": "2026-07-03T04:20:00+08:00",
    "lastShutdownAt": null,
    "lastStartupAt": null,
    "lastStatus": "idle",
    "lastMessage": ""
  }
}
```

校验规则：

- `shutdownTime/startupTime` 必须是 `HH:mm`，分钟粒度，24 小时制。
- `timezone` 第一阶段固定 `Asia/Shanghai`，前端可先不暴露选择。
- `warningMinutes` 去重、排序，只允许 1 到 120 分钟。
- `backupBeforeShutdown` 第一阶段默认 true；允许关闭，但 UI 要有确认提示。
- `shutdownTime != startupTime`。

### 后端架构

建议新增模块：

```text
backend/internal/scheduler/restart_schedule.go
backend/internal/storage/restart_schedule.go
backend/internal/web/restart_schedule_handlers.go
```

职责划分：

- storage：保存配置、最近执行状态、跳过下一次标记。优先 SQLite，字段带 `instance_id`，为多实例留口。
- scheduler：常驻 goroutine，按实例计算下一次 shutdown/startup/warning 触发点。
- web handler：鉴权、校验、读写配置、写审计日志。
- lifecycle：不新增并行控制逻辑，继续复用现有 start/stop job 或抽出共享 service，确保和手动启停互斥。

执行流程：

1. 后端启动时加载所有启用计划，计算下一次关闭、开启和提醒时间。
2. 每次 `PUT` 保存后，scheduler 重新计算该实例下一次触发点。
3. 到达提醒时间：如果实例 running，调用现有喊话能力广播“服务器将在 N 分钟后维护关闭，预计 HH:mm 开启”。
4. 到达关闭时间：创建 `scheduled_shutdown` job。
5. job 内重新读取状态；如果已有生命周期 job running/queued，则本次关闭标记为 `skipped_busy`。
6. 如果 `backupBeforeShutdown=true` 且有 active save，调用现有 `BackupSave` 创建手动备份；失败则默认阻止关闭并记录 `failed_backup`。
7. 调用现有停止流程，停止完成后更新 `lastShutdownAt/lastStatus`。
8. 到达开启时间：创建 `scheduled_startup` job。
9. job 内如果实例已经 running/starting，则标记 `skipped_already_running`；否则调用现有启动流程并等待状态刷新。

### 前端状态流

- `api.ts` 新增 `getRestartSchedule()`、`updateRestartSchedule()`、`skipNextRestartSchedule()`。
- `types.ts` 新增 `RestartSchedule` / `RestartScheduleResult`。
- `ServerControlPage` 新增计划重启弹窗 state：`scheduleOpen`、`scheduleDraft`、`scheduleLoading`、`scheduleSaving`、`scheduleError`。
- 打开弹窗时拉取当前配置；保存成功后关闭或留在弹窗并显示“已保存”。
- 快捷操作按钮旁可以展示一个短摘要：`计划：04:00 关闭 / 04:20 开启`。

### 安全边界

- 不做“强制保存世界”前置，除非后续 SMAPI/Junimo 明确提供可靠 API。
- 所有自动动作写审计日志，metadata 不包含邀请码或敏感信息。
- 计划任务必须走和手动生命周期一致的互斥逻辑，不能直接 `docker compose stop/up` 绕开状态机。
- 关闭前广播失败不阻断维护；关闭前备份失败默认阻断维护。

## 存档与备份增强

可选方向：

- 自动备份策略：启动前、停止后、定时备份。
- 备份备注和保留策略。
- 存档对比：日期、农场、玩家、地图、大小。
- 恢复前二次确认和冲突预览。

禁止方向：

- 不做破坏性“修复存档角色槽”的自动工具。
- 不直接改 Stardew 主存档 XML 来猜测联机异常。

## Mod 管理增强

可选方向：

- 启用/禁用 Mod。
- 依赖关系和版本检查：`MODDEPS-2` 已判断依赖是否已安装、当前存档是否启用、最低版本是否满足，并在前端提示缺失/未启用/版本不足；后续还需要做缺失依赖的一键安装入口、依赖来源索引和更新提示。
- Mod 更新提示。
- Mod 配置编辑。

需要注意：Stardew / SMAPI Mod 生态差异很大，先做只读检测和用户确认，不要自动修改复杂配置。

## 多游戏模式

未来进入 Multi Game Mode 后：

- 显示总面板实例列表。
- 每个游戏有自己的后端 driver 和前端 game module。
- 通用能力只包括用户、权限、任务、日志、诊断、备份入口。
- Stardew 专属 Steam Guard、邀请码、农场设置不能泄漏到其他游戏页面。

## UI 精修

可继续优化：

- 小屏表格改卡片。
- 右侧 rail 折叠。
- 部分 PNG 单独微调。
- 错误态和空态进一步统一。
- Overview 减负，把复杂管理收进对应页面。

每次视觉改动都要跑 `npm.cmd run build`，并检查 1280px、390px、320px。
# REMOTE-MOD / MODSEARCH 后续优化
- 远程安装目前只支持 ZIP。后续如需开放 7z/rar，必须先引入可靠解压器，并补齐 zip-slip、绝对路径、符号链接、单文件大小、总解压大小等同等级安全校验。
- 当前线上搜索入口是 Nexus-only，旧 `/mods/search` 统一搜索骨架已撤回。后续如重新做多来源搜索，需要重新设计接口，再接 StardewModDataset 本地持久化索引/后台刷新（完整名称全文搜索、依赖和 UpdateKeys 匹配）、CurseForge Core API、GitHub Releases（release asset）、ModDrop 稳定下载元数据。
- 多来源一键安装优先级可参考：CurseForge `download-url`、ModDrop `download-url`、GitHub Release / 直链、Nexus Premium、Nexus NXM/临时 CDN。没有自动链接时兜底为管理员粘贴 ZIP URL 或上传文件。
- ModDrop 当前不做页面抓取；除非确认存在稳定公开 API 或 Dataset 中能可靠映射到可下载 ZIP，否则只保留为后续统一搜索来源候选和管理员粘贴 ZIP 直链安装来源。
- Nexus 多文件 Mod 需要文件选择 UI，当前仍自动选 primary/main/first。
# SCHEDULED-RESTART-1 后续增强

计划重启第一阶段已落地：管理员弹窗、`GET|PUT /restart-schedule`、SQLite `restart_schedules`、后台 30 秒轮询、提前广播、关闭前备份、复用 Stop/Start lifecycle job。后续如果继续增强，优先考虑一次性跳过下一次维护、持久化 warning 去重、更多日历规则和更细的玩家在线策略。
