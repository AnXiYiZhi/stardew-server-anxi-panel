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
- 上传/删除后提示需要重启。
- 玩家同步分类（`syncKind`：`server_only`/`client_required`/`unknown`）随 `GET /api/instances/:id/mods` 一起返回，前端不用单独再拉一次。
- `GET /api/instances/:id/mods/sync-plan` 返回分类统计；`PUT /api/instances/:id/mods/:modId/sync-classification` 管理员专用，编辑不受运行状态限制；`POST /api/instances/:id/mods/sync-pack/export` 任何登录用户可用，运行中也允许导出，导出包含 `player-sync-manifest.json`，且永远不含面板自带的 `StardewAnxiPanel.Control`。
- 无 `client_required` Mod 时导出接口返回 `400 no_sync_mods`，前端按钮直接禁用避免命中。
- `GET /api/instances/:id/mods/nexus/search?q=关键词`：任意登录用户可用（不需要管理员权限），后端代理 Nexus Mods 官方 API，前端不直连 N站。未配置 `NEXUS_API_KEY` 返回 `503 nexus_api_key_missing`；空关键词返回 `400 invalid_query`；上游非 2xx 映射为 `404 nexus_mod_not_found` / `502 nexus_unauthorized` / `429 nexus_rate_limited` / `502 nexus_request_failed`，前端按 `errorMessage` 兜底显示后端返回的中文 `message`。返回结果按本地已装 Mod 的 manifest `UpdateKeys`（`Nexus:<id>`）匹配 `installed`，本阶段不做版本新旧判断、不提供安装入口。

### 6. 控制台命令

- 普通用户只看到允许的只读命令。
- 管理员看到完整 allowlist。
- 不允许任意 shell。
- 服务器未运行时命令禁用或返回 `server_not_running`。

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
