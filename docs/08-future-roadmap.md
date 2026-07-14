# 2026-07-15 已完成：MODBUNDLE-1

- [x] Mod ZIP 递归发现多层 manifest，支持整个 Mods 文件夹重新打包上传。
- [x] 禁止静默部分安装；深层无效 Mod、重名和嵌套歧义均原子失败。
- [x] 兼容 Windows GBK/GB18030 ZIP 中文路径、大小写入口文件和数字 `UpdateKeys`。
- [x] 桌面/手机端显示 ZIP、发现、安装和启用数量。
- [x] 用 3095 条目/38 manifest 的 `Mods1.zip` 在隔离目录和本机真实存档完成 38/38 导入与启用验证。
- [x] 聚合 ZIP 按真实子包记录稳定 `packageKey`，保留旧 Nexus 同包删除兼容，阻止无关 Mod 继承第一个 Nexus ID。
- [x] 内容包卡片恢复 `[CP]`/`[FTM]` 前缀；配偶助手数字 `UpdateKeys` 不再生成解析失败卡片。
- [x] 已安装列表展示全部本地 Mod，Nexus 来源仅增强卡片信息，不再决定用户是否有权查看该 Mod。

# 2026-07-15 已完成：PANEL-0.2.10-RELEASE

- [x] 组件升级任务代际修复已完成本地延迟竞态点击验证和完整发布门禁。
- [x] 发布门禁已包含 `test:component-update-flow`，后续未通过该回归测试不得打包。
- [x] 使用新 Tag `v0.2.10` 发布，不覆盖历史 Tag。

# 2026-07-14 已完成：FE-MAINTENANCE-SINGLE-CARD-1

- [x] 用户卡片内直接完成校验、下载、安装和验收，不再跳转到底部开发者详情。
- [x] 正常 Junimo 更新只保留“立即升级”；错误/人工恢复原因直接显示在卡片。
- [x] 清理总览中过期的“阶段一不会升级”文案。

# 2026-07-14 已完成：JUNIMO-ROLLBACK-TAG-RESTORE-1

- [x] 回滚重建期间使用精确 image ID，退出时恢复原始 tag 配置，避免裸 digest 破坏版本检测。
- [x] `rollback_failed`、`invalid_config` 和读取失败不再显示“已是推荐版本/不用做任何事”。
- [x] 增加后端回滚终态和前端维护判断回归测试。

# 2026-07-14 已完成：RUN-SH-LATEST-UPDATE-1

- [x] `run.sh update/force-update` 未指定版本时自动解析最新正式 Release，不再被 `.env` 中旧的精确镜像 tag 截停。
- [x] 日常启动/重启继续固定当前版本；最新版本解析失败时安全终止，不伪报旧版本更新成功。
- [x] 增加 shell 回归测试并接入 tag release gate，覆盖自动目标、显式目标与解析失败。

# 2026-07-14 已完成：RUNTIME-MATRIX-MIRRORS-1

- [x] Junimo server 与 steam-auth-cn 升级矩阵候选顺序和安装流程完全统一。
- [x] 所有别名强制绑定单一 canonical digest，拉取失败或校验失败自动回退，仍拒绝自定义升级目标。
- [x] 增加安装/矩阵顺序一致性测试，以及 Go/Python 同 digest 门禁。
- [x] release gate 区分必需仓库与可回退第三方代理，兼顾制品安全和发布可用性。

# 2026-07-14 已完成：PANEL-UPDATE-HISTORY-STALE-1

- [x] 真实当前版本高于历史成功目标时，不再出现“先显示新版本、再闪回旧版本”。
- [x] succeeded/failed_rolled_back 仅在与当前版本相关时主导页面；活动任务与 rollback_failed 优先级保持不变。
- [x] 增加异步加载旧 apply 状态的精确回归测试。

# 2026-07-14 已完成：PANEL-UPDATE-CONTINUOUS-1 连续升级修复

- [x] 历史成功升级不再覆盖后来发现的更高版本，也不再永久锁住下一次升级。
- [x] 旧 dry-run 必须与当前最新目标精确匹配，避免把上一次版本的环境检查误用于下一版本。
- [x] 增加“历史成功 + 新版本”回归测试，并保留 active、同目标成功和 rollback_failed 安全门禁。

# 2026-07-14 已完成：FE-DIAGNOSTICS-USER-FIRST-1 服务器健康页重构

- [x] 默认视图改为“整体是否正常 → 有没有要处理的版本维护 → 具体检查结果”，降低普通使用者阅读成本。
- [x] 状态来源、兼容矩阵、镜像/构建信息、预检与升级日志集中到默认折叠的维护详情，原功能与安全门禁均保留。
- [x] `.121 → .125` 以“不升级仍可继续使用”的可选推荐呈现；管理员可从摘要定位到预检区，普通用户不出现升级操作。
- [x] 增加 QA 更新状态 fixture，并完成桌面主视图、详情展开、无溢出和无控制台错误验收。

# 2026-07-14 已完成：Panel 0.2.2 推荐 JunimoServer preview.125

- 内嵌推荐矩阵、新实例默认镜像和 `TestedImageTag` 已切换到 `.125`，保留 auth-cn `1.5.0-anxi.2` 的真实 `.121` 溯源并记录跨版本协议兼容验证。
- `.121` 不强制升级：现有实例继续可用，只显示推荐更新；管理员仍通过现有 dry-run/确认/apply/回滚闭环自愿升级。
- `.125` 的 23 个 init 兼容挂载继续保留并通过实镜像脚本验证。后续可独立评估旧联机存档 host-swap 向导，不纳入本次 0.2.2 发版范围。

# 2026-07-13 已完成：JUNIMO-STACK-UPDATE-1 阶段二

- 已完成 server + steam-auth-cn 成对 dry-run：严格空 POST/GET、专用 job/双向互斥、可信候选 inspect/pull/digest、Compose/认证卷/运行态/磁盘 warning 检查、脱敏持久状态和前端刷新恢复/进度展示。
- 阶段一内置推荐版本对及推荐 tag 不变；阶段三 apply、备份、停服重建、成对写回与失败回滚尚未实现。

# 2026-07-13 已完成：JUNIMO-STACK-UPDATE-1 阶段一

- 已完成构建内置、强校验的 Junimo server + steam-auth-cn 推荐版本对清单，实例 `.env` 五字段只读检测、五态模型、管理员详情 API、脱敏 runtimeDiagnostic、总览更新提示和诊断页整体展示。
- 推荐版本保持当前实测的 server `1.5.0-preview.121` + steam-auth-cn `1.5.0-anxi.2`；不跟随远程 latest，不做 preview/anxi semver 排序，自定义镜像不判断可覆盖。
- 阶段一明确不包含 pull、修改 `.env`、stop/recreate、dry-run、apply、升级备份或回滚；阶段二/三列入 `docs/07-later-optimizations.md`，后续需独立安全设计和授权。

# PANEL-UPDATE-RELEASE-1 状态（2026-07-13）

- **已完成，随 v0.2.0 发布**：版本检测、独立 updater/dry-run、apply/回滚、完整 Web 交互和隔离真 Docker 发布闭环均已完成。
- 发布阻塞修复：helper 保持宿主 Compose 绝对路径，避免升级后 Compose labels 指向临时 `/deployment`；已新增 contract regression tests。
- 已验证成功升级与 unhealthy 自动回滚、数据库恢复、游戏服务容器不变、断线重连、权限/并发/unsupported 边界、桌面和移动布局。
- v0.2.0 作为首个包含 Web updater 的正式版本发布；从不含 updater 的历史版本进入 v0.2.0，需要沿用现有部署更新方式完成一次引导升级，后续版本即可在面板内升级。

# 2026-07-13 已完成：FE-PANEL-UPDATE-1 完整 Web 面板升级交互

- 已完成全局 `PanelUpdateProvider/usePanelUpdate`、顶栏/总览双入口同步、管理员二次确认、普通用户只读、完整阶段时间线和桌面/移动统一弹窗。
- 面板预期断线进入专用全屏状态，以 `/health`、`/api/version`、apply 状态退避重连；恢复后保留原路由并自动打开成功、已回滚或恢复失败结果。
- 状态机测试覆盖权限、成功、活动阶段、回滚、终态、双入口派生和退避；浏览器 QA 覆盖 1280 桌面、900 窄屏、390 移动端、普通用户、回滚、离线及断线后成功恢复。
- 完整 Web 升级闭环至此完成；后续只保留历史记录/通知增强，不扩展为任意镜像、任意服务或 shell 操作。

# 2026-07-13 已完成：PANEL-UPDATE-APPLY-1 面板升级后端执行链路

- 已完成管理员 apply API、SQLite 在线一致性备份、独立 helper 精确版本拉取、panel 单服务重建、Docker health + `/health` + `/api/version` 三项验收，以及失败自动恢复数据库/Compose/`.env`/旧镜像。
- 状态跨 panel 重启持久化，终态区分 `succeeded`、`failed_rolled_back`、`rollback_failed`；并发、dev/相同/降级、unsupported、任意 body/镜像和普通用户均拒绝。
- 隔离临时 Compose 真 Docker 测试确认 panel 可替换且 game 哨兵容器不变；脚本化 contract tests 覆盖拉取、重建、unhealthy、版本不匹配、备份失败和回滚成败。
- 后续完整升级交互已由 `FE-PANEL-UPDATE-1` 完成，不扩展服务范围或发布流程。

# 2026-07-13 已完成：PANEL-UPDATER-DRYRUN-1 独立 Updater 与部署环境演练

- 已完成 Docker 自容器 inspect、Compose labels/显式环境兜底识别、能力响应、管理员 dry-run API、独立镜像内 `panel-updater` 和跨主进程重启可读的原子状态文件。
- dry-run 目标只来自项目硬编码可信仓库并使用精确版本 tag；helper 只执行 image inspect/pull 与 Compose config，不执行当前面板的停止、删除、重建或重启。
- 管理员更新弹窗可发起“检查升级环境”并查看 supported/unsupported 与脱敏日志；普通用户保持只读版本展示。本阶段仍无“立即升级”。
- 支持模式：标准 run.sh/单文件 Compose panel 服务；或 inspect 挂载与四个显式宿主机变量完全一致的兜底。缺 Docker Socket/Compose、普通 docker run、缺失/冲突 labels、多 compose 文件和不可验证自定义编排均安全拒绝。

# 2026-07-13 已完成：PANEL-UPDATE-CHECK-1 面板版本自动检测与展示

- 后端已完成稳定 GitHub Release 检测、语义版本比较、启动检查、6 小时抖动调度、成功结果缓存与失败保留，并提供登录可读/管理员可刷新的版本状态接口。
- 前端已完成共享状态、桌面顶栏与总览原区块复用、移动端入口及统一更新详情弹窗；有更新时顶栏和总览统一显示“发现新版本 vX.Y.Z”，普通用户只读，管理员可手动刷新。
- 该阶段边界原为“检测和展示”；后续执行链路现已由 `PANEL-UPDATE-APPLY-1` 独立完成。

# 2026-07-10 已完成：手机端背景、顶栏与存档卡片继续优化

- `MOBILE-SHELL-SAVES-REFINE-1` completed：手机端整体背景改为复用 PC 端 `background_app_black.png` 深色纹理，主体区域复用 PC `.sd-main` 的 image2 页面框素材（四角、四边、中心 tile）；顶栏新增 `mobile_topbar_framed_generated_image2.png` 手机端专用四边框木纹横栏素材（imagegen 生成后整张缩放到 1170×174，不裁切，保留完整边框）并保留鸡图标做轻量化移动版，避免手机宽度下出现 PC 顶栏拼接接缝/撕裂感。移动壳固定为一屏，主体背景、顶栏和底栏保持不动；内容改由 `.sd-mshell-scroll` 内层滚动并被 `.sd-mshell-body` 上下边框安全区裁切，不再遮挡背景自带木纹框线；底部 Tab 占位挪到内层滚动区底部 padding，页面框背景铺满到屏幕底部，不再露出黑底；上方滚动内容额外 padding 收为 0，且 `.sd-mshell-body` 上方安全区减半，组件更贴近上方裁切区内侧顶格显示。
- 手机端存档页删除独立大农场原画卡，农场图缩小为核心信息卡左侧约 72px 宽的 16:9 缩略图，和存档名称、农场名称、农场主、游戏日期合并展示；当前使用中/可用状态徽章放在缩略图上方，不再覆盖图片。
- 手机端服务器模组页恢复隐藏内置/系统运行组件，只展示用户安装 Mod；搜索页和启用/禁用接口逻辑不变。
- 验证：`cd frontend; npx tsc --noEmit -p .` 通过；`cd frontend; npm run build` 通过（仅 Vite 主 chunk 大小提示）。

# 2026-07-10 已完成：手机端模组页 M7

- `MOBILE-MODS-M7-1` completed：手机端底部 Tab 已在“玩家”和“存档”之间新增“模组”，移动壳底栏调整为 6 个入口。新增 `MobileModsPage`，页面右上角保留刷新、导出、上传 Mod，内部二级 Tab 为“搜索 / 服务器模组”。
- 搜索页复用现有 Nexus 搜索接口和 `/mods` 安装状态数据，但按用户反馈做成移动端精简展示：删除搜索框上方 Nexus Key/扩展连接区，去掉安装按钮，移除来源/作者/下载/认可四项，搜索结果每页 4 个，卡片底部前置状态按钮与小号“跳转 N站”按钮同排平齐，分页控件收为单行。
- 服务器模组页把已安装信息和启用状态合并在同一张卡片中：左侧展示 N 站缩略图，右侧展示名称、状态、版本、文件夹、更新时间和同步类型；作者/来源字段已按反馈删除。真实可点击启用开关改为底部标签行右侧的无文字绿色小开关；进入该子页时主动刷新 `GET /mods`，切换启用状态后刷新当前列表。后续 `MOBILE-SHELL-SAVES-REFINE-1` 已将该页口径改回隐藏内置/系统运行组件。移动壳主体改为顶部对齐，避免切换搜索/服务器模组时顶部模块上下跳动。
- 验证：`cd frontend; npx tsc --noEmit -p .` 通过；`cd frontend; npm run build` 通过（仅 Vite 主 chunk 大小提示）。

# 2026-07-10 已完成：手机端卡片与底部 Tab 视觉统一优化 M6

- `MOBILE-VISUAL-UNIFY-M6-1` completed：手机端所有主要卡片（总览/控制/玩家/存档各页面的 `.sd-panel` 卡片）从直角方块升级为和 PC 总览页一致的圆角羊皮纸卡片。背景色取自 PC 总览页"存档/模组"指标卡（`linear-gradient(180deg, rgba(255,245,214,0.96), rgba(248,226,174,0.94)), #f7e3ad`），圆角/边框/阴影取自 PC 总览页"在线玩家"卡片（`border-radius:9px`、`border:2px solid #a06c2c`、三层 inset + drop shadow）。
- 实现方式：`StardewMobileShell.css` 新增 `:root` 变量 `--stardew-mobile-card-bg/border/radius/shadow`，配合 `.sd-mshell .sd-panel` 祖先限定选择器（优先级 (0,2,0) 稳赢全局单类 `.sd-panel`），一条规则覆盖所有手机端页面内的 `.sd-panel` 卡片——不需要逐个页面改 className。作用域限定在 `.sd-mshell` 内，桌面端 `StardewPanel.tsx`、登录页 `.sd-auth-shell` 不受影响。
- 玩家页内每张玩家行卡片（`.sd-mplay-player-card`）也同步引用同一组变量，从 `border:1px solid #dcc898; border-radius:2px; background:rgba(...)` 升级为和外层卡片一致的圆角羊皮纸样式。
- 底部 Tab 栏全面重做：从贴底直角硬条改为悬浮圆润胶囊式导航条（`border-radius:20px`、两侧各留 `10px` 间距、`bottom:10px + safe-area-inset-bottom`），每个 Tab 按钮变成垂直图标+文字的圆角 pill（`border-radius:14px`、`min-height:48px` 满足触控热区）；新增从桌面导航共用的像素图标（5 个 tab 各对应一个 image2 nav icon）；active 态 `background:var(--sd-green-bg)` 绿色高亮；`:active` 有 `scale(0.92)` 按压反馈；文案用 `text-overflow:ellipsis` 防溢出。
- `.sd-mshell` 的 `padding-bottom` 从 `56px + safe-area-inset-bottom` 提升到 `84px + safe-area-inset-bottom`，匹配新底栏更大的浮动占位。
- 未改动任何 TSX 业务逻辑（hooks/store/API/权限判断），仅在 `StardewMobileShell.tsx` 给 `MOBILE_TABS` 补了 `icon` 字段和 Tab 按钮内部的 `<img>` + `<span>` 结构。
- 影响文件：`frontend/src/games/stardew/StardewMobileShell.css`、`frontend/src/games/stardew/StardewMobileShell.tsx`、`frontend/src/games/stardew/mobile/MobilePlayersPage.css`。
- 验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过；构建产物 CSS 确认 `.sd-mshell .sd-panel` 规则生效、tabbar `border-radius:20px` 规则生效。**未做真实浏览器/真机视觉验证**：建议下一位维护者 `npm run dev -- --host 0.0.0.0` 在手机访问 `http://192.168.0.5:5173/`，或 `qa-layout.html?shell=mobile&state=running` 在浏览器缩放到 390×844/393×852/430×932 确认：卡片圆角明显、底栏圆润浮动、Tab 图标和文字间距合理、active 态清晰、无横向滚动、登录页不受影响。

# 2026-07-10 已完成：移动端玩家页 M4

- `MOBILE-PLAYERS-M4-1` completed：`StardewMobileShell` 的“玩家”Tab 从占位卡换成真实页面 `frontend/src/games/stardew/mobile/MobilePlayersPage.tsx`。**最终结构**（经用户两轮反馈调整后）：页面只有一张“在线玩家”卡，卡片头部右上角是刷新按钮，下方是玩家卡片列表——每张卡片姓名+状态徽章（在线/等待/离线/未知）、主机/角色标签、最近活动文案、底部左侧位置信息 + 右侧“踢出”“封禁”按钮，空态显示“暂无在线玩家”。**不含**顶部统计卡和独立的“待授权玩家”同意/拒绝卡片（用户明确要求删除，待认证玩家的同意/拒绝能力保留在“总览”Tab 的 `MobileHomePage.tsx`）。
- 全部复用既有接口，未新增后端 API：`kickPlayer`/`banPlayer`/`dashboardData.refreshPlayers`，`disabled`/`title` 门控逐条对齐桌面 `PlayersPage.tsx` 行内图标按钮。
- 忙碌态精确到单个玩家（`kickBusyId`/`banBusyId` 存目标 ID 而非单一 boolean），操作中该玩家按钮显示“处理中…”，同时锁定其它玩家行的踢出/封禁按钮避免并发误触。
- 桌面端 `PlayersPage.tsx`/`StardewPanel.css` 未改动；新增样式 `frontend/src/games/stardew/mobile/MobilePlayersPage.css`（class 前缀 `sd-mplay-`），只用全局 `stardew-theme.css` 工具类。
- 详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `MOBILE-PLAYERS-M4-1` 小节（含两轮反馈调整的完整记录）。验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做真实浏览器/真机视觉验证**：当前环境没有可用的浏览器自动化/截图工具，建议下一位维护者用 `npm run dev -- --host 0.0.0.0` 或 `qa-layout.html?shell=mobile&state=running` 在 390×844/393×852/430×932 下确认无横向溢出、卡片底部“位置信息+操作按钮”一行不挤压文字、真实连一个运行实例走一遍踢出/封禁完整链路。

# 2026-07-10 已完成：移动端控制页 M3

- `MOBILE-CONTROL-M3-1` completed：`StardewMobileShell` 的“控制”Tab 从占位卡换成真实页面 `frontend/src/games/stardew/mobile/MobileControlPage.tsx`，按用户口径限定为桌面 `ServerControlPage.tsx` 的“全服消息”卡片 + “快捷操作”卡片（去掉手动备份和 VNC 显示相关按钮），不含生命周期启停（已在 `MobileHomePage` 的“快捷控制”卡提供）。快捷操作按单列纵向按钮列表排布 5 个操作：计划重启、服务器密码设置、小屋与联机高级设置、触发节日活动、永久启用 Joja 路线，4 个表单类操作各自弹出全屏弹窗（`max-height:88vh; overflow-y:auto` 防止内容溢出小屏视口）。
- 全部复用既有接口，未新增后端 API：`getRestartSchedule`/`updateRestartSchedule`/`getInstanceServerPassword`/`updateInstanceServerPassword`/`getInstancePasswordStatus`/`getInstanceServerRuntimeSettings`/`updateInstanceServerRuntimeSettings`/`triggerFestivalEvent`/`enableJojaRoute`/`sendSay`。未直接复用 `ServerControlPage.tsx` 组件或它挂在 `StardewPanel.css` 里的类名（移动端不加载该文件），只搬了状态判断逻辑，弹窗/按钮/提示条按移动端布局重排，视觉基础件继续用全局 `stardew-theme.css` 的 `.sd-panel`/`.sd-input`/`.sd-btn-*`/`.sd-notice--*`。
- 顺手清理：全服消息卡片里过时的“该命令当前版本可能返回‘命令不支持’”提示文案（PC 端 `ServerControlPage.tsx` 和移动端一起删除，SMAPI say 命令现已正常支持）；移动端“发送”按钮按用户要求从 `sd-btn-green` 改成 `sd-btn-restart`（棕色，和重启按钮同色），PC 端保持 `sd-btn-green` 不变，两端故意区分颜色。
- 桌面端 `ServerControlPage.tsx` 行为视觉不变（除上述文案删除）；新增样式 `frontend/src/games/stardew/mobile/MobileControlPage.css`（class 前缀 `sd-mctrl-`）。
- 详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `MOBILE-CONTROL-M3-1` 小节。验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做真实浏览器/真机视觉验证**：当前环境没有可用的浏览器自动化/截图工具，建议下一位维护者用 `npm run dev -- --host 0.0.0.0` 或 `qa-layout.html?shell=mobile&state=running` 在 390×844/393×852/430×932 下确认无横向溢出、5 个快捷操作按钮和 4 个弹窗渲染正常（尤其计划重启弹窗字段最多，需确认可纵向滚动到底部按钮）、非管理员禁用态。

# 2026-07-10 已完成：移动端总览页 M2

- `MOBILE-HOME-M2-1` completed：`StardewMobileShell` 的“总览”Tab 从 M0 的静态占位卡换成真实页面 `frontend/src/games/stardew/mobile/MobileHomePage.tsx`（新目录 `mobile/`，为后续 M3 移动端页面预留位置），按单列卡片流展示：①状态摘要卡（存档名/服务器状态/在线玩家/版本，状态文案区分“启动中/停止中”）；②邀请信息卡（邀请码 + 局域网邀请地址，各带复制按钮，长文本用等宽小字 + `break-all` 防撑破卡片）；③快捷控制卡（启动/停止/重启，按钮 `min-height:44px`，停止/重启带确认弹窗）；④待认证玩家批准卡（复用密码认证桥接批准能力）。
- 全部复用既有数据与接口，未新增后端 API：`useStardewDashboardData()` 的 `instanceState`/`saves`/`players`/`jobs`/`versionInfo`/`inviteCode`/`publicIP` 及其刷新函数；生命周期 `startInstance`/`stopInstance`/`restartInstance`；玩家页已有的 `getInstancePasswordStatus`/`approvePlayerAuth`。邀请码/局域网地址复制逻辑参照 `InviteCodeCard.tsx` 重写为移动端轻量展示（未复制其 API 请求逻辑，只读同一份 `dashboardData` 字段）。
- 桌面端 `OverviewPage.tsx`/`StardewPanel.css` 未改动；样式新增 `frontend/src/games/stardew/mobile/MobileHomePage.css`（class 前缀 `sd-mhome-`），按钮沿用 `stardew-theme.css` 全局 `.sd-btn-start`/`.sd-btn-stop`/`.sd-btn-restart`/`.sd-btn-green`/`.sd-btn-tan`/`.sd-btn-delete`/`.sd-panel`/`.sd-notice--*`/`.sd-tag` 等既有工具类（这些类在 `main.tsx` 全局加载，桌面专属的 `StardewPanel.css` 才不可用），只用 `min-height`/`width` 覆盖尺寸做触控热区放大，不重画按钮贴图。`StardewMobileShell` 新增接收 `user` prop（M0 遗留的“暂无 user”限制在本轮补上，`App.tsx`/`qa-layout-main.tsx` 同步传入）。
- 详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `MOBILE-HOME-M2-1` 小节。验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做真实浏览器/真机视觉验证**：当前环境没有可用的浏览器自动化工具，建议下一位维护者用 `npm run dev -- --host 0.0.0.0` 或 `qa-layout.html?shell=mobile` 在 390×844/393×852/430×932 下确认无横向滚动、四张卡片渲染正常、复制按钮反馈、启停按钮状态刷新、非管理员禁用态。

# 2026-07-10 已完成：修复登录页移动端布局崩坏

- `LOGIN-MOBILE-FIX-1` completed：用户提供真机截图反馈登录页在手机浏览器里布局崩坏（标题横条压住顶部鸡形徽标、卡片被横向撑爆、软键盘弹出后登录按钮被遮挡），且"没有输入法的时候也有突出，只是没有这么严重"。根因是登录/初始化页永远使用的 `.sd-auth-shell--image-login` 版式把整张桌面原型图当卡片背景，用固定 16:9 比例反算出 `position:absolute` 大盒子并用百分比坐标摆放输入框；手机竖屏宽高比和 16:9 差异很大，算出来的盒子宽度会远超视口（390px 宽视口下接近 1500px），被外层 `overflow:hidden` 裁掉，且已有的 `@media(max-width:700px)` 手工坐标补丁是按单一机型手算的偏移量，换一个宽高比就会跟着错位。
- 只改了 `frontend/src/App.css` 一个文件：新增 `@media (max-width:768px)` 覆盖，作用域限定在 `.sd-auth-shell`/`.sd-auth-shell--image-login` 及其子选择器，`<=768px` 时整体放弃绝对坐标定位，改回真实文档流的羊皮纸卡片（`width:min(100% - 24px, 420px)`，`min-height:100dvh` 处理地址栏收缩，`overflow-y:auto` 允许纵向滚动，输入框 `min-height:40px`/字号锁 16px 防 iOS 自动缩放，按钮 `min-height:44px`）。卡片装饰复用现有 `background_parchment_tile.png`/`button_primary_small_green_blank.png`；背景第一版复用 `background_login_home_image2.png`（桌面那张画死了假窗口 UI 的原图）铺满裁切，用户反馈"背景还是 PC 端的登陆窗口，很违和"后改用 `background_login_farm_generated.png`（非 image2 版登录页用的纯像素农场背景，没有假 UI 元素，设计上就是给"背景 + 悬浮真实卡片"用的）。三张都是仓库已有素材，未新增或替换任何图片；未改 API、session、权限或用户初始化逻辑；未改 `MOBILE-SHELL-M0-1` 的 `StardewMobileShell`（两者完全独立，作用域不重叠）。
- 用户看了背景返工后的效果又反馈卡片本体"太丑"，想要接近 PC 端右侧木框招牌的质感。没有去裁桌面大图里的招牌区域当卡片背景（那样等于把坐标错位的老问题换个更小的坐标系重演一遍），而是给卡片加了一枚固定像素尺寸的鸡形徽标（复用顶栏已有的 `icon_topbar_chicken_image2_v2.png`）骑在卡片顶部边框上，配合加深的投影和调整过的内边距，呼应桌面招牌"鸡+木框"的视觉但不依赖任何坐标换算。
- 详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `LOGIN-MOBILE-FIX-1` 小节（含桌面版坐标选择器 specificity 的踩坑记录、背景素材两轮返工原因）。验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过；`npm run dev -- --host 0.0.0.0` 已启动供真机复测。**未做真机复测确认**：还需要在真机上刷新页面确认标题横条不再压住图标、卡片不再横向撑爆、背景不再显示违和的桌面窗口、卡片顶部鸡形徽标视觉效果、软键盘弹出后登录按钮可滚动到可见范围。

# 2026-07-10 已完成：移动端基础入口 M0

- `MOBILE-SHELL-M0-1` completed：在同一前端工程内新增移动端入口，未新建独立项目。新增通用 `frontend/src/hooks/useMediaQuery.ts` hook；`App.tsx` 用 `useMediaQuery('(max-width: 768px)')` 在进入 Stardew 面板处分流，`<=768px` 渲染新增占位组件 `frontend/src/games/stardew/StardewMobileShell.tsx`（顶部品牌 + 状态徽章、复用 `.sd-panel` 羊皮纸样式的“移动端面板建设中”占位卡、底部 5 个静态 Tab：总览/控制/玩家/任务/更多），桌面端继续渲染既有 `StardewPanel`，行为视觉不变。
- M0 不做真实路由/分页、不改后端、不新增 API、不改登录和权限逻辑；`StardewMobileShell` 不接收 `user`/`onLogout`，暂无登出入口，留给 M1。样式独立在 `StardewMobileShell.css`（`sd-mshell-` 前缀），只复用 `stardew-theme.css` 已有变量和工具类，未新增图片素材、未引入新 UI 库。
- QA mock 入口 `frontend/qa-layout.html` 新增 `?shell=mobile` 参数，可配合既有 `?state=running/stopped` 用 mock 数据回归移动端占位壳，不用连真实后端。
- 详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `MOBILE-SHELL-M0-1` 小节（含 M1 注意事项）。验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做真实浏览器缩放/移动设备实测**，当前环境没有可用的浏览器自动化/截图工具，建议下一位维护者用 `qa-layout.html?shell=mobile` 在真实浏览器里确认 768px/390px/320px 下无横向滚动、底部 Tab 贴底、状态徽章和 Tab 高亮切换正常。

# 2026-07-09 已完成：用户管理新增"重置密码"，修正权限规则

- `USER-PASSWORD-RESET-1` completed：面板登录账号（注意不是游戏加入密码）此前完全没有改密码的入口——后端 `PATCH /api/users/{id}` 早就支持 `password` 字段，但从未在前端暴露，而且原有权限检查比预期更严格（连管理员改自己密码都会被拒绝）。按用户明确规则实现：普通用户不能改自己密码（无入口，接口本身就是 `requireAdmin`）；普通管理员能改自己的和普通用户的密码，不能改其他管理员的；第一个注册的超级管理员能改所有人的密码，包括自己。后端 `storage.UpdateUser` 权限检查补一个"改自己"豁免条件；前端"设置"页用户列表新增"重置密码"按钮和弹窗，改自己密码后自动跳转登录页（当前 session 会被撤销）。新增测试 `TestPasswordChangePermissions` 覆盖五种权限场景。详见 `docs/backend-handoff/backend-handoff-2026-07-09.md`、`docs/frontend-handoff/frontend-handoff-2026-07-09.md` 的 `USER-PASSWORD-RESET-1`，以及 `website/docs/handbook/accounts.md` 补充的重置密码说明。验证：`cd backend; go build ./... && go test ./...` 全绿；`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。

# 2026-07-09 已完成：启动/停止/重启按钮补齐管理员权限门控

- `ADMIN-GATE-LIFECYCLE-1` completed：用户凭经验提出"普通用户不能启停服务器/上传 Mod"要求复查，全量排查发现 `ServerControlPage.tsx`（`canStart`/`canStop`/`canRestart` 未算 `isAdmin`）和 `OverviewPage.tsx`（组件根本没取 `user` prop，无 `isAdmin` 概念）的启动/停止/重启按钮缺少管理员权限门控——后端 `requireAdmin` 仍会拒绝，但普通用户会看到"可点击"的按钮，点了才被拒绝，体验不一致。两处均已补上 `isAdmin` 判断和对应 tooltip 提示。复查同时确认 Mod 上传、Nexus Mod 一键安装、游戏安装、玩家踢出、存档、设置、任务日志、诊断导出等其余功能本来就正确限制了管理员权限，未发现其它遗漏。详见 `docs/frontend-handoff/frontend-handoff-2026-07-09.md` 的 `ADMIN-GATE-LIFECYCLE-1`。验证：`cd frontend; npx tsc --noEmit -p . && npm run build` 通过；未做非管理员账号的浏览器实测。

# 2026-07-09 已完成：踢出玩家 + 服务器密码设置/认证状态查询

- `PLAYERS-KICK-1` completed：玩家页踢出功能（行内图标按钮 + "管理操作"卡片下拉+按钮）不再是禁用占位，接入真实后端。上游 JunimoServer 没有踢人 REST API，改为复用面板自带 `StardewAnxiPanel.Control` SMAPI Mod 已有的命令队列机制（和喊话 `say` 同一套通道）：面板写命令文件到 `control/commands/`，Mod 每 120 tick 消费一次，按 `UniqueMultiplayerID` 查找在线玩家并调用 `Game1.server.kick(...)`，禁止踢主机。**已重新编译并替换嵌入的 `StardewAnxiPanel.Control.dll`**，影响现有实例需要重启/重新准备 server 容器才能刷新到最新 Mod。fire-and-forget，前端只能提示"指令已提交"，拿不到精确执行结果。
- `PASSWORD-STATUS-1` completed：`ServerControlPage.tsx` 原"服务器设置"占位按钮按用户要求改名为"服务器密码设置"并接入弹窗（不是新建 `SettingsPage.tsx` 区块）。弹窗内可读写 `.env` 的 `SERVER_PASSWORD`（JunimoServer 只支持容器启动时生效，保存后提示需重启 server 容器），并只读展示 JunimoServer `GET /auth` 代理出的密码保护状态（是否启用、已认证/待认证人数、认证超时秒数、最大失败次数）。
- 详见 `docs/backend-handoff/backend-handoff-2026-07-09.md`、`docs/frontend-handoff/frontend-handoff-2026-07-09.md` 的对应小节。验证：`cd backend; go build ./... && go test ./...` 全绿；SMAPI Mod 用 Docker 官方命令重新编译 `Build succeeded, 0 Errors`；`cd frontend; npx tsc --noEmit -p . && npm run build` 通过。**未做真机联机验证和浏览器截图验证**，建议下一位维护者找测试实例走一遍"设密码→重启→玩家登录→踢人→查看认证状态"完整链路。
- 封禁玩家、白名单管理、权限设置三项仍是禁用占位，未在本轮改动范围内。

# 2026-07-09 已完成：手机端适配新一轮系统性修复

- `FE-MOBILE-FIXES-1` completed：不重构现有断点体系，针对调研定位到的具体移动端问题逐项修复：`.sd-input`（含 `.sd-mods-sync-select`、`.sd-players-action-select`）在 `max-width: 640px` 下字号提到 16px，避免 iOS Safari 聚焦表单自动放大整页；`viewport` meta 补 `viewport-fit=cover`，`#root:has(.sd-shell)` 补 `env(safe-area-inset-*)` 内边距；移动端顶部横向导航图标从 42×38px 提到 44×44px 触控热区（`.sd-shell` 第二行同步从 48px 调到 54px）；`.sd-confirm-dialog` 补 `max-height:90vh; overflow-y:auto` 避免长文案/矮视口溢出；Players 表格与存档备份表格的横向滚动容器补惯性滚动和右侧渐变提示，让移动端用户能发现可横滑。详见 `docs/frontend-handoff/frontend-handoff-2026-07-09.md`。
- 中期路线里的“更完整的移动端导航和表格卡片化”（见下文中期路线）不在本轮范围内，仍待后续单独排期。

# 2026-07-08 已完成：run.sh Docker APT 源同步失败兜底

- `RUN-SH-DOCKER-APT-FALLBACK-1` completed：修复一键脚本在 Ubuntu/Debian 安装 Docker 时只依赖阿里云 Docker CE apt 源的问题。遇到 `File has unexpected size ... Mirror sync in progress?` 这类镜像站同步期索引校验失败时，脚本会先停用系统里历史残留的 Docker apt 源并清理索引，再自动切换到阿里云、清华 TUNA、中科大 USTC、Docker 官方源继续重试。真实现场补充：阿里云 ECS 可能在 `/etc/apt/sources.list` 或其它源文件内残留 `http://mirrors.cloud.aliyuncs.com/docker-ce/...`，新脚本会注释这类旧源，只保留当前托管源 `/etc/apt/sources.list.d/anxi-panel-docker.list`。仅改部署脚本与镜像构建文档，未改面板运行镜像、后端 API 或前端逻辑。

# 2026-07-07 已完成：文档门户网站骨架上线（VitePress + GitHub Pages）

- `DOCS-PORTAL-1` completed：`website/` 下手动搭建 VitePress 骨架（`npm create vitepress@latest` 实测解析到无关第三方包 `create-vitepress@0.0.6`，改用 `npm init` + `npm install -D vitepress` 手动创建），新增 `.github/workflows/docs.yml`（push `website/**` 到 `main` 时自动 `docs:build` 并部署到 GitHub Pages），用 `gh api repos/.../pages -f build_type=workflow` 开通 Pages（Source: GitHub Actions）。已推送并验证线上首页 `https://anxiyizhi.github.io/stardew-server-anxi-panel/` 返回 200。
- `DOCS-PORTAL-2` completed：内容迁移（方案第三节映射表）已完成，`guide/`（2 页）、`deploy/`（4 页）、`maintain/`（4 页，比原方案多一页 `admin.md`）、`faq/`（1 页）共 11 个内容页从 `README.md` / `docs/user-guide/` 搬运改写完毕，`npm run docs:build` 验证无死链。待办：推送到 `main` 触发线上部署（当前线上仍是占位首页）。详见 `docs/11-docs-portal.md`。

# 2026-07-07 已完成：Nexus ZIP 下载断点续传与卡死检测

- `NEXUS-ARCHIVE-RESUME-1` completed：`mod_remote_install` / `mod_nexus_install` 的 ZIP body 下载窗口提升到 20 分钟，远程/Nexus Mod 安装 job 整体窗口确认为 30 分钟；下载阶段新增 `.part` 临时文件和 `Range` 断点续传，服务器支持 `206 Content-Range` 时从已下载字节继续，忽略 Range 返回 `200` 时自动丢弃半包并重下；新增 120 秒无新字节超时，连接卡死会取消当前 attempt 并在 20 分钟窗口内最多重试 4 次。验证：`cd backend; go test ./internal/games/stardew_junimo -run "NexusDownloadArchive"`；`cd backend; go test ./internal/games/stardew_junimo ./internal/web`。

# 2026-07-07 已完成：远程 Mod 重复安装幂等 + 批量失败定位

- `MOD-REMOTE-IDEMPOTENT-1` / `FE-MOD-BATCH-ERROR-FOCUS-1` completed：`mod_remote_install` / `mod_nexus_install` 下载类任务遇到已存在 `UniqueID` 时改为跳过重复目录并成功结束，避免浏览器扩展缓存刷新造成的重复提交把“已经装好”的 Mod 误判为失败；普通手动上传仍保留 `400 mod_exists`。批量进度按钮会标明失败的具体 Mod，带 `jobId` 时点击跳转任务与日志；同时用最新 `GET /mods` 兜底校正旧失败 job 中其实已安装的项。新建 Mod 下载类任务展示名改为 `Mod 名 · 任务类型`，例如 `Ridgeside Village · mod_remote_install`。验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`、`cd frontend; npm.cmd run build`。

# 2026-07-07 已完成：修复 mods.go / lifecycle_handlers.go 历史乱码 + 备份分类 bug

- `MOJIBAKE-FIX-1` completed：修复 `mods.go` 和 `lifecycle_handlers.go` 里历史遗留的中文乱码（错误提示、注释），根因是早期某次保存把正确 UTF-8 中文按 GBK 误解码又存回 UTF-8，已用确定性可逆公式全量修复。顺带修复真实功能 bug：备份删除/恢复接口原本因乱码字符串匹配失败，`400`/`404`/`409` 分类全部静默退化成通用 `500`，现已修正。已验证 `go build`、`go vet`、`go test ./...` 全绿，并对 backend 全目录做过乱码特征扫描确认清零。

# 2026-07-07 已完成：Nexus 浏览器扩展适配 Shadow DOM 下载入口

- `NEXUS-MODPAGE-DL-2` completed：扩展 `0.1.1 → 0.1.2`，新增 `deepQueryAll()` 遍历 shadow root 定位下载控件，新增按 `data-tracking` 属性分类的 `findManualDownloadControl()`，新增 `openNexusFileList()`/`waitForFileIdOnPage()` 轮询 `file_id`，替代旧的两步按钮点击模型。仅改浏览器扩展，未改后端接口；发布新镜像后旧实例扩展缓存 ZIP 会被已有版本感知逻辑自动刷新。已验证 `node --check` 和 `go test ./internal/games/stardew_junimo -run TestEnsureNexusInstallerExtensionZip`。

# 2026-07-07 已完成：JunimoServer 静态 init 文件兼容挂载

- `JUNIMO-STATIC-INIT-FIX-1` completed：为 server 容器新增 `.local-container/cont-env/*`、`.local-container/cont-groups/*`、`.local-container/cont-users/*` 兼容挂载，批量遮罩上游 `sdvd/server:1.5.0-preview.121` 中裸静态值被 init 当命令执行的问题。
- 已覆盖 `APP_NAME`、`DBUS_SESSION_BUS_ADDRESS`、`DOCKER_IMAGE_PLATFORM`、XDG 路径、用户/组 id 等真实飞牛现场触发项；旧实例会自动迁移 compose，重启时如本次刚新增挂载会用 `docker compose up` 重建 server 容器。
- 验证：`cd backend; go test ./internal/games/stardew_junimo`；真机热修后 `stardew-server-1` healthy，Junimo API `/health` 返回 ok。

# 2026-07-07 已完成：Nexus 搜索防短断与局域网邀请地址修正

- 已完成：Nexus 搜索后端改用独立 20 秒上下文，避免浏览器刷新、切页、FRP/NAS 链路短断时把上游 GraphQL 请求提前取消并误报 `nexus request failed`。
- 已完成：Nexus 网络类错误新增 `nexus_network_failed`，后端日志保留真实底层错误，前端展示明确网络提示。
- 已完成：“局域网邀请”改为读取当前进入面板的 host；用户用什么 IP/域名加 `:8090` 打开面板，就展示什么 host。

# 2026-07-07 已完成：steam-auth 授权标志收口

- 已完成：`steamAuthLoggedIn` 收口为邀请码卡主 UI 授权标志；在 steam-auth 登录成功日志（`[SteamAuth:*] Logged in as ...` / `[SteamService] ... Logged in as ...`）出现后写 `STEAM_AUTH_COMPLETED=true`，启动/手动刷新成功拿到非空邀请码时也写 true。
- 已完成：启动/重启后如果 server 日志明确出现 `no logged-in accounts`，后端会清空 `STEAM_AUTH_COMPLETED`，前端下一轮状态刷新后显示【登录授权】；`Steam-auth service not ready` 不直接清 false，已有 true 时自动刷新一次 `steam-auth` 服务。
- 已完成：邀请码卡不再用 `steamAuthReady=false` 直接显示重新授权；`steamAuthReady` 保留为诊断字段，主按钮只按 `steamAuthLoggedIn` 显示。
- 已完成：生命周期启动/重启不再等待邀请码，server running 后 job 完成；后端后台最多探测 20 次邀请码，成功后写 `STEAM_AUTH_COMPLETED=true` 与 `/state.inviteCode`，失败不影响服务器和 IP 直连。前端启动中状态改按 active lifecycle job + running/stopping 状态，不再依赖邀请码、在线玩家或 SMAPI 存档加载日志。验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/games/stardew_junimo/config ./internal/web`、`cd backend; go test ./...`、`cd frontend; npm.cmd run build`。

# 2026-07-06 已完成：SteamCMD HOME 与缓存清理加固

- 已完成：SteamCMD fallback 以 `steam` 用户运行时显式设置 `HOME=/home/steam`，避免继续使用 `/root/.local/share/Steam` 自更新缓存引发 139 段错误。
- 已完成：Docker volume 清理改为逐个删除并忽略缺失卷，139 后自动清理 SteamCMD runtime cache 的成功率更高；真实失败会把 Docker 输出写入任务日志。
- 已完成：139 后清理 SteamCMD runtime cache 前，先按 volume 查找并删除残留 one-shot SteamCMD 容器，解决真实云服上 `volume is in use - [container_id]` 导致缓存无法清理的问题。

# 2026-07-06 已完成：SteamCMD SDK 分段下载

- 已完成：SteamCMD fallback 在同一个容器内拆成两次 SteamCMD 进程，分别下载/校验 `413150` 和 `1007`，避免 SDK 阶段因登录后切换 `force_install_dir` 触发 SteamCMD 139 段错误。
- 已完成：过滤旧 daocloud SteamCMD 候选，并在 SteamCMD exit code 139 时自动清理自更新缓存卷后重试一次。

# 2026-07-06 已完成：SDK 后置 SMAPI 预安装

- 已完成：安装流程在游戏文件与 Steam SDK 完成后，通过 JunimoServer 一次性容器预安装 SMAPI，减少 JunimoServer 首次启动时因 GitHub 下载 SMAPI 卡住的概率。
- 已完成：前端把 SMAPI 作为“下载游戏”步骤里的最后一个子状态展示，并区分 `smapi_install_failed` 后置失败。
- 后续可优化：如果后续维护自定义 JunimoServer 镜像，可考虑把 SMAPI 安装包内置进镜像，进一步减少现场 GitHub 下载依赖。

# FE-STEAM-GUARD-SUBMITTED-FEEDBACK-1 状态

- `FE-STEAM-GUARD-SUBMITTED-FEEDBACK-1` completed：Steam / SteamCMD 验证码提交成功后，安装页会显示“验证码已提交，正在等待响应”的本地等待态，不再回到同一个空输入框。等待态保留“重新输入”入口，并在 phase 进入下载、失败或完成后自动清除。

# STEAMCMD-EMAIL-GUARD-PROMPT-1 状态

- `STEAMCMD-EMAIL-GUARD-PROMPT-1` completed：SteamCMD 首次新机器登录时的邮箱 Steam Guard 分行提示已纳入后端和前端识别。后端会把 `This computer has not been authenticated...` / `Please check your email...` / `code from that message` / `set_steam_guard_code` 等日志切到 `steamcmd_guard_required`；前端也会在 job 日志先到时展示 SteamCMD 验证码输入框，避免安装页卡在下载/自更新进度。

# STEAMCMD-CREDENTIAL-REUSE-1 状态

- `STEAMCMD-CREDENTIAL-REUSE-1` completed：基于 SteamCMD 自身的缓存机制实现“一次批准、后续复用”，不共享 steam-auth token。首次完整登录成功后写 `STEAMCMD_AUTH_COMPLETED`；后续使用 `+login <username>` 与 `@NoPromptForPassword` 读取 SteamCMD 缓存，缓存明确失效时同一 job 自动回退完整登录。root/steam 候选镜像的 `Steam` 与 `.local/share/Steam` 统一映射到 `steamcmd-login`，139 重试不再删除授权卷；空统一卷会自动从旧 root/user local 卷迁移 `config/` 与 `ssfn*`。自动化测试通过；真实 Docker 中从旧 root-local 迁移后连续两个全新 SteamCMD 容器均命中 cached credentials、退出码 0、未再次触发 Steam Guard。

# INSTALL-ROUTING-SPLIT-1 状态
- `INSTALL-ROUTING-SPLIT-1` completed：安装流程按用户口述的完整期望重构。把 `reuseCredentials` 粗暴驱动的单一 `steamCMDRetry` 拆成 `reuse` / `steamCMDDirect` / `steamCMDUseCache` 三个正交决策，用已持久化的 driverPhase/state 判“是否已过认证”、用新 `.env` 标志 `STEAMCMD_AUTH_COMPLETED` 判“SteamCMD 是否有缓存”。修复：①镜像拉取失败重试重新拉镜像 + steam-auth（不再误跳 SteamCMD）；②SteamCMD 认证超时重试回到登录验证界面（不再秒报“授权缓存不可用”）；③认证成功后跨会话可靠跳过 steam-auth。并新增「更换 Steam 账号 / 强制重新认证」入口（`forceReauth`：清 steam-session/steamcmd 授权卷 + 重置标志，保留 game-data）；已安装态只保留卡片内换号按钮，操作区不再重复渲染。已验证 `cd backend; go test ./...`、`cd frontend; npm.cmd run build`。

# STEAMCMD-REPAIR-DIRECT-1 状态
- `STEAMCMD-REPAIR-DIRECT-1` completed：安装页“重新安装 / 修复”与复用凭据重试入口已改为直达 SteamCMD 下载/校验。前端不再要求输入 Steam/VNC 凭据；后端收到 `reuseCredentials=true` 后跳过 `steam-auth`，并用已有 SteamCMD 授权缓存执行 app_update。已验证 `cd backend; go test ./internal/games/stardew_junimo ./internal/web` 与 `cd frontend; npm.cmd run build`。

# FE-TOPBAR-BRAND-LIGHTER-2 状态
- `FE-TOPBAR-BRAND-LIGHTER-2` completed：Stardew Shell 左上角品牌标题已按“再细 200”反馈继续减重，`.sd-topbar-brand-text` 字重从 `700` 降到 `500`，暗色描边/投影不透明度同步降低；仅改前端 CSS，未改顶栏状态牌、存档/用户框、API、权限、路由或 Junimo 通信。验证：`cd frontend; npm.cmd run build`；Browser QA 标题 computed `fontWeight=500`，总览/服务器往返后仍为 `500`。

# FE-TOPBAR-BRAND-LIGHTER-1 状态
- `FE-TOPBAR-BRAND-LIGHTER-1` completed：Stardew Shell 左上角品牌标题已按用户反馈再调细，`.sd-topbar-brand-text` 字重从 `800` 降到 `700`，并减少暗色描边/投影层数；仅改前端 CSS，未改顶栏状态牌、存档/用户框、API、权限、路由或 Junimo 通信。验证：`cd frontend; npm.cmd run build`。

# FE-OVERVIEW-HEALTH-SHARE-1 状态
- `FE-OVERVIEW-HEALTH-SHARE-1` completed：诊断页成功执行健康检查后会把结果同步到公共 dashboard 数据层，用户从诊断页返回总览页时“系统健康”卡会显示最新评分和状态，不再保持 `— / 未检查`。普通 dashboard 初始化仍不主动调用 `/api/health/diagnostics`，保留诊断按需触发策略。验证：`cd frontend; npm.cmd run build`；Browser QA 通过总览 `—` -> 诊断页 6 项正常 -> 回总览 `100% / 6项全部通过 / 优秀`。

# PUBLIC-IP-LOOKUP-1 状态
- `PUBLIC-IP-LOOKUP-1` completed：新增 `GET /api/instances/:id/public-ip`，由面板后端检测服务器公网出口 IP，返回 `{ ip, checkedAt, source?, cached }`，成功结果缓存 `10min`，前端手动刷新会强制重新探测。邀请卡片已在邀请码下方新增公网 IP 框和复制/刷新按钮，上方标题保持“邀请码”，下方标题显示“局域网邀请”，公网 IP 失败态不显示复制按钮但两行值框保持同宽，并按用户反馈移除邀请码说明文字。未改 Junimo driver、Docker/Compose、实例状态或邀请码接口。验证：`cd backend; go test ./internal/web`，`cd frontend; npm.cmd run build`。

# FE-MOD-COUNT-FILTER-BUILTIN-1 状态
- `FE-MOD-COUNT-FILTER-BUILTIN-1` completed：总览页模组统计已过滤 SMAPI runtime、`StardewAnxiPanel.Control`、`JunimoServer` / `JunimoHost.Server` 三类内置运行组件；模组统计卡、`已启用 N 个` 和同步包摘要里的已启用/已停用数量均按用户可见 Mod 计算。系统运行组件识别已抽为 `mod-visibility.ts`，由总览页和模组页共用。仅改前端展示口径，未改后端 API、启用切换接口、同步包导出或 Junimo 通信。验证：`cd frontend; npm.cmd run build`。

# DOCKER-POLL-PERF-1 状态

- `DOCKER-POLL-PERF-1` completed：`ComposePs` 已增加默认 `1.5s` 短 TTL 成功结果缓存，并在 `compose up/down/restart` 前后失效，减少状态页、诊断和支持包短时间重复触发 Docker CLI 的开销。
- 前端已停止普通 dashboard 初始化里的健康诊断请求，也移除右侧栏 `/metrics` 常驻轮询；资源指标只在诊断页可见时按 `8s` 间隔采样，后台 tab 隐藏时暂停。
- `DockerVersion` / `ComposeVersion` 保持在 Diagnostics、Docker 状态页、安装前检查或用户手动刷新路径，不进入普通总览轮询。验证：`cd backend; go test -count=1 ./internal/docker`，`cd frontend; npm.cmd run build`。

# ASSET-RUNTIME-SLIM-1 状态
- `ASSET-RUNTIME-SLIM-1` completed：浏览器扩展安装包已从 runtime `zip -r` 改为 `extension-builder` 构建阶段产物，最终 Alpine runtime 不再安装 `zip`；`docs/prototypes` 已从 109 个历史截图/提取文件收敛为轻量索引和 2 张关键总览基准图，完整原型截图改由外部制品承接；超过 300 KB 的运行 PNG 已做无损重压缩并通过像素等价校验；登录背景因色调变化已回退为 PNG-only；favicon 已改为 `.ico` 加 32/64/128 PNG。

# SUPPORT-BUNDLE-STREAM-1 状态

- `SUPPORT-BUNDLE-STREAM-1` completed：支持包导出已改为直接对 HTTP 响应流写 ZIP，不再用内存 `bytes.Buffer` 聚合整个压缩包后一次性返回。下载接口、文件名和 ZIP 条目保持不变；响应不再设置 `Content-Length`，前端应按 Blob 下载处理。已验证 `cd backend; go test ./internal/web -run "SupportBundle|Docker|Metrics"` 与 `cd backend; go test ./...`。

# FE-CLEANUP-UNUSED-ASSETS-1 状态
- `FE-CLEANUP-UNUSED-ASSETS-1` completed：前端生产素材已清理 79 个源码零引用旧 PNG，主要是旧右栏整图、旧顶栏三段、旧导航/字段/图标 sheet 与早期装饰 sprite；`frontend/public/assets` 从约 39.52MB 降到约 18.56MB。同步删除无引用组件 `CommandOutput`、`StatusPill`、`StatusBadge`、`InstanceStateCard`。未改业务逻辑、API、路由或 Junimo 通信；保留动态路径使用的 `new-game` 素材和 QA 入口。已验证素材复扫无非 `new-game` 零引用文件，`cd frontend; npm.cmd run build` 通过。

# FE-MODS-HIDE-SYSTEM-RUNTIME-1 状态
- `FE-MODS-HIDE-SYSTEM-RUNTIME-1` completed：模组页已隐藏 SMAPI runtime、`StardewAnxiPanel.Control` 和 `JunimoServer` / `JunimoHost.Server` 这类系统运行组件；它们不再出现在“添加模组”的已安装卡片或“配置模组”的当前存档启用状态列表中。用户可见“已安装”和解析失败统计同步只计算普通 Mod；玩家同步统计和导出仍保留完整列表逻辑，避免影响基础运行依赖处理。仅改前端 `ModsPage.tsx` 和文档，未改后端 API、启用切换接口、玩家同步包导出或 Junimo 通信。验证：`cd frontend; npm.cmd run build`。

# JUNIMO-MOD-MOUNT-RESTORE-1 状态
- `JUNIMO-MOD-MOUNT-RESTORE-1` completed：启动/重启前自动从当前 `sdvd/server` 镜像同步官方 `JunimoServer` Mod 到宿主 `.local-container/mods`，修复 mods 挂载遮住镜像内置组件后 Junimo API/VNC rendering 不可用的问题。`JunimoHost.Server` 现在是内置强制启用组件，物理 `smapi` 运行时目录不再显示为重复 Mod。验证：`go test ./internal/games/stardew_junimo -run "ListMods|ApplyNewSaveDefault|ApplyModProfileKeeps|Rendering|JunimoServerMod"`、`go test ./internal/web -run "Rendering|VNCConfig"`。

# ENV-BOM-NORMALIZE-1 状态
- `ENV-BOM-NORMALIZE-1` completed：实例 `.env` 读取/写回会剥离 UTF-8 BOM 前缀 key，修复 `﻿IMAGE_VERSION` 导致 `docker compose up` 报 `unexpected character "\ufeff"` 的启动失败。当前 `data/instances/stardew/.env` 已热修，`docker compose config --quiet` 和 `docker compose up -d` 已验证通过；后续排查启动失败时应先区分 Compose 配置解析错误和容器进程错误。

# STEAMCMD-SELFUPDATE-CACHE-1 状态
- `STEAMCMD-SELFUPDATE-CACHE-1` completed：SteamCMD 兜底容器已持久化 `/root/.local/share/Steam` 与 `/home/steam/.local/share/Steam`，用于缓存 SteamCMD 客户端自更新文件；前端将登录前 `[steamcmd] [ N%] Downloading update (... of 40,273 KB)` 标记为 SteamCMD 客户端更新，不再误显示为镜像拉取或游戏文件下载。验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallSteamCMD"`，`cd frontend; npm.cmd run build`。

# STEAMCMD-RETRY-RESUME-1 状态
- `STEAMCMD-RETRY-RESUME-1` completed：SteamCMD 兜底授权超时/失败后，复用凭据重试会直接恢复 SteamCMD fallback，不再先跑 Junimo compose pull 或 `steam-auth`；SteamCMD 镜像候选会先全量 inspect，本地已有任意候选即直接使用，避免重复拉取已下载镜像。前端安装页已补充“重试 SteamCMD 授权/下载”和“不重新拉取已有 SteamCMD 镜像”的提示。验证：`cd backend; go test ./internal/games/stardew_junimo -run "SteamCMD|InstallFallsBack|InstallResumes|InstallUsesExistingLater|InstallSteamCMD"`，`cd frontend; npm.cmd run build`。

# STEAMCMD-FALLBACK-1 状态

- `STEAMCMD-FALLBACK-1` completed：安装流程在 `steam-auth` 已认证成功但游戏文件下载失败时，会自动切换到 SteamCMD 兜底下载，复用已保存 Steam 账号密码并持久化 SteamCMD 授权缓存；需要验证时前端展示 SteamCMD 专属“手机 App 批准 / App 或邮箱验证码”选项。新增 `steamcmd_*` 安装 phase、Docker 通用 TTY 容器执行和 SteamCMD 镜像拉取能力；SteamCMD 镜像按 `STEAMCMD_IMAGE_CANDIDATES` 候选列表兜底，默认顺序为 `docker.1ms.run/steamcmd/steamcmd:latest`、`docker.m.daocloud.io/steamcmd/steamcmd:latest`、`ghcr.io/steamcmd/steamcmd:latest`、`cm2network/steamcmd:latest`，旧实例会补齐新候选并过滤直连 Docker Hub 的 `steamcmd/steamcmd:latest` 和已移除的 `docker.xuanyuan.me/steamcmd/steamcmd:latest`，单次 pull 默认等待 30 分钟，避免单个镜像源 403 或慢速拉取后直接失败。已验证 `cd backend; go test ./internal/docker ./internal/web ./internal/games/stardew_junimo` 与 `cd frontend; npm.cmd run build` 通过。

# FE-STEAM-AUTH-DOWNLOAD-PROGRESS-RESTORE-1 状态
- `FE-STEAM-AUTH-DOWNLOAD-PROGRESS-RESTORE-1` completed：安装页 Steam 认证区现在按最新日志识别下载阶段，日志出现 `Downloading app 413150` / `Progress:` 后会显示游戏下载卡和真实文件/体积进度条，不再停留在手机 App 批准登录；历史 QR URL 也不再压过后续 Guard 验证码输入。仅改 `frontend/src/games/stardew/pages/InstallPage.tsx`，未改后端接口、SSE 或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；Browser QA 壳加载正常、无 overlay、console error/warn 为空。

# INSTALL-INTERRUPTED-STATE-1 状态
- `INSTALL-INTERRUPTED-STATE-1` completed：修复安装任务失败/面板重启后实例仍残留 `steam_auth_running` 导致前端卡在 48% 的问题。后端恢复 interrupted `stardew_install` 时会同步写 `install_interrupted`，steam-auth 容器运行错误会写 `steam_auth_failed`；前端安装页以活跃 job 为准，没有活跃安装任务时把旧运行中 phase 视为中断失败，并加载最近安装任务日志。已验证 `cd backend; go test ./internal/jobs ./internal/games/stardew_junimo` 与 `cd frontend; npm.cmd run build` 通过。

# DEPLOY-RUN-SH-1 状态

- `DEPLOY-RUN-SH-1` completed：新增 `deploy/run.sh` 作为用户优先使用的一键启动/维护脚本，产品口径收敛为快速模式：默认通过 `http://服务器IP:8090` 直接访问面板。脚本默认使用国内阿里云 ACR 面板镜像、默认 `latest` tag，并支持 `PANEL_VERSION=0.1.0` 固定版本；首次运行会生成 `~/.anxi-panel/.env`、`docker-compose.yml` 和 `~/.anxi-panel/data`，自动创建强随机 `PANEL_SECRET`。新版脚本使用宿主机数据目录与容器内同名绝对路径持久化面板数据；菜单覆盖 Docker 安装修复、镜像候选兜底、启动、停止、重启、更新、状态、日志、镜像源切换和显示访问地址；同时支持 `install/docker/stop/restart/update/status/logs/url` 等非交互命令。已同步 `docs/09-image-build.md`。

# FE-OPSRAIL-MAINTENANCE-PHASE-1 状态

- `FE-OPSRAIL-MAINTENANCE-PHASE-1` completed：右栏“进行中”卡的计划维护展示已从纯 `nextShutdownAt/nextStartupAt` 倒计时改为当前维护窗口阶段状态。到关机点后显示 `关机中 / 等待关机结束`，关机完成后保留自动开机倒计时，到开机点后显示 `开机中 / 等待开机结束`，开机 job 成功后才切回下一天倒计时；计划维护对应的生命周期 job 不再重复作为普通任务显示。仅改前端 `StardewPanel.tsx`，未改后端接口、调度器、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；Browser QA 全壳右栏渲染无 overlay、console error/warn 为空。

# FE-OVERVIEW-METRIC-TYPE-UNIFY-1 状态

- `FE-OVERVIEW-METRIC-TYPE-UNIFY-1` completed：总览页四张统计卡（存档 / 模组 / 系统健康 / 运行任务）已按用户截图反馈统一字体节奏。`.sd-ov-metric-strip` 下标题、数字、单位、说明和状态徽章现在使用同一 Verdana / Microsoft YaHei / SimHei 字体链，标题为 `14px/800`，数字为 `34px/800`，并减轻数字阴影，避免截图中的字体割裂和过粗跳字。仅改 `StardewPanel.css`，未改 TSX、API、权限、轮询或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x1024、点击服务器再回总览、390x844，console error/warn 为空，overlay 为 0，无横向溢出。

# FE-RESTART-SCHEDULE-PUT-WRITE-MODEL-1 状态

- `FE-RESTART-SCHEDULE-PUT-WRITE-MODEL-1` completed：服务器控制页“计划重启”保存请求体已收口为独立写入模型，避免把 `GET /restart-schedule` 返回的 `instanceId/nextShutdownAt/lastStatus` 等只读展示字段原样回传，触发后端 `DisallowUnknownFields()` 后显示 `request body must be valid JSON`。仅改前端 `types.ts` 与 `api.ts`，未改接口路径、后端调度器、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-STOPPED-STATUS-RED-1 状态

- `FE-STOPPED-STATUS-RED-1` completed：总览页“服务器控制”状态行、服务器控制页顶部摘要状态和生命周期状态行的 `已停止` 已按用户截图反馈改为红色字样，停止态状态点同步为红点。仅改前端展示类名与 CSS，未改生命周期 API、按钮 handler、权限、轮询或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 stopped 总览、服务器页和 390x844 窄屏，目标文字 computed color 均为 `rgb(192, 32, 32)`，console error/warn 为空，无页面级横向溢出。

# PLAYERS-SAVE-ROSTER-1 状态

- `PLAYERS-SAVE-ROSTER-1` completed：玩家名册已恢复从当前 Stardew 存档主 XML 合并 `<player>` 与 `<farmhands><Farmer>`，解决 `players.json` 在线快照只有 host、缓存也只有 host 时，存档里的 farmhand `test` 不显示的问题。存档离线玩家返回 `status=offline/source=save_file`，在线快照仍覆盖同一玩家；前端“等待加入”只统计 `waiting/pending/joining`，不再把离线名册行算进去。已验证 `cd backend; go test ./internal/games/stardew_junimo` 与 `cd frontend; npm.cmd run build` 通过。

# FE-JOBS-LOG-SCROLL-LOCK-1 状态

- `FE-JOBS-LOG-SCROLL-LOCK-1` completed：修复点击“任务与日志”后 Stardew Shell 被浏览器外层页面向下滚走、顶栏消失、底部露黑的问题。根因是缩放 Shell 的未缩放布局盒子让 `body/#root` 产生页面级纵向滚动，任务日志 `scrollIntoView()` 又触发了外层滚动。已在 `App.css` 对 `.sd-shell` 运行态锁定 `body/#root` 为 `100dvh + overflow:hidden`，并把任务日志、安装日志自动滚到底改为滚动各自日志窗口自身。未改 API、SSE、权限、轮询、路由或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 1112x920 下修复前 `window.scrollY=351/.sd-shell.top=-351`，修复后点击任务日志并强制 `window.scrollTo(0,600)` 仍保持 `scrollY=0/.sd-shell.top=0`，console error 为空。

# FE-MOBILE-NAV-BAR-SIZE-1 状态

- `FE-MOBILE-NAV-BAR-SIZE-1` completed：单栏状态下最上方横向选择栏已按截图反馈适量放大。`max-width: 640px` 下导航行从 `40px` 提到 `48px`，按钮从 `36x30` 提到 `42x38`，图标从 `20px` 提到 `23px`。仅改前端 CSS 和接手文档，未改路由、页面逻辑、API、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过，内置 Browser QA 覆盖 490x844 和 390x844，点击“服务器”导航后 active route 正常切换，console error/warn 为空，页面级横向溢出为 0。

# FE-OVERVIEW-LIFECYCLE-LEFT-1 状态
- `FE-OVERVIEW-LIFECYCLE-LEFT-1` completed：总览页“服务器控制”区启动/停止/重启按钮已按用户截图反馈从生命周期区域中间移回左侧，与标题和状态行对齐。根因是旧生命周期 flex 规则残留 `flex-wrap: wrap` + `align-content: center`，本次在总览最终覆盖段改为 `flex-wrap: nowrap`、`align-content: flex-start`，并让按钮行 `align-self: flex-start`。仅改前端 CSS，未改生命周期 API、按钮 handler、邀请码刷新、权限、轮询或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖默认视口、点击启动交互和 390x844 窄屏，console error/warn 为空且无页面级横向溢出。

# FE-PLAYERS-ACTION-ICONS-IMAGE2-1 状态
- `FE-PLAYERS-ACTION-ICONS-IMAGE2-1` completed：玩家页活动列表文字挤压已修正，分页由上一轮的每页 3 条改为每页 2 条并提高事件行高度；管理操作四个 CSS 临时图标已替换为 imagegen 生成、透明抠底后的 image2/Stardew 像素风 PNG（踢出、封禁、白名单、权限）。未改后端 API、玩家事件接口、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x1024 与 390x844，console error/warn 为空、无页面级横向溢出。

# FE-SAVES-BACKUP-POLICY-LAYOUT-1 状态
- `FE-SAVES-BACKUP-POLICY-LAYOUT-1` completed：存档页“自动备份策略”卡片已按用户反馈修正文字错乱。定时备份项调整为“勾选框 / 定时备份 / 每天 / 时间选择框”同一行顺序，每日快照保留行与滑杆重新分配宽度，左侧策略卡不再被右侧列表拉伸。仅改前端 `SavesSection.tsx` 与 `StardewPanel.css`，未改备份策略保存、恢复/删除、权限、后端 API 或 Junimo 通信。已验证清理过期 tsbuildinfo 后 `cd frontend; npm.cmd run build` 通过；QA mock 全壳截图确认策略卡无文字错乱且 console error/warn 为空。

# FE-TOPBAR-SAVE-STATUS-TYPE-1 状态
- `FE-TOPBAR-SAVE-STATUS-TYPE-1` completed：Stardew Shell 顶栏已按用户截图反馈微调。标题字体从过粗的海报感改为更轻、更小的像素描边风格；运行中/已停止状态改用现有像素状态牌素材；存档框移除下拉箭头，农场图标左移贴边，文本改为“农场名：简略游戏时间”（如 `AnxiFarm：第一年春`）；用户角色框也移除下拉箭头。仅改前端 `StardewPanel.tsx` / `StardewPanel.css`，未改 API、权限、路由或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x720 running/stopped 和 390x844，无横向溢出、无 overlay、console error/warn 为空。

# FE-MODS-PROTOTYPE-V02-LAYOUT-1 状态
- `FE-MODS-PROTOTYPE-V02-LAYOUT-1` completed：模组管理页已按 version-02 原型 `06-mods.png` 回正卡片顺序和比例。首屏固定为标题操作区、三段标签页、Nexus 连接横条、搜索卡、2x2 搜索结果卡和分页；下载结果卡恢复原型两按钮结构，移除额外的会员安装按钮；按用户截图反馈移除底部扩展安装进度条和“全部类别”下拉框；搜索提示改为“输入英文模组名称、ID 或关键词...”，热门标签改为 `UI Info`、`Fishing Mod`、`Backpack Upgrades`、`Tractor` 并保持真实快捷搜索。模组页卡片复用其它页面统一羊皮纸卡片变量；前置状态统一放在统计行“认可”后面，无前置也显示“前置：无”，保证每张卡操作按钮垂直位置一致。仅改前端 `ModsPage.tsx` 与 `StardewPanel.css`，未改后端 API、上传/删除/导出、启用切换、玩家同步包或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x1024 和 390x844，无页面级横向溢出，热门标签点击后搜索框为 `Tractor`。

# FE-DIAGNOSTICS-GAUGE-INNER-SAFE-1 状态
- `FE-DIAGNOSTICS-GAUGE-INNER-SAFE-1` completed：诊断页资源趋势圆环卡已按用户反馈修正数字安全区，保持三张卡片与原型比例，扩大中心底色圆并降低数字/百分号字号，避免红色弧线遮挡 `27.1%` 等百分比读数。仅改前端 CSS，未改资源指标接口、健康检查、轮询、导出诊断包或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 点击“诊断”后确认三张资源卡数字不再被弧线遮挡，console error/warn 为空。

# FE-PLAYERS-TIME-EVENTS-PAGING-1 状态
- `FE-PLAYERS-TIME-EVENTS-PAGING-1` completed：玩家管理页在线时长列已压缩为“今天/昨天/N天前 HH:mm”短格式并保留旧数据回退，避免遮挡收入列；在线玩家表收入列顺序调整为“玩家收入 / 农场收入”；玩家活动改为每页 3 条分页，桌面下与右侧管理操作卡同高，移动端自然堆叠且无页面级横向溢出。未改后端 API、玩家轮询、权限或 Junimo 通信。已验证 `cd frontend; npm.cmd run build` 通过；内置 Browser QA 覆盖 1536x1024 与 390x844，console error/warn 为空。

# FE-SETTINGS-API-PORT-REMOVE-1 状态
- `FE-SETTINGS-API-PORT-REMOVE-1` completed：设置与审计页“端口信息”卡片已移除只读“API 端口”字段，仅保留“面板端口 / VNC 端口 + 保存/刷新”。只改前端展示 JSX 和设置页端口行 CSS，未改 VNC 端口保存接口、权限判断、后端 API、Junimo 通信或轮询逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-SAVES-UPLOAD-BLUE-BG-1 状态
- `FE-SAVES-UPLOAD-BLUE-BG-1` completed：存档页上传横条背景已按用户反馈从羊皮纸虚线样式恢复为之前的蓝色天空版本（蓝色渐变 + 白色像素云 + 木色实线边框）。仅改 `StardewPanel.css` 视觉样式，上传弹窗、ZIP 预览、导入并启动、权限和禁用逻辑不变。已验证 `cd frontend; npm.cmd run build` 通过；QA mock 全壳截图确认上传条为蓝色背景且 console error/warn 为空。

# FE-SAVES-V02-PROTOTYPE-LAYOUT-1 状态
- `FE-SAVES-V02-PROTOTYPE-LAYOUT-1` completed：存档管理页已按 version-02 原型 `03-saves.png` 回正卡片位置和比例。激活存档区改为“信息卡 + 右操作卡”，存档库工具按钮上移到标题右侧，桌面主宽下三张存档卡同排，上传条与底部“自动备份策略 / 备份列表”双栏按原型落位。仅改前端 TSX/CSS 展示结构，创建/上传/选择/删除/导出/备份/恢复接口和权限逻辑不变。已验证 `cd frontend; npm.cmd run build` 通过；QA mock 全壳 1536x1024 对照原型、390x844 无横向溢出且 console error/warn 为空。

# FE-SETTINGS-PROTOTYPE-V02-LAYOUT-2 状态
- `FE-SETTINGS-PROTOTYPE-V02-LAYOUT-2` completed：设置与审计页已按 version-02 原型 `09-settings.png` 回正卡片结构和比例。左列为面板版本、用户管理、端口信息、其他设置，右列为安全与权限、审计日志、安全建议；版本卡补右侧图像槽，安全摘要改单列表，端口卡当时改三端口横排，后续 `FE-SETTINGS-API-PORT-REMOVE-1` 已移除重复的“API 端口”；审计首屏 7 条，安全建议改三条状态徽章。仅改前端页面与 CSS，未改 API、权限、轮询或后端逻辑。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器临时 mock QA 覆盖 1536x1024 与 390x844，console error/warn 为空、无页面级横向溢出，QA 临时文件已删除。

# FE-SERVER-PROTOTYPE-V02-LAYOUT-2 状态
- `FE-SERVER-PROTOTYPE-V02-LAYOUT-2` completed：服务器控制页已按 version-02 原型 `02-server.png` 回正卡片结构和比例。顶部摘要改为服务器专用整行大卡（状态/在线玩家/当前存档/主机农民/游戏日期 + 邀请码横条），中部恢复生命周期左列、快捷操作右列，快捷操作改为原型式浅色工具行，底部控制台命令横跨整行且终端满宽；移动端恢复单列顺序并无页面级横向溢出。未改 API、权限、轮询、Junimo 通信或后端逻辑。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器临时 mock QA 覆盖 1536x1024 与 390x760，console error/warn 为空，QA 临时文件已删除。

# FE-OVERVIEW-BANNER-SCENE-IMAGE2-1 状态
- `FE-OVERVIEW-BANNER-SCENE-IMAGE2-1` completed：总览页顶部农场横幅场景已替换为从 image2 原型 `01-overview.png` 裁出的运行时素材 `overview_banner_scene_image2.png`，不再由 CSS 田野/云层和旧 `sprite_farmhouse_scene.png` 叠加生成。裁切只包含农场场景，不含统计条或页面外壳；仅改前端 CSS 与静态素材，未动组件逻辑/API/后端。已验证新 PNG 预览正常，`cd frontend; npm.cmd run build` 通过。

# FE-PROTOTYPE-SHELL-ALIGN-1 状态
# FE-DIAGNOSTICS-PROTOTYPE-V02-LAYOUT-1 状态
- `FE-DIAGNOSTICS-PROTOTYPE-V02-LAYOUT-1` completed：诊断页已按 version-02 current-frontend-code 原型 `07-diagnostics.png` 回正卡片位置与比例。页面专属收紧主 frame inset，顶部状态横卡约 `975x160`，中部检查项/资源趋势回到等宽双列约 `482x392`，底部告警建议横跨全宽并留在首屏内；资源仪表卡改为标题在上、圆环居中、说明在下，趋势图压回原型短图。未改后端接口、权限、轮询或业务状态。已验证 `cd frontend; npm.cmd run build` 通过；Playwright + 本机 Chrome 回退 QA：1536x1024 与 390x844 无横向溢出、console error/warn 为空。

- `FE-PROTOTYPE-SHELL-ALIGN-1` completed：九页前端布局已对齐 image2 version-02 原型。根因是右信息栏(414px)/左导航(252px)过肥把主内容挤到 791px；已把 `--sd-opsrail-width` 收到 `clamp(268px,19vw,300px)`、`--sd-sidebar-width` 收到 `clamp(196px,14vw,216px)`，主内容区回到 937px，总览恢复 4 卡一行、控制区与邀请码并排。逐页修：服务器(生命周期|快捷操作并排、快捷操作改竖排)、任务日志(列表|详情两列)、玩家(表整行+活动|管理两列，逆转 FE-PLAYERS-LIST-LEFT-1)、诊断(检查信息单行不折)、设置(用户/审计表列不裁切)、存档(上传区改羊皮纸虚线)、模组(结果卡两列网格)、顶栏版本不折行。仅改 `StardewPanel.css`，未动逻辑/接口。已验证 `cd frontend; npm run build` 通过；mock-fetch harness + Playwright 1536 逐页对比原型、pageerror 为 0，真实登录态截图待补。

# FE-PLAYERS-LIST-LEFT-1 状态
- `FE-PLAYERS-LIST-LEFT-1` completed：玩家管理页桌面首屏已调整为左侧宽列显示在线玩家表、右侧窄列显示最近事件，取消旧规则导致玩家表落到右侧第三行的问题，减少中间空白；服务器信息（Junimo）保留为底部整行调试信息。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-DIAG-GAUGE-TOMIK-1 状态
- `FE-DIAG-GAUGE-TOMIK-1` completed：诊断页 CPU/内存/磁盘三资源圆环从铜钱 conic 样式改为 Tomik23 circular-progress-bar 风格（灰底环 + yellow→#ff0000 渐变圆头描边 + 中心百分比），纯 SVG 实现无新依赖；空态只画底环。已验证 `cd frontend; npm.cmd run build` 通过，Playwright 真实登录态 1366/390 截图正常、pageerror 为 0。

# FE-UNIFIED-CARD-PARCHMENT-TONE-1 状态
- `FE-UNIFIED-CARD-PARCHMENT-TONE-1` completed：总览统计卡当前的浅羊皮纸暖黄已提升为统一小卡片背景色，文件尾部覆盖 `--sd-save-card-bg` / `--sd-save-card-bg-strong`，所有复用统一小卡片变量的非模组页小框都会跟随；总览 `.sd-mc` 保持同色且无斜纹。未改卡片尺寸、边框、圆角、阴影、文字布局或业务逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-INSTALL-STEAM-AUTH-ICON-1 状态
- `FE-INSTALL-STEAM-AUTH-ICON-1` completed：安装页“Steam 认证”卡片中间占位图标和栏目标题小图标均已改为复用安装进度第三步的 `icon_install_step_steam_image2.png`，不再使用 CSS 渐变画出的蓝色 Steam 圆球。未改 Steam 认证、Steam Guard、扫码、日志或后端接口。已验证 `cd frontend; npm.cmd run build` 通过；真实安装页受登录页限制，登录态截图待补。

# FE-SETTINGS-FILL-GAP-1 状态
- `FE-SETTINGS-FILL-GAP-1` completed：设置页已从三段式布局改为左右两列堆叠，左列为“面板版本 / 用户管理 / 端口信息 / 其他设置”，右列为“安全与权限 / 审计日志 / 安全建议”，让端口信息和其他设置上移补足短列空位；`780px` 以下再切回单列。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-OVERVIEW-METRIC-CLEAN-BG-1 状态
- `FE-OVERVIEW-METRIC-CLEAN-BG-1` completed：总览页四个统计卡 `.sd-mc` 已移除斜向纸纹背景，并改为干净、偏浅的羊皮纸暖黄渐变；后续按反馈从偏白略微压黄，但不恢复旧的高饱和黄色。保留卡片尺寸、边框、角饰、文字布局和状态徽章。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-INSTALL-HERO-SCENE-REMOVE-1 状态
- `FE-INSTALL-HERO-SCENE-REMOVE-1` completed：安装页顶部状态横幅已移除右侧大农舍场景图 `.sd-install-farm-scene`，不再渲染安装页顶部的 `sprite_farmhouse_scene.png`；状态横幅改为“小土芽图标 + 状态信息”两列。未改安装状态、Steam 认证、日志、进度或后端接口。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-INVITE-CARD-COPY-ORDER-1 状态
- `FE-INVITE-CARD-COPY-ORDER-1` completed：邀请码卡片新增共享 `InviteCodeCard`，服务器摘要卡和总览页服务器控制区统一复用；复制按钮调整到刷新按钮左侧，仅在已有邀请码时渲染，未有码状态不保留空按钮列。总览页已移除旧 `sd-invite-panel` JSX、本地复制状态和独立 `handleCopy()`，后续邀请码展示/复制/刷新只维护一套组件。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器在未登录状态下确认服务器页与总览页应用壳非空、console error/warn 为空，登录态卡片截图待补。

# FE-SETTINGS-ACCOUNT-CARD-REMOVE-1 状态
- `FE-SETTINGS-ACCOUNT-CARD-REMOVE-1` completed：设置与审计页已移除顶部“当前账号”卡片，避免和顶栏用户入口重复；顶部摘要区从三卡改为“面板版本 / 安全与权限”两卡，并清理 `sd-settings-account-*` 死样式。登出仍保留在 Stardew Shell 顶栏，未改 session、权限、用户管理或审计日志逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-PAGE-HEADER-SHADOW-1 状态
- `FE-PAGE-HEADER-SHADOW-1` completed：Stardew 各路由页头已移除标题文字、导航图标和右侧虚线分隔的阴影背景，只保留干净标题、图标和分隔线；未改页面结构、按钮、卡片布局、API 或业务逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-PAGE-TOP-ALIGN-1 状态
- `FE-PAGE-TOP-ALIGN-1` completed：Stardew 各 routed 页面已通过文件尾部 CSS 兜底统一贴齐主内容 frame 顶部，覆盖任务、诊断、安装、设置等页面后置 `padding` 造成的顶部下沉；未改页面结构、卡片布局、API 或业务逻辑。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-SERVER-ACTION-CARDS-1 状态
- `FE-SERVER-ACTION-CARDS-1` completed：服务器控制页生命周期控制卡已下移到顶部摘要卡下方左侧，快捷操作卡放到同一行右侧；快捷操作按钮统一叠加 `.sd-btn--lg`，与生命周期按钮共用 40px lg 尺寸令牌，并移除原 64px 卡片式按钮和伪图标。已验证 `cd frontend; npm.cmd run build` 通过；真实路由受登录页限制，仅完成应用非空、无框架覆盖、console error/warn 为空的烟测，未完成登录态截图验证。

# FE-SERVER-INVITE-IN-SUMMARY-1 状态
- `FE-SERVER-INVITE-IN-SUMMARY-1` completed：服务器控制页移除中部独立“邀请代码”卡片，邀请码复制/刷新入口收敛到顶部服务器摘要卡；“刷新”按钮位于“邀请加入码”显示区右侧，运行中/启动中可用，未运行时禁用。删除 `ServerControlPage` 第二套复制状态和 `handleCopy()`，后续统一复用 `ServerSummaryCard`；“全服消息”在删除邀请码卡片后改为横跨整行。已验证 `cd frontend; npm.cmd run build` 通过；真实路由受登录页限制，内置浏览器确认应用加载非空且 console error/warn 为空，未完成登录态截图验证。

# NEXUS-ERROR-TEXT-1 状态
- `NEXUS-ERROR-TEXT-1` completed：Nexus 搜索/安装错误响应中的中文 mojibake 已修复，`nexus_request_failed` 等错误不再显示为 `璇锋眰澶辫触`。前端 `errorCodeMap` 同步增加 Nexus 错误码中文兜底，后续即使后端 message 异常也会优先显示正常中文。已验证 `cd backend; go test ./internal/web -run TestWriteNexusErrorMessagesAreReadableChinese` 和 `cd frontend; npm.cmd run build` 通过。

# FE-BTN-UNIFY-1 状态
- `FE-BTN-UNIFY-1` completed：9 个页面按钮与操作区统一化。按钮尺寸收敛为 lg 40px / md 28px / sm 22px 三档令牌（`.sd-btn--lg` / 默认 / `.sd-btn--sm`），语义色收敛为绿(主)/棕(次)/红(危险)，删除 `.sd-btn-blue`、`.sd-btn-xs` 与死样式 `sd-btn-gold`/`sd-btn-red`；危险确认弹窗统一红色确认键与"取消+确认"顺序；新增共享 `.sd-actionbar` / `.sd-rowactions` 操作区布局类；删除全部逐页按钮尺寸覆写与 JSX 内联尺寸；统一"刷新/保存/X并启动"等文案；修复总览页无邀请码时邀请面板被 1px 网格列挤成竖排的问题。已验证 `cd frontend; npm.cmd run build` 通过，并用 Playwright 真实登录态对 9 页 × 4 视口做改前/改后截图对比，console 无新增报错。

# FE-MAIN-PAGE-FRAME-SLICES-1 状态
- `FE-MAIN-PAGE-FRAME-SLICES-1` completed：所有 Stardew 页面共用的 `.sd-main` 主内容 frame 已从整图 `100% 100%` 拉伸改为 image2 空框 9 切片平铺。新增四角、上下左右边 tile 和中心羊皮纸 tile，运行时用 9 层 CSS background 实现四角固定、上下 `repeat-x`、左右 `repeat-y`、中心 `repeat`，窗口缩放时边框纹理不再被拉伸；`.sd-main-scroll` 统一滚动视口保持不变。已验证 `cd frontend; npm.cmd run build` 通过，并用内置浏览器临时 QA 页检查 1280x720 与 390x760 下背景层/重复规则正确、无横向溢出、滚动正常、console error/warn 为空。

# FE-MODS-DEPENDENCY-POPOVER-1 状态
- `FE-MODS-DEPENDENCY-POPOVER-1` completed：下载模组页 Nexus 搜索结果的前置信息弹层已改为受控按钮 + 弹层，修复固定高度搜索卡片裁切导致前置信息看起来无法展开的问题；点击信息弹层外部会自动收起，不再需要回点前置按钮。已验证 `cd frontend; npm.cmd run build` 通过，并用内置浏览器临时 QA 页检查 1280x720 与 390x760 展开/外部点击收起、无水平溢出、console error/warn 为空。

# JOB-DISPLAY-NAME-1 状态
- `JOB-DISPLAY-NAME-1` completed：jobs 表新增 `display_name`，任务 API/SSE 返回 `displayName`；Nexus/远程 Mod 安装任务会写入 `mod_nexus_install · <Mod 名>` / `mod_remote_install · <Mod 名>`，前端任务页、右栏进行中、右栏近期任务和总览近期事件均优先展示该字段，解决并行依赖安装时多个任务同名不可区分的问题。已验证后端 storage/jobs/web 测试和前端构建通过。

# MODUPLOAD-DUPLICATE-CODE-1 状态
- `MODUPLOAD-DUPLICATE-CODE-1` completed：Mod ZIP 上传遇到已安装相同 `UniqueID` 时，后端响应错误码改为 `mod_exists`，避免前端显示成“Mod ZIP 无效/解析失败”；真正坏 ZIP、无 manifest、XNB 替换包等仍保持 `invalid_mod_zip`。已验证 `cd backend; go test ./internal/web -run "TestModUpload"` 通过。

# FE-OPSRAIL-DOWNLOAD-PROGRESS-1 状态
- `FE-OPSRAIL-DOWNLOAD-PROGRESS-1` completed：右侧 OpsRail“进行中”卡已接入远程 Mod 安装下载日志进度，`mod_remote_install` / `mod_nexus_install` 会解析 `下载进度：已下载 ...（xx.x%）` 并映射到右栏进度条；扩展 batch 一旦返回新的 `jobId` 会立即刷新 jobs，让任务尽快出现在右栏，Premium 安装路径也同步主动刷新。已验证 `cd frontend; npm.cmd run build` 通过。

# NEXUS-EXT-DOWNLOAD-GUARD-1 状态
- `NEXUS-EXT-DOWNLOAD-GUARD-1` completed：远程 Mod 安装任务新增下载可见进度和错误分界日志，显示连接远程下载服务器、HTTP 响应、Content-Type、压缩包大小、已下载/总量/剩余/百分比；无总量时显示已下载大小。浏览器扩展提交前强制校验 Nexus CDN `.zip`，后台页未真正拿到 ZIP 时不再创建面板安装任务；后端收到 `text/html` 会立即失败并提示这是网页不是 ZIP。已验证后端相关测试与扩展 JS 语法检查通过。

# FE-SERVER-PROTOTYPE-IMAGE2-1 状态
- `FE-SERVER-PROTOTYPE-IMAGE2-1` completed：服务器控制页已按 image2 原型重皮肤为羊皮纸控制台结构，包含当前状态大卡、生命周期控制卡、邀请代码、全服消息、黑色命令终端和快捷操作条；原型图未作为运行时资源，纸纹、铜边、内阴影、绿屏、终端和分隔线均由 CSS 实现。业务逻辑/API/权限/disabled 状态保持不变，按钮和标题图标复用既有 Stardew PNG/图标素材。已验证 `cd frontend; npm.cmd run build` 通过，并用临时 QA 页检查 1280x900 与 390x760 无横向溢出、按钮文字不溢出、命令执行输出可读。

# FE-DIAGNOSTICS-IMAGE2-ICONS-1 状态
- `FE-DIAGNOSTICS-IMAGE2-ICONS-1` completed：诊断页首轮 CSS 自绘图标已替换为 image2 风格透明 PNG 素材。新增 `frontend/public/assets/stardew/ui/diagnostics/`，包含 4x5 生成 sprite sheet 与状态盾牌、三色宝石、检查项、建议区、资源趋势、实时绿点、导出下载等 20 个单图；运行时使用单图背景，不使用整页原型图或整块截图。已验证 `cd frontend; npm.cmd run build` 通过，内置浏览器临时 QA 页检查 1280x900 与 390x760 无横向溢出、按钮不溢出、console error/warn 为空，可见诊断页图标背景均来自 `/assets/stardew/ui/diagnostics/`。

# FE-DIAGNOSTICS-PROTOTYPE-IMAGE2-1 状态
- `FE-DIAGNOSTICS-PROTOTYPE-IMAGE2-1` completed：诊断与健康页已按 image2 原型重皮肤为顶部双操作、系统状态横卡、三计数卡、检查项表、资源趋势卡和底部告警建议区；原型图未作为运行时背景或整块资源引用，业务逻辑/API/权限保持不变。已验证 `cd frontend; npm.cmd run build` 通过，并用内置浏览器临时 QA 页检查 1280x900 与 390x760 无横向溢出、无面板重叠、按钮文字不溢出、console error/warn 为空。注：首轮 CSS 自绘图标已在 `FE-DIAGNOSTICS-IMAGE2-ICONS-1` 中替换为 image2 PNG 素材。

# FE-SETTINGS-PROTOTYPE-IMAGE2-1 状态
- `FE-SETTINGS-PROTOTYPE-IMAGE2-1` completed：设置与审计页已按 image2 `09-settings - 副本.png` 原型重皮肤为顶部三卡、中部用户/审计双栏、底部设置/安全建议双栏结构。原型图未作为运行时背景或整块素材引用；纸纹、铜边、角钉、表头、分隔线和提示面板均为 CSS 实现，按钮/标题图标复用现有 Stardew PNG 素材。功能逻辑、API 调用、权限判断、loading/error/empty/disabled 状态保留。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器真实路由停在登录页，因此使用已删除的临时 `settings-qa.html` 加载同 CSS/素材/同结构 DOM 验证 1280x900 桌面、点击新建用户展开态、390x760 窄屏和底部状态均无页面级横向溢出，console error/warn 为空。

# FE-INSTALL-PROTOTYPE-IMAGE2-1 状态
- `FE-INSTALL-PROTOTYPE-IMAGE2-1` completed：安装页已按 image2 `08-install - 副本.png` 原型重皮肤为顶部状态横幅、五步安装时间线、安装配置/Steam 认证/安装日志三栏工作区。原型图未作为运行时资源；纸张纹理、边框、分隔线、步骤卡、进度条和日志终端均为 CSS 实现，按钮继续复用既有 PNG 按钮体系。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器临时 QA 覆盖 1280x900 认证态、未安装态点击展开表单、390x760 窄屏无横向溢出，console error/warn 为空。

# FE-OPSRAIL-AUTO-COLLAPSE-1 状态
- `FE-OPSRAIL-AUTO-COLLAPSE-1` completed：Stardew Shell 已按主内容预计宽度自动收起右侧 OpsRail，展开态主内容低于 `820px` 收起、收起后高于 `880px` 再展开；总览页 1180px 响应式规则补为 `.sd-main-scroll` 容器查询，避免窗口缩放时中间内容被右栏挤压后仍按桌面布局排布。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器 QA 覆盖 1200x760 自动收起、1200→1600 无刷新自动展开、390x760 移动布局，均无横向溢出且 console error/warn 为空。

# FE-OVERVIEW-PROTOTYPE-IMAGE2-1 状态
- `FE-OVERVIEW-PROTOTYPE-IMAGE2-1` completed：总览页已按 image2 原型重皮肤为农场横幅 + 羊皮纸控制台 + 四摘要卡 + 三列清单结构；页面背景、纸纹、卡片、分隔线、绿屏邀请码和状态标签均由 CSS 实现，未把原型图作为运行时资源。按钮和小图标复用现有 Stardew PNG/图标素材，业务逻辑/API/权限不变。已验证 `cd frontend; npm.cmd run build` 通过，并用内置浏览器临时 QA 页检查 1280x900 与 390x760 无横向溢出、按钮文字不溢出、console error/warn 为空。

# FE-SAVES-SIMPLIFY-3 状态
- `FE-SAVES-SIMPLIFY-3` completed：存档页页头精简为左上角纯文字标题（去框去描述）；全部按钮换回面板既有 PNG 素材按钮（`sd-btn-green/tan/delete`），撤销自绘 `.sd-pxbtn`；备份与恢复规则压缩为单行控件条，说明文字移入悬浮提示，文案精简。已验证前端构建通过。

# FE-SAVES-MOCKUP-2 状态
- `FE-SAVES-MOCKUP-2` completed：存档页按用户完整设计稿改版：眉标行、激活卡图标字段双列表、存档库只显示非激活存档（选择/删除糖块按钮）、横排虚线创建卡、上传横条（📮 + 上传并启动）、备份六列表格（状态徽章 + 5 行折叠）；新增纯 CSS `.sd-pxbtn` 糖块按钮体系；导出/手动备份/启动收敛到激活卡。已验证前端构建通过。

# FE-SAVES-PROTO-CSS-1 状态
- `FE-SAVES-PROTO-CSS-1` completed：存档页按 image2 原型重构为纯 CSS 皮肤（无图片资源）：铜框铆钉激活卡、双列虚线字段、圆角存档卡、虚线创建卡、像素云天空上传横条、表头带备份表格；创建/上传入口从页头移入创建卡与上传横条，业务逻辑未变。已验证前端构建通过。
- `FE-SAVES-PROTO-CSS-1` follow-up：激活存档卡预览槽接入真实农场地图，按存档 `farmType` 复用新建游戏的 `/assets/stardew/new-game/farms/<farmType>.png` 素材，未知类型回落为空羊皮纸块。已验证前端构建通过。

# FE-RIGHT-RAIL-ACTIVE-CARD-1 状态
- `FE-RIGHT-RAIL-ACTIVE-CARD-1` completed：右栏"进行中"卡接入自动关机/自动开机倒计时（restart-schedule）、定时备份倒计时（backups/policy，仅管理员行）和运行中任务进度条（同类型历史耗时中位数估算，封顶 95%）；倒计时行为蓝色档、每秒走动，任务完成后行自动消失；dashboard 30s 轮询同时刷新 jobs 兜底调度器触发的任务。已验证前端构建通过。

# FE-RIGHT-RAIL-HEALTH-STATS-1 状态
- `FE-RIGHT-RAIL-HEALTH-STATS-1` completed：右栏"系统健康"卡按原型接入真实数据：CPU/内存/磁盘使用率（复用 `GET /api/instances/:id/metrics`，5s 轮询，圆润胶囊形进度条）、在线玩家（`onlineCount/当前存档 MaxPlayers`）、网络延迟（metrics 请求前端往返耗时）；按钮改为"查看详情 →"。已验证前端构建通过。
- `FE-RIGHT-RAIL-HEALTH-STATS-1` follow-up：五行增加阈值配色，浆果点/进度条/数值随数值绿→黄→红（使用率 60/85、延迟 100/300ms 分档），在线玩家为 0 时显示红色。已验证 `npm run build` 通过。

# PLAYERS-MAXPLAYERS-1 状态
- `PLAYERS-MAXPLAYERS-1` completed：`stardew_junimo` driver 的 `ListPlayers` 现在用当前存档 `server-settings.json` 的 `Server.MaxPlayers` 兜底 `maxPlayers` 字段（junimo info 解析出的值仍优先），玩家接口在服务器未运行或 info 不含上限时也能返回人数上限。已验证 `go test ./internal/games/stardew_junimo/` 通过。

# FE-RIGHT-RAIL-CARD-FIX-1 状态
- `FE-RIGHT-RAIL-CARD-FIX-1` completed：右栏三卡（系统健康/进行中/近期任务）去掉内部滚动与等高拉伸逻辑，行高改为随内容、与左侧栏按钮一致只随栏宽缩放；四角藤蔓拉伸修复为每卡单一缩放系数换算 border-width/margin，角部切片横纵等比。已验证 `npm run build` 通过。详见 `docs/03-frontend.md` 与 `docs/frontend-handoff/frontend-handoff-2026-07-02.md` 对应条目。

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
- `PLAYERSYNC-PACK-5` completed：玩家同步包安装 SMAPI 时改为调用随包官方 Windows 安装器 `internal/windows/SMAPI.Installer.exe`，传入 `--install --game-path "<Stardew Valley>" --no-prompt`；脚本不再直接解包 `install.dat`，也不做本机 `.NET` / `runtimeconfig` 特调。安装器超过 120 秒未退出时会提示玩家检查安装器窗口是否在等待按键/输入。
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
| MOBILE-SHELL-M0-1 | completed | 移动端基础入口 M0：`useMediaQuery` hook + `App.tsx` 按 `<=768px` 分流到占位组件 `StardewMobileShell`（顶部品牌/状态、羊皮纸占位卡、5 个静态 Tab），桌面端 `StardewPanel` 行为视觉不变；真实移动端页面内容和 Tab 路由映射留给 M1 |
| MOBILE-HOME-M2-1 | completed | 移动端总览页 M2：`StardewMobileShell` 的“总览”Tab 接入真实 `mobile/MobileHomePage.tsx`（状态摘要/邀请信息/快捷控制/待认证玩家批准四张卡片），全部复用现有 API 和 `useStardewDashboardData()`，未新增后端接口，桌面端总览页未改 |
| MOBILE-CONTROL-M3-1 | completed | 移动端控制页 M3：`StardewMobileShell` 的“控制”Tab 接入真实 `mobile/MobileControlPage.tsx`（全服消息卡 + 快捷操作卡：计划重启/密码设置/小屋高级设置/触发节日活动/永久启用Joja路线，去掉手动备份和VNC显示），全部复用现有 API，未新增后端接口，桌面端 `ServerControlPage.tsx` 未改（除删除过时的 say 命令提示文案） |
| MOBILE-PLAYERS-M4-1 | completed | 移动端玩家页 M4：`StardewMobileShell` 的“玩家”Tab 接入真实 `mobile/MobilePlayersPage.tsx`（单卡“在线玩家”：右上角刷新按钮 + 玩家卡片列表[名称/状态徽章/主机角色/最近活动/位置信息 + 踢出/封禁]），踢出/封禁复用 `kickPlayer`/`banPlayer`，未新增后端接口，桌面端 `PlayersPage.tsx` 未改；不含待授权玩家同意/拒绝（保留在 `MOBILE-HOME-M2-1` 的总览 Tab） |
| LOGIN-MOBILE-FIX-1 | completed | 修复登录/初始化页在手机端的布局崩坏：`App.css` 新增 `@media(max-width:768px)` 覆盖，`.sd-auth-shell--image-login` 放弃固定 16:9 比例的绝对坐标定位，改回真实文档流羊皮纸卡片，复用现有三张素材，未新增图片、未改 API/权限 |
| FE-LAZYLOAD-1 | completed | 前端拆分阶段一：桌面端 9 路由（`StardewPanel.tsx`）与移动端 5 页面（`StardewMobileShell.tsx`）改为 `React.lazy` + `Suspense` 按需加载，主 JS chunk 从约 579 KB 降到约 243 KB，构建 chunk 大小警告消失；hook 拆分与 CSS 按页面拆分列入阶段二/三，见 `docs/07-later-optimizations.md` |
| FE-LIFECYCLE-ACTIONS-1 | completed | 前端拆分阶段二第一项：新增 `useStardewLifecycleActions.ts`，把 `OverviewPage.tsx`/`ServerControlPage.tsx` 重复的启停 action、pending 状态、确认弹窗逻辑合并为一个 hook；`OverviewPage.tsx` 555→456 行，`ServerControlPage.tsx` 减少约 90 行重复逻辑；阶段二后续（ServerControlPage 其他领域 hook、SavesSection、ModsPage）见 `docs/07-later-optimizations.md` |
| FE-SERVER-DOMAIN-HOOKS-1 | completed | 前端拆分阶段二第二项：`ServerControlPage.tsx` 拆成 9 个独立领域 hook（备份/计划重启/VNC/密码/运行时设置/节日/Joja/控制台/喊话），页面从 1437 行降到 979 行；阶段二剩余项（SavesSection、ModsPage）见 `docs/07-later-optimizations.md` |
| FE-MODS-MANAGEMENT-HOOK-1 | completed | 前端拆分阶段二 ModsPage 项：新增 `useModsManagement.ts`，集中本服 Mod 列表、上传/删除/导出、玩家同步与当前存档启用切换；`ModsPage.tsx` 2536→2360 行，Nexus 扩展批量状态机保持原有时序 |
| FE-CSS-SPLIT-1 | completed | 前端拆分阶段三：`StardewPanel.css` 拆为共享 Shell CSS 与 9 个桌面页面 CSS，各懒加载页面自行 import；共享 CSS 约 16586→4551 行，页面样式进入独立按需 chunk |
| FE-SAVES-DOMAIN-HOOKS-1 | completed | 前端拆分阶段二 SavesSection 项：新增 `useSaveBackups.ts`（备份列表/策略/手动备份/删除备份）与 `useSaveRestore.ts`（回档确认弹窗），`SavesSection.tsx` 1236→1131 行；存档列表 CRUD、新建游戏、上传存档弹窗因不属于回档领域且低耦合，保留在页面内 |

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
- 更完整的移动端导航和表格卡片化：M0（识别移动端 + 占位壳分流）、M2（总览 Tab 真实化，见 `MOBILE-HOME-M2-1`）、M3（控制 Tab 真实化，见 `MOBILE-CONTROL-M3-1`）、M4（玩家 Tab 真实化，见 `MOBILE-PLAYERS-M4-1`）已完成；`任务`/`更多` 两个 Tab 仍是占位，控制台命令执行、玩家踢出/封禁操作未搬到移动端（均不在已完成里程碑范围内），详见 `docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `MOBILE-PLAYERS-M4-1` 小节“下一步注意事项”。

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
- `FE-PROTOTYPE-LAYOUT-1` completed：前端主要 Stardew 页面已按 `external artifact stardew-page-prototypes-image2-2026-06-30` 的信息架构重新排布。总览页对齐农场横幅、生命周期控制、邀请码、摘要指标和三列摘要；存档页新增当前激活存档重点卡；服务器、任务、玩家、模组、诊断、设置页通过页面级布局 class 调整为原型式分区。现有 API 和功能不变，`ModsPage` 保留三段式工作台。
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

# FE-SHELL-SCALE-1 状态
- `FE-SHELL-SCALE-1` completed：Stardew Shell 已接入全局等比缩放。以 `1536x1024` 为设计基准，`.sd-shell` 计算 `--sd-ui-scale = max(0.72, min(100vw/1536, 100dvh/1024))`，使用反向宽高 + `transform: scale(...)` 让整套 UI 填满当前浏览器可视区；右 OpsRail 隐藏阈值下放到最小全布局之后，`StardewPanel.tsx` 自动折叠估算同步按新 scale/栏宽计算。已验证前端构建通过，并用临时 HTTP QA 页确认 760x504 为最小三栏基准、1920x1080 可继续等比放大且无页面溢出。

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
- `FE-MAIN-PAGE-FRAME-1` completed：所有 Stardew 路由的中间主内容区 `.sd-main` 已统一替换为 image2 存档页空框背景 `main_page_frame_empty_image2.png`。资源从 `external artifact stardew-page-prototypes-image2-2026-06-30 (03-saves-page-frame-empty-image2.png)` 复制到 `frontend/public/assets/stardew/ui/panels/` 供运行时和 Docker 静态发布使用；主内容背景改为居中、不重复、`100% 100%` 铺满，并把页面整体 padding 调整为 `clamp(28px, 2.4vw, 42px)` 避免压到木框角饰。主内容区仍保留 `overflow-y: auto`，但已隐藏 Firefox/Chromium/WebKit 原生滚动条，避免白色竖条压住右侧 frame 边框。已验证前端构建通过，生产 CSS 临时 Shell QA 页在 1280x720 和 390x760 下背景加载、滚动条隐藏、滚动能力保留、无横向溢出、console error/warn 为空。
# FE-MODS-DYNAMIC-PAGESIZE-1 状态
- `FE-MODS-DYNAMIC-PAGESIZE-1` completed：模组下载页 Nexus 搜索结果已改为固定搜索卡片高度 + 动态 pageSize。`.sd-mods-nexus-search-list` 卡片高度固定 `246px`，前端按真实结果网格在 `.sd-main-scroll` 内的可见高度和实际列数计算每页数量，并传给既有 Nexus 搜索 API 的 `pageSize`；加载骨架不参与测量，避免 loading/结果态来回触发刷新；顶部翻页器同步显示“每页 N 个”，底部重复翻页器移除。已验证 `cd frontend; npm.cmd run build` 通过，并用临时本地 QA 页面确认 1040x1120 为 pageSize=4、1040x720 为 pageSize=2、520x720 为 pageSize=1，卡片高度均为 `246px`。
# FE-JOBS-PROTOTYPE-IMAGE2-1 状态
- `FE-JOBS-PROTOTYPE-IMAGE2-1` completed：任务与日志页已按 image2 原型重皮肤为羊皮纸双栏任务台，包含顶部标题虚线、按钮工具条、左侧任务列表、右侧任务详情/进度/SSE 状态/深色日志终端和 VNC 修复提示。原型图未作为运行时资源；纸纹、铜边、选中态、状态徽章、进度条、终端扫描线和警告纸条均由 CSS 实现，按钮/图标复用既有 Stardew PNG/图标素材。业务逻辑、API、权限、loading/error/empty/disabled 状态保持不变。已验证 `cd frontend; npm.cmd run build` 通过，并用临时 QA 页检查 1280x900 与 390x760 无横向溢出、按钮文字不溢出、移动端底部 VNC 提示可滚动完整查看、console error/warn 为空。
# FE-PLAYERS-PROTOTYPE-IMAGE2-1 状态
- `FE-PLAYERS-PROTOTYPE-IMAGE2-1` completed：玩家管理页已按 image2 `05-players - 副本.png` 原型重皮肤为六摘要卡、邀请加入码横条、Junimo 终端、在线玩家表、活动历史和管理员操作区结构。原型图未作为运行时背景或整块素材引用；纸纹、铜边、角钉、分隔线、绿字终端、表格和禁用操作按钮均为 CSS 实现。功能逻辑、API 调用、权限判断、loading/error/empty/disabled 状态保留；按钮/图标复用现有 Stardew PNG/CSS 按钮体系。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器临时 QA 覆盖 1280x900 与 390x760 无页面级横向溢出，桌面表格操作列首屏可见，窄屏表格仅自身横向滚动，console error/warn 为空。
# FE-DIAGNOSTICS-GAUGE-CODE-1 状态
- `FE-DIAGNOSTICS-GAUGE-CODE-1` completed：诊断与健康页 CPU / 内存 / 磁盘三枚资源仪表已完成代码级视觉优化。数值与 `%` 拆分排版，修复圆心文字拥挤；像素分段进度环、硬边描边、羊皮纸内芯、阴影和高光均由 CSS gradient / custom properties / box-shadow 实现。未新增图片素材，未使用原型图或截图作为资源，业务逻辑、API、权限和状态处理保持不变。
# FE-INSTALL-IMAGE2-ICONS-2 状态
- `FE-INSTALL-IMAGE2-ICONS-2` completed：安装页 CSS 自绘图标已替换为从 image2 安装页原型提取/抠图生成的透明 PNG 小素材。新增 `frontend/public/assets/stardew/ui/install/` 下 6 个单图，覆盖顶部状态土芽和五步时间线图标；未使用整页原型图作为背景或整块资源，页面纸卡、边框、分隔线、进度条和日志终端仍由 CSS 实现。已验证 `cd frontend; npm.cmd run build` 通过，并用临时 QA 页在内置浏览器检查 1280x900、390x760 和“安装游戏”展开表单交互，确认图标资源加载、无横向溢出、按钮文字不溢出、console error/warn 为空。
# FE-CARD-UNIFY-SAVES-1 状态
- `FE-CARD-UNIFY-SAVES-1` completed：除模组管理页外，Stardew 其他页面小框已统一为存档管理页卡片基准，使用暖色纸面、铜色 2px 边框、9px 圆角、内描边和轻微阴影；背景按最新反馈改为干净浅色高光 + 纯色纸面，不再铺密集点状纹理。同步收敛标题/说明字号、padding、gap 和窄屏容器查询，模组页 `.sd-mods-*` 主体卡片保持原状。已验证 `cd frontend; npm.cmd run build` 通过，并用已删除的临时 QA 页检查 1280x720/390x760 无横向溢出、无点状纹理、模组卡未误套新样式。
# FE-CARD-UNIFY-SAVES-1 follow-up
- `FE-CARD-UNIFY-SAVES-1` follow-up completed：总览页四个统计卡 `.sd-mc`（存档/模组/系统健康/运行任务）已按反馈仅移除点状 `radial-gradient` 纹理，保留原有结构、尺寸、边框、阴影和布局。已验证 `cd frontend; npm.cmd run build` 通过。
# FE-SERVER-PLAYERS-CARD-LAYOUT-1 状态
- `FE-SERVER-PLAYERS-CARD-LAYOUT-1` completed：服务器控制页已使用共享 `ServerSummaryCard` 替换原大状态卡并移除独立邀请码卡；玩家管理页已移除顶部摘要卡，将“服务器信息（Junimo）”置底；在线玩家表删除“角色”列，主机标识并入玩家名右侧，并新增可见“农场收入 / 玩家收入”列。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器 DOM 快照接口本次返回兼容错误，未完成截图式 QA。

# FE-PLAYERS-PROTOTYPE-CURRENT-1 状态
- `FE-PLAYERS-PROTOTYPE-CURRENT-1` completed：玩家管理页已继续按 version-02 `05-players.png` 原型校准卡片比例和首屏节奏，并通过玩家页专属 `.sd-main:has(.sd-players-page)` 收紧主 frame inset。在线玩家表为整行首块，标题改为 `在线: N` / `等待加入: N` 徽章；活动/最近事件与管理操作保持第二行左右两列，管理操作隐藏原型不存在的说明文字；Junimo 终端整行置底并在桌面首屏可见。已验证 `cd frontend; npm.cmd run build` 通过；内置浏览器 QA 覆盖 1536x1024 与 390x844，console error/warn 为空，桌面表格无自身横向滚动条，窄屏无页面级横向溢出。

# FE-MISSING-GAME-INSTALL-PROMPT-1 状态
- `FE-MISSING-GAME-INSTALL-PROMPT-1` completed：每次登录或已有 session 进入 Stardew 面板后，若实例状态未检测到游戏文件，会弹出“请先安装游戏”引导弹窗；主按钮跳转到 `/instances/stardew/install`。已安装状态、正在运行的 `stardew_install` 任务和当前已在安装页时不会触发该弹窗。已验证 `cd frontend; npm.cmd run build` 通过。

# STEAM-QR-PHASE-CLASSIFY-1 状态
- `STEAM-QR-PHASE-CLASSIFY-1` completed：Steam QR 扫码登录后端阶段识别已修正，`Choice [1]: 2` 不再误判为 Steam Guard 手机批准，QR 模式下保持 `steam_qr_required` 供前端展示扫码入口。现场诊断确认 steam-auth 容器基础网络可解析并连通 Steam Web API/CM 端口，当前失败属于 SteamClient QR 登录连接未建立而非 Docker 完全无网络。已验证 `cd backend; go test ./internal/games/stardew_junimo` 通过。

# FE-STEAM-QR-LOG-FALLBACK-1 状态
- `FE-STEAM-QR-LOG-FALLBACK-1` completed：安装页增加 QR 日志兜底，当后端阶段短暂误显 `steam_guard_mobile_required` 但最新日志证明用户选择 QR（例如 `Choice [1]: 2`）时，前端按 `steam_qr_required` 展示扫码交互，避免 QR 流程误显示 Steam Guard 手机批准。已验证 `cd frontend; npm.cmd run build` 通过。

# FE-STEAM-QR-IMAGE-CODE-1 状态
- `FE-STEAM-QR-IMAGE-CODE-1` completed：安装页 QR 弹窗不再把终端字符画作为主扫码对象，而是从最新 `Or open: https://s.team/q/...` 提取 Steam 登录 URL 并用本地 `qrcode` 生成标准二维码图片；字符画仅作为图片生成失败时的备用。已验证 `cd frontend; npm.cmd run build` 通过，后端 QR 阶段识别相关定向测试通过。

# FE-STEAM-AUTH-OPTIMISTIC-PHASE-1 状态
- `FE-STEAM-AUTH-OPTIMISTIC-PHASE-1` completed：安装页 Steam 认证选择已加入本地乐观阶段，点击扫码登录/手机批准/输入验证码后立即切换到对应交互区，不再等待实例状态轮询；当前有效的 QR URL 日志可修正落后的选择按钮，但后续 Guard/下载/失败日志优先。已验证 `cd frontend; npm.cmd run build` 通过，后端 QR 阶段识别相关定向测试通过。
# STEAM-POST-AUTH-RETRY-1 状态
- `BE-STEAM-POST-AUTH-RETRY-1` / `FE-STEAM-POST-AUTH-RETRY-1` completed：Steam 登录成功后的下载/CDN/磁盘/后续安装失败现在归类为 `error/download_failed` 或 `error/post_auth_failed`，前端通过日志兜底识别认证已成功，并只允许复用已保存凭据重试，不再要求用户重新输入 Steam 账号密码。已验证 `go test ./internal/games/stardew_junimo -run "DownloadFailedAfterSuccessfulAuth|InstallMarksSteamAuthFailedWhenRunErrors"` 与 `cd frontend; npm.cmd run build`。
# STEAMCMD-PULL-PROGRESS-1 状态

- `STEAMCMD-PULL-PROGRESS-1` completed：Junimo 镜像拉取与 SteamCMD 兜底镜像拉取都会通过 `[pull:progress:done:total]` 给前端提供估算百分比；安装页顶部总进度、镜像拉取卡和任务日志详情可展示“约 N%”，避免用户只能看 layer 日志猜测。已验证 `go test ./internal/games/stardew_junimo -run "SteamCMD|DownloadFailed|InstallMarksSteamAuthFailedWhenRunErrors"`、`go test ./internal/docker ./internal/games/stardew_junimo/config` 与 `cd frontend; npm.cmd run build`。
# FE-STEAMCMD-BRACKET-PROGRESS-1 状态
- `FE-STEAMCMD-BRACKET-PROGRESS-1` completed：安装页已支持 SteamCMD 原生 `[ 28%] Downloading update (done of total KB)` 进度日志，并继续支持 SteamCMD 手机 App 批准提示。验证：`cd frontend; npm.cmd run build`。
# JUNIMO-IMAGE-CANDIDATES-1 状态
- `JUNIMO-IMAGE-CANDIDATES-1` completed：`server` 与 `steam-auth-cn` 镜像拉取已接入与 SteamCMD 类似的候选兜底机制，默认顺序为 `docker.1ms.run`、`docker.m.daocloud.io`、`ghcr.io`、原始仓库；本地已有任意候选会直接复用，拉取成功后写回 `.env`，避免后续 compose 回到单一镜像源。验证见后端接手文档。
# JUNIMO-IMAGE-CANDIDATES-2 已完成

- 已完成 JunimoServer 与 steam-auth cn 版镜像候选源自动补齐：旧实例单候选 `.env` 会被扩展为默认候选；steam-auth cn 当前顺序为 1ms、阿里云 ACR 新版个人版、DaoCloud、GHCR、Docker Hub。
- 已完成选中镜像写回：后端会把实际使用的 `SERVER_IMAGE` / `STEAM_SERVICE_IMAGE` 和补齐后的候选列表写回实例 `.env`，方便后续重试复用。
# FE-GAME-INSTALLED-STARTABLE-1 已完成

- 已修复安装完成态 `game_installed` 在前端不可启动的问题：总览页和服务器控制页都会把它作为可启动的未运行状态展示。
- 若没有可用存档，点击启动后仍由后端返回 `save_required` 并引导用户创建/上传/选择存档。
# FE-OPSRAIL-METRICS-RESTORE-1 状态
- `FE-OPSRAIL-METRICS-RESTORE-1` completed：右侧 OpsRail CPU / 内存 / 磁盘已恢复轻量实时显示，Stardew 面板挂载期间调用现有 `/api/instances/:id/metrics`，首次立即采样并按 `2s` 刷新；没有用户打开前端页面时自然不会产生浏览器轮询，页面卸载时停止 timer。普通 dashboard 初始化仍不触发 `/api/health/diagnostics`，保留此前诊断降轮询优化。验证：`cd frontend; npm.cmd run build`；Browser QA 打开 `qa-layout.html?state=running` 确认右侧栏显示 mock metrics 百分比而非空值。
# RELEASE-TAG-CI-1 状态
- `RELEASE-TAG-CI-1` completed：面板仓库已新增 GitHub tag 发版 workflow，推送 `v*` tag 后自动构建面板 Docker 镜像，发布 Docker Hub 与阿里云 ACR 的版本 tag / `latest`，并在 GitHub Release 上传 `deploy/run.sh`。配套 steam-service-cn 仓库已改造 tag workflow，可发布 `junimo-steam-service-cn` 到 Docker Hub、阿里云 ACR 和 GHCR。
# RUN-SH-QUICK-MODE-1 状态
- `RUN-SH-QUICK-MODE-1` completed：`deploy/run.sh` 已扩展为完整一键启动菜单，包含 Docker/Compose 自动安装修复、面板镜像候选兜底拉取、启动/停止/重启、普通更新/强制更新、镜像源切换、脚本自更新、虚拟内存、开机自启、状态/日志/访问地址。脚本默认使用宿主机 `~/.anxi-panel/data` 与容器内同名绝对路径持久化数据，避免面板通过 Docker socket 编排游戏容器时出现 bind mount 路径不一致。
- `RUN-SH-QUICK-MODE-1` docs follow-up：README 与镜像构建文档已补充最低系统要求、推荐配置、多人游玩规格和云服务器安全组口径；对外只要求开放 TCP `8090`、UDP `24642` / `27015`，VNC/noVNC TCP `5800` 按需开放，Junimo API TCP `8080` 明确不要开放公网。
- `RUN-SH-QUICK-MODE-1` docs follow-up：README 与镜像构建文档已补充“一键启动脚本”的国内加速安装入口，推荐国内用户通过自有轻量服务器静态分发 `run.sh`，GitHub Release 地址作为备用；Docker 镜像仍由脚本内候选源拉取，不通过该轻量服务器中转。
- `RELEASE-TAG-CI-1` follow-up：面板仓库 ACR 发布地址已切换到阿里云新版个人版实例域名 `crpi-9z3bkb9g7fxeohrg.cn-hangzhou.personal.cr.aliyuncs.com`；GitHub Actions 和 `deploy/run.sh` 默认国内镜像源同步更新。`ALIYUN_REGISTRY_USERNAME` 使用 ACR 访问凭证登录命令中的 `--username` 值。
- `RELEASE-TAG-CI-1` follow-up：配套 `junimo-steam-service-cn` tag 发布 workflow 已切换到同一 ACR 新版个人版域名；面板内 `STEAM_SERVICE_IMAGE_CANDIDATES` 默认把该 ACR 镜像放在第二候选，顺序为 1ms、ACR、DaoCloud、GHCR、Docker Hub。
- `RELEASE-TAG-CI-1` follow-up：面板仓库 tag 发版 workflow 已增加 GHCR 发布目标 `ghcr.io/anxiyizhi/stardew-server-anxi-panel`，并给 `deploy/run.sh` 增加 GHCR 镜像源选项；配套 steam-service-cn workflow 保持发布 `ghcr.io/<owner>/junimo-steam-service-cn`。

# SAVE-POINTER-SUFFIX-HEAL-1 状态
- `SAVE-POINTER-SUFFIX-HEAL-1` completed（代码已修复+测试通过，尚未部署）：修复 JunimoServer 新建存档时把 `gameloader.json` 存档名前缀写错导致"当前激活存档"永久显示"未知"、新建存档轮询误报超时的问题，面板现在能按数字后缀自动识别并修正真实存档目录。详见 `docs/backend-handoff/backend-handoff-2026-07-07.md`。

# INVITE-COPY-CLIPBOARD-FALLBACK-1 状态
- `INVITE-COPY-CLIPBOARD-FALLBACK-1` completed（代码已修复+构建通过，尚未部署）：修复邀请码/局域网 IP 复制按钮在非 HTTPS 访问下因 `navigator.clipboard` 不可用而完全无反应的问题，新增 `execCommand('copy')` 降级方案。详见 `docs/frontend-handoff/frontend-handoff-2026-07-07.md`。

# FESTIVAL-EVENT-1 / JOJA-ROUTE-1 状态
- `FESTIVAL-EVENT-1`/`JOJA-ROUTE-1` completed（代码已完成 + 后端 build/vet/test 全绿 + 前端 typecheck/build 全绿 + 嵌入 SMAPI Mod 已用 Docker 重新编译替换，尚未真机联机验证）：服务器控制页新增"触发节日活动"（模拟游戏内 `!event`）和"永久启用 Joja 路线"（模拟 `!joja IRREVERSIBLY_ENABLE_JOJA_RUN`，需强确认弹窗逐字输入）两个按钮。后者因上游要求触发者持有 admin 角色，后端会先调用 JunimoServer 自带的 `POST /roles/admin` 把主机提升为管理员再模拟指令。详见 `docs/backend-handoff/backend-handoff-2026-07-10.md`、`docs/frontend-handoff/frontend-handoff-2026-07-10.md`。

# CABIN-STRATEGY-1 状态
- `CABIN-STRATEGY-1` completed（代码已完成 + 后端 build/vet/test 全绿 + 前端 typecheck/build 全绿，尚未浏览器实测和真机联机验证）：按用户明确设计口径把小屋策略设置分层——新建存档页只给简化二选一"小屋模式：推荐/原版"（`NewGameConfig.cabinMode`），服务器控制页新增"小屋与联机高级设置"弹窗给完整设置（`CabinStrategy`/`ExistingCabinBehavior`/`NetworkBroadcastPeriod`），新增 `GET/PUT /api/instances/:id/config/server-runtime-settings` 接口，两层共用同一份 `server-settings.json`。详见 `docs/backend-handoff/backend-handoff-2026-07-10.md`、`docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `CABIN-STRATEGY-1` 小节。

# APPROVE-PENDING-AUTH-1 状态
- `APPROVE-PENDING-AUTH-1` completed（代码已完成 + 后端 build/vet/test 全绿 + 前端 typecheck/build 全绿 + 嵌入 SMAPI Mod 已用 Docker 重新编译替换，尚未真机联机验证）：玩家管理页新增独立"待认证玩家"卡片，管理员可一键批准密码保护下卡在隔离小屋的玩家，不需要玩家自己正确输入 `!login <password>`。上游 JunimoServer REST API 没有对应端点（`GET /auth` 只有计数，没有名单/批准接口），改为让内嵌 `StardewAnxiPanel.Control` SMAPI 模组反射调用 JunimoServer 内部单例 `PasswordProtectionService.TryAuthenticate`——这是控制模组第一次真正反射进 JunimoServer 私有实现（而非公开契约、非游戏内聊天指令模拟）。新增模组启动时的"反射能力自检"（写入 `status.json` 的 `passwordBridgeAvailable`/`passwordBridgeDetail`），前端据此提前禁用"批准"按钮，而不是等用户点击后才发现没生效。新增 `POST /api/instances/:id/players/approve-auth`，`GET /players` 每个玩家新增 `isAuthenticated`。详见 `docs/backend-handoff/backend-handoff-2026-07-10.md`、`docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `APPROVE-PENDING-AUTH-1` 小节。

# PLAYERS-BAN-1 状态
- `PLAYERS-BAN-1` completed（代码已完成 + 后端 build/vet/test 全绿 + 前端 typecheck/build 全绿 + 嵌入 SMAPI Mod 已用 Docker 重新编译替换）：玩家管理页“封禁玩家”已接通。后续 command-result 阶段改为优先按 `uniqueMultiplayerId` 直接调用 `Game1.server.ban`，只有直接 API 不可用才降级唯一名字 `!ban`。用户已在真实实例确认 `Game1.bannedUsers` 随服务器容器重启丢失，UI 使用确定性限制提示；未做面板侧持久化、封禁名单或解封入口。

# PLAYERS-WARP-HOME-1 状态
- `PLAYERS-WARP-HOME-1` completed（代码已完成 + 后端相关包测试通过 + 前端 typecheck/build 通过 + 嵌入 SMAPI Mod 已用 Docker 重新编译替换，尚未真机联机验证）：玩家管理新增“回家”按钮，桌面端和手机端均放在“踢出”左侧；后端新增 `POST /api/instances/:id/players/warp-home`，由嵌入式 SMAPI 控制模组反射调用 JunimoServer `FarmerExtensions.WarpHome(Farmer)`，用于把在线 farmhand 传送回自己的小屋。该能力明确不复用 `TryAuthenticate`，因为已认证玩家不会再次触发认证传送。详见 `docs/backend-handoff/backend-handoff-2026-07-10.md`、`docs/frontend-handoff/frontend-handoff-2026-07-10.md` 的 `PLAYERS-WARP-HOME-1` 小节。
# FE-INSTALL-STEAM-AUTH-BUTTON-1 状态

- `FE-INSTALL-STEAM-AUTH-BUTTON-1` completed：安装页原“更换 Steam 账号 / 重新认证”已替换为总览页同逻辑的“登录授权”按钮，并改为常驻显示；两页现共用 `useSteamAuthLogin`。已验证 `cd frontend; npm.cmd run build` 通过。

# SAVE-BACKUP-GAMEDAY-1 状态

- `SAVE-BACKUP-GAMEDAY-1` completed（代码已完成 + 后端 build/vet/test 全绿 + 前端 typecheck/build 全绿，未做真机联机端到端验证）：存档回档功能重构为完全按"游戏内日期"（年/季/日）管理自动回档点，**取代 `SAVE-BACKUP-POLICY-1`/`SAVE-BACKUP-SCHEDULE-HOUR-1` 里的定时备份/每日快照机制**（取消定时备份，不再有"最新备份"/"每日快照"）。`BackupPolicy` 简化为 `{ gameSaveBackups, retainGameDays }`（默认保留最近 5 个游戏日，1-14 可调）；新增 `auto`/`predelete`/`prerestore` 三类备份 kind，`manual`（手动备份）不再占用自动回档配额；回档前保护备份失败会中止回档且不破坏当前存档；旧 `latest`/`daily`/`scheduled` 磁盘文件不删除，归入前端"其他备份→历史备份"展示。触发时机复用现有 SMAPI `GameLoop.Saved` 事件管线，**未改动、未重新编译嵌入 DLL**。前端"存档"页备份区拆成"游戏日回档"（主列表，按游戏日排序）+"其他备份"（手动/删除前/回档前/历史）两个区块，回档行按钮不再因服务器运行中被无说明地禁用，改为始终可点开确认弹窗并在弹窗内引导先停服。详见 `docs/backend-handoff/backend-handoff-2026-07-11.md`、`docs/frontend-handoff/frontend-handoff-2026-07-11.md`。
- `SAVE-BACKUP-GAMEDAY-MOBILE-1` completed（同日追加，前端 typecheck/build 全绿，未做真机/窄屏实测）：手机端存档页删除恒禁用的"回档"占位按钮，在"存档操作"卡片上面新增和桌面同名的"游戏日回档"卡片（仅管理员可见），复用桌面同一套 `getSaveBackups`/`restoreSaveBackup` API，数据字段和排序口径与桌面一致，仅把 6 列表格改成手机堆叠行展示；回档确认弹窗同样把"服务器运行中"从无说明禁用改为弹窗内醒目引导先停服。详见 `docs/frontend-handoff/frontend-handoff-2026-07-11.md` 的 `SAVE-BACKUP-GAMEDAY-MOBILE-1` 小节。
- `SAVE-RESTORE-AUTORESTART-1` completed（同日追加，后端 build/vet/test 全绿 + 前端 typecheck/build 全绿，未做真机联机端到端验证）：**取代上面两条里"服务器运行中回档需要先手动停服"的交互**——现在确认回档时，如果服务器正在运行，面板会自动完成"停止服务器 → 回档 → 重新启动服务器"整个流程，不需要用户离开弹窗手动操作。后端把这三步编排成一个 lifecycle job（复用现有 `doStop`/`doStart`，不重新实现 compose/Mod 同步/邀请码轮询），`POST .../saves/backups/restore` 新增 `autoRestart` 字段，运行中且传 `true` 时返回 `202 {jobId}`（和启动/停止服务器同一套 job 轮询/SSE 机制），已停止时行为不变。前端桌面和手机端回档按钮和弹窗提交按钮不再因服务器运行中被禁用，弹窗文案改为说明会自动停止/重启服务器。详见 `docs/backend-handoff/backend-handoff-2026-07-11.md`、`docs/frontend-handoff/frontend-handoff-2026-07-11.md` 的 `SAVE-RESTORE-AUTORESTART-1` 小节。

# FE-STARTUP-HOST-CONFIRM-1 状态

- `FE-STARTUP-HOST-CONFIRM-1` completed（同日追加，前端 typecheck 全绿，未做真机联机端到端验证）：修复两个用户反馈的问题。一是服务器控制页"启动/重启"按钮**切换过早**——原本纯按 `active stardew_lifecycle job + state=running` 判定完成（见上文 `FE-LIFECYCLE-BACKGROUND-INVITE-1`），现在叠加一层"主机玩家上线确认"：`state=running` 后，只要在线玩家列表里还没出现 `isHost && status==='online'` 的条目就继续按"启动中"展示，这个判断独立于是否点过启动按钮（刷新页面/换设备打开也生效），同时新增超时兜底避免像 2026-07-06 那次一样因为玩家快照闪烁/不可用导致按钮永久卡死转圈。**上线联调时改过三次**：第一版超时判断挂在了"仅当次点击启动才生效"的本地状态上，刷新页面就失效，已改成完全独立的派生状态；超时阈值也从最初拍脑袋定的 90 秒（用 `docker exec` 只读对比容器内 `status.json`/`players.json` 时间戳实测发现主机上线可能要几分钟）调大到 10 分钟；第三处是 `useStardewDashboardData.ts` 里服务器停止时没有清空在线玩家列表缓存，导致同一浏览器标签页"运行过→停止→再启动"时用旧快照误判主机已在线、按钮一点启动就切回正常态，改成离开 `running` 时同步清空。二是服务器停止后邀请码卡片会残留上一次运行时的旧邀请码——根因是 `refreshInstanceState` 只要后端返回的 `inviteCode` 非空就无条件写入本地状态，没检查 `state`，而后端 `doStop` 按设计不清空该字段；修复为只在 `state` 为 `running`/`starting` 时才采纳该字段，否则清空。局域网邀请（面板访问地址）本来就不受服务器状态影响，确认无需改动。详见 `docs/frontend-handoff/frontend-handoff-2026-07-11.md` 的 `FE-STARTUP-HOST-CONFIRM-1` 小节。
# FE-OVERVIEW-STARTUP-HOST-CONFIRM-1 状态
- `FE-OVERVIEW-STARTUP-HOST-CONFIRM-1` completed：总览页启动按钮现在会等待在线玩家列表中的主机真正在线后才脱离“启动中…”，刷新页面后同样生效，并保留 10 分钟玩家快照异常兜底。前端构建已通过，尚未做真实大存档端到端启动验证。
# PLAYER-OFFLINE-SAVE-FALLBACK-1 状态
- `PLAYER-OFFLINE-SAVE-FALLBACK-1` completed：离线玩家现在可从存档 Farmer 数据补齐最后睡眠位置、坐标和独立钱包收入，且不会覆盖已有运行时缓存；玩家缓存兼容基础 saveId 与带数字后缀的完整存档目录 ID，降低重启/切换标识后历史信息被整体丢弃的风险。后端全仓库测试通过，尚未在生产多人存档验证。
# PLAYER-ROSTER-SQLITE-1 状态
- `PLAYER-ROSTER-SQLITE-1` completed：新增 `save_identities` / `player_roster` / `player_events` SQLite 模型，以 `instance_id + stable_save_id + player_id` 为联合身份，持久化首次出现、最后在线、位置、收入快照和 seen/joined/left 活动；基础 `saveId` 会归一化到完整存档目录 ID。`players.json` 与存档 XML 保持事实输入，旧 `players-cache.json` / `players-events.json` 首次成功导入后删除且不再写入。API 名册与 `recentEvents` 结构不变，后端全量测试通过；真实升级实例仍待验证。
# FE-PLAYER-LOCATION-NORMALIZE-1 状态
- `FE-PLAYER-LOCATION-NORMALIZE-1` completed：新增共享位置格式化工具，统一桌面玩家表、最近事件、移动玩家页和总览展示；`FarmHouse/Cabin/Cellar/Shed/Barn/Coop` 等数字或 UUID 后缀实例名会映射为中文逻辑位置并附坐标，原始唯一名继续保留在 API、SQLite 和桌面悬停标题中。前端 typecheck/build 通过。
# FE-SHARED-WALLET-PERSONAL-INCOME-1 状态
- `FE-SHARED-WALLET-PERSONAL-INCOME-1` completed：玩家表共享钱包的个人收入从误导性的 `0g` 改为“共享模式不统计”，分开钱包个人累计收入与农场团队累计收入展示不变。
# REAL-INSTANCE-CRITICAL-FLOWS-VERIFIED-1 状态

- `REAL-INSTANCE-CRITICAL-FLOWS-VERIFIED-1` completed：用户已确认以下关键链路均完成真实实例验证：大存档启动并等待主机上线；运行中回档自动停止、回档并重启；多人认证批准、踢出、封禁和回家；睡觉后生成游戏日回档点；Steam/SteamCMD 授权与镜像候选降级。本状态覆盖对应旧条目中的“尚未真机/端到端验证”，但不覆盖未明确确认的移动端视觉、封禁跨重启持久化等独立验证项。

# FE-LIFECYCLE-STATE-MACHINE-1 状态

- `FE-LIFECYCLE-STATE-MACHINE-1` completed：总览页与服务器控制页已改为共用 `useStardewLifecycleState`，统一 active lifecycle job、driver stopping、启动/停止 pending、等待主机上线和 10 分钟超时的判定；启动、停止、重启及运行中回档自动重启均消费同一套派生状态。前端构建通过，未修改 API。
# UI-LIFECYCLE-STATUS-1（已完成，2026-07-11）

- [x] 后端提供七态 UI 生命周期语义，前端停止自行拼装状态。
- [x] 现有诊断页展示实例、Driver、SMAPI status/players 来源及更新时间。
- [x] 诊断页补充 Compose 快照、存档/缓存身份、控制文件新鲜度、启动阶段耗时和控制模组/Junimo 版本矩阵。
- [ ] 后续将当前基于三类更新时间差值的启动耗时升级为持久化阶段事件，支持跨重启历史趋势。
# FE-LIFECYCLE-LIVE-SIGNAL-PRIORITY-1 状态
- `FE-LIFECYCLE-LIVE-SIGNAL-PRIORITY-1` completed：在线玩家列表确认主机在线后立即结束启动中间态，不再等待邀请码或滞后的后端 `uiStatus`；点击停止后本地 pending 状态立即展示“停止中”。桌面总览、服务器控制页共享 hook 与手机总览均已统一。相关文件独立 TypeScript 校验通过；完整构建受工作区另一批未完成的 ServerControlPage hook 拆分影响。
# COMMAND-RESULT-PROTOCOL-1 状态

- `COMMAND-RESULT-PROTOCOL-1` 阶段 1 completed：控制命令已有稳定 commandId、命令/结果原子文件协议、`command-results/`、七状态与结构化错误码、`commandResultVersion: 1`、非阻塞提交和只读查询 API；旧控制模组继续兼容。结果闸门保证已有结果不重复消费，崩溃歧义返回 unknown 且不自动重试；终态结果采用 7 天 + 24 小时 expired 墓碑清理策略。嵌入控制模组已重新编译并更新 DLL。阶段 2 的玩家操作精确 succeeded/failed 尚未开始。
# PLAYER-COMMAND-RESULTS-1 状态

- `PLAYER-COMMAND-RESULTS-1` implemented：command result v1 已为 warp-home、kick、approve-auth 接入真实 succeeded/failed 回执，包含完整结构化错误码；桌面与手机共用 500ms/10s 轮询、旧模组兼容、按玩家 busy 和 unknown 不重试语义。ban、broadcast、event、joja 保持 dispatched，未提前扩展。
# BROADCAST-BAN-RESULTS-1 状态

- `BROADCAST-BAN-RESULTS-1` implemented：broadcast/say 已能确认交给游戏聊天系统后返回 succeeded；ban 优先 uniqueMultiplayerId + `Game1.server.ban` 精确调用，名字降级存在重名时拒绝且只能返回 dispatched。桌面/手机已覆盖 succeeded/dispatched/failed/unknown/旧模组。用户真机确认封禁随容器重启丢失；名单持久化与解封入口不在本阶段。event、joja、save-now 保持未接入精确结果。

# EVENT-JOJA-SAVE-RESULTS-1 状态

- `EVENT-JOJA-SAVE-RESULTS-1` completed：trigger-event 与 enable-joja 已区分明确失败、聊天 dispatched 和可持久确认的 succeeded；save-now 已通过 commandId tracker 关联 `GameLoop.Saved`，两分钟超时为 save_timeout，崩溃歧义为 unknown 且不重试。桌面/手机统一轮询与精确文案，新增游戏内保存入口并与 ZIP 备份区分。Go、前端和模组构建全绿，嵌入 DLL 已更新；现有存档实测 event 明确失败、Joja dispatched、save Saved succeeded。

# COMMAND-RESULT-PRODUCTIZATION-1 状态

- `COMMAND-RESULT-PRODUCTIZATION-1` completed：回执历史已 SQLite 化并支持幂等导入、面板重启恢复、安全文件交接、30 天/数量保留和最终审计；任务与日志页及诊断页已展示协议历史与卡死信号。旧模组继续退化为“已提交，无法获取精确结果”，没有新增任何游戏命令或改变命令保证语义。
# SAVE-BACKUPS-EMPTY-LIST-1（已完成，v0.1.12）

- 修复全新服务器零备份时存档页黑屏，恢复首次进入存档页创建新存档的主流程；后端空列表契约与前端兼容保护均已完成。
# INSTALL-RUNTIME-VERIFICATION-1（已完成，v0.1.13）

- 修复新服务器游戏文件未真正安装完成却被误判为“安装成功”的问题；安装、启动和状态协调统一验证完整游戏运行文件，Steam 授权操作不再覆盖安装错误状态。
# 2026-07-13 已完成：JUNIMO-STACK-UPDATE-1 阶段三

- 已完成 server + steam-auth-cn 不可拆分成对 apply：管理员固定确认、实例独占互斥、关键预检重跑、可信镜像精确 digest、认证卷私有快照、五字段原子配置、auth-first/server-second 验收、原运行态恢复和审计。
- 已完成失败成对回滚与重启恢复：恢复原配置/认证卷/旧 digest pair，终态为 `failed_rolled_back` 或 `rollback_failed`；不确定状态禁止猜测，后者保留材料并仅提供人工处理。
- 安全边界固定：不接受目标/服务/命令，不用 latest，不执行 `down -v`，不删除 game-data/steam-session/存档/Mods/settings/control，不复用 Panel updater。真实 Steam 账号与真实推荐上游镜像的发布前长流程验收仍是发布门禁。
# 2026-07-14 已完成：GAME-RUNTIME-VERSION-1 游戏与 SDK 只读版本检测

- 已完成 App 413150 / 1007 推荐矩阵、ACF tokenizer、只读 volume 检测、六态比较、管理员 API、诊断详情、tested 总览提示、只读预检以及手动候选发现 workflow。
- 推荐 buildid 随 Panel 发布且必须经过发布前验证；workflow 输出始终标记 `discovered`，只写 summary/artifact，不改 main、推荐矩阵或 tag。
- **阶段六未实现**：不创建 staging volume、不下载 depot、不执行 app_update、不写 game-data、不停服/重建容器，也没有游戏/SDK更新按钮。后续阶段须单独设计容量复核、账号授权、原子切换、兼容矩阵、健康验收和完整回滚。

## SMAPI 推荐升级子阶段（2026-07-14，已完成代码实现）

- 完成 Panel 内置 SMAPI 4.5.2 推荐 URL/SHA/大小与 game+SDK+Junimo+auth+Control 兼容矩阵；用户实例不追踪 GitHub latest，维护者候选发现不改矩阵/tag。
- 完成实际 game-data DLL/安装产物检测、七态 API、runtime-components 扩展、诊断 UI、严格无目标 POST、可信下载与 ZIP 安全验证。
- 完成显式 GAME_DATA_VOLUME staging 克隆、官方 Linux installer、精确验版、Control 联动、原子切换、完整 stack 验收、原状态恢复、失败自动回滚和重启恢复。
- 完成实际 game-data 容量估算、JunimoServer/Control 日志加载证据、Control 原先不存在时的精确回滚、初装与升级共用可信下载边界，以及隔离 staging installer Docker integration 测试。
- 完成玩家完整包推荐 installer/version/SHA 联动；增量包继续不携带 SMAPI，旧包不可变。
- **阶段八统一发布列车待办**：在真实 release-candidate Panel/Junimo/auth 镜像上跑完整长链路与故障注入；复核 4.5.2 对推荐 Stardew/SDK buildid 的游戏内兼容；生成并实机验证新版 Windows 玩家完整同步包；审查镜像 SBOM/签名与发布说明；之后才允许更新正式版本/tag/镜像。本次未提交、推送、打 tag 或发布包。

## 2026-07-14 阶段八统一发布列车状态

- 已完成：统一 schema v1、内嵌 recommended 快照、五组件精确版本与 digest/checksum、状态机与越级保护、同通道唯一 recommended 校验、withdrawn 新装/升级门禁、minimumPanelVersion 门禁。
- 已完成：取消 auth-cn 发布驱动的 repository_dispatch/自动 PR；每个 Panel 版本直接指定完整 server/auth 及其他组件版本，普通 PR 和 tag 发布继续运行无真实 Steam/registry secrets 的完整 CI 与隔离 Docker integration。
- 已完成：诊断页统一运行环境版本视图；实际更新仍拆分为 Junimo 对、游戏/SDK、SMAPI 三个安全事务。
- 后续发版只需：确认目标 Junimo server 和对应 auth-cn 精确版本，更新 Panel 内嵌组件清单及 digest/buildid/SHA，在本机 Docker Desktop 和 CI 完成测试，然后创建 Panel tag。用户升级 Panel 后收到对应组件升级提示。无需 candidate/tested/recommended 晋级、Environment reviewer、`APPROVED_STACK_VERSION` 或 Steam E2E artifact。当前工作未创建 tag、PR、镜像或生产变更。
# 2026-07-14 更新安全审查问题关闭

- [x] SMAPI 使用真实 Compose 状态决定停服，并在停服后/切卷前 Panel 中断时恢复原运行服务器。
- [x] SMAPI 回滚增加旧栈全链路验收，验收失败保留 `rollback_failed` 与恢复材料。
- [x] Junimo 回滚记录运行容器真实 ImageID，回滚重建固定不可变 ID；auth-only 状态漂移先停服再快照。
- [x] 兼容矩阵 CI 对基线与当前目录按 stackVersion 强制逐级迁移，push 分支修正为 `main`；发布验收记录绑定当前仓库、当前提交、成功 run 与当前 stack 的未过期 artifact。
# 2026-07-14 已完成：真实镜像 inspect 与 .NET auth 探针兼容

- [x] Docker 镜像/容器元数据读取缩小为格式化安全字段，真实 `sdvd/server:1.5.0-preview.121` 不再因环境变量脱敏导致 JSON 解析失败。
- [x] steam-auth ready/ticket 探针移除 Node.js 运行时假设，并用无 Node fixture 与真实 `steam-auth-cn:1.5.0-anxi.2` 验证。
- [ ] 真实已登录 Steam 测试账号的 session 保持仍属于发布 Environment 长链路验收，不能用无凭据探针替代。
# 2026-07-14 已完成：JUNIMO-UPDATE-PROGRESS-1

- [x] 修复 `.121/.125` 均无 `wget` 导致的新版本验收与旧版本回滚连续误判，统一改用 Junimo 镜像已有的 Bash health 契约。
- [x] Junimo dry-run/apply 输出镜像层下载进度，并保留初始失败与具体回滚失败原因。
- [x] 版本维护卡内一键展示校验、下载、安装和验收；技术详情降级为开发者排障信息，失败状态不再被“无需处理”覆盖。
- [x] 游戏/SDK 只展示已有安全预检，不把未实现的 apply 伪装成可在线升级。
# 2026-07-14 已完成：RUNTIME-VERIFY-FIFO-1

- [x] 定位 `.125` 健康运行仍等待五分钟的根因：非 TTY `attach-cli` 验收必然失败。
- [x] 改用正式 FIFO 控制通道验证 `info` 新响应，保留全部其他验收门槛与自动回滚。
- [x] 增加成功路径禁止 `attach-cli` 和控制契约失败回滚测试。
# 2026-07-14 已完成：FE-COMPONENT-UPDATE-GENERATION-1

- [x] 修复历史 dry-run `succeeded` 导致新 dry-run 与 apply 同时 POST 的竞态。
- [x] Junimo 与 SMAPI 均绑定新预检任务 ID，并阻止同一 ID 重复提交 apply。
- [x] 修复历史失败 apply 覆盖较新预检进度的问题。
- [x] 增加纯状态测试、本地延迟竞态 QA 场景，并将测试加入两个正式 CI/发布门禁。
