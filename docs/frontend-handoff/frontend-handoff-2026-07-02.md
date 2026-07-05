# FE-SHELL-SCALE-1 前端接手记录（2026-07-04）

## 改了什么
- Shell 层新增全局等比缩放：`.sd-shell` 计算 `--sd-ui-scale = max(0.72, min(100vw/1536, 100dvh/1024))`，用反向 `width/height` 配合 `transform: scale(...)` 让整套 Stardew UI 视觉上始终填满浏览器可视区。
- 旧的总览页局部密度覆盖已删除，避免生命周期按钮和统计卡被二次压缩；现在顶栏、左侧栏、主 frame、右 OpsRail、按钮和页面内容一起跟随 Shell 比例变化。
- 右 OpsRail 的 CSS 关闭阈值从 960px 下放到 720px，符合“先等比缩到最小基准，再隐藏右栏”的策略；640px 以下继续使用原有移动端顶栏/图标导航/单列内容规则。
- `StardewPanel.tsx` 的 OpsRail 自动折叠估算改为按同一设计基准、最小 scale 和当前栏宽公式计算视觉主内容宽度，避免 JS 按旧宽栏尺寸过早折叠右栏。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/src/games/stardew/StardewPanel.tsx`
- 未改页面业务组件、启动/停止/重启 handler、邀请码刷新/复制、API、权限判断或后端逻辑。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 临时本地 HTTP QA 页（系统 temp 目录，已删除）内联真实构建 CSS 验证：760x504 下 `transform matrix(0.72,...)`、三栏保留、无页面级横/纵溢出；1920x1080 下 `transform matrix(1.05469,...)`、Shell 视觉宽高为 1920x1080、按钮高度随 scale 放大到约 42px。
- 登录态手动检查 `/instances/stardew/overview`：全屏应比 1536 基准继续等比放大；窗口缩到约 760x504 时左右栏仍保留且文字/按钮可读；再窄时先隐藏右栏，再到 640px 以下进入移动端布局。

## 下一步注意事项
- 如果继续调整最小基准，优先改 `SHELL_MIN_UI_SCALE`、CSS `--sd-min-ui-scale` 和 OpsRail 折叠阈值，三者要一起复核。
- 因为 Shell 使用整体 transform，后续如果发现弹窗、fixed 元素或截图工具坐标异常，优先检查是否有元素脱离 `.sd-shell` 渲染。

# FE-PLAYERS-LIST-LEFT-1 前端接手记录（2026-07-04）

## 改了什么
- 玩家管理页首屏布局调整为左侧宽列显示“在线玩家”，右侧窄列显示“玩家活动 / 最近事件”。
- 取消此前 `.sd-players-list-section` 固定落到右列第三行的覆盖，避免桌面首屏出现大面积空白。
- “服务器信息（Junimo）”继续保留在页面底部，并改为横跨整行的低频调试信息。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.css`
- 未改 `PlayersPage.tsx`、玩家数据接口、字段含义、权限判断、轮询或后端逻辑。

## 如何验证
- `cd frontend; npm.cmd run build`
- 登录态手动检查 `/instances/stardew/players`：首屏左侧应是在线玩家表，右侧是最近事件；900px 以下仍回到单列，玩家表只在自身容器内横向滚动。

## 下一步注意事项
- 后续如果再调整玩家页两列比例，优先改文件尾部 `FE-PLAYERS-LIST-LEFT-1` 覆盖段，避免早期 `FE-PLAYERS-PROTOTYPE-IMAGE2-1` 段里的旧 grid-row 规则再次把玩家表推到右下。

# FE-DIAG-GAUGE-TOMIK-1 前端接手记录（2026-07-04）

## 改了什么
- 诊断页 CPU/内存/磁盘三个资源圆环从"铜钱"conic-gradient 样式改为用户选定的 Tomik23 circular-progress-bar 视觉：灰色 `#e6e6e6` 底环、yellow→`#ff0000` 线性渐变描边、圆头端帽、中心百分比。
- 纯 SVG 复现（linearGradient + stroke-dasharray/dashoffset + `rotate(-90)` 顶部起画 + `transition: stroke-dashoffset .6s`），未引入 Tomik23 的 JS 库或任何新依赖。
- `GaugeCard` 的 `color` prop 移除，改为每卡传唯一 `gradientId`（SVG 渐变 id 不能重复）；`percent<=0` 或无数据时只渲染底环，避免 round 端帽在 0% 显示成小圆点。
- CSS 删除铜钱纹路（刻齿、双层内芯伪元素、三层金圈阴影），新增 `.sd-diag-gauge-svg/-track/-arc`；中心数字改页面墨色，去掉羊皮纸描边 text-shadow。

## 影响文件/接口
- `frontend/src/games/stardew/pages/DiagnosticsPage.tsx`（仅 `GaugeCard` 与三处调用）
- `frontend/src/games/stardew/StardewPanel.css`（image2 诊断段的 gauge 规则）
- 未改 metrics 轮询、API、空态逻辑（"启动后显示"/"103 GB / 932 GB"副标题保持）。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- Playwright 真实登录态截图 1366x900：磁盘 11% 显示顶部黄→红渐变圆头弧 + 灰底环，CPU/内存空态灰底环 + "—"；390x844 单列无溢出；两视口 pageerror 为 0。

## 下一步注意事项
- 三个 gauge 的渐变 id 为 `sd-gauge-grad-cpu/memory/disk`，如果以后同页再加 gauge，必须传新的唯一 id。
- 若想恢复按阈值变色（绿/黄/红），改 `linearGradient` 的两个 stop 颜色或按 `value` 切换渐变即可，不要回到 conic 方案。
- 底环/弧线宽度在 `.sd-diag-gauge-track/-arc` 的 `stroke-width: 11`（viewBox 120 坐标系），调粗细只动这两处。

# FE-UNIFIED-CARD-PARCHMENT-TONE-1 前端接手记录（2026-07-04）

## 改了什么
- 把总览统计卡当前使用的浅羊皮纸暖黄提升为统一小卡片背景色。
- 在 `StardewPanel.css` 文件尾部覆盖 `--sd-save-card-bg` 和 `--sd-save-card-bg-strong`，让复用统一小卡片变量的非模组页小框全部跟随同一背景。
- 总览 `.sd-mc` 继续保持同色且无斜纹；本次不改变卡片尺寸、边框、圆角、阴影、文字布局或状态徽章。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.css`
- 未改 TSX、API、权限、轮询、数据模型或后端逻辑。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。

## 下一步注意事项
- 后续如继续微调“小卡片背景色”，优先改 `FE-UNIFIED-CARD-PARCHMENT-TONE-1` 的两个共享变量，避免各页面重新分叉。
- 模组页主体卡片此前刻意排除在统一小卡片规则外；若要覆盖模组页，需要单独评估 Nexus 卡片高度和弹层。

# FE-INSTALL-STEAM-AUTH-ICON-1 前端接手记录（2026-07-04）

## 改了什么
- 安装页“Steam 认证”卡片空状态中间的大图标从 CSS 渐变圆球改为 `<img>`，资源复用安装进度第三步的 `/assets/stardew/ui/install/icon_install_step_steam_image2.png`。
- “Steam 认证”栏目标题左侧的小图标也改为同一张 Steam PNG 背景图，不再单独用 CSS radial-gradient 画一个小 Steam 圆点。

## 影响文件/接口
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 未改安装状态、Steam 认证流程、Steam Guard/扫码交互、安装日志、SSE 或后端接口。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。
- 内置浏览器打开 `http://127.0.0.1:5173/instances/stardew/install`：当前环境停在登录页，应用壳非空、title 为 `Stardew Anxi Panel`、console error/warn 为空。

## 下一步注意事项
- 后续如继续调整安装页 Steam 图标，优先替换 `icon_install_step_steam_image2.png` 这一个共享资源，避免进度条和认证卡再次分叉。
- 需要在真实登录态补看安装页空状态，确认认证卡中间大图标和顶部安装进度 Steam 步骤图标一致。

# FE-SETTINGS-FILL-GAP-1 前端接手记录（2026-07-04）

## 改了什么
- 设置页由原来的三段式布局改成左右两列堆叠布局，减少中间大面积空白。
- 左列依次渲染“面板版本 / 用户管理 / 端口信息 / 其他设置”；右列依次渲染“安全与权限 / 审计日志 / 安全建议”。
- 新增 `.sd-settings-content-grid` 和 `.sd-settings-stack`；中等宽度保持两列，`780px` 以下再回到单列。

## 影响文件/接口
- `frontend/src/games/stardew/pages/SettingsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 未改用户管理、审计日志、VNC 端口、权限判断、API 或后端逻辑。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。

## 下一步注意事项
- 后续新增设置区块时按左右两列信息密度放入对应 `.sd-settings-stack`；不要恢复旧的 `top/main/bottom` 三段网格，否则短列下方会再次出现空洞。
- 如果端口卡在极窄宽度挤压，优先调整 `780px` 断点或端口卡内部 grid，不要把整个设置页过早切回单列。

# FE-INVITE-CARD-COPY-ORDER-1 前端接手记录（2026-07-04）

## 改了什么
- 新增共享组件 `InviteCodeCard`，集中处理邀请码展示、复制、刷新、复制失败提示和运行/启动/未运行状态文案。
- `ServerSummaryCard` 删除内置复制状态和 `handleCopyInvite()`，摘要卡底部直接复用 `InviteCodeCard`。
- 总览页服务器控制区删除旧 `sd-invite-panel` 卡片 JSX、`copied/copyError` 状态和 `handleCopy()`，改用同一个 `InviteCodeCard`，避免服务器页与总览页两套邀请码 UI 分叉。
- 复制按钮现在在刷新按钮左侧，且只有存在邀请码时才渲染；无邀请码时不会保留空网格列。`.sd-players-invite-row` 改为三列，右侧新增 `.sd-players-invite-actions` 按钮组，窄屏下按钮组整行铺满。

## 影响文件/接口
- `frontend/src/games/stardew/InviteCodeCard.tsx`
- `frontend/src/games/stardew/ServerSummaryCard.tsx`
- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 未改后端 API、`dashboardData.refreshInviteCode()`、邀请码轮询、权限判断、启动/停止/重启业务逻辑。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。
- 内置浏览器打开 `http://127.0.0.1:5173/instances/stardew/server` 和 `/instances/stardew/overview`：当前环境停在登录页，应用壳非空、title 为 `Stardew Anxi Panel`、console error/warn 为空。

## 下一步注意事项
- 后续调整邀请码卡片时优先改 `InviteCodeCard` 和 `.sd-players-invite-*` 样式，避免在总览页重新写一套 `sd-invite-panel`。
- 需要在真实登录态补看 1280px 与 390px 两个视口，重点确认有邀请码时按钮顺序为“复制 / 刷新”，无邀请码时刷新按钮不被空列挤压。

# FE-OVERVIEW-METRIC-CLEAN-BG-1 前端接手记录（2026-07-04）

## 改了什么
- 总览页四个 `.sd-mc` 统计卡移除斜向纸纹背景。
- 背景改为干净、偏浅的羊皮纸暖黄渐变；后续按反馈从偏白略微压黄，但不恢复旧的高饱和黄色。保留原有卡片尺寸、边框、角饰、文字布局和状态徽章。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.css`
- 未改 TSX、API、权限、轮询、数据模型或后端逻辑。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。

## 下一步注意事项
- 后续如果继续调整总览统计卡，优先修改文件尾部 `FE-OVERVIEW-METRIC-CLEAN-BG-1` 覆盖，不要在早期 `.sd-mc` 皮肤段重新加斜纹或点状纹理。

# FE-INSTALL-HERO-SCENE-REMOVE-1 前端接手记录（2026-07-04）

## 改了什么
- 安装页顶部状态横幅删除右侧大农舍场景图 `.sd-install-farm-scene`。
- `InstallPage.tsx` 不再渲染 `/assets/stardew/ui/sprites/sprite_farmhouse_scene.png` 这张顶部大图。
- `StardewPanel.css` 清理 `.sd-install-farm-scene`、其内部图片和遮罩伪元素规则；状态横幅从三列改为“小土芽图标 + 状态信息”两列。

## 影响文件/接口
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 未改安装状态、Steam 认证、安装进度、日志、SSE 或后端接口。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。

## 下一步注意事项
- 其它页面仍可使用 `sprite_farmhouse_scene.png` 作为场景素材；本次只移除安装页顶部大图，不要误删总览/存档/服务器页的场景引用。

# FE-SETTINGS-ACCOUNT-CARD-REMOVE-1 前端接手记录（2026-07-04）

## 改了什么
- 设置与审计页顶部删除“当前账号”卡片，不再在设置页重复展示用户名、角色、已登录状态和退出登录按钮。
- 顶部摘要区从三卡改为两卡：只保留“面板版本”和“安全与权限”；窄屏下仍沿用原有单列布局。
- `SettingsPage.tsx` 删除 `AccountSection`，设置页组件不再接收 `onLogout`；顶栏原有登出入口保持不变。
- `StardewPanel.css` 清理 `sd-settings-account-*` 专用样式、账号标题图标规则和统一卡片选择器里的残留账号类。

## 影响文件/接口
- `frontend/src/games/stardew/pages/SettingsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 未改后端接口、session、权限判断、用户管理、审计日志、VNC 端口或顶栏登出逻辑。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。

## 下一步注意事项
- 用户登出仍从 Stardew Shell 顶栏入口触发；后续不要把设置页内的账号卡片恢复成第二个登出入口，除非产品明确需要重复入口。
- 如果继续调整设置页顶部区域，请按两卡布局检查 920px 以上桌面宽度和 920px 以下单列布局。

# FE-PAGE-HEADER-SHADOW-1 前端接手记录（2026-07-04）

## 改了什么
- 在 `StardewPanel.css` 文件尾部新增页头阴影清理覆盖。
- 统一移除服务器、存档、任务、玩家、模组、诊断、安装、设置页页头图标的 `filter`、标题文字的 `text-shadow`，以及右侧虚线分隔的 `filter/box-shadow`。
- 保留页头 DOM、标题字号、图标尺寸、虚线分隔和操作按钮位置，只去掉纸面上的阴影背景。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.css`
- 未改 TSX、API、权限、轮询、数据模型或后端逻辑。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。

## 下一步注意事项
- 如果后续重写统一页头样式，注意不要在 `.sd-page-title`、`.sd-page-icon` 或 `.sd-page-header::after` 上重新加投影；需要层次感时优先靠颜色/线条对比。

# FE-PAGE-TOP-ALIGN-1 前端接手记录（2026-07-04）

## 改了什么
- 在 `StardewPanel.css` 文件尾部新增页面顶部对齐兜底，把 `.sd-main-scroll` 下所有 routed `.sd-page` 的 `padding-block-start` 统一置为 `0`。
- 解决任务、诊断、安装、设置等页面后置 image2 皮肤规则重新声明完整 `padding` 后，页面内容没有贴住主内容 frame 顶部的问题。
- 这次没有调整页面 DOM、卡片顺序、grid 列宽、左右/底部 padding 或任何业务状态，只让页面整体向上贴齐。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.css`
- 未改 TSX、API、权限、轮询、数据模型或后端逻辑。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。

## 下一步注意事项
- 后续如果新增 routed 页面并给页面根类重新写 `padding`，需要确认文件尾部的 `FE-PAGE-TOP-ALIGN-1` 仍覆盖该根类顶部间距。
- 如果某个页面确实需要额外顶部留白，应优先在页面内部第一块内容上处理，不要重新抬高整个 `.sd-page`。

# FE-SERVER-ACTION-CARDS-1 前端接手记录（2026-07-04）

## 改了什么
- 服务器控制页动作卡片重新排布：服务器摘要卡保持整行顶部，生命周期控制卡下移到摘要卡下方左侧，快捷操作卡放在同一行右侧。
- “全服消息”继续横跨整行，排在生命周期/快捷操作之后；“控制台命令”排在全服消息之后。
- 快捷操作按钮统一加 `.sd-btn--lg`，和生命周期按钮共用 lg 尺寸令牌；删除快捷操作区原来的 64px 高卡片按钮、左侧伪图标和两列 grid 按钮布局。
- 容器宽度小于 1180px 时，服务器页恢复单列顺序：标题、摘要、生命周期、快捷操作、全服消息、控制台命令；760px 以下快捷操作按钮铺满一行，避免长文案挤压。

## 影响文件/接口
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 未改启动/停止/重启、手动备份、计划重启、VNC 显示、VNC 跳转、服务器设置占位等 handler；未改 API、权限判断或 disabled 业务逻辑。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。
- 内置浏览器打开 `http://127.0.0.1:5174/instances/stardew/server` 时真实应用停在登录页；已确认页面非空、title 为 `Stardew Anxi Panel`、无框架错误覆盖、console error/warn 为空。
- 因缺少当前登录态，本次未完成服务器页登录态截图验证；后续应在登录态补看 1280/390 两个视口，重点确认生命周期与快捷操作同一行、快捷按钮高度为 40px、窄屏单列无横向溢出。

## 下一步注意事项
- 快捷操作按钮后续新增项也应叠加 `.sd-btn--lg`，不要恢复 `.sd-server-quick-grid > button` 的自定义高度。
- 若 VNC 开启后多出“跳转VNC控制”按钮，右侧快捷卡会自动换行；不要为凑单行把卡片宽度硬撑到产生横向溢出。

# FE-SERVER-INVITE-IN-SUMMARY-1 前端接手记录（2026-07-04）

## 改了什么
- 服务器控制页中部独立“邀请代码”卡片已移除，邀请码入口收敛到页面顶部的 `ServerSummaryCard`。
- 顶部“邀请加入码”行中，刷新按钮移动到邀请码显示区右侧；运行中/启动中可点击，未运行时禁用并提示“服务器未运行时无法获取邀请码”。
- 复制按钮仍只在运行中且已有邀请码时显示；复制失败提示仍由 `ServerSummaryCard` 处理。
- 删除 `ServerControlPage` 中独立邀请码卡片专用的 `copied` / `copyError` 状态和 `handleCopy()`，不再维护第二套复制逻辑。
- 删除卡片后，服务器页“全服消息”改为横跨整行，避免桌面双列布局出现空位。

## 影响文件/接口
- `frontend/src/games/stardew/ServerSummaryCard.tsx`
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 未改后端接口、`dashboardData.refreshInviteCode()`、启动/重启后等待新邀请码轮询、权限判断或 disabled 业务逻辑。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。
- 内置浏览器打开 `http://127.0.0.1:5174/instances/stardew/server` 时真实应用停在登录页；已确认页面非空、title 为 `Stardew Anxi Panel`、console error/warn 为空。
- 内置浏览器 DOM snapshot 接口在当前环境报 `incrementalAriaSnapshot` 不兼容，改用只读 evaluate 检查登录页状态；尝试用 `data:` 临时 QA 页加载真实 CSS 被内置浏览器 URL policy 拦截，因此本次未留下服务器页视觉截图。

## 下一步注意事项
- 若后续要继续视觉精调，优先在真实登录态或允许的临时 localhost QA 页验证 1280/390 两个视口，重点看邀请码行按钮顺序和“全服消息”整行宽度。
- `ServerSummaryCard` 同时被玩家页复用；调整 `.sd-players-invite-row` 时要兼顾玩家页窄屏单列行为。

# FE-BTN-UNIFY-1 前端接手记录（2026-07-04）

## 改了什么
- 9 个页面（总览/服务器/存档/任务日志/玩家/模组/诊断/安装/设置）按钮与操作区统一化。
- 尺寸：`stardew-theme.css` 新增按钮尺寸令牌与修饰符，三档制——lg `40px/15px`（生命周期启动/停止/重启、诊断页头部主操作、安装页主 CTA）、md `28px/13px`（默认档，工具栏/卡片/弹窗）、sm `22px/12px`（表格与列表行内、迷你重试，通过 `.sd-btn--sm`）。`sd-btn-img` 图标随档位 20/15/12px，JSX 内联宽高全部删除。
- 语义色收敛为 绿(主)/棕(次)/红(危险) 三种：删除死样式 `sd-btn-gold`、`sd-btn-red`，删除 `.sd-btn-blue`（3 处使用改 tan）与 `.sd-btn-xs`（改 `.sd-btn--sm`）。
- 危险确认弹窗确认键统一 `sd-btn-delete`（确认停止/确认删除/确认清空/彻底删除/覆盖恢复），确认重启用 `sd-btn-green`；弹窗底部统一"取消(左) + 确认(右)"；有底部动作的弹窗去掉头部"关闭"，纯查看弹窗保留"关闭"。
- 操作区：`stardew-theme.css` 新增共享 `.sd-actionbar`（含 `--end` 右对齐变体、640px 全宽换行）与 `.sd-rowactions`，挂在各页既有操作区容器类旁。
- 删除 `StardewPanel.css` 中全部逐页按钮尺寸覆写（诊断 46px、总览 48px/服务器 52px 生命周期、任务页 38px 工具栏+CSS 自绘图标、服务器发送/执行/标题动作、玩家页 42px!important 等）；服务器页生命周期从全宽巨条改为与总览一致的横向 lg 按钮排。
- 修复总览页无邀请码时邀请面板被挤成 1px 竖排的结构性 bug：`.sd-ctrl-row` 三列(含 1px 分隔列)改两列，`.sd-ctrl-div` 元素隐藏（中缝线由 `.sd-ov-section::before` 绘制）；文件尾部 `FE-OVERVIEW-PROTOTYPE-IMAGE2-2` 的两个 ≥901px 断点块同步改掉三列模板。
- 文案字典：拉数据→"刷新"、提交→"保存"、"X并启动"收敛（创建并启动/上传并启动/导入并启动/启动此存档）、服务器页"备份已保存进度"→"手动备份"、tooltip"重新获取邀请码"→"刷新邀请码"、busy 省略号统一"…"。

## 影响文件/接口
- `frontend/src/games/stardew/stardew-theme.css`、`frontend/src/games/stardew/StardewPanel.css`
- `frontend/src/games/stardew/SavesSection.tsx` 与 `pages/` 下 OverviewPage、ServerControlPage、JobsLogsPage、PlayersPage、ModsPage、DiagnosticsPage、InstallPage、SettingsPage
- 未改任何 handler、API 调用、权限判断、disabled 逻辑、路由或后端接口。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过（项目无 lint/test 脚本）。
- 已用 Playwright（真实后端 + anxi 登录态）对 9 页 × 1920/1366/1024/390 四视口做改前/改后各 36 张全页截图对比：同类按钮同档尺寸、服务器页与总览页生命周期按钮一致、诊断页双主操作同为 lg PNG 按钮、窄屏操作区换行无溢出/重叠。
- 总览页在"已停止无邀请码"状态下邀请面板宽度从 1px 恢复到约 538px（1366 视口实测）。
- console 仅有改前即存在的 metrics 接口 500（本机 Docker 服务不可用），无新增 error/warn。

## 下一步注意事项
- 新按钮/操作区一律走三档令牌：默认 md，行内加 `.sd-btn--sm`，页面级主操作加 `.sd-btn--lg`；不要再写页面作用域的按钮 height/font-size 覆写或内联 style。
- 危险确认一律 `sd-btn-delete`；`sd-btn-start/stop/restart` 只用于生命周期按钮本体。
- 诊断页导出按钮的下载图标现在挂在 `.sd-diag-export-btn::before`；改类名时同步 CSS。
- `.sd-ctrl-div` 元素在总览已被 CSS 隐藏但 DOM 仍在（有邀请码时渲染）；若后续重构总览控制区，可顺手删掉该条件渲染。

# FE-NEXUS-ERROR-TEXT-1 前端接手记录（2026-07-04）

## 改了什么
- `frontend/src/core/helpers.ts` 的 `errorCodeMap` 新增 Nexus 错误码中文映射。
- 下载模组页搜索 Nexus 或会员安装失败时，前端优先按错误码显示稳定中文：未配置 Key、需要 OAuth/认证、未找到 Mod、Key 无效/权限不足、请求过频、通用请求失败。
- 这样即使后端历史构建或代理层返回的 `message` 出现编码异常，用户也不会再看到 `璇锋眰澶辫触` 这类乱码。

## 影响文件/接口
- `frontend/src/core/helpers.ts`
- 依赖既有后端错误码：`nexus_api_key_missing`、`nexus_auth_required`、`nexus_mod_not_found`、`nexus_unauthorized`、`nexus_rate_limited`、`nexus_request_failed`。
- 未改前端请求路径、Nexus 搜索参数、安装流程或页面布局。

## 如何验证
- `cd frontend; npm.cmd run build`

## 下一步注意事项
- 新增 Nexus 错误码时要同步补 `errorCodeMap`，避免回退到后端原始 message。
- 这只是 Nexus 链路兜底；其它模块的历史乱码仍需按接口逐步清理。

# FE-MAIN-PAGE-FRAME-SLICES-1 前端接手记录（2026-07-04）

## 改了什么
- 将所有 Stardew 页面共用的 `.sd-main` 主内容 frame 从整张 `main_page_frame_empty_image2.png` 的 `100% 100%` 拉伸背景，改为 image2 空框切片平铺。
- 从 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/03-saves-page-frame-empty-image2.png` 按原始像素裁出 9 个运行时素材：左上/右上/左下/右下四角，top/bottom/left/right 四条边 tile，以及中心羊皮纸 tile。
- `.sd-main` 现在使用 9 层 CSS background：四角 `no-repeat`，上下边 `repeat-x`，左右边 `repeat-y`，中间纸纹 `repeat`。窗口缩放时边框纹理不再被横向或纵向拉伸。
- 保留既有 `.sd-main-scroll` 统一滚动视口、内侧 inset 和所有路由 DOM；本次只替换 frame 背景绘制方式，没有改页面组件或业务逻辑。

## 影响文件/接口
- `frontend/src/games/stardew/stardew-theme.css`
- `frontend/src/games/stardew/StardewPanel.css`
- 新增 `frontend/public/assets/stardew/ui/panels/main_page_frame_corner_top_left_image2.png`
- 新增 `frontend/public/assets/stardew/ui/panels/main_page_frame_corner_top_right_image2.png`
- 新增 `frontend/public/assets/stardew/ui/panels/main_page_frame_corner_bottom_left_image2.png`
- 新增 `frontend/public/assets/stardew/ui/panels/main_page_frame_corner_bottom_right_image2.png`
- 新增 `frontend/public/assets/stardew/ui/panels/main_page_frame_edge_top_tile_image2.png`
- 新增 `frontend/public/assets/stardew/ui/panels/main_page_frame_edge_bottom_tile_image2.png`
- 新增 `frontend/public/assets/stardew/ui/panels/main_page_frame_edge_left_tile_image2.png`
- 新增 `frontend/public/assets/stardew/ui/panels/main_page_frame_edge_right_tile_image2.png`
- 新增 `frontend/public/assets/stardew/ui/panels/main_page_frame_center_tile_image2.png`
- 未改后端接口、路由、权限判断、数据轮询或页面组件结构。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 使用已删除的临时 `frontend/page-frame-slices-qa.html` 加载真实 `stardew-theme.css`、`StardewPanel.css` 和 public 素材做内置浏览器 QA。
- 1280x720：`.sd-main` 背景层数量为 9；`background-repeat` 为四角 `no-repeat`、上下 `repeat-x`、左右 `repeat-y`、中心 `repeat`；不包含旧的整图 `100% 100%` 拉伸；页面无横向溢出；滚轮后 `.sd-main-scroll.scrollTop` 从 `0` 到 `520`；console error/warn 为空。
- 390x760：背景层数量仍为 9，四边重复规则保持；页面无横向溢出；滚轮后 `.sd-main-scroll.scrollTop` 从 `0` 到 `420`；console error/warn 为空。

## 下一步注意事项
- 后续若继续调整主内容 frame，优先替换这 9 个切片或重新裁切切片；不要把整张 `main_page_frame_empty_image2.png` 再改回 `background-size: 100% 100%`，否则窗口缩放时边框会再次被拉伸。
- `.sd-main-scroll` 仍是唯一滚动视口；不要为了 frame 视觉把滚动放回 `.sd-main` 或单页组件。
- 当前四角/边厚通过 `.sd-main` 内的 `--sd-main-frame-corner-size` 和 `--sd-main-frame-edge-thickness` 控制。如更换切片尺寸，需要同步复查这些变量和既有内侧 inset。

# FE-MODS-DEPENDENCY-POPOVER-1 前端接手记录（2026-07-04）

## 改了什么
- 修复下载模组页 Nexus 搜索结果卡片里的前置信息无法正常展开/可见的问题。
- `NexusRequiredModsBadge` 从 `<details>/<summary>` 默认行为改为受控按钮 + 弹层，`ModsPage` 用 `openNexusRequiredModId` 记录当前打开项。
- 打开后点击信息弹层外部会自动收起；切换 tab、搜索结果刷新导致当前 mod 不在列表中时也会关闭，并支持 `Escape` 关闭。
- 搜索卡片固定高度和动态 pageSize 逻辑保持不变；只有当前前置弹层打开的卡片临时加 `sd-mods-nexus-card-dependency-open`，放开卡片和 footer 的 `overflow`，避免弹层被裁切。

## 影响文件/接口
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 未改后端接口、Nexus 搜索参数、安装流程、动态 pageSize 计算或已安装模组列表。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 内置浏览器真实路由仍可能受登录态影响，因此使用已删除的临时 `frontend/mods-dependency-qa.html` 加载真实构建 CSS 做交互 QA。
- 1280x720：初始弹层隐藏、卡片默认 `overflow:hidden`；点击“缺少前置mod”后弹层显示、按钮 `aria-expanded=true`、卡片和 footer 临时 `overflow:visible`，信息未被裁切；点击外部区域后弹层关闭并恢复默认裁切，console error/warn 为空。
- 390x760：弹层打开后宽度落在视口内，无水平溢出，console error/warn 为空。

## 下一步注意事项
- 前置信息弹层的外部点击监听只在 `openNexusRequiredModId !== null` 时挂载；后续如果把弹层改为 portal，需要同步调整外部点击判定范围。
- 不要把搜索卡片的 `overflow: visible` 改成常驻，否则动态 pageSize 的固定卡片高度约束会失效，搜索结果可能重新撑开 frame。

# JOB-DISPLAY-NAME-1 前端接手记录（2026-07-04）

## 改了什么
- `Job` 类型新增可选 `displayName`，并新增 `jobDisplayName(job)` helper。
- 任务页列表/详情、右侧 OpsRail“进行中”、右侧“近期任务”和总览页近期事件都优先展示 `displayName`；旧任务或普通任务没有该字段时继续回退原来的 `type` / `typeLabel`。
- 目标效果：并行安装多个 Nexus 依赖时，不再只看到多条 `mod_remote_install`，而是能看到 `mod_remote_install · <Mod 名>`。

## 影响文件/接口
- `frontend/src/types.ts`
- `frontend/src/core/helpers.ts`
- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/pages/JobsLogsPage.tsx`
- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- 依赖后端 job payload 的可选 `displayName` 字段。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`。

## 下一步注意事项
- 前端不要用 `displayName` 做任务类型判断；所有逻辑判断仍看 `job.type`。
- 如果后续其它页面展示 job 名称，优先复用 `jobDisplayName(job)`。

# FE-OPSRAIL-DOWNLOAD-PROGRESS-1 前端接手记录（2026-07-04）

## 改了什么
- 右侧 OpsRail 的“进行中”任务卡接入远程 Mod 安装下载日志：`mod_remote_install` / `mod_nexus_install` 优先解析 `下载进度：已下载 ...（xx.x%）`，不再只靠历史耗时估算到 95%。
- `useStardewDashboardData` 为 active job 拉取一次初始日志，并订阅 job SSE 的 `log` 事件，维护 `jobLogsByJobId` 给 `OpsRailActiveCard` 使用；每个 job 保留最近 200 条日志。
- 模组页扩展 batch 一旦拿到新的 `jobId` 就立即 `refreshJobs()`，让右栏进行中任务尽快出现；Premium/API Key 安装拿到 `jobId` 后也主动刷新 jobs。

## 影响文件/接口
- `frontend/src/games/stardew/useStardewDashboardData.ts`
- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/stardew-routes.ts`
- 后端接口未新增；依赖既有 `GET /api/jobs`、`GET /api/jobs/:jobId/logs` 和 `GET /api/jobs/:jobId/stream` 的 `log` / `finished` 事件。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 真实扩展下载链路需在浏览器扩展 + Nexus 登录态下联调：点击普通一键安装后，扩展返回 jobId 时右栏应立即出现 `mod_remote_install`；任务日志出现下载百分比后，右栏进度应随下载推进，而不是等下载 100% 后直接跳到 95%。

## 下一步注意事项
- 右栏下载进度依赖后端日志文案 `下载进度：已下载 ...（xx.x%）`；如果后端文案改动，需要同步调整 `DOWNLOAD_PROGRESS_RE`。
- `jobLogsByJobId` 是 dashboard 公共数据层字段，后续其它轻量组件可复用，但不要在各卡片里重复开启 job 日志轮询。

# NEXUS-EXT-DOWNLOAD-GUARD-1 前端/扩展接手记录（2026-07-04）

## 改了什么
- 浏览器扩展自动提交链路增加最终 URL 兜底校验：只有真实 Nexus CDN `.zip` 才能触发面板远程安装任务。
- `background.js` 的 `finishInstall()` / `postRemoteInstall()` 会拒绝空 URL、普通 Nexus 页面 URL、非 CDN URL 和非 ZIP URL。
- `panel-bridge.js` 的 `PANEL_REMOTE_INSTALL` 同样校验 URL，避免面板页桥接把未就绪页面状态提交给后端。

## 影响文件/接口
- `browser-extensions/nexus-slow-installer/background.js`
- `browser-extensions/nexus-slow-installer/panel-bridge.js`
- 面板 API 不变；扩展仍调用 `POST /api/instances/:id/mods/remote/install`，但调用前更严格。

## 如何验证
- 已执行：`node --check browser-extensions/nexus-slow-installer/background.js`
- 已执行：`node --check browser-extensions/nexus-slow-installer/panel-bridge.js`
- 已执行：`node --check browser-extensions/nexus-slow-installer/content.js`

## 下一步注意事项
- 用户更新扩展后需要在 Chrome/Edge 扩展管理页重新加载扩展，并刷新面板页让新的 `panel-bridge.js` 注入。
- 批量安装时如果某个后台页停在 Nexus 下载页，预期行为是该项保持捕获中并最终超时失败，而不是创建一个无 ZIP 的 `mod_remote_install` job。
- 真实安装是否完成仍以后端 job 状态为准；扩展 `queued` 只表示已经成功创建面板任务。
# FE-PLAYERS-PROTOTYPE-IMAGE2-1 前端接手记录（2026-07-03）

## 改了什么
- 玩家管理页按 image2 原型 `05-players - 副本.png` 做视觉重皮肤：顶部六张摘要卡、邀请加入码横条、中部 Junimo 服务器终端 + 在线玩家表、底部玩家活动历史和管理员操作区对齐原型布局。
- 原型图没有作为运行时背景或整块资源引用。羊皮纸底、纸纹噪点、铜色边框、内描边、角钉、分隔线、绿字终端、玩家表和禁用操作小按钮均由 CSS gradient / border / box-shadow / pseudo-elements 实现。
- `PlayersPage.tsx` 只改展示结构：摘要从旧网格压成 6 卡，玩家表改为“玩家名 / 角色 / 位置 / 在线时长 / 状态 / 操作”6 列；现金、农场收入、个人收入、钱包模式和联机 ID 仍放在行 title 中保留可查信息。邀请码复制/刷新、玩家刷新、loading/error/empty、管理员权限和待接入 disabled 状态未改。
- 按钮/图标复用现有素材：页头、摘要卡和分区标题使用 `ui/icons` 下 image2 PNG；复制用 `sd-btn-green`，刷新/权限用 `.sd-btn-blue`，踢出/封禁用 `sd-btn-delete`。没有新增图片素材。
- 响应式补强：`.sd-players-page` 增加最终覆盖和容器查询；桌面保留六卡 + 左右分栏，中等宽度收成三卡/双操作，窄屏单列。补了 `.sd-players-page > * { min-width: 0 }`，避免表格或按钮把整页 grid 撑宽；玩家表仅自身横向滚动。

## 影响文件/接口
- `frontend/src/games/stardew/pages/PlayersPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端接口、路由、API 参数、权限判断或数据结构。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 真实 `/instances/stardew/players` 当前受登录态影响停在登录页，页面身份检查显示未进入玩家页但 console error/warn 为空。
- 使用已删除的临时 `frontend/players-qa.html` 加载同一份 CSS、public 素材和同结构 DOM 做内置浏览器 QA。
- 1280x900 桌面：六张摘要卡一行显示，邀请加入码横条、Junimo 终端、在线玩家表和活动流在首屏对应原型；页面无横向溢出，玩家表操作列首屏可见且不需要横向滚动，console error/warn 为空。
- 390x760 窄屏：玩家页收为单列，页面无横向溢出；邀请码和按钮可读，表格仅在表格容器内部横向滚动，管理按钮文字不溢出。
- 已用 `view_image` 查看原型图、最终桌面实现截图和移动实现截图。

## 下一步注意事项
- 玩家行的踢出/消息/更多、底部踢出/封禁/白名单/权限仍是待接入禁用入口；后续接 API 时优先走 Junimo / `stardew_junimo` driver 已有能力，不要在前端或 API 层绕过 driver 堆 Stardew 逻辑。
- 玩家表为了贴近原型没有继续展开现金/收入/钱包列，这些信息现在在行 `title` 中；如果用户要求显式展示，可在表格下方加详情抽屉或二级信息行，避免恢复 9 列导致首屏比例跑偏。
- 日期摘要卡使用较短的 `第 N 年春季D日` 格式以匹配原型卡片宽度；如果其它页面也要复用日期格式，应另建格式化 helper，不要直接复用本页内部函数。

# FE-INSTALL-IMAGE2-ICONS-2 前端接手记录（2026-07-03）

## 改了什么
- 用户反馈安装页 CSS 自绘图标质感不佳后，已从 image2 安装页原型 `08-install - 副本.png` 提取并抠图生成透明 PNG 小素材，替换顶部状态横幅土芽和五步安装时间线图标。
- 新增 `frontend/public/assets/stardew/ui/install/`，包含 6 个运行时单图：`icon_install_status_seed_image2.png`、`icon_install_step_seed_image2.png`、`icon_install_step_box_image2.png`、`icon_install_step_steam_image2.png`、`icon_install_step_download_image2.png`、`icon_install_step_star_image2.png`。
- `InstallPage.tsx` 将时间线从 `STEP_ART_CLASS` + CSS class 改为 `STEP_ICON_SRC` + `<img>`；顶部土芽改为 wrapper + `<img>`，避免图片在移动端被 grid 拉伸。
- `StardewPanel.css` 删除安装页步骤 seed/box/steam/download/star 的伪元素绘制，以及顶部土芽的 CSS 土堆/嫩芽绘制；保留 wrapper 分隔线、尺寸、投影和移动端容器查询。
- 未把原型图作为运行时背景或整块截图素材；只引用独立透明小图标。按钮继续复用 `sd-btn-green` / `sd-btn-tan` PNG 按钮体系，页面纸卡/边框/进度/日志终端仍为 CSS 实现。

## 影响文件/接口
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/public/assets/stardew/ui/install/*.png`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端接口、请求路径、安装提交、Steam Guard / QR、SSE 日志、权限判断、loading/error/empty/disabled 逻辑。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 使用已删除的临时 `frontend/install-qa.html` + `frontend/src/install-qa.tsx` 挂载真实 `InstallPage`、真实 CSS 和真实 public 素材做内置浏览器 QA。
- 1280x900 认证态：顶部土芽自然尺寸 `112x92`、五步图标自然尺寸 `72x72`，运行时均加载自 `/assets/stardew/ui/install/`；无页面级横向溢出；按钮文字不溢出；console error/warn 为空。
- 未安装态：点击“安装游戏”后表单展开，`JunimoServer 镜像版本 / Steam 用户名 / Steam 密码 / VNC 密码` 字段可见，按钮无文字溢出，console error/warn 为空。
- 390x760：真实容器查询下五步条纵向排列，顶部土芽保持小图标尺寸，页面 `scrollWidth=clientWidth`，按钮无文字溢出，console error/warn 为空。

## 下一步注意事项
- 这批素材来自 image2 原型的局部小图标，不是整页背景；后续不要把 `08-install - 副本.png` 直接接入运行时。
- 安装页移动端规则依赖 `.sd-main-scroll` 容器查询；做隔离 QA 时需要包同名 container，否则会误判为移动端断点未生效。
- 若后续替换图标，保持 `ui/install/` 下文件名稳定，避免改页面组件逻辑。

# FE-SERVER-PROTOTYPE-IMAGE2-1 前端接手记录（2026-07-03）

## 改了什么
- 服务器控制页按 image2 原型 `02-server-control - 副本.png` 重皮肤：顶部大标题、当前状态大卡、生命周期控制卡、邀请代码、全服消息、控制台命令终端、快捷操作条都调整为原型的羊皮纸 + 像素按钮风格。
- 原型图未作为运行时资源；背景噪点、纸卡、铜色边框、内阴影、分隔线、绿屏邀请码、黑色终端、快捷操作按钮纹理均由 CSS 实现。
- `ServerControlPage.tsx` 只改视觉结构和展示层：状态信息字段化，命令执行结果显示到终端区域，全服消息显示 `0/120` 计数，快捷操作按原型排成按钮条。生命周期、邀请码、喊话、命令、备份、VNC、计划重启的 API 调用、权限判断、loading/error/disabled 状态保持原逻辑。
- 按钮/图标复用既有 Stardew 素材：`sd-btn-start/stop/restart/green/tan/delete`、`icon_button_*`、服务器/玩家/存档/时间/诊断等 `ui/icons`，状态点继续使用 `.sd-dot*`。
- 响应式补强：`.sd-server-page` 增加容器查询断点；主内容变窄时自动单列，消息输入、命令输入、终端和快捷操作不会横向撑出；服务器页输入框局部使用 `box-sizing: border-box` 修复窄屏裁切。

## 影响文件/接口
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端接口、路由、权限、数据轮询或素材文件。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 真实路由当前受登录态影响不适合直接进入，因此使用临时 `frontend/server-control-qa.html` 挂载真实 `ServerControlPage` 组件和 mock 数据做浏览器 QA；QA 文件已删除。
- 1280x900 桌面：页面双列布局，状态卡/生命周期卡/邀请代码/全服消息/命令终端/快捷操作可见，无横向溢出，按钮无文字溢出；点击“执行”后终端显示命令输出。
- 390x760 窄屏：页面单列，消息输入、命令输入、终端、快捷操作按容器宽度收缩，无横向溢出，按钮无文字溢出。

## 下一步注意事项
- 本页视觉覆盖集中在 `StardewPanel.css` 的 `FE-SERVER-PROTOTYPE-IMAGE2-1` 段，并以 `.sd-server-page` 为作用域；后续不要把纸卡/终端规则提升为全局样式。
- QA 页只是验证挂载工具，不能入库长期保留；如需要再次验证，重新临时创建后用完删除。
- 如果真实数据里的命令名称或存档名更长，优先检查 `.sd-server-terminal pre`、`.sd-state-value`、`.sd-server-quick-grid` 的换行/截断，不要放宽到横向滚动。

# FE-DIAGNOSTICS-IMAGE2-ICONS-1 前端接手记录（2026-07-03）

## 改了什么
- 用户反馈诊断页首轮 CSS 自绘图标质感太差后，重新用内置 Image Gen 按 `07-diagnostics - 副本.png` 的 image2 像素 UI 风格生成图标素材，并替换诊断页里刚才那批 CSS 图标。
- 新增 `frontend/public/assets/stardew/ui/diagnostics/`：包含透明 sprite sheet `diag_icon_sheet_image2.png` 和 20 个透明 PNG 单图（状态盾牌、三色宝石、Docker/Compose/目录/文件/启动存档、建议叶子/灯泡/嫩芽/警告/错误、资源趋势镐子、实时绿点、导出下载）。
- `StardewPanel.css` 通过诊断页作用域覆盖，把 `.sd-diag-status-shield`、`.sd-diag-count-gem`、`.sd-diag-check-icon-*`、`.sd-diag-advice-icon`、资源趋势标题图标、实时徽章图标和导出按钮图标改为 PNG 背景图；同时关闭初版 CSS 图标的 `clip-path`、渐变和伪元素残留。
- 生成后处理：从 Codex 默认生成目录复制结果，洋红 chroma-key 阈值转 alpha，切片后检查所有 PNG 四角透明；清掉顶部孤立碎片并重新裁切透明边，避免图标上方脏点。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/public/assets/stardew/ui/diagnostics/*.png`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改 `DiagnosticsPage.tsx`、未改任何 API、权限、loading/error/disabled 逻辑。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 已使用已删除的临时 `frontend/diagnostics-icons-qa.html` 加载真实 CSS 和 public 素材做内置浏览器 QA。
- 1280x900：新盾牌、宝石、检查项图标、资源标题图标、实时绿点、导出按钮图标和建议区图标均可见；无横向溢出；按钮无文字溢出；console error/warn 为空。
- 390x760：移动端无横向溢出，按钮无文字溢出，图标按单列布局正常显示。
- 浏览器计算样式检查：可见诊断页图标背景均来自 `/assets/stardew/ui/diagnostics/`。

## 下一步注意事项
- 这批素材是基于一张 4x5 sprite sheet 切片而来，单图命名稳定；如果后续要替换其中一枚，优先保持同目录和文件名，避免改 CSS。
- `diag_icon_sheet_image2.png` 保留为来源 sheet，运行时使用单图；不要把整张 sheet 当页面背景。
- `.sd-btn-blue` 当前只有导出按钮用了下载图标；如果后续统一蓝色按钮体系，可把该图标规则沉淀到主题按钮样式。

# FE-DIAGNOSTICS-PROTOTYPE-IMAGE2-1 前端接手记录（2026-07-03）

## 改了什么
- 诊断与健康页按 image2 原型 `07-diagnostics - 副本.png` 做视觉重皮肤：顶部标题/双按钮、系统状态横卡、三张计数卡、检查项表、资源趋势卡、底部告警建议区对齐原型布局。
- 原型图没有作为运行时背景或整块素材引用。羊皮纸底、噪点、面板边框、内描边、虚线分隔、资源仪表盘由 CSS 实现；盾牌、宝石、检查项图标和建议图标已在后续 `FE-DIAGNOSTICS-IMAGE2-ICONS-1` 中替换为 image2 风格 PNG；趋势折线继续使用现有 SVG 数据绘制。
- `DiagnosticsPage.tsx` 只改视觉 DOM 外壳：新增 `CountCard`、检查项图标列、检查表头、资源卡头部和全宽建议面板。`getHealthDiagnostics()`、`downloadSupportBundle()`、`getInstanceMetrics()`、管理员导出权限、loading/error/disabled 状态和 metrics 轮询未改。
- 按钮/图标复用：页头图标改用现有 `icon_nav_diagnostics_monitor_image2.png`；重新检查按钮复用绿色 PNG 按钮体系；导出诊断包使用新增 `.sd-btn-blue` CSS 变体，未新增按钮图片。

## 影响文件/接口
- `frontend/src/games/stardew/pages/DiagnosticsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端接口、路由、API 参数、权限判断或数据结构。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 已启动 Vite 本地服务，使用已删除的临时 `frontend/diagnostics-qa.html` 加载真实 CSS、public 素材和同结构 DOM 做内置浏览器 QA。
- 1280x900：页面非空，标题/双按钮/状态横卡/左右面板/建议区渲染正常；按钮文字和检查项名称不溢出；主要面板无重叠；无横向溢出；console error/warn 为空；点击“重新检查”会进入“检查中...”禁用态。
- 390x760：按钮、状态卡、检查表、资源卡、建议区按单列收缩；主要面板宽度均在页面内；无横向溢出；console error/warn 为空。
- 已用 `view_image` 查看原型图、桌面实现截图和移动实现截图。

## 下一步注意事项
- 真实页面仍依赖登录态和后端数据；本次 QA 用同结构 DOM 验证视觉，未触发真实支持包下载，真实下载权限仍由原按钮的 `isAdmin`/`downloadSupportBundle()` 控制。
- 如果后续检查项名称增加到更长文本，优先调整 `.sd-diag-check-row` 的网格列或允许信息列换行，不要放宽到页面横向溢出。
- `.sd-btn-blue` 当前是诊断页新增的通用蓝色 CSS 按钮类；若其他页面也要使用，先确认是否需要沉淀到主题按钮体系。

# FE-SETTINGS-PROTOTYPE-IMAGE2-1 前端接手记录（2026-07-03）

## 改了什么
- 设置与审计页按 image2 原型 `09-settings - 副本.png` 做视觉重皮肤：顶部原为三卡（当前账号 / 面板版本 / 安全与权限），现已按 `FE-SETTINGS-ACCOUNT-CARD-REMOVE-1` 移除“当前账号”卡，保留面板版本 / 安全与权限两卡；中部双栏（用户管理 / 审计日志）、底部双栏（端口信息 + 其他设置 / 安全建议）。
- 没有把原型图作为运行时背景或整块素材。纸纹、铜色边框、四角角钉、内描边、表格表头、行分隔线、底部提示面板均为 `.sd-settings-page` scoped CSS 实现。
- `SettingsPage.tsx` 只加视觉分组外壳和区块 modifier class；用户管理、审计日志、VNC 端口、登出、确认弹窗和权限判断逻辑未改。
- 新增 `SecuritySummarySection` 用于顶部安全摘要；原安全说明保留为底部“安全建议”。设置页头图标切换为已有 `icon_nav_settings_gear_image2.png`。
- 按钮继续复用现有 PNG 按钮类 `sd-btn-green` / `sd-btn-tan` / `sd-btn-delete`；标题图标复用既有 image2 导航/顶栏素材，无新增图片资源。

## 影响文件/接口
- `frontend/src/games/stardew/pages/SettingsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端接口、API 调用、权限、路由或数据轮询。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 已启动临时 Vite 服务并用内置浏览器验证；真实 `/instances/stardew/settings` 当前停在登录页，登录页 console error/warn 为空。
- 使用已删除的临时 `settings-qa.html` 加载同一份 CSS/素材/同结构 DOM 做视觉 QA：
  - 1280x900：顶部三卡、用户/审计双栏、底部双栏比例对齐原型；无横向溢出；按钮无文字溢出；console error/warn 为空。
  - 点击“新建用户”：表单展开，输入框/禁用创建按钮可读，无横向溢出。
  - 390x760：页面单列；审计表只在自身容器内横向滚动；底部待接入/禁用按钮可读；页面无横向溢出。
- 已用 `view_image` 对比原型图和最终桌面截图，确认未使用原型整图作为页面资源。

## 下一步注意事项
- 设置页新增样式都在 `.sd-settings-page` 作用域内；后续不要把这些纸卡/表格规则提升为全局规则，避免影响服务器、模组、存档页。
- 如果真实数据里用户名、IP 或审计目标特别长，优先检查用户行和审计表的截断/横向滚动，不要放宽到页面级横向溢出。
- 真实路由仍需登录态才能直达设置页；本次视觉 QA 使用临时同结构页面，功能逻辑由 TypeScript 构建和既有代码路径兜底。

# FE-INSTALL-PROTOTYPE-IMAGE2-1 前端接手记录（2026-07-03）

## 改了什么
- 安装页按 image2 原型 `08-install - 副本.png` 重皮肤：顶部状态横幅、五步安装时间线、底部“安装配置 / Steam 认证 / 安装日志”三栏工作区，整体接近原型的羊皮纸 + 像素农场 + 深色日志终端。
- 未把原型图作为运行时背景或整块素材。纸张底、噪点、描边、分隔线、步骤卡、进度条、配置栏占位、认证卡和日志终端均由 CSS 实现。
- `InstallPage.tsx` 只调整页面级 DOM 外壳和派生展示值：新增总进度百分比、步骤图标 class、认证中配置栏占位；安装提交、Steam Guard / QR 交互、SSE 日志、权限判断、API 调用和导航逻辑不变。
- 顶部农场横幅复用既有 `sprite_farmhouse_scene.png` 小素材并用 CSS 遮罩融合；按钮继续复用 `sd-btn-green` / `sd-btn-tan` PNG 按钮体系。

## 影响文件/接口
- `frontend/src/games/stardew/pages/InstallPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改任何后端接口、请求路径、权限判断或数据结构。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 真实 `/instances/stardew/install` 当前被登录页拦住，因此使用已删除的临时 `frontend/install-qa.html` + `src/install-qa.tsx` 挂载真实 `InstallPage`、真实 CSS 和 mock props 验证。
- 1280x900 认证态：顶部状态横幅、步骤 3 高亮、三栏工作区、Steam 登录方式、日志空状态可见；console error/warn 为空。
- 未安装态：点击“安装游戏”后表单正常展开，能看到 Steam 用户名、Steam 密码、VNC 密码字段；console error/warn 为空。
- 390x760：页面纵向折叠，无横向溢出（`scrollWidth=clientWidth=390`），按钮/长英文 `Stardew Valley` 未撑破容器。
- 已用 `view_image` 对比原型图、桌面实现截图和移动实现截图。

## 下一步注意事项
- CSS 中安装页新规则集中在 `StardewPanel.css` 文件末尾，分为 scoped override 与 structural rules 两段，用 `.sd-install-page` 和 `.sd-install-*` 限定安装页；后续整理时不要降低页面级作用域，避免影响存档/总览/设置页。
- 顶部横幅只复用既有小农场素材，不是原型右侧完整农场图；如果后续要更接近原图，应生成独立“空农场横幅”生产素材，不要引用整页原型图。
- 认证中配置栏现在显示“配置已提交”占位，避免空栏；如果后端以后能返回已选镜像/账号摘要，可在该占位里展示脱敏信息。

# FE-OPSRAIL-AUTO-COLLAPSE-1 前端接手记录（2026-07-03）

## 改了什么
- 修复窗口缩放时右侧 OpsRail 挤压中间内容的问题：`StardewPanel` 现在按展开右栏时的预计主内容宽度自动决定是否收起右栏。
- 收起阈值：展开态主内容 `<820px` 时收起；已经收起后需回到 `>880px` 才展开，避免拖动窗口时抖动。
- CSS 层新增 `.sd-shell--opsrail-auto-collapsed`，把 shell 第三列设为 `0` 并隐藏 `.sd-opsrail`；左/右栏列宽抽成 `--sd-sidebar-width`、`--sd-opsrail-width`。
- 总览页内部断点改进：`.sd-main-scroll` 开启 container query，总览页 1180px 的控制区/邀请码/摘要卡换行规则现在按主内容实际宽度触发，避免 1200px 视口下主内容已变窄但页面仍按桌面三列排布。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端接口、权限、路由、数据轮询或右栏卡片内容。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 真实 `/instances/stardew/overview` 当前停在登录页，因此用已删除的临时 `opsrail-collapse-qa.html` 挂载真实 `StardewPanel` 组件做布局 QA。
- 1200x760：右栏自动收起，`.sd-shell--opsrail-auto-collapsed=true`，主内容宽 `959px`，总览页控制区/邀请码区按容器宽度换行，无横向溢出。
- 1200x760 不刷新 resize 到 1600x860：右栏自动展开，右栏宽 `430px`，主内容宽 `887px`，无横向溢出。
- 390x760：移动端仍为单列 shell + 横向导航，右栏隐藏，无横向溢出。
- 三个验证点 console error/warn 为空。

## 下一步注意事项
- 后续若用户觉得右栏收起太早或太晚，优先一起复核 `StardewPanel.tsx` 顶部 `OPS_RAIL_COLLAPSE_MAIN_WIDTH` / `OPS_RAIL_EXPAND_MAIN_WIDTH`、`SHELL_MIN_UI_SCALE`，以及 `StardewPanel.css` 里的 `--sd-min-ui-scale` / 右栏隐藏 `@media` 阈值。
- 如果其他页面也出现“主内容已变窄但视口媒体查询没触发”的问题，优先在 `.sd-main-scroll` 容器查询下补页面级 `@container` 规则，不要继续只堆 viewport `@media`。

# FE-OVERVIEW-PROTOTYPE-IMAGE2-1 前端接手记录（2026-07-03）

## 改了什么
- 总览页按 image2 原型 `01-overview - 副本.png` 做视觉重皮肤：农场横幅、服务器控制条、邀请码绿屏、四张摘要卡和下方三列清单更接近原型的像素农场 + 羊皮纸仪表盘。
- 未把原型整图作为背景或运行时资源。横幅只复用既有 `sprite_farmhouse_scene.png` 小素材，天空/云/田地/远山/暗角用 CSS 背景层实现；纸纹、边框、内阴影、分隔线和状态标签全部 CSS 实现。
- `OverviewPage.tsx` 只新增必要视觉外壳：`.sd-lifecycle-actions`、`.sd-invite-panel`、标题图标、摘要状态标签、玩家行式结构。启动/停止/重启/复制/刷新/跳转等 handler 未改，API 与权限判断未改。
- 按钮/图标复用：生命周期按钮仍用既有 `sd-btn-start/stop/restart` PNG 底图与 `icon_button_*`；复制/刷新/管理模组/查看诊断继续复用既有按钮体系；标题与摘要图标复用 `ui/icons` 下现有 image2 图标。
- 响应式补强：1180px 以下布局收成单列/双列，640px 以下横幅压缩、按钮和邀请码允许换行；`.sd-ov-wrap` 显式隐藏横向溢出。

## 影响文件/接口
- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端接口，未新增素材，未改路由或数据层。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 内置浏览器真实应用访问 `/instances/stardew/overview` 被当前登录页拦住，因此使用临时 `frontend/overview-qa.html` 加载同一份 CSS、真实素材和同结构 DOM 做视觉 QA；QA 文件已删除。
- QA 结果：1280x900 桌面无横向溢出、console error/warn 为空；390x760 窄屏无横向溢出，按钮文字与邀请码不溢出。已用 `view_image` 查看原型、桌面实现截图、移动实现截图。

## 下一步注意事项
- 当前横幅没有使用原型整图，只是既有小农场图 + CSS 田野，因此与原型大场景不是逐像素一致；如果后续要更接近原型，应生成/拆分一张独立“空农场横幅”生产素材（不含页面 UI 文案和数据），不要直接引用整页截图。
- 总览页样式都以 `.sd-ov-*` 为主，不要把新纸卡规则提升为全局卡片规则，否则会影响存档/模组/设置页。
- 真实登录态下若总览页数据文案比 QA 样例更长，优先检查 `.sd-mc-sub`、`.sd-invite-code`、`.sd-ov-player-name` 的截断行为，避免放宽到横向溢出。

# FE-SAVES-SIMPLIFY-3 前端接手记录（2026-07-03）

## 改了什么
- 存档页页头从带框 `.sd-page-header`（图标 + 标题 + 描述）精简为左上角 `.sd-saves-page-title`（小图标 + "存档管理"），无框无描述。
- 撤销自绘 `.sd-pxbtn` 按钮体系（FE-SAVES-MOCKUP-2 引入，CSS 已删、TSX 无残留），全部换回既有 PNG 素材按钮：`sd-btn-green` / `sd-btn-delete` / `sd-btn-tan`，与其他页面视觉一致。
- 备份与恢复：自动备份规则压缩为单行控件条（标题 + 游戏保存后更新最新备份 + 每天 HH:00 定时备份 + 快照保留天数滑条 + 保存设置），两行式说明文字移入 `title` 悬浮提示；多处文案精简。
- 整页上提：`.sd-saves-page { padding-top: 0 }`，内容贴住 `.sd-main` 外框内沿；外框安全边界（viewport inset）本身未动，其他页面不受影响。

## 影响文件/素材
- `frontend/src/games/stardew/pages/SavesPage.tsx`、`frontend/src/games/stardew/SavesSection.tsx`、`frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`、`docs/08-future-roadmap.md`

## 如何验证
- 已执行：`cd frontend; npm run build` 通过；`grep pxbtn` 无残留。
- 手动：页头左上角纯文字；按钮与服务器/模组页同款 PNG 素材；备份规则单行、悬浮提示可见、保存设置生效。

## 下一步注意事项
- 若以后需要设计稿里的大号"创建并启动/上传并启动"按钮观感，应新增对应 PNG 素材接入 `stardew-theme.css` 的 PNG 按钮体系，不要再回到自绘 CSS 按钮。
- 备份规则的解释文案在各 label 的 `title` 里，改文案时同步维护。

# FE-SAVES-MOCKUP-2 前端接手记录（2026-07-03）

## 改了什么
- 存档页按用户完整设计稿改版（在 FE-SAVES-PROTO-CSS-1 基础上）：眉标行、激活卡图标字段双列表、存档库只显示非激活存档（每卡：缩略图/农场名/进度/类型·大小 + 选择/删除）、横排虚线创建卡（创建并启动）、上传横条容器（📮 + 文案 + 上传并启动）、备份真表格（六列 + 状态徽章 + 5 行折叠"查看更多备份"）。
- 新增复用按钮体系 `.sd-pxbtn`（`-green/-red/-blue/-lg/-sm`），纯 CSS 糖块像素按钮。
- 功能位移：库卡的"选择并启动/导出/手动备份"收敛到激活卡操作行（使用此存档启动/导出/手动备份）；库卡"选择"= `handleSelect`（设为启动存档）。所有 handler 未改。
- 备份状态徽章：`parseError`→红"解析失败"，同名存档存在→黄"同名冲突"，否则绿"正常"；年份/地图等细节在行 title 悬浮提示里。

## 影响文件/素材
- `frontend/src/games/stardew/SavesSection.tsx`、`frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`、`docs/08-future-roadmap.md`

## 如何验证
- 已执行：`cd frontend; npm run build` 通过。
- 手动：对照设计稿看五块区域；验证选择→激活卡更新、删除确认、备份恢复/彻底删除、创建/上传弹窗、>5 条备份时折叠展开。

## 下一步注意事项
- 想给非激活存档直接"导出/备份"需先"选择"为激活存档再操作；如果用户反馈不便，可在库卡加次级小按钮行。
- 备份表格行最小宽 660px，窄屏时容器横向滚动（`.sd-save-backups-table` overflow-x）；移动端如需卡片式布局另开任务。
- 旧类 `.sd-save-backup-main/-meta/-conflict`、`.sd-saves-header*`、`.sd-saves-active-eyebrow` 的 DOM 已移除，基础 CSS 仍留在文件中（未清理，避免影响并行会话），后续统一清理时注意。

# FE-SAVES-PROTO-CSS-1 前端接手记录（2026-07-03）

## 改了什么
- 存档页按 image2 原型（`03-saves-page-frame-clean-image2-no-buttons-icons-thumbnails.png`）重做视觉，纯 CSS、零图片资源：噪点羊皮纸、铜框铆钉激活卡（radial-gradient 铆钉）、双列虚线字段、圆角铜边存档卡、虚线"创建新存档"卡、像素云天空上传横条（多层白色矩形 background 层叠）、备份表头带 + 行分隔线。
- DOM 调整：页头创建/上传按钮分别移入网格末尾的虚线创建卡和列表下方的天空横条按钮（均仅管理员、运行中禁用）；激活卡左侧预览槽不再引用 `sprite_farmhouse_scene.png`。
- 预览槽后续按用户要求接入真实农场地图：按激活存档 `farmType` 显示 `/assets/stardew/new-game/farms/<farmType>.png`（新建游戏界面同款素材），仅当 `farmTypeLabel` 认识该类型才渲染 img（防未知值 404），未知时回落为空羊皮纸块。
- 业务逻辑零改动；新样式全部以 `.sd-saves-page` 作用域追加在 `StardewPanel.css` 末尾。

## 影响文件/素材
- `frontend/src/games/stardew/SavesSection.tsx`、`frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`、`docs/08-future-roadmap.md`

## 如何验证
- 已执行：`cd frontend; npm run build` 通过。
- 手动：存档页对照原型看五块区域；确认创建卡/上传条在运行中禁用、非管理员不可见；空状态时无天空横条但保留原创建/上传按钮。

## 下一步注意事项
- 天空横条的云朵矩形用百分比定位，条宽变化时云会横向重新分布，属预期；若想要严格像素画云可改为 box-shadow 像素块方案。
- 移动端媒体查询（`.sd-saves-active-card` 窄屏单列）只调布局，与新皮肤（只动颜色/边框/圆角）不冲突；后续改布局时注意 `.sd-saves-page` 前缀的规则特异性更高。
- 备份区内层元素靠 `.sd-save-backups-section > *:not(.sd-save-backups-header)` 的 margin 对齐；往该 section 新增直接子元素时会自动获得左右 12px 边距。

# FE-RIGHT-RAIL-ACTIVE-CARD-1 前端接手记录（2026-07-03）

## 改了什么
- 右栏"进行中"卡改为三类内容：自动关机/自动开机倒计时（restart-schedule 的 `nextShutdownAt`/`nextStartupAt`）、定时备份倒计时（backups/policy 的 `scheduledHour`，仅管理员可见）、运行中任务进度条。
- 新组件 `OpsRailActiveCard`（`StardewPanel.tsx` 内）：内部 1s tick 只重渲染本卡；restart-schedule 与备份策略每 60s 拉取一次，接口失败（含普通用户读备份策略 403）时静默隐藏对应行。
- 任务进度算法：同类型最近成功任务耗时中位数为预期时长（无历史默认 60s），进度 = 已运行/预期封顶 95%；queued 显示"排队中"。倒计时进度条按 24h 周期已过比例填充。
- 行样式复用 `.sd-opsrail-hstat*`，新增蓝色 `--info` 档用于倒计时行。
- `useStardewDashboardData` 的 30s 轮询增加 `refreshJobs()`，兜底调度器后台触发的 job。

## 影响文件/素材
- `frontend/src/games/stardew/StardewPanel.tsx`、`frontend/src/games/stardew/StardewPanel.css`、`frontend/src/games/stardew/useStardewDashboardData.ts`
- `docs/03-frontend.md`、`docs/08-future-roadmap.md`

## 如何验证
- 已执行：`cd frontend; npm run build` 通过。
- 手动：服务器控制页开启计划重启后右栏出现"自动关机/自动开机"倒计时并每秒走动；存档页开启定时备份后出现"定时备份"行；触发备份/重启等任务时出现进度行，完成后消失。

## 下一步注意事项
- 定时备份的下次触发时间是"面板浏览器本地时间的每日 scheduledHour 整点"近似，后端实际按服务器本地时间判断；两端时区不一致时会有偏差，如需精确可让后端在 policy 响应里返回 `nextScheduledAt`。
- 任务进度是估算值（封顶 95%），不是后端真实进度；如果以后 job 支持进度上报，替换 `runningJobPercent()` 即可。
- 调度器触发的 job 最多延迟 30s 出现（依赖 jobs 轮询周期）。

# FE-RIGHT-RAIL-HEALTH-STATS-1 前端接手记录（2026-07-03）

## 改了什么
- 右栏"系统健康"卡按原型改为五行资源统计：CPU 使用率、内存使用率、磁盘使用率（带进度条）、在线玩家、网络延迟；按钮文案从"查看诊断 →"改为"查看详情 →"（仍跳诊断页）。
- `StardewPanel` 新增 metrics 轮询（每 5s 调 `getInstanceMetrics()`，与诊断页同接口）；网络延迟取该请求的前端往返耗时（`performance.now()` 差值取整），无独立后端接口。
- 在线玩家显示 `onlineCount/maxPlayers`；配合后端 `ListPlayers` 新增的 `server-settings.json` `Server.MaxPlayers` 兜底（PLAYERS-MAXPLAYERS-1），未运行时也能显示 `0/12` 这类"在线/当前存档人数上限"。
- 进度条按用户要求做成圆润二次元风：13px 胶囊形轨道（`border-radius: 999px` + 2px 内边距）+ 亮绿渐变糖果填充 + 顶部白高光；样式类为 `.sd-opsrail-hstat-*`。
- 阈值配色（用户后续要求）：每行按数值加 `--ok/--warn/--crit` 修饰类，浆果点/进度条/数值同步变色。使用率 `<60` 绿 / `≥60` 黄 / `≥85` 红；延迟 `<100ms` 绿 / `≥100` 黄 / `≥300` 红；在线玩家为 `0` 时红。阈值在 `StardewPanel.tsx` 的 `usageLevel()` / `latencyLevel()` 里，调整时改这两个函数即可。
- 移除原健康检查摘要行及其 `healthSummaryDot()`、`.sd-opsrail-health-summary`。

## 影响文件/素材
- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`、`docs/08-future-roadmap.md`

## 如何验证
- 已执行：`cd frontend; npm run build` 通过。
- 手动：总览页右栏确认五行数据随轮询更新、进度条圆角与高光正常；停止服务器时 CPU/内存显示 `—`、在线玩家显示 `0/上限`。

## 下一步注意事项
- 网络延迟是面板 API 往返耗时的近似，不是游戏服务器到玩家的延迟；如果以后后端提供真实延迟接口，替换 `apiLatencyMs` 来源即可。
- 健康检查（全部正常/N 错误）已不在右栏展示，只在总览页摘要格和诊断页；如果用户反馈缺失可考虑加回一行。

# FE-MAIN-PAGE-FRAME-3 前端接手记录（2026-07-03）

## 改了什么
- 按用户红框示意重新界定所有 Stardew 页面中间内容滚动视口的四边边界。
- `.sd-main` 的内侧 padding 从原先较小的上下/左右两组 inset，改为四向比例 inset：top `clamp(22px, 5.2%, 60px)`、right `clamp(20px, 5%, 58px)`、bottom `clamp(26px, 6%, 72px)`、left `clamp(18px, 4%, 50px)`。
- `.sd-main-scroll` 仍是唯一滚动视口，所有页面继续通过 `StardewPanel.tsx` 的统一 wrapper 生效；本次只改 frame 内侧边界大小，没有改各页面业务 DOM。

## 影响文件/素材
- `frontend/src/games/stardew/StardewPanel.css`
- 无新增素材、接口或后端变更。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 内置浏览器临时 QA 页使用生产 CSS 和 public 素材验证：
  - 1750x1113 视口下，中间主内容区为 `1068x1033`，`.sd-main-scroll` 相对 `.sd-main` 偏移为 top `55.5px`、right `53.4px`、bottom `64.1px`、left `42.7px`，比例为 `5.2% / 5% / 6% / 4%`，对应用户红框。
  - 桌面滚轮后 `.sd-main-scroll.scrollTop=720`，`.sd-main.scrollTop=0`，说明滚动只发生在红框内层。
  - 390x760 下 inset 为 top/right/bottom/left `22/20/26/18px`，滚轮后 `.sd-main-scroll.scrollTop=620`，无横向溢出。
  - 两个视口 console error/warn 为空。

## 下一步注意事项
- 后续若用户继续微调红框边界，只改 `.sd-main` 的四个 `--sd-main-viewport-inset-*` 变量；不要改回 `.sd-main > .sd-page` 滚动，也不要给单个页面单独裁切，否则各页面会不一致。

# FE-MAIN-PAGE-FRAME-2 前端接手记录（2026-07-03）

## 改了什么
- 修复中间主内容区换成 image2 frame 后，模组页无法滚动的问题。
- `StardewPanel.tsx` 中 `.sd-main` 内新增 `.sd-main-scroll` 包装层：外层 `.sd-main` 继续负责上一步界定的 frame 内侧边界、背景和 `overflow:hidden` 裁切；`.sd-main-scroll` 只在这个内侧边界内滚动。
- `StardewPanel.css` 将滚动/隐藏滚动条规则从 `.sd-main > .sd-page` 改到 `.sd-main-scroll`，并让 `.sd-main-scroll > .sd-page` 仅保持最小高度填满。各页面仍返回普通 `.sd-page`，避免模组页内部布局被滚动容器规则破坏。

## 影响文件/素材
- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 无新增接口或后端变更。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 已用生产构建 CSS + 临时 Shell QA 页在内置浏览器验证：
  - 1280x720：`.sd-main` 为 `overflow:hidden`，`.sd-main-scroll` 为 `overflow-y:auto`，滚轮后 `.sd-main-scroll.scrollTop` 从 `0` 到 `720`，`.sd-main.scrollTop` 保持 `0`。
  - 390x760：滚轮后 `.sd-main-scroll.scrollTop` 从 `0` 到 `620`，无横向溢出。
  - 两个视口均确认背景为 `main_page_frame_empty_image2.png`、滚动条隐藏、console error/warn 为空。

## 下一步注意事项
- 后续所有 Stardew 路由页面仍应返回 `.sd-page`，但不要再依赖 `.sd-main > .sd-page` 作为滚动容器；统一滚动视口是 `.sd-main-scroll`。
- 如果后续调整 frame 边界，优先改 `.sd-main` 的 `--sd-main-viewport-inset-*`，不要把滚动重新放回 `.sd-main` 或单个页面组件，否则内容会重新压到木框或导致复杂页面滚动失效。

# FE-MAIN-PAGE-FRAME-1 前端接手记录（2026-07-03）

> 注意：本条中的 `.sd-main > .sd-page` 滚动容器方案已被上方 `FE-MAIN-PAGE-FRAME-2` 替代；当前滚动视口统一为 `.sd-main-scroll`，`.sd-main` 继续负责上一步界定的 frame 内侧边界和裁切。

## 改了什么
- 按用户要求把 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/03-saves-page-frame-empty-image2.png` 接入为所有 Stardew 页面中间内容区的统一背景。
- 原型图复制到生产 public 素材：`frontend/public/assets/stardew/ui/panels/main_page_frame_empty_image2.png`，避免运行时直接依赖 `docs/` 路径。
- `stardew-theme.css` 新增 `--sd-img-page-frame`，`.sd-main` 从旧羊皮纸平铺改为整张 frame 背景：居中、不重复、`100% 100%` 铺满主内容区，并保持 `image-rendering: pixelated`。
- `--sd-page-padding` 调整为 `clamp(28px, 2.4vw, 42px)`，给 frame 的木框和角饰留出安全边距。
- `.sd-main` 继续使用 `overflow-y: auto` 承载中间页滚动，但隐藏浏览器原生滚动条：Firefox 走 `scrollbar-width: none`，旧 Edge/IE 走 `-ms-overflow-style: none`，Chromium/WebKit 走 `.sd-main::-webkit-scrollbar { display: none; }`。这样滚轮/触控板滚动仍可用，右侧不会再出现白色滚动条压住 frame 边框。
- 根据用户红线示意，滚动裁切改为内侧视窗：`.sd-main` 不再滚动，改为 `overflow: hidden`、固定 frame 背景和内侧 padding；`.sd-main > .sd-page` 才是实际滚动容器。桌面内侧裁切边界约为 top/left `15/14px`，移动端约 `12/10px`。内容多时会在这个内框边界被遮掉，滚动后再显示，避免模组页等长页面压到上边框或木质角饰上。

## 影响文件/素材
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/src/games/stardew/stardew-theme.css`
- `frontend/public/assets/stardew/ui/panels/main_page_frame_empty_image2.png`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 已用生产构建 CSS + 临时 Shell QA 页在内置浏览器验证：
  - 1280x720：`.sd-main` 背景为 `main_page_frame_empty_image2.png`，`background-size: 100% 100%`，页面内容不压边框，无横向溢出。
  - 390x760：移动断点下右栏隐藏，主内容区继续显示同一 frame 背景，无横向溢出。
  - 滚动条隐藏验证：桌面 QA 页 `.sd-main` 的 `scrollbar-width` 为 `none`、`::-webkit-scrollbar` display 为 `none`，滚轮滚动后 `scrollTop` 从 `0` 变为 `620`，证明滚动能力保留。
  - 边界裁切验证：更新后的 QA 页 `.sd-main` 为 `overflow:hidden`，`.sd-main > .sd-page` 为 `overflow-y:auto`，桌面 inner viewport 相对 main 偏移约 `15px/14px`；滚轮后 `.sd-page.scrollTop=650`，内容在顶部和底部 frame 内边界处被裁切。390x760 下 inner viewport 偏移约 `12px/10px`，无横向溢出。
  - 两个视口 console error/warn 均为空。
- 真实 `/instances/stardew/saves` 当前本地浏览器未登录，会停在登录页；因此业务态截图使用临时 Shell QA 页验证同一份生产 CSS 和 public 素材路径。

## 下一步注意事项
- 这次只替换中间主内容区背景，没有替换内部卡片、表格、上传条、当前存档等二级内容框；后续若继续替换这些部件，应按页面内组件逐个接入，避免把带文字/数据的整页截图烘焙进背景。
- 若更换 `main_page_frame_empty_image2.png`，需要重新检查 frame 边框厚度，并同步调整 `--sd-page-padding`，否则标题或卡片可能压到角饰。
- 所有 Stardew 路由页面应继续让最外层返回 `.sd-page`，且作为 `.sd-main` 的直接子节点；如果未来某个路由返回额外 wrapper，需要同步让 wrapper 承担 `.sd-main > .sd-page` 的滚动视窗规则，否则内容会回到外框裁切边界之外。

# FE-RIGHT-RAIL-CARD-FIX-1 前端接手记录（2026-07-03）

## 改了什么
- 右栏三卡去掉滚动/拉伸逻辑：`.sd-opsrail-stack` 由 `repeat(3, minmax(140px, 1fr))` 等高拉伸 + `overflow-y: auto` 改为 `grid-auto-rows: min-content` + `align-content: start` + `overflow: hidden`；行高随内容，与左侧栏按钮一致只随栏宽缩放。
- `.sd-opsrail-list` 去掉 `overflow: auto` 与滚动条隐藏；`.sd-opsrail-recent-list` 规则删除并从 `StardewPanel.tsx` 移除类名。
- 修复四角藤蔓拉伸：旧 border-width 换算按"每边可见框厚 13px"，导致角部切片横纵缩放比不一致（进行中卡纵向拉长约 1.7 倍）。改为每卡单一系数 `s = 13 / (左切片 − 左透明边)`，`border-width = slice × s`、负 `margin = 透明边 × s`。新值见 `docs/03-frontend.md` 对应条目。

## 影响文件/素材
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/src/games/stardew/StardewPanel.tsx`
- `docs/03-frontend.md`、`docs/08-future-roadmap.md`

## 如何验证
- 已执行：`cd frontend; npm run build` 通过。
- 手动：登录后看总览页右栏，缩放窗口高度确认三卡不再滚动/拉伸，四角藤蔓比例正常。

## 下一步注意事项
- 若以后再调三卡的 slice/border-width/margin，必须保持"四边共用一个缩放系数"，否则角部藤蔓会再次变形；不要回到按可见框厚统一 13px 的旧换算。
- 三卡现在按内容收缩，若未来任务列表变长，卡片会向下生长并在木槽上沿被 `.sd-opsrail-stack` 裁掉；如需完整展示需另行设计（如条目上限）。

# FE-RIGHT-RAIL-TOP-FROM-BOTTOM-1 前端接手记录

## 改了什么
- 按用户要求处理 `frontend/public/assets/stardew/ui/panels/right_rail_shell_bottom.png`：移除底段里的南瓜和向日葵遮挡区域，用同图干净木梁纹理补齐横梁，并用左端角饰镜像重建右端角饰。
- 将清理后的底段旋转 180 度，覆盖当前运行时上段素材 `right_rail_shell_top_line_image2.png`。底段原文件未覆盖，仍保留南瓜/向日葵并继续作为 `.sd-opsrail::after` 使用。
- 新上段尺寸为 `1871x840`，实测 alpha bbox 为 `(59,0)-(1871,384)`；横梁主要可见区为 `x123..1807/y146..291`。
- `.sd-opsrail::before` 的定位常量已按新图更新：`top = -146/1684`、`left = -123/1684`、`width = 1871/1684`、`aspect-ratio = 1871/840`；`.sd-opsrail-stack` 顶部 padding 改为按 `238/1684` 留出横梁与藤蔓深度。

## 影响文件/素材
- `frontend/public/assets/stardew/ui/panels/right_rail_shell_top_line_image2.png`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- `docs/frontend-handoff/frontend-handoff-2026-07-02.md`

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`
- 已用 Pillow 校验 `right_rail_shell_top_line_image2.png`：尺寸 `1871x840`、`mode=RGBA`、alpha 范围 `0..255`、bbox `(59,0)-(1871,384)`。
- 已人工预览新上段 PNG：南瓜和向日葵已消除，横梁和角饰无透明破洞。

## 下一步注意事项
- 当前上段 CSS 依赖新图的横梁范围 `x123..1807/y146..291`。如果以后再次替换 `right_rail_shell_top_line_image2.png`，需要重新量 bbox 和横梁范围，并同步 `.sd-opsrail::before` 与 `.sd-opsrail-stack` 顶部 padding 常量。
- 底段 `right_rail_shell_bottom.png` 未改；如果未来也想把底部运行时南瓜/向日葵移除，需要另开任务并同步 `.sd-opsrail::after` 的视觉验证。

# FE-TOPBAR-IMAGE2-REGEN-1 前端接手记录

## 改了什么
- 按用户要求废弃上一批观感不合格的顶栏素材，用 image2 参考 `01-overview.png` / `Top bar.png` 风格重新生成顶栏外壳、控件框和独立图标。
- 本轮没有用脚本从参考图按坐标裁切；脚本只处理 image2 生成结果的洋红 chroma-key 去底、透明 PNG 归一化、预览和校验。
- 顶栏外壳保持三段：`topbar_shell_left.png`、`topbar_shell_middle_tile.png`、`topbar_shell_right.png`。CSS 中左/右端按高度等比渲染，中段只做横向 `repeat-x`，不把整条顶栏或带控件的图拉成 `100% 100%`。
- 控件框改为新 image2 资源并通过 `border-image` 使用：状态按钮、当前存档框、版本框、用户框、登出按钮分别是 `topbar_status_button_9slice.png`、`topbar_save_frame_9slice.png`、`topbar_version_frame_9slice.png`、`topbar_user_frame_9slice.png`、`topbar_logout_button_9slice.png`。
- 顶栏文字继续由 React 渲染：`Stardew Anxi Panel`、运行状态、存档名、版本、角色、登出文案均不烘焙进 PNG。
- 图标改为独立 v2 PNG：鸡、农场、用户头像、叶子、绿色状态点、登出图标、下拉箭头。用户框右侧绿点和登出图标已作为独立层加入 TSX。
- 修复右端缺失：`topbar_shell_right.png` 从 image2 右端候选重新归一化到完整 `360x170` 高度，解决上一版 alpha 只在中间高度导致右侧收口像黑块的问题。

## 影响文件/素材
- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/public/assets/stardew/ui/topbar/topbar_shell_left.png`
- `frontend/public/assets/stardew/ui/topbar/topbar_shell_middle_tile.png`
- `frontend/public/assets/stardew/ui/topbar/topbar_shell_right.png`
- `frontend/public/assets/stardew/ui/topbar/topbar_status_button_9slice.png`
- `frontend/public/assets/stardew/ui/topbar/topbar_save_frame_9slice.png`
- `frontend/public/assets/stardew/ui/topbar/topbar_version_frame_9slice.png`
- `frontend/public/assets/stardew/ui/topbar/topbar_user_frame_9slice.png`
- `frontend/public/assets/stardew/ui/topbar/topbar_logout_button_9slice.png`
- `frontend/public/assets/stardew/ui/topbar/icon_topbar_*_image2_v2.png`

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`
- 已用内置浏览器临时 QA 页检查 1920x900：`topbar_shell_right.png` 以 `background-size: auto 100%` 渲染到完整 80px 顶栏高度，中段为 `repeat-x`，控件框均来自新 `*_9slice.png` 的 `border-image`，页面无横向溢出。
- 已检查 390x760：存档/版本/用户区域按移动端策略隐藏，状态和登出仍可见，`scrollWidth === clientWidth`。
- console error/warn 为空。

## 下一步注意事项
- 这批仍属于 image2 重绘候选接入，观感如果继续精修，应继续用 image2 重生单个不满意的部件，不要从参考原图脚本裁切。
- 顶栏左端和中段交界、右端和登出按钮间距如后续要继续贴近原图，应优先重生 shell left/right endcap，而不是把控件烘焙回 shell。
- `topbar_shell_middle_tile.png` 只允许是纯木板和上下边框；不要混入按钮、文字、头像、叶子或状态点。

# FE-RIGHT-RAIL-PROTOTYPE-ALIGN-1 前端接手记录

## 改了什么
- 按 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30/01-overview-right-sidebar-empty-image2.png` 继续调整右侧 OpsRail 的运行时比例。
- 右栏桌面列宽改为 `clamp(340px, 27vw, 430px)`，比上一版更接近原型的瘦高木质面板比例。
- 顶部 shell 固定段改为按素材有效区域裁剪：源图上方约 103px 透明安全边通过负 `background-position` 裁掉，使上边框贴到右栏顶部。底部 shell 按可见装饰区域贴底，stack 扣底部高度也同步使用可见区域高度。
- 中段继续 `right_rail_shell_middle_tile.png` 纵向 `repeat-y`，横向 `121%` 居中裁掉左右透明边；top/bottom 固定段保持 `108%` 横向有效区对齐，避免黑边。
- `.sd-opsrail-stack` 横向 padding 调整到 `clamp(18px, 1.8vw, 28px)`，让卡片在左右木柱内侧，不再压住侧柱；三行高度改为健康卡较高、进行中较矮、近期任务中等。
- 移除 `.sd-ops-card` 外投影，修复投影在卡片间隔处横穿左右木柱造成的“边框断裂”视觉。

## 影响文件/素材
- `frontend/src/games/stardew/StardewPanel.css`
- 未改 `StardewPanel.tsx`、后端接口、任务/健康数据来源或按钮路由逻辑。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`
- 已用本地 QA 页面复用真实 CSS/素材检查 1280x720 和 1280x900。
- 1280x900 检查结果：顶部固定段贴到右栏顶部；左右木柱不再被卡片投影横向切断；三张卡片位于内框范围；stack 高度等于内容高度、无额外滚动；console error/warn 为空。

## 下一步注意事项
- 这版仍保持三段 shell + 卡片九宫格 + React 动态内容分层。后续若要进一步贴近原型里的 CPU/内存/磁盘进度条，需要在 React 内容层补动态行和进度条，不要烘焙进背景图。
- 如果重新生成 shell 素材，需要重新测 top/middle/bottom 的透明安全边，更新 CSS 里的 103、751、587 等有效区域常量。

# FE-RIGHT-RAIL-BLACK-EDGE-FIX-1 前端接手记录

## 改了什么
- 修复右侧 OpsRail 两侧露黑的问题。根因是 `right_rail_shell_middle_tile.png` 左右包含透明/半透明暗边，原先按 `100%` 宽铺开时会透出 `.sd-opsrail` 的近黑背景。
- `.sd-opsrail-bg` 中段背景改为 `background-size: 121% auto`、居中、纵向 `repeat-y`，通过横向 overscan 裁掉素材两侧透明暗边。
- 顶部/底部固定段增加 `--sd-opsrail-endcap-scale: 1.08`，伪元素宽度为右栏 108%，高度按放大后的宽度和素材比例计算；`.sd-opsrail-stack` 扣除底部装饰高度时也使用同一比例，避免矮窗口重新被底部南瓜/向日葵覆盖。
- `.sd-opsrail` / `.sd-opsrail-bg` 兜底色改成木板棕，作为透明边缘的最后防线。

## 影响文件/素材
- `frontend/src/games/stardew/StardewPanel.css`
- 未修改素材文件、React 组件、后端接口、任务/健康数据来源和按钮路由逻辑。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`
- 已用本地 QA 页面复用真实 `StardewPanel.css` 和真实素材检查 1280x720、1280x560。
- 检查结果：`.sd-opsrail-bg` 仍为 `repeat-y`，背景尺寸为 `121%`；top/bottom 伪元素宽度为右栏 108%，高度按比例计算；1280x560 下 stack 仍是内部滚动；截图确认左右黑边被木纹/立柱覆盖；console error/warn 为空。

## 下一步注意事项
- 如果后续重新生成中段 shell 素材，先扫描左右透明边界；若新素材已无透明暗边，可以把 overscan 比例降回接近 `100%`。
- 不要用整张右栏截图兜底黑边；继续保持三段 shell 与卡片九宫格分层。

# FE-RIGHT-RAIL-3PIECE-RUNTIME-1 前端接手记录

## 改了什么
- 将 `StardewPanel` 右侧 OpsRail 运行时从旧整壳/外框组合迁移到新三段外壳素材：中段 `right_rail_shell_middle_tile.png` 只做纵向 `repeat-y`，顶部 `right_rail_shell_top.png` 和底部 `right_rail_shell_bottom.png` 分别用 `.sd-opsrail::before` / `.sd-opsrail::after` 固定比例覆盖。
- 删除生效规则里对右栏整张 shell 的 `background-size: 100% 100%` 拉伸依赖；顶部和底部固定段高度按右栏宽度与素材原始比例计算，窗口高度变化时不压扁。
- 三张卡片的 `border-image-source` 切到新素材：`right_card_health_9slice.png`、`right_card_progress_9slice.png`、`right_card_recent_9slice.png`。卡片内容、标题、图标、状态点、任务列表和按钮文字仍由 React/CSS 渲染，没有回烘到背景图。
- `.sd-opsrail-stack` 继续负责三张卡片布局，并改为内部滚动容器；滚动视口高度会扣掉底部固定装饰高度，矮窗口下不会让南瓜/向日葵盖住滚动区域。

## 影响文件/素材
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/public/assets/stardew/ui/panels/right_rail_shell_top.png`
- `frontend/public/assets/stardew/ui/panels/right_rail_shell_middle_tile.png`
- `frontend/public/assets/stardew/ui/panels/right_rail_shell_bottom.png`
- `frontend/public/assets/stardew/ui/panels/right_card_health_9slice.png`
- `frontend/public/assets/stardew/ui/panels/right_card_progress_9slice.png`
- `frontend/public/assets/stardew/ui/panels/right_card_recent_9slice.png`
- 未改后端接口、路由定义、任务/健康数据来源，也未改 `StardewPanel.tsx` 的右栏动态内容逻辑。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`
- 已用本地 QA 页面复用真实 `StardewPanel.css` 和真实素材做浏览器检查：1280x720、1280x900、1280x560、390x760。
- 检查结果：`.sd-opsrail-bg` 指向 `right_rail_shell_middle_tile.png` 且 `background-repeat: repeat-y`；top/bottom 伪元素分别指向 `right_rail_shell_top.png` / `right_rail_shell_bottom.png`；三张卡片 `border-image-source` 分别指向新 `right_card_*_9slice.png`；1280x560 下 `.sd-opsrail-stack` 内部滚动；390x760 下 `.sd-opsrail` 为 `display:none`；console error/warn 为空。
- 真实 `/instances/stardew/overview` 当前本地只到登录页，未使用测试账号猜测登录；因此业务态右栏验证通过隔离 QA DOM 完成，React 业务逻辑以构建和未改 TSX 为准。

## 下一步注意事项
- 后续不要恢复 `panel_right_rail_shell_empty_image2.png` 或整张右栏截图作为运行时背景；右栏外壳应保持 top + repeat-y middle + bottom 三段式。
- 卡片框继续使用九宫格或 `border-image`，不要把标题、进度条、任务列表、按钮文案或状态点烘焙进卡片背景。
- 如果将来重生成 shell 素材，需同步检查 top/bottom 的原始宽高比和 `.sd-opsrail-stack` 扣底部装饰高度的公式。

# FE-ASSET-RIGHT-RAIL-SHELL-3PIECE-1 前端接手记录

## 改了什么
- 新增 6 张 image2 重新生成的右侧栏分层 PNG：`right_rail_shell_top.png`、`right_rail_shell_middle_tile.png`、`right_rail_shell_bottom.png`、`right_card_health_9slice.png`、`right_card_progress_9slice.png`、`right_card_recent_9slice.png`。
- 三段 shell 分别只保留：顶部横梁/上边框/藤蔓角饰；左右木柱 + 中间纯木板 repeat-y 背景；底部木梁 + 南瓜 + 向日葵 + 底部藤蔓。
- 三张卡片框只保留木质边框、角饰、藤蔓和空的深棕木纹内容底，作为 health/progress/recent 三类卡片九宫格素材。
- 本轮按参考图风格用 image2 重绘，不是按坐标脚本裁切原图。心形图标、标题文字、CPU/内存/磁盘、进度条、任务列表、按钮文字和箭头均未烘焙进背景图。
- 本次只新增生产素材，未改 `StardewPanel` / CSS 运行时引用；当前右栏仍优先使用既有 `panel_right_rail_*` 系列。

## 影响文件/素材
- `frontend/public/assets/stardew/ui/panels/right_rail_shell_top.png`
- `frontend/public/assets/stardew/ui/panels/right_rail_shell_middle_tile.png`
- `frontend/public/assets/stardew/ui/panels/right_rail_shell_bottom.png`
- `frontend/public/assets/stardew/ui/panels/right_card_health_9slice.png`
- `frontend/public/assets/stardew/ui/panels/right_card_progress_9slice.png`
- `frontend/public/assets/stardew/ui/panels/right_card_recent_9slice.png`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`

## 如何验证
- 用 image2 生成洋红 chroma-key 源图后转为 RGBA 透明 PNG。
- 已用 Pillow 检查 6 张素材均为 `mode=RGBA`、alpha 范围 `0..255`、洋红残留 `0`。
- 输出尺寸：top `1842x854`、middle `1536x1024`、bottom `1871x840`、health card `1053x1494`、progress card `1693x929`、recent card `1535x1025`。
- 已用棋盘底预览人工确认：无中文标题、心形/时钟/剪贴板图标、CPU/内存/磁盘文字、进度条、任务列表、按钮文字或箭头残影。

## 下一步注意事项
- 接入时继续保持 shell、卡片框、标题图标、标题文字、状态点、进度条、任务列表和按钮文字分层，不要重新合并为一张右栏截图。
- `right_rail_shell_middle_tile.png` 是上下开口的中段 tile，适合纵向平铺；不要当完整卡片或整栏背景拉伸。
- 如果替换当前运行时 `panel_right_rail_*`，需要同步重调 CSS 尺寸、定位和 `border-image-slice`。

# FE-SIDEBAR-ROW-BG-1 前端接手记录

## 改了什么
- `StardewPanel` 左侧栏运行时接入三段式侧栏素材，替换整张 `panel_side_rail_shell_empty_image2.png` 按 `100% 100%` 拉伸的背景方式。
- `.sd-sidebar` 现在用 `panel_side_rail_middle_tile_image2.png` 作为可纵向平铺底；`.sd-sidebar::before` 叠顶部段 `panel_side_rail_top_image2.png`，`.sd-sidebar::after` 叠底部段 `panel_side_rail_bottom_image2.png`。
- 导航列表新增 `.sd-nav-list`，每个导航项外面新增 `.sd-nav-row`。完整中段 tile 只由 `.sd-sidebar` 外层绘制，`.sd-nav-row` 不再引用中段素材，只保留轻微上下像素阴影，作为“每个按钮背后的行槽感”；按钮本身继续使用 default / hover / active 三张导航按钮底图。
- 这是方案 B：按钮背后的分段视觉不再烘焙在整张侧栏背景中，而是跟按钮所在行容器共用布局盒子；放大缩小时，行背景和按钮同高同位，避免错位。
- 右边框统一性修正：因为 `.sd-nav-list` 可能出现纵向滚动条，行容器的 `width: 100%` 会被滚动条压窄；如果行容器继续画完整 middle tile，素材右边框会视觉左移。现在边框/立柱只在 `.sd-sidebar` 全宽背景层绘制，滚动容器只影响按钮行，不影响外框。
- 按钮尺寸修正：`.sd-sidebar .sd-nav-item` 宽度改为 `min(86cqi, 210px)`，以侧栏 container query 宽度为基准，不再随 `.sd-nav-row` / `.sd-nav-list` 的滚动条内容宽度变窄。
- 按钮位置修正：`.sd-nav-list` 继续可纵向滚动，但滚动条在桌面端隐藏，避免浏览器为滚动条预留宽度后把 `.sd-nav-row` 的居中区域向左压缩，导致按钮整体左移。
- 移动端补充 `.sd-nav-list` / `.sd-nav-row` 覆盖，继续保持横向图标导航，不显示桌面行背景。

## 影响文件/素材
- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/public/assets/stardew/ui/panels/panel_side_rail_top_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_side_rail_middle_tile_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_side_rail_bottom_image2.png`
- 接口、路由、页面业务逻辑不变。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`
- 移除 `.sd-nav-row` 完整中段背景后已再次执行：`cd frontend; npm.cmd run build`
- 按钮宽度改为 `cqi` 后已再次执行：`cd frontend; npm.cmd run build`
- 隐藏桌面导航滚动条后已再次执行：`cd frontend; npm.cmd run build`
- 已确认本地 Vite 只监听 `localhost:5173` / `::1`，`127.0.0.1:5173` 当前不可连。
- 已用内置浏览器打开 `http://localhost:5173/instances/stardew/overview`，登录页可渲染且 console error/warn 为空。
- 当前本地浏览器没有有效登录态，尝试开发测试账号 `admin/admin-password` 返回用户名或密码错误，因此未完成真实登录态侧栏桌面/移动截图验证。

## 下一步注意事项
- 后续不要再把整张左栏背景图做 `background-size: 100% 100%` 来承载按钮槽位；按钮槽位应继续放在 `.sd-nav-row` 或按钮自身附近。
- 如果将来重新生成独立“导航行槽位”素材，可替换 `.sd-nav-row` 的背景，但仍应保持一行一槽，不要回到整张 9 槽位背景图。
- 底部段已经包含装饰和书架；若要恢复独立装饰动效，需要先改底部段或隐藏对应区域，避免重复叠图。

# FE-ASSET-SIDEBAR-3PIECE-1 前端接手记录

## 改了什么
- 新增左侧栏三段式透明 PNG 素材：`panel_side_rail_top_image2.png`、`panel_side_rail_middle_tile_image2.png`、`panel_side_rail_bottom_image2.png`。
- 三张素材都基于 image2 左侧栏现有空壳/参考图拆分：顶部段只保留顶部木质外框与横梁，中段是可纵向 `repeat-y` 的纯木板/立柱 tile，底部段保留书架、灯笼、盆栽、紫水晶、书本/盒子等固定装饰。
- 中段采用方案 A：背景不预留导航按钮位置，不包含任何横向分隔线、按钮槽位、暗条、分层隔板或固定行高结构；导航按钮区域完全由前端按钮素材单独叠加。
- 中段尺寸为 `598x96`，首尾行已处理为一致，避免 CSS 纵向平铺时出现硬接缝；三张素材均保留 RGBA 透明安全边距。
- 本轮只新增生产素材，未改 `StardewPanel` / CSS 引用；当前运行时仍沿用已接入的左栏背景组合。

## 影响文件/素材
- `frontend/public/assets/stardew/ui/panels/panel_side_rail_top_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_side_rail_middle_tile_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_side_rail_bottom_image2.png`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 接口、路由、页面业务逻辑不变。

## 如何验证
- 已用 Pillow 检查三张素材均为 `mode=RGBA`、alpha 范围 `0..255`、四角 alpha 为 `0`。
- 尺寸分别为 `top 598x128`、`middle 598x96`、`bottom 598x409`。
- 已检查无绿色/洋红 chroma-key 残留；`middle` 首尾行平均差异为 `0.0`，中心区域无结构性暗行 outlier。
- 已人工预览确认三张素材无文字、菜单图标、导航按钮或按钮高亮残影；`middle` 无按钮槽位/横向隔板，拼接预览可形成完整左侧木质栏。

## 下一步注意事项
- 后续接入响应式侧栏时，建议用顶部固定段 + 中段 `background-repeat: repeat-y` + 底部固定段组合，避免直接拉伸整张左栏空壳导致像素纹理变形。
- 导航按钮、图标、中文 label、hover/active 状态仍应作为独立层叠加，不要重新烘焙进这组三段背景素材。
- 底部段已经包含书架和装饰物，接入时不要再叠加 `sidebar_bottom_decor_props_group_image2.png`，否则会重复。

# FE-RIGHT-RAIL-SPLIT-ASSETS-1 前端接手记录

## 改了什么
- `StardewPanel` 右侧 OpsRail 已从整张 `panel_right_rail_image2.png` 背景/透明热区方案，切换为拆分素材组合渲染。
- `.sd-opsrail-bg` 使用 `panel_right_rail_shell_empty_image2.png` 作为右栏木质背景空壳；`.sd-opsrail::after` 使用 `panel_right_rail_outer_border_image2.png` 作为外框覆盖层。
- 三张主卡片改为 `.sd-ops-card`，分别用 `panel_right_rail_card_health_nineslice_image2.png`、`panel_right_rail_card_in_progress_nineslice_image2.png`、`panel_right_rail_card_recent_tasks_nineslice_image2.png` 通过 CSS `border-image` 渲染九宫格木质卡片框。
- 标题图标改用三枚独立 PNG，中文标题、健康状态、任务行和按钮文字仍由 React 渲染；旧图里烘焙的文字/图标/列表不再参与运行时。
- `JOB_STATUS_DOT` 与 `healthSummaryDot()` 未改，绿/黄/红/灰点继续复用 `.sd-dot*` CSS 动态状态；Mod 重启提示收进近期任务卡片底部，不再新增第四张主卡打乱布局。

## 影响文件/素材
- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/public/assets/stardew/ui/panels/panel_right_rail_shell_empty_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_right_rail_outer_border_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_right_rail_card_health_nineslice_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_right_rail_card_in_progress_nineslice_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_right_rail_card_recent_tasks_nineslice_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_right_rail_health_heart_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_right_rail_in_progress_clock_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_right_rail_recent_tasks_clipboard_image2.png`
- 接口、后端、任务/健康数据契约不变。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`
- 已用内置浏览器打开 `http://127.0.0.1:5173/instances/stardew/overview`，确认 1280×720 下右栏显示背景空壳、外框、三张卡片和三个标题图标；DOM 中 `.sd-opsrail` 背景为 `none`，`.sd-opsrail-bg` 指向 `panel_right_rail_shell_empty_image2.png`，三张卡片的 `border-image-source` 分别指向三张 `*_nineslice_image2.png`。
- 已点击右栏“查看诊断”，URL 切到 `/instances/stardew/diagnostics`；回到总览后点击“查看全部任务”，URL 切到 `/instances/stardew/jobs`。
- 已检查 console error/warn 为空。
- 已检查 390×760 移动视口：`.sd-opsrail` 为 `display:none`，`body` / `.sd-shell` 无水平溢出。

## 下一步注意事项
- 后续不要再把 `panel_right_rail_image2.png` 作为右侧栏运行时背景；右栏应继续保持背景空壳、外框、卡片框、图标、文字、状态点和数据层分离。
- 如果新增 CPU/内存/磁盘进度条素材，应作为独立进度槽/填充层接入，不要烘焙进健康卡片框。
- `border-image-slice` 当前按三张 `nineslice` 素材的透明边距和角饰位置调过；更换素材后优先调整 CSS slice/边框宽度，不要退回整张卡片拉伸。

# FE-TOPBAR-SPLIT-ASSETS-1 前端接手记录

## 改了什么
- `StardewPanel` 顶栏已从整张 `panel_top_bar_image2.png` 可见背景迁移为拆分素材组合渲染。
- 新增运行时 topbar 素材目录 `frontend/public/assets/stardew/ui/topbar/`：包含三段式横栏空壳、品牌鸡、品牌文字发光占位、农场图标、下拉箭头、状态框、农场框三段式、版本框、用户框三段式、用户头像和登出按钮底图。
- 顶栏文案继续由 React 动态渲染：品牌名、状态文字、当前农场名、版本号、用户角色和登出文字都不再依赖整张 `Top bar.png` 烘焙内容。
- 状态区域移除 running/stopped 图片引用，改为 `topbar-status-frame.png` 背景 + 现有 `.sd-dot` / `.sd-dot-pulse` 动态点；没有把红绿状态点换成 PNG。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/public/assets/stardew/ui/topbar/*`
- 未改后端接口，未改 `useStardewDashboardData` 数据来源，未改路由定义。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`
- 已用浏览器检查 `http://127.0.0.1:5173/instances/stardew/overview`、`/server`、`/saves`、`/settings`。
- 已确认状态区域显示现有动态 dot class：running 场景可出现 `sd-dot sd-dot-green sd-dot-pulse`，loading/unknown 场景显示 `sd-dot sd-dot-yellow sd-dot-pulse`。
- 已点击顶栏状态、存档、版本、用户区域，分别验证跳转到 `/server`、`/saves`、`/settings`、`/settings`；实际点击登出按钮后回到登录表单，确认 `onLogout` 路径仍有效。
- 已确认存档名样式为 `overflow: hidden`、`text-overflow: ellipsis`、`white-space: nowrap`。
- 已检查 1280×720 桌面和 390×760 移动端均无横向溢出，移动端只保留状态和登出按钮；控制台无 error/warn。

## 下一步注意事项
- 后续不要再把 `panel_top_bar_image2.png` 作为运行时顶栏可见背景；它可以继续作为旧素材保留，但顶栏渲染应使用 `ui/topbar/` 下的拆分层。
- 状态点继续复用 `.sd-dot` / `.sd-dot-pulse`，不要把 running/stopped 状态重新做成静态图片。
- 顶栏农场图标是本次从 image2 原图补出的独立默认图标；如果后续有多农场图标体系，可替换该 icon 层，不需要改按钮底图。

# FE-SIDEBAR-SPLIT-ASSETS-1 前端接手记录

## 改了什么
- `StardewPanel` 左侧栏已从整张 `panel_side_rail_image2.png` / `Left panel.png` 透明热区方案，切换为拆分素材组合渲染。
- 桌面端 `.sd-sidebar` 使用 `panel_side_rail_shell_empty_image2.png` 作为唯一侧栏背景并填满侧栏格子；9 个导航按钮分别使用 default / hover / active 三张按钮底图，未选中 hover 使用 `nav_item_hover_wood_blank_image2.png`，active 与 active:hover 使用 `nav_item_active_wood_blank_image2.png`；导航图标改用 9 个 `icon_nav_*_image2.png` 拆分图标；中文菜单文字由 React 文本渲染。
- `stardew-theme.css` 中旧的全局 `.sd-nav-item:hover` 会用 `background` 简写覆盖按钮底图，已增加桌面 `.sd-sidebar .sd-nav-item` 限定覆盖，防止 hover 露出侧栏横条或退回旧绿色按钮。
- 侧栏四周增加 CSS 像素边框补强，避免空壳背景边缘缺失；底部不再叠加 `sidebar_bottom_decor_props_group_image2.png`，避免与空壳底部残留装饰重复。
- 移动端继续使用横向图标导航，隐藏 label，保留 active 金色像素边框，并移除整张左栏背景。

## 影响文件/素材
- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/src/games/stardew/stardew-theme.css`
- `frontend/public/assets/stardew/ui/panels/panel_side_rail_shell_empty_image2.png`
- `frontend/public/assets/stardew/ui/navigation/nav_item_default_wood_blank_image2.png`
- `frontend/public/assets/stardew/ui/navigation/nav_item_hover_wood_blank_image2.png`
- `frontend/public/assets/stardew/ui/navigation/nav_item_active_wood_blank_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_nav_*_image2.png`
- `frontend/public/assets/stardew/ui/sprites/sidebar_bottom_decor_props_group_image2.png`（本轮未接入运行时，保留为后续备用素材）
- 接口、路由、页面业务逻辑不变。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`
- 已用内置浏览器打开 `http://127.0.0.1:5173/instances/stardew/overview`，确认桌面左侧栏 9 个菜单均可见，图标和中文 label 均由拆分层渲染，“任务日志”完整显示。
- 已点击左侧“服务器”，URL 切到 `/instances/stardew/server`，active 菜单变为“服务器”。
- 已点击左侧“诊断”，URL 切到 `/instances/stardew/diagnostics`，active 菜单变为“诊断”。
- 已检查 console error/warn 为空。
- 已检查 390×760 移动视口：横向图标栏 9 个按钮全部在 390px 内，active 边框可见，`body` / `.sd-shell` / `.sd-sidebar` 均无水平溢出。

## 下一步注意事项
- 后续不要再让运行时代码引用 `docs/prototypes` 或重新使用带文字的 `panel_side_rail_image2.png` 作为可见菜单。
- 桌面按钮当前按侧栏宽度和按钮原始比例控制尺寸，并用 CSS 边框补强侧栏四周；如果以后更换按钮素材，优先调整 `.sd-sidebar .sd-nav-item` 的 `width` / `aspect-ratio`，不要把整张侧栏拉伸回 `100% 100%`。
- `nav_item_hover_wood_blank_image2.png` 当前已接入运行时，只用于未选中按钮 hover；不要把 `.sd-nav-item:hover` 写成全局 `background` 简写，否则会覆盖按钮底图的 repeat/position/size 并重新造成破图。
- 当前没有接入底部独立装饰层，因为它会和空壳底部残留的书架/装饰重复；后续若要恢复，应先产出真正去除底部装饰的纯空壳，或用遮罩清理空壳底部后再叠独立素材。
- 移动端为了避免 390px 下横向滚动，使用固定 36px 图标按钮而不是横向木质长按钮；如果需要移动端显示 label，应重新设计为两行或抽屉式导航。

# FE-SHELL-IMAGE2-1 前端接手记录

## 改了什么
- `StardewPanel` 的顶栏改用 `Top bar.png` 作为整条背景，左侧导航改用 `Left panel.png`，右侧 OpsRail 改用 `01-overview-right-sidebar-empty-image2.png`。
- 顶栏槽位迁移现有逻辑：状态图点击进服务器页，农场名点击进存档页，版本/角色点击进设置页，登出仍调用原登出回调。
- 状态图按实例状态切换 running/stopped 透明 PNG，并给 URL 加状态 query，避免运行/停止切换时浏览器沿用旧缓存图。
- 桌面左侧导航使用透明热区覆盖原型图九个菜单，保留 active/hover/focus；移动端保留横向图标导航。
- 右侧任务栏保留系统健康、任务列表和 Mod 重启提示，内容定位到右栏专用素材的“系统健康 / 进行中 / 近期任务”框内。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/public/assets/stardew/ui/panels/panel_top_bar_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_side_rail_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_right_rail_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_status_running_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_status_stopped_image2.png`
- 接口不变；只调整 Shell UI 和既有点击入口。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`
- 已用内置浏览器登录态打开 `http://127.0.0.1:5173/instances/stardew/overview`，确认首屏非空、无 framework overlay、无 console error/warn。
- 已点击左侧“服务器”透明热区，URL 切到 `/instances/stardew/server`，active 菜单变为“服务器”。
- 已点击右侧栏“查看详情”透明入口，URL 切到 `/instances/stardew/diagnostics`，active 菜单变为“诊断”。
- 已检查 1280×720 桌面与 390×760 移动视口；移动端九个导航按钮同一行排列，页面无水平溢出。

## 下一步注意事项
- 右侧栏现在使用 `01-overview-right-sidebar-empty-image2.png` 对应的独立素材；后续若要填充 CPU/内存/磁盘等进度条，可在 `sd-opsrail-health` 内继续叠加实时资源数据。
- 顶栏农场槽优先显示 `farmName`，依赖存档解析结果；无法解析时显示 active save 目录名。

# FE-PROTOTYPE-LAYOUT-1 前端接手记录

## 改了什么
- 按 `docs/prototypes/stardew-page-prototypes-image2-2026-06-30` 的页面信息架构重排主要 Stardew 页面，不复刻原型静态内容。
- 总览页改成“农场横幅 → 服务器控制/邀请码 → 四个指标 → 在线玩家/近期事件/模组状态”的排布。
- 存档页新增当前激活存档重点卡，并调整按钮组换行策略，避免在当前主栏宽度被右侧滚动条截断。
- 服务器、任务、玩家、模组、诊断、设置页增加页面级布局 class；诊断页明确拆成左侧检查/告警和右侧资源趋势双栏。

## 影响文件/接口
- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/games/stardew/pages/SavesPage.tsx`
- `frontend/src/games/stardew/pages/JobsLogsPage.tsx`
- `frontend/src/games/stardew/pages/PlayersPage.tsx`
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/pages/DiagnosticsPage.tsx`
- `frontend/src/games/stardew/SavesSection.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 接口不变；本次只调整前端布局和现有数据的位置。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`
- 已用内置浏览器登录 `http://127.0.0.1:5174/`，检查 `overview/server/saves/jobs/players/mods/diagnostics/settings` 首屏渲染，无 console error/warn。
- 已用 390px 移动视口检查 `overview/saves/jobs`，确认单列布局可滚动且主要按钮未截断。

## 下一步注意事项
- `ModsPage` 保留现有三段式工作台，不要按旧原型退回单页“已安装 + 上传”结构；后续只继续优化各 tab 内部密度。
- 存档页当前重点卡使用现有 farmhouse 场景图作为占位视觉，后续若后端提供真实农场预览图，可以替换 `.sd-saves-active-art` 背景。
- 诊断页资源趋势当前在无采样数据时仍显示空图表，这是原有数据状态；本次只重排位置。

# PERF-REVIEW-1 前端接手记录

## 改了什么
- `ModsPage` 把已安装 Mod 的排序、Nexus 展示列表、本地隐藏列表、解析错误数、同步分类统计和可打包数量合并进一个 `useMemo`。
- 删除不再需要的 `syncableMods` 中间数组，减少页面频繁 state 更新时的重复 `filter` 和临时数组分配。

## 影响文件/接口
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- 接口不变；只是渲染侧派生数据计算方式调整。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`。

## 下一步注意事项
- 后续继续拆分 `ModsPage` 时，可以把 Nexus 搜索卡片、已安装卡片和配置列表拆成 memoized 子组件；本次先保持单文件结构，避免和当前扩展安装功能线产生大范围冲突。

# NEXUS-EXT-3 前端交接

## 改了什么

- `ModsPage` 的 Nexus 搜索结果“一键安装”不再直接调用 `POST /mods/nexus/install`，改为同页跳转到 Nexus 对应 Mod 的 `?tab=files&anxi_auto=1` 文件页，由浏览器扩展接手获取临时 ZIP 链接。
- 浏览器扩展会在 Nexus 文件页自动打开手动下载/慢速下载流程，拿到 `supporter-files.nexus-cdn.com/*.zip` 临时链接后只在右下角显示一个“提交到面板”按钮。用户点击提交后，扩展调用现有 `POST /api/instances/:id/mods/remote/install`，并立刻跳回面板任务页。
- 任务页支持 `?jobId=<id>`，扩展回跳到 `/instances/:id/jobs?jobId=...` 后会直接选中新创建的安装任务并打开 SSE 日志。
- `ModInstallMethod` 新增 `nexus_extension`，用于标记当前搜索卡片的安装方式已经切到扩展链路。

## 影响文件

- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/pages/JobsLogsPage.tsx`
- `frontend/src/types.ts`
- `browser-extensions/nexus-slow-installer/content.js`
- `browser-extensions/nexus-slow-installer/background.js`
- `browser-extensions/nexus-slow-installer/shared.js`

## 如何验证

- 已执行：`cd frontend; npm.cmd run build`
- 已执行：`node --check browser-extensions/nexus-slow-installer/content.js`
- 已执行：`node --check browser-extensions/nexus-slow-installer/background.js`
- 已执行：`node --check browser-extensions/nexus-slow-installer/shared.js`
- 手动联调建议：面板停服、同浏览器登录面板管理员和 Nexus，点击搜索结果“一键安装”，确认跳到 Nexus 文件页后扩展自动捕获 ZIP；点击右下角提交按钮后应回到任务日志页并选中刚创建的 `mod_remote_install` 任务。

## 下一步注意

- 这个链路依赖浏览器扩展复用同浏览器里的面板登录 Cookie。正式云端部署若出现 401/403，应优先做扩展专用配对 token，而不是把管理员密码或完整 Nexus 链接长期保存到扩展状态。
- 扩展拿到的完整 Nexus CDN 临时 URL 只应作为请求体短暂提交，状态、日志、文档和支持包里只能保留脱敏 URL。
- Nexus 多文件 Mod 目前仍由扩展自动点击页面上可见的第一个手动下载入口；后续如果要精确文件选择，需要在面板或扩展里增加文件列表/版本选择 UI。
# NEXUS-REQ-1 前端接手

## 改了什么
- `NexusModSearchResult` 增加 `requiredMods[]` 类型字段；`ModsPage` 在 Nexus 搜索结果卡片 footer 显示前置状态。
- 缺失前置会显示“安装前置”按钮，点击后用该前置的 Nexus modId 生成 `?tab=files&anxi_auto=1` 链接，复用浏览器扩展安装链路。
- `browser-extensions/nexus-slow-installer/content.js` 会在捕获流程开始后检测 Nexus “Additional files required” 弹窗，并自动点击弹窗里的 `Download` 按钮。

## 影响文件/接口
- `frontend/src/types.ts`
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `browser-extensions/nexus-slow-installer/content.js`
- 依赖后端 `GET /api/instances/:id/mods/nexus/search` 返回 `results[].requiredMods[]`。

## 如何验证
- `cd frontend; npm.cmd run build`
- `node --check browser-extensions/nexus-slow-installer/content.js`
- `node --check browser-extensions/nexus-slow-installer/background.js`
- `node --check browser-extensions/nexus-slow-installer/shared.js`

## 下一步注意事项
- `requiredMods[]` 是 Nexus 页面声明的前置，不等同于 ZIP 内 manifest 的 SMAPI `Dependencies`。安装后仍要看已安装卡片的 `dependencies[]` 状态。
- SMAPI 这类运行时依赖通常由虚拟内置条目表达；不要把它当作普通服务器 Mod 文件夹处理。
# NEXUS-PREMIUM-2 前端接手

## 改了什么
- 删除 `ModsPage` 下载页的“粘贴链接安装”人工入口，包括按钮、弹窗、相关 state、`installRemoteMod()` 前端 API 封装和 `RemoteModInstallRequest` 类型。
- Nexus Key 未配置时，在“配置 Nexus Key”按钮左侧显示 `如果您是尊贵的 Nexus Premium 用户，请填您的 NexusKey`；Key 已配置后该提示消失，只显示已配置状态。
- Key 已配置时，Nexus 搜索结果每个卡片底部新增 `N站会员专属安装` 按钮，复用现有 `installNexusMod()` 直连安装任务。普通 `一键安装` 继续走扩展跳转流程，服务于非 Premium 用户。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/src/api.ts`
- `frontend/src/types.ts`

## 如何验证
- `cd frontend; npm.cmd run build`
- 手动联调建议：未配置 Nexus Key 时确认只显示提示和“配置 Nexus Key”，搜索卡片不显示会员按钮；配置 Key 后提示消失，每个搜索结果卡片底部出现 `N站会员专属安装`，普通 `一键安装` 仍跳转 Nexus 扩展页面。

## 下一步注意事项
- `POST /api/instances/:id/mods/remote/install` 仍由浏览器扩展提交临时 ZIP 使用，前端页面不再提供手动粘贴入口。
- 会员按钮要求管理员、服务器停止、目标 Mod 未安装且当前无安装任务；失败时仍沿用任务日志和安装错误面板展示。
# NEXUS-CARD-UI-1 前端接手

## 改了什么
- 优化 `ModsPage` 的 Nexus 搜索结果卡片布局：卡片改成内容区、主操作区、次操作区，`N站页面/一键安装` 这类统一主按钮固定在同一行，减少不同卡片按钮上下漂移。
- `N站会员专属安装` 放入底部次操作区，不再挤在主按钮行里；前置依赖状态也放在该区域。
- 前置依赖展示从“逐个标签 + 安装前置按钮”改成一个状态入口，仅显示 `缺少前置mod` 或 `前置已满足`。点击或悬停会展开详情，包含前置 Mod 名、NexusId 和当前状态。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`

## 如何验证
- `cd frontend; npm.cmd run build`
- 手动联调建议：登录面板后进入 `模组 -> 下载模组`，搜索带前置依赖的 Nexus Mod，确认卡片主按钮对齐、底部会员按钮不挤占主操作、依赖状态可悬停/点击展开详情。

## 下一步注意事项
- 搜索结果卡片不再直接提供“安装前置”小按钮；如果后续要恢复依赖一键安装，建议放到展开详情内或单独依赖管理弹窗，避免重新撑乱卡片布局。
- 本次没有改后端 `requiredMods[]` 数据结构，前端仍依赖 `name/modId/installed/installedEnabled/installedVersion` 字段渲染详情。

# NEXUS-EXT-BATCH-1 前端接手

## 改了什么
- 普通 `一键安装` 不再让面板页跳转 Nexus。`ModsPage` 通过浏览器扩展 `panel-bridge.js` 发送 `START_BATCH_INSTALL`，扩展后台用非激活标签页同时打开当前 Mod 和未安装 Nexus 前置 Mod 的下载页。
- Nexus content script 读取 `anxi_batch/anxi_item/anxi_auto_submit` 参数，捕获 ZIP 链接后自动提交到面板；批量模式提交成功后关闭后台标签页，不跳任务日志页。
- 搜索卡片的一键安装按钮变成百分比进度条。扩展获取/提交阶段按单项 `opening=10 / capturing=35 / ready=65 / posting=80 / queued=90` 计算，多项取平均；拿到面板 `jobId` 后继续轮询 `GET /api/jobs/:id`，所有 job `succeeded` 才 100%，任一 job `failed/canceled` 会显示失败和对应 Mod 名。扩展无响应、后台页超时或任一项提交失败时显示 `失败请手动安装`。
- 无 `jobId` 的 item 会刷新本地 Mod 列表按 `nexusModId/originNexusModId` 兜底匹配，命中则视为完成，解决 Custom Companions 这类实际已安装但扩展状态卡住的问题。
- 根因修复：content 在 `CAPTURE_URL` / `SUBMIT_CAPTURED_URL` 中显式传 `batchId/itemId/autoSubmit`，background 会从这些字段或 `captureKey=batch:item` 恢复 capture 的 batch 上下文，避免后端创建了任务但 batch item 没有 `jobId`。
- Nexus 文件列表页的 `Manual download` 和前置确认弹窗的 `Download` 不再只依赖 debugger 坐标点击；content script 会优先读取按钮 `href` 并直接跳转，同时把当前 `anxi_batch/anxi_item/anxi_auto_submit` 参数带到下一页。Manual 若是 JS 按钮，会通过 background 在页面主世界触发 `button.click()`，避免后台标签页卡在“正在进入下载页”。
- 批量模式自动提交按来源分流：content 直接生成 ZIP 链接时，`CAPTURE_URL` message 内继续推进；Chrome `downloads.onCreated` 捕获 ZIP 时，background 只保存链接并通知 Nexus 页，content 再发 `SUBMIT_CAPTURED_URL` 提交，避免 MV3 service worker 在下载事件里长 fetch 导致卡在 `posting`。Nexus content script 会把批量安装上下文写入 `sessionStorage`，跳转丢失 URL 参数时仍可识别自动提交，收到 ZIP ready 后直接调用原提交按钮逻辑。批量任务提交面板时优先通过已登录面板标签页里的 `panel-bridge.js` 发同源 `POST /api/instances/:id/mods/remote/install`，复用面板 Cookie/Vite proxy；面板桥接不可达才回退 background 直连。面板提交请求有 30 秒超时，失败会回写 batch 状态。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `browser-extensions/nexus-slow-installer/background.js`
- `browser-extensions/nexus-slow-installer/content.js`
- `browser-extensions/nexus-slow-installer/panel-bridge.js`
- `browser-extensions/nexus-slow-installer/manifest.json`

## 如何验证
- 已执行：`cd frontend; npm.cmd run build`
- 已执行：`node --check browser-extensions/nexus-slow-installer/background.js`
- 已执行：`node --check browser-extensions/nexus-slow-installer/content.js`
- 已执行：`node --check browser-extensions/nexus-slow-installer/shared.js`
- 已执行：`node --check browser-extensions/nexus-slow-installer/panel-bridge.js`

## 下一步注意事项
- 扩展加载或更新后，需要刷新面板页，让 `panel-bridge.js` 注入当前面板页面。
- 如果按钮提示“浏览器扩展未响应”，优先确认扩展管理页已重新加载扩展并刷新面板页；如果提示 `panel origin mismatch`，需要把扩展弹窗里的面板地址改成当前面板访问地址，或在本地开发时使用 `localhost/127.0.0.1` 回环地址。
- Nexus 搜索词、搜索结果、分页和当前扩展 batch 状态都保存在 `sessionStorage`。切到任务日志再回到模组页时不应重新加载默认热门，也不应丢失按钮进度；恢复后会继续 `GET_BATCH_STATUS` 轮询扩展。
- 前端按钮百分比现在会继续追踪面板 `mod_remote_install` job：job 创建只到 90%，后端实际安装成功才 100%；后端失败会反映到按钮失败态，任务日志仍是排查细节的来源。
- 如果没有 job 记录但本地 Mod 已安装，前端以本地 `GET /mods` 结果为准收敛进度；任务日志缺失时仍能让按钮结束。
- 新流程应优先拿到 jobId；本地安装兜底只是消息丢失时的防卡死保护，不应作为常规路径。
- 卡住恢复入口：有扩展安装状态的搜索卡片会显示 `重置状态`。它会清前端 session、停止轮询，并让 `panel-bridge.js` 调用 background `CLEAR_STATE` 清扩展存储；前后端重启不会清掉这些浏览器状态。
- 当前只自动下载 `required.installed=false` 的 Nexus 前置。已安装但当前存档未启用的前置由配置模组页启用逻辑处理，不重复下载安装到服务器。
# NEXUS-EXT-BATCH-2 前端接手

## 改了什么
- `ModsPage` 修复扩展批量安装达到 100% 后仍显示安装中的问题：`done/failed` 现在是终态，旧的 `GET_BATCH_STATUS` running 响应不会覆盖终态。
- 安装完成或本地兜底命中后，会用最新 `GET /mods` 结果同步当前 Nexus 搜索结果和 `requiredMods[]` 的已安装字段，避免切到任务日志再回来后按钮又变回“一键安装”。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`

## 如何验证
- `cd frontend; npm.cmd run build`
- `node --check browser-extensions/nexus-slow-installer/background.js`
- `node --check browser-extensions/nexus-slow-installer/content.js`
- `node --check browser-extensions/nexus-slow-installer/shared.js`
- `node --check browser-extensions/nexus-slow-installer/panel-bridge.js`

## 下一步注意事项
- 扩展 batch 的 job 创建仍只代表进入后端任务队列；按钮必须继续以 `GET /api/jobs/:id` 的最终 `succeeded/failed/canceled` 为准。
- 如果用户已经手动清理或重装 Mod，搜索结果缓存只会在下一次 `GET /mods` 同步后更新；不要只信任扩展 batch 的旧 storage。
# NEXUS-EXT-BATCH-3 前端接手

## 改了什么
- `browser-extensions/nexus-slow-installer/background.js` 的 `START_BATCH_INSTALL` 入口增加 `uniqueBatchTargets()`，先按 Nexus `modId` 去重，缺少 `modId` 时按清理批量参数后的 URL 去重。
- 同一个目标既以 `required` 又以 `target` 身份进入批量队列时，优先保留 `target`，避免当前 Mod 本体被打开两次。
- 同一个 `batchId` 重复发起时直接返回已有 batch；如果面板页刷新导致 tabId 变化，会更新 `panelTabId`，但不会再开 Nexus 标签页。

## 影响文件
- `browser-extensions/nexus-slow-installer/background.js`

## 如何验证
- 已执行：`node --check browser-extensions/nexus-slow-installer/background.js`
- 手动验证建议：点击 Ridgeside Village 这类本体加多个前置的普通“一键安装”，确认每个 Nexus modId 只出现一个后台下载页，本体页不会重复打开。

## 下一步注意事项
- 如果以后前端 targets 里出现缺失 `modId` 的项目，扩展会退回按 URL 去重；请尽量继续保证 `requiredMods[]` 和当前本体都带真实 Nexus modId。
- `batchId` 幂等只阻止同一批次重复开页；用户重新点击产生新 batchId 时仍会开始新的安装批次。
# NEXUS-EXT-CONNECT-1 前端接手

## 改了什么
- `ModsPage` 在管理员下载页新增扩展连通状态，检测按钮放在“配置 Nexus Key”旁边。点击后会向浏览器扩展发送 `PING`，成功时显示“扩展已连通”，并把普通 Nexus “一键安装”从灰色禁用切回可用。
- `panel-bridge.js` 对 `PING` 做一次特殊放行：即使扩展里仍保存旧面板地址，也允许当前已登录面板页先完成注册。注册前会调用 `GET /api/auth/me`，确认当前页面是面板且用户已登录，再把 `window.location.origin` 和实例 ID `stardew` 写入扩展 background 配置。
- `background.js` 新增 `REGISTER_PANEL` 和 `PING` runtime message；其它安装消息仍维持 origin 校验，不会因为自动注册而放开普通网页调用权限。
- 普通“一键安装”现在依赖扩展连通状态；未连通时禁用并在 tooltip 提示先检测扩展。`N站会员专属安装` 仍只依赖 Nexus Key 配置。
- 检测按钮右侧会直接显示检测中、已连通或未响应错误信息；不要只依赖按钮 title，否则扩展未注入时用户会感觉点击没有反应。
- 连通检测不再只相信扩展回包成功；前端会校验 `config.panelBaseUrl` 的 origin 必须等于当前 `window.location.origin`。如果用户换端口而扩展仍保存旧地址，页面会显示旧地址错误，不会提前标成“已连通”。

## 影响文件
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `browser-extensions/nexus-slow-installer/background.js`
- `browser-extensions/nexus-slow-installer/panel-bridge.js`

## 如何验证
- `node --check browser-extensions/nexus-slow-installer/background.js`
- `node --check browser-extensions/nexus-slow-installer/content.js`
- `node --check browser-extensions/nexus-slow-installer/shared.js`
- `node --check browser-extensions/nexus-slow-installer/panel-bridge.js`
- `cd frontend; npm.cmd run build`
- 手动验证：重新加载浏览器扩展，刷新面板页，点击“检测扩展”。成功后按钮显示“扩展已连通”，普通“一键安装”按钮从禁用变为可点；换面板端口或 IP 后再次点击检测应自动更新扩展里的面板地址。

## 下一步注意
- 如果用户看到“浏览器扩展未响应”，优先确认扩展已重新加载且面板页已刷新。
- 如果 `PING` 失败但面板已登录，检查 `panel-bridge.js` 是否被 manifest 注入当前面板 origin，以及 `/api/auth/me` 是否在当前部署路径下可访问。
# NEXUS-EXT-PACK-1 前端接手记录

## 改了什么
- `ModsPage` 下载页在 `配置 Nexus Key` 右侧新增普通 Nexus 用户扩展安装引导文案。
- 新增 `下载浏览器扩展` 按钮，调用 `downloadNexusInstallerExtension()` 下载后端生成的 `anxi-nexus-installer.zip`。
- 按钮下载中显示 `打包中...`；失败时把错误写入现有 Nexus 安装错误区域。

## 影响文件/接口
- `frontend/src/api.ts`
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 依赖接口：`GET /api/instances/:id/mods/nexus/extension/download`

## 如何验证
- `cd frontend; npm.cmd run build`
- 手动验证：进入 Mods 下载页，点击 `下载浏览器扩展`，浏览器应下载 `anxi-nexus-installer.zip`；解压后目录根部应能看到 `manifest.json`。

## 下一步注意事项
- 下载扩展只解决安装包获取；用户仍需要在 Chrome/Edge 扩展管理页加载解压目录，然后回面板点击 `检测扩展` 同步当前面板地址。
# NEWGAME-PLAYERLIMIT-1 前端接手

## 改了什么

- `NewGameCreator` 在左侧联机设置中新增“联机人数上限”，默认 `10人`，范围 `1-100`，提交字段为 `maxPlayers`。
- “初始联机小屋”仍保持真实小屋数 `0-7`，不再承担总人数上限语义；增加小屋时会自动把人数上限至少推到 `startingCabins + 1`。

## 影响文件/接口

- `frontend/src/games/stardew/NewGameCreator.tsx`
- `frontend/src/types.ts`
- 依赖后端 `POST /api/instances/:id/saves/custom-new-game` 接收 `maxPlayers`。

## 如何验证

- 已执行：`cd frontend; npm.cmd run build`
- 已执行：`cd backend; go test ./internal/games/stardew_junimo -run "WriteServerSettings|ValidateNewGameConfig"`

## 下一步注意事项

- 不要把 `startingCabins` 范围直接放到 100；超过原生 7 个初始小屋的玩家容量应走 Junimo `Server.MaxPlayers` 和 `CabinStack` 自动小屋管理。
# VNC-CONTROL-1 前端接手记录

## 改了什么
- `ServerControlPage` 的“快捷操作”新增 VNC 显示切换按钮；`跳转VNC控制` 默认隐藏，仅在显示渲染打开后出现。
- 服务器运行时会先调用 `GET /api/instances/:id/rendering` 读取当前 Junimo 渲染 FPS，避免刷新页面后把已开启的 VNC 显示误显示成 `打开VNC显示`。
- `打开VNC显示` 调用 `POST /api/instances/:id/rendering`，当前固定传 `fps=15`；成功后按钮显示为 `关闭VNC显示`，再次点击传 `fps=0` 关闭服务端渲染。
- `跳转VNC控制` 读取 `GET /api/instances/:id/config/vnc-port` 返回的 `vncPort`，打开 `http://<当前hostname>:<vncPort>/`；关闭 VNC 显示后该按钮会重新隐藏。

## 影响文件/接口
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/api.ts`
- `frontend/src/types.ts`
- 新接口：`GET/POST /api/instances/:id/rendering`
- 复用接口：`GET /api/instances/:id/config/vnc-port`

## 如何验证
- `cd frontend; npm.cmd run build`
- 手动联调建议：服务器运行后先点 `打开VNC显示`，确认按钮变为 `关闭VNC显示`，再点 `跳转VNC控制`，确认 noVNC 页面能打开并用安装时的 VNC 密码进入控制画面；最后点 `关闭VNC显示` 验证可关闭渲染。

## 下一步注意事项
- VNC 控制页是宿主端口页面，不在 React 内嵌 iframe；如果部署在反向代理/HTTPS 后面，可能需要额外代理 noVNC 端口，否则当前按钮仍会按面板 hostname + 自定义 VNC 端口直连。

# FE-ASSET-LEFT-RAIL-SHELL-1 前端接手记录

## 改了什么
- 新增左侧栏木质背景空壳生产素材：`frontend/public/assets/stardew/ui/panels/panel_side_rail_shell_empty_image2.png`。
- 该素材从 image2 原型 `Left panel.png` 处理而来，清掉九个导航按钮、菜单文字、菜单图标、按钮金边和状态残影，保留外框、深色木纹、横向分隔、底部置物架和装饰区。
- PNG 为 RGBA 透明背景，尺寸 `598x1807`，四周有 4px 透明安全边距，供后续把按钮底图、图标和文本拆成独立层后组合使用。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/panels/panel_side_rail_shell_empty_image2.png`
- 未改接口，未改 React/CSS 引用；当前运行中的左侧栏仍使用既有 `panel_side_rail_image2.png`。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、四角 alpha 为 0、alpha 范围为 `0..255`。
- 已检查导航清理区无亮色中文文字残留、无蓝绿菜单图标残留。
- 已人工预览整图，确认按钮高亮/hover/active 视觉状态不再保留，底部装饰区仍正常。

## 下一步注意事项
- 如果后续把 Shell 切到该空壳素材，需要重新叠加独立导航按钮底图、图标和文字，并根据 `598x1807` 含透明边距的尺寸修正 CSS 定位和热区坐标。
- 若要做真正九宫格/平铺版本，应继续拆出外侧立柱、横向分隔条、底部置物架和独立导航按钮底图；不要直接拉伸整张空壳图。

# FE-ASSET-NAV-BUTTON-DEFAULT-1 前端接手记录

## 改了什么
- 新增默认态左侧导航按钮空底图：`frontend/public/assets/stardew/ui/navigation/nav_item_default_wood_blank_image2.png`。
- 该素材从 `Left panel.png` 的单个导航按钮处理而来，清掉中文菜单文字和图标，保留木质主体、金棕边框、四角像素装饰、内侧阴影和高光。
- PNG 为 RGBA 透明背景，尺寸 `442x138`，四周有 4px 透明安全边距；按钮中央为空木纹，适合叠加独立图标和文字。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/navigation/nav_item_default_wood_blank_image2.png`
- 未改接口，未改 React/CSS 引用；当前导航仍沿用既有侧栏整图和透明热区。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、四角 alpha 为 0。
- 已检查按钮中心区域无中文文字和菜单图标残留。
- 已人工预览单图，确认只保留按钮本体和必要投影，没有携带侧栏背景木板。

## 下一步注意事项
- 后续接入时不要把图标和文字烘焙回按钮底图，应继续保持按钮底图、图标、文字分层。
- 若要支持 hover/active 状态，应单独拆对应状态素材，不要在默认态素材上直接调透明度模拟。

# FE-ASSET-NAV-BUTTON-ACTIVE-1 前端接手记录

## 改了什么
- 新增激活态左侧导航按钮空底图：`frontend/public/assets/stardew/ui/navigation/nav_item_active_wood_blank_image2.png`。
- 该素材基于 `Left panel.png` 按钮风格生成候选后选择 C 版继续抠图，清掉绿幕背景，保留木质主体、亮金色双边框、四角像素高光和轻微暖色发光。
- PNG 为 RGBA 透明背景，尺寸 `442x153`，四周有 4px 透明安全边距；按钮中央为空木纹，适合叠加 React 图标和文字。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/navigation/nav_item_active_wood_blank_image2.png`
- 未改接口，未改 React/CSS 引用；当前导航仍沿用既有侧栏整图和透明热区。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、四角 alpha 为 0、alpha 范围为 `0..255`。
- 已人工预览单图，确认无绿幕背景残留，中心区域没有文字或图标。

## 下一步注意事项
- 接入时注意该素材高度比默认态 `nav_item_default_wood_blank_image2.png` 多 15px，原因是激活态保留更明显的外侧高光和发光边缘；CSS 定位应按中心线对齐，而不是只按左上角贴齐。
- 如果后续要做 hover 态，建议从默认态和激活态之间做一个亮度较低的独立素材，不要直接缩放或半透明叠加激活态。

# FE-ASSET-NAV-BUTTON-HOVER-1 前端接手记录

## 改了什么
- 新增悬停态左侧导航按钮空底图：`frontend/public/assets/stardew/ui/navigation/nav_item_hover_wood_blank_image2.png`。
- 该素材以默认态按钮为主体，并从已选 C 版激活态中克制采样金色边框高光；整体比默认态略亮，但没有激活态的强外发光。
- PNG 为 RGBA 透明背景，尺寸 `442x138`，与默认态完全一致；中央木纹区域留空，不含中文文字、图标或侧栏背景。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/navigation/nav_item_hover_wood_blank_image2.png`
- 未改接口，未改 React/CSS 引用；当前仅入库生产素材。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、尺寸 `442x138`、四角 alpha 为 0、alpha 范围 `0..255`。
- 已人工预览单图，确认按钮中心为空，只有轻微 hover 金色高光，状态弱于 `nav_item_active_wood_blank_image2.png`。

## 下一步注意事项
- 后续接入 hover 时可与默认态按同一盒模型切换；active 态仍需按中心线对齐，因为 active 素材高度为 `442x153`。
- 继续保持按钮底图、图标和文字分层，不要把中文菜单文案或图标烘焙回该 hover 底图。

# FE-ASSET-SIDEBAR-DECOR-PROPS-1 前端接手记录

## 改了什么
- 新增并已重生成左侧栏底部装饰整组素材：`frontend/public/assets/stardew/ui/sprites/sidebar_bottom_decor_props_group_image2.png`。
- 新增并已重生成三个可独立复用的单件素材：`sidebar_decor_lantern_glow_image2.png`、`sidebar_decor_potted_plant_image2.png`、`sidebar_decor_purple_crystal_image2.png`。
- 最新版本直接以 `Left panel.png` 底部装饰区为 image2 参考生成，再用洋红 chroma-key 本地转透明，替换掉首版本地抠图/补边素材。
- 整组素材保留灯笼、盆栽、紫水晶、下层小壶、竖书和书本/盒子的相对位置，可带木架结构；三个单件已移除侧栏背景和其它装饰物。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/sprites/sidebar_bottom_decor_props_group_image2.png`
- `frontend/public/assets/stardew/ui/sprites/sidebar_decor_lantern_glow_image2.png`
- `frontend/public/assets/stardew/ui/sprites/sidebar_decor_potted_plant_image2.png`
- `frontend/public/assets/stardew/ui/sprites/sidebar_decor_purple_crystal_image2.png`
- 未改接口，未改 React/CSS 引用；当前仅入库生产素材。

## 如何验证
- 已用 Pillow 检查 4 个输出均为 `mode=RGBA`、alpha 范围 `0..255`、四角 alpha 为 0、洋红残留为 0。
- 最新尺寸为整组 `720x558`、灯笼 `357x484`、盆栽 `490x531`、紫水晶 `454x541`。
- 已人工预览接触表，确认整组无导航按钮/菜单文字残留；单独灯笼、盆栽和紫水晶不带侧栏背景。

## 下一步注意事项
- 接入左侧栏空壳时，整组素材适合作为底部装饰叠层；如果需要交互或动效，可改用单件素材分别定位。
- 单件素材尺寸分别为灯笼 `357x484`、盆栽 `490x531`、紫水晶 `454x541`，后续不要再把它们与菜单背景合并成一张图。

# FE-ASSET-NAV-ICONS-IMAGE2-1 前端接手记录

## 改了什么
- 新增 9 枚 image2 左侧导航透明图标，参考 `Left panel.png` 的图标语义和造型重绘，避免原图同色木纹抠图把按钮底图带入。
- 新增 3x3 sprite sheet：`frontend/public/assets/stardew/ui/icons/icon_nav_sprite_sheet_3x3_image2.png`，排列顺序为总览、服务器、存档、任务日志、玩家、模组、诊断、安装、设置。
- 所有图标均不含按钮底图、中文文字或侧栏背景；单图按图标主体紧裁并保留透明边距，sheet 为 `1254x1254`，3x3 排列，每格约 `418x418`，图标之间有大面积透明间距。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/icons/icon_nav_sprite_sheet_3x3_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_nav_overview_map_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_nav_server_rack_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_nav_saves_chest_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_nav_tasks_scroll_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_nav_players_avatar_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_nav_mods_crystal_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_nav_diagnostics_monitor_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_nav_install_package_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_nav_settings_gear_image2.png`
- 未改接口，未改 React/CSS 引用；当前仅入库生产素材。

## 如何验证
- 已用 Pillow 检查 10 个输出均为 `mode=RGBA`、alpha 范围 `0..255`、四角 alpha 为 0。
- 已人工预览单图接触表和 sheet 预览，确认生产 sheet 无文字标签，图标之间有透明间距，且没有按钮木框或中文菜单文字残留。

## 下一步注意事项
- 如果后续使用 sprite sheet，按 `1254x1254` 总尺寸和 3x3 等分网格计算背景位置；不要使用临时 QA contact sheet，它只用于人工预览且带标签。
- 接入左侧栏时继续保持按钮底图、图标、文字分层，不要把这些图标烘焙回按钮素材。

# FE-ASSET-RIGHT-RAIL-SHELL-1 前端接手记录

## 改了什么
- 新增右侧栏木质背景空壳生产素材：`frontend/public/assets/stardew/ui/panels/panel_right_rail_shell_empty_image2.png`。
- 该素材基于 image2 原型 `01-overview-right-sidebar-empty-image2.png` 的右侧栏风格重绘，保留外层立柱、完整顶部横梁、深棕木纹内底、金棕边框、藤蔓、底部基座和南瓜/向日葵装饰。
- 已移除原图里的三个内部内容卡片、标题文字、心形/时钟/任务板等图标、绿色状态点、进度条和任务列表内容；素材内部留给前端重新叠加独立卡片和数据层。
- PNG 为 RGBA 透明背景，尺寸 `826x1903`；第二版已修正首版顶部横梁中间透明缺口。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/panels/panel_right_rail_shell_empty_image2.png`
- 该素材已在 `FE-RIGHT-RAIL-SPLIT-ASSETS-1` 中接入运行时，作为右侧栏背景空壳层使用。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、尺寸 `826x1903`、四角 alpha 为 0、alpha 范围 `0..255`。
- 已检查无洋红 chroma-key 残留，原先顶部缺口所在横向区域整段 alpha 可见。
- 已人工预览确认无文字、图标、状态点、进度条或卡片框残留。

## 下一步注意事项
- 后续接入该空壳时，需要把系统健康、进行中、近期任务三个卡片框拆成独立素材或 CSS 层，再重新定位文字、图标、状态点和进度条。
- 可变尺寸区域不要直接拉伸整张空壳；继续拆出九宫格卡片框、标题图标、进度条组件和底部装饰等分层资产。

# FE-ASSET-RIGHT-RAIL-BORDER-1 前端接手记录

## 改了什么
- 新增右侧栏外层木质边框生产素材：`frontend/public/assets/stardew/ui/panels/panel_right_rail_outer_border_image2.png`。
- 该素材基于 image2 原型右侧栏风格重绘，只保留最外侧左右竖梁、顶部边缘、底部边缘、金棕木质雕刻、像素阴影和外框藤蔓点缀。
- 中间区域完全透明；已移除内部卡片框、内部卡片角落藤蔓、标题文字、图标、状态点、进度条、列表内容，以及底部南瓜/向日葵装饰。
- PNG 为 RGBA 透明背景，尺寸 `920x1710`，适合作为 CSS 最上层覆盖边框或背景层外框。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/panels/panel_right_rail_outer_border_image2.png`
- 该素材已在 `FE-RIGHT-RAIL-SPLIT-ASSETS-1` 中接入运行时，作为右侧栏外框覆盖层使用。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、尺寸 `920x1710`、四角 alpha 为 0、alpha 范围 `0..255`。
- 中心、上中、下中采样 alpha 均为 0，中心区域批量采样最大 alpha 为 0。
- 已检查无洋红 chroma-key 残留；人工预览确认没有文字、图标、进度条、卡片框或底部装饰残留。

## 下一步注意事项
- 该素材只负责外框覆盖，不应承载内部底板、卡片背景、标题图标、进度条或底部南瓜/向日葵装饰。
- 后续接入时可与 `panel_right_rail_shell_empty_image2.png`、独立卡片框和底部装饰层组合；不要把动态文字和状态数据烘焙回边框图。

# FE-ASSET-RIGHT-RAIL-CARDS-1 前端接手记录

## 改了什么
- 新增右侧栏三张卡片空框生产素材：`frontend/public/assets/stardew/ui/panels/panel_right_rail_card_health_empty_image2.png`、`frontend/public/assets/stardew/ui/panels/panel_right_rail_card_in_progress_empty_image2.png`、`frontend/public/assets/stardew/ui/panels/panel_right_rail_card_recent_tasks_empty_image2.png`。
- 三张素材分别对应顶部“系统健康”大卡、中部“进行中”卡和底部“近期任务”卡，只保留木质边框、深棕内容底、金棕角饰、藤蔓点缀和像素阴影。
- 已移除标题文字、红心/时钟/任务板图标、CPU/内存/磁盘/在线玩家/网络延迟文字、绿色状态点、进度条、“查看详情”文字和箭头、内部横线、任务列表和其它动态内容。
- 卡片外部透明，内部保留干净深棕木纹/皮革质感，便于前端叠加标题、指标、按钮、进度条和列表。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/panels/panel_right_rail_card_health_empty_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_right_rail_card_in_progress_empty_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_right_rail_card_recent_tasks_empty_image2.png`
- 该组固定尺寸空框目前保留为备用；运行时优先使用 `*_nineslice_image2.png` 九宫格卡片框。

## 如何验证
- 已用 Pillow 检查三张素材均为 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`。
- 尺寸分别为健康卡 `1088x1446`、进行中卡 `1604x981`、近期任务卡 `1464x1075`。
- 三张素材中心 alpha 均为 255，中心内容底保留深棕纹理；洋红 chroma-key 残留为 0。
- 已人工预览确认无文字、图标、状态点、进度条、内部横线或列表残影。

## 下一步注意事项
- 后续接入时应保持右侧栏空壳、外层边框、三张卡片框、标题图标、进度条、文字和底部装饰分层，不要重新合并成整张右侧栏截图。
- 三张卡片当前是位图空框；如果要做真正可变尺寸版本，应继续拆九宫格边框、角标和中心纹理。

# FE-ASSET-RIGHT-RAIL-CARDS-NINESLICE-1 前端接手记录

## 改了什么
- 新增右侧栏三张九宫格友好的卡片框生产素材：`frontend/public/assets/stardew/ui/panels/panel_right_rail_card_health_nineslice_image2.png`、`frontend/public/assets/stardew/ui/panels/panel_right_rail_card_in_progress_nineslice_image2.png`、`frontend/public/assets/stardew/ui/panels/panel_right_rail_card_recent_tasks_nineslice_image2.png`。
- 三张素材分别对应顶部系统健康大卡、中部进行中卡和底部近期任务卡；四角像素装饰完整，角落藤蔓集中在角区，上下边框和左右边框保留较长直线重复段，适合后续 `border-image` 或九宫格裁切。
- 中间内容区保留干净深棕木纹/皮革纹理；已移除所有文字、图标、状态点、进度条、内部横线、列表、标签和参考线。
- 该组与 `panel_right_rail_card_*_empty_image2.png` 区分：`nineslice` 版本优先用于可变尺寸卡片，普通 empty 版本优先用于固定尺寸叠图。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/panels/panel_right_rail_card_health_nineslice_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_right_rail_card_in_progress_nineslice_image2.png`
- `frontend/public/assets/stardew/ui/panels/panel_right_rail_card_recent_tasks_nineslice_image2.png`
- 该组已在 `FE-RIGHT-RAIL-SPLIT-ASSETS-1` 中通过 CSS `border-image` 接入运行时，作为三张右栏卡片框。

## 如何验证
- 已用 Pillow 检查三张素材均为 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`。
- 尺寸分别为健康卡 `1403x1121`、进行中卡 `1693x929`、近期任务卡 `1534x1025`。
- 三张素材中心 alpha 均为 255，中心内容底保留深棕纹理；洋红 chroma-key 残留为 0。
- 透明安全边距分别约为健康卡 `104/93/104/131`、进行中卡 `100/119/99/134`、近期任务卡 `62/67/62/59`（左/上/右/下）。
- 已人工预览确认无文字、图标、状态点、进度条、内部横线或列表残影，边框中段规则。

## 下一步注意事项
- 接入时需要在 CSS 里明确九宫格 slice 值，避开四角藤蔓和角饰，只让直线边框段参与平铺或拉伸。
- 中心纹理可作为卡片背景铺底；动态标题、指标、进度条、按钮和任务列表仍由前端单独渲染。

# FE-ASSET-RIGHT-RAIL-TITLE-ICONS-1 前端接手记录

## 改了什么
- 新增右侧栏三枚标题图标生产素材：`frontend/public/assets/stardew/ui/icons/icon_right_rail_health_heart_image2.png`、`frontend/public/assets/stardew/ui/icons/icon_right_rail_in_progress_clock_image2.png`、`frontend/public/assets/stardew/ui/icons/icon_right_rail_recent_tasks_clipboard_image2.png`。
- 三枚素材分别对应 image2 右侧栏里的系统健康红心、进行中蓝色时钟和近期任务剪贴板；只保留图标本体、像素描边、阴影和高光。
- 已移除所有中文文字、卡片框背景、右侧栏背景、进度条、状态点和列表内容，适合前端作为标题图标分层叠加。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/icons/icon_right_rail_health_heart_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_right_rail_in_progress_clock_image2.png`
- `frontend/public/assets/stardew/ui/icons/icon_right_rail_recent_tasks_clipboard_image2.png`
- 该组三枚图标已在 `FE-RIGHT-RAIL-SPLIT-ASSETS-1` 中接入运行时，标题文字仍由 React 渲染。

## 如何验证
- 已用 Pillow 检查三枚图标均为 `mode=RGBA`、四角 alpha 为 0、alpha 范围 `0..255`。
- 尺寸分别为红心 `776x680`、蓝色时钟 `864x940`、剪贴板 `714x934`。
- 三枚图标内容 bbox 四边距均为 4px，洋红 chroma-key 残留为 0。
- 已人工预览确认图标不带中文文字、卡片框背景或右侧栏背景。

## 下一步注意事项
- 接入时继续保持图标、标题文字、卡片框和动态数据分层，不要把标题或状态数据烘焙回图标。
- 如果需要统一标题图标视觉尺寸，建议在 CSS 中用固定盒子和 `object-fit: contain` 控制显示尺寸，而不是重采样覆盖原 PNG。

# FE-ASSET-TOP-BAR-SHELL-1 前端接手记录

## 改了什么
- 新增顶栏整体木质背景空壳生产素材：`frontend/public/assets/stardew/ui/panels/panel_top_bar_shell_empty_image2.png`。
- 该素材基于 image2 原型 `Top bar.png` 风格处理，只保留深棕木纹横栏、金棕像素边框、四角装饰、像素阴影和高光。
- 已移除鸡图标、`Stardew Anxi Panel` 品牌字、状态徽章、农场选择框、版本框、用户角色框、登出按钮以及所有槽位图标/文字；内部木纹留空给前端分层叠加。
- PNG 为 RGBA 透明背景，尺寸 `2137x170`；原顶栏主体按 `2129x162` 对齐，四周额外保留 4px 透明安全边距。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/panels/panel_top_bar_shell_empty_image2.png`
- 未改接口，未改 React/CSS 引用；当前顶栏仍沿用既有 `panel_top_bar_image2.png`。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、尺寸 `2137x170`、四角 alpha 为 0、alpha 范围 `0..255`。
- 已检查无绿幕和白底残留；人工预览确认无品牌字、按钮、选择框、状态徽章、图标或文字残影。

## 下一步注意事项
- 后续接入时需要把品牌区、状态徽章、农场槽、版本槽、角色槽、登出按钮、图标和文字全部作为独立层叠加，不要再烘焙回顶栏背景。
- 该文件带 4px 安全边距；如果替换当前 `panel_top_bar_image2.png`，应同步调整 CSS 尺寸、定位和热区坐标。
- 可变宽区域后续仍建议继续拆成九宫格/横向平铺素材，不要直接拉伸整条顶栏空壳。

# FE-ASSET-TOP-BAR-CORNERS-1 前端接手记录

## 改了什么
- 新增顶栏四角像素装饰单件素材：`topbar_corner_top_left_image2.png`、`topbar_corner_top_right_image2.png`、`topbar_corner_bottom_left_image2.png`、`topbar_corner_bottom_right_image2.png`。
- 新增 2x2 无标签 sprite sheet：`topbar_corner_ornaments_sprite_sheet_2x2_image2.png`，排列顺序为左上、右上、左下、右下。
- 素材基于 image2 顶栏风格重绘，只保留金棕木质/金属角标、像素暗边、阴影和高光；没有整条顶栏背景、木纹底板、文字、按钮、图标、状态徽章或下拉槽位。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/sprites/topbar_corner_top_left_image2.png`
- `frontend/public/assets/stardew/ui/sprites/topbar_corner_top_right_image2.png`
- `frontend/public/assets/stardew/ui/sprites/topbar_corner_bottom_left_image2.png`
- `frontend/public/assets/stardew/ui/sprites/topbar_corner_bottom_right_image2.png`
- `frontend/public/assets/stardew/ui/sprites/topbar_corner_ornaments_sprite_sheet_2x2_image2.png`
- 未改接口，未改 React/CSS 引用；当前顶栏仍沿用既有整图素材。

## 如何验证
- 已用 Pillow 检查 5 个输出均为 `mode=RGBA`、alpha 范围 `0..255`、四角 alpha 为 0。
- 单件尺寸：左上/右上 `104x88`，左下/右下 `104x82`；sprite sheet 尺寸 `224x192`。
- 已检查无绿幕和白底残留；人工预览确认 sheet 无文字标签，角标不带按钮、图标、状态徽章或整条顶栏背景。

## 下一步注意事项
- 后续接入顶栏空壳或九宫格边框时，这 4 个角标应作为独立装饰层定位，不要合并进按钮、文字或动态状态层。
- 如果使用 sprite sheet，按 2x2 排列和透明间距取图；如果做响应式九宫格，优先使用四个单件而不是缩放整张顶栏。

# FE-ASSET-TOP-BAR-CHICKEN-1 前端接手记录

## 改了什么
- 新增顶栏左侧品牌鸡图标素材：`frontend/public/assets/stardew/ui/icons/icon_topbar_chicken_image2.png`。
- 该素材基于 image2 原型 `Top bar.png` 的左侧鸡图标重绘，只保留鸡本体：白色/奶油色羽毛、红色鸡冠、黄色喙、橙色脚、暗色像素描边、像素阴影和高光。
- 已移除 `Stardew Anxi Panel` 文字、顶栏木质背景、按钮、徽章和其它 UI 元素。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/icons/icon_topbar_chicken_image2.png`
- 未改接口，未改 React/CSS 引用；当前顶栏仍沿用既有整图素材。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、尺寸 `92x104`、四角 alpha 为 0、alpha 范围 `0..255`。
- alpha 主体 bbox 为 `(4, 4, 88, 100)`，四周保留 4px 透明安全边距。
- 已检查无绿幕和白底残留；人工预览确认没有品牌文字或木质顶栏背景。

## 下一步注意事项
- 后续接入顶栏空壳时，该鸡图标应作为独立品牌层定位，品牌文字继续由前端渲染，不要把文字重新烘焙进图标。
- `icon_sidebar_chicken.png` 是既有侧栏/旧资源，`icon_topbar_chicken_image2.png` 专用于 image2 顶栏品牌区，后续不要互相覆盖。

# FE-ASSET-TOP-BAR-BRAND-GLOW-1 前端接手记录

## 改了什么
- 新增顶栏品牌文字下方的暖黄色像素发光/阴影占位素材：`frontend/public/assets/stardew/ui/sprites/topbar_brand_text_glow_placeholder_image2.png`。
- 该素材只保留非字形的微弱像素光带和底部暖色阴影；不包含 `Stardew Anxi Panel` 实际文字、不包含鸡图标、不包含木质顶栏背景。
- 目的是让前端动态渲染品牌文字时可以叠一层更接近 image2 原图的浅色底光。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/sprites/topbar_brand_text_glow_placeholder_image2.png`
- 未改接口，未改 React/CSS 引用；当前顶栏仍沿用既有整图素材。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、尺寸 `468x78`、四角 alpha 为 0、alpha 范围 `0..18`。
- alpha 主体 bbox 为 `(12, 27, 457, 66)`，外缘保持透明。
- 已检查无绿幕和白底残留；人工预览确认没有任何可读字形、鸡图标或木质顶栏背景。

## 下一步注意事项
- 后续接入时将该图放在品牌文字层下方，文字本身仍由 CSS/Canvas/DOM 渲染。
- 如果实际字体描边、阴影和 `text-shadow` 已经足够接近原图，可以不启用该占位图；不要把品牌文字烘焙进这张素材。

# FE-ASSET-FARM-SELECT-FRAME-1 前端接手记录

## 改了什么
- 新增顶栏农场选择框空底图：`frontend/public/assets/stardew/ui/fields/field_topbar_farm_select_empty_image2.png`。
- 该素材基于 image2 原型 `Top bar.png` 的农场选择框提取并重绘，只保留金棕像素边框、暗棕木纹内容底、内侧像素阴影和下拉框结构。
- 已移除农场图标、农场名文字、右侧下拉箭头和顶栏背景，内容区保持为空，供前端叠加图标、文本和箭头。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/fields/field_topbar_farm_select_empty_image2.png`
- 未改接口，未改 React/CSS 引用；当前顶栏仍沿用既有整图素材。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、尺寸 `456x132`、四角 alpha 为 0、alpha 范围 `0..255`。
- alpha 主体 bbox 为 `(28, 8, 437, 121)`，外部为透明背景。
- 已检查无绿幕和白底残留；人工预览确认没有农场图标、文字、右侧箭头或顶栏背景残影。

## 下一步注意事项
- 固定宽度农场选择框可以直接使用该空底图；如果农场名长度变化较大，优先使用三段式素材拼接。
- 农场图标、农场名和下拉箭头都应作为独立前端层叠加，不要回烘进空底图。

# FE-ASSET-FARM-SELECT-3PIECE-1 前端接手记录

## 改了什么
- 新增农场选择框三段式透明素材：`field_topbar_farm_select_left_cap_image2.png`、`field_topbar_farm_select_center_tile_image2.png`、`field_topbar_farm_select_right_cap_image2.png`。
- 新增横向无标签 sprite sheet：`frontend/public/assets/stardew/ui/fields/field_topbar_farm_select_3piece_sheet_image2.png`，排列顺序为左端、中段、右端，段间保留 16px 透明间距。
- 左/右端保留金棕角部边框、像素阴影和高光；中段提供可横向平铺的暗棕木纹内容区和上下边框。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/fields/field_topbar_farm_select_left_cap_image2.png`
- `frontend/public/assets/stardew/ui/fields/field_topbar_farm_select_center_tile_image2.png`
- `frontend/public/assets/stardew/ui/fields/field_topbar_farm_select_right_cap_image2.png`
- `frontend/public/assets/stardew/ui/fields/field_topbar_farm_select_3piece_sheet_image2.png`
- 未改接口，未改 React/CSS 引用；当前顶栏仍沿用既有整图素材。

## 如何验证
- 已用 Pillow 检查单件尺寸：左端 `96x132`、中段 `64x132`、右端 `96x132`，sprite sheet `288x132`。
- 4 个文件均为 `mode=RGBA`，四角 alpha 为 0，alpha 范围 `0..255`。
- 已检查无绿幕和白底残留；人工预览确认三段和 sheet 无文字、图标、箭头或标签。

## 下一步注意事项
- 可变宽度农场选择框推荐用左端 + 中段 repeat-x/stretch + 右端组合，避免直接横向拉伸整张空底图造成角部变形。
- 中段不含垂直角部，适合横向平铺；叠加农场图标、农场名和箭头时需要由前端单独定位。

# FE-ASSET-DROPDOWN-ARROW-1 前端接手记录

## 改了什么
- 新增顶栏下拉箭头图标：`frontend/public/assets/stardew/ui/icons/icon_dropdown_arrow_gold_image2.png`。
- 该素材基于 image2 原型 `Top bar.png` 的农场选择框/用户框箭头重绘，只保留浅金/黄色像素箭头、暗色描边和阴影。
- 已移除农场选择框背景、用户框背景、文字和其它 UI 元素。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/icons/icon_dropdown_arrow_gold_image2.png`
- 未改接口，未改 React/CSS 引用；当前顶栏仍沿用既有整图素材。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、尺寸 `42x32`、四角 alpha 为 0、alpha 范围 `0..255`。
- alpha 主体 bbox 为 `(6, 7, 38, 28)`。
- 已检查无绿幕和白底残留；人工预览确认没有农场框、用户框或文字残影。

## 下一步注意事项
- 后续接入时可复用于农场选择框和用户菜单框；箭头应作为独立 icon 层定位，不要合并进空底图。

# FE-ASSET-VERSION-BADGE-FRAME-1 前端接手记录

## 改了什么
- 新增顶栏版本号小框空底图：`frontend/public/assets/stardew/ui/fields/field_topbar_version_badge_empty_image2.png`。
- 该素材基于 image2 原型 `Top bar.png` 的版本框风格重绘，只保留棕色/金色像素边框、暗木纹内部、像素阴影和高光。
- 已移除版本号文字和顶栏背景。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/fields/field_topbar_version_badge_empty_image2.png`
- 未改接口，未改 React/CSS 引用；当前顶栏仍沿用既有整图素材。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、尺寸 `228x116`、四角 alpha 为 0、alpha 范围 `0..255`。
- alpha 主体 bbox 为 `(8, 8, 214, 110)`。
- 已检查无绿幕和白底残留；人工预览确认没有版本文字或顶栏背景残影。

## 下一步注意事项
- 后续接入时版本号文字继续由前端渲染；如果版本字符串变长，可基于该素材再派生三段式版本框。

# FE-ASSET-USER-ROLE-FRAME-1 前端接手记录

## 改了什么
- 新增顶栏用户角色框空底图：`frontend/public/assets/stardew/ui/fields/field_topbar_user_role_empty_image2.png`。
- 该素材基于 image2 原型 `Top bar.png` 的用户框风格重绘，只保留木质/金色边框、暗棕内容底、像素阴影和高光。
- 已移除人物头像、`管理员` 等角色文字、下拉箭头和顶栏背景。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/fields/field_topbar_user_role_empty_image2.png`
- 未改接口，未改 React/CSS 引用；当前顶栏仍沿用既有整图素材。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、尺寸 `308x116`、四角 alpha 为 0、alpha 范围 `0..255`。
- alpha 主体 bbox 为 `(7, 8, 297, 110)`。
- 已检查无绿幕和白底残留；人工预览确认没有头像、角色文字、下拉箭头或顶栏背景残影。

## 下一步注意事项
- 固定宽度用户框可以直接使用该空底图；如果角色文案或本地化文案长度变化较大，优先使用三段式素材拼接。
- 头像、角色文字和下拉箭头都应作为独立前端层叠加，不要回烘进空底图。

# FE-ASSET-USER-ROLE-3PIECE-1 前端接手记录

## 改了什么
- 新增用户角色框三段式透明素材：`field_topbar_user_role_left_cap_image2.png`、`field_topbar_user_role_center_tile_image2.png`、`field_topbar_user_role_right_cap_image2.png`。
- 新增横向无标签 sprite sheet：`frontend/public/assets/stardew/ui/fields/field_topbar_user_role_3piece_sheet_image2.png`，排列顺序为左端、中段、右端，段间保留 16px 透明间距。
- 左/右端保留角部边框、像素阴影和高光；中段提供可横向平铺的暗棕木纹内容区和上下边框。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/fields/field_topbar_user_role_left_cap_image2.png`
- `frontend/public/assets/stardew/ui/fields/field_topbar_user_role_center_tile_image2.png`
- `frontend/public/assets/stardew/ui/fields/field_topbar_user_role_right_cap_image2.png`
- `frontend/public/assets/stardew/ui/fields/field_topbar_user_role_3piece_sheet_image2.png`
- 未改接口，未改 React/CSS 引用；当前顶栏仍沿用既有整图素材。

## 如何验证
- 已用 Pillow 检查单件尺寸：左端 `80x116`、中段 `64x116`、右端 `80x116`，sprite sheet `256x116`。
- 4 个文件均为 `mode=RGBA`，四角 alpha 为 0，alpha 范围 `0..255`。
- 已检查无绿幕和白底残留；人工预览确认三段和 sheet 无头像、文字、箭头或标签。

## 下一步注意事项
- 可变宽度用户框推荐用左端 + 中段 repeat-x/stretch + 右端组合，避免直接横向拉伸整张空底图造成角部变形。
- 中段不含头像或箭头，叠加头像、角色文字和箭头时需要由前端单独定位。

# FE-ASSET-TOP-BAR-USER-AVATAR-1 前端接手记录

## 改了什么
- 新增顶栏右侧用户头像图标素材：`frontend/public/assets/stardew/ui/icons/icon_topbar_user_avatar_image2.png`。
- 该素材基于 image2 原型 `Top bar.png` 的用户框头像提取并重绘，只保留橙色头发、肤色脸部、蓝色衣服、暗色像素描边和高光。
- 已移除用户框背景、`管理员` 文字和下拉箭头。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/icons/icon_topbar_user_avatar_image2.png`
- 未改接口，未改 React/CSS 引用；当前顶栏仍沿用既有整图素材。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、尺寸 `59x73`、四角 alpha 为 0、alpha 范围 `0..255`。
- alpha 主体 bbox 为 `(4, 4, 55, 69)`，四周保留 4px 透明安全边距。
- 已检查无绿幕和白底残留；人工预览确认没有用户框、文字或箭头残影。

## 下一步注意事项
- 后续接入用户角色框时，该头像应作为独立 icon 层定位；如未来支持用户自定义头像，可把该素材作为默认角色图标。

# FE-ASSET-LOGOUT-BUTTON-FRAME-1 前端接手记录

## 改了什么
- 新增顶栏红色登出按钮空底图：`frontend/public/assets/stardew/ui/buttons/button_topbar_logout_empty_image2.png`。
- 该素材基于 image2 原型 `Top bar.png` 的登出按钮风格重绘，只保留红色按钮底、暗红/金棕像素边框、像素阴影、高光和按键质感。
- 已移除登出图标、`登出` 文字、顶栏背景和右侧角饰残影。

## 影响文件/接口
- `frontend/public/assets/stardew/ui/buttons/button_topbar_logout_empty_image2.png`
- 未改接口，未改 React/CSS 引用；当前顶栏仍沿用既有整图素材。

## 如何验证
- 已用 Pillow 检查输出 `mode=RGBA`、尺寸 `224x116`、四角 alpha 为 0、alpha 范围 `0..255`。
- alpha 主体 bbox 为 `(7, 8, 213, 110)`。
- 已检查无绿幕和白底残留；人工预览确认没有登出图标、文字或顶栏角饰残影。

## 下一步注意事项
- 后续接入时登出图标和 `登出` 文字继续由前端渲染；hover/active 状态可基于该底图再派生，不要直接把带文字版本作为状态图。

# FE-SIDEBAR-BOTTOM-ART-CLIP-FIX-1 前端接手记录

## 改了什么
- 修复窗口变矮时左侧栏"设置"按钮出现底部素材割裂的问题：原 `--sd-sidebar-bottom-content-space` 的固定像素封顶（`clamp(84px, 12vh, 132px)`）小于底部装饰图 `panel_side_rail_bottom_image2.png` 的实际渲染高度（`100cqi * 409 / 598`，宽 210px 时约 144px），按钮列表 `.sd-nav-list`（`overflow-y: auto`）的下边界会侵入底图区域，最后一项"设置"被硬裁切，切口下露出书架桌面画。其它按钮压住的是无缝平铺的中段木纹，所以看不出接缝。
- **关键陷阱（首次修复翻车原因）**：`container-type: inline-size` 声明在 `.sd-sidebar` 上，`cqi` 单位只相对祖先容器解析，在 `.sd-sidebar` 自身的 padding 里用 `var(--sd-sidebar-bottom-art-height)`（内含 `100cqi`）会回退成视口宽度，padding-bottom 变成约 1300px，所有按钮直接消失。`::before`/`::after` 是后代，同一变量在它们上面一直解析正确。
- 最终实现：删除 `--sd-sidebar-bottom-content-space` 变量，`.sd-sidebar` padding-bottom 置 0；底部预留改放到 `.sd-nav-list` 的 `margin-bottom: var(--sd-sidebar-bottom-art-height)`（后代中 cqi 正确等于侧栏宽度，与 `::after` 底图高度精确一致）；移动端（≤640px 横向导航）媒体查询里 `.sd-sidebar .sd-nav-list` 补 `margin-bottom: 0`。
- 已回退的尝试：曾把预留空间减到 `calc(100cqi * 361 / 598)`（361 = 409 − 48），让裁切线落到灯笼装饰上沿。依据是用 System.Drawing 逐行扫描底图：顶部 0-48 行是与中段平铺一致的空白木板（最大饱和度 ~80/亮度 ~110），灯笼装饰从第 50 行开始（饱和度跳到 150+）。用户看过实际效果后要求回退，现为整图高度方案；如需重试该方向可直接复用此扫描结论。

## 影响文件
- `frontend/src/games/stardew/StardewPanel.css`

## 如何验证
- `cd frontend; npm.cmd run build`
- 运行时验证（已执行）：用生产构建 CSS + 真实侧栏 DOM 与素材搭建隔离页，无头 Edge 截图。注意视口宽度必须 >640px，否则触发移动端媒体查询。结果：1280×900 下 9 个按钮全部可见、"设置"完整停在底图上沿；1280×560 和 1280×360 滚动到底后"设置"完整、无割裂；把旧 padding 方案叠回去可复现"按钮全部消失"，证明验证方法能抓到该类回归。
- 手动验证：登录面板后拖矮浏览器窗口，观察最后一个按钮"设置"。

## 下一步注意事项
- 以后凡是给 `.sd-sidebar` 自身写样式，不要使用 `cqi`/`cqw` 单位（它是容器本身，单位会回退到视口宽度）；只有它的后代（含伪元素）能用 `cqi` 取侧栏宽度。
- 矮窗口下导航可用高度比旧版少约 10-30px、更早出现滚动。如果之后想给按钮多留空间，不要恢复固定像素封顶，应让 `::after` 底图渲染高度与列表 margin 使用同一变量同步缩小，保证两者始终相等。

# FE-RIGHT-RAIL-PROTO-GEOMETRY-2 前端接手记录

## 改了什么
- 按原型 `01-overview-right-sidebar-empty-image2.png` 重做右栏外壳与卡片几何，替代旧的 `--sd-opsrail-endcap-scale: 1.08` / `121%` / `-103px` 魔法数字方案。所有映射基于 System.Drawing 实测的素材内容包围盒：
  - 顶封头 `right_rail_shell_top.png` 1842x854，内容 x58..1782（宽1725）/y104..468；底封头 `right_rail_shell_bottom.png` 1871x840，内容 x66..1808（宽1743）/下边距149。`::before`/`::after` 按透明边距负偏移外扩（如 `top: calc(-100cqi * 104 / 1725)`），横梁顶边贴 top:0、木槽底边贴 bottom:0。
  - 三素材立柱金色带映射位置实测误差 ≤1px，无需额外对齐补偿。
- 新增裁切素材 `right_rail_shell_middle_tile_seamless.png`（取原 `right_rail_shell_middle_tile.png` 的 x130..1406/y27..1005）。根因：原图顶 27 行、底 18 行是纯黑（亮度 0），`repeat-y` 每次衔接形成约 14px 横向黑带横穿左右立柱——即"左右边框中间割裂"。原素材文件保留未动。
- 卡片九宫格切片按实测透明边距重调（原 active 顶切片 142 但透明边 140、recent 顶切片 104 < 透明边 126，木框被切进中心拉伸区，三框显示不一致）：
  - health（1053x1494，边距 L62 T56 R64 B49）：slice `126 126 156 126`，border-width `23 26 19 26`，margin `-10 -13 -6 -13`
  - active（1693x929，边距 L90 T140 R90 B178）：slice `175 150 220 150`，border-width `65 33 68 33`，margin `-52 -20 -55 -20`
  - recent（1535x1025，边距 L112 T126 R112 B136）：slice `195 178 185 178`，border-width `37 35 49 35`，margin `-24 -22 -36 -22`
  - 公式：每边 `border-width = 13px × slice / (slice − margin)`（可见框厚约 13px），负 margin 吃掉透明边距在渲染中的像素数，使三卡可见框与栅格单元对齐、视觉等宽等框厚。
- 卡片 `background-clip: padding-box`（消除"阴影遮罩"：负 margin 后 border-box 大于可见框，背景从边框图透明边距漏出成暗色矩形；旧规则 `.sd-opsrail-section` 的半透明背景同样受此裁剪约束）、`overflow: hidden`、`border-image-repeat: stretch`（round 在中心填充区产生拼接缝）。
- 三卡等高同步缩放：`grid-template-rows: repeat(3, minmax(140px, 1fr))`；stack `height: calc(100% - 100cqi * 143 / 1743)`（底部停在木槽上沿），padding 顶部 `calc(100cqi * 128 / 1725 + clamp(18px, 2.6vh, 28px))`（避开横梁后再留呼吸间距——间距太小时健康卡上框会顶进 z3 横梁底下被盖住，只露角托，视觉上像卡片被切进梁里）、左右 `calc(100cqi * 92 / 1277)`（对齐立柱内沿）。

## 影响文件
- `frontend/src/games/stardew/StardewPanel.css`（`.sd-opsrail` 外壳与卡片块整体重写）
- 新增 `frontend/public/assets/stardew/ui/panels/right_rail_shell_middle_tile_seamless.png`

## 如何验证
- `cd frontend; npm.cmd run build`
- 运行时验证（已执行）：生产 CSS + 真实右栏 DOM/素材隔离页，无头 Edge 于 1280×940 和 1280×660 截图：横梁到顶、木槽到底、立柱连续无黑带、三卡等高且随窗口高度同步伸缩、无阴影遮罩、卡片内部无拼接缝、南瓜/向日葵压底卡右下角。
- 手动验证：登录面板看 overview 右栏，拖动窗口高度确认三卡同步缩放。

## 下一步注意事项
- ⚠️ 有并行会话同时在改右栏（其 `FE-RIGHT-RAIL-PROTOTYPE-ALIGN-1` 的右栏列宽 `clamp(340px, 27vw, 430px)` 已保留兼容）。若后续右栏又出现缝隙/黑带/遮罩，先检查 `.sd-opsrail-bg` 是否被改回原始 tile、`::before`/`::after` 偏移或卡片 slice/border-width/margin 是否被改动。
- 切片、border-width、负 margin 三者按上面公式联动，单独改任何一个都会破坏对齐；若更换卡片素材，先量透明边距再重算。
- `.sd-opsrail` 自身不要用 `cqi` 单位（容器查询不了自己，会回退视口宽度）；只有 `.sd-opsrail-bg`、`::before`/`::after`、`.sd-opsrail-stack` 等后代可用。

# FE-TOPBAR-SINGLE-SHELL-1 前端接手记录

## 改了什么
- 修复顶栏左中右割裂：三段拼接素材（`topbar_shell_left.png` / `topbar_shell_middle_tile.png` / `topbar_shell_right.png`）的中段金轨位置、粗细和木纹色调与左右端帽不一致，且端帽带不同透明边距，几何和色调都对不上。
- `.sd-topbar-bg` 改用整幅 `topbar-shell.png`（2137x170，System.Drawing 实测内容 bbox (8,6)-(2128,163)，内容高 158）做左右九宫格：`border-image-slice: 0 130 fill`、`border-image-repeat: stretch`、`border-width: 0 calc(var(--sd-topbar-height) * 130 / 158)`。左右 130px 角饰带按条高等比渲染，中段只做横向拉伸——单图九宫格从结构上保证无接缝。
- 四边负偏移 `-6/158`、`-8/158` × 条高吃掉素材透明安全边，金框贴合顶栏边缘；`.sd-topbar-bg-left/mid/right` 三个子 span 改 `display: none`（TSX DOM 未动，方便以后回退）。

## 影响文件
- `frontend/src/games/stardew/StardewPanel.css`（`.sd-topbar-bg` 块）

## 如何验证
- `cd frontend; npm.cmd run build`
- 运行时验证（已执行）：生产 CSS + 顶栏 DOM 隔离页，无头 Edge 于 2552px 和 1280px 宽视口截图，顶栏四角雕花完整、上下金轨全程连贯、无左中右接缝。
- 手动验证：刷新面板，把窗口从窄拖到宽，观察顶栏金轨是否始终连续。

## 下一步注意事项
- ⚠️ 并行会话在同步迭代顶栏（曾把三段列宽从 620/170 改 190/170）；本方案已弃用三段拼接，若那边继续按三段思路调整会互相覆盖，需要先对齐方案。
- 若以后要给顶栏换素材，优先继续用"整幅 + 左右九宫格"结构；如必须三段拼接，三张素材必须同源导出（同一金轨 y 位置/粗细/色调、无各自透明边距），否则接缝无法靠 CSS 消除。
- 中段横向拉伸幅度 = (视口宽 − 2×角饰带宽) / (2137 − 260)，在 1280~2560px 视口范围内约 0.6~1.3 倍，木纹拉伸不可感知；若未来出现超宽屏拉花，可把 `border-image-repeat` 第一个值改 `round`（只影响水平边，不会产生右栏卡片那种中心缝）。

# FE-TOPBAR-LEFT-CAP-SEAM-1 前端接手记录

## 改了什么
- 顶栏当前实际运行的是三段拼接方案（上面 FE-TOPBAR-SINGLE-SHELL-1 的整幅九宫格方案已被并行会话改回三段），左段 `topbar_shell_left.png`（190x170）仍是旧版深色封闭边框风格，与 image2 中段/右段割裂。
- 用 `topbar_shell_right.png`（360x170）水平镜像重新生成 `topbar_shell_left.png`，满足本文件上一节"三段素材必须同源导出"的要求；旧图未入库 git，覆盖前已在会话 scratchpad 备份。
- `.sd-topbar-bg` 左列宽 `190/170` → `360/170`；640px 以下媒体查询左列 `134px` → `110px`（52px 条高下等比宽度，与右列一致；原 134px 会在左段图右侧留约 76px 透明空档）。

## 影响文件
- `frontend/src/games/stardew/StardewPanel.css`（`.sd-topbar-bg` 桌面列宽 + 640px 媒体查询）
- `frontend/public/assets/stardew/ui/topbar/topbar_shell_left.png`（重新生成）

## 如何验证
- `cd frontend; npm.cmd run build` 通过。
- System.Drawing 逐行对比接缝：左段右缘(x=359) vs 中段左缘(x=0) 170 行仅 1 行差异（木纹噪点）；中段右缘(x=511) vs 右段左缘(x=0) 0 行差异。
- 手动验证：刷新面板看顶栏左端，金轨应全程连续、无颜色跳变。

## 下一步注意事项
- 如果以后又出现割裂，先确认三张 `topbar_shell_*.png` 是否仍同源（左段应是右段的镜像风格）；不要单独重绘其中一张。
- 若并行会话再切回整幅九宫格方案（`topbar-shell.png` + border-image），需先对齐，两方案不能混用。
# FE-MODS-DYNAMIC-PAGESIZE-1 前端接手记录（2026-07-03）
## 改了什么
- 下载模组页的 Nexus 搜索结果改为固定卡片高度 + 动态 pageSize。搜索结果列表新增专属 `.sd-mods-nexus-search-list` 和测量 ref，卡片高度固定为 `246px`。
- `ModsPage` 会根据 `.sd-mods-nexus-search-list` 到统一滚动视口 `.sd-main-scroll` 底部的剩余高度、实际 grid 列数和行间距计算 `rows * columns`，范围限制为 `1..20`，并把结果作为 `pageSize` 传给 `searchNexusMods()`。
- pageSize 改变且已有搜索结果时，会用当前关键词回到第 1 页重新搜索；分页总页数和顶部文案同步改为动态 pageSize。搜索结果底部重复分页器已移除，避免把当前 frame 内结果区撑长。
- 加载骨架只使用当前 pageSize 和同样高度占位，不绑定测量 ref；实际测量只发生在真实搜索结果列表上，避免 loading 与结果态顶部位置不同造成 pageSize 来回变化。已安装/添加模组列表没有使用 `.sd-mods-nexus-search-list`，不会被固定高度裁切。
## 影响文件/接口
- `frontend/src/games/stardew/pages/ModsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 继续使用既有 `GET /api/instances/:id/mods/nexus/search?q=&page=&pageSize=`，未改后端接口。
## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 内置浏览器真实应用当前停在登录页，因此使用临时本地 QA 页面加载真实 `StardewPanel.css` 验证布局公式：1040x1120 下 2 列 x 2 行，pageSize=4；1040x720 下 2 列 x 1 行，pageSize=2；520x720 下 1 列 x 1 行，pageSize=1；三种视口卡片高度均为 `246px`。临时 QA 文件已删除。
## 下一步注意事项
- 后续如果调整搜索卡片高度，需要同时更新 CSS `--sd-mods-nexus-search-card-height` 和 `ModsPage.tsx` 中的 `NEXUS_SEARCH_CARD_HEIGHT` fallback。
- 不要把固定高度规则放回通用 `.sd-mods-nexus-card`，否则已安装/添加模组列表的依赖标签和删除操作可能被裁切。
- 动态 pageSize 依赖 `.sd-main-scroll` 作为统一滚动视口；若主内容滚动结构再变，需要同步检查测量逻辑。
# FE-JOBS-PROTOTYPE-IMAGE2-1 前端接手记录（2026-07-03）

## 改了什么
- 任务与日志页按 image2 原型 `04-jobs-logs - 副本.png` 重皮肤：顶部大标题 + 虚线分隔、工具条按钮、左侧任务列表、右侧任务详情/进度/SSE 状态/深色日志终端和 VNC 修复提示改为原型里的羊皮纸 + 像素 UI 氛围。
- 未把原型整图作为背景或运行时资源。纸纹噪点、木/铜描边、内阴影、列表选中绿色框和左侧箭头、状态徽章、进度条斜纹、终端扫描线、VNC 警告纸条均由 CSS 实现。
- `JobsLogsPage.tsx` 只新增展示钩子：任务列表标题行、任务类型图标 class、短 job id 行、详情标题图标外壳、SSE 提示行容器；VNC 修复提示移到日志下方，贴近原型底部警告条位置。所有 API 调用、SSE、清空任务/错误日志、VNC 端口修改、权限判断、loading/error/empty/disabled 状态保持原逻辑。
- 按钮/图标复用：工具条按钮继续用 `sd-btn-tan` / `sd-btn-delete`；任务类型图标复用 `icon_nav_install_package_image2.png`、`icon_sidebar_chicken.png`、`icon_nav_server_rack_image2.png`、`icon_nav_saves_chest_image2.png`、`icon_nav_mods_crystal_image2.png`；VNC 提示图标复用 `sprite_blue_device.png`。
- 响应式补强：`.sd-jobs-page` 作用域下增加容器查询，窄屏左右两栏变单列，按钮纵向铺满，日志/按钮/长 ID 不横向撑出。

## 影响文件/接口
- `frontend/src/games/stardew/pages/JobsLogsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端接口、路由、数据模型或权限。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 真实 `/instances/stardew/jobs` 当前受登录页阻挡，因此使用临时 `frontend/jobs-logs-qa.html` 加载同一份 CSS、真实素材和同结构 DOM 做浏览器 QA；QA 文件已删除。
- 1280x900 桌面：两栏布局约左 350px / 右 761px，无横向溢出，VNC 提示首屏可见，console error/warn 为空。
- 390x760 窄屏：布局单列，无横向溢出；所有按钮文字未溢出；日志窗口不撑宽；滚到底部后 VNC 修复提示完整可见，console error/warn 为空。
- 已用 `view_image` 查看原型图、桌面实现截图、移动实现截图。

## 下一步注意事项
- 当前真实页面如果后端返回很多任务，左侧列表会在自身区域滚动；若用户希望原型底部分页器，可另开任务接入真实分页，不要在本次视觉层硬编码分页。
- 安装任务才有真实 `pullProgress` 进度条，非安装任务不会凭空显示进度，避免伪造业务进度；如果后端未来提供通用 job progress，可复用 `.sd-jobs-pull-progress` 样式。
- 样式覆盖集中在 `StardewPanel.css` 的 `FE-JOBS-PROTOTYPE-IMAGE2-1` 段，并以 `.sd-jobs-page` 为作用域；不要把终端/纸卡/状态徽章规则提升为全局样式。
# FE-DIAGNOSTICS-GAUGE-CODE-1 前端接手记录（2026-07-03）

## 改了什么
- 优化诊断与健康页资源占用三枚圆形仪表（CPU / 内存 / 磁盘）：数值和百分号拆分渲染，避免原先大号 `37.8%`、`3.5%`、`11%` 在圆心里挤压或视觉溢出。
- 圆环从单层 inline `conic-gradient` 改为 CSS custom properties + 页面级样式控制；进度角度仍由实时指标百分比计算，业务数据来源不变。
- 仪表盘质感改为代码实现的像素分段进度色环、硬边描边、羊皮纸内芯、内阴影和像素式投影；没有新增图片素材，也没有把原型图或截图作为运行时背景。

## 影响文件/接口
- `frontend/src/games/stardew/pages/DiagnosticsPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `docs/03-frontend.md`
- `docs/08-future-roadmap.md`
- 未改后端接口、路由、权限判断、API helper 或数据模型。

## 如何验证
- `cd frontend; npm.cmd run build`
- 本地浏览器 QA：加载真实 `StardewPanel.css` 的临时诊断仪表 DOM，检查 1280x900 与 390x760 下三枚仪表的数值和 `%` 不溢出，仪表卡片不重叠，无页面级横向溢出，console error/warn 为空。

## 下一步注意事项
- 以后如继续调整仪表圈，不要重新把百分号并回主数字；三位数或一位小数都需要维持在内芯宽度内。
- 仪表的主题色和进度角度由 `--sd-diag-gauge-color` / `--sd-diag-gauge-angle` 控制；新增资源指标时优先复用这套结构，不要在 TSX 中重新拼整段背景字符串。
# FE-CARD-UNIFY-SAVES-1 前端接手记录（2026-07-04）
## 改了什么
- 除模组管理页外，把 Stardew 其他页面的小框统一到存档管理页卡片基准：暖色纸面、铜色 2px 边框、9px 圆角、内描边和轻微底部阴影。
- 新增一组共享 CSS 变量：`--sd-save-card-bg`、`--sd-save-card-bg-strong`、`--sd-save-card-border`、`--sd-save-card-border-dark`、`--sd-save-card-title`、`--sd-save-card-text`、`--sd-save-card-muted`、`--sd-save-card-soft-line`、`--sd-save-card-shadow`。
- 按最新反馈去掉密集点状纸纹：共享背景变量改为干净线性高光 + 纯色纸面，不再使用铺满卡片的 `radial-gradient` 噪点；存档页 `.sd-save-card` / `.sd-saves-active-card` 也覆盖为这套干净变量，保证基准页本身也一致。
- 覆盖范围包括总览、服务器控制、任务日志、玩家管理、诊断、安装、设置、存档页的小框/面板/统计项/提示块；模组页 `.sd-mods-*` 主体卡片没有加入这次选择器，保持原风格。
- 文字和布局一起调优：标题约 14.5px、说明/元信息约 12.5px；窄屏容器查询下标题约 13.5px，小框 padding 与内部 gap 收敛，列表行、统计格、安装步骤、玩家/设置/诊断小项统一更紧凑。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.css`
- 未修改 TSX、API、后端 handler、权限判断、轮询、路由或数据结构。

## 如何验证
- 已执行：`cd frontend; npm.cmd run build` 通过。
- 真实本地应用当前停在登录页，因此使用已删除的临时 `frontend/public/__codex-card-qa.html` 加载同一份 Vite CSS 做内置浏览器 QA。
- 1280x720：总览/服务器/任务/玩家/诊断/安装/设置小框与存档卡片的背景、边框、圆角、阴影一致；`hasDotTextureOnUnified=false`，确认没有点状纹理；模组卡 `modsTouched=false`，仍是原 1px 边框和无渐变背景；无横向溢出。
- 390x760：页面无横向溢出，标题 `scrollWidth <= width`，无文字裁切；QA 后已删除临时文件。

## 下一步注意事项
- 后续新增非模组页小框时优先复用这组 `--sd-save-card-*` 变量，避免每页单独写背景/字号/边框。
- 如果以后要继续调整存档卡片质感，先改共享变量，再检查非模组页和存档页是否仍一致。
- 模组页当前刻意排除；若未来要统一模组页，需要单独评估 Nexus 卡片固定高度、弹层和 pageSize 计算，不能直接套这组选择器。
# FE-CARD-UNIFY-SAVES-1 follow-up（2026-07-04）
## 改了什么
- 按用户反馈，只清除总览页四个统计卡 `.sd-mc`（存档/模组/系统健康/运行任务）背景里的点状 `radial-gradient` 纹理。
- 保留原有结构、尺寸、边框、圆角、阴影、文字布局、状态徽章和响应式规则；没有把这组卡片改成新的卡片结构。

## 影响文件/接口
- `frontend/src/games/stardew/StardewPanel.css`
- 未修改 TSX、API、路由、权限或后端逻辑。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。
- 已确认 `.sd-mc` 两处背景定义不再包含点状 `radial-gradient`。

# FE-SERVER-PLAYERS-CARD-LAYOUT-1 前端接手记录（2026-07-04）
## 改了什么
- 新增共享组件 `ServerSummaryCard`，集中渲染“服务器状态 / 在线人数 / 最大人数 / 当前农场 / 主机农民 / 游戏日期 / 邀请加入码”这组摘要信息。
- 服务器控制页用该摘要卡替换原来的双栏大状态卡，并移除原独立邀请码卡，避免同一信息在服务器页重复出现。
- 玩家管理页移除顶部摘要卡，首屏直接进入在线玩家表；“服务器信息（Junimo）”移动到页面底部。
- 在线玩家表删除“角色”列，主机标识移动到玩家名右侧；新增“农场收入”和“玩家收入”可见列，并调整表格 grid 列宽和窄屏横向滚动宽度。

## 影响文件/接口
- `frontend/src/games/stardew/ServerSummaryCard.tsx`
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/games/stardew/pages/PlayersPage.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- 未修改后端接口、前端数据类型、权限判断、轮询或玩家数据来源。

## 如何验证
- `cd frontend; npm.cmd run build` 通过。
- 内置浏览器 DOM 快照接口本次返回兼容错误，未完成截图式 QA；临时 QA 页面已删除。

## 下一步注意事项
- 后续若要在其他页面展示服务器摘要，优先复用 `ServerSummaryCard`，不要再复制玩家页旧 JSX。
- 玩家表当前收入列使用已有 `farmIncome` / `personalIncome` 字段和 fallback；如果后端字段语义变化，需要同步更新列名和底部说明。
