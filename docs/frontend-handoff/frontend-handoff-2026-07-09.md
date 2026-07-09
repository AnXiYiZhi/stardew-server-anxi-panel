# FE-MOBILE-FIXES-1 手机端适配新一轮系统性修复

## 背景

项目前端过去多轮迭代零散地做过移动端响应式处理，`StardewPanel.css` 里累积了 16 种不同的 `max-width`/`@container` 断点数值（460/520/620/640/720/760/780/820/880/900/920/960/980/1180px 等），基本是"哪个页面出问题就在哪个页面局部加断点"。这轮用户要求做一次系统性摸底和修复。

前置调研（Explore agent + 直接 grep 确认）定位到 7 类具体问题；经用户确认范围：**不重构断点体系**，只修复调研中发现的具体真实问题；**验证方式仅用浏览器模拟**（无 Playwright/Puppeteer 环境，用系统已安装的 Chrome headless + 独立 CSS 测试页做计算样式验证），不做真机验证。

## 改了什么

1. **iOS 输入框自动缩放**：`frontend/src/games/stardew/stardew-theme.css` 里 `.sd-input` 基础规则字号引用 `--sd-font-size-control`（13px）。在 `@media (max-width: 640px)` 内新增覆盖 `font-size: 16px; min-height: 32px;`——**必须放在 `.sd-input` 基础规则之后**（同优先级下，写在前面会被后面的无条件基础规则盖掉，本轮实测踩过这个坑）。另外 `StardewPanel.css` 里两个独立小字号 `<select>`：`.sd-mods-sync-select`（11px）、`.sd-players-action-select`（12px）同样在各自的 `max-width: 640px` 块内补了 16px 覆盖。`.sd-mods-upload-input` 是 `type="file"`，不会触发 iOS 自动缩放，未改。
2. **安全区适配**：`frontend/index.html` 的 viewport meta 补 `viewport-fit=cover`；`frontend/src/App.css` 的 `#root:has(.sd-shell)` 补 `padding: env(safe-area-inset-top/right/bottom/left, 0px)`（带 0px fallback，无刘海设备无影响）。注意 `.sd-shell` 内部用 `transform: scale(var(--sd-ui-scale))` 缩放整个 1536×1024 设计稿贴合视口，`--sd-ui-fluid-scale` 用的是 `100vw`/`100dvh` 视口单位而非父容器百分比，所以 `#root` 加 padding 后缩放系数会有极小偏差（视觉上可忽略），刘海屏下的真实呈现仍需真机确认。
3. **触控热区**：移动端横向导航图标 `.sd-sidebar .sd-nav-item` 从 42×38px 提到 44×44px；因为可用高度受 `.sd-shell` 第二行网格行高限制，同步把该行从 `48px` 调到 `54px`（54 = 44 触控区 + 上下各 5px padding）。Players 页 `.sd-players-icon-button`（30×30px）三个按钮目前恒为 `disabled` 占位（"待接入"，功能未接入），本轮判断为非活跃控件，暂不调整，留给后续真正接入交互时一并处理。
4. **弹窗溢出**：`.sd-confirm-dialog`（被 ModsPage 等 9 个页面复用的通用确认弹窗）补 `max-height: 90vh; overflow-y: auto;`，长文案或矮视口下可内部滚动，不再整体溢出。`SavesSection.tsx` 的 saves modal 本身已有同类处理，未改。
5. **表格横向滚动可发现性**：Players 表格（`.sd-players-table-placeholder`）和存档备份表格（`.sd-save-backups-table`）在移动端断点内补 `-webkit-overflow-scrolling: touch` + 右侧 18px 渐变遮罩（`::after`，`pointer-events: none`），提示用户表格可以横滑；不做卡片化重构。

## 影响文件

- `frontend/index.html`
- `frontend/src/App.css`
- `frontend/src/games/stardew/stardew-theme.css`
- `frontend/src/games/stardew/StardewPanel.css`

未改任何 TSX 组件、API、权限、路由或 Junimo 通信；纯 CSS/HTML 层面的响应式修复。

## 如何验证

- `cd frontend; npm.cmd run build` 通过。
- 本环境没有 Playwright/Puppeteer，用系统已安装的 Chrome/Edge headless 做了两类验证：
  1. `qa-layout.html?state=running` 在 390×844 与 320×568 下截图，确认 Overview 页无破版、无页面级横向溢出（截图留在本机临时目录，未入库）。
  2. 单独写了一个引用同一份 `stardew-theme.css`/`StardewPanel.css` 的独立测试页，用 `getComputedStyle` 精确核对四个改动点：`.sd-input` 移动端 computed font-size = 16px（桌面宽度下仍是 13px，确认无回归）、`.sd-confirm-dialog` computed `max-height`/`overflow-y` 生效、导航图标 computed 尺寸 = 44×44px、`.sd-players-table-placeholder::after` 在 `.sd-main-scroll` 容器查询上下文里 computed `content`/`position`/`width` 确认渐变遮罩真实生成。
- 未做的验证：没有对 10 个路由逐页做完整截图回归（qa-layout harness 只能通过点击应用内导航切页，本环境没有可编程点击的浏览器自动化工具，只能做静态单页截图）；safe-area 在真实刘海屏/手势条设备上的实际呈现、虚拟键盘弹出时对确认弹窗的实际遮挡效果，都只能推导而无法在此环境模拟验证。

## 下一步注意事项

- **需要真机复核**：safe-area 在真实 iPhone（刘海/灵动岛/手势条）上的实际内边距效果，以及虚拟键盘弹出时 `.sd-confirm-dialog`（`max-height: 90vh`）是否足够避让键盘。这两项浏览器模拟器无法准确复现。
- Players 表格行内的 `.sd-players-icon-button`（发送消息/踢出/更多操作）目前是禁用占位；后续真正接入这些交互时，应同步把触控热区提到 ≥44px（当前 30×30px 偏小），并考虑该改动对表格行高的连带影响。
- 中期路线里"更完整的移动端导航和表格卡片化"（`docs/08-future-roadmap.md`）不在本轮范围内，本轮只做了轻量的横滑可发现性提示，表格结构本身未变。
- 如果后续要系统性收敛 `StardewPanel.css` 里 16 种不同断点值为统一 design token，需要新开一轮单独排期（本轮按用户要求明确排除）。

# PLAYERS-KICK-1 踢出玩家 + PASSWORD-STATUS-1 服务器密码设置弹窗

## 背景

用户要求实现玩家页一直标"待接入"的踢出功能，以及设置服务器加入密码。后端调研发现 JunimoServer 上游没有运行时改密码/踢人的 REST API（详见 `docs/backend-handoff/backend-handoff-2026-07-09.md`），踢人改走面板自带 SMAPI Mod 命令队列，密码走 `.env` + 只读认证状态代理。前端这次只对接后端新增的 4 个接口，不涉及新页面。

用户明确要求：密码设置**不要**做成 `SettingsPage.tsx` 里新的一个区块，而是复用 `ServerControlPage.tsx` 里原本写着"待接入：端口/可见性/密码配置"的"服务器设置"快捷按钮，把它改名成"服务器密码设置"并直接接进去。

## 改了什么

1. **PlayersPage.tsx 踢出玩家**：
   - 新增 `KickTarget` 类型和 `kickConfirmTarget`/`kickSelectId`/`kickBusy`/`kickError`/`kickMessage` 五个 state。
   - 玩家表格行内 `sd-players-icon-boot` 图标按钮：满足 `isAdmin && isRunning && status==='online' && !isHost && uniqueMultiplayerId` 时可点击，点击后设置 `kickConfirmTarget` 弹出确认框（不是直接踢，二次确认避免误触）。
   - "管理操作"卡片踢出玩家的 `<select>` 改为渲染 `onlinePlayers`（在线、非主机、有 `uniqueMultiplayerId` 的玩家）列表，按钮点击同样先弹确认框。
   - 新增 `handleConfirmKick()`：调用 `kickPlayer(uniqueMultiplayerId, name)`，成功后 `await dashboardData.refreshPlayers()` 让名册立刻反映玩家已断开（实际断开有 SMAPI Mod tick 轮询延迟，通常 2 秒内）；结果/错误分别展示在管理操作卡片底部。
   - 页面头部描述文案、管理操作区底部提示语相应更新为"支持踢出在线玩家"而不是统一的"待接入"。
   - 未改动：封禁玩家、白名单管理、权限设置三个卡片保持原样（仍是禁用占位），玩家行内"发送消息""更多操作"两个图标按钮也保持禁用占位。

2. **ServerControlPage.tsx 服务器密码设置**：
   - `key="server-settings"` 按钮改为 `key="server-password-settings"`，移除 `disabled`（改为 `disabled={!isAdmin}`），文案从"服务器设置 / 待接入"改成"服务器密码设置 / 配置玩家加入密码"，`onClick` 绑定新增的 `openPasswordSettings()`。
   - 新增一整套 `passwordOpen`/`passwordDraft`/`passwordVisible`/`passwordLoading`/`passwordSaving`/`passwordError`/`passwordMessage`/`passwordStatus`/`passwordStatusLoading`/`passwordStatusError` state，完全比照文件里已有的 `scheduleOpen`（计划重启）那一套弹窗 state 命名和结构写的，没有引入新的弹窗组件/库。
   - `openPasswordSettings()`：打开弹窗、`getInstanceServerPassword()` 读当前密码回填草稿，同时并行 `loadPasswordStatus()` 读认证状态（服务器未运行时会失败，弹窗里会展示"服务器未运行，无法读取密码保护状态"而不是把整个弹窗打不开）。
   - `handleSaveServerPassword()`：前端先做 128 字符长度校验（和后端一致，双重保险），调用 `updateInstanceServerPassword(passwordDraft)`，成功后提示"密码已保存，需要重启服务器容器后才会生效"。
   - 弹窗 JSX 插入在原有 `scheduleOpen` 弹窗块之后、组件最外层 `</div>` 之前，复用 `sd-confirm-overlay`/`sd-confirm-dialog`/`sd-confirm-warning`/`sd-confirm-actions`/`sd-schedule-field`/`sd-schedule-summary` 这些已存在的 CSS class，**没有新增任何 CSS**。密码输入框旁边加了一个"显示/隐藏"切换按钮（`type` 在 `password`/`text` 之间切换），因为密码弹窗和计划重启弹窗不一样，用户大概率需要肉眼确认自己输入的密码对不对。

3. **types.ts / api.ts**：新增 `InstanceServerPasswordConfig`、`InstancePasswordStatus` 类型；`getInstanceServerPassword`/`updateInstanceServerPassword`/`getInstancePasswordStatus`/`kickPlayer` 四个函数，路径分别对应 `GET/PUT /api/instances/:id/config/server-password`、`GET /api/instances/:id/password-status`、`POST /api/instances/:id/players/kick`。都用现成的 `request<T>()`，没有像 `runCommand`/`sendSay` 那样加 `AbortController` 超时（那两个是等待 attach-cli 真实命令输出才需要 40 秒超时，这几个都是快速的 JSON 读写或 fire-and-forget）。

## 影响文件

- `frontend/src/types.ts`
- `frontend/src/api.ts`
- `frontend/src/games/stardew/pages/PlayersPage.tsx`
- `frontend/src/games/stardew/pages/ServerControlPage.tsx`

未新增/修改任何 CSS 文件，纯粹复用已有 class。

## 如何验证

- `cd frontend; npx tsc --noEmit -p .`：通过，无类型错误。
- `cd frontend; npm run build`（`tsc -b && vite build`）：通过。
- **没有做的验证**：没有连接一个真实运行中的 Stardew 实例在浏览器里实测点击效果（弹窗视觉、确认流程、移动端窄屏下密码弹窗和踢出确认框的实际呈现），也没有用 Browser QA 截图。这次改动风险相对可控（复用现成 CSS class 和现成弹窗模式，`tsc`/`build` 都过了），但如果用户反馈弹窗布局有问题，需要真的打开浏览器看一下。

## 下一步注意事项

- 踢出是 fire-and-forget，`handleConfirmKick` 拿到的 `res.output` 只是"指令已提交"这类固定文案，不代表玩家真的被踢了；如果之后要做更精确的"踢出成功/失败"反馈，需要后端先设计命令结果回传通道（见后端接手文档的"下一步注意事项"），前端目前的 `kickMessage` 展示逻辑不需要跟着改，回传通道设计好之后再决定要不要轮询结果。
- 密码弹窗里的"密码保护状态"部分依赖服务器处于 running 且 JunimoServer API 已就绪，`junimo_api_unavailable` 这类错误目前只是原样展示 `errorMessage(e)` 的后端错误文案，没有专门做更友好的提示；如果用户反馈这块报错文案太技术化，可以在 `loadPasswordStatus` 里针对 `ApiError.code === 'junimo_api_unavailable'` 做单独文案。
- 封禁玩家/白名单/权限设置三个待接入功能没有动，`sd-players-icon-more`/发送消息图标也没动；不要因为这次做了踢出就顺手把其它占位也接了，用户这次明确只要求这两个功能。

# ADMIN-GATE-LIFECYCLE-1 启动/停止/重启按钮补齐管理员权限门控

## 背景

用户在本轮改动即将推送发版前，凭经验提出"普通用户不能参与服务器的启动和停止，也不能给服务器添加 mod"，要求复查。排查（Explore agent 全量扫描 `frontend/src/games/stardew/pages/` 每个页面的按钮 `disabled` 逻辑，对照后端每个 handler 是否 `requireAdmin`）确认：

- Mod 上传（`ModsPage.tsx`）、玩家踢出（`PlayersPage.tsx`，本轮新增）、存档、设置、安装、任务日志、诊断导出等页面**全部正确**用 `disabled={!isAdmin || ...}` 或整块 `{isAdmin ? ... : null}` 挡住了非管理员。用户随后追问"一键安装也不能用"，专门复查了游戏安装（`InstallPage.tsx`，多处 `isAdmin` 分支：839/848/861/875/910/917/1148 行）和 Nexus Mod 一键安装（`ModsPage.tsx` 的 `searchedModBaseCanInstall`/`searchedModCanClickAction`，第 1758/1765/1768 行都强制 `!isAdmin` 时返回不可点，按钮 `disabled={!canInstall}` 在第 2051 行）——**这两处本来就是对的，不是本次修复范围**，确认后未做改动。
- 但**启动/停止/重启这三个最核心的生命周期按钮**在两个地方完全没有 `isAdmin` 判断：`ServerControlPage.tsx`（`canStart`/`canStop`/`canRestart` 三个派生变量只算了运行状态和 busy，没有算 `isAdmin`，尽管组件里 `const isAdmin = user.role === 'admin'` 已经声明）；`OverviewPage.tsx`（总览页的精简版启停按钮）情况更彻底——这个组件的 props 解构里**根本没有取 `user`**，整个文件没有 `isAdmin` 这个概念。后端 `POST /start`、`/stop`、`/restart` 三个接口都是 `requireAdmin`（`lifecycle_handlers.go`），所以后果不是权限被绕过（后端仍然会 403），而是普通用户在两个页面上都能看到"可点击"的启动/停止/重启按钮，点了才发现被拒绝——体验不一致，容易造成困惑或被误认为是 bug。

## 改了什么

- `ServerControlPage.tsx`：`canStart`/`canStop`/`canRestart` 三个表达式前面加 `isAdmin &&`；三个按钮的 `title` 补充非管理员时的提示文案（"仅管理员可启动/停止/重启服务器"），和文件里其它已经做对了的按钮（计划重启、VNC 显示、服务器密码设置）保持一致的提示风格。
- `OverviewPage.tsx`：`export function OverviewPage({ instanceState, onNavigate, dashboardData })` 补上 `user` 解构，新增 `const isAdmin = user.role === 'admin'`；`renderLifecycleButtons()` 里启动/停止/重启三个按钮的 `disabled` 补上 `|| !isAdmin`，并加 `title`（管理员时 `undefined`，即用浏览器默认无 tooltip，非管理员时显示"仅管理员可启动/停止/重启服务器"）。
- 两处的"确认停止/确认重启"二次确认弹窗按钮**没有单独加权限判断**：它们只有在对应的停止/重启按钮被点击后才会出现，而那两个按钮现在已经被 `!isAdmin` 挡住了，属于同一根因的下游症状，不需要重复判断。

## 影响文件

- `frontend/src/games/stardew/pages/ServerControlPage.tsx`
- `frontend/src/games/stardew/pages/OverviewPage.tsx`

## 如何验证

- `cd frontend; npx tsc --noEmit -p .` 通过。
- `cd frontend; npm run build`（`tsc -b && vite build`）通过。
- **未做的验证**：没有用非管理员账号登录浏览器实测这两个页面的按钮是否真的变灰、`title` 提示是否正确显示；纯代码审查 + 类型检查通过，逻辑上 `disabled` 表达式新增的 `isAdmin`/`!isAdmin` 条件是正确的，但建议下一位维护者找一个普通用户账号登录实测一次。

## 下一步注意事项

- 这次是一次**专项复查**，只针对"后端 requireAdmin 但前端按钮没挡"这一类问题做了全量排查，结论是除了这两处启停按钮，其余页面都是对的。以后新增任何会调用 `requireAdmin` 接口的按钮/表单，必须在写的时候就带上 `!isAdmin` 判断，不要等到发版前才靠人工审查兜底。
- 如果以后要重构 `ServerControlPage.tsx`/`OverviewPage.tsx` 的生命周期按钮逻辑，注意 `canStart`/`canStop`/`canRestart`（`ServerControlPage.tsx`）和 `renderLifecycleButtons()`（`OverviewPage.tsx`）是两套独立实现，没有共享逻辑；这次是分别在两处手动加的 `isAdmin` 判断，没有做抽取公共 hook 的重构（超出本次修复范围，按"外科手术式修改"原则未做）。

# USER-PASSWORD-RESET-1 用户管理新增"重置密码"

## 背景

用户提出面板账号（不是游戏加入密码）改密码权限规则："普通用户不能改自己密码，管理员能改自己的和普通用户的，第一个注册的管理员能改所有，包括自己"。调研发现后端 `PATCH /api/users/{id}` 早支持 `password` 字段，前端从未接入；同时后端权限检查有个 bug 比这条规则更严格（详见 `docs/backend-handoff/backend-handoff-2026-07-09.md` 的 `USER-PASSWORD-RESET-1`，已在同一轮修复）。这次前端只需要在已有的"用户管理"区加一个入口。

## 改了什么

- `api.ts` 新增 `updateUserPassword(id, password)`，`PATCH /api/users/{id}` body `{password}`，返回类型和 `updateUserRole` 一样是 `{ user: PanelUser }`。
- `SettingsPage.tsx` `UserManagementSection`：
  - 新增 state：`passwordTarget`（当前弹窗操作的目标用户）、`passwordDraft`、`passwordBusy`、`passwordDialogError`、`passwordSelfChanged`。
  - 新增 `openPasswordDialog(user)` 打开弹窗，`handleChangePassword()` 提交。
  - 每行用户操作区新增"重置密码"按钮，放在最前面（禁用/删除按钮之前）。可见性用 `canChangePassword = isSuperAdmin || isSelf || !isAdminTarget`——**特意没有复用**已有的 `canManageTarget`（`isSelf ? false : (isSuperAdmin || !isAdminTarget)`），因为"能不能管理别人"（禁用/删除，故意排除自己）和"能不能改密码"（故意允许改自己）语义相反，硬复用会把改自己密码也一起禁用掉。
  - 弹窗是新写的（不是复用 `ConfirmDialog`，因为需要一个密码输入框），沿用 `sd-confirm-overlay`/`sd-confirm-dialog`/`sd-confirm-actions` 现成 CSS class。前端也做了 ≥6 位的长度校验（和后端 `validatePassword` 一致），按钮在小于 6 位时直接禁用。
  - **改自己密码的特殊处理**：提交成功后不立即关闭弹窗，而是把 `passwordSelfChanged` 设为 `true`，弹窗内容换成"密码已修改，当前会话已失效，即将跳转到登录页…"，`setTimeout` 1.2 秒后 `window.location.reload()`。这是因为后端改自己密码会撤销当前 session，如果直接关闭弹窗当没事发生，用户下一次点别的按钮会莫名其妙收到"未登录"报错，体验很差；改成显式提示+自动刷新，让 `App.tsx` 的 `boot()` 走一遍探测 401 → 显示登录页的既有逻辑，不需要额外写全局 401 拦截器。
  - 改别人的密码则正常关闭弹窗 + `loadUsers()` 刷新列表，不受影响。

## 影响文件

- `frontend/src/api.ts`
- `frontend/src/games/stardew/pages/SettingsPage.tsx`

## 如何验证

- `cd frontend; npx tsc --noEmit -p .` 通过。
- `cd frontend; npm run build`（`tsc -b && vite build`）通过。
- **未做的验证**：没有用三种角色（超级管理员/普通管理员/普通用户）分别登录浏览器实测一遍"重置密码"按钮的可见性、弹窗交互、改自己密码后是否真的正确跳转登录页。逻辑上是对的（`tsc`/`build` 都过了，后端权限测试也过了），但这是一个涉及登录态的交互流程，强烈建议下一位维护者用真实浏览器走一遍，尤其是"改自己密码后自动 reload 跳登录页"这一步。

## 下一步注意事项

- 如果以后要做"用户自己改自己密码"（面向普通用户的自助改密码入口），这是一个全新功能而不是这次的延伸——这次明确排除了普通用户改自己密码的可能性，别搞混了两个需求。
- `passwordSelfChanged` 这个 state 目前只在这一个组件内部使用，没有跟全局的登录态/session 逻辑打通；如果以后要在别的地方（比如密码过期强制修改）复用"改密码后跳登录页"这个模式，值得抽成一个小 hook，但这次只有一处用到，没有做这个抽象。
