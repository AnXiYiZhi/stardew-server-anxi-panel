# 2026-07-07 已完成：Nexus 浏览器扩展适配 Shadow DOM 下载入口

- `NEXUS-MODPAGE-DL-2` completed：扩展 `0.1.1 → 0.1.2`，新增 `deepQueryAll()` 遍历 shadow root 定位下载控件，新增按 `data-tracking` 属性分类的 `findManualDownloadControl()`，新增 `openNexusFileList()`/`waitForFileIdOnPage()` 轮询 `file_id`，替代旧的两步按钮点击模型。仅改浏览器扩展，未改后端接口；发布新镜像后旧实例扩展缓存 ZIP 会被已有版本感知逻辑自动刷新。已验证 `node --check` 和 `go test ./internal/games/stardew_junimo -run TestEnsureNexusInstallerExtensionZip`。

# 2026-07-07 已完成：JunimoServer 静态 init 文件兼容挂载

- `JUNIMO-STATIC-INIT-FIX-1` completed：为 server 容器新增 `.local-container/cont-env/*`、`.local-container/cont-groups/*`、`.local-container/cont-users/*` 兼容挂载，批量遮罩上游 `sdvd/server:1.5.0-preview.121` 中裸静态值被 init 当命令执行的问题。
- 已覆盖 `APP_NAME`、`DBUS_SESSION_BUS_ADDRESS`、`DOCKER_IMAGE_PLATFORM`、XDG 路径、用户/组 id 等真实飞牛现场触发项；旧实例会自动迁移 compose，重启时如本次刚新增挂载会用 `docker compose up` 重建 server 容器。
- 验证：`cd backend; go test ./internal/games/stardew_junimo`；真机热修后 `stardew-server-1` healthy，Junimo API `/health` 返回 ok。

# 2026-07-07 已完成：Nexus 搜索防短断与局域网邀请地址修正

- 已完成：Nexus 搜索后端改用独立 20 秒上下文，避免浏览器刷新、切页、FRP/NAS 链路短断时把上游 GraphQL 请求提前取消并误报 `nexus request failed`。
- 已完成：Nexus 网络类错误新增 `nexus_network_failed`，后端日志保留真实底层错误，前端展示明确网络提示。
- 已完成：“局域网邀请”改为读取当前进入面板的 host；用户用什么 IP/域名加 `:8090` 打开面板，就展示什么 host。

# 2026-07-07 已完成：steam-auth 授权标志收口

- 已完成：`steamAuthLoggedIn` 收口为邀请码卡主 UI 授权标志；在 steam-auth 登录成功日志（`[SteamAuth:*] Logged in as ...` / `[SteamService] ... Logged in as ...`）出现后写 `STEAM_AUTH_COMPLETED=true`，启动/手动刷新成功拿到非空邀请码时也写 true。
- 已完成：启动/重启后如果 server 日志明确出现 `no logged-in accounts`，后端会清空 `STEAM_AUTH_COMPLETED`，前端下一轮状态刷新后显示【登录授权】；`Steam-auth service not ready` 不直接清 false，已有 true 时自动刷新一次 `steam-auth` 服务。
- 已完成：邀请码卡不再用 `steamAuthReady=false` 直接显示重新授权；`steamAuthReady` 保留为诊断字段，主按钮只按 `steamAuthLoggedIn` 显示。
- 已完成：生命周期启动/重启不再等待邀请码，server running 后 job 完成；后端后台最多探测 20 次邀请码，成功后写 `STEAM_AUTH_COMPLETED=true` 与 `/state.inviteCode`，失败不影响服务器和 IP 直连。前端启动中状态改按 active lifecycle job + running/stopping 状态，不再依赖邀请码、在线玩家或 SMAPI 存档加载日志。验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/games/stardew_junimo/config ./internal/web`、`cd backend; go test ./...`、`cd frontend; npm.cmd run build`。

# 2026-07-06 已完成：SteamCMD HOME 与缓存清理加固

- 已完成：SteamCMD fallback 以 `steam` 用户运行时显式设置 `HOME=/home/steam`，避免继续使用 `/root/.local/share/Steam` 自更新缓存引发 139 段错误。
- 已完成：Docker volume 清理改为逐个删除并忽略缺失卷，139 后自动清理 SteamCMD runtime cache 的成功率更高；真实失败会把 Docker 输出写入任务日志。
- 已完成：139 后清理 SteamCMD runtime cache 前，先按 volume 查找并删除残留 one-shot SteamCMD 容器，解决真实云服上 `volume is in use - [container_id]` 导致缓存无法清理的问题。

# 2026-07-06 已完成：SteamCMD SDK 分段下载

- 已完成：SteamCMD fallback 在同一个容器内拆成两次 SteamCMD 进程，分别下载/校验 `413150` 和 `1007`，避免 SDK 阶段因登录后切换 `force_install_dir` 触发 SteamCMD 139 段错误。
- 已完成：过滤旧 daocloud SteamCMD 候选，并在 SteamCMD exit code 139 时自动清理自更新缓存卷后重试一次。

# 2026-07-06 已完成：SDK 后置 SMAPI 预安装

- 已完成：安装流程在游戏文件与 Steam SDK 完成后，通过 JunimoServer 一次性容器预安装 SMAPI，减少 JunimoServer 首次启动时因 GitHub 下载 SMAPI 卡住的概率。
- 已完成：前端把 SMAPI 作为“下载游戏”步骤里的最后一个子状态展示，并区分 `smapi_install_failed` 后置失败。
- 后续可优化：如果后续维护自定义 JunimoServer 镜像，可考虑把 SMAPI 安装包内置进镜像，进一步减少现场 GitHub 下载依赖。

# FE-STEAM-GUARD-SUBMITTED-FEEDBACK-1 状态

- `FE-STEAM-GUARD-SUBMITTED-FEEDBACK-1` completed：Steam / SteamCMD 验证码提交成功后，安装页会显示“验证码已提交，正在等待响应”的本地等待态，不再回到同一个空输入框。等待态保留“重新输入”入口，并在 phase 进入下载、失败或完成后自动清除。

# STEAMCMD-EMAIL-GUARD-PROMPT-1 状态

- `STEAMCMD-EMAIL-GUARD-PROMPT-1` completed：SteamCMD 首次新机器登录时的邮箱 Steam Guard 分行提示已纳入后端和前端识别。后端会把 `This computer has not been authenticated...` / `Please check your email...` / `code from that message` / `set_steam_guard_code` 等日志切到 `steamcmd_guard_required`；前端也会在 job 日志先到时展示 SteamCMD 验证码输入框，避免安装页卡在下载/自更新进度。

# INSTALL-ROUTING-SPLIT-1 状态
- `INSTALL-ROUTING-SPLIT-1` completed：安装流程按用户口述的完整期望重构。把 `reuseCredentials` 粗暴驱动的单一 `steamCMDRetry` 拆成 `reuse` / `steamCMDDirect` / `steamCMDUseCache` 三个正交决策，用已持久化的 driverPhase/state 判“是否已过认证”、用新 `.env` 标志 `STEAMCMD_AUTH_COMPLETED` 判“SteamCMD 是否有缓存”。修复：①镜像拉取失败重试重新拉镜像 + steam-auth（不再误跳 SteamCMD）；②SteamCMD 认证超时重试回到登录验证界面（不再秒报“授权缓存不可用”）；③认证成功后跨会话可靠跳过 steam-auth。并新增「更换 Steam 账号 / 强制重新认证」入口（`forceReauth`：清 steam-session/steamcmd 授权卷 + 重置标志，保留 game-data）；已安装态只保留卡片内换号按钮，操作区不再重复渲染。已验证 `cd backend; go test ./...`、`cd frontend; npm.cmd run build`。

# STEAMCMD-REPAIR-DIRECT-1 状态
- `STEAMCMD-REPAIR-DIRECT-1` completed：安装页“重新安装 / 修复”与复用凭据重试入口已改为直达 SteamCMD 下载/校验。前端不再要求输入 Steam/VNC 凭据；后端收到 `reuseCredentials=true` 后跳过 `steam-auth`，并用已有 SteamCMD 授权缓存执行 app_update。已验证 `cd backend; go test ./internal/games/stardew_junimo ./internal/web` 与 `cd frontend; npm.cmd run build`。

# FE-TOPBAR-BRAND-LIGHTER-2 状态
- `FE-TOPBAR-BRAND-LIGHTER-2` completed：Stardew Shell 左上角品牌标题已按“再细 200”反馈继续减重，`.sd-topbar-brand-text` 字重从 `700` 降到 `500`，暗色描边/投影不透明度同步降低；仅改前端 CSS，未改顶栏状态牌、存档/用户框、API、权限、路由或 Junimo 通信。验证：`cd frontend; npm.cmd run build`；Browser QA 标题 computed `fontWeight=500`，总览/服务器往返后仍为 `500`。

# FE-TOPBAR-BRAND-LIGHTER-1 状态
- `FE-TOPBAR-BRAND-LIGHTER-1` completed：Stardew Shell 左上角品牌标题已按用户反馈再调细，`.sd-topbar-brand-text` 字重从 `800` 降到 `700`，并减少暗色描边/投影层数；仅改前端 CSS，未改顶栏状态牌、存档/用户框、API、权限、路由或 Junimo 通信。验证：`cd frontend; npm.cmd run build`。

# FE-OVERVIEW-HEALTH-SHARE-1 状态
- `FE-OVERVIEW-HEALTH-SHARE-1` completed：诊断页成功执行健康检查后会把结果同步到公共 dashboard 数据层，用户从诊断页返回总览页时“系统健康”卡会显示最新评分和状态，不再保持 `— / 未检查`。普通 dashboard 初始化仍不主动调用 `/api/health/diagnostics`，保留诊断按需触发策略。验证：`cd frontend; npm.cmd run build`；Browser QA 通过总览 `—` -> 诊断页 6 项正常 -> 回总览 `100% / 6项全部通过 / 优秀`。

# PUBLIC-IP-LOOKUP-1 状态
- `PUBLIC-IP-LOOKUP-1` completed：新增 `GET /api/instances/:id/public-ip`，由面板后端检测服务器公网出口 IP，返回 `{ ip, checkedAt, source?, cached }`，成功结果缓存 `10min`，前端手动刷新会强制重新探测。邀请卡片已在邀请码下方新增公网 IP 框和复制/刷新按钮，上方标题保持“邀请码”，下方标题显示“局域网邀请”，公网 IP 失败态不显示复制按钮但两行值框保持同宽，并按用户反馈移除邀请码说明文字。未改 Junimo driver、Docker/Compose、实例状态或邀请码接口。验证：`cd backend; go test ./internal/web`，`cd frontend; npm.cmd run build`。

# FE-MOD-COUNT-FILTER-BUILTIN-1 状态
- `FE-MOD-COUNT-FILTER-BUILTIN-1` completed：总览页模组统计已过滤 SMAPI runtime、`StardewAnxiPanel.Control`、`JunimoServer` / `JunimoHost.Server` 三类内置运行组件；模组统计卡、`已启用 N 个` 和同步包摘要里的已启用/已停用数量均按用户可见 Mod 计算。系统运行组件识别已抽为 `mod-visibility.ts`，由总览页和模组页共用。仅改前端展示口径，未改后端 API、启用切换接口、同步包导出或 Junimo 通信。验证：`cd frontend; npm.cmd run build`。

# DOCKER-POLL-PERF-1 状态

- `DOCKER-POLL-PERF-1` completed：`ComposePs` 已增加默认 `1.5s` 短 TTL 成功结果缓存，并在 `compose up/down/restart` 前后失效，减少状态页、诊断和支持包短时间重复触发 Docker CLI 的开销。
- 前端已停止普通 dashboard 初始化里的健康诊断请求，也移除右侧栏 `/metrics` 常驻轮询；资源指标只在诊断页可见时按 `8s` 间隔采样，后台 tab 隐藏时暂停。
- `DockerVersion` / `ComposeVersion` 保持在 Diagnostics、Docker 状态页、安装前检查或用户手动刷新路径，不进入普通总览轮询。验证：`cd backend; go test -count=1 ./internal/docker`，`cd frontend; npm.cmd run build`。

# ASSET-RUNTIME-SLIM-1 状态
- `ASSET-RUNTIME-SLIM-1` completed：浏览器扩展安装包已从 runtime `zip -r` 改为 `extension-builder` 构建阶段产物，最终 Alpine runtime 不再安装 `zip`；`docs/prototypes` 已从 109 个历史截图/提取文件收敛为轻量索引和 2 张关键总览基准图，完整原型截图改由外部制品承接；超过 300 KB 的运行 PNG 已做无损重压缩并通过像素等价校验；登录背景因色调变化已回退为 PNG-only；favicon 已改为 `.ico` 加 32/64/128 PNG。

# SUPPORT-BUNDLE-STREAM-1 状态

- `SUPPORT-BUNDLE-STREAM-1` completed：支持包导出已改为直接对 HTTP 响应流写 ZIP，不再用内存 `bytes.Buffer` 聚合整个压缩包后一次性返回。下载接口、文件名和 ZIP 条目保持不变；响应不再设置 `Content-Length`，前端应按 Blob 下载处理。已验证 `cd backend; go test ./internal/web -run "SupportBundle|Docker|Metrics"` 与 `cd backend; go test ./...`。

# FE-CLEANUP-UNUSED-ASSETS-1 状态
- `FE-CLEANUP-UNUSED-ASSETS-1` completed：前端生产素材已清理 79 个源码零引用旧 PNG，主要是旧右栏整图、旧顶栏三段、旧导航/字段/图标 sheet 与早期装饰 sprite；`frontend/public/assets` 从约 39.52MB 降到约 18.56MB。同步删除无引用组件 `CommandOutput`、`StatusPill`、`StatusBadge`、`InstanceStateCard`。未改业务逻辑、API、路由或 Junimo 通信；保留动态路径使用的 `new-game` 素材和 QA 入口。已验证素材复扫无非 `new-game` 零引用文件，`cd frontend; npm.cmd run build` 通过。

# FE-MODS-HIDE-SYSTEM-RUNTIME-1 状态
- `FE-MODS-HIDE-SYSTEM-RUNTIME-1` completed：模组页已隐藏 SMAPI runtime、`StardewAnxiPanel.Control` 和 `JunimoServer` / `JunimoHost.Server` 这类系统运行组件；它们不再出现在“添加模组”的已安装卡片或“配置模组”的当前存档启用状态列表中。用户可见“已安装”和解析失败统计同步只计算普通 Mod；玩家同步统计和导出仍保留完整列表逻辑，避免影响基础运行依赖处理。仅改前端 `ModsPage.tsx` 和文档，未改后端 API、启用切换接口、玩家同步包导出或 Junimo 通信。验证：`cd frontend; npm.cmd run build`。

# JUNIMO-MOD-MOUNT-RESTORE-1 状态
- `JUNIMO-MOD-MOUNT-RESTORE-1` completed：启动/重启前自动从当前 `sdvd/server` 镜像同步官方 `JunimoServer` Mod 到宿主 `.local-container/mods`，修复 mods 挂载遮住镜像内置组件后 Junimo API/VNC rendering 不可用的问题。`JunimoHost.Server` 现在是内置强制启用组件，物理 `smapi` 运行时目录不再显示为重复 Mod。验证：`go test ./internal/games/stardew_junimo -run "ListMods|ApplyNewSaveDefault|ApplyModProfileKeeps|Rendering|JunimoServerMod"`、`go test ./internal/web -run "Rendering|VNCConfig"`。

# ENV-BOM-NORMALIZE-1 状态
- `ENV-BOM-NORMALIZE-1` completed：实例 `.env` 读取/写回会剥离 UTF-8 BOM 前缀 key，修复 `﻿IMAGE_VERSION` 导致 `docker compose up` 报 `unexpected character "\ufeff"` 的启动失败。当前 `data/instances/stardew/.env` 已热修，`docker compose config --quiet` 和 `docker compose up -d` 已验证通过；后续排查启动失败时应先区分 Compose 配置解析错误和容器进程错误。

# STEAMCMD-SELFUPDATE-CACHE-1 状态
- `STEAMCMD-SELFUPDATE-CACHE-1` completed：SteamCMD 兜底容器已持久化 `/root/.local/share/Steam` 与 `/home/steam/.local/share/Steam`，用于缓存 SteamCMD 客户端自更新文件；前端将登录前 `[steamcmd] [ N%] Downloading update (... of 40,273 KB)` 标记为 SteamCMD 客户端更新，不再误显示为镜像拉取或游戏文件下载。验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallSteamCMD"`，`cd frontend; npm.cmd run build`。

# STEAMCMD-RETRY-RESUME-1 状态
- `STEAMCMD-RETRY-RESUME-1` completed：SteamCMD 兜底授权超时/失败后，复用凭据重试会直接恢复 SteamCMD fallback，不再先跑 Junimo compose pull 或 `steam-auth`；SteamCMD 镜像候选会先全量 inspect，本地已有任意候选即直接使用，避免重复拉取已下载镜像。前端安装页已补充“重试 SteamCMD 授权/下载”和“不重新拉取已有 SteamCMD 镜像”的提示。验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallSteamCMD"`，`cd frontend; npm.cmd run build`。

# STEAMCMD-FALLBACK-1 状态

- `STEAMCMD-FALLBACK-1` completed：安装流程在 `steam-auth` 已认证成功但游戏文件下载失败时，会自动切换到 SteamCMD 兜底下载，复用已保存 Steam 账号密码并持久化 SteamCMD 授权缓存；需要验证时前端展示 SteamCMD 专属“手机 App 批准 / App 或邮箱验证码”选项。新增 `steamcmd_*` 安装 phase、Docker 通用 TTY 容器执行和 SteamCMD 镜像拉取能力；SteamCMD 镜像按 `STEAMCMD_IMAGE_CANDIDATES` 候选列表兜底，默认顺序为 `docker.1ms.run/steamcmd/steamcmd:latest`、`docker.m.daocloud.io/steamcmd/steamcmd:latest`、`ghcr.io/steamcmd/steamcmd:latest`、`cm2network/steamcmd:latest`，旧实例会补齐新候选并过滤直连 Docker Hub 的 `steamcmd/steamcmd:latest` 和已移除的 `docker.xuanyuan.me/steamcmd/steamcmd:latest`，单次 pull 默认等待 30 分钟，避免单个镜像源 403 或慢速拉取后直接失败。已验证 `cd backend; go test ./internal/docker ./internal/web ./internal/games/stardew_junimo` 与 `cd frontend; npm.cmd run build` 通过。

# FE-STEAM-AUTH-DOWNLOAD-PROGRESS-RESTORE-1 状态
- `FE-STEAM-AUTH-DOWNLOAD-PROGRESS-RESTORE-1` completed：安装页 Steam 认证区现在按最新日志识别下载阶段，日志出现 `Downloading app 413150` / `Progress:` 后会显示游戏下载卡和真实文件/体积进度条，不再停留在手机 App 批准登录；历史 QR URL 也不再压过后续 Guard 验证码输入。仅改 `frontend/src/games/stardew/pages/InstallPage.tsx`，未改后端接口、SSE 或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；Browser QA 壳加载正常、无 overlay、console error/warn 为空。

# INSTALL-INTERRUPTED-STATE-1 状态
- `INSTALL-INTERRUPTED-STATE-1` completed：修复安装任务失败/面板重启后实例仍残留 `steam_auth_running` 导致前端卡在 48% 的问题。后端恢复 interrupted `stardew_install` 时会同步写 `install_interrupted`，steam-auth 容器运行错误会写 `steam_auth_failed`；前端安装页以活跃 job 为准，没有活跃安装任务时把旧运行中 phase 视为中断失败，并加载最近安装任务日志。已验证 `cd backend; go test ./internal/jobs ./internal/games/stardew_junimo` 与 `cd frontend; npm.cmd run build` 通过。

# DEPLOY-RUN-SH-1 状态

- `DEPLOY-RUN-SH-1` completed：新增 `deploy/run.sh` 作为用户优先使用的一键启动/维护脚本，产品口径收敛为快速模式：默认通过 `http://服务器IP:8090` 直接访问面板。脚本默认使用国内阿里云 ACR 面板镜像、默认 `latest` tag，并支持 `PANEL_VERSION=0.1.0` 固定版本；首次运行会生成 `~/.anxi-panel/.env`、`docker-compose.yml` 和 `~/.anxi-panel/data`，自动创建强随机 `PANEL_SECRET`。新版脚本使用宿主机数据目录与容器内同名绝对路径持久化面板数据；菜单覆盖 Docker 安装修复、镜像候选兜底、启动、停止、重启、更新、状态、日志、镜像源切换和显示访问地址；同时支持 `install/docker/stop/restart/update/status/logs/url` 等非交互命令。已同步 `docs/09-image-build.md`。

# FE-OPSRAIL-MAINTENANCE-PHASE-1 状态

- `FE-OPSRAIL-MAINTENANCE-PHASE-1` completed：右栏“进行中”卡的计划维护展示已从纯 `nextShutdownAt/nextStartupAt` 倒计时改为当前维护窗口阶段状态。到关机点后显示 `关机中 / 等待关机结束`，关机完成后保留自动开机倒计时，到开机点后显示 `开机中 / 等待开机结束`，开机 job 成功后才切回下一天倒计时；计划维护对应的生命周期 job 不再重复作为普通任务显示。仅改前端 `StardewPanel.tsx`，未改后端接口、调度器、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；Browser QA 全壳右栏渲染无 overlay、console error/warn 为空。

# FE-OVERVIEW-METRIC-TYPE-UNIFY-1 状态

- `FE-OVERVIEW-METRIC-TYPE-UNIFY-1` completed：总览页四张统计卡（存档 / 模组 / 系统健康 / 运行任务）已按用户截图反馈统一字体节奏。`.sd-ov-metric-strip` 下标题、数字、单位、说明和状态徽章现在使用同一 Verdana / Microsoft YaHei / SimHei 字体链，标题为 `14px/800`，数字为 `34px/800`，并减轻数字阴影，避免截图中的字体割裂和过粗跳字。仅改 `StardewPanel.css`，未改 TSX、API、权限、轮询或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x1024、点击服务器再回总览、390x844，console error/warn 为空，overlay 为 0，无横向溢出。

# FE-RESTART-SCHEDULE-PUT-WRITE-MODEL-1 状态

- `FE-RESTART-SCHEDULE-PUT-WRITE-MODEL-1` completed：服务器控制页“计划重启”保存请求体已收口为独立写入模型，避免把 `GET /restart-schedule` 返回的 `instanceId/nextShutdownAt/lastStatus` 等只读展示字段原样回传，触发后端 `DisallowUnknownFields()` 后显示 `request body must be valid JSON`。仅改前端 `types.ts` 与 `api.ts`，未改接口路径、后端调度器、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-STOPPED-STATUS-RED-1 状态

- `FE-STOPPED-STATUS-RED-1` completed：总览页“服务器控制”状态行、服务器控制页顶部摘要状态和生命周期状态行的 `已停止` 已按用户截图反馈改为红色字样，停止态状态点同步为红点。仅改前端展示类名与 CSS，未改生命周期 API、按钮 handler、权限、轮询或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 stopped 总览、服务器页和 390x844 窄屏，目标文字 computed color 均为 `rgb(192, 32, 32)`，console error/warn 为空，无页面级横向溢出。

# PLAYERS-SAVE-ROSTER-1 状态

- `PLAYERS-SAVE-ROSTER-1` completed：玩家名册已恢复从当前 Stardew 存档主 XML 合并 `<player>` 与 `<farmhands><Farmer>`，解决 `players.json` 在线快照只有 host、缓存也只有 host 时，存档里的 farmhand `test` 不显示的问题。存档离线玩家返回 `status=offline/source=save_file`，在线快照仍覆盖同一玩家；前端“等待加入”只统计 `waiting/pending/joining`，不再把离线名册行算进去。已验证 `cd backend; go test ./internal/games/stardew_junimo` 与 `cd frontend; npm.cmd run build` 通过。

# FE-JOBS-LOG-SCROLL-LOCK-1 状态

- `FE-JOBS-LOG-SCROLL-LOCK-1` completed：修复点击“任务与日志”后 Stardew Shell 被浏览器外层页面向下滚走、顶栏消失、底部露黑的问题。根因是缩放 Shell 的未缩放布局盒子让 `body/#root` 产生页面级纵向滚动，任务日志 `scrollIntoView()` 又触发了外层滚动。已在 `App.css` 对 `.sd-shell` 运行态锁定 `body/#root` 为 `100dvh + overflow:hidden`，并把任务日志、安装日志自动滚到底改为滚动各自日志窗口自身。未改 API、SSE、权限、轮询、路由或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 1112x920 下修复前 `window.scrollY=351/.sd-shell.top=-351`，修复后点击任务日志并强制 `window.scrollTo(0,600)` 仍保持 `scrollY=0/.sd-shell.top=0`，console error 为空。

# FE-MOBILE-NAV-BAR-SIZE-1 状态

- `FE-MOBILE-NAV-BAR-SIZE-1` completed：单栏状态下最上方横向选择栏已按截图反馈适量放大。`max-width: 640px` 下导航行从 `40px` 提到 `48px`，按钮从 `36x30` 提到 `42x38`，图标从 `20px` 提到 `23px`。仅改前端 CSS 和接手文档，未改路由、页面逻辑、API、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过，内置 Browser QA 覆盖 490x844 和 390x844，点击“服务器”导航后 active route 正常切换，console error/warn 为空，页面级横向溢出为 0。

# FE-OVERVIEW-LIFECYCLE-LEFT-1 状态
- `FE-OVERVIEW-LIFECYCLE-LEFT-1` completed：总览页“服务器控制”区启动/停止/重启按钮已按用户截图反馈从生命周期区域中间移回左侧，与标题和状态行对齐。根因是旧生命周期 flex 规则残留 `flex-wrap: wrap` + `align-content: center`，本次在总览最终覆盖段改为 `flex-wrap: nowrap`、`align-content: flex-start`，并让按钮行 `align-self: flex-start`。仅改前端 CSS，未改生命周期 API、按钮 handler、邀请码刷新、权限、轮询或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖默认视口、点击启动交互和 390x844 窄屏，console error/warn 为空且无页面级横向溢出。

# FE-PLAYERS-ACTION-ICONS-IMAGE2-1 状态
- `FE-PLAYERS-ACTION-ICONS-IMAGE2-1` completed：玩家页活动列表文字挤压已修正，分页由上一轮的每页 3 条改为每页 2 条并提高事件行高度；管理操作四个 CSS 临时图标已替换为 imagegen 生成、透明抠底后的 image2/Stardew 像素风 PNG（踢出、封禁、白名单、权限）。未改后端 API、玩家事件接口、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x1024 与 390x844，console error/warn 为空、无页面级横向溢出。

# FE-SAVES-BACKUP-POLICY-LAYOUT-1 状态
- `FE-SAVES-BACKUP-POLICY-LAYOUT-1` completed：存档页“自动备份策略”卡片已按用户反馈修正文字错乱。定时备份项调整为“勾选框 / 定时备份 / 每天 / 时间选择框”同一行顺序，每日快照保留行与滑杆重新分配宽度，左侧策略卡不再被右侧列表拉伸。仅改前端 `SavesSection.tsx` 与 `StardewPanel.css`，未改备份策略保存、恢复/删除、权限、后端 API 或 Junimo 通信。已验证清理过期 tsbuildinfo 后 `cd frontend; npm.cmd run build` 通过；QA mock 全壳截图确认策略卡无文字错乱且 console error/warn 为空。

# FE-TOPBAR-SAVE-STATUS-TYPE-1 状态
- `FE-TOPBAR-SAVE-STATUS-TYPE-1` completed：Stardew Shell 顶栏已按用户截图反馈微调。标题字体从过粗的海报感改为更轻、更小的像素描边风格；运行中/已停止状态改用现有像素状态牌素材；存档框移除下拉箭头，农场图标左移贴边，文本改为“农场名：简略游戏时间”（如 `AnxiFarm：第一年春`）；用户角色框也移除下拉箭头。仅改前端 `StardewPanel.tsx` / `StardewPanel.css`，未改 API、权限、路由或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x720 running/stopped 和 390x844，无横向溢出、无 overlay、console error/warn 为空。

# FE-MODS-PROTOTYPE-V02-LAYOUT-1 状态
- `FE-MODS-PROTOTYPE-V02-LAYOUT-1` completed：模组管理页已按 version-02 原型 `06-mods.png` 回正卡片顺序和比例。首屏固定为标题操作区、三段标签页、Nexus 连接横条、搜索卡、2x2 搜索结果卡和分页；下载结果卡恢复原型两按钮结构，移除额外的会员安装按钮；按用户截图反馈移除底部扩展安装进度条和“全部类别”下拉框；搜索提示改为“输入英文模组名称、ID 或关键词...”，热门标签改为 `UI Info`、`Fishing Mod`、`Backpack Upgrades`、`Tractor` 并保持真实快捷搜索。模组页卡片复用其它页面统一羊皮纸卡片变量；前置状态统一放在统计行“认可”后面，无前置也显示“前置：无”，保证每张卡操作按钮垂直位置一致。仅改前端 `ModsPage.tsx` 与 `StardewPanel.css`，未改后端 API、上传/删除/导出、启用切换、玩家同步包或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x1024 和 390x844，无页面级横向溢出，热门标签点击后搜索框为 `Tractor`。

# FE-DIAGNOSTICS-GAUGE-INNER-SAFE-1 状态
- `FE-DIAGNOSTICS-GAUGE-INNER-SAFE-1` completed：诊断页资源趋势圆环卡已按用户反馈修正数字安全区，保持三张卡片与原型比例，扩大中心底色圆并降低数字/百分号字号，避免红色弧线遮挡 `27.1%` 等百分比读数。仅改前端 CSS，未改资源指标接口、健康检查、轮询、导出诊断包或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 点击“诊断”后确认三张资源卡数字不再被弧线遮挡，console error/warn 为空。

# FE-PLAYERS-TIME-EVENTS-PAGING-1 状态
- `FE-PLAYERS-TIME-EVENTS-PAGING-1` completed：玩家管理页在线时长列已压缩为“今天/昨天/N天前 HH:mm”短格式并保留旧数据回退，避免遮挡收入列；在线玩家表收入列顺序调整为“玩家收入 / 农场收入”；玩家活动改为每页 3 条分页，桌面下与右侧管理操作卡同高，移动端自然堆叠且无页面级横向溢出。未改后端 API、玩家轮询、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x1024 与 390x844，console error/warn 为空。

# FE-SETTINGS-API-PORT-REMOVE-1 状态
- `FE-SETTINGS-API-PORT-REMOVE-1` completed：设置与审计页“端口信息”卡片已移除只读“API 端口”字段，仅保留“面板端口 / VNC 端口 + 保存/刷新”。只改前端展示 JSX 和设置页端口行 CSS，未改 VNC 端口保存接口、权限判断、后端 API、Junimo 通信或轮询逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-SAVES-UPLOAD-BLUE-BG-1 状态
- `FE-SAVES-UPLOAD-BLUE-BG-1` completed：存档页上传横条背景已按用户反馈从羊皮纸虚线样式恢复为之前的蓝色天空版本（蓝色渐变 + 白色像素云 + 木色实线边框）。仅改 `StardewPanel.css` 视觉样式，上传弹窗、ZIP 预览、导入并启动、权限和禁用逻辑不变。已验证 `cd frontend; npm.cmd run build` 通过；QA mock 全壳截图确认上传条为蓝色背景且 console error/warn 为空。

# FE-SAVES-V02-PROTOTYPE-LAYOUT-1 状态
- `FE-SAVES-V02-PROTOTYPE-LAYOUT-1` completed：存档管理页已按 version-02 原型 `03-saves.png` 回正卡片位置和比例。激活存档区改为“信息卡 + 右操作卡”，存档库工具按钮上移到标题右侧，桌面主宽下三张存档卡同排，上传条与底部“自动备份策略 / 备份列表”双栏按原型落位。仅改前端 TSX/CSS 展示结构，创建/上传/选择/删除/导出/备份/恢复接口和权限逻辑不变。已验证 `cd frontend; npm.cmd run build` 通过；QA mock 全壳 1536x1024 对照原型、390x844 无横向溢出且 console error/warn 为空。

# FE-SETTINGS-PROTOTYPE-V02-LAYOUT-2 状态
- `FE-SETTINGS-PROTOTYPE-V02-LAYOUT-2` completed：设置与审计页已按 version-02 原型 `09-settings.png` 回正卡片结构和比例。左列为面板版本、用户管理、端口信息、其他设置，右列为安全与权限、审计日志、安全建议；版本卡补右侧图像槽，安全摘要改单列表，端口卡当时改三端口横排，后续 `FE-SETTINGS-API-PORT-REMOVE-1` 已移除重复的“API 端口”；审计首屏 7 条，安全建议改三条状态徽章。仅改前端页面与 CSS，未改 API、权限、轮询或后端逻辑。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器临时 mock QA 覆盖 1536x1024 与 390x844，console error/warn 为空、无页面级横向溢出，QA 临时文件已删除。

# FE-SERVER-PROTOTYPE-V02-LAYOUT-2 状态
- `FE-SERVER-PROTOTYPE-V02-LAYOUT-2` completed：服务器控制页已按 version-02 原型 `02-server.png` 回正卡片结构和比例。顶部摘要改为服务器专用整行大卡（状态/在线玩家/当前存档/主机农民/游戏日期 + 邀请码横条），中部恢复生命周期左列、快捷操作右列，快捷操作改为原型式浅色工具行，底部控制台命令横跨整行且终端满宽；移动端恢复单列顺序并无页面级横向溢出。未改 API、权限、轮询、Junimo 通信或后端逻辑。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器临时 mock QA 覆盖 1536x1024 与 390x760，console error/warn 为空，QA 临时文件已删除。

# FE-OVERVIEW-BANNER-SCENE-IMAGE2-1 状态
- `FE-OVERVIEW-BANNER-SCENE-IMAGE2-1` completed：总览页顶部农场横幅场景已替换为从 image2 原型 `01-overview.png` 裁出的运行时素材 `overview_banner_scene_image2.png`，不再由 CSS 田野/云层和旧 `sprite_farmhouse_scene.png` 叠加生成。裁切只包含农场场景，不含统计条或页面外壳；仅改前端 CSS 与静态素材，未动组件逻辑/API/后端。已验证新 PNG 预览正常，`cd frontend; npm.cmd run build` 通过。

# FE-PROTOTYPE-SHELL-ALIGN-1 状态
# FE-DIAGNOSTICS-PROTOTYPE-V02-LAYOUT-1 状态
- `FE-DIAGNOSTICS-PROTOTYPE-V02-LAYOUT-1` completed：诊断页已按 version-02 current-frontend-code 原型 `07-diagnostics.png` 回正卡片位置与比例。页面专属收紧主 frame inset，顶部状态横卡约 `975x160`，中部检查项/资源趋势回到等宽双列约 `482x392`，底部告警建议横跨全宽并留在首屏内；资源仪表卡改为标题在上、圆环居中、说明在下，趋势图压回原型短图。未改后端接口、权限、轮询或业务状态。已验证 `cd frontend; npm.cmd run build` 通过；Playwright + 本机 Chrome 回退 QA：1536x1024 与 390x844 无横向溢出、console error/warn 为空。

- `FE-PROTOTYPE-SHELL-ALIGN-1` completed：九页前端布局已对齐 image2 version-02 原型。根因是右信息栏(414px)/左导航(252px)过肥把主内容挤到 791px；已把 `--sd-opsrail-width` 收到 `clamp(268px,19vw,300px)`、`--sd-sidebar-width` 收到 `clamp(196px,14vw,216px)`，主内容区回到 937px，总览恢复 4 卡一行、控制区与邀请码并排。逐页修：服务器(生命周期|快捷操作并排、快捷操作改竖排)、任务日志(列表|详情两列)、玩家(表整行+活动|管理两列，逆转 FE-PLAYERS-LIST-LEFT-1)、诊断(检查信息单行不折)、设置(用户/审计表列不裁切)、存档(上传区改羊皮纸虚线)、模组(结果卡两列网格)、顶栏版本不折行。仅改 `StardewPanel.css`，未动逻辑/接口。已验证 `cd frontend; npm run build` 通过；mock-fetch harness + Playwright 1536 逐页对比原型、pageerror 为 0，真实登录态截图待补。

# FE-PLAYERS-LIST-LEFT-1 状态
- `FE-PLAYERS-LIST-LEFT-1` completed：玩家管理页桌面首屏已调整为左侧宽列显示在线玩家表、右侧窄列显示最近事件，取消旧规则导致玩家表落到右侧第三行的问题，减少中间空白；服务器信息（Junimo）保留为底部整行调试信息。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-DIAG-GAUGE-TOMIK-1 状态
- `FE-DIAG-GAUGE-TOMIK-1` completed：诊断页 CPU/内存/磁盘三资源圆环从铜钱 conic 样式改为 Tomik23 circular-progress-bar 风格（灰底环 + yellow→#ff0000 渐变圆头描边 + 中心百分比），纯 SVG 实现无新依赖；空态只画底环。已验证 `cd frontend; npm.cmd run build` 通过，Playwright 真实登录态 1366/390 截图正常、pageerror 为 0。

# FE-UNIFIED-CARD-PARCHMENT-TONE-1 状态
- `FE-UNIFIED-CARD-PARCHMENT-TONE-1` completed：总览统计卡当前的浅羊皮纸暖黄已提升为统一小卡片背景色，文件尾部覆盖 `--sd-save-card-bg` / `--sd-save-card-bg-strong`，所有复用统一小卡片变量的非模组页小框都会跟随；总览 `.sd-mc` 保持同色且无斜纹。未改卡片尺寸、边框、圆角、阴影、文字布局或业务逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-INSTALL-STEAM-AUTH-ICON-1 状态
- `FE-INSTALL-STEAM-AUTH-ICON-1` completed：安装页“Steam 认证”卡片中间占位图标和栏目标题小图标均已改为复用安装进度第三步的 `icon_install_step_steam_image2.png`，不再使用 CSS 渐变画出的蓝色 Steam 圆球。未改 Steam 认证、Steam Guard、扫码、日志或后端接口。已验证 `cd frontend; npm.cmd run build` 通过；真实安装页受登录页限制，登录态截图待补。

# FE-SETTINGS-FILL-GAP-1 状态
- `FE-SETTINGS-FILL-GAP-1` completed：设置页已从三段式布局改为左右两列堆叠，左列为“面板版本 / 用户管理 / 端口信息 / 其他设置”，右列为“安全与权限 / 审计日志 / 安全建议”，让端口信息和其他设置上移补足短列空位；`780px` 以下再切回单列。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-OVERVIEW-METRIC-CLEAN-BG-1 状态
- `FE-OVERVIEW-METRIC-CLEAN-BG-1` completed：总览页四个统计卡 `.sd-mc` 已移除斜向纸纹背景，并改为干净、偏浅的羊皮纸暖黄渐变；后续按反馈从偏白略微压黄，但不恢复旧的高饱和黄色。保留卡片尺寸、边框、角饰、文字布局和状态徽章。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-INSTALL-HERO-SCENE-REMOVE-1 状态
- `FE-INSTALL-HERO-SCENE-REMOVE-1` completed：安装页顶部状态横幅已移除右侧大农舍场景图 `.sd-install-farm-scene`，不再渲染安装页顶部的 `sprite_farmhouse_scene.png`；状态横幅改为“小土芽图标 + 状态信息”两列。未改安装状态、Steam 认证、日志、进度或后端接口。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-INVITE-CARD-COPY-ORDER-1 状态
- `FE-INVITE-CARD-COPY-ORDER-1` completed：邀请码卡片新增共享 `InviteCodeCard`，服务器摘要卡和总览页服务器控制区统一复用；复制按钮调整到刷新按钮左侧，仅在已有邀请码时渲染，未有码状态不保留空按钮列。总览页已移除旧 `sd-invite-panel` JSX、本地复制状态和独立 `handleCopy()`，后续邀请码展示/复制/刷新只维护一套组件。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器在未登录状态下确认服务器页与总览页应用壳非空、console error/warn 为空，登录态卡片截图待补。

# FE-SETTINGS-ACCOUNT-CARD-REMOVE-1 状态
- `FE-SETTINGS-ACCOUNT-CARD-REMOVE-1` completed：设置与审计页已移除顶部“当前账号”卡片，避免和顶栏用户入口重复；顶部摘要区从三卡改为“面板版本 / 安全与权限”两卡，并清理 `sd-settings-account-*` 死样式。登出仍保留在 Stardew Shell 顶栏，未改 session、权限、用户管理或审计日志逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-PAGE-HEADER-SHADOW-1 状态
- `FE-PAGE-HEADER-SHADOW-1` completed：Stardew 各路由页头已移除标题文字、导航图标和右侧虚线分隔的阴影背景，只保留干净标题、图标和分隔线；未改页面结构、按钮、卡片布局、API 或业务逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-PAGE-TOP-ALIGN-1 状态
- `FE-PAGE-TOP-ALIGN-1` completed：Stardew 各 routed 页面已通过文件尾部 CSS 兜底统一贴齐主内容 frame 顶部，覆盖任务、诊断、安装、设置等页面后置 `padding` 造成的顶部下沉；未改页面结构、卡片布局、API 或业务逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-SERVER-ACTION-CARDS-1 状态
- `FE-SERVER-ACTION-CARDS-1` completed：服务器控制页生命周期控制卡已下移到顶部摘要卡下方左侧，快捷操作卡放到同一行右侧；快捷操作按钮统一叠加 `.sd-btn--lg`，与生命周期按钮共用 40px lg 尺寸令牌，并移除原 64px 卡片式按钮和伪图标。已验证 `cd frontend; npm.cmd run build` 通过；真实路由受登录页限制，仅完成应用非空、无框架覆盖、console error/warn 为空的烟测，未完成登录态截图验证。

# FE-SERVER-INVITE-IN-SUMMARY-1 状态
- `FE-SERVER-INVITE-IN-SUMMARY-1` completed：服务器控制页移除中部独立“邀请代码”卡片，邀请码复制/刷新入口收敛到顶部服务器摘要卡；“刷新”按钮位于“邀请加入码”显示区右侧，运行中/启动中可用，未运行时禁用。删除 `ServerControlPage` 第二套复制状态和 `handleCopy()`，后续统一复用 `ServerSummaryCard`；“全服消息”在删除邀请码卡片后改为横跨整行。已验证 `cd frontend; npm.cmd run build` 通过；真实路由受登录页限制，内置浏览器确认应用加载非空且 console error/warn 为空，未完成登录态截图验证。

# NEXUS-ERROR-TEXT-1 状态
- `NEXUS-ERROR-TEXT-1` completed：Nexus 搜索/安装错误响应中的中文 mojibake 已修复，`nexus_request_failed` 等错误不再显示为 `璇锋眰澶辫触`。前端 `errorCodeMap` 同步增加 Nexus 错误码中文兜底，后续即使后端 message 异常也会优先显示正常中文。已验证 `cd backend; go test ./internal/web -run TestWriteNexusErrorMessagesAreReadableChinese` 和 `cd frontend; npm.cmd run build` 通过。

# FE-BTN-UNIFY-1 状态
- `FE-BTN-UNIFY-1` completed：9 个页面按钮与操作区统一化。按钮尺寸收敛为 lg 40px / md 28px / sm 22px 三档令牌（`.sd-btn--lg` / 默认 / `.sd-btn--sm`），语义色收敛为绿(主)/棕(次)/红(危险)，删除 `.sd-btn-blue`、`.sd-btn-xs` 与死样式 `sd-btn-gold`/`sd-btn-red`；危险确认弹窗统一红色确认键与"取消+确认"顺序；新增共享 `.sd-actionbar` / `.sd-rowactions` 操作区布局类；删除全部逐页按钮尺寸覆写与 JSX 内联尺寸；统一"刷新/保存/X并启动"等文案；修复总览页无邀请码时邀请面板被 1px 网格列挤成竖排的问题。已验证 `cd frontend; npm.cmd run build` 通过，并用 Playwright 真实登录态对 9 页 × 4 视口做改前/改后截图对比，console 无新增报错。

# FE-MAIN-PAGE-FRAME-SLICES-1 状态
- `FE-MAIN-PAGE-FRAME-SLICES-1` completed：所有 Stardew 页面共用的 `.sd-main` 主内容 frame 已从整图 `100% 100%` 拉伸改为 image2 空框 9 切片平铺。新增四角、上下左右边 tile 和中心羊皮纸 tile，运行时用 9 层 CSS background 实现四角固定、上下 `repeat-x`、左右 `repeat-y`、中心 `repeat`，窗口缩放时边框纹理不再被拉伸；`.sd-main-scroll` 统一滚动视口保持不变。已验证 `cd frontend; npm.cmd run build` 通过，并用内置浏览器临时 QA 页检查 1280x720 与 390x760 下背景层/重复规则正确、无横向溢出、滚动正常、console error/warn 为空。

# FE-MODS-DEPENDENCY-POPOVER-1 状态
- `FE-MODS-DEPENDENCY-POPOVER-1` completed：下载模组页 Nexus 搜索结果的前置信息弹层已改为受控按钮 + 弹层，修复固定高度搜索卡片裁切导致前置信息看起来无法展开的问题；点击信息弹层外部会自动收起，不再需要回点前置按钮。已验证 `cd frontend; npm.cmd run build` 通过，并用内置浏览器临时 QA 页检查 1280x720 与 390x760 展开/外部点击收起、无水平溢出、console error/warn 为空。

# JOB-DISPLAY-NAME-1 状态
- `JOB-DISPLAY-NAME-1` completed：jobs 表新增 `display_name`，任务 API/SSE 返回 `displayName`；Nexus/远程 Mod 安装任务会写入 `mod_nexus_install · <Mod 名>` / `mod_remote_install · <Mod 名>`，前端任务页、右栏进行中、右栏近期任务和总览近期事件均优先展示该字段，解决并行依赖安装时多个任务同名不可区分的问题。已验证后端 storage/jobs/web 测试和前端构建通过。

# MODUPLOAD-DUPLICATE-CODE-1 状态
- `MODUPLOAD-DUPLICATE-CODE-1` completed：Mod ZIP 上传遇到已安装相同 `UniqueID` 时，后端响应错误码改为 `mod_exists`，避免前端显示成“Mod ZIP 无效/解析失败”；真正坏 ZIP、无 manifest、XNB 替换包等仍保持 `invalid_mod_zip`。已验证 `cd backend; go test ./internal/web -run "TestModUpload"` 通过。

# FE-OPSRAIL-DOWNLOAD-PROGRESS-1 状态
- `FE-OPSRAIL-DOWNLOAD-PROGRESS-1` completed：右侧 OpsRail“进行中”卡已接入远程 Mod 安装下载日志进度，`mod_remote_install` / `mod_nexus_install` 会解析 `下载进度：已下载 ...（xx.x%）` 并映射到右栏进度条；扩展 batch 一旦返回新的 `jobId` 会立即刷新 jobs，让任务尽快出现在右栏，Premium 安装路径也同步主动刷新。已验证 `cd frontend; npm.cmd run build` 通过。

# NEXUS-EXT-DOWNLOAD-GUARD-1 状态
- `NEXUS-EXT-DOWNLOAD-GUARD-1` completed：远程 Mod 安装任务新增下载可见进度和错误分界日志，显示连接远程下载服务器、HTTP 响应、Content-Type、压缩包大小、已下载/总量/剩余/百分比；无总量时显示已下载大小。浏览器扩展提交前强制校验 Nexus CDN `.zip`，后台页未真正拿到 ZIP 时不再创建面板安装任务；后端收到 `text/html` 会立即失败并提示这是网页不是 ZIP。已验证后端相关测试与扩展 JS 语法检查通过。

# FE-SERVER-PROTOTYPE-IMAGE2-1 状态
- `FE-SERVER-PROTOTYPE-IMAGE2-1` completed：服务器控制页已按 image2 原型重皮肤为羊皮纸控制台结构，包含当前状态大卡、生命周期控制卡、邀请代码、全服消息、黑色命令终端和快捷操作条；原型图未作为运行时资源，纸纹、铜边、内阴影、绿屏、终端和分隔线均由 CSS 实现。业务逻辑/API/权限/disabled 状态保持不变，按钮和标题图标复用既有 Stardew PNG/图标素材。已验证 `cd frontend; npm.cmd run build` 通过，并用临时 QA 页检查 1280x900 与 390x760 无横向溢出、按钮文字不溢出、命令执行输出可读。

# FE-DIAGNOSTICS-IMAGE2-ICONS-1 状态
- `FE-DIAGNOSTICS-IMAGE2-ICONS-1` completed：诊断页首轮 CSS 自绘图标已替换为 image2 风格透明 PNG 素材。新增 `frontend/public/assets/stardew/ui/diagnostics/`，包含 4x5 生成 sprite sheet 与状态盾牌、三色宝石、检查项、建议区、资源趋势、实时绿点、导出下载等 20 个单图；运行时使用单图背景，不使用整页原型图或整块截图。已验证 `cd frontend; npm.cmd run build` 通过，内置浏览器临时 QA 页检查 1280x900 与 390x760 无横向溢出、按钮不溢出、console error/warn 为空，可见诊断页图标背景均来自 `/assets/stardew/ui/diagnostics/`。

# FE-DIAGNOSTICS-PROTOTYPE-IMAGE2-1 状态
- `FE-DIAGNOSTICS-PROTOTYPE-IMAGE2-1` completed：诊断与健康页已按 image2 原型重皮肤为顶部双操作、系统状态横卡、三计数卡、检查项表、资源趋势卡和底部告警建议区；原型图未作为运行时背景或整块资源引用，业务逻辑/API/权限保持不变。已验证 `cd frontend; npm.cmd run build` 通过，并用内置浏览器临时 QA 页检查 1280x900 与 390x760 无横向溢出、无面板重叠、按钮文字不溢出、console error/warn 为空。注：首轮 CSS 自绘图标已在 `FE-DIAGNOSTICS-IMAGE2-ICONS-1` 中替换为 image2 PNG 素材。

# FE-SETTINGS-PROTOTYPE-IMAGE2-1 状态
- `FE-SETTINGS-PROTOTYPE-IMAGE2-1` completed：设置与审计页已按 image2 `09-settings - 副本.png` 原型重皮肤为顶部三卡、中部用户/审计双栏、底部设置/安全建议双栏结构。原型图未作为运行时背景或整块素材引用；纸纹、铜边、角钉、表头、分隔线和提示面板均为 CSS 实现，按钮/标题图标复用现有 Stardew PNG 素材。功能逻辑、API 调用、权限判断、loading/error/empty/disabled 状态保留。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器真实路由停在登录页，因此使用已删除的临时 `settings-qa.html` 加载同 CSS/素材/同结构 DOM 验证 1280x900 桌面、点击新建用户展开态、390x760 窄屏和底部状态均无页面级横向溢出，console error/warn 为空。

# FE-INSTALL-PROTOTYPE-IMAGE2-1 状态
- `FE-INSTALL-PROTOTYPE-IMAGE2-1` completed：安装页已按 image2 `08-install - 副本.png` 原型重皮肤为顶部状态横幅、五步安装时间线、安装配置/Steam 认证/安装日志三栏工作区。原型图未作为运行时资源；纸张纹理、边框、分隔线、步骤卡、进度条和日志终端均为 CSS 实现，按钮继续复用既有 PNG 按钮体系。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器临时 QA 覆盖 1280x900 认证态、未安装态点击展开表单、390x760 窄屏无横向溢出，console error/warn 为空。

# FE-OPSRAIL-AUTO-COLLAPSE-1 状态
- `FE-OPSRAIL-AUTO-COLLAPSE-1` completed：Stardew Shell 已按主内容预计宽度自动收起右侧 OpsRail，展开态主内容低于 `820px` 收起、收起后高于 `880px` 再展开；总览页 1180px 响应式规则补为 `.sd-main-scroll` 容器查询，避免窗口缩放时中间内容被右栏挤压后仍按桌面布局排布。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器 QA 覆盖 1200x760 自动收起、1200→1600 无刷新自动展开、390x760 移动布局，均无横向溢出且 console error/warn 为空。

# FE-OVERVIEW-PROTOTYPE-IMAGE2-1 状态
- `FE-OVERVIEW-PROTOTYPE-IMAGE2-1` completed：总览页已按 image2 原型重皮肤为农场横幅 + 羊皮纸控制台 + 四摘要卡 + 三列清单结构；页面背景、纸纹、卡片、分隔线、绿屏邀请码和状态标签均由 CSS 实现，未把原型图作为运行时资源。按钮和小图标复用现有 Stardew PNG/图标素材，业务逻辑/API/权限不变。已验证 `cd frontend; npm.cmd run build` 通过，并用内置浏览器临时 QA 页检查 1280x900 与 390x760 无横向溢出、按钮文字不溢出、console error/warn 为空。

# FE-SAVES-SIMPLIFY-3 状态
- `FE-SAVES-SIMPLIFY-3` completed：存档页页头精简为左上角纯文字标题（去框去描述）；全部按钮换回面板既有 PNG 素材按钮（`sd-btn-green/tan/delete`），撤销自绘 `.sd-pxbtn`；备份与恢复规则压缩为单行控件条，说明文字移入悬浮提示，文案精简。已验证前端构建通过。

# FE-SAVES-MOCKUP-2 状态
- `FE-SAVES-MOCKUP-2` completed：存档页按用户完整设计稿改版：眉标行、激活卡图标字段双列表、存档库只显示非激活存档（选择/删除糖块按钮）、横排虚线创建卡、上传横条（📮 + 上传并启动）、备份六列表格（状态徽章 + 5 行折叠）；新增纯 CSS `.sd-pxbtn` 糖块按钮体系；导出/手动备份/启动收敛到激活卡。已验证前端构建通过。

# FE-SAVES-PROTO-CSS-1 状态
- `FE-SAVES-PROTO-CSS-1` completed：存档页按 image2 原型重构为纯 CSS 皮肤（无图片资源）：铜框铆钉激活卡、双列虚线字段、圆角存档卡、虚线创建卡、像素云天空上传横条、表头带备份表格；创建/上传入口从页头移入创建卡与上传横条，业务逻辑未变。已验证前端构建通过。
- `FE-SAVES-PROTO-CSS-1` follow-up：激活存档卡预览槽接入真实农场地图，按存档 `farmType` 复用新建游戏的 `/assets/stardew/new-game/farms/<farmType>.png` 素材，未知类型回落为空羊皮纸块。已验证前端构建通过。

# FE-RIGHT-RAIL-ACTIVE-CARD-1 状态
- `FE-RIGHT-RAIL-ACTIVE-CARD-1` completed：右栏"进行中"卡接入自动关机/自动开机倒计时（restart-schedule）、定时备份倒计时（backups/policy，仅管理员行）和运行中任务进度条（同类型历史耗时中位数估算，封顶 95%）；倒计时行为蓝色档、每秒走动，任务完成后行自动消失；dashboard 30s 轮询同时刷新 jobs 兜底调度器触发的任务。已验证前端构建通过。

# FE-RIGHT-RAIL-HEALTH-STATS-1 状态
- `FE-RIGHT-RAIL-HEALTH-STATS-1` completed：右栏"系统健康"卡按原型接入真实数据：CPU/内存/磁盘使用率（复用 `GET /api/instances/:id/metrics`，5s 轮询，圆润胶囊形进度条）、在线玩家（`onlineCount/当前存档 MaxPlayers`）、网络延迟（metrics 请求前端往返耗时）；按钮改为"查看详情 →"。已验证前端构建通过。
- `FE-RIGHT-RAIL-HEALTH-STATS-1` follow-up：五行增加阈值配色，浆果点/进度条/数值随数值绿→黄→红（使用率 60/85、延迟 100/300ms 分档），在线玩家为 0 时显示红色。已验证 `npm run build` 通过。

# PLAYERS-MAXPLAYERS-1 状态
- `PLAYERS-MAXPLAYERS-1` completed：`stardew_junimo` driver 的 `ListPlayers` 现在用当前存档 `server-settings.json` 的 `Server.MaxPlayers` 兜底 `maxPlayers` 字段（junimo info 解析出的值仍优先），玩家接口在服务器未运行或 info 不含上限时也能返回人数上限。已验证 `go test ./internal/games/stardew_junimo/` 通过。

# FE-RIGHT-RAIL-CARD-FIX-1 状态
- `FE-RIGHT-RAIL-CARD-FIX-1` completed：右栏三卡（系统健康/进行中/近期任务）去掉内部滚动与等高拉伸逻辑，行高改为随内容、与左侧栏按钮一致只随栏宽缩放；四角藤蔓拉伸修复为每卡单一缩放系数换算 border-width/margin，角部切片横纵等比。已验证 `npm run build` 通过。详见 `docs/03-frontend.md` 与 `docs/frontend-handoff/frontend-handoff-2026-07-02.md` 对应条目。

# FE-MAIN-PAGE-FRAME-3 状态
- `FE-MAIN-PAGE-FRAME-3` completed：按用户红框示意，把所有 Stardew 页面共用的中间内容滚动视口重定界为 frame 内侧大矩形。`.sd-main` 四向 inset 改为 top `5.2%`、right `5%`、bottom `6%`、left `4%`（带移动下限和桌面上限），`.sd-main-scroll` 继续在该红框边界内滚动，所有路由统一生效。已验证 `cd frontend; npm.cmd run build` 通过；1750x1113 QA 下主内容区 `1068x1033` 时 inset 为 `55.5/53.4/64.1/42.7px`，390x760 下无横向溢出且滚动正常。

# FE-MAIN-PAGE-FRAME-2 状态
- `FE-MAIN-PAGE-FRAME-2` completed：修复中间内容区 frame 裁切方案导致模组页无法滚动的回归。`.sd-main` 继续以上一步界定的内侧 frame 边界作为裁切框，负责背景、内框 padding 和 `overflow:hidden`；新增 `.sd-main-scroll` 作为统一滚动视口，负责滚轮/触控板滚动和隐藏原生滚动条；各路由页面继续返回普通 `.sd-page`，避免复杂页面布局被滚动容器规则影响。已验证 `cd frontend; npm.cmd run build` 通过，内置浏览器 QA 在 1280x720 和 390x760 下滚动正常、无横向溢出、console error/warn 为空。

# FE-TOPBAR-IMAGE2-REGEN-1 状态
- `FE-TOPBAR-IMAGE2-REGEN-1` completed：按用户要求用 image2 参考 `01-overview.png` / `Top bar.png` 重新生成顶栏拆分素材，替换上一批不合格 topbar 资源。外壳为 `topbar_shell_left.png` + `topbar_shell_middle_tile.png` repeat-x + `topbar_shell_right.png`，控件框为独立 `*_9slice.png`，鸡/农场/头像/叶子/绿点/登出/下拉箭头为独立 v2 图标；文字继续由 React 渲染。右端缺失问题已通过重新归一化 `topbar_shell_right.png` 到完整高度修复。已验证前端构建通过、1920x900 顶栏无横向溢出、390x760 移动端顶栏策略正常。

# FE-RIGHT-RAIL-TOP-FROM-BOTTOM-1 状态
- `FE-RIGHT-RAIL-TOP-FROM-BOTTOM-1` completed：右侧栏上段素材已改为“底段去南瓜/向日葵后旋转 180 度”的版本。处理过程保留原底段文件不变，仅覆盖运行时使用的 `right_rail_shell_top_line_image2.png`，并按新图实测横梁范围同步更新 `.sd-opsrail::before` 的定位/尺寸常量和 stack 顶部留白。已验证前端构建通过，PNG 为 `1871x840` RGBA，alpha 范围 `0..255`，人工预览确认上段不再含南瓜/向日葵且横梁无破洞。

# FE-ASSET-SIDEBAR-3PIECE-1 状态
- `FE-ASSET-SIDEBAR-3PIECE-1` completed：已从 image2 左侧栏生成三段式可复用背景素材 `panel_side_rail_top_image2.png`、`panel_side_rail_middle_tile_image2.png`、`panel_side_rail_bottom_image2.png`。顶部段保留木质顶部外框和横梁，中段按方案 A 调整为可 `repeat-y` 的纯连续木板/立柱 tile，不含横向分隔线、按钮槽位、暗条、分层隔板或固定行高结构；底部段保留书架、灯笼、盆栽、紫水晶和书本/盒子装饰。三张均为 RGBA 透明 PNG，不含导航按钮、中文文字、菜单图标或按钮高亮。本次仅入库生产素材，尚未改运行时代码。

# FE-SIDEBAR-ROW-BG-1 状态
- `FE-SIDEBAR-ROW-BG-1` completed：左侧栏运行时已接入三段式背景素材，替换整张空壳背景 `100% 100%` 拉伸。导航 DOM 新增 `.sd-nav-list` / `.sd-nav-row`，每个按钮背后的木板段改由行容器渲染并随按钮布局走，解决背景固定槽位在放大/缩小时与按钮错位的问题。已验证 `cd frontend; npm.cmd run build` 通过；浏览器登录态侧栏截图验证受当前本地账号不可用阻塞。
- `FE-SIDEBAR-ROW-BG-1` follow-up：为避免 `.sd-nav-list` 滚动条压缩行容器宽度导致中段素材右边框左移，完整 `panel_side_rail_middle_tile_image2.png` 改为只在 `.sd-sidebar` 外层绘制，`.sd-nav-row` 只保留上下阴影槽位感。已验证 `cd frontend; npm.cmd run build` 通过。
- `FE-SIDEBAR-ROW-BG-1` follow-up：导航按钮宽度改用 `min(86cqi, 210px)`，以侧栏容器宽度为基准，不再因 `.sd-nav-list` 滚动条或行容器宽度变化而缩小。已验证 `cd frontend; npm.cmd run build` 通过。
- `FE-SIDEBAR-ROW-BG-1` follow-up：桌面 `.sd-nav-list` 保留可滚动但隐藏滚动条，避免滚动条预留宽度把导航行居中区域压窄、导致按钮整体左移。已验证 `cd frontend; npm.cmd run build` 通过。

# SERVER-SAY-1 状态
- `SERVER-SAY-1` completed：服务器控制页喊话入口已打通。后端 `POST /commands/say` 校验消息后写入 `.local-container/control/commands/broadcast*.json`，由 `StardewAnxiPanel.Control` 在游戏 tick 中调用 Stardew multiplayer chat message 向所有在线玩家发送 `[Panel]` 公告；不依赖不存在的上游 `say` SMAPI 命令。已验证当前 Junimo 镜像包含上游 `/ws chat_send` relay，但面板暂用控制模组文件通道以保持容器网络和部署兼容。

# FE-ASSET-RIGHT-RAIL-SHELL-3PIECE-1 状态
- `FE-ASSET-RIGHT-RAIL-SHELL-3PIECE-1` completed：已用 image2 重新生成右侧栏三段空壳与三张卡片九宫格素材：`right_rail_shell_top.png`、`right_rail_shell_middle_tile.png`、`right_rail_shell_bottom.png`、`right_card_health_9slice.png`、`right_card_progress_9slice.png`、`right_card_recent_9slice.png`。三段 shell 分别只保留顶部横梁/上边框/藤蔓角饰、左右木柱 + 纯木板 repeat-y 中段、底部木梁 + 南瓜 + 向日葵 + 藤蔓；三张卡片框不含标题、图标、CPU/内存/磁盘文字、进度条、任务列表或按钮文字，内容继续由前端渲染。已用 RGBA/alpha/洋红残留检查和棋盘底人工预览验证。

# FE-RIGHT-RAIL-SPLIT-ASSETS-1 状态
- `FE-RIGHT-RAIL-SPLIT-ASSETS-1` completed：Stardew Shell 右侧 OpsRail 已从整张 `panel_right_rail_image2.png` 背景方案迁移为拆分素材组合。运行时使用右栏空壳、外层边框、三张九宫格卡片框和三枚标题图标分层渲染；中文标题、健康摘要、任务列表、按钮文字和 Mod 重启提示继续由 React 数据层渲染，`.sd-dot*` 状态点复用原 CSS 动态效果。已验证 `cd frontend; npm.cmd run build`、1280×720 总览页三卡可见、右栏“查看诊断/查看全部任务”跳转成功、390×760 移动端右栏隐藏且无水平溢出，console 无 error/warn。

# FE-SAVE-START-NAV-1 状态
- `FE-SAVE-START-NAV-1` completed：从存档页发起的选择并启动、创建并启动、上传并启动成功创建任务后会跳转总览页，并触发新邀请码等待态；不再默认跳到任务页。

# FE-QUICK-BACKUP-1 状态
- `FE-QUICK-BACKUP-1` completed：服务器控制页“快捷操作”里的“备份已保存进度”已接入 `POST /api/instances/:id/saves/:name/backup`，按当前激活存档创建手动备份；“保存世界 / 立即保存”占位已移除，避免误导用户以为面板能强制保存尚未写盘的游戏进度。

# SCHEDULED-RESTART-DESIGN-1 状态
- `SCHEDULED-RESTART-1` completed：计划重启第一阶段已接入。管理员可在服务器控制页配置每日关闭/开启维护窗口，后端持久化到 `restart_schedules`，后台调度器提前广播、关闭前备份已落盘存档，并复用现有 Stop/Start 生命周期 job；暂不实现强制保存世界。

# FE-LIFECYCLE-WAIT-1 状态
- `FE-LIFECYCLE-WAIT-1` completed：总览页和服务器控制页的启动/重启按钮现在会在请求发出后显示旋转圆圈与“启动中…”，并持续等待新邀请码出现；停止按钮也会在停止过程中显示旋转圆圈与“停止中…”，直到状态完全停止后才恢复启动按钮。

# PLAYERSYNC-PACK-10 状态
- `PLAYERSYNC-PACK-10` completed：玩家同步安装脚本彻底禁用终端进度渲染，移除 `Write-Progress` 并设置 `$ProgressPreference = "SilentlyContinue"`；进度仅写入日志，控制台只显示独立任务行和最终摘要。当前测试解压包已热修并在真实游戏目录验证不再出现中文双字重叠。
# PLAYERSYNC-PACK-9 状态
- `PLAYERSYNC-PACK-9` completed：玩家同步安装完成摘要移除单独的 `SMAPI 路径`，只保留可直接复制到 Steam 的 `Steam 启动项文本`。当前测试解压包已热修并真实验证。
# PLAYERSYNC-PACK-8 状态
- `PLAYERSYNC-PACK-8` completed：玩家同步安装脚本移除自绘 carriage-return 动态进度行，避免中文终端输出重复字和残留；控制台只输出独立任务日志，进度只写入日志文件。安装摘要单独列出完整 `Steam 启动项文本`，当前测试解压包已热修并真实验证。
# PLAYERSYNC-PACK-7 状态
- `PLAYERSYNC-PACK-7` completed：玩家同步安装脚本新增 Mod 目录内容指纹比对。目标 Mod 与 payload 完全一致时跳过备份和复制，并在 `installed.json.mods[]` 写入 `skippedIdentical=true`；任意文件内容、大小或路径不同都会触发备份旧目录并覆盖安装。全部跳过且未真实备份时 `backupId=null`。当前测试解压包已热修并验证。
# PLAYERSYNC-PACK-6 状态
- `PLAYERSYNC-PACK-6` completed：玩家同步安装脚本的文本进度条改为单行动态刷新。`Show-InstallProgress` 使用 `[Console]::Write([char]13...)` 原地更新当前进度，普通安装事件打印前会清除进度行、打印后重绘，避免 checksum 和 Mod 复制阶段刷出大量重复进度行。当前测试解压包已热修。
# PLAYERSYNC-PACK-5 状态
- `PLAYERSYNC-PACK-5` completed：玩家同步包安装 SMAPI 时改为调用随包官方 Windows 安装器 `internal/windows/SMAPI.Installer.exe`，传入 `--install --game-path "<Stardew Valley>" --no-prompt`；脚本不再直接解包 `install.dat`，也不做本机 `.NET` / `runtimeconfig` 特调。安装器超过 120 秒未退出时会提示玩家检查安装器窗口是否在等待按键/输入。
# PLAYERSYNC-PACK-4 状态

- `PLAYERSYNC-PACK-4` completed：玩家同步安装脚本新增进度条，使用 PowerShell `Write-Progress` + 文本进度行显示安装阶段；checksum 按文件数推进，Mod 安装按待复制 Mod 数推进，SMAPI 阶段显示解压安装包、释放官方安装文件和完成。

# PLAYERSYNC-PACK-3 状态

- `PLAYERSYNC-PACK-3` completed：修复玩家安装脚本对 `[CP]` 等带方括号 Mod 路径的误判。checksum 校验、payload source 检查、目标 Mod 检查、卸载恢复检查都改用 PowerShell `-LiteralPath`，避免把 `[CP] MultipleConstructionOrders` 当成通配符字符集。

# PLAYERSYNC-PACK-2 状态

- `PLAYERSYNC-PACK-2` completed：玩家同步 ZIP 已升级为可执行安装包结构，包含 `安装玩家同步包.bat`、`卸载本同步包.bat`、`tools/`、`payload/mods/`、`payload/smapi/`、`pack-manifest.json` 和 `checksums.sha256`。玩家端脚本会校验 payload、定位 Stardew Valley、备份同名 Mod、复制本包 Mod、尽力设置 Steam 启动项，并把安装状态写入游戏目录 `.anxi-sync/`。SMAPI 采用服务端随包优先策略：导出时若实例目录下已有 `SMAPI*.zip` 则随包携带并校验，否则继续导出 Mod 包并提示玩家自行安装 SMAPI。

# NEXUS-SMAPI-THUMB-1 状态
- `NEXUS-SMAPI-THUMB-1` completed：虚拟内置 SMAPI 条目现在带 Nexus:2400 元数据，`GET /mods` 会通过现有 Nexus GraphQL 补全链路为它缓存并返回缩略图；同包内容包也会从同 Nexus ID 的完整缓存继承 `pictureUrl`，避免半截来源缓存挡住图片。前端继续统一使用 `pictureUrl`，失败时显示来源文字占位。

# MODORIGIN-1 状态
- `MODORIGIN-1` completed：后端区分 Mod 自己的 `nexusModId` 与同包来源 `originNexusModId`，手动上传 Nexus 多 Mod ZIP 时会把 `[CP]` 内容包标记为“来源 N站包，随主 Mod 安装”，并继承来源包缩略图；前端已按 `来源：N站 + Nexus:<id>` / `来源：N站包 + 随 <名称> 安装` 展示。同包 Mod 在已安装列表相邻显示，删除任意成员时弹窗提示并由后端一次性捆绑删除整组。

# MODSYNC-AUTO-1 状态
- `MODSYNC-AUTO-1` completed：玩家同步分类增加自动识别默认值：面板控制组件为服务器专用，SMAPI 内容包和第三方 Mod 默认玩家需同步，并在 `syncNote` 提供自动识别说明；分类下拉改为任意登录用户可修改，仍只写面板自有 `mod-sync.json`。

# NEXUS-INSTALLED-1 状态
- `NEXUS-INSTALLED-1` completed：添加模组页的已安装网格只展示 Nexus 视角数据，隐藏纯本地文件项和服务端控制组件；SMAPI 虚拟项按 Nexus:2400 展示，跳转按钮指向 Nexus 页面，无图时使用来源文字占位而不是文件夹图标。

# MODZIP-3 状态
- `MODZIP-3` completed：Mod manifest 解析兼容 UTF-8 BOM，修复部分 Nexus 原包中 `manifest.json` 以 BOM 开头导致上传显示 `Mod ZIP 无效` 的问题；不放宽非法 JSON 校验。

# MODZIP-4 状态
- `MODZIP-4` completed：Mod manifest 解析兼容 JSONC 风格注释和尾随逗号，修复 SpaceCore 等 Nexus 原包通过远程 CDN 安装时因 `manifest.json` 含 `//` 注释而失败的问题；字符串里的 URL 不会被误处理，ZIP 安全校验和 SMAPI 必填字段规则不变。

# MODDEPS-1 状态
- `MODDEPS-1` completed：后端已解析 SMAPI `Dependencies` 和 `ContentPackFor` 并通过 `GET /mods` 返回 `dependencies[]`；前端已在已安装 Mod 卡片底部给普通用户展示短名“前置：...”标签，完整依赖信息放在悬浮提示中。当前不自动安装依赖，也不判断缺失/版本不满足；完整依赖检查仍列为后续能力。

# MODUPLOAD-2 状态
- `MODUPLOAD-2` completed：Mod 上传弹窗和后端接口已支持一次选择并上传多个 `.zip`。后端按批次逐个导入，任意 ZIP 失败会回滚本次请求已导入的前序 Mod，成功时返回所有导入 Mod；`restartRequired` 继续遵循停服上传不额外提示重启的语义。前端 `ModsPage` / 旧 `ModsSection` 均已改为多文件选择，`uploadMod` 仍保留为兼容封装。

# NEXUS-META-1 状态
- `NEXUS-META-1` completed：后端已通过 Nexus GraphQL v2 无 Key 按 `gameId=1303 + modId` 拉取展示元数据；数字 ID 搜索、手动上传后缩略图补全都不再依赖 Nexus API Key。`GET /mods` 会对带 `UpdateKeys: ["Nexus:<id>"]` 的本地 Mod 自动补齐 Nexus 卡片字段并写入 sidecar 缓存。后续优化可考虑后台异步刷新、缓存过期时间，以及 CurseForge/ModDrop/GitHub 等多来源元数据补全。

# MODRESTART-1 状态
- `MODRESTART-1` completed：停服状态下上传/删除/安装 Mod 不再设置“需要重启”标记；停止服务器和停服 Mod 写入会清理历史标记，前端上传成功文案改为下次启动自动加载。

# MODZIP-1 状态
- `MODZIP-1` completed：Mod ZIP 上传增加 XNB 替换包识别和友好错误提示。当前仍不安装 XNB 内容替换包，只提示用户该类 Nexus 下载不是 SMAPI Mod，不能放入服务器 `Mods` 目录。

# MODZIP-2 状态
- `MODZIP-2` completed：Mod ZIP 上传支持 Nexus 常见单层外壳目录包，能自动剥离外壳并导入内部带 `manifest.json` 的真实 SMAPI Mod 子目录；上传、Nexus 一键安装和远程 ZIP 安装都复用该能力，不再要求用户手动解压重打包。
- 后续仍可优化：缺失依赖检测；更复杂安装说明型 ZIP 的识别和人工确认。

# MODSEARCH-1 / NEXUS-PAGED-1 状态
- `MODSEARCH-1` reverted：统一搜索/统一安装骨架已撤回，后端 `/mods/search` 与 `/mods/search/install` 不再注册，`mod_search.go` 和对应测试已删除。
- `NEXUS-PAGED-1` completed：当前 Mods 下载页只保留 Nexus 专用搜索/安装路径，支持默认热门、GraphQL 下载量排序、分页、数字 ID 查询和 Nexus 一键安装。
- 后续仍可优化：如重新做多来源搜索，需要重新设计接口；候选包括 StardewModDataset 本地/缓存索引、CurseForge Core API、GitHub Release asset、ModDrop 稳定下载来源、provider 去重排序和文件选择 UI；7z/rar 安全解压仍未开放。

# REMOTE-MOD-1 状态
- `REMOTE-MOD-1` completed：管理员可在 Mods 下载页粘贴 `nxm://` 或 `.zip` 直链创建 `mod_remote_install` job；NXM 链路支持非 Premium Nexus 用户通过 `key/expires` 获取 API 下载链接；直链/CDN 链路直接下载 ZIP 并复用现有安全导入，来源文案已覆盖 ModDrop/GitHub/CurseForge。
- 后续仍可优化：接入 CurseForge API Key 与 `download-url` 真一键；GitHub Release asset 安装；StardewModDataset 本地索引用于依赖与搜索；ModDrop 稳定下载来源识别；7z/rar 解压安全支持；多文件 Nexus/CurseForge/ModDrop 文件选择 UI。

# NEXUS-EXT-1 状态
- `NEXUS-EXT-1` prototype：新增 `browser-extensions/nexus-slow-installer` 私用浏览器扩展实验包。扩展可在 Nexus 文件页捕获慢速下载生成的临时 CDN ZIP 链接，并提交给现有 `POST /api/instances/:id/mods/remote/install` 远程安装接口；当前未集成进 ModsPage，也未新增扩展专用 token 后端接口。
- 后续仍可优化：在 ModsPage 增加“扩展安装”入口并带上下文打开 Nexus 文件页；新增扩展配对 token 和更窄 host 权限；扩展安装任务状态回传面板 UI；多文件选择和依赖链自动引导。

# NEXUS-3 状态

- `NEXUS-3` completed：Nexus Mods 无 Key GraphQL 搜索、v1 REST/下载链路 Key-gated、一键安装、`mod_nexus_install` job 进度、下载安装后复用现有 ZIP 安全导入、已安装 Mod Nexus 元数据 sidecar、前端搜索/已安装同款卡片展示已完成。
- 后续仍可优化：真实 Nexus 权限差异（手动下载限制、会员下载、OAuth）下的错误提示细分；多文件 Mod 的文件选择 UI；依赖/更新检查。

# FE-SIDEBAR-SPLIT-ASSETS-1 状态
- `FE-SIDEBAR-SPLIT-ASSETS-1` completed：Stardew Shell 左侧栏已从整张带文字按钮的 `panel_side_rail_image2.png` 透明热区方案迁移为拆分素材组合。桌面端使用左栏空壳作为唯一背景并填满侧栏格子，default / hover / active 三张按钮底图分别承担未选中、悬停、当前页状态，9 个独立导航图标和 React 中文 label；保留现有 9 路由、点击跳转、active、高亮、focus-visible 和移动端横向图标导航。侧栏四周用 CSS 像素边框补强；底部独立装饰层暂不接入运行时，避免与空壳底部残留装饰重复。已验证 `cd frontend; npm.cmd run build`、桌面 `overview -> server -> diagnostics` 点击跳转、390×760 移动视口无水平溢出，console 无 error/warn。

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
| M11 Console | completed | allowlist 命令、Control Mod 文件通道喊话 |
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
| MODSYNC-AUTO-1 | completed | Mod 同步分类自动识别默认值，并允许任意登录用户手动修改服务器专用/玩家需同步/待确认标签 |
| PLAYERSYNC-PACK-2 | completed | 玩家同步包升级为 Windows 安装包结构，内置安装/卸载脚本、payload checksum、pack-manifest、Mod 备份恢复和 Steam 启动项尽力配置 |
| PLAYERSYNC-PACK-3 | completed | 玩家同步安装脚本改用 `-LiteralPath` 处理 Mod 路径，修复 `[CP]` 方括号目录导致 checksum 误报缺文件 |
| PLAYERSYNC-PACK-4 | completed | 玩家同步安装脚本新增 PowerShell 原生进度条和文本进度行，覆盖环境检查、checksum、SMAPI、Mod、Steam 和完成阶段 |
| NEXUS-2 | completed | Mod 管理第二阶段：Nexus Mods 只读搜索（`GET mods/nexus/search`，ID 精确查询走官方 v1 REST、关键词走 GraphQL v2）、`UpdateKeys`/`NexusModID` manifest 解析、已安装匹配、ModsPage 在线搜索区域；不做下载/安装 |
| FE-MODS-WORKBENCH-1 | completed | ModsPage 参考 EMP Mod 管理台改为“下载模组 / 添加模组 / 配置模组”三段式工作台，Nexus 搜索卡片化，已安装与玩家同步归入添加页，配置页预留 SMAPI 配置入口 |
| NEXUS-SETTINGS-1 | completed | Nexus API Key 改为管理员在前端配置并持久化到 SQLite `panel_settings`，后端搜索请求即时读取，不再使用环境变量 |
| MODRESTART-1 | completed | 停服状态下 Mod 写操作不再提示需要重启，旧重启标记会在停止/停服写入时清理 |
| MODZIP-1 | completed | Mod ZIP 上传识别 XNB 替换包并返回明确错误提示，不再误导为 ZIP 损坏 |
| MODZIP-2 | completed | Mod ZIP 上传支持 Nexus 单层外壳目录包，自动剥离外壳并导入内部真实 SMAPI Mod 子目录 |
| MODZIP-3 | completed | Mod manifest 解析兼容 UTF-8 BOM，避免 Nexus 原包因 BOM manifest 被误判为无效 Mod ZIP |
| MODZIP-4 | completed | Mod manifest 解析兼容 JSONC 注释和尾随逗号，避免 SpaceCore 等原包远程安装失败 |
| MODDEPS-1 | completed | Mod 列表解析并展示 SMAPI 前置依赖声明，普通用户可在已安装 Mod 卡片看到需要的前置依赖 |
| MODORIGIN-1 | completed | Nexus 多 Mod ZIP 的内容包记录来源包字段，已安装卡片区分主 N站 Mod 与随包内容包，并支持同包相邻展示与捆绑删除 |
| NEXUS-PAGED-1 / NEXUS-PAGER-2 | completed | 下载页回到 Nexus-only 搜索，支持默认热门、下载量排序、分页和 Nexus 一键安装；旧 `/mods/search` 统一搜索骨架已撤回 |

## 近期优先级

0. 玩家缓存按 `saveId` 隔离已修复；真实新建/切换存档后确认上一存档玩家不再出现在当前玩家列表。
1. 真实运行环境验证邀请码重启刷新、SMAPI DLL 加载，以及玩家页 `farmIncome`/`personalIncome` 显示。
2. 验证玩家页在真实多人场景下的位置、在线状态、中文地图名和最近事件。
3. 继续排查联机角色槽异常，保持只诊断不破坏存档。
4. 做一次完整 release checklist 冒烟测试。
5. 持续清理 UI 中已无 JSX 引用的旧 CSS 规则和旧组件残留；本轮已删除无引用的旧 Stardew Section 组件与前端死 API 封装。
6. ~~用真实 Nexus API Key 验证 Nexus 关键词搜索的 GraphQL v2 返回结构~~ 已完成：通过对 `https://api.nexusmods.com/v2/graphql` 做 schema introspection 和真实搜索请求，确认并修复了 `nexus.go` 里 `mods` 查询的参数结构（游戏域名和关键词都要放进 `filter: ModsFilter` 而不是顶层 `gameDomain` 参数），关键词搜索本身不需要个人 API Key。
7. 为 ModsPage 的依赖缺失检查、更新检查和 SMAPI 配置编辑补齐后端能力；Nexus 安装与按存档启用/禁用已接入。

## 中期路线

- 玩家事件驱动 SSE。
- 完整服务器日志 tail。
- 更完善备份策略。
- 计划重启：管理员配置每日维护窗口（几点关闭、几点开启）、提前广播、关闭前备份，并复用现有停止/启动生命周期 job。
- Mod 依赖缺失/版本检查和更新提示。
- Nexus 多文件选择、权限差异提示和 OAuth/非 Premium 下载体验优化。
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
# SMAPI-RUNTIME-1 状态
- `SMAPI-RUNTIME-1` completed：Mods 列表现在会在面板控制 Mod 已安装时置顶显示虚拟内置组件 `SMAPI`，提醒玩家客户端需要先安装 SMAPI；该条目带 `builtIn=true`，前端禁用删除和同步分类编辑，玩家同步统计/导出排除该虚拟运行组件。
# NEXUS-PAGED-1 状态

- `NEXUS-PAGED-1` completed：ModsPage 在线搜索回到 Nexus-only 路径，GraphQL 关键词搜索按下载量降序下推排序，并支持 `page/pageSize/total/hasMore` 分页。旧统一搜索前端入口已移除。
- `MODSEARCH-1` 统一搜索骨架已撤回：后端 `/mods/search` 与 `/mods/search/install` 路由、`mod_search.go` 和对应测试已移除；当前只保留 Nexus 搜索/安装源。
# NEXUS-PAGER-2 状态

- `NEXUS-PAGER-2` completed：Nexus 搜索结果顶部和底部都有完整分页控件，支持首页/末页/上一页/下一页/指定页跳转。

# SMAPI-SYNC-2 状态

- `SMAPI-SYNC-2` completed：SMAPI 现在作为内置但玩家必需的同步项，进入玩家同步统计与同步包 `pack-manifest.json`；`StardewAnxiPanel.Control` 标为内置服务端控制组件，前端不显示删除按钮，后端拒绝删除，且永远不打包进玩家同步 ZIP。

# PLAYERSYNC-PACK-11 状态
- `PLAYERSYNC-PACK-11` completed：玩家同步安装脚本恢复 ASCII-only 动态进度条，动态行只显示英文阶段和百分比，中文详细状态仍写日志并通过独立任务行输出；继续禁用 `Write-Progress`，当前测试解压包已热修并真实验证中文任务行不再重叠。
# PLAYERSYNC-PACK-12 状态
- `PLAYERSYNC-PACK-12` completed：玩家同步安装脚本将日志写入改为 `Write-LogLine` 短重试且非致命，修复 `install-*.log` 被短暂占用时中断 SMAPI 安装的问题。当前测试解压包已热修并真实安装验证通过。
# PLAYERSYNC-PACK-13 状态
- `PLAYERSYNC-PACK-13` completed：玩家同步安装包终端输出将启动项标题高亮为 Yellow、可复制启动项文本高亮为 Cyan，并保持启动项独立一行。当前测试解压包已热修并真实安装验证正常。
# PLAYERSYNC-PACK-14 状态
- `PLAYERSYNC-PACK-14` completed：玩家同步安装包在启动项标题后新增“请复制到 Steam 的游戏启动项中。”提示，可复制启动项文本仍独立一行。当前测试解压包已热修并真实安装验证正常。
# PLAYERSYNC-PACK-15 状态
- `PLAYERSYNC-PACK-15` completed：新增不带 SMAPI 的 `stardew-player-mods-update-pack.zip` 导出模式。完整版同步包继续用于首次玩家；模组更新包用于已运行过同步包的玩家，客户端检查已有 `StardewModdingAPI.exe` 后只安装/更新真实玩家 Mod，并沿用指纹跳过相同 Mod、不同内容备份覆盖的策略。

# PLAYERSYNC-PACK-16 状态
- `PLAYERSYNC-PACK-16` completed：模组更新包安装脚本不再尝试读取或写入 Steam 启动项，沿用完整版同步包已经设置好的 SMAPI 启动项；更新包摘要只显示已跳过 Steam 配置，不再输出复制启动项文本。完整同步包的 Steam 配置行为保持不变。

# MODPROFILE-1 状态

- `MODPROFILE-1` completed：完成按存档启用/禁用 Mod 第一阶段。新增 `mods-disabled` 目录、`mod-profiles.json`、`PUT /mods/:id/enabled`，配置页可在停服时按当前存档切换 Mod。新建/新导入存档默认禁用所有非内置 Mod。
# MODPROFILE-2 状态

- `MODPROFILE-2` completed：修复切换存档后前端仍显示旧存档 Mod 启用状态的问题；公共数据层会在 active save 变化时刷新 mods，并补充后端 profile 物理目录切换测试。

# NEXUS-DEFAULT-1 状态

- `NEXUS-DEFAULT-1` completed：下载模组页默认展示 Nexus Stardew Valley 热门列表前 20 条；空 `q` 搜索现在作为默认热门列表处理，仍支持分页和已安装匹配。
# NEWGAME-CABINS-1 状态
- `NEWGAME-CABINS-1` completed：自定义新存档的初始联机小屋数现在按真实小屋数显示和提交；后端 `startingCabins` 契约对齐 0-7，并同时写入 Junimo settings、SMAPI control init 与 `new-game-pending` 一次性标记；控制模组只在标记存在时于 Junimo 新建存档前同步 Stardew 原生小屋数/布局参数。后续如仍出现“存档里有 Cabin 但地图不可见”，需要针对 Junimo/存档 XML 的建筑坐标做专项验证。
- `CONTROL-NATIVE-CREATE-REMOVE-1` completed：Anxi Control 模组的历史原生创建存档路径已移除；自定义新存档只通过 Junimo `POST /newgame` 创建，Control 保留新建前参数同步、角色定制和运行期控制。

# FE-BACKUP-COPY-1 状态
- `FE-BACKUP-COPY-1` completed：备份设置区文案从 `latest`/`scheduled` 等内部术语改为用户语义说明；每个自动备份选项解释覆盖行为和保留规则，备份列表类型标签改为“手动备份 / 最新备份 / 每日快照 / 定时备份”。


# SAVE-BACKUP-POLICY-1 ??
- `SAVE-BACKUP-POLICY-1` completed?????????????????????????????? latest????????????????????? 3 ???? 14 ??? daily ???????SMAPI Control ????????????????????????????????????????????? scheduler ?????/?????

# SAVE-BACKUP-SCHEDULE-HOUR-1 状态
- `SAVE-BACKUP-SCHEDULE-HOUR-1` completed：定时备份已从“每隔 N 小时”改为“每天 HH:00 执行一次”，前端使用 00:00-23:00 下拉框，后端以 `scheduledHour` 存储和判断，并兼容读取旧 `scheduledIntervalHours`。
# MODDEPS-2 状态
- `MODDEPS-2` completed：Mod 依赖检测已从“只展示 manifest 声明”升级为后端状态判断。`GET /mods` 会标记依赖是否安装、当前存档是否启用、最低版本是否满足；Nexus 搜索会把当前存档禁用的已安装 Mod 标为 `installed=true, installedEnabled=false`，前端显示“已安装但未启用”。配置页依赖诊断已放在 Mod 名称区域下方，避免长英文名和状态列互相挤压。后续仍可优化：依赖自动安装入口、Nexus/SMAPI 更新提示、多来源依赖索引。

# MODREL-1 状态
- `MODREL-1` completed：Mod 同步分类与启用状态已按关系联动。同步分类按必需依赖连通组一起变，避免“待确认”后切回其它标签时后置 Mod 停留在旧状态；启用会补同包和前置，禁用会关同包和下游但保留 Content Patcher 等共享前置。两个 PUT 接口都会返回本次受影响的 `mods[]`，前端按返回结果批量更新。
# NEXUS-EXT-2 状态
- `NEXUS-EXT-2` completed：修复 Nexus/远程安装任务日志中的核心乱码文案；安装/上传成功后后端会把本次导入的 Mod 标记为当前激活存档启用，ModsPage 也会自动切到“添加模组”页并刷新已安装列表，避免扩展提交成功但用户看不到或用不上新 Mod。

# NEXUS-EXT-3 状态
- `NEXUS-EXT-3` completed：Nexus 搜索结果“一键安装”改为同页跳转到 Nexus 文件页并带 `anxi_auto=1`，由浏览器扩展自动获取临时 ZIP；扩展右下角只保留提交按钮，提交后创建 `mod_remote_install` 任务并跳回 `/instances/:id/jobs?jobId=...`，任务页会直接选中新任务。
# NEXUS-REQ-1 状态
- `NEXUS-REQ-1` completed：Nexus 搜索结果现在返回 `requiredMods[]`，前端搜索卡片会提示缺失/未启用的 Nexus 前置，并可对缺失前置走同一套扩展一键安装。浏览器扩展已支持 Nexus “Additional files required” 弹窗自动点击 `Download` 继续。
# NEXUS-PREMIUM-2 状态
- `NEXUS-PREMIUM-2` completed：Mods 下载页已移除“粘贴链接安装”人工入口和对应前端冗余 API/type。未配置 Nexus Key 时仅在配置按钮左侧提示 Premium 用户填写 NexusKey；配置后提示消失，并在每个 Nexus 搜索结果卡片底部显示 `N站会员专属安装`，走现有 Nexus API Key/Premium 直连安装。普通 `一键安装` 继续服务非 Premium 用户的浏览器扩展流程。
# NEXUS-CARD-UI-1 状态
- `NEXUS-CARD-UI-1` completed：Nexus 搜索结果卡片完成布局整理，主操作按钮固定在统一操作行，会员安装和前置依赖状态进入底部次操作区。前置依赖只显示 `缺少前置mod` / `前置已满足`，点击或悬停后展开具体 Mod 名、NexusId 和状态。

# NEXUS-EXT-BATCH-1 状态

- `NEXUS-EXT-BATCH-1` completed：普通 Nexus 一键安装已改为后台批量扩展流程。面板页保持不跳转，扩展后台同时打开当前 Mod 与未安装前置 Mod 的 Nexus 下载页，自动捕获并提交 ZIP 链接；搜索卡片主按钮显示扩展提交流程百分比，失败时显示 `失败请手动安装`。
- 补充：ModsPage 会把 Nexus 搜索条件、结果、分页和扩展批量安装状态保存到 `sessionStorage`。用户切到任务日志等页面再返回时，不会重新加载默认热门或清空搜索结果；若扩展批量安装仍在进行，会继续通过 `GET_BATCH_STATUS` 轮询并恢复按钮进度。
- 补充：扩展在后台标签页处理 `Manual download` / 前置确认 `Download` 时优先读取 `href` 直跳并保留批量参数；Manual 为 JS 按钮时改用页面主世界 `button.click()`，不再把 debugger 坐标点击作为唯一入口，修复后台页卡在“正在进入下载页”的问题。
- 补充：批量自动提交按 ZIP 来源分流。直接生成链接走同一 message 生命周期继续推进；下载事件捕获链接则回到 content 再发 `SUBMIT_CAPTURED_URL`，避免 MV3 service worker 在 `downloads.onCreated` 长 fetch 时卡在 `posting`。Nexus content script 会用 `sessionStorage` 记住批量安装上下文，跳转丢参后仍会自动触发提交，不再停在人工“提交到面板”按钮。批量任务面板提交优先经已登录面板页 `panel-bridge.js` 同源转发，复用 Cookie/Vite proxy；面板提交请求增加 30 秒超时和失败回写。
- 补充：远程 ZIP 下载改用独立 15 分钟 archive HTTP client，修复 Ridgeside Village 等大包在 10 秒 Nexus API timeout 下读 body 失败。扩展安装按钮进度也改为继续跟踪面板 job 最终状态：job 创建只到 90%，全部 succeeded 才 100%，任一 failed/canceled 则显示失败。content 直接生成 ZIP 和 downloads 捕获 ZIP 都会统一触发原提交按钮逻辑，background 只做消息丢失兜底。
- 补充：无 `jobId` 但本地 Mod 列表已按 `nexusModId/originNexusModId` 命中的扩展 item 会被视为完成，修复实际安装成功但前端进度卡住的问题。
- 补充：扩展提交消息现在显式携带并恢复 `batchId/itemId/autoSubmit`，background 可在捕获/提交阶段把丢失的 batch 上下文补回 capture，确保新任务 `jobId` 写回 batch item；本地已安装匹配只是兜底。
- 补充：ModsPage 新增扩展安装 `重置状态` 入口，通过 `CLEAR_STATE` 清前端 session 和扩展存储，解决前后端重启后旧 batch 仍卡在浏览器里的问题。
# NEXUS-EXT-BATCH-2 状态

- `NEXUS-EXT-BATCH-2` completed：扩展批量安装终态收敛已修复，`done/failed` 不再被旧 batch 轮询覆盖；完成后会用最新本地 Mod 列表同步搜索结果缓存，避免返回下载页后已安装项又显示“一键安装”。
- 补充：Nexus 多 Mod ZIP 来源纠偏已接入。显式 Nexus/远程安装不再先写推断来源；如果批量上下文传错 `result.modId`，后端会优先使用 ZIP 内唯一正数 `UpdateKeys: ["Nexus:<id>"]` 写 sidecar。当前测试实例的 Ridgeside Village 组件来源已从 SpaceCore 1348 修回 Ridgeside 7286。
# NEXUS-EXT-BATCH-3 状态
- `NEXUS-EXT-BATCH-3` completed：浏览器扩展批量安装入口已增加目标去重和 batch 幂等保护。相同 Nexus `modId` 只打开一个后台页，同一 `batchId` 重复发送不会重复开页，修复 Ridgeside Village 批量安装时本体页面被打开两次、其中一页成功关闭另一页遗留后台的问题。
# NEXUS-EXT-CONNECT-1 状态
- `NEXUS-EXT-CONNECT-1` completed：Mods 下载页在“配置 Nexus Key”旁新增“检测扩展”按钮；扩展会自动识别当前面板 `origin` 并写入 `panelBaseUrl`，连通后普通“一键安装”才开放，未连通时按钮灰色禁用。该握手通过 `GET /api/auth/me` 确认当前页是已登录面板，Premium Key 直连安装不受影响。
# NEXUS-EXT-PACK-1 状态
- `NEXUS-EXT-PACK-1` completed：面板已提供浏览器扩展下载引导。Mods 下载页在 `配置 Nexus Key` 右侧提示 Nexus 普通用户安装扩展，并提供 `下载浏览器扩展` 按钮；Docker 构建期会生成 `/app/browser-extensions/anxi-nexus-installer.zip`，后端优先复用实例目录或镜像中的合法预打包文件，缺失或损坏时才从 `browser-extensions/nexus-slow-installer` 兜底生成。
# NEWGAME-PLAYERLIMIT-1 状态
- `NEWGAME-PLAYERLIMIT-1` completed：自定义新建存档新增 `maxPlayers` 联机人数上限，前端可在“新建存档”界面把总在线人数调到 7 个初始小屋之外；后端写入 Junimo `Server.MaxPlayers`，并显式使用 `CabinStack` 自动小屋管理。`startingCabins` 保持 0-7，继续只表达初始小屋数量。
# PERF-REVIEW-1 状态
- `PERF-REVIEW-1` completed：完成一轮低风险性能/冗余/内存优化。后端存档主 XML 的 farm type 兜底改为流式扫描，备份 ZIP 元数据不再无条件读入完整主存档 entry；Nexus sidecar 展示元数据判断改为按 modId 预建 map。前端 `ModsPage` 合并已安装 Mod 派生数据统计并用 `useMemo` 缓存，减少频繁局部 state 更新下的重复排序/过滤和临时数组分配。
# VNC-CONTROL-1 状态
- `VNC-CONTROL-1` completed：服务器控制页新增管理员 VNC 操作入口。页面刷新后会先通过面板后端代理 Junimo `GET /rendering` 恢复真实渲染状态；`打开VNC显示` 通过 `POST /rendering?fps=15` 打开服务端画面渲染，成功后切换为 `关闭VNC显示` 并可通过 `fps=0` 关闭；`跳转VNC控制` 默认隐藏，仅在显示渲染打开后出现，按当前面板 hostname + 自定义 `vncPort` 打开 noVNC 页面。前端不接触 Junimo API key，VNC 密码不回显。
# FE-PROTOTYPE-LAYOUT-1 状态
- `FE-PROTOTYPE-LAYOUT-1` completed：前端主要 Stardew 页面已按 `external artifact stardew-page-prototypes-image2-2026-06-30` 的信息架构重新排布。总览页对齐农场横幅、生命周期控制、邀请码、摘要指标和三列摘要；存档页新增当前激活存档重点卡；服务器、任务、玩家、模组、诊断、设置页通过页面级布局 class 调整为原型式分区。现有 API 和功能不变，`ModsPage` 保留三段式工作台。
# FE-SHELL-IMAGE2-1 状态
- `FE-SHELL-IMAGE2-1` completed：Stardew Shell 顶栏已替换为 image2 `Top bar.png`，左侧导航迁移到 `Left panel.png`，右侧任务栏迁移到 `01-overview-right-sidebar-empty-image2.png`。顶栏继续显示运行/停止状态、当前农场名、面板版本、管理员/普通用户和登出入口；左侧栏用透明热区承接原九路由点击逻辑，移动端保留横向图标导航；右侧 OpsRail 保留健康和任务状态逻辑。

# FE-TOPBAR-SPLIT-ASSETS-1 状态
- `FE-TOPBAR-SPLIT-ASSETS-1` completed：Stardew Shell 顶栏已从整张 `panel_top_bar_image2.png` 可见背景迁移为 `frontend/public/assets/stardew/ui/topbar/` 下的拆分素材组合。横栏空壳使用三段式 shell，品牌、状态框、农场框、版本框、用户框、头像、下拉箭头和登出按钮均分层渲染；状态红绿点继续复用现有 `.sd-dot` / `.sd-dot-pulse` 动态逻辑，没有改成图片。现有状态/存档/版本/用户/登出点击行为和数据来源保持不变，移动端无横向溢出。

# FE-ASSET-TOP-BAR-SHELL-1 状态
- `FE-ASSET-TOP-BAR-SHELL-1` completed：已从 image2 `Top bar.png` 生成可复用顶栏木质背景空壳素材 `panel_top_bar_shell_empty_image2.png`。素材为透明 PNG，移除鸡图标、品牌字、状态徽章、农场/版本/角色槽、登出按钮和所有图标文字，只保留木纹横栏、金棕边框、四角装饰和像素阴影；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-TOP-BAR-CORNERS-1 状态
- `FE-ASSET-TOP-BAR-CORNERS-1` completed：已生成并入库顶栏四角像素装饰素材。新增左上、右上、左下、右下 4 个透明 PNG 和 `topbar_corner_ornaments_sprite_sheet_2x2_image2.png`；素材只保留金棕木质/金属角标、像素阴影和高光，不含顶栏背景、文字、按钮、图标或状态徽章，当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-TOP-BAR-CHICKEN-1 状态
- `FE-ASSET-TOP-BAR-CHICKEN-1` completed：已生成并入库顶栏左侧品牌鸡图标素材 `icon_topbar_chicken_image2.png`。素材为透明 PNG，只保留白色鸡本体、红色鸡冠、黄色喙、橙色脚、像素描边、阴影和高光，不含品牌文字、顶栏木质背景或其它 UI 元素；当前仅入库生产素材，尚未接入 Shell。

# FE-SHELL-SCALE-1 状态
- `FE-SHELL-SCALE-1` completed：Stardew Shell 已接入全局等比缩放。以 `1536x1024` 为设计基准，`.sd-shell` 计算 `--sd-ui-scale = max(0.72, min(100vw/1536, 100dvh/1024))`，使用反向宽高 + `transform: scale(...)` 让整套 UI 填满当前浏览器可视区；右 OpsRail 隐藏阈值下放到最小全布局之后，`StardewPanel.tsx` 自动折叠估算同步按新 scale/栏宽计算。已验证前端构建通过，并用临时 HTTP QA 页确认 760x504 为最小三栏基准、1920x1080 可继续等比放大且无页面溢出。

# FE-ASSET-TOP-BAR-BRAND-GLOW-1 状态
- `FE-ASSET-TOP-BAR-BRAND-GLOW-1` completed：已生成并入库顶栏品牌文字暖黄色发光/阴影占位素材 `topbar_brand_text_glow_placeholder_image2.png`。素材为透明 PNG，不含实际文字、鸡图标或木质顶栏背景，仅供前端动态渲染品牌文字时作为底层光效；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-FARM-SELECT-FRAME-1 状态
- `FE-ASSET-FARM-SELECT-FRAME-1` completed：已生成并入库顶栏农场选择框空底图 `field_topbar_farm_select_empty_image2.png`。素材为透明 PNG，只保留金棕像素边框、暗棕木纹内容底、像素阴影和下拉框外形，已移除农场图标、农场名、下拉箭头和顶栏背景；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-FARM-SELECT-3PIECE-1 状态
- `FE-ASSET-FARM-SELECT-3PIECE-1` completed：已生成并入库农场选择框三段式透明素材和 `field_topbar_farm_select_3piece_sheet_image2.png`。左端/中段/右端均不含农场图标、农场名或下拉箭头，中段可横向平铺；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-DROPDOWN-ARROW-1 状态
- `FE-ASSET-DROPDOWN-ARROW-1` completed：已生成并入库顶栏浅金色下拉箭头图标 `icon_dropdown_arrow_gold_image2.png`。素材为透明 PNG，只保留像素箭头、暗色描边和阴影，不含农场框、用户框或文字；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-VERSION-BADGE-FRAME-1 状态
- `FE-ASSET-VERSION-BADGE-FRAME-1` completed：已生成并入库版本号小框空底图 `field_topbar_version_badge_empty_image2.png`。素材为透明 PNG，只保留棕色/金色像素边框、暗木纹内部、阴影和高光，不含版本号文字或顶栏背景；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-USER-ROLE-FRAME-1 状态
- `FE-ASSET-USER-ROLE-FRAME-1` completed：已生成并入库用户角色框空底图 `field_topbar_user_role_empty_image2.png`。素材为透明 PNG，只保留木质/金色边框、暗棕内容底、像素阴影和高光，已移除头像、角色文字、下拉箭头和顶栏背景；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-USER-ROLE-3PIECE-1 状态
- `FE-ASSET-USER-ROLE-3PIECE-1` completed：已生成并入库用户角色框三段式透明素材和 `field_topbar_user_role_3piece_sheet_image2.png`。左端/中段/右端均不含头像、角色文字或下拉箭头，中段可横向平铺；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-TOP-BAR-USER-AVATAR-1 状态
- `FE-ASSET-TOP-BAR-USER-AVATAR-1` completed：已生成并入库顶栏用户头像图标 `icon_topbar_user_avatar_image2.png`。素材为透明 PNG，只保留橙色头发、蓝色衣服、脸部细节、像素描边和高光，不含用户框背景、文字或箭头；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-LOGOUT-BUTTON-FRAME-1 状态
- `FE-ASSET-LOGOUT-BUTTON-FRAME-1` completed：已生成并入库红色登出按钮空底图 `button_topbar_logout_empty_image2.png`。素材为透明 PNG，只保留红色按钮底、暗红/金棕边框、像素阴影、高光和按键质感，已移除登出图标、文字和顶栏背景；当前仅入库生产素材，尚未接入 Shell。

# FE-ASSET-LEFT-RAIL-SHELL-1 状态
- `FE-ASSET-LEFT-RAIL-SHELL-1` completed：已从 image2 `Left panel.png` 生成可复用左侧栏木质背景空壳素材 `panel_side_rail_shell_empty_image2.png`。素材为透明 PNG，移除导航按钮、菜单文字、菜单图标和按钮状态残影，保留木框、深色木纹、横向分隔、底部置物架与装饰区；当前仅入库生产素材，尚未切换 Shell 引用。
# FE-ASSET-NAV-BUTTON-DEFAULT-1 状态
- `FE-ASSET-NAV-BUTTON-DEFAULT-1` completed：已从 image2 `Left panel.png` 提取并重绘默认态左侧导航按钮空底图 `nav_item_default_wood_blank_image2.png`。素材为透明 PNG，移除中文文字、菜单图标和侧栏背景，只保留木质按钮、金棕边框、像素角饰、内侧阴影和高光；当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-NAV-BUTTON-ACTIVE-1 状态
- `FE-ASSET-NAV-BUTTON-ACTIVE-1` completed：已生成并抠图入库激活态左侧导航按钮空底图 `nav_item_active_wood_blank_image2.png`。素材为透明 PNG，宽度对齐默认态按钮，保留木质中心、亮金色双边框、像素角饰和轻微暖色发光；不含文字、图标和侧栏背景，当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-NAV-BUTTON-HOVER-1 状态
- `FE-ASSET-NAV-BUTTON-HOVER-1` completed：已生成并入库悬停态左侧导航按钮空底图 `nav_item_hover_wood_blank_image2.png`。素材为透明 PNG，尺寸对齐默认态按钮 `442x138`，在木质主体上加入轻微金色高光，状态介于默认态和激活态之间；不含文字、图标和侧栏背景，当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-SIDEBAR-DECOR-PROPS-1 状态
- `FE-ASSET-SIDEBAR-DECOR-PROPS-1` completed：已从 image2 `Left panel.png` 重生成左侧栏底部装饰整组与单件素材。覆盖整组 `sidebar_bottom_decor_props_group_image2.png`，以及单独灯笼、盆栽、紫水晶三个透明 PNG；整组可带木架，单件均不带菜单文字、导航按钮或侧栏背景，当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-NAV-ICONS-IMAGE2-1 状态
- `FE-ASSET-NAV-ICONS-IMAGE2-1` completed：已生成并入库 image2 左侧导航 9 枚透明图标和 3x3 sprite sheet。图标包括总览地图、服务器机柜、存档宝箱、任务日志卷轴、玩家头像、模组绿色晶体、诊断监视器、安装纸箱和设置齿轮；均不含按钮底图、中文文字或侧栏背景，当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-RIGHT-RAIL-SHELL-1 状态
- `FE-ASSET-RIGHT-RAIL-SHELL-1` completed：已生成并入库右侧栏木质背景空壳素材 `panel_right_rail_shell_empty_image2.png`。素材为透明 PNG，移除原右侧栏内部三块卡片、标题文字、图标、状态点、进度条和任务内容，只保留外层立柱、完整顶部横梁、深棕内底、藤蔓、底部基座和南瓜/向日葵装饰；当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-RIGHT-RAIL-BORDER-1 状态
- `FE-ASSET-RIGHT-RAIL-BORDER-1` completed：已生成并入库右侧栏外层木质边框素材 `panel_right_rail_outer_border_image2.png`。素材为透明 PNG，中间区域完全透明，只保留最外侧左右竖梁、顶部/底部边缘、像素阴影、金棕木质雕刻和外框藤蔓点缀；不含内部卡片、文字、图标、进度条、任务内容和底部南瓜/向日葵装饰，当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-RIGHT-RAIL-CARDS-1 状态
- `FE-ASSET-RIGHT-RAIL-CARDS-1` completed：已生成并入库右侧栏三张卡片空框素材 `panel_right_rail_card_health_empty_image2.png`、`panel_right_rail_card_in_progress_empty_image2.png`、`panel_right_rail_card_recent_tasks_empty_image2.png`。三张素材为透明 PNG，分别对应系统健康、进行中和近期任务卡片，只保留木质边框、深棕内容底、金棕角饰、藤蔓和像素阴影；不含标题文字、图标、状态点、进度条、内部横线、任务列表或其它动态内容，当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-RIGHT-RAIL-CARDS-NINESLICE-1 状态
- `FE-ASSET-RIGHT-RAIL-CARDS-NINESLICE-1` completed：已生成并入库右侧栏三张九宫格卡片框素材 `panel_right_rail_card_health_nineslice_image2.png`、`panel_right_rail_card_in_progress_nineslice_image2.png`、`panel_right_rail_card_recent_tasks_nineslice_image2.png`。三张素材为透明 PNG，四角装饰完整，边框中段更规则，中心保留深棕木纹/皮革纹理；不含文字、图标、状态点、进度条、内部横线、任务列表或参考线，当前仅入库生产素材，尚未接入 Shell。
# FE-ASSET-RIGHT-RAIL-TITLE-ICONS-1 状态
- `FE-ASSET-RIGHT-RAIL-TITLE-ICONS-1` completed：已生成并入库右侧栏三枚标题图标 `icon_right_rail_health_heart_image2.png`、`icon_right_rail_in_progress_clock_image2.png`、`icon_right_rail_recent_tasks_clipboard_image2.png`。三枚素材为透明 PNG，分别对应系统健康红心、进行中蓝色时钟和近期任务剪贴板，只保留图标本体、像素描边、阴影和高光；不含文字、卡片框、右侧栏背景、进度条、状态点或列表内容，当前仅入库生产素材，尚未接入 Shell。
# FE-RIGHT-RAIL-3PIECE-RUNTIME-1 状态

- `FE-RIGHT-RAIL-3PIECE-RUNTIME-1` completed：右侧 OpsRail 运行时已接入新三段外壳，顶部固定段使用 `right_rail_shell_top.png`，中段使用 `right_rail_shell_middle_tile.png` 纵向 `repeat-y`，底部固定段使用 `right_rail_shell_bottom.png`。不再在生效规则里用整张右栏 shell 或整张截图做 `100% 100%` 拉伸。
- 三张信息卡片已改用 `right_card_health_9slice.png`、`right_card_progress_9slice.png`、`right_card_recent_9slice.png` 作为 `border-image` 九宫格卡片框；标题、图标、健康状态、任务列表、按钮文字和状态点仍由 React/CSS 动态渲染。
- 已验证 `cd frontend; npm.cmd run build` 通过；本地浏览器 QA 覆盖 1280x720、1280x900、1280x560、390x760，确认中段不纵向拉伸、矮窗口 stack 内部滚动、移动端隐藏右栏、console error/warn 为空。
# FE-RIGHT-RAIL-PROTOTYPE-ALIGN-1 状态

- `FE-RIGHT-RAIL-PROTOTYPE-ALIGN-1` completed：右侧 OpsRail 已按 `01-overview-right-sidebar-empty-image2.png` 原型继续调整运行时比例。右栏列宽改为 `clamp(340px, 27vw, 430px)`；顶部 shell 裁掉上方透明安全边后贴顶；底部 shell 按可见装饰区贴底；卡片收回左右木柱内侧并移除外投影，修复上下边框不贴边和左右边框被横向阴影切断的问题。已验证 `cd frontend; npm.cmd run build` 通过，本地 QA 页面 1280x900 截图确认顶部贴边、侧柱连续、卡片位于内框范围且 console error/warn 为空。

# FE-RIGHT-RAIL-BLACK-EDGE-FIX-1 状态

- `FE-RIGHT-RAIL-BLACK-EDGE-FIX-1` completed：修复右侧 OpsRail 三段 shell 接入后左右两侧露黑的问题。中段 `right_rail_shell_middle_tile.png` 改为 121% 横向 overscan 后居中 `repeat-y`，top/bottom 固定段按 108% 横向 overscan 并同步调整高度和 stack 扣底部装饰高度；兜底色改为木板棕。已验证 `cd frontend; npm.cmd run build` 通过，本地 QA 页面 1280x720 / 1280x560 截图确认黑边消失且矮窗口仍内部滚动。

# FE-MAIN-PAGE-FRAME-1 状态
- `FE-MAIN-PAGE-FRAME-1` completed：所有 Stardew 路由的中间主内容区 `.sd-main` 已统一替换为 image2 存档页空框背景 `main_page_frame_empty_image2.png`。资源从 `external artifact stardew-page-prototypes-image2-2026-06-30 (03-saves-page-frame-empty-image2.png)` 复制到 `frontend/public/assets/stardew/ui/panels/` 供运行时和 Docker 静态发布使用；主内容背景改为居中、不重复、`100% 100%` 铺满，并把页面整体 padding 调整为 `clamp(28px, 2.4vw, 42px)` 避免压到木框角饰。主内容区仍保留 `overflow-y: auto`，但已隐藏 Firefox/Chromium/WebKit 原生滚动条，避免白色竖条压住右侧 frame 边框。已验证前端构建通过，生产 CSS 临时 Shell QA 页在 1280x720 和 390x760 下背景加载、滚动条隐藏、滚动能力保留、无横向溢出、console error/warn 为空。
# FE-MODS-DYNAMIC-PAGESIZE-1 状态
- `FE-MODS-DYNAMIC-PAGESIZE-1` completed：模组下载页 Nexus 搜索结果已改为固定搜索卡片高度 + 动态 pageSize。`.sd-mods-nexus-search-list` 卡片高度固定 `246px`，前端按真实结果网格在 `.sd-main-scroll` 内的可见高度和实际列数计算每页数量，并传给既有 Nexus 搜索 API 的 `pageSize`；加载骨架不参与测量，避免 loading/结果态来回触发刷新；顶部翻页器同步显示“每页 N 个”，底部重复翻页器移除。已验证 `cd frontend; npm.cmd run build` 通过，并用临时本地 QA 页面确认 1040x1120 为 pageSize=4、1040x720 为 pageSize=2、520x720 为 pageSize=1，卡片高度均为 `246px`。
# FE-JOBS-PROTOTYPE-IMAGE2-1 状态
- `FE-JOBS-PROTOTYPE-IMAGE2-1` completed：任务与日志页已按 image2 原型重皮肤为羊皮纸双栏任务台，包含顶部标题虚线、按钮工具条、左侧任务列表、右侧任务详情/进度/SSE 状态/深色日志终端和 VNC 修复提示。原型图未作为运行时资源；纸纹、铜边、选中态、状态徽章、进度条、终端扫描线和警告纸条均由 CSS 实现，按钮/图标复用既有 Stardew PNG/图标素材。业务逻辑、API、权限、loading/error/empty/disabled 状态保持不变。已验证 `cd frontend; npm.cmd run build` 通过，并用临时 QA 页检查 1280x900 与 390x760 无横向溢出、按钮文字不溢出、移动端底部 VNC 提示可滚动完整查看、console error/warn 为空。
# FE-PLAYERS-PROTOTYPE-IMAGE2-1 状态
- `FE-PLAYERS-PROTOTYPE-IMAGE2-1` completed：玩家管理页已按 image2 `05-players - 副本.png` 原型重皮肤为六摘要卡、邀请加入码横条、Junimo 终端、在线玩家表、活动历史和管理员操作区结构。原型图未作为运行时背景或整块素材引用；纸纹、铜边、角钉、分隔线、绿字终端、表格和禁用操作按钮均为 CSS 实现。功能逻辑、API 调用、权限判断、loading/error/empty/disabled 状态保留；按钮/图标复用现有 Stardew PNG/CSS 按钮体系。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器临时 QA 覆盖 1280x900 与 390x760 无页面级横向溢出，桌面表格操作列首屏可见，窄屏表格仅自身横向滚动，console error/warn 为空。
# FE-DIAGNOSTICS-GAUGE-CODE-1 状态
- `FE-DIAGNOSTICS-GAUGE-CODE-1` completed：诊断与健康页 CPU / 内存 / 磁盘三枚资源仪表已完成代码级视觉优化。数值与 `%` 拆分排版，修复圆心文字拥挤；像素分段进度环、硬边描边、羊皮纸内芯、阴影和高光均由 CSS gradient / custom properties / box-shadow 实现。未新增图片素材，未使用原型图或截图作为资源，业务逻辑、API、权限和状态处理保持不变。
# FE-INSTALL-IMAGE2-ICONS-2 状态
- `FE-INSTALL-IMAGE2-ICONS-2` completed：安装页 CSS 自绘图标已替换为从 image2 安装页原型提取/抠图生成的透明 PNG 小素材。新增 `frontend/public/assets/stardew/ui/install/` 下 6 个单图，覆盖顶部状态土芽和五步时间线图标；未使用整页原型图作为背景或整块资源，页面纸卡、边框、分隔线、进度条和日志终端仍由 CSS 实现。已验证 `cd frontend; npm.cmd run build` 通过，并用临时 QA 页在内置浏览器检查 1280x900、390x760 和“安装游戏”展开表单交互，确认图标资源加载、无横向溢出、按钮文字不溢出、console error/warn 为空。
# FE-CARD-UNIFY-SAVES-1 状态
- `FE-CARD-UNIFY-SAVES-1` completed：除模组管理页外，Stardew 其他页面小框已统一为存档管理页卡片基准，使用暖色纸面、铜色 2px 边框、9px 圆角、内描边和轻微阴影；背景按最新反馈改为干净浅色高光 + 纯色纸面，不再铺密集点状纹理。同步收敛标题/说明字号、padding、gap 和窄屏容器查询，模组页 `.sd-mods-*` 主体卡片保持原状。已验证 `cd frontend; npm.cmd run build` 通过，并用已删除的临时 QA 页检查 1280x720/390x760 无横向溢出、无点状纹理、模组卡未误套新样式。
# FE-CARD-UNIFY-SAVES-1 follow-up
- `FE-CARD-UNIFY-SAVES-1` follow-up completed：总览页四个统计卡 `.sd-mc`（存档/模组/系统健康/运行任务）已按反馈仅移除点状 `radial-gradient` 纹理，保留原有结构、尺寸、边框、阴影和布局。已验证 `cd frontend; npm.cmd run build` 通过。
# FE-SERVER-PLAYERS-CARD-LAYOUT-1 状态
- `FE-SERVER-PLAYERS-CARD-LAYOUT-1` completed：服务器控制页已使用共享 `ServerSummaryCard` 替换原大状态卡并移除独立邀请码卡；玩家管理页已移除顶部摘要卡，将“服务器信息（Junimo）”置底；在线玩家表删除“角色”列，主机标识并入玩家名右侧，并新增可见“农场收入 / 玩家收入”列。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器 DOM 快照接口本次返回兼容错误，未完成截图式 QA。

# FE-PLAYERS-PROTOTYPE-CURRENT-1 状态
- `FE-PLAYERS-PROTOTYPE-CURRENT-1` completed：玩家管理页已继续按 version-02 `05-players.png` 原型校准卡片比例和首屏节奏，并通过玩家页专属 `.sd-main:has(.sd-players-page)` 收紧主 frame inset。在线玩家表为整行首块，标题改为 `在线: N` / `等待加入: N` 徽章；活动/最近事件与管理操作保持第二行左右两列，管理操作隐藏原型不存在的说明文字；Junimo 终端整行置底并在桌面首屏可见。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器 QA 覆盖 1536x1024 与 390x844，console error/warn 为空，桌面表格无自身横向滚动条，窄屏无页面级横向溢出。

# FE-MISSING-GAME-INSTALL-PROMPT-1 状态
- `FE-MISSING-GAME-INSTALL-PROMPT-1` completed：每次登录或已有 session 进入 Stardew 面板后，若实例状态未检测到游戏文件，会弹出“请先安装游戏”引导弹窗；主按钮跳转到 `/instances/stardew/install`。已安装状态、正在运行的 `stardew_install` 任务和当前已在安装页时不会触发该弹窗。已验证 `cd frontend; npm.cmd run build` 通过。

# STEAM-QR-PHASE-CLASSIFY-1 状态
- `STEAM-QR-PHASE-CLASSIFY-1` completed：Steam QR 扫码登录后端阶段识别已修正，`Choice [1]: 2` 不再误判为 Steam Guard 手机批准，QR 模式下保持 `steam_qr_required` 供前端展示扫码入口。现场诊断确认 steam-auth 容器基础网络可解析并连通 Steam Web API/CM 端口，当前失败属于 SteamClient QR 登录连接未建立而非 Docker 完全无网络。已验证 `cd backend; go test ./internal/games/stardew_junimo` 通过。

# FE-STEAM-QR-LOG-FALLBACK-1 状态
- `FE-STEAM-QR-LOG-FALLBACK-1` completed：安装页增加 QR 日志兜底，当后端阶段短暂误显 `steam_guard_mobile_required` 但最新日志证明用户选择 QR（例如 `Choice [1]: 2`）时，前端按 `steam_qr_required` 展示扫码交互，避免 QR 流程误显示 Steam Guard 手机批准。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-STEAM-QR-IMAGE-CODE-1 状态
- `FE-STEAM-QR-IMAGE-CODE-1` completed：安装页 QR 弹窗不再把终端字符画作为主扫码对象，而是从最新 `Or open: https://s.team/q/...` 提取 Steam 登录 URL 并用本地 `qrcode` 生成标准二维码图片；字符画仅作为图片生成失败时的备用。已验证 `cd frontend; npm.cmd run build` 通过，后端 QR 阶段识别相关定向测试通过。

# FE-STEAM-AUTH-OPTIMISTIC-PHASE-1 状态
- `FE-STEAM-AUTH-OPTIMISTIC-PHASE-1` completed：安装页 Steam 认证选择已加入本地乐观阶段，点击扫码登录/手机批准/输入验证码后立即切换到对应交互区，不再等待实例状态轮询；当前有效的 QR URL 日志可修正落后的选择按钮，但后续 Guard/下载/失败日志优先。已验证 `cd frontend; npm.cmd run build` 通过，后端 QR 阶段识别相关定向测试通过。
# STEAM-POST-AUTH-RETRY-1 状态
- `BE-STEAM-POST-AUTH-RETRY-1` / `FE-STEAM-POST-AUTH-RETRY-1` completed：Steam 登录成功后的下载/CDN/磁盘/后续安装失败现在归类为 `error/download_failed` 或 `error/post_auth_failed`，前端通过日志兜底识别认证已成功，并只允许复用已保存凭据重试，不再要求用户重新输入 Steam 账号密码。已验证 `go test ./internal/games/stardew_junimo -run "DownloadFailedAfterSuccessfulAuth|InstallMarksSteamAuthFailedWhenRunErrors"` 与 `cd frontend; npm.cmd run build`。
# STEAMCMD-PULL-PROGRESS-1 状态

- `STEAMCMD-PULL-PROGRESS-1` completed：Junimo 镜像拉取与 SteamCMD 兜底镜像拉取都会通过 `[pull:progress:done:total]` 给前端提供估算百分比；安装页顶部总进度、镜像拉取卡和任务日志详情可展示“约 N%”，避免用户只能看 layer 日志猜测。已验证 `go test ./internal/games/stardew_junimo -run "SteamCMD|DownloadFailed|InstallMarksSteamAuthFailedWhenRunErrors"`、`go test ./internal/docker ./internal/games/stardew_junimo/config` 与 `cd frontend; npm.cmd run build`。
# FE-STEAMCMD-BRACKET-PROGRESS-1 状态
- `FE-STEAMCMD-BRACKET-PROGRESS-1` completed：安装页已支持 SteamCMD 原生 `[ 28%] Downloading update (done of total KB)` 进度日志，并继续支持 SteamCMD 手机 App 批准提示。验证：`cd frontend; npm.cmd run build`。
# JUNIMO-IMAGE-CANDIDATES-1 状态
- `JUNIMO-IMAGE-CANDIDATES-1` completed：`server` 与 `steam-auth-cn` 镜像拉取已接入与 SteamCMD 类似的候选兜底机制，默认顺序为 `docker.1ms.run`、`docker.m.daocloud.io`、`ghcr.io`、原始仓库；本地已有任意候选会直接复用，拉取成功后写回 `.env`，避免后续 compose 回到单一镜像源。验证见后端接手文档。
# JUNIMO-IMAGE-CANDIDATES-2 已完成

- 已完成 JunimoServer 与 steam-auth cn 版镜像候选源自动补齐：旧实例单候选 `.env` 会被扩展为默认候选；steam-auth cn 当前顺序为 1ms、阿里云 ACR 新版个人版、DaoCloud、GHCR、Docker Hub。
- 已完成选中镜像写回：后端会把实际使用的 `SERVER_IMAGE` / `STEAM_SERVICE_IMAGE` 和补齐后的候选列表写回实例 `.env`，方便后续重试复用。
# FE-GAME-INSTALLED-STARTABLE-1 已完成

- 已修复安装完成态 `game_installed` 在前端不可启动的问题：总览页和服务器控制页都会把它作为可启动的未运行状态展示。
- 若没有可用存档，点击启动后仍由后端返回 `save_required` 并引导用户创建/上传/选择存档。
# FE-OPSRAIL-METRICS-RESTORE-1 状态
- `FE-OPSRAIL-METRICS-RESTORE-1` completed：右侧 OpsRail CPU / 内存 / 磁盘已恢复轻量实时显示，Stardew 面板挂载期间调用现有 `/api/instances/:id/metrics`，首次立即采样并按 `2s` 刷新；没有用户打开前端页面时自然不会产生浏览器轮询，页面卸载时停止 timer。普通 dashboard 初始化仍不触发 `/api/health/diagnostics`，保留此前诊断降轮询优化。验证：`cd frontend; npm.cmd run build`；Browser QA 打开 `qa-layout.html?state=running` 确认右侧栏显示 mock metrics 百分比而非空值。
# RELEASE-TAG-CI-1 状态
- `RELEASE-TAG-CI-1` completed：面板仓库已新增 GitHub tag 发版 workflow，推送 `v*` tag 后自动构建面板 Docker 镜像，发布 Docker Hub 与阿里云 ACR 的版本 tag / `latest`，并在 GitHub Release 上传 `deploy/run.sh`。配套 steam-service-cn 仓库已改造 tag workflow，可发布 `junimo-steam-service-cn` 到 Docker Hub、阿里云 ACR 和 GHCR。
# RUN-SH-QUICK-MODE-1 状态
- `RUN-SH-QUICK-MODE-1` completed：`deploy/run.sh` 已扩展为完整一键启动菜单，包含 Docker/Compose 自动安装修复、面板镜像候选兜底拉取、启动/停止/重启、普通更新/强制更新、镜像源切换、脚本自更新、虚拟内存、开机自启、状态/日志/访问地址。脚本默认使用宿主机 `~/.anxi-panel/data` 与容器内同名绝对路径持久化数据，避免面板通过 Docker socket 编排游戏容器时出现 bind mount 路径不一致。
- `RUN-SH-QUICK-MODE-1` docs follow-up：README 与镜像构建文档已补充最低系统要求、推荐配置、多人游玩规格和云服务器安全组口径；对外只要求开放 TCP `8090`、UDP `24642` / `27015`，VNC/noVNC TCP `5800` 按需开放，Junimo API TCP `8080` 明确不要开放公网。
- `RUN-SH-QUICK-MODE-1` docs follow-up：README 与镜像构建文档已补充“一键启动脚本”的国内加速安装入口，推荐国内用户通过自有轻量服务器静态分发 `run.sh`，GitHub Release 地址作为备用；Docker 镜像仍由脚本内候选源拉取，不通过该轻量服务器中转。
- `RELEASE-TAG-CI-1` follow-up：面板仓库 ACR 发布地址已切换到阿里云新版个人版实例域名 `crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com`；GitHub Actions 和 `deploy/run.sh` 默认国内镜像源同步更新。`ALIYUN_REGISTRY_USERNAME` 使用 ACR 访问凭证登录命令中的 `--username` 值。
- `RELEASE-TAG-CI-1` follow-up：配套 `junimo-steam-service-cn` tag 发布 workflow 已切换到同一 ACR 新版个人版域名；面板内 `STEAM_SERVICE_IMAGE_CANDIDATES` 默认把该 ACR 镜像放在第二候选，顺序为 1ms、ACR、DaoCloud、GHCR、Docker Hub。
- `RELEASE-TAG-CI-1` follow-up：面板仓库 tag 发版 workflow 已增加 GHCR 发布目标 `ghcr.io/anxiyizhi/stardew-server-anxi-panel`，并给 `deploy/run.sh` 增加 GHCR 镜像源选项；配套 steam-service-cn workflow 保持发布 `ghcr.io/<owner>/junimo-steam-service-cn`。
