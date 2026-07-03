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
- `PLAYERSYNC-PACK-5` completed：修复玩家同步包安装 SMAPI 4.5.2 时找不到 `StardewModdingAPI.exe` 的真机问题。脚本不再调用交互式 SMAPI Windows 安装器或猜测静默参数，改为从官方 ZIP 中解出 `internal/windows/install.dat`，复制为临时 zip 后释放官方 payload 到游戏目录。已热修复当前测试解压包，并在 `D:\steam\steamapps\common\Stardew Valley` 用 `-SkipSteamLaunchOptions` 真实安装验证通过。
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
- `FE-PROTOTYPE-LAYOUT-1` completed：前端主要 Stardew 页面已按 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30` 的信息架构重新排布。总览页对齐农场横幅、生命周期控制、邀请码、摘要指标和三列摘要；存档页新增当前激活存档重点卡；服务器、任务、玩家、模组、诊断、设置页通过页面级布局 class 调整为原型式分区。现有 API 和功能不变，`ModsPage` 保留三段式工作台。
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
- `FE-MAIN-PAGE-FRAME-1` completed：所有 Stardew 路由的中间主内容区 `.sd-main` 已统一替换为 image2 存档页空框背景 `main_page_frame_empty_image2.png`。资源从 `docs/prototypes/.../03-saves-page-frame-empty-image2.png` 复制到 `frontend/public/assets/stardew/ui/panels/` 供运行时和 Docker 静态发布使用；主内容背景改为居中、不重复、`100% 100%` 铺满，并把页面整体 padding 调整为 `clamp(28px, 2.4vw, 42px)` 避免压到木框角饰。主内容区仍保留 `overflow-y: auto`，但已隐藏 Firefox/Chromium/WebKit 原生滚动条，避免白色竖条压住右侧 frame 边框。已验证前端构建通过，生产 CSS 临时 Shell QA 页在 1280x720 和 390x760 下背景加载、滚动条隐藏、滚动能力保留、无横向溢出、console error/warn 为空。
# FE-MODS-DYNAMIC-PAGESIZE-1 状态
- `FE-MODS-DYNAMIC-PAGESIZE-1` completed：模组下载页 Nexus 搜索结果已改为固定搜索卡片高度 + 动态 pageSize。`.sd-mods-nexus-search-list` 卡片高度固定 `246px`，前端按真实结果网格在 `.sd-main-scroll` 内的可见高度和实际列数计算每页数量，并传给既有 Nexus 搜索 API 的 `pageSize`；加载骨架不参与测量，避免 loading/结果态来回触发刷新；顶部翻页器同步显示“每页 N 个”，底部重复翻页器移除。已验证 `cd frontend; npm.cmd run build` 通过，并用临时本地 QA 页面确认 1040x1120 为 pageSize=4、1040x720 为 pageSize=2、520x720 为 pageSize=1，卡片高度均为 `246px`。
