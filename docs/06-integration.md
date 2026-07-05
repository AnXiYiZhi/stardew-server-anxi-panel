# STEAMCMD-REPAIR-DIRECT-1 联调契约

- 前端在“重新安装 / 修复”、认证后下载失败重试、SteamCMD 重试这类复用凭据入口中，应继续调用 `POST /api/instances/:id/install`，请求体为 `{ "reuseCredentials": true, "imageTag": "..." }`，不要再提交 Steam 用户名、密码或 VNC 密码。
- 后端收到 `reuseCredentials=true` 后会读取实例 `.env` 中的已保存凭据，并显式让 `stardew_junimo` driver 直达 SteamCMD 下载/校验路径：跳过 `steam-auth`，也跳过重新选择 Steam 登录方式。
- SteamCMD 直达修复预期使用已保留的 SteamCMD 授权缓存登录；正常联调日志应出现 `[steamcmd] 跳过 steam-auth，优先使用已保留的 SteamCMD 登录授权直接下载/校验。`，不应出现新的 `[steam]` / `steam-auth` 认证流程。
- 若 SteamCMD 缓存不可用，后端返回 `state=error, driverPhase=steamcmd_failed`，前端应提示查看日志/重试，不应展示 Steam 账号密码输入框或 `credentials_required` 文案。
- 验证：已安装实例点击“重新安装 / 修复”后，表单不出现凭据输入；提交后任务日志直接进入 `[steamcmd]`，不出现 `auth_method_required`、二维码、Steam Guard 选择或 steam-auth 容器登录。

# PUBLIC-IP-LOOKUP-1 联调契约

- 新增 `GET /api/instances/:id/public-ip`：任意已登录用户可调用，返回面板后端所在服务器检测到的公网出口 IP，而不是浏览器客户端 IP。
- 响应结构为 `{ "ip": string, "checkedAt": string, "source"?: string, "cached": boolean }`。默认返回后端 `10min` 成功缓存；前端点击刷新时请求 `?refresh=1` 强制重新探测。
- 后端只接受合法公网地址；外部检测服务失败、返回内网/非法地址或超时时，接口返回 `502 public_ip_failed`。前端应显示“检测失败”，并允许用户手动刷新。
- 该接口不依赖 JunimoServer、Docker Compose 或服务器运行状态；它检测的是面板容器/宿主当前出口公网 IP，主要用于用户配置端口转发、直连排查和确认服务器对外地址。
- 验证：`cd backend; go test ./internal/web`，`cd frontend; npm.cmd run build`。

# DOCKER-POLL-PERF-1 Docker 状态与资源轮询契约

- 后端 `ComposePs` 有默认 `1.5s` 短 TTL 缓存，供状态页、支持包、诊断等短时间重复读取复用。接口响应结构不变，前端无需感知缓存命中。
- `ComposeStats --no-stream` 仍只由 `/api/instances/:id/metrics` 触发，不做高频全局轮询。前端资源指标采样应只在诊断页/资源页可见时运行，刷新间隔保持 `5-10s`，当前实现为 `8s`。
- 浏览器 tab 隐藏时前端必须停止资源指标 timer；恢复可见后可以立即采样一次。非资源页和后台 tab 不应持续请求 `/metrics`。
- `/api/health/diagnostics` 会执行 Docker daemon 与 Compose 可用性检查，可能调用 `DockerVersion` / `ComposeVersion`；该接口只用于 Diagnostics、Docker 状态页、安装前检查或用户手动刷新，不进入普通总览初始化和常驻轮询。
- 支持包中的 `ComposeLogs` 仍是一次性 tail 导出；后续大日志 tail 或安装进度应优先保持流式/SSE，不要等待长命令完成后一次性返回。
- 验证：`cd backend; go test -count=1 ./internal/docker`，`cd frontend; npm.cmd run build`。

# SUPPORT-BUNDLE-STREAM-1 联调契约

- `POST /api/instances/:id/support-bundle` 仍由管理员触发，仍返回 `application/zip`，文件名形如 `support-bundle-YYYYMMDD-HHMMSS.zip`。
- 后端现在流式写 ZIP，不再设置 `Content-Length`。前端下载逻辑应以 HTTP 成功和 Blob 内容为准，不要依赖总长度或进度百分比。
- ZIP 内条目和脱敏语义保持不变：版本、健康检查、实例状态、近期任务、审计摘要、Compose 状态、Compose 配置摘要和 server 日志 tail。
- 如果后续支持包单个条目采集失败，应在 ZIP 内写入对应 error/note 条目；流式响应开始后不能再切换成 JSON 错误体。

# JUNIMO-MOD-MOUNT-RESTORE-1 联调契约

- `/data/Mods` 由宿主 `.local-container/mods` 挂载提供；后端必须保证其中包含官方 `JunimoServer` Mod，否则 Junimo API、邀请码和 VNC rendering 都不会就绪。
- `JunimoServer`、`StardewAnxiPanel.Control` 和虚拟 `SMAPI` 都是内置组件：前端应展示为已启用/不可切换，不参与“第三方 Mod 默认禁用”。
- 前端不应展示物理 `smapi` 文件夹；接口层会跳过该目录，只返回虚拟 `SMAPI` 卡。
- VNC 显示失败如果收到 `junimo_api_unavailable`，文案应提示“JunimoServer API 未就绪/官方组件未加载”，不要只显示 Docker 操作失败。

# ENV-BOM-NORMALIZE-1 联调契约

- 启动服务器前后，实例 `.env` 必须是 Docker Compose 可解析的普通 `KEY=value` 文件；如果混入 UTF-8 BOM 前缀，例如 `﻿IMAGE_VERSION`，旧流程会在 `docker compose up` 前置解析阶段失败。
- 后端 `ReadEnvFile` / `UpdateEnvFile` 已对 BOM 前缀 key 做归一化；前端无需新增接口，只需要把生命周期 job 的失败日志展示出来即可。
- 联调排查顺序：先运行 `docker compose -f data/instances/stardew/docker-compose.yml config --quiet` 验证配置解析，再看容器启动日志；不要只根据面板里的 `docker compose up: docker command failed` 判断是镜像或游戏进程问题。

# STEAMCMD-SELFUPDATE-PROGRESS-1 联调契约

- SteamCMD 兜底镜像命中本地后，job 日志会先出现“本地已有 SteamCMD 镜像 ... 直接使用”和“Docker 镜像检查已完成”；之后的 `[steamcmd] [ N%] Downloading update (... of 40,273 KB)` 属于 SteamCMD 客户端自更新，不代表重新拉 Docker 镜像。
- 前端根据登录前的 SteamCMD bracket progress 展示客户端自更新进度；进入 `Logging in user`、`Waiting for user info` 或 app 安装后，后续进度再按游戏/SDK 下载处理。
- SteamCMD 手机 App 批准仍以 `steamcmd_guard_mobile_required` 驱动；日志里的 `Please confirm the login in the Steam Mobile app` 和 `Waiting for confirmation` 都应让页面提示打开 Steam App 批准。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallSteamCMD"`；`cd frontend; npm.cmd run build`。

# STEAMCMD-RETRY-RESUME-1 联调契约

- 当 `steamcmd_failed` 或 `steamcmd_image_pull_failed` 后用户点击复用凭据重试，前端仍提交 `POST /api/instances/:id/install` 且 `reuseCredentials=true`；后端根据持久化 `driverPhase` 直接进入 SteamCMD fallback，不再先跑 `steam-auth`。
- 直达 SteamCMD 重试仍使用同一个 Steam Guard 输入接口：`POST /api/instances/:id/steam-guard/input`。前端看到 `steamcmd_guard_mobile_required` 时提示打开 Steam 手机 App 批准；看到 `steamcmd_guard_required` 时显示验证码输入框。
- 后端会先 inspect 所有 `STEAMCMD_IMAGE_CANDIDATES`。如果用户机器已有任意候选镜像，本次 job 日志会显示使用本地镜像并直接启动 SteamCMD；只有所有候选都缺失时才进入 `steamcmd_image_pulling`。
- 联调复现：先让 SteamCMD 手机批准超时，使实例落到 `state=error, driverPhase=steamcmd_failed`；再点安装页重试。预期不出现新的 `[steam]` / steam-auth 下载流程，不出现已存在 SteamCMD 镜像的 pull，直接出现 `[steamcmd] Logging in user...` 和授权提示。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallResumes|InstallUsesExistingLater"`，`cd frontend; npm.cmd run build`。

# STEAMCMD-FALLBACK-1 联调契约

- 安装任务中只要 `steam-auth` 已经登录成功并进入游戏下载阶段，后续任何游戏文件下载失败都由后端自动尝试 SteamCMD 兜底，不再把用户带回 Steam 账号密码表单。典型日志顺序为 `[steam] Logged in as -> Downloading app 413150 -> Download failed` 后继续出现 `[steamcmd] ...`。
- SteamCMD 兜底继续使用同一个 `stardew_install` job、同一条 SSE 流和同一个 `POST /api/instances/:id/steam-guard/input` 输入接口。前端只需要根据 `driverPhase`/日志展示 SteamCMD 专属授权 UI，不需要新增接口。
- SteamCMD 授权 phase：`steamcmd_guard_choice_required` 展示两个选择（`1`=手机 App 批准，`2`=App/邮箱验证码）；`steamcmd_guard_required` 提交验证码字符串；`steamcmd_guard_mobile_required` 只提示用户在手机 App 批准。提交成功后后端会乐观推进 phase，最终以 job 日志和实例 state 为准。
- SteamCMD 下载 phase：`steamcmd_image_pulling` 表示正在按 `STEAMCMD_IMAGE_CANDIDATES` 拉取兜底镜像，单个候选 403/超时会继续尝试下一个；`steamcmd_auth_running` 表示使用已保存账号密码登录；`steamcmd_downloading` 表示已授权并正在下载/校验 `413150`（并尝试 `1007` SDK）。
- 失败契约：`steamcmd_failed` / `steamcmd_image_pull_failed` 属于下载/环境失败，可重试并复用已保存凭据；`steamcmd_image_pull_failed` 表示全部候选镜像都不可用，运维可在实例 `.env` 中把可用内网镜像放入 `STEAMCMD_IMAGE_CANDIDATES`；`credentials_required` 表示 SteamCMD 认为账号、密码或验证码失败，前端应要求重新输入 Steam 凭据。
- 验证建议：模拟 `Logged in as -> Downloading app 413150 -> Download failed -> [steamcmd] Success! App '413150' fully installed. -> [steamcmd] Success! App '1007' fully installed.`，最终实例应为 `game_installed`，job 应为 succeeded。

# INSTALL-INTERRUPTED-STATE-1 安装任务与实例状态联调契约

- 安装页不能只相信 `instance.driverPhase` 判断任务是否仍在运行，必须同时看 `GET /api/jobs` 中是否存在 queued/running 的 `stardew_install`。没有活跃安装 job 时，残留的运行中 phase 应按 `install_interrupted` 展示。
- 后端启动恢复 interrupted jobs 时，`stardew_install` 会同步更新实例为 `state=error`、`driverPhase=install_interrupted`；steam-auth 容器运行错误会同步更新为 `state=steam_auth_failed`、`driverPhase=steam_auth_failed`。
- 前端收到 `install_interrupted` 应显示失败/可重试，不应继续显示 QR、Steam Guard 或“正在使用已保存凭据认证并下载游戏”。
- 验证：启动安装后中断面板进程再重启，最新 `stardew_install` job 应为 failed，安装页应显示中断并加载该 job 日志，而不是卡在 48%。

# FE-STEAM-AUTH-DOWNLOAD-PROGRESS-RESTORE-1 联调契约

- 前端安装页解释 Steam 认证/下载日志时，以最新日志上下文为准：认证方式菜单下的 `Choice [1]: 2` 表示 QR；`Steam Guard Authentication` 菜单下的 `Choice [1]: 2` 表示输入手机 App/邮箱验证码。
- 历史 `Or open: https://s.team/q/...` URL 只能作为当前 QR 阶段的兜底信号；如果后续日志已经出现 Steam Guard 菜单、`Enter Steam Guard code`、手机批准等待、下载开始或失败 phase，前端不得继续显示扫码窗口。
- 日志出现 `Downloading app 413150`、`Target directory: /data/game`、`Manifest contains` 或 `Progress: N/M files - done/total (...)` 后，前端应显示 `game_downloading` 下载卡。`Progress:` 日志应渲染文件数、体积和进度条；后续 SDK 下载同理显示 `steam_sdk_downloading`。
- 联调复现场景：手机批准后日志出现 `Logged in as ...`、`Downloading app 413150`、`Progress: 300/1470 files ...`，右侧认证区应显示“下载 Stardew Valley 游戏文件中…”和进度条，不应继续显示“请打开 Steam 手机 App，批准此次登录请求”。
- 验证：`cd frontend; npm.cmd run build`；活跃安装任务手动联调上述日志顺序。

# JOB-DISPLAY-NAME-1 联调契约

- `GET /api/jobs`、`GET /api/jobs/:id` 和 job SSE 的 job payload 可能返回 `displayName`；前端应优先展示该字段，没有时回退 `type`。
- `type` 仍是机器可读任务类型，例如 `mod_remote_install`，不要在前端用它判断具体 Mod；Nexus/远程 Mod 安装的用户可读名称在 `displayName` 中，例如 `mod_remote_install · Farm Type Manager (FTM)`。
- 扩展普通一键安装提交 `POST /api/instances/:id/mods/remote/install` 时应继续传 `mod.name`，这样后端能给并行依赖任务写入不同展示名。
- 验证：`cd backend; go test ./internal/storage ./internal/jobs ./internal/web`、`cd frontend; npm.cmd run build`。

# MODUPLOAD-DUPLICATE-CODE-1 联调契约

- `POST /api/instances/:id/mods/upload` 上传合法 SMAPI ZIP 时，如果实例里已存在相同 `UniqueID` 的 Mod，响应应为 `400 { error: { code: "mod_exists", ... } }`。
- 该错误表示“已安装相同 ID 的 Mod”，不是 ZIP 结构损坏；前端可直接显示已有 `mod_exists` 文案。
- 损坏 ZIP、缺少 manifest、XNB 替换包、manifest 解析失败等仍属于 `invalid_mod_zip`。
- 验证：`cd backend; go test ./internal/web -run "TestModUpload"`。

# FE-OPSRAIL-DOWNLOAD-PROGRESS-1 联调契约

- 扩展普通一键安装在成功创建面板后端任务后，应尽快把 `jobId` 返回给面板页；面板收到新的 batch `jobId` 后会立即刷新 `GET /api/jobs`，右栏“进行中”不应再等 30s 轮询才出现。
- 右栏远程安装进度依赖后端 job 日志：`GET /api/jobs/:jobId/logs` 和 `GET /api/jobs/:jobId/stream` 的 `log` 事件需要包含 `下载进度：已下载 ...（xx.x%）` 这类消息，面板据此显示下载阶段进度。
- 下载百分比只代表 ZIP body 下载阶段；右栏会把它映射到任务整体进度的中前段，下载 100% 后仍会显示校验/安装阶段，任务真正完成以后由 `finished` 事件刷新 jobs 并移除进行中行。
- 联调验收：普通一键安装点击后，扩展返回 `jobId` 时右栏立即出现 `mod_remote_install`；下载日志从 0% 到 100% 时右栏进度同步推进；若扩展没有拿到 ZIP 链接，则不应出现后端 job。
- 验证：`cd frontend; npm.cmd run build`。

# NEXUS-EXT-DOWNLOAD-GUARD-1 联调契约

- 扩展只有在捕获到真实 Nexus CDN ZIP 链接时才允许调用 `POST /api/instances/:id/mods/remote/install`。合法链接需满足 HTTPS、host 为 `supporter-files.nexus-cdn.com` 或其它 `*.nexus-cdn.com`，路径以 `.zip` 结尾。
- 后台页仍停在 Nexus 文件页、Manual Download 页、Slow Download 页、Additional files 弹窗或错误页时，不应创建面板安装任务；前端批量进度只能显示捕获中/超时失败，不能显示 queued/jobId。
- 后端远程安装任务日志现在必须能区分卡点：`正在连接远程下载服务器` 表示已拿到 URL 正在等响应头；`远程下载服务器已响应：HTTP ...` + `远程压缩包大小...` / `下载进度...` 表示已经开始读取 ZIP body。
- 如果后端收到 `text/html`，任务应失败并提示远程下载返回网页而不是 ZIP；联调时优先检查扩展是否真的捕获到 CDN ZIP，而不是只打开了 Nexus 下载页面。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`，以及扩展三个脚本 `node --check`。

# PLAYERSYNC-PACK-2 联调契约

- `POST /api/instances/:id/mods/sync-pack/export` 下载文件名仍是 `stardew-player-sync-pack.zip`，但 ZIP 内容升级为安装包：根目录含 `安装玩家同步包.bat`、`卸载本同步包.bat`、`README.txt`、`pack-manifest.json`、`checksums.sha256`、`tools/` 和 `payload/`。
- 前端无需解析 ZIP；仍按 Blob 下载即可。下游若需要读取包内容，应以 `pack-manifest.json` 为准，普通 Mod 文件在 `payload/mods/<folderName>/`，SMAPI 元数据在 `payload/smapi/smapi.json`。
- `checksums.sha256` 校验 `payload/mods` 和随包 SMAPI ZIP；玩家端脚本会在复制 Mod 前校验。若包内没有 SMAPI ZIP，`pack-manifest.json.smapi.bundled=false`，脚本继续安装 Mod 并提示玩家自行安装 SMAPI。
- 玩家端安装状态落在游戏目录 `.anxi-sync/installed.json`、`.anxi-sync/backups/`、`.anxi-sync/logs/`。卸载脚本按该记录移除本包安装的 Mod，可用 `-RestoreBackup` 恢复备份；不会默认卸载玩家已有 SMAPI。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`go test ./...`。

# NEXUS-SMAPI-THUMB-1 联调约定

- `GET /api/instances/:id/mods` 返回的虚拟 SMAPI 条目也会带 `nexusModId=2400` 和 `nexusUrl`，并在后端 Nexus GraphQL 补全成功后带 `pictureUrl`。
- 前端无需为 SMAPI 写死图片 URL；统一使用 `mods[].pictureUrl`。如果首次请求时 Nexus 不可用或未返回图片，保持现有 `NEXUS` 文字占位即可。
- 该行为不要求用户安装真实 SMAPI Mod 文件夹；它只用于面板展示和玩家同步清单语义。
- 对随 Nexus 包安装的内容包，如果其自己的缓存记录没有 `pictureUrl`，后端会按同一个 Nexus `modId` 合并主 Mod 的完整缓存；前端仍只读 `mods[].pictureUrl`，不需要自己按来源包查找图片。

# MODDEPS-1 联调约定

- `GET /api/instances/:id/mods` 的 `mods[]` 可能包含 `dependencies[]`，每项结构为 `{ "uniqueId": string, "minimumVersion"?: string, "required": boolean }`。
- 字段来源是 SMAPI `manifest.json` 的 `Dependencies` 和 `ContentPackFor`；`ContentPackFor` 统一按必需依赖返回，重复 UniqueID 会去重。
- 前端已安装 Mod 卡片只把 `required=true` 的依赖展示为“需要前置依赖：...”标签；`required=false` 的可选依赖暂不展示。
- 该字段不代表后端已验证依赖是否存在，也不代表安装接口会自动补装依赖。后续缺失依赖检查应基于同一 `dependencies[]` 与已安装 `uniqueId` 列表继续扩展。

# MODUPLOAD-2 联调约定
- `POST /api/instances/:id/mods/upload` 仍是管理员专用，服务器 running/starting 时仍返回 `409 server_running`；请求格式是 `multipart/form-data`。
- 前端可以在同一个请求里重复追加多个 `mod` 文件字段，例如 `form.append('mod', fileA)`、`form.append('mod', fileB)`；后端同时兼容字段名 `mods`，但推荐继续使用重复 `mod` 字段。
- 成功响应仍是 `ModsListResult`：`mods[]` 包含本次批量上传导入出的所有 Mod，`restartRequired` 继续遵循现有 Mod 重启语义；停服上传成功时应为 `false`，下次启动会直接加载新 Mod。前端成功后应刷新 `GET /api/instances/:id/mods` 和仪表盘缓存。
- 任意一个 ZIP 校验/解压/导入失败时，后端会回滚本请求已导入的前序 Mod，并返回错误；前端应把这次上传视为失败，不要假设部分成功。
- 单个 ZIP 内含多个顶层 SMAPI Mod 的能力仍由 `UploadModZip` 提供；“一次选择多个 ZIP”和“一个 ZIP 里多个 Mod”可以同时工作。

# NEXUS-META-1 联调约定
- `GET /api/instances/:id/mods` 可能在返回前触发一次 Nexus GraphQL v2 元数据补全：当本地 Mod manifest 有 `UpdateKeys` 中的 `Nexus:<id>` 且 sidecar 尚无缓存时，后端会无 Key 查询 Nexus 缩略图和展示字段，成功后写入 `.local-container/control/nexus-mods.json`。
- 该补全不改变接口结构，只让 `mods[]` 里的 `pictureUrl/nexusSummary/nexusUrl/downloadCount/endorsementCount/updatedAt` 更完整；Nexus 请求失败时接口仍返回 200 和本地 Mod 信息。
- `GET /api/instances/:id/mods/nexus/search?q=<数字ID>` 未配置 Nexus API Key 时也应返回 GraphQL v2 精确 ID 结果；API Key 只影响 v1 REST 查询和 Nexus 下载安装，不再是展示缩略图/元数据的前置条件。

# MODZIP-1 上传错误约定

- `POST /api/instances/:id/mods` 只接受标准 SMAPI Mod ZIP。若用户上传 Nexus 上的老式 XNB 替换包（例如只包含 `Characters/*.xnb`、`Portraits/*.xnb`，没有 `manifest.json`），后端返回 `400 invalid_mod_zip`，message 会明确提示这是 XNB 替换包，不是 SMAPI Mod，不能上传到服务器 `Mods` 目录。
- SMAPI `manifest.json` 解析兼容 UTF-8 BOM，以及字符串外的 `//` / `/* ... */` 注释和尾随逗号；这只用于 manifest 读取，不代表上传接口接受非 ZIP、非 SMAPI Mod 或 XNB 替换包。

# NEXUS-PAGED-1 联调约定
- `GET /api/instances/:id/mods/nexus/search?q=...&page=...&pageSize=...`：任意登录用户可用，返回 Nexus 专用模型 `{ query, results, page, pageSize, total, hasMore }`。
- 空 `q` 合法，用于默认热门列表；关键词搜索由后端通过 Nexus GraphQL v2 下推 `downloads DESC` 排序和 `offset/count` 分页；纯数字 ID 按 Nexus Mod ID 精确查询。
- `POST /api/instances/:id/mods/nexus/install`：管理员专用，服务器 running/starting 时返回 `409 server_running`；需要 Nexus API Key 和后端可用的文件下载权限，成功返回 `202 { "jobId": "..." }`。
- `/api/instances/:id/mods/search` 与 `/api/instances/:id/mods/search/install` 已撤下，不再作为联调契约。
- 前端创建安装任务后订阅 `GET /api/jobs/:jobId/stream`，完成后拉 `GET /api/jobs/:jobId` 并刷新 `GET /api/instances/:id/mods`。粘贴 URL / 上传文件仍作为兜底入口。

# REMOTE-MOD-1 联调约定
- `POST /api/instances/:id/mods/remote/install`：管理员专用，服务器 running/starting 时返回 `409 server_running`。请求体 `{ "url": string, "mod"?: NexusModSearchResult-like }`，成功返回 `202 { "jobId": "..." }`。
- `url` 为 `nxm://...` 时，后端解析 `modId/fileId/key/expires` 并读取 SQLite 中的 Nexus API Key 调 v1 `download_link.json?key=...&expires=...`；未配置 Key 时任务失败为 `ErrNexusAPIKeyMissing`。
- `url` 为 `https://...zip` 时，后端直接下载该 ZIP，再走现有 `UploadModZip` 校验/解压/导入；该直链来源可以是 Nexus CDN、ModDrop、GitHub、CurseForge 等公网 HTTPS ZIP。当前不支持 7z/rar。
- 前端创建任务后订阅 `GET /api/jobs/:jobId/stream`，与 `mod_nexus_install` 相同。任务成功后刷新 `GET /api/instances/:id/mods`。
- 为防止临时授权泄漏，前端和审计日志不保存粘贴 URL；失败信息不应包含完整 NXM/CDN URL。

# NEXUS-EXT-1 联调约定
- `browser-extensions/nexus-slow-installer` 是独立浏览器扩展实验包，用于免费 Nexus 用户的慢速下载链路：本地浏览器登录 Nexus -> 扩展在文件页点击/等待 `Slow download` -> 捕获浏览器下载任务中的 Nexus CDN `.zip` 临时链接 -> 调用面板 `POST /api/instances/:id/mods/remote/install`。
- 扩展第一版不新增后端接口，请求体仍是 `{ "url": string, "mod"?: { "modId": number, "name"?: string, "nexusUrl"?: string } }`；后端按 REMOTE-MOD-1 的 `.zip` 直链规则下载并安装。
- 扩展调用面板接口时使用 `credentials: "include"` 复用同浏览器中的面板管理员登录态。若正式云端部署遇到 SameSite/Cookie 或跨域策略导致无法带登录态，应新增受限的扩展配对 token，而不是让扩展保存管理员密码。
- 联调前置：面板管理员已登录、服务器已停止、Nexus 账号已登录、Nexus CDN 临时链接可由云端后端访问。测试失败时优先区分三类问题：扩展未捕获下载、面板鉴权 401/403、后端下载/导入 ZIP 失败。
- 扩展状态与日志必须脱敏 `md5/expires/user_id/key`；完整临时 URL 只作为请求体短暂发送给面板，不应落入长期文档、审计或支持包。

# NEXUS-EXT-3 联调约定

- 前端 Nexus 搜索结果的“一键安装”主路径已经切到扩展链路：点击后同页跳转到 Nexus Mod 文件页，并附加 `anxi_auto=1`。前端不再为该按钮直接调用 `POST /api/instances/:id/mods/nexus/install`，也不再要求用户配置 Nexus API Key。
- 扩展进入带 `anxi_auto=1` 的 Nexus 页面后自动打开手动下载/慢速下载流程，捕获到 Nexus CDN `.zip` 临时链接后只等待用户点击“提交到面板”。提交时复用 `POST /api/instances/:id/mods/remote/install`，成功响应仍为 `202 { "jobId": "..." }`。
- 扩展提交成功后跳回 `/instances/:id/jobs?jobId=<jobId>`；`JobsLogsPage` 应优先读取 `jobId` 查询参数并加载该任务详情和日志。若任务不在第一页列表里，右侧详情仍应通过 `GET /api/jobs/:jobId` 加载。
- 旧 `POST /mods/nexus/install` 可以保留给后续 Premium/API Key 直连或调试使用，但当前用户入口以扩展 + remote install 为准。

# NEXUS-3 联调约定

- `GET /api/instances/:id/mods/nexus/search?q=...`：无 Nexus API Key 时也应能走 GraphQL v2 搜索；纯数字 query 未配置 Key 时按 GraphQL v2 的 `gameId=1303 + modId` 精确查询，已配置 Key 时仍可按 v1 REST 精确 ID 查询。
- `POST /api/instances/:id/mods/nexus/install`：管理员专用，请求体为当前 Nexus 搜索卡片字段（至少 `modId`，建议带 `name/summary/version/pictureUrl/nexusUrl/downloadCount/endorsementCount`）。未配置 Key 返回 `503 nexus_api_key_missing`；服务器运行中返回 `409 server_running`；成功返回 `202 { jobId }`。
- 前端安装后订阅 `GET /api/jobs/:jobId/stream`，展示 `log` 事件，`finished` 后拉取 `GET /api/jobs/:jobId` 判断 succeeded/failed，并刷新 `GET /api/instances/:id/mods`。
- `GET /api/instances/:id/mods` 的 `mods[]` 现在可能包含 Nexus 卡片字段：`nexusSummary`、`pictureUrl`、`nexusUrl`、`downloadCount`、`endorsementCount`、`updatedAt`。前端可用这些字段把已安装 Mod 渲染成与搜索结果一致的卡片。
- 安装流程不新增前端直连 Nexus；所有 Nexus 文件列表、下载链接、下载 ZIP、解压安装都由后端代理和现有 Mod ZIP 安全校验完成。

# 前后端联调文档

## 联调目标

前后端联调必须验证三件事：

1. API 结构和错误码稳定。
2. 长任务、SSE、Steam Guard、邀请码刷新等异步流程可恢复。
3. UI 状态和后端实例状态一致，不误导用户。

## 本地启动

后端：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go run .\cmd\panel
```

前端：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run dev
```

访问 Vite 地址，前端开发代理应能访问后端 API。完整打包验证见 `docs/09-image-build.md`。

## 关键联调流程

### 1. 初始化与登录

- 首次访问显示管理员初始化。
- 创建管理员后自动登录。
- 登出后回到登录页。
- 错误密码显示中文错误提示。
- 未登录访问 API 返回 401，权限不足返回 403。

### 2. 安装与 Steam Auth

- `prepare` 创建实例目录和 compose/env。
- `install` 创建 job。
- 前端订阅 job SSE。
- Steam Guard / QR / 验证码阶段前端能提交输入。
- job 结束后状态刷新。
- 日志不包含 Steam/VNC 密码。

### 3. 生命周期与邀请码

- 未安装时启动返回结构化错误，前端提示“请先安装游戏”。
- 有可用存档后可启动。
- 同一实例的启动、停止、重启任务必须互斥；用户点击停止后，旧启动任务应变为 `canceled`，不能继续显示 running。
- 启动/重启提交成功后前端清空旧邀请码并等待新码。
- 后端过滤容器内旧 `/tmp/invite-code.txt`。
- 停止后前端清空邀请码。

### 4. 存档

- 上传 ZIP 先预览，不直接写正式目录。
- 用户确认后再提交导入并启动。
- 新建存档必须生成 Stardew/Junimo 可读真实存档，不能只写表单摘要。
- 删除前自动备份。
- 运行中禁止删除或覆盖危险操作。

### 5. Mod

- 上传、删除、导出可用。
- 运行中危险操作禁用。
- 上传/删除/安装 Mod 写操作要求服务器停止；停服修改后下次启动会自动加载，不再提示需要重启。`restartRequired` 只用于运行中已有 Mod 变更待应用的历史/兼容场景，服务器停止时接口应返回 `false`。
- 玩家同步分类（`syncKind`：`server_only`/`client_required`/`unknown`）随 `GET /api/instances/:id/mods` 一起返回，前端不用单独再拉一次。没有手动覆盖时，后端会自动把面板控制组件标为 `server_only`，把 SMAPI 内容包和其他第三方 Mod 标为 `client_required`，并在 `syncNote` 写入自动识别说明。
- `GET /api/instances/:id/mods/sync-plan` 返回分类统计；`PUT /api/instances/:id/mods/:modId/sync-classification` 任意登录用户可用，编辑不受运行状态限制；`POST /api/instances/:id/mods/sync-pack/export` 任何登录用户可用，运行中也允许导出，导出包含 `pack-manifest.json`、`checksums.sha256`、安装脚本和 `payload/mods/`，且永远不含面板自带的 `StardewAnxiPanel.Control`。
- 无 `client_required` Mod 时导出接口返回 `400 no_sync_mods`，前端按钮直接禁用避免命中。
- `GET /api/instances/:id/mods/nexus/search?q=关键词`：任意登录用户可用（不需要管理员权限），后端代理 Nexus Mods 官方 API，前端不直连 N站。鉴权按能力拆开：关键词搜索和无 Key 纯数字 ID 展示查询都走公开只读的 GraphQL v2，**不需要个人 API Key**；配置 Key 后纯数字 ID 可优先走 v1 REST 精确查询。只有当 Nexus 自己因鉴权拒绝 GraphQL 查询时才返回 `502 nexus_auth_required`（提示需要 OAuth/更高权限，配置 Key 不一定能解决）。空关键词作为默认热门列表返回 200；其余上游非 2xx 映射为 `404 nexus_mod_not_found` / `502 nexus_unauthorized`（v1 REST Key 无效/权限不足）/ `429 nexus_rate_limited` / `502 nexus_request_failed`。后端 message 必须保持正常 UTF-8 中文，前端也会按这些 Nexus 错误码兜底显示稳定中文。返回结果按本地已装 Mod 的 manifest `UpdateKeys`（`Nexus:<id>`）匹配 `installed`，本阶段不做版本新旧判断。
- `GET /api/settings/nexus` / `PUT /api/settings/nexus/api-key` / `DELETE /api/settings/nexus/api-key`：管理员专用的 Nexus Key 配置接口。PUT 请求体 `{ "apiKey": string }`，保存后当前进程立即生效；GET 只返回 `{ configured, last4? }`，不会回显完整 Key；DELETE 清除配置。

### 6. 控制台命令

- 普通用户只看到允许的只读命令。
- 管理员看到完整 allowlist。
- 不允许任意 shell。
- 服务器未运行时命令禁用或返回 `server_not_running`。
- `POST /api/instances/:id/commands/say` 请求体为 `{ "message": string }`；成功返回 `CommandRunResult{ command: "say", output, exitCode: 0, durationMs }`，表示后端已把喊话命令写入控制目录。
- 喊话由 `StardewAnxiPanel.Control` 消费 `.local-container/control/commands/*.json` 后发送到游戏聊天，实际玩家可见文本前缀为 `[Panel]`。如果服务器已运行但世界尚未 ready，控制模组会在 `status.json` 记录忽略原因，后端不会暴露任意 SMAPI 命令入口。
- 前端仍应在非 running 状态禁用喊话输入；运行中发送失败时按结构化错误码展示，成功时提示“已提交/已发送”即可，不需要等待聊天回执。

### 7. 玩家页

- `GET /api/instances/:id/players` 返回在线快照和缓存名册。
- 玩家名册会合并当前存档主 XML 中的 `<player>` 与 `<farmhands><Farmer>`；存档存在但当前不在线、也没进入缓存的玩家应显示为 `status=offline`、`source=save_file`，例如 `saveId=test` 可匹配 `Saves/test_数字` 下的 farmhand。
- `maxPlayers` 默认取当前存档 `server-settings.json` 的 `Server.MaxPlayers`（junimo info 解析出的值优先）；服务器未运行时也会返回，供前端显示"在线数/人数上限"。
- 前端显示 online/offline、host、位置、tile/pixel。
- 未知地图 key 保留原值。
- 玩家页固定展示 `money`、`farmIncome`、`personalIncome` 和 `walletMode`；`farmIncome` 是农场/团队累计收入，`personalIncome` 是玩家个人累计收入，不随钱包模式改变含义。
- `recentEvents` 返回最近玩家活动，至少覆盖首次记录、加入和离开；事件必须按 `saveId` 隔离。
- 新建/切换存档后，玩家缓存必须按 `saveId` 隔离；上一存档玩家不应出现在当前存档列表。

## API 约定

错误响应应保持结构化：

```json
{
  "error": {
    "code": "server_not_running",
    "message": "服务器未运行"
  }
}
```

前端优先使用 `code` 映射中文提示，`message` 作为兜底。

## 状态校准

面板状态机是 UI 流程来源，但 Docker 和 Junimo 是运行事实来源。后端启动和关键操作前应校准：

```text
docker compose ps
docker compose logs --tail
Junimo HTTP status（如启用）
SMAPI / control files
.env、docker-compose.yml、.local-container
active save metadata
```

典型规则：

- 面板记录 running，但 compose 显示 server 停止，应回到 stopped 或 error。
- install 完成但未选择存档，应保持 `save_required`。
- start 无有效存档，应返回 `save_required` 并引导前端到 Saves。

## 常用验证命令

后端：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
```

前端：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

冒烟：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-test.ps1
```

可选参数：

- `-SkipDocker`
- `-SkipFrontend`
- `-SkipBackend`
# SMAPI-RUNTIME-1 联调约定

- `GET /api/instances/:id/mods` 可能返回 `mods[0].builtIn=true` 的 `SMAPI` 虚拟条目；前端应把它作为置顶内置组件展示。
- `builtIn=true` 条目不是服务器 Mods 目录中的真实文件夹，不能调用删除接口，也不能调用同步分类更新接口；后端同步包导出会忽略该条目。
- 该条目的 `syncKind=client_required` 表达“玩家客户端需要先安装 SMAPI”，但它不会进入玩家同步 ZIP；前端同步统计应排除 `builtIn`，普通玩家看到的是提示而不是可下载内容。
- SMAPI 条目只有在面板内置控制 Mod `StardewAnxiPanel.Control` 已出现在实例 Mods 目录时才注入，用来避免未准备实例显示运行组件。

# MODORIGIN-1 Nexus 包来源字段

- `GET /api/instances/:id/mods` 的 `mods[]` 可能同时包含 `nexusModId` 和 `originNexusModId` 两类字段。`nexusModId` 只表示该 Mod 文件夹自己的 SMAPI `UpdateKeys` 声明；`originNexusModId` 表示它随某个 Nexus 下载包一起安装。
- 前端展示规则：`nexusModId>0` 显示为主 Nexus Mod；`nexusModId` 为空且 `originSource="nexus"` 时显示为“来源：N站包 / 随 <originModName> 安装”。不要把 `originNexusModId` 当作该内容包自己的 `nexusModId`，否则 `[CP]` 内容包会被误认为独立 N站 Mod。
- 后端会把来源包的 `pictureUrl/downloadCount/endorsementCount/updatedAt` 填到内容包卡片上，用于展示缩略图和统计；同步分类、玩家同步包导出仍按真实 Mod 文件夹处理。
- 删除是例外：`DELETE /api/instances/:id/mods/:modId` 会按来源包 bundle 删除同组真实 Mod 文件夹。前端删除确认应根据当前 `mods[]` 计算同 `nexusModId/originNexusModId` 的组成员并提示用户会一起删除；确认后只调用一次 DELETE，不要在前端循环多次删除。
# NEXUS-PAGED-1 联调契约

- 模组下载页在线搜索只调用 `GET /api/instances/:id/mods/nexus/search?q=...&page=...&pageSize=...`。
- 响应结构为 `NexusModSearchResponse{query, results, page, pageSize, total, hasMore}`；前端用 `hasMore` 控制下一页，用 `page > 1` 控制上一页。
- 关键词搜索在后端通过 Nexus GraphQL v2 下推 `downloads DESC` 排序和 `offset/count` 分页；前端不再调用 `/mods/search` 统一搜索骨架。
- Nexus 一键安装继续调用 `POST /api/instances/:id/mods/nexus/install`；管理员粘贴 Nexus `nxm://` 或 Nexus CDN 临时 ZIP 仍走 `POST /api/instances/:id/mods/remote/install`。
- `/api/instances/:id/mods/search` 与 `/api/instances/:id/mods/search/install` 已撤下，不再作为联调契约。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。

# SMAPI-SYNC-2 联调契约

- `GET /api/instances/:id/mods` 可能同时返回两个 `builtIn=true` 条目：虚拟 `SMAPI` 与真实 `StardewAnxiPanel.Control`。前端不要仅凭 `builtIn` 判断是否排除玩家同步；SMAPI 需要进入同步统计和同步清单，Control 不进入。
- `SMAPI`：`uniqueId=Pathoschild.SMAPI`、`syncKind=client_required`、`builtIn=true`。它代表玩家客户端前置要求，导出同步包时进入 `pack-manifest.json` 的 `mods[]` 和 `smapi` 元数据；只有服务端已缓存 SMAPI ZIP 时才会写入 `payload/smapi/SMAPI*.zip`。
- `StardewAnxiPanel.Control`：`folderName=StardewAnxiPanel.Control`、`builtIn=true`、`syncKind=server_only`。前端不得显示删除按钮或同步分类下拉；后端也会拒绝删除并排除同步包。
- `pack-manifest.json` 条目包含 `builtIn` 与 `packaged`：下游如果做玩家同步安装器，应只自动复制 `packaged=true` 的 Mod；`packaged=false` 的 SMAPI 是玩家前置要求，不是 Mod 文件夹。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 与 `npm.cmd run build`。
# PLAYERSYNC-PACK-15 联调契约

- 完整同步包接口保持不变：`POST /api/instances/:id/mods/sync-pack/export`，下载文件名 `stardew-player-sync-pack.zip`，用于首次加入玩家，可按服务端缓存情况携带 `payload/smapi/SMAPI*.zip`。
- 新增模组更新包接口：`POST /api/instances/:id/mods/sync-pack/export-update`，下载文件名 `stardew-player-mods-update-pack.zip`，用于已经运行过完整版同步包的玩家。
- 更新包 ZIP 内 `pack-manifest.json.packType=mods_update`，不包含 `payload/smapi/`，`checksums.sha256` 只校验 `payload/mods/`。安装脚本会要求玩家游戏目录已存在 `StardewModdingAPI.exe`，否则提示先运行完整版同步包。
- 前端只按 Blob 下载，不需要解析 ZIP；UI 上应把完整包和更新包区分展示。更新包没有真实可打包 Mod 时后端返回 `400 no_sync_mods`，前端按钮也应在只有虚拟 SMAPI 时禁用。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。

# PLAYERSYNC-PACK-16 联调契约

- 模组更新包 `stardew-player-mods-update-pack.zip` 的 `tools/install.ps1` 不再读取或修改 Steam 启动项，ZIP 内也不包含 `tools/steam-launch-options.ps1`。
- 更新包仍要求玩家游戏目录已有 `StardewModdingAPI.exe`；缺失时提示先运行完整版玩家同步包。
- 更新包安装完成摘要显示 `Steam 启动项：已跳过，沿用已有设置`，不会再输出 `Steam 启动项文本` 或复制提示。
- 完整同步包契约不变：首次玩家包仍会尽力自动配置 Steam 启动项，失败时输出可复制 launch options。
- 前端无需新增字段；继续区分“完整同步包”和“模组更新包”的下载按钮即可。

# MODPROFILE-1 联调契约

- `GET /api/instances/:id/mods` 返回的 `mods[]` 新增 `enabled/canToggle/enableNote`，用于展示当前激活存档下的 Mod 启用状态。
- 禁用的 Mod 仍会出现在 `GET /mods` 响应中；前端必须读取 `enabled`，不要用是否出现在列表里判断启用。
- 新增 `PUT /api/instances/:id/mods/:modId/enabled`。管理员专用、服务器 running/starting 时不可用；请求体 `{ "enabled": true|false, "saveName"?: string }`，不传 `saveName` 时使用当前激活存档。
- 新建存档和新导入存档默认只启用内置组件，第三方 Mod 需要在配置页手动开启。旧存档没有 profile 时保持当前物理目录状态。
- 启动前后端会按当前存档 profile 移动 Mod 目录，因此玩家同步包导出仍只打包当前启用目录里的玩家 Mod。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。
# MODPROFILE-2 联调契约

- `POST /api/instances/:id/saves/select` 和 `POST /api/instances/:id/saves/select-and-start` 切换存档后，后端会应用对应存档的 Mod profile；前端在收到切换成功并刷新 saves 后必须刷新 `GET /api/instances/:id/mods`，用新的 `enabled/canToggle/enableNote` 渲染当前存档状态。
- 公共数据层现在监听 `activeSaveName`，活动存档变化会触发 mods 刷新；页面不要缓存旧 `mods` 当作跨存档状态。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。

# NEXUS-DEFAULT-1 联调契约

- `GET /api/instances/:id/mods/nexus/search?q=&page=1&pageSize=20` 合法，返回 Nexus Stardew Valley 默认热门列表，不再返回 `invalid_query`。
- 空 `q` 响应结构仍是 `{ query, results, page, pageSize, total, hasMore }`，其中 `query` 为 `""`；前端用同一套 Nexus 结果卡片和分页控件展示。
- 关键词、数字 ID、安装接口契约不变。只有下载页默认态和空输入刷新热门依赖这个新行为。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。
# NEWGAME-CABINS-1 自定义新存档小屋联调契约

- `POST /api/instances/:id/saves/custom-new-game` 的 `startingCabins` 表示“初始联机小屋数量”，不是总玩家数；合法范围为 0-7。
- 前端新建存档 UI 必须直接显示并提交 `startingCabins`，不要把主玩家加 1 后当作“小屋数”展示。
- 后端会同时写 Junimo `server-settings.json` 的 `Game.StartingCabins`、控制模组 `server-init.json` 的 `cabinCount/cabinLayout`，以及 `.local-container/control/new-game-pending` 一次性标记；控制模组只在该标记存在时提前同步 Stardew 原生新建参数。
- 控制模组不再作为存档创建方；联调时应以 Junimo HTTP `POST /newgame` 和 Junimo 生成的存档目录为准，Control 只负责新建前参数同步和新建后角色定制。
- 联调验证建议：创建 0、1、2 间小屋的新存档后，分别检查 `.local-container/settings/server-settings.json`、`.local-container/control/server-init.json`、`.local-container/control/new-game-pending`，并解析生成存档 XML 中可见/有效 Cabin building 数；已有存档不会因 `server-init.json` 残留而重新应用新建参数。

# FE-QUICK-BACKUP-1 快捷备份联调契约

- 服务器控制页“备份存档”复用现有 `POST /api/instances/:id/saves/:name/backup`，其中 `name` 必须来自 `GET /api/instances/:id/saves` 的 `activeSaveName`。
- 前端仅管理员可点；无激活存档时禁用。成功响应仍为 `{ "backupName": string }`，前端只展示文件名并不解析 ZIP。
- 该操作在 UI 中显示为“备份已保存进度”，语义是“对当前磁盘存档目录打 ZIP 手动备份”，不是“强制 Stardew 立即保存当前游戏内世界”。游戏内未触发保存事件的进度不会因为备份按钮自动写入主存档。
- 手动验证：运行中和停服状态分别点击快捷备份，确认能创建手动备份；随后进入存档页备份列表应能看到同一备份文件。


# SAVE-BACKUP-POLICY-1 ????

- ??????????SMAPI Control `Saved` ?? -> ? `.local-container/control/save-events/*.json` -> ?? `GET /saves/backups` ???????? -> ?? latest ????/???? daily ???
- ??????????????? -> `PUT /api/instances/:id/saves/backups/policy` -> ??? `.local-container/backups/saves/policy.json`?????????????????
- ???????????/?????????????????????? scheduler????????????????????????????
- ????????????????????????/??????????????????????????

# SAVE-BACKUP-SCHEDULE-HOUR-1 联调契约

- `GET|PUT /api/instances/:id/saves/backups/policy` 的定时备份字段使用 `scheduledHour`，取值 0-23，前端按 24 小时制展示为 `HH:00`。
- 后端仍能读取旧 `scheduledIntervalHours` 配置文件，但新响应和新保存结果以 `scheduledHour` 为准，不再要求前端提供时间间隔。
- 定时备份语义：每天到达配置整点后，下一次触发备份维护时覆盖同一份 `scheduled_<save>.zip`；同一自然日不会重复生成。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。
# SCHEDULED-RESTART-1 计划重启联调契约

- `GET /api/instances/:id/restart-schedule` 返回 `{ schedule }`，字段包括 `enabled`、`shutdownTime`、`startupTime`、`timezone`、`warningMinutes`、`backupBeforeShutdown`、`skipIfPlayersOnline`、`nextShutdownAt`、`nextStartupAt`、`lastStatus`、`lastMessage`。
- `PUT /api/instances/:id/restart-schedule` 仅管理员可用，请求体使用同一组配置字段；时间格式为 `HH:MM`，时区默认 `Asia/Shanghai`。
- 后端后台调度器每 30 秒检查启用计划。关闭时间前通过现有喊话通道写 `.local-container/control/commands/*.json`；关闭时可调用现有存档备份能力，再提交 `Stop` 生命周期 job；开启时提交 `Start` 生命周期 job。
- 关闭前备份语义与快捷备份一致：只备份当前已经落盘的 active save，不强制保存游戏内尚未写盘的进度。
- 联调建议：把关闭时间设置到当前时间后 1-2 分钟，确认弹窗保存后返回 `nextShutdownAt`；服务器运行中确认提醒文件写入 control commands；到点后任务中心出现 stop job。开启时间设置到停止后 1-2 分钟，确认 start job 被提交。
# MODDEPS-2 联调契约

- `GET /api/instances/:id/mods` 的 `mods[].dependencies[]` 结构扩展为 `{ uniqueId, minimumVersion?, required, installed, enabled, installedVersion?, satisfied, status? }`。前端应以 `satisfied=false` 或 `status` 判断展示诊断，不要再只把它当作纯 manifest 声明。
- 常见 `status`：`satisfied`、`missing`、`disabled`、`version_mismatch`、`unknown_version`；可选依赖对应 `optional_missing`、`optional_disabled`、`optional_version_mismatch`、`optional_unknown_version`。可选依赖缺失默认不算硬失败。
- `GET /api/instances/:id/mods/nexus/search` 的 `results[]` 新增 `installedEnabled`。当 `installed=true` 且 `installedEnabled=false` 时，表示该 Nexus Mod 已在服务器安装，但当前激活存档没有启用；前端应提示“已安装但未启用”，并禁止重复安装。
- 搜索的安装匹配按当前激活存档计算。后端会读取 `GetActiveSaveName(dataDir)` 并用 `ListModsWithState` 合并 active/disabled 目录，确保禁用目录里的 Mod 仍能被 Nexus ID 匹配到。
- 验证：`cd backend; go test ./...`、`cd frontend; npm.cmd run build`。

# MODREL-1 联调契约

- `PUT /api/instances/:id/mods/:modId/sync-classification` 响应从单个 `{ folderName, syncKind, syncNote }` 升级为 `{ mods, syncKind }`。`mods[]` 是本次按依赖/同包关系被同步分类影响的 Mod，前端必须按返回列表批量更新。
- 同步分类没有方向性：设置 `client_required`、`server_only` 或 `unknown` 时，都包含同 Nexus 包成员、所有已安装必需前置依赖、前置的前置，以及依赖它的已安装下游。这样用户先点“待确认”再切回其它标签时，后置 Mod 不会停留在旧状态。
- `PUT /api/instances/:id/mods/:modId/enabled` 响应从单个 `{ folderName, enabled, saveName }` 升级为 `{ mods, enabled, saveName }`。启用时包含同包成员和必需前置，禁用时包含同包成员和依赖它的下游。
- 共享前置不随某个业务包禁用：例如启用 `[CP] Multiple Construction Orders` 会启用 `Multiple Construction Orders` 和 `Content Patcher`；禁用 `Multiple Construction Orders` 会禁用同包 `[CP]`，但不会禁用 `Content Patcher`，因为它可能仍被其他 Mod 使用。
- 前端不要自行复刻关系图算法；以后联动规则调整时以后端返回的 `mods[]` 为准。
- 验证：`cd backend; go test ./...`、`cd frontend; npm.cmd run build`。
# NEXUS-EXT-2 安装完成可见性与日志

- `mod_remote_install` / `mod_nexus_install` 新任务的安装进度日志应显示正常中文；旧任务历史日志如果已经以乱码入库，不做迁移。
- 前端订阅安装 job 的 `finished` 事件后，成功状态会切到“添加模组”页并刷新 `GET /mods`。后端会把本次导入的 Mod 标记为当前激活存档启用；联调时如果任务已完成但页面没看到，应先确认是否刷新到了添加页，以及 `mods/` / `mods-disabled/` 目录和当前存档 profile。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。
# NEXUS-REQ-1 联调约定

- `GET /api/instances/:id/mods/nexus/search` 的 `results[]` 可能包含 `requiredMods[]`。字段来自 Nexus GraphQL 的 `modRequirements.nexusRequirements`，用于前端搜索卡片提示缺少的 Nexus 前置 Mod。
- `requiredMods[]` 每项包含 `modId/name/notes/nexusUrl/installed/installedEnabled/installedFolderName/installedVersion`。前端可把 `installed=false` 的项渲染为“安装前置”，并跳转该前置 Mod 的 Nexus 文件页。
- 前端主 Mod 与前置 Mod 的扩展安装入口都统一追加 `tab=files&anxi_auto=1`；扩展捕获 ZIP 后仍调用 `POST /api/instances/:id/mods/remote/install`。
- Nexus 页面出现 “Additional files required” 弹窗时，扩展应自动点击弹窗内 `Download` 按钮继续，不要求用户手点。该动作只发生在扩展已开始捕获的上下文里。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`、`cd frontend; npm.cmd run build`、扩展脚本 `node --check`。
# NEXUS-PREMIUM-2 联调约定

- 前端页面不再暴露“粘贴链接安装”按钮；`POST /api/instances/:id/mods/remote/install` 仍作为浏览器扩展提交 Nexus CDN 临时 ZIP 的接口保留。
- `GET /api/settings/nexus` 返回 `configured=false` 时，前端仅提示 Premium 用户配置 NexusKey，不展示会员安装按钮。
- `configured=true` 时，Nexus 搜索结果卡片显示 `N站会员专属安装`，点击后调用 `POST /api/instances/:id/mods/nexus/install`，请求体仍使用当前 `NexusModSearchResult` 字段映射。
- 普通 `一键安装` 不调用后端安装接口，继续跳转 Nexus 文件页并交给扩展获取 ZIP 链接。

# NEXUS-EXT-BATCH-1 联调约定

- 普通 Nexus `一键安装` 现在由面板页向浏览器扩展发送 `START_BATCH_INSTALL` 消息，不直接跳转 Nexus，也不直接调用后端安装接口。
- 扩展后台为每个目标打开非激活标签页，URL 会附加 `anxi_auto=1&anxi_auto_submit=1&anxi_batch=<batchId>&anxi_item=<itemId>`；Nexus content script 会把这些参数短期写入 `sessionStorage`，后续 Nexus 跳转即使丢失查询参数，捕获 ZIP 后仍自动提交到现有 `POST /api/instances/:id/mods/remote/install`。批量任务优先让面板页 `panel-bridge.js` 代表扩展发起同源请求，复用面板登录态与 Vite proxy；桥接不可达时才回退扩展 background 直连面板地址。
- 面板按钮百分比只表示“扩展 ZIP 提交流程”的完成度，不表示后端解压导入 job 的完成度。后端真实安装结果仍以 `mod_remote_install` 任务日志为准。
- 进度折算：扩展阶段单项 `opening=10`、`capturing=35`、`ready=65`、`posting=80`、`queued=90`；批量进度取所有目标平均值。`queued` 只表示面板已创建 job，前端拿到 `items[].jobId` 后继续轮询 `GET /api/jobs/:id`，所有 job `succeeded` 才显示 100%，任一 job `failed/canceled` 时整体失败并显示对应 Mod 名。没有任何 jobId 时才适用扩展提交超时。
- 兜底：若 item 没有 `jobId`，但刷新 `GET /api/instances/:id/mods` 后可通过 `nexusModId` 或 `originNexusModId` 匹配到该 Nexus modId，前端可视为该 item 已完成。
- `CAPTURE_URL` / `SUBMIT_CAPTURED_URL` 消息必须携带 `batchId/itemId/autoSubmit` 或可解析的 `captureKey=batch:item`。background 会用这些字段恢复 capture 的 batch 上下文，并在 `POST /mods/remote/install` 返回后把 `jobId` 写入对应 item。
- 面板可发送 `CLEAR_STATE` 给扩展桥接，用于清理扩展 batch/capture 和前端卡住的 session 状态；该操作只清浏览器进度，不删除已经安装到服务器的 Mod。
- 浏览器扩展新增 `panel-bridge.js`，只在当前页面 origin 等于扩展配置的 `panelBaseUrl` origin 时响应面板消息；正式部署时仍需保证同浏览器已登录面板管理员和 Nexus。
# NEXUS-EXT-BATCH-2 联调契约

- 扩展 batch 的 `done/failed` 在前端视为终态；后续 `GET_BATCH_STATUS` 只能用于补充仍在进行中的 batch，不得覆盖已经完成或失败的按钮状态。
- 搜索页安装进度的最终成功仍以面板 job 为准：所有关联 `mod_remote_install` job 均为 `succeeded` 才显示 100% 完成；任一 job `failed/canceled` 显示失败。没有 `jobId` 的异常情况才允许用 `GET /mods` 的 `nexusModId/originNexusModId` 命中做兜底。
- Nexus 多 Mod ZIP 的来源以 `stardew_junimo.SaveInstalledNexusMetadata` 写入的 sidecar 为准；如果扩展 batch 上下文和 ZIP 内唯一正数 `UpdateKeys: ["Nexus:<id>"]` 冲突，后端会用 ZIP 内声明纠偏。
- Ridgeside Village 验证点：`RidgesideVillage`、`[CC] Ridgeside Village`、`[FTM] Ridgeside Village` 应显示随 `Ridgeside Village [Content Patcher component] / Nexus:7286` 安装，不应显示随 `SpaceCore / Nexus:1348` 安装。
# NEXUS-EXT-BATCH-3 联调约定

- 浏览器扩展批量安装入口 `START_BATCH_INSTALL` 现在是幂等的：同一个 `batchId` 重复发送只返回已有 batch，不再重复打开 Nexus 后台页。
- 扩展会按 Nexus `modId` 对目标去重；缺少 `modId` 时按移除 `anxi_auto/anxi_auto_submit/anxi_batch/anxi_item` 后的 URL 去重。同一 Mod 同时作为前置和本体出现时，本体目标优先。
- 验证 Ridgeside Village 这类“本体 + 多前置”时，预期每个 Nexus modId 只打开一个后台下载页；如果仍看到重复页，先检查面板传入 targets 是否缺失 modId 或 URL 是否指向不同 Nexus Mod。
# NEXUS-EXT-CONNECT-1 联调约定

- 面板下载页通过 `window.postMessage` 向浏览器扩展桥接脚本发送 `PING`，payload 至少包含 `{ panelBaseUrl: window.location.origin, instanceId: "stardew" }`。
- `panel-bridge.js` 收到 `PING` 时允许绕过旧的 `panelBaseUrl` origin 校验，但会先调用当前面板页的 `GET /api/auth/me` 并要求返回已登录用户；验证成功后再向 background 发送 `REGISTER_PANEL`，由 background 保存 `panelBaseUrl` 和 `instanceId`。
- `PING` 成功返回 `{ ok: true, config, state }` 后，前端显示“扩展已连通”，普通 Nexus “一键安装”开放。失败时按钮保持可重试，普通一键安装禁用；Premium Key 直连安装不依赖扩展连通。
- 除 `PING` 外，`START_BATCH_INSTALL`、`GET_BATCH_STATUS`、`CLEAR_STATE` 和后台页提交仍要求当前页面 origin 与扩展配置的 `panelBaseUrl` 匹配，避免普通网页改写扩展安装目标。
- 联调时浏览器扩展更新后需要在扩展管理页重新加载，并刷新面板页，让新的 `panel-bridge.js` 注入当前标签页。
# NEXUS-EXT-PACK-1 联调契约

- 新增扩展包下载接口：`GET /api/instances/:id/mods/nexus/extension/download`。
- 请求要求已登录面板；响应为 `application/zip`，`Content-Disposition` 文件名固定为 `anxi-nexus-installer.zip`。
- 后端优先返回实例目录 `.local-container/browser-extensions/anxi-nexus-installer.zip` 中已有且合法的预打包 ZIP；不存在时优先复制镜像/仓库中的 `browser-extensions/anxi-nexus-installer.zip`，预包不存在或损坏时才兜底生成。合法性至少要求 ZIP 根目录包含 `manifest.json` 和 `background.js`。
- 前端 `下载浏览器扩展` 按钮只负责下载扩展包；安装后仍需用户在 Chrome/Edge 扩展管理页加载解压目录，再回面板点击 `检测扩展` 完成地址同步与连通校验。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`；`cd frontend; npm.cmd run build`。
# NEWGAME-PLAYERLIMIT-1 自定义新存档人数上限联调契约

- `POST /api/instances/:id/saves/custom-new-game` 新增可选字段 `maxPlayers`，表示最大同时在线人数，合法范围 `1-100`；旧客户端不传时后端默认写入 `10`。
- `startingCabins` 仍表示初始联机小屋数量，范围 `0-7`；`maxPlayers` 是总在线人数上限，必须大于等于 `startingCabins + 1`。
- 后端会写 `server-settings.json`：`Server.MaxPlayers=<maxPlayers>`、`Server.CabinStrategy="CabinStack"`、`Server.ExistingCabinBehavior="KeepExisting"`，以及原有 `Game.StartingCabins` / `Game.CabinLayoutNearby`。
- 联调验证建议：新建存档时分别提交 `startingCabins=7,maxPlayers=8` 与 `startingCabins=7,maxPlayers=16`，确认配置文件写入正确；提交 `startingCabins=7,maxPlayers=7` 应返回结构化错误。
# VNC-CONTROL-1 联调契约

- `GET /api/instances/:id/rendering`：管理员专用，服务器必须处于 `running`。用于刷新页面后读取 Junimo 当前服务端渲染状态，成功返回 `{ "fps": 0|N, "output"?: string }`。
- `POST /api/instances/:id/rendering`：管理员专用，服务器必须处于 `running`。请求体 `{ "fps": 15 }` 用于打开 Junimo 服务端渲染，`{ "fps": 0 }` 用于关闭，成功返回 `{ "fps": number, "output"?: string }`。
- 该接口由面板后端在 `server` 容器内代理 JunimoServer `POST /rendering?fps=...`，并按实例 `.env` 注入 `API_KEY`；代理请求会显式带 `Content-Length: 0` 以满足 Junimo 空 POST 要求。前端不得直连 Junimo `API_PORT`，也不得读取 API key。
- 服务器页 `跳转VNC控制` 通过已有 `GET /api/instances/:id/config/vnc-port` 读取宿主 VNC/noVNC 端口，并打开 `http://<当前面板hostname>:<vncPort>/`。VNC 密码只在 noVNC 页面中输入，不在面板前端回显。
- 联调顺序建议：启动服务器 -> 点击 `打开VNC显示` -> 点击 `跳转VNC控制` -> noVNC 页面出现后输入安装时配置的 VNC 密码。
- 验证：`go test ./internal/games/stardew_junimo -run Rendering`、`go test ./internal/web -run "Rendering|VNCConfig"`、`npm.cmd run build`。

# STEAM-QR-PHASE-CLASSIFY-1 联调契约

- 前端安装页继续以 `instance.driverPhase` 决定认证交互区：`steam_qr_required` 显示“Steam 手机扫码”和打开扫码窗口按钮，`steam_guard_mobile_required` 才显示“Steam Guard 验证 / 请打开 Steam 手机 App 批准登录”。
- 后端在用户选择扫码登录（`POST /api/instances/stardew/steam-guard/input`，`input="2"`，当前 phase 为 `auth_method_required`）后应保持 `driverPhase=steam_qr_required`，不应被上游日志 `Choice [1]: 2` 覆盖成 `steam_guard_mobile_required`。
- 前端安装页有防御性兜底：如果当前 `driverPhase=steam_guard_mobile_required`，但最近安装日志显示 `Choice [1]: 2` 或“已选择扫码登录”，且之后没有真正的 Steam Guard 菜单，则按 `steam_qr_required` 渲染 QR 区域。
- QR 弹窗契约：前端应从最新 `Or open: https://s.team/q/...` 日志行提取 Steam 登录 URL，并在本地生成标准二维码图片；终端字符画只能作为备用显示，不能作为主扫码源，也不能把最近多段 `[steam]` 日志直接塞进二维码窗口。
- 前端交互契约：管理员提交 Steam 认证选择后，页面应立即进入对应的本地乐观阶段，不等待后端 `driverPhase` 下一轮刷新。`auth_method_required + input=2` 立即显示 QR 等待；`steam_guard_choice_required + input=1/2` 分别立即显示手机批准等待/验证码输入框。若提交失败，前端回退并显示错误。
- 如果 QR 流程最终出现 `QR authentication failed: SteamClient did not connect...`，应进入 `qr_auth_failed` 或连接失败类状态；前端应提示 QR 登录失败/网络连接问题，而不是继续显示 Guard 手机批准。
- 联调网络判断：容器能解析 Steam 域名、连通 `api.steampowered.com:443` 与 Steam CM 端口，只说明 Docker 基础网络可用；SteamClient 仍可能因 CM 会话不稳定、地区网络或上游 QR 流程问题连接失败。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "QRCodeChoice|SteamMobileApproval|SteamAuthMenus"`。
# STEAM-POST-AUTH-RETRY-1 联调契约
- Steam 认证成功后，任何游戏下载、Steam CDN、磁盘、SDK 或后续安装步骤失败，都不得再把用户引导回 Steam 账号密码输入。后端应使用 `state=error` 搭配 `driverPhase=download_failed` 或 `post_auth_failed`；不要把这类失败写成 `state=steam_auth_failed`。
- 前端应把 `[steam] Logged in as`、`Token expires`、`Game license verified`、`Got depot decryption key`、`Downloading app 413150`、`Target directory: /data/game` 视为“认证已成功”的日志信号。若后续失败，安装页只显示复用已保存凭据重试入口，并提交 `POST /api/instances/:id/install` with `{ "reuseCredentials": true }`。
- 只有真正凭据错误才使用 `credentials_required` 并要求重新输入账号密码；QR 登录未成功也可以提示用户改用账号密码。下载失败、CDN 403、manifest 失败、磁盘不足、后续容器步骤失败都不属于凭据错误。
- 验证建议：模拟日志顺序 `Logged in as -> Downloading app 413150 -> Download failed: ...403`，实例最终应为 `error/download_failed`，前端按钮应为“重试下载（不重新输入账号）”，表单不出现 Steam 用户名/密码字段。
# PULL-PROGRESS-1 镜像拉取进度契约

- 安装 job 日志中的 `[pull:progress:done:total]` 是前端专用隐藏进度信号。
- `pull_running` 阶段的 `done/total` 表示 Junimo 镜像数量；`steamcmd_image_pulling` 阶段的 `done/total` 表示 SteamCMD 镜像 layer 数量。前端应展示为估算百分比，不要要求用户从 Docker layer 日志里猜进度。

# STEAMCMD-DOWNLOAD-PROGRESS-1 游戏文件进度契约

- SteamCMD 游戏文件下载进度不新增 API；前端从 job 日志中的 `[steamcmd] ... progress: N (done / total)` 解析百分比。
- `Success! App '413150' fully installed.` 是 Stardew Valley 游戏文件完成标记；`Success! App '1007' fully installed.` 仅表示 Steam SDK 运行文件完成。
- SteamCMD 手机 App 批准提示包括 `Please confirm the login in the Steam Mobile app` 和 `Waiting for confirmation`；批准超时属于 `steamcmd_failed`，不是安装成功。
# STEAMCMD-BRACKET-PROGRESS-1 兜底下载进度契约补充

- SteamCMD 兜底下载进度来源包括两类日志：`[steamcmd] ... progress: N (done / total)` 和 SteamCMD 原生 `[steamcmd] [ 28%] Downloading update (11,467 of 40,273 KB)...`。
- 前端应把上述两类日志都视为 `steamcmd_downloading`，并展示百分比与已下载/总大小；后端无需新增进度 API。
- SteamCMD 授权提示仍以日志和 `driverPhase` 双兜底：`Please confirm the login in the Steam Mobile app` / `Waiting for confirmation` 对应 `steamcmd_guard_mobile_required`。
# JUNIMO-IMAGE-CANDIDATES-1 联调契约

- 安装页看到 `driverPhase=pull_running` 时，后端可能正在拉取 `steam-auth-cn` 或 `JunimoServer` 候选镜像；日志前缀分别为 `[steam-auth:pull]`、`[server:pull]`，进度仍通过隐藏日志 `[pull:progress:done:total]` 给前端估算。
- 候选顺序为国内镜像源优先，然后 `ghcr.io`，最后原始仓库。单个候选失败会继续尝试下一项，不应立即把安装视为失败；只有全部候选失败时才显示 `pull_failed`。
- 成功命中的候选镜像会写回实例 `.env` 的 `STEAM_SERVICE_IMAGE` 或 `SERVER_IMAGE`，后续 compose / steam-auth TTY 均使用该选中镜像。
- 前端无需新增接口；继续展示 job 日志和 `pull_running` 进度即可。
# JUNIMO-IMAGE-CANDIDATES-2 安装页镜像候选联调

- 安装流程进入 Junimo 镜像检查时，后端会对 `steam-auth cn` 与 `server` 两类镜像分别展开默认候选源；旧 `.env` 中只有单候选值时也会被补齐。
- 前端日志应能看到 `server` 缺失时最多按 `(1/4)` 到 `(4/4)` 尝试：`docker.1ms.run/sdvd/server:<IMAGE_VERSION>`、`docker.m.daocloud.io/sdvd/server:<IMAGE_VERSION>`、`ghcr.io/sdvd/server:<IMAGE_VERSION>`、`sdvd/server:<IMAGE_VERSION>`。
- `steam-auth cn` 同理最多五个候选：`docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2`、`docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`。
- 命中本地任一候选时应直接显示“本地已有镜像 ... 直接使用”，不应先拉取排在前面的缺失候选。
