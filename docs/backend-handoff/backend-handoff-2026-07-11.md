# 2026-07-14 接手补充：升级矩阵复用安装镜像候选

### 改了什么

- server 升级候选改为 `dockerproxy.net → docker.1ms.run → docker.1panel.live → docker.jiaxin.site → dockerproxy.link → Docker Hub`；steam-auth-cn 候选改为 `1ms → ACR → DaoCloud → GHCR → Docker Hub`，与安装常量完全同序。
- 所有别名仍分别登记精确 ref，但同一组件只能有一个 canonical digest。运行时任何拉取失败、digest 缺失或不一致都会继续后续候选，不接受实例 `.env` 注入目标。
- release gate 对 canonical Docker Hub 和自有 ACR/GHCR 严格要求在线；第三方代理暂时不可达只告警，但一旦可访问则 digest 不一致会立即失败。

### 影响接口与文件

- 无 API/响应字段变化。
- 修改 `config/runtime_stack_manifest.json`、`config/runtime_stack.go`、`config/runtime_stack_test.go`、`scripts/compatibility_matrix.py` 和脚本单测。

### 如何验证

- `go test ./internal/games/stardew_junimo/config ./internal/games/stardew_junimo`。
- `python scripts/compatibility_matrix.py validate ...`、Python 单测与 `verify-remote-artifacts`；远程核验确认可访问别名 digest 一致，三个第三方 server 代理当前不可达时仅告警。

### 下一步注意事项

- 修改安装候选顺序时必须同步更新嵌入矩阵，否则 `TestBuiltInRuntimeStackManifestIsValid` 应阻止合并。
- 不要把“代理返回的不同 digest”加入矩阵；即使层内容看似相同，也只能接受与 canonical OCI index digest 完全一致的别名。

# JUNIMO-STACK-UPDATE-1 阶段二 dry-run 接手记录（2026-07-13）

## 改了什么

- `stardew_junimo` 新增成对升级预检服务、八阶段状态、专用 job 和原子状态文件；目标只能取阶段一 embed 清单。安装/生命周期/runtime update 创建任务使用同一 driver 锁和 active jobs 查询双向互斥。
- Docker 新增受限 image/config/volume inspect 与受控两镜像环境覆盖的 Compose quiet 校验；只保留结构化摘要，丢弃 Compose 展开环境和原始 pull 输出。
- Web 新增管理员 POST/GET dry-run，POST 仅接受空 body/严格 `{}`；错误、日志和响应不包含 Steam/registry 凭据或完整环境。

## 影响接口/文件

- 接口：`POST/GET /api/instances/:id/junimo-update/dry-run`；job type `stardew_junimo_update_dry_run`；状态文件 `<instance>/.local-container/junimo-update/dry-run-status.json`。
- 核心文件：`internal/docker/runtime_update.go`、`internal/jobs/manager.go`、`internal/games/stardew_junimo/runtime_update_dry_run*.go`、`driver.go`、`lifecycle.go`、`internal/web/junimo_update_handlers.go`/`instance_handlers.go` 及对应测试。

## 如何验证

- `go test ./...` 覆盖候选首项失败后成功、全失败、digest 缺失、卷缺失、Compose 失败、custom/not-installed、运行中不停车、互斥、脱敏、持久恢复和管理员权限/请求体注入。
- 交付前另执行 `go vet ./...`、`go build ./...` 与 `git diff --check`。所有 Docker 流程测试使用 fake，没有触碰真实实例。

## 下一步注意

- 阶段三 apply 尚未实现。不得直接复用 selected 字段执行；应定义成功 dry-run 的过期/配置漂移校验、成对备份/停服/重建/写回/健康验收和完整回滚状态机。
- dry-run 详细事实只认状态文件，jobs 只认生命周期/互斥；不要创建第二套会覆盖它的状态。不要为卷可读性扩大 Panel 宿主挂载。

# JUNIMO-STACK-UPDATE-1 阶段一接手记录（2026-07-13）

## 改了什么

- `stardew_junimo/config/runtime_stack_manifest.json` 是构建内置且可审查的唯一推荐版本对：server `1.5.0-preview.121` + steam-auth-cn `1.5.0-anxi.2`。`runtime_stack.go` 负责清单强校验、可信仓库校验、实例 `.env` 五字段读取与五态判断；Web 层不实现 Stardew 版本逻辑。
- `stardew_junimo/runtime_stack.go` 根据实例安装状态调用配置层检测。`GET /api/instances/:id/junimo-update` 仅管理员可读，只返回挑选后的版本/运行态字段；普通用户从 `/state.runtimeDiagnostic` 获取不含仓库的 tag 和整体状态。
- runtimeDiagnostic 不再用 `strings.Contains(serverImage, testedTag)`，而是复用完整版本对检测；自定义镜像固定为 `custom_images` + `unsupported/custom_images`。

## 影响文件与接口

- 清单/判断：`backend/internal/games/stardew_junimo/config/runtime_stack_manifest.json`、`runtime_stack.go`、`runtime_stack_test.go`、`backend/internal/games/stardew_junimo/runtime_stack.go`。
- Web：`backend/internal/web/junimo_update_handlers.go`、`junimo_update_handlers_test.go`、`instance_handlers.go`、`instance_ui_status.go`。
- 新接口：`GET /api/instances/:id/junimo-update`（管理员，GET-only）。现有 `GET /api/instances/:id/state` 的 `runtimeDiagnostic` 新增版本对字段并停止返回 `junimoImage` 仓库引用。

## 如何验证

- `go test ./internal/games/stardew_junimo/config ./internal/games/stardew_junimo ./internal/web` 覆盖清单拒绝规则、完全匹配、单边旧、双边旧、自定义镜像、缺 `.env`、未安装、权限和敏感字段不泄漏。
- 完整验证命令仍是 `gofmt`、`go test ./...`、`go vet ./...`、`go build ./...`；阶段一测试不得执行 Docker pull/stop/up。

## 下一步注意事项

- 更新推荐版本时必须把 server/auth 当作一次原子变更，并同时更新内置清单、现有安装默认常量和一致性测试；不要改成远程 latest 或 semver 猜测。
- 阶段二/三的 capability、dry-run、apply、备份/回滚尚未实现；在明确设计并单独授权前，不得给本接口加请求体、目标镜像参数或任何 Docker/.env 写操作。

# PANEL-UPDATE-RELEASE-1 后端接手补充（2026-07-13）

## 改了什么

- 完成成功升级与 unhealthy 自动回滚的隔离真 Docker 发布验收，并修复 helper `/deployment` 挂载导致升级后 Compose labels 不可复用的问题。
- `internal/updater/docker_cli.go` 现在保持宿主安装目录与 Compose 文件的原绝对路径；`updater_contract_test.go` 增加 dry-run/apply 路径回归断言。
- smoke 脚本补 Windows PowerShell 5.1 UTF-8、构建日期参数和构建失败依赖隔离。

## 如何验证与下一步

- 已通过 `go build ./...`、`go vet ./...`、`go test ./...`、Docker integration、成功/回滚 E2E、fresh-volume smoke 和 `git diff --check`。
- E2E 的 0.1.13/0.1.14 是本地测试注入版本，未推送 registry。用户确认正式版本后必须再验证真实可信仓库 pull；历史版本首次进入 updater 发布仍需一次既有部署更新。
- 不要移除 `--no-deps panel`、绝对 Compose 路径、备份权限或三项健康验收；不要把私有备份加入支持包。

# FE-PANEL-UPDATE-1 后端接口消费补充（2026-07-13）

- 前端现已完整消费既有 update/dry-run/apply 契约，无新增后端接口。apply POST 继续无 body，浏览器断线后只查询状态，不会重复提交。
- `/health`、`/api/version` 和持久 apply 状态是 Web 自动重连的三项输入；反向代理或后续 API 改动必须保持 `/health` 与 `/api/version` 无登录可读、版本精确，并保持 apply 终态跨进程可读。
- 前端对 `rollback_failed` 只提供可理解的管理员联系提示，不执行额外破坏性恢复动作。后端仍是升级和回滚结果的唯一事实来源。
- 验证：前端状态机/生产构建及桌面、窄屏、移动浏览器 QA；后端本阶段无代码变化。

# PANEL-UPDATE-APPLY-1 后端接手补充（2026-07-13）

## 改了什么

- 新增 apply API、跨重启状态、SQLite `VACUUM INTO` 备份和独立 helper 的真实 panel 单服务升级/三项验收/自动回滚链路。
- helper 参数、镜像候选、Compose project/file/service 都是结构化白名单；日志和状态不写 registry 凭据、`PANEL_SECRET` 或完整环境。

## 影响接口与文件

- `POST/GET /api/system/update/apply`
- `backend/internal/storage/backup.go`
- `backend/internal/updater/apply_*.go`、`service.go`、`docker_cli.go`、`types.go`
- `backend/cmd/panel-updater/main.go`、`backend/internal/web/updater_handlers.go`

## 如何验证

- `cd backend; go test ./...`
- `cd backend; $env:PANEL_RUN_DOCKER_UPDATE_TEST='1'; go test ./internal/updater -run TestDockerIntegrationApplyUsesIsolatedComposeProject -count=1 -v`
- `cd backend; go build ./cmd/panel ./cmd/panel-updater`

## 下一步注意事项

- `rollback_failed` 必须提示人工处理，不能自动重试破坏性步骤。不要把 helper 扩展成任意 shell/镜像/service 执行器，也不要移除 `--no-deps panel`。
- 备份目录是私有恢复材料，不得加入支持包或下载接口。完整前端恢复体验属于 `PANEL-UPDATE-UX-2`。

# PANEL-UPDATER-DRYRUN-1 后端接手补充（2026-07-13）

## 改了什么

- 新增 `internal/updater`：自容器 inspect、Compose/显式部署识别、可信镜像白名单、helper 参数构造、原子 JSON 状态和 dry-run 引擎。
- 新增独立 `cmd/panel-updater`，由面板 detached 启动；只执行 image inspect/pull 和 compose config。
- 新增管理员 capability 与 dry-run GET/POST API；run.sh 补四个宿主机部署变量，Dockerfile 同镜像构建 updater。

## 影响文件和接口

- `backend/internal/updater/*`、`backend/cmd/panel-updater/main.go`
- `backend/internal/web/updater_handlers.go`、`handler.go`、`backend/cmd/panel/main.go`
- `backend/internal/config/config.go`、`Dockerfile`、`deploy/run.sh`、`deploy/docker-compose.yml`
- 接口：`GET /api/system/update/capability`、`GET|POST /api/system/update/dry-run`（全部管理员）。

## 如何验证

- `cd backend; go test ./internal/updater -run DockerContract`
- `cd backend; go test ./...`
- `cd backend; go build ./...`
- contract tests 覆盖标准 labels、缺 labels 拒绝、显式兜底、镜像白名单、helper 无 shell、dry-run 无破坏命令、状态原子替换和重启读取。

## 下一步注意事项

- supported 仅表示可安全演练，不表示已授权真实升级。不要把 succeeded 直接接成 compose up。
- 状态是跨进程共享文件；面板必须在启动 helper 前完成最后一次写入，避免覆盖 helper 的完成状态。
- 当前只支持单一 compose config 文件与 service=panel。多文件 Compose、自定义编排、普通 docker run 默认 unsupported。
- helper 日志禁止加入 Docker stderr、compose config 输出、环境变量和凭据；后续执行阶段也必须延续。

# PANEL-UPDATE-CHECK-1 后端接手补充（2026-07-13）

## 改了什么

- 新增版本检测服务，读取构建注入的 version/commit/buildDate，通过 `netdns.NewClient` 查询 GitHub 正式 Releases，过滤 draft/prerelease 并进行语义版本比较。
- 启动立即检查，之后默认 6 小时加随机抖动；成功结果驻留内存，网络失败保留上次成功数据并暴露错误状态。
- 新增 `GET /api/system/update`（登录用户）和 `POST /api/system/update/check`（管理员）。

## 影响文件和接口

- `backend/internal/updatecheck/service.go`、`service_test.go`
- `backend/internal/web/update_handlers.go`、`update_handlers_test.go`、`handler.go`
- `backend/cmd/panel/main.go`

## 如何验证

- `cd backend; go test ./...`
- `cd backend; go build ./...`
- 单测覆盖版本比较、稳定 Release 筛选、缓存保留、网络失败、dev/非法版本和权限。

## 下一步注意事项

- 当前缓存只在进程内，重启后会重新检查；这是本阶段接受的持久策略。
- 不要在 API 层添加容器升级逻辑。本阶段没有 apply/upgrade；后续升级执行需走独立设计，并优先复用 Junimo/部署层能力。

# SAVE-BACKUP-GAMEDAY-1 存档回档功能重构：游戏内日期驱动的自动回档点

## 背景

用户要求把自动回档点完全按照"游戏内日期"（年/季/日）管理，不再按现实自然日/整点管理：取消定时备份，自动回档点按游戏日序号排序和去重覆盖，默认保留最近 5 个不同游戏日（可设 1-14），每次正式回档前必须先创建一份不占用自动配额、不被自动清理删除的"回档前保护备份"，手动备份和删除存档前备份同样不占用自动回档额度。

调研确认了两个关键前提，使得本次重构**完全不需要改动 SMAPI 控制模组、不需要重新编译嵌入 DLL**：

1. 触发时机要求"游戏内睡觉并成功保存、存档已经落盘后"——现有 `ModEntry.cs` 的 `OnSaved`（订阅 SMAPI `GameLoop.Saved`，官方文档保证在游戏完成写盘后触发）早已把存档事件写入 `.local-container/control/save-events/*.json`；后端 `RunBackupMaintenance()` 只需要继续消费这些事件即可，无需新增触发逻辑。
2. 游戏年/季/日早已由 `readSaveInfo`/`fillSaveInfoFromXML`（`saves.go`）从存档 XML 解析为 `GameYear`/`GameSeason`/`GameDay`，`enrichBackupInfo` 也已从 ZIP 内解析同样字段用于展示。只需新增一个"游戏日序号"换算函数，不需要新的解析逻辑。

## 改了什么

### 1. `BackupPolicy` 简化（`saves.go`）

```go
type BackupPolicy struct {
    GameSaveBackups bool `json:"gameSaveBackups"`
    RetainGameDays  int  `json:"retainGameDays"`
}
```

删除 `DailySnapshots`/`DailyRetentionDays`/`ScheduledBackups`/`ScheduledHour`/`ScheduledIntervalHour`。`DefaultBackupPolicy()` 变为 `GameSaveBackups=true, RetainGameDays=5`；`normalizeBackupPolicy` 把 `RetainGameDays` clamp 到 `[1,14]`（`<=0` 时回落默认 5）。

兼容性：`gameSaveBackups` 字段名没变，旧 `policy.json` 这个值会自动延续；`scheduledBackups`/`scheduledHour`/`dailySnapshots`/`dailyRetentionDays` 等旧字段会被 Go 的 `encoding/json` 静默忽略，读取不报错——这是 Go 对未知 JSON 字段的默认行为，不需要额外写兼容代码。

### 2. 游戏日序号

新增：

```go
func gameDayOrdinal(year int, season string, day int) int {
    return (year-1)*112 + seasonIndex(season)*28 + day
}
func seasonIndex(season string) int // spring=0 summer=1 fall|autumn=2 winter=3，默认 0
```

`BackupInfo` 新增 `GameDayOrdinal int json:"gameDayOrdinal,omitempty"`，在 `enrichBackupInfo` 解析出 `GameYear>0` 时一并算出并返回，前端排序/展示时直接用这个值，不需要在 TypeScript 里重复实现季节序号映射。

### 3. 备份文件命名与四类新前缀

沿用既有的"临时创建 + 原子改名覆盖"模式（`backupSaveAs`），新增/替换的创建函数：

- `BackupManual(dataDir, saveName)` → `manual_<save>_<timestamp>.zip`。管理员点击"手动备份"、服务器控制页"备份已保存进度"快捷操作、计划重启关闭前备份，三处调用点统一从裸 `BackupSave` 改调这个。
- `BackupPreDelete(dataDir, saveName)` → `predelete_<save>_<timestamp>.zip`。`DeleteSaveWithBackup` 内部改调。
- `BackupPreRestore(dataDir, saveName)` → `prerestore_<save>_<timestamp>.zip`。`RestoreBackup` 覆盖前保护备份改调，失败仍然中止恢复（行为完全不变，只是换了个文件名前缀）。
- `BackupAutoGameDay(dataDir, saveName)`：读**当前存档目录**（不是 ZIP）算出 `gameDayOrdinal`，目标名 `auto_<save>_<ordinal六位补零>.zip`。同一 ordinal 会被 `backupSaveAs` 的"移除已有目标再改名"语义自然覆盖旧文件——这一条设计同时满足两个需求：
  - "同一游戏日多次保存时覆盖该游戏日已有的自动回档点"
  - "回档到较早日期后重新游玩到相同游戏日，新产生的自动回档点覆盖该游戏日的旧自动回档点"
  两者本质上是同一件事（文件名只取决于游戏日序号，与真实时间、与这是第几次到达这一天无关），不需要额外的状态文件去记录"上一次生成时间"之类的东西。存档日期解析失败（`GameYear<=0`/`GameDay<=0`）时返回错误，不会生成 ordinal=0 的脏文件。
- `PruneAutoGameDayBackups(dataDir, saveName, retainGameDays)`：按 `auto_<save>_` 前缀枚举文件，从文件名解析出 ordinal 排序（**不**看文件 mtime），只保留 ordinal 最大的 N 个，其余删除。

删除的旧机制（scheduled 相关整体移除，`maintenance-state.json` 不再需要）：`BackupLatest`、`BackupScheduled`、`BackupDailySnapshot`、`PruneDailySnapshots`、`dailySnapshotDate`、`runScheduledBackupIfDue`、`scheduledBackupDue`、`readBackupMaintenanceState`/`writeBackupMaintenanceState`/`backupMaintenanceStatePath`/`backupMaintenanceState` 类型。

`inferBackupKind`/`parseBackupSaveName` 新增 `auto_`/`manual_`/`predelete_`/`prerestore_` 前缀识别；**保留**旧 `latest_`/`daily_`/`scheduled_` 前缀识别——这三类不再产生新文件，但磁盘上已有的旧 ZIP 继续被正确识别、读取、展示，前端把它们归入"其他备份 → 历史备份"，不做任何自动删除（避免误删用户文件）。无前缀的最老式 `BackupSave` 遗留文件（`<save>_<timestamp>.zip`）继续按默认分支归为 `manual`。

### 4. `RunBackupMaintenance` 重写

```go
func RunBackupMaintenance(dataDir string) (BackupMaintenanceResult, error) {
    policy, _ := ReadBackupPolicy(dataDir)
    // 遍历 save-events/*.json（不变）
    // 每个事件：若 policy.GameSaveBackups，调用 BackupAutoGameDay + PruneAutoGameDayBackups(policy.RetainGameDays)
    // 删除事件文件、ConsumedEvents++（不变）
    // 不再有 scheduled 分支
}
```

调用方 `handleSavesBackupsList`（`GET /api/instances/:id/saves/backups`）完全不变，仍在每次请求时顺带触发维护。

### 5. 顺带修复：`backupSaveAs` 的临时文件命名碰撞

写新测试时在 Windows 上实际复现了一个此前就存在、和本次重构逻辑本身无关的 bug：`backupSaveAs`（`BackupLatest`/`BackupScheduled`/`BackupDailySnapshot` 时代就有的辅助函数）内部靠调用 `BackupSave` 来生成"临时文件"，而 `BackupSave` 自己选择的文件名是秒级时间戳 `<saveName>_<timestamp>.zip`——如果这个临时名恰好和另一个正被打开读取的备份 ZIP 同名（比如 `RestoreBackup` 里"作为恢复源的原始 ZIP"和"覆盖前保护备份的临时文件"在同一秒内先后产生，两者都撞上裸命名模式），Windows 会因目标文件被占用而 `rename` 失败；在其它平台上则可能是更隐蔽的静默覆盖/损坏。已经把核心打包逻辑抽成 `writeSaveZip(dataDir, saveName, backupName)`，`backupSaveAs` 改用纳秒时间戳 + 目标名拼接出的 `.tmp-*` 临时名，从根上消除这个碰撞窗口，`BackupSave`（公开函数，行为不变）也复用这个核心函数。

### 6. Web / 调度器调用点

- `lifecycle_handlers.go: handleSaveBackupCreate`（`POST /saves/:name/backup`）改调 `sj.BackupManual`。
- `restart_schedule_handlers.go: backupActiveSave`（关闭前备份）改调 `sj.BackupManual`——归入"手动备份"桶，不占用自动回档配额。
- `handleSavesBackupPolicy`/`handleSavesBackupsList`/`handleSavesBackupRestore`/`handleSavesBackupDelete` 的路由、URL、权限逻辑全部不变；`ensureInstanceNotRunning` 继续在恢复前拦截运行中的实例，返回 `409 server_running`。

## 影响文件

- `backend/internal/games/stardew_junimo/saves.go`
- `backend/internal/games/stardew_junimo/saves_test.go`
- `backend/internal/web/lifecycle_handlers.go`
- `backend/internal/web/restart_schedule_handlers.go`

前端改动见 `docs/frontend-handoff/frontend-handoff-2026-07-11.md` 的 `SAVE-BACKUP-GAMEDAY-1` 小节。

## 兼容策略

- 旧 `policy.json` 含 `scheduledBackups`/`scheduledHour`/`dailySnapshots`/`dailyRetentionDays`：读取时被 Go 结构体自动忽略，不报错；`gameSaveBackups` 字段名不变，值自动延续。
- 已有 `latest_*.zip`/`daily_*_*.zip`/`scheduled_*.zip` 磁盘文件：不删除、不迁移，`inferBackupKind` 继续识别，前端归入"其他备份 → 历史备份"展示，用户仍可手动恢复/删除。
- `ListBackups`/恢复/删除接口 URL 和参数不变，只是 `kind`/`policy` 的取值集合变化。

## 如何验证

- 新增/替换测试（`saves_test.go`）：
  - `TestBackupPolicy_DefaultAndClamp`：默认 `RetainGameDays=5`，clamp 到 14。
  - `TestReadBackupPolicy_IgnoresLegacyFields`：旧 policy.json 字段不报错，`retainGameDays` 缺失时回落默认 5。
  - `TestGameDayOrdinal_CrossSeasonAndYear`：表驱动覆盖跨季节、跨年份排序。
  - `TestBackupMaintenance_SaveEventCreatesAutoGameDayBackup`：save-event 驱动生成 `auto_<save>_<ordinal>.zip`。
  - `TestBackupAutoGameDay_OverwritesSameGameDay`：同一游戏日两次保存只保留一个文件。
  - `TestBackupAutoGameDay_RestoreEarlierThenReplaySameDayOverwrites`：回档到早期游戏日后重新游玩回原日期，覆盖旧回档点而不是新增。
  - `TestPruneAutoGameDayBackups_KeepsFiveMostRecentGameDays`：默认保留 5 个游戏日的清理逻辑。
  - `TestRunBackupMaintenance_DoesNotTouchManualOrProtectionBackups`：自动清理循环多次触发后，手动备份和保护备份文件仍然存在。
  - `TestDeleteSaveWithBackup_UsesPreDeletePrefix`/`TestRestoreBackup_CreatesPreRestoreProtectionBackup`：三类保护性备份的前缀与创建时机。
  - `TestRestoreBackup_AbortsWhenProtectionBackupFails`：让保护备份的 `BackupSave` 因目标不是目录而失败，断言恢复整体中止且当前"存档"内容未被触碰。
  - `TestInferBackupKind`/更新 `TestParseBackupSaveName`：覆盖全部新旧前缀。
- `cd backend; go build ./... && go vet ./... && go test ./...` 全绿（含 `internal/games/stardew_junimo`、`internal/web`、`internal/storage`、`cmd/panel` 等全仓库回归）。

**未做的验证**：没有连接真实运行中的 JunimoServer 实例走一遍"睡觉存档 → 自动回档点生成 → 回档到早期游戏日 → 重新游玩到相同日 → 确认覆盖旧回档点"的完整端到端链路。本次改动完全基于单元测试和对 SMAPI `GameLoop.Saved`（存档写盘后触发）文档行为的信任，没有实机验证这个时序假设在当前嵌入 DLL 版本下依然成立。建议下一位维护者找一个测试实例，开着面板"游戏日回档"页面，实际睡觉存档几次、切换到不同游戏日再切回来，确认回档点数量、排序和覆盖行为符合预期。

## 下一步注意事项

- `writeSaveZip`/`backupSaveAs` 的临时文件现在用 `.tmp-<nanotimestamp>-<目标名>` 命名，不会被 `ListBackups`（过滤 `.zip` 后缀）之外的逻辑特殊处理；如果备份过程中进程被杀掉，理论上可能残留极少量 `.tmp-*.zip` 孤儿文件（原有 `BackupSave` 失败时的清理逻辑本就有这个已知的、未处理的边界情况，本次没有新增专门的孤儿文件清理，维持现状）。
- 自动回档的保留策略是**按存档名分别计数**的（`auto_<saveName>_` 前缀按 saveName 精确匹配），不是全局共享一个"最近 N 个"配额。如果用户在多个存档之间切换着玩，每个存档各自维护自己的最近 N 个游戏日，这是有意为之（游戏日序号本来就只在同一个存档的时间线里有意义），前端"游戏日回档"列表目前展示的是所有 `kind==='auto'` 的条目（每行自带农场信息），没有额外按当前激活存档过滤——正常使用场景下这基本等价于"只有当前在玩的存档有自动回档点"，如果以后要支持多存档并行维护回档点并需要分组展示，需要在前端加一层按 `saveName` 分组的 UI，这次没有做。
- 计划重启（`SCHEDULED-RESTART-1`）关闭前备份现在归为 `manual` kind，如果以后想把它单独归类展示（比如"计划维护备份"），需要新增一个前缀和对应的 Web/前端标签，目前复用 `BackupManual` 是最小实现。

## 同日追加：修复回档成功后遗留 `.restore-tmp-*` 临时目录

用户实际点了手机端新做的"回档到此日"后，存档库出现一张"解析失败：未找到 SaveGameInfo 文件"的卡片，名字是 `.restore-tmp-2156104854`。定位到 `RestoreBackup`（本次重构之前就存在的老 bug，这次是用户第一次实测才触发）：它在 `Saves/` 目录内部创建 `.restore-tmp-*` 临时目录解压 ZIP，再把里面的存档子目录 rename 到最终位置，但**只在失败分支清理临时目录，成功分支从不清理**——成功后这个已经清空的临时目录会永远留在 `Saves/` 里，而 `listSaveDirs`（`ListSaves` 的数据源）此前不过滤目录名，会把它当成一个损坏的存档展示出来。

两处修复：
1. `RestoreBackup` 的 `defer` 改为无条件 `_ = os.RemoveAll(tempDir)`（成功/失败都清理），删除不再需要的 `success` 局部变量。
2. `listSaveDirs` 跳过所有 `.` 前缀目录——这条对磁盘上**已经存在**的历史残留目录立即生效，不需要用户手动清理就能让它从存档列表消失。

新增测试 `TestListSaveDirs_SkipsDotPrefixedTempDirs`，并给 `TestRestoreBackup_Success` 追加"回档成功后 `Saves/` 下没有任何 `.restore-tmp-*` 残留"的断言。影响文件仍是 `saves.go`/`saves_test.go`，`go build/vet/test ./...` 全绿。详见 `docs/02-backend.md` 的 `SAVE-BACKUP-GAMEDAY-1` 追加修复小节。

**注意**：这个修复不会主动删除磁盘上已经存在的 `.restore-tmp-*` 孤儿目录（比如用户截图里那个），只是让它不再出现在存档列表里；如果要彻底清理磁盘空间，需要手动进入 `.local-container/saves/Saves/` 删除对应文件夹。

## 同日追加：回档时自动停止/重启服务器（SAVE-RESTORE-AUTORESTART-1）

用户反馈运行中点击回档只会被禁用+提示"请先停止服务器"，要求改成"确认后自动停止服务器、完成回档、再重新启动服务器"。

**设计取舍**：`Driver.Stop` 是 fire-and-forget（内部提交一个 job 立刻返回，不等真正停止完成），如果在 HTTP handler 里自己写"调用 Stop 后轮询实例状态直到确认停止"，既要处理 `State` 字段在"停止中"和"已停止"两个阶段都是 `InstanceStateStopped`（只有 `DriverPhase` 区分 "stopping"/"stopped"）的坑，又要处理 HTTP 请求长时间挂起（ComposeDown 默认超时 2 分钟）的风险，还偏离了这个仓库"操作提交即返回 jobId、前端轮询"的一贯架构。改为把"停止 → 回档 → 启动"实现成同一个 lifecycle job 内部的三个顺序步骤，直接复用 `lifecycleRunner.doStop`/`doStart`（原样调用，不重新实现 compose/Mod 同步/邀请码轮询），前端拿到的还是一个普通 jobId，用现成的 job 轮询/SSE 展示进度——和点"启动服务器"按钮的体验完全一致。

**改了什么**：
- `lifecycle.go`：`lifecycleRunner` 新增 `restoreBackupName`/`restoreOverwrite` 字段和 `"restore_restart"` operation；新增 `doRestoreAndRestart`（若运行中先 `doStop`，然后 `RestoreBackup`——和同步回档路径同一个函数，回档失败直接返回不会去启动——若之前运行中最后 `doStart`）；新增 `Driver.RestoreBackupWithRestart(...)`，和 `Start`/`Stop`/`Restart` 同构，提交一个 `restore_restart` job 并返回 jobId。
- `lifecycle_handlers.go`：`POST .../saves/backups/restore` 请求体新增 `autoRestart`；运行中且 `autoRestart=false` 仍然 409（不改变现有调用方行为）；运行中且 `autoRestart=true` 走新增窄接口 `backupRestoreRestarter`（和 `festival_handlers.go` 的 `festivalEventTrigger` 同一套类型断言风格）调用 `RestoreBackupWithRestart`，返回 `202 {jobId}`；已停止时行为完全不变（`200 {saveName}`）。

**影响文件**：`lifecycle.go`、`lifecycle_test.go`（新增）、`console_test.go`（`fakeConsoleDocker` 补 `ComposeDown`/`ComposeUp` mock）、`lifecycle_handlers.go`、`saves_handlers_test.go`。

**测试策略说明**：这个仓库里所有"需要真实 `registry.GameDriver`"的 web 接口（`players_handlers.go`、`festival_handlers.go` 等）历来都没有 HTTP 层集成测试，只在 `stardew_junimo` 包内针对核心函数/`lifecycleRunner` 方法写测试（例如 `ban_test.go`）——这次延续同样的分层策略，没有为这一个功能新增一个 15 方法的 fake `GameDriver` 去测试 web handler 的成功路径。已覆盖的部分：
- `stardew_junimo` 包内：`TestRestoreBackupWithRestart_RequiresJobManager`、`TestDoRestoreAndRestart_StoppedSkipsStopAndStart`、`TestDoRestoreAndRestart_RunningStopsThenRestoresBeforeStarting`（后者故意让 `ComposeUp` 返回错误，避免需要把 `doStart` 完整成功路径——容器就绪轮询、邀请码轮询——全部 mock 出来，同时仍能验证"先停止、期间完成回档、再尝试启动"这个顺序）。
- `internal/web` 包内：`TestSavesBackupRestore_RunningWithoutAutoRestartReturns409`/`_RunningWithAutoRestartBypassesRunningGate`/`_StoppedStillWorksSynchronously`，只验证路由分支选择正确，不验证 driver 调用后的真实效果。

**未做的验证**：没有连接真实运行中的 JunimoServer 实例，实际点一次"运行中回档"，确认服务器真的自动停止、回档、重新启动，玩家能重新连进正确的存档。`doStart`/`doStop` 本身在这个仓库里也从来没有端到端自动化测试，这次继承了同样的验证空白，建议下一位维护者用测试实例走一遍。

## 下一步注意事项（追加）

- `doRestoreAndRestart` 直接复用 `doStop`/`doStart` 方法体，这意味着以后如果这两个方法的内部逻辑变化（比如新增一个启动前置检查），"回档自动重启"会自动跟着变化，不需要单独维护一份重复逻辑——但也意味着如果 `doStart`/`doStop` 出现 bug，这条路径会一起受影响，排查时要意识到这三者共享同一套底层实现。
- 如果回档本身成功但重新启动失败（`doStart` 内部任意一步出错），job 会整体标记为 `failed`，前端只能看到"任务失败"和日志里具体哪一步出错；没有做"回档成功但启动失败"这种部分成功状态的特殊 UI 展示，管理员需要看 job 日志或去服务器页手动重试启动。
# PLAYER-OFFLINE-SAVE-FALLBACK-1 离线玩家信息完善

## 改了什么

- `saveRosterFarmer` 新增解析 `homeLocation`、`lastSleepLocation`、`lastSleepPoint` 和 `useSeparateWallets`。
- 从存档构造离线名册时，位置优先使用 `lastSleepLocation`，缺失时回退 `homeLocation`；坐标使用 `lastSleepPoint`。前端“位置”列保持原样，语义是离线玩家在存档中的最后睡眠位置。
- 独立钱包时使用 Farmer 的 `totalMoneyEarned` 兜底 `personalIncome`；共享钱包只返回农场累计收入，不伪造个人收入。
- 新增 `mergePlayerCacheFallback`：存档字段只填补缓存缺口，不覆盖控制模组在玩家在线时记录的更准确位置、坐标和收入。
- `cacheMatchesSave` 新增存档身份兼容：`FarmName` 与 `FarmName_<纯数字ID>` 视为同一存档，解决 `players.json`/`players-cache.json` 使用基础 ID、`status.json` 使用完整目录名时潜在的历史缓存失效；其它后缀仍严格区分。

## 影响文件与接口

- `backend/internal/games/stardew_junimo/players.go`
- `backend/internal/games/stardew_junimo/players_test.go`
- `GET /api/instances/:id/players` 的 URL 和响应结构不变，仅原先缺失的可选字段会更完整。

## 如何验证

- `go test ./internal/games/stardew_junimo/...` 通过。
- `go test ./...` 后端全仓库通过。
- 新增测试覆盖存档位置/坐标/独立钱包收入解析、基础 ID 与数字后缀目录 ID 的兼容，以及运行时位置不被存档兜底覆盖。

## 下一步注意事项

- 存档无法恢复真实的历史登录时间；`lastSeen` 仍只会在面板实际观察到玩家在线后写入缓存。
- `lastSleepLocation` 是存档时的睡眠位置，不等同于玩家断线瞬间的位置。产品已确认前端列名继续显示“位置”，无需增加“存档位置”文案。
- 已经丢失的历史运行时数据不会凭空恢复；部署本版本后，存档可提供的字段会立即补齐，玩家后续登录产生的运行时字段会继续由缓存优先保留。
# REAL-INSTANCE-LIFECYCLE-BACKUP-VERIFIED-1 生命周期与回档真实实例验证补记

- 用户已确认真实环境验证通过：大存档启动并等待主机上线、睡觉后生成游戏日自动回档点、运行中回档自动执行“停止→回档→重启”。
- 本补记取代下文 `SAVE-BACKUP-GAMEDAY-1`、`SAVE-RESTORE-AUTORESTART-1` 对应的“未做真实实例端到端验证”记录。
# UI-LIFECYCLE-STATUS-1

- 改动：`instance_ui_status.go` 聚合实例、driverPhase、活动 lifecycle job、`status.json`、`players.json`，并由 `/state` 返回七态 `uiStatus`。
- 影响：`backend/internal/web/instance_handlers.go`、`backend/internal/web/instance_ui_status.go`；接口新增字段，旧字段不变。
- 验证：前端 TypeScript 检查通过；后端全包测试当前被工作区既有的 `internal/storage` 重复 `nullString` 定义阻塞。
- 注意：状态文件是只读读取；未来增加阶段耗时时应持久化阶段切换时间，不要从文件 mtime 反推。
- 后续补齐：`runtimeDiagnostic` 已包含 active/cache saveId、存档目录、控制模组与 Junimo 版本匹配、两段启动耗时；测试覆盖存档身份与耗时边界。当前耗时是结构化 JSON 时间戳差值，不使用文件 mtime。
# PLAYER-ROSTER-SQLITE-1 玩家名册 SQLite 化

## 改了什么

- 新增 migration `008_player_roster.sql`：`save_identities` 维护基础/完整存档 ID 映射，`player_roster` 以实例、稳定存档 ID、玩家 ID 为联合主键。
- 新增 `internal/storage/player_roster.go`，提供名册 upsert/list；保存首次出现、最后观测、最后在线、位置、坐标、收入、钱包与来源快照。
- `ListPlayers` 在现有 `players.json + players-cache.json + 存档 XML` 合并后同步 SQLite，再从 SQLite 补齐离线历史。旧缓存成功导入后删除，生产路径不再写 `players-cache.json`；无 Store 的 driver 单元测试/降级运行仍保留旧兼容路径。
- 第二期在同一 migration 增加 `player_events` 和 `player_roster.current_status`。每次运行时快照同步会按数据库上次状态生成 `seen/joined/left`，`recentEvents` 改为读取 SQLite；旧 `players-events.json` 幂等导入后与名册缓存一起退役。
- 完整存档目录名优先作为稳定 ID；缺少 `UniqueMultiplayerID` 时暂用姓名身份，拿到正式 ID 后按同存档同名合并。

## 影响文件与接口

- `backend/migrations/008_player_roster.sql`
- `backend/internal/storage/player_roster.go` / `player_roster_test.go`
- `backend/internal/games/stardew_junimo/players.go` / `players_test.go`
- `GET /api/instances/:id/players` 结构不变；`saveId` 更稳定，离线来源可能为 `sqlite_roster`。

## 如何验证

- `cd backend; go test ./...`
- 集成测试 `TestListPlayersMigratesLegacyCacheToSQLiteRoster` 验证旧缓存导入、文件退役和后续 SQLite 离线恢复。
- 存储测试验证首次/最后时间、最后在线不被离线刷新覆盖、最新快照更新、改名身份稳定、临时姓名身份晋升，以及首次在线→离开→重新加入的事件顺序。

## 下一步注意事项

- 名册与最近活动现在都由 SQLite 承担历史职责；`players.json` 是唯一持续使用的 JSON 玩家输入，只表示当前运行时快照。
- 旧缓存仅在一次请求内导入；若个别记录写库失败，文件会保留供下次重试，不会提前删除。
- 需要在真实升级实例确认 `players-cache.json`、`players-events.json` 首次访问玩家页后消失，并检查 `panel.db` 中同名不同存档没有串记录、玩家上下线事件没有重复。
# COMMAND-RESULT-PROTOCOL-1 控制命令回执阶段 1

## 改了什么

- driver 命令文件协议加入稳定 `commandId`、JSON 内 `id`、临时文件 + 原子 rename，并创建 `command-results/`。
- 控制模组加入协议 v1 能力标志、七状态 `CommandOutcome`、结构化错误码与结果原子写入；`HandleCommand` 改为返回 outcome，但阶段 1 的现有命令仍统一 `dispatched`，没有接入玩家操作精确结果。
- 消费使用持久 `running` 闸门防重复执行，结果成功写入后才删除命令；残留命令遇到已有结果只删除、不执行。
- 新增只读 API `GET /api/instances/:id/commands/:commandId`，Web handler 只负责鉴权/错误映射，状态判定与清理由 `stardew_junimo` driver 完成。

## 影响文件

- Go：`command_results.go`、`console.go`、`driver.go`、各玩家/节日命令提交文件、`instance_handlers.go`、`lifecycle_handlers.go` 及 `command_results_test.go`。
- Mod：`embedded/smapi-mod-src/ControlContract.cs`、`ModEntry.cs`、重编后的 `embedded/smapi-mod/StardewAnxiPanel.Control.dll`。
- 文档：`docs/02-backend.md`、`docs/06-integration.md`、`docs/08-future-roadmap.md` 和本文件。

## 如何验证

- `cd backend; go test ./internal/games/stardew_junimo ./internal/web`。
- `cd backend; go test ./...`。
- Docker 构建：`dotnet build -c Release /p:GamePath=/game`，本次 0 errors（仅 1 个既有 analyzer warning），DLL SHA256 更新为 `7E6CC3ACE96EE155F20C53FD908AE4286F96C5DA853E08D1DDE708364471B110`。

## 下一步注意事项

- 阶段 1 必须停在 `dispatched`；不要提前让踢出、回家、认证等返回精确 succeeded/failed。
- 已知崩溃窗口：模组写入 `running` 后、覆盖终态回执前崩溃，最终会显示 `unknown / execution_interrupted`；这是宁可不确定也不重复执行的设计，不得自动重试。
- 终态保留 7 天，expired 墓碑再保留 24 小时；queued 与活跃 running 不清理。后续如做后台定时清理，可复用现有 driver 函数，不要在 Web handler 重写文件协议。
# PLAYER-COMMAND-RESULTS-1 精确玩家命令回执

- `warp-home`、`kick`、`approve-auth` 的 Mod 处理函数已返回结构化 outcome，成功为 `succeeded/ok`，失败码完整记录在 `docs/02-backend.md`；不再用 `status.json` 充当单命令结果。
- 认证桥通过 `InvocationFailed` 区分 Junimo 明确拒绝与反射/服务失败；三条命令均在消费时重新检查世界、ID、在线状态和主机保护。
- 新增 `PlayerCommandOutcomes.cs` 与 `embedded/smapi-mod-contract-tests`，Docker contract test 覆盖全部错误码和成功封装。Mod 构建、Go 全量测试及真实实例验证结果见本节最终验证记录。
- 未接入 ban、broadcast、event、joja；它们继续返回 dispatched。
- 嵌入 DLL 已重新编译替换，SHA256 为 `5433F2E891550169EE8FBD3D7F52169A9190F2E918D1291D2696AC782B0B493D`。
- 真实多人验证当前未执行：工作机 `docker ps` 没有运行容器，实例 `players.json` 仅有 2026-07-12 前的主机单人陈旧快照；in-app Browser 与 Chrome 也没有打开的面板实例。没有可安全操作的在线 farmhand，不能拿主机测试（协议明确禁止）或把旧快照冒充成功。待用户提供正在运行且有测试 farmhand/待认证玩家的实例后，依次验证回家、重新加入后踢出、未认证状态下批准，并检查对应结果 JSON。
# BROADCAST-BAN-RESULTS-1

- broadcast 已从 dispatched 升级为真实聊天系统调用回执：`sendChatMessage` 返回才 succeeded，且文案不承诺客户端送达；空消息、世界、聊天系统、异常均为结构化 failed。
- ban payload 携带 ID + admin 提升证据。Mod 优先 ID 定位并直接调用 `Game1.server.ban(id)`；直接 API 不可用才按唯一名字派发 `!ban`，重名拒绝，派发成功只返回 dispatched。Go 侧 admin 提升失败统一为 `admin_promotion_failed`。
- 用户已人工验证容器重启后封禁丢失，取代此前“是否持久化未知”的记录。未实现封禁名单、重放补偿或解封入口。
- 验证入口：`embedded/smapi-mod-contract-tests`、`go test ./...`、`npm run test:command-results`、`npm run build`、Docker SMAPI build。真实喊话/封禁仍需有在线测试玩家的运行实例完成本版 DLL 端到端验证。
- 嵌入 DLL SHA256：`12BC1C4201AB17F0873EE9ABF7A548A1A5D140EC8970C008770CE6F8EB532B2F`。已同步到本机真实 `stardew-server-1` 并重启加载；broadcast 实测结果为 `succeeded/ok`，命令文件在结果落盘后删除。ban 对唯一在线主机实测返回 `failed/host_not_supported`，主机保护生效。当前无 farmhand 在线，直接 `Game1.server.ban(id)` 的真实 succeeded 分支仍待有测试玩家时补验。

# EVENT-JOJA-SAVE-RESULTS-1

## 改了什么

- 控制模组新增 `DeferredCommandOutcomes.cs`。event 明确检查世界、当天节日、节日现场和聊天系统，聊天成功只 dispatched；Joja 检查 admin 证据，只有存档 `JojaMember` 才 succeeded；save-now 使用两分钟 pending tracker 关联下一次 `GameLoop.Saved`。
- Go 新增 `Driver.RequestSaveNow` 与 `POST /api/instances/:id/saves/save-now`。Joja admin 提升失败在 v1 模组上提交 `adminPromoted:false` 让模组写结构化 failed；旧模组继续返回兼容错误。
- 已知崩溃窗口：Saved 已发生但 succeeded 原子写入前崩溃时，结果会从 running 变 unknown，tracker 不恢复，也绝不自动重试。

## 影响文件与验证

- 主要文件：`console.go`、`joja.go`、`festival_handlers.go`、`instance_handlers.go`、`ModEntry.cs`、`DeferredCommandOutcomes.cs`、contract tests 与嵌入 DLL。
- `go test ./...`、Docker contract tests、前端测试/构建通过；SMAPI Mod 构建 0 errors（1 个既有 analyzer warning）。DLL SHA256：`ADF4473AF58BBFC58C1A4735389B07F269D73BC40AFD4F7626A3D0C68F2E7EBC`。
- 用户授权使用现有 `1111_442923526`：event 为 `failed/no_festival_today`；Joja 日志确认命令解析，回执保持 `dispatched/ok`；save commandId `c1178eb65b034c96814416dc04c101f9` 在 `GameLoop.Saved` 后由 running 转 succeeded。

## 下一步注意事项

- `dispatched` 不能改写成最终成功；Joja 仅有聊天日志不能证明持久路线状态。save tracker 是进程内状态，崩溃后按 unknown 处理，不能恢复后重放。

# COMMAND-RESULT-PRODUCTIZATION-1 接手记录（2026-07-12）

## 改了什么与影响文件

- 新增 `migrations/009_control_commands.sql`、`internal/storage/control_commands.go`、`internal/web/control_commands.go`。所有文件队列提交都记录安全的 actor/目标元数据；后台导入结果后删除交接文件，单命令查询和历史接口读 SQLite。
- `cmd/panel/main.go` 启动命令结果 scheduler；`internal/config/config.go` 增加 30 天/1000 条默认值及两个环境变量。`instance_ui_status.go`、`health.go` 增加协议诊断和卡死告警。
- 测试位于 `control_commands_test.go`（storage/web）及更新后的 driver result tests，覆盖迁移、幂等、重启、入库后删除、running 闸门和清理边界。

## 验证与注意事项

- `cd backend; go test ./...`；`cd frontend; npm run build`。
- 结果文件必须在数据库事务成功后删除。不要清理 queued/running；不要删除尚未入库的结果；不要从 unknown 自动重放。终态历史按 30 天或数量上限清理。
- active running 文件可能已入库但仍保留，这是故意的模组防重复闸门，不应计为“未入库”。最终审计只写一次，并通过 control_commands 关联 actor、目标和最终状态。
- 本阶段没有更改 C# 或嵌入 DLL；上一阶段 DLL hash 仍为 `ADF4473AF58BBFC58C1A4735389B07F269D73BC40AFD4F7626A3D0C68F2E7EBC`。
- 真机链路：临时隔离 DB + 临时 actor 通过 API 向真实实例提交 `say`，`64a0853e85c997d6b14ad6af48805f29` 为 queued→succeeded/ok，历史 API 的命令、actor、完成时间完整，`command-results` 清零。临时 DB/会话/进程均已删除；真实生产用户与认证库未改动。
# SAVE-BACKUPS-EMPTY-LIST-1 空备份列表契约修复（2026-07-13）

- 改动：`backend/internal/games/stardew_junimo/saves.go` 的 `ListBackups()` 在目录不存在及空目录场景都返回非 nil 空切片，使备份列表 API 输出 `backups: []` 而非 `null`。
- 影响接口：`GET /api/instances/:id/saves/backups`；不改变非空备份数据结构及备份维护策略。
- 验证：`cd backend; go test ./internal/games/stardew_junimo ./internal/web`；`TestListBackups_EmptyDir` 新增非 nil 回归断言。
- 注意：Go 新增返回分支时继续避免把 nil slice 直接暴露到 JSON 数组契约。
# INSTALL-RUNTIME-VERIFICATION-1 安装状态机运行文件闭环（2026-07-13）

- 改动：`Driver.verifyGameDataVolume()` 使用已拉取的 JunimoServer 镜像挂载 `<project>_game-data:/data/game`，统一验证 Stardew 主程序/DLL/413150 manifest、SMAPI 启动器/DLL、1007 manifest 与 Steam SDK `steamclient.so`。SteamCMD 的 `validate` 负责完整 depot 校验；此检查负责阻止“日志成功但卷为空/残缺”的状态误判。
- 影响：`completeInstall()` 在写 `game_installed` 前强制验证；`doStart()` 在启动容器前强制验证；`ReconcileState()` 会修复历史 `game_installed`、`save_required`、`ready_to_start`、`starting`、`running`、`stopped` 的残缺卷状态为 `error/install_verification_failed`。Docker/image 探测失败不改状态，以免临时运行环境问题造成误降级。
- 授权边界：`AuthLoginOnly` 现在恢复调用前保存的 state/phase，仅记录 Steam 授权成功，不再把安装失败伪装为 `game_installed`。
- 验证：`TestDriverInstallFailsWhenRequiredGameRuntimeFilesAreMissing`、`TestDriverReconcileStateMarksInstalledStateInvalidWhenGameVolumeFilesAreMissing`、`TestDriverAuthLoginOnlyMarksSteamAuthCompletedAndRefreshesService`；完整 `cd backend; go test ./...` 通过。
- 下一步注意：若 JunimoServer 后续升级 Steam SDK 所需文件名/目录，应只更新 `verifyGameDataVolume()` 的 SDK 规则和同一测试中的文件清单，安装完成与启动/协调三条路径会自动保持一致。

# STEAMCMD-CREDENTIAL-REUSE-1 SteamCMD 独立授权缓存持久复用（2026-07-13）

## 改了什么

- 本功能只使用 SteamCMD 自己生成的登录缓存，不读取、不转换、不共享 Junimo `steam-auth` 的 refresh token。两者仍分别由 `STEAMCMD_AUTH_COMPLETED` / `STEAM_AUTH_COMPLETED` 跟踪。
- `installer.go run()` 读取 `STEAMCMD_AUTH_COMPLETED=true` 后启用 SteamCMD 用户名缓存登录：`+@NoPromptForPassword 1 ... +login "$STEAM_USERNAME"`；首次或强制换号仍使用用户名+密码完整登录。
- 缓存登录明确输出 `Cached credentials not found` / `No cached credentials` 等失效信号时，同一安装 job 会清空 SteamCMD 标志并自动回退完整登录，避免死循环或卡在密码提示。
- 统一 `<project>_steamcmd-login` 授权卷，同时挂载到 `/home/steam/Steam`、`/home/steam/.local/share/Steam`、`/root/Steam`、`/root/.local/share/Steam`。这是为了兼容 CM2 镜像的 steam 用户布局和官方 `steamcmd/steamcmd` 的 root 布局，切换候选镜像后仍读取同一份 SteamCMD config/machine authorization。
- SteamCMD 退出码 139 的重试不再删除任何授权卷；仅按 `steamcmd-login` / `steamcmd-home` 查找并移除可能残留的一次性容器后重试，防止把登录哨兵一起删掉。旧 local 卷不再挂载，也不自动删除。
- 新增旧卷自动迁移：`buildSteamCMDAuthMigrationOpts()` / `migrateLegacySteamCMDAuthCache()` 在真正登录前检查统一卷；目标无 `config/config.vdf` 时按 root-local、user-local 顺序只读查找，复制 `config/` 与卷根 `ssfn*`。目标已有缓存立即跳过，迁移 best-effort，绝不输出凭据文件内容。

## 影响文件与接口

- `backend/internal/games/stardew_junimo/installer.go`
- `backend/internal/games/stardew_junimo/driver_test.go`
- HTTP API、请求体和前端 phase 不变；缓存失效后仍复用现有 SteamCMD Guard 交互接口。

## 如何验证

- 自动化：`cd backend; go test ./internal/games/stardew_junimo`；新增迁移命令 contract 测试，覆盖统一/旧卷挂载、已有目标不覆盖、复制不 `cat` 凭据。
- 真实 Docker：现场确认旧授权仅存在于 `stardew_steamcmd-root-local/config/config.vdf`，统一卷为空；复制到统一卷后，用项目当前候选镜像和新运行时四路径挂载连续启动两个全新 SteamCMD 容器，两次均只传用户名并命中 `Logging in using cached credentials`，user info 成功、退出码 0、未要求 Steam Guard。
- 可在不输出文件内容的前提下检查 `<instance>_steamcmd-login` 卷内存在 `config/config.vdf` 或 SteamCMD 生成的 machine authorization/sentry 文件；不要把文件内容写入支持包或日志。

## 下一步注意事项

- 旧版本用户升级后，旧会话可能只存在 `steamcmd-user-local` / `steamcmd-root-local`；迁移会优先复用它。仅当旧卷没有有效缓存或 Steam 已吊销会话时，才需要重新批准。
- Steam 主动吊销设备、修改密码、账户安全策略变化或用户点击“更换 Steam 账号 / 强制重新认证”时，再次批准是预期行为。
- 已完成真实 SteamCMD 缓存登录验证，但没有执行完整 `app_update 413150 validate`（避免无必要地长时间校验游戏卷）；登录复用本身已由连续两个新容器验证。
# 2026-07-13 接手增量：JUNIMO-STACK-UPDATE-1 阶段三

## 改了什么

- `stardew_junimo/runtime_update_apply*.go` 实现管理员明确确认后的 server + steam-auth-cn 成对升级状态机、实例任务互斥、关键预检重跑、私有恢复材料、停服后 steam-session volume 克隆、五字段原子 `.env` 更新、auth-first/server-second 验收、原运行态恢复、终态审计和成对自动回滚。
- `internal/docker/runtime_apply.go` 只暴露固定 Junimo 服务、固定 Compose 参数、固定 `/steam/ready`/Junimo health 探针与固定 volume clone/restore 脚本；临时卷必须匹配当前 project 的命名规则。`cmd/panel/main.go` 在普通 jobs 恢复后扫描非终态 apply，并按持久阶段/材料继续验收、回滚或停止猜测。
- Web 新增 `POST/GET /api/instances/:id/junimo-update/apply`；POST 只接受 `{"confirm":true}`。详细状态位于 `.local-container/junimo-update/apply-status.json`，私有材料位于 `recovery/<applyId>`，支持包现有白名单不会包含它。

## 影响文件/接口

- 主要文件：`internal/games/stardew_junimo/runtime_update_apply.go`、`runtime_update_apply_runner.go`、`runtime_update_rollback.go`、`internal/docker/runtime_apply.go`、`internal/web/junimo_update_handlers.go`、`cmd/panel/main.go` 及对应测试。
- 状态终态：`succeeded`、`failed_rolled_back`、`rollback_failed`。后者必须保留材料并人工处理，禁止添加自动破坏性重试。

## 如何验证

- 单元/handler contract 覆盖确认体注入、权限、相同版本、拉取前失败、auth ready/ticket、server/auth digest、server health、成对回滚、运行/停止态、回滚失败保留材料和重启不猜测。
- `go test -tags=integration ./internal/docker -run TestRuntimeApplyIsolatedSteamSessionCloneRestore -count=1` 使用唯一 `anxijunimotest*` volume 验证认证内容在模拟迁移后恢复；不得改成生产 project/volume。

## 下一步注意事项

- 发布前仍须用专用真实 Steam 测试账号和真实清单镜像完成长流程成功/回滚/Panel 中断验收。不要把账号/token 加进自动测试、状态或支持包。
- 任何新增恢复分支必须先证明当前 `.env`、容器 digest 和私有 manifest 一致；不能根据阶段名猜测，也不能扩大 Docker 方法到任意 service/image/shell。
# 2026-07-14 接手增量：GAME-RUNTIME-VERSION-1

## 改了什么

- 在既有 embed runtime stack 清单中加入 game 413150 / SDK 1007 tested 推荐 buildid；新增逐 token ACF/VDF 解析、只读 game-data volume reader、六态检测、管理员查询与只读预检。
- 候选发现脚本/workflow 只生成 `classification=discovered` JSON/summary/artifact；413150 凭据只从受保护 Environment 进入临时 runscript，原始 SteamCMD 输出不打印。

## 影响文件/接口与验证

- 主要文件：`config/runtime_stack*`、`app_manifest.go`、`runtime_components.go`、`internal/docker/runtime_components.go`、`internal/web/junimo_update_handlers.go`、`scripts/discover-steam-builds.ps1`、`.github/workflows/discover-steam-builds.yml`。
- 接口：管理员 `GET /runtime-components` 与 `GET|POST /runtime-components/dry-run`；未安装返回 HTTP 200 明确状态。测试覆盖解析顺序/空白/转义、appid/buildid 错误、缺失、四种匹配组合、未安装、权限、脱敏和发现工具不修改矩阵。

## 下一步注意事项

- 当前只读 helper 依赖已存在 game-data volume 与本地 server 镜像；必须保留 inspect-before-run、`--pull never`、`--network none` 和 readonly mount，防止 Docker 隐式建卷/拉取。
- 阶段六尚无 staging/depot/app_update/切换/停服/重建/回滚；不要把本预检成功当作执行授权，也不要接入现有 Junimo stack apply。

## 2026-07-14 SMAPI 推荐升级接手记录

### 改了什么

- 在 embed runtime stack 清单固定 SMAPI 4.5.2 URL/SHA/大小和 Control 兼容值；新增实际 DLL 检测七态、维护者候选 CLI、严格管理员 API、可信下载/ZIP 校验、staging 官方安装、原子 GAME_DATA_VOLUME 切换、完整验收、自动回滚与 Panel 重启恢复。
- 初始安装也改为只使用内置推荐 SMAPI：Panel 侧完成 allowlist/SHA/ZIP 校验并只读 bind 给 installer，不再接受 `.env` 任意下载目标。完整玩家同步包只携带匹配推荐 SHA 的缓存 installer；全量/增量 manifest 都记录推荐版本与 checksum，增量仍没有 SMAPI payload。

### 影响接口与文件

- 主要文件：`config/runtime_stack_manifest.json`、`smapi_update.go`、`smapi_archive.go`、`smapi_update_workflow.go`、`game_data_volume.go`、`internal/docker/runtime_smapi.go`、`cmd/smapi-candidate`、`.github/workflows/discover-smapi-candidate.yml`、`mod_sync.go`、`internal/web/junimo_update_handlers.go`。
- 接口：`GET /runtime-components` 新增 `smapi`；新增 `GET /smapi-update`、`GET|POST /smapi-update/dry-run`、`GET|POST /smapi-update/apply`。POST 永远不接目标参数。

### 如何验证

- `go test ./...`、`go vet ./...`；`go test -tags=integration ./internal/docker -run 'TestRuntime(SMAPIIsolatedStagingCloneAndInstaller|ApplyIsolatedSteamSessionCloneRestore)' -count=1`。
- 候选实测：`go run ./cmd/smapi-candidate --output <临时文件>`，确认 4.5.2、41,889,142 字节和固定 SHA，随后删除临时候选。

### 下一步注意事项

- 不要把 SMAPI apply 合并进游戏 depot 或 Junimo/auth apply；任一前置矩阵不匹配必须拒绝。不要把 recovery manifest、旧卷或 staging label 变成通用 volume 删除入口。
- recovery manifest 记录旧 Control manifest/DLL 是否存在；rollback lifecycle 必须保持 `preserveControlMod`，否则普通启动的幂等部署会再次覆盖刚恢复的旧 Control。验收必须同时保留 Junimo/Control 日志加载证据、status/players、health/invite/auth ticket。
- 阶段八必须在真实推荐 stack 与存档副本上跑 installer/Control/邀请码/auth ticket 全链路及故障注入回滚；成功前不要更新 tag、镜像或推荐矩阵。

## 2026-07-14 接手补充：统一矩阵与发布列车

改动：扩展内嵌 runtime stack 为统一 schema v1，增加 channel/status/upstreamRef/sourceRevision/images/digests/SMAPI urls/controlMod；正式安装和所有更新门禁只接受 recommended 与满足最低 Panel 版本；Junimo/auth 选择器强制 digest 相符；新增 withdrawn 风险状态、Python schema/状态机/dispatch 生成器、PR/release workflows。

影响接口：原 Junimo update 与 runtime-components 响应的 recommended 对象新增 `channel`、`status`，镜像组件以 `images`、`digests` 返回；POST 仍不接收任何目标参数。初始 install 的 imageTag 即使由旧客户端发送，也只能等于内嵌 server tag。没有新增远程矩阵读取 API。

关键文件：`backend/internal/games/stardew_junimo/config/runtime_stack.go`、`runtime_stack_manifest.json`、`runtime_update_*`、`runtime_components.go`、`smapi_update_workflow.go`、`scripts/compatibility_matrix.py`、`compatibility-matrices/`、`.github/workflows/compatibility-matrix.yml`、`release.yml`。原 `receive-auth-release.yml` 已删除。

验证：`go test ./internal/games/stardew_junimo/config ./internal/games/stardew_junimo`、矩阵 Python unittest、前端 build 已通过；全量验证命令和结果见本任务最终报告。接手注意：不要让 candidate 文件进入运行时 apply 路径；新 stack 必须人工指定完整 server/auth 精确版本对并从 candidate 开始，按 candidate -> tested -> recommended 分 PR，withdrawn 不可复活；代理镜像若返回与 canonical 不同 digest 会自动跳过或明确失败，不能降低校验。

### 2026-07-14 补充：取消 auth 驱动的 Panel PR

- 删除 `.github/workflows/receive-auth-release.yml` 以及 `compatibility_matrix.py from-auth-dispatch`；steam-auth-cn 发布不再通知 Panel，也不会生成 discovered PR。
- 矩阵状态移除 discovered。维护者验证 Junimo 上游后，人工选择对应 auth tag，并一次填写 server/auth tag、digest、auth `upstreamRef`/`sourceRevision` 以及其余兼容元数据，才能创建 candidate。
- 影响文件：矩阵脚本/测试/schema/README、Go 运行时状态校验、前端状态类型和发布流程文档。验证重点：discovered 必须拒绝、新 stack 非 candidate 必须拒绝、candidate 缺任一精确 digest 或 auth 溯源必须拒绝。
## 2026-07-14 接手补充：更新事务与发布门禁安全修复

### 改了什么
- SMAPI apply/恢复改用真实 Compose 状态；切卷前重新检查并静默所有运行中的 server/auth。停服后切卷前发生 Panel 中断时，原先运行的旧服务器会被重新启动并完成旧栈验收。
- SMAPI mutation 回滚新增旧 SMAPI、Junimo/API、auth ticket、Control 日志与 status/players、邀请码全链路验收；只有全部成功才进入 `failed_rolled_back`。
- Junimo apply 从运行容器记录真实 ImageID，回滚以 ImageID 固定旧 server/auth；server 停止但 auth 运行也会先停服再快照。
- 矩阵 CI 新增跨 git 基线状态迁移检查并监听 `main`；原同仓库/同 SHA 的真实 Steam Actions artifact 门禁随后已由 2026-07-14 本机验收流程取代。

### 影响文件/接口
- 后端：`smapi_update_workflow.go`、`runtime_update_apply.go`、`runtime_update_apply_runner.go`、`runtime_update_rollback.go` 及对应测试。
- CI/发布：`scripts/compatibility_matrix.py`、`scripts/tests/test_compatibility_matrix.py`、`compatibility-matrix.yml`、`release.yml`。
- API JSON 字段不变；`serverWasRunning` 语义改为真实 Docker 状态，`failed_rolled_back` 的保证增强。Docker 状态未知时 SMAPI apply 返回 `runtime_state_unavailable`。

### 如何验证
- `cd backend; go test ./internal/games/stardew_junimo -run 'Test(SMAPIUpdate|SMAPIRollback|RuntimeUpdateApply)' -count=1`
- `python -m unittest discover -s scripts/tests -p 'test_compatibility_matrix.py'`
- `python scripts/compatibility_matrix.py validate backend/internal/games/stardew_junimo/config/runtime_stack_manifest.json`
- 发布不再读取验收 Actions run 或 artifact；维护者须先在本机 Docker Desktop 完成真实 Steam 验收，再更新 `panel-release.APPROVED_STACK_VERSION` 并审批受保护 Environment。Release 仍强制 embedded status 为 recommended、stackVersion 精确匹配，并在登录 registry 前执行远程 artifact/digest 与全量测试门禁。

### 2026-07-14 补充：本机 Steam 验收发布模式

- `.github/workflows/release.yml` 删除 `REAL_STEAM_E2E_RECORD`、GitHub Actions run/artifact 查询以及 `actions: read` 权限。
- 本机验收最低要求：目标 server/auth 精确镜像已拉取；auth 返回 `ready=true`、`has_ticket=true`；server、Control/Junimo API 可用；重启 server/auth 后认证状态无需重新扫码。报告只记录版本、时间和布尔结论，不记录任何凭据或 Ticket。
- GitHub 侧以 `panel-release` required reviewer 审批作为本机验收声明，并继续要求 `APPROVED_STACK_VERSION` 与内嵌 recommended stack 完全一致。不要在未完成本机验收时更新该变量或批准部署。

### 下一步注意事项
- 不要把回滚 pin 的 ImageID 改回漂移 tag 后再重建容器；人工恢复应以 recovery manifest 中的真实 ImageID 为准。
- 新矩阵必须保留历史文件并逐级晋升；替换 recommended 时，旧 recommended 应转为 withdrawn 留档。
- `rollback_failed` 的 recovery、旧卷、新卷或 steam-session snapshot 都不可自动清理。
# 2026-07-14 接手补充：真实镜像 inspect / auth 探针修复

## 改了什么
- `internal/docker/runtime_update.go` 的镜像 inspect 改为安全字段格式化输出；`runtime_apply.go` 的容器 inspect 同步处理，避免完整 `Config.Env` 经脱敏后破坏 JSON。
- steam-auth ready 探针由 Node.js HTTP 客户端改为 Bash `/dev/tcp`，兼容当前 `dotnet SteamService.dll` 镜像，同时保持只解析 `ready/has_ticket`。

## 影响文件、验证和注意事项
- 影响 `runtime_update.go`、`runtime_apply.go` 及对应 unit/integration tests；API 无变化。
- 验证命令：`go test ./internal/docker -count=1`；`go test -tags=integration ./internal/docker -run 'TestRuntime(InspectAndAuthProbeWithoutNode|RealImagesOptIn)' -count=1 -v`。
- 真实镜像测试需要本地已有精确镜像并设置两个 opt-in 环境变量；它不读取 token，也不能替代真实已登录 session 的发布验收。未来 auth 基础镜像若移除 Bash，必须先提供等价、受控的容器内健康探针再更新推荐矩阵。
### 2026-07-14 补充：个人项目简化发布流程（覆盖此前发布列车设计）

- 每个 Panel 版本只维护 `runtime_stack_manifest.json` 一份组件目标清单。维护者直接指定 Junimo server、对应 steam-auth-cn、game/SDK、SMAPI/Control 版本，测试后创建 Panel tag；用户升级 Panel 后按内嵌清单收到组件升级提示。
- 删除 candidate/tested 状态晋级、候选目录与 git history transition 校验。运行时只接受内嵌 `recommended`，紧急停用仍保留 `withdrawn`。
- `release.yml` 不再引用 `panel-release` Environment、required reviewer、`APPROVED_STACK_VERSION`、`REAL_STEAM_E2E_RECORD` 或验收 artifact。tag 发布仍执行精确 digest/auth 溯源、全量后端/前端与 Docker integration 门禁。

# 2026-07-14 接手增量：Panel 0.2.2 / Junimo preview.125

## 改了什么与影响文件

- 推荐矩阵、新安装默认值和 `TestedImageTag` 从 `.121` 切到 `.125`；固定 server 索引 digest，auth-cn `1.5.0-anxi.2` 保持原 tag/digest 与真实 `.121` 溯源。
- `.121` 仍是可迁移旧版本：检测结果为受支持且可更新，不强制升级，不自动修改实例。管理员继续走既有 dry-run/confirm/apply/rollback。
- 影响：`config/runtime_stack_manifest.json`、`config/env.go`、`driver.go` 及相关测试；接口形状未变。

## 如何验证与下一步

- 运行兼容矩阵 validate/check-panel-version/verify-remote-artifacts、Go 相关包与发布全量门禁；`.125` 实镜像另已验证 23 个 init 兼容脚本挂载可正常执行。
- 发版后确认 GitHub Release 与三仓库 `0.2.2/latest` 镜像，并用 Web updater 检查 0.2.1→0.2.2；不得把 `.121` 提示改成强制升级。旧联机存档 host-swap 向导留待后续独立功能。
