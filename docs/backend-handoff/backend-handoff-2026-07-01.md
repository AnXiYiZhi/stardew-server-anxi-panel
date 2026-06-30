# 后端接手文档 2026-07-01

## 本次改动：Mod 管理第二阶段——Nexus Mods 只读搜索

在不绕过 `stardew_junimo` driver、不让前端直连 N站的前提下，给 Mod 管理加上 Nexus Mods 在线只读搜索：管理员和普通登录用户都能搜、看基础信息、跳转 N站；不做下载/安装。

### 前置修复确认

第一阶段遗留的两个问题在本次开工前确认**已经修复**（属于上一轮代码评审修复，见本文件下方"代码评审修复"小节，不是本次新改的）：

1. `ExportModSyncPackZip` 玩家同步包导出已用 `os.CreateTemp` 生成唯一临时文件名，不再固定写 `%TEMP%\stardew-player-sync-pack.zip`。
2. `SetModSyncClassification` 已加 `dataDir` 维度的 `sync.Mutex`，`saveModSyncStore` 已改成临时文件 + `os.Rename` 原子写入。

本次没有再改这两处。

### 配置

新增环境变量 `NEXUS_API_KEY`（在 nexusmods.com 账号设置的 "API Access" 页面生成个人 API Key）。不接入 `internal/config`，直接在 `nexus.go` 的 `nexusAPIKey()` 里 `os.Getenv` 读取（每次调用都读，不是进程启动时缓存一份），这样测试可以自由用 `t.Setenv`，也避免给一个纯只读、可选的外部集成增加全局配置结构体的负担。未配置时 `SearchNexusMods` 直接返回哨兵错误 `ErrNexusAPIKeyMissing`，不会发出任何 HTTP 请求。

### Nexus 官方 API 能力边界（重要，决定了实现方式）

开工前查证过：Nexus Mods 官方 v1 REST API（`api-docs.nexusmods.com`，文档化、`apikey` 请求头鉴权）**没有任何关键词全文搜索接口**——只有按 ID 精确查询单个 Mod（`/v1/games/{domain}/mods/{id}.json`）、最近更新列表（无 name/summary 等字段）、MD5 查询等。Nexus 官方维护的 `node-nexus-api` 客户端库文档也明确没有暴露搜索方法。

因此 `SearchNexusMods` 按查询内容分两条路径：

- **纯数字查询** → 走已验证、文档化的 v1 REST 精确 ID 查询：`GET https://api.nexusmods.com/v1/games/stardewvalley/mods/{id}.json`。这条路径可信度高，字段名对照官方文档。
- **其余关键词** → 走 GraphQL v2（`https://api.nexusmods.com/v2/graphql`，和 nexusmods.com 网站搜索框同源）。**这条路径未用真实 API Key 验证过**——`nexusGraphQLSearchQuery` 和 `nexusGraphQLResponse`（`nexus.go`）里的字段名是根据公开资料（搜索结果提到的 `legacyModsByDomain`/`ModsListing` 等 GraphQL 操作名）推测的。如果接入真实 Key 后关键词搜索解析失败或返回空，第一步应该是对照 Nexus 最新 GraphQL schema 核对这两处，而不是怀疑其他逻辑。

两条路径共享同一套请求基础设施（超时、User-Agent、错误映射、API Key 不泄露处理），改起来互不影响。

### 实现

新文件 `backend/internal/games/stardew_junimo/nexus.go`：

- `SearchNexusMods(ctx, query) (NexusModSearchResponse, error)`：检查 API Key → trim 校验 query → 按数字/关键词分流 → 结果裁剪到 `nexusMaxResults = 20`。
- `nexusGetModByID` / `nexusSearchByKeyword`：分别对应上面两条路径，都通过 `doNexusRequest` 发起请求。
- `doNexusRequest`：统一处理超时（10s，`nexusHTTPClient`，可在测试里换底层 endpoint）、非 2xx 映射。**非 2xx 响应体永远不会被读取转发**——只丢弃 body、保留状态码包成 `*NexusAPIError{StatusCode}`，这样即使上游返回的错误页里意外回显了请求细节（比如把 apikey 错误地塞进错误文案），也不会经我们的错误链路泄露出去；apikey 本身只通过请求头发送，从来不出现在 URL 或 body 里。
- `ApplyNexusInstalledMatch(dataDir, results)`：读本地 `ListMods`，按 `NexusModID` 建 map，给搜索结果打 `installed`/`installedFolderName`/`installedVersion`。本阶段只判断"是否已装"，不比较版本新旧。
- `nexusV1BaseURL`/`nexusGraphQLURL` 是包级 `var` 而非 `const`，专门留给测试用 `httptest.Server` 替换。

### manifest 解析扩展

`mods.go`：`modManifest` 新增 `UpdateKeys []string`（对应 SMAPI manifest.json 的 `UpdateKeys` 字段）；新增 `parseNexusModIDFromUpdateKeys(keys []string) (int, bool)`，从形如 `"Nexus:2400"`（大小写不敏感站点名，允许 `Nexus:2400:subkey` 这种带子 key 的变体）的条目里挑出数字 ID。`readModInfo` 解析时把 `UpdateKeys` 和解析出的 `NexusModID` 一起填进 `registry.ModInfo`。

`registry/types.go`：`ModInfo` 新增 `UpdateKeys []string` 和 `NexusModID int`（都是 `omitempty`），所以 `GET /api/instances/:id/mods` 现有响应会自动带上这两个新字段，前端旧代码不受影响。

### API

新增 1 个接口，路由加在 `backend/internal/web/instance_handlers.go`（`mods/export` 之后），handler 在 `backend/internal/web/lifecycle_handlers.go`：

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/instances/:id/mods/nexus/search?q=关键词` | `requireAuth`（任意登录用户） | 返回 `NexusModSearchResponse{query, results}` |

`q` 在 handler 层先 `strings.TrimSpace`，空字符串直接 `400 invalid_query`，不会调用到 `sj.SearchNexusMods`（`sj` 内部也有同样的校验作为第二道防线，因为这是一个公开的包函数，不能假设调用方都先 trim 过）。

错误映射（`writeNexusError`）：

| 场景 | HTTP | code |
| --- | --- | --- |
| 未配置 `NEXUS_API_KEY` | 503 | `nexus_api_key_missing` |
| 空查询 | 400 | `invalid_query` |
| Nexus 404（ID 查询未命中） | 404 | `nexus_mod_not_found` |
| Nexus 401/403 | 502 | `nexus_unauthorized` |
| Nexus 429 | 429 | `nexus_rate_limited` |
| 其他非 2xx / 网络错误 | 502 | `nexus_request_failed` |

### 测试

新文件 `backend/internal/games/stardew_junimo/nexus_test.go`，全部用 `httptest.Server` mock，不依赖真实网络/真实 Key：

- API Key 缺失 → `ErrNexusAPIKeyMissing`。
- 空 query（含纯空格）→ `ErrInvalidNexusQuery`。
- ID 查询结果解析（mock v1 REST 响应，校验字段映射、`NexusURL` 拼接）。
- 关键词搜索结果解析（mock GraphQL 响应，多条结果）。
- 结果数量裁剪到 `nexusMaxResults`。
- 非 2xx 状态码映射为 `*NexusAPIError`（ID 查询和关键词搜索两条路径都覆盖）。
- **API Key 不泄露**：mock 一个把 Key 回显进错误响应体的"有 bug 的上游"，断言最终返回给调用方的 `error.Error()` 不包含该 Key（同时验证请求确实带上了 `apikey` 头，证明鉴权本身没问题，只是错误路径不传播 body）。
- `parseNexusModIDFromUpdateKeys` 各种格式（大小写、带子 key、混合多个 UpdateKeys、纯非 Nexus、空、非数字 ID）。
- `ApplyNexusInstalledMatch` 按 `Nexus:ID` 命中/不命中、没有任何已装 Mod 时不报错。
- `readModInfo` 正确把 `UpdateKeys`/`NexusModID` 填进 `registry.ModInfo`。

验证：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
```

全部通过（含既有用例）。

### 下一步注意事项

- **最高优先级**：拿到真实 `NEXUS_API_KEY` 后第一时间手动验证关键词搜索（GraphQL v2 路径）。ID 精确查询（v1 REST）的字段名是对照官方文档写的，可信度高；GraphQL 这条路径的查询字符串和响应结构是推测的，没有用真实账号跑通过。
- 不涉及 SMAPI mod DLL 或 Junimo 容器内文件，**不需要重启 Stardew server 容器**。
- 本阶段是纯只读搜索，没有下载/安装/校验文件完整性的逻辑；如果以后要做"从 Nexus 直接安装"，注意 Nexus 的文件下载链接通常需要 premium 账号或走 OAuth/SSO 流程，不能直接复用本阶段只读搜索用的个人 `apikey`。
- `installed` 匹配完全依赖 Mod 自己 manifest.json 里声明的 `UpdateKeys`；没声明 `Nexus:<id>` 的已装 Mod（很多老 Mod 或本地自制 Mod 都没有）永远不会被标记为已安装，这是预期行为，不是 bug。
- 新增接口已同步更新 `docs/02-backend.md`、`docs/06-integration.md`、`docs/08-future-roadmap.md`、`docs/09-image-build.md`（`NEXUS_API_KEY` 环境变量表）。

## 本次改动：Mod 玩家同步包（第一阶段）

在不绕过 `stardew_junimo` driver 的前提下，给 Mod 管理加上“同步分类”能力，让管理员能标出哪些 Mod 玩家必须在客户端同步安装，并一键导出对应 ZIP。

### 数据结构

- `registry.ModInfo` 新增 `SyncKind string`（`json:"syncKind"`，恒返回）和 `SyncNote string`（`json:"syncNote,omitempty"`）。
- 新增常量 `ModSyncKindServerOnly` / `ModSyncKindClientRequired` / `ModSyncKindUnknown` 和校验函数 `ValidModSyncKind`。
- 新增 `ModSyncSummary`（total/serverOnly/clientRequired/unknown）和 `ModSyncPlanResult`（mods + summary）。
- 涉及文件：`backend/internal/games/registry/types.go`。

### 持久化

新文件 `backend/internal/games/stardew_junimo/mod_sync.go`：

- 分类数据落在面板自有文件 `<dataDir>/.local-container/control/mod-sync.json`，格式 `{"mods": {"<folderName>": {"syncKind": "...", "syncNote": "..."}}}`。**绝不**写入 Mod 自己的 `manifest.json`。
- `defaultModSyncKind(folderName)`：`StardewAnxiPanel.Control` 默认为 `server_only`，其余默认 `unknown`。
- `GetModSyncClassification` / `SetModSyncClassification`：单个 Mod 的读写。存储里出现非法 `syncKind`（比如手动改坏文件）会被忽略，回退到默认值，不会让接口 500。
- `ApplyModSyncClassification(dataDir, mods []registry.ModInfo) []registry.ModInfo`：批量补全分类，`handleModsList`（`GET /api/instances/:id/mods`）已经在返回前调用它，所以**前端不需要为了拿 syncKind 单独再请求一次**。
- `BuildModSyncPlan(dataDir)`：给 `sync-plan` 接口用，顺带算出统计。
- `ResolveModFolder(dataDir, modID)`：modID 可以是文件夹名或 manifest `UniqueID`，查找顺序复用了 `DeleteMod` 的逻辑。
- `ExportModSyncPackZip(dataDir)`：

  1. 取全部已装 Mod 并补全分类。
  2. 过滤出 `syncKind == client_required` 且文件夹名不是 `StardewAnxiPanel.Control`（双重保险：默认分类是 server_only，这里又强制再排除一次，哪怕有人手贱把控制 Mod 标成 client_required 也导不出去）。
  3. 一个都没有时返回哨兵错误 `ErrNoSyncMods`，handler 据此返回 `400 no_sync_mods`，不走会吞掉错误文案的 `sanitizeErrorMsg`。
  4. 复用 `mods.go` 里新抽出来的 `addModDirToZip(w, root, dirName)` 写入每个 Mod 目录（这个函数是从原 `ExportModsZip` 里抽出来的，行为完全一致，`ExportModsZip` 调用方式不变）。
  5. 额外写一个 `player-sync-manifest.json` 到 ZIP 根目录：`{exportedAt, mods: [{uniqueId, name, version, folderName, syncKind}]}`。

### API

新增 3 个接口，路由加在 `backend/internal/web/instance_handlers.go`（`mods/export` 之后、`DELETE mods/:modId` 之前），handler 实现在 `backend/internal/web/lifecycle_handlers.go`：

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/instances/:id/mods/sync-plan` | `requireAuth`（任意登录用户） | 返回 `ModSyncPlanResult` |
| PUT | `/api/instances/:id/mods/:modId/sync-classification` | `requireAdmin` | body `{syncKind, syncNote?}`；只写面板元数据，**不受服务器运行状态限制**；成功会写审计日志 `mod_sync_classification_update` |
| POST | `/api/instances/:id/mods/sync-pack/export` | `requireAuth`（任意登录用户，玩家也要能下载） | 流式返回 ZIP；**服务器运行中也允许导出** |

`PUT` 接口的鉴权是 `requireAdmin` 而不是更细的"写权限"，对齐前端需求"普通用户只能查看分类和下载，不能修改"。`GET sync-plan` 和 `POST sync-pack/export` 都用 `requireAuth`，和现有 `handleModsExport` 一个级别。

### 测试

新文件 `backend/internal/games/stardew_junimo/mod_sync_test.go`，覆盖：

- 分类文件读写往返（`SetModSyncClassification` → `GetModSyncClassification`，确认落盘路径在 `.local-container/control/mod-sync.json`，且不改 `manifest.json`）。
- 默认分类：普通 Mod 默认 `unknown`，`StardewAnxiPanel.Control` 默认 `server_only`；存储里出现非法值时也回退默认。
- `ResolveModFolder` 按文件夹名/UniqueID 查找，以及拒绝 `../evil`、`foo/bar`、绝对路径等路径穿越输入。
- `ExportModSyncPackZip`：只含 `client_required` Mod、强制排除控制 Mod（即使手动标成 client_required）、无可导出 Mod 时报错、manifest 内容正确、ZIP 内条目不含绝对路径。

验证：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
```

全部通过（含既有用例，确认新增的恒定 `syncKind` 字段没有破坏旧断言）。

## 已修复的遗留问题

`mods.go` 的 `ExportModsZip` 原先在写 ZIP 的循环失败、但后续 `w.Close()` 恰好成功时，会因为 `if err := w.Close(); err != nil` 这种局部变量遮蔽外层 `err`，把循环里的真实失败吞掉、错误地返回成功。本次已修复：循环内失败直接 `return`，`w.Close()`/`zf.Close()` 改成赋值给外层 `err`（不再用 `:=` 遮蔽），失败时统一走函数顶部的 `defer` 清理临时文件。修复后行为与新写的 `ExportModSyncPackZip` 一致。`go test ./...` 全量通过。

## 代码评审修复（同一天补充）

评审发现 `mod_sync.go` 两处并发问题，已修复：

- **P1：`ExportModSyncPackZip` 临时文件名固定**（原 line 218）。原来固定写 `%TEMP%\stardew-player-sync-pack.zip`，两个用户同时导出会互相覆盖/截断 ZIP，一个请求失败时的 `defer` 清理还可能删掉另一个请求正在 `ServeFile` 的文件。改为 `os.CreateTemp("", "stardew-player-sync-pack-*.zip")` 拿到唯一磁盘路径；新增导出常量 `PlayerSyncPackFileName = "stardew-player-sync-pack.zip"`，`handleModSyncPackExport`（`lifecycle_handlers.go`）的 `Content-Disposition` 改用这个固定常量而不是 `filepath.Base(zipPath)`，所以浏览器侧下载文件名依旧稳定、可预期，跟磁盘上的随机文件名完全解耦。
- **P2：`SetModSyncClassification` 无锁读改写**（原 line 99）。两个几乎同时的 `PUT sync-classification` 请求都先读到旧的 `mod-sync.json`，后写入的会覆盖先写入的改动（前端只在 `ModsPage.tsx` 里禁用了正在操作的那一个下拉框，快速点击或两个管理员同时操作都能触发）。修复：新增按 `dataDir` 维度的锁注册表 `modSyncLocks`/`modSyncLockFor`（`map[string]*sync.Mutex` + 一把保护该 map 的 `modSyncLocksMu`），`SetModSyncClassification` 在整个 load→modify→save 过程中持有对应 `dataDir` 的锁；同时把 `saveModSyncStore` 从直接 `os.WriteFile` 改成「写同目录临时文件 `.mod-sync-*.json.tmp` → `os.Rename` 覆盖目标路径」的原子写入，避免进程中途崩溃留下半截 JSON。

涉及文件：`backend/internal/games/stardew_junimo/mod_sync.go`、`backend/internal/web/lifecycle_handlers.go`。

验证：`go build ./...`、`go vet ./...`、`go test ./...` 全量通过（含既有 `mod_sync_test.go` 用例）。

不涉及 SMAPI mod DLL 或 Junimo 容器内文件，不需要重启 Stardew server 容器。

## 下一步注意事项

- 这次改动只涉及面板元数据和 ZIP 导出逻辑，不碰 SMAPI mod DLL 或 Junimo 容器内任何文件，**不需要重启 Stardew server 容器**。
- 新增 Mod 时分类默认 `unknown`，需要管理员手动标注，目前没有任何自动推断逻辑（比如按 manifest 关键字猜测）——如果以后要做，应该还是落在 `stardew_junimo` 里。
- 新增接口已同步更新 `docs/02-backend.md` 和 `docs/06-integration.md`。
