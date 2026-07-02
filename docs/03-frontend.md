# FE-QUICK-BACKUP-1 服务器页快捷备份

- `ServerControlPage` 的“快捷操作”里，“备份存档”已接入现有 `createSaveBackup()`，会对当前激活存档调用 `POST /api/instances/:id/saves/:name/backup` 创建手动备份。
- 按钮文案为“备份已保存进度”，仅管理员可用；没有当前激活存档时禁用并提示。运行中也可点，但只打包已经落盘的存档目录，不会强制保存游戏内尚未写盘的进度。备份成功后在快捷操作区显示备份文件名，失败时显示后端错误文案。
- 原“保存世界 / 立即保存”占位已从快捷操作移除；Stardew 的可靠存档写入仍来自游戏内保存事件，面板当前不展示强制立即保存入口，避免和“备份已保存进度”混淆。
- 影响文件：`frontend/src/games/stardew/pages/ServerControlPage.tsx`。
- 验证：`cd frontend && npm.cmd run build` 通过。

# FE-SAVE-START-NAV-1 存档启动后跳总览

- `SavesPage` 的启动类回调从跳转任务页改为跳转 `overview`，覆盖“选择并启动 / 使用此存档启动 / 创建存档并启动 / 上传存档并启动”这几条从存档页发起的启动流程。
- 启动任务创建后会调用 `dashboardData.requestInviteCodeRefresh()`，进入总览页后复用 `FE-LIFECYCLE-WAIT-1` 的按钮旋转与等待新邀请码逻辑；任务列表仍通过 `dashboardData.refreshJobs()` 后台刷新。
- 影响文件：`frontend/src/games/stardew/pages/SavesPage.tsx`。
- 验证：`cd frontend && npm.cmd run build` 通过。

# FE-LIFECYCLE-WAIT-1 启动/重启/停止等待按钮态

- `useStardewDashboardData` 现在把启动/重启后触发的新邀请码轮询状态暴露为 `inviteCodeRefreshing`，用于页面判断“已发出启动/重启请求，但新邀请码尚未出现”。
- `OverviewPage` 与 `ServerControlPage` 在启动、重启以及后端 `starting` 状态下统一显示带旋转圆圈的 `启动中…` 按钮；只有 `dashboardData.inviteCode` 出现后才恢复为运行态的停止/重启按钮。
- 停止操作现在同样保留等待态：点击停止后显示带旋转圆圈的 `停止中…`，直到实例状态进入 `stopped/ready_to_start/save_required` 后才恢复启动按钮。
- `stardew-theme.css` 新增 `.sd-btn-spinner` 与 `.sd-btn-loading`，按钮尺寸保持原生命周期按钮固定宽高，避免旋转图标造成布局跳动。
- 影响文件：`frontend/src/games/stardew/useStardewDashboardData.ts`、`frontend/src/games/stardew/stardew-routes.ts`、`frontend/src/games/stardew/pages/OverviewPage.tsx`、`frontend/src/games/stardew/pages/ServerControlPage.tsx`、`frontend/src/games/stardew/stardew-theme.css`。
- 验证：`cd frontend && npm.cmd run build` 通过。当前环境绑定本地端口返回 `EACCES`，未完成浏览器渲染验证。

# NEXUS-INSTALLED-1 已安装区只展示 Nexus 视角

- `ModsPage` 添加模组页的“已安装模组”改为“已安装 Nexus 模组”，卡片网格只展示有 Nexus 来源的数据：自身带 `nexusModId`、随 Nexus 包安装的内容包（`originSource=nexus`），以及虚拟 SMAPI 前置项。
- 纯本地文件项和服务端控制组件不再混入主卡片网格；存在这类项目时只显示短提示“已隐藏 N 个本地文件项”，避免把添加页视觉退回文件夹列表。
- SMAPI 虚拟项按 Nexus:2400 展示，跳转按钮指向 N 站页面；前端已移除旧官网 fallback，并用大小写不敏感方式识别 `Pathoschild.SMAPI`。没有缩略图时使用来源文字占位（`NEXUS`），不再显示文件夹图标。
- Nexus 视角卡片底部不再展示 `UniqueID`，避免把内部模组标识当成玩家可读内容。
- 相关文件：`frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`npm.cmd run build` 通过。

# MODDEPS-1 已安装 Mod 前置依赖标签

- `ModInfo` 新增 `dependencies?: ModDependency[]`，字段来自后端解析的 SMAPI `Dependencies` / `ContentPackFor`。
- `ModsPage` 已安装 Mod 卡片底部新增金色标签，只展示必需依赖。页面上显示短文案，形如“前置：Content Patcher”；完整“需要前置依赖：Content Patcher >= 2.0.0”保存在 `title`。
- 已知常见 UniqueID 会映射成人类可读名称；未知依赖会去掉作者/命名空间并拆分驼峰，只显示模组名，例如 `moonslime.MultipleConstructionOrders.CP` 显示为 `Multiple Construction Orders CP`。
- 多个依赖超过 2 个时标签压缩为前两个加“等 N 个”；`.sd-mods-dependency-tag` 单行省略并限制在卡片宽度内，避免长依赖名撑出文本框。
- 该标签普通用户可见；同步分类下拉任意登录用户可用，删除按钮仍仅管理员可用。当前不做缺失状态红绿判断，只提示前置依赖信息。
- 验证：`npm.cmd run build` 通过。

# MODRESTART-1 前端重启提示语义
- ModsPage 上传成功提示改为“下次启动服务器时会自动加载”，不再在停服上传成功后提示需要重启。
- “重启需求”统计改为“运行中重启”，只反映后端返回的 `restartRequired=true` 场景；当前停服 Mod 写操作完成后后端会返回 `false`。

# MODUPLOAD-2 多 ZIP 批量上传入口
- `frontend/src/api.ts` 新增 `uploadMods(files, instanceId?)`，会把多个文件重复 append 为 `mod` 字段后提交到原有 `POST /api/instances/:id/mods/upload`；原 `uploadMod(file)` 保留为单文件兼容封装。
- `ModsPage` 的上传弹窗从单文件状态改为 `File[]`，文件选择器启用 `multiple`，选择后显示文件数量、总大小，数量不超过 5 个时额外展示文件名列表。上传成功或关闭弹窗时会清空 state 和 `<input>` 的值，避免重新选择同一批文件时浏览器不触发 change。
- 旧版 `ModsSection` 也同步改为 `File[] + uploadMods`，保持后续如果仍被引用时行为一致。
- 运行中、非管理员等原有禁用条件不变；按钮只是在未选择任何 ZIP 时禁用，不再要求只有一个文件。
- 验证：`npm.cmd run build` 通过。

# NEXUS-META-1 已安装卡片缩略图
- 前端无需新增请求：`GET /api/instances/:id/mods` 后端会自动用 Nexus GraphQL v2 为带 `UpdateKeys: ["Nexus:<id>"]` 的本地/手动上传 Mod 补齐 `pictureUrl/nexusSummary/nexusUrl/downloadCount/endorsementCount/updatedAt`。
- `ModsPage` 已安装 Mod 卡片继续优先使用 `pictureUrl`，无图时回退本地 Mod 图标；因此手动上传的 Mod 只要 manifest 声明 Nexus 更新键，刷新列表后也能展示与搜索结果一致的 Nexus 缩略图。
- 数字 ID 搜索不再要求 Nexus API Key 才能展示元数据；Key 只和受限下载/安装链路有关。

# MODSEARCH-3 前端展示顺序
- `GET /api/instances/:id/mods/search` 已由后端按 `downloadCount` 从高到低排序，`ModsPage` 直接按接口顺序渲染搜索卡片，不在前端重复排序。

# MODSEARCH-1 前端统一搜索卡片

- `ModsPage` 下载页从“在线搜索（Nexus Mods）”调整为“统一搜索”，调用新接口 `searchMods()`（`GET /api/instances/:id/mods/search`），结果类型改为 `ModSearchResult`。
- 搜索结果卡片组件改为 `ModSearchResultCard`，展示 `来源：{sourceName}`。N站结果按钮显示 `跳转 N站`，其他来源后续按后端返回的 `externalLabel` 显示为 `跳转 {网站名}`。
- 安装按钮不再写死“安装到服务器”，而是使用后端返回的 `installLabel`。当前 N站结果显示 `一键安装`；direct URL 结果后续显示一键安装；`none/manual` 会禁用并提示该来源暂不支持一键安装。
- `installSearchedMod()` 调用 `POST /api/instances/:id/mods/search/install`，继续复用原安装进度面板、SSE 日志订阅和安装后刷新逻辑。粘贴链接安装入口保留，作为管理员粘贴 ZIP/NXM/CDN 链接的兜底。
- 已安装 Mod 列表也复用统一卡片：有 Nexus 元数据时来源显示 N站；本地上传/无来源元数据时来源显示本地。
- 相关文件：`frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/api.ts`、`frontend/src/types.ts`。
- 验证：`npm.cmd run build` 通过。

# REMOTE-MOD-1 前端入口
- `ModsPage` 下载页新增管理员专用“粘贴链接安装”按钮。服务器停止时可打开弹窗，粘贴 `nxm://...` 或 Nexus CDN / ModDrop / GitHub / CurseForge 等来源的 `https://*.zip` 链接后调用 `installRemoteMod()`。
- 远程安装与原 Nexus 一键安装共用同一个安装进度面板、SSE 订阅和任务完成后的 `loadMods()` / `dashboardData.refreshMods()` 刷新逻辑。
- Nexus Premium 直连安装如果任务失败且错误包含 403，前端会提示非 Premium 用户改用 NXM 链接、浏览器生成的 nexus-cdn `.zip` 临时链接，或 ModDrop/GitHub/CurseForge `.zip` 直链继续安装。
- 当前 UI 只承诺 ZIP 直链，避免误导用户以为 7z/rar 已支持。

# NEXUS-3 前端安装入口与统一卡片

- `ModsPage` 下载页文案后续应调整为：无 Key 时可使用 GraphQL v2 关键词搜索和数字 ID 元数据查询；Nexus Mods API Key 仅用于受限下载/一键安装能力。
- 搜索结果卡片的“安装待接入”已替换为真实“安装到服务器”按钮。按钮仅管理员可见可用，且要求服务器停止、Nexus Key 已配置、当前没有其他 Nexus 安装任务、该 Mod 尚未安装。
- 点击安装后调用 `installNexusMod`，订阅 `mod_nexus_install` job SSE 日志，在下载页展示安装进度；任务成功后刷新 `dashboardData.refreshMods()` 和本页 Mod 列表，并把搜索结果标记为已安装。
- 已安装 Mod 列表改用与搜索结果相同的 `NexusResultCard` 展示结构，缩略图优先使用后端返回的 `pictureUrl`，无 Nexus 元数据时回退到本地 Mod 图标；同步分类、删除按钮和 UniqueID 放在同一卡片底部。
- 相关文件：`frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/api.ts`、`frontend/src/types.ts`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`npm.cmd run build` 通过。

# 前端文档

## 总体结构

前端使用 React + TypeScript + Vite。`App.tsx` 负责启动、初始化、登录和进入 Stardew 面板；Stardew 专属页面放在 `frontend/src/games/stardew`。

推荐边界：

```text
frontend/src/api.ts                    后端 API 封装
frontend/src/types.ts                  前后端类型
frontend/src/core                      通用组件与 helper
frontend/src/games/stardew             Stardew 面板
frontend/public/assets/stardew/ui      生产 UI 素材
```

不要让业务组件依赖 `docs/prototypes` 路径；生产素材必须在 `frontend/public/assets/...` 下。

## 路由

当前保持 Single Game Mode。登录后默认进入：

```text
/instances/stardew/overview
```

Stardew 面板内部路由：

| 路由 | 用途 |
| --- | --- |
| `install` | 安装向导、Steam Auth、任务日志 |
| `overview` | 日常总览、邀请码、状态摘要、近期任务 |
| `server` | 启停重启、命令、喊话、控制信息 |
| `saves` | 存档列表、新建、上传预览、选择、删除、导出 |
| `jobs` | 任务与日志 |
| `players` | 玩家名册、在线状态、位置展示 |
| `mods` | Mod 工作台：下载模组、添加模组、配置模组 |
| `diagnostics` | 健康检查、Docker/Compose、支持包 |
| `settings` | 面板用户、审计日志、版本、登出 |

当前未使用 `react-router-dom`，路由通过内部 route + History API 管理。进入 Multi Game Mode 时再考虑正式路由库。

## 数据层

`useStardewDashboardData` 是 Stardew 页面共享数据层，集中维护：

- 实例状态。
- 邀请码。
- saves/mods/jobs/health/players 等摘要。
- 操作后的刷新函数。
- 启动/重启后等待新邀请码的轮询。

页面组件优先调用共享数据层和 `api.ts` 中已有函数，不要在页面里重复拼 API。

## UI 与素材

Stardew UI 使用像素风资源：

```text
frontend/public/assets/stardew/ui/backgrounds
frontend/public/assets/stardew/ui/buttons
frontend/public/assets/stardew/ui/fields
frontend/public/assets/stardew/ui/icons
frontend/public/assets/stardew/ui/navigation
frontend/public/assets/stardew/ui/panels
frontend/public/assets/stardew/ui/sprites
```

重要原则：

- 保留素材原文件名、尺寸和目录结构，避免 CSS 路径失效。
- 图标、按钮、输入框素材从 `public` 进入构建产物，`npm run build` 后同步到 `dist/assets/...`。
- `new-game` 资产和 UI 资产分开维护，不要误改角色/农场预览素材。
- UI 文案要短，按钮和卡片在 320px 宽度也不能溢出。

## 页面职责

| 页面 | 已接入重点 | 注意事项 |
| --- | --- | --- |
| Overview | 状态、邀请码、快速操作、当前存档、健康摘要 | 不承载全部复杂管理 |
| ServerControl | 生命周期、命令、喊话、邀请码刷新 | 危险操作要确认 |
| Install | install job、Steam Guard、日志流 | 不能丢失认证交互 |
| Saves | 新建、上传、选择、删除、导出、备份入口 | running/starting 禁止危险写操作 |
| JobsLogs | 任务列表、日志详情、SSE | 长日志要可滚动 |
| Players | 玩家名册、位置、tile/pixel、中文地图名 | 第三方地图 key 未知时保留原名 |
| Mods | 三段式 Mod 工作台：下载模组（Nexus 在线搜索）、添加模组（已安装列表/玩家同步包/上传删除导出）、配置模组（后续 SMAPI 配置入口） | 运行中限制危险操作；同步分类任意登录用户可改；Nexus 搜索任意登录用户可用；当前 Nexus 安装按钮仍为待接入 |
| Diagnostics | 健康检查、Docker、支持包 | 技术信息不要淹没用户 |
| Settings | 用户、审计、版本、登出 | 面板用户不要放进玩家页 |

## 近期前端修正摘要

- `ModsPage` 参考 `E:\源码\emp_源码\dst-management-platform-web\src\views\game\mod.vue` 的 Mod 管理结构，改为页内三段工作台：`下载模组`、`添加模组`、`配置模组`。下载页承载 Nexus 搜索和卡片网格；添加页承载本服已安装 Mod、玩家同步统计、同步包导出、上传/删除/整包导出；配置页先提供启用/禁用、依赖检查、更新检查、SMAPI 配置的占位入口，等待后端能力接入。该改动只调整前端组织和视觉层级，不新增接口。
- `ModsPage` 新增”在线搜索（Nexus Mods）”区域（未新建路由，无需管理员权限）：搜索框 + 搜索按钮，关键词为空时按钮禁用并提示；搜索中按钮显示”搜索中…”；失败显示中文错误条（复用 `sd-mods-list-error`）。结果用 `NexusResultCard` 卡片展示名称、作者、版本、更新时间、下载量、认可数、已安装标记（绿色 `sd-tag`，标注已安装版本号）和”打开 N 站”按钮（`window.open` 新窗口打开 `nexusUrl`）；本阶段不放安装按钮，只放一个禁用的”安装待接入”小标记。管理员在下载页头部可点“配置 API Key”粘贴 Nexus Key，前端调用 `/api/settings/nexus/api-key` 写入 SQLite 设置，保存后立即生效，页面只显示配置状态和末 4 位。涉及 `frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`（新增 `.sd-mods-nexus-*` / `.sd-mods-panel-actions` 类）、`frontend/src/types.ts`（`NexusModSearchResult`/`NexusModSearchResponse`/`NexusSettingsStatus`，`ModInfo` 新增 `updateKeys?`/`nexusModId?`）、`frontend/src/api.ts`（`searchNexusMods`、`getNexusSettings`、`saveNexusAPIKey`、`deleteNexusAPIKey`）。
- `ModsPage` 新增”玩家同步”区域（未新建路由）：Mod 卡片用 `sd-tag` 展示同步标签（服务器专用/玩家需同步/待确认），任意登录用户都可用下拉框就地修改分类；区域顶部显示三类统计 tag 和“导出玩家同步包”按钮，无 `client_required` Mod 时按钮禁用，导出中显示 loading，失败显示中文错误。后端会自动把内容包和第三方 Mod 默认标为玩家需同步，玩家可再手动改。涉及 `frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`、`frontend/src/types.ts`、`frontend/src/api.ts`。
- 登录/首次注册页已接入 `image2` 原型图整页背景，账号/密码区域、错误提示和按钮文字由前端按背景图风格覆盖绘制；首次注册态底部提示“请尽快注册管理员账号”，按钮显示“注册”，登录态按钮显示“登录”。
- 左侧导航、按钮、输入框、图标、面板等位图资源经过多轮重绘。
- StardewShell 已拆出 9 个路由。
- 服务器控制页、存档页、任务页、玩家页、Mod 页、诊断页和设置页已真实化。
- 邀请码启动/重启后会等待新码，Overview 也提供刷新按钮。
- 玩家位置支持 SMAPI 精确字段、tile/pixel 坐标和原版地图中文映射。
- 玩家页固定展示现金、农场收入、个人收入和钱包模式；农场收入/个人收入不随共享或分开钱包切换含义。
- 玩家页“玩家活动 / 最近事件”已接入后端 `recentEvents`，展示首次记录、加入和离开事件。
- Stardew Shell 已固定为视口高度；长页面只滚动中间 `.sd-main` 内容区，左侧导航、顶部状态栏和右侧任务栏保持固定，移动端顶部栏与横向导航同样不参与页面文档滚动。

## 前端验证

常用命令：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

开发服务器：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run dev
```

视觉 QA 至少覆盖：

- 桌面宽屏。
- 390px 手机宽度。
- 320px 极窄宽度。
- 登录页、Overview、Server、Saves、Players、Diagnostics。
- 长中文按钮、错误提示、Modal、表格转窄屏布局。
# SMAPI-RUNTIME-1 ModsPage 置顶显示 SMAPI

- `ModsPage` 现在会识别后端返回的 `mod.builtIn=true` 条目，并把 SMAPI 作为已安装列表中的置顶内置组件显示。
- 内置 SMAPI 卡片仍复用已安装 Mod 卡片样式；在当前 Nexus 视角下会显示为 Nexus:2400，跳转按钮指向 N 站页面，操作区只显示“内置”，不渲染删除按钮。
- 底部标签显示“置顶 / 玩家需先安装 / 不打包进同步包”；管理员也不会看到同步分类下拉，避免把 SMAPI 当成普通 Mod 操作。
- 玩家同步统计会排除 `builtIn` 条目，避免只有 SMAPI 时误启用“导出玩家同步包”。
- 验证：`npm.cmd run build` 通过。

# MODORIGIN-1 已安装卡片来源包展示

- `ModInfo` 类型新增 `originSource/originNexusModId/originModName/originModUrl`；`ModSource` 新增 `nexus_package`，`ModSearchResult` 新增 `sourceDetail`。
- `ModsPage` 的已安装卡片继续复用 `ModSearchResultCard`。如果 `mod.nexusModId` 存在，来源显示为 `来源：N站` + `Nexus:<id>`；如果没有自己的 Nexus ID 但有 `originSource=nexus`，来源显示为 `来源：N站包`，并额外显示 `随 <originModName> 安装`。
- 典型 UI：主 Mod 显示 `来源：N站`、`Nexus:47289`；`[CP]` 内容包显示 `来源：N站包`、`随 Multiple Construction Orders 安装`。跳转按钮对内容包指向 `originModUrl` 或 Nexus 来源包页面。
- 内容包仍可使用后端返回的 `pictureUrl`，因此手动上传 Nexus ZIP 后，主 Mod 与同包内容包可以展示相同的 Nexus 缩略图。
- 已安装列表会按来源包 bundle 排序，同一个 Nexus 安装包导入出的主 Mod 和内容包相邻显示，主 Mod 排在内容包前面。
- 删除同包任意成员时，确认弹窗会列出将一起删除的同包 Mod，并提示“删除时需要和同包 Mod 一起删除”；确认后仍调用原 `DELETE /mods/:id`，后端负责捆绑删除。
- 验证：`npm.cmd run build` 通过。
# NEXUS-PAGED-1 ModsPage 只走 Nexus 搜索

- `ModsPage` 下载页不再调用 `searchMods` / `/mods/search` 统一搜索接口，改为直接调用 `searchNexusMods` / `/mods/nexus/search`。
- `searchNexusMods(query, page, pageSize)` 会传 `page/pageSize`，页面展示 `total/page/hasMore` 并提供上一页/下一页按钮。
- 搜索结果仍复用现有卡片视觉，但数据源只映射 Nexus 原始结果；安装按钮调用 `installNexusMod` / `/mods/nexus/install`，文案保持“一键安装”。
- 页面文案改为“搜索 Nexus Mods”，粘贴链接安装入口只描述 Nexus `nxm://` 与 Nexus CDN 临时 ZIP 链接，不再把其他站点作为搜索来源展示。
- 涉及文件：`frontend/src/api.ts`、`frontend/src/types.ts`、`frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`npm.cmd run build`。
# NEXUS-PAGER-2 搜索结果分页控件

- `ModsPage` 的 Nexus 搜索结果现在在列表顶部和底部各显示一组分页控件。
- 分页控件支持：首页、上一页、指定页输入跳转、下一页、末页；指定页会按 `1..ceil(total/pageSize)` 自动夹取有效范围。
- 样式新增 `.sd-mods-nexus-page-actions`、`.sd-mods-nexus-page-jump`、`.sd-mods-nexus-page-input`，确保窄屏可换行。
- 验证：`npm.cmd run build`。

# SMAPI-SYNC-2 ModsPage 内置项与玩家同步

- `ModsPage` 现在把 `Pathoschild.SMAPI` 作为内置但可计入玩家同步的运行组件：它继续置顶显示、没有删除按钮和同步分类下拉，但会计入“玩家需同步”统计，并可触发导出玩家同步包。
- `StardewAnxiPanel.Control` 会作为内置服务端控制组件显示：卡片操作区只显示“内置”，底部标签显示“内置 / 服务端控制 / 不打包进同步包”，不渲染删除按钮，也不计入玩家同步统计。

# PLAYERSYNC-PACK-15 前端记录

- `frontend/src/api.ts` 新增 `exportModSyncUpdatePack()`，调用 `POST /api/instances/:id/mods/sync-pack/export-update` 并下载 `stardew-player-mods-update-pack.zip`。
- `ModsPage` 玩家同步区域将原单按钮拆成两个按钮：`导出完整同步包` 用于首次加入玩家，继续包含 SMAPI；`导出模组更新包` 用于已经运行过同步包的玩家，不包含 SMAPI。
- 两个导出按钮共用错误提示，但 busy 状态按 `full/update` 区分；更新包只有存在真实可打包的玩家 Mod 时启用，避免只有虚拟 SMAPI 前置项时导出空更新包。
- 玩家同步提示文案说明客户端会跳过完全相同的 Mod，只备份并覆盖内容不同的同名 Mod。
- 验证：`npm.cmd run build`。
- 内置项排序新增权重：SMAPI 永远排在内置组第一位，面板控制 Mod 排在 SMAPI 后面，避免 Control 抢占 SMAPI 置顶位置。
- 已安装内置卡片中 SMAPI 按 Nexus:2400 指向 N 站页面；Control 没有外部页面，按钮禁用并显示“内置组件”。
- 验证：`npm.cmd run build`。

# MODPROFILE-1 前端记录

- `frontend/src/types.ts` 的 `ModInfo` 新增 `enabled/canToggle/enableNote`，对齐后端按当前存档返回的 Mod 启用状态。
- `frontend/src/api.ts` 新增 `updateModEnabled(modId, enabled, saveName?)`，调用 `PUT /api/instances/:id/mods/:modId/enabled`。
- `ModsPage` 的“配置模组”页从占位改为真实启用列表：按当前激活存档展示每个 Mod 的启用/禁用状态，管理员可在服务器停止时切换；普通用户、运行中状态、内置组件都会禁用开关。
- “添加模组”的已安装 Nexus 卡片底部增加 `已启用/已禁用` 标签，禁用的 Mod 仍会留在列表中，因为后端现在合并扫描 `mods` 与 `mods-disabled`。
- 样式新增 `.sd-mods-enable-*` 与 `.sd-mod-toggle*`，移动端 720px 以下会把状态标签换行，避免长 Mod 名和开关挤压。
- 验证：`npm.cmd run build`。
# MODPROFILE-2 前端记录

- 切换存档后，`SavesPage` 的 `onSavesChanged` 现在同时刷新 `dashboardData.refreshSaves()` 与 `dashboardData.refreshMods()`，避免 ModsPage 继续使用旧存档的全局 mods 缓存。
- `useStardewDashboardData` 新增 active save 监听：只要 `saves.activeSaveName` 发生变化，就自动刷新 mods。这样不管活动存档来自存档页切换、启动流程回写，还是后续其它入口，模组启用/禁用显示都会跟着当前存档更新。
- 涉及文件：`frontend/src/games/stardew/useStardewDashboardData.ts`、`frontend/src/games/stardew/pages/SavesPage.tsx`。
- 验证：`npm.cmd run build`。

# NEXUS-DEFAULT-1 前端记录

- `ModsPage` 下载模组页首次进入时会自动调用 `searchNexusMods('', 1, 20)`，默认展示 Nexus Stardew Valley 热门列表前 20 条。
- 搜索框留空时不再禁用按钮；按钮文案改为“刷新热门”，用于重新拉取默认热门列表。输入关键词或 ID 时仍执行正常搜索。
- 下载页说明文案改为“默认展示 N 站近期热门 20 个模组，也可以输入名称或 ID 搜索”，避免用户进入页面后看到空白搜索区。
- 涉及文件：`frontend/src/games/stardew/pages/ModsPage.tsx`。
- 验证：`npm.cmd run build`。
# NEWGAME-CABINS-1 新建存档小屋数显示

- `NewGameCreator` 左侧“初始联机小屋”数字现在显示真实 `startingCabins`，不再显示 `startingCabins + 1` 的总人数，避免用户选择 2 时实际只发送 1 间小屋。
- 加减按钮仍然调整同一个 `startingCabins` 字段，范围保持 0-7；后端已同步接受 0-7。
- 影响文件：`frontend/src/games/stardew/NewGameCreator.tsx`。
- 验证：`cd frontend && npm.cmd run build` 通过。


# SAVE-BACKUP-POLICY-1 ????

- ????????????????????????????? latest?????????????????????????????? 3 ???? 14 ????????
- ??????????????????? `POST /saves/:name/backup` ????????
- ????????????????????????????????
- ????/API ?? `BackupPolicy`?`BackupMaintenanceResult`?`createSaveBackup`?`updateSaveBackupPolicy`?
- ???`npm.cmd run build` ???

# FE-BACKUP-COPY-1 备份设置文案

- `SavesSection` 的“备份与恢复”设置区已从单行短标签改为“自动备份规则”说明面板。
- `latest` / `scheduled` 等内部命名不再直接展示给用户；文案改为“游戏保存后更新最新备份”“每天固定时间更新定时备份”“每日快照最多保留 N 天”。
- 每项设置补充一行短说明，解释覆盖语义：最新备份和定时备份只覆盖同一份，每日快照每天一份、同日覆盖、超过保留天数自动删除。
- 备份类型标签改为“手动备份 / 最新备份 / 每日快照 / 定时备份”。

# SAVE-BACKUP-SCHEDULE-HOUR-1 定时备份整点设置

- `SavesSection` 的定时备份设置从“每隔 N 小时检查一次”改为“每天 HH:00 执行一次”，使用 00:00-23:00 的 24 小时制下拉框。
- 前端策略类型新增 `scheduledHour`，旧 `scheduledIntervalHours` 只保留为可选兼容字段；读到旧策略时会归一化为默认 04:00，保存时不再提交旧间隔字段。
- 验证：`npm.cmd run build`。
- 验证：`npm.cmd run build` 通过。
# FE-SCHEDULED-RESTART-1 服务器页计划重启

- `ServerControlPage` 的“快捷操作”中，“计划重启”按钮已从待接入改为管理员可点击入口。
- 点击后打开弹窗，读取 `GET /api/instances/:id/restart-schedule` 并编辑：是否启用、关闭时间、开启时间、时区、关服前提醒分钟、关闭前备份、有人在线则跳过。
- 保存调用 `PUT /api/instances/:id/restart-schedule`，保存后弹窗展示后端返回的下次关闭/开启时间和上次执行状态。
- 前端新增 `RestartSchedule` / `RestartScheduleResult` 类型，以及 `getRestartSchedule()` / `updateRestartSchedule()` API helper。
- 影响文件：`frontend/src/games/stardew/pages/ServerControlPage.tsx`、`frontend/src/api.ts`、`frontend/src/types.ts`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build`。
