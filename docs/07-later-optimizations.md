# 后期优化文档

本文只记录已经讨论过、但当前不急着做的优化。当前优先级仍是保持 Stardew 单实例面板稳定。

## 玩家事件驱动更新

当前玩家页通过 5 秒轮询：

```text
GET /api/instances/:id/players
```

后端读取：

- `.local-container/control/players.json`
- `.local-container/control/players-cache.json`

这个成本很低，不走 Docker exec，不扫大日志，当前足够稳定。

当前已完成第一版“最近事件”：后端通过 `players.json` 与 `players-cache.json` 差分生成首次记录、加入和离开事件，写入 `.local-container/control/players-events.json`，并通过 `/players` 响应的 `recentEvents` 返回。它仍属于轮询快照机制，不是实时推送。

未来如果要做更实时的事件推送：

1. SMAPI mod 继续写 `players.json` 当前快照。
2. SMAPI mod 新增 `player-events.jsonl`，追加 join/leave 事件。
3. 后端保留普通 `/players` 首次加载和兜底。
4. 新增：

```text
GET /api/instances/:id/players/stream
```

5. 前端先请求 `/players`，再订阅 SSE；断线后降级 30 秒低频轮询。

注意：

- 不要解析黄色弹窗、截图、VNC 或 OCR。
- 文件 watcher 在 Windows/WSL2/Docker bind mount 下可能不可靠，要有 mtime 兜底。
- 事件日志要轮转或截断，不能无限增长。

## 实时服务器日志流

当前任务日志和部分 command 输出已经可用，但完整 server log tail 仍可独立优化：

- 后端统一封装 compose logs tail。
- 前端 Jobs/Diagnostics 提供过滤、下载、暂停滚动。
- 支持包继续只导出尾部日志并脱敏。

## 计划重启设计

目标：给管理员一个可预期、可取消、有玩家提示的“维护窗口”能力。第一阶段不做复杂 cron，而是让用户配置每天几点关闭服务器、几点重新开启服务器。

### 产品语义

- “关闭时间”：到点开始维护，面板先广播提醒、创建当前激活存档备份，然后停止服务器。
- “开启时间”：到点结束维护，面板自动启动服务器，并按现有启动流程等待新邀请码。
- 如果关闭时间和开启时间相同，不允许保存；如果开启时间早于关闭时间，表示跨天维护窗口，例如 23:30 关闭、06:00 开启。
- 计划只负责服务器生命周期，不承诺强制保存游戏内实时进度；关闭前备份只备份已落盘存档。
- 第一阶段只支持单实例 Stardew 的一条计划，后续 Multi Game Mode 再泛化。

### 前端弹窗

入口：服务器控制页“快捷操作”的“计划重启”按钮。点击打开弹窗，不直接执行。

弹窗字段：

- 启用计划：开关。
- 每天关闭：`HH:mm` 时间输入，24 小时制。
- 每天开启：`HH:mm` 时间输入，24 小时制。
- 关闭前提醒：多选或固定标签，默认 `10 / 5 / 1` 分钟。
- 关闭前备份：默认开启，文案写“备份已保存进度”。
- 在线玩家策略：第一阶段建议只提示并继续执行；后续可加“有玩家在线则跳过本次维护”。
- 下次关闭时间、下次开启时间、最近执行结果：只读摘要。

弹窗操作：

- 保存计划：`PUT /api/instances/:id/restart-schedule`。
- 暂停计划：把 `enabled=false` 保存即可；可以不用单独 pause 接口。
- 取消下一次维护：可选第一阶段暂不做；如做则只跳过最近一组关闭/开启窗口。
- 立即关闭并按计划开启：不建议第一阶段加入，避免绕过现有停止确认。

### 后端 API

建议契约：

```text
GET /api/instances/:id/restart-schedule
PUT /api/instances/:id/restart-schedule
POST /api/instances/:id/restart-schedule/skip-next
```

请求体：

```json
{
  "enabled": true,
  "shutdownTime": "04:00",
  "startupTime": "04:20",
  "timezone": "Asia/Shanghai",
  "warningMinutes": [10, 5, 1],
  "backupBeforeShutdown": true,
  "skipIfPlayersOnline": false
}
```

响应体：

```json
{
  "schedule": {
    "enabled": true,
    "shutdownTime": "04:00",
    "startupTime": "04:20",
    "timezone": "Asia/Shanghai",
    "warningMinutes": [10, 5, 1],
    "backupBeforeShutdown": true,
    "skipIfPlayersOnline": false,
    "nextShutdownAt": "2026-07-03T04:00:00+08:00",
    "nextStartupAt": "2026-07-03T04:20:00+08:00",
    "lastShutdownAt": null,
    "lastStartupAt": null,
    "lastStatus": "idle",
    "lastMessage": ""
  }
}
```

校验规则：

- `shutdownTime/startupTime` 必须是 `HH:mm`，分钟粒度，24 小时制。
- `timezone` 第一阶段固定 `Asia/Shanghai`，前端可先不暴露选择。
- `warningMinutes` 去重、排序，只允许 1 到 120 分钟。
- `backupBeforeShutdown` 第一阶段默认 true；允许关闭，但 UI 要有确认提示。
- `shutdownTime != startupTime`。

### 后端架构

建议新增模块：

```text
backend/internal/scheduler/restart_schedule.go
backend/internal/storage/restart_schedule.go
backend/internal/web/restart_schedule_handlers.go
```

职责划分：

- storage：保存配置、最近执行状态、跳过下一次标记。优先 SQLite，字段带 `instance_id`，为多实例留口。
- scheduler：常驻 goroutine，按实例计算下一次 shutdown/startup/warning 触发点。
- web handler：鉴权、校验、读写配置、写审计日志。
- lifecycle：不新增并行控制逻辑，继续复用现有 start/stop job 或抽出共享 service，确保和手动启停互斥。

执行流程：

1. 后端启动时加载所有启用计划，计算下一次关闭、开启和提醒时间。
2. 每次 `PUT` 保存后，scheduler 重新计算该实例下一次触发点。
3. 到达提醒时间：如果实例 running，调用现有喊话能力广播“服务器将在 N 分钟后维护关闭，预计 HH:mm 开启”。
4. 到达关闭时间：创建 `scheduled_shutdown` job。
5. job 内重新读取状态；如果已有生命周期 job running/queued，则本次关闭标记为 `skipped_busy`。
6. 如果 `backupBeforeShutdown=true` 且有 active save，调用现有 `BackupSave` 创建手动备份；失败则默认阻止关闭并记录 `failed_backup`。
7. 调用现有停止流程，停止完成后更新 `lastShutdownAt/lastStatus`。
8. 到达开启时间：创建 `scheduled_startup` job。
9. job 内如果实例已经 running/starting，则标记 `skipped_already_running`；否则调用现有启动流程并等待状态刷新。

### 前端状态流

- `api.ts` 新增 `getRestartSchedule()`、`updateRestartSchedule()`、`skipNextRestartSchedule()`。
- `types.ts` 新增 `RestartSchedule` / `RestartScheduleResult`。
- `ServerControlPage` 新增计划重启弹窗 state：`scheduleOpen`、`scheduleDraft`、`scheduleLoading`、`scheduleSaving`、`scheduleError`。
- 打开弹窗时拉取当前配置；保存成功后关闭或留在弹窗并显示“已保存”。
- 快捷操作按钮旁可以展示一个短摘要：`计划：04:00 关闭 / 04:20 开启`。

### 安全边界

- 不做“强制保存世界”前置，除非后续 SMAPI/Junimo 明确提供可靠 API。
- 所有自动动作写审计日志，metadata 不包含邀请码或敏感信息。
- 计划任务必须走和手动生命周期一致的互斥逻辑，不能直接 `docker compose stop/up` 绕开状态机。
- 关闭前广播失败不阻断维护；关闭前备份失败默认阻断维护。

## 存档与备份增强

可选方向：

- 自动备份策略：启动前、停止后、定时备份。
- 备份备注和保留策略。
- 存档对比：日期、农场、玩家、地图、大小。
- 恢复前二次确认和冲突预览。

禁止方向：

- 不做破坏性“修复存档角色槽”的自动工具。
- 不直接改 Stardew 主存档 XML 来猜测联机异常。

## Mod 管理增强

可选方向：

- 启用/禁用 Mod。
- 依赖关系和版本检查：`MODDEPS-2` 已判断依赖是否已安装、当前存档是否启用、最低版本是否满足，并在前端提示缺失/未启用/版本不足；后续还需要做缺失依赖的一键安装入口、依赖来源索引和更新提示。
- Mod 更新提示。
- Mod 配置编辑。

需要注意：Stardew / SMAPI Mod 生态差异很大，先做只读检测和用户确认，不要自动修改复杂配置。

## 多游戏模式

未来进入 Multi Game Mode 后：

- 显示总面板实例列表。
- 每个游戏有自己的后端 driver 和前端 game module。
- 通用能力只包括用户、权限、任务、日志、诊断、备份入口。
- Stardew 专属 Steam Guard、邀请码、农场设置不能泄漏到其他游戏页面。

## UI 精修

可继续优化：

- 小屏表格改卡片。
- 右侧 rail 折叠。
- 部分 PNG 单独微调。
- 错误态和空态进一步统一。
- Overview 减负，把复杂管理收进对应页面。

每次视觉改动都要跑 `npm.cmd run build`，并检查 1280px、390px、320px。
# REMOTE-MOD / MODSEARCH 后续优化
- 远程安装目前只支持 ZIP。后续如需开放 7z/rar，必须先引入可靠解压器，并补齐 zip-slip、绝对路径、符号链接、单文件大小、总解压大小等同等级安全校验。
- 当前线上搜索入口是 Nexus-only，旧 `/mods/search` 统一搜索骨架已撤回。后续如重新做多来源搜索，需要重新设计接口，再接 StardewModDataset 本地持久化索引/后台刷新（完整名称全文搜索、依赖和 UpdateKeys 匹配）、CurseForge Core API、GitHub Releases（release asset）、ModDrop 稳定下载元数据。
- 多来源一键安装优先级可参考：CurseForge `download-url`、ModDrop `download-url`、GitHub Release / 直链、Nexus Premium、Nexus NXM/临时 CDN。没有自动链接时兜底为管理员粘贴 ZIP URL 或上传文件。
- ModDrop 当前不做页面抓取；除非确认存在稳定公开 API 或 Dataset 中能可靠映射到可下载 ZIP，否则只保留为后续统一搜索来源候选和管理员粘贴 ZIP 直链安装来源。
- Nexus 多文件 Mod 需要文件选择 UI，当前仍自动选 primary/main/first。
# SCHEDULED-RESTART-1 后续增强

计划重启第一阶段已落地：管理员弹窗、`GET|PUT /restart-schedule`、SQLite `restart_schedules`、后台 30 秒轮询、提前广播、关闭前备份、复用 Stop/Start lifecycle job。后续如果继续增强，优先考虑一次性跳过下一次维护、持久化 warning 去重、更多日历规则和更细的玩家在线策略。
