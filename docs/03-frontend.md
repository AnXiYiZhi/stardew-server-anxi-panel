# FE-MAIN-PAGE-FRAME-SLICES-1 主内容 Frame 切片平铺

- 所有 Stardew 路由共用的 `.sd-main` 主内容背景不再把 `main_page_frame_empty_image2.png` 整图 `100% 100%` 拉伸；已从 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/03-saves-page-frame-empty-image2.png` 按原始 image2 空框确定性裁出 4 个角、4 条边和中心羊皮纸 tile。
- 新增运行时素材：`frontend/public/assets/stardew/ui/panels/main_page_frame_corner_*_image2.png`、`main_page_frame_edge_*_tile_image2.png`、`main_page_frame_center_tile_image2.png`。四角固定绘制，顶部/底部 `repeat-x`，左/右 `repeat-y`，中心纸纹 `repeat`，窗口缩放时边框纹理不会被横向或纵向拉伸。
- `stardew-theme.css` 新增 9 个 frame 切片资源变量；`StardewPanel.css` 的 `.sd-main` 改为 9 层 background，保留 `.sd-main-scroll` 作为唯一滚动视口，原有内侧 inset、裁切和所有页面业务 DOM 不变。
- 影响文件：`frontend/src/games/stardew/stardew-theme.css`、`frontend/src/games/stardew/StardewPanel.css`、新增 `frontend/public/assets/stardew/ui/panels/main_page_frame_*_image2.png`。
- 验证：`cd frontend; npm.cmd run build` 通过；使用临时 `page-frame-slices-qa.html` 加载真实 CSS/素材做内置浏览器 QA，1280x720 下 `.sd-main` 背景为 9 层，`background-repeat` 为四角 no-repeat、四边 repeat、中间 repeat，滚轮后 `.sd-main-scroll.scrollTop` 从 `0` 到 `520`；390x760 下同样 9 层背景、无页面级横向溢出，滚轮后 `scrollTop=420`，console error/warn 为空。临时 QA 文件已删除。

# FE-MODS-DEPENDENCY-POPOVER-1 下载模组页前置信息弹层修复

- 下载模组页 Nexus 搜索结果里的“缺少前置mod / 前置已满足”不再使用 `<details>` 默认开合；改为 React 受控按钮和弹层，当前只记录一个打开的 Nexus `modId`。
- 鼠标点击前置信息弹层外部会自动收起；切换到其它 tab、搜索结果刷新后当前打开项不存在时也会自动关闭。按 `Escape` 也可关闭。
- 修复动态分页搜索卡片固定高度导致前置弹层被裁切的问题：搜索卡片默认仍保持 `246px` 和 `overflow: hidden`，只有当前卡片前置弹层打开时临时给卡片和 footer 加 `overflow: visible` 与更高层级，关闭后恢复原裁切，不影响 pageSize 测量。
- 影响文件：`frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器通过临时 QA 页加载真实构建 CSS，验证 1280x720 下点击前置标签可展开、弹层不被卡片裁切、点击信息页外部自动收起且 console error/warn 为空；390x760 下弹层宽度在视口内且无水平溢出。临时 QA 文件已删除。

# JOB-DISPLAY-NAME-1 任务列表显示 Mod 名

- 前端 `Job` 类型新增可选 `displayName`，任务页、右侧 OpsRail“进行中”、右侧“近期任务”和总览页近期事件都优先展示 `displayName`，没有该字段时回退原来的任务类型/类型标签。
- 这样浏览器扩展普通一键安装并行创建多个 `mod_remote_install` 时，用户能看到 `mod_remote_install · Farm Type Manager (FTM)`、`mod_remote_install · Content Patcher` 这类可区分任务名。
- 新增 `jobDisplayName(job)` helper，集中处理展示名回退，避免各页面各自拼接。
- 影响文件：`frontend/src/types.ts`、`frontend/src/core/helpers.ts`、`frontend/src/games/stardew/StardewPanel.tsx`、`frontend/src/games/stardew/pages/JobsLogsPage.tsx`、`frontend/src/games/stardew/pages/OverviewPage.tsx`。
- 验证：`cd frontend; npm.cmd run build`。

# FE-OPSRAIL-DOWNLOAD-PROGRESS-1 右栏进行中接入远程 Mod 下载进度

- 右侧 OpsRail 的“进行中”卡不再只按历史任务耗时估算 `mod_remote_install` / `mod_nexus_install` 进度；这些远程 Mod 安装任务会优先读取任务日志中的 `下载进度：已下载 ...（xx.x%）`，把真实下载百分比映射到右栏进度条。
- 远程安装的阶段映射：任务启动/准备下载显示低进度，远程服务器响应和压缩包大小日志显示 6%~12%，下载 body 阶段按真实下载百分比推进到约 90%，进入“正在校验并安装 Mod”后显示约 92%，完成前不显示 100%，完成后任务行由既有 SSE finished 刷新移除。
- `useStardewDashboardData` 现在会为已知 queued/running job 拉取一次初始日志，并订阅 `GET /api/jobs/:jobId/stream` 的 `log` 事件，维护 `jobLogsByJobId` 供右栏消费；每个 job 只保留最近 200 条日志，避免右栏组件自己额外轮询。
- 模组页普通一键安装的扩展 batch 一旦返回新的 `jobId`，会立即调用 `dashboardData.refreshJobs()`；因此右栏“进行中”会在扩展创建后端任务后尽快出现，不再依赖 30s dashboard 轮询。
- Premium/API Key 安装路径拿到 `jobId` 后也会主动刷新 jobs，保持两条安装链路在右栏展示一致。
- 影响文件：`frontend/src/games/stardew/useStardewDashboardData.ts`、`frontend/src/games/stardew/StardewPanel.tsx`、`frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/stardew-routes.ts`。
- 验证：`cd frontend; npm.cmd run build` 通过。

# NEXUS-EXT-DOWNLOAD-GUARD-1 扩展安装任务提交防线

- 浏览器扩展的自动提交链路增加最终 URL 校验：`background.js` 的 `finishInstall()` / `postRemoteInstall()` 与 `panel-bridge.js` 的 `PANEL_REMOTE_INSTALL` 只接受 `*.nexus-cdn.com` 下以 `.zip` 结尾的 Nexus CDN 链接。
- 如果后台 Nexus 页面仍停留在普通下载页、Manual Download 页、Slow Download 页、Additional files 弹窗或错误页，扩展不会创建面板远程安装任务；批量项会继续停留在捕获中，或在超时后由既有 batch timeout 标成失败。
- 这样面板下载页的“普通一键安装”不会再把“还没拿到 ZIP”的页面状态误报成已创建后端任务；真正的后端安装结果仍以 `mod_remote_install` job 的状态为准。
- 验证：`node --check browser-extensions/nexus-slow-installer/background.js`、`node --check browser-extensions/nexus-slow-installer/panel-bridge.js`、`node --check browser-extensions/nexus-slow-installer/content.js`。

# FE-INSTALL-IMAGE2-ICONS-2 安装页手绘图标替换为 image2 PNG 素材

- 针对安装页上一轮重皮肤中 CSS 自绘图标质感不佳的问题，已从 image2 安装页原型 `08-install - 副本.png` 提取并抠图生成透明 PNG 小素材，替换顶部状态横幅土芽和五步安装时间线图标。
- 新增素材目录：`frontend/public/assets/stardew/ui/install/`。包含 `icon_install_status_seed_image2.png`、`icon_install_step_seed_image2.png`、`icon_install_step_box_image2.png`、`icon_install_step_steam_image2.png`、`icon_install_step_download_image2.png`、`icon_install_step_star_image2.png`。
- 未把原型整图作为页面背景或整块资源引用；只使用 6 个独立透明小图标。页面纸张背景、卡片、边框、分隔线、进度条、日志终端等结构仍由 CSS gradient / border / box-shadow / pseudo-elements 实现。
- `InstallPage.tsx` 将步骤图标从 `STEP_ART_CLASS` + CSS class 切换为 `STEP_ICON_SRC` + `<img>`；顶部状态横幅土芽改为 wrapper + PNG 图片。安装提交、Steam Guard / QR 交互、SSE 日志、权限判断、API 调用、loading/error/empty/disabled 状态均未改。
- `StardewPanel.css` 删除安装页步骤 seed/box/steam/download/star 的伪元素绘制规则，并移除顶部土芽的 CSS 土堆/嫩芽绘制；保留页面级 scoped 尺寸、分隔线、投影和响应式约束。
- 视觉元素到代码实现映射：顶部土芽 -> `icon_install_status_seed_image2.png` + wrapper 分隔线；五步图标 -> 5 个 `icon_install_step_*_image2.png`；图标投影 -> CSS `filter: drop-shadow(...)`；移动端步骤缩小 -> CSS 容器查询下 42px 图标尺寸；页面纸卡/进度/终端 -> 原 CSS 结构继续实现。
- 验证：`cd frontend; npm.cmd run build` 通过；使用已删除的临时 `install-qa.html` / `src/install-qa.tsx` 挂载真实 `InstallPage` + 真实 CSS/素材做内置浏览器 QA，1280x900 与 390x760 均确认 6 个图标加载自 `/assets/stardew/ui/install/`、无页面级横向溢出、按钮无文字溢出、console error/warn 为空；未安装态点击“安装游戏”后表单正常展开。

# FE-PLAYERS-PROTOTYPE-IMAGE2-1 玩家管理页按 image2 原型视觉重皮肤

- 玩家管理页按 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/05-players - 副本.png` 重做首屏视觉：顶部六张摘要卡、邀请加入码横条、中部 Junimo 服务器终端 + 在线玩家表、底部玩家活动与管理员操作区对齐原型结构。
- 未把原型图作为运行时背景或整块资源引用；整页羊皮纸底、纸纹噪点、铜色边框、内描边、角钉、分隔线、绿字终端、表格表头/行分隔、禁用操作小按钮均由 CSS gradient / border / box-shadow / pseudo-elements 实现。
- `PlayersPage.tsx` 保留现有 `dashboardData.players`、`inviteCode`、`saves`、刷新、复制、loading/error/empty/disabled 和管理员权限判断，仅调整展示结构：摘要压成 6 卡，玩家表改为原型式 6 列，现金/收入/钱包/联机 ID 仍作为行 title 辅助信息保留，待接入的踢出/封禁/白名单/权限入口继续禁用。
- 按钮与图标复用现有 Stardew 素材：页头/摘要/分区图标使用 `ui/icons` 下 image2 PNG，复制按钮复用 `sd-btn-green`，刷新和权限按钮复用已有 `.sd-btn-blue`，踢出/封禁复用 `sd-btn-delete`；没有新增图片素材。
- 响应式：玩家页最终覆盖以 `.sd-players-page` 为作用域，并补 `@container sd-main-scroll` 断点；桌面保留六卡和左右分栏，中等宽度收成 3 卡/2 操作，窄屏单列；玩家表仅在自身容器内部横向滚动，不撑宽整页。
- 影响文件：`frontend/src/games/stardew/pages/PlayersPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；真实 `/instances/stardew/players` 当前受登录态影响停在登录页且 console error/warn 为空，因此使用已删除的临时 `frontend/players-qa.html` 加载同一份 CSS/素材/同结构 DOM 做内置浏览器 QA。1280x900 桌面无页面级横向溢出、六卡/邀请码/终端/玩家表首屏可见、表格操作列无需横向滚动；390x760 窄屏页面无横向溢出，邀请码按钮可读，表格仅自身横向滚动。已用 `view_image` 对比原型图与最终桌面/移动截图。

# FE-DIAGNOSTICS-IMAGE2-ICONS-1 诊断页 CSS 图标替换为 image2 PNG 素材

- 针对诊断页首轮重皮肤中“盾牌/宝石/检查项/建议”CSS 自绘图标质感不足的问题，使用内置 Image Gen 按 `07-diagnostics - 副本.png` 的 image2 像素 UI 风格生成 4x5 图标 sprite sheet，并本地抠掉 chroma-key 背景、切片为透明 PNG 生产素材。
- 新增素材目录：`frontend/public/assets/stardew/ui/diagnostics/`。包含 `diag_icon_sheet_image2.png` 以及 20 个单图：状态盾牌（正常/警告/错误）、三色宝石、Docker/Compose/目录/文件/启动存档检查项图标、建议区叶子/灯泡/嫩芽/警告/错误、资源趋势镐子、实时绿点、导出下载图标。
- `StardewPanel.css` 保持诊断页 DOM 不变，只用背景图覆盖 `.sd-diag-status-shield`、`.sd-diag-count-gem`、`.sd-diag-check-icon-*`、`.sd-diag-advice-icon`、资源趋势标题图标、实时徽章图标和导出按钮图标；去掉初版 CSS 图标的 clip-path / gradient / 伪元素残留影响。
- 生成后处理：洋红背景按阈值转 alpha，单图四角 alpha 均为 0；二次清理贴顶小碎片并重新裁切透明边，避免图标上方出现黑白杂点。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`、新增 `frontend/public/assets/stardew/ui/diagnostics/*.png`。
- 验证：`cd frontend; npm.cmd run build` 通过；使用已删除的临时 `diagnostics-icons-qa.html` 加载真实 CSS/素材做内置浏览器 QA，1280x900 与 390x760 均无横向溢出、按钮文字不溢出、console error/warn 为空；浏览器检查 16 个诊断页可见图标背景均来自 `/assets/stardew/ui/diagnostics/`。

# FE-SERVER-PROTOTYPE-IMAGE2-1 服务器页按 image2 原型视觉重皮肤

- 服务器页按 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/02-server-control - 副本.png` 调整为羊皮纸控制台结构：顶部大标题、左侧当前状态大卡、右侧生命周期按钮卡、中部邀请代码与全服消息、下方控制台命令绿字终端和快捷操作条。
- 未把原型图作为运行时背景或整块素材引用；页面底纹、纸卡、铜色描边、inset 高光、分隔线、绿屏邀请码、黑色终端、快捷操作纸卡均为 CSS gradient / border / box-shadow / pseudo-element 实现。
- `ServerControlPage` 只新增视觉外壳和信息分组：状态卡字段化、命令结果合入右侧终端展示、全服消息增加字数显示、快捷操作改为原型式横向按钮条；启动/停止/重启/刷新邀请码/复制/喊话/执行命令/备份/VNC/计划重启等 handler、API、权限判断和 disabled 状态未改。
- 按钮与图标复用既有素材：生命周期按钮继续使用 `sd-btn-start/stop/restart` 与 `icon_button_*` PNG；页头/分区标题复用 `ui/icons` 下的服务器、玩家、存档、时间、诊断等现有图标；状态点复用 `.sd-dot*`。
- 响应式：服务器页规则以 `.sd-server-page` 为作用域，并补 `@container sd-main-scroll` 断点；主内容变窄时页面自动改为单列，命令区、全服消息和快捷操作按容器宽度收缩，输入框使用局部 `box-sizing: border-box` 避免窗口缩小时内部裁切。
- 影响文件：`frontend/src/games/stardew/pages/ServerControlPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；使用已删除的临时 `frontend/server-control-qa.html` 挂载真实 `ServerControlPage` 组件与 mock 数据做浏览器 QA：1280x900 桌面无横向溢出、按钮无文字溢出、命令执行后终端显示输出；390x760 窄屏无横向溢出、消息/命令/快捷操作单列收缩、按钮无溢出。

# FE-DIAGNOSTICS-PROTOTYPE-IMAGE2-1 诊断与健康页按 image2 原型视觉重皮肤

- 诊断与健康页按 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/07-diagnostics - 副本.png` 重做首屏视觉：顶部标题/操作、系统状态横向总览、正常/警告/错误计数、左侧检查项表、右侧资源趋势、底部告警与建议条对齐原型结构。
- 未把原型图作为运行时背景或整块资源引用。页面羊皮纸底、纸纹噪点、面板边框、内描边、虚线分隔、状态点放大和资源仪表盘由 CSS gradient / border / box-shadow / pseudo-elements 实现；盾牌、宝石、检查项和建议区图标已在 `FE-DIAGNOSTICS-IMAGE2-ICONS-1` 中替换为 image2 风格透明 PNG；SVG 趋势图继续使用现有数据绘制。
- `DiagnosticsPage.tsx` 保留既有 `getHealthDiagnostics()`、`downloadSupportBundle()`、`getInstanceMetrics()`、管理员导出权限、loading/error/disabled 状态和 5s metrics 轮询，仅调整 DOM 外壳：新增计数卡、检查表头/图标列、资源标题行和底部全宽建议面板。
- 按钮/素材复用：页头图标复用 `icon_nav_diagnostics_monitor_image2.png`；“重新检查”复用既有绿色 PNG 按钮体系；“导出诊断包”新增诊断页蓝色 CSS 按钮变体，未新增按钮图片。
- 响应式：1180px 以下主内容改单列；760px 以下按钮、计数卡、仪表盘、检查表和建议面板收成移动端单列/紧凑布局，显式避免横向溢出。
- 影响文件：`frontend/src/games/stardew/pages/DiagnosticsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；使用 Vite 本地服务 + 已删除的临时 `diagnostics-qa.html` 加载真实 CSS/素材/同结构 DOM 做内置浏览器 QA：1280x900 桌面无横向溢出、按钮/检查项文字不溢出、主要面板无重叠、console error/warn 为空，点击“重新检查”进入禁用检查中状态；390x760 窄屏无横向溢出，所有主要面板宽度落在页面内。已用 `view_image` 对比原型和最新桌面/移动截图。

# FE-SETTINGS-PROTOTYPE-IMAGE2-1 设置页按 image2 原型视觉重皮肤

- 设置与审计页按 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/09-settings - 副本.png` 重排视觉结构：顶部为“当前账号 / 面板版本 / 安全与权限”三卡，中部为“用户管理 / 审计日志”双栏，底部为“端口信息 + 其他设置 / 安全建议”两栏。业务数据、API 调用、权限判断、弹窗确认、用户创建/角色/禁用/删除、审计分页和 VNC 端口保存逻辑均保持不变。
- 本次没有把原型图作为运行时背景或整块素材引用；页面背景继续使用既有主内容 frame，设置页卡片、纸纹、铜色边框、角钉、内描边、表格表头、行分隔线、底部提示区均由 `.sd-settings-page` 作用域 CSS 使用 gradient、border、box-shadow 和伪元素实现。
- `SettingsPage.tsx` 为各功能区补充页面级 modifier class，并新增 `SecuritySummarySection`，把原来长说明型安全信息保留为底部“安全建议”，同时在顶部提供与原型对应的安全状态摘要。设置页头图标切换为已有 image2 齿轮素材 `icon_nav_settings_gear_image2.png`。
- 按钮与小图标继续复用现有 Stardew 素材体系：按钮为 `sd-btn-green` / `sd-btn-tan` / `sd-btn-delete`，标题图标复用既有导航/顶栏 image2 PNG；没有新增图片素材。窄屏下顶部/中部/底部网格收为单列，用户行按钮换行，审计表在自身容器内横向滚动，页面整体无横向溢出。
- 影响文件：`frontend/src/games/stardew/pages/SettingsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过。真实 `/instances/stardew/settings` 当前停在登录页且 console error/warn 为空；使用已删除的临时 `settings-qa.html` 加载同一份 CSS/素材/同结构 DOM 做视觉 QA：1280x900 桌面下三卡 + 双栏布局、按钮无文字溢出、无横向溢出、console error/warn 为空；点击“新建用户”后表单展开且无横向溢出；390x760 窄屏单列无横向溢出，审计表仅在表格内部横向滚动，底部待接入/禁用按钮可读。已用 `view_image` 对比原型图与最终桌面截图。

# FE-INSTALL-PROTOTYPE-IMAGE2-1 安装页按 image2 原型视觉重皮肤

- 安装页按 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/08-install - 副本.png` 做页面级视觉改造，未把原型图作为运行时背景或整块素材引用；羊皮纸背景、纸张噪点、面板描边、时间线卡片、绿色进度条、配置/认证/日志三栏、深色终端日志窗均由 CSS 实现。
- `InstallPage.tsx` 保留原有安装、Steam Guard、二维码弹窗、SSE 日志、权限判断和 API 调用逻辑，只调整 DOM 外壳：顶部状态横幅、五步安装时间线、底部三栏工作区。认证中左侧配置栏新增“配置已提交”占位，避免运行中空栏；日志栏在无任务时显示空状态，安装任务开始后继续渲染原实时日志。
- 顶部农场横幅复用既有小素材 `sprite_farmhouse_scene.png`，外层用 CSS 渐变、描边和遮罩融入纸张横幅；步骤图标使用 CSS 图形绘制，Steam/下载/星星等不新增截图素材。按钮继续复用现有 `sd-btn-green` / `sd-btn-tan` PNG 按钮体系。
- 响应式：桌面保持顶部状态 + 横向五步 + 三栏工作区；`760px` 以下步骤条改纵向、底部三栏改单列，表单字段和按钮纵向排列，390px 窄屏无横向溢出。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器真实路由因登录态停在登录页，使用已删除的临时 `install-qa.html` 挂载真实 `InstallPage` + 生产 CSS 做 QA。1280x900 认证态确认顶部状态、五步时间线、三栏、日志空状态可见且 console error/warn 为空；未安装态点击“安装游戏”后表单出现；390x760 无横向溢出。已用 `view_image` 对比原型图、桌面实现截图和移动实现截图。

# FE-OPSRAIL-AUTO-COLLAPSE-1 右栏按主内容压缩自动收起

- Stardew Shell 新增右侧 OpsRail 自动收起逻辑：不再只依赖 `max-width: 960px` 固定断点，而是按“右栏展开时主内容预计宽度”计算。展开态主内容低于 `820px` 时给 `.sd-shell` 加 `.sd-shell--opsrail-auto-collapsed`，右栏列宽归零并隐藏；收起后需回到 `880px` 以上才自动展开，避免窗口拖拽时反复抖动。
- `StardewPanel.tsx` 使用 `ResizeObserver + requestAnimationFrame` 监听 `.sd-shell` 宽度，只维护外层布局状态，不改路由、数据、API、权限或右栏内容逻辑。左栏/右栏宽度公式与 CSS grid 的 `clamp(210px,16.8vw,252px)`、`clamp(340px,27vw,430px)` 保持一致。
- `StardewPanel.css` 将左栏和右栏列宽抽成 `--sd-sidebar-width` / `--sd-opsrail-width`，新增 `.sd-shell--opsrail-auto-collapsed` 覆盖第三列和 `.sd-opsrail` 显示。
- 修复总览页被右栏挤压后的内部断点：`.sd-main-scroll` 增加 `container-type: inline-size`，总览页 1180px 响应式规则同步补为 `@container sd-main-scroll (max-width: 1180px)`，让控制区、邀请码区、摘要卡按主内容实际宽度换行，而不是只看浏览器视口宽度。
- 影响文件：`frontend/src/games/stardew/StardewPanel.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器真实路由仍停在登录页，因此使用已删除的临时 Vite QA HTML 挂载真实 `StardewPanel` 组件验证：1200x760 时 `.sd-shell--opsrail-auto-collapsed=true`、右栏 `display:none`、主内容宽 `959px`、无横向溢出；不刷新从 1200x760 resize 到 1600x860 后右栏自动展开、主内容宽 `887px`、无横向溢出；390x760 移动端仍为单列移动导航、右栏隐藏、无横向溢出；console error/warn 为空。

# FE-OVERVIEW-PROTOTYPE-IMAGE2-1 总览页按 image2 原型视觉重皮肤

- 总览页按 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/01-overview - 副本.png` 调整视觉层级，但未把该原型图作为运行时背景或整块素材引用：页面背景、控制条、摘要卡、三列清单、绿屏邀请码、纸纹噪点、边框、内阴影和分隔线均为 CSS 实现。
- 横幅：继续复用既有小农场场景素材 `sprite_farmhouse_scene.png`，外层用 CSS 叠加天空、云、远山、田地线条、暗角和像素边框，避免新增大面积截图素材。
- 控制区：`OverviewPage` 新增 `.sd-lifecycle-actions` 与 `.sd-invite-panel` 外壳，把生命周期按钮组和邀请码区排成原型式左右双区块；启动/停止/重启按钮继续复用现有 PNG 按钮底图和 `icon_button_*` 图标，刷新/复制继续走既有按钮组件与 handler。
- 摘要区：四张摘要卡新增标题图标和右下角状态标签，卡片用 CSS 羊皮纸噪点、深木描边、inset 高光和底部投影模拟原型纸卡；数据仍来自现有 `dashboardData.saves/mods/health/jobs`。
- 下方三列：在线玩家、近期事件、模组状态改为原型式标题栏 + 行分隔列表；在线玩家有数据时渲染行式头像首字母、名称和位置/角色信息，无数据、读取失败、服务器未运行等原状态文案保留。
- 响应式：1180px 以下控制条、摘要卡和三列改为单列/双列；640px 以下横幅压缩、按钮换行、邀请码绿屏可换行，显式避免横向溢出。
- 影响文件：`frontend/src/games/stardew/pages/OverviewPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器因真实应用停在登录页，使用已删除的临时 `overview-qa.html` 加载同一份 CSS/素材/DOM 做渲染 QA：1280x900 桌面无横向溢出、console error/warn 为空；390x760 窄屏无横向溢出、邀请码和按钮文字未溢出。已用 `view_image` 对比原型和实现截图，确认主要偏差仅为横幅场景使用既有小素材 + CSS 田野，而不是原型整图。

# FE-SAVES-SIMPLIFY-3 存档页页头精简 + 按钮统一素材 + 备份区紧凑化

- 页头：`SavesPage` 移除带框的 `.sd-page-header`（框 + 描述文案），改为左上角 `.sd-saves-page-title`（小图标 + "存档管理"纯文字，无框无描述），节省纵向空间。
- 按钮：撤销 `FE-SAVES-MOCKUP-2` 引入的自绘 `.sd-pxbtn` 糖块按钮体系（CSS 已删除），全部换回面板既有 PNG 素材按钮，与其他页面一致——选择/创建并启动/上传并启动/恢复 → `sd-btn-green`，删除 → `sd-btn-delete`，导出/手动备份/刷新 → `sd-btn-tan`。
- 备份与恢复紧凑化：自动备份规则从 5 列大网格 + 每项两行说明，压缩为单行控件条（"自动备份"标题 + 两个勾选 + 保留天数滑条 + 右侧"保存设置"按钮），详细解释移入各控件 `title` 悬浮提示；定时备份的小时下拉合并进勾选项（"每天 HH:00 定时备份"）；文案精简："保存备份设置"→"保存设置"、运行中提示 → "⚠ 运行中仅可查看，恢复需先停止服务器"、空状态 → "暂无备份。删除存档或覆盖恢复前会自动创建备份。"
- 整页上提：`.sd-saves-page { padding-top: 0 }` 去掉页面自身顶部 `--sd-page-padding`（28–42px），内容直接贴住 `.sd-main` 外框背景内沿（背景限制由 `.sd-main` 的 viewport inset 保证，未改动）。
- 影响文件：`frontend/src/games/stardew/pages/SavesPage.tsx`、`frontend/src/games/stardew/SavesSection.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`npm run build` 通过，源码中 `pxbtn` 无残留；手动确认页头无框、内容贴顶、按钮与其他页面同款、备份规则单行显示且保存生效。

# FE-SAVES-MOCKUP-2 存档页按完整设计稿改版

- 在 `FE-SAVES-PROTO-CSS-1` 骨架上按用户提供的完整设计稿重排存档页，新增视觉仍全部纯 CSS（糖块像素按钮、ZIP 折角纸片图标、状态徽章、字段行、加号块），农场图复用 `new-game/farms` 素材，图标用 emoji。
- 结构：眉标行（⭐ 当前激活存档 / 🍀 存档库 + 右侧刷新）替代旧页头；激活卡改为"地图缩略图 + 大标题 + 当前激活胶囊 + ⭐ + 图标字段双列表（农场主/最后游玩/日期/文件大小/农场类型/存档目录，细底线）"；存档库网格只展示**非激活**存档，每卡为"缩略图 + 农场名 + 进度行 + 类型·大小行 + 选择/删除糖块按钮"；创建卡改横排（虚线加号块 + 文案 + 创建并启动）；上传横条改为"📮 + 上传存档文件文案 + 上传并启动蓝色按钮"的容器；备份区改真表格（备份文件/所属农场/创建时间/大小/状态/操作六列列头带、行分隔线、默认显示 5 行 + "查看更多备份（N）"折叠）。
- 功能位移（逻辑本身未变）：每存档的"选择并启动/导出/手动备份"从库卡收敛——库卡只留"选择"（=设为启动存档）与"删除"，"使用此存档启动/导出/手动备份"集中在激活卡操作行（先选择再操作）；备份行状态徽章语义：`parseError` → 红"解析失败"、同名存档存在 → 黄"同名冲突"、否则绿"正常"，年份/地图等细节移入行 title 悬浮提示。
- 新按钮体系 `.sd-pxbtn`（`-green/-red/-blue/-lg/-sm`）：深色描边 + 厚底投影 + 顶部高光的糖块像素按钮，供本页与后续页面复用。
- 影响文件：`frontend/src/games/stardew/SavesSection.tsx`（SaveCard 重写、激活卡字段化、库过滤、备份表格与折叠 state）、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`npm run build` 通过；手动对照设计稿检查五块区域，确认选择/删除/恢复/彻底删除/创建/上传流程照常。

# FE-SAVES-PROTO-CSS-1 存档页按原型重构（纯 CSS，无图片资源）

- 按 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/03-saves-page-frame-clean-image2-no-buttons-icons-thumbnails.png` 重做存档页视觉，全部用 CSS 实现、不新增任何图片资源：羊皮纸纹理 = 两层低透明度 radial-gradient 噪点；激活卡四角铆钉 = 4 层 radial-gradient 圆点定位到四角；上传条像素云 = 多层白色矩形 background 层叠在蓝天渐变上；虚线创建卡 = `::before` inset 虚线框。
- 布局映射：激活存档卡 → 铜框铆钉相框，左侧预览槽为 CSS 深色羊皮纸块打底（移除 `sprite_farmhouse_scene.png` 引用），内嵌当前存档的农场地图像素图——按存档 `farmType` 匹配 `/assets/stardew/new-game/farms/<farmType>.png`（新建游戏界面同款素材，8 种农场全覆盖），`object-fit: contain` + `image-rendering: pixelated` 放大，farmType 未知时回落为空羊皮纸块；右侧 `sd-save-meta` 改双列虚线底线字段；存档卡网格 → 圆角铜边卡；网格末尾新增管理员"创建新存档"虚线卡（原页头"创建存档"按钮移入）；列表下方新增全宽天空横条按钮作为上传入口（原页头"上传存档"按钮移入，运行中禁用）；备份与恢复 → 圆角面板 + 深色表头带 + 行分隔线表格。
- 页头只保留"刷新列表"；空状态（无存档）里的创建/上传按钮保持不变，此时不渲染天空横条避免重复入口。
- 所有交互逻辑（选择/启动/导出/备份/删除/恢复/策略/弹窗）未改动，只动了 DOM 外壳与样式；新 CSS 全部以 `.sd-saves-page` 作用域追加在 `StardewPanel.css` 末尾覆盖旧皮肤。
- 影响文件：`frontend/src/games/stardew/SavesSection.tsx`（页头按钮精简、创建卡、上传横条）、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`npm run build` 通过；手动打开存档页对照原型确认铆钉、双列虚线、虚线创建卡、像素云上传条与备份表头带。

# FE-RIGHT-RAIL-ACTIVE-CARD-1 右栏进行中卡接入倒计时与任务进度

- 右栏"进行中"卡从纯 job 状态列表升级为：维护计划倒计时 + 定时备份倒计时 + 运行中任务进度条，行样式复用系统健康卡的 `.sd-opsrail-hstat*` 结构，新增蓝色 `--info` 档（浆果点/进度条蓝色渐变）用于倒计时行，任务进度行保持绿色。
- 倒计时数据源：`GET /api/instances/:id/restart-schedule`（普通用户可读）的 `nextShutdownAt`/`nextStartupAt` → "自动关机"/"自动开机"两行；`GET /api/instances/:id/saves/backups/policy`（仅管理员，403 时静默隐藏）的 `scheduledBackups + scheduledHour` → "定时备份"行，下次触发时间按面板本地时间的每日 `scheduledHour` 整点近似。倒计时格式 `HH:MM:SS`，进度条按 24h 周期已经过比例填充，按剩余时间升序排列。
- 任务进度估算（`runningJobPercent()`）：预期时长取 jobs 列表中同类型最近成功任务 `finishedAt - startedAt` 的中位数（`expectedJobDurationMs()`，无历史时默认 60s），进度 = 已运行时间/预期时长，封顶 95%；queued 任务显示"排队中"、进度 0。任务完成后由 SSE finished 事件刷新 jobs，行自动消失。
- 实现为独立组件 `OpsRailActiveCard`（`StardewPanel.tsx` 内），内部 1s tick 只重渲染本卡，不影响主内容区；配置每 60s 重新拉取。`useStardewDashboardData` 的 30s 轮询现在同时刷新 jobs，兜底调度器在后台触发的 job（SSE 只覆盖前端已知任务）。
- 影响文件：`frontend/src/games/stardew/StardewPanel.tsx`、`frontend/src/games/stardew/StardewPanel.css`、`frontend/src/games/stardew/useStardewDashboardData.ts`。
- 验证：`npm run build` 通过；手动开启计划重启/定时备份后看右栏倒计时每秒走动，触发任意任务（如备份）确认出现进度行并在完成后消失。

# FE-RIGHT-RAIL-HEALTH-STATS-1 右栏系统健康卡接入资源数据（原型样式）

- 右栏"系统健康"卡按原型改为资源统计行：CPU 使用率、内存使用率、磁盘使用率（各带像素风绿色进度条）、在线玩家、网络延迟，底部按钮文案从"查看诊断 →"改为原型的"查看详情 →"（仍跳诊断页）。
- 数据来源：CPU/内存/磁盘复用既有 `GET /api/instances/:id/metrics`（`getInstanceMetrics()`，与诊断页同一接口），`StardewPanel` 内部每 5s 轮询一次；在线玩家取 `dashboardData.players` 的 `onlineCount/maxPlayers`（后端 `ListPlayers` 现在会用当前存档 `server-settings.json` 的 `Server.MaxPlayers` 兜底 `maxPlayers`，见 `docs/02-backend.md` PLAYERS-MAXPLAYERS-1）；网络延迟无后端接口，取本次 metrics 请求的前端往返耗时（`performance.now()` 差值取整）。
- 容器未运行或请求失败时各值显示 `—`、进度条宽 0；原健康检查摘要行（全部正常 / N 错误 N 警告）从右栏卡移除，健康状态仍可在总览页摘要格与诊断页看到。
- 新增样式 `.sd-opsrail-hstat-list/-hstat/-hstat-row/-hstat-orb/-hstat-label/-hstat-value/-hstat-bar/-hstat-fill`（绿色浆果点 radial-gradient、标签左对齐、数值右对齐）；进度条按用户要求做成圆润二次元风：13px 高胶囊形轨道（`border-radius: 999px` + 2px 内边距）+ 糖果质感填充（亮绿渐变 + 顶部白色高光 inset）；删除已无引用的 `.sd-opsrail-health-summary` 与 `healthSummaryDot()`。
- 阈值配色：每行按数值加 `sd-opsrail-hstat--ok/--warn/--crit` 修饰类，浆果点、进度条填充和数值文字同步变色。使用率三行 `<60` 绿 / `≥60` 黄 / `≥85` 红（`usageLevel()`）；网络延迟 `<100ms` 绿 / `≥100` 黄 / `≥300` 红（`latencyLevel()`）；在线玩家为 `0` 时红，其余绿。绿色为默认样式，CSS 只覆盖黄/红两档。
- 影响文件：`frontend/src/games/stardew/StardewPanel.tsx`（metrics 轮询 state/effect、健康卡 JSX）、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`npm run build` 通过；手动在总览页确认健康卡五行数据与进度条随轮询更新，服务器停止时显示 `—`。

# FE-MAIN-PAGE-FRAME-3 中间内容视口按红框比例重定界

- 按用户最新红框示意，把所有 Stardew 页面共用的中间滚动视口从靠近外框的小 inset 调整为 frame 内侧的大矩形边界：上 `5.2%`、右 `5%`、下 `6%`、左 `4%`，并分别设置移动/窄宽下限与桌面上限。
- 结构保持 `FE-MAIN-PAGE-FRAME-2`：`.sd-main` 负责 image2 背景、红框比例边界和 `overflow:hidden` 裁切；`.sd-main-scroll` 负责在该边界内滚动；所有页面继续通过同一个 `StardewPanel` wrapper 生效。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器临时 QA 页使用 1750x1113 视口，使中间主内容区为 `1068x1033`，测得 `.sd-main-scroll` 相对 `.sd-main` 偏移为 top `55.5px`、right `53.4px`、bottom `64.1px`、left `42.7px`，比例分别为 `0.052/0.05/0.06/0.04`，与用户红框一致；滚轮后 `.sd-main-scroll.scrollTop=720`、`.sd-main.scrollTop=0`。390x760 下 inset 为 `22/20/26/18px`，滚轮后 `.sd-main-scroll.scrollTop=620`，无横向溢出，console error/warn 为空。

# FE-MAIN-PAGE-FRAME-2 中间内容滚动容器修复

- 修复 `FE-MAIN-PAGE-FRAME-1` 后续发现的模组页无法滚动回归：不再把每个路由自己的 `.sd-page` 强行改成滚动容器，而是在 `StardewPanel.tsx` 的 `.sd-main` 内新增统一包装层 `.sd-main-scroll`。
- 当前结构为：`.sd-main` 负责 image2 中间空框背景、`overflow: hidden` 裁切和内框 padding；`.sd-main-scroll` 负责 `overflow-y: auto`、`overflow-x: hidden`、隐藏原生滚动条和承接滚轮；各页面继续返回普通 `.sd-page`，避免模组页等复杂页面布局被滚动容器规则影响。
- 影响文件：`frontend/src/games/stardew/StardewPanel.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器临时 QA 页引用生产 CSS 和 public 素材验证 1280x720 下 `.sd-main-scroll.scrollTop` 经滚轮从 `0` 变为 `720`，390x760 下从 `0` 变为 `620`；两种视口均无横向溢出，`.sd-main` 保持 `overflow:hidden`，`.sd-main-scroll` 保持 `overflow-y:auto` 且 `scrollbar-width:none`，console error/warn 为空。

# FE-MAIN-PAGE-FRAME-1 中间内容页统一背景框

- 注意：本条中的 `.sd-main > .sd-page` 滚动容器方案已被上方 `FE-MAIN-PAGE-FRAME-2` 替代；当前滚动视口统一为 `.sd-main-scroll`，`.sd-main` 继续负责上一步界定的 frame 内侧边界和裁切。
- 将原型图 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/03-saves-page-frame-empty-image2.png` 复制为运行时素材 `frontend/public/assets/stardew/ui/panels/main_page_frame_empty_image2.png`，作为所有 Stardew 路由中间主内容区的统一背景。
- `stardew-theme.css` 新增 `--sd-img-page-frame` 资源变量，`.sd-main` 从旧羊皮纸 tile 平铺切换为该整张 frame：`background-repeat: no-repeat`、`background-position: center`、`background-size: 100% 100%`、`image-rendering: pixelated`。左侧栏、右侧栏、顶栏和各页面业务 DOM 不变。
- `--sd-page-padding` 从固定 `16px` 调整为 `clamp(28px, 2.4vw, 42px)`，避免页面标题和内容卡片压到 frame 的木质边框/角饰。
- 主内容滚动裁切改为“外层 frame 遮罩 + 内层页面滚动”：`.sd-main` 负责固定背景框、`overflow: hidden` 和内侧视窗 padding（桌面约 top/left `15/14px`，移动约 `12/10px`）；直接子节点 `.sd-main > .sd-page` 才是滚动容器，`overflow-y: auto`，并用 `scrollbar-width: none`、`-ms-overflow-style: none` 和 `::-webkit-scrollbar { display: none; }` 隐藏原生滚动条。内容超出时会在 frame 内侧边界被裁掉，滚动后才显示，不再压到木框/顶边上。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`、`frontend/src/games/stardew/stardew-theme.css`、新增 `frontend/public/assets/stardew/ui/panels/main_page_frame_empty_image2.png`。
- 验证：`cd frontend; npm.cmd run build` 通过；用生产构建 CSS + 临时 Shell QA 页在内置浏览器检查 1280x720 和 390x760，`.sd-main` 背景指向新 frame、尺寸为 `100% 100%`，`.sd-main` 为 `overflow:hidden`，`.sd-page` 为内侧滚动视窗且滚动条隐藏；桌面滚动后 `.sd-page.scrollTop` 到 `650`，顶部/底部内容被 frame 边界裁切，移动端无横向溢出，console error/warn 为空。

# FE-RIGHT-RAIL-CARD-FIX-1 右栏三卡去滚动 + 角部藤蔓等比修复

- 注意：本条替代 `FE-RIGHT-RAIL-PROTO-GEOMETRY-2` 中"三卡等高同步缩放"与 border-width 换算两点；该条其余内容（外壳映射、seamless 中段、切片值、background-clip 等）仍有效。
- 去滚动：`.sd-opsrail-stack` 从 `grid-template-rows: repeat(3, minmax(140px, 1fr))` + `overflow-y: auto` 改为 `grid-auto-rows: min-content` + `align-content: start` + `overflow: hidden`，三卡行高随内容、与左侧栏按钮一致只随栏宽缩放，不随窗口高度拉伸也不滚动；`.sd-opsrail-list` 同步去掉 `overflow: auto` 和滚动条隐藏规则，`.sd-opsrail-recent-list`（仅 `max-height: 100%`）删除并从 TSX 移除类名。
- 角部藤蔓拉伸根因：border-image 角部切片会被缩放进 `border-left-width × border-top-width` 的角盒；旧换算 `W = 13 × slice / (slice − margin)` 让每边可见框厚统一为 13px，但各边透明边距不同导致角盒横纵缩放比不一致（"进行中"卡横向 0.22 / 纵向 0.37，藤蔓被纵向拉长约 1.7 倍）。
- 修复：每张卡改用单一缩放系数 `s = 13 / (左切片 − 左透明边)`，各边 `border-width = slice × s`、负 `margin = 透明边 × s`：health `s≈0.203` → border `26 26 32 26` / margin `-11 -13 -10 -13`；active `s≈0.217` → border `38 33 48 33` / margin `-30 -20 -39 -20`；recent `s≈0.197` → border `38 35 36 35` / margin `-25 -22 -27 -22`。四边共用一个 s 后角部横纵等比，代价是上下可见框厚随素材原始比例变化（active 上/下约 8/9px），这是素材本身的比例。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`（`.sd-opsrail-stack`、`.sd-opsrail-list`、三张 `.sd-ops-card-*`）、`frontend/src/games/stardew/StardewPanel.tsx`（移除 `sd-opsrail-recent-list` 类名）。
- 验证：`npm run build` 通过；手动在总览页确认三卡不再滚动、随内容收缩、四角藤蔓不再变形。

# FE-RIGHT-RAIL-TOP-FROM-BOTTOM-1 右栏上段改用去装饰底段旋转版

- 基于现有 image2 底段素材 `right_rail_shell_bottom.png` 处理：先移除南瓜和向日葵及其遮挡区域，再用同图干净木梁像素和镜像左角饰重建右侧横梁/角饰，保留透明 alpha、木质横梁、角饰和藤蔓风格。
- 将清理后的底段旋转 180 度，覆盖当前运行时使用的上段素材 `right_rail_shell_top_line_image2.png`；原 `right_rail_shell_bottom.png` 保持不变，仍作为底段运行时素材。
- 新上段素材尺寸为 `1871x840`，RGBA alpha 范围 `0..255`，alpha bbox 为 `(59,0)-(1871,384)`；横梁实测范围为 `x123..1807/y146..291`。`.sd-opsrail::before` 已同步更新 top/left/width/aspect-ratio 常量，`.sd-opsrail-stack` 顶部 padding 改按新上段横梁和藤蔓深度预留。
- 影响文件：`frontend/public/assets/stardew/ui/panels/right_rail_shell_top_line_image2.png`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；Pillow 校验新 PNG 为 RGBA、尺寸 `1871x840`、alpha 范围 `0..255`；人工预览确认上段不再含南瓜/向日葵，横梁无透明破洞。

# FE-TOPBAR-SINGLE-SHELL-1 顶栏外壳改用整幅九宫格消除左中右割裂

- 顶栏三段拼接（`topbar_shell_left/middle_tile/right.png`）的中段金轨位置、粗细和木纹色调与左右端帽不一致，接缝处左中右割裂。`.sd-topbar-bg` 改为整幅 `topbar-shell.png`（2137x170，内容 bbox (8,6)-(2128,163)，内容高 158）的左右九宫格：`border-image-slice: 0 130 fill` + `border-image-repeat: stretch`，左右 130px 角饰带按条高等比渲染（`border-width: 0 calc(var(--sd-topbar-height) * 130 / 158)`），中段仅横向拉伸，从结构上保证无缝。
- `.sd-topbar-bg` 四边负偏移（-6/158、-8/158 × 条高）吃掉素材透明安全边，金框贴合顶栏边缘；`.sd-topbar-bg-left/mid/right` 三个子元素改为 `display: none`（DOM 保留未动）。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`（`.sd-topbar-bg` 块）。
- 验证：`npm run build` 通过；生产 CSS + 顶栏 DOM 隔离页无头 Edge 截图，2552px 与 1280px 宽视口下顶栏均为整体：四角雕花完整、上下金轨连贯、无接缝。

# FE-RIGHT-RAIL-PROTO-GEOMETRY-2 右侧栏几何精确对齐（边框到顶、消缝、三卡等高同步缩放）

- 注意：本条是右栏外壳/卡片几何的**最终状态**，替代下方 `FE-RIGHT-RAIL-PROTOTYPE-ALIGN-1` 与 `FE-RIGHT-RAIL-BLACK-EDGE-FIX-1` 中描述的封头裁剪与 121% overscan 方案（两条中的右栏列宽 `clamp(340px, 27vw, 430px)` 等改动仍有效）。
- 全部魔法数字（`--sd-opsrail-endcap-scale: 1.08`、`121%`、`-103` 偏移）替换为按素材实测内容包围盒推导的精确映射：顶封头 1842x854 内容 x58..1782/y104..468，底封头 1871x840 内容 x66..1808/下边距 149，`::before`/`::after` 按透明边距负偏移外扩，横梁顶边贴 `top:0`、木槽底边贴 `bottom:0`，立柱金色带三素材映射误差 ≤1px。
- 中段平铺改用新裁切素材 `right_rail_shell_middle_tile_seamless.png`（取原图 x130..1406/y27..1005；原图顶 27 行/底 18 行为纯黑，repeat-y 衔接处会形成约 14px 横向黑带横穿左右立柱，即"左右边框中段割裂"的根因；原素材保留未动）。
- 三张卡片九宫格切片按实测重调（原"进行中"顶部切片 142 但透明边 140、"近期任务"顶部切片 104 小于透明边 126，木框被切进中心拉伸区导致三框显示不一致）：health `126 126 156 126`、active `175 150 220 150`、recent `195 178 185 178`；每边 border-width 按可见框厚约 13px 换算（`W = 13 × slice / (slice − margin)`），负 margin 吃掉透明边距使三卡可见框与栅格单元对齐、视觉等宽等框厚。
- 卡片 `background-clip: padding-box`（负 margin 后 border-box 大于可见木框，背景会从边框图透明边距漏出形成暗色矩形"阴影遮罩"）、`overflow: hidden`、`border-image-repeat: stretch`（round 会在中心填充区产生拼接缝）。
- 三卡等高：`grid-template-rows: repeat(3, minmax(140px, 1fr))`，窗口缩放时三卡同步伸缩；stack 顶部避开横梁（128/1725）并留 `clamp(18px, 2.6vh, 28px)` 呼吸间距（太小时健康卡上框会顶进 z3 横梁底下被盖住，视觉割裂）、底部停在木槽上沿（143/1743）、左右对齐立柱内沿（92/1277）。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`、新增 `frontend/public/assets/stardew/ui/panels/right_rail_shell_middle_tile_seamless.png`。
- 验证：`npm run build` 通过；生产 CSS + 真实右栏 DOM/素材隔离页无头 Edge 截图，1280×940 与 1280×660 下横梁到顶、木槽到底、立柱连续无黑带、三卡等高同步缩放、无阴影遮罩、卡片内部无拼接缝，南瓜/向日葵压底卡右下角与原型一致。

# FE-TOPBAR-LEFT-CAP-SEAM-1 顶栏左段割裂修复

- 修复顶栏左段与中段拼接割裂：旧 `topbar_shell_left.png`（190x170）是旧版深色封闭边框风格，自带右侧描边，与 image2 风格的中段/右段颜色、金轨都对不上。
- 新 `topbar_shell_left.png` 由 `topbar_shell_right.png`（360x170）水平镜像生成，三段素材同源，接缝天然对齐；旧图备份后被覆盖（未入库过 git）。
- CSS 左列宽从 `calc(var(--sd-topbar-height) * 190 / 170)` 改为 `* 360 / 170`；640px 以下媒体查询左列 `134px` 改 `110px`（与右列一致，等于 52px 条高下的等比宽度，消除左段图与中段间的透明空档）。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`、`frontend/public/assets/stardew/ui/topbar/topbar_shell_left.png`。
- 验证：`cd frontend; npm.cmd run build` 通过；System.Drawing 逐行对比接缝像素，左段右缘 vs 中段左缘 170 行仅 1 行木纹噪点级差异，中段右缘 vs 右段左缘 0 行差异。

# FE-TOPBAR-IMAGE2-REGEN-1 顶栏 image2 重生拆分素材

- 按 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/01-overview.png` / `Top bar.png` 风格重新用 image2 生成顶栏拆分素材，替换上一批观感不合格的 topbar 资源；没有从原图按脚本坐标裁切，脚本仅用于生成图的 chroma-key 去底、尺寸归一化、预览和 alpha 校验。
- 顶栏外壳继续保持三段式：`topbar_shell_left.png`、`topbar_shell_middle_tile.png`、`topbar_shell_right.png`。运行时左/右端 `background-size: auto 100%`，中段 `repeat-x`，不再把整条带控件的顶栏做 `100% 100%` 横向拉伸。
- 控件改为独立资源：`topbar_status_button_9slice.png`、`topbar_save_frame_9slice.png`、`topbar_version_frame_9slice.png`、`topbar_user_frame_9slice.png`、`topbar_logout_button_9slice.png`，由 CSS `border-image` 渲染。农场、版本、用户、状态和登出文字仍由 React 前端渲染。
- 独立图标新增/切换为 v2：`icon_topbar_chicken_image2_v2.png`、`icon_topbar_farm_image2_v2.png`、`icon_topbar_user_avatar_image2_v2.png`、`icon_topbar_leaf_image2_v2.png`、`icon_topbar_green_dot_image2_v2.png`、`icon_topbar_logout_image2_v2.png`、`icon_topbar_dropdown_arrow_image2_v2.png`。
- 修复右端缺失：`topbar_shell_right.png` 重新用 image2 右端候选归一化到完整 `360x170` 高度，避免运行时只显示中间矮木条、右侧收口变成黑块。
- 影响文件：`frontend/src/games/stardew/StardewPanel.tsx`、`frontend/src/games/stardew/StardewPanel.css`、`frontend/public/assets/stardew/ui/topbar/`。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器临时 QA 页检查 1920x900 顶栏，确认右端指向 `topbar_shell_right.png`、尺寸 `auto 100%`、中段 `repeat-x`、控件使用新 `*_9slice.png` border-image、console error/warn 为空；390x760 下存档/版本/用户隐藏且无横向溢出。

# FE-RIGHT-RAIL-PROTOTYPE-ALIGN-1 右侧栏原型比例对齐

- 在三段式右栏外壳基础上继续对齐 `01-overview-right-sidebar-empty-image2.png` 原型：右栏桌面列宽改为 `clamp(340px, 27vw, 430px)`，整体比例更接近原型右栏；三张卡片保持独立 DOM 和九宫格框，但回到外壳内侧而不是压住左右木柱。
- 顶部/底部 shell 现在按素材有效区域裁剪：顶部固定段裁掉源图上方约 103px 透明安全边，使上边框贴到右栏顶部；底部固定段按可见装饰区域贴底；中段继续 `repeat-y` 且横向裁掉左右透明边，保证上下段与中段边框连续。
- `.sd-opsrail-stack` 的横向 padding 调整为 `clamp(18px, 1.8vw, 28px)`，三行高度调整为健康卡更高、进行中较矮、近期任务中等的比例；移除 `.sd-ops-card` 外投影，避免投影横穿左右木柱造成“边框断裂”。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。未修改 `StardewPanel.tsx`、后端接口或右栏动态内容逻辑。
- 验证：`cd frontend; npm.cmd run build` 通过；本地 QA 页面复用真实 CSS/素材检查 1280x900，确认顶部贴边、左右木柱不再被卡片阴影切断、三张卡片位于内框范围、stack 无额外滚动，console error/warn 为空。

# FE-RIGHT-RAIL-BLACK-EDGE-FIX-1 右侧栏两侧黑边修复

- 修复三段式 OpsRail 接入后右栏左右两侧露出黑底的问题：`right_rail_shell_middle_tile.png` 自身左右有透明/半透明暗边，按 `100%` 宽度平铺时会透出 `.sd-opsrail` 的近黑底色。
- `.sd-opsrail-bg` 的中段背景改为 `background-size: 121% auto` 并居中，让中段木板/立柱略微横向 overscan 后裁掉透明暗边；顶部/底部固定段用 `--sd-opsrail-endcap-scale: 1.08` 同步横向 overscan，并按放大后的宽度计算固定段高度和 stack 扣除高度，保持比例不压扁。
- `.sd-opsrail` / `.sd-opsrail-bg` 兜底色从近黑改成木板棕，避免极端透明像素处继续显黑。卡片、标题、图标、状态点、任务列表和按钮仍由 React/CSS 动态渲染，未改业务逻辑。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；本地 QA 页面复用真实 CSS/素材检查 1280x720 和 1280x560，确认中段为 `repeat-y`、背景尺寸为 `121%`、top/bottom 宽度为右栏 `108%`、矮窗口 stack 仍内部滚动、console error/warn 为空。

# FE-RIGHT-RAIL-3PIECE-RUNTIME-1 右侧栏三段外壳运行时接入

- `StardewPanel` 右侧 OpsRail 运行时已从旧 `panel_right_rail_shell_empty_image2.png` 整壳拉伸 + `panel_right_rail_outer_border_image2.png` 外框覆盖，迁移为新三段素材组合：`.sd-opsrail-bg` 使用 `right_rail_shell_middle_tile.png` 作为纵向 `repeat-y` 中段，`.sd-opsrail::before` 使用 `right_rail_shell_top.png` 固定顶部横梁/上边框/藤蔓角饰，`.sd-opsrail::after` 使用 `right_rail_shell_bottom.png` 固定底部木梁/南瓜/向日葵/藤蔓装饰。
- 中段背景只允许纵向重复，CSS 为 `background-repeat: repeat-y`、`background-size: 100% auto`，不再对任何整张右栏截图或带槽位/卡片/文字的图片做 `100% 100%` 拉伸。顶部和底部固定段高度按右栏容器宽度与素材原始比例计算，避免窗口高度变化时压扁或漂移。
- 三张 OpsRail 卡片继续作为独立 `.sd-ops-card` 渲染，并将 `border-image-source` 切到 `right_card_health_9slice.png`、`right_card_progress_9slice.png`、`right_card_recent_9slice.png`；标题、图标、健康状态、任务列表、按钮文案和状态点仍由 React/CSS 动态渲染。
- `.sd-opsrail-stack` 是三张卡片的垂直布局和滚动容器，滚动视口高度会扣掉底部固定装饰高度；矮窗口下优先让 stack 内部滚动，隐藏滚动条，避免滚动条出现导致卡片宽度左移。移动端 `<=960px` 继续隐藏右侧栏。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。未修改后端接口、React 数据来源、路由按钮逻辑或 `StardewPanel.tsx` 的动态内容结构。
- 验证：`cd frontend; npm.cmd run build` 通过；用本地 QA 页面复用真实 `StardewPanel.css` 和真实素材检查 1280x720、1280x900、1280x560、390x760，确认中段为 `repeat-y`、三张卡片为新 `right_card_*_9slice.png` `border-image`、top/bottom 固定段按比例渲染、1280x560 stack 内部滚动、390px 右栏 `display:none`，console error/warn 为空。

# FE-ASSET-RIGHT-RAIL-SHELL-3PIECE-1 右侧栏三段空壳与新卡片框素材

- 新增 6 张基于 image2 重新生成的右侧栏分层素材，位于 `frontend/public/assets/stardew/ui/panels/`：`right_rail_shell_top.png`、`right_rail_shell_middle_tile.png`、`right_rail_shell_bottom.png`、`right_card_health_9slice.png`、`right_card_progress_9slice.png`、`right_card_recent_9slice.png`。
- `right_rail_shell_top.png` 只保留右栏顶部横梁、上边框、藤蔓角饰和像素木质阴影；`right_rail_shell_middle_tile.png` 只保留左右木柱和中间纯木板背景，上下开口，供 `repeat-y`；`right_rail_shell_bottom.png` 只保留底部木梁、南瓜、向日葵和底部藤蔓装饰。
- 三张 `right_card_*_9slice.png` 是独立卡片框：只保留木质边框、角饰、藤蔓和空的深棕木纹内容底，便于后续九宫格或 `border-image` 使用。
- 这批素材没有烘焙心形/时钟/剪贴板图标、标题文字、CPU/内存/磁盘文字、进度条、任务列表、按钮文字或箭头；这些内容仍应由前端 React/CSS 数据层渲染。
- 本轮只新增生产素材，未改 `StardewPanel` 运行时引用；现有运行时仍使用已接入的 `panel_right_rail_*` 系列文件。
- 验证：使用 image2 生成到洋红 chroma-key 背景后本地转 RGBA 透明 PNG；Pillow 检查 6 张素材均为 `mode=RGBA`、alpha 范围 `0..255`、洋红残留 `0`；棋盘底人工预览确认无标题、图标、进度条、列表或按钮文字残影。尺寸分别为 top `1842x854`、middle `1536x1024`、bottom `1871x840`、health card `1053x1494`、progress card `1693x929`、recent card `1535x1025`。

# FE-SIDEBAR-BOTTOM-ART-CLIP-FIX-1 左侧栏底部装饰图割裂修复

- 修复窗口变矮时左侧栏最后一个导航按钮（设置）被 `.sd-nav-list` 下边界裁切、切口下露出底部装饰图导致的素材割裂：原 `--sd-sidebar-bottom-content-space` 的固定像素封顶（`clamp(84px, 12vh, 132px)`）小于 `panel_side_rail_bottom_image2.png` 的实际渲染高度（`100cqi * 409 / 598`），按钮列表会侵入底图区域。
- 关键陷阱：不能直接把 `.sd-sidebar` 自身 padding 改成 `var(--sd-sidebar-bottom-art-height)`——`container-type: inline-size` 声明在 `.sd-sidebar` 上，`cqi` 只相对**祖先**容器解析，在容器自身使用会回退成视口宽度（约 1300px），导致全部按钮被挤出（首次修复即因此翻车）。`::before`/`::after` 伪元素是后代，所以底图高度一直是对的。
- 最终实现：`.sd-sidebar` 的 padding-bottom 置 0，底部预留改放在 `.sd-nav-list` 的 `margin-bottom: var(--sd-sidebar-bottom-art-height)` 上（后代元素中 cqi 正确解析为侧栏宽度，与 `::after` 底图高度一致）；移动端媒体查询里 `.sd-sidebar .sd-nav-list` 补 `margin-bottom: 0`。
- 效果：按钮列表永远停在底部装饰图（PNG 整图）上沿，空间不足时列表滚动，裁切线与底图顶边重合。
- 曾尝试把预留空间减到 `calc(100cqi * 361 / 598)`（361 = 409 − 48，底图顶部 48px 为空白木板，让裁切线落到灯笼装饰上沿），用户确认后已回退到整图高度方案；如需重试该方向，48px 的像素扫描依据见最新前端接手文档。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`npm run build` 通过；用生产 CSS + 真实侧栏 DOM/素材的隔离页在无头 Edge 下截图，1280×900 全部 9 个按钮可见、设置完整；1280×560/360 滚动到底后设置按钮完整停在书架画上方、无割裂；同一环境复现旧 padding 方案确认按钮全部消失（证明验证方法有效）。

# FE-SIDEBAR-ROW-BG-1 左侧栏三段式与行背景接入

- `StardewPanel` 左侧栏运行时已从整张 `panel_side_rail_shell_empty_image2.png` 背景拉伸，切换为三段式背景组合：`.sd-sidebar` 用 `panel_side_rail_middle_tile_image2.png` 做纵向平铺底，`.sd-sidebar::before` 叠 `panel_side_rail_top_image2.png`，`.sd-sidebar::after` 叠 `panel_side_rail_bottom_image2.png`。
- 为解决“背景里一段一段槽位与按钮缩放后错位”的问题，导航 DOM 新增 `.sd-nav-list` 和每项外层 `.sd-nav-row`；`.sd-nav-row` 用轻微上下阴影提供行槽感，按钮底图、图标和中文 label 放在同一个行盒内渲染，槽位跟随按钮布局而不是烘焙在整张侧栏图里。
- 为避免 `.sd-nav-list` 出现滚动条时行容器宽度被压缩、导致行背景里的右边框左移，完整 `panel_side_rail_middle_tile_image2.png` 只保留在 `.sd-sidebar` 外层绘制；`.sd-nav-row` 不再引用中段素材，只保留轻微上下阴影来形成按钮背后的行槽感。
- 导航按钮宽度改用 `min(86cqi, 210px)`，以 `.sd-sidebar` 容器宽度为基准，不再使用相对滚动行容器的百分比；滚动条出现时按钮不会为了给滚动条让位而缩小。
- `.sd-nav-list` 保留 `overflow-y: auto` 但隐藏滚动条（`scrollbar-width: none` 和 `::-webkit-scrollbar`），避免滚动条占据可居中区域并把按钮整体推向左侧；需要溢出时仍可用滚轮/触控板滚动。
- 桌面端 9 个导航按钮的路由、`aria-current`、`aria-label`、hover、active、focus-visible 和原按钮底图不变；移动端继续覆盖为横向图标导航，`.sd-nav-list` 改为横排，`.sd-nav-row` 不显示行背景，避免新增包裹层影响 390px 宽度。
- 该方案是方案 B：保留“每个按钮背后有一段木板”的视觉，但把木板段迁移到按钮行容器中，避免侧栏整体高度变化时背景槽位和按钮位置分离。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器可打开 `http://localhost:5173/instances/stardew/overview` 登录页且无 console error/warn。当前本地浏览器未登录，尝试测试账号 `admin/admin-password` 返回用户名或密码错误，因此未完成真实登录态侧栏截图验证。

# FE-ASSET-SIDEBAR-3PIECE-1 左侧栏三段式背景素材

- 新增三张 image2 左侧栏三段式透明 PNG 生产素材：`frontend/public/assets/stardew/ui/panels/panel_side_rail_top_image2.png`、`panel_side_rail_middle_tile_image2.png`、`panel_side_rail_bottom_image2.png`。
- `top` 只保留左侧栏顶部木质外框、左右立柱、顶部横梁、深棕木纹、金棕像素边框、阴影和高光；不含导航按钮、中文文字、菜单图标或底部装饰。
- `middle_tile` 是 `598x96` 的纵向平铺段，只保留连续可平铺的深棕木板背景、左右木质立柱、边框阴影和细微木纹；不包含任何横向分隔线、按钮槽位、暗条、分层隔板或固定行高结构。首尾行已对齐，可用于 CSS `background-repeat: repeat-y` 或九宫格中段填充。
- `bottom` 只保留底部固定装饰区，包含置物架、灯笼、盆栽、紫色水晶、下层小物件、书本/盒子以及对应外框和阴影；不含导航按钮、中文文字或图标。
- 本次只新增生产素材，未改 `StardewPanel` 运行时代码。后续若接入响应式侧栏，应以 `top + repeat-y middle + bottom` 组合替代整张 `panel_side_rail_shell_empty_image2.png`，导航按钮、图标和 label 继续作为独立层渲染。

# FE-TOPBAR-SPLIT-ASSETS-1 顶栏拆分素材接入

- `StardewPanel` 顶栏已从整张 `panel_top_bar_image2.png` 背景迁移为拆分素材组合渲染：三段式 `topbar-shell-left.png` / `topbar-shell-middle.png` / `topbar-shell-right.png` 作为横栏空壳，品牌鸡、品牌文字发光占位、农场图标、下拉箭头、版本框、用户头像、用户框三段式和登出按钮底图都放在 `frontend/public/assets/stardew/ui/topbar/` 下。
- 顶栏文字、点击逻辑和数据来源保持前端动态渲染：状态点击进服务器页，存档点击进存档页，版本和用户点击进设置页，登出继续调用 `onLogout`；版本号继续使用 `versionInfo.version`，用户身份继续使用当前 `user.role`，存档名优先使用 `activeSave.farmName`，否则回退存档名/选择存档。
- 状态区域不再使用 running/stopped 状态图片，改为木质状态框 + 现有 `.sd-dot` / `.sd-dot-pulse` 动态状态点和文本；running 使用绿色 pulse，starting/stopping/loading 使用黄色 pulse，stopped/error/ready/save-required 使用红色或现有状态色语义。
- 移动端沿用简化策略：隐藏存档、版本、用户区域，只保留品牌图标、状态和登出按钮；390×760 下无横向滚动或按钮重叠。
- 验证：`cd frontend; npm.cmd run build` 通过；浏览器检查 `/instances/stardew/overview`、`/server`、`/saves`、`/settings`，顶栏素材显示正常、状态点为动态 dot、状态/存档/版本/用户点击跳转保持原逻辑；实际点击登出后回到登录表单。长存档名样式为 `text-overflow: ellipsis`，desktop 与 390×760 mobile 均无 console error/warn。

# FE-RIGHT-RAIL-SPLIT-ASSETS-1 右侧 OpsRail 拆分素材接入

- `StardewPanel` 右侧 OpsRail 已从整张 `panel_right_rail_image2.png` 背景/透明热区方案，迁移为拆分素材组合渲染：`.sd-opsrail-bg` 使用 `panel_right_rail_shell_empty_image2.png` 作为木质背景空壳，`.sd-opsrail::after` 使用 `panel_right_rail_outer_border_image2.png` 作为外框覆盖层，三个 `.sd-ops-card` 分别用 `panel_right_rail_card_*_nineslice_image2.png` 做 `border-image` 九宫格卡片框。
- 系统健康、进行中、近期任务标题由 React 渲染中文文本，并分别叠加 `icon_right_rail_health_heart_image2.png`、`icon_right_rail_in_progress_clock_image2.png`、`icon_right_rail_recent_tasks_clipboard_image2.png`；不再使用右栏整图里烘焙的标题、图标、列表或按钮文字。
- 原有数据和交互逻辑保留：健康摘要仍来自 `dashboardData.health`，任务列表仍由 `jobs` 派生，`JOB_STATUS_DOT` 和 `healthSummaryDot()` 继续复用 `.sd-dot*` CSS 状态点；“查看诊断”跳 `diagnostics`，“查看全部任务”跳 `jobs`，Mod 重启提示收进近期任务卡片底部并跳 `mods`。
- 移动端 `<=960px` 继续隐藏右栏；右栏源码中不再引用 `panel_right_rail_image2.png` 作为运行时背景。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器检查 `http://127.0.0.1:5173/instances/stardew/overview`，1280×720 下右栏背景空壳、外框、三张卡片和三个标题图标均可见，控制台无 error/warn；点击“查看诊断”到 `/instances/stardew/diagnostics`，点击“查看全部任务”到 `/instances/stardew/jobs`；390×760 下 `.sd-opsrail` 为 `display:none`，无水平溢出。

# FE-SIDEBAR-SPLIT-ASSETS-1 左侧栏拆分素材接入

- `StardewPanel` 左侧栏已从整张 `panel_side_rail_image2.png` / `Left panel.png` 透明热区方案，迁移为可复用拆分素材组合：`panel_side_rail_shell_empty_image2.png` 作为唯一侧栏背景并填满侧栏格子，`nav_item_default_wood_blank_image2.png` / `nav_item_hover_wood_blank_image2.png` / `nav_item_active_wood_blank_image2.png` 分别作为按钮 default / hover / active 底图。9 个 `icon_nav_*_image2.png` 作为独立导航图标，中文菜单文字由 React `span.sd-nav-label` 渲染。
- `stardew-theme.css` 保留旧主题导航规则，但对桌面 `.sd-sidebar .sd-nav-item` 增加限定覆盖，避免全局 `.sd-nav-item:hover` 的 `background` 简写冲掉拆分素材背景；未选中 hover 使用 hover 底图，active 与 active:hover 使用 active 底图。
- 左侧栏桌面端继续渲染 9 个 `button`，保留 `navigate(entry.route)`、`aria-current`、`aria-label`、`title`、hover、active 和键盘 focus-visible；不再依赖整图里烘焙的菜单文字或图标。
- 侧栏四周用 CSS 像素边框补强，避免空壳按宽度适配时边缘发虚；底部不再叠加 `sidebar_bottom_decor_props_group_image2.png`，避免与空壳底部残留装饰重复。
- 移动端继续使用横向图标导航，隐藏 label，保留 active 金色像素边框；不使用整张左栏背景，390×760 视口下无页面或导航横向溢出。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器检查 `http://127.0.0.1:5173/instances/stardew/overview`，左侧 9 个菜单可见，“任务日志”完整显示，点击“服务器”跳转 `/instances/stardew/server`，点击“诊断”跳转 `/instances/stardew/diagnostics`，desktop 1280×720 与 mobile 390×760 均无 console error/warn。

# FE-SHELL-IMAGE2-1 顶栏与侧栏 image2 替换

- `StardewPanel` Shell 已把顶栏替换为 `Top bar.png` 生产素材，左侧导航替换为 `Left panel.png`，右侧任务栏替换为 `01-overview-right-sidebar-empty-image2.png`；生产文件位于 `frontend/public/assets/stardew/ui/panels/`，页面不直接依赖 `docs/prototypes`。
- 顶栏保留现有逻辑：状态徽章点击进入服务器页，按 `instanceState.state` 切换 `03-saves-status-running-transparent-image2.png` / `03-saves-status-stopped-transparent-image2.png`；农场槽优先显示当前激活存档的 `farmName`，无解析值时回退存档目录；版本槽显示当前面板版本；角色槽显示 `管理员` / `普通用户`；登出槽继续调用原 `onLogout`。
- 左侧栏不再叠加旧文字和图标到桌面原型图上，而是用透明热区覆盖九个菜单位置，保留路由跳转、active 高亮、hover 和键盘焦点；移动端仍回退为横向图标导航，避免大图侧栏挤压小屏。
- 右侧 OpsRail 保留原有系统健康、进行中任务、近期任务和 Mod 重启提示逻辑，内容定位到右栏专用素材的“系统健康 / 进行中 / 近期任务”框内；“查看详情”区域继续跳转诊断页。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器登录态检查 `overview -> server` 点击热区切换成功，右侧栏“查看详情”透明入口可跳转诊断页，桌面 1280×720 与移动 390×760 均无 console error/warn。

# FE-PROTOTYPE-LAYOUT-1 原型信息架构重排

- Stardew 路由页按 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30` 的页面布局重新排布信息层级，但不复刻原型静态内容：现有 API 数据和操作能力保留，按原型中相同功能的位置组织展示。
- `OverviewPage` 改为农场横幅、服务器控制/邀请码、四个摘要指标、在线玩家/近期事件/模组状态三列摘要的结构。
- `ServerControlPage` 增加页面级布局分区，靠近原型的“状态卡 + 生命周期控制 + 邀请码/全服消息 + 控制命令 + 快捷操作”顺序。
- `SavesSection` 新增当前激活存档重点卡，存档库、创建/上传入口、备份与恢复继续保留；移动端和窄主栏下按钮组改为左对齐/换行，避免被滚动条截断。
- `JobsLogsPage`、`PlayersPage`、`ModsPage`、`DiagnosticsPage`、`SettingsPage` 增加页面级 class，并通过 CSS 调整为原型式的列表/详情、概览卡、双栏检查/资源趋势、分区卡片布局。`ModsPage` 仍保留当前三段式“下载模组 / 添加模组 / 配置模组”工作台，不回退为旧单页卡片流。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器登录后检查 `overview/server/saves/jobs/players/mods/diagnostics/settings` 无 console error/warn；390px 移动宽度检查 `overview/saves/jobs` 单列布局。

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

# FE-ASSET-LEFT-RAIL-SHELL-1 左侧栏空壳素材

- 新增 `frontend/public/assets/stardew/ui/panels/panel_side_rail_shell_empty_image2.png`，基于 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/Left panel.png` 生成左侧栏木质背景空壳素材。
- 素材保留原图的外侧竖向木梁、深色木纹、横向分隔阴影、底部置物架和装饰区；移除九个导航按钮、菜单文字、菜单图标、按钮金边和高亮残影。
- 输出为 RGBA 透明 PNG，尺寸 `598x1807`，比原图四周多 4px 透明安全边距；适合后续在前端用 CSS 叠加独立按钮、图标和文字。
- 本次只新增生产素材，未改 `StardewPanel` 引用；现有左侧栏仍使用 `panel_side_rail_image2.png`，后续切换时应同步调整定位尺寸和热区坐标。
- 验证：Pillow 检查 alpha 通道、四角透明、导航区无亮色文字/图标残留；人工预览确认旧按钮轮廓已清理。

# FE-ASSET-NAV-BUTTON-DEFAULT-1 默认导航按钮空底图

- 新增 `frontend/public/assets/stardew/ui/navigation/nav_item_default_wood_blank_image2.png`，基于 image2 `Left panel.png` 中默认态导航按钮提取并重绘。
- 素材只包含一个横向木质导航按钮本体，保留金棕色边框、四角像素装饰、内侧阴影、高光和暗部；移除中文菜单文字、图标和侧栏背景木板。
- 输出为 RGBA 透明 PNG，尺寸 `442x138`，四周保留 4px 透明安全边距，中心木纹区域为空，供前端继续叠加独立图标和文字。
- 本次只新增生产素材，未改 `StardewPanel` 引用；后续若接入，应与独立导航图标素材和按钮文字层组合使用。
- 验证：Pillow 检查 alpha 通道、四角透明、中心区域无中文文字/图标残留；人工预览确认按钮外侧没有整张侧栏背景。

# FE-ASSET-NAV-BUTTON-ACTIVE-1 激活导航按钮空底图

- 新增 `frontend/public/assets/stardew/ui/navigation/nav_item_active_wood_blank_image2.png`，基于 image2 `Left panel.png` 风格生成并抠图生产左侧导航激活态按钮底图。
- 素材只包含一个横向木质导航按钮本体，形状跟默认按钮同源，选中态使用更亮的金色双边框、角饰高光和轻微暖色发光；中央木纹区域为空，不含中文文字或图标。
- 输出为 RGBA 透明 PNG，尺寸 `442x153`，四周保留 4px 透明安全边距；宽度对齐默认态素材，方便后续 CSS 分层替换。
- 本次只新增生产素材，未改 `StardewPanel` 引用；后续接入时应与默认态按钮、独立导航图标和文字层一起调坐标。
- 验证：Pillow 检查 alpha 通道、四角透明、无绿幕背景残留；人工预览确认中心留空且按钮保持像素风激活态。

# FE-ASSET-NAV-BUTTON-HOVER-1 悬停导航按钮空底图

- 新增 `frontend/public/assets/stardew/ui/navigation/nav_item_hover_wood_blank_image2.png`，基于默认态按钮与已选 C 版激活态按钮派生左侧导航 hover 状态空底图。
- 素材只包含一个横向木质导航按钮本体；整体亮度介于默认态和激活态之间，保留木质主体、像素角饰、内侧阴影，并加入克制的金色边缘高光。
- 输出为 RGBA 透明 PNG，尺寸 `442x138`，与默认态素材完全一致，四角透明且保留像素阴影和安全边距；中央木纹区域为空，不含中文文字、图标或侧栏背景。
- 本次只新增生产素材，未改 `StardewPanel` 引用；后续接入 hover 时可直接与默认态同尺寸替换，active 态因外发光高度更高仍需按中心线对齐。
- 验证：Pillow 检查 `mode=RGBA`、尺寸 `442x138`、四角 alpha 为 0、alpha 范围 `0..255`；人工预览确认无文字/图标残留，状态强度弱于 active。

# FE-ASSET-SIDEBAR-DECOR-PROPS-1 左侧栏底部装饰素材

- 新增并已重生成 4 个 image2 左侧栏底部装饰透明素材：`frontend/public/assets/stardew/ui/sprites/sidebar_bottom_decor_props_group_image2.png`、`sidebar_decor_lantern_glow_image2.png`、`sidebar_decor_potted_plant_image2.png`、`sidebar_decor_purple_crystal_image2.png`。
- 最新版本直接以 `Left panel.png` 底部装饰区为 image2 参考生成，再用洋红 chroma-key 本地转透明，替换掉首版本地抠图/补边素材。
- 整组素材保留原图底部装饰物的相对位置，包括上层发光灯笼、盆栽、紫色水晶、下层小壶、竖书和右侧书本/盒子，并只保留与装饰一体的木架结构；不包含导航按钮、菜单文字或整张侧栏木板。
- 单件素材分别只保留灯笼本体与暖色像素光晕、盆栽花盆与绿色叶片、水晶簇与底座阴影；单件不带侧栏背景或其它物件。
- 输出均为 RGBA 透明 PNG，尺寸分别为：整组 `720x558`、灯笼 `357x484`、盆栽 `490x531`、紫水晶 `454x541`；四角透明，保留透明安全边距。
- 本次只更新生产素材，未改 `StardewPanel` 引用；后续接入时建议把整组作为左侧栏底部叠层，单件可作为独立装饰图标复用。
- 验证：Pillow 检查 4 个文件 `mode=RGBA`、alpha 范围 `0..255`、四角 alpha 为 0、洋红残留为 0；人工预览确认无菜单文字/图标残留。

# FE-ASSET-NAV-ICONS-IMAGE2-1 左侧导航图标素材

- 新增 image2 左侧导航 9 枚透明图标与 3x3 sprite sheet：`frontend/public/assets/stardew/ui/icons/icon_nav_sprite_sheet_3x3_image2.png`。
- 单图文件包括：`icon_nav_overview_map_image2.png`、`icon_nav_server_rack_image2.png`、`icon_nav_saves_chest_image2.png`、`icon_nav_tasks_scroll_image2.png`、`icon_nav_players_avatar_image2.png`、`icon_nav_mods_crystal_image2.png`、`icon_nav_diagnostics_monitor_image2.png`、`icon_nav_install_package_image2.png`、`icon_nav_settings_gear_image2.png`。
- 图标参考 `Left panel.png` 的九个导航语义和造型重绘：地图、服务器机柜、宝箱、卷轴日志、玩家头像、绿色晶体、绿色监视器心电图、纸箱包裹、齿轮；均不含按钮底图、菜单文字或侧栏木板。
- 单图均为 RGBA 透明 PNG，并按图标主体紧裁保留透明边距；sprite sheet 为 `1254x1254`，3x3 排列，每格约 `418x418`，图标之间保留大面积透明间距且无文字标签。
- 本次只新增生产素材，未改 `StardewPanel` 引用；后续接入时可用单图逐个定位，也可按 sheet 的 `96px` cell 与 `16px` gap 做 CSS sprite。
- 验证：Pillow 检查 10 个文件 `mode=RGBA`、alpha 范围 `0..255`、四角 alpha 为 0；人工预览确认无按钮木框、中文文字或背景木板残留。

# FE-ASSET-RIGHT-RAIL-SHELL-1 右侧栏空壳素材

- 新增 `frontend/public/assets/stardew/ui/panels/panel_right_rail_shell_empty_image2.png`，基于 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/01-overview-right-sidebar-empty-image2.png` 的右侧栏风格重绘。
- 素材只保留外层木质立柱、完整顶部横梁、深棕木质内底、金棕边框、藤蔓、底部木质基座和南瓜/向日葵装饰；移除三个内部内容卡片、标题文字、图标、状态点、进度条和任务内容。
- 输出为 RGBA 透明 PNG，尺寸 `826x1903`，内部是干净连续的深棕木纹区域，适合后续用 CSS 叠加独立卡片框、标题图标、进度条和装饰层。
- 该素材已在 `FE-RIGHT-RAIL-SPLIT-ASSETS-1` 中接入运行时，作为右侧栏背景空壳层使用。
- 验证：Pillow 检查 `mode=RGBA`、尺寸 `826x1903`、四角 alpha 为 0、alpha 范围 `0..255`、无洋红底色残留；顶部横梁缺口区域已确认整段可见。

# FE-ASSET-RIGHT-RAIL-BORDER-1 右侧栏外层边框素材

- 新增 `frontend/public/assets/stardew/ui/panels/panel_right_rail_outer_border_image2.png`，基于 `01-overview-right-sidebar-empty-image2.png` 的右侧栏风格生成独立外层木质边框。
- 素材只保留最外侧左右竖梁、顶部边缘、底部边缘、像素阴影、金棕木质雕刻和外框藤蔓点缀；中间区域完全透明。
- 已移除内部卡片框、内部卡片角落藤蔓、文字、图标、状态点、进度条、列表内容以及底部南瓜/向日葵装饰，避免和后续卡片层、数据层、装饰层混用。
- 输出为 RGBA 透明 PNG，尺寸 `920x1710`；适合作为 CSS 最上层覆盖边框或背景层外框。
- 验证：Pillow 检查 `mode=RGBA`、尺寸 `920x1710`、四角 alpha 为 0、中心/上中/下中采样 alpha 为 0、中心区域批量采样最大 alpha 为 0、无洋红底色残留；人工预览确认没有内部内容残影。

# FE-ASSET-RIGHT-RAIL-CARDS-1 右侧栏三卡片空框素材

- 新增右侧栏三张可复用卡片空框：`panel_right_rail_card_health_empty_image2.png`、`panel_right_rail_card_in_progress_empty_image2.png`、`panel_right_rail_card_recent_tasks_empty_image2.png`。
- 三张素材分别对应原型里的顶部“系统健康”大卡、中部“进行中”卡和底部“近期任务”卡，只保留木质边框、深棕内容底、金棕角饰、藤蔓点缀和像素阴影。
- 已移除标题文字、红心/时钟/任务板图标、CPU/内存/磁盘/在线玩家/网络延迟文字、绿色状态点、进度条、“查看详情”文字和箭头、内部横线、任务列表和其它动态内容。
- 输出均为 RGBA 透明 PNG，尺寸分别为健康卡 `1088x1446`、进行中卡 `1604x981`、近期任务卡 `1464x1075`；卡片外部透明，卡片内部保留干净深棕木纹/皮革质感，供前端叠加标题、指标、按钮和列表。
- 该组固定尺寸空框目前保留为备用；运行时优先使用 `*_nineslice_image2.png` 九宫格卡片框，与右侧栏空壳、外层边框、标题图标和数据层分开定位。
- 验证：Pillow 检查三张素材 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`、中心 alpha 为 255、无洋红底色残留；人工预览确认无文字/图标/进度条/列表残影。

# FE-ASSET-RIGHT-RAIL-CARDS-NINESLICE-1 右侧栏九宫格卡片框素材

- 新增右侧栏三张九宫格友好的卡片框素材：`panel_right_rail_card_health_nineslice_image2.png`、`panel_right_rail_card_in_progress_nineslice_image2.png`、`panel_right_rail_card_recent_tasks_nineslice_image2.png`。
- 三张素材分别对应顶部系统健康大卡、中部进行中卡和底部近期任务卡；四角像素装饰完整，角落藤蔓集中在不可平铺角区，上下边框和左右边框保留较长直线重复段，便于 `border-image` 或九宫格裁切。
- 中间内容区保留干净深棕木纹/皮革纹理，不含文字、图标、进度条、状态点、内部横线、列表或参考线；素材外部为透明背景并保留安全边距。
- 输出均为 RGBA 透明 PNG，尺寸分别为健康卡 `1403x1121`、进行中卡 `1693x929`、近期任务卡 `1534x1025`；透明边距分别约为 `104/93/104/131`、`100/119/99/134`、`62/67/62/59`（左/上/右/下）。
- 该组已在 `FE-RIGHT-RAIL-SPLIT-ASSETS-1` 中通过 CSS `border-image` 接入运行时，作为三个可变尺寸右栏卡片框。
- 验证：Pillow 检查三张素材 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`、中心 alpha 为 255、无洋红底色残留；人工预览确认无文字/图标/进度条/列表残影，边框中段规则。

# FE-ASSET-RIGHT-RAIL-TITLE-ICONS-1 右侧栏标题图标素材

- 新增右侧栏三枚标题图标：`icon_right_rail_health_heart_image2.png`、`icon_right_rail_in_progress_clock_image2.png`、`icon_right_rail_recent_tasks_clipboard_image2.png`。
- 三枚素材基于 image2 右侧栏原图风格重绘，分别对应系统健康红心、进行中蓝色时钟和近期任务剪贴板；只保留图标本体、像素描边、阴影和高光。
- 已移除所有中文文字、卡片框背景、右侧栏背景、进度条、状态点和列表内容；适合前端作为右侧栏卡片标题图标独立叠加。
- 输出均为 RGBA 透明 PNG，四周固定 4px 透明安全边距；尺寸分别为红心 `776x680`、蓝色时钟 `864x940`、剪贴板 `714x934`。
- 该组三枚图标已在 `FE-RIGHT-RAIL-SPLIT-ASSETS-1` 中接入运行时，标题文字仍由 React 渲染。
- 验证：Pillow 检查三枚图标 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`、内容 bbox 四边距均为 4px、无洋红底色残留；人工预览确认无文字或卡片背景残影。

# FE-ASSET-TOP-BAR-SHELL-1 顶栏空壳素材

- 新增 `frontend/public/assets/stardew/ui/panels/panel_top_bar_shell_empty_image2.png`，基于 image2 `Top bar.png` 的顶栏风格生成可复用木质背景空壳素材。
- 素材只保留整条深棕木纹顶栏、上下金棕像素边框、四角装饰、整体阴影和像素高光；已移除左侧鸡图标、`Stardew Anxi Panel` 品牌字、状态徽章、农场选择框、版本框、用户角色框、登出按钮以及所有槽位图标/文字。
- 输出为 RGBA 透明 PNG，尺寸 `2137x170`，其中原始顶栏主体按 `2129x162` 对齐，四周保留 4px 透明安全边距；内部木纹为干净连续底板，适合后续叠加品牌层、按钮层、图标层、文本层和状态层。
- 本次只新增生产素材，未改 `StardewPanel` 引用；当前顶栏仍使用 `panel_top_bar_image2.png`，后续切换时应按新增安全边距修正定位和热区坐标。
- 验证：Pillow 检查 `mode=RGBA`、尺寸 `2137x170`、四角 alpha 为 0、alpha 范围 `0..255`、无绿幕/白底残留；人工预览确认无文字、按钮、图标或状态残影。

# FE-ASSET-TOP-BAR-CORNERS-1 顶栏四角装饰素材

- 新增 4 个 image2 顶栏角标透明素材：`topbar_corner_top_left_image2.png`、`topbar_corner_top_right_image2.png`、`topbar_corner_bottom_left_image2.png`、`topbar_corner_bottom_right_image2.png`。
- 新增 2x2 无标签 sprite sheet：`frontend/public/assets/stardew/ui/sprites/topbar_corner_ornaments_sprite_sheet_2x2_image2.png`，顺序为左上、右上、左下、右下，四格之间保留透明间距。
- 素材基于 `Top bar.png` / 顶栏空壳风格重绘，只保留金棕木质/金属像素角标、暗色像素阴影和高光；不包含整条顶栏背景、木纹底板、文字、图标、按钮、徽章或下拉槽位。
- 单件输出为 RGBA 透明 PNG，尺寸分别为左上/右上 `104x88`、左下/右下 `104x82`；sprite sheet 尺寸 `224x192`。
- 本次只新增生产素材，未改 `StardewPanel` 引用；后续接入时可作为顶栏空壳或九宫格边框的角标层使用。
- 验证：Pillow 检查 5 个文件 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`、无绿幕/白底残留；人工预览确认 sheet 无标签、无文字/按钮/图标残影。

# FE-ASSET-TOP-BAR-CHICKEN-1 顶栏鸡图标素材

- 新增 `frontend/public/assets/stardew/ui/icons/icon_topbar_chicken_image2.png`，基于 image2 `Top bar.png` 左侧品牌区鸡图标风格重绘。
- 素材只保留白色鸡图标本体，包含白/奶油色羽毛、红色鸡冠、黄色喙、橙色脚、暗色像素描边、像素阴影和高光；不包含 `Stardew Anxi Panel` 文字、顶栏木质背景、按钮、徽章或其它 UI 元素。
- 输出为 RGBA 透明 PNG，尺寸 `92x104`，主体四周保留 4px 透明安全边距；适合作为前端品牌图标单独叠加到顶栏。
- 本次只新增生产素材，未改 `StardewPanel` 引用；当前顶栏仍使用整图 `panel_top_bar_image2.png`。
- 验证：Pillow 检查 `mode=RGBA`、尺寸 `92x104`、四角 alpha 为 0、alpha 范围 `0..255`、无绿幕/白底残留；人工预览确认无文字和木质背景。

# FE-ASSET-TOP-BAR-BRAND-GLOW-1 顶栏品牌文字发光占位素材

- 新增 `frontend/public/assets/stardew/ui/sprites/topbar_brand_text_glow_placeholder_image2.png`，基于 image2 `Top bar.png` 左侧品牌文字区域生成轻量暖黄色像素发光/阴影占位层。
- 素材不包含实际文字、不包含鸡图标、不包含木质顶栏背景；仅保留非字形的浅色像素光带和底部暖色阴影，供前端渲染 `Stardew Anxi Panel` 文本时叠放在文字下方。
- 输出为 RGBA 透明 PNG，尺寸 `468x78`，alpha 范围 `0..18`，主体 bbox 为 `(12, 27, 457, 66)`；适合作为品牌文字底层装饰，文本仍必须由前端动态渲染。
- 本次只新增生产素材，未改 `StardewPanel` 引用；如果后续字体描边方案足够接近原图，也可以不启用该占位层。
- 验证：Pillow 检查 `mode=RGBA`、四角 alpha 为 0、无绿幕/白底残留；人工预览确认没有任何可读字形或鸡图标残影。

# FE-ASSET-FARM-SELECT-FRAME-1 顶栏农场选择框空底图

- 新增 `frontend/public/assets/stardew/ui/fields/field_topbar_farm_select_empty_image2.png`，基于 image2 `Top bar.png` 的农场选择框提取并重绘空底图。
- 素材只保留金棕像素边框、暗棕木纹内容底、内侧像素阴影和下拉框外形；已移除农场图标、农场名文字、右侧下拉箭头和顶栏背景。
- 输出为 RGBA 透明 PNG，尺寸 `456x132`，主体 bbox 为 `(28, 8, 437, 121)`，四角透明；内部内容区为空木纹，方便前端叠加农场图标、农场名和箭头。
- 本次只新增生产素材，未改 `StardewPanel` 引用；固定宽度场景可直接使用该空底图，可变宽度场景优先使用三段式素材。
- 验证：Pillow 检查 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`；人工预览确认无农场图标、文字、箭头和顶栏背景残影。

# FE-ASSET-FARM-SELECT-3PIECE-1 顶栏农场选择框三段式素材

- 新增农场选择框三段式透明 PNG：`field_topbar_farm_select_left_cap_image2.png`、`field_topbar_farm_select_center_tile_image2.png`、`field_topbar_farm_select_right_cap_image2.png`。
- 新增无标签横向 sprite sheet：`frontend/public/assets/stardew/ui/fields/field_topbar_farm_select_3piece_sheet_image2.png`，顺序为左端、中段、右端，段与段之间保留 16px 透明间距。
- 左/右端保留原图金棕角部边框和像素阴影；中段为可横向平铺的暗棕木纹内容区和上下金色边框，不包含农场图标、农场名文字或下拉箭头。
- 单件尺寸分别为左端 `96x132`、中段 `64x132`、右端 `96x132`；sprite sheet 尺寸 `288x132`。本次只新增生产素材，未改 `StardewPanel` 引用。
- 验证：Pillow 检查 4 个文件 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`；人工预览确认 sheet 无标签、三段无文字/图标/箭头残影。

# FE-ASSET-DROPDOWN-ARROW-1 顶栏下拉箭头图标

- 新增 `frontend/public/assets/stardew/ui/icons/icon_dropdown_arrow_gold_image2.png`，基于 image2 `Top bar.png` 中农场选择框/用户框的下拉箭头风格重绘。
- 素材只保留浅金/黄色像素下拉箭头、暗色描边和轻微阴影；不包含农场选择框背景、用户框背景、文字或其它 UI 元素。
- 输出为 RGBA 透明 PNG，尺寸 `42x32`，主体 bbox 为 `(6, 7, 38, 28)`，四角透明；适合复用于农场选择框和用户菜单框。
- 本次只新增生产素材，未改 `StardewPanel` 引用；后续接入时应作为独立 icon 层定位。
- 验证：Pillow 检查 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`；人工预览确认无背景和框体残影。

# FE-ASSET-VERSION-BADGE-FRAME-1 顶栏版本框空底图

- 新增 `frontend/public/assets/stardew/ui/fields/field_topbar_version_badge_empty_image2.png`，基于 image2 `Top bar.png` 右侧版本号小框风格重绘为空底图。
- 素材只保留棕色/金色像素边框、暗木纹内部、像素阴影和高光；不包含 `v1.12.3` 等版本号文字，也不包含顶栏背景。
- 输出为 RGBA 透明 PNG，尺寸 `228x116`，主体 bbox 为 `(8, 8, 214, 110)`，四角透明；适合前端叠加版本号文本。
- 本次只新增生产素材，未改 `StardewPanel` 引用；如果版本文案未来变长，可用中间暗木纹区域轻微横向拉伸或派生三段式。
- 验证：Pillow 检查 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`；人工预览确认无文字和顶栏背景残影。

# FE-ASSET-USER-ROLE-FRAME-1 顶栏用户角色框空底图

- 新增 `frontend/public/assets/stardew/ui/fields/field_topbar_user_role_empty_image2.png`，基于 image2 `Top bar.png` 右侧用户角色框风格重绘为空底图。
- 素材只保留木质/金色边框、暗棕内容底、像素阴影和高光；已移除人物头像、`管理员` 等角色文字、下拉箭头和顶栏背景。
- 输出为 RGBA 透明 PNG，尺寸 `308x116`，主体 bbox 为 `(7, 8, 297, 110)`，四角透明；内容区为空，方便前端叠加头像、角色文字和箭头。
- 本次只新增生产素材，未改 `StardewPanel` 引用；固定宽度场景可直接使用该空底图，可变宽度场景优先使用三段式素材。
- 验证：Pillow 检查 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`；人工预览确认无头像、文字、箭头和顶栏背景残影。

# FE-ASSET-USER-ROLE-3PIECE-1 顶栏用户角色框三段式素材

- 新增用户角色框三段式透明 PNG：`field_topbar_user_role_left_cap_image2.png`、`field_topbar_user_role_center_tile_image2.png`、`field_topbar_user_role_right_cap_image2.png`。
- 新增无标签横向 sprite sheet：`frontend/public/assets/stardew/ui/fields/field_topbar_user_role_3piece_sheet_image2.png`，顺序为左端、中段、右端，段与段之间保留 16px 透明间距。
- 左/右端保留用户框角部边框、像素阴影和高光；中段为可横向平铺的暗棕木纹内容区和上下边框，不包含头像、角色文字或下拉箭头。
- 单件尺寸分别为左端 `80x116`、中段 `64x116`、右端 `80x116`；sprite sheet 尺寸 `256x116`。本次只新增生产素材，未改 `StardewPanel` 引用。
- 验证：Pillow 检查 4 个文件 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`；人工预览确认 sheet 无标签、三段无头像/文字/箭头残影。

# FE-ASSET-TOP-BAR-USER-AVATAR-1 顶栏用户头像图标

- 新增 `frontend/public/assets/stardew/ui/icons/icon_topbar_user_avatar_image2.png`，基于 image2 `Top bar.png` 右侧用户框内人物头像图标提取并重绘。
- 素材只保留人物头像本体，包含橙色头发、肤色脸部、蓝色衣服、暗色像素描边和高光；不包含用户框背景、角色文字或下拉箭头。
- 输出为 RGBA 透明 PNG，尺寸 `59x73`，主体 bbox 为 `(4, 4, 55, 69)`，四周保留 4px 透明安全边距；适合作为前端用户头像或角色图标。
- 本次只新增生产素材，未改 `StardewPanel` 引用；后续接入时应与用户框空底图和下拉箭头分层叠放。
- 验证：Pillow 检查 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`；人工预览确认无框体、文字或箭头残影。

# FE-ASSET-LOGOUT-BUTTON-FRAME-1 顶栏登出按钮空底图

- 新增 `frontend/public/assets/stardew/ui/buttons/button_topbar_logout_empty_image2.png`，基于 image2 `Top bar.png` 右侧红色登出按钮风格重绘为空底图。
- 素材只保留红色按钮底、暗红/金棕像素边框、像素阴影、高光和按键质感；已移除登出图标和 `登出` 文字，也不包含顶栏背景。
- 输出为 RGBA 透明 PNG，尺寸 `224x116`，主体 bbox 为 `(7, 8, 213, 110)`，四角透明；中央区域为空，方便前端叠加图标和文字。
- 本次只新增生产素材，未改 `StardewPanel` 引用；后续可基于该底图派生 hover/active 状态。
- 验证：Pillow 检查 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`；人工预览确认无登出图标、文字和顶栏角饰残影。
# FE-MODS-DYNAMIC-PAGESIZE-1 模组搜索动态分页

- Nexus 搜索结果从固定 20 条改为“固定卡片高度 + 动态 pageSize”：`.sd-mods-nexus-search-list` 专门用于下载页搜索结果，卡片高度锁定为 `246px`，页面根据搜索结果网格到 `.sd-main-scroll` 底部的可见高度、CSS grid 实际列数和行间距计算 `rows * columns`，再把该值作为 `pageSize` 传给 `searchNexusMods()`。
- 动态 pageSize 范围为 `1..20`，默认恢复值为 `8`；窗口大小、错误/安装日志或结果列表变化时会重新测量。pageSize 变化且已有搜索结果时，会用当前关键词回到第 1 页重新请求，避免不同 pageSize 下同一页码产生跳项。
- 顶部分页器显示“每页 N 个”，总页数改为按动态 pageSize 计算；下载页搜索结果底部重复分页器已移除，避免它把结果区撑出当前 frame 可见范围。加载骨架只按当前 pageSize 和相同固定高度占位，不参与测量，避免 loading 与结果态高度差造成重复刷新。
- 已安装/添加模组列表虽然复用 `.sd-mods-nexus-card`，但没有加 `.sd-mods-nexus-search-list`，因此不受固定搜索卡片高度裁切影响。
- 影响文件：`frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器因本地实例停在登录页，使用临时本地 QA 页面加载真实 `StardewPanel.css` 验证布局公式：1040x1120 下 grid 为 2 列、可见 2 行、pageSize=4，1040x720 下 pageSize=2，520x720 下 1 列 pageSize=1；三种视口下搜索卡片计算高度均为 `246px`。临时 QA 文件已删除。
# FE-JOBS-PROTOTYPE-IMAGE2-1 任务与日志页按 image2 原型视觉重皮肤

- 任务与日志页按 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/04-jobs-logs - 副本.png` 调整为羊皮纸双栏任务台：顶部大标题 + 虚线分隔、像素按钮工具条、左侧任务列表、右侧任务详情/进度/SSE 状态/深色日志终端/VNC 修复提示。
- 未把原型图作为运行时资源或整块背景引用；页面纸纹噪点、木/铜色描边、内阴影、标题虚线、选中态绿色框、状态徽章、进度条斜纹、终端扫描线和 VNC 警告纸条均由 CSS gradient / border / box-shadow / pseudo-element 实现。
- `JobsLogsPage.tsx` 只新增展示钩子：任务列表标题行、任务类型图标 class、短 job id 行、详情标题图标外壳、SSE 提示行容器，并把 VNC 修复提示移到日志下方以贴近原型布局。`getJobs/getJob/getJobLogs`、SSE、清空任务/错误日志、VNC 端口修改、权限判断、loading/error/empty/disabled 逻辑保持不变。
- 按钮和图标复用既有素材：工具条继续使用 `sd-btn-tan` / `sd-btn-delete` PNG 按钮体系；任务类型图标复用 `icon_nav_install_package_image2.png`、`icon_sidebar_chicken.png`、`icon_nav_server_rack_image2.png`、`icon_nav_saves_chest_image2.png`、`icon_nav_mods_crystal_image2.png`；VNC 提示复用 `sprite_blue_device.png`。
- 响应式：样式以 `.sd-jobs-page` 为作用域，并补 `@container sd-main-scroll` 断点；主内容变窄时左右两栏改为单列，工具按钮纵向铺满，日志与长 job id 不产生横向溢出。
- 影响文件：`frontend/src/games/stardew/pages/JobsLogsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；真实 `/instances/stardew/jobs` 当前停在登录页，因此使用已删除的临时 `frontend/jobs-logs-qa.html` 加载同一份 CSS、真实素材和同结构 DOM 做浏览器 QA。1280x900 桌面无横向溢出、VNC 提示首屏可见、console error/warn 为空；390x760 窄屏无横向溢出，按钮文字不溢出，日志列宽不撑开，滚到底部后 VNC 修复提示完整可见。已用 `view_image` 对比原型与桌面/移动实现截图。
# FE-DIAGNOSTICS-GAUGE-CODE-1 诊断页资源仪表圈代码优化

- `DiagnosticsPage` 的 CPU / 内存 / 磁盘三枚资源仪表不再把 `37.8%` 作为整串大字塞进圆心；React 结构拆成数值与 `%` 单位两个 span，保留既有 `latestMetric` 数据、loading/error/empty 状态和 API 调用。
- 仪表圈视觉改为纯 CSS 分层实现：CSS custom properties 驱动进度角度和主题色，`conic-gradient` 绘制进度环，`repeating-conic-gradient` 绘制像素分段，`radial-gradient` / 硬边 `box-shadow` 绘制羊皮纸内芯、外圈高光和像素阴影。
- 未新增图片素材，未使用原型图或截图作为背景；按钮、图标、诊断页其它 image2 素材保持既有复用方式。
- 影响文件：`frontend/src/games/stardew/pages/DiagnosticsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build`；本地浏览器 QA 覆盖桌面与窄屏，重点检查三枚仪表数字/单位不溢出、卡片不重叠、console error/warn 为空。
