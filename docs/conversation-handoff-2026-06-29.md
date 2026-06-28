# Conversation Handoff 2026-06-29

## FE-R6: SavesPage 存档管理页真实化（像素视觉迁移）

### 目标

把 `/instances/stardew/saves` 存档管理页从旧样式（`button`、`modal-card` 等 App.css 类）完整迁移为 Stardew 像素主题视觉，保留并强化所有已有功能，补上运行中保护的明确提示。

### 改了什么

| 文件 | 修改 |
|------|------|
| `frontend/src/games/stardew/SavesSection.tsx` | 全面重写 CSS 类为像素主题；删除确认从 `window.confirm` 改为内联对话框；新增 `onSavesChanged` prop；`selectSaveAndStart`/`createNewGame`/`uploadSaveCommitAndStart` 成功后均先 `await loadSaves()` 再通知上层 |
| `frontend/src/games/stardew/pages/SavesPage.tsx` | `onJobStarted` 新增 `refreshInstanceState` 调用；传 `onSavesChanged={dashboardData.refreshSaves}` |
| `frontend/src/games/stardew/StardewPanel.css` | 新增约 160 行存档页专用样式（`sd-saves-*`、`sd-save-*`、`sd-saves-modal-*`、`sd-saves-preview-*`） |
| `docs/handoff-roadmap.md` | 在 Current Context 顶部新增 FE-R6 完成节 |

### 各改动细节

**SavesSection.tsx 视觉迁移**

- 页头：`sd-saves-header`（flex 两端对齐）+ 左侧标题/警示横条 + 右侧操作按钮组。
- 操作按钮：`sd-btn-green`（创建/启动）、`sd-btn-tan`（刷新/上传/设为启动/取消）、`sd-btn-delete`（删除）。
- 运行保护：服务器 `running`/`starting` 时创建/上传/删除/切换按钮保持可见但 `disabled`，顶部显示金色警示条 `sd-saves-running-hint`，`title` tooltip 说明原因，不再隐藏按钮。
- 存档卡片：`sd-save-card`（活跃存档加 `active` class，绿色边框背景）+ `sd-save-active-tag` 徽章 + `sd-save-meta` 元数据行 + `sd-save-card-actions` 纵向按钮列。
- 删除确认：`window.confirm()` 替换为 `confirmDeleteName` state 驱动的 `sd-confirm-overlay` + `sd-confirm-dialog`，视觉风格与 ServerControlPage 一致。
- 弹窗外壳：新游戏弹窗和上传弹窗都使用 `sd-saves-modal-overlay` + `sd-saves-modal-card`（宽版加 `sd-saves-modal-card-wide`），`NewGameCreator` 内部样式不改。
- 上传预览：`sd-saves-preview-table` + `sd-saves-preview-row` + `sd-saves-preview-label` 替代旧的 `upload-preview-*` 类。
- 空状态：`sd-saves-empty`（虚线边框卡片）+ `sd-saves-empty-title` + `sd-saves-empty-hint`，空状态下也提供创建/上传按钮。

**SavesPage.tsx 更新**

- `onJobStarted` 回调增加 `dashboardData.refreshInstanceState()`，让实例状态立即更新（之前只有 `refreshJobs`）。
- 新增 `onSavesChanged={dashboardData.refreshSaves}`，操作完成后同步刷新公共数据层的存档数据（影响 OpsRail/Overview 中的存档摘要）。

**StardewPanel.css 新增类**

31 个新 CSS 类，全部以 `sd-saves-` 或 `sd-save-` 开头，不影响其他页面。

### 影响的接口/文件

- 无新增后端接口，无改动 API 签名。
- `SavesSection` 新增 `onSavesChanged?: () => void` 可选 prop，向后兼容（不传时静默跳过）。
- 未改动 `NewGameCreator.tsx`、`NewGameCreator.css`、`useStardewDashboardData.ts`、`stardew-routes.ts`。

### 已接入的存档 API

| API 函数 | 状态 |
|----------|------|
| `getSaves` | ✅ 接入 |
| `selectSave` | ✅ 接入 |
| `selectSaveAndStart` | ✅ 接入 |
| `deleteSave` | ✅ 接入 |
| `exportSave` | ✅ 接入 |
| `createNewGame` | ✅ 接入 |
| `uploadSavePreview` | ✅ 接入 |
| `uploadSaveCommitAndStart` | ✅ 接入（含取消/清理 token） |
| `getSavesPreflight` | ❌ 未接入（如后端 API 可用可补） |

### 如何验证

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
# 预期：exit 0，39 模块，JS ~261 kB
```

已验证通过（exit 0），JS bundle 261.21 kB，CSS 54.56 kB。

手动验证点：
- `/instances/stardew/saves` 显示像素风存档列表（或像素风空状态）。
- 活跃存档卡片绿色边框 + 「当前」绿色徽章。
- 每张存档卡片展示：存档名、农场名、农民名、游戏时间（年/季/日）、地图类型、文件大小、修改时间，解析失败时显示红色错误条。
- 点击「创建存档」弹出完整 `NewGameCreator` 自定义存档弹窗（三列农场选择、角色/宠物/小屋等字段全保留）。
- 点击「上传存档」弹出 ZIP 上传 → 预览确认流程。
- 点击「删除」弹出像素风二次确认对话框（`sd-confirm-dialog`），取消/确认均正常。
- 服务器运行中时，创建/上传/删除/切换按钮全部 disabled，顶部显示金色警示条。
- 导出存档正常触发浏览器下载。
- 左侧导航、顶部栏、Overview、ServerControlPage 不受影响。

### 下一步注意事项

- `JobsLogsPage` 仍是占位；`onJobStarted` 会跳转到 jobs 页但用户看不到任务详情。下一步应迁移 `JobsSection`（SSE 日志流）进 `JobsLogsPage`（FE-R7）。
- `SavesSection` 内部有自己的 `loadSaves()`，与 `dashboardData.saves` 并行维护，属于可接受的轻微重复请求。后续可通过 `refreshTrigger` 或传入 `dashboardData.saves` 替代内部请求。
- 备份恢复 UI 尚未接入（后端 `GET /api/instances/:id/backups` 等 API 已存在），可作为存档页后续增强。
