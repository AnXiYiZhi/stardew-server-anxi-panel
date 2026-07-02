# JunimoServer REST API 接口整理

整理时间：2026-07-02

来源项目：`E:\codex\junimo-server-upstream`

主要参考：

- `mod/JunimoServer/Services/Api/ApiService.cs`
- `mod/JunimoServer/Services/Api/ApiService.TestEndpoints.cs`
- `docs/developers/api/introduction.md`

本文档整理的是主 `server` 容器通过 `API_PORT` 暴露的 HTTP API，默认地址为：

```text
http://localhost:8080
```

`steam-auth` 服务也有内部 HTTP 接口，但在默认 `docker-compose.yml` 中只通过容器网络 `expose` 给其他容器使用，没有发布到宿主机端口，因此不纳入本面板对接范围。

## 1. 基础配置

| 配置项 | 默认值 | 用途 |
| --- | --- | --- |
| `API_ENABLED` | `true` | 是否启用 HTTP API |
| `API_PORT` | `8080` | API 监听端口 |
| `API_KEY` | 空 | API 鉴权密钥。为空时不启用鉴权 |

启用 `API_KEY` 后，受保护接口需要携带：

```http
Authorization: Bearer <API_KEY>
```

示例：

```bash
curl "http://localhost:8080/status" \
  -H "Authorization: Bearer your-api-key"
```

## 2. 鉴权规则

代码中实际免鉴权的 HTTP 路径如下：

| 路径 | 用途 |
| --- | --- |
| `/health` | 健康检查 |
| `/wait/health` | 健康状态长轮询 |
| `/stats` | 性能状态 |
| `/docs` | Scalar API 文档页面 |
| `/swagger/v1/swagger.json` | OpenAPI JSON |
| `/diagnostics/state` | 游戏引擎诊断快照 |

其他生产接口在 `API_KEY` 设置后都需要 `Authorization: Bearer <API_KEY>`。

WebSocket `/ws` 不走普通 HTTP header 鉴权；如果设置了 `API_KEY`，客户端需要在连接后 10 秒内发送 `auth` 消息。

## 3. 生产 REST 接口

### 3.1 服务器状态

| 方法 | 路径 | 参数 | 用途 |
| --- | --- | --- | --- |
| `GET` | `/status` | 无 | 获取服务器状态和游戏快照 |
| `GET` | `/health` | 无 | 健康检查，判断游戏线程是否卡死 |
| `GET` | `/stats` | 无 | 获取性能统计 |
| `GET` | `/rendering` | 无 | 获取服务端渲染 FPS |
| `POST` | `/rendering` | query: `fps` | 设置服务端渲染 FPS，`0` 表示关闭 |
| `GET` | `/screenshot` | 无 | 截取当前游戏画面，返回 base64 PNG |
| `POST` | `/time` | query: `value` | 设置游戏时间，范围 `600` 到 `2600` |
| `POST` | `/clock-speed` | query: `multiplier` | 设置游戏时钟速度倍数 |
| `POST` | `/newgame` | JSON body | 创建新游戏 |
| `POST` | `/reload` | 无 | 重读配置并重载当前世界 |

#### `GET /status`

返回服务器和游戏快照，适合面板首页状态卡片。

主要字段：

| 字段 | 含义 |
| --- | --- |
| `playerCount` | 当前玩家数 |
| `maxPlayers` | 最大玩家数 |
| `steamInviteCode` | Steam 邀请码，可能为空 |
| `gogInviteCode` | GOG/LAN 邀请码，可能为空 |
| `serverVersion` | JunimoServer 版本 |
| `isOnline` | 服务器是否在线 |
| `isReady` | 游戏是否准备完成 |
| `lastUpdated` | 快照更新时间 |
| `farmName` | 农场名称 |
| `day` | 日期 |
| `season` | 季节 |
| `year` | 年份 |
| `timeOfDay` | 游戏时间，如 `1200` |
| `farmTypeKey` | 农场类型 key |
| `isPaused` | 是否暂停 |
| `version` | 快照版本号，可用于长轮询 |

#### `GET /health`

用于 Docker healthcheck、面板健康状态、自动恢复判断。

主要字段：

| 字段 | 含义 |
| --- | --- |
| `status` | `ok` 或 `degraded` |
| `timestamp` | 检查时间 |
| `lastTickMs` | 距离最近一次游戏 tick 的毫秒数 |
| `pendingActions` | 待执行的游戏线程动作数 |
| `gameAvailable` | 游戏服务是否可用，可能为空 |
| `tickCount` | 累计 tick 数 |
| `isFrozen` | 是否判定为卡死 |

当 `lastTickMs` 超过约 5 秒，`status` 会变为 `degraded`，`isFrozen` 为 `true`。

#### `GET /stats`

用于诊断页或性能卡片。

主要字段：

| 字段 | 含义 |
| --- | --- |
| `fps` | 当前 FPS |
| `tps` | 当前 TPS |
| `targetTps` | 目标 TPS |
| `avgTickMs` | 平均 tick 耗时 |
| `memoryMb` | 托管内存占用 |
| `gcGen0` / `gcGen1` / `gcGen2` | GC 次数 |
| `pendingActions` | 待执行动作数 |
| `gameThreadWaitMs` | 平均游戏线程等待耗时 |

#### `POST /rendering`

设置服务端画面渲染率。

参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `fps` | integer | 是 | 非负整数。`0` 关闭渲染 |

示例：

```bash
curl -X POST "http://localhost:8080/rendering?fps=15" \
  -H "Authorization: Bearer $API_KEY"
```

#### `POST /time`

设置游戏内时间。

参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `value` | integer | 是 | `600` 到 `2600`，如 `1200` 表示中午 |

服务器未准备好时会返回 `success=false`。

#### `POST /clock-speed`

调整游戏时钟速度。

参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `multiplier` | number | 是 | 大于 0。`1` 恢复默认，`10` 表示 10 倍速 |

#### `POST /newgame`

创建新游戏。接口会检查是否有客户端连接；如果有玩家在线，会返回 `409`。

请求 body 所有字段均可选，未传则使用当前配置默认值：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `farmType` | enum/string | 农场类型 |
| `farmName` | string | 农场名称 |
| `startingCabins` | integer | 初始 cabin 数 |
| `cabinStrategy` | string | cabin 策略 |
| `maxPlayers` | integer | 最大玩家数 |
| `allowIpConnections` | boolean | 是否允许 IP 连接。模型中存在，当前处理逻辑未实际使用 |
| `profitMargin` | number | 利润倍率 |
| `separateWallets` | boolean | 是否分开钱包 |

可能状态码：

| 状态码 | 含义 |
| --- | --- |
| `200` | 新游戏创建成功或业务失败响应 |
| `409` | 当前有玩家连接，拒绝创建 |
| `503` | GameManager 未初始化 |
| `504` | 创建新游戏超时 |
| `500` | 创建过程异常 |

#### `POST /reload`

重读 `server-settings.json` 并重载当前世界，不重启容器。接口会检查是否有客户端连接；如果有玩家在线，会返回 `409`。

可能状态码：

| 状态码 | 含义 |
| --- | --- |
| `200` | 重载成功或业务失败响应 |
| `409` | 当前有玩家连接，拒绝重载 |
| `503` | 无法确认玩家数，或 GameManager 未初始化 |
| `504` | 重载超时 |
| `500` | 重载过程异常 |

### 3.2 玩家与邀请码

| 方法 | 路径 | 参数 | 用途 |
| --- | --- | --- | --- |
| `GET` | `/players` | 无 | 获取当前连接玩家 |
| `GET` | `/invite-code` | 无 | 获取当前邀请码 |
| `POST` | `/roles/admin` | query: `name` 或 `playerId` | 授予玩家 admin 角色 |

#### `GET /players`

返回：

| 字段 | 含义 |
| --- | --- |
| `players` | 玩家列表 |
| `version` | 快照版本 |

玩家字段：

| 字段 | 含义 |
| --- | --- |
| `id` | `UniqueMultiplayerID` |
| `name` | 玩家名 |
| `isOnline` | 是否在线 |

#### `GET /invite-code`

返回当前邀请码。

主要字段：

| 字段 | 含义 |
| --- | --- |
| `inviteCode` | 邀请码，可能为空 |
| `error` | 无邀请码时返回错误说明 |

#### `POST /roles/admin`

给玩家授予 admin 角色。

参数二选一，不能同时传：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `name` | string | 玩家名，不区分大小写 |
| `playerId` | long | 玩家 `UniqueMultiplayerID` |

示例：

```bash
curl -X POST "http://localhost:8080/roles/admin?playerId=620826087702429092" \
  -H "Authorization: Bearer $API_KEY"
```

### 3.3 Farmhand

| 方法 | 路径 | 参数 | 用途 |
| --- | --- | --- | --- |
| `GET` | `/farmhands` | 无 | 获取所有 farmhand 槽位 |
| `DELETE` | `/farmhands` | query: `name` 或 `playerId` | 删除离线 farmhand |

#### `GET /farmhands`

返回：

| 字段 | 含义 |
| --- | --- |
| `farmhands` | farmhand 列表 |
| `version` | 快照版本 |

farmhand 字段：

| 字段 | 含义 |
| --- | --- |
| `id` | `UniqueMultiplayerID` |
| `name` | farmhand 名称 |
| `isCustomized` | 是否已经被玩家自定义/认领 |

#### `DELETE /farmhands`

删除 farmhand。目标必须离线，保存过程中不能删除。

参数二选一，不能同时传：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `name` | string | farmhand 名称 |
| `playerId` | long | farmhand `UniqueMultiplayerID` |

成功后会：

- 删除对应 farmhand 数据。
- 若存在对应 cabin，则销毁 cabin。
- 清理 cabin manager 中的历史追踪数据。
- 触发 cabin 数量补齐逻辑。

### 3.4 设置与 Cabin

| 方法 | 路径 | 参数 | 用途 |
| --- | --- | --- | --- |
| `GET` | `/settings` | 无 | 获取当前服务配置摘要 |
| `GET` | `/cabins` | 无 | 获取 cabin 状态和位置 |

#### `GET /settings`

返回：

| 区块 | 字段 |
| --- | --- |
| `game` | `farmName`、`farmType`、`profitMargin`、`startingCabins`、`spawnMonstersAtNight` |
| `server` | `maxPlayers`、`cabinStrategy`、`separateWallets`、`existingCabinBehavior` |

#### `GET /cabins`

返回：

| 字段 | 含义 |
| --- | --- |
| `strategy` | cabin 管理策略 |
| `totalCount` | cabin 总数 |
| `assignedCount` | 已分配数量 |
| `availableCount` | 可用数量 |
| `cabins` | cabin 列表 |
| `savedPositionPlayerIds` | 有保存位置记录的玩家 ID |

cabin 字段：

| 字段 | 含义 |
| --- | --- |
| `tileX` / `tileY` | 位置 |
| `isHidden` | 是否隐藏 |
| `type` | 类型，默认 `Normal` |
| `ownerId` | 归属玩家 ID |
| `ownerName` | 归属玩家名称 |
| `isAssigned` | 是否已分配 |

### 3.5 密码保护/认证

| 方法 | 路径 | 参数 | 用途 |
| --- | --- | --- | --- |
| `GET` | `/auth` | 无 | 获取密码保护状态 |
| `POST` | `/auth/timeout` | query: `value` | 设置认证超时时间 |

#### `GET /auth`

返回：

| 字段 | 含义 |
| --- | --- |
| `enabled` | 密码保护是否启用 |
| `authenticatedCount` | 已认证玩家数 |
| `pendingCount` | 待认证玩家数 |
| `timeoutSeconds` | 认证超时秒数 |
| `maxAttempts` | 最大尝试次数 |

#### `POST /auth/timeout`

参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `value` | integer | 是 | 非负整数。`0` 表示不超时 |

### 3.6 诊断与文档

| 方法 | 路径 | 参数 | 用途 |
| --- | --- | --- | --- |
| `GET` | `/diagnostics/state` | query 可选 | 读取游戏引擎即时状态 |
| `GET` | `/diagnostics/handler-timing` | 无 | 获取 handler 耗时统计 |
| `GET` | `/docs` | 无 | API 文档 UI |
| `GET` | `/swagger/v1/swagger.json` | 无 | OpenAPI JSON |

#### `GET /diagnostics/state`

这是直接读取游戏引擎状态的诊断接口，不依赖普通快照。主要用于测试失败分析、卡死分析、导入存档验证等。

可选参数：

| 参数 | 说明 |
| --- | --- |
| `masterFlag` | 检查 master 的 `mailReceived` 是否包含指定 flag |
| `masterEvent` | 检查 master 的 `eventsSeen` 是否包含指定事件 |
| `masterFriendKey` | 检查 master 对指定 NPC 的 friendship points |

返回内容较多，包括：

- 当前日期时间、游戏模式、最近 tick、平均游戏线程等待。
- 在线 farmer 数、other farmer UID。
- NetReady、新一天同步状态。
- cabin 诊断、farmhand 诊断。
- 正在断开的 farmer。
- FarmHouse、Cellar、master player 相关状态。
- `failedFields`，表示读取失败的字段。

#### `GET /diagnostics/handler-timing`

返回 handler 聚合耗时。目前代码中主要统计 `/farmhands`。

字段包括：

| 字段 | 含义 |
| --- | --- |
| `handler` | handler 名称 |
| `calls` | 调用次数 |
| `snapshotReadAvgMs` | 快照读取平均耗时 |
| `serializeAvgMs` | JSON 序列化平均耗时 |
| `emitAvgMs` | 写响应前准备平均耗时 |
| `writeAvgMs` | 响应写入平均耗时 |
| `totalAvgMs` | 总平均耗时 |

## 4. 长轮询接口

长轮询接口用于减少前端频繁轮询。匹配成功时返回对应普通接口的响应；超时返回 `408`。服务端会把 `timeout` 限制在约 10 秒内。

部分长轮询响应会带：

```http
X-Predicate-Changed-At-Ms-Ago: <milliseconds>
```

用于表示匹配条件大约在多少毫秒前变为满足。

| 方法 | 路径 | 参数 | 成功响应 |
| --- | --- | --- | --- |
| `GET` | `/wait/status` | `since`、`isReady`、`isPaused`、`day`、`playerCount`、`timeout` | 同 `/status` |
| `GET` | `/wait/players` | `since`、`playerId`、`timeout` | 同 `/players` |
| `GET` | `/wait/farmhands` | `since`、`farmhandCount`、`hasFarmhand`、`requireCustomized`、`timeout` | 同 `/farmhands` |
| `GET` | `/wait/health` | `ready`、`timeout` | 同 `/health` |

### `GET /wait/status`

用途：

- 等待新的状态快照版本。
- 等待 `isReady` 变为指定值。
- 等待 `isPaused` 变为指定值。
- 等待日期 `day` 变为指定值。
- 等待玩家数 `playerCount` 变为指定值。

参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `since` | long | 客户端已知的快照版本。服务端只匹配更新版本 |
| `isReady` | boolean | 可选，等待 ready 状态 |
| `isPaused` | boolean | 可选，等待 pause 状态 |
| `day` | integer | 可选，等待日期 |
| `playerCount` | integer | 可选，等待玩家数 |
| `timeout` | integer | 可选，毫秒 |

### `GET /wait/players`

用途：

- 等待玩家列表更新。
- 等待指定 `playerId` 出现在玩家列表中。

参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `since` | long | 客户端已知的快照版本 |
| `playerId` | long | 可选，等待指定玩家出现 |
| `timeout` | integer | 可选，毫秒 |

### `GET /wait/farmhands`

用途：

- 等待 farmhand 列表变化。
- 等待 farmhand 数量达到指定值。
- 等待指定名称 farmhand 出现。
- 等待指定 farmhand 的 `isCustomized` 达到指定值。

参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `since` | long | 客户端已知的快照版本 |
| `farmhandCount` | integer | 可选，等待 farmhand 数量 |
| `hasFarmhand` | string | 可选，等待指定名称 farmhand 出现 |
| `requireCustomized` | boolean | 可选，需要与 `hasFarmhand` 配合 |
| `timeout` | integer | 可选，毫秒 |

### `GET /wait/health`

用途：

- 等待游戏线程恢复健康。
- 冷启动或卡死恢复检测。

参数：

| 参数 | 类型 | 说明 |
| --- | --- | --- |
| `ready` | boolean | 常用 `true`，表示等待 `isFrozen=false` |
| `timeout` | integer | 可选，毫秒 |

## 5. WebSocket 接口

路径：

```text
ws://localhost:8080/ws
```

WebSocket 主要用于实时聊天转发。消息统一为 JSON：

```json
{
  "type": "message_type",
  "payload": {}
}
```

### 5.1 鉴权

如果设置了 `API_KEY`，连接后 10 秒内必须发送：

```json
{
  "type": "auth",
  "payload": {
    "token": "your-api-key"
  }
}
```

成功响应：

```json
{
  "type": "auth_success"
}
```

失败响应：

```json
{
  "type": "auth_failed",
  "error": "Invalid token"
}
```

### 5.2 Client -> Server

| type | payload | 用途 |
| --- | --- | --- |
| `auth` | `{ "token": "api-key" }` | WebSocket 鉴权 |
| `ping` | 无 | 心跳 |
| `chat_send` | `{ "author": "Name", "message": "Hello" }` | 发送聊天到游戏内 |

### 5.3 Server -> Client

| type | payload | 用途 |
| --- | --- | --- |
| `auth_success` | 无 | 鉴权成功 |
| `auth_failed` | `{ "error": "reason" }` | 鉴权失败 |
| `pong` | 无 | 心跳响应 |
| `chat` | `{ "playerName": "Name", "message": "Hello", "timestamp": "ISO8601" }` | 游戏内聊天事件 |

## 6. 测试专用接口

`/test/*` 接口只在 `Env.IsTest` 为真时可用。生产环境会返回 404，不应作为面板功能依赖。

这些接口经过正常 API 鉴权后才会路由；不要把 `/test/*` 加入免鉴权列表。

### 6.1 测试 GET 接口

| 方法 | 路径 | 参数 | 用途 |
| --- | --- | --- | --- |
| `GET` | `/test/crops` | 无 | 枚举世界中所有作物 |
| `GET` | `/test/festival_state` | 无 | 读取当前节日状态 |
| `GET` | `/test/farmers` | query: `location`，默认 `Farm` | 读取指定地点的 farmer 位置 |
| `GET` | `/test/save_tmp_exists` | query: `saveName` | 检查存档主文件旁是否存在 `.tmp` |

### 6.2 测试 POST 接口

| 方法 | 路径 | 参数 | 用途 |
| --- | --- | --- | --- |
| `POST` | `/test/set_date` | JSON body | 直接设置季节、日期、年份 |
| `POST` | `/test/farmevent` | query: `type` | 安排次日晚间 FarmEvent，目前支持 `qiplane` |
| `POST` | `/test/saver_crop` | JSON body | 修改 CropSaver 条目 |
| `POST` | `/test/house_upgrade` | query: `command` | 执行 debug 房屋升级命令，用于验证拦截 |
| `POST` | `/test/stamp_claim` | 无 | 给未自定义 farmhand 造一个 abandoned slot claim |
| `POST` | `/test/galaxy_relogin` | 无 | 触发 Galaxy 重新登录 |
| `POST` | `/test/seed_import_source` | JSON body | 构造导入存档测试源状态 |
| `POST` | `/test/import_save` | JSON body | 克隆并导入存档 |
| `POST` | `/test/console` | JSON body | 调用已注册 SMAPI console command |
| `POST` | `/test/corrupt_save` | JSON body | 克隆并破坏存档，用于恢复/失败路径测试 |

### 6.3 测试接口请求体

#### `POST /test/set_date`

```json
{
  "season": "spring",
  "day": 1,
  "year": 1
}
```

`season` 支持 `spring`、`summer`、`fall`、`winter`。

#### `POST /test/saver_crop`

```json
{
  "locationName": "Farm",
  "tileX": 10,
  "tileY": 10,
  "extraDays": 1,
  "ownerId": 620826087702429092,
  "datePlanted": {
    "season": "spring",
    "day": 1,
    "year": 1
  }
}
```

#### `POST /test/seed_import_source`

```json
{
  "ownerName": "ImportedOwner",
  "houseUpgradeLevel": 2,
  "caveChoice": 1,
  "spouse": "Abigail",
  "mailFlag": "exampleFlag",
  "eventSeen": "12345",
  "shadowFriendshipPoints": 2500,
  "shadowFriendshipKey": "Krobus",
  "daysPlayed": 100,
  "placeChest": true,
  "chestTileX": 3,
  "chestTileY": 3,
  "placeFridgeItem": true,
  "spawnPet": true,
  "placeCellarItem": true,
  "injectFarmhandUserId": "test-user-id"
}
```

#### `POST /test/import_save`

```json
{
  "sourceSaveName": "SourceFarm_123",
  "targetSaveName": "TargetSeed",
  "swapHostTo": "PlayerName",
  "skipClone": false
}
```

#### `POST /test/console`

```json
{
  "name": "saves",
  "args": ["reload", "--force"]
}
```

#### `POST /test/corrupt_save`

```json
{
  "sourceSaveName": "SourceFarm_123",
  "targetSaveName": "BrokenSaveSeed"
}
```

## 7. 面板接入建议

### 首页/总览

建议使用：

- `GET /status`
- `GET /players`
- `GET /farmhands`
- `GET /cabins`
- `GET /health`
- `GET /stats`

刷新方式：

- 简单实现可定时轮询 `/status`、`/players`、`/farmhands`。
- 更优实现可使用 `/wait/status`、`/wait/players`、`/wait/farmhands`。

### 服务器控制

建议功能对应：

| 面板功能 | API |
| --- | --- |
| 开启/关闭渲染 | `POST /rendering?fps=0/15` |
| 设置游戏时间 | `POST /time?value=1200` |
| 设置时间倍率 | `POST /clock-speed?multiplier=10` |
| 重新加载世界 | `POST /reload` |
| 创建新游戏 | `POST /newgame` |

注意：

- `/reload` 和 `/newgame` 会拒绝在线玩家场景。
- `/time` 和 `/roles/admin` 需要服务器已经进入可玩状态。

### 玩家管理

建议功能对应：

| 面板功能 | API |
| --- | --- |
| 在线玩家列表 | `GET /players` |
| farmhand 槽位列表 | `GET /farmhands` |
| 授予 admin | `POST /roles/admin` |
| 删除离线 farmhand | `DELETE /farmhands` |

注意：

- 删除 farmhand 前应在 UI 中提示“必须离线”。
- 优先用 `playerId` 操作，比名称更稳定。

### 聊天/Discord 转发

如果面板需要实时聊天：

- 接入 WebSocket `/ws`。
- 发送 `chat_send` 可把面板消息转入游戏。
- 监听 `chat` 可展示游戏内玩家发言。

### 诊断页

建议功能对应：

| 面板功能 | API |
| --- | --- |
| 健康状态 | `GET /health` |
| 性能指标 | `GET /stats` |
| 游戏线程卡死分析 | `GET /diagnostics/state` |
| handler 性能分析 | `GET /diagnostics/handler-timing` |
| 当前截图 | `GET /screenshot` |

## 8. 注意事项

1. 生产面板不要依赖 `/test/*` 接口。
2. `POST /newgame`、`POST /reload` 都可能耗时较长，前端应显示加载状态，并处理 `409`、`503`、`504`。
3. `GET /screenshot` 返回 base64 PNG，可能较大，不适合高频刷新。
4. `GET /diagnostics/state` 是诊断接口，内容多，建议只在诊断页或错误详情中请求。
5. 如果设置了 `API_KEY`，面板后端代理请求时应统一注入 `Authorization`，不要把 API key 暴露给浏览器前端。
6. 长轮询接口返回 `408` 属于正常超时，不应当作为错误弹窗处理。
