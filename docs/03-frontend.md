# PANEL-UPDATE-HISTORY-STALE-1 历史终态与真实版本解耦（2026-07-14）

- 修复外部完成 `0.2.4` 更新后，页面先显示真实 `0.2.4`、再被历史 `0.2.2 succeeded` 倒写的问题。
- 当前版本优先取 `/api/system/update.currentVersion` 或 `/api/version`；只有成功目标等于当前版本时，历史 succeeded 才能主导成功提示。旧终态继续保留在详情中，但不再修改当前/最新版本。
- 同理，`failed_rolled_back` 只在其 fromVersion 仍是当前版本时主导页面；活动任务与 `rollback_failed` 始终保持最高优先级。
- 回归测试覆盖“当前已通过外部更新升到新版本、随后旧 apply 状态异步加载”的真实闪回顺序。

# PANEL-UPDATE-CONTINUOUS-1 连续升级状态修复（2026-07-14）

- 修复历史 `apply.phase=succeeded` 永久覆盖新版本检测的问题。上一次成功目标与当前 `latestVersion` 不同时，顶栏、总览和版本详情改为展示新版本，不再把旧成功记录误当作“最新”。
- 下一次升级不再被任意历史 `succeeded` 阻止；只有“该成功记录就是当前目标”或 `rollback_failed` 仍会阻止执行。
- `canStartPanelUpdate()` 新增 dry-run 目标精确匹配：旧版本的成功预检不能复用于新版本，管理员必须对当前 `latestVersion` 重新执行环境检查。
- 影响文件：`frontend/src/games/stardew/panel-update-machine.ts`、`frontend/scripts/test-panel-update-machine.ts`。验证覆盖“旧升级成功 + 新版本出现”、旧预检拒绝、新目标预检放行和 rollback_failed 门禁。

# FE-DIAGNOSTICS-USER-FIRST-1 服务器健康页用户视角重构（2026-07-14）

- 原“诊断与健康检查”改为“服务器健康”。默认视图按使用者的决策顺序组织：先给出整体是否正常，再列出真正需要处理的版本维护，最后展示各项检查、建议和资源占用。
- 新增紧凑的“版本维护”区，只在 Junimo、游戏/SDK 或 SMAPI 存在推荐更新时生成任务。Junimo `.121 → .125` 明确标注“不升级仍可继续使用”；管理员点击“查看并预检”才展开对应技术区，普通用户不会看到执行升级入口。
- 服务器状态来源、统一版本矩阵、镜像/digest/buildid、预检、确认升级和升级日志全部收进默认折叠的“维护与技术详情”；“导出支持包”也移入该区域，避免普通用户一进入页面就面对运维术语。
- 没有改变任何诊断、预检或升级 API，也没有改变升级门禁。`qa-layout-main.tsx` 增加了严格结构的更新状态 fixture，用于覆盖 `.121 → .125` 推荐更新和折叠详情交互。
- 影响文件：`frontend/src/games/stardew/pages/DiagnosticsPage.tsx`、`DiagnosticsPage.css`、`frontend/src/qa-layout-main.tsx`。
- 验证：桌面浏览器主视图及折叠详情视觉验收通过，无横向溢出和控制台错误；620px 以下任务按钮由响应式规则改为纵向全宽。状态脚本及生产构建见本次接手记录。

# PANEL-0.2.2 / JUNIMO-125 提示语义（2026-07-14）

- 前端无需新增接口或按钮；总览和诊断页继续消费内嵌矩阵派生的 `update_available`，把 `.121`→`.125` 显示为“推荐升级”，不显示“必须升级”且不自动触发 dry-run/apply。
- `.121` 用户仍可使用现有页面和操作。只有未来明确依赖 `.125` 的单项功能才允许单独禁用并说明版本要求，不能把整个面板锁住。
- 新安装由后端默认值直接落到 `.125`；管理员仍需依次执行预检、确认和成对升级，普通用户只读。

# JUNIMO-STACK-UPDATE-1 阶段二：运行组件升级预检界面（2026-07-13）

- 新增 dry-run 类型与 `getJunimoUpdateDryRun()` / `startJunimoUpdateDryRun()`；POST 不发送 body。管理员诊断页加载时恢复最近状态，活动阶段轮询并展示 progress、目标/选中版本对、checks、warnings、失败码和脱敏日志。
- 唯一可用操作是整体“运行升级预检”；“执行升级（下一阶段提供）”保持 disabled。普通用户不调用管理员 dry-run API，继续只看脱敏 tag/整体状态。
- 镜像/digest/检查项/日志允许任意断行；620px 以下按钮全宽、版本对与检查项纵向布局。`test:junimo-update` 覆盖 dry-run 阶段文案/活动态，生产构建继续执行 `tsc -b`。

# JUNIMO-STACK-UPDATE-1 阶段一：运行组件更新提示（2026-07-13）

- 新增 `JunimoUpdateInfo`/五态类型和 `getJunimoUpdate()`。管理员总览页仅在 `available=true` 时显示“Junimo 运行组件可更新”，唯一操作为“查看详情”并跳转诊断页，不提供 server/auth 独立按钮或执行升级按钮。
- 诊断页把 server 与 steam-auth-cn 作为一个版本对展示：当前两组件镜像/tag、推荐 stackVersion 与两组件版本、整体是否匹配、unsupported 原因和版本说明。管理员可读取完整镜像引用；普通用户只消费 `/state.runtimeDiagnostic` 的 tag/状态，并显示“仓库信息仅管理员可见”。
- 新增 `junimo-update-status.ts` 五态文案/匹配辅助与 `test:junimo-update`。总览提示和诊断镜像长文本使用 `min-width: 0`、`overflow-wrap:anywhere`，窄屏提示卡改为纵向，避免桌面和移动端横向溢出。
- 阶段一 UI 没有 dry-run/apply，也不会触发镜像拉取、配置改写或容器操作；Panel 自身更新弹窗保持独立。

# PANEL-UPDATE-RELEASE-1 Web 验收（2026-07-13）

- 隔离真 Docker 成功链路中，顶栏与总览同时展示 `v0.1.14` 更新；管理员经环境检查和二次确认启动升级，页面在 panel 短暂离线时进入专用升级态，恢复后自动回原页面并展示成功结果。
- 故意 unhealthy 镜像触发回滚后，顶栏、总览和统一弹窗一致显示“升级失败，已恢复”，未把预期断线渲染成普通网络错误，也未显示 helper 原始命令。
- 桌面 1280×800 与移动 390×844 已做浏览器视觉 QA：弹窗无横向溢出，移动顶栏使用紧凑“已恢复”，总览保留完整状态；两条链路浏览器控制台均无错误。
- 普通用户保持只读且没有环境检查/升级按钮；Provider 位于路由和响应式壳层之外，上横栏、总览、弹窗只使用一份状态与轮询。

# FE-PANEL-UPDATE-1 完整 Web 面板升级交互

- `PanelUpdateProvider/usePanelUpdate` 挂在登录后的 Stardew 桌面/移动壳之外，统一初始化版本状态、管理员 dry-run/apply 状态、手动检查、升级触发、单一轮询和结果弹窗；页面路由与桌面/移动响应式切换不会重建升级任务状态。
- `panel-update-machine.ts` 统一派生顶栏、总览、移动端文案与颜色。阶段覆盖备份、拉取、重建、健康等待、回滚、成功、已恢复和恢复失败；顶栏不增高，总览继续复用版本/最新两格。
- 管理员弹窗展示版本、发布时间、Release 说明链接、部署支持状态、安全边界、可折叠环境详情和公开阶段时间线；正式按钮为“立即升级并重启面板”，提交前使用第二层确认。普通用户只读且完全不渲染检查/升级按钮。
- apply 活动请求断开时进入全屏“面板正在升级”，以指数退避检查 `/health`、`/api/version` 和持久 apply 状态，180 秒后提供继续自动等待与非命令行说明。成功或回滚后保留原 URL/路由、刷新页面数据并自动打开结果详情。
- QA harness 支持 `update=available|latest|error`、`apply=pulling|rolling_back|failed_rolled_back|offline|reconnect-success`、`shell=mobile`、`role=user`。验证脚本为 `npm run test:panel-update`、`npm run test:update-status` 和 `npm run build`。

# PANEL-UPDATE-APPLY-1 基础升级触发（历史阶段，已由 FE-PANEL-UPDATE-1 完善）

- 更新详情弹窗在管理员 dry-run 成功且后端确认有正式新版本时提供基础“开始升级”触发；请求不携带镜像或版本。普通用户仍只读且不会请求管理员 apply 状态。
- 当时弹窗仅共享 dashboard 中的 apply 状态并轮询活动阶段；完整确认、离线恢复和结果引导现已由上方 `FE-PANEL-UPDATE-1` 取代。

# PANEL-UPDATER-DRYRUN-1 管理员升级环境演练展示（历史阶段）

- 该阶段只提供“检查升级环境”；现在正式升级按钮由 `FE-PANEL-UPDATE-1` 在 dry-run 成功后展示。
- 弹窗展示 supported/unsupported、reason/code、Compose 项目、当前容器/镜像、目标镜像和脱敏阶段日志；不展示 installDir、composeFile、dataMount 等宿主机详细路径。
- 普通用户仍只看到版本检测信息，不请求、不显示 dry-run 状态，也不能手动执行环境检查。
- `starting/running` 每 2 秒读取管理员状态接口，完成后停止；失败只表示演练未通过，不出现可执行升级按钮。

# PANEL-UPDATE-CHECK-1 面板版本展示（历史阶段）

- 当时由 `useStardewDashboardData` 持有共享状态；现已提升为 App 级 `PanelUpdateProvider`，消费者仍不各自请求接口。
- 顶栏复用原版本区块，总览复用原“版本/最新”信息格；两处在有更新时统一显示“发现新版本 vX.Y.Z”，已是最新时保持“✓ 最新”。
- `UpdateDetailsDialog` 的只读版本信息仍保留；管理员执行与恢复交互现由 `FE-PANEL-UPDATE-1` 提供。
- 移动端首页增加同一状态入口，弹窗和状态样式适配窄屏；QA 页面支持 `update=available|latest|error`。
- 主要文件：`UpdateDetailsDialog.tsx`、`update-status.ts`、`StardewPanel.tsx`、`OverviewPage.tsx`、`MobileHomePage.tsx`。验证：`npm run test:update-status`、`npm run build`。

# MOBILE-SHELL-SAVES-REFINE-1 手机端背景、顶栏与存档卡片优化

- `StardewMobileShell.css`：移动端整体背景改为复用 PC 端 `background_app_black.png` 深色纹理；主体区域复用 PC 端 `.sd-main` 的 image2 主页面框素材（四角、四边、中心 tile），让手机端页面背景更接近桌面端页面框风格。
- 移动端顶部横栏改为轻量 PC 顶栏风格：新增生成素材 `frontend/public/assets/stardew/ui/topbar/mobile_topbar_framed_generated_image2.png`（内置 imagegen 生成四边框木纹顶栏后，不裁切，整张缩放到 1170×174 手机横条），并与 `icon_topbar_chicken_image2_v2.png` 组合使用；状态徽章改成浅棕像素按钮样式。该素材带完整上/下/左/右边框，替代原先拼接 PC 左/右端图和中段 tile 的方案，避免手机宽度下的背景接缝/撕裂感；CSS 使用 `background-size: 100% 100%`。
- 移动壳改为固定一屏舞台：`.sd-mshell` 使用 `height: 100dvh` 和 `overflow: hidden`；`.sd-mshell-body` 承载固定页面框背景，`.sd-mshell-scroll` 作为唯一纵向滚动容器。这样顶部横栏、底部 Tab、页面框背景保持不动，只有内部组件内容上下滑动。
- 移动端主体滚动区改为内层裁切：`.sd-mshell-body` 继续承载 PC 端 image2 页面框背景并固定不滚动，同时设置 `overflow: hidden` 和上下边框安全区；页面内容移入 `.sd-mshell-scroll` 作为唯一纵向滚动容器。这样滚动内容超过背景自带的上/下木纹框线时会被裁掉，不再遮挡框线本身。
- 底部 Tab 的占位不再放在 `.sd-mshell` 外壳 padding 上，避免底栏下方露出黑色外壳背景；`.sd-mshell-body` 的页面框背景铺满到底，最后内容不被 Tab 遮挡的空间由 `.sd-mshell-scroll` 的底部 padding 提供。
- `.sd-mshell-scroll` 顶部额外内距已收为 0，`.sd-mshell-body` 的上边框安全区进一步减半（约 15px），组件会更贴近上方裁切区内侧顶格显示，减少顶部空白；底部安全区保持原高度以避让悬浮 Tab。
- `MobileSavesPage`：农场原画不再单独占一张大卡，缩小为核心信息卡左侧约 72px 宽的 16:9 缩略图，和存档名称、农场名称、农场主、游戏日期合并在同一张“核心信息”卡内；“当前使用中/可用”等状态徽章放在缩略图上方，不再覆盖图片；更多信息和存档操作仍保留独立卡。
- 验证：`cd frontend; npx tsc --noEmit -p .` 通过；`cd frontend; npm run build` 通过（仅保留 Vite chunk 大小提示）。

# MOBILE-MODS-M7-1 手机端模组页

- 手机端底部 Tab 新增“模组”，位置在“玩家”和“存档”之间；`StardewMobileShell` 的底栏改为 6 个入口（总览 / 控制 / 玩家 / 模组 / 存档 / 更多），桌面端导航不受影响。
- 新增 `frontend/src/games/stardew/mobile/MobileModsPage.tsx` 与 `MobileModsPage.css`。页面右上角保留全局“刷新 / 导出 / 上传”操作，内部用紧凑分段控件切换“搜索 / 服务器模组”，不离开手机端页面。
- 搜索页复用现有 `searchNexusMods` 与 `/mods` 数据校正安装状态，移动端单列展示 Nexus 结果：缩略图、名称、Nexus ID、版本、更新时间、简介、前置依赖状态、已安装/已安装未启用状态和“跳转 N站”。按用户追加反馈，搜索框以上的 Nexus Key / 扩展连接区已删除，搜索框升级为内嵌式像素搜索条（浅色输入底、绿色聚焦边框、精简搜索按钮），搜索结果卡片去掉安装按钮，并进一步移除来源、作者、下载量、认可数；每页展示 4 个结果，热门标签保留 `UI Info`、`Fishing Mod`、`Tractor`，底部前置状态按钮与小号 N 站跳转按钮在同一行平齐，上一页 / 页码输入 / 跳转 / 下一页收成单行。
- 服务器模组页复用 `getMods`、`updateModEnabled`、`exportMods`、`uploadMods`，把已安装信息和启用状态融合到同一张卡片：隐藏 `builtIn` 与系统运行组件（SMAPI、StardewAnxiPanel.Control、JunimoServer/JunimoHost.Server），只展示用户安装的服务器 Mod。卡片左侧显示 `pictureUrl` 的 N 站缩略图（没有图片时用 NEXUS/MOD 占位），缩略图放大到约 74px 方形以覆盖到“更新”信息行高度；右侧展示名称、状态、版本、文件夹、更新时间、同步类型、依赖标签等；按用户反馈移除“作者”“来源”字样，“已启用/已禁用”徽章固定在名称行右上角并与名称平齐。N 站链接入口为独立橙棕色矩形标签按钮，文案“跳转N站”，避免和其它标签底色相同；真实启用开关放在底部标签行最右侧，样式为无文字、无外框的绿色小开关。切换到“服务器模组”时主动刷新 `GET /mods`；切换中按单个 Mod 禁用并在成功后刷新列表，失败显示既有 notice。
- 移动壳主体改为顶部对齐，避免“搜索 / 服务器模组”两页内容高度不同导致页头和二级 Tab 上下跳动。
- 影响文件：`frontend/src/games/stardew/StardewMobileShell.tsx`、`frontend/src/games/stardew/StardewMobileShell.css`、`frontend/src/games/stardew/mobile/MobileModsPage.tsx`、`frontend/src/games/stardew/mobile/MobileModsPage.css`、`frontend/src/qa-layout-main.tsx`。未新增后端接口，未改桌面端 `ModsPage.tsx`。
- 验证：`cd frontend; npx tsc --noEmit -p .` 通过；`cd frontend; npm run build` 通过（仅保留 Vite chunk 大小提示）。

# FE-MOD-BATCH-ERROR-FOCUS-1 Nexus 批量安装失败定位

- Nexus 普通一键安装的进度按钮现在会在真实后端 job 失败时显示失败的具体 Mod 名，例如 `SpaceCore 失败`；如果该失败项带有 `jobId`，按钮保持可点击，点击后跳转到任务与日志页并自动选中对应任务。
- 批量进度协调器会用最新 `GET /mods` 结果校正已安装项：即便旧后端 job 曾因重复 `UniqueID` 被标成 failed，只要本地已经能按 `nexusModId` 或 `originNexusModId` 匹配到该 Mod，该项就视为完成，不再把整批安装误判为失败。
- 无后端 job 的扩展捕获/提交失败仍保持原有重置与手动处理流程；只有带 `jobId` 的后端安装失败才作为“点击查看日志”入口。
- 影响文件：`frontend/src/games/stardew/pages/ModsPage.tsx`。未新增 API，复用现有 `GET /api/jobs/:id`、`GET /api/instances/:id/mods` 和任务页 `?jobId=` 查询参数。
- 验证：`cd frontend; npm.cmd run build`。

# FE-PANEL-ACCESS-HOST-INVITE-1 局域网邀请使用当前面板地址

- 邀请码卡片下方“局域网邀请”不再调用后端公网 IP 检测接口，而是从 `window.location.hostname` 读取当前浏览器进入面板使用的 host。
- 用户用 `192.168.x.x:8090` 进入面板时显示局域网 IP；用公网 IP、FRP 域名或反代域名进入时显示对应 host。复制按钮复制的也是该 host。
- “刷新”按钮文案改为“同步”，表示同步当前面板访问地址，而不是重新探测公网出口。
- 验证：`cd frontend; npm.cmd run build`。

# FE-STEAM-AUTH-FLAG-1 邀请码授权按钮按持久标志显示

- `InviteCodeCard` 的授权入口只按 `instanceState.steamAuthLoggedIn` 这个持久标志显示：不是 true 时显示“需登录 Steam 授权”和【登录授权】；true 时不再因为 `steamAuthReady=false` 单独显示重新授权。
- 后端在 steam-auth 登录成功日志出现后把 `steamAuthLoggedIn` 写 true；启动/刷新邀请码成功拿到非空邀请码时也会写 true。如果启动服务器后日志明确显示 `steam-auth` 没有登录账号，后端会写回 false，前端下一轮状态刷新后重新显示授权入口。
- `Steam-auth service not ready` 只表示运行态服务暂未就绪；已有授权标志时后端会自动刷新 `steam-auth` 服务，前端仍只消费 `steamAuthLoggedIn`。
- 如果服务器仍在运行或启动中且需要授权，按钮显示“停服后登录授权”并禁用，提示用户先停止服务器；停止后可点击“登录授权”，复用已保存账号启动 `steam-auth/login` 并跳转安装页查看认证日志。
- `steamAuthReady` 仍保留在类型里作为诊断字段，不再参与邀请码卡主按钮判断。

# FE-LIFECYCLE-BACKGROUND-INVITE-1 启动不等待邀请码

- `InstanceState` 新增可选 `inviteCode`，来自后端后台邀请码探测写入的 driver payload。`useStardewDashboardData` 会优先展示该值，成功后清理旧码等待态。
- 自动邀请码轮询只在显式请求后运行，最多 20 次；没有邀请码不会让前端无限轮询，也不会阻塞停止/重启。
- 总览页和服务器控制页的启动中状态以 active `stardew_lifecycle` job + `running/stopping` 为基础判定，不再依赖邀请码或 SMAPI 存档加载日志。这样切到任务日志再切回来时，后台仍在跑的启动任务不会把按钮闪回“启动”。
- 验证：`cd frontend; npm.cmd run build`。
- **2026-07-11 更新**：服务器控制页与总览页都在 job+state 判定基础上叠加了一层“主机上线确认”（带超时兜底），因为纯 job+state 判定会在游戏实际加载完成前就把按钮切回正常态。详见下方 `FE-STARTUP-HOST-CONFIRM-1` 与 `FE-OVERVIEW-STARTUP-HOST-CONFIRM-1`。这次改动不影响邀请码后台轮询逻辑。

# FE-INSTALL-SMAPI-PREINSTALL-1 安装页显示 SMAPI 子状态

- 安装页新增识别 `smapi_installing` / `smapi_install_failed`。后端任务日志出现 `[smapi]` 时，前端会把当前阶段切到 `smapi_installing`。
- “下载游戏”步骤的子任务进度从 2 段扩展为 3 段：游戏文件、Steam SDK、SMAPI 运行环境。SMAPI 安装发生在 Steam SDK 完成之后。
- `smapi_install_failed` 不再按 Steam 认证失败处理；它属于后置安装失败，进度条只把第 4 步“下载游戏”标红，并允许复用已保存凭据重试。
- 安装页新增 SMAPI 专属提示卡片：“安装 SMAPI 运行环境中...”，说明正在通过加速源安装，完成后进入安装完成。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`、`frontend/src/games/stardew/install-helpers.ts`。
- 验证：`cd frontend; npm.cmd run build`。

# FE-STEAM-GUARD-SUBMITTED-FEEDBACK-1 验证码提交后等待态

- 修复 Steam / SteamCMD 验证码提交成功后页面立刻回到同一个输入框，用户容易误以为提交失败的问题。
- `InstallPage.tsx` 新增 `guardSubmittedKind` 本地状态；普通 Steam Guard 和 SteamCMD 验证码提交成功后会清空输入框，并显示“验证码已提交，正在等待 Steam/SteamCMD 响应”的等待态。等待态提供“重新输入”按钮，避免验证码填错或上游长时间无响应时用户被锁死。
- 当 `effectivePhase` 离开当前验证码/手机批准阶段（例如进入 `steamcmd_downloading`、失败或完成）时，等待态会自动清除。接口不变，仍使用 `POST /api/instances/:id/steam-guard/input`。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build`。

# FE-STEAMCMD-EMAIL-GUARD-PROMPT-1 SteamCMD 邮箱验证码分行提示

- 修复 SteamCMD 原生日志已经提示“请检查邮箱并输入 Steam Guard code”，但安装页仍停留在 `steamcmd_downloading` / 客户端自更新进度的显示问题。
- `inferLatestSteamAuthLogPhase()` 的 SteamCMD 专属分支现在识别 `this computer has not been authenticated`、`please check your email`、`enter the steam guard`、`code from that message`、`set_steam_guard_code`。这些日志只在带 `[steamcmd]` 前缀时触发，命中后前端切到 `steamcmd_guard_required` 并展示验证码输入框。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`。
- 验证：`cd frontend; npm.cmd run build`。

# FE-INSTALL-CHANGE-ACCOUNT-1 更换 Steam 账号 / 强制重新认证入口

- 安装页新增 `forceReauth` 状态与「更换 Steam 账号 / 重新认证」按钮：出现在已安装卡片（与“重新安装 / 修复”并列），以及未安装但可复用凭据重试的操作区（`!isInstalled && canDirectRetry && !authFailed` 时）。已安装态只使用卡片内按钮，避免重复渲染。点击后 `setForceReauth(true)` 并打开表单。
- 凭据输入显示条件由 `!canDirectRetry` 改为 `!canDirectRetry || forceReauth`，即更换账号时即便处于复用态也会显示账号/密码/VNC 输入框。表单标题/提示新增 forceReauth 文案（“将清除已保存的 Steam / SteamCMD 授权缓存并用新账号密码重新认证；已下载的游戏文件会保留。”），提交按钮显示“确认更换账号并重新认证”。
- `handleInstallSubmit`：`forceReauth` 时提交 `{ steamUsername, steamPassword, vncPassword, imageTag, forceReauth: true }`；其余分支不变（复用发 `{ reuseCredentials:true, imageTag }`，全新发完整凭据）。提交成功或点“取消”都会复位 `forceReauth`；“安装/重试”“重新安装/修复”按钮点击时显式 `setForceReauth(false)` 防止残留。
- `api.ts` `installInstance` body 类型新增可选 `forceReauth?: boolean`。
- 说明：镜像拉取失败、连接/认证超时等**认证前**失败重试的“不弹凭据表单、自动账号密码继续”，以及只有 `credentials_required` 才重输凭据的既有逻辑均未改动——后端已把路由收敛，前端复用重试仍照发 `reuseCredentials`。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`、`frontend/src/api.ts`。
- 验证：`cd frontend; npm.cmd run build`。

# FE-STEAMCMD-REPAIR-DIRECT-1 修复/重新安装不再要求输入凭据

- 安装页已把已安装态的“重新安装 / 修复”纳入复用凭据路径，提交 `POST /api/instances/:id/install` 时只发送 `{ reuseCredentials: true, imageTag }`，不再显示 Steam 用户名、Steam 密码或 VNC 密码输入框。
- 表单文案改为“确认修复 / 更新”，说明本次会跳过 `steam-auth`，复用已保存凭据和 SteamCMD 授权缓存直接下载/校验游戏文件。
- SteamCMD 下载卡文案改为通用的“复用已保存凭据和授权缓存下载/校验”，避免把主动修复路径误描述成 `steam-auth` 下载失败后的重新授权。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`。
- 验证：`cd frontend; npm.cmd run build`。

# FE-TOPBAR-BRAND-LIGHTER-2 顶栏品牌标题继续减重

- 按用户“再细 200”反馈继续微调 Stardew Shell 左上角 `Stardew Anxi Panel`：`.sd-topbar-brand-text` 字重从 `700` 降到 `500`，暗色描边/投影不透明度同步再降一点。
- 只影响顶栏品牌文字；未改顶栏状态牌、存档框、版本框、用户框、路由、API、权限、轮询或 Junimo 通信。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build`；Browser QA 打开 `qa-layout.html?state=running`，标题 computed `fontWeight=500`，总览/服务器往返后仍为 `500`。

# FE-OVERVIEW-HEALTH-SHARE-1 概览系统健康卡同步诊断结果
- 诊断页进入时自动执行的健康检查、以及用户点击“重新检查”成功后，会把 `GET /api/health/diagnostics` 的结果写回公共 `dashboardData.health`，因此回到总览页后“系统健康”统计卡会显示最新评分、通过/警告/错误数量和状态徽章，不再停留在 `— / 未检查`。
- 公共 dashboard 初始化仍不主动调用 `/api/health/diagnostics`，保留 `DOCKER-POLL-PERF-1` 的降轮询设计；只有用户打开诊断页或手动检查后，概览页才消费这次已产生的诊断结果。
- 影响文件：`frontend/src/games/stardew/stardew-routes.ts`、`frontend/src/games/stardew/useStardewDashboardData.ts`、`frontend/src/games/stardew/pages/DiagnosticsPage.tsx`。未改后端 API、权限、路由、Junimo 通信或普通概览初始化轮询。
- 验证：`cd frontend; npm.cmd run build`；Browser QA 打开 `http://127.0.0.1:5174/qa-layout.html?state=running`，初始总览健康卡为 `—`，进入诊断页拿到 6 项正常后回总览，健康卡显示 `100% / 6项全部通过 / 优秀`。

# FE-PUBLIC-IP-INVITE-CARD-1 邀请卡增加服务器公网 IP
- `InviteCodeCard` 在邀请码下方新增“服务器公网 IP”一行，展示后端 `GET /api/instances/:id/public-ip` 检测到的面板服务器公网出口 IP，并提供复制、刷新按钮。
- 总览页与服务器摘要页复用同一个 `InviteCodeCard`，因此两处都显示同一套 IP 检测框；刷新按钮会请求 `?refresh=1` 强制重新检测，复制按钮只复制 IP 文本。
- 按用户反馈移除邀请码下方“分享此代码邀请新玩家加入服务器”说明文字，公网 IP 行也不展示说明文案，只保留标题、值和复制/刷新按钮，避免截图中的小框被说明文字撑高。
- 上方邀请码行标题保持“邀请码”；下方公网 IP 检测行标题显示为“局域网邀请”。公网 IP 未检测/检测失败时不显示复制按钮，但操作区保留固定宽度，保证两行值框宽度一致。
- 总览页“服务器控制”卡内的邀请/IP 组上移到右上区域，减少标题右侧原本的大块留白；未改变按钮宽度和两列主布局。
- 数据层新增 `publicIP/publicIPError/publicIPRefreshing/refreshPublicIP()`，初始化与 `refreshAll()` 会做一次缓存读取，手动刷新才强制重新探测。
- 影响文件：`frontend/src/api.ts`、`frontend/src/types.ts`、`frontend/src/games/stardew/stardew-routes.ts`、`frontend/src/games/stardew/useStardewDashboardData.ts`、`frontend/src/games/stardew/InviteCodeCard.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未改邀请码获取、生命周期按钮、Junimo 通信或 Docker 诊断轮询。
- 验证：`cd frontend; npm.cmd run build`。

# FE-MOD-COUNT-FILTER-BUILTIN-1 总览模组统计过滤内置组件
- 总览页模组统计现在复用模组页的系统运行组件识别口径：SMAPI runtime、`StardewAnxiPanel.Control`、`JunimoServer` / `JunimoHost.Server` 不计入用户可见模组统计。
- “模组”统计卡的大数字、`已启用 N 个`、同步包摘要里的已启用/已停用数量都基于过滤后的用户可见 Mod 列表，避免把面板内置依赖算成玩家安装模组。
- 新增共享 helper `frontend/src/games/stardew/mod-visibility.ts`，`OverviewPage` 与 `ModsPage` 共用 `modIsSystemRuntime()`，后续新增内置运行组件时只需同步扩展该 helper。
- 影响文件：`frontend/src/games/stardew/mod-visibility.ts`、`frontend/src/games/stardew/pages/OverviewPage.tsx`、`frontend/src/games/stardew/pages/ModsPage.tsx`。未改后端 API、启用状态接口、同步包导出或 Junimo 通信。
- 验证：`cd frontend; npm.cmd run build`。

# FE-ASSET-RUNTIME-SLIM-1 前端运行素材与原型制品瘦身
- `docs/prototypes/` 已从历史大图目录改为轻量索引目录：当前只保留 `README.md`、`overview-design-baseline-2026-06-30.png` 和 `overview-current-baseline-2026-07-04.png`。完整原型截图、当前实现截图和 `assets/ui-extracted` 提取工作区应作为 Release artifact、对象存储或单独设计仓库制品保存。
- 登录背景已回退为 PNG-only 加载，避免 AVIF/WebP 或重编码造成色调偏移；`background_login_farm_generated.png` 与 `background_login_home_image2.png` 保持原仓库色调。
- 对 `frontend/public/assets` 中超过 300 KB 的运行 PNG 做了无损重压缩并做像素等价校验；右栏 9-slice、tile 等非登录背景素材只做无损压缩，不改变切片参数。
- favicon 从单个 512px / 545 KB PNG 改为 `favicon.ico` 加 32/64/128 PNG，多尺寸图标位于 `frontend/public/favicon-*.png`，默认 `favicon.png` 收敛为 128px。
- 影响文件：`frontend/src/App.css`、`frontend/index.html`、`frontend/public/favicon*`、`frontend/public/assets/stardew/ui/backgrounds/*.{avif,webp,png}`、若干运行 PNG 素材、`docs/prototypes/README.md`。
- 验证：PNG 无损重压缩脚本逐张通过像素等价校验；`docs/prototypes` 从 109 个文件约 71.38 MB 降到 3 个文件约 2.58 MB。

# DOCKER-POLL-PERF-1 诊断与资源指标按需刷新

- 公共 `useStardewDashboardData()` 初始化不再自动请求 `/api/health/diagnostics`，避免用户只是进入总览页时触发 `DockerVersion` / `ComposeVersion` 这类诊断命令。
- 总览页“系统健康”统计卡在未打开诊断前显示“未检查 / 进入诊断页后检查”，不再显示“检查中”造成后台正在持续诊断的误解。
- 右侧 OpsRail 不再常驻轮询 `/api/instances/:id/metrics`；非资源页不持续触发 `docker compose stats --no-stream`。
- `DiagnosticsPage` 进入页面时会主动执行一次健康检查；用户点击“重新检查”时再执行一次。资源指标只在诊断页组件挂载且 `document.visibilityState === "visible"` 时刷新，间隔 `8s`；浏览器 tab 隐藏时清理 timer，回到可见时立即采样一次。
- 验证：`cd frontend; npm.cmd run build`。

# FE-CLEANUP-UNUSED-ASSETS-1 前端无引用素材与死组件清理
- 清理 `frontend/public/assets/stardew/ui/` 下 79 个前端源码零引用的旧 PNG 生产素材，主要集中在旧右栏整图、旧顶栏三段素材、旧导航/字段/图标 sheet 和早期装饰 sprite；清理后 `frontend/public/assets` 从约 39.52MB 降到约 18.56MB。
- 删除无引用 React 组件：`frontend/src/core/CommandOutput.tsx`、`frontend/src/core/StatusPill.tsx`、`frontend/src/core/StatusBadge.tsx`、`frontend/src/games/stardew/InstanceStateCard.tsx`。
- 保留 `frontend/public/assets/stardew/new-game/`，其中宠物、农场等图片存在模板字符串动态路径；保留 `frontend/qa-layout.html` 与 `frontend/src/qa-layout-main.tsx` 作为现有前端回归 QA 入口。
- `docs/prototypes/` 后续已改为轻量索引目录，完整历史原型截图迁出主仓；生产运行代码仍不依赖该路径。
- 本地额外清理了已忽略的 `.gocache/` 与 `tmp/` 缓存目录，属于工作区本地瘦身，不影响仓库代码。
- 验证：前端素材复扫 `UNUSED_NON_NEW_GAME=0`；`cd frontend; npm.cmd run build` 通过。

# FE-MODS-HIDE-SYSTEM-RUNTIME-1 模组页隐藏系统运行组件
- `ModsPage` 新增系统运行组件识别：SMAPI runtime、`StardewAnxiPanel.Control` 和 `JunimoServer` / `JunimoHost.Server` 不再出现在“添加模组”的已安装卡片列表，也不再出现在“配置模组 / 当前存档 Mod 启用状态”开关列表。
- “已安装”统计和解析失败统计改为只统计用户可见 Mod；只剩系统运行组件时，添加页显示“当前没有可展示 Mod”，配置页显示“当前没有可配置 Mod”。
- 玩家同步统计和导出逻辑仍使用后端返回的完整 Mod 列表，避免影响完整同步包对基础运行依赖的既有处理；本次只改用户可见展示层。
- 影响文件：`frontend/src/games/stardew/pages/ModsPage.tsx`。未改后端 API、上传/删除/导出、启用状态切换接口、玩家同步包导出或 Junimo 通信。
- 验证：`cd frontend; npm.cmd run build`。

# FE-STEAMCMD-SELFUPDATE-PROGRESS-1 SteamCMD 自更新进度展示
- 安装页现在会把 SteamCMD 日志中的 `[steamcmd] [ 40%] Downloading update (.. of 40,273 KB)` 在登录前识别为 `steamcmd_update`，显示为“SteamCMD 正在更新客户端中…”，不再误标为 Docker 镜像拉取或 Stardew 游戏文件下载。
- `steamcmd_downloading` 阶段的下载卡优先显示真正的游戏/SDK 进度；只有尚未进入 SteamCMD 登录和 app_update 时，才展示客户端自更新百分比。
- 安装总进度说明会显示“SteamCMD 镜像已就绪，正在更新 SteamCMD 客户端；这不是 Docker 镜像拉取。”，用于解释用户截图里 40MB 更新的来源。
- 影响文件：`frontend/src/games/stardew/install-helpers.ts`、`frontend/src/games/stardew/pages/InstallPage.tsx`。接口和 SSE 契约不变。
- 验证：`cd frontend; npm.cmd run build`。

# FE-STEAMCMD-RETRY-RESUME-1 SteamCMD 重试提示
- 安装页新增 `steamCMDRecoverable` 分支：当当前失败 phase 是 `steamcmd_failed` 或 `steamcmd_image_pull_failed` 且允许复用凭据重试时，按钮显示“重试 SteamCMD 授权/下载”，表单标题显示“重试 SteamCMD 兜底下载”。
- 表单提示明确说明：本次会直接复用已保存账号密码进入 SteamCMD 授权/下载；本地已有 SteamCMD 镜像时不会重新拉取。
- 提交请求仍沿用现有 `POST /api/instances/:id/install`，请求体仍是 `reuseCredentials=true`，不新增前端 API 字段；后端根据实例 `driverPhase` 自动直达 SteamCMD。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`。
- 验证：`cd frontend; npm.cmd run build`。

# FE-STEAMCMD-BRACKET-PROGRESS-1 SteamCMD 方括号进度识别

- 安装页现在同时识别 SteamCMD 真实输出的 `[steamcmd] [ 28%] Downloading update (11,467 of 40,273 KB)...` 格式，不再只识别 `[steamcmd] ... progress: N (done / total)`。
- 该格式会把安装页右侧切到 `steamcmd_downloading`，展示 SteamCMD 百分比、已下载量和总量；不再停在“正在等待 Steam 输出下载进度”。
- SteamCMD 输出 `Please confirm the login in the Steam Mobile app` 或 `Waiting for confirmation` 时，仍会切到 `steamcmd_guard_mobile_required`，提示管理员打开 Steam App 批准。
- 影响文件：`frontend/src/games/stardew/install-helpers.ts`、`frontend/src/games/stardew/pages/InstallPage.tsx`。未新增 API。
- 验证：`cd frontend; npm.cmd run build`。

# FE-STEAMCMD-FALLBACK-1 安装页 SteamCMD 兜底提示

- 安装页新增 SteamCMD 兜底阶段展示：当后端从 `steam-auth` 下载失败自动切换到 SteamCMD 时，右侧认证区会显示“steam-auth 在国内网络下下载失败，面板已自动改用 SteamCMD 复用账号密码下载”，并把 `steamcmd_downloading` 作为正常安装中阶段处理。
- SteamCMD 需要重新授权时，前端不再展示普通 Steam Guard 文案，而是显示“steam-auth 国内网络波动导致下载失败，SteamCMD 兜底需要重新授权”。若后端进入 `steamcmd_guard_choice_required`，页面提供“手机 App 批准”和“App / 邮箱验证码”两个选择；进入 `steamcmd_guard_required` 时展示验证码输入；进入 `steamcmd_guard_mobile_required` 时提示打开 Steam 手机 App 批准 SteamCMD 登录。
- 安装进度和状态横幅纳入 `steamcmd_image_pulling`、`steamcmd_auth_running`、`steamcmd_guard_*`、`steamcmd_downloading`、`steamcmd_failed`、`steamcmd_image_pull_failed`，避免把 SteamCMD 兜底误判为原 `steam-auth` QR/Guard 阶段或已中断安装。
- 失败提示补充 `steamcmd_failed` / `steamcmd_image_pull_failed`，普通兜底下载失败仍可复用已保存凭据重试；若后端返回 `credentials_required`，前端仍要求重新输入 Steam 凭据。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`、`frontend/src/games/stardew/install-helpers.ts`。未新增 API，仍复用 `POST /api/instances/:id/steam-guard/input` 和安装 job SSE。
- 验证：`cd frontend; npm.cmd run build`。

# FE-STEAM-AUTH-DOWNLOAD-PROGRESS-RESTORE-1 安装页 Steam 认证/下载阶段按最新日志显示

- 修复账号密码登录后已经通过 Steam Guard 并开始下载时，前端仍显示“请在手机 App 批准登录”的问题。安装页现在会从最新 Steam 日志识别下载阶段：出现 `Downloading app 413150`、`Manifest contains` 或 `Progress:` 后，`effectivePhase` 会切到 `game_downloading`；SDK 下载进度切到 `steam_sdk_downloading`。
- 恢复安装页游戏下载进度条：复用 `install-helpers.ts` 里的 `extractSteamDownloadProgress()` / `calcSteamDownloadTaskProgress()`，在 Steam 认证卡内显示文件数、已下载/总大小和进度条，不再只是静态“下载中”提示。
- 同步修正旧 QR/Guard 抢状态：同样的 `Choice [1]: 2` 会根据最近菜单上下文解释；认证方式菜单下表示 QR，`Steam Guard Authentication` 菜单下表示输入验证码。历史 `s.team/q` URL 只在当前没有 Guard/下载更新日志时兜底显示扫码。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`。未改后端接口、Steam 输入 API、SSE、安装任务或 Junimo 通信。
- 验证：`cd frontend; npm.cmd run build` 通过；内置 Browser 打开 `qa-layout.html?state=running`，页面非空、无 framework overlay、console error/warn 为空、横向溢出为 0。现有 QA 壳没有活跃安装 job 与 Steam 下载日志，真实下载进度需在安装任务活跃时联调。

# FE-INSTALL-STALE-PHASE-1 安装页旧 phase 防卡死

- 安装页现在优先根据活跃 `stardew_install` job 判断是否真的在安装：如果没有 queued/running 安装任务，但实例仍残留 `pull_running` / `steam_auth_running` / `steam_qr_required` 等运行中 phase，会显示为 `install_interrupted`，不再卡在 48% 或继续提示正在 Steam 认证。
- 当没有活跃安装任务时，页面会自动加载最近一次 `stardew_install` job 的详情和日志，便于看到失败原因；有新的活跃任务出现时仍优先切换到新任务。
- `install_interrupted` 被纳入认证失败/可重试显示链路，进度条、步骤、状态文案和重试按钮都会按中断失败处理。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`。接口不变，仍使用 `GET /api/jobs`、`GET /api/jobs/:id/logs`、`GET /api/instances/:id/state`。
- 验证：`cd frontend; npm.cmd run build`。

# FE-OPSRAIL-MAINTENANCE-PHASE-1 右栏维护窗口阶段展示

- 右栏“进行中”卡的计划重启展示从单纯依赖 `nextShutdownAt/nextStartupAt` 倒计时，改为按 `shutdownTime/startupTime` 在前端派生当前维护窗口阶段。
- 关机时间到达后不再立刻跳到下一天倒计时：服务器尚未停止时显示 `关机中 / 等待关机结束`；停止完成但开机时间未到时显示 `自动开机` 倒计时；开机时间到达后显示 `开机中 / 等待开机结束`；只有计划开机 job 成功后才切回下一天的自动关机/自动开机倒计时。
- 计划维护对应的 `stardew_lifecycle` job 会被语义化阶段行吸收，不再和 `关机中/开机中` 重复显示一条生硬的生命周期任务。普通手动生命周期任务仍按原有运行中任务逻辑展示。
- 实现仍复用现有 `GET /api/instances/:id/restart-schedule`、实例状态、jobs 与 job logs，不新增后端接口，不改变调度器、权限或 Junimo 通信。
- 影响文件：`frontend/src/games/stardew/StardewPanel.tsx`。
- 验证：`cd frontend; npm.cmd run build` 通过；内置 Browser 打开 `http://127.0.0.1:5173/qa-layout.html?state=running`，确认 Stardew Shell 和右栏“进行中”正常渲染、无 Vite overlay、console error/warn 为空。真实路由当前停在登录页，未做登录态截图。

# FE-OVERVIEW-METRIC-TYPE-UNIFY-1 总览统计卡字体统一

- 按用户截图反馈修正总览页四个统计卡（存档 / 模组 / 系统健康 / 运行任务）标题、数字、单位和状态徽章字体割裂的问题。
- 在 `StardewPanel.css` 文件尾部新增 `.sd-ov-metric-strip` 专属覆盖：四张 `.sd-mc` 统一使用 Verdana / Microsoft YaHei / SimHei 字体链；标题收为 `14px/800`，数字从过重的 `38px/900` 调整为 `34px/800`，单位、说明和徽章也继承同一字体链并降低字号/字重差异。
- 数字阴影同步减轻，保留轻微像素高光但去掉“海报粗字”感；仅影响总览统计卡，不改其它页面卡片、TSX、API、权限、轮询或 Junimo 通信。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；内置 Browser QA 打开 `http://127.0.0.1:5173/qa-layout.html?state=running`，1536x1024 下 4 张卡均为统一字体链，标题 `14px/800`、数字 `34px/800`，console error/warn 为空、overlay 为 0、横向溢出为 0；点击“服务器”再回“总览”后统计卡仍正常；390x844 下 4 张卡单列显示且无横向溢出。Browser `domSnapshot()` 仍有既有兼容错误，本次用 evaluate/截图/console 验证。

# FE-RESTART-SCHEDULE-PUT-WRITE-MODEL-1 计划重启保存请求体收口

- 修复服务器控制页“计划重启”弹窗保存时报 `request body must be valid JSON` 的问题。根因是前端读取 `GET /api/instances/:id/restart-schedule` 后把完整 `schedule` DTO 存入草稿，保存时又原样 PUT 回去，额外带上 `instanceId/nextShutdownAt/nextStartupAt/lastStatus/lastMessage/createdAt/updatedAt` 等只读展示字段；后端 `decodeJSON` 开启 `DisallowUnknownFields()`，因此在进入业务校验前返回 `invalid_json`。
- 新增 `RestartScheduleUpdate` 窄类型，只包含后端允许写入的 7 个字段：`enabled/shutdownTime/startupTime/timezone/warningMinutes/backupBeforeShutdown/skipIfPlayersOnline`。
- `updateRestartSchedule()` 现在在 API helper 内显式投影请求体，再调用 `PUT /api/instances/:id/restart-schedule`。弹窗仍可保留后端返回的 next/last 展示字段用于 UI 展示，但不会再随保存请求回传。
- 影响文件：`frontend/src/types.ts`、`frontend/src/api.ts`。未改弹窗交互、后端接口、计划重启调度器、权限判断或 Junimo 通信。
- 验证：`cd frontend; npm.cmd run build` 通过。

# FE-STOPPED-STATUS-RED-1 总览与服务器页停止态红字

- 按用户截图反馈，总览页“服务器控制”状态行和服务器控制页里的“已停止”改为红色字样，不再沿用运行态绿色。
- 总览页为 `sd-lifecycle-status-val` 增加状态后缀类，`stopped` 状态下使用红色；停止态状态点同步改为红点。
- 服务器控制页顶部 `ServerSummaryCard` 的服务器状态和生命周期控制卡下方状态行都增加 `stopped` 状态类，停止态文字为红色；生命周期卡补回“状态 · 已停止”小状态行，和用户截图中的位置一致。
- 影响文件：`frontend/src/games/stardew/pages/OverviewPage.tsx`、`frontend/src/games/stardew/pages/ServerControlPage.tsx`、`frontend/src/games/stardew/ServerSummaryCard.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未改生命周期 API、按钮 handler、权限、轮询或 Junimo 通信。
- 验证：`cd frontend; npm.cmd run build` 通过；内置 Browser QA 打开 `qa-layout.html?state=stopped`，总览 `已停止` computed color 为 `rgb(192, 32, 32)`；点击“服务器”后，摘要卡与生命周期状态行 `已停止` 均为 `rgb(192, 32, 32)`；390x844 下总览和服务器页同样为红色，页面级横向溢出为 0，console error/warn 为空。Browser `domSnapshot()` 仍有既有兼容错误，本次用 evaluate/截图/console 验证。

# FE-PLAYERS-OFFLINE-ROSTER-COUNT-1 玩家页离线名册计数修正

- 配合后端 `PLAYERS-SAVE-ROSTER-1`，玩家页现在可能收到 `status=offline`、`source=save_file` 的存档离线玩家；标题里的“等待加入”徽章不再用“非 online”派生，而是只统计 `waiting/pending/joining`。
- 表格状态列保持现有展示：`online` 显示在线绿点，`waiting/pending/joining` 显示等待黄点，其它状态包括 `offline` 显示离线灰点。
- 影响文件：`frontend/src/games/stardew/pages/PlayersPage.tsx`。未改玩家管理按钮、轮询、权限、后端接口路径或 Junimo 通信。
- 验证：`cd frontend; npm.cmd run build` 通过；`cd backend; go test ./internal/games/stardew_junimo` 通过。

# FE-JOBS-LOG-SCROLL-LOCK-1 任务日志页外层滚动锁定

- 修复点击“任务与日志”后整套 Stardew Shell 被浏览器页面纵向卷走、顶部状态栏消失、底部露出黑色背景的问题。
- 根因：`.sd-shell` 通过 `height: calc(100dvh / var(--sd-ui-scale))` + `transform: scale(...)` 适配 1536x1024 设计稿，视觉尺寸虽然贴合视口，但未缩放的布局盒子会让 `body/#root` 产生页面级纵向滚动；任务日志里 `scrollIntoView()` 又会把外层 `window` 一起滚动。
- `App.css` 新增 `body:has(.sd-shell)` 和 `#root:has(.sd-shell)` 视口锁定：Stardew 主界面挂载时 `height: 100dvh`、`overflow: hidden`，避免浏览器外层滚动条参与 Shell 布局。登录/初始化页不含 `.sd-shell`，不受该规则影响。
- `JobsLogsPage.tsx` 的日志自动滚到底改为滚动 `.sd-jobs-log-window` 自身；`InstallPage.tsx` 的安装日志同样改为滚动 `.sd-install-log-window` 自身，避免同类外层滚动回归。
- 影响文件：`frontend/src/App.css`、`frontend/src/games/stardew/pages/JobsLogsPage.tsx`、`frontend/src/games/stardew/pages/InstallPage.tsx`。未改任务/安装 API、SSE、权限、轮询、路由或 Junimo 通信。
- 验证：`cd frontend; npm.cmd run build` 通过；内置 Browser QA 打开 `http://127.0.0.1:5173/qa-layout.html?state=running`，1112x920 下修复前点击“任务日志”会让 `window.scrollY=351` 且 `.sd-shell.top=-351`；修复后 `documentElement.scrollHeight=clientHeight=920`、`body/root overflow=hidden`、点击任务日志后强制 `window.scrollTo(0, 600)` 仍保持 `scrollY=0`，`.sd-shell.top=0`，console error 为空。
- 下一步注意：后续新增日志/终端自动滚动时不要再对 Shell 内部元素直接调用 `scrollIntoView()`；优先滚动最近的局部日志容器，防止重新触发页面级滚动。

# FE-MOBILE-NAV-BAR-SIZE-1 单栏顶部选择栏放大

- 按用户截图反馈，单栏状态下顶部横向选择栏过小，本次只调整 `frontend/src/games/stardew/StardewPanel.css` 的 `@media (max-width: 640px)` 导航尺寸。
- 单栏 shell 第二行从 `40px` 提高到 `48px`，横向导航 padding/gap 略增，图标按钮从 `36x30` 调整为 `42x38`，导航图标从 `20px` 调整为 `23px`。
- 仅影响窄屏/单栏的顶部横向导航栏；未修改路由、导航数据、页面组件、API、权限或 Junimo 通信。
- 验证：`cd frontend; npm.cmd run build` 通过；内置 Browser QA 打开 `qa-layout.html?state=stopped`，在 490x844 与 390x844 下确认导航栏变大、点击“服务器”后激活态切换正常，console error/warn 为空，页面级横向溢出为 0。

# FE-OVERVIEW-LIFECYCLE-LEFT-1 总览页启动按钮左对齐

- 按用户截图反馈，把总览页“服务器控制”区里的启动/停止/重启按钮从左侧生命周期区域中间移回左侧，与“服务器控制”标题和状态行对齐。
- 根因是旧的 `.sd-lifecycle-actions` 规则留下了 `flex-wrap: wrap` 和 `align-content: center`，后续纵向生命周期覆盖只设置 `flex-direction: column` / `align-items: flex-start`，导致 flex line 仍按横向居中排布。
- 本次在总览最终覆盖段补充 `align-content: flex-start`、`flex-wrap: nowrap`，并让 `.sd-lifecycle-btns` 显式 `align-self: flex-start`。仅修布局对齐，不改按钮尺寸、点击 handler、启动/停止/重启 API、邀请码刷新或 Junimo 通信。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；内置 Browser QA 打开 `frontend/qa-layout.html?state=stopped`，默认视口下按钮相对服务器控制卡左边距从 `155px` 变为 `12px`，与标题左边距一致；点击启动按钮后进入“启动中…”且仍为 `12px`；390x844 下按钮相对左边距为 `9px`、无页面级横向溢出、console error/warn 为空。

# FE-SAVES-BACKUP-POLICY-LAYOUT-1 存档页自动备份策略卡布局修正

- 按用户截图反馈修正存档页“自动备份策略”卡片文字错乱：定时备份项调整为同一行按“勾选框 / 定时备份 / 每天 / 时间选择框”的阅读顺序排列，不再把“定时备份”挤到下一行或错位到左侧。
- “每日快照保留 N 天”拆成稳定的标签与数值组合，滑杆占用剩余宽度，避免窄卡片中中文文本和 range 控件互相挤压。
- 备份区域增加 `align-items: start`，左侧策略卡按自身内容高度收住，不再被右侧备份列表卡拉伸出大段空白。
- 影响文件：`frontend/src/games/stardew/SavesSection.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未改备份策略保存、备份列表、恢复/删除、权限判断、API 或 Junimo 通信。
- 验证：清理过期 `node_modules/.tmp/*.tsbuildinfo` 后 `cd frontend; npm.cmd run build` 通过；QA mock 全壳打开 `frontend/qa-layout.html` 点击“存档”，Edge/Playwright 截图确认策略卡宽 `250px`、高 `179px`，定时备份行无溢出，console error/warn 为空。

# FE-TOPBAR-BRAND-LIGHTER-1 顶栏品牌标题再减重

- 按用户反馈把 Stardew Shell 左上角 `Stardew Anxi Panel` 品牌标题再调细：`.sd-topbar-brand-text` 字重从 `800` 降到 `700`，并减轻暗色描边/投影层数。
- 只影响顶栏品牌文字本身；未改顶栏状态牌、存档框、版本框、用户框、路由、API、权限、轮询或 Junimo 通信。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build`。

# FE-PLAYERS-ACTION-ICONS-IMAGE2-1 玩家页活动行与管理图标修正

- 按用户截图反馈，玩家页“玩家活动 / 最近事件”列表文字被挤压的问题已修正：分页从每页 3 条改为每页 2 条，事件行高度提高，标题/徽章允许换行，描述使用正常行高，不再被 44px 行高和 `overflow:hidden` 压住。
- “管理操作”四个图标不再使用 CSS 临时绘制的靴子、禁入、清单、星星色块；使用内置 imagegen 按 image2/Stardew 像素风生成 2x2 图标 sheet，抠透明后切成 4 个项目内 PNG：踢出玩家、封禁玩家、白名单管理、权限设置。
- 新增素材：`frontend/public/assets/stardew/ui/icons/icon_players_action_sheet_image2.png`、`icon_players_action_boot_image2.png`、`icon_players_action_ban_image2.png`、`icon_players_action_whitelist_image2.png`、`icon_players_action_permission_image2.png`。
- `StardewPanel.css` 的玩家页最终覆盖改为引用新 PNG，重置旧 CSS 图标的 border/clip-path/background；桌面下活动卡与管理卡仍等高，移动端继续自然堆叠。
- 影响文件：`frontend/src/games/stardew/pages/PlayersPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`、上述 5 个 PNG 素材。未改后端 API、玩家事件接口、管理操作权限或 Junimo 通信。
- 验证：`cd frontend; npm.cmd run build` 通过；内置 Browser QA 使用 `frontend/qa-layout.html` 渲染玩家页，1536x1024 下活动列表每页 2 条、事件描述不裁切、活动/管理均为 `260px` 高、4 个图标均加载新 PNG、分页可切到 `2/3`；390x844 下无页面级横向溢出，console error/warn 为空。Browser `domSnapshot()` 仍有兼容错误，已用 evaluate/截图/console 验证。

# FE-TOPBAR-SAVE-STATUS-TYPE-1 顶栏存档/状态/标题字体微调

- 按用户截图反馈调整 Stardew Shell 顶栏：品牌标题从过粗的 `Arial Black` 改为更轻的 Verdana 系像素描边效果，字号降到 `28px`、字重 `800`，描边从 2px 收到 1px，避免标题像海报粗体。
- 顶栏状态按钮在 `running/stopped` 两种状态下改用现有像素状态牌素材：`panel_status_running_image2.png` / `panel_status_stopped_image2.png`，视觉上直接显示“运行中/已停止”牌面；其它读取中、启动中、异常等状态仍保留原有文字和点位逻辑。
- 存档框移除右侧下拉箭头，农场图标向左贴近框边；文本改为“农场名：简略游戏时间”，例如 `AnxiFarm：第一年春`，只展示年份和季节，不再展示具体日期，也不再写“世界：”。
- 用户角色框移除右侧下拉箭头，保留头像、角色文字和在线绿点；点击行为仍进入设置页。
- 影响文件：`frontend/src/games/stardew/StardewPanel.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未改后端 API、存档数据结构、路由、权限或 Junimo 通信。
- 验证：`cd frontend; npm.cmd run build` 通过；内置 Browser QA 打开 `http://127.0.0.1:5173/qa-layout.html?state=running/stopped`，1536x720 下确认运行/停止状态牌素材生效、存档框显示 `AnxiFarm：第一年春`、存档和用户框下拉箭头数量为 0、无横向溢出、无 Vite overlay、console error/warn 为空；390x844 下顶栏仍隐藏存档/用户框且无横向溢出。

# FE-MODS-PROTOTYPE-V02-LAYOUT-1 模组页按 version-02 原型比例回正

- 模组管理页按 `C:/Users/anxi/.codex/generated_images/019f2c2c-8909-7262-bd81-d31356799c21/_sorted_overview_to_settings/version-02-current-frontend-code/06-mods.png` 回正首屏卡片顺序和比例：顶部标题 + 三个操作按钮，下面固定为三段标签页、Nexus 连接横条、搜索 Nexus Mods 卡、2x2 搜索结果卡和分页。
- 下载页结果卡恢复原型的两按钮结构（在 Nexus 查看 / 一键安装），移除此前下载卡底部额外的 `N站会员专属安装` 按钮；一键安装继续走浏览器扩展批量安装路径，Nexus API Key 仍保留在连接栏配置入口。
- 热门标签行改成真实快捷搜索按钮，点击会写入搜索框并调用现有 `searchNexusMods()`，避免只有视觉占位；当前标签为 `UI Info`、`Fishing Mod`、`Backpack Upgrades`、`Tractor`。
- 按用户截图反馈移除下载页底部“扩展安装进度”横条和搜索区“全部类别”下拉框；搜索框提示改为“输入英文模组名称、ID 或关键词...”。
- 模组页工作台、连接条、搜索卡、搜索结果卡复用其它页面统一羊皮纸卡片变量：`--sd-save-card-bg`、`--sd-save-card-border`、`--sd-save-card-shadow`，卡片为 2px 铜色边框和 9px 圆角。
- 搜索结果卡的前置状态统一放入统计行，固定跟在“认可”后面；无前置时显示“前置：无”，有前置时显示“前置已满足 / 缺少前置mod”等原有状态按钮。这样每张卡的“跳转 N站 / 一键安装”操作区保持同一垂直位置。
- 搜索卡高度从之前动态分页用的 `246px` 收回到原型首屏两行节奏，`NEXUS_SEARCH_CARD_HEIGHT` 改为 `198`，桌面 1536x1024 下保持 2 列 4 卡可见；移动端自动单列且无页面级横向溢出。
- 影响文件：`frontend/src/games/stardew/pages/ModsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未改后端 API、Mod 上传/删除/导出、启用状态切换、玩家同步包导出或 Junimo 通信。
- 验证：`cd frontend; npm.cmd run build` 通过；内置 Browser QA 使用 `frontend/qa-layout.html?state=running` 渲染真实 `StardewPanel`，1536x1024 下确认结果卡 4 张、连接栏/搜索卡/分页按原型落位、`selectCount=0`、`progressCount=0`、`premiumButtons=0`、无页面级横向溢出；第一行两张卡操作区同为 `y=628`、第二行同为 `y=836`，前置状态文本位于统计行；390x844 下单列滚动且 `overflowX=0`；点击热门标签 `Tractor` 后搜索框值为 `Tractor`。Browser dev log 缓冲区保留了热更新过程中的旧错误，最终页面 `overlayCount=0` 且构建通过。

# FE-DIAGNOSTICS-GAUGE-INNER-SAFE-1 诊断页资源圆环数字安全区修正

- 按用户反馈修正诊断页资源趋势三张圆环卡中红色弧线遮挡百分比数字的问题：在原型卡片比例不变的前提下，将圆环最小宽度提高到 `clamp(98px, 7.2vw, 108px)`，中心可读底色圆扩大到圆环的 `68%`，并把数字字号调整为 `clamp(19px, 1.85cqi, 23px)`、百分号为 `10.5px`。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。未改 `DiagnosticsPage.tsx`、资源指标 API、轮询、健康检查、导出诊断包或 Junimo 通信逻辑。
- 验证：`cd frontend; npm.cmd run build`；内置 Browser QA 打开 `frontend/qa-layout.html?state=running` 后点击“诊断”，确认 CPU/内存/磁盘三张卡的数字位于中心底色圆内且不再被弧线遮挡，console error/warn 为空。

# FE-PLAYERS-TIME-EVENTS-PAGING-1 玩家页时间列与活动分页微调

- 按用户反馈继续微调玩家管理页：在线时长列改为短时间点格式，优先显示“今天 HH:mm / 昨天 HH:mm / N天前 HH:mm”，避免长时长字符串挤压收入列；没有可计算时间点时才回退到旧 `onlineFor` 文案。
- 在线玩家表收入列顺序已调整为“玩家收入 / 农场收入”，对应表头和行数据同步对调。
- “玩家活动 / 最近事件”改为分页展示，每页 3 条，底部显示上一页/下一页和页码；桌面下活动卡与右侧“管理操作”卡固定同高，移动端恢复自然高度单列堆叠。
- `frontend/src/qa-layout-main.tsx` 的未跟踪 QA mock 补充多条玩家事件，用于验证分页按钮和昨天/几天前格式；产品接口和后端契约不变。
- 影响文件：`frontend/src/games/stardew/pages/PlayersPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`、`frontend/src/qa-layout-main.tsx`。未改后端 API、玩家轮询、权限判断或 Junimo 通信。
- 验证：`cd frontend; npm.cmd run build` 通过；内置 Browser QA 使用 `frontend/qa-layout.html` 渲染玩家页，1536x1024 下收入列顺序正确、在线时间为短格式、活动/管理均为 `248px` 高、分页从 `1/2` 切到 `2/2` 正常、console error/warn 为空；390x844 下无页面级横向溢出。

# FE-SAVES-UPLOAD-BLUE-BG-1 存档上传条恢复蓝色背景

- 按用户反馈，存档页“拖拽存档文件到此处或点击上传”横条背景从羊皮纸虚线样式恢复为之前的蓝色天空版本：蓝色渐变底、白色像素云块、木色实线边框和内高光。
- 仅修改 `.sd-saves-page .sd-saves-upload-strip` 的视觉背景/边框/阴影；上传入口 DOM、按钮文案、弹窗、预览、导入并启动 handler 和权限/运行中禁用逻辑均未改。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；QA mock 全壳存档页截图确认上传条已恢复蓝色背景，console error/warn 为空。

# FE-SETTINGS-API-PORT-REMOVE-1 设置页移除 API 端口展示

- 按用户要求，设置与审计页的“端口信息”卡片移除只读的“API 端口”字段，仅保留“面板端口 / VNC 端口 + 保存/刷新”。
- `SettingsPage.tsx` 只删除显示用的 API 端口 label/input；`StardewPanel.css` 将端口行从三端口列收紧为两端口列。VNC 端口读取、保存、权限判断、提示文案和后端接口均未改。
- 影响文件：`frontend/src/games/stardew/pages/SettingsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未改后端 API、Junimo 通信、用户管理、审计日志或轮询逻辑。

# FE-SAVES-V02-PROTOTYPE-LAYOUT-1 存档页按 version-02 原型卡片比例回正

- 存档管理页按 `C:/Users/anxi/.codex/generated_images/019f2c2c-8909-7262-bd81-d31356799c21/_sorted_overview_to_settings/version-02-current-frontend-code/03-saves.png` 对齐卡片位置和比例：激活存档区改为“信息卡 + 右侧操作卡”双卡，存档库操作按钮上移到栏目标题右侧，存档卡固定为桌面三卡一行，上传条与底部备份区跟随原型顺序。
- 备份与恢复区从单个纵向大面板改为底部两列：“自动备份策略”窄卡 + “备份列表”宽卡；保留原有备份策略、刷新、恢复、删除、保存设置等 handler 和禁用逻辑。
- `SavesSection.tsx` 新增中文 `farmType` 到已有农场缩略图资源的映射，兼容 mock/旧数据里直接返回“标准农场、河边农场、森林农场”等中文值时缩略图不显示的问题。
- 影响文件：`frontend/src/games/stardew/SavesSection.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未改 API、权限判断、轮询、创建/上传/选择/删除/备份/恢复业务逻辑。
- 验证：`cd frontend; npm.cmd run build`；QA 使用 `frontend/qa-layout.html` + mock fetch 渲染真实 `StardewPanel`，Edge/Playwright 截图 1536x1024 对照原型，确认激活区 160px、右操作卡独立、存档库 3 卡同排、上传条和底部备份双栏落位；390x844 无页面级横向溢出、无 Vite overlay、console error/warn 为空。

# FE-SETTINGS-PROTOTYPE-V02-LAYOUT-2 设置页按 version-02 原型卡片比例回正

- 按用户要求，把设置与审计页对齐 `C:/Users/anxi/.codex/generated_images/019f2c2c-8909-7262-bd81-d31356799c21/_sorted_overview_to_settings/version-02-current-frontend-code/09-settings.png` 的卡片位置和比例：左列固定为“面板版本 / 用户管理 / 端口信息 / 其他设置”，右列固定为“安全与权限 / 审计日志 / 安全建议”，桌面比例约 `1.11fr / 0.89fr`，不再被统一小卡片规则改成圆角通用卡。
- `SettingsPage.tsx` 补回原型结构：面板版本卡新增右侧图像槽；安全与权限从两列摘要改为单列表格式并增加中间说明列；端口信息当时改为三端口横排，后续 `FE-SETTINGS-API-PORT-REMOVE-1` 已移除重复的“API 端口”；审计日志首屏页大小改为 7 条；安全建议收敛为三条带状态徽章和底部“前往安全设置”按钮。
- `StardewPanel.css` 文件尾部新增设置页最终覆盖：仅在 `.sd-main:has(.sd-settings-page)` 下收紧主 frame 上下 inset，1536x1024 下七张设置卡均进入首屏，用户表操作按钮不再换行，其他设置六行完整可见；390x844 下右栏隐藏、设置页单列、无页面级横向溢出。
- 影响文件：`frontend/src/games/stardew/pages/SettingsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未改后端 API、用户管理/审计/VNC 端口权限判断、轮询或 Junimo 通信。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器 + 临时 `settings-qa.html` mock 入口渲染真实 `StardewPanel` 设置页，1536x1024 下 section 数为 7、无横向溢出、console error/warn 为空，并用 `view_image` 对比原型和最终实现截图；390x844 下无横向溢出，点击“新建用户”可展开真实表单。QA 临时文件已删除。

# FE-SERVER-PROTOTYPE-V02-LAYOUT-2 服务器页按 version-02 原型卡片比例回正

- 按用户要求，把服务器控制页对齐 `C:/Users/anxi/.codex/generated_images/019f2c2c-8909-7262-bd81-d31356799c21/_sorted_overview_to_settings/version-02-current-frontend-code/02-server.png` 的卡片位置和比例：顶部“服务器摘要”恢复为整行大卡，内部为状态/在线玩家/当前存档/主机农民/游戏日期一排字段，下方邀请码横条；中部为“生命周期控制”左列、“快捷操作”右列工具行；“全服消息”在左列；“控制台命令”底部横跨整行且黑色终端满宽。
- `ServerSummaryCard` 不再复用玩家页六宫格统计卡 DOM，改为服务器页专用摘要结构，避免摘要被玩家页 `.sd-players-overview-grid` 样式撑成两行大卡。`InviteCodeCard` 增加可选 `label/description`，服务器摘要中显示原型式“邀请码”，其它调用保持默认“邀请加入码”。
- `ServerControlPage` 的快捷操作按钮改成原型式浅色工具行：图标 + 主标题 + 说明/状态，保留原有手动备份、计划重启、VNC 显示、跳转 VNC、服务器设置的 disabled、权限、点击 handler 和待接入逻辑。
- `StardewPanel.css` 文件尾部新增服务器页最终覆盖：桌面 1536x1024 下主内容约 `937px`，摘要约 `175px`，生命周期/快捷操作并排，消息/命令位置进入首屏；`880px` 以下恢复单列顺序（摘要 -> 生命周期 -> 快捷操作 -> 全服消息 -> 控制台命令），移动端无页面级横向溢出。
- 影响文件：`frontend/src/games/stardew/InviteCodeCard.tsx`、`frontend/src/games/stardew/ServerSummaryCard.tsx`、`frontend/src/games/stardew/pages/ServerControlPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未改 API、权限、轮询、Junimo 通信或后端逻辑。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器 + 临时 mock QA 入口渲染真实 `StardewPanel` 服务器页，1536x1024 下 console error/warn 为空，摘要/生命周期/快捷操作/消息/命令终端均在原型式首屏布局中，终端满宽；390x760 下 console error/warn 为空、`overflowX=0`、单列顺序正确。临时 QA 文件已删除。

# FE-OVERVIEW-BANNER-SCENE-IMAGE2-1 总览横幅场景替换为 image2 原型素材

- 总览页顶部农场横幅的场景素材已从“CSS 天空/田地 + `sprite_farmhouse_scene.png` 小农舍”替换为 image2 原型 `C:/Users/anxi/.codex/generated_images/019f2c2c-8909-7262-bd81-d31356799c21/_sorted_overview_to_settings/version-02-current-frontend-code/01-overview.png` 中对应的农场横幅场景裁切图。
- 新增运行时素材 `frontend/public/assets/stardew/ui/sprites/overview_banner_scene_image2.png`，裁切范围只包含总览顶部农场场景，不包含下方统计条、左侧导航、右栏或主内容 frame。
- `StardewPanel.css` 文件尾部新增最终覆盖：`.sd-ov-banner-bg` 直接使用新 PNG，隐藏旧的横幅伪元素纹理和旧 `sprite_farmhouse_scene.png` 叠层，避免 CSS 田野/小农舍残留。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`、`frontend/public/assets/stardew/ui/sprites/overview_banner_scene_image2.png`。未改 `OverviewPage.tsx`、接口、权限、轮询或后端逻辑。
- 验证：已预览新 PNG，尺寸 `1015x170`，确认仅含农场横幅场景；`cd frontend; npm.cmd run build` 通过。

# FE-DIAGNOSTICS-PROTOTYPE-V02-LAYOUT-1 诊断页按 version-02 原型比例回正

- 诊断页按 `C:/Users/anxi/.codex/generated_images/019f2c2c-8909-7262-bd81-d31356799c21/_sorted_overview_to_settings/version-02-current-frontend-code/07-diagnostics.png` 的卡片位置和比例回正：顶部状态横卡加高，三枚统计卡维持右侧一行；中部改回检查项表与资源趋势等宽双列；底部告警与建议继续横跨全宽并回到首屏内。
- 仅对诊断页使用 `.sd-main:has(.sd-diag-page)` 收紧主 frame inset，把内容左缘从约 284px 拉回约 242px，主卡宽从约 881px 拉回约 975px；没有调整全局 Shell、其它页面或右侧栏宽度。
- `DiagnosticsPage.tsx` 只调整资源仪表 DOM 顺序：标题放在仪表卡顶部、圆环居中、说明放在底部，并把趋势图标题改为“资源使用趋势（24小时）”。`getHealthDiagnostics()`、`downloadSupportBundle()`、`getInstanceMetrics()`、管理员权限、loading/error/disabled 和 5s 轮询逻辑保持不变。
- 资源趋势卡内部收紧：三枚 gauge 卡保持三列、每卡约 143x174；趋势图高度从偏高的大图压到原型式短图；检查项表与资源趋势面板在 1536x1024 QA 视口下约 482x392。
- 影响文件：`frontend/src/games/stardew/pages/DiagnosticsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；因本轮没有可用 IAB 控制工具，使用 Playwright + 本机 Chrome 回退验证 `qa-layout.html` mock 全壳。1536x1024 下诊断页无 console error/warn、无横向溢出，关键尺寸为 status `975x160`、check/resource `482x392`、advice `975x180`；390x844 下无横向溢出。

# FE-PROTOTYPE-SHELL-ALIGN-1 九页布局对齐 image2 原型（栏宽再平衡）

- 起因：用户反馈"现在的布局大小太丑"，希望九页前端布局完全对齐 `C:/Users/anxi/.codex/generated_images/.../version-02-current-frontend-code/01..09` 原型图。定位到根因：1536 视口下右信息栏 `414px`（过肥）+ 左导航 `252px` + 主 frame 厚留白，把主内容区挤到只有 **791px**，导致统计卡被迫 2×2、控制区/邀请码上下堆叠、任务/服务器/玩家页该并排的区块坍成单列。
- 关键修复（一处变量改动全局生效）：`--sd-sidebar-width` 从 `clamp(210px,16.8vw,252px)` 收到 `clamp(196px,14vw,216px)`；`--sd-opsrail-width` 从 `clamp(340px,27vw,430px)` 收到 `clamp(268px,19vw,300px)`。主内容区从 791 → **937px**，总览统计卡恢复 4 卡一行、服务器控制 + 邀请码并排，全九页不再拥挤。栏宽比例经用户确认采用。
- 逐页对齐（均在 `StardewPanel.css` 内改，未动任何 TSX/handler/API/权限/后端）：
  - 顶栏版本框加宽 `9.1%→11.4%` 并 `white-space:nowrap`，`v1.6.15 (Stable)` 不再折行。
  - 总览邀请码卡在窄列（≈500px）内收敛：`grid-template-columns` 改 `minmax(96px,0.6fr) minmax(0,1fr) auto`、代码字号收小、复制/刷新按钮 `min-width:64px`，不再裁切溢出。
  - 服务器页 `@container` 坍缩断点 `1180px→880px`，937 下恢复"生命周期控制 | 快捷操作"并排；`.sd-server-quick-grid` 从横向 flex-wrap 改为纵向列表（`flex-direction:column`，按钮整宽），对齐原型右列竖排快捷操作。
  - 任务日志页坍缩断点 `940px→820px`，937 下恢复"任务列表 | 任务详情+终端"两列。
  - 玩家页把 `FE-PLAYERS-LIST-LEFT-1` 段重写为 `FE-PLAYERS-PROTOTYPE-LAYOUT-2`：在线玩家表整行、玩家活动(左) | 管理操作(右)两列、Junimo 终端整行；坍缩断点 `900px→820px`。**注意：本项逆转了 `FE-PLAYERS-LIST-LEFT-1`（表左/事件右）以对齐新原型。**
  - 诊断页 `.sd-diag-main-grid` 比例改 `1.08fr | 0.92fr`（检查表更宽）、检查行列宽重排、`.sd-diag-check-msg` 改单行 `nowrap + ellipsis`，信息列（如 `/data/stardew | 可用 215.8 GB`）不再折成两行。
  - 设置页 `.sd-settings-user-row` 与 `.sd-settings-audit-*` 列宽收紧、gap 减小，用户表操作按钮与审计表 IP 列不再裁切。
  - 存档页上传区 `.sd-saves-upload-strip` 从蓝色邮筒渐变改为羊皮纸虚线拖拽区（tan 虚线边 + 纸底 + 棕色文字），对齐原型并与整体羊皮纸风统一。
- 影响文件：仅 `frontend/src/games/stardew/StardewPanel.css`。未新增/修改任何组件、handler、API、权限、轮询或后端逻辑。
- 验证：`cd frontend; npm run build`（`tsc -b && vite build`）通过。QA 用临时 mock-fetch harness（`qa-layout.html` + `src/qa-layout-main.tsx`，拦截 `window.fetch` 返回原型态数据渲染真实 `StardewPanel` 全壳）+ Playwright 1536×1024 逐页截图，与九张原型逐块对比确认结构一致、无拥挤/裁切/溢出、console pageerror 为 0；QA 临时文件已删除。真实登录态截图 QA 待补。
- 下一步注意：栏宽收窄后各页 `@container sd-main-scroll` 断点是按新的 937px 主宽重新校准的；若后续再调 `--sd-opsrail-width`/`--sd-sidebar-width`，需同步复核服务器(880)/任务(820)/玩家(820) 这几个坍缩断点，避免又落回单列。

# FE-SHELL-SCALE-1 Shell 全局等比缩放

- Stardew Shell 新增全局 `--sd-ui-scale`：以 `1536x1024` 为设计基准，按 `min(100vw/1536, 100dvh/1024)` 随窗口等比放大/缩小，并设置 `0.72` 最小可读比例；`.sd-shell` 使用反向 `width/height` + `transform: scale(var(--sd-ui-scale))`，让视觉尺寸始终填满当前浏览器可视区。
- 这次把缩放提升到 Shell 层：顶栏、左侧栏、主 frame、右 OpsRail、按钮、页面内容一起缩放；主内容区仍弹性吃掉宽屏多余空间，不把整页锁死成固定比例图片。
- 结构降级阈值同步调整：右 OpsRail 不再 960px 就隐藏，而是在低于最小全布局宽度附近（720px）才隐藏；640px 以下保留原有移动端顶部图标导航/单列内容规则。
- `StardewPanel.tsx` 的右栏自动折叠估算改为使用同一套设计基准、最小 scale 和当前栏宽公式，避免 JS 仍按旧宽栏尺寸过早收起 OpsRail。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`、`frontend/src/games/stardew/StardewPanel.tsx`。未改页面组件、API、权限、轮询或后端逻辑。
- 验证：`cd frontend; npm.cmd run build` 通过；临时本地 HTTP QA 页加载真实构建 CSS，测得 760x504 下 scale=0.72、三栏保留且无页面溢出，1920x1080 下 scale≈1.0547、Shell 视觉尺寸填满视口且按钮随之放大。真实登录态截图 QA 待补。

# FE-PLAYERS-LIST-LEFT-1 玩家表回到首屏左侧

- 玩家管理页桌面布局从“左侧最近事件 / 右侧在线玩家且下沉”调整为“左侧宽列在线玩家 / 右侧窄列最近事件”，减少首屏中间空白，让核心在线玩家表优先出现在左侧主位。
- 只在 `StardewPanel.css` 文件尾部新增覆盖：交换 `.sd-players-page` 双列比例，取消 `.sd-players-list-section` 固定 `grid-row: 3 / span 2`，最近事件固定到右列，服务器信息（Junimo）作为底部调试信息横跨整行。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。未改 `PlayersPage.tsx`、玩家接口、数据字段、权限判断、轮询或后端逻辑。
- 验证：`cd frontend; npm.cmd run build` 通过；真实登录态截图 QA 待补。

# FE-DIAG-GAUGE-TOMIK-1 诊断页资源圆环改为渐变描边样式

- 诊断页 CPU/内存/磁盘三个资源圆环从"铜钱"conic-gradient 样式改为 Tomik23 circular-progress-bar 风格：`#e6e6e6` 灰色底环 + yellow→`#ff0000` 线性渐变描边 + 圆头端帽（round）+ 中心百分比数字，纯 SVG 实现（linearGradient + stroke-dasharray/dashoffset），未引入该 JS 库或任何新依赖。
- `DiagnosticsPage.tsx` 的 `GaugeCard` 重写为 SVG 圆环组件：底环 circle + 渐变弧 circle（`rotate(-90)` 从顶部起画，`transition: stroke-dashoffset .6s` 平滑动画），`percent<=0` 或无数据时只画底环不画弧（避免 round 端帽在 0% 显示成小圆点）；`color` prop 移除，改为每卡传唯一 `gradientId`。"启动后显示"空态与 `formatGaugeNumber` 逻辑保持不变。
- `StardewPanel.css` 删除 `.sd-diag-gauge-ring` 的铜钱纹路（repeating-conic 刻齿、双层 radial 内芯 `::before/::after`、三层金圈 box-shadow），新增 `.sd-diag-gauge-svg/-track/-arc` 规则；中心数字颜色从每卡语义色改为页面墨色 `var(--sd-diag-ink)` 并去掉羊皮纸描边 text-shadow。
- 影响文件：`frontend/src/games/stardew/pages/DiagnosticsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；Playwright 真实登录态截图 1366x900 与 390x844——磁盘 11% 显示顶部黄→红渐变圆头弧，CPU/内存空态显示灰底环 + "—"，无 pageerror。

# FE-UNIFIED-CARD-PARCHMENT-TONE-1 小卡片统一浅羊皮纸色

- 将总览统计卡当前使用的浅羊皮纸暖黄提升为共享小卡片背景：在 `StardewPanel.css` 文件尾部覆盖 `--sd-save-card-bg` 和 `--sd-save-card-bg-strong`。
- 所有复用统一小卡片变量的非模组页小框都会跟随这组背景色；总览 `.sd-mc` 继续保持同色且无斜纹。
- 只改背景色变量，不改变卡片尺寸、边框、圆角、阴影、文字布局、状态徽章或业务 DOM。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过。

# FE-INSTALL-STEAM-AUTH-ICON-1 Steam 认证卡复用安装进度图标

- 安装页“Steam 认证”卡片中的占位大图标改为复用安装进度第三步的 `icon_install_step_steam_image2.png`，不再使用 CSS 渐变绘制的蓝色圆球。
- “Steam 认证”栏目标题左侧小图标同步改为同一张 Steam PNG 资源，保证标题图标、安装进度图标和认证占位图标风格一致。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未改安装状态、Steam 认证流程、Steam Guard/扫码交互、日志或后端接口。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器打开 `/instances/stardew/install` 当前停在登录页，确认应用壳非空且 console error/warn 为空，未完成登录态安装页截图验证。

# FE-SETTINGS-FILL-GAP-1 设置页两列堆叠补空

- 设置页布局从“顶部摘要 / 用户审计 / 底部端口”三段式，改为左右两列堆叠：左列为“面板版本 / 用户管理 / 端口信息 / 其他设置”，右列为“安全与权限 / 审计日志 / 安全建议”。
- 新增 `.sd-settings-content-grid` 和 `.sd-settings-stack`，在中等宽度下保持两列，让端口信息和其他设置上移填补左列空缺；`780px` 以下再回到单列，避免窄屏挤压。
- 影响文件：`frontend/src/games/stardew/pages/SettingsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未改设置页接口、权限、用户管理、审计日志或 VNC 端口逻辑。
- 验证：`cd frontend; npm.cmd run build` 通过。

# FE-INVITE-CARD-COPY-ORDER-1 邀请码卡片复制按钮与总览复用

- 新增共享 `InviteCodeCard`，统一渲染“邀请加入码”行的状态、复制按钮、刷新按钮和复制失败提示；`ServerSummaryCard` 不再自带第二套复制状态。
- 复制按钮现在位于刷新按钮左侧，只有存在邀请码时才渲染；无邀请码、获取中、获取失败、服务器未运行等状态只保留刷新按钮，不预留隐藏按钮列，避免卡片右侧空洞或窄屏挤压。
- 总览页服务器控制区已替换为同一 `InviteCodeCard`，移除原 `sd-invite-panel` 旧卡片 JSX、本地复制状态和 `handleCopy()`，服务器页与总览页后续共享同一邀请码交互。
- 布局调整：`.sd-players-invite-row` 改为“说明 / 代码状态 / 按钮组”三列，新增 `.sd-players-invite-actions` 承载“复制 + 刷新”；窄屏下按钮组整行铺满并按可用按钮数平分宽度。
- 影响文件：`frontend/src/games/stardew/InviteCodeCard.tsx`、`frontend/src/games/stardew/ServerSummaryCard.tsx`、`frontend/src/games/stardew/pages/OverviewPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器打开 `http://127.0.0.1:5173/instances/stardew/server` 与 `/instances/stardew/overview` 均停在登录页，确认应用壳非空且 console error/warn 为空；因缺少当前登录态未完成真实卡片截图验证。

# FE-OVERVIEW-METRIC-CLEAN-BG-1 总览统计卡去斜纹并提亮

- 总览页四个 `.sd-mc` 统计卡移除斜向 `repeating-linear-gradient` 纸纹，改为干净、偏浅的羊皮纸暖黄背景；后续按反馈从偏白略微压黄，但不恢复旧的高饱和黄色。
- 本次只覆盖统计卡背景，不改变卡片尺寸、边框、铆钉角饰、文字布局、状态徽章或总览其它卡片。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过。

# FE-INSTALL-HERO-SCENE-REMOVE-1 安装页移除顶部大场景图

- 安装页顶部状态横幅移除右侧大农舍场景图：删除 `InstallPage.tsx` 中的 `.sd-install-farm-scene` 节点，不再加载 `/assets/stardew/ui/sprites/sprite_farmhouse_scene.png` 作为安装页顶部大图。
- `StardewPanel.css` 清理 `.sd-install-farm-scene`、图片和遮罩伪元素规则；`.sd-install-status-banner` 从三列改为“小土芽图标 + 状态信息”两列，避免删图后留空。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未改安装状态、Steam 认证、日志、进度或后端接口。
- 验证：`cd frontend; npm.cmd run build` 通过。

# FE-SETTINGS-ACCOUNT-CARD-REMOVE-1 设置页移除当前账号卡

- 设置与审计页删除顶部“当前账号”卡片，避免和顶栏用户入口重复展示；顶部区域现在只保留“面板版本 / 安全与权限”两卡。
- `SettingsPage.tsx` 删除 `AccountSection` 组件和设置页内的退出登录按钮；登出入口仍保留在 Stardew Shell 顶栏，不改鉴权或 session 逻辑。
- `StardewPanel.css` 清理 `sd-settings-account-*` 死样式，并将 `.sd-settings-top-grid` 从三列调整为两列，窄屏仍按既有规则收为单列。
- 影响文件：`frontend/src/games/stardew/pages/SettingsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过。

# FE-PAGE-HEADER-SHADOW-1 页面标题阴影清理

- Stardew 各路由页头去掉标题文字、导航图标和右侧虚线分隔的阴影：在 `StardewPanel.css` 文件尾部新增统一覆盖，将页头图标 `filter`、标题 `text-shadow`、页头分隔线 `filter/box-shadow` 清零。
- 这次只清理页头阴影背景，不改变标题大小、位置、虚线分隔、按钮、卡片或页面布局。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过。

# FE-PAGE-TOP-ALIGN-1 页面顶部对齐兜底

- Stardew 各路由页面统一贴齐主内容 frame 顶部：在 `StardewPanel.css` 文件尾部新增 `.sd-main-scroll > .sd-page` 及各页面类的 `padding-block-start: 0` 覆盖。
- 这次只处理页面根容器顶部 padding，保留各页面既有左右/底部 padding、grid 布局、卡片结构和业务 DOM；用于抵消任务、诊断、安装、设置等页面后置皮肤规则重新写完整 `padding` 后造成的顶部下沉。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过。

# FE-SERVER-ACTION-CARDS-1 服务器页生命周期与快捷操作并排

- 服务器控制页动作区调整为“顶部服务器摘要卡整行 -> 生命周期控制左侧 / 快捷操作右侧 -> 全服消息整行 -> 控制台命令整行”；生命周期卡不再停在顶部右侧空位，改为下移到摘要卡之后。
- 快捷操作卡通过 CSS grid 放到生命周期右侧，窄屏容器查询下顺序改为摘要、生命周期、快捷操作、全服消息、控制台命令单列排列。
- 快捷操作按钮统一叠加 `.sd-btn--lg`，高度、字号和生命周期按钮使用同一 lg 令牌；删除快捷操作区原 64px 卡片式按钮布局和伪图标，改为与生命周期一致的 PNG 按钮尺寸。
- 影响文件：`frontend/src/games/stardew/pages/ServerControlPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未改快捷操作 handler、权限判断、disabled 状态或后端接口。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器打开 `http://127.0.0.1:5174/instances/stardew/server` 时真实应用停在登录页，确认页面非空、无框架错误覆盖、console error/warn 为空，未完成登录态服务器页截图验证。

# FE-SERVER-INVITE-IN-SUMMARY-1 服务器页邀请码入口收敛

- 服务器控制页移除中部独立“邀请代码”卡片，避免同一页面同时出现两处邀请码入口；顶部服务器摘要卡的“邀请加入码”行保留复制，并在邀请码显示区右侧放置“刷新”按钮。
- `ServerSummaryCard` 的刷新按钮现在在运行中或启动中可点击，服务器未运行时保留禁用态和 tooltip；复制按钮仍只在运行中且已有邀请码时显示。
- 删除 `ServerControlPage` 内独立邀请码卡片对应的本地复制状态和 `handleCopy`，邀请码复制/刷新统一走 `ServerSummaryCard` 和 `dashboardData.refreshInviteCode()`，未改 API、权限、轮询或启动/重启等待新邀请码逻辑。
- 布局调整：删除独立邀请码卡片后，服务器页“全服消息”横跨整行；`.sd-players-refresh-btn` / `.sd-players-copy-btn` 在桌面固定到邀请码右侧两列，窄屏重置为单列全宽，避免隐式列造成横向溢出。
- 影响文件：`frontend/src/games/stardew/ServerSummaryCard.tsx`、`frontend/src/games/stardew/pages/ServerControlPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器打开 `http://127.0.0.1:5174/instances/stardew/server` 因真实登录态停在登录页，确认页面非空且 console error/warn 为空；尝试 data 临时 QA 页被内置浏览器 URL policy 拦截，因此未完成真实服务器页截图验证。

# FE-BTN-UNIFY-1 九页面按钮与操作区统一化

- 按钮尺寸收敛为三档令牌（`stardew-theme.css` `:root` 变量）：lg `40px/15px`（生命周期启动/停止/重启、诊断页头部主操作、安装页主 CTA）、md `28px/13px`（默认档：工具栏/卡片/弹窗按钮）、sm `22px/12px`（表格与列表行内、迷你重试）。新增修饰符 `.sd-btn--lg` / `.sd-btn--sm` 可叠加在任何 `sd-btn-*` 上；`sd-btn-img` 图标尺寸随档位由 CSS 给定（20/15/12px），删除了所有 JSX 内联宽高。改造前同类按钮存在 20/26/33/38/46/48/50/52px 共 8 种高度。
- 语义色只保留三种：绿 `sd-btn-green`（主操作）、棕 `sd-btn-tan`（次操作/取消）、红 `sd-btn-delete`（危险）。删除零引用死样式 `sd-btn-gold`、`sd-btn-red`，删除 CSS 渐变蓝按钮 `.sd-btn-blue`（诊断"导出诊断包"改 tan+lg、玩家页"刷新/设置权限"改 tan）和 `.sd-btn-xs`（改 `.sd-btn--sm`）。`sd-btn-restart` 文字色统一为浅色 `#fff7cf` 并与 start/stop 一起带 text-shadow。
- 危险确认统一：所有破坏性确认弹窗（删除/清空/彻底删除/覆盖恢复/确认停止）确认键统一用 `sd-btn-delete`；"确认重启"非破坏性用 `sd-btn-green`；总览/服务器页停止确认不再复用生命周期大按钮。弹窗底部统一为"取消(棕，左) + 确认(语义色，右)"顺序；有底部动作的弹窗删除头部"关闭"按钮（存档上传、Mods 上传、Nexus Key、VNC 端口），纯查看弹窗（QR、新建游戏容器）保留"关闭"。
- 操作区共享布局：`stardew-theme.css` 新增 `.sd-actionbar`（flex + wrap + gap 8px，`--end` 变体右对齐）与 `.sd-rowactions`（行内操作组，gap 6px 右对齐），挂在既有容器类旁（`sd-jobs-toolbar-actions` / `sd-mods-header-actions` / `sd-diag-header-actions` / `sd-settings-section-toolbar` / `sd-saves-eyebrow-actions` 等），页面私有类只留皮肤。
- 删除逐页尺寸覆写：诊断页 46px 头部按钮、总览 48px/服务器 52px 生命周期覆写、任务页 38px 工具栏按钮及其 CSS 自绘图标、服务器页发送 50px/执行 48px/标题动作 36px、玩家页 42px!important 邀请条按钮、总览/服务器邀请刷新 22-30px 等全部移除，回归令牌档位。服务器页生命周期从"三条全宽 52px 巨条"改为与总览一致的横向 lg 按钮排。
- 修复总览页控制区结构性挤压：`.sd-ctrl-row` 原为"左区 | 1px 分隔线 | 邀请区"三列网格，但 `.sd-ctrl-div` 元素只在有邀请码时渲染，无邀请码时邀请面板落进 1px 列被挤成竖排单字。改为两列网格 + 隐藏冗余分隔线元素（中缝线由 `.sd-ov-section::before` 绘制），文件尾部 `FE-OVERVIEW-PROTOTYPE-IMAGE2-2` 两个 ≥901px 断点块同步修正。
- 文案字典：重新拉取数据统一"刷新"（原：刷新列表/刷新备份）；提交统一"保存"（原：保存设置/保存计划/保存端口/保存并生效）；"X并启动"收敛为 创建并启动/上传并启动/导入并启动/启动此存档；服务器页"备份已保存进度"→"手动备份"（与存档页同词，细节在 title）；tooltip"重新获取邀请码"→"刷新邀请码"；busy 态省略号统一"…"。诊断"重新检查"保留（触发检查非拉数据）。
- 影响文件：`frontend/src/games/stardew/stardew-theme.css`、`StardewPanel.css`、`SavesSection.tsx`、`pages/` 下 OverviewPage/ServerControlPage/JobsLogsPage/PlayersPage/ModsPage/DiagnosticsPage/InstallPage/SettingsPage。未改任何 handler、API、权限或 disabled 逻辑。
- 验证：`cd frontend; npm.cmd run build` 通过（项目无 lint/test 脚本）；Playwright 真实登录态下 9 页 × 4 视口（1920/1366/1024/390）改前改后各 36 张截图对比，确认同类按钮同尺寸、窄屏操作区正常换行、无溢出/重叠；console 仅有改前即存在的 metrics 接口 500（Docker 服务不可用所致），无新增报错。

# FE-NEXUS-ERROR-TEXT-1 Nexus 错误码前端中文兜底

- `errorCodeMap` 新增 Nexus 相关错误码映射：`nexus_api_key_missing`、`nexus_auth_required`、`nexus_mod_not_found`、`nexus_unauthorized`、`nexus_rate_limited`、`nexus_request_failed`。
- 下载模组页搜索 Nexus、会员安装或其它 Nexus API 失败时，前端优先按错误码展示稳定中文，不再完全依赖后端返回的 `message`。即使后端或历史构建里 message 出现编码异常，用户也会看到正常中文提示。
- 影响文件：`frontend/src/core/helpers.ts`。
- 验证：`cd frontend; npm.cmd run build`。

# FE-MAIN-PAGE-FRAME-SLICES-1 主内容 Frame 切片平铺

- 所有 Stardew 路由共用的 `.sd-main` 主内容背景不再把 `main_page_frame_empty_image2.png` 整图 `100% 100%` 拉伸；已从 `external artifact stardew-page-prototypes-image2-2026-06-30 (03-saves-page-frame-empty-image2.png)` 按原始 image2 空框确定性裁出 4 个角、4 条边和中心羊皮纸 tile。
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

- 玩家管理页按 `external artifact stardew-page-prototypes-image2-2026-06-30 (05-players - 副本.png)` 重做首屏视觉：顶部六张摘要卡、邀请加入码横条、中部 Junimo 服务器终端 + 在线玩家表、底部玩家活动与管理员操作区对齐原型结构。
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

- 服务器页按 `external artifact stardew-page-prototypes-image2-2026-06-30 (02-server-control - 副本.png)` 调整为羊皮纸控制台结构：顶部大标题、左侧当前状态大卡、右侧生命周期按钮卡、中部邀请代码与全服消息、下方控制台命令绿字终端和快捷操作条。
- 未把原型图作为运行时背景或整块素材引用；页面底纹、纸卡、铜色描边、inset 高光、分隔线、绿屏邀请码、黑色终端、快捷操作纸卡均为 CSS gradient / border / box-shadow / pseudo-element 实现。
- `ServerControlPage` 只新增视觉外壳和信息分组：状态卡字段化、命令结果合入右侧终端展示、全服消息增加字数显示、快捷操作改为原型式横向按钮条；启动/停止/重启/刷新邀请码/复制/喊话/执行命令/备份/VNC/计划重启等 handler、API、权限判断和 disabled 状态未改。
- 按钮与图标复用既有素材：生命周期按钮继续使用 `sd-btn-start/stop/restart` 与 `icon_button_*` PNG；页头/分区标题复用 `ui/icons` 下的服务器、玩家、存档、时间、诊断等现有图标；状态点复用 `.sd-dot*`。
- 响应式：服务器页规则以 `.sd-server-page` 为作用域，并补 `@container sd-main-scroll` 断点；主内容变窄时页面自动改为单列，命令区、全服消息和快捷操作按容器宽度收缩，输入框使用局部 `box-sizing: border-box` 避免窗口缩小时内部裁切。
- 影响文件：`frontend/src/games/stardew/pages/ServerControlPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；使用已删除的临时 `frontend/server-control-qa.html` 挂载真实 `ServerControlPage` 组件与 mock 数据做浏览器 QA：1280x900 桌面无横向溢出、按钮无文字溢出、命令执行后终端显示输出；390x760 窄屏无横向溢出、消息/命令/快捷操作单列收缩、按钮无溢出。

# FE-DIAGNOSTICS-PROTOTYPE-IMAGE2-1 诊断与健康页按 image2 原型视觉重皮肤

- 诊断与健康页按 `external artifact stardew-page-prototypes-image2-2026-06-30 (07-diagnostics - 副本.png)` 重做首屏视觉：顶部标题/操作、系统状态横向总览、正常/警告/错误计数、左侧检查项表、右侧资源趋势、底部告警与建议条对齐原型结构。
- 未把原型图作为运行时背景或整块资源引用。页面羊皮纸底、纸纹噪点、面板边框、内描边、虚线分隔、状态点放大和资源仪表盘由 CSS gradient / border / box-shadow / pseudo-elements 实现；盾牌、宝石、检查项和建议区图标已在 `FE-DIAGNOSTICS-IMAGE2-ICONS-1` 中替换为 image2 风格透明 PNG；SVG 趋势图继续使用现有数据绘制。
- `DiagnosticsPage.tsx` 保留既有 `getHealthDiagnostics()`、`downloadSupportBundle()`、`getInstanceMetrics()`、管理员导出权限、loading/error/disabled 状态和 5s metrics 轮询，仅调整 DOM 外壳：新增计数卡、检查表头/图标列、资源标题行和底部全宽建议面板。
- 按钮/素材复用：页头图标复用 `icon_nav_diagnostics_monitor_image2.png`；“重新检查”复用既有绿色 PNG 按钮体系；“导出诊断包”新增诊断页蓝色 CSS 按钮变体，未新增按钮图片。
- 响应式：1180px 以下主内容改单列；760px 以下按钮、计数卡、仪表盘、检查表和建议面板收成移动端单列/紧凑布局，显式避免横向溢出。
- 影响文件：`frontend/src/games/stardew/pages/DiagnosticsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过；使用 Vite 本地服务 + 已删除的临时 `diagnostics-qa.html` 加载真实 CSS/素材/同结构 DOM 做内置浏览器 QA：1280x900 桌面无横向溢出、按钮/检查项文字不溢出、主要面板无重叠、console error/warn 为空，点击“重新检查”进入禁用检查中状态；390x760 窄屏无横向溢出，所有主要面板宽度落在页面内。已用 `view_image` 对比原型和最新桌面/移动截图。

# FE-SETTINGS-PROTOTYPE-IMAGE2-1 设置页按 image2 原型视觉重皮肤

- 设置与审计页按 `external artifact stardew-page-prototypes-image2-2026-06-30 (09-settings - 副本.png)` 重排视觉结构：顶部原为“当前账号 / 面板版本 / 安全与权限”三卡，现已移除重复的“当前账号”卡，仅保留“面板版本 / 安全与权限”两卡；中部为“用户管理 / 审计日志”双栏，底部为“端口信息 + 其他设置 / 安全建议”两栏。业务数据、API 调用、权限判断、弹窗确认、用户创建/角色/禁用/删除、审计分页和 VNC 端口保存逻辑均保持不变。
- 本次没有把原型图作为运行时背景或整块素材引用；页面背景继续使用既有主内容 frame，设置页卡片、纸纹、铜色边框、角钉、内描边、表格表头、行分隔线、底部提示区均由 `.sd-settings-page` 作用域 CSS 使用 gradient、border、box-shadow 和伪元素实现。
- `SettingsPage.tsx` 为各功能区补充页面级 modifier class，并新增 `SecuritySummarySection`，把原来长说明型安全信息保留为底部“安全建议”，同时在顶部提供与原型对应的安全状态摘要。设置页头图标切换为已有 image2 齿轮素材 `icon_nav_settings_gear_image2.png`。
- 按钮与小图标继续复用现有 Stardew 素材体系：按钮为 `sd-btn-green` / `sd-btn-tan` / `sd-btn-delete`，标题图标复用既有导航/顶栏 image2 PNG；没有新增图片素材。窄屏下顶部/中部/底部网格收为单列，用户行按钮换行，审计表在自身容器内横向滚动，页面整体无横向溢出。
- 影响文件：`frontend/src/games/stardew/pages/SettingsPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。
- 验证：`cd frontend; npm.cmd run build` 通过。真实 `/instances/stardew/settings` 当前停在登录页且 console error/warn 为空；使用已删除的临时 `settings-qa.html` 加载同一份 CSS/素材/同结构 DOM 做视觉 QA：1280x900 桌面下三卡 + 双栏布局、按钮无文字溢出、无横向溢出、console error/warn 为空；点击“新建用户”后表单展开且无横向溢出；390x760 窄屏单列无横向溢出，审计表仅在表格内部横向滚动，底部待接入/禁用按钮可读。已用 `view_image` 对比原型图与最终桌面截图。

# FE-INSTALL-PROTOTYPE-IMAGE2-1 安装页按 image2 原型视觉重皮肤

- 安装页按 `external artifact stardew-page-prototypes-image2-2026-06-30 (08-install - 副本.png)` 做页面级视觉改造，未把原型图作为运行时背景或整块素材引用；羊皮纸背景、纸张噪点、面板描边、时间线卡片、绿色进度条、配置/认证/日志三栏、深色终端日志窗均由 CSS 实现。
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

- 总览页按 `external artifact stardew-page-prototypes-image2-2026-06-30 (01-overview - 副本.png)` 调整视觉层级，但未把该原型图作为运行时背景或整块素材引用：页面背景、控制条、摘要卡、三列清单、绿屏邀请码、纸纹噪点、边框、内阴影和分隔线均为 CSS 实现。
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

- 按 `external artifact stardew-page-prototypes-image2-2026-06-30 (03-saves-page-frame-clean-image2-no-buttons-icons-thumbnails.png)` 重做存档页视觉，全部用 CSS 实现、不新增任何图片资源：羊皮纸纹理 = 两层低透明度 radial-gradient 噪点；激活卡四角铆钉 = 4 层 radial-gradient 圆点定位到四角；上传条像素云 = 多层白色矩形 background 层叠在蓝天渐变上；虚线创建卡 = `::before` inset 虚线框。
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
- 将原型图 `external artifact stardew-page-prototypes-image2-2026-06-30 (03-saves-page-frame-empty-image2.png)` 复制为运行时素材 `frontend/public/assets/stardew/ui/panels/main_page_frame_empty_image2.png`，作为所有 Stardew 路由中间主内容区的统一背景。
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

- 按 `docs/prototypes/overview-design-baseline-2026-06-30.png` / `Top bar.png` 风格重新用 image2 生成顶栏拆分素材，替换上一批观感不合格的 topbar 资源；没有从原图按脚本坐标裁切，脚本仅用于生成图的 chroma-key 去底、尺寸归一化、预览和 alpha 校验。
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

- Stardew 路由页按 `external artifact stardew-page-prototypes-image2-2026-06-30` 的页面布局重新排布信息层级，但不复刻原型静态内容：现有 API 数据和操作能力保留，按原型中相同功能的位置组织展示。
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

`App.tsx` 在进入 `stardew` 视图时会按 `frontend/src/hooks/useMediaQuery.ts` 的 `(max-width: 768px)` 判断分流：非移动端渲染现有桌面端 `StardewPanel`（9 路由 + 顶栏/侧栏/OpsRail 全套 Shell），移动端渲染 `frontend/src/games/stardew/StardewMobileShell.tsx` 占位壳，详见下方“移动端入口”。

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

上述桌面端 9 个路由页面（`StardewPanel.tsx`）和移动端 5 个页面（`StardewMobileShell.tsx`）均已改为 `React.lazy` 按需加载，`renderPage()` / Tab 内容外层套了 `Suspense`，fallback 复用已有占位卡片样式（桌面 `sd-placeholder-card`，移动 `sd-mshell-card`）。只有当前激活的 Tab 才会拉取对应页面代码，切换 Tab 才会触发新 chunk 请求。新增页面时按同样写法用 `lazy(() => import('./pages/XxxPage').then((m) => ({ default: m.XxxPage })))` 接入，不要退回静态 import，否则首屏 JS 体积会重新膨胀。

## 移动端入口（M0）

- `frontend/src/hooks/useMediaQuery.ts`：通用 `useMediaQuery(query: string): boolean` hook，基于 `window.matchMedia` + `change` 事件，不绑定具体断点，可在其它场景复用。
- `App.tsx` 用 `useMediaQuery('(max-width: 768px)')` 判断是否移动端，只在 `view === 'stardew'` 分支处二选一渲染：桌面渲染既有 `StardewPanel`（行为视觉不变），移动端渲染 `StardewMobileShell`。判断在浏览器 resize 跨越 768px 边界时会重新分流。
- `StardewMobileShell.tsx` 是 M0 占位壳，不做真实路由/分页：内部直接调用 `useStardewDashboardData()` 读取 `instanceState.state` 展示“运行中/已停止/初始化中”状态文案，中间是一张复用 `.sd-panel` 羊皮纸样式的占位卡片，底部 5 个 Tab（总览/控制/玩家/任务/更多）只做本地 `useState` 高亮切换，不触发导航或数据请求。
- 样式独立在 `StardewMobileShell.css`，class 前缀 `sd-mshell-`，与桌面 `StardewPanel.css`（`sd-shell`/`sd-topbar`/`sd-sidebar`/`sd-opsrail` 等）完全不共享作用域；只复用 `stardew-theme.css` 里已有的 `--sd-green*`/`--sd-brown*`/`.sd-bg-wood-strip`/`.sd-panel`/`.sd-dot-*` 变量和工具类，未新增图片素材、未引入 UI 库。
- M0 之前 `StardewPanel.css` 里 640px/720px/960px 等断点是桌面 Shell 自身的“挤压单栏”响应式，用于窄浏览器窗口场景；现在 `<=768px` 会先被 `App.tsx` 分流到 `StardewMobileShell`，桌面 Shell 内部这些断点只在 769px~某宽度之间的窄桌面窗口才会触发，行为未删除但触发范围变窄。
- M0 不新增真实移动端页面、不改后端 API、不改登录/权限逻辑；`StardewMobileShell` 目前不接收 `user`/`onLogout`，没有登出入口，属于已知限制，留给后续里程碑。
- QA：`frontend/qa-layout.html?shell=mobile`（可叠加 `&state=running/stopped`）会用既有 mock fetch 渲染 `StardewMobileShell`，`?shell=desktop`（默认）或不带参数渲染原 `StardewPanel`，用于后续迭代的移动端布局回归。

## 移动端总览页（M2）

- `StardewMobileShell` 现在接收 `user: CurrentUser` prop（`App.tsx`/`qa-layout-main.tsx` 同步传入），补上 M0 遗留的“暂无 user”限制；“总览”Tab 激活时渲染 `frontend/src/games/stardew/mobile/MobileHomePage.tsx`，其余四个 Tab（控制/玩家/任务/更多）仍是 M0 占位卡。
- `MobileHomePage` 按单列卡片流展示四张卡片，全部只读/写现有 `useStardewDashboardData()` 数据层和 `api.ts` 现有函数，未新增后端接口：
  1. 状态摘要卡：存档名（`saves.activeSaveName`）、服务器状态（区分“运行中/已停止/启动中/停止中/异常”，比 `stateLabel()` 多识别 `stopping`）、在线玩家（`players.onlineCount/maxPlayers`）、版本（`versionInfo.version`），字段缺失时都有中文兜底文案，不渲染 `undefined`/`null`。
  2. 邀请信息卡：不是直接复用 `InviteCodeCard.tsx` 组件（那套依赖仅在挂载 `StardewPanel` 时才加载的 `StardewPanel.css`，移动端不会加载会导致样式丢失），而是按同一套数据状态判断（`dashboardData.inviteCode`/`steamAuthLoggedIn`/`publicIP`/`publicIPError`/`publicIPRefreshing`）重写了一个轻量展示 + 复制按钮，长文本用 `word-break:break-all` 等宽小字防止撑破卡片，没有复制 `InviteCodeCard` 的任何 API 请求逻辑。
  3. 快捷控制卡：启动/停止/重启复用 `startInstance`/`stopInstance`/`restartInstance` 和 `OverviewPage.tsx` 同款的 `hasActiveLifecycleJob`/`activeLifecycleIsStopping`/`waitingForStartup`/`waitingForStop` 状态判断，按钮固定渲染三个（不像桌面端按状态切换显示哪个），改用 `disabled`+`title` 表达“当前不可操作”；停止/重启走确认弹窗；三个按钮和弹窗操作按钮都通过 `min-height:44px` 覆盖满足触控热区，未修改 `stardew-theme.css` 里 `.sd-btn-start/-stop/-restart` 本身的默认高度。
  4. 待认证玩家批准卡：复用 `PlayersPage.tsx` 同款的“页面自己按需拉取 `getInstancePasswordStatus()`，不进全局轮询”模式和 `approvePlayerAuth()`，展示 `isAuthenticated===false` 的在线玩家并提供批准确认弹窗。
- 视觉上不新增图片素材、不引入 UI 库：卡片壳用全局 `stardew-theme.css` 的 `.sd-panel`，按钮/提示条/徽章用全局 `.sd-btn-*`/`.sd-notice--*`/`.sd-tag*`（这些类在 `main.tsx` 里全局加载，和只在挂载 `StardewPanel` 时才生效的 `StardewPanel.css` 不同，移动端页面可以直接用），新增样式集中在 `frontend/src/games/stardew/mobile/MobileHomePage.css`（class 前缀 `sd-mhome-`）。
- QA mock：`qa-layout-main.tsx` 补了 `password-status` 路由 mock 和一名 `isAuthenticated:false` 的在线玩家，方便在 `qa-layout.html?shell=mobile` 下看到待认证卡片的真实列表态。
- 详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `MOBILE-HOME-M2-1` 小节（含 M3 注意事项）。

## 移动端控制页（M3）

- `StardewMobileShell` “控制”Tab 激活时渲染 `frontend/src/games/stardew/mobile/MobileControlPage.tsx`，其余两个 Tab（玩家/更多）仍是占位卡。
- 范围按用户口径限定为桌面 `ServerControlPage.tsx` 的“全服消息”+“快捷操作”两块能力，**去掉手动备份和 VNC 显示相关按钮**（打开/关闭 VNC 显示、跳转 VNC 控制），不含生命周期启停（启停重启已在“总览”Tab 的快捷控制卡提供，两个 Tab 都读同一份 `dashboardData`，不重复维护）：
  1. 顶部状态条：`state` + `saves.activeSaveName` 的一行紧凑摘要（不是完整卡片），复用和 `MobileHomePage` 相同写法的 `serverStatusText`/`serverStatusDotClass` 私有函数（按仓库“各页面自带小工具函数”的既有风格各自实现一份，未抽公共 helper）。
  2. 全服消息卡：输入框 + 发送按钮，逻辑与桌面 `handleSay`/`sendSay` 完全一致；未运行时展示提示文案，不渲染输入框。发送按钮用 `sd-btn-restart`（棕色，和重启按钮同色）而不是 `sd-btn-green`，和 PC 端按钮颜色故意区分；PC/移动端都已去掉“该命令当前版本可能返回‘命令不支持’”这句过时提示（SMAPI say 命令现已正常支持）。
  3. 快捷操作卡：单列纵向按钮列表（非桌面的多列网格），5 个按钮 —— 计划重启（`getRestartSchedule`/`updateRestartSchedule`）、服务器密码设置（`getInstanceServerPassword`/`updateInstanceServerPassword`/`getInstancePasswordStatus`）、小屋与联机高级设置（`getInstanceServerRuntimeSettings`/`updateInstanceServerRuntimeSettings`）、触发节日活动（`triggerFestivalEvent`）、永久启用 Joja 路线（`enableJojaRoute`，逐字输入 `IRREVERSIBLY_ENABLE_JOJA_RUN` 才能点亮确认按钮）。四个表单类按钮打开全屏弹窗，`disabled`/`title` 门控逻辑（`isAdmin`/`isRunning`）与桌面版逐条对齐。前 4 个按钮（计划重启/密码设置/小屋高级设置/触发节日活动）视觉上不是像素按钮贴图，而是照抄 PC 端 `.sd-server-quick .sd-server-quick-grid > button` 那条纯 CSS 羊皮纸卡片（边框+渐变+内阴影，无 `background-image`），移动端修饰类叫 `.sd-mctrl-action-btn--card`；“永久启用 Joja 路线”按用户明确要求保留独立的红色像素按钮贴图（`sd-btn-delete`），不跟着变成金棕色卡片，这是移动端刻意做的危险操作差异化，PC 端实际上 Joja 视觉和其它按钮一致。
- 未直接复用 `ServerControlPage.tsx` 或它的 CSS 类（`sd-confirm-overlay`/`sd-schedule-*`/`sd-server-quick-grid` 等只在 `StardewPanel.css` 里定义，移动端不加载该文件）；只复用了桌面页面里的**状态判断逻辑和 `api.ts` 现有函数**，弹窗/按钮/提示条重新用移动端自己的 CSS 类排布，视觉基础件（`.sd-panel`/`.sd-input`/`.sd-btn-*`/`.sd-notice--*`）继续走全局 `stardew-theme.css`。新增样式集中在 `frontend/src/games/stardew/mobile/MobileControlPage.css`（class 前缀 `sd-mctrl-`），弹窗统一用 `.sd-mctrl-dialog-overlay`/`.sd-mctrl-dialog`（`max-height:88vh` + `overflow-y:auto` 防止计划重启这类长表单在小屏下溢出视口），表单控件通过 `.sd-mctrl-field .sd-input{min-height:44px}` 覆盖全局 `.sd-input` 默认 26px 高度，满足触控热区要求；未新增图片素材，按钮图标复用桌面 `SERVER_PAGE_ICONS` 里已有的几张 PNG。
- **CSS 覆盖踩坑（写进这里避免下次重犯）**：这个页面里所有“用移动端自己的类覆盖全局 `.sd-btn-*`/`.sd-input` 某个属性”的写法，一律不能只用单类选择器（如 `.sd-mctrl-action-btn--card { background:... }`）。Vite 打包后组件级 CSS 在最终产物里的实际顺序**不一定**排在全局 `stardew-theme.css` 之后（实测发现是反过来的，`.sd-btn-tan` 的 `background-image` 规则排在这个组件文件之后），单类选择器和全局基类优先级相同时，源码顺序更靠后的全局规则会赢，覆盖悄悄失效但不报错，很难肉眼发现。正确做法是把要覆盖的类和元素本身已有的另一个类叠加成复合选择器（如 `.sd-mctrl-action-btn.sd-mctrl-action-btn--card`），让优先级从 (0,1,0) 提到 (0,2,0)，不依赖打包顺序也能稳定生效。验证方法：`npm run build` 后直接读 `dist/assets/index-*.css`，用 `indexOf` 比较两条规则的字节偏移，不要只凭感觉判断“组件 CSS 后 import 所以后生效”。`min-height` 覆盖 `height` 不受此影响（两个不同属性，浏览器盒模型固定取较大值），只有覆盖“同一个属性”时才需要注意。`MobileHomePage.css` 的 `.sd-mhome-copy-btn` 大概率有同样的 `padding` 覆盖风险，这次没有动，留给下一位维护者按同样方法验证修复。
- 未改后端 API、未改鉴权逻辑、未改桌面端 `ServerControlPage.tsx`/`StardewPanel.css`。
- QA mock：`qa-layout-main.tsx` 补了 `/config/server-password`、`/config/server-runtime-settings` 两个 GET 路由 mock（此前只有 `restart-schedule`/`password-status`/`vnc-port`/`rendering`），避免弹窗打开时读到空对象导致受控输入框变成非受控。
- 详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `MOBILE-CONTROL-M3-1` 小节。

## 移动端玩家页（M4）

- `StardewMobileShell` “玩家”Tab 激活时渲染 `frontend/src/games/stardew/mobile/MobilePlayersPage.tsx`，仅剩“更多”Tab 是占位卡。
- 页面结构只有单张“在线玩家”卡：卡片头部左侧标题、右上角一个“刷新”按钮（`dashboardData.refreshPlayers()`），下方是玩家卡片列表——`playerRows` 全量，不是只筛 `status==='online'`，因为字段要求同时展示在线/离线/等待/未知状态。首版曾做过顶部统计卡（在线人数/待授权数量）和独立的“待授权玩家”卡（同意/拒绝待认证玩家），用户反馈后整体删除，改成当前的单卡结构，不做批准/拒绝密码认证相关功能。空列表时展示“暂无在线玩家”，不留白。
- 每张玩家卡片自上而下：①姓名 + 状态徽章（在线绿色/等待黄色/离线或未知默认灰底）；②次要信息行（`isHost` 显示"主机"、`player.role` 存在时显示角色徽章、活动文案 `playerActivityText()`：在线显示 `onlineFor` 或“在线中”，离线显示 `最近活动：${formatDate(lastSeen)}`，都没有显示“—”）；③底部一行 `justify-content:space-between`——左侧位置信息 `playerLocationText()`（取 `locationDisplayName`/`locationName`/`location` 中第一个非空值，有 `tileX`/`tileY` 时附加坐标，都没有值时显示“—”，不翻译成中文地名，避免把桌面页 200 多行的 `LOCATION_ZH` 字典搬进这个文件）、右侧“踢出”“封禁”两个操作按钮。
- 踢出/封禁复用桌面 `kickPlayer()`/`banPlayer()`，未新增接口；`disabled`/`title` 门控条件与桌面 `PlayersPage.tsx` 行内图标按钮逐条对齐（踢出要求 `status==='online'`，封禁不要求在线但都排除主机 `isHost`）；忙碌态保存目标 `uniqueMultiplayerId`。封禁弹窗已按真实验证结论明确提示“服务器容器重启后会丢失，需要重新操作”。
- 未新增图片素材（页头图标复用现有 `icon_nav_players_avatar_image2.png`）；样式集中在 `frontend/src/games/stardew/mobile/MobilePlayersPage.css`（class 前缀 `sd-mplay-`），只用全局 `stardew-theme.css` 的 `.sd-panel`/`.sd-tag*`/`.sd-notice--*`/`.sd-btn-*`，未复用 `StardewPanel.css` 里的桌面玩家表格类名（那批类只在挂载 `StardewPanel` 时才加载）。
- 待认证玩家的同意/拒绝仍保留在“总览”Tab 的待认证玩家批准卡（`MobileHomePage.tsx`，见 `MOBILE-HOME-M2-1`），“玩家”Tab 这次不再重复这块功能。
- 详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `MOBILE-PLAYERS-M4-1` 小节。

## 移动端存档页（M5）

- 底部导航第 4 个 Tab 从"任务"改名为"存档"：`StardewMobileShell.tsx` 里本地私有类型 `MobileTabKey` 的枚举值从 `'jobs'` 改为 `'saves'`（这个类型只在移动端 Shell 内部使用，和桌面 `stardew-routes.ts` 里的 `StardewRoute`/`'jobs'`（任务日志路由）是两个完全独立的命名空间，不会互相影响，改名不涉及桌面端任何路由）。"更多"Tab 仍是占位卡。
- 新增 `frontend/src/games/stardew/mobile/MobileSavesPage.tsx` + `MobileSavesPage.css`（class 前缀 `sd-msave-`），展示当前服务器存档信息 + 导出/导入操作，不新增后端接口，直接消费 `dashboardData.saves`（`SavesListResult`）/`dashboardData.savesError`/`dashboardData.refreshSaves()`：
  - 取值逻辑：`activeSave` 优先取 `saves.find(s => s.isActive || s.name === activeSaveName)`；如果没有显式激活的存档但列表非空，回退展示第一个存档（状态标为"可用"而不是"当前使用中"）；列表为空时展示完整空状态卡（不留白）。
  - 页面顶部一行标题"存档" + 右侧"刷新"按钮（复用 `dashboardData.refreshSaves()`，本地 `refreshBusy` 控制按钮忙碌态文案，页面本身没有专门的 `savesLoading` 字段，借用 `dashboardData.loading && saves===null` 判断首次加载）。
  - 第一块地图原画卡：`aspect-ratio:16/9` 固定容器，`object-fit:contain`（存档地图小图原生尺寸约 88×80 像素画，用 `contain` 完整展示不裁切，`cover` 会裁掉边缘；容器背景铺 `background_parchment_tile.png` 填充留白区域）+ `image-rendering:pixelated` 保持像素锐利；容器右上角叠加状态徽章（`sd-tag-green` 当前使用中 / `sd-tag-gold` 可用 / 默认 未找到）。
  - 地图匹配入口 `saveFarmMapSrc(save)`：按 `farmType` 查 `farmTypeLabel`/`farmTypeAlias` 两个映射表（和桌面 `SavesSection.tsx` 里的同名表逐字一致，按仓库"各页面自带小工具函数"的既有风格独立维护一份，不共享模块），命中则用 `/assets/stardew/new-game/farms/{type}.png`（桌面新建存档页已有的 6 张农场原画素材，未新增图片）；未命中或图片加载失败（`<img onError>`）都回退到 `DEFAULT_MAP_SRC = /assets/stardew/ui/backgrounds/background_login_farm_generated.png`（仓库已有的纯像素农场背景素材，`LOGIN-MOBILE-FIX-1` 已经用它做过登录页背景，不从外部拉图）。当前后端 `SaveInfo` 已经带 `farmType` 字段，这次不是"接口没有字段所以先占位"，而是"接口有字段但值可能不在已知映射表里"，两种情况都会走同一个 fallback 路径。
  - 第二块"核心信息"卡：存档名称、农场名称、农场主（`farmerName`）、游戏日期（复用和 `MobileHomePage.tsx` 逐字一致的 `SEASON_ZH`/日期拼接私有函数），两列网格布局，字段缺失都有"—"兜底。
  - 第三块"更多信息"卡：地图类型（复用地图原画卡同一个 `farmTypeLabel` 文案）、存档大小（`formatBytes`）、最后保存时间（`formatDate(modifiedAt)`）、存档状态文字版；`parseError` 存在时额外展示一条错误提示（复用全局 `.sd-notice--error`）。
  - 第四块"存档操作"卡（不受空状态影响，即使暂无存档也展示，用于承载"导入存档"入口）：**导出存档**——直接复用桌面 `SavesSection.tsx` 的 `handleExport()` 逻辑（`exportSave(name)` 拿到 blob + 文件名后用临时 `<a download>` 触发浏览器下载），未新增 API，`disabled={exportBusy || !displaySave}`，不要求管理员权限（和桌面按钮门控一致）。**导入存档**——同样照抄桌面 `handleUploadPreview`/`handleUploadCommit`/`handleUploadCancel` 三段逻辑（`uploadSavePreview`→预览→`uploadSaveCommitAndStart` 导入并启动，取消时对已生成的 token 调用 `uploadSaveCommitAndStart(token, true)` 尽力清理），弹窗 UI 重新按移动端布局排（`.sd-msave-dialog-*`），`disabled={!isAdmin || isRunning}`（`isRunning` 判定和桌面 `SavesSection.tsx` 一致，包含 `running`/`starting` 两种状态）；导入成功后调用 `dashboardData.requestInviteCodeRefresh()`/`refreshInstanceState()`/`refreshJobs()`/`refreshSaves()` 刷新，而不是桌面版依赖的 `onJobStarted`/`onSavesChanged` 回调（移动端页面 props 里没有这两个）。**回档**——纯占位禁用按钮 + 提示文案，说明该功能依赖桌面端备份列表操作，暂不支持手机浏览器，引导用户去桌面端"存档管理"页操作，没有做任何 API 接线。
  - 视觉基础件全部走全局 `stardew-theme.css`（`.sd-panel`/`.sd-tag*`/`.sd-notice--*`/`.sd-btn-tan`/`.sd-btn-green`），未复用 `StardewPanel.css` 里桌面存档卡（`.sd-save-card*`）或上传弹窗（`.sd-saves-modal-*`）的任何类名（那批类只在挂载 `StardewPanel` 时才加载）。
- 390×844/393×852/430×932 下无横向滚动（地图卡固定 `aspect-ratio` + `object-fit`，信息网格用 `overflow-wrap:anywhere` 防长文本撑破），内容纵向可滚动（沿用 `StardewMobileShell` 现有的文档级滚动，未新增额外滚动容器）。
- 未改后端接口、`SaveInfo`/`SavesListResult` 类型、`SavesSection.tsx`/`SavesPage.tsx`（桌面存档管理页不受影响，创建/上传/删除/备份等能力仍只在桌面端）、`useStardewDashboardData.ts` 内部实现。
- 详见 `docs/frontend-handoff/` 最新一篇的 `MOBILE-SAVES-M5-1` 小节。

## 手机端卡片与底部 Tab 视觉统一（M6）

- `StardewMobileShell.css` 新增 `:root` 级 CSS 变量 `--stardew-mobile-card-bg/border/radius/shadow`，值取自 PC 总览页最终生效的"存档/模组"卡片背景（`linear-gradient(180deg, rgba(255,245,214,0.96), rgba(248,226,174,0.94)), #f7e3ad`）和"在线玩家"卡片的边框/圆角/阴影（`9px`/`2px solid #a06c2c`/三层 inset+drop shadow）。
- `.sd-mshell .sd-panel`（优先级 (0,2,0)）一条规则覆盖手机端所有使用 `.sd-panel` 的卡片/弹窗，不需要各页面逐个改 className，桌面端不受影响。
- 玩家页内每行玩家卡片（`.sd-mplay-player-card`）也同步引用该组变量，从直角改为圆角。
- 底部 Tab 栏从贴底满宽硬条改为悬浮圆角胶囊式导航条：`border-radius:20px`、`bottom:10px+safe-area`、5 个 Tab 各带图标（复用桌面导航 image2 icon PNG）+ 文字、`min-height:48px` 触控热区、active 态绿色 pill、`:active` 缩放反馈、文案 `ellipsis` 防溢出。
- 详见 `docs/frontend-handoff/` 最新一篇的 `MOBILE-VISUAL-UNIFY-M6-1` 小节。

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
- `FE-MOBILE-FIXES-1`：新一轮系统性手机端适配，不改动现有断点数值，只修复具体问题：表单控件移动端字号提到 16px（防 iOS 自动缩放）、`viewport-fit=cover` + `env(safe-area-inset-*)` 安全区、移动端导航图标触控热区提到 44×44px、确认弹窗补 `max-height/overflow-y` 防溢出、Players/存档备份宽表格补横滑渐变提示。详见 `docs/frontend-handoff/frontend-handoff-2026-07-09.md`。
- `MOBILE-SHELL-M0-1`：新增移动端基础入口。`App.tsx` 用新增的 `useMediaQuery('(max-width: 768px)')` 在 Stardew 面板入口处分流，`<=768px` 渲染新占位组件 `StardewMobileShell`（顶部品牌/状态、羊皮纸占位卡、5 个静态 Tab），桌面端行为视觉不变。详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md`。
- `LOGIN-MOBILE-FIX-1`：修复登录/初始化页（`App.tsx` 里 `sd-auth-shell--image-login`，和上面的 `MOBILE-SHELL-M0-1`/`StardewMobileShell` 是完全独立的两套代码，作用域不重叠）在手机端的布局崩坏。根因是桌面版把整张原型图当卡片背景、用固定 16:9 比例反算出绝对定位的大盒子，再用百分比坐标摆放输入框——手机竖屏宽高比不同，算出来的盒子宽度会远超视口，被 `overflow:hidden` 裁掉，叠加旧的 `@media(max-width:700px)` 手工坐标补丁在不同机型宽高比下持续错位。`<=768px` 时整体放弃这套坐标定位，改回真实文档流的羊皮纸卡片，卡片装饰复用现有 `background_parchment_tile.png`/`button_primary_small_green_blank.png`；shell 背景改用 `background_login_farm_generated.png`（非 image2 版登录页用的纯像素农场背景，没有假 UI 元素），而不是继续用画死了一整套假窗口 UI 的 `background_login_home_image2.png`（第一版试过直接铺这张图，用户反馈"背景还是 PC 端的登陆窗口，很违和"）。三张都是仓库已有素材，未新增图片，只改了 `frontend/src/App.css` 一个文件。详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `LOGIN-MOBILE-FIX-1` 小节。
- `MOBILE-HOME-M2-1`：移动端“总览”Tab 从 M0 占位卡换成真实页面，见上方“移动端总览页（M2）”小节；桌面端 `OverviewPage`/`StardewPanel.css` 未改动，未新增后端 API。
- `MOBILE-CONTROL-M3-1`：移动端”控制”Tab 从占位卡换成真实页面，见上方”移动端控制页（M3）”小节；桌面端 `ServerControlPage.tsx`/`StardewPanel.css` 未改动，未新增后端 API，未改鉴权逻辑。同批顺手把”全服消息”卡片里过时的”该命令当前版本可能返回’命令不支持’”提示文案删掉（PC 和移动端都删，SMAPI say 命令现已正常支持）；移动端”发送”按钮改用 `sd-btn-restart`（棕色）而不是 `sd-btn-green`，和桌面端保持颜色差异，按钮尺寸仍由 `.sd-mctrl-say-btn` 覆盖为 `min-height:44px`。
- `MOBILE-VISUAL-UNIFY-M6-1`：手机端卡片与底部 Tab 视觉统一优化，见上方”手机端卡片与底部 Tab 视觉统一（M6）”小节。所有手机端 `.sd-panel` 卡片获得圆角/渐变背景/阴影（取自 PC 总览页”存档/模组”卡的背景色+”在线玩家”卡的圆角/边框/阴影），底部 Tab 栏重做为悬浮胶囊导航条（图标+文字、圆角 pill、active 绿色高亮、按压缩放反馈、safe-area 适配）。只改 CSS 和 Tab 按钮结构，未改业务逻辑。

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

- 新增 `frontend/public/assets/stardew/ui/panels/panel_side_rail_shell_empty_image2.png`，基于 `external artifact stardew-page-prototypes-image2-2026-06-30 (Left panel.png)` 生成左侧栏木质背景空壳素材。
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

- 新增 `frontend/public/assets/stardew/ui/panels/panel_right_rail_shell_empty_image2.png`，基于 `external artifact stardew-page-prototypes-image2-2026-06-30 (01-overview-right-sidebar-empty-image2.png)` 的右侧栏风格重绘。
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

- 任务与日志页按 `external artifact stardew-page-prototypes-image2-2026-06-30 (04-jobs-logs - 副本.png)` 调整为羊皮纸双栏任务台：顶部大标题 + 虚线分隔、像素按钮工具条、左侧任务列表、右侧任务详情/进度/SSE 状态/深色日志终端/VNC 修复提示。
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
# FE-CARD-UNIFY-SAVES-1 非模组页小框统一为干净存档卡片样式
- 除模组管理页外，Stardew 其他页面的小框统一为存档管理页卡片基准：暖色纸面背景、铜色 2px 边框、9px 圆角、内描边和轻微底部阴影。覆盖范围包括总览、服务器控制、任务日志、玩家管理、诊断、安装、设置以及存档页自身的常用小框/面板。
- 按用户反馈去掉密集点状纸纹：`--sd-save-card-bg` / `--sd-save-card-bg-strong` 改为干净的浅色线性高光 + 纯色纸面，不再使用铺满的 `radial-gradient` 噪点；存档页卡片也覆盖为同一套干净变量，保持全局基准一致。
- 文字和布局同步收敛：小框标题统一约 14.5px、说明/元信息约 12.5px，窄屏容器查询下标题约 13.5px；卡片 padding、gap、行内列表背景、统计小格、安装步骤块、设置/玩家/诊断列表等统一为更紧凑的面板节奏，避免缩放后像不同面板拼在一起。
- 模组页保持原状：本次新增规则不包含 `.sd-mods-*` 主体卡片；QA 中确认模组卡仍为原 1px 边框、无新渐变背景。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。未改 TSX、API、权限、轮询或后端逻辑。
- 验证：`cd frontend; npm.cmd run build` 通过。真实本地应用当前停在登录页；使用已删除的临时 `frontend/public/__codex-card-qa.html` 加载同一份 Vite CSS 做内置浏览器 QA：1280x720 下非模组小框与存档卡片背景/边框/圆角/阴影一致且 `hasDotTextureOnUnified=false`，模组卡 `modsTouched=false`；390x760 下无页面级横向溢出，标题无裁切。
# FE-CARD-UNIFY-SAVES-1 follow-up：总览统计卡清除点状纹理
- 按用户最新反馈，仅清除总览页四个统计卡（`.sd-mc`：存档/模组/系统健康/运行任务）背景里的点状 `radial-gradient` 纹理；保留原有卡片结构、尺寸、边框、圆角、阴影、文字布局和状态徽章。
- 影响文件：`frontend/src/games/stardew/StardewPanel.css`。未改 TSX、API、路由、权限或布局结构。
- 验证：`cd frontend; npm.cmd run build` 通过；确认 `.sd-mc` 两处背景定义不再包含点状 `radial-gradient`。

# FE-SERVER-PLAYERS-CARD-LAYOUT-1 服务器摘要卡迁移与玩家表字段优化
- 新增 `frontend/src/games/stardew/ServerSummaryCard.tsx`，把“服务器状态 / 在线人数 / 当前农场 / 邀请加入码”摘要卡抽为共享组件。
- 服务器控制页删除原有大状态卡和独立邀请码卡，把共享摘要卡放到原状态卡位置；生命周期控制、喊话、命令、备份、计划重启等业务逻辑不变。
- 玩家管理页移除该摘要卡，页面首块直接进入在线玩家表；“服务器信息（Junimo）”整段移到页面底部，作为低频调试信息保留。
- 在线玩家表删除“角色”列；主机标识改为贴在玩家名右侧；新增可见“农场收入”和“玩家收入”列，原先只在行 `title` 里的收入信息改为表格正文展示，并重新调整表格列宽和窄屏横向滚动最小宽度。
- 影响文件：`frontend/src/games/stardew/ServerSummaryCard.tsx`、`frontend/src/games/stardew/pages/ServerControlPage.tsx`、`frontend/src/games/stardew/pages/PlayersPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未改 API、数据模型、权限判断或后端逻辑。
- 验证：`cd frontend; npm.cmd run build` 通过。内置浏览器 DOM 快照接口本次返回兼容错误，未完成截图式 QA；临时 QA 页面文件已删除。

# FE-PLAYERS-PROTOTYPE-CURRENT-1 玩家页按 version-02 原型继续精确对齐

- 玩家管理页按 `C:/Users/anxi/.codex/generated_images/019f2c2c-8909-7262-bd81-d31356799c21/_sorted_overview_to_settings/version-02-current-frontend-code/05-players.png` 继续校准布局：第一块为整行在线玩家表，第二行为左侧“玩家活动 / 最近事件”和右侧“管理操作”，底部为整行“服务器信息（Junimo）”终端。
- 通过 `.sd-main:has(.sd-players-page)` 仅对玩家页收紧主 frame inset，让 1536x1024 QA 下主内容约为 `x=232/w=995/y=90`，不影响其它页面。
- 在线玩家标题改为原型式状态徽章：`在线: N` 与非 online 名册行派生出的 `等待加入: N`；精确接入状态不再额外显示“已接入”徽章。管理操作区隐藏原型中不存在的底部说明，仅保留 2x2 操作卡和待接入状态。
- 表格列宽、行高和最小宽度收紧，桌面 QA 下表格不再出现横向滚动条；窄屏仍只让表格容器内部横向滚动，不撑出页面。收入列兼容 QA mock 的 `farmMoney` / `personalMoney` 作为前端回退，后端字段契约不变。
- 影响文件：`frontend/src/games/stardew/pages/PlayersPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`；为恢复当前构建，未跟踪 QA 入口 `frontend/src/server-qa-main.tsx` 的 mock user 补齐了 `isSuperAdmin`。
- 验证：`cd frontend; npm.cmd run build` 通过；内置浏览器 QA 覆盖 `1536x1024` 和 `390x844`，console error/warn 为空，无页面级横向溢出，桌面表格 `clientWidth=955` / `scrollWidth=955`，Junimo 终端首屏可见。

# FE-MISSING-GAME-INSTALL-PROMPT-1 登录后的缺游戏文件安装引导弹窗

- 每次进入 Stardew 面板时，`StardewPanel.tsx` 都会在仪表盘状态加载完成后检查实例状态；首次管理员注册后自动进入、普通账号密码登录、已有 session 刷新进入面板都会触发同一套判断。
- `game_installed/save_required/ready_to_start/starting/running/stopped` 视为已检测到游戏；若存在 `stardew_install` 的 `queued/running` 任务，也视为安装流程已在进行中，不弹窗。当前已在安装页时也不额外弹窗，避免遮挡安装表单。
- 若实例仍处于 `admin_created/uninitialized/junimo_scaffolded/credentials_required/steam_auth_failed/error` 等未检测到游戏文件的状态，显示“请先安装游戏”弹窗；主按钮“去安装游戏”复用现有 `navigate('install')` 跳转到安装页，次按钮“稍后”只关闭本次登录/面板挂载内的提示。
- 影响文件：`frontend/src/App.tsx`、`frontend/src/games/stardew/StardewPanel.tsx`。未新增后端 API，未改变安装、Steam 认证、权限、轮询或 Junimo 通信逻辑。
- 验证：`cd frontend; npm.cmd run build` 通过；临时 mock QA 覆盖首次注册后缺游戏文件弹窗与“去安装游戏”跳转。

# FE-STEAM-QR-LOG-FALLBACK-1 安装页 QR 认证日志兜底

- 安装页新增 `effectivePhase`：默认使用后端 `instance.driverPhase`，但当后端阶段为 `steam_guard_mobile_required` 且最近安装日志明确显示 QR 选择（例如 `[steam] Choice [1]: 2` 或“已选择扫码登录”）时，前端会按 `steam_qr_required` 渲染。
- 该兜底只在最近日志里没有后续 Steam Guard 菜单时生效；如果日志已经进入 `Steam Guard Authentication`、`Approve in Steam` 或 `Enter code` 菜单，仍按 Guard 流程展示。
- 影响范围：安装页顶部“当前阶段”、安装进度文案、右侧 Steam 认证交互区都会使用 `effectivePhase`，因此旧任务或旧后端临时写错阶段时，也不会把 QR 流程误显示成“Steam Guard 验证 / 手机 App 批准”。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`。接口不变，仍消费现有 `GET /jobs/:id/logs`、SSE job log 和 `GET /instances/stardew/state`。
- 验证：`cd frontend; npm.cmd run build` 通过。

# FE-STEAM-QR-SINGLE-CODE-1 安装页 QR 弹窗只显示最新完整二维码

- 修复 QR 弹窗把最近 80 条 `[steam]` 日志整段塞进 `<pre>` 的问题；多次刷新后的二维码、`QR code refreshed`、连接失败日志会混在一起，导致扫码器看到碎片合集而无法扫描。
- `extractQrPayload()` 现在从最新的 `Or open: https://s.team/q/...` 行向上提取连续二维码字符块，只返回最新一张完整 QR 本体；打开链接单独显示在二维码下方，不再混入二维码矩阵。
- QR `<pre>` 只包含二维码图形并居中显示；`.sd-install-qr-link` 单独承载备用链接，支持长链接换行。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。接口、日志来源、SSE 和安装流程不变。
- 验证：`cd frontend; npm.cmd run build` 通过。

# FE-STEAM-QR-IMAGE-CODE-1 安装页 QR 弹窗改为本地生成二维码图片

- 修复 Steam QR 字符画在前端字体、行高和窗口尺寸变化后仍可能缺块/断行的问题。弹窗现在不再把字符画作为主扫码对象，而是从最新 `Or open: https://s.team/q/...` 日志中提取登录 URL，并用前端本地 `qrcode` 包生成标准二维码图片。
- `extractQrPayload()` 只要拿到最新 Steam QR URL 就会返回 payload；字符画仅作为图片生成失败时的备用显示，不再决定“打开扫码窗口”按钮是否可用。
- 新增 `frontend/src/types/qrcode.d.ts` 作为最小本地类型声明，避免引入 `@types/qrcode` 后污染浏览器 `setTimeout` 类型。运行时依赖新增 `qrcode`。
- QR 图片固定为 320px 正方形，带浅色背景、足够 quiet zone 和像素化渲染；备用链接仍单独显示在图片下方，方便手机无法扫码时手动打开。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`、`frontend/src/types/qrcode.d.ts`、`frontend/package.json`、`frontend/package-lock.json`。
- 验证：`cd frontend; npm.cmd run build` 通过；`cd backend; go test ./internal/games/stardew_junimo -run "SteamAuthMenus|SteamGuardCodePrompt|QRCodeChoice|SteamMobileApproval"` 通过。

# FE-STEAM-AUTH-OPTIMISTIC-PHASE-1 Steam 认证选择即时反馈

- 安装页新增 `optimisticPhase`，在管理员点击 Steam 认证选择后立刻推进前端显示，不再等待后端实例状态轮询或 SSE 慢慢刷新。
- 在 `auth_method_required` 阶段点击“扫码登录”会立即显示 `steam_qr_required` 的扫码等待区域；点击“账号密码 / 验证码登录”会先回到认证运行态，避免按钮留在原地造成“没点上”的错觉。
- 在 `steam_guard_choice_required` 阶段点击“手机 App 批准”会立即显示手机批准等待；点击“输入验证码”会立即显示验证码输入框。
- QR 日志兜底只用于修正落后的选择按钮/后端阶段：日志里出现当前有效的 `https://s.team/q/...` 时可渲染扫码区域；但如果后续已经出现 Guard 验证码、手机批准、下载进度或失败状态，应以后续日志/phase 为准。
- 提交认证选择成功后会主动调用 `dashboardData.refreshInstanceState()` 拉一次最新状态；提交失败则清空乐观阶段并显示错误。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`。未改后端接口、安装 job、SSE 或 Steam 输入契约。
- 验证：`cd frontend; npm.cmd run build` 通过；后端 QR 阶段识别定向测试通过。
# FE-STEAM-POST-AUTH-RETRY-1 认证成功后的失败不再要求重新输入账号密码
- 安装页新增 `logsShowSteamAuthSucceeded()` 兜底判断：只要最新安装日志已经出现 `[steam] [SteamAuth:A0] Logged in as`、`Token expires`、`Game license verified`、`Got depot decryption key`、`Downloading app 413150` 或 `/data/game` 目标目录，就在安装页视觉状态上视为 Steam 认证已经成功/已进入后续下载。持久 `STEAM_AUTH_COMPLETED` 以后端真实 steam-auth 登录成功日志或非空邀请码为准。
- 当认证成功后发生 `download_failed`、`post_auth_failed` 或旧后端残留的通用失败状态时，左侧按钮显示“重试下载（不重新输入账号）”，安装表单只保留镜像版本确认，并通过既有 `reuseCredentials=true` 复用 `.env` 中保存的 Steam 凭据；不会再展示 Steam 用户名/密码输入框。
- 真正需要重新输入凭据的场景仍限定在 `credentials_required` 或 QR 登录失败后用户主动改用账号密码。下载/CDN/磁盘/后续安装失败不再被文案描述为“凭据错误”。
- 影响文件：`frontend/src/games/stardew/pages/InstallPage.tsx`、`frontend/src/games/stardew/install-helpers.ts`。未新增 API，继续使用 `POST /api/instances/:id/install` 的 `reuseCredentials` 契约。
- 验证：`cd frontend; npm.cmd run build` 通过。
# FE-PULL-PROGRESS-1 镜像拉取百分比

- 安装页会解析隐藏日志 `[pull:progress:done:total]` 并展示“约 N%”。`pull_running` 表示 Junimo 镜像数量进度，`steamcmd_image_pulling` 表示 SteamCMD layer 进度。
- 顶部安装总进度、右侧镜像拉取卡都会吸收该估算百分比；安装页普通日志窗口会过滤隐藏进度标记，避免用户看到内部控制行。

# FE-STEAMCMD-DOWNLOAD-PROGRESS-1 SteamCMD 游戏下载进度

- 安装页会在 `steamcmd_downloading` 阶段解析 `[steamcmd] ... progress: N (done / total)`，显示 SteamCMD 兜底下载百分比和已下载/总量。
- SteamCMD 输出 `Please confirm the login in the Steam Mobile app` 或 `Waiting for confirmation` 时，前端会切到 `steamcmd_guard_mobile_required`，提示管理员打开 Steam App 批准。
- SteamCMD 手机 App 批准超时时，后端会进入 `steamcmd_failed`，前端不应继续显示安装完成或下载中。
# FE-GAME-INSTALLED-STARTABLE-1 安装完成态可直接启动

- 修复重新安装完成后实例状态为 `game_installed` 时，服务器控制页仍把它当成不可启动状态，导致只显示“服务器未运行”且刷新无效的问题。
- `ServerControlPage` 现在把 `game_installed` 与 `ready_to_start` / `stopped` 一样视为可启动的未运行状态，启动后如果后端发现没有存档，仍走现有 `save_required` 提示与存档页入口。
- `OverviewPage` 移除 `game_installed` 下“前往安装配置”的特殊分支，改为显示启动按钮；这避免安装成功后把用户导回安装页造成误解。
- 验证：`cd frontend; npm.cmd run build`。
# FE-OPSRAIL-METRICS-RESTORE-1 右侧栏资源指标恢复轻量实时显示

- 右侧 OpsRail 的 CPU / 内存 / 磁盘重新接入 `/api/instances/:id/metrics`，Stardew 面板挂载期间立即采样一次，并按 `2s` 间隔刷新；没有用户打开前端页面时自然不会产生浏览器轮询。
- 本次只恢复右侧栏资源数值，不把 `/api/health/diagnostics` 加回普通 dashboard 初始化；Docker/Compose 版本等重诊断仍由诊断页或手动入口触发。
- 请求失败时右侧栏保留上一份样本，避免短暂 Docker/API 波动导致数值闪回空状态；页面卸载时清理 timer。
- 影响文件：`frontend/src/games/stardew/StardewPanel.tsx`。接口契约不变，继续使用现有 metrics API。
- 验证：`cd frontend; npm.cmd run build`；Browser QA 打开 `qa-layout.html?state=running`，确认右侧栏 CPU/内存/磁盘显示 mock metrics 百分比而不是空值。
# NEXUS-MODPAGE-DL-2 Shadow DOM 与 data-tracking 匹配（扩展 0.1.1 → 0.1.2）

- `NEXUS-MODPAGE-DL-1`（0.1.1）按可见按钮文案（`manual`/短 `Manual`）在 `document.querySelectorAll` 里找下载控件，Nexus 部分改版页面把下载控件渲染进 Web Component（shadow root），纯文案匹配也可能撞上无关按钮。
- `content.js` 新增 `deepQueryAll()` 遍历 `document` 及所有打开的 shadow root；`findFileIdOnPage()`、新增的 `findManualDownloadControl()` 都改用它。
- `findManualDownloadControl()` 优先按 Nexus 自带的 `data-tracking*="Download"` 属性分类下载控件（用 `manual` 关键字排除 `vortex`/`mod manager`），按是否已带 `file_id` 排序；找不到才回退旧的文案匹配。旧的两步“短按钮开模态 → 模态内点击”流程统一改成按控件是否带 `file_id` 判断（带 `file_id` 直接当下载链接跳转，否则当列表/模态开关点击）。
- 新增 `openNexusFileList()` + `waitForFileIdOnPage()`：`file_id` 未就绪时主动打开文件列表/跳转文件页，并轮询（含 `MutationObserver`）等待 Nexus 异步渲染出 `file_id`，超时 20 秒才回退到点击流程。
- 仅改 `browser-extensions/nexus-slow-installer/content.js`，并同步 `manifest.json`、`background.js`、`panel-bridge.js` 的版本/请求头到 `0.1.2`；未改后端接口。扩展 0.1.1 → 0.1.2 触发后端 `EnsureNexusInstallerExtensionZip` 版本感知逻辑，旧实例缓存 ZIP 会自动重新打包，无需手动清缓存。
- 验证：`node --check browser-extensions/nexus-slow-installer/content.js background.js panel-bridge.js`；`cd backend; go test ./internal/games/stardew_junimo -run TestEnsureNexusInstallerExtensionZip`。

# INVITE-COPY-CLIPBOARD-FALLBACK-1 邀请码/局域网 IP 复制按钮在非 HTTPS 下失效

- 现象：面板通常经 `http://局域网或公网IP:端口` 访问（非 HTTPS），`navigator.clipboard` 在这种非安全上下文下是 `undefined`；`InviteCodeCard.tsx` 原先直接调用 `navigator.clipboard.writeText(...)`，对 `undefined` 调用方法会同步抛异常，点击处理函数当场中断，复制按钮表现为"点了没反应"。
- `InviteCodeCard.tsx` 新增 `copyText(text)`：仅在 `window.isSecureContext` 为真时用 `navigator.clipboard`，否则/失败时降级用隐藏 `<textarea>` + `document.execCommand('copy')`，两条路径都有 try/catch。邀请码与局域网 IP 两个复制按钮改用它。
- 影响文件：`frontend/src/games/stardew/InviteCodeCard.tsx`。
- 验证：`cd frontend; npm.cmd run build`。

# PLAYERS-KICK-1 踢出玩家 + PASSWORD-STATUS-1 服务器密码设置弹窗

- `PlayersPage.tsx` 的踢出功能接入真实后端：玩家表格每行右侧的踢出图标按钮（`sd-players-icon-boot`）和"管理操作"卡片里的踢出玩家下拉+按钮不再是禁用占位，改为调用 `kickPlayer(uniqueMultiplayerId, name)`（新增于 `api.ts`，对应 `POST /api/instances/:id/players/kick`）。点击后弹出确认弹窗（复用现有 `sd-confirm-overlay`/`sd-confirm-dialog` 通用弹窗样式），确认后提交请求并调用 `dashboardData.refreshPlayers()` 刷新玩家名册。
  - 只有在线（`status === 'online'`）、非主机（`!isHost`）、且带 `uniqueMultiplayerId` 的玩家才能被选中踢出；主机保护是后端 SMAPI Mod 侧做的，前端这里只是提前禁用避免无意义请求。
  - 底部"管理操作"卡片的下拉框只列出满足上述条件的在线玩家；封禁/白名单/权限设置三项仍保持原有"待接入"禁用占位，未改动。
  - 页面顶部描述文案、管理操作区底部提示语同步更新，不再统一写"踢出待接入"。
- **服务器密码设置按用户明确要求放进 `ServerControlPage.tsx` 原来的"服务器设置"快捷按钮里，而不是新建 `SettingsPage.tsx` 区块**：按钮改名"服务器密码设置"并移除 `disabled`，点击调用新增的 `openPasswordSettings()` 打开一个弹窗（复用 `scheduleOpen` 那套 `sd-confirm-overlay` 弹窗模式）：
  - 弹窗内一个密码输入框（`type="password"`/`text` 切换显示）+ 保存按钮，调用新增的 `getInstanceServerPassword`/`updateInstanceServerPassword`（对应后端 `GET/PUT /api/instances/:id/config/server-password`）。保存后明确提示"需要重启服务器容器后才会生效"（因为 JunimoServer 不支持热改密码）。
  - 弹窗下半部分是"密码保护状态"只读展示，调用新增的 `getInstancePasswordStatus`（对应 `GET /api/instances/:id/password-status`，代理 JunimoServer `GET /auth`），展示是否启用、已认证/待认证人数、认证超时秒数、最大失败次数；服务器未运行时不可刷新并提示"服务器未运行，无法读取密码保护状态"。
- `types.ts` 新增 `InstanceServerPasswordConfig`、`InstancePasswordStatus`；`api.ts` 新增 `getInstanceServerPassword`/`updateInstanceServerPassword`/`getInstancePasswordStatus`/`kickPlayer` 四个函数，均沿用现有 `request<T>()` 封装，无特殊超时/AbortController（`say`/`runCommand` 那种 40 秒超时是因为要等 attach-cli 输出，这几个接口是纯 JSON 读写或 fire-and-forget，不需要）。
- 影响文件：`frontend/src/types.ts`、`frontend/src/api.ts`、`frontend/src/games/stardew/pages/PlayersPage.tsx`、`frontend/src/games/stardew/pages/ServerControlPage.tsx`。
- 验证：`cd frontend; npx tsc --noEmit -p .` 通过；`cd frontend; npm run build`（`tsc -b && vite build`）通过。**未做浏览器实测**：没有连一个真实运行中的实例走一遍"设密码→重启→玩家登录→踢人→查看认证状态"的完整交互，弹窗的实际视觉效果、移动端窄屏下的表现也未截图验证。
- 下一步注意事项：踢人是 fire-and-forget，前端拿到的 `output` 只是"指令已提交"，不代表真的踢成功了（详见 `docs/backend-handoff/backend-handoff-2026-07-09.md`）；如果用户反馈"点了踢出但玩家还在"，先确认服务器是否真的在跑最新的 `StardewAnxiPanel.Control.dll`（改了 Mod 源码后必须重启/重新准备 server 容器才会生效），而不是先怀疑前端逻辑。密码弹窗目前没有做"保存后一键重启"的联动按钮，如果后续用户反馈体验割裂，可以考虑在保存成功提示旁加一个跳转/直接触发重启的按钮。

# ADMIN-GATE-LIFECYCLE-1 启动/停止/重启按钮补齐管理员权限门控

- `ServerControlPage.tsx` 的 `canStart`/`canStop`/`canRestart` 三个派生变量补上 `isAdmin &&` 前缀；`OverviewPage.tsx` 补上 `user` prop 解构和 `isAdmin` 变量，`renderLifecycleButtons()` 里三个按钮 `disabled` 补 `|| !isAdmin`。两处都补了非管理员时的 `title` 提示。
- 起因：后端 `/start`、`/stop`、`/restart` 一直是 `requireAdmin`，但这两个组件的按钮此前没有对应的前端门控，普通用户能看到可点击按钮，点击后才被 403 拒绝。全量排查确认其余页面（Mod 上传/一键安装、玩家踢出、存档、设置、任务日志、诊断导出）本来就正确限制了管理员权限。
- 影响文件：`frontend/src/games/stardew/pages/ServerControlPage.tsx`、`frontend/src/games/stardew/pages/OverviewPage.tsx`。
- 验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过；未做非管理员账号浏览器实测，详见 `docs/frontend-handoff/frontend-handoff-2026-07-09.md`。

# USER-PASSWORD-RESET-1 用户管理新增"重置密码"

- `SettingsPage.tsx` 的 `UserManagementSection` 用户列表每行新增"重置密码"按钮，点击弹出输入新密码的弹窗（复用 `sd-confirm-overlay`/`sd-confirm-dialog` 样式，不是简单的 `ConfirmDialog` 因为需要一个密码输入框），调用新增的 `updateUserPassword(id, password)`（`api.ts`，`PATCH /api/users/{id}` body `{password}`）。
- 按钮可见性 `canChangePassword = isSuperAdmin || isSelf || !isAdminTarget`：和已有的"禁用/删除"按钮用的 `canManageTarget`（故意排除 `isSelf`）不是同一个表达式，重置密码允许改自己但管理别人（禁用/删除）不允许改自己，两者语义不同不能共用一个变量。
- 修改自己密码成功后，后端会撤销当前 session（`storage.UpdateUser` 里 `passwordChanged` 触发），前端在弹窗里显示"密码已修改，即将跳转到登录页…" 1.2 秒后 `window.location.reload()`，让 `App.tsx` 的 `boot()` 重新走一遍 `/api/auth/me` 探测到 401 自然显示登录页，没有做更复杂的全局 401 拦截器（那是更大范围的重构，本次不做）。修改别人的密码则直接 `loadUsers()` 刷新列表。
- 影响文件：`frontend/src/api.ts`、`frontend/src/games/stardew/pages/SettingsPage.tsx`。
- 验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过；未做浏览器实测（三种角色分别登录点一遍重置密码流程），建议下一位维护者补一次。

# FESTIVAL-EVENT-1 触发节日活动 + JOJA-ROUTE-1 永久启用 Joja 路线

- `ServerControlPage.tsx` 的"快捷操作"网格新增两个按钮，紧跟在"服务器密码设置"之后：
  - **触发节日活动**：`sd-btn-tan` 样式，点击直接调用新增的 `handleTriggerFestivalEvent()` → `triggerFestivalEvent()`（`api.ts`，对应后端 `POST /api/instances/:id/festival/event`），无需二次确认（上游 `!event` 本身没有副作用风险，卡住时可以反复点）。结果/错误展示复用 `sd-srv-result`/`sd-ov-error`，和"手动备份"按钮的反馈样式一致。
  - **永久启用 Joja 路线**：`sd-btn-delete`（红色危险样式）。因为这是不可逆操作（对应上游 `!joja IRREVERSIBLY_ENABLE_JOJA_RUN`，会永久禁用标准社区中心路线），没有直接调用 API，而是点击后 `openJojaConfirm()` 打开一个新的强确认弹窗（`jojaOpen`，复用 `sd-confirm-overlay`/`sd-confirm-dialog`），弹窗要求管理员在输入框里**精确输入** `IRREVERSIBLY_ENABLE_JOJA_RUN`（`JOJA_CONFIRM_TEXT` 常量）才能点亮"确认永久启用"按钮，和上游命令本身要求逐字匹配参数的交互保持一致。确认后调用新增的 `enableJojaRoute(confirm)`（对应 `POST /api/instances/:id/joja/enable`，body `{confirm}`），后端会再校验一次这个字符串（见 `docs/02-backend.md` FESTIVAL-EVENT-1/JOJA-ROUTE-1），不是只靠前端弹窗把关。
  - 两个按钮的 `disabled`/`title` 都遵循现有惯例：`!isAdmin || !isRunning` 时禁用并给出对应提示，和"手动备份"“VNC 显示”等按钮门控写法一致。
- `api.ts` 新增 `triggerFestivalEvent(instanceId?)`、`enableJojaRoute(confirm, instanceId?)`，均复用已有的 `CommandRunResult` 类型，没有引入新类型；两者都是普通 JSON 请求，没有像 `say`/`runCommand` 那样加 40 秒 `AbortController` 超时（后端是 fire-and-forget 写命令文件，不等待 attach-cli 输出，响应很快）。
- 影响文件：`frontend/src/api.ts`、`frontend/src/games/stardew/pages/ServerControlPage.tsx`。图标复用现有素材（`icon_nav_tasks_scroll_image2.png`、`icon_players_action_permission_image2.png`），没有新增图片资源。
- 验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做浏览器实测**：没有连一个真实运行中的实例实际点一遍这两个按钮观察游戏内聊天记录变化，弹窗的移动端窄屏表现也未截图验证，建议下一位维护者补一次。
- 下一步注意事项：两个操作都是 fire-and-forget，前端拿到的 `output` 只是"指令已提交"，不代表游戏内一定生效（比如当天没有节日时"触发节日活动"不会有效果，前端无法感知）；如果用户反馈"点了但没反应"，先按 `docs/02-backend.md` 的说明确认服务器容器是否已经用了最新编译的 `StardewAnxiPanel.Control.dll`，而不是先怀疑前端。
- follow-up（同日）：按用户反馈"不要让用户一个字一个字打确认文本"，Joja 强确认弹窗的输入框旁新增"填入"按钮（`sd-btn-tan`），点击直接把 `jojaConfirmInput` 设为 `JOJA_CONFIRM_TEXT`，输入框本身仍保留可编辑，不强制用户只能点按钮。这是为了在保留"确认文本必须精确匹配"这层把关的前提下，去掉不必要的手动打字摩擦。

# CABIN-STRATEGY-1 小屋策略设置分层（新建存档简化二选一 + 服务器控制页完整高级设置）

- 需求来源：用户给出明确的设计口径——`CabinStrategy`（小屋策略）不应该只在新建存档时硬编码一次。新建存档页只暴露一个简化二选一（推荐/原版），服务器控制页给完整高级设置（`CabinStrategy`/`ExistingCabinBehavior`/`NetworkBroadcastPeriod`），两边必须共用同一份后端配置来源，改完提示"重启服务器后生效"。后端契约详见 `docs/02-backend.md` `CABIN-STRATEGY-1` 与 `docs/06-integration.md` 对应小节。
- `types.ts`：`NewGameConfig` 新增 `cabinMode?: string`（`"recommended"|"vanilla"`）；新增独立类型 `ServerRuntimeSettings{ cabinStrategy, existingCabinBehavior, networkBroadcastPeriod }`。
- `api.ts` 新增 `getInstanceServerRuntimeSettings(instanceId?)` / `updateInstanceServerRuntimeSettings(settings, instanceId?)`，对应 `GET/PUT /api/instances/:id/config/server-runtime-settings`，写法和 `getInstanceServerPassword`/`updateInstanceServerPassword` 完全同构。
- `NewGameCreator.tsx`：联机设置侧栏在"联机小屋布局"上方新增"小屋模式"步进控件（复用 `ArrowButton` + 左右切换两态的模式，和"资金管理"共享/分开的写法一致，不是新增交互模式），显示"推荐"/"原版"，默认值 `recommended`。这是故意做成二选一而不是三选一（不暴露 `FarmhouseStack`）——新建存档场景下用户只需要"要不要隐藏小屋"这一个决策，`FarmhouseStack` 这类更细的变体留给服务器控制页的高级设置。
- `ServerControlPage.tsx`："快捷操作"网格里紧跟"服务器密码设置"之后新增"小屋与联机高级设置"按钮（`sd-btn-tan sd-btn--lg`，仅 `!isAdmin` 时禁用，不要求服务器运行中——因为这组配置本来就只在容器启动时生效，随时可以编辑）。点击 `openRuntimeSettings()` 打开新弹窗（复用 `sd-confirm-overlay`/`sd-confirm-dialog`，和"服务器密码设置"弹窗同构），弹窗内三个 `<select>`：
  - `CabinStrategy`：`CabinStack`/`FarmhouseStack`/`None` 三选一，选项文案直接说明各自效果。
  - `ExistingCabinBehavior`：`KeepExisting`/`MoveToStack` 二选一。
  - `NetworkBroadcastPeriod`：`1`/`2`/`3` 三个预设刻数（对应用户给的参考表格"1=每个刻，3=原版"），没有做自定义数字输入框——三个预设覆盖了绝大多数场景，加自定义输入框在这个弹窗里是不必要的复杂度。
  保存调用 `handleSaveRuntimeSettings()` → `updateInstanceServerRuntimeSettings()`，成功后提示"设置已保存，需要重启服务器容器后才会生效"，和密码弹窗的提示文案风格一致。
- 影响文件：`frontend/src/types.ts`、`frontend/src/api.ts`、`frontend/src/games/stardew/NewGameCreator.tsx`、`frontend/src/games/stardew/pages/ServerControlPage.tsx`。未新增图片素材（弹窗内是纯 `<select>`，沿用 `sd-input`/`sd-schedule-field` 既有样式类），未改生命周期 API、密码设置、计划重启、VNC 或 Junimo 通信。
- 验证：`cd backend; go build ./... && go vet ./... && go test ./...` 全绿；`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做浏览器实测**：没有连一个真实实例走一遍"新建存档选原版→服务器控制页改小屋策略→重启→确认 `server-settings.json` 变化"的完整链路，弹窗在移动端窄屏下的表现也未截图验证，建议下一位维护者补一次。
- 下一步注意事项：`ExistingCabinBehavior` 在新建存档页没有暴露入口（新档没有"已有小屋"概念，永远由后端写 `KeepExisting`），只能通过服务器控制页的高级设置事后修改；如果以后要统一到同一套表单里，需要重新设计交互而不是简单地把字段搬过去。

# APPROVE-PENDING-AUTH-1 批准待认证玩家

- 需求来源：后端新增反射 JunimoServer `PasswordProtectionService` 批准待认证玩家的能力（详见 `docs/02-backend.md` `APPROVE-PENDING-AUTH-1`），`GET /players` 每个玩家新增 `isAuthenticated`（`boolean | null`），`GET /password-status` 新增 `passwordBridgeAvailable`/`passwordBridgeDetail` 自检字段，新增 `POST /players/approve-auth`。用户明确要求待认证玩家 UI 走**独立卡片**，不合并进现有"在线玩家"表。
- `types.ts`：`StardewPlayerInfo` 新增 `isAuthenticated?: boolean | null`；`InstancePasswordStatus` 新增 `passwordBridgeAvailable?: boolean`/`passwordBridgeDetail?: string`。
- `api.ts` 新增 `approvePlayerAuth(uniqueMultiplayerId, instanceId?)`，写法和 `kickPlayer` 完全同构。
- `PlayersPage.tsx` 新增独立卡片 `sd-players-pending-auth-section`，插在"在线玩家"表和"玩家活动/最近事件"区块之间：
  - 页面首次引入 `useEffect`（此前完全由外部 `dashboardData` 轮询驱动），`isRunning` 为真时按需拉取一次 `getInstancePasswordStatus()`，不进入全局轮询层（参考 `ServerControlPage.tsx` 已有的"页面自己按需拉取"做法）。
  - 只在 `passwordStatus?.enabled` 为真时显示整个卡片；`pendingAuthPlayers` 严格用 `isAuthenticated === false` 过滤（`undefined`/`null` 不算待认证，只是"控制模组版本不支持/反射查询失败"）。
  - `passwordBridgeAvailable === false` 时卡片顶部提示反射桥不可用，禁用"批准"按钮并给出诊断文案（`passwordBridgeDetail` 放进 `title` 属性）。
  - "批准"按钮复用踢出玩家的整套"确认弹窗 → busy → 调用 API → 成功/失败提示 → `dashboardData.refreshPlayers()`"状态模式，没有引入新的交互模式；确认弹窗保留二次确认，因为批准会立即让玩家进入正式农场，不是纯只读操作。
  - 未新增任何 CSS，卡片和行内元素全部复用 `sd-srv-section`/`sd-players-table-*`/`sd-players-badge-waiting`/`sd-players-empty*`/`sd-btn-green`/`sd-confirm-*` 既有类名。
- 影响文件：`frontend/src/types.ts`、`frontend/src/api.ts`、`frontend/src/games/stardew/pages/PlayersPage.tsx`。未新增图片素材，未改踢出玩家、密码设置弹窗或轮询逻辑。
- 验证：`cd backend; go build ./... && go vet ./... && go test ./...` 全绿；`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做浏览器实测**：没有连一个真实开启 `SERVER_PASSWORD` 的运行实例走一遍"玩家连接卡在待认证 → 卡片显示 → 点击批准 → 玩家进入农场"完整链路，也未截图验证移动端窄屏布局，建议下一位维护者补一次。
- 下一步注意事项：页面里 `isWaitingPlayerStatus`（`status === 'waiting'|'pending'|'joining'`）和"等待加入"徽章是历史遗留的预留扩展点，后端从未产出过这些 status 值；本次"待认证"走独立的 `isAuthenticated` 字段和独立卡片，**没有**复用这套逻辑，两者是不同概念不要混淆。`passwordStatus` 只在页面挂载时拉取一次，不会随密码保护开关状态实时刷新，如果以后反馈这点延迟造成困扰可以加轮询或"刷新"按钮，这次按最小实现处理。

# PLAYERS-BAN-1 封禁玩家 + 玩家行操作按钮精简

- 需求来源：后端新增封禁玩家能力（详见 `docs/02-backend.md` `PLAYERS-BAN-1`，复用 JunimoServer `!ban` 聊天指令），新增 `POST /players/ban`。用户明确要求把"在线玩家"表格每行原本 3 个图标按钮（恒禁用的"发送消息"、可用的"踢出"、恒禁用的"更多操作"）精简为只保留"踢出"+新增"封禁"，图标都换成"管理操作"卡片同款真实 PNG（`icon_players_action_boot_image2.png`/`icon_players_action_ban_image2.png`）。
- `api.ts` 新增 `banPlayer(name, uniqueMultiplayerId, instanceId?)`，写法和 `kickPlayer` 完全同构。
- `PlayersPage.tsx`：
  - 行内操作区（`sd-players-row-actions`）删除"发送消息"/"更多操作"两个恒禁用占位按钮，只保留"踢出"图标按钮和新增的"封禁"图标按钮；"封禁"按钮禁用条件 `!isAdmin || !isRunning || player.isHost || !player.uniqueMultiplayerId || banBusy`，**不要求玩家在线**（封禁本来就该支持针对离线/曾经离开的玩家）。
  - "管理操作"卡片"封禁玩家"从恒禁用改为真实可用：`<select>` 选项来源 `banTargetPlayers = playerRows.filter(p => !p.isHost && p.uniqueMultiplayerId)`（同样不按在线状态过滤），"封禁"按钮按 `!isAdmin || !isRunning || !banSelectId || banBusy` 判断，移除"待接入"徽章。
  - 新增状态 `banConfirmTarget`/`banSelectId`/`banBusy`/`banError`/`banMessage`（复用已有 `KickTarget` 类型）和 `handleConfirmBan()`；后续已接入 command-result 轮询。用户真机确认封禁随容器重启丢失，确认弹窗现明确写“重启后会丢失，需要重新操作”。
- `StardewPanel.css`：`.sd-players-icon-boot::before`（行内小按钮唯一生效的那组定义）从纯 CSS `linear-gradient`/`radial-gradient` 画的靴子矢量图形改为直接引用 `icon_players_action_boot_image2.png`；新增 `.sd-players-icon-ban::before` 同样引用 `icon_players_action_ban_image2.png`；删除因移除"更多操作"按钮而变成孤儿样式的 `.sd-players-icon-more::before` 规则（这是本次改动直接导致的孤儿代码清理，不是清理无关的历史遗留）。
- 影响文件：`frontend/src/api.ts`、`frontend/src/games/stardew/pages/PlayersPage.tsx`、`frontend/src/games/stardew/StardewPanel.css`。未新增图片素材（直接复用管理操作卡片已有的两张 PNG）。
- 验证：`cd backend; go build ./... && go vet ./... && go test ./...` 全绿；`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做浏览器实测**：没有连一个真实运行实例实际点一遍行内封禁图标和管理操作卡片封禁按钮，也未截图验证移动端窄屏下两个新图标按钮的间距/触控热区，建议下一位维护者补一次。
- 下一步注意事项：管理操作卡片的 select+button 目前被一条较晚的 CSS 规则（`StardewPanel.css` 约 15131 行 `.sd-players-action-select, .sd-players-action-item > button { display: none; }`）整体隐藏（这是既有状态，"踢出玩家"卡片本来就是这样，"封禁玩家"卡片照抄同一结构保持一致），真正生效的交互入口是行内图标按钮；如果以后要让卡片内的 select/button 重新可见，需要先弄清楚这条 CSS 规则当初为什么要隐藏它们。

# PLAYERS-WARP-HOME-1 玩家回家按钮

- 玩家管理桌面页和手机玩家页新增“回家”操作，位置在“踢出”按钮左侧。桌面端为图标按钮，手机端为 44px+ 触控热区的文字按钮，均复用现有确认弹窗、busy、成功/失败提示和刷新玩家列表流程。
- 前端新增 `warpPlayerHome(uniqueMultiplayerId, name, instanceId?)`，调用 `POST /api/instances/:id/players/warp-home`。按钮禁用条件：非管理员、服务器未运行、目标不在线、目标是主机、缺少 `uniqueMultiplayerId`、当前已有回家操作处理中。
- 桌面端新增 image2 风格图标资源 `frontend/public/assets/stardew/ui/icons/icon_players_action_home_image2.png`，由玩家行 `.sd-players-icon-home::before` 引用。图标尺寸 192x192，显示为小屋 + 绿色回家箭头，用于和踢出/封禁 PNG 图标保持同一视觉体系。
- 手机端 `MobilePlayersPage` 同步增加回家确认弹窗与单玩家 busy 状态；玩家卡片操作区顺序为“回家 / 踢出 / 封禁”，不新增手机端专属接口。
- 影响文件：`frontend/src/api.ts`、`frontend/src/games/stardew/pages/PlayersPage.tsx`、`frontend/src/games/stardew/mobile/MobilePlayersPage.tsx`、`MobilePlayersPage.css`、`StardewPanel.css`、新增 PNG 图标。
- 验证：`cd frontend && npx tsc --noEmit -p .` 通过；`cd frontend && npm run build` 通过。尚未在真机多人联机环境验证点击后玩家实际落点，需结合后端 `PLAYERS-WARP-HOME-1` 做端到端测试。
# FE-INSTALL-STEAM-AUTH-BUTTON-1 安装页常驻 Steam 登录授权入口

- 安装页原“更换 Steam 账号 / 重新认证”入口已替换为总览页邀请码卡同款“登录授权”入口，并且在安装页始终显示，不再受安装完成、已有认证、重试状态或配置表单显隐条件影响。
- 总览页与安装页通过 `useSteamAuthLogin` 共用完整行为：调用现有 `steam-auth/login`、发起中反馈、服务器运行/启动时显示“停服后登录授权”并禁用、成功后跳转安装页、失败时就地显示错误。
- 影响文件：`frontend/src/games/stardew/useSteamAuthLogin.ts`、`InviteCodeCard.tsx`、`pages/InstallPage.tsx`。未新增或修改后端接口。
- 验证：`cd frontend; npm.cmd run build` 通过（仅保留 Vite chunk 大小提示）。

# SAVE-BACKUP-GAMEDAY-1 存档回档功能重构：游戏日回档 + 其他备份两栏 UI

- 后端已把自动备份体系从"现实时间"（最新备份/每日快照/定时备份）改为"游戏内日期驱动"（详见 `docs/02-backend.md`/`docs/backend-handoff/backend-handoff-2026-07-11.md` 的 `SAVE-BACKUP-GAMEDAY-1`）：`BackupPolicy` 简化为 `{ gameSaveBackups, retainGameDays }`；`BackupInfo.kind` 新增 `auto`/`predelete`/`prerestore`，`latest`/`daily`/`scheduled` 变为只读历史 kind；`BackupInfo` 新增 `gameDayOrdinal` 字段。这次做前端接线与页面重排。
- `types.ts`：`BackupPolicy` 改为 `{ gameSaveBackups: boolean; retainGameDays: number }`；`BackupInfo.kind` 联合类型追加 `'auto' | 'predelete' | 'prerestore'`，新增 `gameDayOrdinal?: number`。删除 `dailySnapshots`/`dailyRetentionDays`/`scheduledBackups`/`scheduledHour`/`scheduledIntervalHours`。`api.ts` 里 `getSaveBackups`/`createSaveBackup`/`getSaveBackupPolicy`/`updateSaveBackupPolicy`/`restoreSaveBackup`/`deleteSaveBackup` 的 URL 和函数签名完全不变，只是传输的对象形状随类型变化。
- `SavesSection.tsx`（主要改动文件）：
  - `defaultBackupPolicy`/`normalizeBackupPolicy` 按新形状重写，只 clamp `retainGameDays` 到 1–14。
  - 新增两个派生数组：`autoBackups`（`kind==='auto'`，按 `gameDayOrdinal` 降序——游戏日驱动的排序，不看现实创建时间）、`otherBackups`（其余全部 kind，按 `createdAt` 降序——这些不参与游戏日保留策略，现实时间排序符合直觉）。
  - "自动备份策略"卡片从"游戏保存后更新最新备份 + 定时备份（勾选框+每天+24小时下拉框）+ 每日快照保留滑块"三块精简为两块：勾选框"睡觉存档后创建回档点" + 滑块"保留最近 N 个游戏日"（1–14，默认 5）。删除定时备份相关的整块 JSX（勾选框、"每天"文案、24 小时 `<select>`）。
  - 原"备份列表"卡片改名"游戏日回档"，只渲染 `autoBackups`（后端已按策略限制到 N 个，不再需要"查看更多"折叠）。列改为：游戏内日期、农场、农场主、创建时间、大小、操作；主按钮文案"恢复"→"回档到此日"。
  - 新增独立"其他备份"区块渲染 `otherBackups`，沿用原有六列表格结构（备份文件/所属农场/创建时间/大小/状态/操作）和"查看更多"折叠，`kind` 徽章文案：`manual`→手动备份，`predelete`→删除存档前备份，`prerestore`→回档前保护备份，`latest`/`daily`/`scheduled`/未知前缀→历史备份（旧机制遗留文件，不再产生新的，但继续可查看/回档/删除，不会被误删）。
  - 回档入口可用性调整：游戏日回档和其他备份两个表格里的"回档到此日"行按钮不再因服务器运行中被整体 `disabled`（原来 `disabled={restoreBlocked}` 里含 `isRunning`，导致按钮静默不可点、只能靠 hover title 看到提示），新拆出 `restoreRowBlocked = busy || !isAdmin`（不含 `isRunning`），行按钮始终可点开确认弹窗；弹窗内继续显示"服务器正在运行中，无法直接回档。请先到"服务器"页停止服务器，再回来完成本次回档"的醒目警告，弹窗里"确认/覆盖回档"提交按钮维持 `restoreBlocked`（含 `isRunning`）禁用。这样运行中点击行按钮不再是一个无说明的死按钮，而是主动引导去停服。
  - 弹窗与确认文案统一把"恢复"改为"回档"："恢复备份"→"回档到此日"、"确认恢复"→"确认回档"、"覆盖恢复"→"覆盖回档"。
- `StardewPanel.tsx`：`OpsRailActiveCard`（右栏"进行中"卡）删除 `backupPolicy` state、`getSaveBackupPolicy` 拉取逻辑和 `countdowns` 计算/渲染块（定时备份倒计时行，功能已随后端移除）；保留 `restartRows`/`activeJobs`（计划重启倒计时和任务进度条不受影响）。清理随之产生的未使用 import（`BackupPolicy` 类型、`getSaveBackupPolicy`）。
- `qa-layout-main.tsx`：mock `backupPolicy` 改为 `{ policy: { gameSaveBackups: true, retainGameDays: 5 } }`；mock `backups` 的 5 条记录 `kind` 改为 `'auto'` 并补上 `gameDayOrdinal`，与真实契约保持一致（否则 `tsc` 会因类型不匹配报错）。
- `StardewPanel.css`：删除定时备份专属规则（`.sd-save-backup-toggle--schedule`、`.sd-save-backup-frequency` 及其内部 `select` 规则）；新增 `.sd-save-gameday-table`（与 `.sd-save-backups-table` 共用 `display:grid; overflow-x:auto` 及移动端横滑渐变提示）和 `.sd-save-backup-list-card--full`（"其他备份"区块只有列表卡没有策略卡，需要 `grid-column: 1 / -1` 占满整行，不被两栏网格挤到左侧窄栏）。游戏日回档表格复用既有 6 列 `grid-template-columns`（列数与旧表格相同，只是语义换了，不需要新的列宽定义）。
- 影响文件：`frontend/src/types.ts`、`frontend/src/games/stardew/SavesSection.tsx`、`frontend/src/games/stardew/StardewPanel.tsx`、`frontend/src/games/stardew/StardewPanel.css`、`frontend/src/qa-layout-main.tsx`。未改 `api.ts` 的函数签名、未改新建存档/上传存档/选择存档/删除存档流程。
- 验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过（仅保留既有 Vite chunk 体积提示）。
- **未做的验证**：没有连接真实运行实例走一遍完整链路（打开"存档"页确认"游戏日回档"/"其他备份"两栏渲染、策略卡只剩两个控件、服务器运行中点击行按钮弹出带停服引导的确认框、回档成功后列表刷新），也没有截图确认移动端窄屏下两个新表格的横向滚动表现。建议下一位维护者用 `qa-layout.html` 或真实实例走一遍。

## 下一步注意事项

- "游戏日回档"表格目前展示**全部** `kind==='auto'` 的条目，不按当前激活存档过滤（后端按存档名分别维护各自的最近 N 个游戏日配额）。正常使用场景下基本等价于"只有当前在玩的存档有回档点"；如果以后要支持"多个存档各自维护回档点并分组展示"，需要在前端按 `saveName` 分组，这次没有做。
- 计划重启（`SCHEDULED-RESTART-1`）的"关闭前备份"和服务器控制页"备份已保存进度"快捷操作现在都归为 `manual` kind，混在"其他备份"区块里用同一个"手动备份"标签展示，没有进一步区分来源，这是刻意的最小实现。

# SAVE-BACKUP-GAMEDAY-MOBILE-1 手机端游戏日回档（同日追加）

- 用户要求把手机端"存档操作"卡片里那个恒禁用、提示"回档功能暂不支持手机浏览器"的"回档"按钮删除，改成在"存档操作"卡片**上面**新增一个和桌面同名的"游戏日回档"卡片，让手机端也能直接回档，不用被引导去桌面端。
- `MobileSavesPage.tsx`：
  - 删除"存档操作"卡片里恒禁用的"回档"按钮和它下面的 `.sd-msave-op-hint` 说明文字。
  - 新增独立卡片"游戏日回档"（仅 `isAdmin` 渲染，和桌面"其他备份/游戏日回档"一样是管理员功能），复用 `getSaveBackups`/`restoreSaveBackup` API（和桌面 `SavesSection.tsx` 同一套接口，不新增后端能力）：本地维护 `backups`/`backupsLoading`/`backupsError` 状态，挂载时按 `isAdmin` 拉取一次；`autoBackups = backups.filter(b => b.kind === 'auto').sort(by gameDayOrdinal desc)`，和桌面同一套过滤排序口径。
  - 每个回档点渲染为堆叠行（游戏内日期加粗大字 + 农场/农场主一行 + 创建时间/大小一行 + 右侧"回档到此日"按钮），不是桌面那种 6 列表格——手机窄屏放不下表格，这是本次唯一的视觉改动，数据字段和桌面完全一致。
  - 回档确认弹窗复用页面已有的 `sd-msave-dialog-overlay`/`sd-msave-dialog` 结构（和"导入存档"弹窗同款），逻辑照抄桌面 `SavesSection.tsx` 的 `openRestoreDialog`/`handleRestoreConfirmed`：`restoreSaveExists` 判断是否需要覆盖、`ApiError.code === 'save_exists'` 时切换到覆盖态并展示对应警告、`overwrite=true` 提交按钮走 `sd-btn-delete` 危险色。服务器运行中不会禁用"回档到此日"入口按钮本身，而是在弹窗里展示"服务器正在运行中，无法直接回档，请先到"控制"页停止服务器"的警告，并把弹窗内提交按钮禁用——和桌面这次的调整（`restoreRowBlocked` vs `restoreBlocked`）同一套思路。
  - 页头"刷新"按钮从只刷新 `dashboardData.refreshSaves()` 扩展为同时刷新游戏日回档列表（`Promise.all([refreshSaves(), loadBackups()])`）。
- `MobileSavesPage.css`：删除孤儿规则 `.sd-msave-op-hint`（JSX 不再引用）；新增 `.sd-msave-gameday-list`/`.sd-msave-gameday-row`/`.sd-msave-gameday-main`/`.sd-msave-gameday-date`/`.sd-msave-gameday-meta`/`.sd-msave-gameday-btn` 一套堆叠行样式，独立于桌面 CSS、不跨文件共享类名（沿用这个文件一贯的做法）。
- 影响文件：`frontend/src/games/stardew/mobile/MobileSavesPage.tsx`、`frontend/src/games/stardew/mobile/MobileSavesPage.css`。未新增后端接口，未改桌面 `SavesSection.tsx`。
- 验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。
- **未做的验证**：没有用真实移动设备或浏览器窄屏模式实际点开"游戏日回档"卡片、触发回档确认弹窗、验证覆盖态文案和按钮态，建议下一位维护者用 `qa-layout.html?shell=mobile` 或真机走一遍。

# SAVE-RESTORE-AUTORESTART-1 回档时自动停止/重启服务器（同日追加）

- 后端已把"服务器运行中回档"从"整体禁用+提示先停服"改为"确认后自动停止服务器→完成回档→重新启动服务器"，`POST .../saves/backups/restore` 新增请求体字段 `autoRestart`，运行中且 `autoRestart=true` 时返回 `202 {jobId}`（异步 job，和启动/停止服务器同一套 job 轮询/SSE 机制），已停止时行为不变（`200 {saveName}`）。详见 `docs/backend-handoff/backend-handoff-2026-07-11.md` 的 `SAVE-RESTORE-AUTORESTART-1` 小节。
- `types.ts`：`RestoreBackupResult` 从 `{ saveName: string }` 改为 `{ saveName?: string; jobId?: string }`——两个响应形状二选一，`saveName` 对应停止状态下的同步回档，`jobId` 对应运行中自动重启的异步 job。
- `api.ts`：`restoreSaveBackup(backupName, overwrite, autoRestart, instanceId?)` 新增 `autoRestart` 参数，请求体透传。
- `SavesSection.tsx`：
  - `handleRestoreConfirmed` 调用时传 `autoRestart: isRunning`；响应里有 `jobId` 时调用既有 `onJobStarted(jobId)`（复用页面已经在用的 job 启动回调，接入现有轮询/SSE，不重新实现等待逻辑），不再立即刷新存档/备份列表（因为此时回档实际上还没发生，要等 job 完成）；没有 `jobId`（服务器本来就是停止状态）时保持原有立即刷新逻辑。
  - "游戏日回档"和"其他备份"两个表格的"回档到此日"行按钮、弹窗内"确认/覆盖回档"提交按钮，`restoreBlocked` 统一简化为 `busy || !isAdmin`（不再包含 `isRunning`，两个此前分别叫 `restoreBlocked`/`restoreRowBlocked` 的变量因为条件变得完全一致而合并成一个，减少重复）。
  - 弹窗内运行中警告文案从"无法直接回档，请先停止服务器"改为"确认后将自动停止服务器、完成回档，并重新启动服务器；整个过程可能需要几分钟，请勿在此期间反复点击"；提交按钮文案运行中时追加"（自动重启服务器）"后缀，busy 态运行中显示"正在停止服务器…"而不是笼统的"回档中…"。
- `MobileSavesPage.tsx`：`handleRestoreConfirmed` 同构改动（传 `autoRestart: isRunning`，`jobId` 时调用 `dashboardData.refreshJobs()` 让共享 dashboard hook 的 SSE 机制接管后续轮询和状态刷新，没有引入手机端专属的等待逻辑）；两个提交按钮的 `disabled` 去掉 `isRunning`；弹窗文案和按钮态同步桌面端的调整。
- 影响文件：`frontend/src/types.ts`、`frontend/src/api.ts`、`frontend/src/games/stardew/SavesSection.tsx`、`frontend/src/games/stardew/mobile/MobileSavesPage.tsx`。未新增 CSS。
- 验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。
- **未做的验证**：没有连接真实运行实例点击"运行中回档"，观察服务器是否真的自动停止、回档、重新启动，也没有验证 job 失败时（比如回档本身失败、或重启失败）弹窗关闭后用户能否在"进行中"任务卡片里看清楚失败原因。建议下一位维护者用测试实例走一遍完整链路。

# FE-STARTUP-HOST-CONFIRM-1 启动/重启按钮增加主机上线确认 + 邀请码停机后不再残留旧值

## 背景

用户反馈两个问题：服务器控制页"启动/重启"按钮在后端 job 完成、`state` 变为 `running` 后就立刻切回正常态，但游戏内主机角色实际上可能还没加载完，属于"切换过早"；服务器停止后邀请码卡片仍然显示上一次运行时的旧邀请码，而不是"服务器未运行"。局域网邀请（面板访问地址）本来就不受服务器状态影响，确认无需改动。

`FE-LIFECYCLE-BACKGROUND-INVITE-1`（见上文）记录过相反方向的教训：之前就是按"邀请码/玩家快照出现"判断启动完成，结果因为快照闪烁/邀请码经常拿不到导致按钮永久卡在"启动中…"，才改成纯 job+state 判定。这次修复必须同时解决"切换过早"，又不能重新引入"卡死转圈"，因此新增了超时兜底。

## 改了什么

- `ServerControlPage.tsx`：
  - 新增 `hostOnline` 派生值：从 `dashboardData.players?.players`（已有的、`state==='running'` 时每 5 秒轮询一次的在线玩家列表）中查找 `isHost === true && status === 'online'` 的条目，不新增任何轮询或 API 调用。
  - 新增常量 `HOST_ONLINE_WAIT_TIMEOUT_MS = 10 * 60_000`（10 分钟）、`useRef<number | null>` 记录进入"等待主机上线"状态的起始时间、`useState<boolean> hostConfirmTimedOut` 记录是否已超时。一个独立 effect 监听 `[isRunning, hostOnline, dashboardData.players?.updatedAt]`：只要 `isRunning && !hostOnline`，第一次进入时记下起始时间，超过阈值后把 `hostConfirmTimedOut` 置 true；一旦不再满足 `isRunning && !hostOnline`（主机上线，或服务器不再是 running），立刻复位计时器和超时标记。**超时阈值最初设成 90 秒，上线联调时发现不够用**：直接 `docker exec` 进正在运行的实例读 `.local-container/control/status.json` 和 `players.json` 发现，容器内 SMAPI 侧状态从 `save-loaded` 到主机真正出现在 `players.json` 在线列表里，实测相差了好几分钟（大存档/带模组场景），90 秒会在主机还没真正加载完时就提前超时放行，等于白做。改成 10 分钟留出足够余量。
  - **踩过一次坑**：第一版实现是在"清除 `pendingStartupAction` 的 effect"里加超时判断，结果只对"本次点击启动/重启"这个浏览器会话有效——如果用户是刷新页面或换设备打开面板，此时 job 早已结束、`pendingStartupAction` 从未被设置过（初始值就是 `null`），`startupInProgress` 直接为 false，完全绕过了这层新判断，服务器明明没有主机在线也会显示"停止/重启"正常按钮。修正为独立派生值 `awaitingHostConfirmation = isRunning && !hostOnline && !hostConfirmTimedOut`，直接作为 `startupInProgress` 的一个 OR 分支，不管本次会话有没有点过启动都会生效；`pendingStartupAction` 的清除 effect 恢复成最初的简单版本（`!hasActiveLifecycleJob && isRunning` 就清）。
  - 转圈提示文案统一改成"服务器正在启动，等待主机玩家上线后再操作。"，去掉了原来"请等待邀请码生成后再操作"的措辞——启停按钮的完成判定全程只看在线玩家列表里是否有主机，不看邀请码是否已经拿到，文案不应该暗示邀请码是判断条件。
  - 停止/重启中的 `waitingForStop` 判断未改动，用户没有反馈这一侧有问题。
- `useStardewDashboardData.ts`：
  - `refreshInstanceState` 里根因是只要后端 `state` 响应带的 `inviteCode` 字段非空就无条件 `setInviteCode`，没检查当前 `state`。后端 `doStop` 按设计不清空 `DriverPayload.invite_code`（保留历史元数据是有意行为，未改后端），所以停止后这个字段仍是旧值；虽然已有一个 effect 会在 `state` **变化**时清空邀请码，但每 30 秒一次的状态轮询会在 state 不变的后续轮次里把旧值又塞回来。
  - 改为：只有 `s.state === 'running' || s.state === 'starting'` 时才采纳 `recordedInviteCode`，否则直接 `setInviteCode(null)`，每次轮询都会自纠正。
  - **第三个坑**：`instanceState.state` 变化时清理邀请码的那个 effect，原本没有同步清空 `players`。同一个浏览器标签页里"运行过（有主机在线）→ 停止 → 再启动"时，`players` 会一直带着上一轮的旧快照（`refreshPlayers()` 失败时不会清空 `players`，只设 `playersError`），导致 `ServerControlPage` 的 `hostOnline` 用旧数据误判为真，按钮几乎一点击启动就切回正常态。修复为离开 `running` 时先 `setPlayers(null)` 再发起刷新。

## 影响文件

- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/games/stardew/useStardewDashboardData.ts`

未改后端、未改 `InviteCodeCard.tsx`（局域网邀请本来就不依赖 `instanceState`，无需改动）。

## 如何验证

- `cd frontend; npx tsc -b` 通过，无类型错误。
- 用户上线实测后反馈按钮在主机还没上线时就已经切回正常态；通过 `docker exec stardew-server-1 cat .local-container/control/{status,players}.json`（只读，未执行任何停止/重启）确认了根因：`save-loaded` 到主机出现在 `players.json` 之间实测差了好几分钟，验证了 90 秒超时阈值确实太短，据此改成 10 分钟。
- **仍未做的验证**：本机 `stardew-server-1`/`stardew-steam-auth-1` 容器是用户当前在用的真实实例，没有对它执行"停止→等待 30 秒轮询→确认邀请码不再残留"、"人为让主机迟迟不上线，确认 10 分钟后会超时放行不会永久卡死"这两条端到端链路——因为这需要真实停止/重启服务器，未经用户确认不会主动执行。建议下一位维护者（或用户本人）找一个可以随意重启的测试实例走一遍。

## 下一步注意事项

- `HOST_ONLINE_WAIT_TIMEOUT_MS` 目前是硬编码 10 分钟（已根据实测调大过一次，原为 90 秒），如果以后发现更大存档场景下还是不够，可以继续调大这个常量，不需要改动其余逻辑。
- 如果以后要给"等待主机上线"这个中间态加专属提示文案（目前复用的是原有"服务器正在启动，请等待邀请码生成后再操作"这行通用 hint），可以在 `startupInProgress` 为真但 `hostOnline` 为假时单独渲染一行更精确的提示，这次按最小改动没有做。
# FE-OVERVIEW-STARTUP-HOST-CONFIRM-1 总览页启动等待主机在线

- 总览页生命周期按钮与服务器控制页采用相同的启动完成标准：实例进入 `running` 后，仍需等待在线玩家列表出现 `isHost === true && status === 'online'`，才从“启动中…”切换为“停止/重启”。
- 该判断不依赖本次浏览器是否亲自点击启动，刷新页面或换设备打开总览页时同样生效。
- 主机确认等待保留 10 分钟超时兜底；玩家快照持续不可用时不会永久卡在“启动中…”。服务器停止、报错或主机上线后会重置等待状态。
- 影响文件：`frontend/src/games/stardew/pages/OverviewPage.tsx`。未新增或修改后端接口。
- 验证：`cd frontend; npm.cmd run build` 通过，仅保留既有 Vite chunk 体积提示。
# REAL-INSTANCE-CRITICAL-FLOWS-VERIFIED-1 关键流程真实实例验证标记

- 用户已确认真实环境验证通过：大存档启动按钮持续等待主机上线、运行中回档自动重启、多人认证/踢出/封禁/回家、睡觉生成游戏日回档点，以及 Steam 授权和镜像源降级的前端状态流转。
- 本标记取代相关历史小节中的“未做真机/端到端验证”说明；未明确列出的移动端视觉适配等验证空白仍然保留。

# FE-LIFECYCLE-STATE-MACHINE-1 生命周期状态机统一

- 新增 `frontend/src/games/stardew/useStardewLifecycleState.ts`，统一根据实例 `state/driverPhase`、active `stardew_lifecycle` job、在线主机和页面刚提交的 pending action 推导生命周期阶段。
- 状态机统一输出启动中、等待主机、停止中、运行、停止、待存档、错误和未知状态；总览页与服务器控制页不再各自维护主机上线确认和 10 分钟超时逻辑。
- 启动、停止、重启以及运行中回档产生的自动重启 lifecycle job 均消费相同的 `startupInProgress` / `waitingForStop` 结果。主机等待超时改为独立 timer，到期不再依赖下一次玩家快照更新才能生效。
- 未新增或修改后端接口。验证：`cd frontend; npm.cmd run build` 通过，仅有既有 chunk 体积提示。
# FE-UI-LIFECYCLE-STATUS-1 使用后端标准状态（2026-07-11）

- `useStardewLifecycleState` 优先使用 `/state.uiStatus`，仅在连接旧版后端时保留原有 job/主机快照组合逻辑作为兼容回退。
- 诊断页新增“服务器状态来源”，集中展示 UI 标准状态、实例/Driver、`status.json` 与 `players.json` 的状态和更新时间。
- 同一区域展示文件新鲜/过期、存档目录、缓存身份、Compose 服务状态、两段启动耗时，以及控制模组/Junimo 版本匹配；Compose 仅在诊断页加载和手动刷新时探测。

# FE-LIFECYCLE-ACTIONS-1 生命周期启停操作去重（前端拆分阶段二第一项，2026-07-11）

- 新增 `frontend/src/games/stardew/useStardewLifecycleActions.ts`，在 `useStardewLifecycleState`（状态推导）之上再包一层“操作”hook：`handleStart/handleStop/handleRestart`、`saveStartBlocker`、启停相关 6 个 state（`actionBusy`/`actionError`/`saveRequiredDetected`/`confirmAction`/`pendingStartupAction`/`pendingStopAction`）、3 个派生 `useEffect`、`showSaveRequiredPrompt`/`canStart`/`canStop`/`canRestart` 派生值，以及 `requestConfirm`/`cancelConfirm`/`confirmPendingAction` 三个确认弹窗辅助函数，全部集中到这一个 hook。
- `OverviewPage.tsx` 和 `ServerControlPage.tsx` 原本各自维护一份几乎逐行相同的实现，现在都改为 `useStardewLifecycleActions({ instanceState, dashboardData, isAdmin })` 一行接入；确认弹窗的取消/确认按钮分别接 `cancelConfirm`/`confirmPendingAction`，不再各自手写“记下 action、清空、再调用对应 handler”的闭包。
- 新增页面或组件如果需要触发服务器启停，应复用这个 hook，不要再复制 `handleStart/handleStop/handleRestart` 这类逻辑。
- 验证：`cd frontend && npx tsc -b && npm run build` 通过；用 Playwright 登录真实运行中的实例，在总览页和服务器控制页分别打开“停止”“重启”确认弹窗并点击“取消”，确认弹窗正确显示、UI 状态联动正常、控制台无新增错误——过程中未对实际运行的服务器执行任何真实停止/重启。

# FE-SERVER-DOMAIN-HOOKS-1 ServerControlPage 领域 hook 拆分（前端拆分阶段二第二项，2026-07-11）

- `ServerControlPage.tsx`（原 1437 行）按业务领域拆成 9 个独立 hook，全部放在 `frontend/src/games/stardew/` 下，与 `useStardewLifecycleActions.ts` 同级：
  - `useServerQuickBackup.ts`：手动备份当前激活存档。
  - `useServerRestartSchedule.ts`：计划重启的读取/保存/关闭前提醒分钟切换。
  - `useServerVNCSettings.ts`：VNC 端口读取、显示渲染开关、跳转 VNC 控制页（含 3 个原本挂在页面上的 `useEffect`）。
  - `useServerPassword.ts`：服务器加入密码读取/保存、JunimoServer 密码保护状态查询。
  - `useServerRuntimeSettings.ts`：小屋策略（CabinStrategy）/联机广播频率等运行时设置。
  - `useServerFestival.ts`：触发节日活动指令。
  - `useServerJoja.ts`：永久启用 Joja 路线的二次确认输入校验和提交。
  - `useServerConsole.ts`：控制台命令列表加载（服务器运行时）和执行。
  - `useServerBroadcast.ts`：全服喊话输入与发送。
- 每个 hook 只负责自己的 state + API 调用 + open/close/save 系列 handler，返回值命名尽量贴近原页面里的变量名（比如 `useServerJoja` 返回 `jojaConfirmText` 对应原来的模块级常量 `JOJA_CONFIRM_TEXT`），JSX 改动只是把 `onClick={() => setXxxOpen(...)}` 这类内联闭包换成 hook 暴露的具名函数（`openJojaConfirm`/`closeJojaConfirm`/`updateJojaConfirmInput`/`fillJojaConfirmText` 等），渲染结构和文案完全不变。
- `ServerControlPage.tsx` 现在从 1437 行降到 979 行，页面主体基本只剩 `return (...)` 里的 JSX 和少量派生值（`stateLabelText`/`lifecycleDotClass`/`selectedCommandDef`/`terminalLines`）。
- 新增同类领域（比如以后要加“定时清理日志”“Mod 热更新”这类独立功能）时，参照这 9 个 hook 的模式新开一个 `useServerXxx.ts`，不要继续往 `ServerControlPage.tsx` 里堆 state。
- 验证：`cd frontend && npx tsc -b && npm run build` 通过（`ServerControlPage` chunk 从 32.21 KB 变为 35.92 KB——9 个 hook 只被这一个页面引用，未产生额外可共享的 chunk，属预期）。用 Playwright 登录真实运行中的实例，依次打开“计划重启”“服务器密码设置”“小屋与联机高级设置”“永久启用 Joja 路线”弹窗，确认真实数据正确加载、Joja 确认框输入联动正确，然后全部点击“取消/关闭”退出——**没有保存/提交任何一个弹窗**，未对运行中的实例做任何写操作；控制台无新增错误（仅有登录前既有的 401 探测）。

# FE-PLAYER-LOCATION-NORMALIZE-1 玩家位置统一格式化

- 新增 `frontend/src/games/stardew/location-format.ts`，桌面玩家表、最近事件、移动玩家页和总览在线玩家统一调用 `formatStardewLocation` / `readableStardewLocation`。
- Stardew 内部位置实例名会保留在 API/SQLite 原字段中，但显示前归一化逻辑类型：`FarmHouse<UUID>`、`Cabin<UUID>`、`Cellar<UUID>`、`Shed<UUID>`、`Barn<UUID>`、`Coop<UUID>`，以及对应数字后缀，分别按基础类型映射为中文可读名称。
- 精确映射优先于归一化，例如已有 `Barn2` / `Barn3` 等建筑等级名称仍使用 `LOCATION_ZH` 的具体标签；只有没有精确标签时才剥离实例后缀。
- 玩家位置统一显示为 `可读名称 (tileX, tileY)`；缺坐标时只显示名称。桌面玩家表的 `title` 保留原始唯一位置名，便于诊断。
- 验证：`cd frontend; npx.cmd tsc --noEmit -p . && npm.cmd run build`。

# FE-SHARED-WALLET-PERSONAL-INCOME-1 共享钱包个人收入文案

- 玩家表在 `walletMode=shared` 时不再把 `personalIncome=0` 显示成 `0g`，统一显示“共享模式不统计”。
- 分开钱包仍显示个人累计收入；农场收入继续显示团队累计收入。
- 底部说明同步明确：共享钱包的现金属于团队，原版不记录每位玩家个人累计收入。
# FE-LIFECYCLE-LIVE-SIGNAL-PRIORITY-1 主机在线与停止中状态优先级

- 修复在线玩家列表已出现在线主机后，按钮仍等待邀请码出现才脱离“启动中”的问题。根因是共享生命周期 hook 优先采用刷新频率较低的后端 `uiStatus`，且本地 `pendingStartup` 在 lifecycle job 等待后台邀请码探测期间持续为真。
- 新规则：`state=running` 且共享玩家列表出现 `isHost && status==='online'` 时，立即结束启动中间态，不再等待 `uiStatus`、job 或邀请码刷新；邀请码继续作为后台独立信息加载。
- 修复点击停止后没有立即出现“停止中”：本地 `pendingStop`、`state=stopping` 或 driver stopping phase 现在优先于旧的后端 `uiStatus`；总览页和手机总览也把停止分支放在启动分支之前。
- 影响文件：`useStardewLifecycleState.ts`、`pages/OverviewPage.tsx`、`mobile/MobileHomePage.tsx`。服务器控制页复用共享 hook，无需单独复制判断。
- 验证：上述四个相关 TypeScript 文件独立 `tsc --ignoreConfig --noEmit` 通过。完整前端构建被工作区中尚未接线完成的 `ServerControlPage.tsx` hook 拆分改动阻塞，与本次生命周期修改无关。
# FE-MODS-MANAGEMENT-HOOK-1 ModsPage 本服管理领域 hook 拆分（前端拆分阶段二）

- 新增 `frontend/src/games/stardew/useModsManagement.ts`，集中管理本服 Mod 列表加载、上传弹窗与多文件上传、删除确认、整包导出、玩家同步分类、完整/更新同步包导出，以及当前存档启用状态切换。
- `ModsPage.tsx` 改为通过 `useModsManagement({ dashboardData, activeSaveName })` 接入上述 state、effect 和 handler；Nexus 搜索、API Key 与浏览器扩展批量安装仍保留为同一套强耦合状态机，未改变轮询、sessionStorage 恢复或任务日志跳转行为。
- API、JSX 结构、CSS 类名和用户文案均未调整；页面从 2536 行降到 2360 行。
- 验证：本次改动自身的 TypeScript 未出现错误；完整 `npx tsc -b` / `npm run build` 当前被并行 `SavesSection` 拆分新增文件 `useSaveBackups.ts` 的未使用参数错误阻塞。
# FE-CSS-SPLIT-1 前端拆分阶段三：桌面页面 CSS 按需加载

- `StardewPanel.css` 从约 16586 行的桌面全量样式拆为共享 Shell CSS（约 4551 行）和 9 个页面 CSS：`InstallPage.css`、`OverviewPage.css`、`ServerControlPage.css`、`SavesPage.css`、`JobsLogsPage.css`、`PlayersPage.css`、`ModsPage.css`、`DiagnosticsPage.css`、`SettingsPage.css`。
- 每个懒加载页面在自身 TSX 中 import 同名 CSS，页面规则随对应页面 chunk 按需加载；Shell 布局、导航、通用按钮/卡片、跨页面合并选择器，以及 `InviteCodeCard`/`ServerSummaryCard` 等共享组件使用的规则继续留在 `StardewPanel.css`。
- 拆分基于 CSS AST 处理媒体查询，未修改选择器内容或声明值。首轮 Vite 构建确认生成 9 个独立桌面页面 CSS chunk；随后对跨页面合并选择器做保守回收，10 个 CSS 文件均通过 PostCSS 解析。
- 完整 TypeScript/Vite 复验当前被并行 `SavesSection.tsx` hook 拆分中的重复声明和未完成接线阻塞，与 CSS import/解析无关；待该任务收尾后应重新执行 `npx tsc -b && npm run build`。
- 回归修正：初版拆分改变了原单文件中“页面基础规则 → 文件后半段统一皮肤覆盖”的级联顺序，导致全部桌面页面重新出现旧纸张点纹、旧边框等风格。现已把共享 CSS 中每个页面相关的最终覆盖复制到对应页面 CSS 末尾，恢复原先最终覆盖优先级；共享组件规则仍保留在共享 CSS。阶段三后续必须以级联顺序为第一约束，不能只按选择器归属移动规则。

# FE-SAVES-DOMAIN-HOOKS-1 SavesSection 回档领域 hook 拆分（前端拆分阶段二 SavesSection 项，2026-07-12）

- 新增 `frontend/src/games/stardew/useSaveBackups.ts`：备份列表加载、备份策略读取/保存（`defaultBackupPolicy`/`normalizeBackupPolicy` 常量和函数也搬进这个文件）、手动备份、彻底删除备份，以及 `autoBackups`/`otherBackups` 两个派生排序数组。入参 `{ isAdmin, setBusy }`。
- 新增 `frontend/src/games/stardew/useSaveRestore.ts`：回档确认弹窗的完整状态机（打开/取消/提交、`ApiError` 的 `save_exists` 分支、运行中自动停止/回档/重启的 `jobId` 分支）。入参 `{ saves, isAdmin, isRunning, busy, setBusy, onJobStarted, onStateRefresh, onSavesChanged, loadSaves, loadBackups, clearBackupMessage }`。
- `SavesSection.tsx` 里跨越备份/回档两个 hook 共用的 `busy`/`setBusy` 忙碌锁**没有**下沉进任何一个 hook，仍然声明在 `SavesSection` 组件顶层，作为参数分别传给两个 hook——原代码里手动备份、彻底删除备份、回档提交这三个操作和存档选择/删除/导出共用同一个 `busy`，任意一个进行中会让所有相关按钮一起禁用；如果各自拆一份独立 busy 会改变这个"一个写操作进行中全部按钮联动禁用"的行为，所以保留共享。
- 存档列表 CRUD（`handleSelect`/`handleSelectAndStart`/`handleDeleteConfirmed`/`handleExport`）、新建游戏弹窗（`handleNewGameSubmit`）、上传存档弹窗（`handleUploadPreview`/`handleUploadCommit`/`handleUploadCancel`）**没有**拆分——这些不属于"回档"领域，upload 弹窗本身已经有独立的 `uploadBusy`，耦合度低，本次按 `docs/07-later-optimizations.md` 登记的范围（"回档逻辑拆 hook"）只拆备份和回档两块。
- `SavesSection.tsx` 从 1236 行降到 1131 行；`SaveCard` 组件、`backupKindLabel`、`saveFarmMapSrc`/`saveProgressText` 等纯展示 helper 未改动。
- 验证：`cd frontend && npx tsc -b && npm run build` 通过（此前 ModsPage/CSS 拆分两个并行任务提到的构建阻塞，是因为当时本次改动还在进行中，现已收尾，三项改动可以一起正常构建）。用 Playwright 登录真实运行中的实例，打开"游戏日回档"的"回档到此日"弹窗和"其他备份"的"彻底删除备份"弹窗，截图确认真实数据正确渲染（回档弹窗正确识别到同名存档已存在、展示"确认回档"和"覆盖回档"两个按钮），全部点击"取消"关闭——**没有提交任何一次真实的回档或删除操作**。
# FE-PLAYER-COMMAND-RESULTS-1 玩家操作精确回执

- 桌面与手机玩家页对 `warp-home`、`kick`、`approve-auth` 共用 `player-command-results.ts`：提交响应有 `commandId + queued` 时每 500ms 查询一次结果，最多 10 秒；HTTP 请求本身不等待控制模组。
- queued/running 显示“处理中…”；succeeded 使用具体中文成功信息；failed 按结构化 `errorCode` 映射中文错误；unknown/expired/dispatched 或 10 秒超时显示“未收到执行结果”，不会写成“执行失败”，也不会自动重试命令。
- 旧控制模组提交响应没有 `status: queued` 时不轮询，继续显示后端原“指令已提交”文案。
- busy 改为目标玩家 ID：同一玩家处理期间禁止重复操作，不再因为一个玩家的请求锁住其他玩家。手机端补齐了已有桌面端的待认证玩家批准入口，权限、主机禁用和桥能力检查保持一致。
- 状态分类测试：`npm run test:command-results`；完整验证：`npm run build`。
# FE-BROADCAST-BAN-RESULTS-1 喊话与封禁回执

- 桌面 `useServerBroadcast` 与手机 `MobileControlPage` 均复用 command result 轮询：queued/running 为处理中，succeeded 显示“消息已交给游戏聊天系统”并明确“不保证每个客户端实际收到”，failed 显示结构化中文原因，unknown/超时显示“未收到执行结果”，不自动重试。旧模组继续显示提交文案。
- 桌面与手机玩家页的 ban 也使用同一轮询器：succeeded 显示“已封禁”；仅 Junimo 名字降级派发时显示“封禁指令已发送给 JunimoServer，最终结果请结合游戏状态确认”；failed 显示具体原因；unknown 不视为失败。
- 用户已在真实实例确认封禁记录会在服务器容器重启后丢失，因此两端确认弹窗改为确定性限制说明，不再写“可能失效”。本阶段不新增封禁名单和解封 UI。

# EVENT-JOJA-SAVE-RESULTS-1 前端回执

- 桌面与手机端的节日、Joja 操作复用 command result v1 轮询：queued/running 显示“处理中…”，dispatched 显示“指令已发送，等待游戏处理或需结合游戏状态确认”，succeeded 只表示已确认最终效果，failed 按结构化错误码显示中文。
- `unknown`、`expired`、查询异常和客户端超时统一显示“无法确认最终结果，请先检查当前游戏状态再决定是否重试”，不会自动重试。旧控制模组没有 queued 能力标志时继续展示后端原“指令已提交”文案。
- 桌面/手机服务器控制页新增“请求游戏内保存”。它与“手动备份”明确分开：保存按钮最多等待 125 秒轮询同一 commandId，只有 Saved 回执才显示完成；`save_timeout` 显示明确超时。ZIP 备份仍只打包已落盘目录。
- Joja 的不可逆精确文本确认弹窗保持不变；dispatched/unknown 后不自动再次提交。`npm run test:command-results`、`npm run build` 均通过。

# COMMAND-RESULT-PRODUCTIZATION-1 最近控制命令与诊断

- “任务与日志”页新增响应式“最近控制命令”表格，展示命令类型、目标、提交人、精确状态、提交/完成时间、结构化消息/错误码/白名单详情；每 5 秒刷新并支持工具栏手动刷新。
- `dispatched` 使用独立黄色中性状态，绝不复用 succeeded；unknown/expired/failed 各自保留明确标签。`resultSupported=false` 固定显示“已提交（旧模组）/已提交，无法获取精确结果”。
- 诊断页新增 commandResultVersion、待消费命令、未入库结果、最老待处理、最近模组消费和 commands/command-results 可写性，并直接展示卡死/版本/权限警告。
- 新增前端类型 `ControlCommand`/`ControlCommandsResponse` 和 API `getControlCommands`。桌面表格在窄屏保持横向滚动，不改变现有命令按钮、轮询或手机控制页行为。
- 验证：`npm run build`。
# FE-SAVE-BACKUPS-NULL-GUARD-1 新服务器存档页黑屏修复

- `useSaveBackups.loadBackups()` 不再无条件信任 `result.backups` 为数组；仅在 `Array.isArray` 时写入，否则降级为空数组。
- 该保护兼容旧后端曾返回的 `backups: null`，避免存档页对空值执行展开/过滤时抛出异常并卸载整个 React 面板。
- 验证：`cd frontend; npm run build`。
# JUNIMO-STACK-UPDATE-1 阶段三：成对升级界面（2026-07-13）

- 新增 apply 类型/API 和完整阶段文案；诊断页仅在当前推荐版本对 dry-run `succeeded` 后启用“更新运行组件”，server/auth 始终作为一个操作，不提供单组件按钮。
- 确认弹窗展示当前/目标两组件 tag、升级期间停服、Steam 授权预计保留，以及原停止实例会为验证临时启动后恢复停止。提交体固定为 `{"confirm":true}`，不携带目标信息。
- 页面加载和活动轮询恢复最近 apply；展示进度、成对目标、检查项、warning、脱敏日志，并用不同文案区分 `succeeded`、`failed_rolled_back`、`rollback_failed`。恢复失败只显示人工处理指引，无自动破坏性重试。
- apply 区域及长镜像/digest 使用 `min-width:0`、`overflow-wrap:anywhere`，窄屏按钮全宽、检查项纵向换行，避免页面级横向溢出。
# GAME-RUNTIME-VERSION-1：游戏运行文件版本提示（2026-07-14）

- 管理员诊断页新增“游戏运行文件版本”，以“游戏版本/联机运行库”为主文案，详情展示 App 413150/1007 当前与推荐 buildid、StateFlags、固定 manifest 路径、安装目录标记、缺失/损坏/未知原因和 tested 推荐矩阵版本。
- 管理员可运行只读预检并查看空间估算、Steam 下载能力、staging 能力 checks/warnings；页面明确“仅检查，不提供升级按钮”。长 buildid、路径、安装目录和错误复用 `sd-diag-image-ref`/dry-run 样式任意断行，适配移动端。
- 总览仅在管理员成功读取状态、推荐矩阵 `tested=true` 且整体为 `update_available` 时显示“游戏运行文件可更新”；缺失、损坏、custom/unknown、未测试矩阵均不冒充更新提示。
- 新增 `runtime-components-status.ts` 与 `test:runtime-components`，覆盖六种状态文案和 tested 门控；生产构建继续通过 `tsc -b`。

## SMAPI 推荐版本与安全升级（2026-07-14）

- 管理员诊断页新增独立 SMAPI 卡片，显示实际检测版本、推荐版本、程序集元数据来源、推荐 installer SHA256/大小，以及 Stardew、SDK、Junimo、steam-auth-cn、Control/commandResultVersion 五类兼容门槛。
- 前置不匹配、自定义/未知或安装损坏时升级按钮禁用；卡片链接到同页“游戏运行文件版本”和“Junimo 运行组件版本对”入口，并明确本流程不会连带更新前置组件。
- 管理员总览仅在后端实际检测为 `update_available` 且 `available/supported=true` 时显示 SMAPI 更新提示；missing、invalid、incompatible、custom/unknown 不冒充可更新。
- UI 分开展示 dry-run 与 apply 的下载、ZIP 校验、staging、复制、官方安装、停服、volume 切换、完整 stack 验收、恢复状态和回滚阶段；`rollback_failed` 显示保留材料与人工处理提示。
- 页面提示 SMAPI 升级后玩家可能需要重新导出完整同步包，增量 Mod 包不含 SMAPI，客户端应与服务器推荐 SMAPI 版本保持一致。
- 长 SHA、volume、错误和日志可任意断行；操作按钮在 620px 以下满宽，卡片/网格 `min-width:0`，无新增横向固定宽度。新增 `smapi-update-status.ts` 与 `npm run test:smapi-update`。
- 总览只在 `available=true + supported=true + update_available` 时显示“游戏模组运行环境可更新”，不会把 GitHub discovered 候选当作用户目标。诊断页同时显示只读 dry-run 的 staging 空间估算与“未创建 volume/未下载/未停服”边界；apply POST 由通用 API client 序列化严格 `{confirm:true}`，不再二次 JSON 编码。

## 2026-07-14：统一“运行环境版本”视图

诊断页增加统一版本总览，按 Junimo server/auth、游戏/SDK、SMAPI/控制 Mod 三组显示当前值与当前 Panel 内嵌目标，同时展示 stackVersion、stable/preview 通道、minimumPanelVersion 以及 recommended/withdrawn 状态。用户升级 Panel 后，页面直接比较已安装组件与该 Panel 指定版本并提示对应升级。每组链接到原有独立事务入口，并说明停服、验收、回滚及完整玩家同步包影响。

界面不提供“全部更新到 latest”按钮。withdrawn 与非 recommended 状态使用风险徽标，后端门禁同时禁止操作。矩阵卡片、镜像引用和阶段日志均设置可换行；620px 以下三组改为单列，避免长 digest、buildid 或镜像名导致横向溢出。
# FE-COMPONENT-UPDATE-CARD-1 卡片内一键升级进度（2026-07-14）

- “版本维护”中的 Junimo 与 SMAPI 更新改为用户视角的一键流程：管理员确认一次后，在当前卡片内依次展示校验、下载、安装和验收，不再要求进入多层技术区重复点击。
- Junimo 镜像下载直接展示当前组件、完成层数/总层数和百分比；失败或回滚失败继续留在维护区，不会错误退回“无需处理”。
- 游戏/SDK 当前只有安全预检能力，卡片明确显示“仅校验”，不伪造尚未存在的在线安装进度。
- “维护与技术详情”保留原始 checks、镜像、digest、日志和恢复原因，作为管理员/开发者排障信息，不再承载主要升级入口。
