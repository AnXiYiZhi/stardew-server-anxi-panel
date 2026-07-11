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
