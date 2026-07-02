# PLAYERSYNC-PACK-10 后端接手记录

## 改了什么
- 玩家同步包安装脚本彻底禁用终端进度渲染。
- `install.ps1` 设置 `$ProgressPreference = "SilentlyContinue"`，并移除 `Show-InstallProgress` / `Complete-InstallProgress` 对 `Write-Progress` 的调用。
- 进度 tick 仍写入安装日志；玩家控制台只输出独立任务行和最终摘要，避免中文字符在 Windows Terminal 中被进度重绘打成双字。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 已热修。

## 影响文件/接口
- 接口不变：`POST /api/instances/:id/mods/sync-pack/export`。
- 文件：
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`
  - `docs/02-backend.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo -run "PlayerSync|ExportModSyncPack"`。
- 已执行当前解压包 PowerShell parser 验证，确认 `install.ps1` 语法通过且不含 `Write-Progress`。
- 已在 `D:\steam\steamapps\common\Stardew Valley` 真实运行热修脚本，输出不再出现 `安安装装` / `已已跳跳过过` 这类重叠双字。

## 下一步注意事项
- 不要再在玩家安装终端里使用 carriage-return 自绘进度或 PowerShell `Write-Progress`；中文终端渲染在 bat / Windows Terminal 组合下不稳定。
- 如需给玩家可见进度，建议改做独立 GUI 或只输出离散阶段行；日志文件里已经保留百分比进度。

# PLAYERSYNC-PACK-9 后端接手记录

## 改了什么
- 安装完成摘要移除单独的 `SMAPI 路径` 输出。
- 摘要只保留玩家需要复制到 Steam 的 `Steam 启动项文本`，即完整的 `"<gameDir>\StardewModdingAPI.exe" %command%`。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 已热修。

## 影响文件/接口
- 接口不变：`POST /api/instances/:id/mods/sync-pack/export`。
- 文件：
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`
  - `docs/02-backend.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo -run "PlayerSync|ExportModSyncPack"`。
- 已执行模板提取后的 PowerShell parser 验证：`install.ps1` parse ok。
- 已执行真实安装验证：最终摘要不再显示 `SMAPI 路径`，只显示 `Steam 启动项文本` 和完整可复制 launch options。

## 下一步注意事项
- 用户最终需要的是 Steam 启动项整段文本，不需要单独看到 exe 路径；后续不要再把两者拆开造成复制负担。

# PLAYERSYNC-PACK-8 后端接手记录

## 改了什么
- 回退玩家同步包安装脚本里的自绘 carriage-return 动态进度行，解决真实终端中中文字符重复、行残留、输出粘连的问题。
- `Show-InstallProgress` 后续在 PLAYERSYNC-PACK-10 中已改为只把进度 tick 写入日志；控制台只保留 `Write-Step` 任务日志，一行一条。
- `steam-launch-options.ps1` 的建议启动项改为两行：先输出标题，再输出完整启动项文本。
- 安装完成摘要新增单独的 `Steam 启动项文本`；`Steam 启动项` 行只表达是否已自动设置，不再提示“查看上方”。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 和 `tools\steam-launch-options.ps1` 已热修。

## 影响文件/接口
- 接口不变：`POST /api/instances/:id/mods/sync-pack/export`。
- 文件：
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`
  - `docs/02-backend.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo -run "PlayerSync|ExportModSyncPack"`。
- 已执行模板提取后的 PowerShell parser 验证：`install.ps1` parse ok。
- 已执行真实安装验证：输出为独立任务行，无重复字；最终摘要包含完整 Steam 启动项文本。

## 下一步注意事项
- 不要再用 `\r` / `[char]13` 在控制台重绘中文进度行。不同终端对中文宽度和 bat 捕获处理不一致，容易产生视觉脏输出。
- 玩家终端需要进度时不要再使用 `Write-Progress`；优先用独立 `Write-Step` 阶段行，细粒度百分比保留在日志中。

# PLAYERSYNC-PACK-7 后端接手记录

## 改了什么
- 玩家同步包安装 Mod 前会比较 payload 与目标 Mod 目录的内容指纹。
- 新增 `Get-RelativePath` / `Get-DirectoryFingerprint`。指纹包含目录相对路径、文件相对路径、文件大小和文件 SHA256。
- 如果 `<Stardew Valley>/Mods/<folderName>` 已存在且与 `payload/mods/<folderName>` 指纹完全一致，脚本会跳过备份和复制，打印 `已跳过相同 Mod`，并在 `installed.json.mods[]` 写入 `skippedIdentical=true`。
- 如果任意文件内容、大小或路径不同，脚本仍按原逻辑备份旧目录并复制新目录。因此版本不同、文件变化、同版本热修都会触发更新。
- 全部 Mod 都跳过且没有真实备份时，`installed.json.backupId` 为 `null`，避免指向不存在的备份目录。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 已热修。

## 影响文件/接口
- 接口不变：`POST /api/instances/:id/mods/sync-pack/export`。
- 文件：
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`
  - `docs/02-backend.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo -run "PlayerSync|ExportModSyncPack"`。
- 已执行模板提取后的 PowerShell parser 验证：`install.ps1` parse ok。
- 已执行当前解压包 parser 验证。
- 已用当前游戏目录运行热修后的安装脚本并传入 `-SkipSteamLaunchOptions`，确认 `ContentPatcher`、`PettingAnimation`、`[CP] MultipleConstructionOrders` 均跳过相同 Mod，且没有创建新的 Mod 备份目录。

## 下一步注意事项
- 跳过判断是内容级，不是版本号级；不要改成只比较 manifest 版本。内容级比较能覆盖版本号漏改、同版本重发和版本号变化三种情况。
- 指纹会读取每个 Mod 文件的 SHA256，大型 Mod 数量很多时会增加少量安装前耗时，但能避免无意义备份和覆盖。

# PLAYERSYNC-PACK-6 后端接手记录

## 改了什么
- 玩家同步包安装脚本的文本进度条改为单行动态刷新。
- `Show-InstallProgress` 不再对每个进度 tick 直接 `Write-Host` 新行，而是通过 `Render-InstallProgressLine` 使用 `[Console]::Write([char]13...)` 原地刷新当前行。
- `Write-Step` 会在打印普通安装事件前清除当前进度行，打印完成后再重绘进度行，避免日志文本和进度条挤在同一行。
- `Complete-InstallProgress` 会在 100% 后补换行，保证最后的安装总结从新行开始。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 已热修。

## 影响文件/接口
- 接口不变：`POST /api/instances/:id/mods/sync-pack/export`。
- 文件：
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`
  - `docs/02-backend.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo -run "PlayerSync|ExportModSyncPack"`。
- 已执行模板提取后的 PowerShell parser 验证：`install.ps1` parse ok。
- 已执行当前解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` parser 验证。

## 下一步注意事项
- 进度刷新依赖控制台支持 carriage return；如果极少数宿主不支持，`Render-InstallProgressLine` 会 fallback 到 `Write-Host`，保证功能优先。
- 日志文件仍记录进度 tick，便于排查；本次只改变控制台视觉呈现。

# PLAYERSYNC-PACK-5 后端接手记录

## 改了什么
- 修复玩家同步包在 Windows 真机安装 SMAPI 4.5.2 后仍找不到 `StardewModdingAPI.exe` 的问题。
- 原脚本会在 SMAPI ZIP 内选择看起来像安装器的 exe/bat，并传入 `--install --game-path --no-prompt`。真实测试确认 SMAPI 4.5.2 Windows 安装器没有这组静默参数，非交互调用会进入交互安装流程并可能失败。
- 新脚本改为解压 SMAPI ZIP，定位 `internal/windows/install.dat`，复制成临时 `smapi-install-payload.zip` 后用 `Expand-Archive -Force` 解压到游戏目录。该 `install.dat` 是官方 Windows 安装 payload，包含 `StardewModdingAPI.exe`、`smapi-internal/`、`steam_appid.txt` 和 SMAPI 自带 Mod。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 已热修复。

## 影响文件/接口
- 接口不变：`POST /api/instances/:id/mods/sync-pack/export`。
- 文件：
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`
  - `docs/02-backend.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo -run "PlayerSync|ExportModSyncPack"`。
- 已执行当前解压包 PowerShell parser 验证：`install.ps1` parse ok。
- 已执行 SMAPI payload 流程验证：`install.dat -> smapi-install-payload.zip -> Expand-Archive` 可释放 `StardewModdingAPI.exe`。
- 已在 `D:\steam\steamapps\common\Stardew Valley` 真实运行热修复后的安装脚本，参数为 `-SkipSteamLaunchOptions`，确认 SMAPI、`ContentPatcher`、`PettingAnimation`、`[CP] MultipleConstructionOrders` 安装成功。

## 下一步注意事项
- 不要再尝试静默调用 SMAPI 的交互式 Windows 安装器，除非后续官方明确提供并文档化静默参数。
- 旧失败安装可能已经把 `api-ms-win-*`、`coreclr.dll` 等安装器运行时文件散落到游戏根目录；当前脚本不会自动删除这些历史残留，避免误删玩家文件。需要清理时应按明确清单单独处理。
- 本次真实验证跳过了 Steam 启动项修改；双击安装包正常运行时仍会按原逻辑尽力设置 Steam launch options，Steam 正在运行则只打印手动复制文本。

# PLAYERSYNC-PACK-4 后端接手记录

## 改了什么
- 玩家同步包安装脚本 `tools/install.ps1` 新增阶段进度显示。
- 新增 `Show-InstallProgress`：同时调用 PowerShell `Write-Progress`，并输出文本进度行，避免不同终端下原生进度条不明显。
- 新增 `Complete-InstallProgress`：安装完成时把进度置为 100% 并关闭 PowerShell 原生进度条。
- checksum 阶段按 `checksums.sha256` 文件数推进；Mod 安装阶段按 `packaged=true` 的 Mod 数推进；SMAPI 阶段显示粗粒度的“解压安装包/释放官方安装文件/完成”。
- 已同步热修当前用户解压目录：`C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1`。

## 影响文件/接口
- 接口不变：`POST /api/instances/:id/mods/sync-pack/export`。
- 文件：
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`
  - `docs/02-backend.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 当前解压包 `install.ps1` 已用 PowerShell parser 验证通过。
- 新增 `TestPlayerSyncInstallScriptShowsProgress` 防止模板退回无进度。
- 建议真实安装时观察文本进度行；`Write-Progress` 在 Windows Terminal 顶部显示，文本行会进入同一日志。

## 下一步注意事项
- SMAPI payload 释放阶段仍只有粗粒度进度；若未来要更细进度，需要按 ZIP entry 逐项释放并自行计算文件级进度。

# PLAYERSYNC-PACK-3 后端接手记录

## 改了什么
- 修复玩家端安装/卸载脚本对带方括号 Mod 文件夹的路径处理。
- 真实 Windows 测试里，`payload/mods/[CP] MultipleConstructionOrders/Assets/ConstructionWorker.png` 会被 PowerShell `Test-Path` 和 `Get-FileHash -Path` 按通配符解释，导致 checksum 阶段误报文件不存在。
- `install.ps1` 的 checksum 文件存在检查、`Get-FileHash`、payload source 检查、目标 Mod 检查已改用 `-LiteralPath`。
- `uninstall.ps1` 的 `installed.json`、目标 Mod、备份目录检查也改用 `-LiteralPath`。
- 已同步热修用户当前解压目录：`C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 和 `tools\uninstall.ps1`；后续重新导出的包会从模板自动带上修复。

## 影响文件/接口
- 接口不变：`POST /api/instances/:id/mods/sync-pack/export`。
- 文件：
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`
  - `docs/02-backend.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo -run "PlayerSync|ExportModSyncPack"`。
- 已执行：`go test ./...`。
- 已在当前解压包目录运行等价 checksum 循环：9 个 payload 文件全部存在且 hash 匹配，`[CP] MultipleConstructionOrders/...` 文件确认可通过 `-LiteralPath` 找到。

## 下一步注意事项
- 玩家端 PowerShell 处理来自 manifest/checksum/installed.json 的 Mod 文件夹名时，都必须默认使用 `-LiteralPath`，不要用通配符路径参数。

# PLAYERSYNC-PACK-2 后端接手记录

## 改了什么
- `ExportModSyncPackZip` 从“裸 Mod ZIP + `player-sync-manifest.json`”升级为玩家可执行安装包结构。
- ZIP 根目录现在包含 `安装玩家同步包.bat`、`卸载本同步包.bat`、`README.txt`、`pack-manifest.json`、`checksums.sha256`、`tools/` 和 `payload/`。
- 玩家需同步 Mod 写入 `payload/mods/<folderName>/`；`pack-manifest.json` 记录 `packId/packVersion/exportedAt`、Mod 清单、`builtIn/packaged` 和 SMAPI 元数据。
- `checksums.sha256` 记录 `payload/mods` 文件和随包 SMAPI ZIP 的 SHA256。安装脚本会先校验 payload，再复制 Mod。
- 导出时会查找实例目录 `.local-container/smapi/`、`.local-container/control/smapi/`、`smapi/` 下的 `SMAPI*.zip`。找到则写入 `payload/smapi/` 并记录 hash；找不到仍导出同步包，但玩家安装脚本提示需要自行安装 SMAPI。
- 包内 PowerShell 脚本实现 Windows 环境检查、Steam 游戏目录定位、运行中游戏进程拦截、同名 Mod 备份、安装记录写入、卸载恢复和 Steam 启动项尽力配置。

## 影响接口/文件
- 接口不变：`POST /api/instances/:id/mods/sync-pack/export`，下载文件名仍为 `stardew-player-sync-pack.zip`。
- ZIP 内部结构变化：旧的 `player-sync-manifest.json` 不再作为契约；下游应读取 `pack-manifest.json`。
- 主要文件：
  - `backend/internal/games/stardew_junimo/mod_sync.go`
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo`。
- 已执行：`go test ./internal/web`。
- 已执行：`go test ./...`。
- 测试覆盖：新 ZIP 根目录文件、`tools/`、`payload/mods/`、`payload/smapi/smapi.json`、`pack-manifest.json` 和 `checksums.sha256` 中 payload 文件 hash。

## 下一步注意事项
- 服务端目前不会自动联网下载 SMAPI；推荐后续在面板或后台任务中把官方 SMAPI ZIP 缓存到 `.local-container/smapi/`，再由同步包导出随包携带。
- Steam `localconfig.vdf` 修改是尽力策略：Steam 正在运行、找不到唯一用户或 VDF 结构不可靠时不阻断安装，只打印启动项文本。
- 玩家端状态写入游戏目录 `.anxi-sync/`，不要只依赖解压目录；玩家删掉同步包解压目录后仍应可从游戏目录恢复/卸载。

# NEXUS-SMAPI-THUMB-1 后端交接

## 改了什么
- 虚拟内置 SMAPI 条目现在自带 `UpdateKeys: ["Nexus:2400"]`、`nexusModId=2400` 和 Nexus 页面 URL。
- `GET /mods` 里的 `EnrichNexusMetadataForMods()` 会把虚拟 SMAPI 当成普通 Nexus 元数据项处理，首次请求时通过 GraphQL v2 拉取缩略图并缓存到 `.local-container/control/nexus-mods.json`。
- 这解决了前端把 SMAPI 显示成 Nexus:2400 后仍没有缩略图的问题。
- 同时修正 `ApplyNexusMetadataToMods()`：如果文件夹精确缓存只有来源信息但同一个 Nexus `modId` 的其他记录有完整 `pictureUrl`，会合并更完整的记录，避免 `[CP]` 内容包被半截缓存挡住缩略图。

## 影响文件和接口
- `backend/internal/games/stardew_junimo/mods.go`
- `backend/internal/games/stardew_junimo/mods_test.go`
- `backend/internal/games/stardew_junimo/nexus_metadata.go`
- `backend/internal/games/stardew_junimo/nexus_test.go`
- 既有接口：`GET /api/instances/:id/mods`，SMAPI 虚拟项可能新增返回 `pictureUrl/nexusSummary/nexusUrl/downloadCount/endorsementCount/updatedAt`。

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。

## 下一步注意事项
- 该缩略图依赖 Nexus GraphQL 可用；失败时接口仍返回 200，前端会显示来源文字占位。不要把图片缺失当成 Mod 列表失败。

# MODSYNC-AUTO-1 后端交接

## 改了什么
- `readModInfo` 现在会把 SMAPI `ContentPackFor.UniqueID` 暴露为 `ModInfo.contentPackFor`，并设置 `isContentPack=true`。
- `ApplyModSyncClassification` 的未手动覆盖默认值改为自动识别：`StardewAnxiPanel.Control` / `AnXiYiZhi.StardewAnxiPanel.Control` 为 `server_only`；`ContentPackFor` 内容包和常见 `[CP]`/`[AT]`/`[JA]` 等内容包前缀为 `client_required`；其他第三方 Mod 默认 `client_required`。自动分类会写入 `syncNote`，例如“自动识别：内容包需要玩家同步”。
- `PUT /api/instances/:id/mods/:modId/sync-classification` 从 `requireAdmin` 改为 `requireAuth`，任意登录用户都能修正服务器专用/玩家需同步/待确认标签。该接口仍只写 `.local-container/control/mod-sync.json`，不改 Mod manifest，也不受服务器运行状态限制。

## 影响文件和接口
- `backend/internal/games/registry/types.go`
- `backend/internal/games/stardew_junimo/mods.go`
- `backend/internal/games/stardew_junimo/mod_sync.go`
- `backend/internal/web/lifecycle_handlers.go`
- `GET /api/instances/:id/mods` 会返回新增 `isContentPack/contentPackFor`，并且很多以前默认 `unknown` 的 Mod 现在会直接返回 `client_required`。

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo`、`go test ./internal/web`。
- 覆盖点：普通第三方 Mod 默认玩家需同步；`ContentPackFor` 内容包默认玩家需同步并带自动说明；控制 Mod 默认服务器专用；手动 `unknown/server_only/client_required` 仍会覆盖自动识别。

## 下一步注意事项
- 自动识别策略故意偏保守：不认识的第三方 Mod 默认要求玩家同步，避免玩家少装 Mod 进不来。用户可以在前端把明确服务器专用的 Mod 改掉。

# MODORIGIN-1 后端交接

## 改了什么
- `registry.ModInfo` 新增 `originSource/originNexusModId/originModName/originModUrl`，用于表示内容包随某个 Nexus 包安装，而不是该内容包自己拥有 Nexus ID。
- `UploadModZip` 导入多 Mod ZIP 后会调用 `SaveInferredNexusPackageOrigin`，从同包中带 `Nexus:<id>` 的主 Mod 推断来源包，并写入 `.local-container/control/nexus-mods.json`。
- `ApplyNexusMetadataToMods` 不再把 sidecar `modId` 回填到无 `UpdateKeys` 的内容包 `nexusModId`；内容包只获得 `origin*` 字段，同时继承来源包缩略图和统计展示字段。
- `SaveInstalledNexusMetadata` 改为合并同 Nexus ID 的已有 sidecar 记录；主 Mod 后续通过 GraphQL 拉到缩略图时，同包 `[CP]` 内容包也会同步拿到图片。
- `DeleteMod` 改为 bundle-aware：会先解析目标文件夹，再按同一个 `nexusModId/originNexusModId` 找出同包真实 Mod 文件夹并一起删除，同时清理 sidecar 元数据。这样前端只需要调用一次 `DELETE /mods/:id`。

## 影响文件和接口
- `backend/internal/games/registry/types.go`
- `backend/internal/games/stardew_junimo/mods.go`
- `backend/internal/games/stardew_junimo/nexus_metadata.go`
- `backend/internal/games/stardew_junimo/mods_test.go`
- 既有 `GET /api/instances/:id/mods` 响应新增可选 `origin*` 字段；上传、Nexus 安装、远程 ZIP 安装返回的导入 Mod 也会带这些字段。既有 `DELETE /api/instances/:id/mods/:modId` 的响应结构不变，但行为会删除同包 bundle。

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo`。
- 新增覆盖：`MultipleConstructionOrders` Nexus 外壳 ZIP 导入后，主 Mod 保留 `nexusModId=47289`；`[CP]` 内容包 `nexusModId=0` 且 `originNexusModId=47289`，重新 `ListMods` 后仍持久化，并能继承来源包缩略图；删除 `[CP]` 内容包时主 Mod 和内容包都被删除，sidecar 清空。

## 下一步注意事项
- `originNexusModId` 不能用于判断内容包是否是独立 Nexus Mod；安装/更新逻辑如果后续接入版本检查，应仍以每个文件夹自己的 `UpdateKeys` 为准。
- 当前只推断同一个 ZIP 内的来源关系；如果用户先后单独上传主 Mod 和内容包，后端不会凭名称猜测来源包，避免误关联。

# MODZIP-3 后端交接

## 改了什么
- `readModInfo` 在解析 `manifest.json` 前会移除 UTF-8 BOM，兼容 Nexus 包中由 Windows 工具保存的 manifest。
- 真实触发包：`MultipleConstructionOrders 1.6 V1.1.0-47289-1-1-0-1780961270.zip` 的 `[CP] MultipleConstructionOrders/manifest.json` 带 BOM；修复前 `encoding/json` 会报 `invalid character 'ï' looking for beginning of value`。
- `sanitizeError` 放行常见 Mod 上传校验原因（目录已存在、UniqueID 重复、manifest 解析失败），避免 Web 层只返回“第 N 个 Mod ZIP 无效”。

## 影响文件和接口
- 后端文件：`backend/internal/games/stardew_junimo/mods.go`、`backend/internal/games/stardew_junimo/mods_test.go`、`backend/internal/web/audit.go`、`backend/internal/web/audit_test.go`。
- 既有接口变化：`POST /api/instances/:id/mods/upload`、Nexus 一键安装和远程 ZIP 安装在遇到 BOM manifest 时不再误报 `invalid_mod_zip`。

## 如何验证
- 已用本机原始 Nexus ZIP 临时测试确认 `UploadModZip` 可成功导入两个 Mod。
- 已保留通用测试：`TestReadModInfo_AllowsUTF8BOMManifest`。
- 建议执行：`go test ./internal/games/stardew_junimo ./internal/web`。

## 下一步注意事项
- 这里只兼容 UTF-8 BOM，不支持非 UTF-8 编码 manifest；如果后续遇到 GBK/UTF-16 manifest，应先确认 SMAPI 是否接受，再决定是否转换。

# MODDEPS-1 后端交接

## 改了什么
- `readModInfo` 会解析 SMAPI manifest 的 `Dependencies` 和 `ContentPackFor`，并把它们写入 `registry.ModInfo.Dependencies`。
- 新增 `registry.ModDependency`：`uniqueId`、`minimumVersion`、`required`。`Dependencies[].IsRequired` 缺省为 `true`；`ContentPackFor` 始终按必需依赖处理。
- `ContentPackFor` 和 `Dependencies` 中重复的 `UniqueID` 会去重，优先保留前者的最低版本信息，适配 Content Patcher 内容包常见 manifest 写法。

## 影响文件和接口
- 后端文件：`backend/internal/games/registry/types.go`、`backend/internal/games/stardew_junimo/mods.go`、`backend/internal/games/stardew_junimo/mods_test.go`。
- 既有接口变化：`GET /api/instances/:id/mods` 的 `mods[]` 新增可选字段 `dependencies[]`。上传、Nexus 安装和远程安装导入成功后返回的 Mod 信息也会带该字段。

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。
- 新增覆盖：普通 `Dependencies`、可选依赖 `IsRequired:false`、`ContentPackFor` 作为依赖且与 `Dependencies` 重复时去重。

## 下一步注意事项
- 当前只暴露依赖声明，不检查依赖是否已安装，也不自动补装。后续配置页“依赖检查”应复用该字段与已安装 Mod 的 `uniqueId` 列表做完整性判断。

# MODZIP-2 后端交接

## 改了什么
- `UploadModZip` 现在会识别 Nexus 下载包常见的单层外壳目录：当 ZIP 根目录只有一个目录且该目录本身不是 Mod，但它的直接子目录包含 `manifest.json` 时，后端会剥掉外壳目录，把子目录作为实际 Mod 导入。
- 典型例子：`MultipleConstructionOrders/MultipleConstructionOrders/manifest.json` 和 `MultipleConstructionOrders/[CP] MultipleConstructionOrders/manifest.json` 会导入到服务器 Mods 根目录的两个独立 Mod 文件夹，而不是把外壳目录当成缺少 manifest 的无效 Mod。
- 只支持剥离一层外壳目录，避免误拆更复杂的任意嵌套结构；原有 zip-slip、重复 UniqueID、已存在目录、XNB 替换包等校验保持不变。

## 影响文件和接口
- 后端文件：`backend/internal/games/stardew_junimo/mods.go`、`backend/internal/games/stardew_junimo/mods_test.go`。
- 既有接口变化：`POST /api/instances/:id/mods/upload`、Nexus 一键安装和远程 ZIP 安装都会受益，因为它们最终都复用 `UploadModZip`。

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。
- 新增覆盖：单外壳目录包内含两个真实 Mod 子目录时，应剥离外壳并成功导入两个 Mod。

## 下一步注意事项
- 这只是 SMAPI Mod ZIP 结构兼容，不会自动补装依赖；依赖声明已经由 `MODDEPS-1` 暴露给前端，缺失依赖检查仍是后续能力。
- 如果后续遇到更深层级或安装说明型 ZIP，先确认是否应该被服务器自动安装，不要放宽到任意深度递归导入。

# MODUPLOAD-2 后端交接

## 改了什么
- `handleModsUpload` 从只读取 `r.FormFile("mod")` 改为 `ParseMultipartForm` 后读取多个文件，支持重复的 `mod` 字段，并兼容 `mods` 字段。
- 每个上传 ZIP 仍写入临时文件后交给 `sj.UploadModZip`，没有在 web 层绕过 Stardew Junimo driver 的 Mod 安装校验。
- 新增 `rollbackImportedMods`：批量上传中后续 ZIP 失败时，会逆序删除本次请求前面已经导入的 Mod 文件夹，避免半成功状态。

## 影响文件和接口
- `backend/internal/web/lifecycle_handlers.go`
- `backend/internal/web/saves_handlers_test.go`
- 接口仍是 `POST /api/instances/:id/mods/upload`，请求为 `multipart/form-data`。前端推荐重复提交 `mod` 文件字段；成功响应仍为 `ModsListResult`，但 `mods[]` 可能包含多个 ZIP 导入出的所有 Mod。

## 如何验证
- 已执行：`go test ./...`
- 新增覆盖：`TestModUpload_AcceptsMultipleZipFiles`，同一个 multipart 请求里上传两个 ZIP，断言返回两个 Mod 且停服上传时 `restartRequired=false`。

## 下一步注意事项
- 当前仍只支持 ZIP；7z/rar 需要新增解压器和 zip-slip 等同级安全校验后再开放。
- 批量上传共享原请求大小上限 `maxModFormSize`，如果后续希望上传很多大 Mod，需要产品层面决定是否做分片、队列或进度条。

# NEXUS-META-1 后端交接

## 改了什么
- `SearchNexusMods(ctx, query, apiKey)` 的无 Key 纯数字查询改为 Nexus GraphQL v2 精确 ID 查询，filter 使用 `gameId=1303` + `modId=<id>`；配置 Key 时仍优先使用 v1 REST 精确 ID 查询。
- 新增 `EnrichNexusMetadataForMods(ctx, dataDir, mods)`：对本地已安装 Mod 读取 manifest `UpdateKeys` 中的 `Nexus:<id>`，当 sidecar 缺失时用 GraphQL v2 拉取 `pictureUrl/summary/downloads/endorsements/updatedAt` 等卡片字段，并写入 `.local-container/control/nexus-mods.json`。
- `GET /api/instances/:id/mods` 从单纯 `ApplyNexusMetadataToMods` 改为调用补全函数；Nexus 请求失败不阻断列表返回。

## 影响文件和接口
- `backend/internal/games/stardew_junimo/nexus.go`
- `backend/internal/games/stardew_junimo/nexus_metadata.go`
- `backend/internal/games/stardew_junimo/nexus_test.go`
- `backend/internal/web/lifecycle_handlers.go`
- 接口行为：`GET /api/instances/:id/mods` 可能自动补齐 `mods[].pictureUrl/nexusSummary/nexusUrl/downloadCount/endorsementCount/updatedAt`；`GET /api/instances/:id/mods/search?q=<数字ID>` 无 Key 时也能返回 GraphQL 精确 ID 元数据。

## 如何验证
- 已执行：`go test ./...`。
- 新增覆盖：无 Key 数字 ID 搜索的 GraphQL 请求形状、无 `apikey` header、手动上传/本地 Mod 元数据补全和 sidecar 缓存。

## 下一步注意事项
- GraphQL 元数据补全是展示能力，不代表 Nexus 下载能力；v1 文件列表/下载链接仍受 API Key、账号权限、NXM ticket 或临时 CDN 链接影响。
- 当前补全每次最多处理 20 个缺失 Nexus ID，并成功后缓存；如果以后要大批量刷新，建议做后台任务和缓存过期策略。

# MODSEARCH-3 后端交接

## 改了什么
- `SearchMods` 在返回统一搜索结果前按 `downloadCount` 从高到低做稳定排序；下载量相同的条目保留 provider 原始顺序。
- 新增 `TestSearchModsSortsResultsByDownloadCountDescending`，用 mock GraphQL 返回乱序下载量，断言最终顺序为高到低。
## 影响文件和接口
- `backend/internal/games/stardew_junimo/mod_search.go`
- `backend/internal/games/stardew_junimo/mod_search_test.go`
- 接口行为：`GET /api/instances/:id/mods/search?q=...` 的 `results[]` 顺序现在固定为下载量降序。
## 如何验证
- 执行：`go test ./...`。
## 下一步注意事项
- 后续接入 CurseForge/ModDrop/GitHub/Dataset provider 时，合并结果后仍应走同一个排序函数，再返回前端。

# MODRESTART-1 后端交接

## 改了什么
- Mod 写操作当前都要求服务器停止，因此上传/删除/Nexus 安装/远程安装成功后不再设置 `modsRestartRequired`，而是清理旧标记。
- `GET /api/instances/:id/mods` 新增状态约束：只有实例 running/starting 且底层标记存在时才返回 `restartRequired=true`；停服状态永远返回 `false`。
- `doStop` 停止服务器成功后会清理旧的 Mod 重启标记。

## 影响文件和接口
- 后端文件：`web/lifecycle_handlers.go`、`stardew_junimo/lifecycle.go`、`nexus_install.go`、`remote_install.go`。
- 既有接口变化：`POST /api/instances/:id/mods/upload` 停服成功上传后返回 `restartRequired=false`。

## 如何验证
- 执行：`go test ./internal/games/stardew_junimo ./internal/web`。

## 下一步注意事项
- 如果未来允许运行中热改 Mod，再按实际运行态重新设置 `modsRestartRequired`；当前不要在停服写操作中提示用户重启。

# MODZIP-1 后端交接

## 改了什么
- `UploadModZip` 增加 XNB 替换包识别：没有 `manifest.json`，但包含 `.xnb` 且路径类似 `Characters/`、`Portraits/` 等游戏内容目录时，返回“这是 XNB 替换包，不是 SMAPI Mod”的明确错误。
- `sanitizeError` 对 XNB/SMAPI/manifest 错误保留产品级提示，不再统一显示为“压缩包格式错误或已损坏”。

## 影响文件和接口
- 后端文件：`mods.go`、`mods_test.go`、`web/audit.go`、`web/audit_test.go`。
- 既有接口变化：`POST /api/instances/:id/mods` 对 XNB 替换包仍返回 `400 invalid_mod_zip`，但 message 更明确。

## 如何验证
- 执行：`go test ./internal/games/stardew_junimo ./internal/web`。

## 下一步注意事项
- 当前不支持把 XNB 替换包自动安装进游戏 Content 目录；这类文件通常是客户端内容替换，不应混入服务器 `Mods` 目录。

# MODSEARCH-2 回滚记录

## 改了什么
- 已按产品要求撤回 Dataset / CurseForge 多源搜索接入，`SearchMods` 恢复为仅调用 Nexus GraphQL v2 / REST v1。
- 删除 Dataset provider、CurseForge provider、CurseForge Key 设置接口和 `curseforge_download_url` 安装分支。

## 影响文件和接口
- 后端仍保留 MODSEARCH-1 统一卡片接口：`GET /api/instances/:id/mods/search` 与 `POST /api/instances/:id/mods/search/install`。
- 当前 live 搜索/一键安装来源只有 Nexus；`/api/settings/curseforge` 不存在。

## 如何验证
- 回滚后需执行：`go test ./...`。

## 下一步注意事项
- StardewModDataset、CurseForge、ModDrop、GitHub Release 重新回到后续计划；再次接入前先确认产品优先级。

# MODSEARCH-1 后端交接

## 改了什么
- 新增 `backend/internal/games/stardew_junimo/mod_search.go`，把页面搜索结果抽象成 `ModSearchResult`，统一描述来源、跳转按钮、安装方式、站点 ID 和已安装状态。
- 新增 `GET /api/instances/:id/mods/search`，当前先复用 Nexus GraphQL v2 / REST v1 搜索并映射到统一模型：N站结果显示 `sourceName=N站`、`externalLabel=跳转 N站`、`installLabel=一键安装`。
- 新增 `POST /api/instances/:id/mods/search/install`，按 `installMethod` 分发安装：`nexus_premium` 走现有 `InstallNexusMod`，`direct_url` 走 `InstallModFromDirectURL`，不支持的来源返回 `mod_install_not_supported`。

## 影响文件和接口
- 后端文件：`mod_search.go`、`web/lifecycle_handlers.go`、`web/instance_handlers.go`。
- 新接口：`GET /api/instances/:id/mods/search?q=...`、`POST /api/instances/:id/mods/search/install`。
- 旧 Nexus 专用接口仍保留兼容。

## 如何验证
- 已执行：`go test ./...`。
- 新增测试：`mod_search_test.go` 覆盖 Nexus 结果映射到统一卡片字段。

## 下一步注意事项
- StardewModDataset、CurseForge、ModDrop、GitHub Release 还没有真正接入 provider；下一步应在 `SearchMods` 内追加 provider、去重和排序，不需要再改前端卡片协议。
- CurseForge/ModDrop/GitHub provider 如果能拿到可下载 ZIP URL，应填 `installMethod=direct_url` 和 `installUrl`；Nexus Premium 继续填 `nexus_premium`。

# REMOTE-MOD-1 后端交接

## 改了什么
- 新增 `backend/internal/games/stardew_junimo/remote_install.go`，支持 `InstallRemoteMod` 统一处理 `nxm://` 和 `https://...zip` 两种远程安装输入。
- NXM 链路通过 `ParseNexusNXMURL` 解析 `modId/fileId/key/expires`，再调用 v1 `download_link.json?key=...&expires=...`，覆盖 Nexus 非 Premium 慢速下载授权路径。
- 直链链路限制为 HTTPS ZIP，下载后复用 `UploadModZip`，不新增解压分支；来源可以是 Nexus CDN、ModDrop、GitHub、CurseForge 等公网 ZIP。
- 新增 `POST /api/instances/:id/mods/remote/install`，创建 `mod_remote_install` job。审计日志只记录 jobId，不记录用户粘贴的临时 URL。
- `doNexusRequest` 和远程下载网络错误改为不携带完整 URL，避免泄露 `key/expires/user_id`。

## 影响文件和接口
- 后端文件：`remote_install.go`、`nexus_install.go`、`nexus.go`、`nexus_test.go`、`web/lifecycle_handlers.go`、`web/instance_handlers.go`。
- 新接口：`POST /api/instances/:id/mods/remote/install`，body `{ url, mod? }`，成功 `202 { jobId }`。

## 如何验证
- 已执行：`go test ./...`。
- 测试新增覆盖：NXM URL 解析、`key/expires` 透传到 `download_link.json`、ZIP 下载并导入。

## 下一步注意事项
- 当前直链只支持 ZIP；7z/rar 需要新的解压器和 zip-slip 同等级安全校验。
- CurseForge/GitHub/ModDrop/StardewModDataset 还没有真正接入后端搜索/自动解析安装，只在路线里保留为后续统一来源；其中 ModDrop 先通过 StardewModDataset 索引覆盖，安装侧走管理员粘贴 ZIP 直链。

# NEXUS-3 后端交接

## 改了什么

- Nexus 搜索分流调整：无 API Key 时纯数字 query 不再提前报 `ErrNexusAPIKeyMissing`，而是走 GraphQL v2 `gameId=1303 + modId` 精确元数据查询；配置 Key 后仍可启用 v1 REST 精确 ID 查询。
- 新增 `POST /api/instances/:id/mods/nexus/install`，管理员专用，服务器运行中禁止。接口创建 `mod_nexus_install` job，后端通过 Nexus v1 `files.json` 与 `download_link.json` 下载 ZIP，再复用 `UploadModZip` 完成校验、解压、重复检查和导入。
- 新增 `.local-container/control/nexus-mods.json` 保存 Nexus 搜索卡片元数据。`GET /api/instances/:id/mods` 与 sync plan 已回填 `pictureUrl/nexusUrl/nexusSummary/downloadCount/endorsementCount/updatedAt`。

## 影响文件和接口

- 后端文件：`nexus.go`、`nexus_install.go`、`nexus_metadata.go`、`mod_sync.go`、`registry/types.go`、`web/lifecycle_handlers.go`、`web/instance_handlers.go`。
- 新接口：`POST /api/instances/:id/mods/nexus/install`。
- 既有接口变化：`GET /api/instances/:id/mods/nexus/search` 无 Key 数字 query fallback 到 GraphQL；`GET /api/instances/:id/mods` 返回更多 Nexus 展示字段。

## 如何验证

- 已执行：`go test ./...`。
- 覆盖：无 Key 数字 query fallback、Nexus ZIP 下载/安装、元数据保存和已安装列表回填。

## 下一步注意

- Nexus 的 `download_link` 是否可用受 Nexus 账号权限、文件限制和上游策略影响；当前实现会把非 2xx 映射成结构化失败并进入 job error。后续可以对“需要手动下载/会员/OAuth”的情况做更细提示。
- 当前自动选择主文件，优先 `is_primary`、`category_id=1`、`MAIN`，多主文件/可选文件 UI 仍未做。

# 后端接手文档 2026-07-01

## 同日补充 4：Nexus API Key 改为 SQLite 面板设置，不再读环境变量

本次把 Nexus Key 配置从进程环境变量彻底移除：后端不再读取任何 Nexus Key 环境变量，管理员在前端“下载模组”页点击“配置 API Key”后，后端通过 `PUT /api/settings/nexus/api-key` 写入 SQLite `panel_settings`（key=`nexus_api_key`）。`GET /api/settings/nexus` 只返回 `configured` 和 `last4`，不会回显完整 Key；`DELETE /api/settings/nexus/api-key` 删除配置。保存/删除写审计日志，但 metadata 不包含 Key 本体。

搜索路径现在是：`handleModNexusSearch` 每次请求即时从 `panel_settings` 读取 Key，再调用 `SearchNexusMods(ctx, query, apiKey)`。因此保存后无需重启后端，后续搜索立即生效；后端重启后也会继续从 SQLite 读取。关键词搜索和无 Key 数字 ID 元数据查询都可走公开 GraphQL v2；Key 只影响 v1 REST ID 查询和下载/安装链路。

涉及文件：`backend/internal/storage/settings.go`、`backend/internal/web/settings_handlers.go`、`backend/internal/web/handler.go`、`backend/internal/web/lifecycle_handlers.go`、`backend/internal/games/stardew_junimo/nexus.go`、`nexus_test.go`、`settings_handlers_test.go`。验证方式：`go test ./...` 覆盖 Key 保存/状态/删除/权限、无 Key 关键词搜索、ID 查询缺 Key、Key 不泄露等路径。

## 同日补充 3：`isNexusAuthError` 关键词收紧，避免误判

代码评审发现：`isNexusAuthError`（`nexus.go`）原来用 `strings.Contains(lower, "auth")` 判断 GraphQL 错误是不是鉴权类拒绝，但 `"author"`（Mod 的真实字段名，查询/参数写错时报错信息里经常会出现）也包含子串 `"auth"`，会被误判成 `ErrNexusAuthRequired`，把"查询结构/字段名写错了"这种本该用 `nexus_request_failed` 暴露出来、提示去看日志排查 schema 的错误，错误地映射成"需要 OAuth/更高权限"，掩盖真实问题排查方向。

修复：把判断关键词从单一的 `"auth"` 收紧成一组更明确的词——`unauthenticated`、`unauthorized`、`authentication`、`authorization`、`forbidden`、`permission`（`nexusAuthErrorKeywords`），任意一个命中才算鉴权错误。新增回归测试 `TestSearchNexusMods_GraphQLSchemaErrorMentioningAuthorIsNotAuthError`，构造一条提到 `'author'` 字段的 schema 错误，断言不会被误判成 `ErrNexusAuthRequired`。

涉及文件：`backend/internal/games/stardew_junimo/nexus.go`、`nexus_test.go`。`go build`/`go vet`/`go test ./...` 全部通过。

## 同日补充 2：Nexus 关键词搜索 GraphQL 查询结构已修复并验证（不再是猜测）

实际联调时发现关键词搜索每次都报"Nexus 请求失败，请稍后重试"（`nexus_request_failed`）。排查方式：直接用 `curl` 对 `https://api.nexusmods.com/v2/graphql` 发请求复现，拿到的真实响应是：

```json
{"errors":[{"message":"Field 'mods' doesn't accept argument 'gameDomain'", ...}]}
```

确认是之前实现里**猜测的查询结构是错的**——`mods` 这个 GraphQL 根字段本身没有 `gameDomain` 参数。又对同一个端点做了 schema introspection（`{ __type(name: "ModsFilter") { inputFields { ... } } }` 这类查询，GraphQL introspection 默认是开放的，不需要鉴权）确认了真实结构：

- `mods(filter: ModsFilter, facets: ModsFacet, postFilter: ModsFilter, sort: [...], offset: Int, count: Int): ModPage!`，没有 `gameDomain` 参数。
- 游戏域名要放进 `filter.gameDomainName`：`[{value: "stardewvalley", op: EQUALS}]`（类型是 `BaseFilterValue`，`op` 取值来自枚举 `FilterComparisonOperator`：`EQUALS`/`NOT_EQUALS`/`MATCHES`/`WILDCARD`/`GT`/`GTE`/`LT`/`LTE`）。
- 关键词要放进 `filter.name`：`[{value: <关键词>, op: WILDCARD}]`（类型是 `BaseFilterValueEqualsWildcard`，`op` 取值是 `EQUALS`/`NOT_EQUALS`/`WILDCARD` 这个更窄的枚举）。`WILDCARD` 是子串匹配，不需要在值里手动加 `*`——实测关键词 `tractor` 能匹配到标题 `Wallet Tools - Tractor Mod Addon` 这种把目标词嵌在中间的情况。
- `mods.nodes` 里每个 `Mod` 对象的字段名（`modId`/`name`/`summary`/`version`/`author`/`pictureUrl`/`downloads`/`endorsements`/`updatedAt` 等）和原来猜测的完全一致，不用改 `nexusGraphQLNode`/`nexusGraphQLResponse` 这两个 struct，只需要改请求变量的构造方式和查询字符串本身。

修复后用真实账号之外的公开 introspection 和真实搜索请求（关键词 `tractor`、`stardew valley expanded`）反复验证过，能拿到正确结果（标题、作者、下载量等字段都对得上）；额外测试了用户最初报错时输入的中文关键词"拖拉机"——这个词本身在 Nexus 索引里查不到任何 Mod 标题包含它（绝大多数 Mod 标题是英文），所以合法地返回 0 条结果，这不是 bug，是真实的数据现状（已经在 `docs/02-backend.md` 里写明，建议用户优先用英文关键词或数字 ID）。

涉及改动：`backend/internal/games/stardew_junimo/nexus.go` 的 `nexusGraphQLSearchQuery`（查询字符串去掉了顶层 `$gameDomain` 变量和 `mods(gameDomain: ...)` 参数）和 `nexusSearchByKeyword`（构造 `variables.filter` 时改成 `{gameDomainName: [...], name: [...]}` 而不是原来错的 `{search: query}`）。新增回归测试 `TestSearchNexusMods_KeywordSearchRequestShape`（`nexus_test.go`），直接断言发出去的请求体里 `variables` 没有顶层 `gameDomain` 字段、`filter.gameDomainName`/`filter.name` 的值符合预期，防止以后又不小心改回错误结构。

验证方式：

1. `go build ./...`、`go vet ./...`、`go test ./...` 全部通过。
2. 额外写了一个临时的、跑真实网络请求的手工测试（验证完立刻删除，不留在仓库里），直接调用 `SearchNexusMods(ctx, "tractor")` 和 `SearchNexusMods(ctx, "拖拉机")`，确认前者拿到 20 条真实结果、后者合法返回 0 条，证明修复后的代码路径（不是 curl，是真实 Go HTTP 客户端 + JSON 编解码）端到端可用。

如果以后 Nexus 又改了 GraphQL schema导致这条路径又报错，排查第一步应该是对 `https://api.nexusmods.com/v2/graphql` 重新做一次 introspection（参考上面的查询写法），而不是凭印象改字段名。

## 同日补充：Nexus 搜索鉴权按路径拆开（不再统一要求 Key）

上线前评审发现下面"Mod 管理第二阶段"一节最初的实现把个人 API Key 当成了两条查询路径的统一前置门槛，这个假设过严：Nexus 的 GraphQL v2（关键词搜索用的那条路径）是公开只读查询，通常不需要个人 API Key；需要鉴权的是 v1 REST 的按 ID 精确查询（个人账号级接口）。按统一要求 Key 实现，会导致没配置 Key 时连最基础的关键词搜索都用不了，而这本来不该被挡住。

修复后的行为（`backend/internal/games/stardew_junimo/nexus.go`）：

- **空查询校验**提到最前面，两条路径都先过这一关，返回 `ErrInvalidNexusQuery`。
- **纯数字查询（精确 ID）**：配置个人 API Key 时走 v1 REST；未配置时走 GraphQL v2 `gameId=1303 + modId` 精确元数据查询，不再返回 `ErrNexusAPIKeyMissing`。
- **关键词查询（GraphQL v2）**：**不再要求个人 API Key**。调用方传入的 `apiKey` 可能为空，`setNexusHeaders` 只在 `apiKey != ""` 时才设置 `apikey` 请求头，未配置时干脆不带这个头，而不是带一个空值上去。
- 新增哨兵错误 `ErrNexusAuthRequired`：如果 Nexus 对关键词查询本身返回认证类拒绝（HTTP 401/403，或者 GraphQL 响应体里 `errors[0].message` 包含 `auth`/`forbidden`/`permission` 字样——`isNexusAuthError` 做的判断），`nexusSearchByKeyword` 会把原始错误转换成这个哨兵值再返回。handler（`writeNexusError`）把它映射成 `502 nexus_auth_required`，文案是"该查询需要 Nexus OAuth/认证能力"——刻意不说"请配置个人 API Key"，因为配置个人 Key 大概率解决不了这个问题（这通常意味着该查询需要 OAuth 或更高权限，不是简单的"没传 Key"）。
- 注意 `ErrNexusAuthRequired` 只会从关键词路径产生。ID 查询路径如果遇到 401/403（比如 Key 失效），还是走原来的通用 `*NexusAPIError` → handler 里 `nexus_unauthorized` 这条映射，文案是"Nexus API Key 无效或权限不足"——这个区分是有意的：两条路径的 401/403 含义不一样，一个是"你配的 Key 有问题"，一个是"压根不是配 Key 能解决的"。

涉及测试改动（`nexus_test.go`）：当前 `TestSearchNexusMods_NumericQueryUsesGraphQLIDLookupWithoutAPIKey` 覆盖数字查询 + 无 Key → GraphQL v2 `gameId + modId`，并断言请求**没有**带 `apikey` 头；`TestSearchNexusMods_KeywordSearchWorksWithoutAPIKey` 覆盖无 Key 关键词搜索；`TestSearchNexusMods_KeywordSearchAuthRequired_HTTPStatus` 和 `TestSearchNexusMods_KeywordSearchAuthRequired_GraphQLError` 覆盖 GraphQL 鉴权失败映射。

`go vet ./...`、`go build ./...`、`go test ./...` 全部通过。

下面"Mod 管理第二阶段"一节里凡是提到"未配置 API Key 时 `SearchNexusMods` 直接返回错误、不发任何请求"，已经被 `NEXUS-META-1` 修正：展示型关键词搜索和数字 ID 元数据查询都不需要 Key；Key 只用于 v1 REST 和下载/安装链路。本节是对该小节的修正补充，不是重写，原文保留供参考实现细节（HTTP 客户端结构、错误脱敏等没有变化）。

## 本次改动：Mod 管理第二阶段——Nexus Mods 只读搜索

在不绕过 `stardew_junimo` driver、不让前端直连 N站的前提下，给 Mod 管理加上 Nexus Mods 在线只读搜索：管理员和普通登录用户都能搜、看基础信息、跳转 N站；不做下载/安装。

### 前置修复确认

第一阶段遗留的两个问题在本次开工前确认**已经修复**（属于上一轮代码评审修复，见本文件下方"代码评审修复"小节，不是本次新改的）：

1. `ExportModSyncPackZip` 玩家同步包导出已用 `os.CreateTemp` 生成唯一临时文件名，不再固定写 `%TEMP%\stardew-player-sync-pack.zip`。
2. `SetModSyncClassification` 已加 `dataDir` 维度的 `sync.Mutex`，`saveModSyncStore` 已改成临时文件 + `os.Rename` 原子写入。

本次没有再改这两处。

### 配置

Nexus 个人 API Key 由管理员在前端配置，后端持久化到 SQLite `panel_settings`，不接入环境变量。`SearchNexusMods` 不直接读取配置；HTTP handler 每次从数据库取出当前 Key 后传给 `SearchNexusMods(ctx, query, apiKey)`，所以保存后立即生效，重启后继续生效。未配置时纯数字 ID 查询会改走公开 GraphQL v2 精确元数据查询；需要 Key 的是 v1 REST 查询和下载/安装链路。

### Nexus 官方 API 能力边界（重要，决定了实现方式）

开工前查证过：Nexus Mods 官方 v1 REST API（`api-docs.nexusmods.com`，文档化、`apikey` 请求头鉴权）**没有任何关键词全文搜索接口**——只有按 ID 精确查询单个 Mod（`/v1/games/{domain}/mods/{id}.json`）、最近更新列表（无 name/summary 等字段）、MD5 查询等。Nexus 官方维护的 `node-nexus-api` 客户端库文档也明确没有暴露搜索方法。

因此 `SearchNexusMods` 按查询内容分两条路径：

- **纯数字查询** → 有 Key 时走已验证、文档化的 v1 REST 精确 ID 查询：`GET https://api.nexusmods.com/v1/games/stardewvalley/mods/{id}.json`；无 Key 时走 GraphQL v2 `gameId=1303 + modId` 元数据查询。
- **其余关键词** → 走 GraphQL v2（`https://api.nexusmods.com/v2/graphql`，和 nexusmods.com 网站搜索框同源）。这条路径已通过真实 schema introspection 和真实搜索请求验证；如果以后关键词搜索解析失败，第一步应该重新对照 Nexus 最新 GraphQL schema 核对 `nexusGraphQLSearchQuery` 和 `nexusGraphQLResponse`。

两条路径共享同一套请求基础设施（超时、User-Agent、错误映射、API Key 不泄露处理），改起来互不影响。

### 实现

新文件 `backend/internal/games/stardew_junimo/nexus.go`：

- `SearchNexusMods(ctx, query, apiKey) (NexusModSearchResponse, error)`：trim 校验 query → 按数字/关键词分流 → 数字 ID 有 Key 走 v1 REST、无 Key 走 GraphQL v2 `gameId + modId` → 结果裁剪到 `nexusMaxResults = 20`。
- `nexusGetModByID` / `nexusSearchByKeyword`：分别对应上面两条路径，都通过 `doNexusRequest` 发起请求。
- `doNexusRequest`：统一处理超时（10s，`nexusHTTPClient`，可在测试里换底层 endpoint）、非 2xx 映射。**非 2xx 响应体永远不会被读取转发**——只丢弃 body、保留状态码包成 `*NexusAPIError{StatusCode}`，这样即使上游返回的错误页里意外回显了请求细节（比如把 apikey 错误地塞进错误文案），也不会经我们的错误链路泄露出去；apikey 本身只通过请求头发送，从来不出现在 URL 或 body 里。
- `ApplyNexusInstalledMatch(dataDir, results)`：读本地 `ListMods`，按 `NexusModID` 建 map，给搜索结果打 `installed`/`installedFolderName`/`installedVersion`。本阶段只判断"是否已装"，不比较版本新旧。
- `nexusV1BaseURL`/`nexusGraphQLURL` 是包级 `var` 而非 `const`，专门留给测试用 `httptest.Server` 替换。

### manifest 解析扩展

`mods.go`：`modManifest` 新增 `UpdateKeys []string`（对应 SMAPI manifest.json 的 `UpdateKeys` 字段）；新增 `parseNexusModIDFromUpdateKeys(keys []string) (int, bool)`，从形如 `"Nexus:2400"`（大小写不敏感站点名，允许 `Nexus:2400:subkey` 这种带子 key 的变体）的条目里挑出数字 ID。`readModInfo` 解析时把 `UpdateKeys` 和解析出的 `NexusModID` 一起填进 `registry.ModInfo`。

`registry/types.go`：`ModInfo` 新增 `UpdateKeys []string` 和 `NexusModID int`（都是 `omitempty`），所以 `GET /api/instances/:id/mods` 现有响应会自动带上这两个新字段，前端旧代码不受影响。

### API

新增 1 个接口，路由加在 `backend/internal/web/instance_handlers.go`（`mods/export` 之后），handler 在 `backend/internal/web/lifecycle_handlers.go`：

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/instances/:id/mods/nexus/search?q=关键词` | `requireAuth`（任意登录用户） | 返回 `NexusModSearchResponse{query, results}` |

`q` 在 handler 层先 `strings.TrimSpace`，空字符串直接 `400 invalid_query`，不会调用到 `sj.SearchNexusMods`（`sj` 内部也有同样的校验作为第二道防线，因为这是一个公开的包函数，不能假设调用方都先 trim 过）。

错误映射（`writeNexusError`，**已按上方"同日补充"更新**——`nexus_api_key_missing` 现在只会从 ID 查询路径触发，新增 `nexus_auth_required` 专属关键词路径）：

| 场景 | HTTP | code |
| --- | --- | --- |
| 空查询 | 400 | `invalid_query` |
| Nexus 下载/安装但未在面板配置 Nexus API Key | 503 | `nexus_api_key_missing` |
| 关键词搜索被 Nexus 拒绝（需要 OAuth/更高权限） | 502 | `nexus_auth_required` |
| Nexus 404（ID 查询未命中） | 404 | `nexus_mod_not_found` |
| Nexus 401/403（仅 ID 查询路径，Key 无效/权限不足） | 502 | `nexus_unauthorized` |
| Nexus 429 | 429 | `nexus_rate_limited` |
| 其他非 2xx / 网络错误 | 502 | `nexus_request_failed` |

### 测试

新文件 `backend/internal/games/stardew_junimo/nexus_test.go`，全部用 `httptest.Server` mock，不依赖真实网络/真实 Key：

- API Key 缺失 → `ErrNexusAPIKeyMissing`。
- 空 query（含纯空格）→ `ErrInvalidNexusQuery`。
- ID 查询结果解析（mock v1 REST 响应，校验字段映射、`NexusURL` 拼接）。
- 关键词搜索结果解析（mock GraphQL 响应，多条结果）。
- 结果数量裁剪到 `nexusMaxResults`。
- 非 2xx 状态码映射为 `*NexusAPIError`（ID 查询和关键词搜索两条路径都覆盖）。
- **API Key 不泄露**：mock 一个把 Key 回显进错误响应体的"有 bug 的上游"，断言最终返回给调用方的 `error.Error()` 不包含该 Key（同时验证请求确实带上了 `apikey` 头，证明鉴权本身没问题，只是错误路径不传播 body）。
- `parseNexusModIDFromUpdateKeys` 各种格式（大小写、带子 key、混合多个 UpdateKeys、纯非 Nexus、空、非数字 ID）。
- `ApplyNexusInstalledMatch` 按 `Nexus:ID` 命中/不命中、没有任何已装 Mod 时不报错。
- `readModInfo` 正确把 `UpdateKeys`/`NexusModID` 填进 `registry.ModInfo`。

验证：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
```

全部通过（含既有用例）。

### 下一步注意事项

- 关键词搜索（GraphQL v2 路径）已经用真实 schema introspection 和真实搜索请求验证过；后续如果失败，优先复查 Nexus schema 是否变化。
- 不涉及 SMAPI mod DLL 或 Junimo 容器内文件，**不需要重启 Stardew server 容器**。
- 本阶段是纯只读搜索，没有下载/安装/校验文件完整性的逻辑；如果以后要做"从 Nexus 直接安装"，注意 Nexus 的文件下载链接通常需要 premium 账号或走 OAuth/SSO 流程，不能直接复用本阶段只读搜索用的个人 `apikey`。
- `installed` 匹配完全依赖 Mod 自己 manifest.json 里声明的 `UpdateKeys`；没声明 `Nexus:<id>` 的已装 Mod（很多老 Mod 或本地自制 Mod 都没有）永远不会被标记为已安装，这是预期行为，不是 bug。
- 新增接口已同步更新 `docs/02-backend.md`、`docs/06-integration.md`、`docs/08-future-roadmap.md`、`docs/09-image-build.md`。

## 本次改动：Mod 玩家同步包（第一阶段）

在不绕过 `stardew_junimo` driver 的前提下，给 Mod 管理加上“同步分类”能力，让用户能标出哪些 Mod 玩家必须在客户端同步安装，并一键导出对应 ZIP。

### 数据结构

- `registry.ModInfo` 新增 `SyncKind string`（`json:"syncKind"`，恒返回）和 `SyncNote string`（`json:"syncNote,omitempty"`）。
- 新增常量 `ModSyncKindServerOnly` / `ModSyncKindClientRequired` / `ModSyncKindUnknown` 和校验函数 `ValidModSyncKind`。
- 新增 `ModSyncSummary`（total/serverOnly/clientRequired/unknown）和 `ModSyncPlanResult`（mods + summary）。
- 涉及文件：`backend/internal/games/registry/types.go`。

### 持久化

新文件 `backend/internal/games/stardew_junimo/mod_sync.go`：

- 分类数据落在面板自有文件 `<dataDir>/.local-container/control/mod-sync.json`，格式 `{"mods": {"<folderName>": {"syncKind": "...", "syncNote": "..."}}}`。**绝不**写入 Mod 自己的 `manifest.json`。
- 默认分类已升级为自动识别：`StardewAnxiPanel.Control` 默认为 `server_only`，内容包和其他第三方 Mod 默认为 `client_required`；手动写入 `mod-sync.json` 的分类优先。
- `GetModSyncClassification` / `SetModSyncClassification`：单个 Mod 的读写。存储里出现非法 `syncKind`（比如手动改坏文件）会被忽略，回退到默认值，不会让接口 500。
- `ApplyModSyncClassification(dataDir, mods []registry.ModInfo) []registry.ModInfo`：批量补全分类，`handleModsList`（`GET /api/instances/:id/mods`）已经在返回前调用它，所以**前端不需要为了拿 syncKind 单独再请求一次**。
- `BuildModSyncPlan(dataDir)`：给 `sync-plan` 接口用，顺带算出统计。
- `ResolveModFolder(dataDir, modID)`：modID 可以是文件夹名或 manifest `UniqueID`，查找顺序复用了 `DeleteMod` 的逻辑。
- `ExportModSyncPackZip(dataDir)`：

  1. 取全部已装 Mod 并补全分类。
  2. 过滤出 `syncKind == client_required` 且文件夹名不是 `StardewAnxiPanel.Control`（双重保险：默认分类是 server_only，这里又强制再排除一次，哪怕有人手贱把控制 Mod 标成 client_required 也导不出去）。
  3. 一个都没有时返回哨兵错误 `ErrNoSyncMods`，handler 据此返回 `400 no_sync_mods`，不走会吞掉错误文案的 `sanitizeErrorMsg`。
  4. 复用 `mods.go` 里新抽出来的 `addModDirToZip(w, root, dirName)` 写入每个 Mod 目录（这个函数是从原 `ExportModsZip` 里抽出来的，行为完全一致，`ExportModsZip` 调用方式不变）。
  5. 历史第一阶段曾写 `player-sync-manifest.json` 到 ZIP 根目录；`PLAYERSYNC-PACK-2` 后该契约已升级为 `pack-manifest.json` + `payload/` + 安装脚本。

### API

新增 3 个接口，路由加在 `backend/internal/web/instance_handlers.go`（`mods/export` 之后、`DELETE mods/:modId` 之前），handler 实现在 `backend/internal/web/lifecycle_handlers.go`：

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/instances/:id/mods/sync-plan` | `requireAuth`（任意登录用户） | 返回 `ModSyncPlanResult` |
| PUT | `/api/instances/:id/mods/:modId/sync-classification` | `requireAuth` | body `{syncKind, syncNote?}`；只写面板元数据，**不受服务器运行状态限制**；成功会写审计日志 `mod_sync_classification_update` |
| POST | `/api/instances/:id/mods/sync-pack/export` | `requireAuth`（任意登录用户，玩家也要能下载） | 流式返回 ZIP；**服务器运行中也允许导出** |

`PUT` 接口现已改为 `requireAuth`，普通登录用户也能修正同步分类；`GET sync-plan` 和 `POST sync-pack/export` 同样用 `requireAuth`，和现有 `handleModsExport` 一个级别。

### 测试

新文件 `backend/internal/games/stardew_junimo/mod_sync_test.go`，覆盖：

- 分类文件读写往返（`SetModSyncClassification` → `GetModSyncClassification`，确认落盘路径在 `.local-container/control/mod-sync.json`，且不改 `manifest.json`）。
- 默认分类：普通第三方 Mod 默认 `client_required`，`ContentPackFor` 内容包默认 `client_required`，`StardewAnxiPanel.Control` 默认 `server_only`；存储里出现非法值时也回退默认。
- `ResolveModFolder` 按文件夹名/UniqueID 查找，以及拒绝 `../evil`、`foo/bar`、绝对路径等路径穿越输入。
- `ExportModSyncPackZip`：只含 `client_required` Mod、强制排除控制 Mod（即使手动标成 client_required）、无可导出 Mod 时报错、manifest 内容正确、ZIP 内条目不含绝对路径。

验证：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
```

全部通过（含既有用例，确认新增的恒定 `syncKind` 字段没有破坏旧断言）。

## 已修复的遗留问题

`mods.go` 的 `ExportModsZip` 原先在写 ZIP 的循环失败、但后续 `w.Close()` 恰好成功时，会因为 `if err := w.Close(); err != nil` 这种局部变量遮蔽外层 `err`，把循环里的真实失败吞掉、错误地返回成功。本次已修复：循环内失败直接 `return`，`w.Close()`/`zf.Close()` 改成赋值给外层 `err`（不再用 `:=` 遮蔽），失败时统一走函数顶部的 `defer` 清理临时文件。修复后行为与新写的 `ExportModSyncPackZip` 一致。`go test ./...` 全量通过。

## 代码评审修复（同一天补充）

评审发现 `mod_sync.go` 两处并发问题，已修复：

- **P1：`ExportModSyncPackZip` 临时文件名固定**（原 line 218）。原来固定写 `%TEMP%\stardew-player-sync-pack.zip`，两个用户同时导出会互相覆盖/截断 ZIP，一个请求失败时的 `defer` 清理还可能删掉另一个请求正在 `ServeFile` 的文件。改为 `os.CreateTemp("", "stardew-player-sync-pack-*.zip")` 拿到唯一磁盘路径；新增导出常量 `PlayerSyncPackFileName = "stardew-player-sync-pack.zip"`，`handleModSyncPackExport`（`lifecycle_handlers.go`）的 `Content-Disposition` 改用这个固定常量而不是 `filepath.Base(zipPath)`，所以浏览器侧下载文件名依旧稳定、可预期，跟磁盘上的随机文件名完全解耦。
- **P2：`SetModSyncClassification` 无锁读改写**（原 line 99）。两个几乎同时的 `PUT sync-classification` 请求都先读到旧的 `mod-sync.json`，后写入的会覆盖先写入的改动（前端只在 `ModsPage.tsx` 里禁用了正在操作的那一个下拉框，快速点击或两个管理员同时操作都能触发）。修复：新增按 `dataDir` 维度的锁注册表 `modSyncLocks`/`modSyncLockFor`（`map[string]*sync.Mutex` + 一把保护该 map 的 `modSyncLocksMu`），`SetModSyncClassification` 在整个 load→modify→save 过程中持有对应 `dataDir` 的锁；同时把 `saveModSyncStore` 从直接 `os.WriteFile` 改成「写同目录临时文件 `.mod-sync-*.json.tmp` → `os.Rename` 覆盖目标路径」的原子写入，避免进程中途崩溃留下半截 JSON。

涉及文件：`backend/internal/games/stardew_junimo/mod_sync.go`、`backend/internal/web/lifecycle_handlers.go`。

验证：`go build ./...`、`go vet ./...`、`go test ./...` 全量通过（含既有 `mod_sync_test.go` 用例）。

不涉及 SMAPI mod DLL 或 Junimo 容器内文件，不需要重启 Stardew server 容器。

## 下一步注意事项

- 这次改动只涉及面板元数据和 ZIP 导出逻辑，不碰 SMAPI mod DLL 或 Junimo 容器内任何文件，**不需要重启 Stardew server 容器**。
- 新增 Mod 时会先走 `stardew_junimo` 自动推断默认分类；用户仍可手动覆盖。
- 新增接口已同步更新 `docs/02-backend.md` 和 `docs/06-integration.md`。
# SMAPI-RUNTIME-1 后端交接

## 改了什么
- `registry.ModInfo` 新增 `BuiltIn bool`，JSON 字段为 `builtIn`。
- `ListMods(dataDir)` 在检测到 `StardewAnxiPanel.Control` 目录时，会把虚拟 `SMAPI` 条目插入到列表第一位；该条目 `UniqueID=Pathoschild.SMAPI`、`BuiltIn=true`、`SyncKind=client_required`。
- `FindModByUniqueID` 跳过内置条目；`ApplyModSyncClassification` 保留内置条目已有分类；`BuildModSyncPlan` 和 `ExportModSyncPackZip` 的统计/导出都跳过内置条目。

## 影响接口/文件
- 接口：`GET /api/instances/:id/mods` 的 `mods[]` 可能新增置顶虚拟条目。
- 文件：`backend/internal/games/registry/types.go`、`backend/internal/games/stardew_junimo/mods.go`、`mod_sync.go` 及对应测试。

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。

## 下一步注意事项
- SMAPI 条目不是磁盘目录，不要让删除、分类更新、整包导出或玩家同步包逻辑把它当作普通 Mod。
- 如果以后能读取真实 SMAPI 版本，可只补 `Version` 字段，不要改变 `builtIn` 契约。
# NEXUS-PAGED-1 后端交接

## 改了什么
- `SearchNexusModsPage(ctx, query, apiKey, page, pageSize)` 新增分页入口，旧 `SearchNexusMods` 保留为第一页兼容包装。
- Nexus GraphQL v2 关键词搜索新增 `sort: [{ downloads: { direction: DESC } }]`、`offset`、`count`，响应解析 `totalCount` 并返回 `page/pageSize/total/hasMore`。
- `handleModNexusSearch` 读取 `page/pageSize` 查询参数后直接调用 Nexus 分页搜索。模组页不再依赖统一搜索接口。

## 影响接口/文件
- 接口：`GET /api/instances/:id/mods/nexus/search?q=...&page=...&pageSize=...`。
- 文件：`backend/internal/games/stardew_junimo/nexus.go`、`backend/internal/web/lifecycle_handlers.go`、`backend/internal/games/stardew_junimo/nexus_test.go`。

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。
- 额外确认：实时 Nexus GraphQL schema introspection 显示 `mods` 支持 `sort: [ModsSort!]`、`offset`、`count`。

## 下一步注意事项
- `/mods/search` 统一搜索兼容路由已撤下，`mod_search.go` 和 `mod_search_test.go` 已删除；后续如果再做多来源搜索，需要重新设计接口，不要误以为当前还有统一搜索后端契约。

# SMAPI-SYNC-2 后端接手记录

## 改了什么

- `StardewAnxiPanel.Control` 真实 Mod 目录现在会被 `ListMods` 标为 `builtIn=true`、`syncKind=server_only`。
- `SMAPI` 虚拟条目继续由 Control 存在时注入；`PLAYERSYNC-PACK-2` 后导出玩家同步包时会写入 `pack-manifest.json`，作为玩家客户端必须安装 SMAPI 的清单项。
- 玩家同步 ZIP 仍不会打包 SMAPI 文件，也永远不会打包 `StardewAnxiPanel.Control`；Manifest 条目新增 `builtIn` 与 `packaged` 字段。
- `DeleteMod` / `deleteModFolder` 增加 Control 删除保护，直接按 folderName 删除也会失败。

## 影响文件/接口

- `backend/internal/games/stardew_junimo/mods.go`
- `backend/internal/games/stardew_junimo/mod_sync.go`
- `GET /api/instances/:id/mods`
- `GET /api/instances/:id/mods/sync-pack`
- `DELETE /api/instances/:id/mods/:id`

## 如何验证

- `go test ./internal/games/stardew_junimo ./internal/web`

## 下一步注意事项

- 前端或后续玩家同步安装器不要把 `builtIn=true` 统一理解为“不同步”：SMAPI 是 `builtIn=true` 但 `packaged=false` 的玩家前置要求；Control 才是完全排除的服务端内置组件。

# PLAYERSYNC-PACK-11 后端接手记录

## 改了什么
- 玩家同步包安装脚本恢复动态进度条，但动态刷新行只显示 ASCII 内容，例如 `[============                ]  42% CHECK`。
- `Show-InstallProgress` 仍把中文详细状态写入日志；控制台 `Render-InstallProgressLine` 只按百分比映射 `START/LOCATE/CHECK/SMAPI/MODS/STEAM/RECORD/DONE`。
- `Write-Step` 打印中文任务日志前清空动态行，打印后重绘动态行；进入 Steam 启动项脚本、人工目录输入、失败恢复提示前也会清空动态行。
- 继续禁用 PowerShell `Write-Progress`，不再使用它的进度流渲染。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 已热修。

## 影响文件/接口
- 接口不变：`POST /api/instances/:id/mods/sync-pack/export`。
- 文件：
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`
  - `docs/02-backend.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo -run "PlayerSync|ExportModSyncPack"`。
- 已执行当前解压包 PowerShell parser 验证，确认 `install.ps1` 语法通过且不含 `Write-Progress`。
- 已在 `D:\steam\steamapps\common\Stardew Valley` 真实运行热修脚本，中文任务行不再重叠；Codex 捕获环境为重定向输出，会自动隐藏动态条，普通 bat 终端会显示 ASCII 动态行。

## 下一步注意事项
- 后续如果再改进度显示，动态刷新行必须保持 ASCII-only，不要把中文 `$Status` 或 Mod 名直接放进去。
- 任何会输出中文的 `Write-Host` 或外部脚本调用前，先 `Clear-InstallProgressLine`。
# PLAYERSYNC-PACK-12 后端接手记录

## 改了什么
- 玩家同步包安装脚本新增 `Write-LogLine`，统一处理安装日志写入。
- 原先普通步骤和进度 tick 直接 `Add-Content -Path $LogPath`；在 `$ErrorActionPreference = "Stop"` 下，日志文件被短暂占用会让整个安装失败。
- 新实现使用 `System.IO.File.AppendAllText` 写日志，最多短重试 5 次；仍失败则跳过该条日志，不影响 SMAPI、Mod 安装或 Steam 启动项流程。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 已热修。

## 影响文件/接口
- 接口不变：`POST /api/instances/:id/mods/sync-pack/export`。
- 文件：
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`
  - `docs/02-backend.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo -run "PlayerSync|ExportModSyncPack"`。
- 已执行当前解压包 PowerShell parser 验证，确认 `install.ps1` 不再直接用 `Add-Content -Path $LogPath`。
- 已在 `D:\steam\steamapps\common\Stardew Valley` 真实运行热修脚本，安装通过。

## 下一步注意事项
- 安装日志是辅助诊断信息，绝不能阻断玩家安装流程。
- 后续新增日志写入都走 `Write-LogLine`，不要回到直接 `Add-Content`。
# PLAYERSYNC-PACK-13 后端接手记录

## 改了什么
- 玩家同步包安装脚本和 Steam 启动项辅助脚本给启动项相关输出加颜色。
- `Steam 启动项文本：` / `建议 Steam 启动项：` 使用 `-ForegroundColor Yellow`。
- 真正需要复制的 launch options 使用 `-ForegroundColor Cyan`，并保持独立一行、不加前缀，避免复制时带上多余字符。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 和 `tools\steam-launch-options.ps1` 已热修。

## 影响文件/接口
- 接口不变：`POST /api/instances/:id/mods/sync-pack/export`。
- 文件：
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`
  - `docs/02-backend.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo -run "PlayerSync|ExportModSyncPack"`。
- 已执行当前解压包 PowerShell parser 验证。
- 已在 `D:\steam\steamapps\common\Stardew Valley` 真实运行热修脚本，输出流程正常；Codex 捕获环境不保留颜色，玩家终端会显示颜色。

## 下一步注意事项
- 启动项文本行要继续保持纯 launch options，不要为了视觉效果加引导符号、箭头或额外说明，避免玩家复制错误。
# PLAYERSYNC-PACK-14 后端接手记录

## 改了什么
- 玩家同步包安装完成摘要在 `Steam 启动项文本：` 后新增 `请复制到 Steam 的游戏启动项中。`。
- Steam 启动项辅助脚本在 `建议 Steam 启动项：` 后也输出同一提示。
- 可复制的 launch options 仍保持独立一行并使用 Cyan，不和提示文字拼在一起。
- 当前测试解压包 `C:\Users\anxi\Downloads\stardew-player-sync-pack\tools\install.ps1` 和 `tools\steam-launch-options.ps1` 已热修。

## 影响文件/接口
- 接口不变：`POST /api/instances/:id/mods/sync-pack/export`。
- 文件：
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`
  - `docs/02-backend.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo -run "PlayerSync|ExportModSyncPack"`。
- 已执行当前解压包 PowerShell parser 验证。
- 已在 `D:\steam\steamapps\common\Stardew Valley` 真实运行热修脚本，确认输出顺序为标题、复制提示、独立启动项文本。

## 下一步注意事项
- 启动项文本行继续保持纯 launch options，不要和提示文字合并。
# PLAYERSYNC-PACK-15 后端接手记录

## 改了什么
- 新增“不带 SMAPI 的模组更新包”导出链路，给已经运行过完整版同步包的玩家使用。
- `ExportModSyncPackZip` 保持完整版行为；新增 `ExportModSyncUpdatePackZip`，复用导出核心但 `packType=mods_update`，不写 `payload/smapi/`，也不把 SMAPI ZIP 写入 checksum。
- 玩家端安装脚本识别 `mods_update` 后不安装 SMAPI，只检查游戏目录已有 `StardewModdingAPI.exe`；缺失时提示先运行完整版玩家同步包。
- 更新包仍使用现有目录指纹逻辑：相同 Mod 跳过，不备份；内容不同才备份并覆盖。

## 影响文件/接口
- 新增接口：`POST /api/instances/:id/mods/sync-pack/export-update`，下载名 `stardew-player-mods-update-pack.zip`。
- 旧接口不变：`POST /api/instances/:id/mods/sync-pack/export`，下载名 `stardew-player-sync-pack.zip`。
- 文件：
  - `backend/internal/games/stardew_junimo/mod_sync.go`
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`
  - `backend/internal/web/instance_handlers.go`
  - `backend/internal/web/lifecycle_handlers.go`
  - `docs/02-backend.md`
  - `docs/06-integration.md`
  - `docs/08-future-roadmap.md`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。
- 已执行：`npm.cmd run build`。

## 下一步注意事项
- 更新包不适合首次加入玩家；首次加入仍发完整同步包。
- 不要为更新包引入登录用户导出历史。客户端安装脚本的指纹比对已经能跳过完全相同的 Mod，并覆盖更新内容不同的 Mod。

# PLAYERSYNC-PACK-16 后端接手记录

## 改了什么
- 模组更新包安装脚本不再尝试配置 Steam 启动项，更新包 ZIP 也不再包含 `tools/steam-launch-options.ps1`。
- `packType=mods_update` 时，脚本只检查已有 `StardewModdingAPI.exe`、安装/跳过/更新 Mod，并把 Steam 结果记录为 `reason=mods_update_pack`。
- 更新包最终摘要不再输出 `Steam 启动项文本`；只显示 `Steam 启动项：已跳过，沿用已有设置`。
- 完整同步包仍保留 `steam-launch-options.ps1` 自动配置与手动复制提示。

## 影响文件/接口
- 接口不变：`POST /api/instances/:id/mods/sync-pack/export-update` 仍下载 `stardew-player-mods-update-pack.zip`。
- 文件：
  - `backend/internal/games/stardew_junimo/mod_sync_pack_scripts.go`
  - `backend/internal/games/stardew_junimo/mod_sync_test.go`
  - `docs/02-backend.md`
  - `docs/06-integration.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。
- PowerShell parser 测试覆盖 `install.ps1`，静态测试覆盖更新包跳过 Steam 分支。

## 下一步注意事项
- 更新包默认用于已经跑过完整版同步包的玩家；不要在该分支恢复 Steam `localconfig.vdf` 写入。
- 如果玩家首次加入或缺少 Steam 启动项，仍应发完整版同步包。

# MODPROFILE-1 后端接手记录

## 改了什么
- 新增 `.local-container/mods-disabled/` 和 `.local-container/control/mod-profiles.json`，按存档保存 Mod 启用状态。
- `GET /api/instances/:id/mods` 合并启用与禁用目录，并返回 `enabled/canToggle/enableNote`。
- 新增 `PUT /api/instances/:id/mods/:modId/enabled`，管理员停服状态下可切换当前存档 Mod。
- 启动前应用当前存档 profile；新建/新导入存档默认写入 `defaultEnabled=false`，除内置组件外全部禁用。
- 删除和重复 UniqueID 检查同时扫描启用与禁用目录。

## 影响文件/接口
- `backend/internal/games/stardew_junimo/mod_profiles.go`
- `backend/internal/games/stardew_junimo/mods.go`
- `backend/internal/games/stardew_junimo/lifecycle.go`
- `backend/internal/web/instance_handlers.go`
- `backend/internal/web/lifecycle_handlers.go`
- `GET /api/instances/:id/mods`
- `PUT /api/instances/:id/mods/:modId/enabled`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。
- 已执行：`npm.cmd run build`。

## 下一步注意事项
- 第一阶段只支持停服切换；运行中热切换要另做 SMAPI/容器侧一致性设计。
- 新安装 Mod 如果当前存档 profile 是 `defaultEnabled=false`，下一次启动会按 profile 禁用，前端展示以 `enabled` 为准。
# MODPROFILE-2 后端接手记录

## 改了什么
- 补充 `ApplyModProfile` 跨存档切换测试，确认同一组 Mod 会按不同存档 profile 在启用目录和禁用目录之间移动。
- 后端选择存档路径已有 `ApplyModProfile` 调用，本次主要用测试钉住行为，实际用户可见问题在前端缓存刷新。

## 影响文件/接口
- `backend/internal/games/stardew_junimo/mod_profiles_test.go`
- `POST /api/instances/:id/saves/select`
- `POST /api/instances/:id/saves/select-and-start`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。
- 已执行：`npm.cmd run build`。

## 下一步注意事项
- 如果后续新增其它能改变 active save 的接口，也要保证前端随之刷新 mods，后端则继续在启动或切换前调用 `ApplyModProfile`。

# NEXUS-DEFAULT-1 后端接手记录

## 改了什么
- `SearchNexusModsPage` 允许空查询，空 `q` 会走 Nexus GraphQL v2 默认列表。
- 新增 `nexusSearchPopularPage`，只按 `gameDomainName=stardewvalley` 过滤并按 `downloads DESC` 排序，分页参数继续下推到 Nexus。
- `handleModNexusSearch` 移除空查询 400 拦截。

## 影响文件/接口
- `backend/internal/games/stardew_junimo/nexus.go`
- `backend/internal/games/stardew_junimo/nexus_test.go`
- `backend/internal/web/lifecycle_handlers.go`
- `GET /api/instances/:id/mods/nexus/search`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。
- 已执行：`npm.cmd run build`。

## 下一步注意事项
- 当前“近期热门”使用 Nexus GraphQL 列表按下载量降序返回；如果后续要严格按时间窗口做“近期”，需要先确认 Nexus GraphQL 是否提供稳定的时间范围过滤或 trending 排序字段。
# NEWGAME-CABINS-1 后端接手记录

## 改了什么
- 检查实际实例配置：当前 `data/instances/stardew/.local-container/settings/server-settings.json` 和 `control/server-init.json` 都记录了 `1` 间小屋；当前活跃存档 XML 中能数到 Cabin building，但坐标为 `-20/-20`，说明“配置没写入”不是唯一问题，仍需关注 Junimo/游戏摆放建筑的结果。
- `startingCabins` 后端校验从 0-3 改为 0-7，并补测试断言 `Game.StartingCabins` 和 `server-init.json.cabinCount`。
- `WriteInitConfig` 不再因为缺少角色外观字段而跳过，确保 `cabinCount/cabinLayout` 总会给控制模组。
- `WriteServerSettings` 会写 `.local-container/control/new-game-pending` 一次性标记；`SetActiveSave` 会清理该标记，避免切回既有存档时误用新建参数。
- `StardewAnxiPanel.Control` 新增 `ApplyPendingNewGameWorldOptions()`，仅当 `new-game-pending` 存在时在 Junimo 新建存档前同步 `Game1.startingCabins`、`Game1.cabinsSeparate`、农场类型、利润率和钱包模式；存档加载后删除标记。
- 已审掉 Control 模组历史原生创建存档路径：移除 `TryStartNativeSaveCreate`、`StartNativeCreate`、`FindConfiguredSaveFolder`、`DetectNewSaveFolder` 和相关状态字段；不再订阅 `SaveCreated` 来识别原生创建结果。新写出的 `server-init.json.mode` 为 `panel-newgame`，仅保留旧 `native-create` 字符串兼容读取。
- 已重编并更新嵌入 DLL。

## 影响文件/接口
- 接口仍是 `POST /api/instances/:id/saves/custom-new-game`，但 `startingCabins` 契约明确为 0-7。
- 文件：
  - `backend/internal/games/registry/types.go`
  - `backend/internal/games/stardew_junimo/saves.go`
  - `backend/internal/games/stardew_junimo/saves_test.go`
  - `backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ModEntry.cs`
  - `backend/internal/games/stardew_junimo/embedded/smapi-mod/StardewAnxiPanel.Control.dll`
  - `docs/02-backend.md`
  - `docs/06-integration.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo -run "WriteServerSettings|WriteInitConfig|ValidateNewGameConfig"`。
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。
- 已执行：Docker 编译 SMAPI mod：`dotnet build -c Release /p:GamePath=/game`，成功但有 ModBuildConfig analyzer 编译器版本 warning。

## 下一步注意事项
- 已运行实例目录中的控制模组 DLL 需要重启/重新准备容器后才会拿到新的嵌入 DLL。
- 如果玩家仍看到少于配置的小屋，下一步要用新建测试存档同时核对 `server-settings.json`、`server-init.json`、`new-game-pending` 清理时机、SMAPI status/log 和存档 XML 中 Cabin building 坐标，区分“数量未创建”和“创建在不可见坐标”。


# SAVE-BACKUP-POLICY-1 ????

- ?????`backend/internal/games/stardew_junimo/saves.go`?`saves_test.go`?`backend/internal/web/instance_handlers.go`?`lifecycle_handlers.go`?SMAPI Control `ModEntry.cs` / `ControlContract.cs` ? embedded DLL?
- ???????? `POST /api/instances/:id/saves/:name/backup`??? `GET|PUT /api/instances/:id/saves/backups/policy`????????? `policy` ? `maintenance`?
- ??????????????? `.local-container/control/save-events/`?`RunBackupMaintenance` ???????????????????????? daily ???
- ?????????????????????? goroutine??????????????????????????? lifecycle ??? scheduler?
- ???`go test ./internal/games/stardew_junimo ./internal/web` ???Docker dotnet build ????? SMAPI analyzer ????? warning?

# SAVE-BACKUP-SCHEDULE-HOUR-1 后端接手记录

## 改了什么
- `BackupPolicy` 的定时备份配置从 `scheduledIntervalHours` 改成 `scheduledHour`，默认 4，范围 0-23。
- 旧 `scheduledIntervalHours` 字段只做兼容读取，策略保存后不再写回。
- 定时备份维护逻辑改为按本地自然日判断：到达配置整点后每天最多执行一次，生成/覆盖 `scheduled_<save>.zip`。
- 补充 `TestScheduledBackupRunsOncePerDayAtConfiguredHour`。

## 影响文件/接口
- `backend/internal/games/stardew_junimo/saves.go`
- `backend/internal/games/stardew_junimo/saves_test.go`
- `GET|PUT /api/instances/:id/saves/backups/policy` 的字段以 `scheduledHour` 为准。

## 如何验证
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。

## 下一步注意事项
- 这个维护流程仍由读取备份列表等入口触发，不是后台常驻 scheduler；如果后续做真正后台定时器，要复用同一套每日一次判断。

# SERVER-SAY-1 后端接手记录

## 改了什么
- `POST /api/instances/:id/commands/say` 从固定返回 `command_not_supported` 改为写入面板控制命令文件。
- 后端仍校验服务器必须 running、消息非空、最长 200 字符，并把所有控制字符折叠为空格，避免换行注入其它命令。
- 写入位置为 `.local-container/control/commands/<timestamp>-<id>.json`，内容为 `broadcast` 命令和 `payload.message`。
- `StardewAnxiPanel.Control` 的 `broadcast` 分支现在会在世界 ready 时调用 Stardew multiplayer chat message，向所有玩家发送 `[Panel] <message>`。
- 控制模组发送前会再次清理控制字符，并按 CJK/Kana/Hangul/Cyrillic/Thai 粗略选择聊天语言码，降低中文等非拉丁字符显示为方块的概率。
- 已重新编译并更新嵌入 DLL。

## 影响文件/接口
- `POST /api/instances/:id/commands/say` 成功时返回 `CommandRunResult{ command: "say", exitCode: 0 }`。
- 文件：
  - `backend/internal/games/stardew_junimo/console.go`
  - `backend/internal/games/stardew_junimo/console_test.go`
  - `backend/internal/games/stardew_junimo/embedded/smapi-mod-src/ModEntry.cs`
  - `backend/internal/games/stardew_junimo/embedded/smapi-mod/StardewAnxiPanel.Control.dll`
  - `docs/02-backend.md`
  - `docs/06-integration.md`
  - `docs/08-future-roadmap.md`
  - `docs/backend-handoff/backend-handoff-2026-07-01.md`

## 如何验证
- 已确认上游当前 master（`e5fdae0`，2026-06-22）和当前已验证镜像 `sdvd/server:1.5.0-preview.121` 对应 commit（`8efaf90`，2026-06-18）都包含 `/ws` + `chat_send` relay，但没有 `say` 控制台命令。
- 已执行：`go test ./internal/games/stardew_junimo -run "SendSay|RunCommand|ListCommands|SanitizeSay"`。
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。
- 已执行：Docker 编译 SMAPI mod：`dotnet build -c Release /p:GamePath=/game`，成功但仍有 ModBuildConfig analyzer 编译器版本 warning。

## 下一步注意事项
- 已运行实例目录中的控制模组 DLL 需要重启/重新准备 server 容器后才会刷新；新启动流程会调用 `installSMAPIMod`。
- 面板暂不直接接上游 WebSocket `chat_send`，避免面板容器网络、API key 和部署形态差异导致喊话不可用。后续如接 WS，应保留 Control Mod 文件通道作为 fallback。
# SCHEDULED-RESTART-1 后端接手记录

## 改了什么
- 新增计划重启/维护窗口后端：管理员可保存每天关闭时间和开启时间。
- 新增 SQLite 表 `restart_schedules` 和迁移 `006_restart_schedules.sql`。
- 新增后台 `RestartScheduler`，随 `cmd/panel` 启动并每 30 秒轮询启用计划。
- 调度器提前调用现有 `SendSay` 广播，关闭前可调用 `BackupSave` 备份当前 active save，然后复用 Stardew driver `Stop` / `Start` 生命周期 job。

## 影响文件/接口
- `backend/internal/storage/restart_schedules.go`
- `backend/internal/web/restart_schedule_handlers.go`
- `backend/internal/web/instance_handlers.go`
- `backend/cmd/panel/main.go`
- `backend/migrations/006_restart_schedules.sql`
- 新接口：`GET|PUT /api/instances/:id/restart-schedule`

## 如何验证
- 已执行：`cd backend; go test ./internal/storage ./internal/web ./cmd/panel`。
- 手动联调建议：把关闭时间设置到当前时间后 1-2 分钟，启动服务器后观察提醒 JSON、stop job 和 `lastStatus`；再把开启时间设置到停止后 1-2 分钟，观察 start job。

## 下一步注意事项
- 关闭前备份不是强制保存世界，只备份已经写盘的 active save。
- 计划开启只在到点后 5 分钟宽限内触发，避免补跑旧开启时间。
- 后续如要做“跳过下一次”，需要新增持久化字段和 API；当前只实现每日窗口。
