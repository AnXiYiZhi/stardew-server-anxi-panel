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
