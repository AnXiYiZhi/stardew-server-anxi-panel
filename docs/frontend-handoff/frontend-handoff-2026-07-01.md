# FE-CLEANUP-1 旧前端组件与死封装清理

## 改了什么
- 删除无 JSX 引用的旧 Stardew 分段组件：`ConsoleSection.tsx`、`DockerSection.tsx`、`InstallSection.tsx`、`JobsSection.tsx`、`LifecycleSection.tsx`、`ModsSection.tsx`。当前 Stardew 面板只维护 `pages/*` 路由页；`SavesSection.tsx` 仍被 `pages/SavesPage.tsx` 使用，保留。
- 清理 `frontend/src/api.ts` 中无调用者的旧封装：`getComposeLogs()`、`startTestJob()`、`startFailingTestJob()`、`uploadMod()`、`getModSyncPlan()`。
- 同步删除仅服务这些旧封装的前端类型：`ComposeLogsResponse`、`ModSyncSummary`、`ModSyncPlanResult`。
- 文档同步修正为当前 Nexus-only 搜索/安装事实：旧 `/mods/search` 统一搜索骨架已撤回；Nexus 安装和按存档启用/禁用已接入，依赖缺失检查、更新检查、SMAPI 配置编辑仍是后续。

## 影响文件/接口
- 删除文件：`frontend/src/games/stardew/{ConsoleSection,DockerSection,InstallSection,JobsSection,LifecycleSection,ModsSection}.tsx`
- 修改文件：`frontend/src/api.ts`、`frontend/src/types.ts`
- 前端调用接口不变；后端开发/诊断接口如 `/api/jobs/test`、`/api/docker/logs` 本次未删除。

## 如何验证
- 执行：`cd frontend; npm.cmd run build`
- 执行：`cd backend; go test ./...`

## 下一步注意事项
- 新增 Stardew 页面时优先放到 `frontend/src/games/stardew/pages/`，不要恢复旧 Section 组件体系。
- 如未来重新接多来源 Mod 搜索，需要重新设计接口，不要复用已经撤回的 `/mods/search` 契约。

# FE-QUICK-BACKUP-1 前端交接

## 改了什么
- 服务器控制页“快捷操作”中的“备份存档”从待接入按钮改为真实操作。
- 点击后调用 `createSaveBackup(activeSaveName)`，为当前激活存档创建手动备份，并在本区显示备份文件名。
- 按钮文案为“备份已保存进度”，仅管理员可用；没有激活存档时禁用。它不要求服务器停止，语义与存档页“手动备份”保持一致，但只备份已经落盘的存档目录，不会强制保存游戏内实时进度。
- 原“保存世界 / 立即保存”占位已从快捷操作移除；Stardew 当前可靠保存点是游戏内保存事件，面板尚无强制立即保存 API，前端不要展示一个无法执行的保存按钮。

## 影响文件/接口
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- 复用接口：`POST /api/instances/:id/saves/:name/backup`

## 如何验证
- 已执行：`cd E:\stardew-server-anxi-panel\frontend; npm.cmd run build`。
- 手动联调建议：管理员进入服务器控制页，确认有激活存档时“备份存档”可点击，成功后显示 `manual_*.zip` 或后端返回的备份名；普通用户或无激活存档时按钮禁用。

## 下一步注意事项
- 后续若要实现“立即保存”，必须先确认 Junimo/SMAPI 控制侧是否能可靠触发 Stardew 保存，不要只在 API 层伪造成功；实现前不要把该入口放回快捷操作。

# FE-SAVE-START-NAV-1 前端交接

## 改了什么
- 存档页里“选择并启动 / 使用此存档启动 / 创建存档并启动 / 上传存档并启动”成功创建启动任务后，不再跳到任务页，而是跳到总览页。
- 跳转前会调用 `dashboardData.requestInviteCodeRefresh()`，让总览页立即显示启动等待态，并持续等新邀请码出现。
- 任务列表仍调用 `dashboardData.refreshJobs()` 刷新；用户需要看详细日志时仍可从总览或侧栏进入任务页。

## 影响文件/接口
- `frontend/src/games/stardew/pages/SavesPage.tsx`
- 接口未变，继续使用存档页原有启动接口。

## 如何验证
- 已执行：`cd E:\stardew-server-anxi-panel\frontend; npm.cmd run build`。
- 手动联调建议：停服状态点击某个存档的“选择并启动”，页面应进入 `/instances/stardew/overview`，启动按钮保持旋转 `启动中…`，新邀请码出现后显示停止/重启。

## 下一步注意事项
- 不要把存档启动页重新导向 jobs；任务页适合看日志，但启动后用户第一需求是看服务器状态和邀请码。

# FE-LIFECYCLE-WAIT-1 前端交接

## 改了什么
- 启动和重启不再在接口请求完成后立刻露出停止按钮；总览页与服务器控制页都会保持带旋转圆圈的 `启动中…` 按钮，直到共享数据层拿到新的 `inviteCode`。
- 停止也有同样的等待态：点击停止后保留带旋转圆圈的 `停止中…`，直到后端状态完全停止后才恢复启动按钮。
- `useStardewDashboardData` 暴露 `inviteCodeRefreshing`，复用已有“丢弃旧邀请码并轮询新邀请码”的逻辑，没有新增 API。
- 本地页面各自保留 `pendingStartupAction`，覆盖接口请求尚未返回的短暂窗口；请求失败或存档缺失时会清掉等待态并显示原有错误/引导。

## 影响文件/接口
- `frontend/src/games/stardew/useStardewDashboardData.ts`
- `frontend/src/games/stardew/stardew-routes.ts`
- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/games/stardew/stardew-theme.css`
- 接口未变，继续使用 `POST /start`、`POST /restart`、`GET /invite-code`。

## 如何验证
- 已执行：`cd E:\stardew-server-anxi-panel\frontend; npm.cmd run build`。
- 当前桌面环境启动 Vite 时 `listen EACCES`，未能做浏览器渲染点击验证。
- 手动联调建议：在停服状态点击启动，确认按钮立即变为旋转圆圈 `启动中…`；后端返回 running 但邀请码未出现时仍保持等待；邀请码出现后再显示停止/重启。运行中点击重启同理。点击停止时应显示 `停止中…`，直到状态停稳后才显示启动。

## 下一步注意事项
- 这个等待态以邀请码作为“可对玩家开放操作”的信号；如果未来后端新增更明确的 ready 字段，可以把判断从 `inviteCode` 收敛到该字段。
- 不要在页面里另写轮询逻辑，继续通过 `dashboardData.requestInviteCodeRefresh()` 和公共数据层处理旧邀请码过滤。

# NEXUS-INSTALLED-1 前端交接

## 改了什么
- 添加模组页的已安装列表改为 Nexus 视角：只渲染自身带 Nexus ID、随 Nexus 包安装的内容包，以及虚拟 SMAPI 前置项。
- 纯本地文件项和 `StardewAnxiPanel.Control` 不再进入主卡片网格；如果存在这类项目，页面只显示“已隐藏 N 个本地文件项”的短提示。
- `modToSearchResult()` 对 SMAPI 走 Nexus:2400，按钮文案为“跳转 N站”；旧官网 fallback 已移除，`Pathoschild.SMAPI` 识别为大小写不敏感。无图片时卡片显示来源文字占位，不再显示文件夹图标。
- Nexus 视角卡片底部不再展示 `UniqueID`，保留更接近 N 站的数据字段。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`

## 如何验证
- 已执行：`npm.cmd run build`。
- 手动验证建议：安装一个带 `UpdateKeys: ["Nexus:<id>"]` 的 Mod 和一个纯本地 Mod，添加页应只把前者渲染成 Nexus 卡片，纯本地项只计入隐藏提示。

## 下一步注意事项
- 这只是前端展示策略，不改变后端 `GET /mods` 返回，也不影响上传、删除、同步包导出。删除按钮仍对可见 Nexus 卡片调用原 `DELETE /mods/:id`。

# MODSYNC-AUTO-1 前端交接

## 改了什么
- `ModInfo` 类型新增 `isContentPack?: boolean` 和 `contentPackFor?: string`，用于接收后端从 SMAPI `ContentPackFor` 解析出的内容包信息。
- Mods 已安装卡片底部的同步分类下拉框不再限制管理员，任意登录用户都可以把 Mod 改为“服务器专用 / 玩家需同步 / 待确认”。
- 后端会自动给内容包和第三方 Mod 打上“玩家需同步”标签，前端直接展示 `syncKind`；标签 hover 使用 `syncNote` 展示自动识别说明。
- 分类编辑成功后，前端会把接口返回的 `syncKind/syncNote` 同步回本地 state，避免 hover 说明停留在旧值。

## 影响文件
- `frontend/src/types.ts`
- `frontend/src/games/stardew/pages/ModsPage.tsx`

## 如何验证
- 已执行：`npm.cmd run build`。
- 手动联调建议：用普通登录用户进入 Mods 已安装列表，确认能看到并操作同步分类下拉框；上传 `[CP]` 内容包后刷新，应默认显示“玩家需同步”，hover 标签能看到自动识别说明。

## 下一步注意事项
- 这只是修改面板元数据，不会修改 Mod 文件；前端仍不要给内置 SMAPI 条目渲染分类下拉。

# MODORIGIN-1 前端交接

## 改了什么
- `frontend/src/types.ts` 的 `ModInfo` 新增 `originSource/originNexusModId/originModName/originModUrl`，`ModSource` 新增 `nexus_package`，`ModSearchResult` 新增 `sourceDetail`。
- `ModsPage.modToSearchResult()` 会区分两种来源：带 `nexusModId` 的主 Mod 显示 `来源：N站` 和 `Nexus:<id>`；没有自己的 Nexus ID 但有 `originSource=nexus` 的内容包显示 `来源：N站包` 和 `随 <originModName> 安装`。
- 内容包跳转按钮使用 `originModUrl`，没有时回退到 `https://www.nexusmods.com/stardewvalley/mods/<originNexusModId>`；卡片图片继续使用后端返回的 `pictureUrl`。
- 已安装列表通过 `modBundleKey()` 排序，同一个 Nexus 安装包导入出的主 Mod 和内容包相邻显示；主 Mod 排在内容包前。
- 删除同包任意成员时，确认弹窗会列出同包将被一起删除的 Mod，并提示这是同一个 Nexus 安装包的一部分。确认后只调用一次 `deleteMod(confirmDelete.id)`，由后端执行捆绑删除。

## 影响文件
- `frontend/src/types.ts`
- `frontend/src/games/stardew/pages/ModsPage.tsx`

## 如何验证
- 已执行：`npm.cmd run build`。
- 手动联调建议：上传 `MultipleConstructionOrders ...-47289-...zip` 后刷新已安装 Mod，主 Mod 应显示 `来源：N站 / Nexus:47289`，`[CP]` 内容包应显示 `来源：N站包 / 随 Multiple Construction Orders 安装`，两张卡片应相邻且都能使用 Nexus 缩略图；点击任意一张删除，弹窗应列出两张卡片的名称，确认后两者一起消失。

## 下一步注意事项
- `nexus_package` 是展示来源，不是新的安装来源；已安装卡片没有一键安装动作，后续做更新检查时不要把内容包当作可独立更新的 Nexus Mod。删除是 bundle 级行为，前端不要循环 DELETE 多个成员。

# MODDEPS-1 前端交接

## 改了什么
- `frontend/src/types.ts` 新增 `ModDependency`，`ModInfo` 增加 `dependencies?: ModDependency[]`。
- `ModsPage` 已安装 Mod 卡片底部新增普通用户可见的金色依赖标签，只展示必需依赖；常见 UniqueID 映射为可读名称，例如 `Pathoschild.ContentPatcher` 显示为 `Content Patcher`。
- 未知 UniqueID 会去掉作者/命名空间并拆分驼峰，只显示模组名；标签页面文案使用短前缀“前置：...”，完整依赖和最低版本保存在悬浮提示。
- 标签超过两个依赖时压缩显示；`.sd-mods-dependency-tag` 使用单行省略并限制在卡片宽度内，防止长依赖名撑出卡片。

## 影响文件
- `frontend/src/types.ts`
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`

## 如何验证
- 已执行：`npm.cmd run build`。
- 手动验证：上传或放入一个 manifest 含 `ContentPackFor: {"UniqueID":"Pathoschild.ContentPatcher"}` 的内容包，刷新 Mods 已安装列表，应看到“前置：Content Patcher”标签，悬浮提示显示完整依赖。

## 下一步注意事项
- 当前只是展示 manifest 声明，不标红缺失依赖，也不自动安装。后续依赖检查页面可复用 `dependencies[]` 和已安装 `uniqueId` 列表做缺失/版本判断。

# MODUPLOAD-2 前端交接

## 改了什么
- `api.ts` 新增 `uploadMods(files, instanceId?)`，重复 append `mod` 字段后调用原上传接口。`uploadMod(file)` 兼容封装已在 FE-CLEANUP-1 中删除。
- `ModsPage` 上传弹窗改为多文件选择：`input[type=file]` 增加 `multiple`，状态从单个 `File | null` 改成 `File[]`，展示已选 ZIP 数量、总大小和最多 5 个文件名。
- 旧版 `ModsSection` 后续已确认无引用，并在 FE-CLEANUP-1 中删除；当前只维护 `pages/ModsPage.tsx`。

## 影响文件
- `frontend/src/api.ts`
- `frontend/src/games/stardew/pages/ModsPage.tsx`

## 如何验证
- 已执行：`npm.cmd run build`
- 建议手动联调：管理员在服务器停止状态下打开“上传 Mod”弹窗，一次选择 2 个以上 `.zip`，确认上传成功后已安装列表刷新；停服上传不应出现额外“需要重启”提示，下次启动会直接加载。

## 下一步注意事项
- 当前上传仍是一个 HTTP 请求，没有逐文件进度条；如果之后要支持很大的批量包，建议接成后台 job/SSE 进度。
- 前端 accept 仍只写 `.zip`，不要在文案里暗示 7z/rar 已支持。

# NEXUS-META-1 前端交接

## 改了什么
- 本次没有修改前端代码；后端 `GET /api/instances/:id/mods` 会自动为带 `UpdateKeys: ["Nexus:<id>"]` 的本地/手动上传 Mod 补齐 Nexus 缩略图和卡片元数据。
- `ModsPage` 已安装卡片已使用 `pictureUrl` 优先渲染，所以刷新列表后可直接显示后端补齐的 Nexus 缩略图；无图继续回退本地 Mod 图标。
- 数字 ID 搜索展示元数据不再依赖 Nexus API Key；Key 只与受限下载/安装链路有关。

## 影响文件
- 本次无前端源码改动，仅更新文档。

## 如何验证
- 后端已执行：`go test ./...`。
- 前端可手动验证：上传/放入一个 manifest 含 `UpdateKeys: ["Nexus:2400"]` 的 Mod 后刷新 Mods 列表，卡片应出现 Nexus 缩略图。

## 下一步注意事项
- 现有页面文案里如果仍暗示“ID 搜索必须配置 API Key”，后续可以改成“API Key 仅用于 Nexus 受限下载/一键安装”；展示型搜索和缩略图不再需要 Key。

# MODSEARCH-3 前端交接

## 改了什么
- 搜索结果排序由后端统一负责：`GET /api/instances/:id/mods/search` 返回的 `results[]` 已按 `downloadCount` 从高到低排列。
- `ModsPage` 保持按接口顺序渲染卡片，不额外排序，避免以后多来源搜索时前后端规则不一致。
## 影响文件
- 本次没有改前端代码；交接记录用于说明展示顺序依赖后端返回顺序。
## 如何验证
- 后端测试通过后，前端无需额外逻辑；整体可继续执行 `npm.cmd run build` 做类型/构建确认。

# MODRESTART-1 前端交接

## 改了什么
- ModsPage 停服上传成功文案改为“下次启动服务器时会自动加载”。
- Mod 概览统计里的“重启需求”改为“运行中重启”，避免用户误解为面板项目或已停止的服务器还要重启。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`

## 如何验证
- 执行：`npm.cmd run build`。

# MODSEARCH-2 回滚记录

## 改了什么
- 已按产品要求撤回 CurseForge / Dataset 多源搜索前端接入，下载页头部恢复为只显示 Nexus Key 配置。
- 删除双 provider Key 弹窗逻辑、CurseForge API 封装和 `curseforge_download_url` 安装按钮判断。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/api.ts`
- `frontend/src/types.ts`

## 如何验证
- 回滚后需执行：`npm.cmd run build`。

## 下一步注意事项
- 前端仍保留统一搜索卡片结构；后续如重新接入其他 provider，应先确认后端接口和产品文案。

# MODSEARCH-1 前端交接

## 改了什么
- `ModsPage` 下载页改为“统一搜索”，调用 `searchMods()` 而不是 Nexus 专用搜索封装。
- 新增/使用 `ModSearchResult` 类型和 `installSearchedMod()` API 封装。
- 搜索结果卡片改为 `ModSearchResultCard`，显示来源标签；N站跳转按钮显示“跳转 N站”，安装按钮显示“一键安装”。
- 安装按钮按 `installMethod` 判断可用性：`nexus_premium` 需要 Nexus Key，`direct_url` 需要 `installUrl`，`none/manual` 禁用。安装进度仍复用原 SSE 面板。
- 已安装 Mod 列表也复用统一卡片，有 Nexus 元数据时显示 N站来源，否则显示本地来源。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/api.ts`
- `frontend/src/types.ts`

## 如何验证
- 已执行：`npm.cmd run build`。
- 建议手动联调：无 Key 搜索普通关键词确认 N站结果可见但安装禁用；配置 Key 后确认按钮文案为“一键安装”并可创建安装任务。

## 下一步注意事项
- StardewModDataset/CurseForge/ModDrop/GitHub provider 接入后，只要后端返回同一 `ModSearchResult` 结构，页面无需重造卡片。
- 其他网站跳转按钮文案由 `externalLabel` 控制，应返回“跳转 {网站名}”。

# REMOTE-MOD-1 前端交接

## 改了什么
- `ModsPage` 下载页新增“粘贴链接安装”按钮和弹窗，管理员可粘贴 `nxm://...` 或 Nexus CDN / ModDrop / GitHub / CurseForge 等来源的 `https://...zip` 链接。
- 新增 `installRemoteMod()` API 封装，调用 `POST /api/instances/:id/mods/remote/install`。
- 远程安装复用原 Nexus 安装进度面板、SSE 日志订阅、任务完成后刷新 Mod 列表的逻辑。
- Nexus 直连任务若失败且错误包含 403，前端提示非 Premium 用户改用 NXM、nexus-cdn `.zip` 临时链接，或 ModDrop/GitHub/CurseForge `.zip` 直链。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/api.ts`
- `frontend/src/types.ts`

## 如何验证
- 已执行：`npm.cmd run build`。
- 建议手动联调：用管理员账号在服务器停止状态下粘贴一个 `nxm://`、nexus-cdn `.zip` 临时链接，或 ModDrop `.zip` 直链，确认任务日志、安装完成刷新、已安装列表显示正常。

## 下一步注意事项
- 当前 UI 明确只承诺 ZIP 直链；不要在文案里写 7z/rar 已支持。
- 后续接 CurseForge/GitHub/ModDrop 搜索和自动下载时，建议复用同一个安装进度面板，不要再造一套任务 UI。

# NEXUS-3 前端交接

## 改了什么

- 下载页说明后续应改为“无 Key 可走 GraphQL v2 搜索和 ID 元数据查询；配置 Nexus Mods API Key 后才可能使用受限下载/一键安装”。
- 搜索结果卡片接入“安装到服务器”按钮，调用 `installNexusMod` 创建后端 `mod_nexus_install` job，并订阅 SSE 展示最近安装日志。
- 已安装列表改用与搜索结果同一个 `NexusResultCard` 外观，后端返回 `pictureUrl` 时显示 Nexus 缩略图；手动上传或缺少元数据的 Mod 回退到本地 Mod 图标。
- 同步分类下拉、删除按钮和 UniqueID 现在放在 Nexus 卡片底部。

## 影响文件

- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/api.ts`
- `frontend/src/types.ts`
- `frontend/src/games/stardew/StardewPanel.css`

## 如何验证

- 已执行：`npm.cmd run build`。
- 建议手工联调：无 Key 搜索普通关键词；无 Key 搜索数字应返回 GraphQL 结果或空列表而不是缺 Key；配置 Key 后点击安装，观察安装日志和已安装卡片缩略图。

## 下一步注意

- 当前一次只允许一个 Nexus 安装任务，避免并发写 Mod 目录。
- 多文件选择、依赖检测、更新检测仍是后续能力。

# 前端接手文档 2026-07-01

## 同日补充 2：Nexus API Key 改为下载页内配置

`ModsPage` 的“下载模组”页头新增管理员专用“配置 API Key”按钮。管理员打开弹窗后粘贴 Nexus API Key，前端调用 `saveNexusAPIKey()` 写入后端 SQLite `panel_settings`，保存成功会更新当前页状态并立即生效；后端不再依赖环境变量。弹窗支持删除 Key，页面状态只显示“已配置/未配置”和末 4 位，不回显完整 Key。

涉及文件：`frontend/src/games/stardew/pages/ModsPage.tsx`（新增 `nexusSettings`/`showNexusKeyModal`/保存删除状态与弹窗）、`frontend/src/api.ts`（`getNexusSettings`/`saveNexusAPIKey`/`deleteNexusAPIKey`）、`frontend/src/types.ts`（`NexusSettingsStatus`）、`frontend/src/games/stardew/StardewPanel.css`（`.sd-mods-panel-actions`）。验证：`npm.cmd run build`。

## 同日补充：页头“刷新列表 / 导出 Mod 包”错误提示移到 Tab 外层

代码评审发现：`ModsPage.tsx` 页头的"刷新列表"（`loadMods`，写 `listError`）和"导出 Mod 包"（`handleExport`，写 `exportError`）两个按钮在三段式工作台的所有 Tab 下都能点，但这两个错误条原来只在 `activeTab === 'installed'` 分支里渲染。用户停在"下载模组"或"配置模组"时点这两个按钮失败，错误状态会被设置但页面上完全看不到提示。

修复：把 `{listError && ...}` / `{exportError && ...}` 从 `installed` Tab 内部挪到 `<div className="sd-mods-workbench">` 外层、页头下方，三个 Tab 下都能看到。没有改动这两个状态本身的写入逻辑，纯粹是渲染位置调整。

涉及文件：`frontend/src/games/stardew/pages/ModsPage.tsx`。验证：`npm.cmd run build` 通过。

## 本次改动：Mod 页参考 EMP 管理台改为三段式工作台

参考页面来自 `E:\源码\emp_源码\dst-management-platform-web\src\views\game\mod.vue` 及同目录的 `download.vue`、`add.vue`、`setting.vue`。参考项目的核心结构是顶部 Tab：Download 负责远端搜索，Add 负责已下载 Mod 管理和批量启用，Setting 负责已启用 Mod 配置。

本项目对应改为 `下载模组 / 添加模组 / 配置模组` 三个页内 Tab，仍然只在 `frontend/src/games/stardew/pages/ModsPage.tsx` 内实现，没有新增路由，也没有改后端 API：

- `下载模组`：承载 Nexus Mods 搜索。搜索条增加“名称 / ID”选择器占位，结果改为带预览图的卡片网格，并保留“打开 N 站”和禁用态“安装待接入”。
- `添加模组`：承载原有本服已安装 Mod 列表、玩家同步统计、同步包导出、上传 Mod、删除 Mod、导出整包、重启提示等功能。
- `配置模组`：先做成后续能力入口，占位展示启用/禁用、依赖检查、更新检查、SMAPI 配置面板；当前没有真实启用/禁用或配置接口，所以所有危险/未完成操作保持禁用或说明态。

样式集中在 `frontend/src/games/stardew/StardewPanel.css`，新增 `.sd-mods-workbench`、`.sd-mods-tabs`、`.sd-mods-tab-*`、`.sd-mods-settings-*`，同时把 `.sd-mods-nexus-list` 改为响应式卡片网格。移动端 720px 以下会把 Tab、搜索栏、Nexus 卡片和配置布局纵向堆叠。

验证：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

构建通过（`tsc -b && vite build`）。下一步如果要真正接近 EMP 的完整体验，需要后端补齐“下载安装到服务器 / 启用禁用 / 依赖检查 / 配置表单”能力，再把当前禁用入口逐步点亮。

## 本次改动：Mod 管理第二阶段——Nexus Mods 只读搜索

在既有 `ModsPage` 内加了"在线搜索（Nexus Mods）"区域，没有新建路由。普通登录用户和管理员都能用。

### 类型与 API

`frontend/src/types.ts`：

- `ModInfo` 新增可选字段 `updateKeys?: string[]` 和 `nexusModId?: number`（对应后端 manifest 解析扩展，目前 UI 没有直接展示这两个字段，是为了类型和后端 `GET .../mods` 响应对齐）。
- 新增 `NexusModSearchResult`（`modId`/`name`/`summary`/`author`/`version`/`updatedAt`/`endorsementCount`/`downloadCount`/`pictureUrl`/`nexusUrl`/`installed`/`installedFolderName`/`installedVersion`）和 `NexusModSearchResponse`（`query` + `results`），对应后端 `GET .../mods/nexus/search` 的返回结构。`NexusSettingsStatus` 对应管理员 Key 配置状态（`configured` + 可选 `last4`）。

`frontend/src/api.ts` 新增 `searchNexusMods(query, instanceId?)`，对应 `GET .../mods/nexus/search?q=...`，走通用 `request()` 封装（自动带 `credentials: 'include'`，401/非 2xx 走 `toApiError` 抛 `ApiError`）。本次又补了 `getNexusSettings()`、`saveNexusAPIKey(apiKey)`、`deleteNexusAPIKey()`，对应 `/api/settings/nexus` 和 `/api/settings/nexus/api-key`。

### ModsPage 改动

`frontend/src/games/stardew/pages/ModsPage.tsx`：

- 新增 `NexusResultCard` 组件：展示图片（有 `pictureUrl` 才渲染）、名称、已安装标记（绿色 `sd-tag`，悬浮提示文件夹名，文字带上 `installedVersion`）、作者/版本/更新时间（复用 `sd-mods-meta-item`/`sd-mods-meta-label` 这套已有样式）、摘要（复用 `sd-mods-meta-desc`）、下载量/认可数统计行；操作区是"打开 N 站"按钮（`window.open(nexusUrl, '_blank', 'noopener,noreferrer')`，不是 `<a>` 标签，保持和页面其余操作一致用 `<button>`）+ 一个禁用态的"安装待接入"小标记（复用 `.sd-mods-pending-badge` 样式），**没有任何安装按钮**，避免误导用户以为能直接装。
- 页面级新增"在线搜索（Nexus Mods）"区块（放在概览统计卡片之后、玩家同步区域之前）：输入框（`.sd-input`，回车触发搜索）+ 搜索按钮，关键词 trim 后为空时按钮禁用并在 `title` 提示；搜索中按钮文字变"搜索中…"；失败显示中文错误条（复用 `.sd-mods-list-error`，文案直接来自后端 `message`，因为后端已经返回中文文案，`errorMessage()` 没找到 `errorCodeMap` 命中时会原样透出 `error.message`，所以不需要额外维护一份 nexus 错误码到中文的映射表）；无结果时显示"未找到匹配的 Mod，换个关键词试试。"；结果用 `NexusResultCard` 列表展示。
- 新增 state：`nexusQuery`/`nexusLoading`/`nexusError`/`nexusResults`，和 `handleNexusSearch()`。不接入 `dashboardData`（这是只读外部搜索，不影响本地 Mod 列表/同步分类，没有缓存或跨页面共享的必要）。

`frontend/src/games/stardew/StardewPanel.css` 新增一组 `.sd-mods-nexus-*` 类（`-search-row`/`-empty`/`-list`/`-card`/`-card-pic`/`-card-main`/`-card-header`/`-card-name`/`-card-meta`/`-card-stats`），卡片整体结构和现有 `.sd-mods-card` 保持视觉一致（同样的边框、背景、圆角），只是多了一个 56×56 的封面图位置。操作区直接复用了 `.sd-mods-card-actions`（纵向排列），没有新建。

### 权限边界

- 管理员和普通登录用户都能搜索、查看结果、点"打开 N 站"——对应后端 `requireAuth`（不是 `requireAdmin`）。
- 只有管理员能看到和使用“配置 API Key”入口；普通用户不请求 `/api/settings/nexus`。
- 没有人能点"安装"，因为这个阶段压根没有安装按钮，只有一个禁用提示。

验证：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

构建通过（`tsc -b && vite build`，无类型错误）。下一位维护者如果要看实际效果：用管理员账号在“下载模组”页配置 Nexus API Key 后，搜索一个常见 Mod 名（比如 "Stardew Valley Expanded"）和一个数字 ID（比如 "2400"），确认两条路径都能出结果；再随便挑一个本地已装且 manifest 带 `UpdateKeys: ["Nexus:<id>"]` 的 Mod，搜它确认卡片上出现"已安装"标记。

## 本次改动：Mod 玩家同步包（第一阶段）

在既有 `ModsPage` 内加了一个“玩家同步”区域，没有新建路由，复用现有像素风样式（`sd-tag`、`sd-btn-tan` 等）。

### 类型与 API

`frontend/src/types.ts`：

- 新增 `ModSyncKind = 'server_only' | 'client_required' | 'unknown'`。
- `ModInfo` 新增 `syncKind: ModSyncKind`（恒有值）和可选 `syncNote?: string`。
- 曾新增 `ModSyncSummary`（total/serverOnly/clientRequired/unknown）和 `ModSyncPlanResult`（mods + summary）用于 `GET .../mods/sync-plan`；FE-CLEANUP-1 已删除前端未使用的这两个类型，页面直接基于 `GET /mods` 返回的 `mods[].syncKind` 计算统计。

`frontend/src/api.ts` 新增：

- `getModSyncPlan(instanceId?)` 曾对应 `GET .../mods/sync-plan`，但页面没有调用；FE-CLEANUP-1 已删除该前端封装。
- `updateModSyncClassification(modId, syncKind, syncNote?, instanceId?)` — 对应 `PUT .../mods/:modId/sync-classification`。
- `exportModSyncPack(instanceId?)` — 对应 `POST .../mods/sync-pack/export`，blob 下载模式和现有 `exportMods()` 完全一致（解析 `Content-Disposition` 拿文件名）。

### ModsPage 改动

`frontend/src/games/stardew/pages/ModsPage.tsx`：

- `ModCard` 新增 props：`syncBusy`、`onSyncChange`。每张卡片标题下方新增一行：一个 `sd-tag`（按 `syncKind` 着色：`server_only` 蓝/`client_required` 绿/`unknown` 金）+ 登录用户可见的 `<select>` 下拉框（切换会立即调用 `updateModSyncClassification`）。
- 页面级新增“玩家同步”区块（放在概览统计卡片之后、重启横幅之前）：三个统计 tag（服务器专用/玩家需同步/待确认，数字从已加载的 `mods` 数组本地算出，不额外请求）+ “导出玩家同步包”按钮。按钮在 `clientRequired === 0` 时禁用，导出中显示“导出中…”，失败显示中文错误条（复用 `sd-mods-list-error` 样式）。
- 分类编辑成功后用 `setData` 局部更新对应 Mod 的 `syncKind`（不用整页重新拉取），并调用 `dashboardData.refreshMods()` 让其他依赖 `dashboardData.mods` 的地方保持同步。
- 删除/上传等既有流程未改动。

`frontend/src/games/stardew/StardewPanel.css` 新增 4 个类，全部接在已有 `.sd-mods-*` 命名空间下：`.sd-mods-sync-summary`、`.sd-mods-sync-hint`、`.sd-mods-sync-row`、`.sd-mods-sync-select`。同步标签直接复用全局已有的 `.sd-tag` / `.sd-tag-blue` / `.sd-tag-green` / `.sd-tag-gold`（定义在 `stardew-theme.css`），没有新增标签配色体系。

### 权限边界

- 管理员和普通登录用户：都能看 tag、用下拉框改分类、导出同步包。危险写操作（上传/删除/安装）仍按管理员权限控制。

验证：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

构建通过（`tsc -b && vite build`，无类型错误）。本次没有起开发服务器做浏览器手动验证，下一位维护者如果要看实际效果，建议：装几个测试 Mod（包含 `StardewAnxiPanel.Control`）、用普通登录用户把其中一两个改分类，确认导出按钮可下载同步包。`PLAYERSYNC-PACK-2` 后 ZIP 内部以 `pack-manifest.json`、`checksums.sha256` 和 `payload/mods/` 为准，且 ZIP 里没有控制 Mod。

## 下一步注意事项

- 没有给"玩家需同步"的 Mod 数量变化做特殊提示（比如导出后是否需要重新生成同步包），目前导出永远是当下最新分类的实时快照，足够用。
- 前端已不再保留 `getModSyncPlan` 封装；如果后续真的需要单独同步计划视图，再按实际页面重新加回。
# SMAPI-RUNTIME-1 前端交接

## 改了什么
- `ModInfo` 类型新增 `builtIn?: boolean`，`ModSource` 新增 `builtin`。
- `ModsPage` 会把 `builtIn` 的 SMAPI 条目作为已安装列表中的普通卡片置顶展示；在当前 Nexus 视角下显示为 Nexus:2400，跳转按钮为“跳转 N站”。
- 内置条目不显示删除按钮，不显示同步分类下拉；底部标签显示“置顶 / 玩家需先安装 / 不打包进同步包”。
- 玩家同步统计改为排除 `builtIn` 条目，避免 SMAPI 虚拟组件让同步包导出按钮误判可用。

## 影响文件
- `frontend/src/types.ts`
- `frontend/src/games/stardew/pages/ModsPage.tsx`

## 如何验证
- 已执行：`npm.cmd run build`。

## 下一步注意事项
- 后端返回的 SMAPI 是虚拟运行组件，不要给它接删除、启停、分类或配置表单。
- 如果后续做玩家下载引导，可以在这个卡片上扩展到 SMAPI 安装说明或版本检测。
# NEXUS-PAGED-1 前端交接

## 改了什么
- `ModsPage` 下载页搜索改为直接调用 `searchNexusMods`，不再调用 `searchMods` 统一搜索。
- 搜索结果状态改为 `NexusModSearchResult[]`，通过 `nexusResultToSearchResult` 仅复用卡片布局；安装按钮调用 `installNexusMod`。
- 搜索结果头部展示 `total` 和当前页，并提供上一页/下一页按钮。`searchNexusMods` API 封装新增 `page/pageSize` 参数。
- 文案改为 Nexus-only：搜索区显示“搜索 Nexus Mods”，粘贴安装说明只保留 Nexus `nxm://` 和 Nexus CDN 临时 ZIP。

## 影响接口/文件
- 接口：`GET /api/instances/:id/mods/nexus/search`、`POST /api/instances/:id/mods/nexus/install`。
- 文件：`frontend/src/api.ts`、`frontend/src/types.ts`、`frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。

## 如何验证
- 已执行：`npm.cmd run build`。

## 下一步注意事项
- 当前前端已移除 `searchMods` / `installSearchedMod` API 封装；后端也已撤下 `/mods/search` 和 `/mods/search/install`，下载页只认 Nexus 搜索/安装契约。
# NEXUS-PAGER-2 前端交接

## 改了什么
- Nexus 搜索结果分页条抽成 `renderNexusPager('top' | 'bottom')`，顶部和列表底部各渲染一份。
- 分页支持：首页、上一页、指定页输入跳转、下一页、末页；跳页输入会按总页数夹取有效范围。
- 新增 `nexusPageInput` 状态，搜索成功后同步到实际页码，搜索失败时重置为 `1`。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`

## 如何验证
- 已执行：`npm.cmd run build`。

# SMAPI-SYNC-2 前端接手记录

## 改了什么

- `ModsPage` 新增 SMAPI / 面板控制 Mod 判断 helper，把“内置显示”和“是否计入玩家同步”拆开。
- SMAPI 继续置顶且不可删除/不可改分类，但会计入“玩家需同步”统计，并允许只有 SMAPI 前置时导出玩家同步包。
- `StardewAnxiPanel.Control` 显示为内置服务端控制组件，不显示删除按钮，不显示同步分类下拉，不计入同步统计，也不会有可点击外部页面。
- 内置排序增加权重：SMAPI 第一，Control 第二。

## 影响文件

- `frontend/src/games/stardew/pages/ModsPage.tsx`

## 如何验证

- `npm.cmd run build`

## 下一步注意事项

- 如果后续做玩家同步包安装向导，前端要读取 manifest 里的 `packaged=false` 来展示“需要手动安装 SMAPI”的前置说明，不要尝试把 SMAPI 当作包内文件解压。
# PLAYERSYNC-PACK-15 前端接手记录

## 改了什么
- 玩家同步区域新增第二个导出按钮：`导出完整同步包` 和 `导出模组更新包`。
- `exportModSyncPack()` 继续调用完整版接口；新增 `exportModSyncUpdatePack()` 调用 `POST /api/instances/:id/mods/sync-pack/export-update`。
- 更新包按钮只在存在真实可打包的 `client_required` Mod 时启用，避免只有虚拟 SMAPI 条目时导出无内容更新包。
- 提示文案说明完整包用于首次加入玩家，模组更新包不带 SMAPI，已安装玩家运行后会跳过完全相同的 Mod。

## 影响文件/接口
- `frontend/src/api.ts`
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- 接口：`POST /api/instances/:id/mods/sync-pack/export-update`

## 如何验证
- 已执行：`npm.cmd run build`。

## 下一步注意事项
- 更新包不需要前端解析 ZIP；仍按 Blob 下载即可。
- 如果后续要展示“本次更新包包含哪些 Mod”，应读取后端 `GET /mods` 的当前 `syncKind` 和 `builtIn` 字段，不要维护按用户的导出历史。

# MODPROFILE-1 前端接手记录

## 改了什么
- `ModInfo` 新增 `enabled/canToggle/enableNote`。
- `api.ts` 新增 `updateModEnabled()`。
- `ModsPage` 的“配置模组”页改为当前存档 Mod 启用列表，管理员可在停服时切换；运行中、普通用户、内置组件禁用开关。
- “添加模组”的 Nexus 已安装卡片底部增加 `已启用/已禁用` 标签。
- 新增 `.sd-mods-enable-*` 和 `.sd-mod-toggle*` 样式，移动端避免文字和开关挤压。

## 影响文件/接口
- `frontend/src/types.ts`
- `frontend/src/api.ts`
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `PUT /api/instances/:id/mods/:modId/enabled`

## 如何验证
- 已执行：`npm.cmd run build`。
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。

## 下一步注意事项
- 列表里禁用 Mod 仍会显示，UI 必须看 `enabled` 字段，不要假定返回列表只包含启用 Mod。
- 后续做依赖检查时，可在禁用列表旁提示“启用该 Mod 还需要哪些前置”。
# MODPROFILE-2 前端接手记录

## 改了什么
- `SavesPage` 的存档变更回调现在同时刷新 saves 和 mods。
- `useStardewDashboardData` 使用 `activeSaveNameRef` 记录上一次活动存档；`saves.activeSaveName` 改变后自动调用 `refreshMods()`，避免 ModsPage 使用旧缓存。

## 影响文件/接口
- `frontend/src/games/stardew/useStardewDashboardData.ts`
- `frontend/src/games/stardew/pages/SavesPage.tsx`
- `GET /api/instances/:id/mods`

## 如何验证
- 已执行：`npm.cmd run build`。
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。

## 下一步注意事项
- ModsPage 仍会把 `dashboardData.mods` 同步到局部 state；只要公共数据层刷新了 mods，页面就会更新。不要在存档切换后只刷新 saves。

# NEXUS-DEFAULT-1 前端接手记录

## 改了什么
- `ModsPage` 下载页首次进入时自动拉取 Nexus 默认热门第一页。
- 空搜索框按钮不再禁用，文案为“刷新热门”；输入关键词后仍显示“搜索”。
- 空结果提示区分关键词搜索失败和热门列表未读取到结果。

## 影响文件/接口
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `GET /api/instances/:id/mods/nexus/search?q=&page=1&pageSize=20`

## 如何验证
- 已执行：`npm.cmd run build`。
- 已执行：`go test ./internal/games/stardew_junimo ./internal/web`。

## 下一步注意事项
- 自动加载只在进入下载页后触发一次；用户清空输入后点击“刷新热门”可手动重新拉取默认列表。
# NEWGAME-CABINS-1 前端接手

## 改了什么
- 新建存档 UI 左侧“初始联机小屋”数字从 `startingCabins + 1` 改为直接显示 `startingCabins`。
- 之前用户看到的数字更像“总人数”，例如显示 2 时实际只提交 1 间小屋；现在显示值与提交给后端的小屋数量一致。

## 影响文件/接口
- `frontend/src/games/stardew/NewGameCreator.tsx`
- 接口字段不变，仍提交 `NewGameConfig.startingCabins`；后端契约已同步为 0-7。

## 如何验证
- 已执行：`cd E:\stardew-server-anxi-panel\frontend; npm.cmd run build`。

## 下一步注意事项
- 如果后续想同时提示可容纳人数，可以另加独立文案“可容纳 N 人”，不要再把该数值替代“小屋数”。


# SAVE-BACKUP-POLICY-1 ????

- ?????`frontend/src/games/stardew/SavesSection.tsx`?`frontend/src/games/stardew/StardewPanel.css`?`frontend/src/api.ts`?`frontend/src/types.ts`?
- ?????????????????????????? latest??????????????????????1-14??? 3???????
- ??????? `kind` ???????manual/latest/daily/scheduled?
- ??????????? draft ????????????`GET /saves/backups` ???????????????????
- ???`npm.cmd run build` ???

# FE-BACKUP-COPY-1 前端接手

## 改了什么
- `SavesSection` 的备份策略区改成“自动备份规则”面板，避免把 `latest`、`scheduled` 这种实现名词直接展示给用户。
- 三个自动备份概念分别解释为：
  - 游戏保存后更新“最新备份”：玩家睡觉完成保存后覆盖同一份最新备份。
  - 每天固定时间更新“定时备份”：到达用户选择的整点后覆盖同一份定时备份。
  - 每日快照最多保留 N 天：每天只留一份，同一天再次覆盖，超过天数删旧快照。
- 备份列表的 `kind` 标签文案改为“手动备份 / 最新备份 / 每日快照 / 定时备份”。

## 影响文件
- `frontend/src/games/stardew/SavesSection.tsx`
- `frontend/src/games/stardew/StardewPanel.css`

## 如何验证
- 已执行：`npm.cmd run build`。

## 下一步注意
- 后续如果新增“关闭每日快照”开关，文案要继续保持“会发生什么”的表达，不要直接显示后端字段名。

# SAVE-BACKUP-SCHEDULE-HOUR-1 前端接手

## 改了什么
- `BackupPolicy` 类型新增 `scheduledHour`，旧 `scheduledIntervalHours` 仅保留为可选兼容字段。
- `SavesSection` 的定时备份设置从数字间隔输入改为 00:00-23:00 下拉框，默认 04:00。
- 读到旧策略时会用前端归一化补齐 `scheduledHour`；保存时去掉旧间隔字段。

## 影响文件
- `frontend/src/types.ts`
- `frontend/src/games/stardew/SavesSection.tsx`
- `frontend/src/games/stardew/StardewPanel.css`

## 如何验证
- 已执行：`npm.cmd run build`。

## 下一步注意事项
- UI 文案要继续表达“每天到点覆盖同一份定时备份”，不要再恢复成“每隔 N 小时”。
# FE-SCHEDULED-RESTART-1 前端交接

## 改了什么
- 服务器控制页“快捷操作”中的“计划重启”按钮已接入弹窗，不再显示待接入。
- 弹窗可编辑启用状态、关闭时间、开启时间、时区、关服前提醒、关闭前备份、在线玩家跳过。
- 保存后调用后端 `PUT /api/instances/:id/restart-schedule`，并展示后端返回的下次关闭/开启时间与上次执行状态。

## 影响文件/接口
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/src/api.ts`
- `frontend/src/types.ts`
- 接口：`GET|PUT /api/instances/:id/restart-schedule`

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`。
- 手动联调建议：管理员打开服务器控制页，点击“计划重启”，保存一个接近当前时间的维护窗口，确认弹窗显示 next 时间；普通用户按钮禁用。

## 下一步注意事项
- 文案继续区分“备份已保存进度”和“强制保存世界”：计划关闭前备份不等于立即保存游戏内实时进度。
- 如果后续加“一次性跳过下一次维护”，前端可以在同一个弹窗底部加按钮，但当前后端未提供 skip-next API。
# MODDEPS-2 前端接手记录

## 改了什么
- `types.ts` 对齐后端新增字段：`ModDependency.installed/enabled/installedVersion/satisfied/status`，以及 Nexus 搜索结果的 `installedEnabled`。
- `ModsPage` 的 Nexus 搜索卡片会把 `installed=true && installedEnabled=false` 显示为“已安装但未启用”，安装按钮文案为“已安装未启用”，避免用户重复安装已禁用的同一个 Mod。
- 已安装卡片与“配置模组”列表都会展示依赖诊断标签：缺失前置、前置未启用、版本不足为红色，版本待确认为金色；依赖满足时保留原“前置：...”提示。
- “配置模组”列表已修复长 Mod 名与依赖诊断标签共同挤压时的排版问题：依赖诊断放回名称区域下方，右侧状态列只保留“内置/已启用/已禁用”，避免标题被压成竖排。

## 影响文件/接口
- `frontend/src/types.ts`
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 复用接口：`GET /api/instances/:id/mods`、`GET /api/instances/:id/mods/nexus/search`

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`
- 已用浏览器打开 Vite `http://127.0.0.1:5174/`，验证页面加载、登录输入框可交互。当前浏览器无登录态，未进入 ModsPage 做真实存档数据视觉验证。

## 下一步注意事项
- 不要为了依赖状态再新增单独前端请求；当前状态随 `GET /mods` 返回。
- 自动安装缺失依赖和更新提示仍是后续能力，当前 UI 只做检测提示与重复安装防护。

# MODREL-1 前端接手记录

## 改了什么
- `api.ts` 中 `updateModSyncClassification()` 和 `updateModEnabled()` 的返回类型改为接收后端批量 `mods[]`。
- `ModsPage` 的同步分类下拉和启用开关成功后，按 `folderName` 把后端返回的受影响 Mod 批量合并进本地列表，不再只改当前卡片。
- 联动规则完全由后端决定：同步分类会按必需依赖连通组一起处理；启用/禁用会按同 Nexus 包和依赖方向处理；共享前置不会被某个业务包禁用连带关闭。

## 影响文件/接口
- `frontend/src/api.ts`
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- 接口：`PUT /api/instances/:id/mods/:modId/sync-classification`、`PUT /api/instances/:id/mods/:modId/enabled`

## 如何验证
- `cd frontend; npm.cmd run build`
- `cd backend; go test ./...`

## 下一步注意事项
- 前端不要复制 `mod_relationships.go` 的算法；如果以后需要更明确的联动提示，应让后端在响应里追加原因字段，而不是页面自己推断。

# NEXUS-EXT-1 浏览器扩展实验包

## 改了什么
- 新增 `browser-extensions/nexus-slow-installer`，这是 Chrome/Edge 手动加载的 Manifest V3 扩展，不进入 Vite 构建产物。
- `content.js` 注入 Nexus Mod 文件页，识别 `modId/fileId`，显示右下角小面板，并可自动开始捕获、自动点击 `Slow download`。
- `background.js` 使用 `chrome.downloads.onCreated` 捕获最终 `supporter-files.nexus-cdn.com/*.zip?...` 下载链接，按设置取消本地浏览器下载，然后调用面板现有 `POST /api/instances/:id/mods/remote/install`。
- `popup.html/js` 与 `options.html/js` 提供配置：面板地址、实例 ID、自动捕获、自动点击慢速下载、捕获取消本地下载。
- 完整临时 URL 只发给面板；扩展本地状态会脱敏 `md5/expires/user_id/key`。

## 影响文件/接口
- `browser-extensions/nexus-slow-installer/*`
- 复用接口：`POST /api/instances/:id/mods/remote/install`
- 没有改 `frontend/src`，没有新增后端接口。

## 如何验证
- 静态验证：对扩展目录内 JS 执行 `node --check`。
- 手动验证：Chrome/Edge 开发者模式加载 `browser-extensions/nexus-slow-installer`，配置面板地址和 `stardew` 实例；同浏览器登录面板管理员和 Nexus；停服后打开 Nexus 文件下载页，确认扩展捕获临时 ZIP 链接并创建 `mod_remote_install` 任务。

## 下一步注意事项
- 当前扩展请求依赖浏览器能把面板登录 Cookie 带到跨域 `fetch(..., credentials: "include")`。如果正式云端环境出现 401/403，应新增扩展配对 token，而不是让扩展保存管理员密码。
- manifest 现阶段为了测试使用 `http://*/*`、`https://*/*` host 权限；正式发布前应收窄到 Nexus 域名和已配置的面板域名。
- 后续要在 ModsPage 做真“一键扩展安装”时，应由面板打开 Nexus 文件页并把 mod/file 上下文交给扩展；本次只完成扩展端可测试闭环。
# NEXUS-EXT-2 前端接手记录

## 改了什么
- `ModsPage` 在 Nexus/远程安装 job 成功后，会自动切到“添加模组”页。
- 成功回调会重新拉取 `GET /mods` 并刷新 dashboard mods 缓存，避免扩展提交 CDN ZIP 后任务显示完成，但用户还停留在下载页或旧缓存里看不到新安装的 Mod。

## 影响文件/接口
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- 复用接口：`GET /api/instances/:id/mods`。

## 如何验证
- `npm.cmd run build`
- 手动联调：通过扩展或“粘贴链接安装”完成一个 Nexus ZIP 安装，job succeeded 后页面应切到“添加模组”，新 Mod 应出现在“已安装 Nexus 模组”区域。

## 下一步注意事项
- 已安装区仍按 Nexus 视角过滤：带 `nexusModId`、`originSource=nexus` 或 SMAPI 虚拟项才会显示为主卡片；纯本地 Mod 仍只计入隐藏提示。
