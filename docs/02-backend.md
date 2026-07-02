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

- 玩家同步包 `tools/install.ps1` 不再尝试调用 SMAPI 4.5.2 的交互式 `SMAPI.Installer.exe` / `install on Windows.bat`，也不再传入无效的 `--install --game-path --no-prompt` 参数。
- 真实 Windows 测试确认 SMAPI 4.5.2 安装器没有可用静默参数，非交互调用会进入交互流程并可能因为 Console 句柄失败；失败安装还可能把安装器运行时 DLL 散落到游戏根目录。
- 新流程改为解压随包 SMAPI ZIP 后定位 `internal/windows/install.dat`，复制为临时 `smapi-install-payload.zip`，再用 `Expand-Archive -Force` 释放到 Stardew Valley 游戏目录。该 payload 是官方安装器随包携带的 Windows 安装内容，包含 `StardewModdingAPI.exe`、`smapi-internal/`、`steam_appid.txt` 以及 SMAPI 自带 Mod。
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

# MODSEARCH-3 下载量排序
- `SearchMods` 返回 `ModSearchResult` 前会按 `downloadCount` 从高到低稳定排序；下载量相同的结果保持 provider 原始顺序。当前 live provider 只有 Nexus，后续接入 StardewModDataset/CurseForge/ModDrop/GitHub 时也应复用这条统一排序规则。
- 新增 `TestSearchModsSortsResultsByDownloadCountDescending` 覆盖搜索结果排序，避免前端卡片顺序回退为上游默认顺序。

# MODZIP-1 XNB 替换包提示

- `UploadModZip` 仍只安装标准 SMAPI Mod（顶层 Mod 目录需要 `manifest.json`）。Nexus 上的老式 XNB 替换包虽然也是 `.zip`，但只包含 `Characters/*.xnb`、`Portraits/*.xnb` 等游戏内容替换文件，不能放进服务器 `Mods` 目录。
- 新增 XNB 替换包识别：如果 ZIP 内没有任何 SMAPI `manifest.json`，但包含 `.xnb` 且路径像 `Characters/`、`Portraits/`、`Content/`、`Maps/`、`TileSheets/`，后端返回明确错误：这是 XNB 替换包，不是 SMAPI Mod，不能上传到服务器 Mods 目录。
- Web 层 `sanitizeError` 对 XNB/SMAPI/manifest 这类产品级错误保留友好提示，不再统一压成“压缩包格式错误或已损坏”。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 覆盖 XNB 替换包识别和错误脱敏。

# MODSEARCH-1 统一搜索与统一安装骨架

- 新增 `backend/internal/games/stardew_junimo/mod_search.go`，定义页面级统一搜索结果 `ModSearchResult`：包含 `source/sourceName/sourceModId`、`pageUrl/externalLabel`、`installMethod/installLabel/installUrl`、Nexus/CurseForge 站点 ID、已安装匹配字段等。前端不再需要把搜索结果当成 Nexus 专用结构处理。
- 新增 `GET /api/instances/:id/mods/search?q=...`，当前 live provider 先接现有 Nexus GraphQL v2 / REST v1 搜索，并映射为统一模型：N站结果 `source=nexus`、`sourceName=N站`、跳转按钮文案为 `跳转 N站`、安装按钮文案为 `一键安装`、`installMethod=nexus_premium`。
- 新增 `POST /api/instances/:id/mods/search/install`，管理员专用且要求服务器不在 running/starting。接口按 `installMethod` 分发：`nexus_premium` 复用 `InstallNexusMod`；`direct_url` 复用 `InstallModFromDirectURL`；`none/manual` 返回 `400 mod_install_not_supported`。所有下载后的导入仍走 `UploadModZip` 安全校验，不绕过 `stardew_junimo`。
- 该骨架是给 StardewModDataset、CurseForge、ModDrop、GitHub Release、直链 provider 共用的扩展点。后续接入时只需要在 `SearchMods` 中追加 provider 与去重排序，再按优先级填充 `installMethod/installUrl`：CurseForge download-url、ModDrop download-url、GitHub Release/直链、Nexus Premium、Nexus NXM/临时 CDN。
- 相关文件：`backend/internal/games/stardew_junimo/mod_search.go`、`backend/internal/web/lifecycle_handlers.go`、`backend/internal/web/instance_handlers.go`。
- 验证：`go test ./...` 覆盖统一卡片映射，确认 N站来源、跳转文案和一键安装文案不会回退成旧 Nexus 专用结构。

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
- `steam-auth` sidecar 当前默认使用 `anxiyizhi/junimo-steam-service-cn:1.5.0-anxi.2`。
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
