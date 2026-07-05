# FE-OVERVIEW-HEALTH-SHARE-1 前端接手记录（2026-07-06）
## 改了什么
- 修复总览页“系统健康”卡在访问诊断页后仍显示 `— / 未检查` 的问题。
- `DiagnosticsPage` 成功完成初次健康检查或手动“重新检查”后，会调用公共数据层新增的 `applyHealthDiagnostics(res)`，把诊断结果同步到 `dashboardData.health`。
- 总览页继续不主动跑 `/api/health/diagnostics`，避免重新引入 Docker/Compose 版本等重诊断在普通 dashboard 初始化时的开销。
## 影响文件 / 接口
- `frontend/src/games/stardew/stardew-routes.ts`
- `frontend/src/games/stardew/useStardewDashboardData.ts`
- `frontend/src/games/stardew/pages/DiagnosticsPage.tsx`
- 未新增或修改后端 API；继续消费现有 `GET /api/health/diagnostics`。
## 如何验证
- `cd frontend; npm.cmd run build`
- Browser QA：打开 `http://127.0.0.1:5174/qa-layout.html?state=running`，确认初始总览健康卡为 `—`；点击左侧“诊断”，等待显示 6 项检查正常；再点击“总览”，确认健康卡显示 `100% / 6项全部通过 / 优秀`，console error/warn 为空。
## 下一步注意事项
- 后续不要为了修复总览展示而把 `refreshHealth()` 放回 `refreshAll()` 或全局轮询；如果有其它页面也主动产生健康诊断结果，应复用 `applyHealthDiagnostics()` 同步公共状态。

# FE-TOPBAR-BRAND-LIGHTER-1 前端接手记录（2026-07-06）

## 改了什么
- 按用户反馈，左上角 `Stardew Anxi Panel` 品牌标题再细一点。
- `.sd-topbar-brand-text` 字重从 `800` 调为 `700`，同时减少暗色描边/投影层数和不透明度，保留原有 28px 尺寸、黄色填充和像素标题感。

## 影响文件 / 接口
- `frontend/src/games/stardew/StardewPanel.css`
- 未改顶栏状态牌、存档框、版本框、用户框、React 结构、后端 API、权限、轮询或 Junimo 通信。

## 如何验证
- `cd frontend; npm.cmd run build`

## 下一步注意事项
- 顶栏标题仍是动态文字渲染；继续微调粗细时优先改 `font-weight` 与 `text-shadow`，不要把标题改成固定图片，否则版本/语言适配会变差。

# FE-PUBLIC-IP-INVITE-CARD-1 前端接手记录（2026-07-06）
## 改了什么
- 邀请码卡片下方新增“服务器公网 IP”检测行，显示后端 `GET /api/instances/:id/public-ip` 返回的服务器出口公网 IP。
- 公网 IP 行提供“复制”和“刷新”：复制只写入 IP 文本；刷新会调用 `refreshPublicIP(true)`，后端收到 `?refresh=1` 后强制重新探测。
- 按用户截图反馈，去掉邀请码下方“分享此代码邀请新玩家加入服务器”说明文字；公网 IP 行也保持无说明文案，避免小框被额外文字撑高。
- 上方邀请码行标题保持“邀请码”；下方公网 IP 检测行标题显示为“局域网邀请”。公网 IP 未检测/检测失败时不显示复制按钮，但操作区保留固定宽度，以保持两行值框宽度一致。
- 总览页“服务器控制”卡里的邀请/IP 组做了轻量上移，填掉标题右侧原本的大块留白；没有再收窄或改动服务器详情页摘要卡。

## 影响文件 / 接口
- `frontend/src/api.ts`
- `frontend/src/types.ts`
- `frontend/src/games/stardew/stardew-routes.ts`
- `frontend/src/games/stardew/useStardewDashboardData.ts`
- `frontend/src/games/stardew/InviteCodeCard.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 新增消费接口：`GET /api/instances/:id/public-ip`。未改邀请码接口、生命周期按钮、Junimo 通信或资源指标轮询。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`。
- 后端配套验证：`cd backend; go test ./internal/web`。

## 下一步注意事项
- 公网 IP 必须由后端检测，前端直接访问第三方 IP 服务会拿到用户浏览器所在网络的 IP。
- 如果接口返回 `public_ip_failed`，前端只显示“检测失败”并保留刷新入口；不要把外部服务错误直接展示给普通用户。

# FE-MOD-COUNT-FILTER-BUILTIN-1 前端接手记录（2026-07-06）
## 改了什么
- 总览页模组统计改为只统计用户可见 Mod，不再把 SMAPI runtime、`StardewAnxiPanel.Control`、`JunimoServer` / `JunimoHost.Server` 这三个内置运行组件算进“已启用”和停用统计。
- 总览页“模组”统计卡的大数字、`已启用 N 个` 文案、同步包摘要里的已启用/已停用数量现在都使用过滤后的列表。
- 将模组页原本的系统运行组件判断抽到 `mod-visibility.ts`，`OverviewPage` 和 `ModsPage` 复用同一套识别逻辑。

## 影响文件 / 接口
- `frontend/src/games/stardew/mod-visibility.ts`
- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- 未改后端 API、Mod 启用切换接口、同步包导出接口或 Junimo 通信。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`。

## 下一步注意事项
- 后续新增必须随面板运行的内置组件时，应扩展 `modIsSystemRuntime()`，确保模组页展示和总览统计继续保持同一口径。
- 玩家同步统计/导出仍保留完整列表逻辑；不要为了展示统计把基础运行依赖从同步包处理链路中移除。

# DOCKER-POLL-PERF-1 前端接手记录（2026-07-06）
## 改了什么
- 公共 dashboard 初始化移除了 `/api/health/diagnostics` 请求，避免普通总览入口触发 Docker/Compose version 检查。
- 右侧 OpsRail 不再常驻调用 `/api/instances/:id/metrics`，资源数字默认显示为空值/按需诊断入口，避免所有页面都持续触发 `docker compose stats --no-stream`。
- `DiagnosticsPage` 进入后主动执行一次健康检查；手动“重新检查”仍可再次执行。资源指标采样只在诊断页可见时运行，间隔 `8s`，浏览器 tab 隐藏时停止 timer，恢复可见时立即刷新一次。
- 总览页系统健康卡默认显示“未检查 / 进入诊断页后检查”，不再显示“检查中”。

## 影响文件 / 接口
- `frontend/src/games/stardew/useStardewDashboardData.ts`
- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/pages/DiagnosticsPage.tsx`
- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- 未新增 API；只是调整 `/api/health/diagnostics` 和 `/api/instances/:id/metrics` 的触发时机。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`。
- 后端配套验证：`cd backend; go test -count=1 ./internal/docker`。

## 下一步注意事项
- 后续不要把 `dashboardData.refreshHealth()` 加回普通初始化或全局轮询；Docker 版本诊断只应由 Diagnostics、安装前检查、Docker 状态页或用户手动刷新触发。
- 如果右侧栏以后要恢复资源数字，应复用诊断页采样结果或只做一次性展示，不要重新开全局 5s metrics 轮询。

# ASSET-RUNTIME-SLIM-1 前端/发布瘦身接手记录（2026-07-06）
## 改了什么
- Dockerfile 新增 `extension-builder` 阶段，在构建阶段生成 `browser-extensions/anxi-nexus-installer.zip`；最终 runtime 不再安装 `zip`，只复制构建好的扩展目录。
- `docs/prototypes/` 从完整历史截图目录改为轻量索引目录，只保留 `README.md`、`overview-design-baseline-2026-06-30.png` 和 `overview-current-baseline-2026-07-04.png`；完整原型截图和提取工作区迁出主仓，交给 Release artifact、对象存储或设计仓库保存。
- 对超过 300 KB 的运行 PNG 做一轮无损重压缩并逐张做像素等价校验；登录背景已回退为 PNG-only 加载，避免 AVIF/WebP 或重编码造成色调偏移。
- favicon 从单个 512px / 545 KB PNG 改为 `favicon.ico` + 32/64/128 PNG；默认 `favicon.png` 收敛为 128px。
## 影响文件 / 接口
- `Dockerfile`
- `frontend/src/App.css`
- `frontend/index.html`
- `frontend/public/favicon*`
- `frontend/public/assets/stardew/ui/backgrounds/*.png` 和一批被无损重压缩的运行 PNG。
- `docs/prototypes/README.md` 与两张基准图。
- 文档：`README.md`、`README.en.md`、`docs/03-frontend.md`、`docs/07-later-optimizations.md`、`docs/08-future-roadmap.md`、`docs/09-image-build.md`。
- 未改前端路由、业务 API、权限、轮询或 Junimo 通信。
## 如何验证
- PNG 重压缩脚本只在输出更小且 `RGBA` 像素差异为空时覆盖原文件。
- `docs/prototypes` 体积从 109 个文件约 71.38 MB 降到 3 个文件约 2.58 MB。
- 需要执行：`cd frontend; npm.cmd run build`。
- 建议执行：`docker build -t stardew-server-anxi-panel:local .`，并确认 runtime 层不再安装 `zip`，镜像内仍存在 `/app/browser-extensions/anxi-nexus-installer.zip`。
## 下一步注意事项
- 9-slice / tile 面板素材本轮只做无损压缩，没有改切片参数。若要继续瘦身，应单独做视觉回归，重点看边缘模糊、alpha、拉伸和平铺接缝。
- `scripts/extract-ui-assets.py` 仍会向 `docs/prototypes/assets/ui-extracted` 生成提取工作区；只在设计任务需要时运行，生成物不要默认入主仓。
- 生产代码仍不得引用 `docs/prototypes`；运行素材必须放在 `frontend/public/assets/...`。

# FE-CLEANUP-UNUSED-ASSETS-1 前端接手记录（2026-07-05）
## 改了什么
- 删除 `frontend/public/assets/stardew/ui/` 下 79 个前端源码零引用的旧 PNG 素材，释放约 20.96MB 仓库生产素材体积；清理后运行时素材目录约 18.56MB。
- 删除无引用前端组件：`CommandOutput`、`StatusPill`、`StatusBadge`、`InstanceStateCard`。
- 本地清理已忽略的 `.gocache/` 与 `tmp/`，释放约 1.82GB 工作区缓存；这两个目录不进入 Git 变更。
## 影响文件 / 接口
- 删除文件集中在 `frontend/public/assets/stardew/ui/**`。
- 删除组件文件：`frontend/src/core/CommandOutput.tsx`、`frontend/src/core/StatusPill.tsx`、`frontend/src/core/StatusBadge.tsx`、`frontend/src/games/stardew/InstanceStateCard.tsx`。
- 未改前端路由、API helper、页面业务逻辑、后端接口或 Junimo 通信。
## 如何验证
- 已执行前端素材复扫：排除动态 `new-game` 路径后，`frontend/public/assets` 非新建存档素材无零引用文件。
- 已执行：`cd frontend; npm.cmd run build`。
## 下一步注意事项
- `frontend/public/assets/stardew/new-game/` 有模板字符串动态引用，不能只按文件名静态搜索判断未使用。
- 后续 `ASSET-RUNTIME-SLIM-1` 已把 `docs/prototypes/` 改为轻量索引目录；完整历史原型截图不再保留在主仓。
- `qa-layout.html` / `qa-layout-main.tsx` 仍是当前回归验证入口，虽然不进生产构建，但不要在没有替代 QA 入口时删除。

# FE-MODS-HIDE-SYSTEM-RUNTIME-1 前端接手记录（2026-07-05）
## 改了什么
- 模组页现在把 SMAPI runtime、`StardewAnxiPanel.Control`、`JunimoServer` / `JunimoHost.Server` 识别为系统运行组件。
- 这些组件不再出现在“添加模组”的已安装卡片列表，也不再出现在“配置模组 / 当前存档 Mod 启用状态”开关列表，避免用户误以为可以配置或关闭面板自身依赖。
- 添加页“已安装”统计和解析失败统计只计算用户可见 Mod；当前只有系统运行组件时，添加页显示“当前没有可展示 Mod”，配置页显示“当前没有可配置 Mod”。
## 影响文件 / 接口
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- 未改后端 API、Mod 上传/删除/导出、启用状态 PUT、玩家同步包导出或 Junimo 通信。
## 如何验证
- 已执行：`cd frontend; npm.cmd run build`。
## 下一步注意事项
- 玩家同步统计仍基于完整 `mods` 列表，避免破坏完整同步包对基础运行依赖的既有处理。
- 后续若新增必须强制启用的系统组件，需要同步扩展 `modIsSystemRuntime()`，确保不要漏进用户可操作列表。

# FE-STEAMCMD-SELFUPDATE-PROGRESS-1 前端接手记录（2026-07-05）
## 改了什么
- `install-helpers.ts` 新增 `steamcmd_update` 进度分类。SteamCMD 在登录前输出的 `[steamcmd] [ N%] Downloading update (... of 40,273 KB)` 会被识别为客户端自更新。
- 安装页在该阶段显示“SteamCMD 正在更新客户端中…”，说明 Docker 镜像已经就绪，这不是镜像拉取；进入 `Logging in user` / `Waiting for user info` / app 安装后，进度再归类为游戏或 SDK 下载。
- 安装总进度说明优先使用 `calcSteamDownloadTaskProgress()` 的当前下载说明，避免主进度条仍显示“兜底下载游戏文件”。
## 影响文件 / 接口
- `frontend/src/games/stardew/install-helpers.ts`
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- 没有新增 API，仍从安装 job 日志和 SSE 里解析。
## 如何验证
- 已执行：`cd frontend; npm.cmd run build`。
- 后端配套验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallSteamCMD"`。
## 下一步注意事项
- SteamCMD 日志里同样叫 `Downloading update` 的内容要看上下文：登录前是客户端自更新，登录并进入 `app_update` 后才按游戏/SDK 进度处理。
- 手机 App 批准提示仍看 `steamcmd_guard_mobile_required`；如果日志出现 `Please confirm the login in the Steam Mobile app` 或 `Waiting for confirmation`，页面应展示打开 Steam App 批准。

# FE-STEAMCMD-RETRY-RESUME-1 前端接手记录（2026-07-05）
## 改了什么
- 安装页新增 `steamCMDRecoverable`，专门处理 `steamcmd_failed` / `steamcmd_image_pull_failed` 后的复用凭据重试。
- 主按钮显示“重试 SteamCMD 授权/下载”，表单标题显示“重试 SteamCMD 兜底下载”，提交按钮显示“确认重试 SteamCMD”。
- 表单提示明确：后端会直接复用已保存账号密码进入 SteamCMD 授权/下载，本地已有 SteamCMD 镜像时不会重新拉取。
## 影响文件 / 接口
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- 未新增 API 字段；仍通过 `reuseCredentials=true` 请求安装，后端根据实例 `driverPhase` 自动恢复 SteamCMD。
## 如何验证
- 已执行：`cd frontend; npm.cmd run build`。
- 后端配套验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallResumes|InstallUsesExistingLater"`。
## 下一步注意事项
- SteamCMD 授权提示仍由 `steamcmd_guard_mobile_required` / `steamcmd_guard_required` 驱动；不要用普通 `steam_guard_*` 文案覆盖。
- 如果新增 SteamCMD 可恢复 phase，前端也要同步判断是否属于 `steamCMDRecoverable`。

# FE-STEAMCMD-FALLBACK-1 前端接手记录（2026-07-05）

## 改了什么
- 安装页支持后端新增的 SteamCMD 兜底阶段：`steamcmd_image_pulling`、`steamcmd_auth_running`、`steamcmd_guard_choice_required`、`steamcmd_guard_required`、`steamcmd_guard_mobile_required`、`steamcmd_downloading`、`steamcmd_failed`、`steamcmd_image_pull_failed`。
- 当 `steam-auth` 下载失败后后端自动切换 SteamCMD，右侧认证区会明确提示“steam-auth 国内网络波动导致下载失败，SteamCMD 兜底需要重新授权”，避免用户误以为账号密码又错了。
- `steamcmd_guard_choice_required` 显示两个按钮：手机 App 批准、App / 邮箱验证码；`steamcmd_guard_required` 显示验证码输入框；`steamcmd_guard_mobile_required` 提示打开 Steam 手机 App 批准 SteamCMD 登录。
- `steamcmd_downloading` 显示兜底下载卡，说明 SteamCMD 正在复用已保存账号密码下载，进度以任务日志为准。

## 影响文件 / 接口
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- `frontend/src/games/stardew/install-helpers.ts`
- 未新增接口；仍复用安装 job SSE、`GET /api/jobs/:id/logs` 和 `POST /api/instances/:id/steam-guard/input`。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`。
- 联调建议：构造或真实触发 `steam-auth` 下载失败后进入 `steamcmd_guard_choice_required`，确认页面展示 SteamCMD 专属说明和两个选择；选择验证码后应切到 `steamcmd_guard_required`，提交验证码后任务继续。

## 下一步注意事项
- 不要把 `steamcmd_guard_*` 纳入原 QR 扫码流程；SteamCMD 不复用 `s.team/q` 二维码。
- 若后续解析 SteamCMD 百分比进度，应在 `install-helpers.ts` 新增独立解析器，不要复用 steam-auth 的 `Progress: N/M files` 正则。

# FE-STEAM-AUTH-DOWNLOAD-PROGRESS-RESTORE-1 前端接手记录（2026-07-05）
## 改了什么
- 修复 Steam 日志已经进入 `Downloading app 413150` / `Progress:` 后，安装页仍显示手机 App 批准登录的问题。
- 安装页 `effectivePhase` 新增下载日志优先级：安装中断/失败状态之后，若日志显示游戏下载或 SDK 下载，就切到 `game_downloading` / `steam_sdk_downloading`，压过旧的 Guard/QR 状态。
- 恢复游戏下载进度条：复用 `install-helpers.ts` 的 Steam 下载进度解析，在右侧 Steam 认证卡内显示文件数、体积和进度条。
- 同步修复历史 QR URL 抢占验证码输入：`Choice [1]: 2` 会根据最近菜单上下文解释，Guard 菜单下显示验证码输入而不是扫码。
## 影响文件 / 接口
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- 未改后端接口、安装 job、SSE、`POST /api/instances/:id/steam-guard/input` 契约或 Junimo 通信。
## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 内置 Browser QA：打开 `http://127.0.0.1:5173/qa-layout.html?state=running`，页面非空、无 framework overlay、console error/warn 为空、页面级横向溢出为 0。
- 现有 QA 壳没有活跃 `stardew_install` job 和 Steam 下载日志，真实场景需用一次活跃安装联调：手机批准后日志出现 `Progress: N/M files` 时，应显示下载卡进度条，不应继续显示 Guard 批准。
## 下一步注意事项
- 后续不要用单一 `driverPhase` 或历史 QR URL 决定认证区显示；安装页需要按最新 job 日志修正后端阶段滞后。
- 如果 SteamCMD/steam-auth 的进度日志格式变化，需要同步更新 `install-helpers.ts` 的 `steamDownloadProgressRe`。

# FE-INSTALL-STALE-PHASE-1 前端接手记录（2026-07-05）
## 改了什么
- 安装页现在优先依据活跃 `stardew_install` job 判断是否正在安装；没有 queued/running 安装任务时，旧的运行中 `driverPhase` 会被视为 `install_interrupted`。
- 没有活跃安装任务时会自动加载最近一次安装任务详情和日志，避免页面底部日志空白但右栏显示 failed。
- `install_interrupted` 纳入失败/可重试渲染链路，状态条、步骤、进度条和按钮都会按中断失败处理。
## 影响文件 / 接口
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- 接口不变：继续使用 `GET /api/jobs`、`GET /api/jobs/:id`、`GET /api/jobs/:id/logs`、`GET /api/instances/:id/state`。
## 如何验证
- `cd frontend; npm.cmd run build`
- 手动场景：当 `instances.driver_phase=steam_auth_running` 但最新 `stardew_install` 已 failed 且无活跃安装任务时，安装页应显示安装中断/失败并加载最近失败任务日志，不应继续卡在 48%。
## 下一步注意事项
- 安装页上任何“正在运行”态都要同时核对活跃 job；不要只看 `instance.driverPhase`。
- QR/Steam Guard 交互只应在当前安装 job 活跃时展示，否则应落到最近失败任务的日志和重试入口。

# FE-OPSRAIL-MAINTENANCE-PHASE-1 前端接手记录（2026-07-04）

## 改了什么
- 按用户反馈，右栏“进行中”卡的计划维护不再到点后直接滚到下一天倒计时。
- 右栏现在按 `shutdownTime/startupTime` 派生当前维护窗口阶段：到关机点后显示 `关机中 / 等待关机结束`；关机结束但未到开机点时显示自动开机倒计时；到开机点后显示 `开机中 / 等待开机结束`；开机 job 成功后才回到下一天倒计时。
- 计划维护对应的 `stardew_lifecycle` job 会从普通进行中任务列表隐藏，避免和语义化维护阶段重复显示。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.tsx`
- `docs/03-frontend.md`
- `docs/frontend-handoff/frontend-handoff-2026-07-04.md`
- `docs/08-future-roadmap.md`
- 未新增接口；继续使用现有 `GET /api/instances/:id/restart-schedule`、实例状态、jobs 与 job logs。后端调度器、权限和 Junimo 通信未改。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 内置 Browser QA：打开 `http://127.0.0.1:5173/qa-layout.html?state=running`，确认 Stardew Shell 和右栏“进行中”正常渲染、无 Vite overlay、console error/warn 为空。
- 真实登录态当前停在登录页，未完成真实实例右栏截图；后续联调建议用关闭时间=当前后 1 分钟、开启时间=关闭后 1-2 分钟，观察右栏按 `自动关机 -> 关机中 -> 自动开机倒计时 -> 开机中 -> 次日倒计时` 切换。

## 下一步注意事项
- 后端 `nextShutdownAt/nextStartupAt` 在当前关闭点过后会计算下一轮窗口；右栏维护阶段不要只依赖这两个字段判断当前维护期。
- 如果后续后端为计划维护 job 增加明确 `displayName` 或 action 字段，可替换目前基于日志内容识别 `startup/shutdown` 的前端兜底。

# FE-OVERVIEW-METRIC-TYPE-UNIFY-1 前端接手记录（2026-07-04）

## 改了什么
- 按用户截图反馈，总览页四张统计卡（存档 / 模组 / 系统健康 / 运行任务）存在标题、数字、单位、徽章字体割裂的问题。
- 在 `StardewPanel.css` 文件尾部新增 `FE-OVERVIEW-METRIC-TYPE-UNIFY-1` 覆盖段，只限定 `.sd-ov-metric-strip .sd-mc`。
- 四张卡内部统一字体链为 Verdana / Microsoft YaHei / SimHei；标题收为 `14px/800`，数字从 `38px/900` 调整为 `34px/800`，单位、说明、状态徽章继承同一字体链并降低字号和字重差异。
- 数字阴影改为轻微高光 + 低透明投影，保留像素感但避免截图里那种过粗、突兀的数字风格。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/frontend-handoff/frontend-handoff-2026-07-04.md`
- `docs/08-future-roadmap.md`
- 未改 `OverviewPage.tsx`、路由、后端 API、权限、轮询、右侧状态栏或 Junimo 通信。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 内置 Browser QA：`http://127.0.0.1:5173/qa-layout.html?state=running`。
- 1536x1024：总览统计卡数量为 4，标题 computed font 为 `14px/800`，数字为 `34px/800`，字体链一致；console error/warn 为空，overlay 为 0，页面级横向溢出为 0。
- 交互：点击侧栏“服务器”进入服务器控制页，再点“总览”返回后统计卡仍为 4 张且无横向溢出。
- 390x844：四张统计卡单列显示，标题/数字字号仍稳定，无页面级横向溢出，console error/warn 为空。
- Browser `domSnapshot()` 仍触发既有 `incrementalAriaSnapshot` 兼容错误，本轮使用 evaluate、截图和 console logs 验证。

## 下一步注意事项
- 这组字体覆盖只应留在 `.sd-ov-metric-strip` 下，不要提升到全局 `.sd-mc`，否则会影响服务器、诊断、存档等其它已经按原型校准过的小卡片。
- 如果后续继续调总览统计卡，优先在 `FE-OVERVIEW-METRIC-TYPE-UNIFY-1` 段追加覆盖，避免回到前面多段历史 `.sd-mc` 规则里改来改去。

# FE-RESTART-SCHEDULE-PUT-WRITE-MODEL-1 前端接手记录（2026-07-04）

## 改了什么
- 修复服务器控制页“计划重启”弹窗保存时报 `request body must be valid JSON`。
- 根因是前端把 `GET /api/instances/:id/restart-schedule` 返回的完整 `schedule` DTO 原样作为 PUT 请求体，其中包含 `instanceId`、`nextShutdownAt`、`nextStartupAt`、`lastStatus` 等只读展示字段；后端 JSON 解码禁止未知字段，所以返回 `invalid_json`。
- 新增 `RestartScheduleUpdate` 窄类型，并在 `updateRestartSchedule()` 内显式只提交后端允许写入的 7 个字段。

## 影响文件/接口
- `frontend/src/types.ts`
- `frontend/src/api.ts`
- `docs/03-frontend.md`
- `docs/frontend-handoff/frontend-handoff-2026-07-04.md`
- `docs/08-future-roadmap.md`
- 接口路径不变，仍为 `PUT /api/instances/:id/restart-schedule`；后端契约、调度器、权限判断、弹窗 UI 和 Junimo 通信未改。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 手动联调建议：管理员打开服务器控制页，进入“计划重启”，保存当前配置；Network 中 PUT body 应只包含 `enabled/shutdownTime/startupTime/timezone/warningMinutes/backupBeforeShutdown/skipIfPlayersOnline`，不应再包含 next/last/instanceId 字段。

## 下一步注意事项
- `RestartSchedule` 是读取 DTO，可带展示字段；`RestartScheduleUpdate` 是写入请求体。后续新增只读展示字段时，不要直接把读取 DTO 回传给 PUT。
- 后端保留 `DisallowUnknownFields()` 是有价值的契约护栏，前端新增保存接口时优先建立独立写入模型。

# FE-PLAYERS-OFFLINE-ROSTER-COUNT-1 前端接手记录（2026-07-04）

## 改了什么
- 配合后端 `PLAYERS-SAVE-ROSTER-1`，玩家页现在会正确接收并展示存档中存在但当前不在线的玩家，例如 farmhand `test`。
- 在线玩家标题中的“等待加入”徽章改为只统计 `waiting/pending/joining` 状态；不再把 `offline` 离线玩家算作等待加入。
- 状态文本和状态点复用同一个等待状态判断：等待为黄点，在线为绿点，其它状态包括离线为灰点。

## 影响文件/接口
- `frontend/src/games/stardew/pages/PlayersPage.tsx`
- `docs/03-frontend.md`
- `docs/frontend-handoff/frontend-handoff-2026-07-04.md`
- 未改接口路径、玩家表列、管理操作权限、轮询或 Junimo 通信。后端新增表现是 `GET /api/instances/:id/players` 可能返回 `source=save_file` 的离线名册行。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 后端配套测试：`cd backend; go test ./internal/games/stardew_junimo`，通过。

## 下一步注意事项
- `offline` 是存档名册行，不是“等待加入”。如果未来要展示等待加入玩家，应依赖后端明确状态或继续只使用 `waiting/pending/joining`。

# FE-STOPPED-STATUS-RED-1 前端接手记录（2026-07-04）

## 改了什么
- 按用户截图反馈，总览页和服务器控制页的 `已停止` 状态文字改为红色，不再显示为运行态绿色。
- 总览页“服务器控制”状态行的状态值增加 `sd-lifecycle-status-val-stopped` 类，停止态状态点同步为红点。
- 服务器控制页顶部 `ServerSummaryCard` 的服务器状态增加 `sd-server-summary-state-stopped` 类。
- 服务器控制页生命周期卡补回一行小状态：`状态 · 已停止`，状态值增加 `sd-server-lifecycle-status-val-stopped` 类，位置对齐用户截图中的生命周期控制区域。

## 影响文件 / 接口
- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/games/stardew/ServerSummaryCard.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端 API、生命周期 start/stop/restart handler、权限判断、轮询、邀请码刷新或 Junimo 通信。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 内置 Browser QA：`http://127.0.0.1:5173/qa-layout.html?state=stopped`。
- 默认视口总览页：`.sd-lifecycle-status-val` 文案为 `已停止`，computed color 为 `rgb(192, 32, 32)`，页面非空，页面级横向溢出为 0，console error/warn 为空。
- 点击左侧“服务器”：页面标题为“服务器控制”，生命周期状态行和摘要卡状态文案均为 `已停止`，computed color 均为 `rgb(192, 32, 32)`，console error/warn 为空。
- 390x844：总览和服务器页 stopped 状态文字仍为红色，页面级横向溢出为 0。截图留在 `C:/Users/anxi/AppData/Local/Temp/stardew-status-red-qa/`。
- Browser `domSnapshot()` 仍触发既有 `incrementalAriaSnapshot` 兼容错误，本次用 evaluate、截图和 console logs 完成验证。

## 下一步注意事项
- 停止态红字只挂在 `stopped` 状态类上；`ready_to_start` 仍按“准备启动”处理，不等同染红。
- 如果后续继续调整服务器控制页生命周期卡，不要删掉 `.sd-server-lifecycle-status`，否则用户截图里的小状态反馈会再次消失。

# FE-OVERVIEW-LIFECYCLE-LEFT-1 前端接手记录（2026-07-04）

## 改了什么
- 按用户截图反馈，总览页“服务器控制”区的启动按钮已从生命周期区域中间移到左侧。
- 启动/停止/重启按钮行现在和“服务器控制”标题、下方“状态：已停止/运行中”等状态行使用同一个左边距。
- 根因是旧 `.sd-lifecycle-actions` 规则残留了 `flex-wrap: wrap`，并且后置样式里仍有 `align-content: center` 生效；纵向 flex 容器的 flex line 被横向居中，导致按钮看起来漂在中间。
- 本次在总览页最终覆盖段补充 `align-content: flex-start`、`flex-wrap: nowrap`，并让 `.sd-lifecycle-btns` 显式 `align-self: flex-start`。

## 影响文件 / 接口
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改 `OverviewPage.tsx`、启动/停止/重启 handler、后端生命周期 API、邀请码刷新、权限判断、轮询或 Junimo 通信。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 内置 Browser QA：`http://127.0.0.1:5173/qa-layout.html?state=stopped`。
- 默认视口：服务器控制卡内启动按钮相对卡片左边距从修改前 `155px` 变为 `12px`，与标题左边距 `12px` 一致；状态行也为 `12px`；console error/warn 为空，无 Vite overlay。
- 交互验证：点击启动按钮后按钮文案变为“启动中…”，按钮仍保持 `12px` 左边距，console error/warn 为空。
- 390x844：启动按钮相对卡片左边距为 `9px`，标题也是 `9px`，页面级 `overflowX=0`，console error/warn 为空。

## 下一步注意事项
- 这块总览生命周期区仍复用全局生命周期按钮素材；后续如果继续调总览页，不要恢复 `.sd-lifecycle-actions` 的 `flex-wrap: wrap`，否则 `align-content` 会再次让按钮行居中。
- 独立服务器控制页的生命周期按钮有 `.sd-server-lifecycle` 专属规则，本次未改那一页。

# FE-SAVES-BACKUP-POLICY-LAYOUT-1 前端接手记录（2026-07-04）

## 改了什么
- 按用户截图反馈修正存档页“自动备份策略”卡片内文字错乱。
- 定时备份项现在按“勾选框 / 定时备份 / 每天 / 时间选择框”同一行排列，符合用户期望的阅读顺序，并在 250px 窄策略卡内保持不溢出。
- 每日快照保留行拆出 `sd-save-backup-slider-label`，数值 `N 天` 和标签稳定成组，右侧 range 滑杆使用剩余宽度。
- 备份双栏容器增加 `align-items: start`，左侧策略卡按内容高度显示，不再被右侧备份列表拉伸。

## 影响文件 / 接口
- `frontend/src/games/stardew/SavesSection.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改备份策略保存 handler、备份列表刷新、恢复/删除、创建/上传/选择存档、权限判断、后端 API 或 Junimo 通信。

## 如何验证
- 已执行：清理过期 `frontend/node_modules/.tmp/*.tsbuildinfo` 后 `cd frontend; npm.cmd run build`，通过。
- QA mock 全壳：打开 `http://127.0.0.1:5173/qa-layout.html`，点击“存档”，截图 `C:/Users/anxi/AppData/Local/Temp/saves-policy-layout-after.png`。
- 1536x1024 下策略卡为 `250x179`，定时备份行内部顺序为 checkbox、`定时备份`、`每天`、`04:00` select，`scheduleOverflow=false`，console error/warn 为空。

## 下一步注意
- 这张策略卡是存档页底部左侧窄卡，后续不要把“定时备份”拆成独立下一行，否则会再次和用户期望的“定时备份 / 每天 / 时间框”顺序冲突。
- 如果以后新增周计划或多时间点，请优先扩展为弹窗或下拉配置，不要继续向这条 250px 行内塞更多字段。

# FE-PLAYERS-ACTION-ICONS-IMAGE2-1 前端接手记录（2026-07-04）

## 改了什么
- 按用户截图反馈，玩家页“玩家活动 / 最近事件”里每页 3 条导致文字被挤压，本轮改为每页 2 条，并提高事件行高度、允许标题徽章换行、描述文本使用正常行高。
- 管理操作四个图标从 CSS 临时图形替换为 image2/Stardew 像素风 PNG：踢出玩家、封禁玩家、白名单管理、权限设置。
- 图标生成流程：内置 imagegen 生成 2x2 sprite sheet，纯色绿底；使用 `$CODEX_HOME/skills/.system/imagegen/scripts/remove_chroma_key.py` 抠透明，再切成 4 个 192x192 PNG。
- 生成源图保留在 `C:/Users/anxi/.codex/generated_images/019f2cb5-bb4a-7363-ab3a-60ec96231f8d/ig_0dbdba6a9fb9cd0e016a48f39872948191b271978581713fe0.png`；项目内透明 sheet 和四个最终图标均已落到 `frontend/public/assets/stardew/ui/icons/`。

## 影响文件/接口
- `frontend/src/games/stardew/pages/PlayersPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/public/assets/stardew/ui/icons/icon_players_action_sheet_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_players_action_boot_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_players_action_ban_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_players_action_whitelist_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_players_action_permission_image2.png`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端 API、玩家事件响应、管理操作权限、disabled 状态、轮询或 Junimo 通信。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 内置 Browser QA：`http://127.0.0.1:5173/qa-layout.html`，点击“玩家”进入 mock 全壳玩家页。
- 1536x1024：活动区每页 2 条，事件行高度 `68px`，描述 `scrollHeight == clientHeight` 未裁切；活动卡与管理卡均为 `260px` 高；4 个 `.sd-players-action-icon` 背景 URL 均指向新 PNG；分页从 `1/3` 可切到 `2/3`；console error/warn 为空。
- 390x844：活动/管理自然单列堆叠，无页面级横向溢出；表格仍只在自身容器横向滚动；console error/warn 为空。
- Browser `domSnapshot()` 仍存在 `incrementalAriaSnapshot` 兼容错误，本轮以 evaluate、screenshot 和 console logs 验证。

## 下一步注意事项
- 玩家活动分页现在是每页 2 条；如果后续要恢复 3 条，需要同步加高活动/管理卡或重新设计更紧凑的双行事件排版，否则会再次压字。
- 四个图标是生成式位图资产，当前 CSS 只引用 192x192 透明 PNG 并以 `42px` 显示；不要恢复旧的 CSS `linear-gradient`/`clip-path` 图形。
- `icon_players_action_sheet_image2.png` 是透明 sheet，方便后续统一回看来源；实际 UI 引用四个独立 PNG。

# FE-TOPBAR-SAVE-STATUS-TYPE-1 前端接手记录（2026-07-04）

## 改了什么
- 按用户连续截图反馈优化 Stardew Shell 顶栏。
- 品牌标题先尝试过艺术斜体，用户反馈太大；随后改为像素标题风格；用户再次反馈过粗并希望再小一点，最终改为 Verdana 系轻一点的像素描边：`28px`、`font-weight: 800`、1px 暗色描边和较短投影。
- `running/stopped` 顶栏状态按钮不再使用原来的通用九宫格 + 文本点位，改为直接使用已有素材 `panel_status_running_image2.png` / `panel_status_stopped_image2.png`，让“运行中/已停止”更像游戏内状态牌。其它非 running/stopped 状态仍走原有文字和点位。
- 存档框移除右侧下拉箭头；农场图标左移贴近边框；文本从单独存档名/“世界：”改为 `农场名：简略游戏时间`，例如 `AnxiFarm：第一年春`，只取 `gameYear + gameSeason`，不展示具体日期。
- 用户角色框移除右侧下拉箭头，绿点移到更靠右的位置；点击行为仍保持进入设置页。

## 影响文件 / 接口
- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端 API、存档数据结构、权限判断、路由、轮询或 Junimo 通信。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。
- 内置 Browser QA：`http://127.0.0.1:5173/qa-layout.html?state=running` 和 `?state=stopped`。
- 1536x720：顶栏状态背景分别为 `panel_status_running_image2.png` / `panel_status_stopped_image2.png`；存档框显示 `AnxiFarm：第一年春`；存档框和用户框下拉箭头数量为 `0`；标题字体为 `28px / 800`；页面级 `overflowX=0`；无 Vite overlay；console error/warn 为空。
- 390x844：顶栏仍按移动端策略隐藏存档和用户框，状态牌可见，`overflowX=0`，console error/warn 为空。

## 下一步注意
- 顶栏标题目前仍是动态文字渲染，不是图片字。如果后续要求更接近 Stardew Valley 原版 Logo，建议单独引入可授权的像素字体或生成文字贴图，不要再用系统粗黑字体硬描边。
- 存档框时间显示只依赖现有 `SaveInfo.gameYear/gameSeason`；后端字段缺失时只显示农场名，不应为了顶栏展示扩展 API。

# FE-MODS-PROTOTYPE-V02-LAYOUT-1 前端接手记录（2026-07-04）

## 改了什么
- 按用户“一个一个页面来、原型图放什么卡片就放什么卡片和比例”的要求，把模组管理页对齐 `C:/Users/anxi/.codex/generated_images/019f2c2c-8909-7262-bd81-d31356799c21/_sorted_overview_to_settings/version-02-current-frontend-code/06-mods.png`。
- 下载页首屏顺序锁为：顶部标题/刷新/导出/上传 -> 三段标签页 -> Nexus 连接横条 -> 搜索 Nexus Mods 卡 -> 搜索结果 2x2 卡片 -> 分页。
- 搜索结果卡高度从动态分页阶段的 `246px` 收回到 `198px`，桌面主宽下两列两行贴近原型；移动端恢复单列自然滚动。
- 下载卡移除额外 `N站会员专属安装` 按钮，恢复原型里的“跳转 N站 / 一键安装”两按钮结构；一键安装仍走现有浏览器扩展 batch 流程，Nexus Key 配置入口保留在连接条。
- 按用户截图反馈移除下载页底部“扩展安装进度”横条和搜索区“全部类别”下拉框。
- 搜索框提示改为“输入英文模组名称、ID 或关键词...”；热门标签改为 `UI Info`、`Fishing Mod`、`Backpack Upgrades`、`Tractor`，仍不做假占位，点击会写入搜索框并触发现有 Nexus 搜索。
- 模组页工作台、连接条、搜索卡和结果卡复用其它页面统一羊皮纸卡片变量，卡片为 2px 铜色边框、9px 圆角和统一内阴影/底部阴影，不再使用普通浅色 1px 框。
- 搜索结果卡前置状态从按钮区下方移动到统计行，固定跟在“认可”后面。无前置也显示 `前置：无`，有前置继续显示 `前置已满足 / 缺少前置mod` 并保留弹层；这样每张卡的“跳转 N站 / 一键安装”按钮位置保持一致。

## 影响文件 / 接口
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端 API、上传/删除/导出 Mod、启用/禁用、玩家同步包、任务轮询或 Junimo 通信。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。
- 内置 Browser QA：`http://127.0.0.1:5173/qa-layout.html?state=running`，点击“模组”。
- 1536x1024：结果卡 4 张，连接条/搜索卡/2x2 结果/分页均在原型式顺序；`selectCount=0`、`progressCount=0`、`premiumButtons=0`；结果卡 computed style 为 `2px rgb(160, 108, 44)` 边框和 `9px` 圆角；页面级 `overflowX=0`；无 Vite overlay。
- 前置对齐验证：第一行两张卡操作区同为 `y=628`，第二行同为 `y=836`；前置状态文本出现在统计行，例如 `下载 6,100,000认可 2,600前置已满足`。
- 390x844：页面级 `overflowX=0`，标签页、连接条、搜索卡和结果卡单列堆叠，内容可纵向滚动；`selectCount=0`、`progressCount=0`。
- 交互：点击热门标签 `Tractor` 后，搜索框值变为 `Tractor`，结果卡仍正常渲染。
- 注意：本轮 Browser dev log 缓冲区保留了热更新期间的旧 `nexusInstallJobId` 错误；源码已无该引用，重新 `npm.cmd run build` 通过，最终页面 `overlayCount=0`。

## 下一步注意
- 继续对齐模组页时优先保留本轮文件尾部 `FE-MODS-PROTOTYPE-V02-LAYOUT-1` 覆盖段，不要把模组页 198px 搜索卡高度提升为全局 Nexus 卡高度。
- 如果未来要恢复 Nexus Premium 直连安装或扩展安装进度条，请先设计它在原型卡片里的位置；不要再把第三个按钮或进度横条塞回搜索结果卡/底部首屏，否则会破坏 2x2 首屏比例。

# FE-DIAGNOSTICS-GAUGE-INNER-SAFE-1 前端接手记录（2026-07-04）

## 改了什么
- 按用户反馈修正诊断页资源趋势圆环卡“弧线挡住数字”的问题，保持当前三张竖向卡片和原型比例，只调整圆环内部排版安全区。
- `.sd-diag-page .sd-diag-gauge-ring` 的最小宽度提高到 `98px`，中心 `.sd-diag-gauge-core` 扩大到圆环 `68%`，用羊皮纸底色圆覆盖弧线内侧；数字与百分号略微降字号，避免 `27.1%` 一类小数值贴到右侧红弧。

## 影响文件 / 接口
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。
- 未改 `DiagnosticsPage.tsx`、资源指标接口 `/api/instances/{id}/metrics`、健康检查接口、导出诊断包、轮询逻辑、权限逻辑或 Junimo 通信。

## 如何验证
- `cd frontend; npm.cmd run build`
- 内置 Browser QA：打开 `http://127.0.0.1:5181/qa-layout.html?state=running`，点击左侧“诊断”，确认 CPU/内存/磁盘圆环卡数字不再被弧线遮挡，console error/warn 为空。

## 下一步注意
- 后续如果继续严格贴 prototype 的诊断页整体比例，保留这层数字安全区覆盖；不要只把圆环缩小，否则 0.72 shell scale 下小数百分比会再次贴近弧线。

# FE-PLAYERS-TIME-EVENTS-PAGING-1 前端接手记录（2026-07-04）

## 改了什么
- 按用户反馈继续修玩家管理页：在线时长列不再优先展示长 `onlineFor` 字符串，而是尽量显示短时间点 `今天 HH:mm / 昨天 HH:mm / N天前 HH:mm`。
- 时间点来源优先级：`connectedAt` / `onlineSince` / `joinedAt`，其次兼容 QA/mock 的 `onlineSeconds` + `playersData.updatedAt` 推算；无法计算时回退 `onlineFor` 或离线 `lastSeen`。
- 在线玩家表收入列顺序对调为“玩家收入 / 农场收入”，表头和行数据已同步调整。
- “玩家活动 / 最近事件”改为每页 3 条分页，底部提供上一页/下一页和页码；桌面固定活动卡与右侧管理操作卡同高，窄屏恢复自然高度。
- 本地未跟踪 QA harness `frontend/src/qa-layout-main.tsx` 增加 6 条 mock 玩家事件，用来验证分页与昨天/几天前显示。

## 影响文件/接口
- `frontend/src/games/stardew/pages/PlayersPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/src/qa-layout-main.tsx`（未跟踪 QA mock）
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端 API、玩家数据轮询、权限判断、Junimo 通信或 Stardew 业务逻辑。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 内置 Browser QA：`http://127.0.0.1:5173/qa-layout.html`，点击“玩家”进入 mock 全壳玩家页。
- 1536x1024：console error/warn 为空；表头为“玩家名 / 位置 / 在线时长 / 玩家收入 / 农场收入 / 状态 / 操作”；第一行时间为 `今天 12:15`，收入为 `28,230g` 后接 `128,640g`；活动卡和管理卡均为 `248px` 高；分页从 `1 / 2` 点击“下一页”后切换到 `2 / 2`，下一页按钮禁用。
- 390x844：console error/warn 为空；无页面级横向溢出；玩家表仍只在表格容器内部横向滚动。
- Browser `domSnapshot()` 仍存在 `incrementalAriaSnapshot` 兼容错误，本轮以 locator、evaluate、console logs 和截图验证。

## 下一步注意事项
- 后端当前没有稳定上线时间字段；如果后续补 `connectedAt` / `onlineSince`，前端会自动优先使用。不要再把长中文时长字符串直接塞进在线时长列。
- 活动分页目前是纯前端分页，基于接口返回的 `recentEvents` 切片；如果事件量未来超过当前后端最多 50 条，再考虑服务端分页。
- 玩家页专属 CSS 位于 `FE-PLAYERS-PROTOTYPE-CURRENT-1` 附近，本次是在该段继续覆盖，不要提升为全局表格规则。

# FE-SETTINGS-API-PORT-REMOVE-1 前端接手记录（2026-07-04）

## 改了什么
- 按用户截图反馈，设置与审计页“端口信息”卡片移除了只读的“API 端口”字段。
- 端口信息现在只展示“面板端口 / VNC 端口 + 保存/刷新”，避免和面板端口重复露出同一个端口号。
- 端口行 CSS 从三端口列收紧为两端口列，按钮仍在右侧，移动端仍沿用既有换行规则。

## 影响文件/接口
- `frontend/src/games/stardew/pages/SettingsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端 API、VNC 端口读取/保存、管理员权限判断、轮询、Junimo 通信或其它设置项。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。
- 重点检查设置页端口信息卡不再出现“API 端口”，面板端口与 VNC 端口仍可正常展示，管理员保存/刷新 VNC 端口按钮仍保留。

## 下一步注意事项
- 如果未来需要重新展示后端 API 监听信息，应先明确它和“面板端口”的差异；不要再直接复用 `panelPort` 做一个重复只读字段。

# FE-SAVES-UPLOAD-BLUE-BG-1 前端接手记录（2026-07-04）

## 改了什么
- 存档页上传横条（“拖拽存档文件到此处或点击上传”）背景恢复为之前的蓝色天空版本。
- `.sd-saves-page .sd-saves-upload-strip` 改回蓝色渐变、白色像素云块、木色实线边框与内高光。
- 仅调整视觉背景；上传按钮、上传弹窗、ZIP 预览、导入并启动、运行中禁用和管理员权限逻辑均未改。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.css`
- 未改 TSX、API、handler、权限、轮询或后端逻辑。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。
- QA 使用现有 `frontend/qa-layout.html` mock fetch 渲染真实 `StardewPanel` 存档页；1536x1024 截图确认上传条为蓝色天空背景，console error/warn 为空。

## 下一步注意事项
- 该规则位于存档页 image2 皮肤段，后续继续调存档页时不要再用 `FE-SAVES-V02-PROTOTYPE-LAYOUT-1` 的最终覆盖把上传条改回羊皮纸背景。

# FE-SETTINGS-PROTOTYPE-V02-LAYOUT-2 前端接手记录（2026-07-04）

## 改了什么
- 按用户最新要求，设置与审计页继续对齐 `C:/Users/anxi/.codex/generated_images/019f2c2c-8909-7262-bd81-d31356799c21/_sorted_overview_to_settings/version-02-current-frontend-code/09-settings.png`，重点修正“原型图这个地方放什么卡片、什么比例，现在就放什么卡片什么比例”。
- 页面卡片固定为左列“面板版本 / 用户管理 / 端口信息 / 其他设置”，右列“安全与权限 / 审计日志 / 安全建议”，桌面比例约 `1.11fr / 0.89fr`。
- 面板版本卡新增右侧图像槽；安全与权限改为单列表格式；端口信息当时改为三端口横排，后续 `FE-SETTINGS-API-PORT-REMOVE-1` 已移除重复的“API 端口”；审计日志首屏查询 7 条；安全建议改为三条带状态徽章和底部按钮。
- 在设置页作用域内恢复原型方角纸卡、角钉和细表格线，避免后置统一小卡片规则把设置页改成圆角通用卡。
- 通过 `.sd-main:has(.sd-settings-page)` 仅对设置页收紧主 frame 上下 inset，让 1536x1024 下七张卡片进入首屏。

## 影响文件/接口
- `frontend/src/games/stardew/pages/SettingsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端 API、用户管理/审计/VNC 端口权限判断、轮询、session 或 Junimo 通信。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 真实 `/instances/stardew/settings` 当前受登录页阻挡，因此使用已删除的临时 `settings-qa.html` + `src/settings-qa-main.tsx` mock 入口渲染真实 `StardewPanel` 到设置页。
- 1536x1024：console error/warn 为空；设置 section 数为 7；卡片顺序与原型一致；左列四卡、右列三卡完整进入首屏；无页面级横向溢出；已用 `view_image` 对比原型图与最终实现截图。
- 390x844：console error/warn 为空；右栏隐藏；设置页单列；`documentElement` 横向溢出为 0。
- 交互验证：点击“新建用户”后真实创建表单展开，表单输入数量为 2，仍无横向溢出。

## 下一步注意事项
- 本次最终覆盖集中在 `StardewPanel.css` 文件尾部 `FE-SETTINGS-PROTOTYPE-V02-LAYOUT-2` 段；后续继续逐页对齐时，不要把设置页的表格行高、端口横排或 `.sd-main:has(.sd-settings-page)` inset 提升为全局规则。
- `AUDIT_PAGE_SIZE` 从 20 改为 7 是为匹配原型首屏审计表高度；如果未来要做可配置页大小，应在审计卡内加专用分页/密度控件，不要恢复为一次塞 20 条。
- 临时 QA 入口已经删除；若需要长期保留设置页 mock QA，应和现有 `qa-layout.html` harness 统一设计。

# FE-SERVER-PROTOTYPE-V02-LAYOUT-2 前端接手记录（2026-07-04）

## 改了什么
- 按用户最新要求，服务器控制页继续对齐 `C:/Users/anxi/.codex/generated_images/019f2c2c-8909-7262-bd81-d31356799c21/_sorted_overview_to_settings/version-02-current-frontend-code/02-server.png`，重点修正“原型图这个地方放什么卡片、什么比例，现在就放什么卡片什么比例”。
- 顶部服务器摘要从玩家页六宫格统计卡改为服务器专用整行摘要卡：标题“服务器摘要”，一排展示服务器状态、在线玩家、当前存档、主机农民、游戏日期，下方是一条“邀请码”横条。
- 中部布局锁为原型比例：生命周期控制左列、快捷操作右列；全服消息左列；控制台命令底部横跨整行，终端输出区域改为满宽。
- 快捷操作从大 PNG 按钮堆叠改为原型式浅色工具行，包含图标、标题、说明/状态；保留原按钮的 handler、权限、disabled 与待接入逻辑。
- 移动端补回更靠后的 `@container sd-main-scroll (max-width: 880px)` 单列规则，避免新增桌面 grid 覆盖旧的窄屏顺序。

## 影响文件/接口
- `frontend/src/games/stardew/InviteCodeCard.tsx`
- `frontend/src/games/stardew/ServerSummaryCard.tsx`
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端 API、Stardew/Junimo 通信、权限判断、轮询、生命周期/备份/VNC/say/command handler。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 内置浏览器 QA：临时 `server-qa.html` + `src/server-qa-main.tsx` mock 入口渲染真实 `StardewPanel` 到 `/instances/stardew/server`，数据为原型运行态；QA 后临时文件已删除。
- 1536x1024：console error/warn 为空；主内容 `937px`；摘要约 `933x175`；生命周期约 `590x116`；快捷操作约 `330x321`；全服消息在左列；控制台命令约 `933x268`，终端满宽约 `901px`，主要内容进入首屏。
- 390x760：console error/warn 为空；`documentElement` 横向溢出为 0；服务器页单列顺序为摘要 -> 生命周期 -> 快捷操作 -> 全服消息 -> 控制台命令，快捷操作宽度贴合主滚动区。

## 下一步注意事项
- 本次最终覆盖在 `StardewPanel.css` 文件尾部 `FE-SERVER-PROTOTYPE-V02-LAYOUT-2` 段；后续继续“一个一个页面”对齐时，不要把服务器摘要/快捷操作的专用规则提升为玩家页或总览页全局规则。
- `InviteCodeCard` 新增的 `label/description` 仅是展示参数；总览页仍走默认文案。后续如果其它页面复用时需要不同标题，优先传参而不是复制组件。
- 临时 QA 入口已经删除；若需要长期保留服务器页 mock QA，应和现有 `qa-layout.html` harness 统一设计，避免未跟踪 `src/*qa*.tsx` 影响普通构建。

# FE-PLAYERS-PROTOTYPE-CURRENT-1 前端接手记录（2026-07-04）

## 改了什么
- 按用户要求，把玩家页继续对齐 `C:/Users/anxi/.codex/generated_images/019f2c2c-8909-7262-bd81-d31356799c21/_sorted_overview_to_settings/version-02-current-frontend-code/05-players.png` 的卡片位置和比例。
- 玩家页顶部在线玩家标题改为原型式徽章：`在线: N` 和存在非 online 名册行时的 `等待加入: N`；精确接入状态不再额外显示“已接入”徽章，避免破坏标题宽度。
- 通过 `.sd-main:has(.sd-players-page)` 仅对玩家页收紧主 frame inset，让 1536x1024 QA 下主内容回到约 `x=232/w=995/y=90`，接近原型纸面位置。
- 在线玩家表、活动/最近事件、管理操作、Junimo 终端按原型顺序锁定：表格整行，第二行左活动/右管理，底部终端整行。管理操作区隐藏原型没有的底部说明文字，只保留 2x2 操作卡和待接入徽章。
- 表格列宽和最小宽度收紧，1536x1024 QA 下无表格横向滚动条；移动端仍只允许表格容器内部横向滚动，不造成页面级横向溢出。
- 收入展示兼容 QA mock 里的 `farmMoney` / `personalMoney` 字段作为回退，不改后端接口契约。

## 影响文件/接口
- `frontend/src/games/stardew/pages/PlayersPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/src/server-qa-main.tsx`：补齐未跟踪 QA 入口 mock user 的 `isSuperAdmin`，否则当前 `tsc -b` 会失败。
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端 API、玩家数据轮询、权限判断、Junimo 通信或 Stardew 业务逻辑。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 内置浏览器 QA：`http://127.0.0.1:5173/qa-layout.html`，点击左侧“玩家”进入 mock 全壳玩家页。
- 1536x1024：console error/warn 为空，无页面级横向溢出；在线玩家徽章为 `在线: 3` / `等待加入: 1`；4 行玩家表可见，表格 `clientWidth=955`、`scrollWidth=955`；活动/管理双列并排，Junimo 终端在首屏可见。
- 390x844：console error/warn 为空，`documentElement/body` 均无横向溢出；活动与管理操作改为单列，玩家表仅在自身容器内横向滚动。
- Browser DOM snapshot 接口本轮仍有兼容错误，已用 Browser locator、evaluate、console logs 和截图完成验证。

## 下一步注意事项
- 本次最终覆盖集中在 `StardewPanel.css` 文件尾部 `FE-PLAYERS-PROTOTYPE-CURRENT-1` 段；后续继续逐页对齐时，不要把玩家页的表格宽度、管理卡隐藏说明、两列比例提升为全局规则。
- 如果后端未来提供明确“等待加入”字段，应替换当前“非 online 名册行数”的前端派生逻辑。
- `qa-layout.html` / `qa-layout-main.tsx` 仍是未跟踪 QA harness；若后续决定保留，应正式纳入工程或移出 `src`，避免临时入口继续影响普通构建。

# FE-DIAGNOSTICS-PROTOTYPE-V02-LAYOUT-1 前端接手记录（2026-07-04）

## 改了什么
- 按用户要求，把诊断页布局对齐 `C:/Users/anxi/.codex/generated_images/019f2c2c-8909-7262-bd81-d31356799c21/_sorted_overview_to_settings/version-02-current-frontend-code/07-diagnostics.png` 的卡片位置和比例。
- 通过 `.sd-main:has(.sd-diag-page)` 仅对诊断页收紧主 frame inset，让页面内容回到原型的纸面位置：1536x1024 QA 下主内容从约 `x=284/w=881` 回正到 `x=242/w=975`。
- 顶部状态横卡加高到约 `975x160`，三枚统计卡仍在状态卡右侧一行；中部检查项与资源趋势恢复等宽双列，各约 `482x392`；底部“告警与建议”横跨全宽并保留在首屏内。
- `DiagnosticsPage.tsx` 中 `GaugeCard` 的 DOM 顺序改为“标题 -> 圆环 -> 说明”，趋势图标题改为“资源使用趋势（24小时）”；其余健康检查、导出诊断包、metrics 轮询、管理员权限和错误/loading 状态逻辑不变。
- 资源趋势内部高度收紧：三枚 gauge 卡约 `143x174`，趋势图从偏高的大块压为原型式短图。

## 影响文件/接口
- `frontend/src/games/stardew/pages/DiagnosticsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改 API、后端 handler、权限、数据字段、轮询间隔或右侧栏数据。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 因本轮没有可用 IAB 控制工具，使用 Playwright + 本机 Chrome 回退验证。
- QA 入口：`http://127.0.0.1:5181/qa-layout.html?state=running`，点击左侧“诊断”进入 mock 全壳诊断页。
- 1536x1024：console error/warn 为空，无横向溢出；关键尺寸 `status=975x160`、`check=482x392`、`resource=482x392`、`advice=975x180`。
- 390x844：console error/warn 为空，`documentElement/body` 横向溢出均为 0。
- 截图留在本地 QA 目录：`.codex/qa/diagnostics-after-2.png`、`.codex/qa/diagnostics-mobile.png`。

## 下一步注意事项
- 后续如果继续逐页按 version-02 原型对齐，不要把本次 `.sd-main:has(.sd-diag-page)` 的 frame inset 覆盖提升为全局，否则会影响其它页面已经校准过的布局。
- 如果将来需要支持不支持 `:has()` 的旧浏览器，应改为在 `StardewPanel.tsx` 给 `.sd-main` 增加当前 route class，再迁移这组诊断页专属 inset 覆盖。
- 诊断接口当前没有“最后检查时间”字段，本次没有为了原型中的示例时间硬造数据；如产品需要该行，应先扩展后端响应和前端类型。

# FE-OVERVIEW-BANNER-SCENE-IMAGE2-1 前端接手记录（2026-07-04）

## 改了什么
- 总览页顶部农场横幅场景改为直接使用 image2 原型 `01-overview.png` 的对应场景裁切图。
- 新增 `frontend/public/assets/stardew/ui/sprites/overview_banner_scene_image2.png`，来源为 `C:/Users/anxi/.codex/generated_images/019f2c2c-8909-7262-bd81-d31356799c21/_sorted_overview_to_settings/version-02-current-frontend-code/01-overview.png` 中的总览顶部农场场景。
- `StardewPanel.css` 文件尾部新增最终覆盖，让 `.sd-ov-banner-bg` 直接铺满新 PNG，并隐藏旧的 CSS 田野/云层伪元素和旧 `sprite_farmhouse_scene.png` 叠层。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/public/assets/stardew/ui/sprites/overview_banner_scene_image2.png`
- 未改 `OverviewPage.tsx`、路由、API、权限、轮询或后端逻辑。

## 如何验证
- 已预览新 PNG，尺寸 `1015x170`，只包含农场横幅场景，不包含底部统计条或页面外壳。
- `cd frontend; npm.cmd run build` 通过。

## 下一步注意事项
- 如果后续继续微调总览横幅，优先替换 `overview_banner_scene_image2.png` 或调整文件尾部 `FE-OVERVIEW-BANNER-SCENE-IMAGE2-1` 覆盖段，避免回到前面多段旧规则里的 CSS 田野和 `sprite_farmhouse_scene.png` 叠加方案。
- 该素材是固定场景图；若未来需要按存档 farmType 动态展示真实农场预览，应新增单独数据/素材通道，不要直接复用这张原型横幅。

# FE-SAVES-V02-PROTOTYPE-LAYOUT-1 前端接手记录（2026-07-04）

## 改了什么
- 存档管理页按 version-02 原型 `03-saves.png` 回正卡片位置和比例。
- 激活存档区从“单张大卡内塞按钮”改为原型式“左侧激活信息卡 + 右侧独立操作卡”，启动/导出/手动备份按钮保留原 handler 和禁用逻辑。
- 存档库标题右侧新增原型式工具按钮组（新建游戏 / 上传存档 / 刷新），列表不再混入“创建新存档”卡；桌面主宽下三张存档卡固定同排。
- 上传条保留在存档库下方，文案和按钮改为拖拽/选择文件入口；备份区改为底部两列：自动备份策略窄卡 + 备份列表宽卡。
- `saveFarmMapSrc()` 增加中文 farmType alias，兼容 mock/旧数据返回“标准农场、河边农场、森林农场”等中文值时缩略图不显示。

## 影响文件/接口
- `frontend/src/games/stardew/SavesSection.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 未改 API、权限判断、轮询、创建/上传/选择/删除/导出/备份/恢复业务逻辑。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。
- QA 使用现有 `frontend/qa-layout.html` mock fetch 渲染真实 `StardewPanel`：
  - 1536x1024 Edge/Playwright 截图对照 `C:/Users/anxi/.codex/generated_images/019f2c2c-8909-7262-bd81-d31356799c21/_sorted_overview_to_settings/version-02-current-frontend-code/03-saves.png`，确认激活区约 160px、右操作卡独立、存档库 3 卡同排、上传条和底部备份双栏落位。
  - 390x844 截图无页面级横向溢出，右操作卡和存档卡按单列堆叠。
  - console error/warn 为空，无 Vite overlay。

## 下一步注意事项
- 后续继续逐页按 version-02 原型回正时，存档页最终覆盖集中在 `FE-SAVES-V02-PROTOTYPE-LAYOUT-1` 段；不要把“创建新存档”卡重新放回存档卡网格。
- 存档页当前仍使用已有农场类型预览图资源，季节化缩略图不是本轮新增资源；如要完全还原原型中的春/夏/秋/冬缩略图，需要单独补季节素材。

# FE-PROTOTYPE-SHELL-ALIGN-1 前端接手记录（2026-07-04）

## 改了什么
- 目标：把九页前端布局对齐 image2 version-02 原型（`C:/Users/anxi/.codex/generated_images/.../version-02-current-frontend-code/01..09`）。用户原话："现在的布局大小太丑"，允许改卡片大小。
- 根因定位：1536 视口下右信息栏被设成 `414px`（`--sd-opsrail-width: clamp(340px,27vw,430px)`），左导航 `252px`，加上主 frame 厚留白，主内容区（`.sd-main-scroll`）只有 **791px**。这才是"太丑"的真正原因：总览统计卡被迫 2×2、服务器控制区与邀请码上下堆叠、服务器/任务/玩家页该并排的区块坍成单列。
- 关键修复（一处全局生效）：
  - `--sd-sidebar-width`: `clamp(210px,16.8vw,252px)` → `clamp(196px,14vw,216px)`
  - `--sd-opsrail-width`: `clamp(340px,27vw,430px)` → `clamp(268px,19vw,300px)`
  - 主内容区 791 → **937px**，总览恢复 4 卡一行、服务器控制 + 邀请码并排，全九页不再拥挤。栏宽比例经用户确认。
- 逐页对齐：
  - 顶栏：版本框 `width 9.1%→11.4%` + `white-space:nowrap`，`v1.6.15 (Stable)` 单行。
  - 总览：`.sd-overview-invite-card` 邀请行在 `(max-width:1180) and (min-width:901)` 段收敛列宽/字号/按钮宽，窄列不再裁切。
  - 服务器：坍缩断点 `1180px→880px`（937 下恢复 2 列）；`.sd-server-quick-grid` 横向 flex-wrap → 纵向列表，对齐原型右列竖排。
  - 任务日志：坍缩断点 `940px→820px`（937 下恢复 列表|详情 两列）。
  - 玩家：把 `FE-PLAYERS-LIST-LEFT-1` 段重写为 `FE-PLAYERS-PROTOTYPE-LAYOUT-2`——在线玩家表整行、活动(左)|管理(右)两列、Junimo 终端整行；坍缩 `900px→820px`。**逆转了 FE-PLAYERS-LIST-LEFT-1。**
  - 诊断：`.sd-diag-main-grid` `1.08fr|0.92fr`、检查行列宽重排、`.sd-diag-check-msg` 单行 `nowrap+ellipsis`，信息列不折行。
  - 设置：`.sd-settings-user-row`、`.sd-settings-audit-*` 列宽/gap 收紧，操作按钮与 IP 列不裁切。
  - 存档：`.sd-saves-upload-strip` 蓝色邮筒渐变 → 羊皮纸虚线拖拽区。

## 影响文件/接口
- 仅 `frontend/src/games/stardew/StardewPanel.css`。
- 未改任何组件 TSX、handler、API、权限、轮询或后端逻辑（纯布局/皮肤）。

## 如何验证
- `cd frontend; npm run build`（`tsc -b && vite build`）通过。
- QA 方法：临时 mock-fetch harness（`frontend/qa-layout.html` + `frontend/src/qa-layout-main.tsx`，拦截 `window.fetch` 按端点返回原型态数据，渲染真实 `StardewPanel` 全壳）+ 全局安装的 Playwright 1536×1024 逐页截图，与九张原型逐块对比。已确认九页结构一致、无拥挤/裁切/溢出、console pageerror 为 0。QA 临时文件用完已删除。
- 真实登录态截图 QA 待补（QA 走的是 mock，未走真实后端登录态）。

## 下一步注意事项
- 各页 `@container sd-main-scroll` 坍缩断点（服务器 880 / 任务 820 / 玩家 820）是按新的 937px 主宽校准的。若以后再调 `--sd-opsrail-width` / `--sd-sidebar-width`，务必同步复核这几个断点，否则会又落回单列。
- 存档库缩略图、模组搜索结果、总览横幅是运行时数据/素材，QA 里为 mock/占位，本次未处理其内容，只保证布局容器正确。
- 复用同一套 QA harness 思路（mock `window.fetch` + 真实 shell + Playwright）可低成本复现任意页面的桌面态，后续布局回归可直接照搬。
# FE-MOBILE-NAV-BAR-SIZE-1 前端接手记录（2026-07-04）

## 改了什么
- 按用户截图反馈，单栏状态下最上面的横向选择栏已经适量放大。
- `@media (max-width: 640px)` 下 shell 导航行从 `40px` 提到 `48px`，导航栏 padding/gap 略增。
- 移动端导航按钮从 `36x30` 提到 `42x38`，图标从 `20px` 提到 `23px`，视觉上更容易点按但仍保持图标式横向选择栏。

## 影响文件 / 接口
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/frontend-handoff/frontend-handoff-2026-07-04.md`
- `docs/08-future-roadmap.md`
- 未改 `StardewPanel.tsx`、导航路由、页面组件、后端 API、权限、轮询或 Junimo 通信。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 内置 Browser QA：`http://127.0.0.1:5173/qa-layout.html?state=stopped`。
- 490x844：顶部横向选择栏实际渲染高度约 `35px`，单个按钮约 `30x27px`，图标约 `17x17px`，`documentElement/body` 横向溢出均为 `0`，console error/warn 为空。
- 390x844：点击导航栏“服务器”后 active route 变为 `server`，横向溢出仍为 `0`，console error/warn 为空。
- Browser `domSnapshot()` 仍有既有兼容错误，本次使用 evaluate/截图/console 完成验证。

## 下一步注意事项
- 这组尺寸只应留在 `max-width: 640px` 单栏断点内，不要提升为桌面侧栏规则。
- 如果后续继续加大按钮，要同步检查 390px 窄屏下 9 个入口的横向滚动体验，避免误改成页面级横向溢出。

# FE-JOBS-LOG-SCROLL-LOCK-1 前端接手记录（2026-07-04）

## 改了什么
- 修复点击“任务与日志”后整个 Stardew Shell 被浏览器外层页面向下滚走的问题；顶栏不再消失，页面底部不再露出黑色空白。
- 在 `App.css` 中仅对包含 `.sd-shell` 的运行态页面锁住 `body/#root`：`height: 100dvh` + `overflow: hidden`，抵消缩放 Shell 的未缩放布局盒子造成的页面级纵向滚动。
- 将 `JobsLogsPage.tsx` 的日志自动滚到底从 `scrollIntoView()` 改为滚动 `.sd-jobs-log-window` 自身。
- 同步把 `InstallPage.tsx` 的安装日志自动滚动改为滚动 `.sd-install-log-window` 自身，避免同类问题在安装页出现。

## 影响文件/接口
- `frontend/src/App.css`
- `frontend/src/games/stardew/pages/JobsLogsPage.tsx`
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- `docs/03-frontend.md`
- `docs/frontend-handoff/frontend-handoff-2026-07-04.md`
- `docs/08-future-roadmap.md`
- 未改任务/安装 API、SSE、轮询、权限判断、路由、后端或 Junimo 通信。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 内置 Browser QA：`http://127.0.0.1:5173/qa-layout.html?state=running`，视口 1112x920。
- 修复前复现：点击“任务日志”后 `window.scrollY=351`，`.sd-shell.getBoundingClientRect().top=-351`，与用户截图一致。
- 修复后验证：初始和点击任务日志后 `documentElement.scrollHeight=clientHeight=920`，`body/root overflow=hidden`；强制 `window.scrollTo(0, 600)` 后 `window.scrollY` 仍为 `0`，`.sd-shell.top=0`；console error 为空。

## 下一步注意事项
- Shell 内部日志、终端、列表需要自动滚动时，不要直接对内部锚点调用 `scrollIntoView()`；应滚动最近的局部滚动容器，例如日志窗口自身。
- `body:has(.sd-shell)` 依赖现代浏览器的 `:has()`；如果未来要兼容不支持 `:has()` 的旧浏览器，建议在 `StardewPanel` 挂载时给 `body` 或 `#root` 加显式运行态 class。

# FE-MISSING-GAME-INSTALL-PROMPT-1 前端接手记录（2026-07-04）

## 改了什么
- 每次进入 Stardew 面板后，如果实例状态加载完成但没有检测到 Stardew 游戏文件，会在 Stardew Shell 上弹出“请先安装游戏”引导；首次注册管理员、普通登录和已有 session 刷新进入面板都覆盖。
- 弹窗主按钮“去安装游戏”调用现有 `navigate('install')`，直接跳转到安装界面；“稍后”只关闭本次登录/面板挂载内的提示。
- 若游戏已经安装，或 `stardew_install` 安装任务正在 `queued/running`，或当前已经在安装页，则不会弹窗。

## 影响文件/接口
- `frontend/src/App.tsx`
- `frontend/src/games/stardew/StardewPanel.tsx`
- `docs/03-frontend.md`
- `docs/frontend-handoff/frontend-handoff-2026-07-04.md`
- `docs/08-future-roadmap.md`
- 未新增或修改后端接口；安装页、Steam 认证、实例状态轮询、权限和 Junimo 通信保持不变。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 已用临时 mock QA 服务覆盖首次注册后缺游戏文件路径：提交管理员表单后出现“请先安装游戏”弹窗，点击“去安装游戏”后地址切到 `/instances/stardew/install`。
- 手动联调建议：在已有管理员账号但未安装游戏的环境中重新登录；若 `/api/instances/stardew/state` 返回 `admin_created` 等未安装状态，也应出现同一弹窗。

## 下一步注意事项
- 该提示是“每次登录/进入面板后”的一次性体验；同一次面板挂载中点“稍后”后不再重复弹，重新登录或刷新进入面板会重新判断。
- 判断“已安装”时继续与安装页保持同一组状态：`game_installed/save_required/ready_to_start/starting/running/stopped`。

# FE-STEAM-QR-LOG-FALLBACK-1 前端接手记录（2026-07-04）

## 改了什么
- 安装页增加 QR 日志兜底：当 `driverPhase` 仍显示为 `steam_guard_mobile_required`，但最近安装日志能证明用户选的是 QR（例如 `[steam] Choice [1]: 2` 或“已选择扫码登录”）时，页面改按 `steam_qr_required` 展示。
- 兜底会避开真实 Guard 菜单：如果最近日志已经出现 `Steam Guard Authentication`、`Approve in Steam` 或 `Enter code`，仍按 Guard 流程显示。
- 顶部当前阶段、进度文案和右侧认证交互区都使用 `effectivePhase`，避免用户选择 QR 后还看到“Steam Guard 验证 / 手机 App 批准”。

## 影响文件/接口
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- `docs/03-frontend.md`
- `docs/frontend-handoff/frontend-handoff-2026-07-04.md`
- `docs/06-integration.md`
- `docs/08-future-roadmap.md`
- 未新增或修改后端接口；继续使用实例状态、安装 job 日志和 SSE。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 复现场景：安装日志包含 `Choose authentication method`、`[2] QR Code (Steam Mobile App)` 和 `Choice [1]: 2`，即使状态短暂为 `steam_guard_mobile_required`，安装页也应显示“Steam 手机扫码”。

## 下一步注意事项
- 这是前端防御性兜底；根因仍由后端 `STEAM-QR-PHASE-CLASSIFY-1` 保持 `steam_qr_required`。后续如果 Steam 上游菜单文案变化，要同步更新日志识别条件。

# FE-STEAM-QR-IMAGE-CODE-1 前端接手记录（2026-07-04）

## 改了什么
- 安装页 Steam QR 弹窗从“展示终端字符画”改为“根据 Steam 登录 URL 本地生成标准二维码图片”。现在从最新 `Or open: https://s.team/q/...` 日志行提取 URL，用 `qrcode` 生成 320px 正方形 QR 图片。
- `extractQrPayload()` 不再要求字符画足够完整才启用扫码窗口；只要拿到最新 `s.team/q` 链接即可打开窗口。字符画只作为图片生成失败时的备用，不再作为主扫码对象。
- 新增 `.sd-install-qr-image-wrap` / `.sd-install-qr-image` 样式，提供浅色背景、quiet zone 和像素化渲染；备用 URL 单独显示在二维码下方。

## 影响文件/接口
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/src/types/qrcode.d.ts`
- `frontend/package.json`
- `frontend/package-lock.json`
- 未改后端接口、SSE、安装 job、Steam Guard 输入接口或 Junimo 通信。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 已执行：`cd backend; go test ./internal/games/stardew_junimo -run "SteamAuthMenus|SteamGuardCodePrompt|QRCodeChoice|SteamMobileApproval"`，通过。
- 手动联调时，选择 QR 后日志出现 `Or open: https://s.team/q/...`，点击“打开扫码窗口”应看到一张完整的浅底标准二维码图片，而不是多段或被裁切的终端字符画。

## 下一步注意事项
- 不要再把 `[steam]` 日志尾部整段塞进 QR 弹窗；终端字符画受字体和行高影响，不能作为可靠扫码源。
- 如果以后替换 QR 库，保留“从最新 `s.team/q` URL 生成图片”的契约即可；本地类型声明用于避免 `@types/qrcode` 引入 Node `Timeout` 类型污染前端工程。

# FE-STEAM-AUTH-OPTIMISTIC-PHASE-1 前端接手记录（2026-07-04）

## 改了什么
- 安装页新增 Steam 认证选择的本地乐观阶段：点击“扫码登录”后立即把右侧交互区切到扫码等待，不再停留在“扫码登录 / 账号密码”两个按钮上等待后端状态刷新。
- 同步覆盖 Steam Guard 二级选择：点击“手机 App 批准”立即显示手机批准等待；点击“输入验证码”立即显示验证码输入框。
- 日志里出现当前有效的 `https://s.team/q/...` 时，前端可按 QR 渲染来修正落后的选择按钮/后端阶段；但如果后续已经出现 Guard 验证码、手机批准、下载进度或失败状态，应以后续日志/phase 为准。
- 认证选择提交成功后主动刷新一次实例状态；提交失败则清空乐观阶段并展示错误。

## 影响文件/接口
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- 未改后端接口、安装 job、SSE、Steam Guard 输入接口或 Junimo 通信。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`，通过。
- 已执行：`cd backend; go test ./internal/games/stardew_junimo -run "SteamAuthMenus|SteamGuardCodePrompt|QRCodeChoice|SteamMobileApproval"`，通过。
- 手动联调时，在 `auth_method_required` 点击“扫码登录”后右侧应立刻变成“Steam 手机扫码 / 正在等待容器输出二维码”，日志出现 QR URL 后按钮立即可打开二维码窗口。

## 下一步注意事项
- 后续如果增加新的 Steam 认证选项，需同步扩展 `optimisticPhase` 的映射；不要让高延迟状态刷新成为按钮点击反馈的唯一来源。
- QR URL 日志兜底不能无条件最高；它只能避免底部已有当前二维码时右侧仍停在选择按钮，不能压过后续 Guard 菜单、验证码提示、下载进度或失败状态。
# FE-STEAM-POST-AUTH-RETRY-1 前端接手记录（2026-07-05）
## 改了什么
- 安装页新增日志兜底：出现 `[steam] Logged in as`、`Token expires`、`Game license verified`、`Got depot decryption key`、`Downloading app 413150` 或 `/data/game` 后，视为 Steam 认证成功。
- 认证成功后的 `download_failed`、`post_auth_failed` 或旧状态残留失败不再展示 Steam 账号/密码输入框；按钮改为“重试下载（不重新输入账号）”，表单只确认镜像版本并提交 `reuseCredentials=true`。
- 真正凭据错误仍由 `credentials_required` 处理；QR 登录失败仍允许用户改用账号密码。
## 影响文件 / 接口
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- `frontend/src/games/stardew/install-helpers.ts`
- 未新增 API；继续使用 `POST /api/instances/:id/install`。
## 如何验证
- `cd frontend; npm.cmd run build` 通过。
- 配合后端 `Logged in as -> Downloading app 413150 -> Download failed: ...403` 场景时，前端应显示下载失败和复用凭据重试，不显示 Steam 用户名/密码字段。
## 下一步注意事项
- 不要再用 `state=steam_auth_failed` 单独决定“需要重新输入账号密码”；必须结合 `driverPhase` 和最新安装日志判断是否已经认证成功。
# FE-PULL-PROGRESS-1 前端接手记录（2026-07-05）

## 改了什么
- 安装页现在会解析 `[pull:progress:done:total]` 隐藏日志并显示“约 N%”。
- 顶部安装总进度会在 `pull_running` 和 `steamcmd_image_pulling` 阶段跟随该估算推进；右侧镜像拉取卡同样显示百分比。
- 安装页普通日志窗口会过滤隐藏进度标记，避免用户看到内部控制行。

## 影响文件 / 接口
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- 未新增 API；继续消费 job logs / SSE。

## 如何验证
- `cd frontend; npm.cmd run build`

## 下一步注意事项
- `pull_running` 的 `done/total` 是镜像数量，`steamcmd_image_pulling` 的 `done/total` 是 layer 数量；前端应统一写“约 N%”，不要固定写“几个镜像”。

# FE-STEAMCMD-DOWNLOAD-PROGRESS-1 前端接手记录（2026-07-05）

## 改了什么
- `steamcmd_downloading` 阶段现在解析 `[steamcmd] ... progress: N (done / total)`，显示 SteamCMD 兜底游戏下载百分比。
- SteamCMD 输出 `Please confirm the login in the Steam Mobile app` 或 `Waiting for confirmation` 时，前端会展示手机 App 批准提示。
- SteamCMD 输出 `Success! App '413150' fully installed.` 时，游戏文件进度视为完成；`1007` 完成仅表示 Steam SDK 完成。

## 影响文件 / 接口
- `frontend/src/games/stardew/install-helpers.ts`
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- 不新增接口，仍从安装 job 日志解析。

## 如何验证
- `cd frontend; npm.cmd run build`
- 真实 SteamCMD 日志出现 `progress: N (done / total)` 时，安装页下载卡应显示百分比，不再只提示“以任务日志为准”。
- 真实 SteamCMD 日志出现 `Please confirm the login in the Steam Mobile app` 时，安装页应提示打开 Steam App 批准。

## 下一步注意事项
- SteamCMD 手机 App 批准超时会由后端落到 `steamcmd_failed`；前端不要把该状态继续渲染成下载中或安装完成。
# FE-STEAMCMD-BRACKET-PROGRESS-1 前端接手记录（2026-07-05）

## 改了什么
- 安装页 SteamCMD 兜底进度解析新增原生方括号格式：`[steamcmd] [ 28%] Downloading update (11,467 of 40,273 KB)...`。
- 该日志会被 `inferLatestSteamAuthLogPhase()` 识别为 `steamcmd_downloading`，右侧卡片展示 SteamCMD 兜底下载进度，而不是继续显示“正在等待 Steam 输出下载进度”。
- `Please confirm the login in the Steam Mobile app` / `Waiting for confirmation` 继续触发 SteamCMD 手机 App 批准提示。

## 影响文件 / 接口
- `frontend/src/games/stardew/install-helpers.ts`
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- 未新增或修改 API，继续消费安装 job 日志和实例 `driverPhase`。

## 如何验证
- `cd frontend; npm.cmd run build`
- 真实联调时，日志出现 `[steamcmd] [ 28%] Downloading update (11,467 of 40,273 KB)...` 后，安装页右侧应显示 SteamCMD 百分比进度。

## 下一步注意事项
- SteamCMD 后续如果再出现新的进度文案，优先扩展 `install-helpers.ts` 的独立 SteamCMD 解析器，不要复用 steam-auth 的 `Progress: N/M files` 规则。
# FE-GAME-INSTALLED-STARTABLE-1 安装完成态可直接启动

## 改了什么
- `game_installed` 现在在总览页和服务器控制页都作为可启动的未运行状态处理。
- 服务器控制页的 `isStopped` 包含 `game_installed`，因此安装完成但未启动时会显示可点击的“启动”按钮，而不是进入“当前状态下无法直接启动”的兜底提示。
- 总览页移除了 `game_installed -> 前往安装配置` 的特殊分支，避免重新安装成功后继续把用户导回安装页。

## 影响文件 / 接口
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- 未新增或修改后端接口；`POST /api/instances/:id/start` 仍由后端判断是否缺存档并返回 `save_required` / `active_save_required`。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`。
- 手动联调：实例状态为 `game_installed`、服务器未运行时，总览页和服务器控制页都应显示“启动”；点击启动后，有可用激活存档则进入启动/等待邀请码，没有存档则进入现有创建/上传存档提示。

## 下一步注意事项
- 后续新增安装完成后的中间态时，要同步检查总览页和服务器控制页的“可启动未运行状态”集合，不要只看 `stopped`。
# FE-OPSRAIL-METRICS-RESTORE-1 前端接手记录（2026-07-06）

## 改了什么
- 恢复右侧 OpsRail CPU / 内存 / 磁盘资源指标显示：Stardew 面板挂载期间调用 `/api/instances/:id/metrics`，首次立即采样，之后按 `2s` 间隔刷新；没有用户打开前端页面时自然不会产生浏览器轮询。
- 保留 `DOCKER-POLL-PERF-1` 的重诊断优化：普通 dashboard 初始化仍不主动调用 `/api/health/diagnostics`，Docker/Compose 版本检查继续留在诊断页/手动入口。
- metrics 请求失败时保留上一份右栏样本，不把短暂采集失败渲染成空值。

## 影响文件 / 接口
- `frontend/src/games/stardew/StardewPanel.tsx`
- 未新增 API；继续消费现有 `GET /api/instances/:id/metrics`。
- 未改诊断页趋势图、健康检查、权限、路由或 Junimo 通信。

## 如何验证
- `cd frontend; npm.cmd run build`
- Browser QA：打开 `qa-layout.html?state=running`，确认右侧栏 CPU/内存/磁盘显示 mock metrics 百分比，不再保持空值。

## 下一步注意事项
- 后续不要把 `/api/health/diagnostics` 放回 dashboard 全局初始化；右栏只需要轻量 metrics。
- 如要进一步降开销，优先调大右栏 refresh interval 或复用页面内采样结果，不要再次把右栏数值改成固定空值。
