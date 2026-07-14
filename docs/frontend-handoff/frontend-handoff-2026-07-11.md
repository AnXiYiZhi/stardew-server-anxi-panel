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
