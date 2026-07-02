# PLAYERSYNC-PACK-2 联调契约

- `POST /api/instances/:id/mods/sync-pack/export` 下载文件名仍是 `stardew-player-sync-pack.zip`，但 ZIP 内容升级为安装包：根目录含 `安装玩家同步包.bat`、`卸载本同步包.bat`、`README.txt`、`pack-manifest.json`、`checksums.sha256`、`tools/` 和 `payload/`。
- 前端无需解析 ZIP；仍按 Blob 下载即可。下游若需要读取包内容，应以 `pack-manifest.json` 为准，普通 Mod 文件在 `payload/mods/<folderName>/`，SMAPI 元数据在 `payload/smapi/smapi.json`。
- `checksums.sha256` 校验 `payload/mods` 和随包 SMAPI ZIP；玩家端脚本会在复制 Mod 前校验。若包内没有 SMAPI ZIP，`pack-manifest.json.smapi.bundled=false`，脚本继续安装 Mod 并提示玩家自行安装 SMAPI。
- 玩家端安装状态落在游戏目录 `.anxi-sync/installed.json`、`.anxi-sync/backups/`、`.anxi-sync/logs/`。卸载脚本按该记录移除本包安装的 Mod，可用 `-RestoreBackup` 恢复备份；不会默认卸载玩家已有 SMAPI。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`go test ./...`。

# NEXUS-SMAPI-THUMB-1 联调约定

- `GET /api/instances/:id/mods` 返回的虚拟 SMAPI 条目也会带 `nexusModId=2400` 和 `nexusUrl`，并在后端 Nexus GraphQL 补全成功后带 `pictureUrl`。
- 前端无需为 SMAPI 写死图片 URL；统一使用 `mods[].pictureUrl`。如果首次请求时 Nexus 不可用或未返回图片，保持现有 `NEXUS` 文字占位即可。
- 该行为不要求用户安装真实 SMAPI Mod 文件夹；它只用于面板展示和玩家同步清单语义。
- 对随 Nexus 包安装的内容包，如果其自己的缓存记录没有 `pictureUrl`，后端会按同一个 Nexus `modId` 合并主 Mod 的完整缓存；前端仍只读 `mods[].pictureUrl`，不需要自己按来源包查找图片。

# MODDEPS-1 联调约定

- `GET /api/instances/:id/mods` 的 `mods[]` 可能包含 `dependencies[]`，每项结构为 `{ "uniqueId": string, "minimumVersion"?: string, "required": boolean }`。
- 字段来源是 SMAPI `manifest.json` 的 `Dependencies` 和 `ContentPackFor`；`ContentPackFor` 统一按必需依赖返回，重复 UniqueID 会去重。
- 前端已安装 Mod 卡片只把 `required=true` 的依赖展示为“需要前置依赖：...”标签；`required=false` 的可选依赖暂不展示。
- 该字段不代表后端已验证依赖是否存在，也不代表安装接口会自动补装依赖。后续缺失依赖检查应基于同一 `dependencies[]` 与已安装 `uniqueId` 列表继续扩展。

# MODUPLOAD-2 联调约定
- `POST /api/instances/:id/mods/upload` 仍是管理员专用，服务器 running/starting 时仍返回 `409 server_running`；请求格式是 `multipart/form-data`。
- 前端可以在同一个请求里重复追加多个 `mod` 文件字段，例如 `form.append('mod', fileA)`、`form.append('mod', fileB)`；后端同时兼容字段名 `mods`，但推荐继续使用重复 `mod` 字段。
- 成功响应仍是 `ModsListResult`：`mods[]` 包含本次批量上传导入出的所有 Mod，`restartRequired` 继续遵循现有 Mod 重启语义；停服上传成功时应为 `false`，下次启动会直接加载新 Mod。前端成功后应刷新 `GET /api/instances/:id/mods` 和仪表盘缓存。
- 任意一个 ZIP 校验/解压/导入失败时，后端会回滚本请求已导入的前序 Mod，并返回错误；前端应把这次上传视为失败，不要假设部分成功。
- 单个 ZIP 内含多个顶层 SMAPI Mod 的能力仍由 `UploadModZip` 提供；“一次选择多个 ZIP”和“一个 ZIP 里多个 Mod”可以同时工作。

# NEXUS-META-1 联调约定
- `GET /api/instances/:id/mods` 可能在返回前触发一次 Nexus GraphQL v2 元数据补全：当本地 Mod manifest 有 `UpdateKeys` 中的 `Nexus:<id>` 且 sidecar 尚无缓存时，后端会无 Key 查询 Nexus 缩略图和展示字段，成功后写入 `.local-container/control/nexus-mods.json`。
- 该补全不改变接口结构，只让 `mods[]` 里的 `pictureUrl/nexusSummary/nexusUrl/downloadCount/endorsementCount/updatedAt` 更完整；Nexus 请求失败时接口仍返回 200 和本地 Mod 信息。
- `GET /api/instances/:id/mods/nexus/search?q=<数字ID>` 未配置 Nexus API Key 时也应返回 GraphQL v2 精确 ID 结果；API Key 只影响 v1 REST 查询和 Nexus 下载安装，不再是展示缩略图/元数据的前置条件。

# MODZIP-1 上传错误约定

- `POST /api/instances/:id/mods` 只接受标准 SMAPI Mod ZIP。若用户上传 Nexus 上的老式 XNB 替换包（例如只包含 `Characters/*.xnb`、`Portraits/*.xnb`，没有 `manifest.json`），后端返回 `400 invalid_mod_zip`，message 会明确提示这是 XNB 替换包，不是 SMAPI Mod，不能上传到服务器 `Mods` 目录。
- SMAPI `manifest.json` 解析兼容 UTF-8 BOM，以及字符串外的 `//` / `/* ... */` 注释和尾随逗号；这只用于 manifest 读取，不代表上传接口接受非 ZIP、非 SMAPI Mod 或 XNB 替换包。

# NEXUS-PAGED-1 联调约定
- `GET /api/instances/:id/mods/nexus/search?q=...&page=...&pageSize=...`：任意登录用户可用，返回 Nexus 专用模型 `{ query, results, page, pageSize, total, hasMore }`。
- 空 `q` 合法，用于默认热门列表；关键词搜索由后端通过 Nexus GraphQL v2 下推 `downloads DESC` 排序和 `offset/count` 分页；纯数字 ID 按 Nexus Mod ID 精确查询。
- `POST /api/instances/:id/mods/nexus/install`：管理员专用，服务器 running/starting 时返回 `409 server_running`；需要 Nexus API Key 和后端可用的文件下载权限，成功返回 `202 { "jobId": "..." }`。
- `/api/instances/:id/mods/search` 与 `/api/instances/:id/mods/search/install` 已撤下，不再作为联调契约。
- 前端创建安装任务后订阅 `GET /api/jobs/:jobId/stream`，完成后拉 `GET /api/jobs/:jobId` 并刷新 `GET /api/instances/:id/mods`。粘贴 URL / 上传文件仍作为兜底入口。

# REMOTE-MOD-1 联调约定
- `POST /api/instances/:id/mods/remote/install`：管理员专用，服务器 running/starting 时返回 `409 server_running`。请求体 `{ "url": string, "mod"?: NexusModSearchResult-like }`，成功返回 `202 { "jobId": "..." }`。
- `url` 为 `nxm://...` 时，后端解析 `modId/fileId/key/expires` 并读取 SQLite 中的 Nexus API Key 调 v1 `download_link.json?key=...&expires=...`；未配置 Key 时任务失败为 `ErrNexusAPIKeyMissing`。
- `url` 为 `https://...zip` 时，后端直接下载该 ZIP，再走现有 `UploadModZip` 校验/解压/导入；该直链来源可以是 Nexus CDN、ModDrop、GitHub、CurseForge 等公网 HTTPS ZIP。当前不支持 7z/rar。
- 前端创建任务后订阅 `GET /api/jobs/:jobId/stream`，与 `mod_nexus_install` 相同。任务成功后刷新 `GET /api/instances/:id/mods`。
- 为防止临时授权泄漏，前端和审计日志不保存粘贴 URL；失败信息不应包含完整 NXM/CDN URL。

# NEXUS-EXT-1 联调约定
- `browser-extensions/nexus-slow-installer` 是独立浏览器扩展实验包，用于免费 Nexus 用户的慢速下载链路：本地浏览器登录 Nexus -> 扩展在文件页点击/等待 `Slow download` -> 捕获浏览器下载任务中的 Nexus CDN `.zip` 临时链接 -> 调用面板 `POST /api/instances/:id/mods/remote/install`。
- 扩展第一版不新增后端接口，请求体仍是 `{ "url": string, "mod"?: { "modId": number, "name"?: string, "nexusUrl"?: string } }`；后端按 REMOTE-MOD-1 的 `.zip` 直链规则下载并安装。
- 扩展调用面板接口时使用 `credentials: "include"` 复用同浏览器中的面板管理员登录态。若正式云端部署遇到 SameSite/Cookie 或跨域策略导致无法带登录态，应新增受限的扩展配对 token，而不是让扩展保存管理员密码。
- 联调前置：面板管理员已登录、服务器已停止、Nexus 账号已登录、Nexus CDN 临时链接可由云端后端访问。测试失败时优先区分三类问题：扩展未捕获下载、面板鉴权 401/403、后端下载/导入 ZIP 失败。
- 扩展状态与日志必须脱敏 `md5/expires/user_id/key`；完整临时 URL 只作为请求体短暂发送给面板，不应落入长期文档、审计或支持包。

# NEXUS-EXT-3 联调约定

- 前端 Nexus 搜索结果的“一键安装”主路径已经切到扩展链路：点击后同页跳转到 Nexus Mod 文件页，并附加 `anxi_auto=1`。前端不再为该按钮直接调用 `POST /api/instances/:id/mods/nexus/install`，也不再要求用户配置 Nexus API Key。
- 扩展进入带 `anxi_auto=1` 的 Nexus 页面后自动打开手动下载/慢速下载流程，捕获到 Nexus CDN `.zip` 临时链接后只等待用户点击“提交到面板”。提交时复用 `POST /api/instances/:id/mods/remote/install`，成功响应仍为 `202 { "jobId": "..." }`。
- 扩展提交成功后跳回 `/instances/:id/jobs?jobId=<jobId>`；`JobsLogsPage` 应优先读取 `jobId` 查询参数并加载该任务详情和日志。若任务不在第一页列表里，右侧详情仍应通过 `GET /api/jobs/:jobId` 加载。
- 旧 `POST /mods/nexus/install` 可以保留给后续 Premium/API Key 直连或调试使用，但当前用户入口以扩展 + remote install 为准。

# NEXUS-3 联调约定

- `GET /api/instances/:id/mods/nexus/search?q=...`：无 Nexus API Key 时也应能走 GraphQL v2 搜索；纯数字 query 未配置 Key 时按 GraphQL v2 的 `gameId=1303 + modId` 精确查询，已配置 Key 时仍可按 v1 REST 精确 ID 查询。
- `POST /api/instances/:id/mods/nexus/install`：管理员专用，请求体为当前 Nexus 搜索卡片字段（至少 `modId`，建议带 `name/summary/version/pictureUrl/nexusUrl/downloadCount/endorsementCount`）。未配置 Key 返回 `503 nexus_api_key_missing`；服务器运行中返回 `409 server_running`；成功返回 `202 { jobId }`。
- 前端安装后订阅 `GET /api/jobs/:jobId/stream`，展示 `log` 事件，`finished` 后拉取 `GET /api/jobs/:jobId` 判断 succeeded/failed，并刷新 `GET /api/instances/:id/mods`。
- `GET /api/instances/:id/mods` 的 `mods[]` 现在可能包含 Nexus 卡片字段：`nexusSummary`、`pictureUrl`、`nexusUrl`、`downloadCount`、`endorsementCount`、`updatedAt`。前端可用这些字段把已安装 Mod 渲染成与搜索结果一致的卡片。
- 安装流程不新增前端直连 Nexus；所有 Nexus 文件列表、下载链接、下载 ZIP、解压安装都由后端代理和现有 Mod ZIP 安全校验完成。

# 前后端联调文档

## 联调目标

前后端联调必须验证三件事：

1. API 结构和错误码稳定。
2. 长任务、SSE、Steam Guard、邀请码刷新等异步流程可恢复。
3. UI 状态和后端实例状态一致，不误导用户。

## 本地启动

后端：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go run .\cmd\panel
```

前端：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run dev
```

访问 Vite 地址，前端开发代理应能访问后端 API。完整打包验证见 `docs/09-image-build.md`。

## 关键联调流程

### 1. 初始化与登录

- 首次访问显示管理员初始化。
- 创建管理员后自动登录。
- 登出后回到登录页。
- 错误密码显示中文错误提示。
- 未登录访问 API 返回 401，权限不足返回 403。

### 2. 安装与 Steam Auth

- `prepare` 创建实例目录和 compose/env。
- `install` 创建 job。
- 前端订阅 job SSE。
- Steam Guard / QR / 验证码阶段前端能提交输入。
- job 结束后状态刷新。
- 日志不包含 Steam/VNC 密码。

### 3. 生命周期与邀请码

- 未安装时启动返回结构化错误，前端提示“请先安装游戏”。
- 有可用存档后可启动。
- 同一实例的启动、停止、重启任务必须互斥；用户点击停止后，旧启动任务应变为 `canceled`，不能继续显示 running。
- 启动/重启提交成功后前端清空旧邀请码并等待新码。
- 后端过滤容器内旧 `/tmp/invite-code.txt`。
- 停止后前端清空邀请码。

### 4. 存档

- 上传 ZIP 先预览，不直接写正式目录。
- 用户确认后再提交导入并启动。
- 新建存档必须生成 Stardew/Junimo 可读真实存档，不能只写表单摘要。
- 删除前自动备份。
- 运行中禁止删除或覆盖危险操作。

### 5. Mod

- 上传、删除、导出可用。
- 运行中危险操作禁用。
- 上传/删除/安装 Mod 写操作要求服务器停止；停服修改后下次启动会自动加载，不再提示需要重启。`restartRequired` 只用于运行中已有 Mod 变更待应用的历史/兼容场景，服务器停止时接口应返回 `false`。
- 玩家同步分类（`syncKind`：`server_only`/`client_required`/`unknown`）随 `GET /api/instances/:id/mods` 一起返回，前端不用单独再拉一次。没有手动覆盖时，后端会自动把面板控制组件标为 `server_only`，把 SMAPI 内容包和其他第三方 Mod 标为 `client_required`，并在 `syncNote` 写入自动识别说明。
- `GET /api/instances/:id/mods/sync-plan` 返回分类统计；`PUT /api/instances/:id/mods/:modId/sync-classification` 任意登录用户可用，编辑不受运行状态限制；`POST /api/instances/:id/mods/sync-pack/export` 任何登录用户可用，运行中也允许导出，导出包含 `pack-manifest.json`、`checksums.sha256`、安装脚本和 `payload/mods/`，且永远不含面板自带的 `StardewAnxiPanel.Control`。
- 无 `client_required` Mod 时导出接口返回 `400 no_sync_mods`，前端按钮直接禁用避免命中。
- `GET /api/instances/:id/mods/nexus/search?q=关键词`：任意登录用户可用（不需要管理员权限），后端代理 Nexus Mods 官方 API，前端不直连 N站。鉴权按能力拆开：关键词搜索和无 Key 纯数字 ID 展示查询都走公开只读的 GraphQL v2，**不需要个人 API Key**；配置 Key 后纯数字 ID 可优先走 v1 REST 精确查询。只有当 Nexus 自己因鉴权拒绝 GraphQL 查询时才返回 `502 nexus_auth_required`（提示需要 OAuth/更高权限，配置 Key 不一定能解决）。空关键词作为默认热门列表返回 200；其余上游非 2xx 映射为 `404 nexus_mod_not_found` / `502 nexus_unauthorized`（v1 REST Key 无效/权限不足）/ `429 nexus_rate_limited` / `502 nexus_request_failed`，前端按 `errorMessage` 兜底显示后端返回的中文 `message`。返回结果按本地已装 Mod 的 manifest `UpdateKeys`（`Nexus:<id>`）匹配 `installed`，本阶段不做版本新旧判断。
- `GET /api/settings/nexus` / `PUT /api/settings/nexus/api-key` / `DELETE /api/settings/nexus/api-key`：管理员专用的 Nexus Key 配置接口。PUT 请求体 `{ "apiKey": string }`，保存后当前进程立即生效；GET 只返回 `{ configured, last4? }`，不会回显完整 Key；DELETE 清除配置。

### 6. 控制台命令

- 普通用户只看到允许的只读命令。
- 管理员看到完整 allowlist。
- 不允许任意 shell。
- 服务器未运行时命令禁用或返回 `server_not_running`。
- `POST /api/instances/:id/commands/say` 请求体为 `{ "message": string }`；成功返回 `CommandRunResult{ command: "say", output, exitCode: 0, durationMs }`，表示后端已把喊话命令写入控制目录。
- 喊话由 `StardewAnxiPanel.Control` 消费 `.local-container/control/commands/*.json` 后发送到游戏聊天，实际玩家可见文本前缀为 `[Panel]`。如果服务器已运行但世界尚未 ready，控制模组会在 `status.json` 记录忽略原因，后端不会暴露任意 SMAPI 命令入口。
- 前端仍应在非 running 状态禁用喊话输入；运行中发送失败时按结构化错误码展示，成功时提示“已提交/已发送”即可，不需要等待聊天回执。

### 7. 玩家页

- `GET /api/instances/:id/players` 返回在线快照和缓存名册。
- 前端显示 online/offline、host、位置、tile/pixel。
- 未知地图 key 保留原值。
- 玩家页固定展示 `money`、`farmIncome`、`personalIncome` 和 `walletMode`；`farmIncome` 是农场/团队累计收入，`personalIncome` 是玩家个人累计收入，不随钱包模式改变含义。
- `recentEvents` 返回最近玩家活动，至少覆盖首次记录、加入和离开；事件必须按 `saveId` 隔离。
- 新建/切换存档后，玩家缓存必须按 `saveId` 隔离；上一存档玩家不应出现在当前存档列表。

## API 约定

错误响应应保持结构化：

```json
{
  "error": {
    "code": "server_not_running",
    "message": "服务器未运行"
  }
}
```

前端优先使用 `code` 映射中文提示，`message` 作为兜底。

## 状态校准

面板状态机是 UI 流程来源，但 Docker 和 Junimo 是运行事实来源。后端启动和关键操作前应校准：

```text
docker compose ps
docker compose logs --tail
Junimo HTTP status（如启用）
SMAPI / control files
.env、docker-compose.yml、.local-container
active save metadata
```

典型规则：

- 面板记录 running，但 compose 显示 server 停止，应回到 stopped 或 error。
- install 完成但未选择存档，应保持 `save_required`。
- start 无有效存档，应返回 `save_required` 并引导前端到 Saves。

## 常用验证命令

后端：

```powershell
cd E:\stardew-server-anxi-panel\backend
$env:GOCACHE='E:\stardew-server-anxi-panel\.gocache'
go test ./...
```

前端：

```powershell
cd E:\stardew-server-anxi-panel\frontend
npm.cmd run build
```

冒烟：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\smoke-test.ps1
```

可选参数：

- `-SkipDocker`
- `-SkipFrontend`
- `-SkipBackend`
# SMAPI-RUNTIME-1 联调约定

- `GET /api/instances/:id/mods` 可能返回 `mods[0].builtIn=true` 的 `SMAPI` 虚拟条目；前端应把它作为置顶内置组件展示。
- `builtIn=true` 条目不是服务器 Mods 目录中的真实文件夹，不能调用删除接口，也不能调用同步分类更新接口；后端同步包导出会忽略该条目。
- 该条目的 `syncKind=client_required` 表达“玩家客户端需要先安装 SMAPI”，但它不会进入玩家同步 ZIP；前端同步统计应排除 `builtIn`，普通玩家看到的是提示而不是可下载内容。
- SMAPI 条目只有在面板内置控制 Mod `StardewAnxiPanel.Control` 已出现在实例 Mods 目录时才注入，用来避免未准备实例显示运行组件。

# MODORIGIN-1 Nexus 包来源字段

- `GET /api/instances/:id/mods` 的 `mods[]` 可能同时包含 `nexusModId` 和 `originNexusModId` 两类字段。`nexusModId` 只表示该 Mod 文件夹自己的 SMAPI `UpdateKeys` 声明；`originNexusModId` 表示它随某个 Nexus 下载包一起安装。
- 前端展示规则：`nexusModId>0` 显示为主 Nexus Mod；`nexusModId` 为空且 `originSource="nexus"` 时显示为“来源：N站包 / 随 <originModName> 安装”。不要把 `originNexusModId` 当作该内容包自己的 `nexusModId`，否则 `[CP]` 内容包会被误认为独立 N站 Mod。
- 后端会把来源包的 `pictureUrl/downloadCount/endorsementCount/updatedAt` 填到内容包卡片上，用于展示缩略图和统计；同步分类、玩家同步包导出仍按真实 Mod 文件夹处理。
- 删除是例外：`DELETE /api/instances/:id/mods/:modId` 会按来源包 bundle 删除同组真实 Mod 文件夹。前端删除确认应根据当前 `mods[]` 计算同 `nexusModId/originNexusModId` 的组成员并提示用户会一起删除；确认后只调用一次 DELETE，不要在前端循环多次删除。
# NEXUS-PAGED-1 联调契约

- 模组下载页在线搜索只调用 `GET /api/instances/:id/mods/nexus/search?q=...&page=...&pageSize=...`。
- 响应结构为 `NexusModSearchResponse{query, results, page, pageSize, total, hasMore}`；前端用 `hasMore` 控制下一页，用 `page > 1` 控制上一页。
- 关键词搜索在后端通过 Nexus GraphQL v2 下推 `downloads DESC` 排序和 `offset/count` 分页；前端不再调用 `/mods/search` 统一搜索骨架。
- Nexus 一键安装继续调用 `POST /api/instances/:id/mods/nexus/install`；管理员粘贴 Nexus `nxm://` 或 Nexus CDN 临时 ZIP 仍走 `POST /api/instances/:id/mods/remote/install`。
- `/api/instances/:id/mods/search` 与 `/api/instances/:id/mods/search/install` 已撤下，不再作为联调契约。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。

# SMAPI-SYNC-2 联调契约

- `GET /api/instances/:id/mods` 可能同时返回两个 `builtIn=true` 条目：虚拟 `SMAPI` 与真实 `StardewAnxiPanel.Control`。前端不要仅凭 `builtIn` 判断是否排除玩家同步；SMAPI 需要进入同步统计和同步清单，Control 不进入。
- `SMAPI`：`uniqueId=Pathoschild.SMAPI`、`syncKind=client_required`、`builtIn=true`。它代表玩家客户端前置要求，导出同步包时进入 `pack-manifest.json` 的 `mods[]` 和 `smapi` 元数据；只有服务端已缓存 SMAPI ZIP 时才会写入 `payload/smapi/SMAPI*.zip`。
- `StardewAnxiPanel.Control`：`folderName=StardewAnxiPanel.Control`、`builtIn=true`、`syncKind=server_only`。前端不得显示删除按钮或同步分类下拉；后端也会拒绝删除并排除同步包。
- `pack-manifest.json` 条目包含 `builtIn` 与 `packaged`：下游如果做玩家同步安装器，应只自动复制 `packaged=true` 的 Mod；`packaged=false` 的 SMAPI 是玩家前置要求，不是 Mod 文件夹。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web` 与 `npm.cmd run build`。
# PLAYERSYNC-PACK-15 联调契约

- 完整同步包接口保持不变：`POST /api/instances/:id/mods/sync-pack/export`，下载文件名 `stardew-player-sync-pack.zip`，用于首次加入玩家，可按服务端缓存情况携带 `payload/smapi/SMAPI*.zip`。
- 新增模组更新包接口：`POST /api/instances/:id/mods/sync-pack/export-update`，下载文件名 `stardew-player-mods-update-pack.zip`，用于已经运行过完整版同步包的玩家。
- 更新包 ZIP 内 `pack-manifest.json.packType=mods_update`，不包含 `payload/smapi/`，`checksums.sha256` 只校验 `payload/mods/`。安装脚本会要求玩家游戏目录已存在 `StardewModdingAPI.exe`，否则提示先运行完整版同步包。
- 前端只按 Blob 下载，不需要解析 ZIP；UI 上应把完整包和更新包区分展示。更新包没有真实可打包 Mod 时后端返回 `400 no_sync_mods`，前端按钮也应在只有虚拟 SMAPI 时禁用。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。

# PLAYERSYNC-PACK-16 联调契约

- 模组更新包 `stardew-player-mods-update-pack.zip` 的 `tools/install.ps1` 不再读取或修改 Steam 启动项，ZIP 内也不包含 `tools/steam-launch-options.ps1`。
- 更新包仍要求玩家游戏目录已有 `StardewModdingAPI.exe`；缺失时提示先运行完整版玩家同步包。
- 更新包安装完成摘要显示 `Steam 启动项：已跳过，沿用已有设置`，不会再输出 `Steam 启动项文本` 或复制提示。
- 完整同步包契约不变：首次玩家包仍会尽力自动配置 Steam 启动项，失败时输出可复制 launch options。
- 前端无需新增字段；继续区分“完整同步包”和“模组更新包”的下载按钮即可。

# MODPROFILE-1 联调契约

- `GET /api/instances/:id/mods` 返回的 `mods[]` 新增 `enabled/canToggle/enableNote`，用于展示当前激活存档下的 Mod 启用状态。
- 禁用的 Mod 仍会出现在 `GET /mods` 响应中；前端必须读取 `enabled`，不要用是否出现在列表里判断启用。
- 新增 `PUT /api/instances/:id/mods/:modId/enabled`。管理员专用、服务器 running/starting 时不可用；请求体 `{ "enabled": true|false, "saveName"?: string }`，不传 `saveName` 时使用当前激活存档。
- 新建存档和新导入存档默认只启用内置组件，第三方 Mod 需要在配置页手动开启。旧存档没有 profile 时保持当前物理目录状态。
- 启动前后端会按当前存档 profile 移动 Mod 目录，因此玩家同步包导出仍只打包当前启用目录里的玩家 Mod。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。
# MODPROFILE-2 联调契约

- `POST /api/instances/:id/saves/select` 和 `POST /api/instances/:id/saves/select-and-start` 切换存档后，后端会应用对应存档的 Mod profile；前端在收到切换成功并刷新 saves 后必须刷新 `GET /api/instances/:id/mods`，用新的 `enabled/canToggle/enableNote` 渲染当前存档状态。
- 公共数据层现在监听 `activeSaveName`，活动存档变化会触发 mods 刷新；页面不要缓存旧 `mods` 当作跨存档状态。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。

# NEXUS-DEFAULT-1 联调契约

- `GET /api/instances/:id/mods/nexus/search?q=&page=1&pageSize=20` 合法，返回 Nexus Stardew Valley 默认热门列表，不再返回 `invalid_query`。
- 空 `q` 响应结构仍是 `{ query, results, page, pageSize, total, hasMore }`，其中 `query` 为 `""`；前端用同一套 Nexus 结果卡片和分页控件展示。
- 关键词、数字 ID、安装接口契约不变。只有下载页默认态和空输入刷新热门依赖这个新行为。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。
# NEWGAME-CABINS-1 自定义新存档小屋联调契约

- `POST /api/instances/:id/saves/custom-new-game` 的 `startingCabins` 表示“初始联机小屋数量”，不是总玩家数；合法范围为 0-7。
- 前端新建存档 UI 必须直接显示并提交 `startingCabins`，不要把主玩家加 1 后当作“小屋数”展示。
- 后端会同时写 Junimo `server-settings.json` 的 `Game.StartingCabins`、控制模组 `server-init.json` 的 `cabinCount/cabinLayout`，以及 `.local-container/control/new-game-pending` 一次性标记；控制模组只在该标记存在时提前同步 Stardew 原生新建参数。
- 控制模组不再作为存档创建方；联调时应以 Junimo HTTP `POST /newgame` 和 Junimo 生成的存档目录为准，Control 只负责新建前参数同步和新建后角色定制。
- 联调验证建议：创建 0、1、2 间小屋的新存档后，分别检查 `.local-container/settings/server-settings.json`、`.local-container/control/server-init.json`、`.local-container/control/new-game-pending`，并解析生成存档 XML 中可见/有效 Cabin building 数；已有存档不会因 `server-init.json` 残留而重新应用新建参数。

# FE-QUICK-BACKUP-1 快捷备份联调契约

- 服务器控制页“备份存档”复用现有 `POST /api/instances/:id/saves/:name/backup`，其中 `name` 必须来自 `GET /api/instances/:id/saves` 的 `activeSaveName`。
- 前端仅管理员可点；无激活存档时禁用。成功响应仍为 `{ "backupName": string }`，前端只展示文件名并不解析 ZIP。
- 该操作在 UI 中显示为“备份已保存进度”，语义是“对当前磁盘存档目录打 ZIP 手动备份”，不是“强制 Stardew 立即保存当前游戏内世界”。游戏内未触发保存事件的进度不会因为备份按钮自动写入主存档。
- 手动验证：运行中和停服状态分别点击快捷备份，确认能创建手动备份；随后进入存档页备份列表应能看到同一备份文件。


# SAVE-BACKUP-POLICY-1 ????

- ??????????SMAPI Control `Saved` ?? -> ? `.local-container/control/save-events/*.json` -> ?? `GET /saves/backups` ???????? -> ?? latest ????/???? daily ???
- ??????????????? -> `PUT /api/instances/:id/saves/backups/policy` -> ??? `.local-container/backups/saves/policy.json`?????????????????
- ???????????/?????????????????????? scheduler????????????????????????????
- ????????????????????????/??????????????????????????

# SAVE-BACKUP-SCHEDULE-HOUR-1 联调契约

- `GET|PUT /api/instances/:id/saves/backups/policy` 的定时备份字段使用 `scheduledHour`，取值 0-23，前端按 24 小时制展示为 `HH:00`。
- 后端仍能读取旧 `scheduledIntervalHours` 配置文件，但新响应和新保存结果以 `scheduledHour` 为准，不再要求前端提供时间间隔。
- 定时备份语义：每天到达配置整点后，下一次触发备份维护时覆盖同一份 `scheduled_<save>.zip`；同一自然日不会重复生成。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。
# SCHEDULED-RESTART-1 计划重启联调契约

- `GET /api/instances/:id/restart-schedule` 返回 `{ schedule }`，字段包括 `enabled`、`shutdownTime`、`startupTime`、`timezone`、`warningMinutes`、`backupBeforeShutdown`、`skipIfPlayersOnline`、`nextShutdownAt`、`nextStartupAt`、`lastStatus`、`lastMessage`。
- `PUT /api/instances/:id/restart-schedule` 仅管理员可用，请求体使用同一组配置字段；时间格式为 `HH:MM`，时区默认 `Asia/Shanghai`。
- 后端后台调度器每 30 秒检查启用计划。关闭时间前通过现有喊话通道写 `.local-container/control/commands/*.json`；关闭时可调用现有存档备份能力，再提交 `Stop` 生命周期 job；开启时提交 `Start` 生命周期 job。
- 关闭前备份语义与快捷备份一致：只备份当前已经落盘的 active save，不强制保存游戏内尚未写盘的进度。
- 联调建议：把关闭时间设置到当前时间后 1-2 分钟，确认弹窗保存后返回 `nextShutdownAt`；服务器运行中确认提醒文件写入 control commands；到点后任务中心出现 stop job。开启时间设置到停止后 1-2 分钟，确认 start job 被提交。
# MODDEPS-2 联调契约

- `GET /api/instances/:id/mods` 的 `mods[].dependencies[]` 结构扩展为 `{ uniqueId, minimumVersion?, required, installed, enabled, installedVersion?, satisfied, status? }`。前端应以 `satisfied=false` 或 `status` 判断展示诊断，不要再只把它当作纯 manifest 声明。
- 常见 `status`：`satisfied`、`missing`、`disabled`、`version_mismatch`、`unknown_version`；可选依赖对应 `optional_missing`、`optional_disabled`、`optional_version_mismatch`、`optional_unknown_version`。可选依赖缺失默认不算硬失败。
- `GET /api/instances/:id/mods/nexus/search` 的 `results[]` 新增 `installedEnabled`。当 `installed=true` 且 `installedEnabled=false` 时，表示该 Nexus Mod 已在服务器安装，但当前激活存档没有启用；前端应提示“已安装但未启用”，并禁止重复安装。
- 搜索的安装匹配按当前激活存档计算。后端会读取 `GetActiveSaveName(dataDir)` 并用 `ListModsWithState` 合并 active/disabled 目录，确保禁用目录里的 Mod 仍能被 Nexus ID 匹配到。
- 验证：`cd backend; go test ./...`、`cd frontend; npm.cmd run build`。

# MODREL-1 联调契约

- `PUT /api/instances/:id/mods/:modId/sync-classification` 响应从单个 `{ folderName, syncKind, syncNote }` 升级为 `{ mods, syncKind }`。`mods[]` 是本次按依赖/同包关系被同步分类影响的 Mod，前端必须按返回列表批量更新。
- 同步分类没有方向性：设置 `client_required`、`server_only` 或 `unknown` 时，都包含同 Nexus 包成员、所有已安装必需前置依赖、前置的前置，以及依赖它的已安装下游。这样用户先点“待确认”再切回其它标签时，后置 Mod 不会停留在旧状态。
- `PUT /api/instances/:id/mods/:modId/enabled` 响应从单个 `{ folderName, enabled, saveName }` 升级为 `{ mods, enabled, saveName }`。启用时包含同包成员和必需前置，禁用时包含同包成员和依赖它的下游。
- 共享前置不随某个业务包禁用：例如启用 `[CP] Multiple Construction Orders` 会启用 `Multiple Construction Orders` 和 `Content Patcher`；禁用 `Multiple Construction Orders` 会禁用同包 `[CP]`，但不会禁用 `Content Patcher`，因为它可能仍被其他 Mod 使用。
- 前端不要自行复刻关系图算法；以后联动规则调整时以后端返回的 `mods[]` 为准。
- 验证：`cd backend; go test ./...`、`cd frontend; npm.cmd run build`。
# NEXUS-EXT-2 安装完成可见性与日志

- `mod_remote_install` / `mod_nexus_install` 新任务的安装进度日志应显示正常中文；旧任务历史日志如果已经以乱码入库，不做迁移。
- 前端订阅安装 job 的 `finished` 事件后，成功状态会切到“添加模组”页并刷新 `GET /mods`。后端会把本次导入的 Mod 标记为当前激活存档启用；联调时如果任务已完成但页面没看到，应先确认是否刷新到了添加页，以及 `mods/` / `mods-disabled/` 目录和当前存档 profile。
- 验证：`go test ./internal/games/stardew_junimo ./internal/web`、`npm.cmd run build`。
# NEXUS-REQ-1 联调约定

- `GET /api/instances/:id/mods/nexus/search` 的 `results[]` 可能包含 `requiredMods[]`。字段来自 Nexus GraphQL 的 `modRequirements.nexusRequirements`，用于前端搜索卡片提示缺少的 Nexus 前置 Mod。
- `requiredMods[]` 每项包含 `modId/name/notes/nexusUrl/installed/installedEnabled/installedFolderName/installedVersion`。前端可把 `installed=false` 的项渲染为“安装前置”，并跳转该前置 Mod 的 Nexus 文件页。
- 前端主 Mod 与前置 Mod 的扩展安装入口都统一追加 `tab=files&anxi_auto=1`；扩展捕获 ZIP 后仍调用 `POST /api/instances/:id/mods/remote/install`。
- Nexus 页面出现 “Additional files required” 弹窗时，扩展应自动点击弹窗内 `Download` 按钮继续，不要求用户手点。该动作只发生在扩展已开始捕获的上下文里。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`、`cd frontend; npm.cmd run build`、扩展脚本 `node --check`。
