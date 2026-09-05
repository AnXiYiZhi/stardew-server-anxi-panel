## 2026-09-05：世界加入地址跟随面板访问地址（已修复，未发布）

- `GameLibrary.tsx` 将 `window.location.hostname` 同时传给加入地址显示与复制；`game-library-state.ts` 使用该主机名和当前世界连接信息中的 `gamePort` 生成地址，与详情页直连地址来源一致。IPv6 已有方括号时不重复包裹。
- `GET /api/instances/:id/public-ip` 的响应契约保持原样，卡片仅使用其中的游戏端口；浏览器的面板 HTTP 端口不用于游戏连接。端口无效、连接读取失败/加载中、未安装或待建档时保持原有不可复制状态。
- 验证：`test:game-library` 覆盖探测 IP 与访问地址不同、IPv4/域名/内网/IPv6、世界端口、空主机与非法端口；`test:responsive-layout` 和 `npm --prefix frontend run build` 均通过。
- 本次修改同步到 main，尚未进入正式镜像；下一次发布需将加入地址纳入该候选的专项验收。

## 2026-09-05：官网展示 v0.7.0

- `website/docs/changelog.md` 置顶 v0.7.0，说明游戏库、多世界管理、安装授权和状态恢复，链接正式 Release；v0.6.1 保留为历史版本。
- `website/docs/index.md` 的 release、版本入口和 CURRENT RELEASE 同步为 v0.7.0。现有主题与页面结构保持一致。
- Node 24 Alpine、独立依赖/产物卷的 VitePress production build 通过（6.61s）；提交 `64b8443b43e0e58fd453972003e35464bcec52ad` 的 Pages `33965514760` 构建/部署成功。线上首页与 `/changelog.html` HTTP 200，正文确认 v0.7.0 为最新；本地构建 HTML 确认版本顺序 v0.7.0/v0.6.1。两个测试卷已按归属清理，本轮为正文与构建验证。

# v0.7.0 正式发布完成（2026-09-05）

- 已发布游戏库、多世界创建/改名/删除、迁移 014–016、创建 token/journal 互斥与恢复、共享 Steam 下载、世界安装授权路由、过期会话与首次导入 journal 修复。完整提交 `baaee1b2a0c36609553d420b8f30dc909f23c069`。
- 候选 `33962015399`（约 16m24s）、自动 annotated tag `33962764623`、正式提升 `33962772040`（job 1m14s）全部一次成功；独立 Compatibility `33962015302` 成功。用户指定 v0.6.1 Web 升级链，真实下载人工验收由用户确认。
- 全量代码门禁、fresh/restart、同候选 unhealthy 回滚、SQLite/长期数据/非目标资源保持和升级后真实 Web 多世界创建/改名/删除通过。前端全部状态脚本、production build、网站及真实 SMAPI/Junimo 条件门禁通过；运行栈 manifest 未变，由路径门禁自动跳过远程制品校验。
- 三仓 0.7.0/latest 唯一 digest=`sha256:ad529ad0615b3349a7e6e62e9c8167ef5eab4139a5442c1d6cd398a1f48f17db`，OCI 与正式 health/version 冒烟通过；GitHub Release/四项脚本资产完整。本机任务容器和四个缓存卷清零，候选 DinD/夹具按脚本回收。
- 本轮修复的 Windows journal 原用例连续 30 次通过，并发读写 10 次通过。预检两项负载超时经完整包复验及干净 CI 通过；正式门禁没有失败重跑。只读 GitHub EOF 已有界恢复并记入错题本。
- 下面的实现记录保留当时阶段状态；以上发布结果为当前权威状态。后续不得移动 v0.7.0 tag，发布后文档提交不重建镜像；完整验证矩阵、日志和资源清理证据见 `docs/09-image-build.md`。

# v0.7.0 候选前端门禁（2026-09-05，未发布）

- Node 24 Alpine 使用独立依赖/产物卷，执行当前 package.json 全部 test:*、production audit、production build 成功；Vite build 2.46s。安装授权、游戏库、世界删除、session-expiry 均在本次完整回归中。
- `scripts/run-release-gates.sh` 与 Compatibility workflow 同步列入 game-library、session-expiry、world-delete，保证新增测试进入后续正式候选。发布升级来源固定 v0.6.1，完整候选、升级后 Web 多世界与回滚结果见 `docs/09-image-build.md`。

# Review 前端修复接手（2026-09-05，未发布）

- `stardew_steam_auth` 按 job 状态独立显示 Guard/手机等待/终态，已安装本体不再结束授权界面；失败通过目标世界的 `steamAuthLogin` 重试。`/instances/:id/install` 经 App 和 GamesPage 保留 id；安装修复、日志、状态、Guard 与重试不回落默认世界。轮询增加在途保护、失效隔离、归属校验和快照合并。
- 影响 `App.tsx`、`app-routes.ts`、`GameLibrary.tsx`、`GameInstallRail.tsx`、`stardew-routes.ts`、`install-progress-presentation.ts` 及安装/游戏库/响应式回归。新增 QA invite 三种场景固定第二世界，授权错误目标和重装请求会被夹具拒绝。
- 全部前端状态回归与 production build 通过；Browser 验证第二世界验证码提交完成、手机等待、失败后重新授权、文件缺失后重新安装，路由保持 river-farm。下一步扩展安装导航时继续显式传目标 ID；mock 证据不等于真实 Steam 授权。原环境未部署此轮变更，后端还需要 migration 016。

# WORLD-DELETE-1：长按彻底删除世界（2026-09-05，未发布）

- `/games/stardew` 的管理员非默认世界封面支持 1.2 秒长按，进度描边与提示同步展示；普通短点击仍进入世界，进入按压后的提前松手、移出、超过 10px 拖动、滚动、pointercancel、失焦/切后台取消并吞掉关联点击。事件仅绑定整卡入口按钮，改名/复制/启停子控件不参与。Delete 键为等价入口，旧后端缺少 `isDefault` 时隐藏删除能力。
- 长按完成打开原生 modal dialog，完整显示世界名称与“将永久删除此世界及全部备份，无法恢复。”。默认焦点为取消，支持 Escape、焦点限制和取消后回到封面；确认后显示“删除中…”并通过 ref 防止同步双击重复提交。失败保留错误与确认重试；只有 DELETE 204 后移除卡片并同步世界计数。持久 `instance_deleting` 卡片提供重试并禁用改名/启停。
- 保留既有 64px 名称/编辑区、两行省略、整卡全名 title 和手机横向画布。对话框使用顶层 portal，在 390×844 内完整换行、无页面横向溢出。
- 文件：`GameLibrary.tsx/.css`、`WorldDeleteControl.tsx`、`world-delete-gesture.ts`、`api.ts`/`types.ts`；`test:world-delete` 已进入 release gates。全量 package.json `test:*` 与 production build 通过。Browser/CUA 在独立 4623 夹具以实际 React handler 运行按压、取消、子控件、确认、防重复提交与成功移除回归，1280×720 和 390×844 均 PASS，干净标签控制台无 warning/error。`deleteQa=failed` 验证错误保留和可重试；`worldNames=mixed` 验证全名。
- 复验入口：`qa-layout.html?surface=app&route=/games/stardew&state=stopped&deleteQa=gestures`，点击“运行删除交互回归”；该页面只使用合成 fetch，刷新/HMR 后务必重新进入完整 harness URL。普通用户与默认实例没有入口。后端/API 契约和真实 Docker 验证见 `docs/02-backend.md`；原 8090 未重启，需同步后端才在真实 3000 页面生效。

# WORLD-CARD-SIZING-1：名称长度不改变世界卡片尺寸（2026-09-05，未发布）

- `GameLibrary.css` 为世界标题/改名表单预留统一 64px 区域，名称最多两行（固定行高、超长省略、允许连续英文断行），状态不换行。改名表单使用两列动作布局，输入/保存仍在同一预留空间；卡片原有断点宽度、图片比例和像素风保持不变。
- `GameLibrary.tsx` 的整卡入口增加完整名称 title，原可访问名称继续保留全文。未截断实际数据、未增加 JS 文本测量或 resize 监听、未修改真实世界名称。影响仅为卡片展示，后端 API 无变更。
- `qa-layout-main.tsx` 的 `worldNames=mixed` 夹具提供英文与长中文名称，并让实例 state 与目录返回一致；`test:responsive-layout` 固定两行、预留空间、编辑布局与全文提示契约。响应式、游戏库回归及 production build 通过。
- Browser 验证：1280×800 下两张夹具卡均为 230.39×373.42px，单字、长中文、40 个连续英文字符与进入/保存改名表单后尺寸不变；地址/按钮偏移一致。390×844 保持既有横向手机画布，两卡布局尺寸均约 203×298px，root scrollWidth=clientWidth=390；暖昼/静夜截图正常。
- 原 QA 标签在编辑夹具期间记录一次 createRoot 的 HMR 重执行告警；修改完成后用全新标签验收，页面身份、有效 DOM、无框架错误遮罩与控制台 warning/error 均通过。用户真实 3000 页面复核两卡同为 244×384.06px，地址/按钮偏移一致；改名交互只在 mock 夹具进行。
- 下一步注意：新增标题操作时继续复用预留区域，不根据名称长度计算卡片尺寸；异常信息不应被名称截断规则遮住。本轮不构建/替换 Docker 镜像。

# SESSION-EXPIRY-1：登录失效退出旧世界界面（2026-09-05，未发布）

- `api.ts` 的受保护请求遇到 401 通知 `App.tsx` 清除当前用户并显示“登录已失效，请重新登录后继续。”，保留原路由以便重新登录后继续。登录/初始化接口的 401 保持表单内错误，不自动调用 logout。
- 新增 `auth-session-events.ts` 使用单一订阅集合及会话代次：重复 401 只通知一次，已完成新登录后到达的旧请求不能注销新会话；卸载时取消订阅，不保存凭据。
- `test:session-expiry` 覆盖通知、取消订阅、真实 request 的 401 分流及跨登录延迟响应；连同 `test:game-library`、`test:lifecycle-action-state`、production build 通过。Chrome/CUA 的 `qa-layout.html?surface=app&auth=expired&route=/games/stardew` 夹具确认由世界视图返回带提示的登录表单。
- 真实验收：8090 后端重启后，3000 页面两个世界均显示“已停止”，两个启动按钮可用；数据库状态均为 `stopped/game_files_restored`，原游戏容器保持停止。此项解决过期会话的显示反馈，不宣称隔离不同 localhost 端口的同名 Cookie。

# INSTALL-LOGIN-ERROR-1：登录错误原因展示（2026-09-05，未发布）

- 本机真实失败任务记录 `Invalid Password` 与退出码 5；`install-progress-presentation.ts` 现在只从当前失败安装 job 的日志识别 Invalid/Incorrect Password 或 password check failure，显示“Steam 账号或密码错误，请修改后重试。”。其它任务的历史错误不参与判断；`credentials_required` 时优先保留后端状态原因，不被笼统 job 退出码遮住，验证码错误也不强行归因为账号密码。
- `test-install-state` 增加错误密码、跨 job 污染和验证码原因保留断言；回归、production build 通过。CUA Browser 的 `installQa=bad-password` 夹具证明提示可见、重试可进入编辑表单、控制台无 warning/error。未在浏览器代用户提交真实凭据。
- 已部署 18091 的 `install-test-20260905-login-error`，实际 HTTP bundle `index-tVwGoLWI.js` 含明确提示，health=ok；配置与数据挂载保留。后续修改安装错误展示继续按当前 job 归属处理日志，部署详情见镜像文档。

# STEAM-GUARD-FEEDBACK-1：错误验证码反馈（2026-09-05，未发布）

- 本机安装容器实际返回 `That Steam Guard code was invalid.`，原 runner 未识别，实例仍保留初始输入提示。`installer.go` 现将该可重试拒绝写入 `steamcmd_guard_required` 的 `stateMessage`，后续重复 prompt 不覆盖错误；仍在同一任务等待新验证码，成功登录/下载正常推进，既有终态凭据错误分流保持不变。
- `GameInstallRail.tsx` 将任务轮询错误与提交错误分开，成功轮询不再清除提交失败提示；提交成功显示等待验证说明，按钮提供正在提交状态。API 路径与 DTO 不变，继续使用 Guard input POST 与 job/state 轮询。
- 验证：`TestSteamCMDRejectedGuardRemainsRetryableAndCanDownload`、既有验证码 prompt/手机超时定向 Go 测试通过；前端 `test:install-state`、`test:responsive-layout`、production build 通过。CUA Browser 在 127.0.0.1:4621 的 `installQa=guard-code` 隔离夹具提交错误码，确认错误提示跨轮询保留、重新输入后按钮可用、控制台 warning/error 为空。
- 部署补充（2026-09-05）：18091 已替换为 `install-test-20260905-guard-feedback`，revision=`381e395d4df322d666b53e0a40cccc188fe7fae9-dirty-guard-feedback`、build date=`2026-09-05T05:49:52Z`。原配置/session secret 与数据挂载逐项一致，health/database=ok、initialized=true，实际 HTTP bundle `index-Bd__tfs8.js` 含提交反馈。原等待任务因重启结束，用户需刷新页面后重试安装；未代用户提交真实验证码。18090 原 Panel 的 healthy 与启动时间保持不变。此为本地测试部署，不是正式发布。

# FE-INSTALL-OVERALL-PROGRESS-1 前端接手记录（2026-09-05，未发布）

- 失败重试补充：`GameInstallRail.tsx` 使用 `gameInstallStepProgressLabel` 将失败/取消显示为“请重试”，停止详情不定动画，并以 active mode 约束验证码、手机等待及自动 choice。job 创建后保留当前组件的三项凭据，重试表单可直接修改且密码恢复掩码；完成清空，组件卸载或刷新丢弃，不持久化或回读后端密码。
- 验证：`installQa=retry` 构造 failed job 加陈旧 mobile phase，Browser 确认无“进行中”、无手机等待且进度 animation=none；点击重试后账号与两个掩码密码保留，安装按钮可直接提交。`test-install-state` 覆盖超时/取消/失败仍有数值，`test-responsive-layout` 固定保留与清理边界。旧页面已经清空的值不在回填能力范围。
- 改动：`GameLibrary.tsx/.css` 主卡以固定边框和“总进度约 N%”呈现整体估算，同步 progressbar 与按钮的可访问文本。`install-progress-presentation.ts` 新增纯派生 `overallPercent`，详情原 `percent` 不变；步骤图标保留独立的 `game-install-step-spin` keyframe。
- 进度边界：准备 0%，服务镜像 5–15%，SteamCMD 工具 15–20%，授权 20%，游戏 25–85%，SDK 85–90%，SMAPI 90–99%；权威完成态才显示 100%。无日志时固定在阶段边界；客户端自更新的 100% 不表示游戏完成。
- 接口：没有后端变更，继续复用原 job/state/logs。测试文件为 `test-install-state.ts`、`test-responsive-layout.ts`；安装状态、响应式、游戏库测试及 production build 通过，Browser 夹具中步骤 42% 对应整体 50%，收起/展开恢复相同值且边框 animation=none。
- 下一步：新增 phase 必须同时维护步骤、整体权重和边界测试。整体是明确标为“约”的阶段估算，不代表时间/字节；详情缺少遥测时继续返回 `percent: null`。不得用定时器补涨或在最终校验前显示 100%。

# FE-STEAM-CREDENTIAL-AUTO-GUARD-1 前端接手记录（2026-09-04，未发布）

## 改了什么

- `GameInstallRail.tsx` 与路由兼容用 `InstallPage.tsx` 只呈现账号密码登录：新任务不显示登录方式或 Guard 类型选择；直接验证码提示显示输入框，手机确认提示显示 Steam App 等待。
- 三个旧 choice phase 通过现有 Guard input 各自自动提交一次默认选项，避免旧后端任务卡在菜单；旧二维码活动 phase 只提供取消重开提示。`qrcode` import、状态、弹窗渲染和 npm 依赖已删除。
- `GameInstallRail.tsx` 把无当前 job 的目录 `install_verification_failed` 与游戏级 `installed=false,requiredFiles=missing` 传入进度 presentation，使机器字段压过实例残留的完成态；避免同游戏其它世界完整或页面刷新后，修复页误显示“安装完成 100%”。
- `install-progress-presentation.ts`、`qa-layout-main.tsx`、`test-install-state.ts` 与 `test-responsive-layout.ts` 同步固定自动推进、无选择按钮与缺文件优先级契约。

## 如何验证、下一步注意事项

- `npm run test:install-state`、`npm run test:responsive-layout` 与 `npm run build` 通过。真实本机修复页只显示 Steam 账号、Steam 密码、VNC 密码，空表单的“开始安装”禁用；页面文本无扫码入口，console warning/error 为 0。
- 验收停在提交前，没有写入真实 Steam 凭据或触发安装。以后新增 Guard phase 时继续由后端真实提示决定 code/mobile 展示，不在前端猜测挑战类型。

# FE-WORLD-LIFECYCLE-POLLING-1 前端接手记录（2026-09-04，未发布）

## 改了什么

- `GameLibrary.tsx` 将世界卡启停期间的 1.5 秒刷新从 `catalog.refresh` 改为 `catalog.refreshInstance(instanceId)`。定向刷新只取当前实例状态和全局 jobs 后筛选目标实例，保留目录、其它卡片与已有 connection；同实例 in-flight 请求去重，因此慢 `/state` 不会堆积轮询。
- 卡片徽标在生命周期 control 为 busy 时直接使用同一 `启动中/停止中` 投影，并消费 active lifecycle job 的可选 `operation`。排队中的 stop 即使 state/uiStatus 还是旧 `running/ready`，也明确显示“停止中”；旧后端未返回 operation 时兼容 system-owned stop 形态。完整目录刷新仍只用于初始读取、显式重试与安装流程收敛。

## 如何验证、下一步注意事项

- `npm run test:game-library`、`npm run test:lifecycle-action-state`、`npm run test:responsive-layout` 和 `npm run build` 通过。慢 1.6 秒 QA 夹具覆盖 stopped→启动及 running→停止：过渡期分别保持正确方向和原 `IP:port`，停止任务不再显示“启动中”，随后稳定到权威终态。
- 后续若 jobs 支持按 instance 查询，可把定向刷新改用过滤接口；在此之前不要为轮询重新调用完整 catalog 或 `/public-ip`，加入地址应只在初次载入、显式刷新或其自身失效策略下更新。

# FE-GAME-LIBRARY-INLINE-INSTALL-1 前端接手记录（2026-09-04，未发布）

## 改了什么

- `App.tsx` 将 `stardew-install` 纳入 `GamesPage` 的同页路由分支，移除 `StardewGlobalInstallPage` 渲染器；`/games/stardew/install` 和 `?jobId=` 仍保留为兼容、可刷新恢复的 URL，但画面变成星露谷主卡右侧的内联安装卡。
- 新增 `frontend/src/games/GameInstallRail.tsx`：未安装态显示 Steam 账号、Steam 密码、VNC 密码和安装按钮；提交后复用 `installInstance`、安装目标、409 job 接管、Guard 输入、任务详情/尾页日志轮询和 catalog 刷新。失败可原地回到凭据表单，完成可进入世界选择。
- 新增 `frontend/src/games/stardew/install-progress-presentation.ts`，把权威 installation classifier、`stardew_install` job 与既有日志解析组合为五步展示。`install-helpers.ts` 新增公共 `extractPullProgress`；具体百分比只来自 pull、Steam/SteamCMD 或 SMAPI 的真实进度日志，其他阶段保持 indeterminate。
- `GameLibrary.tsx/.css` 让内联安装卡复用世界轨道的测量、整体居中和对称开合动画；主卡外沿增加方形 360° 进度环，4px 环内沿与卡片边界精确相接，不定态只旋转渐变角而不旋转方形盒。通用 selected 外框也改为 3px 外扩配 3px 边框，内沿直接贴住卡面；安装态隐藏 selected 外框并由真实进度环接管。暖昼/月夜、固定横向手机画布、44px 控件、局部横滚与 reduced-motion 保持原契约。
- `GameInstallRail.tsx` 的 Steam 密码与 VNC 密码现在各自带 44×44px 眼睛按钮，默认掩码、可独立显示/隐藏，并通过动态 `aria-label` 和 `aria-pressed` 暴露状态；按钮只改变输入呈现，不触碰现有安装 API、凭据清理和日志安全契约。
- Steam 登录与 Guard 验证由后端阶段自动推进：旧 choice phase 自动提交账号密码/手机批准选项，直接验证码与手机确认提示继续在安装详情卡内原位呈现。
- `qa-layout-main.tsx` 增加未安装 POST fixture、可轮询安装 job/日志、`installQa=progress` 及 `installQa=auth-method`。`test-install-state.ts` 覆盖空闲、无百分比授权、42% 下载、完成与失败；`test-responsive-layout.ts` 固定同页路由、三项凭据、真实解析、贴边环绕进度、授权自动推进及 standalone renderer 不再出现。

## 影响接口与文件

- 后端接口没有变化：继续使用 `GET /api/games/stardew/installation`、`GET /api/jobs`、`GET /api/jobs/:id`、`GET /api/jobs/:id/logs?latest=true`、`GET /api/instances/:id/state`、`POST /api/instances/:id/install` 与 `POST /api/instances/:id/steam-guard/input`。
- 主要文件：`frontend/src/{App.tsx,qa-layout-main.tsx}`、`frontend/src/games/{GameLibrary.tsx,GameLibrary.css,GameInstallRail.tsx}`、`frontend/src/games/stardew/{install-helpers.ts,install-progress-presentation.ts}` 及安装状态/响应式测试。旧 `InstallPage` 源码未再进入 App 路由或 production chunk，其独立页面外观不再可达。

## 如何验证

- 自动门禁：从 `frontend` 运行 `npm run test:game-library`、`npm run test:install-state`、`npm run test:responsive-layout`、`npm run build`。
- 桌面 Browser：用 `surface=app&route=/games&role=admin&state=uninitialized&installDiagnostic=not-installed&instances=single` 点击主卡，填写三项 QA 凭据并提交；确认 URL 为 `/games/stardew/install?jobId=job_qa_stardew_install`、步骤为“下载游戏文件”、进度为 42%、主卡进度边界清晰，root/body `scrollWidth === clientWidth`，console 无 warning/error。
- 进度直达：追加 `installQa=progress` 可直接恢复同一 42% job。390×844 使用固定横向画布，安装详情可在内部横向轨道继续浏览；`prefers-reduced-motion` 的 animation/transition 禁用由 CSS 和静态门禁固定。
- 授权直达：追加 `installQa=auth-method`，确认旧登录 choice 自动进入 Guard choice、再自动进入手机批准等待；页面不显示可选登录或验证按钮，也不能跳回独立安装页。

## 下一步注意事项

- 后端新增安装阶段时，同步维护 `install-progress-presentation.ts` 的步骤映射和 FE-INSTALL-OVERALL-PROGRESS-1 整体权重；没有可验证日志数值时详情继续返回 `percent: null`，整体停在阶段边界并标为估算。
- 不要把内联安装卡重新接成独立页面，也不要绕过 `installationTargetId` 用任意世界实例作为全局安装目标。安装凭据不得进入 URL、日志、错误对象或 QA 输出。
- 如以后删除旧 `InstallPage.tsx/.css`，需先迁移其中仍由历史状态回归固定的 Steam 邀请授权/修改凭据测试；本次只移除可达页面和 production import，避免无关大范围删除。

# FE-GAME-LIBRARY-WORLD-CARD-CONTROLS-1 前端接手记录（2026-09-04，未发布）

## 改了什么、影响哪些接口/文件

- `WorldChoiceCard` 将世界主入口、地址复制和生命周期控制拆成三个同级按钮。透明的 `.world-choice-open` 仍让整张可视卡片进入现有实例目的地；右侧 `.world-copy-button` 只复制 `stardewJoinAddressValue` 证明有效的真实地址；底部 `.world-lifecycle-button` 是卡内唯一文字操作，昼夜各有匹配当前像素纸面的样式和独立 focus-visible。
- `StardewCatalogItem` 增加当前实例的 `activeLifecycleJob` 投影。目录仍先显示 `GET /api/instances` 的卡片，再分别回填 jobs/state/public-IP；同一次 jobs 结果继续经 `canonicalInstallJobs` 计算安装任务，同时只提取当前实例 queued/running 的 `stardew_lifecycle` job。`stardewWorldLifecycleControl` 组合安装分类、`state`、`uiStatus`、`driverPhase`、active job、当前提交意图和角色权限，不从显示文字推断运行态。
- 启动/停止分别复用 `startInstance(item.instance.id)` 与 `stopInstance(item.instance.id)`。提交中维持“启动中…”或“停止中…”并每 1.5 秒调用现有 `catalog.refresh`；启动沿用 `shouldClearPendingStartupAction` 的 active-job 终态规则，停止只在后端进入稳定停止/需存档/错误状态后解锁。保存前置错误导航至当前实例 saves，其他错误保留在当前卡片。
- 主要文件为 `frontend/src/games/{GameLibrary.tsx,GameLibrary.css,game-library-state.ts}`、`frontend/src/qa-layout-main.tsx`、`frontend/scripts/test-{game-library-state,responsive-layout}.ts`。没有改任何后端 API、实例面板组件、安装页或存档业务契约。

## 如何验证、下一步注意事项

- `npm run test:game-library`、`npm run test:responsive-layout`、`npm run build` 均通过。应用内 Browser 在管理员场景确认复制成功且路由不变，运行世界完成“停止 → 停止中… → 启动 → 启动中… → 停止”；普通用户按钮为原生 disabled 且无新建入口。暖昼、月夜、844×390 横持和 390×844 侧转画布均完成目视检查，console 只有 Vite/React 开发提示，无 warning/error。
- 新增卡内动作时必须继续保持同级按钮结构，不能把辅助按钮嵌入整卡入口。复制值必须来自 `stardewJoinAddressValue`，不能复制加载/错误占位；生命周期必须继续消费实例状态和任务终态，不能在请求返回后直接假定服务器已启动或停止。若后续为停止增加玩家提醒，应在不复用实例面板视觉的前提下共享真实在线人数和现有确认语义。

# FE-GAME-LIBRARY-INLINE-CREATE-1 前端接手记录（2026-09-04，未发布）

## 改了什么、影响哪些接口/文件

- `GamesPage` 新增路由驱动的 `createWorldOpen`。`/games/stardew/new` 不再挂载独立 `StardewNewWorldPage`，而是与 `/games/stardew` 共用同一游戏库和已展开的世界轨道；管理员点击末尾正方形卡时，该卡位切换为 `InlineWorldCreateCard`，直接显示名称输入框、取消和创建按钮。
- 内联表单复用 `getStardewGameInstallation`、`createInstance` 和 `stardewCreateInstanceRequest`。真实安装未确认前禁止提交；成功后读取响应中的 instance ID 并进入 `/instances/:id/saves`。创建请求、自动实例 ID、driver provision、存档语义及安装/修复状态机均未改动。
- 普通用户直达创建深链会 replace 回 `/games/stardew`，且轨道不渲染创建卡。管理员取消或按 Escape 只关闭编辑态、保留世界轨道，焦点回到重新出现的“新建世界”卡；再次点击星露谷主卡仍按原 360ms 对称动画收起整个世界轨道。
- 编辑卡沿用现有昼夜像素材质，输入和操作按钮最小高度为 44px。轨道打开时按内联卡的真实 offset 计算所需滚动，不用旋转后的视觉坐标；390×844 固定侧转画布与 844×390 横持画布都能完整显示表单。主要影响 `frontend/src/{App.tsx,app-routes.ts}`、`frontend/src/games/{GameLibrary.tsx,GameLibrary.css}` 和 `frontend/scripts/test-responsive-layout.ts`。

## 如何验证、下一步注意事项

- `npm run test:game-library`、`npm run test:responsive-layout`、`npm run build` 通过。Browser 在 1360×900 验证暖昼/月夜内联表单、名称输入、创建按钮启用及创建后 `/instances/stardew-3/saves`；Escape 回 `/games/stardew` 后焦点归还。普通用户直达 `/games/stardew/new` 自动回退且无创建入口。
- Browser 在 390×844 和 844×390 都验证内联表单完整可见，root/body 横纵溢出为 0。创建输入使用 `focus({ preventScroll: true })`，避免浏览器默认聚焦滚动与轨道自定义对齐竞争；轨道目标不应按当前过渡中的 `scrollWidth` 提前截断，否则竖持侧转画布只能露出表单一角。
- 后续若增加创建字段，仍应保持同一卡位、短表单和 44px 操作区；需要复杂配置时应在实例创建后进入既有 saves/设置流程，不要恢复第二个大页面或把 driver 内部字段暴露给前端。

# FE-GAME-LIBRARY-FIXED-LANDSCAPE-1 前端接手记录（2026-09-04，未发布）

## 改了什么、影响哪些接口/文件

- `GameLibrary.css` 只对 `.game-hub--library` 增加手机竖持断点：根节点使用 `width: 100dvh`、`height/min-height: 100dvw` 和 `rotate(90deg) translateY(-100%)` 形成固定横向游戏画布，并以交换后的动态视口单位计算主卡、世界卡和轨道居中。手机横持时复用现有紧凑横向断点。没有 Screen Orientation API、方向权限、旋转提示或新的 React 状态。
- 这层变换随 `GamesPage` 一起卸载；世界卡仍通过 `stardewInstanceDestination` 和 `routeToPath` 进入 `/instances/:id/*`，随后由现有 `StardewMobileShell` 在窄屏中正常竖向渲染。`/games`、`/games/stardew`、overview/saves/install/new 的路由和后端接口没有变化。
- `GameLibrary.tsx` 删除品牌左侧柱状图标节点。library 顶栏在暖昼/月夜下都取消独立 surface、底边和投影，直接让农场天空贯穿顶部；品牌、账号和图标分别使用适配主题的深绿或月黄色，退出及原 44×44px 日月切换按钮保留。`world-create-card` 恢复原昼夜棋盘/星点纹理，正方形尺寸、权限、焦点和点击目标保持不变；只删除 `.game-world-rail-content::before` 的中部连接线。
- 影响文件：`frontend/src/{App.css,app-routes.ts}`、`frontend/src/games/{GameLibrary.tsx,GameLibrary.css}`、`frontend/scripts/test-{game-library-state,responsive-layout}.ts` 与三份长期文档。响应式契约新增固定画布、交换维度、无方向提示、无旧品牌图标及两套顶栏/新建卡样式断言。

## 如何验证、下一步注意事项

- `npm run test:game-library`、`npm run test:responsive-layout`、`npm run build` 通过。Browser 在 390×844 验证旋转矩阵、画布包围盒 390×844、无提示节点及 root/body 零横向溢出，在 844×390 验证自然横向画布且横纵均无页面溢出；世界展开稳定后点击青禾农场进入 `/instances/stardew/overview`，再切回 390×844 可见原竖屏移动壳。全新 QA 标签 console warning/error 为 0，右侧保留更新后的世界展开页面。
- 后续调整手机断点时，竖持规则里的水平尺寸必须使用 `dvh`、画布高度/卡片高度必须使用 `dvw`，并同时复验 390×844 与对应 844×390。不要为这层展示增加方向权限或提示；实例页也不要继承 `.game-hub--library` 的旋转。世界轨道打开动画完成前内容仍为 inert，自动化点击世界卡应等待约 360ms 展开完成。

# FE-GAME-LIBRARY-DAY-CARDS-1 前端接手记录（2026-09-04，未发布）

## 改了什么、影响哪些接口/文件

- `frontend/public/assets/game-hub/stardew-steam-store-header.jpg` 是 Steam 商城 App 413150 的官方 460×215 商店头图。`GameLibrary.css` 在主游戏卡封面中以 `contain` 完整显示该图，使用同源天空蓝填充卡片封面的剩余区域，因此桌面与移动端都能直接识别游戏标题和像素场景，页面运行时不依赖 Steam CDN。
- 暖昼主题通过 `.game-hub:not(.game-hub--background-night)` 限定：主卡和世界卡使用奶油纸渐变、木色描边/底边阴影与 12px 细像素网格，新建和未接入卡使用低对比棋盘格；标题、地址、时间及状态使用深绿/棕色高对比文字。`.game-hub--background-night` 则提供配套月夜卡面：深靛蓝渐变、冷蓝描边、细星点/方格、月黄色标题与世界封面月光叠色。选中青框、focus、hover、安装封面滤镜和世界状态颜色仍保持独立。
- `game-library-state.ts` 提供按本地小时分类和计算下一次 06:00/18:00 边界的纯函数。`GamesPage` 默认取设备本地时间，在边界 timer、`visibilitychange` 与 window focus 时同步主题；右上角手动切换把主题和到期毫秒写入版本化 localStorage，到下一个边界即失效并回到自动。存储不可用时内存选择仍能持续到边界，旧的无到期偏好会被视为过期并清理。
- 影响文件为 `frontend/src/games/{GameLibrary.tsx,GameLibrary.css,game-library-state.ts}`、`frontend/scripts/test-{game-library-state,responsive-layout}.ts`、静态封面与三份长期文档。主题时钟只改变视觉选择，没有改变 `GameLibrary.tsx` 的路由/动画/权限逻辑，也没有修改 `useStardewCatalog`、安装状态分类、实例目的地或后端接口。

## 如何验证、下一步注意事项

- `npm run test:game-library`、`npm run test:responsive-layout` 和 `npm run build` 均通过。Browser 在 1360×900 验证暖昼与月夜展开组合，在 390×844 验证月夜展开首世界卡、收起主卡完整居中、下一卡露出和 Escape 焦点归还；两个视口 root/body 横向溢出均为 0，console warning/error 为 0。右侧 QA 最终保留 1360×900 月夜展开态。
- `test:game-library` 覆盖 05:59/06:00/17:59/18:00 映射、跨日下一边界以及手动偏好未到期/到期/非法值；`test:responsive-layout` 固定组件的边界 timer、前台校准和手动到期存储契约。浏览器当前无法安全改写系统时钟，因此跨边界运行态由纯函数与源码契约回归证明，实际当前时段主题和手动切换由 Browser 验证。
- Steam 资源已固定为仓库本地文件；如后续更换商城图，继续保留本地版本并复验 460:215 原图在主卡剩余封面高度内的完整显示，不应改为会裁掉标题的 `cover`。新增昼夜色板时保持两个主题选择器互斥，并同步复验文字、状态和选中框对比度。

# FE-GAME-LIBRARY-SQUARE-CENTER-1 前端接手记录（2026-09-04，未发布）

## 改了什么、影响哪些接口/文件

- `GameLibrary.css` 以 `--game-card-width` 同时驱动游戏项宽度和 `.game-picker` 两侧 scroll padding，收起态的当前主卡因此按自身宽度精确落在视口中心，不再受右侧占位卡或整组内容宽度影响。
- 主游戏卡整体固定为 `1 / 1`，封面使用剩余高度，底部信息区压缩字号、间距与 padding。世界卡保留名称、状态和地址，整卡入口与顶部最新接手记录中的复制/生命周期按钮保持同级语义。管理员“新建世界”卡改为 1:1，桌面宽 190–210px、移动端最高 232px，在轨道中垂直居中；既有 `ResizeObserver` 会按新宽度自动重算组合中心。移动端去掉项目间额外 margin，并让暂未接入卡使用较窄宽度，390px 下仍露出约 35px 下一卡提示。
- `WorldRail` 展开后用主卡布局宽度、首张世界卡边界和现有 `contentWidth` 计算组合中心。完整组合可放入视口时居中全部世界卡与新建入口；中等宽度只居中主卡和首张世界卡；窄屏保持首张世界卡完整可用并留下主卡收回区域。展开不再用浏览器原生 smooth scroll 和额外 30ms 定时，`animatePickerTo` 用 360ms smoothstep 与 CSS 轨道宽度/主卡间距动画同帧运行，世界内容在 50ms 后淡入；展开期间关闭 scroll snap，窗口尺寸变化会复用同一计算。
- 收回只使用一个 `closing` 阶段：同一次状态提交立即让主卡通过 rAF 回中、世界轨道缩宽并带动右侧占位卡向左归位，同时把主卡 `margin-right` 恢复为收起态最终值，全部复用展开的 360ms 对称 smoothstep；世界内容先随轨道移动，再按反向时间线渐隐。结束后才导航 `/games` 并恢复 snap。这样既取消两阶段停顿，也避免路由完成后右侧卡片再被 margin transition 推动一次；移动端通过 `--game-card-gap: 0px` 保持最终间距一致。reduced-motion 直接完成关闭，Escape 和主卡点击共用同一流程。
- 主要影响 `frontend/src/games/{GameLibrary.tsx,GameLibrary.css}`、`frontend/scripts/test-responsive-layout.ts` 及长期文档；React 只复用现有 effect/ref，没有新增状态循环。路由、后端 API、安装分类、实例分流和权限契约均未改变。

## 如何验证、下一步注意事项

- `npm run test:game-library`、`npm run test:responsive-layout`、`npm run build` 通过。应用内 Browser 在 1360×900 实测新建卡 204×204、世界卡 244×约 372，在 390×844 实测新建卡 232×232、世界卡 278×约 399；两端 disclosure 节点均为 0，root/body 无横向溢出，console warning/error 为 0。新建卡点击仍进入 `/games/stardew/new`，最终右侧 QA 保持在 1360×900 的世界轨道展开状态。
- 调整游戏卡宽或卡间距时必须同步修改对应断点的 `--game-card-width` / `--game-card-gap`，不要重新引入基于整组内容宽度的固定居中常量。展开中心必须继续使用不受 CSS transform 动画影响的 `offsetWidth + contentWidth`，不能在动画中用变换后的 `contentRect.right` 推导终点；关闭应保持单一 `closing` 状态同时触发内容退回、主卡回中、轨道缩宽和最终间距恢复，并与展开共用 360ms 对称 smoothstep，不能把间距留到路由切换后再动画。`.game-carousel-card` 不应添加 `overflow: hidden`，否则会裁掉 `inset: -8px` 的选中框。

# FE-GAME-LIBRARY-INLINE-WORLDS-1 前端接手记录（2026-09-03，未发布）

## 改了什么、影响哪些接口/文件

- `GameLibrary.tsx` 将 library variant 的顶部栏收敛为 `ANXI PANEL`、用户名、退出和单个日月图标；`/games` 不再渲染导航、品牌副说明、页面标题、操作提示或分段主题按钮。主题仍只在浏览器本地保存，暖昼默认与静夜素材不变。
- `/games/stardew` 继续由 `App.tsx` 传入 `worldsOpen`，但世界选择不再挂载 `ModalPortal`：`WorldRail` 在星露谷主卡右侧按真实内容宽度展开，主卡再次点击或 Escape 收回并回到 `/games`。移动端自动把第一张世界卡滚到主位置，同时保留可点击的主卡边缘和下一卡提示；所有横向 overflow 都留在 `.game-picker` 内。
- 世界列表数据和动作没有另起契约：仍由 `useStardewCatalog` 渐进加载，继续使用 `classifyInstallationState`、`canonicalInstallJobs`、`stardewInstanceDestination` 和真实 instance DTO；`save_required` 进 saves，安装/修复进全局流程，其余进 overview。管理员才渲染“新建世界”，普通用户没有伪按钮。
- 主要修改 `frontend/src/games/GameLibrary.tsx`、`frontend/src/games/GameLibrary.css`、`frontend/scripts/test-responsive-layout.ts` 及三份长期文档。没有改后端 API、安装状态机、实例面板、安装页、新建页、存档页或 `ModalPortal` 的其它消费者。

## 如何验证、下一步注意事项

- `npm run test:game-library`、`npm run test:responsive-layout`、`npm run build` 通过。Browser 以 1440×900 验证 Enter/点击展开、卡片再点收回、主题切换、直达 `/games/stardew`、世界 overview/saves 分流；390×844 验证自动对齐、横向手势只改变轨道 scrollLeft、主卡保留约 56px、下一卡露出、所有顶栏/世界操作至少 44px，两个视口 root/body 无横向溢出且 console warning/error 为 0。Escape 收回后焦点返回主卡。
- `WorldRail` 的宽度来自 `ResizeObserver`，新增世界或权限变化后会重新测量；不要改成固定卡片数宽度，也不要用全局 `Promise.all` 等待所有详情。窄屏 scroll snap margin 与 picker 的 16px 内边距共同决定主卡保留量，调整时必须同时复验 390px 和更窄视口。
- 本节是当前游戏库外观与世界选择行为；下方 2026-09-02 的分段主题按钮、全页遮罩和 `ModalPortal` 世界弹窗只保留为历史实现记录。项目其它弹窗仍必须继续复用 `ModalPortal`，不能因本页改为内嵌轨道而复制焦点陷阱或滚动锁。

# FE-GAME-LIBRARY-BACKGROUND-SWITCH-1 前端接手记录（2026-09-02，未发布）

## 改了什么、影响哪些接口/文件

- 新增 `frontend/public/assets/game-hub/background_game_library_day_image2.png`，作为 `/games` 与路由世界弹窗的默认暖昼背景；现有 `background_game_library_image2.png` 保留为静夜选项。两张图共享农场空间关系和像素尺度，昼景把暖金、麦色、橄榄绿与浅蓝集中在环境本身，页面仍由中性深色 surface 保证正文与焦点对比度。
- `GameLibrary.tsx` 为 library variant 增加 `day | night` 主题状态和“暖昼 / 静夜”分段按钮。按钮用 `aria-pressed` 暴露选择，版本化 localStorage 键 `anxi.game-library-background.v1` 只保存当前浏览器偏好；缺失、未知或不可读取时默认暖昼，写入失败时当前页选择仍生效。`game-library-state.ts` 提供可单测的偏好归一函数。
- `GameLibrary.css` 分别定义昼夜背景叠层，并在 700px 下保证主题按钮 44px 高；GameHubShell、ModalPortal、安装分类、世界路由和所有后端接口保持既有契约。影响文件还包括 `test-game-library-state.ts`、`test-responsive-layout.ts` 及本接手文档/长期前端文档/路线图。

## 如何验证、下一步注意事项

- `npm run test:game-library`、`npm run test:responsive-layout`、`npm run build` 均通过。Browser 在 1280×900 验证首次暖昼、切到静夜、重新进入 QA 页面后保持、切回暖昼并打开 `/games/stardew`；390×844 验证两按钮均为 58×44px、轨道仍独立横滚且 root/body 无溢出。两个视口 console warning/error 均为 0，弹窗 Escape 关闭后焦点仍归还游戏卡。
- 本偏好不跨浏览器或账号同步；若以后确需账号级同步，应先定义独立的用户外观设置契约和迁移策略，不能把主题写入游戏安装或实例状态。新增主题时继续使用显式联合类型、可访问 pressed 状态和版本化素材名，并复验桌面/移动裁切、遮罩可读性与 ModalPortal 路由层。

# FE-GAME-LIBRARY-IMAGE2-BACKGROUND-1 前端接手记录（2026-09-02，未发布）

## 改了什么、影响哪些接口/文件

- 新增 `frontend/public/assets/game-hub/background_game_library_image2.png`：以现有农场背景为编辑基础，通过内置 ImageGen 重绘为原创、低饱和的深青蓝夜景。主要细节位于底部和两侧，中上部保留给标题、横向游戏卡和世界弹窗；素材内没有文字、UI、角色或第三方品牌元素。
- `GameLibrary.css` 的 `.game-hub` 移除旧网格/径向背景，改为新图片加两层深色可读性渐变，仍以 `--hub-bg` 兜底。`game-hub--stardew` 的既有登录/游戏专属背景没有覆盖，因此影响范围只在中性游戏库及其路由世界弹窗。
- `test-responsive-layout.ts` 增加素材引用契约。没有修改 React 状态、后端 API、安装流程、世界路由或 ModalPortal 行为。

## 如何验证、下一步注意事项

- `npm run test:responsive-layout`、`npm run test:game-library` 和 `npm run build` 通过。Browser 在 1280×900 与 390×844 验证 `/games` 和 `/games/stardew`，素材 URL 已进入 computed background；标题、卡片和弹窗对比度清晰，root/body `scrollWidth == clientWidth`，console warning/error 为 0。
- 当前 PNG 约 1.26 MB，作为单一首屏背景由 Vite 静态托管。若以后需要进一步压缩，必须先在桌面暗部渐变、移动端中心裁切和像素边缘三处做 A/B 视觉复验，不能直接用有损转换牺牲暗部层次；替换时继续使用版本化文件名，不覆盖登录背景。

# FE-GAME-LIBRARY-CARTRIDGE-TRACK-1 前端接手记录（2026-09-02，未发布）

## 改了什么、影响哪些接口/文件

- `GameLibrary.tsx` 把原来的大卡区改为原生横向近方形游戏轨道：roving tabindex 只让当前可用卡进入 Tab 顺序，左右键/Home/End 移动选择，Enter/Space、点击和触摸轻点激活；触摸横移会短暂抑制 click。选中项滚入可见区域，减少动态效果偏好使用即时滚动。`GameLibrary.css` 将横向 overflow 和 scroll-snap 限定在 `.game-picker`，桌面露出多卡、移动端露出下一卡，选中缩放为 `1.055`，青绿 selected 框、白色 focus 框和 hover 边框互不混用。
- 星露谷卡复用 `useStardewCatalog`、`canonicalInstallJobs`、`classifyInstallationState`、`stardewInstallCardState` 和 `stardewGameDestination`。安装中优先于已安装投影；未安装、失败、需修复只过滤封面并保留文字/焦点对比度；安装中只说明真实进度可在既有安装页查看，不合成百分比。第二张“其他游戏”卡固定显示“暂未接入”并原生禁用。
- `App.tsx` 让 `/games` 和 `/games/stardew` 共用同一 `GamesPage`，后一路由只增加 `worldsOpen`。世界选择层复用 `ModalPortal`，保留 instances 首屏后 state/public-IP/jobs 分别回填、世界目的地分类和 admin-only 新建入口；关闭、Escape、遮罩均回到 `/games`，同页保留的触发卡可由 portal 恢复焦点。
- QA fixture 增加仅用于弹窗滚动验收的六世界模式。主要文件是 `frontend/src/{App,qa-layout-main}.tsx`、`frontend/src/games/{GameLibrary.tsx,GameLibrary.css,game-library-state.ts}` 与 `frontend/scripts/test-{game-library-state,responsive-layout}.ts`；后端 API、实例页、安装页、新建页和存档页契约没有改变。

## 如何验证、下一步注意事项

- `npm run test:game-library`、`npm run test:responsive-layout` 和 `npm run build` 通过。Browser 在 1280×900 验证方向键边界、Enter/点击开窗、Escape/遮罩关闭和焦点归还；390×844 验证主卡/下一卡露出、轨道内部横滚不误开、六世界弹窗独立滚动、44×44 关闭按钮及 root/body `scrollWidth == clientWidth`。另行验证未安装卡的精确 aria-label/封面 filter、`save_required → /saves` 以及普通用户无可执行新建按钮；新建干净标签无 console warning/error。
- 当前应用内 Browser 没有 `prefers-reduced-motion` 媒体模拟接口，宿主当前读数为 false；响应式脚本固定了 CSS 下取消 selected transform，组件也以同一媒体查询把 `scrollIntoView` 改为 `behavior:"auto"`。后续若 Browser 增加媒体模拟，应补一条真实渲染复验。
- 后续接入真实第二个游戏时，应由真实 catalog/driver 数据生成可用卡并更新 `GAME_CARD_ENABLED`；不要解除当前占位卡 disabled、伪造实例或在此组件复制安装状态机。增加卡片时保持 roving 顺序、轨道内 overflow 和触摸 click 抑制，并继续让 ModalPortal 负责 inert、滚动锁、焦点陷阱和焦点归还。

# FE-WORLD-CATALOG-PROGRESSIVE-1 前端接手记录（2026-09-01，未发布）

## 改了什么、影响哪些接口/文件

- `GameLibrary.tsx` 的 catalog 不再 `Promise.all` 等待所有实例 `/state` 和 `/public-ip`。`GET /api/instances` 返回后立即以实例 DTO 的持久化 state 渲染卡片；jobs、完整 state 和公网加入地址分别异步回填到对应卡片，单个实例失败只影响自身，刷新时保留已有卡片。
- `game-library-state.ts` 把 `save_required` 提升为世界列表的优先语义：状态红色显示“需要存档”，加入地址显示“创建或上传存档后提供”，不请求无意义的公网地址，主按钮“前往创建 / 上传”进入该实例 `/instances/:id/saves`。即使 installation diagnostic 因镜像别名等原因暂时报告 incomplete，也不能把“没有存档”改成“重新安装游戏”。
- QA fixture 的第二个世界固定为 `save_required`，并支持 `catalogDelay=slow` 给 state/public-IP 各增加 1.6 秒，用于防止重新引入最慢请求阻塞整页。主要影响 `frontend/src/games/{GameLibrary.tsx,game-library-state.ts}`、`frontend/src/qa-layout-main.tsx` 与两个前端状态/布局测试脚本。

## 如何验证、下一步注意事项

- `test:game-library`、`test:responsive-layout` 和 production build 通过。应用内 Browser 实测慢详情接口下约 147ms 已出现两个世界；第二个世界立即显示需要存档文案，按钮真实导航到 `/instances/river-farm/saves`。375px root/body `scrollWidth == clientWidth`，之后恢复 1280×800 世界列表并保留右侧标签。
- 列表 DTO 的 state 是首屏权威快照，完整 `/state` 仍会校正诊断与运行细节；不要为了“统一完成”重新合并成全局 `Promise.all`。加入地址只在安装完整且不需要存档时请求，连接读取失败不得覆盖实例状态或移除卡片。

# FE-INSTANCE-ID-AUTO-1 前端接手记录（2026-09-01，未发布）

## 改了什么、影响哪些接口/文件

- `/games/stardew/new` 只保留“世界名称”，说明内部编号与目录由系统生成；创建请求变为 `{name,gameId:"stardew"}`，成功后仍以响应 `instance.id` 进入 `/instances/:id/saves`。
- 世界卡片不展示内部实例 ID，只呈现世界名称、运行状态、可用的加入地址与卡内快捷操作。已有动态 URL/深链仍使用后端 ID，属于路由实现而不是用户输入。
- `CreateInstanceRequest`、`stardewCreateInstanceRequest` 和 QA fixture 同步新契约；fixture 以兼容旧 `river-farm` 的目录状态返回后端式 `stardew-3`，不让前端推算编号。

## 如何验证、下一步注意事项

- `test:game-library` 固定请求只有 trim 后名称与 gameId；production build 和 Browser 创建交互需要确认页面没有“实例 ID”输入、提交后使用响应 ID 导航。普通用户和未安装分支不变。
- 不要在页面按数组长度拼 `stardew-N`：并发、删除、失败和旧实例都会使列表数量不等于下一个内部编号。若以后确需排障展示内部 ID，只放在管理员诊断/设置的只读区域，不恢复用户输入。

# FE-GAME-INSTALL-MIGRATION-1 前端接手记录（2026-09-01，未发布）

## 改了什么、影响哪些接口/文件

- `/games/stardew/install` 改为先读取 `GET /api/games/stardew/installation`，固定挂载后端 `installationTargetId` 对应的既有 InstallPage；旧 `/instances/:id/install` 兼容落到默认全局安装目标，jobId 仍保留在全局 URL。页面新增公用 Steam 下载账号/SteamCMD 设备缓存的非秘密状态，明确与世界级邀请码授权分离。
- `/games/stardew/new` 已从边界说明页接通 admin-only `POST /api/instances`，只提交名称和 `gameId=stardew`；内部 ID 由后台分配，成功读取真实 instance/ports 收据并进入实例面板。普通用户、未安装、模板维护、资源冲突与后端清理失败都保持真实权限/错误态，不生成假数据。
- `/games` 保持卡片直接导航、卡内安装按钮/轨道和单张“其他游戏暂未开放”；世界列表继续消费真实 instances/state/public-IP，创建后的 `save_required` 世界沿用现有存档页创建/导入存档。现有九页实例面板、桌面/移动返回世界列表和动态深链不变。
- 主要影响 `frontend/src/{App,app-routes,api,types,qa-layout-main}.tsx/ts`、`frontend/src/games/{GameLibrary,game-library-state}.*`、`games/stardew/stardew-routes.ts`、`frontend/scripts/test-{game-library,install-state,responsive-layout}.ts`。

## 如何验证、下一步注意事项

- `npm run test:game-library`、`test:install-state`、`test:responsive-layout`、`test:farm-catalog` 与 production build 通过；回归覆盖全局安装目标、旧安装深链、真实创建响应、管理员/普通用户、空/错误态和移动布局。对应后端 API/driver 测试、vet/build 与 Linux权限位专项也通过。
- 应用内 Browser 通过新的右侧 QA 标签完成 818×1075 游戏库/世界/实例/创建/安装闭环和 390×844 全局安装页验证；root/body 均无横向溢出，console 只有 Vite 与 React 开发提示。标签和 4177 服务保持打开。QA 创建 POST 使用仓库 mock，只证明前端请求 shape 与导航；真实端口、卷复制和清理由后端测试证明，不能把它写成真实 Docker 实例已创建。
- 公用下载状态不是世界运行态：前端不能用 `authorizationCached` 推断邀请码 ready，也不能把某世界 SteamAuth 成功显示为游戏下载完成。未来第二个游戏复用全局 shell 与公用下载状态，但使用自己的主题、安装诊断和创建表单。

# FE-MULTI-GAME-ENTRY-1 前端接手记录（2026-08-31，未发布）

## 改了什么、影响哪些接口/文件

- `App.tsx` 与新 `app-routes.ts` 把登录落点改为 `/games`，保留 `/games/*` 和动态 `/instances/:instanceId/*` 深链；`GameLibrary.tsx/.css` 提供中性游戏库、星露谷世界列表、全局安装和非弹窗创建边界页。星露谷卡片自身直接导航，未接入内容统一呈现为“其他游戏暂未开放”。
- `game-library-state.ts` 与 `test-game-library-state.ts` 复用 `installation-state` 和真实 `stardew_install` active job，以 `GET /api/instances` + `/api/jobs` + 每实例 `/state` 决定安装/世界/实例去向。主卡的安装按钮/轨道只显示权威 0%、100% 或不定进度；QA fixture 支持 multi/empty/error、admin/user 和初始 route，未开放游戏与普通用户创建入口都没有失效按钮。
- 世界卡片额外请求现有 `/api/instances/:id/public-ip`，消费新增的可选 `gamePort/protocol` 字段并展示玩家加入地址；公网 IP 失败、端口缺失/非法或尚未安装时分别显示诚实占位，不从浏览器 host 或固定 `24642` 猜测多实例端口。
- Stardew shell、dashboard hook、路由、玩家详情和 Nexus extension 请求改为消费路由 instance ID；进入任意已有世界仍使用现有九页完整面板，桌面侧栏和移动顶栏新增返回世界列表。全局安装直接挂载现有 InstallPage，任务 jobId 保持在 `/games/stardew/install` URL。
- 接口没有新增创建实例请求。`/games/stardew/new` 诚实记录后端缺少安全创建契约，明确世界是实例而非存档；存档继续在 `/instances/:id/saves` 内管理。

## 如何验证、下一步注意事项

- 已通过 `npm run test:game-library`、`test:install-state`、`test:responsive-layout`、`test:player-mods` 与 `npm run build`；后端 SPA 与公网地址定向测试通过。应用内 Browser 原有 1440×900/390×844 闭环后，又在 1280×900/390×844 验证主卡直接导航、未安装进入安装、安装轨道、每实例不同 `IP:port`、进入/返回、普通用户、空/错误态与零横向溢出；实例面板 console error 为 0。
- 新增/主要影响：`frontend/src/{App.tsx,app-routes.ts,api.ts,instance-id.ts,qa-layout-main.tsx,types.ts}`、`frontend/src/games/{GameLibrary.tsx,GameLibrary.css,game-library-state.ts}`、Stardew shell/dashboard/route/玩家/Mod 文件、前端测试，以及后端 SPA/public-IP/driver capability 与测试。已有未提交的建档农场改动和其它用户工作均未覆盖。
- 下一步先在后端 driver 层确定创建实例、隔离、幂等与清理契约，再接通现有新建入口；同时把仍默认实例作用域的全局 diagnostics 契约化。不要让前端直接创建 Docker 资源，也不要把共享 game-data/container 当成既定设计。

# FE-NEWGAME-WILDERNESS-MONSTERS-1 前端接手记录（2026-08-28，未发布）

## 改了什么、影响哪些接口/文件

- `frontend/src/games/stardew/new-game-farms.ts` 新增官方农场怪物默认值和纯状态转换；`NewGameCreator.tsx` 的左右箭头、官方农场卡、Mod 农场卡和手动 FarmType ID 全部复用同一选择入口。
- 尚未手动改怪物选项时，荒野农场自动得到 `spawnMonstersOnFarm=true`，其它官方农场为 false；用户手动切换复选框后，后续农场选择保留其明确值。公开 `NewGameConfig`、API payload、后端 DTO 和 Junimo 配置字段均未改变。
- `frontend/scripts/test-farm-catalog-state.ts` 增加默认/显式选择回归；`docs/03-frontend.md`、`docs/06-integration.md` 与 `docs/08-future-roadmap.md` 同步记录。

## 如何验证、下一步注意事项

- `npm run test:farm-catalog`、`test:new-game-idempotency`、`test:cabin-strategy-options`、`test:responsive-layout` 与 `npm run build` 通过。应用内 Browser 在 1280×720 QA fixture 实测荒野自动勾选、显式 false/true 跨官方农场切换保持，console warning/error 为 0 且无横向溢出。
- 当前静态/公开 Mod 农场目录不提供 `SpawnMonstersByDefault`，不得按 FarmType 名称猜测。若后续支持 Mod 农场原版默认，先由后端和运行时目录返回经过验证的布尔值，再扩展同一纯状态转换与真实 Mod fixture。

# FE-V060-RELEASE-EVIDENCE-1 前端接手记录（2026-08-27，released in v0.6.0）

## 改了什么、影响哪些接口/文件

- 本文件顶部记录的 SteamCMD 主链、邀请码 opt-in、冷启动等待态、移动生命周期终态、原版小屋默认、倒序安装日志和任务 URL 接管能力，均已进入 `v0.6.0@9c6d9c7696c6aa46f58405f0c02f187aa47111ba`。

## 如何验证、下一步注意事项

- 不可变候选 `33073661356` 成功，包含前端 19 项状态/响应式回归与 production build；同一不可变镜像完成 fresh/restart、`v0.5.13 → v0.6.0` 和 `v0.3.2 → v0.6.0` Web 升级，升级夹具验证权威 DTO/迁移状态。发布前本机 Browser 桌面/移动渲染只作为补充，不冒充候选 proof。
- 自动 annotated tag workflow `33075599631`、正式提升 `33075622114` 均成功；三仓 `0.6.0/latest` 唯一 digest=`sha256:e9c1613a7ffbd13d92d5a197d751cb5de6b08b65f74351e39a4ad0f9b4598d16`。[GitHub Release v0.6.0](https://github.com/AnXiYiZhi/stardew-server-anxi-panel/releases/tag/v0.6.0)
- 后续维护不得移动 `v0.6.0` tag，也不得把发布前本机热预览镜像或截图当作正式候选证明；正式身份只认上述 commit、workflow 与 digest。

# FE-V060-INVITE-COLD-START-WAIT-1 前端接手记录（2026-08-27，released in v0.6.0）

## 改了什么、影响哪些接口/文件

- 已授权实例在 server `starting` 或 `running` 时，后端 `generating` 与传输层请求错误改为受同一轮有界前端轮询预算保护：预算仍有效时，桌面总览、服务器摘要与移动首页统一显示不可重试的“等待中…”并继续取码；后端明确返回 `auth_unavailable` 时立即转真实异常。
- dashboard 以 5 秒间隔最多轮询 125 次，`starting → running` 时重置计数；页面在暖机途中刷新、首次拿到 generating 或短暂请求失败时也会自动恢复轮询。取得邀请码立即 ready；预算耗尽仍无有效码时才显示“Auth 异常（直连仍可用）”并停止自动轮询。该预算不清 Auth session、不触发重新授权，也不改变 disabled 零 invite 请求、`cleanup_pending` 安全收敛或始终可用的 LAN/IP 卡。
- `/state` 与 invite GET 除各自 request generation 外，共用 invite projection epoch：旧 `/state` 仍可提交较新的 runtime/诊断，但必须保留当前 enabled/code 并跳过邀请码视图；权威 disabled DTO 会同步隐藏卡片、清 loading/requested/code 并失效所有旧响应。poll、全量刷新和 job-finish 采用 state→invite 串行，额外的 shared epoch 继续保护外部定时器/手动刷新逆序。隐藏页不会消费预算；首次在隐藏态进入 active runtime 会保留待轮询请求，恢复可见后继续。
- 后端权威 `auth_unavailable` 在数据层保持粘性，普通 refresh/job-finish 不再因一次传输错误改回 generating；只有手动刷新或新 runtime 先显式重置后才复查。job-finish 的 1 秒补偿定时器现有独立 ref，重复事件会替换旧定时器，卸载时清理并以 mounted gate 阻止 await 后继续发 invite GET。
- 前端展示用的本地预算终态与最后一次请求状态分开保存：starting 预算耗尽只把卡片显示为异常，不覆盖最后的 `generating`，因此进入 running 会开启新的运行代预算；后端真实返回的 `auth_unavailable` 则立即终止且不会被状态切换掩盖。
- 影响共享邀请码状态选择器、dashboard data hook、`InviteCodeCard`、`ServerSummaryCard`、`OverviewPage`、`MobileHomePage` 及 install/lifecycle/responsive 状态回归；三处消费者不得各自维护不同的等待预算。

## 如何验证、下一步注意事项

- 已顺序通过 `frontend/package.json` 声明的 19 项状态/响应式测试脚本；shared epoch 收口后又通过 install-state、responsive 与 `npm run build`（Vite 8.0.16，149 modules）。状态回归用可控 deferred 顺序覆盖 ready 后旧 state、disabled 后旧 invite、starting→running 在两种响应顺序下重置预算，以及权威异常不重开；源码契约另固定串行 poll、hidden 恢复、disabled 零请求和桌面/移动一致性。正式候选 bundle 已由 `33073661356` 完成复验，能力已随 `v0.6.0` 发布。

# FE-V060-MOBILE-TERMINAL-1 前端接手记录（2026-08-27，released in v0.6.0）

## 改了什么、影响哪些接口/文件

- `mobile/MobileHomePage.tsx` 新增 `pendingStartupSawActiveJob`，并复用 `lifecycle-action-state.ts` 的 `shouldClearPendingStartupAction`。start/restart 的 lifecycle job 被观察后，无论成功、失败或取消，只要任务进入终态并从 active 列表消失就解除“启动中…”；直接 POST 失败、停止操作和新一轮提交也同步清理该观察标记。
- 移动端 Steam 邀请授权失败按角色展示：管理员保留“重新授权”，普通用户显示“授权异常，请联系管理员”。接口 shape、后端权限、桌面卡片、LAN 卡和 disabled 零 invite 请求均未改变。
- `SteamInviteAuthState` 现接收后端公开的 `cleanup_pending` 枚举值；共享 presentation 把它固定为不可重试的“等待中…”，安装页同时禁用重复授权并显示授权收尾中。字段 shape 与持久化字段不新增，也不把该值归入基础 installation failure。
- 影响 `types.ts`、`steam-invite-state.ts`、dashboard data、`InstallPage.tsx`、`MobileHomePage.tsx`、`frontend/scripts/test-{install-state,lifecycle-action-state,responsive-layout}.ts`；没有新增 API 字段。

## 如何验证、下一步注意事项

- `cleanup_pending`、promotion 二次门禁及 annotated tag 的 candidate run/digest 绑定补充后，已重跑并通过 `npm run test:install-state`、`npm run test:lifecycle-action-state`、`npm run test:cabin-strategy-options`、`npm run test:responsive-layout` 与 `npm run build`（Vite 8.0.16，149 modules）；三份 workflow YAML 与 4 个变更 Bash block 的语法检查也通过。纯状态回归覆盖“已观察 active job → job 消失且非 running”的失败/取消终态，源码契约固定普通用户联系管理员文案；responsive 回归同时固定 promotion 全局串行、tag/run/digest 绑定及 main/latest 复核先于首个 `latest` copy。
- 后续移动生命周期变更继续复用共享纯 selector，不要用 `state === running` 作为唯一 pending 清理条件；restart 提交前本来就是 running，必须先观察到它自己的 active job。

# FE-V060-RELEASE-PREFLIGHT-1 前端接手记录（2026-08-26，released in v0.6.0）

## 改了什么、影响哪些接口/文件

- `InstallPage.tsx` 对安装/授权请求的 202 成功和 409 `install_in_progress` 都先导航到带新 `installJobId` 的安装 URL，再接管任务详情；因此旧查询参数不会在 state/effect 时序中覆盖刚启动的 job。`frontend/scripts/test-install-state.ts` 固定成功、冲突、授权任务与基础安装 classifier 的边界。
- 本版其余前端范围保持同一权威数据流：disabled 由 dashboard hook 保证 invite 零 fetch/poll，三处首页只在 enabled 挂载 Steam 卡；LAN 卡始终存在；安装页使用 SteamCMD 主链、原版小屋默认和倒序日志提示；普通用户看不到启用入口，授权失败不改基础“已安装”。

## 如何验证、下一步注意事项

- `npm run test:install-state` 和 production build 覆盖 URL 接管修复；cabin、lifecycle、responsive 回归覆盖原版/堆叠、桌面/移动条件渲染、管理员权限与倒序提示。最终全量结果以候选 `33073661356` 为权威。
- 候选完成 production build，同一不可变镜像在 fresh、`v0.5.13` 升级和 `v0.3.2` 升级链中验证权威 DTO/迁移状态；“只有 SteamCMD/安装证据时显示 LAN 且零邀请码请求、有强 Auth 证据时显示 Steam 卡”的页面渲染由自动化状态契约与发布前本机 Browser 分别覆盖，不声称升级夹具执行过 Browser。tag、digest、`latest` 和 Release 的正式证据见本文件顶部。
- 后续调整任务路由时，URL 与本地 selected job 必须作为同一个提交动作更新；不能再让旧 URL effect 在 POST 成功后异步夺回任务选择。新增邀请码状态只扩展共享 selector，不在 Overview/ServerSummary/MobileHome 分别猜测。

# FE-STEAM-INVITE-STARTUP-WAIT-1 前端接手记录（2026-08-26，released in v0.6.0）

## 改了什么、影响哪些接口/文件

- `steam-invite-state.ts` 的 generating 文案改为“等待中…”；后续 release preflight 将正常 starting/running 暖机统一为后端 `generating`，网络请求错误只在有界 polling=true 时展示等待，后端明确 `auth_unavailable` 或预算耗尽才展示异常。
- `shouldPollSteamInvite` 与 `useStardewDashboardData` 对 generating、starting 兼容响应和暖机期请求错误保持有界轮询，并在 `starting → running` 重置预算；`InviteCodeCard` 与 `MobileHomePage` 都向共享 selector 传入同一 polling 状态，因此桌面总览、服务器摘要与移动首页不会分叉。影响 `steam-invite-state.ts`、dashboard hook、InviteCodeCard、MobileHome 和 install/responsive tests；API shape 与生命周期完成判定不变。

## 如何验证、下一步注意事项

- 有界预算更新后的 `npm run test:install-state`、`npm run test:responsive-layout` 与 `npm run build` 已再次通过；此前真实热预览只覆盖桌面启动和 390px 移动端重启从“等待中…”进入邀请码 ready，不覆盖 `running` 传输错误或预算耗尽终态；这两条已由候选 `33073661356` 的自动化状态回归与 production bundle 复验。
- 不得因单次传输失败或任意经过时间自行升级成 Auth 异常；后端权威返回 `auth_unavailable`，或本节定义的前端预算耗尽且仍只有 `generating`/请求错误时，才进入异常展示。该终态不等于 session 失效，也不触发重新授权；LAN/IP 卡始终独立可用。

# FE-CABIN-VANILLA-LOG-LATEST-1 前端接手记录（2026-08-26，released in v0.6.0）

## 改了什么、影响哪些接口/文件

- `NewGameCreator.defaultConfig` 默认 `cabinMode=vanilla`，控件只显示“原版/堆叠”；判断以显式 `recommended` 为堆叠特殊值，空值安全落回原版。`server-runtime-settings-state.ts` 的默认和空值 fallback 改为 `None`，共享高级设置把原版放首项，保留 `CabinStack` 与 hidden `FarmhouseStack`。
- `install-state.ts` 新增纯派生 `latestInstallLogsFirst`：复制日志、按 sequence 降序、截取 50 条，不改共享升序 state。`InstallPage` 过滤进度 marker 后使用它，但新日志不会强制抢走用户查看旧记录时的滚动位置；标题栏新增“最新日志在最上方（倒序显示）”，窄屏换行。完整 JobsLogsPage 仍旧到新。
- 影响 `NewGameCreator.tsx`、`server-runtime-settings-state.ts`、`ServerRuntimeSettingsDialog.tsx`、`InstallPage.tsx/.css`、`types.ts`、QA fixture 和 cabin/install/responsive tests；请求 shape、SSE、路由与浏览器持久状态不变。

## 如何验证、下一步注意事项

- `test:cabin-strategy-options`、`test:install-state`、`test:responsive-layout` 与 production build 已通过，覆盖原版 fallback、旧策略保留、乱序输入降序输出、50 条上限、源数组不变和窄屏提示契约；最终 Browser 桌面/390px 使用当前本机热预览收口。
- 不要对 `logs` state 调用 `reverse()`，否则会破坏 QR/Guard/下载阶段解析和 `lastSeq`。后续若要让完整任务日志页也最新置顶，应作为独立交互决策和测试，不要因共享 helper 被动改变。

# FE-STEAM-INVITE-OPTIN-1 前端接手记录（2026-08-26，released in v0.6.0）

## 改了什么、影响哪些接口/文件

- types 与 `steam-invite-state.ts` 以 `/state.steamInviteEnabled` 作为唯一意图；dashboard hook 在 disabled 时对 invite fetch/poll 全部 early return。`LanDirectConnectCard` 从邀请码卡拆出并始终展示，Steam 卡仅 enabled 渲染；Overview、ServerSummary、MobileHome 共用同一规则与授权/运行/生成/异常状态文案。
- InstallPage 改为 SteamCMD 主时间线；`canonicalInstallJobs` 只分类基础 `stardew_install`，安装页另用 page selector 同时接入 `stardew_steam_auth` 的 SSE/QR/Guard。管理员按钮精确为“启用 Steam 邀请码（需要再次登录授权）”，凭据入口“修改 Steam 账号密码”只在 installation classifier 确认已安装时渲染；SteamCMD 与 SteamAuth 共用的账号密码只是授权失效后的回退材料，各自成功的 cache/session 始终优先复用。修改表单只显示用户名/密码并调用独立 `PUT .../steam-credentials`，不再显示 VNC、镜像或共享凭据/cache/session 长说明；成功关闭表单、清空本地密码并刷新 state，不创建 job、不导航日志。安装主区也不显示一次性 Auth、等待或失败提示条；按钮、QR/Guard 和任务日志继续承担状态反馈。SteamAuth 已 `ready` 时显示已启用并禁用重复登录，failed/pending 仍允许重试且不清 SteamCMD cache。普通用户不渲染授权入口；授权失败不污染基础 installation state。
- `App.tsx` 在任何 Stardew shell 挂载前读取 setup-status 的 `defaultInstanceId`，`api.ts` 使用受限 live binding；影响 `App.tsx`、`api.ts`、`instance-id.ts`、types、安装 helpers/state/page、dashboard hook、cards/mobile、QA fixture 与状态/响应式测试。
- `DiagnosticsPage.tsx` 不再把 disabled 实例显示成缺失 Auth 的“版本对”：只展示/验收 JunimoServer，并明确可选 Auth 未启用；enabled 才显示 Auth 镜像、成对目标、认证卷和成对回滚语义。

## 如何验证、下一步注意事项

- 已通过 `npm run test:install-state`、`test:cabin-strategy-options`、`test:lifecycle-action-state`、`test:responsive-layout` 与 `npm run build`。独立凭据 PUT 的最新定向回归另覆盖 API method、无公开 `forceReauth`、两字段表单、无 VNC/镜像和成功刷新 state；后端定向覆盖 admin-only、Docker 运行态 409、两项更新/其它 Auth/cache/game-data 字段保留及零 job。早期真实 Panel 主功能验收使用桌面 `1440×900`、移动 `390×844`：三个页面均有 1 个“局域网直连”、0 个 Steam 邀请码卡，无横向溢出或 console warning/error；旧本地镜像早于最新 Auth 补修，仅作为历史截图证据。
- 发布前最后一轮本机 handoff 使用 commit 元数据为 `3e2be688ab5326bea0e6caacf1b7689a7be01f8d` 的 native `dev` 后端（`127.0.0.1:8090`、`/health=ok`）与 Vite HMR（`127.0.0.1:18096`）。应用内 Browser 在最新 HMR 后 fresh reload，新增 console warning/error 为 0；桌面 `1280` 视口 `clientWidth=scrollWidth=1280`，安装页权威保持已安装、显示倒序任务日志和 Steam Guard 交互，同时共享凭据长说明、一次性 Auth 说明、等待/失败提示条均为 0。移动 `390×844` 自动进入移动首页且 `clientWidth=scrollWidth=390`，保留邀请码状态和始终可见的“局域网直连”。本轮只刷新现有热预览，没有输入凭据、触发新授权或创建容器；页面保持开启供用户继续测试。正式发布身份以本文件顶部 `v0.6.0` commit/workflow/digest 为准。
- 用户随后自行完成现有 Steam Guard 交互；最新 Browser 现场为“已安装”+“Steam 邀请码已启用”，打开“修改 Steam 账号密码”后空表单、确认按钮与倒序日志保持可用，截图指定的三类提示和 QR 失败提示均为 0。桌面 `clientWidth=scrollWidth=1280`、overlay=0、fresh console warning/error=0；页面停在该表单供继续测试，最终邀请码需服务器启动后验证。
- 当前预览数据的 `steamInviteEnabled` 已被此前失败测试置为 true，所以最终 live snapshot 验证的是 enabled 失败可重试与 LAN 常驻；disabled 零 invite fetch/poll 继续由状态/响应式自动化和数据层源码契约覆盖，不能把这次 live snapshot 记作 disabled 现场证据。普通用户入口缺失由响应式/权限源码回归与后端 403 测试覆盖；真实 Guard/QR 与最终邀请码不由自动化代填。
- 新增邀请码状态时只扩展共享 selector/DTO，不能在三个页面分别猜测；disabled 的“零请求”必须在数据层保证，不能只靠 JSX 隐藏。Auth job 仍需出现在安装页日志，但不得重新进入基础安装 classifier。
- setup-status 是未初始化前允许访问的公开接口；任何重构都必须保持设置实例 ID 早于主 shell/hook mount。非法 ID 回退只用于防御 malformed 响应，不能掩盖后端配置错误。
- 本次不提供关闭/删除能力 UI，也不处理先直连后启用导致的角色可见性与迁移工具。

# FE-CONTROL-ONLY-AUTH-PHASE-1 前端接手记录（2026-08-23，released in v0.5.13）

## 改了什么、影响哪些接口/文件

- `panel-update-machine.ts` 将后端新增的字符串阶段 `verifying_auth` 纳入 full-stack active 集合，标签为“正在验证认证服务健康（不等待 Steam 登录）”；`verifying_runtime` 保持“正在验证 SMAPI 实际加载版本”。`UpdateDetailsDialog.tsx` 在更新运行栈与 SMAPI 验收之间增加认证节点。
- 顶栏、总览和详情继续从 `panelUpdateSurface/panelUpdatePhaseLabel` 读取同一后端权威状态，没有新增 component state、effect、计时器或请求。影响仅上述两个源码文件与 `test-panel-update-machine.ts`；API 类型仍允许字符串阶段，不新增 DTO 字段或浏览器持久状态。

## 如何验证、下一步注意事项

- `npm run test:panel-update`、`npm run test:responsive-layout` 与 `npm run build` 已通过，覆盖新阶段 active/non-terminal、总览文案、认证/SMAPI 标签分离和 production bundle。正式候选 `32648758732@be25fb3a4d0dfda4a9240a70e9fdb1d3a01a64cd` 的完整前端回归、fresh/restart 与 `v0.5.12 → v0.5.13` Web 升级后 bundle 验收通过；自动 tag `32649334502`、正式提升 `32649344923` 和 GitHub Release 成功。
- 官网首页与 `website/docs/changelog.md` 已回填 `v0.5.13` 用户说明；docs-only 提交 `616de0bd56999089530f98e729273b116507b994` 的 Docs Portal `32649797827`（build 22 秒、deploy 10 秒）和 Compatibility `32649797822`（约 2 分 28 秒）成功。线上首页/changelog 均为 HTTP 200，版本、Control-only 摘要以及认证/SMAPI 独立文案均命中；该 push 没有触发新候选或移动 tag。
- 后续不要在前端根据 warning 文本、进度百分比或持续时间猜 auth/SMAPI 阶段；阶段归属只认 `fullStack.phase`。`verifying_auth` 必须继续属于 active phase，否则 Panel 主更新已经 `succeeded` 后前端会过早把整个全栈任务判为 terminal。

# FE-STEAM-CREDENTIAL-RECOVERY-1 前端接手记录（2026-08-22，released in v0.5.11）

> 历史实现。`v0.6.0` 已移除公开 `forceReauth` 安装分支，现行入口只保存共享用户名/密码，契约见本文件顶部。

## 改了什么、影响哪些接口/文件

- `InstallPage.tsx` 的管理员操作区现在同时提供两种明确语义：原“登录授权”继续用后端保存的账号尝试授权；新增常驻“更换 Steam 账号 / 重新认证”总是要求重新输入完整凭据并发送 `forceReauth: true`。该按钮不再受“当前诊断是否允许普通安装表单”限制，因此后端旧终态、部分安装或未知诊断不会把用户困在复用错误密码的路径里。
- force 表单会说明只清除 Steam/SteamCMD 授权缓存，游戏文件和存档保留；安装运行/启动、全局 busy 或表单已经展开时常驻按钮禁用。取消、普通修复/安装、提交成功和挂接活动 job 都会清除 force 状态，避免后续普通操作误带该标记。
- 影响 `frontend/src/games/stardew/pages/InstallPage.tsx` 与 `frontend/scripts/test-install-state.ts`；后端请求 shape 沿用既有 `forceReauth`，没有新增 API、路由、持久浏览器状态或样式文件。

## 如何验证、下一步注意事项

- 状态回归、响应式布局回归和 production build 均通过。应用内 Browser 用 `steam_auth_failed + missing-files + admin` 夹具完成“进入安装页 → 常驻按钮 → 完整新凭据表单”交互；桌面与 390px 窄屏无横向溢出、无遮罩、无 console warning/error。
- 常驻入口必须保持 admin-only，不能显示或预填服务端保存的账号密码；“登录授权”与“更换账号”不能合并为同一含糊动作。后续若调整安装表单开放条件，必须继续让 `forceReauth` 覆盖普通诊断 guard，并保留运行中禁用门禁。
- 正式候选 `32575311262`、Compatibility `32575311243`、自动 Tag `32575807110` 与提升 `32575818623` 全绿；`v0.5.11@a9e186249a5c70c2e6fe45b7ed10a09db0b0c8bb` 已发布，三仓版本与 `latest` 统一 digest=`sha256:10c9813328370ae8ac92f11271fb76cd03787aab3b7f7fd523f20d66dfae8876`。官网首页/changelog 同步由后续 docs-only 提交完成，不得移动该 tag 或重建 digest。

# DOCS-PORTAL-0.5.8-0.5.11 前端接手记录（2026-08-22，completed，已上线）

## 改了什么、如何验证

- `website/docs/changelog.md` 已补齐 v0.5.8～v0.5.11，`website/docs/index.md` 的 frontmatter、版本卡与 CURRENT RELEASE 已统一为 v0.5.11；GitHub Release 正文同步用户可见变化、四个正式 workflow、唯一 digest 与 compare，tag、四项资产和发布时间未变。
- docs-only `f545c169ded5edb11f8b2a1b1aad289bea77532b` 的 Pages `32576397782` 成功（build `19s`、deploy `10s`），Compatibility `32576397780` 用时 `2m29s` 全绿。线上首页/changelog 均为 200，并精确包含 v0.5.11、Steam 密码错误摘要、v0.5.8～v0.5.10 历史节与常驻更换账号文案；该 push 没有新候选，不改变 v0.5.11 digest。
- 后续发布要同时更新首页 `release`、版本卡、CURRENT RELEASE、changelog 首节和 GitHub Release 用户摘要；部署后仍需核对线上正文和无误触发候选，再做单独 evidence-only 回填。

# DOCS-PORTAL-0.5.4-0.5.5 接手记录（2026-08-18，completed，已上线）

## 改了什么、影响哪些文件

- `website/docs/index.md` 把版本角标、版本卡与 CURRENT RELEASE 从 `v0.5.3` 更新为 `v0.5.5`；`website/docs/changelog.md` 新增 `v0.5.4` 和 `v0.5.5` 两节，补齐安装进度/授权复用，以及更新检查、日志尾页、人数设置、半屏弹窗和 image2 素材修复。
- GitHub Release 两版正文已独立补齐用户可读说明、候选 run 与 digest。Release 正文属于发布元数据编辑，没有修改 tag target、四项资产或镜像引用，也不会触发新的候选。

## 如何验证、下一步注意事项

- `website` 的 `npm run docs:build` 与 Compatibility `32133444574` 通过；提交 `95f190d` 没有 Validate release candidate。首条 Pages `32133444566` 的 build 成功但 deploy runner 长时间排队，取消后只手动 dispatch 同一 `docs.yml`；重跑 `32135628751` build/deploy 成功。线上首页和 changelog 均返回 200，精确包含 v0.5.5、v0.5.4、人数保存重启与 SteamCMD 授权复用正文。
- 后续发布时首页 frontmatter `release`、CURRENT RELEASE、首页摘要、changelog 首项和 GitHub Release 正文必须保持同一版本；正文可以发布后补充，但不得借此移动既有 annotated tag 或重建已证明 digest。

# FE-JOB-LOG-LATEST-TAIL-1 前端接手记录（2026-08-18，released in v0.5.5）

## 改了什么、影响哪些接口/文件

- `src/api.ts` 新增 `getLatestJobLogs`，`types.ts` 为响应补 `hasEarlier`。任务详情初载/刷新/清错误日志、安装详情初载和 dashboard 活动任务日志均切到最新尾页。
- `JobsLogsPage.tsx` 用服务端 `hasEarlier` 控制截断提示，并明确提示当前显示最新 1000 行；任务页与安装页先读取 job 再读取尾页，封住完成瞬间的并发读取竞态，运行中任务从尾页最后 sequence 接既有 SSE。影响 `InstallPage.tsx`、`useStardewDashboardData.ts` 和静态回归。

## 如何验证、下一步注意事项

- `npm run test:responsive-layout`、`npm run test:install-state`、`npm run build` 通过。回归锁定 API 的 `latest=true`、三个消费入口、`hasEarlier` 和新提示文案。
- 不要把 `logs.length === 1000` 当成存在更早日志，精确真值只来自 `hasEarlier`。运行中任务建立 SSE 时必须继续传最新日志的最后 sequence；若改回默认 `after=0`，长任务会重新回放最早 1000 行并再次隐藏尾部状态。

# DOCS-PORTAL-0.5.3 接手记录（2026-08-17，completed，已上线）

- `website/docs/index.md` 与 `website/docs/changelog.md` 已发布 v0.5.3 的角色密码首次认领、Nexus 精确版本安装/一键更新、建档后人数上限、诊断/刷新改进，并保留对群友「石头磊」和「鹈鹕镇的热心市民」的特别感谢。
- 官网 production build 已进入候选门禁并通过；push 对应的 Deploy docs portal `32033542832` 成功。GitHub Release 正文也已同步相同用户可读范围、候选/正式 workflow 与唯一 digest；正文更新没有移动 annotated tag 或改变镜像。

# FE-NEXUS-MOD-ONECLICK-UPDATE-1 前端接手记录（2026-08-17，released in v0.5.3）

## 改了什么、影响哪些接口/文件

- `ModsPage.tsx` 在可更新的已安装卡片上把“一键更新”放到“查看更新页”左侧。只对管理员、停止态、扩展已连接、直接 Nexus ID、单物理成员且具备 `UniqueID` 的 Mod 开放；其它场景用禁用按钮和 `title` 解释，现有外链不隐藏。
- 安装与更新共用 `startNexusExtensionBatch`、批量进度、失败任务跳转、session 恢复和扩展状态，没有第二套表单。更新目标写入 `operation: update`、`replaceUniqueId`、严格 `expectedVersion` 和 Nexus 页面地址；任务成功后重读 Mods 并强制重新检查更新。
- 扩展升为 0.1.8。`background.js` 把 operation/替换目标贯穿 batch、capture 和恢复状态，background 直连与 `panel-bridge.js` 只在 update 模式向既有远程安装接口发送 `replaceUniqueId`；普通安装 payload 保持兼容。普通批次只打开首项，当前 CDN 链接成功提交给 Panel 后才打开下一 Nexus 页，防止不同页面的 capture/file ID 交叉。
- `ModsPage.css` 为更新提示动作组增加桌面并排和窄屏换行；QA fixture 补齐 Nexus ID，用于验证正常和扩展断连状态。

## 如何验证、下一步注意事项

- `npm run test:nexus-extension-idempotency` 固定 update 上下文从 batch 到 capture 和两条 POST 的传递；production build 通过。应用内 Browser 验证按钮紧邻“查看更新页”、断连时禁用提示和 800px root/body 零横向溢出。
- 不要把本地“已安装”直接当成 update 批次完成条件，否则旧版本会在扩展真正下载前被提前标为 done。聚合包不能只替换其中一个成员；需要继续保留外链并解释为何不可一键更新。
- 真实 Chrome + 0.1.8 已在停止态完成 CDN 捕获与后台任务：Content Patcher `2.9.0 → 2.9.1` 更新保留 config/启用状态；缺前置批次先提交 Content Patcher `2.9.1/file_id=160463`，再打开并安装 Elle's New Barn Animals `1.1.3/file_id=34408`。两个落盘 manifest 精确匹配，临时目录为零。
- v0.5.3 首次候选 `32033542812` 的 selected code gates 已通过；fresh bundle 契约因仍在桌面/移动控制块搜索已抽离的运行设置 JSX 而误失败，且失败发生在候选上传、tag 和正式提升之前。`scripts/release-candidate.sh` 与升级 E2E 现都加载 `ServerRuntimeSettingsDialog` 懒块并在该块检查隐藏的 `FarmhouseStack` 兼容选项，Git Bash 语法、ShellCheck、production chunk 正则和 responsive-layout 回归通过；下一候选必须从修复后的新 commit 完整重跑，不可复用旧 run。
- 修复后的候选 `32034798704` 已完整重跑并成功；自动 Tag `32035705749`、正式提升 `32035725325`、三仓统一 digest、正式镜像冒烟和 Release 四项资产均通过。以后修改共享 runtime settings 懒块命名或产物拆分时，须同步 fresh 与升级后两条 production bundle 门禁。

# FE-REFRESH-ACTIONS-AUDIT-1 前端接手记录（2026-08-17，released in v0.5.3）

## 改了什么、影响哪些接口/文件

- 全量盘点可见刷新入口后，`DiagnosticsPage.tsx` 把健康检查与 Compose 刷新从全有或全无的 `Promise.all` 改成独立落盘；`JobsLogsPage.tsx` 区分列表失败与成功空列表，并在选中任务消失时清理旧详情；`MobileSavesPage.tsx` 兼容旧版 `backups:null`。
- `stardew-routes.ts` 将 `refreshInstanceState/refreshSaves/refreshMods/refreshPlayers/refreshJobs/refreshHealth/refreshInviteCode/refreshPublicIP` 的返回类型从 `void` 校正为 `Promise<void>`。运行时 API 未变化，但今后组合刷新可被类型安全地等待。
- 玩家、邀请码/面板地址、桌面存档与备份、VNC、用户、审计日志、当前认证状态、Mod 更新和 Panel 版本检查均已确认成功时完整替换对应 state，未发现 Mod 搜索页同类单向 merge。

## 如何验证、下一步注意事项

- `test:responsive-layout` 固定诊断独立结算、任务过期选择清理、移动备份空值归一、Mod 本次响应立即对账和刷新 Promise 类型契约；`test:mod-list` 与 production build 同时通过。
- 新增刷新按钮时必须明确四点：真实请求、按钮 busy 防重、成功覆盖当前视图 state、失败保留或清除旧数据的明确策略。多个独立资源不得用一个 `Promise.all` 形成无必要的全有或全无刷新。

# FE-MODS-REFRESH-INSTALLED-1 前端接手记录（2026-08-17，released in v0.5.3）

## 改了什么、影响哪些接口/文件

- `mod-list-utils.ts` 新增共用 `nexusInstalledState(mods, modId)`，把当前服务器清单转换成完整的 Nexus 安装状态；未匹配时必须显式清除 `installedEnabled/installedFolderName/installedVersion`，不能继续把旧搜索 DTO 当作真值。
- 桌面 `ModsPage.tsx` 的页头刷新会用 `loadMods()` 本次响应立即重算当前 Nexus 本体与前置卡片，再异步刷新 dashboard 缓存。移动 `MobileModsPage.tsx` 使用同一对账函数，session 恢复或公共清单变化时也能从“已安装”回到“未安装”。接口没有变化。

## 如何验证、下一步注意事项

- `npm run test:mod-list` 覆盖直接 Nexus ID、Nexus 包来源 ID，以及删除后传入空清单必须返回 `installed:false` 且清空旧元数据；`npm run build` 通过。
- 后续若增加新的 Nexus 安装来源 ID，只扩展共用 helper 的匹配契约，不要在桌面/移动页面各写一套只增不减的 merge。刷新失败时保留原 UI 并展示现有列表错误，不能把未知清单误当成空清单。

# NEXUS-EXT-LATEST-1 前端接手记录（2026-08-17，released in v0.5.3）

## 改了什么、影响哪些接口/文件

- `ModsPage.tsx` 的 `NexusExtensionBatchItem` 增加必填 `expectedVersion`；主目标取 `NexusModSearchResult.version`，前置取新增的 `NexusRequiredMod.version`。任一缺失都在 `START_BATCH_INSTALL` 前 fail closed。
- 扩展 0.1.5 用 `anxi_version` 在 Nexus 页面导航/session 中传递版本；`shared.js` 提供严格版本文本匹配和候选选择，`content.js` 只在单个 file ID 的 DOM 上下文内匹配目标版本。经典页会把候选规范到 `<dd data-id>` 并合并紧邻 `<dt>` 的文件标题/版本，避免下载链接子级 `<li>` 提前截断版本文本。新 Panel 显式传版本；旧 Panel 的 batch 缺字段时扩展先打开 Nexus，再从当前文件页标题补出版本。`background.js` 把版本和 file ID 持久化到 batch/capture，并由 background 直连或 panel bridge 统一提交 `{expectedVersion,nexusFileId}`。
- 版本匹配失败通过 `CAPTURE_FAILED` 立即结束对应批量项；不再点击任意 Manual 控件继续。manifest、请求头、README 和测试断言都升到 0.1.5，面板缓存 ZIP 会按 manifest 版本自动更新。0.1.4 在真实 v0.5.2 Panel 点击时因旧 payload 缺字段而安全失败，未创建后台页、任务或 Mods 写入；该兼容回归是 0.1.5 的直接原因。

## 如何验证、下一步注意事项

- `npm run test:nexus-extension-idempotency` 与 production build 通过；测试固定“2.9.0 候选在前、2.9.1 在后”并断言选择 2.9.1，覆盖 `2.9.10` 边界、缺版本拒绝、URL 参数及 background/bridge 请求体。已登录右侧浏览器的实际 Nexus DOM 探针选择 `2.9.1/file_id=160463`，区分旧版 `2.9.0/file_id=153187`，不存在的 `2.9.10` 返回空。
- 不能把版本参数放进 Nexus CDN 签名链接，也不能在找不到目标版本时恢复“第一个 file_id”逻辑。Nexus 改 DOM 后应先扩展“单文件上下文”的采集选择器并增加 fixture，服务端 manifest 验真继续作为最终保护。
- 当前 0.1.8 的真实 Chrome 已完成本体+前置 ZIP 批量安装与单 Mod 更新；具体 file ID、任务和落盘证据见本文件顶部及 `docs/09-image-build.md`。后续修改 Nexus DOM 选择、批次恢复或提交路径时仍须重跑同等真实链，不能用应用内 Browser 的只读 DOM 探针代替扩展点击、CDN 捕获、Panel 任务与 manifest 终态。

# FE-SERVER-RUNTIME-SETTINGS-UX-2 前端接手记录（2026-08-18，released in v0.5.5）

## 改了什么、影响哪些接口/文件

- `ServerSummaryCard.tsx` / `StardewPanel.css` 给可编辑人数摘要增加独立布局类：44px“修改上限”改为单元格内居中，不再依赖负 margin；极窄摘要单列。`OverviewPage.tsx/.css` 在在线玩家卡片头增加同一管理员入口，并复用现有 hook/dialog。
- `ServerRuntimeSettingsDialog.tsx/.css` 隐藏原生 number spinner，增加 44px 像素风 `− / +`，在 1 与 100 自动禁用；底部动作改为左侧“关闭 / 仅保存”和右侧“保存并重启”，`<=420px` 安全换成两行。
- `useServerRuntimeSettings.ts` 接收页面已有 `restartServer` 生命周期函数。显式“保存并重启”先经过带在线人数的 `alertdialog`，确认后 PUT、刷新 players，再调用 `handleRestart`；停止态按钮禁用，保存失败不会重启，部分成功错误会说明配置已保存。桌面服务器页、桌面总览和移动控制页都只传各自已有生命周期函数，没有直接 import/call restart API。
- `InstallPage.css` 移除五步时间线和 Steam 认证占位图标的重复 `drop-shadow`，图标与进度线各自固定 stacking order；seed/Steam/download 三个旧 PNG 自带不对称黑色外扩像素，按用户要求用 image2 保留原造型、配色、Steam 圆标和下载底座重新生成，抠底并切为三个 72×72 RGBA `icon_install_step_*_image2_regen.png`。Steam 认证卡继续与第三步共用一个源；模组流程的下载图标同步引用再生版。`ServerSummaryCard.tsx` / `StardewPanel.css` 使用专用存档摘要图标，并把顶栏农民头像裁成清晰的 22px 头部图标。旧 PNG 未覆盖，手绘 SVG 与不透明生成草稿均未进入运行时。
- 接口、DTO、权限、最大人数范围和配置文件格式均未改变。影响测试为 `scripts/test-runtime-player-limit.ts`；没有新建后端 endpoint、数据库表或页面私有保存状态。

## 如何验证、下一步注意事项

- 前端当前声明的 19 条状态/布局回归与 `npm run build` 全部通过。真实 QA shell：1280×720 的摘要按钮完全位于 item/grid 内，动作按钮 88/88/111px 全在 520px 弹窗内；430×900 与 390×844 的 stepper、动作区和对话框均无横向溢出，`− / +` 为 44px，运行中在线 3 人确认提示可见。右侧 Browser 又确认五步图标和 Steam 占位图标 `filter:none`、stacking order 为 `2/1` 且无裁切，服务器摘要主机农民为 22×22 方形头像裁切；安装与服务器页 root 均零横向溢出。
- 后续若调整动作顺序，必须保持左侧次要动作、右侧重启动作以及 420px 两行契约；不要恢复原生 spinner 或用负 margin 扩大热区。保存并重启必须继续走页面的 `handleRestart`，不能在设置 hook 内直接调用 `restartInstance`，也不能在停止态把“重启”偷偷降级成启动。

# FE-SERVER-RUNTIME-MAXPLAYERS-1 前端接手记录（2026-08-17，released in v0.5.3）

## 改了什么、影响哪些接口/文件

- `ServerRuntimeSettingsDialog.tsx/.css` 与 `server-runtime-settings-state.ts` 提供桌面/移动共用表单：顶部 `1~100` 数字输入及中文说明、运行/停止生效提示和低于在线人数的非阻塞警告；小屋策略、已有小屋处理和广播频率移入“高级设置”。弹窗只有“仅保存”，不会调用生命周期 API。
- `useServerRuntimeSettings.ts` 成为两端唯一读取/保存流：GET 归一默认值，PUT 始终提交实际 `maxPlayers`，成功 await `refreshPlayers()`。`ServerControlPage.tsx` 与 `MobileControlPage.tsx` 删除各自重复弹窗；快捷操作统一改名为“联机人数与小屋设置 / 人数上限 / 小屋策略 / 广播频率”。
- `ServerSummaryCard.tsx` 新增明确的 `canEditPlayerLimit/onEditPlayerLimit` props；管理员在线玩家摘要显示 44px 触控热区“修改上限”，回调仍是页面已有 `openRuntimeSettings`。运行中当前上限来自 `dashboardData.players.maxPlayers`，配置值只用于重启后提示。
- `scripts/test-runtime-player-limit.ts` 新增数据流/权限/边界/响应式/可访问性契约，并接入 package、兼容矩阵、release gates 和响应式门禁；QA fixture 为 live `12`、configured `16`。

## 如何验证、下一步注意事项

- 全部 19 项适用状态回归和 production build 已通过。应用内 Browser 的真实 QA shell 验证：管理员摘要入口可见、普通用户隐藏；两个桌面入口只打开一个共用弹窗；`0/101` 阻止、`1/100` 接受；目标 `2` 低于在线 `3` 仍可保存；running 显示 live `12` / configured `16` 待重启，stopped 显示下次启动；移动端同表单、同警告和 44px 操作按钮，root 无横向溢出。当前 Browser runtime 不能切换精确窄视口，窄屏由现有响应式静态门禁覆盖，不把强制 mobile shell 描述成 390px 实测。
- 移动浏览器自动化对 number input 的 DOM proxy 报空值，但截图清楚显示受控值 `16`；实际 fill/边界/警告均通过。这是自动化读取代理差异，不要据此另建移动 state。后续修改必须继续保持两端共用 hook/dialog，不能复制第三套实现，也不能把保存后的配置值直接写进 live dashboard 冒充已生效。
- 本任务没有保存并重启、Mod 检测或 tag/Release。若以后加“保存并重启”，必须调用既有生命周期与在线玩家确认流，不能在 hook 内直接发 restart。

# FE-DIAGNOSTICS-EXPORT-ACTION-1 前端接手记录（2026-08-17，released in v0.5.3）

## 改了什么、影响哪些接口/文件

- `pages/DiagnosticsPage.tsx` 将“导出诊断包”从“维护与技术详情”内容区移到页头 `.sd-diag-header-actions`，DOM 顺序为导出在前、重新检查在后；按钮复用 `sd-btn-tan sd-btn--lg` 和既有下载图标，管理员、`exportBusy`、错误提示与 Blob 下载逻辑不变。
- 技术详情只保留升级/深入诊断说明，不再重复导出入口。`scripts/test-responsive-layout.ts` 新增源码契约，锁定页头顺序、详情区无重复按钮，以及 `<=760px` 两列和 `<=460px` 单列响应式规则。没有新增 CSS 图片、API 字段或移动端专用分支。

## 如何验证、下一步注意事项

- `npm run test:responsive-layout` 与 `npm run build` 通过。应用内 Browser 的真实 QA shell 在 1400×900 量得“导出诊断包/重新检查”高度均 35px且无横向滚动；430×900 两按钮纵向满宽，root `scrollWidth=clientWidth=430`。
- 后续不要把按钮重新埋回折叠详情，也不要在页头和详情同时保留两个入口。普通用户仍看到禁用按钮并获得“仅管理员可导出诊断包”的 title；如果后端 ZIP 条目继续扩展，前端无需解析内容，只维持 HTTP/Blob 下载和现有错误展示。

# FE-MOD-UPDATE-REMINDER-1 / FE-MOD-CONFIG-CARDS-1 接手记录（2026-08-16，released in v0.5.2）

## 改了什么、影响哪些接口/文件

- `types.ts/api.ts/useModsManagement.ts` 消费新的 GET/POST Mod 更新检查接口；Hook 在页面挂载时自动读取、物理 Mod 清单指纹变化时重查，并在上传/删除成功后强制刷新。失败只更新页内错误态，不清掉已有 `modUpdates`。
- `ModsPage.tsx` 在「添加模组」页签加入数量徽标、状态条、“只看可更新”和管理员“重新检查”，可更新卡片提供当前→最新版本与外链。`ModsPage.css` 重构「配置模组」为全宽工作区和双列紧凑图片卡，删除常驻右侧说明栏；动态上下文条、四类筛选、依赖/解析/更新标签与 44px 开关热区均已接入，窄容器改为单列。已安装卡片的删除按钮单独复用现有红色服务器停止贴图与浅色文字，解决浅褐色贴图在同色卡片背景上看似未填充的问题，不改变删除条件、确认流程或其它页面的共享 `.sd-btn-delete`。
- `qa-layout-main.tsx` 新增两条可更新结果、真实 UpdateKeys/依赖状态和三张本地像素图 fixture，便于直接打开 `qa-layout.html` 验收两个页签。`scripts/test-responsive-layout.ts` 同时锁定已安装卡片删除按钮的红色填充贴图与浅色文字；生产入口不含 mock。

## 如何验证、下一步注意事项

- Node 24 Linux 洁净安装后的全部 17 组前端状态回归、production audit/build 均通过。应用内 Browser 在默认视口实际点击「添加模组」与「配置模组」：2 个更新徽标、状态条和两条当前→最新外链均存在；配置页为 37 张图片卡、桌面首行 2 张、旧右栏计数 0，四类筛选中“有问题 1”只留下 Custom Companions。已停止态 37 个删除按钮均显示红色填充；运行态保留同一贴图并具有 `disabled`、`not-allowed` 与灰化滤镜。820×732 下删除填充仍可见，root/body/main 横向溢出均为 0；最终标签 console warn/error=0。正式候选 `31945655119` 还在真实 v0.5.1→v0.5.2 Web 升级后从 production bundle 命中更新提醒、配置筛选和新版本卡片契约。
- 更新提醒必须继续留在 Mod 工作台，不扩展浏览器/系统通知；卡片图片失败应保持固定占位尺寸。配置写操作仍受管理员、存档、停服与 `canToggle` 约束，不能为了整卡交互把开关或批量按钮的独立语义合并掉。
- `v0.5.2@51fd82459e4ac8afbf362f7ad12c0651937879a1` 已正式发布；Compatibility `31945655121`、自动 Tag `31946063809`、正式提升 `31946073920` 全绿，三仓版本/`latest` 统一 digest=`sha256:42b5dae824f63d3b5ba44a1f33704a622a62c4d6170225d52a63ac39147aaaed`。正式镜像首次/重启与 GitHub Release 已复核，提醒仍仅存在于页面内。

# DOCS-PORTAL-0.5.0 接手记录（2026-08-16，completed，已上线）

- `website/docs/index.md` 已切换到 v0.5.0；`website/docs/changelog.md` 置顶加入 v0.5.0 的角色加入保护、存档导入恢复、真实最近活动和主机农舍保持，并单列补齐官网此前遗漏的 v0.4.19 全服/角色独立密码与旧配置兼容。主题、CSS、依赖和路由未变。
- Panel 前端运行代码已随 `v0.5.0@9b18dd3fe5192692548bf11a85010dd35303da93` 发布：v0.4.19 的共用玩家加入保护弹窗继续包含，v0.5.0 的“在线 / 最近活动”语义已进入 production bundle。候选 `31899107629`、正式提升 `31899874927` 和统一 digest `sha256:92ea973d55c1f63b4eb356652d491f8d37ef5f69112df1f19c161e4b0e9b611a` 已成功。
- VitePress production build 2.68 秒通过；docs-only 提交 `242453ab631750689de467625346b6b0fb97c206` 只触发并通过 Pages `31900873468`（build 18 秒、deploy 14 秒）与 Compatibility `31900873542`（1 分 55 秒），没有候选重建。线上 1440×900/390×844 从首页真实点击到 changelog，首页 v0.5.0、日志 v0.5.0/v0.4.19/v0.4.18 顺序、角色密码/legacy/存档恢复/真实最近活动/农舍正文、root/body 零横向溢出和 console warn/error=0；v0.5.0 tag 与正式 digest 未改变。

# DOCS-PORTAL-0.4.18 接手记录（2026-08-15，completed，已上线）

- `website/docs/index.md` 与 `website/docs/changelog.md` 已同步 v0.4.18 用户可见范围：停服空 Compose 首次导入、Control-only 缺失 JunimoServer/旧人工事务恢复、共享确认框与最近控制命令分页。官网主题、CSS、依赖和路由未变，Panel API、镜像、tag、digest 与 Release 也未改。
- VitePress production build 2.96 秒通过；本地应用内 Browser 在 1440×900/390×844 从首页真实点击到 `/changelog.html`，版本顺序和三组正文正确，两视口 root/body 无横向溢出、overlay=0、console warn/error=0。
- docs-only 提交 `09601de0d9b9064b88a56d091678194a65c333cd` 的 Pages `31886032569` 与 Compatibility `31886032526` 均成功，未触发候选。线上同样在 1440×900/390×844 完成首页到日志真实点击，v0.4.18/v0.4.17/v0.4.16 顺序、四类正文、横向溢出、overlay 和 console 均通过；不得因后续证据收口重新发布候选或移动 `v0.4.18`。

# v0.4.18 前端发布接手状态（2026-08-15，released）

- `FE-MODAL-VIEWPORT-1` 与 `FE-CONTROL-COMMAND-PAGINATION-1` 已进入 `v0.4.18@56c437004b51763e77d12ffd9b716f39224d7b00`。最终候选 `31884242692` 在 17 项状态回归、production audit/build、fresh 与 `v0.4.17` Web 升级后的 production chunks 中验证共享 modal、JobsLogs/Players 分页和相关响应式契约。
- Compatibility `31884242697`、自动 Tag `31884612425`、正式提升 `31884620508` 成功；三仓 `0.4.18/latest` 六引用统一 digest=`sha256:b304e3b9c83620e94e3a16f33f5730991f74e470820a7481e696b54738eb8d74`，独立正式镜像重启、版本接口和 GitHub Release 四项资产已复核。新增确认框继续必须复用 `ModalPortal`；控制命令 API 在服务端分页前继续保持现有本地 3 条切片契约。

# DOCS-PORTAL-0.4.17 接手记录（2026-08-15，completed）

## 改了什么、影响哪些文件

- `website/docs/index.md` 首页版本切换为 v0.4.17；`website/docs/changelog.md` 置顶新增 v0.4.17，并把 v0.4.16 降为历史条目。说明范围与正式 Release 一致：auth `/health` 服务验收、安装完成后首次上传状态机和“社区中心收集包”文案。
- 只影响官网 Markdown 与长期文档；没有修改主题、CSS、依赖、路由、Panel API、镜像、tag、Release 正文或资产。

## 如何验证、下一步注意事项

- VitePress production build 5.90 秒通过；应用内 Browser 在本地及线上 1440×900/390×844 从首页实际点击版本入口，v0.4.17/v0.4.16/v0.4.15 顺序、三项正文、零横向溢出、零 framework overlay 和零 console error/warn 均通过。发布提交 `94db6f6066120cba903204e6fe1e47d40e06cc95` 的 Pages `31871879333` 与 Compatibility `31871879299` 成功，没有触发候选重建。
- 以后每个正式版本完成后，应在发布证据提交后同步官网首页与 changelog，避免官网版本落后于 GitHub Release。

# v0.4.17 前端发布接手状态（2026-08-15，released）

- “社区中心收集包”文案修正已进入 `v0.4.17@d63c93ffe7d65f8cdfcf2bedb9b336a6839be73f`；`remixedCommunityCenter` 默认值、勾选行为、DTO 与后端语义不变。候选 `31823172958` 通过全部 17 个前端状态测试、production audit/build、fresh/restart 与 `v0.4.16` Web 升级。
- Tag `31823884131`、正式提升 `31823899038` 成功；三仓 `0.4.17/latest` 六引用统一 digest=`sha256:44c328cdf198ec888f3ec54bbe836ce114f5ac27c4ca5fb9cc63747a44083673`。独立正式镜像版本接口/重启和 GitHub Release 四项资产已复核。

# DOCS-RELEASE-NOTES-0.4.15-0.4.16 接手记录（2026-08-14，completed）

## 改了什么、影响哪些文件

- `website/docs/index.md` 的首页版本和摘要切换到 v0.4.16；`website/docs/changelog.md` 在 v0.4.14 之前补入 v0.4.16 与遗漏的 v0.4.15。两版 GitHub Release 使用同一用户可见功能范围，不再只有 compare 链接、候选 run 或依赖提交标题。
- 只影响官网 Markdown、GitHub Release 正文和对应维护文档；不改变网站主题、CSS、依赖、路由、Panel API、运行镜像、tag 或 Release 资产。

## 如何验证、下一步注意事项

- VitePress production build 5.63 秒通过；应用内 Browser 在本地及线上 1440×900/390×844 实际从首页点击版本入口，确认 v0.4.16/v0.4.15/v0.4.14 顺序、两版正文、root/body 零横向溢出、零 overlay 与零 console error/warn。发布提交 `2df79f9` 的 Pages `31802129359` 与 Compatibility `31802129284` 成功；两版 GitHub Release 已同步且正式状态、发布时间、四项附件未变。
- 以后正式发布完成后应在同一收口任务内同步首页 frontmatter、首页版本摘要、changelog 与 GitHub Release 用户可读正文，不能只记录内部发布证据或 compare 链接。

# v0.4.16 前端发布接手状态（2026-08-14，released）

- `FarmhouseStack` 两端隐藏兼容与游戏日回档悬停详情已进入 `v0.4.16@5fa04d137bf760d2124b75cc5e3e8e2b44ff4c7c`。最终候选 `31799350642` 在 fresh 和从 `v0.4.15` Web 升级后的 minified production chunk 中重复验证两项契约，并通过前端 17 项状态测试、audit/build 与网站 build。
- Tag `31799876171`、正式提升 `31799891830` 成功；三仓 `0.4.16/latest` 统一 digest=`sha256:5f07910869d6d895e40ecb3954f5905d0cb6abf830e7cf57062bbcf97ca37e0f`。独立正式镜像版本接口/重启与 GitHub Release 四项资产均通过，下一位无需再为本版补发。

# FE-RELEASE-GATE-RELOCATION-1 接手记录（2026-08-14，released in v0.4.16）

## 改了什么、影响哪些文件

- `frontend/scripts/test-responsive-layout.ts` 的发布契约从“`release.yml` 必须直接包含三个 npm 测试命令”调整为三层断言：`scripts/run-release-gates.sh` 保留 responsive/new-game/Nexus 回归，`release-candidate.yml` 必须调用该统一门禁，正式 `release.yml` 必须只用 `skopeo --preserve-digests` 提升制品且不得 `docker build`。
- 自动发布扩展后，该测试还读取 `release-after-candidate.yml`：要求候选具备 `main` 产品路径 push 入口、成功候选由 `workflow_run` 收口，并显式 dispatch 支持 `workflow_dispatch` 的正式 workflow。`GITHUB_TOKEN` 创建 tag 不承担递归触发职责。
- 没有修改 React/CSS/API；影响仅限前端测试对新候选发布架构的静态保护。Compatibility workflow 仍直接执行核心前端测试，候选 workflow 运行完整 17 项状态测试、audit 和 production build；本版 cabin strategy/save backup detail 两项专项已加入统一门禁并由 responsive 契约锁定。

## 如何验证、下一步注意事项

- 运行 `npm run test:responsive-layout`；同时对两个 workflow 运行 YAML/actionlint，对三个 Bash 脚本运行 `bash -n`/ShellCheck。真实候选链由 Windows wrapper 和受控 TLS DinD 验证。
- 后续新增关键前端发布测试时，应加入 `scripts/run-release-gates.sh` 并扩展本契约；不要把重复 npm 门禁重新放进 tag workflow，否则会失去“只提升已测 digest”的保证。

# FE-CABIN-FARMHOUSESTACK-HIDE-1 接手记录（2026-08-14，released in v0.4.16）

## 改了什么、影响哪些文件

- 桌面 `pages/ServerControlPage.tsx` 和移动 `mobile/MobileControlPage.tsx` 的 CabinStrategy 下拉框把 `FarmhouseStack` 标为 hidden；用户可见选项只剩 `CabinStack` 与 `None`。
- 这不是接口删值：隐藏 option 保留已有 `FarmhouseStack` 受控值的兼容性，`types.ts`、`api.ts` 和后端 GET/PUT 三值契约均未修改。已有配置只有在用户主动选择另一个可见策略并保存时才改变。

## 如何验证、下一步注意事项

- 运行 `npm run test:cabin-strategy-options` 与 `npm run build`，并在桌面与移动控制页分别打开“小屋与联机高级设置”，展开策略选择器，确认只有两个可见选项且 console 无 error/warning。fresh candidate 与上一正式版 Web 升级后的生产 chunk 都必须再次命中 `FarmhouseStack` hidden 兼容契约。
- 后续若决定正式废弃 `FarmhouseStack`，必须另做数据迁移、API 契约和后端校验变更；本次仅隐藏，禁止把旧值静默归一为 `CabinStack`。

# v0.4.15 前端发布接手状态（2026-08-14，released）

- annotated `v0.4.15` 固定在 `d84157dc8a3abc83d13d29c276d6ed332e901ce7`；Compatibility `31725203858` 与 Release workflow `31725256195` 成功。Docker Hub、ACR、GHCR 的 `0.4.15/latest` 统一 digest=`sha256:b91e3cfd8175305723e0b97feb7c4c202179f2e229aff4f6145fe60b354a5c33`，逐仓 fresh/restart health/database/version 通过，GitHub Release 四项资产与 tag 源一致。

# FE-DEPENDENCY-NANOID-SECURITY-1 接手记录（2026-08-14，released in v0.4.15）

## 改了什么、影响哪些文件

- 正式发布门禁在 2026-08-14 新命中 nanoid high advisory；依赖链为 Vite 8.0.16 → PostCSS 8.5.25 → nanoid 3.3.17。PostCSS 已允许 `^3.3.16`，所以只把 `frontend/package-lock.json` 的 nanoid 节点更新到 3.3.18 及其官方 npm tarball integrity；`package.json`、React 源码、API、扩展和用户交互均不变。

## 如何验证、下一步注意事项

- Node 24 任务容器中洁净 `npm ci` 后，production audit 为 0 vulnerabilities；15 项前端状态测试与 production build 全部通过。不要用 `npm audit fix` 无边界升级整个锁文件；后续 advisory 仍先解析依赖链和允许范围，再采用最小补丁并重跑完整前端门禁。
- 因 lockfile 属于镜像 build context，先前 `df90240` 候选虽然已通过两条升级与 unhealthy 回滚，也不能再用于 tag。正式候选必须从包含本修复和发布文档的最终 SHA 重建，并重跑 fresh/restart、v0.4.14/v0.3.2 Web 升级后功能与 unhealthy 自动回滚。

# DOCS-INSTALL-HTTP-CARD-3 接手记录（2026-08-13，completed，未发布）

## 改了什么、影响哪些文件

- 按当前产品要求，在所有活动部署命令入口把“国内加速脚本（HTTP）”恢复为独立卡片，并统一放在官方 GitHub Release 命令正下方。官网三页使用 VitePress `tip`，README、新手指南和镜像部署文档使用 GitHub `[!TIP]`。
- 影响 `README.md`、`docs/user-guide/getting-started.md`、`website/docs/guide/deploy.md`、`website/docs/deploy/quick-start.md`、`website/docs/deploy/windows.md` 与 `docs/09-image-build.md`。国内命令固定为 `curl -fsSL -o run.sh http://anxinas.dpdns.org/run.sh && chmod +x run.sh && bash run.sh`；未修改实际 `deploy/run.sh`、Panel 前后端、镜像源或 Compose。

## 如何验证、下一步注意事项

- 六文件一致性检查确认官方/国内地址各出现一次并保持官方在上；`npm.cmd --prefix website run docs:build` 通过。应用内 Browser 覆盖桌面部署页与 390×844 一键脚本/Windows 页：卡片、HTTP 命令和 Windows `cd ~` 上下文均可见，root/body 无横向溢出，framework overlay 与 console warn/error 为 0；部署安装页实际点击“一键脚本部署”后目标页卡片存在。
- 后续新增任何直接展示 `run.sh` 安装命令的用户页面，都必须同时放置这两个入口并保持同一顺序；不要再只保留官方命令，也不要把国内地址改写成 HTTPS。若国内分发地址变化，应一次同步上述六处并重跑 production build 与桌面/手机渲染检查。

# DOCS-WINDOWS-STANDALONE-1 接手记录（2026-08-13，completed，未发布）

## 改了什么、影响哪些文件

- 新增 `website/docs/deploy/windows.md`，把 Windows + Docker Desktop 从系统要求页拆成部署侧栏独立专页，并排在 NAS 图形化部署之后。页面以 WSL2 + Docker Desktop Linux containers 为唯一支持路径，补全安装、集成、验证、数据目录、一键部署、访问端口、日常维护和排障。
- `website/docs/deploy/requirements.md` 删除 Windows 配置正文，只保留新页入口；同步修改 `.vitepress/config.ts`、`deploy/quick-start.md`、`guide/deploy.md`、首页入口、README、内部新手指南与 `docs/11-docs-portal.md`。Panel React、后端 API、脚本和 Compose 未变。

## 如何验证、下一步注意事项

- `npm.cmd --prefix website run docs:build` 通过（3.96 秒）。应用内 Browser 在 1440×900 和 390×844 验证 Windows 页 9 个章节、桌面侧栏顺序、NAS→Windows 点击、标题/警告框、零页面横向溢出、零 overlay 与零 console warn/error；系统要求页不再包含 Windows H2、`wsl --version` 或旧配置块，并保留新页链接。
- 后续 Windows 支持说明以该专页为唯一详细来源；README、新手指南与 quick-start 只保留摘要和链接。不得把 Windows 写成原生 `.exe`、Windows Service 或 Windows containers，也不要让用户在 PowerShell、`/mnt/c` 或另一个 WSL 发行版重复部署第二套。

# DOCS-NAS-SSH-DEFAULT-1 接手记录（2026-08-13，已发布）

## 改了什么、影响哪些文件

- NAS 部署页首屏改为默认推荐 SSH 一键脚本，原图形化 Compose 方案明确标记为“进阶”；只有熟悉宿主机绝对路径、Docker Socket、端口、持久化挂载和环境变量的 NAS Docker 图形界面用户才建议使用。
- 影响 `website/docs/deploy/{nas,quick-start,requirements}.md`、`website/docs/guide/deploy.md`、`website/docs/index.md`、`website/docs/.vitepress/config.ts`、`README.md` 与 `docs/user-guide/getting-started.md`。没有修改 Panel React、后端 API、`deploy/run.sh` 或图形化 Compose 配置本身。

## 如何验证、下一步注意事项

- `npm.cmd --prefix website run docs:build` 通过（3.92 秒）。应用内 Browser 在 1440×900 和 390×844 验证新标题、SSH 推荐框、图形化进阶条件、零横向溢出、零 framework overlay 与零 console warn/error；点击推荐框的一键脚本链接后进入 `/deploy/quick-start.html`，目标页包含 Linux/NAS 共同优先使用脚本的说明。
- 提交 `5526ef214e1ff25b7e30b9861bf416302a39d08b` 的 Pages `31708671546` 与 compatibility `31708671729` 均成功；NAS 默认推荐 SSH 的说明已上线。
- 后续增加 NAS 部署入口时继续保持“SSH 一键脚本为默认、图形化 Compose 为熟练用户进阶方案”的顺序；不要再使用“NAS 用户通常没有 SSH 习惯”作为推荐图形化部署的理由。若脚本或 NAS 支持边界发生变化，需同步 README、新手指南、官网四个部署入口和侧栏标签。

# FE-SAVE-IMPORT-FIRST-UPLOAD-1 接手记录（2026-08-13，completed，未发布）

## 改了什么、如何验证、下一步

- 上传表单和 API shape 不变。`frontend/src/core/helpers.ts` 与 `frontend/src/games/stardew/save-import.ts` 新增 `save_import_runtime_prepare_failed` 的明确提示：运行组件准备失败时保留上传、检查 Docker/网络后重试；只有后端确认 `junimo_import_unsupported` 才提示升级 Junimo 运行组件。
- `frontend/scripts/test-save-import.ts` 把新错误加入稳定错误闭集；`npm run test:save-import` 与 production build 已通过。bootstrap 名、指纹和清理状态均为后端私有 journal，不得在前端复制一套状态机或把 job 的 202 当成最终成功。
- 正式候选需用空 saves/无 gameloader 的真实 UI 上传验证：用户无需先启动服务器，失败后同一 token 可重试，完成时只显示目标存档且刷新/重启状态正确。完整故障与升级矩阵见 `docs/09-image-build.md`。

# NEXUS-EXT-IDEMPOTENCY-1 接手记录（2026-08-13，completed，未发布）

## 改了什么、影响哪些文件

- `browser-extensions/nexus-slow-installer` 升为 0.1.3。capture 新增持久 `requestId`；相同已知 mod/file 或同一批量项在页面重注入和 worker 重启后复用，不同/未知 fileId 的独立动作或新动作轮换。`background.js` 直连和 `panel-bridge.js` 同源转发都发送相同 Idempotency-Key。
- `shared.js` 的 singleflight 会先发布 Promise 再异步执行 POST，并在 resolve/reject 后立即删除；不缓存失败。background follower 复用网络结果但仍更新自己的 capture/batch，leader 才发通知；提交失败会把 capture 恢复为可重试。
- 影响 `shared.js`、`background.js`、`panel-bridge.js`、manifest/README、`frontend/scripts/test-nexus-extension-idempotency.mjs`、package scripts 与两个 workflow。没有改 React ModsPage 的批量状态机、搜索 sessionStorage 或远程安装请求体。

## 如何验证、下一步注意事项

- `npm run test:nexus-extension-idempotency` 真实加载三份扩展脚本，覆盖 20 路并发单 POST、rejected 后同 worker 立即重试、worker 重启、重注入、不同/未知 fileId、bridge singleflight 和 0.1.3 版本头；三份 JS 另经 `node --check`。当前 15 项前端 `test:*`、`test:responsive-layout` 与 production build 全部通过。
- 不要把 requestId 改回 `instanceId+modId` 或固定 5 分钟 Promise：前者会合并同 Mod 的不同文件，后者会缓存 rejected 并阻断重试。跨进程权威在后端 jobs 唯一键，扩展 Map 只做当次 in-flight 降噪。
- 正式候选需用真实 Chrome/Edge 扩展验证一次自动批量链，并模拟 POST 返回前断流；恢复后 UI/扩展应拿到原 jobId、批量项进入 queued/job 跟踪且不遗留第二个后台页。

# FE-INSTALL-RUNTIME-ERROR-MAPPING-1 接手记录（2026-08-13，released in v0.4.14）

## 改了什么与如何验证

- `installation-state.ts` 统一 `StardewPanel`、`InstallPage`、移动端的安装/repair/runtime-error 判定；后端 files-ok error 显示 retry/diagnostics，unknown 不显示重装，只有明确 missing 才 repair。新建档 fetch 发送 Idempotency-Key，UI 对相同配置失败重试复用同 key。
- 专项状态、幂等、responsive 与 production build 通过。应用内 Browser 在由 v0.4.11 Web 升级得到的 0.4.12 bundle 上验证：桌面 error 页面无“未安装/重装”弹窗，点击“查看诊断”路由正确；390×844 移动端诊断引导可见，root/body 无横向溢出，console error/warn 为 0。
- `v0.4.14@a70efc98feec` 已正式发布；Release workflow `31682847388`、三仓 `0.4.14/latest` 统一 digest 和逐仓启动/重启均通过。生产同步等待新主机正确 SSH 用户名；不要在前端重新维护独立 installed-state 枚举。

# FE-INSTALL-AUTHORITY-1 接手记录（2026-08-11，released in v0.4.11）

## 改了什么、影响哪些文件

- 新增 `frontend/src/games/stardew/install-state.ts`：dashboard 列表与详情 job 按 ID 合并，terminal 状态单调获胜，并产出唯一 active/latest/selected。`InstallPage.tsx` 不再让本地迟到 running 详情覆盖 dashboard succeeded。
- 当前阶段只解析与 active job ID 相同的日志；历史日志继续显示，但 SteamCMD/SMAPI/认证推断、下载进度和 QR 数据不会从终态历史任务污染当前 UI。
- `ApiError` 新增可选 `details`。安装 409 `install_in_progress` 时页面接管 `details.jobId` 并刷新；仅当返回的是另一任务 ID 时清理旧详情/日志，同一任务重复提交保留当前日志连接。`useSteamAuthLogin` 遇到同一冲突直接进入安装页观察已有任务。
- 新增 `frontend/scripts/test-install-state.ts` 和 npm `test:install-state`。后端接口配套见 `docs/06-integration.md`。

## 如何验证与下一步

- `npm run test:install-state` 已覆盖终态胜过迟到 running、新 active 与旧 selected 隔离、日志 ID 必须匹配；全部 13 项前端状态测试与 `npm run build` 在本机洁净 Node 24 环境和 tag Release workflow 中通过。
- 正式升级后真实双提交由后端返回同一活动 owner 的 202/409，前端使用 `details.jobId` 接管同一任务；成功终态即使历史日志含 `steamcmd_downloading` 也不能复活下载卡。该契约已随 tag `v0.4.11` 发布，Release workflow `31521478699` 和三仓回拉通过。官网 post-release 提交 `e3d40b155dd29cefe1fc9410675bbc91eb91d455` 经 Pages `31523817426` / deployment `5856456646` 上线，compatibility `31523817397` 成功；本地桌面/手机 Browser 与线上四页 HTTP/SSR 内容均通过，线上视觉证据缺口已如实记录在 `docs/09-image-build.md`。
- 后续如果增加“查看历史任务”交互，selected 只能控制日志窗口，不能重新成为 active。不要重新引入 `detailJob ?? dashboardJob` 到达顺序优先级，也不要让日志单独决定最终状态。

# FE-MODAL-HEIGHT-GUARD-1 接手记录（2026-08-09，completed）

## 改了什么与影响

- `.sd-confirm-dialog` 的重复 `max-height:90vh/100%` 合并为 `min(90vh, 100%)`；`.sd-install-qr-card` 的重复 `92vh/100%` 合并为 `min(92vh, 100%)`。两类卡片显式使用 `box-sizing:border-box`，padding/border 不再加到 100% 限高/限宽之外；内容溢出仍由原有 `overflow-y:auto` / `overflow:auto` 承接。
- 影响 `frontend/src/games/stardew/StardewPanel.css`、`pages/InstallPage.css` 和 `scripts/test-responsive-layout.ts`。没有修改弹窗 JSX、危险操作确认逻辑、Steam 二维码数据或安装 API。

## 如何验证

- 全部 12 项前端状态测试与 `npm.cmd run build` 通过。
- QA App 除既有 1180×900、769×500 删除确认框/新建游戏验收外，又在 769×240 与 280×653 打开长 Joja 确认框。两视口卡片四边都在 overlay 内、root/body 无横向溢出、console warn/error 为 0；769×240 卡片 `scrollHeight=303 > clientHeight=210`，滚轮交互使内部 `scrollTop 0→93`，证明低高度内容可达且不把页面撑高。
- 正式 Web 升级得到的新 v0.4.10 bundle 已用合成、非敏感 QR URL 完成二维码实测：769×240 与 280×653 的 card 四边均在 overlay 内、root/body/card 无横向溢出；低高度内部 `maxScrollTop=225` 并可滚到底，280×653 完整装入，两视口均可关闭且 console error/warn 为 0。

## 下一步注意事项

- 桌面 overlay 内的高度上限统一使用 `min(<viewport cap>, 100%)`；不要再用连续两条同属性声明表达“同时满足”，因为普通 CSS 级联只会保留后一条。移动端现有 `100vh` → `100dvh` 是兼容回退，不属于这一问题。

# DOCS-INSTALL-HTTPS-2 接手记录（2026-08-09，released）

- README、`docs/user-guide/getting-started.md`、官网 `guide/deploy` / `deploy/quick-start` 及 `docs/09-image-build.md` 的活动安装命令统一为官方 GitHub Release HTTPS。仅支持 HTTP 的 `anxinas.dpdns.org` 不再作为下载后直接执行入口；旧 `DOCS-INSTALL-HTTP-1` 记录只保留历史背景。
- 安装脚本会操作 Docker 与宿主配置，HTTP 200/长度不能抵御中间人替换。国内网络不稳定时引导用户从浏览器打开官方 Release 手工下载 `run.sh`；未来只有可信 HTTPS 或独立签名/摘要校验才能恢复镜像推荐。
- 未修改脚本内容、镜像选择、Panel API 或安装事务。v0.4.10 发布后活动文档 HTTP 可执行入口为 0；精确版与 `latest` 的 `run.sh` HTTPS 下载均为 30,437 B、SHA-256 `8f0040c11661f2e3f4060c66bf8ba205a33aa46fc65e3dec7cbf15b864c7387a`。

# FE-NEW-GAME-MODAL-LAYOUT-1 接手记录（2026-08-09，completed）

## 改了什么与影响

- 新建游戏的响应式容器从页面级 `sd-main-scroll` 改为宽版弹窗自己提供的 `ngc-modal`；桌面弹窗不再因侧栏压缩主内容区而误判成单列。`sd-saves-modal-card` 同时补 `box-sizing:border-box`，高度上限收敛为 `min(90vh, 100%)`。
- 影响 `frontend/src/games/stardew/pages/SavesPage.css`、`NewGameCreator.css`、`scripts/test-responsive-layout.ts`。没有改变 `SavesSection.tsx`、新建存档请求、表单字段、Mod 农场目录或后端接口。

## 如何验证

- 全部 12 项前端状态测试（含 `npm.cmd run test:responsive-layout`）和 `npm.cmd run build` 通过。
- 应用内 Browser 使用 QA App stopped fixture：1180×1063 打开“存档 → 新建游戏”后为左侧联机设置/中间角色表单/右侧农场选择三栏，顶边对齐、root/body 无横向溢出，增加小屋 0→1；769×500 为预期单列且只在弹窗内部滚动。两个视口均无 console warn/error。

## 下一步注意事项

- 新建游戏断点必须继续查询 `ngc-modal` 自身，不能重新绑定 `sd-main-scroll`；若弹窗外复用 `NewGameCreator`，调用方也要提供同名 inline-size 容器或明确设计新的容器名。调整宽版弹窗 padding/border 时要同时复核 1100px 临界值。

# DOCS-INSTALL-HTTP-1 接手记录（2026-08-09，completed）

- `README.md`、`docs/user-guide/getting-started.md` 和 `docs/09-image-build.md` 的国内加速安装地址统一为 `http://anxinas.dpdns.org/run.sh`；`website/docs/guide/deploy.md` 与 `website/docs/deploy/quick-start.md` 已经是正确 HTTP，无需改动。
- 本次没有变更 VitePress 页面源码、组件、依赖或 Panel 功能。同域名 HTTPS 残留为 0；HTTP 端点只读返回 200/27,427 字节且未执行；GitHub README 线上正文与 `clipboard-copy value` 均为完整 HTTP 命令，console warn/error 为 0；compatibility workflow `31305603385` 成功。

# DOCS-HOME-QQ-COMMUNITY-1 接手记录（2026-08-10，released）

## 改了什么、影响与验证

- `website/docs/index.md` 增加 `heroCommunityCard: true`；`ThemeLayout.vue` 通过官方 `home-hero-actions-after` slot，把 `HeroCommunityCard.vue` 放在首页 Hero 两个 CTA 正下方。整卡直接打开 QQ 官方加群链接，不创建独立路由、顶栏项或第七张功能卡，右侧联机邀请票、六入口和版本区保持原结构。
- `community.ts` 是群链接的唯一来源；非首页统一帮助页尾原 GitHub Issues 外链改为“加群反馈”。两个入口均使用 `_blank` 与 `noopener noreferrer`。卡片采用内联 SVG、44px 触控目标、`focus-visible`、细指针 hover、深色主题、390px/359px 窄屏和 reduced-motion 规则，无新增依赖或图片。
- 产品文件：`website/docs/index.md`、`website/docs/.vitepress/theme/{ThemeLayout.vue,HeroCommunityCard.vue,community.ts,custom.css}`。只影响公开文档门户展示，不改变 Panel React、后端 API、GitHub Issues 内容或发布流程。
- 平板跟进：约 870×710 时默认主题已经把标题、tagline 和 actions 居中，但自定义 `max-width:959px` 规则仍把 main 靠左，文案中心为 `352px`、邀请卡中心为 `428px`。新增仅作用于 `640–959px` 的对齐规则：容器子项、main、品牌行、actions 与加群卡共用邀请卡中心；不把这一规则泄漏到 `<=639px` 手机或 `>=960px` 双栏。
- 首屏节奏跟进：桌面 `.VPHero` 顶部 padding 从 `calc(nav + 66px)` 改为 `calc(nav + 26px)`，Hero 与 Features 同步上移 40px；640–959px 从 `nav + 36px` 收到 `nav + 20px`，上移 16px；手机的 `nav + 24px` 不变。1700×1100 下导航到品牌行从 120px 收到 80px，870×710 为 70px，390×844 仍为 55px。
- 顶栏宽度跟进：`custom.css` 仅对 `>=960px` 且无侧栏的 `.VPNavBar .container` 设置 `max-width:1180px`，取代默认 1376px；1700px 下顶栏与 Hero 均为 `253–1433px`，Logo 与菜单各向中间收约 98px。顶栏背景、导航高度、正文侧栏页面和 `<=959px` 折叠菜单不变。
- 平板紧凑跟进：`640–959px` 的无侧栏顶栏单独使用 `max-width:640px`，870px 下实际宽度 793→640px，搜索按钮到汉堡菜单的可见空档 416→263px；Hero 顶部 padding 同时从 `calc(nav + 20px)` 收到 `var(--vp-nav-height)`，稳定后的导航到品牌行间距为 50px。`<=639px` 手机仍使用原顶栏和 Hero 间距，`>=960px` 仍使用 1180px 完整导航。
- 顶栏垂直留白跟进：870×760 下控件在 `.VPNavBar` 的 64px 内本已居中，但 `.VPSkipLink.visually-hidden` 计算结果仍为 `position:relative;height:16px`，先占普通流再把 `.VPNav` 推到 `y=16`。`custom.css` 现让 `.VPSkipLink` 始终绝对定位，1px 隐藏尺寸和裁切只作用于 `:not(:focus)`；正常态导航为 `y=0..64`，Logo 上下间距 19.5/20.5px、搜索 12/12px、汉堡线组 25/25px。locator 键盘聚焦后链接为 80×40px、文本可见、导航仍为 `y=0..64`，保留跳转正文的可访问性。
- 验证：`npm.cmd --prefix website run docs:build` 通过。应用内 Browser 1440×900 下卡片为 456×72，390×844 下为 299×68，均位于按钮后且 root/body 横向溢出为 0；浅/深主题、文档页尾、键盘/触控语义和 console/overlay 通过。实际点击新开标题“QQ群”的 `qun.qq.com` 标签，完整 URL 与用户提供值相等，原首页保持不变。平板补测 640/768/870/959px 的 main/加群卡/邀请卡中心完全一致；顶栏补测 390/640/768/870/959/960/1024/1440/1700px 的标题、搜索与菜单无重叠或溢出，959px 汉堡菜单可开合，console warn/error 为 0。
- 发布：提交 `63aff0380de337faf57a9a6bcac1323b6e3593f6` 经 Pages workflow `31388822404` 成功部署。线上 1280×720、870×760、390×844 的页面身份、非空、overlay、console、root/body overflow、顶栏对称留白和卡片文字左对齐均通过；真实点击打开标题“QQ群”的精确目标。
- 桌面首屏二次收紧（已发布）：`custom.css` 的桌面 Hero padding 从 `calc(nav + 26px) 24px 70px` 收到 `calc(nav + 10px) 24px 50px`，同一 `min-width:960px` 档让 `.VPHomeFeatures` 上提 12px。1700×1100 顶栏下沿到品牌行 `72.36→48px`，加群卡下沿到首张功能卡 `87.64→64px`；提交 `0508dbef3bb21d751f6333948010dcf534252e85` 经 Pages workflow `31390948240` 成功发布。线上 390/959/960/1700px root/body overflow 与 overlay 为 0，959px 汉堡菜单开/关状态正确，console warn/error 为 0。
- 下一步：若群邀请失效，只更新 `community.ts`，不要在组件或 Markdown 复制第二份链接；后续官网变更继续走 `main`、Pages 与线上桌面/390px 复核。

# DOCS-PORTAL-V049-1 接手记录（2026-08-09，released）

- 官网沿用当前正式布局，只把 `website/docs/index.md` 的 release/版本卡/摘要和 `website/docs/changelog.md` 的最新条目更新到 v0.4.9。首页文案链接到现有 `/changelog`，没有新增组件、脚本、依赖、路由或图片。
- changelog 面向用户准确描述 rollback 恢复、可信旧候选规范化、安全重试、三次上限、未知状态支持包、重启续跑和 auth 保留边界；不暴露 recovery 路径、镜像选择或内部 Docker 命令。
- Node 20 Alpine 隔离副本执行全新 `npm ci` 和 production build 通过；Pages `31305028853`、compatibility `31305028888` 均成功。线上默认桌面首页与 changelog 页面身份、非空白、无 overlay、console health 均通过，首页“查看本次更新”实际进入 `/changelog.html`；390×844 首页/日志 root/body 无横向溢出，三类修复、支持包和 v0.4.8 历史均存在。完整证据见 `docs/09-image-build.md`。

# RELEASE-V0.4.10-FRONTEND-1 接手记录（2026-08-09，released）

- 发布范围：FE-MODAL-HEIGHT-GUARD-1、FE-NEW-GAME-MODAL-LAYOUT-1、FE-STEAM-AUTH-WAIT-VISIBILITY-1，已随 tag `v0.4.10` 发布。精确候选与升级得到的新 Panel 均完成真实页面 QA，不以源码 fixture 的单次通过替代升级后验收。
- 全新 Node 24 `npm ci` 报告 `nanoid 3.3.16` 命中 `GHSA-2v37-7h3g-55p8` high advisory；lockfile 已在 PostCSS 的 `^3.3.16` 范围内精确更新为修复版 `3.3.17`。后续发布门禁须同时跑 `npm audit --omit=dev --audit-level=high`，不能只因 Vite/PostCSS 属于构建工具就忽略已知 high。
- 官网 `package-lock.json` 的 `nanoid 3.3.15 / postcss 8.5.16` 同样升级到 `3.3.17 / 8.5.25`，清除有补丁的 high。VitePress 稳定 latest 1.6.4 仍聚合 1 high + 2 moderate，只影响不会发布的 Vite dev server且无稳定修复；2.0 尚为 alpha。发布验证要求 production audit 0、critical 0 与静态 `docs:build`，禁止把 `docs:dev` 暴露公网。
- 空 Node 24 volume 的前端 production audit 0、12 项状态测试和 build 已通过；官网 production audit 0、critical 0 和 VitePress 静态 build 已通过。两个任务 volume 与相关容器已精确清理。
- 升级后 QA：769×240 的 `verifying_auth` 计时 664→666 秒，280×653 为 667→669 秒；两视口全局 `role=status` 均只有静态标题，动态计时不在 live region，root/body 无横向溢出、console error/warn 为 0。二维码弹窗同样在两视口完成 card/overlay 包含、内部滚动、data image、aria-modal 和关闭交互。
- tag `v0.4.10`、Release workflow `31325589153`、三仓回拉与四项 Release 资产已完成；官网 v0.4.10 已由 post-release 提交 `3457efea561f5fbb865eab440576e91cf2de6ec1` 上线。Pages `31326926817`、deployment `5821195957` 和 compatibility `31326926808` 均成功；线上 1440×900、390×844 首页与更新日志、实际点击、最新/历史版本、横向溢出和 console/page/request health 已全部复核。
- 线上视觉终验最初在 Hero 入场和平滑回顶中间帧截到发灰/历史位置；等待 opacity/scrollY 稳定后，1440×900 与 390×844 均正确显示日志顶部，普通发布路径 console/page/request health 为 0，未因测试截图时序改变导航语义。额外的极快历史切换压力序列可触发 VitePress 1.6.4 默认 outline 空引用，当前线上正式 CSS 同样复现；尝试普通文档导航会丢失 back scroll，已撤回。后续只在稳定 VitePress 升级或经评审的 outline guard 中处理，见 `docs/07-later-optimizations.md`。

# FE-RUNTIME-UPDATE-REPAIR-CATALOG-3 接手记录（2026-08-09，completed，v0.4.9 released）

- `types.ts` 增加 `JunimoUpdateRepairPlan`；`DiagnosticsPage.tsx` 只渲染后端给定的检测、方法、步骤和按钮文字。三个自动按钮分别是“修复：恢复旧版后升级”“修复：规范配置并升级”“修复：重新预检并升级”，不安全/未知情况变成“保留现场并导出支持包”，运行中/矩阵不安全显示等待按钮。
- repair 仍只提交严格确认并轮询已有 apply；export 复用支持包；wait 不发请求。正常“立即升级”只在没有计划时显示。`qa-layout-main.tsx` 的 rollback/config fixture 同步了契约。
- Docker Desktop Vite 容器经 Codex Browser 实际点击 rollback 修复，事件为 `repair:POST,repair-apply:GET,repair-apply:GET`，最终成功且 console warn/error 为 0；配置修复、安全重试、未知故障导出和活动事务等待按钮也已逐一匹配，export 在 `up_to_date` 状态下仍可见，wait 确认为禁用。前端 12 项状态脚本和 production build 通过。
- 后续 UI 不应按 `phase/errorCode` 再造映射；增加新错误时由后端返回新计划，前端只需保持通用动作渲染与权限边界。

# FE-RUNTIME-UPDATE-DIAGNOSE-REPAIR-2 接手记录（2026-08-09，completed，未发布）

- Diagnostics 的两个已知故障入口统一为“检测、修复并升级”：`rollback_failed` 和 `repairable/legacy_candidates` 都只 POST 严格 repair API，并轮询持久化 apply 状态；浏览器不再自己串联 config repair/dry-run/apply。
- 页面新增 `resuming_upgrade`，显示修复源 apply ID 和后端 checks。确认框说明未知问题会停住；只有 `succeeded` 是完成，修复后 `failed_rolled_back` 仍显示重新升级失败但旧版安全。
- 影响 `DiagnosticsPage.tsx`、`types.ts`、Junimo 状态 helper/测试和 QA fixture。按钮仍为管理员专属、三次上限、无恢复路径/镜像/命令输入；Docker Desktop QA 容器中已由无代理 Chrome 实际点击并观察四阶段、展开核对修复源和六项 checks，console error/warn 为 0。Codex Browser 插件本机 URL 被客户端拦截，准确 fallback 原因、截图和候选镜像证据见 `docs/09-image-build.md`。

# FE-RUNTIME-UPDATE-REPAIR-1 接手记录（2026-08-08，completed，未发布）

## 改了什么、影响与验证

- Diagnostics 版本维护卡在 Junimo `rollback_failed` 时为管理员显示“一键安全恢复”，确认后只提交严格 `{"confirm":true}`；普通用户没有写入口。202 后继续复用 apply polling，显示首次失败、回滚步骤、`repairAttempts` 和最终恢复结果，三次后禁用按钮。
- `api.ts` 新增 `startJunimoUpdateRepair`，`types.ts` 增加可选 `repairAttempts`；状态标签改为“自动回滚失败，可一键安全恢复”。没有加入浏览器自动重放、恢复路径/镜像选择或 Docker 命令。
- 影响 `DiagnosticsPage.tsx`、`junimo-update-status.ts`、API/types 和 `test-junimo-update-status.ts`。状态测试、TypeScript/production build 与 Docker 候选验证记录在 `docs/09-image-build.md`。
- 后续如给 SMAPI 增加 repair，必须复用相同“只在后端已有 rollback_failed 事务时显示、严格确认、次数有界”的模式，但不能把两个状态文件或接口混用。

# 官网隔离改版撤回交接（2026-08-07，completed）

- `6f34b8a` 误将 `docs-portal-redesign` 隔离稿发布到官网；修复已把所有非玩家 Mod 站点文件恢复到 release commit `0c5e2c4`，删除 `DocsHome.vue`，只保留首页 v0.4.8 版本卡、changelog 和玩家手册新增内容。
- Node 24 VitePress build、Pages `31152244079` 与线上 1440×900/390×844 Browser 通过：原 Hero、联机邀请卡、六入口、原 FAQ 恢复，v0.4.8/CJB/unavailable 内容存在，无溢出或 console warn/error。不要再次从已删除 worktree 或 `6f34b8a` 恢复任务型门户。

# v0.4.8 玩家 Mod 前端与门户发布交接（2026-08-07，released）

- 玩家列表/待认证 CJB 文字提示、桌面静态详情、手机列表/详情子视图、四组比较、内置项过滤与 unavailable 边界已随 `v0.4.8` 发布。release/compatibility workflow `31117969497/31117949897` 成功，三仓正式镜像已分别完成 health/version smoke。
- `docs-portal-redesign` 曾因范围理解错误进入 `main` 和 Pages，现已撤回；Pages `31150162173` 只记录误发布，不是当前目标布局。玩家 Mod 页面与 v0.4.8 文案继续保留，历史 worktree 与非 main 分支仍保持已清理状态。
- 后续维护必须继续共用 `PlayerModsDetail`，保持 CJB 明文、不以颜色独立表达、不把 `mods:null` 当零项、不把 `server_only` 放进玩家缺少。实体 PC 原版/CJB/移动客户端仍未验证。

# FE-PLAYER-MOD-PRESENTATION-2 接手记录（2026-08-06，completed，v0.4.8 released）

## 改了什么、影响与验证

- `PlayerModsDetail` 的统计/分组顺序改为“玩家额外安装 → 玩家缺少 Mod → 版本不同 → 匹配”，结果徽标同步改名。CJB 横幅和条目删除两段解释，只保留“检测到该玩家使用了 CJB 作弊工具”与“检测到 CJB 作弊”直接文字提示。
- `player-mod-details.ts` 防御性过滤 `Pathoschild.SMAPI`、`JunimoHost.Server`、`AnXiYiZhi.StardewAnxiPanel.Control`，所以旧后端或 QA 缓存返回这些条目也不会进入统计/分组。普通第三方 `server_only` 防线不变。
- 影响 `PlayerModsDetail.tsx/.css`、`player-mod-details.ts`、状态测试与 QA fixture。`npm run test:player-mods`、`npm run test:responsive-layout`、`npx tsc -b` 和 production build 通过；桌面与 390×844 手机 Browser 均验证新顺序、旧说明/三类内置项不存在，手机无横向溢出。
- UI 仍只读；CJB 清单仍是可绕过的客户端自报，不能升级为自动管理。后续修改组名或顺序时必须桌面/移动共用同一主体，避免两端漂移。

# FE-PLAYER-MOD-CJB-LABEL-1 接手记录（2026-08-06，completed，v0.4.8 released）

## 改了什么与影响

- 玩家列表 API 的可选 `modRiskFlags` 已进入 `StardewPlayerInfo`。桌面/手机列表共用 `hasPlayerCjbRisk` 与 `playerModActionLabel`：命中时原“查看上报 Mod”按钮显示“检测到 CJB 作弊”；桌面待认证卡和手机总览待认证卡显示同文案红色徽标。
- 共享详情 `PlayerModsDetail` 的总横幅改成“检测到该玩家使用了 CJB 作弊工具”，CJB 条目徽标同步改为“检测到 CJB 作弊”。后续已按反馈移除两段解释；原详情路由/API 与只读行为不变。
- 影响 `types.ts`、`player-mod-details.ts`、`PlayerModsDetail.tsx`、桌面/手机玩家页与 CSS、手机总览、QA fixture 和状态测试。没有增加管理请求，也没有改变认证批准流程。

## 如何验证与下一步

- `npm run test:player-mods`、`npm run test:responsive-layout`、`npx tsc -b`、production build 通过。Browser 在桌面玩家列表/待认证卡/详情及 390×844 手机总览/玩家列表/详情均看到显式文案，手机 root/body 横向溢出为 0。
- `modRiskFlags` 可能来自 stale 的最后有效自报；不要把徽标改成“服务端确认作弊”，也不要据此自动禁入。修改 CJB manifest ID 仍可能绕过。

# FE-PLAYER-MOD-COMPAT-1 第三阶段接手记录（2026-08-06，真实 PC+SMAPI 数据通过，页面矩阵受限，v0.4.8 released）

## 改了什么、影响与验证

- 页面实现未改动；扩展 `scripts/test-player-mod-details.ts` 的兼容 fixture，覆盖 PC 原版/Android/iOS 的统一 unavailable 解释、pending/stale/请求失败、四组比较、server_only 防线、两种 CJB、篡改 UniqueID 不命中、大小写重复、256 字名称和超长版本。
- `unavailable` 必须继续隐藏全部统计并禁止“0 个 Mod”“完全一致”“安全”；只有 `reported + comparison.available` 才展示数字。CJB 仍同时依赖红色视觉和明确 CJB 文字，不附加解释段落。
- 阶段二 Browser 的桌面/280px 证据继续有效。本阶段 12 项状态测试、独立 TypeScript 检查与 production build 通过。本机真实 PC+SMAPI 联机后，接口准确返回三个真实 Mod、两项 match/两项 client_only/零缺失/零版本差异，并经历 stale、重连新 reportedAt 和 server 重启 stale；production dist 嵌入测试 Panel 后详情静态路由返回 `200 text/html`。

## 下一步注意事项

- 应用内 Browser 本轮被本地地址策略拦截，因此没有实际打开真实登录后的详情页；不要把真实 API/路由 200 写成页面视觉通过。下一位仍需从桌面按钮和移动列表分别进入详情，补 PC 原版/CJB/移动端、pending/unavailable、新旧玩家与多玩家视觉；Android 实验性 SMAPI 没有环境就保持未验证。
- 清单为客户端自报，改 CJB manifest UniqueID 可绕过；前端不得升级为自动处罚。接口/路由没有变化，不要新增轮询式管理请求或自行从 `mods:null` 推断零项。

# FE-PLAYER-MOD-VIEW-1 玩家 Mod 详情接手记录（2026-08-06，completed，v0.4.8 released）

## 改了什么

- 新增桌面静态详情 `/instances/stardew/player-mods?playerId=...`。桌面玩家表只有存在联机 ID 才显示“查看上报 Mod”；详情路由仍点亮玩家侧栏。`StardewPanel` 同步跟踪 search，使同一路由不同 playerId 和浏览器前进/后退可重渲染。
- `PlayerModsDetail.tsx/.css` 共用姓名/状态/版本、CJB 文字警示、统计、分组、加载/空/失败状态和响应式布局；桌面由 `pages/PlayerModsPage.tsx` 包装，移动 `MobilePlayersPage` 维护“列表/Mod 详情”子视图与返回按钮。移动壳从详情路径打开时初始选择玩家 Tab。
- 类型/API 位于 `types.ts` 与 `api.ts`；`player-mod-details.ts` 负责状态解释、官方 CJB ID 判断、大小写不敏感去重及 `server_only` 缺失防线。页面只读，不新增任何管理请求。

## 影响文件与接口

- 前端入口与路由：`games/stardew/{stardew-routes,StardewPanel,StardewMobileShell}.tsx`、`pages/{PlayersPage,PlayerModsPage}.tsx`、`mobile/MobilePlayersPage.tsx` 及对应 CSS。
- 共享详情与测试：`games/stardew/{PlayerModsDetail,player-mod-details}.*`、`scripts/test-player-mod-details.ts`、`qa-layout-main.tsx`、`package.json`。
- 后端仅修改 `internal/web/handler.go` 的 SPA 精确白名单并新增路径单测；数据继续读取第一阶段的 `GET /api/instances/:id/players/:uniqueMultiplayerId/mods`，没有修改比较或玩家连接流程。

## 如何验证

- 通过全部 12 项前端状态脚本、独立 `npx.cmd tsc -b`、`npm.cmd run build`；`go test ./internal/games/stardew_junimo ./internal/web` 通过。
- 应用内 Browser：桌面从玩家名册进入，URL 精确携带 playerId；CJB 重复项去重为 1。后续 390×844 复验确认三类面板内置组件隐藏、普通 user Mod 的玩家缺少计数仍为 1，移动详情无根级横向溢出，长名称/版本正常换行。
- `unavailable` 精确文案存在且统计/“0 个 Mod”/“完全一致”/“安全”均不存在；pending、stale、HTTP error 使用不同提示，error 有重试。console error 为 0，QA 端口已释放。

## 下一步注意事项

- `mods:null` 永远是“没有取得可靠清单”，不能转换成 `[]`；只有 comparison available 才能显示统计。当前 stale 不可比较，不要用缓存项自行计算“仍一致”。
- 不得把 `server_only` 缺失放入警告，也不得把 CJB 提示升级为踢出、封禁或自动拦截。后端 summary 只作契约参考，当前 UI 从去重后的 items 计算可见统计以避免重复条目误计数。
- 标准 IP peer context 之外、Steam SDR、原版/移动端或不兼容 SMAPI 仍可能无法提供 ModContext；这是数据边界，不是前端错误。后续若扩展详情必须继续复用共享主体和这组状态测试。

# 2026-08-06：真实移动端侧栏预览

- `frontend/src/App.tsx` 现在识别 `shell=mobile`，可在桌面宽度的 Codex 侧栏或普通浏览器中打开真实、已登录的 `StardewMobileShell`，不再依赖会在控制结束后复原的临时 viewport 模拟；强制手机标签会以“Stardew Anxi Panel · 手机端”作为标题。
- 查询解析集中在 `frontend/src/games/stardew/responsive-layout.ts` 的 `shouldForceCompactShell`，边界用 `frontend/scripts/test-responsive-layout.ts` 覆盖。仅精确的小写 `mobile` 生效，其它值不改变自动适配。
- 验证：运行 `npm run test:responsive-layout` 与 `npm run build`，然后同时打开 `/instances/stardew/players` 和 `/instances/stardew/players?shell=mobile`，前者应为桌面 Shell，后者应出现“移动端主导航”。

# DOCS-HOME-HERO-INVITE-1 接手记录（2026-08-01，released）

## 改了什么

- 首页 frontmatter 增加 `heroInviteCard: true`，Hero H1 固定为两行“一键部署你和朋友的 / 专属联机服务器”；原 `name` 与默认 `image` 删除，两个 CTA、tagline、六张入口卡及 `v0.4.6` 版本摘要保持不变。
- `ThemeLayout.vue` 使用官方 `home-hero-info-before` 与 `home-hero-image` slot 注入品牌 kicker 和 `HeroInviteCard.vue`。邀请卡用现有 Logo、联机状态、玩家到服务器关系、三类能力和数据自持说明形成唯一右侧主视觉；无点击、无额外 tab stop，文本由语义化 `aside/dl` 提供。
- `custom.css` 改为开放式 Hero；浅色按钮是低饱和森林炭黑，暗色按钮是雾灰绿，暗色邀请卡为暖石墨，不再铺满高饱和绿色。响应式在 959px 以下改为上下排列，639px 以下收紧字号与卡片，360px 以下 CTA 纵向堆叠；reduced-motion 取消入场和 hover 位移。
- 首页入口卡首行 hover 曾被 `.VPHomeFeatures { contain: layout paint style; }` 裁切。只对 Features 移除 `paint`，保留 `layout style`；`.VPHomeContent` 继续 paint containment，卡片自身 `overflow:hidden`、gutter 和 `translateY(-4px)` 均不变。

## 影响、验证与下一步

- 产品文件：`website/docs/index.md`、`website/docs/.vitepress/theme/{ThemeLayout.vue,HeroInviteCard.vue,custom.css}`。没有更改 VitePress 路由、版本角标来源、Panel 前端或接口。
- Docker Desktop Linux Node 20 production build 通过。应用内 Browser 验证页面身份、非空、浅/深主题切换与 console；隔离 Chrome/Playwright 验证 1280px hover 时卡片上移 4px、越出 Features 4px仍可命中，390×844/320×568/768×1024 零横向溢出，reduced-motion 无动画。颜色对比最低为暗卡状态色 `6.15:1`。
- Pages workflow `30659672364` 的 build `91252609607` 与 deploy `91252685043` 均成功；deployment `5697130212` 状态 `success`，environment URL 为正式站点并精确绑定 `81b5716`。线上 cache-bust 验证桌面浅/深主题、邀请卡与六入口均存在，1280px hover 的 `translateY(-4px)` 越出 Features 4px 后顶部仍可命中；390×844 首页与 768×1024 正文移动菜单零横向溢出，overlay 与 console error/warn 为空。后续不要恢复整块 Hero 实色底、全屏网格或放大的低分辨率 Logo；更换邀请卡结构时继续验证浅/深色、390px、键盘、reduced-motion 和首行卡 hover 越界。
# FE-RESPONSIVE-VIEWPORT-1 全窗口矩阵接手记录（2026-08-01，completed，v0.4.7 released）

## 改了什么

- 修复平板无法滑动、全屏只显示部分内容以及部分电脑浏览器根 Shell 尺寸失效，支持边界为 280 CSS px 及以上。手机 `<=768px` 与 `<=1366px` 无 hover/粗指针设备进入紧凑壳；普通窄电脑保留完整桌面壳。紧凑壳可进入完整桌面版，自动紧凑条件成立时桌面侧栏可切回“适配版”。
- 根 Shell 缩放改为 `responsive-layout.ts` 的普通数值计算；wrapper 按实际内容盒驱动 ResizeObserver/resize/fullscreen 重算。外层改用 `overflow:clip` 并监听归零，修掉浏览器把隐藏容器程序化滚动后整套面板上移的直接回归；`.sd-main-scroll` 与 `.sd-mshell-scroll` 分别是桌面/紧凑壳唯一主滚动区，路由切换归零。
- 紧凑壳覆盖四边 safe area、惯性滚动、双轴 pan/pinch、16px 输入和 44px 关键触控区；五个移动页最大内容宽扩大到 1120px，280px 底栏退化为仅图标。登录/初始化页在超宽低高度等危险比例使用可滚动文档流；弹窗限高改为相对可用容器内部滚动并限制 overscroll 链，窄屏重连/二次确认不会覆盖 safe area。
- 新建游戏加入容器查询与 viewport fallback，玩家/控制/模组/存档操作区在 280px 可换行；OpsRail 可独立纵向滚动；缺失 ResizeObserver 时安全降级。QA 入口新增真实 App login/setup/panel 状态与完整 API mock。

## 影响文件

- 逻辑：`frontend/src/App.tsx`、`hooks/useMediaQuery.ts`、`games/stardew/{responsive-layout,StardewPanel,StardewMobileShell,NewGameCreator}.tsx`、`pages/ModsPage.tsx`、`qa-layout-main.tsx`。
- 样式：`App.css`、桌面 Shell/更新/新建游戏、五个 `mobile/Mobile*Page.css` 及存档/模组/安装弹窗 CSS；QA viewport meta 同步生产。
- 测试/CI：`frontend/scripts/test-responsive-layout.ts`、`frontend/package.json`、release/compatibility workflow。后端 API、鉴权和 Junimo 通信未改变。
- 发布展示：`website/docs/index.md`、`website/docs/changelog.md` 更新当前版本与用户可见变更；不修改官网主题组件和导航结构。

## 如何验证

- `test:responsive-layout` 已逐像素覆盖 280..3840 × 16 高度并包含 7680×4320；项目全部 11 项前端状态测试和 `npm run build` 通过。两个 workflow 已接入专项命令。
- Browser：移动六页覆盖 280×653/320×480/480×320/820×1180/1366×768，桌面九页覆盖 280 强制桌面、769×500、1366×768、2560×720；认证页覆盖 280×653 至 2560×720。768/769 自动断点、双向壳切换、横屏确认/更新弹窗、低高度超长新建游戏弹窗均通过。
- 结果：页面级横向溢出为 0，Shell/overlay 精确覆盖视口，外层 viewport scroll 始终 `[0,0]`，长内容由内部容器滚动，关键按钮至少 44px，console error/warn 为 0。用户已确认实体平板横竖屏滑动、浏览器全屏、底栏切页和输入法冒烟通过；按用户明确决定，不再等待曾复现问题的朋友电脑验收，该设备未复验保留为剩余风险。
- 正式候选还完成了 320×568、390×844、768×1024、1024×768、1440×900、1920×1080 六视口真实登录矩阵；随后由正式 `0.4.6` 在 Web 内分别完成 unhealthy `failed_rolled_back/health_check_failed` 和精确候选成功升级。升级后的 Panel 重启及 390×844/1920×1080 新功能复验通过，非目标 game sentinel 与长期数据保持。
- 发布证据：tag `619d18dafa76`；Release/compatibility/Pages workflow `30662967983/30662818759/30662818712` 均成功；Docker Hub、ACR、GHCR 的 `0.4.7/latest` digest 均为 `sha256:3f336863ae5ec45a1997edcfc0922269250d5763e8ada49a7ba43f81d59edd7f`，三仓精确镜像分别通过隔离 health/version smoke。

## 下一步注意事项

- 不要恢复 `body/window` 或 `.sd-shell-viewport` 滚动；桌面唯一主滚动区是 `.sd-main-scroll`，紧凑端是 `.sd-mshell-scroll`。`overflow:clip` 后面的 scroll 归零监听是旧浏览器 fallback，不能因现代 Chrome 看似不触发就删除。
- 任务、诊断、安装和设置仍没有原生紧凑页面，但“更多”已提供完整桌面版入口；普通窄电脑继续默认桌面壳，不能无条件扩大纯宽度分流。桌面版选择只保持到本次 React 会话，刷新或退出后会恢复自动分流。
- 不要把有限矩阵写成“所有设备数学证明”：公开支持从 280 CSS px 起；Browser 不能直接模拟非零 safe-area、真实触摸惯性、虚拟键盘和厂商全屏栏，实体平板验收记录与自动矩阵都必须保留。根 Shell 已不依赖单位相除，但装饰性 container unit 仍可能在旧内核有细微差异。
# DOCS-HOME-CARD-POLISH-1 接手记录（2026-08-01，released）

## 改了什么

- 官网首页六张入口卡全部移除序号/`NEW` icon，不保留空占位；六条原路由与 CTA、快速上手“推荐”和动态 `v0.4.6` 版本角标继续保留。
- `custom.css` 复用 guide/deploy/maintain/handbook/changelog/faq 六个栏目色，建立顶部短强调线、轻量色洗、细边框/阴影和底部 CTA 分隔；卡片高度与间距收紧，桌面三列和手机单列均由 flex 保持 CTA 对齐。
- hover 仅在 fine pointer 下产生轻微位移；键盘焦点可见，深色主题有独立对比色，reduced-motion 下卡片及箭头都不位移。

## 影响、验证与下一步

- 产品影响文件：`website/docs/index.md`、`website/docs/.vitepress/theme/custom.css`；没有修改 Hero/说明文案、站内路由、Panel React 或 HTTP API。
- Docker Desktop Linux 的 Node 20 Alpine 隔离构建通过。应用内 Browser 与隔离 Playwright 验证 1280px 浅色/深色、390×844 手机单列、hover、键盘焦点、reduced-motion 与 changelog 导航；六卡、零 icon、无横向溢出/overlay，console error/warn 为空。Pages workflow `30655296293` 的 build/deploy 均成功，deployment `5696310887` 精确绑定 `c19b889`；线上桌面/手机/深色复核仍为六卡、零 icon、无溢出和零 console/page error，版本入口正确进入 `v0.4.6` changelog。
- 这些卡片是并列文档入口，不是顺序流程；后续不要为装饰重新加入序号 icon。新增 feature 时继续复核桌面三列、手机单列、CTA 对齐、深色主题和 reduced-motion。

# DOCS-HOME-PATH-REMOVE-1 接手记录（2026-08-01，completed）

## 改了什么

- 官网首页删除整块“START HERE / 从一台服务器，到朋友加入农场”四步流程区域；后续 `CURRENT RELEASE` 版本卡直接承接 6 张入口卡。
- 同步删除 `custom.css` 中仅由该区域使用的 `.home-path`、`.home-section-heading`、`.home-path-grid` 及桌面/移动断点规则，没有留下隐藏占位或无引用样式。

## 影响、验证与下一步

- 影响文件：`website/docs/index.md`、`website/docs/.vitepress/theme/custom.css`；未修改 Hero、入口卡、版本卡内容或站内链接。
- Node 24 Alpine 隔离容器完成 VitePress production build。应用内 Browser 在 1280px 与 390×844 确认目标文本和 `.home-path` 节点均为 0，入口卡到版本卡间距为 22px，页面无横向溢出、overlay 或 console error/warn；“查看本次更新”仍进入 changelog。
- 后续若需要增加新首页模块，应从用户当前需要的单一行动出发，不恢复这块四步说明；新增模块仍须同时验证桌面三列卡尾部与手机单列卡尾部的衔接。

# DOCS-HERO-COPY-1 接手记录（2026-08-01，completed）

## 改了什么

- 官网首页 Hero 主文案由“把开服这件事，变得像打开网页一样简单”调整为“一键部署你和朋友的专属联机服务器”，把核心利益点收敛到一键部署、专属服务器与朋友联机。
- 只修改 `website/docs/index.md` 的 Hero 文案；未调整主题 CSS、按钮、图片、页面结构或站内链接。`docs/03-frontend.md` 同步记录当前文案。

## 影响、验证与下一步

- Node 24 Alpine 隔离容器完成 VitePress production build。应用内 Browser 在 `1280px` 桌面与 `390×844` 手机视口确认 H1 精确包含新文案，页面无横向溢出、framework overlay 或 console error/warn。
- 实际点击首页“浏览完整手册”后进入 `/handbook/`，页面标题为“深度文档 | Anxi Panel 文档”。后续若再次调整 Hero 长度，至少复核 390px 下的断行与按钮首屏位置；本次无需修改字号或断点。
# FE-MOD-LIST-SEARCH-1 接手记录（2026-07-31，v0.4.6 released）

## 改了什么

- `mod-list-utils.ts` 统一桌面/手机的本地搜索和排序：默认 `installedAt` 降序，缺失/非法时间排后，支持名称正反序；搜索覆盖名称、普通 ID、UniqueID、文件夹、作者、包/来源名和 Nexus 数字 ID，并支持去常见分隔符匹配。
- 桌面“添加模组”和“配置模组”共享查询/排序状态；移动端“服务器模组”使用相同选项。过滤只影响渲染，批量启停与统计继续使用完整列表。安装时间在卡片展示，旧记录显示未知。
- 新增 `scripts/test-mod-list-utils.ts`、`npm run test:mod-list` 并接入两个 workflow。`package-lock.json` 同时把受公告影响的 PostCSS 8.5.15 更新到兼容安全补丁 8.5.25，最终 production audit 为 0。
- 官网首页、changelog、维护页和深度 Mod 手册已准备 `v0.4.6` 文案，明确多 ZIP/单 ZIP 多 Mod、嵌套 ZIP 不递归、三类安装入口都记录时间以及旧 Mod 时间未知的兼容行为。

## 影响、验证与下一步

- 主要文件：`types.ts`、`games/stardew/mod-list-utils.ts`、桌面/手机 Mods 页面与 CSS、前端 package/workflow。后端可选字段契约见 `docs/06-integration.md`。
- Docker Linux 中 `npm ci`、十项测试、audit 和 build 通过。Browser 在 1440×900/390×844 验证时间排序、UniqueID/Nexus ID 搜索、名称排序、空状态和配置页；无横向溢出、overlay、console error/warn。
- Docker Desktop 候选 Panel 的真实登录态右侧栏已复验桌面与 390×844：一键下载项可按来源 Nexus ID `4242` 搜索，本地项可按 UniqueID 片段搜索，添加/配置页共享查询，默认最近安装与名称排序正确；Panel 重启后时间仍显示。手机/桌面均无横向溢出，console error/warn 为空。
- 如后续增加“仅启用/仅禁用”等过滤，仍须让一键启停按钮基于完整列表计算，或明确改名并增加后端显式作用域，不能静默只操作可见结果。

# DOCS-OUTLINE-FOLLOW-1 接手记录（2026-07-29，completed）

- 根因：VitePress `useActiveAnchor` 只切换 `.outline-link.active` 并移动 marker；`section-changelog .VPDocAsideOutline` 是独立 `overflow-y:auto` 容器，框架不会替它更新 `scrollTop`。
- `ThemeLayout.vue` 用 `MutationObserver` 监听 outline link 的 class 变化；active 超出中间 28%–72% 舒适区时，平滑滚到约 42% 高度。`prefers-reduced-motion` 改为即时移动；route/resize/卸载分别负责重连、校正和清理。
- 影响文件仅 `website/docs/.vitepress/theme/ThemeLayout.vue`。不要改成 window 每次 scroll 都强制居中，否则会抢夺用户手动滚动目录；后续升级 VitePress 时需复核 `.VPDocAsideOutline`、`.outline-link.active` 类名和 active 更新契约。
- 验证：`npm.cmd --prefix website run docs:build` 通过；1440×900 真实滚轮验证目录向下、向上、页底 active 可见与维护页往返恢复；点击 v0.3.13 锚点正确。390×844 目录保持隐藏，页面宽度等于 viewport，无 overlay，console error/warn 为 0。
- Pages 工作流 `30423428794` 成功。线上 cache-bust 版本日志在 v0.1.x 区域目录达到最大滚动位置，向下/向上 active 均可见，console error/warn 为 0。

# DOCS-PORTAL-0.4.5 接手记录（2026-07-28）

- GitHub Pages 首页、版本日志、安装手册、FAQ 和更新面板页同步到 `v0.4.5`；版本角标继续只读 `index.md` frontmatter 的 `release`，未修改主题 CSS。
- 面向用户说明固定受审代理顺序、分块续传、连续无进展回退、最终完整性校验与 GitHub 官方兜底；明确旧版本 SMAPI 失败后升级 Panel 并执行“重新安装 / 修复”即可继续，不重复 Steam 认证或游戏/SDK 下载。
- 更新页披露 `v0.4.4/v0.4.3/v0.3.13 → v0.4.5` 的代表性生产一键升级验证，不能扩写成所有历史版本均已逐一验证。
- 影响文件：`website/docs/{index,changelog}.md`、`website/docs/{handbook/install,faq/index,maintain/update}.md`。Panel React、API 与主题样式无变化。
- `npm.cmd --prefix website run docs:build` 已通过。应用内 Browser 已真实点击首页版本卡进入 changelog、FAQ 进入安装手册 SMAPI 锚点；首页、changelog、FAQ、安装手册和更新页在 390×844 下均无横向溢出，无 framework overlay，console error/warn 为 0。
- Pages 工作流 `30372623636` 已成功；线上五个目标页面逐一使用 cache-bust 请求复核为 HTTP 200，首页与 changelog 已显示 `v0.4.5`，FAQ/安装页/更新页均包含新增说明。

# DOCS-PORTAL-0.4.4 接手记录（2026-07-28）

- 公开首页与 changelog 更新为 `v0.4.4`；版本角标继续只读 `index.md` frontmatter 的 `release`，没有在 CSS 重新硬编码。
- 两份存档公开文档已从旧现实时间三策略改成当前游戏日回档机制，并明确后台即时生成、默认连续保留五个游戏日、手动/保护备份不参与清理和历史缺失无法补齐。
- 影响 `website/docs/{index,changelog}.md`、`website/docs/{handbook/saves,maintain/saves-backup}.md`。Panel React 代码未改；VitePress build 与 Browser 桌面/手机验收结果记录在 `docs/11-docs-portal.md`。
- 本地 Browser 真实点击验证：首页 `v0.4.4` → changelog 导航成功；手册索引 → 存档管理、更新面板 → 存档与备份两条站内链路均成功。1440×900 和 390×844 无页面级横向溢出，相关页面 console error/warn 为 0。
- Pages 工作流 `30293213908` 已成功；线上首页和存档手册再次验证为 `v0.4.4` 新文案，不再处于“待发布”状态。

# DOCS-PORTAL-SITEWIDE-1 接手记录（2026-07-22，completed）

## 改了什么

- 新增 `ThemeLayout.vue` 包装 VitePress 默认 Layout，按 base 后的路由识别 `guide/deploy/handbook/maintain/faq/changelog`，为所有非首页页面注入阅读进度、侧栏知识库品牌区、面包屑/栏目状态和统一帮助页尾；`config.ts` 补齐搜索、移动菜单、主题、返回顶部、跳正文和最后更新的中文文案。
- `custom.css` 为所有 Markdown 结构建立栏目级设计系统：标题与步骤编号、列表、代码、表格、custom block、图片、目录、翻页和移动端；FAQ 与 changelog 有独立表达，深色主题使用相同结构的高对比配色。
- 首页性能专项移除 6 张约 9.7 万像素卡片的 `backdrop-filter`、10.9 万像素 Hero 模糊动画、品牌图和按钮的额外毛玻璃，改为带 `contain` 的静态表面。计算样式复核结果从 1 个动画 + 8 个首页额外模糊/滤镜层降为 0 个动画、0 个元素滤镜、只剩全站共用导航模糊。

## 影响文件、验证与下一步

- 影响：`website/docs/.vitepress/theme/ThemeLayout.vue`、`theme/index.ts`、`theme/custom.css`、`.vitepress/config.ts`。`npm.cmd run docs:build` 通过；全部 26 个非首页路由无横向溢出、均有标题/栏目上下文/帮助卡，六类手机代表页无横向溢出，浅色/深色 console error/warn 为空。
- 新增顶级栏目时必须同步 `ThemeLayout.vue` 的 `sections` 和 `custom.css` 的 `section-*` 变量；站点部署在 project base 下，路由识别必须继续先剥离 `site.base`。首页不要重新引入持续动画、大面积 `filter/backdrop-filter` 或覆盖整个首屏的固定半透明层。

# DOCS-PORTAL-MODERN-1 接手记录（2026-07-22，completed）

## 改了什么

- `website/docs/.vitepress/theme/custom.css` 从局部高亮扩展为全站设计系统，覆盖品牌变量、玻璃导航、首页网格/柔光 Hero、入口卡、操作路径、版本摘要，以及正文页的侧栏、目录、标题、代码块和表格；深色主题与 reduced-motion 均有独立处理。
- `website/docs/index.md` 重写首屏产品主张，把 7 张默认功能卡收为 6 张序号入口，并新增四步开服路径与 v0.4.1 版本摘要。原有文档信息架构、路由和 GitHub Pages base 保持不变。

## 影响文件、验证与下一步

- 影响：`website/docs/index.md`、`website/docs/.vitepress/theme/custom.css`。执行 `cd website && npm.cmd run docs:build` 通过；1440px 桌面、1280px 深色正文、390×844 手机均无横向溢出，console error/warn 为空。
- 首页 6 张 feature 当前刻意不配置 icon，避免恢复无意义的序号胶囊。以后改当前版本时，同时更新首页 feature 说明、动态版本角标、底部版本摘要和 changelog；新增卡片后重新核对 VitePress 的桌面自动列布局与手机单列 CTA。

# FE-SAVE-NAME-DELETE-1 接手记录（2026-07-20，completed）

## 改了什么

- `SaveInfo.nameWarning` 贯穿类型与存档卡。历史目录编码异常时桌面端显示后端警告、禁用选择/启动，保留删除等恢复操作；新增 `save_name_encoding_invalid`、`save_delete_failed` 中文映射。
- `handleDelete` 在请求成功或失败后都并行刷新 saves/backups，避免后端实际删除后界面仍保留旧卡片。刷新使用 `Promise.allSettled`，之后必定解除 busy；成功时仍触发状态和外层存档刷新。

## 影响文件、验证与下一步

- 影响：`frontend/src/types.ts`、`frontend/src/core/helpers.ts`、`frontend/src/games/stardew/SavesSection.tsx`。九项状态脚本和 production build 通过，Docker HTTP E2E 验证 UTF-8 名称和删除后权威空列表。
- 后续若手机端增加存档选择/删除卡片，必须复用同一 `nameWarning` 禁止激活和“删除后始终刷新”契约；不要根据显示文本重新拼接或修复名称。

# FE-FARMHAND-DELETE-1 接手记录（2026-07-18，completed）

## 改了什么

- `PlayersPage.tsx` 与 `MobilePlayersPage.tsx` 增加删除人物按钮、共享后端 capability 门禁、活动 job 防重复和一致确认框。在线真人数不包含虚拟 host；人数大于零时显示用户约定的小屋残影/位置异常/重连/整档备份全文。
- `api.ts/types.ts` 增加删除请求与人物 capability；`SavesSection.tsx` 增加“删除人物前保护备份”，`JobsLogsPage.tsx` 增加任务名称。成功提交只表示 job 已创建，页面不得提前移除人物或显示删除成功。

## 影响文件与验证

- 影响：`frontend/src/{api,types}.ts`、桌面/手机玩家页、`PlayersPage.css`、备份和任务日志标签。
- 生产 build 通过；真实后端 E2E 验证运行/停服 capability、202 job 和成功后名册消失。两名其他真人在线允许、目标在线拒绝由后端专项测试覆盖。

## 下一步注意事项

- `canDeleteCharacter` 是权威入口门禁；不要仅凭 `status === 'offline'` 启用按钮。确认时继续提交 `expectedSaveId`，避免切档后误删同 ID。
- 其他玩家在线不能再禁用删除按钮；只能显示风险说明。被删除目标在线仍必须禁用，并接受后端竞态复核结果。

# SAVE-IMPORT-E2E-RELEASE-1 前端接手记录（2026-07-16，真实 E2E 缺失）

- 专项 `npm run test:save-import`、`npx tsc -b`、`npm run build` 均通过；桌面与手机仍共用 hostHandling 校验、结构化错误和 jobs/SSE 恢复逻辑。
- 本轮没有执行真实导入 UI：现有隔离 spike 缺少八类原始 ZIP、逐份 SHA256 和人工游戏客户端条件，不能验证真实玩家选角、takeover 结果、跨日、重启或故障恢复。不要将纯函数测试、mock 请求体或视觉 QA 记录为真实 E2E。
- `SAVE-IMPORT-JUNIMO-1` 仍未完成。后续拿到安全夹具后，两端都必须用同一隔离事务走完整上传，刷新页面观察同一 job，并在游戏客户端确认角色/住宅/家庭语义；unknown/recovery 分支不得显示成功或允许盲目重试。

# FE-SAVE-IMPORT-HOST-1 接手记录（2026-07-16，completed）

## 改了什么

- 新增共享 `SaveImportHostHandling`、校验/禁用/阶段/错误展示模块；桌面和手机上传预览都必须选择 `swap_to_player` 或二次确认的 `virtual_host_takeover`。
- `uploadSaveCommitAndStart` 只接受强类型 `hostHandling`，取消预览拆成独立 helper，旧 `{token,cancel}` 导入提交无法继续从 TypeScript 调用。平台 ID 保持 string，并对 trim 后的纯十进制格式做体验校验。
- 两端提交后消费新导入 job 响应，并从现有 dashboard jobs/SSE 恢复活动任务和阶段；关闭 modal 不取消事务。结构化错误码拥有稳定中文文案，unknown 是中性警告，recovery 明确禁止重试。

## 影响文件与验证

- 主要文件：`types.ts`、`api.ts`、`save-import.ts`、`SavesSection.tsx/.css`、`SavesPage.tsx`、`MobileSavesPage.tsx/.css`、`core/helpers.ts`、`test-save-import.ts`、`qa-layout-main.tsx`。
- 已通过 `npm run test:save-import`、`npx tsc -b`、`npm run build`。桌面弹窗与 390×844 手机弹窗完成真实浏览器 QA；手机文档和 dialog 均为 390px，无横向溢出，console error/warn 为空。

## 下一步注意事项

- 后续不得恢复缺少 `hostHandling` 的提交重载，也不得把 `platformId` 转成 number。新增后端错误码时应先扩充结构化映射，不要解析 Junimo 日志或英文 message。
- job 阶段映射依赖 Panel 自有 job 类型和稳定日志标记；上游成功文本不是成功状态。下方 blocked 记录是后端正式契约落地前的历史，继续保留。

# FE-SAVE-IMPORT-HOST-1 接手记录（2026-07-16，blocked）

- 本次只复核契约并记录 blocked，没有修改前端文件或执行视觉验收。工作区现有前后端未提交改动均保留。
- 当前 API helper 只发 `{token,cancel}`；桌面与手机提交都省略 hostHandling。后端空值默认 `server_owns_original`，与“必须明确同意 takeover”冲突；后端枚举也尚未与任务要求统一。
- SAVE-IMPORT-JUNIMO-1 没有稳定机器终态，因此当前 jobs/SSE 不能提供本任务要求的真实 import 阶段与中性 unknown/recovery 展示。
- 下一步必须先修正后端：缺少 hostHandling 返回稳定 4xx、统一 `swap_to_player/virtual_host_takeover`（或发布明确 DTO 映射）、完成稳定 import job 契约。之后再一次性实现共享校验、桌面和手机请求体/弹窗、错误映射、刷新恢复与双端 QA，不能只补某一端。

# FE-FARM-MOD-PREPARE-1 依赖状态和确认准备（2026-07-15）

## 改了什么

- 模组农场横卡新增依赖完整、待启用数量、缺失 UniqueID 和冲突状态。只有 `readiness=needs_enable` 显示一键准备；missing/conflict 不显示动作。
- 准备确认弹窗从后端 `components` 过滤当前 disabled 项并逐项列出名称/版本/UniqueID，确认后只 POST `farmTypeId`，成功重载目录。弹窗明确不会创建存档或启动服务器。
- 模组卡和准备按钮均不写创建表单，builtin allowlist 提交保护未变。

## 影响文件与验证

- `types.ts`、`api.ts`、`NewGameCreator.tsx/.css`、`farm-catalog-state.ts`、`test-farm-catalog-state.ts`。
- `npm.cmd run test:farm-catalog`；`npm.cmd run build`。

## 下一步注意事项

- 当前 readiness 只代表离线依赖集合；运行时验证和真正创建契约未完成。不要把“依赖完整”改成可选卡或复用官方提交按钮。

# FE-FARM-CATALOG-READONLY-1 新建游戏页只读模组农场（2026-07-15）

## 改了什么

- 新建游戏弹窗加载目录 API，原 8 种官方农场仍来自静态 `builtinFarms` 并保持可选择/可创建；新增模组区只读展示 provider、状态、冲突、说明和图标。
- 模组卡片没有点击/键盘/表单入口，提交前再用 `isBuiltinFarmType` 拦截非官方 ID。API 失败不阻断官方创建；图片失败使用固定占位图；卸载会 abort 并阻止迟到回调 setState。
- 根据实机截图把模组区域从双列窄卡改成单列全宽横卡：56px 图标、说明最多两行，避免卡片过高和中文逐字换行。目录内部解析 warnings 不在页面展示，只保留目录请求整体失败时的非阻断错误提示。

## 影响接口/文件

- `types.ts`、`api.ts`、`NewGameCreator.tsx/.css`、`new-game-farms.ts`、`farm-catalog-state.ts`、`scripts/test-farm-catalog-state.ts` 与 `package.json`。
- 消费 `GET /api/instances/:id/saves/farm-types` 及响应中的受控 `iconUrl`。

## 如何验证

- `cd frontend; npm.cmd run test:farm-catalog`
- `cd frontend; npm.cmd run build`
- 状态测试覆盖官方列表、FrontierFarm、disabled、conflict、无图标/图片 404、API 500、空列表和卸载取消。
- 当前机器 8090/5173/5174 均无面板进程监听；遵守“不启动或停止真实实例”约束，未执行真实登录态页面点击验收。阶段 2 已通过同一扫描器对当前 SVE 做只读文件验证，结果为 `FrontierFarm / 边境农场 / Assets/Tilesheets/Icon.png`。

## 下一步注意事项

- 不要把模组卡改成按钮，也不要把目录 ID 写入现有创建表单。只有后端运行时验证和创建契约完成后才能另行开放。

# FE-JUNIMO-CONFIG-REPAIR-1 修复并升级流程（2026-07-15）

## 改了什么

- 维护卡片对 `repairable=true` 显示“Junimo 配置可自动修复并升级 / 修复并升级”；一次确认内部先调用 repair API，确认返回 `update_available` 后再复用现有 dry-run/apply 状态机。
- 不可修复配置和 `rollback_failed` 仍无自动按钮；修复、下载、安装和验收继续留在同一卡片。

## 影响接口/文件

- `frontend/src/types.ts`、`api.ts`、`DiagnosticsPage.tsx`、`qa-layout-main.tsx`。
- 消费新增 repair 字段和 `POST /junimo-update/repair-config`；无路由或 CSS 契约变化。

## 如何验证

- `npm run test:junimo-update && npm run test:component-update-flow && npm run build`
- `qa-layout.html?junimoConfig=repairable`：桌面和 390px 窄屏均有唯一按钮，窄屏无横向溢出，控制台无错误/警告。

## 下一步注意事项

- repair 成功但复检不是 `update_available` 时必须停止，不能直接启动 dry-run/apply；不要把任意 `invalid_config` 当作可修复。
- 若以后为修复阶段增加独立进度，仍应留在当前维护卡片，不要恢复跳往技术详情的用户流程。

# 2026-07-15 接手补充：FE-MODBUNDLE-1 上传完整性摘要

## 改了什么
- 桌面 Mods 管理 hook 保留 `uploadMods()` 的返回值，成功条显示 ZIP 数、安装数、启用数和当前存档。
- 手机 `MobileModsPage` 对齐同一摘要；两端保留旧后端无 `upload` 时按 `mods.length` 回退。
- 新增共享 `mod-display.ts`。内容包展示名根据 `contentPackFor` 和目录标记补 `[CP]`/`[FTM]`，桌面搜索卡、已安装列表、删除确认和手机列表使用同一结果。
- 桌面 bundle key 优先使用 `ModInfo.packageKey`，旧数据继续回退 Nexus ID；这保证聚合 ZIP 的不同子包不会在删除弹窗中合并。
- 桌面已安装区已取消 Nexus-only 过滤：`userVisibleMods`（排除 SMAPI/Control/Junimo 系统项后）直接全部渲染。无来源项继续走既有 `modToSearchResult()` 的 local 分支，外链按钮禁用但管理能力完整。

## 影响接口/文件
- `types.ts` 新增 `ModUploadSummary`、`ModInfo.packageKey/packageName`，并把摘要挂到 `ModsListResult.upload?`。
- 修改 `useModsManagement.ts`、`pages/ModsPage.tsx`/`ModsPage.css`、`mobile/MobileModsPage.tsx`，新增 `mod-display.ts`；未改上传 multipart 字段或启用切换接口。

## 如何验证
- `cd frontend; npm run build` 通过。后端实包结果为 `1 ZIP / 38 安装 / 38 启用`；SVE 的 CP/FTM 卡片应显示不同前缀，且不再继承 DaisyNiko 的图片和统计。

## 下一步注意事项
- `enabledCount` 表示物理启用/profile 启用成功，不等于已有运行中 SMAPI 加载证据；未来如新增 runtime-loaded 状态，应使用独立字段和标签。
- 展示前缀不能写回 manifest `name`；新增内容包框架时扩展共享 helper，避免桌面/手机再次分叉。
- 不要再用 `nexusModId`、`originNexusModId` 或图片是否存在过滤已安装列表；这些字段只决定来源标签和可用外链。

# 2026-07-14 接手补充：用户更新只看一张卡片

## 改了什么
- `UserUpdateProgress` 移除开发者详情跳转；Junimo/游戏运行文件/SMAPI 的用户进度都在当前维护卡片内呈现。
- Junimo 正常态只有“立即升级”；人工恢复与配置异常直接给出结论，不显示跳转按钮。总览同步删除过期阶段文案。
## 影响接口/文件
- 无接口变化；影响 `DiagnosticsPage.tsx`、`OverviewPage.tsx`。
## 如何验证
- 运行前端全部状态脚本与生产构建；人工确认更新卡片无下跳按钮，点击升级后进度留在原卡片。
## 下一步注意事项
- 开发者技术详情可以继续保留在折叠区，但不得重新成为用户完成更新的必经路径。

# 2026-07-14 接手补充：维护卡片不得伪报无需操作

## 改了什么
- 维护摘要新增检查中状态；`invalid_config`、非标准矩阵、接口错误和 `rollback_failed` 均进入需要关注。
- `rollback_failed` 只提供人工恢复说明和管理员详情入口，不显示普通升级动作。
## 影响接口/文件
- 无接口变化；影响 `DiagnosticsPage.tsx`、`junimo-update-status.ts`、`test-junimo-update-status.ts`。
## 如何验证
- `pnpm run test:junimo-update` 与 `pnpm run build`。
## 下一步注意事项
- 只有明确 `up_to_date` 且没有活动/失败终态时，才能显示“已是推荐版本”；加载中或读取失败不能使用绿色完成态。

# 2026-07-14 接手补充：历史升级终态不得倒写真实版本

- 现象：`/api/version` 已返回 `0.2.4`，页面先显示一瞬间 `0.2.4`，待持久化 `0.2.1 → 0.2.2 succeeded` 加载后又显示 `0.2.2`。
- 修复：`panelUpdateSurface()` 使用 status/versionInfo 作为 observed current；历史 succeeded 仅在 toVersion 等于 observed current（或刚完成同一检测目标）时接管，历史 failed_rolled_back 仅在 fromVersion 等于当前版本时接管。活动任务和 rollback_failed 不受此规则削弱。
- 影响文件：`frontend/src/games/stardew/panel-update-machine.ts`、`frontend/scripts/test-panel-update-machine.ts`；无 API、状态文件或升级 helper 变更。
- 验证：新增“当前 0.1.16 + 历史 succeeded 0.1.15”回归，断言 current/target/topbar/overview 均保持 0.1.16；执行 panel update 状态脚本及生产构建。
- 后续注意：历史状态只能解释同一目标或当前活动任务，任何页面都不得用旧 apply.toVersion 覆盖后端报告的更高实际 currentVersion。

# 2026-07-14 接手补充：连续 Panel 升级状态修复

### 改了什么

- `panelUpdateSurface()` 只让活动升级、未恢复异常或与当前目标相同的终态主导页面；检测到更高正式版本时，历史成功/已恢复记录退回历史展示，新版本重新成为顶栏、总览和详情页主状态。
- `canStartPanelUpdate()` 不再把所有历史 `succeeded` 永久视为门禁；但要求成功 dry-run 的 `targetVersion` 与当前 `latestVersion` 去除 `v` 前缀后精确一致，并继续阻止 active apply、同目标已成功和 `rollback_failed`。

### 影响接口与文件

- 无 API、持久化格式、helper 或镜像选择变更。
- 修改 `frontend/src/games/stardew/panel-update-machine.ts` 和 `frontend/scripts/test-panel-update-machine.ts`。

### 如何验证

- `npm run test:panel-update` 覆盖历史成功目标 `0.1.15` 后发现 `0.1.16`：页面显示 `0.1.16`，旧 dry-run 不放行，新目标 dry-run 放行。
- 同时执行 `npm run test:update-status`、全部 release frontend 状态脚本和 `npm run build`。

### 下一步注意事项

- 历史 apply 终态仍用于结果追溯，不能直接删除；所有决定“当前是否有更新”的 UI 必须优先使用后端 `PanelUpdateStatus`，只在活动任务或同目标终态期间由 apply 接管。
- `rollback_failed` 表示当前部署可能未恢复完整，仍必须优先显示并阻止下一次升级。

# 2026-07-14 接手补充：服务器健康页用户视角重构

### 改了什么

- 页面标题改为“服务器健康”，首屏只保留整体健康结论、可执行的版本维护任务、检查结果、建议与资源占用。
- 原先并列铺开的状态来源、运行环境版本、游戏文件、SMAPI 与 Junimo 技术面板，统一收进默认关闭的“维护与技术详情”。支持包导出、升级预检、确认、执行和日志仍完整保留在这里。
- 新增“版本维护”摘要。Junimo `.121 → .125` 会显示为可选推荐，并明确“不升级仍可继续使用”；管理员从摘要进入详情后再预检，普通用户仍无升级操作。

### 影响接口与文件

- 无接口、请求参数或升级状态机变更。
- 修改 `frontend/src/games/stardew/pages/DiagnosticsPage.tsx`、`frontend/src/games/stardew/pages/DiagnosticsPage.css`、`frontend/src/qa-layout-main.tsx`。

### 如何验证

- QA 页面桌面视觉检查：默认技术详情关闭，整体状态和版本维护位于首屏；展开后 5 个原技术区及支持包导出均存在；无横向溢出、无控制台错误。
- 执行 `npm run test:junimo-update`、`npm run test:runtime-components`、`npm run test:smapi-update` 和 `npm run build`。

### 下一步注意事项

- 后续新增诊断项时，只有会改变用户决策的异常或维护任务才放在默认视图；镜像、digest、buildid、Driver 原始来源和过程日志继续放在技术详情。
- 不要把 `.125` 推荐更新改成强制门禁，也不要因页面折叠而跳过原有 dry-run、确认和 apply 安全流程。

# JUNIMO-STACK-UPDATE-1 阶段二 dry-run 前端接手记录（2026-07-13）

## 改了什么

- 新增 dry-run TypeScript contract 与 GET/POST 调用；管理员诊断页恢复最近状态并在活动阶段轮询，展示整体目标、选中精确镜像/digest、运行态、progress、checks、warnings、失败原因和脱敏日志。
- “运行升级预检”是唯一可用动作；阶段三“执行升级”保持 disabled。普通用户不请求管理员接口，镜像仓库仍不暴露。
- 长引用/检查项/日志支持任意换行，移动端按钮、版本对和检查项改为纵向，避免页面横向溢出。

## 影响文件与验证

- `src/types.ts`、`src/api.ts`、`junimo-update-status.ts`、`DiagnosticsPage.tsx/.css`、`scripts/test-junimo-update-status.ts`。
- `npm.cmd run test:junimo-update`、`npm.cmd run build`（包含 `tsc -b`）通过；package 没有独立 `typecheck` script。

## 下一步注意

- 阶段三之前不得启用执行按钮，也不得把 selected 镜像拆成两个动作或允许前端提交目标。若新增 apply，必须消费后端的新确认契约和停服风险提示，不能从页面状态自行拼目标。

# JUNIMO-STACK-UPDATE-1 阶段一接手记录（2026-07-13）

## 改了什么

- `types.ts`/`api.ts` 新增 Junimo 版本对类型与管理员 GET 调用；`junimo-update-status.ts` 集中五态中文文案和整体匹配判断。
- `OverviewPage` 仅管理员加载详情，仅 `available=true` 显示整体更新提示，唯一按钮“查看详情”导航到诊断页。
- `DiagnosticsPage` 展示当前 server、当前 steam-auth-cn、推荐版本对、是否精确匹配和 unsupported 原因。管理员看到镜像+tag；普通用户只看到 `/state.runtimeDiagnostic` 的 tag/状态，不请求管理员接口、不暴露仓库。

## 影响文件与接口

- `frontend/src/types.ts`、`frontend/src/api.ts`、`frontend/src/games/stardew/junimo-update-status.ts`。
- `OverviewPage.tsx/.css`、`DiagnosticsPage.tsx/.css`、`frontend/scripts/test-junimo-update-status.ts`、`package.json`。
- 消费 `GET /api/instances/:id/junimo-update`（管理员）及 `/state.runtimeDiagnostic` 的脱敏版本字段（登录用户）。

## 如何验证

- `npm.cmd run test:junimo-update` 覆盖五态文案与匹配状态；`npx.cmd tsc -b`、`npm.cmd run build` 做类型和生产构建。
- 桌面/窄屏检查长 ACR/GHCR 镜像引用可换行、提示卡无横向溢出，且页面不存在“升级”“更新 server”“更新 auth”等执行按钮。

## 下一步注意事项

- 阶段二/三未实现前，“查看详情”必须保持只读；不要复用 Panel 自身更新弹窗，也不要在浏览器接收或拼接任意 registry/tag/digest。
- 后续若加入执行流程，仍只能提交后端选定的整体 stackVersion，并需先补 capability/dry-run/备份/回滚协议与独立安全评审。

# PANEL-UPDATE-RELEASE-1 前端接手补充（2026-07-13）

## 改了什么与影响

- 用隔离真 Docker 从 Web 完成成功升级和 unhealthy 自动回滚，确认顶栏、总览、统一弹窗与全屏重连由同一 Provider 同步驱动。
- 成功后自动恢复原页面并打开结果；回滚后桌面与移动端均显示“升级失败，已恢复”，不暴露原始命令。

## 如何验证与下一步

- `npm run test:command-results`、`npm run test:update-status`、`npm run test:panel-update`、`npm run build` 均通过。
- 浏览器验证 1280×800、390×844，无横向溢出且控制台无错误；普通用户权限由组件测试覆盖。
- 正式版本发布后再以真实 registry 镜像复验一次。Provider 必须继续位于路由/桌面移动分流之外，apply POST 网络不确定时只能查询状态，不能自动重提。

# FE-PANEL-UPDATE-1 前端接手补充（2026-07-13）

## 改了什么

- 将更新逻辑从 `useStardewDashboardData` 提升到 App 级 `PanelUpdateProvider/usePanelUpdate`，桌面、移动、顶栏、总览与弹窗共享状态和唯一轮询。
- 新增完整阶段派生、管理员二次确认、普通用户只读、断线全屏退避重连、超时说明、恢复原路由及自动结果弹窗；同时修复 dry-run 请求体被二次 JSON 编码的问题。

## 影响文件

- `PanelUpdateProvider.tsx/.css`、`panel-update-machine.ts`
- `App.tsx`、`useStardewDashboardData.ts`、`UpdateDetailsDialog.tsx/.css`
- 桌面/移动壳、Overview/MobileHome、`api.ts`、QA harness 和 `scripts/test-panel-update-machine.ts`

## 如何验证

- `cd frontend; npm run test:panel-update; npm run test:update-status; npm run build`
- 浏览器 QA：桌面 1280、窄屏 900、移动 390；验证 available、pulling、rolling_back、offline、reconnect-success 和 `role=user`。

## 下一步注意事项

- apply POST 网络不确定时只能查询状态，禁止自动重复提交。`rollback_failed` 不得伪装为已恢复，也不要向用户展示 helper 原始命令。
- Provider 必须继续位于响应式桌面/移动分流之外；否则断点切换会重建轮询并丢失断线状态。

# PANEL-UPDATE-APPLY-1 前端接手补充（2026-07-13）

## 改了什么

- 管理员更新弹窗在 dry-run 成功后提供基础 apply 触发；请求无 body，随后共享轮询持久化 phase/progress/result/error。普通用户无入口。

## 影响文件与验证

- `frontend/src/api.ts`、`useStardewDashboardData.ts`、`UpdateDetailsDialog.tsx/.css` 及 dashboard 类型透传。
- 验证：`npm run test:update-status`、`npm run build`。

## 下一步注意事项

- 本阶段不是完整 UX。后续补断线恢复、二次确认与失败引导，但不能允许前端提交版本/镜像，也不能将 `failed_rolled_back` 显示成升级成功。

# PANEL-UPDATER-DRYRUN-1 前端接手补充（2026-07-13）

## 改了什么

- 管理员更新详情弹窗新增“检查升级环境”，提交最新正式版本并轮询共享 dry-run 状态。
- 展示支持状态、reason/code、Compose 项目、容器/镜像和脱敏日志；不展示宿主机 install/compose/data 路径。
- 普通用户没有按钮，也不请求管理员 dry-run API；没有新增“立即升级”。

## 影响文件和验证

- `frontend/src/api.ts`
- `frontend/src/games/stardew/stardew-routes.ts`
- `frontend/src/games/stardew/useStardewDashboardData.ts`
- `frontend/src/games/stardew/UpdateDetailsDialog.tsx/.css`
- 验证：`cd frontend; npm run test:update-status`、`npm run build`。

## 下一步注意事项

- dry-run 必须继续由 dashboard 共享状态轮询，不能在弹窗或顶栏另起独立请求。
- succeeded 的文案只能是“环境演练通过”，不能显示“升级完成”或提供容器操作按钮。
- capability 完整路径仅管理员 API 可见，但当前 UI 刻意不渲染；后续也不要把这些路径放进普通用户共享版本响应。

# PANEL-UPDATE-CHECK-1 前端接手补充（2026-07-13）

## 改了什么

- dashboard 数据层集中请求更新状态，桌面顶栏、总览、移动端首页和详情弹窗共享同一份数据。
- 顶栏复用版本号区块，总览复用“版本/最新”信息格；有更新时两处统一显示“发现新版本 vX.Y.Z”。
- v0.2.0 发布前补充了双入口一致性回归断言，后续修改版本提示必须同时更新 `panel-update-machine.ts` 与 `test-panel-update-machine.ts`，不得让顶栏和总览再次产生不同文案。
- 新增统一更新详情弹窗；管理员可刷新，普通用户只读，弹窗明确升级执行属于下一阶段。

## 影响文件

- `frontend/src/api.ts`
- `frontend/src/games/stardew/useStardewDashboardData.ts`、`stardew-routes.ts`
- `frontend/src/games/stardew/UpdateDetailsDialog.tsx`、`update-status.ts`
- `frontend/src/games/stardew/StardewPanel.tsx`、`StardewMobileShell.tsx`
- `frontend/src/games/stardew/pages/OverviewPage.tsx`、`mobile/MobileHomePage.tsx` 及对应样式

## 如何验证

- `cd frontend; npm run test:update-status`
- `cd frontend; npm run build`
- QA 页面用 `qa-layout.html?state=running&update=available`，附加 `shell=mobile` 检查移动端。

## 下一步注意事项

- 不要让顶栏和总览各自发请求；新增消费者继续从 `StardewDashboardProps.panelUpdate` 读取。
- `checkStatus=error` 且有成功缓存时仍可提示已知更新，但必须同时展示检查失败；没有成功结果时不能显示“✓ 最新”。
- 历史备注：当时尚无后端执行链路；现在真实按钮已由 `PANEL-UPDATE-APPLY-1 + FE-PANEL-UPDATE-1` 完成，仍不得添加绕过后端状态机的假按钮。

# SAVE-BACKUP-GAMEDAY-1 存档回档功能重构：游戏日回档 + 其他备份两栏 UI

## 背景

后端已把自动备份体系从"现实时间"驱动（最新备份/每日快照/定时备份）改为"游戏内日期"驱动（详见 `docs/backend-handoff/backend-handoff-2026-07-11.md`）：`BackupPolicy` 简化为 `{ gameSaveBackups, retainGameDays }`（取消定时备份），`BackupInfo.kind` 新增 `auto`（游戏日自动回档点）、`predelete`（删除存档前保护备份）、`prerestore`（回档前保护备份），旧的 `latest`/`daily`/`scheduled` 变为只读历史 kind（不再产生新文件，但已有 ZIP 继续可查看）。`BackupInfo` 新增 `gameDayOrdinal` 字段，供前端直接按游戏日排序，不需要自己实现季节序号换算。

用户明确的产品要求：
- "自动备份策略"卡片只保留两个控件："睡觉存档后创建回档点" + "保留最近 N 个游戏日"（默认 5）。
- 备份列表主区域改名"游戏日回档"，只展示自动回档点，每行显示游戏内日期/农场/农场主/现实创建时间/文件大小，主按钮文案"回档到此日"。
- 手动备份、删除存档前备份、回档前保护备份、历史遗留文件放进独立的"其他备份"区域。
- 服务器运行时不能只给一个无说明的禁用按钮，要明确引导用户先停服。

## 改了什么

### `types.ts`
- `BackupPolicy` 改为 `{ gameSaveBackups: boolean; retainGameDays: number }`，删除 `dailySnapshots`/`dailyRetentionDays`/`scheduledBackups`/`scheduledHour`/`scheduledIntervalHours`。
- `BackupInfo.kind` 联合类型追加 `'auto' | 'predelete' | 'prerestore'`（原有 `'manual' | 'latest' | 'daily' | 'scheduled'` 保留，作为历史兼容 kind）。新增 `gameDayOrdinal?: number`。

`api.ts` 完全没有改动——`getSaveBackups`/`createSaveBackup`/`getSaveBackupPolicy`/`updateSaveBackupPolicy`/`restoreSaveBackup`/`deleteSaveBackup` 的 URL、方法、参数都不变，只是它们传输的对象形状随 `types.ts` 变化，属于纯类型层面的改动。

### `SavesSection.tsx`（本次核心改动文件）

- `defaultBackupPolicy`/`normalizeBackupPolicy` 按新形状重写：只 clamp `retainGameDays` 到 `[1, 14]`，`<=0` 或缺失时回落默认 5。
- 新增两个从 `backups` 派生的数组（渲染时计算，不额外存 state）：
  ```ts
  const autoBackups = [...backups].filter(b => b.kind === 'auto').sort((a, b) => (b.gameDayOrdinal ?? 0) - (a.gameDayOrdinal ?? 0))
  const otherBackups = backups.filter(b => b.kind !== 'auto').sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt))
  ```
  `loadBackups()` 里原本会对整个 `backups` 数组按 `createdAt` 排序后再 `setBackups`，现在改为直接 `setBackups(result.backups)`，排序全部下放到这两个派生数组各自的排序规则里（游戏日回档按游戏日序号排、其他备份按现实时间排），避免"先按现实时间整体排序，再各自过滤"这种逻辑上说不通的双重排序。
- "自动备份策略"卡片：删除原来的"定时备份"勾选框 + "每天" + 24 小时 `<select>` 整块 JSX，以及"每日快照保留"滑块；只保留一个勾选框（`gameSaveBackups`，文案"睡觉存档后创建回档点"）和一个滑块（`retainGameDays`，1–14，文案"保留最近 N 个游戏日"）。
- 原"备份列表"卡片拆成两个独立 `<section>`：
  1. **"游戏日回档"**：只渲染 `autoBackups`。表格列改为"游戏内日期 | 农场 | 农场主 | 创建时间 | 大小 | 操作"（不再有"备份文件"文件名列和"状态"徽章列——游戏日回档不存在"同名冲突"这种需要提醒的异常状态，回档本来就是常规操作）。因为后端已经把这个列表限制在 `retainGameDays` 条以内，去掉了原来的"查看更多"折叠（`showAllBackups`）。
  2. **"其他备份"**：渲染 `otherBackups`，沿用原来的六列表格（备份文件/所属农场/创建时间/大小/状态/操作）和"查看更多"折叠。`backupKindLabel` 更新为 `manual→手动备份`、`predelete→删除存档前备份`、`prerestore→回档前保护备份`、`latest`/`daily`/`scheduled`→统一显示"历史备份"。
- **运行中回档的可用性/说明问题**：原代码 `restoreBlocked = busy || isRunning || !isAdmin` 直接绑在行内"恢复"按钮的 `disabled` 上，服务器运行中时按钮整体不可点，用户只能靠 hover 才能看到 `title="服务器运行中，请先停止后再恢复备份"`（触屏设备基本看不到）。这次拆出：
  ```ts
  const restoreBlocked = busy || isRunning || !isAdmin        // 弹窗内"确认/覆盖回档"提交按钮用
  const restoreRowBlocked = busy || !isAdmin                   // 列表行"回档到此日"入口按钮用，不含 isRunning
  ```
  行按钮不再因运行中被禁用，点击总能打开确认弹窗；弹窗里已有的 `isRunning` 警告文案加强为"服务器正在运行中，无法直接回档。请先到"服务器"页停止服务器，再回来完成本次回档"，弹窗内真正提交动作的按钮仍然用含 `isRunning` 的 `restoreBlocked` 禁用。这样服务器运行中点击行按钮的体验从"一个不明所以的死按钮"变成"点开能看到清楚的停服引导"。
- 弹窗与按钮文案统一把"恢复"语汇改成"回档"：对话框标题"恢复备份"→"回档到此日"，正文"确定恢复备份…"→"确定回档到…"，按钮"确认恢复"→"确认回档"、"覆盖恢复"→"覆盖回档"，进行中文案"恢复中…"→"回档中…"。"彻底删除备份"确认弹窗文案未改（对"游戏日回档"和"其他备份"两类条目都通用，删除操作本身语义没变）。

### `StardewPanel.tsx`

`OpsRailActiveCard`（总览页右栏"进行中"卡）此前会额外拉取 `getSaveBackupPolicy()` 只是为了在"定时备份"开启时算出下一次整点、渲染一行倒计时进度条。定时备份功能整体移除后，删除：`backupPolicy` state、对应 `useEffect` 里的拉取逻辑、`countdowns` 数组的计算和渲染（`{countdowns.map(...)}` 那一整块 `.sd-opsrail-hstat--info` 行）、空态判断里的 `countdowns.length === 0`。`restartRows`（计划重启倒计时）和 `activeJobs`（运行中任务进度条）两块完全不受影响。同时清理了因此变成未使用的 import：`BackupPolicy` 类型、`getSaveBackupPolicy`。

### `qa-layout-main.tsx`

Mock 数据必须跟着类型变化更新，否则 `tsc --noEmit` 会报类型错误：
- `backupPolicy` mock 改为 `{ policy: { gameSaveBackups: true, retainGameDays: 5 } }`。
- `backups` mock 从"5 条 `kind: 'daily'`"改为"5 条 `kind: 'auto'`"，并按 `gameYear=1, gameSeason='spring'` + 递减的 `gameDay`/`gameDayOrdinal`（12、11、10、9、8）构造，让 QA 页面里"游戏日回档"表格能看到有意义的排序效果。

### `StardewPanel.css`

- 删除 `.sd-saves-page .sd-save-backup-toggle--schedule`、`.sd-saves-page .sd-save-backup-toggle--schedule .sd-save-backup-toggle-label`、`.sd-saves-page .sd-save-backup-frequency`、`.sd-saves-page .sd-save-backup-toggle--schedule select` 四条规则——JSX 里对应的定时备份勾选框/下拉框已经删除，这几条规则变成孤儿代码，属于本次改动直接产生的清理，不是清理无关历史遗留。**没有**删除更早一代的 `.sd-save-backup-toggle`/`.sd-save-backup-slider` 基础规则（约 1880 行附近），因为新版策略卡片的勾选框和滑块仍然复用这两个基础类名。
- 新增 `.sd-save-gameday-table`：和已有的 `.sd-save-backups-table` 共用 `display:grid; min-width:0; overflow-x:auto`，以及移动端断点下的横滑渐变提示（`::after` 伪元素），保证"游戏日回档"表格在窄屏下和"其他备份"表格有一致的横向滚动手感。列宽本身复用既有的 `.sd-save-backups-thead`/`.sd-save-backup-row` 6 列 `grid-template-columns`（两个表格列数相同，只是语义换了，没有另外定义新的列宽比例）。
- 新增 `.sd-save-backup-list-card--full { grid-column: 1 / -1; }`："其他备份"区域的 `.sd-save-backups-section` 父级网格是"策略卡（窄）+ 列表卡（宽）"两栏布局，但"其他备份"只有一个列表卡子元素，不加这条规则会被两栏网格挤到左侧窄栏，右边空出一大块。

## 影响文件

- `frontend/src/types.ts`
- `frontend/src/games/stardew/SavesSection.tsx`
- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/StardewPanel.css`
- `frontend/src/qa-layout-main.tsx`

未改：`api.ts` 函数签名、新建存档/上传存档/选择存档/删除存档流程、移动端 `MobileSavesPage.tsx`（手机端本来就不展示备份列表/策略，只有一个"回档功能暂不支持手机浏览器"的占位提示，本次未涉及）。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过。
- `cd frontend; npm run build`（`tsc -b && vite build`）：通过，仅保留既有的 Vite chunk 体积提示（与本次改动无关）。

**未做的验证**：没有连接真实运行实例或用 `qa-layout.html` 实际打开浏览器走一遍"游戏日回档"/"其他备份"两栏渲染、策略卡片交互、服务器运行中点击"回档到此日"弹出停服引导confirm 的完整视觉/交互链路，也没有截图确认移动端窄屏下两个新表格的横向滚动表现。建议下一位维护者找时间用 `frontend/qa-layout.html`（打开"存档"路由）或真实实例过一遍。

## 下一步注意事项

- "游戏日回档"表格展示的是全部 `kind==='auto'` 条目，不按当前激活存档过滤（后端按存档名分别维护各自的最近 N 个游戏日配额，详见后端接手文档）。正常使用场景下基本等价于"只有当前在玩的存档有回档点"；如果以后要支持多存档并行维护回档点并分组展示，需要在这里加一层按 `saveName` 分组的 UI，这次没有做。
- 计划重启的"关闭前备份"和服务器控制页"备份已保存进度"快捷操作现在都归为 `manual` kind，混在"其他备份"区块用同一个"手动备份"标签展示，没有进一步按来源区分，这是刻意的最小实现，不是遗漏。
- 如果以后要给"游戏日回档"加"按当前激活存档过滤"或者"分存档 Tab 切换"这类功能，`BackupInfo.saveName` 字段已经足够支撑，不需要后端再新增字段。

# SAVE-BACKUP-GAMEDAY-MOBILE-1 手机端游戏日回档（同日追加）

## 背景

上面 `SAVE-BACKUP-GAMEDAY-1` 只做了桌面端。手机端存档页 `MobileSavesPage.tsx` 里"存档操作"卡片一直有一个恒禁用的"回档"按钮，提示"回档功能依赖桌面端的备份列表操作，暂不支持在手机浏览器使用"。用户明确要求：删掉这个禁用按钮，改成在"存档操作"卡片**上面**新增一个和桌面同名的"游戏日回档"卡片，让手机端直接具备回档能力，不用再引导去桌面端。

## 改了什么

- `frontend/src/games/stardew/mobile/MobileSavesPage.tsx`：
  - 删除"存档操作"卡片里的"回档"禁用按钮和它下面的 `.sd-msave-op-hint` 提示文字段落。
  - 新增独立的"游戏日回档"`<section>`（放在"更多信息"卡片之后、"存档操作"卡片之前），只在 `isAdmin` 为真时渲染——和桌面端"游戏日回档/其他备份"一样是管理员功能，普通用户看不到也点不了。
  - 数据层完全复用桌面同一套 API，没有新增任何后端接口：`getSaveBackups()` 拉列表、`restoreSaveBackup(name, overwrite)` 提交回档。新增本地状态 `backups`/`backupsLoading`/`backupsError`（挂载时按 `isAdmin` 拉取一次，不接入共享 `dashboardData` 30s 轮询，参考桌面 `SavesSection.tsx` 和 `PlayersPage.tsx` 里"页面自己按需拉取"的既有模式）和 `restoreTarget`/`restoreNeedsOverwrite`/`restoreBusy`/`restoreError`（复用桌面同名状态的语义）。
  - `autoBackups = backups.filter(b => b.kind === 'auto').sort(by gameDayOrdinal desc)`，和桌面 `SavesSection.tsx` 的过滤排序逻辑逐字一致。
  - 每一条回档点渲染成一个堆叠行：第一行游戏内日期（`saveDateText(backup)`——这个函数原本只接受 `SaveInfo`，但 `BackupInfo` 恰好结构兼容它用到的 `gameYear`/`gameSeason`/`gameDay` 字段，TypeScript 结构化类型直接允许传入，不需要另写一个适配函数或改动这个共享 helper 的签名）、第二行农场/农场主、第三行创建时间+文件大小，右侧固定一个"回档到此日"按钮。这是手机端和桌面端**唯一**的视觉差异——桌面是 6 列表格，手机窄屏放不下，改成堆叠行，展示的字段集合和排序完全一致。
  - 回档确认弹窗复用页面已有的 `.sd-msave-dialog-overlay`/`.sd-msave-dialog`（和"导入存档"弹窗同一套结构、同一套 `.sd-msave-dialog-btn`/`.sd-msave-dialog-actions` 类名），逻辑照抄桌面 `SavesSection.tsx`：
    - `openRestoreDialog(backup)`：设置 `restoreTarget`，并按 `saveRows.some(s => s.name === backup.saveName)` 预判是否需要覆盖。
    - `handleRestoreConfirmed(overwrite)`：调用 `restoreSaveBackup`；捕获 `ApiError` 且 `code === 'save_exists'` 时切到覆盖态并展示"同名存档已存在…"警告；成功后 `Promise.all([dashboardData.refreshSaves(), loadBackups()])` 同时刷新存档信息和回档列表。
    - 服务器运行中**不会**让"回档到此日"入口按钮变成无说明的死按钮——按钮本身只看 `restoreBusy`（列表行）；点击后弹窗里展示"服务器正在运行中，无法直接回档。请先到"控制"页停止服务器，再回来完成本次回档"的警告，真正提交的按钮（"确认回档"/"覆盖回档"）才用 `!isAdmin || isRunning || restoreBusy` 禁用。这和桌面这次改的 `restoreRowBlocked`/`restoreBlocked` 拆分是同一个设计决策，手机端和桌面端保持一致的"先看到说明，再被挡住"体验，而不是"一上来就看不懂为什么点不动"。
  - 页头"刷新"按钮从只调 `dashboardData.refreshSaves()` 改成 `Promise.all([dashboardData.refreshSaves(), loadBackups()])`，一次刷新同时覆盖存档信息和游戏日回档列表。
- `frontend/src/games/stardew/mobile/MobileSavesPage.css`：
  - 删除 `.sd-msave-op-hint`（JSX 不再引用，属于本次改动直接产生的孤儿清理）。
  - 新增一套 `.sd-msave-gameday-*` 堆叠行样式（`list`/`row`/`main`/`date`/`meta`/`meta-muted`/`btn`），独立维护、不和桌面 `StardewPanel.css` 共享类名（沿用这个文件顶部注释里写明的既有约定："这个文件独立维护一份 sd-msave- 前缀的等价类，不跨文件共享 CSS 类名"）。

## 影响文件

- `frontend/src/games/stardew/mobile/MobileSavesPage.tsx`
- `frontend/src/games/stardew/mobile/MobileSavesPage.css`

未新增后端接口；未改桌面端 `SavesSection.tsx`、`useStardewDashboardData.ts`、`api.ts` 函数签名。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过。
- `cd frontend; npm run build`（`tsc -b && vite build`）：通过，仅保留既有 Vite chunk 体积提示。

**未做的验证**：没有用真实移动设备、也没有用浏览器窄屏模式实际点开"游戏日回档"卡片，走一遍"打开卡片 → 点击回档到此日 → 弹窗展示（含服务器运行中警告/同名覆盖警告）→ 提交成功 → 列表刷新"的完整交互链路；也没有截图确认 320-430px 宽度下堆叠行的文字换行和按钮触控热区表现。建议下一位维护者用 `frontend/qa-layout.html?shell=mobile`（并把 QA mock 里 `backups`/`backupPolicy` 已经在桌面那次改动里更新为游戏日形状，可以直接复用）或真机走一遍。

## 下一步注意事项

- 手机端目前没有实现桌面端"自动备份策略"设置卡片（睡觉存档后创建回档点开关 + 保留天数滑块），只做了只读列表 + 回档操作，这是用户本次明确要求的范围（"在存档操作上面加一个和 PC 一样的游戏日回档卡片"，没有要求策略设置入口）。如果以后要在手机端也开放策略调整，需要参考桌面 `SavesSection.tsx` 的 `backupPolicyDraft`/`handleBackupPolicySave` 实现一套等价逻辑，这次没有做。
- 和桌面一样，"游戏日回档"列表展示的是全部 `kind==='auto'` 条目，不按当前激活存档过滤。

# SAVE-RESTORE-AUTORESTART-1 回档时自动停止/重启服务器（同日追加）

## 背景

用户反馈：服务器运行中点击回档，之前的实现只会禁用提交按钮并提示"请先停止服务器"，用户必须离开当前弹窗，去服务器页手动停止，再回来重新走一遍回档流程。用户要求改成"确认后自动停止服务器、完成回档、再重新启动服务器"。后端已经实现（详见 `docs/backend-handoff/backend-handoff-2026-07-11.md` 的 `SAVE-RESTORE-AUTORESTART-1` 小节）：`POST .../saves/backups/restore` 请求体新增 `autoRestart`，运行中且传 `autoRestart:true` 时返回 `202 {jobId}`（后端把"停止→回档→启动"编排成一个 lifecycle job），已停止时行为完全不变（`200 {saveName}`）。这次做前端接线，桌面和手机端各自改。

## 改了什么

- `types.ts`：`RestoreBackupResult` 从 `{ saveName: string }` 改为 `{ saveName?: string; jobId?: string }`——两个字段是互斥的两种响应形状，不是新增可选补充字段。
- `api.ts`：`restoreSaveBackup(backupName, overwrite, autoRestart, instanceId?)` 新增第三个参数 `autoRestart`，请求体透传给后端。
- `SavesSection.tsx`（桌面）：
  - `handleRestoreConfirmed(overwrite)` 调用 `restoreSaveBackup(name, overwrite, isRunning)`——是否自动重启完全由当前服务器运行状态决定，不需要用户额外勾选。
  - 响应处理分叉：`result.jobId` 存在 → 调用页面已有的 `onJobStarted(jobId)`（和"选择存档并启动"、"新建游戏"、"上传存档并启动"三处用的是同一个回调，接入 `StardewPanel.tsx`/`useStardewDashboardData.ts` 既有的 job 列表 + SSE 轮询机制，不需要新写等待逻辑），并且**不**立即调用 `loadSaves()`/`loadBackups()`——因为这时候回档还没真的发生（job 刚提交），立即刷新只会看到旧数据；`result.saveName` 存在（服务器本来就是停止状态，同步完成）→ 保持原有的立即刷新逻辑不变。
  - 简化了禁用条件：原来行按钮用 `restoreRowBlocked = busy || !isAdmin`、弹窗提交按钮用 `restoreBlocked = busy || isRunning || !isAdmin`，两者不同是因为之前设计是"行按钮总能点开弹窗看说明，弹窗里提交按钮运行中才真正拦住"。现在运行中也允许真正提交（走自动重启），两个变量的条件变得完全一样，所以合并成一个 `restoreBlocked = busy || !isAdmin`，删掉了 `restoreRowBlocked`，所有引用点统一替换。
  - 弹窗文案：运行中警告从"服务器正在运行中，无法直接回档。请先到"服务器"页停止服务器，再回来完成本次回档"改为"服务器正在运行中。确认后将自动停止服务器、完成回档，并重新启动服务器；整个过程可能需要几分钟，请勿在此期间反复点击"。提交按钮文案运行中时追加"（自动重启服务器）"，`busy` 态下运行中显示"正在停止服务器…"而不是笼统的"回档中…"（给用户一个更准确的当前阶段提示，虽然前端其实无法区分"正在停止"和"正在回档"和"正在启动"这三个子阶段，只是比完全不提示要好）。
- `MobileSavesPage.tsx`（手机端）：
  - `handleRestoreConfirmed` 同构：调用 `restoreSaveBackup(name, overwrite, isRunning)`；`result.jobId` 存在时调用 `dashboardData.refreshJobs()`——手机端页面没有 `onJobStarted` 这个 prop（`MobileSavesPageProps` 只挑了 `user`/`instanceState`/`dashboardData`），但 `useStardewDashboardData.ts` 这个共享 hook 本身会对它 `jobs` 状态里任何非终态的 job 自动建立 SSE 订阅，不需要页面手动传回调——调用 `refreshJobs()` 只是为了让新提交的 job 尽快进入这个共享列表，不用等最长 30 秒的兜底轮询；没有 `jobId` 时保持原有的 `Promise.all([refreshSaves(), loadBackups()])`。
  - 两个提交按钮（"确认回档"/"覆盖回档"）的 `disabled` 去掉 `isRunning`（原来是 `!isAdmin || isRunning || restoreBusy`，现在是 `!isAdmin || restoreBusy`）。
  - 弹窗文案和按钮态同步桌面端的调整（自动重启说明文案、按钮后缀、busy 态阶段提示）。

## 影响文件

- `frontend/src/types.ts`
- `frontend/src/api.ts`
- `frontend/src/games/stardew/SavesSection.tsx`
- `frontend/src/games/stardew/mobile/MobileSavesPage.tsx`

未新增 CSS、未改后端接口契约以外的任何 API 函数。

## 如何验证

- `cd frontend; npx tsc --noEmit -p . && npm run build` 通过。

**未做的验证**：没有连接真实运行实例，在服务器运行中实际点一次"确认回档（自动重启服务器）"，观察：弹窗关闭后"进行中"任务卡片是否出现新任务、任务完成后存档/服务器状态是否正确刷新为回档后的新存档、任务失败时（比如回档本身冲突失败、或重启失败）用户能否在现有 UI 里看清楚失败原因。也没有在手机端窄屏下实测这一条流程。建议下一位维护者用测试实例走一遍。

## 下一步注意事项

- 前端目前完全信任后端返回的 `jobId`/`saveName` 二选一契约，没有做"两者都为空"或"两者都存在"这类防御性处理——如果后端契约以后变化，需要同步更新这里的分支判断。
- job 失败时的用户体验完全依赖现有"进行中"任务卡片和 job 日志展示，没有为"回档自动重启"这个操作单独定制失败态的文案或引导（比如"回档失败，当前存档未受影响"这类更具体的提示）。如果以后有用户反馈看不懂失败原因，可以考虑在这里加针对性的错误分类展示，这次按最小实现处理。

# FE-STARTUP-HOST-CONFIRM-1 启动/重启按钮增加主机上线确认 + 邀请码停机后不再残留旧值（同日追加）

## 背景

用户反馈两个问题：

1. 服务器控制页"启动/重启"按钮转圈判断不准，确认后的具体症状是**切换过早**——后端 job 完成、`state` 变为 `running` 后前端立刻切回正常态，但游戏内主机角色可能还没加载完，属于假启动完成。
2. 服务器停止后，邀请码卡片仍显示上一次运行时的旧邀请码，而不是"服务器未运行"；局域网邀请（面板访问地址）本来就不受服务器状态影响，确认这块不用改。

调研时发现 `docs/03-frontend.md` 的 `FE-LIFECYCLE-BACKGROUND-INVITE-1` 记录过相反方向的教训：之前就是按"邀请码/玩家快照出现"判断启动完成，因为快照会因旧 `status.json`/`players.json` 缓存或双次加载而闪烁、邀请码也经常获取失败，导致按钮长期卡在"启动中…"，才改成现在这套纯 job+state 判定。本次修复必须在解决"切换过早"的同时不重蹈"卡死转圈"的覆辙，因此新增了超时兜底：主机上线信号迟迟不出现时也会强制放行。

这次修复本身分三轮才做对，过程记录见下面"改了什么"和"下一步注意事项"——前两轮方向对但各有一个具体错误：一是超时判断挂错了 effect，只在当次点击启动的会话里生效；二是超时阈值（90 秒）拍脑袋定的，实测比真实的存档加载时间短得多；三是（本节新增）在线玩家列表在服务器停止后没有被清空，导致点击"启动"后按钮几乎立刻切回正常态——这三次都是用户在真实运行实例上验证后指出来的，不是凭空发现的。

## 改了什么

### `ServerControlPage.tsx`

- 新增派生值 `hostOnline = (dashboardData.players?.players ?? []).some(p => p.isHost && p.status === 'online')`。玩家列表数据来自 `useStardewDashboardData.ts` 里已有的轮询（`state==='running'` 时每 5 秒一次，组件挂载时也会立即拉取一次），没有新增任何 API 调用或轮询逻辑。判断只看在线玩家列表，**不涉及邀请码**——邀请码是否已经生成和"启动是否算完成"没有关系。
- **踩过一次坑，记录下来避免再犯**：第一版实现是把这个超时判断塞进"清除 `pendingStartupAction` 的 effect"里（该 effect 只在 `!hasActiveLifecycleJob && isRunning` 时触发，负责把本地乐观状态 `pendingStartupAction` 清掉）。问题是 `pendingStartupAction` 只在**当次浏览器会话里用户自己点了启动/重启**时才会被设成非 null；如果用户是刷新页面、或者换一个浏览器/设备打开面板，此时后端 job 早已跑完，`pendingStartupAction` 初始值就是 `null`，`startupInProgress` 直接算出 false，完全不会进入这层新判断——服务器明明还没有主机在线，按钮却已经显示"停止/重启"正常态。这正是用户截图里复现的现象。
- 修正为一个完全独立于 `pendingStartupAction` 的派生值：
  - 新增顶层常量 `HOST_ONLINE_WAIT_TIMEOUT_MS`、组件内 `hostWaitStartedAtRef = useRef<number | null>(null)` 和 `const [hostConfirmTimedOut, setHostConfirmTimedOut] = useState(false)`。
  - 一个独立 effect 监听 `[isRunning, hostOnline, dashboardData.players?.updatedAt]`：只要 `isRunning && !hostOnline`，第一次进入时记下起始时间戳；超过 `HOST_ONLINE_WAIT_TIMEOUT_MS` 后把 `hostConfirmTimedOut` 置 true（超时兜底，避免重现 2026-07-06 那次因为玩家快照闪烁/不可用导致按钮永久转圈的问题）；只要不再满足 `isRunning && !hostOnline`（主机上线了，或服务器状态本身就不是 running 了），立刻把计时器和超时标记都复位。依赖数组里的 `dashboardData.players?.updatedAt` 是关键：即使 `hostOnline` 布尔值本身没变化，玩家列表每次轮询刷新也会让这个 effect 重新求值一次，保证超时判断不会因为"依赖没变化所以 effect 不重跑"而永远不生效。
  - `const awaitingHostConfirmation = isRunning && !hostOnline && !hostConfirmTimedOut`，直接作为 `startupInProgress` 的一个 OR 分支：`isStarting || pendingStartupAction || (job 未完成) || awaitingHostConfirmation`。这样无论本次浏览器会话有没有点过启动，只要"服务器状态是 running 但在线玩家列表里没有主机"，按钮就会保持"启动中…"转圈，直到主机上线或超时。
  - `pendingStartupAction` 的清除 effect 恢复成最初最简单的版本（`!hasActiveLifecycleJob && isRunning` 就清），不再和主机上线判断混在一起——两个关注点已经解耦。
  - 转圈提示文案统一为"服务器正在启动，等待主机玩家上线后再操作。"，去掉了原来"请等待邀请码生成后再操作"的措辞，避免让人以为邀请码是判断条件之一。
- `waitingForStop`（停止/重启中的转圈）判断完全没动，用户没有反馈这一侧有问题。
- **`HOST_ONLINE_WAIT_TIMEOUT_MS` 的取值改过一次，记录一下排查过程**：第一版给的是 `90_000`（90 秒），是拍脑袋估的。用户反馈"在线玩家列表明明没有主机，按钮已经变成停止/重启了"之后，直接 `docker exec stardew-server-1 cat .local-container/control/status.json` 和 `.../players.json`（只读命令，没有对实例做任何停止/重启操作）对比两个文件的 `updatedAt` 时间戳，确认了容器内 SMAPI 侧状态到达 `save-loaded` 和主机真正被写进 `players.json` 在线列表之间，实测差了好几分钟——90 秒会在主机还没真正加载完时就先超时放行，等于这层新加的确认逻辑形同虚设。改成 `10 * 60_000`（10 分钟）留出足够余量，同时保留超时兜底避免真正卡死的情况。

### `useStardewDashboardData.ts`

- 根因定位在 `refreshInstanceState`（约 61-80 行）：只要后端状态响应里的 `s.inviteCode` 非空就无条件 `setInviteCode(recordedInviteCode)`，没检查 `s.state`。后端 `doStop` 按设计不会清空 `DriverPayload.invite_code`（保留历史元数据是有意行为，这次没有改后端），所以停止后这个字段仍然是运行时的旧值。虽然已有一个 effect（337-348 行附近）会在 `instanceState.state` **发生变化**时清空邀请码，但每 30 秒一次的 `refreshInstanceState` 轮询（以及任务结束后的刷新）会在 state 没有变化的后续轮次里把旧值又塞回来，把清空效果覆盖掉，这是服务器停止后邀请码"复活"的真正原因。
- 修复：在 `refreshInstanceState` 内部加一道判断，只有 `s.state === 'running' || s.state === 'starting'`（和文件里已有的 `stateCanExposeInvite` 判断口径一致，394-395 行附近）时才采纳/写入 `recordedInviteCode`；否则直接 `setInviteCode(null)`。这样每一次轮询都会自我纠正，不再依赖"state 变化"这个窄窗口。
- **第三个根因（用户反馈"点击启动按钮，在线玩家还没出现按钮就已经变了"之后才定位到）**：`ServerControlPage.tsx` 的 `hostOnline` 判断读的是 `dashboardData.players`，但这个 hook 里 `instanceState.state` 变化时那个 effect（342-353 行附近）只在离开 `running` 时清了 `inviteCode`，**没有清 `players`**。如果用户在同一个浏览器标签页里"运行过一次（有主机在线）→ 停止 → 再启动"，`players` 状态会一直保留着上一轮"主机在线"的旧快照，因为 `refreshPlayers()` 在请求失败时（比如容器刚重启、还没起来）只会设置 `playersError`，不会清空 `players`。于是新一轮启动后，`ServerControlPage` 里的 `hostOnline` 用的是上一轮残留的旧数据直接判定为 true，`awaitingHostConfirmation` 一开始就是 false，按钮几乎立刻切回正常态，和这次新加的确认逻辑完全对不上。
  - 修复：在这个 effect 离开 `running` 的分支里，`refreshPlayers()` 之前先 `setPlayers(null)`，确保每次服务器从 running 变成非 running 时旧快照会被立刻清空，不会带到下一轮启动里。

## 影响文件

- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/games/stardew/useStardewDashboardData.ts`

未改后端；未改 `InviteCodeCard.tsx`（局域网邀请本来就完全独立于 `instanceState`，不需要改动）。

## 如何验证

- `cd frontend; npx tsc -b` 通过，无类型错误。
- 用户在真实运行实例上实测反馈了两轮问题（超时判断挂错 effect、超时阈值太短），均已修复，详见"改了什么"。诊断过程用的是只读命令 `docker exec stardew-server-1 cat .local-container/control/{status,players}.json` 对比时间戳，没有对实例执行任何停止/启动/重启操作。
- **仍未做的验证**：
  1. 完整走一遍"点击启动 → 主机加载到 `players.json` 里的实际耗时 → 按钮在这段时间内应始终保持转圈 → 主机出现后立刻切回正常态"的连续过程（目前是靠对比两次静态快照的时间戳推断出来的，不是全程盯着看的）。
  2. 人为制造"主机长期不上线"的情况（比如临时把 `HOST_ONLINE_WAIT_TIMEOUT_MS` 调小到几秒测试），确认超时后会强制放行，不会重现 2026-07-06 那次永久转圈的问题。
  3. 停止服务器后，跨过一次自然的 30 秒状态轮询，确认邀请码卡片持续显示"服务器未运行"而不会在某次轮询后又冒出旧邀请码；局域网邀请地址应在整个过程中保持正常展示。

建议下一位维护者（或用户本人）找机会把上面三条走完整。

## 下一步注意事项

- `HOST_ONLINE_WAIT_TIMEOUT_MS` 目前是硬编码常量 `10 * 60_000`（10 分钟，已经根据实测从 90 秒调大过一次）。如果以后发现某些更大的存档/更多模组场景下 10 分钟还是不够，可以继续调大这个常量，不需要改动周围逻辑；也可以考虑改成从后端 job 的实际耗时动态计算，而不是写死常量，这次按最小改动没有做。
- 目前"等待主机上线"这个中间态复用的还是通用 hint（"服务器正在启动，等待主机玩家上线后再操作。"），不区分"job 还没完成"和"job 完成了但主机还没上线"这两个子阶段。如果以后有人反馈想知道具体卡在哪个阶段，可以拆开这两种情况分别提示，这次按最小改动没有做。
- `hostOnline` 的判断只看 `isHost && status === 'online'`，如果以后 `players.json`/SMAPI Control Mod 对"主机"这个角色的上报方式发生变化（比如 headless 主机不再有独立的玩家条目），这里需要同步调整识别条件。
# FE-OVERVIEW-STARTUP-HOST-CONFIRM-1 总览页启动中间态补齐主机在线确认

## 改了什么

- 修复总览页启动按钮在 lifecycle job 结束、实例刚进入 `running` 时过早脱离“启动中…”的问题。
- `OverviewPage.tsx` 现在和 `ServerControlPage.tsx` 使用相同条件：在线玩家列表中出现 `isHost && status === 'online'` 后才展示“停止/重启”。
- 主机确认是从服务端状态和共享玩家数据派生的，不依赖当前页面是否发起过启动，因此刷新总览页后仍然有效。
- 保留 10 分钟超时兜底，避免 `players.json` 快照不可用或持续闪烁时按钮永久卡住；离开 `running` 或主机上线后会清理等待计时。

## 影响文件与接口

- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- 未新增或修改 API；复用 `useStardewDashboardData` 已有的实例状态、任务列表和玩家列表轮询数据。

## 如何验证

- `cd frontend; npm.cmd run build`：通过，仅保留既有 Vite chunk 体积提示。
- 真机联调建议：点击总览页“启动”，确认 lifecycle job 结束且实例进入 `running` 后按钮仍为“启动中…”；待在线玩家列表出现主机后，按钮才切为“停止/重启”。

## 下一步注意事项

- 总览页与服务器控制页目前各自维护相同的主机确认逻辑和 10 分钟常量。后续若调整确认条件或超时，应同步修改两处，或抽成共享 hook，避免行为再次分叉。
# REAL-INSTANCE-LIFECYCLE-BACKUP-VERIFIED-1 生命周期与回档真实实例验证补记

- 用户已确认真实大存档启动等待主机上线、睡觉生成游戏日回档点、运行中回档自动停止/重启均已完成真实实例验证。
- 本标记取代相关小节中的“未做真机/端到端验证”；未明确确认的窄屏视觉验证仍保持原记录。

# FE-LIFECYCLE-STATE-MACHINE-1 总览与服务器控制共用生命周期状态机

## 改了什么

- 新增 `useStardewLifecycleState.ts`，集中消费 `InstanceState`、jobs、players 以及页面本地 pending start/stop，统一推导 `phase`、`startupInProgress`、`waitingForStop`、active lifecycle job、主机在线确认和超时结果。
- `OverviewPage.tsx` 与 `ServerControlPage.tsx` 删除各自重复的 `HOST_ONLINE_WAIT_TIMEOUT_MS`、`useRef`、timeout state/effect 及 job/driver phase 组合判断，改为调用共享 hook。
- 10 分钟主机上线兜底由真正的 `setTimeout` 驱动；即使玩家轮询不再更新，到期也会自动结束等待。离开 `running` 或主机上线时会清理 timer 并重置超时状态。
- 运行中回档的“停止→回档→重新启动”本来就产生 `stardew_lifecycle` job，因此无需增加回档专用分支，会与启动、停止、重启共同进入同一状态机。

## 影响文件与接口

- `frontend/src/games/stardew/useStardewLifecycleState.ts`
- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- 未修改后端和 API 契约。

## 如何验证

- `cd frontend; npm.cmd run build`：通过，仅保留既有 Vite chunk 体积提示。
- 建议后续真机复测四条入口：启动、停止、重启、运行中回档自动重启；在总览与服务器控制页之间切换，确认相同后端状态下按钮阶段一致。

## 下一步注意事项

- hook 当前统一了桌面总览页和服务器控制页。若后续调整主机识别条件或超时阈值，只修改该文件。
- 手机端首页仍有自己的精简生命周期按钮逻辑；若产品要求手机端也展示“等待主机”这一细分阶段，应复用同一 hook，不要复制条件。
# FE-UI-LIFECYCLE-STATUS-1

- 改动：共享 lifecycle hook 优先消费后端 `uiStatus`；诊断页展示四类状态来源和更新时间。
- 影响：`types.ts`、`useStardewLifecycleState.ts`、`DiagnosticsPage.tsx`。
- 验证：`cd frontend; npx tsc --noEmit -p .`。
- 注意：兼容回退只服务旧后端，新增 UI 不应再直接用 players/job 拼生命周期状态。
- 后续补齐：诊断页现已展示新鲜度、存档/缓存身份、Compose、阶段耗时和版本矩阵；Compose 请求只在进入页面与手动刷新发生。
- 现场语义修正：`status.json` 显示为“阶段事件”而不判过期；玩家快照允许 `1111` 与 `1111_<数字>` 视为同一存档身份；跨轮次阶段时间戳不再生成虚假的超长耗时；详细来源卡片通过 grid order 固定在页面底部。

# FE-LAZYLOAD-1 前端拆分阶段一：Tab 页面懒加载 + 构建配置治理

## 背景

前端构建持续提示主 chunk 超过 500 KB（改动前实测 `dist/assets/index-*.js` ≈ 579 KB，`index-*.css` ≈ 319 KB）。根因：项目未使用路由库，`StardewPanel.tsx`（桌面端 9 路由）和 `StardewMobileShell.tsx`（移动端 5 页面）都是手写 `switch(route)`/条件渲染做 Tab 切换，页面组件全部是静态 import，无论停在哪个 Tab 都会把全部页面代码一次性下载执行。同时单页面文件已经很大（`ModsPage.tsx` 2554 行、`InstallPage.tsx` 1464 行、`ServerControlPage.tsx` 1437 行等），`StardewPanel.css` 单文件 16582 行。这三类问题量级都不小，经与用户确认分阶段推进，本次只做风险最低、见效最快的懒加载 + 构建配置部分；hook 拆分（含 `ModsPage.tsx`）和 CSS 按页面拆分列为阶段二、阶段三，已登记进 `docs/07-later-optimizations.md`，不在本次改动范围。

## 改了什么

### `StardewPanel.tsx`

- 桌面端 9 个页面组件（`InstallPage`/`OverviewPage`/`ServerControlPage`/`SavesPage`/`JobsLogsPage`/`PlayersPage`/`ModsPage`/`DiagnosticsPage`/`SettingsPage`）的静态 `import { XxxPage } from './pages/XxxPage'` 全部改为 `lazy(() => import('./pages/XxxPage').then((m) => ({ default: m.XxxPage })))`——页面文件本身的具名导出没有改动，用 `.then` 映射成 `default` 满足 `React.lazy` 的签名要求，不需要为了懒加载去改 9 个页面文件的导出方式。
- `renderPage()` 的调用处（`<div className="sd-main-scroll">{renderPage()}</div>`）外层套 `<Suspense fallback={<PageLoadingFallback />}>`；`PageLoadingFallback` 是本次新增的最小组件，复用已有的 `sd-placeholder-grid`/`sd-placeholder-card` 类名，没有新增 CSS。

### `StardewMobileShell.tsx`

- 移动端 5 个页面组件（`MobileHomePage`/`MobileControlPage`/`MobilePlayersPage`/`MobileModsPage`/`MobileSavesPage`）同样改为 `lazy(...)`，Tab 内容的条件渲染块外层套 `<Suspense fallback={<MobilePageLoadingFallback />}>`；fallback 复用已有的 `sd-mshell-card sd-panel` 类名。

### `vite.config.ts`

- 未改动。依赖精简（`react`/`react-dom`/`qrcode`），没有引入 `manualChunks` 做 vendor 拆分——懒加载后主 JS chunk 已经降到 500 KB 警告线以下，没必要为了进一步压缩引入额外配置复杂度。

## 影响文件

- `frontend/src/games/stardew/StardewPanel.tsx`
- `frontend/src/games/stardew/StardewMobileShell.tsx`

未改：9 个桌面页面组件、5 个移动页面组件的内部实现和导出方式；`vite.config.ts`；`App.tsx`（仍静态 import `StardewPanel`/`StardewMobileShell` 本身，只是这两个 Shell 内部的子页面改成懒加载）。

## 如何验证

- `cd frontend && npm run build`：通过。构建产物从单一 `index-*.js`（579 KB）拆分为 20+ 个页面级 chunk，主 JS chunk 降到约 243 KB（`index-CrwuFr1p.js`），构建警告完全消失。CSS 仍是单文件 `index-H9A-pUpH.css` ≈ 298 KB（`StardewPanel.css` 未拆分，属阶段三范围），部分移动端页面 CSS（`MobileHomePage.css`/`MobileControlPage.css`/`MobilePlayersPage.css`/`MobileModsPage.css`/`MobileSavesPage.css`）和桌面 `SavesPage.css` 已随各自组件的独立 import 自然拆出为独立 chunk。
- 用 Playwright 登录真实 dev 实例（`npm run dev`，后端已在运行）后逐个点击桌面端全部 9 个 Tab 和移动端全部 5 个页面：每次切换 Tab 只触发对应页面的一次按需模块请求（dev 模式下是 `/src/games/stardew/pages/XxxPage.tsx` 这类 ESM 请求），生产构建下对应各自的 hashed chunk；截图确认各页面渲染正常、展示真实数据（设置页用户列表/审计日志、存档页游戏日回档列表等）。控制台无新增错误；捕获到的两条 401（`GET /api/auth/me`）经核实是登录前的既有启动时鉴权探测，与本次改动无关，改动前后行为一致。

## 下一步注意事项

- 阶段二（hook 拆分）和阶段三（CSS 按页面拆分）已登记进 `docs/07-later-optimizations.md`，包含已确认的范围（`ModsPage.tsx` 纳入阶段二；CSS 拆分方式为"按页面拆文件 + 各页面自行 import"），下一位维护者可直接按登记内容执行，不需要重新调研。
- 新增桌面/移动页面时按同样写法接入懒加载（`lazy(() => import(...).then((m) => ({ default: m.XxxPage })))` + 外层 `Suspense`），不要退回静态 import，否则首屏 JS 体积会重新膨胀。
- `useStardewLifecycleState.ts`（见上方 `FE-LIFECYCLE-STATE-MACHINE-1`）已是可复用的领域 hook 雏形，阶段二拆分 `useStardewLifecycleActions` 等 hook 时可参考其结构。

# FE-LIFECYCLE-ACTIONS-1 前端拆分阶段二第一项：生命周期启停操作去重

## 背景

`docs/07-later-optimizations.md` 登记的阶段二第一项：`OverviewPage.tsx` 和 `ServerControlPage.tsx` 里各自维护了一份几乎逐行相同的服务器启停逻辑——`handleStart/handleStop/handleRestart`、`saveStartBlocker`、6 个 state（`actionBusy`/`actionError`/`saveRequiredDetected`/`confirmAction`/`pendingStartupAction`/`pendingStopAction`）、3 个派生 `useEffect`（清空 saveRequiredDetected、job 完成后清 pendingStartupAction、终态清 pendingStopAction）、`showSaveRequiredPrompt`/`canStart`/`canStop`/`canRestart` 的推导公式，以及确认弹窗的“记下 action → 清空 → 调用对应 handler”闭包。这次抽成共享 hook，是本次 phase 2 的第一块，也是风险最低、边界最清晰的一块（`useStardewLifecycleState.ts` 已经把状态推导独立出来，这次只是把“操作”这一层也独立出来）。

## 改了什么

### 新增 `frontend/src/games/stardew/useStardewLifecycleActions.ts`

- 内部调用既有 `useStardewLifecycleState`（不重复实现状态推导），在其基础上管理 6 个 action 相关 state、3 个派生 effect、`handleStart/handleStop/handleRestart`、`saveStartBlocker`，以及新增的 `requestConfirm(action)`/`cancelConfirm()`/`confirmPendingAction()` 三个辅助函数（把两个页面里原本各自手写的“确认弹窗关闭并触发对应 handler”闭包也收进 hook，调用方不再需要自己拼这段逻辑）。
- 额外把 `showSaveRequiredPrompt`（存档提示条件）和 `canStart`/`canStop`/`canRestart`（`ServerControlPage.tsx` 专用的按钮可用性推导，公式两页面完全一致）也收进 hook 的返回值——这两处虽然不是原本 roadmap 里点名的“启停+pending 状态”，但输入完全来自 hook 已有的 `state`/`isRunning`/`isStarting`/`isAdmin`/`actionBusy`/`dashboardData.saves`，是同一份逻辑的自然延伸，顺手一并去重。
- Hook 签名：`useStardewLifecycleActions({ instanceState, dashboardData, isAdmin })`，返回值是 `useStardewLifecycleState` 全部字段 + 上述 action 相关字段的合集。

### `OverviewPage.tsx`

- 删除本地的 `saveStartBlocker`、6 个 state、`useStardewLifecycleState` 调用、3 个 effect、`handleStart/handleStop/handleRestart`、本地 `showSaveRequiredPrompt` 计算，改为一行 `useStardewLifecycleActions({ instanceState, dashboardData, isAdmin })`。
- 确认弹窗按钮从 `onClick={() => setConfirmAction('stop')}` / 手写的确认闭包，改为 `onClick={() => requestConfirm('stop')}` / `onClick={confirmPendingAction}`；取消按钮改为 `onClick={cancelConfirm}`。
- 文件从 555 行降到 456 行。

### `ServerControlPage.tsx`

- 同构改动：删除对应的 state/effect/handler/`saveStartBlocker`/`useStardewLifecycleState` 调用和 `showSaveRequiredPrompt`/`canStart`/`canStop`/`canRestart` 的本地计算，改为同一个 hook 调用；这个页面额外从 hook 拿 `isStopped`（页面里还有一处直接用到）。
- 同步替换确认弹窗的三处 `setConfirmAction(...)` 调用为 `requestConfirm`/`cancelConfirm`/`confirmPendingAction`。
- 未改动的部分：手动备份、计划重启、VNC 配置、服务器密码、小屋与联机高级设置、触发节日活动、永久启用 Joja 路线、控制台命令、全服喊话——这些是阶段二后续待拆的独立领域，这次没有动。

### 一处行为收敛（不是新 bug，也不是回归）

- 原 `ServerControlPage.tsx` 的 `handleStop` 会额外调用 `dashboardData.refreshJobs()`，原 `OverviewPage.tsx` 的 `handleStop` 没有调用。合并成同一个 hook 后两个页面现在都会在停止后 `refreshJobs()`——这是让行为收敛到更完整的一版（和 `handleStart`/`handleRestart` 里已有的 `refreshJobs()` 调用保持一致），不是刻意保留的差异，因此没有做条件分支去维持原有的不一致。

## 影响文件

- `frontend/src/games/stardew/useStardewLifecycleActions.ts`（新增）
- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`

未改：`useStardewLifecycleState.ts`、`useStardewDashboardData.ts`、`api.ts`、后端接口；`ServerControlPage.tsx` 里备份/VNC/密码/运行时设置/节日/Joja/控制台/喊话相关的 state 和 handler。

## 如何验证

- `cd frontend && npx tsc -b`：通过，无类型错误。
- `cd frontend && npm run build`：通过。`OverviewPage` chunk 从 14.55 KB 降到 13.26 KB，`ServerControlPage` chunk 从 33.51 KB 降到 32.21 KB，新增共享 `useStardewLifecycleActions-*.js` chunk（7.61 KB，两页面共用，只下载一次）。
- 用 Playwright 登录真实运行中的 dev 实例（`anxi`账号），在总览页和服务器控制页分别点击“停止”“重启”按钮打开确认弹窗，截图确认弹窗文案、按钮状态正确，然后点击“取消”关闭弹窗——**全程没有点击“确认停止/确认重启”，没有对真实运行的服务器执行任何实际停止/重启操作**，只验证了 UI 接线和渲染正确性。控制台无新增错误（仅有登录前既有的 401 探测，与本次改动无关）。

**未做的验证**：没有实际点击“确认停止”/“确认重启”走一遍真实的服务器停止/启动流程（因为会真的中断当前运行中的测试实例，未经用户明确同意不做这类破坏性操作）。逻辑本身是从原有两份重复代码逐字合并而来，行为路径没有新增分支，真机走一遍完整启停流程建议由下一位维护者或用户本人在合适时机验证。

## 下一步注意事项

- 阶段二后续待拆分：`ServerControlPage.tsx`（备份计划/VNC/密码/运行时设置/节日/Joja/控制台等独立领域 hook）、`SavesSection.tsx`（回档逻辑）、`ModsPage.tsx`（当前最大文件，2554 行）。详见 `docs/07-later-optimizations.md`。
- 新增需要触发服务器启停的页面或组件时，应复用 `useStardewLifecycleActions`，不要再复制一份 `handleStart/handleStop/handleRestart`。
- 移动端 `MobileHomePage.tsx` 仍有自己精简的生命周期按钮逻辑（直接调用 `startInstance`/`stopInstance`/`restartInstance`），没有并入这次的 hook——手机端目前的按钮语义和桌面端不完全一致（比如手机端没有独立的确认弹窗流程），如果以后要让手机端也复用同一套 `useStardewLifecycleActions`，需要单独评估交互差异。

# FE-SERVER-DOMAIN-HOOKS-1 前端拆分阶段二第二项：ServerControlPage 领域 hook 拆分

## 背景

`docs/07-later-optimizations.md` 登记的阶段二第二项：`ServerControlPage.tsx`（1437 行）在上一步 `FE-LIFECYCLE-ACTIONS-1` 去掉生命周期启停重复逻辑后仍然很大，混杂了 8 类互不相干的业务——手动备份、计划重启、VNC 端口/显示渲染、服务器密码、小屋与联机高级设置、触发节日活动、永久启用 Joja 路线、控制台命令、全服喊话。这些业务彼此没有状态耦合（都是各自独立的 state + 各自的 API 调用 + 各自的弹窗开关），是这次拆分里风险最低、边界最清晰的一批：拆分动作纯粹是"把一段状态和函数从页面组件搬进一个新文件"，不涉及合并/去重不同实现（不像 `useStardewLifecycleActions` 那次需要合并两份有细微差异的代码）。

## 改了什么

新增 9 个 hook 文件，全部放在 `frontend/src/games/stardew/` 下：

- `useServerQuickBackup.ts`：`quickBackupBusy/Message/Error` + `handleQuickBackup`。入参 `{ activeSaveName, isAdmin }`。
- `useServerRestartSchedule.ts`：`scheduleOpen/Draft/Loading/Saving/Error/Saved` + `openRestartSchedule/closeRestartSchedule/handleSaveRestartSchedule/toggleScheduleWarning`。入参 `{ isAdmin, refreshJobs }`（`refreshJobs` 是回调而不是整个 `dashboardData`，保持 hook 的依赖最小）。`defaultRestartSchedule` 常量也搬进这个文件。
- `useServerVNCSettings.ts`：`vncPort/PortLoading/DisplayBusy/RenderingEnabled/RenderingStatusLoading/Message/Error` + 原来挂在页面上的 3 个 `useEffect`（运行状态变化时重置渲染开关、运行中轮询渲染状态、管理员登录时读取 VNC 端口）+ `handleToggleVNCDisplay/handleOpenVNCControl`。`vncDisplayFPS` 常量和 `buildVNCControlURL` 函数也搬进这个文件，作为返回值暴露给页面用于文案拼接。入参 `{ isAdmin, isRunning }`。
- `useServerPassword.ts`：密码设置弹窗的完整状态机（草稿/可见性/加载/保存/错误/消息）+ JunimoServer 密码保护状态查询（`passwordStatus*`）+ `openPasswordSettings/closePasswordSettings/togglePasswordVisible/updatePasswordDraft/loadPasswordStatus/handleSaveServerPassword`。入参 `{ isAdmin }`。
- `useServerRuntimeSettings.ts`：小屋策略/联机广播频率弹窗状态机 + `openRuntimeSettings/closeRuntimeSettings/handleSaveRuntimeSettings`。`defaultRuntimeSettings` 常量搬进这个文件。入参 `{ isAdmin }`。
- `useServerFestival.ts`：`festivalBusy/Message/Error` + `handleTriggerFestivalEvent`。入参 `{ isAdmin, isRunning }`。
- `useServerJoja.ts`：`jojaOpen/ConfirmInput/Busy/Message/Error` + `openJojaConfirm/closeJojaConfirm/updateJojaConfirmInput/fillJojaConfirmText/handleEnableJoja`。原模块级常量 `JOJA_CONFIRM_TEXT` 改名为 hook 返回值 `jojaConfirmText`（避免和 hook 内部同名局部变量混淆）。入参 `{ isAdmin, isRunning }`。
- `useServerConsole.ts`：`commands/Loading/Error/selectedCommand/commandBusy/commandResult/commandError` + 服务器运行时自动加载命令列表的 `useEffect` + `selectCommand/handleRunCommand`。入参 `{ isRunning }`。
- `useServerBroadcast.ts`：`sayMessage/Busy/Result/Error` + `handleSay`。无入参（`sendSay` API 本身不需要额外上下文）。

`ServerControlPage.tsx` 改动：

- 删除全部对应的 9 组 state、3 个 VNC 相关 `useEffect`、1 个命令加载 `useEffect`、9 组 handler 函数，以及 `defaultRuntimeSettings`/`defaultRestartSchedule`/`vncDisplayFPS`/`JOJA_CONFIRM_TEXT`/`buildVNCControlURL` 这 5 个模块级常量和函数（全部随对应逻辑搬进各自 hook）。
- 顶部改为连续 9 次 hook 调用，返回值解构出的变量名和原来页面里的本地变量名基本一一对应，JSX 部分因此几乎不用改内容，只改了少数几处内联闭包：
  - `onClick={() => setScheduleOpen(false)}` → `onClick={closeRestartSchedule}`（其余 `close*`/`open*` 同理）。
  - 密码输入框 `onChange={(e) => { setPasswordDraft(...); setPasswordMessage(null) }}` → `onChange={(e) => updatePasswordDraft(e.target.value)}`（把"改值顺带清空消息"这个副作用收进 hook 自己的函数里，页面不用知道这个细节）。
  - Joja 确认输入框和"填入"按钮同理，改用 `updateJojaConfirmInput`/`fillJojaConfirmText`。
  - 控制台命令 `<select>` 的 `onChange` 从三行内联改成 `selectCommand(e.target.value)`。
- 页面文件从 1437 行降到 979 行。

## 影响文件

- 新增：`frontend/src/games/stardew/useServerQuickBackup.ts`、`useServerRestartSchedule.ts`、`useServerVNCSettings.ts`、`useServerPassword.ts`、`useServerRuntimeSettings.ts`、`useServerFestival.ts`、`useServerJoja.ts`、`useServerConsole.ts`、`useServerBroadcast.ts`
- 修改：`frontend/src/games/stardew/pages/ServerControlPage.tsx`

未改：`OverviewPage.tsx`、`useStardewLifecycleActions.ts`、`useStardewLifecycleState.ts`、`api.ts` 任何函数签名、后端接口。JSX 渲染结构、文案、CSS 类名全部保持不变——这次只是把状态和逻辑的"归属文件"改了，不是重新设计交互。

## 如何验证

- `cd frontend && npx tsc -b`：通过，无类型错误。
- `cd frontend && npm run build`：通过。`ServerControlPage` chunk 从 32.21 KB 变为 35.92 KB（略增，因为 9 个 hook 目前只被这一个页面引用，Vite 没有理由把它们拆成独立 chunk，全部内联进 `ServerControlPage` 自己的 chunk 里；这是预期结果，不是回归）。
- 用 Playwright 登录真实运行中的 dev 实例，依次点击"计划重启""服务器密码设置""小屋与联机高级设置""永久启用 Joja 路线"四个按钮打开对应弹窗，截图确认：
  - 计划重启弹窗正确读取到真实配置（关闭/开启时间、时区、提醒分钟勾选、备份开关）。
  - Joja 弹窗输入框输入文本后"确认永久启用"按钮正确保持禁用（因为输入的测试文本和确认文本不匹配）。
  - 四个弹窗全部只做"打开 → 截图 → 点击取消/关闭"，**没有点击任何一个"保存"按钮，没有对运行中的实例提交任何写操作**。
  - 额外确认控制台命令下拉框加载出真实命令列表（查看设置/校验设置/渲染状态/自动托管状态/可见性状态），证明 `useServerConsole` 的自动加载 `useEffect` 正常工作。
  - 控制台无新增错误（仅有登录前既有的 401 探测）。

**未做的验证**：没有实际点击任何一个"保存/执行/发送/触发"按钮走一遍真实的写操作流程（手动备份、保存计划重启、保存密码、保存运行时设置、触发节日活动、执行控制台命令、发送全服喊话、确认启用 Joja 路线）——这些操作大多会真实影响运行中的测试实例或游戏内状态（尤其触发节日活动和启用 Joja 路线，后者还不可逆），未经用户明确同意不做。这次拆分是纯粹的"代码搬家"，每个 handler 内部逻辑和原来逐字一致，没有新增分支，真机走一遍完整操作流程建议由下一位维护者或用户本人在合适时机验证。

## 下一步注意事项

- 阶段二剩余项：`SavesSection.tsx`（1236 行，回档逻辑拆 hook）、`ModsPage.tsx`（当前最大文件，2554 行）。详见 `docs/07-later-optimizations.md`。
- 新增 ServerControlPage 相关功能时，参照这 9 个 hook 的模式新开一个 `useServerXxx.ts`，不要继续往页面组件里堆 state。
- `useServerVNCSettings`/`useServerPassword` 等 hook 目前只被 `ServerControlPage.tsx` 引用；如果以后移动端也要做等价功能（比如手机端 VNC 控制），可以直接复用这些 hook，不需要重新实现。
# FE-PLAYER-LOCATION-NORMALIZE-1 玩家位置实例名归一化

## 改了什么

- 新增 `location-format.ts` 作为唯一位置展示入口。
- 修复存档/SMAPI 可能返回 `FarmHouse<UUID>` 一类 `NameOrUniqueName`，导致 UI 暴露内部 UUID 的问题。
- 桌面玩家表、最近事件、移动玩家页、总览在线玩家均改用共享函数；位置统一显示中文逻辑名称和可用坐标。
- 原始唯一名没有改写：桌面表格悬停标题仍可看到原值，后端与数据库也继续保存原值。

## 影响文件

- `frontend/src/games/stardew/location-format.ts`
- `frontend/src/games/stardew/pages/PlayersPage.tsx`
- `frontend/src/games/stardew/mobile/MobilePlayersPage.tsx`
- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- 未修改 API 和后端数据模型。

## 如何验证

- `cd frontend; npx.cmd tsc --noEmit -p .`
- `cd frontend; npm.cmd run build`
- 建议真实页面确认 `FarmHouseeb266bf0-3eb0-4174-b9b7-f22a893a70bd (10, 9)` 显示为 `农舍 (10, 9)`，悬停仍能看到原始值。

## 下一步注意事项

- 新增位置展示入口必须复用 `location-format.ts`，不要重新排列 `locationDisplayName/locationName/location`。
- 如遇新的带实例后缀建筑类型，只扩展 `INSTANCE_LOCATION_BASES` 和核心标签；不要修改数据库原始字段。

# FE-SHARED-WALLET-PERSONAL-INCOME-1 共享钱包收入语义修正

- `PlayersPage.tsx` 在共享钱包时固定显示“共享模式不统计”，忽略 `individualMoneyEarned` 缺失时产生的误导性 `0g`。
- 分开钱包逻辑不变；底部钱包和收入说明已同步修正。
- 接口和数据库字段保持不变，仅调整展示语义。
# FE-LIFECYCLE-LIVE-SIGNAL-PRIORITY-1 生命周期实时信号优先级修复

## 改了什么

- `useStardewLifecycleState` 不再让滞后的后端 `uiStatus=waiting_for_host/loading_save` 或本地 `pendingStartup` 压住已经由 5 秒玩家轮询确认的在线主机；`isRunning && hostOnline` 直接判定运行就绪。
- `pendingStop`、实例 `stopping` 和 driver stopping phase 的优先级提高到 backend UI projection 之前，点击确认停止后立即显示“停止中…”。
- `OverviewPage` 与 `MobileHomePage` 的按钮渲染顺序统一为停止中优先于启动中；手机总览也新增在线主机判断，避免桌面与手机再次分叉。
- 邀请码请求与轮询没有删除，但不再参与生命周期按钮完成判定。

## 影响文件与验证

- `frontend/src/games/stardew/useStardewLifecycleState.ts`
- `frontend/src/games/stardew/pages/OverviewPage.tsx`
- `frontend/src/games/stardew/mobile/MobileHomePage.tsx`
- 相关文件独立 TypeScript 校验通过。完整 build 当前被工作区已有的 `ServerControlPage.tsx` 未完成 hook 拆分错误阻塞；不要把该批既有错误误归因于本修复。
# FE-MODS-MANAGEMENT-HOOK-1 前端拆分阶段二：ModsPage 本服管理领域 hook

## 改了什么

- 新增 `frontend/src/games/stardew/useModsManagement.ts`，把本服 Mod 列表的本地缓存与刷新、上传弹窗/多文件上传、删除确认、整包导出、同步分类、完整/更新同步包导出、当前存档启用切换集中到一个领域 hook。
- `ModsPage.tsx` 删除对应 18 个 state/ref、两个列表同步 effect 和七组 handler，改为一次 `useModsManagement({ dashboardData, activeSaveName })` 解构接线；页面从 2536 行降到 2360 行。
- 上传和删除弹窗的 open/close 行为也收进 hook，继续保持打开时清错误、忙碌时禁止关闭、关闭上传时清文件 input 的原行为。
- Nexus 搜索、Key 管理、浏览器扩展连通检测、批量安装轮询、sessionStorage 恢复和后端 job 对账没有拆散或改写；它们共享大量 timer/ref/result 状态，当前继续作为一套状态机留在页面，避免纯粹为减行数制造跨 hook 循环依赖。

## 影响文件

- 新增：`frontend/src/games/stardew/useModsManagement.ts`
- 修改：`frontend/src/games/stardew/pages/ModsPage.tsx`
- 文档：`docs/03-frontend.md`、`docs/07-later-optimizations.md`、`docs/08-future-roadmap.md`、本接手文档。
- 未改后端、API 签名、CSS、移动端 `MobileModsPage.tsx`、`SavesSection.tsx`。

## 如何验证

- `cd frontend; npx.cmd tsc -b`：本次两个 Mods 文件不再报错；当前被并行存档拆分新增的 `src/games/stardew/useSaveBackups.ts(26,43)` 未使用 `busy` 参数阻塞。
- `cd frontend; npm.cmd run build`：同样在 TypeScript 前置阶段被上述并行文件阻塞，尚未进入 Vite 打包。
- 待并行存档任务修正后重新执行两条命令；浏览器建议只做只读冒烟：打开三个工作台 Tab、上传/删除弹窗后取消、切换 Nexus 分页。不要在运行中的真实实例执行写操作。

## 下一步注意事项

- 新增本服 Mod 写操作应放进 `useModsManagement.ts`，不要重新把 state/handler 堆回页面。
- Nexus 批量安装是一套带 interval/timeout、扩展消息、sessionStorage 和后端 job 对账的状态机；后续若继续下沉，应整体迁移为单一 `useModsNexusWorkspace`，不要把 timer、job 对账和搜索结果同步拆成互相回调的多个小 hook。
- `SavesSection.tsx` 正由另一任务并行拆分，本次没有触碰其代码。
# FE-CSS-SPLIT-1 前端拆分阶段三：桌面页面 CSS 拆分

## 改了什么

- 原 `frontend/src/games/stardew/StardewPanel.css` 约 16586 行，所有桌面页面规则都会随 Shell 首屏加载。现按页面前缀拆出 9 个文件：`pages/InstallPage.css`、`OverviewPage.css`、`ServerControlPage.css`、`SavesPage.css`、`JobsLogsPage.css`、`PlayersPage.css`、`ModsPage.css`、`DiagnosticsPage.css`、`SettingsPage.css`。
- 9 个页面 TSX 各自 import 同名 CSS，和阶段一的 `React.lazy` 页面 chunk 对齐；用户未进入的页面不会在首屏加载其专属样式。
- `StardewPanel.css` 保留 Shell frame、顶栏/侧栏/OpsRail、基础变量、通用按钮/卡片、跨页面合并选择器和共享组件规则，当前约 4551 行。
- 拆分使用 PostCSS AST 递归处理普通规则与媒体查询；规则声明和选择器内容不改写。二次审计把选择器列表中包含通用类的规则，以及 `StardewPanel`、`InviteCodeCard`、`ServerSummaryCard`、`NewGameCreator` 引用的样式回收到共享 CSS，避免页面 chunk 未加载时共享 UI 缺样式。

## 影响文件

- 修改：`frontend/src/games/stardew/StardewPanel.css` 和 9 个桌面页面 TSX。
- 新增：上述 9 个 `pages/*Page.css`。
- 未改 API、状态、权限、事件 handler、后端接口和移动端独立 CSS。

## 如何验证

- 首轮 `npx.cmd vite build` 成功，构建产物生成 9 个桌面页面 CSS chunk；当时共享 `index.css` 为 95.95 kB，各页面 CSS 约 14.94–31.29 kB。
- 共享规则二次保守回收后，使用 PostCSS 重新解析 `StardewPanel.css` + 9 个页面 CSS，全部解析成功；页面 CSS 总源文件大小约 255 kB，共享 CSS 约 110 kB。
- 最终 `npx tsc -b` / `npm run build` 当前无法完成：并行存档拆分中的 `SavesSection.tsx` 存在 hook 返回值与旧函数重复声明、未完成接线等错误；后续单独运行 Vite 也会在解析该 TSX 时被阻塞。这些错误不在本阶段修改文件范围内。

## 下一步注意事项

- 等 `SavesSection` 并行任务完成后，必须重新执行 `cd frontend; npx.cmd tsc -b; npm.cmd run build`，并确认产物仍有 9 个页面 CSS chunk。
- 浏览器 QA 至少覆盖 1280px 桌面下逐个打开 9 个 Tab，重点检查共享右栏、服务器摘要卡、邀请码卡和通用按钮；390px/320px 检查页面内部横向滚动与弹窗。
- 新增页面专属规则写入对应 `pages/XxxPage.css`；Shell/导航/OpsRail 或被两个以上页面复用的规则继续写入 `StardewPanel.css`。不要重新把页面规则集中回共享文件。

### 全页面级联顺序回归修正

- 初版拆分上线检查发现不只是总览，而是全部桌面页面风格发生变化：旧点状纸纹和旧边框重新覆盖统一卡片皮肤。根因不是选择器缺失，而是懒加载页面 CSS 总在共享 CSS 之后注入；原单文件后半段的 `FE-CARD-UNIFY-SAVES-1` 等跨页面最终覆盖因此被较早定义的页面规则反向压住。
- 修正方式：保留共享规则，同时把共享 CSS 中命中各页面前缀的最终覆盖按共享文件顺序复制到对应页面 CSS 末尾，让页面 chunk 内重新形成“页面规则 → 最终统一覆盖”的顺序。9 个页面全部处理，不只修总览。
- 维护警告：以后调整拆分脚本或手工搬 CSS 时，不得把这些末尾覆盖去重回共享文件；在懒加载 CSS 模型下，这些重复是保证原视觉级联顺序所必需的。

# FE-SAVES-DOMAIN-HOOKS-1 前端拆分阶段二：SavesSection 回档领域 hook 拆分

## 背景

`docs/07-later-optimizations.md` 登记的阶段二剩余项之一：`SavesSection.tsx`（1236 行）混杂了存档列表 CRUD、备份列表/策略、回档确认弹窗、彻底删除备份、新建游戏弹窗、上传存档弹窗六块逻辑。本次只拆"回档"相关的两块——备份（`useSaveBackups`）和回档确认（`useSaveRestore`）——范围对齐文档里登记的"回档逻辑拆 hook"，其余三块（存档 CRUD、新建游戏、上传存档）不属于回档领域，本次不动。

这次改动和 `useModsManagement`/CSS 拆分两个并行任务在同一天进行，期间彼此的 `npx tsc -b`/`npm run build` 会互相报错（因为都在改同一批文件树里未完成的中间状态），两边接手文档都记录了这个阻塞——现在三项都已收尾，`npx tsc -b && npm run build` 可以对全部改动一起跑通过，不需要再单独等待。

## 改了什么

### 新增 `frontend/src/games/stardew/useSaveBackups.ts`

- 备份列表加载 `loadBackups`（未登录管理员时清空列表并直接返回）、备份策略草稿 `backupPolicyDraft` 及 `backupPolicyChanged` 派生值、保存策略 `handleBackupPolicySave`、手动备份 `handleManualBackup`、彻底删除备份 `handleBackupDeleteConfirmed` 及其弹窗开关（`deleteBackupTarget`/`openDeleteBackupDialog`/`cancelDeleteBackupDialog`）、`autoBackups`/`otherBackups` 两个派生排序数组、`showAllBackups` 折叠状态。
- `defaultBackupPolicy` 常量和 `normalizeBackupPolicy`（`retainGameDays` clamp 到 `[1,14]`）函数从页面搬进这个文件。
- 入参 `{ isAdmin, setBusy }`；手动备份和删除备份用的是调用方传入的共享 `setBusy`，不是这个 hook 自己的独立忙碌状态。
- 额外暴露 `clearBackupMessage`，供 `useSaveRestore` 在提交回档前清空遗留的备份操作消息（对应原代码 `handleRestoreConfirmed` 开头的 `setBackupMessage('')`）。

### 新增 `frontend/src/games/stardew/useSaveRestore.ts`

- 回档确认弹窗完整状态机：`openRestoreDialog`（打开时按 `saves.some(...)` 预判是否需要覆盖）、`cancelRestoreDialog`、`handleRestoreConfirmed(overwrite)`。
- `handleRestoreConfirmed` 保留原有的两条分支：`result.jobId` 存在（运行中，自动停止→回档→重启，走 job 轮询刷新）vs `result.saveName` 存在（已停止，同步完成，立即 `loadSaves()`/`loadBackups()`/`onStateRefresh()`/`onSavesChanged?.()`）；捕获 `ApiError` 且 `code === 'save_exists'` 时切到覆盖态。
- `restoreSaveExists`/`restoreBlocked` 两个派生值也搬进这个 hook。
- 入参 `{ saves, isAdmin, isRunning, busy, setBusy, onJobStarted, onStateRefresh, onSavesChanged, loadSaves, loadBackups, clearBackupMessage }`——`saves` 需要从页面的 `data?.saves ?? []` 传入，`loadSaves`/`loadBackups` 是跨 hook 的回调（`loadBackups` 来自 `useSaveBackups` 的返回值）。

### `SavesSection.tsx`

- 删除对应的 state（`backups`/`backupsLoading`/`backupMessage`/`backupPolicy`/`backupPolicyDraft`/`backupPolicyBusy`/`restoreBackup`/`restoreNeedsOverwrite`/`restoreError`/`deleteBackupTarget`/`showAllBackups`）、`loadBackups` 回调、5 个 handler（`handleManualBackup`/`handleBackupPolicySave`/`openRestoreDialog`/`handleRestoreConfirmed`/`handleBackupDeleteConfirmed`）、`defaultBackupPolicy`/`normalizeBackupPolicy`，以及底部对应的派生值（`restoreSaveExists`/`restoreBlocked`/`backupPolicyChanged`/`autoBackups`/`otherBackups`）。
- `busy`/`setBusy` **保留在页面顶层**，作为参数传给两个新 hook——原代码里手动备份、删除备份、回档提交和存档选择/删除/导出共用同一把 `busy` 锁（任意一个操作进行中，所有相关按钮一起禁用），拆进独立 hook 各自的 busy 会改变这个联动禁用行为，所以没有下沉。
- JSX 改动只是把内联的 `setDeleteBackupTarget(backup)`/`setDeleteBackupTarget(null)`/`{ setRestoreBackup(null); setRestoreNeedsOverwrite(false); setRestoreError('') }` 换成 hook 暴露的具名函数 `openDeleteBackupDialog`/`cancelDeleteBackupDialog`/`cancelRestoreDialog`，渲染结构和文案未改。
- 文件从 1236 行降到 1131 行。

## 影响文件

- 新增：`frontend/src/games/stardew/useSaveBackups.ts`、`useSaveRestore.ts`
- 修改：`frontend/src/games/stardew/SavesSection.tsx`

未改：存档 CRUD（`handleSelect`/`handleSelectAndStart`/`handleDeleteConfirmed`/`handleExport`）、新建游戏弹窗、上传存档弹窗、`SaveCard` 组件、`api.ts`、后端接口、CSS。

## 如何验证

- `cd frontend && npx tsc -b`：通过，无类型错误。
- `cd frontend && npm run build`：通过（`SavesPage` chunk 39.24 kB，含 `SavesSection` 拆分后的代码；这次构建同时验证了并行的 ModsPage hook 拆分和 CSS 拆分结果都能正常一起编译，之前两边文档提到的"被并行任务阻塞"已解除）。
- 用 Playwright 登录真实运行中的实例，进入"存档"页：截图确认当前激活存档卡片、游戏日回档表格（真实两条回档点）、其他备份区域正常渲染；点击"回档到此日"，确认弹窗正确读取到目标备份、正确识别"同名存档已存在"并展示"确认回档（自动重启服务器）"和"覆盖回档（自动重启服务器）"两个按钮（因为服务器正在运行）；点击"删除"打开"彻底删除备份"确认弹窗，文案和目标备份名正确。两个弹窗都点击"取消"关闭——**没有提交任何一次真实的回档或备份删除操作**。控制台无新增错误（仅有登录前既有的 401 探测）。

**未做的验证**：没有实际点击"确认回档"/"覆盖回档"/"彻底删除"走一遍真实的写操作（会真的触发停止服务器、覆盖当前存档或永久删除备份文件，未经用户明确同意不做）；没有测试存档 CRUD、新建游戏、上传存档这三块未改动的功能是否受影响（这次没有碰它们的代码，风险低，但也没有重新过一遍）。

## 下一步注意事项

- 阶段二（`OverviewPage`/`ServerControlPage` 启停去重、`ServerControlPage` 9 个领域 hook、`ModsPage` 本服管理 hook、`SavesSection` 回档 hook）和阶段三（CSS 按页面拆分）至此全部完成，见 `docs/07-later-optimizations.md` 的完整记录。
- `SavesSection.tsx` 剩余的存档 CRUD、新建游戏、上传存档三块如果以后觉得文件还是太大，可以参照 `useSaveBackups`/`useSaveRestore` 的模式继续拆，但要注意它们和这两个新 hook 一样共用页面顶层的 `busy` 锁，拆分时不要各自发明一份独立忙碌状态。
- `useModsManagement.ts`（ModsPage 拆分）如果以后要做类似"共享忙碌锁"的重构，可以参考这次 `busy`/`setBusy` 作为显式参数传递的做法。
# FE-PLAYER-COMMAND-RESULTS-1 桌面/手机玩家回执

- 新增共享 `player-command-results.ts`，统一三条玩家命令的 500ms/10s 轮询、七状态分类、中文错误码、旧模组回退与“不自动重试”规则。
- `PlayersPage.tsx` 和 `MobilePlayersPage.tsx` 均已接入；手机端补齐 approve-auth 入口。busy 使用玩家 ID，同一目标防重复，其他玩家不被全局锁住。
- unknown、expired、意外 dispatched 和超时均显示“未收到执行结果”，不使用失败样式；只有 `status=failed` 才显示结构化中文错误。
- 验证：`npm run test:command-results` 覆盖 queued/running/succeeded/failed/unknown/expired/旧模组能力判断；`npm run build` 覆盖桌面与手机编译。
- 真实多人 UI 验证待补：当前本机无运行容器、浏览器无打开的面板实例，磁盘玩家快照只有主机，无法安全点击三条操作。自动测试与构建通过不等同于真实多人验证。
# FE-BROADCAST-BAN-RESULTS-1

- 桌面/手机喊话和封禁都接入共享 command-result 轮询。broadcast succeeded 明确只代表交给聊天系统；ban 区分 succeeded、dispatched、failed、unknown，dispatched 使用指定 Junimo 最终结果确认文案。
- 处理中、unknown、旧模组提交均使用中性提示；只有 succeeded 使用成功样式、failed 使用错误样式，不自动重试。
- 用户人工确认封禁在容器重启后丢失，两端确认弹窗已改为确定性提示。没有新增名单或解封 UI。
- 本机真实控制协议已验证 broadcast succeeded 与 ban host_not_supported；无在线 farmhand，尚不能从 UI 安全验证 ban succeeded。前端状态分类测试已覆盖 succeeded/dispatched/failed/unknown。

# EVENT-JOJA-SAVE-RESULTS-1

## 改了什么

- event/Joja 的桌面 hook 和手机处理函数接入同一 command-result 分类器；dispatched、succeeded、failed、unknown/expired 和旧模组有独立文案，不解析英文 outcome.message。
- 桌面与手机服务器控制页新增“请求游戏内保存”，500ms 轮询最多 125 秒；与“手动备份 ZIP”分开展示。Joja 强确认不变，unknown/dispatched 不自动重试。

## 影响文件/接口

- `player-command-results.ts`、`useServerFestival.ts`、`useServerJoja.ts`、新增 `useServerSaveNow.ts`、桌面/手机控制页、`api.ts`。
- 新接口：`POST /api/instances/:id/saves/save-now`；结果查询复用 `GET /api/instances/:id/commands/:commandId`。

## 如何验证与下一步

- `npm run test:command-results`、`npm run build` 通过，覆盖 event/Joja/save 的 dispatched/succeeded/failed/unknown。
- 协议真机已在现有存档验证 event failed、Joja dispatched、save succeeded；本轮未通过浏览器逐个截图验证桌面/手机视觉，后续 UI 回归需确认两端文案一致。

# COMMAND-RESULT-PRODUCTIZATION-1 接手记录（2026-07-12）

- `JobsLogsPage.tsx/.css` 增加最近控制命令表格；`DiagnosticsPage.tsx` 增加协议版本、队列、入库、消费时间和目录权限。`types.ts`/`api.ts` 增加 SQLite 历史响应类型与查询方法。
- 状态渲染硬边界：succeeded=绿色精确成功；dispatched=黄色已派发；failed/unknown/expired 不得伪装成功；旧模组 `resultSupported=false` 固定显示无法获取精确结果。
- 结构化结果只展示后端已脱敏/白名单字段。移动窄屏使用同一表格数据与语义，通过横向滚动保持列完整；没有改现有桌面/手机命令按钮。
- 验证：`npm run build`。后续若增加筛选/分页，应继续调用 `/control-commands`，不要重新遍历浏览器端 commandId 或读取 status.json。
# FE-SAVE-BACKUPS-NULL-GUARD-1 新服务器存档页黑屏修复（2026-07-13）

- 改动：`frontend/src/games/stardew/useSaveBackups.ts` 在写入备份 state 前使用 `Array.isArray(result.backups)` 校验，旧后端或异常响应的 `null` 降级为 `[]`。
- 影响：仅影响管理员首次进入存档页的空备份边界；存档 CRUD、备份/回档接口和正常非空列表行为不变。
- 验证：`cd frontend; npm run build`。
- 注意：后端已同步保证新响应输出 `backups: []`；前端保护仍需保留，用于兼容旧版本与滚动升级。
# 2026-07-13 接手增量：JUNIMO-STACK-UPDATE-1 阶段三

## 改了什么

- `types.ts`/`api.ts` 新增 apply 状态和 `get/startJunimoUpdateApply`。诊断页只有当前推荐版本 dry-run `succeeded` 才启用整体“更新运行组件”。
- 确认弹窗同时显示 server/auth 当前与目标 tag、停服影响、Steam 授权保护和原停止实例临时启动验证；POST 始终只发送 `{confirm:true}`。
- 页面加载恢复最近 apply，活动阶段轮询；展示完整阶段、进度、检查项、warning、脱敏日志，并分别呈现成功、已回滚、回滚失败。`rollback_failed` 只显示人工指引，无自动重试。

## 影响文件/接口与验证

- 文件：`frontend/src/types.ts`、`api.ts`、`games/stardew/junimo-update-status.ts`、`pages/DiagnosticsPage.tsx/.css`。
- 接口：`POST/GET /api/instances/:id/junimo-update/apply`。生产构建通过 `tsc -b && vite build`；移动 CSS 延续任意断行、全宽按钮和纵向检查项，无页面横向溢出。

## 下一步注意事项

- 不得拆成 server/auth 两个按钮，不得允许编辑 target。若后端为 `rollback_failed`，只提供材料保全/人工处理文案，不能增加“重试回滚”。真实账号发布验收由部署流程执行，前端不得采集或显示 refresh token。
# 2026-07-14 接手增量：GAME-RUNTIME-VERSION-1

## 改了什么与影响文件

- `types.ts/api.ts` 增加 runtime-components 与只读 dry-run 契约；诊断页新增游戏版本/联机运行库当前与推荐 buildid、缺失/损坏状态、路径与预检展示；总览增加 tested 且 mismatch 时的“游戏运行文件可更新”。
- 主要文件：`DiagnosticsPage.tsx/.css`、`OverviewPage.tsx/.css`、`runtime-components-status.ts`、`scripts/test-runtime-components-status.ts`。复用现有任意断行和 620px dry-run 移动布局，没有升级按钮。

## 如何验证与下一步

- `npm.cmd run test:runtime-components`、`npm.cmd run build`。状态测试必须保证 untested/missing/invalid/custom 不显示更新 banner。
- 阶段六如实现执行 UI，必须使用新的单独确认/状态机契约；不得把 buildid 作为普通用户主标题，不得复用 Junimo server/auth apply 按钮直接更新 game-data。

## 2026-07-14 SMAPI 诊断与升级 UI 接手记录

### 改了什么与影响文件

- `DiagnosticsPage.tsx` 增加管理员 SMAPI 卡片、兼容前置入口、dry-run/apply 状态机、回滚提示与玩家同步包提醒；`OverviewPage.tsx` 只对受支持的实际 `update_available` 显示入口；`types.ts`/`api.ts` 增加严格接口类型与固定 POST；新增 `smapi-update-status.ts` 和状态测试。
- dry-run 展示只读 staging 空间估算并明确不创建 volume/不下载/不停服；总览复用 `shouldShowSMAPIUpdate` 状态函数。固定确认体传对象给通用 request，由 request 统一 JSON 编码，禁止预先 `JSON.stringify` 造成双重编码。
- UI 不提供 URL/version/SHA/ZIP 输入；前置失败时禁用按钮，并链接 runtime components / Junimo 卡片。长 SHA、volume 和日志均可断行，移动端按钮满宽。

### 如何验证

- `npm run test:smapi-update`、`npm run test:runtime-components`、`npm run test:junimo-update`、`npm run build`。

### 下一步注意事项

- 阶段八需用真实 apply 状态录屏/人工走查桌面与手机宽度，尤其是 `rolling_back/rollback_failed`、超长错误和玩家重新下载完整同步包提示；不要把候选版本或 GitHub latest 暴露为用户升级目标。

## 2026-07-14 接手补充：统一运行环境版本

诊断页新增统一矩阵总览，按 Junimo/auth、游戏/SDK、SMAPI/Control 分组，展示当前/推荐、通道、状态、最低 Panel 版本、依赖顺序和停服影响；仍链接到三个独立事务，不提供全部 latest。类型已适配后端 `images/digests` 与矩阵状态。withdrawn/non-recommended 使用显式徽标，长值可折行，移动端为单列。

影响文件：`frontend/src/types.ts`、`frontend/src/games/stardew/pages/DiagnosticsPage.tsx`、`DiagnosticsPage.css`、`junimo-update-status.ts`。验证：`npm run build`。后续接手时若增加“按推荐顺序更新”，只能编排现有 dry-run/confirm/apply，必须逐阶段停下让管理员确认，不得增加无条件全部更新按钮。
## 2026-07-14 接手补充：兼容矩阵取消 discovered 状态

- steam-auth-cn 发布不再驱动 Panel 自动 PR，前端兼容矩阵状态类型同步收敛为 `candidate | tested | recommended | withdrawn`。
- 诊断页删除 discovered 徽标文案和样式；用户仍只会收到内嵌 recommended 版本对，候选状态不提供安装入口。
- 影响文件：`frontend/src/types.ts`、`frontend/src/games/stardew/pages/DiagnosticsPage.tsx`、`DiagnosticsPage.css`。验证：运行全部前端状态脚本与 `npm run build`。
## 2026-07-14 接手补充：Panel 直接指定组件版本

- candidate/tested 不再是发布或前端状态；兼容状态收敛为 `recommended | withdrawn`。
- 用户升级 Panel 后，诊断页直接将实例当前版本与该 Panel 内嵌组件清单比较并提示升级，不涉及任何 GitHub 审批状态。

# 2026-07-14 接手增量：0.2.2 的 Junimo .125 推荐提示

- 无前端代码和接口变更；矩阵数据更新后，现有总览/诊断状态机会把 `.121` 显示为可选的推荐升级到 `.125`。
- 文案语义必须保持“推荐升级”：不得阻止 `.121` 用户使用现有功能，不得自动触发预检或升级。新安装默认 `.125` 由后端负责。
- 验证沿用 `test:junimo-update`、`test:runtime-components`、`test:smapi-update` 和生产构建；发版后检查桌面/移动提示没有出现“必须升级”或整页禁用。
# 2026-07-14 接手补充：组件更新卡片内一键流程

### 改了什么

- `DiagnosticsPage` 的版本维护卡直接承载 Junimo/SMAPI 校验、下载、安装和验收进度。用户确认一次后始终重新 dry-run，成功即自动 apply。
- Junimo 下载显示组件、镜像层和百分比；失败状态持续显示在维护区。回滚失败同时展示初始失败与恢复失败，不再出现“下方日志报错、上方却无需处理”的割裂。
- 游戏/SDK 只展示现有预检进度；技术详情保留 checks、镜像、digest 和日志，但移除重复动作按钮。

### 影响接口/文件

- `types.ts` 消费 Junimo 新增的可选 download/cause/rollback 字段；`DiagnosticsPage.tsx`、`DiagnosticsPage.css` 和 `qa-layout-main.tsx` 负责交互、样式与 pulling/rollback-failed fixture。

### 如何验证

- 状态脚本覆盖 Junimo、SMAPI 和 runtime components；生产构建通过。
- QA 页面分别检查镜像拉取和回滚失败，桌面/390px 窄屏无横向溢出，失败卡不消失。

### 下一步注意事项

- `rollback_failed` 按钮必须保持禁用；没有后端显式恢复 API 前，不得在前端增加“重试/清除”伪操作。
# 2026-07-14 接手补充：组件升级任务代际与本地竞态 QA

## 改了什么

- `DiagnosticsPage` 用本次 dry-run ID 代替布尔请求标记，Junimo/SMAPI 仅在同 ID 成功后启动 apply，并用 ref 防止重复提交。
- 新增工作流时间比较，较新的预检/安装进度不再被历史失败结果覆盖。
- QA 增加 `junimoWorkflow=race-retry` 动态场景，记录 dry-run/apply 请求顺序并模拟旧状态抢跑会收到的 409。

## 影响接口/文件

- HTTP API 不变；严格消费已有 `dryRunId`/`updateId`、`startedAt` 字段。
- 影响 `DiagnosticsPage.tsx`、`component-update-flow.ts`、对应测试脚本、`qa-layout-main.tsx`、package scripts 和两个 GitHub workflow。

## 如何验证

- `npm run test:component-update-flow`
- `npm run test:junimo-update && npm run test:smapi-update && npm run build`
- 本地 QA 一次点击后事件包含且只包含一个新 `dry-run:POST` 和一个 `apply:POST`，顺序中间必须有成功轮询，最终卡片为“升级成功”且无旧失败文案。

## 下一步注意事项

- 后续任何“两段式”组件更新都必须绑定服务端任务 ID，禁止再用裸布尔值连接历史状态与新操作。
- QA-only 的自动确认与事件输出只存在于 `qa-layout-main.tsx`，不得进入生产入口。

## 2026-07-15：阶段 7 模组农场创建 UI

- feature flag 开启且服务端 selectable 时可选择显式 custom ID；disabled/missing/conflict 不可提交。高级输入仍由后端做 provider、依赖和 runtime 验证。
- 保存列表桌面/移动端显示 `farmTypeLabel (farmType)`，custom 类型使用固定占位图。补齐全部事务错误文案。
- 模组卡向左展开；浏览器实测 1280px 下约 400px，390px 移动端“边境农场 (FrontierFarm)”无横向溢出。
- `test:farm-catalog`、production build 通过。后续隔离真实 SVE E2E 已确认创建后列表显示 `边境农场 (FrontierFarm)`，容器重启及 `FrontierFarm → Standard → FrontierFarm` 双向切档后仍正确；默认开关继续关闭。

## 2026-07-15：阶段 8 发布前兼容收尾

- `catalogResponseState` 将缺失的 `moddedCreationEnabled` 严格归一为 false，保证旧后端 + 新前端不会意外开放；状态脚本新增混合版本回归。
- release 与 compatibility workflow 均加入 `test:farm-catalog`。真实导入后 API 继续返回 `边境农场 (FrontierFarm)`，依赖 profile 正常。
- 本轮 localhost 被 in-app Browser 客户端策略阻止，未伪造 900px/console 结果；既有 1280/390 无溢出证据有效，发版候选需补 900px 人工走查。
# 2026-07-16 接手补充：整包导入摘要与 SVE 旧存档警告

## 改了什么

- 桌面和移动 Mod 页新增内置重复件跳过摘要，并展示服务端返回的持久兼容性警告。
- 无来源、本地 Mod 仍正常展示；`[CP]` 名称与既有分组/同步标识不变。

## 影响与验证

- 消费 `ModsListResult.compatibilityWarnings` 和 `ModUploadSummary.skippedBuiltInCount/skippedBuiltInNames`。
- 影响 `src/types.ts`、`ModsPage.tsx/.css`、`MobileModsPage.tsx`；使用 TypeScript production build 验证。

## 下一步注意事项

- 警告必须以活动存档为准，切换或新建存档后重新拉取列表即可消失；前端不要自行用 Mod 名称猜测兼容性。

# 2026-07-16 接手补充：模组地图入口默认开放

- 新后端默认返回 `moddedCreationEnabled=true`，通过 `selectable` 门禁的模组地图会显示可选入口；前端代码无需硬编码开启。
- 旧后端缺字段、请求失败或部署方显式 false 时仍严格关闭。继续以服务端 `selectable` 为最终依据，不得仅凭全局开关放行 disabled/missing/conflict 项。
# 2026-07-16 接手补充：当前存档 Mod 一键启停

## 改了什么

- 桌面 `ModsPage` 设置页与移动 `MobileModsPage` 已增加“一键启用全部 / 一键禁用全部”，统一调用 `updateAllModsEnabled`；批量进行时会锁定单项开关，结束后刷新页面列表和 dashboard Mod 数据。
- 按钮遵守管理员、停服、活动存档和可切换状态；所有 built-in runtime/Control/Junimo 组件均不参与批量状态判断。

## 影响接口/文件与验证

- 接口：`PUT /api/instances/:id/mods/enabled`，请求 `{enabled, saveName?}`，响应包含 `changedCount`。
- 文件：`frontend/src/api.ts`、`games/stardew/useModsManagement.ts`、`pages/ModsPage.tsx/.css`、`mobile/MobileModsPage.tsx/.css`。
- 验证：`npm.cmd run build` 已通过；后续视觉走查需同时覆盖桌面设置 tab 和移动已安装 tab 的运行中/无存档/批量 busy 状态。
# 2026-07-16 接手补充：强制 125 展示语义

- 推荐对象新增 `runtimeUpdatePolicy`。当前为 `required`，总览/维护卡片明确显示 Panel 自动升级到 125，不再沿用历史“不升级仍可继续使用”。
- 正常路径不需要第二次 Junimo confirm；新 Panel 后端自动启动后，前端继续轮询既有 dry-run/apply。原按钮只作为失败后的管理员手动重试，不应删除恢复入口或自行拼接升级请求。
- 影响 `types.ts`、`OverviewPage.tsx`、`DiagnosticsPage.tsx`、QA fixture；验证需包含类型检查、状态脚本、production build 和桌面/移动维护卡片走查。

## FE-GAME-LANGUAGE-1：服务器游戏语言（2026-07-16）

- 桌面和移动控制页新增语言快捷卡与弹窗，选项来自 `game-languages.ts` 的官方 12 语言清单，默认中文。
- `useGameLanguage` 统一 GET/PUT、保存反馈与运行中“保存并重启”；它只控制服务器/Mod 文本，不影响面板界面语言。
- 影响桌面/移动控制页、`useGameLanguage.ts`、`api.ts`、`types.ts`；`npm.cmd run build` 通过。
- restart API 若未来返回 job ID，应把按钮反馈接到统一生命周期进度；当前沿用既有 restart 请求和 dashboard 轮询。
# SAVE-IMPORT-E2E-RELEASE-1 addendum (2026-07-17)

- Backend real technical E2E now includes completed takeover/as-is and swap jobs with second restarts. The frontend contract remains explicit host handling, string platform ID, structured unknown/recovery messaging and jobs/SSE recovery.
- Full frontend release acceptance is still open: a human game client and eight rich fixtures are unavailable, and desktop/mobile have not both traversed the same live semantic-validation job. Do not change the umbrella status to completed.
- `npm run test:save-import`, `npx tsc -b`, and `npm run build` passed on 2026-07-17. The live completed swap job was restored on the mobile saves page with no horizontal overflow at the active 429px test viewport; the mandatory same-job desktop plus 390x844 release pass remains outstanding.

## 2026-07-17 local rich-save follow-up

- A new real upload/takeover job completed through the existing API/job contract, and frontend save-import tests, TypeScript build and production build passed again.
- The isolated noVNC HTML loaded but could not establish its WebSocket under the FPS-zero test configuration. Treat this as missing visual/game-client evidence; it does not change frontend completion or the umbrella release gate.
# PANEL-POLL-LEAK-1 接手记录（2026-07-18，completed）

- 玩家/邀请码 dashboard timeout 与右侧栏指标 timeout 均监听 `visibilitychange`，隐藏时立即清理、可见时恢复；卸载 cleanup 保持。诊断页原有指标可见性门禁不变。
- 邀请码接口的 `n/a` 被归一为未就绪，不显示为邀请码；启动/重启触发的轮询继续等待真实值。影响 `useStardewDashboardData.ts` 与 `StardewPanel.tsx`。
- `npm.cmd run build` 通过。后续新增任何玩家、邀请码或指标轮询必须沿用可见性门禁；不要只依赖浏览器对后台 timer 的节流。
# 2026-07-20 handoff：全栈升级状态

- 影响文件：`src/api.ts`、`games/stardew/{PanelUpdateProvider,UpdateDetailsDialog,panel-update-machine}.tsx/ts`。
- `PanelUpdateCapability` 新增 `composeService`、`conversionRequired`；`PanelUpdateApplyStatus.fullStack.instances` 是逐实例状态来源。
- apply 始终发送 `confirmFullStack: true`。Panel 自身 `phase=succeeded` 但 full-stack 仍活动时，遮罩和轮询必须保持；只有全栈 `succeeded/not_needed/failed_safe/manual_action` 才可结束。
- 已通过 TypeScript/Vite production build。后续新增状态必须同步 phase label、active/terminal 集合和时间线。
# DOCS-PORTAL-0.4.1：首页发布信息与视觉层级（2026-07-20）

- 展示文档的发布信息已更新到 `v0.4.1`；首页顶部“快速上手”导航固定为 30px 紧凑胶囊，避免链接自身的导航栏高度把背景撑成宽大色块。
- 首页第五张、即桌面第二行左侧“版本更新日志”卡使用完整高亮边框、渐变、阴影、悬停位移和“最新版本”角标；深色主题同步适配。样式通过稳定的 `/changelog` 链接定位，后续调整 feature 顺序不会漂移到其它卡片。
- 影响文件：`website/docs/.vitepress/theme/custom.css`、`website/docs/index.md`、`website/docs/changelog.md`。验证方式：`cd website && npm run docs:build`，并在桌面/窄屏分别检查导航、卡片换行、横向溢出和浏览器 console。
# DOCS-PORTAL-0.4.2：SQLite 修复发布信息（2026-07-24）

- 首页 CURRENT RELEASE、版本更新卡和 changelog 已切到 `v0.4.2`，面向用户解释扫描路径 404、初始化缓存、SQLite 驱动升级和取消恢复保护。
- 影响文件：`website/docs/index.md`、`website/docs/changelog.md`。`npm run docs:build` 通过；Panel 前端无功能改动，九项状态脚本与 production build 通过。
- Pages 发布工作流已成功，线上首页和 changelog 均确认来自包含 `v0.4.2` 的 main 提交；GitHub Release 与三仓精确镜像也已成功发布。
# DOCS-PORTAL-0.4.2-VISUAL 接手记录（2026-07-24）

- 用户截图确认更新卡正文已是 `v0.4.2`，但 CSS 伪元素仍硬编码 `v0.4.1`；现改由首页 frontmatter `release` 经 `ThemeLayout.vue` 的继承 CSS 变量提供角标内容。
- 全站 `.anxi-layout .vp-doc p/strong` 位于首页规则之后且覆盖流程区局部颜色，造成深色底上的深色副标题/步骤标题。流程区现使用独立高对比变量和更高 specificity。
- 影响文件：`website/docs/index.md`、`.vitepress/theme/{ThemeLayout.vue,custom.css}`。production build、浅/深色桌面、390px、零溢出与 console 均通过，Pages 线上角标和流程区计算颜色已确认。后续升级版本时修改 frontmatter、feature 说明、CURRENT RELEASE 和 changelog；不要把版本重新写进 CSS。
# DOCS-PORTAL-0.4.3 接手记录（2026-07-26，已发布，QA passed）

- 首页 frontmatter、版本更新卡和 CURRENT RELEASE 已切换到 `v0.4.3`，changelog 新增 Panel 一分钟健康监控、连续三次 SQLite code 9 精确退出及 Docker unhealthy/restart policy 边界说明。
- `v0.4.3` tag 已包含 `v0.4.2` 发布后提交的动态 release 角标与深色流程区高对比度修复。不要把版本号重新硬编码到 `custom.css`；继续由 `ThemeLayout.vue` 把 frontmatter `release` 注入 `--home-release-label`。
- 影响文件：`website/docs/{index,changelog}.md`。验证需覆盖 production build、浅/深色桌面、390px 移动端、console、首页更新卡点击到 changelog；发布后还需确认 Pages 线上内容。
- 本地与 Pages 线上 Browser QA 均完成：1440×900 浅/深、390×844、版本角标/流程区计算颜色、更新卡点击、changelog 首项、横向溢出和 console 全部通过；线上首页显示 `v0.4.3`，无 error/warning。

# DOCS-PORTAL-DRAFT-REVERT-1 接手记录（2026-07-29，未发布草稿已撤回）

## 改了什么

- 用户要求取消本轮 GitHub Pages 重构并回到原本上线版本。四个已跟踪的 `website/` 文件已从 `HEAD` 恢复，两个未跟踪草稿文件 `DocsHome.vue`、`calm-docs.css` 已删除；没有提交、推送或发布。
- 先前素材清理与本次门户回退分开处理：旧农舍 PNG、两张零引用手机顶栏图、两张纯历史原型基线图及三个失效 CSS URL 保持删除；原型脚本继续使用现行总览横幅素材。
- 动态拼接的猫狗品种图仍由 `NewGameCreator.tsx` 使用，继续保留，不得因为没有完整文件名字面量而误删。

## 影响与验证

- 门户回退影响 `website/docs/.vitepress/{config.ts,theme/ThemeLayout.vue,theme/index.ts}` 与 `website/docs/index.md`；恢复后的文件内容哈希与 `HEAD` 一致，网站源码相对 Git 无差异。
- VitePress production build 通过；线上页面标题、导航、六项功能入口、流程区和 `v0.4.5` 发布摘要与恢复源码一致。无 API、Panel 状态或正式发布变化。

## 下一步注意事项

- 仓库内不再存在该未发布首页草稿；后续任何新方向都从当前线上基线另建隔离预览，获得用户确认后才允许修改正式 `website/` 源码。
- 不要恢复或再次导入已删除的农舍素材，也不要把 `.codex`、generated_images 或 visualizations 中的代理输出当作产品素材。

# FE-MOD-UPLOAD-GUIDANCE-1 接手记录（2026-07-31）

## 改了什么

- 桌面 Mod 页顶部上传按钮外增加能力气泡，鼠标悬停或键盘聚焦时展示；按钮因服务器运行而禁用时，外层悬停仍可阅读能力说明，原有停服原因继续由按钮 `title` 保留。
- 桌面与移动上传弹窗使用共享 `ModUploadGuidance` 说明牌，明确可一次选择一个或多个 ZIP，也可在一个 ZIP 中放多个 Mod 文件夹；同时说明 ZIP 套 ZIP 不会递归解压，并直接告诉用户先解压内层 ZIP、再单独上传。

## 影响文件与验证

- 影响 `frontend/src/games/stardew/ModUploadGuidance.{tsx,css}`、`pages/ModsPage.{tsx,css}`、`mobile/MobileModsPage.{tsx,css}`；未改变 `/api/instances/:id/mods/upload` 请求、响应或上传限制。
- 已通过九项前端状态脚本与 `frontend` 的 `npm.cmd run build`。应用内 Browser 已验证桌面入口说明关联、弹窗常驻说明、打开弹窗时入口气泡不残留；390×844 移动弹窗宽度未超过视口，桌面/移动 console error/warn 均为空。Browser 指针模拟未触发 `:hover`，悬停行为由同一 CSS 规则和实际浏览器继续保障。

## 下一步注意事项

- 后端未来若新增递归解压 ZIP 中 ZIP 的能力，必须同步更新共享组件中的两行文案和入口气泡；在安全边界未改变前，不要把“递归目录扫描”表述为“递归解压压缩包”。

# FE-STEAM-AUTH-WAIT-VISIBILITY-1 接手记录（2026-08-09，released in v0.4.10）

## 改动与影响

- `junimo-update-status.ts` 新增纯函数 `junimoApplyWaitingNotice`，认证阶段标题改为“正在尝试 Steam 连接”；`DiagnosticsPage.tsx/.css` 在用户进度卡和技术详情中展示累计等待、自动重试、验收边界和“不是卡死”。接口保持使用现有 `phase/updatedAt`。唯一 `role=status` 只包住阶段标题，动态计时和重复技术详情不进入 live region，避免 1.8 秒轮询导致读屏重复播报。

## 验证与下一步

- `test-junimo-update-status.ts` 覆盖分钟/秒格式、非认证阶段和 live-region 单一归属；全部 12 个 `test:*` 脚本与 production build 已重跑。除本地 Panel/Vite fixture 外，正式 Web 升级得到的新 Panel 已在 769×240 与 280×653 验证提示、计时增长、唯一 live headline、动态详情隔离、零横向溢出和 console health。
- 后端若以后拆分新的 Steam 等待 phase，应同步扩展纯函数和状态测试；不要只更换标题而移除 elapsed/自动刷新说明，也不要把“正在尝试连接”表述成升级必须等到 Steam 登录成功。
# FE-INSTALL-DIAGNOSTIC-MAPPING-1 接手记录（2026-08-13，completed，未发布）

## 改了什么

- 新增 `frontend/src/games/stardew/installation-state.ts`，把实例状态和可选 `installationDiagnostic` 收敛为 `installed/installing/not_installed/install_failed/repair_required/runtime_error/unknown`。首次安装提示只由明确 `not_installed` 或尚未开始的 scaffold 状态触发；`error` 不再等价于“未安装”。`junimo_scaffolded` 即使已生成 Compose/发现镜像，也不能假定已有 Steam/VNC 凭据而直接走 reuseCredentials 修复。
- `StardewPanel.tsx` 删除本地 installed-state 集合；`InstallPage.tsx` 删除重复集合和 `state === 'error'` 的直接重试门禁，按文件、Compose、镜像、Control 与诊断可用性分别显示安装、修复、启动重试或诊断。Control mismatch、Docker unavailable 和未知/矛盾证据不能打开安装表单。
- `MobileHomePage.tsx` 同步消费分类器；移动动作会切换到完整桌面版的 `install` 或 `diagnostics` 精确路由，不再只给无目标的异常提示。`frontend/src/types.ts` 增加与后端 JSON 一致的可选 `InstallationDiagnostic`。

## 影响文件与验证

- 影响：`frontend/src/types.ts`、`games/stardew/{installation-state.ts,StardewPanel.tsx,StardewMobileShell.tsx}`、`games/stardew/pages/{InstallPage.tsx,InstallPage.css}`、`games/stardew/mobile/MobileHomePage.tsx`、`frontend/scripts/test-install-state.ts`。
- 已通过：`npm run test:install-state`、`npm run test:responsive-layout`、`npm run build`。
- 下一步：候选镜像必须在明确未安装、确认缺文件、Control mismatch、Docker unavailable、普通启动失败五种 API fixture 下分别验收桌面首次弹窗、安装页按钮和移动端目标路由；不要把 source/build 通过写成 Browser 或真机已通过。

# FE-NEWGAME-IDEMPOTENCY-1 接手记录（2026-08-13，completed，未发布）

- `api.createNewGame` 强制接收 request ID 并写入 `Idempotency-Key`。`SavesSection` 用配置指纹管理一个 pending ref：同配置失败后保留，配置变化才换 key，只在 `createNewGame` resolve 后清理；这与后端缺 key 返回 HTTP 428 的硬契约一致。
- `scripts/test-new-game-idempotency.ts` 真实 mock fetch 验证 URL/body/credentials/header，并用 TypeScript AST 锁定 ref 的生成、复用、失败保留与成功清理顺序；已接入 compatibility-matrix/release workflow 和 responsive 门禁存在性断言。
- 2026-08-13 当前源码已通过全部 14 项 `test:*`、production audit（0 vulnerabilities）与 production build。本地 Browser fixture 已核对 desktop runtime error 零重装文案、390px diagnostics 路由与零溢出、missing-files 只给 repair，console 为 0；候选镜像/升级后/正式真机仍待发布门禁。

# FE-SAVE-GAMEDAY-HOVER-DETAILS-1 接手记录（2026-08-14，released in v0.4.16）

## 改了什么

- `SavesSection.tsx` 把“其他备份”原本内联的整行 `title` 拼装提取为 `backupDetailsTitle(backup)`；“游戏日回档”和“其他备份”两类桌面行都调用该函数。
- 自动回档的详情首段显示用户可读的“游戏日回档”，其余字段沿用既有顺序：农民、游戏内日期、地图。没有新增自定义浮层或状态，因此不会产生两套悬停生命周期。

## 影响与验证

- 影响文件：`frontend/src/games/stardew/SavesSection.tsx`、`frontend/scripts/test-save-backup-details.ts`、`frontend/package.json`；无后端/API/CSS 变更，手机端本来已把这些字段常驻显示在堆叠行中，不需要增加悬停行为。
- `npm.cmd run test:save-backup-details` 验证两个桌面区块都绑定共享详情函数，并锁定农民、地图字段仍在详情中；`npm.cmd run build` 通过。桌面 Browser QA 实际移动鼠标到首条回档行，fixture 的 5 条回档记录都得到“游戏日回档 · 农民 · 游戏内日期”详情，页面无横向溢出、无残留弹窗，console error/warn 为 0。fresh candidate 和上一正式版 Web 升级后的 Saves production chunk 还会再次验证这组详情契约。

## 下一步注意事项

- 后续给备份详情增加字段时只修改 `backupDetailsTitle`，不要在两个列表里重新内联；若改成自定义 tooltip，需要同时补键盘聚焦和读屏关联，不能只保留鼠标入口。

# FE-NEWGAME-COMMUNITY-BUNDLE-COPY-1 接手记录（2026-08-14，released in v0.4.17）

## 改了什么

- 新建存档高级设置中的“社区中心手机包”是文字误写，现已更正为 Stardew 高级游戏选项使用的“社区中心收集包”。
- 仍由 `NewGameConfig.remixedCommunityCenter` 的复选框表示普通/重新混合选择，没有改控件语义、默认值或请求结构。

## 影响文件与验证

- 产品代码只影响 `frontend/src/games/stardew/NewGameCreator.tsx`；无 CSS、类型、API 或后端改动。
- 前端 production build 通过；源码和 `dist` 产物已做正反向文案检查，确认新文案存在且旧误字不存在。

## 下一步注意事项

- 后续若把复选框改成“普通/重新混合”显式选择器，应继续保留“社区中心收集包”作为选项名称；不要把栏目名和选中值“重新混合”合并成一个含义不清的复选框标签。

# FE-MODAL-VIEWPORT-1 接手记录（2026-08-15，released in v0.4.18）

## 改了什么

- 新增 `frontend/src/core/ModalPortal.tsx`，把 Stardew 桌面端 `sd-confirm-overlay` 和移动控制/存档确认框 portal 到 `document.body`。共享层负责语义角色与标题关联、滚动锁定、初始焦点、Tab 循环、Esc、关闭后的触发按钮焦点归还，并在模态期间给其它 `body` 子树设置 `inert` 与 `aria-hidden`。
- 已迁移设置页用户删除/密码、日志清理/VNC、服务器控制、玩家操作、存档、Mod 删除、总览生命周期与首次安装提示；移动端迁移计划重启、密码、运行设置、游戏语言、Joja、存档上传与回档。设置页和任务页父容器的 `position: relative` 不再覆盖弹窗 `fixed`。危险删除、Joja、踢出/封禁与回档使用 `alertdialog`。
- 待认证玩家表新增 `sd-players-pending-table(-row)` 三列布局，移除完整玩家表的七列和 `min-width: 870px` 约束。登录/初始化页流式回退阈值调整为 `max-aspect-ratio: 8/5`，覆盖 16:10 与 3:2。
- Joja 输入旁的“填入”按钮、自动填入行为、确认词和提交门禁均原样保留。

## 影响文件与验证

- 主要影响 `frontend/src/core/ModalPortal.tsx`、`App.css`、`SavesSection.tsx`、`StardewPanel.tsx`、桌面 `pages/{Settings,JobsLogs,ServerControl,Players,Mods,Overview}Page`、移动 `Mobile{Control,Saves}Page`，以及 `scripts/test-responsive-layout.ts`；没有 API、后端状态或业务提交逻辑变化。
- 全部 17 项 `npm.cmd run test:*` 和 production build 通过。应用内 Browser 在 1536×1024 验证 3:2 登录表单完整、删除用户/清空日志居中、待认证表无横向滚动、新建与删除存档语义正确；390×844 验证计划重启、Joja（含“填入”）、回档的焦点锁定、背景隔离和关闭后焦点归还。两种视口均无横向溢出，console error/warn 为 0。

## 下一步注意事项

- 新增确认框必须复用 `ModalPortal`，提供唯一标题 ID；不可再把 `role=dialog` 只挂在页面内普通 `div` 上。危险且需立即注意的不可逆确认优先使用 `role="alertdialog"`。
- 模态异步提交期间应把 `onEscape` 设为 `undefined`，与禁用取消按钮保持一致。若以后加入嵌套模态，共享层已有栈与背景状态恢复逻辑，不要在页面组件另写第二套 body 锁定。
- 候选镜像与 `v0.4.17` Web 升级后的 production bundle 已抽验这组弹窗、3:2/移动视口及共享可访问性契约，并随 `v0.4.18` 正式发布。

# FE-CONTROL-COMMAND-PAGINATION-1 接手记录（2026-08-15，released in v0.4.18）

## 改了什么

- `JobsLogsPage.tsx` 给“最近控制命令”增加本地分页，页大小固定为 3；卡片标题继续展示接口返回的总条数，分页显示“第 n / m 页”，首尾按钮按边界禁用。
- 现有 5 秒 `listControlCommands` 刷新没有增加请求。每次成功刷新后会按新总页数校正当前页；渲染阶段直接从现有数组切片，不新增 effect、缓存或服务端分页契约。
- `JobsLogsPage.css` 限制控制命令卡片与表格滚动容器的最小宽度，使表格的 900px 最小宽度留在内部滚动层；分页栏按卡片可见宽度布局，右侧栏窄宽度下“下一页”不会被裁掉。

## 影响文件与验证

- 影响 `frontend/src/games/stardew/pages/JobsLogsPage.{tsx,css}`、`frontend/src/qa-layout-main.tsx`、`frontend/scripts/test-responsive-layout.ts`；无后端/API、数据排序、轮询周期或命令状态语义变化。
- QA fixture 扩为 7 条不同状态的控制命令。全部 17 项 `npm.cmd run test:*` 与 `npm.cmd run build` 通过；应用内 Browser 在右侧 967×732 预览实际点击到第 2、3 页，确认行数为 3/3/1、首尾禁用正确、页面无横向溢出、表格仅内部滚动、console error/warn 为 0。

## 下一步注意事项

- 若未来后端支持服务端分页，应同时提供总条数和稳定游标/页码，并明确 5 秒轮询时当前页的更新策略；在接口变更前不要把本地切片和服务端分页混用。
- 候选镜像和 `v0.4.17` Web 升级后的 production bundle 已抽验该分页，并随 `v0.4.18` 正式发布。

# FE-PLAYER-AUTH-MODES-1 接手记录（2026-08-15，released in v0.4.19，included in v0.5.0）

## 改了什么

- 桌面/移动服务器控制入口统一改为“玩家加入保护”，共用 `PlayerAuthSettingsDialog.tsx`。旧桌面 hook 和移动页重复 state/handler 已移除。
- 弹窗包含 none/global/role 三张模式卡；global 编辑统一密码，role 展示当前角色 configured 状态并只提交本次填写的密码。已有角色密码不回显，留空保持不变。
- 底部运行状态同时展示 configured/runtime 模式、待重启、Control role patch 和 Junimo 认证人数。角色列表无数据、未配齐、密码过长、revision 冲突都有具体方向性提示。
- 新增独立 `PlayerAuthSettingsDialog.css`，桌面三列、620px 以下单列；角色列表内部滚动且无页面横向溢出。ModalPortal 继续统一焦点和背景隔离。

## 影响文件与验证

- 主要文件：`types.ts`、`api.ts`、`PlayerAuthSettingsDialog.{tsx,css}`、桌面 `pages/ServerControlPage.tsx`、移动 `MobileControlPage.tsx`、`qa-layout-main.tsx`、`scripts/test-responsive-layout.ts`；删除 `useServerPassword.ts`。
- Node 22 Linux 容器执行全部 17 项前端状态/布局测试与 production build 通过。Browser 在 1280×720 验证 680px 桌面 role 弹窗，在 390×844 验证 358px 移动 role/global 弹窗；页面横向溢出均为 0，长角色列表只滚动弹窗。

## 下一步注意事项

- PUT 200 只表示配置已保存，不能改成“立即生效”；运行中必须以 `restartRequired` 提醒重启。revision conflict 后应重新打开/读取，不能自动用旧草稿覆盖。
- 不要在前端缓存或回显角色密码，也不要把 `roleId` 作为可编辑字段。设备绑定目前不在范围内，禁止用浏览器本地标识冒充游戏客户端授权。
- v0.4.19/v0.5.0 已完成自动角色隔离、Panel approve、revision/重启、旧实例兼容和真实 Web 升级/回滚并正式发布；两个真人客户端实际输入独立密码仍没有人工记录。以后修改客户端交互时必须补该人工矩阵，但不得把这个验证缺口误写成当前版本未发布。

# FE-PLAYER-LAST-SEEN-SEMANTICS-1 接手记录（2026-08-15，released in v0.5.0）

## 改了什么

- 桌面 `PlayersPage.tsx` 的第三列表头由“在线时长”改为“在线 / 最近活动”，避免同一列在线时显示持续时长、离线时显示最后在线时间却仍被理解成单一时长。
- 页面没有新增客户端时间推断；`lastSeen` 仍完全来自玩家 API。后端不再为从未在线的存档角色返回该字段后，桌面和移动端都会自然隐藏错误的“上次 今天 HH:mm”。

## 验证与注意事项

- Dockerfile 同款 Node 22 Alpine 洁净 `npm ci && npm run build` 通过，Vite 8.0.16 完成 140 modules production build。
- 若以后调整文案或移动端布局，不得用 `playersData.updatedAt` 或浏览器当前时间补齐缺失的 `lastSeen`；字段缺失代表没有可信的真实在线历史。

# FE-PLAYER-AUTH-SELF-ENROLL-1 接手记录（2026-08-17，released in v0.5.3）

## 改了什么

- `PlayerAuthSettingsDialog.tsx` 允许没有非主机角色或仍有 waiting 角色时启用 role。角色卡新增 `credentialStatus` 三态以及 store 级异常提示；管理员可代设/重置，清除后回到等待玩家第一次 `!login` 自助设置。
- 角色模式说明、空状态和计数已改为“已设置 / 待设置”，不再阻止未配置角色保存。请求仍只提交本次输入与清除，不回显、不持久化明文。
- `useStardewLifecycleActions.ts` 不再用 restart 请求前就存在的 `running` 投影清除 pending。判定抽到 `lifecycle-action-state.ts`：restart 必须观察 lifecycle job 后才在终态解锁；start 保留 running fallback。

## 影响与验证

- 类型变化位于 `frontend/src/types.ts`；桌面 `ServerControlPage.tsx`、移动 `MobileControlPage.tsx` 和 `StardewMobileShell.tsx` 继续共用同一弹窗与生命周期 hook。QA fixture、样式和 responsive 源码契约同步更新。
- 新增 `scripts/test-lifecycle-action-state.ts` 与 npm script；宿主定向测试、全量前端状态脚本和 Docker production build 的精确结果见 `docs/09-image-build.md`。

## 下一步注意事项

- `waiting` 表示玩家可首次认领，`error` 表示凭据层 fail closed；任何 UI 重构都不能把两者合并成“未设置”。也不能因角色列表为空而禁止 role，因为新建/导入后出现的角色需要使用同一首次登录契约。
- 2026-08-17 用户确认两个真实客户端完成首次认领、各自正确登录、交叉失败、清除后重认领、Panel 批准和重启保持矩阵；能力已随 annotated `v0.5.3@ede7fa3` 和同 digest 正式镜像发布。

# FE-INSTALL-SMAPI-LIVE-PROGRESS-1 接手记录（2026-08-18，released in v0.5.4）

## 改了什么

- `InstallPage.tsx` 将第四步改为“下载与环境”，右栏标题随当前阶段在 Steam 认证、镜像下载、下载任务和 SMAPI 安装之间切换。SMAPI 阶段不再使用已完成的 SDK/SteamCMD progress，也不再显示静止的“等待安装器输出”。
- `install-helpers.ts` 新增 SMAPI marker 解析与阶段进度换算：下载占安装子任务 80%→100% 区间，进而让顶部总进度在约 89%→96% 连续推进。非法字节/候选字段或非当前安装 job 均忽略。
- SMAPI 卡片显示候选源、格式化字节、百分比、缓存命中、下载完成后的校验/写入文案与持续活动点；所有进度条具备 ARIA 数值，reduced-motion 关闭脉冲与宽度过渡。隐藏控制 marker 不进入可见任务日志。

## 影响与验证

- 影响 `pages/InstallPage.{tsx,css}`、`install-helpers.ts`、`scripts/test-install-state.ts`、`scripts/test-responsive-layout.ts`。没有新增 API；job/SSE marker 契约见 `docs/06-integration.md`。
- `npm run test:install-state`、`npm run test:responsive-layout` 与 `npm run build` 已通过。状态测试覆盖下载 50%、缓存 100%、越界 marker 与错误 job type；布局测试固定下载图标、reduced-motion、ARIA 和第四步文案。

## 下一步注意事项

- 旧后端没有 SMAPI marker 时必须保留活动提示，不能重新回退到 SteamCMD 100% 或静态 Steam 认证卡；收到 100% 后也只能说下载结束/正在校验或安装，不能提前显示整个安装完成。
- 正式候选 `32108845520`、Compatibility `32108845544`、自动 Tag `32109534507` 和正式提升 `32109555161` 均成功；能力已随 `v0.5.4@e0b888c` 与 digest `sha256:4d5dbc6faf23cb15aa859deca62022e7e03dd896a7fc4c77086ac805ddb33cb2` 发布。以后修改下载阶段仍需复验慢速下载、候选切换、缓存命中、窄屏与 reduced-motion。

# FE-NEW-GAME-MODAL-COMPACT-LAYOUT-2 接手记录（2026-08-18，released in v0.5.5）

## 改了什么

- `NewGameCreator.css` 不再在 `ngc-modal <= 1100px` 时直接把三栏改成单列。新层级为 1100px 压缩三栏、780px 两栏且农场选择横跨底部、560px 单列但联机设置内部保持双列、480/360px 手机与极窄屏细化。
- 压缩布局同步缩小面板 padding、字段 label 列和农场卡间距，并把角色预览改成稳定的两列结构；删除旧 `transform:scale(.88)`，避免视觉缩放与真实占位高度不一致。无 container query 回退使用对应 viewport 断点。

## 影响与验证

- 影响 `frontend/src/games/stardew/NewGameCreator.css`、`frontend/scripts/test-responsive-layout.ts`。表单 JSX、新建存档 API、字段语义和后端均未修改。
- `npm run test:responsive-layout`、TypeScript 与 Vite production build 通过。Browser 默认 948×805 为 `221 / 430 / 192px` 三栏，840×720 与 769×500 为两栏并把八张农场图排成四列；页面级横向溢出为 0，弹窗内部滚动可达，console warn/error 为 0。

## 下一步注意事项

- 响应式判断必须继续基于 `.sd-saves-modal-card-wide` 提供的 `ngc-modal`，不能恢复 `sd-main-scroll`。1100px 是压缩三栏入口，不再是单列入口；若调整列宽，至少复验默认半屏、840×720、769×500 和小于 560px 的强制桌面夹具。
- 本地源码、production build 与 Browser QA 已完成；候选 `32127766494`、Compatibility `32127766392`、自动 Tag `32128518008` 与正式提升 `32128533342` 均成功，能力已随 `v0.5.5@a77fbe6` 和 digest `sha256:584a460c90103966394e71c67fe5416822985c9b8246013b5d2cff80400174de` 发布。

# FE-NEW-GAME-FARM-CAVE-CHOICE-1 接手记录（2026-08-23，released in v0.5.12）

## 改了什么

- `NewGameCreator` 在农场类型后加入三张农场山洞单选卡：原版事件、果蝠、蘑菇。受控字段 `farmCaveChoice` 的默认值为 `vanilla`，类型限定为 `vanilla | bats | mushrooms`，随既有新建存档 payload 原样提交。
- 卡片使用 `aria-pressed` 暴露单选状态；原版选项说明保留游戏内后续选择，果蝠/蘑菇说明会在创建时锁定。窄屏规则绑定真实的 `ngc-modal` 命名容器并改为单列。

## 影响、验证与注意事项

- 影响 `frontend/src/types.ts`、`games/stardew/NewGameCreator.tsx/.css` 和 `scripts/test-new-game-idempotency.ts`。idempotency 专项和 production build 通过；Browser 在 1280×720、390×844 验证默认/点击状态、单列布局、无横向溢出和零 console warning/error。
- 不要把 UI 默认值改成 `bats` 或 `mushrooms`；后端缺省也必须保持 `vanilla`。响应式选择器依赖 `.sd-saves-modal-card-wide` 的 `container-name: ngc-modal`，独立夹具必须复现该祖先契约后才能判定移动端回归。
- 正式候选 `32623320406` 的完整前端回归、production build、候选镜像和升级后 bundle 验收通过；自动 tag `32623853636`、正式提升 `32623863894` 与 GitHub Release 成功，能力已随 `v0.5.12@5141cd54` 发布。
# INSTALL-POLL-1：授权超时中文与检查容器频率（2026-09-05，未发布）

- 授权超时在安装进度中显示“Steam 登录授权确认超时，请重新安装并及时完成 Steam 验证。”；保留原始任务错误供诊断。
- `GameInstallRail.tsx` 终态只刷新一次游戏目录并结束轮询；`driver.go` 的读取状态校正复用 10 分钟内成功的游戏文件检查证据。安装、启动仍直接检查文件，缓存过期重新检查。
- Docker 现场近 3 分钟观察到 14 次 server 镜像临时容器创建/销毁，与读状态重复 verifier 路径一致。新增缓存复用/过期后缺文件回归及中文超时断言；Go ReconcileState/InstallationDiagnostic/SteamCMD 与前端安装状态、production build 通过。接口不变，后续注意启动检查不可被读取缓存替代。
# WORLD-CARD-1：存档地图与世界名称编辑（2026-09-05，未发布）

- `GameLibrary.tsx` 世界卡片按当前启用存档的 farmType 选择八种内置农场素材；首次加载、无存档、未知地图及读取/图片失败时显示标准农场。管理员自定义地图可读取已有 farm catalog 图标；实例状态更新时重新读取存档。
- 名称旁 13px 铅笔，管理员点击可行内修改，Enter/保存提交、Esc/取消退出，失败保留输入并显示错误。`PATCH /api/instances/:id` 只改名称和更新时间，复用管理员权限与审计，校验 1–40 字及控制字符；storage.RenameInstance 不改变目录、ID、存档与运行状态。
- `TestInstanceRenamePersistsNameAndPreservesRuntime` 验证未登录、非法名称、持久化与运行状态保留；前端游戏库回归和 production build 通过。Browser 夹具森林地图来源、铅笔行内输入、Enter 保存已验收。后续注意自定义图标遵循已有目录权限，普通用户不显示改名入口。
