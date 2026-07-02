# PERF-REVIEW-1 ModsPage 派生数据缓存

- `ModsPage` 的已安装 Mod 派生数据改为 `useMemo` 缓存，并把排序后的 Nexus 展示列表、本地隐藏列表、解析错误数、玩家同步统计和可打包数量合并到一次遍历中。
- 扩展批量安装进度、分页输入、Nexus Key 状态等频繁局部 state 变化时，不再反复对同一份 `mods` 做多次 `filter` / `sort`。
- UI 与接口契约不变；该优化只减少重复渲染计算和临时数组分配。
- 验证：`cd frontend; npm.cmd run build`。

# NEXUS-EXT-3 前端扩展安装入口

- `ModsPage` 的 Nexus 搜索结果“一键安装”不再直接调用 `installNexusMod()` / `POST /mods/nexus/install`，改为同页跳转到 `https://www.nexusmods.com/stardewvalley/mods/:modId?tab=files&anxi_auto=1`，让浏览器扩展在用户已登录 Nexus 的本地浏览器里完成下载链接获取。
- 搜索结果安装按钮不再要求 Nexus API Key；仍要求管理员、服务器停服、目标 Mod 未安装，且当前没有远程安装忙碌状态。
- `JobsLogsPage` 支持 `?jobId=` 查询参数。扩展提交成功后跳回 `/instances/:id/jobs?jobId=<jobId>`，页面会优先选中该任务并打开实时日志。
- `ModInstallMethod` 新增 `nexus_extension`，用于区分当前扩展链路和旧的后端 Nexus premium 下载链路。
- 涉及文件：`frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/pages/JobsLogsPage.tsx`、`frontend/src/types.ts`、`browser-extensions/nexus-slow-installer/*`。
- 验证：`cd frontend; npm.cmd run build` 通过；扩展脚本 `node --check` 通过。

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
- `frontend/src/api.ts` 使用 `uploadMods(files, instanceId?)`，会把多个文件重复 append 为 `mod` 字段后提交到原有 `POST /api/instances/:id/mods/upload`。
- `ModsPage` 的上传弹窗从单文件状态改为 `File[]`，文件选择器启用 `multiple`，选择后显示文件数量、总大小，数量不超过 5 个时额外展示文件名列表。上传成功或关闭弹窗时会清空 state 和 `<input>` 的值，避免重新选择同一批文件时浏览器不触发 change。
- 旧版 `ModsSection` 已清理删除，当前只维护路由页 `pages/ModsPage.tsx` 这一套 Mod UI。
- 运行中、非管理员等原有禁用条件不变；按钮只是在未选择任何 ZIP 时禁用，不再要求只有一个文件。
- 验证：`npm.cmd run build` 通过。

# NEXUS-META-1 已安装卡片缩略图
- 前端无需新增请求：`GET /api/instances/:id/mods` 后端会自动用 Nexus GraphQL v2 为带 `UpdateKeys: ["Nexus:<id>"]` 的本地/手动上传 Mod 补齐 `pictureUrl/nexusSummary/nexusUrl/downloadCount/endorsementCount/updatedAt`。
- `ModsPage` 已安装 Mod 卡片继续优先使用 `pictureUrl`，无图时回退本地 Mod 图标；因此手动上传的 Mod 只要 manifest 声明 Nexus 更新键，刷新列表后也能展示与搜索结果一致的 Nexus 缩略图。
- 数字 ID 搜索不再要求 Nexus API Key 才能展示元数据；Key 只和受限下载/安装链路有关。

# NEXUS-PAGED-1 / NEXUS-PAGER-2 前端搜索

- `ModsPage` 下载页当前只调用 Nexus 专用接口 `searchNexusMods()`（`GET /api/instances/:id/mods/nexus/search`），不再调用已撤回的 `/mods/search` 统一搜索骨架。
- 搜索结果仍复用 `ModSearchResultCard` 作为展示模型，但数据来源只映射 Nexus 结果；安装按钮调用 `installNexusMod()`，管理员在停服且配置 Key 后可一键安装。
- 搜索结果顶部和底部都有分页控件，支持首页、上一页、指定页、下一页、末页。空关键词合法，用于刷新默认热门列表。
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
| Mods | 三段式 Mod 工作台：下载模组（Nexus 在线搜索/一键安装）、添加模组（已安装列表/玩家同步包/上传删除导出）、配置模组（按当前存档启用/禁用） | 运行中限制危险写操作；同步分类任意登录用户可改；Nexus 搜索任意登录用户可用；依赖缺失检查、更新检查和 SMAPI 配置编辑仍是后续 |
| Diagnostics | 健康检查、Docker、支持包 | 技术信息不要淹没用户 |
| Settings | 用户、审计、版本、登出 | 面板用户不要放进玩家页 |

## 近期前端修正摘要

- `FE-CLEANUP-1`：删除无引用旧 Stardew Section 组件，清理前端死 API 封装和对应类型；`App.css` 裁掉旧单页仪表盘/Section 历史样式，仅保留全局 reset、基础登录表单和 `sd-auth-*` 登录页样式。当前 Stardew 路由页样式由 `StardewPanel.css` 与 `stardew-theme.css` 维护。
- `ModsPage` 参考 `E:\源码\emp_源码\dst-management-platform-web\src\views\game\mod.vue` 的 Mod 管理结构，改为页内三段工作台：`下载模组`、`添加模组`、`配置模组`。下载页承载 Nexus 热门/搜索/分页和一键安装；添加页承载本服已安装 Mod、玩家同步统计、同步包导出、上传/删除/整包导出；配置页按当前激活存档展示启用/禁用开关。依赖缺失检查、更新检查和 SMAPI 配置编辑仍留给后续能力。
- `ModsPage` 的 Nexus 下载页无需管理员即可搜索和查看结果；空关键词默认展示热门列表。管理员可在下载页头部配置 Nexus API Key，停服时可一键安装 Nexus 结果或粘贴 `nxm://` / Nexus CDN `.zip` 临时链接创建安装任务。所有安装仍由后端代理下载并复用 Mod ZIP 安全导入，不让前端直连写服务器目录。
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
# MODDEPS-2 前端依赖状态与禁用安装提示

- `frontend/src/types.ts` 的 `ModDependency` 已对齐后端依赖状态字段：`installed/enabled/installedVersion/satisfied/status`；`NexusModSearchResult` 和 `ModSearchResult` 新增 `installedEnabled`。
- 下载页 Nexus 搜索结果现在区分“已安装”和“已安装但未启用”。当后端返回 `installed=true, installedEnabled=false` 时，卡片显示金色“已安装但未启用”标签，安装按钮文案显示“已安装未启用”，tooltip 引导去“配置模组”开启当前存档。
- 已安装 Nexus 卡片和“配置模组”列表会根据依赖状态显示前置诊断：缺失前置、前置未启用、前置版本不足显示红色标签；版本无法确认显示金色标签。满足依赖时保留原来的“前置：...”提示。
- “配置模组”列表中的依赖诊断标签放在 Mod 名称/UniqueID 下方，不再和“已启用/已禁用”状态、开关挤在同一列；Mod 名称和 UniqueID 固定单行省略，避免长英文名被压成竖排。
- 本次没有新增前端请求；依赖诊断和搜索安装状态都复用现有 `GET /mods` 与 `GET /mods/nexus/search` 响应。
- 验证：`cd frontend; npm.cmd run build`；浏览器 smoke 使用 Vite `http://127.0.0.1:5174/` 验证登录页加载、无 console error/warn、输入框可交互。当前浏览器无登录态，未进入 ModsPage 做真实数据渲染。

# MODREL-1 前端联动更新

- `updateModSyncClassification()` 返回类型改为 `{ mods, syncKind }`，`updateModEnabled()` 返回类型改为 `{ mods, enabled, saveName }`；两个接口都会返回本次受联动影响的 Mod 列表。
- `ModsPage` 不再只更新当前卡片。同步分类和启用/禁用成功后，页面会按 `folderName` 合并后端返回的 `mods[]`，让依赖链、同 Nexus 包成员和共享前置状态立即反映到 UI。
- 前端不复制后端联动规则，只展示结果。当前规则：同步分类按必需依赖连通组一起变，所以“待确认”后再切回“玩家需同步/服务器专用”也会把后置 Mod 一起带回；启用会补前置和同包，禁用会禁同包和下游但保留共享前置。
- 验证：`cd frontend; npm.cmd run build`。

# NEXUS-EXT-1 浏览器扩展实验版

- 新增独立 Chrome / Edge Manifest V3 扩展目录：`browser-extensions/nexus-slow-installer`。该扩展不打进 Vite 前端产物，作为本地手动加载测试包维护。
- 扩展在 `nexusmods.com` Mod 文件页识别 `file_id`，可自动开始捕获并点击 `Slow download`；浏览器生成 `supporter-files.nexus-cdn.com/*.zip?...` 下载任务后，后台脚本通过 `chrome.downloads` 捕获链接、可取消本地浏览器下载，并把链接提交给面板已有 `POST /api/instances/:id/mods/remote/install`。
- 扩展设置页/弹窗可配置面板地址、实例 ID、是否自动开始、是否自动点慢速下载、是否取消本地下载。第一版复用面板管理员登录 Cookie 调接口；若云端部署下浏览器策略导致 401/403，后续应新增扩展专用 token 接口。
- 扩展状态只保存脱敏后的下载 URL，`md5/expires/user_id/key` 不写入明文状态；后端仍负责 ZIP 校验、解压和 Mod 安全导入。
- 验证：对 `browser-extensions/nexus-slow-installer` 内 JS 运行 `node --check`；手动验证需要在 Chrome/Edge 加载已解压扩展、登录面板管理员和 Nexus，停服后打开 N 站文件下载页。
# NEXUS-EXT-2 安装完成后刷新已安装页

- `ModsPage` 的 Nexus/远程安装 job 成功后，会自动切到“添加模组”页，并重新拉取 `GET /api/instances/:id/mods`，再刷新公共 dashboard mods 缓存。
- 后端会把本次导入的 Mod 标记为当前激活存档启用；这样通过浏览器扩展捕获 CDN ZIP 安装成功后，像 SpaceCore 这种带 `UpdateKeys: ["Nexus:1348"]` 的 Mod 会直接出现在“已安装 Nexus 模组”区域，避免用户停留在下载页误以为没有安装。
- 验证：`npm.cmd run build`。
# NEXUS-REQ-1 前置依赖提示与扩展弹窗

- `NexusModSearchResult` 新增 `requiredMods?: NexusRequiredMod[]`，用于展示 Nexus 页面声明的前置 Mod。前端卡片会在 footer 显示“缺少前置/前置未启用/前置已安装”状态。
- 缺失的 Nexus 前置会在当前搜索结果卡片里显示“安装前置”按钮，点击后复用现有扩展一键安装链路，跳转到对应前置 Mod 的 `?tab=files&anxi_auto=1` 页面。
- 浏览器扩展 `content.js` 新增 “Additional files required” 弹窗处理：检测到 Nexus 前置确认弹窗后，只点击弹窗内文本为 `Download` 的按钮，然后继续等待 ZIP 链接。
- 该检测只处理 Nexus 声明的前置 Mod；安装 ZIP 后的 SMAPI `manifest.json` 依赖状态仍由已安装列表的 `dependencies[]` 标签展示。
- 验证：`cd frontend; npm.cmd run build`，以及扩展 `content.js/background.js/shared.js` 的 `node --check`。
# NEXUS-PREMIUM-2 前端入口

- `ModsPage` 已移除管理员“粘贴链接安装”按钮、弹窗、`installRemoteMod()` 前端封装和 `RemoteModInstallRequest` 类型；普通非 Premium 安装继续走浏览器扩展打开 Nexus 文件页并提交临时 ZIP 链接。
- Nexus Key 未配置时，“配置 Nexus Key”按钮左侧显示提示：`如果您是尊贵的 Nexus Premium 用户，请填您的 NexusKey`；Key 已配置后该提示消失，保留已配置状态标签。
- Nexus 搜索结果在 Key 已配置时，每个模组卡片底部都会显示 `N站会员专属安装` 按钮，调用现有 `installNexusMod()` / `POST /api/instances/:id/mods/nexus/install` 直连安装；未安装 Key 时不显示该会员按钮。
- 普通 `一键安装` 按钮仍用于扩展流程，直接跳转 `https://www.nexusmods.com/stardewvalley/mods/:modId?tab=files&anxi_auto=1`。
- 影响文件：`frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`、`frontend/src/api.ts`、`frontend/src/types.ts`。
- 验证：`cd frontend; npm.cmd run build`。
# NEXUS-CARD-UI-1 搜索卡片布局优化

- `ModsPage` 的 Nexus 搜索结果卡片改为内容区、主操作区、次操作区三段式布局；跳转 N 站和普通一键安装两个主按钮固定在同一操作行，避免随简介长短上下漂移。
- `N站会员专属安装` 移到卡片底部次操作区，和前置依赖状态并列展示；配置 Nexus Key 后仍对每个搜索结果显示。
- 前置依赖不再逐个摊开显示，也不再在卡片里渲染“安装前置”小按钮；页面只显示 `缺少前置mod` 或 `前置已满足`。点击或鼠标悬停该状态入口时，会展开具体前置 Mod 名称、NexusId 和安装/启用状态。
- 影响文件：`frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build`。内置浏览器可打开本地登录页且无 console error，但因当前浏览器未登录面板，本次未完成登录后搜索结果截图验证。

# NEXUS-EXT-BATCH-1 后台批量扩展安装

- `ModsPage` 的普通 `一键安装` 不再让当前面板页跳转 Nexus；点击后通过浏览器扩展的 panel bridge 发起批量任务，后台打开当前 Mod 下载页和所有未安装 Nexus 前置 Mod 下载页。
- 按钮本身变成百分比进度条：扩展获取/提交阶段按 `opening=10 / capturing=35 / ready=65 / posting=80 / queued=90` 折算，多个目标取平均值；拿到 `items[].jobId` 后前端继续轮询 `GET /api/jobs/:id`，所有 job `succeeded` 才显示 100%，任一 job `failed/canceled` 会显示失败和对应 Mod 名。扩展未响应、后台页超时或提交失败时，按钮显示 `失败请手动安装`。
- 无 `jobId` 的扩展 item 会刷新本地 Mod 列表做兜底：如果 `nexusModId` 或 `originNexusModId` 已经匹配到该 Nexus modId，前端把该 item 视为完成，避免“实际已安装但扩展 batch 卡在 70% 左右”。
- 根因修复：`CAPTURE_URL` / `SUBMIT_CAPTURED_URL` 消息会携带 `batchId/itemId/autoSubmit`；background 即使最早 `START_CAPTURE` 丢了批量上下文，也会从消息或 `captureKey=batch:item` 反推并写回 capture，确保 `mod_remote_install` 返回的 `jobId` 能落到对应 batch item。
- 卡住恢复：搜索卡片存在扩展安装状态时会显示 `重置状态`，点击后清理前端 `sessionStorage`、停止轮询，并通过 `panel-bridge.js` 转发 `CLEAR_STATE` 清理扩展 `chrome.storage.local` 里的 batch/capture。前后端重启不会清浏览器状态，卡在旧进度时应使用这个入口。
- 已安装但当前存档未启用的前置不会重复下载；仍由配置模组页的启用逻辑处理。缺失前置与当前 Mod 会同时打开后台页，由扩展自动提交 ZIP 链接。
- Nexus 搜索状态和扩展安装 batch 状态会写入 `sessionStorage`；用户切到任务日志等页面再回到模组页时，会恢复搜索词、搜索结果、分页和按钮进度，并继续轮询扩展 batch。
- 扩展在 Nexus 文件列表页找到 `Manual download` 后，会优先读取按钮/链接的 `href` 并直接跳转，同时保留 `anxi_batch/anxi_item/anxi_auto_submit` 参数；若 Nexus 给的是 JS 按钮，则退到主世界 `button.click()`，最后才使用 debugger/鼠标事件兜底。前置确认弹窗里的 `Download` 也优先走链接直跳。这样避免后台非激活标签页里 debugger 坐标点击返回成功但页面不跳转，导致状态卡在“正在进入下载页”。
- 批量自动提交按 ZIP 来源分流：无论 content 直接生成 ZIP 链接还是 Chrome `downloads.onCreated` 捕获 ZIP，Nexus 页都会自动调用原“提交到面板”按钮对应的 `SUBMIT_CAPTURED_URL` 逻辑；background 仅在下载事件消息丢失时延迟兜底接手，避免停在“ZIP 已获取，后台自动提交”。Nexus 页会把 `anxi_batch/anxi_item/anxi_auto_submit` 记入 `sessionStorage`，即使 Nexus 跳转丢失查询参数，拿到 ZIP 后也会自动提交。批量任务提交面板时优先通过已登录的面板标签页 `panel-bridge.js` 发起同源 `POST /api/instances/:id/mods/remote/install`，复用面板 Cookie/Vite proxy；只有面板页桥接不可达时才回退到 background 直连。提交请求有 30 秒超时，失败会回写 batch 状态。
- 相关文件：`frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`、`browser-extensions/nexus-slow-installer/background.js`、`browser-extensions/nexus-slow-installer/content.js`、`browser-extensions/nexus-slow-installer/panel-bridge.js`、`browser-extensions/nexus-slow-installer/manifest.json`。
- 验证：`cd frontend; npm.cmd run build` 通过；扩展脚本 `background.js/content.js/shared.js/panel-bridge.js` 均通过 `node --check`。
# NEXUS-EXT-BATCH-2 扩展批量安装终态修复

- `ModsPage` 的扩展批量安装状态现在把 `done/failed` 视为终态；后续 `GET_BATCH_STATUS` 轮询返回的旧 running batch 不会再把 `100%` 覆盖回安装中。
- 安装完成后会用最新 `GET /mods` 结果回填当前 Nexus 搜索结果和前置依赖的 `installed/installedEnabled/installedFolderName/installedVersion`，切到任务日志再回来也不会把已安装项恢复成“一键安装”。
- 无 `jobId` 但本地 Mod 已经按 `nexusModId/originNexusModId` 命中的兜底逻辑保留；命中时同步更新搜索卡片缓存。
- 验证：`cd frontend; npm.cmd run build`，扩展脚本 `background.js/content.js/shared.js/panel-bridge.js` 均通过 `node --check`。
# NEXUS-EXT-BATCH-3 扩展批量目标去重

- `browser-extensions/nexus-slow-installer/background.js` 的 `START_BATCH_INSTALL` 入口现在会先按 Nexus `modId` 去重，缺少 `modId` 时按清理过批量参数的 URL 去重；同一个 Mod 同时作为前置和本体出现时优先保留本体目标。
- 同一个 `batchId` 被重复发送时，扩展会返回已有 batch 并更新 panel tab 绑定，不再重复打开 Nexus 后台标签页。这样 Ridgeside Village 这类“本体 + 多个前置”批量安装不会因为重复目标留下第二个本体下载页。
- 验证：`node --check browser-extensions/nexus-slow-installer/background.js` 通过。
# NEXUS-EXT-CONNECT-1 扩展连通检测

- `ModsPage` 的下载页在管理员进入后会向浏览器扩展发送 `PING`；同一个按钮放在“配置 Nexus Key”旁边，文案为“检测扩展 / 扩展已连通”。
- `PING` 会携带 `window.location.origin` 和实例 ID `stardew`。扩展桥接脚本先用当前面板页 `GET /api/auth/me` 验证已登录，再把当前面板地址写入扩展配置，避免正式上线后仍停留在旧的 `127.0.0.1:5173`。
- 普通“一键安装”按钮现在依赖扩展连通状态：未检测、检测失败或检测中时灰色禁用，tooltip 提示先检测扩展；连通后才允许走后台批量扩展安装。`N站会员专属安装` 仍只依赖 Nexus Key，不受扩展连通状态影响。
- 检测按钮右侧会直接显示当前结果或错误原因，避免扩展未注入/未重新加载时用户看起来像“点击没反应”。
- 连通成功必须以扩展返回的 `panelBaseUrl` origin 等于当前 `window.location.origin` 为准；换端口后如果扩展仍是旧地址，前端显示错误而不是“已连通”。
- `panel-bridge.js` 只对 `PING` 放行自动注册当前面板；其它 `START_BATCH_INSTALL`、`GET_BATCH_STATUS`、`CLEAR_STATE` 仍要求当前页面 origin 和扩展配置一致。
- 验证：`node --check browser-extensions/nexus-slow-installer/background.js`、`node --check browser-extensions/nexus-slow-installer/content.js`、`node --check browser-extensions/nexus-slow-installer/shared.js`、`node --check browser-extensions/nexus-slow-installer/panel-bridge.js`、`cd frontend; npm.cmd run build`。
# NEXUS-EXT-PACK-1 前端扩展安装引导

- `ModsPage` 下载页在 `配置 Nexus Key` 按钮右侧新增提示：`Nexus 普通用户启用一键下载，请先安装浏览器扩展`。
- 提示右侧新增 `下载浏览器扩展` 按钮，调用 `downloadNexusInstallerExtension()` 下载后端生成的 `anxi-nexus-installer.zip`；下载中显示 `打包中...` 并禁用按钮。
- 下载失败会写入当前 Nexus 安装错误区域，便于直接看到扩展源码缺失或后端打包失败原因。
- `api.ts` 新增 `GET /api/instances/:id/mods/nexus/extension/download` 的 blob 下载封装，继续复用面板登录 Cookie。
- 验证：`cd frontend; npm.cmd run build`。
# NEWGAME-PLAYERLIMIT-1 新建存档人数上限

- `NewGameCreator` 左侧联机设置新增“联机人数上限”步进器，提交字段为 `maxPlayers`，默认 `10`，范围 `1-100`。
- “初始联机小屋”仍显示并提交真实 `startingCabins`，范围保持 `0-7`；增加小屋时会自动把 `maxPlayers` 提高到至少 `startingCabins + 1`，降低人数上限时也不会低于当前小屋数加主玩家。
- 用户语义：小屋数决定新存档初始可见小屋，人数上限决定 Junimo 允许的最大同时在线人数；超过 7 的玩家由 Junimo 的 `CabinStack` 自动小屋管理接住，不需要在前端把小屋数放到 7 以上。
- 影响文件：`frontend/src/games/stardew/NewGameCreator.tsx`、`frontend/src/types.ts`。
- 验证：`cd frontend; npm.cmd run build` 通过；后端 `WriteServerSettings|ValidateNewGameConfig` 针对性测试通过。
# VNC-CONTROL-1 服务器页 VNC 入口

- `ServerControlPage` 的“快捷操作”新增 VNC 显示切换入口：服务器运行时先调用 `getInstanceRenderingFPS()` 读取真实渲染 FPS，刷新页面后也能恢复 `关闭VNC显示` 状态；`打开VNC显示` 调用 `setInstanceRenderingFPS(15)`，成功后按钮切换为 `关闭VNC显示` 并调用 `setInstanceRenderingFPS(0)` 关闭；`跳转VNC控制` 默认隐藏，仅在显示渲染打开后出现，读取 `getInstanceVNCConfig()` 返回的 `vncPort` 并打开 `http://<当前hostname>:<vncPort>/`。
- 两个按钮仅在服务器 `running` 时可用；普通用户不可用。打开显示成功/失败和跳转窗口拦截会在快捷操作区显示结果。
- 前端新增 `InstanceRenderingResult` 类型与 `getInstanceRenderingFPS()` / `setInstanceRenderingFPS()` API helper；跳转入口继续复用已有 `GET /api/instances/:id/config/vnc-port`，支持用户自定义 VNC 端口。
- 验证：`cd frontend; npm.cmd run build`。
