# STEAMCMD-REPAIR-DIRECT-1 修复/重新安装直达 SteamCMD

- `POST /api/instances/:id/install` 在收到 `reuseCredentials=true` 时，除了继续从实例 `.env` 读取已保存 `STEAM_USERNAME` / `STEAM_PASSWORD` / `VNC_PASSWORD`，现在会显式传递 `SteamCMDRetry=true` 给 driver。
- 这条路径用于安装页“重新安装 / 修复”、认证后下载失败重试、SteamCMD 重试等复用凭据入口；后端会跳过 Junimo 镜像检查和 `steam-auth`，直接进入 SteamCMD 下载/校验，不再让用户重新输入 Steam 账号密码，也不再走一遍 `steam-auth` 登录流程。
- SteamCMD 直达模式会优先使用已有 SteamCMD 登录授权缓存执行 `+login "$STEAM_USERNAME" +app_update 413150 validate ...`，不再在命令里用账号密码触发新一轮 Steam Guard 批准；首次兜底且没有缓存的旧路径仍保留账号密码登录能力。
- 如果直达修复模式下 SteamCMD 仍输出 Guard/批准提示，后端不会切到 `credentials_required` 要用户重新输入凭据，而是按 `steamcmd_failed` 报告“授权缓存不可用”，避免 UI 误导用户再次输入账号密码。
- 影响文件：`backend/internal/web/install_handlers.go`、`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver_test.go`。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。

# PUBLIC-IP-LOOKUP-1 服务器公网 IP 检测接口
- 新增 `GET /api/instances/:id/public-ip`，登录用户可调用，用于从面板后端所在服务器主动检测公网出口 IP；前端不要在浏览器里查 IP，避免拿到访问者客户端的公网 IP。
- 接口默认使用短超时 HTTP 客户端按顺序请求 `api.ipify.org`、`checkip.amazonaws.com`、`ifconfig.me/ip`，只接受 `netip.ParseAddr()` 可解析且非 private/loopback/link-local/multicast/unspecified 的公网地址。
- 响应结构为 `{ ip, checkedAt, source?, cached }`。后端默认缓存成功结果 `10min`；请求 `?refresh=1` 或 `?refresh=true` 会强制重新检测。失败返回 `502 public_ip_failed`，不会暴露外部服务原始错误。
- 影响文件：`backend/internal/web/public_ip.go`、`backend/internal/web/public_ip_test.go`、`backend/internal/web/handler.go`、`backend/internal/web/instance_handlers.go`。未改 Junimo driver、Docker/Compose 状态、邀请码接口或实例状态模型。
- 验证：`cd backend; go test ./internal/web`。

# SUPPORT-BUNDLE-STREAM-1 支持包流式 ZIP 导出

- `POST /api/instances/:id/support-bundle` 不再先用 `bytes.Buffer` 在内存中完整拼出 ZIP 后一次性写回，而是设置下载响应头后直接用 `zip.NewWriter(w)` 流式写入 `http.ResponseWriter`。
- 响应继续是 `application/zip` 和 `support-bundle-YYYYMMDD-HHMMSS.zip`；因为改为流式响应，不再设置 `Content-Length`，浏览器下载和前端 Blob 处理不受影响。
- 支持包内容不变：`version.json`、`health.json`、`instance-state.json`、`jobs.json`、`audit-logs.json`、`compose-ps.json`、`docker-compose.yml` 或说明、`server-logs.txt`，敏感信息仍通过 Docker redact 逻辑脱敏。
- 新增 `TestSupportBundleStreamsValidZip` 覆盖下载仍是合法 ZIP、关键条目存在且不写固定 `Content-Length`。
- 验证：`cd backend; go test ./internal/web -run "SupportBundle|Docker|Metrics"`，`cd backend; go test ./...`。

# DOCKER-POLL-PERF-1 Docker Compose 状态短缓存与轮询边界

- `backend/internal/docker.Client.ComposePs()` 现在对成功的 `docker compose ps --format json` 结果做短 TTL 缓存，默认 `1.5s`。同一实例在状态页、资源页、支持包或诊断路径短时间内重复读取 Compose 状态时，可复用同一份解析结果，减少 Docker CLI 进程启动开销。
- 缓存只覆盖 `ComposePs` 成功结果，不缓存失败；`ComposeUp`、`ComposeDown`、`ComposeRestart` 和 `ComposeRestartServices` 会在执行前后清理对应 workDir 的 `ComposePs` 缓存，避免生命周期命令后短时间读到旧状态。
- `ComposeStats --no-stream` 不做后端缓存，仍只通过 `/api/instances/:id/metrics` 按需执行。它比 `ComposePs` 重，前端应限制为诊断/资源可见页低频刷新。
- `DockerVersion` / `ComposeVersion` 仍用于 `/api/health/diagnostics`、Docker 状态页或安装前检查，不应进入普通总览常驻轮询。
- 验证：`cd backend; go test -count=1 ./internal/docker`。

# JUNIMO-MOD-MOUNT-RESTORE-1 官方 JunimoServer Mod 挂载修复

- 根因：实例 `.local-container/mods` 会完整挂载到容器 `/data/Mods`，如果宿主目录里没有 `JunimoServer/`，就会遮住 `sdvd/server` 镜像内置的官方 `JunimoServer` Mod，导致 8080 API、邀请码、VNC rendering 全部不可用。
- 启动/重启前现在会检查 `.local-container/mods/JunimoServer/manifest.json`；缺失时用当前 server 镜像临时容器把 `/data/Mods/JunimoServer` 同步回宿主 mods 目录。
- `JunimoHost.Server` 现在按内置服务端组件处理：永远启用、不可切换、不写入玩家同步包、不受新存档“默认禁用第三方 Mod”影响。
- 物理 `mods/smapi/` 是 SMAPI 自带运行时目录，不再作为本地 Mod 扫描，页面只保留虚拟的 `SMAPI` 内置卡，避免出现重复 `smapi` 且解析失败。
- VNC rendering 调用如果遇到 Junimo API connection refused，会返回 `junimo_api_unavailable` 结构化错误，不再只显示笼统 Docker 操作失败。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "ListMods|ApplyNewSaveDefault|ApplyModProfileKeeps|Rendering|JunimoServerMod"`，`cd backend; go test ./internal/web -run "Rendering|VNCConfig"`。

# ENV-BOM-NORMALIZE-1 实例 .env 隐藏 BOM 归一化

- `config.ReadEnvFile()` 现在会剥离行首和 key 前缀的 UTF-8 BOM，避免实例 `.env` 中混入 `﻿IMAGE_VERSION` 这类不可见变量名后，被 Docker Compose 报 `unexpected character "\ufeff"`。
- `UpdateEnvFile()` 通过读取归一化后的 key 再写回 `.env`，会把 BOM 污染的重复 key 收敛成正常 `IMAGE_VERSION`，不再把非法变量名保留下来。
- 已热修当前测试实例 `data/instances/stardew/.env`，删除中间残留的 BOM 前缀 `IMAGE_VERSION` 行；`docker compose -f data/instances/stardew/docker-compose.yml config --quiet` 已通过。
- 验证：`cd backend; go test ./internal/games/stardew_junimo/config`。

# STEAMCMD-SELFUPDATE-CACHE-1 SteamCMD 客户端自更新缓存

- SteamCMD 兜底容器现在额外挂载 `steamcmd-root-local` 到 `/root/.local/share/Steam`，并挂载 `steamcmd-user-local` 到 `/home/steam/.local/share/Steam`，用于持久化 SteamCMD 容器内客户端自更新文件。
- 用户日志里的 `[steamcmd] [ 40%] Downloading update (.. of 40,273 KB)` 是 SteamCMD 客户端自更新，不是 Docker 镜像拉取；后端会在启动 SteamCMD 前写明镜像检查已完成，避免误解。
- `buildSteamCMDOpts()` 会创建并 chown 上述目录；仍只使用 SteamCMD 候选镜像和现有 `game-data`、`steamcmd-login`、`steamcmd-home` 命名卷，不新增 Junimo/server 镜像来源。
- 影响文件：`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver_test.go`。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallSteamCMD"`。

# STEAMCMD-RETRY-RESUME-1 SteamCMD 兜底重试直达

- 安装请求复用已保存凭据时，如果实例当前 `driverPhase` 已处于 `steamcmd_auth_running`、`steamcmd_guard_*`、`steamcmd_downloading`、`steamcmd_failed` 或 `steamcmd_image_pull_failed`，`stardew_junimo` 会跳过 Junimo compose pull 和 `steam-auth`，直接进入 `runSteamCMDFallback()`。
- 直达重试会重新注册当前 job 的 Steam Guard 输入通道，所以前端继续使用 `POST /api/instances/:id/steam-guard/input` 提交 SteamCMD 手机批准选择或 App/邮箱验证码。
- `ensureSteamCMDImage()` 现在先 inspect 完整 `STEAMCMD_IMAGE_CANDIDATES` 列表；只要任意候选本地存在就直接使用，不会因为前序候选缺失而先 pull。只有所有候选都不存在时才按顺序拉取。
- 新增 `registry.InstallRequest.SteamCMDRetry` 作为显式入口；当前 web 层仍通过 `reuseCredentials=true` + 已持久化 `driverPhase` 由 driver 自动推断。
- 影响文件：`backend/internal/games/registry/types.go`、`backend/internal/games/stardew_junimo/driver.go`、`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver_test.go`。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallSteamCMD"`；前端联动验证见 `FE-STEAMCMD-RETRY-RESUME-1`。

# STEAMCMD-FALLBACK-1 steam-auth 下载失败自动切 SteamCMD

- `stardew_junimo` 安装流程中，`steam-auth` 已经认证成功但游戏文件下载失败时，不再直接结束为 `download_failed`。后端会在同一个 `stardew_install` job 内自动切换到 SteamCMD 兜底下载，复用 `.env` 中保存的 `STEAM_USERNAME` / `STEAM_PASSWORD`，不要求用户重新输入账号密码。
- 新增通用 Docker TTY 执行能力 `RunContainerTTY()` 和单镜像拉取 `PullImageStreaming()`；SteamCMD 会按 `STEAMCMD_IMAGE_CANDIDATES` 候选列表逐个 `inspect/pull`，默认顺序为 `docker.1ms.run/steamcmd/steamcmd:latest`、`docker.m.daocloud.io/steamcmd/steamcmd:latest`、`ghcr.io/steamcmd/steamcmd:latest`、`cm2network/steamcmd:latest`。旧实例仍写着旧候选列表时，安装流程会补齐新候选并过滤直连 Docker Hub 的 `steamcmd/steamcmd:latest` 和已移除的 `docker.xuanyuan.me/steamcmd/steamcmd:latest`；单次 Docker pull 默认等待 30 分钟。兜底容器挂载 `game-data` 到 `/data/game`，并挂载独立 `steamcmd-login` / `steamcmd-home` 命名卷保存 SteamCMD 登录授权缓存。
- SteamCMD 阶段新增可见 phase：`steamcmd_image_pulling`、`steamcmd_auth_running`、`steamcmd_guard_choice_required`、`steamcmd_guard_required`、`steamcmd_guard_mobile_required`、`steamcmd_downloading`、`steamcmd_failed`、`steamcmd_image_pull_failed`。需要验证时继续复用 `POST /api/instances/:id/steam-guard/input`，但文案明确是“steam-auth 国内网络波动下载失败，需要重新授权 SteamCMD”。
- SteamCMD 手机 App 批准超时不再视为安装成功；即使容器退出码为 0，也必须看到 `Success! App '413150' fully installed.` 才会把兜底下载判定为成功。SteamCMD `Update state ... progress: N (done / total)` 继续作为任务日志输出，供前端显示下载百分比。
- SteamCMD 命令会下载/校验 Stardew Valley app `413150`，并尝试把 Steamworks SDK Redistributable app `1007` 安装到 `/data/game/.steam-sdk`。若 SDK 未输出完成标记只记录 warning，游戏 app 完成仍视为兜底下载成功。
- 影响文件：`backend/internal/docker/streaming.go`、`backend/internal/docker/tty_run*.go`、`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver.go`、`backend/internal/web/install_handlers.go`、对应测试文件。
- 验证：`cd backend; go test ./internal/docker ./internal/web ./internal/games/stardew_junimo`；目标覆盖 `TestDriverInstallFallsBackToSteamCMDAfterSuccessfulAuthDownloadFailure` 和 `TestDriverInstallTriesNextSteamCMDImageCandidateAfterPullFailure`。

# INSTALL-INTERRUPTED-STATE-1 安装中断状态回收

- `jobs.Manager.RecoverInterruptedJobs()` 现在在面板重启时不仅把 queued/running job 标记为 failed，也会针对 `stardew_install` 的 instance target 同步把实例改为 `state=error`、`driverPhase=install_interrupted`，避免任务已失败但安装页仍读到旧的 `steam_auth_running`。
- `stardew_junimo` 安装流程中，`RunSteamAuthTTY` 自身返回错误时会把实例改为 `steam_auth_failed` / `driverPhase=steam_auth_failed`，并写入脱敏后的 job error log；未知非零退出码不再回退到误导性的 `credentials_required` phase。
- 影响文件：`backend/internal/jobs/manager.go`、`backend/internal/games/stardew_junimo/installer.go`、对应测试文件。接口路径不变，变化体现在实例状态和安装页可见 phase。
- 验证：`cd backend; go test ./internal/jobs ./internal/games/stardew_junimo`。

# NEXUS-ERROR-TEXT-1 Nexus 错误中文乱码修复

- `writeNexusError()` 的 Nexus 错误响应 message 已修复为正常 UTF-8 中文：未配置 Key、需要 OAuth/认证、未找到 Mod、Key 无效/权限不足、请求过频和通用请求失败不再返回 `璇锋眰澶辫触` 这类 mojibake。
- 错误码和 HTTP 状态不变：`nexus_api_key_missing`、`nexus_auth_required`、`nexus_mod_not_found`、`nexus_unauthorized`、`nexus_rate_limited`、`nexus_request_failed` 仍按原契约返回。
- 新增 `TestWriteNexusErrorMessagesAreReadableChinese`，直接覆盖 `writeNexusError` 的可读中文响应，防止 Nexus 搜索/安装失败时再次把乱码透给前端。
- 验证：`cd backend; go test ./internal/web -run TestWriteNexusErrorMessagesAreReadableChinese`。

# PLAYERS-SAVE-ROSTER-1 存档离线玩家名册恢复

- `GET /api/instances/:id/players` 的 Junimo driver 合并逻辑现在会在 `StardewAnxiPanel.Control players.json` 在线快照和 `players-cache.json` 之外，读取当前存档主 XML 的 `<player>` 与 `<farmhands><Farmer>`，把存档里存在但当前不在线、也尚未进入缓存的玩家补入名册。
- 存档目录优先取 Junimo gameloader 当前存档；找不到时按控制快照 `saveId` 匹配 `Saves/<saveId>` 或 `Saves/<saveId>_*`。因此控制文件里 `saveId=test`、真实目录为 `test_442477055` 时，farmhand `test` 会显示为 `status=offline`、`source=save_file`。
- 仍由 `backend/internal/games/stardew_junimo/players.go` 完成，不在 Web/API 层堆 Stardew XML 逻辑；在线快照会覆盖同一玩家的存档离线项，缓存仍按 `saveId` 隔离，避免上一存档玩家串到当前存档。
- 新增 `TestListPlayersMergesControlSnapshotWithSaveFarmhands` 覆盖“缓存只有 host / 在线快照只有 host / 存档 farmhands 有 test”的场景。
- 验证：`cd backend; go test ./internal/games/stardew_junimo`；`cd frontend; npm.cmd run build`。

# JOB-DISPLAY-NAME-1 任务展示名字段

- jobs 表新增 `display_name` 字段（迁移 `007_job_display_name.sql`），用于保存面向用户的任务展示名；`type` 继续保持机器可读任务类型，不用于拼接 Mod 名，避免影响任务筛选和历史耗时统计。
- `jobs.Spec` / `CreateJobParams` 增加 `DisplayName`，`GET /api/jobs`、`GET /api/jobs/:id` 和 job SSE 里的 job payload 增加 `displayName`。
- Nexus/远程 Mod 安装任务现在写入展示名：`mod_nexus_install · <Mod 名>`、`mod_remote_install · <Mod 名>`；若请求未带名称但有 `modId`，退回 `Nexus Mod #<id>`；普通任务和旧任务没有展示名时前端继续按 `type` 展示。
- 验证：`cd backend; go test ./internal/storage ./internal/jobs ./internal/web`。

# MODUPLOAD-DUPLICATE-CODE-1 重复 Mod 上传返回专用错误码

- `POST /api/instances/:id/mods/upload` 在 `UploadModZip` 命中已安装相同 `UniqueID` 时，现在返回错误码 `mod_exists`，不再统一归为 `invalid_mod_zip`。
- ZIP 结构校验、manifest 解析、XNB 替换包识别和真正损坏 ZIP 的行为不变；只有错误消息含后端重复标记 `(mod_exists)` 时才切换错误码。
- 前端已有 `mod_exists -> 已安装相同 ID 的 Mod` 文案，因此用户重复上传已安装 Mod 时不会再误以为 ZIP 无法解析。
- 验证：`cd backend; go test ./internal/web -run "TestModUpload"`。

# NEXUS-EXT-DOWNLOAD-GUARD-1 远程 Mod 下载进度与网页响应拦截

- `mod_remote_install` / `mod_nexus_install` 的 ZIP 下载阶段现在会向 job 日志输出可见进度：连接远程下载服务器、HTTP 响应码、响应 `Content-Type`、压缩包大小，以及下载进度“已下载 / 总量 / 剩余 / 百分比”。无 `Content-Length` 时仍会输出已下载大小与“总大小未知”。
- 下载进度在 `stardew_junimo.nexusDownloadArchive()` 内统一实现，NXM、Nexus API Key 直连和浏览器扩展捕获 CDN ZIP 三条路径共享；日志按 5 MB 或 2 秒节流，完成时补最终进度，避免大包刷屏。
- 如果远程响应 `Content-Type` 是 `text/html`，后端会立即失败并提示“远程下载返回的是网页，不是 ZIP 压缩包；请确认浏览器扩展已经拿到 Nexus CDN ZIP 下载链接”，避免把 Nexus 下载页/错误页当作 ZIP 任务继续跑。
- 浏览器扩展提交层额外兜底：`background.finishInstall()` / `postRemoteInstall()` 和 `panel-bridge` 只允许真实 Nexus CDN `.zip` 链接创建面板远程安装任务；还停在普通 Nexus 下载页时不会再提前触发后端任务。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "InstallNexusMod|NexusDownloadArchive|Remote|Download"`、`go test ./internal/games/stardew_junimo ./internal/web`、扩展 `node --check`。

# PLAYERSYNC-PACK-10 禁用终端进度渲染

- 玩家同步包 `tools/install.ps1` 完全禁用控制台进度渲染：设置 `$ProgressPreference = "SilentlyContinue"`，并移除 `Show-InstallProgress` / `Complete-InstallProgress` 里的 `Write-Progress`。
- 进度百分比仍写入 `.anxi-sync/logs/install-*.log`，用于排查；玩家终端只显示 `Write-Step` 的独立任务行和最终摘要。
- 修复真实 Windows Terminal 中 `Write-Progress` 与中文 `Write-Host` 混用导致的“安安装装/已已跳跳过过”双字重叠和多行粘连。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 已热修，并在 `D:\steam\steamapps\common\Stardew Valley` 真实安装验证输出干净。

# PLAYERSYNC-PACK-9 Steam 启动项摘要精简

- 玩家同步包安装完成摘要移除单独的 `SMAPI 路径` 输出，只保留玩家真正需要复制到 Steam 的 `Steam 启动项文本`。
- 最终输出形态为 `Steam 启动项：已设置/未自动设置` + `Steam 启动项文本：` + 完整 launch options，一行标题、一行可复制内容。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 已热修，并真实验证输出不再显示单独路径。

# PLAYERSYNC-PACK-8 终端输出清理

- 玩家同步包 `tools/install.ps1` 移除自绘单行进度条的 carriage-return 覆盖输出，避免 Windows Terminal / bat 捕获中文宽度时出现重复字、残留字和多行粘连。
- `Show-InstallProgress` 后续在 PLAYERSYNC-PACK-10 中已改为只把进度写入日志文件；控制台只输出 `Write-Step` 任务日志，一行一条。
- `tools/steam-launch-options.ps1` 的建议启动项改为标题和启动项文本分两行输出。
- 安装完成摘要不再写“未自动设置，请查看上方提示”，而是固定输出 `Steam 启动项：已设置/未自动设置` 和 `Steam 启动项文本`，方便玩家直接复制。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 与 `tools\steam-launch-options.ps1` 已热修，并用真实安装验证输出干净。

# PLAYERSYNC-PACK-7 相同 Mod 跳过安装

- 玩家同步包 `tools/install.ps1` 新增目录内容指纹比对：安装每个 Mod 前分别计算 `payload/mods/<folderName>` 与 `<Stardew Valley>/Mods/<folderName>` 的稳定指纹。
- 指纹由目录相对路径、文件相对路径、文件大小和 SHA256 组成；完全一致时认为目标 Mod 已经是同一份内容，脚本会跳过备份和复制，并在 `installed.json` 对该 Mod 写入 `skippedIdentical=true`。
- 版本不同或文件内容不同不会跳过：只要 manifest、DLL、图片、content.json 等任意文件内容/路径/大小变化，指纹就不同，脚本会先备份旧目录再复制新目录。版本号相同但文件变了也会更新。
- 全部 Mod 都跳过且没有实际备份时，`installed.json.backupId` 写入 `null`，避免指向一个不存在的备份目录。
- 已热修当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1`，并用当前游戏目录验证三项已安装 Mod 均会 `已跳过相同 Mod`。

# PLAYERSYNC-PACK-6 单行动态进度

- 玩家同步包 `tools/install.ps1` 的文本进度条改为单行动态刷新，不再为 checksum / Mod 复制的每个进度 tick 都 `Write-Host` 新行。
- 新增 `Render-InstallProgressLine` / `Clear-InstallProgressLine` / `Finish-InstallProgressLine` / `Redraw-InstallProgressLine`：控制台使用 `[Console]::Write([char]13...)` 回到行首刷新当前进度；普通安装事件通过 `Write-Step` 先清除进度行、打印事件，再重绘进度行。
- 日志文件仍会保留关键安装步骤与进度记录；控制台上则只保留一条活动进度线，避免玩家看到大量重复进度行。
- 已热修当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1`。

# PLAYERSYNC-PACK-5 SMAPI payload 安装修复

- 玩家同步包 `tools/install.ps1` 恢复调用 SMAPI 官方 Windows 安装器：解压随包 SMAPI ZIP 后定位 `internal/windows/SMAPI.Installer.exe`，传入 `--install --game-path "<Stardew Valley>" --no-prompt`。
- 真实 Windows 测试确认 SMAPI 4.5.2 安装器没有可用静默参数，非交互调用会进入交互流程并可能因为 Console 句柄失败；失败安装还可能把安装器运行时 DLL 散落到游戏根目录。
- 官方安装器调用通过 `Start-Process` 执行，脚本会等待安装器退出；超过 120 秒会终止并提示玩家关注安装器窗口是否在等待按键/输入。脚本不再直接解包 `install.dat`，也不做本机 `.NET` / `runtimeconfig` 特调。
- 热修复已同步到当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1`。已用 `-GamePath "D:\steam\steamapps\common\Stardew Valley" -SkipSteamLaunchOptions` 真实安装验证，SMAPI 和玩家同步 Mod 均安装成功。
- 测试补充：`TestPlayerSyncInstallScriptUsesSMAPIInstallPayload` 防止脚本回退到交互安装器调用；`TestPlayerSyncPowerShellScriptsParse` 覆盖脚本语法。

# PLAYERSYNC-PACK-4 安装进度显示

- 玩家同步包的 `tools/install.ps1` 新增 `Show-InstallProgress` / `Complete-InstallProgress`，安装时会同时调用 PowerShell `Write-Progress` 和输出文本进度条。
- 进度阶段覆盖：开始、环境检查、读取清单、定位 Stardew Valley、进程/权限检查、payload checksum、SMAPI 解压/安装、Mod 备份与复制、Steam 启动项、写入安装记录、完成。
- checksum 阶段按 `checksums.sha256` 文件数推进，Mod 阶段按 `packaged=true` 的 Mod 数推进；SMAPI 阶段显示“解压 SMAPI 安装包 / 释放 SMAPI 官方安装文件 / 完成”。
- 已同步热修当前解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1`；后续重新导出的包会自动带进度条。

# PLAYERSYNC-PACK-3 方括号 Mod 路径修复

- 玩家端安装脚本在校验 `checksums.sha256` 和复制/卸载 Mod 时，所有来自 payload 或 `installed.json` 的 Mod 路径必须使用 PowerShell `-LiteralPath`。
- 真实触发场景：`payload/mods/[CP] MultipleConstructionOrders/Assets/ConstructionWorker.png` 中的 `[CP]` 会被 `Test-Path` / `Get-FileHash -Path` 当作通配符字符集，导致明明存在的文件被误报为“checksum 指向的文件不存在”。
- 已修复 `tools/install.ps1` 的 checksum 校验、payload source 检查、目标 Mod 存在检查，以及 `tools/uninstall.ps1` 的安装记录、目标 Mod、备份目录检查。
- 测试补充：`TestPlayerSyncInstallScriptUsesLiteralPathsForModFolders` 防止模板退回通配符路径；真实解压包用等价 checksum 循环验证 9 个 payload 文件全部存在且 hash 匹配。

# PLAYERSYNC-PACK-2 玩家同步安装包

- `POST /api/instances/:id/mods/sync-pack/export` 仍返回 `stardew-player-sync-pack.zip`，但 ZIP 内容已升级为玩家可执行安装包：根目录包含 `安装玩家同步包.bat`、`卸载本同步包.bat`、`README.txt`、`pack-manifest.json`、`checksums.sha256`、`tools/` 和 `payload/`。
- 玩家需同步 Mod 写入 `payload/mods/<folderName>/`，不再放在 ZIP 根目录；`StardewAnxiPanel.Control` 继续永远排除。SMAPI 虚拟条目写入 `pack-manifest.json`，不会被当作普通 Mod 文件夹复制。
- `payload/smapi/smapi.json` 总会写入。导出逻辑会优先查找服务端实例目录下 `.local-container/smapi/`、`.local-container/control/smapi/`、`smapi/` 里的 `SMAPI*.zip`；找到时随包写入 `payload/smapi/` 并记录 SHA256，找不到时同步包仍可导出，但玩家脚本只安装 Mod 并提示自行安装 SMAPI。
- `checksums.sha256` 覆盖 `payload/mods` 文件和随包 SMAPI ZIP。安装脚本在玩家电脑上先校验 payload 完整性，再安装/更新 SMAPI、备份同名 Mod、复制新 Mod，并尽力设置 Steam 启动项。
- 玩家端安装状态写入游戏目录 `.anxi-sync/installed.json`、`.anxi-sync/backups/`、`.anxi-sync/logs/`。卸载脚本按 `installed.json` 移除本包安装的 Mod，并可通过 `-RestoreBackup` 恢复备份；不会默认卸载玩家已有 SMAPI。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`go test ./...`。

# NEXUS-SMAPI-THUMB-1 虚拟 SMAPI 缩略图补全

- `smapiRuntimeModInfo()` 现在给内置虚拟 SMAPI 条目补 `UpdateKeys: ["Nexus:2400"]`、`nexusModId=2400` 和 Nexus 页面 URL。
- `GET /api/instances/:id/mods` 继续通过 `EnrichNexusMetadataForMods()` 统一补全 Nexus 元数据；因此虚拟 SMAPI 没有真实 `manifest.json` 也会进入 GraphQL v2 补全链路，成功后返回 `pictureUrl/nexusSummary/downloadCount/endorsementCount/updatedAt`。
- `ApplyNexusMetadataToMods()` 现在会按同一个 Nexus `modId` 合并更完整的缓存记录，避免内容包自己的最小来源记录遮住主 Mod 已缓存的 `pictureUrl`。
- 缩略图仍走 `.local-container/control/nexus-mods.json` 缓存；首次拉取失败不会影响 Mod 列表，只会暂时显示前端占位。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 通过。

# MODZIP-3 UTF-8 BOM manifest 兼容

- `readModInfo` 读取 `manifest.json` 后会剥离 UTF-8 BOM（`EF BB BF`）再交给 `encoding/json` 解析，兼容部分 Nexus ZIP 内由 Windows 工具保存的 manifest。
- 修复的真实场景：`MultipleConstructionOrders 1.6 V1.1.0-47289-1-1-0-1780961270.zip` 中 `[CP] MultipleConstructionOrders/manifest.json` 带 BOM，之前会报 `invalid character 'ï' looking for beginning of value`，Web 层最终显示为 `Mod ZIP 无效`。
- 该修复不放宽 JSON 语法校验，只处理文件开头 BOM；非法 JSON 仍会按原逻辑拒绝。
- `sanitizeError` 现在会保留常见 Mod 上传校验原因（目录已存在、UniqueID 重复、具体 Mod 的 manifest 解析失败等），避免前端只显示笼统的“第 N 个 Mod ZIP 无效”。
- 验证：`go test ./internal/games/stardew_junimo -run "UploadModZip|ReadModInfo"`。

# MODZIP-4 manifest JSONC 兼容

- `readModInfo` 新增 `decodeModManifest`，先按标准 JSON 解析；失败后仅对 SMAPI `manifest.json` 做 JSONC 兼容清理：移除字符串外的 `//` 行注释、`/* ... */` 块注释和对象/数组尾随逗号，再重新解析。
- 修复的真实场景：Nexus CDN 远程安装 `SpaceCore` 时，ZIP 下载和解压已成功，但 `manifest.json` 含注释导致 `invalid character '/' looking for beginning of object key string`，最终 `mod_remote_install` 失败。
- 清理逻辑会保留字符串内容，例如 `https://...` 中的 `//` 不会被当作注释；ZIP 路径校验、manifest 必填字段、重复 UniqueID、XNB 替换包识别等安全规则不变。
- 验证：`go test ./internal/games/stardew_junimo -run "ReadModInfo|UploadModZip"`。

# MODDEPS-1 Mod 前置依赖字段

- `GET /api/instances/:id/mods` 的 `mods[]` 现在会从每个 Mod 的 `manifest.json` 解析前置依赖并返回 `dependencies[]`：`{ uniqueId, minimumVersion?, required }`。
- 支持 SMAPI 标准 `Dependencies` 数组，并把 `ContentPackFor` 也归一为一个必需依赖；例如 Content Patcher 内容包会返回 `Pathoschild.ContentPatcher`，前端可直接展示“需要前置依赖：Content Patcher”。
- `IsRequired` 缺省按必需依赖处理，`false` 会作为可选依赖返回；同一个 `UniqueID` 在 `ContentPackFor` 和 `Dependencies` 中重复时会去重，优先保留 `ContentPackFor` 的最低版本信息。
- 该阶段只做解析和展示字段，不自动下载依赖，也不判断依赖是否已安装；真正的缺失依赖检查仍留给后续配置页能力。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 覆盖 `Dependencies`、可选依赖和 `ContentPackFor` 去重。

# MODZIP-2 Nexus 外壳目录自动剥离

- `UploadModZip` 支持 Nexus 常见的“单层外壳目录”ZIP：如果压缩包根目录只有一个包装目录，且包装目录本身没有 `manifest.json`，后端会向内看一层，把其中带 `manifest.json` 的子目录作为真正的 SMAPI Mod 导入。
- 示例结构 `MultipleConstructionOrders/MultipleConstructionOrders/manifest.json` + `MultipleConstructionOrders/[CP] MultipleConstructionOrders/manifest.json` 会自动导入为服务器 `Mods/MultipleConstructionOrders` 和 `Mods/[CP] MultipleConstructionOrders`，不要求用户手动解压重打包。
- 仍只剥离一层外壳目录；zip-slip、绝对路径、重复目录名、重复 UniqueID、已安装冲突和 XNB 替换包识别等原有安全校验继续生效。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 覆盖 Nexus 外壳目录多 Mod 包上传。

# MODRESTART-1 停服改 Mod 不再提示重启

- 当前所有 Mod 写操作（上传、删除、Nexus 一键安装、远程 ZIP 安装）都要求服务器不在 running/starting。既然写入发生在停服状态，下次正常启动会直接加载新 Mod，因此不再设置 `modsRestartRequired`。
- `GET /api/instances/:id/mods` 只会在实例状态为 running/starting 且底层标记存在时返回 `restartRequired=true`；实例 stopped/ready_to_start 等停服状态永远返回 `false`，避免用户停服改完 Mod 后仍看到“需要重启”。
- 停止服务器流程会清理旧的 `modsRestartRequired` 标记；停服上传/删除/安装成功后也会清理历史残留。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 覆盖停服上传返回 `restartRequired=false` 和 Nexus 安装不再置位。

# MODUPLOAD-2 多 ZIP 批量上传
- `POST /api/instances/:id/mods/upload` 现在支持一次上传多个 Mod ZIP：前端可重复提交 `mod` 字段，后端也兼容 `mods` 字段。总请求大小仍沿用 `maxModFormSize` 限制，不为每个文件单独放宽。
- Handler 会逐个把上传文件写入临时 ZIP，再统一交给 `stardew_junimo.UploadModZip(dataDir, tmpPath)` 校验、解压和导入；因此单个 ZIP 里包含多个顶层 SMAPI Mod 的既有能力仍然保留。
- 本次批量上传采用“批次失败回滚”语义：如果第 N 个 ZIP 写入或导入失败，会调用 `rollbackImportedMods` 逆序删除本次请求前面已经导入的 Mod 文件夹，并返回结构化错误。这样避免用户一次选择多个 ZIP 时只成功一半。
- 成功时返回 `registry.ModsListResult`，其中 `mods` 是本次所有 ZIP 导入出来的 Mod 汇总；`restartRequired` 继续遵循 `MODRESTART-1` 语义，停服上传返回 `false`，运行中历史标记只在实例 running/starting 时展示。
- 新增回归测试 `TestModUpload_AcceptsMultipleZipFiles`，覆盖同一个 multipart 请求中重复 `mod` 字段上传两个 ZIP 并返回两个导入结果。

# NEXUS-META-1 GraphQL 无 Key 元数据补全
- `SearchNexusMods(ctx, query, apiKey)` 现在在“纯数字 query 且未配置 Nexus API Key”时也会走 GraphQL v2 精确 ID 查询：`ModsFilter` 使用 `gameId=1303` + `modId=<id>`，不再把无 Key 数字搜索降级成关键词搜索。配置了 Key 时仍保留 v1 REST 精确 ID 查询。
- `GET /api/instances/:id/mods` 改为调用 `EnrichNexusMetadataForMods(ctx, dataDir, mods)`：先读取已有 `.local-container/control/nexus-mods.json`，再对缺失 sidecar 的本地 Mod 解析 `UpdateKeys: ["Nexus:<id>"]`，通过 Nexus GraphQL v2 拉取 `pictureUrl/summary/downloads/endorsements/updatedAt` 等展示元数据。
- GraphQL 补全是非阻断逻辑：Nexus 请求失败不会影响 Mod 列表，只会暂时保留本地图标；成功后写回 sidecar，后续列表请求直接走缓存，不需要 API Key，也不需要重启。
- Nexus API Key 仍只用于 v1 文件列表/下载链路等受限能力；普通搜索、数字 ID 展示元数据、手动上传后缩略图补全都不再依赖 Key。

# NEXUS-PAGED-1 搜索排序与分页
- 当前在线搜索入口是 `GET /api/instances/:id/mods/nexus/search`。关键词搜索通过 Nexus GraphQL v2 下推 `downloads DESC` 排序，并使用 `page/pageSize` 计算 `offset/count`。
- 空查询作为 Stardew Valley 默认热门列表处理；数字 ID 搜索仍支持无 Key GraphQL 精确查询和有 Key v1 REST 精确查询。
- 旧 `SearchMods` / `ModSearchResult` 统一搜索骨架已撤回，不再作为当前接口契约。

# MODZIP-1 XNB 替换包提示

- `UploadModZip` 仍只安装标准 SMAPI Mod（顶层 Mod 目录需要 `manifest.json`）。Nexus 上的老式 XNB 替换包虽然也是 `.zip`，但只包含 `Characters/*.xnb`、`Portraits/*.xnb` 等游戏内容替换文件，不能放进服务器 `Mods` 目录。
- 新增 XNB 替换包识别：如果 ZIP 内没有任何 SMAPI `manifest.json`，但包含 `.xnb` 且路径像 `Characters/`、`Portraits/`、`Content/`、`Maps/`、`TileSheets/`，后端返回明确错误：这是 XNB 替换包，不是 SMAPI Mod，不能上传到服务器 Mods 目录。
- Web 层 `sanitizeError` 对 XNB/SMAPI/manifest 这类产品级错误保留友好提示，不再统一压成“压缩包格式错误或已损坏”。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 覆盖 XNB 替换包识别和错误脱敏。

# MODSEARCH-1 撤回记录

- `/api/instances/:id/mods/search` 与 `/api/instances/:id/mods/search/install` 已撤回，当前不注册路由。
- `backend/internal/games/stardew_junimo/mod_search.go` 和对应测试已删除；当前只保留 Nexus 专用搜索/安装接口，以及管理员粘贴 NXM/CDN ZIP 的远程安装兜底。
- 后续如果重新做 StardewModDataset、CurseForge、ModDrop、GitHub Release 等多来源搜索，应重新设计接口和排序/去重策略，不要假设当前仍有统一搜索后端契约。

# NEXUS-3 下载安装与已安装卡片元数据

- `SearchNexusMods(ctx, query, apiKey)` 已调整为：纯数字 query 在已配置 Nexus API Key 时走 v1 REST 精确 ID 查询；无 Key 时走 GraphQL v2 的 `gameId=1303 + modId` 精确元数据查询，避免展示型 ID 搜索依赖 Key。
- 新增 `POST /api/instances/:id/mods/nexus/install`，管理员专用且要求服务器不在 running/starting。接口请求体复用搜索结果卡片字段，后端创建 `mod_nexus_install` job，通过 Nexus v1 `files.json` + `download_link.json` 获取主文件下载链接，下载 ZIP 后交给既有 `UploadModZip` 校验/解压/导入，并设置 Mod restart-required 标记。
- 新增 `.local-container/control/nexus-mods.json` 面板自有元数据文件，按已安装 Mod 文件夹保存 Nexus 卡片信息（summary、pictureUrl、nexusUrl、downloads、endorsements、updatedAt 等）。`GET /api/instances/:id/mods` 和 sync plan 会调用 `ApplyNexusMetadataToMods`，因此前端可把服务器已安装 Mod 展示成与搜索结果相同的卡片。
- 相关文件：`backend/internal/games/stardew_junimo/nexus_install.go`、`nexus_metadata.go`、`nexus.go`、`mod_sync.go`、`backend/internal/games/registry/types.go`、`backend/internal/web/lifecycle_handlers.go`、`backend/internal/web/instance_handlers.go`。
- 验证：`go test ./...` 已覆盖无 Key 数字查询 GraphQL fallback、Nexus ZIP 下载安装、缩略图元数据保存与已安装列表回填。

# 后端文档

## 总体结构

后端是 Go 单体服务，负责面板鉴权、任务编排、Docker/Compose 控制、Stardew Junimo driver、SQLite 持久化、审计和静态前端托管。

建议边界如下：

```text
backend/cmd/panel              启动入口
backend/internal/auth          登录、密码、session
backend/internal/docker        Docker / Compose 封装与脱敏
backend/internal/jobs          长任务、日志、SSE
backend/internal/storage       SQLite、迁移、用户、状态、审计
backend/internal/static        嵌入 frontend/dist
backend/internal/web           HTTP handler、鉴权、路由
backend/internal/games         GameDriver 与游戏实现
backend/internal/games/stardew_junimo
```

Stardew 业务逻辑优先放在 `backend/internal/games/stardew_junimo`，不要散落到通用 handler 或 Docker 层。

## GameDriver 边界

`GameDriver` 表示“面板对某个游戏实例下达通用意图”的后端接口。Stardew 当前 driver ID 是 `stardew_junimo`，默认实例 ID 是 `stardew`。

driver 应负责：

- 实例目录准备与 compose 模板。
- 安装流程与 Steam Auth。
- 启动、停止、重启、状态校准。
- 邀请码读取。
- 存档、Mod、控制台命令、玩家信息。
- 与 JunimoServer 容器通信。

通用 web 层只做鉴权、参数解析、调用 driver、写审计、返回结构化错误。

## Junimo 通信优先级

1. 挂载文件：`.env`、`docker-compose.yml`、settings、saves、mods、backups、control JSON。
2. Docker Compose：`pull`、`up -d`、`down`、`restart`、`ps`、`logs`、`exec`、`run`。
3. Steam Auth TTY：扫码登录走 `steam-auth setup`，账号密码安装优先走 `steam-auth download`。
4. Junimo / SMAPI 控制：`attach-cli` 或当前 FIFO / output log 通信。
5. Junimo HTTP API：仅在启用并具备 API key 时用于状态或监控。
6. VNC：只做高级调试，不作为常规控制通道。

## 状态与任务

核心状态包括：

```text
uninitialized
admin_created
junimo_scaffolded
credentials_required
steam_auth_running
steam_auth_failed
steam_auth_done
game_installed
save_required
ready_to_start
starting
running
stopped
error
```

`jobs` 和 `job_logs` 用于安装、认证、启动等长任务。日志通过：

```text
GET /api/jobs/:id/stream
```

推送 `log`、`finished`、`ping` 事件。写日志前必须脱敏。

生命周期任务必须可取消。同一实例的 `stardew_lifecycle` 任务采用“最后操作生效”：新的启动、停止或重启请求会先取消该实例仍在 queued/running 的旧生命周期任务。被取消的任务状态必须落到 `canceled`，不能继续挂在 running，也不能在取消后把实例状态重新写回 running。

## 主要 API 分组

| 分组 | 代表接口 |
| --- | --- |
| 健康与版本 | `GET /health`, `GET /api/version`, `GET /api/health/diagnostics` |
| 认证 | `GET /api/auth/status`, `POST /api/auth/setup`, `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me` |
| 用户 | `/api/users` 管理员接口 |
| 实例状态 | `GET /api/instances/:id/state` |
| 安装 | `POST /api/instances/:id/prepare`, `GET /api/instances/:id/install-options`, `POST /api/instances/:id/install`, Steam Guard 输入接口 |
| 生命周期 | `POST /api/instances/:id/start`, `stop`, `restart` |
| 邀请码 | `GET /api/instances/:id/invite-code` |
| 存档 | `GET /api/instances/:id/saves`, 上传预览、提交启动、选择、删除、导出、备份恢复 |
| Mods | `GET /api/instances/:id/mods`, 上传、删除、导出、玩家同步分类(`mods/sync-plan`、`mods/:modId/sync-classification`、`mods/sync-pack/export`)、Nexus 只读搜索(`mods/nexus/search`) |
| 命令 | `GET /api/instances/:id/commands`, `POST /commands/run`, `POST /commands/say` |
| 玩家 | `GET /api/instances/:id/players` |
| Docker 诊断 | compose ps/logs/status 相关调试接口 |
| 审计与支持包 | `GET /api/audit-logs`, `POST /api/instances/:id/support-bundle` |

新增接口时必须返回结构化错误码，前端通过错误码映射中文提示。

## Stardew 关键实现

### Steam Auth

- 官方服务名保留 `steam-auth`、`server`、`discord-bot`。
- 官方镜像版本变量是 `IMAGE_VERSION`。
- `steam-auth` sidecar 当前默认先使用 `docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`，第二候选使用阿里云 ACR 新版个人版镜像 `crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2`，再保留 DaoCloud、GHCR、Docker Hub 兜底。
- `.env` 会写入 Steam 连接等待和认证重试相关变量。
- 不要用普通 stdin 重定向跑 `steam-auth setup` 的账号密码分支；该分支会用 `Console.ReadKey()` 读密码，后台 pipe 会失败。

### 邀请码

Junimo 会把邀请码写入容器内 `/tmp/invite-code.txt`。`docker compose restart` 可能保留旧文件，因此重启前后要清理或过滤旧码，前端启动/重启后也要等待非旧码的新邀请码。

Stardew 生命周期里的“重启服务器”必须只重启 Compose 的 `server` 服务，不要重启 `steam-auth`。`steam-auth` 重启会重新登录 Steam，短时间多次尝试可能触发 `RateLimitExceeded`，导致 Junimo 启动时 30 秒内无法访问 steam-auth，最终日志显示 `Steam-auth service not ready` 且邀请码为 `n/a`。

### 控制台与喊话

- `POST /api/instances/:id/commands/run` 只执行 allowlist 中的固定命令，继续通过 Junimo FIFO `/tmp/smapi-input` 与 `/tmp/server-output.log` 读取结果，不允许任意 shell。
- `POST /api/instances/:id/commands/say` 不依赖上游 SMAPI `say` 命令；后端校验运行状态和 200 字符限制后，把 `broadcast` JSON 写入 `.local-container/control/commands/`。
- `StardewAnxiPanel.Control` 在 `UpdateTicked` 中消费 `broadcast` 命令，调用 Stardew multiplayer chat message 发送 `[Panel] <message>` 给所有玩家。消息会移除控制字符并按 CJK/Kana/Hangul/Cyrillic/Thai 粗略选择聊天字体语言码，避免中文公告显示为方块。
- 当前已验证 Junimo 镜像 `sdvd/server:1.5.0-preview.121` 也带上游 `/ws` + `chat_send` 聊天 relay；面板暂不直接依赖该 WebSocket，以避免面板容器网络和 API key 配置差异影响喊话。
- 更新了嵌入控制模组 DLL。已运行实例需要重启/重新准备 `server` 容器后，`installSMAPIMod` 才会把新 DLL 写入实例 Mods 目录。

### 玩家信息

当前通过 SMAPI 控制模组写 `.local-container/control/players.json`，后端合并 `players-cache.json` 输出玩家名册。前端 5 秒轮询，成本低且稳定。

收入口径由 SMAPI mod 写入并保持固定含义：`farmIncome` 始终表示农场/团队累计收入，`personalIncome` 始终表示玩家 `individualMoneyEarned` 个人累计收入；`totalMoneyEarned` 仅保留为旧字段兼容。修改嵌入 DLL 后必须同步 `embedded/smapi-mod` 和已部署实例，并重启 Stardew server 容器才会生效。

最近事件由后端根据当前在线快照和 `players-cache.json` 中上一轮状态差分生成，并写入 `.local-container/control/players-events.json`。当前支持首次记录、加入和离开事件，最多保留最近 50 条，并按 `saveId` 隔离，避免切换存档后显示旧存档活动。

`players-cache.json` 必须按存档隔离：缓存文件带 `saveId`，只有 `cache.saveId == players.json.saveId` 时才合并历史离线玩家。切换/新建存档后，旧存档名册不能混入新存档玩家列表；旧版无 `saveId` 的缓存遇到当前存档 ID 时应被忽略并在下一次读取后重写。

`maxPlayers` 兜底（PLAYERS-MAXPLAYERS-1）：`ListPlayers` 现在默认从当前存档的 `server-settings.json` 读取 `Server.MaxPlayers`（`readServerMaxPlayers`，文件缺失/解析失败/非正数时返回 nil），junimo info 解析出的上限仍会覆盖兜底值。这样服务器未运行或 info 输出不含上限时，`GET /api/instances/:id/players` 也能返回 `maxPlayers`，前端右栏/总览可显示"在线数/人数上限"。测试见 `players_test.go` 的 `TestReadServerMaxPlayers`。

### Mod 玩家同步包

`stardew_junimo` 提供 Mod 同步分类能力，逻辑全部在 `backend/internal/games/stardew_junimo/mod_sync.go`，不绕过 driver。

- `ModInfo` 新增 `syncKind`(`server_only` | `client_required` | `unknown`) 和可选 `syncNote`。`GET /api/instances/:id/mods` 在返回前会调用 `ApplyModSyncClassification` 补全这两个字段，前端不需要单独再拉一次分类。
- 分类持久化在面板自有文件 `.local-container/control/mod-sync.json`，绝不写入 Mod 自身的 `manifest.json`。没有手动覆盖时会自动识别：面板控制 Mod 默认 `server_only`；SMAPI `ContentPackFor` 内容包和常见 `[CP]`/`[AT]`/`[JA]` 等内容包前缀默认 `client_required`；其他第三方 Mod 为了联机安全也默认 `client_required`。用户可再手动改成 `server_only` 或 `unknown`。
- `GET /api/instances/:id/mods/sync-plan`：返回全部已装 Mod 及分类统计（`serverOnly`/`clientRequired`/`unknown`）。
- `PUT /api/instances/:id/mods/:modId/sync-classification`：登录用户可用，只写面板元数据，不受服务器运行状态限制。`:modId` 可以是文件夹名或 `UniqueID`（复用 `ResolveModFolder`，同 `DeleteMod` 的查找顺序）。
- `POST /api/instances/:id/mods/sync-pack/export`：导出全部 `syncKind == client_required` 的 Mod 为玩家同步安装包 ZIP，附带面板生成的 `pack-manifest.json`、`checksums.sha256`、安装/卸载脚本和 `payload/mods/`。服务器运行中也允许导出。`StardewAnxiPanel.Control` 无论分类如何，始终被排除在导出之外（双重保险：默认分类 + 导出时强制跳过）。没有任何 Mod 命中 `client_required` 时返回 `400 no_sync_mods`。

涉及：`backend/internal/games/registry/types.go`、`backend/internal/games/stardew_junimo/mod_sync.go`、`backend/internal/games/stardew_junimo/mods.go`（抽出共用的 `addModDirToZip`）、`backend/internal/web/lifecycle_handlers.go`、`backend/internal/web/instance_handlers.go`。

测试覆盖：分类文件读写、自动默认 client_required/server_only、导出只含 client_required、导出排除控制 Mod、`ResolveModFolder` 路径安全，见 `backend/internal/games/stardew_junimo/mod_sync_test.go`。

`mods.go` 的 `ExportModsZip` 原本在循环失败但 `w.Close()` 成功时会因 `if err := w.Close(); err != nil` 遮蔽外层 `err`，把失败误判为成功；已修复为循环内失败直接 `return`、`Close()` 结果直接赋值给外层 `err`，与 `ExportModSyncPackZip` 写法一致。

`ExportModSyncPackZip` 原本固定写入 `%TEMP%\stardew-player-sync-pack.zip`，两个并发导出请求会互相覆盖/截断对方的 ZIP，且一个请求失败后的 `defer` 清理可能删掉另一个请求正在 `ServeFile` 的文件；已改为 `os.CreateTemp("", "stardew-player-sync-pack-*.zip")` 生成唯一临时路径，对外 `Content-Disposition` 文件名固定用新增的 `PlayerSyncPackFileName` 常量，与实际磁盘路径解耦。

`SetModSyncClassification` 原本对 `mod-sync.json` 是无锁的 load-modify-save，两个几乎同时的分类更新请求会让后写入的覆盖先写入的；已加上按 `dataDir` 维度的 `sync.Mutex`（`modSyncLockFor`）包住整个读改写流程，并把 `saveModSyncStore` 改成临时文件 + `os.Rename` 的原子写入，避免中途崩溃留下半截 JSON。

### Nexus Mods 只读搜索（第二阶段）

`backend/internal/games/stardew_junimo/nexus.go` 提供 Stardew Valley（`stardewvalley` game domain）只读搜索，不做下载/安装。Nexus 个人 API Key 不再从环境变量读取，而是由管理员在前端配置后写入 SQLite `panel_settings`（key=`nexus_api_key`），当前请求即时读取，保存后无需重启后端即可生效。

- **鉴权按路径拆开，不是统一要求 Key**：
  - 查询关键词若是纯数字，配置 Nexus API Key 时按官方文档化的 v1 REST 接口 `GET https://api.nexusmods.com/v1/games/stardewvalley/mods/{id}.json` 精确查询；未配置 Key 时改走 GraphQL v2 `gameId=1303 + modId` 精确元数据查询。Key 不再是展示型 ID 搜索的前置条件。
  - 其余关键词走 GraphQL v2（`https://api.nexusmods.com/v2/graphql`，与 nexusmods.com 网站搜索框同源）做关键词搜索。这是**公开只读查询，不要求个人 API Key**——即使未配置 Key 也会正常发起请求（`setNexusHeaders` 在 Key 为空时直接不带 `apikey` 头，而不是发一个空头）；如果配置了 Key 会顺带带上（无害，可能有助于提升速率限制），但从不作为前置门槛。如果 Nexus 对这条公开查询本身返回认证类拒绝（HTTP 401/403，或 GraphQL `errors` 数组里出现 auth/forbidden/permission 相关字样），统一映射为哨兵错误 `ErrNexusAuthRequired`（不是 `ErrNexusAPIKeyMissing`，因为配置个人 Key 也未必能解决——这通常意味着需要 OAuth 或更高权限）。
  - **注意**：Nexus 官方 v1 REST API 文档中没有任何关键词全文搜索接口（只有按 ID 查询、最近更新列表、MD5 查询）。GraphQL v2 这条路径的查询结构（`nexusGraphQLSearchQuery`，见 `nexus.go`）**已用真实 schema introspection 和真实搜索请求验证过**（直接对 `https://api.nexusmods.com/v2/graphql` 发 `{ __type(name: "...") { ... } }` 内省查询确认的）：`mods` 根字段本身不接受 `gameDomain` 参数，游戏域名和关键词都要放进 `filter`（`ModsFilter`）变量里——`gameDomainName: [{value: "stardewvalley", op: EQUALS}]` 限定游戏，`name: [{value: <关键词>, op: WILDCARD}]` 做子串搜索（`WILDCARD` 不需要在值里手动加 `*`，会自动做子串匹配，比如关键词 `tractor` 能匹配到标题里包含 `Tractor` 的所有 Mod）。早期实现猜测过 `mods(gameDomain: ..., filter: ..., count: ...)` 这种顶层参数形式，被 Nexus 用 GraphQL `errors` 数组拒绝（`argumentNotAccepted`），导致每次关键词搜索都失败并兜底成 `nexus_request_failed`，已修复。
  - 结果数量上限 `nexusMaxResults = 20`。
  - `WILDCARD` 子串匹配只对得上 Mod 标题里实际出现的字符串：纯中文关键词（比如"拖拉机"）如果没有任何 Mod 标题恰好包含这几个汉字，会合法地返回 0 条结果（不是错误），因为 Nexus 上的 Mod 标题绝大多数是英文/其他语言；建议优先用英文关键词（如 `tractor`）或者数字 ID 搜索。
- **请求安全**：固定 10 秒超时（`nexusRequestTimeout`）、固定 `User-Agent`；非 2xx 响应一律不读取/转发响应体，只保留状态码包成 `*NexusAPIError`，避免上游错误页（可能回显请求细节）泄露给前端；API Key 只通过请求头发送，从不出现在 URL 或错误信息里，未配置时也不会发送空请求头。
- **query 校验**：空查询（trim 后）现在作为默认热门列表处理，返回 Nexus Stardew Valley 列表第一页；`ErrInvalidNexusQuery` 仅保留给安装/下载等缺少有效 Mod ID 的请求使用。
- **已安装匹配**：`ApplyNexusInstalledMatch(dataDir, results)` 读取本地已装 Mod，按 manifest `UpdateKeys` 中 `Nexus:<id>` 解析出的 `NexusModID` 做匹配，命中则把 `installed`/`installedFolderName`/`installedVersion` 填上；本阶段只判断"已安装"，不做版本新旧比较。
- **manifest 解析扩展**：`modManifest`/`registry.ModInfo` 新增 `UpdateKeys []string` 和 `NexusModID int`（由 `parseNexusModIDFromUpdateKeys` 从 `UpdateKeys` 里挑出 `Nexus:` 前缀的条目解析），`readModInfo` 在解析时一并填充，所以 `GET .../mods` 现有列表也会带上这两个新字段（向后兼容，新字段都是 `omitempty`）。

API：`GET /api/instances/:id/mods/nexus/search?q=关键词`，`requireAuth`（任意登录用户，普通玩家也能用，不需要管理员权限）。错误码映射：

| 场景 | HTTP | code |
| --- | --- | --- |
| 空查询 | 200 | 默认 Nexus 热门列表 |
| Nexus 下载/安装但面板未配置 Nexus API Key | 503 | `nexus_api_key_missing` |
| 关键词搜索被 Nexus 拒绝（需要 OAuth/更高权限） | 502 | `nexus_auth_required` |
| Nexus 返回 404（按 ID 查询未命中） | 404 | `nexus_mod_not_found` |
| Nexus 返回 401/403（ID 查询路径，Key 无效/权限不足） | 502 | `nexus_unauthorized` |
| Nexus 返回 429 | 429 | `nexus_rate_limited` |
| 其他非 2xx / 网络错误 | 502 | `nexus_request_failed` |

注意 `nexus_unauthorized` 只会从 ID 查询路径（v1 REST 直接返回 401/403）触发；关键词路径的 401/403 在 `nexusSearchByKeyword` 内部就已经转换成 `ErrNexusAuthRequired`，到 handler 时已经不是 `*NexusAPIError` 了，所以两个 code 不会互相覆盖。

### Nexus API Key 面板设置

管理员通过面板配置 Nexus Key，不再依赖环境变量：

- `GET /api/settings/nexus`：管理员专用，返回 `{ "configured": boolean, "last4"?: string }`，只暴露末 4 位。
- `PUT /api/settings/nexus/api-key`：管理员专用，请求体 `{ "apiKey": string }`；trim 后写入 SQLite `panel_settings`，立即对后续搜索请求生效，不需要重启。
- `DELETE /api/settings/nexus/api-key`：管理员专用，删除 `panel_settings.nexus_api_key`。

保存和删除都会写审计日志（`nexus_api_key_update` / `nexus_api_key_delete`），日志 metadata 不包含 Key 本体。`SearchNexusMods` 本身不访问存储，HTTP handler 负责读取 `panel_settings` 后以参数传入，避免把面板配置逻辑混进 `stardew_junimo` 客户端。

涉及文件：`backend/internal/games/registry/types.go`（`ModInfo` 新增字段）、`backend/internal/games/stardew_junimo/mods.go`（manifest 解析扩展）、`backend/internal/games/stardew_junimo/nexus.go`（Nexus 客户端）、`backend/internal/web/instance_handlers.go`（实例路由）、`backend/internal/web/lifecycle_handlers.go`（`handleModNexusSearch`/`writeNexusError`）、`backend/internal/web/settings_handlers.go`（Key 配置接口）、`backend/internal/storage/settings.go`（`panel_settings` 读写）。

测试覆盖（`backend/internal/games/stardew_junimo/nexus_test.go`）：无 Key 数字 ID 走 GraphQL v2 `gameId + modId` 且不带 `apikey` 头、关键词路径在无 Key 时仍能正常工作、空 query、ID 查询结果解析、关键词搜索结果解析、结果数量上限裁剪、非 2xx 状态码映射、GraphQL 鉴权失败映射、API Key 不泄露、`UpdateKeys`/`NexusModID` 解析与 `installed` 匹配，以及本地已安装 Mod 元数据补全和 sidecar 缓存。

### 单人菜单暂停

`StardewAnxiPanel.Control` SMAPI mod 支持真人玩家菜单暂停。当前逻辑仍在 Junimo 服务端读取远端 `Farmer.hasMenuOpen || Farmer.requestingTimePause` 来触发背包/菜单暂停；实测服务端能通过该字段看到背包打开。AUTOPAUSE-7 的规则是：只有 1 个真人玩家时，该玩家打开菜单即暂停；有 2 个及以上真人玩家时，必须所有真人玩家都打开菜单才暂停，避免单个玩家翻背包影响其他正在行动的人。时钟控制沿用 AUTOPAUSE-6 参考 `Pause Time in Multiplayer` 的 1.6 分支做法，不再靠 `IsTimePaused/pauseTime` 维持暂停，而是在 `UpdateTicking` 中保存 `Game1.gameTimeInterval` 并写入 `-100` 哨兵冻结时钟；关闭背包后发现哨兵则恢复保存的 interval，同时清理旧版本可能残留的 `Game1.netWorldState.Value.IsTimePaused`、`Game1.netWorldState.Value.IsPaused`、`Game1.isTimePaused` 和 `Game1.pauseTime`。如果暂停 tick 中抛异常，mod 会先释放时钟，避免再次永久卡住。

## 安全要求

- 控制台命令必须 allowlist，普通用户和管理员可见命令不同。
- 存档 ZIP 解压必须防 zip-slip、绝对路径和符号链接。
- Mod 上传必须限制文件类型、大小和运行中危险操作。
- 错误响应不暴露内部路径、堆栈、token、密码。
- 支持包不包含完整数据库、完整存档、完整 Mod 或 Steam session。
- Docker Socket 权限风险必须在部署和 UI 中提示。

## 后端验证

常用命令：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
```

SMAPI mod 编译：

```powershell
docker run --rm `
  -v "E:\stardew-server-anxi-panel\backend\internal\games\stardew_junimo\embedded\smapi-mod-src:/src" `
  -v "E:\stardew-anxi-panel\runtime\game:/game" `
  -w /src mcr.microsoft.com/dotnet/sdk:6.0 `
  dotnet build -c Release /p:GamePath=/game
```

改动后如果影响嵌入 DLL，必须说明是否需要重启 Stardew server 容器。
# REMOTE-MOD-1 NXM / 直链远程安装
- 新增 `POST /api/instances/:id/mods/remote/install`，管理员专用且要求服务器不在 running/starting。请求体 `{ "url": string, "mod"?: Nexus 卡片字段 }`，后端创建 `mod_remote_install` job。
- `url` 支持两类：`nxm://stardewvalley/mods/{modId}/files/{fileId}?key=...&expires=...`，以及 `https://.../*.zip` 远程压缩包直链。NXM 路径会使用 SQLite 中保存的 Nexus API Key + 链接里的 `key/expires` 调 v1 `download_link.json`，用于非 Premium 用户的慢速下载授权；直链路径会直接下载 ZIP，来源可以是 Nexus CDN、ModDrop、GitHub、CurseForge 等公网 HTTPS ZIP。
- 远程下载完成后统一复用现有 `UploadModZip` 安全校验/解压/导入，并设置 restart-required。当前只承诺 ZIP；7z/rar 需要后续引入解压器和同等安全校验后再开放。
- 为避免泄露临时授权参数，Nexus/API/CDN 网络错误不会把完整 URL 写入 job error；审计日志只记录 jobId，不记录粘贴的 URL。
- 新增 `ParseNexusNXMURL`、`InstallNexusModWithTicket`、`InstallRemoteMod`，测试覆盖 NXM 解析、`key/expires` 透传、ZIP 下载导入。
# SMAPI-RUNTIME-1 Mod 列表置顶 SMAPI 运行组件

- `GET /api/instances/:id/mods` 现在会在检测到面板内置控制 Mod `StardewAnxiPanel.Control` 已安装时，在 `mods[]` 第一位注入一个虚拟内置条目：`SMAPI`（`uniqueId=Pathoschild.SMAPI`、`builtIn=true`、`syncKind=client_required`）。
- 该条目表示 Stardew Valley 的 SMAPI 加载器/运行组件，用于提醒玩家客户端也需要先安装 SMAPI；它不是 `.local-container/mods` 下的真实目录，不参与删除、上传覆盖或 ZIP 导出。
- `FindModByUniqueID` 会跳过 `builtIn` 条目，避免把虚拟 SMAPI 当作可删除的磁盘 Mod；玩家同步包导出也会跳过 `builtIn` 条目，避免尝试打包不存在的运行组件。
- `ModInfo` 新增 `builtIn?: boolean` 字段；前端可据此禁用危险操作并显示“内置/置顶/不打包进同步包”等状态。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 覆盖 SMAPI 置顶注入和同步包不导出内置 SMAPI。

# MODORIGIN-1 Nexus 包来源元数据

- `ModInfo` 新增 `originSource/originNexusModId/originModName/originModUrl`，用于表达“这个文件夹本身没有 Nexus UpdateKey，但它是随某个 Nexus 包安装的内容包”。
- `UploadModZip` 在同一个 ZIP 导入出多个 Mod 时，会优先选择带 `Nexus:<id>` 的主 Mod 作为包来源，并通过 `.local-container/control/nexus-mods.json` 保存到所有同包导入的 Mod。典型例子：`Multiple Construction Orders` 自身显示 `nexusModId=47289`，`[CP] Multiple Construction Orders` 显示 `originSource=nexus/originNexusModId=47289/originModName=Multiple Construction Orders`，但自己的 `nexusModId` 仍为空。
- `ApplyNexusMetadataToMods` 不再把 sidecar 里的 `modId` 写回没有 `UpdateKeys` 的内容包 `nexusModId`；它只填 `origin*` 字段，并复用来源包的 `pictureUrl/downloadCount/endorsementCount/updatedAt` 作为卡片展示元数据。
- 后续 GraphQL 补齐主 Mod 缩略图时，`SaveInstalledNexusMetadata` 会合并并同步更新 sidecar 中同一个 Nexus ID 的内容包记录，确保内容包也能显示来源包缩略图。
- `DeleteMod` 现在会把同一个 Nexus 包来源的真实 Mod 文件夹视为捆绑组：删除主 Mod 或 `[CP]` 内容包任意一个，都会一并删除同 `nexusModId/originNexusModId` 的组成员，并清理 sidecar 里的 Nexus 元数据记录。
- 验证：`go test ./internal/games/stardew_junimo` 覆盖 Nexus 外壳 ZIP 内主 Mod + `[CP]` 内容包的来源字段、持久化、缩略图继承和捆绑删除。
# NEXUS-PAGED-1 Nexus 搜索排序与分页

- `GET /api/instances/:id/mods/nexus/search` 继续作为模组页唯一在线搜索入口；前端不再走 `/mods/search` 统一搜索骨架。
- 查询参数新增 `page`、`pageSize`（默认 `1/20`，后端最大页大小 50）。响应 `NexusModSearchResponse` 新增 `page/pageSize/total/hasMore`。
- Nexus GraphQL v2 关键词搜索会把排序下推到 Nexus：`sort: [{ downloads: { direction: DESC } }]`，并传 `offset/count`。这样热门官方 Mod 会在全量匹配结果里按下载量排序，而不是只在默认前 20 条里本地排序。
- 数字 ID 搜索仍保持原分流：有 Nexus API Key 走 v1 REST 精确查询，无 Key 走 GraphQL v2 `gameId + modId` 精确查询。
- 涉及文件：`backend/internal/games/stardew_junimo/nexus.go`、`backend/internal/web/lifecycle_handlers.go`、`backend/internal/games/stardew_junimo/nexus_test.go`。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。
- 统一搜索骨架已移除：`/api/instances/:id/mods/search`、`/api/instances/:id/mods/search/install` 路由不再注册，`mod_search.go` / `mod_search_test.go` 已删除。

# SMAPI-SYNC-2 SMAPI 与面板控制 Mod 同步语义

- `ListMods` 在检测到 `StardewAnxiPanel.Control` 时仍注入虚拟 `SMAPI` 条目；该条目保持 `builtIn=true`、`uniqueId=Pathoschild.SMAPI`、`syncKind=client_required`，现在会进入玩家同步清单，用于提醒玩家客户端必须先安装 SMAPI。
- `StardewAnxiPanel.Control` 真实目录会被识别为 `builtIn=true`、`syncKind=server_only`。后端删除入口会拒绝删除该内置控制 Mod，避免通过 folderName 绕过前端按钮保护。
- 玩家同步 ZIP 导出会跳过 `StardewAnxiPanel.Control`，不管它的持久化分类是否被写成 `client_required`。SMAPI 写入 `pack-manifest.json` 的 `mods[]` 与 `smapi` 元数据；只有服务端已缓存 SMAPI 官方 ZIP 时才会额外随包写入 `payload/smapi/SMAPI*.zip`。
- `pack-manifest.json` 的每个 Mod 条目带 `builtIn` 与 `packaged` 字段：SMAPI 为 `builtIn=true, packaged=false`，普通玩家需同步 Mod 为 `packaged=true`；玩家安装脚本只复制 `packaged=true` 的 Mod。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。

# PLAYERSYNC-PACK-11 ASCII 动态进度条

- 玩家同步包 `tools/install.ps1` 恢复动态进度条，但终端动态行只使用 ASCII：`[====    ]  42% CHECK/MODS/SMAPI/...`。
- `Show-InstallProgress` 继续把中文详细状态写入日志；控制台渲染由 `Render-InstallProgressLine` 根据百分比映射为英文阶段名，不把中文 `$Status` 放进原地刷新行。
- `Write-Step` 打印中文任务日志前会 `Clear-InstallProgressLine`，打印后再 `Redraw-InstallProgressLine`；调用 Steam 启动项辅助脚本、人工粘贴游戏目录、失败恢复提示前也会先清理进度行，避免中文提示和动态条粘在同一行。
- 仍禁用 PowerShell `Write-Progress`，避免 Windows Terminal 进度流重绘造成中文双字。当前测试解压包已热修，并在真实游戏目录验证普通中文任务行不再重叠。
# PLAYERSYNC-PACK-12 日志写入非致命化

- 玩家同步包 `tools/install.ps1` 新增 `Write-LogLine`，替代安装日志里的直接 `Add-Content`。
- 日志写入会短重试 5 次；如果日志文件被 Windows Terminal、杀软或其他短暂句柄占用，最终失败也只跳过该条日志，不再中断安装。
- 修复真实安装时 `install-*.log` 被占用导致 SMAPI 阶段失败的问题；安装主流程不再依赖日志文件可写。
- 当前测试解压包已热修，并在 `D:\steam\steamapps\common\Stardew Valley` 真实安装验证通过。
# PLAYERSYNC-PACK-13 启动项高亮

- 玩家同步包终端输出新增颜色提示：`Steam 启动项文本` / `建议 Steam 启动项` 标题用 Yellow，真正需要复制的启动项整行用 Cyan。
- 启动项文本本身仍单独占一行，只包含 `"<gameDir>\StardewModdingAPI.exe" %command%`，方便玩家直接选中复制。
- 当前测试解压包的 `tools/install.ps1` 与 `tools/steam-launch-options.ps1` 已热修，并真实安装验证输出流程正常。
# PLAYERSYNC-PACK-14 启动项复制提示

- 玩家同步包终端输出在 `Steam 启动项文本：` 和 `建议 Steam 启动项：` 后新增提示：`请复制到 Steam 的游戏启动项中。`

# PLAYERSYNC-PACK-15 模组更新包

- 新增轻量模组更新包导出：`POST /api/instances/:id/mods/sync-pack/export-update`，下载文件名为 `stardew-player-mods-update-pack.zip`。
- 新增 `ExportModSyncUpdatePackZip(dataDir)`，复用玩家同步包导出核心，但 `pack-manifest.json.packType=mods_update`，不写入 `payload/smapi/`、不写入 SMAPI ZIP，也不在 `checksums.sha256` 记录 SMAPI。
- 更新包只允许存在真实可打包的 `client_required` Mod 时导出；只有虚拟 SMAPI 前置项时返回 `400 no_sync_mods`。
- 更新包根目录使用 `安装模组更新.bat`、`卸载本次模组更新.bat` 和专属 README。安装脚本识别 `mods_update` 后会先确认 `<Stardew Valley>\StardewModdingAPI.exe` 已存在；缺失时停止并提示先运行完整版玩家同步包或手动安装 SMAPI。
- 完整同步包接口不变：`POST /api/instances/:id/mods/sync-pack/export` 仍返回 `stardew-player-sync-pack.zip`，继续按服务端缓存情况随包携带 SMAPI。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。
- 提示行使用 Yellow；真正可复制的 launch options 仍保持独立 Cyan 行，避免玩家复制到说明文字。
- 当前测试解压包的 `tools/install.ps1` 与 `tools/steam-launch-options.ps1` 已热修，并真实安装验证输出顺序正常。

# PLAYERSYNC-PACK-16 模组更新包跳过 Steam 启动项

- `packType=mods_update` 的玩家端安装脚本不再调用 `tools/steam-launch-options.ps1`，更新包 ZIP 也不再包含该辅助脚本；因此不会尝试读取或写入 Steam `localconfig.vdf`。
- 更新包仍会检查 `<Stardew Valley>\StardewModdingAPI.exe` 是否存在；存在后只安装/跳过/更新 Mod，Steam 启动项沿用完整版同步包或玩家已有设置。
- 更新包最终摘要显示 `Steam 启动项：已跳过，沿用已有设置`，不再打印可复制启动项文本，避免玩家误以为本次更新包还需要配置 Steam。
- 完整同步包行为不变：首次玩家使用 `POST /api/instances/:id/mods/sync-pack/export` 导出的完整包，仍会尽力配置 Steam 启动项或输出可复制文本。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。

# MODPROFILE-1 按存档启用/禁用 Mod

- 新增 `.local-container/mods-disabled/` 作为禁用 Mod 的物理目录；`.local-container/mods/` 仍是容器启动时挂载到 `/data/Mods` 的启用目录。
- 新增 `.local-container/control/mod-profiles.json`，按 `saveName` 保存每个存档的 Mod 启用状态。新建/新导入存档的 profile 使用 `defaultEnabled=false`，即除内置组件外默认不启用任何第三方 Mod。
- `GET /api/instances/:id/mods` 现在通过 `ListModsWithState(dataDir, activeSaveName)` 返回启用与禁用目录的合并列表，并在 `ModInfo` 上返回 `enabled/canToggle/enableNote`。
- 新增 `PUT /api/instances/:id/mods/:modId/enabled`，管理员专用且要求服务器不在 running/starting。请求体 `{ "enabled": boolean, "saveName"?: string }`；未传 `saveName` 时使用当前激活存档。
- 启动链路在 `docker compose up` 前应用当前存档 profile；新建存档启动前先执行 `ApplyNewSaveDefaultModState`，把非内置 Mod 全部移动到禁用目录，待 Junimo 写出真实存档名后再持久化 disabled profile。
- 上传存档确认并启动时会立即为该导入存档写入默认禁用 profile 并应用，避免新接入面板的存档继承服务器已有 Mod。
- 删除与重复 UniqueID 检查改为同时扫描启用与禁用目录，避免禁用后无法删除或重复上传误判。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。
# MODPROFILE-2 切换存档刷新 Mod 状态

- 后端原有 `handleSaveSelect` / `handleSaveSelectAndStart` 已在 `SetActiveSave` 后调用 `ApplyModProfile`；本次补充测试确保两个存档 profile 之间切换时，真实 Mod 文件夹会在 `.local-container/mods/` 与 `.local-container/mods-disabled/` 间移动到正确位置。
- 新增测试覆盖：`TestApplyModProfileSwitchesPhysicalStateBetweenSaves`，验证 SaveA 启用 ModA/禁用 ModB、SaveB 禁用 ModA/启用 ModB 的来回切换。
- 涉及文件：`backend/internal/games/stardew_junimo/mod_profiles_test.go`。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。

# NEXUS-DEFAULT-1 下载页默认热门 Mod

- `GET /api/instances/:id/mods/nexus/search` 现在允许 `q` 为空；空查询不再返回 `400 invalid_query`，而是调用 Nexus GraphQL v2 的 Stardew Valley 列表查询。
- 默认列表只带 `gameDomainName=stardewvalley` 过滤，并按 `downloads DESC` 排序，继续使用 `page/pageSize/total/hasMore` 分页；前端默认请求第一页 20 条。
- 关键词搜索和数字 ID 搜索路径不变：关键词继续带 `name WILDCARD`，数字 ID 有 Key 时走 v1 REST，无 Key 时走 GraphQL 精确 ID。
- 涉及文件：`backend/internal/games/stardew_junimo/nexus.go`、`backend/internal/web/lifecycle_handlers.go`、`backend/internal/games/stardew_junimo/nexus_test.go`。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。
# NEWGAME-CABINS-1 自定义新存档初始联机小屋

- `NewGameConfig.startingCabins` 后端契约从 `0-3` 对齐到 Stardew 1.6/控制模组使用的 `0-7`；`WriteServerSettings` 继续写入 Junimo `Game.StartingCabins` 和 `Game.CabinLayoutNearby`。
- `WriteInitConfig` 现在始终为自定义新建存档写 `.local-container/control/server-init.json`，即使请求没有角色外观字段，也会把 `cabinCount/cabinLayout` 交给 `StardewAnxiPanel.Control`。
- `WriteServerSettings` 会写 `.local-container/control/new-game-pending` 一次性标记；选择既有存档时 `SetActiveSave` 会清除该标记。
- SMAPI 控制模组不再执行 `Game1.game1.loadForNewGame(false)` 这条历史原生创建路径；真实创建统一由 Junimo HTTP `POST /newgame` 完成。
- 控制模组仅当 `new-game-pending` 存在时提前同步 `Game1.startingCabins`、`Game1.cabinsSeparate`、农场类型、利润率和钱包模式；存档创建/加载后会删除标记，避免后续启动既有存档时误触发新建参数。
- `server-init.json.mode` 新写为 `panel-newgame`；控制模组只为兼容旧文件保留读取旧 `native-create` 字符串，不再包含对应的创建执行逻辑。
- 已重编嵌入 DLL：`backend/internal/games/stardew_junimo/embedded/smapi-mod/StardewAnxiPanel.Control.dll`。影响已运行实例时，需要重启/重新准备服务端容器让内置控制模组刷新到实例目录。
- 验证：`go test ./internal/games/stardew_junimo -run "WriteServerSettings|WriteInitConfig|ValidateNewGameConfig"`、`go test ./internal/games/stardew_junimo ./internal/web` 通过；SMAPI mod 通过 Docker + `/p:GamePath=/game` 编译通过。


# SAVE-BACKUP-POLICY-1 ????

- ??????????????? `.local-container/backups/saves/policy.json`???????????????????`dailyRetentionDays=3`??? 14 ????????????????? 24 ????? 168 ???
- ??/?????`POST /api/instances/:id/saves/:name/backup` ?????`GET|PUT /api/instances/:id/saves/backups/policy` ??/?????`GET /api/instances/:id/saves/backups` ??? `policy` ? `maintenance`??????????/?????
- ??????????????????????????? `latest_<save>.zip`??????? `scheduled_<save>.zip`?????? `daily_<save>_<YYYYMMDD>.zip`??????????????????
- SMAPI Control ? `GameLoop.Saved` ?? `.local-container/control/save-events/*.json`???????????? latest/daily ??????????
- ???`go test ./internal/games/stardew_junimo ./internal/web` ???SMAPI Control DLL ??????? embedded mod?

# SAVE-BACKUP-SCHEDULE-HOUR-1 定时备份整点设置

- `BackupPolicy` 将定时备份从 `scheduledIntervalHours` 改为 `scheduledHour`，合法范围为 0-23，默认 4，表示每天 04:00 后触发一次。
- `scheduledIntervalHours` 仅作为旧配置读取兼容字段保留，写入策略时会清零并通过 `omitempty` 不再落盘。
- `RunBackupMaintenance` 的定时备份判断改为本地时间每日一次：未到 `scheduledHour` 不执行；当天已经执行过不再重复执行；下一天到点后覆盖同一份 `scheduled_<save>.zip`。
- 新增测试 `TestScheduledBackupRunsOncePerDayAtConfiguredHour` 覆盖未到点、当天重复、次日再次执行。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。
# SCHEDULED-RESTART-1 计划重启后端

- 新增 `restart_schedules` SQLite 表（迁移 `006_restart_schedules.sql`），按 `instance_id` 保存每日维护窗口：`shutdown_time`、`startup_time`、`timezone`、`warning_minutes_json`、`backup_before_shutdown`、`skip_if_players_online`，以及 `last_shutdown_at/last_startup_at/last_status/last_message`。
- 新增接口：`GET /api/instances/:id/restart-schedule` 读取配置；`PUT /api/instances/:id/restart-schedule` 仅管理员可更新配置。响应统一为 `{ schedule }`，并返回 `nextShutdownAt/nextStartupAt` 供前端展示。
- `cmd/panel` 启动 `web.RestartScheduler` 后台轮询。调度器只做编排：到关闭时间前按配置调用现有 `SendSay` 写控制命令；到关闭窗口时可先调用 `BackupSave` 备份当前激活存档，再复用 driver `Stop` 提交生命周期 job；到开启时间附近复用 driver `Start` 提交启动 job。
- 关闭前备份只打包已经落盘的 active save，不强制保存游戏内实时进度。备份失败会记录 `backup_failed` 并阻止本次关闭。
- 计划开启只在到点后 5 分钟宽限窗口内触发，避免面板首次启动时补跑很久以前的开启时间；计划关闭可在维护窗口内补跑一次。状态通过 `last_*_at` 防止 30 秒轮询重复提交。
- 验证：`cd backend; go test ./internal/storage ./internal/web ./cmd/panel`。
# MODDEPS-2 依赖状态与按存档安装匹配

- `GET /api/instances/:id/mods` 现在会在每个 `dependencies[]` 项上补齐状态字段：`installed/enabled/installedVersion/satisfied/status`。状态覆盖必需依赖缺失、依赖已安装但当前存档未启用、最低版本不满足、版本无法确认，以及可选依赖缺失等场景。
- 依赖判断在 `stardew_junimo.ListModsWithState(dataDir, activeSaveName)` 内完成，基于合并后的 `mods` 与 `mods-disabled` 物理列表，并叠加当前存档 profile 的 enabled 状态；不绕过 Junimo driver，也不在 web handler 堆 Stardew 逻辑。
- `GET /api/instances/:id/mods/nexus/search` 的已安装匹配改为调用当前存档维度的 `ApplyNexusInstalledMatch(dataDir, activeSaveName, results)`，因此当前存档禁用的 Mod 仍会显示 `installed=true`，同时返回 `installedEnabled=false`。
- 版本比较采用保守的数字段比较，例如 `1.10` 大于 `1.9`；无法解析的版本会返回 `unknown_version`，供前端提示人工确认。本阶段只做检测和展示，不自动安装依赖，也不阻止启用。
- 验证：`cd backend; go test ./...`。

# MODREL-1 Mod 依赖与来源包联动

- 新增 `mod_relationships.go`，统一计算已安装 Mod 的必需依赖图、反向依赖图和 Nexus 来源包 bundle。来源包只使用已有 `nexusModId/originNexusModId` sidecar 信息，不靠 `[CP]` 文件夹名前缀猜测。
- `PUT /mods/:modId/sync-classification` 改为调用 `SetModSyncClassificationCascade` 并返回 `{ mods, syncKind }`。同步分类按已安装必需依赖连通组一起变：同包成员、前置依赖、前置的前置以及依赖它的下游都会跟随当前选择，避免先点“待确认”后再切回其它标签时下游停留在旧状态。
- `PUT /mods/:modId/enabled` 改为调用 `SetModEnabledForSaveCascade` 并返回 `{ mods, enabled, saveName }`。启用时会一起启用同 Nexus 包成员和必需前置；禁用时会一起禁用同包成员和依赖它的下游，但不会禁用共享前置，例如禁用 Multiple Construction Orders 包不会禁用 Content Patcher。
- 验证覆盖：同步分类的 `Content -> Core -> Framework` 依赖链方向；MCO 主 Mod 与 `[CP]` 内容包同包启停，同时 Content Patcher 作为共享前置保持独立。
- 验证：`cd backend; go test ./...`；`cd frontend; npm.cmd run build`。
# NEXUS-EXT-2 远程安装任务日志中文修复

- `mod_remote_install` 与 `mod_nexus_install` 的 job 进度日志写入点已修复为正常 UTF-8 中文：`准备从远程链接安装 Mod`、`准备安装 Nexus Mod #...`、`已导入：...`。
- 安装或上传 Mod 成功后，如果当前实例有激活存档，后端会把本次导入的 Mod 写入该存档 profile 并标记为启用，避免后续 profile 应用把刚安装的 Mod 移到 `mods-disabled/`。
- 这次只修任务日志里用户直接看到的安装阶段文案；历史已经写入数据库的乱码日志不会被回写修复。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`。
# NEXUS-REQ-1 搜索结果前置依赖

- `GET /api/instances/:id/mods/nexus/search` 的 GraphQL 查询新增读取 `mods.nodes[].modRequirements.nexusRequirements`，搜索结果可返回 `requiredMods[]`。
- `requiredMods[]` 每项结构为 `{ modId, name, notes?, nexusUrl, installed, installedEnabled, installedFolderName?, installedVersion? }`。来源是 Nexus 页面声明的 Nexus 前置 Mod，不是压缩包内 `manifest.json` 的 SMAPI UniqueID。
- `ApplyNexusInstalledMatch(dataDir, saveName, results)` 现在会同时给搜索结果本身和 `requiredMods[]` 标记本地安装/当前存档启用状态，判断仍基于已安装 Mod 的 `UpdateKeys: ["Nexus:<id>"]`。
- GraphQL 只拉取前 10 个 Nexus 前置；外部依赖和自引用会被过滤。已安装后的精确 SMAPI 依赖状态仍由 `GET /mods` 的 `dependencies[]` 负责兜底。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。
# REMOTE-MOD-DOWNLOAD-1 远程 ZIP 大包下载超时

- `stardew_junimo.nexusDownloadArchive()` 不再复用 Nexus 搜索/API 的 10 秒 `nexusHTTPClient`，改用专门的 `nexusArchiveHTTPClient`，ZIP body 读取 timeout 放宽到 15 分钟。
- 触发场景：Ridgeside Village 等大体积 Nexus CDN ZIP 在免费慢速下载链路下，10 秒内无法读完整个 body，旧实现会在 `io.Copy` 阶段报 `context deadline exceeded (Client.Timeout or context cancellation while reading body)`。
- 搜索、GraphQL、v1 REST 等接口仍使用 10 秒短超时；只有实际 ZIP 下载走长超时，并继续受 `maxModZipBytes` 200 MB 限制和 job 30 分钟总 timeout 约束。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web` 通过。
# NEXUS-EXT-BATCH-2 Nexus 来源纠偏

- `UploadModZip` 拆出内部 `uploadModZip(..., uploadModZipOptions)`；普通上传继续推断 Nexus 包来源，`mod_nexus_install` 与 `mod_remote_install` 这类已经携带显式 Nexus 上下文的安装不再先写推断来源，避免临时 ZIP 安装过程中产生错误 sidecar。
- `SaveInstalledNexusMetadata` 会在多 Mod 包中检查导入 Mod 自己的正数 `Nexus:<id>`。如果浏览器扩展批量上下文传来的 `result.modId` 与包内唯一正数 Nexus ID 冲突，会以包内声明为准纠偏，并清除旧的其它 Nexus ID 缓存字段后再写入。
- 修复场景：Ridgeside Village 包内 `[CP] Ridgeside Village` 声明 `Nexus:7286`，其它组件为 `Nexus:-1`；即使批量流程误把 result 带成 SpaceCore `1348`，三个 Ridgeside 组件也会归到 Ridgeside CP 组件，而不是显示“随 SpaceCore 安装”。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。

# NEXUS-EXT-PACK-1 浏览器扩展下载包

- 新增 `GET /api/instances/:id/mods/nexus/extension/download`，任意已登录用户可下载面板打包好的 Nexus 普通用户浏览器扩展 ZIP。
- 后端下载接口采用折中策略：优先复用实例目录 `.local-container/browser-extensions/anxi-nexus-installer.zip` 中已有且合法的预打包 ZIP；如果文件不存在，会优先复制镜像/仓库中的预打包 `browser-extensions/anxi-nexus-installer.zip`；如果预包也不存在或损坏，才从 `browser-extensions/nexus-slow-installer` 源码重新生成。
- ZIP 根目录直接包含 `manifest.json`、`background.js`、`content.js` 等扩展文件，并额外写入 `安装说明.txt`。玩家解压后选择该解压目录即可加载扩展，不需要再进入内层文件夹。
- Docker 镜像运行层会复制 `browser-extensions/` 到 `/app/browser-extensions/`，并在构建时生成 `/app/browser-extensions/anxi-nexus-installer.zip`；正式部署优先使用这个预打包 ZIP。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。
# NEWGAME-PLAYERLIMIT-1 自定义新存档联机人数上限

- `NewGameConfig` 新增 `maxPlayers`，表示最大同时在线人数，合法范围 `1-100`；未传或为 `0` 时保持兼容并在写配置前默认成 `10`。
- `WriteServerSettings` 现在把人数上限写入 Junimo 官方 `server-settings.json` 的 `Server.MaxPlayers`，并显式写入 `Server.CabinStrategy="CabinStack"` 与 `Server.ExistingCabinBehavior="KeepExisting"`，继续让 Junimo 的自动小屋管理处理超过原版初始小屋上限的玩家加入。
- `startingCabins` 仍表示新建存档时地图上初始联机小屋数量，范围保持 `0-7`；`maxPlayers` 不能小于 `startingCabins + 1`，避免小屋数和总人数上限矛盾。
- 涉及文件：`backend/internal/games/registry/types.go`、`backend/internal/games/stardew_junimo/saves.go`、`backend/internal/games/stardew_junimo/saves_test.go`。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "WriteServerSettings|ValidateNewGameConfig"` 通过；`cd frontend; npm.cmd run build` 通过。
# PERF-REVIEW-1 存档与 Nexus 元数据轻量优化

- 存档信息解析的 `whichFarm` 兜底读取改为流式扫描：`readWhichFarmFromMainFile` 不再 `os.ReadFile` 整个主存档 XML，备份 ZIP 元数据解析也不再为了文件大小提前读取主存档 entry。
- `enrichBackupInfo` 只通过 ZIP header 读取主存档未压缩大小；只有 `SaveGameInfo` 需要农场类型兜底时才打开主存档 entry 流式扫描 `<whichFarm>`。
- Nexus 已安装元数据补全把“同 Nexus modId 是否已有展示元数据”的判断预先构建为 map，避免每个 Mod 都遍历 sidecar store。
- 接口契约不变，主要收益是降低大存档/多 Mod 列表下的内存峰值和重复扫描。
- 验证：`cd backend; go test ./...`。
# VNC-CONTROL-1 服务器页 VNC 显示代理

- 新增 `GET/POST /api/instances/:id/rendering`，管理员专用且要求实例为 `running`。`GET` 返回当前 Junimo 服务端渲染 `{ "fps": number }`，用于前端刷新后恢复 VNC 按钮真实状态；`POST` 请求体 `{ "fps": number }`，当前前端用 `15` 打开 VNC 显示、`0` 关闭。
- Web 层只做鉴权、状态校准和路由，实际调用在 `stardew_junimo.GetRenderingFPS()` / `SetRenderingFPS()` 内完成：通过 `docker compose exec server curl http://localhost:<API_PORT>/rendering` 访问 JunimoServer REST API；POST 空 body 需要显式 `Content-Length: 0`。
- `API_PORT` / `API_KEY` 从实例 `.env` 读取，后端在容器内注入 `Authorization: Bearer ...`；浏览器前端不会接触 Junimo API key。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run Rendering`、`cd backend; go test ./internal/web -run "Rendering|VNCConfig"`。

# STEAM-QR-PHASE-CLASSIFY-1 Steam QR 阶段识别修正

- 修复扫码登录选择后，steam-auth 日志中的 `Choice [1]: 2` 被后端误识别为 Steam Guard 手机 App 批准的问题。该行只是上游菜单的默认值提示与用户选择回显，不能作为 `steam_guard_mobile_required` 的依据。
- `installer.go` 现在只把明确的 `waiting for approval`、`open steam app`、`approve in steam mobile` 文案识别为手机批准；QR 模式下不再让泛化的 `steam guard/two-factor/authenticator` 文案覆盖 `steam_qr_required`。
- 影响文件：`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver_test.go`。接口不变，仍通过 `POST /api/instances/:id/steam-guard/input` 发送菜单选择或验证码。
- 现场诊断：同一 `anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2` 镜像在 `stardew_default` 网络内可解析 Steam 域名，TCP `api.steampowered.com:443` 与 `cm1-ord1.cm.steampowered.com:27017` 可连通，TLS 验证正常；本轮问题不是 Docker 容器完全断网，而是 QR 认证阶段 SteamClient 会话连接未建立成功。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamAuthMenus|SteamGuardCodePrompt|QRCodeChoice|SteamMobileApproval"`；`cd backend; go test ./internal/games/stardew_junimo`。
# BE-STEAM-POST-AUTH-RETRY-1 Steam 认证成功后的失败不再归类为认证失败
- 修复 steam-auth 已经登录成功后，游戏下载或后续安装步骤失败仍把实例写成 `steam_auth_failed` 的问题。`runSteamAuthAttempt()` 现在在 `authSucceeded=true` 后遇到容器错误会写 `state=error, driverPhase=post_auth_failed`；遇到 `Download failed:` / `Game download failed` 会写 `state=error, driverPhase=download_failed`。
- 这样前端可以明确区分“账号密码/Steam Guard 错误”和“认证后的 CDN/磁盘/后续安装失败”，后者重试时复用 `.env` 中已保存的 Steam 凭据，不要求用户重新输入账号密码。
- 影响文件：`backend/internal/games/stardew_junimo/installer.go`、`backend/internal/games/stardew_junimo/driver_test.go`。未改变 `POST /api/instances/:id/install` 请求体，继续使用已有 `reuseCredentials=true` 重试契约。
- 验证：`cd backend; go test ./internal/games/stardew_junimo -run "DownloadFailedAfterSuccessfulAuth|InstallMarksSteamAuthFailedWhenRunErrors"` 通过；同时保留原 steam-auth 容器启动失败仍写 `steam_auth_failed` 的覆盖测试。
# STEAMCMD-PULL-PROGRESS-1 镜像拉取进度估算

- 镜像拉取进度会写入隐藏 job 日志 `[pull:progress:done:total]`。Junimo compose pull 按服务镜像完成数估算；SteamCMD 单镜像 pull 按 Docker layer 的 `Pulling fs layer` / `Pull complete` / `Already exists` 估算。
- `pull_running` 和 `steamcmd_image_pulling` 阶段会同步更新实例 `stateMessage`，文案包含“约 N%”，让安装页顶部和镜像拉取卡不再停在纯等待状态。
# JUNIMO-IMAGE-CANDIDATES-1 Junimo/server 与 steam-auth-cn 镜像候选兜底

- 安装阶段不再对 `server` / `steam-auth` 走单点 `docker compose pull`；后端会分别按候选镜像列表执行 `ImageInspect`，本地已有任意候选即直接使用，全部缺失时才逐个 `docker pull`。
- 新增 `.env` 键：`SERVER_IMAGE`、`SERVER_IMAGE_CANDIDATES`、`STEAM_SERVICE_IMAGE_CANDIDATES`。拉取成功或命中本地镜像后，后端会把实际使用的镜像写回 `SERVER_IMAGE` / `STEAM_SERVICE_IMAGE`，`docker-compose.yml` 通过这些变量启动。
- 默认候选顺序：server 为 `docker.1ms.run/sdvd/server:<tag>`、`docker.m.daocloud.io/sdvd/server:<tag>`、`ghcr.io/sdvd/server:<tag>`、`sdvd/server:<tag>`；steam-auth-cn 为 `docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2`、`docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`。
- 旧实例安装时会把 `server` compose 镜像行从 `sdvd/server:${IMAGE_VERSION:-...}` 迁移为 `${SERVER_IMAGE:-sdvd/server:1.5.0-preview.121}`；`IMAGE_VERSION` 仍保留用于版本选择和默认候选生成。
- 拉取进度继续通过 `[pull:progress:done:total]` 隐藏日志给前端估算百分比；单个候选失败会记录 warning 并继续下一个候选，全部失败才进入 `pull_failed`。
- 影响文件：`backend/internal/games/stardew_junimo/installer.go`、`compose_template.go`、`driver.go`、`config/env.go` 及对应测试。
- 验证：`cd backend; go test ./internal/games/stardew_junimo/config`；`cd backend; go test ./internal/games/stardew_junimo -run "Prepare|Migrate.*ComposeImage|SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallMarksSteamAuthFailed|SteamAuthMenus|SteamGuard|QRCode"`。
# JUNIMO-IMAGE-CANDIDATES-2 老实例候选源自动补齐

- 修复旧实例 `.env` 中 `SERVER_IMAGE_CANDIDATES` 或 `STEAM_SERVICE_IMAGE_CANDIDATES` 只有单个旧值时，安装流程只显示/尝试 `(1/1)` 的问题。
- `stardew_junimo` 现在会始终把默认候选源排在前面，再追加实例 `.env` 中已有候选和当前主镜像值并去重；server 默认顺序为 `docker.1ms.run/sdvd/server:<IMAGE_VERSION>`、`docker.m.daocloud.io/sdvd/server:<IMAGE_VERSION>`、`ghcr.io/sdvd/server:<IMAGE_VERSION>`、`sdvd/server:<IMAGE_VERSION>`。
- steam-auth cn 版同样补齐：`docker.1ms.run/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com/anxi-panel/junimo-steam-service-cn:1.5.0-anxi.2`、`docker.m.daocloud.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`ghcr.io/anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`、`anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`。
- 安装流程写入 `.env` 时会同步写回补齐后的候选列表，并在镜像命中/拉取成功后写回实际使用的 `SERVER_IMAGE` / `STEAM_SERVICE_IMAGE`。
