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
