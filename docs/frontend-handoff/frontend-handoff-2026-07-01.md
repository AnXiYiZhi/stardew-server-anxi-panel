# 前端接手文档 2026-07-01

## 本次改动：Mod 页参考 EMP 管理台改为三段式工作台

参考页面来自 `E:\源码\emp_源码\dst-management-platform-web\src\views\game\mod.vue` 及同目录的 `download.vue`、`add.vue`、`setting.vue`。参考项目的核心结构是顶部 Tab：Download 负责远端搜索，Add 负责已下载 Mod 管理和批量启用，Setting 负责已启用 Mod 配置。

本项目对应改为 `下载模组 / 添加模组 / 配置模组` 三个页内 Tab，仍然只在 `frontend/src/games/stardew/pages/ModsPage.tsx` 内实现，没有新增路由，也没有改后端 API：

- `下载模组`：承载 Nexus Mods 搜索。搜索条增加“名称 / ID”选择器占位，结果改为带预览图的卡片网格，并保留“打开 N 站”和禁用态“安装待接入”。
- `添加模组`：承载原有本服已安装 Mod 列表、玩家同步统计、同步包导出、上传 Mod、删除 Mod、导出整包、重启提示等功能。
- `配置模组`：先做成后续能力入口，占位展示启用/禁用、依赖检查、更新检查、SMAPI 配置面板；当前没有真实启用/禁用或配置接口，所以所有危险/未完成操作保持禁用或说明态。

样式集中在 `frontend/src/games/stardew/StardewPanel.css`，新增 `.sd-mods-workbench`、`.sd-mods-tabs`、`.sd-mods-tab-*`、`.sd-mods-settings-*`，同时把 `.sd-mods-nexus-list` 改为响应式卡片网格。移动端 720px 以下会把 Tab、搜索栏、Nexus 卡片和配置布局纵向堆叠。

验证：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

构建通过（`tsc -b && vite build`）。下一步如果要真正接近 EMP 的完整体验，需要后端补齐“下载安装到服务器 / 启用禁用 / 依赖检查 / 配置表单”能力，再把当前禁用入口逐步点亮。

## 本次改动：Mod 管理第二阶段——Nexus Mods 只读搜索

在既有 `ModsPage` 内加了"在线搜索（Nexus Mods）"区域，没有新建路由。普通登录用户和管理员都能用（不像玩家同步分类下拉框那样要求管理员）。

### 类型与 API

`frontend/src/types.ts`：

- `ModInfo` 新增可选字段 `updateKeys?: string[]` 和 `nexusModId?: number`（对应后端 manifest 解析扩展，目前 UI 没有直接展示这两个字段，是为了类型和后端 `GET .../mods` 响应对齐）。
- 新增 `NexusModSearchResult`（`modId`/`name`/`summary`/`author`/`version`/`updatedAt`/`endorsementCount`/`downloadCount`/`pictureUrl`/`nexusUrl`/`installed`/`installedFolderName`/`installedVersion`）和 `NexusModSearchResponse`（`query` + `results`），对应后端 `GET .../mods/nexus/search` 的返回结构。

`frontend/src/api.ts` 新增 `searchNexusMods(query, instanceId?)`，对应 `GET .../mods/nexus/search?q=...`，走通用 `request()` 封装（自动带 `credentials: 'include'`，401/非 2xx 走 `toApiError` 抛 `ApiError`）。

### ModsPage 改动

`frontend/src/games/stardew/pages/ModsPage.tsx`：

- 新增 `NexusResultCard` 组件：展示图片（有 `pictureUrl` 才渲染）、名称、已安装标记（绿色 `sd-tag`，悬浮提示文件夹名，文字带上 `installedVersion`）、作者/版本/更新时间（复用 `sd-mods-meta-item`/`sd-mods-meta-label` 这套已有样式）、摘要（复用 `sd-mods-meta-desc`）、下载量/认可数统计行；操作区是"打开 N 站"按钮（`window.open(nexusUrl, '_blank', 'noopener,noreferrer')`，不是 `<a>` 标签，保持和页面其余操作一致用 `<button>`）+ 一个禁用态的"安装待接入"小标记（复用 `.sd-mods-pending-badge` 样式），**没有任何安装按钮**，避免误导用户以为能直接装。
- 页面级新增"在线搜索（Nexus Mods）"区块（放在概览统计卡片之后、玩家同步区域之前）：输入框（`.sd-input`，回车触发搜索）+ 搜索按钮，关键词 trim 后为空时按钮禁用并在 `title` 提示；搜索中按钮文字变"搜索中…"；失败显示中文错误条（复用 `.sd-mods-list-error`，文案直接来自后端 `message`，因为后端已经返回中文文案，`errorMessage()` 没找到 `errorCodeMap` 命中时会原样透出 `error.message`，所以不需要额外维护一份 nexus 错误码到中文的映射表）；无结果时显示"未找到匹配的 Mod，换个关键词试试。"；结果用 `NexusResultCard` 列表展示。
- 新增 state：`nexusQuery`/`nexusLoading`/`nexusError`/`nexusResults`，和 `handleNexusSearch()`。不接入 `dashboardData`（这是只读外部搜索，不影响本地 Mod 列表/同步分类，没有缓存或跨页面共享的必要）。

`frontend/src/games/stardew/StardewPanel.css` 新增一组 `.sd-mods-nexus-*` 类（`-search-row`/`-empty`/`-list`/`-card`/`-card-pic`/`-card-main`/`-card-header`/`-card-name`/`-card-meta`/`-card-stats`），卡片整体结构和现有 `.sd-mods-card` 保持视觉一致（同样的边框、背景、圆角），只是多了一个 56×56 的封面图位置。操作区直接复用了 `.sd-mods-card-actions`（纵向排列），没有新建。

### 权限边界

- 管理员和普通登录用户都能搜索、查看结果、点"打开 N 站"——对应后端 `requireAuth`（不是 `requireAdmin`）。
- 没有人能点"安装"，因为这个阶段压根没有安装按钮，只有一个禁用提示。

验证：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

构建通过（`tsc -b && vite build`，无类型错误）。本次没有起开发服务器做真实 Nexus 账号的端到端验证（需要真实 `NEXUS_API_KEY`），下一位维护者如果要看实际效果：在后端配置好 `NEXUS_API_KEY` 后启动开发服务器，搜索一个常见 Mod 名（比如 "Stardew Valley Expanded"）和一个数字 ID（比如 "2400"），确认两条路径都能出结果；再随便挑一个本地已装且 manifest 带 `UpdateKeys: ["Nexus:<id>"]` 的 Mod，搜它确认卡片上出现"已安装"标记。

## 本次改动：Mod 玩家同步包（第一阶段）

在既有 `ModsPage` 内加了一个“玩家同步”区域，没有新建路由，复用现有像素风样式（`sd-tag`、`sd-btn-tan` 等）。

### 类型与 API

`frontend/src/types.ts`：

- 新增 `ModSyncKind = 'server_only' | 'client_required' | 'unknown'`。
- `ModInfo` 新增 `syncKind: ModSyncKind`（恒有值）和可选 `syncNote?: string`。
- 新增 `ModSyncSummary`（total/serverOnly/clientRequired/unknown）和 `ModSyncPlanResult`（mods + summary），对应后端 `GET .../mods/sync-plan` 的返回结构。

`frontend/src/api.ts` 新增：

- `getModSyncPlan(instanceId?)` — 对应 `GET .../mods/sync-plan`（目前 `ModsPage` 没有调用这个接口，因为 `getMods()` 已经在每个 Mod 上带了 `syncKind`，没必要再多打一次请求；保留这个封装是为了和后端三接口一一对应，以后如果有单独的分类统计场景可以直接用）。
- `updateModSyncClassification(modId, syncKind, syncNote?, instanceId?)` — 对应 `PUT .../mods/:modId/sync-classification`。
- `exportModSyncPack(instanceId?)` — 对应 `POST .../mods/sync-pack/export`，blob 下载模式和现有 `exportMods()` 完全一致（解析 `Content-Disposition` 拿文件名）。

### ModsPage 改动

`frontend/src/games/stardew/pages/ModsPage.tsx`：

- `ModCard` 新增 props：`isAdmin`、`syncBusy`、`onSyncChange`。每张卡片标题下方新增一行：一个 `sd-tag`（按 `syncKind` 着色：`server_only` 蓝/`client_required` 绿/`unknown` 金）+ 管理员可见的 `<select>` 下拉框（切换会立即调用 `updateModSyncClassification`）。非管理员只看得到 tag，看不到下拉框。
- 页面级新增“玩家同步”区块（放在概览统计卡片之后、重启横幅之前）：三个统计 tag（服务器专用/玩家需同步/待确认，数字从已加载的 `mods` 数组本地算出，不额外请求）+ “导出玩家同步包”按钮。按钮在 `clientRequired === 0` 时禁用，导出中显示“导出中…”，失败显示中文错误条（复用 `sd-mods-list-error` 样式）。
- 分类编辑成功后用 `setData` 局部更新对应 Mod 的 `syncKind`（不用整页重新拉取），并调用 `dashboardData.refreshMods()` 让其他依赖 `dashboardData.mods` 的地方保持同步。
- 删除/上传等既有流程未改动。

`frontend/src/games/stardew/StardewPanel.css` 新增 4 个类，全部接在已有 `.sd-mods-*` 命名空间下：`.sd-mods-sync-summary`、`.sd-mods-sync-hint`、`.sd-mods-sync-row`、`.sd-mods-sync-select`。同步标签直接复用全局已有的 `.sd-tag` / `.sd-tag-blue` / `.sd-tag-green` / `.sd-tag-gold`（定义在 `stardew-theme.css`），没有新增标签配色体系。

### 权限边界

- 管理员：能看 tag、能用下拉框改分类、能导出。
- 普通登录用户：只能看 tag 和点导出按钮，下拉框不渲染（`isAdmin &&` 直接控制），对应后端 `PUT sync-classification` 是 `requireAdmin`、`POST sync-pack/export` 是 `requireAuth` 的权限设计。

验证：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

构建通过（`tsc -b && vite build`，无类型错误）。本次没有起开发服务器做浏览器手动验证，下一位维护者如果要看实际效果，建议：装几个测试 Mod（包含 `StardewAnxiPanel.Control`）、用管理员账号把其中一两个标成“玩家需同步”，确认导出按钮从禁用变可用，下载的 ZIP 里没有控制 Mod、但有 `player-sync-manifest.json`；再用普通账号确认看不到分类下拉框但能下载。

## 下一步注意事项

- 没有给"玩家需同步"的 Mod 数量变化做特殊提示（比如导出后是否需要重新生成同步包），目前导出永远是当下最新分类的实时快照，足够用。
- 新增 API 封装已同步 `docs/06-integration.md`；如果后续真的用上了 `getModSyncPlan`，记得在这里补一笔。
